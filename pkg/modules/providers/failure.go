package providers

import (
	"encoding/json"
	"errors"
)

// FailureReason is a constrained type describing why a provider rejected a
// send. WASM modules MUST map provider-specific error codes onto one of these
// canonical reasons so host-side policy (opt-out suppression, bounce
// handling, retry accounting) never has to match on message text.
type FailureReason string

const (
	// ReasonOptedOut indicates the recipient has opted out of receiving
	// messages from this sender, as enforced by the provider itself.
	ReasonOptedOut FailureReason = "recipient_opted_out"

	// ReasonInvalidNumber indicates the recipient address is malformed or
	// cannot receive messages on this channel.
	ReasonInvalidNumber FailureReason = "invalid_recipient"

	// ReasonUnregistered indicates the sender identity is not registered or
	// verified with the provider (e.g. unregistered 10DLC campaign).
	ReasonUnregistered FailureReason = "sender_unregistered"

	// ReasonRateLimited indicates the provider throttled the request.
	ReasonRateLimited FailureReason = "rate_limited"

	// ReasonUnknown indicates the failure could not be classified. It is also
	// what the host assumes for modules that report errors as plain strings.
	ReasonUnknown FailureReason = "unknown"
)

// Valid reports whether r is one of the canonical failure reasons.
func (r FailureReason) Valid() bool {
	switch r {
	case ReasonOptedOut, ReasonInvalidNumber, ReasonUnregistered, ReasonRateLimited, ReasonUnknown:
		return true
	default:
		return false
	}
}

// ModuleError is the JSON body a module may emit on the error path so the
// host can classify a failure without matching on message text.
type ModuleError struct {
	Reason  FailureReason `json:"reason"`
	Message string        `json:"message,omitempty"`
}

// moduleErrorJSON keeps the error interface off ModuleError itself: TinyGo's
// reflection cannot do AssignableTo, which panics encoding/json when it
// marshals a type that implements error.
type moduleErrorJSON string

func (e moduleErrorJSON) Error() string { return string(e) }

// Fail wraps err as a ModuleError carrying a canonical reason. The returned
// error's message is the ModuleError JSON document, so a module reports it
// with pdk.SetError(providers.Fail(reason, err)) on its usual error path and
// the host recovers the reason from the error body.
//
// Emitting a ModuleError is optional: a module that cannot classify a failure
// keeps calling pdk.SetError(err) with a plain error and the host reads that
// as ReasonUnknown.
func Fail(reason FailureReason, err error) error {
	var message string
	if err != nil {
		message = err.Error()
	}

	body, jsonErr := json.Marshal(ModuleError{Reason: reason, Message: message})
	if jsonErr != nil {
		return errors.New(message)
	}

	return moduleErrorJSON(body)
}
