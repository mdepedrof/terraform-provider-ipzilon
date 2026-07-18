GOBIN ?= $(shell go env GOPATH)/bin
BINARY = terraform-provider-ipzilon

.PHONY: build install fmt testacc

build:
	go build -o $(BINARY) .

install: build
	mkdir -p $(GOBIN)
	cp $(BINARY) $(GOBIN)/

fmt:
	gofmt -s -w .

testacc:
	TF_ACC=1 go test ./internal/... -v -timeout 120s
