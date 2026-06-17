package v1

import (
	"context"
	"errors"
	"net/http"
	"time"

	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/http/json"
	"github.com/lunogram/platform/internal/http/problem"
	"github.com/lunogram/platform/internal/rbac"
	"github.com/lunogram/platform/internal/rbac/access"
	"github.com/lunogram/platform/internal/store"
	"github.com/lunogram/platform/internal/store/management"
	"go.uber.org/zap"
)

func NewAccessPoliciesController(logger *zap.Logger, db *sqlx.DB, engine *rbac.Engine) *AccessPoliciesController {
	return &AccessPoliciesController{
		logger: logger,
		store:  management.NewState(db),
		engine: engine,
	}
}

type AccessPoliciesController struct {
	logger *zap.Logger
	store  *management.State
	engine *rbac.Engine
}

func (srv *AccessPoliciesController) CreateAccessPolicy(w http.ResponseWriter, r *http.Request, projectID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Create, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.CreateAccessPolicyJSONRequestBody
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	in, err := buildCreateInput(body)
	if err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	logger := srv.logger.With(zap.Stringer("project_id", projectID), zap.String("name", in.Name), zap.String("type", string(in.Type)))
	logger.Info("creating access policy")

	policy, err := srv.store.CreateAccessPolicy(ctx, projectID, in)
	if err != nil {
		logger.Error("failed to create access policy", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if err := srv.provision(ctx, policy); err != nil {
		logger.Error("failed to provision RBAC for access policy", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	logger.Info("access policy created", zap.Stringer("policy_id", policy.ID))
	json.Write(w, http.StatusCreated, toOAPIAccessPolicy(policy, true))
}

func (srv *AccessPoliciesController) ListAccessPolicies(w http.ResponseWriter, r *http.Request, projectID uuid.UUID, params oapi.ListAccessPoliciesParams) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	pagination := store.Pagination{Limit: params.Limit.ToInt(), Offset: params.Offset.ToInt()}
	policies, total, err := srv.store.ListAccessPolicies(ctx, projectID, pagination)
	if err != nil {
		srv.logger.Error("failed to list access policies", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	results := make([]oapi.AccessPolicy, len(policies))
	for i := range policies {
		// Never include secrets in list responses.
		results[i] = toOAPIAccessPolicy(&policies[i], false)
	}

	json.Write(w, http.StatusOK, oapi.AccessPolicyListResponse{
		Total:   total,
		Limit:   pagination.Limit,
		Offset:  pagination.Offset,
		Results: results,
	})
}

func (srv *AccessPoliciesController) GetAccessPolicy(w http.ResponseWriter, r *http.Request, projectID, policyID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Read, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	policy := srv.fetch(ctx, w, projectID, policyID)
	if policy == nil {
		return
	}

	json.Write(w, http.StatusOK, toOAPIAccessPolicy(policy, false))
}

func (srv *AccessPoliciesController) UpdateAccessPolicy(w http.ResponseWriter, r *http.Request, projectID, policyID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Update, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	var body oapi.UpdateAccessPolicyJSONRequestBody
	if err := json.Decode(r.Body, &body); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	existing := srv.fetch(ctx, w, projectID, policyID)
	if existing == nil {
		return
	}

	in := management.UpdateAccessPolicyInput{Name: body.Name, Description: body.Description}
	if body.Role != nil {
		role := string(*body.Role)
		in.Role = &role
	}
	if body.Grants != nil {
		in.Grants = toStoreGrants(*body.Grants)
	}

	if err := srv.enforcePublicWriteOnly(existing, in); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	if err := srv.store.UpdateAccessPolicy(ctx, projectID, policyID, in); err != nil {
		srv.logger.Error("failed to update access policy", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	updated := srv.fetch(ctx, w, projectID, policyID)
	if updated == nil {
		return
	}

	// Re-provision RBAC only when the effective scope changed.
	if body.Role != nil || body.Grants != nil {
		if err := srv.deprovision(ctx, existing); err != nil {
			srv.logger.Error("failed to deprovision old access policy scope", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
		if err := srv.provision(ctx, updated); err != nil {
			srv.logger.Error("failed to provision new access policy scope", zap.Error(err))
			oapi.WriteProblem(w, err)
			return
		}
	}

	json.Write(w, http.StatusOK, toOAPIAccessPolicy(updated, false))
}

func (srv *AccessPoliciesController) DeleteAccessPolicy(w http.ResponseWriter, r *http.Request, projectID, policyID uuid.UUID) {
	ctx := r.Context()
	actor := rbac.FromContext(ctx)
	if err := srv.engine.Allowed(ctx, rbac.Delete, rbac.OrganizationScope(actor.OrganizationID)); err != nil {
		oapi.WriteProblem(w, err)
		return
	}

	policy := srv.fetch(ctx, w, projectID, policyID)
	if policy == nil {
		return
	}

	if err := srv.store.DeleteAccessPolicy(ctx, projectID, policyID); err != nil {
		srv.logger.Error("failed to delete access policy", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	if err := srv.deprovision(ctx, policy); err != nil {
		srv.logger.Error("failed to deprovision RBAC for access policy", zap.Error(err))
		oapi.WriteProblem(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// fetch loads a policy, writing a problem response and returning nil when it is
// not found or on error.
func (srv *AccessPoliciesController) fetch(ctx context.Context, w http.ResponseWriter, projectID, policyID uuid.UUID) *management.AccessPolicy {
	policy, err := srv.store.GetAccessPolicy(ctx, projectID, policyID)
	if errors.Is(err, store.ErrNoRows) {
		oapi.WriteProblem(w, problem.ErrNotFound(problem.Describe("access policy not found")))
		return nil
	}
	if err != nil {
		srv.logger.Error("failed to fetch access policy", zap.Error(err))
		oapi.WriteProblem(w, err)
		return nil
	}
	return policy
}

// provision writes the RBAC tuples granting a policy its scope: the custom
// permission set when present, otherwise the role preset.
func (srv *AccessPoliciesController) provision(ctx context.Context, policy *management.AccessPolicy) error {
	if len(policy.Grants) > 0 {
		return access.ProvisionPolicyGrants(ctx, srv.engine, policy.ID, policy.ProjectID, toAccessGrants(policy.Grants))
	}
	return access.ProvisionApiKey(ctx, srv.engine, policy.ID, policy.ProjectID, policy.Role)
}

func (srv *AccessPoliciesController) deprovision(ctx context.Context, policy *management.AccessPolicy) error {
	if len(policy.Grants) > 0 {
		return access.DeprovisionPolicyGrants(ctx, srv.engine, policy.ID, policy.ProjectID, toAccessGrants(policy.Grants))
	}
	return access.DeprovisionApiKey(ctx, srv.engine, policy.ID, policy.ProjectID, policy.Role)
}

// enforcePublicWriteOnly rejects an update that would grant a public policy any
// read access, preserving the invariant that browser-exposed keys are
// write-only.
func (srv *AccessPoliciesController) enforcePublicWriteOnly(existing *management.AccessPolicy, in management.UpdateAccessPolicyInput) error {
	if existing.Scope == nil || *existing.Scope != management.ScopePublic {
		return nil
	}
	if in.Role != nil && *in.Role != rbac.ProjectClient {
		return problem.ErrBadRequest(problem.Describe("public policies must use the write-only client role"))
	}
	for _, g := range in.Grants {
		if g.Verb == string(rbac.Read) {
			return problem.ErrBadRequest(problem.Describe("public policies may not be granted read access"))
		}
	}
	return nil
}

// buildCreateInput validates a create request and maps it to a store input,
// enforcing the public-key write-only invariant.
func buildCreateInput(body oapi.CreateAccessPolicy) (management.CreateAccessPolicyInput, error) {
	in := management.CreateAccessPolicyInput{
		Type:        management.PolicyType(body.Type),
		Name:        body.Name,
		Description: body.Description,
	}
	if body.Scope != nil {
		scope := string(*body.Scope)
		in.Scope = &scope
	}
	if body.Grants != nil {
		in.Grants = toStoreGrants(*body.Grants)
	}
	if body.IssuerConfig != nil {
		in.IssuerConfig = toStoreIssuerConfig(*body.IssuerConfig)
	}
	if body.SessionConfig != nil {
		in.SessionConfig = toStoreSessionConfig(*body.SessionConfig)
	}

	role := rbac.ProjectSupport
	if body.Role != nil {
		role = string(*body.Role)
	}

	if in.Scope != nil && *in.Scope == management.ScopePublic {
		// Public keys are browser-exposed: default to the write-only client
		// preset and reject any read access.
		if body.Role == nil {
			role = rbac.ProjectClient
		}
		if role != rbac.ProjectClient {
			return in, problem.ErrBadRequest(problem.Describe("public policies must use the write-only client role"))
		}
		for _, g := range in.Grants {
			if g.Verb == string(rbac.Read) {
				return in, problem.ErrBadRequest(problem.Describe("public policies may not be granted read access"))
			}
		}
	}
	in.Role = role

	return in, nil
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

func toStoreIssuerConfig(c oapi.TrustedIssuerConfig) *management.IssuerConfig {
	out := &management.IssuerConfig{}
	if c.JwksUrl != nil {
		out.JWKSURL = *c.JwksUrl
	}
	if c.PublicCert != nil {
		out.PublicCert = *c.PublicCert
	}
	if c.Iss != nil {
		out.Issuer = *c.Iss
	}
	if c.Aud != nil {
		out.Audience = *c.Aud
	}
	if c.SubjectClaim != nil {
		out.SubjectClaim = *c.SubjectClaim
	}
	return out
}

func toStoreSessionConfig(c oapi.SessionPolicyConfig) *management.SessionConfig {
	out := &management.SessionConfig{}
	if c.TtlSeconds != nil {
		out.TTL = time.Duration(*c.TtlSeconds) * time.Second
	}
	if c.Role != nil {
		out.Role = string(*c.Role)
	}
	return out
}

// toOAPIAccessPolicy maps a stored policy to its API representation. The secret
// is included only when withSecret is true (the create response).
func toOAPIAccessPolicy(p *management.AccessPolicy, withSecret bool) oapi.AccessPolicy {
	out := oapi.AccessPolicy{
		Id:          p.ID,
		ProjectId:   p.ProjectID,
		Type:        oapi.AccessPolicyType(p.Type),
		Name:        p.Name,
		Description: p.Description,
		Role:        oapi.ProjectRole(p.Role),
		CreatedAt:   p.CreatedAt,
		UpdatedAt:   p.UpdatedAt,
	}
	if p.Scope != nil {
		scope := oapi.ApiKeyScope(*p.Scope)
		out.Scope = &scope
	}
	if len(p.Grants) > 0 {
		grants := make([]oapi.PermissionGrant, len(p.Grants))
		for i, g := range p.Grants {
			grants[i] = oapi.PermissionGrant{Resource: g.Resource, Verb: oapi.PermissionGrantVerb(g.Verb)}
		}
		out.Grants = &grants
	}
	if p.IssuerConfig != nil {
		out.IssuerConfig = toOAPIIssuerConfig(p.IssuerConfig)
	}
	if p.SessionConfig != nil {
		out.SessionConfig = toOAPISessionConfig(p.SessionConfig)
	}
	if withSecret {
		out.Secret = p.Secret
	}
	return out
}

func toOAPIIssuerConfig(c *management.IssuerConfig) *oapi.TrustedIssuerConfig {
	out := &oapi.TrustedIssuerConfig{}
	if c.JWKSURL != "" {
		out.JwksUrl = &c.JWKSURL
	}
	if c.PublicCert != "" {
		out.PublicCert = &c.PublicCert
	}
	if c.Issuer != "" {
		out.Iss = &c.Issuer
	}
	if c.Audience != "" {
		out.Aud = &c.Audience
	}
	if c.SubjectClaim != "" {
		out.SubjectClaim = &c.SubjectClaim
	}
	return out
}

func toOAPISessionConfig(c *management.SessionConfig) *oapi.SessionPolicyConfig {
	out := &oapi.SessionPolicyConfig{}
	if c.TTL > 0 {
		seconds := int(c.TTL / time.Second)
		out.TtlSeconds = &seconds
	}
	if c.Role != "" {
		role := oapi.ProjectRole(c.Role)
		out.Role = &role
	}
	return out
}
