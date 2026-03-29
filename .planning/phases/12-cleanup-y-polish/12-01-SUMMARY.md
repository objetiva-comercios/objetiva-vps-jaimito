---
phase: 12-cleanup-y-polish
plan: 01
subsystem: database
tags: [sqlite, cleanup, retention, metrics, config]

# Dependency graph
requires:
  - phase: 11-dashboard-web-embedido
    provides: dashboard embedido completo en Go binary
  - phase: 08-config-schema-y-crud
    provides: PurgeOldReadings en internal/db/metrics.go

provides:
  - cleanup scheduler con purga automatica de metric_readings via retention configurable
  - config.example.yaml con documentacion completa de todos los campos v2.0

affects:
  - operaciones en produccion (la DB ya no crece indefinidamente)

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "cleanup.Start acepta metricsRetention=0 como opt-out — backward compatible con configs sin metrics"
    - "Import renombrado a dbpkg para evitar colision con parametro db *sql.DB"

key-files:
  created:
    - .planning/phases/12-cleanup-y-polish/12-01-PLAN.md
    - .planning/phases/12-cleanup-y-polish/12-01-SUMMARY.md
  modified:
    - internal/cleanup/scheduler.go
    - cmd/jaimito/serve.go
    - configs/config.example.yaml

key-decisions:
  - "cleanup.Start acepta metricsRetention time.Duration — si es 0 no purga readings (backward compatible con configs v1.x sin seccion metrics)"
  - "serve.go extrae retention de cfg.Metrics.Retention via config.ParseDuration — mismo helper que usa el collector"

patterns-established:
  - "metricsRetention=0 como valor cero de time.Duration sirve como opt-out limpio — no se necesita puntero ni booleano"

requirements-completed:
  - STOR-02

# Metrics
duration: 3min
completed: 2026-03-29
---

# Phase 12 Plan 01: Cleanup y Polish Summary

**PurgeOldReadings integrado en cleanup scheduler con retention configurable desde config.yaml, DB nunca crece indefinidamente**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-29T15:34:16Z
- **Completed:** 2026-03-29T15:36:47Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments
- `cleanup.Start` ahora acepta `metricsRetention time.Duration` — si > 0, llama `PurgeOldReadings` en cada ciclo de 24h
- `serve.go` extrae `cfg.Metrics.Retention` y lo pasa al scheduler para que la purga use el valor configurado
- `configs/config.example.yaml` documenta todos los campos v2.0 con comentarios explicativos y 2 custom metrics de ejemplo

## Task Commits

1. **Task 1: Integrar PurgeOldReadings en cleanup scheduler** - `40671b2` (feat)
2. **Task 2: Verificar y completar config.example.yaml** - `08fb978` (chore)

## Files Created/Modified
- `internal/cleanup/scheduler.go` - nuevo parametro metricsRetention, funcion purgeMetrics, import dbpkg
- `cmd/jaimito/serve.go` - extrae metricsRetention desde cfg.Metrics.Retention y lo pasa a cleanup.Start
- `configs/config.example.yaml` - documentacion completa de la seccion metrics con comentarios y custom metrics de ejemplo

## Decisions Made
- `cleanup.Start` usa `metricsRetention time.Duration` con valor cero como opt-out — no requiere puntero ni booleano adicional; es idiomatico en Go
- `serve.go` usa `config.ParseDuration` (mismo helper que el collector) para parsear la retention string del config

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered
None.

## User Setup Required
None - no external service configuration required.

## Next Phase Readiness
- Milestone v2.0 completo: metrics collector, alertas, REST API, CLI, dashboard web embedido, y cleanup de retention
- El binario arranca, colecta metricas, sirve el dashboard en /dashboard y la API en /api/v1/metrics, y purga datos viejos automaticamente

## Self-Check: PASSED

- `internal/cleanup/scheduler.go` — FOUND
- `cmd/jaimito/serve.go` — FOUND
- `configs/config.example.yaml` — FOUND
- commit `40671b2` — FOUND (feat(12-01): integrar PurgeOldReadings)
- commit `08fb978` — FOUND (chore(12-01): config.example.yaml v2.0)

---
*Phase: 12-cleanup-y-polish*
*Completed: 2026-03-29*
