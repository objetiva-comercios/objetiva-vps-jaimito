---
phase: 11-dashboard-web-embedido
plan: 02
subsystem: ui
tags: [go, alpine, uplot, tailwind, dashboard, html, sparklines, accordion]

# Dependency graph
requires:
  - phase: 11-01
    provides: internal/web package, DashboardHandler, vendor libs (Alpine.js, uPlot), build script
  - phase: 10-rest-api-y-cli
    provides: GET /api/v1/metrics and GET /api/v1/metrics/{name}/readings endpoints
provides:
  - internal/web/index.html completo (~127KB autocontenido, zero CDN)
  - internal/web/build/output.css Tailwind CSS pre-compilado (~11.7KB minificado)
  - Dashboard operacional en GET /dashboard con tabla de metricas, sparklines SVG, graficos uPlot expandibles, auto-refresh 30s, hostname VPS, estilo terminal dark
affects:
  - Phase 12 (Cleanup y Polish): dashboard ya funcional, no requiere cambios estructurales

# Tech tracking
tech-stack:
  added:
    - Alpine.js 3.15.9 (inline ~46KB) — componente x-data="dashboard" con polling, accordion, sparkData acumulador
    - uPlot 1.6.32 (inline ~51KB) — graficos time-series con threshold plugin personalizado
    - Tailwind CSS v4 (compilado ~11.7KB) — clases utilitarias compiladas y embedidas inline
    - makeThresholdPlugin — plugin uPlot custom que dibuja lineas horizontales warning/critical con canvas draw hook
    - SVG polyline sparklines — generados desde sparkData acumulado via polling, normalizados a viewBox 80x24
  patterns:
    - Alpine.js init/destroy lifecycle: clearInterval + uPlot.destroy() en x-effect para cleanup correcto
    - sparkData accumulator: acumula hasta 20 ultimos last_value por metrica via polling cada 30s
    - Accordion one-at-a-time: openMetric null/name toggle + $nextTick para renderChart post-DOM-update
    - Tailwind v4 auto-detection: @import "tailwindcss" + @source explicit en input.css para escanear index.html
    - Placeholder contract: /* TAILWIND_CSS */ en <style> block, reemplazado por sed en build script

key-files:
  created:
    - internal/web/build/output.css
  modified:
    - internal/web/index.html

key-decisions:
  - "sparkData acumulado via polling (no readings endpoint para sparklines) — GET /api/v1/metrics solo retorna last_value, sparklines se forman tras varias iteraciones de 30s"
  - "makeThresholdPlugin como funcion global fuera de Alpine — reutilizable, separacion de responsabilidades"
  - "Tailwind v4 build con @source explicit — Tailwind v4 CLI requiere @source para escanear HTML fuera del directorio de input CSS"
  - "CSS compilado inline en <style> junto a uPlot CSS — zero requests externos, cumple DASH-05"
  - "Human verification checkpoint aprobado — dashboard renderiza correctamente en browser"

# Metrics
duration: 125min
completed: 2026-03-28
---

# Phase 11 Plan 02: HTML Completo del Dashboard Web Embedido Summary

**Dashboard autocontenido (~127KB) con Alpine.js polling/accordion/sparklines, uPlot graficos time-series con threshold lines, Tailwind CSS pre-compilado inline — zero CDN, zero conexion a internet en runtime**

## Performance

- **Duration:** ~125 min (incluyendo checkpoint de verificacion visual)
- **Completed:** 2026-03-28
- **Tasks:** 3 (Task 1 auto + Task 2 auto + Task 3 checkpoint:human-verify)
- **Files modified:** 2

## Accomplishments

- Dashboard HTML completo (~127KB) creado en internal/web/index.html con Alpine.js 3.15.9 y uPlot 1.6.32 inline (zero CDN)
- Componente Alpine.js x-data="dashboard" con: polling fetchMetrics() cada 30s, accordion toggleMetric() (un solo grafico expandido a la vez), sparkData acumulador (ultimos 20 valores por metrica), sparklinePoints() normalizado a viewBox 80x24
- makeThresholdPlugin — plugin uPlot con hook draw que dibuja lineas horizontales dashed para umbrales warning (#eab308) y critical (#ef4444)
- Tabla de metricas: nombre, valor con timestamp, sparkline SVG, dot de estado (ok=green/warning=yellow/critical=red con animate-pulse)
- Header con hostname {{HOSTNAME}} (inyectado por DashboardHandler), timestamp de ultima actualizacion con toLocaleTimeString
- Lucide icons inline: server, refresh-cw (animate-spin cuando loading), alert-circle, bar-chart-2
- Error banner, loading skeleton (animate-pulse), empty state con iconografia
- Tailwind CSS v4 compilado (11.7KB) e insertado inline via script build-dashboard.sh — placeholder /* TAILWIND_CSS */ reemplazado por sed
- go build ./... compila sin errores con el HTML final embedido (~127KB)
- Verificacion visual aprobada por el usuario

## Task Commits

1. **Task 1: Crear index.html completo del dashboard** - `8887b2a` (feat)
2. **Task 2: Compilar Tailwind CSS e insertar inline** - `b4efc32` (feat)
3. **Task 3: Verificacion visual del dashboard** - Checkpoint:human-verify (APROBADO por usuario)

## Files Created/Modified

- `internal/web/index.html` — Dashboard HTML completo: Alpine.js (polling + accordion + sparklines), uPlot (charts + threshold plugin), Tailwind CSS inline, Lucide SVG icons, todos los vendors inline (~127KB)
- `internal/web/build/output.css` — CSS Tailwind compilado con clases usadas (~11.7KB, luego insertado inline)

## Success Criteria Verification

- [x] Dashboard renderiza tabla de metricas con nombre, valor, sparkline SVG e indicador de estado coloreado (DASH-02) — verificado visualmente
- [x] Click en fila expande grafico uPlot con historial; segundo click colapsa (DASH-03) — accordion con openMetric state
- [x] Solo un grafico expandido a la vez (D-07) — toggleMetric() destruye instancia anterior antes de crear nueva
- [x] Datos se actualizan cada 30s sin page reload (DASH-04) — setInterval(fetchMetrics, 30000) en init()
- [x] Tailwind CSS pre-compilado, Alpine.js, uPlot, Lucide icons — todo inline (DASH-05) — zero CDN references confirmado
- [x] Header muestra hostname VPS + timestamp ultima actualizacion (DASH-06) — {{HOSTNAME}} + lastUpdated
- [x] Lineas de umbral warning/critical en graficos (D-08) — makeThresholdPlugin
- [x] Zero dependencias externas en runtime — funciona sin internet — verificado

## Decisions Made

- sparkData acumulado via polling: el endpoint GET /api/v1/metrics solo retorna last_value (no array de readings), por lo que los sparklines se construyen acumulando valores conforme llegan cada 30s — los primeros ~20 polls son necesarios para formar sparklines visibles
- makeThresholdPlugin como funcion global fuera del componente Alpine — patron correcto para plugins uPlot reutilizables
- Tailwind v4 requiere @source explicit en input.css para escanear archivos HTML fuera del directorio del CSS de entrada — adaptacion del build script necesaria
- CSS compilado inline junto a uPlot CSS en el mismo bloque `<style>` — un solo bloque, cero requests externos

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 3 - Build Blocker] Tailwind v4 no soporta --content flag**
- **Found during:** Task 2
- **Issue:** El script build-dashboard.sh original usaba el patron de Tailwind v3 (`--content` flag). Tailwind v4 CLI no soporta ese flag — usa auto-deteccion con `@source` en el CSS de entrada.
- **Fix:** Actualizado input.css a `@import "tailwindcss"; @source "../index.html";` y ajustado el comando de compilacion a `--input internal/web/build/input.css --output internal/web/build/output.css --minify`
- **Files modified:** internal/web/build/input.css (creado), scripts/build-dashboard.sh (actualizado)
- **Commit:** b4efc32

## Known Stubs

None — todos los datos provienen del API real (/api/v1/metrics). Los sparklines aparecen vacios en la primera carga (requieren polling acumulado), pero esto es comportamiento documentado y esperado — no un stub.

## Self-Check: PASSED

- [x] internal/web/index.html — FOUND
- [x] internal/web/build/output.css — FOUND
- [x] commit 8887b2a — FOUND
- [x] commit b4efc32 — FOUND
- [x] go build ./... — PASSED

---
*Phase: 11-dashboard-web-embedido*
*Plan: 02*
*Completed: 2026-03-28*
