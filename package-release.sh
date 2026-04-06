#!/usr/bin/env bash
set -euo pipefail

APP_NAME="surge-sub-converter"
VERSION="${VERSION:-v0.1.0}"
OUT_DIR="${OUT_DIR:-dist}"

if ! command -v go >/dev/null 2>&1; then
  echo "需要 Go 1.22+ 才能打包"
  exit 1
fi

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
mkdir -p "${SCRIPT_DIR}/${OUT_DIR}"

build_one() {
  local goos="$1"
  local goarch="$2"
  local suffix="$3"
  local work_dir
  work_dir="$(mktemp -d)"
  trap 'rm -rf "${work_dir}"' RETURN

  echo "构建 ${goos}/${goarch}"
  (cd "${SCRIPT_DIR}" && GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o "${work_dir}/${APP_NAME}" .)
  tar -czf "${SCRIPT_DIR}/${OUT_DIR}/${APP_NAME}-${suffix}.tar.gz" -C "${work_dir}" "${APP_NAME}"
}

build_one linux amd64 linux-amd64
build_one linux arm64 linux-arm64

echo "打包完成，输出目录: ${SCRIPT_DIR}/${OUT_DIR}"
echo "建议发布文件名:"
echo "- ${APP_NAME}-linux-amd64.tar.gz"
echo "- ${APP_NAME}-linux-arm64.tar.gz"
echo "发布版本: ${VERSION}"
