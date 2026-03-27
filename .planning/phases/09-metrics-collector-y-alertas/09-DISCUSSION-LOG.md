# Phase 9: Metrics Collector y Alertas - Discussion Log

> **Audit trail only.** Do not use as input to planning, research, or execution agents.
> Decisions are captured in CONTEXT.md — this log preserves the alternatives considered.

**Date:** 2026-03-27
**Phase:** 09-metrics-collector-y-alertas
**Areas discussed:** Scheduling del collector, gopsutil vs shell commands, Formato de alertas Telegram, Comportamiento en restart

---

## Scheduling del collector

### Estrategia de scheduling

| Option | Description | Selected |
|--------|-------------|----------|
| Goroutine por métrica | Cada métrica tiene su propia goroutine con ticker independiente. Simple, aislado, escala bien para 5-20 métricas. | ✓ |
| Scheduler centralizado | Un solo loop con tick mínimo que revisa cuáles métricas toca ejecutar. Más control pero más complejidad. | |
| Vos decidí | Claude elige la estrategia que mejor se adapte al codebase. | |

**User's choice:** Goroutine por métrica (Recomendado)
**Notes:** Consistente con el patrón del dispatcher existente.

### Arranque del collector

| Option | Description | Selected |
|--------|-------------|----------|
| Automático si hay config | Si cfg.Metrics != nil, arranca el collector. Cero fricción. | ✓ |
| Flag --no-metrics | Arrancar siempre pero permitir deshabilitarlo con flag. | |
| Vos decidí | Claude elige. | |

**User's choice:** Automático si hay config (Recomendado)
**Notes:** Ninguna.

---

## gopsutil vs shell commands

### Mecanismo de ejecución de métricas predefinidas

| Option | Description | Selected |
|--------|-------------|----------|
| Solo shell commands | Descartar gopsutil. Todas las métricas son shell commands en config.yaml. Más simple, sin dependencia extra. | ✓ |
| gopsutil con fallback a command | Si command vacío y name matchea predefinida, usa gopsutil. Si tiene command, ejecuta shell. | |
| Solo gopsutil para predefinidas | Las 5 predefinidas siempre usan gopsutil, custom siempre shell. | |
| Vos decidí | Claude elige. | |

**User's choice:** Solo shell commands (Recomendado)
**Notes:** Fiel a D-02 de Phase 8. Se descarta la decisión de investigación sobre gopsutil/v4.

### Shell interpreter

| Option | Description | Selected |
|--------|-------------|----------|
| sh -c | POSIX sh, portable a cualquier Linux. | ✓ |
| bash -c | Permite bash-isms pero puede no estar en containers mínimos. | |
| Vos decidí | Claude elige. | |

**User's choice:** sh -c (Recomendado)
**Notes:** Ninguna.

---

## Formato de alertas Telegram

### Canal de destino

| Option | Description | Selected |
|--------|-------------|----------|
| Canal 'general' | Todas las alertas al canal general. Simple, sin canal extra. | ✓ |
| Canal configurable por métrica | Campo 'channel' opcional en MetricDef. | |
| Canal 'metrics' dedicado | Canal fijo separado para alertas de métricas. | |

**User's choice:** Canal 'general' (Recomendado)
**Notes:** Ninguna.

### Prioridad del mensaje

| Option | Description | Selected |
|--------|-------------|----------|
| Mapear umbral a prioridad | warning → high (⚠️), critical → critical (🔴). | ✓ |
| Siempre 'critical' | Todas las alertas como critical. | |
| Vos decidí | Claude elige. | |

**User's choice:** Mapear umbral a prioridad (Recomendado)
**Notes:** Aprovecha el sistema de emojis existente del dispatcher.

### Contenido del mensaje

| Option | Description | Selected |
|--------|-------------|----------|
| Compacto con valor y umbral | Título con nombre+transición, body con valor, umbral, host. | ✓ |
| Mínimo solo nombre y estado | Solo qué métrica cambió de estado. | |
| Detallado con historial | Valor actual, umbral, últimos 3 valores, tendencia. | |

**User's choice:** Compacto con valor y umbral (Recomendado)
**Notes:** Ninguna.

---

## Comportamiento en restart

### Estado de alertas post-restart

| Option | Description | Selected |
|--------|-------------|----------|
| Rehidratar desde DB | Leer last_status de tabla metrics al arrancar. Evita alertas duplicadas. | ✓ |
| Aceptar alerta duplicada | Arrancar en 'ok' siempre. Raro y mejor pecar de más alertas. | |
| Grace period post-restart | Esperar 2 ciclos antes de evaluar umbrales. | |

**User's choice:** Rehidratar desde DB (Recomendado)
**Notes:** La tabla metrics ya tiene el campo last_status.

### Persistencia del estado

| Option | Description | Selected |
|--------|-------------|----------|
| En cada lectura | Actualizar last_value y last_status en cada poll. Siempre consistente. | ✓ |
| Solo en transición de estado | Escribir solo cuando cambia. Menos writes pero riesgo de inconsistencia ante crash. | |
| Vos decidí | Claude elige. | |

**User's choice:** En cada lectura (Recomendado)
**Notes:** UpdateMetricStatus() ya existe desde Phase 8.

---

## Claude's Discretion

- Estructura interna del paquete internal/collector/
- Formato exacto del hostname en alertas
- Manejo de errores internos del collector
- Detalles del flujo collect-then-write

## Deferred Ideas

Ninguna — la discusión se mantuvo dentro del scope de la fase.
