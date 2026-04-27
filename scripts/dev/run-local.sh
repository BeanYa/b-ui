#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
BINARY_PATH="${BINARY_PATH:-${REPO_ROOT}/build/out/b-ui}"

bash "${REPO_ROOT}/scripts/build/build-all.sh"

BUI_DB_FOLDER="${BUI_DB_FOLDER:-db}" \
BUI_DEBUG="${BUI_DEBUG:-true}" \
"${BINARY_PATH}" "$@"
