# Contract: atributo `items` de los data sources de listado

Aplica a `ipzilon_hubs`, `ipzilon_ip_addresses`, `ipzilon_network_zones`, `ipzilon_networks`,
`ipzilon_scopes`, `ipzilon_sites` e `ipzilon_subnets`.

## Tipo del valor (sin cambios)

`list(object({...}))` con los campos de [data-model.md](../data-model.md). `Computed`, nunca nulo.

## Esquema publicado (`terraform providers schema -json`)

Antes:

```json
"items": { "nested_type": { "nesting_mode": "list", "attributes": { "id": { "type": "number", "computed": true, "description": "Hub ID." }, ... } }, "computed": true, ... }
```

Después:

```json
"items": { "type": ["list", ["object", { "id": "number", "site_id": "number", ... }]], "computed": true, "description": "Matching hubs; ... Each item has: ..." }
```

## Mocks en `terraform test` (Terraform >= 1.16.0)

| Entrada del mock | Resultado de `items` |
|------------------|----------------------|
| `items = [{ id = 1 }]` | 1 elemento, `id = 1`, resto de campos generados |
| `items = [{...}, {...}]` | 2 elementos con los valores dados |
| `items = []` | lista vacía |
| sin `items` en el mock | lista vacía |
| campo con tipo incorrecto | error de conversión que señala el campo |

## Documentación generada (ejemplo `ipzilon_hubs`)

```markdown
- `items` (List of Object) Matching hubs; an empty list when nothing matches. Each item has: `id` — Hub ID. `site_id` — ID of the site the hub belongs to. `name` — Hub name. `address_space` — Hub address space CIDR. `location` — Free-text location label. `description` — Free-text description. (see [below for nested schema](#nestedatt--items))

<a id="nestedatt--items"></a>
### Nested Schema for `items`

Read-Only:

- `address_space` (String)
- `description` (String)
- `id` (Number)
- `location` (String)
- `name` (String)
- `site_id` (Number)
```

El resumen inicial de cada data source no cambia (p. ej. `Matching zones, ordered by CIDR.` en
`ipzilon_network_zones`).
