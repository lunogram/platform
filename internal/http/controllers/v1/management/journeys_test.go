package v1

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestCreateJourney(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := management.Config{
		URI: container.RunPostgreSQL(t),
	}

	err := management.Migrate(config)
	require.NoError(t, err)

	db, err := management.New(ctx, logger, config)
	require.NoError(t, err)

	projects := management.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	journeys := NewJourneysController(logger, db)

	type test struct {
		body oapi.CreateJourneyJSONRequestBody
		code int
	}

	description := "A test journey description"

	tests := map[string]test{
		"simple": {
			body: oapi.CreateJourneyJSONRequestBody{
				Name: "Welcome Journey",
			},
			code: 201,
		},
		"with_description": {
			body: oapi.CreateJourneyJSONRequestBody{
				Name:        "Onboarding Journey",
				Description: &description,
			},
			code: 201,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/journeys", bytes.NewReader(bb))
			journeys.CreateJourney(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 201 {
				var journey oapi.Journey
				err = json.Unmarshal(res.Body.Bytes(), &journey)
				require.NoError(t, err)
				require.Equal(t, test.body.Name, journey.Name)
				require.Equal(t, projectID, journey.ProjectId)
				if test.body.Description != nil {
					require.NotNil(t, journey.Description)
					require.Equal(t, *test.body.Description, *journey.Description)
				}
			}
		})
	}
}

func TestListJourneys(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := management.Config{
		URI: container.RunPostgreSQL(t),
	}

	err := management.Migrate(config)
	require.NoError(t, err)

	db, err := management.New(ctx, logger, config)
	require.NoError(t, err)

	projects := management.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	journeysStore := journey.NewJourneysStore(db)

	for i := 0; i < 5; i++ {
		_, err := journeysStore.CreateJourney(ctx, journey.Journey{
			ProjectID: projectID,
			Name:      "Test Journey",
		})
		require.NoError(t, err)
	}

	journeys := NewJourneysController(logger, db)

	type test struct {
		limit  int
		offset int
		code   int
		count  int
		total  int
	}

	tests := map[string]test{
		"default_pagination": {
			limit:  20,
			offset: 0,
			code:   200,
			count:  5,
			total:  5,
		},
		"with_limit": {
			limit:  2,
			offset: 0,
			code:   200,
			count:  2,
			total:  5,
		},
		"with_offset": {
			limit:  2,
			offset: 3,
			code:   200,
			count:  2,
			total:  5,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/journeys", nil)

			limit := oapi.Limit(test.limit)
			offset := oapi.Offset(test.offset)

			params := oapi.ListJourneysParams{
				Limit:  &limit,
				Offset: &offset,
			}

			journeys.ListJourneys(res, req, projectID, params)

			require.Equal(t, test.code, res.Code, res.Body.String())

			var response oapi.JourneyListResponse
			err = json.Unmarshal(res.Body.Bytes(), &response)
			require.NoError(t, err)
			require.Equal(t, test.total, response.Total)
			require.Equal(t, test.count, len(response.Results))
			require.Equal(t, test.limit, response.Limit)
			require.Equal(t, test.offset, response.Offset)
		})
	}
}

func TestGetJourney(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := management.Config{
		URI: container.RunPostgreSQL(t),
	}

	err := management.Migrate(config)
	require.NoError(t, err)

	db, err := management.New(ctx, logger, config)
	require.NoError(t, err)

	projects := management.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	journeysStore := journey.NewJourneysStore(db)
	description := "Test Description"
	journeyID, err := journeysStore.CreateJourney(ctx, journey.Journey{
		ProjectID:   projectID,
		Name:        "Test Journey",
		Description: &description,
	})
	require.NoError(t, err)

	journeys := NewJourneysController(logger, db)

	type test struct {
		journeyID uuid.UUID
		code      int
	}

	tests := map[string]test{
		"success": {
			journeyID: journeyID,
			code:      200,
		},
		"not_found": {
			journeyID: uuid.New(),
			code:      404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/journeys/"+test.journeyID.String(), nil)
			journeys.GetJourney(res, req, projectID, test.journeyID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var journey oapi.Journey
				err = json.Unmarshal(res.Body.Bytes(), &journey)
				require.NoError(t, err)
				require.Equal(t, test.journeyID, journey.Id)
				require.Equal(t, "Test Journey", journey.Name)
				require.NotNil(t, journey.Description)
				require.Equal(t, description, *journey.Description)
			}
		})
	}
}

func TestUpdateJourney(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := management.Config{
		URI: container.RunPostgreSQL(t),
	}

	err := management.Migrate(config)
	require.NoError(t, err)

	db, err := management.New(ctx, logger, config)
	require.NoError(t, err)

	projects := management.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	journeysStore := journey.NewJourneysStore(db)
	journeyID, err := journeysStore.CreateJourney(ctx, journey.Journey{
		ProjectID: projectID,
		Name:      "Original Journey",
	})
	require.NoError(t, err)

	journeys := NewJourneysController(logger, db)

	type test struct {
		body oapi.UpdateJourneyJSONRequestBody
		code int
	}

	newName := "Updated Journey"
	newDescription := "Updated description"

	tests := map[string]test{
		"update_name": {
			body: oapi.UpdateJourneyJSONRequestBody{
				Name: &newName,
			},
			code: 200,
		},
		"update_description": {
			body: oapi.UpdateJourneyJSONRequestBody{
				Description: &newDescription,
			},
			code: 200,
		},
		"update_all": {
			body: oapi.UpdateJourneyJSONRequestBody{
				Name:        &newName,
				Description: &newDescription,
			},
			code: 200,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			bb, err := json.Marshal(test.body)
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("PATCH", "/v1/journeys/"+journeyID.String(), bytes.NewReader(bb))
			journeys.UpdateJourney(res, req, projectID, journeyID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var journey oapi.Journey
				err = json.Unmarshal(res.Body.Bytes(), &journey)
				require.NoError(t, err)
				require.Equal(t, journeyID, journey.Id)

				if test.body.Name != nil {
					require.Equal(t, *test.body.Name, journey.Name)
				}
				if test.body.Description != nil {
					require.NotNil(t, journey.Description)
					require.Equal(t, *test.body.Description, *journey.Description)
				}
			}
		})
	}
}

func TestDeleteJourney(t *testing.T) {
	t.Parallel()

	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	config := management.Config{
		URI: container.RunPostgreSQL(t),
	}

	err := management.Migrate(config)
	require.NoError(t, err)

	db, err := management.New(ctx, logger, config)
	require.NoError(t, err)

	projects := management.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	journeysStore := journey.NewJourneysStore(db)
	journeyID, err := journeysStore.CreateJourney(ctx, journey.Journey{
		ProjectID: projectID,
		Name:      "Journey to Delete",
	})
	require.NoError(t, err)

	journeys := NewJourneysController(logger, db)

	type test struct {
		journeyID uuid.UUID
		code      int
	}

	tests := map[string]test{
		"success": {
			journeyID: journeyID,
			code:      204,
		},
		"not_found": {
			journeyID: uuid.New(),
			code:      404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE", "/v1/journeys/"+test.journeyID.String(), nil)
			journeys.DeleteJourney(res, req, projectID, test.journeyID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 204 {
				journey, err := journeysStore.GetJourney(ctx, projectID, test.journeyID)
				require.Error(t, err)
				require.Nil(t, journey)
			}
		})
	}
}
