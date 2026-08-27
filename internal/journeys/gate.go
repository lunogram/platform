package journeys

import (
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/rules/eval"
	"github.com/lunogram/platform/internal/rules/query"
	"github.com/lunogram/platform/internal/store/journey"
	"go.uber.org/zap"
)

const (
	GateYes = "yes"
	GateNo  = "no"
)

type GateData struct {
	SelectedExternalID string `json:"selected_child"`
}

func HandleGate(ctx HandlerContext, step journey.JourneyVersionStep, state journey.JourneyUserState) (journey.JourneyUserState, journey.JourneyVersionStepChildren, error) {
	config, err := DecodeStepData[oapi.GateStepData](step.Data)
	if err != nil {
		return state, nil, err
	}

	selected, err := selectGateBranch(ctx, step, config, state)
	if err != nil {
		return state, nil, err
	}

	state.CompletedAt = Now()

	if selected == nil {
		return state, nil, nil
	}

	state, err = WithStateData(state, GateData{
		SelectedExternalID: selected.ChildExternalID,
	})
	if err != nil {
		return state, nil, err
	}

	return state, []journey.JourneyVersionStepChild{*selected}, nil
}

func selectGateBranch(ctx HandlerContext, step journey.JourneyVersionStep, config oapi.GateStepData, state journey.JourneyUserState) (*journey.JourneyVersionStepChild, error) {
	if len(step.Children) == 0 {
		return nil, nil
	}

	var yes *journey.JourneyVersionStepChild
	var no *journey.JourneyVersionStepChild

	for i := range step.Children {
		child := &step.Children[i]
		if child.Path == nil {
			continue
		}

		switch *child.Path {
		case GateYes:
			yes = child
		case GateNo:
			no = child
		}
	}

	resolved, err := render.RenderRuleSet(config.Rule, ctx.Data)
	if err != nil {
		ctx.logger.Warn("gate: render error", zap.String("step_id", step.ExternalID), zap.Error(err))
		return nil, err
	}

	match, err := evaluateGateRules(ctx, resolved, step, state)
	if err != nil {
		ctx.logger.Warn("gate: evaluation error", zap.String("step_id", step.ExternalID), zap.Error(err))
		return nil, err
	}

	ctx.logger.Debug("gate: evaluation result",
		zap.String("step_id", step.ExternalID),
		zap.Bool("matches", match),
	)

	if match && yes != nil {
		metrics.JourneyGateEvaluationsTotal.WithLabelValues(ctx.ProjectID.String(), "yes").Inc()
		return yes, nil
	}

	if !match && no != nil {
		metrics.JourneyGateEvaluationsTotal.WithLabelValues(ctx.ProjectID.String(), "no").Inc()
		return no, nil
	}

	metrics.JourneyGateEvaluationsTotal.WithLabelValues(ctx.ProjectID.String(), "none").Inc()
	return nil, nil
}

// evaluateGateRules splits the rule set into local rules (evaluated in-memory),
// step visit rules (counted against the journey state) and historical rules
// (evaluated via SQL), then combines the results using the root operator
// (AND/OR). Partitions are evaluated cheapest first so a decided AND/OR
// short-circuits before the database is queried.
func evaluateGateRules(ctx HandlerContext, rs rules.RuleSet, step journey.JourneyVersionStep, state journey.JourneyUserState) (bool, error) {
	partitions := []struct {
		name     string
		ruleSet  *rules.RuleSet
		evaluate func(rules.RuleSet) (bool, error)
	}{
		{
			name:    "local",
			ruleSet: rs.Local(),
			evaluate: func(set rules.RuleSet) (bool, error) {
				return evaluateLocalRules(set, ctx.Data)
			},
		},
		{
			name:    "step_visits",
			ruleSet: rs.StepVisits(),
			evaluate: func(set rules.RuleSet) (bool, error) {
				return evaluateStepVisitRules(ctx, set, step, state)
			},
		},
		{
			name:    "historical",
			ruleSet: rs.Historical(),
			evaluate: func(set rules.RuleSet) (bool, error) {
				return evaluateHistoricalRules(ctx, set, state)
			},
		},
	}

	match := true
	decided := false

	for _, partition := range partitions {
		if partition.ruleSet == nil {
			continue
		}

		result, err := partition.evaluate(*partition.ruleSet)
		if err != nil {
			return false, err
		}

		ctx.logger.Debug("gate: partition evaluation",
			zap.String("partition", partition.name),
			zap.Bool("match", result),
		)

		if !decided {
			match, decided = result, true
		} else if rs.Operator == rules.OperatorOr {
			match = match || result
		} else {
			match = match && result
		}

		if rs.Operator == rules.OperatorOr && match {
			break
		}
		if rs.Operator != rules.OperatorOr && !match {
			break
		}
	}

	return match, nil
}

// evaluateLocalRules evaluates local rules in-memory against the journey's
// data map.
func evaluateLocalRules(rs rules.RuleSet, data map[string]any) (bool, error) {
	return eval.NewEvaluator().Evaluate(rs, data)
}

// evaluateHistoricalRules evaluates historical rules via SQL query.
func evaluateHistoricalRules(ctx HandlerContext, rs rules.RuleSet, state journey.JourneyUserState) (bool, error) {
	start := time.Now()

	builder := query.NewQueryBuilder(ctx.ProjectID, &ctx.UserID).WithSinceTimestamp(state.EnteredAt)
	q, err := builder.Query(rs)
	if err != nil {
		metrics.ObserveDatasetQuery(metrics.QueryGateHistorical, ctx.ProjectID, start, 0, err)
		return false, err
	}

	var match uuid.UUID
	err = ctx.DB.GetContext(ctx, &match, q.SQL, q.Args...)
	matched := err == nil

	// A miss and a query failure are indistinguishable to the caller, which
	// treats both as "did not match", so neither is recorded as an error here.
	rows := 0
	if matched {
		rows = 1
	}
	metrics.ObserveDatasetQuery(metrics.QueryGateHistorical, ctx.ProjectID, start, rows, nil)

	return matched, nil
}
