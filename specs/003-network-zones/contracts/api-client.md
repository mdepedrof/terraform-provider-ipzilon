# Contrato: llamadas a la API de IPzilon 3.1.0 que hace el provider

Fuente: `ipzilon/specs/003-network-zones/contracts/zones-api.md`. Solo se listan las rutas que usa
el provider. Todas pasan por `internal/client`; los listados se recorren con `client.GetAll`
(`{items, total}`, `limit`/`offset`).

| Recurso / data source | Operación | Llamada | Respuestas tratadas |
|---|---|---|---|
| `ipzilon_network_zone` | Create | `POST /networks/{network_id}/zones` `{name, cidr, description?}` | 201; 400/404/409/422 → error con `detail` |
| | Read / Import | `GET /zones/{id}` | 200; 404 → `RemoveResource` |
| | Update | `PATCH /zones/{id}` `{name, cidr, description}` | 200; 400/409/422 → error |
| | Delete | `DELETE /zones/{id}` | 204; 404 → OK |
| `ipzilon_next_network_zone` | Create | `POST /networks/{network_id}/next-available-zone` `{prefix_length, name, description?}` | 201; 409 truncado → `allocationErrorDiag`; resto → error |
| `ipzilon_last_network_zone` | Create | `POST /networks/{network_id}/last-available-zone` (ídem) | ídem |
| `next_`/`last_network_zone` | Read / Update / Delete | `GET` / `PATCH {name, description}` / `DELETE /zones/{id}` | como `ipzilon_network_zone` |
| `ipzilon_subnet` | Create | `POST /subnets/` `{…, zone_id?}` | 400/404 por zona → error |
| | Update | `PATCH /subnets/{id}` `{…, force: true, zone_id?}` | ídem |
| `ipzilon_next_subnet` / `last_subnet` | Create | `POST /networks/{id}/{next\|last}-available-subnet` `{prefix_length, name?, description?, zone_id?}` | 404/400/409 → error |
| | Update | `PATCH /subnets/{id}` `{name, description, zone_id?}` | ídem |
| subredes (los tres) | Read | `GET /subnets/{id}` → `zone_id` | |
| `ipzilon_network_zones` | Read por id | `GET /zones/{id}` | 404 → error |
| | Read listado | `GET /zones/?network_id=&name=&cidr=` (paginado) | lista vacía sin error |
| `ipzilon_subnets` | `zone_id` sin `network_id` | `GET /zones/{zone_id}` (para obtener `network_id`) | 404 → error |
| | listado | `GET /networks/{network_id}/subnets?zone_id=` o `?no_zone=true` (paginado) | 400/404/422 → error |

`zone_id` solo se incluye en el cuerpo cuando está en la configuración (`omitempty`).

## Comprobación de versión

Sin peticiones adicionales: `Client.APIVersion` ya se lee de `GET /health` al configurar el
provider. `RequireAPIVersion("3.1.0", feature)` se evalúa antes de cualquier llamada de zonas o
con `zone_id`/`no_zone`.

## Rollback defensivo

Si `POST` de subred con `zone_id` responde 201 pero sin ese `zone_id` (servidor que ignora el
campo), el provider llama a `DELETE /subnets/{id}` y devuelve el error de versión.
