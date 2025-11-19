#!/usr/bin/env bash
#
# Script genérico para ejecutar golizer (detecta automáticamente la plataforma)
#
# USO:
#   ./run.sh                    # Ejecutar con configuración por defecto
#   ./run.sh --fps 60           # Sobrescribir parámetros
#   FULLSCREEN=true ./run.sh    # Forzar fullscreen
#

set -euo pipefail

# Detectar arquitectura y binario correspondiente
ARCH="$(uname -m)"
case "${ARCH}" in
  x86_64 | amd64)
    PLATFORM="debian"
    BINARY_NAME="golizer-debian"
    DEFAULT_BACKEND="${DEFAULT_BACKEND:-ascii}"
    ;;
  aarch64 | arm64 | armv7l | armv6l)
    PLATFORM="pi"
    BINARY_NAME="golizer-pi"
    DEFAULT_BACKEND="${DEFAULT_BACKEND:-sdl}"
    # Variables de entorno SDL para Raspberry Pi
    export SDL_VIDEO_CENTERED=1
    export SDL_VIDEODRIVER="${SDL_VIDEODRIVER:-x11}"
    export SDL_RENDER_SCALE_QUALITY=0
    export SDL_RENDER_DRIVER="${SDL_RENDER_DRIVER:-opengles2}"
    # Desactivar compositores para mejor rendimiento
    killall xcompmgr 2>/dev/null || true
    killall compton 2>/dev/null || true
    ;;
  *)
    echo "Unsupported architecture: ${ARCH}"
    exit 1
    ;;
esac

# Configuración por defecto (puede ser sobrescrita con variables de entorno)
FULLSCREEN="${FULLSCREEN:-false}"
QUALITY="${QUALITY:-auto}"
FPS="${FPS:-auto}"
SCALE="${SCALE:-1.0}"
BACKEND="${BACKEND:-${DEFAULT_BACKEND}}"

# Directorio del script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOLIZER_BIN="${SCRIPT_DIR}/${BINARY_NAME}"

if [[ ! -x "${GOLIZER_BIN}" ]]; then
    echo "Error: ${BINARY_NAME} binary not found at ${GOLIZER_BIN}"
    echo "Run ./build.sh first"
    exit 1
fi

# Mostrar configuración
echo "==> Starting golizer"
echo "    Platform: ${PLATFORM} (${ARCH})"
echo "    Binary: ${BINARY_NAME}"
echo "    Backend: ${BACKEND}"
if [[ "${PLATFORM}" == "pi" ]]; then
    echo "    Video driver: ${SDL_VIDEODRIVER}"
    echo "    Render driver: ${SDL_RENDER_DRIVER}"
fi
echo "    Quality: ${QUALITY}"
echo "    FPS: ${FPS}"
echo "    Fullscreen: ${FULLSCREEN}"
echo ""

# Construir argumentos
ARGS=(
    --backend "${BACKEND}"
    --quality "${QUALITY}"
    --scale "${SCALE}"
)

# Agregar FPS solo si no es "auto"
if [[ "${FPS}" != "auto" ]]; then
    ARGS+=(--fps "${FPS}")
fi

if [[ "${FULLSCREEN}" == "true" ]]; then
    ARGS+=(--fullscreen)
fi

# Ejecutar golizer con los argumentos, permitiendo override desde línea de comando
"${GOLIZER_BIN}" "${ARGS[@]}" "$@"

