package main

import (
	gocrypto "crypto"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	gosha256 "crypto/sha256"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"strings"
	"time"

	pdk "github.com/extism/go-pdk"
	"github.com/lunogram/platform/modules/providers/webpush/types"
)

//go:export manifest
func Manifest() int32 {
	manifest := types.ProviderManifest{
		Metadata: types.Metadata{
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
									Name: "vapidPublicKey",
									Schema: &types.JSONSchema{
										Type:        "string",
										Title:       "VAPID Public Key",
										Description: "VAPID public key (base64url). Required for Web Push.",
									},
								},
								{
									Name: "vapidPrivateKey",
									Schema: &types.JSONSchema{
										Type:        "string",
										Title:       "VAPID Private Key",
										Description: "VAPID private key (base64url). Required for Web Push.",
										Format:      "password",
									},
								},
								{
									Name: "vapidEmail",
									Schema: &types.JSONSchema{
										Type:        "string",
										Title:       "VAPID Email",
										Description: "Contact email for VAPID (e.g. mailto:admin@example.com). Required for Web Push.",
									},
								},
								{
									Name: "fcmProjectId",
									Schema: &types.JSONSchema{
										Type:        "string",
										Title:       "FCM Project ID",
										Description: "Firebase project ID. Required for FCM.",
									},
								},
								{
									Name: "fcmServiceAccountJSON",
									Schema: &types.JSONSchema{
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
	VapidPublicKey       string `json:"vapidPublicKey"`
	VapidPrivateKey      string `json:"vapidPrivateKey"`
	VapidEmail           string `json:"vapidEmail"`
	FCMProjectID         string `json:"fcmProjectId"`
	FCMServiceAccountB64 string `json:"fcmServiceAccountJSON"`
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
	pdk.Log(pdk.LogInfo, "Send() called")

	var req types.SendRequest[Config]
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse request: %w", err))
		return -1
	}

	if req.Channel != types.ChannelPush {
		pdk.SetError(fmt.Errorf("unsupported channel: %s", req.Channel))
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

	response := types.SendResponse{
		Status:   "sent",
		Metadata: map[string]any{},
	}

	if hasWebPush {
		if req.Config.VapidPublicKey == "" || req.Config.VapidPrivateKey == "" || req.Config.VapidEmail == "" {
			pdk.SetError(fmt.Errorf("WebPushTargets provided but VAPID config incomplete"))
			return -1
		}

		wpPayload, err := buildWebPushPayload(push)
		if err != nil {
			pdk.SetError(fmt.Errorf("failed to build Web Push payload: %w", err))
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
// Web Push — manual VAPID + AES-128-GCM encryption, pdk.HTTPRequest
// ---------------------------------------------------------------------------

func buildWebPushPayload(push types.PushPayload) ([]byte, error) {
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

func sendAllWebPush(config Config, targets []types.WebPushTarget, payload []byte) (ok, fail int, errs []string) {
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

func sendWebPushNotification(config Config, target types.WebPushTarget, payload []byte) error {
	if target.Endpoint == "" {
		return fmt.Errorf("missing endpoint")
	}
	if target.Keys.Auth == "" {
		return fmt.Errorf("missing auth key")
	}
	if target.Keys.P256dh == "" {
		return fmt.Errorf("missing p256dh key")
	}

	// Encrypt payload per RFC 8291 (aes128gcm)
	encrypted, _, _, err := encryptWebPushPayload(payload, target.Keys.P256dh, target.Keys.Auth)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	// Build VAPID JWT
	vapidJWT, err := buildVAPIDJWT(target.Endpoint, config.VapidPrivateKey, config.VapidEmail)
	if err != nil {
		return fmt.Errorf("VAPID JWT failed: %w", err)
	}

	// Authorization: vapid t=<jwt>, k=<pubkey>
	authHeader := "vapid t=" + vapidJWT + ", k=" + config.VapidPublicKey

	pdk.Log(pdk.LogDebug, fmt.Sprintf("endpoint: %s", target.Endpoint))
	pdk.Log(pdk.LogDebug, fmt.Sprintf("origin: %s", extractOrigin(target.Endpoint)))
	pdk.Log(pdk.LogDebug, fmt.Sprintf("auth header: %s", authHeader))

	resp := pdk.NewHTTPRequest(pdk.MethodPost, target.Endpoint).
		SetHeader("Authorization", authHeader).
		SetHeader("Content-Type", "application/octet-stream").
		SetHeader("Content-Encoding", "aes128gcm").
		SetHeader("TTL", "86400").
		SetBody(encrypted).
		Send()

	switch resp.Status() {
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
		if resp.Status() >= 500 {
			return fmt.Errorf("push service error (%d)", resp.Status())
		}
		return fmt.Errorf("unexpected status %d", resp.Status())
	}
}

// buildVAPIDJWT builds an ES256 JWT for VAPID authentication.
// VAPID spec: https://datatracker.ietf.org/doc/html/rfc8292
func buildVAPIDJWT(endpoint, vapidPrivateKeyB64, email string) (string, error) {
	privKeyBytes, err := base64.RawURLEncoding.DecodeString(vapidPrivateKeyB64)
	if err != nil {
		return "", fmt.Errorf("failed to decode VAPID private key: %w", err)
	}

	curve := elliptic.P256()
	privKey := new(ecdsa.PrivateKey)
	privKey.D = new(big.Int).SetBytes(privKeyBytes)
	privKey.PublicKey.Curve = curve
	privKey.PublicKey.X, privKey.PublicKey.Y = curve.ScalarBaseMult(privKeyBytes)

	header, err := jsonBase64URL(map[string]string{"typ": "JWT", "alg": "ES256"})
	if err != nil {
		return "", err
	}
	claims, err := jsonBase64URL(map[string]any{
		"aud": extractOrigin(endpoint),
		"exp": time.Now().Add(12 * time.Hour).Unix(),
		"sub": email,
	})
	if err != nil {
		return "", err
	}

	signingInput := header + "." + claims
	digest := sha256.Sum256([]byte(signingInput))

	r, s, err := ecdsa.Sign(rand.Reader, privKey, digest[:])
	if err != nil {
		return "", fmt.Errorf("ECDSA sign failed: %w", err)
	}

	// ES256 sig = R || S, each 32 bytes zero-padded
	sig := append(zeroPad(r.Bytes(), 32), zeroPad(s.Bytes(), 32)...)
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// encryptWebPushPayload encrypts the payload per RFC 8291 (aes128gcm).
// Returns: encrypted body, sender public key (uncompressed), salt, error.
func encryptWebPushPayload(payload []byte, p256dhB64, authB64 string) ([]byte, []byte, []byte, error) {
	p256dh, err := decodeBase64Lenient(p256dhB64)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode p256dh: %w", err)
	}
	auth, err := decodeBase64Lenient(authB64)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to decode auth: %w", err)
	}

	if len(p256dh) != 65 || p256dh[0] != 0x04 {
		return nil, nil, nil, fmt.Errorf("invalid p256dh: expected 65-byte uncompressed point")
	}

	curve := elliptic.P256()
	receiverX := new(big.Int).SetBytes(p256dh[1:33])
	receiverY := new(big.Int).SetBytes(p256dh[33:65])

	// Ephemeral sender key pair
	senderPriv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}
	senderPubKey := elliptic.Marshal(curve, senderPriv.PublicKey.X, senderPriv.PublicKey.Y)

	// ECDH
	sharedX, _ := curve.ScalarMult(receiverX, receiverY, senderPriv.D.Bytes())
	sharedSecret := zeroPad(sharedX.Bytes(), 32)

	// Random salt
	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	// PRK = HMAC-SHA256(auth, sharedSecret)
	prk := hmacSHA256(auth, sharedSecret)

	// IKM = HKDF-Expand(prk, "WebPush: info\x00" || receiverPub || senderPub, 32)
	keyInfo := append([]byte("WebPush: info\x00"), p256dh...)
	keyInfo = append(keyInfo, senderPubKey...)
	ikm := hkdfExpand(prk, keyInfo, 32)

	// Second-stage PRK with salt
	prk2 := hmacSHA256(salt, ikm)

	// CEK = HKDF-Expand(prk2, "Content-Encoding: aes128gcm\x00", 16)
	cek := hkdfExpand(prk2, []byte("Content-Encoding: aes128gcm\x00"), 16)

	// Nonce = HKDF-Expand(prk2, "Content-Encoding: nonce\x00", 12)
	nonce := hkdfExpand(prk2, []byte("Content-Encoding: nonce\x00"), 12)

	// AES-128-GCM encrypt — payload || 0x02 delimiter
	padded := append(payload, 0x02)
	ciphertext, err := aesGCMEncrypt(cek, nonce, padded)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("AES-GCM failed: %w", err)
	}

	// RFC 8188 content header: salt(16) || rs(4) || idlen(1) || keyid || ciphertext
	rs := uint32(4096)
	header := make([]byte, 21+len(senderPubKey))
	copy(header[0:16], salt)
	header[16] = byte(rs >> 24)
	header[17] = byte(rs >> 16)
	header[18] = byte(rs >> 8)
	header[19] = byte(rs)
	header[20] = byte(len(senderPubKey))
	copy(header[21:], senderPubKey)

	return append(header, ciphertext...), senderPubKey, salt, nil
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

	// Strip PEM armor manually — encoding/pem works in TinyGo but let's keep it simple
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
// Crypto primitives — stdlib only, TinyGo-safe
// ---------------------------------------------------------------------------

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(gosha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}

// hkdfExpand is RFC 5869 HKDF-Expand — inline so we don't need golang.org/x/crypto/hkdf
func hkdfExpand(prk, info []byte, length int) []byte {
	var okm, prev []byte
	for i := 1; len(okm) < length; i++ {
		data := append(prev, info...)
		data = append(data, byte(i))
		prev = hmacSHA256(prk, data)
		okm = append(okm, prev...)
	}
	return okm[:length]
}

func aesGCMEncrypt(key, nonce, plaintext []byte) ([]byte, error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	gcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	return gcm.Seal(nil, nonce, plaintext, nil), nil
}

// ---------------------------------------------------------------------------
// Util
// ---------------------------------------------------------------------------

func extractOrigin(endpoint string) string {
	// "https://fcm.googleapis.com/fcm/send/abc" -> "https://fcm.googleapis.com"
	for i := 8; i < len(endpoint); i++ {
		if endpoint[i] == '/' {
			return endpoint[:i]
		}
	}
	return endpoint
}

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

// decodeBase64Lenient tries padded then unpadded base64 decoding.
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
