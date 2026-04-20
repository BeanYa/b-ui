#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)
TEST_ROOT="${REPO_ROOT}/build/tmp/runtime-test"
MOCK_BIN="${TEST_ROOT}/mock-bin"
EMPTY_BIN="${TEST_ROOT}/empty-bin"
ORIGINAL_PATH="${PATH}"
BASH_BIN=$(command -v bash)
CHMOD_BIN=$(command -v chmod)
RM_BIN=$(command -v rm)

# shellcheck source=./_runtime.sh
source "${SCRIPT_DIR}/_runtime.sh"

fail() {
  printf 'FAIL: %s\n' "$1" >&2
  exit 1
}

assert_windows_path() {
  local value="$1"

  case "${value}" in
    [A-Za-z]:\\*) ;;
    *) fail "expected Windows path, got '${value}'" ;;
  esac
}

assert_equals() {
  local expected="$1"
  local actual="$2"
  local message="$3"

  if [[ "${expected}" != "${actual}" ]]; then
    fail "${message}: expected '${expected}', got '${actual}'"
  fi
}

reset_mock_path() {
  PATH="${MOCK_BIN}"
  hash -r
}

write_mock_command() {
  local name="$1"
  local body="$2"

  printf '%s\n' "#!${BASH_BIN}" 'set -euo pipefail' "${body}" > "${MOCK_BIN}/${name}"
  "${CHMOD_BIN}" +x "${MOCK_BIN}/${name}"
}

rm -rf "${TEST_ROOT}"
mkdir -p "${MOCK_BIN}"
mkdir -p "${EMPTY_BIN}"

write_mock_command 'pwsh.exe' 'exit 0'
write_mock_command 'wslpath' 'printf "C:\\\\mock\\\\path\\n"'
write_mock_command 'cygpath' 'printf "C:\\\\mock\\\\path\\n"'

linux_default_tags='with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_musl,badlinkname,tfogo_checklinkname0,with_tailscale'
windows_default_tags='with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_purego,badlinkname,tfogo_checklinkname0,with_tailscale'
default_ldflags='-w -s -checklinkname=0'
darwin_ldflags='-w -s -checklinkname=0 -extldflags "-Wl,-no_warn_duplicate_libraries"'

PATH="${EMPTY_BIN}"
hash -r
assert_equals 'linux' "$(bui_target_os)" "expected plain host to default to linux"
if bui_should_use_windows_toolchain; then
  fail 'expected plain host to avoid Windows toolchain'
fi
assert_equals "${linux_default_tags}" "$(bui_default_build_tags)" "expected linux defaults without host bridge"
assert_equals "${default_ldflags}" "$(bui_backend_ldflags)" "expected linux backend ldflags without Darwin-only linker flag"

PATH="${MOCK_BIN}"
"${RM_BIN}" -f "${MOCK_BIN}/wslpath" "${MOCK_BIN}/cygpath"
hash -r
assert_equals 'linux' "$(bui_target_os)" "expected pwsh alone to stay on linux target"
if bui_should_use_windows_toolchain; then
  fail 'expected pwsh without path bridge to avoid Windows toolchain'
fi
assert_equals "${REPO_ROOT}" "$(bui_to_host_path "${REPO_ROOT}")" "expected path to stay unchanged without host bridge"
assert_equals "${linux_default_tags}" "$(bui_default_build_tags)" "expected linux defaults when host bridge is missing"

write_mock_command 'wslpath' 'printf "C:\\\\mock\\\\path\\n"'
reset_mock_path
assert_equals 'windows' "$(bui_target_os)" "expected bridged host to default to windows"
assert_equals 'linux' "$(GOOS=linux bui_target_os)" "expected GOOS override to set linux target"
assert_equals 'darwin' "$(GOOS=darwin bui_target_os)" "expected GOOS override to set darwin target"
bui_should_use_windows_toolchain || fail 'expected bridged host to use Windows toolchain'
if GOOS=linux bui_should_use_windows_toolchain; then
  fail 'expected linux target to avoid Windows toolchain even with host bridge'
fi
assert_windows_path "$(bui_to_host_path "${REPO_ROOT}")"
assert_equals "${windows_default_tags}" "$(bui_default_build_tags)" "expected Windows defaults with host bridge"
assert_equals "${linux_default_tags}" "$(GOOS=linux bui_default_build_tags)" "expected linux override defaults"
assert_equals "${default_ldflags}" "$(GOOS=linux bui_backend_ldflags)" "expected linux backend ldflags to omit Darwin-only linker flag"
assert_equals "${darwin_ldflags}" "$(GOOS=darwin bui_backend_ldflags)" "expected darwin backend ldflags to keep Darwin-only linker flag"

assert_equals 'custom,tag' "$(BUILD_TAGS='custom,tag' bui_resolve_build_tags)" "expected BUILD_TAGS override"

printf 'runtime helpers ok\n'
