SHELL := /bin/bash
.DEFAULT_GOAL := build

BIN			= $(CURDIR)/bin
BUILD_DIR	= $(CURDIR)/build

GOPATH			= $(HOME)/go
GOBIN			= $(GOPATH)/bin
GO				?= GOGC=off $(shell which go)
TINYGO			?= $(shell which tinygo)
NODE			?= $(shell which node)
PNPM			?= $(shell which pnpm)
PKGS			= $(or $(PKG),$(shell env $(GO) list ./...))
VERSION			?= $(shell git describe --tags --always --match=v*)
SHORT_COMMIT	?= $(shell git rev-parse --short HEAD)

PATH := $(GOBIN):$(BIN):$(PATH)

EMBEDDED =
LDFLAGS	= -w -s -X "github.com/lunogram/services/nexus/internal/build.version=$(VERSION)" -X "github.com/lunogram/services/nexus/internal/build.commit=$(SHORT_COMMIT)"

# Printing
V ?= 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m▶\033[0m")

$(BUILD_DIR):
	@mkdir -p $@

PROVIDER_MODULES := $(notdir $(wildcard ./modules/providers/*))

$(PROVIDER_MODULES):
	$(info $(M) building $@ module…)
	$Q cd modules/providers/$@ &&  $(TINYGO) build -target=wasi -buildmode c-shared -opt=2 -no-debug -o ../../../services/nexus/internal/providers/modules/$@.wasm ./main.go
# Tools
$(BIN):
	@mkdir -p $@
$(BIN)/%: | $(BIN) ; $(info $(M) building $(@F)…)
	$Q GOBIN=$(BIN) $(GO) install $(shell $(GO) list tool | grep $(@F))

$(EMBEDDED):
	$Q mkdir -p $(shell dirname $@)
	$Q touch $@

GOLANGCI_LINT = $(BIN)/golangci-lint
STRINGER = $(BIN)/stringer
MINIMOCK = $(BIN)/minimock
OAPI_CODEGEN = $(BIN)/oapi-codegen

TOOLCHAIN = $(STRINGER) $(MINIMOCK) $(OAPI_CODEGEN)

.PHONY: build # Build all services
build: $(PROVIDER_MODULES)
	@true

# Targets
.PHONY: lint
lint: | $(EMBEDDED) $(GOLANGCI_LINT) $(BUF) ; $(info $(M) running linters…) @ ## Run the project linters
	$Q $(PNPM) run lint
	$Q $(GOLANGCI_LINT) run --max-issues-per-linter 10 --timeout 5m

.PHONY: test
test: | $(EMBEDDED) ; $(info $(M) running tests) @ ## Run all tests
	$Q $(GO) test $(PKGS) -timeout 300s -race -count 1

.PHONY: test-short
test-short: | $(EMBEDDED) ; $(info $(M) running short tests) @ ## Run all short tests
	$Q $(GO) test $(PKGS) -timeout 120s -race -count 1 -short

.PHONY: fmt
fmt: | $(EMBEDDED) ; $(info $(M) running go fmt…) @ ## Run gofmt on all source files
	$Q $(GO) fmt $(PKGS)

.PHONY: generate
generate: | $(EMBEDDED) $(TOOLCHAIN) ; $(info $(M) running go generate…) @ ## Run gogenerate on all source files
	$Q $(GO) generate $(PKGS)
	$Q $(MAKE) fmt

.PHONY: clean
clean: ; $(info $(M) cleaning…)	@ ## Cleanup everything
	@rm -rf $(BIN)
	@rm -rf $(BUILD)
	@find . -name '*_mock_test.go' -exec rm -r {} \;
	@find . -name '*_string.go' -exec rm -r {} \;
	@find . -name '*_gen.go' -exec rm -r {} \;
	@find . -name '*.sql.go' -exec rm -r {} \;
	@find . -name '*.wasm' -exec rm -r {} \;

.PHONY: help
help:
	@grep -E '^[ a-zA-Z_-]+:.*?## .*$$' $(MAKEFILE_LIST) | \
		awk 'BEGIN {FS = ":.*?## "}; {printf "\033[36m%-15s\033[0m %s\n", $$1, $$2}'
