#!/bin/bash
# Linux Helper — 下载安装脚本
# 自动检测架构，从 GitHub Releases 下载对应二进制
set -euo pipefail

REPO="vansour/linux-helper"
BINDIR="/usr/local/bin"
BINARY="linux-helper"

echo ""
echo "  Linux Helper 安装器"
echo "  ===================="
echo ""

# Detect version
echo "  正在获取最新版本..."
VERSION=$(curl -sSL "https://api.github.com/repos/$REPO/releases/latest" 2>/dev/null \
  | grep '"tag_name"' | cut -d'"' -f4 2>/dev/null || echo "")
if [[ -z "$VERSION" ]]; then
  VERSION="latest"
fi
echo "  版本: $VERSION"

# Detect architecture
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64)  ARCH="amd64" ;;
  aarch64) ARCH="arm64"  ;;
  i386|i686) ARCH="386" ;;
  *)
    echo "  错误: 不支持的架构: $ARCH"
    exit 1
    ;;
esac
echo "  架构: $ARCH"

# Download
URL="https://github.com/$REPO/releases/download/$VERSION/$BINARY-linux-$ARCH"
echo "  下载: $URL"
echo ""
curl -sSL "$URL" -o "$BINDIR/$BINARY"
chmod +x "$BINDIR/$BINARY"

echo ""
echo "  安装完成！"
echo ""
echo "  运行方式:"
echo "    sudo linux-helper"
echo ""
