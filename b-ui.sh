#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
TARGET_REL="src/services/runtime/b-ui.sh"
TARGET_PATH="${SCRIPT_DIR}/${TARGET_REL}"
REPO_OWNER="${REPO_OWNER:-BeanYa}"
REPO_NAME="${REPO_NAME:-b-ui}"

if [[ -f "${TARGET_PATH}" ]]; then
    exec bash "${TARGET_PATH}" "$@"
fi

exec bash <(curl -Ls "https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/${TARGET_REL}") "$@"
