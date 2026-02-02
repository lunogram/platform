# GitHub Copilot Instructions

## Project Overview

This is the Lunogram platform - a multi-service monorepo for customer engagement and messaging.

### Architecture

- **cmd/lunogram** - Single binary entry point
- **internal** - Go backend packages (API, embedded console, workers)
- **internal/http/console** - Embedded React frontend (built from `console/`)
- **console** - React TypeScript frontend source (Vite, React Router 7)

### Build

The console frontend is built and embedded into the Go binary:

```bash
make build        # Builds WASM modules + console, copies dist to internal/http/console/dist
make console      # Builds only the console
```

The embedded console is served at the root path (`/`) by the management HTTP server.

## Code Style & Conventions

### Go (internal)

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
7. **Required vs nullable**: Use the `required` array to specify required fields instead of `nullable: true` for optional fields. Only use `nullable: true` when a field can explicitly be set to `null`

```yaml
# ✅ Good - use required array
properties:
  name:
    type: string
  description:
    type: string
required:
  - name

# ❌ Bad - don't use nullable for optional fields
properties:
  name:
    type: string
  description:
    type: string
    nullable: true
```

#### Testing

- Write tests in `_test.go` files alongside implementation
- Use `testify` for assertions
- Use unexported field names for test types (lowercase)
- Use `testcontainers` for database integration tests
- Follow table-driven test patterns
- Include both success and error cases
- Test status codes and response structure
- Avoid obvious comments - code should be self-explanatory
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

### TypeScript (console)

- Use TypeScript strictly - no `any` types
- Follow React hooks best practices
- Use React Router 7 loaders for data fetching
- Keep API client (`api.ts`) in sync with backend OpenAPI spec
- Update types when backend changes

### Database

- Always use migrations (internal/store/migrations)
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

1. Edit `oapi/resources.yml`
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

### Verifying Compilation

When verifying that code compiles after making changes, use the linter which checks compilation without creating build artifacts:

```bash
# ✅ Good - runs linters and verifies compilation
make lint

# ❌ Bad - leaves binary in directory
go build
```

The linter will catch compilation errors and style issues without creating binary files that could be accidentally committed to git.

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
- ❌ Don't add obvious comments explaining what code does - write self-documenting code instead

## References

- OpenAPI Spec: `oapi/resources.yml`
- Database Schema: `internal/store/migrations/`
- API Client: `console/src/api.ts`
- Example Tests: `internal/http/controllers/v1/*_test.go`
