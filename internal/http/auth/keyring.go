package auth

import (
	"crypto/ecdsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"errors"
	"fmt"
	"strings"

	"github.com/golang-jwt/jwt/v5"
)

// keyIDLength is how much of the key thumbprint is used as a `kid`. Sixteen
// base64url characters is 96 bits of a SHA-256 digest -- far more than enough to
// distinguish the handful of keys a rotation ever holds, while keeping the token
// header small.
const keyIDLength = 16

// Keyring holds the one key that signs and the set of keys that verify. A
// rotation adds the retiring key to the verify-only set so tokens already in
// browsers keep working, then drops it once they have all expired.
//
// Verification dispatches on the token's `kid` and never tries every key in
// turn. Trying keys in turn would mean an attacker can have each of our keys
// attempt to verify their forgery, turning key rotation into extra attack
// surface instead of less; it also hides the fact that a token was minted under
// a key we no longer intend to honour.
type Keyring struct {
	active    *ecdsa.PrivateKey
	activeKID string
	verify    map[string]*ecdsa.PublicKey
}

// NewKeyring builds a keyring from a PEM-encoded EC (P-256) private key plus any
// number of previous keys that must still verify. A blank active key returns
// (nil, nil): the caller decides whether that is a disabled feature or a fatal
// misconfiguration.
func NewKeyring(activePEM string, previousPEMs []string) (*Keyring, error) {
	if activePEM == "" {
		return nil, nil
	}

	active, err := parseECPrivateKeyPEM(activePEM)
	if err != nil {
		return nil, err
	}

	activeKID, err := KeyID(&active.PublicKey)
	if err != nil {
		return nil, err
	}

	ring := &Keyring{
		active:    active,
		activeKID: activeKID,
		verify:    map[string]*ecdsa.PublicKey{activeKID: &active.PublicKey},
	}

	for _, previous := range previousPEMs {
		if previous == "" {
			continue
		}
		key, err := parseECPrivateKeyPEM(previous)
		if err != nil {
			return nil, fmt.Errorf("auth: previous signing key: %w", err)
		}
		kid, err := KeyID(&key.PublicKey)
		if err != nil {
			return nil, err
		}
		// A previous key only ever verifies; the private half is parsed because
		// that is the format operators already hold, not because it is used.
		ring.verify[kid] = &key.PublicKey
	}

	return ring, nil
}

// ActiveKID is the `kid` stamped on freshly minted tokens.
func (k *Keyring) ActiveKID() string { return k.activeKID }

// Sign signs claims with the active key, stamping the active `kid` so a verifier
// can pick the right key without guessing.
func (k *Keyring) Sign(claims jwt.Claims) (string, error) {
	token := jwt.NewWithClaims(jwt.SigningMethodES256, claims)
	token.Header["kid"] = k.activeKID
	return token.SignedString(k.active)
}

// Keyfunc resolves the verification key named by the token's `kid`. A token
// without a `kid`, or naming a key this ring does not hold, is rejected outright.
func (k *Keyring) Keyfunc(token *jwt.Token) (any, error) {
	kid, ok := token.Header["kid"].(string)
	if !ok || kid == "" {
		return nil, errors.New("auth: token has no key id")
	}
	key, ok := k.verify[kid]
	if !ok {
		return nil, fmt.Errorf("auth: unknown key id %q", kid)
	}
	return key, nil
}

// KeyID derives a stable identifier for a public key: the first
// [keyIDLength] base64url characters of the SHA-256 digest of its SPKI DER
// encoding. It is derived rather than configured so two replicas holding the
// same key always agree on its id without any coordination.
func KeyID(pub *ecdsa.PublicKey) (string, error) {
	der, err := x509.MarshalPKIXPublicKey(pub)
	if err != nil {
		return "", fmt.Errorf("auth: cannot encode public key: %w", err)
	}
	sum := sha256.Sum256(der)
	return base64.RawURLEncoding.EncodeToString(sum[:])[:keyIDLength], nil
}

// parseECPrivateKeyPEM decodes a PEM block and parses the EC private key inside.
//
// A literal `\n` two-character sequence is treated as a newline first. PEM is
// multi-line and most deployment surfaces (compose files, Kubernetes env vars,
// PaaS dashboards) only carry single-line values, so the escaped form is how a
// key actually reaches the process; refusing it would push operators towards
// keeping their signing key in a file instead.
func parseECPrivateKeyPEM(pemKey string) (*ecdsa.PrivateKey, error) {
	pemKey = strings.ReplaceAll(pemKey, `\n`, "\n")

	block, _ := pem.Decode([]byte(pemKey))
	if block == nil {
		return nil, errors.New("auth: signing key is not valid PEM")
	}
	return parseECPrivateKey(block.Bytes)
}

// parseECPrivateKey accepts either a SEC1 (`EC PRIVATE KEY`) or PKCS#8
// (`PRIVATE KEY`) EC private key.
func parseECPrivateKey(der []byte) (*ecdsa.PrivateKey, error) {
	if key, err := x509.ParseECPrivateKey(der); err == nil {
		return key, nil
	}
	if key, err := x509.ParsePKCS8PrivateKey(der); err == nil {
		if ec, ok := key.(*ecdsa.PrivateKey); ok {
			return ec, nil
		}
		return nil, errors.New("auth: signing key is not an EC key")
	}
	return nil, errors.New("auth: unsupported signing key (want a PEM EC private key)")
}
