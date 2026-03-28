---
phase: 11-dashboard-web-embedido
plan: 01
subsystem: ui
tags: [go, embed, alpine, uplot, tailwind, dashboard, html, chi]

# Dependency graph
requires:
  - phase: 10-rest-api-y-cli
    provides: GET /api/v1/metrics and GET /api/v1/metrics/{name}/readings endpoints the dashboard will consume
provides:
  - internal/web package with DashboardHandler (go:embed + hostname injection)
  - GET /dashboard route registered in chi router (unauthenticated)
  - Alpine.js 3.15.9 and uPlot 1.6.32 vendored locally
  - Tailwind CLI build script for CSS compilation
  - Wave 0 test stubs (TestDashboardHandler, TestEmbedAssets, TestHostnameInjection)
affects:
  - 11-02 (Plan 02 replaces index.html placeholder with full dashboard HTML)

# Tech tracking
tech-stack:
  added:
    - embed (Go stdlib) — go:embed directive for single-file HTML embedding
    - sync.OnceValue (Go 1.21+) — hostname caching at first call
    - Alpine.js 3.15.9 (vendor) — reactive JS for dashboard interactivity
    - uPlot 1.6.32 (vendor) — lightweight time-series charting
    - Tailwind CLI standalone — CSS compilation without Node.js
  patterns:
    - DashboardHandler as closure: reads embedded HTML once at creation, injects hostname per request via bytes.ReplaceAll
    - go:embed single file pattern: //go:embed index.html for minimal embed surface
    - Vendor libs in internal/web/vendor/ for inline use in Plan 02 HTML
    - Build script pattern: scripts/build-dashboard.sh downloads tool if missing, then runs it

key-files:
  created:
    - internal/web/embed.go
    - internal/web/index.html
    - internal/web/dashboard_test.go
    - internal/web/vendor/alpine.min.js
    - internal/web/vendor/uplot.min.js
    - internal/web/vendor/uplot.min.css
    - scripts/build-dashboard.sh
  modified:
    - internal/api/server.go
    - .gitignore

key-decisions:
  - "DashboardHandler reads embedded HTML once at handler creation (not per-request) for performance"
  - "sync.OnceValue for hostname caching — avoids repeated syscalls per request"
  - "GET /dashboard registered as unauthenticated route, consistent with HealthHandler pattern (D-14)"
  - "Vendor libs downloaded to internal/web/vendor/ for inline use in Plan 02 — not go:embed'd directly"
  - "tools/ added to .gitignore — Tailwind CLI binary is ~17MB, not committed"

patterns-established:
  - "internal/web package: go:embed + handler pattern for serving embedded static assets"
  - "Wave 0 test stubs: tests created before implementation, pass immediately once implementation lands"

requirements-completed: [DASH-01, DASH-05]

# Metrics
duration: 8min
completed: 2026-03-28
---

# Phase 11 Plan 01: Infraestructura Go para Dashboard Web Embedido Summary

**Paquete internal/web con go:embed + DashboardHandler (hostname injection via sync.OnceValue), GET /dashboard en chi router, Alpine.js 3.15.9 + uPlot 1.6.32 vendoreados, y build script de Tailwind CSS**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-28T00:00:00Z
- **Completed:** 2026-03-28T00:08:00Z
- **Tasks:** 3 (Task 0 + Task 1 + Task 2)
- **Files modified:** 9

## Accomplishments

- Paquete internal/web creado con embed.go (go:embed index.html, DashboardHandler, hostname injection via sync.OnceValue + bytes.ReplaceAll)
- GET /dashboard registrado en chi router como ruta no autenticada, siguiendo el patron de HealthHandler
- Alpine.js 3.15.9 (46KB) y uPlot 1.6.32 JS+CSS vendoreados en internal/web/vendor/
- Build script scripts/build-dashboard.sh para compilar CSS con Tailwind CLI standalone (sin Node.js)
- 3 tests Wave 0 creados y pasando: TestDashboardHandler, TestEmbedAssets, TestHostnameInjection
- go build ./... compila sin errores con el placeholder HTML

## Task Commits

1. **Task 0: Wave 0 test stubs** - `4547423` (test)
2. **Task 1: embed.go + index.html placeholder** - `3051711` (feat)
3. **Task 2: server.go route + vendors + build script** - `40027e2` (feat)

## Files Created/Modified

- `internal/web/embed.go` - Paquete web: go:embed index.html, DashboardHandler con hostname injection
- `internal/web/index.html` - Placeholder HTML con {{HOSTNAME}} para compilacion con go:embed
- `internal/web/dashboard_test.go` - Tests Wave 0: TestDashboardHandler, TestEmbedAssets, TestHostnameInjection
- `internal/web/vendor/alpine.min.js` - Alpine.js 3.15.9 CDN vendoreado (46KB)
- `internal/web/vendor/uplot.min.js` - uPlot 1.6.32 IIFE vendoreado (51KB)
- `internal/web/vendor/uplot.min.css` - uPlot 1.6.32 CSS vendoreado (1.8KB)
- `scripts/build-dashboard.sh` - Script Tailwind CLI: descarga binario si no existe, compila CSS
- `internal/api/server.go` - Agregado import web + r.Get("/dashboard", web.DashboardHandler())
- `.gitignore` - Agregado tools/ para excluir binario Tailwind CLI (~17MB)

## Decisions Made

- DashboardHandler lee el HTML embedido una sola vez en la creacion del handler (no por request) — performance
- sync.OnceValue para hostname caching — evita syscalls repetidos
- GET /dashboard sin autenticacion — consistente con HealthHandler (D-14 del CONTEXT.md)
- Vendor libs en internal/web/vendor/ para uso inline en Plan 02 — no se embeden directamente aun
- tools/ en .gitignore — el binario de Tailwind CLI es ~17MB, no debe commitearse

## Deviations from Plan

None - plan executed exactly as written.

## Issues Encountered

None.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Plan 02 puede crear el HTML completo del dashboard sin preocuparse por la infraestructura Go
- Los vendors (Alpine.js, uPlot) estan listos para inlinear en el HTML final
- El build script de Tailwind esta listo para compilar CSS una vez que Plan 02 defina las clases usadas
- GET /dashboard ya responde HTTP 200 con el placeholder HTML (hostname inyectado)

---
*Phase: 11-dashboard-web-embedido*
*Completed: 2026-03-28*
