package management

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
)

// PolicyType identifies how an access policy authenticates a client. The API
// key is the original type; trusted-issuer (JWKS / PEM) and short-term session
// policies extend the model.
type PolicyType string

const (
	PolicyTypeAPIKey        PolicyType = "api_key"
	PolicyTypeTrustedIssuer PolicyType = "trusted_issuer"
	PolicyTypeSession       PolicyType = "session"
)

// Scope values recorded on an access policy's secret. Public keys are safe to
// expose in client-side code; secret keys are backend-only.
const (
	ScopePublic = "public"
	ScopeSecret = "secret"
)

// Grant is one (resource, verb) entry in a policy's custom permission set. It
// mirrors access.Grant and is persisted in the access_policies.grants JSONB
// column. An empty set means the policy uses its role preset instead.
type Grant struct {
	Resource string `json:"resource"`
	Verb     string `json:"verb"`
}

// IssuerConfig holds the trusted-issuer verification settings for a
// PolicyTypeTrustedIssuer policy. Exactly one of JWKSURL or PublicCert is set.
type IssuerConfig struct {
	JWKSURL      string `json:"jwks_url,omitempty"`
	PublicCert   string `json:"public_cert,omitempty"`
	Issuer       string `json:"iss,omitempty"`
	Audience     string `json:"aud,omitempty"`
	SubjectClaim string `json:"subject_claim,omitempty"`
}

// SessionConfig holds the short-term session-signing settings for a
// PolicyTypeSession policy.
type SessionConfig struct {
	TTL  time.Duration `json:"ttl,omitempty"`
	Role string        `json:"role,omitempty"`
}

// AccessPolicy is a project-scoped configuration of how a client authenticates
// to the API. Type-specific fields (Secret, IssuerConfig, SessionConfig) are
// populated according to Type.
type AccessPolicy struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Type        PolicyType
	Name        string
	Description *string
	Scope       *string
	Role        string

	// Secret is the raw key value for api_key policies. It is nil for policy
	// types that carry no Lunogram-held secret (e.g. trusted_issuer).
	Secret *string

	// Grants is the custom permission set. Nil/empty means the policy resolves
	// through its Role preset.
	Grants        []Grant
	IssuerConfig  *IssuerConfig
	SessionConfig *SessionConfig

	CreatedAt time.Time
	UpdatedAt time.Time
}

// accessPolicyRow mirrors the access_policies table for scanning. JSONB columns
// are read as raw bytes and decoded into the typed [AccessPolicy] fields.
type accessPolicyRow struct {
	ID            uuid.UUID `db:"id"`
	ProjectID     uuid.UUID `db:"project_id"`
	Type          string    `db:"type"`
	Value         *string   `db:"value"`
	Scope         *string   `db:"scope"`
	Name          string    `db:"name"`
	Description   *string   `db:"description"`
	Role          string    `db:"role"`
	Grants        []byte    `db:"grants"`
	IssuerConfig  []byte    `db:"issuer_config"`
	SessionConfig []byte    `db:"session_config"`
	CreatedAt     time.Time `db:"created_at"`
	UpdatedAt     time.Time `db:"updated_at"`
}

func (r accessPolicyRow) toPolicy() (*AccessPolicy, error) {
	p := AccessPolicy{
		ID:          r.ID,
		ProjectID:   r.ProjectID,
		Type:        PolicyType(r.Type),
		Name:        r.Name,
		Description: r.Description,
		Scope:       r.Scope,
		Role:        r.Role,
		Secret:      r.Value,
		CreatedAt:   r.CreatedAt,
		UpdatedAt:   r.UpdatedAt,
	}
	if err := decodeJSONB(r.Grants, &p.Grants); err != nil {
		return nil, fmt.Errorf("management: decode grants for policy %s: %w", r.ID, err)
	}
	if len(r.IssuerConfig) > 0 {
		if err := decodeJSONB(r.IssuerConfig, &p.IssuerConfig); err != nil {
			return nil, fmt.Errorf("management: decode issuer_config for policy %s: %w", r.ID, err)
		}
	}
	if len(r.SessionConfig) > 0 {
		if err := decodeJSONB(r.SessionConfig, &p.SessionConfig); err != nil {
			return nil, fmt.Errorf("management: decode session_config for policy %s: %w", r.ID, err)
		}
	}
	return &p, nil
}

// decodeJSONB unmarshals a JSONB column into dst, treating SQL NULL and empty
// payloads as a no-op so dst keeps its zero value.
func decodeJSONB(raw []byte, dst any) error {
	if len(raw) == 0 {
		return nil
	}
	return json.Unmarshal(raw, dst)
}

// encodeJSONB marshals v for storage in a JSONB column, returning nil (SQL NULL)
// for empty values so absent config is stored as NULL rather than "null".
func encodeJSONB(v any) ([]byte, error) {
	switch t := v.(type) {
	case nil:
		return nil, nil
	case []Grant:
		if len(t) == 0 {
			return nil, nil
		}
	case *IssuerConfig:
		if t == nil {
			return nil, nil
		}
	case *SessionConfig:
		if t == nil {
			return nil, nil
		}
	}
	return json.Marshal(v)
}

// CreateAccessPolicyInput carries the fields needed to create an access policy.
// Type-specific fields are optional and validated by the caller.
type CreateAccessPolicyInput struct {
	Type          PolicyType
	Name          string
	Description   *string
	Scope         *string
	Role          string
	Secret        *string
	Grants        []Grant
	IssuerConfig  *IssuerConfig
	SessionConfig *SessionConfig
}

func NewAccessPoliciesStore(db store.DB) *AccessPoliciesStore {
	return &AccessPoliciesStore{db: db}
}

type AccessPoliciesStore struct {
	db store.DB
}

func (s *AccessPoliciesStore) CreateAccessPolicy(ctx context.Context, projectID uuid.UUID, in CreateAccessPolicyInput) (*AccessPolicy, error) {
	// API-key policies carry a Lunogram-issued secret; generate one when the
	// caller did not supply it.
	if in.Type == PolicyTypeAPIKey && in.Secret == nil {
		value, err := generateKeyValue()
		if err != nil {
			return nil, err
		}
		in.Secret = &value
	}

	grants, err := encodeJSONB(in.Grants)
	if err != nil {
		return nil, err
	}
	issuer, err := encodeJSONB(in.IssuerConfig)
	if err != nil {
		return nil, err
	}
	session, err := encodeJSONB(in.SessionConfig)
	if err != nil {
		return nil, err
	}

	const stmt = `
	INSERT INTO access_policies
		(project_id, type, value, scope, name, description, role, grants, issuer_config, session_config)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10)
	RETURNING id, project_id, type, value, scope, name, description, role, grants, issuer_config, session_config, created_at, updated_at`

	var row accessPolicyRow
	err = s.db.GetContext(ctx, &row, stmt,
		projectID, string(in.Type), in.Secret, in.Scope, in.Name, in.Description, in.Role, grants, issuer, session)
	if err != nil {
		return nil, err
	}
	return row.toPolicy()
}

func (s *AccessPoliciesStore) GetAccessPolicy(ctx context.Context, projectID, policyID uuid.UUID) (*AccessPolicy, error) {
	const stmt = `
	SELECT id, project_id, type, value, scope, name, description, role, grants, issuer_config, session_config, created_at, updated_at
	FROM access_policies
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`

	var row accessPolicyRow
	if err := s.db.GetContext(ctx, &row, stmt, policyID, projectID); err != nil {
		return nil, err
	}
	return row.toPolicy()
}

func (s *AccessPoliciesStore) ListAccessPolicies(ctx context.Context, projectID uuid.UUID, pagination store.Pagination) ([]AccessPolicy, int, error) {
	const stmt = `
	SELECT id, project_id, type, value, scope, name, description, role, grants, issuer_config, session_config,
	       created_at, updated_at, COUNT(*) OVER () AS total_count
	FROM access_policies
	WHERE project_id = $1 AND deleted_at IS NULL
	ORDER BY created_at DESC
	LIMIT $2 OFFSET $3`

	type listRow struct {
		accessPolicyRow
		TotalCount int `db:"total_count"`
	}

	var rows []listRow
	if err := s.db.SelectContext(ctx, &rows, stmt, projectID, pagination.Limit, pagination.Offset); err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return []AccessPolicy{}, 0, nil
	}

	policies := make([]AccessPolicy, len(rows))
	for i, r := range rows {
		p, err := r.toPolicy()
		if err != nil {
			return nil, 0, err
		}
		policies[i] = *p
	}
	return policies, rows[0].TotalCount, nil
}

// UpdateAccessPolicyInput carries the mutable fields of an access policy. Nil
// fields are left unchanged; Grants is only rewritten when non-nil.
type UpdateAccessPolicyInput struct {
	Name        *string
	Description *string
	Role        *string
	Grants      []Grant
}

func (s *AccessPoliciesStore) UpdateAccessPolicy(ctx context.Context, projectID, policyID uuid.UUID, in UpdateAccessPolicyInput) error {
	var grants []byte
	if in.Grants != nil {
		encoded, err := encodeJSONB(in.Grants)
		if err != nil {
			return err
		}
		grants = encoded
	}

	const stmt = `
	UPDATE access_policies
	SET name        = COALESCE($1, name),
	    description = COALESCE($2, description),
	    role        = COALESCE($3, role),
	    grants      = CASE WHEN $4::boolean THEN $5::jsonb ELSE grants END
	WHERE id = $6 AND project_id = $7 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt,
		in.Name, in.Description, in.Role, in.Grants != nil, grants, policyID, projectID)
	return err
}

func (s *AccessPoliciesStore) DeleteAccessPolicy(ctx context.Context, projectID, policyID uuid.UUID) error {
	const stmt = `
	UPDATE access_policies
	SET deleted_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, policyID, projectID)
	return err
}
