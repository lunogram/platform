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
	"github.com/lunogram/platform/services/nexus/internal/claim"
	"github.com/lunogram/platform/services/nexus/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/services/nexus/internal/http/json"
	"github.com/lunogram/platform/services/nexus/internal/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/importer"
	"github.com/lunogram/platform/services/nexus/internal/pubsub"
	"github.com/lunogram/platform/services/nexus/internal/pubsub/schemas"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"go.uber.org/zap"
)

func NewUsersController(logger *zap.Logger, pub pubsub.Publisher, db *sqlx.DB, maxUploadSize int64) *UsersController {
	return &UsersController{
		logger:        logger,
		db:            db,
		store:         store.NewState(db),
		pubsub:        pub,
		maxUploadSize: maxUploadSize,
	}
}

type UsersController struct {
	logger        *zap.Logger
	db            *sqlx.DB
	pubsub        pubsub.Publisher
	store         *store.State
	maxUploadSize int64
}

func (srv *UsersController) ListUsers(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListUsersParams) {
	ctx := r.Context()
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))

	search := params.Search.ToString()
	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	logger.Info("listing users", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	users, total, err := srv.store.ListUsers(ctx, projectID, pagination, search)
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
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	body := oapi.IdentifyUser{}
	err := json.Decode(r.Body, &body)
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

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	defer tx.Rollback() //nolint:errcheck
	users := store.NewUsersStore(tx)

	params := store.UpsertUserParams{
		AnonymousID: body.AnonymousId,
		ExternalID:  body.ExternalId,
		Email:       body.Email,
		Phone:       body.Phone,
		Timezone:    body.Timezone,
		Locale:      body.Locale,
		Data:        data,
	}

	user, err := users.IdentifyAndGetUser(ctx, projectID, params, true)
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
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	logger := srv.logger.With(
		zap.String("project_id", projectID.String()),
		zap.String("user_id", userID.String()),
	)

	logger.Info("getting user")

	user, err := srv.store.GetUser(ctx, projectID, userID)
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

func (srv *UsersController) UpdateUser(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	_, err := srv.store.GetUser(ctx, projectID, userID)
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

	update := store.UserUpdate{
		Email:    body.Email,
		Phone:    body.Phone,
		Timezone: body.Timezone,
		Locale:   body.Locale,
		Data:     body.Data,
	}

	err = srv.store.UpdateUser(ctx, userID, update)
	if err != nil {
		logger.Error("failed to update user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updatedUser, err := srv.store.GetUser(ctx, projectID, userID)
	if err != nil {
		logger.Error("failed to get updated user", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user updated")
	json.Write(w, http.StatusOK, updatedUser.OAPI())
}

func (srv *UsersController) DeleteUser(w http.ResponseWriter, r *http.Request, projectID, userID uuid.UUID) {
	ctx := r.Context()
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	_, err := srv.store.GetUser(ctx, projectID, userID)
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

	err = srv.store.DeleteUser(ctx, projectID, userID)
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
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	_, err := srv.store.GetUser(ctx, projectID, userID)
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

	logger.Info("listing user events", zap.Int("limit", pagination.Limit), zap.Int("offset", pagination.Offset))

	events, total, err := srv.store.ListUserEvents(ctx, projectID, userID, pagination)
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
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	_, err := srv.store.GetUser(ctx, projectID, userID)
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

	subscriptions, total, err := srv.store.GetUserSubscriptions(ctx, projectID, userID, pagination)
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
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	_, err := srv.store.GetUser(ctx, projectID, userID)
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
		_, err := srv.store.GetSubscription(ctx, projectID, sub.SubscriptionId)
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

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}
	defer tx.Rollback()

	for _, sub := range subscriptions {
		err = srv.store.SetSubscriptionState(ctx, tx, userID, sub.SubscriptionId, sub.State == "subscribed")
		if err != nil {
			logger.Error("failed to update subscription", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	if err = tx.Commit(); err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrInternal())
		return
	}

	user, err := srv.store.GetUser(ctx, projectID, userID)
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
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	_, err := srv.store.GetUser(ctx, projectID, userID)
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

	journeys, total, err := srv.store.ListUserJourneyEntrances(ctx, projectID, userID, pagination)
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

func (srv *UsersController) ListUserSchemas(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))
	logger.Info("listing user schemas")

	schemas, err := srv.store.ListUserSchemas(ctx, projectID)
	if err != nil {
		logger.Error("failed to list user schemas", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("user schemas listed", zap.Int("count", len(schemas)))

	results := make([]oapi.SchemaPath, len(schemas))
	for i, schema := range schemas {
		results[i] = oapi.SchemaPath{
			Path:  schema.Path,
			Types: []string(schema.Types),
		}
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
	_, ok := claim.FromContext(ctx)
	if !ok {
		srv.logger.Error("session not found in context")
		oapi.WriteProblem(w, problem.ErrUnauthorized(problem.Describe("unauthorized")))
		return
	}

	logger := srv.logger.With(zap.String("project_id", projectID.String()))
	logger.Info("importing users from CSV")

	err := r.ParseMultipartForm(srv.maxUploadSize)
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
	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		return err
	}

	defer tx.Rollback() //nolint:errcheck
	stores := store.NewState(tx)

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

		_, err = stores.UpsertUser(ctx, projectID, user)
		if err != nil {
			logger.Warn("failed to upsert user", zap.Error(err))
			return err
		}

		imported++
	}

	logger.Info("import completed", zap.Int("users_imported", imported))
	return tx.Commit()
}
