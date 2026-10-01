---

description: "Tareas para soportar las zonas de red de IPzilon 3.1.0 (provider v3.1.0)"
---

# Tasks: Zonas de red (IPzilon 3.1.0)

**Input**: Documentos de diseño en `specs/003-network-zones/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: SÍ se incluyen. El Principio V exige tests unitarios sin API para la lógica pura
(conversiones, filtros, URLs, versión) y SHOULD de aceptación para recursos nuevos antes de la
release. Unitarios con `net/http/httptest` (patrón de `internal/resources/next_network_test.go`,
`newTestClient`); aceptación con `terraform-plugin-testing` (patrón de
`internal/resources/acc_import_test.go`, `testAccPreCheck`).

**Organization**: tareas agrupadas por historia de usuario (US1–US4 de spec.md).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: se puede ejecutar en paralelo (ficheros distintos, sin dependencias pendientes)
- **[Story]**: historia a la que pertenece (US1…US4)

## Path Conventions

Provider Go de proyecto único: `internal/client`, `internal/resources`, `internal/datasources`,
`internal/provider`, `examples/`, `docs/` (generado), `README.md`. Los tests de import por tabla
viven en el paquete externo `resources_test` (`read_after_import_test.go`).

---

## Phase 1: Setup

- [X] T001 Comprobar la línea base en la rama `003-network-zones` ejecutando `go build ./... && go vet ./... && go test ./...` desde la raíz; anotar cualquier fallo previo antes de tocar código
- [X] T002 Preguntar al usuario con `AskUserQuestion` si se crea el issue `[Feat] - Zonas de red (IPzilon 3.1.0)` (formato `[Type] - Summary` de `CLAUDE.md`, sin menciones de autoría de Claude), mostrando título y descripción literales en `preview`; si se crea, anotar su número para la PR → issue #19

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: modelos del cliente, comprobación de versión y helpers de zona que usan todas las historias.

**⚠️ CRITICAL**: ninguna historia puede empezar hasta completar esta fase.

- [X] T003 [P] Añadir en `internal/client/models.go` (sección `// NetworkZone`): `NetworkZone{ID int64 "id"; NetworkID int64 "network_id"; Name string "name"; CIDR string "cidr"; Description *string "description"}` **sin** mapear `total_ips`, `used_ips`, `available_ips`, `alert_percent`, `alert_metric`, `subnet_count`, `created_at`, `updated_at` (Principio I); `NetworkZoneCreate{Name string "name"; CIDR string "cidr"; Description *string "description,omitempty"}`; `NetworkZoneUpdate{Name *string "name,omitempty"; CIDR *string "cidr,omitempty"; Description *string "description"}` (sin `omitempty` en `description`: `null` vacía la descripción; comentar que la API rechaza `name`/`cidr` nulos con 422); `AllocateZoneBody{PrefixLength int64 "prefix_length"; Name string "name"; Description *string "description,omitempty"}`. En los modelos existentes añadir `ZoneID *int64` con JSON `zone_id` en `Subnet` (comentario: calculado por contención; `nil` fuera de zona o en IPzilon 3.0.x) y `zone_id,omitempty` en `SubnetCreate`, `SubnetUpdate` y `AllocateSubnetBody` (comentario: aserción en create/update, espacio de búsqueda en allocate) — data-model.md §Cliente
- [X] T004 [P] Añadir en `internal/client/client.go` la constante `MinZonesAPIVersion = "3.1.0"` y el método `(c *Client) RequireAPIVersion(min, feature string) error`: reutiliza `semverRe`; si `c.APIVersion` no es release `X.Y.Z` (vacía, `0.0.0-dev`…) devuelve `nil`; si es menor que `min` (comparación numérica major/minor/patch) devuelve `fmt.Errorf("%w: %s requires IPzilon >= %s (server reports %s)", ErrUnsupportedAPIVersion, feature, min, c.APIVersion)`; sin peticiones HTTP (research R3)
- [X] T005 Añadir en `internal/client/client_test.go` un test por tabla de `RequireAPIVersion("3.1.0", "network zones")`: `3.0.1` y `3.0.0` → error que envuelve `ErrUnsupportedAPIVersion` y contiene `requires IPzilon >= 3.1.0 (server reports 3.0.1)`; `3.1.0`, `3.1.5`, `3.2.0`, `4.0.0`, `""`, `0.0.0-dev` → `nil` (depende de T004)
- [X] T006 Crear `internal/resources/zones.go` con: (a) `zoneNameValidators = []validator.String{stringvalidator.RegexMatches(regexp.MustCompile("^[^A-Z]*$"), "must be lowercase — the server normalizes all strings")}` (research R5); (b) `requireZones(c *client.Client, feature string, diags *diag.Diagnostics) bool` que llama a `c.RequireAPIVersion(client.MinZonesAPIVersion, feature)` y, si falla, añade el error `"IPzilon version not supported"` con el mensaje y devuelve `false`; (c) `prefixLengthValidators = []validator.Int64{int64validator.Between(8, 32)}` para los recursos de zona automática
- [X] T007 [P] Crear `internal/resources/zones_test.go` con tests de `zoneNameValidators` (`pooled_zone_1` válido, `Pooled` inválido, null/unknown sin error) y de `requireZones` (cliente con `APIVersion: "3.0.1"` → `false` + diagnóstico; `"3.1.0"` y `""` → `true`) (depende de T004, T006)

**Checkpoint**: `go build ./... && go test ./internal/client/... ./internal/resources/...` en verde.

---

## Phase 3: User Story 1 - Gestionar zonas de red con un CIDR explícito (Priority: P1) 🎯 MVP

**Goal**: recurso `ipzilon_network_zone` con CRUD, import y errores de IPzilon visibles.

**Independent Test**: contra IPzilon 3.1.0, crear una zona en una Network, cambiar nombre, descripción y CIDR (in situ, mismo id), importarla en un estado vacío (`plan` vacío) y destruirla sin afectar a sus subredes (quickstart escenarios 1, 3, 4, 5, 10).

### Tests for User Story 1

- [X] T008 [P] [US1] Crear `internal/resources/network_zone_test.go` (paquete `resources`, `httptest` + `newTestClient`) con: Create envía `POST /networks/7/zones` con cuerpo `{name, cidr, description}` y el estado sale de la respuesta; Update envía `PATCH /zones/12` con `name`, `cidr` y `description` (y `"description": null` cuando el plan la tiene nula); Read con 404 retira el recurso del estado; Delete con 404 no da error; con `APIVersion: "3.0.1"` `ModifyPlan`, Create y Read fallan con el error de versión **sin** llegar a hacer ninguna petición (el servidor de test falla si recibe alguna); un 400 de la API (`Zone 10.0.20.0/24 is not within network 10.0.16.0/22`) aparece literal en el diagnóstico (FR-006)

### Implementation for User Story 1

- [X] T009 [US1] Crear `internal/resources/network_zone.go` con `NetworkZoneResource` (`_network_zone`), `networkZoneModel{ID, NetworkID, Name, CIDR, Description}` y una única `networkZoneFromAPI(client.NetworkZone) networkZoneModel` usada en Create, Read y Update (patrón spec 002). Esquema según contracts/terraform-schema.md: `id` Computed + `UseStateForUnknown`; `network_id` Required + `RequiresReplace` ("Network this zone belongs to."); `name` Required + `zoneNameValidators` ("Zone name, unique within its network (must be lowercase — the server normalizes all strings)."); `cidr` Required + `cidrLimits(true)` (descripción: IPv4, /8 or smaller, inside the network, smaller than it, no overlap with other zones; changing it is in place and IPzilon rejects it if a subnet would be left outside or straddle the boundary); `description` Optional + Computed. Descripción del recurso indicando que requiere IPzilon >= 3.1.0 y que destruir la zona no borra sus subredes. Create `POST /networks/{network_id}/zones` (`NetworkZoneCreate`), Read `GET /zones/{id}` (404 → `RemoveResource`), Update `PATCH /zones/{id}` (`NetworkZoneUpdate` con `Name`, `CIDR` y `Description: strPtr(plan.Description)`), Delete `DELETE /zones/{id}` (404 = OK), `ImportState` con `importByID`. Llamar a `requireZones(r.client, "ipzilon_network_zone", …)` al principio de Create, Read, Update y Delete, e implementar `ModifyPlan` que también lo llama (error ya en `plan`, research R3). Resúmenes de error: `"Create network zone failed"`, `"Read network zone failed"`, `"Update network zone failed"`, `"Delete network zone failed"` con `err.Error()` como detalle (research R4, R9)
- [X] T010 [US1] Registrar `resources.NewNetworkZoneResource` en `Resources()` de `internal/provider/provider.go`
- [X] T011 [US1] Añadir la fila `ipzilon_network_zone` a `importCases` en `internal/resources/read_after_import_test.go` (path `/zones/12`, cuerpo con las métricas de `NetworkZoneResponse` incluidas para comprobar que se ignoran, `want` con `network_id`, `name`, `cidr`, `description`) para que `TestReadAfterImport` y `TestReadAfterImportCoversAllResources` pasen (depende de T009, T010)
- [X] T012 [P] [US1] Crear `examples/resources/ipzilon_network_zone/resource.tf` (zona `pooled_zone_1`, `10.0.16.0/23`, con `network_id = ipzilon_network.example.id` y descripción) e `import.sh` (`terraform import ipzilon_network_zone.example 12`); sin credenciales (Principio IV)
- [X] T013 [US1] Añadir `TestAccImport_NetworkZone` en `internal/resources/acc_import_test.go` siguiendo el patrón de `TestAccImport_NextNetwork`: hub → scope → network → zona; paso de cambio de `name`/`description`/`cidr` comprobando con `plancheck.ExpectResourceAction(..., plancheck.ResourceActionUpdate)`; paso de import con `plan` vacío (depende de T009, T010)

**Checkpoint**: US1 funcional y testeable por separado (`go test ./...` en verde).

---

## Phase 4: User Story 2 - Asignar subredes dentro de una zona (Priority: P1)

**Goal**: `zone_id` en `ipzilon_subnet`, `ipzilon_next_subnet` e `ipzilon_last_subnet`, sin recrear nunca subredes y sin diff en configuraciones sin zonas.

**Independent Test**: con una Network con zonas, aplicar subredes automáticas con y sin zona y una subred explícita que declara su zona; comprobar dónde cae cada una, que `zone_id` está en el estado y que borrar la zona o importar la subred no la recrea (quickstart escenarios 1, 2, 5, 6, 7, 8, 13).

### Tests for User Story 2

- [X] T014 [P] [US2] Crear `internal/resources/subnet_zone_test.go` (paquete `resources`) con tests `httptest` para los tres recursos de subred: (a) Create con `zone_id` en la configuración envía `"zone_id": 12` y Create sin `zone_id` no envía la clave; (b) Update con `zone_id` solo en el estado (no en `req.Config`) **no** envía `zone_id`, y con `zone_id` en la configuración sí lo envía; (c) Create con `zone_id` cuya respuesta 201 trae `zone_id: null` provoca `DELETE /subnets/{id}` y error de versión (rollback, research R3), y en Update la misma situación da el error de versión **sin** borrar la subred; (d) con `APIVersion: "3.0.1"` y `zone_id` configurado falla antes de cualquier petición (también en `ModifyPlan`), y sin `zone_id` funciona igual que hoy; (e) `subnetFromAPI`/`nextSubnetFromAPI` rellenan `zone_id` (valor y `null`); (f) `zoneIDFollowsCIDR` en `ipzilon_subnet`: sin `zone_id` en la configuración y con cambio de `cidr` el plan deja `zone_id` desconocido; sin cambio de `cidr` conserva el valor del estado

### Implementation for User Story 2

- [X] T015 [US2] Añadir a `internal/resources/zones.go`: (a) `zoneIDAttribute(desc string) schema.Int64Attribute` → `Optional + Computed`, `PlanModifiers: int64planmodifier.UseStateForUnknown()`, **sin** `RequiresReplace`; (b) `configZoneID(ctx, cfg tfsdk.Config, diags) *int64` que lee `zone_id` de la **configuración** (`cfg.GetAttribute(ctx, path.Root("zone_id"), &v)`) y devuelve `nil` si es nulo o desconocido (research R2: nunca se envía un valor que solo viene del estado); (c) `checkZoneApplied(ctx, c *client.Client, requested *int64, got client.Subnet, rollback bool, diags) bool` que, si `requested != nil` y `got.ZoneID` es `nil` o distinto, hace `DELETE /subnets/{got.ID}` (ignorando 404) solo si `rollback` es `true` (Create) y en todo caso añade el error `"<feature> requires IPzilon >= 3.1.0: the server ignored zone_id"`; (d) `zoneIDFollowsCIDR() planmodifier.Int64`: si `zone_id` no está en la configuración y el `cidr` del plan es igual al del estado, usa el valor del estado; si el `cidr` cambia, deja `zone_id` desconocido (research R2)
- [X] T016 [US2] Modificar `internal/resources/subnet.go`: `zone_id` en `subnetModel` y esquema (`zoneIDAttribute` con `zoneIDFollowsCIDR()` en lugar de `UseStateForUnknown`; descripción `"Zone that contains this subnet. When set, IPzilon checks on create and update that the CIDR is inside this zone (IPzilon >= 3.1.0). Always reflects the zone computed by IPzilon (null if none); changing zones never replaces the subnet."`); `subnetFromAPI` rellena `ZoneID: types.Int64PointerValue(s.ZoneID)`; Create envía `ZoneID: configZoneID(req.Config)` y, si no es nil, antes `requireZones` y después `checkZoneApplied(..., rollback=true)`; Update envía `ZoneID: configZoneID(req.Config)` (si no es nil, `requireZones` antes y `checkZoneApplied(..., rollback=false)` después) manteniendo `Force: true` (Principio II); implementar `ModifyPlan` llamando a `requireZones` si `zone_id` está en la configuración (depende de T015)
- [X] T017 [US2] Modificar `internal/resources/next_subnet.go`: `zone_id` en `nextSubnetModel`, esquema (`zoneIDAttribute("Zone to allocate the block from (IPzilon >= 3.1.0). Without it, in a network that has zones IPzilon allocates outside every zone. Always reflects the zone computed by IPzilon; changing it never replaces the subnet.")`) y `nextSubnetFromAPI`; Create envía `AllocateSubnetBody.ZoneID` desde `configZoneID` con `requireZones`/`checkZoneApplied`; Update envía `SubnetUpdate.ZoneID` desde `configZoneID`; implementar `ModifyPlan` llamando a `requireZones` si `zone_id` está en la configuración. `allocationErrorDiag` sin cambios para 409 truncado (depende de T015)
- [X] T018 [US2] Modificar `internal/resources/last_subnet.go` igual que T017, incluido `ModifyPlan` (misma descripción cambiando "allocate the block from" por "allocate the last free block from"); sigue compartiendo `nextSubnetFromAPI` (depende de T017)
- [X] T019 [US2] Actualizar las filas `ipzilon_subnet`, `ipzilon_next_subnet` e `ipzilon_last_subnet` de `importCases` en `internal/resources/read_after_import_test.go`: cuerpo con `"zone_id": 12` y `want` con `zone_id: int64(12)`; añadir un subtest `zone_id_null` con `"zone_id": null` que compruebe `zone_id` nulo; verificar que `TestStateMatchesAcrossOperations` sigue en verde (depende de T016–T018)
- [X] T020 [P] [US2] Ampliar `examples/resources/ipzilon_next_subnet/resource.tf`, `examples/resources/ipzilon_last_subnet/resource.tf` y `examples/resources/ipzilon_subnet/resource.tf` con un segundo recurso que use `zone_id = ipzilon_network_zone.example.id` (comentando que requiere IPzilon >= 3.1.0), manteniendo el ejemplo sin zona
- [X] T021 [US2] Añadir en `internal/resources/acc_import_test.go` `TestAccSubnetInZone`: network + zona + `ipzilon_next_subnet` con `zone_id` (comprobar con `statecheck`/`resource.TestCheckResourceAttrPair` que `zone_id` = id de la zona y que el `cidr` está dentro del de la zona) + `ipzilon_next_subnet` sin `zone_id` (`zone_id` nulo, CIDR fuera de la zona); paso que quita `zone_id` de `ipzilon_next_subnet.in_zone` y elimina la zona en la misma configuración, comprobando `plancheck.ExpectResourceAction("ipzilon_next_subnet.in_zone", plancheck.ResourceActionNoop)` y, tras el apply, que el siguiente refresh deja `zone_id` nulo sin cambios en el `plan`; paso de import de la subred en zona con `plan` vacío (depende de T009, T017)

**Checkpoint**: US1 + US2 forman el MVP: crear zonas y asignar subredes en ellas. `make testacc` previo sin zonas sigue en verde (SC-004).

---

## Phase 5: User Story 3 - Consultar zonas y subredes por zona sin conocer ids (Priority: P2)

**Goal**: data source `ipzilon_network_zones` sin ids y filtros `zone_id`/`no_zone` en `ipzilon_subnets`.

**Independent Test**: con varias Networks y zonas (dos con el mismo nombre), leer `ipzilon_network_zones` por `cidr`, por `name`, por `name` + `network_id` y por `id`, e `ipzilon_subnets` por `zone_id` y por `network_id` + `no_zone` (quickstart escenarios 11 y 12).

### Tests for User Story 3

- [X] T022 [P] [US3] Añadir en `internal/datasources/helpers_test.go` tests por tabla de `zonesURL` (sin filtros → `/zones/`; `network_id`, `name`, `cidr` sueltos y combinados, con escape de `/` en el CIDR) y `networkSubnetsURL` (sin filtro → `/networks/7/subnets`; `zone_id=12`; `no_zone=true`)
- [X] T023 [P] [US3] Crear `internal/datasources/network_zones_test.go` e `internal/datasources/subnets_test.go` (`httptest`): zonas por `id` → `GET /zones/12`; por `cidr` → `GET /zones/?cidr=…` con dos páginas (`total` > tamaño de página) devolviendo todas en orden; `id` + `name` → error de filtros excluyentes; lista vacía sin error; subredes con solo `zone_id` → `GET /zones/12` y después `GET /networks/7/subnets?zone_id=12`; `network_id` + `zone_id` → sin la petición a `/zones/12`; `no_zone` sin `network_id` → error `no_zone requires network_id`; `zone_id` + `no_zone` → error; items con `zone_id`; `APIVersion: "3.0.1"` + `zone_id` → error de versión sin peticiones

### Implementation for User Story 3

- [X] T024 [US3] Añadir en `internal/datasources/helpers.go` `zonesURL(networkID *int64, name, cidr *string) string` (`GET /zones/` con `network_id`, `name`, `cidr` opcionales vía `url.Values`) y `networkSubnetsURL(networkID int64, zoneID *int64, noZone bool) string` (`/networks/{id}/subnets` con `zone_id` o `no_zone=true`), con comentario de doc al estilo de `hubNetworksURL`
- [X] T025 [US3] Crear `internal/datasources/network_zones.go` (`_network_zones`): argumentos `id`, `network_id` (Int64, Optional), `name`, `cidr` (String, Optional, "exact match (server-side)"); `items` `ListNestedAttribute` Computed con `id`, `network_id`, `name`, `cidr`, `description` (todos con `Description`; **sin** métricas, Principio I). `ConfigValidators` (`datasourcevalidator.Conflicting` o validación en Read) para que `id` sea excluyente con los demás. Read: comprobar la versión con `d.client.RequireAPIVersion(client.MinZonesAPIVersion, "ipzilon_network_zones")` y, si falla, añadir el error `"IPzilon version not supported"`; con `id` → `GET /zones/{id}`; si no → `client.GetAll[client.NetworkZone](ctx, d.client, zonesURL(...))`. Descripción del data source: busca zonas por nombre y/o CIDR sin conocer ids, el nombre solo es único por Network (puede devolver varias), sin filtros devuelve todas, requiere IPzilon >= 3.1.0 (research R7) (depende de T024)
- [X] T026 [US3] Modificar `internal/datasources/subnets.go`: argumentos `zone_id` (Int64, "List subnets inside this zone (IPzilon >= 3.1.0). network_id is not required.") y `no_zone` (Bool, "When true, list only the subnets of network_id outside every zone (IPzilon >= 3.1.0)."); `zone_id` en `subnetItem`, `subnetItemSchema` ("Zone that contains the subnet (null if none).") y `subnetToItem`; reglas (research R8): `id` excluyente con todo; `zone_id` y `no_zone` excluyentes; `no_zone` exige `network_id`; al menos uno de `id`, `network_id`, `zone_id`; con `zone_id` o `no_zone` comprobar versión; con `zone_id` sin `network_id` resolver la Network con `GET /zones/{zone_id}`; listar con `client.GetAll[client.Subnet](ctx, d.client, networkSubnetsURL(...))`. Actualizar la descripción del data source (`Provide id, network_id or zone_id.`) sin cambiar el comportamiento de `id`/`network_id` (FR-021) (depende de T024)
- [X] T027 [US3] Registrar `datasources.NewNetworkZonesDataSource` en `DataSources()` de `internal/provider/provider.go` (depende de T025)
- [X] T028 [P] [US3] Crear `examples/data-sources/ipzilon_network_zones/data-source.tf` (por `cidr` sin ids; por `name` + `network_id`; salida con `items[0].id`) y ampliar `examples/data-sources/ipzilon_subnets/data-source.tf` con `zone_id` y con `network_id` + `no_zone = true`

**Checkpoint**: US3 funcional; `go test ./internal/datasources/...` en verde.

---

## Phase 6: User Story 4 - Reservar zonas automáticamente por tamaño (Priority: P3)

**Goal**: `ipzilon_next_network_zone` e `ipzilon_last_network_zone` con reserva atómica en IPzilon.

**Independent Test**: con una Network con zonas y subredes, aplicar una zona automática `/24` desde el principio y otra desde el final; el `cidr` no cambia en `plan` sucesivos; cambiar `prefix_length` recrea; importar deja `plan` vacío (quickstart escenarios 9 y 10).

### Tests for User Story 4

- [X] T029 [P] [US4] Crear `internal/resources/alloc_network_zone_test.go` (paquete `resources`, `httptest`): Create de `next` envía `POST /networks/7/next-available-zone` y el de `last` `POST /networks/7/last-available-zone` con `{prefix_length, name, description}`; `prefix_length` se deriva del CIDR devuelto; Update envía `PATCH /zones/{id}` solo con `name` y `description` (sin `cidr`); un 409 `Search truncated after 4096 steps …` produce `Address space too fragmented` sugiriendo `ipzilon_network_zone`; un 409 `No free /24 block available for a zone in …` aparece literal; versión `3.0.1` → error en `ModifyPlan` y en Create sin peticiones

### Implementation for User Story 4

- [X] T030 [US4] Crear `internal/resources/next_network_zone.go` con `NextNetworkZoneResource` (`_next_network_zone`), `allocZoneModel{ID, NetworkID, PrefixLength, Name, Description, CIDR}`, `allocZoneFromAPI(client.NetworkZone) (allocZoneModel, error)` (usa `prefixLengthValue`) y `setAllocZoneState`. Esquema (contracts/terraform-schema.md): `id` Computed + `UseStateForUnknown`; `network_id` Required + `RequiresReplace`; `prefix_length` Required + `RequiresReplace` + `prefixLengthValidators` ("8..32"); `name` Required + `zoneNameValidators`; `description` Optional + Computed; `cidr` Computed + `UseStateForUnknown` ("Assigned CIDR block (computed by server). The block overlaps no zone and no subnet of the network."). Create `POST /networks/{id}/next-available-zone` (`AllocateZoneBody`) con `allocationErrorDiag(err, prefixLength, "ipzilon_network_zone")`; Read `GET /zones/{id}` (404 → `RemoveResource`); Update `PATCH /zones/{id}` con `NetworkZoneUpdate{Name, Description}` (sin `CIDR`); Delete `DELETE /zones/{id}` (404 = OK); `importByID`; `requireZones` en `ModifyPlan`, Create, Read, Update y Delete (research R3, R6) (depende de T006)
- [X] T031 [US4] Crear `internal/resources/last_network_zone.go` con `LastNetworkZoneResource` (`_last_network_zone`) igual que T030 pero con `last-available-zone`, compartiendo `allocZoneModel`/`allocZoneFromAPI` (patrón `last_subnet.go`) (depende de T030)
- [X] T032 [US4] Registrar `resources.NewNextNetworkZoneResource` y `resources.NewLastNetworkZoneResource` en `internal/provider/provider.go` y añadir sus filas a `importCases` en `internal/resources/read_after_import_test.go` (`want` con `prefix_length` derivado) (depende de T030, T031)
- [X] T033 [P] [US4] Crear `examples/resources/ipzilon_next_network_zone/{resource.tf,import.sh}` y `examples/resources/ipzilon_last_network_zone/{resource.tf,import.sh}` (`prefix_length = 25`, `name = "personal_zone"`)
- [X] T034 [US4] Añadir `TestAccImport_NextNetworkZone` y `TestAccImport_LastNetworkZone` en `internal/resources/acc_import_test.go` (patrón `TestAccImport_NextNetwork`; comprobar que `cidr` no cambia en un segundo `plan` y que el import deja `plan` vacío) (depende de T032)

**Checkpoint**: todas las historias funcionales por separado.

---

## Phase 7: Polish & Cross-Cutting Concerns

- [X] T035 Regenerar la documentación con `make generate` y revisar que `docs/resources/network_zone.md`, `next_network_zone.md`, `last_network_zone.md`, `docs/data-sources/network_zones.md` y las páginas de subredes incluyen `zone_id` con su descripción; commitear `docs/` junto con el cambio de esquema (Principio IV)
- [X] T036 [P] Actualizar `README.md`: añadir los tres recursos y el data source a las listas de *Resources* y *Data Sources*; nueva fila en *Compatibility* (provider `~> 3.0` v3.1.0: IPzilon ≥ 3.0.0, zonas ≥ 3.1.0); sección breve *Network zones* con un ejemplo de zona + `ipzilon_next_subnet` con `zone_id`, la semántica de `zone_id` (calculado, nunca recrea, sin zona en una Network con zonas se asigna fuera de ellas) y que destruir una zona no borra sus subredes; mantener el pin `~> 3.0` en ejemplos y `docs/index.md`
- [X] T037 Ejecutar `go build ./... && go vet ./... && go test ./...` y `go list -deps . | grep terraform-plugin-sdk` (debe salir vacío); corregir cualquier fallo
- [X] T038 Ejecutar `TF_ACC=1 make testacc` contra IPzilon 3.1.0 (`IPZILON_API_URL`, `IPZILON_TOKEN`, `IPZILON_TEST_SITE_ID`, `IPZILON_TEST_ADDRESS_SPACE`): todos los `TestAccImport_*` existentes y nuevos en verde (SC-002, SC-004); anotar el resultado en este fichero → **2026-10-01**: verde contra `ghcr.io/mdepedrof/ipzilon:3.1.0` local (SQLite): 14 `TestAcc*` `PASS` (9 imports existentes + `TestAccImport_NetworkZone`, `TestAccNetworkZone_UpdateInPlace`, `TestAccSubnetInZone`, `TestAccImport_NextNetworkZone`, `TestAccImport_LastNetworkZone`), 0 `FAIL`
- [X] T039 Recorrer los escenarios manuales de `specs/003-network-zones/quickstart.md` §2 (1–14), incluidos el 6 (IPzilon 3.0.x → error de versión, sin subredes creadas; SC-007) y el 13 (configuración real sin zonas → `plan` vacío; FR-013); anotar resultados → **2026-10-01**, binario local con `dev_overrides`: contra IPzilon 3.1.0 OK los escenarios 1, 2, 3, 5, 7, 8, 11, 12 y 14 (los 4, 9 y 10 los cubren los `TestAcc*`); contra la imagen local `ipzilon-local:3.0.0` (reporta `0.0.0-dev`, versión desconocida) una configuración sin zonas aplica y deja `plan` vacío, y con `zone_id` la subred se revierte y no queda fuera de la zona (escenario 6 en su variante de versión desconocida; el bloqueo en `plan` por versión 3.0.x lo cubren los unitarios). Hallazgos corregidos durante la prueba: `ipzilon_subnets` devolvía `items = null` sin resultados (ahora lista vacía, test `TestSubnetsEmptyIsAnEmptyList`) y los recursos de zona contra un servidor sin rutas de zona daban `404 Not Found` / retiraban la zona del estado (ahora error de versión, `TestNetworkZoneUnknownVersionWithoutZoneRoutes`). **Pendiente del usuario: escenario 13** (`plan` de un proyecto real sin zonas tras actualizar a v3.1.0)
- [ ] T040 Commit con el comando `commit-message` (confirmando el mensaje con `AskUserQuestion`, sin `Co-Authored-By`), PR a `main` con la plantilla completada y, tras el merge, tag `v3.1.0` (confirmando antes con `AskUserQuestion`); después actualizar `main`, limpiar ramas locales y marcar la spec como cerrada

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (Phase 1)**: sin dependencias.
- **Foundational (Phase 2)**: depende de Setup; **bloquea** todas las historias.
- **US1 (Phase 3)**: depende de Foundational.
- **US2 (Phase 4)**: depende de Foundational. Su código no depende de US1; el test de aceptación T021 sí necesita `ipzilon_network_zone` (T009).
- **US3 (Phase 5)**: depende de Foundational; independiente de US1/US2 en código.
- **US4 (Phase 6)**: depende de Foundational (T006); independiente del resto en código.
- **Polish (Phase 7)**: depende de todas las historias que se vayan a publicar.

### Ficheros compartidos (no paralelizar entre sí)

- `internal/provider/provider.go`: T010, T027, T032.
- `internal/resources/read_after_import_test.go`: T011, T019, T032.
- `internal/resources/acc_import_test.go`: T013, T021, T034.
- `internal/resources/zones.go`: T006, T015.

### Within Each User Story

- Tests primero (deben fallar), luego implementación, registro en el provider, fila de import, ejemplos y aceptación.

## Parallel Example: User Story 1

```text
T008 [US1] network_zone_test.go          (en paralelo con)
T012 [US1] examples/resources/ipzilon_network_zone/*
```

## Parallel Example: User Story 2

```text
T014 [US2] subnet_zone_test.go           (en paralelo con)
T020 [US2] ejemplos de subredes con zone_id
```

## Parallel Example: User Story 3

```text
T022 [US3] helpers_test.go               (en paralelo con)
T023 [US3] network_zones_test.go / subnets_test.go
T028 [US3] ejemplos de data sources
```

## Parallel Example: entre historias (tras Phase 2)

```text
US1 (network_zone.go)  |  US3 (datasources/*)  |  US4 (next/last_network_zone.go)
```

## Implementation Strategy

### MVP (US1 + US2)

1. Phase 1 + Phase 2.
2. US1: zonas con CIDR explícito → validar con quickstart 1, 3, 4, 5, 10.
3. US2: subredes en zonas → validar con quickstart 1, 2, 5, 7, 8, 13.
4. **STOP**: el MVP ya permite el caso real (subredes de host pools agrupadas en zonas).

### Incremental Delivery

1. MVP (US1 + US2).
2. US3: consultas sin ids.
3. US4: zonas automáticas.
4. Polish y release única **v3.1.0** (todas las historias juntas; no se publican incrementos
   parciales salvo decisión del propietario).
