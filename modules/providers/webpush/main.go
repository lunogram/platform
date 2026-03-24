package main

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/SherClockHolmes/webpush-go"
	"github.com/extism/go-pdk"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
)

// safeTransport wraps HTTP transport to ensure resp.Body is never nil
type safeTransport struct {
	inner http.RoundTripper
}

func (t *safeTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp, err := t.inner.RoundTrip(req)
	if err != nil {
		return nil, err
	}
	if resp.Body == nil {
		resp.Body = io.NopCloser(bytes.NewReader(nil))
	}
	return resp, nil
}

//go:export manifest
func Manifest() int32 {
	manifest := providers.ProviderManifest{
		Metadata: modules.Metadata{
			ID:          "webpush",
			Title:       "Web Push",
			Description: "Send push notifications via Web Push Protocol and/or FCM",
			Icon:        "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgc3Ryb2tlPSJjdXJyZW50Q29sb3IiIHN0cm9rZS13aWR0aD0iMiIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIiBzdHJva2UtbGluZWpvaW49InJvdW5kIj48cGF0aCBkPSJNMTggOGE2IDYgMCAwIDAtMTItMCIvPjxwYXRoIGQ9Ik0yIDhoMTYiLz48cGF0aCBkPSJNNiAxNWwtMi0zaDE2bC0yIDMiLz48cGF0aCBkPSJNNiAxNWgxMiIvPjwvc3ZnPg==",
			Color:       "#5A67D8",
			Tags:        []string{"push", "web", "browser", "notifications", "fcm", "firebase"},
		},
		Website: "https://developer.mozilla.org/en-US/docs/Web/API/Push_API",
		Version: "1.1.0",
		License: "MIT",
		Author: modules.Author{
			Name:  "Lunogram",
			Email: "dev@lunogram.io",
			URL:   "https://lunogram.com",
		},
		Spec: providers.ProviderSpec{
			Channels: []providers.Channel{providers.ChannelPush},
			Config: &modules.JSONSchema{
				Type: "object",
				Properties: []modules.JSONSchemaProperty{
					{
						Name: "data",
						Schema: &modules.JSONSchema{
							Type: "object",
							Properties: []modules.JSONSchemaProperty{
								{
									Name: "vapidPublicKey",
									Schema: &modules.JSONSchema{
										Type:        "string",
										Title:       "VAPID Public Key",
										Description: "Your VAPID public key (base64url encoded). Required for Web Push.",
									},
								},
								{
									Name: "vapidPrivateKey",
									Schema: &modules.JSONSchema{
										Type:        "string",
										Title:       "VAPID Private Key",
										Description: "Your VAPID private key (base64url encoded). Required for Web Push.",
										Format:      "password",
									},
								},
								{
									Name: "vapidEmail",
									Schema: &modules.JSONSchema{
										Type:        "string",
										Title:       "VAPID Email",
										Description: "Contact email for VAPID (e.g., mailto:admin@example.com). Required for Web Push.",
									},
								},
								{
									Name: "fcmProjectId",
									Schema: &modules.JSONSchema{
										Type:        "string",
										Title:       "FCM Project ID",
										Description: "Your Firebase project ID. Required for FCM.",
									},
								},
								{
									Name: "fcmServiceAccountJSON",
									Schema: &modules.JSONSchema{
										Type:        "string",
										Title:       "FCM Service Account JSON (base64)",
										Description: "Base64-encoded Firebase service account JSON. Required for FCM.",
										Format:      "password",
									},
								},
							},
							Required: []string{},
						},
					},
				},
			},
		},
	}

	err := pdk.OutputJSON(manifest)
	if err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

type Config struct {
	// Web Push / VAPID
	VapidPublicKey  string `json:"vapidPublicKey"`
	VapidPrivateKey string `json:"vapidPrivateKey"`
	VapidEmail      string `json:"vapidEmail"`

	// FCM HTTP v1
	FCMProjectID         string `json:"fcmProjectId"`
	FCMServiceAccountB64 string `json:"fcmServiceAccountJSON"`
}

// serviceAccount is the subset of fields we need from the service account JSON.
// Google hands you a 20-field JSON file; we only care about three of them.
type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

// fcmMessage is the FCM HTTP v1 request body.
// Docs: https://firebase.google.com/docs/reference/fcm/rest/v1/projects.messages/send
type fcmMessage struct {
	Message fcmMessageBody `json:"message"`
}

type fcmMessageBody struct {
	Token        string            `json:"token"`
	Notification *fcmNotification  `json:"notification,omitempty"`
	Data         map[string]string `json:"data,omitempty"`
	Android      *fcmAndroidConfig `json:"android,omitempty"`
	APNS         *fcmAPNSConfig    `json:"apns,omitempty"`
}

type fcmNotification struct {
	Title    string `json:"title,omitempty"`
	Body     string `json:"body,omitempty"`
	ImageURL string `json:"image,omitempty"`
}

type fcmAndroidConfig struct {
	Notification *fcmAndroidNotification `json:"notification,omitempty"`
}

type fcmAndroidNotification struct {
	Sound string `json:"sound,omitempty"`
}

type fcmAPNSConfig struct {
	Payload *fcmAPNSPayload `json:"payload,omitempty"`
}

type fcmAPNSPayload struct {
	APS *fcmAPS `json:"aps,omitempty"`
}

type fcmAPS struct {
	Sound string `json:"sound,omitempty"`
}

//go:export send
func Send() int32 {
	pdk.Log(pdk.LogInfo, "Send() called") // does this appear?``
	var req providers.SendRequest[Config]
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse request: %w", err))
		return -1
	}

	if req.Channel != providers.ChannelPush {
		pdk.SetError(fmt.Errorf("unsupported channel: %s (expected 'push')", req.Channel))
		return -1
	}

	push, err := req.GetPushPayload()
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse push payload: %w", err))
		return -1
	}

	hasWebPush := len(push.WebPushTargets) > 0
	hasFCM := len(push.Tokens) > 0

	if !hasWebPush && !hasFCM {
		pdk.SetError(fmt.Errorf("no targets: need WebPushTargets and/or FCM tokens"))
		return -1
	}

	response := providers.SendResponse{
		Status:   "sent",
		Metadata: map[string]any{},
	}

	// -------------------------------------------------------------------------
	// Web Push leg
	// -------------------------------------------------------------------------
	if hasWebPush {
		if req.Config.VapidPublicKey == "" || req.Config.VapidPrivateKey == "" || req.Config.VapidEmail == "" {
			pdk.SetError(fmt.Errorf("WebPushTargets provided but VAPID config incomplete"))
			return -1
		}

		wpPayload, err := buildWebPushPayload(push)
		if err != nil {
			pdk.SetError(fmt.Errorf("failed to marshal Web Push payload: %w", err))
			return -1
		}

		wpOk, wpFail, wpErrs := sendAllWebPush(req.Config, push.WebPushTargets, wpPayload)
		pdk.Log(pdk.LogInfo, fmt.Sprintf("Web Push: %d ok, %d failed", wpOk, wpFail))

		wpMeta := map[string]any{
			"success_count": wpOk,
			"failure_count": wpFail,
			"total_targets": len(push.WebPushTargets),
		}
		if len(wpErrs) > 0 {
			wpMeta["errors"] = wpErrs
		}
		response.Metadata["webpush"] = wpMeta
		response.Metadata["webpush_status"] = legStatus(wpOk, wpFail)
	}

	// -------------------------------------------------------------------------
	// FCM HTTP v1 leg
	// -------------------------------------------------------------------------
	if hasFCM {
		if req.Config.FCMProjectID == "" {
			pdk.SetError(fmt.Errorf("FCM tokens provided but fcmProjectId missing"))
			return -1
		}
		if req.Config.FCMServiceAccountB64 == "" {
			pdk.SetError(fmt.Errorf("FCM tokens provided but fcmServiceAccountJSON missing"))
			return -1
		}

		accessToken, err := fetchFCMAccessToken(req.Config.FCMServiceAccountB64)
		if err != nil {
			pdk.SetError(fmt.Errorf("failed to get FCM access token: %w", err))
			return -1
		}

		fcmOk, fcmFail, fcmErrs := sendAllFCM(accessToken, req.Config.FCMProjectID, push)
		pdk.Log(pdk.LogInfo, fmt.Sprintf("FCM: %d ok, %d failed", fcmOk, fcmFail))

		fcmMeta := map[string]any{
			"success_count": fcmOk,
			"failure_count": fcmFail,
			"total_targets": len(push.Tokens),
		}
		if len(fcmErrs) > 0 {
			fcmMeta["errors"] = fcmErrs
		}
		response.Metadata["fcm"] = fcmMeta
		response.Metadata["fcm_status"] = legStatus(fcmOk, fcmFail)
	}

	response.Status = rollUpStatus(response.Metadata, hasWebPush, hasFCM)

	if err := pdk.OutputJSON(response); err != nil {
		pdk.SetError(err)
		return -1
	}

	return 0
}

func legStatus(success, failure int) string {
	if success == 0 {
		return "failed"
	}
	if failure > 0 {
		return "partial"
	}
	return "sent"
}

func rollUpStatus(meta map[string]any, hadWP, hadFCM bool) string {
	allFailed := true
	anyPartial := false

	check := func(key string) {
		v, ok := meta[key]
		if !ok {
			return
		}
		s, _ := v.(string)
		if s != "failed" {
			allFailed = false
		}
		if s == "partial" {
			anyPartial = true
		}
	}

	if hadWP {
		check("webpush_status")
	}
	if hadFCM {
		check("fcm_status")
	}

	if allFailed {
		return "failed"
	}
	if anyPartial {
		return "partial"
	}
	return "sent"
}

// ---------------------------------------------------------------------------
// Web Push helpers
// ---------------------------------------------------------------------------

func buildWebPushPayload(push *providers.PushPayload) ([]byte, error) {
	n := map[string]any{"title": push.Title, "body": push.Body}
	if len(push.Data) > 0 {
		n["data"] = push.Data
	}
	if push.ImageURL != nil {
		n["image"] = *push.ImageURL
	}
	if push.Badge != nil {
		n["badge"] = *push.Badge
	}
	if push.Sound != nil {
		n["sound"] = *push.Sound
	}
	return json.Marshal(n)
}

func sendAllWebPush(config Config, targets []providers.WebPushTarget, payload []byte) (ok, fail int, errs []string) {
	for i, target := range targets {
		if err := sendWebPushNotification(config, target, payload); err != nil {
			fail++
			msg := fmt.Sprintf("subscription %d failed: %v", i+1, err)
			errs = append(errs, msg)
			pdk.Log(pdk.LogWarn, msg)
		} else {
			ok++
		}
	}
	return
}

func sendWebPushNotification(config Config, target providers.WebPushTarget, payload []byte) error {
	if target.Endpoint == "" {
		return fmt.Errorf("missing endpoint")
	}
	if target.Keys.Auth == "" {
		return fmt.Errorf("missing auth key")
	}
	if target.Keys.P256dh == "" {
		return fmt.Errorf("missing p256dh key")
	}

	resp, err := webpush.SendNotification(payload, &webpush.Subscription{
		Endpoint: target.Endpoint,
		Keys:     webpush.Keys{Auth: target.Keys.Auth, P256dh: target.Keys.P256dh},
	}, &webpush.Options{
		Subscriber:      config.VapidEmail,
		VAPIDPublicKey:  config.VapidPublicKey,
		VAPIDPrivateKey: config.VapidPrivateKey,
		TTL:             86400,
	})
	if err != nil {
		return fmt.Errorf("send failed: %w", err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case 201:
		return nil
	case 400:
		return fmt.Errorf("invalid request (400)")
	case 401:
		return fmt.Errorf("unauthorized - check VAPID keys (401)")
	case 404:
		return fmt.Errorf("subscription not found (404)")
	case 410:
		return fmt.Errorf("subscription expired (410) - remove from DB")
	case 413:
		return fmt.Errorf("payload too large (413)")
	case 429:
		return fmt.Errorf("rate limited (429)")
	default:
		if resp.StatusCode >= 500 {
			return fmt.Errorf("push service error (%d)", resp.StatusCode)
		}
		return fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
}

// ---------------------------------------------------------------------------
// FCM HTTP v1 helpers — zero dependency OAuth2, TinyGo-safe
// ---------------------------------------------------------------------------

// fetchFCMAccessToken does the full service-account → JWT → access token dance
// without touching golang.org/x/oauth2, which TinyGo cannot handle.
func fetchFCMAccessToken(serviceAccountB64 string) (string, error) {
	// 1. Decode base64 — try padded first, fall back to unpadded
	saJSON, err := base64.StdEncoding.DecodeString(serviceAccountB64)
	if err != nil {
		saJSON, err = base64.RawStdEncoding.DecodeString(serviceAccountB64)
		if err != nil {
			return "", fmt.Errorf("failed to base64-decode service account: %w", err)
		}
	}

	// 2. Parse only the fields we need
	var sa serviceAccount
	if err := json.Unmarshal(saJSON, &sa); err != nil {
		return "", fmt.Errorf("failed to parse service account JSON: %w", err)
	}

	// 3. Build + sign the JWT
	jwt, err := buildServiceAccountJWT(sa)
	if err != nil {
		return "", fmt.Errorf("failed to build JWT: %w", err)
	}

	// 4. Exchange JWT for an access token
	return exchangeJWTForToken(sa.TokenURI, jwt)
}

// buildServiceAccountJWT constructs and RS256-signs a JWT for the Google
// OAuth2 token endpoint. No external deps — just stdlib crypto.
func buildServiceAccountJWT(sa serviceAccount) (string, error) {
	now := time.Now().Unix()

	// Header
	header, err := jsonBase64URL(map[string]string{
		"alg": "RS256",
		"typ": "JWT",
	})
	if err != nil {
		return "", err
	}

	// Claims
	claims, err := jsonBase64URL(map[string]any{
		"iss":   sa.ClientEmail,
		"scope": "https://www.googleapis.com/auth/firebase.messaging",
		"aud":   sa.TokenURI,
		"iat":   now,
		"exp":   now + 3600,
	})
	if err != nil {
		return "", err
	}

	signingInput := header + "." + claims

	// Parse PEM private key
	block, _ := pem.Decode([]byte(sa.PrivateKey))
	if block == nil {
		return "", fmt.Errorf("failed to decode PEM block from private key")
	}

	privateKey, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse private key: %w", err)
	}

	rsaKey, ok := privateKey.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("private key is not RSA")
	}

	// Sign
	h := sha256.New()
	h.Write([]byte(signingInput))
	digest := h.Sum(nil)

	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, crypto.SHA256, digest)
	if err != nil {
		return "", fmt.Errorf("failed to sign JWT: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// exchangeJWTForToken POSTs the signed JWT to Google's token endpoint and
// returns the access_token string.
func exchangeJWTForToken(tokenURI, jwt string) (string, error) {
	body := "grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Ajwt-bearer&assertion=" + jwt

	resp, err := http.Post(tokenURI, "application/x-www-form-urlencoded", strings.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("token request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 256))
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.StatusCode, errBody)
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("token response had empty access_token")
	}

	return result.AccessToken, nil
}

// jsonBase64URL marshals v to JSON then base64url-encodes it (no padding).
func jsonBase64URL(v any) (string, error) {
	b, err := json.Marshal(v)
	if err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b), nil
}

func sendAllFCM(accessToken, projectID string, push *providers.PushPayload) (ok, fail int, errs []string) {
	for i, token := range push.Tokens {
		if err := sendFCMNotification(accessToken, projectID, token, push); err != nil {
			fail++
			msg := fmt.Sprintf("token %d failed: %v", i+1, err)
			errs = append(errs, msg)
			pdk.Log(pdk.LogWarn, msg)
		} else {
			ok++
		}
	}
	return
}

func sendFCMNotification(accessToken, projectID, token string, push *providers.PushPayload) error {
	if token == "" {
		return fmt.Errorf("empty FCM token")
	}

	msg := fcmMessageBody{
		Token:        token,
		Notification: &fcmNotification{Title: push.Title, Body: push.Body},
	}

	if push.ImageURL != nil {
		msg.Notification.ImageURL = *push.ImageURL
	}

	// FCM data values must ALL be strings — it throws a fit otherwise
	if len(push.Data) > 0 {
		stringData := make(map[string]string, len(push.Data))
		for k, v := range push.Data {
			stringData[k] = fmt.Sprintf("%v", v)
		}
		msg.Data = stringData
	}

	// Sound needs per-platform config because FCM is allergic to simplicity
	if push.Sound != nil {
		msg.Android = &fcmAndroidConfig{Notification: &fcmAndroidNotification{Sound: *push.Sound}}
		msg.APNS = &fcmAPNSConfig{Payload: &fcmAPNSPayload{APS: &fcmAPS{Sound: *push.Sound}}}
	}

	body, err := json.Marshal(fcmMessage{Message: msg})
	if err != nil {
		return fmt.Errorf("failed to marshal FCM message: %w", err)
	}

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", projectID)
	httpReq, err := http.NewRequest(http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to build request: %w", err)
	}
	httpReq.Header.Set("Content-Type", "application/json")
	httpReq.Header.Set("Authorization", "Bearer "+accessToken)

	resp, err := http.DefaultClient.Do(httpReq)
	if err != nil {
		return fmt.Errorf("FCM request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 200 {
		return nil
	}

	errBody, _ := io.ReadAll(io.LimitReader(resp.Body, 512))

	switch resp.StatusCode {
	case 400:
		return fmt.Errorf("invalid request (400): %s", errBody)
	case 401:
		return fmt.Errorf("unauthorized - check service account permissions (401)")
	case 403:
		return fmt.Errorf("forbidden - FCM API not enabled or wrong project (403)")
	case 404:
		return fmt.Errorf("app instance not found - token may be invalid/expired (404)")
	case 429:
		return fmt.Errorf("rate limited (429)")
	default:
		if resp.StatusCode >= 500 {
			return fmt.Errorf("FCM server error (%d): %s", resp.StatusCode, errBody)
		}
		return fmt.Errorf("unexpected status %d: %s", resp.StatusCode, errBody)
	}
}

func main() {}
