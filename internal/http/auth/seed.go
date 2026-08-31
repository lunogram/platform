package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/google/uuid"
	"github.com/lunogram/platform/internal/password"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

// Seeder writes the configured credential into an admin account.
type Seeder struct {
	exchanger *Exchanger
	mgmt      *management.State
	logger    *zap.Logger
}

func NewSeeder(exchanger *Exchanger, mgmt *management.State, logger *zap.Logger) *Seeder {
	return &Seeder{exchanger: exchanger, mgmt: mgmt, logger: logger}
}

// Seed makes the configured email and password into a real account.
//
// The pair used to be a credential the login compared against, which meant
// everybody who signed in with it was the same synthetic identity, and the
// plaintext had to stay in the environment for as long as the deployment ran.
// Seeding it instead gives the quickstart a genuine admin: it holds its own RBAC
// tuples, appears in the admin list, can change its own password, and needs the
// plaintext exactly once.
//
// It never replaces a secret that is already stored. A deployment restarts for
// reasons that have nothing to do with credentials, and re-applying the
// environment on every boot would silently undo a password somebody changed in
// the console — the change would appear to work and then revert. What it does do
// is fill a missing secret, which is what carries a deployment that has been
// signing in with the old configured credential across to a stored one: the
// admin is already there, found by address, and only the hash is new.
//
// A failure is returned rather than logged and swallowed. An operator who set
// these variables is telling us how they intend to sign in, and starting a
// deployment they cannot get into is worse than not starting.
func (s *Seeder) Seed(ctx context.Context, email, plain string) error {
	email = strings.ToLower(strings.TrimSpace(email))
	if email == "" || plain == "" {
		return nil
	}

	admin, err := s.mgmt.ResolveAdminByEmail(ctx, email)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return s.create(ctx, email, plain)
	case errors.Is(err, management.ErrAmbiguousEmail):
		// Two admins claim the address and nothing can say which one the
		// configured credential meant. Refusing is the only honest answer.
		return fmt.Errorf("auth: the configured AUTH_BASIC_EMAIL is claimed by more than one admin")
	case err != nil:
		return fmt.Errorf("auth: failed to look up the configured admin: %w", err)
	}

	return s.complete(ctx, admin.ID, plain)
}

// create provisions the account the configured credential names.
func (s *Seeder) create(ctx context.Context, email, plain string) error {
	hash, err := password.Hash(plain)
	if err != nil {
		return fmt.Errorf("auth: failed to hash the configured password: %w", err)
	}

	identity := &VerifiedIdentity{
		Issuer:   management.LocalIssuer,
		Provider: management.IdentityProviderBasic,
		Email:    email,
		// The operator configured this address, so it is theirs by definition:
		// there is no self-service profile here for somebody to type another
		// person's address into.
		EmailVerified: true,
	}

	adminID, _, err := s.exchanger.ProvisionAdmin(ctx, identity, Provisioning{
		// The subject of a local identity is the admin's own id, which does not
		// exist until the transaction has created them.
		Credential: func(adminID uuid.UUID) (string, string, error) { return adminID.String(), hash, nil },
	})
	if err != nil {
		return fmt.Errorf("auth: failed to provision the configured admin: %w", err)
	}

	s.logger.Info("seeded the configured admin account from AUTH_BASIC_EMAIL / AUTH_BASIC_PASSWORD; "+
		"the password is now stored as a hash, so the plaintext can be removed from the environment",
		zap.String("admin_id", adminID.String()))
	return nil
}

// complete fills in a missing secret on an admin that already exists.
func (s *Seeder) complete(ctx context.Context, adminID uuid.UUID, plain string) error {
	identity, err := s.mgmt.GetLocalIdentity(ctx, adminID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		identity = nil
	case err != nil:
		return fmt.Errorf("auth: failed to look up the configured admin's credential: %w", err)
	}

	if identity != nil && identity.SecretHash != nil {
		s.logger.Info("the configured admin already has a password, so AUTH_BASIC_PASSWORD was not applied; "+
			"remove it from the environment, or reset the password through the console",
			zap.String("admin_id", adminID.String()))
		return nil
	}

	hash, err := password.Hash(plain)
	if err != nil {
		return fmt.Errorf("auth: failed to hash the configured password: %w", err)
	}

	if identity == nil {
		if _, err := s.mgmt.CreateAdminIdentity(ctx, management.AdminIdentity{
			AdminID:       adminID,
			Provider:      management.IdentityProviderBasic,
			Issuer:        management.LocalIssuer,
			Subject:       adminID.String(),
			EmailVerified: true,
			SecretHash:    &hash,
		}); err != nil {
			return fmt.Errorf("auth: failed to store the configured password: %w", err)
		}

		s.logger.Info("gave the configured admin a stored password; "+
			"the plaintext can be removed from the environment",
			zap.String("admin_id", adminID.String()))
		return nil
	}

	if err := s.mgmt.SetAdminIdentitySecret(ctx, identity.ID, hash); err != nil {
		return fmt.Errorf("auth: failed to store the configured password: %w", err)
	}

	s.logger.Info("filled in the configured admin's missing password; "+
		"the plaintext can be removed from the environment",
		zap.String("admin_id", adminID.String()))
	return nil
}
