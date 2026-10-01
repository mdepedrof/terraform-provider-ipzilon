---

description: "Tareas para filtros sin ids, lista vacía y mover hubs de site (IPzilon 3.2.0, provider v3.2.0)"
---

# Tasks: Filtros sin ids, lista vacía y mover hubs de site (IPzilon 3.2.0)

**Input**: Documentos de diseño en `specs/004-global-filters-hub-move/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: SÍ se incluyen. La issue #16 los pide expresamente ("con test unitario", "Tests
unitarios de construcción de URL y validación de filtros") y el Principio V exige unitarios sin
API para la lógica pura y SHOULD de aceptación antes de la release. Unitarios de data sources con
`fakeAPI`/`readDataSource`/`itemsOf` (`internal/datasources/network_zones_test.go`); de recursos
con `newFakeAPI`/`newHarness` (`internal/resources/harness_test.go`); aceptación con
`terraform-plugin-testing` (`internal/resources/acc_import_test.go`, `testAccPreCheck`).

**Organization**: tareas agrupadas por historia de usuario (US1–US4 de spec.md).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: se puede ejecutar en paralelo (ficheros distintos, sin dependencias pendientes)
- **[Story]**: historia a la que pertenece (US1…US4)

## Path Conventions

Provider Go de proyecto único: `internal/client`, `internal/datasources`, `internal/resources`,
`examples/`, `docs/` (generado con `make generate`, nunca a mano), `README.md`.

---

## Phase 1: Setup

- [X] T001 Comprobar la línea base en la rama `004-global-filters-hub-move` ejecutando `go build ./... && go vet ./... && go test ./...` desde la raíz; anotar cualquier fallo previo antes de tocar código. Las issues ya existen (#16 y #21): no se crea ninguna nueva; la PR las cerrará con `Closes #16` y `Closes #21`

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: constantes de versión, predicado `405`, helpers de URL/validación y utilidades de test que usan varias historias.

**⚠️ CRITICAL**: ninguna historia puede empezar hasta completar esta fase.

- [X] T002 [P] En `internal/client/client.go` añadir, junto a `MinZonesAPIVersion`, las constantes `MinGlobalListsAPIVersion = "3.2.0"` (comentario: primera release con `GET /hubs/`, `/scopes/`, `/networks/`, `/subnets/`; solo la exigen las búsquedas sin id de padre) y `MinHubMoveAPIVersion = "3.2.0"` (comentario: primera release que acepta `site_id` en `PATCH /hubs/{id}`; las anteriores lo ignoran sin error), y el predicado `IsMethodNotAllowed(err error) bool` que devuelve `apiErrorCode(err) == http.StatusMethodNotAllowed` (research R4)
- [X] T003 [P] En `internal/client/client_test.go` añadir tests de `IsMethodNotAllowed` (`&APIError{Code: 405}` → true; 404, 409 y un error no API → false) y casos `RequireAPIVersion(MinGlobalListsAPIVersion, "x")`: `3.1.0`, `3.1.9` → error que envuelve `ErrUnsupportedAPIVersion` y contiene `requires IPzilon >= 3.2.0`; `3.2.0`, `3.3.1`, `""`, `0.0.0-dev` → `nil` (depende de T002)
- [X] T004 [P] Crear `internal/datasources/validation.go` con el helper puro `networkCIDR(s string) (network string, err error)`: `netip.ParsePrefix(s)`; si falla → `fmt.Errorf("'%s' is not a valid CIDR", s)`; si `p.Masked() != p` → `fmt.Errorf("'%s' is not a network address; did you mean '%s'?", s, p.Masked())`; si no, devuelve `p.String()`. Y `validateGlobalCIDR(ctx, cfg tfsdk.Config, attr string, diags *diag.Diagnostics)` que lee el atributo String `attr`, no hace nada si es nulo o desconocido, y si `networkCIDR` falla añade `diags.AddAttributeError(path.Root(attr), "Invalid CIDR", err.Error())` (research R5)
- [X] T005 [P] En `internal/datasources/helpers.go` añadir `requireGlobalLists(c *client.Client, feature string, diags *diag.Diagnostics) bool` (llama a `c.RequireAPIVersion(client.MinGlobalListsAPIVersion, feature)`; si falla añade el error `"IPzilon version not supported"` con el mensaje y devuelve `false`; sin peticiones) y `globalListError(diags *diag.Diagnostics, summary, feature string, err error)`: si `client.IsMethodNotAllowed(err)` añade `"IPzilon version not supported"` con `fmt.Sprintf("%s requires IPzilon >= %s: the server has no global listing (%s)", feature, client.MinGlobalListsAPIVersion, err)`; si no, `diags.AddError(summary, err.Error())` (research R4)
- [X] T006 En `internal/datasources/network_zones_test.go` ampliar `fakeAPI`: si el cuerpo de una ruta empieza por `status:<código> ` (p. ej. `status:405 {"detail":"Method Not Allowed"}`), responder con ese código HTTP y el resto como cuerpo; el resto de rutas no cambia. Añadir el helper `validateDataSource(t, d datasource.DataSource, vals map[string]any) datasource.ValidateConfigResponse` que construye la configuración igual que `readDataSource` (admitiendo además el marcador `unknown` → `tftypes.UnknownValue`) y llama a `d.(datasource.DataSourceWithValidateConfig).ValidateConfig`
- [X] T007 Crear `internal/datasources/validation_test.go` con tests por tabla de `networkCIDR` (`10.0.16.0/22` → OK y mismo texto; `10.0.16.5/22` → error con `did you mean '10.0.16.0/22'?`; `foo` → `is not a valid CIDR`; `2001:db8::/32` → OK; `2001:db8::1/32` → error con sugerencia) y de `requireGlobalLists` (cliente `APIVersion: "3.1.0"` → `false` + diagnóstico; `"3.2.0"` y `""` → `true`) y `globalListError` (405 → resumen `IPzilon version not supported` y texto `requires IPzilon >= 3.2.0`; 404 → resumen original con el mensaje de la API) (depende de T002, T004, T005)

**Checkpoint**: `go build ./... && go test ./internal/client/... ./internal/datasources/...` en verde.

---

## Phase 3: User Story 1 - Recorrer listados vacíos sin errores (Priority: P1) 🎯 MVP

**Goal**: `ipzilon_sites`, `ipzilon_hubs`, `ipzilon_scopes`, `ipzilon_networks` e `ipzilon_ip_addresses` devuelven `items = []` sin resultados (FR-001).

**Independent Test**: cada data source contra una API falsa que responde `{"items":[],"total":0}` deja `items` como lista no nula de longitud 0 (quickstart §2, escenarios 1–2).

### Tests for User Story 1

- [X] T008 [P] [US1] Crear `internal/datasources/empty_lists_test.go` con un test por tabla que, para `NewSitesDataSource` (`name = "x"`, ruta `/sites/?name=x&limit=1000&offset=0`), `NewHubsDataSource` (`site_id = 1`, `/sites/1/hubs?limit=1000&offset=0`), `NewScopesDataSource` (`hub_id = 1`, `/hubs/1/scopes?limit=1000&offset=0`; y otro caso con `kind = "project"` para cubrir el filtro en cliente), `NewNetworksDataSource` (`scope_id = 1`, `/scopes/1/networks?limit=1000&offset=0`) y `NewIPAddressesDataSource` (`subnet_id = 1`, `status = "reserved"`, `/subnets/1/ips?status=reserved&limit=1000&offset=0`), responda `{"items":[],"total":0}` y compruebe que `items` en el estado **no es nulo** y tiene longitud 0 (comprobar las rutas exactas que genera `GetAll` antes de fijarlas). Debe fallar antes de T009. `ipzilon_subnets` e `ipzilon_network_zones` ya están cubiertos por `TestSubnetsEmptyIsAnEmptyList` y `TestNetworkZonesEmptyIsNotAnError` (SC-002: siete data sources)

### Implementation for User Story 1

- [X] T009 [US1] Sustituir `var items []T` por `items := []T{}` en `Read` de `internal/datasources/sites.go`, `hubs.go`, `scopes.go`, `networks.go` e `ip_addresses.go` (en las ramas por id seguir asignando el slice de un elemento); en `scopes.go` comprobar que el filtro `kind` (`filtered := items[:0]`) sigue devolviendo un slice no nulo (research R1). T008 en verde

**Checkpoint**: US1 funcional por sí sola; no depende de IPzilon 3.2.0.

---

## Phase 4: User Story 2 - Localizar una IP por su dirección (Priority: P1)

**Goal**: filtro `address` en `ipzilon_ip_addresses` (FR-002–FR-005).

**Independent Test**: con API falsa, `subnet_id + address` devuelve un elemento ocupado o libre y falla si la API devuelve lista vacía; las combinaciones prohibidas fallan en `ValidateConfig` (quickstart §2, escenarios 3–6).

### Tests for User Story 2

- [X] T010 [US2] Ampliar `internal/datasources/ip_addresses_test.go`: (a) `subnetIPsURL(1, nil, ptr("10.0.1.17"))` → `/subnets/1/ips?address=10.0.1.17`, `subnetIPsURL(1, ptr("used"), nil)` → `/subnets/1/ips?status=used` y sin filtros → `/subnets/1/ips`; (b) `Read` con `subnet_id = 42`, `address = "10.0.1.17"` y respuesta con `{"id":7,…,"status":"used"}` → 1 elemento con `id = 7`; (c) dirección libre con `{"id":null,…,"status":"available"}` → 1 elemento con `id` nulo y `status = "available"`; (d) respuesta `{"items":[],"total":0}` → diagnóstico de error `IP address not found` cuyo detalle contiene `10.0.1.99` y `subnet 42`; (e) `validateDataSource`: `address = "nope"` → `Invalid address`; `address` sin `subnet_id` → error `Missing filter`; `address` + `status` y `address` + `id` → `Conflicting filters`; `address` desconocido → sin error; `subnet_id + address` válidos (IPv4 `10.0.1.17` e IPv6 `2001:db8::10`) → sin error (depende de T006)

### Implementation for User Story 2

- [X] T011 [US2] En `internal/datasources/ip_addresses.go`: añadir `Address types.String \`tfsdk:"address"\`` a `ipAddressesModel` y el atributo `"address": schema.StringAttribute{Optional: true, Description: "Look up one address of subnet_id, occupied or free (a free address is returned with id = null and status = available). Requires subnet_id; cannot be combined with id or status. Fails if the address is not in the subnet."}`; actualizar la `Description` del data source para mencionar `address`; cambiar `subnetIPsURL(subnetID int64, status, address *string)` para añadir `status` o `address` con `url.Values`; en `Read`, si hay `address`, llamar a `GetAll` con esa URL y si devuelve 0 elementos añadir `resp.Diagnostics.AddError("IP address not found", fmt.Sprintf("%s is not an address of subnet %d", address, subnetID))` (research R2, FR-005)
- [X] T012 [US2] Implementar `ValidateConfig` (`var _ datasource.DataSourceWithValidateConfig = &IPAddressesDataSource{}`) en `internal/datasources/ip_addresses.go`: si `address` es conocido y no nulo y `netip.ParseAddr` falla → `AddAttributeError(path.Root("address"), "Invalid address", "'<v>' is not a valid IP address")`; si `address` no es nulo (conocido o desconocido) y `subnet_id` es nulo → `AddAttributeError(..., "Missing filter", "address requires subnet_id")`; si `address` no es nulo y `id` o `status` no son nulos → `"Conflicting filters"`, `"address cannot be combined with id or status"` (FR-003, FR-004). T010 en verde
- [X] T013 [P] [US2] Añadir a `examples/data-sources/ipzilon_ip_addresses/data-source.tf` un bloque `data "ipzilon_ip_addresses" "gw" { subnet_id = 1  address = "10.0.1.17" }` con comentario (dirección estable frente al id; libre → `id = null`, `status = "available"`) y un `output` con `one(data.ipzilon_ip_addresses.gw.items).status`

**Checkpoint**: US2 funcional; no depende de IPzilon 3.2.0 (ruta existente desde 3.0.0).

---

## Phase 5: User Story 3 - Localizar hubs, scopes, networks y subredes sin conocer ids (Priority: P1)

**Goal**: búsquedas sin id de padre mediante los listados globales de IPzilon 3.2.0 (FR-006–FR-016), manteniendo los listados por padre cuando hay id de padre (research R3).

**Independent Test**: con API falsa, cada data source sin padre llama exactamente a la ruta global esperada; con padre, a la ruta actual; contra versión `3.1.0` falla sin peticiones; `405` → error de versión; `ValidateConfig` rechaza CIDR con bits de host solo en búsquedas globales (quickstart §2, escenarios 7–12).

### Tests for User Story 3

- [X] T014 [P] [US3] Añadir a `internal/datasources/helpers_test.go` tests por tabla de los constructores nuevos (orden de `url.Values.Encode()`): `globalHubsURL(nil, nil)` → `/hubs/`; `globalHubsURL(ptr("10.0.0.0/16"), ptr("hub-weu"))` → `/hubs/?address_space=10.0.0.0%2F16&name=hub-weu`; `globalScopesURL` con `name`, `cidr`, `kind`, `parentID` → `/scopes/?cidr=…&kind=project&name=avd&parent_id=12`; `globalNetworksURL(ptr("10.0.16.0/22"), nil)` → `/networks/?cidr=10.0.16.0%2F22`; `globalSubnetsURL` con `name`, `cidr`, `networkID`, `zoneID` → `/subnets/?cidr=…&name=…&network_id=7&zone_id=9`; sin filtros → ruta sin `?`
- [X] T015 [P] [US3] Crear `internal/datasources/global_lists_test.go` con tests de `Read` (versión `3.2.0` salvo que se indique): (a) `ipzilon_networks` con solo `cidr = "10.0.16.0/22"` → única llamada `/networks/?cidr=10.0.16.0%2F22&limit=1000&offset=0` y el elemento devuelto; (b) `ipzilon_hubs` con solo `address_space`; (c) `ipzilon_scopes` con `kind = "project"`, `name = "avd"` → `kind` enviado al servidor; (d) `ipzilon_scopes` con solo `parent_id = 12` → `/scopes/?parent_id=12…`; (e) `ipzilon_subnets` con solo `cidr` y con `cidr` + `zone_id` → `/subnets/?…`; (f) dos elementos con el mismo CIDR → `items` de longitud 2 (R6); (g) sin ningún filtro en hubs/scopes/networks → listado global completo; (h) con padre (`site_id`, `hub_id`, `scope_id`, `network_id` sin `name`/`cidr`, `zone_id` solo) → mismas rutas que hoy y **ninguna** ruta global, también con versión `3.1.0` (FR-016); (i) versión `3.1.0` sin padre → error `IPzilon version not supported` con `requires IPzilon >= 3.2.0` y **cero** peticiones; (j) versión `""` y respuesta `status:405 {"detail":"Method Not Allowed"}` → mismo error de versión; (k) respuesta `status:404 {"detail":"Hub not found"}` con `hub_id` en scopes globales → error que contiene `Hub not found` (FR-014); (l) `ipzilon_subnets` sin ningún filtro → sigue fallando con `Missing filter` (depende de T006)
- [X] T016 [P] [US3] Crear `internal/datasources/global_validation_test.go` con tests de `ValidateConfig` vía `validateDataSource`: `ipzilon_networks { cidr = "10.0.16.5/22" }` → `Invalid CIDR` con `did you mean '10.0.16.0/22'?`; igual con `scope_id = 1` → **sin** error (búsqueda por padre, comportamiento actual); igual con `scope_id = unknown` → sin error; `ipzilon_hubs { address_space = "foo" }` → `Invalid CIDR`; `ipzilon_scopes { cidr = "10.0.16.5/20" }` → error y con `hub_id` → sin error; `ipzilon_scopes { root_only = true }` sin `hub_id` → `Missing filter` con `root_only requires hub_id`; `ipzilon_subnets { cidr = "10.0.16.65/26" }` → error (las búsquedas con `name`/`cidr` de subnets son siempre globales); `ipzilon_subnets { network_id = 7, no_zone = true, name = "x" }` → `Conflicting filters`; `cidr` desconocido → sin error (depende de T006)

### Implementation for User Story 3

- [X] T017 [US3] En `internal/datasources/helpers.go` añadir los constructores `globalHubsURL(addressSpace, name *string) string` (`/hubs/`; sin `site_id`: con site se usa `siteHubsURL`, research R3), `globalScopesURL(name, cidr, kind *string, parentID *int64) string` (`/scopes/`), `globalNetworksURL(cidr, name *string) string` (`/networks/`) y `globalSubnetsURL(name, cidr *string, networkID, zoneID *int64) string` (`/subnets/`), con el mismo estilo que `zonesURL` (`url.Values`, sin `?` si no hay filtros) — contracts/api-client.md. T014 en verde
- [X] T018 [P] [US3] En `internal/datasources/hubs.go`: sustituir `validateFilters` por la lógica de R3 (si hay `id` → búsqueda singular como hoy, ignorando el resto; si hay `site_id` → `siteHubsURL` como hoy; si no → `requireGlobalLists(d.client, "ipzilon_hubs without site_id", …)` y `GetAll` sobre `globalHubsURL(address_space, name)`, con errores vía `globalListError`); actualizar `Description` del data source ("Provide id (singular lookup) or any combination of site_id, name and address_space. Without site_id the lookup is global (IPzilon >= 3.2.0).") y de `site_id`, `name`, `address_space` (quitar "Only applies when site_id is set"; `address_space` sin `site_id` compara la red y no admite bits de host); implementar `ValidateConfig`: si `site_id` es nulo y `id` es nulo → `validateGlobalCIDR(…, "address_space")` (FR-006, FR-012)
- [X] T019 [P] [US3] En `internal/datasources/scopes.go`: si hay `id` → como hoy; si hay `hub_id` → `hubScopesURL` + filtro `kind` en cliente como hoy; si no → `requireGlobalLists(…, "ipzilon_scopes without hub_id")` y `globalScopesURL(name, cidr, kind, parent_id)` (kind en servidor). Actualizar descripciones (`hub_id` opcional; `kind`: "With hub_id it is filtered client-side; without hub_id it is filtered server-side (IPzilon >= 3.2.0)."; `parent_id`: sin `hub_id` devuelve los hijos directos; `root_only`: "Requires hub_id."). `ValidateConfig`: `root_only` verdadero y `hub_id` nulo → `AddAttributeError(path.Root("root_only"), "Missing filter", "root_only requires hub_id: the global scope listing has no root_only filter")`; si `hub_id` y `id` son nulos → `validateGlobalCIDR(…, "cidr")`. Mantener el error actual `parent_id and root_only are mutually exclusive` (FR-007, FR-012, FR-013)
- [X] T020 [P] [US3] En `internal/datasources/networks.go`: mantener los errores actuales de `id` con padre y de `hub_id` + `scope_id`; quitar el error `Missing filter` cuando no hay padre y en ese caso usar `requireGlobalLists(…, "ipzilon_networks without hub_id or scope_id")` + `globalNetworksURL(cidr, name)`; actualizar descripciones (data source: "Provide id (singular), hub_id, scope_id or only name/cidr. Without hub_id and scope_id the lookup is global (IPzilon >= 3.2.0)."; `cidr`/`name` aplican siempre). `ValidateConfig`: si `id`, `hub_id` y `scope_id` son nulos → `validateGlobalCIDR(…, "cidr")` (FR-008, FR-012)
- [X] T021 [P] [US3] En `internal/datasources/subnets.go`: añadir `Name`/`CIDR types.String` (`tfsdk:"name"`/`"cidr"`) al modelo y al esquema (`name`: "Filter: exact name match (server-side, IPzilon >= 3.2.0). network_id is not required."; `cidr`: "Filter: subnets whose network equals this CIDR (server-side, IPzilon >= 3.2.0; no host bits). network_id is not required."); en `Read`, si hay `name` o `cidr` (y no `id`) → `requireGlobalLists(…, "name/cidr filters in ipzilon_subnets")` y `GetAll` sobre `globalSubnetsURL(name, cidr, network_id?, zone_id?)` con `globalListError`; en otro caso, comportamiento actual intacto (incluida la resolución de la network con solo `zone_id`). Ajustar `validateFilters` para que `name`/`cidr` cuenten como filtro. `ValidateConfig`: `no_zone` verdadero con `name` o `cidr` no nulos → `"Conflicting filters"`, `"no_zone cannot be combined with name or cidr"`; `validateGlobalCIDR(…, "cidr")` siempre que `id` sea nulo (FR-009, FR-013)
- [X] T022 [US3] Ejecutar `go test ./internal/datasources/...`: T015 y T016 en verde (depende de T017–T021)
- [X] T023 [P] [US3] Actualizar ejemplos: `examples/data-sources/ipzilon_hubs/data-source.tf` (búsqueda solo por `address_space`), `examples/data-sources/ipzilon_scopes/data-source.tf` (`kind = "project"` + `name`), `examples/data-sources/ipzilon_networks/data-source.tf` (solo `cidr = "10.0.16.0/22"` y `output` con `one(...items).id`), `examples/data-sources/ipzilon_subnets/data-source.tf` (solo `cidr`); comentar en cada uno que sin id de padre se requiere IPzilon >= 3.2.0 y que un CIDR repetido entre hubs puede devolver varios elementos
- [X] T024 [US3] Añadir `TestAccDataSources_GlobalLookup` (implementado en el fichero nuevo `internal/resources/acc_global_test.go`, mismo paquete y harness que `internal/resources/acc_import_test.go`) (`TF_ACC`, `testAccPreCheck`, IPzilon 3.2.0): crear hub/scope/network/subnet con nombres únicos y comprobar con `resource.TestCheckResourceAttrPair` que `ipzilon_hubs { address_space }`, `ipzilon_scopes { kind, name }`, `ipzilon_networks { cidr }` e `ipzilon_subnets { cidr }` devuelven `items.0.id` igual al recurso creado, y que `ipzilon_networks { name = "<inexistente>" }` deja `items.#` = `0`

**Checkpoint**: US3 funcional; las configuraciones con id de padre siguen idénticas.

---

## Phase 6: User Story 4 - Mover un hub de site sin recrearlo (Priority: P2)

**Goal**: `ipzilon_hub.site_id` actualizable in situ con IPzilon ≥ 3.2.0 (FR-017–FR-020, research R7).

**Independent Test**: con `newFakeAPI`/`newHarness`, el plan de un cambio de `site_id` no exige reemplazo, `ModifyPlan` falla con versión `3.1.0`, `Update` envía `site_id` solo si cambia y detecta un servidor que lo ignora (quickstart §2, escenarios 13–15).

### Tests for User Story 4

- [X] T025 [US4] Crear `internal/resources/hub_test.go` con: (a) el esquema de `site_id` no tiene plan modifiers (`len(PlanModifiers) == 0`); (b) `modifyPlan` con estado `site_id = 1` y plan `site_id = 2`, versión `3.1.0` → error `IPzilon version not supported` con `moving ipzilon_hub to another site requires IPzilon >= 3.2.0` y cero peticiones; con versión `3.2.0` o `""` → sin error; mismo `site_id` con versión `3.1.0` → sin error; sin estado (create) → sin error; (c) `update` de 1 → 2 con versión `3.2.0` y `PATCH /hubs/5` que devuelve `site_id: 2` → el cuerpo enviado contiene `"site_id":2`, el estado queda con `site_id = 2` e `id = 5`; (d) `update` solo de nombre → el cuerpo **no** contiene `site_id`; (e) `PATCH` que devuelve `site_id: 1` tras pedir 2 (versión `""`) → error con `the server ignored site_id` y estado con `site_id = 1`; (f) `PATCH` con `409 {"detail":"Hub name 'x' already exists in site 2"}` → error `Update hub failed` que contiene el detalle (FR-018); (g) deriva: estado con `site_id = 2` (hub movido fuera de Terraform) y configuración con `site_id = 1` → `modifyPlan` con versión `3.2.0` sin error y sin `RequiresReplace` en la respuesta (actualización in situ, no recreación). Debe fallar antes de T026–T028

### Implementation for User Story 4

- [X] T026 [P] [US4] En `internal/client/models.go` añadir a `HubUpdate` el campo `SiteID *int64 \`json:"site_id,omitempty"\`` con comentario: "set only when the hub moves to another site (IPzilon >= 3.2.0; older releases ignore it)" (data-model.md §Cliente)
- [X] T027 [US4] En `internal/resources/hub.go`: quitar `int64planmodifier.RequiresReplace()` de `site_id` y actualizar su `Description` a "ID of the site this hub belongs to. Changing it moves the hub in place, keeping its id and everything under it (IPzilon >= 3.2.0)."; añadir `var _ resource.ResourceWithModifyPlan = &HubResource{}` y `ModifyPlan` que, si `r.client != nil`, el plan y el estado no son nulos y el `site_id` planificado es conocido y distinto del del estado, llama a `r.client.RequireAPIVersion(client.MinHubMoveAPIVersion, "moving ipzilon_hub to another site")` y añade el error `"IPzilon version not supported"` si falla; eliminar el import de `int64planmodifier` si queda sin uso
- [X] T028 [US4] En `Update` de `internal/resources/hub.go`: si `plan.SiteID` ≠ `state.SiteID`, repetir la comprobación de versión y rellenar `HubUpdate.SiteID`; tras el `PATCH`, si se pidió mover y `h.SiteID` ≠ el pedido, guardar `hubFromAPI(h)` en el estado y añadir `"IPzilon version not supported"` con `fmt.Sprintf("moving ipzilon_hub to another site requires IPzilon >= %s: the server ignored site_id (hub %d is still in site %d)", client.MinHubMoveAPIVersion, h.ID, h.SiteID)`; los errores del `PATCH` se siguen mostrando como `"Update hub failed"` con el mensaje de la API (FR-018, FR-019). T025 en verde
- [X] T029 [P] [US4] Añadir a `examples/resources/ipzilon_hub/resource.tf` un comentario sobre `site_id`: cambiarlo mueve el hub in situ conservando su id y el de sus scopes, networks y subredes (IPzilon >= 3.2.0); IPzilon rechaza sites de otro tipo, nombres repetidos o `address_space` solapado en el destino
- [X] T030 [US4] Añadir `TestAccHub_MoveSite` (en `internal/resources/acc_global_test.go`): se salta con `t.Skip` si `IPZILON_TEST_ALT_SITE_ID` está vacío; paso 1 crea hub + scope + network + subnet en `IPZILON_TEST_SITE_ID`; paso 2 cambia `site_id` del hub a `IPZILON_TEST_ALT_SITE_ID` con `ConfigPlanChecks.PreApply` = `plancheck.ExpectResourceAction("ipzilon_hub.test", plancheck.ResourceActionUpdate)` y `ExpectResourceAction` `NoOp` para scope/network/subnet; comprobar que el `id` del hub no cambia (`statecheck.CompareValue`) y que el plan posterior está vacío (SC-004)

**Checkpoint**: US4 funcional contra IPzilon 3.2.0; contra 3.1.x falla en `plan` con la versión mínima.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T031 Añadir `Description` a los atributos de elemento que no la tienen (Principio IV): `id` en `siteItemSchema`, `hubItemSchema`, `scopeItemSchema`, `networkItemSchema` y `subnetItemSchema` (p. ej. "Hub ID."); `site_id` ("ID of the site the hub belongs to."), `hub_id` y `parent_id` ("Parent scope ID (null for root scopes)."), `scope_id` y `network_id` en `internal/datasources/{sites,hubs,scopes,networks,subnets}.go`; y a `items` ("Matching <entity>; an empty list when nothing matches.") en todos los data sources de listado, incluidos `ip_addresses.go` y `network_zones.go` si les falta. Comprobar con `grep -n 'Computed: true}' internal/datasources/*.go` que no queda ninguno
- [X] T032 Regenerar la documentación con `make generate` y revisar que `docs/data-sources/{hubs,scopes,networks,subnets,ip_addresses}.md` y `docs/resources/hub.md` reflejan los atributos nuevos (`address`, `name`/`cidr` de subnets) y las descripciones modificadas; commitear `docs/` junto con el cambio de esquema (Principio IV)
- [X] T033 [P] Actualizar `README.md`: en *Compatibility* cambiar la fila `~> 3.0` a `>= 3.0.0 (network zones: >= 3.1.0, provider >= 3.1.0; lookups without parent ids and moving hubs between sites: >= 3.2.0, provider >= 3.2.0)`; añadir una sección breve *Lookups without ids* (ejemplo `ipzilon_networks { cidr = ... }` con `one()`, CIDR por red sin bits de host, posibles varios resultados, `items = []` sin coincidencias, `ipzilon_ip_addresses` con `address`) y una nota de que cambiar `site_id` de `ipzilon_hub` ya no recrea el hub; mantener el pin `~> 3.0`
- [X] T034 Ejecutar `go build ./... && go vet ./... && go test ./...` y `go list -deps . | grep terraform-plugin-sdk` (debe salir vacío); corregir cualquier fallo
- [X] T035 Ejecutar `TF_ACC=1 make testacc` contra IPzilon 3.2.0 (`IPZILON_API_URL`, `IPZILON_TOKEN`, `IPZILON_TEST_SITE_ID`, `IPZILON_TEST_ADDRESS_SPACE`, `IPZILON_TEST_ALT_SITE_ID` apuntando a un segundo site del mismo tipo): todos los `TestAcc*` existentes y nuevos en verde (SC-004, SC-005). Si hay instancia 3.1.x disponible, ejecutar también contra ella los `TestAcc*` existentes (deben seguir en verde; los nuevos de 3.2.0 fallarán con el error de versión, que es lo esperado); si no la hay, anotarlo y confiar en los unitarios con versión 3.1.0 (T015 h/i, T025 b). Anotar el resultado en este fichero → **2026-10-01**: imagen local `ipzilon-local:3.2.0` construida desde el tag `v3.2.0` (la de GHCR es privada), SQLite, dos sites `azure`: 16 `TestAcc*` `PASS` (14 existentes + `TestAccDataSources_GlobalLookup` y `TestAccHub_MoveSite`), 0 `FAIL`. Contra `ghcr.io/mdepedrof/ipzilon:3.1.1`: los 14 existentes `PASS`; los 2 nuevos fallan con `requires IPzilon >= 3.2.0 (server reports 3.1.1)`, como se espera (el del hub, ya en `plan`)
- [X] T036 Recorrer los escenarios manuales de `specs/004-global-filters-hub-move/quickstart.md` §2 (1–15), incluidos los de IPzilon 3.1.x (12 y 15) si hay instancia disponible, y el 11 (configuración real con ids de padre → mismos resultados y `plan` vacío); anotar resultados → **2026-10-01**, binario local con `dev_overrides`: contra 3.2.0 OK los escenarios 1–10, 13 y 14 (listas vacías `[]`/`0`; IP ocupada con `id = 5`, libre con `id = null` y `available`; dirección de fuera → `IP address not found`; `address` sin `subnet_id` / con `status` → error en `plan`; network, hub, scope y subred encontrados sin ids; `10.60.0.5/22` → `Invalid CIDR`; `root_only` sin hub → `Missing filter`; mover a site inexistente → `API error 404: Site not found` sin cambiar el hub; mover al site 2 → `update in-place`, mismo `id`, `plan` siguiente vacío). Contra 3.1.1 OK el 11 (búsqueda con `site_id`), el 12 y el 15 (`IPzilon version not supported` en `plan`). **Pendiente del usuario: escenario 11 en un proyecto real** (configuración existente con ids de padre → `plan` vacío tras actualizar a v3.2.0)
- [X] T037 Commit con el comando `commit-message` (confirmando el mensaje con `AskUserQuestion`, sin `Co-Authored-By`), PR a `main` con la plantilla completada y `Closes #16` / `Closes #21`; tras el merge, tag `v3.2.0` (confirmando antes con `AskUserQuestion`); después actualizar `main`, limpiar ramas locales y marcar la spec como cerrada → commits `098ab20` y `9bc1afa` (convergencia T038–T039), PR #22 mergeada (`e85d941`, CI en verde), issues #16 y #21 cerradas, tag `v3.2.0` publicado el 2026-10-01 (release GoReleaser con `darwin_arm64`, `linux_amd64`, `windows_amd64` y `SHA256SUMS` firmados); `main` actualizada y rama `004-global-filters-hub-move` eliminada. **Sigue pendiente el escenario 11 de T036** (`plan` de un proyecto real con ids de padre tras actualizar a v3.2.0)

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias.
- **Foundational (Phase 2)**: depende de Setup; **bloquea** todas las historias (US1 solo necesita
  T006 para sus tests; US4 solo T002).
- **US1 (Phase 3)**: depende de Foundational.
- **US2 (Phase 4)**: depende de Foundational (T006). Toca `ip_addresses.go`, igual que T009 de US1:
  hacer US1 antes o coordinar.
- **US3 (Phase 5)**: depende de Foundational (T002, T004–T006). Toca `hubs.go`, `scopes.go`,
  `networks.go` igual que T009: hacer US1 antes.
- **US4 (Phase 6)**: depende de T002; independiente de US1–US3 en código.
- **Polish (Phase 7)**: depende de todas las historias.

### Ficheros compartidos (no paralelizar entre sí)

- `internal/datasources/ip_addresses.go`: T009, T011, T012.
- `internal/datasources/{hubs,scopes,networks}.go`: T009 y T018/T019/T020 respectivamente.
- `internal/datasources/helpers.go`: T005, T017.
- `internal/datasources/network_zones_test.go`: T006.
- `internal/resources/acc_import_test.go`: T024, T030.
- `internal/resources/hub.go`: T027, T028.
- `internal/datasources/{sites,hubs,scopes,networks,subnets}.go`: T031 después de US1–US3.

### Within Each User Story

- Tests primero (deben fallar), luego implementación, ejemplos y aceptación.

## Parallel Example: User Story 3

```text
T014 helpers_test.go | T015 global_lists_test.go | T016 global_validation_test.go
-- después de T017 --
T018 hubs.go | T019 scopes.go | T020 networks.go | T021 subnets.go | T023 examples
```

## Parallel Example: User Story 4

```text
T026 models.go (HubUpdate.SiteID)  (en paralelo con)  T029 examples/resources/ipzilon_hub
```

## Parallel Example: entre historias (tras Phase 2 y US1)

```text
US2 (ip_addresses.go)  |  US3 (hubs/scopes/networks/subnets.go)  |  US4 (hub.go, models.go)
```

## Implementation Strategy

### MVP (US1 + US2)

1. Phase 1 + Phase 2.
2. US1: listas vacías → quickstart 1–2.
3. US2: IP por dirección → quickstart 3–6.
4. **STOP**: ambas funcionan contra cualquier IPzilon ≥ 3.0.0.

### Incremental Delivery

1. MVP (US1 + US2).
2. US3: búsquedas sin ids (IPzilon 3.2.0).
3. US4: mover hubs de site.
4. Polish y release única **v3.2.0** (todas las historias juntas; no se publican incrementos
   parciales salvo decisión del propietario).

## Phase 8: Convergence

- [X] T038 Documentar en `CONTRIBUTING.md` (bloque de variables de `make testacc`, junto a `IPZILON_TEST_SITE_ID`) la variable opcional `IPZILON_TEST_ALT_SITE_ID=<id of a second site of the same type>` y que sin ella `TestAccHub_MoveSite` se salta per plan: Testing (`IPZILON_TEST_ALT_SITE_ID`) (partial)
- [X] T039 Actualizar la `Description` de `network_id` en `internal/datasources/subnets.go` (p. ej. "Filter: subnets of this network. Optional with zone_id, name or cidr; required with no_zone.") y regenerar `docs/` con `make generate` per FR-023 (partial)
