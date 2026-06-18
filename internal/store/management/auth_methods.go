package management

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lib/pq"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store"
)

// MethodType identifies how an auth method authenticates a client.
type MethodType string

const (
	MethodTypeAPIKey        MethodType = "api_key"
	MethodTypeTrustedIssuer MethodType = "trusted_issuer"
	MethodTypeSession       MethodType = "session"
)

// Subject-scope values recorded on an auth method. They define the data
// boundary: SubjectScopeAll acts across every subject's records (the only valid
// value for api_key), while SubjectScopeOwn confines a verified end user to
// their own records.
const (
	SubjectScopeAll = "all"
	SubjectScopeOwn = "own"
)

// Grant is one (resource, verb) entry in an auth method's custom permission set.
type Grant struct {
	Resource string
	Verb     string
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

	// SubjectScope is the data boundary ("all" or "own"). See [SubjectScopeAll].
	SubjectScope string

	// API key (set for MethodTypeAPIKey).
	Secret       *string // full plaintext; set only by Create (shown once)
	SecretPrefix *string
	Scope        *string

	TrustedIssuer *TrustedIssuer // MethodTypeTrustedIssuer
	Session       *Session       // MethodTypeSession

	CreatedAt time.Time
	UpdatedAt time.Time
}

func NewAuthMethodsStore(db store.DB) *AuthMethodsStore {
	return &AuthMethodsStore{db: db}
}

type AuthMethodsStore struct {
	db store.DB
}

// withinTx runs fn inside a transaction when db is a real *sqlx.DB. When db is
// already a transaction (e.g. a State built over a *sqlx.Tx), fn runs directly
// on it so the caller's transaction is reused. Either way fn's writes are
// atomic.
func withinTx(ctx context.Context, db store.DB, fn func(sqlx.ExtContext) error) error {
	beginner, ok := db.(interface {
		BeginTxx(context.Context, *sql.TxOptions) (*sqlx.Tx, error)
	})
	if !ok {
		return fn(db)
	}

	tx, err := beginner.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // no-op once committed

	if err := fn(tx); err != nil {
		return err
	}
	return tx.Commit()
}

// CreateAuthMethodInput carries the fields to create an auth method. The
// type-specific block (Scope, TrustedIssuer, Session) must match Type.
type CreateAuthMethodInput struct {
	Type          MethodType
	Name          string
	Description   *string
	Role          string
	SubjectScope  string
	Scope         *string
	Grants        []Grant
	TrustedIssuer *TrustedIssuer
	Session       *Session
}

// CreateAuthMethod inserts an auth method, its type-specific credential row, and
// its grants in a single transaction. For api_key methods the generated secret
// is returned once on the result's Secret field.
func (s *AuthMethodsStore) CreateAuthMethod(ctx context.Context, projectID uuid.UUID, in CreateAuthMethodInput) (*AuthMethod, error) {
	id := uuid.New()
	var secret *string

	err := withinTx(ctx, s.db, func(q sqlx.ExtContext) error {
		subjectScope := in.SubjectScope
		if subjectScope == "" {
			subjectScope = SubjectScopeAll
		}
		if _, err := q.ExecContext(ctx,
			`INSERT INTO auth_methods (id, project_id, type, name, description, role, subject_scope) VALUES ($1, $2, $3, $4, $5, $6, $7)`,
			id, projectID, string(in.Type), in.Name, in.Description, in.Role, subjectScope); err != nil {
			return err
		}

		switch in.Type {
		case MethodTypeAPIKey:
			scope := ptr.FromOr(in.Scope, ScopeSecret)
			plaintext, prefix, hash, err := newSecret(scope)
			if err != nil {
				return err
			}
			secret = &plaintext
			if _, err := q.ExecContext(ctx,
				`INSERT INTO auth_method_api_keys (auth_method_id, secret_hash, secret_prefix, scope) VALUES ($1, $2, $3, $4)`,
				id, hash, prefix, scope); err != nil {
				return err
			}

		case MethodTypeTrustedIssuer:
			ti := ptr.FromOr(in.TrustedIssuer, TrustedIssuer{})
			subjectClaim := ti.SubjectClaim
			if subjectClaim == "" {
				subjectClaim = "sub"
			}

			// A trusted issuer is resolved at auth time by `iss` alone, with no
			// project context, so the active issuer must be globally unique;
			// otherwise a token could authenticate against the wrong project.
			// (Soft-deleted methods keep their child row, so this is enforced
			// here rather than via a DB constraint that would block re-adding a
			// deleted issuer.)
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
			if _, err := q.ExecContext(ctx,
				`INSERT INTO auth_method_trusted_issuers (auth_method_id, jwks_url, public_cert, issuer, audience, subject_claim)
				 VALUES ($1, $2, $3, $4, $5, $6)`,
				id,
				sql.NullString{String: ti.JWKSURL, Valid: ti.JWKSURL != ""},
				sql.NullString{String: ti.PublicCert, Valid: ti.PublicCert != ""},
				ti.Issuer,
				sql.NullString{String: ti.Audience, Valid: ti.Audience != ""},
				subjectClaim); err != nil {
				return err
			}

		case MethodTypeSession:
			ttl := 900
			if in.Session != nil && in.Session.TTLSeconds > 0 {
				ttl = in.Session.TTLSeconds
			}
			if _, err := q.ExecContext(ctx,
				`INSERT INTO auth_method_sessions (auth_method_id, ttl_seconds) VALUES ($1, $2)`,
				id, ttl); err != nil {
				return err
			}

		default:
			return fmt.Errorf("management: unknown auth method type %q", in.Type)
		}

		return insertGrants(ctx, q, id, in.Grants)
	})
	if err != nil {
		return nil, err
	}

	method, err := s.GetAuthMethod(ctx, projectID, id)
	if err != nil {
		return nil, err
	}
	method.Secret = secret // shown to the caller exactly once
	return method, nil
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

// authMethodRow is the flattened projection joining an auth method to all three
// optional credential tables; only the columns for the row's type are non-NULL.
type authMethodRow struct {
	ID           uuid.UUID `db:"id"`
	ProjectID    uuid.UUID `db:"project_id"`
	Type         string    `db:"type"`
	Name         string    `db:"name"`
	Description  *string   `db:"description"`
	Role         string    `db:"role"`
	SubjectScope string    `db:"subject_scope"`
	CreatedAt    time.Time `db:"created_at"`
	UpdatedAt    time.Time `db:"updated_at"`

	SecretPrefix *string `db:"secret_prefix"`
	Scope        *string `db:"scope"`

	JWKSURL      *string `db:"jwks_url"`
	PublicCert   *string `db:"public_cert"`
	Issuer       *string `db:"issuer"`
	Audience     *string `db:"audience"`
	SubjectClaim *string `db:"subject_claim"`

	TTLSeconds *int `db:"ttl_seconds"`
}

func (r authMethodRow) toMethod() *AuthMethod {
	m := &AuthMethod{
		ID:           r.ID,
		ProjectID:    r.ProjectID,
		Type:         MethodType(r.Type),
		Name:         r.Name,
		Description:  r.Description,
		Role:         r.Role,
		SubjectScope: r.SubjectScope,
		SecretPrefix: r.SecretPrefix,
		Scope:        r.Scope,
		CreatedAt:    r.CreatedAt,
		UpdatedAt:    r.UpdatedAt,
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
	return m
}

// authMethodColumns is the shared projection; authMethodFrom is the join graph.
// They are kept separate so List can add a window column to the SELECT list
// without it landing in the FROM clause.
const authMethodColumns = `m.id, m.project_id, m.type, m.name, m.description, m.role, m.subject_scope, m.created_at, m.updated_at,
	       k.secret_prefix, k.scope,
	       t.jwks_url, t.public_cert, t.issuer, t.audience, t.subject_claim,
	       s.ttl_seconds`

const authMethodFrom = `
	FROM auth_methods m
	LEFT JOIN auth_method_api_keys k ON k.auth_method_id = m.id
	LEFT JOIN auth_method_trusted_issuers t ON t.auth_method_id = m.id
	LEFT JOIN auth_method_sessions s ON s.auth_method_id = m.id`

func (s *AuthMethodsStore) GetAuthMethod(ctx context.Context, projectID, methodID uuid.UUID) (*AuthMethod, error) {
	stmt := `SELECT ` + authMethodColumns + authMethodFrom + `
	WHERE m.id = $1 AND m.project_id = $2 AND m.deleted_at IS NULL`

	var row authMethodRow
	if err := s.db.GetContext(ctx, &row, stmt, methodID, projectID); err != nil {
		return nil, err
	}
	method := row.toMethod()

	grants, err := s.grantsFor(ctx, []uuid.UUID{methodID})
	if err != nil {
		return nil, err
	}
	method.Grants = grants[methodID]
	return method, nil
}

func (s *AuthMethodsStore) ListAuthMethods(ctx context.Context, projectID uuid.UUID, pagination store.Pagination) ([]AuthMethod, int, error) {
	stmt := `SELECT ` + authMethodColumns + `,
	       COUNT(*) OVER () AS total_count` + authMethodFrom + `
	WHERE m.project_id = $1 AND m.deleted_at IS NULL
	ORDER BY m.created_at DESC
	LIMIT $2 OFFSET $3`

	type listRow struct {
		authMethodRow
		TotalCount int `db:"total_count"`
	}

	var rows []listRow
	if err := s.db.SelectContext(ctx, &rows, stmt, projectID, pagination.Limit, pagination.Offset); err != nil {
		return nil, 0, err
	}
	if len(rows) == 0 {
		return []AuthMethod{}, 0, nil
	}

	ids := make([]uuid.UUID, len(rows))
	for i, r := range rows {
		ids[i] = r.ID
	}
	grants, err := s.grantsFor(ctx, ids)
	if err != nil {
		return nil, 0, err
	}

	methods := make([]AuthMethod, len(rows))
	for i, r := range rows {
		m := r.toMethod()
		m.Grants = grants[m.ID]
		methods[i] = *m
	}
	return methods, rows[0].TotalCount, nil
}

// grantsFor loads grants for the given auth methods, keyed by method id.
func (s *AuthMethodsStore) grantsFor(ctx context.Context, ids []uuid.UUID) (map[uuid.UUID][]Grant, error) {
	out := make(map[uuid.UUID][]Grant, len(ids))
	if len(ids) == 0 {
		return out, nil
	}

	const stmt = `
	SELECT auth_method_id, resource, verb
	FROM auth_method_grants
	WHERE auth_method_id = ANY($1::uuid[])
	ORDER BY resource, verb`

	var rows []struct {
		AuthMethodID uuid.UUID `db:"auth_method_id"`
		Resource     string    `db:"resource"`
		Verb         string    `db:"verb"`
	}
	if err := s.db.SelectContext(ctx, &rows, stmt, pq.Array(ids)); err != nil {
		return nil, err
	}
	for _, r := range rows {
		out[r.AuthMethodID] = append(out[r.AuthMethodID], Grant{Resource: r.Resource, Verb: r.Verb})
	}
	return out, nil
}

// UpdateAuthMethodInput carries the mutable fields. Nil fields are unchanged;
// Grants is rewritten only when non-nil (replacing the whole set).
type UpdateAuthMethodInput struct {
	Name         *string
	Description  *string
	Role         *string
	SubjectScope *string
	Grants       []Grant
}

func (s *AuthMethodsStore) UpdateAuthMethod(ctx context.Context, projectID, methodID uuid.UUID, in UpdateAuthMethodInput) error {
	return withinTx(ctx, s.db, func(q sqlx.ExtContext) error {
		if _, err := q.ExecContext(ctx,
			`UPDATE auth_methods
			 SET name = COALESCE($1, name), description = COALESCE($2, description),
			     role = COALESCE($3, role), subject_scope = COALESCE($4, subject_scope)
			 WHERE id = $5 AND project_id = $6 AND deleted_at IS NULL`,
			in.Name, in.Description, in.Role, in.SubjectScope, methodID, projectID); err != nil {
			return err
		}

		// Replace the whole grant set when provided.
		if in.Grants != nil {
			if _, err := q.ExecContext(ctx, `DELETE FROM auth_method_grants WHERE auth_method_id = $1`, methodID); err != nil {
				return err
			}
			if err := insertGrants(ctx, q, methodID, in.Grants); err != nil {
				return err
			}
		}

		return nil
	})
}

func (s *AuthMethodsStore) DeleteAuthMethod(ctx context.Context, projectID, methodID uuid.UUID) error {
	const stmt = `
	UPDATE auth_methods
	SET deleted_at = NOW()
	WHERE id = $1 AND project_id = $2 AND deleted_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, methodID, projectID)
	return err
}
