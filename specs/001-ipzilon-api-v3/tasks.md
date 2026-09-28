---

description: "Tareas para adaptar terraform-provider-ipzilon a IPzilon 3.0.0"
---

# Tasks: Adaptación del provider a IPzilon 3.0.0

**Input**: Documentos de diseño en `specs/001-ipzilon-api-v3/`

**Prerequisites**: [plan.md](./plan.md), [spec.md](./spec.md), [research.md](./research.md),
[data-model.md](./data-model.md), [contracts/](./contracts/), [quickstart.md](./quickstart.md)

**Tests**: SÍ se incluyen. Los exige FR-019 y el Principio V de la constitución (lógica pura
con tests unitarios sin API en vivo). Patrón: `net/http/httptest` como en
`internal/resources/next_network_test.go`.

**Organization**: tareas agrupadas por historia de usuario (US1–US5 de spec.md).

## Format: `[ID] [P?] [Story] Description`

- **[P]**: se puede ejecutar en paralelo (ficheros distintos, sin dependencias pendientes)
- **[Story]**: historia a la que pertenece (US1…US5)

## Path Conventions

Provider Go de proyecto único: `internal/client`, `internal/datasources`,
`internal/resources`, `internal/provider`, `examples/`, `docs/` (generado), `README.md`.

---

## Phase 1: Setup

**Purpose**: dejar el entorno listo; no hay que inicializar nada nuevo.

- [X] T001 Comprobar que la línea base compila y pasa tests en la rama `001-ipzilon-api-v3` ejecutando `go build ./... && go vet ./... && go test ./...` desde la raíz del repositorio; anotar cualquier fallo previo antes de tocar código

---

## Phase 2: Foundational (Blocking Prerequisites)

**Purpose**: cambios en `internal/client` que necesitan todas las historias (R1, R2, R3, R5, R6, R7).

**⚠️ CRITICAL**: ninguna historia puede empezar hasta completar esta fase.

### Tests (escribir primero; deben fallar)

- [X] T002 [P] Crear `internal/client/client_test.go` con tests de reintentos usando `httptest` y un `sleep` falso inyectado: (a) 429 con `Retry-After: 2` y luego 200 → 2 intentos y espera registrada de 2 s; (b) 503 sin `Retry-After` → esperas 1 s, 2 s (±20 % jitter) hasta éxito; (c) 429 persistente con `Retry-After: 60` → se detiene al superar 10 min de espera acumulada (10 reintentos) y el error contiene `(gave up after 10 retries in 10m0s)`; (c2) 429 persistente con `Retry-After: 1` → se detiene en 30 reintentos (`retryMax`); (d) `Retry-After: 3600` → espera acotada a 60 s; (e) 400, 404, 409 y 500 → 1 solo intento; (f) `POST` reintentado reenvía el mismo cuerpo JSON; (g) contexto cancelado durante la espera → devuelve `context.Canceled`; (h) cuerpo 429 `{"error": "Rate limit exceeded: x"}` → mensaje `Rate limit exceeded: x`
- [X] T003 [P] Añadir a `internal/client/client_test.go` tests de `GetAll`: (a) 2 páginas (`total` 1500, 1000 + 500 elementos) → 1500 en orden y peticiones con `limit=1000&offset=0` y `offset=1000`; (b) `path` con query previa `?name=x` → se concatena con `&`; (c) `{"items": [], "total": 0}` → lista vacía sin error; (d) página vacía con `total` > 0 → termina sin bucle; (e) servidor que siempre devuelve 1 elemento con `total` fijo e `items` repetidos más allá de `ceil(total/1000)+1` páginas → error; (f) cuerpo que es una lista JSON (`[...]`) → error que envuelve `ErrUnsupportedAPIVersion` y menciona `>= 3.0.0`
- [X] T004 [P] Añadir a `internal/client/client_test.go` tests de detección de versión y helpers de error: sonda `/health` con `{"status":"ok","version":"2.3.1"}` → `Client.APIVersion == "2.3.1"` y `CheckAPIVersion()` devuelve `ErrUnsupportedAPIVersion`; `3.0.0` y `3.1.2` → nil; `0.0.0-dev`, cadena vacía y sonda fallida → nil (no bloquea); `IsConflict` true solo para `APIError{Code: 409}`; `IsSearchTruncated` true para 409 con mensaje que empieza por `Search truncated after 4096 steps` y false para 409 `No free /24 block available in 10.0.0.0/16`

### Implementación

- [X] T005 Modificar `internal/client/models.go`: añadir `type Page[T any] struct { Items []T \`json:"items"\`; Total int \`json:"total"\` }`; cambiar `IPAddress.ID` de `int64` a `*int64` ("`nil` para direcciones libres sin fila"); añadir `IPAddressRegister{Address string \`json:"address"\`; Status string \`json:"status"\`; Hostname *string \`json:"hostname,omitempty"\`; Description *string \`json:"description,omitempty"\`}` con comentario "cuerpo de POST /subnets/{id}/ips; Status siempre enviado (la API usa available por defecto)"
- [X] T006 Modificar `internal/client/client.go` — contexto y reintentos (R3, R6): `do(ctx context.Context, method, path string, body, out any)` serializa el cuerpo una vez y crea un `http.NewRequestWithContext` por intento; reintenta solo 429 y 503 mientras la espera acumulada no supere `retryMaxElapsed = 10m` y con un máximo de `retryMax = 30` reintentos; espera = `Retry-After` en segundos o fecha HTTP si existe, si no `retryBaseWait (1s) * 2^intento` con jitter ±20 %; cada espera acotada a `retryMaxWait = 60s`; espera vía campo inyectable `sleep func(ctx, time.Duration) error` que respeta la cancelación; registra cada reintento con `tflog.Debug(ctx, "retrying IPzilon request", ...)` con método, ruta, código, intento y espera (nunca cabeceras ni token); al agotar devuelve `&APIError{Code, Message + fmt.Sprintf(" (gave up after %d retries in %s)", n, elapsed)}`; al leer el error usa `detail`, si no `error`, si no el cuerpo crudo. Cambiar las firmas públicas a `Get(ctx, path, out)`, `Post(ctx, path, body, out)`, `Patch(ctx, path, body, out)`, `Delete(ctx, path)`. Inicializar los valores por defecto en `New`
- [X] T007 Modificar `internal/client/client.go` — paginación (R1): función libre `func GetAll[T any](ctx context.Context, c *Client, path string) ([]T, error)` que pide `limit=1000&offset=N` (separador `?` o `&` según `path`), inspecciona el primer byte no blanco del cuerpo y, si es `[`, devuelve `fmt.Errorf("%w: ...", ErrUnsupportedAPIVersion)`; acumula `Items` hasta `len(Items)==0 || offset >= Total`; corta con error si se superan `ceil(Total/1000)+1` páginas. Para inspeccionar el cuerpo crudo, `do` debe aceptar `out *json.RawMessage` sin cambios
- [X] T008 Modificar `internal/client/client.go` — versión y helpers (R2, R7): `resolveAPIBase` devuelve también la versión leída de `{"version": ...}` de `/health`; guardarla en el campo exportado `Client.APIVersion`; añadir `var ErrUnsupportedAPIVersion = errors.New("unsupported IPzilon version")`, `func (c *Client) CheckAPIVersion() error` (error solo si `APIVersion` es semver `X.Y.Z` válido con `X < 3`; mensaje: `IPzilon <v> is not supported by this provider version: requires IPzilon >= 3.0.0. Pin the provider to ~> 2.2 to keep using IPzilon 2.x.`), `func IsConflict(err error) bool` y `func IsSearchTruncated(err error) bool` (409 + `strings.HasPrefix(msg, "Search truncated")`); `IsNotFound` pasa a usar `errors.As`
- [X] T009 Adaptar todos los llamadores al nuevo `ctx` (cambio mecánico, en un commit aislado): pasar el `ctx` de cada método CRUD/Read a `Get/Post/Patch/Delete` en `internal/resources/*.go` e `internal/datasources/*.go`, y a las llamadas `c.Post(...)` de `internal/resources/next_network_test.go` (usar `context.Background()`). Temporalmente, donde se use `ip.ID` (int64 → *int64) en `internal/resources/ip_address.go`, `internal/resources/next_ip_address.go` e `internal/datasources/ip_addresses.go`, compilar con `types.Int64PointerValue(ip.ID)`; se refina en US1/US2
- [X] T010 Modificar `internal/provider/provider.go` `Configure`: tras `client.New(apiURL, token)` llamar a `c.CheckAPIVersion()` y, si falla, `resp.Diagnostics.AddError("Unsupported IPzilon version", err.Error())` y `return`
- [X] T011 Ejecutar `go build ./... && go vet ./... && go test ./internal/client/...` y confirmar que T002–T004 pasan

**Checkpoint**: cliente preparado para 3.0 (reintentos, paginación, versión); US3 queda implementada funcionalmente en esta fase.

---

## Phase 3: User Story 1 - Data sources de listado (Priority: P1) 🎯 MVP

**Goal**: los 6 data sources de listado devuelven todos los elementos paginados (FR-001–FR-004, FR-007).

**Independent Test**: quickstart V1 (subred /22 → 1024 elementos; libres con `id = null`).

### Tests

- [X] T012 [P] [US1] Crear `internal/datasources/ip_addresses_test.go` con test de `ipToItem`: `client.IPAddress{ID: nil, Status: "available"}` → `items.id` es `types.Int64Null()`; con `ID` = puntero a 7 → `types.Int64Value(7)`

### Implementación

- [X] T013 [P] [US1] `internal/datasources/sites.go:94-95`: sustituir `var sites []client.Site` + `d.client.Get(...)` por `sites, err := client.GetAll[client.Site](ctx, d.client, sitesURL(stringFilter(cfg.Name)))`, conservando el diagnóstico `List sites failed`
- [X] T014 [P] [US1] `internal/datasources/hubs.go:103-107`: usar `client.GetAll[client.Hub](ctx, d.client, reqURL)` conservando `siteHubsURL` y el diagnóstico `List hubs failed`
- [X] T015 [P] [US1] `internal/datasources/scopes.go:126-136`: usar `client.GetAll[client.Scope](ctx, d.client, reqURL)` conservando `hubScopesURL` (filtros `parent_id`, `root_only`, `cidr`, `name`) y `List scopes failed`
- [X] T016 [P] [US1] `internal/datasources/networks.go:120-135`: usar `client.GetAll[client.Network]` en las dos ramas (`hubNetworksURL` y `scopeNetworksURL`), conservando `List networks failed`
- [X] T017 [P] [US1] `internal/datasources/subnets.go:96-98`: usar `client.GetAll[client.Subnet](ctx, d.client, fmt.Sprintf("/networks/%d/subnets", ...))`, conservando `List subnets failed`
- [X] T018 [US1] `internal/datasources/ip_addresses.go`: (a) listado con `client.GetAll[client.IPAddress]`; construir la query con `url.Values` (`status`) en lugar de concatenar; (b) `ipToItem` usa `types.Int64PointerValue(ip.ID)`; (c) en el esquema, `items.id` con `Description: "IP record ID. Null for free addresses that have no stored record (IPzilon >= 3.0)."`; (d) `status` con `Description` que advierta: "Without status the whole subnet is listed (e.g. 65,536 items for a /16, fetched in pages of 1000); set status to limit cost."; (e) `Description` del data source con la misma advertencia; (f) la consulta por `id` sigue con `GET /ips/{id}`; (g) `items.subnet_id` con `Description: "Subnet containing this address."` (Principio IV: todo atributo con `Description`); (h) en la consulta por `id`, si `client.IsNotFound(err)` → `AddError("IP not found", fmt.Sprintf("IP %d not found. In IPzilon >= 3.0 released addresses have no record and a re-occupied address gets a new id; look it up by subnet_id instead.", id))`; el resto de errores mantiene `Get IP failed` (caso límite de spec.md)
- [X] T019 [US1] Ejecutar `go build ./... && go test ./internal/datasources/...`

**Checkpoint**: US1 funcional; validar con quickstart V1 contra IPzilon 3.0.0.

---

## Phase 4: User Story 2 - `ipzilon_ip_address` (Priority: P1)

**Goal**: crear, modificar, liberar e importar una IP concreta con 3.0 (FR-005, FR-006, FR-008, FR-009).

**Independent Test**: quickstart V2.

### Tests

- [X] T020 [P] [US2] Crear `internal/resources/ip_address_test.go` que, con `httptest`, pruebe la función auxiliar de alta `registerIP(ctx, c, subnetID, body) (client.IPAddress, diag.Diagnostics)` (se implementa en T023): (a) 201 → devuelve la IP y el servidor recibe `POST /subnets/5/ips` con `{"address":"10.0.1.17","status":"used","hostname":"web"}` (sin `description` si es nulo); (b) 409 `IP 10.0.1.17 is already in use in this subnet` → diagnóstico con título `IP address in use` y detalle `Address 10.0.1.17 is already in use in subnet 5.`; (c) 400 `IP 10.9.9.9 is not within subnet 10.0.1.0/24` → `IP address outside subnet` / `Address 10.9.9.9 is not within subnet 5.`; (d) 400 con otro mensaje → `Create IP failed` con el mensaje de la API; (e) 404 → `Subnet not found` / `Subnet 5 does not exist.`; (f) 201 con `"id": null` → error `Unexpected response: IP address without id`; (g) la función de baja `releaseIP(ctx, c, id)` (T024) → el servidor recibe `DELETE /ips/7` y un 404 no produce error, un 403 sí; (h) el esquema de `ipzilon_ip_address` y el de `ipzilon_next_ip_address` rechazan `status = "available"` y aceptan `used` y `reserved` (validar con `validator.StringRequest` sobre los validadores del atributo)
- [X] T021 [P] [US2] Añadir a `internal/resources/helpers_test.go` test de `ipIDValue`: puntero no nulo → `types.Int64Value`, nil → error

### Implementación

- [X] T022 [US2] `internal/resources/helpers.go`: añadir `func ipIDValue(id *int64) (types.Int64, error)` que devuelve error `Unexpected response: IP address without id` si `id == nil`
- [X] T023 [US2] `internal/resources/ip_address.go` `Create` (líneas 99-139): eliminar la búsqueda `GET …/ips?address=` y el `PATCH`; extraer `registerIP` que hace `r.client.Post(ctx, fmt.Sprintf("/subnets/%d/ips", subnetID), client.IPAddressRegister{Address, Status: "used" o el de plan, Hostname: strPtr(plan.Hostname), Description: strPtr(plan.Description)}, &ip)` y traduce errores según la tabla de `contracts/api-client.md` (`client.IsConflict` → 409; 400 con `is not within subnet` → fuera de rango; `client.IsNotFound` → subred inexistente); estado desde la respuesta
- [X] T024 [US2] `internal/resources/ip_address.go`: `ipFromAPI` devuelve `(ipAddressModel, error)` usando `ipIDValue`; `Read` y `Update` añaden el error a diagnósticos; `Delete` pasa de `PATCH` a `available` a `releaseIP(ctx, c, id)` = `c.Delete(ctx, fmt.Sprintf("/ips/%d", id))` ignorando `client.IsNotFound` (FR-008; en IPzilon 3.0 `DELETE` libera la dirección); definir `releaseIP` en `internal/resources/helpers.go` para reutilizarla en T025; comentario de `Delete`: "releases the address: IPzilon 3.0 deletes the stored record and the address becomes free again"; `status` con `Validators: []validator.String{stringvalidator.OneOf("used", "reserved")}` y `Description: "IP status: used (default) or reserved. To free the address, destroy the resource."` (FR-020); `id` con `Description: "IPzilon record ID of the occupied address."` (Principio IV); añadir `stringvalidator` de formato IP al atributo `address` (validación de forma, Principio I) y cambiar su `Description` a `"IP address to occupy (e.g. 10.0.1.5). Must be inside the subnet and not already in use."`; `Description` del recurso: `"Occupies a specific IP address in a subnet. Destroy releases it."`
- [X] T025 [US2] `internal/resources/next_ip_address.go`: tras `reserve-ip`, obtener `id` con `ipIDValue(ip.ID)`; en el `PATCH` de metadatos usar `*ip.ID`; `Read`/`Update` usan `ipIDValue`; `Delete` usa `releaseIP` (T024) en lugar del `PATCH` a `available`; `status` con `stringvalidator.OneOf("used", "reserved")` y la misma `Description` que en T024 (FR-020). Con esa validación el `PATCH` nunca deja la IP libre, así que no hace falta tratar un `id: null` en la respuesta
- [X] T026 [US2] Ejecutar `go build ./... && go test ./internal/resources/...`

**Checkpoint**: US1 + US2 completas → provider funcional contra 3.0 (SC-001).

---

## Phase 5: User Story 3 - Resiliencia ante límite y saturación (Priority: P2)

**Goal**: ejecuciones grandes terminan pese a 429/503 (FR-010–FR-012). La lógica se implementó en T006 (fundacional porque vive en `do()`).

**Independent Test**: quickstart V3 (`RATE_LIMIT_DEFAULT=60/minute`, ≥ 200 recursos, `-parallelism=10`).

- [X] T027 [US3] Revisar que ningún recurso ni data source envuelve o reintenta por su cuenta y que los diagnósticos de error muestran el sufijo `(gave up after N retries in <duración>)` sin alterarlo: `grep -rn "APIError\|Retry" internal/resources internal/datasources`
- [X] T028 [US3] Documentar el comportamiento de reintentos en la `Description` del esquema del provider en `internal/provider/provider.go` (p. ej. en el atributo `token` o en la descripción general: "Requests rejected with 429/503 are retried honouring Retry-After for up to 10 minutes per request; each API token has its own rate-limit quota")

**Checkpoint**: US3 verificable de forma independiente con V3.

---

## Phase 6: User Story 4 - Búsqueda de bloque truncada (Priority: P3)

**Goal**: mensaje específico cuando la búsqueda de bloque libre se trunca (FR-013).

**Independent Test**: quickstart V4 o test unitario.

### Tests

- [X] T029 [P] [US4] Crear `internal/resources/allocation_errors_test.go` para `allocationErrorDiag(err, prefixLength, explicitResource)`: 409 `Search truncated after 4096 steps without finding a free /28 block in 10.0.0.0/12: …` → título `Address space too fragmented`, detalle que contiene `/28`, `ipzilon_subnet` y el mensaje de la API; 409 `No free /24 block available in 10.0.0.0/16` y cualquier otro error → `ok == false` (el llamador mantiene su diagnóstico actual)

### Implementación

- [X] T030 [US4] Crear `internal/resources/allocation_errors.go` con `func allocationErrorDiag(err error, prefixLength int64, explicitResource string) (summary, detail string, ok bool)` basado en `client.IsSearchTruncated`; detalle: `IPzilon stopped searching for a free /<N> block because the address space is too fragmented. Declare an explicit CIDR with <explicitResource> instead. API: <msg>`
- [X] T031 [P] [US4] Usar `allocationErrorDiag` en el `Create` de `internal/resources/next_subnet.go` (línea ~101, recurso explícito `ipzilon_subnet`) antes de `Reserve next subnet failed`
- [X] T032 [P] [US4] Usar `allocationErrorDiag` en el `Create` de `internal/resources/last_subnet.go` (línea ~101, `ipzilon_subnet`) antes de `Reserve last subnet failed`
- [X] T033 [P] [US4] Usar `allocationErrorDiag` en el `Create` de `internal/resources/next_network.go` (línea ~101, `ipzilon_network`) antes de `Reserve next network failed`

**Checkpoint**: US4 completa.

---

## Phase 7: User Story 5 - Validación /8 e IPv4 en `plan` (Priority: P3)

**Goal**: `plan` falla con CIDR > /8 o subred IPv6, solo al crear o cambiar el valor (FR-014, R8).

**Independent Test**: quickstart V5 (sin API).

### Tests

- [X] T034 [P] [US5] Crear `internal/resources/cidr_validation_test.go` para `checkCIDRChange(prior, planned types.String, ipv4Only bool) (summary, detail string, ok bool)`: `10.0.0.0/7` nuevo → error `/7 is too large: the maximum size is /8`; `10.0.0.0/8` → ok; `fd00::/64` con `ipv4Only` → error `IPv6 subnets are not supported`; `fd00::/64` sin `ipv4Only` → ok; prior `10.0.0.0/7` == planned `10.0.0.0/7` → ok (objeto heredado); planned desconocido o nulo → ok; CIDR no parseable → ok (se deja a la API)

### Implementación

- [X] T035 [US5] Crear `internal/resources/cidr_validation.go` con `checkCIDRChange` (usa `net/netip.ParsePrefix`; solo valida si `planned` es conocido, no nulo y distinto de `prior`) y un plan modifier `cidrLimits(ipv4Only bool) planmodifier.String` que obtiene el valor previo de `req.StateValue` y añade el diagnóstico con `resp.Diagnostics.AddAttributeError(req.Path, ...)`
- [X] T036 [P] [US5] Añadir `cidrLimits(false)` a `PlanModifiers` de `address_space` en `internal/resources/hub.go`
- [X] T037 [P] [US5] Añadir `cidrLimits(false)` a `PlanModifiers` de `cidr` en `internal/resources/scope.go` (junto a `UseStateForUnknown`)
- [X] T038 [P] [US5] Añadir `cidrLimits(false)` a `cidr` en `internal/resources/network.go` (esquema actual, línea ~55; no tocar el esquema auxiliar de la línea ~186)
- [X] T039 [P] [US5] Añadir `cidrLimits(true)` a `cidr` en `internal/resources/subnet.go`; actualizar su `Description` a `"Subnet CIDR (IPv4, /8 or smaller). Changing it keeps the stored addresses that remain inside the new range; stored addresses left outside are released (the provider always sends force: true)."` y la `Description` del recurso quitando "auto-populates all IP records" (FR-015)
- [X] T040 [US5] Ejecutar `go build ./... && go test ./internal/resources/...`

**Checkpoint**: todas las historias completas.

---

## Phase 8: Polish & Cross-Cutting Concerns

**Purpose**: documentación, versión y validación final (FR-016–FR-018, Principios III y IV).

- [X] T041 [P] Actualizar `examples/provider/provider.tf` a `version = "~> 3.0"`
- [X] T042 [P] Actualizar `examples/data-sources/ipzilon_ip_addresses/data-source.tf`: añadir `status = "available"` al ejemplo por subred con un comentario sobre el coste de omitirlo y que las libres tienen `id = null`
- [X] T043 [P] Revisar `examples/resources/ipzilon_ip_address/resource.tf` e `import.sh`: el comentario debe decir que se ocupa una dirección libre y que solo se importan direcciones ocupadas, reservadas o anotadas
- [X] T044 Actualizar `README.md`: (a) nueva sección `## Compatibility` con la tabla provider `~> 2.2` ↔ IPzilon 2.x (≤ 2.3.1) y provider `~> 3.0` ↔ IPzilon ≥ 3.0.0; (b) nueva sección `## Breaking Changes (v3.0.0)` antes de la de v2.0.0: fin de soporte de IPzilon 2.x, error `Unsupported IPzilon version`, `items[*].id` nulo en `ipzilon_ip_addresses`, `status` de `ipzilon_ip_address`/`ipzilon_next_ip_address` solo admite `used` o `reserved` (quien tenga `available` debe cambiarlo a `reserved` o destruir el recurso antes de actualizar), `destroy` libera la dirección con `DELETE`, validaciones /8 e IPv6, reintentos 429/503; pasos de migración: fijar la imagen de IPzilon a `2.3.1` (no `:latest`) y el provider a `~> 2.2` hasta migrar, migrar IPzilon a 3.0.0 y luego subir el provider a `~> 3.0` (el estado no requiere cambios salvo el caso de `status = "available"`); (c) versión de los ejemplos de la sección `## Usage` a `~> 3.0`
- [ ] T045 Ejecutar `make generate` y comprobar que `docs/index.md` muestra `~> 3.0` y que `docs/data-sources/ip_addresses.md`, `docs/resources/ip_address.md` y `docs/resources/subnet.md` reflejan las nuevas descripciones; no editar `docs/` a mano (Principio IV)
- [ ] T046 Ejecutar `make fmt`, `go build ./...`, `go vet ./...` y `go test ./...` (quickstart V0); todo en verde
- [ ] T047 Ejecutar la validación manual de `quickstart.md` V1–V7 contra IPzilon 3.0.0 (y 2.3.1 para V6/V7) y anotar el resultado en la descripción de la PR
- [ ] T048 Medir SC-005 durante el `apply` de quickstart V3 (o de `examples/terraform/` contra IPzilon 3.0.0 sin límite bajo): contar las peticiones con `TF_LOG=DEBUG terraform apply 2>&1 | grep -c "Sending HTTP Request"` o en los logs de acceso de IPzilon, restar los reintentos (`grep -c "retrying IPzilon request"`) y dividir por el número de recursos gestionados; el resultado debe ser ≤ 2 de media. Si no hay línea de log por petición, añadir en `do()` de `internal/client/client.go` un `tflog.Debug(ctx, "IPzilon request", map[string]any{"method": method, "path": path})` sin cabeceras ni token. Anotar el resultado en la descripción de la PR
- [X] T049 Preparar la respuesta al equipo de IPzilon (§9.4 del informe): confirmación de SC-011, decisiones (`status` opcional en `ipzilon_ip_addresses`; `force: true` se mantiene; baja con `DELETE /ips/{id}`; `status` de los recursos de dirección limitado a `used`/`reserved`) y fecha estimada de `v3.0.0`
- [ ] T050 Publicar la release `v3.0.0` **después** de mergear la PR y **coordinada con el equipo de IPzilon** (no antes de que IPzilon 3.0.0 esté disponible o tenga fecha cerrada): actualizar `main` y limpiar ramas locales; crear y subir el tag (`git tag v3.0.0 && git push origin v3.0.0`); comprobar que el workflow `.github/workflows/release.yml` (GoReleaser) termina en verde, que la release de GitHub tiene los binarios firmados con GPG y `SHA256SUMS`, y que el Terraform Registry muestra `3.0.0`; en `README.md` la sección `## Releasing` puede seguir usando `v2.0.0` como ejemplo o actualizarse a `v3.0.0`

---

## Dependencies & Execution Order

### Phase Dependencies

- **Setup (T001)** → **Foundational (T002–T011)** → bloquea todas las historias.
- **US1 (T012–T019)** y **US2 (T020–T026)**: tras Foundational; independientes entre sí (ficheros distintos).
- **US3 (T027–T028)**: tras Foundational (su lógica está en T006).
- **US4 (T029–T033)** y **US5 (T034–T040)**: tras Foundational; independientes entre sí y de US1/US2.
- **Polish (T041–T050)**: tras las historias que se vayan a publicar; T045 después de todos los cambios de esquema (T018, T024, T039); T048 durante o después de T047; T050 al final, tras mergear la PR y con la confirmación del equipo de IPzilon (T049).

### Dentro de Foundational

- T002–T004 (tests, mismo fichero: secuenciales entre sí, en paralelo con T005).
- T005 → T006 → T007 → T008 (mismo fichero `client.go` salvo T005) → T009 → T010 → T011.

### User Story Dependencies

- US1: ninguna otra historia.
- US2: ninguna otra historia (T025 toca `next_ip_address.go`, solo US2).
- US3: ninguna.
- US4: ninguna.
- US5: ninguna.

---

## Parallel Example: User Story 1

```bash
# Tras Foundational, los 5 data sources simples en paralelo:
Task: "T013 GetAll en internal/datasources/sites.go"
Task: "T014 GetAll en internal/datasources/hubs.go"
Task: "T015 GetAll en internal/datasources/scopes.go"
Task: "T016 GetAll en internal/datasources/networks.go"
Task: "T017 GetAll en internal/datasources/subnets.go"
# y en paralelo el test T012 de ip_addresses
```

## Parallel Example: User Story 5

```bash
# Tras T035:
Task: "T036 cidrLimits en internal/resources/hub.go"
Task: "T037 cidrLimits en internal/resources/scope.go"
Task: "T038 cidrLimits en internal/resources/network.go"
Task: "T039 cidrLimits en internal/resources/subnet.go"
```

---

## Implementation Strategy

### MVP (lo mínimo para desbloquear IPzilon 3.0.0)

1. Phase 1 + Phase 2 (incluye reintentos: US3 queda cubierta).
2. Phase 3 (US1) y Phase 4 (US2): con ambas, el 100 % de recursos y data sources funcionan contra 3.0 (SC-001).
3. **Parar y validar** con quickstart V1, V2, V3, V6, V7.

### Entrega incremental

1. MVP (US1 + US2 + US3) → primera PR revisable.
2. US4 + US5 → mejoras de experiencia (P3), pueden ir en la misma release.
3. Polish → release `v3.0.0` coordinada con IPzilon 3.0.0.

### Commits sugeridos

1. T005–T008 cliente + tests. 2. T009–T010 `ctx` mecánico + versión. 3. US1. 4. US2.
5. US3 docs. 6. US4. 7. US5. 8. Docs/ejemplos/README + `make generate`.
Cada commit con el comando `commit-message` y confirmación previa del mensaje.

---

## Notes

- [P] = ficheros distintos, sin dependencias pendientes.
- Los tests deben fallar antes de implementar (T002–T004, T012, T020–T021, T029, T034).
- No introducir dependencias nuevas (R3) ni `terraform-plugin-sdk/v2`.
- `force: true` en `SubnetUpdate` se mantiene (Principio II).
