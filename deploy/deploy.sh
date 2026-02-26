#!/usr/bin/env bash
#
# PicaPica Nest デプロイスクリプト
#
# 使い方:
#   sudo bash deploy.sh              # 最新リリースをデプロイ
#   sudo bash deploy.sh v0.1.0       # 特定バージョンをデプロイ
#
# 前提:
#   - Debian 13 / Ubuntu (systemd 環境)
#   - root 権限で実行
#   - インターネット接続あり（GitHub からバイナリを取得）

set -euo pipefail

REPO="ameyamatmk/picapica-nest"
GITHUB_API="https://api.github.com/repos/${REPO}"
USER="picapica"
HOME_DIR="/home/${USER}"
BIN_DIR="${HOME_DIR}/bin"
CONFIG_DIR="${HOME_DIR}/.picapica-nest"
SERVICE_NAME="picapica-nest"
ARCH="$(uname -m)"

# アーキテクチャ変換
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
    echo ">> 最新リリースを取得中..."
    VERSION=$(curl -fsSL "${GITHUB_API}/releases/latest" | grep -Po '"tag_name":\s*"\K[^"]+')
    if [ -z "${VERSION}" ]; then
        echo "Error: 最新リリースが見つかりません"
        exit 1
    fi
fi
echo ">> バージョン: ${VERSION}"

# --- ダウンロード URL を取得 ---
echo ">> ダウンロード URL を取得中..."
DOWNLOAD_URL=$(curl -fsSL "${GITHUB_API}/releases/tags/${VERSION}" \
    | grep -Po '"browser_download_url":\s*"\K[^"]*'"${BINARY_NAME}"'[^"]*')
if [ -z "${DOWNLOAD_URL}" ]; then
    echo "Error: バイナリ ${BINARY_NAME} がリリース ${VERSION} に見つかりません"
    exit 1
fi

# --- ユーザー作成 ---
if ! id "${USER}" &>/dev/null; then
    echo ">> ユーザー ${USER} を作成中..."
    useradd --system --create-home --home-dir "${HOME_DIR}" --shell /usr/sbin/nologin "${USER}"
fi

# --- ディレクトリ作成 ---
echo ">> ディレクトリを準備中..."
install -d -o "${USER}" -g "${USER}" -m 755 "${BIN_DIR}"
install -d -o "${USER}" -g "${USER}" -m 750 "${CONFIG_DIR}"

# --- バイナリダウンロード ---
echo ">> バイナリをダウンロード中: ${BINARY_NAME} (${VERSION})"
curl -fsSL -o "${BIN_DIR}/picapica-nest.tmp" "${DOWNLOAD_URL}"
chmod 755 "${BIN_DIR}/picapica-nest.tmp"

# アトミックに置き換え
mv "${BIN_DIR}/picapica-nest.tmp" "${BIN_DIR}/picapica-nest"
chown "${USER}:${USER}" "${BIN_DIR}/picapica-nest"
echo ">> バイナリ: ${BIN_DIR}/picapica-nest"

# --- 初期設定 ---
if [ ! -f "${CONFIG_DIR}/config.json" ]; then
    echo ""
    echo "=========================================="
    echo "  config.json を作成してください:"
    echo "  ${CONFIG_DIR}/config.json"
    echo ""
    echo "  最低限必要な設定:"
    echo "  - providers.anthropic.api_key"
    echo "  - channels.discord.bot_token"
    echo "=========================================="
fi

# --- systemd サービス ---
SCRIPT_DIR="$(cd "$(dirname "$0")" && pwd)"
SERVICE_FILE="${SCRIPT_DIR}/${SERVICE_NAME}.service"
if [ ! -f "${SERVICE_FILE}" ]; then
    echo "Error: ${SERVICE_FILE} が見つかりません"
    exit 1
fi

echo ">> systemd サービスをインストール中..."
cp "${SERVICE_FILE}" "/etc/systemd/system/${SERVICE_NAME}.service"
systemctl daemon-reload
systemctl enable "${SERVICE_NAME}"

echo ""
echo "=========================================="
echo "  デプロイ完了!"
echo ""
echo "  config.json を設定後、以下で起動:"
echo "    systemctl start ${SERVICE_NAME}"
echo ""
echo "  ログ確認:"
echo "    journalctl -u ${SERVICE_NAME} -f"
echo ""
echo "  ステータス確認:"
echo "    systemctl status ${SERVICE_NAME}"
echo "=========================================="
