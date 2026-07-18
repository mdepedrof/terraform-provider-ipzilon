GOBIN ?= $(shell go env GOPATH)/bin
BINARY = terraform-provider-ipzilon

.PHONY: build install fmt test testacc generate

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
	TF_ACC=1 go test ./internal/... -v -timeout 120s

# Regenerate docs/. Run after changing any resource/datasource schema.
generate:
	go install github.com/hashicorp/terraform-plugin-docs/cmd/tfplugindocs@latest
	tfplugindocs generate --provider-name ipzilon
