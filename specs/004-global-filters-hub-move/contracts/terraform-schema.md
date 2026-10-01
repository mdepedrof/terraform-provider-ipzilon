# Contrato: esquema Terraform (interfaz pública)

Cambios visibles para el usuario en v3.2.0 del provider. Todo es aditivo.

## Data sources

### `ipzilon_hubs`

```hcl
data "ipzilon_hubs" "weu" {
  address_space = "10.0.0.0/16"   # sin site_id → requiere IPzilon >= 3.2.0
}
```

- Filtros: `id` | (`site_id`?, `name`?, `address_space`?). Sin `site_id`: búsqueda global.
- Descripción del data source: "List hubs. Provide id (singular lookup) or any combination of
  site_id, name and address_space. Without site_id the lookup is global (IPzilon >= 3.2.0)."

### `ipzilon_scopes`

```hcl
data "ipzilon_scopes" "avd" {
  kind = "project"
  name = "avd"
}
```

- Filtros: `id` | (`hub_id`?, `parent_id`?, `root_only`?, `kind`?, `cidr`?, `name`?).
- `root_only` requiere `hub_id`. Sin `hub_id`: búsqueda global y `kind` en servidor.

### `ipzilon_networks`

```hcl
data "ipzilon_networks" "avd" {
  cidr = "10.0.16.0/22"
}
```

- Filtros: `id` | (`hub_id`? xor `scope_id`?, `cidr`?, `name`?). Sin padre: búsqueda global.
- `hub_id` en búsqueda global y por padre: networks de cualquier scope del hub.

### `ipzilon_subnets`

```hcl
data "ipzilon_subnets" "hosts" {
  cidr = "10.0.16.64/26"
}
```

- Nuevos: `name` (String, Optional), `cidr` (String, Optional).
- Filtros: `id` | (`network_id`?, `zone_id`?, `no_zone`?, `name`?, `cidr`?), con al menos uno.
- `name`/`cidr` → búsqueda global (IPzilon >= 3.2.0); incompatibles con `no_zone`.

### `ipzilon_ip_addresses`

```hcl
data "ipzilon_ip_addresses" "gw" {
  subnet_id = 42
  address   = "10.0.1.17"
}
```

- Nuevo: `address` (String, Optional). Requiere `subnet_id`; incompatible con `id` y `status`.
- Resultado: exactamente un elemento (`id = null`, `status = "available"` si está libre) o error.

### Todos los data sources de listado

- `items` es `[]` (nunca `null`) sin resultados.

## Errores (resumen)

| Situación | Fase | Resumen del diagnóstico |
|---|---|---|
| CIDR mal formado en búsqueda global | plan | `Invalid CIDR` — `'foo' is not a valid CIDR` |
| CIDR con bits de host en búsqueda global | plan | `Invalid CIDR` — `'10.0.16.5/22' is not a network address; did you mean '10.0.16.0/22'?` |
| `address` no es IP | plan | `Invalid address` |
| `address` sin `subnet_id` / con `id` / con `status` | plan | `Conflicting filters` / `Missing filter` |
| `root_only` sin `hub_id` | plan | `Missing filter` — `root_only requires hub_id` |
| `no_zone` con `name`/`cidr` | plan | `Conflicting filters` |
| Búsqueda global contra IPzilon < 3.2.0 | plan/read | `IPzilon version not supported` — `… requires IPzilon >= 3.2.0` |
| `address` fuera de la subred | read | `IP address not found` — `<X> is not an address of subnet <N>` |
| Padre inexistente | read | mensaje de IPzilon (`Hub not found`, …) |

## Recurso `ipzilon_hub`

- `site_id`: Required, **sin** `RequiresReplace`. Cambiarlo es una actualización in situ.

```hcl
resource "ipzilon_hub" "weu" {
  site_id       = var.site_id   # cambiar el site mueve el hub conservando su id
  name          = "hub-weu"
  address_space = "10.0.0.0/16"
}
```

| Situación | Fase | Diagnóstico |
|---|---|---|
| Cambio de site contra IPzilon < 3.2.0 | plan | `IPzilon version not supported` — `moving ipzilon_hub to another site requires IPzilon >= 3.2.0` |
| Site de destino inexistente | apply | `Update hub failed` — `API error 404: Site not found` |
| Site de otro tipo / nombre repetido / solape | apply | `Update hub failed` — `API error 409: <detail>` |
| Servidor ignora `site_id` (versión desconocida) | apply | `IPzilon version not supported` — `… the server ignored site_id` |
