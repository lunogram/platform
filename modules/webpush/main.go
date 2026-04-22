package main

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"math/big"
	"time"

	pdk "github.com/extism/go-pdk"
	"github.com/lunogram/platform/pkg/modules"
	"github.com/lunogram/platform/pkg/modules/providers"
)

const (
	ExitSuccess   int32 = 0
	ExitTransient int32 = -1
	ExitPermanent int32 = -2

	sendStatusSent    = "sent"
	sendStatusFailed  = "failed"
	sendStatusPartial = "partial"
	ttlSeconds        = "86400"
)

//go:wasmexport manifest
func Manifest() int32 {
	manifest := providers.ProviderManifest{
		Metadata: modules.Metadata{
			ID:          "webpush",
			Title:       "Web Push",
			Description: "Send push notifications to browsers via the Web Push Protocol (VAPID)",
			Icon:        "data:image/svg+xml;base64,PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciIHZpZXdCb3g9IjAgMCAyNCAyNCIgZmlsbD0iIzI1NjNFQiI+PHBhdGggZD0iTTEyIDJDNi40OCAyIDIgNi40OCAyIDEyczQuNDggMTAgMTAgMTAgMTAtNC40OCAxMC0xMFMxNy41MiAyIDEyIDJ6bS0xIDE3LjkzYy0zLjk1LS40OS03LTMuODUtNy03LjkzIDAtLjYyLjA4LTEuMjEuMjEtMS43OUw5IDE1djFjMCAxLjEuOSAyIDIgMnYxLjkzem02LjktMi41NGMtLjI2LS44MS0xLTEuMzktMS45LTEuMzloLTF2LTNjMC0uNTUtLjQ1LTEtMS0xSDh2LTJoMmMuNTUgMCAxLS40NSAxLTFWN2gyYzEuMSAwIDItLjkgMi0ydi0uNDFjMi45MyAxLjE5IDUgNC4wNiA1IDcuNDEgMCAyLjA4LS44IDMuOTctMi4xIDUuMzl6Ii8+PC9zdmc+",
			Color:       "#2563EB",
			Tags:        []string{"push", "web", "browser", "notifications", "vapid"},
		},
		Website: "https://developer.mozilla.org/en-US/docs/Web/API/Push_API",
		Version: "2.0.0",
		License: "MIT",
		Author: modules.Author{
			Name:  "Lunogram",
			Email: "dev@lunogram.io",
			URL:   "https://lunogram.com",
		},
		Spec: providers.ProviderSpec{
			Channels: []providers.Channel{providers.ChannelPush},
			Platforms: []providers.Platform{
				providers.PlatformWeb,
			},
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
										Type:   "string",
										Hidden: true,
									},
									Hidden: true,
								},
								{
									Name: "vapidPrivateKey",
									Schema: &modules.JSONSchema{
										Type:   "string",
										Hidden: true,
									},
									Hidden: true,
								},
								{
									Name: "vapidEmail",
									Schema: &modules.JSONSchema{
										Type:        "string",
										Title:       "VAPID Email",
										Description: "Contact email for VAPID (e.g. mailto:admin@example.com). Required for Web Push.",
									},
								},
							},
							Required: []string{"vapidPublicKey", "vapidPrivateKey", "vapidEmail"},
						},
					},
				},
			},
		},
	}

	if err := pdk.OutputJSON(manifest); err != nil {
		pdk.SetError(err)
		return ExitTransient
	}

	return ExitSuccess
}

type Config struct {
	VapidPublicKey  string `json:"vapidPublicKey"`
	VapidPrivateKey string `json:"vapidPrivateKey"`
	VapidEmail      string `json:"vapidEmail"`
}

//go:wasmexport send
func Send() int32 {
	var req providers.SendRequest[Config]
	if err := pdk.InputJSON(&req); err != nil {
		pdk.SetError(fmt.Errorf("failed to parse request: %w", err))
		return ExitPermanent
	}

	if req.Channel != providers.ChannelPush {
		pdk.SetError(fmt.Errorf("unsupported channel: %s", req.Channel))
		return ExitPermanent
	}

	if req.Config.VapidPublicKey == "" || req.Config.VapidPrivateKey == "" || req.Config.VapidEmail == "" {
		pdk.SetError(fmt.Errorf("VAPID config incomplete: vapidPublicKey, vapidPrivateKey, and vapidEmail are required"))
		return ExitPermanent
	}

	push, err := req.GetPushPayload()
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to parse push payload: %w", err))
		return ExitPermanent
	}

	if len(push.WebPushTargets) == 0 {
		pdk.SetError(fmt.Errorf("no WebPushTargets in payload"))
		return ExitPermanent
	}

	wpPayload, err := buildWebPushPayload(push)
	if err != nil {
		pdk.SetError(fmt.Errorf("failed to build Web Push payload: %w", err))
		return ExitPermanent
	}

	ok, fail, errs := sendAllWebPush(req.Config, push.WebPushTargets, wpPayload)
	response := buildSendResponse("webpush", ok, fail, len(push.WebPushTargets), errs)

	if err := pdk.OutputJSON(response); err != nil {
		pdk.SetError(err)
		return ExitTransient
	}

	return ExitSuccess
}

func buildWebPushPayload(push providers.PushPayload) ([]byte, error) {
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

	encrypted, _, _, err := encryptWebPushPayload(payload, target.Keys.P256dh, target.Keys.Auth)
	if err != nil {
		return fmt.Errorf("encryption failed: %w", err)
	}

	vapidJWT, err := buildVAPIDJWT(target.Endpoint, config.VapidPrivateKey, config.VapidEmail)
	if err != nil {
		return fmt.Errorf("VAPID JWT failed: %w", err)
	}

	authHeader := "vapid t=" + vapidJWT + ", k=" + config.VapidPublicKey

	pdk.Log(pdk.LogDebug, fmt.Sprintf("endpoint: %s", target.Endpoint))
	pdk.Log(pdk.LogDebug, fmt.Sprintf("origin: %s", extractOrigin(target.Endpoint)))
	pdk.Log(pdk.LogDebug, fmt.Sprintf("auth header: %s", authHeader))

	resp := pdk.NewHTTPRequest(pdk.MethodPost, target.Endpoint).
		SetHeader("Authorization", authHeader).
		SetHeader("Content-Type", "application/octet-stream").
		SetHeader("Content-Encoding", "aes128gcm").
		SetHeader("TTL", ttlSeconds).
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

	sig, err := signES256P256(privKey, digest[:])
	if err != nil {
		return "", fmt.Errorf("ECDSA sign failed: %w", err)
	}
	return signingInput + "." + base64.RawURLEncoding.EncodeToString(sig), nil
}

// encryptWebPushPayload encrypts the payload per RFC 8291 (aes128gcm).
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

	senderPriv, err := ecdsa.GenerateKey(curve, rand.Reader)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate ephemeral key: %w", err)
	}
	senderPubKey := elliptic.Marshal(curve, senderPriv.PublicKey.X, senderPriv.PublicKey.Y)

	sharedX, _ := curve.ScalarMult(receiverX, receiverY, senderPriv.D.Bytes())
	sharedSecret := zeroPad(sharedX.Bytes(), 32)

	salt := make([]byte, 16)
	if _, err := rand.Read(salt); err != nil {
		return nil, nil, nil, fmt.Errorf("failed to generate salt: %w", err)
	}

	prk := hmacSHA256(auth, sharedSecret)

	keyInfo := append([]byte("WebPush: info\x00"), p256dh...)
	keyInfo = append(keyInfo, senderPubKey...)
	ikm := hkdfExpand(prk, keyInfo, 32)

	prk2 := hmacSHA256(salt, ikm)

	cek := hkdfExpand(prk2, []byte("Content-Encoding: aes128gcm\x00"), 16)
	nonce := hkdfExpand(prk2, []byte("Content-Encoding: nonce\x00"), 12)

	padded := append(payload, 0x02)
	ciphertext, err := aesGCMEncrypt(cek, nonce, padded)
	if err != nil {
		return nil, nil, nil, fmt.Errorf("AES-GCM failed: %w", err)
	}

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

func buildSendResponse(providerName string, ok, fail, total int, errs []string) providers.SendResponse {
	meta := map[string]any{
		"success_count": ok,
		"failure_count": fail,
		"total_targets": total,
	}
	if len(errs) > 0 {
		meta["errors"] = errs
	}

	return providers.SendResponse{
		Status:   statusForCounts(ok, fail),
		Metadata: map[string]any{providerName: meta},
	}
}

func statusForCounts(ok, fail int) string {
	if ok == 0 {
		return sendStatusFailed
	}
	if fail > 0 {
		return sendStatusPartial
	}
	return sendStatusSent
}

func hmacSHA256(key, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	_, _ = mac.Write(data)
	return mac.Sum(nil)
}

func signES256P256(priv *ecdsa.PrivateKey, digest []byte) ([]byte, error) {
	r, s, err := ecdsa.Sign(rand.Reader, priv, digest)
	if err != nil {
		return nil, err
	}
	params := priv.Params()
	if params != nil && params.N != nil {
		halfOrder := new(big.Int).Rsh(new(big.Int).Set(params.N), 1)
		if s.Cmp(halfOrder) > 0 {
			s.Sub(params.N, s)
		}
	}

	sig := make([]byte, 64)
	r.FillBytes(sig[:32])
	s.FillBytes(sig[32:])
	return sig, nil
}

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

func extractOrigin(endpoint string) string {
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
