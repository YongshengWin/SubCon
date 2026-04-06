#!/usr/bin/env bash
set -euo pipefail

APP_NAME="surge-sub-converter"
INSTALL_DIR="/opt/${APP_NAME}"
BIN_PATH="/usr/local/bin/${APP_NAME}"
CTL_PATH="/usr/local/bin/sub"
SERVICE_FILE="/etc/systemd/system/${APP_NAME}.service"

if [[ "${EUID}" -ne 0 ]]; then
  echo "请使用 root 运行卸载脚本"
  exit 1
fi

if systemctl list-unit-files | grep -q "^${APP_NAME}.service"; then
  systemctl disable --now "${APP_NAME}" || true
fi

rm -f "${SERVICE_FILE}"
rm -f "${BIN_PATH}"
rm -f "${CTL_PATH}"
rm -rf "${INSTALL_DIR}"

systemctl daemon-reload

echo "已卸载 ${APP_NAME}"
