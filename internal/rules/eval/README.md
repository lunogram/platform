# Rules Evaluator

The `eval` package provides in-memory evaluation of rules without frequency constraints, complementing the `query` package which generates SQL queries for database-backed rule evaluation.

## Overview

The evaluator allows you to check if user and event data matches rule conditions directly in memory, similar to how the query package builds SQL queries for database evaluation. This is useful for:

- Real-time evaluation without database queries
- Testing rule logic
- Validating data before persistence
- Client-side rule evaluation

## Limitations

**Important**: The evaluator cannot evaluate rules with frequency constraints. Frequency-based rules require aggregating historical event data over time periods, which must be done using database queries via the `query` package.

## Usage

### Basic Example

```go
import (
    "github.com/lunogram/platform/services/nexus/internal/rules"
    "github.com/lunogram/platform/services/nexus/internal/rules/evaluator"
)

func run() {
    eval := evaluator.NewEvaluator()
    
    ruleSet := rules.RuleSet{
        Rule: rules.Rule{
            Type:     rules.RuleTypeString,
            Group:    rules.RuleGroupUser,
            Path:     ".email",
            Operator: rules.OperatorEndsWith,
            Value:    "@example.com",
        },
    }
    
    data := evaluator.Data{
        User: map[string]any{
            "email": "user@example.com",
            "age":   25,
        },
    }
    
    matches, err := eval.Evaluate(ruleSet, data)
    if err != nil {
        // Handle error
    }
    
    if matches {
        // User matches the rule
    }
}
```

### Complex Rules with Logical Operators

```go
// AND/OR logic with nested rules
ruleSet := rules.RuleSet{
    Rule: rules.Rule{
        Type:     rules.RuleTypeWrapper,
        Group:    rules.RuleGroupParent,
        Operator: rules.OperatorAnd,
        Children: []rules.Rule{
            {
                Type:     rules.RuleTypeNumber,
                Group:    rules.RuleGroupUser,
                Path:     ".age",
                Operator: rules.OperatorGreaterThan,
                Value:    18,
            },
            {
                Type:     rules.RuleTypeWrapper,
                Group:    rules.RuleGroupParent,
                Operator: rules.OperatorOr,
                Children: []rules.Rule{
                    {
                        Type:     rules.RuleTypeString,
                        Group:    rules.RuleGroupUser,
                        Path:     ".country",
                        Operator: rules.OperatorEquals,
                        Value:    "US",
                    },
                    {
                        Type:     rules.RuleTypeString,
                        Group:    rules.RuleGroupUser,
                        Path:     ".country",
                        Operator: rules.OperatorEquals,
                        Value:    "CA",
                    },
                },
            },
        },
    },
}

data := evaluator.Data{
    User: map[string]any{
        "age":     25,
        "country": "CA",
    },
}

matches, _ := eval.Evaluate(ruleSet, data)
// matches = true (age > 18 AND country = "CA")
```

### Nested Data Paths

The evaluator supports both dot notation and bracket notation for accessing nested data:

```go
ruleSet := rules.RuleSet{
    Rule: rules.Rule{
        Type:     rules.RuleTypeString,
        Group:    rules.RuleGroupUser,
        Path:     ".data.subscription.tier",
        Operator: rules.OperatorEquals,
        Value:    "premium",
    },
}

data := evaluator.Data{
    User: map[string]any{
        "data": map[string]any{
            "subscription": map[string]any{
                "tier": "premium",
            },
        },
    },
}

matches, _ := eval.Evaluate(ruleSet, data)
// matches = true
```

Bracket notation for keys with spaces:

```go
ruleSet := rules.RuleSet{
    Rule: rules.Rule{
        Type:     rules.RuleTypeNumber,
        Group:    rules.RuleGroupUser,
        Path:     ".data['purchase agreement'].value",
        Operator: rules.OperatorGreaterThan,
        Value:    1000,
    },
}

data := evaluator.Data{
    User: map[string]any{
        "data": map[string]any{
            "purchase agreement": map[string]any{
                "value": 1500,
            },
        },
    },
}

matches, _ := eval.Evaluate(ruleSet, data)
// matches = true
```

### Event Rules

Evaluate event-based rules (without frequency constraints):

```go
ruleSet := rules.RuleSet{
    Rule: rules.Rule{
        Type:  rules.RuleTypeWrapper,
        Group: rules.RuleGroupEvent,
        Value: "order.created",
        Children: []rules.Rule{
            {
                Type:     rules.RuleTypeNumber,
                Group:    rules.RuleGroupEvent,
                Path:     ".data.amount",
                Operator: rules.OperatorGreaterThan,
                Value:    100,
            },
        },
    },
}

data := evaluator.Data{
    Event: map[string]any{
        "name": "order.created",
        "data": map[string]any{
            "amount": 150,
        },
    },
}

matches, _ := eval.Evaluate(ruleSet, data)
// matches = true
```

### Mixed User and Event Rules

```go
ruleSet := rules.RuleSet{
    Rule: rules.Rule{
        Type:     rules.RuleTypeWrapper,
        Group:    rules.RuleGroupParent,
        Operator: rules.OperatorAnd,
        Children: []rules.Rule{
            {
                Type:     rules.RuleTypeString,
                Group:    rules.RuleGroupUser,
                Path:     ".email",
                Operator: rules.OperatorEndsWith,
                Value:    "@example.com",
            },
            {
                Type:  rules.RuleTypeWrapper,
                Group: rules.RuleGroupEvent,
                Value: "login",
                Children: []rules.Rule{
                    {
                        Type:     rules.RuleTypeString,
                        Group:    rules.RuleGroupEvent,
                        Path:     ".data.device",
                        Operator: rules.OperatorEquals,
                        Value:    "mobile",
                    },
                },
            },
        },
    },
}

data := evaluator.Data{
    User: map[string]any{
        "email": "user@example.com",
    },
    Event: map[string]any{
        "name": "login",
        "data": map[string]any{
            "device": "mobile",
        },
    },
}

matches, _ := eval.Evaluate(ruleSet, data)
// matches = true
```

## Supported Operators

### Comparison Operators
- `=` (equals)
- `!=` (not equals)
- `<` (less than)
- `<=` (less than or equal)
- `>` (greater than)
- `>=` (greater than or equal)

### Logical Operators
- `and` - All child rules must match
- `or` - At least one child rule must match

### String Operators
- `contains` - Case-insensitive substring match
- `not contain` - Case-insensitive substring non-match
- `starts with` - Case-insensitive prefix match
- `not start with` - Case-insensitive prefix non-match
- `ends with` - Case-insensitive suffix match

### Existence Operators
- `is set` - Value exists (not null/undefined)
- `is not set` - Value does not exist (null/undefined)
- `empty` - Value is empty string or empty array

### Array Operators
- `any` - Value exists in array
- `none` - Value does not exist in array

### Date Operators
- All comparison operators (`<`, `<=`, `>`, `>=`, `=`, `!=`)
- `is same day` - Same calendar day (ignores time)

