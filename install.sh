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
  echo "  - 无法获取最新版本，将尝试从 main 分支下载..."
  echo ""
  echo "注意：直接使用 main 分支二进制可能不是最新稳定版。"
  echo "建议在 https://github.com/$REPO/releases 查看 latest release。"
  echo ""
fi
echo "  版本: ${VERSION:-main}"

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

# Download (fall back to raw github content for latest if version is empty)
if [[ -z "$VERSION" ]]; then
  URL="https://github.com/$REPO/releases/download/v0.0.1/$BINARY-linux-$ARCH"
  echo "  下载稳定版: $URL"
else
  URL="https://github.com/$REPO/releases/download/$VERSION/$BINARY-linux-$ARCH"
  echo "  下载: $URL"
fi
echo ""
if ! curl -sSL "$URL" -o "$BINDIR/$BINARY"; then
  echo "  - 下载失败，尝试备用方式..."
  echo "  - 请手动下载: $URL"
  exit 1
fi
chmod +x "$BINDIR/$BINARY"

echo ""
echo "  安装完成！"
echo ""
echo "  运行方式:"
echo "    sudo linux-helper"
echo ""
