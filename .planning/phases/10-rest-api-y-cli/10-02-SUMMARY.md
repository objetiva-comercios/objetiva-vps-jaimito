---
phase: 10-rest-api-y-cli
plan: 02
subsystem: cli
tags: [go, cobra, client, cli, metrics, tabwriter, isatty]

requires:
  - phase: 10-01
    provides: GET /api/v1/metrics, POST /api/v1/metrics, MetricResponse types

provides:
  - "client.GetMetrics() — HTTP GET /api/v1/metrics without auth, returns []MetricRow"
  - "client.PostMetric() — HTTP POST /api/v1/metrics with Bearer auth, returns *PostMetricResponse"
  - "jaimito status — tabla de metricas con NAME/VALUE/STATUS/UPDATED y emojis de estado"
  - "jaimito metric -n name --value X — ingesta manual de metrica con auth"

affects:
  - phase: 11 (dashboard que consumirá GET /api/v1/metrics; client ya expuesto)

tech-stack:
  added: []
  patterns:
    - "client methods siguen patron de Notify(): context, io.ReadAll, json.Unmarshal, errorResponse"
    - "status.go usa text/tabwriter stdlib para tabla alineada (lipgloss/v2 API no verificada)"
    - "ANSI colors condicionales via isatty.IsTerminal — no se colorea en pipes"
    - "metric.go sigue patron exacto de send.go: flags en init(), MarkFlagRequired, resolveAPIKey/resolveServer"

key-files:
  created:
    - "internal/client/client.go — extendido con MetricRow, Thresholds, PostMetricRequest, PostMetricResponse, GetMetrics(), PostMetric()"
    - "cmd/jaimito/status.go — cobra command 'status' con tabla formateada y colores TTY"
    - "cmd/jaimito/metric.go — cobra command 'metric' con flags -n/--value requeridos"

key-decisions:
  - "GetMetrics sin Authorization header — endpoint público per D-06 (plan de investigacion)"
  - "PostMetric con Bearer auth — escritura de datos requiere autenticacion (D-11)"
  - "text/tabwriter en lugar de lipgloss/v2 — API de lipgloss v2 table no verificada en RESEARCH"
  - "MarkFlagRequired('name') y MarkFlagRequired('value') — cobra valida antes de RunE"

duration: ~2min
completed: 2026-03-27
---

# Phase 10 Plan 02: CLI de Metricas Summary

**Client HTTP extendido con GetMetrics/PostMetric y 2 comandos CLI: `jaimito status` (tabla de metricas) y `jaimito metric -n name --value X` (ingesta manual).**

## Performance

- **Duration:** ~2 min
- **Started:** 2026-03-27T12:33:34Z
- **Completed:** 2026-03-27T12:35:12Z
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Extendido `internal/client/client.go` con 4 nuevos tipos y 2 nuevos methods
- `GetMetrics()`: GET /api/v1/metrics sin Authorization header (endpoint publico)
- `PostMetric()`: POST /api/v1/metrics con Bearer auth
- `jaimito status`: tabla con tabwriter, emojis ✅/⚠️/🔴, colores ANSI condicionales por TTY, mensaje amigable si server no esta corriendo
- `jaimito metric`: flags -n/--name y --value marcados como requeridos, output `name = value (recorded at ts)`
- `go build ./...` y `go vet ./...` pasan sin errores
- `go run ./cmd/jaimito --help` muestra ambos comandos registrados

## Task Commits

1. **Task 1: client.go + status.go** — `aae83a9`
2. **Task 2: metric.go** — `7988ee7`

## Files Created/Modified

- `internal/client/client.go` — MetricRow, Thresholds, PostMetricRequest, PostMetricResponse, GetMetrics(), PostMetric()
- `cmd/jaimito/status.go` — comando `jaimito status` con renderMetricsTable (tabwriter + isatty)
- `cmd/jaimito/metric.go` — comando `jaimito metric` con MarkFlagRequired y PostMetric

## Decisions Made

- **tabwriter en lugar de lipgloss/v2**: RESEARCH indica que la API de lipgloss/v2 table no fue verificada. tabwriter es stdlib y garantiza correcta alineacion sin dependencias extra.
- **ANSI colors manuales**: Escape codes directos (\033[33m, \033[31m) en lugar de lipgloss para colores — minimal y sin overhead.
- **Error amigable en status**: Se detecta "connection refused" o "dial" en el mensaje de error y se reemplaza con "server not reachable at X — is jaimito running?"

## Deviations from Plan

None — plan ejecutado exactamente como especificado.

## Self-Check: PASSED

- FOUND: internal/client/client.go (with GetMetrics, PostMetric, MetricRow)
- FOUND: cmd/jaimito/status.go (with statusCmd, renderMetricsTable, isatty detection)
- FOUND: cmd/jaimito/metric.go (with metricCmd, MarkFlagRequired)
- FOUND: commit aae83a9
- FOUND: commit 7988ee7
- PASS: go build ./... (no errors)
- PASS: go vet ./... (no warnings)
- PASS: go run ./cmd/jaimito --help | grep -E "status|metric" (both commands listed)

---
*Phase: 10-rest-api-y-cli*
*Completed: 2026-03-27*
