package management

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
)

// secretPrefixLen is the number of leading characters of a secret retained for
// display/identification (e.g. "sk_" plus 8 hex characters).
const secretPrefixLen = 11

// generateRandomHex returns a cryptographically secure 32-byte random value,
// hex-encoded.
func generateRandomHex() (string, error) {
	b := make([]byte, 32)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}
	return hex.EncodeToString(b), nil
}

// hashSecret returns the hex-encoded SHA-256 of a secret. Secrets are stored and
// looked up by this hash so the plaintext is never persisted. It matches the
// pgcrypto expression used by the migration: encode(digest(value,'sha256'),'hex').
func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// secretPrefix returns the leading, non-sensitive portion of a secret.
func secretPrefix(secret string) string {
	if len(secret) <= secretPrefixLen {
		return secret
	}
	return secret[:secretPrefixLen]
}

// newSecret generates a fresh API secret, returning the full plaintext (shown to
// the caller exactly once), its display prefix, and the hash to persist. Keys are
// prefixed "sk_" so they are recognisable and detectable by secret scanners. API
// keys are always private (backend-only); there is no browser-safe key variant.
func newSecret() (plaintext, prefix, hash string, err error) {
	raw, err := generateRandomHex()
	if err != nil {
		return "", "", "", err
	}

	plaintext = "sk_" + raw
	return plaintext, secretPrefix(plaintext), hashSecret(plaintext), nil
}
