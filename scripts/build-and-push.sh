#!/usr/bin/env bash
# Build and push the Fargate service images to ECR (bash, Linux/macOS/Git Bash).
# reservations is gone — it now runs as Lambda (see scripts/build-lambdas.sh).
set -euo pipefail

PROJECT_NAME="${PROJECT_NAME:-conference-app}"
REGION="${REGION:-us-east-1}"
TAG="${TAG:-latest}"
SERVICES=(auth files notifications)

echo "Resolving AWS account id..."
ACCOUNT="$(aws sts get-caller-identity --query Account --output text | tr -d '[:space:]')"
if [ -z "$ACCOUNT" ]; then
  echo "Failed to resolve AWS account id. Export Learner Lab credentials first." >&2
  exit 1
fi
REGISTRY="${ACCOUNT}.dkr.ecr.${REGION}.amazonaws.com"
echo "Account: $ACCOUNT, registry: $REGISTRY"

echo "Logging in to ECR..."
aws ecr get-login-password --region "$REGION" | docker login --username AWS --password-stdin "$REGISTRY"

cd "$(dirname "$0")/../backend"

for svc in "${SERVICES[@]}"; do
  REPO="${PROJECT_NAME}/${svc}"
  IMAGE="${REGISTRY}/${REPO}:${TAG}"
  echo "==> Building ${svc} -> ${IMAGE}"
  docker build -f "cmd/${svc}/Dockerfile" -t "${REPO}:${TAG}" -t "${IMAGE}" .
  echo "==> Pushing ${IMAGE}"
  docker push "${IMAGE}"
done

echo ""
echo "Done. To roll out images to existing ECS services:"
for svc in "${SERVICES[@]}"; do
  printf "  aws ecs update-service --cluster %s-cluster --service %s-%s --force-new-deployment --region %s\n" \
    "$PROJECT_NAME" "$PROJECT_NAME" "$svc" "$REGION"
done
