package management

import (
	"context"
	"encoding/json"
	"errors"
	"strconv"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/store"
)

type SenderIdentities []SenderIdentity

func (identities SenderIdentities) OAPI() []oapi.SenderIdentity {
	result := make([]oapi.SenderIdentity, len(identities))
	for index, identity := range identities {
		result[index] = identity.OAPI()
	}
	return result
}

type SenderIdentity struct {
	ID         uuid.UUID       `db:"id"`
	ProjectID  uuid.UUID       `db:"project_id"`
	ProviderID uuid.UUID       `db:"provider_id"`
	Channel    string          `db:"channel"`
	Traits     json.RawMessage `db:"traits"`
	CreatedAt  time.Time       `db:"created_at"`
	UpdatedAt  time.Time       `db:"updated_at"`
}

// TraitsMap returns Traits as a map, or an empty map if nil/empty.
func (identity SenderIdentity) TraitsMap() map[string]any {
	if len(identity.Traits) == 0 {
		return map[string]any{}
	}
	var m map[string]any
	if err := json.Unmarshal(identity.Traits, &m); err != nil {
		return map[string]any{}
	}
	return m
}

// Address extracts the address from the traits JSONB.
func (identity SenderIdentity) Address() string {
	m := identity.TraitsMap()
	addr, _ := m["address"].(string)
	return addr
}

func (identity SenderIdentity) OAPI() oapi.SenderIdentity {
	return oapi.SenderIdentity{
		Id:         identity.ID,
		ProjectId:  identity.ProjectID,
		ProviderId: identity.ProviderID,
		Channel:    oapi.SenderIdentityChannel(identity.Channel),
		Traits:     identity.TraitsMap(),
		CreatedAt:  identity.CreatedAt,
		UpdatedAt:  identity.UpdatedAt,
	}
}

func NewSenderIdentitiesStore(db store.DB) *SenderIdentitiesStore {
	return &SenderIdentitiesStore{
		db: db,
	}
}

type SenderIdentitiesStore struct {
	db store.DB
}

func (s *SenderIdentitiesStore) CreateSenderIdentity(ctx context.Context, identity SenderIdentity) (uuid.UUID, error) {
	traits := identity.Traits
	if len(traits) == 0 {
		traits = json.RawMessage(`{}`)
	}

	stmt := `
	INSERT INTO sender_identities (project_id, provider_id, channel, traits)
	VALUES ($1, $2, $3, $4)
	RETURNING id`

	var id uuid.UUID
	err := s.db.GetContext(ctx, &id, stmt, identity.ProjectID, identity.ProviderID, identity.Channel, traits)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			return uuid.Nil, problem.ErrConflict(problem.Describe("a sender identity with this provider, channel, and address already exists"))
		}
		return uuid.Nil, err
	}

	return id, nil
}

func (s *SenderIdentitiesStore) ListSenderIdentities(ctx context.Context, projectID uuid.UUID, providerID *uuid.UUID, channel *string, pagination store.Pagination) (SenderIdentities, int, error) {
	query := `
	SELECT id, project_id, provider_id, channel, traits, created_at, updated_at,
		COUNT(*) OVER () AS total_count
	FROM sender_identities
	WHERE project_id = $1`

	args := []any{projectID}
	argIdx := 2

	if providerID != nil {
		query += ` AND provider_id = $` + strconv.Itoa(argIdx)
		args = append(args, *providerID)
		argIdx++
	}

	if channel != nil {
		query += ` AND channel = $` + strconv.Itoa(argIdx)
		args = append(args, *channel)
		argIdx++
	}

	query += `
	ORDER BY created_at DESC
	LIMIT $` + strconv.Itoa(argIdx) + ` OFFSET $` + strconv.Itoa(argIdx+1)
	args = append(args, pagination.Limit, pagination.Offset)

	var results []struct {
		SenderIdentity
		TotalCount int `db:"total_count"`
	}
	err := s.db.SelectContext(ctx, &results, query, args...)
	if err != nil {
		return nil, 0, err
	}

	identities := make(SenderIdentities, len(results))
	total := 0

	for i, r := range results {
		identities[i] = r.SenderIdentity
		if i == 0 {
			total = r.TotalCount
		}
	}

	return identities, total, nil
}

func (s *SenderIdentitiesStore) GetSenderIdentity(ctx context.Context, projectID, identityID uuid.UUID) (*SenderIdentity, error) {
	query := `
	SELECT id, project_id, provider_id, channel, traits, created_at, updated_at
	FROM sender_identities
	WHERE project_id = $1
	AND id = $2`

	var identity SenderIdentity
	err := s.db.GetContext(ctx, &identity, query, projectID, identityID)
	if err != nil {
		return nil, err
	}

	return &identity, nil
}

func (s *SenderIdentitiesStore) DeleteSenderIdentity(ctx context.Context, projectID, identityID uuid.UUID) error {
	query := `
	DELETE FROM sender_identities
	WHERE project_id = $1
	AND id = $2`

	_, err := s.db.ExecContext(ctx, query, projectID, identityID)
	return err
}
