# Data Model: Filtros sin ids, lista vacía y mover hubs de site

**Spec**: [spec.md](./spec.md) | **Research**: [research.md](./research.md)

No hay entidades nuevas: cambian modelos de cliente, de configuración de data sources y del
recurso `ipzilon_hub`. Los elementos devueltos (`items`) no cambian de forma.

## Cliente (`internal/client`)

| Tipo / símbolo | Cambio |
|---|---|
| `HubUpdate` | + `SiteID *int64 \`json:"site_id,omitempty"\`` (solo se rellena al mover el hub) |
| `MinGlobalListsAPIVersion` | nueva constante `"3.2.0"` |
| `MinHubMoveAPIVersion` | nueva constante `"3.2.0"` |
| `IsMethodNotAllowed(err)` | nuevo predicado (`APIError.Code == 405`) |
| `Hub`, `Scope`, `Network`, `Subnet`, `IPAddress` | sin cambios (los listados globales devuelven los mismos elementos) |

## Configuración de data sources (`internal/datasources`)

Leyenda: **N** = nuevo, **M** = semántica modificada, = sin cambios.

### `ipzilon_hubs` (`hubsModel`)

| Atributo | Estado | Notas |
|---|---|---|
| `id` | = | búsqueda singular; gana sobre los filtros |
| `site_id` | M | opcional de verdad; sin él → listado global |
| `name` | M | aplica también sin `site_id` |
| `address_space` | M | aplica también sin `site_id`; en búsqueda global, CIDR sin bits de host |
| `items` | M | `[]` sin resultados |

### `ipzilon_scopes` (`scopesModel`)

| Atributo | Estado | Notas |
|---|---|---|
| `id` | = | |
| `hub_id` | M | opcional; sin él → listado global |
| `parent_id` | M | sin `hub_id` → filtro `parent_id` del listado global (hijos directos) |
| `root_only` | M | exige `hub_id` (error en `plan` si no) |
| `kind` | M | con `hub_id`: filtro en cliente (como hoy); sin `hub_id`: filtro en servidor |
| `cidr` | M | en búsqueda global, CIDR sin bits de host |
| `name` | = | |
| `items` | M | `[]` sin resultados |

### `ipzilon_networks` (`networksModel`)

| Atributo | Estado | Notas |
|---|---|---|
| `id` | = | |
| `hub_id`, `scope_id` | M | opcionales y excluyentes entre sí (como hoy); sin ninguno → listado global |
| `cidr` | M | en búsqueda global, CIDR sin bits de host |
| `name` | M | aplica también sin padre |
| `items` | M | `[]` sin resultados |

### `ipzilon_subnets` (`subnetsModel`)

| Atributo | Estado | Notas |
|---|---|---|
| `id` | = | |
| `network_id` | M | opcional cuando hay `name` o `cidr` |
| `zone_id` | = | con `name`/`cidr` se envía al listado global |
| `no_zone` | M | exige `network_id` (como hoy) e incompatible con `name`/`cidr` |
| `name` | N | Optional; exacto; fuerza el listado global |
| `cidr` | N | Optional; red sin bits de host; fuerza el listado global |
| `items` | = | ya era `[]` |

### `ipzilon_ip_addresses` (`ipAddressesModel`)

| Atributo | Estado | Notas |
|---|---|---|
| `id` | = | |
| `subnet_id` | = | |
| `status` | M | incompatible con `address` |
| `address` | N | Optional; IP válida; exige `subnet_id`; incompatible con `id`/`status`; 0 resultados → error |
| `items` | M | `[]` sin resultados (salvo con `address`, que falla) |

## Recurso `ipzilon_hub` (`hubModel`)

| Atributo | Antes | Ahora |
|---|---|---|
| `site_id` | Required + `RequiresReplace` | Required, actualizable in situ (IPzilon ≥ 3.2.0) |
| resto | = | = |

### Transición "mover hub"

```text
estado.site_id = A, plan.site_id = B (A ≠ B, B conocido)
  ModifyPlan: versión < 3.2.0 → error en plan
  Update: PATCH /hubs/{id} {..., site_id: B}
    404/409          → error con el detail de IPzilon; estado sin cambios (site A)
    200 y site_id=B  → estado = respuesta (mismo id)
    200 y site_id=A  → error "server ignored site_id"; estado = respuesta (site A)
```

## Reglas de validación (resumen)

| Regla | Dónde | Cuándo se omite |
|---|---|---|
| `address` es IP | `ValidateConfig` IPs | valor desconocido |
| `address` exige `subnet_id`; no con `id`/`status` | `ValidateConfig` IPs | — (se evalúa nulo vs. no nulo) |
| CIDR/`address_space` sin bits de host | `ValidateConfig` hubs/scopes/networks/subnets | padre no nulo o desconocido; valor desconocido |
| `root_only` exige `hub_id` | `ValidateConfig` scopes | — |
| `no_zone` no con `name`/`cidr` | `ValidateConfig` subnets | — |
| `kind` ∈ {`landing_zone`, `project`} | validator existente | — |
