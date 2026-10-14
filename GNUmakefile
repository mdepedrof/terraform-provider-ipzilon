GOBIN ?= $(shell go env GOPATH)/bin
BINARY = terraform-provider-ipzilon

.PHONY: build install fmt test testacc testmock generate

build:
	go build -o $(BINARY) .

install: build
	mkdir -p $(GOBIN)
	cp $(BINARY) $(GOBIN)/

fmt:
	gofmt -s -w .

test:
	go test ./...

testacc:
	TF_ACC=1 go test ./internal/... -v -timeout 600s

# Run terraform test with mock_provider in test/mock/ against a fresh build of
# the provider (dev_overrides, no live API). Requires terraform in PATH.
testmock:
	@set -e; tmp=$$(mktemp -d); trap 'rm -rf "$$tmp"' EXIT; \
	go build -o "$$tmp/$(BINARY)" .; \
	printf 'provider_installation {\n  dev_overrides {\n    "registry.terraform.io/mdepedrof/ipzilon" = "%s"\n  }\n  direct {}\n}\n' "$$tmp" > "$$tmp/terraformrc"; \
	TF_CLI_CONFIG_FILE="$$tmp/terraformrc" terraform -chdir=test/mock test

# Regenerate docs/. Run after changing any resource/datasource schema.
generate:
	go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
	tfplugindocs generate --provider-name ipzilon
