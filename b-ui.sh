#!/bin/bash

red='\033[0;31m'
green='\033[0;32m'
yellow='\033[0;33m'
plain='\033[0m'

REPO_OWNER="${REPO_OWNER:-BeanYa}"
REPO_NAME="${REPO_NAME:-b-ui}"
PROJECT_NAME="${PROJECT_NAME:-B-UI}"
INSTALL_SCRIPT_URL="https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/install.sh"
SCRIPT_RAW_URL="https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/main/b-ui.sh"
RELEASES_API_URL="https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases"
CLI_NAME="${CLI_NAME:-b-ui}"
CLI_PATH="${CLI_PATH:-/usr/bin/${CLI_NAME}}"
INSTALL_ROOT="${INSTALL_ROOT:-/usr/local/s-ui}"
SERVICE_NAME="${SERVICE_NAME:-b-ui}"
LEGACY_SERVICE_NAME="${LEGACY_SERVICE_NAME:-s-ui}"
DISPLAY_NAME="${DISPLAY_NAME:-B-UI}"

function LOGD() {
    echo -e "${yellow}[DEG] $* ${plain}"
}

function LOGE() {
    echo -e "${red}[ERR] $* ${plain}"
}

function LOGI() {
    echo -e "${green}[INF] $* ${plain}"
}

[[ $EUID -ne 0 ]] && LOGE "ERROR: You must be root to run this script! \n" && exit 1

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

confirm() {
    if [[ $# > 1 ]]; then
        echo && read -p "$1 [Default$2]: " temp
        if [[ x"${temp}" == x"" ]]; then
            temp=$2
        fi
    else
        read -p "$1 [y/n]: " temp
    fi
    if [[ x"${temp}" == x"y" || x"${temp}" == x"Y" ]]; then
        return 0
    else
        return 1
    fi
}

confirm_restart() {
    confirm "Restart the ${DISPLAY_NAME} service" "y"
    if [[ $? == 0 ]]; then
        restart
    else
        show_menu
    fi
}

before_show_menu() {
    echo && echo -n -e "${yellow}Press enter to return to the main menu: ${plain}" && read temp
    show_menu
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

get_latest_release_tag() {
    local api_response=""
    local resolved_tag=""

    api_response=$(curl -fsSL "${RELEASES_API_URL}/latest" 2>/dev/null || true)
    resolved_tag=$(printf '%s\n' "${api_response}" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)
    if [[ -n "${resolved_tag}" ]]; then
        printf '%s\n' "${resolved_tag}"
        return 0
    fi

    api_response=$(curl -fsSL "${RELEASES_API_URL}?per_page=20" 2>/dev/null || true)
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

run_remote_installer() {
    local install_mode="$1"
    local target_version="$2"

    if [[ -n "${target_version}" ]]; then
        bash <(curl -Ls "${INSTALL_SCRIPT_URL}") "${install_mode}" "${target_version}"
    else
        bash <(curl -Ls "${INSTALL_SCRIPT_URL}") "${install_mode}"
    fi
}

install() {
    bash <(curl -Ls "${INSTALL_SCRIPT_URL}")
    if [[ $? == 0 ]]; then
        if [[ $# == 0 ]]; then
            start
        else
            start 0
        fi
    fi
}

update() {
    local force_update=0
    local menu_mode=0
    local requested_version=""
    local current_version=""
    local target_version=""
    local normalized_current=""
    local normalized_target=""

    for arg in "$@"; do
        case "$arg" in
        --menu)
            menu_mode=1
            ;;
        --force | -f)
            force_update=1
            ;;
        --help | -h)
            echo "Usage: ${CLI_NAME} update [version] [--force]"
            echo "  ${CLI_NAME} update           Update to the latest published release if the version changed"
            echo "  ${CLI_NAME} update v0.0.1    Update to a specific release"
            echo "  ${CLI_NAME} update --force   Reinstall the latest release even when the version matches"
            return 0
            ;;
        *)
            if [[ -z "${requested_version}" ]]; then
                requested_version="$arg"
            else
                LOGE "Too many arguments"
                return 1
            fi
            ;;
        esac
    done

    current_version=$(get_current_installed_version || true)
    if [[ -n "${requested_version}" ]]; then
        target_version=$(canonicalize_release_tag "${requested_version}")
    else
        target_version=$(get_latest_release_tag || true)
    fi

    if [[ -z "${target_version}" ]]; then
        LOGE "Failed to detect the target release version from GitHub"
        if [[ ${menu_mode} -eq 1 ]]; then
            before_show_menu
        fi
        return 1
    fi

    normalized_current=$(normalize_version "${current_version}")
    normalized_target=$(normalize_version "${target_version}")

    if [[ -n "${normalized_current}" ]]; then
        LOGI "Current installed version: ${normalized_current}"
    else
        LOGI "Current installed version: unknown"
    fi
    LOGI "Target version: ${target_version}"

    if [[ ${force_update} -ne 1 && -n "${normalized_current}" && "${normalized_current}" == "${normalized_target}" ]]; then
        LOGI "${PROJECT_NAME} is already on ${target_version}. Use '${CLI_NAME} update --force' to reinstall the same version."
        if [[ ${menu_mode} -eq 1 ]]; then
            before_show_menu
        fi
        return 0
    fi

    if [[ ${force_update} -eq 1 ]]; then
        confirm "This will forcefully reinstall ${PROJECT_NAME} ${target_version} and overwrite local program files. Continue?" "n"
    else
        confirm "Update ${PROJECT_NAME} to ${target_version}? Current data will be kept." "y"
    fi
    if [[ $? != 0 ]]; then
        LOGE "Cancelled"
        if [[ ${menu_mode} -eq 1 ]]; then
            before_show_menu
        fi
        return 0
    fi

    if [[ ${force_update} -eq 1 ]]; then
        run_remote_installer --force-update "${target_version}"
    else
        run_remote_installer --update "${target_version}"
    fi
    if [[ $? == 0 ]]; then
        if [[ ${force_update} -eq 1 ]]; then
            LOGI "Forced update to ${PROJECT_NAME} ${target_version} completed"
        else
            LOGI "Update to ${PROJECT_NAME} ${target_version} completed"
        fi
        exit 0
    fi

    return 1
}

custom_version() {
    echo "Enter the panel version (like v0.0.1):"
    read panel_version

    if [ -z "$panel_version" ]; then
        echo "Panel version cannot be empty. Exiting."
    exit 1
    fi
    update --menu "$panel_version"
}

uninstall() {
    confirm "Are you sure you want to uninstall the panel?" "n"
    if [[ $? != 0 ]]; then
        if [[ $# == 0 ]]; then
            show_menu
        fi
        return 0
    fi
    systemctl stop "${SERVICE_NAME}"
    systemctl disable "${SERVICE_NAME}"
    rm "/etc/systemd/system/${SERVICE_NAME}.service" -f
    systemctl daemon-reload
    systemctl reset-failed
    rm -rf /etc/s-ui/
    rm -rf "${INSTALL_ROOT}/"

    echo ""
    echo -e "Uninstalled Successfully, If you want to remove this script, then after exiting the script run ${green}rm ${CLI_PATH} -f${plain} to delete it."
    echo ""

    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

reset_admin() {
    echo "It is not recommended to set admin's credentials to default!"
    confirm "Are you sure you want to reset admin's credentials to default ?" "n"
    if [[ $? == 0 ]]; then
        "${INSTALL_ROOT}/sui" admin -reset
    fi
    before_show_menu
}

set_admin() {
    echo "It is not recommended to set admin's credentials to a complex text."
    read -p "Please set up your username:" config_account
    read -p "Please set up your password:" config_password
    "${INSTALL_ROOT}/sui" admin -username ${config_account} -password ${config_password}
    before_show_menu
}

view_admin() {
    "${INSTALL_ROOT}/sui" admin -show
    before_show_menu
}

reset_setting() {
    confirm "Are you sure you want to reset settings to default ?" "n"
    if [[ $? == 0 ]]; then
        "${INSTALL_ROOT}/sui" setting -reset
    fi
    before_show_menu
}

set_setting() {
    echo -e "Enter the ${yellow}panel port${plain} (leave blank for existing/default value):"
    read config_port
    echo -e "Enter the ${yellow}panel path${plain} (leave blank for existing/default value):"
    read config_path

    echo -e "Enter the ${yellow}subscription port${plain} (leave blank for existing/default value):"
    read config_subPort
    echo -e "Enter the ${yellow}subscription path${plain} (leave blank for existing/default value):" 
    read config_subPath

    echo -e "${yellow}Initializing, please wait...${plain}"
    params=""
    [ -z "$config_port" ] || params="$params -port $config_port"
    [ -z "$config_path" ] || params="$params -path $config_path"
    [ -z "$config_subPort" ] || params="$params -subPort $config_subPort"
    [ -z "$config_subPath" ] || params="$params -subPath $config_subPath"
    "${INSTALL_ROOT}/sui" setting ${params}
    before_show_menu
}

view_setting() {
    "${INSTALL_ROOT}/sui" setting -show
    view_uri
    before_show_menu
}

view_uri() {
    info=$("${INSTALL_ROOT}/sui" uri)
    if [[ $? != 0 ]]; then
        LOGE "Get current uri error"
        before_show_menu
    fi
    LOGI "You may access the Panel with following URL(s):"
    echo -e "${green}${info}${plain}"
}

start() {
    check_status $1
    if [[ $? == 0 ]]; then
        echo ""
        LOGI -e "${DISPLAY_NAME} is already running. If you need to restart it, please select restart."
    else
        systemctl start $1
        sleep 2
        check_status $1
        if [[ $? == 0 ]]; then
            LOGI "${DISPLAY_NAME} started successfully"
        else
            LOGE "Failed to start ${DISPLAY_NAME}. It may take longer than two seconds to start; please check the logs later."
        fi
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

stop() {
    check_status $1
    if [[ $? == 1 ]]; then
        echo ""
        LOGI "${DISPLAY_NAME} is already stopped"
    else
        systemctl stop $1
        sleep 2
        check_status
        if [[ $? == 1 ]]; then
            LOGI "${DISPLAY_NAME} stopped successfully"
        else
            LOGE "Failed to stop ${DISPLAY_NAME}. The stop may have exceeded two seconds; please check the logs later."
        fi
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

restart() {
    systemctl restart $1
    sleep 2
    check_status $1
    if [[ $? == 0 ]]; then
        LOGI "${DISPLAY_NAME} restarted successfully"
    else
        LOGE "Failed to restart ${DISPLAY_NAME}. It may take longer than two seconds to start; please check the logs later."
    fi
    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

status() {
    systemctl status "${SERVICE_NAME}" -l
    if [[ $# == 0 ]]; then
        before_show_menu
    fi
}

enable() {
    systemctl enable $1
    if [[ $? == 0 ]]; then
        LOGI "${DISPLAY_NAME} autostart enabled successfully"
    else
        LOGE "Failed to enable ${DISPLAY_NAME} autostart"
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

disable() {
    systemctl disable $1
    if [[ $? == 0 ]]; then
        LOGI "${DISPLAY_NAME} autostart disabled successfully"
    else
        LOGE "Failed to disable ${DISPLAY_NAME} autostart"
    fi

    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

show_log() {
    journalctl -u $1.service -e --no-pager -f
    if [[ $# == 1 ]]; then
        before_show_menu
    fi
}

update_shell() {
    wget -O "${CLI_PATH}" --no-check-certificate "${SCRIPT_RAW_URL}"
    if [[ $? != 0 ]]; then
        echo ""
        LOGE "Failed to download script, Please check whether the machine can connect Github"
        before_show_menu
    else
        chmod +x "${CLI_PATH}"
        LOGI "Upgrade script succeeded, Please rerun the script" && exit 0
    fi
}

check_status() {
    if [[ ! -f "/etc/systemd/system/$1.service" ]]; then
        return 2
    fi
    temp=$(systemctl status "$1" | grep Active | awk '{print $3}' | cut -d "(" -f2 | cut -d ")" -f1)
    if [[ x"${temp}" == x"running" ]]; then
        return 0
    else
        return 1
    fi
}

resolve_service_name() {
    if [[ -f "/etc/systemd/system/${SERVICE_NAME}.service" ]]; then
        return 0
    fi

    if [[ "${LEGACY_SERVICE_NAME}" != "${SERVICE_NAME}" && -f "/etc/systemd/system/${LEGACY_SERVICE_NAME}.service" ]]; then
        SERVICE_NAME="${LEGACY_SERVICE_NAME}"
    fi
}

check_enabled() {
    temp=$(systemctl is-enabled $1)
    if [[ x"${temp}" == x"enabled" ]]; then
        return 0
    else
        return 1
    fi
}

check_uninstall() {
    check_status "${SERVICE_NAME}"
    if [[ $? != 2 ]]; then
        echo ""
        LOGE "Panel is already installed, Please do not reinstall"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

check_install() {
    check_status "${SERVICE_NAME}"
    if [[ $? == 2 ]]; then
        echo ""
        LOGE "Please install the panel first"
        if [[ $# == 0 ]]; then
            before_show_menu
        fi
        return 1
    else
        return 0
    fi
}

show_status() {
    check_status $1
    case $? in
    0)
        echo -e "${DISPLAY_NAME} state: ${green}Running${plain}"
        show_enable_status $1
        ;;
    1)
        echo -e "${DISPLAY_NAME} state: ${yellow}Not Running${plain}"
        show_enable_status $1
        ;;
    2)
        echo -e "${DISPLAY_NAME} state: ${red}Not Installed${plain}"
        ;;
    esac
}

show_enable_status() {
    check_enabled $1
    if [[ $? == 0 ]]; then
        echo -e "Start ${DISPLAY_NAME} automatically: ${green}Yes${plain}"
    else
        echo -e "Start ${DISPLAY_NAME} automatically: ${red}No${plain}"
    fi
}

check_panel_status() {
    count=$(ps -ef | grep "sui" | grep -v "grep" | wc -l)
    if [[ count -ne 0 ]]; then
        return 0
    else
        return 1
    fi
}

show_panel_status() {
    check_panel_status
    if [[ $? == 0 ]]; then
        echo -e "${CLI_NAME} state: ${green}Running${plain}"
    else
        echo -e "${CLI_NAME} state: ${red}Not Running${plain}"
    fi
}

bbr_menu() {
    echo -e "${green}\t1.${plain} Enable BBR"
    echo -e "${green}\t2.${plain} Disable BBR"
    echo -e "${green}\t0.${plain} Back to Main Menu"
    read -p "Choose an option: " choice
    case "$choice" in
    0)
        show_menu
        ;;
    1)
        enable_bbr
        ;;
    2)
        disable_bbr
        ;;
    *) echo "Invalid choice" ;;
    esac
}

disable_bbr() {
    if ! grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf || ! grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf; then
        echo -e "${yellow}BBR is not currently enabled.${plain}"
        exit 0
    fi
    sed -i 's/net.core.default_qdisc=fq/net.core.default_qdisc=pfifo_fast/' /etc/sysctl.conf
    sed -i 's/net.ipv4.tcp_congestion_control=bbr/net.ipv4.tcp_congestion_control=cubic/' /etc/sysctl.conf
    sysctl -p
    if [[ $(sysctl net.ipv4.tcp_congestion_control | awk '{print $3}') == "cubic" ]]; then
        echo -e "${green}BBR has been replaced with CUBIC successfully.${plain}"
    else
        echo -e "${red}Failed to replace BBR with CUBIC. Please check your system configuration.${plain}"
    fi
}

enable_bbr() {
    if grep -q "net.core.default_qdisc=fq" /etc/sysctl.conf && grep -q "net.ipv4.tcp_congestion_control=bbr" /etc/sysctl.conf; then
        echo -e "${green}BBR is already enabled!${plain}"
        exit 0
    fi
    case "${release}" in
    ubuntu | debian | armbian)
        apt-get update && apt-get install -yqq --no-install-recommends ca-certificates
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
    *)
        echo -e "${red}Unsupported operating system. Please check the script and install the necessary packages manually.${plain}\n"
        exit 1
        ;;
    esac
    echo "net.core.default_qdisc=fq" | tee -a /etc/sysctl.conf
    echo "net.ipv4.tcp_congestion_control=bbr" | tee -a /etc/sysctl.conf
    sysctl -p
    if [[ $(sysctl net.ipv4.tcp_congestion_control | awk '{print $3}') == "bbr" ]]; then
        echo -e "${green}BBR has been enabled successfully.${plain}"
    else
        echo -e "${red}Failed to enable BBR. Please check your system configuration.${plain}"
    fi
}

install_acme() {
    cd ~
    LOGI "install acme..."
    curl https://get.acme.sh | sh
    if [ $? -ne 0 ]; then
        LOGE "install acme failed"
        return 1
    else
        LOGI "install acme succeed"
    fi
    return 0
}

ssl_cert_issue_main() {
    echo -e "${green}\t1.${plain} Get SSL"
    echo -e "${green}\t2.${plain} Revoke"
    echo -e "${green}\t3.${plain} Force Renew"
    echo -e "${green}\t4.${plain} Self-signed Certificate"
    read -p "Choose an option: " choice
    case "$choice" in
        1) ssl_cert_issue ;;
        2) 
            local domain=""
            read -p "Please enter your domain name to revoke the certificate: " domain
            ~/.acme.sh/acme.sh --revoke -d ${domain}
            LOGI "Certificate revoked"
            ;;
        3)
            local domain=""
            read -p "Please enter your domain name to forcefully renew an SSL certificate: " domain
            ~/.acme.sh/acme.sh --renew -d ${domain} --force ;;
        4)
            generate_self_signed_cert
            ;;
        *) echo "Invalid choice" ;;
    esac
}

ssl_cert_issue() {
    if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
        echo "acme.sh could not be found. we will install it"
        install_acme
        if [ $? -ne 0 ]; then
            LOGE "install acme failed, please check logs"
            exit 1
        fi
    fi
    case "${release}" in
    ubuntu | debian | armbian)
        apt update && apt install socat -y
        ;;
    centos | almalinux | rocky | oracle)
        yum -y update && yum -y install socat
        ;;
    fedora)
        dnf -y update && dnf -y install socat
        ;;
    arch | manjaro | parch)
        pacman -Sy --noconfirm socat
        ;;
    *)
        echo -e "${red}Unsupported operating system. Please check the script and install the necessary packages manually.${plain}\n"
        exit 1
        ;;
    esac
    if [ $? -ne 0 ]; then
        LOGE "install socat failed, please check logs"
        exit 1
    else
        LOGI "install socat succeed..."
    fi

    local domain=""
    read -p "Please enter your domain name:" domain
    LOGD "your domain is:${domain},check it..."
    local currentCert=$(~/.acme.sh/acme.sh --list | tail -1 | awk '{print $1}')

    if [ ${currentCert} == ${domain} ]; then
        local certInfo=$(~/.acme.sh/acme.sh --list)
        LOGE "system already has certs here,can not issue again,current certs details:"
        LOGI "$certInfo"
        exit 1
    else
        LOGI "your domain is ready for issuing cert now..."
    fi

    certPath="/root/cert/${domain}"
    if [ ! -d "$certPath" ]; then
        mkdir -p "$certPath"
    else
        rm -rf "$certPath"
        mkdir -p "$certPath"
    fi

    local WebPort=80
    read -p "please choose which port do you use,default will be 80 port:" WebPort
    if [[ ${WebPort} -gt 65535 || ${WebPort} -lt 1 ]]; then
        LOGE "your input ${WebPort} is invalid,will use default port"
    fi
    LOGI "will use port:${WebPort} to issue certs,please make sure this port is open..."
    ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
    ~/.acme.sh/acme.sh --issue -d ${domain} --standalone --httpport ${WebPort}
    if [ $? -ne 0 ]; then
        LOGE "issue certs failed,please check logs"
        rm -rf ~/.acme.sh/${domain}
        exit 1
    else
        LOGE "issue certs succeed,installing certs..."
    fi
    ~/.acme.sh/acme.sh --installcert -d ${domain} \
        --key-file /root/cert/${domain}/privkey.pem \
        --fullchain-file /root/cert/${domain}/fullchain.pem

    if [ $? -ne 0 ]; then
        LOGE "install certs failed,exit"
        rm -rf ~/.acme.sh/${domain}
        exit 1
    else
        LOGI "install certs succeed,enable auto renew..."
    fi

    ~/.acme.sh/acme.sh --upgrade --auto-upgrade
    if [ $? -ne 0 ]; then
        LOGE "auto renew failed, certs details:"
        ls -lah cert/*
        chmod 755 $certPath/*
        exit 1
    else
        LOGI "auto renew succeed, certs details:"
        ls -lah cert/*
        chmod 755 $certPath/*
    fi
}

ssl_cert_issue_CF() {
    echo -E ""
    LOGD "******Instructions for use******"
    echo "1) New certificate from Cloudflare"
    echo "2) Force renew existing Certificates"
    echo "3) Back to Menu"
    read -p "Enter your choice [1-3]: " choice

    certPath="/root/cert-CF"

    case $choice in
        1|2)
            force_flag=""
            if [ "$choice" -eq 2 ]; then
                force_flag="--force"
                echo "Forcing SSL certificate reissuance..."
            else
                echo "Starting SSL certificate issuance..."
            fi
            
            LOGD "******Instructions for use******"
            LOGI "This Acme script requires the following data:"
            LOGI "1.Cloudflare Registered e-mail"
            LOGI "2.Cloudflare Global API Key"
            LOGI "3.The domain name that has been resolved DNS to the current server by Cloudflare"
            LOGI "4.The script applies for a certificate. The default installation path is /root/cert "
            confirm "Confirmed?[y/n]" "y"
            if [ $? -eq 0 ]; then
                if ! command -v ~/.acme.sh/acme.sh &>/dev/null; then
                    echo "acme.sh could not be found. Installing..."
                    install_acme
                    if [ $? -ne 0 ]; then
                        LOGE "Install acme failed, please check logs"
                        exit 1
                    fi
                fi

                CF_Domain=""
                if [ ! -d "$certPath" ]; then
                    mkdir -p $certPath
                else
                    rm -rf $certPath
                    mkdir -p $certPath
                fi

                LOGD "Please set a domain name:"
                read -p "Input your domain here: " CF_Domain
                LOGD "Your domain name is set to: ${CF_Domain}"

                CF_GlobalKey=""
                CF_AccountEmail=""
                LOGD "Please set the API key:"
                read -p "Input your key here: " CF_GlobalKey
                LOGD "Your API key is: ${CF_GlobalKey}"

                LOGD "Please set up registered email:"
                read -p "Input your email here: " CF_AccountEmail
                LOGD "Your registered email address is: ${CF_AccountEmail}"

                ~/.acme.sh/acme.sh --set-default-ca --server letsencrypt
                if [ $? -ne 0 ]; then
                    LOGE "Default CA, Let's Encrypt failed, script exiting..."
                    exit 1
                fi

                export CF_Key="${CF_GlobalKey}"
                export CF_Email="${CF_AccountEmail}"

                ~/.acme.sh/acme.sh --issue --dns dns_cf -d ${CF_Domain} -d *.${CF_Domain} $force_flag --log
                if [ $? -ne 0 ]; then
                    LOGE "Certificate issuance failed, script exiting..."
                    exit 1
                else
                    LOGI "Certificate issued Successfully, Installing..."
                fi

                mkdir -p ${certPath}/${CF_Domain}
                if [ $? -ne 0 ]; then
                    LOGE "Failed to create directory: ${certPath}/${CF_Domain}"
                    exit 1
                fi

                ~/.acme.sh/acme.sh --installcert -d ${CF_Domain} -d *.${CF_Domain} \
                    --fullchain-file ${certPath}/${CF_Domain}/fullchain.pem \
                    --key-file ${certPath}/${CF_Domain}/privkey.pem

                if [ $? -ne 0 ]; then
                    LOGE "Certificate installation failed, script exiting..."
                    exit 1
                else
                    LOGI "Certificate installed Successfully, Turning on automatic updates..."
                fi

                ~/.acme.sh/acme.sh --upgrade --auto-upgrade
                if [ $? -ne 0 ]; then
                    LOGE "Auto update setup failed, script exiting..."
                    exit 1
                else
                    LOGI "The certificate is installed and auto-renewal is turned on."
                    ls -lah ${certPath}/${CF_Domain}
                    chmod 755 ${certPath}/${CF_Domain}
                fi
            fi
            show_menu
            ;;
        3)
            echo "Exiting..."
            show_menu
            ;;
        *)
            echo "Invalid choice, please select again."
            show_menu
            ;;
    esac
}

generate_self_signed_cert() {
    cert_dir="/etc/sing-box"
    mkdir -p "$cert_dir"
    LOGI "Choose certificate type:"
    echo -e "${green}\t1.${plain} Ed25519 (*recommended*)"
    echo -e "${green}\t2.${plain} RSA 2048"
    echo -e "${green}\t3.${plain} RSA 4096"
    echo -e "${green}\t4.${plain} ECDSA prime256v1"
    echo -e "${green}\t5.${plain} ECDSA secp384r1"
    read -p "Enter your choice [1-5, default 1]: " cert_type
    cert_type=${cert_type:-1}

    case "$cert_type" in
        1)
            algo="ed25519"
            key_opt="-newkey ed25519"
            ;;
        2)
            algo="rsa"
            key_opt="-newkey rsa:2048"
            ;;
        3)
            algo="rsa"
            key_opt="-newkey rsa:4096"
            ;;
        4)
            algo="ecdsa"
            key_opt="-newkey ec -pkeyopt ec_paramgen_curve:prime256v1"
            ;;
        5)
            algo="ecdsa"
            key_opt="-newkey ec -pkeyopt ec_paramgen_curve:secp384r1"
            ;;
        *)
            algo="ed25519"
            key_opt="-newkey ed25519"
            ;;
    esac

    LOGI "Generating self-signed certificate ($algo)..."
    sudo openssl req -x509 -nodes -days 3650 $key_opt \
        -keyout "${cert_dir}/self.key" \
        -out "${cert_dir}/self.crt" \
        -subj "/CN=myserver"
    if [[ $? -eq 0 ]]; then
        sudo chmod 600 "${cert_dir}/self."*
        LOGI "Self-signed certificate generated successfully!"
        LOGI "Certificate path: ${cert_dir}/self.crt"
        LOGI "Key path: ${cert_dir}/self.key"
    else
        LOGE "Failed to generate self-signed certificate."
    fi
    before_show_menu
}

show_usage() {
    echo -e "B-UI Control Menu Usage"
    echo -e "------------------------------------------"
    echo -e "SUBCOMMANDS:" 
    echo -e "${CLI_NAME}              - Admin Management Script"
    echo -e "${CLI_NAME} start        - Start ${PROJECT_NAME}"
    echo -e "${CLI_NAME} stop         - Stop ${PROJECT_NAME}"
    echo -e "${CLI_NAME} restart      - Restart ${PROJECT_NAME}"
    echo -e "${CLI_NAME} status       - Current status of ${PROJECT_NAME}"
    echo -e "${CLI_NAME} enable       - Enable autostart on OS startup"
    echo -e "${CLI_NAME} disable      - Disable autostart on OS startup"
    echo -e "${CLI_NAME} log          - Check ${PROJECT_NAME} logs"
    echo -e "${CLI_NAME} update       - Update to the latest release when the version changed"
    echo -e "${CLI_NAME} update --force - Force reinstall the latest release"
    echo -e "${CLI_NAME} install      - Install"
    echo -e "${CLI_NAME} uninstall    - Uninstall"
    echo -e "${CLI_NAME} help         - Control menu usage"
    echo -e "------------------------------------------"
}

show_menu() {
  echo -e "
  ${green}B-UI Admin Management Script ${plain}
————————————————————————————————
  ${green}0.${plain} Exit
————————————————————————————————
  ${green}1.${plain} Install
  ${green}2.${plain} Update
  ${green}3.${plain} Custom Version
  ${green}4.${plain} Uninstall
————————————————————————————————
  ${green}5.${plain} Reset admin credentials to default
  ${green}6.${plain} Set admin credentials
  ${green}7.${plain} View admin credentials
————————————————————————————————
  ${green}8.${plain} Reset Panel Settings
  ${green}9.${plain} Set Panel settings
  ${green}10.${plain} View Panel Settings
————————————————————————————————
  ${green}11.${plain} B-UI Start
  ${green}12.${plain} B-UI Stop
  ${green}13.${plain} B-UI Restart
  ${green}14.${plain} B-UI Check State
  ${green}15.${plain} B-UI Check Logs
  ${green}16.${plain} B-UI Enable Autostart
  ${green}17.${plain} B-UI Disable Autostart
————————————————————————————————
  ${green}18.${plain} Enable or Disable BBR
  ${green}19.${plain} SSL Certificate Management
  ${green}20.${plain} Cloudflare SSL Certificate
————————————————————————————————
 "
    show_status "${SERVICE_NAME}"
    echo && read -p "Please enter your selection [0-20]: " num

    case "${num}" in
    0)
        exit 0
        ;;
    1)
        check_uninstall && install
        ;;
    2)
        check_install && update --menu
        ;;
    3)
        check_install && custom_version
        ;;
    4)
        check_install && uninstall
        ;;
    5)
        check_install && reset_admin
        ;;
    6)
        check_install && set_admin
        ;;
    7)
        check_install && view_admin
        ;;
    8)
        check_install && reset_setting
        ;;
    9)
        check_install && set_setting
        ;;
    10)
        check_install && view_setting
        ;;
    11)
        check_install && start "${SERVICE_NAME}"
        ;;
    12)
        check_install && stop "${SERVICE_NAME}"
        ;;
    13)
        check_install && restart "${SERVICE_NAME}"
        ;;
    14)
        check_install && status "${SERVICE_NAME}"
        ;;
    15)
        check_install && show_log "${SERVICE_NAME}"
        ;;
    16)
        check_install && enable "${SERVICE_NAME}"
        ;;
    17)
        check_install && disable "${SERVICE_NAME}"
        ;;
    18)
        bbr_menu
        ;;
    19)
        ssl_cert_issue_main
        ;;
    20)
        ssl_cert_issue_CF
        ;;
    *)
        LOGE "Please enter the correct number [0-20]"
        ;;
    esac
}

resolve_service_name

if [[ $# > 0 ]]; then
    case $1 in
    "start")
        check_install 0 && start "${SERVICE_NAME}" 0
        ;;
    "stop")
        check_install 0 && stop "${SERVICE_NAME}" 0
        ;;
    "restart")
        check_install 0 && restart "${SERVICE_NAME}" 0
        ;;
    "status")
        check_install 0 && status 0
        ;;
    "enable")
        check_install 0 && enable "${SERVICE_NAME}" 0
        ;;
    "disable")
        check_install 0 && disable "${SERVICE_NAME}" 0
        ;;
    "log")
        check_install 0 && show_log "${SERVICE_NAME}" 0
        ;;
    "update")
        shift
        check_install 0 && update "$@"
        ;;
    "install")
        check_uninstall 0 && install 0
        ;;
    "uninstall")
        check_install 0 && uninstall 0
        ;;
    *) show_usage ;;
    esac
else
    show_menu
fi
