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
INSTALL_ROOT="${INSTALL_ROOT:-/usr/local/s-ui}"
INSTALL_PARENT="$(dirname "${INSTALL_ROOT}")"
CLI_NAME="${CLI_NAME:-b-ui}"
CLI_PATH="${CLI_PATH:-/usr/bin/${CLI_NAME}}"
LEGACY_CLI_PATH="${LEGACY_CLI_PATH:-/usr/bin/s-ui}"
SERVICE_NAME="${SERVICE_NAME:-s-ui}"
DB_FILE="${INSTALL_ROOT}/db/s-ui.db"
BACKUP_ROOT="${BACKUP_ROOT:-/var/backups/s-ui}"
MODE="install"
TARGET_VERSION=""
EXISTING_INSTALL=0
CURRENT_BACKUP_DIR=""

cur_dir=$(pwd)

show_usage() {
    cat <<'EOF'
Usage:
  bash install.sh
  bash install.sh <version>
  bash install.sh --migrate [version]
  bash install.sh --update [version]
  bash install.sh --force-update [version]

Modes:
  default         Fresh install or manual reinstall. Existing installs keep data but still show setup prompts.
  --migrate       Migrate an existing upstream install in place, keep current settings, and switch the command to b-ui.
                  When used through migrate-to-b-ui.sh without a version, migration is followed by an explicit update check to the latest b-ui release.
  --update        Update an existing b-ui install only when the target version is newer/different.
  --force-update  Reinstall the target version even when the current version already matches.

Examples:
  bash install.sh
  bash install.sh v0.0.1
  bash install.sh --migrate
  bash install.sh --update
  bash install.sh --force-update v0.0.1
EOF
}

for arg in "$@"; do
    case "$arg" in
    -h | --help)
        show_usage
        exit 0
        ;;
    --migrate | --auto-migrate | --keep-config)
        MODE="migrate"
        ;;
    --update)
        MODE="update"
        ;;
    --force-update)
        MODE="force-update"
        ;;
    *)
        if [[ -z "$TARGET_VERSION" ]]; then
            TARGET_VERSION="$arg"
        else
            echo -e "${red}Fatal error: ${plain} too many arguments: $*"
            exit 1
        fi
        ;;
    esac
done

# check root
[[ $EUID -ne 0 ]] && echo -e "${red}Fatal error: ${plain} Please run this script with root privilege \n " && exit 1

# Check OS and set release variable
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

echo "arch: $(arch)"

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
    if [[ -d "${INSTALL_ROOT}" || -f "${DB_FILE}" || -f "/etc/systemd/system/${SERVICE_NAME}.service" || -f "${CLI_PATH}" || -f "${LEGACY_CLI_PATH}" ]]; then
        EXISTING_INSTALL=1
    fi
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
    for file in "${INSTALL_ROOT}"/db/s-ui.db*; do
        cp -f "${file}" "${CURRENT_BACKUP_DIR}/"
        copied=1
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

    if [[ -f "${CLI_PATH}" ]]; then
        cp -a "${CLI_PATH}" "${CURRENT_BACKUP_DIR}/${CLI_NAME}-cli"
    fi

    if [[ "${LEGACY_CLI_PATH}" != "${CLI_PATH}" && -f "${LEGACY_CLI_PATH}" ]]; then
        cp -a "${LEGACY_CLI_PATH}" "${CURRENT_BACKUP_DIR}/legacy-s-ui-cli"
    fi

    if [[ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]]; then
        cp -a "/etc/systemd/system/${SERVICE_NAME}.service" "${CURRENT_BACKUP_DIR}/${SERVICE_NAME}.service"
    fi

    backup_existing_db
    echo -e "${yellow}Created rollback backup in ${CURRENT_BACKUP_DIR}${plain}"
}

rollback_installation() {
    if [[ -z "${CURRENT_BACKUP_DIR}" || ! -d "${CURRENT_BACKUP_DIR}" ]]; then
        echo -e "${red}Rollback skipped: no backup directory is available.${plain}"
        return 1
    fi

    echo -e "${yellow}Install failed. Restoring the previous ${SERVICE_NAME} installation...${plain}"

    systemctl stop "${SERVICE_NAME}" 2>/dev/null || true

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
        cp -af "${CURRENT_BACKUP_DIR}/${SERVICE_NAME}.service" "/etc/systemd/system/${SERVICE_NAME}.service"
    fi

    if [[ -f "${INSTALL_ROOT}/sui" ]]; then
        chmod +x "${INSTALL_ROOT}/sui"
    fi

    systemctl daemon-reload
    systemctl enable "${SERVICE_NAME}" --now 2>/dev/null || systemctl start "${SERVICE_NAME}" 2>/dev/null || true

    if systemctl is-active --quiet "${SERVICE_NAME}"; then
        echo -e "${green}Rollback succeeded. Previous ${SERVICE_NAME} service is running again.${plain}"
        return 0
    fi

    echo -e "${red}Rollback completed, but ${SERVICE_NAME} did not start automatically. Please inspect ${CURRENT_BACKUP_DIR}.${plain}"
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
    local version_output=""
    local current_version=""

    if [[ -x "${INSTALL_ROOT}/sui" ]]; then
        version_output=$("${INSTALL_ROOT}/sui" -v 2>/dev/null | awk 'NR==1 {print $NF}')
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

    if [[ ${EXISTING_INSTALL} -ne 1 ]]; then
        echo -e "${yellow}No existing ${SERVICE_NAME} installation was detected. Continuing with a fresh install.${plain}"
        return 0
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

    if [[ "${MODE}" == "update" && -n "${normalized_current}" && "${normalized_current}" == "${normalized_target}" ]]; then
        echo -e "${green}${PROJECT_NAME} is already on ${TARGET_VERSION}. Nothing to do.${plain}"
        exit 0
    fi
}

download_release_asset() {
    local asset_name="b-ui-linux-$(arch).tar.gz"
    local download_url=""

    download_url="${RELEASE_BASE_URL}/download/${TARGET_VERSION}/${asset_name}"
    echo -e "Beginning the installation of ${PROJECT_NAME} ${TARGET_VERSION}..."

    wget --no-check-certificate -O "/tmp/${asset_name}" "${download_url}"
    if [[ $? -ne 0 ]]; then
        echo -e "${red}Downloading ${PROJECT_NAME} failed.${plain}"
        echo -e "${yellow}Tried: ${download_url}${plain}"
        echo -e "${yellow}Please verify that the release asset ${asset_name} exists under ${REPO_OWNER}/${REPO_NAME}.${plain}"
        exit 1
    fi
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

resolve_package_dir() {
    if [[ -d "/tmp/b-ui" ]]; then
        printf '%s\n' "/tmp/b-ui"
        return 0
    fi

    if [[ -d "/tmp/s-ui" ]]; then
        printf '%s\n' "/tmp/s-ui"
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

config_after_install() {
    echo -e "${yellow}Migration... ${plain}"
    "${INSTALL_ROOT}/sui" migrate
    if [[ $? -ne 0 ]]; then
        echo -e "${red}Database migration failed.${plain}"
        rollback_installation
        exit 1
    fi

    if [[ "${MODE}" != "install" && ${EXISTING_INSTALL} -eq 1 ]]; then
        echo -e "${green}Detected an existing compatible installation. Current settings and credentials have been kept.${plain}"
        return 0
    fi
    
    echo -e "${yellow}Install/update finished! For security it's recommended to modify panel settings ${plain}"
    read -p "Do you want to continue with the modification [y/n]? ": config_confirm
    if [[ "${config_confirm}" == "y" || "${config_confirm}" == "Y" ]]; then
        echo -e "Enter the ${yellow}panel port${plain} (leave blank for existing/default value):"
        read config_port
        echo -e "Enter the ${yellow}panel path${plain} (leave blank for existing/default value):"
        read config_path

        # Sub configuration
        echo -e "Enter the ${yellow}subscription port${plain} (leave blank for existing/default value):"
        read config_subPort
        echo -e "Enter the ${yellow}subscription path${plain} (leave blank for existing/default value):" 
        read config_subPath

        # Set configs
        echo -e "${yellow}Initializing, please wait...${plain}"
        params=""
        [ -z "$config_port" ] || params="$params -port $config_port"
        [ -z "$config_path" ] || params="$params -path $config_path"
        [ -z "$config_subPort" ] || params="$params -subPort $config_subPort"
        [ -z "$config_subPath" ] || params="$params -subPath $config_subPath"
        "${INSTALL_ROOT}/sui" setting ${params}

        read -p "Do you want to change admin credentials [y/n]? ": admin_confirm
        if [[ "${admin_confirm}" == "y" || "${admin_confirm}" == "Y" ]]; then
            # First admin credentials
            read -p "Please set up your username:" config_account
            read -p "Please set up your password:" config_password

            # Set credentials
            echo -e "${yellow}Initializing, please wait...${plain}"
            "${INSTALL_ROOT}/sui" admin -username ${config_account} -password ${config_password}
        else
            echo -e "${yellow}Your current admin credentials: ${plain}"
            "${INSTALL_ROOT}/sui" admin -show
        fi
    else
        echo -e "${red}cancel...${plain}"
        if [[ ! -f "${DB_FILE}" ]]; then
            local usernameTemp=$(head -c 6 /dev/urandom | base64)
            local passwordTemp=$(head -c 6 /dev/urandom | base64)
            echo -e "this is a fresh installation,will generate random login info for security concerns:"
            echo -e "###############################################"
            echo -e "${green}username:${usernameTemp}${plain}"
            echo -e "${green}password:${passwordTemp}${plain}"
            echo -e "###############################################"
            echo -e "${red}if you forgot your login info,you can type ${green}${CLI_NAME}${red} for configuration menu${plain}"
            "${INSTALL_ROOT}/sui" admin -username ${usernameTemp} -password ${passwordTemp}
        else
            echo -e "${red} this is your upgrade,will keep old settings,if you forgot your login info,you can type ${green}${CLI_NAME}${red} for configuration menu${plain}"
        fi
    fi
}

prepare_services() {
    if [[ -f "/etc/systemd/system/sing-box.service" ]]; then
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

    download_release_asset

    if [[ ${EXISTING_INSTALL} -eq 1 ]]; then
        echo -e "${yellow}Compatible legacy installation detected. ${PROJECT_NAME} will replace the binaries in place, keep the existing data directory, and switch the management command to ${CLI_NAME}.${plain}"
    fi

    if [[ -e "${INSTALL_ROOT}/" ]]; then
        systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
        backup_existing_installation
    fi

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

    chmod +x "${package_dir}/sui" "${cli_script_source}"
    mkdir -p "${INSTALL_ROOT}" || { rollback_installation; exit 1; }
    cp -rf "${package_dir}/." "${INSTALL_ROOT}/" || { rollback_installation; exit 1; }
    cp "${cli_script_source}" "${CLI_PATH}" || { rollback_installation; exit 1; }
    chmod +x "${CLI_PATH}" || { rollback_installation; exit 1; }
    remove_legacy_cli
    cp -f "${package_dir}"/*.service /etc/systemd/system/ || { rollback_installation; exit 1; }
    rm -rf "${package_dir}"

    config_after_install
    prepare_services

    restart_and_verify_service

    echo -e "${green}${PROJECT_NAME}${plain} installation finished, it is up and running now..."
    if [[ -n "${CURRENT_BACKUP_DIR}" ]]; then
        echo -e "${yellow}Rollback backup: ${CURRENT_BACKUP_DIR}${plain}"
    fi
    echo -e "You may access the Panel with following URL(s):${green}"
    "${INSTALL_ROOT}/sui" uri
    echo -e "${plain}"
    echo -e ""
    "${CLI_NAME}" help
}

echo -e "${green}Executing...${plain}"
detect_existing_install
resolve_target_version
check_update_requirement
install_base
install_app
