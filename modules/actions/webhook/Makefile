MODULE  := webhook
OUT     := ../../../internal/integrations/modules/$(MODULE).wasm
TINYGO  ?= $(shell which tinygo)
NODE    ?= $(shell which node)
NPM     := $(dir $(NODE))npm
NPX     := $(dir $(NODE))npx

.PHONY: all preview wasm clean

all: preview wasm

preview:
	@cd preview && $(NPM) ci --silent && $(NPX) vite build

wasm:
	@$(TINYGO) build -target=wasi -buildmode c-shared -opt=2 -no-debug -o $(OUT) ./main.go

clean:
	@rm -rf preview/dist preview/node_modules $(OUT)
