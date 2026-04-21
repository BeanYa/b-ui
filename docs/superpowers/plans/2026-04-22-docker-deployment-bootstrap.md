# Docker Deployment Bootstrap Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Add a dedicated interactive Docker deployment installer that generates Compose files, starts B-UI, initializes panel/admin settings, and optionally bootstraps one protocol-ready TLS template, client, and inbound using the existing backend APIs.

**Architecture:** Keep Docker deployment fully separate from the existing bare-metal `install.sh` path by adding a new `scripts/release/install-docker.sh` entrypoint and a checked-in Compose template. Use the existing container image and entrypoint unchanged, use CLI commands for base settings and admin credentials, then use the existing `/api/login`, `/api/keypairs`, and `/api/save` flows to create TLS, client, and inbound objects in the same way the panel UI does. Cover the script with sourceable shell tests that validate argument handling, compose rendering, port deduplication, and protocol payload generation without requiring a live Docker daemon.

**Tech Stack:** Bash, Docker Compose, curl, jq, existing B-UI API endpoints, Markdown docs, existing shell test pattern in `tests/`

---

## File Structure

- Create: `scripts/release/install-docker.sh`
  Responsibility: interactive Docker deployment flow, Compose rendering, readiness checks, login/bootstrap API calls, protocol-specific payload generation, and final operator output.
- Create: `scripts/release/templates/docker-compose.bootstrap.yml.tpl`
  Responsibility: stable Compose template with environment placeholders for image, container name, mounts, and generated port mappings.
- Create: `tests/install-docker-mode.sh`
  Responsibility: isolated shell regression tests for sourceability, compose rendering, duplicate-port handling, protocol payload generation, and bootstrap mode gating.
- Modify: `README.md`
  Responsibility: add a short Docker deployment entrypoint and point readers to the manual for the interactive bootstrap path.
- Modify: `docs/manual.md`
  Responsibility: document the Docker bootstrap flow, generated files, protocol options, self-signed vs mounted certificate behavior, and the `IP:port` panel-access model.

## Task 1: Create the Docker Installer Test Harness

**Files:**
- Create: `tests/install-docker-mode.sh`
- Test: `tests/install-docker-mode.sh`

- [ ] **Step 1: Write the failing shell test for sourceability and compose rendering helpers**

```bash
#!/usr/bin/env bash

set -euo pipefail

ROOT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
DOCKER_INSTALL_SCRIPT="${ROOT_DIR}/scripts/release/install-docker.sh"

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

test_script_can_be_sourced_without_running_main() {
    local status=0

    set +e
    bash -lc 'source "'"${DOCKER_INSTALL_SCRIPT}"'"; declare -F render_compose_template >/dev/null; declare -F build_tls_payload >/dev/null' >/dev/null 2>&1
    status=$?
    set -e

    assert_eq "${status}" "0" "install-docker.sh should be sourceable for isolated shell tests"
}

test_compose_render_deduplicates_ports() {
    local output=""

    output="$(bash -lc '
        source "'"${DOCKER_INSTALL_SCRIPT}"'"
        PORT_MAPPINGS=("127.0.0.1:2095:2095" "2095:2095" "8443:8443")
        unique_port_mappings
        printf "%s\n" "${PORT_MAPPINGS[@]}"
    ')"

    assert_contains "${output}" "127.0.0.1:2095:2095" "deduplicated mappings should keep the first explicit panel mapping"
    assert_contains "${output}" "8443:8443" "deduplicated mappings should keep distinct inbound mappings"
}

test_script_can_be_sourced_without_running_main
test_compose_render_deduplicates_ports

echo "PASS: install docker mode checks"
```

- [ ] **Step 2: Run the test to verify it fails before implementation**

Run: `bash tests/install-docker-mode.sh`
Expected: FAIL because `scripts/release/install-docker.sh` does not exist yet.

- [ ] **Step 3: Create the minimal script skeleton required by the test**

```bash
#!/usr/bin/env bash

set -euo pipefail

PORT_MAPPINGS=()

unique_port_mappings() {
    local deduped=()
    local seen=()
    local mapping=""
    for mapping in "${PORT_MAPPINGS[@]}"; do
        local container_port="${mapping##*:}"
        if [[ " ${seen[*]} " == *" ${container_port} "* ]]; then
            continue
        fi
        deduped+=("${mapping}")
        seen+=("${container_port}")
    done
    PORT_MAPPINGS=("${deduped[@]}")
}

render_compose_template() {
    :
}

build_tls_payload() {
    :
}

main() {
    echo "Docker installer not implemented yet"
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
```

- [ ] **Step 4: Run the test to verify the skeleton now passes**

Run: `bash tests/install-docker-mode.sh`
Expected: PASS with `PASS: install docker mode checks`

- [ ] **Step 5: Commit the test harness and script skeleton**

```bash
git add tests/install-docker-mode.sh scripts/release/install-docker.sh
git commit -m "test: add docker installer harness"
```

## Task 2: Implement Deployment Rendering and Base Bootstrap Inputs

**Files:**
- Modify: `scripts/release/install-docker.sh`
- Create: `scripts/release/templates/docker-compose.bootstrap.yml.tpl`
- Test: `tests/install-docker-mode.sh`

- [ ] **Step 1: Add a failing test for Compose rendering and required placeholders**

```bash
test_render_compose_template_includes_expected_mounts_and_ports() {
    local temp_dir=""
    local output=""

    temp_dir="$(mktemp -d)"
    output="$(bash -lc '
        source "'"${DOCKER_INSTALL_SCRIPT}"'"
        DEPLOY_DIR="'"${temp_dir}"'"
        IMAGE_REF="ghcr.io/beanya/b-ui:latest"
        CONTAINER_NAME="b-ui"
        PORT_MAPPINGS=("2095:2095" "8443:8443")
        render_compose_template
    ')"

    assert_contains "${output}" "ghcr.io/beanya/b-ui:latest" "compose render should include the configured image"
    assert_contains "${output}" "./db:/app/db" "compose render should mount the database directory"
    assert_contains "${output}" "2095:2095" "compose render should include the panel port mapping"
    assert_contains "${output}" "8443:8443" "compose render should include the inbound port mapping"

    rm -rf "${temp_dir}"
}
```

- [ ] **Step 2: Run the shell test to confirm render coverage fails**

Run: `bash tests/install-docker-mode.sh`
Expected: FAIL because `render_compose_template` still returns nothing.

- [ ] **Step 3: Add the checked-in Compose template file**

```yaml
services:
  b-ui:
    image: ${IMAGE_REF}
    container_name: ${CONTAINER_NAME}
    hostname: ${CONTAINER_NAME}
    volumes:
      - ./db:/app/db
      - ./cert:/app/cert
    tty: true
    restart: unless-stopped
    ports:
${PORT_LINES}
    entrypoint: ./entrypoint.sh
```
```

- [ ] **Step 4: Implement prompt collection, directory setup, and Compose rendering**

```bash
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
COMPOSE_TEMPLATE="${SCRIPT_DIR}/templates/docker-compose.bootstrap.yml.tpl"
DEPLOY_DIR=""
IMAGE_REF="ghcr.io/beanya/b-ui:latest"
CONTAINER_NAME="b-ui"
PANEL_PORT="2095"
PANEL_PATH=""
SUB_PORT="2096"
SUB_PATH=""

render_compose_template() {
    unique_port_mappings

    local port_lines=""
    local mapping=""
    for mapping in "${PORT_MAPPINGS[@]}"; do
        port_lines+="      - \"${mapping}\"\n"
    done

    IMAGE_REF="${IMAGE_REF}" \
    CONTAINER_NAME="${CONTAINER_NAME}" \
    PORT_LINES="${port_lines%\n}" \
    envsubst < "${COMPOSE_TEMPLATE}"
}

write_deploy_files() {
    mkdir -p "${DEPLOY_DIR}/db" "${DEPLOY_DIR}/cert"
    cat > "${DEPLOY_DIR}/docker-compose.yml" <<EOF
$(render_compose_template)
EOF
    cat > "${DEPLOY_DIR}/.env" <<EOF
IMAGE_REF=${IMAGE_REF}
CONTAINER_NAME=${CONTAINER_NAME}
PANEL_PORT=${PANEL_PORT}
PANEL_PATH=${PANEL_PATH}
SUB_PORT=${SUB_PORT}
SUB_PATH=${SUB_PATH}
EOF
}
```

- [ ] **Step 5: Re-run the shell test to verify Compose rendering passes**

Run: `bash tests/install-docker-mode.sh`
Expected: PASS with the new render assertion covered.

- [ ] **Step 6: Commit the deployment rendering layer**

```bash
git add scripts/release/install-docker.sh scripts/release/templates/docker-compose.bootstrap.yml.tpl tests/install-docker-mode.sh
git commit -m "feat: render docker bootstrap deployment files"
```

## Task 3: Implement Container Startup, Login, and Base Settings Bootstrap

**Files:**
- Modify: `scripts/release/install-docker.sh`
- Test: `tests/install-docker-mode.sh`

- [ ] **Step 1: Add a failing test for base settings argument rendering**

```bash
test_build_setting_args_includes_only_filled_values() {
    local output=""

    output="$(bash -lc '
        source "'"${DOCKER_INSTALL_SCRIPT}"'"
        PANEL_PORT="2095"
        PANEL_PATH="admin"
        SUB_PORT="2096"
        SUB_PATH="sub"
        build_setting_args
    ')"

    assert_contains "${output}" "-port 2095" "settings args should include the panel port"
    assert_contains "${output}" "-path admin" "settings args should include the panel path"
    assert_contains "${output}" "-subPort 2096" "settings args should include the subscription port"
    assert_contains "${output}" "-subPath sub" "settings args should include the subscription path"
}
```

- [ ] **Step 2: Run the shell test to confirm the base bootstrap helper is missing**

Run: `bash tests/install-docker-mode.sh`
Expected: FAIL because `build_setting_args` does not exist yet.

- [ ] **Step 3: Implement Docker prerequisite checks, startup, readiness polling, and CLI-based setting/admin bootstrap**

```bash
COOKIE_JAR=""
PANEL_BASE_URL=""

require_tools() {
    command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
    docker compose version >/dev/null 2>&1 || { echo "docker compose is required" >&2; exit 1; }
    command -v curl >/dev/null 2>&1 || { echo "curl is required" >&2; exit 1; }
    command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 1; }
}

build_setting_args() {
    local args=()
    [[ -n "${PANEL_PORT}" ]] && args+=("-port" "${PANEL_PORT}")
    [[ -n "${PANEL_PATH}" ]] && args+=("-path" "${PANEL_PATH}")
    [[ -n "${SUB_PORT}" ]] && args+=("-subPort" "${SUB_PORT}")
    [[ -n "${SUB_PATH}" ]] && args+=("-subPath" "${SUB_PATH}")
    printf '%s ' "${args[@]}"
}

start_compose_stack() {
    docker compose -f "${DEPLOY_DIR}/docker-compose.yml" up -d
}

wait_for_panel() {
    PANEL_BASE_URL="http://127.0.0.1:${PANEL_PORT}"
    local attempt=1
    while [[ ${attempt} -le 30 ]]; do
        if curl -fsS "${PANEL_BASE_URL}/api/status" >/dev/null 2>&1; then
            return 0
        fi
        sleep 2
        attempt=$((attempt + 1))
    done
    echo "panel did not become ready" >&2
    return 1
}

apply_base_settings() {
    local setting_args
    setting_args="$(build_setting_args)"
    docker compose -f "${DEPLOY_DIR}/docker-compose.yml" exec -T b-ui sh -lc "./sui setting ${setting_args}"
    docker compose -f "${DEPLOY_DIR}/docker-compose.yml" exec -T b-ui sh -lc "./sui admin -username '${ADMIN_USERNAME}' -password '${ADMIN_PASSWORD}'"
}
```

- [ ] **Step 4: Add login and authenticated API helpers for later bootstrap steps**

```bash
login_panel() {
    COOKIE_JAR="${DEPLOY_DIR}/cookies.txt"
    curl -fsS -c "${COOKIE_JAR}" -X POST \
        -d "user=${ADMIN_USERNAME}" \
        -d "pass=${ADMIN_PASSWORD}" \
        "${PANEL_BASE_URL}/api/login" >/dev/null
}

api_get_json() {
    local path="$1"
    curl -fsS -b "${COOKIE_JAR}" "${PANEL_BASE_URL}${path}"
}

api_post_save() {
    local object_name="$1"
    local action_name="$2"
    local data_json="$3"
    local init_users="${4:-}"

    curl -fsS -b "${COOKIE_JAR}" -X POST \
        --data-urlencode "object=${object_name}" \
        --data-urlencode "action=${action_name}" \
        --data-urlencode "data=${data_json}" \
        --data-urlencode "initUsers=${init_users}" \
        "${PANEL_BASE_URL}/api/save"
}
```

- [ ] **Step 5: Re-run the shell test to verify the base-bootstrap helpers pass**

Run: `bash tests/install-docker-mode.sh`
Expected: PASS with `build_setting_args` covered.

- [ ] **Step 6: Commit the startup and base bootstrap layer**

```bash
git add scripts/release/install-docker.sh tests/install-docker-mode.sh
git commit -m "feat: bootstrap docker panel settings and admin"
```

## Task 4: Implement Protocol Bootstrap Payloads and API Save Flow

**Files:**
- Modify: `scripts/release/install-docker.sh`
- Modify: `tests/install-docker-mode.sh`
- Reference: `src/frontend/src/plugins/tlsTemplates.ts`

- [ ] **Step 1: Add failing tests for TLS and Reality payload builders**

```bash
test_build_standard_tls_payload_uses_inline_certificate_mode() {
    local output=""

    output="$(bash -lc '
        source "'"${DOCKER_INSTALL_SCRIPT}"'"
        TLS_TEMPLATE_NAME="tls-template"
        TLS_SERVER_NAME="example.com"
        TLS_CERT_CONTENT="CERT_LINE"
        TLS_KEY_CONTENT="KEY_LINE"
        TLS_CERT_MODE="content"
        build_tls_payload standard
    ')"

    assert_contains "${output}" '"name": "tls-template"' "standard tls payload should include the generated template name"
    assert_contains "${output}" '"server_name": "example.com"' "standard tls payload should include the configured SNI"
    assert_contains "${output}" '"certificate": [' "standard tls payload should use inline certificate content when requested"
}

test_build_reality_tls_payload_includes_generated_keys() {
    local output=""

    output="$(bash -lc '
        source "'"${DOCKER_INSTALL_SCRIPT}"'"
        TLS_TEMPLATE_NAME="reality-template"
        TLS_SERVER_NAME="www.youtube.com"
        REALITY_PRIVATE_KEY="server-private"
        REALITY_PUBLIC_KEY="client-public"
        REALITY_SHORT_ID="abcd1234"
        REALITY_HANDSHAKE_SERVER="www.youtube.com"
        REALITY_HANDSHAKE_PORT="443"
        build_tls_payload reality
    ')"

    assert_contains "${output}" '"private_key": "server-private"' "reality payload should include the server private key"
    assert_contains "${output}" '"public_key": "client-public"' "reality payload should include the client public key"
    assert_contains "${output}" '"short_id": [' "reality payload should include a short id array"
}
```

- [ ] **Step 2: Run the shell test to verify the payload builders fail before implementation**

Run: `bash tests/install-docker-mode.sh`
Expected: FAIL because `build_tls_payload` is still a stub.

- [ ] **Step 3: Implement keypair retrieval helpers and TLS payload generation aligned with existing presets**

```bash
fetch_tls_keypair() {
    api_get_json "/api/keypairs?k=tls&o=${TLS_SERVER_NAME:-''}" | jq -r '.obj[]'
}

fetch_reality_keypair() {
    api_get_json "/api/keypairs?k=reality" | jq -r '.obj[]'
}

build_tls_payload() {
    local mode="$1"
    case "${mode}" in
        standard)
            if [[ "${TLS_CERT_MODE}" == "content" ]]; then
                jq -n \
                    --arg name "${TLS_TEMPLATE_NAME}" \
                    --arg server_name "${TLS_SERVER_NAME}" \
                    --arg cert "${TLS_CERT_CONTENT}" \
                    --arg key "${TLS_KEY_CONTENT}" '
                    {
                      id: 0,
                      name: $name,
                      server: {
                        enabled: true,
                        server_name: $server_name,
                        alpn: ["h2", "http/1.1"],
                        certificate: [$cert],
                        key: [$key]
                      },
                      client: {
                        insecure: true
                      }
                    }'
            else
                jq -n \
                    --arg name "${TLS_TEMPLATE_NAME}" \
                    --arg server_name "${TLS_SERVER_NAME}" \
                    --arg cert_path "${TLS_CERT_PATH}" \
                    --arg key_path "${TLS_KEY_PATH}" '
                    {
                      id: 0,
                      name: $name,
                      server: {
                        enabled: true,
                        server_name: $server_name,
                        alpn: ["h2", "http/1.1"],
                        certificate_path: $cert_path,
                        key_path: $key_path
                      },
                      client: {
                        insecure: true
                      }
                    }'
            fi
            ;;
        hysteria2)
            jq -n \
                --arg name "${TLS_TEMPLATE_NAME}" \
                --arg server_name "${TLS_SERVER_NAME}" \
                --arg cert_path "${TLS_CERT_PATH}" \
                --arg key_path "${TLS_KEY_PATH}" '
                {
                  id: 0,
                  name: $name,
                  server: {
                    enabled: true,
                    server_name: $server_name,
                    certificate_path: $cert_path,
                    key_path: $key_path
                  },
                  client: {
                    insecure: true
                  }
                }'
            ;;
        reality)
            jq -n \
                --arg name "${TLS_TEMPLATE_NAME}" \
                --arg server_name "${TLS_SERVER_NAME}" \
                --arg handshake_server "${REALITY_HANDSHAKE_SERVER}" \
                --argjson handshake_port "${REALITY_HANDSHAKE_PORT}" \
                --arg private_key "${REALITY_PRIVATE_KEY}" \
                --arg public_key "${REALITY_PUBLIC_KEY}" \
                --arg short_id "${REALITY_SHORT_ID}" '
                {
                  id: 0,
                  name: $name,
                  server: {
                    enabled: true,
                    server_name: $server_name,
                    reality: {
                      enabled: true,
                      handshake: {
                        server: $handshake_server,
                        server_port: $handshake_port
                      },
                      private_key: $private_key,
                      short_id: [$short_id]
                    }
                  },
                  client: {
                    utls: {
                      enabled: true,
                      fingerprint: "chrome"
                    },
                    reality: {
                      enabled: true,
                      public_key: $public_key,
                      short_id: $short_id
                    }
                  }
                }'
            ;;
    esac
}
```

- [ ] **Step 4: Implement client and inbound payload builders plus the bootstrap execution order**

```bash
build_client_payload() {
    jq -n \
        --arg name "${CLIENT_NAME}" \
        --argjson volume "${CLIENT_VOLUME}" \
        --argjson expiry "${CLIENT_EXPIRY}" \
        --argjson auto_reset "${CLIENT_AUTO_RESET}" \
        --argjson reset_days "${CLIENT_RESET_DAYS}" '
        {
          id: 0,
          enable: true,
          name: $name,
          config: {},
          inbounds: [],
          links: [],
          volume: $volume,
          expiry: $expiry,
          up: 0,
          down: 0,
          autoReset: $auto_reset,
          resetDays: $reset_days
        }'
}

build_inbound_payload() {
    jq -n \
        --arg type "${BOOTSTRAP_PROTOCOL_TYPE}" \
        --arg tag "${INBOUND_TAG}" \
        --argjson tls_id "${BOOTSTRAP_TLS_ID}" \
        --argjson listen_port "${INBOUND_PORT}" \
        --arg public_addr "${PUBLIC_SERVER_ADDR}" '
        {
          id: 0,
          type: $type,
          tag: $tag,
          tls_id: $tls_id,
          listen: "::",
          listen_port: $listen_port,
          addrs: [
            {
              addr: $public_addr
            }
          ]
        }'
}

run_protocol_bootstrap() {
    local tls_payload client_payload inbound_payload tls_response client_response

    tls_payload="$(build_tls_payload "${BOOTSTRAP_TLS_MODE}")"
    tls_response="$(api_post_save tls new "${tls_payload}")"
    BOOTSTRAP_TLS_ID="$(printf '%s' "${tls_response}" | jq -r '.obj.tls[0].id')"

    client_payload="$(build_client_payload)"
    client_response="$(api_post_save clients new "${client_payload}")"
    BOOTSTRAP_CLIENT_ID="$(printf '%s' "${client_response}" | jq -r '.obj.clients[0].id')"

    inbound_payload="$(build_inbound_payload)"
    api_post_save inbounds new "${inbound_payload}" "${BOOTSTRAP_CLIENT_ID}" >/dev/null
}
```

- [ ] **Step 5: Re-run the shell test to verify all protocol payload tests pass**

Run: `bash tests/install-docker-mode.sh`
Expected: PASS with TLS, Reality, and base helper assertions all succeeding.

- [ ] **Step 6: Commit the protocol bootstrap implementation**

```bash
git add scripts/release/install-docker.sh tests/install-docker-mode.sh
git commit -m "feat: bootstrap docker protocol presets"
```

## Task 5: Wire the Full Interactive Flow and Update User Docs

**Files:**
- Modify: `scripts/release/install-docker.sh`
- Modify: `README.md`
- Modify: `docs/manual.md`
- Test: `tests/install-docker-mode.sh`

- [ ] **Step 1: Add the interactive flow and final output wiring to the script**

```bash
prompt_with_default() {
    local label="$1"
    local default_value="$2"
    local value=""
    read -r -p "${label} [${default_value}]: " value
    printf '%s' "${value:-${default_value}}"
}

collect_inputs() {
    DEPLOY_DIR="$(prompt_with_default 'Deployment directory' './b-ui-docker')"
    IMAGE_REF="$(prompt_with_default 'Docker image' 'ghcr.io/beanya/b-ui:latest')"
    CONTAINER_NAME="$(prompt_with_default 'Container name' 'b-ui')"
    PANEL_PORT="$(prompt_with_default 'Panel port' '2095')"
    PANEL_PATH="$(prompt_with_default 'Panel path' '')"
    SUB_PORT="$(prompt_with_default 'Subscription port' '2096')"
    SUB_PATH="$(prompt_with_default 'Subscription path' '')"
    read -r -p 'Admin username: ' ADMIN_USERNAME
    read -r -p 'Admin password: ' ADMIN_PASSWORD
}

main() {
    require_tools
    collect_inputs
    write_deploy_files
    start_compose_stack
    wait_for_panel
    apply_base_settings
    login_panel
    if [[ "${ENABLE_PROTOCOL_BOOTSTRAP:-0}" == "1" ]]; then
        run_protocol_bootstrap
    fi
    echo "Panel: http://<server-ip>:${PANEL_PORT}${PANEL_PATH:+/${PANEL_PATH}}"
    echo "Subscription: http://<server-ip>:${SUB_PORT}${SUB_PATH:+/${SUB_PATH}}"
    echo "Compose file: ${DEPLOY_DIR}/docker-compose.yml"
}
```

- [ ] **Step 2: Add a short Docker entrypoint section to `README.md`**

```md
## Docker 部署

如果你希望用 GHCR 镜像完成交互式 Docker 初始化，请使用独立 Docker 安装脚本，而不是裸机 `install.sh`：

```sh
bash ./scripts/release/install-docker.sh
```

这个流程会：

- 生成本地 `docker-compose.yml` 和 `.env`
- 启动容器
- 初始化面板端口、订阅端口、管理员账号
- 可选创建一组 `VLESS + TLS`、`VLESS + Reality` 或 `Hysteria2` 的初始配置

默认面板访问方式是 `IP:端口`。如果你需要域名、HTTPS 或 Nginx 反代，请在宿主机自行配置。
```

- [ ] **Step 3: Add the full Docker workflow section to `docs/manual.md`**

```md
## Docker 部署

### 使用方式

```sh
bash ./scripts/release/install-docker.sh
```

### 生成结果

脚本会在你选择的目录中生成：

- `docker-compose.yml`
- `.env`
- `db/`
- `cert/`

### 初始化内容

脚本会先完成：

- 面板端口和路径
- 订阅端口和路径
- 管理员用户名和密码

然后你可以选择自动创建一套初始代理配置：

- `VLESS + TLS`
- `VLESS + Reality`
- `Hysteria2`

### 证书行为

`VLESS + TLS` 和 `Hysteria2` 支持两种证书来源：

- 自动生成自签证书内容，行为与 TLS 设置弹窗中的“生成”一致
- 使用挂载到容器内的证书路径

`VLESS + Reality` 不使用常规证书文件，而是自动生成 Reality 密钥对，行为与面板中的 Reality 生成按钮一致。

### 面板访问

脚本不会为面板自动配置域名、ACME 或 Nginx。默认请通过 `IP:端口` 访问，宿主机 HTTPS 和反代需要你自行配置。
```

- [ ] **Step 4: Run verification for the script test and docs accuracy**

Run: `bash tests/install-docker-mode.sh`
Expected: PASS with `PASS: install docker mode checks`

Run: `bash tests/install-update-mode.sh`
Expected: PASS with `PASS: install update mode checks`

- [ ] **Step 5: Commit the interactive flow and documentation updates**

```bash
git add scripts/release/install-docker.sh README.md docs/manual.md tests/install-docker-mode.sh
git commit -m "feat: add docker deployment bootstrap"
```

## Final Verification

- [ ] **Step 1: Validate the generated Compose file locally**

Run: `docker compose -f ./b-ui-docker/docker-compose.yml config`
Expected: merged Compose output with the configured image, `./db:/app/db`, `./cert:/app/cert`, and deduplicated port mappings.

- [ ] **Step 2: Run the new shell regression test suite**

Run: `bash tests/install-docker-mode.sh`
Expected: PASS

- [ ] **Step 3: Re-run the existing installer mode regression**

Run: `bash tests/install-update-mode.sh`
Expected: PASS

- [ ] **Step 4: Smoke-test the Docker bootstrap manually**

Run: `bash ./scripts/release/install-docker.sh`
Expected: the script writes deployment files, starts the container, sets panel/admin values, and prints an `http://<server-ip>:<panel-port>` panel URL.
