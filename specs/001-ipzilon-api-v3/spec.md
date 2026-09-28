# Feature Specification: Adaptación del provider a IPzilon 3.0.0

**Feature Branch**: `001-ipzilon-api-v3`

**Created**: 2026-09-28

**Status**: Draft

**Input**: User description: "quiero que analices los cambios creados por el equipo de ipzilon en el informe /root/developer/github/ipzilon/specs/002-performance-scale-audit/contract-changes.md para adaptar el provider a ipzilon en esa nueva version que se generará"

**Documento de origen**: *Cambios de contrato de la API — IPzilon 3.0.0* (auditoría de rendimiento y escalabilidad, feature 002 de IPzilon). Las referencias `§N` de esta spec apuntan a sus apartados.

## Contexto

IPzilon publicará la versión mayor **3.0.0** en un único salto desde 2.3.1, sin convivencia de
versiones. La publicación queda bloqueada hasta que exista una versión del provider adaptada.
Verificado por el equipo de IPzilon con el provider actual (revisión `ae53b90`):

- **Siguen funcionando**: `ipzilon_hub`, `ipzilon_scope`, `ipzilon_network`, `ipzilon_subnet`,
  `ipzilon_next_subnet`, `ipzilon_last_subnet`, `ipzilon_next_network`,
  `ipzilon_next_ip_address`, lecturas por `id` e importación.
- **Dejan de funcionar**: todos los data sources de listado (los listados pasan a devolverse por
  páginas) y la creación de `ipzilon_ip_address` (las direcciones libres dejan de tener
  identificador propio).
- **Riesgo nuevo**: la API empieza a aplicar un límite de peticiones por token y a rechazar
  peticiones cuando está saturada; hoy el provider aborta el `apply` ante esas respuestas.

## Clarifications

### Session 2026-09-28

- Q: ¿`status` obligatorio en `ipzilon_ip_addresses` al listar por `subnet_id`? → A: No; se
  mantiene opcional y se documenta el coste de listar la subred completa (FR-007).
- Q: ¿Qué versiones de IPzilon soporta el provider adaptado? → A: Solo IPzilon ≥ 3.0.0; se publica
  como versión MAJOR del provider (FR-016).
- Q (resuelta por la constitución): ¿dejar de forzar `force: true` en cambios de CIDR de subred?
  → A: No; se mantiene por el Principio II (FR-015).
- Q: ¿Cómo se libera una dirección gestionada por Terraform? → A: Borrando su registro en
  IPzilon (en 3.0 borrar = liberar), no marcándola como `available` (FR-008).
- Q: ¿Admite `status = "available"` en `ipzilon_ip_address` / `ipzilon_next_ip_address`? → A:
  No; solo `used` o `reserved`. Una IP que se quiere bloquear se declara `reserved`; liberar es
  destruir el recurso (FR-020).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Consultar inventario con los data sources de listado (Priority: P1)

Como operador de infraestructura que usa Terraform, quiero que `ipzilon_sites`, `ipzilon_hubs`,
`ipzilon_scopes`, `ipzilon_networks`, `ipzilon_subnets` e `ipzilon_ip_addresses` sigan devolviendo
**todos** los elementos que cumplen el filtro contra IPzilon 3.0.0, aunque la API los entregue por
páginas, para que mis configuraciones que dependen de ellos no fallen ni devuelvan resultados
incompletos.

**Why this priority**: hoy fallan todos los data sources de listado contra 3.0.0 (p. ej.
`List hubs failed`); es el fallo con mayor alcance y bloquea cualquier configuración que los use.

**Independent Test**: contra una instancia 3.0.0, ejecutar `terraform plan` con cada data source de
listado (con y sin filtros) sobre un inventario con más elementos que el tamaño máximo de página y
comparar el número y contenido de los elementos con el inventario real.

**Acceptance Scenarios**:

1. **Given** un hub con 121 redes, **When** se lee `ipzilon_networks` por `hub_id`, **Then** se
   obtienen las 121 redes en el mismo orden que devuelve la API.
2. **Given** una subred con más de 1000 direcciones, **When** se lee `ipzilon_ip_addresses` por
   `subnet_id`, **Then** se obtienen todas las direcciones, sin duplicados ni huecos.
3. **Given** filtros de servidor (`name`, `cidr`, `address_space`, …), **When** se lee un data
   source filtrado, **Then** el resultado contiene solo los elementos filtrados y no se descargan
   páginas innecesarias.
4. **Given** un listado vacío, **When** se lee el data source, **Then** se devuelve una lista vacía
   sin error.

---

### User Story 2 - Gestionar una dirección IP concreta con `ipzilon_ip_address` (Priority: P1)

Como operador, quiero crear, modificar, liberar e importar una dirección IP concreta con
`ipzilon_ip_address` contra IPzilon 3.0.0, y recibir un error claro si la dirección ya está
ocupada o no pertenece a la subred.

**Why this priority**: la creación falla hoy contra 3.0.0 (`Lookup IP failed`), por lo que el recurso
queda inutilizable para altas nuevas.

**Independent Test**: contra una instancia 3.0.0, aplicar una configuración que ocupe una dirección
libre, modificar su `hostname`, destruirla y comprobar en IPzilon que vuelve a figurar como libre.

**Acceptance Scenarios**:

1. **Given** una dirección libre de una subred, **When** se aplica un `ipzilon_ip_address` sobre
   ella, **Then** la dirección queda ocupada con el estado y las anotaciones indicados y el recurso
   guarda el identificador asignado por IPzilon.
2. **Given** una dirección ya ocupada, reservada o anotada, **When** se intenta crear el recurso
   sobre ella, **Then** el `apply` falla con un mensaje que indica que la dirección ya está en uso.
3. **Given** una dirección fuera del rango de la subred, **When** se intenta crear el recurso,
   **Then** el `apply` falla con un mensaje que indica que la dirección no pertenece a la subred.
4. **Given** un recurso creado, **When** se modifica `hostname` o `description`, **Then** el cambio
   se aplica in situ sin reemplazo y el siguiente `plan` no muestra cambios.
5. **Given** un recurso creado, **When** se destruye, **Then** la dirección vuelve a figurar como
   libre en IPzilon y el `destroy` termina sin error.
6. **Given** una dirección liberada fuera de Terraform, **When** se ejecuta `plan`, **Then** el
   recurso desaparece del estado y Terraform propone volver a crearlo.
7. **Given** un estado creado con el provider anterior contra IPzilon 2.3.1, **When** IPzilon se
   migra a 3.0.0 y se ejecuta `plan` con el provider adaptado, **Then** no aparecen cambios.
8. **Given** un `ipzilon_ip_address` o `ipzilon_next_ip_address` con `status = "available"`,
   **When** se ejecuta `plan`, **Then** falla indicando que solo se admiten `used` o `reserved` y
   que para liberar la dirección hay que destruir el recurso.

---

### User Story 3 - Ejecuciones resilientes ante límite de peticiones y saturación (Priority: P2)

Como operador que lanza `apply`, `plan` o `destroy` de cientos de recursos en paralelo, quiero que
el provider espere y reintente automáticamente cuando IPzilon rechace temporalmente peticiones por
límite de uso o por saturación, para que la ejecución termine bien sin intervención manual.

**Why this priority**: no rompe ningún flujo actual con los valores por defecto de la API
(20.000 peticiones/minuto por token), pero en entornos con límites más bajos o picos de carga
aborta ejecuciones que hoy funcionan.

**Independent Test**: contra una instancia 3.0.0 configurada con un límite bajo (p. ej. 60
peticiones/minuto), aplicar una configuración de ≥ 200 recursos con `-parallelism=10` y comprobar
que termina correctamente.

**Acceptance Scenarios**:

1. **Given** una API que responde "límite superado" indicando un tiempo de espera, **When** el
   provider recibe esa respuesta, **Then** espera al menos ese tiempo y reintenta la petición.
2. **Given** una API que responde "servidor ocupado", **When** el provider recibe esa respuesta,
   **Then** reintenta con espera creciente.
3. **Given** que el rechazo persiste tras el número máximo de reintentos, **When** se agotan,
   **Then** la operación falla con un mensaje que explica que la API sigue limitando o saturada y
   cuántas veces se ha reintentado.
4. **Given** cualquier otro error de la API (4xx de validación, 404, 409), **When** se recibe,
   **Then** no se reintenta y se muestra inmediatamente.

---

### User Story 4 - Mensajes claros cuando la búsqueda de un bloque libre se trunca (Priority: P3)

Como operador que usa `ipzilon_next_subnet`, `ipzilon_last_subnet` o `ipzilon_next_network`, quiero
distinguir entre "no queda espacio" y "el espacio está tan fragmentado que la búsqueda se ha
detenido", con la sugerencia de pedir un CIDR concreto, para saber cómo resolverlo.

**Why this priority**: es un caso límite en espacios muy fragmentados; hoy se mostraría el error
crudo de la API, que ya es comprensible pero no orienta la acción.

**Independent Test**: provocar una búsqueda truncada en un espacio fragmentado y comprobar el
mensaje mostrado por `apply`.

**Acceptance Scenarios**:

1. **Given** un padre con espacio muy fragmentado, **When** se crea un recurso de asignación
   dinámica y la búsqueda se trunca, **Then** el error indica que el espacio está fragmentado y
   sugiere usar un recurso con CIDR explícito (`ipzilon_subnet` / `ipzilon_network`).
2. **Given** un padre sin espacio libre, **When** se crea el recurso, **Then** el error sigue
   indicando falta de espacio, diferenciado del caso anterior.

---

### User Story 5 - Detectar en `plan` CIDR que la API rechazará (Priority: P3)

Como operador, quiero que `terraform plan` avise si declaro un hub, scope, red o subred mayor que
/8, o una subred IPv6, para no descubrirlo a mitad de `apply`.

**Why this priority**: la API ya los rechaza; es una mejora de experiencia permitida por el
Principio I (validaciones de forma).

**Independent Test**: `terraform validate`/`plan` con un `ipzilon_network` de `/7` y un
`ipzilon_subnet` IPv6, sin necesidad de API.

**Acceptance Scenarios**:

1. **Given** un `cidr`/`address_space` con prefijo menor que 8, **When** se ejecuta `plan`,
   **Then** falla con un mensaje que indica que el tamaño máximo es /8.
2. **Given** un `ipzilon_subnet` con CIDR IPv6, **When** se ejecuta `plan`, **Then** falla indicando
   que las subredes IPv6 no están soportadas.
3. **Given** un objeto existente mayor que /8 importado al estado, **When** se ejecuta `plan` sin
   cambiar su CIDR, **Then** no falla (la API conserva los objetos existentes).

---

### Edge Cases

- **Listado que cambia durante la paginación**: si el inventario cambia entre páginas, el
  resultado puede ser inconsistente; se acepta (la API no ofrece instantáneas) y la siguiente
  lectura se corrige sola. El provider nunca debe entrar en un bucle infinito: debe detenerse al
  recibir una página vacía o al cubrir el total anunciado.
- **`ipzilon_ip_addresses` sin `status` sobre una subred grande**: una /16 son 65.536 elementos;
  ver FR-007.
- **Direcciones libres sin identificador**: en `ipzilon_ip_addresses` las libres no tienen `id`;
  nunca debe mostrarse un `0` u otro valor inventado.
- **Consulta de `ipzilon_ip_addresses` por `id` de una dirección liberada**: la API responde "no
  encontrado"; el data source debe fallar con un mensaje claro (no hay estado que limpiar).
- **Liberación que devuelve un elemento sin identificador**: al destruir, el provider no debe
  intentar leer ni interpretar el identificador de la respuesta.
- **`status = "available"` en recursos de dirección**: en 3.0 una dirección libre sin anotaciones
  no tiene registro; aceptarlo provocaría un 400 al crear o, al modificar, la liberación de la IP
  y un bucle de recreación. Se rechaza en `plan` (FR-020).
- **Estados existentes con `status = "available"`**: tras actualizar el provider, el `plan` falla
  hasta que el usuario cambie el valor o destruya el recurso; se documenta en *Breaking Changes*.
- **Reintentos durante `destroy`**: se aplican igual que en `apply`/`plan`.
- **Espera indicada por la API muy larga**: la espera por reintento debe estar acotada para que
  una ejecución no quede bloqueada indefinidamente.
- **Petición no idempotente reintentada**: un rechazo por límite o saturación implica que la API
  no ha procesado la petición, por lo que reintentar altas es seguro.
- **Dos altas simultáneas de la misma subred**: la segunda recibe un conflicto; se muestra el
  error de la API sin reintentar.

## Requirements *(mandatory)*

### Functional Requirements

**Paginación (§2)**

- **FR-001**: Todos los data sources de listado MUST devolver la totalidad de los elementos que
  cumplen los filtros, recorriendo todas las páginas que ofrezca la API.
- **FR-002**: El provider MUST solicitar el tamaño de página máximo admitido (1000) para minimizar
  el número de peticiones y el consumo de cupo.
- **FR-003**: El recorrido de páginas MUST terminar al cubrir el total anunciado o al recibir una
  página vacía, y MUST conservar el orden devuelto por la API.
- **FR-004**: Los filtros de servidor existentes MUST seguir aplicándose en el servidor
  (Principio I), combinados con la paginación.

**Direcciones IP (§3)**

- **FR-005**: `ipzilon_ip_address` MUST ocupar la dirección indicada en una única operación de
  alta, enviando el estado (por defecto `used`) y las anotaciones configuradas, y MUST guardar el
  identificador que devuelva IPzilon.
- **FR-006**: `ipzilon_ip_address` MUST traducir el conflicto de alta en el mensaje "la dirección
  X ya está en uso" y el rechazo por rango en "la dirección X no pertenece a la subred Y".
- **FR-007**: `ipzilon_ip_addresses` MUST representar las direcciones libres con `id` nulo.
  El filtro `status` MUST seguir siendo opcional al listar por `subnet_id`; la documentación del
  data source MUST advertir que, sin `status`, se devuelve la subred completa (p. ej. 65.536
  elementos en una /16) con el consiguiente coste en tiempo y en cupo de peticiones.
- **FR-008**: Leer, modificar e importar `ipzilon_ip_address` e `ipzilon_next_ip_address` MUST
  mantener su comportamiento actual; si la dirección gestionada ya no existe, el recurso MUST
  eliminarse del estado. La baja de ambos recursos MUST liberar la dirección borrando su registro
  en IPzilon (no marcándola como `available`); un "no encontrado" al liberar se ignora.
- **FR-020**: El atributo `status` de `ipzilon_ip_address` e `ipzilon_next_ip_address` MUST
  aceptar solo `used` o `reserved` y rechazar cualquier otro valor en `plan`. Es un cambio
  incompatible que MUST documentarse en *Breaking Changes* (Principio III).
- **FR-009**: Los estados de Terraform existentes MUST seguir siendo válidos tras migrar IPzilon a
  3.0.0, sin reimportar ni ejecutar `terraform state` manualmente.

**Resiliencia (§5)**

- **FR-010**: Todas las peticiones a la API MUST reintentarse automáticamente ante "límite de
  peticiones superado" y "servidor ocupado", respetando el tiempo de espera que indique la API y,
  en su defecto, con espera creciente.
- **FR-011**: Los reintentos MUST estar acotados en número de intentos y en espera máxima; al
  agotarse, el error MUST indicar el motivo y el número de reintentos realizados.
- **FR-012**: Ninguna otra respuesta de error MUST reintentarse.

**Asignación dinámica y validaciones (§6, §7)**

- **FR-013**: `ipzilon_next_subnet`, `ipzilon_last_subnet` e `ipzilon_next_network` MUST mostrar
  un mensaje específico cuando la búsqueda de bloque libre se trunque, sugiriendo declarar un CIDR
  concreto.
- **FR-014**: Los recursos `ipzilon_hub`, `ipzilon_scope`, `ipzilon_network` e `ipzilon_subnet`
  MUST rechazar en `plan` prefijos mayores que /8, y `ipzilon_subnet` MUST rechazar CIDR IPv6,
  sin impedir la gestión de objetos existentes cuyo CIDR no cambie.
- **FR-015**: Los cambios de CIDR de subred MUST seguir enviando la confirmación forzada
  (Principio II); la documentación del recurso MUST advertir que las direcciones guardadas que
  queden fuera del nuevo rango se liberan.

**Compatibilidad, versión y documentación**

- **FR-016**: La versión adaptada MUST dar soporte únicamente a IPzilon ≥ 3.0.0; no se mantiene
  compatibilidad con 2.x. Al dejar de funcionar contra instancias 2.x, MUST publicarse como nueva
  versión MAJOR del provider (`v3.0.0`, desde `v2.2.1`) y coordinarse con la publicación de
  IPzilon 3.0.0. Contra una API 2.x, el provider MUST fallar con un mensaje que indique la
  versión mínima de IPzilon requerida, en lugar de un error de formato genérico.
- **FR-017**: El `README.md` MUST documentar la matriz de compatibilidad provider ↔ IPzilon, el
  cambio de `id` nulo en `ipzilon_ip_addresses`, el comportamiento de reintentos y una sección
  *Breaking Changes* con el fin del soporte de IPzilon 2.x y los pasos de migración (actualizar
  IPzilon a 3.0.0 antes o a la vez que el provider) (Principio III).
- **FR-018**: La documentación de `docs/` MUST regenerarse y los ejemplos actualizarse en el mismo
  cambio (Principio IV).
- **FR-019**: La lógica de paginación, reintentos y traducción de errores MUST cubrirse con tests
  unitarios que no dependan de una API en vivo (Principio V).

### Key Entities

- **Página de listado**: conjunto parcial de elementos más el total de elementos que cumplen el
  filtro; el provider la consume de forma transparente.
- **Dirección IP**: identificada por `(subred, dirección)`. Solo tiene identificador propio si
  está ocupada, reservada o anotada; al liberarse lo pierde y, si se vuelve a ocupar, recibe uno
  distinto.
- **Rechazo temporal**: respuesta de la API por límite de peticiones o saturación, con tiempo de
  espera sugerido; se resuelve reintentando.
- **Búsqueda truncada**: resultado de una asignación dinámica que se detiene por fragmentación sin
  concluir que no haya espacio.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: El 100 % de los recursos y data sources del provider funcionan contra IPzilon 3.0.0,
  incluidos los dos casos hoy fallidos (`ipzilon_ip_address` al crear y data sources de listado).
- **SC-002**: Los data sources de listado devuelven exactamente el mismo número de elementos que
  el inventario real con más de 1000 elementos (0 perdidos, 0 duplicados).
- **SC-003**: Un `apply` de ≥ 200 recursos con `-parallelism=10` contra una API limitada a 60
  peticiones/minuto termina correctamente sin intervención manual.
- **SC-004**: Un `plan` sobre un estado creado contra 2.3.1 muestra 0 cambios tras migrar IPzilon
  a 3.0.0.
- **SC-005**: Un `apply` consume ≤ 2 peticiones de media por recurso gestionado, sin contar
  reintentos (margen medido por IPzilon), de modo que los listados no multiplican el consumo de cupo.
- **SC-006**: El equipo de IPzilon recibe la confirmación de SC-011 de su informe y una fecha
  estimada de publicación del provider adaptado, desbloqueando la publicación de 3.0.0.

## Assumptions

- La API 3.0.0 se comporta exactamente como describe el documento de cambios de contrato; si
  algún punto difiere, se actualiza esta spec.
- Los listados por padre caben en una página en los inventarios medidos, pero el provider pagina
  igualmente todos los listados para no depender de ello.
- Valores de reintento por defecto: se reintenta mientras la espera acumulada por petición no
  supere 10 minutos (con un máximo de 30 reintentos como red de seguridad), y cada espera se
  acota a 60 s; no se expone como configuración del provider en esta versión.
- Los endpoints de listado no consumidos por el provider (usuarios, tokens, alertas, dashboard,
  consumidores de contenedor, `/cidrs/`) y la edición/borrado masivos de direcciones quedan fuera
  de alcance.
- Se mantiene `force: true` en los cambios de CIDR de subred por mandato del Principio II de la
  constitución; cambiarlo requeriría enmendarla.
- No se exponen los nuevos campos de fecha nulos de las direcciones libres, coherente con el
  esquema actual.
- Hasta publicar el provider adaptado, los usuarios deben fijar la imagen de IPzilon a `2.3.1` y no
  usar `:latest`; quienes sigan en IPzilon 2.x deben fijar el provider a `~> 2.2`. Ambas cosas se
  comunican en el `README.md`.
- Las versiones fijadas en ejemplos y en `docs/index.md` pasan a `~> 3.0` (Principio IV).
- Las pruebas de aceptación requieren una instancia real de IPzilon 3.0.0 (y 2.3.1 para SC-004),
  proporcionada por el equipo de IPzilon.
