#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
INSTALL_SCRIPT="${ROOT_DIR}/scripts/release/install-docker.sh"

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
        fail "${message}: expected output to omit '${needle}'"
    fi
}

run_source_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc 'source "'"${INSTALL_SCRIPT}"'"; declare -F require_tools >/dev/null; declare -F build_setting_args >/dev/null; declare -F start_compose_stack >/dev/null; declare -F wait_for_panel >/dev/null; declare -F apply_base_settings >/dev/null; declare -F login_panel >/dev/null; declare -F api_get_json >/dev/null; declare -F api_post_save >/dev/null; declare -F render_compose_template >/dev/null; declare -F write_deploy_files >/dev/null; declare -F fetch_tls_keypair >/dev/null; declare -F fetch_reality_keypair >/dev/null; declare -F build_tls_payload >/dev/null; declare -F build_client_payload >/dev/null; declare -F build_inbound_payload >/dev/null; declare -F run_protocol_bootstrap >/dev/null; printf "PORTS:%s" "${#PORT_MAPPINGS[@]}"' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_build_tls_payload_check() {
    local preset="$1"
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"

        fetch_tls_keypair() {
            printf "%s\n" \
                "-----BEGIN PRIVATE KEY-----" \
                "private-line" \
                "-----END PRIVATE KEY-----" \
                "-----BEGIN CERTIFICATE-----" \
                "certificate-line" \
                "-----END CERTIFICATE-----"
        }

        fetch_reality_keypair() {
            printf "%s\n" \
                "PrivateKey: reality-private" \
                "PublicKey: reality-public"
        }

        BOOTSTRAP_REALITY_SHORT_ID="0123abcd"
        build_tls_payload "'"${preset}"'"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_build_inbound_payload_check() {
    local preset="$1"
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        BOOTSTRAP_LISTEN_PORT="8443"
        build_inbound_payload "'"${preset}"'" "41"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_fetch_tls_keypair_endpoint_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"

        api_get_json() {
            printf "endpoint:%s\n" "$1" >&2
            printf "{\"success\":true,\"obj\":[\"-----BEGIN PRIVATE KEY-----\",\"private-line\",\"-----END PRIVATE KEY-----\",\"-----BEGIN CERTIFICATE-----\",\"certificate-line\",\"-----END CERTIFICATE-----\"]}"
        }

        fetch_tls_keypair ""
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_protocol_bootstrap_failure_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"

        BOOTSTRAP_TLS_PRESET="reality"
        BOOTSTRAP_CLIENT_NAME="docker-user"
        BOOTSTRAP_CLIENT_UUID="11111111-1111-4111-8111-111111111111"
        BOOTSTRAP_LISTEN_PORT="8443"

        fetch_reality_keypair() {
            printf "%s\n" "PrivateKey: reality-private" "PublicKey: reality-public"
        }

        api_post_save() {
            local object="$1"
            local action="$2"
            local payload="$3"
            case "${object}" in
                tls)
                    if [[ "${action}" == "del" ]]; then
                        printf "rollback:tls:%s\n" "${payload}" >&2
                        printf '{"success":true}'
                        return 0
                    fi
                    printf "{\"success\":true,\"obj\":{\"tls\":[{\"id\":41,\"name\":\"reality-template\"}]}}"
                    ;;
                clients)
                    if [[ "${action}" == "del" ]]; then
                        printf "rollback:clients:%s\n" "${payload}" >&2
                        printf '{"success":true}'
                        return 0
                    fi
                    printf "{\"success\":true,\"obj\":{\"clients\":[{\"id\":52,\"enable\":true,\"name\":\"docker-user\"}]}}"
                    ;;
                inbounds)
                    printf "{\"success\":false,\"msg\":\"save: inbound failed\"}"
                    ;;
            esac
        }

        run_protocol_bootstrap
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_failed_main_cleanup_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        set -euo pipefail
        sandbox="$(mktemp -d)"
        trap "rm -rf \"${sandbox}\"" EXIT
        deploy_dir="${sandbox}/deploy"
        mkdir -p "${deploy_dir}"

        DEPLOY_DIR="${deploy_dir}"
        IMAGE_REF="ghcr.io/example/b-ui:2.0.0"
        PANEL_PORT="3000"
        PANEL_PATH="/panel"
        SUB_PORT="4000"
        SUB_PATH="/sub"
        ADMIN_USERNAME="admin"
        ADMIN_PASSWORD="secret"
        BOOTSTRAP_TLS_PRESET="reality"
        BOOTSTRAP_CLIENT_NAME="docker-user"
        BOOTSTRAP_CLIENT_UUID="11111111-1111-4111-8111-111111111111"
        BOOTSTRAP_LISTEN_PORT="8443"
        COOKIE_JAR="${deploy_dir}/panel-cookie.jar"

        source "'"${INSTALL_SCRIPT}"'"

        require_tools() { :; }
        collect_inputs() { :; }
        write_deploy_files() { :; }
        start_compose_stack() { :; }
        wait_for_panel() { :; }
        apply_base_settings() { :; }
        login_panel() { : >"${COOKIE_JAR}"; }
        run_protocol_bootstrap() { return 1; }

        set +e
        main
        status=$?
        set -e
        printf "status:%s\n" "${status}"
        printf "cookie:%s\n" "$(test -f "${COOKIE_JAR}" && printf yes || printf no)"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_render_compose_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        IMAGE_REF="ghcr.io/example/b-ui:1.2.3"
        CONTAINER_NAME="b-ui-bootstrap"
        PANEL_PORT="3000"
        PANEL_PATH="/panel"
        SUB_PORT="4000"
        SUB_PATH="/sub"
        PORT_MAPPINGS=(
            "3000:3000"
            "3300:3000"
            "4000:4000"
            "4400:4000"
            "5000:5000/udp"
            "5500:5000/udp"
        )
        unique_port_mappings
        render_compose_template
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_write_deploy_files_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        DEPLOY_DIR="$(mktemp -d)"
        IMAGE_REF="ghcr.io/example/b-ui:9.9.9"
        CONTAINER_NAME="b-ui-installer"
        PANEL_PORT="3000"
        PANEL_PATH="/panel"
        SUB_PORT="4000"
        SUB_PATH="/sub"
        PORT_MAPPINGS=(
            "3000:3000"
            "3999:3000"
            "4000:4000"
        )
        write_deploy_files
        compose_file="${DEPLOY_DIR}/docker-compose.yml"
        printf "compose:%s\n" "$(test -f "${compose_file}" && printf yes || printf no)"
        printf "db:%s\n" "$(test -d "${DEPLOY_DIR}/db" && printf yes || printf no)"
        printf "cert:%s\n" "$(test -d "${DEPLOY_DIR}/cert" && printf yes || printf no)"
        printf "%s" "$(<"${compose_file}")"
        rm -rf "${DEPLOY_DIR}"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_empty_port_render_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        IMAGE_REF="ghcr.io/example/b-ui:1.2.3"
        CONTAINER_NAME="b-ui-bootstrap"
        PORT_MAPPINGS=()
        render_compose_template
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_empty_port_write_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        DEPLOY_DIR="$(mktemp -d)"
        IMAGE_REF="ghcr.io/example/b-ui:1.2.3"
        CONTAINER_NAME="b-ui-bootstrap"
        PORT_MAPPINGS=()
        set +e
        write_deploy_files
        status=$?
        set -e
        printf "status:%s\n" "${status}"
        printf "db:%s\n" "$(test -d "${DEPLOY_DIR}/db" && printf yes || printf no)"
        printf "cert:%s\n" "$(test -d "${DEPLOY_DIR}/cert" && printf yes || printf no)"
        printf "compose:%s\n" "$(test -f "${DEPLOY_DIR}/docker-compose.yml" && printf yes || printf no)"
        rm -rf "${DEPLOY_DIR}"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_unique_port_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        PORT_MAPPINGS=(
            "8080:80"
            "9090:80"
            "443:443"
            "4443:443"
            "127.0.0.1:2222:22"
        )
        unique_port_mappings
        printf "%s\n" "${PORT_MAPPINGS[@]}"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_direct_execution_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        set -euo pipefail
        sandbox="$(mktemp -d)"
        trap "rm -rf \"${sandbox}\"" EXIT
        deploy_dir="${sandbox}/deploy"
        log_file="${sandbox}/commands.log"

        TEST_LOG_FILE="${log_file}"
        DEPLOY_DIR="${deploy_dir}"
        IMAGE_REF="ghcr.io/example/b-ui:2.0.0"
        CONTAINER_NAME="b-ui-bootstrap"
        PANEL_PORT=""
        PANEL_PATH=""
        SUB_PORT=""
        SUB_PATH=""
        ADMIN_USERNAME="admin"
        ADMIN_PASSWORD="sec\"ret&more"
        BOOTSTRAP_TLS_PRESET="standard"
        BOOTSTRAP_CLIENT_NAME="docker-user"
        BOOTSTRAP_CLIENT_PASSWORD="change-me"
        BOOTSTRAP_CLIENT_UUID="11111111-1111-4111-8111-111111111111"
        BOOTSTRAP_LISTEN_PORT="8443"
        BOOTSTRAP_SERVER_NAME=""
        WAIT_RETRIES="3"
        WAIT_DELAY="0"

        source "'"${INSTALL_SCRIPT}"'"

        require_tools() {
            printf "require-tools\n" >>"${TEST_LOG_FILE}"
        }

        prompt_with_default() {
            local var_name="$1"
            case "${var_name}" in
                PANEL_PORT) printf "%s" "3000" ;;
                PANEL_PATH) printf "%s" "/panel" ;;
                SUB_PORT) printf "%s" "4000" ;;
                SUB_PATH) printf "%s" "/sub" ;;
                ADMIN_USERNAME) printf "%s" "admin" ;;
                ADMIN_PASSWORD) printf "%s" "sec\"ret&more" ;;
                BOOTSTRAP_PROTOCOL) printf "%s" "2" ;;
                BOOTSTRAP_SERVER_NAME) printf "%s" "" ;;
                BOOTSTRAP_CLIENT_NAME) printf "%s" "docker-user" ;;
                BOOTSTRAP_CLIENT_UUID) printf "%s" "11111111-1111-4111-8111-111111111111" ;;
                BOOTSTRAP_LISTEN_PORT) printf "%s" "8443" ;;
                *) printf "%s" "$2" ;;
            esac
        }

        start_compose_stack() {
            printf "compose-up\n" >>"${TEST_LOG_FILE}"
        }

        wait_for_panel() {
            printf "wait:%s\n" "${PANEL_BASE_URL}" >>"${TEST_LOG_FILE}"
        }

        apply_base_settings() {
            printf "settings:%s\n" "$(build_setting_args | paste -sd" " -)" >>"${TEST_LOG_FILE}"
            printf "admin:%s:%s\n" "${ADMIN_USERNAME}" "${ADMIN_PASSWORD}" >>"${TEST_LOG_FILE}"
        }

        login_panel() {
            : >"${COOKIE_JAR}"
            printf "login:%s:%s\n" "${ADMIN_USERNAME}" "${ADMIN_PASSWORD}" >>"${TEST_LOG_FILE}"
        }

        api_get_json() {
            printf "api-get:%s\n" "$1" >>"${TEST_LOG_FILE}"
            case "$1" in
                */api/keypairs\?k=tls*)
                    printf "{\"success\":true,\"obj\":[\"-----BEGIN PRIVATE KEY-----\",\"tls-private-line\",\"-----END PRIVATE KEY-----\",\"-----BEGIN CERTIFICATE-----\",\"tls-certificate-line\",\"-----END CERTIFICATE-----\"]}"
                    ;;
            esac
        }

        api_post_save() {
            local object="$1"
            local action="$2"
            local payload="$3"
            local init_users="${4:-}"
            printf "save-object:%s\n" "${object}" >>"${TEST_LOG_FILE}"
            printf "save-action:%s\n" "${action}" >>"${TEST_LOG_FILE}"
            printf "save-payload:%s\n" "${payload}" >>"${TEST_LOG_FILE}"
            printf "save-init-users:%s\n" "${init_users}" >>"${TEST_LOG_FILE}"
            case "${object}" in
                tls)
                    printf "{\"success\":true,\"obj\":{\"tls\":[{\"id\":41,\"name\":\"tls-template\"}]}}"
                    ;;
                clients)
                    printf "{\"success\":true,\"obj\":{\"clients\":[{\"id\":52,\"enable\":true,\"name\":\"docker-user\"}]}}"
                    ;;
                inbounds)
                    printf "{\"success\":true,\"obj\":{\"inbounds\":[{\"id\":63,\"type\":\"trojan\",\"tag\":\"trojan-8443\"}]}}"
                    ;;
            esac
        }

        main

        printf 'compose:%s\n' "$(test -f "${deploy_dir}/docker-compose.yml" && printf yes || printf no)"
        printf 'cookie:%s\n' "$(test -f "${deploy_dir}/panel-cookie.jar" && printf yes || printf no)"
        printf '%s\n' '---COMPOSE---'
        cat "${deploy_dir}/docker-compose.yml"
        printf '%s\n' '---LOG---'
        cat "${log_file}"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_protocol_port_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        PORT_MAPPINGS=(
            "8080:80/tcp"
            "9090:80/tcp"
            "8443:443/udp"
            "9443:443/udp"
            "10000-10001:20000-20001/tcp"
            "11000-11001:20000-20001/tcp"
        )
        unique_port_mappings
        printf "%s\n" "${PORT_MAPPINGS[@]}"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_mixed_protocol_port_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        PORT_MAPPINGS=(
            "8080:80"
            "9090:80/tcp"
            "8443:443/udp"
            "9443:443/udp"
        )
        unique_port_mappings
        printf "%s\n" "${PORT_MAPPINGS[@]}"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_build_setting_args_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        PANEL_PORT="3000"
        PANEL_PATH="/panel"
        SUB_PORT=""
        SUB_PATH="/sub"
        build_setting_args
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_collect_inputs_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"

        prompt_with_default() {
            local var_name="$1"
            case "${var_name}" in
                PANEL_PORT) printf "%s" "3100" ;;
                PANEL_PATH) printf "%s" "panel-admin" ;;
                SUB_PORT) printf "%s" "4100" ;;
                SUB_PATH) printf "%s" "sub-feed" ;;
                ADMIN_USERNAME) printf "%s" "operator" ;;
                ADMIN_PASSWORD) printf "%s" "operator-pass" ;;
                BOOTSTRAP_PROTOCOL) printf "%s" "2" ;;
                BOOTSTRAP_SERVER_NAME) printf "%s" "panel.example.com" ;;
                BOOTSTRAP_CLIENT_NAME) printf "%s" "field-user" ;;
                BOOTSTRAP_CLIENT_UUID) printf "%s" "22222222-2222-4222-8222-222222222222" ;;
                BOOTSTRAP_LISTEN_PORT) printf "%s" "9443" ;;
                *) printf "%s" "$2" ;;
            esac
        }

        collect_inputs

        printf "PANEL_PORT=%s\n" "${PANEL_PORT}"
        printf "PANEL_PATH=%s\n" "${PANEL_PATH}"
        printf "SUB_PORT=%s\n" "${SUB_PORT}"
        printf "SUB_PATH=%s\n" "${SUB_PATH}"
        printf "ADMIN_USERNAME=%s\n" "${ADMIN_USERNAME}"
        printf "ADMIN_PASSWORD=%s\n" "${ADMIN_PASSWORD}"
        printf "BOOTSTRAP_TLS_PRESET=%s\n" "${BOOTSTRAP_TLS_PRESET}"
        printf "BOOTSTRAP_SERVER_NAME=%s\n" "${BOOTSTRAP_SERVER_NAME}"
        printf "BOOTSTRAP_CLIENT_NAME=%s\n" "${BOOTSTRAP_CLIENT_NAME}"
        printf "BOOTSTRAP_CLIENT_UUID=%s\n" "${BOOTSTRAP_CLIENT_UUID}"
        printf "BOOTSTRAP_LISTEN_PORT=%s\n" "${BOOTSTRAP_LISTEN_PORT}"
        printf "PORT:%s\n" "${PORT_MAPPINGS[0]}"
        printf "PORT:%s\n" "${PORT_MAPPINGS[1]}"
        printf "PORT:%s\n" "${PORT_MAPPINGS[2]}"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_collect_inputs_skip_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"

        BOOTSTRAP_LISTEN_PORT="8443"

        prompt_with_default() {
            local var_name="$1"
            case "${var_name}" in
                PANEL_PORT) printf "%s" "3100" ;;
                PANEL_PATH) printf "%s" "panel-admin" ;;
                SUB_PORT) printf "%s" "4100" ;;
                SUB_PATH) printf "%s" "sub-feed" ;;
                ADMIN_USERNAME) printf "%s" "operator" ;;
                ADMIN_PASSWORD) printf "%s" "operator-pass" ;;
                BOOTSTRAP_PROTOCOL) printf "%s" "1" ;;
                *) printf "%s" "$2" ;;
            esac
        }

        collect_inputs

        printf "BOOTSTRAP_TLS_PRESET=%s\n" "${BOOTSTRAP_TLS_PRESET}"
        printf "BOOTSTRAP_LISTEN_PORT=%s\n" "${BOOTSTRAP_LISTEN_PORT:-}"
        printf "PORT:%s\n" "${PORT_MAPPINGS[0]}"
        printf "PORT:%s\n" "${PORT_MAPPINGS[1]}"
        printf "PORT:%s\n" "${PORT_MAPPINGS[2]:-}"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_collect_inputs_hysteria2_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"

        prompt_with_default() {
            local var_name="$1"
            case "${var_name}" in
                PANEL_PORT) printf "%s" "3100" ;;
                PANEL_PATH) printf "%s" "panel-admin" ;;
                SUB_PORT) printf "%s" "4100" ;;
                SUB_PATH) printf "%s" "sub-feed" ;;
                ADMIN_USERNAME) printf "%s" "operator" ;;
                ADMIN_PASSWORD) printf "%s" "operator-pass" ;;
                BOOTSTRAP_PROTOCOL) printf "%s" "4" ;;
                BOOTSTRAP_SERVER_NAME) printf "%s" "hy.example.com" ;;
                BOOTSTRAP_CLIENT_NAME) printf "%s" "hy-user" ;;
                BOOTSTRAP_CLIENT_PASSWORD) printf "%s" "hy-pass" ;;
                BOOTSTRAP_LISTEN_PORT) printf "%s" "8443" ;;
                *) printf "%s" "$2" ;;
            esac
        }

        collect_inputs

        printf "BOOTSTRAP_TLS_PRESET=%s\n" "${BOOTSTRAP_TLS_PRESET}"
        printf "PORT:%s\n" "${PORT_MAPPINGS[2]:-}"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_api_post_save_with_init_users_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"

        PANEL_BASE_URL="http://127.0.0.1:3000/panel"
        COOKIE_JAR="/tmp/cookies.txt"
        DEPLOY_DIR="$(mktemp -d)"

        curl() {
            local stdin_data=""
            stdin_data="$(cat)"
            printf "curl:%s\n" "$*"
            printf "stdin:%s\n" "${stdin_data}"
        }

        api_post_save "inbounds" "new" "{\"id\":0}" "52"
        rm -rf "${DEPLOY_DIR}"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_validate_bootstrap_inputs_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        IMAGE_REF=""
        PANEL_PORT=""
        validate_bootstrap_inputs
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_validate_bootstrap_inputs_collision_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        IMAGE_REF="ghcr.io/example/b-ui:1.2.3"
        PANEL_PORT="3000"
        SUB_PORT="3000"
        BOOTSTRAP_TLS_PRESET="standard"
        BOOTSTRAP_LISTEN_PORT="3000"
        validate_bootstrap_inputs
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_validate_bootstrap_inputs_invalid_port_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        IMAGE_REF="ghcr.io/example/b-ui:1.2.3"
        PANEL_PORT="70000"
        SUB_PORT="abc"
        validate_bootstrap_inputs
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_validate_bootstrap_inputs_noninteractive_defaults_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        source "'"${INSTALL_SCRIPT}"'"
        IMAGE_REF="ghcr.io/example/b-ui:1.2.3"
        PANEL_PORT="3000"
        ADMIN_USERNAME="admin"
        ADMIN_PASSWORD="admin"
        BOOTSTRAP_TLS_PRESET="hysteria2"
        BOOTSTRAP_CLIENT_PASSWORD="change-me"
        validate_bootstrap_inputs
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

run_require_tools_missing_compose_check() {
    local output=""
    local status=0

    set +e
    output="$(bash -lc '
        set -euo pipefail
        sandbox="$(mktemp -d)"
        trap "rm -rf \"${sandbox}\"" EXIT
        stub_dir="${sandbox}/bin"
        mkdir -p "${stub_dir}"

        cat >"${stub_dir}/docker" <<'"'"'EOF'"'"'
#!/usr/bin/env bash
set -euo pipefail
if [[ "$1" == "compose" && "$2" == "version" ]]; then
    exit 1
fi
exit 0
EOF

        cat >"${stub_dir}/curl" <<'"'"'EOF'"'"'
#!/usr/bin/env bash
set -euo pipefail
exit 0
EOF

        chmod +x "${stub_dir}/docker" "${stub_dir}/curl"

        PATH="${stub_dir}:${PATH}" bash -c "source \"\$1\"; require_tools" _ "${INSTALL_SCRIPT}"
    ' 2>&1)"
    status=$?
    set -e

    RUN_STATUS="${status}"
    RUN_OUTPUT="${output}"
}

test_script_can_be_sourced_without_running_main() {
    run_source_check

    assert_eq "${RUN_STATUS}" "0" "install-docker.sh should be sourceable"
    assert_contains "${RUN_OUTPUT}" "PORTS:0" "sourcing should preserve empty PORT_MAPPINGS"
}

test_unique_port_mappings_deduplicates_by_container_port() {
    local expected_output=$'8080:80\n443:443\n127.0.0.1:2222:22'

    run_unique_port_check

    assert_eq "${RUN_STATUS}" "0" "unique_port_mappings should succeed"
    assert_eq "${RUN_OUTPUT}" "${expected_output}" "unique_port_mappings should keep first mapping per container port"
}

test_unique_port_mappings_supports_protocols_and_ranges() {
    local expected_output=$'8080:80/tcp\n8443:443/udp\n10000-10001:20000-20001/tcp'

    run_protocol_port_check

    assert_eq "${RUN_STATUS}" "0" "unique_port_mappings should support port protocols and ranges"
    assert_eq "${RUN_OUTPUT}" "${expected_output}" "unique_port_mappings should deduplicate protocol and range mappings by container target"
}

test_unique_port_mappings_treats_plain_ports_as_tcp() {
    local expected_output=$'8080:80\n8443:443/udp'

    run_mixed_protocol_port_check

    assert_eq "${RUN_STATUS}" "0" "unique_port_mappings should normalize plain TCP ports"
    assert_eq "${RUN_OUTPUT}" "${expected_output}" "unique_port_mappings should treat 80 and 80/tcp as the same container target"
}

test_build_setting_args_includes_only_populated_values() {
    local expected_output=$'-panelPort\n3000\n-panelPath\n/panel\n-subPath\n/sub'

    run_build_setting_args_check

    assert_eq "${RUN_STATUS}" "0" "build_setting_args should succeed"
    assert_eq "${RUN_OUTPUT}" "${expected_output}" "build_setting_args should emit each populated flag and value as separate arguments"
}

test_collect_inputs_prompts_and_builds_standard_defaults() {
    run_collect_inputs_check

    assert_eq "${RUN_STATUS}" "0" "collect_inputs should succeed"
    assert_contains "${RUN_OUTPUT}" "PANEL_PORT=3100" "collect_inputs should store the prompted panel port"
    assert_contains "${RUN_OUTPUT}" "PANEL_PATH=/panel-admin" "collect_inputs should normalize the prompted panel path"
    assert_contains "${RUN_OUTPUT}" "SUB_PORT=4100" "collect_inputs should store the prompted subscriber port"
    assert_contains "${RUN_OUTPUT}" "SUB_PATH=/sub-feed" "collect_inputs should normalize the prompted subscriber path"
    assert_contains "${RUN_OUTPUT}" "ADMIN_USERNAME=operator" "collect_inputs should store the prompted admin username"
    assert_contains "${RUN_OUTPUT}" "ADMIN_PASSWORD=operator-pass" "collect_inputs should store the prompted admin password"
    assert_contains "${RUN_OUTPUT}" "BOOTSTRAP_TLS_PRESET=standard" "collect_inputs should map protocol option 2 to the standard preset"
    assert_contains "${RUN_OUTPUT}" "BOOTSTRAP_SERVER_NAME=panel.example.com" "collect_inputs should capture the TLS server name"
    assert_contains "${RUN_OUTPUT}" "BOOTSTRAP_CLIENT_NAME=field-user" "collect_inputs should capture the client name"
    assert_contains "${RUN_OUTPUT}" "BOOTSTRAP_CLIENT_UUID=22222222-2222-4222-8222-222222222222" "collect_inputs should capture the client UUID"
    assert_contains "${RUN_OUTPUT}" "BOOTSTRAP_LISTEN_PORT=9443" "collect_inputs should capture the inbound listen port"
    assert_contains "${RUN_OUTPUT}" "PORT:3100:3100" "collect_inputs should add the panel port mapping"
    assert_contains "${RUN_OUTPUT}" "PORT:4100:4100" "collect_inputs should add the subscriber port mapping"
    assert_contains "${RUN_OUTPUT}" "PORT:9443:9443" "collect_inputs should add the bootstrap inbound port mapping"
}

test_collect_inputs_skip_mode_clears_bootstrap_listener_state() {
    run_collect_inputs_skip_check

    assert_eq "${RUN_STATUS}" "0" "collect_inputs should support skip mode"
    assert_contains "${RUN_OUTPUT}" "BOOTSTRAP_TLS_PRESET=" "collect_inputs should clear the bootstrap preset in skip mode"
    assert_contains "${RUN_OUTPUT}" "BOOTSTRAP_LISTEN_PORT=" "collect_inputs should clear the bootstrap listen port in skip mode"
    assert_contains "${RUN_OUTPUT}" "PORT:3100:3100" "skip mode should keep the panel port mapping"
    assert_contains "${RUN_OUTPUT}" "PORT:4100:4100" "skip mode should keep the subscriber port mapping"
    assert_not_contains "${RUN_OUTPUT}" "8443:8443" "skip mode should not keep the bootstrap inbound port mapping"
}

test_collect_inputs_hysteria2_uses_udp_port_mapping() {
    run_collect_inputs_hysteria2_check

    assert_eq "${RUN_STATUS}" "0" "collect_inputs should support the hysteria2 bootstrap option"
    assert_contains "${RUN_OUTPUT}" "BOOTSTRAP_TLS_PRESET=hysteria2" "collect_inputs should map protocol option 4 to hysteria2"
    assert_contains "${RUN_OUTPUT}" "PORT:8443:8443/udp" "collect_inputs should publish the hysteria2 inbound port as UDP"
}

test_validate_bootstrap_inputs_requires_image_and_panel_port() {
    run_validate_bootstrap_inputs_check

    assert_eq "${RUN_STATUS}" "1" "validate_bootstrap_inputs should fail when required inputs are missing"
    assert_contains "${RUN_OUTPUT}" "IMAGE_REF" "validate_bootstrap_inputs should require the image reference"
    assert_contains "${RUN_OUTPUT}" "PANEL_PORT" "validate_bootstrap_inputs should require the panel port"
}

test_validate_bootstrap_inputs_rejects_port_collisions() {
    run_validate_bootstrap_inputs_collision_check

    assert_eq "${RUN_STATUS}" "1" "validate_bootstrap_inputs should fail when bootstrap ports collide"
    assert_contains "${RUN_OUTPUT}" "must not reuse" "validate_bootstrap_inputs should explain port collision failures"
}

test_validate_bootstrap_inputs_rejects_invalid_port_values() {
    run_validate_bootstrap_inputs_invalid_port_check

    assert_eq "${RUN_STATUS}" "1" "validate_bootstrap_inputs should fail when port values are invalid"
    assert_contains "${RUN_OUTPUT}" "must be an integer between 1 and 65535" "validate_bootstrap_inputs should explain invalid port ranges"
}

test_validate_bootstrap_inputs_rejects_noninteractive_default_secrets() {
    run_validate_bootstrap_inputs_noninteractive_defaults_check

    assert_eq "${RUN_STATUS}" "1" "validate_bootstrap_inputs should fail closed on non-interactive default secrets"
    assert_contains "${RUN_OUTPUT}" "non-interactive" "validate_bootstrap_inputs should explain the non-interactive secret requirement"
}

test_require_tools_rejects_missing_docker_compose() {
    run_require_tools_missing_compose_check

    assert_eq "${RUN_STATUS}" "1" "require_tools should fail when docker compose is unavailable"
}

test_api_post_save_preserves_init_users_argument() {
    run_api_post_save_with_init_users_check

    assert_eq "${RUN_STATUS}" "0" "api_post_save should succeed with a stubbed curl command"
    assert_contains "${RUN_OUTPUT}" 'curl:--silent --show-error --fail -b /tmp/cookies.txt -X POST --data-urlencode object=inbounds --data-urlencode action=new --data-urlencode data@' "api_post_save should move JSON payloads into a temporary curl data file"
    assert_contains "${RUN_OUTPUT}" 'initUsers=52' "api_post_save should preserve initUsers in the streamed curl config"
}

test_direct_execution_bootstraps_startup_readiness_and_base_settings() {
    run_direct_execution_check

    assert_eq "${RUN_STATUS}" "0" "direct execution should complete with mocked docker dependencies"
    assert_contains "${RUN_OUTPUT}" "compose:yes" "direct execution should write docker-compose.yml"
    assert_contains "${RUN_OUTPUT}" "require-tools" "direct execution should validate the required tooling first"
    assert_contains "${RUN_OUTPUT}" "compose-up" "direct execution should start the compose stack"
    assert_contains "${RUN_OUTPUT}" "settings:-panelPort 3000 -panelPath /panel -subPort 4000 -subPath /sub" "direct execution should apply only populated base settings"
    assert_contains "${RUN_OUTPUT}" 'admin:admin:sec"ret&more' "direct execution should bootstrap the admin user"
    assert_contains "${RUN_OUTPUT}" "wait:http://127.0.0.1:3000/panel" "direct execution should poll the normalized panel base URL"
    assert_contains "${RUN_OUTPUT}" 'login:admin:sec"ret&more' "direct execution should log in with the admin credentials"
    assert_contains "${RUN_OUTPUT}" "api-get:/api/keypairs?k=tls&o=" "direct execution should fetch TLS key material before saving templates"
    assert_contains "${RUN_OUTPUT}" "save-object:tls" "direct execution should save a TLS template first"
    assert_contains "${RUN_OUTPUT}" 'save-payload:{"id":0,"name":"tls-template"' "direct execution should save the standard TLS payload"
    assert_contains "${RUN_OUTPUT}" "save-object:clients" "direct execution should save a bootstrap client"
    assert_contains "${RUN_OUTPUT}" 'save-payload:{"enable":true,"name":"docker-user","config":{"vless":{"name":"docker-user","uuid":"11111111-1111-4111-8111-111111111111","flow":"xtls-rprx-vision"}}' "direct execution should save the VLESS bootstrap client payload"
    assert_contains "${RUN_OUTPUT}" "save-object:inbounds" "direct execution should save a bootstrap inbound"
    assert_contains "${RUN_OUTPUT}" 'save-payload:{"id":0,"type":"vless","tag":"vless-8443","tls_id":41' "direct execution should wire the inbound to the saved TLS template"
    assert_contains "${RUN_OUTPUT}" "save-init-users:52" "direct execution should attach the saved client to the inbound creation call"
    assert_contains "${RUN_OUTPUT}" '      - "8443:8443"' "direct execution should expose the bootstrap inbound port in docker-compose.yml"
    assert_contains "${RUN_OUTPUT}" "Panel URL: http://<server-ip>:3000/panel" "direct execution should print the final operator-facing panel URL"
    assert_contains "${RUN_OUTPUT}" "Compose file: " "direct execution should print the compose file location"
    assert_contains "${RUN_OUTPUT}" "docker-compose.yml" "direct execution should reference the compose file name"
    assert_contains "${RUN_OUTPUT}" "cookie:no" "direct execution should remove the temporary panel cookie jar before finishing"
}

test_render_compose_template_renders_bootstrap_compose() {
    run_render_compose_check

    assert_eq "${RUN_STATUS}" "0" "render_compose_template should succeed"
    assert_contains "${RUN_OUTPUT}" "services:" "compose should define services"
    assert_contains "${RUN_OUTPUT}" "  b-ui:" "compose should define the b-ui service"
    assert_contains "${RUN_OUTPUT}" "    image: ghcr.io/example/b-ui:1.2.3" "compose should include the image reference"
    assert_contains "${RUN_OUTPUT}" "    container_name: b-ui-bootstrap" "compose should include the container name"
    assert_contains "${RUN_OUTPUT}" "    hostname: b-ui-bootstrap" "compose should include the hostname"
    assert_contains "${RUN_OUTPUT}" "      - ./db:/app/db" "compose should mount the db directory"
    assert_contains "${RUN_OUTPUT}" "      - ./cert:/app/cert" "compose should mount the cert directory"
    assert_contains "${RUN_OUTPUT}" "    tty: true" "compose should enable tty"
    assert_contains "${RUN_OUTPUT}" "    restart: unless-stopped" "compose should configure restart policy"
    assert_contains "${RUN_OUTPUT}" "    entrypoint: ./entrypoint.sh" "compose should define the bootstrap entrypoint"
    assert_contains "${RUN_OUTPUT}" "      PANEL_PORT: \"3000\"" "compose should render the panel port placeholder"
    assert_contains "${RUN_OUTPUT}" "      PANEL_PATH: \"/panel\"" "compose should render the panel path placeholder"
    assert_contains "${RUN_OUTPUT}" "      SUB_PORT: \"4000\"" "compose should render the subscriber port placeholder"
    assert_contains "${RUN_OUTPUT}" "      SUB_PATH: \"/sub\"" "compose should render the subscriber path placeholder"
    assert_contains "${RUN_OUTPUT}" "      - \"3000:3000\"" "compose should include the first panel port mapping"
    assert_contains "${RUN_OUTPUT}" "      - \"4000:4000\"" "compose should include the first subscriber port mapping"
    assert_contains "${RUN_OUTPUT}" "      - \"5000:5000/udp\"" "compose should include UDP port mappings"
}

test_write_deploy_files_writes_compose_and_directories() {
    run_write_deploy_files_check

    assert_eq "${RUN_STATUS}" "0" "write_deploy_files should succeed"
    assert_contains "${RUN_OUTPUT}" "compose:yes" "write_deploy_files should create the compose file"
    assert_contains "${RUN_OUTPUT}" "db:yes" "write_deploy_files should create the db directory"
    assert_contains "${RUN_OUTPUT}" "cert:yes" "write_deploy_files should create the cert directory"
    assert_contains "${RUN_OUTPUT}" "    image: ghcr.io/example/b-ui:9.9.9" "write_deploy_files should render the requested image"
    assert_contains "${RUN_OUTPUT}" "      PANEL_PORT: \"3000\"" "write_deploy_files should render the panel port"
    assert_contains "${RUN_OUTPUT}" "      - \"3000:3000\"" "write_deploy_files should keep the first mapping for panel port"
    assert_contains "${RUN_OUTPUT}" "      - \"4000:4000\"" "write_deploy_files should render the subscriber port"
}

test_render_compose_template_requires_at_least_one_port_mapping() {
    run_empty_port_render_check

    assert_eq "${RUN_STATUS}" "1" "render_compose_template should fail without any port mappings"
    assert_contains "${RUN_OUTPUT}" "at least one port mapping" "render_compose_template should explain the missing port mappings"
}

test_fetch_tls_keypair_keeps_empty_server_name_query_empty() {
    run_fetch_tls_keypair_endpoint_check

    assert_eq "${RUN_STATUS}" "0" "fetch_tls_keypair should succeed with an empty server name"
    assert_contains "${RUN_OUTPUT}" 'endpoint:/api/keypairs?k=tls&o=' "fetch_tls_keypair should keep the empty server name query empty"
    assert_not_contains "${RUN_OUTPUT}" "o=''" "fetch_tls_keypair should not rewrite an empty server name to a quoted literal"
}

test_build_tls_payload_standard_uses_the_standard_template_shape() {
    run_build_tls_payload_check "standard"

    assert_eq "${RUN_STATUS}" "0" "build_tls_payload should build the standard preset"
    assert_contains "${RUN_OUTPUT}" '"name":"tls-template"' "standard payload should use the standard template name"
    assert_contains "${RUN_OUTPUT}" '"server":{"enabled":true,"server_name":"","alpn":["h2","http/1.1"]' "standard payload should enable server TLS with SNI and ALPN"
    assert_contains "${RUN_OUTPUT}" '"key":["-----BEGIN PRIVATE KEY-----","private-line","-----END PRIVATE KEY-----"]' "standard payload should embed the fetched private key"
    assert_contains "${RUN_OUTPUT}" '"certificate":["-----BEGIN CERTIFICATE-----","certificate-line","-----END CERTIFICATE-----"]' "standard payload should embed the fetched certificate"
    assert_contains "${RUN_OUTPUT}" '"client":{"insecure":true}' "standard payload should enable insecure client mode"
}

test_build_tls_payload_hysteria2_uses_h3_and_tls_1_3_defaults() {
    run_build_tls_payload_check "hysteria2"

    assert_eq "${RUN_STATUS}" "0" "build_tls_payload should build the hysteria2 preset"
    assert_contains "${RUN_OUTPUT}" '"name":"hysteria2-template"' "hysteria2 payload should use the hysteria2 template name"
    assert_contains "${RUN_OUTPUT}" '"server":{"enabled":true,"server_name":""' "hysteria2 payload should keep the empty server name"
    assert_contains "${RUN_OUTPUT}" '"alpn":["h3"]' "hysteria2 payload should pin ALPN to h3"
    assert_contains "${RUN_OUTPUT}" '"min_version":"1.3"' "hysteria2 payload should pin the minimum TLS version to 1.3"
    assert_contains "${RUN_OUTPUT}" '"max_version":"1.3"' "hysteria2 payload should pin the maximum TLS version to 1.3"
    assert_contains "${RUN_OUTPUT}" '"client":{"insecure":true}' "hysteria2 payload should enable insecure client mode"
}

test_build_inbound_payload_hysteria2_includes_bandwidth_defaults() {
    run_build_inbound_payload_check "hysteria2"

    assert_eq "${RUN_STATUS}" "0" "build_inbound_payload should build the hysteria2 inbound"
    assert_contains "${RUN_OUTPUT}" '"type":"hysteria2"' "hysteria2 inbound payload should use the hysteria2 type"
    assert_contains "${RUN_OUTPUT}" '"up_mbps":100' "hysteria2 inbound payload should include a default upstream bandwidth"
    assert_contains "${RUN_OUTPUT}" '"down_mbps":100' "hysteria2 inbound payload should include a default downstream bandwidth"
}

test_build_tls_payload_reality_uses_reality_keys_and_short_id() {
    run_build_tls_payload_check "reality"

    assert_eq "${RUN_STATUS}" "0" "build_tls_payload should build the reality preset"
    assert_contains "${RUN_OUTPUT}" '"name":"reality-template"' "reality payload should use the reality template name"
    assert_contains "${RUN_OUTPUT}" '"server":{"enabled":true,"server_name":"www.youtube.com","reality":{"enabled":true,"handshake":{"server":"www.youtube.com","server_port":443}' "reality payload should keep the frontend handshake defaults"
    assert_contains "${RUN_OUTPUT}" '"private_key":"reality-private"' "reality payload should include the fetched private key"
    assert_contains "${RUN_OUTPUT}" '"short_id":["0123abcd"]' "reality payload should include the configured short id"
    assert_contains "${RUN_OUTPUT}" '"client":{"utls":{"enabled":true,"fingerprint":"chrome"},"reality":{"enabled":true,"public_key":"reality-public","short_id":"0123abcd"}}' "reality payload should include the fetched public key and matching client short id"
}

test_run_protocol_bootstrap_fails_when_inbound_save_reports_failure() {
    run_protocol_bootstrap_failure_check

    assert_eq "${RUN_STATUS}" "1" "run_protocol_bootstrap should fail when inbound save reports success false"
    assert_contains "${RUN_OUTPUT}" 'rollback:clients:{"id":52}' "run_protocol_bootstrap should roll back the saved client when inbound creation fails"
    assert_contains "${RUN_OUTPUT}" 'rollback:tls:{"id":41}' "run_protocol_bootstrap should roll back the saved TLS template when inbound creation fails"
}

test_main_cleans_cookie_jar_even_when_bootstrap_fails() {
    run_failed_main_cleanup_check

    assert_eq "${RUN_STATUS}" "0" "failed main cleanup check should complete after capturing the failure"
    assert_contains "${RUN_OUTPUT}" "status:1" "main should still fail when protocol bootstrap fails"
    assert_contains "${RUN_OUTPUT}" "cookie:no" "main should remove the panel cookie jar on failure paths"
}

test_write_deploy_files_refuses_empty_port_mappings_without_side_effects() {
    run_empty_port_write_check

    assert_eq "${RUN_STATUS}" "0" "empty-port write check should complete after capturing the failure status"
    assert_contains "${RUN_OUTPUT}" "status:1" "write_deploy_files should fail without any port mappings"
    assert_contains "${RUN_OUTPUT}" "db:no" "write_deploy_files should not create the db directory on empty port input"
    assert_contains "${RUN_OUTPUT}" "cert:no" "write_deploy_files should not create the cert directory on empty port input"
    assert_contains "${RUN_OUTPUT}" "compose:no" "write_deploy_files should not create a compose file on empty port input"
}

test_script_can_be_sourced_without_running_main
test_unique_port_mappings_deduplicates_by_container_port
test_unique_port_mappings_supports_protocols_and_ranges
test_unique_port_mappings_treats_plain_ports_as_tcp
test_build_setting_args_includes_only_populated_values
test_collect_inputs_prompts_and_builds_standard_defaults
test_collect_inputs_skip_mode_clears_bootstrap_listener_state
test_collect_inputs_hysteria2_uses_udp_port_mapping
test_validate_bootstrap_inputs_requires_image_and_panel_port
test_validate_bootstrap_inputs_rejects_port_collisions
test_validate_bootstrap_inputs_rejects_invalid_port_values
test_validate_bootstrap_inputs_rejects_noninteractive_default_secrets
test_require_tools_rejects_missing_docker_compose
test_api_post_save_preserves_init_users_argument
test_direct_execution_bootstraps_startup_readiness_and_base_settings
test_render_compose_template_renders_bootstrap_compose
test_write_deploy_files_writes_compose_and_directories
test_render_compose_template_requires_at_least_one_port_mapping
test_fetch_tls_keypair_keeps_empty_server_name_query_empty
test_build_tls_payload_standard_uses_the_standard_template_shape
test_build_tls_payload_hysteria2_uses_h3_and_tls_1_3_defaults
test_build_tls_payload_reality_uses_reality_keys_and_short_id
test_build_inbound_payload_hysteria2_includes_bandwidth_defaults
test_run_protocol_bootstrap_fails_when_inbound_save_reports_failure
test_main_cleans_cookie_jar_even_when_bootstrap_fails
test_write_deploy_files_refuses_empty_port_mappings_without_side_effects

echo "PASS: install docker mode checks"
