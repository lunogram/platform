package sso

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/lunogram/platform/internal/ssrf"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testCertificate(t *testing.T) *x509.Certificate {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "idp"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	raw, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	certificate, err := x509.ParseCertificate(raw)
	require.NoError(t, err)
	return certificate
}

func metadataDocument(entityID, location string, certificate *x509.Certificate, use string) string {
	return fmt.Sprintf(`<?xml version="1.0"?>
<EntityDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata" entityID="%s">
  <IDPSSODescriptor protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">
    <KeyDescriptor use="%s">
      <KeyInfo xmlns="http://www.w3.org/2000/09/xmldsig#">
        <X509Data><X509Certificate>%s</X509Certificate></X509Data>
      </KeyInfo>
    </KeyDescriptor>
    <SingleSignOnService Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect" Location="%s"/>
  </IDPSSODescriptor>
</EntityDescriptor>`, entityID, use, base64.StdEncoding.EncodeToString(certificate.Raw), location)
}

// The metadata a deployment fetches decides which key proves its assertions, so
// the document has to name the entity id the operator configured.
func TestSAMLMetadataBindsTheDocumentToTheEntityID(t *testing.T) {
	t.Parallel()

	certificate := testCertificate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, metadataDocument("urn:the:idp", "https://idp.test/sso", certificate, "signing"))
	}))
	t.Cleanup(server.Close)

	// The stub is on loopback over plaintext, so the test runs under
	// relaxations an operator would have to opt into explicitly.
	cache := NewSAMLMetadata(http.DefaultClient, ssrf.Policy{AllowPrivate: true, AllowHTTP: true}, 0)

	descriptor, err := cache.Descriptor(context.Background(), server.URL, "urn:the:idp")
	require.NoError(t, err)
	assert.Equal(t, "urn:the:idp", descriptor.EntityID)
	assert.Equal(t, "https://idp.test/sso", SAMLSignOnLocation(descriptor))
	assert.Len(t, SAMLSigningCertificates(descriptor), 1)

	_, err = cache.Descriptor(context.Background(), server.URL, "urn:someone:else")
	require.ErrorIs(t, err, ErrSAMLEntityMismatch)
}

// The document, not the operator, names the sign-on endpoint, so it is held to
// the same outbound policy as an endpoint an operator configured.
func TestSAMLMetadataRefusesAnEndpointThePolicyRefuses(t *testing.T) {
	t.Parallel()

	certificate := testCertificate(t)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, metadataDocument("urn:the:idp", "http://169.254.169.254/sso", certificate, "signing"))
	}))
	t.Cleanup(server.Close)

	cache := NewSAMLMetadata(http.DefaultClient, ssrf.Policy{AllowPrivate: true, AllowHTTP: true}, 0)

	_, err := cache.Descriptor(context.Background(), server.URL, "urn:the:idp")
	require.ErrorIs(t, err, ErrSAMLMetadataInsecure)
}

// A key the provider publishes for encryption is not one it said it signs with.
func TestSAMLSigningCertificatesIgnoresEncryptionKeys(t *testing.T) {
	t.Parallel()

	certificate := testCertificate(t)

	signing, err := ParseSAMLMetadata([]byte(metadataDocument("urn:idp", "https://idp.test/sso", certificate, "signing")))
	require.NoError(t, err)
	assert.Len(t, SAMLSigningCertificates(signing), 1)

	unspecified, err := ParseSAMLMetadata([]byte(metadataDocument("urn:idp", "https://idp.test/sso", certificate, "")))
	require.NoError(t, err)
	assert.Len(t, SAMLSigningCertificates(unspecified), 1, "a key with no use is usable for both")

	encryption, err := ParseSAMLMetadata([]byte(metadataDocument("urn:idp", "https://idp.test/sso", certificate, "encryption")))
	require.NoError(t, err)
	assert.Empty(t, SAMLSigningCertificates(encryption))
}

// Choosing one entity out of a federation bundle means choosing which identity
// provider signs this deployment's logins, which is the operator's decision.
func TestParseSAMLMetadataRefusesABundle(t *testing.T) {
	t.Parallel()

	_, err := ParseSAMLMetadata([]byte(`<?xml version="1.0"?>
<EntitiesDescriptor xmlns="urn:oasis:names:tc:SAML:2.0:metadata">
  <EntityDescriptor entityID="urn:one"/>
  <EntityDescriptor entityID="urn:two"/>
</EntitiesDescriptor>`))
	require.Error(t, err)
	assert.Contains(t, err.Error(), "several entities")
}

// The three fields an identity provider's setup screen shows are enough to
// configure a deployment that cannot fetch a metadata URL.
func TestDescriptorFromFields(t *testing.T) {
	t.Parallel()

	certificate := testCertificate(t)
	bundle := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: certificate.Raw}))

	descriptor, err := DescriptorFromFields("urn:idp", "https://idp.test/sso", bundle)
	require.NoError(t, err)
	assert.Equal(t, "urn:idp", descriptor.EntityID)
	assert.Equal(t, "https://idp.test/sso", SAMLSignOnLocation(descriptor))
	require.Len(t, SAMLSigningCertificates(descriptor), 1)
	assert.Equal(t, certificate.Raw, SAMLSigningCertificates(descriptor)[0].Raw)

	_, err = DescriptorFromFields("urn:idp", "https://idp.test/sso", "")
	require.ErrorIs(t, err, ErrSAMLNoCertificate)
}

// Identity providers hand out two certificates across a rotation, pasted into
// the same setting.
func TestParseCertificatesReadsABundle(t *testing.T) {
	t.Parallel()

	first := testCertificate(t)
	second := testCertificate(t)
	bundle := string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: first.Raw})) +
		string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: second.Raw}))

	certificates, err := ParseCertificates(bundle)
	require.NoError(t, err)
	require.Len(t, certificates, 2)

	_, err = ParseCertificates(string(pem.EncodeToMemory(&pem.Block{Type: "PRIVATE KEY", Bytes: []byte("nope")})))
	require.Error(t, err, "a private key pasted where a certificate belongs is refused, not ignored")

	// Reading this as a bundle of one would put the deployment live trusting
	// half the keys the operator pasted in, and it would only show up as failed
	// logins at the rotation the second key was there for.
	_, err = ParseCertificates(bundle[:len(bundle)-40])
	require.Error(t, err, "a truncated second certificate is refused, not silently dropped")
}

// The redirect binding is preferred because that is what the deployment sends,
// but a provider offering only HTTP-POST is still usable.
func TestSAMLSignOnLocationPrefersRedirect(t *testing.T) {
	t.Parallel()

	descriptor := &saml.EntityDescriptor{IDPSSODescriptors: []saml.IDPSSODescriptor{{
		SingleSignOnServices: []saml.Endpoint{
			{Binding: saml.HTTPPostBinding, Location: "https://idp.test/post"},
			{Binding: saml.HTTPRedirectBinding, Location: "https://idp.test/redirect"},
		},
	}}}
	assert.Equal(t, "https://idp.test/redirect", SAMLSignOnLocation(descriptor))
	assert.True(t, SAMLSupportsRedirectBinding(descriptor))

	postOnly := &saml.EntityDescriptor{IDPSSODescriptors: []saml.IDPSSODescriptor{{
		SingleSignOnServices: []saml.Endpoint{{Binding: saml.HTTPPostBinding, Location: "https://idp.test/post"}},
	}}}
	assert.Equal(t, "https://idp.test/post", SAMLSignOnLocation(postOnly))
	assert.False(t, SAMLSupportsRedirectBinding(postOnly))
}
