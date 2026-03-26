---
phase: 04-wizard-scaffold
plan: 02
subsystem: ui
tags: [bubbletea, lipgloss, tui, wizard, terminal, config-detection]

requires:
  - phase: 04-01
    provides: "WizardModel con Step interface, SetupData, WelcomeStep, sidebar de progreso"

provides:
  - "DetectConfigStep con tres ramas (valido/invalido/inexistente)"
  - "obfuscateToken() — ofusca bot token mostrando solo ultimos 6 chars"
  - "backupConfig() — copia config.yaml a config.yaml.bak antes de sobreescribir"
  - "SetupData.ConfigExists bool para deteccion explicita de archivo"
  - "WizardModel.sidebarOffset para steps internos sin afectar sidebar visible"
  - "NewWizardModelWithExists() y RunWizardWithExists() como API publica extendida"

affects:
  - "fase-05-telegram-validation"
  - "fase-06-config-generation"

tech-stack:
  added: []
  patterns:
    - "sidebarOffset: steps internos (pre-wizard) no visibles en el sidebar de 7 steps"
    - "DetectConfigStep.Init() skipea inmediatamente si ConfigExists=false (Mode='new', Done()=true)"
    - "backupConfig() ejecutado en modo 'fresh' antes de sobreescribir — silenciado si falla"
    - "NewWizardModel infiere configExists desde existingCfg/configErr; NewWizardModelWithExists para deteccion explicita"

key-files:
  created:
    - "cmd/jaimito/setup/detect_config_test.go — 8 tests TDD para DetectConfigStep, obfuscateToken, backupConfig"
  modified:
    - "cmd/jaimito/setup/steps.go — DetectConfigStep, obfuscateToken, backupConfig"
    - "cmd/jaimito/setup/wizard.go — SetupData.ConfigExists, WizardModel.sidebarOffset, NewWizardModelWithExists, RunWizardWithExists"
    - "cmd/jaimito/setup.go — runSetup usa os.Stat para deteccion explicita de archivo"

key-decisions:
  - "sidebarOffset en WizardModel: DetectConfigStep es step 0 interno cuando hay config, sidebar sigue mostrando 7 steps desde index 0"
  - "NewWizardModel infiere configExists (compatibilidad con tests existentes); runSetup usa os.Stat para deteccion real"
  - "backupConfig error es silenciado en el wizard flow — backup es best-effort, no critico"

patterns-established:
  - "Steps internos (no visibles en sidebar) se insertan al inicio del slice de steps con sidebarOffset ajustado"
  - "ConfigExists bool separa la deteccion del archivo de la validacion del contenido"

requirements-completed: [WIZ-02]

duration: 4min
completed: 2026-03-25
---

# Phase 4 Plan 02: Detect Config Step Summary

**DetectConfigStep con tres ramas (config valido/invalido/inexistente): resumen compacto con token ofuscado, backup automatico en modo fresh, skip silencioso cuando no hay config, y 8 tests TDD pasando.**

## Performance

- **Duration:** 4 min
- **Started:** 2026-03-25T00:37:13Z
- **Completed:** 2026-03-25T00:41:00Z
- **Tasks:** 1
- **Files modified:** 4

## Accomplishments

- Config valido detectado: muestra token ofuscado (ultimos 6 chars), cantidad de canales, listen address, db path, y 3 opciones (Editar / Crear desde cero / Cancelar)
- Config invalido detectado: muestra error especifico y 2 opciones (Crear desde cero / Cancelar) — sin opcion Editar
- Config inexistente: wizard skipea el step silenciosamente y arranca directo con Mode="new"
- Backup automatico a config.yaml.bak al seleccionar "Crear desde cero"
- sidebarOffset implementado para que DetectConfigStep no afecte la sidebar de 7 steps
- Todos los tests del plan 01 siguen pasando (6 tests originales + 8 nuevos = 14 totales)

## Task Commits

1. **Task 1: DetectConfigStep con tres ramas + backup + pre-llenado** - `e747a24` (feat)

**Plan metadata:** `494769e` (docs: complete detect-config-step plan)

## Files Created/Modified

- `cmd/jaimito/setup/detect_config_test.go` — 8 tests TDD: ValidConfig, InvalidConfig, MissingConfig, SelectEdit, SelectFresh, SelectCancel, ObfuscateToken, BackupConfig
- `cmd/jaimito/setup/steps.go` — DetectConfigStep struct con Init/Update/View/Done, obfuscateToken, backupConfig
- `cmd/jaimito/setup/wizard.go` — SetupData.ConfigExists, WizardModel.sidebarOffset, NewWizardModel/NewWizardModelWithExists, RunWizard/RunWizardWithExists, renderSidebar actualizado
- `cmd/jaimito/setup.go` — runSetup usa os.Stat + RunWizardWithExists para deteccion explicita

## Decisions Made

- sidebarOffset en WizardModel: la sidebar siempre muestra los 7 steps visibles; DetectConfigStep es un step 0 interno que no tiene entrada en stepNames. El contador [N/7] y el indicador ▸ se calculan con `currentStep - sidebarOffset`.
- NewWizardModel infiere `configExists = existingCfg != nil || configErr != nil` para mantener compatibilidad con tests existentes. La funcion nueva `NewWizardModelWithExists` recibe el bool explicitamente (usado por `runSetup` con `os.Stat`).
- `backupConfig` error silenciado en el wizard: el backup es best-effort. Si falla (disco lleno, permisos), el usuario ya eligio "Crear desde cero" y la operacion continua.

## Deviations from Plan

None - plan ejecutado exactamente como estaba escrito. La unica decision de diseno (sidebarOffset vs alternativas) estaba documentada en el plan como "evaluar segun como quedo plan 01" — se eligio sidebarOffset como la mas limpia dado el estado actual del WizardModel.

## Issues Encountered

- El plan sugeria tres alternativas para manejar DetectConfigStep en la sidebar. Se eligio sidebarOffset sobre las otras dos (ejecutar en setup.go, o embeber en WelcomeStep) porque mantiene el DetectConfigStep como un Step independiente testeable sin afectar el layout del sidebar.

## Known Stubs

- El campo `ExistingCfg` en SetupData se mantiene para pre-llenar los steps futuros en modo "edit". Actualmente los PlaceholderSteps no usan este valor — sera implementado en fases 5-7 cuando se creen BotTokenStep, ChannelStep, etc.

## User Setup Required

None - no external service configuration required.

## Next Phase Readiness

- DetectConfigStep completo y testeado. Fase 5 puede agregar BotTokenStep que lea `data.ExistingCfg.Telegram.Token` cuando `data.Mode == "edit"` para pre-llenar el campo.
- SetupData.Mode=("new"|"edit"|"fresh") disponible para steps futuros que necesiten comportarse diferente segun el modo
- sidebarOffset y la arquitectura de steps internos esta establecida — nuevos steps internos pueden agregarse incrementando sidebarOffset

---
*Phase: 04-wizard-scaffold*
*Completed: 2026-03-25*

## Self-Check: PASSED

- detect_config_test.go: FOUND
- steps.go: FOUND
- SUMMARY.md: FOUND
- Commit e747a24 (task): FOUND
- Commit 494769e (docs): FOUND
- go test ./cmd/jaimito/setup/...: PASS (14 tests)
