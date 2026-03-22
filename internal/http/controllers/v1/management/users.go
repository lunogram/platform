package v1

import (
	"context"
	"database/sql"
	"encoding/csv"
	"errors"
	"io"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/importer"
	"github.com/lunogram/platform/internal/pubsub"
	"github.com/lunogram/platform/internal/pubsub/schemas"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/journey"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewUsersController(logger *zap.Logger, pub pubsub.Publisher, usersDB, journeyDB *sqlx.DB, mgmt *management.State, maxUploadSize int64, engine *rbac.Engine) *UsersController {
	return &UsersController{
		logger:        logger,
		usersDB:       usersDB,
		mgmt:          mgmt,
		users:         subjects.NewState(usersDB, logger),
		journey:       journey.NewState(journeyDB),
		pubsub:        pub,
		maxUploadSize: maxUploadSize,
		engine:        engine,
	}
}

type UsersController struct {
	logger        *zap.Logger
	usersDB       *sqlx.DB
	pubsub        pubsub.Publisher
	mgmt          *management.State
	users         *subjects.State
	journey       *journey.State
	maxUploadSize int64
	engine        *rbac.Engine
}

func (srv *UsersController) ListUsers(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListUsersParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("users", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))

	search := params.Search.ToString()
	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing users", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	users, total, err := srv.users.ListUsers(ctx, projectID, pagination, search)
	if err != nil {
		logger.Error("failed to list users", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("users listed", zap.Int("total", total), zap.Int("count", len(users)))

	response := oapi.UserList{
		Results: users.OAPI(),
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *UsersController) IdentifyUser(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("users", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.IdentifyUser{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	if body.AnonymousId == nil && body.ExternalId == nil {
		srv.logger.Error("either anonymous_id or external_id required")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("either anonymous_id or external_id required")))
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))
	logger.Info("upserting user")

	var data map[string]any
	if body.Data != nil {
		data = *body.Data
	}

	tx, err := srv.usersDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	defer tx.Rollback() //nolint:errcheck
	usersStore := subjects.NewUsersStore(tx)

	params := subjects.UpsertUserParams{
		AnonymousID: body.AnonymousId,
		ExternalID:  body.ExternalId,
		Email:       body.Email,
		Phone:       body.Phone,
		Timezone:    body.Timezone,
		Locale:      body.Locale,
		Data:        data,
	}

	user, err := usersStore.IdentifyAndGetUser(ctx, projectID, params, true)
	if err != nil {
		logger.Error("failed to identify user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	msg := schemas.User{
		ProjectID:   projectID,
		ID:          user.ID,
		AnonymousID: user.AnonymousID,
		ExternalID:  user.ExternalID,
		Email:       user.Email,
		Phone:       user.Phone,
		Timezone:    user.Timezone,
		Locale:      user.Locale,
		Data:        data,
		Version:     user.Version,
	}

	err = srv.pubsub.Publish(ctx, schemas.UsersProcess(projectID), msg)
	if err != nil {
		logger.Error("failed to publish user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("user upserted", zap.String("user_id", user.ID.String()))
	json.Write(w, http.StatusOK, user.OAPI())
}

func (srv *UsersController) GetUser(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("users", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	logger.Info("getting user")

	user, err := srv.users.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user retrieved")
	json.Write(w, http.StatusOK, user.OAPI())
}

func (srv *UsersController) GetUserDevices(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("users", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.users.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	logger.Info("listing user devices")

	devices, err := srv.users.ListDevicesByUser(ctx, projectID, userID)
	if err != nil {
		logger.Error("failed to list user devices", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user devices listed", zap.Int("count", len(devices)))

	response := oapi.UserDeviceList{
		Results: devices.OAPI(),
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *UsersController) DeleteUserDevice(w http.ResponseWriter, r *http.Request, projectID, userID, deviceID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("users", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
		zap.String("device_id", deviceID.String()),
	)

	logger.Info("deleting user device")

	err = srv.users.DeleteDevice(ctx, projectID, deviceID)
	if err != nil {
		logger.Error("failed to delete device", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("device deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *UsersController) UpdateUser(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("users", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.users.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.UpdateUser{}
	err = json.Decode(r.Body, &body)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	logger.Info("updating user")

	var data map[string]any
	if body.Data != nil {
		err = json.Unmarshal(*body.Data, &data)
		if err != nil {
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("data must be a JSON object")))
			return
		}
	}

	tx, err := srv.usersDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	defer tx.Rollback() //nolint:errcheck
	users := subjects.NewUsersStore(tx)

	update := subjects.UserUpdate{
		Email:    body.Email,
		Phone:    body.Phone,
		Timezone: body.Timezone,
		Locale:   body.Locale,
		Data:     body.Data,
	}

	err = users.UpdateUser(ctx, userID, update)
	if err != nil {
		logger.Error("failed to update user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updatedUser, err := users.GetUser(ctx, projectID, userID)
	if err != nil {
		logger.Error("failed to get updated user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// Publish to pubsub for recomputation and schema extraction
	msg := schemas.User{
		ProjectID:   projectID,
		ID:          updatedUser.ID,
		AnonymousID: updatedUser.AnonymousID,
		ExternalID:  updatedUser.ExternalID,
		Email:       updatedUser.Email,
		Phone:       updatedUser.Phone,
		Timezone:    updatedUser.Timezone,
		Locale:      updatedUser.Locale,
		Data:        data,
		Version:     updatedUser.Version,
	}

	err = srv.pubsub.Publish(ctx, schemas.UsersProcess(projectID), msg)
	if err != nil {
		logger.Error("failed to publish user", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	logger.Info("user updated")
	json.Write(w, http.StatusOK, updatedUser.OAPI())
}

func (srv *UsersController) DeleteUser(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("users", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.users.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	logger.Info("deleting user")

	err = srv.users.DeleteUser(ctx, projectID, userID)
	if err != nil {
		logger.Error("failed to delete user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *UsersController) GetUserEvents(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID, params oapi.GetUserEventsParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("events", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.users.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	search := params.Search.ToString()
	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing user events", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	events, total, err := srv.users.ListUserEvents(ctx, projectID, userID, pagination, search)
	if err != nil {
		logger.Error("failed to list user events", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user events listed", zap.Int("total", total), zap.Int("count", len(events)))

	response := oapi.UserEventList{
		Results: events.OAPI(),
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *UsersController) GetUserSubscriptions(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID, params oapi.GetUserSubscriptionsParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("subscriptions", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.users.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing user subscriptions", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	subscriptions, total, err := srv.mgmt.GetUserSubscriptions(ctx, projectID, userID, pagination)
	if err != nil {
		logger.Error("failed to list user subscriptions", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user subscriptions listed", zap.Int("total", total), zap.Int("count", len(subscriptions)))

	response := oapi.UserSubscriptionList{
		Results: subscriptions.OAPI(),
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *UsersController) UpdateUserSubscriptions(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("subscriptions", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.users.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var subscriptions oapi.UpdateUserSubscriptions
	err = json.Decode(r.Body, &subscriptions)
	if err != nil {
		srv.logger.Error("failed to decode request body", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid request body")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	logger.Info("updating user subscriptions", zap.Int("count", len(subscriptions)))

	// Validate all subscriptions exist before starting transaction
	for _, sub := range subscriptions {
		_, err := srv.mgmt.GetSubscription(ctx, projectID, sub.SubscriptionId)
		if errors.Is(err, sql.ErrNoRows) {
			logger.Info("subscription not found", zap.String("subscription_id", sub.SubscriptionId.String()))
			oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("subscription not found")))
			return
		}
		if err != nil {
			logger.Error("failed to get subscription", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	for _, sub := range subscriptions {
		err = srv.mgmt.SetSubscriptionState(ctx, userID, sub.SubscriptionId, sub.State == "subscribed")
		if err != nil {
			logger.Error("failed to update subscription", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	user, err := srv.users.GetUser(ctx, projectID, userID)
	if err != nil {
		logger.Error("failed to get updated user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user subscriptions updated")
	json.Write(w, http.StatusOK, user.OAPI())
}

func (srv *UsersController) GetUserJourneys(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID, params oapi.GetUserJourneysParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("journeys", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.users.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing user journeys", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	journeys, total, err := srv.journey.ListUserJourneyEntrances(ctx, projectID, userID, pagination)
	if err != nil {
		logger.Error("failed to list user journeys", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user journeys listed", zap.Int("total", total), zap.Int("count", len(journeys)))

	response := oapi.UserJourneyList{
		Results: journeys.OAPI(),
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	}

	json.Write(w, http.StatusOK, response)
}

// userDirectColumns are well-known columns that exist directly on the users table
// and should be included in user schema suggestions without the .data prefix.
var userDirectColumns = []oapi.SchemaPath{
	{Path: ".email", Types: []string{"string"}},
	{Path: ".phone", Types: []string{"string"}},
	{Path: ".locale", Types: []string{"string"}},
	{Path: ".timezone", Types: []string{"string"}},
	{Path: ".external_id", Types: []string{"string"}},
	{Path: ".anonymous_id", Types: []string{"string"}},
	{Path: ".created_at", Types: []string{"date"}},
}

func (srv *UsersController) ListUserSchemas(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("users", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))
	logger.Info("listing user schemas")

	schemas, err := srv.users.ListUserSchemas(ctx, projectID)
	if err != nil {
		logger.Error("failed to list user schemas", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user schemas listed", zap.Int("count", len(schemas)))

	// Start with well-known user direct columns
	results := make([]oapi.SchemaPath, 0, len(userDirectColumns)+len(schemas))
	results = append(results, userDirectColumns...)

	// Add discovered user data properties with .data prefix so that the
	// query builder correctly targets the JSONB data column.
	for _, schema := range schemas {
		results = append(results, oapi.SchemaPath{
			Path:  ".data" + schema.Path,
			Types: []string(schema.Types),
		})
	}

	response := struct {
		Results []oapi.SchemaPath `json:"results"`
	}{
		Results: results,
	}

	json.Write(w, http.StatusOK, response)
}

func (srv *UsersController) ImportUsers(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("users", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))
	logger.Info("importing users from CSV")

	err = r.ParseMultipartForm(srv.maxUploadSize)
	if err != nil {
		logger.Error("failed to parse multipart form", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("file too large or invalid form data")))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		logger.Error("failed to get file from form", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("file is required")))
		return
	}
	defer file.Close()

	logger = logger.With(zap.String("filename", header.Filename), zap.Int64("size", header.Size))

	err = srv.processUserImport(ctx, logger, projectID, file)
	if err != nil {
		logger.Error("failed to import users", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("users imported successfully")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *UsersController) processUserImport(ctx context.Context, logger *zap.Logger, projectID uuid.UUID, file io.Reader) error {
	reader := csv.NewReader(file)
	headers, err := reader.Read()
	if err != nil {
		return problem.ErrBadRequest(problem.Describe("invalid CSV format"))
	}

	transformer, err := importer.NewUsers(headers)
	if err != nil {
		if errors.Is(err, importer.ErrMissingExternalID) {
			return problem.ErrBadRequest(problem.Describe("external_id column is required"))
		}
		return err
	}

	imported := 0
	tx, err := srv.usersDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		return err
	}

	defer tx.Rollback() //nolint:errcheck
	usersStore := subjects.NewState(tx, logger)

	for {
		record, err := reader.Read()
		if err == io.EOF {
			break
		}
		if err != nil {
			logger.Warn("failed to read CSV row", zap.Error(err))
			return err
		}

		user, err := transformer.MapRecord(record)
		if err != nil {
			logger.Warn("failed to map CSV row to user", zap.Error(err))
			return err
		}

		_, err = usersStore.UpsertUser(ctx, projectID, user)
		if err != nil {
			logger.Warn("failed to upsert user", zap.Error(err))
			return err
		}

		imported++
	}

	logger.Info("import completed", zap.Int("users_imported", imported))
	return tx.Commit()
}

func (srv *UsersController) GetUserOrganizations(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID, params oapi.GetUserOrganizationsParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("organizations", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.users.GetUser(ctx, projectID, userID)
	if errors.Is(err, sql.ErrNoRows) {
		srv.logger.Info("user not found")
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("user not found")))
		return
	}

	if err != nil {
		srv.logger.Error("failed to get user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	search := params.Search.ToString()
	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing user organizations")

	orgs, total, err := srv.users.ListUserOrganizations(ctx, projectID, userID, pagination, search)
	if err != nil {
		logger.Error("failed to list user organizations", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user organizations listed", zap.Int("total", total), zap.Int("count", len(orgs)))

	json.Write(w, http.StatusOK, oapi.OrganizationList{
		Results: subjects.Organizations(orgs).OAPI(),
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
	})
}
