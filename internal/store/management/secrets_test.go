package management

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestHashSecretIsDeterministic(t *testing.T) {
	t.Parallel()
	assert.Equal(t, hashSecret("pk_abc"), hashSecret("pk_abc"))
	assert.NotEqual(t, hashSecret("pk_abc"), hashSecret("pk_abd"))
	assert.Len(t, hashSecret("anything"), 64) // hex-encoded SHA-256
}

func TestNewSecretPrefixesByScope(t *testing.T) {
	t.Parallel()

	pub, pubPrefix, pubHash, err := newSecret(ScopePublic)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(pub, "pk_"))
	assert.Equal(t, pub[:secretPrefixLen], pubPrefix)
	assert.Equal(t, hashSecret(pub), pubHash)

	sec, _, _, err := newSecret(ScopeSecret)
	require.NoError(t, err)
	assert.True(t, strings.HasPrefix(sec, "sk_"))

	// Two generations never collide.
	other, _, _, err := newSecret(ScopePublic)
	require.NoError(t, err)
	assert.NotEqual(t, pub, other)
}
