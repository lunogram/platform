# Authentication Migration Proposal: Platform → Nexus

## Executive Summary

This proposal outlines the migration of authentication endpoints from the Node.js Platform service to the Go Nexus service while maintaining the modular, provider-based architecture that supports multiple authentication strategies.

## Current Architecture

### Platform Service (Node.js/TypeScript)

**Location:** `services/platform/src/auth/`

**Core Components:**

1. **AuthProvider (Abstract Base Class)**
   - Provides common authentication flow
   - Handles admin creation/lookup
   - Generates OAuth tokens
   - Manages organization association
   - Abstract methods: `start()`, `validate()`, optional `webhook()`

2. **Six Authentication Providers:**

   a. **BasicAuthProvider**
   - Simple email/password authentication
   - Validates credentials against configured values
   - Returns to login form on start

   b. **EmailAuthProvider**
   - Magic link authentication via email
   - Uses SMTP for email delivery
   - Generates JWT with 15-minute expiration
   - No password required

   c. **SAMLAuthProvider**
   - SAML 2.0 SSO integration
   - Uses `@node-saml/node-saml` library
   - Supports both redirect and POST bindings
   - Handles organization cookies for multi-tenant

   d. **OpenIDAuthProvider**
   - OpenID Connect authentication
   - Uses `openid-client` library
   - Supports discovery from issuer URL
   - Manages nonce and state via cookies

   e. **GoogleAuthProvider**
   - Google OAuth integration
   - Wraps OpenIDAuthProvider with Google-specific config
   - Uses Google's OpenID Connect endpoint

   f. **CloudAuthProvider**
   - Clerk.com integration
   - JWT validation via JWKS
   - Webhook support for user lifecycle events (create/update/delete)
   - Uses Svix for webhook signature verification

3. **Supporting Components:**
   - `Admin` - User model with organization association
   - `AdminRepository` - Database operations for admins
   - `AccessToken` - Token storage model
   - `TokenRepository` - Token generation and management
   - `AuthMiddleware` - JWT validation middleware
   - `AuthController` - Route definitions

**API Endpoints:**
```
GET  /auth/methods                  - List available auth methods
POST /auth/check                    - Check if email has org-specific auth
GET  /auth/login/:driver            - Start authentication flow
POST /auth/login/:driver            - Start authentication flow (with body)
GET  /auth/login/:driver/callback   - Handle authentication callback
POST /auth/login/:driver/callback   - Handle authentication callback (POST)
POST /auth/:driver/webhook          - Handle provider webhooks
```

### Nexus Service (Go)

**Location:** `services/nexus/internal/http/auth/`

**Existing Components:**
- JWT authentication middleware
- API key authentication
- Admin store with full CRUD operations
- Access token storage (in database schema)
- OpenAPI-first approach with code generation

## Proposed Architecture

### 1. Package Structure

```
services/nexus/internal/http/auth/
├── auth.go              # Existing middleware (keep)
├── cloud.go             # Existing token extraction (keep)
├── provider.go          # New: Provider interface
├── registry.go          # New: Provider registry and factory
├── handlers.go          # New: HTTP handlers
├── token.go             # New: Token generation utilities
└── providers/
    ├── basic.go         # Basic auth implementation
    ├── email.go         # Email magic link implementation
    ├── saml.go          # SAML implementation
    ├── openid.go        # OpenID Connect implementation
    ├── google.go        # Google OAuth implementation
    └── clerk.go         # Clerk/Cloud implementation
```

### 2. Core Interfaces

```go
// Provider defines the interface that all auth providers must implement
type Provider interface {
    // Name returns the unique identifier for this provider
    Name() string
    
    // Start initiates the authentication flow
    // For redirect-based auth, this returns a URL to redirect to
    // For form-based auth, this returns a form URL
    Start(ctx context.Context, req StartRequest) (*StartResponse, error)
    
    // Validate completes the authentication flow
    // Validates the callback/credentials and returns admin information
    Validate(ctx context.Context, req ValidateRequest) (*ValidateResponse, error)
}

// WebhookProvider is an optional interface for providers that support webhooks
type WebhookProvider interface {
    Provider
    
    // HandleWebhook processes webhook events from the provider
    HandleWebhook(ctx context.Context, req WebhookRequest) error
}

// StartRequest contains common fields for starting authentication
type StartRequest struct {
    RedirectURL string            // Where to redirect after success
    Organization *uuid.UUID       // Optional organization context
    Query       url.Values        // Query parameters
    Body        map[string]any    // Request body (for POST)
}

// StartResponse contains the result of starting authentication
type StartResponse struct {
    RedirectURL string            // URL to redirect the user to
    SetCookies  []http.Cookie     // Cookies to set (for state management)
}

// ValidateRequest contains fields for validating authentication
type ValidateRequest struct {
    Query        url.Values       // Query parameters (for callbacks)
    Body         map[string]any   // Request body (for POST callbacks)
    Cookies      []*http.Cookie   // Request cookies
    Headers      http.Header      // Request headers (for webhooks)
    RawBody      []byte           // Raw request body (for webhook verification)
}

// ValidateResponse contains the authenticated admin information
type ValidateResponse struct {
    Admin        AdminInfo
    RedirectURL  string
    SetCookies   []http.Cookie    // Cookies to set/clear
}

// AdminInfo contains the admin details from the provider
type AdminInfo struct {
    Email        string
    ExternalID   *string          // For external identity providers
    FirstName    *string
    LastName     *string
    ImageURL     *string
}
```

### 3. Provider Registry

```go
// Registry manages available authentication providers
type Registry struct {
    providers map[string]Provider
    config    Config
}

// Config contains authentication configuration
type Config struct {
    TokenLifetime   time.Duration
    JWTSecret       string
    BaseURL         string
    APIBaseURL      string
    
    // Provider-specific configs
    Basic           *BasicConfig
    Email           *EmailConfig
    SAML            *SAMLConfig
    OpenID          *OpenIDConfig
    Google          *GoogleConfig
    Clerk           *ClerkConfig
}

// NewRegistry creates a new provider registry with configured providers
func NewRegistry(config Config) (*Registry, error) {
    r := &Registry{
        providers: make(map[string]Provider),
        config:    config,
    }
    
    // Register enabled providers based on config
    if config.Basic != nil {
        r.Register(NewBasicProvider(config.Basic))
    }
    if config.Email != nil {
        r.Register(NewEmailProvider(config.Email, config))
    }
    // ... etc for other providers
    
    return r, nil
}
```

### 4. HTTP Handlers

```go
type Handlers struct {
    registry *Registry
    stores   *store.Stores
    config   Config
    logger   *zap.Logger
}

// GetMethods returns list of available authentication methods
// GET /auth/methods
func (h *Handlers) GetMethods(w http.ResponseWriter, r *http.Request)

// CheckAuth checks if an email has organization-specific authentication
// POST /auth/check
func (h *Handlers) CheckAuth(w http.ResponseWriter, r *http.Request)

// StartAuth initiates authentication flow for a provider
// GET/POST /auth/login/:driver
func (h *Handlers) StartAuth(w http.ResponseWriter, r *http.Request)

// ValidateAuth handles authentication callback
// GET/POST /auth/login/:driver/callback
func (h *Handlers) ValidateAuth(w http.ResponseWriter, r *http.Request)

// HandleWebhook processes provider webhooks
// POST /auth/:driver/webhook
func (h *Handlers) HandleWebhook(w http.ResponseWriter, r *http.Request)
```

### 5. Provider Implementations

Each provider will be implemented in its own file following the interface:

#### BasicProvider
- Validates email/password against configured credentials
- No external dependencies
- Simplest implementation

#### EmailProvider
- Generates JWT-based magic links
- Sends email via SMTP
- 15-minute token expiration
- Dependencies: `net/smtp`, `gopkg.in/gomail.v2` or similar

#### SAMLProvider
- SAML 2.0 authentication
- Uses `github.com/crewjam/saml` library
- Handles RelayState for redirect URLs
- Organization cookie management
- Dependencies: `github.com/crewjam/saml`

#### OpenIDProvider
- OpenID Connect authentication
- Uses `github.com/coreos/go-oidc/v3` and `golang.org/x/oauth2`
- OIDC Discovery support
- Nonce and state management via cookies
- Dependencies: `github.com/coreos/go-oidc/v3`, `golang.org/x/oauth2`

#### GoogleProvider
- Wraps OpenIDProvider with Google-specific configuration
- Hardcoded issuer: `https://accounts.google.com`
- Response type: `id_token`

#### ClerkProvider
- Clerk.com integration
- JWT validation via JWKS
- Webhook support with Svix signature verification
- User lifecycle management (create/update/delete)
- Dependencies: `github.com/clerk/clerk-sdk-go`, `github.com/svix/svix-webhooks`

### 6. Token Management

```go
// GenerateAccessToken creates a new JWT token and stores it in the database
func GenerateAccessToken(ctx context.Context, stores *store.Stores, admin *store.Admin, req *http.Request, config Config) (*OAuthResponse, error)

// SetOAuthCookie sets the OAuth token cookie on the response
func SetOAuthCookie(w http.ResponseWriter, token *OAuthResponse, secure bool)

// ClearOAuthCookie removes the OAuth token cookie
func ClearOAuthCookie(w http.ResponseWriter)
```

### 7. Database Integration

Use existing store layer:
- `store.AdminsStore.CreateAdmin()` - Create new admin
- `store.AdminsStore.GetAdminByEmail()` - Lookup existing admin
- `store.AdminsStore.GetAdminByExternalID()` - Lookup by provider ID
- New: `store.AccessTokensStore` for token management

### 8. OpenAPI Specification

Add to management API (since auth is for admins):

```yaml
paths:
  /auth/methods:
    get:
      summary: List authentication methods
      description: Returns available authentication methods for the platform
      tags:
        - Authentication
      responses:
        '200':
          description: List of authentication methods
          content:
            application/json:
              schema:
                type: array
                items:
                  $ref: '#/components/schemas/AuthMethod'

  /auth/check:
    post:
      summary: Check authentication
      description: Check if an email has organization-specific authentication configured
      tags:
        - Authentication
      requestBody:
        required: true
        content:
          application/json:
            schema:
              type: object
              required:
                - email
              properties:
                email:
                  type: string
                  format: email
      responses:
        '200':
          description: Authentication check result
          content:
            application/json:
              schema:
                type: object
                properties:
                  hasAuth:
                    type: boolean

  /auth/login/{driver}:
    get:
      summary: Start authentication
      description: Initiates authentication flow for the specified driver
      tags:
        - Authentication
      parameters:
        - name: driver
          in: path
          required: true
          schema:
            $ref: '#/components/schemas/AuthDriver'
        - name: r
          in: query
          description: Redirect URL after successful authentication
          schema:
            type: string
      responses:
        '302':
          description: Redirect to authentication provider or form
        '204':
          description: No content (for email provider after sending email)
        default:
          $ref: '#/components/responses/Error'
    post:
      summary: Start authentication (POST)
      description: Initiates authentication flow with request body
      tags:
        - Authentication
      parameters:
        - name: driver
          in: path
          required: true
          schema:
            $ref: '#/components/schemas/AuthDriver'
        - name: r
          in: query
          description: Redirect URL after successful authentication
          schema:
            type: string
      requestBody:
        content:
          application/json:
            schema:
              type: object
              additionalProperties: true
      responses:
        '302':
          description: Redirect to authentication provider or form
        '204':
          description: No content (for email provider after sending email)
        default:
          $ref: '#/components/responses/Error'

  /auth/login/{driver}/callback:
    get:
      summary: Authentication callback
      description: Handles authentication callback from provider
      tags:
        - Authentication
      parameters:
        - name: driver
          in: path
          required: true
          schema:
            $ref: '#/components/schemas/AuthDriver'
      responses:
        '302':
          description: Redirect to application with authentication cookie set
        default:
          $ref: '#/components/responses/Error'
    post:
      summary: Authentication callback (POST)
      description: Handles POST authentication callback from provider
      tags:
        - Authentication
      parameters:
        - name: driver
          in: path
          required: true
          schema:
            $ref: '#/components/schemas/AuthDriver'
      requestBody:
        content:
          application/x-www-form-urlencoded:
            schema:
              type: object
              additionalProperties: true
          application/json:
            schema:
              type: object
              additionalProperties: true
      responses:
        '302':
          description: Redirect to application with authentication cookie set
        default:
          $ref: '#/components/responses/Error'

  /auth/{driver}/webhook:
    post:
      summary: Provider webhook
      description: Handles webhooks from authentication providers
      tags:
        - Authentication
      parameters:
        - name: driver
          in: path
          required: true
          schema:
            $ref: '#/components/schemas/AuthDriver'
      requestBody:
        content:
          application/json:
            schema:
              type: object
              additionalProperties: true
      responses:
        '200':
          description: Webhook processed successfully
        '404':
          description: Provider does not support webhooks
        default:
          $ref: '#/components/responses/Error'

components:
  schemas:
    AuthMethod:
      type: object
      required:
        - driver
        - name
      properties:
        driver:
          $ref: '#/components/schemas/AuthDriver'
        name:
          type: string
          description: Display name for the authentication method
        publicConfig:
          type: object
          additionalProperties: true
          description: Public configuration visible to clients

    AuthDriver:
      type: string
      enum:
        - basic
        - email
        - saml
        - openid
        - google
        - clerk
```

## Migration Strategy

### Phase 1: Foundation (Week 1)
1. Create provider interface and registry
2. Implement token generation utilities
3. Create HTTP handlers scaffold
4. Add OpenAPI specification
5. Generate OAPI types with `make generate`

### Phase 2: Simple Providers (Week 2)
1. Implement BasicProvider
2. Implement EmailProvider (requires SMTP config)
3. Add integration tests for simple flows
4. Test with existing frontend

### Phase 3: OAuth/OIDC Providers (Week 3)
1. Implement OpenIDProvider
2. Implement GoogleProvider
3. Add cookie management for state/nonce
4. Integration tests

### Phase 4: SAML Provider (Week 4)
1. Implement SAMLProvider
2. Handle both redirect and POST bindings
3. Integration tests with test IdP

### Phase 5: Cloud Provider (Week 5)
1. Implement ClerkProvider
2. Add webhook handler
3. Test user lifecycle events
4. Integration tests

### Phase 6: Cutover (Week 6)
1. Feature flag to enable new auth endpoints
2. Run both services in parallel
3. Monitor for errors
4. Gradual traffic migration
5. Remove old endpoints from platform service

## Testing Strategy

### Unit Tests
- Each provider implementation
- Token generation
- Cookie management
- Webhook signature verification

### Integration Tests
- Full authentication flow for each provider
- Database operations (admin creation/lookup)
- Token validation
- Cookie handling

### Test Providers
- Use test SAML IdP (e.g., `samltest.id`)
- Use Google OAuth test project
- Use Clerk test instance
- Mock SMTP for email testing

## Configuration

Add to nexus config:

```go
type AuthConfig struct {
    TokenLifetime   time.Duration `yaml:"tokenLifetime"`
    JWTSecret       string        `yaml:"jwtSecret" env:"JWT_SECRET"`
    
    Drivers         []string      `yaml:"drivers"` // Enabled auth providers
    
    Basic           *BasicConfig  `yaml:"basic"`
    Email           *EmailConfig  `yaml:"email"`
    SAML            *SAMLConfig   `yaml:"saml"`
    OpenID          *OpenIDConfig `yaml:"openid"`
    Google          *GoogleConfig `yaml:"google"`
    Clerk           *ClerkConfig  `yaml:"clerk"`
}

type BasicConfig struct {
    Name     string `yaml:"name"`
    Email    string `yaml:"email"`
    Password string `yaml:"password" env:"AUTH_BASIC_PASSWORD"`
}

type EmailConfig struct {
    Name     string      `yaml:"name"`
    From     string      `yaml:"from"`
    SMTP     SMTPConfig  `yaml:"smtp"`
}

type SAMLConfig struct {
    Name              string `yaml:"name"`
    CallbackURL       string `yaml:"callbackUrl"`
    EntryPoint        string `yaml:"entryPoint"`
    Issuer            string `yaml:"issuer"`
    Certificate       string `yaml:"certificate" env:"AUTH_SAML_CERT"`
    // ... other SAML config
}

// ... similar for other providers
```

## Dependencies

New Go dependencies to add:
```
github.com/crewjam/saml                  # SAML support
github.com/coreos/go-oidc/v3             # OpenID Connect
golang.org/x/oauth2                      # OAuth2 flows
github.com/clerk/clerk-sdk-go/v2         # Clerk integration
github.com/svix/svix-webhooks/go         # Webhook verification
gopkg.in/gomail.v2                       # Email sending
```

## Backwards Compatibility

- Maintain exact same endpoint structure
- Same cookie names and format
- Same JWT token structure
- Same database schema
- Frontend requires no changes

## Security Considerations

1. **Secrets Management**: All secrets in environment variables
2. **CSRF Protection**: State/nonce validation for OAuth flows
3. **Token Rotation**: Support for token refresh (future)
4. **Rate Limiting**: Add to auth endpoints
5. **Audit Logging**: Log all authentication attempts
6. **Webhook Verification**: Validate webhook signatures

## Rollback Plan

1. Keep platform service running during migration
2. Use feature flag to switch between services
3. If issues arise, redirect auth endpoints back to platform
4. Full rollback possible within 5 minutes

## Success Metrics

- All 6 auth providers working in nexus
- Zero downtime during migration
- Same or better response times
- Frontend compatibility maintained
- All tests passing
- Documentation complete

## Future Enhancements (Post-Migration)

1. Add more OAuth providers (GitHub, Microsoft)
2. Multi-factor authentication (MFA)
3. Passkey/WebAuthn support
4. Token refresh/rotation
5. Session management improvements
6. Better audit logging
7. Rate limiting improvements
