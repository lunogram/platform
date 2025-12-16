package statemachine

import "errors"

var (
	// ErrStateNotFound is returned when a state is not registered
	ErrStateNotFound = errors.New("state not found")
)
