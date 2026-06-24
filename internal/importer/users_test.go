package importer

import (
	"testing"

	"github.com/lunogram/platform/internal/store/subjects"
	"github.com/stretchr/testify/require"
)

func TestNewUsers(t *testing.T) {
	type test struct {
		headers []string
		err     error
	}

	tests := map[string]test{
		"valid headers with external_id": {
			headers: []string{"external_id", "email", "phone"},
			err:     nil,
		},
		"valid headers out of order": {
			headers: []string{"email", "phone", "external_id", "timezone"},
			err:     nil,
		},
		"valid headers with custom fields": {
			headers: []string{"external_id", "email", "custom_field_1", "custom_field_2"},
			err:     nil,
		},
		"missing external_id": {
			headers: []string{"email", "phone", "timezone"},
			err:     ErrMissingExternalID,
		},
		"only external_id": {
			headers: []string{"external_id"},
			err:     nil,
		},
		"case insensitive external_id": {
			headers: []string{"External_ID", "email"},
			err:     nil,
		},
		"external_id with whitespace": {
			headers: []string{" external_id ", "email"},
			err:     nil,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mapper, err := NewUsers(test.headers)

			if test.err != nil {
				require.ErrorIs(t, err, test.err)
				require.Nil(t, mapper)
			} else {
				require.NoError(t, err)
				require.NotNil(t, mapper)
				require.Equal(t, len(test.headers), len(mapper.Setters))
				require.Equal(t, test.headers, mapper.Headers)
			}
		})
	}
}

func TestUserMapperMapRecord(t *testing.T) {
	type test struct {
		headers  []string
		record   []string
		validate func(*testing.T, subjects.UpsertUserParams)
	}

	tests := map[string]test{
		"standard fields": {
			headers: []string{"external_id", "email", "phone", "timezone", "locale"},
			record:  []string{"user-123", "test@example.com", "+1234567890", "UTC", "en"},
			validate: func(t *testing.T, user subjects.UpsertUserParams) {
				require.Len(t, user.Identifiers, 1)
				require.Equal(t, "default", user.Identifiers[0].Source)
				require.Equal(t, "user-123", user.Identifiers[0].ExternalID)
				require.NotNil(t, user.Email)
				require.Equal(t, "test@example.com", *user.Email)
				require.NotNil(t, user.Phone)
				require.Equal(t, "+1234567890", *user.Phone)
				require.NotNil(t, user.Timezone)
				require.Equal(t, "UTC", *user.Timezone)
				require.NotNil(t, user.Locale)
				require.Equal(t, "en", *user.Locale)
				require.Empty(t, user.Data)
			},
		},
		"out of order fields": {
			headers: []string{"email", "external_id", "timezone"},
			record:  []string{"test@example.com", "user-456", "America/New_York"},
			validate: func(t *testing.T, user subjects.UpsertUserParams) {
				require.Len(t, user.Identifiers, 1)
				require.Equal(t, "default", user.Identifiers[0].Source)
				require.Equal(t, "user-456", user.Identifiers[0].ExternalID)
				require.NotNil(t, user.Email)
				require.Equal(t, "test@example.com", *user.Email)
				require.NotNil(t, user.Timezone)
				require.Equal(t, "America/New_York", *user.Timezone)
			},
		},
		"with custom fields": {
			headers: []string{"external_id", "email", "company", "role"},
			record:  []string{"user-789", "admin@example.com", "Acme Inc", "Admin"},
			validate: func(t *testing.T, user subjects.UpsertUserParams) {
				require.Len(t, user.Identifiers, 1)
				require.Equal(t, "default", user.Identifiers[0].Source)
				require.Equal(t, "user-789", user.Identifiers[0].ExternalID)
				require.NotNil(t, user.Email)
				require.Equal(t, "admin@example.com", *user.Email)
				require.NotNil(t, user.Data)
				require.Equal(t, "Acme Inc", user.Data["company"])
				require.Equal(t, "Admin", user.Data["role"])
			},
		},
		"only custom fields": {
			headers: []string{"external_id", "department", "employee_id"},
			record:  []string{"user-999", "Engineering", "EMP-001"},
			validate: func(t *testing.T, user subjects.UpsertUserParams) {
				require.Len(t, user.Identifiers, 1)
				require.Equal(t, "default", user.Identifiers[0].Source)
				require.Equal(t, "user-999", user.Identifiers[0].ExternalID)
				require.NotNil(t, user.Data)
				require.Equal(t, "Engineering", user.Data["department"])
				require.Equal(t, "EMP-001", user.Data["employee_id"])
			},
		},
		"with whitespace": {
			headers: []string{"external_id", "email", "phone"},
			record:  []string{"  user-111  ", "  test@example.com  ", "  +1234567890  "},
			validate: func(t *testing.T, user subjects.UpsertUserParams) {
				require.Len(t, user.Identifiers, 1)
				require.Equal(t, "default", user.Identifiers[0].Source)
				require.Equal(t, "user-111", user.Identifiers[0].ExternalID)
				require.NotNil(t, user.Email)
				require.Equal(t, "test@example.com", *user.Email)
				require.NotNil(t, user.Phone)
				require.Equal(t, "+1234567890", *user.Phone)
			},
		},
		"minimal record": {
			headers: []string{"external_id"},
			record:  []string{"user-minimal"},
			validate: func(t *testing.T, user subjects.UpsertUserParams) {
				require.Len(t, user.Identifiers, 1)
				require.Equal(t, "default", user.Identifiers[0].Source)
				require.Equal(t, "user-minimal", user.Identifiers[0].ExternalID)
				require.Nil(t, user.Email)
				require.Nil(t, user.Phone)
				require.Nil(t, user.Timezone)
				require.Nil(t, user.Locale)
				require.Empty(t, user.Data)
			},
		},
		"timezone GMT offset conversion": {
			headers: []string{"external_id", "timezone"},
			record:  []string{"user-gmt", "GMT+2"},
			validate: func(t *testing.T, user subjects.UpsertUserParams) {
				require.Len(t, user.Identifiers, 1)
				require.Equal(t, "user-gmt", user.Identifiers[0].ExternalID)
				require.NotNil(t, user.Timezone)
				require.Equal(t, "Europe/Amsterdam", *user.Timezone)
			},
		},
		"timezone IANA passthrough": {
			headers: []string{"external_id", "timezone"},
			record:  []string{"user-iana", "America/New_York"},
			validate: func(t *testing.T, user subjects.UpsertUserParams) {
				require.Len(t, user.Identifiers, 1)
				require.Equal(t, "user-iana", user.Identifiers[0].ExternalID)
				require.NotNil(t, user.Timezone)
				require.Equal(t, "America/New_York", *user.Timezone)
			},
		},
		"empty values": {
			headers: []string{"external_id", "email", "phone"},
			record:  []string{"user-empty", "", ""},
			validate: func(t *testing.T, user subjects.UpsertUserParams) {
				require.Len(t, user.Identifiers, 1)
				require.Equal(t, "default", user.Identifiers[0].Source)
				require.Equal(t, "user-empty", user.Identifiers[0].ExternalID)
				require.NotNil(t, user.Email)
				require.Equal(t, "", *user.Email)
				require.NotNil(t, user.Phone)
				require.Equal(t, "", *user.Phone)
			},
		},
		"mixed standard and custom fields": {
			headers: []string{"external_id", "email", "subscription_tier", "phone", "join_date"},
			record:  []string{"user-mixed", "user@example.com", "premium", "+1234567890", "2025-01-01"},
			validate: func(t *testing.T, user subjects.UpsertUserParams) {
				require.Len(t, user.Identifiers, 1)
				require.Equal(t, "default", user.Identifiers[0].Source)
				require.Equal(t, "user-mixed", user.Identifiers[0].ExternalID)
				require.NotNil(t, user.Email)
				require.Equal(t, "user@example.com", *user.Email)
				require.NotNil(t, user.Phone)
				require.Equal(t, "+1234567890", *user.Phone)
				require.NotNil(t, user.Data)
				require.Equal(t, "premium", user.Data["subscription_tier"])
				require.Equal(t, "2025-01-01", user.Data["join_date"])
			},
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			mapper, err := NewUsers(test.headers)
			require.NoError(t, err)

			user, err := mapper.MapRecord(test.record)
			require.NoError(t, err)

			test.validate(t, user)
		})
	}
}

func TestUserFieldMap(t *testing.T) {
	require.Contains(t, UserFieldMap, "external_id")
	require.Contains(t, UserFieldMap, "email")
	require.Contains(t, UserFieldMap, "phone")
	require.Contains(t, UserFieldMap, "timezone")
	require.Contains(t, UserFieldMap, "locale")
	require.Len(t, UserFieldMap, 5)
}
