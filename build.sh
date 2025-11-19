#!/bin/bash
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-$(pwd)}"
BUILD_TAGS="${BUILD_TAGS:-}"

# Detectar arquitectura actual
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64 | amd64)
    TARGET="debian"
    OUTPUT="${REPO_ROOT}/golizer-debian"
    GOOS="linux"
    GOARCH="amd64"
    ;;
  aarch64 | arm64)
    TARGET="pi"
    OUTPUT="${REPO_ROOT}/golizer-pi"
    GOOS="linux"
    GOARCH="arm64"
    ;;
  armv7l | armv6l)
    TARGET="pi"
    OUTPUT="${REPO_ROOT}/golizer-pi"
    GOOS="linux"
    GOARCH="arm"
    ;;
  *)
    echo "Unsupported architecture: ${ARCH}"
    exit 1
    ;;
esac

pushd "${REPO_ROOT}" >/dev/null

go mod tidy

echo "==> Detecting platform"
echo "    Architecture: ${ARCH}"
echo "    Target: ${TARGET} (${GOOS}/${GOARCH})"

echo ""
echo "==> Detecting render backend support"
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

echo ""
echo "==> Building golizer for ${TARGET}"
if [[ -n "${BUILD_TAGS}" ]]; then
  GOOS="${GOOS}" GOARCH="${GOARCH}" go build -tags "${BUILD_TAGS}" -o "${OUTPUT}" ./cmd/visualizer
else
  GOOS="${GOOS}" GOARCH="${GOARCH}" go build -o "${OUTPUT}" ./cmd/visualizer
fi

popd >/dev/null

echo ""
echo "============================================"
echo "✓ Build complete!"
echo "  Platform: ${TARGET} (${ARCH})"
echo "  Binary:   ${OUTPUT}"
echo "============================================"

