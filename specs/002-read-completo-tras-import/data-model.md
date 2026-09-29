# Data Model: Read completo tras import

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md)

Sin cambios de esquema. Este documento fija, por recurso, de dónde sale cada atributo del estado.
Regla común: **todo atributo del modelo sale de la respuesta de la API o se deriva de ella**.

| Recurso | Endpoint de lectura | Atributos directos de la API | Atributos derivados |
|---|---|---|---|
| `ipzilon_hub` | `GET /hubs/{id}` | id, site_id, name, address_space, location, description | — |
| `ipzilon_scope` | `GET /scopes/{id}` | id, hub_id, parent_id, name, kind, cidr, description | — |
| `ipzilon_network` | `GET /networks/{id}` | id, scope_id, name, cidr, description | — |
| `ipzilon_subnet` | `GET /subnets/{id}` | id, network_id, name, cidr, description | — |
| `ipzilon_ip_address` | `GET /ips/{id}` | id, subnet_id, address, status, is_azure_reserved, hostname, description | — |
| `ipzilon_next_ip_address` | `GET /ips/{id}` | **igual que `ip_address`, incl. `subnet_id`** | — |
| `ipzilon_next_subnet` / `ipzilon_last_subnet` | `GET /subnets/{id}` | id, network_id, name, cidr, description | `prefix_length` = máscara de `cidr` |
| `ipzilon_next_network` | `GET /networks/{id}` | id, scope_id, name, cidr, description | `prefix_length` = máscara de `cidr` |

## Entidades

- **Modelo de recurso** (`<x>Model`): estructura con un campo por atributo del esquema.
- **`<x>FromAPI`**: función pura `objeto de la API → (modelo, error)`; único punto de construcción
  del estado en Create, Read y Update. Error si falta un dato imprescindible (IP sin `id`, CIDR no
  parseable).
- **`prefixLengthValue(cidr)`**: `(types.Int64, error)`; única derivación de `prefix_length`.
- **Fila del test de import**: `{nombre de tipo, constructor, ruta de lectura, JSON de respuesta}`.

## Reglas de validación

- Tras `Read` con estado inicial `{id}`: ningún atributo `Required` nulo; ningún atributo
  desconocido.
- `Description`/`Hostname` nulos en la API → nulos en el estado (no cadena vacía).
- Tras Update: estado == `FromAPI(respuesta del PATCH)`.

## Transiciones

`import (solo id)` → `Read` → estado completo → `plan` vacío. 404 en `Read` → el recurso se elimina
del estado (sin cambios).
