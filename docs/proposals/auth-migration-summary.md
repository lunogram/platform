# Auth Migration Summary

## Quick Overview

Migrate authentication endpoints from Node.js Platform service to Go Nexus service while maintaining the modular provider-based architecture.

## Why This Migration?

1. **Consolidation**: Move all API endpoints to Nexus service
2. **Performance**: Go's better performance characteristics
3. **Type Safety**: Stronger typing with Go and OpenAPI
4. **Maintainability**: Single codebase for all backend APIs
5. **Scalability**: Better resource utilization

## What's Being Migrated?

### 6 Authentication Providers

| Provider | Type | Use Case |
|----------|------|----------|
| **Basic** | Email/Password | Development & testing |
| **Email** | Magic Link | Passwordless auth via email |
| **SAML** | SSO | Enterprise single sign-on |
| **OpenID** | OAuth 2.0 | Standards-based OAuth |
| **Google** | OAuth 2.0 | Google account login |
| **Clerk** | Cloud Service | Managed auth with webhooks |

### API Endpoints

```
GET  /auth/methods                  - List available methods
POST /auth/check                    - Check org-specific auth
GET  /auth/login/:driver            - Start auth flow
POST /auth/login/:driver            - Start auth flow (POST)
GET  /auth/login/:driver/callback   - Handle callback
POST /auth/login/:driver/callback   - Handle callback (POST)
POST /auth/:driver/webhook          - Process webhooks
```

## Key Design Principles

### 1. Modularity Preserved

```go
// Provider interface - all providers implement this
type Provider interface {
    Name() string
    Start(ctx context.Context, req StartRequest) (*StartResponse, error)
    Validate(ctx context.Context, req ValidateRequest) (*ValidateResponse, error)
}

// Optional webhook support
type WebhookProvider interface {
    Provider
    HandleWebhook(ctx context.Context, req WebhookRequest) error
}
```

### 2. Configuration-Driven

```yaml
auth:
  drivers: [basic, email, google]  # Enable specific providers
  tokenLifetime: 24h
  
  basic:
    name: "Admin Login"
    email: "admin@example.com"
    password: "${AUTH_BASIC_PASSWORD}"
  
  email:
    name: "Email Login"
    from: "auth@lunogram.com"
    smtp:
      host: smtp.sendgrid.net
      port: 587
```

### 3. OpenAPI-First

All endpoints defined in OpenAPI spec before implementation:
- Type generation with `oapi-codegen`
- Automatic validation
- Self-documenting API
- Frontend type generation

### 4. Zero Frontend Changes

- Same endpoint URLs
- Same request/response formats
- Same cookie structure
- Same JWT token format
- Seamless cutover

## Implementation Phases

### Phase 1: Foundation (Week 1)
- Provider interface
- Registry system
- Token utilities
- OpenAPI spec
- Handler scaffolding

**Deliverable**: Core architecture, no providers yet

### Phase 2: Simple Providers (Week 2)
- BasicProvider
- EmailProvider
- Integration tests

**Deliverable**: Two working auth methods

### Phase 3: OAuth/OIDC (Week 3)
- OpenIDProvider
- GoogleProvider
- Cookie state management

**Deliverable**: OAuth flows working

### Phase 4: SAML (Week 4)
- SAMLProvider
- Redirect & POST bindings

**Deliverable**: Enterprise SSO support

### Phase 5: Cloud (Week 5)
- ClerkProvider
- Webhook handling

**Deliverable**: All 6 providers working

### Phase 6: Cutover (Week 6)
- Feature flag
- Parallel operation
- Traffic migration
- Monitoring

**Deliverable**: Production migration complete

## Technical Stack

### New Dependencies

```
github.com/crewjam/saml           # SAML 2.0
github.com/coreos/go-oidc/v3      # OpenID Connect
golang.org/x/oauth2                # OAuth 2.0
github.com/clerk/clerk-sdk-go/v2  # Clerk
github.com/svix/svix-webhooks/go  # Webhook verification
gopkg.in/gomail.v2                 # Email sending
```

### Existing Infrastructure

- PostgreSQL (admins, access_tokens tables)
- JWT token generation
- Cookie management
- OpenAPI validation
- Chi router

## Migration Safety

### Backwards Compatibility

✅ Endpoint URLs unchanged
✅ Request/response formats unchanged
✅ Database schema unchanged
✅ Cookie names unchanged
✅ JWT token format unchanged

### Rollback Strategy

1. Keep Platform service running
2. Feature flag for new endpoints
3. Quick rollback capability (< 5 minutes)
4. Monitor metrics during migration
5. Gradual traffic shift

### Testing Strategy

```
Unit Tests
├── Provider implementations
├── Token generation
├── Cookie management
└── Webhook verification

Integration Tests
├── Full auth flows
├── Database operations
├── Token validation
└── Cookie handling

Manual Testing
├── Test SAML IdP
├── Google OAuth project
├── Clerk test instance
└── Mock SMTP server
```

## Security Considerations

| Aspect | Implementation |
|--------|----------------|
| **Secrets** | Environment variables only |
| **CSRF** | State/nonce validation |
| **Webhooks** | Signature verification |
| **Tokens** | JWT with expiration |
| **Cookies** | HttpOnly, Secure flags |
| **Audit** | Log all auth attempts |
| **Rate Limiting** | TBD in Phase 6 |

## Success Criteria

- ✅ All 6 providers working
- ✅ Zero downtime migration
- ✅ Frontend compatibility maintained
- ✅ All tests passing
- ✅ Response time <= Platform service
- ✅ Documentation complete

## Post-Migration Enhancements

Future improvements (not in scope):

1. Additional OAuth providers (GitHub, Microsoft)
2. Multi-factor authentication (MFA)
3. Passkey/WebAuthn support
4. Token refresh mechanism
5. Enhanced session management
6. Advanced audit logging
7. Rate limiting improvements

## Getting Started

1. Read full proposal: `docs/proposals/auth-migration.md`
2. Review existing platform code: `services/platform/src/auth/`
3. Review nexus structure: `services/nexus/internal/http/`
4. Set up development environment
5. Begin Phase 1 implementation

## Questions?

- **Architecture**: See full proposal for detailed diagrams
- **Timeline**: 6 weeks for full migration
- **Risk**: Low - gradual rollout with rollback capability
- **Impact**: Zero frontend changes required

## Resources

- Full Proposal: [auth-migration.md](./auth-migration.md)
- Platform Auth Code: `services/platform/src/auth/`
- Nexus Auth Code: `services/nexus/internal/http/auth/`
- OpenAPI Specs: `services/nexus/internal/http/controllers/v*/oapi/`
