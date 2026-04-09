#!/usr/bin/env bash
set -euo pipefail

APP_NAME="surge-sub-converter"
OUT_DIR="${OUT_DIR:-dist}"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

detect_version() {
  if [[ -n "${VERSION:-}" ]]; then
    printf '%s\n' "${VERSION}"
    return 0
  fi

  if git -C "${SCRIPT_DIR}" describe --tags --exact-match >/dev/null 2>&1; then
    git -C "${SCRIPT_DIR}" describe --tags --exact-match
    return 0
  fi

  echo "未设置 VERSION，且当前提交不在 tag 上。请先打 tag，或显式传入 VERSION=vX.Y.Z" >&2
  exit 1
}

build_ldflags() {
  local version="$1"
  printf '%s' "-s -w -X main.version=${version} -X main.userAgent=surge-sub-converter/${version}"
}

verify_binary_version() {
  local binary="$1"
  local version="$2"
  if ! grep -aFq "${version}" "${binary}"; then
    echo "构建产物版本校验失败: ${binary} 未包含 ${version}" >&2
    exit 1
  fi
}

VERSION="$(detect_version)"

if ! command -v go >/dev/null 2>&1; then
  echo "需要 Go 1.22+ 才能打包"
  exit 1
fi

mkdir -p "${SCRIPT_DIR}/${OUT_DIR}"
rm -f "${SCRIPT_DIR}/${OUT_DIR}/${APP_NAME}-linux-amd64.tar.gz" "${SCRIPT_DIR}/${OUT_DIR}/${APP_NAME}-linux-arm64.tar.gz"

build_one() {
  local goos="$1"
  local goarch="$2"
  local suffix="$3"
  local work_dir
  work_dir="$(mktemp -d)"
  trap 'rm -rf "${work_dir}"' RETURN

  echo "构建 ${goos}/${goarch}"
  (cd "${SCRIPT_DIR}" && GOOS="${goos}" GOARCH="${goarch}" CGO_ENABLED=0 go build -trimpath -ldflags="$(build_ldflags "${VERSION}")" -o "${work_dir}/${APP_NAME}" .)
  verify_binary_version "${work_dir}/${APP_NAME}" "${VERSION}"
  tar -czf "${SCRIPT_DIR}/${OUT_DIR}/${APP_NAME}-${suffix}.tar.gz" -C "${work_dir}" "${APP_NAME}"
}

build_one linux amd64 linux-amd64
build_one linux arm64 linux-arm64

echo "打包完成，输出目录: ${SCRIPT_DIR}/${OUT_DIR}"
echo "建议发布文件名:"
echo "- ${APP_NAME}-linux-amd64.tar.gz"
echo "- ${APP_NAME}-linux-arm64.tar.gz"
echo "发布版本: ${VERSION}"
