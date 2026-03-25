# jaimito

Hub de notificaciones push para VPS. Centraliza todas las alertas generadas en un VPS (eventos de servicios, resultados de cron jobs, errores de aplicación, health checks) y las despacha a Telegram a través de un único binario Go respaldado por SQLite. Los servicios envían mensajes mediante una API HTTP webhook o un CLI companion; jaimito los encola, persiste y entrega con reintentos automáticos. Ninguna notificación se pierde silenciosamente.

## Tecnologías

| Categoría | Tecnología |
|-----------|------------|
| Lenguaje | Go 1.24 |
| Base de datos | SQLite (WAL mode) via `modernc.org/sqlite` v1.46 (CGO-free) |
| HTTP Router | `go-chi/chi` v5 |
| CLI | `spf13/cobra` v1.10 |
| Telegram | `go-telegram/bot` v1.19 |
| Migraciones | `adlio/schema` |
| IDs | `google/uuid` v7 (time-ordered) |
| Configuración | YAML via `gopkg.in/yaml.v3` |
| TUI (Setup Wizard) | `charm.land/bubbletea` v2 |
| Despliegue | systemd, binario estático |

## Requisitos previos

- **Go 1.24** o superior
- Un **bot de Telegram** (token obtenido de [@BotFather](https://t.me/BotFather))
- El **chat ID** de Telegram donde el bot enviará mensajes
- **Linux con systemd** (para despliegue en producción)

## Instalación

### Instalación automática (recomendada)

```bash
curl -sL https://raw.githubusercontent.com/objetiva-comercios/objetiva-vps-jaimito/main/install.sh | bash
```

El instalador compila el binario, lo instala en `/usr/local/bin/jaimito`, y lanza el **setup wizard interactivo** que guía la configuración paso a paso: valida el bot token contra Telegram, verifica cada chat ID, genera una API key, y escribe el config YAML automáticamente.

### Instalación manual

```bash
# 1. Clonar el repositorio
git clone https://github.com/objetiva-comercios/objetiva-vps-jaimito.git
cd objetiva-vps-jaimito

# 2. Compilar el binario
go build -o jaimito ./cmd/jaimito

# 3. Instalar el binario
sudo cp jaimito /usr/local/bin/jaimito

# 4. Ejecutar el setup wizard
sudo jaimito setup

# O configurar manualmente:
sudo mkdir -p /etc/jaimito /var/lib/jaimito
sudo cp configs/config.example.yaml /etc/jaimito/config.yaml
sudo nano /etc/jaimito/config.yaml
```

## Configuración

El archivo de configuración vive en `/etc/jaimito/config.yaml` (override con `--config`).

```yaml
telegram:
  token: "TU_BOT_TOKEN_AQUI"

database:
  path: "/var/lib/jaimito/jaimito.db"

server:
  listen: "127.0.0.1:8080"

channels:
  - name: general
    chat_id: 123456789
    priority: normal
  - name: cron
    chat_id: 123456789
    priority: low
  - name: errors
    chat_id: 123456789
    priority: high

seed_api_keys:
  - name: "default"
    key: "sk-TU_CLAVE_AQUI"
```

| Campo | Requerido | Default | Descripción |
|-------|-----------|---------|-------------|
| `telegram.token` | Sí | — | Token del bot de Telegram |
| `database.path` | No | `/var/lib/jaimito/jaimito.db` | Ruta al archivo SQLite |
| `server.listen` | No | `127.0.0.1:8080` | Dirección y puerto del servidor HTTP |
| `channels` | Sí | — | Lista de canales (mínimo 1, debe incluir `general`) |
| `channels[].name` | Sí | — | Nombre único del canal |
| `channels[].chat_id` | Sí | — | ID del chat de Telegram (negativo para grupos) |
| `channels[].priority` | Sí | — | Prioridad por defecto: `low`, `normal`, `high` |
| `seed_api_keys` | No | — | Claves API pre-cargadas al iniciar (prefijo `sk-`) |

### Variables de entorno

Los subcomandos CLI (`send`, `wrap`) usan estas variables para conectarse al servidor:

```env
JAIMITO_API_KEY=sk-tu-clave-de-api-aqui
JAIMITO_SERVER=127.0.0.1:8080
```

| Variable | Propósito | Flag equivalente |
|----------|-----------|------------------|
| `JAIMITO_API_KEY` | Clave de autenticación para `send` y `wrap` | `--key` |
| `JAIMITO_SERVER` | Dirección del servidor | `--server` |

Prioridad de resolución: flag `--key`/`--server` > variable de entorno > config file > default.

## Uso

| Comando | Descripción |
|---------|-------------|
| `jaimito` | Inicia el servidor daemon |
| `jaimito setup` | Wizard interactivo de configuración (valida Telegram, genera API key, escribe config) |
| `jaimito --config /ruta/config.yaml` | Inicia con config personalizado |
| `jaimito send "mensaje"` | Envía notificación al canal `general` |
| `jaimito send -c cron -p high "mensaje"` | Envía con canal y prioridad específicos |
| `jaimito send -t "Título" "cuerpo"` | Envía con título (negrita en Telegram) |
| `jaimito send --tags backup,cron "mensaje"` | Envía con tags (hashtags en Telegram) |
| `jaimito send --stdin` | Lee el cuerpo del mensaje desde stdin |
| `jaimito wrap -- /path/to/script.sh` | Ejecuta un comando y notifica si falla |
| `jaimito wrap -c cron -- comando args` | Wrap con canal específico |
| `jaimito keys create --name mi-servicio` | Crea una nueva API key (prefijo `sk-`) |
| `jaimito keys list` | Lista las claves activas |
| `jaimito keys revoke <id>` | Revoca una clave por su UUID |
| `go build ./cmd/jaimito` | Compila el binario |
| `go test ./...` | Ejecuta todos los tests |
| `go vet ./...` | Análisis estático |

### Iniciar el servidor

```bash
# Foreground
./jaimito

# Con config personalizado
./jaimito --config /ruta/a/config.yaml
```

Secuencia de inicio:

1. Carga y valida la configuración YAML
2. Valida el token del bot de Telegram (`getMe`)
3. Valida cada `chat_id` configurado (`getChat`)
4. Abre la base de datos SQLite con WAL mode
5. Aplica migraciones de schema pendientes
6. Reclama mensajes en estado `dispatching` (crash recovery)
7. Inserta las `seed_api_keys` si no existen
8. Inicia el dispatcher de Telegram (polling cada 1s)
9. Inicia el scheduler de limpieza (cada 24h)
10. Levanta el servidor HTTP

### Despliegue con systemd

```bash
# Copiar binario y unit file
sudo cp jaimito /usr/local/bin/jaimito
sudo cp systemd/jaimito.service /etc/systemd/system/

# Habilitar e iniciar
sudo systemctl daemon-reload
sudo systemctl enable --now jaimito

# Verificar
systemctl status jaimito
curl -s http://127.0.0.1:8080/api/v1/health
```

### Enviar notificaciones

```bash
export JAIMITO_API_KEY=sk-tu-clave-aqui

# Mensaje simple
jaimito send "Backup completado"

# Con canal y prioridad
jaimito send -c cron -p high "Backup falló"

# Con título
jaimito send -t "Deploy" "v1.2.3 desplegado en producción"

# Desde stdin
df -h / | jaimito send --stdin -t "Disk Report" -c system
```

### Monitorear cron jobs

```bash
export JAIMITO_API_KEY=sk-tu-clave-aqui

# Si el comando falla, envía notificación con exit code y output capturado
jaimito wrap -- /path/to/backup.sh

# Con canal y prioridad
jaimito wrap -c cron -p high -- /usr/local/bin/certbot renew
```

Comportamiento de `wrap`:

- **Éxito (exit 0)**: sale silenciosamente, sin notificación
- **Fallo (exit != 0)**: envía notificación con nombre del comando, código de salida y output capturado (truncado a 3500 bytes), luego sale con el mismo código del comando original

### Gestionar claves API

```bash
# Crear una clave nueva (se imprime una sola vez)
jaimito keys create --name backup-service
# Output: sk-a1b2c3d4e5f6...

# Listar claves activas
jaimito keys list

# Revocar una clave (efecto inmediato, sin reiniciar)
jaimito keys revoke 550e8400-e29b-41d4-a716-446655440000
```

Las claves se almacenan como hash SHA-256 en la base de datos. La clave raw solo se muestra al momento de creación.

## Arquitectura del proyecto

```
├── cmd/
│   └── jaimito/
│       ├── main.go            # Entry point
│       ├── root.go            # Comando raíz, flags globales, resolvers
│       ├── serve.go           # Servidor daemon (secuencia de inicio)
│       ├── send.go            # Subcomando send
│       ├── wrap.go            # Subcomando wrap (monitoreo de cron)
│       ├── keys.go            # Subcomando keys (create/list/revoke)
│       └── setup/             # Setup wizard interactivo (bubbletea TUI)
│           ├── wizard.go      # Orquestador del wizard
│           ├── steps.go       # Interfaz de pasos
│           ├── bot_token_step.go     # Validación bot token
│           ├── general_channel_step.go  # Canal general
│           ├── extra_channels_step.go   # Canales adicionales
│           ├── server_step.go        # Dirección HTTP
│           ├── database_step.go      # Ruta base de datos
│           ├── apikey_step.go        # Generación API key
│           ├── summary_step.go       # Resumen y test notification
│           └── styles.go             # Estilos TUI
├── internal/
│   ├── api/
│   │   ├── handlers.go        # POST /api/v1/notify, GET /api/v1/health
│   │   ├── server.go          # Router chi con middleware stack
│   │   ├── middleware.go      # Autenticación Bearer token (SHA-256)
│   │   └── response.go        # Helpers de respuesta JSON
│   ├── config/
│   │   ├── config.go          # Carga YAML, validación, defaults
│   │   └── config_test.go     # Tests de configuración
│   ├── db/
│   │   ├── db.go              # Conexión SQLite, WAL mode, single-writer
│   │   ├── messages.go        # Cola: enqueue, status transitions, cleanup
│   │   ├── apikeys.go         # CRUD claves API (SHA-256, crypto/rand)
│   │   └── schema/
│   │       ├── 001_initial.sql        # Tablas: messages, dispatch_log, api_keys
│   │       └── 002_nullable_title.sql # Migración: title nullable
│   ├── dispatcher/
│   │   └── dispatcher.go      # Polling 1s, entrega a Telegram, 5 reintentos
│   ├── telegram/
│   │   ├── client.go          # Validación de bot y chats (getMe, getChat)
│   │   └── format.go          # MarkdownV2: emoji por prioridad, escaping
│   ├── client/
│   │   └── client.go          # Cliente HTTP para /api/v1/notify
│   └── cleanup/
│       └── scheduler.go       # Purga: 30d entregados, 90d fallidos
├── configs/
│   └── config.example.yaml    # Configuración de ejemplo
├── systemd/
│   └── jaimito.service        # Unit file para systemd
├── go.mod                     # Módulo Go
└── go.sum
```

### Flujo de datos

```
Servicio/Cron → CLI (send/wrap) → HTTP POST /api/v1/notify
                                        ↓
                              Bearer auth (SHA-256)
                                        ↓
                              SQLite (messages table)
                                        ↓
                              Dispatcher (polling 1s)
                                        ↓
                              Telegram Bot API → Chat
```

## API

| Método | Ruta | Auth | Descripción |
|--------|------|------|-------------|
| `GET` | `/api/v1/health` | No | Health check: `{"status": "ok", "service": "jaimito"}` |
| `POST` | `/api/v1/notify` | Bearer | Encola una notificación |

### POST /api/v1/notify

**Headers:**

```
Authorization: Bearer sk-<clave>
Content-Type: application/json
```

**Body:**

```json
{
  "body": "Backup falló",
  "channel": "cron",
  "priority": "high",
  "title": "Alerta de backup",
  "tags": ["backup", "cron"],
  "metadata": {"job_id": "12345"}
}
```

| Campo | Requerido | Default | Descripción |
|-------|-----------|---------|-------------|
| `body` | Sí | — | Contenido del mensaje |
| `channel` | No | `general` | Canal de notificación (debe existir en config) |
| `priority` | No | `normal` | Prioridad: `low`, `normal`, `high` |
| `title` | No | — | Título del mensaje (negrita en Telegram) |
| `tags` | No | — | Lista de etiquetas (se muestran como `#tag`) |
| `metadata` | No | — | Objeto JSON arbitrario (almacenado, no mostrado) |

**Respuestas:**

| Código | Significado |
|--------|-------------|
| `202` | Mensaje encolado: `{"id": "uuid-v7"}` |
| `400` | Payload inválido o canal inexistente |
| `401` | Token ausente, inválido o revocado |

**Ejemplo con curl:**

```bash
curl -X POST http://127.0.0.1:8080/api/v1/notify \
  -H "Authorization: Bearer sk-tu-clave-aqui" \
  -H "Content-Type: application/json" \
  -d '{"body": "Backup completado", "channel": "cron"}'
```

### Formato en Telegram

Los mensajes se formatean en MarkdownV2 con emoji según prioridad:

| Prioridad | Emoji | Ejemplo |
|-----------|-------|---------|
| `low` | 🟢 | 🟢 Mensaje de baja prioridad |
| `normal` | 🟡 | 🟡 **Título** Mensaje normal |
| `high` | 🔴 | 🔴 **Título** Mensaje urgente |

Los tags se agregan como hashtags al final del mensaje: `#backup #cron`.

## Scripts y automatización

### Monitoreo de cron jobs con wrap

`jaimito wrap` es la forma recomendada de monitorear cron jobs. Envolvé cualquier comando existente sin modificar su lógica:

```bash
# Ejemplo en crontab
0 2 * * * JAIMITO_API_KEY=sk-tu-clave jaimito wrap -c cron -- /usr/local/bin/backup.sh
0 3 * * 0 JAIMITO_API_KEY=sk-tu-clave jaimito wrap -c cron -- pg_dump -F c mydb -f /backups/mydb.dump
0 4 * * * JAIMITO_API_KEY=sk-tu-clave jaimito wrap -c cron -p high -- /usr/local/bin/certbot renew
```

### Limpieza automática de la base de datos

El servidor ejecuta un ciclo de limpieza cada 24 horas (primer ejecución al iniciar):

- Mensajes entregados con más de **30 días** se eliminan
- Mensajes fallidos con más de **90 días** se eliminan
- Registros de `dispatch_log` asociados se eliminan en la misma transacción

### Reintentos de entrega

El dispatcher revisa la cola cada 1 segundo y entrega los mensajes a Telegram:

- **Backoff exponencial**: 2s, 4s, 8s, 16s entre reintentos
- **Máximo 5 intentos** antes de marcar como `failed`
- **Rate limit (HTTP 429)**: respeta el `retry_after` exacto de Telegram
- **Crash recovery**: al reiniciar, mensajes en estado `dispatching` se reclaman a `queued`

### Notificaciones desde otros servicios

Cualquier servicio en el VPS puede enviar notificaciones via HTTP:

```bash
curl -s -X POST http://127.0.0.1:8080/api/v1/notify \
  -H "Authorization: Bearer $JAIMITO_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"body\": \"Deploy v$VERSION completado\", \"channel\": \"deploys\"}"
```

## Deploy

### Instalacion automatica

```bash
curl -sL https://raw.githubusercontent.com/objetiva-comercios/objetiva-vps-jaimito/main/install.sh | bash
```

El script `install.sh` automatiza todo el flujo: verifica dependencias (Go 1.24+, git, systemd), clona el repositorio, compila el binario, lo instala en `/usr/local/bin/jaimito`, lanza el **setup wizard** (`jaimito setup`) para configurar interactivamente, instala el servicio systemd, y ejecuta un health check.

Es idempotente: si detecta una instalacion previa, detiene el servicio, actualiza el repositorio, recompila, y ofrece reconfigurar con el wizard.

### Actualizacion

```bash
# Re-ejecutar el instalador (detecta instalacion previa)
curl -sL https://raw.githubusercontent.com/objetiva-comercios/objetiva-vps-jaimito/main/install.sh | bash
```

Ver [DEPLOY.md](DEPLOY.md) para documentacion completa de deploy, troubleshooting, y configuracion manual.

### Setup wizard

`jaimito setup` es un wizard interactivo TUI (bubbletea) que guía la configuración inicial:

1. **Bot Token** — Pide el token y lo valida contra la API de Telegram (`getMe`)
2. **Canal general** — Pide el chat ID y lo valida contra Telegram (`getChat`)
3. **Canales extra** — Permite agregar canales adicionales (deploys, errors, cron, etc.)
4. **Dirección HTTP** — Configura la dirección de escucha (default: `127.0.0.1:8080`)
5. **Base de datos** — Ruta al archivo SQLite (default: `/var/lib/jaimito/jaimito.db`)
6. **API Key** — Genera automáticamente una clave con prefijo `sk-`
7. **Resumen** — Muestra la configuración completa y ofrece enviar una notificación de test

El wizard escribe `/etc/jaimito/config.yaml` con permisos `0600`. Se invoca automáticamente durante la instalación con `install.sh`, o manualmente con `sudo jaimito setup`.

## Estado del proyecto

v1.1 completado — setup wizard interactivo con validación Telegram en vivo.
Funcionalidad completa: HTTP API, Telegram dispatch, CLI companion (`send`, `wrap`, `keys`, `setup`).
