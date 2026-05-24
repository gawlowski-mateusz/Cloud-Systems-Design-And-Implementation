# Tear down — fully delete every AWS resource

The Learner Lab budget is small. When you stop working on the project, run this runbook end-to-end to remove every billable resource (Fargate tasks, RDS, ALB, NAT-equivalent traffic, DynamoDB, S3, Cognito, SNS, ECR, CloudWatch log groups).

Run all commands from the **repo root** in a PowerShell window with active Learner Lab credentials (see [`DEPLOY.md`](DEPLOY.md) section 1).

## 1. Empty the two S3 buckets

`terraform destroy` refuses to delete non-empty buckets. Clear them first (also wipes versioned objects on the media bucket):

```pwsh
Push-Location infrastructure
$media          = terraform output -raw media_bucket
$frontendUrl    = terraform output -raw frontend_url
$frontendBucket = ($frontendUrl -replace "http://","" -split "\.")[0]
Pop-Location

# Frontend bucket (no versioning, simple recursive delete).
aws s3 rm "s3://$frontendBucket" --recursive --region us-east-1

# Media bucket (versioned — delete all versions and delete-markers, then bucket can go).
aws s3 rm "s3://$media" --recursive --region us-east-1
$versions = aws s3api list-object-versions --bucket $media --region us-east-1 `
  --query '{Objects: Versions[].{Key:Key,VersionId:VersionId}}' --output json
if ($versions -and $versions -ne "null" -and $versions -notmatch '"Objects": null') {
  aws s3api delete-objects --bucket $media --region us-east-1 --delete $versions | Out-Null
}
$markers = aws s3api list-object-versions --bucket $media --region us-east-1 `
  --query '{Objects: DeleteMarkers[].{Key:Key,VersionId:VersionId}}' --output json
if ($markers -and $markers -ne "null" -and $markers -notmatch '"Objects": null') {
  aws s3api delete-objects --bucket $media --region us-east-1 --delete $markers | Out-Null
}
```

## 2. Scale ECS services to zero and clear ECR images

Scaling tasks to zero immediately stops Fargate billing. Emptying ECR makes the destroy cleaner (Terraform's `force_delete = true` would also handle it, but this avoids edge cases when the lab session is short on time):

```pwsh
$cluster = "conference-app-cluster"
foreach ($svc in "auth","reservations","files","notifications") {
  aws ecs update-service --cluster $cluster --service "conference-app-$svc" --desired-count 0 --region us-east-1 | Out-Null

  $repo = "conference-app/$svc"
  $imageIds = aws ecr list-images --repository-name $repo --region us-east-1 --query 'imageIds[*]' --output json
  if ($imageIds -and $imageIds -ne "[]") {
    aws ecr batch-delete-image --repository-name $repo --region us-east-1 --image-ids $imageIds | Out-Null
  }
}
```

## 3. Destroy the rest with Terraform

```pwsh
cd infrastructure
terraform destroy -auto-approve
cd ..
```

Expected duration:

- ECS services + tasks: ~2 minutes
- Application Load Balancer + target groups + listener: ~2 minutes
- RDS instance: ~5 minutes (longest single step)
- Everything else: seconds

## 4. Verify nothing remains

Each of these queries should return an empty list. If any prints a name, run `terraform destroy -auto-approve` again or delete it manually in the AWS console.

```pwsh
aws ecs list-clusters             --region us-east-1 --query 'clusterArns[?contains(@, `conference-app`)]'
aws elbv2 describe-load-balancers --region us-east-1 --query 'LoadBalancers[?contains(LoadBalancerName, `conference-app`)].LoadBalancerName'
aws rds describe-db-instances     --region us-east-1 --query 'DBInstances[?contains(DBInstanceIdentifier, `conference-app`)].DBInstanceIdentifier'
aws dynamodb list-tables          --region us-east-1 --query 'TableNames[?contains(@, `conference-app`)]'
aws s3 ls                         --region us-east-1 | Select-String "conference-app"
aws ecr describe-repositories     --region us-east-1 --query 'repositories[?contains(repositoryName, `conference-app`)].repositoryName'
aws cognito-idp list-user-pools   --region us-east-1 --max-results 60 --query 'UserPools[?contains(Name, `conference-app`)].Id'
aws sns list-topics               --region us-east-1 --query 'Topics[?contains(TopicArn, `conference-app`)].TopicArn'
aws logs describe-log-groups      --region us-east-1 --log-group-name-prefix /ecs/conference-app --query 'logGroups[].logGroupName'
```

## 5. Optional — emergency stop without Terraform

If Terraform state is lost or corrupted (e.g. lab session ended mid-apply), here is the minimum to halt all billing manually. Order matters because ALB → target groups → services → tasks have a dependency chain:

```pwsh
# 5.1 Drop Fargate task counts to 0 (stops the most expensive resource).
$cluster = "conference-app-cluster"
foreach ($svc in "auth","reservations","files","notifications") {
  aws ecs update-service --cluster $cluster --service "conference-app-$svc" --desired-count 0 --region us-east-1 | Out-Null
}

# 5.2 Delete RDS without final snapshot.
aws rds delete-db-instance --db-instance-identifier conference-app-postgres `
  --skip-final-snapshot --delete-automated-backups --region us-east-1

# 5.3 Delete ALB (also removes listeners and rules implicitly).
$albArn = aws elbv2 describe-load-balancers --names conference-app-alb --region us-east-1 --query 'LoadBalancers[0].LoadBalancerArn' --output text
aws elbv2 delete-load-balancer --load-balancer-arn $albArn --region us-east-1
```

After 5–10 minutes those three commands alone bring the recurring cost down to near zero. DynamoDB (on-demand, no items), S3 (a few KB), and CloudWatch log groups are negligible but can be cleaned up later from the console.

## 6. Sign out of the Learner Lab

In the lab portal, click **End Lab**. This snapshots the lab account and prevents any leftover resources from billing further. Note: ending the lab does **not** delete resources — you must run the destroy steps above first if you want a clean account on the next session.
