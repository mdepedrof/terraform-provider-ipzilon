# Feature Specification: Read completo tras import en todos los recursos

**Feature Branch**: `002-read-completo-tras-import`

**Created**: 2026-09-29

**Status**: Draft

**Input**: User description: "Corregir de forma sistémica el fallo por el que Read() tras un `terraform import` no rellena todos los atributos del state (brief `brief-read-tras-import-2026-09-29`), con un único mecanismo común para todos los recursos, revisando también los data sources, con red de seguridad automática y aceptación previa a la release v3.0.1."

**Documento de origen**: *Brief: el `Read()` tras un import no rellena todos los atributos* (2026-09-29, migración Netbox → Ipzilon de `tf-az-project-camaras`).

## Contexto

Tras `terraform import` de `ipzilon_next_ip_address`, el siguiente `plan` propone **reemplazar** el
recurso porque el atributo `subnet_id` (que fuerza reemplazo) queda vacío. Desde IPzilon 3.0 un
reemplazo libera la IP y vuelve a reservar otra, que puede ser distinta: en el proyecto `camaras`
cambiaría la dirección de dos balanceadores (ILB de events y LB interno de EMQX). La migración a
IPzilon está bloqueada por este motivo.

Es la **segunda vez** que ocurre (la v2.2.1, #14, corrigió `next_network`, `next_subnet` y
`last_subnet`). El principio III de la constitución ya exige que `Read` rellene todos los
atributos tras un import, pero nada lo comprueba automáticamente.

### Análisis de la causa y de la solución elegida

Hay dos patrones en los recursos:

| Patrón | Recursos | Resultado |
|--------|----------|-----------|
| El estado se construye entero a partir de la respuesta de la API | `hub`, `scope`, `network`, `subnet`, `ip_address` | Correcto |
| El estado se completa campo a campo sobre el estado previo, en Create, Read y Update por separado | `next_ip_address`, `next_subnet`, `next_network`, `last_subnet` | Olvidar un campo es fácil y ya ha fallado dos veces |

**Solución elegida (mecanismo único)**: los nueve recursos construyen su estado con una única
función de conversión "respuesta de la API → estado" que se usa en Create, Read y Update, sin
partir nunca del estado previo; los atributos que la API no devuelve (p. ej. la longitud de
prefijo) se derivan siempre de otros que sí devuelve (el CIDR). Sobre ello se añade una red de
seguridad automática (prueba genérica por tabla) y una aceptación import → plan vacío previa a
cada release. Se descartan los parches puntuales por recurso: no impiden la tercera recurrencia.

**Data sources (revisión hecha)**: ninguno lee estado previo; construyen sus resultados
íntegramente desde la respuesta de la API, por lo que el patrón defectuoso no existe en ellos.
Quedan cubiertos por la misma regla de construcción desde la API sin cambios funcionales; solo
se verifica que ninguno dependa de estado previo.

**Prototipo**: el brief menciona un prototipo local (rama `fix/next-ip-address-read-subnet-id`,
`/tmp/claude-0/tp3`) que **no existe ya en esta máquina**; el trabajo parte de `main`. Si el
autor lo recupera, puede servir como referencia.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Importar una IP reservada sin que el plan la reemplace (Priority: P1)

Un operador migra a IPzilon infraestructura ya existente e importa una IP con
`ipzilon_next_ip_address` (p. ej. el id 2204). El siguiente `plan` debe salir vacío.

**Why this priority**: desbloquea la migración de `camaras` y evita cambiar la IP de dos
balanceadores en producción.

**Independent Test**: importar una IP existente y ejecutar `plan`; no debe mostrar ningún cambio
ni reemplazo.

**Acceptance Scenarios**:

1. **Given** una IP reservada existente en IPzilon, **When** el operador la importa como
   `ipzilon_next_ip_address` con la misma configuración, **Then** el `plan` posterior no muestra
   cambios y no propone reemplazo.
2. **Given** el mismo import, **When** se compara el estado importado con el de un recurso creado
   por Terraform, **Then** ambos contienen los mismos atributos con los mismos valores.

---

### User Story 2 - Import → plan vacío en los nueve recursos (Priority: P1)

Un operador importa cualquiera de los recursos gestionados (`hub`, `scope`, `network`, `subnet`,
`ip_address`, `next_subnet`, `last_subnet`, `next_network`, `next_ip_address`) y el `plan`
posterior no muestra cambios.

**Why this priority**: es el mismo defecto potencial en todos los recursos; se corrige con un
único mecanismo, no recurso a recurso (ya falló dos veces con parches puntuales).

**Independent Test**: para cada recurso, partir de un estado que solo contiene el `id`, ejecutar
la lectura y comprobar que todos los atributos del esquema quedan rellenos.

**Acceptance Scenarios**:

1. **Given** un estado que solo contiene el `id`, **When** se ejecuta la lectura de cualquiera de
   los nueve recursos, **Then** ningún atributo obligatorio queda nulo y ningún atributo
   calculado queda desconocido.
2. **Given** un recurso que tras Create, Read y Update se representa a partir de la respuesta de
   la API, **When** se compara el estado resultante de las tres operaciones para el mismo objeto,
   **Then** es idéntico.
3. **Given** los recursos de reserva por prefijo, **When** se importan, **Then** la longitud de
   prefijo se deriva del CIDR y coincide con la de la configuración.

---

### User Story 3 - Red de seguridad que impide una tercera recurrencia (Priority: P2)

Un desarrollador añade un recurso nuevo o un atributo nuevo; si su lectura no rellena todos los
atributos tras un import, la suite de pruebas falla antes de mergear.

**Why this priority**: sin comprobación automática el fallo reaparecerá con el próximo recurso.

**Independent Test**: eliminar a propósito el relleno de un atributo en un recurso y comprobar que
la prueba genérica falla; un recurso no incluido en la tabla también hace fallar la suite.

**Acceptance Scenarios**:

1. **Given** la tabla de recursos de la prueba genérica, **When** un recurso deja un atributo
   obligatorio sin rellenar tras la lectura, **Then** la prueba falla indicando recurso y
   atributo.
2. **Given** un recurso registrado en el provider pero ausente de la tabla, **When** se ejecutan
   las pruebas, **Then** la suite falla pidiendo añadirlo.

---

### User Story 4 - Verificación real antes de cada release (Priority: P2)

Antes de publicar una versión, el mantenedor ejecuta las pruebas de aceptación contra un IPzilon
real y comprueba, por recurso, que import → plan da vacío.

**Why this priority**: las pruebas unitarias no detectan diferencias entre lo que la API devuelve
y lo que el estado necesita; la CI no dispone de API.

**Independent Test**: ejecutar `make testacc` con credenciales de un IPzilon de pruebas; cada
recurso pasa por crear → importar → plan vacío.

**Acceptance Scenarios**:

1. **Given** un IPzilon real accesible, **When** se ejecutan las pruebas de aceptación, **Then**
   para cada uno de los nueve recursos se verifica que el import seguido de plan no propone
   cambios.
2. **Given** una release pendiente, **When** el mantenedor sigue la lista de verificación de
   release, **Then** incluye este paso como requisito previo al tag.

---

### User Story 5 - IPs migradas con hostname igual a description (Priority: P3)

Una IP migrada desde Netbox tiene en la API `hostname` igual a `description` (id 2204). El
operador no debe ver diffs in situ inesperados tras el import.

**Why this priority**: es un posible segundo bloqueo de la migración, pero no destructivo.

**Independent Test**: importar la IP 2204 (o una equivalente) con `hostname` y `description`
configurados de forma distinta o solo uno de ellos, y revisar el `plan`.

**Acceptance Scenarios**:

1. **Given** una IP cuyo `hostname` coincide con su `description` en la API, **When** se importa
   y se planifica con la configuración esperada, **Then** el comportamiento queda determinado y
   documentado: o el plan es vacío, o el diff es explicable y se indica cómo evitarlo.

---

### User Story 6 - Release v3.0.1 (Priority: P1)

El mantenedor publica la v3.0.1 con la corrección para que `camaras` (y después `avd`) puedan
importar.

**Why this priority**: es el entregable que desbloquea la migración.

**Independent Test**: la versión publicada permite importar y planificar sin cambios en un stack
consumidor.

**Acceptance Scenarios**:

1. **Given** el cambio mergeado y las pruebas de aceptación en verde, **When** se publica el tag
   v3.0.1, **Then** el stack `camaras` importa sus IPs sin reemplazos en el plan.

### Edge Cases

- Import de un `id` que ya no existe: el recurso se elimina del estado / se informa el error
  (comportamiento actual de Read, sin regresión).
- CIDR devuelto por la API con formato no parseable: error claro en lugar de un estado con
  longitud de prefijo vacía.
- IP importada con `hostname` o `description` nulos en la API: el estado refleja nulo, sin
  convertirlos en cadena vacía.
- Update tras el que la API devuelve valores normalizados (p. ej. `hostname` distinto al enviado):
  el estado refleja lo devuelto por la API.
- Recurso nuevo registrado que no figura en la prueba genérica: la suite falla.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: Tras un `terraform import`, el estado de cada uno de los nueve recursos MUST contener
  todos los atributos del esquema con valores coherentes con la API, de modo que el `plan`
  siguiente no proponga cambios ni reemplazos.
- **FR-002**: Los nueve recursos MUST construir su estado exclusivamente a partir de la respuesta
  de la API mediante un patrón único (una función de conversión por recurso, usada en Create, Read y
  Update, más un helper común para los atributos derivados), sin combinarlo con el estado previo.
- **FR-003**: Los atributos que la API no devuelve (longitud de prefijo) MUST derivarse siempre
  del dato que sí devuelve (CIDR) en un único punto común; si la derivación falla, MUST
  informarse un error en lugar de dejar el atributo vacío.
- **FR-004**: `Update` MUST reflejar en el estado el objeto completo devuelto por la API tras la
  modificación, no una mezcla parcial con el estado anterior.
- **FR-005**: MUST existir una prueba automática genérica, dirigida por tabla, que para cada
  recurso parta de un estado con solo `id`, ejecute la lectura y falle si algún atributo
  obligatorio queda nulo o algún calculado desconocido, identificando recurso y atributo.
- **FR-006**: La suite MUST fallar si un recurso registrado en el provider no figura en la tabla
  de la prueba genérica.
- **FR-007**: Las pruebas de aceptación (`make testacc`) MUST verificar, para cada uno de los
  nueve recursos, el ciclo crear → importar → plan vacío contra un IPzilon real, y ese paso MUST
  constar como requisito previo a cada release.
- **FR-008**: Los data sources MUST revisarse y confirmarse que construyen sus resultados solo
  desde la respuesta de la API, sin depender de estado previo; cualquier excepción hallada MUST
  corregirse con el mismo mecanismo.
- **FR-009**: El comportamiento de importación de IPs cuyo `hostname` coincide con `description`
  MUST quedar determinado (sin diffs inesperados) o documentado con el procedimiento para
  evitarlos.
- **FR-010**: La corrección MUST NOT cambiar el esquema público (atributos, tipos, obligatoriedad)
  ni exponer datos de UI/monitorización (`alert_*`, `total_ips`, `used_ips`), por lo que MUST
  publicarse como versión PATCH (v3.0.1) sin *Breaking Changes*.
- **FR-011**: La documentación de contribución MUST indicar que todo recurso nuevo se construye con
  el mecanismo común y se añade a la tabla de la prueba genérica.

### Key Entities

- **Recurso gestionado**: cada uno de los nueve tipos (`hub`, `scope`, `network`, `subnet`,
  `ip_address`, `next_subnet`, `last_subnet`, `next_network`, `next_ip_address`) con sus atributos
  obligatorios, opcionales y calculados.
- **Estado tras import**: estado que contiene solo el `id` antes de la primera lectura; debe
  completarse íntegramente desde la API.
- **Registro de recursos de la prueba genérica**: tabla que enumera los recursos cubiertos y contra
  la que se compara la lista de recursos del provider.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: El 100 % de los nueve recursos (9 de 9) pasa import → plan vacío en las pruebas de
  aceptación previas a la release.
- **SC-002**: Importar las IPs de `camaras` (incl. id 2204) produce 0 reemplazos y 0 cambios en el
  `plan`.
- **SC-003**: Una regresión deliberada (dejar un atributo sin rellenar en cualquier recurso) hace
  fallar la suite de pruebas en el 100 % de los casos probados, sin necesidad de API real.
- **SC-004**: Añadir un recurso al provider sin registrarlo en la tabla de la prueba genérica hace
  fallar la suite.
- **SC-005**: La v3.0.1 se publica sin cambios de esquema y sin pasos de migración para los
  usuarios existentes (0 entradas nuevas en *Breaking Changes*).
- **SC-006**: Los stacks `camaras` y, después, `avd` completan su migración sin recurrir a
  parches manuales del estado.

## Assumptions

- La API de IPzilon 3.0.x devuelve en cada objeto todos los campos necesarios para reconstruir el
  estado, salvo la longitud de prefijo, derivable del CIDR.
- Se dispone de una instancia de IPzilon de pruebas y credenciales para `make testacc` antes de
  publicar (hoy la CI no tiene API).
- El prototipo local del brief ya no está disponible; se parte de `main` (`4fba8f5`).
- Ningún data source lee estado previo (verificado por revisión de código), por lo que no se
  espera cambio funcional en ellos.
- El problema de Terraform 1.16.0 con varios bloques `import {}` sobre `for_each` es ajeno al
  provider y queda fuera de alcance.
- Los nombres de recurso `hub`, `scope`, etc. corresponden a los tipos `ipzilon_*` publicados.
