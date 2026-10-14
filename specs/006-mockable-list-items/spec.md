# Feature Specification: Hacer mockeable `items` de los data sources de listado

**Feature Branch**: `fix/26-mockable-items`

**Created**: 2026-10-14

**Status**: Draft

**Input**: User description: "Issue #26: el atributo computado `items` de los 7 data sources de
listado no se puede mockear con `terraform test` en Terraform 1.16.0."

**Documentos de origen**:

- Issue #26 *[Bug] - items de los data sources no se puede mockear en terraform test*.
- Código de mocks de Terraform v1.16.0: `internal/moduletest/mocking/fill.go` (`fillAttribute`).

## Contexto

Los data sources de listado (`ipzilon_hubs`, `ipzilon_ip_addresses`, `ipzilon_network_zones`,
`ipzilon_networks`, `ipzilon_scopes`, `ipzilon_sites`, `ipzilon_subnets`) devuelven sus
resultados en `items`, una lista de objetos declarada en el esquema como **atributo anidado**
(*nested attribute* con modo lista).

`terraform test` no sabe dar valor a un atributo computado anidado de tipo lista, set o mapa a
partir de `override_data` o `mock_data`:

- Si el mock le da una lista (`items = [{ id = 1 }]`), Terraform la rechaza:
  `Failed to compute attribute ".items": incompatible types; expected object type, found tuple.`
- Si le da un objeto (`items = { id = 1 }`), lo acepta, pero `items` queda como lista vacía.

Es un fallo de Terraform presente desde 1.7 (noviembre de 2023) y sin cambios en `main` a fecha de
esta spec. Mientras exista, ningún stack que lea estos data sources (p. ej. `tf-az-stack-avm`, con
postcondiciones sobre `ipzilon_hubs.items` e `ipzilon_scopes.items`) puede probarse con
`mock_provider`.

Terraform sí rellena bien una lista de objetos declarada como **atributo simple** con tipo
`list(object({...}))`. El tipo del valor es el mismo en los dos casos, así que cambiar la forma de
declararlo no altera el estado ni las expresiones de los consumidores. Se ha comprobado con un
prototipo sobre `ipzilon_hubs`: el test de la issue pasa.

**Decisión**: declarar `items` como atributo simple de tipo lista de objetos en los 7 data sources.
El coste es que la documentación generada deja de mostrar una descripción por campo de cada
elemento; para no perderla, la descripción de `items` enumera los campos con su significado,
generada a partir de las descripciones que ya existen en el código (una sola fuente). No se
reporta el fallo en hashicorp/terraform.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Probar con mocks un stack que lee data sources de listado (Priority: P1)

Como mantenedor de un stack que consume el provider (p. ej. `tf-az-stack-avm`), quiero poder dar
valores a `items` con `override_data` o `mock_data` en `terraform test`, para probar la lógica del
stack (postcondiciones, `items[0].id`, bucles `for`) sin una instancia de IPzilon.

**Why this priority**: es el bug de la issue; sin esto los stacks consumidores no tienen tests
unitarios.

**Independent Test**: ejecutar con `mock_provider "ipzilon" {}` un `.tftest.hcl` que hace
`override_data` con `items = [{ id = 1 }]` sobre cada uno de los 7 data sources, con una
postcondición `length(self.items) == 1` y una aserción sobre `items[0].id`; debe pasar.

**Acceptance Scenarios**:

1. **Given** el `main.tf` y el `tests/t.tftest.hcl` de la reproducción de la issue, **When** se
   ejecuta `terraform test`, **Then** el run pasa, la postcondición se cumple y
   `items[0].id` vale `1`.
2. **Given** un `override_data` que da solo algunos campos de cada elemento
   (`items = [{ id = 1 }]`), **When** se ejecuta el plan, **Then** los campos no indicados reciben
   un valor generado por el mock, sin error.
3. **Given** un `override_data` con todos los campos de cada elemento, **When** se ejecuta el plan,
   **Then** los valores del mock llegan tal cual a las expresiones del stack.
4. **Given** un `override_data` con varios elementos o con `items = []`, **When** se ejecuta el
   plan, **Then** `length(...items)` devuelve ese mismo número.
5. **Given** un bloque `mock_data` dentro de `mock_provider` en lugar de `override_data`,
   **When** se ejecuta el plan, **Then** el resultado es el mismo que en los escenarios
   anteriores.
6. **Given** cualquiera de los 7 data sources, **When** se repiten los escenarios anteriores,
   **Then** el resultado es el mismo.

---

### User Story 2 - Actualizar el provider sin tocar los stacks existentes (Priority: P1)

Como mantenedor de un stack que ya usa el provider con una instancia real de IPzilon, quiero
actualizar a la nueva versión sin cambiar código ni ver diferencias en el plan.

**Why this priority**: el arreglo no puede romper a los consumidores actuales; tiene la misma
prioridad que la US1.

**Independent Test**: con el estado generado por la versión anterior, ejecutar `plan` con la nueva
versión sobre una configuración que usa los 7 data sources (indexación, `length()`, `for`,
postcondiciones) y comprobar que no hay cambios ni errores.

**Acceptance Scenarios**:

1. **Given** un stack con estado creado por la versión anterior, **When** se ejecuta `plan` con la
   nueva versión, **Then** no hay diferencias y no se necesita ninguna migración de estado.
2. **Given** expresiones como `items[0].id`, `length(items)`, `[for i in items : i.name]` o
   `one(items)`, **When** se evalúan con la nueva versión, **Then** dan el mismo resultado.
3. **Given** una búsqueda sin coincidencias, **When** se lee el data source, **Then** `items`
   sigue siendo una lista vacía (nunca nula).

---

### User Story 3 - Seguir entendiendo los campos de `items` en el Registry (Priority: P2)

Como usuario que consulta la documentación del provider en el Registry, quiero saber qué
significa cada campo de los elementos de `items`, igual que ahora.

**Why this priority**: es el coste del arreglo; se mitiga sin bloquear la US1.

**Independent Test**: regenerar la documentación y comprobar que, para cada data source, la
descripción de `items` nombra todos los campos del elemento con su significado.

**Acceptance Scenarios**:

1. **Given** la documentación regenerada de cualquiera de los 7 data sources, **When** se lee
   `items`, **Then** su descripción enumera cada campo del elemento con el texto que hoy aparece
   como descripción de ese campo.
2. **Given** que se añade, quita o cambia la descripción de un campo de un elemento, **When** se
   regenera la documentación, **Then** la descripción de `items` refleja el cambio sin editarla
   por separado.

### Edge Cases

- **Terraform corrige el fallo en el futuro**: el esquema nuevo sigue funcionando; volver al
  atributo anidado sería opcional y no forma parte de este cambio.
- **Herramientas que leen `terraform providers schema -json`**: `items` pasa a aparecer como
  atributo con `type` en lugar de `nested_type`. El valor y su tipo no cambian; solo afecta a
  herramientas que inspeccionan la forma del esquema.
- **Mock con un campo que no existe en el elemento**: Terraform lo descarta sin error, como con
  cualquier objeto.
- **Mock con un tipo incorrecto en un campo** (p. ej. `id = "abc"`): Terraform falla con un error
  de conversión que señala el campo; es el comportamiento esperado.
- **Campos opcionales que la API no devuelve** (p. ej. `location`, `description`): siguen
  llegando como nulos en una lectura real, igual que ahora.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: En los 7 data sources de listado, `items` MUST poder recibir valores de
  `override_data` y de `mock_data` en `terraform test` (Terraform >= 1.16.0), incluyendo listas
  con uno o varios elementos, listas vacías y elementos con solo una parte de los campos.
- **FR-002**: El tipo del valor de `items` (lista de objetos con los mismos campos y tipos que hoy)
  MUST NOT cambiar, de modo que el estado existente sea válido sin migración y las expresiones de
  los consumidores den el mismo resultado.
- **FR-003**: El contenido de `items` en una lectura real MUST NOT cambiar: mismos elementos, mismo
  orden, campos ausentes como nulos y lista vacía (nunca nula) cuando no hay coincidencias.
- **FR-004**: La descripción de `items` de cada data source MUST enumerar todos los campos del
  elemento con su significado, generada a partir de una única definición por campo en el código.
- **FR-005**: La definición del tipo de cada elemento y su documentación MUST salir de esa misma
  definición, de modo que no puedan divergir.
- **FR-006**: El repositorio MUST incluir un ejemplo ejecutable con `terraform test` y
  `mock_provider` que cubra los 7 data sources con los escenarios de la US1.
- **FR-007**: Ese ejemplo MUST ejecutarse en la CI contra el provider compilado desde el propio
  código, sin necesitar una instancia de IPzilon.
- **FR-008**: Los tests unitarios MUST comprobar que `items` no se declara como atributo anidado
  en ninguno de los 7 data sources, para que una regresión se detecte con `go test` sin Terraform.
- **FR-009**: Los tests unitarios existentes MUST seguir pasando sin cambiar sus expectativas
  sobre el contenido de `items`.
- **FR-010**: La documentación del Registry MUST regenerarse con `tfplugindocs` en el mismo cambio.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: El `.tftest.hcl` de la reproducción de la issue pasa, y lo mismo para los 7 data
  sources (7 de 7).
- **SC-002**: Un stack existente que actualiza a la nueva versión obtiene un `plan` sin cambios y
  no modifica ninguna línea de su configuración.
- **SC-003**: En la documentación regenerada, el 100 % de los campos de los elementos de `items`
  de los 7 data sources aparece descrito.
- **SC-004**: La CI ejecuta el ejemplo de `terraform test` en cada PR y falla si algún data source
  deja de ser mockeable.
- **SC-005**: 0 regresiones en los tests unitarios existentes.

## Assumptions

- Versión mínima de Terraform para los mocks: la de la issue (1.16.0). Las versiones anteriores
  del CLI siguen funcionando para `plan`/`apply`; `terraform test` con mocks existe desde 1.7.
- Cambio compatible: no se renombra ni elimina ningún atributo y el tipo del valor no cambia, así
  que encaja en una versión PATCH del provider (Principio III de la constitución). Se publicará
  como `v3.2.2`.
- Solo cambia la descripción por campo dentro del esquema publicado; la redacción de cada campo
  se mantiene.
- El ejemplo de `terraform test` vive en el repositorio (no en `examples/` de tfplugindocs, que
  solo admite los ficheros de ejemplo de la documentación) y no se publica en el Registry.
- Fuera de alcance: reportar el fallo en hashicorp/terraform, cambiar atributos anidados de los
  recursos (no tienen) y cualquier cambio en IPzilon.
