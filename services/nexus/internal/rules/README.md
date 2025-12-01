# Rules Engine

A Go implementation of a flexible rule evaluation system for user segmentation and targeting. The engine supports **both in-memory evaluation** and **PostgreSQL query generation** from rule definitions.

## Overview

The rules engine allows you to:
1. **Evaluate rules in-memory** against user data, events, and journey information
2. **Generate PostgreSQL queries** to filter users directly in the database

This dual-mode approach enables efficient filtering whether you're working with small datasets in memory or need to leverage database indexes for large-scale queries.

## Features

- **Dual Execution Modes**: In-memory evaluation AND SQL query generation
- **Multiple Rule Types**: Number, String, Boolean, Date, Array, and Wrapper rules
- **Flexible Operators**: Comparison, contains, starts/ends with, empty, set/not set
- **Event Rules**: Match users based on event history and frequency (rolling/fixed time periods)
- **Rule Composition**: Combine rules with AND/OR logic, including nested conditions
- **SQL Query Generation**: Generate optimized PostgreSQL queries from rule trees
- **JSONPath Support**: Access nested user data with JSONPath expressions
- **Reserved Path Optimization**: Direct column access for common fields (email, external_id, etc.)

## Usage

### In-Memory Evaluation

```go
import "github.com/lunogram/platform/services/nexus/internal/rules"

// Create a simple number rule
rule := rules.Make(rules.MakeParams{
    Type:     rules.RuleTypeNumber,
    Path:     "$.age",
    Operator: rules.OpGreaterThan,
    Value:    18,
})

// Evaluate against user data
input := rules.RuleCheckInput{
    User:    map[string]interface{}{"age": 25},
    Events:  []rules.TemplateEvent{},
    Journey: map[string]interface{}{},
}

result := rules.Check(input, rule) // returns true
```

### SQL Query Generation

The same rule definition can generate optimized PostgreSQL queries:

```go
projectID := uuid.New()
query := rules.GetRuleQuery(projectID, rule)
// Output: SELECT id FROM users WHERE (data->>'age')::int > 18 AND project_id = '...'
```

#### Query Examples

**Simple condition:**
```go
rule := rules.Make(rules.MakeParams{
    Type:     rules.RuleTypeString,
    Path:     "$.email",
    Operator: rules.OpContains,
    Value:    "@gmail.com",
})
query := rules.GetRuleQuery(projectID, rule)
// SELECT id FROM users WHERE email LIKE '%@gmail.com%' AND project_id = '...'
```

**Composite AND rule:**
```go
rule := rules.Make(rules.MakeParams{
    Type:     rules.RuleTypeWrapper,
    Operator: rules.OpAnd,
    Children: []*rules.RuleTree{
        rules.Make(rules.MakeParams{
            Type:     rules.RuleTypeNumber,
            Path:     "$.age",
            Operator: rules.OpGreaterThan,
            Value:    21,
        }),
        rules.Make(rules.MakeParams{
            Type:     rules.RuleTypeString,
            Path:     "$.country",
            Operator: rules.OpEqual,
            Value:    "US",
        }),
    },
})
query := rules.GetRuleQuery(projectID, rule)
// SELECT id FROM users WHERE (data->>'age')::int > 21 and (data->>'country')::text = 'US' AND project_id = '...'
```

**Event-based query:**
```go
rule := &rules.RuleTree{
    Rule: rules.Rule{
        UUID:     uuid.New().String(),
        Type:     rules.RuleTypeWrapper,
        Group:    rules.RuleGroupEvent,
        Path:     "$.name",
        Operator: rules.OpAnd,
        Value:    mustMarshal("purchase"),
    },
    Frequency: &rules.EventRuleFrequency{
        Operator: rules.OpGreaterThanEq,
        Count:    3,
        Period: rules.EventRulePeriod{
            Type:  rules.PeriodTypeRolling,
            Unit:  &rules.TimeUnitDay,
            Value: intPtr(30),
        },
    },
}
query := rules.GetRuleQuery(projectID, rule)
// SELECT user_id AS id FROM user_events 
// WHERE project_id = '...' AND name = 'purchase' AND created_at >= now() - INTERVAL '30 day'
// GROUP BY project_id, user_id
// HAVING count(*) >= 3
```

**Nested conditions:**
```go
rule := rules.Make(rules.MakeParams{
    Type:     rules.RuleTypeWrapper,
    Operator: rules.OpAnd,
    Children: []*rules.RuleTree{
        rules.Make(rules.MakeParams{
            Type:     rules.RuleTypeNumber,
## SQL Query Builder

The query builder generates PostgreSQL-compatible SQL from rule trees. Key features:

### JSONB Data Access

For custom user fields stored in the `data` JSONB column:
- Simple fields: `(data->>'field_name')::type`
- Nested fields: `(data->'parent'->'child'->>'field')::type`

### Reserved Path Optimization

Common fields bypass JSONB accessors for better performance:
- User paths: `external_id`, `email`, `phone`, `timezone`, `locale`, `created_at`, `has_push_device`
- Event paths: `name`, `created_at`

Example:
```go
// Reserved path - direct column access
rule := rules.Make(rules.MakeParams{
    Type:     rules.RuleTypeString,
    Path:     "$.email",
    Operator: rules.OpContains,
    Value:    "@example.com",
})
// Query: email LIKE '%@example.com%'

// Custom field - JSONB accessor
rule := rules.Make(rules.MakeParams{
    Type:     rules.RuleTypeString,
    Path:     "$.company",
    Operator: rules.OpContains,
    Value:    "Tech",
})
// Query: (data->>'company')::text LIKE '%Tech%'
```

### Operator SQL Mapping

| Operator | SQL Output | Example |
|----------|-----------|---------|
| `=` | `column = value` | `age = 25` |
| `!=` | `column != value` | `status != 'inactive'` |
| `>`, `<`, `>=`, `<=` | `column op value` | `score > 100` |
| `contains` | `column LIKE '%value%'` | `email LIKE '%@gmail%'` |
| `starts with` | `column LIKE 'value%'` | `name LIKE 'John%'` |
| `ends with` | `column LIKE '%value'` | `email LIKE '%@company.com'` |
| `not contain` | `column NOT LIKE '%value%'` | `email NOT LIKE '%spam%'` |
| `is set` | `column IS NOT NULL` | `verified_at IS NOT NULL` |
| `is not set` | `column IS NULL` | `deleted_at IS NULL` |
| `empty` | `column = ''` or `column = '[]'::jsonb` | `description = ''` |

### Event Queries

Event rules generate queries against the `user_events` table:

**Basic event:**
```sql
SELECT user_id AS id 
FROM user_events 
WHERE project_id = '...' AND name = 'purchase'
GROUP BY project_id, user_id
HAVING count(*) >= 1
```

**With rolling time period:**
```sql
SELECT user_id AS id 
FROM user_events 
WHERE project_id = '...' AND name = 'page_view' AND created_at >= now() - INTERVAL '7 day'
GROUP BY project_id, user_id
HAVING count(*) >= 5
```

**With fixed time period:**
```sql
SELECT user_id AS id 
FROM user_events 
WHERE project_id = '...' AND name = 'signup' 
  AND (created_at >= '2024-01-01T00:00:00Z' AND created_at <= '2024-12-31T23:59:59Z')
GROUP BY project_id, user_id
HAVING count(*) = 1
```

### Mixed User & Event Queries

When combining user properties with event rules, queries use `INTERSECT`:

```sql
SELECT id FROM users 
WHERE (data->>'age')::int >= 21 and (data->>'tier')::text = 'premium' AND project_id = '...' 
INTERSECT 
SELECT user_id AS id FROM user_events 
WHERE project_id = '...' AND name = 'purchase'
GROUP BY project_id, user_id
HAVING count(*) >= 1
```
            Children: []*rules.RuleTree{
                rules.Make(rules.MakeParams{
                    Type:     rules.RuleTypeString,
                    Path:     "$.country",
                    Operator: rules.OpEqual,
                    Value:    "US",
                }),
                rules.Make(rules.MakeParams{
                    Type:     rules.RuleTypeString,
                    Path:     "$.country",
                    Operator: rules.OpEqual,
                    Value:    "CA",
                }),
            },
        }),
    },
})
query := rules.GetRuleQuery(projectID, rule)
// SELECT id FROM users WHERE (data->>'age')::int >= 21 and ((data->>'country')::text = 'US' or (data->>'country')::text = 'CA') AND project_id = '...'
```

## Rule Types

### Number Rules
- **Operators**: `=`, `!=`, `<`, `<=`, `>`, `>=`, `is set`, `is not set`
- **Path**: JSONPath to number field (e.g., `$.age`, `$.score`)

### String Rules  
- **Operators**: `=`, `!=`, `contains`, `not contain`, `starts with`, `not start with`, `ends with`, `empty`, `is set`, `is not set`
- **Path**: JSONPath to string field (e.g., `$.name`, `$.email`)

### Boolean Rules
- **Operators**: `=`, `!=`
- **Path**: JSONPath to boolean field (e.g., `$.active`, `$.verified`)

### Date Rules
- **Operators**: `=`, `!=`, `<`, `<=`, `>`, `>=`, `is same day`, `is set`, `is not set`
- **Path**: JSONPath to date field (e.g., `$.created_at`)
- **Value**: RFC3339 formatted string or Unix timestamp

### Array Rules
- **Operators**: `is set`, `is not set`, `empty`, `contains`
- **Path**: JSONPath to array field (e.g., `$.tags`, `$.subscriptions`)

### Wrapper Rules
- **Operators**: `and`, `or`
- **Children**: Array of child rules to evaluate
- **Groups**: `user` (user properties), `event` (event-based), `parent` (top-level wrapper)

## Reserved Paths

Certain paths map directly to database columns and don't require JSONPath processing:

**User Paths**: `external_id`, `email`, `phone`, `timezone`, `locale`, `created_at`, `has_push_device`

**Event Paths**: `name`, `created_at`

## Architecture

```
rules/
├── types.go        # Core types and constants
├── engine.go       # Main evaluation engine and registry
├── helpers.go      # JSONPath, query building, type conversion
├── number.go       # Number rule implementation
├── string.go       # String rule implementation
├── boolean.go      # Boolean rule implementation
├── date.go         # Date rule implementation
├── array.go        # Array rule implementation
├── wrapper.go      # Wrapper rule and event rules
└── engine_test.go  # Test suite
```

## Testing

```bash
go test ./internal/rules/... -v
```

## Migration from TypeScript

This is a direct port of the TypeScript RuleEngine from `services/platform/src/rules/`. Key differences:

- Generics used for type-safe value extraction
- Registry pattern with interface-based rule checkers
- Explicit error handling vs. exceptions
- JSONPath library: `oliveagle/jsonpath`

## Future Enhancements

- Custom operators
- Rule validation
- Rule serialization/deserialization
- Performance optimizations for large rule sets
- More comprehensive event rule testing
