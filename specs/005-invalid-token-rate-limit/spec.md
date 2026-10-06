# Feature Specification: Documentar el 429 por tokens inexistentes (IPzilon 3.4.0)

**Feature Branch**: `005-invalid-token-rate-limit`

**Created**: 2026-10-06

**Status**: Draft

**Input**: User description: "quiero abordar el Issuse https://github.com/mdepedrof/terraform-provider-ipzilon/issues/24"

**Documentos de origen**:

- Issue #24 *[Docs] - Documentar el 429 por tokens inexistentes de IPzilon 3.4.0*.
- *Cambios de contrato — límite de tokens de API inexistentes*, feature 008 de IPzilon
  (`ipzilon/specs/008-backend-request-wait/contracts/configuration.md`) y la variable
  `RATE_LIMIT_INVALID_API_TOKEN` en `ipzilon/docs/reference/environment-vars.md`.

## Contexto

IPzilon **3.4.0** añade un límite por **dirección de cliente**: cuando desde una dirección se
acumulan `RATE_LIMIT_INVALID_API_TOKEN` (30/min por defecto) respuestas 401 por un token de API
**inexistente**, durante una ventana completa (60 s por defecto) **cualquier** petición con token
de API desde esa dirección recibe:

```text
HTTP 429
Retry-After: <segundos restantes, ≤ 60>
{"error": "Too many invalid API tokens"}
```

- Afecta también a peticiones con un token **válido** (p. ej. runners que salen por la misma IP
  pública/NAT que otro proceso con un token borrado o mal copiado).
- Los tokens revocados o caducados no cuentan; un token válido nunca genera esos 401.
- Las sesiones de usuario, el login y `/health*` no se ven afectados.

**Impacto funcional en el provider: ninguno.** Desde v3.0.0 el provider reintenta los 429
respetando `Retry-After` hasta 10 minutos por petición, así que un `plan`/`apply` con token válido
puede esperar hasta ~60 s, pero no falla. El problema es de **comprensión**: hoy la documentación
solo menciona el cupo por token, y la espera parece un límite de cupo del propio token, lo que
lleva a diagnosticar mal la causa (p. ej. a crear tokens nuevos, que no ayuda).

## User Scenarios & Testing *(mandatory)*

### User Story 1 - Entender por qué un `apply` con token válido espera ~60 s (Priority: P1)

Como operador que ejecuta Terraform en un runner compartido, quiero que la documentación del
provider explique que, además del cupo por token, IPzilon puede bloquear temporalmente la
dirección de salida cuando se acumulan tokens inexistentes, para identificar la causa (otro
proceso con un token borrado o mal copiado tras la misma IP) en lugar de culpar a mi token.

**Why this priority**: es lo que pide la issue de forma obligatoria y lo que evita diagnósticos
erróneos; no cambia el comportamiento del provider.

**Independent Test**: leer la descripción del provider en el Registry (`docs/index.md`) y la
sección de compatibilidad/límites del `README.md` y comprobar que describen ambos límites, el
mensaje que los distingue, la duración máxima de la espera y que el provider la absorbe sin fallar.

**Acceptance Scenarios**:

1. **Given** la página del provider en el Registry, **When** un operador lee la descripción,
   **Then** encuentra que, además del cupo por token, existe un bloqueo temporal por dirección de
   cliente cuando se acumulan peticiones con tokens inexistentes (IPzilon >= 3.4.0), y que el
   provider también lo reintenta.
2. **Given** el `README.md`, **When** un operador busca por qué un `plan` se queda esperando,
   **Then** encuentra el mensaje `Too many invalid API tokens`, que la espera dura como mucho una
   ventana (60 s por defecto, configurable en IPzilon), que afecta a todos los clientes con token
   de API tras la misma dirección (incluido uno válido), y qué hacer (localizar y corregir el
   proceso con el token inexistente).
3. **Given** la documentación actualizada, **When** se compara con la anterior, **Then** la
   información existente sobre el cupo por token y los reintentos de 429/503 se mantiene.

---

### User Story 2 - Distinguir el bloqueo por dirección en los logs (Priority: P2)

Como operador que depura una ejecución lenta con los logs de Terraform activados, quiero ver un
aviso explícito cuando el provider espera por un 429 de tokens inexistentes, para no confundirlo
con el agotamiento del cupo de mi propio token.

**Why this priority**: la issue lo marca como opcional; mejora el diagnóstico, pero el valor
principal ya lo da la documentación.

**Independent Test**: con un servidor simulado que responde un 429 con cuerpo
`{"error": "Too many invalid API tokens"}` y `Retry-After`, y luego 200, comprobar que la
petición acaba bien y que se registra un aviso que nombra el bloqueo por dirección; con un 429 de
cupo normal, comprobar que ese aviso no aparece.

**Acceptance Scenarios**:

1. **Given** una petición que recibe un 429 con el mensaje `Too many invalid API tokens`,
   **When** el provider la reintenta, **Then** registra un aviso (nivel warning) que explica que
   la dirección de cliente está bloqueada temporalmente por peticiones con tokens de API
   inexistentes desde esa misma dirección, que no se debe al cupo del token configurado, y cuánto
   va a esperar.
2. **Given** un 429 por cupo normal (cualquier otro mensaje) o un 503, **When** el provider lo
   reintenta, **Then** el registro es el mismo que hoy, sin el aviso anterior.
3. **Given** una petición que recibe ese 429 varias veces seguidas, **When** se reintenta,
   **Then** el aviso aparece una sola vez para esa petición (los reintentos siguientes quedan en
   el nivel de depuración actual).
4. **Given** cualquier 429, **When** el provider lo reintenta, **Then** los tiempos de espera, el
   límite de 10 minutos y el error final si se agota no cambian.

### Edge Cases

- **El token configurado en el provider es el inexistente**: el provider falla con el 401 en la
  primera petición (no reintenta 401). Si la dirección ya está bloqueada, primero espera la
  ventana y después recibe el 401; el aviso de US2 debe sugerir comprobar también el propio token.
- **Varias peticiones en paralelo** (`-parallelism`): cada petición afectada puede emitir su
  aviso; es aceptable un aviso por petición, no por reintento.
- **Cuerpo con el mensaje en otro campo o con texto adicional**: el aviso se basa en el mensaje
  de error ya extraído del cuerpo; si IPzilon cambia el texto, el 429 se sigue reintentando igual
  y solo se pierde el aviso.
- **IPzilon < 3.4.0**: nunca devuelve ese 429; no hay cambio de comportamiento ni de versión
  mínima requerida.
- **Se agota el presupuesto de 10 minutos** (solo posible si el bloqueo se renueva
  continuamente): el error final conserva el mensaje `Too many invalid API tokens`, como hoy.

## Requirements *(mandatory)*

### Functional Requirements

- **FR-001**: La descripción del provider (la que aparece en el Registry) MUST mencionar, además
  del cupo por token, el bloqueo temporal por dirección de cliente tras acumularse peticiones con
  tokens de API inexistentes (IPzilon >= 3.4.0), y que esas respuestas también se reintentan.
- **FR-002**: El `README.md` MUST explicar ese bloqueo: mensaje `Too many invalid API tokens`,
  umbral y ventana por defecto (30 fallos/min, 60 s) configurables en IPzilon con
  `RATE_LIMIT_INVALID_API_TOKEN`, que afecta también a tokens válidos tras la misma dirección, que
  los tokens revocados o caducados no cuentan, que el provider espera sin fallar y cómo resolverlo.
- **FR-003**: La documentación generada del Registry MUST regenerarse a partir de la descripción
  del esquema, sin ediciones manuales, en el mismo cambio.
- **FR-004**: Al reintentar un 429 cuyo mensaje empiece por `Too many invalid API tokens`, el provider
  MUST registrar un aviso de nivel warning que identifique el bloqueo por dirección, indique que no
  es el cupo del token configurado, sugiera revisar procesos con tokens inexistentes (incluido el
  propio) e incluya la espera.
- **FR-005**: El aviso de FR-004 MUST emitirse como mucho una vez por petición y MUST NOT
  emitirse para otros 429 ni para los 503.
- **FR-006**: El aviso MUST NOT incluir el token ni otras credenciales.
- **FR-007**: La política de reintentos (respeto de `Retry-After`, espera máxima por intento,
  presupuesto de 10 minutos y error final) MUST NOT cambiar.
- **FR-008**: El comportamiento de FR-004 y FR-005 MUST cubrirse con tests unitarios que no
  dependan de una instancia real de IPzilon.

## Success Criteria *(mandatory)*

### Measurable Outcomes

- **SC-001**: Un operador que solo lee la página del provider en el Registry o el `README.md`
  puede nombrar las dos causas posibles de un 429 (cupo del token y bloqueo por dirección) y la
  espera máxima del bloqueo (60 s por defecto).
- **SC-002**: Cada petición que reintenta uno o más 429 `Too many invalid API tokens` deja
  exactamente un aviso en el log que lo explica; las peticiones que solo reintentan otros 429 o
  503 no dejan ninguno.
- **SC-003**: Una ejecución con token válido bloqueada por dirección termina sin error en cuanto
  vence la ventana, igual que antes del cambio (0 regresiones en los tests de reintento
  existentes).
- **SC-004**: La comprobación de documentación sincronizada de la CI pasa sin diferencias tras
  regenerar.

## Assumptions

- Se implementan las dos propuestas de la issue: la documentación (obligatoria) y el aviso en el
  log (opcional en la issue, incluido aquí por su bajo coste). Si se decide descartar el aviso,
  la US2 y FR-004–FR-006/FR-008 se eliminan sin afectar a la US1.
- El aviso solo es visible con los logs de Terraform activados (`TF_LOG=WARN` o más detallado);
  no se muestra como advertencia en la salida normal de `plan`/`apply`, porque la espera no es un
  problema de la configuración del usuario y el provider la resuelve solo.
- La detección se hace por el mensaje de error que IPzilon devuelve en el cuerpo
  (`Too many invalid API tokens`), que es el contrato publicado en 3.4.0.
- No cambia la versión mínima de IPzilon soportada (>= 3.0.0) ni el esquema de ningún recurso;
  el cambio encaja en una versión PATCH del provider.
- Fuera de alcance: exponer o configurar desde el provider los límites de IPzilon, y cualquier
  cambio en IPzilon.
