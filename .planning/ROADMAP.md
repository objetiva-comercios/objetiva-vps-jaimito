# Roadmap: jaimito

## Milestones

- ✅ **v1.0 MVP** — Phases 1-3 (shipped 2026-03-23)
- ✅ **v1.1 Setup Wizard** — Phases 4-7 (shipped 2026-03-26)
- 🚧 **v2.0 Métricas y Dashboard** — Phases 8-12 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-3) — SHIPPED 2026-03-23</summary>

- [x] Phase 1: Foundation (3/3 plans) — completed 2026-02-21
- [x] Phase 2: Core Pipeline (4/4 plans) — completed 2026-02-21
- [x] Phase 3: CLI and Developer Experience (3/3 plans) — completed 2026-02-23

</details>

<details>
<summary>✅ v1.1 Setup Wizard (Phases 4-7) — SHIPPED 2026-03-26</summary>

- [x] **Phase 4: Wizard Scaffold** — El operador puede ejecutar `jaimito setup` y el wizard arranca, detecta configs existentes y aborta en terminales no-interactivas (completed 2026-03-25)
- [x] **Phase 5: Validacion Telegram** — El operador ingresa bot token y chat IDs con validacion en vivo contra la API de Telegram en cada paso (completed 2026-03-25)
- [x] **Phase 6: Configuracion y Escritura** — El wizard genera una API key criptografica, valida el config completo y lo escribe a disco con permisos correctos (completed 2026-03-25)
- [x] **Phase 7: Verificacion e Integracion** — El wizard envia una notificacion de test que confirma el setup, e install.sh invoca el wizard automaticamente en instalaciones nuevas (completed 2026-03-26)

</details>

### 🚧 v2.0 Métricas y Dashboard (In Progress)

**Milestone Goal:** Extender jaimito para recolectar métricas del sistema, almacenarlas, y mostrarlas en un dashboard web embedido — monitoreo liviano sin instalar Prometheus/Grafana.

- [ ] **Phase 8: Config, Schema y CRUD** — El sistema tiene las tablas SQLite, tipos de config y funciones de base de datos necesarias para que las demás fases compilen y funcionen
- [ ] **Phase 9: Metrics Collector y Alertas** — jaimito recolecta métricas del sistema de forma autónoma a intervalos configurables y envía alertas a Telegram cuando se cruzan umbrales
- [ ] **Phase 10: REST API y CLI** — El operador puede consultar métricas actuales e históricas via API y terminal, e ingestar métricas manuales desde scripts
- [ ] **Phase 11: Dashboard Web Embedido** — El operador puede abrir el dashboard en un browser y ver todas las métricas con gráficos expandibles y auto-refresh
- [ ] **Phase 12: Cleanup y Polish** — La retención de datos se aplica automáticamente y la configuración de ejemplo documenta todas las opciones de v2.0

## Phase Details

### Phase 4: Wizard Scaffold
**Goal**: El operador puede invocar `jaimito setup` y ver un TUI interactivo que detecta el estado del sistema antes de comenzar
**Depends on**: Phase 3 (v1.0 cobra CLI base)
**Requirements**: WIZ-01, WIZ-02, WIZ-03
**Success Criteria** (what must be TRUE):
  1. El operador ejecuta `jaimito setup` y ve un wizard bubbletea con pasos visibles
  2. El wizard detecta config existente en `/etc/jaimito/config.yaml` y ofrece tres opciones: editar, crear desde cero, o cancelar
  3. Ejecutar `jaimito setup` sin terminal (ej. en pipe o cron) produce un error descriptivo y exit code 1 en lugar de un panic o hang
  4. Las dependencias bubbletea v2, bubbles v2, lipgloss v2 y golang.org/x/term estan integradas en el modulo Go sin errores de compilacion
**Plans**: 2 plans

Plans:
- [x] 04-01-PLAN.md — Dependencias TUI, cobra setup command, terminal detection, wizard model con Step interface, welcome step, sidebar de progreso
- [x] 04-02-PLAN.md — DetectConfigStep con tres ramas (valido/invalido/inexistente), resumen compacto, backup automatico

### Phase 5: Validacion Telegram
**Goal**: El operador puede ingresar su bot token y chat IDs y recibir confirmacion instantanea de que son validos antes de continuar
**Depends on**: Phase 4
**Requirements**: TELE-01, TELE-02, TELE-03
**Success Criteria** (what must be TRUE):
  1. El operador ingresa un bot token y el wizard muestra el username y display name del bot si el token es valido, o un error claro si no lo es
  2. El operador ingresa un chat ID para el canal general y el wizard confirma que el bot tiene acceso a ese chat via `bot.GetChat()`
  3. El operador puede agregar canales extra (deploys, errors, cron, etc.) con validacion de cada chat ID en el momento de ingreso
  4. Un token invalido o chat ID inaccesible no avanza al siguiente paso — el operador debe corregirlo o cancelar
**Plans**: 2 plans

Plans:
- [x] 05-01-PLAN.md — ValidateTokenWithInfo() en telegram/client.go + BotTokenStep con validacion async, spinner, sequence number y modo edit
- [x] 05-02-PLAN.md — GeneralChannelStep y ExtraChannelsStep con validacion async de chat IDs, loop de canales extra con maquina de estados

### Phase 6: Configuracion y Escritura
**Goal**: El wizard genera y escribe un archivo de configuracion valido que jaimito puede usar inmediatamente al arrancar
**Depends on**: Phase 5
**Requirements**: CONF-01, CONF-02, CONF-03, CONF-04
**Success Criteria** (what must be TRUE):
  1. El wizard muestra una API key unica con prefijo `sk-` que el operador puede copiar antes de confirmar
  2. El operador ve un resumen completo de toda la configuracion (bot, canales, server, db, API key) antes de que se escriba nada a disco
  3. El archivo `/etc/jaimito/config.yaml` se crea con permisos 0600 y contenido que pasa `config.Validate()` sin errores
  4. Si `config.Validate()` falla internamente, el wizard informa el problema y no escribe el archivo
**Plans**: 2 plans

Plans:
- [x] 06-01: Implementar `db.GenerateRawKey()`, steps de server/db config, y pantalla de resumen
- [x] 06-02: Implementar generacion de YAML, validacion pre-escritura, y escritura con permisos 0600

### Phase 7: Verificacion e Integracion
**Goal**: El operador confirma que su setup funciona de punta a punta antes de salir, e install.sh incorpora el wizard en el flujo de instalacion automatica
**Depends on**: Phase 6
**Requirements**: TEST-01, INST-01
**Success Criteria** (what must be TRUE):
  1. Al completar el wizard, el operador recibe un mensaje de Telegram en su canal general que confirma que el setup esta funcionando
  2. Ejecutar install.sh en un VPS sin config existente invoca `jaimito setup` automaticamente via redireccion `/dev/tty`
  3. La instalacion via `curl | bash` completa el wizard interactivo sin errores de stdin aunque el script se ejecute en pipe
**Plans**: 2 plans

Plans:
- [x] 07-01-PLAN.md — Notificacion de test automatica post-escritura en SummaryStep con patron async (spinner + resultMsg)
- [x] 07-02-PLAN.md — Integrar `jaimito setup` en install.sh con redireccion `/dev/tty` y manejo de reinstalacion

### Phase 8: Config, Schema y CRUD
**Goal**: El sistema tiene las tablas SQLite, tipos de configuracion y funciones de base de datos que permiten a las fases siguientes compilar y funcionar sin necesitar retrofitting
**Depends on**: Phase 7 (v1.1 completo, config.yaml base establecida)
**Requirements**: STOR-01, STOR-02, STOR-03, MCOL-03, ALRT-01
**Success Criteria** (what must be TRUE):
  1. El binario compila con los nuevos tipos `MetricDef` y `MetricsConfig` en `config.yaml` sin errores
  2. La migración `003_metrics.sql` se aplica al arrancar jaimito y crea las tablas `metrics` y `metric_reads` con índices correctos
  3. Las funciones CRUD en `internal/db/metrics.go` (upsert metric, insert reading, query readings, list metrics) pasan sus tests unitarios
  4. Un `config.yaml` de ejemplo con sección `metrics` (retention, alert_cooldown, y una métrica custom) es reconocido como válido por `config.Validate()`
  5. `internal/db/metrics.go` puede ejecutar DELETE de readings con `recorded_at < now - 7 days` sin errores de schema
**Plans**: 2 plans

Plans:
- [ ] 08-01-PLAN.md — Tipos Go (MetricsConfig, MetricDef, Thresholds), parseDuration con soporte "d", validacion, config.example.yaml con 5 metricas
- [ ] 08-02-PLAN.md — Migracion 003_metrics.sql, funciones CRUD (UpsertMetric, InsertReading, QueryReadings, ListMetrics, PurgeOldReadings, UpdateMetricStatus), tests

### Phase 9: Metrics Collector y Alertas
**Goal**: jaimito recolecta métricas del sistema de forma autónoma a intervalos configurables y envía alertas a Telegram cuando una métrica cruza un umbral por primera vez
**Depends on**: Phase 8
**Requirements**: MCOL-01, MCOL-02, MCOL-04, MCOL-05, ALRT-02, ALRT-03, ALRT-04
**Success Criteria** (what must be TRUE):
  1. Con jaimito corriendo, las métricas predefinidas (disk_root, ram_used, cpu_load, uptime_days) se registran en la DB a sus intervalos configurados sin intervención manual
  2. Un comando shell que demora más de 10 segundos es cancelado y el fallo se registra en el log, pero las demás métricas del siguiente ciclo se recolectan normalmente
  3. Cuando una métrica cruza el umbral warning por primera vez, llega una alerta a Telegram; si sigue en warning en el siguiente poll, no llega una segunda alerta
  4. Cuando el estado pasa de warning a critical, llega una nueva alerta; cuando vuelve a ok, el estado se resetea correctamente para la próxima transición
  5. docker_running se recolecta si Docker está instalado; si no está instalado, la métrica falla silenciosamente sin afectar el servicio
**Plans**: TBD

### Phase 10: REST API y CLI
**Goal**: El operador puede consultar el estado de las métricas y su historial desde la terminal o via HTTP, e ingestar lecturas manuales desde scripts externos
**Depends on**: Phase 8
**Requirements**: API-01, API-02, API-03, CLI-01, CLI-02
**Success Criteria** (what must be TRUE):
  1. `GET /api/v1/metrics` retorna JSON con la lista de métricas, su último valor, configuración y estado (ok/warning/critical) sin requerir autenticación
  2. `GET /api/v1/metrics/{name}/readings?since=2h` retorna el historial de lecturas de esa métrica filtrado por el parámetro de tiempo (2h, 24h, 7d)
  3. `POST /api/v1/metrics` con Bearer token ingesta una lectura manual y retorna 201; sin token retorna 401
  4. `jaimito status` imprime una tabla en terminal con nombre, valor actual, unidad, estado y tiempo de última lectura de cada métrica
  5. `jaimito metric -n nombre --value 42.5` envía una lectura manual via POST y confirma éxito o imprime el error recibido
**Plans**: TBD

### Phase 11: Dashboard Web Embedido
**Goal**: El operador puede abrir el dashboard en un browser y ver todas las métricas del VPS en tiempo real con gráficos expandibles y sin instalar nada extra
**Depends on**: Phase 10
**Requirements**: DASH-01, DASH-02, DASH-03, DASH-04, DASH-05, DASH-06
**Success Criteria** (what must be TRUE):
  1. `GET /dashboard` retorna el HTML del dashboard embedido en el binario sin requerir archivos externos en el sistema de archivos
  2. La tabla muestra todas las métricas con nombre, valor actual, sparkline de tendencia e indicador de estado (ok/warning/critical) coloreado
  3. Hacer click en una fila expande un gráfico uPlot con el historial de esa métrica; un segundo click lo colapsa
  4. El dashboard muestra el hostname del VPS y actualiza los datos cada 30 segundos sin recargar la página completa
  5. El dashboard funciona correctamente con Tailwind CSS pre-compilado embedido, sin requerir conexión a internet ni CDN externos
**Plans**: TBD
**UI hint**: yes

### Phase 12: Cleanup y Polish
**Goal**: La retención de datos se aplica automáticamente para que la DB no crezca indefinidamente, y el ejemplo de config documenta todas las capacidades de v2.0
**Depends on**: Phase 8 (schema y CRUD listos), Phase 9 (collector activo)
**Requirements**: MCOL-05 (ya cubierto en Phase 9 — ver nota), STOR-02
**Success Criteria** (what must be TRUE):
  1. Después de 7 días corriendo, los readings más viejos de 7 días son eliminados automáticamente por el scheduler periódico
  2. El archivo `configs/config.example.yaml` incluye una sección `metrics` comentada que documenta todos los campos: retention, alert_cooldown, y al menos dos métricas custom de ejemplo
  3. El binario compilado arranca, recolecta métricas, sirve el dashboard y el API en un VPS limpio con solo el binary y el config.yaml
**Plans**: TBD

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | Complete | 2026-02-21 |
| 2. Core Pipeline | v1.0 | 4/4 | Complete | 2026-02-21 |
| 3. CLI and Developer Experience | v1.0 | 3/3 | Complete | 2026-02-23 |
| 4. Wizard Scaffold | v1.1 | 2/2 | Complete | 2026-03-25 |
| 5. Validacion Telegram | v1.1 | 2/2 | Complete | 2026-03-25 |
| 6. Configuracion y Escritura | v1.1 | 2/2 | Complete | 2026-03-25 |
| 7. Verificacion e Integracion | v1.1 | 2/2 | Complete | 2026-03-26 |
| 8. Config, Schema y CRUD | v2.0 | 0/2 | Planned    |  |
| 9. Metrics Collector y Alertas | v2.0 | 0/? | Not started | - |
| 10. REST API y CLI | v2.0 | 0/? | Not started | - |
| 11. Dashboard Web Embedido | v2.0 | 0/? | Not started | - |
| 12. Cleanup y Polish | v2.0 | 0/? | Not started | - |
