# Data Model: Zonas de red (IPzilon 3.1.0)

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

Modelos de cliente (`internal/client/models.go`) y de estado de Terraform. El esquema completo de
cada atributo está en [contracts/terraform-schema.md](./contracts/terraform-schema.md).

## Cliente (`internal/client`)

### `NetworkZone` (nuevo; respuesta de `GET/POST/PATCH` de zonas)

| Campo Go | JSON | Tipo | Notas |
|---|---|---|---|
| `ID` | `id` | `int64` | |
| `NetworkID` | `network_id` | `int64` | |
| `Name` | `name` | `string` | minúsculas (normalizado por IPzilon) |
| `CIDR` | `cidr` | `string` | |
| `Description` | `description` | `*string` | `null` si vacía |

Las métricas de la respuesta (`total_ips`, `used_ips`, `available_ips`, `alert_percent`,
`alert_metric`, `subnet_count`) y `created_at`/`updated_at` **no** se mapean (Principio I).

### `NetworkZoneCreate` (nuevo; `POST /networks/{id}/zones`)

| Campo | JSON | Tipo |
|---|---|---|
| `Name` | `name` | `string` |
| `CIDR` | `cidr` | `string` |
| `Description` | `description,omitempty` | `*string` |

### `NetworkZoneUpdate` (nuevo; `PATCH /zones/{id}`)

| Campo | JSON | Tipo | Notas |
|---|---|---|---|
| `Name` | `name,omitempty` | `*string` | `null` no permitido por la API |
| `CIDR` | `cidr,omitempty` | `*string` | omitido en `next_`/`last_` |
| `Description` | `description` | `*string` | sin `omitempty`: `null` vacía la descripción |

### `AllocateZoneBody` (nuevo; `POST /networks/{id}/{next|last}-available-zone`)

| Campo | JSON | Tipo |
|---|---|---|
| `PrefixLength` | `prefix_length` | `int64` |
| `Name` | `name` | `string` |
| `Description` | `description,omitempty` | `*string` |

### Cambios en modelos existentes

| Modelo | Campo nuevo | JSON | Notas |
|---|---|---|---|
| `Subnet` | `ZoneID *int64` | `zone_id` | calculado por contención; `nil` fuera de zona o en IPzilon 3.0.x |
| `SubnetCreate` | `ZoneID *int64` | `zone_id,omitempty` | aserción |
| `SubnetUpdate` | `ZoneID *int64` | `zone_id,omitempty` | aserción sobre el CIDR final |
| `AllocateSubnetBody` | `ZoneID *int64` | `zone_id,omitempty` | espacio de búsqueda |

### `Client.RequireAPIVersion(min, feature string) error` (nuevo)

Compara `Client.APIVersion` (leída de `/health` al configurar) con `min`. Solo bloquea versiones
release (`X.Y.Z`) menores; versiones vacías o no release pasan. Error envuelto en
`ErrUnsupportedAPIVersion`. Constante `MinZonesAPIVersion = "3.1.0"`.

## Estado de Terraform

### `networkZoneModel` — `ipzilon_network_zone`

| Atributo | Tipo | Origen en Read |
|---|---|---|
| `id` | Int64 | `ID` |
| `network_id` | Int64 | `NetworkID` |
| `name` | String | `Name` |
| `cidr` | String | `CIDR` |
| `description` | String | `Description` |

Construido solo por `networkZoneFromAPI` (Create, Read, Update).

### `allocZoneModel` — `ipzilon_next_network_zone` / `ipzilon_last_network_zone`

| Atributo | Tipo | Origen en Read |
|---|---|---|
| `id` | Int64 | `ID` |
| `network_id` | Int64 | `NetworkID` |
| `prefix_length` | Int64 | `prefixLengthValue(CIDR)` |
| `name` | String | `Name` |
| `description` | String | `Description` |
| `cidr` | String | `CIDR` |

Una única `allocZoneFromAPI(client.NetworkZone) (allocZoneModel, error)` para ambos recursos.

### Modelos de subred (`subnetModel`, `nextSubnetModel`; `last_subnet` comparte este último)

Nuevo atributo `zone_id` (Int64, `null` fuera de zona) rellenado desde `Subnet.ZoneID` en
`subnetFromAPI` / `nextSubnetFromAPI`.

### Data sources

- `networkZonesModel`: filtros `id`, `network_id`, `name`, `cidr`; `items []networkZoneItem`
  (`id`, `network_id`, `name`, `cidr`, `description`).
- `subnetsModel`: filtros nuevos `zone_id`, `no_zone`; `subnetItem` gana `zone_id`.

## Reglas de validación

| Regla | Dónde | Momento |
|---|---|---|
| `name` de zona sin mayúsculas | validador de esquema (3 recursos de zona) | `plan` |
| `cidr` de zona IPv4 y ≤ `/8` al crear o cambiar | `cidrLimits(true)` | `plan` |
| `prefix_length` 8..32 | `int64validator.Between(8, 32)` | `plan` |
| CIDR dentro de la Network, menor que ella, sin solapes, sin subredes a caballo ni fuera | IPzilon (400) | `apply` |
| Nombre único por Network | IPzilon (409) | `apply` |
| Subred dentro de la zona declarada / zona de la misma Network | IPzilon (400/404) | `apply` |
| `zone_id` y `no_zone` excluyentes; `id` excluyente con el resto | validadores del data source | `plan` |
| `no_zone` exige `network_id` | `Read` del data source | `plan` |
| IPzilon ≥ 3.1.0 si se usan zonas | `RequireAPIVersion` | `plan`/`apply` |

## Ciclo de vida

```text
ipzilon_network_zone
  create ── POST /networks/{nid}/zones ──► existe
  name/cidr/description ── PATCH /zones/{id} ──► existe (mismo id)
  network_id ── destroy + create
  borrada fuera de TF ── Read 404 ──► fuera del estado ──► plan propone crear
  destroy ── DELETE /zones/{id} ──► subredes contenidas quedan con zone_id = null

subred (zone_id calculado)
  zona creada/redimensionada/borrada en IPzilon ──► Read actualiza zone_id
                                                     sin diff si zone_id no está configurado
```
