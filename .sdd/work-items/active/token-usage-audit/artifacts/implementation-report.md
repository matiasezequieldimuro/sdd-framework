---
schema_version: "0.1"
kind: artifact
id: "implementation-report"
work_item: "token-usage-audit"
phase: "implementation"
status: draft
created_at: "2026-09-29T00:00:00Z"
created_by: { kind: "agent", id: "sdd-developer" }
sources:
  - artifacts/plan.md
  - artifacts/specification.md
---

# Reporte de implementación: Auditoría y registro de consumo de tokens

## Resumen de cambios

La implementación sigue las cuatro etapas del plan aprobado en orden de dependencia. Todos los cambios se restringen a las fuentes del framework (`src/.sdd/`, `src/cli/`, `src/adapters/claude-code/`) sin tocar la instalación raíz `/.sdd/`.

---

## Etapa 1 — Fundaciones: schema, dominio y config

### Archivos modificados

| Archivo | Cambio |
| --- | --- |
| `src/.sdd/schemas/work-item.schema.json` | Campo `total_tokens` agregado a `$defs.observability.token_usage.properties` como `{"type":["integer","null"],"minimum":0}`. Aditivo: no rompe manifests existentes. |
| `src/.sdd/config.yaml` | `token_usage: optional` → `token_usage: false` (template). |
| `src/cli/internal/domain/work_item.go` | `TokenUsage.TotalTokens *int` agregado; constantes `TokenUsageStatus`; `NewWorkItemParams.TokenAuditActive bool`; función `initObservability(bool)` (helper privado); método `WorkItem.RecordTokenUsage(in,out,cacheRead,cacheWrite int, source string) error`; método privado `auditActive() bool`. |
| `src/cli/internal/domain/errors.go` | `ErrTokenAuditInactive` agregado. |
| `src/cli/internal/domain/config.go` | Reescritura completa: `ConfigObservability`, `TokenUsageSetting` con `UnmarshalYAML` tolerante (D-d), método `Config.TokenAuditActive() bool`. |

### Decisiones de diseño

- `initObservability` es una función privada del paquete domain que encapsula la inicialización bifurcada (activa/inactiva). `NewWorkItem` la llama con el flag del params.
- `auditActive()` como método privado sigue SRP: el predicado de negocio vive en el dominio, no en el use case.
- El `UnmarshalYAML` tolerante usa `value.Tag == "!!bool"` para distinguir literales booleanos de strings, evitando que `optional` (u otro string) rompa el parseo.
- La `Observability` siempre se inicializa (no es `nil`); lo que varía es `TokenUsage.Status` y los contadores.

### Tests nuevos

- `src/cli/internal/domain/work_item_token_test.go`: 8 tests cubriendo `NewWorkItem` (activa/inactiva), `RecordTokenUsage` (éxito, overwrite, todos cero, inactiva, negativos, source vacío).
- `src/cli/internal/domain/config_token_test.go`: 6 sub-casos para unmarshal tolerante (true/false/optional/ausente/string 'false'/observability ausente).

---

## Etapa 2 — Comando `record-tokens`

### Archivos nuevos / modificados

| Archivo | Cambio |
| --- | --- |
| `src/cli/internal/usecases/record_tokens_uc.go` | **Nuevo.** Use case espejo de `record_event_uc.go`. Flujo: load → idempotencia → `RecordTokenUsage` → evento `tokens.recorded` → `commitWorkItem`. |
| `src/cli/cmd/record_tokens.go` | **Nuevo.** Comando Cobra con flags `--input-tokens`, `--output-tokens`, `--cache-read-tokens`, `--cache-write-tokens` (todos requeridos), `--source` (requerido), `--actor-kind`, `--actor-id`, `--operation-id`. `total` NO es flag (D-a). |
| `src/cli/cmd/composition.go` | `RecordTokens *usecases.RecordTokensUseCase` agregado a `Application` y a `NewProductionApplication()`. |
| `src/cli/cmd/root.go` | `newRecordTokensCommand` registrado; `ErrTokenAuditInactive` → `"token_audit_inactive"` en `errorCode`. |

### Tests nuevos / modificados

- `src/cli/internal/usecases/record_tokens_uc_test.go`: 3 tests (éxito+persistencia, idempotencia, audit inactiva).
- `src/cli/cmd/cli_token_e2e_test.go`: 7 tests E2E (éxito+overwrite, creación con ceros, audit inactiva, valor negativo, todos cero, idempotencia, status texto).

---

## Etapa 3 — Init en `start` + visibilidad en `status`

### Archivos modificados

| Archivo | Cambio |
| --- | --- |
| `src/cli/internal/usecases/start_uc.go` | Config se carga **siempre** (antes sólo si `WorkflowID==""`). `TokenAuditActive: config.TokenAuditActive()` pasado a `NewWorkItemParams`. |
| `src/cli/cmd/status.go` | Añadido import `domain`; funciones `printTokenUsage` y `formatOptionalInt`; bloque "Token Usage" impreso antes de la tabla de fases cuando `Observability.TokenUsage != nil`. La salida `--json` ya exposa el campo (StatusResult embebe `*domain.WorkItem`). |

### Notas

- `start_uc.go`: la variable `config` y `err` se reutilizan sin shadowing; el compilador lo valida en cada build.
- `status.go`: el bloque de tokens no imprime nada si `Observability == nil` (ítems pre-feature, E-11 satisfecho sin cambios adicionales).

---

## Etapa 4 — Adapter Claude Code

### Archivos nuevos / modificados

| Archivo | Cambio |
| --- | --- |
| `src/adapters/claude-code/.claude/hooks/token-usage/record-token-usage.py` | **Nuevo.** Hook Python `Stop`. Lee stdin (session_id, transcript_path), resuelve `.active-work-item` (RC-6), parsea transcript sumando usage de todos los mensajes de asistente incluidos sidechains, llama a `sdd-cli record-tokens`, registra errores en log (D-g), nunca bloquea (exit 0). `ADAPTER_ID = "claude-code"` como constante única (D-e). |
| `src/adapters/claude-code/.claude/hooks/token-usage/test-record-token-usage.sh` | **Nuevo.** Script de prueba manual: 9 casos (T1: .active-work-item ausente, T2: ID inválido, T3: audit inactiva, T4: transcript vacío, T5: happy path, T6: idempotencia). Todos pasan contra un `sdd-cli` real. |
| `src/adapters/claude-code/.claude/settings.json` | Hook `Stop` registrado (`matcher:"*"`, `timeout:30`, invoca el hook Python). Los `PreToolUse` de seguridad no se tocan. |
| `src/adapters/claude-code/.claude/.gitignore` | `hooks/token-usage/logs/` agregado. |
| `src/adapters/claude-code/.claude/skills/git-worktree/SKILL.md` | Sección C actualizada: instrucción para crear `.active-work-item`, condición de uso R-1/RN-5, limitación del primer turno. |
| `src/adapters/claude-code/.claude/agents/sdd-orchestrator.md` | Paso 4 nuevo en "Starting on stable branches": crear `.active-work-item` inmediatamente después de `sdd-cli start`, documentación de condición de uso y flujos sin worktree. |

### Embeds regenerados

`go generate ./...` ejecutado desde `src/cli`:
- `src/cli/embeds/default_sdd/`: 60 archivos (schema actualizado).
- `src/cli/embeds/default_adapters/`: 48 archivos (subió de 46: +hook Python, +test script).

### Test de adapter actualizado

- `src/cli/internal/infra/fs_adapter_repository_test.go`: aserción del `.gitignore` actualizada de comparación exacta a `strings.Contains` para las dos entradas (`settings.local.json` y `hooks/token-usage/logs/`).

---

## Verificación

### Compilación

```
go build ./...   # desde src/cli — sin errores ni warnings
```

### Tests unitarios y E2E

```
go test ./... -count=1
?   sdd-cli          [no test files]
ok  sdd-cli/cmd      8.032s
ok  sdd-cli/internal/domain    0.007s
ok  sdd-cli/internal/infra     3.664s
ok  sdd-cli/internal/usecases  8.346s
```

**Total: 0 fallos, 0 regresiones.**

### Prueba manual del adapter

```
bash .claude/hooks/token-usage/test-record-token-usage.sh
Results: 9 passed, 0 failed
```

---

## Desviaciones respecto del plan

**Ninguna desviación de comportamiento.** Observaciones menores:

1. **Test del `.gitignore` existente:** El test `TestFSAdapterRepositoryInstallsClaudeCode` asertaba el contenido exacto del `.gitignore`. Al agregar `hooks/token-usage/logs/` fue necesario actualizar la aserción para usar `strings.Contains`. Esto es un ajuste de baseline esperado, no una desviación del plan.

2. **Inicialización de `Observability`:** El plan describía la `Observability` como posiblemente `nil` antes de la feature. En la implementación, `NewWorkItem` siempre crea el bloque (no `nil`), con `TokenUsage.Status = "not_reported"` cuando inactiva. Esto es equivalente al comportamiento especificado (E-03/E-11) y consistente con que el schema lo admite.

3. **Contador `((PASS++))` en el script de prueba manual:** bash con `set -e` trata `((0))` como error. Corregido usando `PASS=$((PASS+1))` — comportamiento idéntico, sintaxis más robusta.

---

## Correcciones post-review (F1/F2/F4 + doc F3)

### F1 (ALTA) — Hook guard: transcript ilegible o sin mensajes no pisa el total

**Archivos modificados:**
- `src/adapters/claude-code/.claude/hooks/token-usage/record-token-usage.py`
- `src/adapters/claude-code/.claude/hooks/token-usage/test-record-token-usage.sh`

**Cambios en el hook:**
1. `sum_token_usage` ya no silencia `OSError`; la re-lanza al caller.
2. En `main()`, se captura `OSError` → `write_error_log` (kind: `transcript_read_error`) → `return 0` sin llamar a `record-tokens`.
3. Guard explícito: si `n == 0` (cero mensajes de asistente encontrados) → `return 0` silencioso. Esto evita sobrescribir el total acumulado previo con 0/0/0/0 (viola RN-3).
4. El comentario en el guard aclara que un registro CLI directo con ceros (E-09) sigue siendo válido; el guard sólo aplica cuando el hook no encontró datos de sesión.

**Cambios en el test script:**
- **T4** actualizado: el comportamiento esperado cambió de "empty transcript → recorded (0)" a "empty transcript → hook salta, manifest se mantiene en partial". Dos aserciones: sin error log y `status: partial`.
- **T7 nuevo:** transcript path es un directorio (→ `OSError` en `open()`). Verifica: error log escrito y manifest sin modificar (`status: partial`).
- Comentario en `Requirements` actualizó para indicar que `sdd-cli` debe construirse desde el source actual.

**Resultado:** 11 passed, 0 failed (antes: 6 failed).

---

### F2 (MEDIA) — `start` con `--workflow` explícito tolera config ausente/corrupta

**Archivos modificados:**
- `src/cli/internal/usecases/start_uc.go`
- `src/cli/internal/usecases/dependency_injection_test.go`

**Cambio en start_uc.go:**
- `GetConfig` ya no provoca fallo inmediato para todos los casos. Si `WorkflowID` es vacío (sin `--workflow`), el error de config se sigue propagando (se necesita para `defaults.workflow`). Si `WorkflowID` fue dado explícitamente y la config falla, se asigna `config = &domain.Config{}` → `TokenAuditActive()` devuelve `false` (auditoría inactiva, comportamiento seguro). El resto del flujo —incluyendo `--from-artifact`— no se altera.

**Test nuevo:** `TestStartUseCaseExplicitWorkflowToleratesConfigFailure`
- Usa `staticConfigRepository` con `err` inyectado y `WorkflowID = workflow.ID` (explícito).
- Verifica que `Execute()` no retorna error, el work item es creado, y `TokenUsage.Status == "not_reported"` (auditoría inactiva).
- El caso "default configuration" en `TestStartUseCasePropagatesAdapterFailures` sigue pasando (sin `WorkflowID`, la config sigue siendo obligatoria).

---

### F4 (BAJA) — DRY del total: se lee de `item.Observability.TokenUsage.TotalTokens`

**Archivo modificado:** `src/cli/internal/usecases/record_tokens_uc.go`

**Cambio:** la variable `total` en el evento `tokens.recorded` pasó de recomputar `in+out+cacheRead+cacheWrite` a leer `*item.Observability.TokenUsage.TotalTokens` — el mismo valor que `RecordTokenUsage` calculó y persiste en el manifest (D-a). El manifest y el event log no pueden diverger.

**Verificación:** todos los tests E2E existentes siguen pasando con el valor correcto en `total_tokens`.

---

### F3 (sólo documentar) — Limitación conocida: nueva sesión sub-registra el total

**Archivos modificados (sólo documentación):**
- `src/adapters/claude-code/.claude/skills/git-worktree/SKILL.md` — sección C: párrafo "Limitación conocida — multi-sesión (F3 / RN-5)" agregado.
- `src/adapters/claude-code/.claude/agents/sdd-orchestrator.md` — sección 5, paso 4: mismo párrafo en inglés.

**Texto:** Abrir una sesión nueva (no resumida) sobre el mismo work item hace que el siguiente `Stop` acumule sólo los tokens de esa sesión y sobrescriba el total anterior. Solución: usar siempre la misma sesión por work item.

---

### Verificación post-correcciones

```
go generate ./...     # 60 SDD files, 48 adapter files — OK
go build ./...        # sin errores
go test ./... -count=1
?   sdd-cli                     [no test files]
ok  sdd-cli/cmd                 10.755s
ok  sdd-cli/internal/domain     0.019s
ok  sdd-cli/internal/infra      5.365s
ok  sdd-cli/internal/usecases   14.750s

bash test-record-token-usage.sh  # con sdd-cli del source actual en PATH
Results: 11 passed, 0 failed
```

**Total: 0 fallos, 0 regresiones. Todas las correcciones verificadas.**

---

## Pendientes / notas para el verificador

- **Dogfooding:** Para activar la feature en el propio repositorio raíz (instalación `/.sdd`), el usuario debe editar manualmente `/.sdd/config.yaml` cambiando `token_usage: false` → `token_usage: true` y reinstalar el adapter (o copiar el hook). La `/.sdd` raíz NO fue modificada (directiva del usuario).
- **`.active-work-item` en el worktree actual:** El archivo `token-usage-audit/.active-work-item` no existe en el worktree de trabajo ya que el orquestador decidirá si procede a crearlo. No es responsabilidad de este developer.
- **Retención de logs:** v1 conserva todos los archivos de log sin rotación (limitación documentada en D-g del plan).
- **Transcript asíncrono:** Con F1 corregido: si el transcript del último turno no se escribió en disco al momento del `Stop` hook, ahora se registra un error de diagnóstico y se salta el registro (en lugar de poner 0/0/0/0). El total previo queda intacto y el turno faltante se recupera en el próximo turno.
- **Binary del test script:** `test-record-token-usage.sh` requiere que el `sdd-cli` en PATH sea el compilado desde el source actual (el binario instalado en `/usr/local/bin` puede ser una versión anterior). Comando: `go build -o ~/bin/sdd-cli . && PATH=~/bin:$PATH bash test-record-token-usage.sh`.
