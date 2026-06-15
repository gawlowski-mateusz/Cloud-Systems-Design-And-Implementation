# Build and push the Fargate service images to ECR (Windows PowerShell).
# reservations is gone — it now runs as Lambda (see scripts/build-lambdas.ps1).
# Usage:
#   cd repo-root
#   ./scripts/build-and-push.ps1            # uses default project name + latest tag
#   ./scripts/build-and-push.ps1 -Tag v3    # custom tag

param(
    [string]$ProjectName = "conference-app",
    [string]$Region = "us-east-1",
    [string]$Tag = "latest",
    [string[]]$Services = @("auth", "files", "notifications")
)

$ErrorActionPreference = "Stop"

Write-Host "Resolving AWS account id..." -ForegroundColor Cyan
$Account = (aws sts get-caller-identity --query Account --output text).Trim()
if ([string]::IsNullOrWhiteSpace($Account)) {
    throw "Failed to resolve AWS account id. Are your Learner Lab credentials exported?"
}
$Registry = "$Account.dkr.ecr.$Region.amazonaws.com"
Write-Host "Account: $Account, registry: $Registry" -ForegroundColor Green

Write-Host "Logging in to ECR..." -ForegroundColor Cyan
aws ecr get-login-password --region $Region | docker login --username AWS --password-stdin $Registry

$repoRoot = (Resolve-Path "$PSScriptRoot/..").Path
Push-Location "$repoRoot/backend"
try {
    foreach ($svc in $Services) {
        $repo = "$ProjectName/$svc"
        $image = "${Registry}/${repo}:${Tag}"
        Write-Host "==> Building $svc -> $image" -ForegroundColor Yellow
        docker build -f "cmd/$svc/Dockerfile" -t "${repo}:${Tag}" -t $image .
        Write-Host "==> Pushing $image" -ForegroundColor Yellow
        docker push $image
    }
}
finally {
    Pop-Location
}

Write-Host ""
Write-Host "Done. To roll out images to existing ECS services, run:" -ForegroundColor Green
foreach ($svc in $Services) {
    Write-Host ("  aws ecs update-service --cluster {0}-cluster --service {0}-{1} --force-new-deployment --region {2}" -f $ProjectName, $svc, $Region)
}
