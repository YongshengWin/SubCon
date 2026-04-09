#!/usr/bin/env bash
set -euo pipefail

APP_NAME="surge-sub-converter"
INSTALL_DIR="/opt/${APP_NAME}"
BIN_PATH="/usr/local/bin/${APP_NAME}"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"
APP_PORT="${APP_PORT:-8090}"

detect_version() {
  if [[ -n "${VERSION:-}" ]]; then
    printf '%s\n' "${VERSION}"
    return 0
  fi
  if git -C "${SCRIPT_DIR}" describe --tags --exact-match >/dev/null 2>&1; then
    git -C "${SCRIPT_DIR}" describe --tags --exact-match
    return 0
  fi
  git -C "${SCRIPT_DIR}" describe --tags --always --dirty 2>/dev/null || echo "dev"
}

if [[ "${EUID}" -ne 0 ]]; then
  echo "请使用 root 运行安装脚本"
  exit 1
fi

if ! command -v go >/dev/null 2>&1; then
  echo "未检测到 Go，请先安装 Go 1.22+"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
VERSION="$(detect_version)"
mkdir -p "${INSTALL_DIR}"

cp "${SCRIPT_DIR}/go.mod" "${INSTALL_DIR}/go.mod"
cp "${SCRIPT_DIR}/main.go" "${INSTALL_DIR}/main.go"
cp "${SCRIPT_DIR}/templates.go" "${INSTALL_DIR}/templates.go"
cp "${SCRIPT_DIR}/clash_template.go" "${INSTALL_DIR}/clash_template.go"

cd "${INSTALL_DIR}"
go build -trimpath -ldflags="-s -w -X main.version=${VERSION} -X main.userAgent=surge-sub-converter/${VERSION}" -o "${BIN_PATH}" .

cat > "${SERVICE_FILE}" <<EOF
[Unit]
Description=Surge Subscription Converter
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
WorkingDirectory=${INSTALL_DIR}
ExecStart=${BIN_PATH}
Environment=SSC_LISTEN=0.0.0.0
Environment=SSC_PORT=${APP_PORT}
Restart=always
RestartSec=3

[Install]
WantedBy=multi-user.target
EOF

systemctl daemon-reload
systemctl enable --now "${APP_NAME}"

echo
echo "安装完成"
echo "前端页面: http://你的服务器IP:${APP_PORT}/"
echo "健康检查: http://你的服务器IP:${APP_PORT}/healthz"
echo "使用方式: 打开前端页面生成短链后再使用，公开入口不再展示明文转换链接"
