#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
FRONTEND_ARTIFACT_DIR="${REPO_ROOT}/build/tmp/frontend_dist"
BACKEND_WEB_DIR="${REPO_ROOT}/src/backend/internal/infra/web/html"
LOG_FILE="${REPO_ROOT}/docker-build-test.log"
PLATFORMS="${PLATFORMS:-linux/amd64,linux/386,linux/arm64/v8,linux/arm/v7,linux/arm/v6}"

printf '==> Building frontend assets...\n'
bash "${SCRIPT_DIR}/build-frontend.sh"

rm -rf "${FRONTEND_ARTIFACT_DIR}"
mkdir -p "${FRONTEND_ARTIFACT_DIR}"
cp -R "${BACKEND_WEB_DIR}/." "${FRONTEND_ARTIFACT_DIR}/"

printf '==> Testing Docker build for: %s\n' "${PLATFORMS}"
docker buildx build \
  --platform "${PLATFORMS}" \
  -f "${REPO_ROOT}/packaging/docker/Dockerfile.frontend-artifact" \
  --build-arg CRONET_RELEASE=latest \
  --progress=plain \
  "${REPO_ROOT}" 2>&1 | tee "${LOG_FILE}"

printf '==> Done. Check %s for full output.\n' "${LOG_FILE}"
