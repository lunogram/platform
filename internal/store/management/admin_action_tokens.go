package management

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"time"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/store"
)

// Token purposes. The set is mirrored by a CHECK constraint on
// admin_action_tokens.purpose.
const (
	// ActionTokenEmailVerification proves the recipient owns the address an
	// account was registered with.
	ActionTokenEmailVerification = "email_verification"
	// ActionTokenPasswordReset authorises setting a new password without
	// knowing the old one.
	ActionTokenPasswordReset = "password_reset"
)

// Action token lifetimes.
//
// Verification is long because it is a convenience: the account already works,
// and the link only has to survive somebody getting round to their inbox. Reset
// is short because it is a full account takeover in one URL, and that URL sits
// in an inbox, in mail-server logs, and in whatever scans the message on the way.
const (
	EmailVerificationTTL = 24 * time.Hour
	PasswordResetTTL     = time.Hour
)

// actionTokenBytes is the entropy in a token. 32 bytes is far past what a
// brute-force over a one-hour window could reach, and the token is single-use
// so there is no second guess.
const actionTokenBytes = 32

func NewAdminActionTokensStore(db store.DB) *AdminActionTokensStore {
	return &AdminActionTokensStore{db: db}
}

type AdminActionTokensStore struct {
	db store.DB
}

// NewActionToken generates a token and the hash it is stored under.
//
// The plaintext is returned once, to be put in an email and then forgotten: it
// is never written to the database, never logged and never returned in an HTTP
// response.
func NewActionToken() (plaintext string, hash []byte, err error) {
	raw := make([]byte, actionTokenBytes)
	if _, err := rand.Read(raw); err != nil {
		return "", nil, err
	}

	plaintext = base64.RawURLEncoding.EncodeToString(raw)
	return plaintext, HashActionToken(plaintext), nil
}

// HashActionToken is the one-way function the table is keyed on. SHA-256 is
// sufficient because the input is full-entropy CSPRNG output, not a human
// secret that needs stretching.
func HashActionToken(plaintext string) []byte {
	sum := sha256.Sum256([]byte(plaintext))
	return sum[:]
}

// CreateAdminActionToken records a token for an admin.
func (s *AdminActionTokensStore) CreateAdminActionToken(ctx context.Context, adminID uuid.UUID, purpose string, hash []byte, ttl time.Duration) error {
	stmt := `
	INSERT INTO admin_action_tokens (admin_id, purpose, token_hash, expires_at)
	VALUES ($1, $2, $3, NOW() + ($4 * INTERVAL '1 second'))`

	_, err := s.db.ExecContext(ctx, stmt, adminID, purpose, hash, int64(ttl.Seconds()))
	return err
}

// ConsumeAdminActionToken redeems a token and returns the admin it was issued
// to, or sql.ErrNoRows when it does not exist, has expired, has already been
// used, or was issued for a different purpose.
//
// Selection and consumption are ONE statement on purpose. Reading the row and
// then marking it used would let two concurrent redemptions of the same link
// both pass the read; the guarded UPDATE makes the second one match zero rows.
// The purpose is part of the guard so a verification link can never be replayed
// as a password reset.
func (s *AdminActionTokensStore) ConsumeAdminActionToken(ctx context.Context, purpose string, hash []byte) (uuid.UUID, error) {
	stmt := `
	UPDATE admin_action_tokens
	SET consumed_at = NOW()
	WHERE token_hash = $1
	AND purpose = $2
	AND consumed_at IS NULL
	AND expires_at > NOW()
	RETURNING admin_id`

	var adminID uuid.UUID
	if err := s.db.GetContext(ctx, &adminID, stmt, hash, purpose); err != nil {
		return uuid.Nil, err
	}
	return adminID, nil
}

// InvalidateAdminActionTokens burns every outstanding token of one purpose for
// an admin.
//
// A password change has to do this: a reset link that was already in flight is
// a way back into the account for whoever triggered it, and the whole point of
// changing the password was to close that door.
func (s *AdminActionTokensStore) InvalidateAdminActionTokens(ctx context.Context, adminID uuid.UUID, purpose string) error {
	stmt := `
	UPDATE admin_action_tokens
	SET consumed_at = NOW()
	WHERE admin_id = $1 AND purpose = $2 AND consumed_at IS NULL`

	_, err := s.db.ExecContext(ctx, stmt, adminID, purpose)
	return err
}

// PurgeExpiredAdminActionTokens deletes tokens that can no longer be redeemed.
// Retention beyond expiry buys nothing: the row holds a hash of a dead secret.
func (s *AdminActionTokensStore) PurgeExpiredAdminActionTokens(ctx context.Context, retain time.Duration) (int64, error) {
	stmt := `
	DELETE FROM admin_action_tokens
	WHERE expires_at < NOW() - $1::interval
	OR (consumed_at IS NOT NULL AND consumed_at < NOW() - $1::interval)`

	result, err := s.db.ExecContext(ctx, stmt, retain.String())
	if err != nil {
		return 0, err
	}
	return result.RowsAffected()
}
