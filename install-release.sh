#!/usr/bin/env bash
set -euo pipefail

APP_NAME="surge-sub-converter"
INSTALL_DIR="/opt/${APP_NAME}"
CONFIG_DIR="${INSTALL_DIR}/data"
ENV_FILE="${INSTALL_DIR}/sub.env"
LINKS_FILE="${CONFIG_DIR}/subscriptions.txt"
BIN_PATH="/usr/local/bin/${APP_NAME}"
CTL_PATH="/usr/local/bin/sub"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"
TMP_DIR="$(mktemp -d)"
APP_PORT="${APP_PORT:-8090}"
APP_LISTEN="${APP_LISTEN:-0.0.0.0}"
APP_TEST_URL="${APP_TEST_URL:-http://www.gstatic.com/generate_204}"
PUBLIC_SCHEME="${PUBLIC_SCHEME:-http}"
PUBLIC_HOST="${PUBLIC_HOST:-}"
PUBLIC_IP="${PUBLIC_IP:-}"
REPO_OWNER="${REPO_OWNER:-YongshengWin}"
REPO_NAME="${REPO_NAME:-SubCon}"
REPO_BRANCH="${REPO_BRANCH:-main}"
RAW_BASE="${RAW_BASE:-https://raw.githubusercontent.com/${REPO_OWNER}/${REPO_NAME}/${REPO_BRANCH}}"
BASE_URL="${BASE_URL:-https://github.com/${REPO_OWNER}/${REPO_NAME}/releases/download}"
API_URL="${API_URL:-https://api.github.com/repos/${REPO_OWNER}/${REPO_NAME}/releases/latest}"

cleanup() {
  rm -rf "${TMP_DIR}"
}
trap cleanup EXIT

require_root() {
  if [[ "${EUID}" -ne 0 ]]; then
    echo "请使用 root 运行安装脚本"
    exit 1
  fi
}

detect_arch() {
  local arch
  arch="$(uname -m)"
  case "${arch}" in
    x86_64|amd64) echo "linux-amd64" ;;
    aarch64|arm64) echo "linux-arm64" ;;
    *)
      echo "不支持的架构: ${arch}"
      exit 1
      ;;
  esac
}

fetch() {
  local url="$1"
  local out="$2"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${url}" -o "${out}"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO "${out}" "${url}"
    return
  fi
  echo "需要 curl 或 wget"
  exit 1
}

fetch_text() {
  local url="$1"
  if command -v curl >/dev/null 2>&1; then
    curl -fsSL "${url}"
    return
  fi
  if command -v wget >/dev/null 2>&1; then
    wget -qO- "${url}"
    return
  fi
  echo "需要 curl 或 wget"
  exit 1
}

detect_public_ip() {
  fetch_text "https://api.ipify.org" 2>/dev/null || true
}

detect_version() {
  if [[ -n "${VERSION:-}" ]]; then
    echo "${VERSION}"
    return
  fi

  local response
  response="$(fetch_text "${API_URL}")"
  VERSION="$(printf '%s' "${response}" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  if [[ -z "${VERSION}" ]]; then
    echo "无法自动获取最新版本，请先设置 REPO_OWNER/REPO_NAME 或手动传入 VERSION"
    exit 1
  fi
  echo "${VERSION}"
}

install_binary() {
  local target="$1"
  local version="$2"
  local pkg_name="${APP_NAME}-${target}.tar.gz"
  local download_url="${BASE_URL}/${version}/${pkg_name}"

  echo "下载 ${download_url}"
  fetch "${download_url}" "${TMP_DIR}/${pkg_name}"
  tar -xzf "${TMP_DIR}/${pkg_name}" -C "${TMP_DIR}"

  mkdir -p "${INSTALL_DIR}" "${CONFIG_DIR}"
  install -m 0755 "${TMP_DIR}/${APP_NAME}" "${BIN_PATH}"
}

write_env_file() {
  if [[ -f "${ENV_FILE}" ]]; then
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
  fi

  cat > "${ENV_FILE}" <<EOF
SSC_VERSION=${SSC_VERSION:-${VERSION:-unknown}}
SSC_LISTEN=${SSC_LISTEN:-${APP_LISTEN}}
SSC_PORT=${SSC_PORT:-${APP_PORT}}
SSC_TEST_URL=${SSC_TEST_URL:-${APP_TEST_URL}}
SSC_CERT_FILE=${SSC_CERT_FILE:-}
SSC_KEY_FILE=${SSC_KEY_FILE:-}
SUB_PUBLIC_SCHEME=${SUB_PUBLIC_SCHEME:-${PUBLIC_SCHEME}}
SUB_PUBLIC_HOST=${SUB_PUBLIC_HOST:-${PUBLIC_HOST}}
SUB_PUBLIC_IP=${SUB_PUBLIC_IP:-${PUBLIC_IP:-}}
SUB_REPO_OWNER=${SUB_REPO_OWNER:-${REPO_OWNER}}
SUB_REPO_NAME=${SUB_REPO_NAME:-${REPO_NAME}}
SUB_REPO_BRANCH=${SUB_REPO_BRANCH:-${REPO_BRANCH}}
SUB_RAW_BASE=${SUB_RAW_BASE:-${RAW_BASE}}
SUB_RELEASE_BASE=${SUB_RELEASE_BASE:-${BASE_URL}}
SUB_RELEASE_API=${SUB_RELEASE_API:-${API_URL}}
EOF

  if [[ ! -f "${LINKS_FILE}" ]]; then
    touch "${LINKS_FILE}"
  fi
}

install_ctl() {
  cat > "${CTL_PATH}" <<'EOF'
#!/usr/bin/env bash
set -euo pipefail

APP_NAME="surge-sub-converter"
INSTALL_DIR="/opt/${APP_NAME}"
CONFIG_DIR="${INSTALL_DIR}/data"
ENV_FILE="${INSTALL_DIR}/sub.env"
LINKS_FILE="${CONFIG_DIR}/subscriptions.txt"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"
REMOTE_INSTALL_URL=""
REMOTE_UNINSTALL_URL=""

mkdir -p "${CONFIG_DIR}"
touch "${LINKS_FILE}"

# ── 颜色和样式 ────────────────────────────
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[0;33m'
CYAN='\033[0;36m'
BOLD='\033[1m'
DIM='\033[2m'
NC='\033[0m'

info()    { printf "${CYAN}▶ %s${NC}\n" "$1"; }
ok()      { printf "${GREEN}✔ %s${NC}\n" "$1"; }
warn()    { printf "${YELLOW}⚠ %s${NC}\n" "$1"; }
fail()    { printf "${RED}✘ %s${NC}\n" "$1"; }
divider() { printf "${DIM}────────────────────────────────────────${NC}\n"; }

load_env() {
  if [[ -f "${ENV_FILE}" ]]; then
    # shellcheck disable=SC1090
    source "${ENV_FILE}"
  fi
  SSC_LISTEN="${SSC_LISTEN:-0.0.0.0}"
  SSC_PORT="${SSC_PORT:-8090}"
  SSC_CERT_FILE="${SSC_CERT_FILE:-}"
  SSC_KEY_FILE="${SSC_KEY_FILE:-}"
  SUB_PUBLIC_SCHEME="${SUB_PUBLIC_SCHEME:-http}"
  SUB_PUBLIC_HOST="${SUB_PUBLIC_HOST:-}"
  SUB_PUBLIC_IP="${SUB_PUBLIC_IP:-}"
  SSC_VERSION="${SSC_VERSION:-unknown}"
  SUB_REPO_OWNER="${SUB_REPO_OWNER:-YongshengWin}"
  SUB_REPO_NAME="${SUB_REPO_NAME:-SubCon}"
  SUB_REPO_BRANCH="${SUB_REPO_BRANCH:-main}"
  SUB_RAW_BASE="${SUB_RAW_BASE:-https://raw.githubusercontent.com/${SUB_REPO_OWNER}/${SUB_REPO_NAME}/${SUB_REPO_BRANCH}}"
  SUB_RELEASE_BASE="${SUB_RELEASE_BASE:-https://github.com/${SUB_REPO_OWNER}/${SUB_REPO_NAME}/releases/download}"
  SUB_RELEASE_API="${SUB_RELEASE_API:-https://api.github.com/repos/${SUB_REPO_OWNER}/${SUB_REPO_NAME}/releases/latest}"
  REMOTE_INSTALL_URL="${SUB_RAW_BASE}/install-release.sh"
  REMOTE_UNINSTALL_URL="${SUB_RAW_BASE}/uninstall.sh"
}

save_env() {
  cat > "${ENV_FILE}" <<EOF2
SSC_VERSION=${SSC_VERSION:-unknown}
SSC_LISTEN=${SSC_LISTEN}
SSC_PORT=${SSC_PORT}
SSC_TEST_URL=${SSC_TEST_URL:-http://www.gstatic.com/generate_204}
SSC_CERT_FILE=${SSC_CERT_FILE:-}
SSC_KEY_FILE=${SSC_KEY_FILE:-}
SUB_PUBLIC_SCHEME=${SUB_PUBLIC_SCHEME}
SUB_PUBLIC_HOST=${SUB_PUBLIC_HOST}
SUB_PUBLIC_IP=${SUB_PUBLIC_IP:-}
SUB_REPO_OWNER=${SUB_REPO_OWNER}
SUB_REPO_NAME=${SUB_REPO_NAME}
SUB_REPO_BRANCH=${SUB_REPO_BRANCH}
SUB_RAW_BASE=${SUB_RAW_BASE}
SUB_RELEASE_BASE=${SUB_RELEASE_BASE}
SUB_RELEASE_API=${SUB_RELEASE_API}
EOF2
}

public_base() {
  if [[ -n "${SUB_PUBLIC_HOST}" ]]; then
    if [[ "${SUB_PUBLIC_HOST}" == *:* ]]; then
      printf '%s://%s' "${SUB_PUBLIC_SCHEME}" "${SUB_PUBLIC_HOST}"
    else
      if [[ "${SSC_PORT}" == "80" && "${SUB_PUBLIC_SCHEME}" == "http" ]] || [[ "${SSC_PORT}" == "443" && "${SUB_PUBLIC_SCHEME}" == "https" ]]; then
        printf '%s://%s' "${SUB_PUBLIC_SCHEME}" "${SUB_PUBLIC_HOST}"
      else
        printf '%s://%s:%s' "${SUB_PUBLIC_SCHEME}" "${SUB_PUBLIC_HOST}" "${SSC_PORT}"
      fi
    fi
  elif [[ -n "${SUB_PUBLIC_IP}" ]]; then
    printf 'http://%s:%s' "${SUB_PUBLIC_IP}" "${SSC_PORT}"
  else
    printf 'http://127.0.0.1:%s' "${SSC_PORT}"
  fi
}

rawurlencode() {
  local string="${1}"
  local strlen=${#string}
  local encoded=""
  local pos c o
  for (( pos=0; pos<strlen; pos++ )); do
    c=${string:$pos:1}
    case "${c}" in
      [-_.~a-zA-Z0-9]) o="${c}" ;;
      *) printf -v o '%%%02X' "'${c}" ;;
    esac
    encoded+="${o}"
  done
  echo "${encoded}"
}

show_header() {
  load_env
  local base tls_mode
  base="$(public_base)"
  tls_mode="$(detect_tls_mode)"

  echo
  printf "${BOLD}${CYAN}  ┌────────────────────────────────────────┐${NC}\n"
  printf "${BOLD}${CYAN}  │                                        │${NC}\n"
  printf "${BOLD}${CYAN}  │${NC}   ${BOLD}■ 订阅转换管理面板${NC}  ${DIM}%s${NC}\n" "${SSC_VERSION}"
  printf "${BOLD}${CYAN}  │                                        │${NC}\n"
  printf "${BOLD}${CYAN}  └────────────────────────────────────────┘${NC}\n"
  echo
  printf "  ${DIM}服务地址${NC}  ${BOLD}%s${NC}\n" "${base}"
  if [[ -n "${SUB_PUBLIC_HOST}" ]]; then
    printf "  ${DIM}外部域名${NC}  ${BOLD}%s://%s${NC}\n" "${SUB_PUBLIC_SCHEME}" "${SUB_PUBLIC_HOST}"
  elif [[ -n "${SUB_PUBLIC_IP}" ]]; then
    printf "  ${DIM}绑定  IP${NC}  ${BOLD}%s${NC}\n" "${SUB_PUBLIC_IP}"
  fi
  case "${tls_mode}" in
    原生\ HTTPS*|Caddy\ 反代*)
      printf "  ${DIM}TLS 模式${NC}  ${GREEN}✔ %s${NC}\n" "${tls_mode}" ;;
    *)
      printf "  ${DIM}TLS 模式${NC}  ${YELLOW}✘ %s${NC}\n" "${tls_mode}" ;;
  esac
  echo
}

count_links() {
  awk 'NF{n++} END{print n+0}' "${LINKS_FILE}"
}

detect_tls_mode() {
  if [[ -n "${SSC_CERT_FILE}" && -n "${SSC_KEY_FILE}" ]]; then
    echo "原生 HTTPS"
    return
  fi
  if systemctl is-active --quiet caddy 2>/dev/null; then
    echo "Caddy 反代"
    return
  fi
  echo "未启用"
}

list_links() {
  load_env
  local base
  base="$(public_base)"
  if [[ ! -s "${LINKS_FILE}" ]]; then
    warn "暂无已保存的订阅转换链接"
    return
  fi
  echo
  local i=1
  while IFS='|' read -r name target source; do
    [[ -z "${name}" ]] && continue
    local encoded
    encoded="$(rawurlencode "${source}")"
    printf "  ${BOLD}${CYAN}%d.${NC} %s\n" "$i" "${name}"
    printf "     ${DIM}目标:${NC} %s\n" "${target}"
    printf "     ${DIM}来源:${NC} %s\n" "${source}"
    printf "     ${DIM}链接:${NC} ${GREEN}%s${NC}\n" "${base}/convert?target=${target}&url=${encoded}"
    echo
    i=$((i+1))
  done < "${LINKS_FILE}"
}

add_link() {
  load_env
  local name target source
  echo
  read -r -p "$(printf "${CYAN}名称: ${NC}")" name
  read -r -p "$(printf "${CYAN}目标客户端${NC}(surge/clash/stash/quantumultx): ")" target
  read -r -p "$(printf "${CYAN}原始订阅URL: ${NC}")" source
  case "${target}" in
    surge|clash|stash|quantumultx) ;;
    *) fail "不支持的目标客户端"; return 1 ;;
  esac
  [[ -z "${name}" || -z "${source}" ]] && { fail "名称和订阅URL不能为空"; return 1; }
  echo "${name}|${target}|${source}" >> "${LINKS_FILE}"
  ok "已添加"
}

delete_link() {
  load_env
  if [[ ! -s "${LINKS_FILE}" ]]; then
    warn "暂无可删除项目"
    return
  fi
  list_links
  local index
  read -r -p "$(printf "${CYAN}输入要删除的序号: ${NC}")" index
  [[ ! "${index}" =~ ^[0-9]+$ ]] && { fail "序号无效"; return 1; }
  awk -F'|' -v idx="${index}" 'BEGIN{n=0} NF{n++; if(n!=idx) print $0}' "${LINKS_FILE}" > "${LINKS_FILE}.tmp"
  mv "${LINKS_FILE}.tmp" "${LINKS_FILE}"
  ok "已删除"
}

set_public_domain() {
  load_env
  echo
  read -r -p "$(printf "${CYAN}域名或主机${NC}(例如 sub.example.com，留空则清除): ")" SUB_PUBLIC_HOST
  if [[ -n "${SUB_PUBLIC_HOST}" ]]; then
    read -r -p "$(printf "${CYAN}协议${NC}(http/https，默认 ${GREEN}https${NC}): ")" SUB_PUBLIC_SCHEME
    SUB_PUBLIC_SCHEME="${SUB_PUBLIC_SCHEME:-https}"
    read -r -p "$(printf "${CYAN}服务端口${NC}(当前 ${BOLD}${SSC_PORT}${NC}，回车保持不变): ")" new_port
    if [[ -n "${new_port}" ]]; then
      SSC_PORT="${new_port}"
    fi
  else
    SUB_PUBLIC_SCHEME="http"
  fi
  save_env
  systemctl daemon-reload
  systemctl restart "${APP_NAME}"
  ok "已保存"
  if [[ -n "${SUB_PUBLIC_HOST}" ]]; then
    info "访问地址: ${SUB_PUBLIC_SCHEME}://${SUB_PUBLIC_HOST}/"
    warn "Cloudflare 代理场景请确认 SSL/TLS 模式已正确设置"
    warn "Cloudflare 默认不代理 8090，HTTPS 推荐使用 443 或 8443"
  fi
}

bind_public_ip() {
  load_env
  echo
  read -r -p "$(printf "${CYAN}绑定公网IP${NC}(留空则清除): ")" SUB_PUBLIC_IP
  save_env
  ok "已保存"
  if [[ -n "${SUB_PUBLIC_IP}" ]]; then
    info "当前访问地址: http://${SUB_PUBLIC_IP}:${SSC_PORT}/"
  fi
}

update_service() {
  load_env
  local latest
  latest="$(curl -fsSL "${SUB_RELEASE_API}" | sed -n 's/.*"tag_name":[[:space:]]*"\([^"]*\)".*/\1/p' | head -n1)"
  if [[ -z "${latest}" ]]; then
    fail "无法获取远端版本"
    return 1
  fi
  echo
  printf "  当前版本: ${BOLD}%s${NC}\n" "${SSC_VERSION}"
  printf "  远端版本: ${BOLD}%s${NC}\n" "${latest}"
  echo
  if [[ "${SSC_VERSION}" == "${latest}" ]]; then
    ok "已经是最新版本"
    return 0
  fi
  info "开始更新..."
  curl -fsSL "${REMOTE_INSTALL_URL}" | VERSION="${latest}" bash
}

install_caddy() {
  if command -v caddy >/dev/null 2>&1; then
    return 0
  fi
  if command -v apt-get >/dev/null 2>&1; then
    apt-get update
    apt-get install -y debian-keyring debian-archive-keyring apt-transport-https curl gnupg
    curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/gpg.key | gpg --dearmor -o /usr/share/keyrings/caddy-stable-archive-keyring.gpg
    curl -fsSL https://dl.cloudsmith.io/public/caddy/stable/debian.deb.txt | tee /etc/apt/sources.list.d/caddy-stable.list >/dev/null
    apt-get update
    apt-get install -y caddy
    return 0
  fi
  fail "当前系统未实现自动安装 Caddy，请手动安装后重试"
  return 1
}

deploy_https_proxy() {
  load_env
  local domain email local_port
  echo
  read -r -p "$(printf "${CYAN}域名${NC}(例如 sub.example.com): ")" domain
  [[ -z "${domain}" ]] && { fail "域名不能为空"; return 1; }
  read -r -p "$(printf "${CYAN}邮箱${NC}(可选，用于 HTTPS 证书通知): ")" email
  local_port="${SSC_PORT}"
  install_caddy || return 1

  SSC_LISTEN="127.0.0.1"
  SUB_PUBLIC_HOST="${domain}"
  SUB_PUBLIC_SCHEME="https"
  save_env

  mkdir -p /etc/caddy
  {
    echo "{"
    [[ -n "${email}" ]] && echo "  email ${email}"
    echo "}"
    echo
    echo "${domain} {"
    echo "  encode gzip"
    echo "  reverse_proxy 127.0.0.1:${local_port}"
    echo "}"
  } > /etc/caddy/Caddyfile

  systemctl daemon-reload
  systemctl restart "${APP_NAME}"
  systemctl enable --now caddy
  systemctl restart caddy
  ok "已部署 HTTPS 反代"
  info "访问地址: https://${domain}/"
  warn "如使用 Cloudflare 橙云，请确保 80/443 已放行，SSL/TLS 模式建议 Full 或 Full (strict)"
}

setup_shared_cert() {
  load_env
  local domain cert key port

  echo
  info "此模式将复用已有的 TLS 证书（如 acme.sh / 3x-ui 的证书）"
  echo
  printf "  ${DIM}常见证书路径:${NC}\n"
  printf "    ${DIM}acme.sh :${NC} ~/.acme.sh/域名_ecc/fullchain.cer\n"
  printf "    ${DIM}3x-ui   :${NC} /root/cert/域名/fullchain.pem\n"
  echo

  local found_certs=()
  for d in /root/cert/*/fullchain.pem /root/.acme.sh/*_ecc/fullchain.cer; do
    [[ -f "$d" ]] && found_certs+=("$d")
  done
  if [[ ${#found_certs[@]} -gt 0 ]]; then
    ok "检测到以下证书文件:"
    for f in "${found_certs[@]}"; do
      printf "    ${GREEN}%s${NC}\n" "${f}"
    done
    echo
  fi

  read -r -p "$(printf "${CYAN}域名${NC}(例如 sub.example.com): ")" domain
  read -r -p "$(printf "${CYAN}证书文件路径${NC}(fullchain.pem): ")" cert
  read -r -p "$(printf "${CYAN}私钥文件路径${NC}(privkey.pem): ")" key
  read -r -p "$(printf "${CYAN}监听端口${NC}(默认 ${GREEN}8443${NC}，Cloudflare 兼容): ")" port
  port="${port:-8443}"

  [[ -z "${domain}" || -z "${cert}" || -z "${key}" ]] && { fail "域名、证书、私钥都不能为空"; return 1; }
  [[ ! -f "${cert}" ]] && { fail "证书文件不存在: ${cert}"; return 1; }
  [[ ! -f "${key}" ]] && { fail "私钥文件不存在: ${key}"; return 1; }

  SSC_LISTEN="0.0.0.0"
  SSC_PORT="${port}"
  SSC_CERT_FILE="${cert}"
  SSC_KEY_FILE="${key}"
  SUB_PUBLIC_HOST="${domain}:${port}"
  SUB_PUBLIC_SCHEME="https"
  save_env

  systemctl daemon-reload
  systemctl restart "${APP_NAME}"

  echo
  ok "原生 HTTPS 已启用"
  info "访问地址: https://${domain}:${port}/"
  echo
  warn "Cloudflare 橙云支持的 HTTPS 端口: 443, 2053, 2083, 2087, 2096, 8443"
  warn "如使用 Cloudflare，请确保 SSL/TLS 模式为 Full 或 Full (strict)"
  echo

  local bare_domain="${domain%%:*}"
  if command -v acme.sh >/dev/null 2>&1 || [[ -f /root/.acme.sh/acme.sh ]]; then
    info "正在配置 acme.sh 证书续签后自动重载..."
    local acme_cmd="acme.sh"
    [[ ! -x "$(command -v acme.sh 2>/dev/null)" ]] && acme_cmd="/root/.acme.sh/acme.sh"
    if "${acme_cmd}" --install-cert -d "${bare_domain}" \
        --fullchain-file "${cert}" \
        --key-file "${key}" \
        --reloadcmd "systemctl restart ${APP_NAME}" 2>/dev/null; then
      ok "已配置证书续期自动重载"
    else
      warn "自动配置失败，请手动执行:"
      printf "  ${DIM}acme.sh --install-cert -d %s --reloadcmd \"systemctl restart %s\"${NC}\n" "${bare_domain}" "${APP_NAME}"
    fi
  else
    warn "未检测到 acme.sh，请手动配置证书续期:"
    printf "  ${DIM}acme.sh --install-cert -d %s --reloadcmd \"systemctl restart %s\"${NC}\n" "${bare_domain}" "${APP_NAME}"
  fi
}


generate_nginx_config() {
  load_env
  local domain path

  echo
  read -r -p "$(printf "${CYAN}域名${NC}(已在 nginx 中配置的): ")" domain
  read -r -p "$(printf "${CYAN}转发路径${NC}(默认 ${GREEN}/sub/${NC}): ")" path
  path="${path:-/sub/}"

  echo
  ok "请将以下内容添加到你的 nginx server 块中:"
  echo
  divider
  echo "    location ${path} {"
  echo "        proxy_pass http://127.0.0.1:${SSC_PORT}/;"
  echo '        proxy_set_header Host $host;'
  echo '        proxy_set_header X-Real-IP $remote_addr;'
  echo "    }"
  divider
  echo
  info "添加后执行: nginx -t && systemctl reload nginx"

  SUB_PUBLIC_HOST="${domain}"
  SUB_PUBLIC_SCHEME="https"
  save_env
  ok "域名已保存"
}

https_menu() {
  echo
  info "选择 HTTPS 配置方式:"
  echo
  printf "  ${BOLD}1.${NC} 复用已有证书 ${GREEN}(推荐，适合 3x-ui 共存)${NC}\n"
  printf "  ${BOLD}2.${NC} 仅设置域名 ${DIM}(配合 Cloudflare CDN)${NC}\n"
  printf "  ${BOLD}3.${NC} Nginx 反代 ${DIM}(生成配置片段)${NC}\n"
  printf "  ${BOLD}4.${NC} Caddy 反代 ${DIM}(高级)${NC}\n"
  printf "  ${BOLD}0.${NC} 返回\n"
  echo
  read -r -p "$(printf "${CYAN}请输入: ${NC}")" sub_choice
  case "${sub_choice}" in
    1) setup_shared_cert ;;
    2) set_public_domain ;;
    3) generate_nginx_config ;;
    4) deploy_https_proxy ;;
    0) return ;;
    *) warn "无效选项" ;;
  esac
}

show_status() {
  load_env
  local active enabled pid listen_url tls_mode caddy_state link_count
  active="$(systemctl is-active "${APP_NAME}" 2>/dev/null || true)"
  enabled="$(systemctl is-enabled "${APP_NAME}" 2>/dev/null || true)"
  pid="$(systemctl show -p MainPID --value "${APP_NAME}" 2>/dev/null || true)"
  listen_url="${SSC_LISTEN}:${SSC_PORT}"
  tls_mode="$(detect_tls_mode)"
  caddy_state="$(systemctl is-active caddy 2>/dev/null || true)"
  link_count="$(count_links)"

  echo
  divider
  printf "  ${BOLD}服务状态看板${NC}\n"
  divider
  echo

  if [[ "${active}" == "active" ]]; then
    printf "  运行状态   ${GREEN}● 运行中${NC}\n"
  else
    printf "  运行状态   ${RED}● 已停止${NC}\n"
  fi

  if [[ "${enabled}" == "enabled" ]]; then
    printf "  开机启动   ${GREEN}✔ 已启用${NC}\n"
  else
    printf "  开机启动   ${DIM}✘ 未启用${NC}\n"
  fi

  printf "  当前版本   ${BOLD}%s${NC}\n" "${SSC_VERSION}"
  printf "  进程 PID   %s\n" "${pid:-0}"
  printf "  监听地址   %s\n" "${listen_url}"
  printf "  外部地址   ${BOLD}%s${NC}\n" "$(public_base)"

  case "${tls_mode}" in
    原生\ HTTPS*|Caddy\ 反代*)
      printf "  TLS 模式   ${GREEN}✔ %s${NC}\n" "${tls_mode}" ;;
    *)
      printf "  TLS 模式   ${YELLOW}✘ %s${NC}\n" "${tls_mode}" ;;
  esac

  if [[ "${caddy_state}" == "active" ]]; then
    printf "  Caddy      ${GREEN}● 运行中${NC}\n"
  else
    printf "  Caddy      ${DIM}● %s${NC}\n" "${caddy_state:-inactive}"
  fi

  printf "  已保存订阅 %s\n" "${link_count}"

  if [[ -n "${SUB_PUBLIC_HOST}" ]]; then
    printf "  绑定域名   ${BOLD}%s://%s${NC}\n" "${SUB_PUBLIC_SCHEME}" "${SUB_PUBLIC_HOST}"
  fi
  if [[ -n "${SUB_PUBLIC_IP}" ]]; then
    printf "  绑定公网IP ${BOLD}%s${NC}\n" "${SUB_PUBLIC_IP}"
  fi
  if [[ -n "${SSC_CERT_FILE}" ]]; then
    printf "  证书文件   ${DIM}%s${NC}\n" "${SSC_CERT_FILE}"
  fi
  if [[ -n "${SSC_KEY_FILE}" ]]; then
    printf "  私钥文件   ${DIM}%s${NC}\n" "${SSC_KEY_FILE}"
  fi

  echo
  divider
  printf "  ${BOLD}最近日志${NC}\n"
  divider
  journalctl -u "${APP_NAME}" -n 8 --no-pager 2>/dev/null || true
  echo
}

restart_service() {
  systemctl restart "${APP_NAME}"
  ok "已重启"
}

uninstall_service() {
  echo
  read -r -p "$(printf "${RED}确认卸载? [y/N]: ${NC}")" ans
  [[ "${ans}" != "y" && "${ans}" != "Y" ]] && return 0
  curl -fsSL "${REMOTE_UNINSTALL_URL}" | bash
}

menu() {
  show_header

  printf "  ${DIM}── 基础管理 ─────────────────────────${NC}\n"
  printf "   ${BOLD}1.${NC}  更新服务\n"
  printf "   ${BOLD}2.${NC}  查看服务状态\n"
  printf "   ${BOLD}7.${NC}  重启服务\n"
  printf "   ${BOLD}8.${NC}  卸载服务\n"
  echo
  printf "  ${DIM}── HTTPS / 域名 ─────────────────────${NC}\n"
  printf "   ${BOLD}6.${NC}  HTTPS / 域名设置\n"
  echo
  printf "  ${DIM}── 订阅管理 ─────────────────────────${NC}\n"
  printf "   ${BOLD}3.${NC}  查看订阅链接\n"
  printf "   ${BOLD}4.${NC}  添加转换订阅链接\n"
  printf "   ${BOLD}5.${NC}  删除转换订阅链接\n"
  echo
  printf "  ${BOLD}11.${NC} 绑定公网 IP\n"
  printf "   ${BOLD}0.${NC}  退出\n"
  echo
  read -r -p "$(printf "${CYAN}请输入数字: ${NC}")" choice
  case "${choice}" in
    1) update_service ;;
    2) show_status ;;
    3) list_links ;;
    4) add_link ;;
    5) delete_link ;;
    6) https_menu ;;
    7) restart_service ;;
    8) uninstall_service ;;
    0) exit 0 ;;
    *) warn "无效选项" ;;
  esac
}

if [[ $# -gt 0 ]]; then
  case "$1" in
    1|update) update_service ;;
    2|status) show_status ;;
    3|list) list_links ;;
    4|add) add_link ;;
    5|delete|del|remove) delete_link ;;
    6|domain|https|port|ssl|cert|proxy|tls|native-https) https_menu ;;
    7|restart) restart_service ;;
    8|uninstall) uninstall_service ;;
    9) https_menu ;;
    10) https_menu ;;
    11|bind-ip|ip) bind_public_ip ;;
    *) menu ;;
  esac
else
  menu
fi
EOF
  chmod 755 "${CTL_PATH}"
}

install_service() {
  cat > "${SERVICE_FILE}" <<EOF
[Unit]
Description=Surge Subscription Converter
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BIN_PATH}
EnvironmentFile=-${ENV_FILE}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

  systemctl daemon-reload
  systemctl enable --now "${APP_NAME}"
}

main() {
  require_root
  local target
  local version
  target="$(detect_arch)"
  version="$(detect_version)"
  VERSION="${version}"
  SSC_VERSION="${version}"
  install_binary "${target}" "${version}"
  write_env_file
  install_ctl
  install_service

  echo
  echo "安装完成"
  echo "安装版本: ${version}"
  local public_ip
  public_ip="$(detect_public_ip)"
  echo "管理命令: sub"
  if [[ -n "${PUBLIC_HOST}" ]]; then
    echo "前端页面: ${PUBLIC_SCHEME}://${PUBLIC_HOST}/"
  elif [[ -n "${PUBLIC_IP}" ]]; then
    echo "前端页面: http://${PUBLIC_IP}:${APP_PORT}/"
  elif [[ -n "${public_ip}" ]]; then
    echo "前端页面: http://${public_ip}:${APP_PORT}/"
  else
    echo "前端页面: http://你的服务器IP:${APP_PORT}/"
  fi
  echo "健康检查: http://127.0.0.1:${APP_PORT}/healthz"
  echo "查看日志: journalctl -u ${APP_NAME} -f"
}

main "$@"
