#!/bin/bash

set -euo pipefail

#git pull;

REPO_ROOT="${REPO_ROOT:-$(pwd)}"
SUDO_BIN="${SUDO:-sudo}"
INSTALL_ROOT="/usr/local/lib/golizer"
WRAPPER_PATH="/usr/local/bin/golizer"

echo "==> Downloading Go dependencies"
pushd "${REPO_ROOT}" >/dev/null
go mod tidy

echo "==> Detecting render backend support"
BUILD_TAGS="${BUILD_TAGS:-}"
if pkg-config --exists sdl2 >/dev/null 2>&1; then
	if [[ -z "${BUILD_TAGS}" ]]; then
		BUILD_TAGS="sdl"
	else
		BUILD_TAGS="${BUILD_TAGS} sdl"
	fi
	echo "    SDL2 detected -> enabling SDL backend (-tags ${BUILD_TAGS})"
else
	echo "    SDL2 not detected -> building ASCII backend only"
fi

echo "==> Building golizer binary"
if [[ -n "${BUILD_TAGS}" ]]; then
	go build -tags "${BUILD_TAGS}" -o golizer ./cmd/visualizer
else
	go build -o golizer ./cmd/visualizer
fi
popd >/dev/null

echo "==> Installing binary with high-priority wrapper (sudo may prompt)"
"${SUDO_BIN}" install -d "${INSTALL_ROOT}"
"${SUDO_BIN}" install -m 0755 "${REPO_ROOT}/golizer" "${INSTALL_ROOT}/golizer.bin"
rm -f "${REPO_ROOT}/golizer"

TEMP_WRAPPER="${REPO_ROOT}/.golizer_wrapper.tmp"
cat > "${TEMP_WRAPPER}" <<'EOF'
#!/bin/bash
set -euo pipefail
BIN_DIR="/usr/local/lib/golizer"
TARGET="${BIN_DIR}/golizer.bin"
if [[ ! -x "${TARGET}" ]]; then
	echo "golizer wrapper error: missing binary at ${TARGET}" >&2
	exit 1
fi
exec nice -n -5 "${TARGET}" "$@"
EOF

"${SUDO_BIN}" install -m 0755 "${TEMP_WRAPPER}" "${WRAPPER_PATH}"
rm -f "${TEMP_WRAPPER}"

echo ""
echo "golizer is ready!"
echo "Binary: ${INSTALL_ROOT}/golizer.bin"
echo "Wrapper: ${WRAPPER_PATH} (runs with nice -n -5)"
echo ""
echo "If this is your first time installing Go, reload your shell or run:"
echo "  source ~/.profile"
echo ""
echo "Recommended Raspberry Pi run:"
echo "  golizer --quality auto"


