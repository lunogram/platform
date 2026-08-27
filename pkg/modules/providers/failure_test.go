package providers

import (
	"encoding/json"
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFailureReasonValid(t *testing.T) {
	t.Parallel()

	canonical := []FailureReason{
		ReasonOptedOut,
		ReasonInvalidNumber,
		ReasonUnregistered,
		ReasonRateLimited,
		ReasonUnknown,
	}

	for _, reason := range canonical {
		t.Run(string(reason), func(t *testing.T) {
			assert.True(t, reason.Valid())
		})
	}

	junk := []FailureReason{"", "carrier_hates_us", "Recipient_Opted_Out", "recipient_opted_out ", "optedOut"}

	for _, reason := range junk {
		t.Run("rejects "+string(reason), func(t *testing.T) {
			assert.False(t, reason.Valid())
		})
	}
}

func TestFailureReasonWireValues(t *testing.T) {
	t.Parallel()

	// These strings cross the sandbox boundary and are matched by the host.
	// Changing one silently breaks every module built against the old value.
	assert.Equal(t, FailureReason("recipient_opted_out"), ReasonOptedOut)
	assert.Equal(t, FailureReason("invalid_recipient"), ReasonInvalidNumber)
	assert.Equal(t, FailureReason("sender_unregistered"), ReasonUnregistered)
	assert.Equal(t, FailureReason("rate_limited"), ReasonRateLimited)
	assert.Equal(t, FailureReason("unknown"), ReasonUnknown)
}

func TestFailEmitsParseableModuleError(t *testing.T) {
	t.Parallel()

	for _, reason := range []FailureReason{ReasonOptedOut, ReasonInvalidNumber, ReasonUnregistered, ReasonRateLimited, ReasonUnknown} {
		t.Run(string(reason), func(t *testing.T) {
			cause := fmt.Errorf("failed to send SMS (to=%s): %w", "+15551234567", errors.New("blocked"))

			err := Fail(reason, cause)
			require.Error(t, err)

			var decoded ModuleError
			require.NoError(t, json.Unmarshal([]byte(err.Error()), &decoded))
			assert.Equal(t, reason, decoded.Reason)
			assert.Equal(t, cause.Error(), decoded.Message)
			assert.True(t, decoded.Reason.Valid())
		})
	}
}

func TestFailWithNilCause(t *testing.T) {
	t.Parallel()

	err := Fail(ReasonRateLimited, nil)
	require.Error(t, err)

	var decoded ModuleError
	require.NoError(t, json.Unmarshal([]byte(err.Error()), &decoded))
	assert.Equal(t, ReasonRateLimited, decoded.Reason)
	assert.Empty(t, decoded.Message)
	assert.NotContains(t, err.Error(), "null")
}

func TestFailEscapesMessagesThatBreakJSON(t *testing.T) {
	t.Parallel()

	cause := errors.New(`provider said {"code": 21610} and "quoted" plus a newline
and a backslash \`)

	err := Fail(ReasonOptedOut, cause)

	var decoded ModuleError
	require.NoError(t, json.Unmarshal([]byte(err.Error()), &decoded))
	assert.Equal(t, ReasonOptedOut, decoded.Reason)
	assert.Equal(t, cause.Error(), decoded.Message)
}

func TestModuleErrorRoundTrip(t *testing.T) {
	t.Parallel()

	original := ModuleError{Reason: ReasonUnregistered, Message: "10DLC campaign not registered"}

	body, err := json.Marshal(original)
	require.NoError(t, err)
	assert.JSONEq(t, `{"reason":"sender_unregistered","message":"10DLC campaign not registered"}`, string(body))

	var decoded ModuleError
	require.NoError(t, json.Unmarshal(body, &decoded))
	assert.Equal(t, original, decoded)
}

func TestModuleErrorOmitsEmptyMessage(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(ModuleError{Reason: ReasonOptedOut})
	require.NoError(t, err)

	assert.JSONEq(t, `{"reason":"recipient_opted_out"}`, string(body))
}
