#!/usr/bin/env bash

set -euo pipefail

GO_VERSION="${GO_VERSION:-1.24.10}"
APT_PACKAGES=(curl tar git build-essential portaudio19-dev)

ARCH="$(uname -m)"
case "${ARCH}" in
	aarch64 | arm64) GO_ARCH="arm64" ;;
	armv7l | armv6l) GO_ARCH="armv6l" ;;
	x86_64 | amd64) GO_ARCH="amd64" ;;
	*)
		echo "Unsupported architecture: ${ARCH}"
		exit 1
		;;
esac

if command -v sudo >/dev/null 2>&1 && [[ "${EUID}" -ne 0 ]]; then
	SUDO="sudo"
elif [[ "${EUID}" -eq 0 ]]; then
	SUDO=""
else
	echo "sudo is required to install system packages."
	exit 1
fi

echo "==> Installing apt packages: ${APT_PACKAGES[*]}"
"${SUDO}" apt-get update
"${SUDO}" apt-get install -y "${APT_PACKAGES[@]}"

TMPDIR="$(mktemp -d)"
cleanup() {
	rm -rf "${TMPDIR}"
}
trap cleanup EXIT

INSTALL_GO=false
if command -v go >/dev/null 2>&1; then
	INSTALLED_VERSION="$(go version | awk '{print $3}')"
	if [[ "${INSTALLED_VERSION}" != "go${GO_VERSION}" ]]; then
		echo "==> Updating Go from ${INSTALLED_VERSION} to go${GO_VERSION}"
		INSTALL_GO=true
	else
		echo "==> Go ${GO_VERSION} already installed"
	fi
else
	echo "==> Installing Go ${GO_VERSION}"
	INSTALL_GO=true
fi

if [[ "${INSTALL_GO}" == true ]]; then
	GO_TARBALL="go${GO_VERSION}.linux-${GO_ARCH}.tar.gz"
	curl -fsSL "https://go.dev/dl/${GO_TARBALL}" -o "${TMPDIR}/go.tgz"
	"${SUDO}" rm -rf /usr/local/go
	"${SUDO}" tar -C /usr/local -xzf "${TMPDIR}/go.tgz"
fi

export PATH="/usr/local/go/bin:${PATH}"
if ! grep -qs '/usr/local/go/bin' "${HOME}/.profile"; then
	echo 'export PATH=/usr/local/go/bin:$PATH' >>"${HOME}/.profile"
fi

mkdir -p "${HOME}/.local/bin"
if ! grep -qs "${HOME}/.local/bin" "${HOME}/.profile"; then
	echo 'export PATH=$HOME/.local/bin:$PATH' >>"${HOME}/.profile"
fi
export PATH="${HOME}/.local/bin:${PATH}"

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
if [[ ! -f "${REPO_ROOT}/go.mod" ]]; then
	echo "Could not locate go.mod in ${REPO_ROOT}"
	exit 1
fi

