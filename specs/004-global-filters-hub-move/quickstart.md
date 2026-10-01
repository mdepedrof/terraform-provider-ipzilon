# Quickstart: validar la feature 004

Guía de validación de extremo a extremo. Esquema en
[contracts/terraform-schema.md](./contracts/terraform-schema.md); rutas en
[contracts/api-client.md](./contracts/api-client.md).

## Requisitos

- Go según `go.mod`, Terraform ≥ 1.5.
- Instancia de IPzilon **3.2.0** con un token y **dos sites del mismo tipo** con espacio libre.
- Variables: `IPZILON_API_URL`, `IPZILON_TOKEN`, `IPZILON_TEST_SITE_ID`,
  `IPZILON_TEST_ADDRESS_SPACE` (p. ej. `10.250.0.0/16`) e `IPZILON_TEST_ALT_SITE_ID` (nueva).
- Opcional: una instancia 3.1.x para comprobar los errores de versión.

## 1. Verificación automática

```bash
go build ./... && go vet ./... && go test ./...
make generate && git diff --exit-code docs/    # docs al día
go list -deps . | grep terraform-plugin-sdk     # debe salir vacío
TF_ACC=1 make testacc                           # contra IPzilon 3.2.0
```

Resultado esperado: todo en verde; `TestAccHub_MoveSite` y `TestAccDataSources_GlobalLookup` se
ejecutan (no `SKIP`) cuando están definidas las variables.

## 2. Escenarios manuales (provider local con `dev_overrides`)

| # | Configuración | Resultado esperado | Spec |
|---|---|---|---|
| 1 | `ipzilon_networks { name = "no-existe" }` + `output = length(...items)` | `0`, sin error | US1, FR-001 |
| 2 | Igual para `sites`, `hubs`, `scopes`, `ip_addresses` (`status = "reserved"` en subred vacía) | `[]` en todos | US1 |
| 3 | `ipzilon_ip_addresses { subnet_id = S, address = <ocupada> }` | 1 elemento con `id` | US2-1 |
| 4 | Igual con una dirección libre | 1 elemento, `id = null`, `status = "available"` | US2-2 |
| 5 | Igual con una dirección de otra subred | error `… is not an address of subnet S` | US2-3, FR-005 |
| 6 | `address` sin `subnet_id`, o con `status` | falla en `terraform validate`/`plan` | US2-4 |
| 7 | `ipzilon_networks { cidr = "<cidr de una network>" }` | esa network, sin ids | US3-1 |
| 8 | `ipzilon_hubs { address_space = ... }`, `ipzilon_scopes { kind, name }`, `ipzilon_subnets { cidr }` | el elemento esperado | US3-2..4 |
| 9 | `ipzilon_networks { cidr = "10.0.16.5/22" }` | falla en `plan` sugiriendo `10.0.16.0/22` | US3-8 |
| 10 | `ipzilon_scopes { root_only = true }` sin `hub_id` | falla en `plan` | FR-013 |
| 11 | Configuraciones existentes con ids de padre, contra 3.1.x y 3.2.0 | mismos resultados | US3-11, FR-016 |
| 12 | Escenario 7 contra IPzilon 3.1.x | error `… requires IPzilon >= 3.2.0` | US3-10 |
| 13 | Hub gestionado con scope/network/subnet; cambiar `site_id` al site alternativo | `plan`: `~ update in-place` del hub, 0 a destruir; `apply` OK; mismo id; `plan` siguiente vacío | US4-1, US4-2 |
| 14 | Mover a un site inexistente | `apply` falla con `Site not found`; el hub no cambia | US4-3 |
| 15 | Escenario 13 contra IPzilon 3.1.x | falla en `plan` con la versión mínima | US4-4 |
