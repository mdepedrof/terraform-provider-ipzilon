# Quickstart: validar la adaptación a IPzilon 3.0.0

**Feature**: [spec.md](./spec.md) | Contratos: [api-client.md](./contracts/api-client.md),
[terraform-schema.md](./contracts/terraform-schema.md)

## Prerrequisitos

- Go (versión de `go.mod`) y Terraform ≥ 1.5.
- Instancia de IPzilon **3.0.0** (rama `feat/002-performance-scale-audit` o imagen publicada),
  con un token de API `ipam_…` de administrador. Para V6 y V7, además una instancia **2.3.1**
  (`ghcr.io/mdepedrof/ipzilon:2.3.1`).
- `~/.terraformrc` con `dev_overrides` apuntando al binario local (ver `CONTRIBUTING.md`).

```bash
make build install
export IPZILON_URL=http://localhost:8000 IPZILON_TOKEN=ipam_xxx
```

## V0 — Verificación automática (sin API)

```bash
go build ./... && go vet ./... && go test ./...
```

Esperado: todo en verde; incluye tests de paginación, reintentos, detección de versión,
traducción de errores y validadores CIDR (FR-019).

## V1 — Data sources de listado (US1, SC-002)

Sobre un inventario con > 1000 elementos en al menos un listado (p. ej. una subred /22):

```hcl
data "ipzilon_ip_addresses" "all"  { subnet_id = var.subnet_22 }
data "ipzilon_ip_addresses" "free" {
  subnet_id = var.subnet_22
  status    = "available"
}
data "ipzilon_hubs" "by_site"      { site_id = var.site_id }
output "n_all" { value = length(data.ipzilon_ip_addresses.all.items) }   # 1023
output "free_ids_null" { value = alltrue([for i in data.ipzilon_ip_addresses.free.items : i.id == null]) }
```

Esperado: `n_all` = tamaño de la subred menos la dirección de red, que la API no lista (1023 en una /22), y igual al `total` de la API; ninguna dirección repetida; `free_ids_null = true`
salvo libres anotadas; `ipzilon_hubs` devuelve lo mismo que la UI.

## V2 — `ipzilon_ip_address` (US2)

1. `apply` sobre una dirección libre → creado, `id` no nulo.
2. Segundo recurso sobre la misma dirección → falla con `IP address in use`.
3. Dirección fuera de la subred → falla con `IP address outside subnet`.
4. Cambiar `hostname` → update in situ; `plan` posterior sin cambios.
5. `destroy` → la dirección vuelve a aparecer libre (`id = null`) en V1.
6. Liberarla desde la UI y ejecutar `plan` → Terraform propone recrearla.
7. `terraform import ipzilon_ip_address.x <id>` de una dirección ocupada → `plan` sin cambios.
8. `status = "available"` en `ipzilon_ip_address` o `ipzilon_next_ip_address` → `plan` falla
   indicando que solo se admiten `used` o `reserved`.
9. `destroy` de un `ipzilon_next_ip_address` → en los logs de IPzilon aparece `DELETE /ips/{id}`
   (no un `PATCH`) y la dirección vuelve a estar libre.

## V3 — Reintentos (US3, SC-003)

Arrancar IPzilon 3.0.0 con `RATE_LIMIT_DEFAULT=60/minute` y aplicar ≥ 200 recursos
(p. ej. `count = 200` de `ipzilon_next_ip_address`):

```bash
TF_LOG=DEBUG terraform apply -parallelism=10 -auto-approve 2>&1 | grep -c "retrying IPzilon request"
```

Esperado: `apply` completo sin errores; en el log aparecen reintentos por 429. Luego `destroy`
con el mismo resultado.

**SC-005**: con el mismo log, (peticiones − reintentos) / recursos gestionados ≤ 2 (tarea T048).

## V4 — Búsqueda truncada (US4)

En una red muy fragmentada (p. ej. /12 con /28 alternos ocupados), `ipzilon_next_subnet` con un
`prefix_length` grande → error `Address space too fragmented` que sugiere un CIDR explícito.
Si no se puede reproducir, queda cubierto por el test unitario de V0.

## V5 — Validaciones en `plan` (US5)

```hcl
resource "ipzilon_network" "big" {
  scope_id = 1
  name     = "big"
  cidr     = "10.0.0.0/7"
}

resource "ipzilon_subnet" "v6" {
  network_id = 1
  name       = "v6"
  cidr       = "fd00::/64"
}
```

Esperado: `terraform plan` falla con `/7 is too large: the maximum size is /8` e
`IPv6 subnets are not supported`, sin llamar a la API de escritura.

## V6 — Migración de estado (SC-004)

1. Con provider `v2.2.1` + IPzilon 2.3.1: `apply` de un ejemplo completo
   (`examples/terraform/`), incluidos `ipzilon_ip_address` y `ipzilon_next_ip_address`.
2. Migrar IPzilon a 3.0.0 (procedimiento §8 del informe).
3. Con el provider adaptado: `terraform plan` → **No changes**.

## V7 — IPzilon 2.x rechazado (FR-016)

Provider adaptado contra IPzilon 2.3.1: `terraform plan` → error
`Unsupported IPzilon version` que menciona `>= 3.0.0` y `~> 2.2`.
