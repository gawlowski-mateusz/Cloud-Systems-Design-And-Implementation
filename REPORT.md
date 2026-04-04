# Report - Project 1

## 1. Application topic

A web application for conference room reservations with media handling was implemented.

## 2. Backend + Frontend as separate modules

1. Backend: API in Go (Gin), `backend/` folder.
2. Frontend: static app (HTML/CSS/JS), `frontend/` folder.
3. Modules are independent and can be hosted separately.

## 3. Endpoints

### GET

1. `GET /health`
2. `GET /api/reservations`
3. `GET /api/media`
4. `GET /api/media/:id`

### POST

1. `POST /api/auth/register`
2. `POST /api/auth/login`
3. `POST /api/reservations`
4. `POST /api/media` (file upload)

The GET/POST file requirement is satisfied by `POST /api/media` and `GET /api/media/:id`.

## 4. Docker

1. `backend/Dockerfile` + `backend/docker-compose.yml`
2. `frontend/Dockerfile` + `frontend/docker-compose.yml`

The requirement for separate Docker configuration for both modules is met.

## 5. AWS CLI

Configuration command:

```bash
aws configure
```

Verification:

```bash
aws sts get-caller-identity
```

## 6. Terraform

The `infrastructure/` folder contains:

1. `main.tf`
2. `variables.tf`
3. `outputs.tf`
4. `terraform.tfvars.example`

Configuration includes:

1. AWS Elastic Beanstalk (frontend and backend separately),
2. RDS PostgreSQL,
3. S3 for files,
4. CloudWatch log groups,
5. AWS Cognito User Pool + App Client.

## 7. Cognito

Backend supports `AUTH_PROVIDER=cognito` mode.

Required variables:

1. `COGNITO_REGION`
2. `COGNITO_CLIENT_ID`

User registration/login is handled by AWS Cognito.

## 8. Deployment and tests

Functional verification includes:

1. registration and login,
2. reservation creation and retrieval,
3. file upload and download,
4. PostgreSQL integration,
5. successful startup of backend/frontend containers.

Result for the current project stage:

1. stable backend: `conference-app-backend-stable-v15` (`v15-stable`, Green/Ok),
2. stable frontend: `conference-app-frontend-env` (`v5-stable`, Green/Ok),
3. `/health` and `/ready` return `200`,
4. frontend URL returns `200` after `v5-stable` deployment.

Resolved deployment issues:

1. ZIP bundles created with `Compress-Archive` caused unzip errors on EB (replaced with `tar -a`),
2. backend startup timeout (added `connect_timeout` and `PingContext` in DB connection),
3. frontend 503 after deployment (added `frontend/.ebignore`, excluded `docker-compose.yml` from EB bundle),
4. Cognito login for newly registered users (`UNCONFIRMED` account requires confirmation).

## 9. Equivalent setup via AWS Console

Equivalent infrastructure can be created manually in AWS Console:

1. RDS,
2. S3,
3. Cognito,
4. 2x Elastic Beanstalk,
5. CloudWatch.

Then deploy both modules and confirm endpoint/UI behavior.

## 10. Summary

Project requirements were addressed through:

1. application modules (backend + frontend),
2. Docker configuration,
3. Terraform configuration,
4. AWS Cognito integration,
5. startup and deployment documentation.

Final project state:

1. working backend and frontend environments on AWS Elastic Beanstalk,
2. working registration/login, reservations, and media features,
3. documented, repeatable deployment process for AWS Learner Lab.
