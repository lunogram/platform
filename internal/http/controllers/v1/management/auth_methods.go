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
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
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

	if err := srv.provision(ctx, method); err != nil {
		logger.Error("failed to provision RBAC for auth method", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("auth method created", zap.Stringer("method_id", method.ID))
	json.Write(w, http.StatusCreated, toOAPIAuthMethod(method, true))
}

func (srv *AuthMethodsController) ListAuthMethods(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListAuthMethodsParams) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
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
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
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
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
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
		in.Grants = toStoreGrants(*body.Grants)
	}
	if body.SubjectScope != nil {
		subjectScope, err := subjectScopeFor(existing.Type, body.SubjectScope)
		if err != nil {
			oapi.WriteProblem(w, err)
			return
		}
		in.SubjectScope = ptr.To(subjectScope)
	}

	if err := enforcePublicWriteOnly(existing.Scope, in.Role, in.Grants); err != nil {
		oapi.WriteProblem(w, err)
		return
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
		if err := srv.deprovision(ctx, existing); err != nil {
			srv.logger.Error("failed to deprovision old auth method scope", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
		if err := srv.provision(ctx, updated); err != nil {
			srv.logger.Error("failed to provision new auth method scope", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	json.Write(w, http.StatusOK, toOAPIAuthMethod(updated, false))
}

func (srv *AuthMethodsController) DeleteAuthMethod(w http.ResponseWriter, r *http.Request, projectID, methodID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Delete, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
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

	if err := srv.deprovision(ctx, method); err != nil {
		srv.logger.Error("failed to deprovision RBAC for auth method", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
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

// buildCreateAuthMethodInput validates a create request and maps it to a store
// input, enforcing the public-key write-only invariant.
func buildCreateAuthMethodInput(body oapi.CreateAuthMethod) (management.CreateAuthMethodInput, error) {
	in := management.CreateAuthMethodInput{
		Type:        management.MethodType(body.Type),
		Name:        body.Name,
		Description: body.Description,
	}
	if body.Scope != nil {
		in.Scope = ptr.To(string(*body.Scope))
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
			SubjectClaim: ptr.From(body.TrustedIssuer.SubjectClaim),
		}
	}
	if body.Session != nil {
		in.Session = &management.Session{TTLSeconds: ptr.From(body.Session.TtlSeconds)}
	}

	role := rbac.ProjectSupport
	if body.Role != nil {
		role = string(*body.Role)
	}
	// Public keys are browser-exposed: default to the write-only client preset
	// and reject any read access.
	if in.Scope != nil && *in.Scope == management.ScopePublic && body.Role == nil {
		role = rbac.ProjectClient
	}
	in.Role = role

	subjectScope, err := subjectScopeFor(in.Type, body.SubjectScope)
	if err != nil {
		return in, err
	}
	in.SubjectScope = subjectScope

	if err := enforcePublicWriteOnly(in.Scope, &role, in.Grants); err != nil {
		return in, err
	}

	return in, nil
}

// subjectScopeFor resolves and validates the data boundary for a method. An
// api_key has no verified subject to confine, so it is always "all"; verified
// types (trusted_issuer, session) default to "own" and may opt into "all".
func subjectScopeFor(t management.MethodType, requested *oapi.SubjectScope) (string, error) {
	apiKey := t == management.MethodTypeAPIKey
	if requested == nil {
		if apiKey {
			return management.SubjectScopeAll, nil
		}
		return management.SubjectScopeOwn, nil
	}
	scope := string(*requested)
	if apiKey && scope != management.SubjectScopeAll {
		return "", problem.ErrBadRequest(problem.Describe(`api_key methods must use the "all" subject scope`))
	}
	return scope, nil
}

// enforcePublicWriteOnly rejects a public-scoped method that would gain any read
// access, keeping browser-exposed keys write-only. role/grants are the values
// being applied (nil = unchanged).
func enforcePublicWriteOnly(scope *string, role *string, grants []management.Grant) error {
	if scope == nil || *scope != management.ScopePublic {
		return nil
	}
	if role != nil && *role != rbac.ProjectClient {
		return problem.ErrBadRequest(problem.Describe("public methods must use the write-only client role"))
	}
	for _, g := range grants {
		if g.Verb == string(rbac.Read) {
			return problem.ErrBadRequest(problem.Describe("public methods may not be granted read access"))
		}
	}
	return nil
}

func toStoreGrants(grants []oapi.PermissionGrant) []management.Grant {
	out := make([]management.Grant, len(grants))
	for i, g := range grants {
		out[i] = management.Grant{Resource: g.Resource, Verb: string(g.Verb)}
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
		}
		out.Grants = &grants
	}
	if m.Scope != nil {
		out.Scope = ptr.To(oapi.ApiKeyScope(*m.Scope))
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
			issuer.SubjectClaim = ptr.To(ti.SubjectClaim)
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
