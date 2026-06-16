MODULE  := fcm
OUT     := ../../internal/integrations/modules/$(MODULE).wasm
TINYGO  ?= $(shell which tinygo)

.PHONY: wasm clean

wasm:
	@$(TINYGO) build -target=wasi -buildmode c-shared -opt=2 -no-debug -o $(OUT) ./main.go

clean:
	@rm -f $(OUT)
