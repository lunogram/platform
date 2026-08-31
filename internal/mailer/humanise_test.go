package mailer

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestHumaniseDuration(t *testing.T) {
	t.Parallel()

	tests := map[time.Duration]string{
		// The TTLs the platform actually issues.
		time.Hour:        "1 hour",
		24 * time.Hour:   "1 day",
		48 * time.Hour:   "2 days",
		720 * time.Hour:  "30 days",
		45 * time.Minute: "45 minutes",

		// expires_in is caller-supplied, so the awkward shapes arrive too. The
		// old formatter called each of these an hour, a flat day count, or zero.
		90 * time.Minute:              "1 hour and 30 minutes",
		60 * time.Hour:                "2 days and 12 hours",
		36 * time.Hour:                "1 day and 12 hours",
		2*time.Hour + 1*time.Minute:   "2 hours and 1 minute",
		25 * time.Hour:                "1 day and 1 hour",
		30 * time.Second:              "1 minute",
		time.Minute:                   "1 minute",
		24*time.Hour + 30*time.Minute: "1 day",
		time.Hour + 59*time.Minute + 59*time.Second: "1 hour and 59 minutes",
	}

	for ttl, expected := range tests {
		assert.Equal(t, expected, humaniseDuration(ttl), "%s", ttl)
	}
}

// Truncating is deliberate: a link may be described as lasting less than it
// does, never more, or the message promises access that is already gone.
func TestHumaniseDurationNeverOverstates(t *testing.T) {
	t.Parallel()

	assert.Equal(t, "1 day", humaniseDuration(24*time.Hour+59*time.Minute))
	assert.Equal(t, "2 days", humaniseDuration(48*time.Hour+59*time.Second))
	assert.Equal(t, "1 hour", humaniseDuration(time.Hour+59*time.Second))
}
