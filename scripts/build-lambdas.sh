#!/usr/bin/env bash
# Cross-compile the reservation Lambdas to infrastructure/build/<fn>/bootstrap
# (provided.al2023 runtime). Run this before `terraform apply`.
set -euo pipefail

FUNCTIONS=("create-reservation" "list-reservations" "get-reservation")
REPO_ROOT="$(cd "$(dirname "$0")/.." && pwd)"
BUILD_DIR="$REPO_ROOT/infrastructure/build"

cd "$REPO_ROOT/backend"
for fn in "${FUNCTIONS[@]}"; do
  out_dir="$BUILD_DIR/$fn"
  mkdir -p "$out_dir"
  echo "==> Building lambda $fn -> $out_dir/bootstrap"
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 \
    go build -trimpath -ldflags="-s -w" -o "$out_dir/bootstrap" "./cmd/lambda/$fn"
done

echo "Done. Now run: cd infrastructure && terraform apply"
