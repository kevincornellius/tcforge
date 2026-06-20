#!/bin/sh
set -e

REPO="kevincornellius/tcforge"
INSTALL_DIR="/usr/local/bin"
BINARY="tcforge"

OS=$(uname -s)
ARCH=$(uname -m)

case "$OS/$ARCH" in
  Linux/x86_64)  PLATFORM="linux-amd64"  ;;
  Linux/aarch64) PLATFORM="linux-arm64"  ;;
  Darwin/x86_64) PLATFORM="darwin-amd64" ;;
  Darwin/arm64)  PLATFORM="darwin-arm64" ;;
  *)
    echo "Unsupported platform: $OS/$ARCH"
    echo "Download manually from https://github.com/$REPO/releases"
    exit 1
    ;;
esac

LATEST=$(curl -fsSL "https://api.github.com/repos/$REPO/releases/latest" \
  | grep '"tag_name"' | sed -E 's/.*"([^"]+)".*/\1/')

if [ -z "$LATEST" ]; then
  echo "Could not find a release. Check https://github.com/$REPO/releases"
  exit 1
fi

URL="https://github.com/$REPO/releases/download/$LATEST/tcforge-$PLATFORM"
TMP="/tmp/tcforge-install-$$"

echo "Downloading tcforge $LATEST ($PLATFORM)..."
curl -fsSL "$URL" -o "$TMP"
chmod +x "$TMP"

# Sanity check — make sure we got a binary, not an error page
if ! "$TMP" --version > /dev/null 2>&1 && ! "$TMP" --help > /dev/null 2>&1; then
  echo "Downloaded file does not appear to be a valid tcforge binary."
  rm -f "$TMP"
  exit 1
fi

if [ -w "$INSTALL_DIR" ]; then
  mv "$TMP" "$INSTALL_DIR/$BINARY"
else
  sudo mv "$TMP" "$INSTALL_DIR/$BINARY"
fi

echo "tcforge $LATEST installed to $INSTALL_DIR/$BINARY"
echo "Run 'tcforge --help' to get started."
