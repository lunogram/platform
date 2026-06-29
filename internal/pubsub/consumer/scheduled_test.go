package consumer

import (
	"testing"

	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestResolveAssignmentID(t *testing.T) {
	t.Parallel()

	subject := uuid.New()
	schedule := uuid.New()

	// A provided id is passed through unchanged.
	provided := uuid.New()
	assert.Equal(t, provided, resolveAssignmentID(provided, subject, schedule))

	// An omitted id is derived deterministically from (subject, schedule), so a
	// redelivered message resolves to the same assignment and stays idempotent.
	first := resolveAssignmentID(uuid.Nil, subject, schedule)
	second := resolveAssignmentID(uuid.Nil, subject, schedule)
	require.NotEqual(t, uuid.Nil, first)
	assert.Equal(t, first, second)

	// Different subjects or schedules derive different ids.
	assert.NotEqual(t, first, resolveAssignmentID(uuid.Nil, uuid.New(), schedule))
	assert.NotEqual(t, first, resolveAssignmentID(uuid.Nil, subject, uuid.New()))
}
