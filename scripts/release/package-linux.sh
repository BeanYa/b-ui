#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
STAGE_DIR="${REPO_ROOT}/build/tmp/package-linux"
PACKAGE_ROOT="${STAGE_DIR}/b-ui"

normalize_archive_arch() {
    case "$1" in
    x86_64 | x64 | amd64) printf 'amd64\n' ;;
    i*86 | x86) printf '386\n' ;;
    armv8* | armv8 | arm64 | aarch64) printf 'arm64\n' ;;
    armv7* | armv7 | arm) printf 'armv7\n' ;;
    armv6* | armv6) printf 'armv6\n' ;;
    armv5* | armv5) printf 'armv5\n' ;;
    s390x) printf 's390x\n' ;;
    *)
        printf 'Unsupported CPU architecture for release packaging: %s\n' "$1" >&2
        exit 1
        ;;
    esac
}

set_linux_target_env() {
    GOOS=linux

    case "${ARCHIVE_ARCH}" in
    amd64 | 386 | arm64 | s390x)
        GOARCH="${ARCHIVE_ARCH}"
        unset GOARM || true
        ;;
    armv7)
        GOARCH=arm
        GOARM=7
        ;;
    armv6)
        GOARCH=arm
        GOARM=6
        ;;
    armv5)
        GOARCH=arm
        GOARM=5
        ;;
    esac

    export GOOS GOARCH
    if [[ -n "${GOARM:-}" ]]; then
        export GOARM
    fi
}

ARCHIVE_ARCH="$(normalize_archive_arch "${ARCHIVE_ARCH:-$(uname -m)}")"
OUTPUT_ARCHIVE="${REPO_ROOT}/dist/release/b-ui-linux-${ARCHIVE_ARCH}.tar.gz"

if ! command -v go >/dev/null 2>&1; then
    printf 'Linux release packaging requires a Go toolchain available in the current shell. Use a Linux runner/WSL environment with Go installed, or the CI release workflow.\n' >&2
    exit 1
fi

bash "${REPO_ROOT}/scripts/build/build-frontend.sh"
set_linux_target_env
bash "${REPO_ROOT}/scripts/build/build-backend.sh"

rm -rf "${STAGE_DIR}"
mkdir -p "${PACKAGE_ROOT}/src/services/runtime" "${PACKAGE_ROOT}/src/services/systemd"

cp "${REPO_ROOT}/build/out/b-ui" "${PACKAGE_ROOT}/b-ui"
cp "${REPO_ROOT}/src/services/runtime/b-ui.sh" "${PACKAGE_ROOT}/src/services/runtime/b-ui.sh"
cp "${REPO_ROOT}/src/services/systemd/b-ui.service" "${PACKAGE_ROOT}/src/services/systemd/b-ui.service"
cp "${REPO_ROOT}/src/services/runtime/b-ui.sh" "${PACKAGE_ROOT}/b-ui.sh"
cp "${REPO_ROOT}/src/services/systemd/b-ui.service" "${PACKAGE_ROOT}/b-ui.service"

tar -czf "${OUTPUT_ARCHIVE}" -C "${STAGE_DIR}" b-ui
printf 'Created %s\n' "${OUTPUT_ARCHIVE}"
