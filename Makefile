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
LDFLAGS	= -w -s -X "github.com/lunogram/platform/internal/build.version=$(VERSION)" -X "github.com/lunogram/platform/internal/build.commit=$(SHORT_COMMIT)"

# Printing
V ?= 0
Q = $(if $(filter 1,$V),,@)
M = $(shell printf "\033[34;1m▶\033[0m")

$(BUILD_DIR):
	@mkdir -p $@

MODULES := $(notdir $(shell find ./modules -mindepth 1 -maxdepth 1 -type d))

# Tools
$(BIN):
	@mkdir -p $@
$(BIN)/%: | $(BIN) ; $(info $(M) building $(@F)…)
	$Q GOBIN=$(BIN) $(GO) install $(shell $(GO) list tool | grep $(@F))

$(BIN)/tailwindcss: | $(BIN) ; $(info $(M) building tailwindcss…)
	$Q ./etc/install-tailwindcss.sh $(BIN)

$(EMBEDDED):
	$Q mkdir -p $(shell dirname $@)
	$Q touch $@

GOLANGCI_LINT = $(BIN)/golangci-lint
STRINGER = $(BIN)/stringer
MINIMOCK = $(BIN)/minimock
OAPI_CODEGEN = $(BIN)/oapi-codegen
TAILWINDCSS = $(BIN)/tailwindcss

TOOLCHAIN = $(STRINGER) $(MINIMOCK) $(OAPI_CODEGEN) $(TAILWINDCSS)

.PHONY: build
build: modules console lunogram ## Build all services

.PHONY: lunogram
lunogram: ; $(info $(M) building lunogram…)
	$Q CGO_ENABLED=0 $(GO) build -ldflags='$(LDFLAGS)' -o $(BIN)/lunogram ./cmd/lunogram

.PHONY: modules
modules: ; $(info $(M) building WASM modules…) @ ## Build all WASM modules
	$Q mkdir -p internal/integrations/modules
	$Q set -e; for module in $(MODULES); do \
		printf "$(M) building %s…\n" "$$module"; \
		$(MAKE) -C modules/$$module wasm TINYGO=$(TINYGO) NODE=$(NODE); \
	done

.PHONY: console
console: ; $(info $(M) building console…)
	$Q cd console && $(PNPM) run build
	$Q rm -rf internal/http/console/dist/*
	$Q cp -r console/dist/* internal/http/console/dist/

# Targets
.PHONY: lint
lint: | $(EMBEDDED) $(GOLANGCI_LINT) $(BUF) ; $(info $(M) running linters…) @ ## Run the project linters
	$Q cd console && $(PNPM) run lint
	$Q $(GOLANGCI_LINT) run --max-issues-per-linter 10 --timeout 5m

.PHONY: test
test: | $(EMBEDDED) ; $(info $(M) running tests) @ ## Run all tests
	$Q $(GO) test $(PKGS) -timeout 600s -race -p 8

.PHONY: test-short
test-short: | $(EMBEDDED) ; $(info $(M) running short tests) @ ## Run all short tests
	$Q $(GO) test $(PKGS) -timeout 120s -race -count 1 -short

.PHONY: fmt
fmt: | $(EMBEDDED) ; $(info $(M) running go fmt…) @ ## Run gofmt on all source files
	$Q $(GO) fmt $(PKGS)

.PHONY: generate
generate: | $(EMBEDDED) $(TOOLCHAIN) ; $(info $(M) updating generated files…) @ ## Update all generated files
	$Q $(GO) generate $(PKGS)
	$Q cd console && $(PNPM) run generate
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
