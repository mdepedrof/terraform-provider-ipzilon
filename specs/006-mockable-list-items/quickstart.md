# Quickstart: validar que `items` es mockeable

**Feature**: [spec.md](spec.md)

## Requisitos

- Go (versión de `go.mod`) y Terraform >= 1.16.0 en el `PATH`.
- No hace falta una instancia de IPzilon.

## 1. Tests unitarios (FR-008, FR-009)

```bash
go build ./... && go vet ./... && go test ./...
```

Esperado: todo pasa, incluido el test que comprueba que `items` no es un atributo anidado en los 7
data sources.

## 2. `terraform test` con mocks (FR-001, FR-006)

```bash
make testmock
```

Compila el provider, lo usa con `dev_overrides` y ejecuta `terraform test` en `test/mock/`.
Esperado: `Success! N passed, 0 failed.` cubriendo, para los 7 data sources, `override_data` con un
elemento parcial, con varios elementos y con lista vacía, y `mock_data`.

Regresión manual: con la rama `main` (antes del cambio), el mismo comando falla con
`Failed to compute attribute ".items": incompatible types; expected object type, found tuple.`

## 3. Reproducción de la issue (SC-001)

El caso `override_data` con `items = [{ id = 1 }]` y la postcondición `length(self.items) == 1`
sobre `ipzilon_hubs` forma parte de `test/mock/`; pasa en el paso 2.

## 4. Documentación (FR-010, SC-003)

```bash
make generate && git diff --stat docs/
```

Esperado: cambian solo los 7 ficheros de `docs/data-sources/`; en cada uno, `items` aparece como
`(List of Object)` y su descripción nombra todos los campos (ver
[contracts/items-attribute.md](contracts/items-attribute.md)). Un segundo `make generate` no
produce diferencias.

## 5. Sin cambios para consumidores (US2, SC-002)

Con una instancia local de IPzilon y un stack con estado creado por v3.2.1 que lea los 7 data
sources: instalar el binario nuevo (`make install` + `dev_overrides`) y ejecutar
`terraform plan`. Esperado: `No changes.` y los mismos valores en los outputs.
