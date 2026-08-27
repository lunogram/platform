// Package compliance holds the SMS compliance decisions that must be made
// independently of any store or transport: which timezone a recipient is in,
// and (in later phases) how an inbound keyword and a quiet-hours window are
// interpreted. Everything here takes plain values and returns plain values so
// the rules can be exercised without a database.
package compliance

import (
	"strings"

	"github.com/nyaruka/phonenumbers"
)

// TimezoneSource names the input that produced a resolved timezone. It is
// recorded alongside the zone so a later quiet-hours rollout can be judged
// against how the zone was obtained: a zone inferred from a dialling prefix is
// a weaker fact than one the customer told us.
type TimezoneSource string

const (
	TimezoneSourceUser    TimezoneSource = "user"
	TimezoneSourcePhone   TimezoneSource = "phone"
	TimezoneSourceProject TimezoneSource = "project"

	// TimezoneSourceUnresolved is returned with an empty zone when no input
	// yielded one.
	TimezoneSourceUnresolved TimezoneSource = ""
)

// RecipientTimezoneInput carries every fact the resolver may consider. Absent
// facts are empty strings; the caller is responsible for the lookups.
type RecipientTimezoneInput struct {
	UserTimezone    string
	Phone           string
	ProjectTimezone string
}

// ResolveRecipientTimezone returns the most accurate IANA zone available for a
// recipient, together with the input it came from.
//
// The chain runs from the most specific fact to the least: the recipient's own
// stored timezone, then the dialling prefix of their number (which resolves to
// area-code granularity, not merely country), then the project default. An
// empty zone with TimezoneSourceUnresolved means nothing resolved; that is a
// normal outcome and never an error.
func ResolveRecipientTimezone(in RecipientTimezoneInput) (string, TimezoneSource) {
	if zone := strings.TrimSpace(in.UserTimezone); zone != "" {
		return zone, TimezoneSourceUser
	}

	if zone := timezoneForPhone(in.Phone); zone != "" {
		return zone, TimezoneSourcePhone
	}

	if zone := strings.TrimSpace(in.ProjectTimezone); zone != "" {
		return zone, TimezoneSourceProject
	}

	return "", TimezoneSourceUnresolved
}

// timezoneForPhone returns the first zone the number's prefix maps to, or an
// empty string when the number is unparseable or maps to no known zone.
//
// A prefix spanning several zones (a country-wide mobile range, say) offers no
// basis for choosing between them, so the first is taken rather than guessing;
// callers that need certainty must fall back to a stated timezone.
func timezoneForPhone(phone string) string {
	phone = strings.TrimSpace(phone)
	if phone == "" {
		return ""
	}

	number, err := phonenumbers.Parse(phone, "")
	if err != nil {
		return ""
	}

	zones, err := phonenumbers.GetTimezonesForNumber(number)
	if err != nil {
		return ""
	}

	for _, zone := range zones {
		if zone != "" && zone != phonenumbers.UNKNOWN_TIMEZONE {
			return zone
		}
	}

	return ""
}
