---

description: "Tareas para documentar el 429 por tokens inexistentes de IPzilon 3.4.0 (provider v3.2.1)"
---

# Tasks: Documentar el 429 por tokens inexistentes (IPzilon 3.4.0)

**Input**: Documentos de diseño en `specs/005-invalid-token-rate-limit/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: SÍ se incluyen para US2. La spec los exige (FR-008) y el Principio V pide unitarios
sin API para la lógica pura. Se usan los helpers de `internal/client/client_test.go`
(`sequenceServer`, `newTestClient`, `fakeSleeper`) y `tflogtest` de
`github.com/hashicorp/terraform-plugin-log` v0.11.0 (ya en `go.mod`). US1 es solo documentación:
se valida regenerando `docs/` y revisando el texto, sin tests de código. No hacen falta tests de
aceptación (research R6, R7).

**Organization**: tareas agrupadas por historia de usuario (US1–US2 de spec.md).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: se puede ejecutar en paralelo (ficheros distintos, sin dependencias pendientes)
- **[Story]**: historia a la que pertenece (US1, US2)

## Path Conventions

Provider Go de proyecto único: `internal/client`, `internal/provider`, `docs/` (generado con
`make generate`, nunca a mano), `README.md`.

---

## Phase 1: Setup

- [X] T001 Comprobar la línea base en la rama `005-invalid-token-rate-limit` ejecutando `go build ./... && go vet ./... && go test ./...` desde la raíz; anotar cualquier fallo previo antes de tocar código. La issue ya existe (#24): no se crea ninguna nueva; la PR la cerrará con `Closes #24`

---

## Phase 2: Foundational (Blocking Prerequisites)

No hay prerrequisitos compartidos: US1 y US2 tocan ficheros distintos y no dependen entre sí.

---

## Phase 3: User Story 1 - Entender por qué un `apply` con token válido espera ~60 s (Priority: P1) 🎯 MVP

**Goal**: la descripción del provider en el Registry y el `README.md` explican el bloqueo por
dirección de cliente de IPzilon 3.4.0 además del cupo por token (FR-001–FR-003).

**Independent Test**: leer `docs/index.md` y la sección *Compatibility* del `README.md` y
comprobar que nombran las dos causas de un 429, el mensaje `Too many invalid API tokens`, la
espera máxima (60 s por defecto) y que el provider la reintenta sin fallar; la información previa
sobre el cupo por token y los reintentos de 429/503 se mantiene.

### Implementation for User Story 1

- [X] T002 [P] [US1] En `internal/provider/provider.go`, en `Schema`, sustituir el final de la `Description` del provider `"… for up to 10 minutes per request; each API token has its own rate-limit quota."` por el texto nuevo literal de `specs/005-invalid-token-rate-limit/contracts/provider-docs.md` §Description: `"… for up to 10 minutes per request. Each API token has its own rate-limit quota; in addition, IPzilon >= 3.4.0 blocks API-token requests from a client address for up to a minute (\"Too many invalid API tokens\") after too many requests with non-existent tokens from that address, even when the token in use is valid. Those 429s are retried the same way."`. El principio de la descripción (hasta `Requires IPzilon >= 3.0.0.` y la frase de reintentos) no cambia
- [X] T003 [P] [US1] En `README.md`, sección `## Compatibility`, mantener el párrafo que empieza por `Requests rejected by IPzilon with \`429\`` y añadir justo después (antes de `## Breaking Changes (v3.0.0)`) el párrafo literal de `specs/005-invalid-token-rate-limit/contracts/provider-docs.md` §README (empieza por `Since IPzilon 3.4.0 there is also a per-client-address block:` e incluye `Since provider v3.2.1, with \`TF_LOG=WARN\` it also logs a warning naming the cause.`), respetando el ajuste de línea a ~80 columnas del resto del README. La tabla de compatibilidad no cambia (research R7)
- [X] T004 [US1] Regenerar la documentación con `make generate` (si `tfplugindocs` ya está en `$(go env GOPATH)/bin`, basta con `$(go env GOPATH)/bin/tfplugindocs generate --provider-name ipzilon`); comprobar con `git diff --stat docs/` que solo cambia `docs/index.md` y que contiene `Too many invalid API tokens` en sus dos apariciones de la descripción (cabecera `description:` y cuerpo). Nunca editar `docs/` a mano (Principio IV). Depende de T002

**Checkpoint**: US1 completa y entregable por sí sola.

---

## Phase 4: User Story 2 - Distinguir el bloqueo por dirección en los logs (Priority: P2)

**Goal**: al reintentar un 429 `Too many invalid API tokens`, el provider registra un aviso
(`WARN`) una sola vez por petición; el resto de 429/503 y la política de reintentos no cambian
(FR-004–FR-008).

**Independent Test**: `go test ./internal/client -run 'InvalidToken' -v` con un servidor
simulado: 429 de tokens inexistentes → un aviso y éxito; 429 de cupo o 503 → ningún aviso.

### Tests for User Story 2 ⚠️

> Escribirlos primero y comprobar que FALLAN antes de T008.

- [X] T005 [US2] En `internal/client/client_test.go` añadir un helper `warnLogs(t *testing.T, buf *bytes.Buffer) []map[string]any` que decodifique `buf` con `tflogtest.MultilineJSONDecode` y devuelva solo las entradas con `"@level" == "warn"` (importar `bytes` y `github.com/hashicorp/terraform-plugin-log/tflogtest`). El contexto de cada test se crea con `ctx := tflogtest.RootLogger(context.Background(), &buf)`
- [X] T006 [US2] En `internal/client/client_test.go` añadir `TestDo_InvalidTokenBlockWarnsOnce`: `sequenceServer` con dos respuestas `{code: 429, body: \`{"error":"Too many invalid API tokens"}\`, retryAfter: "42"}` seguidas de `{code: 200, body: \`{"id":1}\`}`; `c.Get(ctx, "/x", &out)` debe terminar sin error, con 3 llamadas, `fs.waits == [42s 42s]` y **exactamente una** entrada en `warnLogs`, cuyo `@message` contiene `non-existent API tokens` y `not the rate-limit quota of the configured token`, con campos `method == "GET"`, `path == "/x"`, `status == 429` (número JSON, `float64(429)`) y `wait == "42s"`; además, el buffer completo no debe contener `test-token` (token de `newTestClient`) ni `Authorization` (FR-005, FR-006, contracts/log-warning.md)
- [X] T007 [US2] En `internal/client/client_test.go` añadir `TestDo_OtherRetriesDoNotWarnInvalidToken` como tabla con dos casos: (a) `429` con `{"error":"Rate limit exceeded: 60 per 1 minute"}` y `retryAfter: "2"`; (b) `503` con cuerpo vacío y `retryAfter: "1"`; cada uno seguido de `200`: la petición termina bien y `warnLogs` devuelve 0 entradas (FR-005). Añadir también el caso (c): `429` `Too many invalid API tokens` con `retryAfter: "60"` y `c.retryMaxElapsed = 30 * time.Second` (se rinde sin reintentar) → el error es `*APIError` con `Code == 429` y `Message` que empieza por `Too many invalid API tokens (gave up after 0 retries`, y 0 avisos (el aviso solo se emite si se reintenta, contracts/log-warning.md §Cuándo)

### Implementation for User Story 2

- [X] T008 [US2] En `internal/client/client.go` añadir junto a las constantes de reintento la constante `invalidTokensBlockedMessage = "Too many invalid API tokens"` con comentario: IPzilon >= 3.4.0 answers 429 with this error while it blocks API-token requests from a client address after too many requests with non-existent tokens (research R1)
- [X] T009 [US2] En `internal/client/client.go`, en `do`, declarar antes del bucle `warnedInvalidTokens := false`; dentro de la rama `if resp.StatusCode == http.StatusTooManyRequests || …`, **después** del `if retries >= … { return apiErr }` y **antes** del `tflog.Debug(ctx, "retrying IPzilon request", …)` existente, añadir: si `resp.StatusCode == http.StatusTooManyRequests && !warnedInvalidTokens && strings.HasPrefix(parseAPIError(resp.StatusCode, respBody).Message, invalidTokensBlockedMessage)` → `tflog.Warn(ctx, <mensaje>, map[string]any{"method": method, "path": path, "status": resp.StatusCode, "wait": wait.String()})` y `warnedInvalidTokens = true`. `<mensaje>` es el texto literal de `specs/005-invalid-token-rate-limit/contracts/log-warning.md` §Nivel y mensaje (`IPzilon is temporarily blocking API-token requests from this client address because too many requests with non-existent API tokens came from it; this is not the rate-limit quota of the configured token. Retrying after Retry-After. Check for processes behind the same address (NAT, shared runner) using a deleted or mistyped token, including this provider's own token.`). No tocar `retryWait`, el `tflog.Debug` ni el cálculo de abandono (FR-007). Depende de T008
- [X] T010 [US2] Ejecutar `go test ./internal/client -run 'InvalidToken|TestDo_' -v`: T006 y T007 pasan, y todos los `TestDo_*` existentes siguen en verde (SC-002, SC-003)

**Checkpoint**: US1 y US2 completas.

---

## Phase 5: Polish & Cross-Cutting Concerns

- [X] T011 Ejecutar `gofmt -s -l .` (sin salida), `go build ./... && go vet ./... && go test ./...` y `go list -deps . | grep terraform-plugin-sdk` (debe devolver vacío; `tflogtest` solo se importa en tests) (Principio V, restricciones técnicas)
- [X] T012 Ejecutar de nuevo `make generate` y comprobar que `git status --short docs/` no muestra cambios respecto a T004 (SC-004, quickstart §2)
- [X] T013 Opcional: escenario manual de `specs/005-invalid-token-rate-limit/quickstart.md` §3 contra IPzilon 3.4.0 local (imagen construida desde el tag `v3.4.0` si GHCR es privado): 31 peticiones con `Bearer ipam_inexistente` → las últimas `429`; después `TF_LOG=WARN terraform plan` con el token válido y el binario local (`dev_overrides`) muestra el aviso y termina sin error en ≤ ~60 s. Anotar el resultado en esta tarea, o "no ejecutado" y el motivo → **2026-10-06**: imagen local `ipzilon-local:3.4.0` construida desde el tag `v3.4.0`, solo backend con SQLite. Con el token válido: `200`; 31 peticiones con `ipam_inexistente` → 31 × `401` y la 32.ª `429`; con el token válido → `429` `retry-after: 60` `{"error": "Too many invalid API tokens"}`. `TF_LOG=WARN terraform plan` con el binario local (`dev_overrides`) y el token válido: 1 aviso `[WARN]` con `path="/sites/?limit=1000&offset=0"` y `status=429`, el token no aparece en el log, el `plan` tarda 60 s y termina con `exit=0` (`+ sites = 0`)
- [X] T014 Commit con el comando `commit-message` (confirmando el mensaje con `AskUserQuestion`, sin `Co-Authored-By`), PR a `main` con la plantilla completada y `Closes #24`; tras el merge, tag `v3.2.1` (confirmando antes con `AskUserQuestion`); después actualizar `main`, limpiar ramas locales y marcar la spec como cerrada → commit `8c7f6f2`, PR #25 mergeada (`4cabcf2`, CI en verde), issue #24 cerrada, tag `v3.2.1` publicado el 2026-10-06 (release GoReleaser con `darwin_arm64`, `linux_amd64`, `windows_amd64` y `SHA256SUMS` firmados); `main` actualizada y rama `005-invalid-token-rate-limit` eliminada (local y remota)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias.
- **Foundational (Phase 2)**: vacía.
- **US1 (Phase 3)** y **US2 (Phase 4)**: independientes entre sí; ambas tras T001.
- **Polish (Phase 5)**: tras US1 y US2 (T012 tras T004; T014 al final).

### Within Each User Story

- US1: T002 y T003 en paralelo → T004 (regenera a partir de T002).
- US2: T005 → T006, T007 (mismo fichero, secuenciales) → comprobar que fallan → T008 → T009 → T010.

### Ficheros compartidos (no paralelizar entre sí)

- `internal/client/client_test.go`: T005, T006, T007.
- `internal/client/client.go`: T008, T009.

## Parallel Example: entre historias (tras T001)

```text
T002 [US1] provider.go (Description)   |  T005–T007 [US2] client_test.go
T003 [US1] README.md                   |  T008–T009 [US2] client.go (tras ver fallar los tests)
```

## Implementation Strategy

### MVP (US1)

1. T001 → T002, T003 → T004: la documentación sola ya resuelve la parte obligatoria de la issue.

### Incremental Delivery

1. US1 (documentación).
2. US2 (aviso en el log), con tests primero.
3. Polish y release única **v3.2.1** (ambas historias juntas en una sola PR).
