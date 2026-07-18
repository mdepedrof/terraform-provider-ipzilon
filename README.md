# Terraform Provider for IPzilon

Manage IP address space in [IPzilon](https://github.com/mdepedrof/ipzilon) — an IPAM for Azure and on-premise networks — using Terraform.

## Resources

| Resource | Description |
|---|---|
| `ipzilon_hub` | Hub VNet inside a site |
| `ipzilon_landing_zone` | Landing zone inside a hub (supports parent/child hierarchy) |
| `ipzilon_network` | VNet/spoke CIDR inside a landing zone |
| `ipzilon_subnet` | Subnet with explicit CIDR |
| `ipzilon_next_subnet` | Atomically reserves the **first** free block of a given prefix |
| `ipzilon_last_subnet` | Atomically reserves the **last** free block of a given prefix |
| `ipzilon_ip_address` | Marks a specific IP as used/reserved |
| `ipzilon_next_ip_address` | Atomically reserves the **next available** IP in a subnet |

## Data Sources

`ipzilon_hubs` · `ipzilon_landing_zones` · `ipzilon_networks` · `ipzilon_subnets` · `ipzilon_ip_addresses`

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
      version = "~> 1.0"
    }
  }
}

provider "ipzilon" {
  api_url = "https://ipam.example.com"   # or env: IPZILON_API_URL
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

resource "ipzilon_landing_zone" "app" {
  hub_id = ipzilon_hub.prod.id
  name   = "app-tier"
}

resource "ipzilon_network" "spoke" {
  landing_zone_id = ipzilon_landing_zone.app.id
  name            = "spoke-app"
  cidr            = "10.0.1.0/24"
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
git tag v1.0.0
git push origin v1.0.0
```

The release workflow builds binaries for Linux, macOS (Apple Silicon), and Windows, signs them with GPG, and publishes a GitHub release. The Terraform Registry picks it up automatically.

## Security

To report a vulnerability, see [SECURITY.md](SECURITY.md).

## License

[MIT](LICENSE)
