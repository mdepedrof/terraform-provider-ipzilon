# Research: Documentar el 429 por tokens inexistentes (IPzilon 3.4.0)

Fuentes: issue #24; `ipzilon/specs/008-backend-request-wait/contracts/configuration.md`
(*Límite de tokens de API inexistentes*); `ipzilon/docs/reference/environment-vars.md`
(`RATE_LIMIT_INVALID_API_TOKEN`); `internal/client/client.go` (bucle de reintentos de `do`).

## R1 — Contrato de IPzilon 3.4.0

- **Decision**: tratar como bloqueo por dirección todo `429` cuyo mensaje (extraído por
  `parseAPIError` del campo `error`) empiece por `Too many invalid API tokens`.
- **Rationale**: es el cuerpo publicado en el contrato de la spec 008 de IPzilon. El 429 del
  límite global usa `Rate limit exceeded: …`, así que el prefijo los distingue sin ambigüedad.
  Usar prefijo (como `IsSearchTruncated`) tolera que IPzilon añada detalle al final.
- **Alternatives considered**: comparar el texto exacto (frágil ante un sufijo); usar el valor de
  `Retry-After` (no distingue causas); una cabecera propia (IPzilon no la envía).

## R2 — Dónde emitir el aviso

- **Decision**: en `Client.do`, dentro de la rama de 429/503, tras decidir que se reintenta
  (después de la comprobación de abandono) y antes de dormir.
- **Rationale**: es el único punto por el que pasan todas las peticiones (Constitución: HTTP solo
  en `internal/client`); allí se conocen método, ruta y espera. Si el presupuesto se agota, el
  error final ya contiene `Too many invalid API tokens (gave up after …)` y no hace falta aviso.
- **Alternatives considered**: en los recursos/data sources (duplicaría código en ~20 sitios y no
  conocen los reintentos); un diagnóstico de Terraform (el cliente no tiene acceso a
  `diag.Diagnostics` y la espera se resuelve sola: mostrarla en la salida normal sería ruido).

## R3 — Frecuencia del aviso

- **Decision**: un `tflog.Warn` como mucho una vez por llamada a `do` (variable local del bucle).
  El `tflog.Debug` actual de cada reintento se mantiene sin cambios para todos los 429/503.
- **Rationale**: con `Retry-After ≤ 60` y una ventana de 60 s, una petición normalmente reintenta
  una o dos veces; repetir el aviso solo añade ruido. Con paralelismo cada petición afectada
  avisa una vez (aceptado en la spec). Mantener el Debug intacto evita tocar el comportamiento
  existente.
- **Alternatives considered**: aviso global una vez por proceso (estado compartido con
  concurrencia, y se perdería información si el bloqueo se repite más tarde); sustituir el Debug
  por Warn en esos reintentos (más ruido y más cambios).

## R4 — Contenido del aviso

- **Decision**: mensaje en inglés (como el resto de logs y descripciones), con los campos
  `method`, `path` y `wait`, sin token ni cabeceras. Ver [contracts/log-warning.md](./contracts/log-warning.md).
- **Rationale**: debe decir qué pasa (bloqueo por dirección), qué no es (el cupo del token
  configurado), qué hacer (buscar procesos con tokens borrados o mal copiados tras la misma
  dirección, incluido el propio) y cuánto se espera.

## R5 — Documentación

- **Decision**: ampliar la `Description` del esquema del provider (fuente de `docs/index.md`,
  regenerado con `tfplugindocs`; no hay `templates/`) y el párrafo de límites de la sección
  *Compatibility* del `README.md`. Texto en [contracts/provider-docs.md](./contracts/provider-docs.md).
- **Rationale**: son los dos sitios donde hoy se documenta el cupo por token (Principio IV). La
  descripción del Registry se mantiene breve; el detalle (umbral, ventana, variable de IPzilon,
  NAT, tokens revocados, solución) va en el README.
- **Alternatives considered**: una guía aparte en `docs/guides/` (desproporcionado para un
  párrafo y exigiría plantillas de `tfplugindocs`).

## R6 — Tests

- **Decision**: tests unitarios en `internal/client/client_test.go` que capturan el log con
  `tflogtest.RootLogger(ctx, &buf)` y cuentan las entradas `@level == "warn"` con
  `tflogtest.MultilineJSONDecode`: (a) 429 de tokens inexistentes ×2 y luego 200 → éxito y un
  solo aviso; (b) 429 de cupo normal y 503 → ningún aviso; (c) el aviso no contiene el token.
- **Rationale**: `tflogtest` está en el módulo `terraform-plugin-log` v0.11.0 ya requerido; no
  añade dependencias. Los tests existentes cubren que la política de reintentos no cambia (FR-007).

## R7 — Versión

- **Decision**: release PATCH **v3.2.1**; la versión mínima de IPzilon sigue en 3.0.0 y la tabla
  de compatibilidad del README no cambia.
- **Rationale**: sin cambios de esquema ni de comportamiento funcional; contra IPzilon < 3.4.0 el
  429 nunca aparece.
