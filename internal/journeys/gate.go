package journeys

import (
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/node/metrics"
	"github.com/lunogram/platform/internal/render"
	"github.com/lunogram/platform/internal/rules/query"
	"github.com/lunogram/platform/internal/store/journey"
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

	resolvedRule, err := render.RenderRuleSet(config.Rule, ctx.Data)
	if err != nil {
		return nil, err
	}

	builder := query.NewQueryBuilder(ctx.ProjectID, &ctx.UserID).WithSinceTimestamp(state.EnteredAt)
	query, err := builder.Query(resolvedRule)
	if err != nil {
		return nil, err
	}

	var match uuid.UUID
	err = ctx.DB.GetContext(ctx, &match, query.SQL, query.Args...)
	matchesRule := err == nil

	if matchesRule && yes != nil {
		metrics.JourneyGateEvaluationsTotal.WithLabelValues(ctx.ProjectID.String(), "yes").Inc()
		return yes, nil
	}

	if !matchesRule && no != nil {
		metrics.JourneyGateEvaluationsTotal.WithLabelValues(ctx.ProjectID.String(), "no").Inc()
		return no, nil
	}

	metrics.JourneyGateEvaluationsTotal.WithLabelValues(ctx.ProjectID.String(), "none").Inc()
	return nil, nil
}
