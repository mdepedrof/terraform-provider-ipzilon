# Contributing

## Requirements

- Go 1.24+
- Terraform CLI 1.x
- A running IPzilon API (see [ipzilon repo](https://github.com/mdepedrof/ipzilon))

## Local setup

```bash
git clone https://github.com/mdepedrof/terraform-provider-ipzilon
cd terraform-provider-ipzilon
go mod download
```

Install the provider locally for development:

```bash
make install
```

Then configure `~/.terraformrc` to use the local binary:

```hcl
provider_installation {
  dev_overrides {
    "registry.terraform.io/mdepedrof/ipzilon" = "/path/to/go/bin"
  }
  direct {}
}
```

With `dev_overrides` active, skip `terraform init` — go directly to `terraform plan`.

## Development workflow

```bash
make build      # compile
make test       # unit tests
make testacc    # acceptance tests (requires a live API — set IPZILON_API_URL and IPZILON_TOKEN)
make fmt        # format code
make generate   # regenerate docs/ from schema
```

## Changing a resource or data source

1. Edit the schema in `internal/resources/` or `internal/datasources/`
2. Run `make generate` to regenerate `docs/`
3. Commit both the schema change and the updated docs together

The CI workflow will fail if `docs/` is out of sync with the schema.

## Pull requests

- One PR per logical change
- Include a description of what changed and why
- Run `make test` and `make generate` before opening the PR
- The CI workflow must pass

## Releasing

Releases are created by pushing a version tag:

```bash
git tag v1.2.3
git push origin v1.2.3
```

The release workflow builds binaries for all supported platforms, signs them with GPG, and creates a GitHub release. The Terraform Registry picks it up automatically.
