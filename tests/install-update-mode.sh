#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_SCRIPT="${ROOT_DIR}/install.sh"

RUN_STATUS=0
RUN_OUTPUT=""

fail() {
    echo "FAIL: $*" >&2
    exit 1
}

assert_eq() {
    local actual="$1"
    local expected="$2"
    local message="$3"
    if [[ "${actual}" != "${expected}" ]]; then
        fail "${message}: expected '${expected}', got '${actual}'"
    fi
}

assert_contains() {
    local haystack="$1"
    local needle="$2"
    local message="$3"
    if [[ "${haystack}" != *"${needle}"* ]]; then
        fail "${message}: expected output to contain '${needle}'"
    fi
}

assert_not_contains() {
    local haystack="$1"
    local needle="$2"
    local message="$3"
    if [[ "${haystack}" == *"${needle}"* ]]; then
        fail "${message}: did not expect output to contain '${needle}'"
    fi
}

make_version_binary() {
    local binary_path="$1"
    local version_value="$2"

    cat >"${binary_path}" <<EOF
#!/usr/bin/env bash
if [[ "\${1:-}" == "-v" ]]; then
    echo "sui ${version_value}"
    exit 0
fi
exit 0
EOF
    chmod +x "${binary_path}"
}

run_update_check() {
    local install_kind="$1"
    local current_version="$2"
    local target_version="$3"
    local scenario_dir=""
    local status=0
    local output=""

    scenario_dir="$(mktemp -d)"
    mkdir -p "${scenario_dir}/bin" "${scenario_dir}/install/db" "${scenario_dir}/etc/systemd/system"

    case "${install_kind}" in
    none)
        ;;
    legacy)
        touch "${scenario_dir}/bin/b-ui"
        touch "${scenario_dir}/etc/systemd/system/b-ui.service"
        ;;
    b-ui)
        touch "${scenario_dir}/bin/b-ui"
        touch "${scenario_dir}/etc/systemd/system/b-ui.service"
        make_version_binary "${scenario_dir}/install/sui" "${current_version}"
        ;;
    *)
        rm -rf "${scenario_dir}"
        fail "Unknown install kind: ${install_kind}"
        ;;
    esac

    set +e
    output="$(
        INSTALL_ROOT="${scenario_dir}/install" \
        CLI_PATH="${scenario_dir}/bin/b-ui" \
        LEGACY_CLI_PATH="${scenario_dir}/bin/b-ui" \
        SYSTEMD_DIR="${scenario_dir}/etc/systemd/system" \
        TEST_TARGET_VERSION="${target_version}" \
        bash -lc '
            source "'"${INSTALL_SCRIPT}"'"
            MODE="update"
            TARGET_VERSION="${TEST_TARGET_VERSION}"
            detect_existing_install
            check_update_requirement
            echo "__CONTINUE__"
        ' 2>&1
    )"
    status=$?
    set -e

    rm -rf "${scenario_dir}"

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

test_script_can_be_sourced_without_running_main() {
    local status=0

    set +e
    bash -lc 'source "'"${INSTALL_SCRIPT}"'"; declare -F check_update_requirement >/dev/null' >/dev/null 2>&1
    status=$?
    set -e

    assert_eq "${status}" "0" "install.sh should be sourceable for isolated shell tests"
}

test_update_refuses_missing_b_ui_install() {
    run_update_check "none" "" "v1.2.0"
    assert_eq "${RUN_STATUS}" "1" "update without b-ui install should fail"
    assert_contains "${RUN_OUTPUT}" "System does not have b-ui installed" "missing install message"
    assert_contains "${RUN_OUTPUT}" "bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh)" "missing install command"
}

test_update_refuses_legacy_only_s_ui_install() {
    run_update_check "legacy" "" "v1.2.0"
    assert_eq "${RUN_STATUS}" "1" "legacy-only install should require migration"
    assert_contains "${RUN_OUTPUT}" "Detected b-ui but b-ui is not installed" "legacy detection message"
    assert_contains "${RUN_OUTPUT}" "bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --migrate" "missing migrate command"
}

test_update_exits_when_current_version_is_equal_or_newer() {
    run_update_check "b-ui" "v1.3.0" "v1.2.0"
    assert_eq "${RUN_STATUS}" "0" "equal-or-newer b-ui install should exit successfully"
    assert_contains "${RUN_OUTPUT}" "already up to date" "up-to-date message"
    assert_contains "${RUN_OUTPUT}" "bash <(curl -Ls https://raw.githubusercontent.com/BeanYa/b-ui/main/install.sh) --force-update" "missing force-update command"
    assert_not_contains "${RUN_OUTPUT}" "__CONTINUE__" "up-to-date path should stop before install"
}

test_update_continues_when_current_version_is_older() {
    run_update_check "b-ui" "v1.0.0" "v1.2.0"
    assert_eq "${RUN_STATUS}" "0" "older b-ui install should continue to install flow"
    assert_contains "${RUN_OUTPUT}" "__CONTINUE__" "outdated install should continue"
    assert_not_contains "${RUN_OUTPUT}" "Compatible legacy installation detected" "normal b-ui update should not use legacy wording"
}

test_script_can_be_sourced_without_running_main
test_update_refuses_missing_b_ui_install
test_update_refuses_legacy_only_s_ui_install
test_update_exits_when_current_version_is_equal_or_newer
test_update_continues_when_current_version_is_older

echo "PASS: install update mode checks"
