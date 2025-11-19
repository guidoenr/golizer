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
      env "${ARM_ENV[@]}" go build -tags "${BUILD_TAGS}" -o "${PI_OUTPUT}" ./cmd/visualizer
    else
      env "${ARM_ENV[@]}" go build -o "${PI_OUTPUT}" ./cmd/visualizer
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
