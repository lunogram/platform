package journeys

import (
	"fmt"

	"github.com/google/uuid"
	"github.com/lib/pq"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/rules/eval"
	"github.com/lunogram/platform/internal/store/journey"
)

// stepVisitsKey is the reserved root under which resolved visit counts are
// exposed to the in-memory evaluator.
const stepVisitsKey = "step_visits"

// evaluateStepVisitRules resolves how often the user reached each referenced
// step and evaluates the comparisons in-memory.
//
// A visit count includes the visit in progress: a user standing on a step for
// the third time compares against 3. The state row for the current visit is
// only written once the step completes, so the current step is counted one
// higher unless its row already exists (a resumed step).
func evaluateStepVisitRules(ctx HandlerContext, rs rules.RuleSet, step journey.JourneyVersionStep, state journey.JourneyUserState) (bool, error) {
	resolver := newStepVisitResolver(step.ExternalID)
	resolved := resolver.resolve(rs.Rule)

	if len(resolver.targets) == 0 {
		return true, nil
	}

	counts, err := countStepVisits(ctx, resolver.targets, step, state)
	if err != nil {
		return false, err
	}

	return eval.NewEvaluator().Evaluate(rules.RuleSet{Rule: resolved}, counts)
}

// stepVisitTarget identifies a step whose visit count a rule compares against.
type stepVisitTarget struct {
	scope  rules.StepScope
	stepID string
}

// stepVisitResolver rewrites journey_step leaves so they read their count from
// the map built by countStepVisits, collecting the steps to count along the
// way. Counts are addressed by position rather than by step id so that ids
// carrying path syntax cannot alter the lookup.
type stepVisitResolver struct {
	currentStepID string
	targets       []stepVisitTarget
	positions     map[stepVisitTarget]int
}

func newStepVisitResolver(currentStepID string) *stepVisitResolver {
	return &stepVisitResolver{
		currentStepID: currentStepID,
		positions:     map[stepVisitTarget]int{},
	}
}

// resolve rewrites the rule tree. A leaf without a step refers to the step
// being handled.
func (r *stepVisitResolver) resolve(rule rules.Rule) rules.Rule {
	if rule.Group == rules.RuleGroupJourneyStep && !rule.IsWrapper() {
		target := stepVisitTarget{scope: rule.Scope(), stepID: rule.Path}
		if target.stepID == "" {
			target.stepID = r.currentStepID
		}

		position, seen := r.positions[target]
		if !seen {
			position = len(r.targets)
			r.positions[target] = position
			r.targets = append(r.targets, target)
		}

		rule.Path = fmt.Sprintf("%s.%d", stepVisitsKey, position)
		rule.Type = rules.RuleTypeNumber
		return rule
	}

	if len(rule.Children) > 0 {
		children := make([]rules.Rule, len(rule.Children))
		for i, child := range rule.Children {
			children[i] = r.resolve(child)
		}
		rule.Children = children
	}

	return rule
}

// countStepVisits counts how often the user reached each target step, keyed by
// the position the resolver assigned it.
func countStepVisits(ctx HandlerContext, targets []stepVisitTarget, step journey.JourneyVersionStep, state journey.JourneyUserState) (map[string]any, error) {
	steps := map[rules.StepScope][]string{}
	for _, target := range targets {
		steps[target.scope] = append(steps[target.scope], target.stepID)
	}

	recorded := map[rules.StepScope]map[string]int{}
	for scope, stepIDs := range steps {
		counts, err := queryStepVisits(ctx, scope, stepIDs, state)
		if err != nil {
			return nil, err
		}
		recorded[scope] = counts
	}

	visits := make(map[string]any, len(targets))
	for position, target := range targets {
		count := recorded[target.scope][target.stepID]

		// The state row for the visit in progress is written once the step
		// completes, so count it here unless it already exists.
		if target.stepID == step.ExternalID && state.ID == uuid.Nil {
			count++
		}

		visits[fmt.Sprint(position)] = count
	}

	return map[string]any{stepVisitsKey: visits}, nil
}

// queryStepVisits counts the visits recorded for the given steps, either within
// the user's current run through the journey or across all of their runs.
func queryStepVisits(ctx HandlerContext, scope rules.StepScope, stepIDs []string, state journey.JourneyUserState) (map[string]int, error) {
	stmt := `
	SELECT external_step_id, COUNT(*) AS visits
	FROM journey_user_state
	WHERE journey_entry_id = $1 AND external_step_id = ANY($2)
	GROUP BY external_step_id`
	args := []any{state.JourneyEntryID, pq.Array(stepIDs)}

	if scope == rules.StepScopeJourney {
		stmt = `
	SELECT external_step_id, COUNT(*) AS visits
	FROM journey_user_state
	WHERE journey_id = $1 AND user_id = $2 AND external_step_id = ANY($3)
	GROUP BY external_step_id`
		args = []any{state.JourneyID, ctx.UserID, pq.Array(stepIDs)}
	}

	rows, err := ctx.DB.QueryContext(ctx, stmt, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	counts := map[string]int{}
	for rows.Next() {
		var stepID string
		var visits int
		if err := rows.Scan(&stepID, &visits); err != nil {
			return nil, err
		}
		counts[stepID] = visits
	}

	return counts, rows.Err()
}
