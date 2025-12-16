# Authentication Architecture Diagrams

Visual representations of the authentication system architecture before and after migration.

## Current Architecture (Platform Service)

```
┌─────────────────────────────────────────────────────────────────┐
│                      Frontend (React)                           │
│                                                                 │
│  Components:                                                    │
│  • LoginForm                                                    │
│  • OAuthButtons                                                 │
│  • SessionManager                                               │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         │ HTTP/HTTPS
                         │
┌────────────────────────▼────────────────────────────────────────┐
│              Platform Service (Node.js/Koa)                     │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │              AuthController                               │  │
│  │  GET  /auth/methods                                      │  │
│  │  POST /auth/check                                        │  │
│  │  GET  /auth/login/:driver                                │  │
│  │  POST /auth/login/:driver                                │  │
│  │  GET  /auth/login/:driver/callback                       │  │
│  │  POST /auth/login/:driver/callback                       │  │
│  │  POST /auth/:driver/webhook                              │  │
│  └──────────────────────┬───────────────────────────────────┘  │
│                         │                                       │
│  ┌──────────────────────▼───────────────────────────────────┐  │
│  │              Auth.ts (Factory)                           │  │
│  │  • initProvider()                                        │  │
│  │  • loadProvider()                                        │  │
│  │  • authMethods()                                         │  │
│  └──────────────────────┬───────────────────────────────────┘  │
│                         │                                       │
│  ┌──────────────────────▼───────────────────────────────────┐  │
│  │         AuthProvider (Abstract Class)                    │  │
│  │  • start(ctx): Promise<void>                             │  │
│  │  • validate(ctx): Promise<void>                          │  │
│  │  • webhook?(ctx): Promise<void>                          │  │
│  │  • login(params, ctx): Promise<OAuthResponse>            │  │
│  │  • loadAuthOrganization(ctx)                             │  │
│  └──────────────────────┬───────────────────────────────────┘  │
│                         │                                       │
│         ┌───────────────┼───────────────┐                       │
│         │               │               │                       │
│  ┌──────▼──────┐ ┌─────▼─────┐ ┌──────▼──────┐               │
│  │    Basic    │ │   Email   │ │    SAML     │               │
│  │  Provider   │ │ Provider  │ │  Provider   │               │
│  └─────────────┘ └───────────┘ └─────────────┘               │
│         │               │               │                       │
│  ┌──────▼──────┐ ┌─────▼─────┐ ┌──────▼──────┐               │
│  │   OpenID    │ │  Google   │ │    Cloud    │               │
│  │  Provider   │ │ Provider  │ │  Provider   │               │
│  └─────────────┘ └───────────┘ └─────────────┘               │
│                         │                                       │
│  ┌──────────────────────▼───────────────────────────────────┐  │
│  │         AdminRepository & TokenRepository                │  │
│  │  • getAdminByEmail()                                     │  │
│  │  • createOrUpdateAdmin()                                 │  │
│  │  • generateAccessToken()                                 │  │
│  │  • setCookiesOauthToken()                                │  │
│  └──────────────────────┬───────────────────────────────────┘  │
└─────────────────────────┼───────────────────────────────────────┘
                          │
                          │
┌─────────────────────────▼───────────────────────────────────────┐
│                    PostgreSQL Database                          │
│  Tables:                                                        │
│  • admins (id, organization_id, email, external_id, ...)       │
│  • access_tokens (id, admin_id, token, expires_at, ...)        │
│  • organizations (id, auth config, ...)                         │
└─────────────────────────────────────────────────────────────────┘
```

## Target Architecture (Nexus Service)

```
┌─────────────────────────────────────────────────────────────────┐
│                      Frontend (React)                           │
│                     (No Changes Required)                       │
│                                                                 │
│  Components:                                                    │
│  • LoginForm                                                    │
│  • OAuthButtons                                                 │
│  • SessionManager                                               │
└────────────────────────┬────────────────────────────────────────┘
                         │
                         │ HTTP/HTTPS (Same URLs)
                         │
┌────────────────────────▼────────────────────────────────────────┐
│                  Nexus Service (Go/Chi)                         │
│                                                                 │
│  ┌──────────────────────────────────────────────────────────┐  │
│  │              Auth Handlers (handlers.go)                  │  │
│  │  GetMethods(w, r)      → GET  /auth/methods              │  │
│  │  CheckAuth(w, r)       → POST /auth/check                │  │
│  │  StartAuth(w, r)       → GET/POST /auth/login/:driver    │  │
│  │  ValidateAuth(w, r)    → GET/POST /auth/login/:driver/   │  │
│  │                            callback                        │  │
│  │  HandleWebhook(w, r)   → POST /auth/:driver/webhook      │  │
│  └──────────────────────┬───────────────────────────────────┘  │
│                         │                                       │
│  ┌──────────────────────▼───────────────────────────────────┐  │
│  │           Provider Registry (registry.go)                │  │
│  │  • providers map[string]Provider                         │  │
│  │  • Register(Provider)                                    │  │
│  │  • Get(name string) Provider                             │  │
│  │  • List() []AuthMethod                                   │  │
│  └──────────────────────┬───────────────────────────────────┘  │
│                         │                                       │
│  ┌──────────────────────▼───────────────────────────────────┐  │
│  │          Provider Interface (provider.go)                │  │
│  │  type Provider interface {                               │  │
│  │    Name() string                                         │  │
│  │    Start(ctx, StartRequest) (*StartResponse, error)      │  │
│  │    Validate(ctx, ValidateRequest)                        │  │
│  │             (*ValidateResponse, error)                   │  │
│  │  }                                                        │  │
│  │                                                           │  │
│  │  type WebhookProvider interface {                        │  │
│  │    Provider                                              │  │
│  │    HandleWebhook(ctx, WebhookRequest) error             │  │
│  │  }                                                        │  │
│  └──────────────────────┬───────────────────────────────────┘  │
│                         │                                       │
│         ┌───────────────┼───────────────┐                       │
│         │               │               │                       │
│  ┌──────▼──────┐ ┌─────▼─────┐ ┌──────▼──────┐               │
│  │ providers/  │ │ providers/│ │ providers/  │               │
│  │  basic.go   │ │ email.go  │ │  saml.go    │               │
│  │             │ │           │ │             │               │
│  │ Basic       │ │ Email     │ │ SAML        │               │
│  │ Provider    │ │ Provider  │ │ Provider    │               │
│  └─────────────┘ └───────────┘ └─────────────┘               │
│         │               │               │                       │
│  ┌──────▼──────┐ ┌─────▼─────┐ ┌──────▼──────┐               │
│  │ providers/  │ │ providers/│ │ providers/  │               │
│  │ openid.go   │ │ google.go │ │  clerk.go   │               │
│  │             │ │           │ │             │               │
│  │ OpenID      │ │ Google    │ │ Clerk       │               │
│  │ Provider    │ │ Provider  │ │ Provider    │               │
│  └─────────────┘ └───────────┘ └─────────────┘               │
│                         │                                       │
│  ┌──────────────────────▼───────────────────────────────────┐  │
│  │           Token Management (token.go)                    │  │
│  │  • GenerateAccessToken()                                 │  │
│  │  • SetOAuthCookie()                                      │  │
│  │  • ClearOAuthCookie()                                    │  │
│  └──────────────────────┬───────────────────────────────────┘  │
│                         │                                       │
│  ┌──────────────────────▼───────────────────────────────────┐  │
│  │              Store Layer (store/)                        │  │
│  │  • AdminsStore                                           │  │
│  │    - CreateAdmin()                                       │  │
│  │    - GetAdminByEmail()                                   │  │
│  │    - GetAdminByExternalID()                              │  │
│  │  • AccessTokensStore                                     │  │
│  │    - CreateAccessToken()                                 │  │
│  │    - GetAccessToken()                                    │  │
│  │  • OrganizationsStore                                    │  │
│  │    - GetOrganization()                                   │  │
│  └──────────────────────┬───────────────────────────────────┘  │
└─────────────────────────┼───────────────────────────────────────┘
                          │
                          │
┌─────────────────────────▼───────────────────────────────────────┐
│                    PostgreSQL Database                          │
│  Tables: (No schema changes)                                    │
│  • admins (id, organization_id, email, external_id, ...)       │
│  • access_tokens (id, admin_id, token, expires_at, ...)        │
│  • organizations (id, auth config, ...)                         │
└─────────────────────────────────────────────────────────────────┘
```

## Authentication Flow Sequence

### Example: Google OAuth Flow

```
┌─────────┐         ┌──────────┐         ┌────────┐         ┌──────────┐
│ Browser │         │  Nexus   │         │ Google │         │ Database │
└────┬────┘         └─────┬────┘         └────┬───┘         └─────┬────┘
     │                    │                   │                   │
     │ 1. Click "Login"   │                   │                   │
     ├───────────────────>│                   │                   │
     │ GET /auth/login/   │                   │                   │
     │     google?r=/app  │                   │                   │
     │                    │                   │                   │
     │                    │ 2. Get Provider   │                   │
     │                    ├──┐                │                   │
     │                    │  │ registry.Get() │                   │
     │                    │<─┘                │                   │
     │                    │                   │                   │
     │                    │ 3. Start Auth     │                   │
     │                    ├──┐                │                   │
     │                    │  │ provider.Start()│                  │
     │                    │<─┘ Generate state│                   │
     │                    │    + nonce        │                   │
     │                    │                   │                   │
     │ 4. Redirect + Cookies                 │                   │
     │<───────────────────┤                   │                   │
     │ Set-Cookie: nonce  │                   │                   │
     │ Set-Cookie: state  │                   │                   │
     │ Location: google   │                   │                   │
     │                    │                   │                   │
     │ 5. Redirect to Google                 │                   │
     ├──────────────────────────────────────>│                   │
     │                    │                   │                   │
     │ 6. Login Form      │                   │                   │
     │<───────────────────────────────────────┤                   │
     │                    │                   │                   │
     │ 7. Submit Credentials                  │                   │
     ├───────────────────────────────────────>│                   │
     │                    │                   │                   │
     │ 8. Redirect back   │                   │                   │
     │<───────────────────────────────────────┤                   │
     │ /auth/login/google/callback            │                   │
     │ ?code=xyz&state=abc                    │                   │
     │                    │                   │                   │
     │ 9. Callback        │                   │                   │
     ├───────────────────>│                   │                   │
     │ GET /auth/login/   │                   │                   │
     │ google/callback    │                   │                   │
     │                    │                   │                   │
     │                    │ 10. Get Provider  │                   │
     │                    ├──┐                │                   │
     │                    │<─┘                │                   │
     │                    │                   │                   │
     │                    │ 11. Validate      │                   │
     │                    ├──┐                │                   │
     │                    │  │ provider.      │                   │
     │                    │  │ Validate()     │                   │
     │                    │  │                │                   │
     │                    │  │ 12. Exchange code                  │
     │                    │  ├───────────────>│                   │
     │                    │  │                │                   │
     │                    │  │ 13. ID Token   │                   │
     │                    │  │<───────────────┤                   │
     │                    │  │                │                   │
     │                    │  │ 14. Verify     │                   │
     │                    │  │     nonce/state│                   │
     │                    │  │     Parse claims│                  │
     │                    │<─┘                │                   │
     │                    │                   │                   │
     │                    │ 15. Get or Create Admin              │
     │                    ├──────────────────────────────────────>│
     │                    │  SELECT/INSERT INTO admins           │
     │                    │                   │                   │
     │                    │ 16. Admin record  │                   │
     │                    │<───────────────────────────────────────┤
     │                    │                   │                   │
     │                    │ 17. Generate JWT  │                   │
     │                    ├──┐                │                   │
     │                    │<─┘                │                   │
     │                    │                   │                   │
     │                    │ 18. Store Token   │                   │
     │                    ├──────────────────────────────────────>│
     │                    │  INSERT INTO access_tokens           │
     │                    │                   │                   │
     │                    │ 19. Success       │                   │
     │                    │<───────────────────────────────────────┤
     │                    │                   │                   │
     │ 20. Redirect + Cookie                 │                   │
     │<───────────────────┤                   │                   │
     │ Set-Cookie: oauth= │                   │                   │
     │    {"access_token":│                   │                   │
     │     "jwt...",      │                   │                   │
     │     "expires_at":..│                   │                   │
     │ Location: /app     │                   │                   │
     │                    │                   │                   │
     │ 21. Access App     │                   │                   │
     │ (authenticated)    │                   │                   │
     │                    │                   │                   │
```

## Provider Interface Design

```
┌─────────────────────────────────────────────────────────────┐
│                      Provider Interface                     │
│                                                             │
│  +Name() string                                             │
│  +Start(ctx, StartRequest) (*StartResponse, error)          │
│  +Validate(ctx, ValidateRequest) (*ValidateResponse, error) │
└─────────────────────────┬───────────────────────────────────┘
                          │
                          │ implements
                          │
        ┌─────────────────┼─────────────────┐
        │                 │                 │
┌───────▼────────┐  ┌────▼──────────┐  ┌──▼──────────────┐
│ BasicProvider  │  │ EmailProvider │  │ SAMLProvider    │
│                │  │               │  │                 │
│ Simple         │  │ Magic Link    │  │ Enterprise SSO  │
│ Credentials    │  │ via SMTP      │  │ SAML 2.0        │
└────────────────┘  └───────────────┘  └─────────────────┘
        │                 │                 │
┌───────▼────────┐  ┌────▼──────────┐  ┌──▼──────────────┐
│ OpenIDProvider │  │GoogleProvider │  │ ClerkProvider   │
│                │  │               │  │                 │
│ OIDC Standard  │  │ Wraps OpenID  │  │ Managed Auth    │
│ with Discovery │  │ for Google    │  │ + Webhooks      │
└────────────────┘  └───────────────┘  └────────┬────────┘
                                                 │
                                                 │ also implements
                                                 │
                          ┌──────────────────────▼──────┐
                          │   WebhookProvider Interface │
                          │                             │
                          │ +HandleWebhook(ctx,         │
                          │   WebhookRequest) error     │
                          └─────────────────────────────┘
```

## Data Flow: Token Generation

```
┌──────────────────────────────────────────────────────────────┐
│              Provider.Validate() Returns                     │
│              ValidateResponse                                │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ AdminInfo {                                             │ │
│  │   Email:      "user@example.com"                        │ │
│  │   ExternalID: "google-oauth2|123456"                    │ │
│  │   FirstName:  "John"                                    │ │
│  │   LastName:   "Doe"                                     │ │
│  │   ImageURL:   "https://..."                             │ │
│  │ }                                                        │ │
│  └────────────────────────────────────────────────────────┘ │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│              Get or Create Admin                             │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ 1. Query: SELECT * FROM admins                          │ │
│  │    WHERE email = 'user@example.com'                     │ │
│  │    AND organization_id = $1                             │ │
│  │                                                          │ │
│  │ 2. If not found:                                        │ │
│  │    INSERT INTO admins (                                 │ │
│  │      organization_id, email, external_id,               │ │
│  │      first_name, last_name, image_url, role             │ │
│  │    ) VALUES (...)                                       │ │
│  │                                                          │ │
│  │ 3. Return: Admin record with UUID                       │ │
│  └────────────────────────────────────────────────────────┘ │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│              Generate JWT Token                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ jwt.NewWithClaims(jwt.SigningMethodHS256,               │ │
│  │   RegisteredClaims{                                     │ │
│  │     Subject:   admin.ID.String()                        │ │
│  │     Issuer:    "https://api.lunogram.com"               │ │
│  │     ExpiresAt: Now + 24h                                │ │
│  │   }                                                      │ │
│  │ )                                                        │ │
│  │                                                          │ │
│  │ token.SignedString(config.JWTSecret)                    │ │
│  └────────────────────────────────────────────────────────┘ │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│              Store Access Token                              │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ INSERT INTO access_tokens (                             │ │
│  │   admin_id,                                             │ │
│  │   token,                                                │ │
│  │   expires_at,                                           │ │
│  │   ip,                                                   │ │
│  │   user_agent                                            │ │
│  │ ) VALUES ($1, $2, $3, $4, $5)                           │ │
│  └────────────────────────────────────────────────────────┘ │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│              Set OAuth Cookie                                │
│  ┌────────────────────────────────────────────────────────┐ │
│  │ http.Cookie{                                            │ │
│  │   Name:     "oauth"                                     │ │
│  │   Value:    JSON({                                      │ │
│  │               access_token: "eyJ...",                   │ │
│  │               expires_at: "2025-12-17T..."              │ │
│  │             })                                          │ │
│  │   Path:     "/"                                         │ │
│  │   HttpOnly: true                                        │ │
│  │   Secure:   true                                        │ │
│  │   SameSite: Lax                                         │ │
│  │   Expires:  24h                                         │ │
│  │ }                                                        │ │
│  └────────────────────────────────────────────────────────┘ │
└────────────────────────┬─────────────────────────────────────┘
                         │
                         ▼
┌──────────────────────────────────────────────────────────────┐
│              Redirect to Application                         │
│  HTTP 302 Found                                              │
│  Location: https://app.lunogram.com/dashboard                │
│  Set-Cookie: oauth={"access_token":"eyJ..."}; HttpOnly; ...  │
└──────────────────────────────────────────────────────────────┘
```

## Configuration Structure

```
┌─────────────────────────────────────────────────────────────┐
│                    config.yaml                              │
│                                                             │
│  auth:                                                      │
│    tokenLifetime: 24h                                       │
│    jwtSecret: ${JWT_SECRET}                                 │
│    drivers: [basic, email, google, saml]                    │
│                                                             │
│    basic:                                                   │
│      name: "Admin Login"                                    │
│      email: "admin@example.com"                             │
│      password: ${AUTH_BASIC_PASSWORD}                       │
│                                                             │
│    email:                                                   │
│      name: "Email Login"                                    │
│      from: "auth@lunogram.com"                              │
│      smtp:                                                  │
│        host: "smtp.sendgrid.net"                            │
│        port: 587                                            │
│        username: "apikey"                                   │
│        password: ${SMTP_PASSWORD}                           │
│                                                             │
│    google:                                                  │
│      name: "Continue with Google"                           │
│      clientId: "123.apps.googleusercontent.com"             │
│      clientSecret: ${GOOGLE_CLIENT_SECRET}                  │
│      redirectUri: "https://api.../auth/login/google/..."   │
│                                                             │
│    saml:                                                    │
│      name: "Company SSO"                                    │
│      callbackUrl: "https://api.../auth/login/saml/..."     │
│      entryPoint: "https://idp.example.com/sso"              │
│      issuer: "lunogram"                                     │
│      certificate: ${SAML_CERT}                              │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ Parsed by
                          ▼
┌─────────────────────────────────────────────────────────────┐
│              internal/config/config.go                      │
│                                                             │
│  type Config struct {                                       │
│    Auth AuthConfig                                          │
│  }                                                           │
│                                                             │
│  type AuthConfig struct {                                   │
│    TokenLifetime time.Duration                              │
│    JWTSecret     string                                     │
│    Drivers       []string                                   │
│    Basic         *BasicConfig                               │
│    Email         *EmailConfig                               │
│    Google        *GoogleConfig                              │
│    SAML          *SAMLConfig                                │
│  }                                                           │
└─────────────────────────────────────────────────────────────┘
                          │
                          │ Used by
                          ▼
┌─────────────────────────────────────────────────────────────┐
│              auth.Registry                                  │
│                                                             │
│  NewRegistry(config) {                                      │
│    registry := &Registry{}                                  │
│                                                             │
│    if config.Basic != nil {                                 │
│      registry.Register(NewBasicProvider(config.Basic))      │
│    }                                                         │
│    if config.Email != nil {                                 │
│      registry.Register(NewEmailProvider(config.Email))      │
│    }                                                         │
│    // ... register other enabled providers                  │
│                                                             │
│    return registry                                          │
│  }                                                           │
└─────────────────────────────────────────────────────────────┘
```

## Migration Phases

```
Week 1: Foundation
├── Provider Interface ✓
├── Registry System ✓
├── Token Utilities ✓
├── OpenAPI Spec ✓
└── Handler Scaffold ✓

Week 2: Simple Providers
├── BasicProvider ✓
│   ├── Implementation
│   ├── Tests
│   └── Integration
└── EmailProvider ✓
    ├── SMTP Integration
    ├── Magic Links
    ├── Tests
    └── Integration

Week 3: OAuth/OIDC
├── OpenIDProvider ✓
│   ├── Discovery
│   ├── Token Exchange
│   ├── State Management
│   └── Tests
└── GoogleProvider ✓
    ├── Wrapper Implementation
    └── Tests

Week 4: SAML
└── SAMLProvider ✓
    ├── AuthnRequest
    ├── Assertion Parsing
    ├── Bindings (Redirect/POST)
    ├── Tests
    └── Integration with Test IdP

Week 5: Cloud
└── ClerkProvider ✓
    ├── JWT Validation
    ├── Webhook Handler
    ├── User Lifecycle
    ├── Signature Verification
    └── Tests

Week 6: Cutover
├── Feature Flag
├── Monitoring
├── Load Testing
├── Security Review
├── Staged Rollout
│   ├── 10% Traffic
│   ├── 50% Traffic
│   └── 100% Traffic
└── Cleanup
```

---

**Note**: These diagrams are conceptual representations. Actual implementation may vary based on specific requirements and constraints discovered during development.
