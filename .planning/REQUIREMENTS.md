# Requirements: jaimito

**Defined:** 2026-03-24
**Core Value:** Every event that happens on the VPS gets reliably captured and delivered to Telegram — no notification is ever lost silently.

## v1.1 Requirements

Requirements for the Setup Wizard milestone. Each maps to roadmap phases.

### Wizard TUI

- [x] **WIZ-01**: Operador puede ejecutar `jaimito setup` y ver un wizard interactivo paso-a-paso
- [x] **WIZ-02**: Wizard detecta si ya existe config y ofrece editar/crear desde cero/cancelar
- [x] **WIZ-03**: Wizard detecta terminal no-interactiva y aborta con error descriptivo

### Validación Telegram

- [x] **TELE-01**: Operador ingresa bot token y se valida en vivo contra API (retorna username y display name)
- [x] **TELE-02**: Operador ingresa chat ID del canal general y se valida contra `bot.GetChat()`
- [x] **TELE-03**: Operador puede agregar canales extra (deploys, errors, cron, etc.) con validación de cada chat ID

### Configuración

- [x] **CONF-01**: Wizard genera API key criptográfica (`sk-` prefix) y la muestra al operador
- [ ] **CONF-02**: Wizard genera config YAML válido y lo escribe con permisos 0600
- [ ] **CONF-03**: Operador ve un resumen completo antes de confirmar escritura
- [ ] **CONF-04**: Config generado pasa `config.Validate()` antes de escribirse

### Verificación

- [ ] **TEST-01**: Wizard envía notificación de test al canal general para probar el setup completo

### Integración

- [ ] **INST-01**: `install.sh` invoca `jaimito setup` automáticamente cuando no existe config (con `< /dev/tty`)

## Future Requirements

Deferred to future milestones. Tracked but not in current roadmap.

### Reliability

- **RATE-01**: Rate limiting per channel
- **DEDUP-01**: Simple deduplication (same message within N minutes)
- **QUIET-01**: Quiet hours (do-not-disturb window)

### CLI Extensions

- **LIST-01**: `jaimito list` CLI (query recent messages)
- **DRAIN-01**: Message queue drain on SIGTERM

### Observability

- **WAL-01**: WAL checkpoint monitoring in health endpoint

## Out of Scope

Explicitly excluded. Documented to prevent scope creep.

| Feature | Reason |
|---------|--------|
| Email/SMTP dispatcher | Apprise covers this; out of jaimito's scope |
| Web dashboard | Telegram IS the history; a dashboard is a product pivot |
| Multi-user support | Single-VPS model is a feature, not a bug |
| Plugin system | Maintenance burden exceeds value at this scale |
| WhatsApp/PagerDuty/Matrix | Deferred to v2+; recommend Apprise bridge |
| Message grouping/digest | Deferred to future milestone |
| Wizard web UI | CLI-only; keep single-binary principle |

## Traceability

Which phases cover which requirements. Updated during roadmap creation.

| Requirement | Phase | Status |
|-------------|-------|--------|
| WIZ-01 | Phase 4 | Complete |
| WIZ-02 | Phase 4 | Complete |
| WIZ-03 | Phase 4 | Complete |
| TELE-01 | Phase 5 | Complete |
| TELE-02 | Phase 5 | Complete |
| TELE-03 | Phase 5 | Complete |
| CONF-01 | Phase 6 | Complete |
| CONF-02 | Phase 6 | Pending |
| CONF-03 | Phase 6 | Pending |
| CONF-04 | Phase 6 | Pending |
| TEST-01 | Phase 7 | Pending |
| INST-01 | Phase 7 | Pending |

**Coverage:**
- v1.1 requirements: 12 total
- Mapped to phases: 12
- Unmapped: 0

---
*Requirements defined: 2026-03-24*
*Last updated: 2026-03-24 after roadmap creation*
