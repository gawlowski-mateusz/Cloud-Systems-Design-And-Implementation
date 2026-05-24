# Report — Project 2 (microservices on AWS Fargate)

## 1. Application topic

Conference room reservation system with multimedia file handling. The existing Project 1 codebase was refactored into 4 independent microservices, each deployed as an ECS Fargate service inside an AWS Academy Learner Lab account.

## 2. Architecture overview

```text
Browser
   │
   ▼
┌────────────────────────────┐
│ S3 static website (front)  │
└────────────────────────────┘
   │ fetch(API_BASE_URL = ALB DNS)
   ▼
┌────────────────────────────┐
│   Application Load Balancer│
└──┬──────┬──────┬──────┬────┘
 /auth/* /reservations/* /files/* /notifications/*
   ▼      ▼      ▼      ▼
 ┌────┐ ┌────┐ ┌────┐ ┌─────────┐
 │auth│ │res.│ │file│ │notifs.  │  Fargate (each: 2..4 replicas, CPU 70 % target)
 └─┬──┘ └─┬──┘ └─┬──┘ └────┬────┘
   │      │      │         │
Cognito   RDS    S3 + Dyn  Dynamo + SNS(http subscriber)
              ▲              ▲
              └──── SNS ◄────┘ (reservations + files publish events)
```

## 3. Microservice modules

### 3.1 Auth service (AWS Cognito)

- Endpoints: `POST /auth/register`, `POST /auth/login`, `GET /auth/me`, `GET /auth/health`.
- Cognito User Pool + Client (`USER_PASSWORD_AUTH` flow). Auto-confirm via `AdminConfirmSignUp` so no email confirmation is required in the lab.
- A copy of the user profile (`fullName`, `email`) is stored in DynamoDB table `conference-app-auth-profiles` (PK `user_sub`).
- Login returns the Cognito `idToken` which all other services validate via the JWKS endpoint of the user pool.

### 3.2 Reservations service (main business logic)

- Endpoints: `GET /reservations`, `GET /reservations/:id`, `POST /reservations`, `GET /reservations/health`.
- Uses **Amazon RDS for PostgreSQL** (`db.t3.micro`, single instance). Schema (one table `reservations` keyed on `user_sub`) is bootstrapped automatically on startup.
- Conflict detection prevents overlapping bookings of the same hall.
- On successful create, publishes a `ReservationCreated` event to the SNS topic.

### 3.3 Files service (multimedia)

- Endpoints: `GET /files`, `GET /files/:id`, `POST /files`, `GET /files/health`.
- Files stored in S3 bucket `conference-app-media-<account>` under `users/<sub>/<file-id>-<safe-name>`.
- Metadata in **DynamoDB** table `conference-app-file-metadata` (PK `user_sub`, SK `file_id`).
- On upload publishes `FileUploaded` event to SNS.

### 3.4 Notifications service (AWS SNS)

- Endpoints: `GET /notifications`, `POST /notifications`, `POST /notifications/sns`, `GET /notifications/health`.
- The `POST /notifications/sns` endpoint is a public **SNS HTTPS subscriber** — it auto-confirms the subscription and persists every incoming `Notification` envelope into DynamoDB table `conference-app-notification-history` (PK `user_sub`, SK `event_ts`).
- `GET /notifications` returns the caller's history merged with `*` (broadcast) entries.
- `POST /notifications` lets an authenticated user publish a manual broadcast (admin/test path).

## 4. Endpoint inventory

8 GET + 6 POST in total — well above the 4 + 4 minimum, including 1 GET + 1 POST for multimedia files.

| Method | Path | Service |
|---|---|---|
| GET | /auth/me | auth |
| GET | /auth/health | auth |
| GET | /reservations | reservations |
| GET | /reservations/:id | reservations |
| GET | /reservations/health | reservations |
| GET | /files | files |
| GET | /files/:id | files (media GET) |
| GET | /files/health | files |
| GET | /notifications | notifications |
| GET | /notifications/health | notifications |
| POST | /auth/register | auth |
| POST | /auth/login | auth |
| POST | /reservations | reservations |
| POST | /files | files (media POST) |
| POST | /notifications | notifications |
| POST | /notifications/sns | notifications (SNS subscriber) |

## 5. Auto-scaling & redundancy

- Every ECS service: `desired_count = 2`, Application Auto Scaling target tracking on `ECSServiceAverageCPUUtilization = 70 %`, `min = 2`, `max = 4`.
- This guarantees the required redundancy of two identical replicas per module and lets the system scale out under load.

## 6. Database services (≥ 2 AWS DB services)

- **Amazon RDS for PostgreSQL** — reservations service.
- **Amazon DynamoDB** — three independent tables for auth profiles, file metadata, and notification history (each on `PAY_PER_REQUEST`).

## 7. Docker

Each service has its own multi-stage Dockerfile at [backend/cmd/<svc>/Dockerfile](backend/cmd/auth/Dockerfile): build stage `golang:1.24-alpine` produces a static binary; runtime stage `alpine:3.21` runs it as non-root user `10001`. Images are pushed to per-service ECR repos by [scripts/build-and-push.ps1](scripts/build-and-push.ps1).

## 8. Terraform layout

All infrastructure is in [infrastructure/](infrastructure/), split by concern:

- [providers.tf](infrastructure/providers.tf), [variables.tf](infrastructure/variables.tf), [outputs.tf](infrastructure/outputs.tf), [iam.tf](infrastructure/iam.tf)
- Compute & routing: [network.tf](infrastructure/network.tf), [alb.tf](infrastructure/alb.tf), [ecs.tf](infrastructure/ecs.tf), [ecr.tf](infrastructure/ecr.tf), [cloudwatch.tf](infrastructure/cloudwatch.tf)
- Data: [rds.tf](infrastructure/rds.tf), [dynamodb.tf](infrastructure/dynamodb.tf), [s3.tf](infrastructure/s3.tf)
- Integration: [cognito.tf](infrastructure/cognito.tf), [sns.tf](infrastructure/sns.tf)
- Frontend: [frontend.tf](infrastructure/frontend.tf) + [templates/config.js.tftpl](infrastructure/templates/config.js.tftpl)

## 9. AWS Academy Learner Lab compatibility

- The lab only allows the pre-existing `LabRole`. Both ECS task execution and task role on every task definition point at `data.aws_iam_role.lab_role.arn`. **No `aws_iam_role` resources are declared anywhere in the Terraform.**
- The region is locked to `us-east-1` via a variable validation in [variables.tf](infrastructure/variables.tf).
- Default VPC and its public subnets are used; no NAT gateway is required because tasks have public IPs.
- All Fargate tasks use the smallest tier (`cpu = 256`, `memory = 512`) and DynamoDB tables are on-demand, keeping the lab budget comfortably within ~$5/day even with 8 running tasks.

## 10. Verification flow

After `terraform apply` and image push:

1. `terraform output -raw frontend_url` → open in browser.
2. Register a user (Cognito SignUp + auto-confirm + DynamoDB profile write).
3. Log in → Cognito returns ID token stored in `localStorage`.
4. Create a reservation → ALB routes to reservations service, row inserted in Postgres, `ReservationCreated` published to SNS, notifications service receives it via HTTPS subscription and writes it to DynamoDB.
5. Upload a file → ALB routes to files service, bytes go to S3, metadata to DynamoDB, `FileUploaded` published to SNS, notifications service stores the event.
6. Refresh the Notifications panel — both events appear within ~5 seconds.
7. `aws ecs describe-services` confirms `runningCount = 2` for every service.
