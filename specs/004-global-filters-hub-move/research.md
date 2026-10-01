# Research: Filtros sin ids, lista vacía y mover hubs de site (IPzilon 3.2.0)

**Fecha**: 2026-10-01 | **Spec**: [spec.md](./spec.md)

IPzilon **3.2.0** se publicó el 2026-10-01 con las specs 005 (listados globales filtrables,
mdepedrof/ipzilon#202) y 006 (mover un hub de site, mdepedrof/ipzilon#203). Todas las decisiones
siguientes parten de esa versión. No quedan puntos `NEEDS CLARIFICATION`.

---

## R1. Lista vacía en vez de `null`

**Decisión**: inicializar `items` como slice vacío (`[]T{}`) en `ipzilon_sites`, `ipzilon_hubs`,
`ipzilon_scopes`, `ipzilon_networks` e `ipzilon_ip_addresses`, como ya hacen `ipzilon_subnets` e
`ipzilon_network_zones`. En `ipzilon_scopes` el filtro de `kind` en cliente (`items[:0]`) parte ya
de un slice no nulo cuando la inicialización es `[]scopeItem{}`.

**Razón**: el framework convierte un slice `nil` en lista nula y uno vacío en `[]`. `GetAll` ya
devuelve `[]T{}`; el `null` nace de `var items []T` en el data source.

**Alternativas**: normalizar en un helper genérico al hacer `State.Set` → innecesario para cinco
líneas.

## R2. `address` en `ipzilon_ip_addresses`

**Decisión**:

- Atributo nuevo `address` (Optional, String). Ruta: `GET /subnets/{subnet_id}/ips?address=X`
  vía `GetAll` (ya existe en IPzilon ≥ 3.0.0, no exige 3.2.0).
- `subnetIPsURL` pasa a recibir también `address` (filtros `status` y `address` excluyentes).
- Validación de forma en `ValidateConfig` (R5): `address` es una IP válida (`netip.ParseAddr`,
  IPv4 o IPv6), exige `subnet_id` y es incompatible con `id` y `status`.
- Si la API devuelve 0 elementos, `Read` falla con
  `Address <X> is not an address of subnet <N>` (dirección fuera de la subred o mal formada, §8).
- Con resultado: un elemento; si está libre, `id = null` y `status = "available"` (ya lo modela
  `ipToItem` con `Int64PointerValue`).

**Razón**: la dirección es la referencia estable de una IP desde IPzilon 3.0.0; la API ya ofrece
el filtro y devuelve lista vacía en los casos de error, que el issue pide convertir en error.

**Alternativas**: `GET /ips/?address=` global → no existe en la API; obligaría a recorrer todas
las subredes.

## R3. Elección de ruta: listado por padre vs. listado global

**Decisión**: se usa el **listado por padre actual** siempre que la configuración indique el id
del padre que hoy es obligatorio y no use filtros nuevos que ese listado no admita; en cualquier
otro caso, el **listado global** (requiere 3.2.0):

| Data source | Listado por padre (sin cambios, 3.1.x OK) | Listado global (3.2.0) |
|---|---|---|
| `ipzilon_hubs` | `site_id` → `GET /sites/{id}/hubs` | sin `site_id` → `GET /hubs/` |
| `ipzilon_scopes` | `hub_id` → `GET /hubs/{id}/scopes` (+ `kind` en cliente, como hoy) | sin `hub_id` → `GET /scopes/` (`kind`, `parent_id` en servidor) |
| `ipzilon_networks` | `hub_id` → `GET /hubs/{id}/networks`; `scope_id` → `GET /scopes/{id}/networks` | sin `hub_id` ni `scope_id` → `GET /networks/` |
| `ipzilon_subnets` | sin `name`/`cidr`: comportamiento actual (`network_id`, `zone_id`, `no_zone`) | con `name` o `cidr` → `GET /subnets/` (con `network_id`/`zone_id` opcionales) |

- `id` sigue siendo búsqueda singular y gana sobre los filtros, como hoy (no se añade un error
  nuevo para no romper configuraciones existentes).
- `hub_id` + `scope_id` a la vez en networks sigue siendo error, como hoy.
- Sin ningún filtro en hubs/scopes/networks: lista el inventario completo mediante el listado
  global (antes era error "Missing filter"; pasa a ser válido, igual que `ipzilon_sites` e
  `ipzilon_network_zones`). En `ipzilon_subnets` sin ningún filtro se mantiene el error actual
  (listar todas las subredes no aporta y puede ser muy grande).
- `ipzilon_subnets` con solo `zone_id` mantiene la resolución actual de la network (compatible con
  3.1.x); no se cambia a `GET /subnets/?zone_id=` para no exigir 3.2.0 a configuraciones que hoy
  funcionan.

**Razón**: FR-016 exige resultados idénticos y compatibilidad con 3.1.x para los filtros
existentes. Además, los listados por padre comparan el CIDR por texto y el global por red: usar
el global con id de padre cambiaría resultados en objetos guardados con bits de host.

**Alternativas**:
- Usar siempre el listado global → rompe 3.1.x y cambia la semántica del CIDR.
- Elegir ruta según la versión del servidor → dos comportamientos para la misma configuración;
  más difícil de razonar y probar.

## R4. Versión mínima y `405`

**Decisión**:

- Nueva constante `client.MinGlobalListsAPIVersion = "3.2.0"` y
  `client.MinHubMoveAPIVersion = "3.2.0"` (dos constantes con el mismo valor para que cada
  mensaje nombre su función y se puedan mover por separado si hiciera falta).
- Antes de un listado global: `RequireAPIVersion(MinGlobalListsAPIVersion, "<data source> without <padre>")`
  (sin petición extra; la versión se leyó de `/health`).
- Para versiones desconocidas/dev (no bloqueadas): si el listado global responde `405 Method Not
  Allowed`, se traduce a `IPzilon version not supported: <función> requires IPzilon >= 3.2.0 (the
  server has no global listing: <error>)`. Helper `globalListError` en
  `internal/datasources/helpers.go` y predicado `client.IsMethodNotAllowed`.

**Razón**: §7 recomienda exigir la versión solo cuando se usan las búsquedas sin id y traducir el
`405`. Es el mismo patrón que `requireZones` + `zoneAPIError` de la spec 003.

## R5. Validación en `plan` de los data sources

**Decisión**: implementar `datasource.DataSourceWithValidateConfig` en `ipzilon_hubs`,
`ipzilon_scopes`, `ipzilon_networks`, `ipzilon_subnets` e `ipzilon_ip_addresses` para las reglas
**nuevas**:

- `ipzilon_ip_addresses`: `address` formato IP; `address` exige `subnet_id`; `address` con `id`
  o `status` → error.
- Búsquedas globales (padre **nulo**, no desconocido): `cidr`/`address_space` deben ser un CIDR
  válido **sin bits de host**; el error sugiere la red (`did you mean '10.0.16.0/22'?`). Helper
  puro `networkCIDR(s) (canonical string, err)` con `netip.ParsePrefix` + `Masked()`.
- `ipzilon_scopes`: `root_only = true` sin `hub_id` → error; `kind` ya tiene `OneOf`.
- `ipzilon_subnets`: `name`/`cidr` con `no_zone` → error (el listado global no tiene ese
  filtro); `no_zone` sin `network_id` ya es error.
- Valores desconocidos: se omite la regla que dependa de ellos (la API vuelve a validar).

Las comprobaciones actuales en `Read` (conflictos de id/padre) se mantienen tal cual.

**Razón**: Principio I (validación de forma en `plan`) y SC-006. `ValidateConfig` corre en
`validate`/`plan` sin llamar a la API. La validación de bits de host se limita a búsquedas
globales para no cambiar configuraciones existentes con id de padre (Assumptions de la spec).

**Alternativas**: validators de atributo (`stringvalidator`) → no pueden condicionar la regla a
que el padre sea nulo; se usarían igual para el formato básico, pero un único `ValidateConfig`
mantiene las reglas juntas y testeables.

## R6. Varios resultados

**Decisión**: los data sources devuelven todas las coincidencias, sin error con `total > 1`.

**Razón**: son data sources de listado (convención del provider y de `ipzilon_network_zones`);
la recomendación de §5 aplica a data sources de un solo elemento. Los ejemplos muestran `one()`
para el caso "espero exactamente uno".

## R7. Mover un hub de site

**Decisión**:

- `ipzilon_hub.site_id`: se quita `int64planmodifier.RequiresReplace()`.
- `client.HubUpdate` gana `SiteID *int64 \`json:"site_id,omitempty"\``. `Update` lo envía **solo
  si cambia** respecto al estado (el resto del `PATCH` queda idéntico a hoy).
- `ModifyPlan` (nuevo en `HubResource`): si hay estado, el `site_id` planificado es conocido y
  distinto del del estado, exige `MinHubMoveAPIVersion` → el error aparece ya en `plan`.
- `Update`: repite la comprobación de versión y, tras el `PATCH`, si `h.SiteID` ≠ pedido, añade
  `IPzilon version not supported: moving ipzilon_hub to another site requires IPzilon >= 3.2.0:
  the server ignored site_id` y guarda en el estado la respuesta real (site antiguo), sin diff
  perpetuo silencioso (006 §4).
- Errores `404`/`409` del `PATCH`: se muestran tal cual (`Update hub failed: API error 409: …`),
  como ya hace el recurso.

**Razón**: 006 §3–§4 e issue #21. Comprobar en `plan` evita un `apply` a medias; la comprobación
posterior cubre versiones desconocidas/dev, igual que `checkZoneApplied` en la spec 003.

**Alternativas**:
- `RequiresReplaceIf(versión < 3.2.0)` → mantendría la recreación contra 3.1.x, pero recrear un
  hub con hijos falla en la práctica y la spec (US4-4) pide error de versión.
- Enviar siempre `site_id` → inocuo según 006 §3, pero cambia el cuerpo de todas las
  actualizaciones; enviarlo solo al cambiar minimiza el impacto.

## R8. Compatibilidad de esquema y versión del provider

**Decisión**: release MINOR **v3.2.0** del provider. Cambios: atributos opcionales nuevos
(`address` en IPs; `name`, `cidr` en subnets), padres que pasan de "obligatorio de facto" a
opcional, `items` vacío en vez de nulo y quitar `RequiresReplace` de `site_id`. Ninguno rompe
configuraciones ni estado; no hace falta `UpgradeState`.

**Razón**: Principio III. Quitar `RequiresReplace` solo convierte en in situ lo que antes era
recreación; sin cambios de estado.

## R9. Pruebas

**Decisión**:

- Unitarios (`internal/datasources`, `httptest` vía `fakeAPI`/`readDataSource` existentes):
  lista vacía en los cinco data sources; URLs globales y por padre; elección de ruta (R3);
  versión 3.1.0 → error sin petición; `405` → error de versión; `address` (ocupada, libre, vacía
  → error); `ValidateConfig` de cada regla; helper `networkCIDR`.
- Unitarios (`internal/resources`): `ModifyPlan` del hub (versión), `Update` con `site_id`
  ignorado → error, `HubUpdate` serializa `site_id` solo si se mueve; `site_id` sin
  `RequiresReplace` en el esquema.
- Aceptación (`TF_ACC=1`, IPzilon 3.2.0): `TestAccHub_MoveSite` (requiere una variable nueva
  `IPZILON_TEST_ALT_SITE_ID`, se salta si no está; sites no son gestionables por el provider) y
  `TestAccDataSources_GlobalLookup` (crea hub/scope/network/subnet y los busca sin ids).

## R10. Documentación

**Decisión**: actualizar `Description` de los atributos afectados, ejemplos de
`ipzilon_hubs`, `ipzilon_scopes`, `ipzilon_networks`, `ipzilon_subnets`, `ipzilon_ip_addresses`
(búsqueda sin ids, `address`, `one()`) y `ipzilon_hub` (comentario sobre mover de site);
`make generate`; README: tabla *Compatibility* (`global lookups and hub moves: >= 3.2.0, provider
>= 3.2.0`) y sección breve "Lookups without ids". Pin de ejemplos `~> 3.0` sin cambios.
