# Quickstart: validar la corrección

**Feature**: [spec.md](./spec.md) | Contratos: [read-after-import.md](./contracts/read-after-import.md)

## 1. Unitarios (sin API)

```bash
make test           # go test ./...
go vet ./... && gofmt -l .
```

Esperado: verde. `TestReadAfterImport` cubre los nueve recursos; `TestReadAfterImportCoversAllResources`
falla si se registra un recurso sin fila.

## 2. Comprobar que el test protege (regresión deliberada)

Comentar temporalmente el relleno de `subnet_id` en `nextIPFromAPI` y ejecutar
`go test ./internal/resources -run ReadAfterImport`. Esperado: falla con
`ipzilon_next_ip_address.subnet_id`. Revertir.

## 3. Aceptación contra un IPzilon real (antes de cada release)

```bash
export IPZILON_API_URL=... IPZILON_TOKEN=...   # instancia de pruebas
export IPZILON_TEST_SITE_ID=<id de un site existente>
export IPZILON_TEST_ADDRESS_SPACE=10.250.0.0/16  # /16 IPv4 libre
make testacc
```

Esperado: para los 9 recursos, crear → importar → plan vacío.

## 4. Caso real de `camaras`

En el stack consumidor, con el provider local (`dev_overrides`):

```bash
terraform import ipzilon_next_ip_address.ip_events_ilb 2204
terraform plan      # Esperado: sin cambios y sin "must be replaced"
```

Si aparece un diff in situ de `hostname`/`description`, ver R6 de [research.md](./research.md).

## 5. Release

Checklist en `CONTRIBUTING.md`: pasos 1 y 3 en verde → tag `v3.0.1` (GoReleaser).
