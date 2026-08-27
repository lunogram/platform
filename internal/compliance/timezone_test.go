package compliance

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveRecipientTimezoneFallbackChain(t *testing.T) {
	t.Parallel()

	tests := map[string]struct {
		input  RecipientTimezoneInput
		zone   string
		source TimezoneSource
	}{
		"user timezone wins over every other input": {
			input: RecipientTimezoneInput{
				UserTimezone:    "Australia/Sydney",
				Phone:           "+14155552671",
				ProjectTimezone: "Europe/Amsterdam",
			},
			zone:   "Australia/Sydney",
			source: TimezoneSourceUser,
		},
		"phone prefix resolves on area code": {
			input: RecipientTimezoneInput{
				Phone:           "+14155552671",
				ProjectTimezone: "Europe/Amsterdam",
			},
			zone:   "America/Los_Angeles",
			source: TimezoneSourcePhone,
		},
		"a different area code in the same country resolves differently": {
			input:  RecipientTimezoneInput{Phone: "+12125551234"},
			zone:   "America/New_York",
			source: TimezoneSourcePhone,
		},
		"non-US numbers resolve too": {
			input:  RecipientTimezoneInput{Phone: "+31612345678"},
			zone:   "Europe/Amsterdam",
			source: TimezoneSourcePhone,
		},
		"project timezone is the last resort": {
			input:  RecipientTimezoneInput{ProjectTimezone: "Europe/Amsterdam"},
			zone:   "Europe/Amsterdam",
			source: TimezoneSourceProject,
		},
		"an unparseable number falls through to the project": {
			input: RecipientTimezoneInput{
				Phone:           "not-a-number",
				ProjectTimezone: "Europe/Amsterdam",
			},
			zone:   "Europe/Amsterdam",
			source: TimezoneSourceProject,
		},
		"blank inputs are not treated as answers": {
			input: RecipientTimezoneInput{
				UserTimezone:    "   ",
				Phone:           "  ",
				ProjectTimezone: "Europe/Amsterdam",
			},
			zone:   "Europe/Amsterdam",
			source: TimezoneSourceProject,
		},
		"nothing resolves": {
			input:  RecipientTimezoneInput{},
			zone:   "",
			source: TimezoneSourceUnresolved,
		},
		"an unparseable number with no project default resolves to nothing": {
			input:  RecipientTimezoneInput{Phone: "+999999999999999"},
			zone:   "",
			source: TimezoneSourceUnresolved,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()

			zone, source := ResolveRecipientTimezone(tc.input)
			require.Equal(t, tc.zone, zone)
			require.Equal(t, tc.source, source)
		})
	}
}

// TestTimezoneForPhoneRejectsUnknownSentinel guards the one behaviour the
// upstream library expresses as a value rather than an error: a prefix it
// cannot place yields "Etc/Unknown", which must never be stored as if it were
// a real zone.
func TestTimezoneForPhoneRejectsUnknownSentinel(t *testing.T) {
	t.Parallel()

	require.Equal(t, "", timezoneForPhone("+999999999999999"))
}
