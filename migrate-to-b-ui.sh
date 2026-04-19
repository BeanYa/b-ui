#!/bin/bash

set -e

REPO_OWNER="${REPO_OWNER:-BeanYa}"
REPO_NAME="${REPO_NAME:-b-ui}"
INSTALL_SCRIPT_URL="https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/install.sh"

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
TARGET_VERSION=""

show_usage() {
    cat <<'EOF'
Usage:
  bash migrate-to-b-ui.sh
  bash migrate-to-b-ui.sh <version>

This script migrates an existing s-ui installation to the BeanYa/b-ui fork
in place while keeping the current service name, install path, and database.
EOF
}

for arg in "$@"; do
    case "$arg" in
    -h | --help)
        show_usage
        exit 0
        ;;
    *)
        if [[ -z "${TARGET_VERSION}" ]]; then
            TARGET_VERSION="$arg"
        else
            echo "Too many arguments: $*"
            exit 1
        fi
        ;;
    esac
done

args=(--auto-migrate)
if [[ -n "${TARGET_VERSION}" ]]; then
    args+=("${TARGET_VERSION}")
fi

if [[ -f "${SCRIPT_DIR}/install.sh" ]]; then
    bash "${SCRIPT_DIR}/install.sh" "${args[@]}"
else
    bash <(curl -Ls "${INSTALL_SCRIPT_URL}") "${args[@]}"
fi
