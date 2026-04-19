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
  bash migrate-to-b-ui.sh --help

This script migrates an existing upstream installation to BeanYa/b-ui
in place, switches the service and management command to b-ui, and then
ensures the installation is updated to the latest published b-ui release
by default.
If a version is provided, migration targets that version instead.
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

args=(--migrate)
if [[ -n "${TARGET_VERSION}" ]]; then
    args+=("${TARGET_VERSION}")
fi

run_install_script() {
    if [[ -f "${SCRIPT_DIR}/install.sh" ]]; then
        bash "${SCRIPT_DIR}/install.sh" "$@"
    else
        bash <(curl -Ls "${INSTALL_SCRIPT_URL}") "$@"
    fi
}

run_install_script "${args[@]}"

if [[ -z "${TARGET_VERSION}" ]]; then
    echo "Migration completed. Ensuring the installation is on the latest published b-ui release..."
    run_install_script --update
fi
