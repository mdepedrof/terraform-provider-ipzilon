# Data Model: Hacer mockeable `items` de los data sources de listado

**Feature**: [spec.md](spec.md) | **Date**: 2026-10-14

No cambia ningún modelo de la API ni del estado. Cambia cómo se declara `items` en el esquema de
los 7 data sources. Los campos, sus tipos y su orden son los actuales.

## Definición de campo de elemento (`itemField`)

| Campo | Tipo | Descripción |
|-------|------|-------------|
| `name` | string | Nombre del atributo dentro del elemento; MUST coincidir con la etiqueta `tfsdk` del struct del modelo. |
| `typ` | `attr.Type` | `types.Int64Type`, `types.StringType` o `types.BoolType`. |
| `description` | string | Texto actual de la descripción del campo (se conserva literal). |

Cada data source declara una lista ordenada de `itemField`; de ella se derivan:

- el `ElementType` de `items`: `types.ObjectType{AttrTypes: {name: typ, ...}}`;
- la descripción de `items`: el resumen actual + `Each item has: ` + `` `name` — description ``
  por cada campo, en el orden declarado.

## Campos por data source (orden de declaración)

| Data source | Campos |
|-------------|--------|
| `ipzilon_hubs` | `id` (Int64), `site_id` (Int64), `name` (String), `address_space` (String), `location` (String), `description` (String) |
| `ipzilon_ip_addresses` | `id` (Int64), `subnet_id` (Int64), `address` (String), `status` (String), `is_azure_reserved` (Bool), `hostname` (String), `description` (String) |
| `ipzilon_network_zones` | `id` (Int64), `network_id` (Int64), `name` (String), `cidr` (String), `description` (String) |
| `ipzilon_networks` | `id` (Int64), `scope_id` (Int64), `name` (String), `cidr` (String), `description` (String) |
| `ipzilon_scopes` | `id` (Int64), `hub_id` (Int64), `parent_id` (Int64), `name` (String), `kind` (String), `cidr` (String), `description` (String) |
| `ipzilon_sites` | `id` (Int64), `name` (String), `type` (String), `description` (String) |
| `ipzilon_subnets` | `id` (Int64), `network_id` (Int64), `name` (String), `cidr` (String), `description` (String), `zone_id` (Int64) |

## Reglas

- Los structs de elemento (`hubItem`, `ipAddrItem`, …) y su conversión desde la API no cambian;
  el framework los convierte en objetos igual que ahora.
- Un campo declarado sin campo `tfsdk` equivalente en el struct (o al revés) es un error que
  detectan los tests de `Read` existentes, porque `State.Set` falla.
- `items` sigue siendo `Computed`, nunca nulo: lista vacía cuando no hay coincidencias.
