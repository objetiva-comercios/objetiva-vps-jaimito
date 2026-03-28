# Deploy — jaimito

## Instalacion rapida

```bash
curl -sL https://raw.githubusercontent.com/objetiva-comercios/objetiva-vps-jaimito/main/install.sh | bash
```

## Requisitos

- **Go 1.25+** — compilar el binario
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

      ┌─────────────────────────────────────┐
      │  Metricas (v2.0)                    │
      │                                     │
      │  collector → SQLite → API → Dashboard│
      │  config.yaml metrics.definitions    │
      │  GET /api/v1/metrics                │
      │  GET /api/v1/metrics/{name}/readings│
      │  GET /dashboard (web embedido)      │
      └─────────────────────────────────────┘
```

**Componentes:**
- **Binario unico**: `/usr/local/bin/jaimito` (servidor + CLI)
- **Config**: `/etc/jaimito/config.yaml`
- **Base de datos**: `/var/lib/jaimito/jaimito.db` (SQLite WAL-mode)
- **Servicio**: `jaimito.service` (systemd)
- **Dashboard**: `GET /dashboard` — interfaz web embedida (go:embed), zero dependencias externas

## Configuracion inicial

### Setup wizard (recomendado)

```bash
sudo jaimito setup
```

El wizard interactivo guia la configuracion paso a paso:

1. Pide el **bot token** de Telegram y lo valida contra la API (`getMe`)
2. Pide el **chat ID** del canal general y lo valida (`getChat`)
3. Permite agregar **canales extra** (deploys, errors, cron, etc.) con validacion
4. Configura la **direccion HTTP** (default: `127.0.0.1:8080`)
5. Configura la **ruta de base de datos** (default: `/var/lib/jaimito/jaimito.db`)
6. Genera una **API key** automaticamente (prefijo `sk-`)
7. Muestra un **resumen** y ofrece enviar notificacion de test

Escribe `/etc/jaimito/config.yaml` con permisos `0600`. Se ejecuta automaticamente durante `install.sh` si no existe config previa. En reinstalaciones, el instalador pregunta si queres reconfigurar.

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

### Configuracion de metricas (v2.0)

La seccion `metrics` en `config.yaml` define las metricas a recolectar:

```yaml
metrics:
  retention: "7d"            # Retencion de readings (s, m, h, d)
  alert_cooldown: "30m"      # Tiempo minimo entre alertas por metrica
  collect_interval: "60s"    # Intervalo default de recoleccion
  definitions:
    - name: disk_root
      command: "df / | awk 'NR==2 {print $5}' | tr -d '%'"
      interval: "300s"
      category: system
      type: gauge
      thresholds:
        warning: 80
        critical: 90
    - name: ram_used
      command: "free | awk '/^Mem:/ {printf \"%.0f\", $3/$2*100}'"
      interval: "60s"
      category: system
      type: gauge
      thresholds:
        warning: 80
        critical: 95
    - name: cpu_load
      command: "uptime | awk -F'load average:' '{print $2}' | awk -F',' '{print $1}' | tr -d ' '"
      interval: "60s"
      category: system
      type: gauge
    - name: docker_running
      command: "docker ps -q | wc -l | tr -d ' '"
      interval: "120s"
      category: docker
      type: gauge
    - name: uptime_days
      command: "awk '{printf \"%.1f\", $1/86400}' /proc/uptime"
      interval: "3600s"
      category: system
      type: counter
```

Las metricas se recolectan ejecutando los comandos shell en los intervalos configurados. Cuando una metrica cruza un umbral (warning/critical), se envia una alerta a Telegram automaticamente.

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

**Metodo rapido (recomendado):**
1. Enviar un mensaje en el grupo de Telegram
2. Reenviar ese mensaje a [@RawDataBot](https://t.me/RawDataBot)
3. El bot responde con el JSON del mensaje — buscar `"chat": {"id": -100XXXXXXXXX}`

**Nota:** el chat ID de grupos es negativo (empieza con `-100`). Un ID positivo como `42795671` es un chat privado, no un grupo.

**Metodo alternativo:**
1. Agregar el bot al grupo
2. Enviar un mensaje al grupo
3. Visitar `https://api.telegram.org/bot<TOKEN>/getUpdates`
4. Buscar el campo `chat.id` en la respuesta

## Servicios

| Servicio | Puerto | Protocolo | Descripcion |
|----------|--------|-----------|-------------|
| jaimito (HTTP API) | 8080 | HTTP | API de notificaciones y metricas (localhost only) |
| jaimito (Dashboard) | 8080 | HTTP | Dashboard web embedido en `/dashboard` |

### Endpoints

| Metodo | Ruta | Auth | Descripcion |
|--------|------|------|-------------|
| `POST` | `/api/v1/notify` | Bearer `sk-*` | Enviar notificacion |
| `GET` | `/api/v1/health` | No | Health check |
| `GET` | `/api/v1/metrics` | No | Listar metricas con ultimo valor y estado |
| `GET` | `/api/v1/metrics/{name}/readings` | No | Historial de readings de una metrica |
| `POST` | `/api/v1/metrics` | Bearer `sk-*` | Ingestar metrica manualmente |
| `GET` | `/dashboard` | No | Dashboard web embedido |

## Dashboard web (v2.0)

El dashboard es una interfaz web autocontenida servida desde el binario via `go:embed`. No requiere conexion a internet — Alpine.js, uPlot y Tailwind CSS estan embebidos inline.

**Acceso:** `http://127.0.0.1:8080/dashboard`

**Funcionalidades:**
- Tabla de metricas con nombre, valor, sparkline SVG e indicador de estado (verde/amarillo/rojo)
- Click en una fila expande un grafico temporal con historial de 24h (uPlot)
- Lineas de umbral warning/critical en los graficos
- Auto-refresh cada 30 segundos sin recargar la pagina
- Header con hostname del VPS y timestamp de ultima actualizacion
- Estilo terminal dark (monospace, colores slate/blue)

**Sin autenticacion:** el dashboard solo escucha en localhost (127.0.0.1). Para acceso remoto, usar Tailscale o SSH tunnel.

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

# Ver metricas actuales (v2.0)
jaimito status

# Ingestar metrica manualmente (v2.0)
jaimito metric disk_root 42.5
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
- `chat_id` incorrecto o generico (100000001) → reconfigurar con `sudo jaimito setup` usando IDs reales obtenidos via @RawDataBot
- El bot no tiene acceso al chat → agregar el bot al grupo primero
- Puerto 8080 ya en uso → cambiar `server.listen` en config

### "connection refused" al usar `jaimito send`

El servicio no esta corriendo. Verificar:
```bash
sudo systemctl status jaimito
sudo journalctl -u jaimito --no-pager -n 20
```

Si los logs muestran `chat_id unreachable`, los chat IDs en la config no son validos. Reconfigurar con `sudo jaimito setup`.

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

### Dashboard no muestra metricas

1. Verificar que la seccion `metrics` esta habilitada en `config.yaml` (descomentarla)
2. Verificar que el collector esta corriendo: `sudo journalctl -u jaimito -f | grep metric`
3. Verificar la API: `curl -s http://127.0.0.1:8080/api/v1/metrics | jq`
4. Si las metricas aparecen en la API pero no en el dashboard, esperar 30s (auto-refresh)

## Estructura del proyecto

```
objetiva-vps-jaimito/
├── cmd/jaimito/          # Entrypoint y CLI (main, root, serve, send, wrap, keys, setup, status, metric)
│   └── setup/            # Setup wizard interactivo (bubbletea TUI)
├── configs/
│   └── config.example.yaml
├── internal/
│   ├── api/              # HTTP router, middleware, handlers (notify, health, metrics, dashboard)
│   ├── cleanup/          # Scheduler de purga de mensajes viejos
│   ├── client/           # HTTP client para CLI send/wrap
│   ├── collector/        # Recolector de metricas (ejecuta comandos shell por intervalos)
│   ├── config/           # Parser de config YAML
│   ├── db/               # SQLite: schema, mensajes, API keys, metricas
│   ├── dispatcher/       # Polling loop → Telegram con retry
│   ├── telegram/         # Bot client y formatter MarkdownV2
│   └── web/              # Dashboard web embedido (go:embed, Alpine.js, uPlot, Tailwind)
├── scripts/
│   └── build-dashboard.sh  # Compilar Tailwind CSS para el dashboard
├── systemd/
│   └── jaimito.service
├── install.sh
├── go.mod
└── go.sum
```
