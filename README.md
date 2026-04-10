# Cloud-Systems-Design-And-Implementation

Project: a web application for conference room reservations with media file support.

## 1. Functional scope

### Backend API (Go + Gin)

The required endpoints are implemented:

1. `GET /health` - API and DB connection status
2. `GET /ready` - backend readiness and DB connection status
3. `GET /api/reservations` - reservation list for the authenticated user
4. `GET /api/media` - media file list for the authenticated user
5. `GET /api/media/:id` - download media file
6. `POST /api/auth/register` - user registration
7. `POST /api/auth/login` - user login
8. `POST /api/reservations` - create reservation
9. `POST /api/media` - media file upload (`multipart/form-data`, `file` field)

This satisfies the minimum requirement of 2x GET and 2x POST, including GET/POST for files.

### Frontend module

The frontend works as an independent static application:

1. registration and login,
2. reservation creation and listing,
3. file upload,
4. file listing and download.

The backend URL is configurable in `frontend/config.js`:

```js
window.APP_CONFIG = {
  API_BASE_URL: window.location.origin
};
```

In the current production setup, frontend calls `/api/...` on the same origin and nginx proxies those requests to the backend Elastic Beanstalk endpoint.

For local development, `API_BASE_URL` can be temporarily set to `http://localhost:8080`.

## 2. Project structure

1. `backend/` - independent API module
2. `frontend/` - independent UI module
3. `infrastructure/` - Terraform for AWS

## 3. Docker

### Backend

Files:

1. `backend/Dockerfile`
2. `backend/docker-compose.yml`
3. `backend/.ebignore`

Run:

```bash
cd backend
docker compose up -d --build
```

This starts:

1. API (`localhost:8080`)
2. PostgreSQL (`localhost:5432`)

For Elastic Beanstalk, build the package from `backend/` without local `docker-compose.yml`. The `backend/.ebignore` file excludes it from the package, so EB uses `backend/Dockerfile` instead of compose mode with local Postgres.

### Frontend

Files:

1. `frontend/Dockerfile`
2. `frontend/nginx.conf`
3. `frontend/docker-compose.yml`

Run:

```bash
cd frontend
docker compose up -d --build
```

Frontend is available at `http://localhost:3000`.

## 4. AWS CLI configuration

In Learner Lab, values in `AWS Details` change dynamically. Configure CLI after each lab start:

```bash
aws configure
```

Provide:

1. Access Key ID
2. Secret Access Key
3. Region (for this project: `us-east-1`)
4. Output format (`json`)

If you prefer command-based setup (including temporary session token):

```bash
aws configure set aws_access_key_id <ACCESS_KEY_ID>
aws configure set aws_secret_access_key <SECRET_ACCESS_KEY>
aws configure set aws_session_token <SESSION_TOKEN>
aws configure set region us-east-1
aws configure set output json
```

Verification:

```bash
aws sts get-caller-identity
```

## 5. Terraform

### Contents

The `infrastructure/` folder contains:

1. `main.tf`
2. `variables.tf`
3. `outputs.tf`
4. `terraform.tfvars.example`

Configuration includes:

1. two independent AWS Elastic Beanstalk environments (frontend and backend),
2. RDS PostgreSQL,
3. S3 for media files,
4. Learner Lab mode with local authentication (`AUTH_PROVIDER=local`),
5. no Cognito and no Terraform-managed CloudWatch log groups (IAM-limited in Learner Lab).

### Running Terraform

```bash
cd infrastructure
cp terraform.tfvars.example terraform.tfvars
terraform init
terraform plan
terraform apply
```

### Learner Lab notes

With the currently used Learner Lab role in this project:

1. `ec2:DescribeVpcs` works in `us-east-1` and fails in `eu-central-1`,
2. Cognito read APIs are denied,
3. CloudWatch Logs read APIs are denied.

For Learner Lab use the following values in `infrastructure/terraform.tfvars`:

1. `aws_region = "us-east-1"`
2. `enable_cognito = false`
3. `manage_cloudwatch_log_groups = false`

These values are now enforced by Terraform variable validation, so this setup is intentionally Learner-Lab-only.

If you switch AWS account, use a separate Terraform workspace (or separate state file) so Terraform does not try to refresh resources from the previous account:

```bash
terraform workspace new learner-lab
terraform init -reconfigure
```

## 6. Authentication in Learner Lab

In this Learner Lab setup, backend uses local authentication only:

1. `local` - local account + password,
2. `cognito` is disabled by infrastructure policy constraints.

Backend environment is configured by Terraform with:

1. `AUTH_PROVIDER=local`

Practical notes:

1. registration/login is handled by local backend account storage,
2. no Cognito resources are created in Learner Lab mode.

## 7. Deployment to AWS Elastic Beanstalk

Example flow:

1. Build ZIP packages for backend and frontend.
2. Upload them to the versions bucket (`aws_s3_bucket.eb_versions`).
3. Set ZIP keys in `terraform.tfvars`:
   - `backend_source_bundle_key`
   - `frontend_source_bundle_key`
4. Run `terraform apply`.
5. Read URLs from `terraform output`.

Verified ZIP packaging method (Windows + EB):

1. Do not use `Compress-Archive` for EB bundles (it may cause `\\` path separator unzip errors on instance).
2. Use `tar -a -c -f ...`.
3. Backend ZIP should include `Dockerfile`, prebuilt `server` binary and `.ebignore` (and should not include `docker-compose.yml`).
4. Frontend ZIP should include `Dockerfile`, `nginx.conf`, static files, and should not include local compose files.

Current stable deployment state:

1. Backend EB: `conference-app-backend-env-v4` (`v6`, env id `e-wupevtiepf`, health Green/Ok),
2. Frontend EB: `conference-app-frontend-env` (`v5`, env id `e-jgmmph3wzh`, health Green/Ok),
3. region: `us-east-1`.

Current public endpoints:

1. Frontend: `http://3.91.146.55` (CNAME: `conference-app-frontend-env.eba-pp32weaw.us-east-1.elasticbeanstalk.com`)
2. Backend direct: `http://54.221.212.68` (CNAME: `conference-app-backend-env-v4.eba-9m87vpny.us-east-1.elasticbeanstalk.com`)
3. Backend via frontend proxy: `http://3.91.146.55/api/...`

## 8. Verification

After deployment, check:

1. backend `GET /health` returns `200`,
2. backend `GET /ready` returns `200` and `db=connected`,
3. frontend URL returns `200` (no 503),
4. registration/login (local auth),
5. reservation creation and listing,
6. file upload and download,
7. writes to RDS,
8. media objects in S3.

## 9. Equivalent setup in AWS Console

To satisfy the requirement for AWS web console configuration:

1. manually create RDS PostgreSQL,
2. create S3 bucket,
3. create 2 Elastic Beanstalk applications,
4. configure backend env vars identically to Terraform (`AUTH_PROVIDER=local` in Learner Lab mode),
5. deploy frontend and backend as separate deployments,
6. use pre-existing `LabInstanceProfile` for EC2 instances,
7. treat Cognito and custom CloudWatch logs as optional outside Learner Lab (when IAM permissions allow).

## 10. Report

Summary report is in `REPORT.md`.
