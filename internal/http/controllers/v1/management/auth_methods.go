package v1

import (
	"context"
	"errors"
	"net/http"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rbac/access"
	"github.com/lunogram/platform/internal/ssrf"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewAuthMethodsController(logger *zap.Logger, db *sqlx.DB, engine *rbac.Engine) *AuthMethodsController {
	return &AuthMethodsController{
		logger: logger,
		store:  management.NewState(db),
		engine: engine,
	}
}

type AuthMethodsController struct {
	logger *zap.Logger
	store  *management.State
	engine *rbac.Engine
}

func (srv *AuthMethodsController) CreateAuthMethod(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	if !srv.authorizeProject(ctx, w, projectID, rbac.Create) {
		return
	}

	var body oapi.CreateAuthMethodJSONRequestBody
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	in, err := buildCreateAuthMethodInput(body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("name", in.Name), zap.String("type", string(in.Type)))
	logger.Info("creating auth method")

	method, err := srv.store.CreateAuthMethod(ctx, projectID, in)
	if err != nil {
		logger.Error("failed to create auth method", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// RBAC tuples live in OpenFGA, outside the store transaction. If
	// provisioning fails, roll the method back so we never leave a usable
	// credential without its authorization.
	if err := srv.provision(ctx, method); err != nil {
		logger.Error("failed to provision RBAC for auth method, rolling back", zap.Error(err))
		if delErr := srv.store.DeleteAuthMethod(ctx, projectID, method.ID); delErr != nil {
			logger.Error("failed to roll back auth method after provisioning error", zap.Stringer("method_id", method.ID), zap.Error(delErr))
		}
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("auth method created", zap.Stringer("method_id", method.ID))
	json.Write(w, http.StatusCreated, toOAPIAuthMethod(method, true))
}

func (srv *AuthMethodsController) ListAuthMethods(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListAuthMethodsParams) {
	ctx := r.Context()
	if !srv.authorizeProject(ctx, w, projectID, rbac.Read) {
		return
	}

	pagination := store.Pagination{Limit: params.Limit.ToInt(), Offset: params.Offset.ToInt()}
	methods, total, err := srv.store.ListAuthMethods(ctx, projectID, pagination)
	if err != nil {
		srv.logger.Error("failed to list auth methods", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	results := make([]oapi.AuthMethod, len(methods))
	for i := range methods {
		results[i] = toOAPIAuthMethod(&methods[i], false) // never expose secrets in list
	}

	json.Write(w, http.StatusOK, oapi.AuthMethodListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: results,
	})
}

func (srv *AuthMethodsController) GetAuthMethod(w http.ResponseWriter, r *http.Request, projectID, methodID uuid.UUID) {
	ctx := r.Context()
	if !srv.authorizeProject(ctx, w, projectID, rbac.Read) {
		return
	}

	method := srv.fetch(ctx, w, projectID, methodID)
	if method == nil {
		return
	}

	json.Write(w, http.StatusOK, toOAPIAuthMethod(method, false))
}

func (srv *AuthMethodsController) UpdateAuthMethod(w http.ResponseWriter, r *http.Request, projectID, methodID uuid.UUID) {
	ctx := r.Context()
	if !srv.authorizeProject(ctx, w, projectID, rbac.Update) {
		return
	}

	var body oapi.UpdateAuthMethodJSONRequestBody
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	existing := srv.fetch(ctx, w, projectID, methodID)
	if existing == nil {
		return
	}

	in := management.UpdateAuthMethodInput{Name: body.Name, Description: body.Description}
	if body.Role != nil {
		in.Role = ptr.To(string(*body.Role))
	}
	if body.Grants != nil {
		grants := toStoreGrants(*body.Grants)
		if err := validateGrants(grants); err != nil {
			oapi.WriteProblem(w, err)
			return
		}
		in.Grants = grants
	}
	// A role preset and a custom grant set are mutually exclusive effective
	// scopes. Selecting a role without an explicit grant set clears any existing
	// grants so the role actually takes effect ([]Grant{} clears, nil leaves
	// unchanged).
	if body.Role != nil && body.Grants == nil {
		in.Grants = []management.Grant{}
	}
	if body.SubjectScope != nil {
		subjectScope, err := subjectScopeFor(existing.Type, body.SubjectScope)
		if err != nil {
			oapi.WriteProblem(w, err)
			return
		}
		in.SubjectScope = ptr.To(subjectScope)
	}

	if err := srv.store.UpdateAuthMethod(ctx, projectID, methodID, in); err != nil {
		srv.logger.Error("failed to update auth method", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updated := srv.fetch(ctx, w, projectID, methodID)
	if updated == nil {
		return
	}

	// Re-provision RBAC only when the effective scope changed.
	if body.Role != nil || body.Grants != nil {
		if err := srv.reprovision(ctx, existing, updated); err != nil {
			srv.logger.Error("failed to re-provision RBAC for auth method", zap.Stringer("method_id", methodID), zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	json.Write(w, http.StatusOK, toOAPIAuthMethod(updated, false))
}

func (srv *AuthMethodsController) DeleteAuthMethod(w http.ResponseWriter, r *http.Request, projectID, methodID uuid.UUID) {
	ctx := r.Context()
	if !srv.authorizeProject(ctx, w, projectID, rbac.Delete) {
		return
	}

	method := srv.fetch(ctx, w, projectID, methodID)
	if method == nil {
		return
	}

	if err := srv.store.DeleteAuthMethod(ctx, projectID, methodID); err != nil {
		srv.logger.Error("failed to delete auth method", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	// The method is soft-deleted and can no longer authenticate, so any tuples
	// left behind are inert (its UUID is never reused). Log a deprovision
	// failure for cleanup rather than failing an otherwise-successful delete.
	if err := srv.deprovision(ctx, method); err != nil {
		srv.logger.Error("failed to deprovision RBAC for deleted auth method", zap.Stringer("method_id", methodID), zap.Error(err))
	}

	w.WriteHeader(http.StatusNoContent)
}

// authorizeProject verifies the actor may perform verb within its organization
// and that projectID belongs to that organization, closing cross-organization
// access via a guessed project id. It writes a problem response and returns
// false when access is denied. A missing or foreign project returns 404 so its
// existence is not revealed.
func (srv *AuthMethodsController) authorizeProject(ctx context.Context, w http.ResponseWriter, projectID uuid.UUID, verb rbac.Permission) bool {
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, verb, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return false
	}

	project, err := srv.store.GetProject(ctx, projectID)
	if errors.Is(err, store.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project not found")))
		return false
	}
	if err != nil {
		srv.logger.Error("failed to load project for authorization", zap.Stringer("project_id", projectID), zap.Error(err))
		oapi.WriteProblem(w, err)
		return false
	}
	if project.OrganizationID == nil || *project.OrganizationID != actor.OrganizationID {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("project not found")))
		return false
	}
	return true
}

// fetch loads a method, writing a problem response and returning nil when it is
// not found or on error.
func (srv *AuthMethodsController) fetch(ctx context.Context, w http.ResponseWriter, projectID, methodID uuid.UUID) *management.AuthMethod {
	method, err := srv.store.GetAuthMethod(ctx, projectID, methodID)
	if errors.Is(err, store.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("auth method not found")))
		return nil
	}
	if err != nil {
		srv.logger.Error("failed to fetch auth method", zap.Error(err))
		oapi.WriteProblem(w, err)
		return nil
	}
	return method
}

// provision writes the RBAC tuples granting a method its scope: the custom
// permission set when present, otherwise the role preset.
func (srv *AuthMethodsController) provision(ctx context.Context, method *management.AuthMethod) error {
	if len(method.Grants) > 0 {
		return access.ProvisionPolicyGrants(ctx, srv.engine, method.ID, method.ProjectID, toAccessGrants(method.Grants))
	}
	return access.ProvisionApiKey(ctx, srv.engine, method.ID, method.ProjectID, method.Role)
}

func (srv *AuthMethodsController) deprovision(ctx context.Context, method *management.AuthMethod) error {
	if len(method.Grants) > 0 {
		return access.DeprovisionPolicyGrants(ctx, srv.engine, method.ID, method.ProjectID, toAccessGrants(method.Grants))
	}
	return access.DeprovisionApiKey(ctx, srv.engine, method.ID, method.ProjectID, method.Role)
}

// reprovision moves a method's RBAC tuples from its previous effective scope
// (old) to the new one (updated). If provisioning the new scope fails it
// restores the previous one, so a failed update never leaves the method without
// authorization.
func (srv *AuthMethodsController) reprovision(ctx context.Context, old, updated *management.AuthMethod) error {
	if err := srv.deprovision(ctx, old); err != nil {
		return err
	}
	if err := srv.provision(ctx, updated); err != nil {
		if restoreErr := srv.provision(ctx, old); restoreErr != nil {
			srv.logger.Error("failed to restore RBAC scope after provisioning error", zap.Stringer("method_id", old.ID), zap.Error(restoreErr))
		}
		return err
	}
	return nil
}

// buildCreateAuthMethodInput validates a create request and maps it to a store
// input.
func buildCreateAuthMethodInput(body oapi.CreateAuthMethod) (management.CreateAuthMethodInput, error) {
	in := management.CreateAuthMethodInput{
		Type:        management.MethodType(body.Type),
		Name:        body.Name,
		Description: body.Description,
	}
	if body.Grants != nil {
		in.Grants = toStoreGrants(*body.Grants)
	}
	if body.TrustedIssuer != nil {
		in.TrustedIssuer = &management.TrustedIssuer{
			JWKSURL:      ptr.From(body.TrustedIssuer.JwksUrl),
			PublicCert:   ptr.From(body.TrustedIssuer.PublicCert),
			Issuer:       ptr.From(body.TrustedIssuer.Iss),
			Audience:     ptr.From(body.TrustedIssuer.Aud),
			SubjectClaim: claimSub(body.TrustedIssuer.Claim),
		}
	}
	if body.Session != nil {
		in.Session = &management.Session{TTLSeconds: ptr.From(body.Session.TtlSeconds)}
	}

	role := rbac.ProjectSupport
	if body.Role != nil {
		role = string(*body.Role)
	}
	in.Role = role

	subjectScope, err := subjectScopeFor(in.Type, body.SubjectScope)
	if err != nil {
		return in, err
	}
	in.SubjectScope = subjectScope

	if err := validateTypeConfig(in); err != nil {
		return in, err
	}
	if err := validateGrants(in.Grants); err != nil {
		return in, err
	}

	return in, nil
}

// claimSub reads the subject claim from the API claim mapping; empty lets the
// store apply its "sub" default.
func claimSub(claim *oapi.ClaimMapping) string {
	if claim == nil {
		return ""
	}
	return ptr.From(claim.Sub)
}

// validateTypeConfig rejects a create request whose credential config does not
// match its type, and validates the trusted-issuer config (exactly one of
// jwks_url / public_cert, a non-empty issuer, and a safe jwks_url).
func validateTypeConfig(in management.CreateAuthMethodInput) error {
	switch in.Type {
	case management.MethodTypeAPIKey:
		if in.TrustedIssuer != nil || in.Session != nil {
			return problem.ErrBadRequest(problem.Describe("api_key methods must not include trusted_issuer or session config"))
		}
	case management.MethodTypeTrustedIssuer:
		if in.Session != nil {
			return problem.ErrBadRequest(problem.Describe("trusted_issuer methods must not include session config"))
		}
		if in.TrustedIssuer == nil {
			return problem.ErrBadRequest(problem.Describe("trusted_issuer methods require trusted_issuer config"))
		}
		return validateTrustedIssuer(in.TrustedIssuer)
	case management.MethodTypeSession:
		if in.TrustedIssuer != nil {
			return problem.ErrBadRequest(problem.Describe("session methods must not include trusted_issuer config"))
		}
	default:
		return problem.ErrBadRequest(problem.Describe("unknown auth method type: " + string(in.Type)))
	}
	return nil
}

func validateTrustedIssuer(ti *management.TrustedIssuer) error {
	if ti.Issuer == "" {
		return problem.ErrBadRequest(problem.Describe("trusted_issuer requires iss"))
	}
	hasJWKS, hasCert := ti.JWKSURL != "", ti.PublicCert != ""
	if hasJWKS == hasCert {
		return problem.ErrBadRequest(problem.Describe("trusted_issuer requires exactly one of jwks_url or public_cert"))
	}
	if hasJWKS {
		if err := validateJWKSURL(ti.JWKSURL); err != nil {
			return err
		}
	}
	return nil
}

// validateJWKSURL is a cheap up-front guard rejecting an obviously-unsafe JWKS
// URL at configuration time. It delegates to ssrf.ValidateSourceURL so the
// config-time and dial-time guards share one definition of "not public" and
// cannot drift. The authoritative SSRF protection (which also covers DNS
// rebinding) remains the dial-time guard in the jwks fetcher.
func validateJWKSURL(raw string) error {
	if err := ssrf.ValidateSourceURL(raw); err != nil {
		return problem.ErrBadRequest(problem.Describe("jwks_url is not a valid public https URL"))
	}
	return nil
}

// validateGrants rejects a custom permission set referencing an unknown resource
// or verb, which would otherwise write a tuple that silently never resolves. It
// also rejects an instance allow-list on a grant that could never enforce one:
// the list is only honored for a create grant on a constrainable resource
// (today only events). Accepting it elsewhere would store an allow-list that is
// silently never applied — a false sense of restriction. This makes the
// guarantee structural: a constraint cannot exist apart from a grant that
// enforces it.
func validateGrants(grants []management.Grant) error {
	if len(grants) == 0 {
		return nil
	}
	known := grantableResources()
	enforced := constrainableResources()
	for _, g := range grants {
		if _, ok := known[g.Resource]; !ok {
			return problem.ErrBadRequest(problem.Describe("unknown grant resource: " + g.Resource))
		}
		switch rbac.Permission(g.Verb) {
		case rbac.Read, rbac.Create, rbac.Update, rbac.Delete:
		default:
			return problem.ErrBadRequest(problem.Describe("unknown grant verb: " + g.Verb))
		}
		if len(g.Instances) == 0 {
			continue
		}
		if rbac.Permission(g.Verb) != rbac.Create {
			return problem.ErrBadRequest(problem.Describe("instances are only supported on a create grant: " + g.Resource))
		}
		if _, ok := enforced[g.Resource]; !ok {
			return problem.ErrBadRequest(problem.Describe("instances are not supported for resource: " + g.Resource))
		}
	}
	return nil
}

// grantableResources is the set of resources a custom grant (permission set) may
// name. Any resource in the authorization model can carry a grant.
func grantableResources() map[string]struct{} {
	known := make(map[string]struct{}, len(rbac.Resources()))
	for _, r := range rbac.Resources() {
		known[r] = struct{}{}
	}
	return known
}

// constrainableResources is the set of resources whose create grant honors an
// instance-name allow-list (its grant's instances). It is the single source of
// truth shared with the request-time enforcement site (CreateConstraints.Enforce
// via the events controller); today only client events are enforced. Instances
// on any other resource would never apply and are rejected at configuration
// time by [validateGrants].
func constrainableResources() map[string]struct{} {
	return map[string]struct{}{"events": {}}
}

// subjectScopeFor resolves and validates the data boundary for a method. An
// api_key has no verified subject to confine, so it is always "all"; verified
// types (trusted_issuer, session) default to "own" and may opt into "all".
func subjectScopeFor(t management.MethodType, requested *oapi.SubjectScope) (management.SubjectScope, error) {
	apiKey := t == management.MethodTypeAPIKey
	if requested == nil {
		if apiKey {
			return management.SubjectScopeAll, nil
		}
		return management.SubjectScopeOwn, nil
	}
	scope := management.SubjectScope(*requested)
	if apiKey && scope != management.SubjectScopeAll {
		return "", problem.ErrBadRequest(problem.Describe(`api_key methods must use the "all" subject scope`))
	}
	return scope, nil
}

func toStoreGrants(grants []oapi.PermissionGrant) []management.Grant {
	out := make([]management.Grant, len(grants))
	for i, g := range grants {
		out[i] = management.Grant{Resource: g.Resource, Verb: string(g.Verb)}
		if g.Instances != nil {
			out[i].Instances = *g.Instances
		}
	}
	return out
}

func toAccessGrants(grants []management.Grant) []access.Grant {
	out := make([]access.Grant, len(grants))
	for i, g := range grants {
		out[i] = access.Grant{Resource: g.Resource, Verb: rbac.Permission(g.Verb)}
	}
	return out
}

// toOAPIAuthMethod maps a stored method to its API representation. The secret is
// included only when withSecret is true (the create response).
func toOAPIAuthMethod(m *management.AuthMethod, withSecret bool) oapi.AuthMethod {
	out := oapi.AuthMethod{
		Id:           m.ID,
		ProjectId:    m.ProjectID,
		Type:         oapi.AuthMethodType(m.Type),
		Name:         m.Name,
		Description:  m.Description,
		Role:         oapi.ProjectRole(m.Role),
		SubjectScope: ptr.To(oapi.SubjectScope(m.SubjectScope)),
		CreatedAt:    m.CreatedAt,
		UpdatedAt:    m.UpdatedAt,
	}
	if len(m.Grants) > 0 {
		grants := make([]oapi.PermissionGrant, len(m.Grants))
		for i, g := range m.Grants {
			grants[i] = oapi.PermissionGrant{Resource: g.Resource, Verb: oapi.PermissionGrantVerb(g.Verb)}
			if len(g.Instances) > 0 {
				grants[i].Instances = ptr.To(g.Instances)
			}
		}
		out.Grants = &grants
	}
	if ti := m.TrustedIssuer; ti != nil {
		issuer := &oapi.TrustedIssuer{}
		if ti.JWKSURL != "" {
			issuer.JwksUrl = ptr.To(ti.JWKSURL)
		}
		if ti.PublicCert != "" {
			issuer.PublicCert = ptr.To(ti.PublicCert)
		}
		if ti.Issuer != "" {
			issuer.Iss = ptr.To(ti.Issuer)
		}
		if ti.Audience != "" {
			issuer.Aud = ptr.To(ti.Audience)
		}
		if ti.SubjectClaim != "" {
			issuer.Claim = &oapi.ClaimMapping{Sub: ptr.To(ti.SubjectClaim)}
		}
		out.TrustedIssuer = issuer
	}
	if m.Session != nil {
		out.Session = &oapi.SessionConfig{TtlSeconds: ptr.To(m.Session.TTLSeconds)}
	}
	if withSecret {
		out.Secret = m.Secret
	}
	return out
}
