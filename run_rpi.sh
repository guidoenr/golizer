#!/usr/bin/env bash
#
# Script para ejecutar golizer en Raspberry Pi con configuración SDL optimizada
#
# USO:
#   ./run_rpi.sh                    # Ejecutar con configuración por defecto
#   ./run_rpi.sh --fps 60           # Sobrescribir parámetros
#   FULLSCREEN=false ./run_rpi.sh   # Ejecutar en modo ventana
#

set -euo pipefail

# Configuración por defecto (puede ser sobrescrita con variables de entorno)
FULLSCREEN="${FULLSCREEN:-true}"
QUALITY="${QUALITY:-eco}"
FPS="${FPS:-30}"
SCALE="${SCALE:-1.0}"

# Variables de entorno SDL para Raspberry Pi
# Estas mejoran la compatibilidad y rendimiento en plataformas ARM
export SDL_VIDEO_CENTERED=1
export SDL_VIDEODRIVER="${SDL_VIDEODRIVER:-x11}"
export SDL_RENDER_SCALE_QUALITY=0
export SDL_RENDER_DRIVER="${SDL_RENDER_DRIVER:-opengles2}"

# Desactivar compositor si está disponible (reduce overhead gráfico)
if command -v xcompmgr >/dev/null 2>&1; then
    killall xcompmgr 2>/dev/null || true
fi
if command -v compton >/dev/null 2>&1; then
    killall compton 2>/dev/null || true
fi

# Directorio del script
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
GOLIZER_BIN="${SCRIPT_DIR}/golizer"

if [[ ! -x "${GOLIZER_BIN}" ]]; then
    echo "Error: golizer binary not found at ${GOLIZER_BIN}"
    echo "Run ./build.sh first"
    exit 1
fi

# Mostrar configuración
echo "==> Starting golizer on Raspberry Pi"
echo "    Video driver: ${SDL_VIDEODRIVER}"
echo "    Render driver: ${SDL_RENDER_DRIVER}"
echo "    Quality: ${QUALITY}"
echo "    FPS: ${FPS}"
echo "    Fullscreen: ${FULLSCREEN}"
echo "    Binary: ${GOLIZER_BIN}"
echo ""

# Construir argumentos
ARGS=(
    --backend sdl
    --quality "${QUALITY}"
    --fps "${FPS}"
    --scale "${SCALE}"
)

if [[ "${FULLSCREEN}" == "true" ]]; then
    ARGS+=(--fullscreen)
fi

# Ejecutar golizer con los argumentos, permitiendo override desde línea de comando
"${GOLIZER_BIN}" "${ARGS[@]}" "$@"
