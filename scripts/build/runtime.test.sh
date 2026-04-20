#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
REPO_ROOT=$(cd "${SCRIPT_DIR}/../.." && pwd)

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

linux_default_tags='with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_musl,badlinkname,tfogo_checklinkname0,with_tailscale'
windows_default_tags='with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_purego,badlinkname,tfogo_checklinkname0,with_tailscale'

if command -v pwsh.exe >/dev/null 2>&1; then
  assert_equals 'windows' "$(bui_target_os)" "expected default target OS"
  assert_equals 'linux' "$(GOOS=linux bui_target_os)" "expected GOOS override to set linux target"
  bui_should_use_windows_toolchain || fail "expected Windows toolchain detection to succeed"
  if GOOS=linux bui_should_use_windows_toolchain; then
    fail "expected linux target to avoid Windows toolchain"
  fi
  assert_windows_path "$(bui_to_host_path "${REPO_ROOT}")"
  assert_equals "${windows_default_tags}" "$(bui_default_build_tags)" "expected Windows defaults"
  assert_equals "${linux_default_tags}" "$(GOOS=linux bui_default_build_tags)" "expected linux override defaults"
else
  assert_equals 'linux' "$(bui_target_os)" "expected default target OS"
  if bui_should_use_windows_toolchain; then
    fail "expected non-Windows environment to avoid Windows toolchain"
  fi

  assert_equals "${linux_default_tags}" "$(bui_default_build_tags)" "expected non-Windows defaults"
fi

assert_equals 'custom,tag' "$(BUILD_TAGS='custom,tag' bui_resolve_build_tags)" "expected BUILD_TAGS override"

printf 'runtime helpers ok\n'
