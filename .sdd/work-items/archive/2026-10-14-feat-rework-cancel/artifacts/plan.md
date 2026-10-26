---
schema_version: "1.0"
kind: artifact
id: "plan"
work_item: "feat-rework-cancel"
phase: "plan"
status: draft
created_at: "2026-10-07T02:13:23Z"
created_by: { kind: "cli", id: "sdd" }
sources:
  - artifacts/specification.md
---

# Plan de implementación: Comandos `rework` y `cancel` para work items

## Enfoque y decisiones

### Resumen y contexto

Se exponen dos mecanismos de control de ciclo de vida ya modelados en el dominio pero sin
interfaz pública: `rework` (retomar un work item desde una fase anterior invalidando el trabajo
descendiente) y `cancel` (cerrar formalmente un work item abandonado, reservando su ID). El
trabajo se traza íntegramente a la especificación aprobada (`artifacts/specification.md`) y al
PRD (`artifacts/prd.md`).

El motor es una máquina de estados sobre el filesystem construida con Clean Architecture en Go:

- **Dominio** (`internal/domain/`): entidades y máquina de estados pura, sin I/O.
- **Ports** (`internal/ports/`): interfaces (repositorios, clock, id generator).
- **Use cases** (`internal/usecases/`): orquestación (idempotencia → dominio → commit).
- **Infra** (`internal/infra/`): persistencia transaccional en `.sdd/`.
- **CLI** (`cmd/`): handlers Cobra + composition root + mapeo de errores a códigos JSON.

Ambos comandos extienden patrones ya existentes:

- `rework` es un **mutador in-place** (el expediente permanece en `active/`): sigue exactamente
  el patrón de `begin`/`complete` (cargar → idempotencia → método de dominio → `commitWorkItem`).
- `cancel` es un **mutador con movimiento físico transaccional**: sigue el patrón de `archive`
  (`ArchiveUseCase` + `fs_archive_repository.go`), con destino `cancelled/<id>/` (sin prefijo
  de fecha) y marker `.cancel.json`.

### Áreas afectadas

| Capa | Archivos nuevos | Archivos modificados |
|---|---|---|
| Dominio | — | `work_item.go`, `errors.go`, `workflow_validation.go`, `work_item_validation.go` |
| Ports | — | `repository.go` |
| Use cases | `rework_uc.go`, `cancel_uc.go` | `transition_helpers.go` (helper nuevo), `start_uc.go`, `next_uc.go`, `validate_uc.go` |
| Infra | `fs_cancel_repository.go` | `fs_repository.go` (`WorkItemExists`, `recoverIfNeeded`, `FindWorkItem`) |
| CLI | `cmd/rework.go`, `cmd/cancel.go` | `composition.go`, `root.go` |
| Tests | — | `workflow_lifecycle_test.go`, `cli_e2e_test.go`, tests de dominio |

Dependencias externas: ninguna nueva. No cambia ningún JSON Schema (`work-item.schema.json`
ya admite `status: cancelled`; `event.schema.json` valida `type` por patrón abierto, admite
`work_item.reactivated` y `work_item.cancelled`). Confirmado en `data-modeling.md` y en la spec.

### Esfuerzo estimado

**Alto.** Justificación: toca cuatro capas, introduce un segundo repositorio transaccional con
recovery (el código más delicado del motor), un cambio sutil pero crítico en la validación de
invariantes, y requiere cobertura de tests en tres niveles (dominio, ciclo de vida, E2E).

### Decisiones de diseño

**DD-1. `rework` NO usa `canMutatePhase`; valida sus propias precondiciones.**
`canMutatePhase` (work_item.go:482) solo permite mutar fases cuando el item está `active`, o
`completed` si la fase es `optional`. `rework` debe operar sobre items `completed` y sobre fases
no opcionales (reactivación, RB-RW-05), por lo que no puede reutilizar ese guard. `ReworkPhase`
implementa su propia cadena de validación (estado del item, existencia y estado de la fase).
Alternativa descartada: relajar `canMutatePhase` — rompería invariantes de `begin`/`deliver`.

**DD-2. Nueva estructura `ReworkMutation` en lugar de reutilizar `PhaseMutation`.**
`PhaseMutation{Transition, Unblocked}` modela "una transición + desbloqueos a `ready`" con una
sola causa. `rework` produce transiciones heterogéneas: la fase objetivo (`→ ready`, causa
`rework_requested`) y N descendientes (`→ superseded`, causa `rework_superseded`), más un posible
cambio de estado del work item. Mezclarlas en `PhaseMutation` forzaría al helper
`phaseMutationEvents` a adivinar causas. Se introduce un tipo cohesivo `ReworkMutation` que
preserva la distinción semántica (SRP). Alternativa descartada: sobrecargar `phaseMutationEvents`
con flags — aumentaría su complejidad ciclomática y acoplaría causas no relacionadas.

**DD-3. `cancel` como repositorio separado `fs_cancel_repository.go`, no dentro de `fs_archive_repository.go`.**
Aunque el mecanismo es análogo (AS-03), son responsabilidades distintas con destinos y markers
distintos. Un archivo separado mantiene cohesión (SRP) y evita inflar un archivo ya extenso.
Se **reutilizan** las primitivas compartidas ya presentes en `fs_repository.go`
(`copyDirectory`, `appendStagedEvents`, `writeAndSync`, `syncDirectory`, `syncTree`,
`recoverWorkItemTransaction`, `lockWorkItem`, `eventOperationExists`, `containedPath`,
`pathExists`). No se duplica esa lógica. Alternativa descartada: factorizar un "mover
transaccional genérico" compartido entre archive y cancel — es un refactor de mayor alcance,
riesgoso sobre el código más delicado del motor, y queda fuera del scope de esta feature
(se documenta como deuda futura, ver Riesgos).

**DD-4. El `reason` viaja en los eventos, no en el `manifest.yaml`.**
No existe campo para `reason` en `WorkItem` ni en `PhaseState`, y el schema no lo contempla.
Igual que `archive` (que no persiste su `reason` en el manifest), el `reason` de `rework`/`cancel`
se registra en `data.reason` de cada evento auditable. Esto satisface INV-06 sin cambiar schema.

**DD-5. Exención de `superseded` en la validación de dependencias (cambio crítico).**
`work_item_validation.go` (líneas 151-159) marca `work_item.dependencies_unsatisfied` para toda
fase que no sea `not_applicable` ni `blocked` cuyas dependencias no estén satisfechas. Tras un
`rework`, un descendiente queda `superseded` mientras su dependencia (la fase objetivo) vuelve a
`ready` (estado que NO satisface dependencia). Sin un cambio, el manifest reworkeado **falla**
`ValidateAgainst`, que se ejecuta dentro de `CommitWorkItem.validateCommit` → el commit aborta.
Se exime `superseded` del chequeo de dependencias, de forma análoga a `blocked`/`not_applicable`.
Esto es correcto por diseño: un `superseded` representa trabajo intencionalmente invalidado cuyas
dependencias fueron reseteadas. Ver Etapa 1, tarea 1.5. Este es el riesgo técnico principal.

**DD-6. `start` con ID cancelado: se reutiliza `ErrWorkItemAlreadyExists` con mensaje diferenciado.**
No se crea un error nuevo. `WorkItemExists` se extiende para detectar `cancelled/` (reserva el ID,
RB-CA-06). Para el mensaje diferenciado (CA-CA-05), `StartWorkItemUseCase` consulta si el ID está
en `cancelled/` y devuelve `fmt.Errorf("%w: ... id reservado por cancelación", ErrWorkItemAlreadyExists, id)`.
`errors.Is` sigue resolviendo a código `already_exists` (root.go:144) sin tocar el mapeo.

**DD-7. Opacidad de lectura (DD-01 del PRD): nuevo sentinel `ErrWorkItemCancelled`.**
`status`, `next`, `validate <id>` deben devolver un error diagnóstico, no el contenido. Se
introduce `ErrWorkItemCancelled` (dominio) mapeado a `not_found` en root.go (la spec admite
`not_found` o `invalid_transition` con mensaje). El contenido del expediente cancelado nunca se
expone por la interfaz pública (INV-05).

### Cumplimiento SOLID / Clean Architecture / Clean Code

Validado para cada archivo nuevo y cada archivo modificado:

- **SRP**: cada use case orquesta una operación; `fs_cancel_repository.go` encapsula solo el
  movimiento a `cancelled/`; `ReworkMutation` modela solo el resultado del retrabajo.
- **OCP/DIP**: los use cases dependen de nuevas interfaces de puerto (`WorkItemCanceller`,
  `CancelledWorkItemChecker`), no de implementaciones concretas. La infra las satisface.
- **ISP**: se definen interfaces pequeñas y específicas (checker de existencia separado del
  mutador), evitando "fat interfaces".
- **Reuse before creating**: no se crea ninguna primitiva de filesystem nueva; se reutilizan las
  de `fs_repository.go`. No se crean helpers huérfanos. Los comandos Cobra siguen el mismo molde
  que `archive.go`/`begin.go`.
- **Patrones**: no se introduce ningún patrón nuevo "por defecto". Se continúan los patrones ya
  presentes (repository, use case, composition root, transacción staging+rename+marker+recovery).

---

## Pasos de implementación

> Las dos funcionalidades son en gran medida **tracks independientes**: `rework` (DD-2, Etapas
> 1A y 3A) y `cancel` (Etapas 1B, 2 y 3B) solo comparten `errors.go`, `composition.go` y
> `root.go`. Pueden delegarse a dos agentes desarrolladores en paralelo si se coordina el cierre
> en la Etapa 4. En ejecución secuencial, seguir el orden de etapas tal cual.
>
> **Nota transversal sobre `Revision`**: los métodos de dominio NO incrementan `item.Revision`.
> El incremento (+1) lo realiza la capa de infra en el commit (`CommitWorkItem` /
> `CancelWorkItem`, `persistedItem.Revision = expectedRevision + 1`). No incrementar manualmente
> en el dominio ni en el use case para evitar doble incremento.

### Etapa 1A — Dominio de `rework`

Objetivo: máquina de estados pura de `rework`, sin I/O. Al finalizar, los tests unitarios de
`ReworkPhase` pasan.

- **1A.1** `internal/domain/errors.go`: agregar sentinels.
  ```go
  ErrWorkItemCannotCancel = errors.New("work item cannot be cancelled")
  ErrWorkItemCancelled    = errors.New("work item was cancelled")
  ```
  (El primero pertenece al track de cancel pero se agrega aquí para no tocar el archivo dos veces
  si se ejecuta secuencialmente; si se trabaja en paralelo, coordinar este archivo.)

- **1A.2** `internal/domain/workflow.go` o `workflow_validation.go`: agregar `Descendants`,
  espejo de `Ancestors` (workflow_validation.go:320) pero recorriendo hijos (fases que declaran
  `requires` hacia el nodo). Colocar junto a `Ancestors` por cohesión.
  ```go
  func (workflow *Workflow) Descendants(phaseID string) (map[string]struct{}, error) {
      if _, exists := workflow.Phase(phaseID); !exists {
          return nil, ErrPhaseNotFound
      }
      children := make(map[string][]string, len(workflow.Phases))
      for _, phase := range workflow.Phases {
          for _, requiredID := range phase.Requires {
              children[requiredID] = append(children[requiredID], phase.ID)
          }
      }
      descendants := make(map[string]struct{})
      var visit func(string)
      visit = func(currentID string) {
          for _, childID := range children[currentID] {
              if _, seen := descendants[childID]; seen {
                  continue
              }
              descendants[childID] = struct{}{}
              visit(childID)
          }
      }
      visit(phaseID)
      return descendants, nil
  }
  ```

- **1A.3** `internal/domain/work_item.go`: agregar el tipo `ReworkMutation` junto a
  `PhaseMutation` (work_item.go:113).
  ```go
  type ReworkMutation struct {
      Reactivated bool              // true si el work item pasó de completed a active
      Target      PhaseTransition   // fase objetivo -> ready
      Superseded  []PhaseTransition // descendientes -> superseded (orden topológico)
  }
  ```

- **1A.4** `internal/domain/work_item.go`: implementar `ReworkPhase`. Precondiciones PC-RW-02,
  PC-RW-03, PC-RW-04, PC-RW-05 (PC-RW-01 la resuelve el use case al cargar; PC-RW-06 la resuelve
  Cobra). Emite las transiciones vía `item.transition` (reutiliza el helper existente, que fija
  `From` desde el estado actual). Propagación en orden topológico usando `OrderedPhases`.
  ```go
  func (item *WorkItem) ReworkPhase(workflow *Workflow, phaseID string) (ReworkMutation, error) {
      if workflow == nil {
          return ReworkMutation{}, fmt.Errorf("%w: workflow is required", ErrInvalidWorkItem)
      }
      if item.Status == WorkItemArchived || item.Status == WorkItemCancelled {
          return ReworkMutation{}, fmt.Errorf("%w: work item status is %s", ErrInvalidTransition, item.Status)
      }
      if item.Status != WorkItemActive && item.Status != WorkItemCompleted {
          return ReworkMutation{}, fmt.Errorf("%w: work item status is %s", ErrInvalidTransition, item.Status)
      }
      _, state, err := item.phase(workflow, phaseID) // ErrPhaseNotFound si no existe
      if err != nil {
          return ReworkMutation{}, err
      }
      if state.Status != PhaseCompleted && state.Status != PhaseApproved && state.Status != PhaseAccepted {
          return ReworkMutation{}, invalidPhaseTransition(phaseID, state.Status, PhaseReady)
      }

      var mutation ReworkMutation
      if item.Status == WorkItemCompleted {
          item.Status = WorkItemActive
          mutation.Reactivated = true
      }
      mutation.Target = item.transition(phaseID, PhaseReady).Transition

      descendants, err := workflow.Descendants(phaseID)
      if err != nil {
          return ReworkMutation{}, err
      }
      ordered, err := workflow.OrderedPhases()
      if err != nil {
          return ReworkMutation{}, err
      }
      for _, phase := range ordered {
          if _, isDescendant := descendants[phase.ID]; !isDescendant {
              continue
          }
          current := item.Phases[phase.ID].Status
          switch current {
          case PhaseNotApplicable, PhaseBlocked, PhaseSuperseded:
              continue // sin cambio (CA-RW-09 / CA-RW-11)
          case PhaseReady:
              // Desbloqueada pero no iniciada → retrocede a blocked (RB-RW-02)
              mutation.Superseded = append(mutation.Superseded, item.transition(phase.ID, PhaseBlocked).Transition)
          default:
              // Fase efectivamente trabajada → superseded (RB-RW-02)
              mutation.Superseded = append(mutation.Superseded, item.transition(phase.ID, PhaseSuperseded).Transition)
          }
      }
      return mutation, nil
  }
  ```
  Nota: el campo `Superseded` del `ReworkMutation` contiene tanto las transiciones a `superseded`
  como la de `ready → blocked`; el nombre del campo es representativo, no exclusivo. Los `Approval`
  records NO se tocan (RB-RW-03 / CA-RW-07). La fase objetivo deja su `Approval` previo intacto;
  al estar en `ready`, `approvalViolations` no exige coherencia (ver Etapa 1A.5 para por qué valida).

- **1A.5** `internal/domain/work_item_validation.go`: **cambio crítico (DD-5)**. En el loop
  por fase (`ViolationsAgainst`), agregar la exención de `superseded` inmediatamente después del
  bloque de `PhaseBlocked` (línea ~150) y antes del chequeo `!isEntry && !dependenciesSatisfied`:
  ```go
  if state.Status == PhaseSuperseded {
      continue
  }
  ```
  Esto exime a `superseded` del chequeo de dependencias, del chequeo de estado de aprobación y
  del de `accepted`, de forma coherente con `blocked`/`not_applicable`. Verificar que ningún test
  existente dependa de que un `superseded` dispare `dependencies_unsatisfied` (no debería haber
  ninguno; `superseded` no se producía antes de esta feature).

- **1A.6** Tests de dominio (nuevo archivo `internal/domain/work_item_rework_test.go`): tabla de
  estados válidos/inválidos (ver Estrategia de pruebas, bloque A).

Resultado esperado: `go test ./internal/domain/...` pasa; `ReworkPhase` cubre ESC-RW-01/03/07/08/09/11.

### Etapa 1B — Dominio de `cancel`

Objetivo: método de dominio `Cancel`. Independiente de 1A salvo por `errors.go`.

- **1B.1** `internal/domain/work_item.go`: implementar `Cancel` (espejo minimal de `Archive`,
  pero sin workflow ni elegibilidad de fases: `cancel` no valida fases, RB-CA-04 / CA-CA-06).
  ```go
  func (item *WorkItem) Cancel() error {
      if item.Status != WorkItemActive {
          return fmt.Errorf("%w: current status is %s", ErrWorkItemCannotCancel, item.Status)
      }
      item.Status = WorkItemCancelled
      return nil
  }
  ```
  No modifica `item.Phases` (POST-CA-03). No persiste `reason` (DD-4).

- **1B.2** Test de dominio (`work_item_cancel_test.go`): `active` → OK; `completed`/`archived`/
  `cancelled` → `ErrWorkItemCannotCancel`.

Resultado esperado: `Cancel` cubierto a nivel unitario (CA-CA-02/03).

### Etapa 2 — Ports e infraestructura de `cancel`

Objetivo: persistencia transaccional del movimiento a `cancelled/` y detección del directorio en
los comandos de lectura. Depende de 1B (usa `ErrWorkItemCannotCancel`, `ErrWorkItemCancelled`,
`WorkItemCancelled`).

- **2.1** `internal/ports/repository.go`:
  - Agregar la ubicación: `WorkItemLocationCancelled WorkItemLocation = "cancelled"`.
  - Agregar el commit de cancelación (sin fecha):
    ```go
    type WorkItemCancelCommit struct {
        Item        *domain.WorkItem
        Event       domain.Event
        OperationID string
    }
    ```
  - Agregar las interfaces (ISP — pequeñas y específicas):
    ```go
    type CancelledWorkItemChecker interface {
        WorkItemCancelledExists(baseDir string, id string) (bool, error)
    }

    type WorkItemCanceller interface {
        CancelledWorkItemChecker
        FindCancelledWorkItem(baseDir string, id string) (*LocatedWorkItem, error)
        CancelledOperationApplied(baseDir string, id string, operationID string) (bool, error)
        CancelWorkItem(baseDir string, commit WorkItemCancelCommit) (*LocatedWorkItem, error)
    }
    ```

- **2.2** `internal/infra/fs_cancel_repository.go` (nuevo). Modelar sobre
  `fs_archive_repository.go`, adaptando destino y marker. Métodos (como receptores de
  `*FSWorkItemRepository`, igual que archive):
  - `cancelDestination(id)` → nombre `<id>` y relativo `.sdd/work-items/cancelled/<id>`
    (sin fecha; validar identifier).
  - `WorkItemCancelledExists(baseDir, id)` → `pathExists(cancelled/<id>/manifest.yaml)`.
  - `FindCancelledWorkItem(baseDir, id)` → `recoverIfNeeded` + `findCancelledWorkItemNoRecovery`;
    lee `cancelled/<id>/manifest.yaml`, exige `status == WorkItemCancelled` (si no, `ErrArchiveConflict`
    o un error de consistencia), retorna `LocatedWorkItem{Location: WorkItemLocationCancelled}`.
  - `CancelledOperationApplied(baseDir, id, opID)` → `eventOperationExists` sobre el `events.jsonl`
    del expediente cancelado.
  - `CancelWorkItem(baseDir, commit)` → réplica de `ArchiveWorkItem` (fs_archive_repository.go:181)
    con estas diferencias:
    - destino `cancelled/<id>` (crear `cancelled/` con `os.MkdirAll` si no existe — RB-CA-05;
      el directorio no se crea en `init`, a diferencia de `active/`/`archive/`).
    - marker `.transactions/<id>.cancel.json` (struct análogo a `archiveTransaction`).
    - stage/backup: `.transactions/<id>.cancel-stage` / `.cancel-backup`.
    - precondición de estado: `current.Status == WorkItemActive` (no `completed`); `commit.Item.Status
      == WorkItemCancelled`.
    - validación del stage: NO hay gate de artefacto (a diferencia de `validateArchiveStage`); basta
      `validateCommit` del manifest + evento.
    - validar el evento: `type == "work_item.cancelled"`, `work_item == id`, `correlation_id ==
      operationID`, `data.from == "active"`, `data.to == "cancelled"`, `data.reason` no vacío.
    - reutilizar `copyDirectory`, `appendStagedEvents`, `writeAndSync`, `syncDirectory`, `syncTree`,
      `lockWorkItem`, `recoverWorkItemTransaction`, los hooks de commit y la mecánica de
      rollback/rename de archive (adaptando nombres de hook, p. ej. `cancel_before_active_backup`).
  - `recoverCancelIfNeeded(baseDir, id)` y `recoverCancelTransactionLocked(baseDir, id)` →
    réplica de `recoverArchiveIfNeeded`/`recoverArchiveTransactionLocked`
    (fs_archive_repository.go:464-599) con el marker `.cancel.json` y destino `cancelled/<id>`.
    Cubre ESC-CA-08 / CA-CA-08.
  - Declarar asserts de interfaz al pie: `var _ ports.WorkItemCanceller = (*FSWorkItemRepository)(nil)`.

- **2.3** `internal/infra/fs_repository.go`:
  - `recoverIfNeeded` (línea 282): agregar `if err := r.recoverCancelIfNeeded(baseDir, id); err != nil { return err }`
    junto a la llamada a `recoverArchiveIfNeeded`.
  - `WorkItemExists` (línea 52): además de active y archive, consultar `cancelled/` para reservar
    el ID (RB-CA-06 / INV-01). Si el ID aparece en más de una ubicación → `ErrArchiveConflict`.
    Retornar `true` si existe en cualquiera de las tres.
  - `FindWorkItem` (fs_archive_repository.go:27): tras resolver active/archive, si el ID está solo
    en `cancelled/`, retornar el sentinel `domain.ErrWorkItemCancelled` (opacidad DD-7 / INV-05).
    No retornar el contenido. Mantener la detección de conflicto (active + cancelled simultáneos).

Resultado esperado: `go test ./internal/infra/...` compila; el movimiento transaccional y su
recovery funcionan (se validan vía E2E en Etapa 5).

### Etapa 3A — Use case y comando de `rework`

Objetivo: orquestación de `rework`. Depende de 1A.

- **3A.1** `internal/usecases/transition_helpers.go`: agregar un helper de eventos de rework
  (reuso de `newOperationEvent`, sin tocar `phaseMutationEvents`). Construye, en orden: evento
  `work_item.reactivated` (si `mutation.Reactivated`), luego la fase objetivo (`rework_requested`),
  luego cada `superseded` (`rework_superseded`). Todos con `reason` (INV-06).
  ```go
  func reworkEvents(
      workItemID string, mutation domain.ReworkMutation, actor domain.Actor,
      reason, operationID string, clock ports.Clock, idGenerator ports.IDGenerator,
  ) ([]domain.Event, error) {
      var events []domain.Event
      if mutation.Reactivated {
          ev, err := newOperationEvent(workItemID, "work_item.reactivated", actor,
              map[string]interface{}{"from": "completed", "to": "active", "reason": reason},
              operationID, clock, idGenerator)
          if err != nil { return nil, err }
          events = append(events, ev)
      }
      target := mutation.Target
      ev, err := newOperationEvent(workItemID, "phase.transitioned", actor,
          map[string]interface{}{"phase": target.Phase, "from": target.From, "to": target.To,
              "cause": "rework_requested", "reason": reason},
          operationID, clock, idGenerator)
      if err != nil { return nil, err }
      events = append(events, ev)
      for _, t := range mutation.Superseded {
          ev, err := newOperationEvent(workItemID, "phase.transitioned", actor,
              map[string]interface{}{"phase": t.Phase, "from": t.From, "to": t.To,
                  "cause": "rework_superseded", "reason": reason},
              operationID, clock, idGenerator)
          if err != nil { return nil, err }
          events = append(events, ev)
      }
      return events, nil
  }
  ```

- **3A.2** `internal/usecases/rework_uc.go` (nuevo). Patrón de `begin_phase_uc.go`:
  `ReworkInput{WorkItemID, PhaseID, Reason, Actor, OperationID}`. `ReworkUseCase` con deps
  `workItemRepo ports.WorkItemMutationRepository`, `workflowRepo`, `clock`, `idGenerator`.
  `Execute`:
  1. `ValidateActor`; validar `Reason` no vacío (`ErrInvalidWorkItem`/`ErrInvalidActor` — ver
     nota de error abajo); `ValidateIdentifier`.
  2. `loadWorkItemAndWorkflow` (ErrWorkItemNotFound → not_found; cubre PC-RW-01).
  3. `operationApplied` → si aplicado, retornar item (CA-RW-12 / ESC-RW-10).
  4. `item.ReworkPhase(workflow, input.PhaseID)` (dominio).
  5. `reworkEvents(...)`.
  6. `commitWorkItem(baseDir, repo, item, nil, events, operationID)` (sin artifacts; AS-01).
  Nota sobre `--reason` vacío: Cobra marca `--reason` como requerido (ESC-RW-06 devuelve error de
  flag requerido → código `invalid_arguments`). Como defensa en profundidad, el use case también
  valida `strings.TrimSpace(input.Reason) != ""` y devuelve `fmt.Errorf("%w: --reason is required",
  domain.ErrInvalidWorkItem)` → `invalid_input`.

- **3A.3** `cmd/rework.go` (nuevo). Molde de `begin.go`:
  - `Use: "rework <work-item-id>"`, `Args: cobra.ExactArgs(1)`.
  - Flags: `--phase`/`-p` (requerido), `--reason` (requerido), `--actor-kind` (default `agent`),
    `--actor-id` (default `agent`), `--operation-id`. `mustMarkFlagRequired` para `phase` y `reason`.
  - Texto de salida: `Work item '<id>': phase '<fase>' reworked. <n> phase(s) superseded.`
    (si reactivado, anteponer la variante de POST/salida de la spec). El use case puede devolver
    `(*domain.WorkItem, int superseded, bool reactivated)` o el envelope JSON con el `WorkItem`;
    para el conteo, derivarlo de `len(mutation.Superseded)` propagado por el resultado.

Resultado esperado: `sdd-cli rework` end-to-end sobre un item activo y uno completado.

### Etapa 3B — Use case y comando de `cancel` + diagnósticos de lectura

Objetivo: orquestación de `cancel` y comportamiento diagnóstico de `start`/`status`/`next`/
`validate`. Depende de 1B y 2.

- **3B.1** `internal/usecases/cancel_uc.go` (nuevo). Patrón de `archive_uc.go`:
  `CancelInput{WorkItemID, Reason, Actor, OperationID}`,
  `CancelResult{WorkItem *domain.WorkItem, Location ports.WorkItemLocation}` (POST-CA salida JSON).
  `CancelUseCase` deps: `activeItems ports.WorkItemReader`, `canceller ports.WorkItemCanceller`,
  `clock`, `ids`. `Execute`:
  1. `ValidateIdentifier`, `ValidateActor`, `ValidateOperationID`; `Reason` no vacío.
  2. `FindCancelledWorkItem` → si existe y `operationID` aplicado, retornar resultado (idempotencia
     CA-CA-10 / ESC-CA-09); si existe sin coincidir → `ErrWorkItemCannotCancel` (ya cancelado,
     invalid_transition — PC-CA-02).
  3. `activeItems.GetWorkItem` (ErrWorkItemNotFound → not_found; PC-CA-01). No requiere workflow.
  4. `candidate := *item; candidate.Cancel()` para fallar temprano si no está `active` (PC-CA-02),
     luego `item.Cancel()`.
  5. Construir evento `work_item.cancelled` (`newOperationEvent` o `domain.NewEvent`): `data.from =
     "active"`, `data.to = "cancelled"`, `data.reason = reason`, `CorrelationID = operationID`.
  6. `canceller.CancelWorkItem(...)`; si `ErrOperationAlreadyApplied`, resolver con
     `FindCancelledWorkItem` (igual que archive_uc.go:141).
  7. Retornar `CancelResult{Location: WorkItemLocationCancelled}`.
  No se requiere actor `human` (RB-CA-02 / CA-CA-07): no validar `actor.Kind == human`.

- **3B.2** `cmd/cancel.go` (nuevo). Molde de `archive.go`:
  - `Use: "cancel <work-item-id>"`, `Args: cobra.ExactArgs(1)`.
  - Flags: `--reason` (requerido), `--actor-kind` (default `agent`), `--actor-id` (default `agent`),
    `--operation-id`. `mustMarkFlagRequired("reason")`.
  - Texto de salida: `Work item '<id>' cancelled.`; JSON: envelope con `CancelResult`.

- **3B.3** `internal/usecases/start_uc.go`: mensaje diferenciado para ID cancelado (DD-6, CA-CA-05).
  En la rama `if exists` (tras descartar idempotencia, línea ~79), consultar el checker de
  cancelados. Para ello, inyectar `ports.CancelledWorkItemChecker` en `StartWorkItemUseCase`
  (la FS repo ya lo implementa; se pasa `workItems` en composition). Si el ID está en `cancelled/`:
  `return nil, fmt.Errorf("%w: work item '%s' id is reserved by a prior cancellation", domain.ErrWorkItemAlreadyExists, in.ID)`.
  Caso contrario, `ErrWorkItemAlreadyExists` genérico (como hoy).

- **3B.4** `internal/usecases/next_uc.go`: tras `GetWorkItem` fallando con `ErrWorkItemNotFound`,
  consultar el checker de cancelados; si está cancelado, `return nil, fmt.Errorf("%w: work item '%s'
  was cancelled", domain.ErrWorkItemCancelled, id)` (CA-CA-09 / ESC-CA-06). Inyectar
  `ports.CancelledWorkItemChecker` en `NextUseCase`.

- **3B.5** `internal/usecases/validate_uc.go`: en el scope de work item, antes de `InspectWorkItem`,
  consultar el checker de cancelados y devolver `ErrWorkItemCancelled` si aplica. Inyectar
  `ports.CancelledWorkItemChecker` en `ValidateUseCase`.
  (`status` ya queda cubierto por el cambio en `FindWorkItem` de la Etapa 2.3, que devuelve el
  sentinel; no requiere cambio en `status_uc.go`.)

Resultado esperado: `cancel` end-to-end; `start`/`status`/`next`/`validate` responden diagnóstico
sobre IDs cancelados.

### Etapa 4 — Composition root y mapeo de errores

Objetivo: cablear todo y exponer los comandos. Depende de 3A y 3B.

- **4.1** `cmd/composition.go`:
  - Agregar campos `Rework *usecases.ReworkUseCase` y `Cancel *usecases.CancelUseCase` a
    `Application`.
  - Construirlos en `NewProductionApplication`: `Rework` con `(workItems, workflows, clock, ids)`;
    `Cancel` con `(workItems, workItems /*canceller*/, clock, ids)`.
  - Pasar el checker de cancelados a `Start`, `Next`, `Validate` (todos reciben `workItems`, que
    implementa `CancelledWorkItemChecker`). Ajustar las firmas de sus constructores en consecuencia.

- **4.2** `cmd/root.go`:
  - Registrar `newReworkCommand(application.Rework, options)` y
    `newCancelCommand(application.Cancel, options)` en `root.AddCommand(...)`.
  - En `errorCode`: agregar `ErrWorkItemCannotCancel` al grupo `invalid_transition`; mapear
    `ErrWorkItemCancelled` a `not_found` (grupo de not_found). `ErrWorkItemAlreadyExists` ya está
    mapeado a `already_exists` (cubre DD-6).

Resultado esperado: `go build ./...` ok; `sdd-cli rework`/`cancel` visibles y funcionales; todos
los códigos de error del contrato (sección "Contrato de errores" de la spec) correctos.

### Etapa 5 — Tests de integración y E2E

Objetivo: cobertura de ciclo de vida y de binario. Depende de todas las anteriores.
Ver detalle en Estrategia de pruebas (bloques C y D).

---

## Archivos o áreas afectadas

**Nuevos**
- `src/cli/internal/usecases/rework_uc.go`
- `src/cli/internal/usecases/cancel_uc.go`
- `src/cli/internal/infra/fs_cancel_repository.go`
- `src/cli/cmd/rework.go`
- `src/cli/cmd/cancel.go`
- `src/cli/internal/domain/work_item_rework_test.go`
- `src/cli/internal/domain/work_item_cancel_test.go`

**Modificados**
- `src/cli/internal/domain/errors.go` (2 sentinels)
- `src/cli/internal/domain/work_item.go` (`ReworkMutation`, `ReworkPhase`, `Cancel`)
- `src/cli/internal/domain/workflow_validation.go` (`Descendants`)
- `src/cli/internal/domain/work_item_validation.go` (exención `superseded` — DD-5)
- `src/cli/internal/ports/repository.go` (ubicación, commit, interfaces de cancel)
- `src/cli/internal/usecases/transition_helpers.go` (`reworkEvents`)
- `src/cli/internal/usecases/start_uc.go` (mensaje ID cancelado)
- `src/cli/internal/usecases/next_uc.go` (diagnóstico cancelado)
- `src/cli/internal/usecases/validate_uc.go` (diagnóstico cancelado)
- `src/cli/internal/infra/fs_repository.go` (`recoverIfNeeded`, `WorkItemExists`, `FindWorkItem`)
- `src/cli/cmd/composition.go` (wiring)
- `src/cli/cmd/root.go` (registro de comandos, `errorCode`)
- `src/cli/internal/usecases/workflow_lifecycle_test.go`
- `src/cli/cmd/cli_e2e_test.go`

---

## Estrategia de pruebas

### A. Tests unitarios de dominio — `ReworkPhase` (nuevo `work_item_rework_test.go`)

Tabla de estado de la fase objetivo (usar `feature-standard` como workflow de fixture):

| Estado fase objetivo | Resultado esperado |
|---|---|
| `completed` | OK → `ready` |
| `approved` | OK → `ready` |
| `accepted` | OK → `ready` |
| `ready` | `ErrInvalidTransition` |
| `in_progress` | `ErrInvalidTransition` |
| `awaiting_approval` | `ErrInvalidTransition` |
| `blocked` | `ErrInvalidTransition` |
| `rejected` | `ErrInvalidTransition` |
| `not_applicable` | `ErrInvalidTransition` |
| `superseded` | `ErrInvalidTransition` |

Casos adicionales:
- Estado del work item: `active` OK; `completed` → OK con `Reactivated == true`; `archived` y
  `cancelled` → `ErrInvalidTransition`.
- Fase inexistente → `ErrPhaseNotFound` (CA-RW-05).
- Propagación transitiva: con objetivo `plan`, verificar que `implementation`, `verification`,
  `human-code-review`, `archive` quedan `superseded` (CA-RW-01) y que `prd`/`specification` no
  cambian.
- Descendiente ya `superseded` → no reaparece en `mutation.Superseded` (CA-RW-09).
- Descendiente `not_applicable` → no se afecta (CA-RW-11).
- `Approvals` intactos antes/después (CA-RW-07).
- Orden topológico de `mutation.Superseded`.

### B. Tests unitarios de dominio — `Cancel` (nuevo `work_item_cancel_test.go`)

- `active` → `cancelled`, `Phases` sin cambios (CA-CA-06).
- `completed` / `archived` / `cancelled` → `ErrWorkItemCannotCancel` (CA-CA-02/03).

Más: test de `work_item_validation.go` que confirme que un manifest con fase `superseded` cuya
dependencia está en `ready`/`in_progress` **no** produce `work_item.dependencies_unsatisfied`
(regresión de DD-5).

### C. Test de ciclo de vida — `workflow_lifecycle_test.go` (ampliar)

Extender el escenario para ejercitar, con repos reales de infra (sin mocks):
- **Rework**: completar hasta `implementation`/`verification`, ejecutar `ReworkUseCase` sobre
  `plan`; aseverar `plan == ready`, descendientes `superseded`, item `active`, revision +1,
  `Approvals` intactos, y que `events.jsonl` contiene exactamente los eventos esperados con sus
  `cause` y `reason`. Luego `begin plan` tiene éxito (CA-RW-02).
- **Rework con reactivación**: llevar el item a `completed`, `rework` sobre una fase; aseverar
  `work_item.reactivated` como primer evento del lote y item de vuelta a `active` (CA-RW-03).
- **Cancel**: sobre un item `active`, `CancelUseCase`; aseverar que `active/<id>/` ya no existe,
  `cancelled/<id>/manifest.yaml` tiene `status: cancelled`, fases sin cambios, y
  `work_item.cancelled` con `reason` en el `events.jsonl` del expediente cancelado (CA-CA-01/06).
- **Idempotencia**: repetir `rework` y `cancel` con el mismo `--operation-id`; aseverar que no se
  duplican eventos (CA-RW-12 / CA-CA-10).

### D. Tests E2E del binario — `cli_e2e_test.go` (ampliar)

- `rework` sobre un work item avanzado: exit 0, JSON con `WorkItem` reworkeado; `begin` posterior
  OK.
- `rework` sin `--reason` → `invalid_arguments` (ESC-RW-06); fase inexistente → `not_found`;
  item `archived` → `invalid_transition` (ESC-RW-04).
- `cancel` exitoso (ESC-CA-01): exit 0, `Work item '<id>' cancelled.`, directorio movido.
- `cancel` de item `completed` → `invalid_transition` (CA-CA-02); sin `--reason` →
  `invalid_arguments`.
- Reserva de ID: tras `cancel`, `start <id>` → `already_exists` con mensaje de cancelación
  (CA-CA-05).
- Opacidad: `status`/`next`/`validate <id>` sobre ID cancelado → error diagnóstico `not_found`
  con mensaje de cancelación (CA-CA-09 / ESC-CA-06), sin exponer contenido.
- `cancel` con `--actor-kind agent` → éxito (CA-CA-07).

Seguir el estilo de helpers ya presentes (`runJSONBinary`, `runTestBinary`, `responseData`,
`setupTestEnv`). No usar mocks para el filesystem; ejercitar el movimiento transaccional real.

---

## Riesgos, rollback y dependencias

### Riesgos

- **RK-1 (crítico). Regresión por la exención de `superseded` (DD-5).** Cambiar
  `work_item_validation.go` afecta a todos los manifests. Mitigación: la exención es aditiva
  (solo agrega un `continue` para un estado que antes no se producía por ningún comando); correr
  la suite completa (`go test ./...`) y agregar el test de regresión del bloque B. Si algún test
  existente asumía otra cosa sobre `superseded`, revisarlo antes de avanzar.
- **RK-2. Consistencia del movimiento transaccional de `cancel` (RK-03 del PRD).** El recovery es
  el código más delicado. Mitigación: replicar fielmente `recoverArchiveTransactionLocked`
  (tres ramas: destino publicado, activo restaurado, backup pendiente) y cubrir con un test que
  simule interrupción vía el `commitHook` (mecanismo ya usado por los tests de archive) o
  inyectando el marker manualmente y reinvocando una operación que dispare `recoverIfNeeded`.
- **RK-3. Integridad del DAG en la propagación (RK-01 del PRD).** `Descendants` asume un DAG sin
  ciclos; `OrderedPhases` ya falla ante ciclos. Mitigación: la validación semántica del workflow
  en `start` garantiza DAG acíclico; `ReworkPhase` propaga `error` de `OrderedPhases`/`Descendants`.
- **RK-4. `start`/`next`/`validate` ganan una dependencia nueva (checker de cancelados).** Cambiar
  las firmas de `NewStartWorkItemUseCase`, `NewNextUseCase` y `NewValidateUseCase` rompe sus
  call-sites. Blast radius concreto (todo test salvo composition.go) a actualizar:
  `cmd/composition.go`; `cmd/root_test.go`; `internal/usecases/contract_integration_test.go`,
  `dependency_injection_test.go`, `workflow_lifecycle_test.go`, `validate_uc_test.go`,
  `usecases_test.go`, `idempotency_test.go`. Mitigación: pasar `infra.NewFSWorkItemRepository()`
  (o el fake correspondiente) como checker en cada call-site. Alternativa evaluada y descartada:
  ensanchar `WorkItemExistenceChecker`/`WorkItemReader` con el método de cancelados para evitar
  nuevos parámetros — traslada el blast radius a todas las implementaciones fake de esas
  interfaces y viola ISP; se prefiere una interfaz nueva y específica (`CancelledWorkItemChecker`).
- **RK-5. Deuda de duplicación archive/cancel.** `fs_cancel_repository.go` replica estructura de
  `fs_archive_repository.go`. Trade-off aceptado conscientemente (DD-3) para no refactorizar el
  path transaccional dentro del scope de esta feature. Documentar como candidato a factorización
  futura de un "mover transaccional genérico".

### Supuestos

- AS-01 a AS-04 del PRD se asumen válidos. En particular AS-05: `docs/internal/9-guia-tecnica-cli.md`
  no existe; el plan se basa en la spec aprobada y en el código fuente leído, que es la fuente de
  verdad prioritaria.
- Se asume que `work-item.schema.json` ya acepta `status: cancelled` y que `event.schema.json`
  valida `type` por patrón abierto (confirmado en `data-modeling.md` y en la sección "Sin cambio"
  de la spec). Verificar al inicio de la Etapa 2 leyendo ambos schemas; si no fuera así, escalar
  antes de continuar (sería un cambio de contrato fuera del alcance declarado).

### Efectos colaterales

- `begin` acepta `superseded → in_progress` (ya soportado, transiciones-de-fase-y-gates.md); no
  requiere cambios. Un `rework` seguido de `begin` sobre la fase objetivo (`ready → in_progress`)
  también funciona sin cambios.
- `complete`/`approve`/`deliver` sobre la fase objetivo reworkeada vuelven a desbloquear
  (`unlockReadyPhases`) las fases `blocked`; las `superseded` se retoman con `begin` (diseño
  esperado). Validar que el flujo post-rework reconstruye el camino correctamente en el test C.

### Rollback

- Todos los cambios son aditivos salvo DD-5 (una línea en validación) y las firmas de tres
  constructores. El rollback es revertir el commit/PR. No hay migración de datos: ningún manifest
  existente cambia de forma; `cancelled/` solo se crea al primer `cancel`. Un expediente ya
  cancelado quedaría huérfano tras un rollback del binario, pero su contenido (manifest + events)
  permanece íntegro y legible manualmente.

---

## Trazabilidad

| Elemento del plan | Fuente |
|---|---|
| `ReworkPhase`, `ReworkMutation`, `Descendants` | spec §`rework`, RB-RW-01..08, CA-RW-01..12 |
| Exención `superseded` en validación (DD-5) | derivado de POST-RW-02 + work_item_validation.go:151 |
| `Cancel`, `fs_cancel_repository.go`, recovery | spec §`cancel`, RB-CA-01..07, CA-CA-01..10, AS-03 |
| Reserva de ID / mensaje `already_exists` | RB-CA-06, CA-CA-05, spec §"Impacto sobre comandos existentes" |
| Diagnóstico `status`/`next`/`validate` | DD-01 (PRD), INV-05, CA-CA-09 |
| Eventos `work_item.reactivated`/`cancelled`/causas | spec §"Eventos emitidos", INV-06 |
| Idempotencia por `--operation-id` | AS-04, CA-RW-12, CA-CA-10 |
| Patrón transaccional reutilizado | `fs_archive_repository.go`, data-and-process-flow.md |
| Estados de dominio | `work_item.go`, data-modeling.md |
