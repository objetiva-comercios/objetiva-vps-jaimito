---
phase: 08-config-schema-y-crud
plan: 02
subsystem: db
tags: [sqlite, migration, crud, metrics, tdd]
dependency_graph:
  requires: [internal/db/db.go, internal/db/schema/001_initial.sql]
  provides: [internal/db/schema/003_metrics.sql, internal/db/metrics.go]
  affects: [Phase 9 collector, Phase 10 API, Phase 11 dashboard, Phase 12 scheduler]
tech_stack:
  added: []
  patterns: [INSERT ON CONFLICT (upsert), sql.NullFloat64 for nullable REAL, RFC3339 strftime format, TDD red-green]
key_files:
  created:
    - internal/db/schema/003_metrics.sql
    - internal/db/metrics.go
    - internal/db/metrics_test.go
  modified:
    - internal/db/db_verify_test.go
decisions:
  - "strftime RFC3339 instead of datetime('now') for metrics tables — enables Go time.RFC3339 range queries"
  - "QueryReadings and ListMetrics return empty slice (not nil) — consistent caller contract"
  - "PurgeOldReadings uses time.Now().UTC().Add(-olderThan) cutoff — correct direction for purge"
metrics:
  duration: "132s"
  completed_date: "2026-03-26"
  tasks_completed: 2
  files_created: 3
  files_modified: 1
---

# Phase 08 Plan 02: Metrics Schema y CRUD — Summary

SQLite migration 003_metrics.sql con tablas `metrics` y `metric_readings`, e implementacion completa de 6 funciones CRUD con tipos exportados y 11 tests unitarios pasando.

## Tasks Completed

| Task | Description | Commit | Files |
|------|-------------|--------|-------|
| 1 | Migracion 003_metrics.sql + actualizar db_verify_test.go | 04ae554 | 003_metrics.sql, db_verify_test.go |
| 2 RED | Tests fallando para todas las funciones CRUD | 7060804 | metrics_test.go |
| 2 GREEN | Implementar metrics.go con 6 funciones CRUD | 410db83 | metrics.go |

## What Was Built

### Migration 003_metrics.sql

Dos tablas nuevas en SQLite:

- `metrics`: Estado actual de cada metrica (name PK, category, type, last_value REAL nullable, last_status, updated_at). Timestamps usan `strftime('%Y-%m-%dT%H:%M:%SZ', 'now')` para formato RFC3339 compatible con queries Go.
- `metric_readings`: Historial de readings (id autoincrement, metric_name FK a metrics, value REAL, recorded_at). Indice compuesto `idx_metric_readings_name_time` en (metric_name, recorded_at) para queries de rango eficientes.

### metrics.go — 6 funciones CRUD

- `UpsertMetric(ctx, db, name, category, type)` — idempotente via `ON CONFLICT DO UPDATE`
- `InsertReading(ctx, db, metricName, value)` — inserta reading, respeta FK constraint
- `QueryReadings(ctx, db, metricName, since)` — filtra por tiempo RFC3339, retorna ASC
- `ListMetrics(ctx, db)` — lista todas con `sql.NullFloat64` para last_value nullable
- `PurgeOldReadings(ctx, db, olderThan)` — elimina readings viejos, retorna count
- `UpdateMetricStatus(ctx, db, name, value, status)` — actualiza last_value y last_status

### Exported Types

- `MetricRow` — representa fila de tabla metrics con LastValue como `*float64`
- `ReadingRow` — representa fila de metric_readings

## Test Results

```
=== RUN   TestOpenAndApplySchema     PASS
=== RUN   TestUpsertMetric           PASS
=== RUN   TestInsertReading          PASS
=== RUN   TestInsertReading_NoMetric PASS
=== RUN   TestQueryReadings_FilterByTime  PASS
=== RUN   TestQueryReadings_Empty    PASS
=== RUN   TestQueryReadings_OrderByTime   PASS
=== RUN   TestListMetrics            PASS
=== RUN   TestListMetrics_Empty      PASS
=== RUN   TestPurgeOldReadings       PASS
=== RUN   TestPurgeOldReadings_Empty PASS
=== RUN   TestUpdateMetricStatus     PASS
14 tests PASS
```

## Deviations from Plan

None — plan executed exactly as written.

## Known Stubs

None — all functions are fully implemented and tested.

## Self-Check: PASSED

- `internal/db/schema/003_metrics.sql` — EXISTS
- `internal/db/metrics.go` — EXISTS
- `internal/db/metrics_test.go` — EXISTS
- Commit 04ae554 — FOUND
- Commit 7060804 — FOUND
- Commit 410db83 — FOUND
- `go test ./internal/db/... -count=1` — PASS (14 tests)
- `go build ./...` — PASS
