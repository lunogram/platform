package subjects

import (
	"context"
	"database/sql"
	"fmt"
	"testing"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
	"github.com/stretchr/testify/require"
)

func TestCreateVersion(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Version Test",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	// First version should be 1
	v1, err := db.CreateVersion(ctx, listID, nil)
	require.NoError(t, err)
	require.NotEqual(t, uuid.Nil, v1)

	ver1, err := db.GetDraftVersion(ctx, listID)
	require.NoError(t, err)
	require.Equal(t, 1, ver1.VersionNumber)
	require.Equal(t, ListVersionStatusDraft, ver1.Status)
	require.Nil(t, ver1.RuleID)

	// Create a second version — should auto-increment to 2
	// First publish v1 so we can create another draft
	err = db.PublishVersion(ctx, listID, v1)
	require.NoError(t, err)

	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID: projectID,
		Rule:      store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		Version:   1,
	})
	require.NoError(t, err)

	v2, err := db.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)

	ver2, err := db.GetDraftVersion(ctx, listID)
	require.NoError(t, err)
	require.Equal(t, 2, ver2.VersionNumber)
	require.Equal(t, ListVersionStatusDraft, ver2.Status)
	require.NotNil(t, ver2.RuleID)
	require.Equal(t, ruleID, *ver2.RuleID)
	require.NotEqual(t, v1, v2)
}

func TestSetListVersionID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Version ID Test",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	// Initially version_id should be nil
	list, err := db.GetList(ctx, projectID, listID)
	require.NoError(t, err)
	require.Nil(t, list.VersionID)

	// Create version and set it
	versionID, err := db.CreateVersion(ctx, listID, nil)
	require.NoError(t, err)

	err = db.SetListVersionID(ctx, listID, versionID)
	require.NoError(t, err)

	list, err = db.GetList(ctx, projectID, listID)
	require.NoError(t, err)
	require.NotNil(t, list.VersionID)
	require.Equal(t, versionID, *list.VersionID)
}

func TestEnsureDraftVersion_CreatesNew(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Ensure Draft New",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	// No versions exist yet — should create one
	version, err := db.EnsureDraftVersion(ctx, projectID, listID)
	require.NoError(t, err)
	require.NotNil(t, version)
	require.Equal(t, ListVersionStatusDraft, version.Status)
	require.Equal(t, 1, version.VersionNumber)
	require.Nil(t, version.RuleID) // no published rule to copy

	// version_id should now point to the draft
	list, err := db.GetList(ctx, projectID, listID)
	require.NoError(t, err)
	require.NotNil(t, list.VersionID)
	require.Equal(t, version.ID, *list.VersionID)
}

func TestEnsureDraftVersion_ReturnsExisting(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Ensure Draft Existing",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	// Create a draft version explicitly
	v1, err := db.EnsureDraftVersion(ctx, projectID, listID)
	require.NoError(t, err)

	// Call again — should return the same draft
	v2, err := db.EnsureDraftVersion(ctx, projectID, listID)
	require.NoError(t, err)
	require.Equal(t, v1.ID, v2.ID)
	require.Equal(t, v1.VersionNumber, v2.VersionNumber)
}

func TestEnsureDraftVersion_DuplicatesPublishedRule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create a rule
	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID:      projectID,
		Rule:           store.JSONB[rules.RuleSet]{Data: rules.RuleSet{Rule: rules.Rule{Type: rules.RuleTypeWrapper, Group: rules.RuleGroupParent}}},
		DependsOnUsers: true,
		Version:        1,
	})
	require.NoError(t, err)

	// Create list, version with rule, and publish
	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Published With Rule",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	versionID, err := db.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)

	err = db.PublishVersion(ctx, listID, versionID)
	require.NoError(t, err)

	// Now ensure draft — should duplicate the published rule
	draft, err := db.EnsureDraftVersion(ctx, projectID, listID)
	require.NoError(t, err)
	require.NotNil(t, draft.RuleID)
	require.NotEqual(t, ruleID, *draft.RuleID, "draft rule should be a copy, not the same ID")

	// Verify the copied rule has the same content
	copiedRule, err := db.GetRule(ctx, projectID, *draft.RuleID)
	require.NoError(t, err)
	require.True(t, copiedRule.DependsOnUsers)
	require.Equal(t, rules.RuleTypeWrapper, copiedRule.Rule.Data.Type)
}

func TestPublishVersion(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID: projectID,
		Rule:      store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		Version:   1,
	})
	require.NoError(t, err)

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Publish Test",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	versionID, err := db.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)

	err = db.SetListVersionID(ctx, listID, versionID)
	require.NoError(t, err)

	// Publish
	err = db.PublishVersion(ctx, listID, versionID)
	require.NoError(t, err)

	// Check status changed
	published, err := db.GetPublishedVersion(ctx, listID)
	require.NoError(t, err)
	require.Equal(t, versionID, published.ID)
	require.Equal(t, ListVersionStatusPublished, published.Status)
	require.NotNil(t, published.PublishedAt)

	// No draft should remain
	_, err = db.GetDraftVersion(ctx, listID)
	require.Error(t, err)
	require.Equal(t, sql.ErrNoRows, err)

	// version_id should point to the published version
	list, err := db.GetList(ctx, projectID, listID)
	require.NoError(t, err)
	require.NotNil(t, list.VersionID)
	require.Equal(t, versionID, *list.VersionID)
}

func TestPublishVersion_ArchivesOldPublished(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Archive Test",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	// Create and publish v1
	v1, err := db.CreateVersion(ctx, listID, nil)
	require.NoError(t, err)
	err = db.PublishVersion(ctx, listID, v1)
	require.NoError(t, err)

	// Create and publish v2
	v2, err := db.CreateVersion(ctx, listID, nil)
	require.NoError(t, err)
	err = db.PublishVersion(ctx, listID, v2)
	require.NoError(t, err)

	// v1 should be archived — verify via GetPublishedVersion (which only finds published)
	// and by checking that the published version is v2, not v1
	published, err := db.GetPublishedVersion(ctx, listID)
	require.NoError(t, err)
	require.Equal(t, v2, published.ID, "v2 should be the published version, meaning v1 was archived")
}

func TestGetDraftVersion(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Draft Get",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	// No draft yet
	_, err = db.GetDraftVersion(ctx, listID)
	require.Error(t, err)

	// Create draft
	vID, err := db.CreateVersion(ctx, listID, nil)
	require.NoError(t, err)

	draft, err := db.GetDraftVersion(ctx, listID)
	require.NoError(t, err)
	require.Equal(t, vID, draft.ID)
	require.Equal(t, ListVersionStatusDraft, draft.Status)
}

func TestGetPublishedVersion(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Published Get",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	// No published yet
	_, err = db.GetPublishedVersion(ctx, listID)
	require.Error(t, err)

	// Create and publish
	vID, err := db.CreateVersion(ctx, listID, nil)
	require.NoError(t, err)
	err = db.PublishVersion(ctx, listID, vID)
	require.NoError(t, err)

	pub, err := db.GetPublishedVersion(ctx, listID)
	require.NoError(t, err)
	require.Equal(t, vID, pub.ID)
	require.Equal(t, ListVersionStatusPublished, pub.Status)
}

func TestUpdateVersionRuleID(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Update Rule Test",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	versionID, err := db.CreateVersion(ctx, listID, nil)
	require.NoError(t, err)

	// Initially no rule
	draft, err := db.GetDraftVersion(ctx, listID)
	require.NoError(t, err)
	require.Nil(t, draft.RuleID)

	// Create and assign rule
	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID: projectID,
		Rule:      store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		Version:   1,
	})
	require.NoError(t, err)

	err = db.UpdateVersionRuleID(ctx, versionID, ruleID)
	require.NoError(t, err)

	draft, err = db.GetDraftVersion(ctx, listID)
	require.NoError(t, err)
	require.NotNil(t, draft.RuleID)
	require.Equal(t, ruleID, *draft.RuleID)
}

func TestGetPublishedRule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	ruleSet := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeWrapper,
			Group:    rules.RuleGroupParent,
			Operator: rules.OperatorAnd,
		},
	}
	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID: projectID,
		Rule:      store.JSONB[rules.RuleSet]{Data: ruleSet},
		Version:   1,
	})
	require.NoError(t, err)

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Published Rule Test",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	versionID, err := db.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)

	err = db.PublishVersion(ctx, listID, versionID)
	require.NoError(t, err)

	result, err := db.GetPublishedRule(ctx, listID)
	require.NoError(t, err)
	require.NotNil(t, result)
	require.Equal(t, rules.RuleTypeWrapper, result.Type)
	require.Equal(t, rules.RuleGroupParent, result.Group)
	require.Equal(t, rules.OperatorAnd, result.Operator)
}

func TestGetPublishedRule_NilWhenDraftOnly(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID: projectID,
		Rule:      store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		Version:   1,
	})
	require.NoError(t, err)

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Draft Only",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	// Create draft version with rule — don't publish
	_, err = db.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)

	result, err := db.GetPublishedRule(ctx, listID)
	require.NoError(t, err)
	require.Nil(t, result, "draft-only list should not return a published rule")
}

func TestDuplicateList(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID:      projectID,
		Rule:           store.JSONB[rules.RuleSet]{Data: rules.RuleSet{Rule: rules.Rule{Type: rules.RuleTypeWrapper}}},
		DependsOnUsers: true,
		Version:        1,
	})
	require.NoError(t, err)

	// Create and publish a list
	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Original",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	versionID, err := db.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)
	err = db.PublishVersion(ctx, listID, versionID)
	require.NoError(t, err)

	// Duplicate
	newID, err := db.DuplicateList(ctx, projectID, listID, "Copy of Original")
	require.NoError(t, err)
	require.NotEqual(t, listID, newID)

	// New list should exist with a draft version
	newList, err := db.GetList(ctx, projectID, newID)
	require.NoError(t, err)
	require.Equal(t, "Copy of Original", newList.Name)
	require.Equal(t, ListVersionStatusDraft, newList.State)

	// Draft version should have a duplicated rule (different ID)
	draftVersion, err := db.GetDraftVersion(ctx, newID)
	require.NoError(t, err)
	require.NotNil(t, draftVersion.RuleID)
	require.NotEqual(t, ruleID, *draftVersion.RuleID)

	// Duplicated rule should have the same content
	copiedRule, err := db.GetRule(ctx, projectID, *draftVersion.RuleID)
	require.NoError(t, err)
	require.True(t, copiedRule.DependsOnUsers)
}

func TestDuplicateList_NoPublishedRule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create a list without any versions
	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "No Rule",
		Type:      ListTypeStatic,
	})
	require.NoError(t, err)

	newID, err := db.DuplicateList(ctx, projectID, listID, "Copy of No Rule")
	require.NoError(t, err)

	newList, err := db.GetList(ctx, projectID, newID)
	require.NoError(t, err)
	require.Equal(t, "Copy of No Rule", newList.Name)
	// No version should be created if there's no published rule to copy
	require.Nil(t, newList.VersionID)
}

func TestDuplicateRule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	ruleSet := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeWrapper,
			Group:    rules.RuleGroupParent,
			Operator: rules.OperatorOr,
			Children: []rules.Rule{
				{Type: rules.RuleTypeWrapper, Group: rules.RuleGroupUser},
			},
		},
	}

	origID, err := db.CreateRule(ctx, Rule{
		ProjectID:              projectID,
		Rule:                   store.JSONB[rules.RuleSet]{Data: ruleSet},
		DependsOnUsers:         true,
		DependsOnOrganizations: true,
		Version:                1,
	})
	require.NoError(t, err)

	dupID, err := db.DuplicateRule(ctx, projectID, origID)
	require.NoError(t, err)
	require.NotEqual(t, origID, dupID)

	dup, err := db.GetRule(ctx, projectID, dupID)
	require.NoError(t, err)
	require.True(t, dup.DependsOnUsers)
	require.True(t, dup.DependsOnOrganizations)
	require.Equal(t, rules.OperatorOr, dup.Rule.Data.Operator)
	require.Len(t, dup.Rule.Data.Children, 1)
}

func TestSelectListUsersDependency_OnlyPublished(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID:      projectID,
		Rule:           store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		DependsOnUsers: true,
		Version:        1,
	})
	require.NoError(t, err)

	// Draft-only list
	draftListID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Draft Only",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)
	_, err = db.CreateVersion(ctx, draftListID, &ruleID)
	require.NoError(t, err)

	// Draft list should NOT appear
	result, err := db.SelectListUsersDependency(ctx, projectID)
	require.NoError(t, err)
	require.NotContains(t, result, draftListID)

	// Published list
	pubRuleID, err := db.CreateRule(ctx, Rule{
		ProjectID:      projectID,
		Rule:           store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		DependsOnUsers: true,
		Version:        1,
	})
	require.NoError(t, err)

	pubListID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Published",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	vID, err := db.CreateVersion(ctx, pubListID, &pubRuleID)
	require.NoError(t, err)
	err = db.PublishVersion(ctx, pubListID, vID)
	require.NoError(t, err)

	result, err = db.SelectListUsersDependency(ctx, projectID)
	require.NoError(t, err)
	require.Contains(t, result, pubListID)
	require.NotContains(t, result, draftListID)
}

func TestSelectListOrganizationsDependency_OnlyPublished(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID:              projectID,
		Rule:                   store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		DependsOnOrganizations: true,
		Version:                1,
	})
	require.NoError(t, err)

	// Draft-only list
	draftListID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Draft Org",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)
	_, err = db.CreateVersion(ctx, draftListID, &ruleID)
	require.NoError(t, err)

	result, err := db.SelectListOrganizationsDependency(ctx, projectID)
	require.NoError(t, err)
	require.NotContains(t, result, draftListID)

	// Published list
	pubRuleID, err := db.CreateRule(ctx, Rule{
		ProjectID:              projectID,
		Rule:                   store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		DependsOnOrganizations: true,
		Version:                1,
	})
	require.NoError(t, err)

	pubListID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Published Org",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	vID, err := db.CreateVersion(ctx, pubListID, &pubRuleID)
	require.NoError(t, err)
	err = db.PublishVersion(ctx, pubListID, vID)
	require.NoError(t, err)

	result, err = db.SelectListOrganizationsDependency(ctx, projectID)
	require.NoError(t, err)
	require.Contains(t, result, pubListID)
	require.NotContains(t, result, draftListID)
}

func TestSelectListOrganizationUsersDependency_OnlyPublished(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID:                  projectID,
		Rule:                       store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		DependsOnOrganizationUsers: true,
		Version:                    1,
	})
	require.NoError(t, err)

	// Draft-only list
	draftListID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Draft Org Users",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)
	_, err = db.CreateVersion(ctx, draftListID, &ruleID)
	require.NoError(t, err)

	result, err := db.SelectListOrganizationUsersDependency(ctx, projectID)
	require.NoError(t, err)
	require.NotContains(t, result, draftListID)

	// Published list
	pubRuleID, err := db.CreateRule(ctx, Rule{
		ProjectID:                  projectID,
		Rule:                       store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		DependsOnOrganizationUsers: true,
		Version:                    1,
	})
	require.NoError(t, err)

	pubListID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Published Org Users",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	vID, err := db.CreateVersion(ctx, pubListID, &pubRuleID)
	require.NoError(t, err)
	err = db.PublishVersion(ctx, pubListID, vID)
	require.NoError(t, err)

	result, err = db.SelectListOrganizationUsersDependency(ctx, projectID)
	require.NoError(t, err)
	require.Contains(t, result, pubListID)
	require.NotContains(t, result, draftListID)
}

func TestListEventListDependencies_OnlyPublished(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create an event first
	eventID, err := db.UpsertEvent(ctx, projectID, "test.event", SubjectTypeUser)
	require.NoError(t, err)

	// Create a rule that depends on events
	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID:       projectID,
		Rule:            store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		DependsOnEvents: true,
		Version:         1,
	})
	require.NoError(t, err)

	// Link rule to event
	err = db.SetRuleEventDependencies(ctx, projectID, ruleID, []EventDependency{
		{Name: "test.event", SubjectType: SubjectTypeUser},
	})
	require.NoError(t, err)

	// Draft-only list
	draftListID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Draft Event",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)
	_, err = db.CreateVersion(ctx, draftListID, &ruleID)
	require.NoError(t, err)

	// Should not find draft list
	result, err := db.ListEventListDependencies(ctx, eventID)
	require.NoError(t, err)
	require.NotContains(t, result, draftListID)

	// Create another rule with events for a published list
	pubRuleID, err := db.CreateRule(ctx, Rule{
		ProjectID:       projectID,
		Rule:            store.JSONB[rules.RuleSet]{Data: rules.RuleSet{}},
		DependsOnEvents: true,
		Version:         1,
	})
	require.NoError(t, err)
	err = db.SetRuleEventDependencies(ctx, projectID, pubRuleID, []EventDependency{
		{Name: "test.event", SubjectType: SubjectTypeUser},
	})
	require.NoError(t, err)

	pubListID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Published Event",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	vID, err := db.CreateVersion(ctx, pubListID, &pubRuleID)
	require.NoError(t, err)
	err = db.PublishVersion(ctx, pubListID, vID)
	require.NoError(t, err)

	result, err = db.ListEventListDependencies(ctx, eventID)
	require.NoError(t, err)
	require.Contains(t, result, pubListID)
	require.NotContains(t, result, draftListID)
}

func TestGetList_StateDraft(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "State Draft",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	// Create a draft version and point to it
	vID, err := db.CreateVersion(ctx, listID, nil)
	require.NoError(t, err)
	err = db.SetListVersionID(ctx, listID, vID)
	require.NoError(t, err)

	list, err := db.GetList(ctx, projectID, listID)
	require.NoError(t, err)
	require.Equal(t, ListVersionStatusDraft, list.State)
}

func TestGetList_StatePublished(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "State Published",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	vID, err := db.CreateVersion(ctx, listID, nil)
	require.NoError(t, err)
	err = db.PublishVersion(ctx, listID, vID)
	require.NoError(t, err)

	list, err := db.GetList(ctx, projectID, listID)
	require.NoError(t, err)
	require.Equal(t, ListVersionStatusPublished, list.State)
}

func TestGetList_DraftRule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	ruleSet := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeWrapper,
			Group:    rules.RuleGroupUser,
			Operator: rules.OperatorAnd,
		},
	}
	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID: projectID,
		Rule:      store.JSONB[rules.RuleSet]{Data: ruleSet},
		Version:   1,
	})
	require.NoError(t, err)

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Draft Rule Test",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	vID, err := db.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)
	err = db.SetListVersionID(ctx, listID, vID)
	require.NoError(t, err)

	list, err := db.GetList(ctx, projectID, listID)
	require.NoError(t, err)
	require.NotNil(t, list.DraftRule, "draft_rule should be populated")
	require.Equal(t, rules.RuleGroupUser, list.DraftRule.Data.Group)
	require.Nil(t, list.Rule, "published rule should be nil for draft-only list")
}

func TestGetList_PublishedRule(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	ruleSet := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeWrapper,
			Group:    rules.RuleGroupOrganization,
			Operator: rules.OperatorOr,
		},
	}
	ruleID, err := db.CreateRule(ctx, Rule{
		ProjectID: projectID,
		Rule:      store.JSONB[rules.RuleSet]{Data: ruleSet},
		Version:   1,
	})
	require.NoError(t, err)

	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Published Rule Test",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	vID, err := db.CreateVersion(ctx, listID, &ruleID)
	require.NoError(t, err)
	err = db.PublishVersion(ctx, listID, vID)
	require.NoError(t, err)

	list, err := db.GetList(ctx, projectID, listID)
	require.NoError(t, err)
	require.NotNil(t, list.Rule, "published rule should be populated")
	require.Equal(t, rules.RuleGroupOrganization, list.Rule.Data.Group)
}

func TestListLists(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create lists in different states
	draftListID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Draft List",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)
	draftV, err := db.CreateVersion(ctx, draftListID, nil)
	require.NoError(t, err)
	err = db.SetListVersionID(ctx, draftListID, draftV)
	require.NoError(t, err)

	pubListID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Published List",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)
	pubV, err := db.CreateVersion(ctx, pubListID, nil)
	require.NoError(t, err)
	err = db.PublishVersion(ctx, pubListID, pubV)
	require.NoError(t, err)

	// Also create a static list with no versions
	_, err = db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Static List",
		Type:      ListTypeStatic,
	})
	require.NoError(t, err)

	lists, total, err := db.ListLists(ctx, projectID, store.Pagination{Limit: 10, Offset: 0}, "", false)
	require.NoError(t, err)
	require.Equal(t, 3, total)
	require.Len(t, lists, 3)

	// Verify states
	stateMap := make(map[string]ListVersionStatus)
	for _, l := range lists {
		stateMap[l.Name] = l.State
	}
	require.Equal(t, ListVersionStatusDraft, stateMap["Draft List"])
	require.Equal(t, ListVersionStatusPublished, stateMap["Published List"])
	require.Equal(t, ListVersionStatusDraft, stateMap["Static List"]) // default when no version
}

func TestListOAPI_StateMappings(t *testing.T) {
	t.Parallel()

	// published -> ready
	published := List{State: ListVersionStatusPublished}
	require.Equal(t, "ready", string(published.OAPI().State))

	// draft -> draft
	draft := List{State: ListVersionStatusDraft}
	require.Equal(t, "draft", string(draft.OAPI().State))
}

func TestListOAPI_VersionNumber(t *testing.T) {
	t.Parallel()

	// VersionNumber 0 -> nil
	l0 := List{VersionNumber: 0}
	require.Nil(t, l0.OAPI().VersionNumber)

	// VersionNumber > 0 -> populated
	l1 := List{VersionNumber: 3}
	require.NotNil(t, l1.OAPI().VersionNumber)
	require.Equal(t, 3, *l1.OAPI().VersionNumber)
}

func TestPreviewListUsers(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create users with different data
	_, err := db.CreateUser(ctx, projectID, ptr.To("alice@example.com"), nil, []byte(`{"name":"Alice","age":30}`), nil, nil, []ExternalIDParam{{Source: "default", ExternalID: "alice"}})
	require.NoError(t, err)

	_, err = db.CreateUser(ctx, projectID, ptr.To("bob@example.com"), nil, []byte(`{"name":"Bob","age":17}`), nil, nil, []ExternalIDParam{{Source: "default", ExternalID: "bob"}})
	require.NoError(t, err)

	_, err = db.CreateUser(ctx, projectID, ptr.To("carol@example.com"), nil, []byte(`{"name":"Carol","age":25}`), nil, nil, []ExternalIDParam{{Source: "default", ExternalID: "carol"}})
	require.NoError(t, err)

	// Rule: age >= 18 — should match Alice (30) and Carol (25) but not Bob (17)
	ruleset := rules.RuleSet{
		Rule: rules.Rule{
			UUID:     uuid.New(),
			Path:     "user.data.age",
			Group:    rules.RuleGroupUser,
			Type:     rules.RuleTypeNumber,
			Operator: rules.OperatorGreaterEqual,
			Value:    float64(18),
		},
	}

	users, total, err := db.PreviewListUsers(ctx, projectID, ruleset, 25)
	require.NoError(t, err)
	require.Equal(t, 2, total, "should match 2 users (age >= 18)")
	require.Len(t, users, 2)

	// Verify the returned users are Alice and Carol (not Bob)
	extIDs := make(map[string]bool)
	for _, u := range users {
		rec := u.ExternalIDBySource("default")
		require.NotNil(t, rec)
		extIDs[rec.ExternalID] = true
	}
	require.True(t, extIDs["alice"], "Alice should match")
	require.True(t, extIDs["carol"], "Carol should match")
	require.False(t, extIDs["bob"], "Bob should not match")
}

func TestPreviewListUsers_RespectsLimit(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create 5 users
	for i := 0; i < 5; i++ {
		_, err := db.CreateUser(ctx, projectID, nil, nil, []byte(`{"name":"Test"}`), nil, nil, []ExternalIDParam{{Source: "default", ExternalID: fmt.Sprintf("user_%d", i)}})
		require.NoError(t, err)
	}

	// Rule that matches all users (just a wrapper with no conditions)
	ruleset := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeWrapper,
			Group:    rules.RuleGroupParent,
			Operator: rules.OperatorAnd,
		},
	}

	// Request limit of 2
	users, total, err := db.PreviewListUsers(ctx, projectID, ruleset, 2)
	require.NoError(t, err)
	require.Equal(t, 5, total, "total should reflect all matching users")
	require.Len(t, users, 2, "returned users should be limited to 2")
}

func TestPreviewListUsers_NoMatches(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create a user with age 10
	_, err := db.CreateUser(ctx, projectID, nil, nil, []byte(`{"name":"Young","age":10}`), nil, nil, []ExternalIDParam{{Source: "default", ExternalID: "young"}})
	require.NoError(t, err)

	// Rule: age >= 100 — should match nobody
	ruleset := rules.RuleSet{
		Rule: rules.Rule{
			UUID:     uuid.New(),
			Path:     "user.data.age",
			Group:    rules.RuleGroupUser,
			Type:     rules.RuleTypeNumber,
			Operator: rules.OperatorGreaterEqual,
			Value:    float64(100),
		},
	}

	users, total, err := db.PreviewListUsers(ctx, projectID, ruleset, 25)
	require.NoError(t, err)
	require.Equal(t, 0, total)
	require.Empty(t, users)
}

func TestPreviewListUsers_DoesNotWriteListUsers(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	// Create users
	_, err := db.CreateUser(ctx, projectID, nil, nil, []byte(`{"name":"User1"}`), nil, nil, []ExternalIDParam{{Source: "default", ExternalID: "user1"}})
	require.NoError(t, err)

	// Create a dynamic list
	listID, err := db.CreateList(ctx, List{
		ProjectID: projectID,
		Name:      "Preview No Write",
		Type:      ListTypeDynamic,
	})
	require.NoError(t, err)

	// Rule that matches all users
	ruleset := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeWrapper,
			Group:    rules.RuleGroupParent,
			Operator: rules.OperatorAnd,
		},
	}

	// Preview should return users
	users, total, err := db.PreviewListUsers(ctx, projectID, ruleset, 25)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, users, 1)

	// list_users should still be empty — preview doesn't persist
	listUsers, listTotal, err := db.SelectListUsers(ctx, projectID, listID, store.Pagination{Limit: 100, Offset: 0}, "")
	require.NoError(t, err)
	require.Equal(t, 0, listTotal, "list_users should be empty after preview")
	require.Empty(t, listUsers)
}

func TestListOAPI_Rules(t *testing.T) {
	t.Parallel()

	rs := store.JSONB[rules.RuleSet]{Data: rules.RuleSet{Rule: rules.Rule{Type: rules.RuleTypeWrapper}}}
	l := List{Rule: &rs, DraftRule: &rs}
	o := l.OAPI()
	require.NotNil(t, o.Rule)
	require.NotNil(t, o.DraftRule)

	// nil rules
	l2 := List{}
	o2 := l2.OAPI()
	require.Nil(t, o2.Rule)
	require.Nil(t, o2.DraftRule)
}

func TestListLists_ArchivedFilter(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	activeID, err := db.CreateList(ctx, List{ProjectID: projectID, Name: "Active List", Type: ListTypeDynamic})
	require.NoError(t, err)

	archivedID, err := db.CreateList(ctx, List{ProjectID: projectID, Name: "Archived List", Type: ListTypeDynamic})
	require.NoError(t, err)
	require.NoError(t, db.DeleteList(ctx, projectID, archivedID))

	page := store.Pagination{Limit: 10, Offset: 0}

	// archivedOnly=false returns only active lists, with a total that excludes archived ones.
	active, total, err := db.ListLists(ctx, projectID, page, "", false)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, active, 1)
	require.Equal(t, activeID, active[0].ID)
	require.Nil(t, active[0].DeletedAt)

	// archivedOnly=true returns only archived lists, with a matching total for pagination.
	archived, total, err := db.ListLists(ctx, projectID, page, "", true)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, archived, 1)
	require.Equal(t, archivedID, archived[0].ID)
	require.NotNil(t, archived[0].DeletedAt)
}

func TestUnarchiveList(t *testing.T) {
	t.Parallel()

	db := NewContainerStore(t)
	projectID := uuid.New()
	ctx := context.Background()

	listID, err := db.CreateList(ctx, List{ProjectID: projectID, Name: "Restore Me", Type: ListTypeDynamic})
	require.NoError(t, err)
	require.NoError(t, db.DeleteList(ctx, projectID, listID))

	// Restoring an archived list clears deleted_at and brings it back to the active list.
	require.NoError(t, db.UnarchiveList(ctx, projectID, listID))

	active, total, err := db.ListLists(ctx, projectID, store.Pagination{Limit: 10, Offset: 0}, "", false)
	require.NoError(t, err)
	require.Equal(t, 1, total)
	require.Len(t, active, 1)
	require.Equal(t, listID, active[0].ID)

	// Unarchiving a list that is not archived (already active) reports no rows affected.
	err = db.UnarchiveList(ctx, projectID, listID)
	require.ErrorIs(t, err, sql.ErrNoRows)

	// Unarchiving a non-existent list reports no rows affected.
	err = db.UnarchiveList(ctx, projectID, uuid.New())
	require.ErrorIs(t, err, sql.ErrNoRows)
}
