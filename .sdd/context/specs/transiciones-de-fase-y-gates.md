---
schema_version: "0.1"
kind: reconstructed-spec
id: "transiciones-de-fase-y-gates"
capability: "begin / deliver / approve / reject"
status: reconstruida-desde-codigo
created_at: "2026-09-22"
created_by: { kind: "agent", id: "sdd-doc-subagent" }
sources:
  - src/cli/cmd/begin.go
  - src/cli/cmd/deliver.go
  - src/cli/cmd/approve.go
  - src/cli/cmd/reject.go
  - src/cli/internal/usecases/begin_phase_uc.go
  - src/cli/internal/usecases/deliver_phase_uc.go
  - src/cli/internal/usecases/approve_uc.go
  - src/cli/internal/usecases/reject_uc.go
  - src/cli/internal/usecases/transition_helpers.go
  - src/cli/internal/domain/work_item.go
  - src/cli/internal/domain/work_item_validation.go
---

# Especificación: Transiciones de fase y gates humanos (`begin` / `deliver` / `approve` / `reject`)

> Especificación reconstruida desde el código fuente durante el onboarding del motor
> `sdd-cli`. La fuente de verdad prioritaria es el código.

## Requisitos y comportamiento

### Propósito

Estos cuatro comandos implementan la **máquina de estados de fases** de un work item. El
agente hace `begin` y `deliver`; la persona (actor `human`) resuelve los gates con `approve`
o `reject`. Cada transición valida las invariantes del dominio, prepara los artefactos de las
fases que se desbloquean, emite eventos y commitea de forma atómica.

### Ciclo de estados de una fase (`PhaseStatus`)

```
blocked → ready (dependencias satisfechas)
ready → in_progress (begin)
in_progress → awaiting_approval (deliver, gate required/optional+--request-approval)
in_progress → completed (deliver, gate none / optional sin approval)
awaiting_approval → approved (approve, humano)
awaiting_approval → rejected (reject, humano)
rejected → in_progress (begin, retrabajo)
approved|accepted → completed (complete --phase; ver spec de cierre)
```

Estados adicionales: `not_applicable` (ancestros en entrada externa), `accepted` (fase de
entrada externa sin gate), `superseded` (modelado; `begin` lo acepta como reinicio pero no hay
comando que lo produzca en la BETA).

### `begin`

- Flags: `--phase`/`-p` (requerido), `--actor-kind` (default `agent`), `--actor-id`
  (default `agent`), `--operation-id`.
- `WorkItem.BeginPhase`: la fase debe estar en `ready`, `rejected` o `superseded`; pasa a
  `in_progress`. Requiere `canMutatePhase` (item `active`, o `completed` si la fase es
  `optional`). Evento `phase.transitioned` (cause `phase_begun`).

### `deliver`

- Flags: `--phase`/`-p` (requerido), `--request-approval` (bool), `--actor-kind`/`--actor-id`
  (default `agent`/`agent`), `--operation-id`.
- `WorkItem.DeliverPhase`: la fase debe estar `in_progress`. El destino depende de la política
  de aprobación de la fase:
  - `required` → `awaiting_approval` (+ `Approval` pending; evento `approval.requested`).
  - `optional` → `completed`, o `awaiting_approval` si `--request-approval` (+ pending +
    `approval.requested`).
  - `none` → `completed`; usar `--request-approval` aquí → `ErrApprovalNotAllowed`.
- Al pasar a `completed`, **desbloquea** las fases cuyas dependencias quedan satisfechas
  (`unlockReadyPhases`: `blocked → ready`) y **prepara sus templates** en el mismo commit
  (`prepareArtifactsForTransitions`, solo para transiciones a `ready`).
- Eventos: `phase.transitioned` (cause `phase_delivered`) + transiciones de desbloqueo
  (cause `dependencies_satisfied`) + `approval.requested` si corresponde.

### `approve`

- Flags: `--phase`/`-p` (requerido), `--by`/`-b` (default `human`), `--comment`/`-c`,
  `--operation-id`. El actor se fuerza a `kind: human`.
- `WorkItem.ApprovePhase`: exige **actor `human`** (`ErrHumanActorRequired`), política
  distinta de `none` (`ErrApprovalNotAllowed`) y estado `awaiting_approval`
  (`ErrPhaseNotAwaitingApproval`). Pasa a `approved`, resuelve el `Approval` pending
  (status `approved`, `by`, `at` RFC3339, `comment`), **desbloquea** dependientes y prepara
  sus templates.
- Eventos: `approval.recorded` (`{phase, status: approved, comment}`) + `phase.transitioned`
  (cause `approval_recorded`) + desbloqueos.

### `reject`

- Flags: idénticos a `approve` (`--by` default `human`); actor forzado a `human`.
- `WorkItem.RejectPhase`: mismas precondiciones (humano, política ≠ none,
  `awaiting_approval`). Pasa a `rejected`, resuelve el `Approval` pending como `rejected`.
  **No** desbloquea nada. El retrabajo se hace con `begin` (`rejected → in_progress`), sin
  borrar el historial de aprobaciones.
- Eventos: `approval.recorded` (`{status: rejected}`) + `phase.transitioned`
  (cause `approval_rejected`).

### Idempotencia y salida

- Los cuatro comandos son idempotentes vía `--operation-id`: si ya fue aplicado (existe un
  evento con ese `correlation_id`), devuelven el item actual sin re-transicionar.
- Salida: JSON con el `WorkItem` actualizado; texto con un mensaje por comando
  (ej. `Phase 'X' started/delivered ...`, `Successfully approved/rejected ...`).

## Escenarios verificables

- `begin`→`deliver` en fase con gate `required` → queda `awaiting_approval` con `Approval`
  pending y evento `approval.requested`.
- `approve` por actor `agent` → `ErrHumanActorRequired` (un agente no aprueba su trabajo).
- `approve` de una fase con gate → desbloquea la fase siguiente a `ready` y renderiza su template.
- `reject`→`begin` → la fase vuelve a `in_progress`; el histórico conserva el rechazo.
- `deliver --request-approval` en fase `none` → `ErrApprovalNotAllowed`.
- Reintento con el mismo `--operation-id` → sin efecto duplicado.

## Reglas, contratos y restricciones

- Ninguna fase se saltea: solo se transiciona desde estados permitidos (`invalidPhaseTransition`).
- Gate humano: `approve`/`reject` exigen actor `human` y fase `awaiting_approval`.
- `canMutatePhase`: item `active`, o `completed` solo para fases `optional`.
- Una sola aprobación `pending` por fase (validado por `work_item_validation.go`).
- Las decisiones (`approved`/`rejected`) requieren `by` humano y `at` RFC3339Nano válido.
- Los desbloqueos preparan artefactos solo para transiciones a `ready`.

## Errores y códigos (envelope JSON)

- `ErrInvalidTransition`, `ErrPhaseNotAwaitingApproval`, `ErrApprovalNotAllowed`,
  `ErrHumanActorRequired` → `invalid_transition`.
- `ErrPhaseNotFound` → `not_found`; `ErrConcurrentModification` → `concurrent_modification`;
  `ErrWorkItemLocked` → `work_item_locked`; actor inválido → `invalid_input`.

## Dependencias con otras capacidades

- Requiere un work item creado por `start`.
- El paso `approved|accepted → completed` y el cierre lo cubre la spec de cierre.
- Comparte persistencia transaccional, locking y revisión optimista (ver `fs_repository.go`).

## Deltas sobre baseline

No aplica (especificación reconstruida).

- **Pendiente/Desconocido**: `superseded` es aceptado como estado de partida en `begin`, pero
  no existe comando que lleve una fase a `superseded` (retrabajo semántico / invalidación
  transitoria fuera de alcance en la BETA).

## Trazabilidad

- `src/cli/cmd/{begin,deliver,approve,reject}.go` — flags, defaults de actor, salida.
- `src/cli/internal/usecases/{begin_phase_uc,deliver_phase_uc,approve_uc,reject_uc}.go`.
- `src/cli/internal/usecases/transition_helpers.go` — carga, preparación de artefactos, eventos, commit.
- `src/cli/internal/domain/work_item.go` — `BeginPhase`/`DeliverPhase`/`ApprovePhase`/`RejectPhase`, `unlockReadyPhases`.
- `src/cli/internal/domain/work_item_validation.go` — invariantes de aprobación y de estado.
