---
phase: 09-metrics-collector-y-alertas
plan: 01
subsystem: collector
tags: [go, sqlite, goroutine, ticker, exec, shell, metrics]

# Dependency graph
requires:
  - phase: 08-config-schema-y-crud
    provides: MetricsConfig, MetricDef, ParseDuration, UpsertMetric, InsertReading, UpdateMetricStatus, ListMetrics, QueryReadings

provides:
  - internal/collector/runner.go — computeTimeout + runCommand (sh -c con timeout y WaitDelay)
  - internal/collector/collector.go — Start(), goroutine-per-metric con ticker independiente, collectAndPersist()
  - internal/collector/collector_test.go — 13 tests unitarios e integracion con SQLite in-memory

affects:
  - 09-02 (alertas): usa collector.go como base, agrega state machine ok/warning/critical
  - serve.go: agregar collector.Start() al startup

# Tech tracking
tech-stack:
  added: []
  patterns:
    - goroutine-per-metric con time.NewTicker independiente
    - startup-then-interval (collectAndPersist inmediato + ticker)
    - collect-then-write (runCommand -> UpdateMetricStatus -> InsertReading)
    - slog.Error en fallos de metrica individual sin detener otras goroutines

key-files:
  created:
    - internal/collector/runner.go
    - internal/collector/collector.go
    - internal/collector/collector_test.go
  modified: []

key-decisions:
  - "runCommand usa 'sh -c' (POSIX sh, no bash) para portabilidad en cualquier Linux"
  - "cmd.WaitDelay = 5s fuerza kill del proceso hijo despues del timeout del contexto"
  - "Status hardcodeado como 'ok' en collectAndPersist — state machine real viene en Plan 02"
  - "TDD estricto: RED (tests fallan) -> GREEN (implementacion) -> commit por cada ciclo"

patterns-established:
  - "runner: computeTimeout = min(80%*interval, 30s) — threshold fijo en 30s"
  - "collector: Start() fire-and-forget igual que dispatcher.Start() y cleanup.Start()"
  - "testDB helper: db.Open(':memory:') + db.ApplySchema() para tests de integracion"

requirements-completed: [MCOL-01, MCOL-02, MCOL-04, MCOL-05]

# Metrics
duration: 3min
completed: 2026-03-27
---

# Phase 09 Plan 01: Metrics Collector Summary

**Shell command executor con timeout/WaitDelay y goroutine-per-metric loop persistiendo lecturas a SQLite — incluyendo fallo silencioso de docker_running cuando Docker no esta instalado**

## Performance

- **Duration:** ~3 min
- **Started:** 2026-03-27T01:52:16Z
- **Completed:** 2026-03-27T01:55:28Z
- **Tasks:** 2 (TDD: 4 commits total)
- **Files modified:** 3

## Accomplishments

- `runner.go`: computeTimeout (min 80%*interval, 30s) + runCommand con exec.CommandContext("sh", "-c"), WaitDelay=5s, parse float64
- `collector.go`: Start() con UpsertMetric + goroutine-per-metric, runMetricLoop con startup-then-interval, collectAndPersist con slog.Error en fallos (MCOL-05)
- MCOL-02 cubierto: docker_running falla silenciosamente cuando Docker no esta instalado — runCommand retorna error, collectAndPersist loguea y el loop continua sin crash
- 13 tests pasan (6 runner + 7 collector incluyendo 2 de integracion con SQLite in-memory)
- `go build ./...` y `go test ./...` exitosos, sin regresiones

## Task Commits

Cada task fue commiteado atomicamente (TDD = test + feat):

1. **Task 1 RED — tests runner** - `a8ec4a1` (test)
2. **Task 1 GREEN — runner.go** - `de95805` (feat)
3. **Task 2 RED — tests collector** - `5f9074c` (test)
4. **Task 2 GREEN — collector.go** - `5f33e0e` (feat)

## Files Created/Modified

- `internal/collector/runner.go` — computeTimeout + runCommand con sh -c, timeout context, WaitDelay=5s
- `internal/collector/collector.go` — Start(), runMetricLoop(), collectAndPersist(), helpers resolveInterval/category/metricType
- `internal/collector/collector_test.go` — Tests unitarios (computeTimeout x4, runCommand x5) + integracion (resolveInterval x2, category x2, metricType x2, collectAndPersist, collectCommandFailure)

## Decisions Made

- Status hardcodeado como `"ok"` en collectAndPersist — la state machine real (ok/warning/critical con thresholds y cooldown) se implementa en Plan 02 de esta misma fase
- Worktree mergeado con master antes de ejecutar para traer los cambios de Phase 8 (MetricsConfig, db/metrics.go, migration 003)

## Deviations from Plan

**1. [Rule 3 - Blocking] Merge de master necesario para obtener Phase 8**
- **Found during:** Inicio de ejecucion
- **Issue:** El worktree estaba en una rama antigua sin los cambios de Phase 8 (MetricsConfig, db/metrics.go, 003_metrics.sql) necesarios para implementar el collector
- **Fix:** `git merge master --no-edit` en el worktree — fast-forward limpio
- **Files modified:** (25 archivos de Phase 8 traidos, sin conflictos)
- **Verification:** `go build ./...` exitoso despues del merge
- **Committed in:** merge commit integrado automaticamente

---

**Total deviations:** 1 auto-fixed (1 blocking — merge de dependencia faltante)
**Impact on plan:** Necesario para obtener las dependencias del Plan. Sin scope creep.

## Issues Encountered

None — plan ejecutado exitosamente siguiendo el spec exacto.

## Next Phase Readiness

- `collector.Start()` listo para integrarse en `serve.go` (agregar despues de `cleanup.Start()` si `cfg.Metrics != nil`)
- Plan 02 puede implementar la state machine de alertas sobre la base de este collector
- Todos los contratos de DB (UpsertMetric, InsertReading, UpdateMetricStatus) correctamente usados

---
*Phase: 09-metrics-collector-y-alertas*
*Completed: 2026-03-27*
