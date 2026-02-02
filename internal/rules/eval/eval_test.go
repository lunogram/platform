package eval

import (
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/rules"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestEvaluatorSimpleUserRules(t *testing.T) {
	t.Parallel()

	evaluator := NewEvaluator()

	type test struct {
		ruleSet rules.RuleSet
		data    map[string]any
		want    bool
		wantErr bool
	}

	tests := map[string]test{
		"string equals - match": {
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".email",
					Operator: rules.OperatorEquals,
					Value:    "user@example.com",
				},
			},
			data: map[string]any{
				"email": "user@example.com",
			},
			want:    true,
			wantErr: false,
		},
		"string contains - match": {
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".name",
					Operator: rules.OperatorContains,
					Value:    "john",
				},
			},
			data: map[string]any{
				"name": "John Doe",
			},
			want:    true,
			wantErr: false,
		},
		"number greater than - match": {
			ruleSet: rules.RuleSet{
				Rule: rules.Rule{
					Type:     rules.RuleTypeNumber,
					Group:    rules.RuleGroupUser,
					Path:     ".age",
					Operator: rules.OperatorGreaterThan,
					Value:    18,
				},
			},
			data: map[string]any{
				"age": 25,
			},
			want:    true,
			wantErr: false,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			got, err := evaluator.Evaluate(tc.ruleSet, tc.data)
			if tc.wantErr {
				assert.Error(t, err)
			} else {
				require.NoError(t, err)
				assert.Equal(t, tc.want, got)
			}
		})
	}
}

func TestEvaluatorNestedPaths(t *testing.T) {
	t.Parallel()

	evaluator := NewEvaluator()

	ruleSet := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeString,
			Group:    rules.RuleGroupUser,
			Path:     ".data.subscription.tier",
			Operator: rules.OperatorEquals,
			Value:    "premium",
		},
	}

	data := map[string]any{
		"data": map[string]any{
			"subscription": map[string]any{
				"tier": "premium",
			},
		},
	}

	got, err := evaluator.Evaluate(ruleSet, data)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestEvaluatorLogicalOperators(t *testing.T) {
	t.Parallel()

	evaluator := NewEvaluator()

	ruleSet := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeWrapper,
			Group:    rules.RuleGroupParent,
			Operator: rules.OperatorAnd,
			Children: []rules.Rule{
				{
					Type:     rules.RuleTypeNumber,
					Group:    rules.RuleGroupUser,
					Path:     ".age",
					Operator: rules.OperatorGreaterThan,
					Value:    18,
				},
				{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupUser,
					Path:     ".country",
					Operator: rules.OperatorEquals,
					Value:    "US",
				},
			},
		},
	}

	data := map[string]any{
		"age":     25,
		"country": "US",
	}

	got, err := evaluator.Evaluate(ruleSet, data)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestEvaluatorEventRules(t *testing.T) {
	t.Parallel()

	evaluator := NewEvaluator()

	ruleSet := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeWrapper,
			Group:    rules.RuleGroupParent,
			Operator: rules.OperatorAnd,
			Children: []rules.Rule{
				{
					Type:     rules.RuleTypeString,
					Group:    rules.RuleGroupEvent,
					Path:     ".name",
					Operator: rules.OperatorEquals,
					Value:    "order.created",
				},
				{
					Type:     rules.RuleTypeNumber,
					Group:    rules.RuleGroupEvent,
					Path:     ".data.amount",
					Operator: rules.OperatorGreaterThan,
					Value:    100,
				},
			},
		},
	}

	data := map[string]any{
		"name": "order.created",
		"data": map[string]any{
			"amount": 150,
		},
	}

	got, err := evaluator.Evaluate(ruleSet, data)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestEvaluatorDateOperators(t *testing.T) {
	t.Parallel()

	evaluator := NewEvaluator()
	now := time.Now()
	yesterday := now.AddDate(0, 0, -1)

	ruleSet := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeDate,
			Group:    rules.RuleGroupUser,
			Path:     ".created_at",
			Operator: rules.OperatorGreaterThan,
			Value:    yesterday,
		},
	}

	data := map[string]any{
		"created_at": now,
	}

	got, err := evaluator.Evaluate(ruleSet, data)
	require.NoError(t, err)
	assert.True(t, got)
}

func TestEvaluatorFrequencyReturnsError(t *testing.T) {
	t.Parallel()

	evaluator := NewEvaluator()

	ruleSet := rules.RuleSet{
		Rule: rules.Rule{
			Type:  rules.RuleTypeWrapper,
			Group: rules.RuleGroupEvent,
			Value: "purchase.completed",
			Frequency: &rules.Frequency{
				Count: 3,
				Period: rules.Period{
					Type:  rules.PeriodTypeRolling,
					Unit:  rules.PeriodUnitDay,
					Value: 7,
				},
				Operator: rules.OperatorGreaterThan,
			},
		},
	}

	data := map[string]any{
		"name": "purchase.completed",
	}

	_, err := evaluator.Evaluate(ruleSet, data)
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "cannot evaluate frequency-based rules in memory")
}

func BenchmarkEvaluator_SimpleRule(b *testing.B) {
	evaluator := NewEvaluator()

	ruleSet := rules.RuleSet{
		Rule: rules.Rule{
			Type:     rules.RuleTypeString,
			Group:    rules.RuleGroupUser,
			Path:     ".email",
			Operator: rules.OperatorEquals,
			Value:    "user@example.com",
			UUID:     uuid.New(),
		},
	}

	data := map[string]any{
		"email": "user@example.com",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, _ = evaluator.Evaluate(ruleSet, data)
	}
}
