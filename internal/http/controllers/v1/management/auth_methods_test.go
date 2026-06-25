package v1

import (
	"testing"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/lunogram/platform/internal/store/management"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestValidateGrantInstances(t *testing.T) {
	t.Parallel()

	t.Run("accepts grants with no instances", func(t *testing.T) {
		require.NoError(t, validateGrants([]management.Grant{{Resource: "events", Verb: "create"}}))
	})

	t.Run("accepts instances on a create grant for an enforced resource", func(t *testing.T) {
		require.NoError(t, validateGrants([]management.Grant{
			{Resource: "events", Verb: "create", Instances: []string{"purchase"}},
		}))
	})

	t.Run("rejects instances on a non-create grant", func(t *testing.T) {
		// Instances are only enforced for create today; an allow-list on a read
		// grant would be stored but never applied.
		err := validateGrants([]management.Grant{
			{Resource: "events", Verb: "read", Instances: []string{"purchase"}},
		})
		require.Error(t, err)
	})

	t.Run("rejects instances on an unenforced resource", func(t *testing.T) {
		// Instances on a resource with no request-time enforcement would be
		// stored but never applied — a false sense of restriction. It must be
		// rejected at configuration time, even though the resource is a valid
		// grant target.
		err := validateGrants([]management.Grant{
			{Resource: "users", Verb: "create", Instances: []string{"alice"}},
		})
		require.Error(t, err)
	})
}

func TestBuildCreateAuthMethodInput(t *testing.T) {
	t.Parallel()

	subjectScope := func(s oapi.SubjectScope) *oapi.SubjectScope { return &s }

	t.Run("defaults api keys to the support role", func(t *testing.T) {
		in, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type: "api_key",
			Name: "backend",
		})
		require.NoError(t, err)
		assert.Equal(t, "support", in.Role)
	})

	t.Run("maps trusted-issuer config including the subject claim", func(t *testing.T) {
		in, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type: "trusted_issuer",
			Name: "idp",
			TrustedIssuer: &oapi.TrustedIssuer{
				JwksUrl: ptr.To("https://idp.example/jwks.json"),
				Iss:     ptr.To("https://idp.example"),
				Claim:   &oapi.ClaimMapping{Sub: ptr.To("user_id")},
			},
		})
		require.NoError(t, err)
		require.NotNil(t, in.TrustedIssuer)
		assert.Equal(t, "https://idp.example/jwks.json", in.TrustedIssuer.JWKSURL)
		assert.Equal(t, "https://idp.example", in.TrustedIssuer.Issuer)
		assert.Equal(t, "user_id", in.TrustedIssuer.SubjectClaim)
	})

	t.Run("api keys default to the all subject scope", func(t *testing.T) {
		in, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type: "api_key",
			Name: "backend",
		})
		require.NoError(t, err)
		assert.Equal(t, "all", string(in.SubjectScope))
	})

	t.Run("verified types default to the own subject scope", func(t *testing.T) {
		in, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type:    "session",
			Name:    "web sessions",
			Session: &oapi.SessionConfig{TtlSeconds: ptr.To(900)},
		})
		require.NoError(t, err)
		assert.Equal(t, "own", string(in.SubjectScope))
	})

	t.Run("a verified type may opt into the all subject scope", func(t *testing.T) {
		in, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type:         "trusted_issuer",
			Name:         "support console",
			SubjectScope: subjectScope("all"),
			TrustedIssuer: &oapi.TrustedIssuer{
				JwksUrl: ptr.To("https://idp.example/jwks.json"),
				Iss:     ptr.To("https://idp.example"),
			},
		})
		require.NoError(t, err)
		assert.Equal(t, "all", string(in.SubjectScope))
	})

	t.Run("rejects an api key confined to own data", func(t *testing.T) {
		_, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type:         "api_key",
			Name:         "backend",
			SubjectScope: subjectScope("own"),
		})
		assert.Error(t, err)
	})

	t.Run("rejects an api key carrying trusted-issuer config", func(t *testing.T) {
		_, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type:          "api_key",
			Name:          "backend",
			TrustedIssuer: &oapi.TrustedIssuer{Iss: ptr.To("https://idp.example")},
		})
		assert.Error(t, err)
	})

	t.Run("rejects a trusted issuer with no jwks_url or public_cert", func(t *testing.T) {
		_, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type:          "trusted_issuer",
			Name:          "idp",
			TrustedIssuer: &oapi.TrustedIssuer{Iss: ptr.To("https://idp.example")},
		})
		assert.Error(t, err)
	})

	t.Run("rejects a trusted issuer with both jwks_url and public_cert", func(t *testing.T) {
		_, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type: "trusted_issuer",
			Name: "idp",
			TrustedIssuer: &oapi.TrustedIssuer{
				Iss:        ptr.To("https://idp.example"),
				JwksUrl:    ptr.To("https://idp.example/jwks.json"),
				PublicCert: ptr.To("-----BEGIN CERTIFICATE-----"),
			},
		})
		assert.Error(t, err)
	})

	t.Run("rejects a trusted issuer with an unsafe jwks_url", func(t *testing.T) {
		_, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type: "trusted_issuer",
			Name: "idp",
			TrustedIssuer: &oapi.TrustedIssuer{
				Iss:     ptr.To("https://idp.example"),
				JwksUrl: ptr.To("http://169.254.169.254/jwks.json"),
			},
		})
		assert.Error(t, err)
	})

	t.Run("rejects a grant referencing an unknown resource", func(t *testing.T) {
		_, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type: "api_key",
			Name: "backend",
			Grants: &[]oapi.PermissionGrant{
				{Resource: "not_a_resource", Verb: "read"},
			},
		})
		assert.Error(t, err)
	})

	t.Run("accepts a grant referencing a known resource and verb", func(t *testing.T) {
		_, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type: "api_key",
			Name: "backend",
			Grants: &[]oapi.PermissionGrant{
				{Resource: "inbox", Verb: "read"},
			},
		})
		require.NoError(t, err)
	})

	t.Run("maps a grant's instance allow-list onto the store input", func(t *testing.T) {
		in, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type: "api_key",
			Name: "backend",
			Grants: &[]oapi.PermissionGrant{
				{Resource: "events", Verb: "create", Instances: ptr.To([]string{"purchase", "signup"})},
			},
		})
		require.NoError(t, err)
		require.Len(t, in.Grants, 1)
		assert.Equal(t, []string{"purchase", "signup"}, in.Grants[0].Instances)
	})

	t.Run("rejects an instance allow-list on an unenforced resource", func(t *testing.T) {
		_, err := buildCreateAuthMethodInput(oapi.CreateAuthMethod{
			Type: "api_key",
			Name: "backend",
			Grants: &[]oapi.PermissionGrant{
				{Resource: "users", Verb: "create", Instances: ptr.To([]string{"x"})},
			},
		})
		assert.Error(t, err)
	})
}
