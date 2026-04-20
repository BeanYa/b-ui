#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
OUTPUT_PATH="${OUTPUT_PATH:-${REPO_ROOT}/build/out/sui}"

# shellcheck source=./_runtime.sh
source "${SCRIPT_DIR}/_runtime.sh"

BUILD_TAGS="$(bui_resolve_build_tags)"
LDFLAGS="$(bui_backend_ldflags)"

if bui_should_use_windows_toolchain; then
  bui_run_powershell_file \
    "${SCRIPT_DIR}/build-backend.ps1" \
    -RepoRoot "$(bui_to_host_path "${REPO_ROOT}")" \
    -OutputPath "$(bui_to_host_path "${OUTPUT_PATH}")" \
    -BuildTags "${BUILD_TAGS}"
  exit 0
fi

mkdir -p "$(dirname "${OUTPUT_PATH}")"

cd "${REPO_ROOT}"
go build -ldflags "${LDFLAGS}" -tags "${BUILD_TAGS}" -o "${OUTPUT_PATH}" ./src/backend/cmd/b-ui
