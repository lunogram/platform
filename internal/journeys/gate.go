package journeys

import (
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/rules/query"
	"github.com/lunogram/platform/internal/store"
)

const (
	GateYes = "yes"
	GateNo  = "no"
)

type GateData struct {
	SelectedExternalID string `json:"selected_child"`
}

func HandleGate(ctx HandlerContext, step store.JourneyVersionStep, state store.JourneyUserState) (store.JourneyUserState, store.JourneyVersionStepChildren, error) {
	config, err := DecodeStepData[oapi.GateStepData](step.Data)
	if err != nil {
		return state, nil, err
	}

	selected, err := selectGateBranch(ctx, step, config)
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

	return state, []store.JourneyVersionStepChild{*selected}, nil
}

func selectGateBranch(ctx HandlerContext, step store.JourneyVersionStep, config oapi.GateStepData) (*store.JourneyVersionStepChild, error) {
	if len(step.Children) == 0 {
		return nil, nil
	}

	var yes *store.JourneyVersionStepChild
	var no *store.JourneyVersionStepChild

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

	builder := query.NewQueryBuilder(ctx.ProjectID, &ctx.UserID)
	query, err := builder.Query(config.Rule)
	if err != nil {
		return nil, err
	}

	var match uuid.UUID
	err = ctx.DB.GetContext(ctx, &match, query.SQL, query.Args...)
	matchesRule := err == nil

	if matchesRule && yes != nil {
		return yes, nil
	}

	if !matchesRule && no != nil {
		return no, nil
	}

	return nil, nil
}
