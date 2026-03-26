---
phase: 08-config-schema-y-crud
plan: 01
subsystem: config
tags: [go, yaml, config, metrics, validation, duration-parsing]

requires:
  - phase: 07-verificacion-e-integracion
    provides: v1.1 milestone complete — config.go and config_test.go stable base

provides:
  - MetricsConfig, MetricDef, Thresholds Go types with yaml tags
  - parseDuration() with custom 'd' (days) unit support
  - ParseDuration() exported for Phase 9+ collector use
  - MetricsConfig.validate() with duplicate name, empty field, and threshold checks
  - Config.Validate() extended to validate metrics section if present
  - config.example.yaml with commented metrics section and 5 predefined metrics

affects:
  - 08-02 (schema/CRUD — depends on MetricDef types for UpsertMetric params)
  - 09 (collector — uses ParseDuration to convert interval strings)
  - 10 (API — uses MetricRow from db layer built in 08-02)

tech-stack:
  added: []
  patterns:
    - "parseDuration: custom duration parser handles 'd' suffix before delegating to time.ParseDuration"
    - "Pointer fields for optional YAML sections: *MetricsConfig nil when absent (retrocompat D-04)"
    - "TDD: failing tests written first, implementation added to make them pass"

key-files:
  created: []
  modified:
    - internal/config/config.go
    - internal/config/config_test.go
    - configs/config.example.yaml

key-decisions:
  - "MetricsConfig uses pointer (*MetricsConfig) so configs without metrics section parse to nil — backward compatible with v1.0/v1.1 configs (D-04)"
  - "parseDuration not exported; ParseDuration (uppercase) exported for Phase 9+ use"
  - "Default values for category='custom' and type='gauge' applied at runtime, not in unmarshal (D-12)"
  - "5 predefined metrics in config.example.yaml are fully commented — operators uncomment to enable (D-02)"

patterns-established:
  - "Pattern: optional config section as pointer — nil means disabled, non-nil triggers validation"
  - "Pattern: custom duration parser intercepts 'd' suffix, delegates remainder to stdlib"

requirements-completed:
  - STOR-03
  - MCOL-03
  - ALRT-01

duration: 12min
completed: 2026-03-26
---

# Phase 08 Plan 01: Config Schema y CRUD Summary

**MetricsConfig, MetricDef, Thresholds structs added to config.go with parseDuration custom parser ('d' unit), strict validation, and config.example.yaml with 5 predefined metrics**

## Performance

- **Duration:** ~12 min
- **Started:** 2026-03-26T00:00:00Z
- **Completed:** 2026-03-26
- **Tasks:** 2
- **Files modified:** 3

## Accomplishments

- Added MetricsConfig, MetricDef, Thresholds types to internal/config/config.go
- Implemented parseDuration() with 'd' (days) support and ParseDuration() exported wrapper
- Extended Config.Validate() to validate metrics section if present, ignore if absent (retrocompat)
- Added 17 new tests covering all behaviors including TDD-first red→green cycle
- Extended configs/config.example.yaml with commented metrics section and 5 predefined metrics

## Task Commits

Each task was committed atomically:

1. **Task 1: Agregar MetricsConfig, MetricDef, Thresholds, parseDuration y validacion a config.go** - `359db54` (feat)
2. **Task 2: Extender config.example.yaml con seccion metrics y las 5 metricas predefinidas** - `2e57b41` (chore)

## Files Created/Modified

- `internal/config/config.go` - Added MetricsConfig, MetricDef, Thresholds structs; parseDuration/ParseDuration functions; MetricsConfig.validate(); Metrics field in Config struct; extended Validate()
- `internal/config/config_test.go` - Added 17 new tests: parseDuration (6 tests), metrics load (2 tests), metrics validation (9 tests)
- `configs/config.example.yaml` - Added commented metrics section with 5 predefined metrics (disk_root, ram_used, cpu_load, docker_running, uptime_days)

## Decisions Made

- Followed plan decisions D-01 to D-04, D-11, D-12 exactly as specified in CONTEXT.md
- parseDuration handles empty string as error (test TestParseDuration_EmptyString) — Go stdlib time.ParseDuration returns (0, nil) for empty string, which would silently pass; fixed to return error
- Helper functions containsStr/containsSubstr added to test file to avoid importing strings package in test (avoids import cycle in package config)

## Deviations from Plan

None — plan executed exactly as written.

## Issues Encountered

None. The TDD cycle was clean: tests failed to compile (undefined parseDuration, undefined Metrics field) in RED phase, then passed after implementation in GREEN phase.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- MetricsConfig, MetricDef, Thresholds types ready for Phase 08-02 (schema migration + CRUD)
- ParseDuration exported and available for Phase 09 collector
- All config tests pass (32 total including 17 new)
- Build compiles cleanly

---
*Phase: 08-config-schema-y-crud*
*Completed: 2026-03-26*
