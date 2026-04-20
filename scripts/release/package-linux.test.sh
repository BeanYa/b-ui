#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
TEST_ROOT="${REPO_ROOT}/build/tmp/package-linux-test"
MOCK_BIN="${TEST_ROOT}/mock-bin"
PW_LOG_FILE="${TEST_ROOT}/pwsh.log"
GO_LOG_FILE="${TEST_ROOT}/go.log"
ARCHIVE_PATH="${REPO_ROOT}/dist/release/b-ui-linux-amd64.tar.gz"

fail() {
    printf 'FAIL: %s\n' "$1" >&2
    exit 1
}

assert_file_exists() {
    local path="$1"

    [[ -f "${path}" ]] || fail "expected file ${path}"
}

assert_contains() {
    local needle="$1"
    local haystack="$2"
    local message="$3"

    [[ "${haystack}" == *"${needle}"* ]] || fail "${message}: missing '${needle}'"
}

assert_not_contains() {
    local needle="$1"
    local haystack="$2"
    local message="$3"

    [[ "${haystack}" != *"${needle}"* ]] || fail "${message}: found '${needle}'"
}

rm -rf "${TEST_ROOT}" "${REPO_ROOT}/build/tmp/package-linux" "${ARCHIVE_PATH}"
mkdir -p "${MOCK_BIN}"

cat > "${MOCK_BIN}/pwsh.exe" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${PW_LOG_FILE}"
EOF

cat > "${MOCK_BIN}/wslpath" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
path="${@: -1}"
printf 'C:\\mock\\%s\n' "${path##*/}"
EOF

cat > "${MOCK_BIN}/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

printf 'GOOS=%s GOARCH=%s GOARM=%s ARGS=%s\n' "${GOOS:-}" "${GOARCH:-}" "${GOARM:-}" "$*" >> "${GO_LOG_FILE}"

[[ "${GOOS:-}" == 'linux' ]] || {
  printf 'expected GOOS=linux, got %s\n' "${GOOS:-}" >&2
  exit 1
}

[[ "${GOARCH:-}" == 'amd64' ]] || {
  printf 'expected GOARCH=amd64, got %s\n' "${GOARCH:-}" >&2
  exit 1
}

output_path=''
previous=''
for arg in "$@"; do
  if [[ "${previous}" == '-o' ]]; then
    output_path="${arg}"
    break
  fi

  previous="${arg}"
done

[[ -n "${output_path}" ]] || {
  printf 'missing -o output path\n' >&2
  exit 1
}

mkdir -p "$(dirname "${output_path}")"
printf '\177ELFmock\n' > "${output_path}"
EOF

chmod +x "${MOCK_BIN}/pwsh.exe" "${MOCK_BIN}/wslpath" "${MOCK_BIN}/go"

export PW_LOG_FILE GO_LOG_FILE
export PATH="${MOCK_BIN}:${PATH}"

ARCHIVE_ARCH=amd64 bash "${SCRIPT_DIR}/package-linux.sh"

assert_file_exists "${ARCHIVE_PATH}"

archive_listing="$(tar -tzf "${ARCHIVE_PATH}")"
pwsh_log="$(<"${PW_LOG_FILE}")"
go_log="$(<"${GO_LOG_FILE}")"

assert_contains 'b-ui/sui' "${archive_listing}" 'archive layout'
assert_contains 'b-ui/src/services/runtime/b-ui.sh' "${archive_listing}" 'archive layout'
assert_contains 'b-ui/src/services/systemd/b-ui.service' "${archive_listing}" 'archive layout'
assert_contains 'b-ui/b-ui.sh' "${archive_listing}" 'archive layout'
assert_contains 'b-ui/b-ui.service' "${archive_listing}" 'archive layout'
assert_contains 'build-frontend.ps1' "${pwsh_log}" 'expected frontend build to use host PowerShell path'
assert_not_contains 'build-backend.ps1' "${pwsh_log}" 'expected backend build to stay on linux shell path'
assert_contains 'GOOS=linux GOARCH=amd64' "${go_log}" 'expected linux backend target'

printf 'package-linux script behavior ok\n'
