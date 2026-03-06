package test

import (
	_ "embed"
)

//go:generate sh -c "cd ./action/ && tinygo build -target=wasi -buildmode c-shared -opt=2 -no-debug -o ../action.wasm ./main.go"

//go:embed action.wasm
var ActionWASM []byte
