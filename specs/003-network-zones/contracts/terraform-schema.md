# Contrato: interfaz Terraform expuesta a los usuarios (provider v3.1.0)

Todos los cambios son **aditivos**: ningún recurso, atributo o comportamiento existente se
renombra, elimina o pasa a obligatorio. Las configuraciones sin zonas no ven cambios en `plan`.

## Compatibilidad

| Provider | IPzilon soportado |
|---|---|
| `~> 3.0` (v3.1.0) sin zonas | ≥ 3.0.0 |
| `~> 3.0` (v3.1.0) con zonas | ≥ 3.1.0 (si no: error `... requires IPzilon >= 3.1.0`) |

## `resource "ipzilon_network_zone"` (nuevo)

| Atributo | Tipo | Modo | Modificadores / validación |
|---|---|---|---|
| `id` | number | Computed | `UseStateForUnknown` |
| `network_id` | number | Required | `RequiresReplace` |
| `name` | string | Required | in situ; sin mayúsculas |
| `cidr` | string | Required | in situ; IPv4, ≤ /8 al crear o cambiar |
| `description` | string | Optional + Computed | in situ |

```hcl
resource "ipzilon_network_zone" "pooled" {
  network_id  = ipzilon_network.avd.id
  name        = "pooled_zone_1"
  cidr        = "10.0.16.0/23"
  description = "subredes /28 de host pools pooled"
}
```

- `destroy` no borra las subredes contenidas.
- `import`: `terraform import ipzilon_network_zone.pooled <id>`.

## `resource "ipzilon_next_network_zone"` / `"ipzilon_last_network_zone"` (nuevos)

| Atributo | Tipo | Modo | Modificadores / validación |
|---|---|---|---|
| `id` | number | Computed | `UseStateForUnknown` |
| `network_id` | number | Required | `RequiresReplace` |
| `prefix_length` | number | Required | `RequiresReplace`; 8..32 |
| `name` | string | Required | in situ; sin mayúsculas |
| `description` | string | Optional + Computed | in situ |
| `cidr` | string | Computed | `UseStateForUnknown` (asignado por IPzilon) |

```hcl
resource "ipzilon_next_network_zone" "personal" {
  network_id    = ipzilon_network.avd.id
  prefix_length = 25
  name          = "personal_zone"
}
```

El bloque asignado no solapa ninguna zona ni subred de la Network. `import` por id deriva
`prefix_length` del CIDR.

## `zone_id` en `ipzilon_subnet`, `ipzilon_next_subnet`, `ipzilon_last_subnet` (nuevo atributo)

| Atributo | Tipo | Modo | Modificadores |
|---|---|---|---|
| `zone_id` | number | Optional + Computed | `UseStateForUnknown`; **sin** `RequiresReplace` |

Semántica:

- `next_`/`last_subnet`: zona en la que buscar el bloque libre. Sin `zone_id` en una Network con
  zonas, IPzilon busca **fuera** de las zonas.
- `subnet`: aserción de que el CIDR está dentro de esa zona (IPzilon la valida al crear y al
  modificar).
- En el estado, siempre la zona que contiene la subred según IPzilon (`null` si ninguna).
- Se envía a IPzilon solo cuando está en la configuración. Cambios de zona en IPzilon no producen
  diff si `zone_id` no está configurado; nunca recrean la subred.

```hcl
resource "ipzilon_next_subnet" "hp1" {
  network_id    = ipzilon_network.avd.id
  zone_id       = ipzilon_network_zone.pooled.id
  prefix_length = 28
  name          = "hp_pooled_01"
}
```

## `data "ipzilon_network_zones"` (nuevo)

| Argumento | Tipo | Notas |
|---|---|---|
| `id` | number | Excluyente con los demás |
| `network_id` | number | Opcional |
| `name` | string | Exacto (servidor); puede devolver varias zonas |
| `cidr` | string | Exacto (servidor) |
| `items` | list(object) | `id`, `network_id`, `name`, `cidr`, `description` |

Sin filtros devuelve todas las zonas. Orden: el de IPzilon (CIDR numérico, después id).

```hcl
data "ipzilon_network_zones" "pooled" {
  cidr = "10.0.16.0/23" # sin ningún id
}
# data.ipzilon_network_zones.pooled.items[0].id
```

## `data "ipzilon_subnets"` (argumentos y atributo nuevos)

| Argumento | Tipo | Notas |
|---|---|---|
| `zone_id` | number | Subredes de esa zona; no exige `network_id` |
| `no_zone` | bool | Subredes fuera de toda zona; exige `network_id`; excluyente con `zone_id` |
| `items[*].zone_id` | number | Zona que contiene cada subred (`null` si ninguna) |

Se mantienen `id` y `network_id` con el mismo comportamiento; `id` sigue siendo excluyente con
el resto.

```hcl
data "ipzilon_subnets" "pooled" {
  zone_id = data.ipzilon_network_zones.pooled.items[0].id
}
```

## Errores visibles

| Situación | Momento | Mensaje |
|---|---|---|
| Zonas contra IPzilon < 3.1.0 | `plan`/`apply` | `<feature> requires IPzilon >= 3.1.0 (server reports X.Y.Z)` |
| `name` con mayúsculas | `plan` | atributo debe estar en minúsculas |
| `cidr` IPv6 o mayor que /8 | `plan` | igual que `ipzilon_subnet` |
| `zone_id` + `no_zone`; `id` + otros | `plan` | filtros excluyentes |
| `no_zone` sin `network_id` | `plan` | `no_zone requires network_id` |
| Reglas de zona/subred de IPzilon (400/404/409) | `apply` | `detail` de IPzilon tal cual |
| Búsqueda truncada | `apply` | `Address space too fragmented` + sugerencia de recurso con CIDR explícito |
