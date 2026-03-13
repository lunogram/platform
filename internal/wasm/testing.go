package wasm

import (
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/pkg/modules"
)

// NewTestModule creates a Module with the given manifest for testing purposes.
// The resulting module has no underlying WASM plugin, so Call() will panic.
// Use this only when you need a module in a registry for manifest inspection.
func NewTestModule[T modules.Manifest](manifest T, cfg config.WASM) *Module[T] {
	return &Module[T]{
		manifest: manifest,
		config:   cfg,
	}
}
