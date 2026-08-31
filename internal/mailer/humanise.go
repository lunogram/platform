package mailer

import (
	"fmt"
	"strings"
	"time"
)

// humaniseDuration writes a duration the way a sentence would say it: "2 days",
// "1 hour and 30 minutes", "45 minutes".
//
// It exists rather than a dependency because the only caller is a sentence in an
// email footer, and the general-purpose formatters all need configuring into
// this shape anyway -- they lead with a full breakdown ("2 days 0 hours 0
// minutes") and leave the joining, the pluralisation and the truncation to the
// caller regardless.
//
// At most two units are emitted, because the third never changes what the
// recipient does. The remainder is truncated rather than rounded: a link
// described as lasting slightly less than it does is harmless, one described as
// lasting longer is a broken promise. For the same reason anything under a
// minute reads as "1 minute" rather than "0 minutes" -- an expiry is quoted to
// somebody deciding whether to act now, and "0" tells them nothing they can use.
func humaniseDuration(d time.Duration) string {
	if d < time.Minute {
		return "1 minute"
	}

	days := int(d / (24 * time.Hour))
	hours := int(d % (24 * time.Hour) / time.Hour)
	minutes := int(d % time.Hour / time.Minute)

	var parts []string
	switch {
	case days > 0:
		parts = append(parts, plural(days, "day"))
		if hours > 0 {
			parts = append(parts, plural(hours, "hour"))
		}
	case hours > 0:
		parts = append(parts, plural(hours, "hour"))
		if minutes > 0 {
			parts = append(parts, plural(minutes, "minute"))
		}
	default:
		parts = append(parts, plural(minutes, "minute"))
	}

	return strings.Join(parts, " and ")
}

func plural(n int, unit string) string {
	if n == 1 {
		return fmt.Sprintf("1 %s", unit)
	}
	return fmt.Sprintf("%d %ss", n, unit)
}
