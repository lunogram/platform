# Auth Migration Quick Reference

## Provider Interface Cheat Sheet

```go
// Every provider must implement
type Provider interface {
    Name() string
    Start(ctx, StartRequest) (*StartResponse, error)
    Validate(ctx, ValidateRequest) (*ValidateResponse, error)
}

// Optional for providers with webhooks
type WebhookProvider interface {
    Provider
    HandleWebhook(ctx, WebhookRequest) error
}
```

## Request/Response Types

```go
// Starting authentication
type StartRequest struct {
    RedirectURL  string
    Organization *uuid.UUID
    Query        url.Values
    Body         map[string]any
}

type StartResponse struct {
    RedirectURL string
    SetCookies  []http.Cookie
}

// Validating authentication
type ValidateRequest struct {
    Query   url.Values
    Body    map[string]any
    Cookies []*http.Cookie
    Headers http.Header
    RawBody []byte
}

type ValidateResponse struct {
    Admin       AdminInfo
    RedirectURL string
    SetCookies  []http.Cookie
}

// Admin information from provider
type AdminInfo struct {
    Email      string
    ExternalID *string
    FirstName  *string
    LastName   *string
    ImageURL   *string
}
```

## Handler Flow

```go
// 1. User hits /auth/login/google
StartAuth() {
    provider := registry.Get("google")
    response := provider.Start(ctx, request)
    // Set cookies, redirect user
}

// 2. Provider redirects to /auth/login/google/callback
ValidateAuth() {
    provider := registry.Get("google")
    response := provider.Validate(ctx, request)
    
    // Get or create admin
    admin := getOrCreateAdmin(response.Admin)
    
    // Generate JWT token
    token := generateAccessToken(admin)
    
    // Set OAuth cookie, redirect to app
}
```

## Provider Comparison

| Provider | Start Action | Validate Action | Webhook | Dependencies |
|----------|--------------|-----------------|---------|--------------|
| **Basic** | Redirect to form | Check credentials | No | None |
| **Email** | Send magic link | Verify JWT | No | gomail, SMTP |
| **SAML** | Redirect to IdP | Parse assertion | No | crewjam/saml |
| **OpenID** | Redirect to provider | Exchange code | No | go-oidc, oauth2 |
| **Google** | Use OpenID | Use OpenID | No | go-oidc, oauth2 |
| **Clerk** | N/A (JS SDK) | Verify JWT | Yes | clerk-sdk-go, svix |

## Common Patterns

### Cookie Management

```go
// Set secure cookie
cookie := http.Cookie{
    Name:     "oauth",
    Value:    tokenJSON,
    Path:     "/",
    HttpOnly: true,
    Secure:   true,
    SameSite: http.SameSiteLaxMode,
    Expires:  expiresAt,
}
http.SetCookie(w, &cookie)

// Clear cookie
cookie := http.Cookie{
    Name:     "oauth",
    Value:    "",
    Path:     "/",
    MaxAge:   -1,
}
http.SetCookie(w, &cookie)
```

### JWT Token Generation

```go
// Create token
token := jwt.NewWithClaims(jwt.SigningMethodHS256, jwt.RegisteredClaims{
    Subject:   admin.ID.String(),
    Issuer:    config.BaseURL,
    ExpiresAt: jwt.NewNumericDate(expiresAt),
})

// Sign token
tokenString, err := token.SignedString([]byte(config.JWTSecret))

// Store in database
accessToken := store.AccessToken{
    AdminID:   admin.ID,
    Token:     tokenString,
    ExpiresAt: expiresAt,
    IP:        getIP(r),
    UserAgent: r.UserAgent(),
}
stores.CreateAccessToken(ctx, accessToken)
```

### Admin Lookup or Create

```go
// Try to find existing admin
admin, err := stores.GetAdminByEmail(ctx, email, orgID)
if errors.Is(err, sql.ErrNoRows) {
    // Create new admin
    admin, err = stores.CreateAdmin(ctx, store.Admin{
        OrganizationID: orgID,
        Email:          email,
        FirstName:      &firstName,
        LastName:       &lastName,
        Role:           "member",
    })
}
```

### Redirect with OAuth Cookie

```go
// Generate token
token, err := generateAccessToken(ctx, stores, admin, r, config)

// Set cookie
setOAuthCookie(w, token, r.TLS != nil)

// Redirect to app
http.Redirect(w, r, redirectURL, http.StatusFound)
```

## Configuration Examples

### Basic Provider
```yaml
basic:
  name: "Admin Login"
  email: "admin@example.com"
  password: "${AUTH_BASIC_PASSWORD}"
```

### Email Provider
```yaml
email:
  name: "Email Login"
  from: "noreply@lunogram.com"
  smtp:
    host: "smtp.sendgrid.net"
    port: 587
    username: "apikey"
    password: "${SMTP_PASSWORD}"
```

### SAML Provider
```yaml
saml:
  name: "Company SSO"
  callbackUrl: "https://api.lunogram.com/auth/login/saml/callback"
  entryPoint: "https://idp.example.com/sso"
  issuer: "lunogram"
  certificate: "${SAML_CERT}"
```

### OpenID Provider
```yaml
openid:
  name: "OpenID Connect"
  issuerUrl: "https://auth.example.com"
  clientId: "lunogram-client"
  clientSecret: "${OIDC_CLIENT_SECRET}"
  redirectUri: "https://api.lunogram.com/auth/login/openid/callback"
```

### Google Provider
```yaml
google:
  name: "Continue with Google"
  clientId: "123456789.apps.googleusercontent.com"
  clientSecret: "${GOOGLE_CLIENT_SECRET}"
  redirectUri: "https://api.lunogram.com/auth/login/google/callback"
```

### Clerk Provider
```yaml
clerk:
  name: "Clerk Auth"
  secretKey: "${CLERK_SECRET_KEY}"
  webhookSecret: "${CLERK_WEBHOOK_SECRET}"
```

## Testing Snippets

### Unit Test Structure
```go
func TestBasicProvider_Validate(t *testing.T) {
    type test struct {
        name    string
        request ValidateRequest
        want    *ValidateResponse
        wantErr bool
    }
    
    tests := map[string]test{
        "valid credentials": {
            request: ValidateRequest{
                Body: map[string]any{
                    "email":    "admin@example.com",
                    "password": "secret",
                },
            },
            want: &ValidateResponse{
                Admin: AdminInfo{
                    Email:     "admin@example.com",
                    FirstName: ptr("Admin"),
                },
            },
        },
        "invalid credentials": {
            request: ValidateRequest{
                Body: map[string]any{
                    "email":    "admin@example.com",
                    "password": "wrong",
                },
            },
            wantErr: true,
        },
    }
    
    for name, tc := range tests {
        t.Run(name, func(t *testing.T) {
            provider := NewBasicProvider(config)
            got, err := provider.Validate(context.Background(), tc.request)
            
            if tc.wantErr {
                assert.Error(t, err)
                return
            }
            
            assert.NoError(t, err)
            assert.Equal(t, tc.want, got)
        })
    }
}
```

### Integration Test Structure
```go
func TestAuthFlow_Email(t *testing.T) {
    // Setup test database
    db := setupTestDB(t)
    stores := store.NewStores(db)
    
    // Create handler
    handler := NewHandlers(registry, stores, config, logger)
    
    // Start auth flow
    req := httptest.NewRequest("POST", "/auth/login/email", strings.NewReader(`{"email":"test@example.com"}`))
    w := httptest.NewRecorder()
    handler.StartAuth(w, req)
    
    // Verify email would be sent (check logs or mock)
    assert.Equal(t, http.StatusNoContent, w.Code)
    
    // Extract token from "sent" email (in tests, capture the token)
    token := extractTokenFromEmail(t)
    
    // Validate with token
    req = httptest.NewRequest("GET", "/auth/login/email/callback?token="+token, nil)
    w = httptest.NewRecorder()
    handler.ValidateAuth(w, req)
    
    // Verify redirect and cookie
    assert.Equal(t, http.StatusFound, w.Code)
    assert.Contains(t, w.Header().Get("Set-Cookie"), "oauth")
    
    // Verify admin created
    admin, err := stores.GetAdminByEmail(context.Background(), "test@example.com", orgID)
    assert.NoError(t, err)
    assert.NotNil(t, admin)
}
```

## Common Gotchas

### 1. Cookie Security
❌ Don't: `Secure: false` in production
✅ Do: `Secure: r.TLS != nil`

### 2. State Management
❌ Don't: Store state in memory
✅ Do: Use signed cookies for OAuth state/nonce

### 3. Redirect URLs
❌ Don't: Allow arbitrary redirects (open redirect vulnerability)
✅ Do: Validate redirect URLs against allowed domains

### 4. Error Messages
❌ Don't: "Invalid password for admin@example.com"
✅ Do: "Invalid credentials" (prevent user enumeration)

### 5. Token Expiration
❌ Don't: Forget to check token expiration
✅ Do: Verify `exp` claim in JWT

## Useful Commands

```bash
# Generate OpenAPI types
make generate

# Run tests
make test

# Run linter
make lint

# Run specific provider tests
go test ./internal/http/auth/providers -v -run TestEmail

# Check database migrations
psql -d lunogram -c "SELECT * FROM access_tokens LIMIT 5;"
```

## Debug Checklist

When auth isn't working:

- [ ] Check provider is registered in config
- [ ] Verify environment variables are set
- [ ] Check database connectivity
- [ ] Verify JWT secret matches
- [ ] Check cookie domain/path settings
- [ ] Verify redirect URL is allowed
- [ ] Check logs for detailed errors
- [ ] Test with curl to isolate frontend issues

## Resources

- Full Proposal: [auth-migration.md](./auth-migration.md)
- Summary: [auth-migration-summary.md](./auth-migration-summary.md)
- Platform Code: `services/platform/src/auth/`
- Nexus Code: `services/nexus/internal/http/auth/`
