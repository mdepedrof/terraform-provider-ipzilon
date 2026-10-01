# Contrato: llamadas a la API de IPzilon 3.2.0

Rutas usadas por el provider. Todas las listas se recorren con `client.GetAll`
(`limit=1000&offset=N`), que devuelve `[]T{}` sin resultados.

## Rutas nuevas (IPzilon >= 3.2.0)

| Data source | Ruta | Query params enviados |
|---|---|---|
| `ipzilon_hubs` sin `site_id` | `GET /hubs/` | `name`, `address_space` |
| `ipzilon_scopes` sin `hub_id` | `GET /scopes/` | `name`, `cidr`, `kind`, `parent_id` |
| `ipzilon_networks` sin padre | `GET /networks/` | `name`, `cidr` |
| `ipzilon_subnets` con `name`/`cidr` | `GET /subnets/` | `name`, `cidr`, `network_id`, `zone_id` |

Los parámetros se codifican con `url.Values.Encode()` (orden alfabético), igual que los helpers
existentes. Contra 3.1.x estas rutas responden `405` → error de versión (research R4).

## Ruta existente con filtro nuevo (IPzilon >= 3.0.0)

| Data source | Ruta |
|---|---|
| `ipzilon_ip_addresses` con `address` | `GET /subnets/{subnet_id}/ips?address=X` |

Respuesta: la fila guardada, el elemento libre (`"id": null`, `"status": "available"`) o
`{"items": [], "total": 0}` si la dirección está fuera de la subred o mal formada.

## Mover hub (IPzilon >= 3.2.0)

```http
PATCH /hubs/{id}
{"name": "hub-weu", "address_space": "10.0.0.0/16", "location": null, "description": null, "site_id": 2}
```

- `site_id` solo se incluye cuando cambia (`omitempty`).
- `200` → `HubResponse` con el `site_id` nuevo y el mismo `id`.
- `404 Site not found`; `409` con `detail` (tipo de site, nombre repetido, `address_space`
  solapado).
- Versiones anteriores: `200` con el `site_id` antiguo (campo ignorado) → el provider lo detecta.

## Sin cambios

`GET /sites/{id}/hubs`, `GET /hubs/{id}/scopes`, `GET /hubs/{id}/networks`,
`GET /scopes/{id}/networks`, `GET /networks/{id}/subnets`, `GET /zones/`, `GET /zones/{id}` y las
búsquedas por id se siguen usando exactamente igual.
