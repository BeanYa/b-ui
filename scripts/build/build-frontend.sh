#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
FRONTEND_DIR="${REPO_ROOT}/src/frontend"
FRONTEND_DIST_DIR="${FRONTEND_DIR}/dist"
BACKEND_WEB_DIR="${REPO_ROOT}/src/backend/internal/infra/web/html"

# shellcheck source=./_runtime.sh
source "${SCRIPT_DIR}/_runtime.sh"

if bui_should_use_windows_toolchain; then
  bui_run_powershell_file \
    "${SCRIPT_DIR}/build-frontend.ps1" \
    -RepoRoot "$(bui_to_host_path "${REPO_ROOT}")" \
    -FrontendDir "$(bui_to_host_path "${FRONTEND_DIR}")" \
    -FrontendDistDir "$(bui_to_host_path "${FRONTEND_DIST_DIR}")" \
    -BackendWebDir "$(bui_to_host_path "${BACKEND_WEB_DIR}")"
  exit 0
fi

cd "${FRONTEND_DIR}"
npm ci --include=dev
npm run build:dist

rm -rf "${BACKEND_WEB_DIR}"
mkdir -p "${BACKEND_WEB_DIR}"
cp -R "${FRONTEND_DIST_DIR}/." "${BACKEND_WEB_DIR}/"
