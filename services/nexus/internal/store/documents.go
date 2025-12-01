package store

import (
	"context"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/services/nexus/oapi"
)

type Documents []Document

func (documents Documents) OAPI() []oapi.Document {
	result := make([]oapi.Document, len(documents))
	for index, document := range documents {
		result[index] = document.OAPI()
	}
	return result
}

type Document struct {
	ID          uuid.UUID `db:"id"`
	ProjectID   uuid.UUID `db:"project_id"`
	Name        string    `db:"name"`
	Filename    string    `db:"filename"`
	Key         string    `db:"key"`
	ContentType string    `db:"content_type"`
	SizeBytes   int64     `db:"size_bytes"`
	CreatedAt   time.Time `db:"created_at"`
	UpdatedAt   time.Time `db:"updated_at"`
}

func (document Document) OAPI() oapi.Document {
	return oapi.Document{
		Id:          document.ID,
		ProjectId:   document.ProjectID,
		Name:        document.Name,
		Filename:    document.Filename,
		Key:         document.Key,
		ContentType: document.ContentType,
		SizeBytes:   document.SizeBytes,
		CreatedAt:   document.CreatedAt,
		UpdatedAt:   document.UpdatedAt,
	}
}

func NewDocumentsStore(db DB) *DocumentsStore {
	return &DocumentsStore{
		db: db,
	}
}

type DocumentsStore struct {
	db DB
}

type CreateDocumentParams struct {
	Name        string
	Filename    string
	ContentType string
	SizeBytes   int64
}

func (s *DocumentsStore) CreateDocument(ctx context.Context, projectID uuid.UUID, params CreateDocumentParams) (uuid.UUID, error) {
	stmt := `
	INSERT INTO documents (project_id, name, filename, content_type, size_bytes)
	VALUES ($1, $2, $3, $4, $5)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt,
		projectID,
		params.Name,
		params.Filename,
		params.ContentType,
		params.SizeBytes,
	)
	if err != nil {
		return uuid.Nil, err
	}

	return id, nil
}

func (s *DocumentsStore) ListDocuments(ctx context.Context, projectID uuid.UUID, pagination Pagination) (Documents, int, error) {
	query := `
	SELECT id, project_id, name, filename, key, content_type, size_bytes, 
		created_at, updated_at,
		COUNT(*) OVER () AS total_count
	FROM documents
	WHERE project_id = $1
	AND deleted_at IS NULL
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	var results []struct {
		Document
		TotalCount int `db:"total_count"`
	}
	err := s.db.SelectContext(ctx, &results, query, projectID, pagination.Limit, pagination.Offset)
	if err != nil {
		return nil, 0, err
	}

	documents := make(Documents, len(results))
	total := 0

	for i, r := range results {
		documents[i] = r.Document
		if i == 0 {
			total = r.TotalCount
		}
	}

	return documents, total, nil
}

func (s *DocumentsStore) GetDocument(ctx context.Context, projectID, documentID uuid.UUID) (*Document, error) {
	query := `
	SELECT id, project_id, name, filename, key, content_type, size_bytes,
		created_at, updated_at
	FROM documents
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NULL`

	var document Document
	err := s.db.GetContext(ctx, &document, query, projectID, documentID)
	if err != nil {
		return nil, err
	}

	return &document, nil
}

func (s *DocumentsStore) UpdateDocumentKey(ctx context.Context, documentID uuid.UUID, key string) error {
	stmt := `
	UPDATE documents
	SET key = $1, updated_at = NOW()
	WHERE id = $2`

	_, err := s.db.ExecContext(ctx, stmt, key, documentID)
	return err
}

func (s *DocumentsStore) DeleteDocument(ctx context.Context, projectID, documentID uuid.UUID) error {
	stmt := `
	UPDATE documents
	SET deleted_at = NOW()
	WHERE project_id = $1
	AND id = $2
	AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, projectID, documentID)
	return err
}
