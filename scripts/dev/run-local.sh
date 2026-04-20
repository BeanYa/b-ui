#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
BINARY_PATH="${BINARY_PATH:-${REPO_ROOT}/build/out/sui}"

bash "${REPO_ROOT}/scripts/build/build-all.sh"

SUI_DB_FOLDER="${SUI_DB_FOLDER:-db}" \
SUI_DEBUG="${SUI_DEBUG:-true}" \
"${BINARY_PATH}" "$@"
