# Auth Migration Implementation Checklist

Use this checklist to track progress during implementation.

## Phase 1: Foundation (Week 1)

### Core Architecture
- [ ] Create `internal/http/auth/provider.go`
  - [ ] Define `Provider` interface
  - [ ] Define `WebhookProvider` interface
  - [ ] Define `StartRequest`, `StartResponse` types
  - [ ] Define `ValidateRequest`, `ValidateResponse` types
  - [ ] Define `WebhookRequest` type
  - [ ] Define `AdminInfo` type

- [ ] Create `internal/http/auth/registry.go`
  - [ ] Implement `Registry` struct
  - [ ] Implement `NewRegistry()` constructor
  - [ ] Implement `Register()` method
  - [ ] Implement `Get()` method
  - [ ] Implement `List()` method
  - [ ] Add configuration struct

- [ ] Create `internal/http/auth/token.go`
  - [ ] Implement `GenerateAccessToken()`
  - [ ] Implement `SetOAuthCookie()`
  - [ ] Implement `ClearOAuthCookie()`
  - [ ] Add `OAuthResponse` type

- [ ] Create `internal/http/auth/handlers.go`
  - [ ] Implement `Handlers` struct
  - [ ] Implement `GetMethods()` handler
  - [ ] Implement `CheckAuth()` handler
  - [ ] Implement `StartAuth()` handler
  - [ ] Implement `ValidateAuth()` handler
  - [ ] Implement `HandleWebhook()` handler
  - [ ] Add helper functions

### Database Layer
- [ ] Create `internal/store/access_tokens.go`
  - [ ] Define `AccessToken` struct
  - [ ] Implement `CreateAccessToken()`
  - [ ] Implement `GetAccessToken()`
  - [ ] Implement `DeleteAccessToken()`
  - [ ] Implement `CleanupExpiredTokens()`
  - [ ] Add to `Stores` struct

- [ ] Update admin store
  - [ ] Add `GetOrCreateAdmin()` method
  - [ ] Add `GetAdminByEmailGlobal()` (across orgs)

### OpenAPI Specification
- [ ] Update `services/nexus/internal/http/controllers/v1/management/oapi/resources.yml`
  - [ ] Add `/auth/methods` endpoint
  - [ ] Add `/auth/check` endpoint
  - [ ] Add `/auth/login/{driver}` endpoints (GET/POST)
  - [ ] Add `/auth/login/{driver}/callback` endpoints (GET/POST)
  - [ ] Add `/auth/{driver}/webhook` endpoint
  - [ ] Add `AuthMethod` schema
  - [ ] Add `AuthDriver` enum schema
  - [ ] Add `CheckAuthRequest` schema
  - [ ] Add `CheckAuthResponse` schema

- [ ] Generate types
  - [ ] Run `make generate`
  - [ ] Verify generated code compiles
  - [ ] Fix any compilation errors

### Configuration
- [ ] Update `internal/config/config.go`
  - [ ] Add `AuthConfig` struct
  - [ ] Add provider-specific config structs
  - [ ] Add environment variable mappings
  - [ ] Add validation

### Integration
- [ ] Wire up routes in `internal/http/controllers/v1/management/http.go`
  - [ ] Register auth handlers
  - [ ] Add middleware if needed
  - [ ] Update router

### Testing
- [ ] Create `internal/http/auth/provider_test.go`
  - [ ] Test interface compliance
- [ ] Create `internal/http/auth/registry_test.go`
  - [ ] Test provider registration
  - [ ] Test provider lookup
- [ ] Create `internal/http/auth/token_test.go`
  - [ ] Test token generation
  - [ ] Test cookie management

### Documentation
- [ ] Update README with auth configuration examples
- [ ] Document environment variables

## Phase 2: Simple Providers (Week 2)

### Basic Provider
- [ ] Create `internal/http/auth/providers/basic.go`
  - [ ] Implement `BasicProvider` struct
  - [ ] Implement `Name()` method
  - [ ] Implement `Start()` method (redirect to form)
  - [ ] Implement `Validate()` method (check credentials)
  - [ ] Add `BasicConfig` struct

- [ ] Create `internal/http/auth/providers/basic_test.go`
  - [ ] Test credential validation (success)
  - [ ] Test credential validation (failure)
  - [ ] Test missing credentials
  - [ ] Test start flow

### Email Provider
- [ ] Add dependencies
  - [ ] Add `gopkg.in/gomail.v2` to `go.mod`
  - [ ] Run `go mod tidy`

- [ ] Create `internal/http/auth/providers/email.go`
  - [ ] Implement `EmailProvider` struct
  - [ ] Implement `Name()` method
  - [ ] Implement `Start()` method (send email)
  - [ ] Implement `Validate()` method (verify token)
  - [ ] Add `EmailConfig` struct
  - [ ] Implement `sendEmail()` helper
  - [ ] Implement `generateMagicLink()` helper
  - [ ] Add email template

- [ ] Create `internal/http/auth/providers/email_test.go`
  - [ ] Test token generation
  - [ ] Test token validation (success)
  - [ ] Test token validation (expired)
  - [ ] Test token validation (invalid)
  - [ ] Mock SMTP for testing

### Integration Tests
- [ ] Create `internal/http/auth/integration_test.go`
  - [ ] Test basic auth flow end-to-end
  - [ ] Test email auth flow end-to-end
  - [ ] Test admin creation
  - [ ] Test token storage
  - [ ] Use testcontainers for DB

### Configuration
- [ ] Add example config for basic provider
- [ ] Add example config for email provider
- [ ] Document SMTP configuration

### Manual Testing
- [ ] Test basic auth in browser
- [ ] Test email auth in browser
- [ ] Verify cookies are set correctly
- [ ] Verify redirect works
- [ ] Test with frontend

## Phase 3: OAuth/OIDC Providers (Week 3)

### Dependencies
- [ ] Add `github.com/coreos/go-oidc/v3` to `go.mod`
- [ ] Add `golang.org/x/oauth2` to `go.mod`
- [ ] Run `go mod tidy`

### OpenID Provider
- [ ] Create `internal/http/auth/providers/openid.go`
  - [ ] Implement `OpenIDProvider` struct
  - [ ] Implement `Name()` method
  - [ ] Implement `Start()` method (redirect to provider)
  - [ ] Implement `Validate()` method (exchange code)
  - [ ] Add `OpenIDConfig` struct
  - [ ] Implement OIDC discovery
  - [ ] Implement nonce management
  - [ ] Implement state management
  - [ ] Add cookie helpers

- [ ] Create `internal/http/auth/providers/openid_test.go`
  - [ ] Test authorization URL generation
  - [ ] Test token exchange (mocked)
  - [ ] Test claims parsing
  - [ ] Test nonce validation
  - [ ] Test state validation

### Google Provider
- [ ] Create `internal/http/auth/providers/google.go`
  - [ ] Implement `GoogleProvider` struct
  - [ ] Wrap `OpenIDProvider` with Google config
  - [ ] Implement `Name()` method
  - [ ] Implement `Start()` method (delegate)
  - [ ] Implement `Validate()` method (delegate)
  - [ ] Add `GoogleConfig` struct

- [ ] Create `internal/http/auth/providers/google_test.go`
  - [ ] Test provider initialization
  - [ ] Test delegation to OpenID

### Integration Tests
- [ ] Test OpenID flow with test provider
- [ ] Test Google flow (if test account available)
- [ ] Test cookie state management
- [ ] Test nonce validation
- [ ] Test callback handling

### Configuration
- [ ] Add example config for OpenID provider
- [ ] Add example config for Google provider
- [ ] Document callback URL setup

### Manual Testing
- [ ] Test OpenID auth in browser
- [ ] Test Google auth in browser
- [ ] Verify cookies are set correctly
- [ ] Test callback handling
- [ ] Test with frontend

## Phase 4: SAML Provider (Week 4)

### Dependencies
- [ ] Add `github.com/crewjam/saml` to `go.mod`
- [ ] Run `go mod tidy`

### SAML Provider
- [ ] Create `internal/http/auth/providers/saml.go`
  - [ ] Implement `SAMLProvider` struct
  - [ ] Implement `Name()` method
  - [ ] Implement `Start()` method (AuthnRequest)
  - [ ] Implement `Validate()` method (Assertion)
  - [ ] Add `SAMLConfig` struct
  - [ ] Handle redirect binding
  - [ ] Handle POST binding
  - [ ] Implement RelayState management
  - [ ] Parse SAML assertions
  - [ ] Extract profile attributes

- [ ] Create `internal/http/auth/providers/saml_test.go`
  - [ ] Test AuthnRequest generation
  - [ ] Test assertion parsing
  - [ ] Test signature verification
  - [ ] Test attribute extraction
  - [ ] Mock SAML responses

### Integration Tests
- [ ] Test SAML flow with test IdP (samltest.id)
- [ ] Test redirect binding
- [ ] Test POST binding
- [ ] Test RelayState handling

### Configuration
- [ ] Add example config for SAML provider
- [ ] Document certificate setup
- [ ] Document IdP configuration

### Manual Testing
- [ ] Test SAML auth with test IdP
- [ ] Verify metadata generation
- [ ] Test callback handling
- [ ] Test with frontend

## Phase 5: Cloud Provider (Week 5)

### Dependencies
- [ ] Add `github.com/clerk/clerk-sdk-go/v2` to `go.mod`
- [ ] Add `github.com/svix/svix-webhooks/go` to `go.mod`
- [ ] Run `go mod tidy`

### Clerk Provider
- [ ] Create `internal/http/auth/providers/clerk.go`
  - [ ] Implement `ClerkProvider` struct
  - [ ] Implement `Name()` method
  - [ ] Implement `Start()` method (not used)
  - [ ] Implement `Validate()` method (verify JWT)
  - [ ] Implement `HandleWebhook()` method
  - [ ] Add `ClerkConfig` struct
  - [ ] Handle user.created webhook
  - [ ] Handle user.updated webhook
  - [ ] Handle user.deleted webhook
  - [ ] Verify webhook signatures

- [ ] Create `internal/http/auth/providers/clerk_test.go`
  - [ ] Test JWT validation
  - [ ] Test webhook signature verification
  - [ ] Test user.created event
  - [ ] Test user.updated event
  - [ ] Test user.deleted event
  - [ ] Mock Clerk API

### Integration Tests
- [ ] Test Clerk JWT validation
- [ ] Test webhook handling
- [ ] Test user lifecycle events
- [ ] Test with Clerk test instance

### Configuration
- [ ] Add example config for Clerk provider
- [ ] Document webhook setup
- [ ] Document Clerk dashboard configuration

### Manual Testing
- [ ] Test Clerk auth in browser
- [ ] Test webhook delivery
- [ ] Verify user sync
- [ ] Test with frontend

## Phase 6: Cutover (Week 6)

### Feature Flag
- [ ] Add auth feature flag to config
- [ ] Implement flag check in routing
- [ ] Test flag switching

### Monitoring
- [ ] Add auth metrics
  - [ ] Login attempts
  - [ ] Login successes
  - [ ] Login failures
  - [ ] Provider usage
  - [ ] Response times
- [ ] Add auth logging
  - [ ] Structured logging for auth events
  - [ ] Error tracking
  - [ ] Audit trail

### Performance Testing
- [ ] Load test basic auth
- [ ] Load test email auth
- [ ] Load test OAuth flows
- [ ] Compare with platform service
- [ ] Identify bottlenecks

### Security Review
- [ ] Review token generation
- [ ] Review cookie security
- [ ] Review CSRF protection
- [ ] Review webhook verification
- [ ] Review error messages (no user enumeration)
- [ ] Review rate limiting
- [ ] Penetration testing

### Documentation
- [ ] Update API documentation
- [ ] Update deployment guide
- [ ] Update runbook
- [ ] Create troubleshooting guide
- [ ] Update configuration reference

### Deployment
- [ ] Deploy to staging
- [ ] Run smoke tests
- [ ] Enable feature flag in staging
- [ ] Verify all providers work
- [ ] Test with frontend in staging
- [ ] Monitor metrics

### Production Cutover
- [ ] Deploy to production
- [ ] Keep feature flag off initially
- [ ] Monitor platform service metrics
- [ ] Enable flag for 10% traffic
- [ ] Monitor for errors
- [ ] Gradually increase to 50%
- [ ] Monitor for 24 hours
- [ ] Increase to 100%
- [ ] Monitor for 48 hours

### Cleanup
- [ ] Remove old endpoints from platform service
- [ ] Remove unused dependencies
- [ ] Archive old code
- [ ] Update frontend to point directly to nexus
- [ ] Remove proxy routes

### Post-Migration
- [ ] Collect metrics
- [ ] Document lessons learned
- [ ] Identify optimization opportunities
- [ ] Plan future enhancements

## Success Criteria

Verify all items are complete:

- [ ] All 6 providers implemented and tested
- [ ] All integration tests passing
- [ ] All unit tests passing
- [ ] OpenAPI spec complete and validated
- [ ] Frontend compatibility verified
- [ ] Security review passed
- [ ] Performance benchmarks met
- [ ] Documentation complete
- [ ] Monitoring in place
- [ ] Zero downtime migration
- [ ] No user-facing issues
- [ ] Rollback plan tested
- [ ] Team trained on new system

## Notes

Use this section to track issues, decisions, and important information during implementation:

```
Date: ____
Issue: 
Resolution:

Date: ____
Decision:
Rationale:

Date: ____
Performance note:
```
