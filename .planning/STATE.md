---
gsd_state_version: 1.0
milestone: v2.0
milestone_name: Métricas y Dashboard
status: Ready to plan
stopped_at: Completed 10-rest-api-y-cli-02-PLAN.md
last_updated: "2026-03-27T12:41:40.952Z"
progress:
  total_phases: 9
  completed_phases: 7
  total_plans: 14
  completed_plans: 14
---

# Project State

## Project Reference

See: .planning/PROJECT.md (updated 2026-03-26)

**Core value:** Every event that happens on the VPS gets reliably captured and delivered to Telegram — no notification is ever lost silently.
**Current focus:** Phase 10 — rest-api-y-cli

## Current Position

Phase: 11
Plan: Not started

## Performance Metrics

**Velocity (v1.1 reference):**

- Total plans completed: 10 (v1.0 + v1.1)
- Average plans per phase: 2
- Estimated plans v2.0: ~10 (2/phase x 5 phases)

**By Phase (v1.1 actual):**

| Phase | Plans | Total | Avg/Plan |
|-------|-------|-------|----------|
| 4. Wizard Scaffold | 2/2 | — | — |
| 5. Validacion Telegram | 2/2 | — | — |
| 6. Configuracion y Escritura | 2/2 | — | — |
| 7. Verificacion e Integracion | 2/2 | — | — |

**Recent Trend (v1.1 raw data):**

| Phase 04-wizard-scaffold P01 | 8 | 1 tasks | 8 files |
| Phase 04-wizard-scaffold P02 | 4 | 1 tasks | 4 files |
| Phase 05-validacion-telegram P01 | 20 | 2 tasks | 7 files |
| Phase 05 P02 | 5 | 2 tasks | 5 files |
| Phase 06 P01 | 6 | 2 tasks | 11 files |
| Phase 06 P02 | 5 | 1 tasks | 3 files |
| Phase 07 P01 | 15 | 1 tasks | 2 files |
| Phase 07 P02 | 15 | 2 tasks | 1 files |
| Phase 08 P01 | 12 | 2 tasks | 3 files |
| Phase 08 P02 | 132 | 2 tasks | 4 files |
| Phase 09 P01 | 3 | 2 tasks | 3 files |
| Phase 09-metrics-collector-y-alertas P02 | 10 | 2 tasks | 5 files |
| Phase 10 P01 | 8 | 2 tasks | 4 files |
| Phase 10-rest-api-y-cli P02 | 2 | 2 tasks | 3 files |

## Accumulated Context

### Decisions

Decisions are logged in PROJECT.md Key Decisions table.

**Decisiones heredadas de v1.0/v1.1 relevantes para v2.0:**

- Single-writer SQLite pool (SetMaxOpenConns(1)) — suficiente y correcto; no abrir segunda conexion para el dashboard
- chi v5 para HTTP — el dashboard y la API de metricas se registran como rutas adicionales en el router existente
- modernc.org/sqlite (CGO-free) — sin cambio; migracion numerada `003_metrics.sql` sigue el patron existente
- cobra para CLI — `jaimito metric` y `jaimito status` son nuevos subcomandos cobras
- Dispatcher existente sin cambios — alertas de umbral se enolan via `db.EnqueueMessage()` existente

**Decisiones de arquitectura v2.0 (de investigacion):**

- gopsutil/v4 para metricas predefinidas (CGO-free, amd64+arm64)
- Alpine.js v3 + uPlot v1.6.32 + Tailwind CSS pre-compilado embedidos via go:embed
- Tailwind debe pre-compilarse con CLI standalone antes de `go build` — NO usar Play CDN
- exec.CommandContext con timeout = min(80% del intervalo, 30s) + cmd.WaitDelay = 5s
- Patron collect-then-write: ejecutar comando fuera de transaccion, luego INSERT inmediato
- go:embed requiere rutas relativas al archivo que contiene la directiva — embed.go en internal/web/
- Alert state machine (ok/warning/critical) desde el inicio — no retrofittable
- Cooldown configurable (default 30 minutos) con estado persistido en memoria (no en DB)
- [Phase 08]: MetricsConfig uses pointer (*MetricsConfig) so configs without metrics section parse to nil — backward compatible with v1.0/v1.1 configs (D-04)
- [Phase 08]: parseDuration not exported; ParseDuration (uppercase) exported for Phase 9+ use
- [Phase 08-02]: strftime RFC3339 en tablas de metricas en vez de datetime('now') — habilita queries de rango con time.RFC3339 desde Go
- [Phase 08-02]: QueryReadings y ListMetrics retornan slice vacio (no nil) — contrato consistente para callers
- [Phase 09]: runCommand usa 'sh -c' (POSIX sh) con WaitDelay=5s para forzar kill despues del timeout
- [Phase 09]: Status hardcodeado 'ok' en Phase 9-01; state machine real (ok/warning/critical) implementada en 9-02
- [Phase 09-02]: shouldAlert cooldown: !s.lastAlert.IsZero() garantiza primera alerta siempre; recovery (cualquier->ok) ignora cooldown
- [Phase 09-02]: evaluator.go separado de collector.go — responsabilidades claras: state machine vs loop
- [Phase 10]: Thresholds vienen del config en memoria (no de la DB) — GET /api/v1/metrics incluye campo thresholds solo cuando existen en cfg.Metrics.Definitions
- [Phase 10]: IngestHandler usa status hardcodeado 'ok' — solo el evaluator evalua thresholds (D-10); escritura manual no interfiere con state machine de alertas
- [Phase 10-rest-api-y-cli]: text/tabwriter en jaimito status: lipgloss/v2 table API no verificada en RESEARCH, tabwriter stdlib es mas seguro
- [Phase 10-rest-api-y-cli]: GetMetrics sin auth header: endpoint publico per D-06 de investigacion de metricas

### Pending Todos

- [ ] Precompilar Tailwind CSS antes de Phase 11 — requiere Tailwind CLI standalone instalado localmente
- [ ] Verificar API exacta de gopsutil/v4 para docker running container count antes de Phase 9
- [ ] Definir routing del dashboard (hash vs single-path) al inicio de Phase 11

### Blockers/Concerns

None.

## Session Continuity

Last session: 2026-03-27T12:36:37.954Z
Stopped at: Completed 10-rest-api-y-cli-02-PLAN.md
Resume file: None
