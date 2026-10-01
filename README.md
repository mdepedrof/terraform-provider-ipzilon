# Terraform Provider for IPzilon

Manage IP address space in [IPzilon](https://github.com/mdepedrof/ipzilon) — an IPAM for Azure and on-premise networks — using Terraform.

## Resources

| Resource | Description |
|---|---|
| `ipzilon_hub` | Hub VNet inside a site |
| `ipzilon_scope` | Scope (landing zone or project) inside a hub — supports parent/child hierarchy up to 4 levels |
| `ipzilon_network` | VNet/spoke CIDR inside a scope |
| `ipzilon_subnet` | Subnet with explicit CIDR |
| `ipzilon_next_subnet` | Atomically reserves the **first** free block of a given prefix |
| `ipzilon_last_subnet` | Atomically reserves the **last** free block of a given prefix |
| `ipzilon_ip_address` | Marks a specific IP as used/reserved |
| `ipzilon_next_ip_address` | Atomically reserves the **next available** IP in a subnet |
| `ipzilon_network_zone` | Named zone inside a network that groups the subnets it contains (IPzilon >= 3.1.0) |
| `ipzilon_next_network_zone` | Atomically reserves the **first** empty block of a given prefix as a zone (IPzilon >= 3.1.0) |
| `ipzilon_last_network_zone` | Atomically reserves the **last** empty block of a given prefix as a zone (IPzilon >= 3.1.0) |

## Data Sources

`ipzilon_hubs` · `ipzilon_scopes` · `ipzilon_networks` · `ipzilon_subnets` · `ipzilon_ip_addresses` · `ipzilon_network_zones`

## Requirements

- Terraform ≥ 1.0
- Go ≥ 1.25.8 (to build from source)
- A running [IPzilon](https://github.com/mdepedrof/ipzilon) instance (>= 3.0.0 for provider 3.x, see [Compatibility](#compatibility))

## Usage

```hcl
terraform {
  required_providers {
    ipzilon = {
      source  = "registry.terraform.io/mdepedrof/ipzilon"
      version = "~> 3.0"
    }
  }
}

provider "ipzilon" {
  api_url = "https://ipzilon.example.com"  # or env: IPZILON_API_URL
  token   = var.ipzilon_token            # or env: IPZILON_TOKEN
}
```

## Example

```hcl
resource "ipzilon_hub" "prod" {
  site_id       = 1
  name          = "hub-prod"
  address_space = "10.0.0.0/16"
}

resource "ipzilon_scope" "app" {
  hub_id = ipzilon_hub.prod.id
  name   = "app-tier"
  kind   = "landing_zone"
}

resource "ipzilon_network" "spoke" {
  scope_id = ipzilon_scope.app.id
  name     = "spoke-app"
  cidr     = "10.0.1.0/24"
}

# Reserve the first available /27
resource "ipzilon_next_subnet" "web" {
  network_id    = ipzilon_network.spoke.id
  prefix_length = 27
  name          = "web-tier"
}

# Reserve the next available IP in that subnet
resource "ipzilon_next_ip_address" "vm" {
  subnet_id = ipzilon_next_subnet.web.id
  hostname  = "app-vm-01"
}

output "vm_ip" {
  value = ipzilon_next_ip_address.vm.address
}
```

A full example with all resources and data sources is in [`examples/terraform/`](examples/terraform/).

## Network zones

Since provider v3.1.0 (requires IPzilon >= 3.1.0), a network can be split into
named zones that group subnets by CIDR containment:

```hcl
resource "ipzilon_network_zone" "pooled" {
  network_id = ipzilon_network.spoke.id
  name       = "pooled_zone_1"
  cidr       = "10.0.1.0/25"
}

# Allocate the first free /28 inside the zone
resource "ipzilon_next_subnet" "hp" {
  network_id    = ipzilon_network.spoke.id
  zone_id       = ipzilon_network_zone.pooled.id
  prefix_length = 28
  name          = "hp-pooled-01"
}

# Find a zone without knowing any id
data "ipzilon_network_zones" "pooled" {
  cidr = "10.0.1.0/25"
}
```

- `zone_id` on `ipzilon_subnet`, `ipzilon_next_subnet` and `ipzilon_last_subnet`
  always reflects the zone that contains the subnet according to IPzilon, and
  never forces a replacement.
- In a network that has zones, `ipzilon_next_subnet`/`ipzilon_last_subnet`
  without `zone_id` allocate **outside** every zone.
- Destroying a zone does not delete its subnets; they are left outside any zone.
- Using zones against IPzilon 3.0.x fails in `plan` with
  `... requires IPzilon >= 3.1.0`; configurations without zones keep working.

## Lookups without ids

Since provider v3.2.0 (requires IPzilon >= 3.2.0), `ipzilon_hubs`,
`ipzilon_scopes`, `ipzilon_networks` and `ipzilon_subnets` find objects by
their properties alone, without the id of any parent. The search runs in
IPzilon:

```hcl
data "ipzilon_networks" "avd" {
  cidr = "10.0.16.0/22"
}

resource "ipzilon_next_subnet" "hosts" {
  network_id    = one(data.ipzilon_networks.avd.items).id
  prefix_length = 26
  name          = "snet-hosts"
}

# The id of an IP changes when it is released and occupied again: look it up
# by address (works with IPzilon >= 3.0.0).
data "ipzilon_ip_addresses" "gw" {
  subnet_id = 42
  address   = "10.0.1.17"
}
```

- Without a parent id, `cidr`/`address_space` compare the **network**, so the
  value must have no host bits (`10.0.16.5/22` fails in `plan` suggesting
  `10.0.16.0/22`). With a parent id the filters behave as before.
- The same CIDR may exist in several hubs or sites: a lookup by CIDR alone can
  return several items. Use `one()` when you expect exactly one, or add a
  parent id to narrow it down.
- Every listing data source returns `items = []` (never `null`) when nothing
  matches.
- A lookup without parent ids against IPzilon < 3.2.0 fails with
  `... requires IPzilon >= 3.2.0`; lookups with parent ids keep working.

Also since v3.2.0, changing `site_id` of an `ipzilon_hub` moves the hub to the
new site **in place**, keeping its id and everything under it (IPzilon >=
3.2.0); against an older IPzilon it fails in `plan`.

## Compatibility

| Provider | IPzilon |
|----------|---------|
| `~> 3.0` | >= 3.0.0 (network zones: >= 3.1.0, provider >= 3.1.0; lookups without parent ids and moving hubs between sites: >= 3.2.0, provider >= 3.2.0) |
| `~> 2.2` | 2.x (up to 2.3.1) |

The provider checks the version reported by IPzilon's `/health` endpoint and
fails with `Unsupported IPzilon version` when it is older than the one it
supports.

Requests rejected by IPzilon with `429` (rate limit) or `503` (server busy) are
retried automatically, honouring `Retry-After`, for up to 10 minutes per
request. Each API token has its own rate-limit quota, so use one token per
pipeline if runs should not share it.

## Breaking Changes (v3.0.0)

Version 3.0.0 adapts the provider to IPzilon 3.0.0, which paginates listings,
stores only occupied/reserved/annotated IP addresses and enforces a
rate limit. It **does not work with IPzilon 2.x**.

### 1. IPzilon >= 3.0.0 required

Upgrade in this order:

1. Until you migrate, pin the IPzilon image to `2.3.1` (not `:latest`) and the
   provider to `~> 2.2`.
2. Migrate IPzilon to 3.0.0.
3. Upgrade the provider to `~> 3.0`. Existing state keeps working as is (the
   IDs of occupied/reserved addresses are preserved by the IPzilon migration),
   except for the `status = "available"` case below.

### 2. `status` on `ipzilon_ip_address` / `ipzilon_next_ip_address`

`status` only accepts `used` or `reserved`. A free address has no record in
IPzilon 3.0, so `status = "available"` is rejected at plan time: to free an
address, destroy the resource; to block it, use `reserved`. If a configuration
sets `available`, change it to `reserved`/`used` or remove the resource before
upgrading.

Destroying either resource now releases the address with `DELETE /ips/{id}`:
the record is deleted and the address becomes free again. A re-occupied
address gets a new ID.

### 3. `ipzilon_ip_addresses`: `id` can be null

Free addresses are listed with `id = null`. Without `status`, the data source
lists the whole subnet (e.g. 65,536 items for a /16, fetched in pages of
1000); set `status` to limit the cost. Looking up a released address by `id`
fails with `IP not found`.

### 4. CIDR validation at plan time

Hubs (`address_space`), scopes, networks and subnets (`cidr`) larger than `/8`
and IPv6 subnets are rejected at plan time when created or changed, mirroring
IPzilon 3.0. Existing objects larger than `/8` keep working while their CIDR
does not change.

## Breaking Changes (v2.0.0)

Version 2.0.0 follows a backend rename: the "landing zone" container concept
is now called **scope**, and scopes support a `kind` of `landing_zone` or
`project`, nestable up to 4 levels (previously 2).

### 1. `ipzilon_landing_zone` → `ipzilon_scope`

The resource type was renamed. Existing state must be moved manually:

```bash
terraform state mv 'ipzilon_landing_zone.app' 'ipzilon_scope.app'
```

Repeat for every `ipzilon_landing_zone` instance in your state, then update
your `.tf` files to use `resource "ipzilon_scope" "app" { ... }`.

### 2. `kind` is now required on `ipzilon_scope`

Every `ipzilon_scope` block must set `kind = "landing_zone"` or
`kind = "project"` explicitly — there is no provider-side default, even
though the API defaults to `landing_zone`. Add `kind` to all existing
configurations before running `terraform plan`.

Rules enforced by the backend:
- A root-level scope (no `parent_id`) must be `kind = "landing_zone"`.
- A `landing_zone` cannot be nested under a `project` — once a branch enters
  `kind = "project"`, everything nested below it must also be `project`.

### 3. `ipzilon_network`: `landing_zone_id` → `scope_id`

The attribute was renamed. **State is migrated automatically** — the
provider implements Terraform's state upgrade mechanism, so existing
`ipzilon_network` resources will transparently move from
`landing_zone_id` to `scope_id` in state on the next plan/apply, no
`terraform state mv` or re-import required.

However, you must still **edit your `.tf` files by hand**: replace
`landing_zone_id = ...` with `scope_id = ...` in every `ipzilon_network`
block, or `terraform plan` will report `scope_id` as required/missing.

### 4. Data sources

- `ipzilon_landing_zones` → `ipzilon_scopes` (now also exposes/filters by
  `kind`).
- `ipzilon_networks`: filter attribute `landing_zone_id` → `scope_id`.

### 5. Max nesting depth: 2 → 4

Scopes can now be nested up to 4 levels deep (previously 2), subject to the
`landing_zone`/`project` nesting rule above.

## Importing existing resources

Every resource supports `terraform import` with the numeric IPzilon id
(`terraform import ipzilon_next_ip_address.app_vm 2204`). `Read` fills every attribute from the API,
so once your configuration matches the imported object the next `plan` shows no changes and never
proposes a replacement.

IPs migrated from another IPAM can come back from the API with `hostname` equal to `description`
(for example id 2204). If your configuration sets a different `hostname`, the first `plan` shows an
in-place update of `hostname`; it is a real difference and converges after one `apply`. To avoid it,
set `hostname` explicitly to the value stored in IPzilon.

## Documentation

Full attribute reference and import syntax: [registry.terraform.io/providers/mdepedrof/ipzilon](https://registry.terraform.io/providers/mdepedrof/ipzilon/latest/docs)

## Development

```bash
# Build and install locally
make install

# Configure Terraform to use the local binary (~/.terraformrc)
# provider_installation { dev_overrides { "registry.terraform.io/mdepedrof/ipzilon" = "/path/to/go/bin" } direct {} }

make test        # unit tests
make testacc     # acceptance tests (requires a live API)
make generate    # regenerate docs/ after schema changes
```

See [CONTRIBUTING.md](CONTRIBUTING.md) for the full development guide.

## Releasing

```bash
git tag v3.0.0
git push origin v3.0.0
```

The release workflow builds binaries for Linux, macOS (Apple Silicon), and Windows, signs them with GPG, and publishes a GitHub release. The Terraform Registry picks it up automatically.

## Security

To report a vulnerability, see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
