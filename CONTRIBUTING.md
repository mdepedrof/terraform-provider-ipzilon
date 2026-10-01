# Contributing

## Requirements

- Go 1.25+
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

## New resources

Every resource MUST leave a complete state after `terraform import` (constitution, principle III):

1. Build the state only from the API object with one `<x>FromAPI` function and use it in `Create`,
   `Read` and `Update` (never copy fields one by one onto the previous state).
2. Derive from the API the attributes it does not return, with the shared helper
   (`prefix_length` comes from `cidr` through `prefixLengthValue`); return the error instead of
   leaving the attribute empty.
3. Add a row to `importCases` in `internal/resources/read_after_import_test.go`.
   `TestReadAfterImportCoversAllResources` fails if a registered resource has no row.
4. Add a `TestAccImport_*` case in `internal/resources/acc_import_test.go`.
5. Data sources never read the previous state (`grep -n "req.State" internal/datasources/*.go`
   must return nothing).

## Dependencies

The provider only uses `terraform-plugin-framework` (constitution, technical constraints).
`terraform-plugin-testing`, used only in `_test.go` files for the acceptance tests, depends on
`terraform-plugin-sdk/v2`, which therefore appears as an *indirect* dependency in `go.mod`. It is
accepted only as a transitive test dependency: no provider code may import it, and it must not be
linked into the binary (`go list -deps . | grep terraform-plugin-sdk` must return nothing).

## Pull requests

- One PR per logical change
- Include a description of what changed and why
- Run `make test` and `make generate` before opening the PR
- The CI workflow must pass

## Releasing

Before tagging, run the acceptance tests against a real IPzilon of the latest supported release
(>= 3.2.0). They create, import and plan every resource (import → empty plan), look objects up
without ids and move a hub between sites, and must all pass:

```bash
export IPZILON_API_URL=... IPZILON_TOKEN=...
export IPZILON_TEST_SITE_ID=<existing site id>
export IPZILON_TEST_ADDRESS_SPACE=10.250.0.0/16   # free IPv4 /16
export IPZILON_TEST_ALT_SITE_ID=<id of a second site of the same type>   # optional
make test && make testacc
```

Without `IPZILON_TEST_ALT_SITE_ID`, `TestAccHub_MoveSite` is skipped.

Checklist: `make test` green, `make testacc` green, `make generate` leaves `docs/` unchanged
(unless the schema changed), and no unexpected entry in *Breaking Changes*.

Releases are created by pushing a version tag:

```bash
git tag v1.2.3
git push origin v1.2.3
```

The release workflow builds binaries for all supported platforms, signs them with GPG, and creates a GitHub release. The Terraform Registry picks it up automatically.
