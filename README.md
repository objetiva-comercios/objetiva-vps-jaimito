# jaimito

Hub de notificaciones push para VPS. Centraliza alertas de servicios, cron jobs, errores de aplicación y health checks en un único binario Go respaldado por SQLite, y las despacha a Telegram con reintentos automáticos. Los servicios envían mensajes a través de una API HTTP o un CLI companion; jaimito los encola, persiste y entrega sin que ninguna notificación se pierda silenciosamente.

## Tecnologías

| Categoría | Tecnología |
|-----------|------------|
| Lenguaje | Go 1.24 |
| Base de datos | SQLite (WAL mode) via `modernc.org/sqlite` |
| HTTP | `go-chi/chi` v5 |
| CLI | `spf13/cobra` |
| Telegram | `go-telegram/bot` v1 |
| Migraciones | `adlio/schema` |
| Despliegue | systemd, binario estático |

## Requisitos previos

- Go 1.24 o superior
- Un bot de Telegram (token obtenido de [@BotFather](https://t.me/BotFather))
- El chat ID de Telegram donde el bot enviará mensajes
- Linux con systemd (para despliegue en producción)

## Instalación

```bash
# 1. Clonar el repositorio
git clone https://github.com/chiguire/jaimito.git
cd jaimito

# 2. Compilar el binario
go build -o jaimito ./cmd/jaimito

# 3. Crear directorios necesarios
sudo mkdir -p /etc/jaimito
sudo mkdir -p /var/lib/jaimito

# 4. Copiar configuración de ejemplo
sudo cp configs/config.example.yaml /etc/jaimito/config.yaml

# 5. Editar la configuración con tus datos
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

**Notas:**
- `telegram.token` y al menos un canal `general` son obligatorios
- `priority` acepta: `low`, `normal`, `high`
- Las claves en `seed_api_keys` deben empezar con `sk-`. Generá una con: `openssl rand -hex 32 | sed 's/^/sk-/'`
- `database.path` y `server.listen` tienen defaults y pueden omitirse

**Variables de entorno** (para comandos CLI):

| Variable | Propósito |
|----------|-----------|
| `JAIMITO_API_KEY` | Clave de autenticación para `send` y `wrap` |
| `JAIMITO_SERVER` | Dirección del servidor (default: `127.0.0.1:8080`) |

## Uso

### Iniciar el servidor

```bash
# Directamente
./jaimito

# Con config personalizado
./jaimito --config /ruta/a/config.yaml

# Como servicio systemd
sudo cp systemd/jaimito.service /etc/systemd/system/
sudo systemctl enable --now jaimito
```

### Gestionar claves API

```bash
# Crear una clave nueva
jaimito keys create --name mi-servicio
# Output: sk-a1b2c3d4e5f6...

# Listar claves activas
jaimito keys list

# Revocar una clave
jaimito keys revoke <id>
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
echo "uso de disco: 90%" | jaimito send --stdin -c monitoring
```

### Monitorear cron jobs

```bash
export JAIMITO_API_KEY=sk-tu-clave-aqui

# Si el comando falla, envía notificación con código de salida y output
jaimito wrap -- /path/to/backup.sh

# Con canal específico
jaimito wrap -c cron -- pg_dump -F c mydb -f /backups/mydb.dump
```

En caso de éxito el comando sale silenciosamente. Si falla, envía una notificación a Telegram con el nombre del comando, código de salida y la salida capturada, y sale con el mismo código del comando original.

### Compilación

| Comando | Descripción |
|---------|-------------|
| `go build ./cmd/jaimito` | Compilar el binario |
| `go test ./...` | Ejecutar todos los tests |
| `go vet ./...` | Análisis estático |

## Arquitectura del proyecto

```
├── cmd/
│   └── jaimito/
│       ├── main.go            # Entry point
│       ├── root.go            # Comando raíz, flags globales
│       ├── serve.go           # Servidor daemon (default)
│       ├── send.go            # Subcomando send
│       ├── wrap.go            # Subcomando wrap
│       └── keys.go            # Subcomando keys (create/list/revoke)
├── internal/
│   ├── api/
│   │   ├── handlers.go        # Endpoints: /notify, /health
│   │   ├── server.go          # Router chi con middleware
│   │   ├── middleware.go      # Autenticación Bearer token
│   │   └── response.go        # Helpers de respuesta JSON
│   ├── config/
│   │   └── config.go          # Carga YAML, validación, defaults
│   ├── db/
│   │   ├── db.go              # Conexión SQLite, WAL mode
│   │   ├── messages.go        # Cola de mensajes, transiciones de estado
│   │   ├── apikeys.go         # CRUD de claves API (SHA-256)
│   │   └── schema/            # Migraciones SQL
│   ├── dispatcher/
│   │   └── dispatcher.go      # Polling 1s, entrega a Telegram, reintentos
│   ├── telegram/
│   │   ├── client.go          # Validación de bot y chats
│   │   └── format.go          # Formato MarkdownV2 con emoji por prioridad
│   ├── client/
│   │   └── client.go          # Cliente HTTP para la API
│   └── cleanup/
│       └── scheduler.go       # Purga automática: 30d entregados, 90d fallidos
├── configs/
│   └── config.example.yaml    # Configuración de ejemplo
├── systemd/
│   └── jaimito.service        # Unit file para systemd
├── go.mod
└── go.sum
```

## API

### Health check

```
GET /api/v1/health
```

Respuesta `200 OK`:
```json
{"status": "ok", "service": "jaimito"}
```

### Enviar notificación

```
POST /api/v1/notify
Authorization: Bearer sk-<clave>
Content-Type: application/json
```

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

Solo `body` es obligatorio. `channel` default `general`, `priority` default `normal`.

| Código | Significado |
|--------|-------------|
| 202 | Mensaje encolado, retorna `{"id": "uuid"}` |
| 400 | Payload inválido o canal inexistente |
| 401 | Token ausente o inválido |

## Ciclo de vida del mensaje

```
queued → dispatching → delivered
                     → failed (después de 5 reintentos)
```

- **Polling**: el dispatcher revisa la cola cada 1 segundo
- **Reintentos**: backoff exponencial (2s, 4s, 8s, 16s), máximo 5 intentos
- **Rate limit (429)**: respeta el `retry_after` exacto de Telegram
- **Crash recovery**: mensajes en `dispatching` se reclaman a `queued` al reiniciar
- **Limpieza automática**: cada 24h elimina entregados >30 días y fallidos >90 días
