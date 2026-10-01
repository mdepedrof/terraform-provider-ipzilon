# Implementation Plan: Filtros sin ids, lista vacía y mover hubs de site (IPzilon 3.2.0)

**Branch**: `004-global-filters-hub-move` | **Date**: 2026-10-01 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/004-global-filters-hub-move/spec.md`

## Summary

Implementa al 100 % la issue #16 y la issue #21 sobre IPzilon **3.2.0** (publicada el
2026-10-01):

1. Los cinco data sources de listado que aún devuelven `items = null` pasan a devolver `[]`.
2. `ipzilon_ip_addresses` gana `address` (ruta existente `?address=`): un elemento (ocupada o
   libre) o error si la dirección no es de la subred.
3. `ipzilon_hubs`, `ipzilon_scopes`, `ipzilon_networks` e `ipzilon_subnets` permiten buscar sin
   id de padre mediante los listados globales de IPzilon 3.2.0 (`GET /hubs/`, `/scopes/`,
   `/networks/`, `/subnets/`), con el filtrado en el servidor. Con id de padre se sigue usando el
   listado por padre actual, de modo que nada cambia contra 3.1.x.
4. `ipzilon_hub.site_id` deja de forzar la recreación: se envía en el `PATCH` y el hub se mueve
   conservando su id; se exige IPzilon ≥ 3.2.0 en `plan` y se comprueba el resultado.

Validación de forma en `plan` con `ValidateConfig` (IP, CIDR sin bits de host en búsquedas
globales, combinaciones de filtros). Todo es aditivo: release MINOR **v3.2.0**. Detalle en
[research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.25.8 (`go.mod`)

**Primary Dependencies**: `terraform-plugin-framework` v1.19.0, `-framework-validators` v0.19.0,
`terraform-plugin-testing` v1.16.0 (solo tests). Sin dependencias nuevas (`net/netip` de la
biblioteca estándar para validar IP/CIDR).

**Storage**: N/A (estado de Terraform)

**Testing**: `go test ./...` con `net/http/httptest` (`fakeAPI`, `readDataSource` existentes);
`TF_ACC=1 make testacc` contra IPzilon 3.2.0 con las variables actuales más
`IPZILON_TEST_ALT_SITE_ID` (nueva, para mover un hub).

**Target Platform**: plugin de Terraform (binarios GoReleaser multi-plataforma)

**Project Type**: Terraform provider (cliente de la API REST de IPzilon)

**Performance Goals**: ninguna petición extra en los flujos actuales; la comprobación de versión
usa la versión leída de `/health`. `ipzilon_ip_addresses` con `address` hace una única petición
en lugar de listar la subred.

**Constraints**: cambios aditivos (FR-021); HTTP solo en `internal/client`; filtros en el servidor
(sin recorrer el inventario en cliente); filtros con id de padre idénticos y compatibles con
IPzilon 3.1.x; sin exponer métricas de UI.

**Scale/Scope**: 5 data sources modificados, 1 recurso modificado (`ipzilon_hub`), 2 constantes
y 1 predicado en el cliente, 1 campo en `HubUpdate`, helpers de URL/validación, ejemplos, docs
regeneradas, README.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio / norma | Evaluación | Estado |
|---|---|---|
| I. API como fuente de verdad | Filtros sin ids resueltos en los listados globales de IPzilon (nada se filtra en cliente salvo `kind` con `hub_id`, que ya existía y el listado por padre no admite). Validaciones del provider solo de forma (IP, CIDR sin bits de host, combinaciones). Reglas del movimiento (tipo de site, nombre, solape) solo en IPzilon, con su mensaje. Sin métricas de UI | ✅ |
| II. Asignación atómica | No se tocan recursos de asignación | ✅ N/A |
| III. Compatibilidad de esquema y estado | Solo atributos opcionales nuevos y relajación de requisitos → MINOR v3.2.0, sin `UpgradeState`. Import de `ipzilon_hub` sin cambios. `site_id` deja `RequiresReplace` porque **sí** es actualizable in situ con IPzilon ≥ 3.2.0; contra versiones anteriores el provider no actualiza a ciegas: falla en `plan` con la versión mínima y comprueba el resultado tras el `PATCH` (ver nota) | ✅ |
| IV. Docs y ejemplos | `Description` en **todos** los atributos de los data sources tocados, incluidos los que ya faltaban (`id`, ids de padre de cada elemento e `items`, tarea T031); ejemplos de los 5 data sources y de `ipzilon_hub`; `make generate` en el mismo commit; pin `~> 3.0` sin cambios; README (compatibilidad) | ✅ |
| V. Pruebas | Unitarios de URLs, elección de ruta, versión/405, `address`, `ValidateConfig`, lista vacía, hub (`ModifyPlan`, `site_id` ignorado). Aceptación nueva para búsqueda global y movimiento de hub antes del tag | ✅ |
| Restricciones técnicas | Solo plugin-framework y validators; HTTP en `internal/client` | ✅ |
| Flujo | Rama `004-global-filters-hub-move` desde `main` actualizada; commits con `commit-message`; sin coautoría | ✅ |

**Nota (Principio III, `RequiresReplace`)**: la norma exige `RequiresReplace` para atributos "que
no puedan actualizarse in situ en la API". Desde IPzilon 3.2.0 `site_id` se actualiza in situ, y
esa es la versión que el provider exige para cambiarlo. No es una excepción, pero se documenta
porque el comportamiento contra 3.1.x pasa de "recrear el hub" a "error de versión en `plan`",
que es lo que pide la spec (US4-4): recrear un hub con hijos no es viable en la práctica.

**Re-check post-diseño (Fase 1)**: sin cambios; ninguna violación. `Complexity Tracking` vacío.

## Project Structure

### Documentation (this feature)

```text
specs/004-global-filters-hub-move/
├── plan.md              # Este fichero
├── research.md          # Fase 0: decisiones R1–R10
├── data-model.md        # Fase 1: modelos de cliente, data sources y hub
├── quickstart.md        # Fase 1: guía de validación
├── contracts/
│   ├── terraform-schema.md  # Interfaz pública (esquema Terraform y errores)
│   └── api-client.md        # Llamadas a la API de IPzilon 3.2.0
├── checklists/requirements.md
└── tasks.md             # Fase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
internal/
├── client/
│   ├── client.go            # + MinGlobalListsAPIVersion, MinHubMoveAPIVersion, IsMethodNotAllowed
│   ├── client_test.go
│   └── models.go            # HubUpdate + SiteID
├── datasources/
│   ├── helpers.go           # + URLs globales (hubs/scopes/networks/subnets), networkCIDR,
│   │                        #   requireGlobalLists, globalListError
│   ├── helpers_test.go
│   ├── sites.go             # items = []
│   ├── hubs.go              # ruta global sin site_id, ValidateConfig, items = []
│   ├── scopes.go            # ruta global sin hub_id, ValidateConfig, items = []
│   ├── networks.go          # ruta global sin padre, ValidateConfig, items = []
│   ├── subnets.go           # name/cidr → ruta global, ValidateConfig
│   ├── ip_addresses.go      # address, ValidateConfig, items = []
│   └── *_test.go            # tests nuevos/ampliados por data source
└── resources/
    ├── hub.go               # site_id sin RequiresReplace, ModifyPlan, Update con site_id
    ├── hub_test.go          # nuevo: ModifyPlan/Update del movimiento
    └── acc_*_test.go        # TestAccHub_MoveSite, TestAccDataSources_GlobalLookup
examples/
├── data-sources/ipzilon_{hubs,scopes,networks,subnets,ip_addresses}/data-source.tf
└── resources/ipzilon_hub/resource.tf
docs/                        # regenerado con make generate
README.md                    # Compatibility + búsquedas sin ids
```

**Structure Decision**: proyecto Go único con la estructura de la constitución (`internal/client`,
`internal/datasources`, `internal/resources`); no se añaden paquetes. Los tests de aceptación de
data sources se ubican en `internal/resources` junto al harness existente (`acc_import_test.go`),
que ya sabe crear la jerarquía hub/scope/network/subnet.

## Complexity Tracking

Sin violaciones de la constitución que justificar.
