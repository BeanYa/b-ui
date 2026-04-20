#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)

# shellcheck source=../build/_runtime.sh
source "${REPO_ROOT}/scripts/build/_runtime.sh"

bash "${REPO_ROOT}/scripts/ci/check-layout.sh"

if bui_should_use_windows_toolchain; then
  bui_run_powershell_file \
    "${SCRIPT_DIR}/verify-go.ps1" \
    -RepoRoot "$(bui_to_host_path "${REPO_ROOT}")"
else
  (
    cd "${REPO_ROOT}"
    go test ./...
  )
fi

bash "${REPO_ROOT}/scripts/build/build-frontend.sh"
bash "${REPO_ROOT}/scripts/build/build-backend.sh"
