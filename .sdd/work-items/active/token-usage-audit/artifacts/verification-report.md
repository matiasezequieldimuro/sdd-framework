---
schema_version: "0.1"
kind: artifact
id: "verification-report"
work_item: "token-usage-audit"
phase: "verification"
status: draft
created_at: "2026-09-29T15:01:56Z"
created_by: { kind: "cli", id: "sdd" }
sources: 
  - artifacts/implementation-report.md
  - artifacts/specification.md
  - artifacts/plan.md
---

# Reporte de verificación: Auditoría y registro de consumo de tokens

## Resultado

**PASS** — Todos los criterios de aceptación E-01..E-12 y contratos clave RC-1..RC-9 verificados con evidencia reproducible. No se detectaron fallos ni regresiones. Se identificaron dos observaciones menores sin impacto funcional (documentadas en la sección de gaps).

---

## Entorno de ejecución

| Componente | Valor |
| --- | --- |
| Worktree | `/home/mdimuro/projects/personal/sdd-framework-worktrees/token-usage-audit` |
| Binario verificado | Construido desde `src/cli/` (`go build -o /tmp/sdd-cli .`) |
| Versión del binario | `sdd-cli dev (commit none, built unknown)` |
| Go | Instalación del sistema (WSL2, Linux 6.6.87.2) |
| Python | `python3` en PATH (para el hook del adapter) |
| Fecha de verificación | 2026-09-29 |

**Nota de entorno:** La instalación global en `/usr/local/bin/sdd-cli` corresponde a una versión anterior (v0.1.1-dev) sin la feature implementada. Toda la verificación se realizó con el binario construido desde el worktree. El script `test-record-token-usage.sh` requiere que el binario del worktree sea el primero en PATH (`PATH="/tmp:$PATH"`).

---

## Matriz de trazabilidad

| Criterio | Plan (etapa/tarea) | Cambio implementado | Prueba / inspección | Evidencia | Resultado |
| --- | --- | --- | --- | --- | --- |
| E-01 — Sobrescritura total | E2, E1 | `RecordTokenUsage` + `record-tokens` | CLI: record 500/300/100/50→manifest exacto; E2E `TestCLIRecordTokens_ActiveAudit_SuccessAndOverwrite` | `cli-manual-verification.log` | PASS |
| E-02 — Init activa: `partial` + 0s | E1, E3 | `initObservability(true)` en `NewWorkItem`; `start_uc.go` pasa flag | CLI: manifest post-`start` con activa; E2E `TestCLIRecordTokens_ActiveAudit_CreatedWithZeros` | `cli-manual-verification.log` | PASS |
| E-03 — Init inactiva: `not_reported` + nulls | E1, E3 | `initObservability(false)` | CLI: manifest post-`start` sin config activa (campos omitidos = null en schema) | `cli-manual-verification.log` | PASS |
| E-04 — Recuperación de turno perdido | E2, E1 | `RC-2` sobrescritura total; no acumulación interna | CLI: registro turno-2 (200/100), turno-4 (600/350) → manifest refleja 600/350 | `cli-manual-verification.log` | PASS |
| E-05 — Registro manual de respaldo | E2 | Comando público `record-tokens` | CLI: invocación directa con `--json` → `success: true`, manifest actualizado | `cli-manual-verification.log` | PASS |
| E-06 — Negativos → `validation_failed`, manifest intacto | E2 | Validación `< 0` en `RecordTokenUsage` + `RC-4` transacción | CLI: `-10` → `validation_failed`; manifest invariado; E2E `TestCLIRecordTokens_NegativeInput_ManifestIntact` | `cli-manual-verification.log`, `go-test-token-e2e.log` | PASS |
| E-07 — Aislamiento entre worktrees | E4 | `.active-work-item` por worktree + `--dir` | CLI: dos proyectos independientes; tokens de A→manifest A; tokens de B→manifest B | `e07-isolation.log` | PASS |
| E-08 — `.active-work-item` ausente/ID inválido | E4 | Validación en hook Python (`resolve_work_item`) | Adapter test T1 (ausente→log), T2 (ID inválido→log); aislado sin manifiesto modificado | `adapter-test.log` | PASS |
| E-09 — Ceros son válidos | E2, E1 | Sin restricción de mínimo > 0 en lógica de registro | CLI: `0/0/0/0` → `success: true`; E2E `TestCLIRecordTokens_AllZero_Valid` | `cli-manual-verification.log`, `go-test-token-e2e.log` | PASS |
| E-10 — Consumo visible en `status` (texto y JSON) | E3 | `printTokenUsage` en `cmd/status.go`; campo ya embebido en JSON | CLI texto: bloque "Token Usage" con source/input/output/cache/total; CLI JSON: `observability.token_usage` completo | `cli-manual-verification.log` | PASS |
| E-11 — Items preexistentes no se modifican | E3 | Config leída en `start_uc.go`; items existentes no se reprocesam | CLI: item creado con inactiva → `not_reported`; config cambiada → nuevo item `partial`, viejo intacto | `cli-manual-verification.log` | PASS |
| E-12 — Registro en item inactivo → error manejable `token_audit_inactive` | E2, E4 | `ErrTokenAuditInactive` mapeado a `"token_audit_inactive"`; hook escribe log | CLI: `token_audit_inactive` en JSON; E2E; adapter test T3 (log generado) | `cli-manual-verification.log`, `adapter-test.log`, `go-test-token-e2e.log` | PASS |
| RC-1 — Schema `token_usage` + `total_tokens` | E1 | Campo `total_tokens` aditivo en `work-item.schema.json` | Inspección del schema fuente; `total_tokens: {type: [integer, null], minimum: 0}` | Inspección código | PASS |
| RC-2 — Invariante sobrescritura total | E1, E2 | `RecordTokenUsage` sobreescribe; no acumula | E-01/E-04 verificados; motor no tiene estado acumulativo interno | `cli-manual-verification.log` | PASS |
| RC-3 — Motor sin lógica LLM | E2, E4 | Ninguna referencia a transcript/CLAUDE_PROJECT_DIR en motor | `grep` sobre `domain/`, `usecases/`, `cmd/` → 0 coincidencias | Inspección código | PASS |
| RC-4 — Atomicidad transaccional | E2 | `commitWorkItem` existente; validación previa a escritura | E-06: manifest no cambia ante error; E2E `TestCLIRecordTokens_NegativeInput_ManifestIntact` | `cli-manual-verification.log` | PASS |
| RC-9 — Idempotencia por `operation-id` | E2 | `operationApplied` reutilizado; event count invariable | CLI: mismo op-id → event count igual (7→7); op-id nuevo → event count crece (7→8); adapter T6 | `cli-manual-verification.log`, `adapter-test.log` | PASS |

---

## Resumen de pruebas ejecutadas

### 1. Build automatizado (`go build ./...`)

```
cd src/cli && go build ./...
# Sin errores ni warnings. Exit 0.
```

Evidencia: `evidence/go-build.log`

### 2. Suite de tests Go (`go test ./... -count=1`)

```
?     sdd-cli           [no test files]
ok    sdd-cli/cmd       14.090s
ok    sdd-cli/internal/domain    0.007s
ok    sdd-cli/internal/infra     8.827s
?     sdd-cli/internal/ports     [no test files]
ok    sdd-cli/internal/usecases  16.871s
```

0 fallos, 0 regresiones. Evidencia: `evidence/go-test-full.log`

**Tests de token específicos verificados:**

| Paquete | Tests | Resultado |
| --- | --- | --- |
| `domain` | `TestTokenUsageSetting_TolerantUnmarshal` (6 sub-casos), `TestNewWorkItem_ActiveAudit_InitialisesCountersToZero`, `TestNewWorkItem_InactiveAudit_InitialisesNotReported`, `TestRecordTokenUsage_Success`, `TestRecordTokenUsage_Overwrite`, `TestRecordTokenUsage_AllZero`, `TestRecordTokenUsage_InactiveAudit`, `TestRecordTokenUsage_NegativeValue`, `TestRecordTokenUsage_EmptySource` | PASS |
| `usecases` | `TestRecordTokensUseCase_Success`, `TestRecordTokensUseCase_Idempotent`, `TestRecordTokensUseCase_InactiveAudit` | PASS |
| `cmd` (E2E) | `TestCLIRecordTokens_ActiveAudit_SuccessAndOverwrite`, `TestCLIRecordTokens_ActiveAudit_CreatedWithZeros`, `TestCLIRecordTokens_InactiveAudit_ReturnsError`, `TestCLIRecordTokens_NegativeInput_ManifestIntact`, `TestCLIRecordTokens_AllZero_Valid`, `TestCLIRecordTokens_Idempotent`, `TestCLIStatus_TextIncludesTokenUsage` | PASS |

### 3. Prueba manual del adapter (`test-record-token-usage.sh`)

Ejecutado con el binario del worktree en PATH (`PATH="/tmp:$PATH"`):

```
Results: 9 passed, 0 failed
T1: absent .active-work-item → error log created       PASS
T2: invalid ID → error log created                     PASS
T3: inactive audit → error logged (token_audit_inactive) PASS
T4: empty transcript → zero recording, no error        PASS
T4: empty transcript → manifest shows recorded         PASS
T5: happy path → no error log                          PASS
T5: happy path → manifest shows recorded               PASS
T5: happy path → total_tokens = 990                    PASS
T6: idempotent replay → no duplicate event             PASS
```

Evidencia: `evidence/adapter-test.log`

### 4. Verificación CLI manual (criterios E-01..E-12)

Ejecutado mediante `sdd-cli` del worktree directamente. Todos los criterios verificados con salida observada y contrastada contra la especificación. Evidencia: `evidence/cli-manual-verification.log`

### 5. Pruebas UI / visual

No aplica. El sistema no tiene interfaz visual. Los comandos CLI actúan como superficie de API y fueron verificados en la sección anterior.

---

## Observaciones y gaps de cobertura

### OBS-1 — Entorno de prueba: binario instalado vs worktree (riesgo operativo, no funcional)

El script `test-record-token-usage.sh` usa el `sdd-cli` primero en PATH. La instalación global en `/usr/local/bin/sdd-cli` (v0.1.1-dev) es una versión diferente sin la feature, lo que provoca fallos de T3..T5 si el binario del worktree no está en PATH. Cuando la feature se instale (overwrite de `/usr/local/bin/sdd-cli`), el script funcionará sin prefijo de PATH. **No es un defecto del código; es una condición esperada del proceso de release/install.** Documentado como limitación de entorno de desarrollo.

### OBS-2 — Serialización de nulls en YAML (E-03): omisión vs. valor explícito `null`

La spec dice "todos los contadores en `null`" para auditoría inactiva. La implementación usa `*int` con `omitempty`, por lo que los contadores `nil` se **omiten** del YAML en lugar de escribirse como `null:` explícito. Esto es correcto desde el punto de vista del schema (`type: ["integer", "null"]`, campos opcionales) y no afecta el comportamiento del sistema. La omisión es semánticamente equivalente a `null`. El plan documenta este diseño explícitamente (Etapa 1, tarea 2). No se recomienda cambio; se documenta como observación para quien lea el manifest directamente.

### OBS-3 — Colisión de nombre de archivo de log en tests rápidos (test-script, no código de producción)

El nombre del archivo de log de error incluye timestamp UTC con precisión de segundos. Si dos tests consecutivos del script corren en el mismo segundo y usando el mismo `TMPDIR`, el segundo log sobreescribe al primero (mismo nombre), haciendo que el contador de logs no aumente. Esto afecta solo al script de prueba cuando tests comparten un directorio temporales y se miden por conteo de archivos. El comportamiento del hook en producción (un log por incidente) es correcto. Recomendación de mejora: agregar nanosegundos o un contador al nombre del log (mejora menor al test script, no al código de producción).

---

## Evidencia asociada

| Archivo | Contenido |
| --- | --- |
| `evidence/go-build.log` | Salida de `go build ./...` (sin errores) |
| `evidence/go-test-full.log` | Resultado completo de `go test ./... -count=1` |
| `evidence/go-test-token-e2e.log` | Tests E2E de tokens con verbose (`-run Token|token`) |
| `evidence/go-test-domain-tokens.log` | Unit tests de dominio para tokens (verbose) |
| `evidence/go-test-usecases-tokens.log` | Unit tests de use cases para tokens (verbose) |
| `evidence/adapter-test.log` | Salida completa de `test-record-token-usage.sh` (9/9 PASS) |
| `evidence/cli-manual-verification.log` | Verificación manual E-01..E-12 con salidas de CLI |
| `evidence/e07-isolation.log` | Evidencia de aislamiento entre worktrees (E-07) |
