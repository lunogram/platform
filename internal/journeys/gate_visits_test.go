package journeys

import (
	"context"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap"
)

func stepVisitRule(stepID string, operator rules.Operator, value any, scope *rules.StepScope) rules.Rule {
	return rules.Rule{
		Type:      rules.RuleTypeNumber,
		Group:     rules.RuleGroupJourneyStep,
		Path:      stepID,
		Operator:  operator,
		Value:     value,
		StepScope: scope,
	}
}

func stepVisitRuleSet(operator rules.Operator, children ...rules.Rule) rules.RuleSet {
	return rules.RuleSet{Rule: rules.Rule{
		Type:     rules.RuleTypeWrapper,
		Group:    rules.RuleGroupParent,
		Operator: operator,
		Children: children,
	}}
}

func TestEvaluateStepVisitRules(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	projectID := uuid.New()
	userID := uuid.New()
	journeyID := uuid.New()
	entryID := uuid.New()

	journeyScope := rules.StepScopeJourney

	gate := journey.JourneyVersionStep{Type: GateStepType, ExternalID: "gate"}
	state := journey.JourneyUserState{JourneyID: journeyID, JourneyEntryID: entryID}

	visitRows := func(stepID string, visits int) *sqlmock.Rows {
		return sqlmock.NewRows([]string{"external_step_id", "visits"}).AddRow(stepID, visits)
	}

	t.Run("counts the visit in progress", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		// Two recorded visits plus the one in progress means the user is
		// passing the gate for the third time.
		mock.ExpectQuery("FROM journey_user_state").
			WithArgs(entryID, sqlmock.AnyArg()).
			WillReturnRows(visitRows("gate", 2))

		hctx := HandlerContext{Context: ctx, DB: db, ProjectID: projectID, UserID: userID, logger: logger}

		match, err := evaluateStepVisitRules(hctx, stepVisitRuleSet(rules.OperatorAnd,
			stepVisitRule("", rules.OperatorGreaterThan, 2, nil),
		), gate, state)

		require.NoError(t, err)
		assert.True(t, match)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("does not count the visit in progress when the state row exists", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		mock.ExpectQuery("FROM journey_user_state").
			WithArgs(entryID, sqlmock.AnyArg()).
			WillReturnRows(visitRows("gate", 3))

		hctx := HandlerContext{Context: ctx, DB: db, ProjectID: projectID, UserID: userID, logger: logger}

		resumed := state
		resumed.ID = uuid.New()

		match, err := evaluateStepVisitRules(hctx, stepVisitRuleSet(rules.OperatorAnd,
			stepVisitRule("", rules.OperatorGreaterThan, 3, nil),
		), gate, resumed)

		require.NoError(t, err)
		assert.False(t, match)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("a step never reached counts as zero", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		mock.ExpectQuery("FROM journey_user_state").
			WithArgs(entryID, sqlmock.AnyArg()).
			WillReturnRows(sqlmock.NewRows([]string{"external_step_id", "visits"}))

		hctx := HandlerContext{Context: ctx, DB: db, ProjectID: projectID, UserID: userID, logger: logger}

		match, err := evaluateStepVisitRules(hctx, stepVisitRuleSet(rules.OperatorAnd,
			stepVisitRule("reminder", rules.OperatorGreaterThan, 0, nil),
		), gate, state)

		require.NoError(t, err)
		assert.False(t, match)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("journey scope counts every run", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		mock.ExpectQuery("FROM journey_user_state").
			WithArgs(journeyID, userID, sqlmock.AnyArg()).
			WillReturnRows(visitRows("reminder", 4))

		hctx := HandlerContext{Context: ctx, DB: db, ProjectID: projectID, UserID: userID, logger: logger}

		match, err := evaluateStepVisitRules(hctx, stepVisitRuleSet(rules.OperatorAnd,
			stepVisitRule("reminder", rules.OperatorGreaterThan, 3, &journeyScope),
		), gate, state)

		require.NoError(t, err)
		assert.True(t, match)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("each scope is counted with its own query", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		mock.MatchExpectationsInOrder(false)
		mock.ExpectQuery("FROM journey_user_state").
			WithArgs(entryID, sqlmock.AnyArg()).
			WillReturnRows(visitRows("gate", 1))
		mock.ExpectQuery("FROM journey_user_state").
			WithArgs(journeyID, userID, sqlmock.AnyArg()).
			WillReturnRows(visitRows("gate", 9))

		hctx := HandlerContext{Context: ctx, DB: db, ProjectID: projectID, UserID: userID, logger: logger}

		match, err := evaluateStepVisitRules(hctx, stepVisitRuleSet(rules.OperatorAnd,
			stepVisitRule("", rules.OperatorLessEqual, 2, nil),
			stepVisitRule("", rules.OperatorGreaterThan, 5, &journeyScope),
		), gate, state)

		require.NoError(t, err)
		assert.True(t, match)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("the same step is counted once for both rules", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		mock.ExpectQuery("FROM journey_user_state").
			WithArgs(entryID, sqlmock.AnyArg()).
			WillReturnRows(visitRows("gate", 2))

		hctx := HandlerContext{Context: ctx, DB: db, ProjectID: projectID, UserID: userID, logger: logger}

		match, err := evaluateStepVisitRules(hctx, stepVisitRuleSet(rules.OperatorAnd,
			stepVisitRule("", rules.OperatorGreaterThan, 1, nil),
			stepVisitRule("", rules.OperatorLessThan, 5, nil),
		), gate, state)

		require.NoError(t, err)
		assert.True(t, match)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("a step id carrying path syntax is counted as itself", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		mock.ExpectQuery("FROM journey_user_state").
			WithArgs(entryID, sqlmock.AnyArg()).
			WillReturnRows(visitRows("step.with.dots", 4))

		hctx := HandlerContext{Context: ctx, DB: db, ProjectID: projectID, UserID: userID, logger: logger}

		match, err := evaluateStepVisitRules(hctx, stepVisitRuleSet(rules.OperatorAnd,
			stepVisitRule("step.with.dots", rules.OperatorGreaterThan, 3, nil),
		), gate, state)

		require.NoError(t, err)
		assert.True(t, match)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("query failures surface", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		mock.ExpectQuery("FROM journey_user_state").WillReturnError(sqlmock.ErrCancelled)

		hctx := HandlerContext{Context: ctx, DB: db, ProjectID: projectID, UserID: userID, logger: logger}

		_, err = evaluateStepVisitRules(hctx, stepVisitRuleSet(rules.OperatorAnd,
			stepVisitRule("", rules.OperatorGreaterThan, 1, nil),
		), gate, state)

		require.Error(t, err)
	})
}

func TestEvaluateGateRulesWithStepVisits(t *testing.T) {
	logger := zap.NewNop()
	ctx := context.Background()

	projectID := uuid.New()
	userID := uuid.New()
	entryID := uuid.New()

	gate := journey.JourneyVersionStep{Type: GateStepType, ExternalID: "gate"}
	state := journey.JourneyUserState{JourneyEntryID: entryID}

	userRule := rules.Rule{
		Type:     rules.RuleTypeString,
		Group:    rules.RuleGroupUser,
		Path:     ".tier",
		Operator: rules.OperatorEquals,
		Value:    "pro",
	}

	t.Run("combines step visits with historical rules", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		mock.ExpectQuery("FROM journey_user_state").
			WillReturnRows(sqlmock.NewRows([]string{"external_step_id", "visits"}).AddRow("gate", 0))
		mock.ExpectQuery("^SELECT u\\.id FROM users u").
			WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(userID.String()))

		hctx := HandlerContext{Context: ctx, DB: db, ProjectID: projectID, UserID: userID, logger: logger}

		match, err := evaluateGateRules(hctx, stepVisitRuleSet(rules.OperatorAnd,
			stepVisitRule("", rules.OperatorLessEqual, 3, nil),
			userRule,
		), gate, state)

		require.NoError(t, err)
		assert.True(t, match)
		assert.NoError(t, mock.ExpectationsWereMet())
	})

	t.Run("a failed step visit rule short-circuits the historical query", func(t *testing.T) {
		mockDB, mock, err := sqlmock.New()
		require.NoError(t, err)
		defer mockDB.Close()
		db := sqlx.NewDb(mockDB, "sqlmock")

		mock.ExpectQuery("FROM journey_user_state").
			WillReturnRows(sqlmock.NewRows([]string{"external_step_id", "visits"}).AddRow("gate", 5))

		hctx := HandlerContext{Context: ctx, DB: db, ProjectID: projectID, UserID: userID, logger: logger}

		match, err := evaluateGateRules(hctx, stepVisitRuleSet(rules.OperatorAnd,
			stepVisitRule("", rules.OperatorLessEqual, 3, nil),
			userRule,
		), gate, state)

		require.NoError(t, err)
		assert.False(t, match)
		assert.NoError(t, mock.ExpectationsWereMet())
	})
}
