package integrations

import (
	"encoding/json"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	providertypes "github.com/lunogram/platform/pkg/modules/providers"
)

// guestError mirrors how a module failure reaches the host: pdk.SetError(err)
// transmits err.Error() verbatim and the Extism SDK hands it back as a plain
// error alongside the module's non-zero exit code.
func guestError(err error) error {
	return errors.New(err.Error())
}

func TestNewProviderErrorParsesModuleError(t *testing.T) {
	t.Parallel()

	failure := providertypes.Fail(providertypes.ReasonOptedOut, errors.New("failed to send SMS (to=+15551234567)"))

	provErr := newProviderError(exitCodePermanent, nil, guestError(failure))

	assert.Equal(t, providertypes.ReasonOptedOut, provErr.Reason)
	assert.Equal(t, "failed to send SMS (to=+15551234567)", provErr.Message)
	assert.True(t, provErr.IsPermanent())
}

func TestNewProviderErrorLegacyPlainStringFallback(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		message string
	}{
		{name: "twilio style", message: "failed to send SMS (to=+1555, from=+1666): twilio API error (status=400, code=21610): message blocked"},
		{name: "sendgrid style", message: "sendgrid API error (status=413): payload too large"},
		{name: "mailgun style", message: `mailgun API error (status=400): {"message":"to parameter is not a valid address"}`},
		{name: "resend style", message: "failed to send email (from=a@b.c, to=[d@e.f], subject=\"hi\"): unauthorized"},
		{name: "amazon ses style", message: "failed to send email via SES: InvalidParameterValue"},
		{name: "apns style", message: "APNs config incomplete: teamId, keyId, privateKey, and bundleId are required"},
		{name: "fcm style", message: "failed to get FCM access token: invalid_grant"},
		{name: "webpush style", message: "no WebPushTargets in payload"},
		{name: "webhook style", message: "missing required field: endpoint"},
		{name: "bare json object without a reason", message: `{"errors":[{"message":"nope"}]}`},
		{name: "json array", message: `[{"message":"nope"}]`},
		{name: "empty", message: ""},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			provErr := newProviderError(exitCodePermanent, nil, errors.New(tt.message))

			assert.Equal(t, providertypes.ReasonUnknown, provErr.Reason)
			assert.Equal(t, tt.message, provErr.Message)
		})
	}
}

func TestNewProviderErrorUnrecognisedReasonDowngradesToUnknown(t *testing.T) {
	t.Parallel()

	body, err := json.Marshal(map[string]string{"reason": "carrier_hates_us", "message": "boom"})
	require.NoError(t, err)

	provErr := newProviderError(exitCodePermanent, nil, errors.New(string(body)))

	assert.Equal(t, providertypes.ReasonUnknown, provErr.Reason)
	assert.Equal(t, "boom", provErr.Message)
}

func TestNewProviderErrorReasonRoundTripsEveryCanonicalValue(t *testing.T) {
	t.Parallel()

	reasons := []providertypes.FailureReason{
		providertypes.ReasonOptedOut,
		providertypes.ReasonInvalidNumber,
		providertypes.ReasonUnregistered,
		providertypes.ReasonRateLimited,
		providertypes.ReasonUnknown,
	}

	for _, reason := range reasons {
		t.Run(string(reason), func(t *testing.T) {
			failure := providertypes.Fail(reason, errors.New("nope"))

			provErr := newProviderError(exitCodePermanent, nil, guestError(failure))

			assert.Equal(t, reason, provErr.Reason)
			assert.Equal(t, "nope", provErr.Message)
		})
	}
}
