package journeys

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type mockPublisher struct {
	publishedEvents []mockEvent
}

type mockEvent struct {
	subject schemas.Subject
	data    any
}

func (m *mockPublisher) Publish(ctx context.Context, subject schemas.Subject, v any, _ ...pubsub.PublishOption) error {
	m.publishedEvents = append(m.publishedEvents, mockEvent{
		subject: subject,
		data:    v,
	})
	return nil
}

func TestHandleEvent(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectID := uuid.New()
	userID := uuid.New()

	type test struct {
		step           journey.JourneyVersionStep
		state          journey.JourneyUserState
		data           map[string]any
		expectedEvents int
		wantErr        bool
	}

	tests := map[string]test{
		"simple event with no template": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: EventStepType,
				Data: json.RawMessage(`{"event_name":"journey_milestone"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:          journey.JourneyUserState{},
			data:           map[string]any{},
			expectedEvents: 1,
			wantErr:        false,
		},
		"event with static template": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: EventStepType,
				Data: json.RawMessage(`{"event_name":"completed_step","template":"{\"milestone\":\"step_1\"}"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:          journey.JourneyUserState{},
			data:           map[string]any{},
			expectedEvents: 1,
			wantErr:        false,
		},
		"event with liquid template variables": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: EventStepType,
				Data: json.RawMessage(`{"event_name":"product_viewed","template":"{\"product_name\":\"{{ journey.product.name }}\",\"category\":\"{{ journey.product.category }}\"}"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state: journey.JourneyUserState{},
			data: map[string]any{
				"journey": map[string]any{
					"product": map[string]any{
						"name":     "Premium Widget",
						"category": "widgets",
					},
				},
			},
			expectedEvents: 1,
			wantErr:        false,
		},
		"event_name with liquid variable": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: EventStepType,
				Data: json.RawMessage(`{"event_name":"{{ journey.entrance.event_type }}_completed"}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state: journey.JourneyUserState{},
			data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"event_type": "onboarding",
					},
				},
			},
			expectedEvents: 1,
			wantErr:        false,
		},
		"event with empty template": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: EventStepType,
				Data: json.RawMessage(`{"event_name":"simple_event","template":""}`),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next-step"},
				},
			},
			state:          journey.JourneyUserState{},
			data:           map[string]any{},
			expectedEvents: 1,
			wantErr:        false,
		},
		"missing event_name": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: EventStepType,
				Data: json.RawMessage(`{"template":"{\"key\":\"value\"}"}`),
			},
			state:          journey.JourneyUserState{},
			data:           map[string]any{},
			expectedEvents: 0,
			wantErr:        true,
		},
		"invalid JSON template": {
			step: journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: EventStepType,
				Data: json.RawMessage(`{"event_name":"test_event","template":"not valid json"}`),
			},
			state:          journey.JourneyUserState{},
			data:           map[string]any{},
			expectedEvents: 0,
			wantErr:        true,
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()
			db := sqlx.NewDb(mockDB, "sqlmock")

			mockPub := &mockPublisher{}

			if !tc.wantErr {
				// Expect GetUser query
				now := time.Now()
				rows := sqlmock.NewRows([]string{
					"id", "project_id", "email", "phone", "data",
					"timezone", "locale", "version", "created_at", "updated_at",
					"has_push_device", "external_ids",
				}).AddRow(
					userID, projectID, nil, nil, []byte("{}"),
					nil, nil, int32(0), now, now,
					false, []byte("[]"),
				)
				mock.ExpectQuery(`SELECT (.+) FROM users`).
					WithArgs(userID, projectID).
					WillReturnRows(rows)
			}

			hctx := HandlerContext{
				Context:   ctx,
				DB:        db,
				Publisher: mockPub,
				ProjectID: projectID,
				UserID:    userID,
				Data:      tc.data,
			}

			gotState, gotChildren, err := HandleEvent(hctx, tc.step, tc.state)

			if tc.wantErr {
				assert.Error(t, err)
				return
			}

			require.NoError(t, err)
			assert.NotNil(t, gotState.CompletedAt)

			if tc.expectedEvents > 0 {
				assert.Len(t, mockPub.publishedEvents, tc.expectedEvents)
				assert.NoError(t, mock.ExpectationsWereMet())
				require.Len(t, gotChildren, 1)
				assert.Equal(t, "next-step", gotChildren[0].ChildExternalID)

				// Verify event structure
				event := mockPub.publishedEvents[0]
				assert.Equal(t, schemas.Subject("users.events.process."+projectID.String()), event.subject)

				eventData, ok := event.data.(schemas.UserEvent)
				require.True(t, ok, "event data should be schemas.UserEvent type")
				assert.NotNil(t, eventData.ID)
				assert.NotEmpty(t, eventData.Name)
				assert.Equal(t, projectID, eventData.ProjectID)
				assert.Equal(t, userID, eventData.UserID)
			}
		})
	}
}

func TestHandleEventTemplateRendering(t *testing.T) {
	t.Parallel()

	ctx := context.Background()
	projectID := uuid.New()
	userID := uuid.New()

	type test struct {
		template     string
		data         map[string]any
		eventName    string
		expectedName string
		checkPayload func(t *testing.T, data map[string]any)
	}

	tests := map[string]test{
		"renders user data": {
			template:     `{"user_email":"{{ user.email }}","user_name":"{{ user.name }}"}`,
			eventName:    "user_action",
			expectedName: "user_action",
			data: map[string]any{
				"user": map[string]any{
					"email": "test@example.com",
					"name":  "Test User",
				},
			},
			checkPayload: func(t *testing.T, data map[string]any) {
				assert.Equal(t, "test@example.com", data["user_email"])
				assert.Equal(t, "Test User", data["user_name"])
			},
		},
		"renders journey step data": {
			template:     `{"product_id":"{{ journey.entrance.product_id }}","quantity":{{ journey.entrance.quantity }}}`,
			eventName:    "cart_updated",
			expectedName: "cart_updated",
			data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"product_id": "prod-123",
						"quantity":   5,
					},
				},
			},
			checkPayload: func(t *testing.T, data map[string]any) {
				assert.Equal(t, "prod-123", data["product_id"])
				// quantity is rendered as string but parsed as float64 by JSON
				assert.Equal(t, float64(5), data["quantity"])
			},
		},
		"renders complex nested data": {
			template:     `{"summary":"{{ user.first_name }} {{ user.last_name }} completed {{ journey.course.name }}","course_id":"{{ journey.course.id }}"}`,
			eventName:    "course_completed",
			expectedName: "course_completed",
			data: map[string]any{
				"user": map[string]any{
					"first_name": "John",
					"last_name":  "Doe",
				},
				"journey": map[string]any{
					"course": map[string]any{
						"id":   "course-456",
						"name": "Advanced Go",
					},
				},
			},
			checkPayload: func(t *testing.T, data map[string]any) {
				assert.Equal(t, "John Doe completed Advanced Go", data["summary"])
				assert.Equal(t, "course-456", data["course_id"])
			},
		},
		"renders event_name with variable": {
			template:     `{"step":"done"}`,
			eventName:    "{{ journey.entrance.event_type }}_completed",
			expectedName: "onboarding_completed",
			data: map[string]any{
				"journey": map[string]any{
					"entrance": map[string]any{
						"event_type": "onboarding",
					},
				},
			},
			checkPayload: func(t *testing.T, data map[string]any) {
				assert.Equal(t, "done", data["step"])
			},
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			mockDB, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer mockDB.Close()
			db := sqlx.NewDb(mockDB, "sqlmock")

			mockPub := &mockPublisher{}

			// Expect GetUser query
			now := time.Now()
			rows := sqlmock.NewRows([]string{
				"id", "project_id", "email", "phone", "data",
				"timezone", "locale", "version", "created_at", "updated_at",
				"has_push_device", "external_ids",
			}).AddRow(
				userID, projectID, nil, nil, []byte("{}"),
				nil, nil, int32(0), now, now,
				false, []byte("[]"),
			)
			mock.ExpectQuery(`SELECT (.+) FROM users`).
				WithArgs(userID, projectID).
				WillReturnRows(rows)

			stepData := map[string]any{
				"event_name": tc.eventName,
				"template":   tc.template,
			}
			stepDataJSON, _ := json.Marshal(stepData)

			step := journey.JourneyVersionStep{
				ID:   uuid.New(),
				Type: EventStepType,
				Data: json.RawMessage(stepDataJSON),
				Children: []journey.JourneyVersionStepChild{
					{ChildExternalID: "next"},
				},
			}

			hctx := HandlerContext{
				Context:   ctx,
				DB:        db,
				Publisher: mockPub,
				ProjectID: projectID,
				UserID:    userID,
				Data:      tc.data,
			}

			gotState, gotChildren, err := HandleEvent(hctx, step, journey.JourneyUserState{})

			require.NoError(t, err)
			assert.NotNil(t, gotState.CompletedAt)
			require.Len(t, gotChildren, 1)

			// Verify the event was published
			require.Len(t, mockPub.publishedEvents, 1)
			assert.NoError(t, mock.ExpectationsWereMet())

			// Check the event payload
			event := mockPub.publishedEvents[0]
			eventData, ok := event.data.(schemas.UserEvent)
			require.True(t, ok)

			if tc.expectedName != "" {
				assert.Equal(t, tc.expectedName, eventData.Name)
			}

			payloadData := eventData.Data
			require.True(t, ok)

			if tc.checkPayload != nil {
				tc.checkPayload(t, payloadData)
			}
		})
	}
}
