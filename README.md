# jaimito

Hub de notificaciones push y monitoreo para VPS. Centraliza todas las alertas generadas en un VPS (eventos de servicios, resultados de cron jobs, errores de aplicacion, health checks) y las despacha a Telegram a traves de un unico binario Go respaldado por SQLite. Los servicios envian mensajes mediante una API HTTP webhook o un CLI companion; jaimito los encola, persiste y entrega con reintentos automaticos. Ninguna notificacion se pierde silenciosamente.

En v2.0, jaimito tambien recolecta metricas del sistema (disco, RAM, CPU, Docker, custom), las almacena en SQLite, envia alertas a Telegram cuando se cruzan umbrales, y las muestra en un dashboard web embedido — monitoreo liviano sin instalar Prometheus/Grafana.

## Tecnologias

| Categoria | Tecnologia |
|-----------|------------|
| Lenguaje | Go 1.25 |
| Base de datos | SQLite (WAL mode) via `modernc.org/sqlite` v1.46 (CGO-free) |
| HTTP Router | `go-chi/chi` v5 |
| CLI | `spf13/cobra` v1.10 |
| Telegram | `go-telegram/bot` v1.19 |
| Migraciones | `adlio/schema` |
| IDs | `google/uuid` v7 (time-ordered) |
| Configuracion | YAML via `gopkg.in/yaml.v3` |
| TUI (Setup Wizard) | `charm.land/bubbletea` v2 |
| Dashboard Frontend | Alpine.js 3.15, uPlot 1.6.32, Tailwind CSS (pre-compilado) |
| Dashboard Icons | Lucide (SVG inline) |
| Despliegue | systemd, binario estatico |

## Requisitos previos

- **Go 1.25** o superior
- Un **bot de Telegram** (token obtenido de [@BotFather](https://t.me/BotFather))
- El **chat ID** de Telegram donde el bot enviara mensajes (obtener reenviando un mensaje del grupo a [@RawDataBot](https://t.me/RawDataBot))
- **Linux con systemd** (para despliegue en produccion)

## Instalacion

### Instalacion automatica (recomendada)

```bash
curl -sL https://raw.githubusercontent.com/objetiva-comercios/objetiva-vps-jaimito/main/install.sh | bash
```

El instalador compila el binario, lo instala en `/usr/local/bin/jaimito`, y lanza el **setup wizard interactivo** que guia la configuracion paso a paso: valida el bot token contra Telegram, verifica cada chat ID, genera una API key, y escribe el config YAML automaticamente.

### Instalacion manual

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

## Configuracion

El archivo de configuracion vive en `/etc/jaimito/config.yaml` (override con `--config`).

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

# Metricas (v2.0) — descomentar para habilitar monitoreo
# metrics:
#   retention: "7d"
#   alert_cooldown: "30m"
#   collect_interval: "60s"
#   definitions:
#     - name: disk_root
#       command: "df / | awk 'NR==2 {print $5}' | tr -d '%'"
#       interval: "300s"
#       category: system
#       type: gauge
#       thresholds:
#         warning: 80
#         critical: 90
```

| Campo | Requerido | Default | Descripcion |
|-------|-----------|---------|-------------|
| `telegram.token` | Si | — | Token del bot de Telegram |
| `database.path` | No | `/var/lib/jaimito/jaimito.db` | Ruta al archivo SQLite |
| `server.listen` | No | `127.0.0.1:8080` | Direccion y puerto del servidor HTTP |
| `channels` | Si | — | Lista de canales (minimo 1, debe incluir `general`) |
| `seed_api_keys` | No | — | Claves API pre-cargadas al iniciar (prefijo `sk-`) |
| `metrics.retention` | No | `7d` | Retencion de readings |
| `metrics.alert_cooldown` | No | `30m` | Tiempo minimo entre alertas por metrica |
| `metrics.collect_interval` | No | `60s` | Intervalo default de recoleccion |
| `metrics.definitions` | No | — | Lista de metricas a recolectar |

### Variables de entorno

Los subcomandos CLI (`send`, `wrap`) usan estas variables para conectarse al servidor:

| Variable | Proposito | Flag equivalente |
|----------|-----------|------------------|
| `JAIMITO_API_KEY` | Clave de autenticacion para `send` y `wrap` | `--key` |
| `JAIMITO_SERVER` | Direccion del servidor | `--server` |

Prioridad de resolucion: flag `--key`/`--server` > variable de entorno > config file > default.

## Uso

### Comandos

| Comando | Descripcion |
|---------|-------------|
| `jaimito` | Inicia el servidor daemon |
| `jaimito setup` | Wizard interactivo de configuracion |
| `jaimito send` | Envia una notificacion a Telegram |
| `jaimito wrap` | Ejecuta un comando y notifica si falla |
| `jaimito keys` | Gestionar API keys (create/list/revoke) |
| `jaimito status` | Ver metricas actuales del sistema (v2.0) |
| `jaimito metric` | Ingestar una metrica manualmente (v2.0) |

### Enviar notificaciones

```bash
export JAIMITO_API_KEY=sk-tu-clave-aqui

# Mensaje simple
jaimito send "Backup completado"

# Con canal y prioridad
jaimito send -c cron -p high "Backup fallo"

# Con titulo
jaimito send -t "Deploy" "v1.2.3 desplegado en produccion"

# Desde stdin — ideal para pipes y scripts
df -h / | jaimito send --stdin -t "Disco" -c monitoring
tail -20 /var/log/syslog | jaimito send --stdin -c system
docker ps --format "table {{.Names}}\t{{.Status}}" | jaimito send --stdin -t "Containers"
```

### Monitorear cron jobs

```bash
# Si el comando falla, envia notificacion con exit code y output capturado
jaimito wrap -- /path/to/backup.sh

# Con canal y prioridad
jaimito wrap -c cron -p high -- certbot renew
```

Comportamiento de `wrap`:
- **Exito (exit 0)**: sale silenciosamente, sin notificacion
- **Fallo (exit != 0)**: envia notificacion con nombre del comando, codigo de salida y output capturado (truncado a 3500 bytes)

### Metricas y dashboard (v2.0)

```bash
# Ver metricas actuales en terminal
jaimito status

# Ingestar metrica manualmente
jaimito metric disk_root 42.5

# Dashboard web
# Abrir http://127.0.0.1:8080/dashboard en el browser
```

El dashboard web es una interfaz autocontenida embedida en el binario (go:embed). Muestra una tabla de metricas con sparklines, graficos expandibles (uPlot), indicadores de estado coloreados, y auto-refresh cada 30 segundos. Funciona sin conexion a internet — Alpine.js, uPlot y Tailwind CSS estan inline.

### Gestionar claves API

```bash
# Crear una clave nueva (se imprime una sola vez)
jaimito keys create --name backup-service

# Listar claves activas
jaimito keys list

# Revocar una clave (efecto inmediato, sin reiniciar)
jaimito keys revoke 550e8400-e29b-41d4-a716-446655440000
```

### Despliegue con systemd

```bash
sudo cp jaimito /usr/local/bin/jaimito
sudo cp systemd/jaimito.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now jaimito
systemctl status jaimito
```

## API

| Metodo | Ruta | Auth | Descripcion |
|--------|------|------|-------------|
| `GET` | `/api/v1/health` | No | Health check |
| `POST` | `/api/v1/notify` | Bearer `sk-*` | Encolar notificacion |
| `GET` | `/api/v1/metrics` | No | Listar metricas con ultimo valor y estado |
| `GET` | `/api/v1/metrics/{name}/readings?since=24h` | No | Historial de readings |
| `POST` | `/api/v1/metrics` | Bearer `sk-*` | Ingestar metrica manual |
| `GET` | `/dashboard` | No | Dashboard web embedido |

### POST /api/v1/notify

```bash
curl -X POST http://127.0.0.1:8080/api/v1/notify \
  -H "Authorization: Bearer sk-tu-clave-aqui" \
  -H "Content-Type: application/json" \
  -d '{"body": "Backup completado", "channel": "cron"}'
```

| Campo | Requerido | Default | Descripcion |
|-------|-----------|---------|-------------|
| `body` | Si | — | Contenido del mensaje |
| `channel` | No | `general` | Canal de notificacion |
| `priority` | No | `normal` | Prioridad: `low`, `normal`, `high` |
| `title` | No | — | Titulo del mensaje (negrita en Telegram) |
| `tags` | No | — | Lista de etiquetas (se muestran como `#tag`) |

| Codigo | Significado |
|--------|-------------|
| `202` | Mensaje encolado |
| `400` | Payload invalido o canal inexistente |
| `401` | Token ausente, invalido o revocado |

### GET /api/v1/metrics

Retorna array de metricas con ultimo valor, estado y umbrales:

```json
[
  {
    "name": "disk_root",
    "category": "system",
    "type": "gauge",
    "last_value": 42.5,
    "last_status": "ok",
    "updated_at": "2026-03-28T12:00:00Z",
    "thresholds": { "warning": 80.0, "critical": 95.0 }
  }
]
```

### GET /api/v1/metrics/{name}/readings?since=24h

Retorna historial de lecturas de una metrica:

```json
{
  "metric": "disk_root",
  "since": "24h",
  "readings": [
    { "value": 42.5, "recorded_at": "2026-03-28T12:00:00Z" }
  ]
}
```

## Arquitectura del proyecto

```
├── cmd/jaimito/          # Entrypoint y CLI (main, root, serve, send, wrap, keys, setup, status, metric)
│   └── setup/            # Setup wizard interactivo (bubbletea TUI)
├── configs/
│   └── config.example.yaml
├── internal/
│   ├── api/              # HTTP router, middleware, handlers (notify, health, metrics, dashboard)
│   ├── cleanup/          # Scheduler de purga de mensajes viejos
│   ├── client/           # HTTP client para CLI send/wrap/status/metric
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

Collector → comandos shell cada N segundos
    ↓
SQLite (metrics + metric_reads)
    ↓
Evaluator → umbral cruzado? → alerta a Telegram
    ↓
API REST → Dashboard web (polling 30s)
```

## Deploy

### Instalacion automatica

```bash
curl -sL https://raw.githubusercontent.com/objetiva-comercios/objetiva-vps-jaimito/main/install.sh | bash
```

El script `install.sh` automatiza todo el flujo: verifica dependencias (Go 1.25+, git, systemd), clona el repositorio, compila el binario, lo instala en `/usr/local/bin/jaimito`, lanza el **setup wizard** para configurar interactivamente, instala el servicio systemd, y ejecuta un health check.

Es idempotente: si detecta una instalacion previa, detiene el servicio, actualiza el repositorio, recompila, y ofrece reconfigurar con el wizard.

### Actualizacion

```bash
# Re-ejecutar el instalador (detecta instalacion previa)
curl -sL https://raw.githubusercontent.com/objetiva-comercios/objetiva-vps-jaimito/main/install.sh | bash
```

Ver [DEPLOY.md](DEPLOY.md) para documentacion completa de deploy, troubleshooting, y configuracion manual.

### Setup wizard

`jaimito setup` es un wizard interactivo TUI (bubbletea) que guia la configuracion inicial:

1. **Bot Token** — Pide el token y lo valida contra la API de Telegram (`getMe`)
2. **Canal general** — Pide el chat ID y lo valida contra Telegram (`getChat`)
3. **Canales extra** — Permite agregar canales adicionales (deploys, errors, cron, etc.)
4. **Direccion HTTP** — Configura la direccion de escucha (default: `127.0.0.1:8080`)
5. **Base de datos** — Ruta al archivo SQLite (default: `/var/lib/jaimito/jaimito.db`)
6. **API Key** — Genera automaticamente una clave con prefijo `sk-`
7. **Resumen** — Muestra la configuracion completa y ofrece enviar una notificacion de test

## Estado del proyecto

- **v1.0 MVP** — completado (notificaciones, CLI, Telegram dispatch)
- **v1.1 Setup Wizard** — completado (wizard interactivo con validacion Telegram)
- **v2.0 Metricas y Dashboard** — en progreso (15 de 16 planes completados, fase 11 en ejecucion)
