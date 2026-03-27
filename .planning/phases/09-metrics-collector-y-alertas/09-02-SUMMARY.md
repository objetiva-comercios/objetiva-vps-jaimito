---
phase: 09-metrics-collector-y-alertas
plan: 02
subsystem: collector
tags: [go, sqlite, state-machine, alertas, threshold, cooldown, telegram]

# Dependency graph
requires:
  - phase: 09-01
    provides: collector.go (Start, runMetricLoop, collectAndPersist), runner.go, collector_test.go
  - phase: 08-config-schema-y-crud
    provides: MetricsConfig, Thresholds, MetricDef, ParseDuration, ListMetrics, UpdateMetricStatus

provides:
  - internal/collector/evaluator.go — metricState, evaluateLevel, shouldAlert, hydrateStates, levelToPriority, sendAlert
  - internal/collector/collector.go — Start() con rehidratacion + collectAndEvaluate() con state machine
  - internal/telegram/format.go — priority "critical" con emoji rojo
  - cmd/jaimito/serve.go — collector.Start() integrado en startup sequence

affects:
  - dispatcher: mensajes de alerta encolados al canal "general" por sendAlert
  - telegram: priority "critical" ahora tiene emoji rojo en FormatMessage

# Tech tracking
tech-stack:
  added: []
  patterns:
    - state-machine por metrica: ok/warning/critical con cooldown configurable
    - rehidratacion de estado al arrancar (hydrateStates via ListMetrics)
    - mutex por instancia de metricState para concurrencia segura
    - shouldAlert: transicion + cooldown antes de enviar alerta
    - sendAlert: EnqueueMessage al canal "general" con uuid + hostname

key-files:
  created:
    - internal/collector/evaluator.go
  modified:
    - internal/collector/collector.go
    - internal/collector/collector_test.go
    - internal/telegram/format.go
    - cmd/jaimito/serve.go

key-decisions:
  - "fromLevel se lee desde state.currentLevel en collectAndEvaluate antes de transition() — correcto en goroutine unica por metrica"
  - "shouldAlert cooldown check: !s.lastAlert.IsZero() permite primera alerta siempre (zero time = nunca alerto)"
  - "Tests de integracion usan title LIKE (no body LIKE) porque el nombre de metrica va en el titulo del mensaje"
  - "Merge de master al inicio del plan para obtener los commits de Plan 01"

patterns-established:
  - "evaluator.go: state machine separado de collector.go para mantener responsabilidades claras"
  - "recovery alert: newLevel==ok y currentLevel!=ok siempre alerta (ignora cooldown)"
  - "escalation: warning->critical siempre alerta si cooldown expirado"

requirements-completed: [ALRT-02, ALRT-03, ALRT-04]

# Metrics
duration: ~10min
completed: 2026-03-27
---

# Phase 09 Plan 02: Alert State Machine Summary

**State machine de alertas (ok/warning/critical) con cooldown configurable, rehidratacion desde DB al arrancar, y dispatcher integrado en serve.go**

## Performance

- **Duration:** ~10 min
- **Started:** 2026-03-27T02:00:00Z
- **Completed:** 2026-03-27T02:10:00Z
- **Tasks:** 2 (TDD task 1 + integration task 2)
- **Files modified:** 5

## Accomplishments

- `evaluator.go`: metricState (sync.Mutex + currentLevel + lastAlert), evaluateLevel contra Thresholds, shouldAlert con cooldown, hydrateStates desde DB, levelToPriority, sendAlert via db.EnqueueMessage con uuid + hostname
- `collector.go`: Start() rehidrata estados, collectAndPersist renombrado a collectAndEvaluate con firma extendida (*metricState), evaluacion de umbrales y alertas en cada poll
- `format.go`: "critical" agregado al map priorityEmoji con emoji rojo (mismo que "high")
- `serve.go`: collector.Start(ctx, database, cfg.Metrics) integrado en startup si cfg.Metrics != nil
- 29 tests pasan (13 de Plan 01 actualizados + 16 nuevos de Plan 02)
- `go build ./...` y `go test ./...` exitosos, sin regresiones

## Task Commits

1. **Task 1 RED — failing tests evaluator** - `c0d21ae` (test)
2. **Task 1 GREEN — evaluator.go + format.go** - `fdf7532` (feat)
3. **Task 2 GREEN — collector.go + serve.go + tests integracion** - `0a40b62` (feat)

## Files Created/Modified

- `internal/collector/evaluator.go` — state machine completa: metricState, evaluateLevel, shouldAlert, hydrateStates, levelToPriority, sendAlert
- `internal/collector/collector.go` — Start() con hydrateStates + goroutines con *metricState, collectAndEvaluate con pipeline eval+alert
- `internal/collector/collector_test.go` — 16 tests nuevos: 5 evaluateLevel, 6 shouldAlert, 1 hydrateStates, 1 levelToPriority, 1 sendAlert, 2 collectAndEvaluate integración
- `internal/telegram/format.go` — "critical": "🔴" en priorityEmoji
- `cmd/jaimito/serve.go` — import collector + paso 12 (collector.Start si cfg.Metrics != nil)

## Decisions Made

- `fromLevel` se captura desde `state.currentLevel` sin lock antes de `transition()` — correcto porque cada metrica tiene su propia goroutine (no hay contention real entre shouldAlert y la lectura siguiente)
- `shouldAlert` cooldown: `!s.lastAlert.IsZero()` garantiza que la primera alerta siempre se envía (zero time.Time nunca cumple el cooldown)
- Recovery (cualquier nivel -> ok) siempre alerta sin importar el cooldown — comportamiento intencional para avisar de recuperacion
- Tests de integración buscan por `title LIKE '%metric_name%'` en lugar de `body LIKE` porque el nombre de la métrica aparece en el título del mensaje, no en el body

## Deviations from Plan

**1. [Rule 3 - Blocking] Merge de master necesario para obtener Plan 01**
- **Found during:** Inicio de ejecucion
- **Issue:** El worktree estaba sin los commits de Plan 01 (evaluator.go, runner.go, collector.go del plan anterior)
- **Fix:** `git merge master --no-edit` — fast-forward limpio con 29 archivos
- **Committed in:** fast-forward merge automatico

**2. [Rule 1 - Bug] Test query usaba body LIKE en vez de title LIKE**
- **Found during:** Task 2 GREEN, ejecucion de tests
- **Issue:** `TestCollectAndEvaluate_AlertOnTransition` y `NoDoubleAlert` buscaban el nombre de la metrica en el campo `body` de messages, pero `sendAlert` lo pone en el campo `title`
- **Fix:** Cambiar query de `body LIKE '%test_alert%'` a `title LIKE '%test_alert%'`
- **Files modified:** internal/collector/collector_test.go
- **Committed in:** 0a40b62

---

**Total deviations:** 2 (1 blocking merge, 1 bug en test query)
**Impact on plan:** Ninguno — correccion inline, sin cambios de arquitectura

## Known Stubs

None — todas las funciones implementadas con logica real, sin placeholders ni TODOs pendientes.

## Self-Check: PASSED

- FOUND: internal/collector/evaluator.go
- FOUND: internal/collector/collector.go (updated)
- FOUND: internal/collector/collector_test.go (updated)
- FOUND: internal/telegram/format.go (updated)
- FOUND: cmd/jaimito/serve.go (updated)
- FOUND: commit c0d21ae (test RED evaluator)
- FOUND: commit fdf7532 (feat evaluator.go)
- FOUND: commit 0a40b62 (feat collector + serve integration)

---
*Phase: 09-metrics-collector-y-alertas*
*Completed: 2026-03-27*
