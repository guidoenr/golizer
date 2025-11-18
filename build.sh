#!/bin/bash

set -euo pipefail

REPO_ROOT="${REPO_ROOT:-$(pwd)}"
SUDO_BIN="${SUDO:-sudo}"

echo "==> Downloading Go dependencies"
pushd "${REPO_ROOT}" >/dev/null
go mod tidy

echo "==> Building golizer binary"
go build -o golizer ./cmd/visualizer
popd >/dev/null

echo "==> Installing binary to /usr/local/bin (sudo may prompt)"
"${SUDO_BIN}" install -m 0755 "${REPO_ROOT}/golizer" /usr/local/bin/golizer
rm -f "${REPO_ROOT}/golizer"

echo ""
echo "golizer is ready!"
echo "Binary: /usr/local/bin/golizer"
echo ""
echo "If this is your first time installing Go, reload your shell or run:"
echo "  source ~/.profile"
echo ""
echo "Recommended Raspberry Pi run:"
echo "  golizer --quality auto"


