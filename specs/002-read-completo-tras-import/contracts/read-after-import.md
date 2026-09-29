# Contrato interno: construcción del estado desde la API

**Feature**: [spec.md](./spec.md) | El esquema público de Terraform **no cambia** (ver
[data-model.md](./data-model.md)); este contrato fija invariantes para el código y los tests.

## C1. Invariante de los recursos (FR-001, FR-002, FR-004)

Para todo recurso `X` registrado en el provider:

1. Existe `xFromAPI(obj) (xModel, error)` y es la **única** forma de rellenar `State` en
   `Create`, `Read` y `Update`.
2. `Read` no lee `req.State` salvo para obtener `id` (y para decidir eliminar en 404).
3. `Update` termina con `FromAPI(respuesta del PATCH)`.
4. Ningún error de derivación se ignora (`prefixLengthValue` devuelve error → diagnóstico).

## C2. Contrato del test genérico (FR-005, FR-006)

- Entrada por fila: constructor del recurso, ruta y JSON de la API.
- Procedimiento: estado = esquema con todo nulo salvo `id` → `Read` → recorrer atributos.
- Falla si: atributo `Required` nulo, o cualquier atributo desconocido, o `Read` con diagnóstico
  de error. Mensaje: `<tipo>.<atributo>`.
- Completitud: `{tipos de provider.Resources()} == {tipos de la tabla}`.

## C3. Contrato de aceptación (FR-007)

Por recurso: `crear` → `import + ImportStateVerify` → `plan` con `ExpectEmptyPlan`. Entorno:
`TF_ACC=1`, `IPZILON_API_URL`, `IPZILON_TOKEN`, binario `terraform`. Requisito previo a todo tag.
