package linkwrap

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strings"

	"github.com/google/uuid"
)

// LinkPayload is the plaintext structure encrypted into each tracking URL.
type LinkPayload struct {
	ProjectID  uuid.UUID
	CampaignID uuid.UUID
	UserID     uuid.UUID
	URL        string
}

// Encrypt produces a base64url-encoded encrypted token from a LinkPayload.
// The payload is serialized as a compact binary format (3 × 16-byte raw UUIDs
// followed by the URL bytes), then encrypted with AES-256-GCM.
func Encrypt(key []byte, payload LinkPayload) (string, error) {
	// Binary encode: 3 × 16-byte UUIDs + URL bytes
	plaintext := make([]byte, 0, 48+len(payload.URL))
	plaintext = append(plaintext, payload.ProjectID[:]...)
	plaintext = append(plaintext, payload.CampaignID[:]...)
	plaintext = append(plaintext, payload.UserID[:]...)
	plaintext = append(plaintext, payload.URL...)

	block, err := aes.NewCipher(key)
	if err != nil {
		return "", err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return "", err
	}

	nonce := make([]byte, gcm.NonceSize())
	if _, err := io.ReadFull(rand.Reader, nonce); err != nil {
		return "", err
	}

	// nonce is prepended to the ciphertext
	ciphertext := gcm.Seal(nonce, nonce, plaintext, nil)
	return base64.RawURLEncoding.EncodeToString(ciphertext), nil
}

// Decrypt decodes and decrypts a base64url token back into a LinkPayload.
func Decrypt(key []byte, token string) (LinkPayload, error) {
	var payload LinkPayload

	data, err := base64.RawURLEncoding.DecodeString(token)
	if err != nil {
		return payload, fmt.Errorf("invalid token encoding: %w", err)
	}

	block, err := aes.NewCipher(key)
	if err != nil {
		return payload, err
	}

	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return payload, err
	}

	nonceSize := gcm.NonceSize()
	if len(data) < nonceSize {
		return payload, fmt.Errorf("token too short")
	}

	nonce, ciphertext := data[:nonceSize], data[nonceSize:]
	plaintext, err := gcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return payload, fmt.Errorf("decryption failed: %w", err)
	}

	// Binary decode: 3 × 16-byte UUIDs + remaining URL bytes
	if len(plaintext) < 48 {
		return payload, fmt.Errorf("payload too short")
	}
	copy(payload.ProjectID[:], plaintext[0:16])
	copy(payload.CampaignID[:], plaintext[16:32])
	copy(payload.UserID[:], plaintext[32:48])
	payload.URL = string(plaintext[48:])

	return payload, nil
}

// IsSafeRedirectURL validates that a URL is safe to redirect to.
func IsSafeRedirectURL(u string) bool {
	parsed, err := url.Parse(u)
	if err != nil {
		return false
	}

	// Must be an absolute HTTP(S) URL
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return false
	}

	// Must have a host
	if parsed.Host == "" {
		return false
	}

	// Block redirects to localhost/private IPs (SSRF prevention)
	host := parsed.Hostname()
	if host == "localhost" || host == "127.0.0.1" || host == "::1" {
		return false
	}

	// Block javascript: URLs that might be URL-encoded
	if strings.Contains(strings.ToLower(u), "javascript:") {
		return false
	}

	return true
}

// shouldWrapURL determines if a URL should be wrapped for click tracking.
// Skips mailto:, tel:, #anchors, javascript:, and data: URIs.
// Also skips unsubscribe and preference URLs (already handled by the platform).
func shouldWrapURL(u string) bool {
	if u == "" {
		return false
	}

	lower := strings.ToLower(u)

	// Skip non-HTTP schemes
	if strings.HasPrefix(lower, "mailto:") ||
		strings.HasPrefix(lower, "tel:") ||
		strings.HasPrefix(lower, "javascript:") ||
		strings.HasPrefix(lower, "data:") ||
		strings.HasPrefix(lower, "#") {
		return false
	}

	// Skip platform-managed URLs (unsubscribe, preferences)
	if strings.Contains(lower, "/unsubscribe/") || strings.Contains(lower, "/preferences/") {
		return false
	}

	// Only wrap http:// and https:// URLs
	return strings.HasPrefix(lower, "http://") || strings.HasPrefix(lower, "https://")
}
