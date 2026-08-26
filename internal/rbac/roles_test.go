package rbac

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsProjectRoleHigher(t *testing.T) {
	t.Parallel()

	assert.True(t, IsProjectRoleHigher(ProjectAdmin, ProjectEditor))
	assert.True(t, IsProjectRoleHigher(ProjectEditor, ProjectSupport))
	assert.False(t, IsProjectRoleHigher(ProjectSupport, ProjectAdmin))
	assert.False(t, IsProjectRoleHigher(ProjectEditor, ProjectEditor))

	// An unranked role must never out-rank a real one, and must never be
	// treated as a ceiling that a real role fits under.
	assert.False(t, IsProjectRoleHigher("owner", ProjectSupport))
	assert.True(t, IsProjectRoleHigher(ProjectSupport, "owner"))
}

func TestIsKnownProjectRole(t *testing.T) {
	t.Parallel()

	for _, role := range []string{ProjectSupport, ProjectClient, ProjectEditor, ProjectAdmin} {
		assert.True(t, IsKnownProjectRole(role), role)
	}
	// "owner" is an organization role; the project type has no such relation.
	assert.False(t, IsKnownProjectRole(OrganizationOwner))
	assert.False(t, IsKnownProjectRole(""))
}

func TestEffectiveProjectRole(t *testing.T) {
	t.Parallel()

	assert.Equal(t, ProjectAdmin, EffectiveProjectRole(OrganizationOwner, ""))
	assert.Equal(t, ProjectAdmin, EffectiveProjectRole(OrganizationAdmin, ProjectSupport))
	assert.Equal(t, ProjectSupport, EffectiveProjectRole(OrganizationMember, ProjectSupport))
	assert.Equal(t, "", EffectiveProjectRole(OrganizationMember, ""))
	assert.Equal(t, "", EffectiveProjectRole("", ""))
}
