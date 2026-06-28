#!/usr/bin/env bash
# ============================================================
# auto_snell.sh — Snell v5 全自动安装 & SubCon 自动注册脚本
# 用途：在 VPS 上一键部署 Snell v5 代理服务端，并向 SubCon 注册节点
# ============================================================
set -euo pipefail

# -------------------- 颜色定义 --------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
NC='\033[0m'  # 恢复默认

# -------------------- 工具函数 --------------------
info()    { echo -e "${GREEN}[✓]${NC} $*" >&2; }
warn()    { echo -e "${YELLOW}[!]${NC} $*" >&2; }
error()   { echo -e "${RED}[✗]${NC} $*" >&2; }
title()   { echo -e "\n${CYAN}${BOLD}>>> $* <<<${NC}\n" >&2; }

# -------------------- 常量 --------------------
SNELL_CONF_DIR="/etc/snell"
SNELL_CONF_FILE="${SNELL_CONF_DIR}/snell-server.conf"
SNELL_BIN="/usr/local/bin/snell-server"
SNELL_SERVICE_FILE="/etc/systemd/system/snell.service"
SNELL_VERSION_PAGE="https://manual.nssurge.com/others/snell.html"
SNELL_FALLBACK_VERSION="v5.0.1"
DOMAIN_SUFFIX="115emby.top"
PORT_MIN=10000
PORT_MAX=60000
PORT_MAX_RETRIES=20

# ============================================================
# 卸载逻辑
# ============================================================
do_uninstall() {
    title "开始卸载 Snell"

    local subcon_url=""
    local secret=""

    # 解析卸载参数
    shift  # 移除 --uninstall
    while [[ $# -gt 0 ]]; do
        case "$1" in
            --subcon-url)
                subcon_url="$2"
                shift 2
                ;;
            --secret)
                secret="$2"
                shift 2
                ;;
            *)
                shift
                ;;
        esac
    done

    # 如果提供了 SubCon 地址和密钥，先发送注销请求
    if [[ -n "${subcon_url}" && -n "${secret}" ]]; then
        info "尝试从 SubCon 注销节点..."

        # 从现有配置中读取 host 信息
        local host=""
        local port=""
        if [[ -f "${SNELL_CONF_FILE}" ]]; then
            port=$(grep -oP 'listen\s*=\s*[\d.]*:\K\d+' "${SNELL_CONF_FILE}" 2>/dev/null || true)
        fi

        # 尝试从注册信息文件读取 host
        local reg_info_file="${SNELL_CONF_DIR}/.registration_info"
        if [[ -f "${reg_info_file}" ]]; then
            host=$(grep -oP '^host=\K.*' "${reg_info_file}" 2>/dev/null || true)
        fi

        if [[ -n "${host}" ]]; then
            local timestamp
            timestamp=$(date +%s)
            local body="{\"host\":\"${host}\"}"
            local signature
            signature=$(echo -n "${timestamp}|${body}" | openssl dgst -sha256 -hmac "${secret}" | awk '{print $NF}')

            local http_code
            http_code=$(curl -s -o /dev/null -w "%{http_code}" -X DELETE "${subcon_url}/api/node" \
                -H "Content-Type: application/json" \
                -H "X-Timestamp: ${timestamp}" \
                -H "X-Signature: ${signature}" \
                -d "${body}" 2>/dev/null || echo "000")

            if [[ "${http_code}" == "200" || "${http_code}" == "204" ]]; then
                info "已从 SubCon 注销节点 (${host})"
            else
                warn "SubCon 注销请求返回 HTTP ${http_code}，可能未成功"
            fi
        else
            warn "无法从配置中获取 host 信息，跳过 SubCon 注销"
        fi
    fi

    # 停止并禁用 snell 服务
    if systemctl is-active --quiet snell 2>/dev/null; then
        info "停止 snell 服务..."
        systemctl stop snell
    fi
    if systemctl is-enabled --quiet snell 2>/dev/null; then
        info "禁用 snell 服务..."
        systemctl disable snell
    fi

    # 删除文件
    if [[ -f "${SNELL_BIN}" ]]; then
        rm -f "${SNELL_BIN}"
        info "已删除 ${SNELL_BIN}"
    fi
    if [[ -d "${SNELL_CONF_DIR}" ]]; then
        rm -rf "${SNELL_CONF_DIR}"
        info "已删除 ${SNELL_CONF_DIR}"
    fi
    if [[ -f "${SNELL_SERVICE_FILE}" ]]; then
        rm -f "${SNELL_SERVICE_FILE}"
        info "已删除 ${SNELL_SERVICE_FILE}"
    fi

    # 重载 systemd
    systemctl daemon-reload
    info "已重载 systemd 配置"

    echo ""
    info "${GREEN}${BOLD}Snell 已完全卸载！${NC}"
    exit 0
}

# ============================================================
# 主入口 — 参数检测
# ============================================================
# 检查是否为卸载模式
if [[ "${1:-}" == "--uninstall" ]]; then
    # root 权限检查
    if [[ "$(id -u)" -ne 0 ]]; then
        error "请使用 root 权限运行此脚本（sudo bash $0 --uninstall ...）"
        exit 1
    fi
    do_uninstall "$@"
fi

# ============================================================
# 安装模式 — root 权限检查
# ============================================================
if [[ "$(id -u)" -ne 0 ]]; then
    error "请使用 root 权限运行此脚本（sudo bash $0）"
    exit 1
fi

title "Snell v5 全自动安装脚本"

# -------------------- 1. 检测 CPU 架构 --------------------
detect_arch() {
    local arch
    arch=$(uname -m)
    case "${arch}" in
        x86_64)   echo "amd64" ;;
        aarch64)  echo "aarch64" ;;
        armv7l)   echo "armv7l" ;;
        *)
            error "不支持的 CPU 架构: ${arch}"
            exit 1
            ;;
    esac
}

ARCH=$(detect_arch)
info "检测到 CPU 架构: ${BOLD}${ARCH}${NC}"

# -------------------- 2. 获取最新 Snell v5 版本号 --------------------
get_snell_version() {
    info "正在从官方页面获取最新 Snell v5 版本号..."
    local version
    version=$(curl -sL --connect-timeout 10 --max-time 15 "${SNELL_VERSION_PAGE}" \
        | grep -oP 'snell-server-v\K5\.[0-9]+\.[0-9]+[a-z0-9]*' \
        | head -1 || true)

    if [[ -n "${version}" ]]; then
        echo "v${version}"
    else
        warn "无法从官方页面获取版本号，使用回退版本 ${SNELL_FALLBACK_VERSION}"
        echo "${SNELL_FALLBACK_VERSION}"
    fi
}

SNELL_VERSION=$(get_snell_version)
info "将安装 Snell 版本: ${BOLD}${SNELL_VERSION}${NC}"

# -------------------- 3. 安装依赖 --------------------
if ! command -v unzip &>/dev/null; then
    warn "未检测到 unzip，正在安装..."
    if command -v apt-get &>/dev/null; then
        apt-get update -qq && apt-get install -y -qq unzip
    elif command -v yum &>/dev/null; then
        yum install -y -q unzip
    elif command -v dnf &>/dev/null; then
        dnf install -y -q unzip
    elif command -v pacman &>/dev/null; then
        pacman -Sy --noconfirm unzip
    else
        error "无法自动安装 unzip，请手动安装后重试"
        exit 1
    fi
    info "unzip 安装完成"
fi

# -------------------- 4. 智能端口分配 --------------------
generate_port() {
    local port
    local attempt=0
    while [[ ${attempt} -lt ${PORT_MAX_RETRIES} ]]; do
        port=$(( RANDOM % (PORT_MAX - PORT_MIN + 1) + PORT_MIN ))
        if ! ss -tlnp 2>/dev/null | grep -q ":${port} "; then
            echo "${port}"
            return 0
        fi
        warn "端口 ${port} 已被占用，重新分配..."
        attempt=$((attempt + 1))
    done
    error "经过 ${PORT_MAX_RETRIES} 次尝试仍未找到可用端口，请手动指定"
    exit 1
}

PORT=$(generate_port)
info "分配端口: ${BOLD}${PORT}${NC}"

# -------------------- 5. 生成 PSK 密码 --------------------
PSK=$(openssl rand -base64 16 | tr -d '=+/' | head -c 16)
info "已生成 PSK 密码"

# -------------------- 6. 获取服务器公网 IP --------------------
get_public_ip() {
    local ip
    ip=$(curl -s --connect-timeout 5 --max-time 10 https://api.ipify.org 2>/dev/null \
        || curl -s --connect-timeout 5 --max-time 10 https://ifconfig.me 2>/dev/null \
        || curl -s --connect-timeout 5 --max-time 10 https://icanhazip.com 2>/dev/null \
        || echo "")
    echo "${ip}"
}

PUBLIC_IP=$(get_public_ip)
if [[ -n "${PUBLIC_IP}" ]]; then
    info "检测到公网 IP: ${BOLD}${PUBLIC_IP}${NC}"
else
    warn "无法获取公网 IP"
fi

# -------------------- 7. 交互式输入 --------------------
echo ""
title "节点配置"

# 域名前缀
read -rp "$(echo -e "${CYAN}请输入域名前缀（如 vmiss），将自动拼接为 xxx.${DOMAIN_SUFFIX}${NC}\n${YELLOW}直接回车将使用公网 IP (${PUBLIC_IP}) 作为 host: ${NC}")" DOMAIN_PREFIX

if [[ -n "${DOMAIN_PREFIX}" ]]; then
    HOST="${DOMAIN_PREFIX}.${DOMAIN_SUFFIX}"
else
    if [[ -z "${PUBLIC_IP}" ]]; then
        error "未输入域名前缀，且无法获取公网 IP，请手动输入域名前缀"
        exit 1
    fi
    HOST="${PUBLIC_IP}"
    DOMAIN_PREFIX=""
fi
info "节点 Host: ${BOLD}${HOST}${NC}"

# 节点名称
DEFAULT_NAME="Snell-${DOMAIN_PREFIX:-${PUBLIC_IP}}"
read -rp "$(echo -e "${CYAN}请输入节点名称（默认: ${DEFAULT_NAME}）: ${NC}")" NODE_NAME
NODE_NAME="${NODE_NAME:-${DEFAULT_NAME}}"
info "节点名称: ${BOLD}${NODE_NAME}${NC}"

# SubCon 注册地址
read -rp "$(echo -e "${CYAN}请输入 SubCon 服务地址（直接回车默认: http://dmit.115emby.top:8090，输入 skip 跳过注册）: ${NC}")" SUBCON_URL
SUBCON_URL="${SUBCON_URL:-http://dmit.115emby.top:8090}"

SUBCON_SECRET=""
if [[ "${SUBCON_URL}" != "skip" ]]; then
    # 去除末尾斜杠
    SUBCON_URL="${SUBCON_URL%/}"
    read -rp "$(echo -e "${CYAN}请输入注册密钥 (SSC_NODE_SECRET): ${NC}")" SUBCON_SECRET
    if [[ -z "${SUBCON_SECRET}" ]]; then
        warn "未输入注册密钥，将跳过自动注册"
        SUBCON_URL=""
    fi
fi

# -------------------- 8. 下载并安装 Snell --------------------
echo ""
title "安装 Snell v5"

SNELL_URL="https://dl.nssurge.com/snell/snell-server-${SNELL_VERSION}-linux-${ARCH}.zip"
TEMP_DIR=$(mktemp -d)
trap 'rm -rf "${TEMP_DIR}"' EXIT

info "下载 Snell: ${SNELL_URL}"
if ! curl -L --connect-timeout 15 --max-time 120 -o "${TEMP_DIR}/snell-server.zip" "${SNELL_URL}"; then
    error "下载 Snell 失败，请检查网络连接和版本号"
    exit 1
fi

info "解压安装..."
unzip -o "${TEMP_DIR}/snell-server.zip" -d "${TEMP_DIR}" >/dev/null
chmod +x "${TEMP_DIR}/snell-server"
mv "${TEMP_DIR}/snell-server" "${SNELL_BIN}"
info "Snell 已安装到 ${SNELL_BIN}"

# -------------------- 9. 写入配置文件 --------------------
title "配置 Snell"

mkdir -p "${SNELL_CONF_DIR}"

cat > "${SNELL_CONF_FILE}" <<EOF
[snell-server]
listen = 0.0.0.0:${PORT}
psk = ${PSK}
ipv6 = false
EOF

info "配置文件已写入 ${SNELL_CONF_FILE}"

# 保存注册信息（用于卸载时注销节点）
cat > "${SNELL_CONF_DIR}/.registration_info" <<EOF
host=${HOST}
name=${NODE_NAME}
port=${PORT}
EOF
chmod 600 "${SNELL_CONF_DIR}/.registration_info"

# -------------------- 10. 创建 systemd 服务 --------------------
title "配置 systemd 服务"

cat > "${SNELL_SERVICE_FILE}" <<EOF
[Unit]
Description=Snell Proxy Service
After=network.target

[Service]
Type=simple
User=nobody
Group=nogroup
LimitNOFILE=32768
ExecStart=/usr/local/bin/snell-server -c /etc/snell/snell-server.conf
AmbientCapabilities=CAP_NET_BIND_SERVICE
StandardOutput=syslog
StandardError=syslog
SyslogIdentifier=snell-server
Restart=on-failure
RestartSec=10s

[Install]
WantedBy=multi-user.target
EOF

info "服务文件已写入 ${SNELL_SERVICE_FILE}"

# 启动服务
systemctl daemon-reload
systemctl enable snell
systemctl start snell
info "snell 服务已启动"

# 等待 2 秒后验证
sleep 2
if systemctl is-active --quiet snell; then
    info "${GREEN}${BOLD}snell 服务运行正常！${NC}"
else
    error "snell 服务启动失败，请检查日志: journalctl -u snell -n 20"
    exit 1
fi

# -------------------- 11. 自动注册到 SubCon --------------------
REGISTER_RESULT=""
if [[ -n "${SUBCON_URL}" && -n "${SUBCON_SECRET}" ]]; then
    echo ""
    title "注册到 SubCon"

    TIMESTAMP=$(date +%s)
    BODY="{\"host\":\"${HOST}\",\"port\":${PORT},\"psk\":\"${PSK}\",\"version\":5,\"name\":\"${NODE_NAME}\"}"
    SIGNATURE=$(echo -n "${TIMESTAMP}|${BODY}" | openssl dgst -sha256 -hmac "${SUBCON_SECRET}" | awk '{print $NF}')

    info "正在向 ${SUBCON_URL} 发送注册请求..."

    RESPONSE=$(curl -s -w "\n%{http_code}" -X POST "${SUBCON_URL}/api/node" \
        -H "Content-Type: application/json" \
        -H "X-Timestamp: ${TIMESTAMP}" \
        -H "X-Signature: ${SIGNATURE}" \
        -d "${BODY}" 2>/dev/null || echo -e "\n000")

    HTTP_BODY=$(echo "${RESPONSE}" | head -n -1)
    HTTP_CODE=$(echo "${RESPONSE}" | tail -n 1)

    if [[ "${HTTP_CODE}" == "200" || "${HTTP_CODE}" == "201" ]]; then
        info "${GREEN}${BOLD}节点注册成功！${NC}"
        REGISTER_RESULT="✓ 注册成功 (HTTP ${HTTP_CODE})"
        if [[ -n "${HTTP_BODY}" ]]; then
            info "服务端响应: ${HTTP_BODY}"
        fi
    else
        warn "注册请求返回 HTTP ${HTTP_CODE}"
        REGISTER_RESULT="✗ 注册失败 (HTTP ${HTTP_CODE}): ${HTTP_BODY}"
        if [[ -n "${HTTP_BODY}" ]]; then
            warn "服务端响应: ${HTTP_BODY}"
        fi
    fi
fi

# -------------------- 12. 打印安装摘要 --------------------
echo ""
echo -e "${CYAN}${BOLD}╔════════════════════════════════════════════════════╗${NC}"
echo -e "${CYAN}${BOLD}║          Snell v5 安装完成 — 信息摘要             ║${NC}"
echo -e "${CYAN}${BOLD}╠════════════════════════════════════════════════════╣${NC}"
echo -e "${CYAN}║${NC}  Snell 版本:  ${GREEN}${BOLD}${SNELL_VERSION}${NC}"
echo -e "${CYAN}║${NC}  监听端口:    ${GREEN}${BOLD}${PORT}${NC}"
echo -e "${CYAN}║${NC}  PSK 密码:    ${GREEN}${BOLD}${PSK}${NC}"
echo -e "${CYAN}║${NC}  节点名称:    ${GREEN}${BOLD}${NODE_NAME}${NC}"
echo -e "${CYAN}║${NC}  Host:        ${GREEN}${BOLD}${HOST}${NC}"
echo -e "${CYAN}${BOLD}╠════════════════════════════════════════════════════╣${NC}"
echo -e "${CYAN}║${NC}  ${YELLOW}${BOLD}Surge 配置行:${NC}"
echo -e "${CYAN}║${NC}  ${GREEN}${NODE_NAME} = snell, ${HOST}, ${PORT}, psk=${PSK}, version=5${NC}"
echo -e "${CYAN}${BOLD}╠════════════════════════════════════════════════════╣${NC}"
echo -e "${CYAN}║${NC}  ${YELLOW}${BOLD}标准 URI:${NC}"
echo -e "${CYAN}║${NC}  ${GREEN}snell://${PSK}@${HOST}:${PORT}?version=5#${NODE_NAME}${NC}"
if [[ -n "${REGISTER_RESULT}" ]]; then
    echo -e "${CYAN}${BOLD}╠════════════════════════════════════════════════════╣${NC}"
    echo -e "${CYAN}║${NC}  ${YELLOW}${BOLD}SubCon 注册:${NC} ${REGISTER_RESULT}"
fi
echo -e "${CYAN}${BOLD}╚════════════════════════════════════════════════════╝${NC}"
echo ""
info "卸载命令: ${YELLOW}sudo bash $0 --uninstall --subcon-url <URL> --secret <密钥>${NC}"
info "查看日志: ${YELLOW}journalctl -u snell -f${NC}"
echo ""
