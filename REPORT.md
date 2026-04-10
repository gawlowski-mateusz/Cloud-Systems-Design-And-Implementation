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

Command-based setup for temporary Learner Lab credentials (including session token):

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
4. Learner Lab mode with local authentication (`AUTH_PROVIDER=local`),
5. Cognito and Terraform-managed CloudWatch log groups disabled in current deployment (IAM-limited in Learner Lab).

## 7. Authentication mode

Current deployed mode uses local authentication only.

Deployment variables in Learner Lab:

1. `AUTH_PROVIDER=local`
2. `enable_cognito=false`
3. `manage_cloudwatch_log_groups=false`

User registration/login is handled by the backend local user storage.

Implementation note:

1. Cognito code path exists in backend, but it is not active in the current Learner Lab deployment.

## 8. Deployment and tests

Functional verification includes:

1. registration and login,
2. reservation creation and retrieval,
3. file upload and download,
4. PostgreSQL integration,
5. successful startup of backend/frontend containers.

Result for the current project stage:

1. stable backend: `conference-app-backend-env-v4` (`v6`, env id `e-wupevtiepf`, Green/Ok),
2. stable frontend: `conference-app-frontend-env` (`v5`, env id `e-jgmmph3wzh`, Green/Ok),
3. `/health` and `/ready` return `200`,
4. frontend URL returns `200` after `v5` deployment,
5. current outputs: frontend `3.91.146.55`, backend `54.221.212.68`, RDS `conference-app-postgres.c1e8a80kwpsb.us-east-1.rds.amazonaws.com`.

Communication model in the final stage:

1. frontend uses same-origin API calls (`window.location.origin/api/...`),
2. nginx in frontend container proxies `/api/` to backend,
3. this removed cross-origin browser fetch errors.

Resolved deployment issues:

1. ZIP bundles created with `Compress-Archive` caused unzip errors on EB (replaced with `tar -a`),
2. backend EB deployment failed until bundle included the prebuilt `server` binary expected by `backend/Dockerfile`,
3. frontend deployment failed when ZIP used Windows `\\` separators (fixed by tar-created ZIP),
4. frontend 502 was fixed by updating nginx upstream to the current backend EB CNAME in `us-east-1`,
5. frontend auth 404 was fixed by forwarding full request URI in nginx (`proxy_pass ...$request_uri`),
6. state/account drift after switching accounts was stabilized by using a dedicated Terraform workspace (`learner-lab`).

## 9. Equivalent setup via AWS Console

Equivalent infrastructure can be created manually in AWS Console:

1. RDS,
2. S3,
3. 2x Elastic Beanstalk,
4. backend environment variables for local auth,
5. pre-existing `LabInstanceProfile` in Learner Lab.

Then deploy both modules and confirm endpoint/UI behavior. Cognito and custom CloudWatch setup is optional outside Learner Lab when IAM permissions allow.

## 10. Summary

Project requirements were addressed through:

1. application modules (backend + frontend),
2. Docker configuration,
3. Terraform configuration,
4. authentication and media flows in Learner Lab deployment mode,
5. startup and deployment documentation.

Final project state:

1. working backend and frontend environments on AWS Elastic Beanstalk,
2. working registration/login, reservations, and media features,
3. documented, repeatable deployment process for AWS Learner Lab.
