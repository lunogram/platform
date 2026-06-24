package subjects

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/require"
)

func TestGetOrganizationByExternalID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	externalID := "org_external_123"
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: externalID}},
		Name:        ptr.To("Test Organization"),
	})
	require.NoError(t, err)

	org, err := db.GetOrganizationByExternalID(ctx, projectID, "default", externalID)
	require.NoError(t, err)
	require.Equal(t, orgID, org.ID)
	require.Equal(t, externalID, org.ExternalIDBySource("default").ExternalID)
}

func TestListOrganizations(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create multiple orgs
	for i := 0; i < 5; i++ {
		_, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
			Identifiers: []ExternalIDParam{{Source: "default", ExternalID: uuid.New().String()}},
			Name:        ptr.To("Test Org"),
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
		"all orgs": {
			pagination:    store.Pagination{Limit: 10, Offset: 0},
			expectedCount: 5,
			expectedTotal: 5,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			orgs, total, err := db.ListOrganizations(ctx, projectID, tt.pagination, "")
			require.NoError(t, err)
			require.Equal(t, tt.expectedCount, len(orgs))
			require.Equal(t, tt.expectedTotal, total)
		})
	}
}

func TestUpsertOrganization(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	type test struct {
		setupIdentifiers []ExternalIDParam
		setupName        *string
		upsertData       UpsertOrganizationParams
		expectedName     *string
		description      string
	}

	tests := map[string]test{
		"insert new org": {
			upsertData: UpsertOrganizationParams{
				Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "new_org"}},
				Name:        ptr.To("New Organization"),
				Data:        map[string]any{"plan": "free"},
			},
			expectedName: ptr.To("New Organization"),
			description:  "should create new org",
		},
		"update existing org by external_id": {
			setupIdentifiers: []ExternalIDParam{{Source: "default", ExternalID: "existing_org"}},
			setupName:        ptr.To("Old Name"),
			upsertData: UpsertOrganizationParams{
				Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "existing_org"}},
				Name:        ptr.To("Updated Name"),
				Data:        map[string]any{},
			},
			expectedName: ptr.To("Updated Name"),
			description:  "should update name on conflict",
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			var existingOrgID uuid.UUID
			if tt.setupIdentifiers != nil {
				var err error
				existingOrgID, err = db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
					Identifiers: tt.setupIdentifiers,
					Name:        tt.setupName,
				})
				require.NoError(t, err)
			}

			orgID, err := db.UpsertOrganization(ctx, projectID, tt.upsertData)
			require.NoError(t, err)
			require.NotEqual(t, uuid.Nil, orgID)

			if tt.setupIdentifiers != nil {
				require.Equal(t, existingOrgID, orgID, "should return existing org ID on conflict")
			}

			org, err := db.GetOrganization(ctx, projectID, orgID)
			require.NoError(t, err)
			require.Equal(t, tt.expectedName, org.Name, tt.description)
		})
	}
}

func TestUpdateOrganizationWithDataMerge(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_merge"}},
		Data:        map[string]any{"plan": "free", "seats": 10},
	})
	require.NoError(t, err)

	// Update with merged data
	updateData := json.RawMessage(`{"seats":50,"feature_flags":["beta"]}`)
	err = db.UpdateOrganization(ctx, projectID, orgID, OrganizationUpdate{
		Data: &updateData,
	})
	require.NoError(t, err)

	org, err := db.GetOrganization(ctx, projectID, orgID)
	require.NoError(t, err)

	var orgData map[string]any
	err = json.Unmarshal(org.Data, &orgData)
	require.NoError(t, err)

	// Original key preserved
	require.Equal(t, "free", orgData["plan"])
	// Updated key changed
	require.Equal(t, float64(50), orgData["seats"])
	// New key added
	require.NotNil(t, orgData["feature_flags"])
}

func TestUpsertOrganizationDataMergeOnConflict(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_upsert_merge"}},
		Name:        ptr.To("Original Name"),
		Data:        map[string]any{"plan": "free", "seats": 10},
	})
	require.NoError(t, err)

	t.Run("preserves data when upserting with nil data", func(t *testing.T) {
		upsertedID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
			Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_upsert_merge"}},
			Name:        ptr.To("Updated Name"),
			Data:        nil,
		})
		require.NoError(t, err)
		require.Equal(t, orgID, upsertedID)

		org, err := db.GetOrganization(ctx, projectID, orgID)
		require.NoError(t, err)
		require.Equal(t, ptr.To("Updated Name"), org.Name)

		var orgData map[string]any
		err = json.Unmarshal(org.Data, &orgData)
		require.NoError(t, err)
		require.Equal(t, "free", orgData["plan"], "existing data should be preserved")
		require.Equal(t, float64(10), orgData["seats"], "existing data should be preserved")
	})

	t.Run("merges data when upserting with new data", func(t *testing.T) {
		upsertedID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
			Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_upsert_merge"}},
			Data:        map[string]any{"seats": 50, "feature": "beta"},
		})
		require.NoError(t, err)
		require.Equal(t, orgID, upsertedID)

		org, err := db.GetOrganization(ctx, projectID, orgID)
		require.NoError(t, err)

		var orgData map[string]any
		err = json.Unmarshal(org.Data, &orgData)
		require.NoError(t, err)
		require.Equal(t, "free", orgData["plan"], "original key should be preserved")
		require.Equal(t, float64(50), orgData["seats"], "updated key should be changed")
		require.Equal(t, "beta", orgData["feature"], "new key should be added")
	})
}

func TestDeleteOrganization(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_delete"}},
	})
	require.NoError(t, err)

	err = db.DeleteOrganization(ctx, projectID, orgID)
	require.NoError(t, err)

	_, err = db.GetOrganization(ctx, projectID, orgID)
	require.Error(t, err, "should return error when org is deleted")
}

func TestOrganizationVersionAutoIncrement(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_version"}},
	})
	require.NoError(t, err)

	org, err := db.GetOrganization(ctx, projectID, orgID)
	require.NoError(t, err)
	initialVersion := org.Version

	err = db.UpdateOrganization(ctx, projectID, orgID, OrganizationUpdate{
		Name: ptr.To("Updated Name"),
	})
	require.NoError(t, err)

	org, err = db.GetOrganization(ctx, projectID, orgID)
	require.NoError(t, err)
	require.Equal(t, initialVersion+1, org.Version, "version should auto-increment on update")
}

func TestUpsertAndGetOrganizationMember(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create org
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_membership"}},
	})
	require.NoError(t, err)

	// Create user
	userID, err := db.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []ExternalIDParam{
		{Source: "anonymous", ExternalID: "anon_org_member"},
	})
	require.NoError(t, err)

	// Add user to org with org-specific data
	orgUser, err := db.UpsertAndGetOrganizationMember(ctx, orgID, userID, map[string]any{
		"role":       "admin",
		"department": "engineering",
	})
	require.NoError(t, err)
	require.Equal(t, int32(0), orgUser.Version, "first insert should have version 0")

	// Verify membership
	members, total, err := db.ListOrganizationMembers(ctx, projectID, orgID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Equal(t, 1, len(members))
	require.Equal(t, userID, members[0].ID)

	// Verify org-specific data
	var orgData map[string]any
	err = json.Unmarshal(members[0].OrganizationData, &orgData)
	require.NoError(t, err)
	require.Equal(t, "admin", orgData["role"])
	require.Equal(t, "engineering", orgData["department"])
}

func TestUpsertAndGetOrganizationMemberMergesData(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create org and user
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_data_merge"}},
	})
	require.NoError(t, err)

	userID, err := db.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []ExternalIDParam{
		{Source: "anonymous", ExternalID: "anon_data_merge"},
	})
	require.NoError(t, err)

	// Add user with initial data
	orgUser, err := db.UpsertAndGetOrganizationMember(ctx, orgID, userID, map[string]any{
		"role": "member",
	})
	require.NoError(t, err)
	require.Equal(t, int32(0), orgUser.Version, "first insert should have version 0")

	// Add user again with additional data (should merge)
	orgUser, err = db.UpsertAndGetOrganizationMember(ctx, orgID, userID, map[string]any{
		"role":       "admin",
		"department": "sales",
	})
	require.NoError(t, err)
	require.Greater(t, orgUser.Version, int32(0), "second insert should have version > 0")

	// Verify data was merged
	members, _, err := db.ListOrganizationMembers(ctx, projectID, orgID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Equal(t, 1, len(members))

	var orgData map[string]any
	err = json.Unmarshal(members[0].OrganizationData, &orgData)
	require.NoError(t, err)
	require.Equal(t, "admin", orgData["role"])
	require.Equal(t, "sales", orgData["department"])
}

func TestRemoveUserFromOrganization(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create org and user
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_remove"}},
	})
	require.NoError(t, err)

	userID, err := db.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []ExternalIDParam{
		{Source: "anonymous", ExternalID: "anon_remove"},
	})
	require.NoError(t, err)

	// Add and then remove user
	_, err = db.UpsertAndGetOrganizationMember(ctx, orgID, userID, nil)
	require.NoError(t, err)

	err = db.RemoveUserFromOrganization(ctx, orgID, userID)
	require.NoError(t, err)

	// Verify user is no longer a member
	members, total, err := db.ListOrganizationMembers(ctx, projectID, orgID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.Equal(t, 0, len(members))
}

func TestListOrganizationMembersWithPagination(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create org
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_pagination"}},
	})
	require.NoError(t, err)

	// Create and add multiple users
	for i := 0; i < 5; i++ {
		userID, err := db.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []ExternalIDParam{
			{Source: "anonymous", ExternalID: uuid.New().String()},
		})
		require.NoError(t, err)
		_, err = db.UpsertAndGetOrganizationMember(ctx, orgID, userID, nil)
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
		"all members": {
			pagination:    store.Pagination{Limit: 10, Offset: 0},
			expectedCount: 5,
			expectedTotal: 5,
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			members, total, err := db.ListOrganizationMembers(ctx, projectID, orgID, tt.pagination)
			require.NoError(t, err)
			require.Equal(t, tt.expectedCount, len(members))
			require.Equal(t, tt.expectedTotal, total)
		})
	}
}

func TestListUserOrganizations(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create user
	userID, err := db.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []ExternalIDParam{
		{Source: "anonymous", ExternalID: "anon_multi_org"},
	})
	require.NoError(t, err)

	// Create multiple orgs and add user to them
	orgIDs := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
			Identifiers: []ExternalIDParam{{Source: "default", ExternalID: uuid.New().String()}},
			Name:        ptr.To("Test Org"),
		})
		require.NoError(t, err)
		orgIDs[i] = orgID
		_, err = db.UpsertAndGetOrganizationMember(ctx, orgID, userID, nil)
		require.NoError(t, err)
	}

	// List user's orgs
	orgs, total, err := db.ListUserOrganizations(ctx, projectID, userID, store.Pagination{Limit: 100, Offset: 0}, "")
	require.NoError(t, err)
	require.Equal(t, 3, len(orgs))
	require.Equal(t, 3, total)

	// Verify all orgs are returned
	returnedIDs := make(map[uuid.UUID]bool)
	for _, org := range orgs {
		returnedIDs[org.ID] = true
	}
	for _, id := range orgIDs {
		require.True(t, returnedIDs[id], "org should be in user's orgs list")
	}
}

func TestCountOrganizationMembers(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create org
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_count"}},
	})
	require.NoError(t, err)

	// Initially 0 members
	count, err := db.CountOrganizationMembers(ctx, orgID)
	require.NoError(t, err)
	require.Equal(t, 0, count)

	// Add users
	for i := 0; i < 3; i++ {
		userID, err := db.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []ExternalIDParam{
			{Source: "anonymous", ExternalID: uuid.New().String()},
		})
		require.NoError(t, err)
		_, err = db.UpsertAndGetOrganizationMember(ctx, orgID, userID, nil)
		require.NoError(t, err)
	}

	count, err = db.CountOrganizationMembers(ctx, orgID)
	require.NoError(t, err)
	require.Equal(t, 3, count)
}

func TestUpdateOrganizationUserData(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create org and user
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_user_data"}},
	})
	require.NoError(t, err)

	userID, err := db.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []ExternalIDParam{
		{Source: "anonymous", ExternalID: "anon_user_data"},
	})
	require.NoError(t, err)

	// Add user with initial data
	_, err = db.UpsertAndGetOrganizationMember(ctx, orgID, userID, map[string]any{
		"role": "member",
	})
	require.NoError(t, err)

	// Update org user data
	updateData := json.RawMessage(`{"role":"admin","permissions":["read","write"]}`)
	err = db.UpdateOrganizationUserData(ctx, orgID, userID, updateData)
	require.NoError(t, err)

	// Verify update
	members, _, err := db.ListOrganizationMembers(ctx, projectID, orgID, store.Pagination{Limit: 10, Offset: 0})
	require.NoError(t, err)
	require.Equal(t, 1, len(members))

	var orgData map[string]any
	err = json.Unmarshal(members[0].OrganizationData, &orgData)
	require.NoError(t, err)
	require.Equal(t, "admin", orgData["role"])
	require.NotNil(t, orgData["permissions"])
}

func TestDeleteOrganizationCascadesMembers(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create org
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_cascade"}},
	})
	require.NoError(t, err)

	// Create user and add to org
	userID, err := db.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []ExternalIDParam{
		{Source: "anonymous", ExternalID: "anon_cascade"},
	})
	require.NoError(t, err)
	_, err = db.UpsertAndGetOrganizationMember(ctx, orgID, userID, nil)
	require.NoError(t, err)

	// Verify user is member
	count, err := db.CountOrganizationMembers(ctx, orgID)
	require.NoError(t, err)
	require.Equal(t, 1, count)

	// Delete org
	err = db.DeleteOrganization(ctx, projectID, orgID)
	require.NoError(t, err)

	// User should still exist (not cascaded)
	user, err := db.GetUser(ctx, projectID, userID)
	require.NoError(t, err)
	require.NotNil(t, user)

	// User should no longer be in any orgs
	orgs, total, err := db.ListUserOrganizations(ctx, projectID, userID, store.Pagination{Limit: 100, Offset: 0}, "")
	require.NoError(t, err)
	require.Equal(t, 0, len(orgs))
	require.Equal(t, 0, total)
}

func TestUpsertOrganizationSchema(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	paths := rules.Paths{
		{Path: "plan", Type: rules.TypeString},
		{Path: "seats", Type: rules.TypeNumber},
		{Path: "active", Type: rules.TypeBool},
	}

	err := db.UpsertOrganizationSchema(ctx, projectID, paths)
	require.NoError(t, err)

	// Verify schemas were stored
	schemas, err := db.ListOrganizationSchemas(ctx, projectID)
	require.NoError(t, err)
	require.Equal(t, 3, len(schemas))

	// Verify paths and types
	schemaMap := make(map[string][]string)
	for _, s := range schemas {
		schemaMap[s.Path] = s.Types
	}

	require.Contains(t, schemaMap, "plan")
	require.Contains(t, schemaMap["plan"], "string")

	require.Contains(t, schemaMap, "seats")
	require.Contains(t, schemaMap["seats"], "number")

	require.Contains(t, schemaMap, "active")
	require.Contains(t, schemaMap["active"], "boolean")
}

func TestUpsertOrganizationSchemaDeduplication(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	paths := rules.Paths{
		{Path: "plan", Type: rules.TypeString},
	}

	// Insert same schema twice
	err := db.UpsertOrganizationSchema(ctx, projectID, paths)
	require.NoError(t, err)
	err = db.UpsertOrganizationSchema(ctx, projectID, paths)
	require.NoError(t, err)

	// Should still only have one entry
	schemas, err := db.ListOrganizationSchemas(ctx, projectID)
	require.NoError(t, err)
	require.Equal(t, 1, len(schemas))
}

func TestUpsertOrganizationSchemaEmptyPaths(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Should not error with empty paths
	err := db.UpsertOrganizationSchema(ctx, projectID, rules.Paths{})
	require.NoError(t, err)

	schemas, err := db.ListOrganizationSchemas(ctx, projectID)
	require.NoError(t, err)
	require.Equal(t, 0, len(schemas))
}

func TestUpsertOrganizationUserSchema(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	paths := rules.Paths{
		{Path: "role", Type: rules.TypeString},
		{Path: "permissions", Type: rules.TypeString},
		{Path: "level", Type: rules.TypeNumber},
	}

	err := db.UpsertOrganizationUserSchema(ctx, projectID, paths)
	require.NoError(t, err)

	// Verify schemas were stored
	schemas, err := db.ListOrganizationUserSchemas(ctx, projectID)
	require.NoError(t, err)
	require.Equal(t, 3, len(schemas))

	// Verify paths and types
	schemaMap := make(map[string][]string)
	for _, s := range schemas {
		schemaMap[s.Path] = s.Types
	}

	require.Contains(t, schemaMap, "role")
	require.Contains(t, schemaMap["role"], "string")

	require.Contains(t, schemaMap, "permissions")
	require.Contains(t, schemaMap["permissions"], "string")

	require.Contains(t, schemaMap, "level")
	require.Contains(t, schemaMap["level"], "number")
}

func TestUpsertOrganizationUserSchemaDeduplication(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	paths := rules.Paths{
		{Path: "role", Type: rules.TypeString},
	}

	// Insert same schema twice
	err := db.UpsertOrganizationUserSchema(ctx, projectID, paths)
	require.NoError(t, err)
	err = db.UpsertOrganizationUserSchema(ctx, projectID, paths)
	require.NoError(t, err)

	// Should still only have one entry
	schemas, err := db.ListOrganizationUserSchemas(ctx, projectID)
	require.NoError(t, err)
	require.Equal(t, 1, len(schemas))
}

func TestUpsertOrganizationUserSchemaEmptyPaths(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Should not error with empty paths
	err := db.UpsertOrganizationUserSchema(ctx, projectID, rules.Paths{})
	require.NoError(t, err)

	schemas, err := db.ListOrganizationUserSchemas(ctx, projectID)
	require.NoError(t, err)
	require.Equal(t, 0, len(schemas))
}

func TestSelectListOrganizationsDependency(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create a rule that depends on organizations
	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID:              projectID,
		Rule:                   store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		DependsOnOrganizations: true,
		Version:                1,
	})
	require.NoError(t, err)

	// Create a list and publish a version with this rule
	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Org Dependent List",
	})
	require.NoError(t, err)

	versionID, err := db.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)

	err = db.PublishVersion(ctx, listID, versionID)
	require.NoError(t, err)

	// Should find the list as a dependency
	result, err := db.SelectListOrganizationsDependency(ctx, projectID)
	require.NoError(t, err)
	require.Contains(t, result, listID)
}

func TestSelectListOrganizationsDependencyNoMatch(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create a rule that does NOT depend on organizations
	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID:              projectID,
		Rule:                   store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		DependsOnOrganizations: false,
		DependsOnUsers:         true,
		Version:                1,
	})
	require.NoError(t, err)

	// Create a list and publish a version with this rule
	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "User Dependent List",
	})
	require.NoError(t, err)

	versionID, err := db.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)

	err = db.PublishVersion(ctx, listID, versionID)
	require.NoError(t, err)

	// Should NOT find the list as an organization dependency
	result, err := db.SelectListOrganizationsDependency(ctx, projectID)
	require.NoError(t, err)
	require.NotContains(t, result, listID)
}

func TestSelectListOrganizationUsersDependency(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create a rule that depends on organization users
	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID:                  projectID,
		Rule:                       store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		DependsOnOrganizationUsers: true,
		Version:                    1,
	})
	require.NoError(t, err)

	// Create a list and publish a version with this rule
	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Org User Dependent List",
	})
	require.NoError(t, err)

	versionID, err := db.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)

	err = db.PublishVersion(ctx, listID, versionID)
	require.NoError(t, err)

	// Should find the list as a dependency
	result, err := db.SelectListOrganizationUsersDependency(ctx, projectID)
	require.NoError(t, err)
	require.Contains(t, result, listID)
}

func TestSelectListOrganizationUsersDependencyNoMatch(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create a rule that does NOT depend on organization users
	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID:                  projectID,
		Rule:                       store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		DependsOnOrganizationUsers: false,
		DependsOnOrganizations:     true,
		Version:                    1,
	})
	require.NoError(t, err)

	// Create a list and publish a version with this rule
	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Org Dependent List",
	})
	require.NoError(t, err)

	versionID, err := db.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)

	err = db.PublishVersion(ctx, listID, versionID)
	require.NoError(t, err)

	// Should NOT find the list as an organization user dependency
	result, err := db.SelectListOrganizationUsersDependency(ctx, projectID)
	require.NoError(t, err)
	require.NotContains(t, result, listID)
}

func TestLookupOrganizationID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	externalID := "lookup_org_external_123"

	// Create an organization
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: externalID}},
		Name:        ptr.To("Lookup Test Organization"),
	})
	require.NoError(t, err)

	// Lookup the organization ID by external ID
	foundID, err := db.LookupOrganizationID(ctx, projectID, []ExternalIDParam{{Source: "default", ExternalID: externalID}})
	require.NoError(t, err)
	require.Equal(t, orgID, foundID)
}

func TestLookupOrganizationIDNotFound(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Lookup a non-existent organization
	_, err := db.LookupOrganizationID(ctx, projectID, []ExternalIDParam{{Source: "default", ExternalID: "non_existent_org"}})
	require.Error(t, err, "should return error when organization not found")
}

func TestInsertOrganizationEvent(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create an organization
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_event_test"}},
		Name:        ptr.To("Event Test Organization"),
	})
	require.NoError(t, err)

	// Create an event
	eventID, err := db.UpsertEvent(ctx, projectID, "subscription.upgraded", SubjectTypeOrganization)
	require.NoError(t, err)

	// Insert organization event with data
	eventData := map[string]any{
		"plan":     "enterprise",
		"seats":    100,
		"features": []string{"sso", "audit_logs"},
	}
	orgEventID, err := db.InsertOrganizationEvent(ctx, orgID, eventID, eventData)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, orgEventID)
}

func TestInsertOrganizationEventWithNilData(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create an organization
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_event_nil_data"}},
	})
	require.NoError(t, err)

	// Create an event
	eventID, err := db.UpsertEvent(ctx, projectID, "org.created", SubjectTypeOrganization)
	require.NoError(t, err)

	// Insert organization event with nil data
	orgEventID, err := db.InsertOrganizationEvent(ctx, orgID, eventID, nil)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, orgEventID)
}

func TestInsertMultipleOrganizationEvents(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create an organization
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_multi_events"}},
	})
	require.NoError(t, err)

	// Create an event
	eventID, err := db.UpsertEvent(ctx, projectID, "user.joined", SubjectTypeOrganization)
	require.NoError(t, err)

	// Insert multiple events for the same organization
	eventIDs := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		eventIDs[i], err = db.InsertOrganizationEvent(ctx, orgID, eventID, map[string]any{
			"user_number": i + 1,
		})
		require.NoError(t, err)
	}

	// All event IDs should be unique
	uniqueIDs := make(map[uuid.UUID]bool)
	for _, id := range eventIDs {
		require.False(t, uniqueIDs[id], "event IDs should be unique")
		uniqueIDs[id] = true
	}
}

func TestListOrganizationUserIDs(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create an organization
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_user_ids"}},
	})
	require.NoError(t, err)

	// Initially should return empty list
	userIDs, err := db.ListOrganizationUserIDs(ctx, orgID)
	require.NoError(t, err)
	require.Empty(t, userIDs)

	// Create users and add them to the organization
	expectedUserIDs := make([]uuid.UUID, 3)
	for i := 0; i < 3; i++ {
		userID, err := db.CreateUser(ctx, projectID, nil, nil, json.RawMessage(`{}`), nil, nil, []ExternalIDParam{
			{Source: "anonymous", ExternalID: uuid.New().String()},
		})
		require.NoError(t, err)
		expectedUserIDs[i] = userID

		_, err = db.UpsertAndGetOrganizationMember(ctx, orgID, userID, nil)
		require.NoError(t, err)
	}

	// Now should return all user IDs
	userIDs, err = db.ListOrganizationUserIDs(ctx, orgID)
	require.NoError(t, err)
	require.Len(t, userIDs, 3)

	// Verify all expected user IDs are present
	userIDSet := make(map[uuid.UUID]bool)
	for _, id := range userIDs {
		userIDSet[id] = true
	}
	for _, expectedID := range expectedUserIDs {
		require.True(t, userIDSet[expectedID], "expected user ID should be in result")
	}
}

func TestListOrganizationUserIDsEmpty(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create an organization with no members
	orgID, err := db.UpsertOrganization(ctx, projectID, UpsertOrganizationParams{
		Identifiers: []ExternalIDParam{{Source: "default", ExternalID: "org_empty_users"}},
	})
	require.NoError(t, err)

	// Should return empty list, not error
	userIDs, err := db.ListOrganizationUserIDs(ctx, orgID)
	require.NoError(t, err)
	require.Empty(t, userIDs)
}
