# Implementation Plan: Documentar el 429 por tokens inexistentes (IPzilon 3.4.0)

**Branch**: `005-invalid-token-rate-limit` | **Date**: 2026-10-06 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/005-invalid-token-rate-limit/spec.md`

## Summary

Implementa la issue #24. IPzilon **3.4.0** bloquea durante una ventana (60 s por defecto) las
peticiones con token de API de una dirección de cliente tras acumularse 401 por tokens
inexistentes, con `429` + `Retry-After` + `{"error": "Too many invalid API tokens"}`. El provider
ya reintenta ese 429 sin fallar, así que el cambio es de **diagnóstico**:

1. **Documentación** (US1): ampliar la `Description` del esquema del provider (que genera
   `docs/index.md`) y el párrafo de límites del `README.md` con el bloqueo por dirección.
2. **Aviso en el log** (US2): en `Client.do`, al reintentar un 429 cuyo mensaje empiece por
   `Too many invalid API tokens`, emitir un `tflog.Warn` una sola vez por petición. La política
   de reintentos no cambia.

Sin cambios de esquema ni de versión mínima de IPzilon: release PATCH **v3.2.1**. Detalle en
[research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.25.8 (`go.mod`)

**Primary Dependencies**: `terraform-plugin-framework` v1.19.0, `terraform-plugin-log` v0.11.0
(`tflog`; en tests, `tflogtest`, ya incluido en el mismo módulo). Sin dependencias nuevas.

**Storage**: N/A

**Testing**: `go test ./...` con `httptest` y los helpers existentes de
`internal/client/client_test.go` (`sequenceServer`, `newTestClient`, `fakeSleeper`); los logs se
capturan con `tflogtest.RootLogger` y `tflogtest.MultilineJSONDecode`. No hacen falta tests de
aceptación: el comportamiento contra IPzilon real no cambia.

**Target Platform**: plugin de Terraform (binarios GoReleaser multi-plataforma)

**Project Type**: Terraform provider (cliente de la API REST de IPzilon)

**Performance Goals**: ninguna petición ni espera adicional; detectar el mensaje solo parsea el
cuerpo ya leído de cada 429 reintentado, sin peticiones extra.

**Constraints**: política de reintentos intacta (FR-007); el token nunca en logs (FR-006); docs
regeneradas con `tfplugindocs`, nunca editadas a mano.

**Scale/Scope**: 1 constante + ~10 líneas en `internal/client/client.go`, 2–3 tests unitarios,
`Description` en `internal/provider/provider.go`, `docs/index.md` regenerado, párrafo del
`README.md`.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio / norma | Evaluación | Estado |
|---|---|---|
| I. API como fuente de verdad | No se añade lógica de negocio; solo se lee el mensaje que devuelve la API para explicarlo. No se exponen métricas de UI. | ✅ |
| II. Asignación atómica | No aplica: no se tocan recursos de asignación. | ✅ N/A |
| III. Compatibilidad de esquema y estado | Solo cambia la `Description` del provider; ningún atributo, recurso ni estado. Release PATCH. | ✅ |
| IV. Documentación sincronizada | `docs/index.md` se regenera con `make generate` en el mismo PR; `README.md` actualizado. Sin ejemplos nuevos (no hay recursos nuevos). | ✅ |
| V. Pruebas | Tests unitarios del aviso sin API en vivo; los tests de reintento existentes deben seguir en verde. | ✅ |
| Credenciales | El aviso solo incluye método, ruta, estado y espera; nunca el token ni cabeceras. | ✅ |
| HTTP solo en `internal/client` | El cambio vive en `Client.do`. | ✅ |
| Flujo | Rama `005-invalid-token-rate-limit` desde `main` actualizada; commits con `commit-message`. | ✅ |

**Re-evaluación tras el diseño (Fase 1)**: sin cambios; ninguna violación que justificar.

## Project Structure

### Documentation (this feature)

```text
specs/005-invalid-token-rate-limit/
├── plan.md              # Este fichero
├── research.md          # Fase 0
├── data-model.md        # Fase 1 (sin entidades de datos; describe el evento de aviso)
├── quickstart.md        # Fase 1
├── contracts/
│   ├── provider-docs.md # Texto de la Description y del README
│   └── log-warning.md   # Contrato del aviso en el log
├── checklists/
│   └── requirements.md
└── tasks.md             # Fase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
internal/
├── client/
│   ├── client.go        # constante del mensaje + aviso en el bucle de reintentos de do()
│   └── client_test.go   # tests del aviso (sí/no/una vez)
└── provider/
    └── provider.go      # Description del esquema del provider

docs/index.md            # regenerado (make generate)
README.md                # párrafo de límites en "Compatibility"
```

**Structure Decision**: proyecto único existente; no se crean paquetes ni ficheros de código
nuevos.

## Complexity Tracking

Sin violaciones de la constitución.
