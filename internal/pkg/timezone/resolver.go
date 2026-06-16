package timezone

import (
	"fmt"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"

	gotz "github.com/tkuchiki/go-timezone"
)

var offsetOverride = map[int]string{
	0:      "Europe/London",
	3600:   "Europe/Paris",
	7200:   "Europe/Amsterdam",
	10800:  "Europe/Moscow",
	14400:  "Asia/Dubai",
	18000:  "Asia/Karachi",
	21600:  "Asia/Dhaka",
	25200:  "Asia/Bangkok",
	28800:  "Asia/Shanghai",
	32400:  "Asia/Tokyo",
	36000:  "Australia/Sydney",
	39600:  "Pacific/Noumea",
	43200:  "Pacific/Auckland",
	-3600:  "Atlantic/Azores",
	-7200:  "Etc/GMT+2",
	-10800: "America/Sao_Paulo",
	-14400: "America/Halifax",
	-18000: "America/New_York",
	-21600: "America/Chicago",
	-25200: "America/Denver",
	-28800: "America/Los_Angeles",
	-32400: "America/Anchorage",
	-36000: "Pacific/Honolulu",
	-39600: "Pacific/Pago_Pago",
	-43200: "Etc/GMT+12",
	-34200: "Pacific/Marquesas",
	-16200: "America/Caracas",
	-12600: "America/St_Johns",
	12600:  "Asia/Tehran",
	16200:  "Asia/Kabul",
	19800:  "Asia/Kolkata",
	20700:  "Asia/Kathmandu",
	23400:  "Asia/Yangon",
	31500:  "Australia/Eucla",
	34200:  "Australia/Darwin",
	37800:  "Australia/Lord_Howe",
	45900:  "Pacific/Chatham",
}

var abbrMap = map[string]string{
	"est":  "America/New_York",
	"edt":  "America/New_York",
	"cst":  "America/Chicago",
	"cdt":  "America/Chicago",
	"mst":  "America/Denver",
	"mdt":  "America/Denver",
	"pst":  "America/Los_Angeles",
	"pdt":  "America/Los_Angeles",
	"cet":  "Europe/Paris",
	"cest": "Europe/Paris",
	"eet":  "Europe/Helsinki",
	"eest": "Europe/Helsinki",
	"bst":  "Europe/London",
	"ist":  "Asia/Kolkata",
	"jst":  "Asia/Tokyo",
	"aest": "Australia/Sydney",
	"aedt": "Australia/Sydney",
	"awst": "Australia/Perth",
	"nzst": "Pacific/Auckland",
	"nzdt": "Pacific/Auckland",
	"hst":  "Pacific/Honolulu",
	"akst": "America/Anchorage",
	"akdt": "America/Anchorage",
	"wet":  "Europe/Lisbon",
	"west": "Europe/Lisbon",
	"cat":  "Africa/Harare",
	"eat":  "Africa/Nairobi",
	"wat":  "Africa/Lagos",
	"sast": "Africa/Johannesburg",
}

var offsetToIANA map[int]string

func init() {
	tz := gotz.New()
	offsetToIANA = make(map[int]string, len(tz.TzInfos())+len(offsetOverride))

	for k, v := range offsetOverride {
		offsetToIANA[k] = v
	}

	names := make([]string, 0, len(tz.TzInfos()))
	for name := range tz.TzInfos() {
		names = append(names, name)
	}
	sort.Strings(names)

	for _, name := range names {
		info := tz.TzInfos()[name]
		if info.IsDeprecated() {
			continue
		}
		for _, off := range []int{info.StandardOffset(), info.DaylightOffset()} {
			if _, exists := offsetToIANA[off]; !exists {
				offsetToIANA[off] = name
			}
		}
	}
}

var (
	gmtPrefix       = regexp.MustCompile(`^(?i)(?:GMT|UTC|UT)\s*`)
	offsetWithColon = regexp.MustCompile(`^([+-])(\d{1,2}):(\d{2})$`)
	offsetFlat      = regexp.MustCompile(`^([+-])(\d{2})(\d{2})$`)
	offsetHoursOnly = regexp.MustCompile(`^([+-])(\d{1,2})$`)
)

func Resolve(tz string) (string, error) {
	normalized := strings.TrimSpace(tz)
	if normalized == "" {
		return "", fmt.Errorf("timezone: empty string")
	}

	lower := strings.ToLower(normalized)

	if mapped, ok := abbrMap[lower]; ok {
		return mapped, nil
	}

	stripped := gmtPrefix.ReplaceAllString(normalized, "")
	if stripped != normalized {
		if resolved, ok := tryParseOffset(stripped); ok {
			return resolved, nil
		}
	}

	if resolved, ok := tryParseOffset(normalized); ok {
		return resolved, nil
	}

	_, err := time.LoadLocation(normalized)
	if err == nil {
		return normalized, nil
	}

	return "", fmt.Errorf("timezone: unable to resolve %q", tz)
}

func tryParseOffset(s string) (string, bool) {
	m := offsetWithColon.FindStringSubmatch(s)
	if m != nil {
		return lookupOffset(m[1], m[2], m[3])
	}

	m = offsetFlat.FindStringSubmatch(s)
	if m != nil {
		return lookupOffset(m[1], m[2], m[3])
	}

	m = offsetHoursOnly.FindStringSubmatch(s)
	if m != nil {
		return lookupOffset(m[1], m[2], "")
	}

	return "", false
}

func lookupOffset(sign, hoursStr, minutesStr string) (string, bool) {
	hours, err := strconv.Atoi(hoursStr)
	if err != nil {
		return "", false
	}

	minutes := 0
	if minutesStr != "" {
		minutes, err = strconv.Atoi(minutesStr)
		if err != nil {
			return "", false
		}
	}

	totalSeconds := (hours*60 + minutes) * 60
	if sign == "-" {
		totalSeconds = -totalSeconds
	}

	mapped, ok := offsetToIANA[totalSeconds]
	if !ok {
		return "", false
	}
	return mapped, true
}
