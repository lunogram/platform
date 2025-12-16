# Subscription Endpoints Migration

This document describes the migration of subscription preference management endpoints from the Node.js platform service to the Go nexus service.

## Overview

The subscription preference endpoints allow users to:
1. Unsubscribe from email campaigns via unsubscribe links
2. View and manage their subscription preferences

These endpoints have been migrated from TypeScript/Node.js to Go with server-side rendered HTML templates.

## Endpoints

### Email Unsubscribe
- **URL**: `GET /unsubscribe/email`
- **Query Parameters**:
  - `user_id` (UUID, required) - The user's ID
  - `campaign_id` (UUID, required) - The campaign ID containing the subscription
- **Response**: HTML page confirming unsubscription
- **Location**: Nexus public API

### Subscription Preferences
- **URL**: `GET /preferences/{userID}`
- **Path Parameters**:
  - `userID` (UUID, required) - The user's ID
- **Query Parameters**:
  - `project_id` (UUID, required) - The project ID
  - `u` (optional) - Set to "1" to show success message
- **Response**: HTML page with subscription preferences form
- **Location**: Nexus public API

### Update Preferences
- **URL**: `POST /preferences/{userID}`
- **Path Parameters**:
  - `userID` (UUID, required) - The user's ID
- **Query Parameters**:
  - `project_id` (UUID, required) - The project ID
- **Form Data**:
  - `subscription_ids[]` - Array of subscription IDs to remain subscribed to
- **Response**: 303 redirect to preferences page with success message
- **Location**: Nexus public API

## Implementation Details

### Technology Stack
- **Backend**: Go 1.25+
- **Templates**: Go html/template
- **Styling**: Tailwind CSS v4 (compiled via go:generate)
- **Frontend Enhancement**: HTMX 1.9.10
- **Database**: PostgreSQL (existing schema)

### Template Features
- **Responsive Design**: Mobile-first with Tailwind CSS
- **Multi-language Support**: English, Spanish, French, German, Portuguese, Italian
- **Accessibility**: Semantic HTML with proper ARIA labels
- **Progressive Enhancement**: Works without JavaScript, enhanced with HTMX

### Architecture

```
services/nexus/internal/http/controllers/v1/public/
├── subscriptions.go              # Controller handlers
├── subscriptions_test.go         # Tests
└── templates/
    ├── unsubscribe.html         # Unsubscribe confirmation page
    ├── preferences.html         # Preferences management page
    ├── templates.go             # Template rendering functions
    ├── generate.go              # go:generate directives
    ├── input.css                # Tailwind input
    ├── tailwind.config.js       # Tailwind configuration
    ├── static/styles.css        # Generated CSS (not in git)
    └── README.md                # Template documentation
```

## Database

Uses existing tables:
- `subscriptions` - Subscription definitions
- `user_subscription` - User subscription states
- `campaigns` - Campaign with subscription_id reference
- `users` - User information including locale

## Building

### CSS Generation
```bash
cd services/nexus
go generate ./internal/http/controllers/v1/public/templates
```

Or manually:
```bash
cd services/nexus/internal/http/controllers/v1/public/templates
tailwindcss -i ./input.css -o ./static/styles.css --minify
```

### Running Tests
```bash
cd services/nexus
go test ./internal/http/controllers/v1/public -run "TestUnsubscribe|TestGetPreferences|TestUpdatePreferences"
```

### Building Binary
```bash
cd services/nexus
go build
```

The templates and CSS are embedded in the binary using `//go:embed` directives.

## Differences from Platform Service

### Node.js Platform (Old)
- Routes: `/unsubscribe/email`, `/preferences/:userId`
- Templates: Handlebars with inline CSS
- Localization: Inline strings object
- Form handling: Koa body parser

### Go Nexus (New)
- Routes: Same URLs, now in public API
- Templates: html/template with embedded Tailwind CSS
- Localization: GetStrings() function with locale parameter
- Form handling: Standard Go form parsing
- Added: HTMX support for progressive enhancement
- Added: Comprehensive test coverage

## Migration Strategy

### Phase 1: Parallel Running (Current)
- Both platform and nexus serve these endpoints
- Platform proxies unknown routes to nexus
- Allows gradual rollout

### Phase 2: Full Migration
- Update all unsubscribe links to point to nexus
- Remove subscription controller from platform service
- Deprecate platform subscription routes

### Phase 3: Cleanup
- Remove platform service entirely (when all endpoints migrated)

## Testing

The implementation includes comprehensive tests:
- `TestUnsubscribeEmail` - Tests unsubscribe functionality
- `TestGetPreferences` - Tests preferences page rendering
- `TestUpdatePreferences` - Tests form submission and state updates

All tests use testcontainers for real PostgreSQL integration testing.

## Security Considerations

1. **UUID Validation**: All user and campaign IDs validated as UUIDs
2. **Resource Verification**: Campaigns and users verified to exist before operations
3. **Multi-tenant Isolation**: All operations filtered by project_id
4. **No Authentication Required**: Public endpoints by design (for email links)
5. **CSRF Protection**: Forms use POST with same-origin policy

## Future Enhancements

1. **Signature Validation**: Add HMAC signatures to unsubscribe links
2. **Rate Limiting**: Add rate limiting to prevent abuse
3. **Analytics**: Track unsubscribe rates and preference changes
4. **HTMX Enhancement**: Add dynamic updates without page reload
5. **Email Preview**: Show which campaigns user is subscribed to
