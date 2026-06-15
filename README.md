# Cloud-Systems-Design-And-Implementation

Conference room reservation app. The main business logic (reservations) runs as **AWS Lambda**
behind **API Gateway**; supporting services (auth, files, notifications) run on **AWS Fargate**;
components communicate through an **event-driven Amazon SQS** pipeline. Frontend is an S3 static
website. Designed to run inside an **AWS Academy Learner Lab** account.

## 1. Architecture

```
                 ┌─────────────────────────────────┐
 Browser ──HTTP─►│ S3 static website (frontend)    │
                 └──────┬───────────────────┬───────┘
        /reservations*  │                   │  /auth* /files* /notifications*
                        ▼                   ▼
        ┌──────────────────────────┐   ┌─────────────────────────────────┐
        │ API Gateway (HTTP API)   │   │ Application Load Balancer        │
        │ Cognito JWT authorizer   │   └────┬──────────┬─────────────┬────┘
        └───────────┬──────────────┘    /auth*    /files*     /notifications*
        POST/GET reservations              ▼          ▼               ▼
                    ▼                   ┌──────┐   ┌──────┐    ┌───────────────┐
        ┌──────────────────────┐       │ auth │   │files │    │ notifications │
        │ Lambda x3            │       │ (2x) │   │(2x)  │    │     (2x)      │
        │ create/list/get      │       └──┬───┘   └──┬───┘    └───────┬───────┘
        └─────────┬────────────┘          │Cognito   │S3+Dynamo       │ polls
                  │ DynamoDB               ▼          ▼                ▼
                  ▼                     Cognito    S3 + Dynamo   ┌───────────────┐
            reservations table          User Pool                │  SQS app-events│◄─ producers
                  │                                               │  + DLQ         │   (Lambda, files)
                  └────────── publish event ─────────────────────►└───────────────┘
                                                consumer → DynamoDB (history) + SNS topic → email
```

- **reservations** = 3 Lambda functions (create / list / get) behind an API Gateway HTTP API with a
  native **Cognito JWT authorizer**. Data lives in a DynamoDB table (no RDS).
- Notifications are **event-driven**: producers (the create-reservation Lambda and the files
  service) publish to **SQS**; the notifications Fargate service **long-polls** the queue and
  persists events idempotently. A **Dead Letter Queue** captures messages that keep failing.
- Notifications are **emailed** to the recipient: the notifications service publishes each event to
  an **Amazon SNS** topic that has the user's address subscribed (SES is unavailable in the Learner
  Lab). The subscriber confirms the subscription once via the link SNS emails.
- **CloudWatch** provides per-compute log groups, alarms, and a dashboard.

## 2. Services and endpoints

| Service | Entry | Method | Path | Notes |
|---|---|---|---|---|
| **auth** | ALB | POST | `/auth/register` | Cognito `SignUp` + auto-confirm + DynamoDB profile |
| | ALB | POST | `/auth/login` | Cognito `InitiateAuth`, returns `idToken` |
| | ALB | GET  | `/auth/me` | profile lookup (JWT required) |
| | ALB | GET  | `/auth/health` | liveness probe |
| **reservations** | API GW | POST | `/reservations` | Lambda: create + publish `ReservationCreated` to SQS |
| | API GW | GET  | `/reservations` | Lambda: list caller's reservations |
| | API GW | GET  | `/reservations/{id}` | Lambda: single reservation |
| **files** | ALB | GET  | `/files` | list metadata (DynamoDB) |
| | ALB | GET  | `/files/:id` | download from S3 |
| | ALB | POST | `/files` | upload to S3 + DynamoDB + publish `FileUploaded` to SQS |
| | ALB | GET  | `/files/health` | liveness probe |
| **notifications** | ALB | GET  | `/notifications` | history (DynamoDB) |
| | ALB | GET  | `/notifications/health` | liveness probe |

Reservation routes are protected by the API Gateway JWT authorizer; the other authenticated
routes validate the Cognito id token in-process (`internal/sharedauth`).

## 3. AWS services used

- **Compute:** ECS Fargate (3 services × 2 replicas) **+ AWS Lambda** (3 reservation functions)
- **Edge / routing:** Application Load Balancer (Fargate) + **API Gateway HTTP API** (reservations)
- **Messaging:** **Amazon SQS** queue `conference-app-app-events` **+ Dead Letter Queue**
- **Email:** **Amazon SNS** email subscription — the notifications service publishes each event and
  SNS emails the subscribed address
- **Auth:** Cognito User Pool + Client; API Gateway Cognito JWT authorizer (HTTP API)
- **Databases:** **DynamoDB** (4 tables: profiles, file metadata, notifications, reservations)
- **Object storage:** S3 (media bucket) + S3 (static frontend)
- **Container registry:** ECR (3 private repos)
- **Monitoring:** CloudWatch — separate log groups (`/ecs/*` for Fargate, `/aws/lambda/*` for
  Lambda), metric alarms (Lambda errors/duration, SQS depth, DLQ-not-empty, notification errors),
  and a dashboard (`conference-app-overview`)

## 4. Repository layout

```
backend/
  go.mod                      # single module: neurosciolar/backend
  cmd/<svc>/main.go           # auth, files, notifications (Fargate)
  cmd/<svc>/Dockerfile        # multi-stage Go build
  cmd/lambda/<op>/main.go     # create-/list-/get-reservation (Lambda, builds to `bootstrap`)
  internal/
    sharedauth/               # Cognito JWKS validator + Gin middleware
    awsclients/               # shared SDK config factory
    dynamostore/              # profiles, file_metadata, notifications (idempotent put)
    reservationstore/         # DynamoDB store for reservations (+ hall_date_index GSI)
    events/                   # SQS publisher (shared event contract)
    lambdahttp/               # Lambda JSON/CORS + JWT-claim helpers
    snspub/                   # publish to the notifications SNS topic
    emailnotify/              # format the notification email + publish it
    cognitoauth/  files/  notifications/   # notifications = SQS consumer + email sender
frontend/
  index.html app.js config.js styles.css
infrastructure/
  providers.tf variables.tf outputs.tf iam.tf
  network.tf alb.tf ecs.tf ecr.tf cloudwatch.tf
  lambda.tf apigateway.tf sqs.tf sns.tf dynamodb.tf cognito.tf s3.tf
  frontend.tf templates/config.js.tftpl
scripts/
  build-lambdas.ps1 / .sh     # cross-compile Lambda bootstrap binaries
  build-and-push.ps1 / .sh    # build & push the 3 Fargate images to ECR
```

## 5. Configure your AWS account (Learner Lab)

Learner Lab issues **temporary credentials that expire when the lab session ends** — repeat this
each session.

1. In AWS Academy, start the lab and wait for the dot next to **AWS** to turn green.
2. Click **AWS Details → AWS CLI: Show**. Copy the shown block (it contains
   `aws_access_key_id`, `aws_secret_access_key`, `aws_session_token`).
3. Paste it into your credentials file under a `[default]` profile:
   - Windows: `C:\Users\<you>\.aws\credentials`
   - Linux/macOS: `~/.aws/credentials`
4. Set the region (once): create `~/.aws/config` with
   ```
   [default]
   region = us-east-1
   ```
5. Verify:
   ```pwsh
   aws sts get-caller-identity
   ```
   It should print the lab account id. (`LabRole` already exists in this account — Terraform
   reuses it, it does not create roles.)

Tools required locally: `aws` CLI, `terraform` (≥ 1.6), `docker` (running), `go` (≥ 1.24).

## 6. Deploy

Set `notification_email` in `infrastructure/terraform.tfvars` to the address you log in with. After
`terraform apply`, **confirm the SNS subscription** via the link emailed to that address — until you
do, no notification emails are delivered. (SES is not used: the Learner Lab role cannot verify SES
identities, so SNS email subscriptions are used instead.)

```pwsh
# 1. Compile the Lambda binaries (linux/amd64 -> infrastructure/build/<fn>/bootstrap)
./scripts/build-lambdas.ps1

# 2. Build & push the 3 Fargate images to ECR (auth, files, notifications)
./scripts/build-and-push.ps1

# 3. Provision everything (single apply — no SNS two-phase dance)
cd infrastructure
terraform init
terraform apply -auto-approve

# 4. Roll ECS onto the freshly pushed images
$cluster = "conference-app-cluster"
foreach ($svc in "auth","files","notifications") {
  aws ecs update-service --cluster $cluster --service "conference-app-$svc" --force-new-deployment --region us-east-1 | Out-Null
}
```

> `build-and-push` pushes images **after** ECR repos exist. On a clean account run
> `terraform apply` once to create the repos, then push, then `update-service` (step 4). On
> reruns the order above is fine.

### Verify

```pwsh
cd infrastructure
terraform output -raw frontend_url            # open in a browser
terraform output -raw reservations_api_url    # API Gateway base URL
terraform output -raw sqs_queue_url
```

Open `frontend_url`, register, log in, create a reservation and upload a file, then refresh the
Notifications panel — events flow create-reservation/files → SQS → notifications consumer →
DynamoDB. Once you have confirmed the SNS subscription, each event also arrives as an email. Watch
the `conference-app-overview` CloudWatch dashboard for Lambda/SQS/DLQ metrics.

## 7. Environment variables

| Service | Required env vars |
|---|---|
| **auth** (Fargate) | `AWS_REGION`, `COGNITO_USER_POOL_ID`, `COGNITO_CLIENT_ID`, `DYNAMO_PROFILES_TABLE` |
| **files** (Fargate) | `AWS_REGION`, `COGNITO_*`, `S3_MEDIA_BUCKET`, `DYNAMO_FILES_TABLE`, `SQS_QUEUE_URL` |
| **notifications** (Fargate) | `AWS_REGION`, `COGNITO_*`, `DYNAMO_NOTIFICATIONS_TABLE`, `SQS_QUEUE_URL`, `NOTIFICATIONS_TOPIC_ARN` |
| **reservation Lambdas** | `RESERVATIONS_TABLE`, `SQS_QUEUE_URL` (auth handled by API Gateway authorizer) |

Fargate services also accept `PORT` (default `8080`) and `GIN_MODE` (default `release`). All env
values are wired by Terraform (`ecs.tf`, `lambda.tf`).

## 8. Learner Lab constraints honored

- **No custom IAM roles created** — Fargate tasks and Lambda functions all use the pre-existing
  `LabRole` (resolved via `data "aws_iam_role"`).
- Region locked to `us-east-1` via variable validation.
- Serverless-first: reservations use Lambda + DynamoDB (no VPC attachment, no connection pool).
- Default VPC, no NAT gateway (Fargate tasks use public IPs; Lambda reaches AWS APIs over the
  public endpoints).
- Pay-per-request DynamoDB, on-demand SQS, Fargate at 0.25 vCPU / 512 MiB; Terraform state local.

## 9. Cleanup

```pwsh
cd infrastructure
terraform destroy -auto-approve
```
