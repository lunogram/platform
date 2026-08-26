package management

import (
	"context"
	"sync"
	"time"

	"github.com/google/uuid"
	iredis "github.com/lunogram/platform/internal/redis"
	"github.com/lunogram/platform/internal/store"
)

func NewAdminSessionsStore(db store.DB, cache *adminSessionCache) *AdminSessionsStore {
	return &AdminSessionsStore{db: db, cache: cache}
}

type AdminSessionsStore struct {
	db    store.DB
	cache *adminSessionCache
}

// AdminSession is one console login. The minted token carries only this row's
// id, so everything that can change -- role, organization, email -- is re-read
// per request instead of being frozen into a bearer credential.
type AdminSession struct {
	ID              uuid.UUID  `db:"id" json:"id"`
	AdminID         uuid.UUID  `db:"admin_id" json:"admin_id"`
	AdminIdentityID *uuid.UUID `db:"admin_identity_id" json:"admin_identity_id"`
	Impersonated    bool       `db:"impersonated" json:"impersonated"`
	// ImpersonatorAdminID is set only when the upstream impersonator maps to an
	// admin of ours, which it usually does not. ImpersonatorSubject is always
	// recorded for an impersonated session.
	ImpersonatorAdminID *uuid.UUID `db:"impersonator_admin_id" json:"impersonator_admin_id"`
	ImpersonatorSubject *string    `db:"impersonator_subject" json:"impersonator_subject"`
	UpstreamExpiresAt   *time.Time `db:"upstream_expires_at" json:"upstream_expires_at"`
	IssuedAt            time.Time  `db:"issued_at" json:"issued_at"`
	LastSeenAt          time.Time  `db:"last_seen_at" json:"last_seen_at"`
	ExpiresAt           time.Time  `db:"expires_at" json:"expires_at"`
	AbsoluteExpiresAt   time.Time  `db:"absolute_expires_at" json:"absolute_expires_at"`
	RevokedAt           *time.Time `db:"revoked_at" json:"revoked_at"`
	Refreshable         bool       `db:"refreshable" json:"refreshable"`
	UserAgent           *string    `db:"user_agent" json:"user_agent"`
	IP                  *string    `db:"ip" json:"ip"`
	CreatedAt           time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time  `db:"updated_at" json:"updated_at"`
}

// Active reports whether the session may still authenticate a request at now.
// All three predicates are checked together so no caller can accidentally test
// one and forget the others.
func (s *AdminSession) Active(now time.Time) bool {
	return s.RevokedAt == nil &&
		now.Before(s.ExpiresAt) &&
		now.Before(s.AbsoluteExpiresAt)
}

const adminSessionColumns = `id, admin_id, admin_identity_id, impersonated, impersonator_admin_id,
	impersonator_subject, upstream_expires_at, issued_at, last_seen_at, expires_at,
	absolute_expires_at, revoked_at, refreshable, user_agent, ip, created_at, updated_at`

// CreateAdminSession records a new console session. The impersonation
// invariants (a clamped, non-refreshable lifetime bounded by the upstream
// session) are enforced by CHECK constraints on the table, so a caller that
// forgets to clamp gets a write failure rather than an over-privileged session.
func (s *AdminSessionsStore) CreateAdminSession(ctx context.Context, session AdminSession) (*AdminSession, error) {
	stmt := `
	INSERT INTO admin_sessions (
		admin_id, admin_identity_id, impersonated, impersonator_admin_id, impersonator_subject,
		upstream_expires_at, expires_at, absolute_expires_at, refreshable, user_agent, ip
	)
	VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::inet)
	RETURNING ` + adminSessionColumns

	var created AdminSession
	err := s.db.GetContext(ctx, &created, stmt,
		session.AdminID,
		session.AdminIdentityID,
		session.Impersonated,
		session.ImpersonatorAdminID,
		session.ImpersonatorSubject,
		session.UpstreamExpiresAt,
		session.ExpiresAt,
		session.AbsoluteExpiresAt,
		session.Refreshable,
		session.UserAgent,
		session.IP,
	)
	if err != nil {
		return nil, err
	}
	return &created, nil
}

// GetAdminSession resolves a session by id through the two-tier cache. It
// returns the row as stored: liveness is the caller's decision via
// [AdminSession.Active], so a revoked or expired session is still returned here
// and rejected at the middleware with the reason it deserves.
//
// Unlike the JWKS cache this never serves a value on a backing-store error.
// Failing open would serve a session past an explicit revoke, which is the
// exact failure this table exists to prevent; a Redis error degrades to a
// Postgres read, and a Postgres error propagates.
func (s *AdminSessionsStore) GetAdminSession(ctx context.Context, id uuid.UUID) (*AdminSession, error) {
	return s.cache.get(ctx, id, func(ctx context.Context) (AdminSession, error) {
		stmt := `
		SELECT ` + adminSessionColumns + `
		FROM admin_sessions
		WHERE id = $1`

		var session AdminSession
		if err := s.db.GetContext(ctx, &session, stmt, id); err != nil {
			return AdminSession{}, err
		}
		return session, nil
	})
}

// RefreshAdminSession extends a session's idle window and stamps last_seen_at.
// The new expiry is clamped to absolute_expires_at by the table's CHECK, and
// only a live, refreshable session is touched -- an impersonated session is
// written with refreshable = FALSE and can therefore never be extended.
func (s *AdminSessionsStore) RefreshAdminSession(ctx context.Context, id uuid.UUID, expiresAt time.Time) (*AdminSession, error) {
	stmt := `
	UPDATE admin_sessions
	SET expires_at = LEAST($2, absolute_expires_at),
	    last_seen_at = NOW()
	WHERE id = $1
	AND refreshable
	AND revoked_at IS NULL
	AND expires_at > NOW()
	AND absolute_expires_at > NOW()
	RETURNING ` + adminSessionColumns

	var session AdminSession
	err := s.db.GetContext(ctx, &session, stmt, id, expiresAt)
	s.cache.invalidate(ctx, id)
	if err != nil {
		return nil, err
	}
	return &session, nil
}

// RevokeAdminSession ends a session immediately. It is idempotent: re-revoking
// keeps the original revocation time.
func (s *AdminSessionsStore) RevokeAdminSession(ctx context.Context, id uuid.UUID) error {
	stmt := `UPDATE admin_sessions SET revoked_at = NOW() WHERE id = $1 AND revoked_at IS NULL`
	_, err := s.db.ExecContext(ctx, stmt, id)
	// The cache is invalidated even when the write failed: dropping a live entry
	// can only cost a database read, whereas keeping one after a revoke that may
	// have partially applied would serve a dead session.
	s.cache.invalidate(ctx, id)
	return err
}

// RevokeAdminSessionsForAdmin ends every live session an admin holds. It is what
// makes deleting an admin take effect immediately rather than at token expiry.
func (s *AdminSessionsStore) RevokeAdminSessionsForAdmin(ctx context.Context, adminID uuid.UUID) error {
	stmt := `
	UPDATE admin_sessions
	SET revoked_at = NOW()
	WHERE admin_id = $1 AND revoked_at IS NULL
	RETURNING id`

	var ids []uuid.UUID
	err := s.db.SelectContext(ctx, &ids, stmt, adminID)
	for _, id := range ids {
		s.cache.invalidate(ctx, id)
	}
	return err
}

// RevokeAdminSessionsForIdentity ends every live session established through a
// single identity, leaving sessions the admin holds via their other identities
// untouched.
func (s *AdminSessionsStore) RevokeAdminSessionsForIdentity(ctx context.Context, identityID uuid.UUID) error {
	stmt := `
	UPDATE admin_sessions
	SET revoked_at = NOW()
	WHERE admin_identity_id = $1 AND revoked_at IS NULL
	RETURNING id`

	var ids []uuid.UUID
	err := s.db.SelectContext(ctx, &ids, stmt, identityID)
	for _, id := range ids {
		s.cache.invalidate(ctx, id)
	}
	return err
}

// ListAdminSessions returns an admin's sessions, newest first, including ended
// ones so a session list can show what was revoked and when.
func (s *AdminSessionsStore) ListAdminSessions(ctx context.Context, adminID uuid.UUID) ([]AdminSession, error) {
	stmt := `
	SELECT ` + adminSessionColumns + `
	FROM admin_sessions
	WHERE admin_id = $1
	ORDER BY issued_at DESC`

	var sessions []AdminSession
	if err := s.db.SelectContext(ctx, &sessions, stmt, adminID); err != nil {
		return nil, err
	}
	return sessions, nil
}

// PurgeExpiredAdminSessions deletes sessions that ended more than retain ago.
// Sessions are events with no soft-delete flag, so retention is a purge rather
// than a liveness predicate on the hot lookup.
func (s *AdminSessionsStore) PurgeExpiredAdminSessions(ctx context.Context, retain time.Duration) (int64, error) {
	stmt := `
	DELETE FROM admin_sessions
	WHERE absolute_expires_at < NOW() - $1::interval
	OR (revoked_at IS NOT NULL AND revoked_at < NOW() - $1::interval)`

	result, err := s.db.ExecContext(ctx, stmt, retain.String())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}

// adminSessionL1TTL bounds how long a resolved session stays in the per-process
// cache. It is deliberately tiny: a console page fires 5-20 requests and those
// must not each become a Redis round trip, while 5s is short enough that a
// revocation propagates within one page load even on a replica that never saw
// the explicit invalidation.
const adminSessionL1TTL = 5 * time.Second

// adminSessionL2TTL bounds the shared (Redis) tier. Writes invalidate
// explicitly; this is the backstop for a missed invalidation, and is what keeps
// the steady-state hot path off Postgres entirely.
const adminSessionL2TTL = 60 * time.Second

// adminSessionL1Capacity is the point at which a store sweeps expired entries.
// The map only ever holds sessions seen by this process within the L1 TTL, so
// the sweep is the whole eviction policy.
const adminSessionL1Capacity = 4096

type adminSessionL1Entry struct {
	session AdminSession
	expires time.Time
}

// adminSessionCache fronts the session lookup with a per-process L1 over the
// shared Redis L2 over Postgres, mirroring the shape of the JWKS cache. It
// differs from that one in a way that matters: it never serves a value it could
// not confirm. A Redis failure degrades to a Postgres read and a Postgres
// failure propagates, because serving a stale session past an explicit
// revocation is precisely what the session table exists to prevent.
//
// A nil receiver (no Redis configured) is a pass-through to the loader, so the
// store can be constructed unconditionally.
type adminSessionCache struct {
	l2 *iredis.Cache[AdminSession]

	mu sync.Mutex
	l1 map[uuid.UUID]adminSessionL1Entry
}

func newAdminSessionCache(l2 *iredis.Cache[AdminSession]) *adminSessionCache {
	return &adminSessionCache{l2: l2, l1: make(map[uuid.UUID]adminSessionL1Entry)}
}

func (c *adminSessionCache) get(ctx context.Context, id uuid.UUID, load func(context.Context) (AdminSession, error)) (*AdminSession, error) {
	if c == nil {
		session, err := load(ctx)
		if err != nil {
			return nil, err
		}
		return &session, nil
	}

	if session, ok := c.getL1(id); ok {
		return &session, nil
	}

	session, err := c.l2.GetOrLoad(ctx, id.String(), load)
	if err != nil {
		return nil, err
	}
	c.setL1(id, session)
	return &session, nil
}

func (c *adminSessionCache) getL1(id uuid.UUID) (AdminSession, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.l1[id]
	if !ok || !time.Now().Before(entry.expires) {
		return AdminSession{}, false
	}
	return entry.session, true
}

func (c *adminSessionCache) setL1(id uuid.UUID, session AdminSession) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if len(c.l1) >= adminSessionL1Capacity {
		now := time.Now()
		for key, entry := range c.l1 {
			if !now.Before(entry.expires) {
				delete(c.l1, key)
			}
		}
	}
	c.l1[id] = adminSessionL1Entry{session: session, expires: time.Now().Add(adminSessionL1TTL)}
}

// invalidate drops the entry from both tiers. The L1 of other processes cannot
// be reached, which is why its TTL is measured in seconds.
func (c *adminSessionCache) invalidate(ctx context.Context, id uuid.UUID) {
	if c == nil {
		return
	}

	c.mu.Lock()
	delete(c.l1, id)
	c.mu.Unlock()

	_ = c.l2.Invalidate(ctx, id.String())
}
