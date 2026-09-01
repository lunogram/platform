package sso

import (
	"bytes"
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/pem"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/crewjam/saml"
	"github.com/lunogram/platform/internal/ssrf"
	xrv "github.com/mattermost/xml-roundtrip-validator"
	"golang.org/x/sync/singleflight"
)

var (
	// ErrSAMLEntityMismatch reports that the metadata document does not
	// describe the entity the deployment is configured for.
	ErrSAMLEntityMismatch = errors.New("sso: the metadata document names a different entity id")
	// ErrSAMLMetadataIncomplete reports that the document describes no identity
	// provider the login flow could use.
	ErrSAMLMetadataIncomplete = errors.New("sso: the metadata document describes no usable identity provider")
	// ErrSAMLMetadataInsecure reports an endpoint the deployment's outbound
	// policy refuses: plaintext, or an address that is not a public one.
	ErrSAMLMetadataInsecure = errors.New("sso: the metadata document advertises an endpoint that is not reachable under the outbound policy")
	// ErrSAMLNoCertificate reports that no signing certificate could be found,
	// which would leave nothing to prove an assertion against.
	ErrSAMLNoCertificate = errors.New("sso: the identity provider has no signing certificate")
)

// maxSAMLMetadataBytes caps a metadata document, so a misbehaving or hostile
// endpoint cannot be read into memory without bound. SAML metadata is larger
// than a discovery document -- it carries certificates -- but not by orders of
// magnitude.
const maxSAMLMetadataBytes = 4 << 20

const samlMetadataFetchTimeout = 15 * time.Second

// samlMetadataTTL is how long a provider's metadata is kept in process.
//
// It is shorter than the OpenID Connect equivalent on purpose. A discovery
// document names a JWKS whose contents are refetched on rotation; SAML metadata
// carries the signing certificates themselves, so this cache is the only thing
// that picks up a certificate roll.
const samlMetadataTTL = 5 * time.Minute

type samlMetadataEntry struct {
	descriptor *saml.EntityDescriptor
	expires    time.Time
}

// SAMLMetadata fetches and caches identity provider metadata.
//
// It is the SAML counterpart of [Discovery], and it is deliberately weaker in
// one respect: there is no origin binding. An OpenID Connect issuer is a URL, so
// its discovery document can be required to come from its own origin. A SAML
// entity id is an opaque URI -- Entra publishes https://sts.windows.net/<tenant>/,
// others publish a urn: with no host at all -- so there is no origin to compare
// against. What binds the document here is that the operator chose the metadata
// URL, it must be reachable under the outbound policy (which refuses plaintext),
// and the document must name the entity id the operator configured.
type SAMLMetadata struct {
	client       *http.Client
	ttl          time.Duration
	fetchTimeout time.Duration
	policy       ssrf.Policy

	group   singleflight.Group
	mu      sync.RWMutex
	entries map[string]samlMetadataEntry
}

func NewSAMLMetadata(client *http.Client, policy ssrf.Policy, ttl time.Duration) *SAMLMetadata {
	if ttl <= 0 {
		ttl = samlMetadataTTL
	}
	return &SAMLMetadata{
		client:       client,
		ttl:          ttl,
		fetchTimeout: samlMetadataFetchTimeout,
		policy:       policy,
		entries:      make(map[string]samlMetadataEntry),
	}
}

// Descriptor returns the identity provider's metadata, from cache when fresh.
func (m *SAMLMetadata) Descriptor(ctx context.Context, metadataURL, entityID string) (*saml.EntityDescriptor, error) {
	if err := m.ValidateMetadataURL(metadataURL); err != nil {
		return nil, err
	}

	// Keyed on the pair rather than the URL alone, for the reason
	// [Discovery.Metadata] is: a document cached for one entity id has only been
	// checked against that one, and serving it to another -- or joining its
	// in-flight fetch -- would skip the entity match below.
	key := metadataURL + "\n" + entityID

	m.mu.RLock()
	entry, ok := m.entries[key]
	m.mu.RUnlock()
	if ok && time.Now().Before(entry.expires) {
		return entry.descriptor, nil
	}

	// Detached from every caller's context, so a login whose browser closed
	// mid-flight does not fail the logins that coalesced behind it.
	shared := m.group.DoChan(key, func() (any, error) {
		fetchCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), m.fetchTimeout)
		defer cancel()

		descriptor, err := m.fetch(fetchCtx, metadataURL)
		if err != nil {
			return nil, err
		}
		if descriptor.EntityID != entityID {
			return nil, fmt.Errorf("%w: %q", ErrSAMLEntityMismatch, descriptor.EntityID)
		}
		if err := m.ValidateDescriptor(descriptor); err != nil {
			return nil, err
		}

		m.mu.Lock()
		m.entries[key] = samlMetadataEntry{descriptor: descriptor, expires: time.Now().Add(m.ttl)}
		m.mu.Unlock()
		return descriptor, nil
	})

	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	case result := <-shared:
		if result.Err != nil {
			return nil, result.Err
		}
		return result.Val.(*saml.EntityDescriptor), nil
	}
}

func (m *SAMLMetadata) fetch(ctx context.Context, metadataURL string) (*saml.EntityDescriptor, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, metadataURL, nil)
	if err != nil {
		return nil, err
	}

	response, err := m.client.Do(request)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("sso: the metadata endpoint returned %s", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxSAMLMetadataBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxSAMLMetadataBytes {
		return nil, fmt.Errorf("sso: the metadata document exceeds %d bytes", maxSAMLMetadataBytes)
	}

	return ParseSAMLMetadata(body)
}

// ParseSAMLMetadata reads a SAML 2.0 metadata document.
//
// A document describing several entities (an EntitiesDescriptor, which
// federations publish) is refused rather than searched. Picking one entity out
// of a bundle means choosing which identity provider signs this deployment's
// logins, and that is the operator's decision to write down, not this
// function's to make.
func ParseSAMLMetadata(document []byte) (*saml.EntityDescriptor, error) {
	descriptor := &saml.EntityDescriptor{}
	if err := xmlUnmarshalStrict(document, descriptor); err != nil {
		return nil, fmt.Errorf("sso: the metadata document could not be read: %w", err)
	}
	if descriptor.EntityID == "" {
		return nil, fmt.Errorf("%w: it names no entity id", ErrSAMLMetadataIncomplete)
	}
	return descriptor, nil
}

// ValidateDescriptor holds every endpoint the document advertises to the
// deployment's outbound policy, and requires at least one usable sign-on
// endpoint and one signing certificate.
//
// The document, not the operator, is what names the sign-on endpoint, so an
// endpoint in it is as sensitive as one an operator configured. Checking only
// that the field is non-empty would accept a plaintext sign-on URL, which is a
// browser redirect an attacker on the path could rewrite.
func (m *SAMLMetadata) ValidateDescriptor(descriptor *saml.EntityDescriptor) error {
	location := SAMLSignOnLocation(descriptor)
	if location == "" {
		return fmt.Errorf("%w: it advertises no HTTP-Redirect or HTTP-POST SingleSignOnService", ErrSAMLMetadataIncomplete)
	}
	if err := ssrf.ValidateURL(location, m.policy); err != nil {
		return fmt.Errorf("%w: %q: %w", ErrSAMLMetadataInsecure, location, err)
	}
	if len(SAMLSigningCertificates(descriptor)) == 0 {
		return ErrSAMLNoCertificate
	}
	return nil
}

// ValidateMetadataURL checks that a metadata URL is reachable under the
// deployment's outbound policy. It runs at startup and again on every login, so
// a URL that stopped being acceptable stops being used rather than staying
// trusted because it passed once.
func (m *SAMLMetadata) ValidateMetadataURL(metadataURL string) error {
	if err := ssrf.ValidateURL(metadataURL, m.policy); err != nil {
		return fmt.Errorf("%w: %q: %w", ErrSAMLMetadataInsecure, metadataURL, err)
	}
	return nil
}

// SAMLSignOnLocation is where an AuthnRequest is sent, preferring the
// HTTP-Redirect binding.
//
// Redirect is preferred because that is what this deployment sends: a signed
// AuthnRequest carried in query parameters, which every identity provider
// implements. A provider that only advertises HTTP-POST is still usable, and
// the caller sends a self-submitting form instead.
func SAMLSignOnLocation(descriptor *saml.EntityDescriptor) string {
	if location := samlSignOnLocation(descriptor, saml.HTTPRedirectBinding); location != "" {
		return location
	}
	return samlSignOnLocation(descriptor, saml.HTTPPostBinding)
}

// SAMLSupportsRedirectBinding reports whether the provider advertises the
// binding this deployment prefers.
func SAMLSupportsRedirectBinding(descriptor *saml.EntityDescriptor) bool {
	return samlSignOnLocation(descriptor, saml.HTTPRedirectBinding) != ""
}

func samlSignOnLocation(descriptor *saml.EntityDescriptor, binding string) string {
	if descriptor == nil {
		return ""
	}
	for _, idp := range descriptor.IDPSSODescriptors {
		for _, service := range idp.SingleSignOnServices {
			if service.Binding == binding && service.Location != "" {
				return service.Location
			}
		}
	}
	return ""
}

// SAMLSigningCertificates is every certificate the provider publishes for
// signing.
//
// A key descriptor with no `use` is usable for both signing and encryption by
// the metadata specification, so it counts here; one marked "encryption" does
// not. Accepting an encryption-only key as a signing key would mean proving an
// assertion against a key the provider never said it signs with.
func SAMLSigningCertificates(descriptor *saml.EntityDescriptor) []*x509.Certificate {
	if descriptor == nil {
		return nil
	}

	var certificates []*x509.Certificate
	for _, idp := range descriptor.IDPSSODescriptors {
		for _, key := range idp.KeyDescriptors {
			if key.Use != "" && key.Use != "signing" {
				continue
			}
			for _, data := range key.KeyInfo.X509Data.X509Certificates {
				certificate, err := parseBase64Certificate(data.Data)
				if err != nil {
					continue
				}
				certificates = append(certificates, certificate)
			}
		}
	}
	return certificates
}

// DescriptorFromFields builds the metadata an operator would otherwise have to
// publish, from the three things every identity provider's setup screen shows:
// its entity id, its sign-on URL, and its signing certificate.
//
// It exists because not every deployment can fetch a metadata URL. An IdP that
// only offers a downloadable file, or an egress policy that refuses the call,
// would otherwise make the driver unconfigurable.
func DescriptorFromFields(entityID, signOnURL string, certificatesPEM string) (*saml.EntityDescriptor, error) {
	certificates, err := ParseCertificates(certificatesPEM)
	if err != nil {
		return nil, err
	}
	if len(certificates) == 0 {
		return nil, ErrSAMLNoCertificate
	}

	keys := make([]saml.KeyDescriptor, 0, len(certificates))
	for _, certificate := range certificates {
		keys = append(keys, saml.KeyDescriptor{
			Use: "signing",
			KeyInfo: saml.KeyInfo{
				X509Data: saml.X509Data{
					X509Certificates: []saml.X509Certificate{{
						Data: base64Certificate(certificate),
					}},
				},
			},
		})
	}

	return &saml.EntityDescriptor{
		EntityID: entityID,
		IDPSSODescriptors: []saml.IDPSSODescriptor{{
			SSODescriptor: saml.SSODescriptor{
				RoleDescriptor: saml.RoleDescriptor{KeyDescriptors: keys},
			},
			SingleSignOnServices: []saml.Endpoint{
				{Binding: saml.HTTPRedirectBinding, Location: signOnURL},
			},
		}},
	}, nil
}

// ParseCertificates reads every CERTIFICATE block in a PEM bundle. Identity
// providers hand out one certificate normally and two across a rotation, and
// both are pasted into the same setting.
func ParseCertificates(bundle string) ([]*x509.Certificate, error) {
	rest := []byte(strings.TrimSpace(bundle))
	var certificates []*x509.Certificate

	for len(rest) > 0 {
		var block *pem.Block
		block, rest = pem.Decode(rest)
		// Whatever is left is not a certificate. Stopping quietly here would
		// take a bundle whose second block is truncated as a bundle of one,
		// and the deployment would go live trusting half the keys the operator
		// pasted in -- which only shows up as failed logins at the rotation
		// the second key was there for.
		if block == nil {
			return nil, fmt.Errorf("sso: %d bytes after the last certificate are not a PEM block", len(bytes.TrimSpace(rest)))
		}
		if block.Type != "CERTIFICATE" {
			return nil, fmt.Errorf("sso: expected a CERTIFICATE block, found %q", block.Type)
		}

		certificate, err := x509.ParseCertificate(block.Bytes)
		if err != nil {
			return nil, fmt.Errorf("sso: the certificate could not be read: %w", err)
		}
		certificates = append(certificates, certificate)
		rest = bytes.TrimSpace(rest)
	}

	return certificates, nil
}

// xmlUnmarshalStrict validates that a document survives an XML round trip
// before unmarshalling it.
//
// The round-trip check is not decoration. Go's XML parser and the canonicaliser
// that signature verification runs over disagree about a handful of malformed
// documents, and that disagreement is the XML signature wrapping attack: a
// document that means one thing to the verifier and another to the parser. The
// check refuses those documents outright.
func xmlUnmarshalStrict(document []byte, into any) error {
	if err := xrv.Validate(bytes.NewReader(document)); err != nil {
		return fmt.Errorf("the document does not survive an xml round trip: %w", err)
	}
	if err := xml.Unmarshal(document, into); err != nil {
		if strings.Contains(err.Error(), "have <EntitiesDescriptor>") {
			return errors.New("it describes several entities; configure the one identity provider's own metadata")
		}
		return err
	}
	return nil
}

// parseBase64Certificate reads a certificate as XML carries it: base64 with
// whatever whitespace the document's indentation introduced.
func parseBase64Certificate(data string) (*x509.Certificate, error) {
	cleaned := strings.Map(func(r rune) rune {
		if unicode.IsSpace(r) {
			return -1
		}
		return r
	}, data)

	raw, err := base64.StdEncoding.DecodeString(cleaned)
	if err != nil {
		return nil, err
	}
	return x509.ParseCertificate(raw)
}

func base64Certificate(certificate *x509.Certificate) string {
	return base64.StdEncoding.EncodeToString(certificate.Raw)
}
