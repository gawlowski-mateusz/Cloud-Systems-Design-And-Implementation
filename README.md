# Cloud-Systems-Design-And-Implementation

Conference room reservation app refactored into a 4-microservice architecture on **AWS Fargate**, with frontend hosted as an S3 static website. Designed to run inside an **AWS Academy Learner Lab** account.

## 1. Architecture

```
                 ┌─────────────────────────────────┐
 Browser ──HTTP─►│ S3 static website (frontend)    │
                 └─────────────────────────────────┘
                              │ fetch
                              ▼
                 ┌─────────────────────────────────┐
                 │ Application Load Balancer       │  path-based routing
                 └────┬───────┬───────┬──────┬─────┘
              /auth/* │  /reservations/* │ /files/* │ /notifications/*
                     ▼                  ▼          ▼                ▼
                 ┌──────┐         ┌────────────┐ ┌──────┐    ┌───────────────┐
                 │ auth │         │reservations│ │files │    │ notifications │
                 │ (2x) │         │   (2x)     │ │(2x)  │    │     (2x)      │
                 └──┬───┘         └─────┬──────┘ └──┬───┘    └───────┬───────┘
                    │ Cognito           │ RDS       │ S3 + DynamoDB  │ DynamoDB
                    ▼                   ▼           ▼                ▼
                 Cognito          Postgres       S3 bucket     SNS topic ◄── publishers
              User Pool +          (single        + Dynamo      + Dynamo subscriber
                 JWKS              instance)       table         (HTTPS endpoint)
```

Every Fargate service runs **2 identical replicas** with auto-scaling up to 4 on CPU > 70 %.

## 2. Services and endpoints

8 GET endpoints + 6 POST endpoints across the system (incl. 1 GET + 1 POST for multimedia files).

| Service | Method | Path | Notes |
|---|---|---|---|
| **auth** | POST | `/auth/register` | Cognito `SignUp` + auto-confirm + DynamoDB profile write |
| | POST | `/auth/login` | Cognito `InitiateAuth`, returns `idToken` |
| | GET  | `/auth/me` | profile lookup (JWT required) |
| | GET  | `/auth/health` | liveness probe |
| **reservations** | GET  | `/reservations` | list caller's reservations (RDS Postgres) |
| | GET  | `/reservations/:id` | single reservation |
| | POST | `/reservations` | create + publish `ReservationCreated` to SNS |
| | GET  | `/reservations/health` | liveness probe |
| **files** | GET  | `/files` | list metadata (DynamoDB) |
| | GET  | `/files/:id` | download from S3 |
| | POST | `/files` | upload to S3 + DynamoDB metadata + `FileUploaded` SNS event |
| | GET  | `/files/health` | liveness probe |
| **notifications** | GET  | `/notifications` | history (DynamoDB) |
| | POST | `/notifications` | manual broadcast (publishes to SNS) |
| | POST | `/notifications/sns` | SNS HTTPS subscriber (auto-confirms + persists events) |
| | GET  | `/notifications/health` | liveness probe |

## 3. AWS services used

- **Compute:** ECS Fargate (1 cluster, 4 services, 2 replicas each)
- **Edge / routing:** Application Load Balancer
- **Container registry:** ECR (4 private repos)
- **Auth:** Cognito User Pool + Client (PKCE-less USER_PASSWORD_AUTH flow)
- **Databases:** **RDS PostgreSQL** (reservations) + **DynamoDB** (3 tables: profiles, file metadata, notifications) — **2 different DB services**, satisfying the project requirement
- **Object storage:** S3 (media bucket)
- **Messaging:** SNS topic `conference-app-app-events`, HTTPS subscription to notifications service
- **Logging:** CloudWatch (1 log group per ECS service, retention 3 days)
- **Frontend hosting:** S3 static website

## 4. Repository layout

```
backend/
  go.mod              # single module: neurosciolar/backend
  cmd/<svc>/main.go   # one binary per service
  cmd/<svc>/Dockerfile # multi-stage Go build
  internal/
    sharedauth/        # Cognito JWKS validator + Gin middleware
    awsclients/        # shared SDK config factory
    dynamostore/       # 3 stores: profiles, file_metadata, notifications
    cognitoauth/       # Cognito proxy (register/login/me)
    reservations/      # SQL handler
    files/             # S3 + DynamoDB metadata handler
    notifications/     # SNS publisher + HTTPS subscriber
    rdsdb/             # Postgres connect + schema bootstrap
frontend/
  index.html app.js config.js styles.css   # static site, no nginx
infrastructure/
  providers.tf variables.tf outputs.tf iam.tf
  network.tf alb.tf ecs.tf ecr.tf cloudwatch.tf
  rds.tf dynamodb.tf cognito.tf sns.tf s3.tf
  frontend.tf templates/config.js.tftpl
scripts/
  build-and-push.ps1   # Windows PowerShell
  build-and-push.sh    # bash
```

## 5. Deploying to AWS Academy Learner Lab

### Prerequisites

- Active Learner Lab session (`aws sts get-caller-identity` returns the lab account)
- `terraform`, `docker`, `aws` CLI, `go 1.24+` installed
- Region: **us-east-1** (locked)

### Step 1: Provision base infrastructure

```pwsh
cd infrastructure
terraform init
terraform apply -auto-approve
```

This creates the ECR repos, Cognito, RDS, DynamoDB tables, ALB, ECS cluster, S3 buckets, and SNS topic. **ECS services boot but tasks will fail until images are pushed.**

### Step 2: Build & push container images

```pwsh
cd ..
./scripts/build-and-push.ps1
```

(Linux/macOS: `./scripts/build-and-push.sh`.) The script logs into ECR, builds 4 multi-stage images, and pushes them as `:latest`.

### Step 3: Force ECS to pull fresh images

```pwsh
$cluster = "conference-app-cluster"
foreach ($svc in "auth","reservations","files","notifications") {
  aws ecs update-service --cluster $cluster --service "conference-app-$svc" --force-new-deployment --region us-east-1 | Out-Null
}
```

### Step 4: Verify

```pwsh
terraform output -raw alb_dns_name
terraform output -raw frontend_url
aws ecs describe-services --cluster conference-app-cluster `
  --services conference-app-auth conference-app-reservations conference-app-files conference-app-notifications `
  --query 'services[].[serviceName,runningCount,desiredCount]' --region us-east-1
```

Open the `frontend_url`, register a user, log in, create a reservation, upload a file, then refresh the Notifications panel.

## 6. Local development

Each service can be run locally against the deployed AWS resources by exporting its env vars (see [Environment variables](#7-environment-variables-per-service)) and running:

```pwsh
go run ./backend/cmd/auth
go run ./backend/cmd/reservations
go run ./backend/cmd/files
go run ./backend/cmd/notifications
```

## 7. Environment variables per service

| Service | Required env vars |
|---|---|
| **auth** | `AWS_REGION`, `COGNITO_USER_POOL_ID`, `COGNITO_CLIENT_ID`, `DYNAMO_PROFILES_TABLE` |
| **reservations** | `AWS_REGION`, `COGNITO_USER_POOL_ID`, `COGNITO_CLIENT_ID`, `DB_HOST`, `DB_PORT`, `DB_USER`, `DB_PASSWORD`, `DB_NAME`, `DB_SSLMODE`, `SNS_TOPIC_ARN` |
| **files** | `AWS_REGION`, `COGNITO_USER_POOL_ID`, `COGNITO_CLIENT_ID`, `S3_MEDIA_BUCKET`, `DYNAMO_FILES_TABLE`, `SNS_TOPIC_ARN` |
| **notifications** | `AWS_REGION`, `COGNITO_USER_POOL_ID`, `COGNITO_CLIENT_ID`, `DYNAMO_NOTIFICATIONS_TABLE`, `SNS_TOPIC_ARN` |

All services also accept `PORT` (default `8080`) and `GIN_MODE` (default `release`).

## 8. Learner Lab constraints honored

- **No custom IAM roles created.** All Fargate task execution and task roles are the pre-existing `LabRole` (resolved via `data "aws_iam_role"`).
- Region locked to `us-east-1` via variable validation.
- All resources fit on free-tier / pay-per-request pricing (DynamoDB on-demand, single `db.t3.micro` RDS, Fargate at 0.25 vCPU / 512 MiB per task).
- Default VPC; no NAT gateway (tasks use public IPs to reach AWS APIs).
- Terraform state stored locally (Learner Lab sessions are ephemeral).

## 9. Cleanup

```pwsh
cd infrastructure
terraform destroy -auto-approve
```
