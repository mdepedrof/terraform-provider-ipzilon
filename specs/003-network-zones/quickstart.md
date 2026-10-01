# Quickstart: validación de zonas de red (IPzilon 3.1.0)

**Feature**: [spec.md](./spec.md) | Esquema: [contracts/terraform-schema.md](./contracts/terraform-schema.md)

## Requisitos

- Go (versión de `go.mod`) y `terraform` en el `PATH`.
- Una instancia de IPzilon **3.1.0** (`ghcr.io/mdepedrof/ipzilon:3.1.0`) y, para el escenario 6,
  otra 3.0.x.
- Token de administrador y un site/espacio de pruebas:

```bash
export IPZILON_API_URL=https://ipzilon.example.internal
export IPZILON_TOKEN=...            # nunca en ficheros versionados
export IPZILON_TEST_SITE_ID=...
export IPZILON_TEST_ADDRESS_SPACE=10.200.0.0/16
```

## 1. Verificación automática

```bash
go build ./... && go vet ./... && go test ./...   # unitarios, sin API
make generate && git diff --exit-code docs/       # docs regeneradas y commiteadas
make testacc                                      # aceptación contra 3.1.0
```

Esperado: todo en verde, incluidos los `TestAccImport_*` existentes (SC-004) y los nuevos
`TestAccImport_NetworkZone`, `_NextNetworkZone`, `_LastNetworkZone`.

## 2. Escenarios manuales (binario local con `dev_overrides`)

Partir de una configuración con hub → scope → `ipzilon_network.avd` (`10.200.16.0/22`).

| # | Paso | Resultado esperado | Spec |
|---|---|---|---|
| 1 | Añadir `ipzilon_network_zone.pooled` (`10.200.16.0/23`) y `ipzilon_next_subnet` `/28` con `zone_id` de esa zona; `apply` | Subred dentro de `10.200.16.0/23`; `zone_id` en el estado; `plan` vacío | US1-1, US2-1, SC-001 |
| 2 | `ipzilon_next_subnet` `/28` **sin** `zone_id`; `apply` | CIDR fuera de `10.200.16.0/23`; `zone_id = null` | US2-2 |
| 3 | Reducir el `cidr` de la zona a `10.200.16.0/24` cuando la subred del paso 1 queda fuera | `apply` falla con `Zone … would leave subnet '…' outside`; sin cambio en IPzilon | US1-7 |
| 4 | Cambiar `name` y `description` de la zona | Update in situ, mismo `id` | US1-2 |
| 5 | Borrar la zona en la UI de IPzilon; `plan` | Propone crear la zona; las subredes **no** se recrean (solo cambia su `zone_id` si no está configurado) | US1-6, US2-6, SC-003 |
| 6 | Contra IPzilon 3.0.x, `plan` con la zona | Error `… requires IPzilon >= 3.1.0`; sin subredes creadas | FR-022, SC-007 |
| 7 | `ipzilon_subnet` con CIDR explícito existente: añadir `zone_id` de la zona que ya lo contiene | `plan` vacío (sin update ni recreación) | US2-5 |
| 8 | `ipzilon_subnet` con `zone_id` de una zona que no contiene su CIDR | `apply` falla con `Subnet … is not within zone '…'` | US2-4 |
| 9 | `ipzilon_next_network_zone` `/25` y `ipzilon_last_network_zone` `/25` | Bloques vacíos al principio y al final; `cidr` estable en `plan`; cambiar `prefix_length` → recreación | US4 |
| 10 | `terraform import` de cada recurso de zona y de una subred en zona en un estado vacío | `plan` vacío | US1-4, SC-002 |
| 11 | `data "ipzilon_network_zones"` solo con `cidr`; solo con `name` (con nombre repetido en otra Network); con `name` + `network_id` | 1 / 2 / 1 resultados, sin escribir ids | US3-1/2, SC-005 |
| 12 | `data "ipzilon_subnets"` con solo `zone_id`; con `network_id` + `no_zone = true`; con `zone_id` + `no_zone` | Subredes de la zona / fuera de zona / error en `plan` | US3-3/4/6 |
| 13 | Configuración existente sin zonas (proyecto real) tras actualizar a v3.1.0 | `plan` sin cambios | FR-013, SC-004 |
| 14 | Reducir el `cidr` de `ipzilon_network.avd` de forma que una zona quede fuera | `apply` falla con `Network CIDR … would leave zone '…' outside` | Edge Cases |

## 3. Limpieza

`terraform destroy`; comprobar que destruir la zona no borró subredes ajenas (US1-5).
