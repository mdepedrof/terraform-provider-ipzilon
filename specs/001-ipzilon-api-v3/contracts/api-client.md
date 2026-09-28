# Contrato: cliente HTTP del provider ↔ API IPzilon 3.0.0

Qué consume el provider de la API y qué comportamiento garantiza `internal/client`.
Referencia de la API: informe *Cambios de contrato — IPzilon 3.0.0*.

## Endpoints consumidos

| Uso en el provider | Método y ruta | Cambio 3.0 | Tratamiento |
|---|---|---|---|
| Sonda / versión | `GET /health` (y `/api/health`) | — | Lee `version`; `MAJOR < 3` ⇒ error en `Configure` |
| `ipzilon_sites` | `GET /sites/?name=` | Paginado | `GetAll` |
| `ipzilon_hubs` | `GET /sites/{id}/hubs?…` | Paginado | `GetAll` |
| `ipzilon_scopes` | `GET /hubs/{id}/scopes?…` | Paginado | `GetAll` |
| `ipzilon_networks` | `GET /hubs/{id}/networks?…`, `GET /scopes/{id}/networks?…` | Paginado | `GetAll` |
| `ipzilon_subnets` | `GET /networks/{id}/subnets` | Paginado | `GetAll` |
| `ipzilon_ip_addresses` | `GET /subnets/{id}/ips[?status=]` | Paginado; `id` nulo | `GetAll` + `ID *int64` |
| `ipzilon_ip_address` Create | `POST /subnets/{id}/ips` | Nuevo uso | Ver tabla de errores |
| `ipzilon_next_subnet` / `ipzilon_last_subnet` | `POST /networks/{id}/next-available-subnet` / `last-available-subnet` | 409 truncado | `IsSearchTruncated` |
| `ipzilon_next_network` | `POST /scopes/{id}/next-available-network` | 409 truncado | `IsSearchTruncated` |
| Lecturas/altas/cambios/bajas por `id` | `GET/POST/PATCH/DELETE /{hubs,scopes,networks,subnets}/…`, `GET/PATCH /ips/{id}` | Sin cambios (salvo tope /8) | — |
| Baja de `ipzilon_ip_address` / `ipzilon_next_ip_address` | `DELETE /ips/{id}` | En 3.0 libera (antes borraba) | Sustituye al `PATCH` a `available`; 404 se ignora |
| `ipzilon_next_ip_address` | `POST /subnets/{id}/reserve-ip` | Sin cambios | — |

## Paginación

Petición: `path` + `limit=1000&offset=N` (se concatena con `&` si `path` ya tiene query).
Respuesta esperada: `{"items": [...], "total": <int>}`. Si el cuerpo es una lista JSON
(`[`), error de versión mínima (IPzilon 2.x).

## Reintentos

| Código | ¿Reintenta? | Espera |
|---|---|---|
| 429 | Sí | `Retry-After` o backoff exp. (1, 2, 4, 8, 16 s… ±20 %), tope 60 s por espera |
| 503 | Sí | Igual |
| Resto (2xx, 3xx, 4xx, 5xx ≠ 503, errores de red) | No | — |

Se reintenta mientras la espera acumulada sea ≤ 10 min (máx. 30 reintentos). Error final:
`API error <code>: <msg> (gave up after N retries in <duración>)`.
Mensaje de error: `detail` o, si no existe, `error` (429 trae `{"error": …}`), o el cuerpo crudo.

## Errores traducidos a diagnósticos

| Recurso | Respuesta API | Título del diagnóstico | Detalle |
|---|---|---|---|
| `ipzilon_ip_address` Create | 409 | `IP address in use` | `Address X is already in use in subnet N.` |
| `ipzilon_ip_address` Create | 400 `…is not within subnet…` | `IP address outside subnet` | `Address X is not within subnet N.` |
| `ipzilon_ip_address` Create | 404 | `Subnet not found` | `Subnet N does not exist.` |
| `next_subnet`, `last_subnet`, `next_network` Create | 409 `Search truncated…` | `Address space too fragmented` | Sugerir `ipzilon_subnet`/`ipzilon_network` con CIDR explícito + mensaje API |
| Provider `Configure` | `/health` con versión < 3.0.0 | `Unsupported IPzilon version` | `requires IPzilon >= 3.0.0 … pin the provider to ~> 2.2` |
| Cualquier listado | Lista desnuda | `Unsupported IPzilon version` | Igual |
