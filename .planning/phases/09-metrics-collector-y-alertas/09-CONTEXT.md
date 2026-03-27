# Phase 9: Metrics Collector y Alertas - Context

**Gathered:** 2026-03-27
**Status:** Ready for planning

<domain>
## Phase Boundary

jaimito recolecta métricas del sistema de forma autónoma a intervalos configurables y envía alertas a Telegram cuando una métrica cruza un umbral por primera vez. Incluye: collector loop con goroutines, ejecución de shell commands, evaluación de thresholds, state machine de alertas (ok/warning/critical), cooldown, envío de alertas via dispatcher existente, e integración en el startup de serve.go. NO incluye: API REST (Phase 10), CLI metric/status (Phase 10), dashboard (Phase 11), scheduler de purge (Phase 12).

</domain>

<decisions>
## Implementation Decisions

### Scheduling del collector
- **D-01:** Una goroutine por métrica con su propio `time.NewTicker` independiente. Consistente con el patrón de `dispatcher.Start()` y `cleanup.Start()`. Escala bien para 5-20 métricas y aísla fallos entre métricas.
- **D-02:** El collector arranca automáticamente en `serve.go` si `cfg.Metrics != nil`. Sin flag extra. Consistente con D-04 de Phase 8 (sin sección metrics = deshabilitado).

### Ejecución de métricas (shell commands)
- **D-03:** Se descarta gopsutil/v4. Todas las métricas (predefinidas y custom) son shell commands definidos en config.yaml. Las predefinidas usan `df`/`free`/`awk`/`docker` etc. Fiel a D-02 de Phase 8 (no hardcodear en Go).
- **D-04:** Los commands se ejecutan con `exec.CommandContext(ctx, "sh", "-c", def.Command)`. POSIX sh, no bash. Portable a cualquier Linux.
- **D-05:** Timeout = min(80% del intervalo, 30s) + `cmd.WaitDelay = 5s` (decisión heredada de investigación).

### Formato de alertas Telegram
- **D-06:** Las alertas van al canal "general" (primer canal configurado). Sin canal dedicado ni campo channel por métrica.
- **D-07:** Prioridad mapeada desde umbral: `warning` → priority `"high"` (⚠️), `critical` → priority `"critical"` (🔴). Aprovecha el sistema de emojis del dispatcher existente.
- **D-08:** Formato compacto: título con emoji + nombre + transición (ej: "📉 disk_root: ok → warning"), body con valor actual, umbral cruzado, y hostname del VPS.

### State machine y restart
- **D-09:** Al arrancar, rehidratar el state machine desde `last_status` de la tabla `metrics` (via `ListMetrics()`). Evita alertas duplicadas tras restart.
- **D-10:** Persistir `last_value` y `last_status` en la DB en cada lectura (via `UpdateMetricStatus()` existente), no solo en transiciones. Siempre consistente ante crash.
- **D-11:** Alertas solo en transición de nivel (ok→warning, warning→critical, critical→ok recovery, etc.), no en cada poll. Cooldown configurable (default 30min) con estado en memoria rehidratado desde DB.

### Claude's Discretion
- Estructura interna del paquete `internal/collector/` (archivos, funciones auxiliares)
- Formato exacto del hostname en el body de la alerta
- Manejo de errores internos del collector (logging strategy)
- Patrón collect-then-write: detalles de implementación del flujo collect → parse → evaluate → persist → alert

</decisions>

<canonical_refs>
## Canonical References

**Downstream agents MUST read these before planning or implementing.**

### Config y tipos (Phase 8 output)
- `internal/config/config.go` — MetricsConfig, MetricDef, Thresholds structs, ParseDuration (exportada para Phase 9+)
- `configs/config.example.yaml` — Ejemplo con sección metrics (predefinidas como shell commands)

### DB CRUD (Phase 8 output)
- `internal/db/metrics.go` — UpsertMetric, InsertReading, UpdateMetricStatus, ListMetrics, QueryReadings, PurgeOldReadings
- `internal/db/schema/003_metrics.sql` — Tablas metrics y metric_readings

### Patrones existentes
- `internal/dispatcher/dispatcher.go` — Patrón goroutine+ticker+context para el collector
- `internal/cleanup/` — Otro ejemplo de background scheduler
- `internal/db/messages.go` — EnqueueMessage() para enviar alertas via pipeline existente
- `cmd/jaimito/serve.go` — Startup sequence donde se integra collector.Start()

### Requisitos
- `.planning/REQUIREMENTS.md` — MCOL-01, MCOL-02, MCOL-04, MCOL-05, ALRT-02, ALRT-03, ALRT-04
- `.planning/ROADMAP.md` — Phase 9 success criteria

</canonical_refs>

<code_context>
## Existing Code Insights

### Reusable Assets
- `db.UpdateMetricStatus()` — Ya existe, actualiza last_value y last_status en tabla metrics
- `db.InsertReading()` — Ya existe, graba lectura en metric_readings
- `db.ListMetrics()` — Ya existe, se usa para rehidratar state machine al arrancar
- `db.UpsertMetric()` — Ya existe, se llama al startup para poblar tabla metrics desde config
- `db.EnqueueMessage()` — Para enviar alertas via el pipeline de notificaciones existente
- `config.ParseDuration()` — Exportada en Phase 8, parsea "5m", "30s", "7d" etc.

### Established Patterns
- Single-writer SQLite pool (MaxOpenConns=1) — collector escribe en serie, sin contención
- Goroutine+ticker+context — dispatcher y cleanup ya usan este patrón exacto
- `slog` para logging estructurado — usar para errores de métricas individuales
- RFC3339 timestamps en SQLite — consistente con Phase 8 decision

### Integration Points
- `serve.go` línea ~84 — después de `cleanup.Start()`, agregar `collector.Start(ctx, database, cfg)` si `cfg.Metrics != nil`
- `db.EnqueueMessage()` — interfaz para enviar alertas al canal general
- `config.MetricDef` — struct ya definida con todos los campos necesarios para el collector

</code_context>

<specifics>
## Specific Ideas

No specific requirements — open to standard approaches

</specifics>

<deferred>
## Deferred Ideas

None — discussion stayed within phase scope

</deferred>

---

*Phase: 09-metrics-collector-y-alertas*
*Context gathered: 2026-03-27*
