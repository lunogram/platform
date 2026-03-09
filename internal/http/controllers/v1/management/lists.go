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
	"github.com/lunogram/platform/internal/pubsub/consumer"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rules"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/lunogram/platform/internal/store/subjects"
	"go.uber.org/zap"
)

func NewListsController(logger *zap.Logger, usersDB *sqlx.DB, projects *management.ProjectsStore, pub pubsub.Publisher, maxUploadSize int64, engine *rbac.Engine) *ListsController {
	return &ListsController{
		logger:        logger,
		usersDB:       usersDB,
		store:         subjects.NewState(usersDB),
		projects:      projects,
		maxUploadSize: maxUploadSize,
		pub:           pub,
		engine:        engine,
	}
}

type ListsController struct {
	logger        *zap.Logger
	usersDB       *sqlx.DB
	store         *subjects.State
	projects      *management.ProjectsStore
	maxUploadSize int64
	pub           pubsub.Publisher
	engine        *rbac.Engine
}

func (srv *ListsController) CreateList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("lists", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	body := oapi.CreateListJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("name", body.Name))
	logger.Info("creating list")

	_, err = srv.projects.GetProject(ctx, projectID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("project not found", zap.Stringer("project_id", projectID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get project", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.usersDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}
	defer tx.Rollback() //nolint:errcheck

	lists := subjects.NewListsStore(tx)
	rules := subjects.NewRulesStore(tx)
	events := subjects.NewEventsStore(tx)

	// Create the list (no rule_id — rules live in list_versions now)
	listID, err := lists.CreateList(ctx, subjects.List{
		ProjectID: projectID,
		Name:      body.Name,
		Type:      subjects.ListType(body.Type),
	})
	if err != nil {
		logger.Error("failed to create list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// If a rule is provided, create it and attach to a draft version
	if body.Type == oapi.CreateListTypeDynamic && body.Rule != nil {
		ruleID, err := srv.createRule(ctx, logger, rules, events, projectID, *body.Rule)
		if err != nil {
			oapi.WriteProblem(w, err)
			return
		}

		// Create a draft version with the rule
		versionID, err := lists.CreateVersion(ctx, listID, &ruleID)
		if err != nil {
			logger.Error("failed to create draft version", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		err = lists.SetListVersionID(ctx, listID, versionID)
		if err != nil {
			logger.Error("failed to set version_id", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	list, err := lists.GetList(ctx, projectID, listID)
	if err != nil {
		logger.Error("failed to fetch created list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list created", zap.Stringer("list_id", listID))
	json.Write(w, http.StatusCreated, list.OAPI())
}

func (srv *ListsController) ListLists(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListListsParams) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("lists", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing lists")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	search := params.Search.ToString()

	result, total, err := srv.store.ListLists(ctx, projectID, pagination, search)
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
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("lists", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("getting list")

	list, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("list not found", zap.Stringer("list_id", listID))
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
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("lists", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("updating list")
	body := oapi.UpdateListJSONRequestBody{}
	err := json.Decode(r.Body, &body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	_, err = srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	tx, err := srv.usersDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	defer tx.Rollback() //nolint:errcheck

	lists := subjects.NewListsStore(tx)
	rules := subjects.NewRulesStore(tx)
	events := subjects.NewEventsStore(tx)

	// Update list name
	err = lists.UpdateList(ctx, projectID, listID, subjects.ListUpdate{
		Name: &body.Name,
	})
	if err != nil {
		logger.Error("failed to update list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// If a rule is provided, save it to the draft version
	if body.Rule != nil {
		draft, err := lists.EnsureDraftVersion(ctx, projectID, listID)
		if err != nil {
			logger.Error("failed to ensure draft version", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		// Create or update the draft version's rule
		ruleID, err := rules.CreateOrUpdateRule(ctx, projectID, draft.RuleID, *body.Rule)
		if err != nil {
			logger.Error("failed to create or update rule", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		// Upsert events and set event dependencies
		err = srv.upsertRuleEvents(ctx, logger, rules, events, projectID, ruleID, *body.Rule)
		if err != nil {
			oapi.WriteProblem(w, err)
			return
		}

		// Update the version's rule_id if it changed (e.g. new rule was created)
		if draft.RuleID == nil || *draft.RuleID != ruleID {
			err = lists.UpdateVersionRuleID(ctx, draft.ID, ruleID)
			if err != nil {
				logger.Error("failed to update version rule_id", zap.Error(err))
				oapi.WriteProblem(w, err)
				return
			}
		}
	}

	// If published flag is set, publish the draft version
	if body.Published != nil && *body.Published {
		draft, err := lists.GetDraftVersion(ctx, listID)
		if err != nil {
			logger.Error("failed to get draft version for publish", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("no draft version to publish")))
			return
		}

		err = lists.PublishVersion(ctx, listID, draft.ID)
		if err != nil {
			logger.Error("failed to publish version", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		// Recompute membership using the now-published rule
		if draft.RuleID != nil {
			rule, err := rules.GetRule(ctx, projectID, *draft.RuleID)
			if err != nil {
				logger.Error("failed to get published rule for recompute", zap.Error(err))
				oapi.WriteProblem(w, err)
				return
			}

			recomputed, err := lists.RecomputeList(ctx, projectID, listID, rule.Rule.Data)
			if err != nil {
				logger.Error("failed to recompute list", zap.Error(err))
				oapi.WriteProblem(w, err)
				return
			}

			err = consumer.PublishListRecomputeEvents(ctx, logger, srv.pub, projectID, listID, recomputed)
			if err != nil {
				logger.Error("failed to publish list recompute events", zap.Error(err))
				oapi.WriteProblem(w, err)
				return
			}
		}
	}

	list, err := lists.GetList(ctx, projectID, listID)
	if err != nil {
		logger.Error("failed to fetch updated list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = tx.Commit()
	if err != nil {
		logger.Error("failed to commit transaction", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("list updated")
	json.Write(w, http.StatusOK, list.OAPI())
}

// createRule creates a new rule and sets up event dependencies. Returns the rule ID.
func (srv *ListsController) createRule(ctx context.Context, logger *zap.Logger, rulesStore *subjects.RulesStore, events *subjects.EventsStore, projectID uuid.UUID, ruleset rules.RuleSet) (uuid.UUID, error) {
	ruleID, err := rulesStore.CreateOrUpdateRule(ctx, projectID, nil, ruleset)
	if err != nil {
		logger.Error("failed to create rule", zap.Error(err))
		return uuid.Nil, err
	}

	err = srv.upsertRuleEvents(ctx, logger, rulesStore, events, projectID, ruleID, ruleset)
	if err != nil {
		return uuid.Nil, err
	}

	return ruleID, nil
}

// upsertRuleEvents upserts events referenced by the ruleset and sets rule-event dependencies.
func (srv *ListsController) upsertRuleEvents(ctx context.Context, logger *zap.Logger, rulesStore *subjects.RulesStore, events *subjects.EventsStore, projectID, ruleID uuid.UUID, ruleset rules.RuleSet) error {
	for _, event := range ruleset.UserEvents() {
		_, err := events.UpsertEvent(ctx, projectID, event, subjects.SubjectTypeUser)
		if err != nil {
			logger.Error("failed to upsert event", zap.String("event", event), zap.Error(err))
			return err
		}
	}

	for _, event := range ruleset.OrganizationEvents() {
		_, err := events.UpsertEvent(ctx, projectID, event, subjects.SubjectTypeOrganization)
		if err != nil {
			logger.Error("failed to upsert organization event", zap.String("event", event), zap.Error(err))
			return err
		}
	}

	eventDeps := make([]subjects.EventDependency, 0, len(ruleset.UserEvents())+len(ruleset.OrganizationEvents()))
	for _, event := range ruleset.UserEvents() {
		eventDeps = append(eventDeps, subjects.EventDependency{
			Name:        event,
			SubjectType: subjects.SubjectTypeUser,
		})
	}

	for _, event := range ruleset.OrganizationEvents() {
		eventDeps = append(eventDeps, subjects.EventDependency{
			Name:        event,
			SubjectType: subjects.SubjectTypeOrganization,
		})
	}

	err := rulesStore.SetRuleEventDependencies(ctx, projectID, ruleID, eventDeps)
	if err != nil {
		logger.Error("failed to set rule event dependencies", zap.Error(err))
		return err
	}

	return nil
}

func (srv *ListsController) DeleteList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("lists", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("deleting list")

	_, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("list not found", zap.Stringer("list_id", listID))
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
	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("lists", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("duplicating list")

	list, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("list not found", zap.Stringer("list_id", listID))
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
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.ProjectResourceScope("lists", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("importing users to list")

	list, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if list.Type != subjects.ListTypeStatic {
		logger.Error("list is not static", zap.String("type", string(list.Type)))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("only static lists support user imports")))
		return
	}

	err = r.ParseMultipartForm(srv.maxUploadSize)
	if err != nil {
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
	tx, err := srv.usersDB.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		return err
	}

	defer tx.Rollback() //nolint:errcheck
	stores := subjects.NewState(tx)

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
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("lists", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("getting list users")

	_, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("list not found", zap.Stringer("list_id", listID))
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

	search := params.Search.ToString()

	users, total, err := srv.store.SelectListUsers(ctx, projectID, listID, pagination, search)
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

func (srv *ListsController) PreviewListUsers(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID, params oapi.PreviewListUsersParams) {
	ctx := r.Context()
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("lists", projectID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("list_id", listID))
	logger.Info("previewing list users")

	list, err := srv.store.GetList(ctx, projectID, listID)
	if errors.Is(err, sql.ErrNoRows) {
		logger.Info("list not found", zap.Stringer("list_id", listID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("list not found")))
		return
	}

	if err != nil {
		logger.Error("failed to get list", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if list.Type != subjects.ListTypeDynamic {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("preview is only available for dynamic lists")))
		return
	}

	if list.DraftRule == nil {
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("list has no draft rule")))
		return
	}

	limit := 25
	if params.Limit != nil {
		limit = params.Limit.ToInt()
	}

	users, total, err := srv.store.PreviewListUsers(ctx, projectID, list.DraftRule.Data, limit)
	if err != nil {
		logger.Error("failed to preview list users", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("preview list users retrieved", zap.Int("count", len(users)), zap.Int("total", total))
	json.Write(w, http.StatusOK, oapi.UserList{
		Total:   total,
		Limit:   limit,
		Offset:  0,
		Results: users.OAPI(),
	})
}

func (srv *ListsController) RecountList(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, listID uuid.UUID) {
	oapi.WriteProblem(w, problem.ErrUnimplemented())
}
