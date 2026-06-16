// Package ptr provides generic pointer utility functions for converting
// between pointer and value types.
package ptr

// To returns a pointer to the given value.
func To[T any](v T) *T {
	return &v
}

// From dereferences a pointer, returning the zero value if nil.
func From[T any](p *T) T {
	if p == nil {
		var zero T
		return zero
	}
	return *p
}

// FromOr dereferences a pointer, returning the fallback if nil.
func FromOr[T any](p *T, fallback T) T {
	if p == nil {
		return fallback
	}
	return *p
}
