---
phase: 10-rest-api-y-cli
plan: 01
subsystem: api
tags: [go, chi, sqlite, rest, metrics, httptest, tdd]

requires:
  - phase: 09-metrics-collector-y-alertas
    provides: db.MetricRow, db.ReadingRow, db.ListMetrics, db.QueryReadings, db.InsertReading, db.UpdateMetricStatus
  - phase: 08-config-schema-y-crud
    provides: config.MetricsConfig, config.MetricDef, config.Thresholds, config.ParseDuration

provides:
  - "GET /api/v1/metrics — lista metricas con thresholds del config, sin auth"
  - "GET /api/v1/metrics/{name}/readings — historial de readings con ?since= (default 24h, max 7d)"
  - "POST /api/v1/metrics — ingesta manual de readings con Bearer auth, inserta reading + actualiza status"
  - "db.MetricExists — helper COUNT(1) para verificar existencia de metrica"
  - "13 tests httptest cubriendo happy path, 400/401/404 y edge cases"

affects:
  - phase: 10-02 (CLI commands que consumen estos endpoints)
  - phase: 11 (dashboard web que consulta GET /api/v1/metrics y readings)

tech-stack:
  added: []
  patterns:
    - "Handlers como closures sobre *sql.DB y *config.Config — patron ya establecido en NotifyHandler"
    - "chi.URLParam(r, 'name') para path params — requiere router completo en tests, no handler aislado"
    - "maxRetention const para validacion de ?since= — 7 * 24 * time.Hour"
    - "Pointer *float64 en IngestRequest para detectar value ausente vs zero"
    - "Thresholds con omitempty — Phase 11 espera ausencia del campo, no null"

key-files:
  created:
    - "internal/api/handlers_metrics_test.go — 13 tests httptest para los 3 endpoints"
  modified:
    - "internal/db/metrics.go — agregado db.MetricExists"
    - "internal/api/handlers.go — agregados tipos de respuesta y 3 handlers (MetricsListHandler, ReadingsHandler, IngestHandler)"
    - "internal/api/server.go — 3 rutas nuevas registradas en NewRouter"

key-decisions:
  - "Thresholds vienen del config en memoria, no de la DB — consistente con D-01 de investigacion"
  - "IngestHandler usa status hardcodeado 'ok' — evaluator es el unico que evalua thresholds (D-10)"
  - "GET /api/v1/metrics sin auth — datos de metricas son informativos, no sensibles"
  - "POST /api/v1/metrics con Bearer auth — escritura de datos requiere autenticacion"
  - "?since= default 24h, maximo 7d — alineado con retention policy del config"

patterns-established:
  - "Test helpers: newMetricsTestDB, testMetricsConfig, setupRouter en mismo package api para acceso a tipos internos"
  - "bearerHeader() helper para consistencia en tests de endpoints autenticados"

requirements-completed: [API-01, API-02, API-03]

duration: 8min
completed: 2026-03-27
---

# Phase 10 Plan 01: REST API de Metricas Summary

**3 endpoints REST de metricas en chi (GET lista, GET readings con ?since=, POST ingesta con auth) con helper db.MetricExists y 13 tests httptest que pasan.**

## Performance

- **Duration:** ~8 min
- **Started:** 2026-03-27T12:25:00Z
- **Completed:** 2026-03-27T12:33:00Z
- **Tasks:** 2 (TDD: RED + GREEN para ambas tareas)
- **Files modified:** 4

## Accomplishments

- Agregado `db.MetricExists` helper en `internal/db/metrics.go` (COUNT(1) query)
- Implementados 3 handlers REST con patron closure: `MetricsListHandler`, `ReadingsHandler`, `IngestHandler`
- Registradas 3 rutas en `NewRouter`: GET sin auth, POST con BearerAuth
- 13 tests httptest cubriendo todos los casos especificados en el plan (happy path, 400/401/404, edge cases)
- Proyecto compila sin errores (`go build ./...`), suite completa pasa (`go test ./... -count=1`)

## Task Commits

Cada tarea fue commiteada atomicamente (TDD: RED primero, luego GREEN):

1. **Task 1 RED: Tests failing (handlers no definidos)** - `f7bd654` (test)
2. **Task 1+2 GREEN: MetricExists + 3 handlers + server routes** - `571fd9c` (feat)

_Nota: Las tasks 1 y 2 se consolidaron en un solo RED commit (tests) + GREEN commit (implementacion) porque ambas comparten el mismo ciclo TDD._

## Files Created/Modified

- `internal/api/handlers_metrics_test.go` — 13 tests httptest: TestMetricsListHandler_OK/ThresholdsOmitted/Empty, TestReadingsHandler_OK/DefaultSince/CustomSince/InvalidSince/ExceedsMax/NotFound, TestIngestHandler_Success/Unauthorized/NotFound/InvalidBody/MissingValue/MissingName
- `internal/db/metrics.go` — agregado `func MetricExists(ctx, db, name) (bool, error)`
- `internal/api/handlers.go` — agregados tipos ThresholdInfo, MetricResponse, ReadingItem, ReadingsListResponse, IngestRequest, IngestResponse y 3 handlers
- `internal/api/server.go` — 3 rutas registradas en NewRouter

## Decisions Made

- **Thresholds desde config, no DB**: Los thresholds se obtienen del `cfg.Metrics.Definitions` en memoria construyendo un `map[string]*config.Thresholds`. Nunca se persisten en la DB. Consistente con D-01.
- **POST status hardcodeado "ok"**: `IngestHandler` llama `db.UpdateMetricStatus(..., "ok")`. Solo el evaluator evalua thresholds (D-10). Esto evita que ingesta manual interfiera con la state machine de alertas.
- **chi.URLParam en tests**: Los tests de ReadingsHandler usan el router completo via `NewRouter` para que `chi.URLParam` funcione correctamente desde el context del router chi.

## Deviations from Plan

None — plan ejecutado exactamente como especificado.

## Issues Encountered

- La worktree estaba en el estado de la rama v1.1 (commit `7fb46a9`), antes de que los cambios de Phase 8 y 9 fueran mergeados a master. Se hizo `git merge master` antes de ejecutar para traer `internal/db/metrics.go`, `internal/collector/`, `internal/config/config.go` con `MetricsConfig` y `ParseDuration`. Sin este merge, la compilacion hubiera fallado.

## Next Phase Readiness

- Los 3 endpoints REST estan listos para ser consumidos por Phase 10-02 (CLI commands) y Phase 11 (dashboard)
- El endpoint GET /api/v1/metrics incluye thresholds del config — el dashboard puede mostrar barras de warning/critical sin queries adicionales
- POST /api/v1/metrics es idempotente respecto a readings (no deduplica), el evaluator mantiene la state machine separada

## Self-Check: PASSED

- FOUND: internal/api/handlers_metrics_test.go
- FOUND: internal/db/metrics.go (with MetricExists)
- FOUND: internal/api/handlers.go (with MetricsListHandler, ReadingsHandler, IngestHandler)
- FOUND: internal/api/server.go (with /api/v1/metrics routes)
- FOUND: commit f7bd654 (RED — failing tests)
- FOUND: commit 571fd9c (GREEN — implementation)
- PASS: go test ./internal/api/... -count=1

---
*Phase: 10-rest-api-y-cli*
*Completed: 2026-03-27*
