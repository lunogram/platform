package journeys

import (
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

	match, err := evaluateGateRules(ctx, resolved, state)
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

// evaluateGateRules splits the rule set into local rules (evaluated in-memory)
// and historical rules (evaluated via SQL), then combines the results using the
// root operator (AND/OR).
func evaluateGateRules(ctx HandlerContext, rs rules.RuleSet, state journey.JourneyUserState) (_ bool, err error) {
	local := rs.Local()
	historical := rs.Historical()

	ctx.logger.Debug("gate: split result",
		zap.Bool("has_local_rules", local != nil),
		zap.Bool("has_historical_rules", historical != nil),
		zap.Int("total_children", len(rs.Children)),
	)

	var localMatch, historicalMatch bool

	if local != nil {
		localMatch, err = evaluateLocalRules(*local, ctx.Data)
		if err != nil {
			return false, err
		}
		ctx.logger.Debug("gate: local evaluation", zap.Bool("match", localMatch))
	}

	// When only local rules exist we can return early.
	if local != nil && historical == nil {
		return localMatch, nil
	}

	// Short-circuit: skip the database query when the local result already
	// determines the outcome.
	if local != nil {
		if rs.Operator == rules.OperatorAnd && !localMatch {
			return false, nil
		}
		if rs.Operator == rules.OperatorOr && localMatch {
			return true, nil
		}
	}

	historicalMatch, err = evaluateHistoricalRules(ctx, *historical, state)
	if err != nil {
		return false, err
	}
	ctx.logger.Debug("gate: historical evaluation", zap.Bool("match", historicalMatch))

	// When only historical rules exist we can return early.
	if local == nil {
		return historicalMatch, nil
	}

	if rs.Operator == rules.OperatorOr {
		return localMatch || historicalMatch, nil
	}

	return localMatch && historicalMatch, nil
}

// evaluateLocalRules evaluates local rules in-memory against the journey's
// data map.
func evaluateLocalRules(rs rules.RuleSet, data map[string]any) (bool, error) {
	return eval.NewEvaluator().Evaluate(rs, data)
}

// evaluateHistoricalRules evaluates historical rules via SQL query.
func evaluateHistoricalRules(ctx HandlerContext, rs rules.RuleSet, state journey.JourneyUserState) (bool, error) {
	builder := query.NewQueryBuilder(ctx.ProjectID, &ctx.UserID).WithSinceTimestamp(state.EnteredAt)
	q, err := builder.Query(rs)
	if err != nil {
		return false, err
	}

	var match uuid.UUID
	err = ctx.DB.GetContext(ctx, &match, q.SQL, q.Args...)
	return err == nil, nil
}
