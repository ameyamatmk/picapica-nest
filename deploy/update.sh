#!/usr/bin/env bash
#
# PicaPica Nest アップデートスクリプト
#
# 使い方:
#   sudo bash update.sh              # 最新リリースに更新
#   sudo bash update.sh v0.2.0       # 特定バージョンに更新
#

set -euo pipefail

REPO="ameyamatmk/picapica-nest"
GITHUB_API="https://api.github.com/repos/${REPO}"
USER="picapica"
BIN_DIR="/home/${USER}/bin"
SERVICE_NAME="picapica-nest"
ARCH="$(uname -m)"

case "${ARCH}" in
    x86_64)  ARCH_SUFFIX="linux-amd64" ;;
    aarch64) ARCH_SUFFIX="linux-arm64" ;;
    *)
        echo "Error: unsupported architecture: ${ARCH}"
        exit 1
        ;;
esac

BINARY_NAME="picapica-nest-${ARCH_SUFFIX}"

# --- バージョン解決 ---
VERSION="${1:-}"
if [ -z "${VERSION}" ]; then
    VERSION=$(curl -fsSL "${GITHUB_API}/releases/latest" | grep -Po '"tag_name":\s*"\K[^"]+')
    if [ -z "${VERSION}" ]; then
        echo "Error: 最新リリースが見つかりません"
        exit 1
    fi
fi

# 現在のバージョン
CURRENT="unknown"
if [ -x "${BIN_DIR}/picapica-nest" ]; then
    CURRENT=$(sudo -u "${USER}" "${BIN_DIR}/picapica-nest" version 2>/dev/null | head -1 || echo "unknown")
fi
echo ">> 現在: ${CURRENT}"
echo ">> 更新先: ${VERSION}"

# --- ダウンロード URL を取得 ---
DOWNLOAD_URL=$(curl -fsSL "${GITHUB_API}/releases/tags/${VERSION}" \
    | grep -Po '"browser_download_url":\s*"\K[^"]*'"${BINARY_NAME}"'[^"]*')
if [ -z "${DOWNLOAD_URL}" ]; then
    echo "Error: バイナリ ${BINARY_NAME} がリリース ${VERSION} に見つかりません"
    exit 1
fi

# --- ダウンロード ---
echo ">> ダウンロード中: ${BINARY_NAME} (${VERSION})"
curl -fsSL -o "${BIN_DIR}/picapica-nest.tmp" "${DOWNLOAD_URL}"
chmod 755 "${BIN_DIR}/picapica-nest.tmp"

# --- サービス停止 → バイナリ差替 → 起動 ---
echo ">> サービスを再起動中..."
systemctl stop "${SERVICE_NAME}" || true
mv "${BIN_DIR}/picapica-nest.tmp" "${BIN_DIR}/picapica-nest"
chown "${USER}:${USER}" "${BIN_DIR}/picapica-nest"
systemctl start "${SERVICE_NAME}"

# --- 確認 ---
sleep 2
systemctl status "${SERVICE_NAME}" --no-pager
echo ""
echo ">> アップデート完了!"
