// Package webhook implements the outbound hook engine: named events, hooks
// bound to them in configuration, bodies rendered from JSONNet templates, and
// delivery through the guarded transport in internal/outbound.
//
// Adding an event is a single [MustRegister] call. Nothing else in the engine
// knows the event names — hooks bind to them by string in configuration, so a
// new event needs no new field, client or method.
package webhook

import (
	"sort"
	"sync"
	"time"
)

// Definition is a named, versioned event that hooks can subscribe to. The name
// is the stable identifier operators write in configuration and receivers
// switch on; Version travels in the envelope so a receiver can tell payload
// shapes apart without the name having to change.
type Definition struct {
	Name    string
	Version string
}

var (
	registryMu sync.RWMutex
	registry   = map[string]*Definition{}
)

// MustRegister adds def to the event registry and returns it. It panics on a
// duplicate or incomplete registration, both of which are programming errors
// visible at init time.
func MustRegister(def Definition) *Definition {
	if def.Name == "" {
		panic("webhook: event name is required")
	}
	if def.Version == "" {
		panic("webhook: event version is required for " + def.Name)
	}

	registryMu.Lock()
	defer registryMu.Unlock()
	if _, exists := registry[def.Name]; exists {
		panic("webhook: event already registered: " + def.Name)
	}
	stored := &def
	registry[def.Name] = stored
	return stored
}

// Registered returns every registered event name in sorted order. Config
// validation uses it to reject hooks bound to an event that does not exist,
// and to tell the operator which names it could have meant.
func Registered() []string {
	registryMu.RLock()
	defer registryMu.RUnlock()
	names := make([]string, 0, len(registry))
	for name := range registry {
		names = append(names, name)
	}
	sort.Strings(names)
	return names
}

// IsRegistered reports whether an event with the given name exists.
func IsRegistered(name string) bool {
	registryMu.RLock()
	defer registryMu.RUnlock()
	_, ok := registry[name]
	return ok
}

// Event is one occurrence of a registered event, ready to dispatch.
type Event struct {
	definition *Definition
	payload    any
	occurredAt time.Time
}

// Occurred builds an event carrying payload. The payload is whatever marshals
// to the event's documented JSON shape; the engine marshals it once and hands
// it to every hook's template.
func (d *Definition) Occurred(payload any) Event {
	return Event{definition: d, payload: payload, occurredAt: time.Now().UTC()}
}

// Name returns the event's registered name.
func (e Event) Name() string {
	if e.definition == nil {
		return ""
	}
	return e.definition.Name
}
