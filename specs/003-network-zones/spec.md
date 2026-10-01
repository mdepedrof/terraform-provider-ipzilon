# Feature Specification: Zonas de red (IPzilon 3.1.0)

**Feature Branch**: `003-network-zones`

**Created**: 2026-10-01

**Status**: Draft

**Input**: User description: "Hemos actualizado Ipzilon a la version 3.1.0 para extender la funcionalidad de networks y permitir agrupar subnets en Zonas dentro de la network. Hay que modificar el provider para permitir crear, modificar zonas, y los cambios a nivel de subnets para meterlas en zonas. Tambien hay que modificar el provider para actualizar los datasources para añadir los nuevos elementos para las zonas y luego tambien en el resto de recursos de datasources para poder filtrar por las propiedades de los elementos PERO SIN necesidad de saber el id. por ejemplo na network solo por su cidr sin tener que añadir el id."

**Documento de origen**: *Cambios de contrato de la API — IPzilon 3.1.0 (zonas de red)*, feature 003
de IPzilon (`ipzilon/specs/003-network-zones/contract-changes.md` y `contracts/zones-api.md`). Las
referencias `§N` de esta spec apuntan a los apartados de `contract-changes.md`.

## Contexto

IPzilon **3.1.0** (publicada el 2026-10-01) introduce las **zonas de red**: bloques lógicos con
nombre dentro de una Network que agrupan subredes. Todos los cambios de la API son aditivos y el
provider actual (v3.0.1) sigue funcionando contra 3.1.0 sin cambios (verificado por el equipo de
IPzilon: 56 tests de aceptación en verde).

Puntos clave del modelo de zonas:

- Una zona pertenece a una única Network; su CIDR está dentro del de la Network, es estrictamente
  menor y no solapa otras zonas de la misma Network. El nombre es único **por Network**.
- La pertenencia de una subred a una zona **no se guarda**: se calcula por contención del CIDR.
  Indicar una zona al crear o modificar una subred con CIDR explícito es una *aserción* que la API
  valida.
- Una subred nunca puede quedar a caballo del límite de una zona.
- Borrar una zona **no** borra sus subredes; pasan a estar "fuera de zona".
- En una Network con zonas, la asignación automática de subredes sin zona indicada busca hueco
  **fuera** de las zonas.

Además, la API 3.1.0 ofrece un listado **global** de zonas filtrable por nombre y/o CIDR sin
conocer ningún id.

## Clarifications

### Session 2026-10-01

- Q: La API 3.1.0 solo permite buscar sin ids sites y zonas; hubs, scopes, networks y subredes solo
  se listan colgando de su padre. ¿Cómo se resuelven los filtros sin id en esos data sources? →
  A: Se sacan de esta feature. Esta spec se limita a las zonas (incluida la búsqueda de zonas sin
  ids) y a no romper los filtros actuales; el resto pasa a una spec posterior.

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Gestionar zonas de red con un CIDR explícito (Priority: P1)

Como operador de infraestructura que usa Terraform, quiero declarar zonas dentro de una Network
indicando su nombre, CIDR y descripción, modificarlas, borrarlas e importarlas, para organizar el
espacio de direcciones de la Network igual que lo hago desde la interfaz de IPzilon.

**Why this priority**: sin el recurso de zona no se puede usar ninguna otra parte de la
funcionalidad desde Terraform; es la base del resto de historias.

**Independent Test**: contra IPzilon 3.1.0, aplicar una configuración que cree una zona en una
Network existente, cambiar su nombre, descripción y CIDR, importarla en un estado vacío y
destruirla, comprobando en IPzilon cada paso y que el `plan` posterior a cada `apply` o import no
muestra cambios.

**Acceptance Scenarios**:

1. **Given** una Network `10.0.16.0/22` sin zonas, **When** se aplica una zona `pooled_zone_1`
   con CIDR `10.0.16.0/23`, **Then** la zona existe en IPzilon con esos datos y el siguiente
   `plan` no muestra cambios.
2. **Given** una zona gestionada, **When** se cambia su nombre, descripción o CIDR, **Then** la
   zona se actualiza in situ, sin destruirse ni recrearse, y conserva su id.
3. **Given** una zona gestionada, **When** se cambia la Network a la que pertenece, **Then** el
   `plan` anuncia que la zona se destruirá y se volverá a crear.
4. **Given** una zona creada fuera de Terraform, **When** se importa por su id, **Then** todos los
   atributos quedan rellenos y el siguiente `plan` no muestra cambios.
5. **Given** una zona gestionada que contiene subredes, **When** se destruye, **Then** la zona
   desaparece y las subredes siguen existiendo, ahora fuera de zona.
6. **Given** una zona gestionada que alguien borra en IPzilon, **When** se ejecuta `plan`,
   **Then** Terraform detecta que ya no existe y propone crearla de nuevo.
7. **Given** una configuración con un CIDR fuera de la Network, que solapa otra zona, que deja
   una subred a caballo, que al encoger deja subredes fuera o con un nombre repetido en la
   Network, **When** se aplica, **Then** el `apply` falla mostrando el mensaje explicativo de
   IPzilon (incluido el nombre de la zona o subred en conflicto).

---

### User Story 2 - Asignar subredes dentro de una zona (Priority: P1)

Como operador, quiero indicar la zona al pedir la siguiente (o última) subred libre de una Network,
y poder declarar a qué zona pertenece una subred con CIDR explícito, para que las subredes de cada
carga de trabajo caigan en el bloque reservado para ella.

**Why this priority**: es el uso principal de las zonas (p. ej. subredes `/28` de host pools
agrupadas en una zona). Además, en cuanto una Network tiene zonas, la asignación sin zona cambia
de comportamiento y el usuario necesita poder elegir.

**Independent Test**: con una Network con dos zonas, aplicar subredes automáticas con y sin zona y
una subred con CIDR explícito que declara su zona; comprobar en IPzilon dónde cae cada subred, que
la zona de cada una se refleja en el estado y que los `plan` siguientes no muestran cambios.

**Acceptance Scenarios**:

1. **Given** una Network con la zona `pooled_zone_1` (`10.0.16.0/23`), **When** se aplica una
   subred automática `/28` indicando esa zona, **Then** la subred recibe un CIDR dentro de
   `10.0.16.0/23` y el estado refleja la zona.
2. **Given** una Network con zonas, **When** se aplica una subred automática sin indicar zona,
   **Then** la subred recibe un CIDR fuera de todas las zonas y el estado refleja que no tiene
   zona.
3. **Given** una zona llena, **When** se pide una subred automática en ella, **Then** el `apply`
   falla con el mensaje de IPzilon que indica que no hay hueco en esa zona.
4. **Given** una subred con CIDR explícito que declara una zona que no la contiene (o que es de
   otra Network), **When** se aplica, **Then** el `apply` falla con el mensaje de IPzilon.
5. **Given** una subred existente sin zona declarada en la configuración, **When** se le añade la
   zona que ya la contiene, **Then** el `plan` no muestra cambios (y nunca una recreación).
6. **Given** una subred gestionada dentro de una zona, **When** la zona se borra o se importa la
   subred, **Then** el siguiente `plan` actualiza la zona calculada sin destruir ni recrear la
   subred.
7. **Given** una configuración que no usa zonas, **When** se actualiza el provider a esta
   versión, **Then** el `plan` no muestra cambios en ninguna subred existente.

---

### User Story 3 - Consultar zonas y subredes por zona sin conocer ids (Priority: P2)

Como operador, quiero localizar una zona por su nombre y/o CIDR sin conocer su id ni el de su
Network, y listar las subredes de una zona (o las que están fuera de zona), para usar esos datos
en otros recursos de mi configuración.

**Why this priority**: permite referenciar zonas creadas por otros equipos o desde la interfaz;
es la vía natural para encadenar con la asignación de subredes de la Historia 2.

**Independent Test**: con un inventario con varias Networks y zonas (incluidas dos zonas con el
mismo nombre en Networks distintas), leer el data source de zonas por nombre, por CIDR, por ambos,
por Network y por id, y el de subredes por zona y "sin zona", comparando con el inventario real.

**Acceptance Scenarios**:

1. **Given** una zona `pooled_zone_1` con CIDR `10.0.16.0/23`, **When** se consulta el data
   source de zonas solo por ese CIDR, **Then** se obtiene esa zona con su id, Network, nombre,
   CIDR y descripción.
2. **Given** dos zonas con el mismo nombre en Networks distintas, **When** se consulta solo por
   nombre, **Then** se obtienen las dos; **When** se añade la Network como filtro, **Then** se
   obtiene solo una.
3. **Given** una zona con subredes, **When** se listan las subredes indicando solo la zona,
   **Then** se obtienen exactamente las subredes contenidas en ella, cada una con su zona.
4. **Given** una Network con subredes dentro y fuera de zonas, **When** se listan sus subredes
   pidiendo solo las que no tienen zona, **Then** se obtienen exactamente las de fuera de zona.
5. **Given** un filtro sin coincidencias, **When** se consulta, **Then** se obtiene una lista
   vacía sin error.
6. **Given** filtros contradictorios (zona concreta y "sin zona" a la vez), **When** se consulta,
   **Then** el `plan` falla con un mensaje que explica que son excluyentes.

---

### User Story 4 - Reservar zonas automáticamente por tamaño (Priority: P3)

Como operador, quiero pedir la siguiente (o última) zona libre de un tamaño dado dentro de una
Network, igual que hago con subredes y networks, para no tener que calcular a mano qué bloque está
libre.

**Why this priority**: es cómodo y coherente con los recursos `next_*` existentes, pero las zonas
suelen planificarse a mano; la Historia 1 cubre el caso esencial.

**Independent Test**: con una Network con zonas y subredes, aplicar una zona automática `/24`
desde el principio y otra desde el final; comprobar que caen en bloques vacíos, que el CIDR no
cambia en los `plan` siguientes y que se pueden importar.

**Acceptance Scenarios**:

1. **Given** una Network con espacio libre, **When** se aplica una zona automática `/24`,
   **Then** la zona recibe el primer bloque `/24` que no solapa ninguna zona ni subred, y el CIDR
   asignado no cambia en `plan` posteriores.
2. **Given** la variante "última", **When** se aplica, **Then** la zona recibe el último bloque
   libre de ese tamaño.
3. **Given** una zona automática gestionada, **When** se cambia la Network o el tamaño,
   **Then** el `plan` anuncia que se destruirá y se volverá a crear.
4. **Given** una zona automática gestionada, **When** se cambia el nombre o la descripción,
   **Then** se actualiza in situ conservando su CIDR.
5. **Given** una Network sin bloques vacíos del tamaño pedido, **When** se aplica, **Then** el
   `apply` falla con el mensaje de IPzilon.

---

### Edge Cases

- **IPzilon 3.0.x**: si se usan zonas contra una instancia 3.0.x, las rutas de zona no existen y
  la zona indicada en una subred se ignora en silencio. El provider debe dar un error claro que
  indique que se necesita IPzilon ≥ 3.1.0, en lugar de un "no encontrado" genérico o, peor, crear
  la subred fuera de la zona sin avisar.
- **Cambio de comportamiento sin zona**: una subred automática sin zona en una Network que acaba
  de recibir su primera zona busca fuera de las zonas; las subredes ya existentes no se mueven,
  pero las nuevas pueden caer en otro sitio que antes.
- **Zona eliminada fuera de Terraform** con subredes gestionadas dentro: las subredes no deben
  recrearse; solo cambia su zona calculada.
- **Cambio del CIDR de una subred** que ya declara zona: la API valida que el CIDR final sigue
  dentro de la zona; si no, el `apply` falla con su mensaje.
- **Cambio del CIDR de la Network** que dejaría una zona fuera: lo rechaza IPzilon y el error
  debe mostrarse tal cual al aplicar el recurso de Network.
- **Nombre de zona repetido** en la misma Network, también en carreras entre ejecuciones
  concurrentes: el `apply` falla con el mensaje de IPzilon, sin dejar estado parcial.
- **Búsqueda truncada** en asignaciones automáticas (zona o subred en zona): se muestra el
  mensaje de IPzilon igual que en las asignaciones actuales.
- **Mayúsculas**: IPzilon normaliza nombres a minúsculas; un nombre de zona con mayúsculas en la
  configuración no debe provocar cambios perpetuos en el `plan`.
- **Zona sin descripción**: vaciar la descripción en la configuración debe vaciarla en IPzilon.
- **Listados grandes**: los listados de zonas y subredes por zona se devuelven por páginas y deben
  obtenerse completos, en el orden de la API (por CIDR).
- **Varios resultados** al filtrar por nombre sin acotar: no es un error; el data source devuelve
  todos y es la configuración del usuario la que decide.

## Requirements *(mandatory)*

### Functional Requirements

**Recurso de zona (§2)**

- **FR-001**: El provider DEBE ofrecer un recurso de zona de red con Network, nombre, CIDR y
  descripción configurables, e id calculado.
- **FR-002**: Cambiar el nombre, el CIDR o la descripción de una zona DEBE aplicarse in situ;
  cambiar la Network DEBE forzar su recreación.
- **FR-003**: Si la zona ya no existe en IPzilon, la lectura DEBE retirarla del estado para que
  Terraform detecte el drift; borrar una zona que ya no existe DEBE considerarse correcto.
- **FR-004**: El recurso de zona DEBE poder importarse por id, quedando todos sus atributos
  rellenos de modo que el siguiente `plan` no muestre cambios, y DEBE tener ejemplo de import.
- **FR-005**: Destruir una zona NO DEBE destruir ni modificar las subredes que contiene.
- **FR-006**: Los errores de validación de IPzilon (fuera de la Network, solape, subred a
  caballo, subredes que quedan fuera, nombre repetido, formato) DEBEN mostrarse al usuario con el
  mensaje original de la API.
- **FR-007**: El recurso de zona NO DEBE exponer métricas de ocupación ni de alertas de IPzilon
  (ocupación, IPs totales/usadas/disponibles, porcentaje de alerta, número de subredes), aunque la
  API las devuelva (Principio I de la constitución).

**Subredes en zonas (§4, §5)**

- **FR-008**: Los recursos de siguiente y última subred libre DEBEN admitir una zona opcional en
  la que buscar el hueco; sin zona, el comportamiento es el que defina IPzilon (toda la Network,
  o fuera de las zonas si la Network las tiene).
- **FR-009**: El recurso de subred con CIDR explícito DEBE admitir una zona opcional que IPzilon
  valida al crear y al modificar.
- **FR-010**: En los tres recursos de subred, la zona DEBE reflejar en el estado la zona que la
  contiene según IPzilon (o ninguna), también tras un import.
- **FR-011**: La zona de una subred NUNCA DEBE forzar su recreación: ni al añadirla o quitarla en
  la configuración, ni cuando cambie en IPzilon (zona borrada, creada o redimensionada).
- **FR-012**: Si el usuario no declara zona en una subred, los cambios de la zona calculada en
  IPzilon NO DEBEN producir diferencias en el `plan`.
- **FR-013**: Las configuraciones existentes que no usan zonas NO DEBEN mostrar cambios en el
  `plan` tras actualizar el provider.

**Zonas automáticas (§3)**

- **FR-014**: El provider DEBE ofrecer recursos de siguiente y última zona libre en una Network a
  partir de un tamaño de prefijo y un nombre, con la reserva hecha de forma atómica por IPzilon
  (Principio II).
- **FR-015**: El CIDR asignado a una zona automática NO DEBE cambiar entre `plan` sucesivos;
  cambiar la Network o el tamaño DEBE forzar la recreación; nombre y descripción se actualizan in
  situ.
- **FR-016**: Las zonas automáticas DEBEN poder importarse por id con todos sus atributos
  rellenos, incluido el tamaño de prefijo derivado del CIDR.

**Data sources de zonas y subredes (§6)**

- **FR-017**: El provider DEBE ofrecer un data source de zonas que permita buscar por id, por
  Network, por nombre y por CIDR, de forma combinable, con coincidencia exacta resuelta en
  IPzilon y **sin exigir ningún id**.
- **FR-018**: El data source de zonas DEBE devolver todas las coincidencias (varias si el nombre
  se repite entre Networks), completas aunque la API pagine, en el orden de la API.
- **FR-019**: El data source de subredes DEBE permitir filtrar por zona y por "sin zona", siendo
  ambos filtros excluyentes; filtrar por zona NO DEBE exigir indicar también la Network.
- **FR-020**: Cada subred devuelta por el data source de subredes DEBE incluir la zona que la
  contiene (o ninguna).

**Compatibilidad, versión y documentación**

- **FR-021**: Los filtros actuales de los data sources existentes DEBEN seguir funcionando y
  devolviendo los mismos resultados.
- **FR-022**: Si una configuración usa zonas contra una instancia de IPzilon anterior a 3.1.0, el
  provider DEBE fallar con un mensaje que indique que se requiere IPzilon ≥ 3.1.0; las
  configuraciones que no usan zonas DEBEN seguir funcionando contra 3.0.x.
- **FR-023**: Todos los cambios DEBEN ser aditivos (ningún recurso, atributo o comportamiento
  existente se renombra, elimina o pasa a obligatorio), de modo que puedan publicarse en una
  versión MINOR del provider.
- **FR-024**: Cada recurso y data source nuevo o modificado DEBE tener descripción en todos sus
  atributos, ejemplo de uso, ejemplo de import cuando aplique y documentación regenerada.

### Key Entities

- **Zona de red**: bloque lógico con nombre dentro de una Network. Atributos relevantes para el
  provider: id, Network, nombre (único dentro de su Network, en minúsculas), CIDR (dentro de la
  Network, menor que ella, sin solapes con otras zonas) y descripción opcional.
- **Network**: contenedor de zonas y subredes; su CIDR no puede cambiar de forma que deje una
  zona fuera.
- **Subred**: gana una zona calculada por contención (o ninguna). Al crearla o modificarla se
  puede declarar una zona como aserción; en asignación automática la zona define dónde se busca.
- **Zona automática**: zona cuyo CIDR elige IPzilon a partir de un tamaño de prefijo, en un
  bloque que no solapa ninguna zona ni subred existente.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un operador puede crear una zona y asignar en ella una subred automática con una
  sola ejecución de `terraform apply`, sin consultar ni copiar ids a mano más allá de la Network.
- **SC-002**: El 100 % de las zonas y subredes importadas producen un `plan` sin cambios
  inmediatamente después del import.
- **SC-003**: Ningún cambio de zona (añadir o quitar la zona en la configuración, borrar o crear
  zonas en IPzilon) provoca la destrucción o recreación de una subred.
- **SC-004**: Las configuraciones existentes que no usan zonas producen un `plan` sin cambios
  tras actualizar el provider y la batería de aceptación existente sigue en verde contra IPzilon
  3.1.0.
- **SC-005**: Un operador puede obtener una zona indicando solo su CIDR o su nombre, sin escribir
  ningún id en la configuración.
- **SC-006**: Los data sources devuelven el 100 % de los elementos que cumplen el filtro, sin
  duplicados ni huecos, aunque superen el tamaño de página de la API.
- **SC-007**: Usar zonas contra IPzilon 3.0.x produce siempre un error que menciona la versión
  mínima requerida, nunca una subred creada fuera de la zona pedida.

## Assumptions

- **Fuera de alcance (trabajo futuro)**: localizar hubs, scopes, networks y subredes solo por sus
  propiedades (p. ej. una Network solo por su CIDR) sin indicar el id del padre. La API 3.1.0 no
  ofrece listados globales filtrables de esos elementos; se abordará en una spec posterior,
  previsiblemente pidiendo a IPzilon esos listados con filtrado en el servidor (Principio I).
- El alcance cubre lo descrito como acciones 2–7 en `contract-changes.md §0`; la acción 1
  (compatibilidad) ya está verificada por el equipo de IPzilon.
- Siguiendo la convención del provider, el data source de zonas es un único data source de
  listado con filtro opcional por id (como `ipzilon_hubs` o `ipzilon_networks`), en lugar de dos
  data sources separados para "una zona" y "varias zonas".
- Los nombres de recursos y data sources seguirán los propuestos por IPzilon
  (`ipzilon_network_zone`, `ipzilon_next_network_zone`, `ipzilon_last_network_zone`,
  `ipzilon_network_zones`); el nombre exacto se fija en el plan.
- La zona de las subredes se modela como opcional y calculada, sin forzar recreación, tal como
  recomienda IPzilon (§4, §5).
- La comprobación de versión de IPzilon solo se hace cuando la configuración usa zonas, para no
  añadir peticiones ni romper configuraciones contra 3.0.x.
- Las sugerencias de subnetting, el listado plano de CIDRs y las alertas de utilización por zona
  quedan fuera de alcance (son funciones de UI/monitorización, Principio I).
- Se publicará como versión MINOR del provider (v3.1.0), sin cambios incompatibles ni migración de
  estado.
- Las pruebas de aceptación se ejecutarán contra la imagen publicada de IPzilon 3.1.0.
