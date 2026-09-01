package verifiers

import (
	"context"
	"crypto/subtle"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"slices"
	"strings"

	"github.com/coreos/go-oidc/v3/oidc"
	"github.com/golang-jwt/jwt/v5"
	"github.com/lunogram/platform/internal/config"
	"github.com/lunogram/platform/internal/http/auth"
	"github.com/lunogram/platform/internal/jwks"
	"github.com/lunogram/platform/internal/sso"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
	"golang.org/x/oauth2"
)

// OIDCDriver is the driver name the deployment's OpenID Connect login is
// offered under. Like every other driver it is selected with AUTH_DRIVER and
// configured from the environment.
const OIDCDriver = "oidc"

// OIDCCallbackPath is where an identity provider returns the browser to, with
// the provider's id in place of %s. It is appended to the deployment's public
// URL and is what an operator registers with the provider.
const OIDCCallbackPath = "/api/auth/oidc/%s/callback"

var (
	// ErrOIDCNotConfigured reports that AUTH_DRIVER names oidc without the
	// settings a federated login cannot run without.
	ErrOIDCNotConfigured = errors.New("the oidc auth driver needs an issuer, a client id and a client secret: set AUTH_OIDC_ISSUER, AUTH_OIDC_CLIENT_ID and AUTH_OIDC_CLIENT_SECRET, or issuer, client_id and client_secret on the provider under auth.oidc.providers")
	// ErrOIDCFlowInvalid reports a callback whose state was never issued, has
	// expired, has already been redeemed, or reached a browser other than the
	// one the login was started in.
	ErrOIDCFlowInvalid = errors.New("the login request has expired or was already used")
	// ErrOIDCProviderDenied reports that the identity provider itself refused
	// the authorization request.
	ErrOIDCProviderDenied = errors.New("the identity provider refused the login")
	// ErrOIDCDomainNotAllowed reports that the provider asserted an address in a
	// domain it is not configured to speak for.
	ErrOIDCDomainNotAllowed = errors.New("this provider may not sign in that email domain")
)

// maxUserInfoBytes caps a UserInfo response, so a misbehaving or hostile
// endpoint cannot be read into memory without bound.
const maxUserInfoBytes = 1 << 20

// idTokenSigningMethods is the set of signature algorithms an ID token may be
// signed with. It is an allow-list rather than "whatever the issuer advertises"
// so a provider announcing a symmetric or "none" algorithm cannot talk us into
// verifying a token against something that is not a signature.
var idTokenSigningMethods = []string{
	"RS256", "RS384", "RS512",
	"ES256", "ES384", "ES512",
	"PS256", "PS384", "PS512",
}

// OIDCOptions are the collaborators a federated login needs.
type OIDCOptions struct {
	Config    config.OIDCProvider
	Flows     *sso.FlowStore
	Discovery *sso.Discovery
	// Keys resolves the issuer's JWKS. It is the platform's existing two-tier
	// cache: rotation, coalescing and negative caching are already solved there,
	// and go-oidc's own remote key set would be a second answer to the same
	// question.
	Keys *jwks.Cache
	// BaseURL is the deployment's public URL. The redirect_uri is derived from
	// it and never from a request parameter, so an open redirect in the console
	// cannot be turned into a way to have authorization codes delivered
	// somewhere else.
	BaseURL string
	// HTTPClient talks to the identity provider's token endpoint. Nil is not
	// accepted: the caller decides the SSRF policy.
	HTTPClient *http.Client
	Logger     *zap.Logger
}

// NewOIDC builds one provider's federated login.
//
// A provider that is named without being configured is refused here rather than
// at the first login: the failure is a variable nobody set, and it should read
// that way instead of as "single sign-on is broken".
func NewOIDC(opts OIDCOptions) (*OIDC, error) {
	settings := withProviderDefaults(opts.Config)

	if settings.Issuer == "" || settings.ClientID == "" || settings.ClientSecret == "" {
		return nil, ErrOIDCNotConfigured
	}
	if missing := missingCollaborators(opts); len(missing) > 0 {
		return nil, fmt.Errorf("the oidc auth driver cannot be built without %s", strings.Join(missing, ", "))
	}
	if err := opts.Discovery.ValidateDiscoveryURL(settings.DiscoveryURL, settings.Issuer); err != nil {
		return nil, err
	}

	// An allow-list that normalises away to nothing would read as "any domain",
	// which is the opposite of what somebody who configured one meant. A typo or
	// an environment reference that expanded to nothing must not silently take
	// the boundary off.
	domains := normalisedDomains(settings.AllowedDomains)
	if len(settings.AllowedDomains) > 0 && len(domains) == 0 {
		return nil, fmt.Errorf("%w: allowed_domains is configured but names no domain", ErrOIDCNotConfigured)
	}

	return &OIDC{
		config:    settings,
		scopes:    withOpenIDScope(settings.Scopes),
		domains:   domains,
		flows:     opts.Flows,
		discovery: opts.Discovery,
		keys:      opts.Keys,
		baseURL:   strings.TrimRight(opts.BaseURL, "/"),
		client:    opts.HTTPClient,
		logger:    opts.Logger,
	}, nil
}

// withProviderDefaults fills in what the operator left unset.
//
// The claim defaults live here rather than in [config.Defaults] because they are
// per provider, and a deployment listing several would otherwise get them on
// none of them.
func withProviderDefaults(settings config.OIDCProvider) config.OIDCProvider {
	settings.ID = strings.TrimSpace(settings.ID)
	settings.Name = strings.TrimSpace(settings.Name)
	settings.Issuer = strings.TrimSpace(settings.Issuer)
	settings.DiscoveryURL = strings.TrimSpace(settings.DiscoveryURL)

	if settings.Name == "" {
		settings.Name = settings.ID
	}
	if len(settings.Scopes) == 0 {
		settings.Scopes = []string{"openid", "email", "profile"}
	}
	if settings.EmailClaim == "" {
		settings.EmailClaim = "email"
	}
	if settings.GivenNameClaim == "" {
		settings.GivenNameClaim = "given_name"
	}
	if settings.FamilyNameClaim == "" {
		settings.FamilyNameClaim = "family_name"
	}

	// email_verified attests the standard email claim and nothing else. Reading
	// it for an address taken from somewhere else would let a token carry a
	// verified address the login ignores alongside an editable one it uses --
	// preferred_username, say -- and the exchange links accounts by the address
	// it was handed.
	if settings.EmailVerifiedClaim == "" && settings.EmailClaim == "email" {
		settings.EmailVerifiedClaim = "email_verified"
	}

	if settings.DiscoveryURL == "" && settings.Issuer != "" {
		settings.DiscoveryURL = sso.DefaultDiscoveryURL(settings.Issuer)
	}
	return settings
}

// normalisedDomains lower-cases the allow-list once, so the check on every login
// is a comparison rather than a conversion.
func normalisedDomains(domains []string) []string {
	if len(domains) == 0 {
		return nil
	}
	normalised := make([]string, 0, len(domains))
	for _, domain := range domains {
		if domain = strings.ToLower(strings.TrimSpace(domain)); domain != "" {
			normalised = append(normalised, domain)
		}
	}
	return normalised
}

// missingCollaborators names what the caller did not supply, so a wiring
// mistake points at the thing that is actually absent.
func missingCollaborators(opts OIDCOptions) []string {
	var missing []string
	if opts.Flows == nil {
		missing = append(missing, "Redis, which holds outstanding authorization requests")
	}
	if opts.Keys == nil {
		missing = append(missing, "a JWKS cache")
	}
	if opts.Discovery == nil {
		missing = append(missing, "a discovery cache")
	}
	if opts.HTTPClient == nil {
		missing = append(missing, "an HTTP client")
	}
	return missing
}

// emailDomain is the part of an address an allow-list matches on, lower-cased.
// An address with no "@" has no domain and matches nothing.
func emailDomain(email string) string {
	at := strings.LastIndex(email, "@")
	if at < 0 {
		return ""
	}
	return strings.ToLower(strings.TrimSpace(email[at+1:]))
}

// OIDC proves a credential issued by the deployment's OpenID Connect provider.
//
// Like every other verifier it stops at the proof: it resolves no admin, creates
// no organization and mints no session. What it adds over the others is the
// challenge half of the exchange -- [OIDC.Authorize] mints the state, nonce,
// PKCE verifier and browser binding that [OIDC.Complete] later checks the
// response against -- because a redirect-based credential is only provable if
// the same component issued the challenge.
type OIDC struct {
	config    config.OIDCProvider
	scopes    []string
	domains   []string
	flows     *sso.FlowStore
	discovery *sso.Discovery
	keys      *jwks.Cache
	baseURL   string
	client    *http.Client
	logger    *zap.Logger
}

func (o *OIDC) Driver() string { return OIDCDriver }

// Authorization is a login that has been started: where to send the browser,
// and the binding value that browser must carry back.
type Authorization struct {
	URL string
	// Binding is set on the browser as a short-lived cookie. Its twin is held in
	// the flow, and the callback refuses a response the two do not agree on.
	Binding string
}

// Authorize begins a login: it stores the state, nonce, PKCE verifier and
// browser binding server-side and returns the URL the browser is sent to.
//
// redirect is where the console lands once the session exists. It is validated
// as a same-site path before it is stored, so the callback cannot be used to
// bounce somebody off this deployment.
//
// r is read for the browser binding this login should be recorded under, and
// for nothing else.
func (o *OIDC) Authorize(ctx context.Context, r *http.Request, redirect string) (Authorization, error) {
	_, oauthConfig, err := o.oauthConfig(ctx)
	if err != nil {
		return Authorization{}, err
	}

	state, err := sso.NewOpaqueValue()
	if err != nil {
		return Authorization{}, err
	}
	nonce, err := sso.NewOpaqueValue()
	if err != nil {
		return Authorization{}, err
	}
	// Reused when the browser already carries one, so a second login started in
	// another tab does not invalidate the first.
	binding, err := auth.BrowserBinding(r, sso.NewOpaqueValue)
	if err != nil {
		return Authorization{}, err
	}
	verifier := oauth2.GenerateVerifier()

	err = o.flows.Save(ctx, state, sso.Flow{
		ProviderID:   o.config.ID,
		Nonce:        nonce,
		CodeVerifier: verifier,
		Binding:      binding,
		Redirect:     SafeRedirectPath(redirect),
	})
	if err != nil {
		return Authorization{}, err
	}

	return Authorization{
		URL: oauthConfig.AuthCodeURL(state,
			oidc.Nonce(nonce),
			oauth2.S256ChallengeOption(verifier),
		),
		Binding: binding,
	}, nil
}

// Verify proves the authorization response carried by a callback request. It is
// the [auth.Verifier] half of this driver; the redirect the login started with
// is only needed by the browser-facing callback, which uses [OIDC.Complete].
func (o *OIDC) Verify(ctx context.Context, r *http.Request) (*auth.VerifiedIdentity, error) {
	identity, _, err := o.Complete(ctx, r)
	return identity, err
}

// Complete redeems the state, exchanges the code with the PKCE verifier, proves
// the ID token, and returns the identity together with the post-login redirect
// the login started with.
//
// The redirect comes from the STORED flow rather than from the callback URL. The
// URL is whatever a browser was pointed at; the flow is what this deployment
// issued, and it is the only thing that says where the person was going before
// they were sent away to authenticate.
func (o *OIDC) Complete(ctx context.Context, r *http.Request) (*auth.VerifiedIdentity, string, error) {
	query := r.URL.Query()

	// The flow is redeemed and checked BEFORE the provider's own error is read.
	// An authorization error carries a state too, so classifying it first would
	// report an unknown or already-spent state as a genuine denial, and would
	// let a response with the wrong browser binding skip that check entirely.
	flow, err := o.flows.Consume(ctx, query.Get("state"))
	if errors.Is(err, sso.ErrFlowNotFound) {
		return nil, "", ErrOIDCFlowInvalid
	}
	if err != nil {
		return nil, "", err
	}

	redirect := SafeRedirectPath(flow.Redirect)

	// The callback that received this is the one registered for THIS provider,
	// so a flow belonging to another was either redeemed at the wrong endpoint
	// or was never issued by us.
	if flow.ProviderID != o.config.ID {
		return nil, redirect, fmt.Errorf("%w: it was started with a different provider", ErrOIDCFlowInvalid)
	}

	// The state is a bearer value: whoever holds the callback URL can present
	// it. Without this check somebody could authenticate as themselves, stop
	// before following the callback, and hand that URL to another person, whose
	// browser would then be given the attacker's session. The binding cookie is
	// what ties the response to the browser the login was started in.
	if subtle.ConstantTimeCompare([]byte(auth.GetOIDCBinding(r)), []byte(flow.Binding)) != 1 {
		return nil, redirect, fmt.Errorf("%w: it was answered in a different browser", ErrOIDCFlowInvalid)
	}

	// Only now, on a response this deployment actually issued and this browser
	// actually started, is the provider's own refusal reported as one.
	if denial := query.Get("error"); denial != "" {
		return nil, redirect, fmt.Errorf("%w: %s", ErrOIDCProviderDenied, denial)
	}

	code := query.Get("code")
	if code == "" {
		return nil, redirect, ErrOIDCFlowInvalid
	}

	metadata, oauthConfig, err := o.oauthConfig(ctx)
	if err != nil {
		return nil, redirect, err
	}

	token, err := oauthConfig.Exchange(
		context.WithValue(ctx, oauth2.HTTPClient, o.client),
		code,
		oauth2.VerifierOption(flow.CodeVerifier),
	)
	if err != nil {
		return nil, redirect, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	rawIDToken, ok := token.Extra("id_token").(string)
	if !ok || rawIDToken == "" {
		return nil, redirect, fmt.Errorf("%w: the token response carried no id_token", ErrInvalidToken)
	}

	idToken, err := o.verifyIDToken(ctx, metadata, rawIDToken)
	if err != nil {
		return nil, redirect, err
	}

	// Constant-time, though the nonce is not a secret an attacker gets to guess
	// interactively: it is compared exactly once per flow and the flow is gone
	// either way.
	if subtle.ConstantTimeCompare([]byte(idToken.Nonce), []byte(flow.Nonce)) != 1 {
		return nil, redirect, fmt.Errorf("%w: the id token's nonce does not match the login it answers", ErrInvalidToken)
	}

	identity, err := o.identity(ctx, metadata, token, idToken)
	if err != nil {
		return nil, redirect, err
	}
	return identity, redirect, nil
}

// verifyIDToken proves the token's signature against the issuer's published
// keys and its iss, aud and exp against the configured provider.
func (o *OIDC) verifyIDToken(ctx context.Context, metadata sso.Metadata, rawIDToken string) (*oidc.IDToken, error) {
	verifier := oidc.NewVerifier(
		o.config.Issuer,
		&cachedKeySet{cache: o.keys, url: metadata.JWKSURI},
		&oidc.Config{
			ClientID:             o.config.ClientID,
			SupportedSigningAlgs: allowedSigningAlgorithms(metadata.SigningAlgorithms),
		},
	)

	idToken, err := verifier.Verify(ctx, rawIDToken)
	if err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}
	return idToken, nil
}

// identity turns a proved ID token into an [auth.VerifiedIdentity].
//
// EmailVerified requires the provider's own claim, because that is the flag the
// exchange links identities by: an identity provider that lets a user type any
// address into their profile must not be able to inherit an existing account by
// claiming its address.
func (o *OIDC) identity(ctx context.Context, metadata sso.Metadata, token *oauth2.Token, idToken *oidc.IDToken) (*auth.VerifiedIdentity, error) {
	var claims map[string]any
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("%w: %w", ErrInvalidToken, err)
	}

	// OpenID Connect lets a provider return the email and profile scopes from
	// UserInfo rather than in the ID token, and several do -- including the
	// attestation on its own, with the address in the token. Without this a
	// compliant provider either fails every login with "no email address" or
	// silently never links to an existing account, however the scopes are set.
	tokenEmail := strings.TrimSpace(stringClaim(claims, o.config.EmailClaim))
	if o.needsUserInfo(claims) && metadata.UserInfoEndpoint != "" {
		profile, err := o.userInfo(ctx, metadata.UserInfoEndpoint, token, idToken.Subject)
		if err != nil {
			return nil, err
		}

		// An attestation is only ever about the address it accompanies.
		//
		// When the ID token named no address, its own email_verified attests
		// nothing and must not be allowed to vouch for whatever UserInfo
		// supplies -- that would hand an unattested address to linkByEmail and
		// let it attach the login to an existing admin. UserInfo has to supply
		// both halves itself.
		//
		// When the ID token did name one, UserInfo has to agree with it before
		// its attestation is allowed to speak.
		profileEmail := strings.TrimSpace(stringClaim(profile, o.config.EmailClaim))
		if tokenEmail == "" {
			delete(claims, o.config.EmailVerifiedClaim)
		} else if !strings.EqualFold(profileEmail, tokenEmail) {
			delete(profile, o.config.EmailVerifiedClaim)
		}
		mergeMissingClaims(claims, profile)
	}

	email := strings.TrimSpace(stringClaim(claims, o.config.EmailClaim))
	if email == "" {
		return nil, ErrInvalidEmail
	}
	if !o.assertsDomain(email) {
		o.logger.Warn("refusing a federated login for an address outside the provider's allowed domains",
			zap.String("provider", o.config.ID),
			zap.String("domain", emailDomain(email)))
		return nil, ErrOIDCDomainNotAllowed
	}

	return &auth.VerifiedIdentity{
		Issuer:        o.config.Issuer,
		Subject:       idToken.Subject,
		Provider:      management.IdentityProviderOIDC,
		Email:         email,
		EmailVerified: o.config.EmailVerifiedClaim != "" && boolClaim(claims, o.config.EmailVerifiedClaim),
		FirstName:     optionalStringClaim(claims, o.config.GivenNameClaim),
		LastName:      optionalStringClaim(claims, o.config.FamilyNameClaim),
	}, nil
}

// needsUserInfo reports whether the ID token left any configured claim for the
// provider to answer at UserInfo instead.
//
// The rule is deliberately uniform across the claims rather than special-casing
// the address: the ID token is the fast path, and whatever it leaves unanswered
// is asked for once. Exempting the names would provision an admin without them
// against a provider that keeps the profile scope at UserInfo, which is a
// configuration OpenID Connect explicitly allows.
//
// A claim that is present and false is an answer, not a gap: the provider said
// the address is unverified, and asking again elsewhere would be looking for a
// better reply.
func (o *OIDC) needsUserInfo(claims map[string]any) bool {
	// Present-and-null is not an answer. A provider that emits
	// "email_verified": null has said nothing, and asking UserInfo is the point
	// of this check.
	wanted := []string{o.config.EmailClaim, o.config.EmailVerifiedClaim, o.config.GivenNameClaim, o.config.FamilyNameClaim}
	for _, name := range wanted {
		if name != "" && !answered(claims[name]) {
			return true
		}
	}
	return false
}

// mergeMissingClaims fills in what the ID token did not carry. A claim the ID
// token already answers keeps its value: the token is signed and UserInfo is a
// bearer-authenticated GET, so the weaker source never overrides the stronger
// one. A claim the ID token left unanswered -- absent, null or empty -- is a gap
// the profile may fill.
func mergeMissingClaims(claims, profile map[string]any) {
	for name, value := range profile {
		if answered(claims[name]) {
			continue
		}
		claims[name] = value
	}
}

// answered reports whether a claim value says anything. A missing key, a null
// and an empty string are all silence; false is an answer, because that is the
// provider saying no rather than saying nothing.
//
// [OIDC.needsUserInfo] uses the same rule, and it has to: reading a null as an
// answer here would fetch UserInfo and then discard what it returned.
func answered(value any) bool {
	switch typed := value.(type) {
	case nil:
		return false
	case string:
		return typed != ""
	default:
		return true
	}
}

// userInfo reads the profile the provider kept out of the ID token.
//
// The response is only usable when its `sub` is the subject the ID token was
// already proved for. Without that check the endpoint would be a way to attach
// one person's profile -- their address, and so their account -- to another
// person's authenticated subject.
func (o *OIDC) userInfo(ctx context.Context, endpoint string, token *oauth2.Token, subject string) (map[string]any, error) {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	token.SetAuthHeader(request)
	request.Header.Set("Accept", "application/json")

	response, err := o.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("oidc: the userinfo endpoint could not be reached: %w", err)
	}
	defer response.Body.Close()

	if response.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("oidc: the userinfo endpoint returned %s", response.Status)
	}

	body, err := io.ReadAll(io.LimitReader(response.Body, maxUserInfoBytes+1))
	if err != nil {
		return nil, err
	}
	if len(body) > maxUserInfoBytes {
		return nil, fmt.Errorf("oidc: the userinfo response exceeds %d bytes", maxUserInfoBytes)
	}

	var claims map[string]any
	if err := json.Unmarshal(body, &claims); err != nil {
		// A provider configured to sign the response answers application/jwt,
		// which is not something this driver reads. Say so rather than
		// presenting it as a missing address.
		return nil, fmt.Errorf("oidc: the userinfo response is not a JSON object: %w", err)
	}

	if subtle.ConstantTimeCompare([]byte(stringClaim(claims, "sub")), []byte(subject)) != 1 {
		return nil, fmt.Errorf("%w: the userinfo response describes a different subject", ErrInvalidToken)
	}
	return claims, nil
}

// assertsDomain reports whether this provider may speak for an address.
//
// An empty allow-list means any, which is the right answer for a deployment
// with one provider. With several it is the boundary between them: a verified
// address links a login to an existing admin whichever provider asserted it, so
// the allow-list is what stops the least trustworthy one reaching every account.
func (o *OIDC) assertsDomain(email string) bool {
	if len(o.domains) == 0 {
		return true
	}
	return slices.Contains(o.domains, emailDomain(email))
}

// ID is how this provider is named in its login URLs.
func (o *OIDC) ID() string { return o.config.ID }

// Name is what the login page calls this provider.
func (o *OIDC) Name() string { return o.config.Name }

// oauthConfig resolves the provider's endpoints and builds the OAuth2 client.
func (o *OIDC) oauthConfig(ctx context.Context) (sso.Metadata, *oauth2.Config, error) {
	metadata, err := o.discovery.Metadata(ctx, o.config.DiscoveryURL, o.config.Issuer)
	if err != nil {
		return sso.Metadata{}, nil, err
	}

	return metadata, &oauth2.Config{
		ClientID:     o.config.ClientID,
		ClientSecret: o.config.ClientSecret,
		Endpoint: oauth2.Endpoint{
			AuthURL:   metadata.AuthorizationEndpoint,
			TokenURL:  metadata.TokenEndpoint,
			AuthStyle: tokenAuthStyle(metadata.TokenAuthMethods),
		},
		RedirectURL: o.RedirectURI(),
		Scopes:      o.scopes,
	}, nil
}

// tokenAuthStyle picks how the client authenticates at the token endpoint from
// what the provider says it accepts.
//
// OpenID Connect Discovery makes client_secret_basic the default when the
// provider publishes nothing, so that is what an absent list means here. Pinning
// one style outright would fail every exchange against a provider that only
// implements the other, and oauth2's auto-detection reaches that answer by
// sending the client secret in a request body first.
func tokenAuthStyle(supported []string) oauth2.AuthStyle {
	if len(supported) == 0 || slices.Contains(supported, "client_secret_basic") {
		return oauth2.AuthStyleInHeader
	}
	if slices.Contains(supported, "client_secret_post") {
		return oauth2.AuthStyleInParams
	}
	// Something we do not implement -- private_key_jwt, say. Let oauth2 probe
	// rather than refuse a provider that may still accept one of the two.
	return oauth2.AuthStyleAutoDetect
}

// RedirectURI is where the identity provider returns the browser to. It is
// derived from the deployment's public URL and this provider's id, never from a
// request parameter, and it is what the operator registers with the provider.
func (o *OIDC) RedirectURI() string {
	return o.baseURL + fmt.Sprintf(OIDCCallbackPath, url.PathEscape(o.config.ID))
}

// withOpenIDScope guarantees the one scope the protocol requires. A deployment
// that lists scopes at all is easy to leave openid out of, and the failure that
// follows -- a token response with no id_token in it -- says nothing about why.
func withOpenIDScope(scopes []string) []string {
	requested := make([]string, 0, len(scopes)+1)
	for _, scope := range scopes {
		if scope = strings.TrimSpace(scope); scope != "" && !slices.Contains(requested, scope) {
			requested = append(requested, scope)
		}
	}
	if !slices.Contains(requested, oidc.ScopeOpenID) {
		requested = append([]string{oidc.ScopeOpenID}, requested...)
	}
	return requested
}

// SafeRedirectPath reduces a post-login redirect to a same-site path. Anything
// absolute, protocol-relative or unparseable becomes "/", so the callback can
// never bounce a freshly authenticated browser off this deployment.
//
// The fragment is carried through. The console preserves it when it builds the
// redirect, so dropping it here would land somebody who signed in from
// /settings#keys on /settings -- and only when they used this driver.
func SafeRedirectPath(redirect string) string {
	if redirect == "" || !strings.HasPrefix(redirect, "/") || strings.HasPrefix(redirect, "//") {
		return "/"
	}
	parsed, err := url.Parse(redirect)
	if err != nil || parsed.IsAbs() || parsed.Host != "" {
		return "/"
	}

	safe := parsed.RequestURI()
	if parsed.Fragment != "" {
		safe += "#" + parsed.EscapedFragment()
	}
	return safe
}

// allowedSigningAlgorithms intersects what the issuer advertises with what the
// platform is willing to verify, falling back to the one algorithm OpenID
// Connect makes mandatory.
func allowedSigningAlgorithms(advertised []string) []string {
	allowed := make([]string, 0, len(advertised))
	for _, alg := range advertised {
		if slices.Contains(idTokenSigningMethods, alg) {
			allowed = append(allowed, alg)
		}
	}
	if len(allowed) == 0 {
		return []string{"RS256"}
	}
	return allowed
}

// cachedKeySet adapts the platform's JWKS cache to go-oidc's key set, so a
// federated login verifies signatures through the same cache every other
// externally-issued JWT goes through.
type cachedKeySet struct {
	cache *jwks.Cache
	url   string
}

func (k *cachedKeySet) VerifySignature(ctx context.Context, rawJWT string) ([]byte, error) {
	keyFunc, err := k.cache.Keyfunc(ctx, k.url)
	if err != nil {
		return nil, err
	}

	payload, err := verifySignedPayload(rawJWT, keyFunc)
	if err == nil {
		return payload, nil
	}

	// A key id the cached set does not know usually means the issuer has just
	// rotated. Refresh once and try again; a second failure is a real one.
	refreshed, refreshErr := k.cache.Refresh(ctx, k.url)
	if refreshErr != nil {
		return nil, err
	}
	return verifySignedPayload(rawJWT, refreshed)
}

// verifySignedPayload proves an ID token's signature and returns its claims
// segment. Claim validation is left to go-oidc, which owns the iss/aud/exp
// rules; this checks the signature and nothing else, under an explicit
// algorithm allow-list so a token cannot arrive signed by an algorithm the
// verification key was never meant for.
func verifySignedPayload(rawJWT string, keyFunc jwt.Keyfunc) ([]byte, error) {
	_, err := jwt.Parse(rawJWT, keyFunc,
		jwt.WithValidMethods(idTokenSigningMethods),
		jwt.WithoutClaimsValidation(),
	)
	if err != nil {
		return nil, err
	}

	segments := strings.Split(rawJWT, ".")
	if len(segments) != 3 {
		return nil, errors.New("oidc: malformed id token")
	}
	payload, err := base64.RawURLEncoding.DecodeString(segments[1])
	if err != nil {
		return nil, fmt.Errorf("oidc: malformed id token payload: %w", err)
	}
	if !json.Valid(payload) {
		return nil, errors.New("oidc: the id token payload is not JSON")
	}
	return payload, nil
}

func stringClaim(claims map[string]any, name string) string {
	value, _ := claims[name].(string)
	return value
}

func optionalStringClaim(claims map[string]any, name string) *string {
	value := stringClaim(claims, name)
	if value == "" {
		return nil
	}
	return &value
}

// boolClaim reads a boolean claim, accepting the string forms some providers
// still emit for email_verified.
func boolClaim(claims map[string]any, name string) bool {
	switch value := claims[name].(type) {
	case bool:
		return value
	case string:
		return value == "true"
	default:
		return false
	}
}
