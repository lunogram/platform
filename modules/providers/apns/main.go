package main

import (
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pdk "github.com/extism/go-pdk"
	"github.com/lunogram/platform/modules/providers/apns/types"
)

//go:export manifest
func Manifest() int32 {
	manifest := types.ProviderManifest{
		Metadata: types.Metadata{
			ID:          "apns",
			Title:       "APNs (Apple Push Notification service)",
			Description: "Send push notifications to iOS devices via Apple Push Notification service (APNs)",
			Icon:        "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgc3Ryb2tlPSJjdXJyZW50Q29sb3IiIHN0cm9rZS13aWR0aD0iMiIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIiBzdHJva2UtbGluZWpvaW49InJvdW5kIj48cGF0aCBkPSJNMTggOGE2IDYgMCAwIDAtMTItMCIvPjxwYXRoIGQ9Ik0yIDhoMTYiLz48cGF0aCBkPSJNNiAxNWwtMi0zaDE2bC0yIDMiLz48cGF0aCBkPSJNNiAxNWgxMiIvPjwvc3ZnPg==",
			Color:       "#555555",
			Tags:        []string{"push", "ios", "apns", "apple", "notifications"},
		},
		Website: "https://developer.apple.com/documentation/usernotifications",
		Version: "1.0.0",
		License: "MIT",
		Author: types.Author{
			Name:  "Lunogram",
			Email: "dev@lunogram.io",
			URL:   "https://lunogram.com",
		},
		Spec: types.ProviderSpec{
			Channels: []types.Channel{types.ChannelPush},
			Config: &types.JSONSchema{
				Type: "object",
				Properties: []types.JSONSchemaProperty{
					{
						Name: "data",
						Schema: &types.JSONSchema{
							Type: "object",
							Properties: []types.JSONSchemaProperty{
								{
									Name: "apnsTeamId",
									Schema: &types.JSONSchema{
										Type:        "string",
										Title:       "Team ID",
										Description: "Apple Developer Team ID (10 characters).",
									},
								},
								{
									Name: "apnsKeyId",
									Schema: &types.JSONSchema{
										Type:        "string",
										Title:       "Key ID",
										Description: "APNs authentication key ID (10 characters).",
									},
								},
								{
									Name: "apnsPrivateKey",
									Schema: &types.JSONSchema{
										Type:          "string",
										Title:         "Private Key (.p8)",
										Description:   ".p8 private key from Apple Developer. Upload the file or paste and encode.",
										Format:        "password",
										FileUpload:    true,
										FileAccept:    ".p8",
										RequireBase64: true,
									},
								},
								{
									Name: "apnsBundleId",
									Schema: &types.JSONSchema{
										Type:        "string",
										Title:       "Bundle ID",
										Description: "iOS app bundle ID (e.g. com.yourcompany.app).",
									},
								},
								{
									Name: "apnsProduction",
									Schema: &types.JSONSchema{
										Type:        "boolean",
										Title:       "Production Mode",
										Description: "Use production APNs server (uncheck for sandbox/development).",
									},
								},
							},
							Required: []string{"apnsTeamId", "apnsKeyId", "apnsPrivateKey", "apnsBundleId"},
						},
					},
				},
			},
		},
	}

	if err := pdk.OutputJSON(manifest); err != nil {
		pdk.SetError(err)
		return -1
	}
	return 0
}

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

type Config struct {
	TeamID     string `json:"apnsTeamId"`
	KeyID      string `json:"apnsKeyId"`
	PrivateKey string `json:"apnsPrivateKey"`
	BundleID   string `json:"apnsBundleId"`
	Production *bool  `json:"apnsProduction"`
}

type apnsPayload struct {
	APS  apnsAPS        `json:"aps"`
	Data map[string]any `json:"data,omitempty"`
}

type apnsAPS struct {
	Alert *apnsAlert `json:"alert,omitempty"`
	Badge *int       `json:"badge,omitempty"`
	Sound *string    `json:"sound,omitempty"`
}

type apnsAlert struct {
	Title string `json:"title,omitempty"`
	Body  string `json:"body,omitempty"`
}

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

//go:export send
func Send() int32 {
	pdk.Log(pdk.LogInfo, "APNs Send() called")

	var req types.SendRequest[Config]
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse request: %w", err))
		return -1
	}

	if req.Channel != types.ChannelPush {
		pdk.SetError(fmt.Errorf("unsupported channel: %s", req.Channel))
		return -1
	}

	if req.Config.TeamID == "" || req.Config.KeyID == "" ||
		req.Config.PrivateKey == "" || req.Config.BundleID == "" {
		pdk.SetError(fmt.Errorf("APNs config incomplete: teamId, keyId, privateKey, and bundleId are required"))
		return -1
	}

	push, err := req.GetPushPayload()
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse push payload: %w", err))
		return -1
	}

	if len(push.APNsTokens) == 0 {
		pdk.SetError(fmt.Errorf("no APNs tokens in payload"))
		return -1
	}

	ok, fail, errs := sendAllAPNs(req.Config, push)
	pdk.Log(pdk.LogInfo, fmt.Sprintf("APNs: %d ok, %d failed", ok, fail))

	meta := map[string]any{
		"success_count": ok,
		"failure_count": fail,
		"total_targets": len(push.APNsTokens),
	}
	if len(errs) > 0 {
		meta["errors"] = errs
	}

	status := "sent"
	if ok == 0 {
		status = "failed"
	} else if fail > 0 {
		status = "partial"
	}

	response := types.SendResponse{
		Status:   status,
		Metadata: map[string]any{"apns": meta},
	}

	if err := pdk.OutputJSON(response); err != nil {
		pdk.SetError(err)
		return -1
	}
	return 0
}

// ---------------------------------------------------------------------------
// APNs HTTP/2 — Native iOS push
// ---------------------------------------------------------------------------

func sendAllAPNs(config Config, push types.PushPayload) (ok, fail int, errs []string) {
	for i, token := range push.APNsTokens {
		if err := sendAPNsNotification(config, token, push); err != nil {
			fail++
			msg := fmt.Sprintf("APNs token %d failed: %v", i+1, err)
			errs = append(errs, msg)
			pdk.Log(pdk.LogWarn, msg)
		} else {
			ok++
		}
	}
	return
}

func sendAPNsNotification(config Config, deviceToken string, push types.PushPayload) error {
	if deviceToken == "" {
		return fmt.Errorf("empty APNs device token")
	}

	apnsJWT, err := buildAPNsJWT(config.TeamID, config.KeyID, config.PrivateKey)
	if err != nil {
		return fmt.Errorf("failed to build APNs JWT: %w", err)
	}

	alert := &apnsAlert{
		Title: push.Title,
		Body:  push.Body,
	}

	aps := apnsAPS{Alert: alert}
	if push.Badge != nil {
		aps.Badge = push.Badge
	}
	if push.Sound != nil {
		aps.Sound = push.Sound
	}

	payload := apnsPayload{APS: aps}
	if len(push.Data) > 0 {
		payload.Data = push.Data
	}

	payloadBytes, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal APNs payload: %w", err)
	}

	endpoint := "https://api.push.apple.com"
	productionMode := "nil (defaulting to production)"
	if config.Production != nil {
		if *config.Production {
			productionMode = "true (production)"
		} else {
			productionMode = "false (sandbox)"
			endpoint = "https://api.sandbox.push.apple.com"
		}
	}

	url := fmt.Sprintf("%s/3/device/%s", endpoint, deviceToken)

	resp := pdk.NewHTTPRequest(pdk.MethodPost, url).
		SetHeader("Authorization", "bearer "+apnsJWT).
		SetHeader("apns-topic", config.BundleID).
		SetHeader("apns-push-type", "alert").
		SetHeader("apns-priority", "10").
		SetHeader("Content-Type", "application/json").
		SetBody(payloadBytes).
		Send()

	responseBody := string(resp.Body())

	switch resp.Status() {
	case 200:
		return nil
	case 400:
		pdk.Log(pdk.LogWarn, fmt.Sprintf("APNs request failed - team_id=%s, key_id=%s, bundle_id=%s, production=%s, endpoint=%s, token_length=%d",
			config.TeamID, config.KeyID, config.BundleID, productionMode, endpoint, len(deviceToken)))
		pdk.Log(pdk.LogWarn, fmt.Sprintf("APNs response: %s", responseBody))
		return fmt.Errorf("bad request (400): %s", responseBody)
	case 403:
		pdk.Log(pdk.LogWarn, fmt.Sprintf("APNs request failed - team_id=%s, key_id=%s, bundle_id=%s, production=%s, endpoint=%s, token_length=%d",
			config.TeamID, config.KeyID, config.BundleID, productionMode, endpoint, len(deviceToken)))
		pdk.Log(pdk.LogWarn, fmt.Sprintf("APNs response: %s", responseBody))
		return fmt.Errorf("forbidden (403) - check certificate/bundle ID/team ID: %s", responseBody)
	case 404:
		pdk.Log(pdk.LogWarn, fmt.Sprintf("APNs response: %s", responseBody))
		return fmt.Errorf("device token invalid/expired (404): %s", responseBody)
	case 410:
		pdk.Log(pdk.LogWarn, fmt.Sprintf("APNs response: %s", responseBody))
		return fmt.Errorf("device token no longer active (410): %s", responseBody)
	case 413:
		pdk.Log(pdk.LogWarn, fmt.Sprintf("APNs response: %s", responseBody))
		return fmt.Errorf("payload too large (413): %s", responseBody)
	case 429:
		pdk.Log(pdk.LogWarn, fmt.Sprintf("APNs response: %s", responseBody))
		return fmt.Errorf("rate limited (429): %s", responseBody)
	case 500:
		pdk.Log(pdk.LogWarn, fmt.Sprintf("APNs response: %s", responseBody))
		return fmt.Errorf("APNs server error (500): %s", responseBody)
	case 503:
		pdk.Log(pdk.LogWarn, fmt.Sprintf("APNs response: %s", responseBody))
		return fmt.Errorf("APNs service unavailable (503): %s", responseBody)
	default:
		pdk.Log(pdk.LogWarn, fmt.Sprintf("APNs unexpected status - team_id=%s, key_id=%s, bundle_id=%s, production=%s, endpoint=%s",
			config.TeamID, config.KeyID, config.BundleID, productionMode, endpoint))
		pdk.Log(pdk.LogWarn, fmt.Sprintf("APNs response: %s", responseBody))
		return fmt.Errorf("unexpected status %d: %s", resp.Status(), responseBody)
	}
}

func buildAPNsJWT(teamID, keyID, privateKeyB64 string) (string, error) {
	privKeyPEM, err := decodeBase64Lenient(privateKeyB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode APNs private key: %w", err)
	}

	privKeyStr := string(privKeyPEM)
	const pemHeader = "-----BEGIN PRIVATE KEY-----"
	const pemFooter = "-----END PRIVATE KEY-----"
	start := strings.Index(privKeyStr, pemHeader)
	end := strings.Index(privKeyStr, pemFooter)

	var derBytes []byte
	if start != -1 && end != -1 {
		b64Key := strings.ReplaceAll(privKeyStr[start+len(pemHeader):end], "\n", "")
		b64Key = strings.TrimSpace(b64Key)
		derBytes, err = base64.StdEncoding.DecodeString(b64Key)
		if err != nil {
			return "", fmt.Errorf("failed to decode PEM content: %w", err)
		}
	} else {
		derBytes = privKeyPEM
	}

	key, err := x509.ParsePKCS8PrivateKey(derBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse PKCS8 key: %w", err)
	}

	ecdsaKey, ok := key.(*ecdsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("APNs key is not ECDSA (expected P-256)")
	}

	header, err := jsonBase64URL(map[string]string{
		"alg": "ES256",
		"kid": keyID,
	})
	if err != nil {
		return "", err
	}

	claims, err := jsonBase64URL(map[string]any{
		"iss": teamID,
		"iat": time.Now().Unix(),
	})
	if err != nil {
		return "", err
	}

	signingInput := header + "." + claims
	digest := sha256.Sum256([]byte(signingInput))

	r, s, err := ecdsa.Sign(rand.Reader, ecdsaKey, digest[:])
	if err != nil {
		return "", fmt.Errorf("ECDSA sign failed: %w", err)
	}

	sig := append(zeroPad(r.Bytes(), 32), zeroPad(s.Bytes(), 32)...)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// ---------------------------------------------------------------------------
// Util
// ---------------------------------------------------------------------------

func jsonBase64URL(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func zeroPad(b []byte, size int) []byte {
	if len(b) >= size {
		return b
	}
	padded := make([]byte, size)
	copy(padded[size-len(b):], b)
	return padded
}

func decodeBase64Lenient(s string) ([]byte, error) {
	if b, err := base64.StdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.RawStdEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	if b, err := base64.URLEncoding.DecodeString(s); err == nil {
		return b, nil
	}
	return base64.RawURLEncoding.DecodeString(s)
}

func main() {}
