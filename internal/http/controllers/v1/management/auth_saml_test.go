package v1

import (
	"bytes"
	"compress/flate"
	"context"
	"crypto/rand"
	"crypto/rsa"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/beevik/etree"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/container"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/http/auth/verifiers"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/sso"
	"github.com/lunogram/platform/internal/ssrf"
	"github.com/lunogram/platform/internal/store/management"
	teststore "github.com/lunogram/platform/internal/store/test"
	goredis "github.com/redis/go-redis/v9"
	dsig "github.com/russellhaering/goxmldsig"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"
)

const (
	samlNS      = "urn:oasis:names:tc:SAML:2.0:assertion"
	samlpNS     = "urn:oasis:names:tc:SAML:2.0:protocol"
	statusOK    = "urn:oasis:names:tc:SAML:2.0:status:Success"
	spEntityID  = "https://console.example.test/saml"
	idpEntityID = "urn:lunogram:test:idp"
)

// stubIdP is a SAML identity provider that exists only to sign things. It has a
// key, a certificate and an entity id; it is never dialled, because a provider
// configured with an explicit sign-on URL and certificate needs no metadata
// fetched. What it produces is a real signed assertion, so the driver's
// signature verification is exercised rather than stubbed.
type stubIdP struct {
	key  *rsa.PrivateKey
	cert *x509.Certificate
}

func newStubIdP(t *testing.T) *stubIdP {
	t.Helper()

	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)

	template := x509.Certificate{
		SerialNumber: big.NewInt(1),
		Subject:      pkix.Name{CommonName: "stub-idp"},
		NotBefore:    time.Now().Add(-time.Hour),
		NotAfter:     time.Now().Add(24 * time.Hour),
	}
	raw, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	require.NoError(t, err)

	certificate, err := x509.ParseCertificate(raw)
	require.NoError(t, err)

	return &stubIdP{key: key, cert: certificate}
}

func (idp *stubIdP) certificatePEM() string {
	return string(pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: idp.cert.Raw}))
}

// assertionOptions is what a test varies about the response the stub issues.
// The zero value produces a response the driver accepts.
type assertionOptions struct {
	email        string
	nameID       string
	nameIDFormat string
	inResponseTo string
	assertionID  string
	audience     string
	issuer       string
	destination  string
	notOnOrAfter time.Time
	attributes   map[string]string
	// unsigned leaves the assertion without a signature, which is what an
	// attacker who cannot reach the identity provider's key can produce.
	unsigned bool
	// signedByOther signs with a key the provider never published.
	signedByOther *stubIdP
}

// respond builds and signs a SAML Response and returns it base64 encoded, as
// the HTTP-POST binding carries it.
func (idp *stubIdP) respond(t *testing.T, opts assertionOptions) string {
	t.Helper()

	now := time.Now().UTC()
	if opts.email == "" {
		opts.email = "ada@example.test"
	}
	if opts.nameID == "" {
		opts.nameID = "subject-" + opts.email
	}
	if opts.nameIDFormat == "" {
		opts.nameIDFormat = "urn:oasis:names:tc:SAML:2.0:nameid-format:persistent"
	}
	if opts.assertionID == "" {
		opts.assertionID = "_assertion-" + fmt.Sprint(now.UnixNano())
	}
	if opts.audience == "" {
		opts.audience = spEntityID
	}
	if opts.issuer == "" {
		opts.issuer = idpEntityID
	}
	if opts.notOnOrAfter.IsZero() {
		opts.notOnOrAfter = now.Add(5 * time.Minute)
	}
	if opts.attributes == nil && opts.email != "" {
		opts.attributes = map[string]string{
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress": opts.email,
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname":    "Ada",
			"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname":      "Lovelace",
		}
	}

	assertion := etree.NewElement("saml:Assertion")
	assertion.CreateAttr("xmlns:saml", samlNS)
	assertion.CreateAttr("ID", opts.assertionID)
	assertion.CreateAttr("Version", "2.0")
	assertion.CreateAttr("IssueInstant", now.Format(time.RFC3339))
	assertion.CreateElement("saml:Issuer").SetText(opts.issuer)

	subject := assertion.CreateElement("saml:Subject")
	nameID := subject.CreateElement("saml:NameID")
	nameID.CreateAttr("Format", opts.nameIDFormat)
	nameID.SetText(opts.nameID)

	confirmation := subject.CreateElement("saml:SubjectConfirmation")
	confirmation.CreateAttr("Method", "urn:oasis:names:tc:SAML:2.0:cm:bearer")
	data := confirmation.CreateElement("saml:SubjectConfirmationData")
	data.CreateAttr("InResponseTo", opts.inResponseTo)
	data.CreateAttr("Recipient", opts.destination)
	data.CreateAttr("NotOnOrAfter", opts.notOnOrAfter.Format(time.RFC3339))

	conditions := assertion.CreateElement("saml:Conditions")
	conditions.CreateAttr("NotBefore", now.Add(-time.Minute).Format(time.RFC3339))
	conditions.CreateAttr("NotOnOrAfter", opts.notOnOrAfter.Format(time.RFC3339))
	audience := conditions.CreateElement("saml:AudienceRestriction")
	audience.CreateElement("saml:Audience").SetText(opts.audience)

	if len(opts.attributes) > 0 {
		statement := assertion.CreateElement("saml:AttributeStatement")
		for name, value := range opts.attributes {
			attribute := statement.CreateElement("saml:Attribute")
			attribute.CreateAttr("Name", name)
			attribute.CreateElement("saml:AttributeValue").SetText(value)
		}
	}

	if !opts.unsigned {
		signer := idp
		if opts.signedByOther != nil {
			signer = opts.signedByOther
		}
		assertion = signer.sign(t, assertion)
	}

	response := etree.NewElement("samlp:Response")
	response.CreateAttr("xmlns:samlp", samlpNS)
	response.CreateAttr("xmlns:saml", samlNS)
	response.CreateAttr("ID", "_response-"+fmt.Sprint(now.UnixNano()))
	response.CreateAttr("Version", "2.0")
	response.CreateAttr("IssueInstant", now.Format(time.RFC3339))
	response.CreateAttr("Destination", opts.destination)
	response.CreateAttr("InResponseTo", opts.inResponseTo)
	response.CreateElement("saml:Issuer").SetText(opts.issuer)
	status := response.CreateElement("samlp:Status")
	status.CreateElement("samlp:StatusCode").CreateAttr("Value", statusOK)
	response.AddChild(assertion)

	document := etree.NewDocument()
	document.SetRoot(response)
	raw, err := document.WriteToBytes()
	require.NoError(t, err)

	return base64.StdEncoding.EncodeToString(raw)
}

func (idp *stubIdP) sign(t *testing.T, el *etree.Element) *etree.Element {
	t.Helper()

	context, err := dsig.NewSigningContext(idp.key, [][]byte{idp.cert.Raw})
	require.NoError(t, err)
	context.Canonicalizer = dsig.MakeC14N10ExclusiveCanonicalizerWithPrefixList("")
	require.NoError(t, context.SetSignatureMethod(dsig.RSASHA256SignatureMethod))

	signed, err := context.SignEnveloped(el)
	require.NoError(t, err)
	return signed
}

type samlEnv struct {
	t     *testing.T
	auth  *AuthController
	state *management.State
	idp   *stubIdP
	acs   string
}

func newSAMLEnv(t *testing.T) *samlEnv {
	t.Helper()

	logger := zaptest.NewLogger(t)
	mgmtDB, _, _ := teststore.RunPostgreSQL(t)
	state := management.NewState(mgmtDB)
	engine := rbac.NewTestEngine(t)
	idp := newStubIdP(t)

	options, err := goredis.ParseURL(container.RunRedis(t))
	require.NoError(t, err)
	rdb := goredis.NewClient(options)
	t.Cleanup(func() { rdb.Close() })

	cfg := config.Node{
		PublicURL: "https://console.example.test",
		Redis:     config.Redis{KeyPrefix: "test:" + t.Name() + ":"},
		Auth: config.Auth{
			Drivers: []string{verifiers.SAMLDriver},
			SAML: config.SAMLAuth{
				EntityID: spEntityID,
				Provider: config.SAMLProvider{
					ID:          testProviderID,
					EntityID:    idpEntityID,
					SSOURL:      "https://idp.example.test/sso",
					Certificate: idp.certificatePEM(),
				},
			},
		},
	}

	controller, err := NewAuthController(logger, mgmtDB, state, cfg, engine, consoleSignerFor(t), nil, nil, nil,
		verifiers.Deps{
			SAMLFlows:    sso.NewSAMLFlowStore(rdb, cfg.Redis.KeyPrefix),
			Assertions:   sso.NewAssertionReplayStore(rdb, cfg.Redis.KeyPrefix),
			SAMLMetadata: sso.NewSAMLMetadata(http.DefaultClient, ssrf.Policy{}, 0),
			BaseURL:      cfg.PublicBaseURL(),
		})
	require.NoError(t, err)

	return &samlEnv{
		t:     t,
		auth:  controller,
		state: state,
		idp:   idp,
		acs:   "https://console.example.test/api/auth/saml/" + testProviderID + "/acs",
	}
}

// begin runs the start endpoint and returns the RelayState the deployment
// minted, the AuthnRequest ID it asked the provider to answer, and the binding
// cookie it set on the browser.
func (env *samlEnv) begin(redirect string) (relayState, requestID string, binding *http.Cookie) {
	return env.beginCarrying(redirect, nil)
}

func (env *samlEnv) beginCarrying(redirect string, carried *http.Cookie) (relayState, requestID string, binding *http.Cookie) {
	env.t.Helper()

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/saml/"+testProviderID+"/start", nil)
	if carried != nil {
		req.AddCookie(carried)
	}

	params := oapi.StartSAMLLoginParams{}
	if redirect != "" {
		params.R = &redirect
	}
	env.auth.StartSAMLLogin(res, req, testProviderID, params)
	require.Equal(env.t, http.StatusFound, res.Code, res.Body.String())

	target, err := url.Parse(res.Header().Get("Location"))
	require.NoError(env.t, err)

	query := target.Query()
	relayState = query.Get("RelayState")
	require.NotEmpty(env.t, relayState)

	requestID = authnRequestID(env.t, query.Get("SAMLRequest"))

	for _, cookie := range res.Result().Cookies() {
		if cookie.Name == auth.SAMLBindingCookie && cookie.Value != "" {
			binding = cookie
			assert.Equal(env.t, http.SameSiteNoneMode, cookie.SameSite,
				"the browser returns by cross-site POST, which a Lax cookie is not sent on")
			assert.True(env.t, cookie.Secure, "SameSite=None is only accepted on a Secure cookie")
			assert.True(env.t, cookie.HttpOnly)
		}
	}
	if binding == nil && carried != nil {
		// Nothing new was set because the browser's existing binding was
		// reused, which is the point of carrying one.
		binding = carried
	}
	require.NotNil(env.t, binding, "the start endpoint binds the login to this browser")

	return relayState, requestID, binding
}

// authnRequestID inflates the AuthnRequest the redirect binding carries and
// reads its ID, exactly as the identity provider on the other end would.
func authnRequestID(t *testing.T, encoded string) string {
	t.Helper()

	compressed, err := base64.StdEncoding.DecodeString(encoded)
	require.NoError(t, err)

	raw, err := io.ReadAll(flate.NewReader(bytes.NewReader(compressed)))
	require.NoError(t, err)

	document := etree.NewDocument()
	require.NoError(t, document.ReadFromBytes(raw))
	return document.Root().SelectAttrValue("ID", "")
}

// acsPost runs the assertion consumer service with a form body, carrying the
// binding cookie the browser was given.
func (env *samlEnv) acsPost(response, relayState string, binding *http.Cookie) *httptest.ResponseRecorder {
	env.t.Helper()

	form := url.Values{"SAMLResponse": {response}, "RelayState": {relayState}}
	res := httptest.NewRecorder()
	req := httptest.NewRequest("POST", "/api/auth/saml/"+testProviderID+"/acs", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	if binding != nil {
		req.AddCookie(binding)
	}

	env.auth.CompleteSAMLLogin(res, req, testProviderID)
	return res
}

// signIn runs a whole login: start, have the stub issue a response answering
// it, and complete.
func (env *samlEnv) signIn(opts assertionOptions) *httptest.ResponseRecorder {
	env.t.Helper()

	relayState, requestID, binding := env.begin("")
	opts.inResponseTo = requestID
	opts.destination = env.acs

	return env.acsPost(env.idp.respond(env.t, opts), relayState, binding)
}

func TestSAMLLogin(t *testing.T) {
	t.Parallel()
	env := newSAMLEnv(t)

	t.Run("a proved assertion becomes a console session", func(t *testing.T) {
		res := env.signIn(assertionOptions{email: "ada@example.test"})
		assertLandedInConsole(t, res, "/")

		admin, err := env.state.ResolveAdminByEmail(context.Background(), "ada@example.test")
		require.NoError(t, err)

		identity, err := env.state.GetAdminIdentity(context.Background(), idpEntityID, "subject-ada@example.test")
		require.NoError(t, err)
		assert.Equal(t, admin.ID, identity.AdminID)
		assert.Equal(t, management.IdentityProviderSAML, identity.Provider,
			"the identity is keyed by the provider's entity id and the NameID")
	})

	t.Run("a redirect the login started with is honoured", func(t *testing.T) {
		relayState, requestID, binding := env.begin("/campaigns?tab=sent")
		response := env.idp.respond(t, assertionOptions{
			email: "deep@example.test", inResponseTo: requestID, destination: env.acs,
		})
		assertLandedInConsole(t, env.acsPost(response, relayState, binding), "/campaigns?tab=sent")
	})

	t.Run("the post-login redirect is reduced to a same-site path", func(t *testing.T) {
		relayState, requestID, binding := env.begin("https://evil.test/steal")
		response := env.idp.respond(t, assertionOptions{
			email: "bounce@example.test", inResponseTo: requestID, destination: env.acs,
		})
		assertLandedInConsole(t, env.acsPost(response, relayState, binding), "/")
	})
}

func TestSAMLLoginRefusals(t *testing.T) {
	t.Parallel()
	env := newSAMLEnv(t)

	t.Run("an unsigned assertion is refused", func(t *testing.T) {
		res := env.signIn(assertionOptions{email: "unsigned@example.test", unsigned: true})
		assertRejected(t, res, "failed")
	})

	t.Run("an assertion signed by another key is refused", func(t *testing.T) {
		res := env.signIn(assertionOptions{
			email:         "forged@example.test",
			signedByOther: newStubIdP(t),
		})
		assertRejected(t, res, "failed")
	})

	t.Run("an assertion for another audience is refused", func(t *testing.T) {
		res := env.signIn(assertionOptions{email: "elsewhere@example.test", audience: "https://other.test/saml"})
		assertRejected(t, res, "failed")
	})

	t.Run("an assertion from another issuer is refused", func(t *testing.T) {
		res := env.signIn(assertionOptions{email: "stranger@example.test", issuer: "urn:someone:else"})
		assertRejected(t, res, "failed")
	})

	t.Run("an expired assertion is refused", func(t *testing.T) {
		res := env.signIn(assertionOptions{
			email:        "stale@example.test",
			notOnOrAfter: time.Now().Add(-time.Hour),
		})
		assertRejected(t, res, "failed")
	})

	t.Run("a transient NameID is refused rather than provisioning a new admin each login", func(t *testing.T) {
		res := env.signIn(assertionOptions{
			email:        "transient@example.test",
			nameIDFormat: "urn:oasis:names:tc:SAML:2.0:nameid-format:transient",
		})
		assertRejected(t, res, "transient")
	})

	t.Run("an assertion carrying no address is refused", func(t *testing.T) {
		res := env.signIn(assertionOptions{
			email:      "nobody@example.test",
			attributes: map[string]string{"urn:oid:2.5.4.42": "Ada"},
		})
		assertRejected(t, res, "email")
	})

	// An unsolicited response has no flow to redeem, so it bypasses the
	// RelayState, the browser binding and InResponseTo alike.
	t.Run("an identity-provider-initiated response is refused", func(t *testing.T) {
		response := env.idp.respond(t, assertionOptions{
			email: "unsolicited@example.test", destination: env.acs,
		})
		assertRejected(t, env.acsPost(response, "", nil), "expired")
	})

	// The state a browser carries is a bearer value, so a response answering a
	// login started elsewhere must not open a session here.
	t.Run("a response answering another browser's login is refused", func(t *testing.T) {
		relayState, requestID, _ := env.begin("")
		response := env.idp.respond(t, assertionOptions{
			email: "victim@example.test", inResponseTo: requestID, destination: env.acs,
		})

		other := &http.Cookie{Name: auth.SAMLBindingCookie, Value: "a-binding-from-somewhere-else"}
		assertRejected(t, env.acsPost(response, relayState, other), "expired")
	})

	t.Run("a response answering a different request is refused", func(t *testing.T) {
		relayState, _, binding := env.begin("")
		response := env.idp.respond(t, assertionOptions{
			email: "mismatched@example.test", inResponseTo: "id-never-issued", destination: env.acs,
		})
		assertRejected(t, env.acsPost(response, relayState, binding), "failed")
	})

	t.Run("a response sent to another destination is refused", func(t *testing.T) {
		relayState, requestID, binding := env.begin("")
		response := env.idp.respond(t, assertionOptions{
			email: "misdirected@example.test", inResponseTo: requestID,
			destination: "https://console.example.test/api/auth/saml/other/acs",
		})
		assertRejected(t, env.acsPost(response, relayState, binding), "failed")
	})

	// The RelayState is single use, so the second attempt finds no flow.
	t.Run("a relay state cannot be redeemed twice", func(t *testing.T) {
		relayState, requestID, binding := env.begin("")
		response := env.idp.respond(t, assertionOptions{
			email: "once@example.test", inResponseTo: requestID, destination: env.acs,
		})

		assertLandedInConsole(t, env.acsPost(response, relayState, binding), "/")
		assertRejected(t, env.acsPost(response, relayState, binding), "expired")
	})

	// The identity provider does not sign RelayState, so a captured assertion
	// could otherwise be replayed inside a login the attacker starts
	// themselves, carrying a RelayState that has never been spent.
	t.Run("an assertion cannot be replayed under a fresh relay state", func(t *testing.T) {
		relayState, requestID, binding := env.begin("")
		response := env.idp.respond(t, assertionOptions{
			email: "replayed@example.test", inResponseTo: requestID, destination: env.acs,
			assertionID: "_assertion-replay-fixture",
		})
		assertLandedInConsole(t, env.acsPost(response, relayState, binding), "/")

		// A second login, started honestly, carrying the captured assertion.
		// Everything about the flow is fresh; only the assertion is not.
		second, secondRequest, secondBinding := env.begin("")
		replayed := env.idp.respond(t, assertionOptions{
			email: "replayed@example.test", inResponseTo: secondRequest, destination: env.acs,
			assertionID: "_assertion-replay-fixture",
		})
		assertRejected(t, env.acsPost(replayed, second, secondBinding), "expired")
	})
}

// A browser may hold several logins at once -- two tabs, or a retry after a back
// button -- and they share one binding, so starting a second must not invalidate
// the first.
func TestSAMLBindingIsSharedAcrossFlows(t *testing.T) {
	t.Parallel()
	env := newSAMLEnv(t)

	first, firstRequest, binding := env.begin("")
	second, secondRequest, carried := env.beginCarrying("", binding)
	assert.Equal(t, binding.Value, carried.Value, "a second login reuses the browser's binding")

	response := env.idp.respond(t, assertionOptions{
		email: "tabs@example.test", inResponseTo: secondRequest, destination: env.acs,
	})
	assertLandedInConsole(t, env.acsPost(response, second, carried), "/")

	// The first tab still completes, with its own assertion.
	response = env.idp.respond(t, assertionOptions{
		email: "tabs@example.test", inResponseTo: firstRequest, destination: env.acs,
	})
	assertLandedInConsole(t, env.acsPost(response, first, binding), "/")
}

// The metadata an operator registers is published rather than described in
// prose, so the entity id, the assertion consumer service URL and the
// certificate cannot drift from what the deployment actually uses.
func TestSAMLServiceProviderMetadata(t *testing.T) {
	t.Parallel()
	env := newSAMLEnv(t)

	res := httptest.NewRecorder()
	req := httptest.NewRequest("GET", "/api/auth/saml/"+testProviderID+"/metadata", nil)
	env.auth.GetSAMLMetadata(res, req, testProviderID)

	require.Equal(t, http.StatusOK, res.Code, res.Body.String())
	assert.Equal(t, "application/samlmetadata+xml", res.Header().Get("Content-Type"))

	document := res.Body.String()
	assert.Contains(t, document, spEntityID)
	assert.Contains(t, document, env.acs)
}
