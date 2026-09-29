# Contrato: interfaz Terraform expuesta a los usuarios (provider v3.0.0)

La interfaz pública del provider es su esquema. Esta versión **no** renombra, elimina ni hace
obligatorio ningún atributo. Cambios visibles:

## Compatibilidad

| Provider | IPzilon soportado |
|---|---|
| `~> 2.2` | 2.x (≤ 2.3.1) |
| `~> 3.0` | ≥ 3.0.0 |

## `data "ipzilon_ip_addresses"`

```hcl
data "ipzilon_ip_addresses" "free" {
  subnet_id = ipzilon_subnet.app.id
  status    = "available" # recomendado: sin status se lista la subred entera
}
# data.ipzilon_ip_addresses.free.items[*].id  => null en las direcciones libres
```

## `resource "ipzilon_ip_address"`

Mismo esquema salvo `status`, que solo admite `used` (por defecto) o `reserved` (igual en
`ipzilon_next_ip_address`). **Cambio incompatible**: `status = "available"` ya no se admite;
para liberar una dirección, destruye el recurso. Comportamiento:

- `create`: ocupa `address` en `subnet_id` (falla si ya está en uso o fuera de rango).
- `update`: in situ para `status`, `hostname`, `description`.
- `destroy`: libera la dirección (borra su registro en IPzilon; vuelve a quedar libre).
- `import`: por `id` de una dirección ocupada, reservada o anotada.

## Validaciones nuevas en `plan` (solo al crear o cambiar el valor)

| Recurso | Atributo | Regla |
|---|---|---|
| `ipzilon_hub` | `address_space` | prefijo ≥ /8 |
| `ipzilon_scope`, `ipzilon_network` | `cidr` | prefijo ≥ /8 |
| `ipzilon_subnet` | `cidr` | prefijo ≥ /8 e IPv4 |

## Errores nuevos o cambiados

Ver [api-client.md](./api-client.md#errores-traducidos-a-diagnósticos).
