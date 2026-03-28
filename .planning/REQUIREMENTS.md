# Requirements: jaimito

**Defined:** 2026-03-24
**Core Value:** Every event that happens on the VPS gets reliably captured and delivered to Telegram — no notification is ever lost silently.

## v1.1 Requirements (Complete)

### Wizard TUI

- [x] **WIZ-01**: Operador puede ejecutar `jaimito setup` y ver un wizard interactivo paso-a-paso
- [x] **WIZ-02**: Wizard detecta si ya existe config y ofrece editar/crear desde cero/cancelar
- [x] **WIZ-03**: Wizard detecta terminal no-interactiva y aborta con error descriptivo

### Validación Telegram

- [x] **TELE-01**: Operador ingresa bot token y se valida en vivo contra API (retorna username y display name)
- [x] **TELE-02**: Operador ingresa chat ID del canal general y se valida contra `bot.GetChat()`
- [x] **TELE-03**: Operador puede agregar canales extra (deploys, errors, cron, etc.) con validación de cada chat ID

### Configuración

- [x] **CONF-01**: Wizard genera API key criptográfica (`sk-` prefix) y la muestra al operador
- [x] **CONF-02**: Wizard genera config YAML válido y lo escribe con permisos 0600
- [x] **CONF-03**: Operador ve un resumen completo antes de confirmar escritura
- [x] **CONF-04**: Config generado pasa `config.Validate()` antes de escribirse

### Verificación

- [x] **TEST-01**: Wizard envía notificación de test al canal general para probar el setup completo

### Integración

- [x] **INST-01**: `install.sh` invoca `jaimito setup` automáticamente cuando no existe config (con `< /dev/tty`)

## v2.0 Requirements

Requirements for Métricas y Dashboard milestone. Each maps to roadmap phases.

### Metrics Collection

- [x] **MCOL-01**: Jaimito ejecuta comandos shell a intervalos configurables para recolectar métricas del sistema
- [x] **MCOL-02**: Métricas predefinidas incluidas: disk_root (%), ram_used (%), cpu_load, docker_running, uptime_days
- [x] **MCOL-03**: El usuario puede definir métricas custom en config.yaml con nombre, categoría, comando, intervalo y tipo
- [x] **MCOL-04**: Cada comando se ejecuta con timeout de 10 segundos (exec.CommandContext + WaitDelay)
- [x] **MCOL-05**: Si un comando falla, se registra en logs sin afectar las demás métricas

### Storage

- [ ] **STOR-01**: Los readings se almacenan en tabla metric_readings (SQLite) con metric_name, value, recorded_at
- [ ] **STOR-02**: Purga automática de readings mayores a 7 días (reutiliza patrón de cleanup existente)
- [x] **STOR-03**: La sección metrics en config.yaml define retention y alert_cooldown globales

### Dashboard

- [x] **DASH-01**: Dashboard web accesible en GET /dashboard servido desde archivos embedidos (go:embed)
- [ ] **DASH-02**: Vista principal: tabla con todas las métricas (nombre, valor actual, sparkline de tendencia, indicador de estado)
- [ ] **DASH-03**: Click en una fila expande un gráfico temporal (uPlot) con historial de esa métrica
- [ ] **DASH-04**: Auto-refresh cada 30 segundos sin recargar la página (Alpine.js polling)
- [x] **DASH-05**: Frontend usa Tailwind CSS pre-compilado, Lucide icons inline SVG, Alpine.js, uPlot
- [ ] **DASH-06**: El dashboard muestra hostname del VPS y timestamp de última actualización

### Alerting

- [x] **ALRT-01**: Cada métrica define umbrales warning y critical en config.yaml
- [x] **ALRT-02**: Cuando una métrica cruza un umbral, jaimito envía alerta a Telegram via el dispatcher existente
- [x] **ALRT-03**: State machine por métrica (ok→warning→critical): alerta solo en transición de nivel, no en cada poll
- [x] **ALRT-04**: Cooldown configurable (default 30 minutos) para evitar alert storms

### CLI

- [x] **CLI-01**: `jaimito metric -n name --value X` envía una métrica manual via POST /api/v1/metrics
- [x] **CLI-02**: `jaimito status` muestra las métricas actuales en formato tabla (consulta GET /api/v1/metrics)

### API

- [x] **API-01**: GET /api/v1/metrics retorna lista de métricas con último valor, config y estado (sin auth)
- [x] **API-02**: GET /api/v1/metrics/{name}/readings retorna historial con query param ?since=2h|24h|7d (sin auth)
- [x] **API-03**: POST /api/v1/metrics permite ingestar readings manuales (con Bearer auth)

## Future Requirements

Deferred to future milestones. Tracked but not in current roadmap.

### Reliability

- **RATE-01**: Rate limiting per channel
- **DEDUP-01**: Simple deduplication (same message within N minutes)
- **QUIET-01**: Quiet hours (do-not-disturb window)

### CLI Extensions

- **LIST-01**: `jaimito list` CLI (query recent messages)
- **DRAIN-01**: Message queue drain on SIGTERM

### Observability

- **WAL-01**: WAL checkpoint monitoring in health endpoint

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Email/SMTP dispatcher | Apprise covers this; out of jaimito's scope |
| Multi-user support | Single-VPS model is a feature, not a bug |
| Plugin system | Maintenance burden exceeds value at this scale |
| WhatsApp/PagerDuty/Matrix | Deferred to v2+; recommend Apprise bridge |
| Message grouping/digest | Deferred to future milestone |
| Wizard web UI | CLI-only; keep single-binary principle |
| WebSocket real-time | Polling cada 30s satisface el caso de uso; WebSocket agrega complejidad sin beneficio |
| Prometheus exporter | Jaimito es autocontenido; exportar a Prometheus contradice el propósito |
| Centralización multi-VPS | Cada VPS tiene su propio dashboard; centralizar es v3+ |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| WIZ-01 | Phase 4 | Complete |
| WIZ-02 | Phase 4 | Complete |
| WIZ-03 | Phase 4 | Complete |
| TELE-01 | Phase 5 | Complete |
| TELE-02 | Phase 5 | Complete |
| TELE-03 | Phase 5 | Complete |
| CONF-01 | Phase 6 | Complete |
| CONF-02 | Phase 6 | Complete |
| CONF-03 | Phase 6 | Complete |
| CONF-04 | Phase 6 | Complete |
| TEST-01 | Phase 7 | Complete |
| INST-01 | Phase 7 | Complete |
| MCOL-01 | Phase 9 | Complete |
| MCOL-02 | Phase 9 | Complete |
| MCOL-03 | Phase 8 | Complete |
| MCOL-04 | Phase 9 | Complete |
| MCOL-05 | Phase 9 | Complete |
| STOR-01 | Phase 8 | Pending |
| STOR-02 | Phase 12 | Pending |
| STOR-03 | Phase 8 | Complete |
| DASH-01 | Phase 11 | Complete |
| DASH-02 | Phase 11 | Pending |
| DASH-03 | Phase 11 | Pending |
| DASH-04 | Phase 11 | Pending |
| DASH-05 | Phase 11 | Complete |
| DASH-06 | Phase 11 | Pending |
| ALRT-01 | Phase 8 | Complete |
| ALRT-02 | Phase 9 | Complete |
| ALRT-03 | Phase 9 | Complete |
| ALRT-04 | Phase 9 | Complete |
| CLI-01 | Phase 10 | Complete |
| CLI-02 | Phase 10 | Complete |
| API-01 | Phase 10 | Complete |
| API-02 | Phase 10 | Complete |
| API-03 | Phase 10 | Complete |

**Coverage:**
- v1.1 requirements: 12 total (all complete)
- v2.0 requirements: 23 total (all pending)
- Mapped to phases: 35 total (12 v1.1 + 23 v2.0)
- Unmapped: 0

---
*Requirements defined: 2026-03-24*
*Last updated: 2026-03-26 — v2.0 traceability mapped to Phases 8-12*
