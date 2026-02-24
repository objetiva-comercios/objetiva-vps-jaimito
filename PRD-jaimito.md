# Product Requirements Document
## **jaimito** — VPS Push Notification Hub

> **Versión:** 1.0  
> **Fecha:** Febrero 2026  
> **Estado:** Borrador  
> **Autor:** [Tu nombre]

---

## 1. Resumen Ejecutivo

**jaimito** es un servicio ligero y auto-hospedado diseñado para centralizar todas las notificaciones generadas en un VPS (alertas de servicios, resultados de cron jobs, eventos de aplicaciones, monitoreo de salud) y reenviarlas a múltiples canales de entrega como Telegram, correo electrónico, o cualquier servicio HTTP mediante un despachador genérico tipo cURL.

El sistema actúa como un hub único de notificaciones: recibe mensajes a través de distintos ingestores (webhook HTTP, CLI, file watcher), los almacena en una cola interna, aplica reglas de enrutamiento basadas en canales y prioridad, y los despacha a los destinos configurados con soporte de reintentos y agrupación inteligente.

---

## 2. Problema y Contexto

En un VPS típico con múltiples servicios y proyectos, las notificaciones están fragmentadas: cada servicio tiene su propia lógica de alerta (o no tiene ninguna), los scripts de cron fallan silenciosamente, y no hay un punto central para gestionar qué se notifica, cómo y a quién.

### 2.1 Problemas específicos

- No hay visibilidad unificada de los eventos del servidor.
- Los scripts de cron fallan sin que nadie se entere hasta que es tarde.
- Cada proyecto implementa su propia lógica de notificación (o ninguna).
- No hay historial centralizado de notificaciones enviadas.
- No existe forma de agrupar ráfagas de alertas repetitivas.
- Agregar un nuevo canal de entrega requiere modificar cada servicio individualmente.

### 2.2 Objetivos del producto

- **Centralizar:** Un único punto de entrada para todas las notificaciones.
- **Desacoplar:** Los servicios envían al hub; el hub decide cómo y dónde entregar.
- **Persistir:** Historial completo de notificaciones con búsqueda.
- **Ser resiliente:** Cola interna con reintentos automáticos.
- **Mínimo overhead:** Binario único, sin dependencias externas pesadas (no Redis, no Postgres).

---

## 3. Arquitectura General

El sistema se compone de tres capas desacopladas que se comunican a través de una cola interna:

| Capa | Componentes | Responsabilidad |
|------|-------------|-----------------|
| Ingesta | Webhook HTTP, CLI, File Watcher, Systemd Watcher | Recibir eventos y normalizarlos al formato interno |
| Cola interna | SQLite (tabla messages) | Persistir mensajes, gestionar estados y reintentos |
| Despacho | Telegram, Email (SMTP), HTTP Genérico (cURL) | Entregar notificaciones al destino final |

**Flujo de un mensaje:**

**Origen** → **Ingestor** → **Normalización** → **Cola (SQLite)** → **Router** → **Despachador** → **Destino**

---

## 4. Capa de Ingesta

La capa de ingesta es responsable de recibir mensajes desde distintas fuentes y normalizarlos a un formato interno unificado antes de encolarlos.

### 4.1 Webhook HTTP (v1)

Endpoint principal para recibir notificaciones vía HTTP POST. Es el ingestor más versátil y el único imprescindible para v1.

**Endpoint:** `POST /api/v1/notify`

**Headers requeridos:**

| Header | Valor | Descripción |
|--------|-------|-------------|
| Authorization | `Bearer <API_KEY>` | Token de autenticación |
| Content-Type | `application/json` | Tipo de contenido |

**Payload:**

| Campo | Tipo | Requerido | Descripción |
|-------|------|-----------|-------------|
| title | string | No | Título corto del mensaje (máx 200 chars) |
| body | string | Sí | Contenido del mensaje. Soporta Markdown básico |
| channel | string | No | Canal/topic (default: `"general"`) |
| priority | string | No | `critical` \| `high` \| `normal` \| `low` (default: `normal`) |
| tags | string[] | No | Etiquetas para filtrado y búsqueda |
| targets | string[] | No | Destinos específicos (override de las reglas del canal) |
| metadata | object | No | Datos adicionales arbitrarios (clave-valor) |
| dedupe_key | string | No | Clave para deduplicación en ventana de tiempo |

**Ejemplo de uso:**

```bash
curl -X POST https://vps.example.com/api/v1/notify \
  -H "Authorization: Bearer sk-abc123" \
  -H "Content-Type: application/json" \
  -d '{
    "title": "Deploy OK",
    "body": "App v2.1 deployed successfully",
    "channel": "deploys",
    "priority": "normal"
  }'
```

**Respuestas:**

| Código | Significado | Body |
|--------|-------------|------|
| 202 | Aceptado y encolado | `{"id":"msg_abc123","status":"queued"}` |
| 400 | Payload inválido | `{"error":"body is required"}` |
| 401 | Token inválido o faltante | `{"error":"unauthorized"}` |
| 429 | Rate limit excedido | `{"error":"too many requests","retry_after":30}` |

### 4.2 CLI Companion (v1)

Herramienta de línea de comandos que facilita el envío de notificaciones desde scripts, cron jobs y uso interactivo en terminal. Internamente es un wrapper del webhook HTTP, pero ofrece una interfaz ergonómica.

```bash
# Notificación simple
jaimito send "Backup completado con éxito"

# Con canal y prioridad
jaimito send -c deploys -p high "Deploy v2.1 fallido"

# Pipe desde stdin (captura salida de otro comando)
apt upgrade 2>&1 | jaimito send -c system --stdin

# Resultado de un comando (envía solo si falla)
jaimito wrap -c cron -- /opt/scripts/backup.sh
```

### 4.3 File Watcher (v1)

Monitor de archivos de log que observa cambios y genera notificaciones cuando detecta patrones configurados. Útil para servicios que escriben a log pero no tienen capacidad de webhook.

| Parámetro | Tipo | Descripción |
|-----------|------|-------------|
| path | string | Ruta al archivo a observar |
| patterns | regex[] | Patrones que disparan notificación |
| channel | string | Canal al que se envía el match |
| priority_map | object | Mapeo de patrón a prioridad |
| debounce_seconds | int | Ventana de debounce para evitar ráfagas |

### 4.4 Systemd Watcher (v2)

Monitor de units de systemd que detecta cambios de estado (`failed`, `restarting`, `inactive`) y genera notificaciones automáticamente. Se implementará en v2, escuchando el bus de D-Bus de systemd.

---

## 5. Formato Interno de Mensaje

Todos los ingestores normalizan los eventos al siguiente esquema interno antes de persistirlos en la cola:

| Campo | Tipo | Descripción |
|-------|------|-------------|
| id | UUID v7 | Identificador único, ordenable por tiempo |
| created_at | datetime (UTC) | Timestamp de recepción |
| source | string | Ingestor de origen (webhook, cli, filewatcher) |
| channel | string | Canal/topic asignado |
| priority | enum | `critical` \| `high` \| `normal` \| `low` |
| title | string \| null | Título opcional |
| body | string | Contenido del mensaje |
| tags | string[] | Etiquetas para filtrado |
| metadata | jsonb | Datos adicionales del origen |
| dedupe_key | string \| null | Clave de deduplicación |
| status | enum | `queued` \| `dispatching` \| `delivered` \| `failed` \| `grouped` |
| targets_override | string[] \| null | Destinos explícitos (bypass de reglas) |
| retry_count | int | Número de reintentos realizados |
| delivered_at | datetime \| null | Timestamp de entrega exitosa |
| error | string \| null | Último mensaje de error |

---

## 6. Canales y Topics

Los canales son la unidad organizativa principal del sistema. Cada notificación pertenece a un canal, y cada canal tiene reglas de enrutamiento que determinan a qué destinos se envía y bajo qué condiciones.

### 6.1 Canales predefinidos

| Canal | Descripción | Destinos por defecto | Prioridad mínima |
|-------|-------------|----------------------|------------------|
| general | Notificaciones genéricas | Telegram | normal |
| deploys | Despliegues de aplicaciones | Telegram | normal |
| errors | Errores de aplicaciones | Telegram + Email | high |
| cron | Resultados de cron jobs | Telegram | normal |
| system | Alertas del sistema (disco, RAM, CPU) | Telegram + Email | high |
| security | Eventos de seguridad (SSH, firewall) | Telegram + Email | critical |
| monitoring | Health checks y uptime | Telegram | normal |

### 6.2 Configuración de canal

Cada canal se configura en el archivo YAML principal del servicio:

```yaml
channels:
  errors:
    targets: [telegram, email]
    min_priority: high
    rate_limit: 10/minute
    group_window: 60s
    quiet_hours: "23:00-07:00"
    quiet_hours_action: hold  # hold | downgrade | deliver
```

### 6.3 Canales dinámicos

Los canales pueden crearse implícitamente al enviar un mensaje con un canal no existente. En ese caso, hereda la configuración del canal `"general"`. También pueden crearse explícitamente vía API o archivo de configuración.

---

## 7. Prioridad y Severidad

Cada mensaje tiene un nivel de prioridad que determina su comportamiento de enrutamiento, agrupación y entrega:

| Prioridad | Comportamiento | Agrupación | Quiet Hours | Reintentos |
|-----------|---------------|------------|-------------|------------|
| critical | Entrega inmediata a TODOS los destinos configurados | Nunca se agrupa | Se ignoran (siempre entrega) | 10, backoff exponencial |
| high | Entrega inmediata a destinos del canal | Solo tras 5+ en ventana | Telegram sí, email se retiene | 5 reintentos |
| normal | Entrega estándar según reglas del canal | Según group_window | Se retienen hasta fin de periodo | 3 reintentos |
| low | Entrega diferida (batch cada 5 min) | Siempre en digest | Se descartan | 1 reintento |

### 7.1 Escalado automático

El sistema puede escalar la prioridad de un mensaje automáticamente en los siguientes casos:

- Si un mensaje con prioridad `normal` falla en entrega 3 veces consecutivas, se escala a `high`.
- Si se reciben más de N mensajes del mismo canal en M segundos (configurable), el primero se escala a `high` como alerta de ráfaga.
- Los mensajes del canal `security` siempre se tratan como mínimo `high`, independientemente de la prioridad enviada.

---

## 8. Rate Limiting y Agrupación

### 8.1 Rate Limiting

El rate limiting se aplica en dos niveles para prevenir abusos y proteger los límites de las APIs de destino.

**Rate limit global:**

| Parámetro | Valor por defecto | Configurable |
|-----------|-------------------|--------------|
| Máx mensajes/minuto (ingesta) | 60 | Sí |
| Máx mensajes/hora (ingesta) | 500 | Sí |
| Máx mensajes/minuto por destino | 20 | Sí |
| Burst permitido | 10 mensajes en 5 seg | Sí |

**Rate limit por canal:** Cada canal puede definir su propio rate limit. Si se excede, los mensajes excedentes se encolan con estado `rate_limited` y se agrupan en el siguiente ciclo de despacho.

### 8.2 Agrupación de mensajes

Cuando múltiples mensajes del mismo canal llegan dentro de una ventana de tiempo configurable (`group_window`), el sistema los agrupa en un único mensaje de tipo digest.

- Los mensajes se agrupan por canal + prioridad.
- El digest incluye: número total de mensajes, lista resumida de los primeros 5 títulos, enlace al historial completo (si hay dashboard).
- Un mensaje `critical` nunca se agrupa, siempre se entrega individualmente.
- La ventana de agrupación por defecto es 60 segundos, configurable por canal.

### 8.3 Deduplicación

Los mensajes que incluyen un `dedupe_key` se deduplican dentro de una ventana de 5 minutos (configurable). Si llega un mensaje con un `dedupe_key` ya visto en esa ventana, se descarta con respuesta 202 pero estado `deduped`. Esto previene que errores en bucle generen cientos de notificaciones idénticas.

---

## 9. Capa de Despacho (Dispatchers)

Los despachadores son plugins que envían los mensajes a su destino final. Cada despachador implementa una interfaz común y se registra en la configuración.

### 9.1 Telegram (v1)

Despachador principal. Envía mensajes a un chat o grupo de Telegram usando la Bot API.

| Configuración | Tipo | Descripción |
|---------------|------|-------------|
| bot_token | string | Token del bot de Telegram |
| chat_id | string | ID del chat destino (puede ser grupo) |
| parse_mode | string | `MarkdownV2` \| `HTML` (default: `MarkdownV2`) |
| disable_preview | bool | Desactivar previews de links (default: `true`) |
| thread_id | int \| null | ID del hilo en grupos con topics habilitados |

Formato de mensaje: El título se muestra en negrita, seguido del cuerpo. Se incluye un emoji según la prioridad, el canal como hashtag, y las tags como hashtags adicionales.

### 9.2 Email / SMTP (v1)

Envía notificaciones por correo electrónico usando SMTP directo. Ideal como canal secundario para alertas críticas.

| Configuración | Tipo | Descripción |
|---------------|------|-------------|
| smtp_host | string | Servidor SMTP |
| smtp_port | int | Puerto (465 para SSL, 587 para STARTTLS) |
| smtp_user | string | Usuario de autenticación |
| smtp_pass | string | Contraseña |
| from_address | string | Dirección del remitente |
| to_addresses | string[] | Destinatarios |
| subject_prefix | string | Prefijo del asunto (default: `"[jaimito]"`) |

### 9.3 HTTP Genérico / cURL (v1)

Despachador abierto que permite enviar notificaciones a cualquier servicio externo que acepte HTTP. Funciona como un cliente cURL configurable: se define la URL, el método, headers, cuerpo y parámetros de query, y el sistema construye la petición dinámicamente usando templates con los datos del mensaje.

**Configuración base:**

| Campo | Tipo | Descripción |
|-------|------|-------------|
| name | string | Nombre único del destino (ej: `"slack-ops"`, `"discord-alerts"`, `"custom-api"`) |
| url | string (template) | URL destino. Soporta variables: `{{title}}`, `{{body}}`, `{{channel}}`, `{{priority}}` |
| method | string | Método HTTP: `POST` \| `PUT` \| `PATCH` \| `GET` (default: `POST`) |
| headers | map[string]string | Headers personalizados. Soporta templates en valores |
| query_params | map[string]string | Parámetros de query string. Soporta templates |
| body_template | string (template) | Cuerpo de la petición. Template con acceso a todos los campos del mensaje |
| content_type | string | Content-Type del body (default: `application/json`) |
| timeout_seconds | int | Timeout de la petición (default: 10) |
| success_codes | int[] | Códigos HTTP considerados exitosos (default: `[200, 201, 202, 204]`) |
| auth_type | string | `none` \| `bearer` \| `basic` \| `custom_header` (default: `none`) |
| auth_value | string | Token bearer, credenciales basic (`user:pass`), o valor del header custom |
| auth_header | string | Nombre del header de auth si `auth_type=custom_header` |
| retry_on_codes | int[] | Códigos HTTP que disparan reintento (default: `[429, 500, 502, 503]`) |

**Variables disponibles en templates:**

| Variable | Tipo | Ejemplo |
|----------|------|---------|
| `{{title}}` | string | Deploy fallido |
| `{{body}}` | string | Error en build paso 3... |
| `{{channel}}` | string | deploys |
| `{{priority}}` | string | high |
| `{{tags}}` | string (comma-sep) | app-web,prod |
| `{{tags_json}}` | string (JSON array) | `["app-web","prod"]` |
| `{{timestamp}}` | string (ISO 8601) | 2026-02-19T15:30:00Z |
| `{{id}}` | string (UUID) | msg_01JABCDEF... |
| `{{source}}` | string | webhook |
| `{{metadata}}` | string (JSON) | `{"commit":"a1b2c3"}` |
| `{{metadata.KEY}}` | string | Acceso directo a un campo de metadata |

**Ejemplos de configuración:**

```yaml
dispatchers:
  # Ejemplo 1: Discord via webhook nativo
  discord-ops:
    type: http
    url: https://discord.com/api/webhooks/1234/abcd
    method: POST
    body_template: '{"content":"[{{priority}}] **{{title}}**\n{{body}}"}'

  # Ejemplo 2: Slack via Incoming Webhook
  slack-alerts:
    type: http
    url: https://hooks.slack.com/services/T00/B00/xxxx
    method: POST
    body_template: '{"text":"{{title}}\n{{body}}","channel":"#ops"}'

  # Ejemplo 3: Ntfy (self-hosted o ntfy.sh)
  ntfy-phone:
    type: http
    url: https://ntfy.sh/mi-vps-alerts
    method: POST
    headers:
      Title: "{{title}}"
      Priority: "{{priority}}"
      Tags: "{{tags}}"
    body_template: "{{body}}"
    content_type: text/plain

  # Ejemplo 4: API propia con auth Bearer
  mi-dashboard:
    type: http
    url: https://dashboard.example.com/api/events
    method: POST
    auth_type: bearer
    auth_value: sk-dashboard-token-xyz
    body_template: >
      {
        "event": "notification",
        "title": "{{title}}",
        "body": "{{body}}",
        "channel": "{{channel}}",
        "priority": "{{priority}}",
        "metadata": {{metadata}}
      }
```

### 9.4 Despachadores futuros (v2+)

- WhatsApp Business API (requiere número verificado y cuenta de negocio).
- PagerDuty / OpsGenie (para integración con equipos on-call).
- Matrix / Gotify (alternativas self-hosted).

---

## 10. Cola Interna y Persistencia

### 10.1 Motor de almacenamiento

SQLite en modo WAL (Write-Ahead Logging) como único backend de persistencia. Se elige SQLite por: cero dependencias externas, excelente rendimiento para el volumen esperado (cientos de mensajes/hora), backup trivial (copiar un archivo), y compatibilidad universal.

### 10.2 Esquema de la base de datos

**Tabla: messages**

| Columna | Tipo | Índice | Descripción |
|---------|------|--------|-------------|
| id | TEXT (UUID v7) | PK | Identificador único ordenable |
| created_at | DATETIME | Índice | Timestamp de creación |
| source | TEXT | - | Ingestor de origen |
| channel | TEXT | Índice | Canal asignado |
| priority | TEXT | Índice | Nivel de prioridad |
| title | TEXT | - | Título opcional |
| body | TEXT | - | Contenido |
| tags | TEXT (JSON) | - | Etiquetas serializadas |
| metadata | TEXT (JSON) | - | Datos adicionales |
| dedupe_key | TEXT | Índice | Clave de deduplicación |
| status | TEXT | Índice | Estado actual |
| targets_override | TEXT (JSON) | - | Destinos explícitos |
| retry_count | INTEGER | - | Reintentos realizados |
| next_retry_at | DATETIME | Índice | Próximo reintento |
| delivered_at | DATETIME | - | Timestamp de entrega |
| error | TEXT | - | Último error |

**Tabla: dispatch_log**

Registro de cada intento de entrega individual:

| Columna | Tipo | Descripción |
|---------|------|-------------|
| id | INTEGER AUTOINCREMENT | PK |
| message_id | TEXT | FK a messages.id |
| target | TEXT | Despachador usado (telegram, email, etc) |
| attempted_at | DATETIME | Timestamp del intento |
| success | BOOLEAN | Resultado |
| response_code | INTEGER | Código HTTP de respuesta |
| error | TEXT | Detalle del error si falló |

### 10.3 Retención de datos

- Mensajes entregados: se retienen 30 días (configurable).
- Mensajes fallidos: se retienen 90 días.
- Dispatch log: se retiene 7 días.
- Un job de limpieza corre diariamente para purgar registros vencidos.

---

## 11. Autenticación y Seguridad

### 11.1 API Keys

El sistema soporta múltiples API keys, cada una con un nombre descriptivo y opcionalmente restricciones de canales. Esto permite dar a cada servicio o proyecto su propia key con acceso solo a los canales relevantes.

| Campo | Tipo | Descripción |
|-------|------|-------------|
| name | string | Nombre descriptivo (ej: `"backup-script"`) |
| key | string | Token generado (prefijo `sk-`) |
| allowed_channels | string[] \| null | Canales permitidos (null = todos) |
| created_at | datetime | Fecha de creación |
| last_used_at | datetime | Último uso |
| active | bool | Estado de la key |

### 11.2 Seguridad de la red

- El webhook debe estar detrás de HTTPS (via reverse proxy como Nginx/Caddy).
- Se recomienda limitar acceso por IP si solo se usa desde el propio VPS (bind a `127.0.0.1`).
- Si se expone públicamente, considerar rate limiting adicional en el reverse proxy.
- Las credenciales de los despachadores (bot tokens, SMTP passwords) se almacenan en el archivo de configuración con permisos `600`.

### 11.3 Gestión de keys via CLI

```bash
jaimito keys create --name backup-script --channels cron,system
jaimito keys list
jaimito keys revoke sk-abc123
```

---

## 12. Configuración

Toda la configuración se define en un único archivo YAML. La ubicación por defecto es `/etc/jaimito/config.yaml`, override con `--config` o la variable de entorno `JAIMITO_CONFIG`.

```yaml
server:
  host: 127.0.0.1
  port: 8787

database:
  path: /var/lib/jaimito/jaimito.db
  retention_days: 30

dispatchers:
  telegram:
    bot_token: "123456:ABC-DEF"
    chat_id: "-1001234567890"
  email:
    smtp_host: smtp.gmail.com
    smtp_port: 587
    # ...etc

channels:
  errors:
    targets: [telegram, email]
    min_priority: high
    rate_limit: 10/minute
    group_window: 60s
```

---

## 13. API de Consulta

Además del endpoint de envío, el sistema expone endpoints de consulta para verificar el estado del servicio y consultar el historial de notificaciones.

| Método | Endpoint | Descripción |
|--------|----------|-------------|
| GET | `/api/v1/health` | Health check (no requiere auth) |
| GET | `/api/v1/messages` | Listar mensajes con filtros (channel, priority, status, desde, hasta) |
| GET | `/api/v1/messages/:id` | Detalle de un mensaje con su dispatch log |
| GET | `/api/v1/stats` | Estadísticas: mensajes por canal, tasa de entrega, errores |
| DELETE | `/api/v1/messages/:id` | Cancelar un mensaje encolado (solo si status=queued) |

---

## 14. Consideraciones Técnicas

### 14.1 Stack tecnológico recomendado

| Componente | Tecnología | Justificación |
|------------|------------|---------------|
| Lenguaje | Go | Binario único, bajo consumo de memoria, concurrencia nativa |
| Base de datos | SQLite (CGo) | Cero dependencias, WAL mode para concurrencia |
| HTTP server | net/http o Chi | Minimalista, sin frameworks pesados |
| Configuración | YAML (gopkg.in/yaml.v3) | Legible y estándar para config de servidores |
| CLI | cobra | Estándar de facto para CLIs en Go |
| Proceso | systemd unit | Gestión nativa de servicio en Linux |

Alternativa válida: Rust (con axum + rusqlite) si se prefiere mayor garantía de seguridad de memoria, o Python (FastAPI + aiosqlite) si se prioriza velocidad de desarrollo sobre rendimiento.

### 14.2 Despliegue

- Binario único compilado para `linux/amd64` (y opcionalmente `arm64`).
- Unit de systemd para gestión del servicio (start, stop, restart, logs).
- Directorio de datos: `/var/lib/jaimito/` (base de datos).
- Directorio de config: `/etc/jaimito/` (configuración).
- Logs: stdout/stderr capturados por journald.

### 14.3 Observabilidad

- Endpoint `/api/v1/health` para monitores externos.
- Logging estructurado en formato JSON a stdout.
- Opcionalmente: el propio jaimito puede notificarse a sí mismo sobre sus propios errores (meta-notificación).

---

## 15. Roadmap

| Versión | Alcance | Estimación |
|---------|---------|------------|
| v0.1 (MVP) | Webhook HTTP + CLI send + Telegram dispatcher + SQLite + config YAML básica | 2-3 semanas |
| v0.2 | Email SMTP dispatcher + canales con reglas + prioridad + rate limiting | 1-2 semanas |
| v0.3 | HTTP Genérico dispatcher + agrupación + deduplicación | 1-2 semanas |
| v0.4 | File watcher + CLI wrap + API de consulta + estadísticas | 2 semanas |
| v1.0 | Estabilización + documentación + tests + empaquetado | 1-2 semanas |
| v2.0 | Systemd watcher + dashboard web + WhatsApp + despachadores avanzados | Futuro |

---

## 16. Métricas de Éxito

- Tasa de entrega exitosa > 99% en condiciones normales.
- Latencia de entrega < 5 segundos desde recepción hasta despacho (prioridad normal).
- Latencia de entrega < 1 segundo para prioridad critical.
- Consumo de memoria < 50 MB en operación normal.
- Cero notificaciones perdidas (toda notificación recibida debe quedar persistida).
- 100% de cron jobs del VPS monitoreados a través de jaimito en un mes post-despliegue.
