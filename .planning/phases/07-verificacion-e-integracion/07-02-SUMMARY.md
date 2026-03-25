---
phase: 07-verificacion-e-integracion
plan: 02
subsystem: infra
tags: [install.sh, wizard, setup, jaimito, bash, shell]

# Dependency graph
requires:
  - phase: 06-wizard-setup
    provides: "jaimito setup subcommand interactivo via /dev/tty"
provides:
  - "install.sh invoca wizard automaticamente en instalacion nueva y ofrece reconfigurar en reinstalacion"
  - "Instalacion zero-manual: operador no necesita editar config.yaml a mano"
affects:
  - deploy
  - install
  - operacion-vps

# Tech tracking
tech-stack:
  added: []
  patterns:
    - "stdin interactivo via /dev/tty en scripts ejecutados con curl|bash"
    - "fallback gracioso cuando wizard falla: servicio no se inicia, hint para correr despues"

key-files:
  created: []
  modified:
    - install.sh

key-decisions:
  - "Usar sudo ${BINARY_DEST} setup < /dev/tty para compatibilidad con curl|bash (D-08, D-10)"
  - "Si wizard falla en instalacion nueva, CONFIG_NEEDS_EDIT=true previene inicio del servicio"
  - "Prompt de reinstalacion usa printf + read < /dev/tty (no echo -e) para shell portable"

patterns-established:
  - "Pattern: scripts curl|bash SIEMPRE redirigen stdin interactivo desde /dev/tty"
  - "Pattern: fallback en wizard = warn + no iniciar servicio + hint para correr manualmente"

requirements-completed: [INST-01]

# Metrics
duration: 15min
completed: 2026-03-25
---

# Phase 07 Plan 02: Integracion del wizard en install.sh - Summary

**install.sh reemplaza edicion manual de config.yaml con invocacion automatica de `jaimito setup` via /dev/tty, con manejo diferenciado instalacion nueva vs reinstalacion y fallback si el wizard falla**

## Performance

- **Duration:** ~15 min
- **Started:** 2026-03-25
- **Completed:** 2026-03-25
- **Tasks:** 2 (1 auto + 1 human-verify)
- **Files modified:** 1

## Accomplishments

- Eliminado el bloque de `config.example.yaml` con instrucciones manuales de edicion
- Instalacion nueva: invoca `sudo "${BINARY_DEST}" setup < /dev/tty` automaticamente
- Reinstalacion: pregunta si reconfigurar antes de invocar wizard (prompt con `/dev/tty`)
- Wizard fallido: `CONFIG_NEEDS_EDIT=true` impide que el servicio se inicie, con hint `sudo jaimito setup`
- Syntax check `bash -n install.sh` pasa sin errores

## Task Commits

Cada tarea fue commiteada atomicamente:

1. **Task 1: Reemplazar bloque config en install.sh con invocacion del wizard** - `3910d8f` (feat)
2. **Task 2: Verificacion visual del install.sh modificado** - checkpoint aprobado por operador (sin commit propio)

**Plan metadata:** (pendiente commit docs)

## Files Created/Modified

- `install.sh` - Bloque `# -- Config --` reemplazado con invocacion de `jaimito setup < /dev/tty`, manejo de reinstalacion y fallback si wizard falla

## Decisions Made

- `sudo "${BINARY_DEST}" setup < /dev/tty`: usa variable BINARY_DEST (ya definida), sudo obligatorio para escribir en /etc/jaimito, redireccion /dev/tty necesaria para compatibilidad curl|bash
- Fallback cuando wizard falla: `CONFIG_NEEDS_EDIT=true` — previene inicio del servicio. Operador puede correr `sudo jaimito setup` cuando este listo
- Prompt de reinstalacion: `printf` + `read -r RECONFIG < /dev/tty` — patron shell portable sin dependencia de flags de echo

## Deviations from Plan

Ninguna — plan ejecutado exactamente como fue escrito.

## Issues Encountered

Ninguno.

## User Setup Required

Ninguno — la integracion es transparente para el operador. El wizard se invoca automaticamente durante `curl | bash install.sh`.

## Next Phase Readiness

- install.sh completo e integrado con el wizard interactivo
- Flujo end-to-end listo: `curl | bash install.sh` guia al operador via wizard sin edicion manual
- Fase 07 verification e integracion completa con ambos planes ejecutados

---
*Phase: 07-verificacion-e-integracion*
*Completed: 2026-03-25*
