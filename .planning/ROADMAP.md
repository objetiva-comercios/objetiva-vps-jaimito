# Roadmap: jaimito

## Milestones

- ✅ **v1.0 MVP** — Phases 1-3 (shipped 2026-03-23)
- 🚧 **v1.1 Setup Wizard** — Phases 4-7 (in progress)

## Phases

<details>
<summary>✅ v1.0 MVP (Phases 1-3) — SHIPPED 2026-03-23</summary>

- [x] Phase 1: Foundation (3/3 plans) — completed 2026-02-21
- [x] Phase 2: Core Pipeline (4/4 plans) — completed 2026-02-21
- [x] Phase 3: CLI and Developer Experience (3/3 plans) — completed 2026-02-23

</details>

### 🚧 v1.1 Setup Wizard (In Progress)

**Milestone Goal:** Eliminar la barrera del config.yaml manual con un wizard interactivo que guia al operador paso a paso, valida todo en vivo contra la API de Telegram, y envia una notificacion de test antes de terminar.

- [x] **Phase 4: Wizard Scaffold** — El operador puede ejecutar `jaimito setup` y el wizard arranca, detecta configs existentes y aborta en terminales no-interactivas (completed 2026-03-25)
- [x] **Phase 5: Validacion Telegram** — El operador ingresa bot token y chat IDs con validacion en vivo contra la API de Telegram en cada paso (completed 2026-03-25)
- [ ] **Phase 6: Configuracion y Escritura** — El wizard genera una API key criptografica, valida el config completo y lo escribe a disco con permisos correctos
- [ ] **Phase 7: Verificacion e Integracion** — El wizard envia una notificacion de test que confirma el setup, e install.sh invoca el wizard automaticamente en instalaciones nuevas

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
**Plans**: TBD

Plans:
- [ ] 06-01: Implementar `db.GenerateRawKey()`, steps de server/db config, y pantalla de resumen
- [ ] 06-02: Implementar generacion de YAML, validacion pre-escritura, y escritura con permisos 0600

### Phase 7: Verificacion e Integracion
**Goal**: El operador confirma que su setup funciona de punta a punta antes de salir, e install.sh incorpora el wizard en el flujo de instalacion automatica
**Depends on**: Phase 6
**Requirements**: TEST-01, INST-01
**Success Criteria** (what must be TRUE):
  1. Al completar el wizard, el operador recibe un mensaje de Telegram en su canal general que confirma que el setup esta funcionando
  2. Ejecutar install.sh en un VPS sin config existente invoca `jaimito setup` automaticamente via redireccion `/dev/tty`
  3. La instalacion via `curl | bash` completa el wizard interactivo sin errores de stdin aunque el script se ejecute en pipe
**Plans**: TBD

Plans:
- [ ] 07-01: Implementar step de notificacion de test via bot API
- [ ] 07-02: Integrar `jaimito setup` en install.sh con redireccion `/dev/tty`

## Progress

| Phase | Milestone | Plans Complete | Status | Completed |
|-------|-----------|----------------|--------|-----------|
| 1. Foundation | v1.0 | 3/3 | Complete | 2026-02-21 |
| 2. Core Pipeline | v1.0 | 4/4 | Complete | 2026-02-21 |
| 3. CLI and Developer Experience | v1.0 | 3/3 | Complete | 2026-02-23 |
| 4. Wizard Scaffold | v1.1 | 2/2 | Complete   | 2026-03-25 |
| 5. Validacion Telegram | v1.1 | 2/2 | Complete   | 2026-03-25 |
| 6. Configuracion y Escritura | v1.1 | 0/2 | Not started | - |
| 7. Verificacion e Integracion | v1.1 | 0/2 | Not started | - |
