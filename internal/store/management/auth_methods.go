package management

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/ptr"
	iredis "github.com/lunogram/platform/internal/redis"
	"github.com/lunogram/platform/internal/store"
)

// MethodType identifies how an auth method authenticates a client.
type MethodType string

const (
	MethodTypeAPIKey        MethodType = "api_key"
	MethodTypeTrustedIssuer MethodType = "trusted_issuer"
	MethodTypeSession       MethodType = "session"
)

// SubjectScope is the data boundary an auth method acts within: SubjectScopeAll
// acts across every subject's records (the only valid value for api_key), while
// SubjectScopeOwn confines a verified end user to their own records.
type SubjectScope string

const (
	SubjectScopeAll SubjectScope = "all"
	SubjectScopeOwn SubjectScope = "own"
)

// Grant is one (resource, verb) entry in an auth method's custom permission set.
// The json tags match the aggregate built in the read queries.
type Grant struct {
	Resource string `json:"resource"`
	Verb     string `json:"verb"`
}

// TrustedIssuer holds the external-JWT validation config for a trusted_issuer
// method. Exactly one of JWKSURL or PublicCert is set.
type TrustedIssuer struct {
	JWKSURL      string
	PublicCert   string
	Issuer       string
	Audience     string
	SubjectClaim string
}

// Session holds the config for a session method.
type Session struct {
	TTLSeconds int
}

// AuthMethod is a configured way for a client to authenticate to the API. The
// id is the RBAC subject; Role + Grants are the authorization it confers.
// Type-specific fields are populated according to Type.
type AuthMethod struct {
	ID          uuid.UUID
	ProjectID   uuid.UUID
	Type        MethodType
	Name        string
	Description *string
	Role        string
	Grants      []Grant

	// SubjectScope is the data boundary. See [SubjectScope].
	SubjectScope SubjectScope

	// Secret holds the full plaintext for an api_key method; it is set only by
	// CreateAuthMethod (shown once) and never populated by reads.
	Secret *string

	TrustedIssuer *TrustedIssuer // MethodTypeTrustedIssuer
	Session       *Session       // MethodTypeSession

	CreatedAt time.Time
	UpdatedAt time.Time
}

// NewAuthMethodsStore builds the auth-method write store. The caches may be nil;
// when set, writes invalidate the cached auth lookups they affect so the read
// path (AuthStore) does not serve a stale credential.
func NewAuthMethodsStore(db store.DB, apiKeys *iredis.Cache[APIKey], issuers *iredis.Cache[TrustedIssuerAuthMethod]) *AuthMethodsStore {
	return &AuthMethodsStore{db: db, apiKeys: apiKeys, issuers: issuers}
}

type AuthMethodsStore struct {
	db      store.DB
	apiKeys *iredis.Cache[APIKey]
	issuers *iredis.Cache[TrustedIssuerAuthMethod]
}

// invalidateCaches drops the cached auth lookups for a method after it changes,
// so a role/scope edit or a delete is observed immediately rather than at TTL.
// It is keyed on the credential the read path uses (api-key secret hash, trusted
// issuer iss), looked up from the child rows. Best-effort: a cache miss to clear
// is harmless and errors only mean the entry lapses at its TTL.
func (s *AuthMethodsStore) invalidateCaches(ctx context.Context, methodID uuid.UUID) {
	if s.apiKeys == nil && s.issuers == nil {
		return
	}
	var row struct {
		SecretHash *string `db:"secret_hash"`
		Issuer     *string `db:"issuer"`
	}
	const q = `
	SELECT k.secret_hash, t.issuer
	FROM auth_methods m
	LEFT JOIN auth_method_api_keys k ON k.auth_method_id = m.id
	LEFT JOIN auth_method_trusted_issuers t ON t.auth_method_id = m.id
	WHERE m.id = $1`
	if err := s.db.GetContext(ctx, &row, q, methodID); err != nil {
		return
	}
	if row.SecretHash != nil {
		_ = s.apiKeys.Invalidate(ctx, *row.SecretHash)
	}
	if row.Issuer != nil {
		_ = s.issuers.Invalidate(ctx, *row.Issuer)
	}
}

// txBeginner is the subset of *sqlx.DB used to start a write transaction. Writes
// require a real connection pool (not an existing transaction); auth-method
// writes are never issued through a transaction-backed State.
type txBeginner interface {
	BeginTxx(context.Context, *sql.TxOptions) (*sqlx.Tx, error)
}

func (s *AuthMethodsStore) begin(ctx context.Context) (*sqlx.Tx, error) {
	db, ok := s.db.(txBeginner)
	if !ok {
		return nil, errors.New("management: auth methods store requires a *sqlx.DB for writes")
	}
	return db.BeginTxx(ctx, nil)
}

// CreateAuthMethodInput carries the fields to create an auth method. The
// type-specific block (TrustedIssuer, Session) must match Type.
type CreateAuthMethodInput struct {
	Type          MethodType
	Name          string
	Description   *string
	Role          string
	SubjectScope  SubjectScope
	Grants        []Grant
	TrustedIssuer *TrustedIssuer
	Session       *Session
}

// CreateAuthMethod inserts an auth method, its type-specific credential row, and
// its grants in a single transaction. For api_key methods the generated secret
// is returned once on the result's Secret field.
func (s *AuthMethodsStore) CreateAuthMethod(ctx context.Context, projectID uuid.UUID, in CreateAuthMethodInput) (*AuthMethod, error) {
	tx, err := s.begin(ctx)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	id := uuid.New()
	subjectScope := in.SubjectScope
	if subjectScope == "" {
		subjectScope = SubjectScopeAll
	}
	if _, err := tx.ExecContext(ctx,
		`INSERT INTO auth_methods (id, project_id, type, name, description, role, subject_scope) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
		id, projectID, string(in.Type), in.Name, in.Description, in.Role, string(subjectScope)); err != nil {
		return nil, err
	}

	var secret *string
	switch in.Type {
	case MethodTypeAPIKey:
		secret, err = insertAPIKey(ctx, tx, id)
	case MethodTypeTrustedIssuer:
		err = insertTrustedIssuer(ctx, tx, id, in.TrustedIssuer)
	case MethodTypeSession:
		err = insertSession(ctx, tx, id, in.Session)
	default:
		err = fmt.Errorf("management: unknown auth method type %q", in.Type)
	}
	if err != nil {
		return nil, err
	}

	if err := insertGrants(ctx, tx, id, in.Grants); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	method, err := s.GetAuthMethod(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	method.Secret = secret // shown to the caller exactly once
	return method, nil
}

// insertAPIKey mints a fresh sk_ secret for an api_key method, persists its hash
// and display prefix, and returns the plaintext to show the caller once.
func insertAPIKey(ctx context.Context, q sqlx.ExecerContext, methodID uuid.UUID) (*string, error) {
	plaintext, prefix, hash, err := newSecret()
	if err != nil {
		return nil, err
	}
	if _, err := q.ExecContext(ctx,
		`INSERT INTO auth_method_api_keys (auth_method_id, secret_hash, secret_prefix) VALUES ($1, $2, $3)`,
		methodID, hash, prefix); err != nil {
		return nil, err
	}
	return &plaintext, nil
}

// insertTrustedIssuer persists a trusted_issuer method's validation config. A
// trusted issuer is resolved at auth time by `iss` alone, with no project
// context, so the active issuer must be globally unique; otherwise a token could
// authenticate against the wrong project. (Soft-deleted methods keep their child
// row, so this is enforced here rather than via a DB constraint that would block
// re-adding a deleted issuer.)
func insertTrustedIssuer(ctx context.Context, q sqlx.ExtContext, methodID uuid.UUID, in *TrustedIssuer) error {
	ti := ptr.FromOr(in, TrustedIssuer{})
	subjectClaim := ti.SubjectClaim
	if subjectClaim == "" {
		subjectClaim = "sub"
	}

	var existing uuid.UUID
	err := sqlx.GetContext(ctx, q, &existing,
		`SELECT m.id FROM auth_method_trusted_issuers t
		 JOIN auth_methods m ON m.id = t.auth_method_id
		 WHERE t.issuer = $1 AND m.deleted_at IS NULL LIMIT 1`, ti.Issuer)
	if err == nil {
		return problem.ErrConflict(problem.Describe("a trusted issuer is already registered for this iss"))
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return err
	}

	_, err = q.ExecContext(ctx,
		`INSERT INTO auth_method_trusted_issuers (auth_method_id, jwks_url, public_cert, issuer, audience, subject_claim)
		 VALUES ($1, $2, $3, $4, $5, $6)`,
		methodID,
		sql.NullString{String: ti.JWKSURL, Valid: ti.JWKSURL != ""},
		sql.NullString{String: ti.PublicCert, Valid: ti.PublicCert != ""},
		ti.Issuer,
		sql.NullString{String: ti.Audience, Valid: ti.Audience != ""},
		subjectClaim)
	return err
}

// insertSession persists a session method's config.
func insertSession(ctx context.Context, q sqlx.ExecerContext, methodID uuid.UUID, in *Session) error {
	ttl := 900
	if in != nil && in.TTLSeconds > 0 {
		ttl = in.TTLSeconds
	}
	_, err := q.ExecContext(ctx,
		`INSERT INTO auth_method_sessions (auth_method_id, ttl_seconds) VALUES ($1, $2)`,
		methodID, ttl)
	return err
}

// insertGrants writes a method's permission set. It runs on a transaction (or
// any sqlx executor) so it composes with create/update.
func insertGrants(ctx context.Context, q sqlx.ExecerContext, methodID uuid.UUID, grants []Grant) error {
	for _, g := range grants {
		if _, err := q.ExecContext(ctx,
			`INSERT INTO auth_method_grants (auth_method_id, resource, verb) VALUES ($1, $2, $3)`,
			methodID, g.Resource, g.Verb); err != nil {
			return err
		}
	}
	return nil
}

// row is the flattened projection of an auth method joined to its optional
// credential tables, with its grants aggregated as JSON so a method and its
// permission set load in a single round-trip. Only the columns for the row's
// type are non-NULL.
type row struct {
	ID           uuid.UUID `db:"id"`
	ProjectID    uuid.UUID `db:"project_id"`
	Type         string    `db:"type"`
	Name         string    `db:"name"`
	Description  *string   `db:"description"`
	Role         string    `db:"role"`
	SubjectScope string    `db:"subject_scope"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`

	JWKSURL      *string `db:"jwks_url"`
	PublicCert   *string `db:"public_cert"`
	Issuer       *string `db:"issuer"`
	Audience     *string `db:"audience"`
	SubjectClaim *string `db:"subject_claim"`

	TTLSeconds *int `db:"ttl_seconds"`

	// Grants is a JSON array of {resource, verb} aggregated in the query.
	Grants []byte `db:"grants"`

	// TotalCount is the window count returned by List; it is absent (zero) for
	// single-row reads.
	TotalCount int `db:"total_count"`
}

func (r row) toMethod() (*AuthMethod, error) {
	m := &AuthMethod{
		ID:           r.ID,
		ProjectID:    r.ProjectID,
		Type:         MethodType(r.Type),
		Name:         r.Name,
		Description:  r.Description,
		Role:         r.Role,
		SubjectScope: SubjectScope(r.SubjectScope),
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
	}
	if len(r.Grants) > 0 {
		if err := json.Unmarshal(r.Grants, &m.Grants); err != nil {
			return nil, err
		}
	}
	switch m.Type {
	case MethodTypeTrustedIssuer:
		m.TrustedIssuer = &TrustedIssuer{
			JWKSURL:      ptr.From(r.JWKSURL),
			PublicCert:   ptr.From(r.PublicCert),
			Issuer:       ptr.From(r.Issuer),
			Audience:     ptr.From(r.Audience),
			SubjectClaim: ptr.From(r.SubjectClaim),
		}
	case MethodTypeSession:
		if r.TTLSeconds != nil {
			m.Session = &Session{TTLSeconds: *r.TTLSeconds}
		}
	}
	return m, nil
}

// The read queries are written out in full (rather than composed from shared
// fragments) so each is a single, greppable static string. Grants are folded in
// as a JSON aggregate so a method loads in one round-trip.
const getAuthMethodQuery = `
SELECT m.id, m.project_id, m.type, m.name, m.description, m.role, m.subject_scope, m.created_at, m.updated_at,
       t.jwks_url, t.public_cert, t.issuer, t.audience, t.subject_claim,
       s.ttl_seconds,
       COALESCE((
           SELECT json_agg(json_build_object('resource', g.resource, 'verb', g.verb) ORDER BY g.resource, g.verb)
           FROM auth_method_grants g
           WHERE g.auth_method_id = m.id
       ), '[]') AS grants
FROM auth_methods m
LEFT JOIN auth_method_trusted_issuers t ON t.auth_method_id = m.id
LEFT JOIN auth_method_sessions s ON s.auth_method_id = m.id
WHERE m.id = $1 AND m.project_id = $2 AND m.deleted_at IS NULL`

const listAuthMethodsQuery = `
SELECT m.id, m.project_id, m.type, m.name, m.description, m.role, m.subject_scope, m.created_at, m.updated_at,
       t.jwks_url, t.public_cert, t.issuer, t.audience, t.subject_claim,
       s.ttl_seconds,
       COALESCE((
           SELECT json_agg(json_build_object('resource', g.resource, 'verb', g.verb) ORDER BY g.resource, g.verb)
           FROM auth_method_grants g
           WHERE g.auth_method_id = m.id
       ), '[]') AS grants,
       COUNT(*) OVER () AS total_count
FROM auth_methods m
LEFT JOIN auth_method_trusted_issuers t ON t.auth_method_id = m.id
LEFT JOIN auth_method_sessions s ON s.auth_method_id = m.id
WHERE m.project_id = $1 AND m.deleted_at IS NULL
ORDER BY m.created_at DESC
LIMIT $2 OFFSET $3`

func (s *AuthMethodsStore) GetAuthMethod(ctx context.Context, projectID, methodID uuid.UUID) (*AuthMethod, error) {
	var r row
	if err := s.db.GetContext(ctx, &r, getAuthMethodQuery, methodID, projectID); err != nil {
		return nil, err
	}
	return r.toMethod()
}

func (s *AuthMethodsStore) ListAuthMethods(ctx context.Context, projectID uuid.UUID, pagination store.Pagination) ([]AuthMethod, int, error) {
	var rows []row
	if err := s.db.SelectContext(ctx, &rows, listAuthMethodsQuery, projectID, pagination.Limit, pagination.Offset); err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return []AuthMethod{}, 0, nil
	}

	methods := make([]AuthMethod, len(rows))
	for i := range rows {
		m, err := rows[i].toMethod()
		if err != nil {
			return nil, 0, err
		}
		methods[i] = *m
	}
	return methods, rows[0].TotalCount, nil
}

// UpdateAuthMethodInput carries the mutable fields. Nil fields are unchanged;
// Grants is rewritten only when non-nil (replacing the whole set).
type UpdateAuthMethodInput struct {
	Name         *string
	Description  *string
	Role         *string
	SubjectScope *SubjectScope
	Grants       []Grant
}

func (s *AuthMethodsStore) UpdateAuthMethod(ctx context.Context, projectID, methodID uuid.UUID, in UpdateAuthMethodInput) error {
	tx, err := s.begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	var subjectScope *string
	if in.SubjectScope != nil {
		subjectScope = ptr.To(string(*in.SubjectScope))
	}
	if _, err := tx.ExecContext(ctx,
		`UPDATE auth_methods
		 SET name = COALESCE($1, name), description = COALESCE($2, description),
		     role = COALESCE($3, role), subject_scope = COALESCE($4, subject_scope)
		 WHERE id = $5 AND project_id = $6 AND deleted_at IS NULL`,
		in.Name, in.Description, in.Role, subjectScope, methodID, projectID); err != nil {
		return err
	}

	// Replace the whole grant set when provided.
	if in.Grants != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM auth_method_grants WHERE auth_method_id = $1`, methodID); err != nil {
			return err
		}
		if err := insertGrants(ctx, tx, methodID, in.Grants); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}
	s.invalidateCaches(ctx, methodID)
	return nil
}

func (s *AuthMethodsStore) DeleteAuthMethod(ctx context.Context, projectID, methodID uuid.UUID) error {
	const stmt = `
	UPDATE auth_methods
	SET deleted_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`

	if _, err := s.db.ExecContext(ctx, stmt, methodID, projectID); err != nil {
		return err
	}
	s.invalidateCaches(ctx, methodID)
	return nil
}
