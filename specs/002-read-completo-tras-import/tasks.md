---

description: "Tareas para la corrección sistémica del Read tras import (v3.0.1)"
---

# Tasks: Read completo tras import en todos los recursos

**Input**: Documentos de diseño en `specs/002-read-completo-tras-import/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: SÍ se incluyen. Los exigen FR-005/FR-006/FR-007 y el Principio V (todo bug corregido
SHOULD llevar un test que lo reproduzca). Unitarios con `net/http/httptest` (patrón de
`internal/resources/next_network_test.go`); aceptación con `terraform-plugin-testing`.

**Organization**: tareas agrupadas por historia de usuario (US1–US6 de spec.md).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: se puede ejecutar en paralelo (ficheros distintos, sin dependencias pendientes)
- **[Story]**: historia a la que pertenece (US1…US6)

## Path Conventions

Provider Go de proyecto único: `internal/client`, `internal/datasources`, `internal/resources`,
`internal/provider`, `CONTRIBUTING.md`. Los tests de import viven en el paquete externo
`resources_test` (importa `internal/provider` sin ciclo).

---

## Phase 1: Setup

- [X] T001 Comprobar la línea base en la rama `002-read-completo-tras-import` ejecutando `go build ./... && go vet ./... && go test ./...` desde la raíz; anotar cualquier fallo previo antes de tocar código
- [X] T001b (OMITIDA por decisión del usuario: no se crea issue) Crear el issue `[Bug] - Read no rellena todos los atributos tras import` (formato `[Type] - Summary` de `CLAUDE.md`, sin menciones de autoría de Claude) y anotar su número para la PR (Constitución, Flujo); pedir confirmación del título y la descripción con `AskUserQuestion` antes de crearlo
- [X] T002 Añadir la dependencia de test con `go get github.com/hashicorp/terraform-plugin-testing@latest && go mod tidy` en `go.mod`/`go.sum` (solo se usará en `_test.go`; Dependabot ya cubre `go.mod`)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: helper común de derivación y arnés del test por tabla, que necesitan todas las historias.

**⚠️ CRITICAL**: ninguna historia puede empezar hasta completar esta fase.

- [X] T003 Añadir `prefixLengthValue(cidr string) (types.Int64, error)` en `internal/resources/helpers.go` (derivación con `net.ParseCIDR`; error con `fmt.Errorf("invalid CIDR %q: %w", cidr, err)`) **conservando `cidrPrefixLength` sin cambios** para que `next_subnet.go`, `next_network.go` y `last_subnet.go` sigan compilando hasta US2 (T012/T013 lo eliminan al migrar los llamadores); añadir a `internal/resources/helpers_test.go` casos de `prefixLengthValue` (`10.1.4.0/24` → 24; IPv6; CIDR inválido → error)
- [X] T004 Crear el arnés en `internal/resources/read_after_import_test.go` (paquete `resources_test`): tipo `importCase{name string; newResource func() resource.Resource; path string; body string}`; función `readAfterImport(t, tc)` que levanta `httptest.NewServer` respondiendo `body` en `path` crea el cliente con el literal `&client.Client{BaseURL: server.URL, Token: "test", HTTPClient: http.DefaultClient}` (campos exportados; `newTestClient` es privado del paquete `resources` y no se puede usar desde `resources_test`, y así se evita la sonda `/health`), llama a `Configure` con `ProviderData`, obtiene el esquema (`Schema()`), construye un `tfsdk.State` con todos los atributos nulos salvo `id`, invoca `Read` y devuelve la respuesta. Función `assertComplete(t, name, schema, state)` que recorre `schema.Attributes` y falla con `<tipo>.<atributo>` si un atributo `IsRequired()` es nulo o cualquier atributo es desconocido (contrato C2 en [contracts/read-after-import.md](./contracts/read-after-import.md))

**Checkpoint**: `go build ./... && go test ./internal/resources/...` compila y pasa (la tabla aún vacía).

---

## Phase 3: User Story 1 - Importar una IP reservada sin que el plan la reemplace (Priority: P1) 🎯 MVP

**Goal**: `terraform import` de `ipzilon_next_ip_address` seguido de `plan` sin cambios ni reemplazo.

**Independent Test**: `go test ./internal/resources -run ReadAfterImport/next_ip_address` falla antes del arreglo (`subnet_id` nulo) y pasa después.

### Tests (escribir primero; deben fallar)

- [X] T005 [US1] Añadir a `internal/resources/read_after_import_test.go` la fila `ipzilon_next_ip_address` (`NewNextIPAddressResource`, `GET /ips/2204`, JSON con `id 2204`, `subnet_id 65`, `address`, `status reserved`, `is_azure_reserved false`, `hostname`/`description` no nulos) y `TestReadAfterImport` que ejecute `readAfterImport` + `assertComplete` por fila; comprobar además que `subnet_id == 65`. Ejecutarlo y confirmar que **falla** por `ipzilon_next_ip_address.subnet_id`
- [X] T006 [P] [US1] Añadir a `internal/resources/read_after_import_test.go` un test de igualdad Create/Read/Update para `next_ip_address`: con un servidor `httptest` que responde el mismo objeto a `POST /subnets/65/reserve-ip`, `GET /ips/2204` y `PATCH /ips/2204`, el estado resultante de las tres operaciones es idéntico (FR-004, US2 escenario 2). Añadir en el arnés (T004) los helpers para construir `tfsdk.Plan` y `tfsdk.State` desde un modelo (`tfsdk.Plan{Schema: s, Raw: ...}` con `tftypes` a partir del esquema) y para invocar `Create`, `Read` y `Update`; comparar los tres estados resultantes con `Equal`

### Implementación

- [X] T007 [US1] En `internal/resources/next_ip_address.go` crear `nextIPFromAPI(ip client.IPAddress) (nextIPAddressModel, error)` que construya el modelo entero (`ID` con `ipIDValue`, **`SubnetID: types.Int64Value(ip.SubnetID)`**, `Hostname`, `Description`, `Address`, `Status`, `IsAzureReserved` con `types.StringPointerValue` para los opcionales) y usarlo como único origen de `State.Set` en `Create`, `Read` y `Update` (eliminar las asignaciones `state.X = …`; el error de `nextIPFromAPI` se devuelve como diagnóstico). `Update` usa la respuesta del PATCH
- [X] T008 [US1] Ejecutar `go test ./internal/resources/...` y confirmar que T005 y T006 pasan

**Checkpoint**: el bloqueo de `camaras` queda resuelto a nivel unitario.

---

## Phase 4: User Story 2 - Import → plan vacío en los nueve recursos (Priority: P1)

**Goal**: los nueve recursos construyen el estado solo con `*FromAPI` en Create, Read y Update; `prefix_length` siempre derivado del `cidr`.

**Independent Test**: `go test ./internal/resources -run ReadAfterImport` verde con las 9 filas.

### Tests (escribir primero)

- [X] T009 [P] [US2] Añadir a `internal/resources/read_after_import_test.go` las filas de `ipzilon_next_subnet` (`GET /subnets/{id}`), `ipzilon_last_subnet` (ídem), `ipzilon_next_network` (`GET /networks/{id}`), con `cidr` `10.1.4.0/24` y comprobación de `prefix_length == 24`
- [X] T010 [P] [US2] Añadir a `internal/resources/read_after_import_test.go` las filas de `ipzilon_hub`, `ipzilon_scope`, `ipzilon_network`, `ipzilon_subnet` e `ipzilon_ip_address` con respuestas JSON completas
- [X] T011 [P] [US2] Añadir a `internal/resources/read_after_import_test.go` casos límite: CIDR no parseable en `next_subnet` → `Read` devuelve diagnóstico de error (no estado con `prefix_length` vacío); `description` nula en la API → nula en el estado (no cadena vacía); `Read` con 404 → recurso eliminado del estado

### Implementación

- [X] T012 [P] [US2] En `internal/resources/next_subnet.go` y `internal/resources/last_subnet.go` crear `nextSubnetFromAPI(s client.Subnet)` y `lastSubnetFromAPI(s client.Subnet)` (o una función compartida si los modelos son idénticos) que rellenen `ID`, `NetworkID`, `Name`, `CIDR`, `Description` y `PrefixLength` con `prefixLengthValue(s.CIDR)`; usarlas en `Create` (derivar `prefix_length` del CIDR devuelto, no de `plan.PrefixLength`), `Read` y `Update`; eliminar el `if err == nil` que tragaba el error
- [X] T013 [P] [US2] En `internal/resources/next_network.go` crear `nextNetworkFromAPI(n client.Network)` con `ID`, `ScopeID`, `Name`, `CIDR`, `Description`, `PrefixLength` (vía `prefixLengthValue`) y usarla en `Create`, `Read` y `Update` (hoy `Update` solo copia `Name`); quitar el error tragado
- [X] T014 [P] [US2] Revisar `internal/resources/hub.go`, `scope.go`, `network.go`, `subnet.go` e `ip_address.go`: confirmar que `Create`, `Read` y `Update` terminan siempre en `State.Set(ctx, <x>FromAPI(...))` sin leer `req.State` salvo para el `id`; corregir cualquier desvío (p. ej. usar la respuesta del PATCH en `Update`)
- [X] T015 [US2] Eliminar `cidrPrefixLength` de `internal/resources/helpers.go` (ya sin llamadores; `grep -rn cidrPrefixLength internal/` vacío) y ejecutar `go test ./internal/resources/...` y confirmar que T009–T011 pasan con las 9 filas

**Checkpoint**: 9/9 recursos con `Read` completo tras import.

---

## Phase 5: User Story 3 - Red de seguridad que impide una tercera recurrencia (Priority: P2)

**Goal**: la suite falla si un recurso deja atributos sin rellenar o si falta en la tabla.

**Independent Test**: regresión deliberada → la suite falla; recurso registrado sin fila → la suite falla.

- [X] T016 [US3] Añadir a `internal/resources/read_after_import_test.go` `TestReadAfterImportCoversAllResources`: instanciar `provider.New("test")()`, llamar a `Resources(ctx)`, obtener el `TypeName` de cada constructor (`Metadata` con `ProviderTypeName: "ipzilon"`) y comparar con los nombres de la tabla; fallar listando los que falten o sobren (FR-006)
- [X] T017 [US3] Comprobar que el test protege (quickstart §2): quitar temporalmente el relleno de `SubnetID` en `nextIPFromAPI`, ejecutar `go test ./internal/resources -run ReadAfterImport`, confirmar que falla con `ipzilon_next_ip_address.subnet_id` y revertir; repetir comentando la fila de un recurso para verificar T016
- [X] T018 [P] [US3] Confirmar por revisión (R7) que ningún data source de `internal/datasources/*.go` lee estado previo (`grep -n "req.State" internal/datasources/*.go` sin resultados) y dejar la regla escrita en `CONTRIBUTING.md` (sección "Nuevos recursos" de T022) en lugar de un test sobre el código fuente, que sería frágil

---

## Phase 6: User Story 4 - Verificación real antes de cada release (Priority: P2)

**Goal**: aceptación import → plan vacío por recurso contra un IPzilon real.

**Independent Test**: `TF_ACC=1 make testacc` con `IPZILON_API_URL`/`IPZILON_TOKEN` de una instancia de pruebas.

- [X] T019 [US4] Crear `internal/resources/acc_import_test.go` (paquete `resources_test`) con `testAccPreCheck` (exige `IPZILON_API_URL`, `IPZILON_TOKEN`; se salta sin `TF_ACC=1`) y `testAccProtoV6ProviderFactories` apuntando a `provider.New("test")`; añadir `TestAccImport_Hub`, `_Scope`, `_Network`, `_Subnet`, cada uno con `resource.Test`: paso 1 crear, paso 2 `ImportState`+`ImportStateVerify`, paso 3 `PlanOnly` con `plancheck.ExpectEmptyPlan()`. Dependencias entre recursos vía la config (hub → scope → network → subnet). Variables de entorno de prueba, validadas en `testAccPreCheck`: `IPZILON_TEST_SITE_ID` (site existente) y `IPZILON_TEST_ADDRESS_SPACE` (espacio libre, p. ej. `10.250.0.0/16`), con recursos nombrados `tf-acc-<random>`; todos los tests llevan `CheckDestroy` que comprueba con `GET` que el objeto ya no existe (hub, scope, network, subnet: 404; IPs: liberada)
- [X] T020 [US4] En `internal/resources/acc_import_test.go` añadir `TestAccImport_IPAddress`, `_NextIPAddress`, `_NextSubnet`, `_LastSubnet`, `_NextNetwork` con el mismo esquema de tres pasos (`next_*` y `last_subnet`: `ImportStateVerify` incluye `prefix_length` y, para `next_ip_address`, `subnet_id`)
- [X] T021 [US4] En `GNUmakefile` subir el timeout de `testacc` (de `120s` a `600s`) y comprobar que `go vet ./...` compila los tests de aceptación sin `TF_ACC`
- [X] T022 [US4] En `CONTRIBUTING.md` añadir la sección de release: pasos previos obligatorios `make test`, `make testacc` con import → plan vacío en los 9 recursos y, en la validación real, el caso de la IP 2204 antes de empujar el tag `vX.Y.Z` (FR-007); y añadir una sección "Nuevos recursos" con la regla (FR-011): todo recurso nuevo construye su estado con una función `<x>FromAPI` usada en Create, Read y Update, deriva de la API los atributos que esta no devuelve con el helper común, se añade a la tabla de `internal/resources/read_after_import_test.go` (el test de completitud falla si falta) y a los tests de aceptación; los data sources nunca leen estado previo

---

## Phase 7: User Story 5 - IPs migradas con hostname igual a description (Priority: P3)

**Goal**: comportamiento determinado y documentado (R6), sin compensación en el provider.

**Independent Test**: test unitario con `hostname == description` en la API y revisión del `plan` de la IP 2204.

- [X] T023 [P] [US5] Añadir a `internal/resources/read_after_import_test.go` un caso para `next_ip_address` e `ip_address` con `hostname` y `description` iguales en la API: el estado los refleja tal cual y, con `hostname` no configurado, el modelo no queda desconocido
- [X] T024 [US5] Documentar en `README.md` (sección de migración/import) y `CONTRIBUTING.md` que un diff in situ de `hostname` en IPs migradas es legítimo y converge con un `apply`, y cómo evitarlo (fijar `hostname` explícito); el texto se contrasta con la IP 2204 real en T029, que puede ajustarlo

---

## Phase 8: User Story 6 - Release v3.0.1 (Priority: P1)

**Goal**: publicar la corrección sin cambios de esquema.

**Independent Test**: en el stack `camaras`, `terraform import` + `plan` sin reemplazos usando la versión publicada.

- [X] T025 [US6] Regenerar la documentación con `make generate` y confirmar que `docs/` **no** cambia (`git diff --stat docs/` vacío; Principio IV, FR-010)
- [X] T026 [US6] Comprobar que no hay cambios de esquema ni entradas nuevas en *Breaking Changes* de `README.md` (SC-005) y que los ejemplos siguen fijando `~> 3.0`
- [X] T027 [US6] Puerta final antes de la PR: `go build ./... && go vet ./... && gofmt -l . && make test` (los mismos comandos que la CI) y corregir cualquier fallo
- [ ] T028 [US6] Abrir la PR (sin issue asociado, T001b omitida), con el commit generado con el comando `commit-message` (sin coautoría; pedir confirmación del mensaje con `AskUserQuestion` antes de commitear y de crear la PR)
- [ ] T029 [US6] Ejecutar `TF_ACC=1 make testacc` contra un IPzilon real y validar el caso de `camaras` (quickstart §4: `terraform import ipzilon_next_ip_address.ip_events_ilb 2204` + `plan` sin cambios) usando el provider local (`dev_overrides`); si el `plan` muestra un diff in situ de `hostname`, ajustar el texto de T024 en la misma PR
- [ ] T030 [US6] Tras mergear la PR: actualizar `main`, limpiar ramas locales y, **tras confirmación explícita** (acción externa e irreversible), publicar el tag `v3.0.1` (GoReleaser), según la sección de release de `CONTRIBUTING.md`; anunciar el desbloqueo de `camaras` y `avd`

---

## Dependencies & Execution Order

- **Phase 1 → Phase 2** bloquea todo. T003 y T004 son independientes entre sí ([P] posible, ficheros distintos).
- **US1 (Phase 3)** depende de Phase 2 y es el MVP. **US2 (Phase 4)** depende de Phase 2; sus tareas T012–T014 tocan ficheros distintos y son paralelas, pero comparten `read_after_import_test.go` con US1 (T009–T011 se añaden tras T005).
- **US3** depende de US1+US2 (tabla completa). **US4** puede empezar tras Phase 2 en paralelo a US1–US3 (fichero distinto), pero su ejecución completa exige US1+US2 hechas.
- **US5** depende de US1. **US6** depende de todas las anteriores (T029 requiere US1, US2 y US4).

## Parallel Examples

```text
Tras Phase 2:   T005+T007 (US1)   ‖   T019 (US4, fichero acc_import_test.go)
US2:            T012 ‖ T013 ‖ T014  (next_subnet/last_subnet, next_network, resto de recursos)
US2 tests:      T009 ‖ T010 ‖ T011  (misma tabla: aplicar en serie si se edita a la vez)
```

## Implementation Strategy

- **MVP**: Phase 1–3 (US1) desbloquea a `camaras` a nivel de código y deja el test que reproduce el bug.
- **Incremental**: US2 (9/9 recursos) → US3 (red de seguridad) → US4 (aceptación) → US5 (documentación de migradas) → US6 (release v3.0.1).
- **Orden recomendado**: TDD estricto en cada fase (test rojo antes de la implementación) y un commit por historia.
