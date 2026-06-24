package main

import (
	"crypto/hmac"
	"crypto/sha256"
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

// validateConfig checks the required SES credentials and returns a map of
// field-level error messages keyed by the config property name. An empty map
// means the configuration is valid.
func validateConfig(config Config) map[string]string {
	errs := make(map[string]string)
	if config.AccessKeyID == "" {
		errs["accessKeyId"] = "Access key ID is required"
	}
	if config.SecretAccessKey == "" {
		errs["secretAccessKey"] = "Secret access key is required"
	}
	if config.Region == "" {
		errs["region"] = "AWS region is required"
	}
	return errs
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
	bodyHash := sha256.Sum256(body)
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

	canonicalReqHash := sha256.Sum256([]byte(canonicalReq))
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

// hmacSHA256 computes HMAC-SHA256 of data using key. The standard library's
// crypto/sha256 and crypto/hmac are pure Go and run correctly under the TinyGo
// wasi target used to build this module.
func hmacSHA256(key []byte, data []byte) []byte {
	mac := hmac.New(sha256.New, key)
	mac.Write(data)
	return mac.Sum(nil)
}
