# Implementation Plan: Hacer mockeable `items` de los data sources de listado

**Branch**: `fix/26-mockable-items` | **Date**: 2026-10-14 | **Spec**: [spec.md](spec.md)

**Input**: Feature specification from `specs/006-mockable-list-items/spec.md`

## Summary

`terraform test` no puede dar valor a `items` en los 7 data sources de listado porque Terraform
(desde 1.7) ignora o rechaza los mocks de atributos computados **anidados** de tipo lista
([research R1](research.md#r1-causa-del-fallo-en-terraform-test)). Se declara `items` como
`schema.ListAttribute` con `ElementType: types.ObjectType{...}`: el tipo del valor
(`list(object)`) no cambia, así que el estado y las expresiones de los consumidores siguen igual.
Para conservar la documentación, cada data source define una lista ordenada de campos de la que un
helper deriva el `ObjectType` y una descripción de `items` que enumera los campos. Se añade un
módulo `test/mock/` con `terraform test`, un objetivo `make testmock`, un job en la CI y un test
unitario que impide volver al atributo anidado. Se publica como `v3.2.2`.

## Technical Context

**Language/Version**: Go (versión de `go.mod`)

**Primary Dependencies**: `terraform-plugin-framework` (`datasource/schema`, `types`, `attr`);
CLI de Terraform 1.16.x solo para `make testmock` y el job de CI.

**Storage**: N/A (sin cambios de estado ni `UpgradeState`, ver [R3](research.md#r3-compatibilidad-del-estado-y-de-los-consumidores))

**Testing**: `go test ./...` (unitarios); `terraform test` con `mock_provider` en `test/mock/`

**Target Platform**: provider de Terraform (linux/darwin/windows vía GoReleaser)

**Project Type**: plugin de Terraform (single project)

**Performance Goals**: N/A

**Constraints**: sin cambios en el valor de `items` en lecturas reales; docs generados por
tfplugindocs sin edición manual; CI sin acceso a IPzilon

**Scale/Scope**: 7 data sources, 1 helper nuevo, 1 test unitario nuevo, 1 módulo de
`terraform test`, 1 job de CI, README

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio | Evaluación |
|-----------|------------|
| I. API como fuente de verdad | ✅ No cambian llamadas, filtros ni datos expuestos. |
| II. Asignación atómica | ✅ No aplica (solo data sources). |
| III. Compatibilidad de esquema y estado | ✅ No se renombra ni elimina nada y el tipo del valor es idéntico; versión PATCH (`v3.2.2`). Los data sources no tienen estado que migrar. |
| IV. Docs y ejemplos sincronizados | ✅ Docs regenerados en el mismo PR; `examples/` no cambia. Los campos de `items` no pueden tener `Description` propia en un `ListAttribute`; según la enmienda v1.0.3 del Principio IV (incluida en esta PR) se describen en la `Description` de `items`, generada desde una única fuente. |
| V. Pruebas | ✅ Bug con test que lo reproduce sin API (`test/mock/`) y guarda unitaria con `go test`. `build`, `vet` y `test` siguen siendo obligatorios. |
| Restricciones técnicas | ✅ Sin `terraform-plugin-sdk/v2`; código en `internal/datasources`. |
| Flujo | ✅ Rama `fix/26-mockable-items` desde `main` actualizada; commit con `commit-message`. |

**Post-design re-check**: sin violaciones. La constitución se enmienda a v1.0.3 (PATCH, aclaración del Principio IV) en esta misma PR.

## Project Structure

### Documentation (this feature)

```text
specs/006-mockable-list-items/
├── plan.md
├── research.md
├── data-model.md
├── quickstart.md
├── contracts/
│   └── items-attribute.md
├── checklists/
│   └── requirements.md
└── tasks.md             # /speckit-tasks
```

### Source Code (repository root)

```text
internal/datasources/
├── items.go                 # NUEVO: itemField + itemsAttribute() (ObjectType + descripción)
├── items_test.go            # NUEVO: formato de la descripción y guarda de los 7 esquemas
├── hubs.go                  # hubItemSchema → hubItemFields; items vía itemsAttribute
├── ip_addresses.go          # ídem
├── network_zones.go         # ídem
├── networks.go              # ídem
├── scopes.go                # ídem
├── sites.go                 # ídem
└── subnets.go               # ídem

test/mock/                   # NUEVO: módulo para terraform test (no se publica)
├── main.tf                  # lee los 7 data sources; outputs y postcondiciones
└── tests/
    └── list_data_sources.tftest.hcl   # override_data (parcial, varios, vacío) y mock_data

GNUmakefile                  # NUEVO objetivo testmock
.github/workflows/ci.yml     # NUEVO job "Mock tests" (setup-terraform@v4, 1.16.x)
README.md                    # sección de testing con mocks y make testmock en Development
.specify/memory/constitution.md  # enmienda v1.0.3 del Principio IV
docs/data-sources/*.md       # regenerados (7 ficheros)
```

**Structure Decision**: proyecto único existente. El helper vive en `internal/datasources` junto a
los demás helpers de data sources. El módulo de prueba va en `test/mock/` porque `examples/` lo
procesa tfplugindocs y no admite módulos de prueba.

## Diseño

### Helper (`internal/datasources/items.go`)

```go
type itemField struct {
    name        string
    typ         attr.Type
    description string
}

// itemsAttribute declares items as a plain list of objects instead of a
// nested attribute, because terraform test cannot mock computed nested
// lists (hashicorp/terraform internal/moduletest/mocking/fill.go).
func itemsAttribute(summary string, fields []itemField) schema.ListAttribute
```

- `ElementType`: `types.ObjectType{AttrTypes: map[name]typ}`.
- `Description`: `summary + " Each item has: " + join("`name` — description", " ")`
  (formato en [contracts/items-attribute.md](contracts/items-attribute.md)).
- `Computed: true`.

### Data sources

En cada uno, `xxxItemSchema` (`schema.NestedAttributeObject`) se sustituye por
`xxxItemFields []itemField` con el mismo orden, tipos y textos, y `items` pasa a
`itemsAttribute("<resumen actual>", xxxItemFields)`. Los structs de modelo, `Read` y las
conversiones no cambian.

### Tests

- `items_test.go`: descripción generada para una lista de campos fija; para los 7 data sources,
  `items` es `schema.ListAttribute`, su `ElementType` es `types.ObjectType`, los nombres de
  `AttrTypes` coinciden con los campos declarados y la descripción contiene cada `` `campo` ``.
- Los tests de `Read` existentes (`empty_lists_test.go`, `global_lists_test.go`, …) validan que el
  struct de cada elemento sigue encajando con el tipo.

### `test/mock/` y CI

- `main.tf`: un bloque por data source con una postcondición `length(self.items) == N` cuando
  aplica, y outputs con `length()`, `items[0].id` y un `for`.
- `tests/list_data_sources.tftest.hcl`: `mock_provider "ipzilon" {}` y `run` con `override_data`
  (un elemento parcial, varios elementos, lista vacía) para los 7 data sources, y un fichero o
  bloque con `mock_data`. Incluye el caso literal de la issue.
- `make testmock`: `go build` a un directorio temporal, genera un `TF_CLI_CONFIG_FILE` con
  `dev_overrides` para `registry.terraform.io/mdepedrof/ipzilon` y ejecuta
  `terraform -chdir=test/mock test`.
- CI: job `mock` con `actions/checkout@v7`, `actions/setup-go@v7`,
  `hashicorp/setup-terraform@v4` (`terraform_version: 1.16.x`, `terraform_wrapper: false`) y
  `make testmock`.

### README

- Nueva sección *Testing with mocks*: desde v3.2.2 `items` se puede mockear con `override_data` /
  `mock_data`; ejemplo mínimo.
- *Development*: `make testmock`.

## Complexity Tracking

Sin violaciones que justificar: la descripción de los campos de `items` queda cubierta por la
enmienda v1.0.3 del Principio IV.
