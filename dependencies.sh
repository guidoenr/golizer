#!/usr/bin/env bash

set -euo pipefail

# all system dependencies needed for golizer (excluding Go)
APT_PACKAGES=(
	portaudio19-dev      # audio capture (PortAudio)
	libsdl2-2.0-0        # SDL2 runtime (for SDL backend)
	libsdl2-dev          # SDL2 development headers (for building)
	avahi-daemon         # mDNS/Bonjour for golizer.local access
	avahi-utils          # avahi utilities
	build-essential      # gcc, make, etc. (for building)
	pkg-config           # pkg-config (for finding libraries)
)

if command -v sudo >/dev/null 2>&1 && [[ "${EUID}" -ne 0 ]]; then
	SUDO="sudo"
elif [[ "${EUID}" -eq 0 ]]; then
	SUDO=""
else
	echo "sudo is required to install system packages."
	exit 1
fi

echo "==> Installing golizer dependencies..."
echo "    packages: ${APT_PACKAGES[*]}"
"${SUDO}" apt-get update
"${SUDO}" apt-get install -y "${APT_PACKAGES[@]}"

# enable and start avahi-daemon for mDNS
if "${SUDO}" systemctl is-enabled avahi-daemon >/dev/null 2>&1; then
	echo "==> avahi-daemon already enabled"
else
	echo "==> Enabling avahi-daemon for mDNS support"
	"${SUDO}" systemctl enable avahi-daemon
fi

if "${SUDO}" systemctl is-active avahi-daemon >/dev/null 2>&1; then
	echo "==> avahi-daemon already running"
else
	echo "==> Starting avahi-daemon"
	"${SUDO}" systemctl start avahi-daemon
fi

echo ""
echo "✓ All dependencies installed!"
echo ""
echo "Next steps:"
echo "  1. Install Go (if not already installed):"
echo "     - Download from https://go.dev/dl/"
echo "     - Or use your system's package manager"
echo "  2. Build golizer:"
echo "     ./build.sh"
echo "  3. Run:"
echo "     ./golizer-pi  # or ./golizer-debian"

