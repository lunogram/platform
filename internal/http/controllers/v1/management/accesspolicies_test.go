package v1

import (
	"testing"

	"github.com/lunogram/platform/internal/http/controllers/v1/management/oapi"
	"github.com/lunogram/platform/internal/ptr"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestBuildCreateInput(t *testing.T) {
	t.Parallel()

	scope := func(s oapi.ApiKeyScope) *oapi.ApiKeyScope { return &s }
	role := func(r oapi.ProjectRole) *oapi.ProjectRole { return &r }

	t.Run("defaults role to support for secret keys", func(t *testing.T) {
		in, err := buildCreateInput(oapi.CreateAccessPolicy{
			Type:  oapi.AccessPolicyType("api_key"),
			Name:  "backend",
			Scope: scope("secret"),
		})
		require.NoError(t, err)
		assert.Equal(t, "support", in.Role)
	})

	t.Run("defaults public keys to the write-only client role", func(t *testing.T) {
		in, err := buildCreateInput(oapi.CreateAccessPolicy{
			Type:  oapi.AccessPolicyType("api_key"),
			Name:  "web",
			Scope: scope("public"),
		})
		require.NoError(t, err)
		assert.Equal(t, "client", in.Role)
	})

	t.Run("rejects a public key with a readable role", func(t *testing.T) {
		_, err := buildCreateInput(oapi.CreateAccessPolicy{
			Type:  oapi.AccessPolicyType("api_key"),
			Name:  "web",
			Scope: scope("public"),
			Role:  role("editor"),
		})
		assert.Error(t, err)
	})

	t.Run("rejects a public key granted read access", func(t *testing.T) {
		_, err := buildCreateInput(oapi.CreateAccessPolicy{
			Type:  oapi.AccessPolicyType("api_key"),
			Name:  "web",
			Scope: scope("public"),
			Grants: &[]oapi.PermissionGrant{
				{Resource: "inbox", Verb: oapi.PermissionGrantVerb("read")},
			},
		})
		assert.Error(t, err)
	})

	t.Run("allows a public key with write-only grants", func(t *testing.T) {
		in, err := buildCreateInput(oapi.CreateAccessPolicy{
			Type:  oapi.AccessPolicyType("api_key"),
			Name:  "web",
			Scope: scope("public"),
			Grants: &[]oapi.PermissionGrant{
				{Resource: "events", Verb: oapi.PermissionGrantVerb("create")},
			},
		})
		require.NoError(t, err)
		assert.Len(t, in.Grants, 1)
	})

	t.Run("maps trusted-issuer config", func(t *testing.T) {
		in, err := buildCreateInput(oapi.CreateAccessPolicy{
			Type: oapi.AccessPolicyType("trusted_issuer"),
			Name: "idp",
			IssuerConfig: &oapi.TrustedIssuerConfig{
				JwksUrl: ptr.To("https://idp.example/jwks.json"),
				Iss:     ptr.To("https://idp.example"),
			},
		})
		require.NoError(t, err)
		require.NotNil(t, in.IssuerConfig)
		assert.Equal(t, "https://idp.example/jwks.json", in.IssuerConfig.JWKSURL)
	})
}
