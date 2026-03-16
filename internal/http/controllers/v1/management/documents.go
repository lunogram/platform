package v1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"mime"
	"mime/multipart"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/storage"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewDocumentsController(logger *zap.Logger, db *sqlx.DB, storage storage.Storage, maxUploadSize int64, urlResolver *storage.URLResolver, engine *rbac.Engine) *DocumentsController {
	return &DocumentsController{
		logger:        logger,
		db:            db,
		storage:       storage,
		maxUploadSize: maxUploadSize,
		urlResolver:   urlResolver,
		engine:        engine,
	}
}

type DocumentsController struct {
	logger        *zap.Logger
	db            *sqlx.DB
	storage       storage.Storage
	maxUploadSize int64
	urlResolver   *storage.URLResolver
	engine        *rbac.Engine
}

func (srv *DocumentsController) uploadDocument(ctx context.Context, logger *zap.Logger, projectID uuid.UUID, header *multipart.FileHeader) (uuid.UUID, error) {
	if header.Size > srv.maxUploadSize {
		logger.Error("file too large", zap.String("filename", header.Filename), zap.Int64("size", header.Size))
		return uuid.Nil, problem.ErrBadRequest(problem.Describe(fmt.Sprintf("file %s exceeds maximum size", header.Filename)))
	}

	contentType := header.Header.Get("Content-Type")

	file, err := header.Open()
	if err != nil {
		logger.Error("failed to open uploaded file", zap.Error(err))
		return uuid.Nil, err
	}
	defer file.Close()

	documentID := uuid.New()
	exts, err := mime.ExtensionsByType(contentType)
	if err != nil || len(exts) == 0 {
		logger.Error("unsupported content type", zap.String("content_type", contentType))
		return uuid.Nil, problem.ErrBadRequest(problem.Describe(fmt.Sprintf("unsupported content type: %s", contentType)))
	}

	key := fmt.Sprintf("%s%s", documentID, exts[0])

	tx, err := srv.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error("failed to begin transaction", zap.Error(err))
		return uuid.Nil, problem.ErrInternal()
	}
	defer tx.Rollback() //nolint:errcheck

	document := management.CreateDocumentParams{
		Name:        header.Filename,
		Key:         key,
		Filename:    header.Filename,
		ContentType: contentType,
		SizeBytes:   header.Size,
	}

	documents := management.NewDocumentsStore(tx)
	err = documents.CreateDocument(ctx, projectID, documentID, document)
	if err != nil {
		logger.Error("failed to create document record", zap.Error(err))
		return uuid.Nil, err
	}

	err = srv.storage.Write(ctx, key, file)
	if err != nil {
		logger.Error("failed to write file to storage", zap.Error(err))
		return uuid.Nil, problem.ErrInternal()
	}

	logger.Info("document uploaded", zap.Stringer("document_id", documentID), zap.String("filename", header.Filename), zap.String("key", key))
	return documentID, tx.Commit()
}

func (srv *DocumentsController) UploadDocuments(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Create, rbac.ProjectResourceScope("documents", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("uploading documents")

	err = r.ParseMultipartForm(srv.maxUploadSize)
	if err != nil {
		logger.Error("failed to parse multipart form", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("file too large or invalid form data")))
		return
	}

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		logger.Error("no files provided")
		oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe("no files provided")))
		return
	}

	documents := make([]uuid.UUID, len(files))

	for index, header := range files {
		documents[index], err = srv.uploadDocument(ctx, logger, projectID, header)
		if err != nil {
			oapi.WriteProblem(w, err)
			return
		}
	}

	json.Write(w, http.StatusCreated, map[string][]uuid.UUID{
		"documents": documents,
	})
}

func (srv *DocumentsController) ListDocuments(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListDocumentsParams) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("documents", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing documents")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	documentsStore := management.NewDocumentsStore(srv.db)
	result, total, err := documentsStore.ListDocuments(ctx, projectID, pagination)
	if err != nil {
		logger.Error("failed to list documents", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("listed documents", zap.Int("count", len(result)))
	json.Write(w, http.StatusOK, oapi.DocumentListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: result.OAPIWithURLs(srv.urlResolver.URL),
	})
}

func (srv *DocumentsController) GetDocument(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, documentID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("documents", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("document_id", documentID))
	logger.Info("getting document file")

	documentsStore := management.NewDocumentsStore(srv.db)
	document, err := documentsStore.GetDocument(ctx, projectID, documentID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("document not found", zap.Stringer("document_id", documentID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("document not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch document", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	file, err := srv.storage.Read(ctx, document.Key)
	if err != nil {
		logger.Error("failed to read document file from storage", zap.Error(err))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("document file not found")))
		return
	}
	defer file.Close()

	w.Header().Set("Content-Type", document.ContentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("inline; filename=%s", document.Filename))

	if _, err := io.Copy(w, file); err != nil {
		logger.Error("failed to write document to response", zap.Error(err))
		return
	}

	logger.Info("document file retrieved")
}

func (srv *DocumentsController) GetDocumentMetadata(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, documentID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Read, rbac.ProjectResourceScope("documents", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("document_id", documentID))
	logger.Info("getting document metadata")

	documentsStore := management.NewDocumentsStore(srv.db)
	document, err := documentsStore.GetDocument(ctx, projectID, documentID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("document not found", zap.Stringer("document_id", documentID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("document not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch document", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("document metadata retrieved")
	json.Write(w, http.StatusOK, document.OAPIWithURL(srv.urlResolver.URL(document.Key)))
}

func (srv *DocumentsController) DeleteDocument(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, documentID uuid.UUID) {
	ctx := r.Context()
	err := srv.engine.Allowed(ctx, rbac.Delete, rbac.ProjectResourceScope("documents", projectID))
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("document_id", documentID))
	logger.Info("deleting document")

	documentsStore := management.NewDocumentsStore(srv.db)
	document, err := documentsStore.GetDocument(ctx, projectID, documentID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Info("document not found", zap.Stringer("document_id", documentID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("document not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch document", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = documentsStore.DeleteDocument(ctx, projectID, documentID)
	if err != nil {
		logger.Error("failed to delete document", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	err = srv.storage.Delete(ctx, document.Key)
	if err != nil {
		logger.Warn("failed to delete document file from storage", zap.Error(err))
	}

	logger.Info("document deleted")
	w.WriteHeader(http.StatusNoContent)
}
