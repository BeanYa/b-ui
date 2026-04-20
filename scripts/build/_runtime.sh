#!/usr/bin/env bash

set -euo pipefail

bui_should_use_windows_toolchain() {
  [[ "$(bui_target_os)" == 'windows' ]] && bui_has_windows_toolchain
}

bui_has_windows_toolchain() {
  command -v pwsh.exe >/dev/null 2>&1
}

bui_target_os() {
  if [[ -n "${GOOS:-}" ]]; then
    printf '%s\n' "${GOOS}"
    return
  fi

  if bui_has_windows_toolchain; then
    printf 'windows\n'
    return
  fi

  printf 'linux\n'
}

bui_to_host_path() {
  local path="$1"

  if bui_should_use_windows_toolchain; then
    if command -v wslpath >/dev/null 2>&1; then
      wslpath -w "$path"
      return
    fi

    if command -v cygpath >/dev/null 2>&1; then
      cygpath -aw "$path"
      return
    fi
  fi

  printf '%s\n' "$path"
}

bui_run_powershell_file() {
  local script_path="$1"
  shift

  pwsh.exe -NoLogo -NoProfile -File "$(bui_to_host_path "$script_path")" "$@"
}

bui_default_build_tags() {
  if [[ "$(bui_target_os)" == 'windows' ]]; then
    printf '%s\n' 'with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_purego,badlinkname,tfogo_checklinkname0,with_tailscale'
    return
  fi

  printf '%s\n' 'with_quic,with_grpc,with_utls,with_acme,with_gvisor,with_naive_outbound,with_musl,badlinkname,tfogo_checklinkname0,with_tailscale'
}

bui_resolve_build_tags() {
  printf '%s\n' "${BUILD_TAGS:-$(bui_default_build_tags)}"
}
