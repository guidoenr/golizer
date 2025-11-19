#!/bin/bash
set -euo pipefail

REPO_ROOT="${REPO_ROOT:-$(pwd)}"
PI_OUTPUT="${REPO_ROOT}/golizer-pi"
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

if [[ "${SKIP_ARM64:-0}" -ne 1 ]]; then
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
      echo "    go build ${BUILD_TAGS} -> ${PI_OUTPUT}"
      env "${ARM_ENV[@]}" go build -tags "${BUILD_TAGS}" -o "${PI_OUTPUT}" ./cmd/visualizer 2>&1 | tee build-pi.log
    else
      echo "    go build -> ${PI_OUTPUT}"
      env "${ARM_ENV[@]}" go build -o "${PI_OUTPUT}" ./cmd/visualizer 2>&1 | tee build-pi.log
    fi
    STATUS=$?
    set -e
    if [[ ${STATUS} -ne 0 ]]; then
      echo "    Cross-compilation failed; set SKIP_ARM64=1 to skip this step."
      echo "    See build-pi.log for details."
      rm -f "${PI_OUTPUT}"
    else
      echo "    Build succeeded. Log: build-pi.log"
    fi
  fi
else
  echo "==> SKIP_ARM64=1 -> skipping Raspberry Pi build"
fi

popd >/dev/null

echo ""
echo "Binaries generated:" 
[[ -f "${PI_OUTPUT}" ]] && echo "  ${PI_OUTPUT}"
