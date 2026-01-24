package test

import (
	_ "embed"
)

//go:generate sh -c "cd ./provider/ && tinygo build -target=wasi -buildmode c-shared -opt=2 -no-debug -o ../provider.wasm ./main.go"

//go:embed provider.wasm
var ProviderWASM []byte
