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
	"github.com/lunogram/platform/pkg/http/json"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/importer"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"go.uber.org/zap"
)

func NewListsController(logger *zap.Logger, db *sqlx.DB, maxUploadSize int64) *ListsController {
	return &ListsController{
		logger:        logger,
		db:            db,
		store:         store.NewStores(db),
		maxUploadSize: maxUploadSize,
	}
}

type ListsController struct {
	logger        *zap.Logger
	db            *sqlx.DB
	store         *store.Stores
	maxUploadSize int64
}

func (srv *ListsController) CreateList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	body := oapi.CreateListJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("name", body.Name))
	logger.Info("creating list")

	_, err = srv.store.ProjectsStore.GetProject(ctx, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("project not found", zap.Stringer("project_id", projectID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	var rule store.RuleData
	if body.Rule != nil {
		if err := json.Unmarshal(*body.Rule, &rule); err != nil {
			logger.Error("failed to unmarshal rule", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid rule format")))
			return
		}
	}

	// Static lists start in 'ready' state since they don't require rule configuration.
	// Dynamic lists start in 'draft' state to allow rule setup before activation.
	state := store.ListStateReady
	if body.Type == oapi.CreateListTypeDynamic {
		state = store.ListStateDraft
	}

	listID, err := srv.store.CreateList(ctx, store.List{
		ProjectID: projectID,
		Name:      body.Name,
		Type:      store.ListType(body.Type),
		State:     store.ListState(state),
		Rule:      store.JSONB[store.RuleData]{Data: rule},
	})
	if err != nil {
		logger.Error("failed to create list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list created", zap.Stringer("list_id", listID))
	list, err := srv.store.GetList(ctx, projectID, listID)
	if err != nil {
		logger.Error("failed to fetch created list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusCreated, list.OAPI())
}

func (srv *ListsController) ListLists(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListListsParams) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing lists")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	result, total, err := srv.store.ListLists(ctx, projectID, pagination)
	if err != nil {
		logger.Error("failed to list lists", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("listed lists", zap.Int("count", len(result)))
	json.Write(w, http.StatusOK, oapi.ListListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: result.OAPI(),
	})
}

func (srv *ListsController) GetList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("getting list")

	list, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list retrieved")
	json.Write(w, http.StatusOK, list.OAPI())
}

func (srv *ListsController) UpdateList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID) {
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("updating list")

	ctx := r.Context()
	body := oapi.UpdateListJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	update := store.ListUpdate{
		Name:      &body.Name,
		Published: body.Published,
	}

	if body.Rule != nil {
		var rule store.RuleData
		if err := json.Unmarshal(*body.Rule, &rule); err != nil {
			logger.Error("failed to unmarshal rule", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("invalid rule format")))
			return
		}
		update.Rule = &store.JSONB[store.RuleData]{Data: rule}
	}

	err = srv.store.UpdateList(ctx, projectID, listID, update)
	if err != nil {
		logger.Error("failed to update list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	list, err := srv.store.GetList(ctx, projectID, listID)
	if err != nil {
		logger.Error("failed to fetch updated list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list updated")
	json.Write(w, http.StatusOK, list.OAPI())
}

func (srv *ListsController) DeleteList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("deleting list")

	_, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.store.DeleteList(ctx, projectID, listID)
	if err != nil {
		logger.Error("failed to delete list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list deleted")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *ListsController) DuplicateList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("duplicating list")

	list, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	newName := "Copy of " + list.Name
	newListID, err := srv.store.DuplicateList(ctx, projectID, listID, newName)
	if err != nil {
		logger.Error("failed to duplicate list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list duplicated", zap.Stringer("new_list_id", newListID))
	duplicated, err := srv.store.GetList(ctx, projectID, newListID)
	if err != nil {
		logger.Error("failed to fetch duplicated list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	json.Write(w, http.StatusCreated, duplicated.OAPI())
}

func (srv *ListsController) ImportListUsers(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("importing users to list")

	list, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if list.Type != store.ListTypeStatic {
		logger.Error("list is not static", zap.String("type", string(list.Type)))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("only static lists support user imports")))
		return
	}

	if err := r.ParseMultipartForm(srv.maxUploadSize); err != nil {
		logger.Error("failed to parse multipart form", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("file too large or invalid form data")))
		return
	}

	file, header, err := r.FormFile("file")
	if err != nil {
		logger.Error("failed to get file from form", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("no file provided")))
		return
	}
	defer file.Close()

	logger = logger.With(zap.String("filename", header.Filename), zap.Int64("size", header.Size))

	err = srv.processUserImport(ctx, logger, projectID, listID, file)
	if err != nil {
		logger.Error("failed to import users", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("users imported successfully")
	w.WriteHeader(http.StatusNoContent)
}

func (srv *ListsController) processUserImport(ctx context.Context, logger *zap.Logger, projectID uuid.UUID, listID uuid.UUID, file io.Reader) error {
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
	stores := store.NewStores(tx)

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

		id, err := stores.UpsertUser(ctx, projectID, user)
		if err != nil {
			logger.Warn("failed to upsert user", zap.Error(err))
			return err
		}

		err = stores.AddUserToList(ctx, listID, id)
		if err != nil {
			logger.Warn("failed to add user to list", zap.Stringer("user_id", id), zap.Error(err))
			return err
		}

		imported++
	}

	logger.Info("import completed", zap.Int("users_added", imported))
	return tx.Commit()
}

func (srv *ListsController) GetListUsers(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID, params oapi.GetListUsersParams) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("getting list users")

	_, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Error("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	users, total, err := srv.store.ListListUsers(ctx, projectID, listID, pagination)
	if err != nil {
		logger.Error("failed to list list users", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list users retrieved", zap.Int("count", len(users)))
	json.Write(w, http.StatusOK, oapi.UserList{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: users.OAPI(),
	})
}
