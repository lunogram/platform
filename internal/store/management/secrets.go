package management

import (
	"crypto/sha256"
	"encoding/hex"
)

// secretPrefixLen is the number of leading characters of a secret retained for
// display/identification (e.g. "pk_" plus 8 hex characters).
const secretPrefixLen = 11

// hashSecret returns the hex-encoded SHA-256 of a secret. Secrets are stored and
// looked up by this hash so the plaintext is never persisted. It matches the
// pgcrypto expression used by the backfill migration:
// encode(digest(value, 'sha256'), 'hex').
func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// secretPrefix returns the leading, non-sensitive portion of a secret, used to
// identify a key in list views without revealing it.
func secretPrefix(secret string) string {
	if len(secret) <= secretPrefixLen {
		return secret
	}
	return secret[:secretPrefixLen]
}

// newSecret generates a fresh API secret for the given scope. It returns the
// full plaintext (shown to the caller exactly once), its display prefix, and the
// hash to persist. Public keys are prefixed "pk_" and secret keys "sk_" so they
// are recognisable and detectable by secret scanners.
func newSecret(scope string) (plaintext, prefix, hash string, err error) {
	raw, err := generateKeyValue()
	if err != nil {
		return "", "", "", err
	}

	tag := "sk_"
	if scope == ScopePublic {
		tag = "pk_"
	}

	plaintext = tag + raw
	return plaintext, secretPrefix(plaintext), hashSecret(plaintext), nil
}
