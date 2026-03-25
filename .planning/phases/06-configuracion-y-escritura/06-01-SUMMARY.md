---
phase: 06-configuracion-y-escritura
plan: "01"
subsystem: wizard
tags: [go, bubbletea, tdd, wizard, api-key, server-config, database-config]
dependency_graph:
  requires:
    - "05-02: ExtraChannelsStep (wizard framework establecido)"
  provides:
    - "db.GenerateRawKey(): funcion pura para generar sk- + 64 hex chars"
    - "ServerStep: confirm-with-defaults, valida host:port"
    - "DatabaseStep: confirm-with-defaults, warning dir inexistente"
    - "APIKeyStep: genera key, recuadro amarillo, confirmacion s, modo edit"
    - "SetupData.ServerListen, DatabasePath, GeneratedAPIKey, KeepExistingKey"
    - "wizard.go: 8 stepNames, contador dinamico [N/8]"
  affects:
    - "06-02: SummaryStep puede leer ServerListen, DatabasePath, GeneratedAPIKey de SetupData"
tech_stack:
  added:
    - "net.SplitHostPort para validacion de listen address"
    - "os.Stat + filepath.Dir para chequeo de dir padre en DatabaseStep"
    - "lipgloss.RoundedBorder + BorderForeground(ColorYellow) para API key box"
  patterns:
    - "confirm-with-defaults: Enter acepta default, value no vacio valida"
    - "TDD RED-GREEN: tests escritos antes de implementacion"
    - "test helpers exportados: SetInputValueForTest, SetAskingModeForTest, GetGeneratedKeyForTest"
key_files:
  created:
    - "internal/db/apikeys_test.go: TestGenerateRawKey_Format, TestGenerateRawKey_Unique"
    - "cmd/jaimito/setup/server_step.go: ServerStep con confirm-with-defaults y net.SplitHostPort"
    - "cmd/jaimito/setup/server_step_test.go: 7 tests (Init, EnterDefault, ValidAddress, InvalidFormat, InvalidPort, EditMode, EmptyAfterTyping)"
    - "cmd/jaimito/setup/database_step.go: DatabaseStep con os.Stat warning no bloqueante"
    - "cmd/jaimito/setup/database_step_test.go: 6 tests (Init, EnterDefault, CustomPath, DirMissing, EmptyPath, EditMode)"
    - "cmd/jaimito/setup/apikey_step.go: APIKeyStep completa (generacion, recuadro, confirmacion, modo edit)"
    - "cmd/jaimito/setup/apikey_step_test.go: 8 tests (Init, View, Confirm_S, Confirm_Blocked, Confirm_N, EditMode_*)"
  modified:
    - "internal/db/apikeys.go: +GenerateRawKey(), CreateKey() refactoreado"
    - "cmd/jaimito/setup/wizard.go: SetupData +4 campos, stepNames 7->8, contador dinamico, visibleSteps actualizado"
    - "cmd/jaimito/setup/styles.go: +WarningStyle (ColorYellow Bold)"
    - "cmd/jaimito/setup/wizard_test.go: [1/7]->[1/8], +API Key en expectedSteps"
decisions:
  - "GenerateRawKey() es funcion pura en db package — reutilizable desde wizard sin acceso a DB"
  - "APIKeyStep implementada completamente en Task 1 como stub funcional — evita compilacion rota"
  - "WarningStyle agregado a styles.go como variable global — consistente con patron del paquete"
  - "View() muestra defaultValue como texto plano ademas del placeholder del textinput — testeable sin ANSI parsing"
  - "wizard_test.go actualizado inline (Rule 1 auto-fix) — test basado en 7 steps era incorrecto con nueva arquitectura"
metrics:
  duration: "6 minutes"
  completed: "2026-03-25"
  tasks: 2
  files: 11
---

# Phase 06 Plan 01: Configuracion-y-escritura - Server, Database, APIKey Steps Summary

GenerateRawKey funcion pura + ServerStep/DatabaseStep confirm-with-defaults + APIKeyStep con recuadro amarillo y confirmacion + wizard actualizado a 8 steps con contador dinamico.

## Tasks Completed

| # | Task | Commit | Status |
|---|------|--------|--------|
| 1 | GenerateRawKey + ServerStep + DatabaseStep + tests | c7f58bd | DONE |
| 2 | APIKeyStep con generacion, recuadro, confirmacion y modo edit | f34fbc4 | DONE |

## What Was Built

### db.GenerateRawKey()

Funcion pura extraida del cuerpo de `CreateKey()`. Genera una API key criptograficamente segura:
- Prefijo `sk-` + 64 chars hex (32 bytes random)
- Total 67 chars
- `CreateKey()` ahora la delega internamente

### ServerStep (confirm-with-defaults)

Step que recolecta `data.ServerListen`:
- Valor por defecto: `127.0.0.1:8080`
- Enter con input vacio acepta el default
- Validacion: `net.SplitHostPort()` + rango 1-65535
- Error inline si invalido, sin bloquear UX
- Pre-llenado en modo edit desde `ExistingCfg.Server.Listen`

### DatabaseStep (confirm-with-defaults)

Step que recolecta `data.DatabasePath`:
- Valor por defecto: `/var/lib/jaimito/jaimito.db`
- Enter con input vacio acepta el default
- Warning amarillo si directorio padre no existe (NO bloquea — `os.Stat` es advisory)
- Pre-llenado en modo edit desde `ExistingCfg.Database.Path`

### APIKeyStep

Step que genera y confirma la API key:
- Init llama `db.GenerateRawKey()`, guarda en `s.generatedKey` y `data.GeneratedAPIKey`
- View muestra warning `ATENCION` en `WarningStyle` (ColorYellow Bold)
- Key en recuadro `lipgloss.RoundedBorder()` con `BorderForeground(ColorYellow)`
- Gate de confirmacion: solo `s`/`S` avanza `Done()=true`
- Modo edit con `SeedAPIKeys` existentes: selector Generar/Mantener con navegacion up/down
- Mantener: `data.KeepExistingKey=true`, sin generar key nueva

### wizard.go actualizaciones

- `SetupData`: +`ServerListen`, `DatabasePath`, `GeneratedAPIKey`, `KeepExistingKey`
- `stepNames`: 7 → 8 entries (agrega `"API Key"` antes de `"Resumen"`)
- Contador: `"[%d/7]"` hardcoded → `fmt.Sprintf("[%d/%d]", sidebarStep+1, len(stepNames))`
- `visibleSteps`: `PlaceholderStep{"Servidor"}` y `PlaceholderStep{"Base de Datos"}` reemplazados por `&ServerStep{}` y `&DatabaseStep{}`; `&APIKeyStep{}` agregado; `PlaceholderStep{"Resumen"}` se mantiene para plan 06-02

## Deviations from Plan

### Auto-fixed Issues

**1. [Rule 1 - Bug] Redeclaracion de variable en CreateKey tras refactor**
- **Found during:** Task 1 - GREEN phase
- **Issue:** Despues de refactorear CreateKey() para usar GenerateRawKey(), la linea `_, err := database.ExecContext(...)` causaba error de compilacion porque `err` ya estaba declarada
- **Fix:** Cambiar `:=` por `=` en la sentencia ExecContext
- **Files modified:** `internal/db/apikeys.go`
- **Commit:** c7f58bd (incluido en el mismo commit de la feature)

**2. [Rule 1 - Bug] wizard_test.go esperaba [1/7] y 7 step names**
- **Found during:** Task 1 - GREEN phase, al correr `go test ./cmd/jaimito/setup/...`
- **Issue:** `TestWizardModel_Sidebar` chequeaba `[1/7]` y lista de 7 steps — incompatible con la nueva arquitectura de 8 steps
- **Fix:** Actualizar test a `[1/8]` y agregar `"API Key"` a `expectedSteps`
- **Files modified:** `cmd/jaimito/setup/wizard_test.go`
- **Commit:** c7f58bd

**3. [Rule 2 - Missing] textinput placeholder no testeable via strings.Contains**
- **Found during:** Task 1 - GREEN phase al ver que `TestServerStep_Init_NewMode` y `TestDatabaseStep_Init_NewMode` fallaban porque el textinput renderiza el placeholder con ANSI codes
- **Issue:** Las pruebas buscaban `"127.0.0.1:8080"` en plain text pero el textinput lo renderiza como `\x1b[37m> \x1b[m\x1b[7;37m1\x1b[m...`
- **Fix:** Agregar `fmt.Sprintf("Por defecto: %s\n", s.defaultValue)` en `View()` para que el default sea testeable sin parsear ANSI
- **Files modified:** `server_step.go`, `database_step.go`
- **Commit:** c7f58bd

**4. [Rule 2 - Missing] WarningStyle no existia en styles.go**
- **Found during:** Task 1 - implementacion de DatabaseStep
- **Issue:** DatabaseStep necesitaba un estilo warning (ColorYellow Bold) — no habia `WarningStyle` definido en styles.go
- **Fix:** Agregar `WarningStyle = lipgloss.NewStyle().Foreground(ColorYellow).Bold(true)` a styles.go
- **Files modified:** `cmd/jaimito/setup/styles.go`
- **Commit:** c7f58bd

## Known Stubs

- `PlaceholderStep{name: "Resumen"}` en `wizard.go` visibleSteps — intencional, plan 06-02 lo reemplaza con `SummaryStep`

## Self-Check: PASSED

- [x] `internal/db/apikeys.go` — FOUND
- [x] `internal/db/apikeys_test.go` — FOUND
- [x] `cmd/jaimito/setup/server_step.go` — FOUND
- [x] `cmd/jaimito/setup/database_step.go` — FOUND
- [x] `cmd/jaimito/setup/apikey_step.go` — FOUND
- [x] commit a6e3b98 (test RED) — FOUND
- [x] commit c7f58bd (feat GREEN Task 1) — FOUND
- [x] commit f34fbc4 (feat GREEN Task 2) — FOUND
- [x] `go test ./... -count=1` — ALL PASS
- [x] `go build ./...` — OK
