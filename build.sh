#!/bin/bash
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-$(pwd)}"
DEBIAN_OUTPUT="${REPO_ROOT}/golizer-debian"
BUILD_TAGS="${BUILD_TAGS:-}"

pushd "${REPO_ROOT}" >/dev/null

go mod tidy

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

echo "==> Building golizer-debian"
if [[ -n "${BUILD_TAGS}" ]]; then
  go build -tags "${BUILD_TAGS}" -o "${DEBIAN_OUTPUT}" ./cmd/visualizer
else
  go build -o "${DEBIAN_OUTPUT}" ./cmd/visualizer
fi

popd >/dev/null

echo ""
echo "Binaries generated:" 
[[ -f "${DEBIAN_OUTPUT}" ]] && echo "  ${DEBIAN_OUTPUT}"

