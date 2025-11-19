#!/bin/bash
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-$(pwd)}"
DEBIAN_OUTPUT="${REPO_ROOT}/golizer-debian"
PI_OUTPUT="${REPO_ROOT}/golizer-pi"

# Skip arm64 cross build automatically if we are already on arm
if [[ $(uname -m) == "aarch64" || $(uname -m) == "arm64" ]]; then
  SKIP_ARM64=1
fi

# Optional verbose mode
BUILD_FLAGS=()
if [[ "${VERBOSE_BUILD:-0}" -eq 1 ]]; then
  BUILD_FLAGS+=(-v -x)
  echo "==> Verbose build enabled"
fi

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

echo "==> Building golizer-debian"
if [[ -n "${BUILD_TAGS}" ]]; then
  go build ${BUILD_FLAGS[*]} -tags "${BUILD_TAGS}" -o "${DEBIAN_OUTPUT}" ./cmd/visualizer
else
  go build ${BUILD_FLAGS[*]} -o "${DEBIAN_OUTPUT}" ./cmd/visualizer
fi

default_skip=${SKIP_ARM64:-0}
if [[ "${default_skip}" -ne 1 ]]; then
  echo "==> Building golizer-pi (arm64)"
  ARM_ENV=(GOOS=linux GOARCH=arm64 CGO_ENABLED=1)
  if command -v aarch64-linux-gnu-gcc >/dev/null 2>&1; then
    ARM_ENV+=(CC=aarch64-linux-gnu-gcc CXX=aarch64-linux-gnu-g++)
  else
    echo "    aarch64-linux-gnu-gcc not found; skipping arm64 build."
    ARM_ENV=()
  fi
  if [[ -n "${ARM_ENV[*]}" ]]; then
    set +e
    if [[ -n "${BUILD_TAGS}" ]]; then
      env "${ARM_ENV[@]}" go build ${BUILD_FLAGS[*]} -tags "${BUILD_TAGS}" -o "${PI_OUTPUT}" ./cmd/visualizer
    else
      env "${ARM_ENV[@]}" go build ${BUILD_FLAGS[*]} -o "${PI_OUTPUT}" ./cmd/visualizer
    fi
    STATUS=$?
    set -e
    if [[ ${STATUS} -ne 0 ]]; then
      echo "    Cross-compilation failed; set SKIP_ARM64=1 to skip this step."
      rm -f "${PI_OUTPUT}"
    fi
  fi
else
  echo "==> SKIP_ARM64=1 -> skipping Raspberry Pi build"
fi

popd >/dev/null

echo ""
echo "Binaries generated:"
[[ -f "${DEBIAN_OUTPUT}" ]] && echo "  ${DEBIAN_OUTPUT}"
[[ -f "${PI_OUTPUT}" ]] && echo "  ${PI_OUTPUT}"

