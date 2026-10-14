---

description: "Tareas para hacer mockeable `items` de los data sources de listado (provider v3.2.2)"
---

# Tasks: Hacer mockeable `items` de los data sources de listado

**Input**: Documentos de diseño en `specs/006-mockable-list-items/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: SÍ. La spec los exige: un ejemplo ejecutable con `terraform test` (FR-006, FR-007) y
una guarda unitaria con `go test` (FR-008). El Principio V pide que el bug vaya con un test que lo
reproduzca sin API. Los tests se escriben antes del cambio y se comprueba que fallan.

**Organization**: tareas agrupadas por historia de usuario (US1–US3 de spec.md).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: se puede ejecutar en paralelo (ficheros distintos, sin dependencias pendientes)
- **[Story]**: historia a la que pertenece (US1, US2, US3)

## Path Conventions

Provider Go de proyecto único: `internal/datasources`, `test/mock/` (módulo de `terraform test`,
no se publica), `GNUmakefile`, `.github/workflows/ci.yml`, `docs/` (generado con `make generate`,
nunca a mano), `README.md`.

---

## Phase 1: Setup

- [X] T001 Comprobar la línea base en la rama `fix/26-mockable-items` ejecutando `go build ./... && go vet ./... && go test ./...` desde la raíz y `terraform version` (>= 1.16.0); anotar cualquier fallo previo antes de tocar código. La issue ya existe (#26): no se crea ninguna nueva; la PR la cerrará con `Closes #26`
- [X] T002 En `GNUmakefile`, añadir `testmock` a `.PHONY` y un objetivo `testmock` que: (1) cree un directorio temporal (`mktemp -d`), (2) compile el provider con `go build -o <tmp>/terraform-provider-ipzilon .`, (3) escriba `<tmp>/terraformrc` con `provider_installation { dev_overrides { "registry.terraform.io/mdepedrof/ipzilon" = "<tmp>" } direct {} }`, (4) ejecute `TF_CLI_CONFIG_FILE=<tmp>/terraformrc terraform -chdir=test/mock test` y (5) borre el temporal aunque el test falle, propagando el código de salida de `terraform test`. Todo en una sola receta de shell (`set -e` y `trap` para limpiar), sin `terraform init` (no hace falta con `dev_overrides`, research R5)

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: el helper que usan los 7 data sources (plan §Diseño, data-model §itemField).

- [X] T003 Crear `internal/datasources/items_test.go` con `TestItemsAttribute`: para `fields := []itemField{{"id", types.Int64Type, "Hub ID."}, {"name", types.StringType, "Hub name."}, {"is_azure_reserved", types.BoolType, "True for IPs auto-reserved by Azure (.1/.2/.3/broadcast)."}}` y `summary := "Matching hubs; an empty list when nothing matches."`, comprobar que `itemsAttribute(summary, fields)` devuelve un `schema.ListAttribute` con `Computed == true`, `ElementType` igual a `types.ObjectType{AttrTypes: map[string]attr.Type{"id": types.Int64Type, "name": types.StringType, "is_azure_reserved": types.BoolType}}` (usar `.Equal`) y `Description` exactamente igual a ``"Matching hubs; an empty list when nothing matches. Each item has: `id` — Hub ID. `name` — Hub name. `is_azure_reserved` — True for IPs auto-reserved by Azure (.1/.2/.3/broadcast)."`` (orden de declaración, separador `" — "`, campos unidos por un espacio; contrato en `specs/006-mockable-list-items/contracts/items-attribute.md`). Ejecutar `go test ./internal/datasources/` y comprobar que no compila / falla
- [X] T004 Crear `internal/datasources/items.go` con `type itemField struct { name string; typ attr.Type; description string }` y `func itemsAttribute(summary string, fields []itemField) schema.ListAttribute` (import `github.com/hashicorp/terraform-plugin-framework/datasource/schema`, `.../attr`, `.../types`). Construye `types.ObjectType{AttrTypes: ...}` con los campos y la descripción ``summary + " Each item has: " + strings.Join([]string{"`"+name+"` — "+description, ...}, " ")``. Comentario de una o dos líneas en inglés explicando por qué no se usa `ListNestedAttribute`: `terraform test` no puede mockear atributos anidados computados de tipo lista (hashicorp/terraform `internal/moduletest/mocking/fill.go`). Debe pasar T003. Depende de T003

**Checkpoint**: `go test ./internal/datasources/` pasa; ningún data source usa todavía el helper.

---

## Phase 3: User Story 1 - Probar con mocks un stack que lee data sources de listado (Priority: P1) 🎯 MVP

**Goal**: `items` de los 7 data sources acepta valores de `override_data` y `mock_data` en
`terraform test` (FR-001, FR-005, FR-006, FR-007, FR-008).

**Independent Test**: `make testmock` pasa cubriendo los 7 data sources; en `main` (antes del
cambio) falla con `incompatible types; expected object type, found tuple`.

### Tests for User Story 1 (escribir primero y ver que fallan)

- [X] T005 [P] [US1] Crear `test/mock/main.tf` con `terraform { required_providers { ipzilon = { source = "mdepedrof/ipzilon" } } }`, un bloque `provider "ipzilon" {}` y un `data` por cada uno de los 7 data sources (`ipzilon_hubs`, `ipzilon_ip_addresses`, `ipzilon_network_zones`, `ipzilon_networks`, `ipzilon_scopes`, `ipzilon_sites`, `ipzilon_subnets`) con los argumentos mínimos válidos según su `ValidateConfig`/`Read` (tomar como referencia `examples/data-sources/<nombre>/data-source.tf`; por ejemplo `ipzilon_hubs` con `name = "x"`). El de `ipzilon_hubs` reproduce literalmente la issue: `lifecycle { postcondition { condition = length(self.items) == 1  error_message = "len" } }` y `output "hub" { value = data.ipzilon_hubs.regional.items[0].id }`. Para cada uno de los otros 6, outputs `<ds>_count = length(data.<ds>.<n>.items)` y `<ds>_ids = [for i in data.<ds>.<n>.items : i.id]`. Sin credenciales ni datos reales (Principio IV)
- [X] T006 [P] [US1] Crear `test/mock/tests/list_data_sources.tftest.hcl` con `mock_provider "ipzilon" {}` y `override_data` para los 7 data sources de `test/mock/main.tf`, en `run` con `command = plan` y `assert`: (a) caso de la issue: `ipzilon_hubs` con `items = [{ id = 1 }]` → `output.hub == 1`; (b) un elemento parcial (solo `id`) en cada uno de los otros 6 → `count == 1` y `ids == [<id>]`; (c) en un `run` aparte, con `override_data` dentro del `run`, varios elementos en `ipzilon_sites` (`[{ id = 1, name = "a" }, { id = 2, name = "b" }]` → `sites_count == 2` e `sites_ids == [1, 2]`) y lista vacía en `ipzilon_subnets` (`items = []` → `subnets_count == 0`); no usar `ipzilon_hubs` en este `run`, porque su postcondición exige exactamente un elemento; (d) en el caso (b), comprobar en un data source que un campo no indicado recibe un valor generado (p. ej. un output `networks_first_name = data.ipzilon_networks.<n>.items[0].name` con `assert` `!= null`). Crear además `test/mock/tests/mock_data.tftest.hcl` con `mock_provider "ipzilon" { mock_data "<ds>" { defaults = { items = [{ id = 7 }] } } }` para los 7 data sources y un `run` que compruebe `count == 1` e `ids == [7]` (en `ipzilon_hubs`, `output.hub == 7`). Escenarios en `specs/006-mockable-list-items/contracts/items-attribute.md` §Mocks
- [X] T007 [US1] Ejecutar `make testmock` antes de cambiar los data sources y comprobar que falla con `Failed to compute attribute ".items": incompatible types; expected object type, found tuple.` (reproducción de la issue). Si falla por otro motivo (argumentos de `main.tf`, sintaxis), corregir T005/T006 hasta que el único error sea ese. Depende de T002, T005, T006
- [X] T008 [US1] Añadir a `internal/datasources/items_test.go` `TestListDataSourcesItemsAreMockable`: tabla con los 7 constructores (`NewHubsDataSource`, `NewIPAddressesDataSource`, … — comprobar los nombres reales en `internal/provider/provider.go`) y su lista de campos esperada de `specs/006-mockable-list-items/data-model.md` §Campos por data source; para cada uno, llamar a `Schema` y comprobar que `resp.Schema.Attributes["items"]` es `schema.ListAttribute` (type assertion; no `schema.ListNestedAttribute`), que su `ElementType` es igual (`.Equal`) al `types.ObjectType` esperado con exactamente esos nombres **y tipos** de `data-model.md` (Int64/String/Bool), para que un cambio de tipo, que alteraría el valor (FR-002), también falle, que es `Computed` y que `Description` contiene `` `<campo>` — `` para cada campo. Ejecutar `go test ./internal/datasources/` y comprobar que falla para los 7. Depende de T004

### Implementation for User Story 1

Cada tarea sustituye `xxxItemSchema` (`schema.NestedAttributeObject`) por `xxxItemFields []itemField`
con los mismos nombres, tipos y textos de `Description` literales, en el orden de la tabla de
`data-model.md`, y cambia `"items"` a `itemsAttribute("<resumen actual literal>", xxxItemFields)`.
No tocar el struct del modelo, `Read` ni las conversiones. Quitar imports que queden sin uso.

- [X] T009 [P] [US1] `internal/datasources/hubs.go`: `hubItemFields` con `id` (Int64, "Hub ID."), `site_id` (Int64, "ID of the site the hub belongs to."), `name` (String, "Hub name."), `address_space` (String, "Hub address space CIDR."), `location` (String, "Free-text location label."), `description` (String, "Free-text description."); resumen `"Matching hubs; an empty list when nothing matches."`
- [X] T010 [P] [US1] `internal/datasources/ip_addresses.go`: `ipAddrItemFields` con `id` (Int64, "IP record ID. Null for free addresses that have no stored record (IPzilon >= 3.0)."), `subnet_id` (Int64, "Subnet containing this address."), `address` (String, "IP address."), `status` (String, "IP status: available, used, or reserved."), `is_azure_reserved` (Bool, "True for IPs auto-reserved by Azure (.1/.2/.3/broadcast)."), `hostname` (String, "Hostname — semantic name for the address."), `description` (String, "Free-text description."); resumen `"Matching IP addresses; an empty list when nothing matches (a lookup by address returns exactly one)."`
- [X] T011 [P] [US1] `internal/datasources/network_zones.go`: `networkZoneItemFields` con `id` (Int64, "Zone ID."), `network_id` (Int64, "Network the zone belongs to."), `name` (String, "Zone name (unique within its network)."), `cidr` (String, "Zone CIDR block."), `description` (String, "Free-text description."); resumen `"Matching zones, ordered by CIDR."`
- [X] T012 [P] [US1] `internal/datasources/networks.go`: `networkItemFields` con `id` (Int64, "Network ID."), `scope_id` (Int64, "ID of the scope the network belongs to."), `name` (String, "Network name."), `cidr` (String, "Network CIDR block."), `description` (String, "Free-text description."); resumen `"Matching networks; an empty list when nothing matches."`
- [X] T013 [P] [US1] `internal/datasources/scopes.go`: `scopeItemFields` con `id` (Int64, "Scope ID."), `hub_id` (Int64, "ID of the hub the scope belongs to."), `parent_id` (Int64, "Parent scope ID (null for root scopes)."), `name` (String, "Scope name."), `kind` (String, "Scope kind (landing_zone or project)."), `cidr` (String, "CIDR block assigned to this scope."), `description` (String, "Free-text description."); resumen `"Matching scopes; an empty list when nothing matches."`
- [X] T014 [P] [US1] `internal/datasources/sites.go`: `siteItemFields` con `id` (Int64, "Site ID."), `name` (String, "Site name."), `type` (String, "Site type."), `description` (String, "Free-text description."); resumen `"Matching sites; an empty list when nothing matches."`
- [X] T015 [P] [US1] `internal/datasources/subnets.go`: `subnetItemFields` con `id` (Int64, "Subnet ID."), `network_id` (Int64, "ID of the network the subnet belongs to."), `name` (String, "Subnet name."), `cidr` (String, "Subnet CIDR block."), `description` (String, "Free-text description."), `zone_id` (Int64, "Zone that contains the subnet (null if none; IPzilon >= 3.1.0)."); resumen `"Matching subnets; an empty list when nothing matches."`
- [X] T016 [US1] Ejecutar `go build ./... && go test ./internal/datasources/` (pasa T008) y `make testmock` (pasan todos los `run`, `0 failed`). Depende de T007–T015
- [X] T017 [US1] En `.github/workflows/ci.yml`, añadir un job `mock` (`name: Mock tests (terraform test)`, `runs-on: ubuntu-latest`) con los pasos `actions/checkout@v7`, `actions/setup-go@v7` (`go-version-file: go.mod`, `cache: true`), `hashicorp/setup-terraform@v4` (`terraform_version: 1.16.x`, `terraform_wrapper: false`) y `run: make testmock` con `timeout-minutes: 5`; mantener el estilo y comentarios del fichero

**Checkpoint**: los 7 data sources se pueden mockear; `go test` y `make testmock` en verde.

---

## Phase 4: User Story 2 - Actualizar el provider sin tocar los stacks existentes (Priority: P1)

**Goal**: el contenido y el tipo de `items` en lecturas reales no cambian (FR-002, FR-003, FR-009).

**Independent Test**: los tests de `Read` existentes pasan sin cambiar sus expectativas y un
`plan` con estado de v3.2.1 no muestra cambios.

- [X] T018 [US2] Ejecutar `go test ./...` y comprobar que pasan sin modificar `internal/datasources/empty_lists_test.go`, `global_lists_test.go`, `ip_addresses_test.go`, `network_zones_test.go`, `subnets_test.go` ni `validation_test.go` (`git diff --stat -- internal/datasources/*_test.go` solo muestra `items_test.go`). En especial `items` sigue siendo lista vacía y no nula sin coincidencias (`empty_lists_test.go`) y los campos nulos de la API siguen llegando como nulos. Depende de T016
- [X] T019 [US2] Escenario de `specs/006-mockable-list-items/quickstart.md` §5 contra IPzilon local (ver la memoria de IPzilon local: build desde el tag si GHCR es privado, SQLite, token y sites): crear con v3.2.1 un estado que lea los 7 data sources, cambiar al binario local con `dev_overrides` y ejecutar `terraform plan` → `No changes.` y mismos outputs (criterio de aceptación de la issue: plan sin diferencias, SC-002). Anotar el resultado en esta tarea. Depende de T018 → **2026-10-14**: imagen local `ipzilon-local:3.4.0` construida desde el tag `v3.4.0`, solo backend con SQLite. Con el provider v3.2.1 del Registry, `apply` de hub, scopes, network, zona, subnet e IP más los 7 data sources (con postcondición sobre `ipzilon_hubs.items`, `one()`, `for`, indexación y una búsqueda sin resultados), 15 objetos en el estado; en v3.2.1 `items` sale en `providers schema -json` como `nested_type`. Con el binario local (`dev_overrides`), `items` pasa a `type` y `terraform plan -detailed-exitcode` → `No changes.` (exit 0); tras `apply -refresh-only` los outputs son idénticos a los de v3.2.1 (campos nulos siguen nulos, búsqueda vacía = `[]`)

**Checkpoint**: sin regresiones para consumidores.

---

## Phase 5: User Story 3 - Seguir entendiendo los campos de `items` en el Registry (Priority: P2)

**Goal**: la documentación generada describe cada campo de `items` (FR-004, FR-010).

**Independent Test**: en los 7 ficheros de `docs/data-sources/`, la descripción de `items` nombra
todos los campos con su texto.

- [X] T020 [US3] Regenerar con `make generate` (o `$(go env GOPATH)/bin/tfplugindocs generate --provider-name ipzilon` si ya está instalado); comprobar con `git diff --stat docs/` que solo cambian los 7 ficheros de `docs/data-sources/` y que en cada uno `items` aparece como `(List of Object)` con `Each item has:` y todos los campos de `data-model.md`, como en `contracts/items-attribute.md` §Documentación generada. Nunca editar `docs/` a mano (Principio IV). Depende de T016
- [X] T021 [P] [US3] En `README.md`, añadir antes de `## Compatibility` una sección `## Testing with mocks` (en inglés, ~80 columnas como el resto): desde provider v3.2.2 los `items` de los data sources de listado se pueden dar con `override_data` o `mock_data` en `terraform test` (Terraform >= 1.16.0), con un ejemplo mínimo en HCL de `mock_provider "ipzilon" {}` + `override_data { target = data.ipzilon_hubs.regional  values = { items = [{ id = 1 }] } }`. En `## Development`, añadir `make testmock     # terraform test with mock_provider (requires terraform)` bajo `make testacc`

**Checkpoint**: docs y README actualizados.

---

## Phase 6: Polish & Cross-Cutting Concerns

- [X] T022 Ejecutar `gofmt -s -l .` (sin salida), `go build ./... && go vet ./... && go test ./...`, `make testmock` y `go list -deps . | grep terraform-plugin-sdk` (debe devolver vacío) (Principio V, restricciones técnicas)
- [X] T023 Ejecutar de nuevo `make generate` y comprobar que `git status --short docs/` no muestra cambios respecto a T020 (quickstart §4)
- [ ] T024 Commit con el comando `commit-message` (confirmando el mensaje con `AskUserQuestion`, sin `Co-Authored-By`), PR a `main` con la plantilla completada, `Closes #26` y la explicación de la enmienda v1.0.3 de la constitución (Principio IV: los campos de una lista de objetos se describen en la `Description` del atributo que los contiene; ya aplicada en `.specify/memory/constitution.md`); tras el merge, tag `v3.2.2` (confirmando antes con `AskUserQuestion`); después actualizar `main`, limpiar ramas locales y marcar la spec como cerrada

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias.
- **Foundational (Phase 2)**: tras T001; bloquea US1.
- **US1 (Phase 3)**: tras Phase 2 (T005–T007 solo necesitan T002 y pueden empezar antes).
- **US2 (Phase 4)** y **US3 (Phase 5)**: tras T016.
- **Polish (Phase 6)**: tras todo lo anterior (T024 al final).

### Within Each User Story

- US1: T005, T006 en paralelo → T007 (ver fallar) ; T008 (ver fallar) → T009–T015 en paralelo → T016 → T017.
- US2: T018 → T019.
- US3: T020 y T021 en paralelo.

### Ficheros compartidos (no paralelizar entre sí)

- `internal/datasources/items_test.go`: T003, T008.
- `README.md`: solo T021.

## Parallel Example: User Story 1

```text
T005 test/mock/main.tf                 |  T006 test/mock/tests/*.tftest.hcl
T009 hubs.go  T010 ip_addresses.go  T011 network_zones.go  T012 networks.go
T013 scopes.go  T014 sites.go  T015 subnets.go         (tras T004 y T008)
```

## Implementation Strategy

### MVP (US1)

1. T001–T004 → T005–T008 (tests en rojo) → T009–T016: los 7 data sources mockeables.

### Incremental Delivery

1. US1 (mockeable + CI).
2. US2 (verificación de compatibilidad).
3. US3 (docs y README).
4. Polish y release única **v3.2.2** en una sola PR.
