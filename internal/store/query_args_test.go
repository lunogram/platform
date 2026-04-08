package store

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestQueryArgs_Add(t *testing.T) {
	qa := NewQueryArgs()

	p1 := qa.Add("project-1")
	p2 := qa.Add("source-a")
	p3 := qa.Add(42)

	assert.Equal(t, "$1", p1)
	assert.Equal(t, "$2", p2)
	assert.Equal(t, "$3", p3)
	assert.Equal(t, 3, qa.Len())
	assert.Equal(t, []any{"project-1", "source-a", 42}, qa.Args())
}

func TestQueryArgs_Clause(t *testing.T) {
	qa := NewQueryArgs()

	clause := qa.Clause("(source = %s AND external_id = %s)", qa.Add("github"), qa.Add("ext-123"))

	assert.Equal(t, "(source = $1 AND external_id = $2)", clause)
	assert.Equal(t, []any{"github", "ext-123"}, qa.Args())
}

func TestQueryArgs_MultipleClausesIncrementCorrectly(t *testing.T) {
	qa := NewQueryArgs()

	projectPlaceholder := qa.Add("proj-1")
	assert.Equal(t, "$1", projectPlaceholder)

	c1 := qa.Clause("(source = %s AND external_id = %s)", qa.Add("s1"), qa.Add("e1"))
	c2 := qa.Clause("(source = %s AND external_id = %s)", qa.Add("s2"), qa.Add("e2"))

	assert.Equal(t, "(source = $2 AND external_id = $3)", c1)
	assert.Equal(t, "(source = $4 AND external_id = $5)", c2)
	assert.Equal(t, 5, qa.Len())
	assert.Equal(t, []any{"proj-1", "s1", "e1", "s2", "e2"}, qa.Args())
}

func TestQueryArgs_Empty(t *testing.T) {
	qa := NewQueryArgs()

	assert.Equal(t, 0, qa.Len())
	assert.Empty(t, qa.Args())
}

func TestQueryArgs_NilValue(t *testing.T) {
	qa := NewQueryArgs()

	p := qa.Add(nil)

	assert.Equal(t, "$1", p)
	assert.Equal(t, []any{nil}, qa.Args())
}

func TestQueryArgs_ClauseWithSinglePlaceholder(t *testing.T) {
	qa := NewQueryArgs()

	clause := qa.Clause("project_id = %s", qa.Add("proj-abc"))

	assert.Equal(t, "project_id = $1", clause)
}
