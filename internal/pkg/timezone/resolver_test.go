package timezone

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestResolveIANAPassthrough(t *testing.T) {
	tests := []string{
		"Europe/Amsterdam",
		"America/New_York",
		"Asia/Tokyo",
		"Australia/Sydney",
		"UTC",
		"Europe/London",
		"Pacific/Auckland",
	}

	for _, tz := range tests {
		t.Run(tz, func(t *testing.T) {
			resolved, err := Resolve(tz)
			require.NoError(t, err)
			require.Equal(t, tz, resolved)
		})
	}
}

func TestResolveAliases(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"EST", "America/New_York"},
		{"EDT", "America/New_York"},
		{"CST", "America/Chicago"},
		{"CDT", "America/Chicago"},
		{"MST", "America/Denver"},
		{"MDT", "America/Denver"},
		{"PST", "America/Los_Angeles"},
		{"PDT", "America/Los_Angeles"},
		{"CET", "Europe/Paris"},
		{"CEST", "Europe/Paris"},
		{"BST", "Europe/London"},
		{"IST", "Asia/Kolkata"},
		{"JST", "Asia/Tokyo"},
		{"AEST", "Australia/Sydney"},
		{"HST", "Pacific/Honolulu"},
		{"AKST", "America/Anchorage"},
		{"WET", "Europe/Lisbon"},
		{"CAT", "Africa/Harare"},
		{"EAT", "Africa/Nairobi"},
		{"SAST", "Africa/Johannesburg"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			resolved, err := Resolve(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, resolved)
		})
	}
}

func TestResolveGMTOffsets(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"GMT+2", "Europe/Amsterdam"},
		{"GMT-5", "America/New_York"},
		{"GMT+8", "Asia/Shanghai"},
		{"GMT-8", "America/Los_Angeles"},
		{"GMT+0", "Europe/London"},
		{"GMT+1", "Europe/Paris"},
		{"GMT+3", "Europe/Moscow"},
		{"GMT+12", "Pacific/Auckland"},
		{"GMT-11", "Pacific/Pago_Pago"},
		{"GMT-10", "Pacific/Honolulu"},
		{"GMT-9", "America/Anchorage"},
		{"GMT-7", "America/Denver"},
		{"GMT-6", "America/Chicago"},
		{"GMT-4", "America/Halifax"},
		{"GMT-3", "America/Sao_Paulo"},
		{"GMT-2", "Etc/GMT+2"},
		{"GMT-1", "Atlantic/Azores"},
		{"GMT+4", "Asia/Dubai"},
		{"GMT+5", "Asia/Karachi"},
		{"GMT+6", "Asia/Dhaka"},
		{"GMT+7", "Asia/Bangkok"},
		{"GMT+9", "Asia/Tokyo"},
		{"GMT+10", "Australia/Sydney"},
		{"GMT+11", "Pacific/Noumea"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			resolved, err := Resolve(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, resolved)
		})
	}
}

func TestResolveUTCOffsets(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"UTC+2", "Europe/Amsterdam"},
		{"UTC-5", "America/New_York"},
		{"UTC+8", "Asia/Shanghai"},
		{"UTC+0", "Europe/London"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			resolved, err := Resolve(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, resolved)
		})
	}
}

func TestResolveFormattedOffsets(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"+2", "Europe/Amsterdam"},
		{"-5", "America/New_York"},
		{"+0200", "Europe/Amsterdam"},
		{"-0500", "America/New_York"},
		{"+02:00", "Europe/Amsterdam"},
		{"-05:00", "America/New_York"},
		{"GMT+02:00", "Europe/Amsterdam"},
		{"UTC+02:00", "Europe/Amsterdam"},
		{"GMT+0200", "Europe/Amsterdam"},
		{"UTC-0500", "America/New_York"},
		{"UT+2", "Europe/Amsterdam"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			resolved, err := Resolve(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, resolved)
		})
	}
}

func TestResolveHalfHourOffsets(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"GMT+5:30", "Asia/Kolkata"},
		{"GMT+3:30", "Asia/Tehran"},
		{"GMT+4:30", "Asia/Kabul"},
		{"GMT+6:30", "Asia/Yangon"},
		{"GMT+9:30", "Australia/Darwin"},
		{"GMT+5:45", "Asia/Kathmandu"},
		{"GMT-3:30", "America/St_Johns"},
		{"GMT-4:30", "America/Caracas"},
		{"GMT-9:30", "Pacific/Marquesas"},
		{"+5:30", "Asia/Kolkata"},
		{"+0530", "Asia/Kolkata"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			resolved, err := Resolve(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, resolved)
		})
	}
}

func TestResolveCaseInsensitive(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"gmt+2", "Europe/Amsterdam"},
		{"gmt-5", "America/New_York"},
		{"Gmt+2", "Europe/Amsterdam"},
		{"est", "America/New_York"},
		{"Est", "America/New_York"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			resolved, err := Resolve(tt.input)
			require.NoError(t, err)
			require.Equal(t, tt.expected, resolved)
		})
	}
}

func TestResolveInvalid(t *testing.T) {
	tests := []string{
		"",
		"NotATimezone",
		"Foobar/Unknown",
		"GMT+X",
		"++2",
	}

	for _, tt := range tests {
		t.Run(tt, func(t *testing.T) {
			_, err := Resolve(tt)
			require.Error(t, err)
		})
	}
}

func TestResolveWithWhitespace(t *testing.T) {
	resolved, err := Resolve("  GMT+2  ")
	require.NoError(t, err)
	require.Equal(t, "Europe/Amsterdam", resolved)

	resolved, err = Resolve("  Europe/Amsterdam  ")
	require.NoError(t, err)
	require.Equal(t, "Europe/Amsterdam", resolved)
}
