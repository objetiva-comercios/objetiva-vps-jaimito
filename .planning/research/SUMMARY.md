# Project Research Summary

**Project:** jaimito v2.0 — Metricas y Dashboard
**Domain:** Go metrics collector + embedded web dashboard added to existing notification hub
**Researched:** 2026-03-26
**Confidence:** HIGH

## Executive Summary

jaimito v2.0 extiende un Go binary ya existente y funcional (notificaciones Telegram + SQLite + CLI) con recoleccion de metricas de sistema y un dashboard web embebido. El enfoque correcto, validado por investigacion directa del codebase y analisis de herramientas comparables (Beszel, Netdata, Glances), es construir todo dentro del mismo binario sin introducir nuevos procesos, bases de datos externas, ni toolchains de frontend. La unica dependencia Go nueva es `gopsutil/v4` para metricas nativas; el frontend usa Alpine.js, uPlot y Tailwind CSS pre-compilado embebidos via `go:embed`.

La arquitectura se organiza en capas claramente separadas: un paquete `internal/metrics/` nuevo (goroutine de coleccion), una extension del paquete `internal/db/` existente (nuevo schema SQL + funciones CRUD), nuevas rutas en `internal/api/` (handlers de metricas + servido del dashboard), y activos estaticos embebidos en `internal/web/`. El dispatcher y el bot de Telegram no cambian — las alertas de umbral se enolan como mensajes ordinarios via `db.EnqueueMessage()`, reutilizando toda la infraestructura de entrega existente.

Los riesgos principales son de implementacion: inyeccion de comandos via config.yaml (mitigado con permisos 0600 y metricas predefinidas hardcodeadas en Go), goroutines colgadas por falta de timeout en `exec.CommandContext` (requiere timeout explicito + `cmd.WaitDelay`), contension en el pool SQLite de un solo escritor (mitigado separando ejecucion de comando de las escrituras a la DB), y errores de compilacion por rutas incorrectas en `go:embed`. Todos estos riesgos tienen patrones de prevencion documentados y bien conocidos.

---

## Key Findings

### Recommended Stack

El stack v2.0 agrega exactamente una dependencia Go nueva sobre el stack existente. No hay cambios de framework, no hay nuevo servidor web, no hay nueva base de datos. El dashboard completo vive como archivos estaticos embebidos (~30KB total gzipeado).

**Core technologies:**

- `gopsutil/v4@v4.26.2`: metricas de sistema (CPU, RAM, disco, uptime, Docker) — CGO-free, puro Go, soporta amd64 y arm64; elimina parsing de output de comandos shell para metricas predefinidas
- `Alpine.js v3.14.x` (~7.1KB gzipeado): reactividad del dashboard (tabla, toggle de chart, auto-refresh) — cero build toolchain en CI, funciona con un `<script>` tag
- `uPlot v1.6.32` (~15KB gzipeado): chart de series temporales — 5x mas liviano que Chart.js, renderiza 3600 puntos a 60fps, API orientada a time-series puro
- Tailwind CSS v4 pre-compilado (<10KB): generado una vez por el desarrollador con el CLI standalone (sin Node.js), commiteado al repo, embebido via `go:embed`
- `go:embed` (stdlib): embebe todo el directorio `web/static/` en el binario; sin nueva dependencia
- SQLite WAL via `modernc.org/sqlite` (ya existente): dos tablas nuevas (`metrics` + `metric_reads`) via migracion numerada `003_metrics.sql`

**Critico:** No usar Tailwind Play CDN (requiere red en runtime), no usar Chart.js (254KB vs 47.9KB de uPlot), no abrir una segunda conexion SQLite para lecturas del dashboard (el pool `SetMaxOpenConns(1)` es suficiente y correcto a escala VPS).

### Expected Features

**Must have (table stakes):**
- CPU, RAM, disco, uptime como metricas predefinidas con zero config
- Vista de tabla con valor actual, unidad, status (OK/WARN/CRIT) y ultima lectura
- Chart de series temporales expandible por click (uPlot, ultimas N lecturas)
- Alertas de umbral warning/critical a Telegram (estado maquina, no por cada lectura)
- Purga automatica de lecturas antiguas (7 dias por defecto)
- `jaimito status` CLI — salida tabular de valores actuales desde terminal
- `jaimito metric push` CLI — ingestion manual para scripts externos

**Should have (competitive):**
- Metricas custom via comandos shell en config.yaml — el diferenciador principal (ssl_expiry, pg_connections, queue_depth)
- Maquina de estados de alertas (NORMAL/WARNING/CRITICAL por metrica, transition-only) — evita alert storm
- REST API: `GET /api/v1/metrics`, `GET /api/v1/metrics/{name}/readings`, `POST /api/v1/metrics/ingest`
- Docker container count como metrica predefinida (graceful si Docker no esta instalado)

**Defer (v2.x):**
- Notificacion de recuperacion ("CPU volvio a normal")
- Retention window configurable en config.yaml (hoy hardcodeado 7 dias)
- Metricas Docker mas granulares (stats por container)

**Fuera de scope (v3+):**
- Downsampling/agregacion de metricas historicas
- Dashboard multi-VPS
- Canales de alerta adicionales (Slack, email)

### Architecture Approach

La arquitectura es una extension minima del binario existente. Un nuevo paquete `internal/metrics/collector.go` implementa una goroutine con ticker grueso (10s) que evalua que metricas vencieron su intervalo, ejecuta el comando shell via `exec.CommandContext` con timeout, y escribe el resultado en SQLite. Las alertas se generan encolando mensajes ordinarios via `db.EnqueueMessage()` — el dispatcher existente los entrega a Telegram sin ningun cambio. El dashboard es un `index.html` + `app.js` servido desde `internal/web/static/` via `http.FileServer(http.FS(web.StaticFS))` registrado como catch-all en el router chi existente.

**Major components:**
1. `internal/metrics/collector.go` (NUEVO) — goroutine de coleccion, evaluacion de umbrales, encolado de alertas
2. `internal/db/metrics.go` + `003_metrics.sql` (NUEVO) — schema y CRUD para `metrics` y `metric_reads`
3. `internal/api/handlers.go` (MODIFICADO) — handlers `MetricsHandler`, `ReadingsHandler`, `IngestHandler`
4. `internal/web/` (NUEVO) — `embed.go` + activos estaticos del dashboard
5. `cmd/jaimito/metric.go` + `status.go` (NUEVO) — subcomandos CLI

**Archivos sin cambios:** `dispatcher.go`, `telegram/`, `db/db.go`, `db/messages.go`, `api/middleware.go`, todos los subcomandos existentes.

### Critical Pitfalls

1. **Sin timeout en exec.CommandContext** (M2) — usar siempre `exec.CommandContext` con timeout = min(80% del intervalo, 30s) + `cmd.WaitDelay = 5s`. Un comando que cuelga bloquea la goroutine de coleccion indefinidamente y acumula goroutines zombies.

2. **Escritura a SQLite dentro de la ejecucion del comando** (M4) — jamas abrir una transaccion, ejecutar el shell command, y despues commitear. Patron correcto: ejecutar comando fuera de cualquier transaccion → guardar resultado → abrir TX → INSERT → commit inmediato.

3. **Rutas incorrectas en go:embed** (M5) — la directiva `//go:embed` solo acepta rutas relativas al archivo que la contiene, sin `..`. Crear `internal/web/embed.go` y colocar `static/` como subdirectorio de `internal/web/`.

4. **Tailwind Play CDN en produccion** (M8) — Pre-compilar Tailwind con el CLI standalone antes de `go build` y commitear el CSS resultante. El Play CDN requiere red en runtime y carga 300KB inutiles.

5. **go:embed excluye archivos que empiezan con `.` o `_`** (M6) — nombrar todos los activos sin punto ni guion bajo al inicio. Agregar test que verifica que `index.html` y `tailwind.css` estan presentes en el FS embebido.

6. **Orphaned child processes al matar sh** (M3) — para comandos que invocan cadenas de procesos (docker, systemctl), usar `cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}` y matar el grupo completo en timeout.

---

## Implications for Roadmap

La investigacion de arquitectura ya provee un orden de fases explicitamente validado por dependencias del compilador. Las fases pueden ejecutarse en orden y algunas en paralelo.

### Phase 1: Config + Schema Foundation
**Rationale:** Todo lo demas depende de estas dos piezas. `MetricDef` struct en config debe existir antes de escribir el collector; el schema SQL debe existir antes de las funciones CRUD. Sin esto, ninguna otra fase compila.
**Delivers:** `internal/config/config.go` extendido con `MetricDef`, migracion `003_metrics.sql` con tablas `metrics` y `metric_reads` (+ indice compuesto), funciones CRUD en `internal/db/metrics.go`
**Addresses:** Dependencia base de todas las features de v2.0
**Avoids:** M4 (schema correcto desde el inicio previene retrofitting de indices en tabla poblada)

### Phase 2: Metrics Collector
**Rationale:** El nucleo del milestone. Depende de Phase 1 (config types + db functions). Una vez que el collector funciona con metricas predefinidas, el valor central del milestone es visible y testeable end-to-end.
**Delivers:** `internal/metrics/collector.go` con goroutine de ticker, evaluacion de umbrales, y encolado de alertas; integracion en `serve.go`
**Uses:** `gopsutil/v4` para metricas predefinidas; `exec.CommandContext` con timeout para metricas custom
**Avoids:** M1 (permisos 0600), M2 (timeout obligatorio), M3 (process group kill), M4 (collect-then-write)

### Phase 3: REST API + CLI
**Rationale:** Independiente del Phase 2 en implementacion (ambos dependen de Phase 1). Puede desarrollarse en paralelo al collector. La API es prerequisito del dashboard; los CLI usan la API o la DB directamente.
**Delivers:** Endpoints `GET /api/v1/metrics`, `GET /api/v1/metrics/{name}/readings`, `POST /api/v1/metrics/ingest`; subcomandos `jaimito status` y `jaimito metric push`
**Implements:** Router chi extension, handlers, auth inheritance para ingest

### Phase 4: Embedded Dashboard
**Rationale:** Depende de Phase 3 (necesita la API estable). El dashboard es un consumidor de la API, no un productor. El valor visible del milestone — la interfaz web — se entrega aqui.
**Delivers:** `internal/web/embed.go` + `static/index.html` + `static/app.js`; tabla de metricas con Alpine.js; chart expandible con uPlot; auto-refresh 30s
**Uses:** Alpine.js v3, uPlot v1.6.32, Tailwind CSS pre-compilado
**Avoids:** M5 (embed.go en lugar correcto), M6 (nombres sin `.`/`_`), M7 (no routing por URL path), M8 (no Play CDN)

### Phase 5: Cleanup + Polish
**Rationale:** Puede integrarse al final una vez que el schema existe. Bajo riesgo, bajo esfuerzo. Cierra la operacion continua del sistema.
**Delivers:** Purga de `metric_reads` en `cleanup/scheduler.go`; configuracion de ejemplo en `configs/config.example.yaml`; metricas Docker predefinidas (opcional, graceful si Docker no esta instalado)

### Phase Ordering Rationale

- **Config + Schema primero** porque Go no compila si los tipos no existen y SQLite no tiene las tablas.
- **Collector y API pueden desarrollarse en paralelo** una vez que Phase 1 esta completa — son independientes entre si y no comparten estado mutable fuera de la DB.
- **Dashboard al final** porque es un consumidor puro de la API; desarrollarlo antes significaria mockar la API o esperar.
- **Cleanup separado** porque no bloquea nada y puede integrarse en cualquier momento post-schema.

### Research Flags

Fases con patrones bien documentados (skip research-phase durante planning):
- **Phase 1 (Config + Schema):** Extension de struct Go existente + SQL migration numerada. Patron establecido, el codebase ya lo hace con las migraciones anteriores.
- **Phase 3 (REST API + CLI):** chi router extension + cobra subcommand. Patron establecido en el codebase existente.
- **Phase 5 (Cleanup):** DELETE con WHERE sobre timestamp. Trivial, ya existe el scheduler.

Fases que pueden necesitar micro-investigacion puntual durante planning:
- **Phase 2 (Collector):** Verificar la API exacta de `gopsutil/v4` para cada metrica predefinida con pkg.go.dev antes de implementar. El pitfall M3 (process group kill) puede requerir verificacion en el entorno VPS especifico.
- **Phase 4 (Dashboard):** La integracion uPlot + Alpine.js tiene opciones de API que conviene verificar con ejemplos actuales. El paso de Tailwind standalone CLI en el Makefile necesita establecerse correctamente antes de escribir HTML.

---

## Confidence Assessment

| Area | Confidence | Notes |
|------|------------|-------|
| Stack | HIGH | Versiones verificadas contra pkg.go.dev y GitHub releases. gopsutil v4.26.2, uPlot v1.6.32, Alpine.js v3 son stable. Tamanios de bundle verificados con bundlephobia. |
| Features | HIGH | Basado en analisis directo de Beszel, Netdata, Glances y patrones documentados de uso de operadores VPS. Anti-features estan fundamentadas con casos reales de alert fatigue. |
| Architecture | HIGH | Basado en inspeccion directa del codebase existente. Las integraciones estan especificadas a nivel de linea de codigo con referencias a archivos concretos. |
| Pitfalls | HIGH | M1-M4 son pitfalls de Go/SQLite con amplia documentacion y ejemplos de codigo verificados. M5-M8 son comportamientos documentados de la stdlib `embed`. |

**Overall confidence:** HIGH

### Gaps to Address

- **API de gopsutil/v4 para Docker:** La investigacion menciona `docker.GetDockerStat()` pero la implementacion exacta para "running container count" debe verificarse con pkg.go.dev durante Phase 2. Si la API no cubre el caso, fallback a `exec.Command("docker", "ps", "--format", "json")`.
- **Tailwind input.css:** El archivo fuente CSS de Tailwind para jaimito no existe aun y debe crearse antes del primer build del dashboard.
- **Routing del dashboard:** La investigacion recomienda evitar URL-based SPA routing (Pitfall M7). La decision entre hash routing y single-path debe tomarse al inicio de Phase 4.
- **Config file permissions en VPS de destino:** El enforcement de 0600 (Pitfall M1) debe validarse en el VPS real donde corre jaimito para confirmar que el usuario del servicio tiene los permisos adecuados.

---

## Sources

### Primary (HIGH confidence)
- Codebase jaimito — inspeccion directa de `serve.go`, `db/db.go`, `api/server.go`, `config/config.go`, `schema/*.sql`
- `pkg.go.dev/github.com/shirou/gopsutil/v4` — v4.26.2, CGO-free confirmado
- `github.com/leeoniya/uPlot` — v1.6.32, benchmarks de performance (Canvas vs Chart.js)
- `pkg.go.dev/embed` — comportamiento documentado de go:embed (exclusion de `.`/`_`, rutas relativas sin `..`)
- `bundlephobia.com/package/alpinejs` — 7.1KB gzipeado verificado
- `tailwindcss.com/blog/standalone-cli` — CLI sin Node.js, comportamiento de purging

### Secondary (MEDIUM confidence)
- Beszel GitHub + beszel.dev — feature set y arquitectura hub+agent
- Netdata, Glances, OpsDash — analisis comparativo de features y footprint
- Smashing Magazine 2025 — patrones UX de dashboards de monitoreo en tiempo real
- Alert fatigue post-mortems (Icinga, Datadog) — justificacion para transition-only alerts
- Community discussions sobre Tailwind v4 standalone CLI output size (<10KB purged)

### Tertiary (LOW confidence)
- VPS Monitoring Guide 2026 (simpleobservability.com) — recomendaciones de thresholds; validar con el operador real
- Dashboard layout patterns (datawirefra.me) — convenciones de layout; la implementacion especifica puede diferir

---
*Research completed: 2026-03-26*
*Ready for roadmap: yes*
