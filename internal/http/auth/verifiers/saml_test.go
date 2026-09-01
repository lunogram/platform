package verifiers

import (
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/pem"
	"math/big"
	"testing"
	"time"

	"github.com/crewjam/saml"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/sso"
	"github.com/lunogram/platform/internal/ssrf"
	goredis "github.com/redis/go-redis/v9"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

func testSAMLOptions(t *testing.T, settings config.SAMLProvider) SAMLOptions {
	t.Helper()

	logger := zaptest.NewLogger(t)
	return SAMLOptions{
		Config: settings,
		// Never dialled: these tests only construct the driver. A nil client
		// would give a nil store, which is what the driver refuses to build on.
		Flows:      sso.NewSAMLFlowStore(goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:0"}), "test:"),
		Assertions: sso.NewAssertionReplayStore(goredis.NewClient(&goredis.Options{Addr: "127.0.0.1:0"}), "test:"),
		Metadata:   sso.NewSAMLMetadata(nil, ssrf.Policy{}, 0),
		BaseURL:    "https://console.example.test",
		Logger:     logger,
	}
}

// A deployment that names the driver without configuring it is refused at boot,
// so the failure reads as a variable nobody set rather than as a broken login.
func TestNewSAMLRequiresItsSettings(t *testing.T) {
	t.Parallel()

	_, err := NewSAML(testSAMLOptions(t, config.SAMLProvider{ID: "idp"}))
	require.ErrorIs(t, err, ErrSAMLNotConfigured)

	_, err = NewSAML(testSAMLOptions(t, config.SAMLProvider{ID: "idp", EntityID: "urn:idp"}))
	require.ErrorIs(t, err, ErrSAMLNotConfigured, "an entity id alone says nothing about where to send anybody")

	_, err = NewSAML(testSAMLOptions(t, config.SAMLProvider{
		ID: "idp", EntityID: "urn:idp", SSOURL: "https://idp.test/sso",
	}))
	require.ErrorIs(t, err, ErrSAMLNotConfigured, "a sign-on URL without a certificate leaves nothing to prove an assertion against")
}

// The two ways of naming the same three things are refused together rather than
// resolved, because there is no order in which one obviously wins.
func TestNewSAMLRefusesBothMetadataForms(t *testing.T) {
	t.Parallel()

	_, err := NewSAML(testSAMLOptions(t, config.SAMLProvider{
		ID:          "idp",
		EntityID:    "urn:idp",
		MetadataURL: "https://idp.test/metadata",
		SSOURL:      "https://idp.test/sso",
	}))
	require.ErrorIs(t, err, ErrSAMLNotConfigured)
}

// The binding cookie has to be SameSite=None to survive the POST binding, and a
// SameSite=None cookie must be Secure, so there is no binding to be had over
// plaintext. The deployment is refused rather than silently unbound.
func TestNewSAMLRequiresAnHTTPSDeployment(t *testing.T) {
	t.Parallel()

	opts := testSAMLOptions(t, config.SAMLProvider{
		ID: "idp", EntityID: "urn:idp", SSOURL: "https://idp.test/sso", Certificate: testCertificatePEM(t),
	})
	opts.BaseURL = "http://console.example.test"

	_, err := NewSAML(opts)
	require.ErrorIs(t, err, ErrSAMLInsecureBaseURL)
}

// The assertion consumer service URL is derived from the deployment's public URL
// and never from a request parameter, and it is what the operator registers.
func TestNewSAMLDerivesItsURLs(t *testing.T) {
	t.Parallel()

	provider, err := NewSAML(testSAMLOptions(t, config.SAMLProvider{
		ID: "okta", EntityID: "urn:idp", SSOURL: "https://idp.test/sso", Certificate: testCertificatePEM(t),
	}))
	require.NoError(t, err)

	assert.Equal(t, SAMLDriver, provider.Driver())
	assert.Equal(t, "https://console.example.test/api/auth/saml/okta/acs", provider.ACSURL())
	assert.Equal(t, "https://console.example.test/api/auth/saml/okta/metadata", provider.MetadataURL())
	assert.Equal(t, "okta", provider.Name(), "an unnamed provider is called by its id")
}

// An unset sp_entity_id means this provider's metadata URL, which is what the
// operator registers and what the published metadata says either way. The
// service provider library applies that default itself, at the audience it
// validates an assertion against as well as at the issuer it puts on an
// AuthnRequest, so the id is passed through verbatim rather than filled in
// twice. This pins that, because the two would disagree if it ever stopped.
func TestSAMLEntityIDDefaultsToItsMetadataURL(t *testing.T) {
	t.Parallel()

	settings := config.SAMLProvider{
		ID: "okta", EntityID: "urn:idp", SSOURL: "https://idp.test/sso", Certificate: testCertificatePEM(t),
	}

	provider, err := NewSAML(testSAMLOptions(t, settings))
	require.NoError(t, err)

	sp, _, err := provider.serviceProvider(t.Context())
	require.NoError(t, err)
	assert.Equal(t, provider.MetadataURL(), sp.Metadata().EntityID)

	request, err := sp.MakeAuthenticationRequest(sp.GetSSOBindingLocation(saml.HTTPRedirectBinding), saml.HTTPRedirectBinding, saml.HTTPPostBinding)
	require.NoError(t, err)
	assert.Equal(t, provider.MetadataURL(), request.Issuer.Value)

	opts := testSAMLOptions(t, settings)
	opts.EntityID = "https://console.example.test/saml"
	provider, err = NewSAML(opts)
	require.NoError(t, err)

	sp, _, err = provider.serviceProvider(t.Context())
	require.NoError(t, err)
	assert.Equal(t, "https://console.example.test/saml", sp.Metadata().EntityID,
		"a configured entity id is what the deployment is known by")
}

// A sign-on URL the operator typed is held to the same outbound policy as one a
// provider published, because it is where the browser is sent with an
// AuthnRequest and nothing downstream tells the two apart.
func TestNewSAMLRefusesAPlaintextSignOnURL(t *testing.T) {
	t.Parallel()

	_, err := NewSAML(testSAMLOptions(t, config.SAMLProvider{
		ID: "idp", EntityID: "urn:idp", SSOURL: "http://idp.test/sso", Certificate: testCertificatePEM(t),
	}))
	require.ErrorIs(t, err, sso.ErrSAMLMetadataInsecure)
}

// An allow-list that normalises away to nothing would read as "any domain",
// which is the opposite of what somebody who configured one meant.
func TestNewSAMLRefusesAnEmptyAllowList(t *testing.T) {
	t.Parallel()

	_, err := NewSAML(testSAMLOptions(t, config.SAMLProvider{
		ID: "idp", EntityID: "urn:idp", SSOURL: "https://idp.test/sso", Certificate: testCertificatePEM(t),
		AllowedDomains: []string{"  ", ""},
	}))
	require.ErrorIs(t, err, ErrSAMLNotConfigured)
}

// SAML carries no attestation of an address, so the attestation is the
// operator's and defaults to trusting the directory they configured.
func TestSAMLTrustEmailDefaultsOn(t *testing.T) {
	t.Parallel()

	base := config.SAMLProvider{
		ID: "idp", EntityID: "urn:idp", SSOURL: "https://idp.test/sso", Certificate: testCertificatePEM(t),
	}

	provider, err := NewSAML(testSAMLOptions(t, base))
	require.NoError(t, err)
	assert.True(t, provider.trustEmail)

	off := false
	base.TrustEmail = &off
	provider, err = NewSAML(testSAMLOptions(t, base))
	require.NoError(t, err)
	assert.False(t, provider.trustEmail, "an explicit false is what turns it off")
}

// A key pair is loaded together, so a certificate that does not belong to the
// key is refused at boot rather than producing signatures nobody can verify.
func TestParseSAMLKeypair(t *testing.T) {
	t.Parallel()

	pair, err := ParseSAMLKeypair("", "")
	require.NoError(t, err)
	assert.Nil(t, pair, "a deployment may configure no key material at all")

	_, err = ParseSAMLKeypair(testCertificatePEM(t), "")
	require.ErrorIs(t, err, ErrSAMLKeypairIncomplete)

	certificate, key := testKeypairPEM(t)
	pair, err = ParseSAMLKeypair(certificate, key)
	require.NoError(t, err)
	require.NotNil(t, pair)
	assert.NotNil(t, pair.Certificate)
	assert.NotNil(t, pair.Key)
}

// A configured attribute is used on its own. Falling back to the candidates
// when it misses would mean a deployment that pointed at a specific attribute
// silently reading a different one.
func TestSAMLLookup(t *testing.T) {
	t.Parallel()

	attributes := map[string]string{
		"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress": "claimed@example.test",
		"mail": "directory@example.test",
	}

	assert.Equal(t, "claimed@example.test", samlLookup(attributes, "", samlEmailAttributes),
		"the candidates are tried in order")
	assert.Equal(t, "directory@example.test", samlLookup(attributes, "mail", samlEmailAttributes))
	assert.Equal(t, "", samlLookup(attributes, "upn", samlEmailAttributes),
		"a configured attribute that is absent reads as absent, not as a reason to look elsewhere")
}

// Attributes are keyed by both Name and FriendlyName so either can be
// configured, and the first value of a multi-valued attribute wins.
func TestSAMLAttributes(t *testing.T) {
	t.Parallel()

	assertion := &saml.Assertion{AttributeStatements: []saml.AttributeStatement{{
		Attributes: []saml.Attribute{
			{
				Name:         "urn:oid:0.9.2342.19200300.100.1.3",
				FriendlyName: "mail",
				Values: []saml.AttributeValue{
					{Value: "  first@example.test "},
					{Value: "second@example.test"},
				},
			},
			{Name: "empty", Values: []saml.AttributeValue{{Value: "   "}}},
		},
	}}}

	attributes := samlAttributes(assertion)
	assert.Equal(t, "first@example.test", attributes["urn:oid:0.9.2342.19200300.100.1.3"])
	assert.Equal(t, "first@example.test", attributes["mail"], "the friendly name reaches the same value")
	assert.NotContains(t, attributes, "empty", "whitespace is not a value")
}

// A transient NameID is a per-session pseudonym. Storing it as the identity key
// would provision a fresh admin on every sign-in.
func TestSAMLRefusesATransientNameID(t *testing.T) {
	t.Parallel()

	provider, err := NewSAML(testSAMLOptions(t, config.SAMLProvider{
		ID: "idp", EntityID: "urn:idp", SSOURL: "https://idp.test/sso", Certificate: testCertificatePEM(t),
	}))
	require.NoError(t, err)

	_, err = provider.identity(&saml.Assertion{Subject: &saml.Subject{NameID: &saml.NameID{
		Format: string(saml.TransientNameIDFormat),
		Value:  "AAdzZWNyZXQx",
	}}})
	require.ErrorIs(t, err, ErrSAMLTransientNameID)
}

// A provider that puts the address in the NameID and nowhere else is a common
// setup, and it has said the format is an address.
func TestSAMLReadsTheAddressFromAnEmailNameID(t *testing.T) {
	t.Parallel()

	provider, err := NewSAML(testSAMLOptions(t, config.SAMLProvider{
		ID: "idp", EntityID: "urn:idp", SSOURL: "https://idp.test/sso", Certificate: testCertificatePEM(t),
	}))
	require.NoError(t, err)

	identity, err := provider.identity(&saml.Assertion{Subject: &saml.Subject{NameID: &saml.NameID{
		Format: string(saml.EmailAddressNameIDFormat),
		Value:  "person@example.test",
	}}})
	require.NoError(t, err)

	assert.Equal(t, "urn:idp", identity.Issuer, "the identity key is the provider's entity id")
	assert.Equal(t, "person@example.test", identity.Subject)
	assert.Equal(t, "person@example.test", identity.Email)
	assert.True(t, identity.EmailVerified)
}

// An address outside the domains a provider may speak for is refused, because a
// verified address links a login to an existing admin whichever provider
// asserted it.
func TestSAMLEnforcesAllowedDomains(t *testing.T) {
	t.Parallel()

	provider, err := NewSAML(testSAMLOptions(t, config.SAMLProvider{
		ID: "contractors", EntityID: "urn:idp", SSOURL: "https://idp.test/sso",
		Certificate:    testCertificatePEM(t),
		AllowedDomains: []string{"Partner.Example"},
	}))
	require.NoError(t, err)

	_, err = provider.identity(&saml.Assertion{Subject: &saml.Subject{NameID: &saml.NameID{
		Format: string(saml.EmailAddressNameIDFormat),
		Value:  "staff@example.test",
	}}})
	require.ErrorIs(t, err, ErrSAMLDomainNotAllowed)

	identity, err := provider.identity(&saml.Assertion{Subject: &saml.Subject{NameID: &saml.NameID{
		Format: string(saml.EmailAddressNameIDFormat),
		Value:  "someone@PARTNER.example",
	}}})
	require.NoError(t, err)
	assert.Equal(t, "someone@PARTNER.example", identity.Email, "the allow-list is case-insensitive, the address is untouched")
}

// An assertion naming no address has nothing the exchange could resolve.
func TestSAMLRequiresAnAddress(t *testing.T) {
	t.Parallel()

	provider, err := NewSAML(testSAMLOptions(t, config.SAMLProvider{
		ID: "idp", EntityID: "urn:idp", SSOURL: "https://idp.test/sso", Certificate: testCertificatePEM(t),
	}))
	require.NoError(t, err)

	_, err = provider.identity(&saml.Assertion{Subject: &saml.Subject{NameID: &saml.NameID{
		Format: string(saml.PersistentNameIDFormat),
		Value:  "0oa1b2c3",
	}}})
	require.ErrorIs(t, err, ErrInvalidEmail)
}

// testKeypairPEM mints a throwaway certificate and key. Nothing in these tests
// reaches a network or a real identity provider; the pair exists so that the key
// material the driver refuses or accepts is real key material.
func testKeypairPEM(t *testing.T) (certificatePEM, privateKeyPEM string) {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "test"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(time.Hour),
	}
	raw, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	certificate := pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: raw})
	private := pem.EncodeToMemory(&pem.Block{Type: "RSA PRIVATE KEY", Bytes: x509.MarshalPKCS1PrivateKey(key)})
	return string(certificate), string(private)
}

func testCertificatePEM(t *testing.T) string {
	t.Helper()

	certificate, _ := testKeypairPEM(t)
	return certificate
}
