#!/usr/bin/env bash
set -euo pipefail

APP_NAME="surge-sub-converter"
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
BUILD_ROOT="${SCRIPT_DIR}/.release-build"
UPLOAD_DIR="${BUILD_ROOT}/upload"
VERIFY_DIR="${BUILD_ROOT}/verify"

detect_repo() {
  local origin
  origin="$(git -C "${SCRIPT_DIR}" remote get-url origin 2>/dev/null || true)"
  case "${origin}" in
    git@github.com:*.git)
      printf '%s\n' "${origin#git@github.com:}" | sed 's/\.git$//'
      ;;
    https://github.com/*)
      printf '%s\n' "${origin#https://github.com/}" | sed 's/\.git$//'
      ;;
    *)
      echo "无法从 origin 推断 GitHub 仓库，请设置 GH_REPO=owner/repo" >&2
      exit 1
      ;;
  esac
}

reset_dir() {
  local dir="$1"
  rm -rf "${dir}"
  mkdir -p "${dir}"
}

verify_asset() {
  local archive="$1"
  local version="$2"
  local extract_dir="$3"

  rm -rf "${extract_dir}"
  mkdir -p "${extract_dir}"
  tar -xzf "${archive}" -C "${extract_dir}"
  if ! grep -aFq "${version}" "${extract_dir}/${APP_NAME}"; then
    echo "下载的二进制版本校验失败，期望 ${version}" >&2
    exit 1
  fi
}

VERSION="${1:-${VERSION:-}}"
if [[ -z "${VERSION}" ]]; then
  echo "用法: VERSION=vX.Y.Z ./publish-release.sh" >&2
  echo "或: ./publish-release.sh vX.Y.Z" >&2
  exit 1
fi

REPO="${GH_REPO:-$(detect_repo)}"

if ! command -v gh >/dev/null 2>&1; then
  echo "需要 gh 命令来上传 GitHub Release" >&2
  exit 1
fi

if ! git -C "${SCRIPT_DIR}" show-ref --verify --quiet "refs/tags/${VERSION}"; then
  echo "本地缺少 tag ${VERSION}，请先创建并推送 tag" >&2
  exit 1
fi

reset_dir "${UPLOAD_DIR}"
reset_dir "${VERIFY_DIR}"

echo "构建发布包 ${VERSION}"
(
  cd "${SCRIPT_DIR}"
  VERSION="${VERSION}" OUT_DIR=".release-build/upload" ./package-release.sh
)

assets=(
  "${UPLOAD_DIR}/${APP_NAME}-linux-amd64.tar.gz"
  "${UPLOAD_DIR}/${APP_NAME}-linux-arm64.tar.gz"
)

for asset in "${assets[@]}"; do
  if [[ ! -f "${asset}" ]]; then
    echo "缺少构建产物: ${asset}" >&2
    exit 1
  fi
done

if gh release view "${VERSION}" --repo "${REPO}" >/dev/null 2>&1; then
  echo "更新已存在的 GitHub Release ${VERSION}"
  gh release upload "${VERSION}" "${assets[@]}" --clobber --repo "${REPO}"
else
  echo "创建 GitHub Release ${VERSION}"
  gh release create "${VERSION}" "${assets[@]}" --verify-tag --generate-notes --title "${VERSION}" --repo "${REPO}"
fi

echo "回传校验 GitHub Release 资产"
gh release download "${VERSION}" --repo "${REPO}" --dir "${VERIFY_DIR}" --pattern "${APP_NAME}-linux-*.tar.gz" --clobber

for suffix in linux-amd64 linux-arm64; do
  local_asset="${UPLOAD_DIR}/${APP_NAME}-${suffix}.tar.gz"
  remote_asset="${VERIFY_DIR}/${APP_NAME}-${suffix}.tar.gz"

  if [[ ! -f "${remote_asset}" ]]; then
    echo "下载校验失败，缺少远端资产: ${remote_asset}" >&2
    exit 1
  fi

  local_sum="$(shasum -a 256 "${local_asset}" | awk '{print $1}')"
  remote_sum="$(shasum -a 256 "${remote_asset}" | awk '{print $1}')"
  if [[ "${local_sum}" != "${remote_sum}" ]]; then
    echo "远端资产摘要不匹配: ${remote_asset}" >&2
    echo "本地: ${local_sum}" >&2
    echo "远端: ${remote_sum}" >&2
    exit 1
  fi

  verify_asset "${remote_asset}" "${VERSION}" "${VERIFY_DIR}/${suffix}"
done

echo "发布完成: ${VERSION}"
echo "仓库: ${REPO}"
