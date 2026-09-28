# Implementation Plan: Adaptación del provider a IPzilon 3.0.0

**Branch**: `001-ipzilon-api-v3` | **Date**: 2026-09-28 | **Spec**: [spec.md](./spec.md)

**Input**: Feature specification from `specs/001-ipzilon-api-v3/spec.md`

## Summary

IPzilon 3.0.0 pagina los listados, deja sin identificador a las direcciones IP libres y aplica
límite de peticiones y rechazo por saturación. El provider se adapta en `internal/client`
(paginación genérica `GetAll`, reintentos 429/503 con `Retry-After`, `IPAddress.ID` nulable,
detección de IPzilon < 3.0.0 vía `/health`), cambia el alta de `ipzilon_ip_address` a
`POST /subnets/{id}/ips` y la baja de los recursos de dirección a `DELETE /ips/{id}`, limita
`status` a `used`/`reserved`, traduce los 409 de búsqueda truncada y valida en `plan` el tope /8 e
IPv4. Se publica como `v3.0.0` (fin de soporte de IPzilon 2.x y `status` sin `available`), sin
`UpgradeState`. Detalle de decisiones en [research.md](./research.md).

## Technical Context

**Language/Version**: Go 1.25.8 (`go.mod`)

**Primary Dependencies**: `terraform-plugin-framework` v1.19.0,
`terraform-plugin-framework-validators` v0.19.0, `terraform-plugin-log` (ya indirecta; se usa
`tflog` para registrar reintentos). **Sin dependencias nuevas** (R3).

**Storage**: N/A (estado de Terraform)

**Testing**: `go test ./...` con `net/http/httptest` (patrón ya usado en
`internal/resources/next_network_test.go`); aceptación manual/`make testacc` contra IPzilon 3.0.0
según [quickstart.md](./quickstart.md).

**Target Platform**: plugin de Terraform (binarios GoReleaser multi-plataforma)

**Project Type**: Terraform provider (cliente de la API REST de IPzilon)

**Performance Goals**: ≤ ~2 peticiones por recurso gestionado (SC-005); listados con
`limit=1000` (una /16 = 66 peticiones).

**Constraints**: reintentos acotados (≤ 10 min acumulados y ≤ 30 reintentos por petición, espera ≤ 60 s cada una, cancelables);
no calcular asignaciones en cliente (Principio II); HTTP solo en `internal/client`.

**Scale/Scope**: 2 ficheros de cliente, 6 data sources, 9 recursos (4 con cambios de lógica,
4 con validadores), docs y ejemplos regenerados.

## Constitution Check

*GATE: Must pass before Phase 0 research. Re-check after Phase 1 design.*

| Principio / norma | Evaluación | Estado |
|---|---|---|
| I. API como fuente de verdad | Filtros siguen en servidor (FR-004); validaciones nuevas son de forma (/8, IPv4, formato IP) y reflejan reglas que la API ya aplica; `Read` elimina del estado ante 404; no se exponen métricas de UI | ✅ |
| II. Asignación atómica en servidor | `next_*` siguen delegando en endpoints atómicos; solo cambia el mensaje de error; `force: true` se mantiene | ✅ |
| III. Compatibilidad de esquema y estado | Sin renombrados, eliminaciones ni atributos nuevos obligatorios; `items[*].id` ya era `Computed`. Único cambio incompatible de esquema: `status` de `ipzilon_ip_address`/`ipzilon_next_ip_address` deja de admitir `available` (FR-020); sin `UpgradeState` posible (no hay valor equivalente), se documenta el procedimiento manual (cambiar a `reserved`/`used` o destruir). MAJOR `v3.0.0` con *Breaking Changes* en `README.md`. Import intacto | ✅ |
| IV. Docs y ejemplos | `make generate` en el mismo PR; `~> 3.0` en ejemplos, `README.md` y `docs/index.md`; descripciones actualizadas; T018 y T024 completan las `Description` que faltaban (`items.subnet_id`, `id` de `ipzilon_ip_address`) | ✅ (tarea explícita) |
| V. Pruebas | Tests unitarios para `GetAll`, reintentos, detección de versión, helpers de error, validadores, alta de `ipzilon_ip_address` con `httptest` | ✅ (tarea explícita) |
| Restricciones técnicas | Solo plugin-framework; HTTP en `internal/client`; sin dependencias nuevas; `token` no se registra en logs de reintento | ✅ |
| Flujo | Rama `001-ipzilon-api-v3`; commits con `commit-message`; issue `[Feat] - Adaptación a IPzilon 3.0.0` si se crea | ✅ |

**Re-check post-diseño (Fase 1)**: sin cambios. La única decisión con riesgo, la validación /8,
se diseñó como validación solo al crear o cambiar el valor (R8) para no romper el `plan` de
objetos heredados, alineada con el Principio I. Sin violaciones → *Complexity Tracking* vacío.

## Project Structure

### Documentation (this feature)

```text
specs/001-ipzilon-api-v3/
├── spec.md
├── plan.md              # Este fichero
├── research.md          # Fase 0
├── data-model.md        # Fase 1
├── quickstart.md        # Fase 1
├── contracts/
│   ├── api-client.md       # Contrato provider ↔ API 3.0
│   └── terraform-schema.md # Interfaz pública del provider v3.0.0
├── checklists/requirements.md
└── tasks.md             # Fase 2 (/speckit-tasks)
```

### Source Code (repository root)

```text
internal/
├── client/
│   ├── client.go            # do(ctx) con reintentos 429/503; sonda /health con versión;
│   │                        # Get/Post/Patch/Delete(ctx, …); GetAll[T]; IsConflict,
│   │                        # IsSearchTruncated, ErrUnsupportedAPIVersion
│   ├── client_test.go       # NUEVO: paginación, reintentos, versión, errores
│   └── models.go            # Page[T], IPAddress.ID *int64, IPAddressRegister
├── datasources/
│   ├── sites.go hubs.go scopes.go networks.go subnets.go   # GetAll
│   └── ip_addresses.go      # GetAll + id nulable + descripción de coste
├── provider/provider.go     # Configure: error si IPzilon < 3.0.0
└── resources/
    ├── ip_address.go        # Create vía POST /subnets/{id}/ips; Delete vía DELETE; validadores IP y status
    ├── ip_address_test.go   # NUEVO: 201/409/400/404 con httptest
    ├── next_ip_address.go   # *ip.ID tras reserve-ip; Delete vía DELETE; status used|reserved
    ├── next_subnet.go last_subnet.go next_network.go   # IsSearchTruncated
    ├── hub.go scope.go network.go subnet.go             # validación /8 (+IPv4 en subnet)
    ├── cidr_validation.go   # NUEVO: lógica /8 e IPv4 al crear o cambiar
    ├── cidr_validation_test.go  # NUEVO
    └── helpers.go           # ipIDValue
docs/                        # make generate
examples/                    # ~> 3.0; nota de coste en ipzilon_ip_addresses
README.md                    # compatibilidad + Breaking Changes v3.0.0
```

**Structure Decision**: se mantiene la estructura exigida por la constitución
(`internal/client`, `internal/resources`, `internal/datasources`, `internal/provider`); solo se
añaden ficheros de test y `cidr_validation.go`.

## Orden de implementación sugerido

1. **Cliente** (bloqueante): `ctx` en firmas, reintentos, `GetAll`, `Page[T]`, `ID *int64`,
   sonda de versión, helpers de error, con sus tests. Ajuste mecánico de llamadores.
2. **US1**: 6 data sources a `GetAll`; `ipzilon_ip_addresses` con `id` nulo.
3. **US2**: `ipzilon_ip_address.Create` + tests; `next_ip_address` con `*ID`.
4. **US3**: queda cubierta por el paso 1; validación con V3 de quickstart.
5. **US4**: `IsSearchTruncated` en los 3 recursos `next_*`/`last_*`.
6. **US5**: `cidr_validation.go` y su conexión a los 4 recursos.
7. **Release**: README (compatibilidad, *Breaking Changes*, fijar IPzilon 2.3.1 / provider
   `~> 2.2`), ejemplos `~> 3.0`, `make generate`, quickstart V1–V7 contra IPzilon real.

## Riesgos

| Riesgo | Mitigación |
|---|---|
| El texto de los 409 cambia en IPzilon | Detección por prefijo `Search truncated`; test unitario con el texto actual; fallback: se muestra el mensaje crudo |
| `APP_VERSION` no semver en despliegues | No bloquear; segunda barrera en `GetAll` (R2) |
| Firma `ctx` toca muchos ficheros | Cambio mecánico en un commit aislado; `go build` lo valida |
| IPzilon 3.0.0 se publica antes que el provider | El informe se compromete a no publicarla sin confirmación (SC-011); README indica fijar `2.3.1` |

## Complexity Tracking

Sin violaciones de la constitución.
