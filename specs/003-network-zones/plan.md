# Implementation Plan: Zonas de red (IPzilon 3.1.0)

**Branch**: `003-network-zones` | **Date**: 2026-10-01 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/003-network-zones/spec.md`

## Summary

IPzilon 3.1.0 añade zonas de red: bloques con nombre dentro de una Network que agrupan subredes
por contención de CIDR. El provider incorpora tres recursos nuevos (`ipzilon_network_zone`,
`ipzilon_next_network_zone`, `ipzilon_last_network_zone`), el atributo `zone_id` (Optional +
Computed, `UseStateForUnknown`, sin `RequiresReplace`) en los tres recursos de subred, el data
source `ipzilon_network_zones` (búsqueda por nombre/CIDR sin ids, filtrada en el servidor) y los
filtros `zone_id`/`no_zone` en `ipzilon_subnets`. Las zonas exigen IPzilon ≥ 3.1.0 mediante una
comprobación de versión sin peticiones extra que solo se activa cuando se usan zonas; el resto
sigue funcionando contra 3.0.x. Todo es aditivo: release MINOR **v3.1.0**. Detalle en
[research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.25.8 (`go.mod`)

**Primary Dependencies**: `terraform-plugin-framework` v1.19.0, `-framework-validators` v0.19.0,
`terraform-plugin-testing` v1.16.0 (solo tests). Sin dependencias nuevas.

**Storage**: N/A (estado de Terraform)

**Testing**: `go test ./...` con `net/http/httptest` (unitarios, sin API); `TF_ACC=1 make testacc`
contra IPzilon 3.1.0 (`IPZILON_API_URL`, `IPZILON_TOKEN`, `IPZILON_TEST_SITE_ID`,
`IPZILON_TEST_ADDRESS_SPACE`).

**Target Platform**: plugin de Terraform (binarios GoReleaser multi-plataforma)

**Project Type**: Terraform provider (cliente de la API REST de IPzilon)

**Performance Goals**: sin peticiones extra en recursos existentes; la comprobación de versión usa
la versión ya leída de `/health`. Una petición adicional solo en `ipzilon_subnets` con `zone_id`
sin `network_id`.

**Constraints**: cambios aditivos (FR-023); HTTP solo en `internal/client`; sin exponer métricas de
zona (`total_ips`, `used_ips`, `available_ips`, `alert_*`, `subnet_count`); filtros en el servidor;
configuraciones sin zonas sin diff y compatibles con IPzilon 3.0.x.

**Scale/Scope**: 3 recursos nuevos, 3 recursos modificados (`zone_id`), 1 data source nuevo, 1
modificado, 1 método nuevo en el cliente, ejemplos y docs regeneradas, README.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio / norma | Evaluación | Estado |
|---|---|---|
| I. API como fuente de verdad | Reglas de zona (contención, solapes, subredes a caballo, nombre único) validadas solo en IPzilon; el provider añade validaciones de forma (minúsculas, IPv4, ≤ /8, 8..32). `zone_id` en el estado = valor calculado por la API. 404 en Read → fuera del estado. **No** se exponen las métricas de `NetworkZoneResponse`. Filtros `name`/`cidr`/`network_id`/`zone_id`/`no_zone` resueltos en el servidor | ✅ |
| II. Asignación atómica | `next_`/`last_network_zone` delegan en `next-/last-available-zone`; `cidr` Computed + `UseStateForUnknown`; `network_id` y `prefix_length` con `RequiresReplace`. `next_`/`last_subnet` con `zone_id` siguen en el endpoint atómico. `force: true` intacto en `ipzilon_subnet` | ✅ |
| III. Compatibilidad de esquema y estado | Solo atributos y recursos nuevos, todos opcionales en los existentes → MINOR v3.1.0, sin `UpgradeState` (el nuevo `zone_id` se rellena en el primer refresh). Import por id con `import.sh` en los 3 recursos nuevos; Read rellena todo (`prefix_length` derivado). Atributos no actualizables in situ (`network_id`, `prefix_length`) con `RequiresReplace` | ✅ |
| IV. Docs y ejemplos | Ejemplos `resource.tf`/`import.sh`/`data-source.tf` nuevos; `Description` en todos los atributos; `make generate` en el mismo commit; pin `~> 3.0` sin cambios | ✅ |
| V. Pruebas | Unitarios para conversiones, URLs, versión, envío de `zone_id` y rollback; tabla de `TestReadAfterImport` ampliada; `TestAccImport_*` nuevos antes del tag | ✅ |
| Restricciones técnicas | Solo plugin-framework y validators; HTTP en `internal/client`; sin credenciales en ejemplos | ✅ |
| Flujo | Rama `003-network-zones` desde `main` actualizada; commits con `commit-message`; sin coautoría | ✅ |

**Re-check post-diseño (Fase 1)**: sin cambios; ninguna violación. El rollback defensivo
(research R3) borra solo la subred que el propio provider acaba de crear en la misma operación.

## Project Structure

### Documentation (this feature)

```text
specs/003-network-zones/
├── plan.md              # Este fichero
├── research.md          # Fase 0: decisiones R1–R11
├── data-model.md        # Fase 1: modelos de cliente y de estado
├── quickstart.md        # Fase 1: guía de validación
├── contracts/
│   ├── terraform-schema.md  # Interfaz pública (esquema Terraform)
│   └── api-client.md        # Llamadas a la API de IPzilon 3.1.0
├── checklists/requirements.md
└── tasks.md             # Fase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
internal/
├── client/
│   ├── client.go            # + RequireAPIVersion, MinZonesAPIVersion
│   ├── client_test.go       # + tests de RequireAPIVersion
│   └── models.go            # + NetworkZone*, AllocateZoneBody; ZoneID en Subnet*/AllocateSubnetBody
├── resources/
│   ├── network_zone.go      # nuevo: ipzilon_network_zone
│   ├── next_network_zone.go # nuevo: ipzilon_next_network_zone (+ allocZoneFromAPI)
│   ├── last_network_zone.go # nuevo: ipzilon_last_network_zone (comparte conversión)
│   ├── zones.go             # nuevo: helpers de zona (validador de minúsculas, zone_id a enviar, rollback, zoneIDFollowsCIDR)
│   ├── zones_test.go        # nuevo: unitarios de los helpers
│   ├── network_zone_test.go       # nuevo: unitarios de ipzilon_network_zone
│   ├── subnet_zone_test.go        # nuevo: zone_id en los recursos de subred
│   ├── alloc_network_zone_test.go # nuevo: next_/last_network_zone
│   ├── subnet.go            # + zone_id
│   ├── next_subnet.go       # + zone_id
│   ├── last_subnet.go       # + zone_id
│   ├── read_after_import_test.go  # + 3 recursos nuevos y zone_id en subredes
│   └── acc_import_test.go   # + TestAccImport_*Zone y escenarios de subred en zona
├── datasources/
│   ├── network_zones.go     # nuevo: ipzilon_network_zones
│   ├── subnets.go           # + zone_id/no_zone y items[*].zone_id
│   ├── helpers.go           # + zonesURL, networkSubnetsURL
│   ├── helpers_test.go      # + tests de URLs
│   ├── network_zones_test.go  # nuevo: unitarios del data source de zonas
│   └── subnets_test.go        # nuevo: filtros zone_id/no_zone
└── provider/provider.go     # registro de los recursos y data source nuevos

examples/
├── resources/ipzilon_network_zone/{resource.tf,import.sh}
├── resources/ipzilon_next_network_zone/{resource.tf,import.sh}
├── resources/ipzilon_last_network_zone/{resource.tf,import.sh}
├── resources/ipzilon_{subnet,next_subnet,last_subnet}/resource.tf   # + ejemplo con zone_id
└── data-sources/{ipzilon_network_zones,ipzilon_subnets}/data-source.tf

docs/                        # regenerado con make generate
README.md                    # sección de zonas y versión mínima de IPzilon
```

**Structure Decision**: estructura existente del provider (constitución, *Restricciones
técnicas*). Los recursos de zona siguen el patrón de `next_subnet`/`last_subnet` (conversión
única `*FromAPI` compartida) y los data sources el de `helpers.go` (constructores de URL
testeables).

## Fases de implementación (orden sugerido para `/speckit-tasks`)

1. **Cliente**: modelos, `ZoneID` en subredes, `RequireAPIVersion` + tests.
2. **US1 (P1)** `ipzilon_network_zone` + ejemplo + import + tabla de import.
3. **US2 (P1)** `zone_id` en los tres recursos de subred, regla de envío desde la configuración,
   rollback defensivo + tests; verificar `plan` vacío en configuraciones sin zonas.
4. **US3 (P2)** `ipzilon_network_zones` y filtros de `ipzilon_subnets` + tests de URLs.
5. **US4 (P3)** `next_`/`last_network_zone` + ejemplos + import.
6. Docs (`make generate`), README, aceptación contra 3.1.0 y release v3.1.0.

## Complexity Tracking

Sin violaciones de la constitución que justificar.
