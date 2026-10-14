# Research: Hacer mockeable `items` de los data sources de listado

**Feature**: [spec.md](spec.md) | **Date**: 2026-10-14

## R1. Causa del fallo en `terraform test`

**Decision**: el fallo está en Terraform, no en el provider. Se evita cambiando la forma en que se
declara `items`, no su tipo.

**Rationale**: en Terraform v1.16.0, `internal/moduletest/mocking/fill.go` resuelve los valores de
mock de cada atributo computado con `fillAttribute`:

```go
if attribute.NestedType != nil {
    if !in.Type().IsObjectType() {
        return cty.NilVal, path.NewErrorf("incompatible types; expected object type, found %s", ...)
    }
    switch attribute.NestedType.Nesting {
    case configschema.NestingSingle, configschema.NestingGroup: // rellena con el valor del mock
    case configschema.NestingList: return cty.ListValEmpty(...), nil // ignora el valor del mock
    case configschema.NestingSet:  return cty.SetValEmpty(...), nil
    case configschema.NestingMap:  return cty.MapValEmpty(...), nil
    }
}
return fillType(in, attribute.Type, path)
```

Un atributo anidado de tipo lista rechaza una lista (`found tuple`) e ignora un objeto (lista
vacía): son exactamente los dos síntomas de la issue. El código no ha cambiado desde noviembre de
2023 (commit `48aa56a37`, Terraform 1.7) y sigue igual en `main`. Un atributo simple con
`type = list(object({...}))` pasa por `fillType`, que convierte la lista elemento a elemento y
genera los campos que falten.

**Alternatives considered**: reportar el fallo upstream y esperar (descartado por el propietario:
sin fecha y bloquea los tests de los consumidores); mover las lecturas a un módulo hijo en cada
consumidor para usar `override_module` (obliga a reestructurar cada stack).

## R2. Comprobación empírica con un prototipo

**Decision**: el cambio a `schema.ListAttribute{ElementType: types.ObjectType{...}}` cubre todos
los escenarios de la US1.

**Rationale**: con `ipzilon_hubs` cambiado en local y Terraform 1.16.0 + `dev_overrides`:

| Escenario | Antes | Después |
|-----------|-------|---------|
| `override_data` con `items = [{ id = 1 }]` (test de la issue) | `expected object type, found tuple` | pasa; `items[0].id == 1` |
| `override_data` con dos elementos parciales (`id`, `name`) | error | `length == 2`, `[for …: h.name] == ["x","y"]` |
| `override_data` con `items = []` | error | `length == 0` |
| `mock_data` con `defaults = { items = [{ id = 7 }] }` | lista vacía | `length == 1`; los campos no dados reciben valores generados (p. ej. `location = "oq36tigk"`) |
| `mock_provider` sin override | lista vacía | lista vacía (sin cambio) |

Los tests unitarios existentes (`go test ./internal/...`) pasan sin cambios con el prototipo: el
framework sigue convirtiendo `[]hubItem` (structs con `tfsdk`) en una lista de objetos.

## R3. Compatibilidad del estado y de los consumidores

**Decision**: no hace falta `UpgradeState` ni versión MAJOR.

**Rationale**: el tipo implícito de un `ListNestedAttribute` es `list(object({...}))`, idéntico al
de `ListAttribute{ElementType: ObjectType{...}}` con los mismos campos. El valor guardado en el
estado y el que ven las expresiones son iguales. Además los data sources se releen en cada plan y
no tienen `UpgradeState`. Lo único que cambia es la representación en
`terraform providers schema -json` (`type` en lugar de `nested_type`) y en los docs (`List of
Object` en lugar de `Attributes List`).

## R4. Documentación por campo

**Decision**: una definición ordenada de campos por data source (nombre, tipo, descripción) de la
que un helper deriva el `ObjectType` y la descripción de `items`. Formato de la descripción:
`<resumen> Each item has: `campo` — <descripción>. `campo` — <descripción>.` (ver
[contracts/items-attribute.md](contracts/items-attribute.md)).

**Rationale**: `ListAttribute` no admite descripción por campo; tfplugindocs lista los campos del
objeto solo con su tipo. Mantener una lista ordenada (no un mapa) conserva el orden en que se
escriben hoy los campos (`id` primero) y hace la salida determinista. Las descripciones actuales
contienen frases completas con puntos internos (p. ej. `id` de `ipzilon_ip_addresses`), por eso se
usa `campo — texto` en lugar de paréntesis.

**Alternatives considered**: ordenar alfabéticamente un mapa (menos legible: `address_space`
delante de `id`); plantillas propias de tfplugindocs (se desincronizan del código); no documentar
los campos (pierde información, descartado por el propietario).

## R5. Ejecutar el ejemplo de `terraform test` en la CI

**Decision**: un módulo de prueba en `test/mock/` (fuera de `examples/`, que es de tfplugindocs),
un objetivo `make testmock` que compila el provider en un directorio temporal y lo usa con
`dev_overrides` vía `TF_CLI_CONFIG_FILE`, y un job `mock` en `.github/workflows/ci.yml` con
`hashicorp/setup-terraform@v4` y Terraform `1.16.x`.

**Rationale**: con `dev_overrides` no hace falta `terraform init` ni publicar el binario; los mocks
no llaman a IPzilon, así que la CI no necesita credenciales. `1.16.x` es la versión de la issue y
la más reciente (1.16.6); fijar la minor evita que una versión nueva cambie el resultado sin
cambiar el provider.

**Alternatives considered**: lanzar `terraform test` desde un test de Go (acopla `go test` a tener
Terraform instalado y rompe el job actual); usar `terraform-plugin-testing` (no ejecuta
`terraform test` ni mocks).

## R6. Guarda contra regresiones sin Terraform

**Decision**: un test unitario que instancia los 7 data sources, obtiene su esquema y comprueba que
`items` es un `schema.ListAttribute` con `ElementType` de tipo objeto (y por tanto no un atributo
anidado), y que la descripción nombra cada campo.

**Rationale**: cumple FR-008 con `go test` solo, de modo que el job `Build & Test` también detecta
la regresión aunque no se ejecute el job `mock`.
