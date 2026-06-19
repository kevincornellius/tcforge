#!/bin/sh
set -e

REPO="kevincornellius/tcforge"
INSTALL_DIR="/usr/local/bin"
BINARY="tcforge"

OS=$(uname -s)
ARCH=$(uname -m)

case "$OS/$ARCH" in
  Linux/x86_64)   PLATFORM="linux-amd64" ;;
  Linux/aarch64)  PLATFORM="linux-arm64" ;;
  Darwin/x86_64)  PLATFORM="darwin-amd64" ;;
  Darwin/arm64)   PLATFORM="darwin-arm64" ;;
  *)
    echo "Unsupported platform: $OS/$ARCH"
    echo "Download a binary manually from https://github.com/$REPO/releases"
    exit 1
    ;;
esac

LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')
URL="https://github.com/$REPO/releases/download/$LATEST/tcforge-$PLATFORM"

echo "Installing tcforge $LATEST ($PLATFORM)..."
curl -fsSL "$URL" -o "/tmp/$BINARY"
chmod +x "/tmp/$BINARY"

if [ -w "$INSTALL_DIR" ]; then
  mv "/tmp/$BINARY" "$INSTALL_DIR/$BINARY"
else
  sudo mv "/tmp/$BINARY" "$INSTALL_DIR/$BINARY"
fi

echo "tcforge installed to $INSTALL_DIR/$BINARY"
echo "Run 'tcforge --help' to get started."
