package main

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"

	"github.com/lunogram/platform/pkg/modules/providers"
)

// Exit code convention for WASM provider modules:
//
//	 0  — success
//	-1  — transient/retryable error
//	-2  — permanent/non-retryable error
const (
	ExitTransient int32 = -1
	ExitPermanent int32 = -2
	ExitSuccess   int32 = 0
)

// Config holds the Amazon SES provider configuration persisted by the platform.
type Config struct {
	AccessKeyID      string `json:"accessKeyId"`
	SecretAccessKey  string `json:"secretAccessKey"`
	Region           string `json:"region"`
	SessionToken     string `json:"sessionToken,omitempty"`
	ConfigurationSet string `json:"configurationSet,omitempty"`
}

type SESv2SendEmailRequest struct {
	FromEmailAddress string `json:"FromEmailAddress"`
	Destination      struct {
		ToAddresses  []string `json:"ToAddresses,omitempty"`
		CcAddresses  []string `json:"CcAddresses,omitempty"`
		BccAddresses []string `json:"BccAddresses,omitempty"`
	} `json:"Destination"`
	ReplyToAddresses []string `json:"ReplyToAddresses,omitempty"`
	Content          struct {
		Simple struct {
			Subject struct {
				Data string `json:"Data"`
			} `json:"Subject"`
			Body struct {
				Html *struct {
					Data string `json:"Data"`
				} `json:"Html,omitempty"`
				Text *struct {
					Data string `json:"Data"`
				} `json:"Text,omitempty"`
			} `json:"Body"`
		} `json:"Simple"`
	} `json:"Content"`
	ConfigurationSetName *string `json:"ConfigurationSetName,omitempty"`
}

// formatAddress formats an EmailAddress as "Name <addr>" or just "addr".
func formatAddress(address providers.EmailAddress) string {
	if address.Name != "" {
		return fmt.Sprintf("%s <%s>", address.Name, address.Address)
	}
	return address.Address
}

// classifyError inspects an error and returns the appropriate WASM
// exit code: transient (retryable) or permanent.
func classifyError(err error) int32 {
	if err == nil {
		return ExitSuccess
	}
	msg := err.Error()
	if strings.Contains(msg, "Throttling") || strings.Contains(msg, "Rate exceeded") || strings.Contains(msg, "status 5") {
		return ExitTransient
	}
	return ExitPermanent
}

// ComposeSendEmailRequest converts platform email payload to an SES SDK request.
func ComposeSendEmailRequest(email providers.EmailPayload, cfg Config) *SESv2SendEmailRequest {
	req := &SESv2SendEmailRequest{
		FromEmailAddress: formatAddress(email.From),
	}
	req.Destination.ToAddresses = []string{email.To}
	if email.Cc != nil {
		req.Destination.CcAddresses = []string{*email.Cc}
	}
	if email.Bcc != nil {
		req.Destination.BccAddresses = []string{*email.Bcc}
	}
	if email.ReplyTo != nil {
		req.ReplyToAddresses = []string{*email.ReplyTo}
	}
	req.Content.Simple.Subject.Data = email.Subject
	if email.HTML != "" {
		req.Content.Simple.Body.Html = &struct {
			Data string `json:"Data"`
		}{Data: email.HTML}
	}
	if email.Text != "" {
		req.Content.Simple.Body.Text = &struct {
			Data string `json:"Data"`
		}{Data: email.Text}
	}
	if cfg.ConfigurationSet != "" {
		req.ConfigurationSetName = &cfg.ConfigurationSet
	}
	return req
}

// SignV4 adds the AWS V4 signature headers to the request.
func SignV4(req *http.Request, body []byte, accessKey, secretKey, sessionToken, region, service string, t time.Time) {
	bodyHash := sha256Sum256(body)
	bodyHashStr := hex.EncodeToString(bodyHash[:])

	amzDate := t.UTC().Format("20060102T150405Z")
	dateStamp := t.UTC().Format("20060102")

	req.Header.Set("x-amz-date", amzDate)
	req.Header.Set("x-amz-content-sha256", bodyHashStr)
	if sessionToken != "" {
		req.Header.Set("X-Amz-Security-Token", sessionToken)
	}

	if req.Host == "" {
		req.Host = req.URL.Host
	}

	canonicalReq := req.Method + "\n"

	canonicalURI := req.URL.Path
	if canonicalURI == "" {
		canonicalURI = "/"
	}
	if !strings.HasPrefix(canonicalURI, "/") {
		canonicalURI = "/" + canonicalURI
	}
	canonicalReq += canonicalURI + "\n"

	canonicalReq += req.URL.RawQuery + "\n"

	signedHeaders := []string{"host"}
	var canonicalHeaders string
	for k := range req.Header {
		hk := strings.ToLower(k)
		if hk == "host" || strings.HasPrefix(hk, "x-amz-") || hk == "content-type" {
			signedHeaders = append(signedHeaders, hk)
		}
	}
	sort.Strings(signedHeaders)

	for _, hk := range signedHeaders {
		if hk == "host" {
			canonicalHeaders += hk + ":" + req.Host + "\n"
			continue
		}
		val := req.Header.Get(hk)
		canonicalHeaders += hk + ":" + strings.TrimSpace(val) + "\n"
	}
	canonicalReq += canonicalHeaders + "\n"

	signedHeadersStr := strings.Join(signedHeaders, ";")
	canonicalReq += signedHeadersStr + "\n"

	canonicalReq += bodyHashStr

	canonicalReqHash := sha256Sum256([]byte(canonicalReq))
	canonicalReqHashStr := hex.EncodeToString(canonicalReqHash[:])

	algorithm := "AWS4-HMAC-SHA256"
	credentialScope := dateStamp + "/" + region + "/" + service + "/aws4_request"
	stringToSign := algorithm + "\n" + amzDate + "\n" + credentialScope + "\n" + canonicalReqHashStr

	kDate := hmacSHA256([]byte("AWS4"+secretKey), []byte(dateStamp))
	kRegion := hmacSHA256(kDate, []byte(region))
	kService := hmacSHA256(kRegion, []byte(service))
	kSigning := hmacSHA256(kService, []byte("aws4_request"))
	signature := hmacSHA256(kSigning, []byte(stringToSign))
	signatureStr := hex.EncodeToString(signature)

	authHeader := algorithm + " Credential=" + accessKey + "/" + credentialScope + ", SignedHeaders=" + signedHeadersStr + ", Signature=" + signatureStr
	req.Header.Set("Authorization", authHeader)
}

func hmacSHA256(key []byte, data []byte) []byte {
	const blockSize = 64
	keyBlock := make([]byte, blockSize)
	if len(key) > blockSize {
		keySum := sha256Sum256(key)
		copy(keyBlock, keySum[:])
	} else {
		copy(keyBlock, key)
	}

	oKeyPad := make([]byte, blockSize)
	iKeyPad := make([]byte, blockSize)
	for i := 0; i < blockSize; i++ {
		oKeyPad[i] = keyBlock[i] ^ 0x5c
		iKeyPad[i] = keyBlock[i] ^ 0x36
	}

	innerMsg := make([]byte, 0, blockSize+len(data))
	innerMsg = append(innerMsg, iKeyPad...)
	innerMsg = append(innerMsg, data...)
	innerSum := sha256Sum256(innerMsg)

	outerMsg := make([]byte, 0, blockSize+len(innerSum))
	outerMsg = append(outerMsg, oKeyPad...)
	outerMsg = append(outerMsg, innerSum[:]...)
	outerSum := sha256Sum256(outerMsg)
	return outerSum[:]
}

// sha256Sum256 is a tinygo-safe SHA-256 implementation to avoid WASM stdlib panics.
func sha256Sum256(data []byte) [32]byte {
	var h0 uint32 = 0x6a09e667
	var h1 uint32 = 0xbb67ae85
	var h2 uint32 = 0x3c6ef372
	var h3 uint32 = 0xa54ff53a
	var h4 uint32 = 0x510e527f
	var h5 uint32 = 0x9b05688c
	var h6 uint32 = 0x1f83d9ab
	var h7 uint32 = 0x5be0cd19

	bitLen := uint64(len(data)) * 8
	padLen := (56 - (len(data)+1)%64 + 64) % 64
	totalLen := len(data) + 1 + padLen + 8
	buf := make([]byte, totalLen)
	copy(buf, data)
	buf[len(data)] = 0x80
	binary.BigEndian.PutUint64(buf[totalLen-8:], bitLen)

	var w [64]uint32
	for i := 0; i < totalLen; i += 64 {
		block := buf[i : i+64]
		for j := 0; j < 16; j++ {
			w[j] = binary.BigEndian.Uint32(block[j*4 : j*4+4])
		}
		for j := 16; j < 64; j++ {
			s0 := rotr(w[j-15], 7) ^ rotr(w[j-15], 18) ^ (w[j-15] >> 3)
			s1 := rotr(w[j-2], 17) ^ rotr(w[j-2], 19) ^ (w[j-2] >> 10)
			w[j] = w[j-16] + s0 + w[j-7] + s1
		}

		a := h0
		b := h1
		c := h2
		d := h3
		e := h4
		f := h5
		g := h6
		h := h7

		for j := 0; j < 64; j++ {
			s1 := rotr(e, 6) ^ rotr(e, 11) ^ rotr(e, 25)
			ch := (e & f) ^ (^e & g)
			t1 := h + s1 + ch + sha256K[j] + w[j]
			s0 := rotr(a, 2) ^ rotr(a, 13) ^ rotr(a, 22)
			maj := (a & b) ^ (a & c) ^ (b & c)
			t2 := s0 + maj

			h = g
			g = f
			f = e
			e = d + t1
			d = c
			c = b
			b = a
			a = t1 + t2
		}

		h0 += a
		h1 += b
		h2 += c
		h3 += d
		h4 += e
		h5 += f
		h6 += g
		h7 += h
	}

	var out [32]byte
	binary.BigEndian.PutUint32(out[0:], h0)
	binary.BigEndian.PutUint32(out[4:], h1)
	binary.BigEndian.PutUint32(out[8:], h2)
	binary.BigEndian.PutUint32(out[12:], h3)
	binary.BigEndian.PutUint32(out[16:], h4)
	binary.BigEndian.PutUint32(out[20:], h5)
	binary.BigEndian.PutUint32(out[24:], h6)
	binary.BigEndian.PutUint32(out[28:], h7)
	return out
}

func rotr(x uint32, n uint32) uint32 {
	return (x >> n) | (x << (32 - n))
}

var sha256K = [64]uint32{
	0x428a2f98, 0x71374491, 0xb5c0fbcf, 0xe9b5dba5,
	0x3956c25b, 0x59f111f1, 0x923f82a4, 0xab1c5ed5,
	0xd807aa98, 0x12835b01, 0x243185be, 0x550c7dc3,
	0x72be5d74, 0x80deb1fe, 0x9bdc06a7, 0xc19bf174,
	0xe49b69c1, 0xefbe4786, 0x0fc19dc6, 0x240ca1cc,
	0x2de92c6f, 0x4a7484aa, 0x5cb0a9dc, 0x76f988da,
	0x983e5152, 0xa831c66d, 0xb00327c8, 0xbf597fc7,
	0xc6e00bf3, 0xd5a79147, 0x06ca6351, 0x14292967,
	0x27b70a85, 0x2e1b2138, 0x4d2c6dfc, 0x53380d13,
	0x650a7354, 0x766a0abb, 0x81c2c92e, 0x92722c85,
	0xa2bfe8a1, 0xa81a664b, 0xc24b8b70, 0xc76c51a3,
	0xd192e819, 0xd6990624, 0xf40e3585, 0x106aa070,
	0x19a4c116, 0x1e376c08, 0x2748774c, 0x34b0bcb5,
	0x391c0cb3, 0x4ed8aa4a, 0x5b9cca4f, 0x682e6ff3,
	0x748f82ee, 0x78a5636f, 0x84c87814, 0x8cc70208,
	0x90befffa, 0xa4506ceb, 0xbef9a3f7, 0xc67178f2,
}
