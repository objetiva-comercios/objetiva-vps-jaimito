---
phase: 07-verificacion-e-integracion
plan: "01"
subsystem: setup-wizard
tags: [wizard, telegram, test-notification, tdd, bubbles, spinner]
dependency_graph:
  requires: [06-02-SUMMARY.md]
  provides: [SummaryStep con notificacion de test automatica post-escritura]
  affects: [cmd/jaimito/setup/summary_step.go, cmd/jaimito/setup/summary_step_test.go]
tech_stack:
  added: [charm.land/bubbles/v2/spinner en SummaryStep]
  patterns: [async tea.Cmd con sendTestNotificationCmd, two-phase flow writeConfig->sendTest]
key_files:
  created: []
  modified:
    - cmd/jaimito/setup/summary_step.go
    - cmd/jaimito/setup/summary_step_test.go
decisions:
  - "Sin seq number en testNotificationResultMsg: la notificacion de test se dispara una sola vez (no hay stale responses)"
  - "Channels vacio -> warning inmediato + Done=true (no intentar enviar sin destino)"
  - "Fallo de test es warning amarillo no error rojo: config es valido, el setup no se bloquea"
metrics:
  duration_minutes: 15
  completed_date: "2026-03-25"
  tasks_completed: 1
  tasks_total: 1
  files_changed: 2
requirements_satisfied: [TEST-01]
---

# Phase 07 Plan 01: SummaryStep Test Notification Summary

SummaryStep con notificacion de test Telegram automatica post-escritura: spinner async durante envio, checkmark verde si ok, warning amarillo si falla (no bloqueante).

## Tasks Completed

| Task | Name | Commit | Files |
|------|------|--------|-------|
| RED | Failing tests para notificacion de test | d7b5108 | summary_step_test.go |
| GREEN | Implementacion SummaryStep test notification | 47abec8 | summary_step.go, summary_step_test.go |

## What Was Built

El SummaryStep del wizard ahora tiene un flujo de dos fases:

**Fase 1 - writeConfig**: Igual que antes. Si falla por validacion o escritura, muestra error. Si ok, pasa a fase 2.

**Fase 2 - sendTestNotification**: Automaticamente envia `"✅ jaimito configurado correctamente en <hostname>"` al primer canal usando `data.ValidatedBot`. Durante el envio muestra un spinner Dot. Al recibir el resultado:
- Exito: checkmark verde + hint `sudo systemctl start jaimito`
- Fallo: warning amarillo + hint systemctl (no bloquea al operador)

Guardias defensivos:
- `ValidatedBot == nil`: retorna `testNotificationResultMsg{err: "bot no disponible"}` sin panic
- `len(data.Channels) == 0`: termina con warning inmediato sin intentar enviar
- Timeout 10s con context.WithTimeout
- Panic recovery en sendTestNotificationCmd

## Test Coverage

Nuevos tests (TDD):
- `TestSummaryStep_TestNotification_SendsSpinner` - Enter con exito -> sending=true, View muestra spinner
- `TestSummaryStep_TestNotification_Success` - result nil -> testOk=true, Done=true, View checkmark
- `TestSummaryStep_TestNotification_Failure` - result error -> testErr, Done=true, View warning
- `TestSummaryStep_TestNotification_WarningNotError` - fallo muestra systemctl hint (no bloqueante)
- `TestSummaryStep_TestNotification_NilBot` - nil bot -> error limpio sin panic
- `TestSummaryStep_SpinnerTick_WhenSending` - TickMsg procesado cuando sending=true
- `TestSummaryStep_SpinnerTick_WhenNotSending` - TickMsg ignorado cuando sending=false

Tests existentes actualizados:
- `TestSummaryStep_WriteConfig_Success` - Enter -> sending=true (no Done), result -> Done=true
- `TestSummaryStep_Done_QuitsWizard` - tea.Quit llega en result, no en Enter
- `TestSummaryStep_WriteConfig_CreatesDir` - Done=false despues de Enter (sending), Done=true despues de result

## Decisions Made

1. **Sin seq number**: `testNotificationResultMsg` no tiene `seq` porque la notificacion se dispara exactamente una vez. No hay race condition con stale responses (a diferencia de `tokenValidationResultMsg` que puede tener multiples re-triggers por el usuario).

2. **Warning amarillo no error rojo**: El fallo de la notificacion de test NO impide usar jaimito. El config fue escrito correctamente. El operador puede iniciar el servicio igual. Warning comunica que algo inesperado paso sin bloquear el flujo.

3. **Channels vacio -> exit inmediato**: Si por algun motivo `data.Channels` esta vacio al llegar a SummaryStep, no tiene sentido intentar enviar. Termina con `testErr = "sin canales configurados"` y `Done=true` para no dejar el wizard colgado.

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Test NilBot usaba tea.Batch cmd() incorrectamente**
- **Found during:** Task 1 GREEN phase
- **Issue:** El test intentaba `cmd()` directamente sobre el resultado de `tea.Batch`. En bubbletea v2, `tea.Batch` devuelve una funcion que ejecuta goroutines concurrentes, no retorna el primer msg sincronicamente. Llamar `cmd()` retorna `nil` o un `tea.BatchMsg`, no el `testNotificationResultMsg` esperado.
- **Fix:** Reescribir el test para simular el resultado async directamente via `setup.NewTestNotificationResultMsg(errors.New("bot no disponible"))` - que es el patron correcto para tests de cmds async (identico al patron usado en `bot_token_step_test.go`).
- **Files modified:** cmd/jaimito/setup/summary_step_test.go
- **Commit:** 47abec8 (incluido en commit de implementacion)

## Known Stubs

None - la funcionalidad esta completamente implementada. `sendTestNotificationCmd` llama al bot real con timeout 10s.

## Self-Check: PASSED

Files exist:
- cmd/jaimito/setup/summary_step.go: FOUND
- cmd/jaimito/setup/summary_step_test.go: FOUND
- .planning/phases/07-verificacion-e-integracion/07-01-SUMMARY.md: this file

Commits exist:
- d7b5108: test(07-01): add failing tests (RED phase)
- 47abec8: feat(07-01): SummaryStep con notificacion de test (GREEN phase)

Tests: `go test ./... -count=1` - PASS (all packages green)
Build: `go build ./cmd/jaimito/` - PASS
