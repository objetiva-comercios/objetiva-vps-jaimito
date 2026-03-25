---
phase: 04-wizard-scaffold
plan: 01
subsystem: ui
tags: [bubbletea, lipgloss, bubbles, cobra, tui, wizard, terminal]

requires: []
provides:
  - "cmd/jaimito/setup cobra subcommand con deteccion de terminal no-interactiva"
  - "WizardModel bubbletea v2 con Step interface y SetupData"
  - "Sidebar de progreso con 7 steps, indicadores visual (activo/completado/pendiente)"
  - "WelcomeStep con banner ASCII ╔═╗ y PlaceholderStep para steps 2-7"
  - "Tema lipgloss con paleta de colores locked (cyan/verde/rojo/amarillo/gris)"
  - "FormatNonInteractiveError() y RunWizard() como API publica del paquete setup"
affects:
  - "04-02-detect-config"
  - "fase-05-telegram-validation"
  - "fase-06-config-generation"

tech-stack:
  added:
    - "charm.land/bubbletea/v2 v2.0.2 — TUI event loop (v2 API: Init() Cmd, Update(Msg) (Model, Cmd), View() tea.View)"
    - "charm.land/bubbles/v2 v2.0.0 — componentes TUI (textinput, spinner para fases posteriores)"
    - "charm.land/lipgloss/v2 v2.0.2 — estilos terminal (colores hex, bold, render)"
    - "golang.org/x/term v0.41.0 — term.IsTerminal() para deteccion de TTY"
  patterns:
    - "Step interface: Init(data)/Update(msg,data)/View(data)/Done() — NO tea.Model anidados"
    - "WizardModel con value receiver, completedMap para estado de pasos, steps[]Step por puntero"
    - "tea.KeyPressMsg (v2 API) — NO tea.KeyMsg (v1 API obsoleta)"
    - "Pitfall W2: apertura /dev/tty para output cuando stdout no es TTY"
    - "renderSidebar: step activo siempre muestra ▸ aunque este en completedMap"

key-files:
  created:
    - "cmd/jaimito/setup/styles.go — constantes de color y estilos lipgloss"
    - "cmd/jaimito/setup/wizard.go — WizardModel, SetupData, Step interface, renderLayout, FormatNonInteractiveError, RunWizard"
    - "cmd/jaimito/setup/steps.go — WelcomeStep con marco ASCII, PlaceholderStep"
    - "cmd/jaimito/setup.go — cobra subcommand setup con term.IsTerminal guard"
    - "cmd/jaimito/setup/wizard_test.go — 6 tests TDD"
  modified:
    - "go.mod — 4 nuevas dependencias directas + transitivas"
    - ".gitignore — patron /jaimito (con slash) para no ignorar cmd/jaimito/"

key-decisions:
  - "Step interface simple (no tea.Model anidados) para evitar complejidad de message-passing"
  - "SetupData como puntero compartido — mutacion directa por cada step"
  - "renderSidebar prioriza step activo sobre completedMap para navegacion correcta hacia atras"
  - ".gitignore corregido: /jaimito en vez de jaimito para no ignorar cmd/jaimito/ directory"

patterns-established:
  - "TDD: tests antes de implementacion, RED verificado, GREEN con fix iterativo"
  - "Todos los Update() de Step retornan (Step, tea.Cmd) — nunca mutan el receiver directamente"
  - "PlaceholderStep como stub temporal mientras fases posteriores implementan steps reales"

requirements-completed: [WIZ-01, WIZ-03]

duration: 8min
completed: 2026-03-25
---

# Phase 4 Plan 01: Wizard Scaffold Summary

**Scaffold completo del wizard `jaimito setup` con bubbletea v2: cobra subcommand, deteccion TTY, modelo WizardModel con Step interface, sidebar de 7 steps con indicadores visuales, WelcomeStep con banner ASCII y 6 tests TDD pasando.**

## Performance

- **Duration:** 8 min
- **Started:** 2026-03-25T00:25:13Z
- **Completed:** 2026-03-25T00:34:07Z
- **Tasks:** 1
- **Files modified:** 8

## Accomplishments

- Binario `jaimito setup` compila y registra el subcommand cobra correctamente
- Terminal no-interactiva (stdin pipe) detectada y reportada con error descriptivo en rojo via lipgloss
- Pantalla de bienvenida renderiza marco doble ASCII ╔═╗ con nombre y lista de lo que se va a configurar
- Sidebar de 7 steps con indicadores ▸ (activo, cyan), ✓ (completado, verde), espaciado (pendiente, gris) y contador [N/7]
- Navegacion hacia atras con Esc funciona correctamente (el step activo siempre muestra ▸)
- Confirmacion de salida con doble Ctrl+C implementada

## Task Commits

1. **Task 1: Dependencias TUI + styles + wizard model + setup cobra command + welcome step** - `014dcc0` (feat)

**Plan metadata:** pendiente (commit final de docs)

## Files Created/Modified

- `cmd/jaimito/setup/styles.go` — Paleta de colores locked y estilos lipgloss (StepActive, StepDone, StepPending, TitleStyle, ErrorStyle, HintStyle)
- `cmd/jaimito/setup/wizard.go` — WizardModel, SetupData struct, Step interface, renderSidebar, renderLayout, FormatNonInteractiveError, RunWizard con Pitfall W2
- `cmd/jaimito/setup/steps.go` — WelcomeStep con marco ASCII ╔═╗, PlaceholderStep temporal para steps 2-7
- `cmd/jaimito/setup.go` — Cobra subcommand "setup" con term.IsTerminal guard y llamada a RunWizard
- `cmd/jaimito/setup/wizard_test.go` — 6 tests TDD: FormatNonInteractiveError, WelcomeStep_View, WelcomeStep_Done, WizardModel_Sidebar, WizardModel_ConfirmExit, WizardModel_BackNavigation
- `go.mod` / `go.sum` — 4 dependencias nuevas + transitivas
- `.gitignore` — Fix patron /jaimito para no ignorar cmd/jaimito/

## Decisions Made

- Step interface simple (Init/Update/View/Done) sin tea.Model anidados — evita complejidad de message-passing entre modelos
- renderSidebar muestra step activo como ▸ incluso si fue completado antes — necesario para navegacion correcta hacia atras
- .gitignore: `jaimito` cambiado a `/jaimito` (con slash) para matchear solo el binario en la raiz y no el directorio cmd/jaimito/

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Sidebar navegacion back: step activo se mostraba como completado**
- **Found during:** Task 1 (TestWizardModel_BackNavigation fallo)
- **Issue:** renderSidebar verificaba completedMap antes que currentStep. Cuando se navega hacia atras a un step ya completado, se mostraba ✓ en vez de ▸
- **Fix:** Inversion de prioridad en renderSidebar: `if i == currentStep` se evalua primero
- **Files modified:** cmd/jaimito/setup/wizard.go (renderSidebar)
- **Verification:** TestWizardModel_BackNavigation PASS
- **Committed in:** 014dcc0 (incluido en el commit del task)

**2. [Rule 1 - Bug] .gitignore patron jaimito bloqueaba tracking de cmd/jaimito/setup/**
- **Found during:** Task 1 (git status no mostraba los archivos nuevos del directorio setup/)
- **Issue:** El patron `jaimito` sin slash matchea cualquier directorio llamado "jaimito" en el arbol, incluyendo cmd/jaimito/ y sus subdirectorios
- **Fix:** Cambiado a `/jaimito` (con slash al inicio) para matchear solo el binario en la raiz del proyecto
- **Files modified:** .gitignore
- **Verification:** `git check-ignore cmd/jaimito/setup/wizard.go` retorna exit 1 (no ignorado)
- **Committed in:** 014dcc0

---

**Total deviations:** 2 auto-fixed (2 Rule 1 - Bug)
**Impact on plan:** Ambos fixes necesarios para correctitud. Sin scope creep.

## Issues Encountered

- Las dependencias charm.land/* se agregaron con `go get` pero `go mod tidy` las eliminaba (eran "unused" hasta que el codigo que las usa existia). Se agregaron de nuevo despues de crear los archivos .go.

## Known Stubs

- `PlaceholderStep` en cmd/jaimito/setup/steps.go — steps 2-7 muestran "Proximamente..." hasta que se implementen en planes posteriores (plan 02 agrega DetectConfigStep; fases 5-7 agregan bot token, canales, DB, resumen)
- Estos stubs son **intencionales** y el plan los documenta explicitamente. El objetivo de este plan es solo el scaffold, no la implementacion completa.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- Scaffold completo. Plan 02 puede agregar DetectConfigStep en steps.go sin modificar wizard.go
- Step interface y SetupData definidos — todos los steps futuros siguen el mismo patron
- PlaceholderSteps activos — el wizard es navegable de extremo a extremo con Enter/Esc
- Las 4 dependencias TUI estan en go.mod y compilando

---
*Phase: 04-wizard-scaffold*
*Completed: 2026-03-25*
