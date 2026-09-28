# Data Model: Adaptación del provider a IPzilon 3.0.0

**Feature**: [spec.md](./spec.md) | **Research**: [research.md](./research.md)

Solo se describen los modelos que cambian. Hub, Scope, Network, Subnet y Site no cambian de
forma (§9.2 del informe).

## Modelos del cliente (`internal/client/models.go`)

### `Page[T]` (nuevo)

| Campo | Tipo | Notas |
|---|---|---|
| `Items` | `[]T` | Elementos de la página, en el orden de la API |
| `Total` | `int` | Total que cumple el filtro (no los devueltos) |

Uso exclusivo de `GetAll` (R1). Invariantes del recorrido: `offset` crece en `len(Items)`;
termina con `len(Items) == 0 || offset >= Total`.

### `IPAddress` (modificado)

| Campo | Antes | Después | Notas |
|---|---|---|---|
| `ID` | `int64` | `*int64` | `nil` para direcciones libres sin fila |
| resto | — | sin cambios | `created_at`/`updated_at` no se mapean |

### `IPAddressRegister` (nuevo) — cuerpo de `POST /subnets/{id}/ips`

| Campo | JSON | Tipo | Regla |
|---|---|---|---|
| `Address` | `address` | `string` | Obligatorio; IPv4/IPv6 válida |
| `Status` | `status` | `string` | Siempre enviado; `used` por defecto en el provider |
| `Hostname` | `hostname,omitempty` | `*string` | Opcional |
| `Description` | `description,omitempty` | `*string` | Opcional |

### `Client` (modificado)

Campos nuevos no exportados para reintentos (R3): `retryMax` (5), `retryBaseWait` (1 s),
`retryMaxWait` (60 s), `sleep func(ctx, d) error` (inyectable en tests). Campo nuevo
`APIVersion string` rellenado por la sonda `/health` (R2).

### `APIError` (sin cambios de forma)

Helpers nuevos: `IsConflict(err)`, `IsSearchTruncated(err)`, y `ErrUnsupportedAPIVersion`
(error centinela envuelto con la versión detectada).

## Ciclo de vida de una dirección IP (3.0)

```text
                 POST /subnets/{id}/ips (201, id nuevo)
   [libre, id=null] ───────────────────────────────▶ [ocupada/reservada/anotada, id=N]
          ▲                                                │   │
          │  PATCH /ips/N → available sin anotaciones      │   │ PATCH /ips/N (otros cambios)
          │  (200, id=null)  ó  DELETE /ips/N (204)        │   └──────▶ misma fila, id=N
          └────────────────────────────────────────────────┘
   Re-ocupar tras liberar ⇒ id distinto (N' ≠ N). GET /ips/N tras liberar ⇒ 404.
   El provider libera siempre con DELETE /ips/N (destroy) y nunca planifica status=available.
```

Implicaciones para el provider:

- `ipzilon_ip_address` / `ipzilon_next_ip_address`: el `id` en estado identifica la fila; un
  404 en `Read` elimina el recurso del estado (sin cambios).
- `ipzilon_ip_addresses`: `items[*].id` es nulo para libres.

## Esquemas Terraform afectados

| Recurso / data source | Atributo | Cambio | ¿Incompatible? |
|---|---|---|---|
| `ipzilon_ip_addresses` | `items[*].id` | Puede ser `null` | No (ya `Computed`); documentado |
| `ipzilon_ip_addresses` | `status`, descripción del data source | Advertencia de coste | No |
| `ipzilon_ip_address` | `address` | Validador de formato IP; descripción actualizada | No |
| `ipzilon_ip_address`, `ipzilon_next_ip_address` | `status` | Solo `used` o `reserved` | **Sí** (FR-020, *Breaking Changes*) |
| `ipzilon_ip_address` | `id`; `ipzilon_ip_addresses` `items[*].subnet_id` | Se añade `Description` (Principio IV) | No |
| `ipzilon_hub` | `address_space` | Tope /8 al crear o cambiar | No (la API ya lo rechaza) |
| `ipzilon_scope`, `ipzilon_network` | `cidr` | Tope /8 al crear o cambiar | No |
| `ipzilon_subnet` | `cidr` | Tope /8 e IPv4 al crear o cambiar; descripción sobre `force` | No |

Ninguno requiere `UpgradeState`; el cambio de `status` no tiene migración automática posible y
se documenta el procedimiento manual.
