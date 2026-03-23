# Project Retrospective

*A living document updated after each milestone. Lessons feed forward into future planning.*

## Milestone: v1.0 — MVP

**Shipped:** 2026-03-23
**Phases:** 3 | **Plans:** 10

### What Was Built
- Go binary con config YAML, SQLite WAL-mode persistence, y systemd unit
- HTTP API (`POST /api/v1/notify`) con Bearer token auth y health endpoint
- Telegram dispatcher con MarkdownV2, exponential backoff, y 429 handling
- CLI con cobra: `send`, `wrap`, y `keys` subcomandos
- `jaimito wrap` para monitoreo de cron jobs — el killer feature del MVP

### What Worked
- Fases alineadas al flujo de datos natural (foundation → pipeline → CLI) eliminaron rework
- Investigación pre-fase (CONTEXT.md + RESEARCH.md) evitó problemas de API descubiertos tarde
- modernc.org/sqlite CGO-free simplificó el build pipeline significativamente
- Separación API/dispatcher (enqueue to DB vs read independently) mantuvo boundaries limpios
- Velocidad de ejecución aceleró con cada fase (5 min/plan → 1.3 min/plan)

### What Was Inefficient
- Audit del milestone se hizo antes de completar Phase 3 — requirió re-evaluación post-completion
- El campo `one_liner` en SUMMARY.md no se completó en ningún summary — dificultó extracción automática de accomplishments

### Patterns Established
- Single-writer SQLite con SetMaxOpenConns(1) como patrón estándar
- HashToken compartido entre seed y auth para prevenir divergencia de hash
- os.Exit(exitCode) en CLI wrapping para preservar transparencia de exit codes
- Best-effort notification: fallo en notify no altera el exit code del comando wrapeado

### Key Lessons
1. Investigar APIs de terceros (go-telegram/bot, modernc.org/sqlite) antes de planificar evita retrabajos por DSN format differences, type mismatches, etc.
2. Separar capas por responsabilidad (API enqueue → DB → dispatcher read) hace testing y debugging trivial
3. El patrón startup-then-interval para schedulers (cleanup, dispatcher) asegura consistencia desde el primer segundo

### Cost Observations
- Model mix: balanced profile (sonnet executor, opus planner)
- Execution time: ~27 min total across 10 plans
- Notable: Velocidad aceleró 4x entre Phase 1 (5 min/plan) y Phase 3 (1.3 min/plan) — contexto acumulado redujo overhead

---

## Cross-Milestone Trends

### Process Evolution

| Milestone | Phases | Plans | Key Change |
|-----------|--------|-------|------------|
| v1.0 | 3 | 10 | Initial milestone — established research-before-plan pattern |

### Cumulative Quality

| Milestone | Tests | LOC | Files |
|-----------|-------|-----|-------|
| v1.0 | 16 | 2,090 Go | 62 |

### Top Lessons (Verified Across Milestones)

1. Research third-party APIs before planning — confirmed across all 3 phases
2. Data-flow-aligned phase ordering eliminates rework — foundation → pipeline → CLI
