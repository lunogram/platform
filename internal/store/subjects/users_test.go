package subjects

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func ptr[T any](v T) *T {
	return &v
}

// NewContainerStore creates a test database and returns a users State
func NewContainerStore(t *testing.T) *State {
	t.Helper()

	uri := container.RunPostgreSQL(t)
	usersURI := container.CreateSchema(t, uri, "users")

	require.NoError(t, Migrate(usersURI))

	ctx := graceful.NewContext(t.Context())
	logger := zaptest.NewLogger(t)

	usersDB, err := store.Connect(ctx, logger, usersURI)
	require.NoError(t, err)

	return NewState(usersDB, logger)
}

func TestCreateUser(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	type test struct {
		user User
	}

	tests := map[string]test{
		"create user with all fields": {
			user: User{
				ProjectID:   projectID,
				AnonymousID: ptr("anon_123"),
				ExternalID:  ptr("user_123"),
				Email:       ptr("test@example.com"),
				Phone:       ptr("+1234567890"),
				Data:        json.RawMessage(`{"first_name":"John","last_name":"Doe"}`),
				Timezone:    ptr("America/New_York"),
				Locale:      ptr("en"),
			},
		},
		"create user with minimal fields": {
			user: User{
				ProjectID:   projectID,
				Data:        json.RawMessage(`{}`),
				AnonymousID: ptr("anon_456"),
			},
		},
		"create user with JSONB data": {
			user: User{
				ProjectID:   projectID,
				AnonymousID: ptr("anon_789"),
				Data:        json.RawMessage(`{"custom_field":"value","nested":{"key":"value"}}`),
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			userID, err := db.CreateUser(ctx, tt.user)
			require.NoError(t, err)
			require.NotEqual(t, uuid.Nil, userID)

			user, err := db.GetUser(ctx, projectID, userID)
			require.NoError(t, err)
			require.Equal(t, tt.user.AnonymousID, user.AnonymousID)
			require.Equal(t, tt.user.ExternalID, user.ExternalID)
			require.Equal(t, tt.user.Email, user.Email)
			require.Equal(t, tt.user.Phone, user.Phone)
			require.Equal(t, tt.user.Timezone, user.Timezone)
			require.Equal(t, tt.user.Locale, user.Locale)
		})
	}
}

func TestGetUserByExternalID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	externalID := "user_external_123"
	userID, err := db.CreateUser(ctx, User{
		ProjectID:   projectID,
		Data:        json.RawMessage(`{}`),
		AnonymousID: ptr("anon_123"),
		ExternalID:  &externalID,
		Email:       ptr("external@example.com"),
	})
	require.NoError(t, err)

	user, err := db.GetUserByExternalID(ctx, projectID, externalID)
	require.NoError(t, err)
	require.Equal(t, userID, user.ID)
	require.Equal(t, externalID, *user.ExternalID)
}

func TestGetUserByAnonymousID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	anonymousID := "anon_unique_123"
	userID, err := db.CreateUser(ctx, User{
		ProjectID:   projectID,
		AnonymousID: &anonymousID,
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	user, err := db.GetUserByAnonymousID(ctx, projectID, anonymousID)
	require.NoError(t, err)
	require.Equal(t, userID, user.ID)
	require.Equal(t, &anonymousID, user.AnonymousID)
}

func TestListUsersWithSearch(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	_, err := db.CreateUser(ctx, User{
		ProjectID:   projectID,
		Data:        json.RawMessage(`{}`),
		AnonymousID: ptr("anon_1"),
		ExternalID:  ptr("user_john"),
		Email:       ptr("john@example.com"),
	})
	require.NoError(t, err)

	_, err = db.CreateUser(ctx, User{
		ProjectID:   projectID,
		Data:        json.RawMessage(`{}`),
		AnonymousID: ptr("anon_2"),
		ExternalID:  ptr("user_jane"),
		Email:       ptr("jane@example.com"),
	})
	require.NoError(t, err)

	_, err = db.CreateUser(ctx, User{
		ProjectID:   projectID,
		Data:        json.RawMessage(`{}`),
		AnonymousID: ptr("anon_3"),
		Phone:       ptr("+1234567890"),
	})
	require.NoError(t, err)

	type test struct {
		search        string
		expectedCount int
		description   string
	}

	tests := map[string]test{
		"search by email": {
			search:        "john",
			expectedCount: 1,
			description:   "should find user by email substring",
		},
		"search by external_id": {
			search:        "user_jane",
			expectedCount: 1,
			description:   "should find user by external_id",
		},
		"search by phone": {
			search:        "123456",
			expectedCount: 1,
			description:   "should find user by phone substring",
		},
		"no search returns all": {
			search:        "",
			expectedCount: 3,
			description:   "empty search should return all users",
		},
		"no matches": {
			search:        "nonexistent",
			expectedCount: 0,
			description:   "should return empty list when no matches",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			users, total, err := db.ListUsers(ctx, projectID, store.Pagination{Limit: 20, Offset: 0}, tt.search)
			require.NoError(t, err)
			require.Equal(t, tt.expectedCount, len(users), tt.description)
			require.Equal(t, tt.expectedCount, total, "total count should match result count")
		})
	}
}

func TestListUsersWithPagination(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	for i := 0; i < 5; i++ {
		anonID := uuid.New().String()
		_, err := db.CreateUser(ctx, User{
			ProjectID:   projectID,
			AnonymousID: &anonID,
			Data:        json.RawMessage(`{}`),
		})
		require.NoError(t, err)
	}

	type test struct {
		pagination    store.Pagination
		expectedCount int
		expectedTotal int
	}

	tests := map[string]test{
		"first page": {
			pagination:    store.Pagination{Limit: 2, Offset: 0},
			expectedCount: 2,
			expectedTotal: 5,
		},
		"second page": {
			pagination:    store.Pagination{Limit: 2, Offset: 2},
			expectedCount: 2,
			expectedTotal: 5,
		},
		"last page partial": {
			pagination:    store.Pagination{Limit: 2, Offset: 4},
			expectedCount: 1,
			expectedTotal: 5,
		},
		"beyond last page": {
			pagination:    store.Pagination{Limit: 2, Offset: 10},
			expectedCount: 0,
			expectedTotal: 0,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			users, total, err := db.ListUsers(ctx, projectID, tt.pagination, "")
			require.NoError(t, err)
			require.Equal(t, tt.expectedCount, len(users))
			require.Equal(t, tt.expectedTotal, total)
		})
	}
}

func TestUpsertUser(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	type test struct {
		setupUser     *User
		upsertData    UpsertUserParams
		expectedEmail *string
		description   string
	}

	tests := map[string]test{
		"insert new user with external_id": {
			upsertData: UpsertUserParams{
				AnonymousID: ptr("anon_new"),
				ExternalID:  ptr("user_new"),
				Email:       ptr("new@example.com"),
				Data:        map[string]any{},
			},
			expectedEmail: ptr("new@example.com"),
			description:   "should create new user",
		},
		"update existing user by external_id": {
			setupUser: &User{
				ProjectID:   projectID,
				AnonymousID: ptr("anon_existing"),
				ExternalID:  ptr("user_existing"),
				Email:       ptr("old@example.com"),
				Data:        json.RawMessage(`{}`),
			},
			upsertData: UpsertUserParams{
				AnonymousID: ptr("anon_different"),
				ExternalID:  ptr("user_existing"),
				Email:       ptr("updated@example.com"),
				Data:        map[string]any{},
			},
			expectedEmail: ptr("updated@example.com"),
			description:   "should update email on conflict",
		},
		"upsert with JSONB data": {
			setupUser: &User{
				ProjectID:   projectID,
				AnonymousID: ptr("anon_json"),
				ExternalID:  ptr("user_json"),
				Data:        json.RawMessage(`{"old":"value"}`),
			},
			upsertData: UpsertUserParams{
				AnonymousID: ptr("anon_json"),
				ExternalID:  ptr("user_json"),
				Data:        map[string]any{"new": "data"},
			},
			expectedEmail: nil,
			description:   "should update JSONB data",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var existingUserID uuid.UUID
			if tt.setupUser != nil {
				var err error
				existingUserID, err = db.CreateUser(ctx, *tt.setupUser)
				require.NoError(t, err)
			}

			userID, err := db.UpsertUser(ctx, projectID, tt.upsertData)
			require.NoError(t, err)
			require.NotEqual(t, uuid.Nil, userID)

			if tt.setupUser != nil {
				require.Equal(t, existingUserID, userID, "should return existing user ID on conflict")
			}

			user, err := db.GetUser(ctx, projectID, userID)
			require.NoError(t, err)
			require.Equal(t, tt.expectedEmail, user.Email, tt.description)
		})
	}
}

func TestUpdateUserWithDataMerge(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	initialData := json.RawMessage(`{"first_name":"John","age":30,"nested":{"key":"value"}}`)
	userID, err := db.CreateUser(ctx, User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_merge"),
		Data:        initialData,
	})
	require.NoError(t, err)

	type test struct {
		updateData   *json.RawMessage
		expectedKeys []string
		description  string
	}

	tests := map[string]test{
		"merge adds new keys": {
			updateData:   ptr(json.RawMessage(`{"last_name":"Doe"}`)),
			expectedKeys: []string{"first_name", "last_name", "age", "nested"},
			description:  "should preserve existing keys and add new ones",
		},
		"merge overwrites existing keys": {
			updateData:   ptr(json.RawMessage(`{"age":31}`)),
			expectedKeys: []string{"first_name", "age", "nested"},
			description:  "should update existing key value",
		},
		"nil data preserves existing": {
			updateData:   nil,
			expectedKeys: []string{"first_name", "age", "nested"},
			description:  "should not modify data when nil",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			err := db.UpdateUser(ctx, userID, UserUpdate{
				Data: tt.updateData,
			})
			require.NoError(t, err)

			user, err := db.GetUser(ctx, projectID, userID)
			require.NoError(t, err)

			var userData map[string]any
			err = json.Unmarshal(user.Data, &userData)
			require.NoError(t, err)

			for _, key := range tt.expectedKeys {
				_, exists := userData[key]
				require.True(t, exists, "expected key %s to exist: %s", key, tt.description)
			}
		})
	}
}

func TestDeleteUser(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID, err := db.CreateUser(ctx, User{
		ProjectID:   projectID,
		Data:        json.RawMessage(`{}`),
		AnonymousID: ptr("anon_delete"),
	})
	require.NoError(t, err)

	err = db.DeleteUser(ctx, projectID, userID)
	require.NoError(t, err)

	_, err = db.GetUser(ctx, projectID, userID)
	require.Error(t, err, "should return error when user is deleted")
}

func TestVersionAutoIncrement(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	userID, err := db.CreateUser(ctx, User{
		ProjectID:   projectID,
		Data:        json.RawMessage(`{}`),
		AnonymousID: ptr("anon_version"),
	})
	require.NoError(t, err)

	user, err := db.GetUser(ctx, projectID, userID)
	require.NoError(t, err)
	initialVersion := user.Version

	err = db.UpdateUser(ctx, userID, UserUpdate{
		Email: ptr("test@example.com"),
	})
	require.NoError(t, err)

	user, err = db.GetUser(ctx, projectID, userID)
	require.NoError(t, err)
	require.Equal(t, initialVersion+1, user.Version, "version should auto-increment on update")
}

func TestUserOAPIConversion(t *testing.T) {
	t.Parallel()

	user := User{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		AnonymousID:   ptr("anon_123"),
		ExternalID:    ptr("user_123"),
		Email:         ptr("test@example.com"),
		Phone:         ptr("+1234567890"),
		Data:          json.RawMessage(`{"key":"value"}`),
		HasPushDevice: false,
		Timezone:      ptr("UTC"),
		Locale:        ptr("en"),
		Version:       1,
	}

	oapiUser := user.OAPI()

	require.Equal(t, user.ID, oapiUser.Id)
	require.Equal(t, *user.AnonymousID, oapiUser.AnonymousId)
	require.Equal(t, user.ExternalID, oapiUser.ExternalId)
	require.Equal(t, user.Version, oapiUser.Version)
	require.False(t, oapiUser.HasPushDevice, "should be false when has_push_device is false")
}

func TestUserOAPIConversionWithDevices(t *testing.T) {
	t.Parallel()

	user := User{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		Data:          json.RawMessage(`{}`),
		AnonymousID:   ptr("anon_123"),
		HasPushDevice: true,
		Version:       1,
	}

	oapiUser := user.OAPI()
	require.True(t, oapiUser.HasPushDevice, "should be true when has_push_device is true")

	userWithoutDevice := User{
		ID:            uuid.New(),
		ProjectID:     uuid.New(),
		Data:          json.RawMessage(`{}`),
		AnonymousID:   ptr("anon_456"),
		HasPushDevice: false,
		Version:       1,
	}

	oapiUserWithout := userWithoutDevice.OAPI()
	require.False(t, oapiUserWithout.HasPushDevice, "should be false when has_push_device is false")
}

func TestGetUserWithDevices(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create a user
	userID, err := db.CreateUser(ctx, User{
		ProjectID:   projectID,
		AnonymousID: ptr("anon_with_devices"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	// Initially user should not have push device
	user, err := db.GetUser(ctx, projectID, userID)
	require.NoError(t, err)
	require.False(t, user.HasPushDevice, "should not have push device initially")

	// Insert a device with push token
	token := "push_token_ios_123"
	_, err = db.CreateDevice(ctx, Device{
		ProjectID: projectID,
		UserID:    userID,
		DeviceID:  "device_ios",
		Token:     &token,
		OS:        ptr("iOS"),
		Model:     ptr("iPhone 14"),
	})
	require.NoError(t, err)

	// Fetch the user again and verify has_push_device is true
	user, err = db.GetUser(ctx, projectID, userID)
	require.NoError(t, err)
	require.True(t, user.HasPushDevice, "should have push device after inserting device with token")

	// Verify OAPI conversion
	oapiUser := user.OAPI()
	require.True(t, oapiUser.HasPushDevice, "should detect push device from database")
}

func TestListUsersWithDevices(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create two users
	user1ID, err := db.CreateUser(ctx, User{
		ProjectID:   projectID,
		AnonymousID: ptr("user1_with_device"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	user2ID, err := db.CreateUser(ctx, User{
		ProjectID:   projectID,
		AnonymousID: ptr("user2_no_device"),
		Data:        json.RawMessage(`{}`),
	})
	require.NoError(t, err)

	// Add device with push token to user1 only
	token := "push_token_xyz"
	_, err = db.CreateDevice(ctx, Device{
		ProjectID: projectID,
		UserID:    user1ID,
		DeviceID:  "device1",
		Token:     &token,
	})
	require.NoError(t, err)

	// List users
	users, total, err := db.ListUsers(ctx, projectID, store.Pagination{Limit: 10, Offset: 0}, "")
	require.NoError(t, err)
	require.Equal(t, 2, total)

	// Find users by ID
	var user1, user2 *User
	for i := range users {
		if users[i].ID == user1ID {
			user1 = &users[i]
		}
		if users[i].ID == user2ID {
			user2 = &users[i]
		}
	}

	require.NotNil(t, user1, "user1 should be in results")
	require.NotNil(t, user2, "user2 should be in results")

	// Verify has_push_device is set correctly
	require.True(t, user1.HasPushDevice, "user1 should have push device")
	require.False(t, user2.HasPushDevice, "user2 should not have push device")

	// Verify OAPI conversion
	require.True(t, user1.OAPI().HasPushDevice, "user1 OAPI should have push device")
	require.False(t, user2.OAPI().HasPushDevice, "user2 OAPI should not have push device")
}
