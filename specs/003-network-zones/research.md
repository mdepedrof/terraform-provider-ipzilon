# Research: Zonas de red (IPzilon 3.1.0)

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Fecha**: 2026-10-01

Fuentes: `ipzilon/specs/003-network-zones/contract-changes.md` y `contracts/zones-api.md` (IPzilon
v3.1.0), lectura del backend publicado (`backend/app/routers/zones.py`, `networks.py`,
`schemas/network_zone.py`) y del provider en `main` (`9a0a10c`).

## Hallazgos

- La API 3.1.0 es aditiva: el provider v3.0.1 pasa los 56 tests de aceptación contra ella.
- `GET /zones/` admite `name`, `cidr` y `network_id`, todos opcionales y combinables; normaliza
  `name` y `cidr` a minúsculas antes de comparar. `GET /networks/{id}/zones` da lo mismo pero
  responde 404 si la Network no existe.
- `NetworkZoneResponse` trae métricas (`total_ips`, `used_ips`, `available_ips`,
  `alert_percent`, `alert_metric`, `subnet_count`). La constitución (Principio I) prohíbe exponer
  las métricas de UI y monitorización: no se exponen.
- `SubnetResponse.zone_id` siempre presente (`null` fuera de zona) y **calculado** por
  contención. `zone_id` en `SubnetCreate`/`SubnetUpdate` es una aserción que no se guarda.
- La validación del CIDR de zona es la misma que la de subred: IPv4, legible, no mayor que `/8`,
  `strict=False`.
- Todas las cadenas se normalizan a minúsculas en el servidor (`schemas/base.py`).
- El provider ya conoce la versión del servidor: `client.New` lee `/health` y guarda
  `Client.APIVersion`; `CheckAPIVersion` bloquea < 3.0.0 y no bloquea versiones desconocidas o
  no release (`0.0.0-dev`).
- Patrones existentes a reutilizar: `<x>FromAPI` único por recurso (spec 002), `last_subnet`
  comparte conversión con `next_subnet`, `prefixLengthValue`, `cidrLimits(true)`,
  `allocationErrorDiag`, `importByID`, `client.GetAll`, constructores de URL con filtros en
  `internal/datasources/helpers.go`, `TestReadAfterImportCoversAllResources` y
  `TestAccImport_*`.

## R1. Nombres de recursos y data sources

- **Decision**: `ipzilon_network_zone`, `ipzilon_next_network_zone`, `ipzilon_last_network_zone`
  y un único data source `ipzilon_network_zones`.
- **Rationale**: son los nombres propuestos por IPzilon (§0, §2, §3) y siguen la convención del
  provider (`next_`/`last_` + recurso; data sources en plural con filtro opcional por `id`, como
  `ipzilon_hubs`).
- **Alternatives considered**: `ipzilon_zone` (ambiguo con zonas de Azure/DNS); data sources
  singular + plural (rompe la convención del provider, que no tiene ningún data source singular).

## R2. `zone_id` en los recursos de subred

- **Decision**: `zone_id` Int64 `Optional + Computed` con `int64planmodifier.UseStateForUnknown()`
  y **sin** `RequiresReplace` en `ipzilon_subnet`, `ipzilon_next_subnet` e
  `ipzilon_last_subnet`. El estado siempre refleja el `zone_id` que devuelve la API (también tras
  import). Se envía `zone_id` en el cuerpo:
  - Create: solo si está configurado (como aserción en `subnet`; como espacio de búsqueda en
    `next_`/`last_`).
  - Update: solo si `zone_id` está **en la configuración** (`req.Config`, no el plan). Así la
    aserción acompaña a cualquier cambio de CIDR o de zona de una subred que declara zona. Un
    valor que solo viene del estado (por `UseStateForUnknown`) nunca se envía. Añadir a la
    configuración la zona que ya contiene la subred no produce diff (el estado ya tiene ese
    valor); declarar otra zona produce un update in situ cuyo PATCH falla con el 400 de IPzilon,
    porque la pertenencia es por contención y no se puede "mover" una subred de zona.
  - En `ipzilon_subnet` no basta `UseStateForUnknown`: si cambia `cidr`, la zona calculada puede
    cambiar en el mismo apply y Terraform fallaría con *Provider produced inconsistent result
    after apply*. Se usa un plan modifier propio, `zoneIDFollowsCIDR`, que copia el valor del
    estado solo si `cidr` no cambia y `zone_id` no está en la configuración; si `cidr` cambia,
    deja el valor desconocido. `next_`/`last_subnet` no lo necesitan: su CIDR solo cambia
    recreando el recurso.
- **Rationale**: es el modelado que recomienda IPzilon (§4, §5). `UseStateForUnknown` hace que,
  con `zone_id` sin configurar, el plan tome el valor refrescado: borrar, crear o redimensionar
  zonas en IPzilon nunca produce diff (FR-011, FR-012). Sin `RequiresReplace`, ningún cambio de
  zona recrea la subred (SC-003). Quitar `zone_id` de la configuración tampoco produce diff (el
  plan conserva el valor del estado).
- **Consecuencia aceptada**: si el usuario declara `zone_id = X` y la zona X desaparece fuera de
  Terraform, el refresco deja `zone_id = null`, el plan propone volver a `X` in situ y el PATCH
  falla con `Zone not found`. Es el comportamiento correcto ante drift: la configuración
  referencia algo que ya no existe. Si la zona la gestiona el mismo Terraform, el grafo de
  dependencias lo evita.
- **Alternatives considered**: `RequiresReplace` en `next_`/`last_` (IPzilon lo desaconseja: una
  subred importada o cuya zona se borra se recrearía y perdería sus IPs); `zone_id` solo
  `Computed` en `subnet` (impide declarar la aserción, pedida en FR-009); enviar siempre `zone_id`
  en el PATCH, o enviar el valor del plan (con `zone_id` sin configurar, el plan conserva el
  valor del estado y fallaría si esa zona se acaba de borrar).

## R3. Comprobación de versión (IPzilon ≥ 3.1.0 solo para zonas)

- **Decision**: nuevo método `Client.RequireAPIVersion(min, feature string) error` en
  `internal/client`, con la misma regla que `CheckAPIVersion`: versiones release menores que `min`
  devuelven un error que envuelve `ErrUnsupportedAPIVersion` y dice
  `"<feature> requires IPzilon >= 3.1.0 (server reports 3.0.1)"`; versiones desconocidas o no
  release pasan. Se llama, **sin peticiones extra** (la versión ya está en memoria):
  - En Create, Read, Update e ImportState de los tres recursos de zona.
  - En Create/Update de los recursos de subred **solo** si se envía `zone_id`.
  - En `ipzilon_network_zones` siempre, y en `ipzilon_subnets` solo si se usa `zone_id` o
    `no_zone`.
- **También en el plan**: además se comprueba en `ModifyPlan` de los tres recursos de zona y, en
  las subredes, cuando `zone_id` está en la configuración. Así el error aparece en
  `terraform plan` también para recursos nuevos (SC-007, quickstart escenario 6).
- **Defensa adicional para versiones desconocidas** (SC-007): tras un Create de subred con
  `zone_id`, si la respuesta no trae ese `zone_id` (un 3.0.x ignora el campo), el provider borra
  la subred que acaba de crear y falla con el mismo mensaje de versión. Así nunca queda una subred
  fuera de la zona pedida.
- **Rationale**: FR-022 y SC-007; las configuraciones sin zonas siguen funcionando contra 3.0.x.
  Read de zona contra 3.0.x devolvería 404 y retiraría el recurso en silencio: el chequeo previo
  lo convierte en un error explicativo.
- **Alternatives considered**: subir el mínimo global a 3.1.0 en `CheckAPIVersion` (rompe
  configuraciones sin zonas contra 3.0.x, contrario a FR-022); detectar por 404 en las rutas de
  zona (indistinguible de "zona borrada").

## R4. Recurso `ipzilon_network_zone`

- **Decision**:
  - `id` Computed + `UseStateForUnknown`; `network_id` Required + `RequiresReplace`; `name`
    Required, in situ; `cidr` Required, in situ, con `cidrLimits(true)`; `description`
    Optional + Computed.
  - Create `POST /networks/{network_id}/zones`; Read `GET /zones/{id}` (404 → `RemoveResource`);
    Update `PATCH /zones/{id}` con `name`, `cidr` y `description` (`null` vacía); Delete
    `DELETE /zones/{id}` (404 = ya borrada); Import por id.
  - Una única `networkZoneFromAPI(client.NetworkZone) networkZoneModel` usada en Create, Read y
    Update (patrón de la spec 002).
- **Rationale**: tabla §2 del contrato, Principios I y III.
- **Alternatives considered**: `cidr` con `RequiresReplace` (innecesario: la API redimensiona in
  situ y valida que ninguna subred quede fuera o a caballo).

## R5. Minúsculas en `name` de zona

- **Decision**: validador de forma en `name` de los tres recursos de zona que rechaza mayúsculas
  en `plan` (`stringvalidator.RegexMatches(^[^A-Z]*$)`), con el mismo texto de descripción que el
  resto ("must be lowercase — the server normalizes all strings").
- **Rationale**: IPzilon normaliza a minúsculas; sin validación, un nombre con mayúsculas genera un
  diff perpetuo (edge case de la spec). Es validación de forma (Principio I) y, al aplicarse solo a
  recursos nuevos, no rompe nada.
- **Alternatives considered**: solo documentarlo, como `hub`/`network`/`subnet` (deja el diff
  perpetuo); normalizar en el provider con un plan modifier (oculta lo que se guarda); extenderlo
  a los recursos existentes (fuera de alcance y potencialmente incompatible).

## R6. Recursos `ipzilon_next_network_zone` / `ipzilon_last_network_zone`

- **Decision**: dos recursos con el mismo modelo y la misma conversión (como
  `next_subnet`/`last_subnet`): `network_id` y `prefix_length` Required + `RequiresReplace`;
  `name` Required (la API lo exige) in situ; `description` Optional + Computed; `cidr` Computed +
  `UseStateForUnknown`. Create `POST /networks/{id}/{next|last}-available-zone`; Update
  `PATCH /zones/{id}` solo con `name` y `description`; `prefix_length` derivado con
  `prefixLengthValue` en Read e import. Errores 409 "Search truncated" con `allocationErrorDiag`
  sugiriendo `ipzilon_network_zone`.
- **Rationale**: Principio II (asignación atómica en el servidor, `cidr` estable, padre y tamaño
  fuerzan recreación) y §3 del contrato.
- **Alternatives considered**: un solo recurso con atributo `direction` (rompe la convención
  `next_`/`last_` y el patrón de `TestReadAfterImportCoversAllResources`).

## R7. Data source `ipzilon_network_zones`

- **Decision**: filtros opcionales `id`, `network_id`, `name`, `cidr`. Con `id` (excluyente con
  los demás) → `GET /zones/{id}`. Sin `id` → siempre el listado global
  `GET /zones/?network_id=&name=&cidr=` recorrido con `client.GetAll` (todas las páginas, orden de
  la API). Sin ningún filtro devuelve todas las zonas (documentado). `items` con `id`,
  `network_id`, `name`, `cidr` y `description`; sin métricas.
- **Rationale**: FR-017/FR-018, filtrado en el servidor (Principio I). Usar siempre la ruta global
  simplifica: una sola URL con tres filtros opcionales.
- **Alternatives considered**: `GET /networks/{id}/zones` cuando hay `network_id` (dos caminos
  para lo mismo; la única diferencia es el 404 de Network inexistente, que en un filtro es
  razonable tratar como lista vacía); exigir al menos un filtro (contrario a la idea de buscar sin
  ids; el número de zonas es pequeño).

## R8. `ipzilon_subnets`: filtros por zona y `zone_id` en los items

- **Decision**: nuevos filtros `zone_id` (Int64) y `no_zone` (Bool) e item `zone_id`. Reglas:
  - `id` excluyente con cualquier otro filtro.
  - `zone_id` y `no_zone` excluyentes (validador `ConflictsWith` en plan).
  - `zone_id` sin `network_id`: se resuelve la Network con `GET /zones/{id}` y se lista
    `GET /networks/{network_id}/subnets?zone_id=`.
  - `no_zone = true` exige `network_id` ("subredes fuera de zona" solo tiene sentido en una
    Network).
  - `network_id` + `zone_id` de otra Network: se envía tal cual y se muestra el 400 de IPzilon.
- **Rationale**: FR-019/FR-020; filtrado en el servidor; una petición extra solo cuando falta
  `network_id`.
- **Alternatives considered**: exigir siempre `network_id` (contrario a FR-019).

## R9. Errores de la API

- **Decision**: los recursos de zona y de subred muestran el `detail` de IPzilon tal cual
  (`APIError.Message` ya forma parte de `err.Error()`), con resúmenes del estilo existente
  (`"Create network zone failed"`). Los 409 "Search truncated" pasan por `allocationErrorDiag`;
  el resto de 409 (sin hueco, nombre repetido) se muestran tal cual.
- **Rationale**: FR-006; los mensajes de IPzilon ya nombran la zona o subred en conflicto.

## R10. Pruebas

- **Decision**:
  - Unitarias (sin API, `httptest`): conversiones `*FromAPI` de zona y `zone_id` en subredes;
    `RequireAPIVersion` (3.0.1 bloquea, 3.1.0/3.2.0/dev/"" pasan); constructores de URL de zonas
    y de subredes con `zone_id`/`no_zone`; selección de `zone_id` a enviar en Update de subred;
    rollback de Create con `zone_id` ignorado; añadir los tres recursos nuevos a la tabla de
    `TestReadAfterImport` (la comprobación de completitud lo exige).
  - Aceptación (`TF_ACC=1`, IPzilon 3.1.0): `TestAccImport_NetworkZone`, `_NextNetworkZone`,
    `_LastNetworkZone`; ciclo de subred automática en zona y fuera de zona; `plan` vacío tras
    añadir `zone_id` existente y tras borrar la zona.
- **Rationale**: Principio V.

## R11. Versión y documentación

- **Decision**: release MINOR **v3.1.0** del provider. Ejemplos nuevos en `examples/resources/`
  (`resource.tf` + `import.sh` para los tres recursos) y `examples/data-sources/ipzilon_network_zones/`;
  se amplían los ejemplos de `ipzilon_subnet`, `ipzilon_next_subnet` e `ipzilon_subnets`;
  `make generate` en el mismo commit. El pin `~> 3.0` de ejemplos y `docs/index.md` no cambia.
  README: sección de zonas con la nota de versión mínima de IPzilon para usarlas.
- **Rationale**: Principios III y IV; todos los cambios son aditivos (FR-023).
