package providers

import (
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// HMACJWTGenerator generates JWT tokens using HMAC-SHA256 (symmetric key)
type HMACJWTGenerator struct {
	secret    []byte
	tokenLife time.Duration
}

// NewHMACJWTGenerator creates a JWT generator that uses HMAC-SHA256
func NewHMACJWTGenerator(secret string, tokenLife time.Duration) *HMACJWTGenerator {
	return &HMACJWTGenerator{
		secret:    []byte(secret),
		tokenLife: tokenLife,
	}
}

// Generate creates a new JWT token for the given admin ID using HMAC
func (g *HMACJWTGenerator) Generate(adminID uuid.UUID) (token string, expiresAt time.Time, err error) {
	expiresAt = time.Now().Add(g.tokenLife)

	// TODO: configure a issuer
	claims := jwt.RegisteredClaims{
		Subject:   adminID.String(),
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		NotBefore: jwt.NewNumericDate(time.Now()),
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err = jwtToken.SignedString(g.secret)
	if err != nil {
		return "", time.Time{}, err
	}

	return token, expiresAt, nil
}

// TokenLife returns the configured token lifetime
func (g *HMACJWTGenerator) TokenLife() time.Duration {
	return g.tokenLife
}

// Keyfunc returns a jwt.Keyfunc for validating tokens
func (g *HMACJWTGenerator) Keyfunc() jwt.Keyfunc {
	return func(token *jwt.Token) (interface{}, error) {
		return g.secret, nil
	}
}
