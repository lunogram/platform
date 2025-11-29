# GitHub Copilot Instructions

## Project Overview

This is the Lunogram platform - a multi-service monorepo for customer engagement and messaging.

### Architecture

- **services/nexus** - Go backend API (OpenAPI 3.0, PostgreSQL)
- **services/console** - React TypeScript frontend (Vite, React Router 7)
- **services/platform** - Legacy Node.js backend (being migrated to Nexus)

## Code Style & Conventions

### Go (services/nexus)

#### General Principles

- Use `golangci-lint` for linting - follow all configured rules
- Format code with `gofmt`
- Generate code with `make generate` after OpenAPI changes
- Use `sqlx` for database operations with struct tags

#### API Development

1. **OpenAPI First**: Always define endpoints in `oapi/resources.yml` before implementation
2. **Use oapi-codegen types**: Never create custom types when generated types exist
3. **Simple schemas**: Prefer simple object types with `additionalProperties` over complex `oneOf` discriminated unions
4. **Flexible data fields**: Use `json.RawMessage` with `x-go-type` for type-specific data

```yaml
data:
  type: object
  additionalProperties: true
  x-go-type: json.RawMessage
```

5. **Derive relationships**: Get type/state from parent resources rather than duplicating in request bodies
6. **Clean up unused schemas**: Remove schemas that aren't referenced

#### Testing

- **Always use named test types** instead of anonymous structs in test tables:

```go
// ✅ Good
type test struct {
    input string
    expected int
    code int
}

tests := map[string]test{
    "success": {
        input: "hello",
        expected: 5,
        code: 200,
    },
}

// ❌ Bad
tests := map[string]struct{
    input string
    expected int
}{
    "success": {
        input: "hello",
        expected: 5,
    },
}
```

- Use unexported field names for test types (lowercase)
- Use `testcontainers` for database integration tests
- Follow table-driven test patterns
- Include both success and error cases
- Test status codes and response structure

#### Store Layer

- Separate concerns: controllers handle HTTP, stores handle database
- Use transactions for multi-step operations
- Implement soft deletes with `deleted_at` timestamp
- Always filter by `project_id` for multi-tenant isolation
- Use `COUNT(*) OVER ()` for pagination total counts

```go
SELECT id, name, COUNT(*) OVER () AS total_count
FROM campaigns
WHERE project_id = $1 AND deleted_at IS NULL
LIMIT $2 OFFSET $3
```

#### Controllers

- Log all operations with structured logging (zap)
- Check resource existence before operations
- Return appropriate HTTP status codes:
  - 200 OK - successful GET/PATCH
  - 201 Created - successful POST
  - 204 No Content - successful DELETE
  - 404 Not Found - resource doesn't exist
  - 500 Internal Server Error - unexpected errors
- Use `problem` package for consistent error responses
- Validate input using generated OAPI types

### TypeScript (services/console)

- Use TypeScript strictly - no `any` types
- Follow React hooks best practices
- Use React Router 7 loaders for data fetching
- Keep API client (`api.ts`) in sync with backend OpenAPI spec
- Update types when backend changes

### Database

- Always use migrations (services/nexus/internal/store/migrations)
- Use UUIDs for primary keys
- Include `created_at`, `updated_at` timestamps
- Use `deleted_at` for soft deletes
- Create proper indexes for foreign keys and frequently queried columns

## Workflow

### Adding New Endpoints

1. Define in `oapi/resources.yml` (OpenAPI spec)
2. Run `make generate` to generate types
3. Implement store methods if needed
4. Implement controller handler
5. Write tests with named test types
6. Update frontend API client and types
7. Verify all tests pass

### Making OpenAPI Changes

1. Edit `services/nexus/oapi/resources.yml`
2. Run `make generate`
3. Update controller implementations
4. Update tests
5. Check for errors with `go build`

### Testing

```bash
# Run all tests
make test

# Run specific package tests
go test ./internal/http/controllers/v1/... -v

# Run specific test
go test ./internal/http/controllers/v1/... -v -run TestCampaignCreation
```

## Common Patterns

### Resource Nesting

Templates are nested under campaigns:
- ✅ `POST /projects/{projectID}/campaigns/{campaignID}/templates`
- ❌ `POST /projects/{projectID}/templates` (with campaign_id in body)

### Type Derivation

Derive types from parent resources instead of request bodies:

```go
// Template type is derived from campaign.Channel
campaign, _ := store.GetCampaign(ctx, projectID, campaignID)
templateType := campaign.Channel // "email", "sms", "push"
```

### Pagination

Always include pagination parameters:

```yaml
parameters:
  - $ref: '#/components/parameters/Limit'
  - $ref: '#/components/parameters/Offset'
```

Return pagination metadata:

```json
{
  "data": [...],
  "total": 42,
  "limit": 20,
  "offset": 0
}
```

## Don't Do

- ❌ Don't use anonymous types in test tables
- ❌ Don't create endpoints that bypass resource hierarchy
- ❌ Don't skip migrations for schema changes
- ❌ Don't use discriminated unions if simple objects work
- ❌ Don't duplicate parent resource data in child creation
- ❌ Don't forget to run `make generate` after OpenAPI changes
- ❌ Don't hardcode UUIDs in tests - use generated ones
- ❌ Don't expose internal errors to API responses

## References

- OpenAPI Spec: `services/nexus/oapi/resources.yml`
- Database Schema: `services/nexus/internal/store/migrations/`
- API Client: `services/console/src/api.ts`
- Example Tests: `services/nexus/internal/http/controllers/v1/*_test.go`
