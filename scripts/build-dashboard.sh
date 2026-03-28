#!/usr/bin/env bash
# build-dashboard.sh — Descarga Tailwind CLI si no existe y compila CSS para el dashboard.
# Uso: ./scripts/build-dashboard.sh
set -euo pipefail

TOOLS_DIR="$(cd "$(dirname "$0")/.." && pwd)/tools"
WEB_DIR="$(cd "$(dirname "$0")/.." && pwd)/internal/web"
TAILWIND="$TOOLS_DIR/tailwindcss"

# Detectar arquitectura
ARCH=$(uname -m)
case "$ARCH" in
    x86_64)  TAILWIND_BIN="tailwindcss-linux-x64" ;;
    aarch64) TAILWIND_BIN="tailwindcss-linux-arm64" ;;
    *)       echo "Arquitectura no soportada: $ARCH"; exit 1 ;;
esac

# Descargar Tailwind CLI si no existe
if [ ! -x "$TAILWIND" ]; then
    echo "Descargando Tailwind CLI standalone..."
    mkdir -p "$TOOLS_DIR"
    curl -sLo "$TAILWIND" "https://github.com/tailwindlabs/tailwindcss/releases/latest/download/$TAILWIND_BIN"
    chmod +x "$TAILWIND"
    echo "Tailwind CLI descargado en $TAILWIND"
fi

# Crear input CSS para Tailwind v4
mkdir -p "$WEB_DIR/build"
echo '@import "tailwindcss";' > "$WEB_DIR/build/input.css"

# Compilar CSS escaneando index.html
"$TAILWIND" \
    --input "$WEB_DIR/build/input.css" \
    --output "$WEB_DIR/build/output.css" \
    --content "$WEB_DIR/index.html" \
    --minify

echo "CSS compilado en $WEB_DIR/build/output.css"
echo "Tamano: $(wc -c < "$WEB_DIR/build/output.css") bytes"
