package verifiers

import (
	"context"
	"crypto"
	"crypto/subtle"
	"crypto/tls"
	"crypto/x509"
	"encoding/base64"
	"encoding/xml"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/crewjam/saml"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/sso"
	"github.com/lunogram/platform/internal/store/management"
	dsig "github.com/russellhaering/goxmldsig"
	"go.uber.org/zap"
)

// SAMLDriver is the driver name the deployment's SAML login is offered under.
const SAMLDriver = "saml"

// SAMLACSPath is the assertion consumer service: where the identity provider
// posts its response, with the provider's id in place of %s. It is appended to
// the deployment's public URL and is what an operator registers.
const SAMLACSPath = "/api/auth/saml/%s/acs"

// SAMLMetadataPath is where this deployment publishes its own service provider
// metadata, so an operator can hand a URL to their identity provider instead of
// copying fields between two screens.
const SAMLMetadataPath = "/api/auth/saml/%s/metadata"

// maxSAMLResponseBytes caps the form body at the assertion consumer service.
// The endpoint is unauthenticated by construction -- proving the credential is
// what it does -- so it must not read an arbitrary body into memory.
const maxSAMLResponseBytes = 4 << 20

var (
	// ErrSAMLNotConfigured reports that AUTH_DRIVER names saml without the
	// settings a login cannot run without.
	ErrSAMLNotConfigured = errors.New("the saml auth driver needs an identity provider's entity id and either its metadata url or its sign-on url and certificate: set AUTH_SAML_ENTITY_ID with AUTH_SAML_METADATA_URL, or with AUTH_SAML_SSO_URL and AUTH_SAML_CERTIFICATE")
	// ErrSAMLFlowInvalid reports a response whose RelayState was never issued,
	// has expired, has already been redeemed, or reached a browser other than
	// the one the login was started in.
	ErrSAMLFlowInvalid = errors.New("the login request has expired or was already used")
	// ErrSAMLProviderDenied reports that the identity provider itself refused
	// the authentication request.
	ErrSAMLProviderDenied = errors.New("the identity provider refused the login")
	// ErrSAMLDomainNotAllowed reports that the provider asserted an address in a
	// domain it is not configured to speak for.
	ErrSAMLDomainNotAllowed = errors.New("this provider may not sign in that email domain")
	// ErrSAMLInsecureBaseURL reports a deployment whose public URL is not https.
	// The browser binding has to be SameSite=None to survive the POST binding,
	// and a SameSite=None cookie must be Secure, so there is no binding to be
	// had over plaintext.
	ErrSAMLInsecureBaseURL = errors.New("the saml auth driver requires an https PUBLIC_URL: the browser binding is a SameSite=None cookie, which browsers refuse over plain http")
	// ErrSAMLTransientNameID reports a NameID this deployment cannot use as an
	// identity.
	ErrSAMLTransientNameID = errors.New("the identity provider asserted a transient NameID, which names a session rather than a person")
	// ErrSAMLKeypairIncomplete reports one half of a configured key pair.
	ErrSAMLKeypairIncomplete = errors.New("set both AUTH_SAML_SP_CERTIFICATE and AUTH_SAML_SP_PRIVATE_KEY, or neither")
)

// samlEmailAttributes are the attribute names an email address is looked for
// under when the operator named none.
//
// SAML has no equivalent of OpenID Connect's standard claim names: an address
// arrives under a WS-Federation claim URI from Entra and ADFS, an X.500 object
// identifier from Shibboleth and most academic providers, or a bare word from
// almost everything else. Trying the known ones in order is what makes the
// common provider work unconfigured; anything unusual sets email_attribute.
var samlEmailAttributes = []string{
	"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/emailaddress",
	"urn:oid:0.9.2342.19200300.100.1.3",
	"urn:oasis:names:tc:SAML:attribute:subject-id",
	"email",
	"emailAddress",
	"mail",
}

var samlGivenNameAttributes = []string{
	"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/givenname",
	"urn:oid:2.5.4.42",
	"givenName",
	"firstName",
	"first_name",
}

var samlFamilyNameAttributes = []string{
	"http://schemas.xmlsoap.org/ws/2005/05/identity/claims/surname",
	"urn:oid:2.5.4.4",
	"surname",
	"lastName",
	"last_name",
	"sn",
}

// SAMLKeypair is this deployment's own certificate and key, as published in its
// service provider metadata. It signs AuthnRequests and decrypts encrypted
// assertions.
type SAMLKeypair struct {
	Certificate *x509.Certificate
	Key         crypto.Signer
}

// ParseSAMLKeypair reads the deployment's PEM certificate and private key.
// Empty on both sides means no key pair, which is a deployment whose providers
// neither want signed requests nor encrypt their assertions.
func ParseSAMLKeypair(certificatePEM, privateKeyPEM string) (*SAMLKeypair, error) {
	certificatePEM = strings.TrimSpace(certificatePEM)
	privateKeyPEM = strings.TrimSpace(privateKeyPEM)

	if certificatePEM == "" && privateKeyPEM == "" {
		return nil, nil
	}
	if certificatePEM == "" || privateKeyPEM == "" {
		return nil, ErrSAMLKeypairIncomplete
	}

	// Loaded as a pair rather than separately, so a certificate that does not
	// belong to the key is refused at boot instead of producing signatures no
	// identity provider can verify.
	pair, err := tls.X509KeyPair([]byte(certificatePEM), []byte(privateKeyPEM))
	if err != nil {
		return nil, fmt.Errorf("the saml service provider key pair could not be read: %w", err)
	}

	leaf, err := x509.ParseCertificate(pair.Certificate[0])
	if err != nil {
		return nil, fmt.Errorf("the saml service provider certificate could not be read: %w", err)
	}

	signer, ok := pair.PrivateKey.(crypto.Signer)
	if !ok {
		return nil, errors.New("the saml service provider private key cannot sign")
	}
	return &SAMLKeypair{Certificate: leaf, Key: signer}, nil
}

// SAMLOptions are the collaborators a SAML login needs.
type SAMLOptions struct {
	Config config.SAMLProvider
	// EntityID is this deployment's own entity id, shared by every provider.
	// Empty means the provider's metadata URL, which is what most identity
	// providers expect.
	EntityID string
	// Keypair is this deployment's own key material, or nil.
	Keypair *SAMLKeypair
	// Flows holds outstanding authentication requests; Assertions records the
	// ones already redeemed. Both are Redis-backed because a login starts and
	// finishes on different replicas.
	Flows      *sso.SAMLFlowStore
	Assertions *sso.AssertionReplayStore
	// Metadata resolves and caches a provider's published metadata. It is
	// unused by a provider configured with explicit fields.
	Metadata *sso.SAMLMetadata
	// BaseURL is the deployment's public URL. The assertion consumer service
	// URL derives from it and never from a request parameter, so an open
	// redirect in the console cannot be turned into a way to have assertions
	// delivered somewhere else.
	BaseURL string
	Logger  *zap.Logger
}

// SAML proves a credential issued by one of the deployment's SAML identity
// providers.
//
// Like every other verifier it stops at the proof: it resolves no admin,
// creates no organization and mints no session. What it adds over the others is
// the request half of the exchange -- [SAML.Authorize] mints the RelayState,
// AuthnRequest ID and browser binding that [SAML.Complete] later checks the
// response against -- because a redirect-based credential is only provable if
// the same component issued the request.
type SAML struct {
	config config.SAMLProvider
	// descriptor is the identity provider's metadata when the operator
	// configured it by hand. It is nil when a metadata URL was configured
	// instead, and the document is then resolved per login through metadata.
	descriptor *saml.EntityDescriptor
	metadata   *sso.SAMLMetadata

	domains      []string
	trustEmail   bool
	signRequests bool

	entityID   string
	keypair    *SAMLKeypair
	flows      *sso.SAMLFlowStore
	assertions *sso.AssertionReplayStore
	baseURL    string
	logger     *zap.Logger
}

// NewSAML builds one provider's login, or refuses.
//
// A provider that is named without being configured is refused here rather than
// at the first login: the failure is a variable nobody set, and it should read
// that way instead of as "single sign-on is broken".
func NewSAML(opts SAMLOptions) (*SAML, error) {
	settings := withSAMLProviderDefaults(opts.Config)

	if settings.EntityID == "" {
		return nil, ErrSAMLNotConfigured
	}
	if settings.MetadataURL == "" && (settings.SSOURL == "" || settings.Certificate == "") {
		return nil, ErrSAMLNotConfigured
	}
	// Both forms name the same three things by different routes and there is no
	// order in which one obviously wins, so the ambiguity is refused rather than
	// resolved. This mirrors the AUTH_SAML_* / auth.saml.providers rule one
	// level up.
	if settings.MetadataURL != "" && (settings.SSOURL != "" || settings.Certificate != "") {
		return nil, fmt.Errorf("%w: configure metadata_url, or sso_url with certificate, not both", ErrSAMLNotConfigured)
	}
	if missing := missingSAMLCollaborators(opts); len(missing) > 0 {
		return nil, fmt.Errorf("the saml auth driver cannot be built without %s", strings.Join(missing, ", "))
	}

	baseURL := strings.TrimRight(opts.BaseURL, "/")
	if !strings.HasPrefix(strings.ToLower(baseURL), "https://") {
		return nil, ErrSAMLInsecureBaseURL
	}

	// An allow-list that normalises away to nothing would read as "any domain",
	// which is the opposite of what somebody who configured one meant.
	domains := normalisedDomains(settings.AllowedDomains)
	if len(settings.AllowedDomains) > 0 && len(domains) == 0 {
		return nil, fmt.Errorf("%w: allowed_domains is configured but names no domain", ErrSAMLNotConfigured)
	}

	provider := &SAML{
		config:     settings,
		metadata:   opts.Metadata,
		domains:    domains,
		trustEmail: settings.TrustEmail == nil || *settings.TrustEmail,
		entityID:   opts.EntityID,
		keypair:    opts.Keypair,
		flows:      opts.Flows,
		assertions: opts.Assertions,
		baseURL:    baseURL,
		logger:     opts.Logger,
	}

	// Signed by default whenever there is a key, because a provider that wants
	// signed requests refuses unsigned ones and a provider that does not care
	// ignores the signature. Only an explicit false turns it off.
	provider.signRequests = opts.Keypair != nil && (settings.SignRequests == nil || *settings.SignRequests)

	if settings.MetadataURL != "" {
		if err := opts.Metadata.ValidateMetadataURL(settings.MetadataURL); err != nil {
			return nil, err
		}
		return provider, nil
	}

	descriptor, err := sso.DescriptorFromFields(settings.EntityID, settings.SSOURL, settings.Certificate)
	if err != nil {
		return nil, err
	}
	provider.descriptor = descriptor
	return provider, nil
}

// withSAMLProviderDefaults fills in what the operator left unset. The attribute
// names are deliberately left empty rather than defaulted here, because their
// default is a list of candidates rather than one name; see [samlLookup].
func withSAMLProviderDefaults(settings config.SAMLProvider) config.SAMLProvider {
	settings.ID = strings.TrimSpace(settings.ID)
	settings.Name = strings.TrimSpace(settings.Name)
	settings.EntityID = strings.TrimSpace(settings.EntityID)
	settings.MetadataURL = strings.TrimSpace(settings.MetadataURL)
	settings.SSOURL = strings.TrimSpace(settings.SSOURL)
	settings.NameIDFormat = strings.TrimSpace(settings.NameIDFormat)

	if settings.Name == "" {
		settings.Name = settings.ID
	}
	return settings
}

// missingSAMLCollaborators names what the caller did not supply, so a wiring
// mistake points at the thing that is actually absent.
func missingSAMLCollaborators(opts SAMLOptions) []string {
	var missing []string
	if opts.Flows == nil {
		missing = append(missing, "Redis, which holds outstanding authentication requests")
	}
	if opts.Assertions == nil {
		missing = append(missing, "Redis, which records the assertions already redeemed")
	}
	if opts.Metadata == nil && opts.Config.MetadataURL != "" {
		missing = append(missing, "a metadata cache")
	}
	return missing
}

func (s *SAML) Driver() string { return SAMLDriver }

// ID is how this provider is named in its login URLs.
func (s *SAML) ID() string { return s.config.ID }

// Name is what the login page calls this provider.
func (s *SAML) Name() string { return s.config.Name }

// ACSURL is where the identity provider posts its response. It derives from the
// deployment's public URL and this provider's id, never from a request
// parameter, and it is what the operator registers with the provider.
func (s *SAML) ACSURL() string {
	return s.baseURL + fmt.Sprintf(SAMLACSPath, url.PathEscape(s.config.ID))
}

// MetadataURL is where this deployment publishes its own metadata for this
// provider.
func (s *SAML) MetadataURL() string {
	return s.baseURL + fmt.Sprintf(SAMLMetadataPath, url.PathEscape(s.config.ID))
}

// SAMLAuthorization is a login that has been started.
//
// Exactly one of URL and PostForm is set, because a provider advertises one
// binding or the other. URL is a redirect the browser follows; PostForm is a
// self-submitting HTML form, which is how the HTTP-POST binding sends an
// AuthnRequest.
type SAMLAuthorization struct {
	URL      string
	PostForm []byte
	// Binding is set on the browser as a short-lived cookie. Its twin is held
	// in the flow, and the assertion consumer service refuses a response the
	// two do not agree on.
	Binding string
}

// Authorize begins a login: it stores the AuthnRequest ID and browser binding
// server-side under a fresh RelayState, and returns what to send the browser.
//
// redirect is where the console lands once the session exists. It is validated
// as a same-site path before it is stored, so the response cannot be used to
// bounce somebody off this deployment.
func (s *SAML) Authorize(ctx context.Context, r *http.Request, redirect string) (SAMLAuthorization, error) {
	provider, descriptor, err := s.serviceProvider(ctx)
	if err != nil {
		return SAMLAuthorization{}, err
	}

	relayState, err := sso.NewOpaqueValue()
	if err != nil {
		return SAMLAuthorization{}, err
	}
	// Reused when the browser already carries one, so a second login started in
	// another tab does not invalidate the first.
	binding, err := auth.SAMLBrowserBinding(r, sso.NewOpaqueValue)
	if err != nil {
		return SAMLAuthorization{}, err
	}

	useRedirect := sso.SAMLSupportsRedirectBinding(descriptor)
	outgoing := saml.HTTPPostBinding
	if useRedirect {
		outgoing = saml.HTTPRedirectBinding
	}

	// The response always comes back over HTTP-POST, whichever binding carried
	// the request out: an assertion does not fit in a URL, and the assertion
	// consumer service only reads a form body.
	request, err := provider.MakeAuthenticationRequest(
		provider.GetSSOBindingLocation(outgoing),
		outgoing,
		saml.HTTPPostBinding,
	)
	if err != nil {
		return SAMLAuthorization{}, err
	}

	err = s.flows.Save(ctx, relayState, sso.SAMLFlow{
		ProviderID: s.config.ID,
		RequestID:  request.ID,
		Binding:    binding,
		Redirect:   SafeRedirectPath(redirect),
	})
	if err != nil {
		return SAMLAuthorization{}, err
	}

	if !useRedirect {
		return SAMLAuthorization{PostForm: request.Post(relayState), Binding: binding}, nil
	}

	target, err := request.Redirect(relayState, provider)
	if err != nil {
		return SAMLAuthorization{}, err
	}
	return SAMLAuthorization{URL: target.String(), Binding: binding}, nil
}

// Verify proves the response carried by an assertion consumer service request.
// It is the [auth.Verifier] half of this driver; the redirect the login started
// with is only needed by the browser-facing endpoint, which uses
// [SAML.Complete].
func (s *SAML) Verify(ctx context.Context, r *http.Request) (*auth.VerifiedIdentity, error) {
	identity, _, err := s.Complete(ctx, r)
	return identity, err
}

// Complete redeems the RelayState, proves the response, and returns the
// identity together with the post-login redirect the login started with.
//
// The redirect comes from the STORED flow rather than from the request. The
// request is whatever a browser was pointed at; the flow is what this
// deployment issued, and it is the only thing that says where the person was
// going before they were sent away to authenticate.
func (s *SAML) Complete(ctx context.Context, r *http.Request) (*auth.VerifiedIdentity, string, error) {
	form, err := samlResponseForm(r)
	if err != nil {
		return nil, "", err
	}

	// The flow is redeemed and checked BEFORE the response is looked at, so a
	// RelayState that was never issued, has expired or has already been spent
	// is refused without any of the provider's own content being read.
	flow, err := s.flows.Consume(ctx, form.Get("RelayState"))
	if errors.Is(err, sso.ErrFlowNotFound) {
		return nil, "", ErrSAMLFlowInvalid
	}
	if err != nil {
		return nil, "", err
	}

	redirect := SafeRedirectPath(flow.Redirect)

	// The endpoint that received this is the one registered for THIS provider,
	// so a flow belonging to another was either redeemed at the wrong endpoint
	// or was never issued by us.
	if flow.ProviderID != s.config.ID {
		return nil, redirect, fmt.Errorf("%w: it was started with a different provider", ErrSAMLFlowInvalid)
	}

	// RelayState is a bearer value the identity provider echoes back without
	// signing. Without this check somebody could authenticate as themselves,
	// stop before the response reached us, and replay that POST in another
	// person's browser, whose session would then be the attacker's.
	if subtle.ConstantTimeCompare([]byte(auth.GetSAMLBinding(r)), []byte(flow.Binding)) != 1 {
		return nil, redirect, fmt.Errorf("%w: it was answered in a different browser", ErrSAMLFlowInvalid)
	}

	encoded := form.Get("SAMLResponse")
	if encoded == "" {
		return nil, redirect, ErrSAMLFlowInvalid
	}
	document, err := base64.StdEncoding.DecodeString(encoded)
	if err != nil {
		return nil, redirect, fmt.Errorf("%w: the response is not valid base64", ErrInvalidToken)
	}

	provider, _, err := s.serviceProvider(ctx)
	if err != nil {
		return nil, redirect, err
	}

	// The destination is checked against the deployment's own assertion
	// consumer service URL rather than the URL this request happened to arrive
	// at. Behind a proxy the two differ, and the one derived from PUBLIC_URL is
	// the one the operator registered.
	acs := provider.AcsURL
	assertion, err := provider.ParseXMLResponse(document, []string{flow.RequestID}, acs)
	if err != nil {
		return nil, redirect, s.classify(err)
	}

	if err := s.claimAssertion(ctx, assertion); err != nil {
		return nil, redirect, err
	}

	identity, err := s.identity(assertion)
	if err != nil {
		return nil, redirect, err
	}
	return identity, redirect, nil
}

// claimAssertion records the assertion so it can never be accepted twice.
//
// Redeeming the flow already makes a RelayState single use, but the identity
// provider does not sign RelayState. The assertion is the signed part, so it is
// the thing that has to be single use: otherwise a captured response could be
// replayed inside a login the attacker starts themselves, which would carry a
// RelayState that has never been spent.
func (s *SAML) claimAssertion(ctx context.Context, assertion *saml.Assertion) error {
	until := assertion.IssueInstant.Add(saml.MaxIssueDelay)
	if assertion.Conditions != nil && !assertion.Conditions.NotOnOrAfter.IsZero() {
		until = assertion.Conditions.NotOnOrAfter.Add(saml.MaxClockSkew)
	}

	err := s.assertions.Claim(ctx, assertion.ID, until)
	if errors.Is(err, sso.ErrAssertionReplayed) {
		return fmt.Errorf("%w: the assertion has already been used", ErrSAMLFlowInvalid)
	}
	return err
}

// serviceProvider resolves the identity provider's metadata and builds the
// service provider around it.
func (s *SAML) serviceProvider(ctx context.Context) (*saml.ServiceProvider, *saml.EntityDescriptor, error) {
	descriptor := s.descriptor
	if descriptor == nil {
		fetched, err := s.metadata.Descriptor(ctx, s.config.MetadataURL, s.config.EntityID)
		if err != nil {
			return nil, nil, err
		}
		descriptor = fetched
	}

	acs, err := url.Parse(s.ACSURL())
	if err != nil {
		return nil, nil, err
	}
	metadata, err := url.Parse(s.MetadataURL())
	if err != nil {
		return nil, nil, err
	}

	provider := &saml.ServiceProvider{
		EntityID:    s.entityID,
		AcsURL:      *acs,
		MetadataURL: *metadata,
		IDPMetadata: descriptor,
		// An unsolicited response has no flow to redeem, so it bypasses the
		// RelayState, the browser binding and InResponseTo alike. This
		// deployment starts every login it completes.
		AllowIDPInitiated: false,
	}

	if s.config.NameIDFormat != "" {
		provider.AuthnNameIDFormat = saml.NameIDFormat(s.config.NameIDFormat)
	}
	if s.keypair != nil {
		provider.Certificate = s.keypair.Certificate
		provider.Key = s.keypair.Key
	}
	if s.signRequests {
		provider.SignatureMethod = dsig.RSASHA256SignatureMethod
	}
	return provider, descriptor, nil
}

// Metadata is this deployment's own service provider metadata for this
// provider, as XML.
func (s *SAML) Metadata(ctx context.Context) ([]byte, error) {
	provider, _, err := s.serviceProvider(ctx)
	if err != nil {
		return nil, err
	}
	return xml.MarshalIndent(provider.Metadata(), "", "  ")
}

// classify turns the library's deliberately opaque parse failure into one of
// the handful of things this driver distinguishes.
//
// [saml.InvalidResponseError] says only "Authentication failed" by design, so
// the reason has to be read off the wrapped error. Everything that is not a
// provider-side refusal is reported as an invalid flow, which is what it looks
// like from the login page either way.
func (s *SAML) classify(err error) error {
	var invalid *saml.InvalidResponseError
	if errors.As(err, &invalid) && invalid.PrivateErr != nil {
		err = invalid.PrivateErr
	}

	var status saml.ErrBadStatus
	if errors.As(err, &status) {
		return fmt.Errorf("%w: %s", ErrSAMLProviderDenied, status.Status)
	}
	return fmt.Errorf("%w: %w", ErrInvalidToken, err)
}

// identity turns a proved assertion into an [auth.VerifiedIdentity].
func (s *SAML) identity(assertion *saml.Assertion) (*auth.VerifiedIdentity, error) {
	if assertion.Subject == nil || assertion.Subject.NameID == nil {
		return nil, fmt.Errorf("%w: the assertion names no subject", ErrInvalidToken)
	}

	nameID := assertion.Subject.NameID
	subject := strings.TrimSpace(nameID.Value)
	if subject == "" {
		return nil, fmt.Errorf("%w: the assertion names no subject", ErrInvalidToken)
	}

	// A transient NameID is a per-session pseudonym: the provider mints a new
	// one every login. Storing it as the identity key would provision a fresh
	// admin on every sign-in and leave the person's real account behind, so it
	// is refused rather than silently accepted.
	if nameID.Format == string(saml.TransientNameIDFormat) {
		return nil, ErrSAMLTransientNameID
	}

	attributes := samlAttributes(assertion)

	email := strings.TrimSpace(samlLookup(attributes, s.config.EmailAttribute, samlEmailAttributes))
	if email == "" && nameID.Format == string(saml.EmailAddressNameIDFormat) {
		// A provider configured to put the address in the NameID and nowhere
		// else is a common Okta and ADFS setup, and it has said the format is
		// an address.
		email = subject
	}
	if email == "" {
		return nil, ErrInvalidEmail
	}
	if !s.assertsDomain(email) {
		s.logger.Warn("refusing a saml login for an address outside the provider's allowed domains",
			zap.String("provider", s.config.ID),
			zap.String("domain", emailDomain(email)))
		return nil, ErrSAMLDomainNotAllowed
	}

	return &auth.VerifiedIdentity{
		Issuer:   s.config.EntityID,
		Subject:  subject,
		Provider: management.IdentityProviderSAML,
		Email:    email,
		// SAML has no attribute that attests an address, so the attestation is
		// the operator's: they configured this identity provider and said it
		// speaks for its users. See config.SAMLProvider.TrustEmail.
		EmailVerified: s.trustEmail,
		FirstName:     optionalString(samlLookup(attributes, s.config.GivenNameAttribute, samlGivenNameAttributes)),
		LastName:      optionalString(samlLookup(attributes, s.config.FamilyNameAttribute, samlFamilyNameAttributes)),
	}, nil
}

// assertsDomain reports whether this provider may speak for an address.
func (s *SAML) assertsDomain(email string) bool {
	if len(s.domains) == 0 {
		return true
	}
	return slices.Contains(s.domains, emailDomain(email))
}

// samlAttributes flattens an assertion's attribute statements, keyed by both
// Name and FriendlyName so either can be configured or matched.
//
// The first value wins. A multi-valued attribute is legal and says nothing
// about which value is meant, and picking the last would make the answer depend
// on document order.
func samlAttributes(assertion *saml.Assertion) map[string]string {
	values := make(map[string]string)

	for _, statement := range assertion.AttributeStatements {
		for _, attribute := range statement.Attributes {
			for _, value := range attribute.Values {
				text := strings.TrimSpace(value.Value)
				if text == "" {
					continue
				}
				if attribute.Name != "" {
					if _, seen := values[attribute.Name]; !seen {
						values[attribute.Name] = text
					}
				}
				if attribute.FriendlyName != "" {
					if _, seen := values[attribute.FriendlyName]; !seen {
						values[attribute.FriendlyName] = text
					}
				}
				break
			}
		}
	}
	return values
}

// samlLookup reads the attribute the operator named, or tries the candidates
// this driver knows about.
//
// A configured name is used on its own. Falling back to the candidates when it
// misses would mean a deployment that pointed at a specific attribute silently
// reading a different one, and for the email attribute that is the difference
// between the address the operator vouched for and one the directory happens to
// carry.
func samlLookup(attributes map[string]string, configured string, candidates []string) string {
	if configured = strings.TrimSpace(configured); configured != "" {
		return attributes[configured]
	}
	for _, candidate := range candidates {
		if value := attributes[candidate]; value != "" {
			return value
		}
	}
	return ""
}

// samlResponseForm reads the assertion consumer service's form body under a cap.
//
// r.ParseForm is deliberately not used: it reads the body without a limit, and
// this endpoint is unauthenticated by construction.
func samlResponseForm(r *http.Request) (url.Values, error) {
	if r.Body == nil {
		return nil, ErrSAMLFlowInvalid
	}

	body, err := io.ReadAll(io.LimitReader(r.Body, maxSAMLResponseBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxSAMLResponseBytes {
		return nil, fmt.Errorf("%w: the response exceeds %d bytes", ErrInvalidToken, maxSAMLResponseBytes)
	}

	form, err := url.ParseQuery(string(body))
	if err != nil {
		return nil, ErrSAMLFlowInvalid
	}
	return form, nil
}

func optionalString(value string) *string {
	if value = strings.TrimSpace(value); value == "" {
		return nil
	}
	return &value
}
