#!/usr/bin/env bash
set -e

VERSION=4.1.18
OS=$(uname -s)
ARCH=$(uname -m)

case "$OS" in
  Darwin)
    case "$ARCH" in
      arm64) BINARY="tailwindcss-macos-arm64" ;;
      x86_64) BINARY="tailwindcss-macos-x64" ;;
      *) echo "unknown architecture: $ARCH"; exit 1 ;;
    esac
    ;;
  Linux)
    case "$ARCH" in
      aarch64) BINARY="tailwindcss-linux-arm64" ;;
      x86_64) BINARY="tailwindcss-linux-x64" ;;
      *) echo "unknown architecture: $ARCH"; exit 1 ;;
    esac
    ;;
  MINGW*|MSYS*|CYGWIN*)
    BINARY="tailwindcss-windows-x64.exe"
    ;;
  *)
    echo "unknown operating system: $OS"
    exit 1
    ;;
esac

DOWNLOAD_URL="https://github.com/tailwindlabs/tailwindcss/releases/download/v${VERSION}/${BINARY}"

curl -sLo "$1/tailwindcss" "$DOWNLOAD_URL"
chmod +x "$1/tailwindcss"
