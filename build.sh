#!/bin/bash
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-$(pwd)}"
OUTPUT="${REPO_ROOT}/golizer"
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

echo "==> Building golizer"
if [[ -n "${BUILD_TAGS}" ]]; then
  go build -tags "${BUILD_TAGS}" -o "${OUTPUT}" ./cmd/visualizer
else
  go build -o "${OUTPUT}" ./cmd/visualizer
fi

popd >/dev/null

echo ""
echo "Binary generated: ${OUTPUT}"

