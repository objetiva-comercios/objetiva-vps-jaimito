# jaimito

Hub de notificaciones push para VPS. Centraliza alertas de servicios, cron jobs, errores de aplicación y health checks en un único binario Go respaldado por SQLite, y las despacha a Telegram con reintentos automáticos. Los servicios envían mensajes a través de una API HTTP o un CLI companion; jaimito los encola, persiste y entrega sin que ninguna notificación se pierda silenciosamente. Pensado para un VPS personal con múltiples servicios y cron jobs que hoy fallan sin aviso.

## Tecnologías

| Categoría | Tecnología |
|-----------|------------|
| Lenguaje | Go 1.24 |
| Base de datos | SQLite (WAL mode) via `modernc.org/sqlite` |
| HTTP | `go-chi/chi` v5 |
| CLI | `spf13/cobra` |
| Telegram | `go-telegram/bot` v1 |
| Migraciones | `adlio/schema` |
| IDs | `google/uuid` v7 |
| Despliegue | systemd, binario estático |

## Requisitos previos

- Go 1.24 o superior
- Un bot de Telegram (token obtenido de [@BotFather](https://t.me/BotFather))
- El chat ID de Telegram donde el bot enviará mensajes
- Linux con systemd (para despliegue en producción)

## Instalación

```bash
# 1. Clonar el repositorio
git clone https://github.com/objetiva-comercios/objetiva-vps-jaimito.git
cd objetiva-vps-jaimito

# 2. Compilar el binario
go build -o jaimito ./cmd/jaimito

# 3. Crear directorios necesarios
sudo mkdir -p /etc/jaimito
sudo mkdir -p /var/lib/jaimito
sudo chown $USER:$USER /var/lib/jaimito

# 4. Copiar configuración de ejemplo
sudo cp configs/config.example.yaml /etc/jaimito/config.yaml

# 5. Editar la configuración con tus datos
sudo nano /etc/jaimito/config.yaml

# 6. Generar una API key
openssl rand -hex 32 | sed 's/^/sk-/'
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

### Campos de configuración

| Campo | Requerido | Default | Descripción |
|-------|-----------|---------|-------------|
| `telegram.token` | Sí | — | Token del bot de Telegram obtenido de @BotFather |
| `database.path` | No | `/var/lib/jaimito/jaimito.db` | Ruta al archivo SQLite |
| `server.listen` | No | `127.0.0.1:8080` | Dirección y puerto del servidor HTTP |
| `channels` | Sí | — | Lista de canales de notificación (mínimo 1, debe incluir `general`) |
| `channels[].name` | Sí | — | Nombre único del canal |
| `channels[].chat_id` | Sí | — | ID del chat de Telegram (negativo para grupos) |
| `channels[].priority` | Sí | — | Prioridad por defecto: `low`, `normal`, `high` |
| `seed_api_keys` | No | — | Claves API pre-cargadas al iniciar (deben empezar con `sk-`) |

### Variables de entorno

Los subcomandos CLI (`send`, `wrap`) usan estas variables de entorno para conectarse al servidor:

```env
JAIMITO_API_KEY=sk-tu-clave-de-api-aqui
JAIMITO_SERVER=127.0.0.1:8080
```

| Variable | Propósito | Flag equivalente |
|----------|-----------|------------------|
| `JAIMITO_API_KEY` | Clave de autenticación para `send` y `wrap` | `--key` |
| `JAIMITO_SERVER` | Dirección del servidor | `--server` |

La prioridad de resolución es: flag `--key`/`--server` > variable de entorno > config file > default.

### Obtener el chat ID de Telegram

1. Creá un bot con [@BotFather](https://t.me/BotFather) y copiá el token
2. Abrí el chat con tu bot y enviá un mensaje (ej: "hola")
3. Ejecutá:

```bash
curl -s "https://api.telegram.org/botTU_TOKEN/getUpdates" | python3 -m json.tool
```

4. Buscá el campo `"chat": {"id": NUMERO}` — ese es tu `chat_id`

Para grupos, el `chat_id` es negativo (ej: `-100123456789`). Todos los canales pueden apuntar al mismo `chat_id` o a distintos grupos.

## Uso

| Comando | Descripción |
|---------|-------------|
| `jaimito` | Inicia el servidor daemon (default) |
| `jaimito --config /ruta/config.yaml` | Inicia el servidor con config personalizado |
| `jaimito send "mensaje"` | Envía una notificación al canal `general` |
| `jaimito send -c cron -p high "mensaje"` | Envía con canal y prioridad específicos |
| `jaimito send -t "Título" "cuerpo"` | Envía con título |
| `jaimito send --tags backup,cron "mensaje"` | Envía con tags |
| `jaimito send --stdin` | Lee el cuerpo del mensaje desde stdin |
| `jaimito wrap -- /path/to/script.sh` | Ejecuta un comando y notifica si falla |
| `jaimito wrap -c cron -- comando args` | Wrap con canal específico |
| `jaimito wrap -p high -- comando args` | Wrap con prioridad específica |
| `jaimito keys create --name mi-servicio` | Crea una nueva API key (`sk-` prefijo) |
| `jaimito keys list` | Lista las claves activas con ID, nombre y fecha |
| `jaimito keys revoke <id>` | Revoca una clave por su UUID |
| `go build ./cmd/jaimito` | Compila el binario |
| `go test ./...` | Ejecuta todos los tests |
| `go vet ./...` | Análisis estático |

### Iniciar el servidor

```bash
# Directamente (foreground)
./jaimito

# Con config personalizado
./jaimito --config /ruta/a/config.yaml
```

Al iniciar, el servidor ejecuta la siguiente secuencia:
1. Carga y valida la configuración YAML
2. Valida el token del bot de Telegram (llamada a `getMe`)
3. Valida cada `chat_id` configurado (llamada a `getChat`)
4. Abre la base de datos SQLite con WAL mode
5. Aplica migraciones de schema pendientes
6. Reclama mensajes en estado `dispatching` (crash recovery)
7. Inserta las `seed_api_keys` si no existen
8. Inicia el dispatcher de Telegram (polling cada 1s)
9. Inicia el scheduler de limpieza (cada 24h)
10. Levanta el servidor HTTP en `server.listen`

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

### Gestionar claves API

```bash
# Crear una clave nueva (se imprime una sola vez)
jaimito keys create --name backup-service
# Output: sk-a1b2c3d4e5f6...

# Listar claves activas
jaimito keys list
# ID                                    NAME              CREATED              LAST USED
# 550e8400-e29b-41d4-a716-446655440000  backup-service    2026-02-24 15:00:00  2026-02-24 15:30:00

# Revocar una clave (efecto inmediato, sin reiniciar)
jaimito keys revoke 550e8400-e29b-41d4-a716-446655440000
```

Las claves se almacenan como hash SHA-256 en la base de datos. La clave raw solo se muestra al momento de creación.

### Enviar notificaciones

```bash
export JAIMITO_API_KEY=sk-tu-clave-aqui

# Mensaje simple
jaimito send "Backup completado"

# Con canal y prioridad
jaimito send -c cron -p high "Backup falló"

# Con título
jaimito send -t "Deploy" "v1.2.3 desplegado en producción"

# Con tags
jaimito send --tags deploy,produccion "Deploy exitoso"

# Desde stdin (útil para pipear output de otros comandos)
echo "uso de disco: 90%" | jaimito send --stdin -c monitoring
df -h / | jaimito send --stdin -t "Disk Report" -c system
```

### Monitorear cron jobs

```bash
export JAIMITO_API_KEY=sk-tu-clave-aqui

# Si el comando falla, envía notificación con código de salida y output capturado
jaimito wrap -- /path/to/backup.sh

# Con canal específico
jaimito wrap -c cron -- pg_dump -F c mydb -f /backups/mydb.dump

# Con prioridad alta
jaimito wrap -c cron -p high -- /usr/local/bin/certbot renew
```

Comportamiento de `wrap`:
- **Éxito (exit 0)**: sale silenciosamente, sin notificación
- **Fallo (exit != 0)**: envía notificación con nombre del comando, código de salida y salida capturada (truncada a 3500 bytes), luego sale con el mismo código del comando original
- La notificación es best-effort: si falla el envío, preserva el exit code del comando original

## Arquitectura del proyecto

```
├── cmd/
│   └── jaimito/
│       ├── main.go            # Entry point
│       ├── root.go            # Comando raíz, flags globales, resolvers
│       ├── serve.go           # Servidor daemon (secuencia de 10 pasos)
│       ├── send.go            # Subcomando send
│       ├── wrap.go            # Subcomando wrap (monitoreo de cron)
│       └── keys.go            # Subcomando keys (create/list/revoke)
├── internal/
│   ├── api/
│   │   ├── handlers.go        # POST /api/v1/notify, GET /api/v1/health
│   │   ├── server.go          # Router chi con middleware stack
│   │   ├── middleware.go      # Autenticación Bearer token (SHA-256)
│   │   └── response.go        # Helpers de respuesta JSON
│   ├── config/
│   │   ├── config.go          # Carga YAML, validación, defaults
│   │   └── config_test.go     # Tests de validación
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
│       └── scheduler.go       # Purga: 30d entregados, 90d fallidos, cada 24h
├── configs/
│   └── config.example.yaml    # Configuración de ejemplo completa
├── systemd/
│   └── jaimito.service        # Unit file para systemd
├── go.mod                     # Módulo: github.com/chiguire/jaimito
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

### Schema de base de datos

| Tabla | Propósito |
|-------|-----------|
| `messages` | Cola de mensajes: id (UUID v7), channel, priority, title, body, tags, metadata, status, timestamps |
| `dispatch_log` | Historial de intentos de entrega: attempt number, status, error, timestamp |
| `api_keys` | Claves de autenticación: key_hash (SHA-256), name, created_at, last_used_at, revoked |

Estados del mensaje: `queued` → `dispatching` → `delivered` / `failed`

## API

| Método | Ruta | Auth | Descripción |
|--------|------|------|-------------|
| `GET` | `/api/v1/health` | No | Health check. Retorna `{"status": "ok", "service": "jaimito"}` |
| `POST` | `/api/v1/notify` | Bearer | Envía una notificación a la cola |

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
| `tags` | No | — | Lista de etiquetas (se muestran como `#tag` en Telegram) |
| `metadata` | No | — | Objeto JSON arbitrario (almacenado, no mostrado) |

**Respuestas:**

| Código | Body | Significado |
|--------|------|-------------|
| 202 | `{"id": "019c9039-c807-7e68-975a-5d9af09374f2"}` | Mensaje encolado exitosamente |
| 400 | `{"error": "body is required"}` | Payload inválido o canal inexistente |
| 401 | `{"error": "unauthorized"}` | Token ausente, inválido o revocado |

**Ejemplo con curl:**

```bash
curl -X POST http://127.0.0.1:8080/api/v1/notify \
  -H "Authorization: Bearer sk-tu-clave-aqui" \
  -H "Content-Type: application/json" \
  -d '{"body": "Backup completado", "channel": "cron", "priority": "normal"}'
```

### Formato en Telegram

Los mensajes se formatean en MarkdownV2 con emoji según prioridad:

| Prioridad | Emoji | Ejemplo |
|-----------|-------|---------|
| `low` | 🟢 | 🟢 Mensaje bajo |
| `normal` | 🟡 | 🟡 **Título** Mensaje normal |
| `high` | 🔴 | 🔴 **Título** Mensaje urgente |

Los tags se agregan como hashtags al final: `#backup #cron`.

## Scripts y automatización

### Monitoreo de cron jobs

`jaimito wrap` es la forma recomendada de monitorear cron jobs. Envolvé cualquier comando existente sin modificar su lógica:

```bash
# Ejemplo en crontab
0 2 * * * JAIMITO_API_KEY=sk-tu-clave jaimito wrap -c cron -- /usr/local/bin/backup.sh
0 3 * * 0 JAIMITO_API_KEY=sk-tu-clave jaimito wrap -c cron -- pg_dump -F c mydb -f /backups/mydb.dump
0 4 * * * JAIMITO_API_KEY=sk-tu-clave jaimito wrap -c cron -p high -- /usr/local/bin/certbot renew
```

### Limpieza automática de la base de datos

El servidor ejecuta un ciclo de limpieza cada 24 horas (primer ejecución al iniciar, luego cada 24h):

- Mensajes entregados con más de 30 días → eliminados
- Mensajes fallidos con más de 90 días → eliminados
- Registros de `dispatch_log` asociados → eliminados en la misma transacción

### Reintentos de entrega

El dispatcher revisa la cola cada 1 segundo y entrega los mensajes a Telegram:

- Backoff exponencial: 2s, 4s, 8s, 16s entre reintentos
- Máximo 5 intentos antes de marcar como `failed`
- Rate limit (HTTP 429): respeta el `retry_after` exacto de Telegram
- Crash recovery: al reiniciar, mensajes en estado `dispatching` se reclaman a `queued`

### Notificaciones desde otros servicios

Cualquier servicio en el VPS puede enviar notificaciones via HTTP:

```bash
# Desde un script de deploy
curl -s -X POST http://127.0.0.1:8080/api/v1/notify \
  -H "Authorization: Bearer $JAIMITO_API_KEY" \
  -H "Content-Type: application/json" \
  -d "{\"body\": \"Deploy v$VERSION completado\", \"channel\": \"deploys\"}"
```
