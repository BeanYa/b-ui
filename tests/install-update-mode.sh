#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_SCRIPT="${ROOT_DIR}/install.sh"
CENTRAL_INSTALL_SCRIPT="${ROOT_DIR}/scripts/release/install.sh"

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
    echo "b-ui ${version_value}"
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
    mkdir -p "${scenario_dir}/bin" "${scenario_dir}/etc/systemd/system"

    case "${install_kind}" in
    none)
        ;;
    legacy)
        mkdir -p "${scenario_dir}/legacy-install/db"
        touch "${scenario_dir}/bin/s-ui"
        touch "${scenario_dir}/etc/systemd/system/s-ui.service"
        touch "${scenario_dir}/legacy-install/db/s-ui.db"
        ;;
    b-ui)
        mkdir -p "${scenario_dir}/install/db"
        touch "${scenario_dir}/bin/b-ui"
        touch "${scenario_dir}/etc/systemd/system/b-ui.service"
        make_version_binary "${scenario_dir}/install/b-ui" "${current_version}"
        ;;
    *)
        rm -rf "${scenario_dir}"
        fail "Unknown install kind: ${install_kind}"
        ;;
    esac

    set +e
    output="$(
        INSTALL_ROOT="${scenario_dir}/install" \
        LEGACY_INSTALL_ROOT="${scenario_dir}/legacy-install" \
        CLI_PATH="${scenario_dir}/bin/b-ui" \
        LEGACY_CLI_PATH="${scenario_dir}/bin/s-ui" \
        LEGACY_SERVICE_NAME="s-ui" \
        LEGACY_DB_FILE="${scenario_dir}/legacy-install/db/s-ui.db" \
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

test_install_script_legacy_defaults_are_s_ui() {
    local script_contents=""

    script_contents="$(<"${CENTRAL_INSTALL_SCRIPT}")"

    assert_contains "${script_contents}" 'LEGACY_CLI_PATH="${LEGACY_CLI_PATH:-/usr/bin/s-ui}"' "legacy cli default"
    assert_contains "${script_contents}" 'LEGACY_SERVICE_NAME="${LEGACY_SERVICE_NAME:-s-ui}"' "legacy service default"
    assert_contains "${script_contents}" 'LEGACY_DB_FILE="${LEGACY_DB_FILE:-${LEGACY_INSTALL_ROOT}/db/s-ui.db}"' "legacy db default"
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
    assert_contains "${RUN_OUTPUT}" "Detected legacy s-ui installation but b-ui is not installed" "legacy detection message"
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

test_update_package_copy_preserves_existing_database_files() {
    local scenario_dir=""
    local status=0
    local output=""

    scenario_dir="$(mktemp -d)"
    mkdir -p "${scenario_dir}/install/db" "${scenario_dir}/package/db"
    printf 'existing-main' > "${scenario_dir}/install/db/b-ui.db"
    printf 'existing-wal' > "${scenario_dir}/install/db/b-ui.db-wal"
    printf 'package-main' > "${scenario_dir}/package/db/b-ui.db"
    printf '#!/usr/bin/env bash\n' > "${scenario_dir}/package/b-ui.sh"

    set +e
    output="$(
        INSTALL_ROOT="${scenario_dir}/install" \
        bash -lc '
            source "'"${INSTALL_SCRIPT}"'"
            copy_package_to_install_root "'"${scenario_dir}/package"'"
            printf "db:%s\n" "$(cat "'"${scenario_dir}/install/db/b-ui.db"'")"
            printf "wal:%s\n" "$(cat "'"${scenario_dir}/install/db/b-ui.db-wal"'")"
            if [[ -f "'"${scenario_dir}/install/b-ui.sh"'" ]]; then
                printf "script:copied\n"
            fi
        ' 2>&1
    )"
    status=$?
    set -e

    rm -rf "${scenario_dir}"

    assert_eq "${status}" "0" "package copy helper should run"
    assert_contains "${output}" "db:existing-main" "package copy must not overwrite the existing main database"
    assert_contains "${output}" "wal:existing-wal" "package copy must not overwrite existing database sidecars"
    assert_contains "${output}" "script:copied" "package copy should still install program files"
}

test_install_mode_preserves_existing_credentials() {
    local scenario_dir=""
    local status=0
    local output=""
    local command_log=""

    scenario_dir="$(mktemp -d)"
    mkdir -p "${scenario_dir}/install" "${scenario_dir}/bin"
    command_log="${scenario_dir}/commands.log"
    cat >"${scenario_dir}/install/b-ui" <<'EOF'
#!/usr/bin/env bash
printf '%s\n' "$*" >>"${TEST_COMMAND_LOG}"
exit 0
EOF
    chmod +x "${scenario_dir}/install/b-ui"

    set +e
    output="$(
        INSTALL_ROOT="${scenario_dir}/install" \
        BINARY_PATH="${scenario_dir}/install/b-ui" \
        TEST_COMMAND_LOG="${command_log}" \
        bash -lc '
            source "'"${INSTALL_SCRIPT}"'"
            MODE="install"
            EXISTING_INSTALL=1
            INSTALLATION_KIND="b-ui"
            config_after_install
            cat "'"${command_log}"'"
        ' 2>&1
    )"
    status=$?
    set -e

    rm -rf "${scenario_dir}"

    assert_eq "${status}" "0" "existing install config should complete"
    assert_contains "${output}" "migrate" "existing install should still migrate database schema"
    assert_not_contains "${output}" "admin -username" "existing install should not overwrite admin credentials"
    assert_contains "${output}" "Current settings and credentials have been kept" "existing install should report preserved credentials"
}

test_legacy_migration_stages_s_ui_database_without_package_defaults() {
    local scenario_dir=""
    local status=0
    local output=""

    scenario_dir="$(mktemp -d)"
    mkdir -p "${scenario_dir}/legacy/db" "${scenario_dir}/package/db"
    printf 'legacy-main' > "${scenario_dir}/legacy/db/s-ui.db"
    printf 'package-main' > "${scenario_dir}/package/db/b-ui.db"
    printf '#!/usr/bin/env bash\n' > "${scenario_dir}/package/b-ui.sh"

    set +e
    output="$(
        INSTALL_ROOT="${scenario_dir}/install" \
        LEGACY_INSTALL_ROOT="${scenario_dir}/legacy" \
        LEGACY_DB_FILE="${scenario_dir}/legacy/db/s-ui.db" \
        bash -lc '
            source "'"${INSTALL_SCRIPT}"'"
            MODE="migrate"
            INSTALLATION_KIND="legacy-only"
            copy_package_to_install_root "'"${scenario_dir}/package"'"
            stage_legacy_database_for_migration
            printf "legacy:%s\n" "$(cat "'"${scenario_dir}/install/db/s-ui.db"'")"
            if [[ -f "'"${scenario_dir}/install/db/b-ui.db"'" ]]; then
                printf "package-db:present\n"
            else
                printf "package-db:absent\n"
            fi
        ' 2>&1
    )"
    status=$?
    set -e

    rm -rf "${scenario_dir}"

    assert_eq "${status}" "0" "legacy migration staging should complete"
    assert_contains "${output}" "legacy:legacy-main" "legacy s-ui database should be staged for migration"
    assert_contains "${output}" "package-db:absent" "package default database must not be installed before migration"
}

test_install_script_legacy_defaults_are_s_ui
test_script_can_be_sourced_without_running_main
test_update_refuses_missing_b_ui_install
test_update_refuses_legacy_only_s_ui_install
test_update_exits_when_current_version_is_equal_or_newer
test_update_continues_when_current_version_is_older
test_update_package_copy_preserves_existing_database_files
test_install_mode_preserves_existing_credentials
test_legacy_migration_stages_s_ui_database_without_package_defaults

echo "PASS: install update mode checks"
