#!/usr/bin/env bash

set -euo pipefail

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_TEMPLATE="${SCRIPT_DIR}/templates/docker-compose.bootstrap.yml.tpl"
DEPLOY_DIR="${DEPLOY_DIR:-${PWD}/deploy}"
IMAGE_REF="${IMAGE_REF:-ghcr.io/beanya/b-ui:latest}"
CONTAINER_NAME="${CONTAINER_NAME:-b-ui}"
PANEL_PORT="${PANEL_PORT:-}"
PANEL_PATH="${PANEL_PATH:-}"
SUB_PORT="${SUB_PORT:-}"
SUB_PATH="${SUB_PATH:-}"
ADMIN_USERNAME="${ADMIN_USERNAME:-admin}"
ADMIN_PASSWORD="${ADMIN_PASSWORD:-admin}"
COOKIE_JAR="${COOKIE_JAR:-${DEPLOY_DIR}/panel-cookie.jar}"
PANEL_BASE_URL="${PANEL_BASE_URL:-}"
WAIT_RETRIES="${WAIT_RETRIES:-30}"
WAIT_DELAY="${WAIT_DELAY:-2}"
BOOTSTRAP_TLS_PRESET="${BOOTSTRAP_TLS_PRESET:-}"
BOOTSTRAP_CLIENT_NAME="${BOOTSTRAP_CLIENT_NAME:-docker-user}"
BOOTSTRAP_CLIENT_PASSWORD="${BOOTSTRAP_CLIENT_PASSWORD:-change-me}"
BOOTSTRAP_CLIENT_UUID="${BOOTSTRAP_CLIENT_UUID:-11111111-1111-4111-8111-111111111111}"
BOOTSTRAP_LISTEN="${BOOTSTRAP_LISTEN:-::}"
BOOTSTRAP_LISTEN_PORT="${BOOTSTRAP_LISTEN_PORT:-8443}"
BOOTSTRAP_SERVER_NAME="${BOOTSTRAP_SERVER_NAME:-}"
BOOTSTRAP_REALITY_SERVER_NAME="${BOOTSTRAP_REALITY_SERVER_NAME:-www.youtube.com}"
BOOTSTRAP_REALITY_SERVER_PORT="${BOOTSTRAP_REALITY_SERVER_PORT:-443}"
BOOTSTRAP_REALITY_SHORT_ID="${BOOTSTRAP_REALITY_SHORT_ID:-0123abcd}"

PORT_MAPPINGS=()
TLS_KEY_LINES=()
TLS_CERTIFICATE_LINES=()

container_port_key() {
    local mapping="$1"
    local protocol=""
    local target=""

    protocol="${mapping##*/}"
    if [[ "${protocol}" == "${mapping}" ]]; then
        protocol="tcp"
    else
        mapping="${mapping%/*}"
    fi

    target="${mapping##*:}"
    printf '%s/%s\n' "${target}" "${protocol}"
}

unique_port_mappings() {
    local mapping=""
    local container_port=""
    local -a deduped=()
    local -A seen_ports=()

    for mapping in "${PORT_MAPPINGS[@]}"; do
        container_port="$(container_port_key "${mapping}")"
        if [[ -n "${seen_ports[${container_port}]:-}" ]]; then
            continue
        fi

        seen_ports["${container_port}"]=1
        deduped+=("${mapping}")
    done

    PORT_MAPPINGS=("${deduped[@]}")
}

render_compose_template() {
    local template=""
    local rendered=""
    local port_lines=""
    local mapping=""

    if [[ ${#PORT_MAPPINGS[@]} -eq 0 ]]; then
        printf '%s\n' 'render_compose_template requires at least one port mapping.' >&2
        return 1
    fi

    template="$(<"${COMPOSE_TEMPLATE}")"

    for mapping in "${PORT_MAPPINGS[@]}"; do
        port_lines+="      - \"${mapping}\""$'\n'
    done
    port_lines="${port_lines%$'\n'}"

    rendered="${template//'${IMAGE_REF}'/${IMAGE_REF}}"
    rendered="${rendered//'${CONTAINER_NAME}'/${CONTAINER_NAME}}"
    rendered="${rendered//'${PANEL_PORT}'/${PANEL_PORT}}"
    rendered="${rendered//'${PANEL_PATH}'/${PANEL_PATH}}"
    rendered="${rendered//'${SUB_PORT}'/${SUB_PORT}}"
    rendered="${rendered//'${SUB_PATH}'/${SUB_PATH}}"
    rendered="${rendered//'${PORT_LINES}'/${port_lines}}"

    printf '%s\n' "${rendered}"
}

require_tools() {
    local tool=""

    for tool in docker curl; do
        if ! command -v "${tool}" >/dev/null 2>&1; then
            printf 'Missing required tool: %s\n' "${tool}" >&2
            return 1
        fi
    done

    if ! docker compose version >/dev/null 2>&1; then
        printf '%s\n' 'Missing required tool: docker compose' >&2
        return 1
    fi
}

normalize_path_value() {
    local value="$1"

    if [[ -n "${value}" && "${value}" != /* ]]; then
        value="/${value}"
    fi
    if [[ "${value}" == "/" ]]; then
        value=""
    fi

    printf '%s' "${value}"
}

prompt_with_default() {
    local var_name="$1"
    local prompt_label="$2"
    local default_value="${3:-}"
    local response=""
    local prompt_text="${prompt_label}"

    if [[ ! -t 0 ]]; then
        printf '%s' "${default_value}"
        return 0
    fi

    if [[ -n "${default_value}" ]]; then
        prompt_text+=" [${default_value}]"
    fi
    prompt_text+=": "

    read -r -p "${prompt_text}" response
    if [[ -z "${response}" ]]; then
        response="${default_value}"
    fi

    printf '%s' "${response}"
}

protocol_choice_default() {
    case "${BOOTSTRAP_TLS_PRESET}" in
        '') printf '%s' '1' ;;
        standard) printf '%s' '2' ;;
        reality) printf '%s' '3' ;;
        hysteria2) printf '%s' '4' ;;
        *) printf '%s' '1' ;;
    esac
}

collect_inputs() {
    local protocol_choice=""
    local protocol_default=""

    PANEL_PORT="$(prompt_with_default 'PANEL_PORT' 'Panel port' "${PANEL_PORT:-3000}")"
    PANEL_PATH="$(normalize_path_value "$(prompt_with_default 'PANEL_PATH' 'Panel path' "${PANEL_PATH:-/panel}")")"
    SUB_PORT="$(prompt_with_default 'SUB_PORT' 'Subscription port' "${SUB_PORT:-4000}")"
    SUB_PATH="$(normalize_path_value "$(prompt_with_default 'SUB_PATH' 'Subscription path' "${SUB_PATH:-/sub}")")"
    ADMIN_USERNAME="$(prompt_with_default 'ADMIN_USERNAME' 'Admin username' "${ADMIN_USERNAME:-admin}")"
    ADMIN_PASSWORD="$(prompt_with_default 'ADMIN_PASSWORD' 'Admin password' "${ADMIN_PASSWORD:-admin}")"

    protocol_default="$(protocol_choice_default)"
    protocol_choice="$(prompt_with_default 'BOOTSTRAP_PROTOCOL' 'Protocol bootstrap (1: skip, 2: VLESS + TLS, 3: VLESS + Reality, 4: Hysteria2)' "${protocol_default}")"

    case "${protocol_choice}" in
        ''|1)
            BOOTSTRAP_TLS_PRESET=""
            BOOTSTRAP_LISTEN_PORT=""
            BOOTSTRAP_SERVER_NAME=""
            BOOTSTRAP_CLIENT_NAME=""
            BOOTSTRAP_CLIENT_UUID=""
            BOOTSTRAP_CLIENT_PASSWORD=""
            BOOTSTRAP_REALITY_SERVER_NAME=""
            BOOTSTRAP_REALITY_SERVER_PORT=""
            BOOTSTRAP_REALITY_SHORT_ID=""
            ;;
        2)
            BOOTSTRAP_TLS_PRESET='standard'
            BOOTSTRAP_SERVER_NAME="$(prompt_with_default 'BOOTSTRAP_SERVER_NAME' 'TLS server name (leave empty for self-signed)' "${BOOTSTRAP_SERVER_NAME:-}")"
            BOOTSTRAP_CLIENT_NAME="$(prompt_with_default 'BOOTSTRAP_CLIENT_NAME' 'Client name' "${BOOTSTRAP_CLIENT_NAME:-docker-user}")"
            BOOTSTRAP_CLIENT_UUID="$(prompt_with_default 'BOOTSTRAP_CLIENT_UUID' 'Client UUID' "${BOOTSTRAP_CLIENT_UUID:-11111111-1111-4111-8111-111111111111}")"
            BOOTSTRAP_LISTEN_PORT="$(prompt_with_default 'BOOTSTRAP_LISTEN_PORT' 'Inbound listen port' "${BOOTSTRAP_LISTEN_PORT:-8443}")"
            ;;
        3)
            BOOTSTRAP_TLS_PRESET='reality'
            BOOTSTRAP_CLIENT_NAME="$(prompt_with_default 'BOOTSTRAP_CLIENT_NAME' 'Client name' "${BOOTSTRAP_CLIENT_NAME:-docker-user}")"
            BOOTSTRAP_CLIENT_UUID="$(prompt_with_default 'BOOTSTRAP_CLIENT_UUID' 'Client UUID' "${BOOTSTRAP_CLIENT_UUID:-11111111-1111-4111-8111-111111111111}")"
            BOOTSTRAP_LISTEN_PORT="$(prompt_with_default 'BOOTSTRAP_LISTEN_PORT' 'Inbound listen port' "${BOOTSTRAP_LISTEN_PORT:-8443}")"
            BOOTSTRAP_REALITY_SERVER_NAME="$(prompt_with_default 'BOOTSTRAP_REALITY_SERVER_NAME' 'Reality handshake server name' "${BOOTSTRAP_REALITY_SERVER_NAME:-www.youtube.com}")"
            BOOTSTRAP_REALITY_SERVER_PORT="$(prompt_with_default 'BOOTSTRAP_REALITY_SERVER_PORT' 'Reality handshake server port' "${BOOTSTRAP_REALITY_SERVER_PORT:-443}")"
            BOOTSTRAP_REALITY_SHORT_ID="$(prompt_with_default 'BOOTSTRAP_REALITY_SHORT_ID' 'Reality short id' "${BOOTSTRAP_REALITY_SHORT_ID:-0123abcd}")"
            ;;
        4)
            BOOTSTRAP_TLS_PRESET='hysteria2'
            BOOTSTRAP_SERVER_NAME="$(prompt_with_default 'BOOTSTRAP_SERVER_NAME' 'TLS server name (leave empty for self-signed)' "${BOOTSTRAP_SERVER_NAME:-}")"
            BOOTSTRAP_CLIENT_NAME="$(prompt_with_default 'BOOTSTRAP_CLIENT_NAME' 'Client name' "${BOOTSTRAP_CLIENT_NAME:-docker-user}")"
            BOOTSTRAP_CLIENT_PASSWORD="$(prompt_with_default 'BOOTSTRAP_CLIENT_PASSWORD' 'Client password' "${BOOTSTRAP_CLIENT_PASSWORD:-change-me}")"
            BOOTSTRAP_LISTEN_PORT="$(prompt_with_default 'BOOTSTRAP_LISTEN_PORT' 'Inbound listen port' "${BOOTSTRAP_LISTEN_PORT:-8443}")"
            ;;
        *)
            printf 'Unsupported protocol bootstrap option: %s\n' "${protocol_choice}" >&2
            return 1
            ;;
    esac

    PORT_MAPPINGS=()
    if [[ -n "${PANEL_PORT}" ]]; then
        PORT_MAPPINGS+=("${PANEL_PORT}:${PANEL_PORT}")
    fi
    if [[ -n "${SUB_PORT}" ]]; then
        PORT_MAPPINGS+=("${SUB_PORT}:${SUB_PORT}")
    fi
    if [[ -n "${BOOTSTRAP_LISTEN_PORT:-}" ]]; then
        PORT_MAPPINGS+=("$(port_mapping_for_bootstrap_listener)")
    fi

    if [[ -z "${PANEL_BASE_URL}" ]]; then
        PANEL_BASE_URL="http://127.0.0.1:${PANEL_PORT}${PANEL_PATH}"
    fi
}

validate_bootstrap_inputs() {
    local missing=0
    local invalid=0

    validate_port_number() {
        local port_name="$1"
        local port_value="$2"

        if [[ -z "${port_value}" ]]; then
            return 0
        fi

        if [[ ! "${port_value}" =~ ^[0-9]+$ ]] || (( port_value < 1 || port_value > 65535 )); then
            printf '%s must be an integer between 1 and 65535.\n' "${port_name}" >&2
            return 1
        fi

        return 0
    }

    if [[ -z "${IMAGE_REF}" ]]; then
        printf '%s\n' 'Missing required bootstrap input: IMAGE_REF' >&2
        missing=1
    fi

    if [[ -z "${PANEL_PORT}" ]]; then
        printf '%s\n' 'Missing required bootstrap input: PANEL_PORT' >&2
        missing=1
    fi

    if [[ ${missing} -eq 1 ]]; then
        return 1
    fi

    validate_port_number 'PANEL_PORT' "${PANEL_PORT}" || invalid=1
    validate_port_number 'SUB_PORT' "${SUB_PORT}" || invalid=1
    validate_port_number 'BOOTSTRAP_LISTEN_PORT' "${BOOTSTRAP_LISTEN_PORT:-}" || invalid=1

    if [[ -n "${SUB_PORT}" && "${SUB_PORT}" == "${PANEL_PORT}" ]]; then
        printf '%s\n' 'Bootstrap input error: SUB_PORT must not reuse PANEL_PORT.' >&2
        invalid=1
    fi

    if [[ -n "${BOOTSTRAP_TLS_PRESET:-}" && -n "${BOOTSTRAP_LISTEN_PORT:-}" ]]; then
        if [[ "${BOOTSTRAP_LISTEN_PORT}" == "${PANEL_PORT}" ]]; then
            printf '%s\n' 'Bootstrap input error: BOOTSTRAP_LISTEN_PORT must not reuse PANEL_PORT.' >&2
            invalid=1
        fi
        if [[ -n "${SUB_PORT}" && "${BOOTSTRAP_LISTEN_PORT}" == "${SUB_PORT}" ]]; then
            printf '%s\n' 'Bootstrap input error: BOOTSTRAP_LISTEN_PORT must not reuse SUB_PORT.' >&2
            invalid=1
        fi
    fi

    if [[ ${invalid} -eq 1 ]]; then
        return 1
    fi

    if [[ ! -t 0 ]]; then
        if [[ "${ADMIN_USERNAME}" == "admin" && "${ADMIN_PASSWORD}" == "admin" ]]; then
            printf '%s\n' 'Bootstrap input error: non-interactive runs must override the default admin credentials.' >&2
            return 1
        fi
        if [[ "${BOOTSTRAP_TLS_PRESET:-}" == "hysteria2" && "${BOOTSTRAP_CLIENT_PASSWORD}" == "change-me" ]]; then
            printf '%s\n' 'Bootstrap input error: non-interactive runs must override the default Hysteria2 client password.' >&2
            return 1
        fi
    fi
}

build_setting_args() {
    local -a args=()

    if [[ -n "${PANEL_PORT}" ]]; then
        args+=("-panelPort" "${PANEL_PORT}")
    fi
    if [[ -n "${PANEL_PATH}" ]]; then
        args+=("-panelPath" "${PANEL_PATH}")
    fi
    if [[ -n "${SUB_PORT}" ]]; then
        args+=("-subPort" "${SUB_PORT}")
    fi
    if [[ -n "${SUB_PATH}" ]]; then
        args+=("-subPath" "${SUB_PATH}")
    fi

    if [[ ${#args[@]} -gt 0 ]]; then
        printf '%s\n' "${args[@]}"
    fi
}

write_deploy_files() {
    local rendered=""

    unique_port_mappings
    rendered="$(render_compose_template)" || return 1
    mkdir -p "${DEPLOY_DIR}/db" "${DEPLOY_DIR}/cert"
    printf '%s\n' "${rendered}" >"${DEPLOY_DIR}/docker-compose.yml"
}

start_compose_stack() {
    docker compose -f "${DEPLOY_DIR}/docker-compose.yml" up -d
}

wait_for_panel() {
    local attempt=1

    while (( attempt <= WAIT_RETRIES )); do
        if curl --silent --show-error --fail --output /dev/null "${PANEL_BASE_URL}"; then
            return 0
        fi

        if (( attempt == WAIT_RETRIES )); then
            break
        fi

        sleep "${WAIT_DELAY}"
        attempt=$((attempt + 1))
    done

    printf 'Timed out waiting for panel readiness at %s\n' "${PANEL_BASE_URL}" >&2
    return 1
}

apply_base_settings() {
    local line=""
    local -a command=(docker compose -f "${DEPLOY_DIR}/docker-compose.yml" exec -T b-ui ./b-ui setting)

    while IFS= read -r line; do
        if [[ -n "${line}" ]]; then
            command+=("${line}")
        fi
    done < <(build_setting_args)

    if [[ ${#command[@]} -gt 8 ]]; then
        "${command[@]}"
    fi

    printf './b-ui admin -username %s -password %s\n' \
        "$(shell_quote "${ADMIN_USERNAME}")" \
        "$(shell_quote "${ADMIN_PASSWORD}")" | \
        docker compose -f "${DEPLOY_DIR}/docker-compose.yml" exec -T b-ui sh
}

login_panel() {
    local user_file=""
    local pass_file=""
    local status=0

    mkdir -p "${DEPLOY_DIR}"
    user_file="$(mktemp "${DEPLOY_DIR}/panel-user.XXXXXX")"
    pass_file="$(mktemp "${DEPLOY_DIR}/panel-pass.XXXXXX")"
    printf '%s' "${ADMIN_USERNAME}" >"${user_file}"
    printf '%s' "${ADMIN_PASSWORD}" >"${pass_file}"

    curl \
        --silent \
        --show-error \
        --fail \
        -c "${COOKIE_JAR}" \
        -X POST \
        --data-urlencode "user@${user_file}" \
        --data-urlencode "pass@${pass_file}" \
        "${PANEL_BASE_URL}/api/login" >/dev/null || status=$?

    rm -f "${user_file}" "${pass_file}"
    return ${status}
}

api_get_json() {
    local endpoint="$1"

    curl --silent --show-error --fail -b "${COOKIE_JAR}" "${PANEL_BASE_URL}${endpoint}"
}

api_post_save() {
    local object="$1"
    local action="$2"
    local payload="$3"
    local init_users="${4:-}"
    local payload_file=""
    local status=0
    local -a curl_args=()

    mkdir -p "${DEPLOY_DIR}"
    payload_file="$(mktemp "${DEPLOY_DIR}/api-payload.XXXXXX")"
    printf '%s' "${payload}" >"${payload_file}"

    curl_args=(
        --silent
        --show-error
        --fail
        -b "${COOKIE_JAR}"
        -X POST
        --data-urlencode "object=${object}"
        --data-urlencode "action=${action}"
        --data-urlencode "data@${payload_file}"
    )

    if [[ -n "${init_users}" ]]; then
        curl_args+=(--data-urlencode "initUsers=${init_users}")
    fi

    curl "${curl_args[@]}" "${PANEL_BASE_URL}/api/save" || status=$?

    rm -f "${payload_file}"
    return ${status}
}

json_escape() {
    local value="$1"

    value="${value//\\/\\\\}"
    value="${value//\"/\\\"}"
    value="${value//$'\n'/\\n}"
    printf '%s' "${value}"
}

shell_quote() {
    local value="$1"

    value="${value//\'/\'\\\'\'}"
    printf "'%s'" "${value}"
}

json_array_from_lines() {
    local line=""
    local first=1

    printf '['
    for line in "$@"; do
        if [[ ${first} -eq 0 ]]; then
            printf ','
        fi
        first=0
        printf '"%s"' "$(json_escape "${line}")"
    done
    printf ']'
}

extract_json_obj_lines() {
    local response="$1"
    local item=""
    local items=""

    if [[ "${response}" != *'"success":true'* ]]; then
        printf 'API request failed: %s\n' "${response}" >&2
        return 1
    fi

    items="$(printf '%s' "${response}" | sed -nE 's/.*"obj":\[([^]]*)\].*/\1/p')"
    if [[ -z "${items}" ]]; then
        printf '%s\n' 'Unable to extract API response payload.' >&2
        return 1
    fi

    while IFS= read -r item; do
        printf '%s\n' "${item}"
    done < <(printf '%s' "${items}" | grep -oE '"([^"\\]|\\.)*"' | sed -E 's/^"//; s/"$//')
}

split_tls_keypair() {
    local line=""
    local section=""

    TLS_KEY_LINES=()
    TLS_CERTIFICATE_LINES=()

    while IFS= read -r line; do
        case "${line}" in
            '-----BEGIN PRIVATE KEY-----')
                section='key'
                TLS_KEY_LINES+=("${line}")
                ;;
            '-----END PRIVATE KEY-----')
                TLS_KEY_LINES+=("${line}")
                section=''
                ;;
            '-----BEGIN CERTIFICATE-----')
                section='certificate'
                TLS_CERTIFICATE_LINES+=("${line}")
                ;;
            '-----END CERTIFICATE-----')
                TLS_CERTIFICATE_LINES+=("${line}")
                section=''
                ;;
            *)
                if [[ "${section}" == 'key' ]]; then
                    TLS_KEY_LINES+=("${line}")
                elif [[ "${section}" == 'certificate' ]]; then
                    TLS_CERTIFICATE_LINES+=("${line}")
                fi
                ;;
        esac
    done
}

response_is_success() {
    local response="$1"
    [[ "${response}" == *'"success":true'* ]]
}

require_success_response() {
    local response="$1"
    local label="$2"

    if response_is_success "${response}"; then
        return 0
    fi

    printf '%s failed: %s\n' "${label}" "${response}" >&2
    return 1
}

protocol_from_preset() {
    case "$1" in
        standard) printf '%s\n' 'vless' ;;
        hysteria2) printf '%s\n' 'hysteria2' ;;
        reality) printf '%s\n' 'vless' ;;
        *) return 1 ;;
    esac
}

port_mapping_for_bootstrap_listener() {
    if [[ -z "${BOOTSTRAP_LISTEN_PORT:-}" ]]; then
        return 0
    fi

    case "${BOOTSTRAP_TLS_PRESET:-}" in
        hysteria2)
            printf '%s\n' "${BOOTSTRAP_LISTEN_PORT}:${BOOTSTRAP_LISTEN_PORT}/udp"
            ;;
        *)
            printf '%s\n' "${BOOTSTRAP_LISTEN_PORT}:${BOOTSTRAP_LISTEN_PORT}"
            ;;
    esac
}

tls_name_from_preset() {
    case "$1" in
        standard) printf '%s\n' 'tls-template' ;;
        hysteria2) printf '%s\n' 'hysteria2-template' ;;
        reality) printf '%s\n' 'reality-template' ;;
        *) return 1 ;;
    esac
}

inbound_tag_from_preset() {
    local protocol=""

    protocol="$(protocol_from_preset "$1")" || return 1
    printf '%s-%s\n' "${protocol}" "${BOOTSTRAP_LISTEN_PORT}"
}

fetch_tls_keypair() {
    local server_name="${1:-}"
    local response=""

    response="$(api_get_json "/api/keypairs?k=tls&o=${server_name}")" || return 1
    require_success_response "${response}" 'TLS keypair fetch' || return 1
    extract_json_obj_lines "${response}"
}

fetch_reality_keypair() {
    local response=""

    response="$(api_get_json '/api/keypairs?k=reality')" || return 1
    require_success_response "${response}" 'Reality keypair fetch' || return 1
    extract_json_obj_lines "${response}"
}

build_tls_payload() {
    local preset="$1"
    local name=""
    local server_name="${BOOTSTRAP_SERVER_NAME}"
    local line=""
    local private_key=""
    local public_key=""

    name="$(tls_name_from_preset "${preset}")" || return 1

    case "${preset}" in
        standard|hysteria2)
            split_tls_keypair < <(fetch_tls_keypair "${server_name}")
            printf '{"id":0,"name":"%s","server":{"enabled":true,"server_name":"%s"' \
                "$(json_escape "${name}")" \
                "$(json_escape "${server_name}")"
            if [[ "${preset}" == 'standard' ]]; then
                printf ',"alpn":["h2","http/1.1"]'
            else
                printf ',"alpn":["h3"],"min_version":"1.3","max_version":"1.3"'
            fi
            printf ',"certificate":%s,"key":%s},"client":{"insecure":true}}\n' \
                "$(json_array_from_lines "${TLS_CERTIFICATE_LINES[@]}")" \
                "$(json_array_from_lines "${TLS_KEY_LINES[@]}")"
            ;;
        reality)
            while IFS= read -r line; do
                case "${line}" in
                    'PrivateKey: '*) private_key="${line#PrivateKey: }" ;;
                    'PublicKey: '*) public_key="${line#PublicKey: }" ;;
                esac
            done < <(fetch_reality_keypair)
            printf '{"id":0,"name":"%s","server":{"enabled":true,"server_name":"%s","reality":{"enabled":true,"handshake":{"server":"%s","server_port":%s},"private_key":"%s","short_id":["%s"]}},"client":{"utls":{"enabled":true,"fingerprint":"chrome"},"reality":{"enabled":true,"public_key":"%s","short_id":"%s"}}}\n' \
                "$(json_escape "${name}")" \
                "$(json_escape "${BOOTSTRAP_REALITY_SERVER_NAME}")" \
                "$(json_escape "${BOOTSTRAP_REALITY_SERVER_NAME}")" \
                "${BOOTSTRAP_REALITY_SERVER_PORT}" \
                "$(json_escape "${private_key}")" \
                "$(json_escape "${BOOTSTRAP_REALITY_SHORT_ID}")" \
                "$(json_escape "${public_key}")" \
                "$(json_escape "${BOOTSTRAP_REALITY_SHORT_ID}")"
            ;;
        *)
            printf 'Unsupported TLS preset: %s\n' "${preset}" >&2
            return 1
            ;;
    esac
}

build_client_payload() {
    local preset="$1"
    local config_key=""
    local config_payload=""

    case "${preset}" in
        standard)
            config_key='vless'
            config_payload=$(printf '{"name":"%s","uuid":"%s","flow":"xtls-rprx-vision"}' \
                "$(json_escape "${BOOTSTRAP_CLIENT_NAME}")" \
                "$(json_escape "${BOOTSTRAP_CLIENT_UUID}")")
            ;;
        hysteria2)
            config_key='hysteria2'
            config_payload=$(printf '{"name":"%s","password":"%s"}' \
                "$(json_escape "${BOOTSTRAP_CLIENT_NAME}")" \
                "$(json_escape "${BOOTSTRAP_CLIENT_PASSWORD}")")
            ;;
        reality)
            config_key='vless'
            config_payload=$(printf '{"name":"%s","uuid":"%s","flow":"xtls-rprx-vision"}' \
                "$(json_escape "${BOOTSTRAP_CLIENT_NAME}")" \
                "$(json_escape "${BOOTSTRAP_CLIENT_UUID}")")
            ;;
        *)
            printf 'Unsupported client preset: %s\n' "${preset}" >&2
            return 1
            ;;
    esac

    printf '{"enable":true,"name":"%s","config":{"%s":%s},"inbounds":[],"volume":0,"expiry":0,"up":0,"down":0,"desc":"","group":""}\n' \
        "$(json_escape "${BOOTSTRAP_CLIENT_NAME}")" \
        "${config_key}" \
        "${config_payload}"
}

build_inbound_payload() {
    local preset="$1"
    local tls_id="$2"
    local protocol=""
    local tag=""

    protocol="$(protocol_from_preset "${preset}")" || return 1
    tag="$(inbound_tag_from_preset "${preset}")" || return 1

    printf '{"id":0,"type":"%s","tag":"%s","tls_id":%s,"listen":"%s","listen_port":%s' \
        "${protocol}" \
        "$(json_escape "${tag}")" \
        "${tls_id}" \
        "$(json_escape "${BOOTSTRAP_LISTEN}")" \
        "${BOOTSTRAP_LISTEN_PORT}"
    if [[ "${preset}" == 'hysteria2' ]]; then
        printf ',"up_mbps":100,"down_mbps":100'
    fi
    printf '}\n'
}

extract_saved_id() {
    local response="$1"
    local collection="$2"
    local field="$3"
    local value="$4"
    local escaped_value=""

    escaped_value="$(printf '%s' "${value}" | sed -E 's/[][(){}.^$*+?|\\]/\\&/g')"

    printf '%s' "${response}" | sed -nE "s/.*\"${collection}\":\[[^]]*\"id\":([0-9]+),[^]]*\"${field}\":\"${escaped_value}\".*/\\1/p"
}

run_protocol_bootstrap() {
    local tls_payload=""
    local client_payload=""
    local inbound_payload=""
    local tls_response=""
    local client_response=""
    local tls_id=""
    local client_id=""
    local tls_name=""

    rollback_protocol_bootstrap() {
        if [[ -n "${client_id}" ]]; then
            api_post_save 'clients' 'del' "{\"id\":${client_id}}" >/dev/null || true
        fi
        if [[ -n "${tls_id}" ]]; then
            api_post_save 'tls' 'del' "{\"id\":${tls_id}}" >/dev/null || true
        fi
    }

    tls_name="$(tls_name_from_preset "${BOOTSTRAP_TLS_PRESET}")" || return 1
    tls_payload="$(build_tls_payload "${BOOTSTRAP_TLS_PRESET}")" || return 1
    tls_response="$(api_post_save 'tls' 'new' "${tls_payload}")" || return 1
    require_success_response "${tls_response}" 'TLS save' || return 1
    tls_id="$(extract_saved_id "${tls_response}" 'tls' 'name' "${tls_name}")"
    if [[ -z "${tls_id}" ]]; then
        printf 'Unable to extract saved TLS id from API response.\n' >&2
        return 1
    fi

    client_payload="$(build_client_payload "${BOOTSTRAP_TLS_PRESET}")" || return 1
    client_response="$(api_post_save 'clients' 'new' "${client_payload}")" || return 1
    require_success_response "${client_response}" 'Client save' || {
        rollback_protocol_bootstrap
        return 1
    }
    client_id="$(extract_saved_id "${client_response}" 'clients' 'name' "${BOOTSTRAP_CLIENT_NAME}")"
    if [[ -z "${client_id}" ]]; then
        printf 'Unable to extract saved client id from API response.\n' >&2
        rollback_protocol_bootstrap
        return 1
    fi

    inbound_payload="$(build_inbound_payload "${BOOTSTRAP_TLS_PRESET}" "${tls_id}")" || return 1
    require_success_response "$(api_post_save 'inbounds' 'new' "${inbound_payload}" "${client_id}")" 'Inbound save' || {
        rollback_protocol_bootstrap
        return 1
    }
}

print_final_output() {
    local operator_panel_url="http://<server-ip>:${PANEL_PORT}${PANEL_PATH}"

    printf '\nDocker bootstrap complete.\n'
    printf 'Panel URL: %s\n' "${operator_panel_url}"
    printf 'Compose file: %s\n' "${DEPLOY_DIR}/docker-compose.yml"
    if [[ -n "${BOOTSTRAP_TLS_PRESET}" ]]; then
        printf 'Protocol bootstrap: %s\n' "${BOOTSTRAP_TLS_PRESET}"
    else
        printf '%s\n' 'Protocol bootstrap: skipped'
    fi
}

cleanup_runtime_artifacts() {
    rm -f "${COOKIE_JAR}"
}

main() {
    local status=0

    require_tools || status=$?
    if [[ ${status} -eq 0 ]]; then
        collect_inputs || status=$?
    fi
    if [[ ${status} -eq 0 ]]; then
        validate_bootstrap_inputs || status=$?
    fi
    if [[ ${status} -eq 0 ]]; then
        write_deploy_files || status=$?
    fi
    if [[ ${status} -eq 0 ]]; then
        start_compose_stack || status=$?
    fi
    if [[ ${status} -eq 0 ]]; then
        wait_for_panel || status=$?
    fi
    if [[ ${status} -eq 0 ]]; then
        apply_base_settings || status=$?
    fi
    if [[ ${status} -eq 0 ]]; then
        login_panel || status=$?
    fi
    if [[ ${status} -eq 0 && -n "${BOOTSTRAP_TLS_PRESET}" ]]; then
        run_protocol_bootstrap || status=$?
    fi

    cleanup_runtime_artifacts

    if [[ ${status} -eq 0 ]]; then
        print_final_output
    fi

    return ${status}
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
