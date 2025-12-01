package v1

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http/httptest"
	"net/textproto"
	"os"
	"testing"

	"github.com/cloudproud/graceful"
	"github.com/google/uuid"
	"github.com/lunogram/platform/pkg/container"
	"github.com/lunogram/platform/services/nexus/internal/config"
	"github.com/lunogram/platform/services/nexus/internal/storage"
	"github.com/lunogram/platform/services/nexus/internal/store"
	"github.com/lunogram/platform/services/nexus/oapi"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func TestDocumentUpload(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	uploadDir := t.TempDir()
	cfg := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
		Storage: storage.Config{
			Type:          "local",
			MaxUploadSize: 10485760,
			Local: storage.LocalConfig{
				Directory: uploadDir,
			},
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, cfg.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	storageBackend, err := storage.New(cfg.Storage)
	require.NoError(t, err)

	documents := NewDocumentsController(logger, db, storageBackend, cfg.Storage.MaxUploadSize)

	type test struct {
		filename    string
		contentType string
		content     []byte
		code        int
	}

	tests := map[string]test{
		"jpeg-image": {
			filename:    "test.jpg",
			contentType: "image/jpeg",
			content:     []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 0x4A, 0x46, 0x49, 0x46},
			code:        201,
		},
		"png-image": {
			filename:    "test.png",
			contentType: "image/png",
			content:     []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A},
			code:        201,
		},
		"pdf-document": {
			filename:    "document.pdf",
			contentType: "application/pdf",
			content:     []byte("%PDF-1.4"),
			code:        201,
		},
		"video-file": {
			filename:    "video.mp4",
			contentType: "video/mp4",
			content:     []byte("fake mp4 content"),
			code:        201,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			body := &bytes.Buffer{}
			writer := multipart.NewWriter(body)

			h := textproto.MIMEHeader{}
			h.Set("Content-Disposition", fmt.Sprintf(`form-data; name="files"; filename="%s"`, test.filename))
			h.Set("Content-Type", test.contentType)

			part, err := writer.CreatePart(h)
			require.NoError(t, err)

			_, err = part.Write(test.content)
			require.NoError(t, err)

			err = writer.Close()
			require.NoError(t, err)

			res := httptest.NewRecorder()
			req := httptest.NewRequest("POST", "/v1/documents", body)
			req.Header.Set("Content-Type", writer.FormDataContentType())

			documents.UploadDocuments(res, req, projectID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 201 {
				var result map[string][]uuid.UUID
				err := json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Len(t, result["documents"], 1)
				require.NotEqual(t, uuid.Nil, result["documents"][0])
			}
		})
	}
}

func TestListDocuments(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	uploadDir := t.TempDir()
	cfg := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
		Storage: storage.Config{
			Type:          "local",
			MaxUploadSize: 10485760,
			Local: storage.LocalConfig{
				Directory: uploadDir,
			},
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, cfg.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	documentsStore := store.NewDocumentsStore(db)

	for i := 0; i < 3; i++ {
		documentID := uuid.New()
		err := documentsStore.CreateDocument(ctx, projectID, documentID, store.CreateDocumentParams{
			Name:        fmt.Sprintf("test-document-%d.jpg", i),
			Key:         fmt.Sprintf("%s.jpg", documentID),
			Filename:    fmt.Sprintf("test-%d.jpg", i),
			ContentType: "image/jpeg",
			SizeBytes:   1024,
		})
		require.NoError(t, err)
	}

	storageBackend, err := storage.New(cfg.Storage)
	require.NoError(t, err)

	documents := NewDocumentsController(logger, db, storageBackend, cfg.Storage.MaxUploadSize)

	type test struct {
		params   oapi.ListDocumentsParams
		expected int
	}

	tests := map[string]test{
		"list-all": {
			params: oapi.ListDocumentsParams{
				Limit:  ptr(oapi.PaginationLimit(10)),
				Offset: ptr(oapi.PaginationOffset(0)),
			},
			expected: 3,
		},
		"with-pagination": {
			params: oapi.ListDocumentsParams{
				Limit:  ptr(oapi.PaginationLimit(2)),
				Offset: ptr(oapi.PaginationOffset(0)),
			},
			expected: 2,
		},
		"offset-beyond-total": {
			params: oapi.ListDocumentsParams{
				Limit:  ptr(oapi.PaginationLimit(10)),
				Offset: ptr(oapi.PaginationOffset(100)),
			},
			expected: 0,
		},
		"limit-exceeds-available": {
			params: oapi.ListDocumentsParams{
				Limit:  ptr(oapi.PaginationLimit(100)),
				Offset: ptr(oapi.PaginationOffset(0)),
			},
			expected: 3,
		},
		"offset-at-boundary": {
			params: oapi.ListDocumentsParams{
				Limit:  ptr(oapi.PaginationLimit(10)),
				Offset: ptr(oapi.PaginationOffset(2)),
			},
			expected: 1,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", "/v1/documents", nil)
			documents.ListDocuments(res, req, projectID, test.params)

			require.Equal(t, 200, res.Code, res.Body.String())

			var result oapi.DocumentListResponse
			err := json.Unmarshal(res.Body.Bytes(), &result)
			require.NoError(t, err)
			require.Equal(t, test.expected, len(result.Results))
			require.GreaterOrEqual(t, result.Total, test.expected)
		})
	}
}

func TestGetDocumentMetadata(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	uploadDir := t.TempDir()
	cfg := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
		Storage: storage.Config{
			Type:          "local",
			MaxUploadSize: 10485760,
			Local: storage.LocalConfig{
				Directory: uploadDir,
			},
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, cfg.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	documentsStore := store.NewDocumentsStore(db)
	documentID := uuid.New()
	err = documentsStore.CreateDocument(ctx, projectID, documentID, store.CreateDocumentParams{
		Name:        "test-document.jpg",
		Key:         fmt.Sprintf("%s.jpg", documentID),
		Filename:    "test.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024,
	})
	require.NoError(t, err)

	storageBackend, err := storage.New(cfg.Storage)
	require.NoError(t, err)

	documents := NewDocumentsController(logger, db, storageBackend, cfg.Storage.MaxUploadSize)

	type test struct {
		documentID uuid.UUID
		code       int
	}

	tests := map[string]test{
		"existing-document": {
			documentID: documentID,
			code:       200,
		},
		"non-existing-document": {
			documentID: uuid.New(),
			code:       404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", fmt.Sprintf("/v1/documents/%s/metadata", test.documentID), nil)
			documents.GetDocumentMetadata(res, req, projectID, test.documentID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				var result oapi.Document
				err := json.Unmarshal(res.Body.Bytes(), &result)
				require.NoError(t, err)
				require.Equal(t, test.documentID, result.Id)
				require.Equal(t, "test-document.jpg", result.Name)
				require.Equal(t, "test.jpg", result.Filename)
				require.Equal(t, "image/jpeg", result.ContentType)
				require.Equal(t, int64(1024), result.SizeBytes)
			}
		})
	}
}

func TestGetDocument(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	uploadDir := t.TempDir()
	cfg := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
		Storage: storage.Config{
			Type:          "local",
			MaxUploadSize: 10485760,
			Local: storage.LocalConfig{
				Directory: uploadDir,
			},
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, cfg.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	storageBackend, err := storage.New(cfg.Storage)
	require.NoError(t, err)

	documents := NewDocumentsController(logger, db, storageBackend, cfg.Storage.MaxUploadSize)

	testContent := []byte("fake document content")

	documentsStore := store.NewDocumentsStore(db)
	documentID := uuid.New()
	key := fmt.Sprintf("%s.jpg", documentID)
	err = documentsStore.CreateDocument(ctx, projectID, documentID, store.CreateDocumentParams{
		Name:        "test-document.jpg",
		Key:         key,
		Filename:    "test.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   int64(len(testContent)),
	})
	require.NoError(t, err)

	err = os.WriteFile(fmt.Sprintf("%s/%s", uploadDir, key), testContent, 0644)
	require.NoError(t, err)

	type test struct {
		documentID uuid.UUID
		code       int
	}

	tests := map[string]test{
		"existing-document": {
			documentID: documentID,
			code:       200,
		},
		"non-existing-document": {
			documentID: uuid.New(),
			code:       404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("GET", fmt.Sprintf("/v1/documents/%s", test.documentID), nil)
			documents.GetDocument(res, req, projectID, test.documentID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 200 {
				require.Equal(t, "image/jpeg", res.Header().Get("Content-Type"))
				content, err := io.ReadAll(res.Body)
				require.NoError(t, err)
				require.Equal(t, testContent, content)
			}
		})
	}
}

func TestDeleteDocument(t *testing.T) {
	logger := zaptest.NewLogger(t)
	ctx := graceful.NewContext(t.Context())
	uploadDir := t.TempDir()
	cfg := config.Service{
		Store: store.Config{
			URI: container.RunPostgreSQL(t),
		},
		Storage: storage.Config{
			Type:          "local",
			MaxUploadSize: 10485760,
			Local: storage.LocalConfig{
				Directory: uploadDir,
			},
		},
	}

	err := store.Migrate(cfg.Store)
	require.NoError(t, err)

	db, err := store.Connect(ctx, cfg.Store)
	require.NoError(t, err)

	projects := store.NewProjectsStore(db)
	projectID, err := projects.CreateProject(ctx, DefaultProject)
	require.NoError(t, err)

	storageBackend, err := storage.New(cfg.Storage)
	require.NoError(t, err)

	documents := NewDocumentsController(logger, db, storageBackend, cfg.Storage.MaxUploadSize)

	documentsStore := store.NewDocumentsStore(db)
	documentID := uuid.New()
	key := fmt.Sprintf("%s.jpg", documentID)
	err = documentsStore.CreateDocument(ctx, projectID, documentID, store.CreateDocumentParams{
		Name:        "to-delete.jpg",
		Key:         key,
		Filename:    "delete.jpg",
		ContentType: "image/jpeg",
		SizeBytes:   1024,
	})
	require.NoError(t, err)

	testFile := fmt.Sprintf("%s/%s", uploadDir, key)
	err = os.WriteFile(testFile, []byte("test"), 0644)
	require.NoError(t, err)

	type test struct {
		documentID uuid.UUID
		code       int
	}

	tests := map[string]test{
		"successful-delete": {
			documentID: documentID,
			code:       204,
		},
		"non-existing-document": {
			documentID: uuid.New(),
			code:       404,
		},
	}

	for name, test := range tests {
		t.Run(name, func(t *testing.T) {
			res := httptest.NewRecorder()
			req := httptest.NewRequest("DELETE", fmt.Sprintf("/v1/documents/%s", test.documentID), nil)
			documents.DeleteDocument(res, req, projectID, test.documentID)

			require.Equal(t, test.code, res.Code, res.Body.String())

			if test.code == 204 {
				_, err := os.Stat(testFile)
				require.True(t, os.IsNotExist(err))
			}
		})
	}
}
