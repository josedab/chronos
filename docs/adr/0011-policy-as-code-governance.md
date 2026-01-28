# ADR-0011: Policy-as-Code Governance Engine

## Status

Accepted

## Context

As Chronos usage scales across teams and organizations, governance becomes critical:

- **Security**: Jobs shouldn't call internal-only endpoints from untrusted namespaces
- **Compliance**: Certain job patterns violate regulatory requirements
- **Cost control**: Prevent jobs with excessive timeouts or retry counts
- **Standardization**: Enforce naming conventions and tagging requirements
- **Review workflow**: Some changes require approval before taking effect

Manual review doesn't scale. We need automated policy enforcement.

Options considered:

1. **Hardcoded rules**: Build constraints into the application code
   - Fast but inflexible
   - Requires redeployment to change rules
   
2. **External policy engine (OPA)**: Use Open Policy Agent for decisions
   - Powerful Rego language
   - Additional operational dependency
   
3. **Built-in declarative policies**: JSON/YAML policy definitions
   - No external dependencies
   - Domain-specific for job scheduling
   - Easy to understand and modify

## Decision

We implemented a **built-in policy-as-code engine** in `internal/policy/`:

```go
// internal/policy/engine.go
type Policy struct {
    ID          string     `json:"id"`
    Name        string     `json:"name"`
    Type        PolicyType `json:"type"`
    Severity    Severity   `json:"severity"`
    Action      Action     `json:"action"`
    Enabled     bool       `json:"enabled"`
    Rules       []Rule     `json:"rules"`
}

type PolicyType string

const (
    PolicyTypeJob       PolicyType = "job"       // Validate job configs
    PolicyTypeExecution PolicyType = "execution" // Validate at runtime
    PolicyTypeCluster   PolicyType = "cluster"   // Cluster-wide rules
)
```

### Condition Language

16 operators for flexible conditions:

```go
type Operator string

const (
    OpEquals         Operator = "equals"
    OpNotEquals      Operator = "not_equals"
    OpContains       Operator = "contains"
    OpNotContains    Operator = "not_contains"
    OpStartsWith     Operator = "starts_with"
    OpEndsWith       Operator = "ends_with"
    OpMatches        Operator = "matches"        // Regex
    OpIn             Operator = "in"
    OpNotIn          Operator = "not_in"
    OpGreaterThan    Operator = "greater_than"
    OpGreaterOrEqual Operator = "greater_or_equal"
    OpLessThan       Operator = "less_than"
    OpLessOrEqual    Operator = "less_or_equal"
    OpExists         Operator = "exists"
    OpNotExists      Operator = "not_exists"
    OpIsEmpty        Operator = "is_empty"
    OpIsNotEmpty     Operator = "is_not_empty"
)
```

### Logical Composition

Conditions support AND, OR, and NOT:

```go
type Condition struct {
    Field    string      `json:"field"`
    Operator Operator    `json:"operator"`
    Value    interface{} `json:"value"`
    And      []Condition `json:"and,omitempty"`
    Or       []Condition `json:"or,omitempty"`
    Not      *Condition  `json:"not,omitempty"`
}
```

### Policy Actions

```go
type Action string

const (
    ActionDeny  Action = "deny"  // Block the operation
    ActionWarn  Action = "warn"  // Allow but log warning
    ActionAudit Action = "audit" // Allow and audit log
)
```

## Consequences

### Positive

- **Proactive enforcement**: Invalid configurations rejected before causing problems
- **Self-documenting**: Policies explain what's allowed and why
- **Audit trail**: All policy evaluations are logged
- **No dependencies**: Runs embedded in Chronos
- **Gradual rollout**: Start with `warn`, upgrade to `deny`

### Negative

- **Language limitations**: Not as powerful as Rego or full programming languages
- **Policy maintenance**: Policies must be updated as requirements change
- **Performance overhead**: Every job operation evaluates policies
- **Debugging**: Failed policies require understanding condition logic

### Example Policies

**Require HTTPS for webhooks:**

```json
{
  "id": "require-https",
  "name": "Require HTTPS Webhooks",
  "type": "job",
  "severity": "error",
  "action": "deny",
  "enabled": true,
  "rules": [{
    "id": "https-only",
    "name": "Webhook URL must use HTTPS",
    "condition": {
      "field": "webhook.url",
      "operator": "starts_with",
      "value": "https://"
    },
    "message": "Webhook URLs must use HTTPS for security"
  }]
}
```

**Limit retry attempts:**

```json
{
  "id": "max-retries",
  "name": "Maximum Retry Limit",
  "type": "job",
  "severity": "error",
  "action": "deny",
  "rules": [{
    "id": "retry-limit",
    "condition": {
      "field": "retry_policy.max_attempts",
      "operator": "less_or_equal",
      "value": 10
    },
    "message": "Jobs cannot have more than 10 retry attempts"
  }]
}
```

**Require tags for production jobs:**

```json
{
  "id": "require-tags",
  "name": "Production Job Tagging",
  "type": "job",
  "severity": "warning",
  "action": "warn",
  "rules": [{
    "id": "owner-tag",
    "condition": {
      "and": [
        {"field": "namespace", "operator": "equals", "value": "production"},
        {"field": "tags.owner", "operator": "exists"}
      ]
    },
    "message": "Production jobs should have an owner tag"
  }]
}
```

### Evaluation Flow

```go
func (e *Engine) Evaluate(ctx context.Context, policyType PolicyType, evalCtx *EvaluationContext) (*AggregateResult, error) {
    results := &AggregateResult{Passed: true}

    for _, compiled := range e.compiled {
        if compiled.policy.Type != policyType || !compiled.policy.Enabled {
            continue
        }

        result := e.evaluatePolicy(compiled, evalCtx)
        results.Results = append(results.Results, result)

        if !result.Passed && compiled.policy.Action == ActionDeny {
            results.Passed = false
        }
    }

    return results, nil
}
```

### Violation Response

```go
type Violation struct {
    RuleID      string   `json:"rule_id"`
    RuleName    string   `json:"rule_name"`
    Message     string   `json:"message"`
    Severity    Severity `json:"severity"`
    Field       string   `json:"field,omitempty"`
    ActualValue string   `json:"actual_value,omitempty"`
}
```

API returns violations in error responses:

```json
{
  "error": "policy violation",
  "violations": [
    {
      "rule_id": "https-only",
      "rule_name": "Webhook URL must use HTTPS",
      "message": "Webhook URLs must use HTTPS for security",
      "field": "webhook.url",
      "actual_value": "http://internal.example.com/webhook"
    }
  ]
}
```

### Built-in Policies

Common policies shipped with Chronos:

```go
// internal/policy/builtin.go
var BuiltinPolicies = []*Policy{
    RequireHTTPS,
    MaxTimeoutLimit,
    MaxRetryLimit,
    ValidCronExpression,
    NoLocalhostWebhooks,
    RequireNamespace,
}
```

## References

- [Open Policy Agent](https://www.openpolicyagent.org/)
- [Policy as Code](https://www.hashicorp.com/resources/what-is-policy-as-code)
- [Kubernetes Admission Controllers](https://kubernetes.io/docs/reference/access-authn-authz/admission-controllers/)
