# Research: Read completo tras import en todos los recursos

**Feature**: [spec.md](./spec.md) | **Plan**: [plan.md](./plan.md) | **Fecha**: 2026-09-29

Fuentes: brief de origen y lectura del código en `main` (`4fba8f5`).

## Hallazgos de la auditoría del código

- `next_ip_address`: Create rellena `SubnetID`; **Read y Update no** → bug reportado.
- `next_subnet`, `next_network`, `last_subnet`: Read rellena todo, pero `prefix_length` solo
  `if err == nil` (**el error de parseo se traga en silencio**); Create usa `plan.PrefixLength`
  en vez de derivarlo; Update reasigna campos a mano sobre el estado.
- `next_network` Update solo copia `Name` al estado (ni `description` ni `cidr`): confirma que la
  mezcla con el estado previo es frágil, aunque `Computed`+`Optional` lo enmascare hoy.
- `hub`, `scope`, `network`, `subnet`, `ip_address`: ya usan `*FromAPI` en Create, Read y Update.
- Data sources: ninguno hace `req.State.Get`/lee estado previo; construyen desde la API.
- No existen tests de aceptación (`resource.Test`) ni dependencia `terraform-plugin-testing`.
- El prototipo del brief (`/tmp/claude-0/tp3`) no existe en esta máquina.

## R1. Mecanismo común: función `FromAPI` por recurso vs abstracción genérica

- **Decision**: cada recurso define `<x>FromAPI(obj client.X) (<x>Model, error)` y **Create, Read y
  Update terminan siempre con `State.Set(ctx, <x>FromAPI(obj))`**. Los recursos `next_subnet` y
  `last_subnet` (mismo modelo y misma respuesta) comparten la función de conversión.
  `prefix_length` se obtiene siempre con `prefixLengthValue(cidr)`.
- **Rationale**: cierra el hueco de raíz (no hay estado previo que "olvidar") con un patrón que ya
  funciona en 5 recursos; cambio mecánico, sin cambiar esquema; el compilador no obliga a rellenar
  campos, pero el test genérico (R3) sí.
- **Alternatives considered**: (a) parche solo en `next_ip_address` → tercera recurrencia segura;
  (b) interfaz/genéricos `Resource[T]` con Create/Read/Update comunes → reescritura grande, riesgo
  de regresión y sobreingeniería para 9 recursos; (c) reflexión para comprobar campos nulos en
  runtime → oculta errores; mejor comprobarlo en test.

## R2. `Update` toma el objeto completo del PATCH

- **Decision**: sí. El estado tras Update es `FromAPI(respuesta del PATCH)`; se descarta la mezcla
  con el estado. Si la API no devolviera `id` (caso IP, R6 de la spec 001) se trata como error.
- **Rationale**: elimina la divergencia Create/Update; refleja normalizaciones del servidor.
- **Alternatives considered**: mantener estado + parches → origen del bug.

## R3. Red de seguridad unitaria

- **Decision**: `read_after_import_test.go` con tabla `{nombre, constructor, ruta y JSON de la
  API}`. Para cada fila: servidor `httptest` que responde el JSON, cliente apuntando a él,
  `Configure`, `State` construido desde el esquema con todo nulo salvo `id`, `Read()`, y
  comprobación recorriendo `resp.Schema.Attributes`: falla si un atributo `Required` es nulo o
  cualquiera es desconocido, indicando recurso y atributo. **Test de completitud**: la lista de
  recursos del provider (`Resources()`) debe coincidir con la tabla (por `TypeName`); falta o
  sobra → falla. Se valida que el test falla sin el arreglo (regresión deliberada).
- **Rationale**: cubre el patrón "estado solo con `id`" del brief sin API real, en la CI actual.
- **Alternatives considered**: solo aceptación (la CI no tiene API); reflexión sobre modelos.

## R4. Tabla de recursos y provider

- **Decision**: el test de completitud no duplica la lista: instancia `provider.New(...)` y llama
  a `Resources(ctx)`. Tabla y completitud viven en `internal/resources/read_after_import_test.go`
  con **paquete externo `resources_test`**, que puede importar `internal/provider` sin ciclo
  (los constructores `New*Resource` son exportados). El cliente de test se construye con un
  `client.Client{BaseURL, HTTPClient}` literal (campos exportados), sin la sonda `/health`.
- **Rationale**: el registro real es la fuente de verdad de "qué recursos existen".
- **Alternatives considered**: lista manual en el test (se desincroniza — justo lo que se evita).

## R5. Aceptación import → plan vacío

- **Decision**: añadir `github.com/hashicorp/terraform-plugin-testing` (solo `_test.go`) y un
  `acc_import_test.go` por recurso con `resource.Test`: paso 1 crear; paso 2 `ImportState` +
  `ImportStateVerify`; paso 3 `PlanOnly` con `ExpectEmptyPlan`. `PreCheck` exige `IPZILON_API_URL`,
  `IPZILON_TOKEN` y ejecuta solo con `TF_ACC=1`. `make testacc` sube su timeout (120 s es poco
  para 9 recursos). Se añade el paso a la checklist de release en `CONTRIBUTING.md`.
- **Rationale**: es la única prueba que detecta diferencias reales entre API y esquema.
- **Alternatives considered**: script shell con `terraform import` (no reutilizable ni verificable
  en Go); test manual (no exigible, como hasta ahora).

## R6. `hostname` igual a `description` en IPs migradas (id 2204)

- **Decision**: no compensarlo en el provider (Principio I: la API es la fuente de verdad).
  `hostname` y `description` son `Optional+Computed`: si la configuración no fija `hostname`, no
  hay diff; si lo fija distinto de lo que devuelve la API, el diff in situ es **legítimo** y
  converge tras un `apply` (PATCH). Se cubre con un test unitario (la API devuelve ambos iguales
  → el estado los refleja tal cual) y se documenta en `README.md`/`CONTRIBUTING.md` cómo migrar
  (fijar `hostname` explícitamente o aceptar un único apply). Verificación real: con la IP 2204 en
  la aceptación manual previa a la release.
- **Rationale**: normalizar en el cliente ocultaría el drift y divergiría del backend.
- **Alternatives considered**: `DiffSuppress`/plan modifier que ignore la diferencia (oculta
  cambios reales); tratar `hostname==description` como nulo (inventa semántica).

## R7. Data sources

- **Decision**: sin cambios de código. Se añade una comprobación en revisión (regla en
  `CONTRIBUTING.md`) y, si es barato, un test que asegure que ningún data source lee `req.State`.
- **Rationale**: el defecto exige mezclar con estado previo, que los data sources no tienen.

## R8. Alcance y versión

- **Decision**: `v3.0.1` (PATCH): sin cambio de esquema. Se elimina el error tragado de
  `cidrPrefixLength`: ahora falla con diagnóstico claro (comportamiento corregido, no ruptura).
- **Rationale**: Principio III.
