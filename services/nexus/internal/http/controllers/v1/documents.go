package v1

import (
	"errors"
	"fmt"
	"io"
	"mime"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/pkg/http/json"
	"github.com/lunogram/platform/pkg/http/problem"
	"github.com/lunogram/platform/services/nexus/internal/storage"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"go.uber.org/zap"
)

// buildStorageKey constructs a storage key (URL path) from the document ID and content type.
// Returns a key like "<uuid>.<extension>" or an error if the content type is unsupported.
func buildStorageKey(id uuid.UUID, contentType string) (string, error) {
	exts, err := mime.ExtensionsByType(contentType)
	if err != nil || len(exts) == 0 {
		return "", fmt.Errorf("unsupported content type: %s", contentType)
	}
	return fmt.Sprintf("%s%s", id, exts[0]), nil
}

func NewDocumentsController(logger *zap.Logger, db *sqlx.DB, storage storage.Storage, maxUploadSize int64) *DocumentsController {
	return &DocumentsController{
		logger:        logger,
		db:            db,
		storage:       storage,
		maxUploadSize: maxUploadSize,
	}
}

type DocumentsController struct {
	logger        *zap.Logger
	db            *sqlx.DB
	storage       storage.Storage
	maxUploadSize int64
}

func (srv *DocumentsController) UploadDocuments(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("uploading documents")

	if err := r.ParseMultipartForm(srv.maxUploadSize); err != nil {
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

	// Process each file in its own transaction
	for index, header := range files {
		if header.Size > srv.maxUploadSize {
			logger.Error("file too large", zap.String("filename", header.Filename), zap.Int64("size", header.Size))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe(fmt.Sprintf("file %s exceeds maximum size", header.Filename))))
			return
		}

		contentType := header.Header.Get("Content-Type")

		file, err := header.Open()
		if err != nil {
			logger.Error("failed to open uploaded file", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
		defer file.Close()

		// Start transaction for this file
		tx, err := srv.db.BeginTxx(ctx, nil)
		if err != nil {
			logger.Error("failed to begin transaction", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}

		// Create document record first to get database-generated ID
		documentsStore := store.NewDocumentsStore(tx)
		documentID, err := documentsStore.CreateDocument(ctx, projectID, store.CreateDocumentParams{
			Name:        header.Filename,
			Filename:    header.Filename,
			ContentType: contentType,
			SizeBytes:   header.Size,
		})
		if err != nil {
			tx.Rollback() //nolint:errcheck
			logger.Error("failed to create document record", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}

		// Build storage key with extension from content type
		key, err := buildStorageKey(documentID, contentType)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			logger.Error("failed to build storage key", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrBadRequest(problem.Describe(err.Error())))
			return
		}

		// Store file to storage backend
		err = srv.storage.Write(ctx, key, file)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			logger.Error("failed to write file to storage", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}

		// Update document record with storage key
		err = documentsStore.UpdateDocumentKey(ctx, documentID, key)
		if err != nil {
			tx.Rollback() //nolint:errcheck
			// Try to delete the file from storage since DB update failed
			if delErr := srv.storage.Delete(ctx, key); delErr != nil {
				logger.Warn("failed to delete orphaned file from storage", zap.Error(delErr))
			}
			logger.Error("failed to update document key", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}

		// Commit transaction for this file
		if err := tx.Commit(); err != nil {
			logger.Error("failed to commit transaction", zap.Error(err))
			oapi.WriteProblem(w, problem.ErrInternal())
			return
		}

		documents[index] = documentID
		logger.Info("document uploaded", zap.Stringer("document_id", documentID), zap.String("filename", header.Filename), zap.String("key", key))
	}

	json.Write(w, http.StatusCreated, map[string][]uuid.UUID{
		"documents": documents,
	})
}

func (srv *DocumentsController) ListDocuments(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListDocumentsParams) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID))
	logger.Info("listing documents")

	pagination := store.Pagination{
		Limit:  params.Limit.ToInt(),
		Offset: params.Offset.ToInt(),
	}

	documentsStore := store.NewDocumentsStore(srv.db)
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
		Results: result.OAPI(),
	})
}

func (srv *DocumentsController) GetDocument(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, documentID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("document_id", documentID))
	logger.Info("getting document file")

	documentsStore := store.NewDocumentsStore(srv.db)
	document, err := documentsStore.GetDocument(ctx, projectID, documentID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Error("document not found", zap.Stringer("document_id", documentID))
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
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("document_id", documentID))
	logger.Info("getting document metadata")

	documentsStore := store.NewDocumentsStore(srv.db)
	document, err := documentsStore.GetDocument(ctx, projectID, documentID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Error("document not found", zap.Stringer("document_id", documentID))
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("document not found")))
		return
	}

	if err != nil {
		logger.Error("failed to fetch document", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("document metadata retrieved")
	json.Write(w, http.StatusOK, document.OAPI())
}

func (srv *DocumentsController) DeleteDocument(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, documentID uuid.UUID) {
	ctx := r.Context()
	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.Stringer("document_id", documentID))
	logger.Info("deleting document")

	documentsStore := store.NewDocumentsStore(srv.db)
	document, err := documentsStore.GetDocument(ctx, projectID, documentID)
	if errors.Is(err, store.ErrNoRows) {
		logger.Error("document not found", zap.Stringer("document_id", documentID))
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

	// Use the stored key directly
	key := document.Key
	if key == "" {
		// Fallback: build key from content type if not stored (for backwards compatibility)
		key, err = buildStorageKey(documentID, document.ContentType)
		if err != nil {
			logger.Warn("failed to build storage key for deletion", zap.Error(err))
			// Continue with database deletion even if storage key can't be built
		}
	}

	if key != "" {
		if err := srv.storage.Delete(ctx, key); err != nil {
			logger.Warn("failed to delete document file from storage", zap.Error(err))
		}
	}

	logger.Info("document deleted")
	w.WriteHeader(http.StatusNoContent)
}
