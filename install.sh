#!/bin/sh
# Bifrost installer — curl -fsSL https://raw.githubusercontent.com/kogungor/bifrost/dev/install.sh | sh
set -e

REPO="kogungor/bifrost"
INSTALL_DIR="/usr/local/bin"

# Detect OS
OS="$(uname -s | tr '[:upper:]' '[:lower:]')"
case "$OS" in
  linux)  OS="linux" ;;
  darwin) OS="darwin" ;;
  *)      echo "Unsupported OS: $OS"; exit 1 ;;
esac

# Detect arch
ARCH="$(uname -m)"
case "$ARCH" in
  x86_64|amd64)  ARCH="amd64" ;;
  arm64|aarch64) ARCH="arm64" ;;
  *)             echo "Unsupported architecture: $ARCH"; exit 1 ;;
esac

# Get latest version tag
echo "Fetching latest release..."
VERSION="$(curl -fsSL "https://api.github.com/repos/${REPO}/releases/latest" | grep '"tag_name"' | sed -E 's/.*"tag_name":\s*"([^"]+)".*/\1/')"
if [ -z "$VERSION" ]; then
  echo "Error: could not determine latest version"
  exit 1
fi
echo "Latest version: ${VERSION}"

# Download
TARBALL="bifrost_${OS}_${ARCH}.tar.gz"
URL="https://github.com/${REPO}/releases/download/${VERSION}/${TARBALL}"
TMP="$(mktemp -d)"
trap 'rm -rf "$TMP"' EXIT

echo "Downloading ${URL}..."
curl -fsSL "$URL" -o "${TMP}/${TARBALL}"

# Extract
tar -xzf "${TMP}/${TARBALL}" -C "$TMP"

# Install
if [ -w "$INSTALL_DIR" ]; then
  mv "${TMP}/bifrost" "${INSTALL_DIR}/bifrost"
else
  echo "Installing to ${INSTALL_DIR} (requires sudo)..."
  sudo mv "${TMP}/bifrost" "${INSTALL_DIR}/bifrost"
fi

chmod +x "${INSTALL_DIR}/bifrost"

echo "bifrost ${VERSION} installed to ${INSTALL_DIR}/bifrost"
echo ""
echo "Get started:"
echo "  bifrost install        # register slash commands"
echo "  bifrost install --mcp  # also register MCP server"
