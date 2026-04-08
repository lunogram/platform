package main

import (
	gocrypto "crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	pdk "github.com/extism/go-pdk"
	"github.com/lunogram/platform/modules/providers/fcm/types"
)

//go:export manifest
func Manifest() int32 {
	manifest := types.ProviderManifest{
		Metadata: types.Metadata{
			ID:          "fcm",
			Title:       "FCM (Firebase Cloud Messaging)",
			Description: "Send push notifications to Android devices via Firebase Cloud Messaging (FCM v1 API)",
			Icon:        "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0ibm9uZSIgc3Ryb2tlPSJjdXJyZW50Q29sb3IiIHN0cm9rZS13aWR0aD0iMiIgc3Ryb2tlLWxpbmVjYXA9InJvdW5kIiBzdHJva2UtbGluZWpvaW49InJvdW5kIj48cGF0aCBkPSJNMTggOGE2IDYgMCAwIDAtMTItMCIvPjxwYXRoIGQ9Ik0yIDhoMTYiLz48cGF0aCBkPSJNNiAxNWwtMi0zaDE2bC0yIDMiLz48cGF0aCBkPSJNNiAxNWgxMiIvPjwvc3ZnPg==",
			Color:       "#FF6F00",
			Tags:        []string{"push", "android", "fcm", "firebase", "notifications"},
		},
		Website: "https://firebase.google.com/docs/cloud-messaging",
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
									Name: "fcmProjectId",
									Schema: &types.JSONSchema{
										Type:        "string",
										Title:       "Project ID",
										Description: "Firebase project ID.",
									},
								},
								{
									Name: "fcmServiceAccountJSON",
									Schema: &types.JSONSchema{
										Type:          "string",
										Title:         "Service Account JSON",
										Description:   "Firebase service account JSON. Upload the file or paste and encode.",
										Format:        "password",
										FileUpload:    true,
										FileAccept:    ".json",
										RequireBase64: true,
									},
								},
							},
							Required: []string{"fcmProjectId", "fcmServiceAccountJSON"},
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
	ProjectID         string `json:"fcmProjectId"`
	ServiceAccountB64 string `json:"fcmServiceAccountJSON"`
}

type serviceAccount struct {
	ClientEmail string `json:"client_email"`
	PrivateKey  string `json:"private_key"`
	TokenURI    string `json:"token_uri"`
}

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

// ---------------------------------------------------------------------------
// Entry point
// ---------------------------------------------------------------------------

//go:export send
func Send() int32 {
	pdk.Log(pdk.LogInfo, "FCM Send() called")

	var req types.SendRequest[Config]
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse request: %w", err))
		return -1
	}

	if req.Channel != types.ChannelPush {
		pdk.SetError(fmt.Errorf("unsupported channel: %s", req.Channel))
		return -1
	}

	if req.Config.ProjectID == "" {
		pdk.SetError(fmt.Errorf("fcmProjectId is required"))
		return -1
	}
	if req.Config.ServiceAccountB64 == "" {
		pdk.SetError(fmt.Errorf("fcmServiceAccountJSON is required"))
		return -1
	}

	push, err := req.GetPushPayload()
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse push payload: %w", err))
		return -1
	}

	if len(push.Tokens) == 0 {
		pdk.SetError(fmt.Errorf("no FCM tokens in payload"))
		return -1
	}

	accessToken, err := fetchFCMAccessToken(req.Config.ServiceAccountB64)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to get FCM access token: %w", err))
		return -1
	}

	ok, fail, errs := sendAllFCM(accessToken, req.Config.ProjectID, push)
	pdk.Log(pdk.LogInfo, fmt.Sprintf("FCM: %d ok, %d failed", ok, fail))

	meta := map[string]any{
		"success_count": ok,
		"failure_count": fail,
		"total_targets": len(push.Tokens),
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
		Metadata: map[string]any{"fcm": meta},
	}

	if err := pdk.OutputJSON(response); err != nil {
		pdk.SetError(err)
		return -1
	}
	return 0
}

// ---------------------------------------------------------------------------
// FCM HTTP v1 — pdk.HTTPRequest, manual JWT, no net/http or oauth2
// ---------------------------------------------------------------------------

func fetchFCMAccessToken(serviceAccountB64 string) (string, error) {
	saJSON, err := decodeBase64Lenient(serviceAccountB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode service account: %w", err)
	}

	var sa serviceAccount
	if err := json.Unmarshal(saJSON, &sa); err != nil {
		return "", fmt.Errorf("failed to parse service account JSON: %w", err)
	}

	jwt, err := buildServiceAccountJWT(sa)
	if err != nil {
		return "", fmt.Errorf("failed to build JWT: %w", err)
	}

	return exchangeJWTForToken(sa.TokenURI, jwt)
}

func buildServiceAccountJWT(sa serviceAccount) (string, error) {
	now := time.Now().Unix()

	header, err := jsonBase64URL(map[string]string{"alg": "RS256", "typ": "JWT"})
	if err != nil {
		return "", err
	}
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

	const pemHeader = "-----BEGIN PRIVATE KEY-----"
	const pemFooter = "-----END PRIVATE KEY-----"
	start := strings.Index(sa.PrivateKey, pemHeader)
	end := strings.Index(sa.PrivateKey, pemFooter)
	if start == -1 || end == -1 {
		return "", fmt.Errorf("invalid PEM private key")
	}
	b64Key := strings.ReplaceAll(sa.PrivateKey[start+len(pemHeader):end], "\n", "")
	b64Key = strings.TrimSpace(b64Key)

	derBytes, err := base64.StdEncoding.DecodeString(b64Key)
	if err != nil {
		return "", fmt.Errorf("failed to decode private key body: %w", err)
	}

	key, err := x509.ParsePKCS8PrivateKey(derBytes)
	if err != nil {
		return "", fmt.Errorf("failed to parse PKCS8 key: %w", err)
	}
	rsaKey, ok := key.(*rsa.PrivateKey)
	if !ok {
		return "", fmt.Errorf("service account key is not RSA")
	}

	digest := sha256.Sum256([]byte(signingInput))
	sig, err := rsa.SignPKCS1v15(rand.Reader, rsaKey, gocrypto.SHA256, digest[:])
	if err != nil {
		return "", fmt.Errorf("RSA sign failed: %w", err)
	}

	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

func exchangeJWTForToken(tokenURI, jwt string) (string, error) {
	body := []byte("grant_type=urn%3Aietf%3Aparams%3Aoauth%3Agrant-type%3Ajwt-bearer&assertion=" + jwt)

	resp := pdk.NewHTTPRequest(pdk.MethodPost, tokenURI).
		SetHeader("Content-Type", "application/x-www-form-urlencoded").
		SetBody(body).
		Send()

	if resp.Status() != 200 {
		return "", fmt.Errorf("token endpoint returned %d: %s", resp.Status(), string(resp.Body()))
	}

	var result struct {
		AccessToken string `json:"access_token"`
	}
	if err := json.Unmarshal(resp.Body(), &result); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}
	if result.AccessToken == "" {
		return "", fmt.Errorf("empty access_token in response")
	}
	return result.AccessToken, nil
}

func sendAllFCM(accessToken, projectID string, push types.PushPayload) (ok, fail int, errs []string) {
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

func sendFCMNotification(accessToken, projectID, token string, push types.PushPayload) error {
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
	if len(push.Data) > 0 {
		stringData := make(map[string]string, len(push.Data))
		for k, v := range push.Data {
			stringData[k] = fmt.Sprintf("%v", v)
		}
		msg.Data = stringData
	}
	if push.Sound != nil {
		msg.Android = &fcmAndroidConfig{Notification: &fcmAndroidNotification{Sound: *push.Sound}}
		msg.APNS = &fcmAPNSConfig{Payload: &fcmAPNSPayload{APS: &fcmAPS{Sound: *push.Sound}}}
	}

	body, err := json.Marshal(fcmMessage{Message: msg})
	if err != nil {
		return fmt.Errorf("failed to marshal FCM message: %w", err)
	}

	url := fmt.Sprintf("https://fcm.googleapis.com/v1/projects/%s/messages:send", projectID)
	resp := pdk.NewHTTPRequest(pdk.MethodPost, url).
		SetHeader("Content-Type", "application/json").
		SetHeader("Authorization", "Bearer "+accessToken).
		SetBody(body).
		Send()

	if resp.Status() == 200 {
		return nil
	}
	switch resp.Status() {
	case 400:
		return fmt.Errorf("invalid request (400): %s", string(resp.Body()))
	case 401:
		return fmt.Errorf("unauthorized - check service account (401)")
	case 403:
		return fmt.Errorf("forbidden - FCM API not enabled or wrong project (403)")
	case 404:
		return fmt.Errorf("token invalid/expired (404)")
	case 429:
		return fmt.Errorf("rate limited (429)")
	default:
		if resp.Status() >= 500 {
			return fmt.Errorf("FCM server error (%d)", resp.Status())
		}
		return fmt.Errorf("unexpected status %d", resp.Status())
	}
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
