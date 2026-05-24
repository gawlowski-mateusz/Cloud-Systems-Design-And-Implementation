# Deploy from zero — copy-paste runbook

Bring the project up from an empty AWS Academy Learner Lab account. Every block is meant to be pasted into PowerShell on Windows, in the listed order, from the repo root.

## 0. Prerequisites

Installed on the workstation:

- Docker Desktop (running, with Linux containers backend)
- `terraform` ≥ 1.6
- `aws` CLI v2
- `go` ≥ 1.24 (only if you want to build locally for debugging)

An active Learner Lab session is required for every command in this file. When the lab session ends, credentials expire — re-export them following section 1.

## 1. Configure AWS credentials

In the lab portal: **AWS Details → AWS CLI → Show**. Copy the three values and paste into PowerShell:

```pwsh
$env:AWS_ACCESS_KEY_ID     = "ASIA...PASTE..."
$env:AWS_SECRET_ACCESS_KEY = "PASTE..."
$env:AWS_SESSION_TOKEN     = "PASTE..."
$env:AWS_DEFAULT_REGION    = "us-east-1"
```

Verify (should print your lab account id):

```pwsh
aws sts get-caller-identity --query Account --output text
```

## 2. Allow the build script to run in this window

```pwsh
Set-ExecutionPolicy -Scope Process -ExecutionPolicy Bypass -Force
```

## 3. Provision base infrastructure

This creates ECR, Cognito, RDS, DynamoDB, ALB, ECS cluster + services, S3 buckets and the SNS topic. The four Fargate services start but tasks won't reach `running` until step 4 pushes the images. **`enable_sns_subscription` stays `false` here on purpose** — see step 6.

```pwsh
cd infrastructure
terraform init
terraform apply -auto-approve
cd ..
```

## 4. Build and push 4 service images to ECR

```pwsh
./scripts/build-and-push.ps1
```

## 5. Roll out the freshly pushed images

```pwsh
$cluster = "conference-app-cluster"
foreach ($svc in "auth","reservations","files","notifications") {
  aws ecs update-service --cluster $cluster --service "conference-app-$svc" --force-new-deployment --region us-east-1 | Out-Null
}
```

## 6. Wait until every service reports `runningCount = 2`

Re-run this command every ~30 s. Don't continue until all four rows read `2 | 2`. Typical wait: 2–5 minutes for the first image pull.

```pwsh
aws ecs describe-services --cluster conference-app-cluster `
  --services conference-app-auth conference-app-reservations conference-app-files conference-app-notifications `
  --query 'services[].[serviceName,runningCount,desiredCount]' --output table --region us-east-1
```

If a task keeps failing to start, tail the logs:

```pwsh
aws logs tail /ecs/conference-app/auth          --since 5m --region us-east-1
aws logs tail /ecs/conference-app/reservations  --since 5m --region us-east-1
aws logs tail /ecs/conference-app/files         --since 5m --region us-east-1
aws logs tail /ecs/conference-app/notifications --since 5m --region us-east-1
```

## 7. Smoke-test the four services through the ALB

```pwsh
Push-Location infrastructure
$alb = terraform output -raw alb_dns_name
Pop-Location
curl.exe "http://$alb/auth/health"
curl.exe "http://$alb/reservations/health"
curl.exe "http://$alb/files/health"
curl.exe "http://$alb/notifications/health"
```

Each must return `{"service":"<name>","status":"ok"}`.

## 8. Create the SNS HTTP subscription

The notifications service is up, so SNS can now POST the subscription confirmation to `/notifications/sns` and the service auto-confirms it. Setting the flag persistently in `terraform.tfvars` makes future `apply`s keep the subscription in place:

```pwsh
Add-Content -Path infrastructure/terraform.tfvars -Value "`nenable_sns_subscription = true"
cd infrastructure
terraform apply -auto-approve
cd ..
```

## 9. Print the URLs and open the app

```pwsh
Push-Location infrastructure
"Backend ALB : http://$(terraform output -raw alb_dns_name)"
"Frontend    : $(terraform output -raw frontend_url)"
Pop-Location
```

Open the `Frontend` URL in your browser:

1. Register a user (any email + password ≥ 8 chars with at least one upper, lower, and digit).
2. Log in.
3. Create a reservation.
4. Upload a small file.
5. Click **Refresh** on the **Notifications** panel — both `ReservationCreated` and `FileUploaded` should appear within a few seconds.

## 10. (Optional) Redeploy after a code change

Only repeat steps 4 and 5 — Terraform state is unchanged.

```pwsh
./scripts/build-and-push.ps1
$cluster = "conference-app-cluster"
foreach ($svc in "auth","reservations","files","notifications") {
  aws ecs update-service --cluster $cluster --service "conference-app-$svc" --force-new-deployment --region us-east-1 | Out-Null
}
```
