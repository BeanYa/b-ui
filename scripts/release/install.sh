#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

REPO_OWNER="${REPO_OWNER:-BeanYa}"
REPO_NAME="${REPO_NAME:-b-ui}"
PROJECT_NAME="${PROJECT_NAME:-B-UI}"
RELEASE_BASE_URL="https://github.com/${REPO_OWNER}/${REPO_NAME}/releases"
GITHUB_API_BASE_URL="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}"
SYSTEMD_DIR="${SYSTEMD_DIR:-/etc/systemd/system}"
INSTALL_ROOT="${INSTALL_ROOT:-/usr/local/b-ui}"
LEGACY_INSTALL_ROOT="${LEGACY_INSTALL_ROOT:-/usr/local/s-ui}"
INSTALL_PARENT="$(dirname "${INSTALL_ROOT}")"
CLI_NAME="${CLI_NAME:-b-ui}"
CLI_PATH="${CLI_PATH:-/usr/bin/${CLI_NAME}}"
BINARY_NAME="${BINARY_NAME:-b-ui}"
LEGACY_BINARY_NAME="${LEGACY_BINARY_NAME:-sui}"
BINARY_PATH="${BINARY_PATH:-${INSTALL_ROOT}/${BINARY_NAME}}"
LEGACY_BINARY_PATH="${LEGACY_BINARY_PATH:-${INSTALL_ROOT}/${LEGACY_BINARY_NAME}}"
LEGACY_CLI_PATH="${LEGACY_CLI_PATH:-/usr/bin/s-ui}"
SERVICE_NAME="${SERVICE_NAME:-b-ui}"
LEGACY_SERVICE_NAME="${LEGACY_SERVICE_NAME:-s-ui}"
INSTALL_COMMAND="bash <(curl -Ls https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/install.sh)"
MIGRATE_COMMAND="${INSTALL_COMMAND} --migrate"
FORCE_UPDATE_COMMAND="${INSTALL_COMMAND} --force-update"
DB_FILE="${DB_FILE:-${INSTALL_ROOT}/db/b-ui.db}"
LEGACY_DB_FILE="${LEGACY_DB_FILE:-${LEGACY_INSTALL_ROOT}/db/s-ui.db}"
BACKUP_ROOT="${BACKUP_ROOT:-/var/backups/b-ui}"
DOWNLOAD_RETRY_COUNT="${DOWNLOAD_RETRY_COUNT:-8}"
DOWNLOAD_RETRY_DELAY="${DOWNLOAD_RETRY_DELAY:-15}"
MODE="install"
TARGET_VERSION=""
EXISTING_INSTALL=0
INSTALLATION_KIND="none"
CURRENT_BACKUP_DIR=""
PREVIOUS_SERVICE_NAME=""
release=""

# Non-interactive installation parameters
ARG_USER=""
ARG_PWD=""
ARG_PANEL_PORT=""
ARG_PANEL_PATH=""
ARG_SUB_PORT=""
ARG_SUB_PATH=""
ARG_DOMAIN=""
ARG_CERT_PATH=""
ARG_KEY_PATH=""
ARG_ACME_PORT="80"

cur_dir=$(pwd)

show_usage() {
    cat <<'EOF'
Usage:
  bash install.sh [OPTIONS] [VERSION]
  bash install.sh --migrate [OPTIONS] [VERSION]
  bash install.sh --update [VERSION]
  bash install.sh --force-update [VERSION]

Modes:
  default         Fresh install or manual reinstall. Fresh installs use the b-ui command and b-ui service by default.
  --migrate       Migrate an existing upstream install in place, keep current settings, and switch both the service and command to b-ui.
                  When used through migrate-to-b-ui.sh without a version, migration is followed by an explicit update check to the latest b-ui release.
  --update        Update an existing b-ui install only when the target version is newer/different.
  --force-update  Reinstall the target version even when the current version already matches.

Options:
  --user            Admin username
  --pwd             Admin password
  --panel-port      Panel port (default: 2095)
  --panel-path      Panel security entry path (default: /app/)
  --sub-port        Subscription port (default: 2096)
  --sub-path        Subscription entry path (default: /sub/)
  --domain          Domain name for panel (if not provided, IP mode is used)
  --cert-path       SSL certificate file path (requires --key-path)
  --key-path        SSL key file path (requires --cert-path)
  --acme-port       Port for ACME standalone validation (default: 80)

  If --domain is provided without --cert-path and --key-path, ACME will be used
  to automatically apply for and renew the certificate.
  If --domain is not provided, IP mode is used by default.
  BBR optimization is enabled by default on fresh installs.

Examples:
  bash install.sh
  bash install.sh --panel-port 8080 --panel-path /admin/
  bash install.sh --domain example.com
  bash install.sh --domain example.com --cert-path /etc/certs/fullchain.pem --key-path /etc/certs/privkey.pem
  bash install.sh --user admin --pwd mypassword --panel-port 8080
  bash install.sh --migrate --domain example.com
  bash install.sh v0.0.1
EOF
}

parse_args() {
    MODE="install"
    TARGET_VERSION=""

    while [[ $# -gt 0 ]]; do
        case "$1" in
        -h | --help)
            show_usage
            exit 0
            ;;
        --migrate | --auto-migrate | --keep-config)
            MODE="migrate"
            shift
            ;;
        --update)
            MODE="update"
            shift
            ;;
        --force-update)
            MODE="force-update"
            shift
            ;;
        --user)
            ARG_USER="$2"
            shift 2
            ;;
        --pwd)
            ARG_PWD="$2"
            shift 2
            ;;
        --panel-port)
            ARG_PANEL_PORT="$2"
            shift 2
            ;;
        --panel-path)
            ARG_PANEL_PATH="$2"
            shift 2
            ;;
        --sub-port)
            ARG_SUB_PORT="$2"
            shift 2
            ;;
        --sub-path)
            ARG_SUB_PATH="$2"
            shift 2
            ;;
        --domain)
            ARG_DOMAIN="$2"
            shift 2
            ;;
        --cert-path)
            ARG_CERT_PATH="$2"
            shift 2
            ;;
        --key-path)
            ARG_KEY_PATH="$2"
            shift 2
            ;;
        --acme-port)
            ARG_ACME_PORT="$2"
            shift 2
            ;;
        --*)
            echo -e "${red}Fatal error: ${plain} unknown option: $1"
            show_usage
            exit 1
            ;;
        *)
            if [[ -z "${TARGET_VERSION}" ]]; then
                TARGET_VERSION="$1"
                shift
            else
                echo -e "${red}Fatal error: ${plain} too many arguments: $*"
                exit 1
            fi
            ;;
        esac
    done
}

validate_cert_params() {
    if [[ -n "${ARG_CERT_PATH}" && -z "${ARG_KEY_PATH}" ]]; then
        echo -e "${red}Fatal error: ${plain} --cert-path requires --key-path to also be provided"
        exit 1
    fi
    if [[ -z "${ARG_CERT_PATH}" && -n "${ARG_KEY_PATH}" ]]; then
        echo -e "${red}Fatal error: ${plain} --key-path requires --cert-path to also be provided"
        exit 1
    fi
    if [[ -n "${ARG_CERT_PATH}" && -n "${ARG_KEY_PATH}" ]]; then
        if [[ ! -f "${ARG_CERT_PATH}" ]]; then
            echo -e "${red}Fatal error: ${plain} certificate file not found: ${ARG_CERT_PATH}"
            exit 1
        fi
        if [[ ! -f "${ARG_KEY_PATH}" ]]; then
            echo -e "${red}Fatal error: ${plain} key file not found: ${ARG_KEY_PATH}"
            exit 1
        fi
    fi
}

enable_bbr() {
    if grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf && grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf; then
        echo -e "${green}BBR is already enabled!${plain}"
        return 0
    fi

    echo -e "${yellow}Enabling BBR optimization...${plain}"

    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update -yqq && apt-get install -yqq --no-install-recommends ca-certificates
        ;;
    centos | almalinux | rocky | oracle)
        yum -y update && yum -y install ca-certificates
        ;;
    fedora)
        dnf -y update && dnf -y install ca-certificates
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm ca-certificates
        ;;
    opensuse-tumbleweed)
        zypper refresh && zypper -q install -y ca-certificates
        ;;
    *)
        echo -e "${yellow}Unsupported OS for automatic BBR setup. Skipping BBR.${plain}"
        return 0
        ;;
    esac

    echo "net.core.default_qdisc=fq" | tee -a /etc/sysctl.conf
    echo "net.ipv4.tcp_congestion_control=bbr" | tee -a /etc/sysctl.conf
    sysctl -p

    if [[ $(sysctl net.ipv4.tcp_congestion_control | awk '{print $3}') == "bbr" ]]; then
        echo -e "${green}BBR has been enabled successfully.${plain}"
    else
        echo -e "${yellow}BBR enable attempt completed. Please check your kernel supports BBR.${plain}"
    fi
}

install_acme() {
    echo -e "${yellow}Installing acme.sh...${plain}"
    curl -s https://get.acme.sh | sh
    if [[ $? -ne 0 ]]; then
        echo -e "${red}Failed to install acme.sh${plain}"
        return 1
    fi
    echo -e "${green}acme.sh installed successfully.${plain}"
    return 0
}

handle_acme_cert() {
    local domain="$1"
    local web_port="${2:-80}"

    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
        install_acme || return 1
    fi

    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update -yqq && apt-get install -yqq socat
        ;;
    centos | almalinux | rocky | oracle)
        yum -y install socat
        ;;
    fedora)
        dnf -y install socat
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm socat
        ;;
    opensuse-tumbleweed)
        zypper -q install -y socat
        ;;
    *)
        echo -e "${red}Unsupported OS for ACME. Please install socat manually.${plain}"
        return 1
        ;;
    esac

    local certPath="/root/cert/${domain}"
    if [[ ! -d "$certPath" ]]; then
        mkdir -p "$certPath"
    fi

    echo -e "${yellow}Issuing SSL certificate for ${domain}...${plain}"

    local currentCert
    currentCert=$(~/.acme.sh/acme.sh --list 2>/dev/null | tail -1 | awk '{print $1}')
    if [[ "${currentCert}" == "${domain}" ]]; then
        echo -e "${yellow}Certificate for ${domain} already exists, skipping issuance.${plain}"
    else
        ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
        ~/.acme.sh/acme.sh --issue -d "${domain}" --standalone --httpport "${web_port}"
        if [[ $? -ne 0 ]]; then
            echo -e "${red}Failed to issue certificate for ${domain}${plain}"
            rm -rf ~/.acme.sh/"${domain}"
            return 1
        fi
    fi

    ~/.acme.sh/acme.sh --installcert -d "${domain}" \
        --key-file "${certPath}/privkey.pem" \
        --fullchain-file "${certPath}/fullchain.pem"

    if [[ $? -ne 0 ]]; then
        echo -e "${red}Failed to install certificate for ${domain}${plain}"
        return 1
    fi

    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    chmod 755 "${certPath}"/*

    ARG_CERT_PATH="${certPath}/fullchain.pem"
    ARG_KEY_PATH="${certPath}/privkey.pem"

    echo -e "${green}Certificate issued and auto-renew enabled for ${domain}${plain}"
    echo -e "${green}  Cert: ${ARG_CERT_PATH}${plain}"
    echo -e "${green}  Key:  ${ARG_KEY_PATH}${plain}"
    return 0
}

require_root() {
    [[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege \n " && exit 1
}

detect_os_release() {
    if [[ -f /etc/os-release ]]; then
        source /etc/os-release
        release=$ID
    elif [[ -f /usr/lib/os-release ]]; then
        source /usr/lib/os-release
        release=$ID
    else
        echo "Failed to check the system OS, please contact the author!" >&2
        exit 1
    fi

    echo "The OS release is: $release"
}

arch() {
    case "$(uname -m)" in
    x86_64 | x64 | amd64) echo 'amd64' ;;
    i*86 | x86) echo '386' ;;
    armv8* | armv8 | arm64 | aarch64) echo 'arm64' ;;
    armv7* | armv7 | arm) echo 'armv7' ;;
    armv6* | armv6) echo 'armv6' ;;
    armv5* | armv5) echo 'armv5' ;;
    s390x) echo 's390x' ;;
    *) echo -e "${green}Unsupported CPU architecture! ${plain}" && rm -f install.sh && exit 1 ;;
    esac
}

normalize_version() {
    local version_value="$1"
    version_value=$(printf '%s' "${version_value}" | tr -d '\r\n[:space:]')
    version_value="${version_value#v}"
    printf '%s' "${version_value}"
}

canonicalize_release_tag() {
    local version_value=""
    version_value=$(normalize_version "$1")
    if [[ -n "${version_value}" ]]; then
        printf 'v%s\n' "${version_value}"
    fi
}

detect_existing_install() {
    PREVIOUS_SERVICE_NAME=""
    EXISTING_INSTALL=0
    INSTALLATION_KIND="none"

    if [[ -f "${SYSTEMD_DIR}/${SERVICE_NAME}.service" ]]; then
        PREVIOUS_SERVICE_NAME="${SERVICE_NAME}"
    elif [[ -f "${SYSTEMD_DIR}/${LEGACY_SERVICE_NAME}.service" ]]; then
        PREVIOUS_SERVICE_NAME="${LEGACY_SERVICE_NAME}"
    fi

    if [[ -d "${INSTALL_ROOT}" || -d "${LEGACY_INSTALL_ROOT}" || -f "${DB_FILE}" || -f "${LEGACY_DB_FILE}" || -f "${SYSTEMD_DIR}/${SERVICE_NAME}.service" || -f "${SYSTEMD_DIR}/${LEGACY_SERVICE_NAME}.service" || -f "${CLI_PATH}" || -f "${LEGACY_CLI_PATH}" ]]; then
        EXISTING_INSTALL=1
    fi

    if [[ -d "${INSTALL_ROOT}" || -f "${DB_FILE}" || -f "${SYSTEMD_DIR}/${SERVICE_NAME}.service" || -f "${CLI_PATH}" ]]; then
        INSTALLATION_KIND="b-ui"
        return 0
    fi

    if [[ -d "${LEGACY_INSTALL_ROOT}" || -f "${LEGACY_DB_FILE}" || -f "${SYSTEMD_DIR}/${LEGACY_SERVICE_NAME}.service" || -f "${LEGACY_CLI_PATH}" ]]; then
        INSTALLATION_KIND="legacy-only"
    fi
}

resolve_installed_binary_path() {
    if [[ -x "${BINARY_PATH}" ]]; then
        printf '%s\n' "${BINARY_PATH}"
        return 0
    fi

    if [[ -x "${LEGACY_BINARY_PATH}" ]]; then
        printf '%s\n' "${LEGACY_BINARY_PATH}"
        return 0
    fi

    return 1
}

version_is_gte() {
    local left_version=""
    local right_version=""
    local max_version=""

    left_version=$(normalize_version "$1")
    right_version=$(normalize_version "$2")

    if [[ -z "${left_version}" || -z "${right_version}" ]]; then
        return 1
    fi

    max_version=$(printf '%s\n%s\n' "${left_version}" "${right_version}" | sort -V | tail -n1)
    [[ "${max_version}" == "${left_version}" ]]
}

prepare_backup_dir() {
    if [[ -n "${CURRENT_BACKUP_DIR}" ]]; then
        return 0
    fi

    CURRENT_BACKUP_DIR="${BACKUP_ROOT}/$(date +%Y%m%d-%H%M%S)"
    mkdir -p "${CURRENT_BACKUP_DIR}"
}

backup_existing_db() {
    if [[ ! -d "${INSTALL_ROOT}/db" ]]; then
        return 0
    fi

    local copied=0

    prepare_backup_dir
    shopt -s nullglob
    for pattern in "${DB_FILE}"* "${LEGACY_DB_FILE}"*; do
        for file in ${pattern}; do
            cp -f "${file}" "${CURRENT_BACKUP_DIR}/"
            copied=1
        done
    done
    shopt -u nullglob

    if [[ ${copied} -eq 1 ]]; then
        echo -e "${yellow}Backed up existing database files to ${CURRENT_BACKUP_DIR}${plain}"
    fi
}

backup_existing_installation() {
    if [[ ${EXISTING_INSTALL} -ne 1 ]]; then
        return 0
    fi

    prepare_backup_dir

    if [[ -d "${INSTALL_ROOT}" ]]; then
        tar -czf "${CURRENT_BACKUP_DIR}/install-root.tar.gz" -C "${INSTALL_ROOT}" .
    fi

    if [[ "${LEGACY_INSTALL_ROOT}" != "${INSTALL_ROOT}" && -d "${LEGACY_INSTALL_ROOT}" ]]; then
        tar -czf "${CURRENT_BACKUP_DIR}/legacy-install-root.tar.gz" -C "${LEGACY_INSTALL_ROOT}" .
    fi

    if [[ -f "${CLI_PATH}" ]]; then
        cp -a "${CLI_PATH}" "${CURRENT_BACKUP_DIR}/${CLI_NAME}-cli"
    fi

    if [[ "${LEGACY_CLI_PATH}" != "${CLI_PATH}" && -f "${LEGACY_CLI_PATH}" ]]; then
        cp -a "${LEGACY_CLI_PATH}" "${CURRENT_BACKUP_DIR}/legacy-s-ui-cli"
    fi

    if [[ -f "${SYSTEMD_DIR}/${SERVICE_NAME}.service" ]]; then
        cp -a "${SYSTEMD_DIR}/${SERVICE_NAME}.service" "${CURRENT_BACKUP_DIR}/${SERVICE_NAME}.service"
    fi

    if [[ "${LEGACY_SERVICE_NAME}" != "${SERVICE_NAME}" && -f "${SYSTEMD_DIR}/${LEGACY_SERVICE_NAME}.service" ]]; then
        cp -a "${SYSTEMD_DIR}/${LEGACY_SERVICE_NAME}.service" "${CURRENT_BACKUP_DIR}/${LEGACY_SERVICE_NAME}.service"
    fi

    backup_existing_db
    echo -e "${yellow}Created rollback backup in ${CURRENT_BACKUP_DIR}${plain}"
}

rollback_installation() {
    if [[ -z "${CURRENT_BACKUP_DIR}" || ! -d "${CURRENT_BACKUP_DIR}" ]]; then
        echo -e "${red}Rollback skipped: no backup directory is available.${plain}"
        return 1
    fi

    echo -e "${yellow}Install failed. Restoring the previous service installation...${plain}"

    systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
    if [[ "${LEGACY_SERVICE_NAME}" != "${SERVICE_NAME}" ]]; then
        systemctl stop "${LEGACY_SERVICE_NAME}" 2>/dev/null || true
    fi

    if [[ -f "${CURRENT_BACKUP_DIR}/install-root.tar.gz" ]]; then
        rm -rf "${INSTALL_ROOT}"
        mkdir -p "${INSTALL_ROOT}"
        tar -xzf "${CURRENT_BACKUP_DIR}/install-root.tar.gz" -C "${INSTALL_ROOT}"
    fi

    if [[ -f "${CURRENT_BACKUP_DIR}/${CLI_NAME}-cli" ]]; then
        cp -af "${CURRENT_BACKUP_DIR}/${CLI_NAME}-cli" "${CLI_PATH}"
        chmod +x "${CLI_PATH}"
    fi

    if [[ -f "${CURRENT_BACKUP_DIR}/legacy-s-ui-cli" ]]; then
        cp -af "${CURRENT_BACKUP_DIR}/legacy-s-ui-cli" "${LEGACY_CLI_PATH}"
        chmod +x "${LEGACY_CLI_PATH}"
    fi

    if [[ -f "${CURRENT_BACKUP_DIR}/${SERVICE_NAME}.service" ]]; then
        cp -af "${CURRENT_BACKUP_DIR}/${SERVICE_NAME}.service" "${SYSTEMD_DIR}/${SERVICE_NAME}.service"
    else
        rm -f "${SYSTEMD_DIR}/${SERVICE_NAME}.service"
    fi

    if [[ "${LEGACY_SERVICE_NAME}" != "${SERVICE_NAME}" ]]; then
        if [[ -f "${CURRENT_BACKUP_DIR}/${LEGACY_SERVICE_NAME}.service" ]]; then
            cp -af "${CURRENT_BACKUP_DIR}/${LEGACY_SERVICE_NAME}.service" "${SYSTEMD_DIR}/${LEGACY_SERVICE_NAME}.service"
        else
            rm -f "${SYSTEMD_DIR}/${LEGACY_SERVICE_NAME}.service"
        fi
    fi

    [[ ! -f "${BINARY_PATH}" ]] || chmod +x "${BINARY_PATH}"
    [[ ! -f "${LEGACY_BINARY_PATH}" ]] || chmod +x "${LEGACY_BINARY_PATH}"

    systemctl daemon-reload
    local restored_service_name="${PREVIOUS_SERVICE_NAME:-${SERVICE_NAME}}"
    systemctl enable "${restored_service_name}" --now 2>/dev/null || systemctl start "${restored_service_name}" 2>/dev/null || true

    if systemctl is-active --quiet "${restored_service_name}"; then
        echo -e "${green}Rollback succeeded. Previous ${restored_service_name} service is running again.${plain}"
        return 0
    fi

    echo -e "${red}Rollback completed, but ${restored_service_name} did not start automatically. Please inspect ${CURRENT_BACKUP_DIR}.${plain}"
    return 1
}

restart_and_verify_service() {
    systemctl enable "${SERVICE_NAME}" --now
    if [[ $? -ne 0 ]]; then
        rollback_installation
        exit 1
    fi

    sleep 2
    if ! systemctl is-active --quiet "${SERVICE_NAME}"; then
        echo -e "${red}${SERVICE_NAME} failed to stay active after restart.${plain}"
        rollback_installation
        exit 1
    fi
}

resolve_latest_release_tag() {
    local api_response=""
    local resolved_tag=""

    api_response=$(curl -fsSL "${GITHUB_API_BASE_URL}/releases/latest" 2>/dev/null || true)
    resolved_tag=$(printf '%s\n' "${api_response}" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    if [[ -n "${resolved_tag}" ]]; then
        printf '%s\n' "${resolved_tag}"
        return 0
    fi

    api_response=$(curl -fsSL "${GITHUB_API_BASE_URL}/releases?per_page=20" 2>/dev/null || true)
    resolved_tag=$(printf '%s\n' "${api_response}" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    if [[ -n "${resolved_tag}" ]]; then
        printf '%s\n' "${resolved_tag}"
        return 0
    fi

    return 1
}

get_current_installed_version() {
    local current_binary=""
    local version_output=""
    local current_version=""

    current_binary=$(resolve_installed_binary_path || true)
    if [[ -n "${current_binary}" && -x "${current_binary}" ]]; then
        version_output=$("${current_binary}" -v 2>/dev/null | awk 'NR==1 {print $NF}')
        current_version=$(normalize_version "${version_output}")
        if [[ -n "${current_version}" ]]; then
            printf '%s\n' "${current_version}"
            return 0
        fi
    fi

    return 1
}

resolve_target_version() {
    local resolved_tag=""

    if [[ -n "${TARGET_VERSION}" ]]; then
        TARGET_VERSION=$(canonicalize_release_tag "${TARGET_VERSION}")
        return 0
    fi

    resolved_tag=$(resolve_latest_release_tag || true)
    if [[ -z "${resolved_tag}" ]]; then
        echo -e "${red}No GitHub release was found for ${REPO_OWNER}/${REPO_NAME}.${plain}"
        echo -e "${yellow}Installation cannot continue until a release is published.${plain}"
        exit 1
    fi

    TARGET_VERSION="${resolved_tag}"
}

check_update_requirement() {
    local current_version=""
    local normalized_current=""
    local normalized_target=""

    if [[ "${MODE}" != "update" && "${MODE}" != "force-update" ]]; then
        return 0
    fi

    if [[ "${INSTALLATION_KIND}" == "none" ]]; then
        echo -e "${red}System does not have b-ui installed. Run the install script first.${plain}"
        echo -e "${yellow}${INSTALL_COMMAND}${plain}"
        exit 1
    fi

    if [[ "${INSTALLATION_KIND}" == "legacy-only" ]]; then
        echo -e "${red}Detected legacy s-ui installation but b-ui is not installed. Run migration first.${plain}"
        echo -e "${yellow}${MIGRATE_COMMAND}${plain}"
        exit 1
    fi

    current_version=$(get_current_installed_version || true)
    normalized_current=$(normalize_version "${current_version}")
    normalized_target=$(normalize_version "${TARGET_VERSION}")

    if [[ -n "${normalized_current}" ]]; then
        echo -e "${yellow}Current installed version: ${normalized_current}${plain}"
    else
        echo -e "${yellow}Current installed version: unknown${plain}"
    fi
    echo -e "${yellow}Target version: ${TARGET_VERSION}${plain}"

    if [[ "${MODE}" == "update" && -n "${normalized_current}" ]] && version_is_gte "${normalized_current}" "${normalized_target}"; then
        echo -e "${green}${PROJECT_NAME} is already up to date. Use '${FORCE_UPDATE_COMMAND}' to reinstall anyway.${plain}"
        exit 0
    fi
}

download_release_asset() {
    local asset_name="b-ui-linux-$(arch).tar.gz"
    local download_url=""
    local attempt=1

    download_url="${RELEASE_BASE_URL}/download/${TARGET_VERSION}/${asset_name}"
    echo -e "Beginning the installation of ${PROJECT_NAME} ${TARGET_VERSION}..."

    while [[ ${attempt} -le ${DOWNLOAD_RETRY_COUNT} ]]; do
        rm -f "/tmp/${asset_name}"
        if wget --tries=1 --no-check-certificate -O "/tmp/${asset_name}" "${download_url}"; then
            return 0
        fi

        rm -f "/tmp/${asset_name}"
        if [[ ${attempt} -lt ${DOWNLOAD_RETRY_COUNT} ]]; then
            echo -e "${yellow}Release asset ${asset_name} is not reachable yet (attempt ${attempt}/${DOWNLOAD_RETRY_COUNT}). Retrying in ${DOWNLOAD_RETRY_DELAY}s...${plain}"
            sleep "${DOWNLOAD_RETRY_DELAY}"
        fi

        attempt=$((attempt + 1))
    done

    echo -e "${red}Downloading ${PROJECT_NAME} failed.${plain}"
    echo -e "${yellow}Tried: ${download_url}${plain}"
    echo -e "${yellow}Please verify that the release asset ${asset_name} exists under ${REPO_OWNER}/${REPO_NAME}, or retry after the release assets finish publishing.${plain}"
    exit 1
}

install_base() {
    case "${release}" in
    centos | almalinux | rocky | oracle)
        yum -y update && yum install -y -q wget curl tar tzdata
        ;;
    fedora)
        dnf -y update && dnf install -y -q wget curl tar tzdata
        ;;
    arch | manjaro | parch)
        pacman -Syu && pacman -Syu --noconfirm wget curl tar tzdata
        ;;
    opensuse-tumbleweed)
        zypper refresh && zypper -q install -y wget curl tar timezone
        ;;
    *)
        apt-get update && apt-get install -y -q wget curl tar tzdata
        ;;
    esac
}

remove_legacy_cli() {
    if [[ "${LEGACY_CLI_PATH}" != "${CLI_PATH}" && -e "${LEGACY_CLI_PATH}" ]]; then
        rm -f "${LEGACY_CLI_PATH}"
    fi
}

remove_legacy_binary() {
    if [[ "${LEGACY_BINARY_PATH}" != "${BINARY_PATH}" && -e "${LEGACY_BINARY_PATH}" ]]; then
        rm -f "${LEGACY_BINARY_PATH}"
    fi
}

remove_legacy_service() {
    if [[ "${LEGACY_SERVICE_NAME}" == "${SERVICE_NAME}" ]]; then
        return 0
    fi

    systemctl stop "${LEGACY_SERVICE_NAME}" 2>/dev/null || true
    systemctl disable "${LEGACY_SERVICE_NAME}" 2>/dev/null || true
    rm -f "${SYSTEMD_DIR}/${LEGACY_SERVICE_NAME}.service"
    systemctl reset-failed "${LEGACY_SERVICE_NAME}" 2>/dev/null || true
}

resolve_package_dir() {
    if [[ -d "/tmp/b-ui" ]]; then
        printf '%s\n' "/tmp/b-ui"
        return 0
    fi

    return 1
}

resolve_cli_script() {
    local package_dir="$1"

    if [[ -f "${package_dir}/b-ui.sh" ]]; then
        printf '%s\n' "${package_dir}/b-ui.sh"
        return 0
    fi

    if [[ -f "${package_dir}/s-ui.sh" ]]; then
        printf '%s\n' "${package_dir}/s-ui.sh"
        return 0
    fi

    return 1
}

normalize_package_binary() {
    local package_dir="$1"

    if [[ ! -f "${package_dir}/${BINARY_NAME}" && -f "${package_dir}/${LEGACY_BINARY_NAME}" ]]; then
        mv "${package_dir}/${LEGACY_BINARY_NAME}" "${package_dir}/${BINARY_NAME}"
    fi
}

resolve_package_binary() {
    local package_dir="$1"

    if [[ -f "${package_dir}/${BINARY_NAME}" ]]; then
        printf '%s\n' "${package_dir}/${BINARY_NAME}"
        return 0
    fi

    if [[ -f "${package_dir}/${LEGACY_BINARY_NAME}" ]]; then
        printf '%s\n' "${package_dir}/${LEGACY_BINARY_NAME}"
        return 0
    fi

    return 1
}

stage_legacy_database_for_migration() {
    if [[ "${MODE}" != "migrate" && "${INSTALLATION_KIND}" != "legacy-only" ]]; then
        return 0
    fi

    shopt -s nullglob
    local legacy_files=("${LEGACY_DB_FILE}"*)
    shopt -u nullglob
    if [[ ${#legacy_files[@]} -eq 0 ]]; then
        return 0
    fi

    mkdir -p "${INSTALL_ROOT}/db"
    for legacy_file in "${legacy_files[@]}"; do
        cp -af "${legacy_file}" "${INSTALL_ROOT}/db/$(basename "${legacy_file}")"
    done
}

copy_package_to_install_root() {
    local package_dir="$1"
    local item=""
    local name=""

    mkdir -p "${INSTALL_ROOT}" || return 1

    shopt -s dotglob nullglob
    for item in "${package_dir}"/*; do
        name="$(basename "${item}")"
        case "${name}" in
        db | *.db | *.db-wal | *.db-shm)
            continue
            ;;
        esac
        cp -rf "${item}" "${INSTALL_ROOT}/" || {
            shopt -u dotglob nullglob
            return 1
        }
    done
    shopt -u dotglob nullglob
}

config_after_install() {
    echo -e "${yellow}Migration... ${plain}"
    "${BINARY_PATH}" migrate
    if [[ $? -ne 0 ]]; then
        echo -e "${red}Database migration failed.${plain}"
        rollback_installation
        exit 1
    fi

    # For any existing b-ui/s-ui install, keep existing settings
    # but apply any explicitly provided parameter overrides
    if [[ ${EXISTING_INSTALL} -eq 1 ]]; then
        if [[ "${MODE}" == "migrate" || "${INSTALLATION_KIND}" == "legacy-only" ]]; then
            echo -e "${green}Detected an existing compatible installation. Current settings, credentials, and migrated database contents have been kept.${plain}"
        else
            echo -e "${green}Detected an existing b-ui installation. Current settings and credentials have been kept.${plain}"
        fi

        # Apply overrides if any params were provided in migrate/update mode
        if [[ -n "${ARG_PANEL_PORT}" || -n "${ARG_PANEL_PATH}" || -n "${ARG_SUB_PORT}" || -n "${ARG_SUB_PATH}" || -n "${ARG_DOMAIN}" || -n "${ARG_CERT_PATH}" || -n "${ARG_KEY_PATH}" ]]; then
            echo -e "${yellow}Applying provided parameter overrides...${plain}"
            apply_settings_params
        fi
        if [[ -n "${ARG_USER}" || -n "${ARG_PWD}" ]]; then
            apply_admin_params
        fi
        return 0
    fi

    # Handle ACME cert before DB init (when domain provided without cert/key)
    if [[ -n "${ARG_DOMAIN}" && -z "${ARG_CERT_PATH}" && -z "${ARG_KEY_PATH}" ]]; then
        handle_acme_cert "${ARG_DOMAIN}" "${ARG_ACME_PORT}" || {
            echo -e "${red}ACME certificate issuance failed.${plain}"
            rollback_installation
            exit 1
        }
    fi

    # Apply settings via CLI
    apply_settings_params

    # Apply admin credentials
    if [[ -n "${ARG_USER}" || -n "${ARG_PWD}" ]]; then
        apply_admin_params
    else
        # Fresh install without admin params: generate random credentials
        local usernameTemp
        local passwordTemp
        usernameTemp=$(head -c 6 /dev/urandom | base64)
        passwordTemp=$(head -c 6 /dev/urandom | base64)
        echo -e "${yellow}No admin credentials provided, generating random credentials:${plain}"
        echo -e "###############################################"
        echo -e "${green}username: ${usernameTemp}${plain}"
        echo -e "${green}password: ${passwordTemp}${plain}"
        echo -e "###############################################"
        echo -e "${red}If you forgot your login info, type ${green}${CLI_NAME}${red} for configuration menu${plain}"
        "${BINARY_PATH}" admin -username "${usernameTemp}" -password "${passwordTemp}"
    fi
}

apply_settings_params() {
    local params=""
    [[ -z "${ARG_PANEL_PORT}" ]] || params="${params} -port ${ARG_PANEL_PORT}"
    [[ -z "${ARG_PANEL_PATH}" ]] || params="${params} -path ${ARG_PANEL_PATH}"
    [[ -z "${ARG_SUB_PORT}" ]] || params="${params} -subPort ${ARG_SUB_PORT}"
    [[ -z "${ARG_SUB_PATH}" ]] || params="${params} -subPath ${ARG_SUB_PATH}"
    [[ -z "${ARG_DOMAIN}" ]] || params="${params} -domain ${ARG_DOMAIN}"
    [[ -z "${ARG_CERT_PATH}" ]] || params="${params} -certFile ${ARG_CERT_PATH}"
    [[ -z "${ARG_KEY_PATH}" ]] || params="${params} -keyFile ${ARG_KEY_PATH}"

    if [[ -n "${params}" ]]; then
        echo -e "${yellow}Applying settings...${plain}"
        "${BINARY_PATH}" setting ${params}
    else
        echo -e "${yellow}No custom settings provided, using defaults.${plain}"
    fi
}

apply_admin_params() {
    local user="${ARG_USER:-admin}"
    local pwd="${ARG_PWD:-admin}"
    echo -e "${yellow}Setting admin credentials...${plain}"
    "${BINARY_PATH}" admin -username "${user}" -password "${pwd}"
}

prepare_services() {
    if [[ -f "${SYSTEMD_DIR}/sing-box.service" ]]; then
        echo -e "${yellow}Stopping sing-box service... ${plain}"
        systemctl stop sing-box
        rm -f "${INSTALL_ROOT}/bin/sing-box" "${INSTALL_ROOT}/bin/runSingbox.sh" "${INSTALL_ROOT}/bin/signal"
    fi
    if [[ -e "${INSTALL_ROOT}/bin" ]]; then
        echo -e "###############################################################"
        echo -e "${green}${INSTALL_ROOT}/bin${red} directory exists yet!"
        echo -e "Please check the content and delete it manually after migration ${plain}"
        echo -e "###############################################################"
    fi
    systemctl daemon-reload
}

install_app() {
    cd /tmp/
    local package_dir=""
    local cli_script_source=""
    local package_binary_path=""

    download_release_asset

    if [[ "${MODE}" == "migrate" || "${INSTALLATION_KIND}" == "legacy-only" ]]; then
        echo -e "${yellow}Compatible legacy installation detected. ${PROJECT_NAME} will replace the binaries in place, migrate the legacy database to b-ui.db on first start, keep the existing data directory, and switch the service and command to b-ui.${plain}"
    fi

    if [[ ${EXISTING_INSTALL} -eq 1 ]]; then
        systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
        if [[ "${LEGACY_SERVICE_NAME}" != "${SERVICE_NAME}" ]]; then
            systemctl stop "${LEGACY_SERVICE_NAME}" 2>/dev/null || true
        fi
        backup_existing_installation
    fi

    rm -rf /tmp/b-ui
    tar zxvf b-ui-linux-$(arch).tar.gz
    if [[ $? -ne 0 ]]; then
        echo -e "${red}Failed to extract the downloaded package.${plain}"
        rollback_installation
        exit 1
    fi
    rm b-ui-linux-$(arch).tar.gz -f

    package_dir=$(resolve_package_dir) || {
        echo -e "${red}The extracted package did not contain a supported root directory.${plain}"
        rollback_installation
        exit 1
    }
    cli_script_source=$(resolve_cli_script "${package_dir}") || {
        echo -e "${red}The extracted package did not contain a supported management script.${plain}"
        rollback_installation
        exit 1
    }
    normalize_package_binary "${package_dir}"
    package_binary_path=$(resolve_package_binary "${package_dir}") || {
        echo -e "${red}The extracted package did not contain a supported application binary.${plain}"
        rollback_installation
        exit 1
    }

    chmod +x "${package_binary_path}" "${cli_script_source}"
    mkdir -p "${INSTALL_ROOT}" || { rollback_installation; exit 1; }
    copy_package_to_install_root "${package_dir}" || { rollback_installation; exit 1; }
    chmod +x "${BINARY_PATH}" || { rollback_installation; exit 1; }
    stage_legacy_database_for_migration
    cp "${cli_script_source}" "${CLI_PATH}" || { rollback_installation; exit 1; }
    chmod +x "${CLI_PATH}" || { rollback_installation; exit 1; }
    remove_legacy_binary
    remove_legacy_cli
    cp -f "${package_dir}"/*.service "${SYSTEMD_DIR}/" || { rollback_installation; exit 1; }
    remove_legacy_service
    rm -rf "${package_dir}"

    config_after_install
    prepare_services

    restart_and_verify_service

    echo -e "${green}${PROJECT_NAME}${plain} installation finished, it is up and running now..."
    if [[ -n "${CURRENT_BACKUP_DIR}" ]]; then
        echo -e "${yellow}Rollback backup: ${CURRENT_BACKUP_DIR}${plain}"
    fi
    echo -e "${yellow}Service name:${plain} ${SERVICE_NAME}"
    echo -e "${yellow}Management command:${plain} ${CLI_NAME}"
    echo -e "You may access the Panel with following URL(s):${green}"
    "${BINARY_PATH}" uri
    echo -e "${plain}"
    echo -e ""
    "${CLI_NAME}" help
}

main() {
    echo -e "${green}Executing...${plain}"
    parse_args "$@"
    validate_cert_params
    require_root
    detect_os_release
    echo "arch: $(arch)"
    detect_existing_install
    resolve_target_version
    check_update_requirement
    install_base
    enable_bbr
    install_app
}

if [[ "${BASH_SOURCE[0]}" == "$0" ]]; then
    main "$@"
fi
