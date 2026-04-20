#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
TEST_ROOT="${REPO_ROOT}/build/tmp/verify-test"
MOCK_BIN="${TEST_ROOT}/mock-bin"
PW_LOG_FILE="${TEST_ROOT}/pwsh.log"
GO_LOG_FILE="${TEST_ROOT}/go.log"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

assert_contains() {
  local needle="$1"
  local haystack="$2"
  local message="$3"

  [[ "${haystack}" == *"${needle}"* ]] || fail "${message}: missing '${needle}'"
}

rm -rf "${TEST_ROOT}"
mkdir -p "${MOCK_BIN}"

cat > "${MOCK_BIN}/pwsh.exe" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${PW_LOG_FILE}"
EOF

cat > "${MOCK_BIN}/go" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail
printf '%s\n' "$*" >> "${GO_LOG_FILE}"
printf 'go should not be invoked directly from bash when Windows toolchain is available\n' >&2
exit 1
EOF

chmod +x "${MOCK_BIN}/pwsh.exe" "${MOCK_BIN}/go"

export PW_LOG_FILE GO_LOG_FILE
export PATH="${MOCK_BIN}:${PATH}"

bash "${SCRIPT_DIR}/verify.sh"

pwsh_log="$(<"${PW_LOG_FILE}")"

assert_contains 'verify-go.ps1' "${pwsh_log}" 'expected go test verification to use PowerShell delegation'
assert_contains 'build-frontend.ps1' "${pwsh_log}" 'expected frontend build to use PowerShell delegation'
assert_contains 'build-backend.ps1' "${pwsh_log}" 'expected backend build to use PowerShell delegation'

if [[ -s "${GO_LOG_FILE}" ]]; then
  fail 'expected no direct bash go invocation when Windows toolchain is available'
fi

printf 'verify script behavior ok\n'
