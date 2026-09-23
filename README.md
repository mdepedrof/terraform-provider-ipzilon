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

## Data Sources

`ipzilon_hubs` · `ipzilon_scopes` · `ipzilon_networks` · `ipzilon_subnets` · `ipzilon_ip_addresses`

## Requirements

- Terraform ≥ 1.0
- Go ≥ 1.25.8 (to build from source)
- A running [IPzilon](https://github.com/mdepedrof/ipzilon) instance

## Usage

```hcl
terraform {
  required_providers {
    ipzilon = {
      source  = "registry.terraform.io/mdepedrof/ipzilon"
      version = "~> 2.0"
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
git tag v2.0.0
git push origin v2.0.0
```

The release workflow builds binaries for Linux, macOS (Apple Silicon), and Windows, signs them with GPG, and publishes a GitHub release. The Terraform Registry picks it up automatically.

## Security

To report a vulnerability, see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
