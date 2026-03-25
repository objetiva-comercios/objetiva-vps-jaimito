#!/bin/bash
# =============================================================================
# jaimito — Instalador automatico
# =============================================================================
# Uso:
#   curl -sL https://raw.githubusercontent.com/objetiva-comercios/objetiva-vps-jaimito/main/install.sh | bash
#
# O desde el VPS:
#   bash install.sh
#
# Que hace:
#   1. Verifica dependencias (git, go 1.24+)
#   2. Clona o actualiza el repositorio
#   3. Compila el binario Go
#   4. Instala en /usr/local/bin/jaimito
#   5. Crea directorio de datos /var/lib/jaimito
#   6. Copia config de ejemplo si no existe
#   7. Instala y activa el servicio systemd
#   8. Verifica que el servicio este corriendo
#
# Requisitos:
#   - Go 1.24+
#   - git
#   - systemd
#   - sudo
# =============================================================================

set -euo pipefail

# -- Config ------------------------------------------------------------------
INSTALL_DIR="${HOME}/proyectos"
REPO_DIR="${INSTALL_DIR}/objetiva-vps-jaimito"
REPO_URL="https://github.com/objetiva-comercios/objetiva-vps-jaimito.git"
BINARY_NAME="jaimito"
BINARY_DEST="/usr/local/bin/${BINARY_NAME}"
CONFIG_DIR="/etc/jaimito"
CONFIG_FILE="${CONFIG_DIR}/config.yaml"
DATA_DIR="/var/lib/jaimito"
SERVICE_NAME="jaimito"
SERVICE_FILE="systemd/jaimito.service"
HEALTH_URL="http://127.0.0.1:8080/api/v1/health"
GO_MIN_VERSION="1.24"

# -- Colores -----------------------------------------------------------------
RED='\033[0;31m'
GREEN='\033[0;32m'
YELLOW='\033[1;33m'
CYAN='\033[0;36m'
NC='\033[0m'

info()  { echo -e "${CYAN}[INFO]${NC}  $*"; }
ok()    { echo -e "${GREEN}[OK]${NC}    $*"; }
warn()  { echo -e "${YELLOW}[WARN]${NC}  $*"; }
error() { echo -e "${RED}[ERROR]${NC} $*"; exit 1; }

# -- Banner ------------------------------------------------------------------
echo ""
echo "=========================================="
echo "  jaimito — Instalador"
echo "  VPS Push Notification Hub"
echo "=========================================="
echo ""

# -- Verificar dependencias --------------------------------------------------
info "Verificando dependencias..."

command -v git >/dev/null 2>&1 || error "git no encontrado. Instalar con: sudo apt install git"
ok "git encontrado"

GO_INSTALLED=true
command -v go >/dev/null 2>&1 || GO_INSTALLED=false

if [ "$GO_INSTALLED" = true ]; then
    GO_VERSION=$(go version | grep -oP '\d+\.\d+' | head -1)
    GO_MAJOR=$(echo "$GO_VERSION" | cut -d. -f1)
    GO_MINOR=$(echo "$GO_VERSION" | cut -d. -f2)
    REQ_MAJOR=$(echo "$GO_MIN_VERSION" | cut -d. -f1)
    REQ_MINOR=$(echo "$GO_MIN_VERSION" | cut -d. -f2)

    if [ "$GO_MAJOR" -lt "$REQ_MAJOR" ] || { [ "$GO_MAJOR" -eq "$REQ_MAJOR" ] && [ "$GO_MINOR" -lt "$REQ_MINOR" ]; }; then
        warn "Go ${GO_VERSION} encontrado, se requiere ${GO_MIN_VERSION}+"
        GO_INSTALLED=false
    else
        ok "go ${GO_VERSION} encontrado"
    fi
fi

if [ "$GO_INSTALLED" = false ]; then
    GO_TAR="go${GO_MIN_VERSION}.1.linux-$(dpkg --print-architecture 2>/dev/null || echo amd64).tar.gz"
    info "Instalando Go ${GO_MIN_VERSION}..."
    curl -sL "https://go.dev/dl/${GO_TAR}" | sudo tar -C /usr/local -xzf -
    export PATH=$PATH:/usr/local/go/bin
    if ! grep -q '/usr/local/go/bin' "$HOME/.bashrc" 2>/dev/null; then
        echo 'export PATH=$PATH:/usr/local/go/bin' >> "$HOME/.bashrc"
    fi
    ok "Go $(go version | grep -oP '\d+\.\d+\.\d+' | head -1) instalado"
fi

command -v systemctl >/dev/null 2>&1 || error "systemctl no encontrado. Este instalador requiere systemd."
ok "systemd encontrado"

# -- Manejar instalacion previa ----------------------------------------------
REINSTALL=false
if [ -d "$REPO_DIR" ]; then
    warn "Instalacion previa detectada en ${REPO_DIR}"
    info "Deteniendo servicio si esta corriendo..."
    sudo systemctl stop "${SERVICE_NAME}" 2>/dev/null || true
    REINSTALL=true

    info "Actualizando repositorio..."
    cd "$REPO_DIR"
    git fetch origin
    git reset --hard origin/main
    ok "Repositorio actualizado"
else
    # -- Clonar repositorio --------------------------------------------------
    info "Clonando repositorio..."
    mkdir -p "$INSTALL_DIR"
    git clone "$REPO_URL" "$REPO_DIR"
    cd "$REPO_DIR"
    ok "Repositorio clonado en ${REPO_DIR}"
fi

# -- Compilar ----------------------------------------------------------------
info "Compilando binario Go..."
cd "$REPO_DIR"
go build -o "${BINARY_NAME}" ./cmd/jaimito/
ok "Binario compilado: ${BINARY_NAME}"

# -- Instalar binario --------------------------------------------------------
info "Instalando binario en ${BINARY_DEST}..."
sudo cp "${BINARY_NAME}" "${BINARY_DEST}"
sudo chmod 755 "${BINARY_DEST}"
ok "Binario instalado"

# -- Crear directorio de datos -----------------------------------------------
info "Creando directorio de datos ${DATA_DIR}..."
sudo mkdir -p "${DATA_DIR}"
ok "Directorio de datos listo"

# -- Config ------------------------------------------------------------------
CONFIG_NEEDS_EDIT=false
if [ ! -f "$CONFIG_FILE" ]; then
    info "Iniciando wizard de configuracion interactivo..."
    sudo mkdir -p "${CONFIG_DIR}"
    if sudo "${BINARY_DEST}" setup < /dev/tty; then
        ok "Configuracion completada via wizard"
    else
        warn "El wizard no completo la configuracion."
        warn "Instala el servicio pero no lo iniciamos."
        warn "Cuando estes listo: sudo jaimito setup"
        CONFIG_NEEDS_EDIT=true
    fi
else
    ok "Config existente encontrada en ${CONFIG_FILE}"
    printf "${YELLOW}[WARN]${NC}  Reconfigurar con el wizard? (s/n): "
    read -r RECONFIG < /dev/tty
    if [ "$RECONFIG" = "s" ] || [ "$RECONFIG" = "S" ]; then
        if sudo "${BINARY_DEST}" setup < /dev/tty; then
            ok "Reconfiguracion completada"
        else
            warn "Wizard cancelado. Configuracion anterior preservada."
        fi
    else
        ok "Configuracion existente preservada"
    fi
fi

# -- Instalar servicio systemd -----------------------------------------------
info "Instalando servicio systemd..."
sudo cp "${SERVICE_FILE}" "/etc/systemd/system/${SERVICE_NAME}.service"
sudo systemctl daemon-reload
sudo systemctl enable "${SERVICE_NAME}"
ok "Servicio ${SERVICE_NAME} instalado y habilitado"

# -- Iniciar servicio --------------------------------------------------------
if [ "$CONFIG_NEEDS_EDIT" = true ]; then
    warn "Servicio NO iniciado — completar la configuracion primero:"
    warn "  sudo jaimito setup"
    warn "  sudo systemctl start ${SERVICE_NAME}"
else
    info "Iniciando servicio..."
    sudo systemctl start "${SERVICE_NAME}"

    # -- Health check --------------------------------------------------------
    info "Verificando que el servicio este corriendo..."
    RETRIES=0
    MAX_RETRIES=10
    while [ $RETRIES -lt $MAX_RETRIES ]; do
        if curl -sf "${HEALTH_URL}" >/dev/null 2>&1; then
            ok "Servicio corriendo y respondiendo en ${HEALTH_URL}"
            break
        fi
        RETRIES=$((RETRIES + 1))
        sleep 2
    done

    if [ $RETRIES -eq $MAX_RETRIES ]; then
        warn "Health check no respondio despues de ${MAX_RETRIES} intentos"
        warn "Verificar logs: sudo journalctl -u ${SERVICE_NAME} --no-pager -n 30"
    fi
fi

# -- Resultado ---------------------------------------------------------------
echo ""
echo "=========================================="
echo "  Instalacion completa"
echo "=========================================="
echo ""
echo -e "${CYAN}Binario:${NC}    ${BINARY_DEST}"
echo -e "${CYAN}Config:${NC}     ${CONFIG_FILE}"
echo -e "${CYAN}Base datos:${NC} ${DATA_DIR}/jaimito.db"
echo -e "${CYAN}Servicio:${NC}   ${SERVICE_NAME}"
echo ""
echo -e "${CYAN}Comandos utiles:${NC}"
echo "  sudo systemctl status ${SERVICE_NAME}     # Estado"
echo "  sudo journalctl -u ${SERVICE_NAME} -f     # Logs en vivo"
echo "  sudo systemctl restart ${SERVICE_NAME}     # Reiniciar"
echo ""
# Extraer API key del config para mostrar el export listo
API_KEY=$(grep -A1 'seed_api_keys' "${CONFIG_FILE}" 2>/dev/null | grep 'key:' | head -1 | sed 's/.*key: *"\{0,1\}\([^"]*\)"\{0,1\}/\1/' | xargs)
if [ -n "$API_KEY" ] && [ "$API_KEY" != "sk-REPLACE_ME_WITH_A_REAL_KEY" ]; then
    echo -e "${CYAN}API Key (copiar y guardar):${NC}"
    echo -e "  ${YELLOW}export JAIMITO_API_KEY=${API_KEY}${NC}"
    echo ""
    # Agregar al .bashrc si no esta
    if ! grep -q "JAIMITO_API_KEY" "$HOME/.bashrc" 2>/dev/null; then
        echo "export JAIMITO_API_KEY=${API_KEY}" >> "$HOME/.bashrc"
        ok "API key agregada a ~/.bashrc (disponible en nuevas sesiones)"
    fi
    echo ""
fi
echo -e "${CYAN}Enviar notificacion:${NC}"
echo "  jaimito send \"Hola desde el VPS\""
echo "  jaimito send -c deploys -p high \"Deploy exitoso\""
echo ""
echo -e "${CYAN}Monitorear cron job:${NC}"
echo "  jaimito wrap -c cron -- /path/to/script.sh"
echo ""
echo -e "${CYAN}Gestionar API keys:${NC}"
echo "  jaimito keys create --name mi-servicio"
echo "  jaimito keys list"
echo "  jaimito keys revoke <id>"
echo ""
