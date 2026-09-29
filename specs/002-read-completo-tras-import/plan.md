# Implementation Plan: Read completo tras import en todos los recursos

**Branch**: `002-read-completo-tras-import` | **Date**: 2026-09-29 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/002-read-completo-tras-import/spec.md`

## Summary

Tras `terraform import`, `ipzilon_next_ip_address` deja `subnet_id` (RequiresReplace) vacío y el
`plan` propone reemplazar la IP. La causa es estructural: cuatro recursos (`next_ip_address`,
`next_subnet`, `next_network`, `last_subnet`) completan el estado campo a campo sobre el estado
previo, en Create, Read y Update por separado. Se unifica: **cada recurso tiene una única función
`<x>FromAPI(objeto) (modelo, error)` que construye el modelo entero desde la respuesta de la API**
y se usa en Create, Read y Update (los nueve recursos, sin `state.X = …` sueltos); `prefix_length`
se deriva en un único helper (`prefixLengthValue`) que devuelve error en vez de callar. Red de
seguridad: test unitario por tabla (`httptest`, estado solo con `id`, `Read()`, sin atributos
obligatorios nulos ni desconocidos) más un test de completitud contra los recursos registrados en
el provider. Aceptación: `make testacc` con `terraform-plugin-testing` (crear → importar →
plan vacío) por recurso, requisito previo al tag `v3.0.1`. Los data sources no tienen el patrón
(no leen estado previo): solo se verifica. Detalle en [research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.25.8 (`go.mod`)

**Primary Dependencies**: `terraform-plugin-framework` v1.19.0, `-validators` v0.19.0. **Una
dependencia nueva, solo de test**: `terraform-plugin-testing` (aceptación, R5).

**Storage**: N/A (estado de Terraform)

**Testing**: `go test ./...` con `net/http/httptest` (unitarios, sin API); `TF_ACC=1 make testacc`
con un binario `terraform` y un IPzilon real (`IPZILON_API_URL`, `IPZILON_TOKEN`).

**Target Platform**: plugin de Terraform (binarios GoReleaser multi-plataforma)

**Project Type**: Terraform provider (cliente de la API REST de IPzilon)

**Performance Goals**: sin cambio: el número de peticiones por recurso no aumenta (Read sigue
haciendo un único GET; Update ya recibe el objeto del PATCH).

**Constraints**: esquema público intacto (FR-010); HTTP solo en `internal/client`; sin exponer
`alert_*`/`total_ips`/`used_ips`.

**Scale/Scope**: 9 recursos (4 refactorizados a fondo, 5 revisados/alineados), 6 data sources
(solo revisión), 2 ficheros de test nuevos, 1 de aceptación, `CONTRIBUTING.md`, checklist de release.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio / norma | Evaluación | Estado |
|---|---|---|
| I. API como fuente de verdad | El estado se construye solo de lo que dice la API; 404 sigue eliminando del estado; `prefix_length` se deriva del CIDR devuelto; sin métricas de UI | ✅ |
| II. Asignación atómica | Los `next_*`/`last_subnet` siguen delegando en el endpoint atómico; `RequiresReplace` y `UseStateForUnknown` intactos | ✅ |
| III. Compatibilidad de esquema y estado | Sin cambios de esquema → PATCH `v3.0.1`, sin `UpgradeState` ni *Breaking Changes*. Materializa el requisito "Read rellena todo tras import" | ✅ |
| IV. Docs y ejemplos | Esquema sin cambios ⇒ `docs/` no cambia (`make generate` sin diff); se actualiza `CONTRIBUTING.md`; versión de ejemplos sigue `~> 3.0` | ✅ |
| V. Pruebas | Test que reproduce el bug sin API (Principio V "todo bug SHOULD llevar test"); aceptación previa a release | ✅ |
| Restricciones técnicas | Solo plugin-framework; `terraform-plugin-testing` es del mismo ecosistema HashiCorp y solo en `_test.go`; Dependabot ya cubre `go.mod` | ✅ |
| Flujo | Rama `002-…` desde `main` actualizada; commits con `commit-message`; sin coautoría | ✅ |

**Re-check post-diseño (Fase 1)**: sin cambios; ninguna violación, no se requiere *Complexity
Tracking*. La única dependencia nueva es de test.

## Project Structure

### Documentation (this feature)

```text
specs/002-read-completo-tras-import/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── read-after-import.md
├── checklists/requirements.md
└── tasks.md             # /speckit-tasks (no creado aquí)
```

### Source Code (repository root)

```text
internal/
├── client/                       # sin cambios
├── datasources/                  # solo revisión (sin cambios previstos)
├── provider/
│   └── provider.go               # exponer la lista de recursos para el test de completitud
└── resources/
    ├── helpers.go                # cidrPrefixLength → prefixLengthValue(cidr) (types.Int64, error)
    ├── next_ip_address.go        # nextIPFromAPI + Create/Read/Update
    ├── next_subnet.go            # nextSubnetFromAPI + Create/Read/Update
    ├── next_network.go           # nextNetworkFromAPI + Create/Read/Update
    ├── last_subnet.go            # lastSubnetFromAPI + Create/Read/Update
    ├── hub.go scope.go network.go subnet.go ip_address.go   # alinear a "FromAPI en las 3 ops"
    ├── read_after_import_test.go # tabla + completitud (US2/US3)
    └── acc_import_test.go        # aceptación import → plan vacío (US4), TF_ACC
CONTRIBUTING.md                   # regla + checklist de release
```

**Structure Decision**: se mantiene la estructura vigente; el mecanismo común es un patrón
(función `FromAPI` por recurso + helper de derivación) y no una abstracción genérica (R1).

## Complexity Tracking

Sin violaciones que justificar.
