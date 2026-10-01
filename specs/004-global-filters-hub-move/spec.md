# Feature Specification: Filtros sin ids, lista vacía y mover hubs de site (IPzilon 3.2.0)

**Feature Branch**: `004-global-filters-hub-move`

**Created**: 2026-10-01

**Status**: Draft

**Input**: User description: "el equipo de desarollo de Ipzilon ha lanzado una version nueva que contiene algunas funcionalidades que hay que implementar en el provider. la primera parte de la Issue #16 y los cambios que han implementado lo han dejado comentado en la propia Issue (esta Issue hay que implementarla al 100% ademas de lo que viene de Ipzilon). Ademas han añadido otra funcionalidad que es para poder mover hubs entre sites sin cambios de id's y evitar recreaciones en terraform (esto está documentado en la Issue #21 que tambien hay que implementar)"

**Documentos de origen**:

- Issue #16 *[Feat] - Filtros sin ids y lista vacía en data sources* (puntos 1, 2 y 3 y sus
  criterios de aceptación) y su comentario con el resumen de IPzilon 3.2.0.
- *Cambios de contrato de la API — IPzilon 3.2.0 (listados globales filtrables)*, feature 005 de
  IPzilon (`ipzilon/specs/005-global-list-filters/contract-changes.md`). Las referencias `§N` sin
  más indicación apuntan a este documento.
- Issue #21 *[Feat] - Mover hub de site sin recrear* y *Cambios de contrato de la API — mover un
  Hub de Site*, feature 006 de IPzilon (`ipzilon/specs/006-hub-site-move/contract-changes.md`),
  citado como `006 §N`.

## Contexto

Hoy los data sources de listado obligan a conocer ids para localizar un elemento y devuelven una
lista nula cuando no hay resultados, lo que rompe las expresiones `for` sobre `items`:

| Data source | Filtros sin ids | Exige id de |
|---|---|---|
| `ipzilon_sites` | `name` | — |
| `ipzilon_hubs` | `name`, `address_space` | `site_id` |
| `ipzilon_scopes` | `name`, `cidr`, `kind` | `hub_id` |
| `ipzilon_networks` | `name`, `cidr` | `hub_id` o `scope_id` |
| `ipzilon_subnets` | — | `network_id` o `zone_id` |
| `ipzilon_ip_addresses` | `status` | `subnet_id` |
| `ipzilon_network_zones` | `name`, `cidr` | — (resuelto en v3.1.0) |

Desde IPzilon 3.0.0 el id de una IP tampoco es estable (al liberarla desaparece su registro): lo
estable es la pareja subred + dirección. En Terraform lo natural es localizar elementos por sus
propiedades (nombre, CIDR, dirección), no por ids.

IPzilon **3.2.0** (publicada el 2026-10-01 e incluye las specs 005 y 006) añade, de forma aditiva:

- **Listados globales filtrables** de hubs, scopes, networks y subredes, sin id del padre y con el
  filtrado resuelto en el servidor (§2–§5). Sin resultados devuelven una lista vacía. Los filtros
  de CIDR comparan la **red**, no el texto. Contra 3.1.x esas rutas no existen (§7).
- **Mover un hub de site** conservando su id y el de todo lo que cuelga de él (006 §3). Contra
  una versión anterior la petición se acepta pero el hub **no se mueve**, sin error (006 §4).

Los puntos 1 y 2 de la issue #16 (lista vacía y búsqueda de una IP por dirección) no necesitan
cambios en IPzilon (§8).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Recorrer listados vacíos sin errores (Priority: P1)

Como operador que usa Terraform, quiero que cualquier data source de listado devuelva una lista
vacía cuando no hay coincidencias, para poder recorrer sus elementos con `for`, `length` o
`one()` sin protegerme contra valores nulos.

**Why this priority**: hoy una búsqueda sin resultados rompe la configuración en vez de dar una
lista vacía; es un fallo de uso diario, no depende de IPzilon 3.2.0 y es el cambio más pequeño.

**Independent Test**: contra cualquier IPzilon soportado, consultar `ipzilon_sites`,
`ipzilon_hubs`, `ipzilon_scopes`, `ipzilon_networks` e `ipzilon_ip_addresses` con un filtro sin
coincidencias y usar el resultado en una expresión `for` y en `length`.

**Acceptance Scenarios**:

1. **Given** un filtro sin coincidencias en cualquiera de los cinco data sources, **When** se
   consulta, **Then** `items` es una lista vacía (longitud 0), no nula, y no hay error.
2. **Given** una expresión `for` sobre los `items` de un listado vacío, **When** se ejecuta
   `plan`, **Then** produce una colección vacía sin error.
3. **Given** un filtro con coincidencias, **When** se consulta, **Then** los resultados son los
   mismos que con la versión anterior del provider.

---

### User Story 2 - Localizar una IP por su dirección (Priority: P1)

Como operador, quiero obtener una IP de una subred indicando su dirección, esté ocupada o libre,
para referenciarla de forma estable aunque su id cambie al liberarla y volver a ocuparla.

**Why this priority**: el id de una IP no es estable desde IPzilon 3.0.0; la dirección es la
única referencia fiable. No depende de IPzilon 3.2.0.

**Independent Test**: con una subred que tiene una IP ocupada, consultar el data source de IPs por
la dirección ocupada, por una libre, por una de fuera de la subred y con combinaciones de filtros
no permitidas.

**Acceptance Scenarios**:

1. **Given** la IP `10.0.1.17` ocupada en la subred 42, **When** se consulta con esa subred y esa
   dirección, **Then** se obtiene un único elemento con su id, su dirección y su estado.
2. **Given** la IP `10.0.1.20` libre en la subred 42, **When** se consulta por esa dirección,
   **Then** se obtiene un único elemento sin id y con estado `available`.
3. **Given** una dirección fuera de la subred indicada, **When** se consulta, **Then** la lectura
   falla con un mensaje claro que indica que la dirección no pertenece a la subred.
4. **Given** una dirección sin subred, o combinada con el filtro por id o por estado, **When** se
   ejecuta `plan`, **Then** falla con un mensaje que explica la combinación no permitida.
5. **Given** un valor que no es una dirección IP, **When** se ejecuta `plan`, **Then** falla con
   un mensaje de formato sin llamar a IPzilon.

---

### User Story 3 - Localizar hubs, scopes, networks y subredes sin conocer ids (Priority: P1)

Como operador, quiero localizar un hub, un scope, una network o una subred solo por sus
propiedades (nombre, CIDR, espacio de direcciones, tipo de scope), sin escribir el id de su padre,
para referenciar infraestructura creada por otros equipos o desde la interfaz de IPzilon.

**Why this priority**: es el objetivo principal de la issue #16 y la novedad central de IPzilon
3.2.0; evita cadenas de data sources solo para averiguar ids intermedios.

**Independent Test**: contra IPzilon 3.2.0, con un inventario con varios sites, hubs, scopes,
networks y subredes (incluido un CIDR repetido en dos hubs), consultar cada data source solo por
nombre, solo por CIDR, por ambos y combinando con ids de padre, y comparar con el inventario.

**Acceptance Scenarios**:

1. **Given** una network `10.0.16.0/22`, **When** se consulta el data source de networks solo por
   ese CIDR, **Then** se obtiene esa network con todos sus atributos, sin indicar hub ni scope.
2. **Given** un hub con espacio de direcciones `10.0.0.0/16`, **When** se consulta el data source
   de hubs solo por ese espacio, **Then** se obtiene ese hub con su site.
3. **Given** un scope de tipo `project` llamado `avd`, **When** se consulta por tipo y nombre,
   **Then** se obtiene ese scope con su hub y su scope padre.
4. **Given** una subred `10.0.16.64/26`, **When** se consulta el data source de subredes solo por
   CIDR o solo por nombre, **Then** se obtiene esa subred con su network y su zona.
5. **Given** un mismo CIDR en dos hubs distintos, **When** se consulta solo por CIDR, **Then** se
   obtienen los dos elementos; **When** se añade el id del padre, **Then** se obtiene solo uno.
6. **Given** un filtro de scopes por scope padre sin indicar hub, **When** se consulta, **Then**
   se obtienen solo sus hijos directos.
7. **Given** un filtro de subredes por zona, **When** se consulta, **Then** se obtienen las
   subredes contenidas en la zona sin que el provider tenga que resolver antes su network.
8. **Given** un CIDR mal formado o con bits de host (p. ej. `10.0.16.5/22`) en una búsqueda sin
   id de padre, o un tipo de scope no permitido, **When** se ejecuta `plan`, **Then** falla con
   un mensaje de formato que, para los bits de host, sugiere la red correcta (`10.0.16.0/22`).
9. **Given** un id de padre inexistente, **When** se consulta, **Then** la lectura falla con el
   mensaje de IPzilon (`<Entidad> not found`).
10. **Given** una búsqueda sin id de padre contra IPzilon 3.1.x, **When** se consulta, **Then**
    la lectura falla con un mensaje que indica que esa búsqueda necesita IPzilon ≥ 3.2.0.
11. **Given** una configuración que ya usa ids de padre, **When** se actualiza el provider,
    **Then** devuelve los mismos resultados, también contra IPzilon 3.1.x.

---

### User Story 4 - Mover un hub de site sin recrearlo (Priority: P2)

Como operador, quiero cambiar el site de un hub gestionado y que se mueva in situ, conservando su
id y el de todo lo que cuelga de él, para consolidar sites sin destruir ni recrear hubs, scopes,
networks ni subredes.

**Why this priority**: hoy cambiar el site obliga a recrear el hub, lo que en la práctica es
inviable porque arrastra toda su jerarquía; aun así la issue #21 lo marca como prioridad baja
porque ningún stack gestiona hoy `ipzilon_hub`.

**Independent Test**: contra IPzilon 3.2.0, con un hub gestionado con scopes, networks y subredes
gestionados, cambiar su site en la configuración, aplicar y comprobar que el hub conserva su id,
que ningún recurso dependiente cambia y que el siguiente `plan` no muestra cambios.

**Acceptance Scenarios**:

1. **Given** un hub gestionado en el site A, **When** se cambia su site a B, **Then** el `plan`
   anuncia una actualización in situ (no una recreación) y, tras el `apply`, el hub está en B con
   el mismo id.
2. **Given** scopes, networks y subredes gestionados bajo ese hub, **When** se mueve el hub,
   **Then** ninguno de ellos muestra cambios en el `plan` ni en el `apply`.
3. **Given** un site de destino inexistente, de otro tipo, con un hub del mismo nombre o con un
   espacio de direcciones solapado, **When** se aplica, **Then** el `apply` falla mostrando el
   mensaje original de IPzilon y el hub sigue en su site.
4. **Given** una instancia de IPzilon anterior a la que permite mover hubs, **When** se aplica un
   cambio de site, **Then** el `apply` falla con un mensaje que indica la versión mínima de
   IPzilon necesaria, sin dejar un diff perpetuo.
5. **Given** un cambio solo de nombre del hub, **When** se aplica, **Then** sigue actualizándose
   in situ como hasta ahora.

---

### Edge Cases

- **CIDR repetido entre hubs o sites** (espacios solapados): una búsqueda solo por CIDR puede
  devolver varios elementos. No es un error: el data source devuelve todos y la configuración del
  usuario decide (coherente con `ipzilon_network_zones`).
- **Objetos guardados con bits de host** (p. ej. una network `10.0.16.5/22`): la búsqueda global
  por `10.0.16.0/22` los encuentra porque compara la red. Las búsquedas con id de padre conservan
  su comparación actual por texto.
- **`root_only` en scopes**: el listado global no lo admite; usarlo sin hub debe fallar en `plan`
  con un mensaje claro.
- **Subredes por zona y network a la vez** que no se corresponden: la lectura falla con el
  mensaje de IPzilon (`Zone <z> does not belong to network <n>`).
- **Subredes con `no_zone`** sin network: sigue exigiendo network, como en v3.1.0, porque el
  listado global no tiene ese filtro.
- **Filtro por tipo de scope**: con hub sigue funcionando igual que hoy; sin hub se resuelve en
  IPzilon.
- **Valores desconocidos en `plan`** (ids o CIDR que dependen de recursos aún no creados): la
  validación de forma se omite y se hace en la lectura.
- **IPzilon con versión desconocida o de desarrollo**: no se bloquea, como en las comprobaciones
  de versión existentes; si la ruta global no existe, el error de la API (`405`) se traduce al
  mensaje de versión mínima.
- **IP fuera de la subred y dirección mal formada**: la API devuelve lista vacía en ambos casos;
  el provider debe convertirlo en error, no en lista vacía.
- **IPv6**: una dirección IPv6 en `address` es un formato válido; un CIDR IPv6 en subredes
  simplemente no coincide.
- **Mover un hub y cambiar a la vez su nombre o espacio de direcciones**: se aplican en la misma
  actualización y IPzilon valida el resultado final en el site de destino.
- **Hub movido fuera de Terraform**: el siguiente `plan` muestra el cambio de site como
  actualización in situ para volver al site configurado, nunca como recreación.
- **Listados grandes**: los listados globales se paginan y deben obtenerse completos, en el orden
  de la API.

## Requirements *(mandatory)*

### Functional Requirements

**Lista vacía (issue #16, punto 1; §8)**

- **FR-001**: `ipzilon_sites`, `ipzilon_hubs`, `ipzilon_scopes`, `ipzilon_networks` e
  `ipzilon_ip_addresses` DEBEN devolver `items` como lista vacía, nunca nula, cuando no haya
  resultados, igual que `ipzilon_subnets` e `ipzilon_network_zones`.

**IP por dirección (issue #16, punto 2; §8)**

- **FR-002**: `ipzilon_ip_addresses` DEBE admitir un filtro `address` que devuelva la IP de esa
  dirección en la subred indicada: ocupada, reservada o anotada con su id y estado; libre como un
  único elemento sin id y con estado `available`.
- **FR-003**: `address` DEBE exigir `subnet_id` y ser incompatible con el filtro por id y con el
  filtro por estado; cualquier combinación no permitida DEBE fallar en `plan`.
- **FR-004**: El formato de `address` DEBE validarse en `plan` como dirección IP.
- **FR-005**: Si IPzilon no devuelve ningún elemento para la dirección pedida (fuera de la subred
  o mal formada), la lectura DEBE fallar con un mensaje que nombre la dirección y la subred.

**Filtros sin ids del padre (issue #16, punto 3; §2–§7)**

- **FR-006**: `ipzilon_hubs` DEBE permitir buscar por nombre y/o espacio de direcciones sin
  indicar site; el site pasa a ser un filtro opcional combinable.
- **FR-007**: `ipzilon_scopes` DEBE permitir buscar por nombre, CIDR y/o tipo sin indicar hub; hub
  y scope padre pasan a ser filtros opcionales combinables.
- **FR-008**: `ipzilon_networks` DEBE permitir buscar por nombre y/o CIDR sin indicar hub ni
  scope; hub y scope pasan a ser filtros opcionales combinables.
- **FR-009**: `ipzilon_subnets` DEBE admitir filtros nuevos por nombre y por CIDR, y permitir
  buscar por ellos, y por zona, sin indicar network.
- **FR-010**: Las búsquedas sin id de padre DEBEN resolverse en IPzilon, sin recorrer el
  inventario en el provider (Principio I), y DEBEN devolver todas las coincidencias, completas
  aunque la API pagine y en el orden de la API.
- **FR-011**: Los elementos devueltos por una búsqueda sin id de padre DEBEN tener los mismos
  atributos que los de la búsqueda con id de padre equivalente.
- **FR-012**: Los filtros de CIDR y espacio de direcciones de las búsquedas sin id de padre DEBEN
  validarse en `plan` como CIDR sin bits de host, sugiriendo la red correcta; el tipo de scope
  DEBE validarse como `landing_zone` o `project`.
- **FR-013**: Las combinaciones que el listado global no admite (`root_only` sin hub, "sin zona"
  sin network, "sin zona" con nombre o CIDR) DEBEN fallar en `plan` con un mensaje claro.
- **FR-014**: Un id de padre inexistente y cualquier otro error de IPzilon DEBEN mostrarse con el
  mensaje original de la API.
- **FR-015**: Las búsquedas sin id de padre contra IPzilon anterior a 3.2.0 DEBEN fallar con un
  mensaje que indique que requieren IPzilon ≥ 3.2.0; la versión NO DEBE exigirse cuando se usen
  solo los filtros que ya existían con id de padre.
- **FR-016**: Los filtros actuales con id de padre DEBEN seguir funcionando y devolviendo los
  mismos resultados, también contra IPzilon 3.1.x.

**Mover un hub de site (issue #21; 006 §3, §4)**

- **FR-017**: Cambiar el site de `ipzilon_hub` DEBE aplicarse como actualización in situ,
  conservando el id del hub, sin forzar su recreación ni la de ningún recurso dependiente.
- **FR-018**: Si IPzilon rechaza el movimiento (site inexistente, de otro tipo, nombre repetido o
  espacio de direcciones solapado en el destino), el `apply` DEBE fallar con el mensaje original
  de la API y el estado DEBE seguir reflejando el site real.
- **FR-019**: Contra una versión de IPzilon que no permite mover hubs, un cambio de site DEBE
  fallar con un mensaje que indique la versión mínima; el provider DEBE además comprobar tras la
  actualización que el site devuelto es el pedido y fallar si no lo es, para que nunca quede un
  diff perpetuo ni un movimiento silenciosamente ignorado.
- **FR-020**: Los cambios de nombre, espacio de direcciones, ubicación o descripción de un hub
  DEBEN seguir comportándose como hasta ahora.

**Compatibilidad, pruebas y documentación**

- **FR-021**: Todos los cambios DEBEN ser aditivos (ningún recurso, atributo o comportamiento
  existente se renombra, elimina o pasa a obligatorio), de modo que puedan publicarse en una
  versión MINOR del provider sin migración de estado.
- **FR-022**: La construcción de las búsquedas, la validación de filtros y la conversión de
  resultados vacíos DEBEN cubrirse con tests unitarios que no dependan de una API en vivo.
- **FR-023**: Cada data source y recurso modificado DEBE tener descripción en sus atributos nuevos
  o modificados, ejemplos actualizados (incluida una búsqueda sin ids y un movimiento de hub) y
  documentación regenerada.

### Key Entities

- **Hub**: pertenece a un site; se localiza por nombre y/o espacio de direcciones, y su site
  puede cambiar conservando el id.
- **Scope**: pertenece a un hub y opcionalmente a un scope padre; se localiza por nombre, CIDR y
  tipo (`landing_zone` o `project`).
- **Network**: cuelga de un scope; se localiza por nombre y/o CIDR. Filtrar por hub incluye las
  de cualquier scope del hub a cualquier profundidad.
- **Subred**: cuelga de una network y puede estar contenida en una zona; se localiza por nombre,
  CIDR o zona.
- **IP**: se identifica de forma estable por subred + dirección; puede estar ocupada (con id) o
  libre (sin id, estado `available`).

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un operador puede obtener una network, subred, scope o hub escribiendo solo su
  nombre o su CIDR, sin ningún id en la configuración.
- **SC-002**: El 100 % de las búsquedas sin coincidencias en los siete data sources de listado
  devuelven una lista vacía usable en expresiones `for` y `length`.
- **SC-003**: Consultar una IP por dirección devuelve exactamente un elemento para direcciones
  ocupadas o libres de la subred, y un error para direcciones de fuera, en el 100 % de los casos.
- **SC-004**: Mover un hub de site produce cero recreaciones de recursos en el `plan` y el
  siguiente `plan` tras el `apply` no muestra cambios.
- **SC-005**: Las configuraciones existentes producen un `plan` sin cambios tras actualizar el
  provider y la batería de aceptación existente sigue en verde contra IPzilon 3.1.x y 3.2.0.
- **SC-006**: Toda combinación de filtros no permitida o valor mal formado se detecta en `plan`,
  antes de cualquier `apply`.
- **SC-007**: Usar las funciones nuevas contra una versión de IPzilon que no las soporta produce
  siempre un error que menciona la versión mínima, nunca un resultado silenciosamente incorrecto.

## Assumptions

- El alcance cubre al 100 % la issue #16 (puntos 1, 2 y 3 y todos sus criterios de aceptación) y
  la issue #21, más las acciones 2–8 de `contract-changes.md §0` y 2–4 de `006 §0`.
- Se mantiene la convención del provider: data sources de listado que devuelven todas las
  coincidencias. Varios resultados al buscar solo por CIDR no son un error (la recomendación de
  §5 de fallar con `total > 1` aplica a data sources de un solo elemento, que este provider no
  tiene para estas entidades).
- Cuando se indique un id de padre se sigue usando el listado por padre actual, para garantizar
  resultados idénticos y compatibilidad con IPzilon 3.1.x; los listados globales se usan solo
  cuando no hay id de padre o se usan filtros nuevos que solo el listado global admite.
- La validación de CIDR sin bits de host en `plan` se aplica solo a las búsquedas sin id de
  padre, para no cambiar el comportamiento de las configuraciones existentes.
- La versión mínima para los filtros sin ids y para mover hubs es IPzilon 3.2.0, publicada el
  2026-10-01 con las specs 005 y 006.
- Las comprobaciones de versión se hacen solo cuando se usan las funciones nuevas y no bloquean
  versiones desconocidas o de desarrollo, como las existentes.
- Ningún atributo de UI o monitorización de IPzilon se expone en los data sources (Principio I).
- Se publicará como versión MINOR del provider (v3.2.0), sin cambios incompatibles ni migración
  de estado; quitar el reemplazo forzado del site del hub solo relaja el `plan`.
- Las pruebas de aceptación se ejecutarán contra la imagen publicada de IPzilon 3.2.0.
