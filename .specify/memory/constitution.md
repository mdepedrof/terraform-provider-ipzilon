# Terraform Provider IPzilon Constitution

## Core Principles

### I. La API de IPzilon es la fuente de verdad

El provider es un cliente fino de la API de IPzilon y MUST limitarse a traducir recursos de
Terraform a llamadas de la API y viceversa.

- Las reglas de negocio (jerarquía de scopes, solapamiento de CIDR, `kind` válidos, etc.) MUST
  validarse en el backend; el provider solo añade validaciones de forma (tipos, rangos, formatos)
  que mejoren la experiencia en `plan`.
- `Read` MUST reflejar el estado real devuelto por la API; si el objeto ya no existe, el recurso
  MUST eliminarse del estado para que Terraform detecte el drift.
- El provider MUST NOT exponer datos orientados a la UI o a la monitorización de IPzilon
  (`alert_percent`, `alert_metric`, `total_ips`, `used_ips`), ni siquiera como atributos de solo
  lectura en data sources.
- Los filtros de data sources (`cidr`, `name`, …) SHOULD resolverse en el servidor cuando la API
  lo permita, en lugar de filtrar en cliente.

**Razón**: duplicar lógica del backend en el provider provoca divergencias entre versiones y
errores difíciles de diagnosticar; el alcance del provider es gestionar infraestructura, no métricas.

### II. Asignación atómica delegada al servidor

Los recursos que reservan espacio de direcciones de forma dinámica (`ipzilon_next_subnet`,
`ipzilon_last_subnet`, `ipzilon_next_network`, `ipzilon_next_ip_address` y los que se añadan)
MUST delegar el cálculo del bloque libre en un único endpoint atómico de la API.

- El provider MUST NOT calcular en cliente el siguiente CIDR o IP libre.
- El valor asignado (`cidr`, `address`) MUST ser `Computed` con `UseStateForUnknown`, de modo que
  no cambie entre `plan` sucesivos.
- Los atributos que determinan la asignación (padre, `prefix_length`) MUST llevar
  `RequiresReplace`.
- Las actualizaciones que la API pueda rechazar por conflicto con asignaciones propias MUST
  enviar los parámetros necesarios (p. ej. `force: true`) para que un `apply` idempotente no falle.

**Razón**: varias ejecuciones concurrentes de Terraform solo pueden evitar colisiones si la
reserva es atómica en el servidor.

### III. Compatibilidad de esquema y estado

El provider sigue versionado semántico (`MAJOR.MINOR.PATCH`) y protege el estado de los usuarios.

- Renombrar o eliminar un recurso o atributo, o convertir en obligatorio un atributo opcional,
  es un cambio incompatible y MUST publicarse en una versión MAJOR, documentado en la sección
  *Breaking Changes* del `README.md` con los pasos de migración.
- Cuando cambie el esquema de un recurso existente, SHOULD implementarse `UpgradeState` para
  migrar el estado automáticamente; si no es posible, MUST documentarse el `terraform state mv`
  o procedimiento manual equivalente.
- Todo recurso MUST soportar `terraform import` y disponer de su `import.sh` de ejemplo.
- Los atributos que no puedan actualizarse in situ en la API MUST marcarse con `RequiresReplace`.

**Razón**: un provider de IPAM gestiona datos críticos de red; perder o corromper el estado
tiene un coste operativo alto.

### IV. Documentación y ejemplos sincronizados

- `docs/` MUST generarse con `tfplugindocs` (`make generate`) y nunca editarse a mano; el cambio
  de esquema y la documentación regenerada MUST ir en el mismo commit/PR. La CI lo verifica.
- Cada recurso y data source MUST tener ejemplo en `examples/` (`resource.tf` o
  `data-source.tf`) y cada atributo MUST tener `Description`.
- Las versiones fijadas en ejemplos y en `docs/index.md` MUST coincidir con la versión MAJOR
  vigente.
- Los ejemplos MUST NOT contener credenciales ni datos sensibles reales.

**Razón**: la documentación del Terraform Registry es la interfaz principal del usuario con el
provider.

### V. Pruebas y verificación continua

- `go build ./...`, `go vet ./...` y `go test ./...` MUST pasar antes de mergear cualquier PR.
- La lógica pura (helpers, conversión de modelos, filtros, cálculo de planes) MUST tener tests
  unitarios que no dependan de una API en vivo.
- Los recursos nuevos o con cambios de comportamiento SHOULD cubrirse con tests de aceptación
  (`make testacc`, `TF_ACC=1`) ejecutados contra una instancia real de IPzilon antes de publicar
  una versión.
- Todo bug corregido SHOULD ir acompañado de un test que lo reproduzca cuando sea viable sin API.

**Razón**: la CI no dispone de una API de IPzilon, por lo que los tests unitarios son la red de
seguridad automática y los de aceptación la validación previa a la release.

## Restricciones técnicas y de seguridad

- **Lenguaje y framework**: Go en la versión fijada en `go.mod` y
  `terraform-plugin-framework` (con `terraform-plugin-framework-validators`). MUST NOT
  introducirse `terraform-plugin-sdk/v2` ni mezclar ambos frameworks.
- **Estructura**: cliente HTTP y modelos en `internal/client`, recursos en `internal/resources`,
  data sources en `internal/datasources`, registro en `internal/provider`. Las llamadas HTTP
  MUST pasar por `internal/client`.
- **Dependencias**: MUST mantenerse al día mediante Dependabot; las vulnerabilidades conocidas
  (GHSA/CVE) MUST corregirse fijando la versión parcheada con prioridad sobre otro trabajo.
- **Credenciales**: `token` MUST ser `Sensitive` y configurable vía `IPZILON_TOKEN`; nunca debe
  aparecer en logs, diagnósticos ni ejemplos.
- **Releases**: MUST generarse con GoReleaser a partir de un tag `vX.Y.Z`, firmadas con GPG y con
  `SHA256SUMS`. Solo la última versión publicada recibe correcciones de seguridad.
- **Vulnerabilidades**: se reportan por GitHub Security Advisories, nunca en issues públicas.

## Flujo de desarrollo y calidad

- Todo trabajo (issue, bug o feature) MUST hacerse en una rama nueva creada desde `main`
  actualizada. Tras mergear una PR MUST actualizarse `main` y limpiarse las ramas locales.
- Los títulos de issues MUST seguir el formato `[Type] - Summary` (p. ej. `[Feat] - Recurso
  next_network`).
- Los mensajes de commit MUST seguir el formato consistente del proyecto (comando
  `commit-message`) y MUST NOT incluir menciones de coautoría de herramientas de IA.
- Una PR por cambio lógico, con la plantilla de PR completada (tipo de cambio y checklist).
- Toda PR a `main` MUST tener la CI en verde (build, vet, test y docs actualizadas) y la
  aprobación del propietario definido en `CODEOWNERS`.
- Ante una nueva versión de la API de IPzilon, la revisión MUST centrarse en los cambios que
  afecten a los recursos gestionados, aplicando el Principio I.

## Governance

- Esta constitución prevalece sobre cualquier otra práctica del repositorio. `CONTRIBUTING.md`,
  `README.md` y las plantillas de Spec Kit MUST ser coherentes con ella; en caso de conflicto,
  se corrige el documento subordinado o se enmienda la constitución.
- **Enmiendas**: se proponen mediante PR que modifique `.specify/memory/constitution.md`,
  explicando el motivo y el impacto; requieren la aprobación del propietario del repositorio.
- **Versionado de la constitución**: MAJOR para eliminar o redefinir principios de forma
  incompatible; MINOR para añadir principios o secciones o ampliar materialmente las normas;
  PATCH para aclaraciones y correcciones de redacción.
- **Cumplimiento**: cada plan (`/speckit-plan`) MUST incluir un *Constitution Check* contra estos
  principios, y cada revisión de PR MUST verificarlos. Cualquier excepción MUST justificarse
  explícitamente en el plan o en la PR.

**Version**: 1.0.0 | **Ratified**: 2026-09-28 | **Last Amended**: 2026-09-28
