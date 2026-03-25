# Deploy — jaimito

## Instalacion rapida

```bash
curl -sL https://raw.githubusercontent.com/objetiva-comercios/objetiva-vps-jaimito/main/install.sh | bash
```

## Requisitos

- **Go 1.24+** — compilar el binario
- **git** — clonar el repositorio
- **systemd** — gestionar el servicio
- **sudo** — instalar binario y servicio

## Arquitectura

```
┌─────────────┐     HTTP POST       ┌──────────────┐     Bot API      ┌──────────┐
│ cron / apps │ ──────────────────→  │   jaimito    │ ──────────────→  │ Telegram │
│  (clients)  │  /api/v1/notify     │  (Go binary) │   MarkdownV2    │  (chats) │
└─────────────┘     Bearer auth     └──────┬───────┘                  └──────────┘
                                           │
      jaimito send ─────────────→ HTTP API │
      jaimito wrap ─────────────→ HTTP API │
      jaimito keys ─────────────→ SQLite ──┘
                                     │
                              /var/lib/jaimito/
                                jaimito.db
```

**Componentes:**
- **Binario unico**: `/usr/local/bin/jaimito` (servidor + CLI)
- **Config**: `/etc/jaimito/config.yaml`
- **Base de datos**: `/var/lib/jaimito/jaimito.db` (SQLite WAL-mode)
- **Servicio**: `jaimito.service` (systemd)

## Configuracion inicial

### Setup wizard (recomendado)

```bash
sudo jaimito setup
```

El wizard interactivo guía la configuración paso a paso:

1. Pide el **bot token** de Telegram y lo valida contra la API (`getMe`)
2. Pide el **chat ID** del canal general y lo valida (`getChat`)
3. Permite agregar **canales extra** (deploys, errors, cron, etc.) con validación
4. Configura la **dirección HTTP** (default: `127.0.0.1:8080`)
5. Configura la **ruta de base de datos** (default: `/var/lib/jaimito/jaimito.db`)
6. Genera una **API key** automáticamente (prefijo `sk-`)
7. Muestra un **resumen** y ofrece enviar notificación de test

Escribe `/etc/jaimito/config.yaml` con permisos `0600`. Se ejecuta automáticamente durante `install.sh` si no existe config previa. En reinstalaciones, el instalador pregunta si querés reconfigurar.

### Archivo de config (manual)

```bash
sudo nano /etc/jaimito/config.yaml
```

| Campo | Descripcion | Ejemplo |
|-------|-------------|---------|
| `telegram.token` | Bot token de Telegram (obtener de @BotFather) | `123456:ABC-DEF...` |
| `database.path` | Ruta a la base SQLite | `/var/lib/jaimito/jaimito.db` |
| `server.listen` | Direccion de escucha HTTP | `127.0.0.1:8080` |
| `channels[].name` | Nombre del canal | `general`, `cron`, `errors` |
| `channels[].chat_id` | ID del chat de Telegram destino | `-100123456789` |
| `channels[].priority` | Prioridad por defecto del canal | `normal`, `high`, `low`, `critical` |
| `seed_api_keys[].name` | Nombre descriptivo de la API key | `default` |
| `seed_api_keys[].key` | API key con prefijo `sk-` | `sk-abc123...` |

### Variables de entorno (CLI)

| Variable | Descripcion | Ejemplo |
|----------|-------------|---------|
| `JAIMITO_API_KEY` | API key para comandos `send` y `wrap` | `sk-abc123...` |
| `JAIMITO_SERVER` | Direccion del servidor (override config) | `127.0.0.1:8080` |

### Generar una API key

```bash
# Opcion A: via setup wizard (genera automaticamente)
sudo jaimito setup

# Opcion B: usar el CLI despues de instalar
jaimito keys create --name mi-servicio

# Opcion C: generar manualmente con openssl
openssl rand -hex 32 | sed 's/^/sk-/'
```

### Obtener el chat_id de Telegram

1. Crear un bot con [@BotFather](https://t.me/BotFather) y copiar el token
2. Agregar el bot al grupo/canal destino
3. Enviar un mensaje al grupo
4. Visitar `https://api.telegram.org/bot<TOKEN>/getUpdates`
5. Buscar el campo `chat.id` en la respuesta

## Servicios

| Servicio | Puerto | Protocolo | Descripcion |
|----------|--------|-----------|-------------|
| jaimito (HTTP API) | 8080 | HTTP | API de notificaciones (localhost only) |

### Endpoints

| Metodo | Ruta | Auth | Descripcion |
|--------|------|------|-------------|
| `POST` | `/api/v1/notify` | Bearer `sk-*` | Enviar notificacion |
| `GET` | `/api/v1/health` | No | Health check |

## Comandos utiles

```bash
# Estado del servicio
sudo systemctl status jaimito

# Logs en vivo
sudo journalctl -u jaimito -f

# Reiniciar
sudo systemctl restart jaimito

# Detener
sudo systemctl stop jaimito

# Enviar notificacion de prueba
export JAIMITO_API_KEY=sk-tu-key
jaimito send "Test desde el VPS"
jaimito send -c deploys -p high "Deploy exitoso"

# Monitorear un cron job
jaimito wrap -c cron -- /path/to/backup.sh

# Gestionar API keys
jaimito keys create --name nuevo-servicio
jaimito keys list
jaimito keys revoke <id>
```

## Actualizacion

```bash
# Opcion A: re-ejecutar el instalador (detecta instalacion previa)
curl -sL https://raw.githubusercontent.com/objetiva-comercios/objetiva-vps-jaimito/main/install.sh | bash

# Opcion B: manual
cd ~/proyectos/objetiva-vps-jaimito
git pull
go build -o jaimito ./cmd/jaimito/
sudo cp jaimito /usr/local/bin/jaimito
sudo systemctl restart jaimito
```

## Troubleshooting

### El servicio no arranca

```bash
sudo journalctl -u jaimito --no-pager -n 50
```

**Causas comunes:**
- Config no editada (token de Telegram invalido) → ejecutar `sudo jaimito setup`
- `chat_id` incorrecto (el bot no tiene acceso al chat) → el wizard valida esto automaticamente
- Puerto 8080 ya en uso → cambiar `server.listen` en config o reconfigurar con `sudo jaimito setup`

### Notificaciones no llegan a Telegram

1. Verificar que el bot token es valido: `curl https://api.telegram.org/bot<TOKEN>/getMe`
2. Verificar que el bot esta en el chat/grupo destino
3. Verificar que el `chat_id` en la config es correcto
4. Revisar logs: `sudo journalctl -u jaimito -f`

### `jaimito send` falla con "API key required"

```bash
# Configurar la variable de entorno
export JAIMITO_API_KEY=sk-tu-key-aqui

# O usar el flag directamente
jaimito send --key sk-tu-key-aqui "mensaje"
```

### Permisos de la base de datos

```bash
# Si hay errores de permisos en /var/lib/jaimito/
sudo chown $(whoami) /var/lib/jaimito/
```

## Estructura del proyecto

```
objetiva-vps-jaimito/
├── cmd/jaimito/          # Entrypoint y CLI (main, root, serve, send, wrap, keys, setup)
│   └── setup/            # Setup wizard interactivo (bubbletea TUI)
├── configs/
│   └── config.example.yaml
├── internal/
│   ├── api/              # HTTP router, middleware, handlers
│   ├── cleanup/          # Scheduler de purga de mensajes viejos
│   ├── client/           # HTTP client para CLI send/wrap
│   ├── config/           # Parser de config YAML
│   ├── db/               # SQLite: schema, mensajes, API keys
│   ├── dispatcher/       # Polling loop → Telegram con retry
│   └── telegram/         # Bot client y formatter MarkdownV2
├── systemd/
│   └── jaimito.service
├── install.sh
├── go.mod
└── go.sum
```
