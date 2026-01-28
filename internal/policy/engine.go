// Package policy provides policy-as-code enforcement for Chronos jobs.
package policy

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"
)

// Common errors.
var (
	ErrPolicyNotFound     = errors.New("policy not found")
	ErrPolicyExists       = errors.New("policy already exists")
	ErrPolicyViolation    = errors.New("policy violation")
	ErrInvalidPolicy      = errors.New("invalid policy")
	ErrPolicyEvalFailed   = errors.New("policy evaluation failed")
)

// PolicyType represents the type of policy.
type PolicyType string

const (
	// PolicyTypeJob applies to job configurations
	PolicyTypeJob PolicyType = "job"
	// PolicyTypeExecution applies to job executions
	PolicyTypeExecution PolicyType = "execution"
	// PolicyTypeCluster applies to cluster operations
	PolicyTypeCluster PolicyType = "cluster"
)

// Severity represents the severity of a policy violation.
type Severity string

const (
	SeverityError   Severity = "error"
	SeverityWarning Severity = "warning"
	SeverityInfo    Severity = "info"
)

// Action represents the action to take on policy violation.
type Action string

const (
	ActionDeny  Action = "deny"
	ActionWarn  Action = "warn"
	ActionAudit Action = "audit"
)

// Policy represents a policy rule.
type Policy struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Type        PolicyType        `json:"type"`
	Severity    Severity          `json:"severity"`
	Action      Action            `json:"action"`
	Enabled     bool              `json:"enabled"`
	Rules       []Rule            `json:"rules"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Rule represents a single policy rule.
type Rule struct {
	ID          string      `json:"id"`
	Name        string      `json:"name"`
	Description string      `json:"description,omitempty"`
	Condition   Condition   `json:"condition"`
	Message     string      `json:"message"`
}

// Condition represents a policy condition.
type Condition struct {
	Field    string      `json:"field"`
	Operator Operator    `json:"operator"`
	Value    interface{} `json:"value"`
	And      []Condition `json:"and,omitempty"`
	Or       []Condition `json:"or,omitempty"`
	Not      *Condition  `json:"not,omitempty"`
}

// Operator represents a comparison operator.
type Operator string

const (
	OpEquals          Operator = "equals"
	OpNotEquals       Operator = "not_equals"
	OpContains        Operator = "contains"
	OpNotContains     Operator = "not_contains"
	OpStartsWith      Operator = "starts_with"
	OpEndsWith        Operator = "ends_with"
	OpMatches         Operator = "matches" // regex
	OpIn              Operator = "in"
	OpNotIn           Operator = "not_in"
	OpGreaterThan     Operator = "greater_than"
	OpGreaterOrEqual  Operator = "greater_or_equal"
	OpLessThan        Operator = "less_than"
	OpLessOrEqual     Operator = "less_or_equal"
	OpExists          Operator = "exists"
	OpNotExists       Operator = "not_exists"
	OpIsEmpty         Operator = "is_empty"
	OpIsNotEmpty      Operator = "is_not_empty"
)

// EvaluationResult represents the result of evaluating a policy.
type EvaluationResult struct {
	PolicyID   string       `json:"policy_id"`
	PolicyName string       `json:"policy_name"`
	Passed     bool         `json:"passed"`
	Violations []Violation  `json:"violations,omitempty"`
	Warnings   []Violation  `json:"warnings,omitempty"`
	Duration   time.Duration `json:"duration"`
}

// Violation represents a policy violation.
type Violation struct {
	RuleID      string   `json:"rule_id"`
	RuleName    string   `json:"rule_name"`
	Message     string   `json:"message"`
	Severity    Severity `json:"severity"`
	Field       string   `json:"field,omitempty"`
	ActualValue string   `json:"actual_value,omitempty"`
}

// EvaluationContext provides context for policy evaluation.
type EvaluationContext struct {
	Data     map[string]interface{} `json:"data"`
	Metadata map[string]string      `json:"metadata,omitempty"`
}

// Engine evaluates policies against input data.
type Engine struct {
	mu       sync.RWMutex
	policies map[string]*Policy
	compiled map[string]*compiledPolicy
}

type compiledPolicy struct {
	policy *Policy
	rules  []*compiledRule
}

type compiledRule struct {
	rule      *Rule
	evaluator conditionEvaluator
}

type conditionEvaluator func(ctx *EvaluationContext) (bool, error)

// NewEngine creates a new policy engine.
func NewEngine() *Engine {
	return &Engine{
		policies: make(map[string]*Policy),
		compiled: make(map[string]*compiledPolicy),
	}
}

// AddPolicy adds a policy to the engine.
func (e *Engine) AddPolicy(policy *Policy) error {
	if policy == nil || policy.ID == "" {
		return ErrInvalidPolicy
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.policies[policy.ID]; exists {
		return ErrPolicyExists
	}

	// Compile the policy
	compiled, err := e.compilePolicy(policy)
	if err != nil {
		return fmt.Errorf("failed to compile policy: %w", err)
	}

	e.policies[policy.ID] = policy
	e.compiled[policy.ID] = compiled

	return nil
}

// UpdatePolicy updates an existing policy.
func (e *Engine) UpdatePolicy(policy *Policy) error {
	if policy == nil || policy.ID == "" {
		return ErrInvalidPolicy
	}

	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.policies[policy.ID]; !exists {
		return ErrPolicyNotFound
	}

	// Compile the policy
	compiled, err := e.compilePolicy(policy)
	if err != nil {
		return fmt.Errorf("failed to compile policy: %w", err)
	}

	policy.UpdatedAt = time.Now()
	e.policies[policy.ID] = policy
	e.compiled[policy.ID] = compiled

	return nil
}

// RemovePolicy removes a policy from the engine.
func (e *Engine) RemovePolicy(policyID string) error {
	e.mu.Lock()
	defer e.mu.Unlock()

	if _, exists := e.policies[policyID]; !exists {
		return ErrPolicyNotFound
	}

	delete(e.policies, policyID)
	delete(e.compiled, policyID)

	return nil
}

// GetPolicy retrieves a policy by ID.
func (e *Engine) GetPolicy(policyID string) (*Policy, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policy, exists := e.policies[policyID]
	if !exists {
		return nil, ErrPolicyNotFound
	}

	return policy, nil
}

// ListPolicies returns all policies.
func (e *Engine) ListPolicies() []*Policy {
	e.mu.RLock()
	defer e.mu.RUnlock()

	policies := make([]*Policy, 0, len(e.policies))
	for _, p := range e.policies {
		policies = append(policies, p)
	}

	return policies
}

// Evaluate evaluates all enabled policies of a given type against the context.
func (e *Engine) Evaluate(ctx context.Context, policyType PolicyType, evalCtx *EvaluationContext) (*AggregateResult, error) {
	e.mu.RLock()
	defer e.mu.RUnlock()

	start := time.Now()
	results := &AggregateResult{
		Passed:  true,
		Results: make([]*EvaluationResult, 0),
	}

	for _, compiled := range e.compiled {
		if compiled.policy.Type != policyType || !compiled.policy.Enabled {
			continue
		}

		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}

		result := e.evaluatePolicy(compiled, evalCtx)
		results.Results = append(results.Results, result)

		if !result.Passed && compiled.policy.Action == ActionDeny {
			results.Passed = false
		}
	}

	results.Duration = time.Since(start)
	return results, nil
}

// EvaluatePolicy evaluates a single policy against the context.
func (e *Engine) EvaluatePolicy(ctx context.Context, policyID string, evalCtx *EvaluationContext) (*EvaluationResult, error) {
	e.mu.RLock()
	compiled, exists := e.compiled[policyID]
	e.mu.RUnlock()

	if !exists {
		return nil, ErrPolicyNotFound
	}

	return e.evaluatePolicy(compiled, evalCtx), nil
}

// evaluatePolicy evaluates a compiled policy.
func (e *Engine) evaluatePolicy(compiled *compiledPolicy, evalCtx *EvaluationContext) *EvaluationResult {
	start := time.Now()
	result := &EvaluationResult{
		PolicyID:   compiled.policy.ID,
		PolicyName: compiled.policy.Name,
		Passed:     true,
		Violations: make([]Violation, 0),
		Warnings:   make([]Violation, 0),
	}

	for _, rule := range compiled.rules {
		passed, err := rule.evaluator(evalCtx)
		if err != nil {
			result.Passed = false
			result.Violations = append(result.Violations, Violation{
				RuleID:   rule.rule.ID,
				RuleName: rule.rule.Name,
				Message:  fmt.Sprintf("evaluation error: %v", err),
				Severity: SeverityError,
			})
			continue
		}

		if !passed {
			violation := Violation{
				RuleID:   rule.rule.ID,
				RuleName: rule.rule.Name,
				Message:  rule.rule.Message,
				Severity: compiled.policy.Severity,
				Field:    rule.rule.Condition.Field,
			}

			// Get actual value for the violation
			if evalCtx != nil && rule.rule.Condition.Field != "" {
				if val, err := getFieldValue(evalCtx.Data, rule.rule.Condition.Field); err == nil {
					violation.ActualValue = fmt.Sprintf("%v", val)
				}
			}

			switch compiled.policy.Severity {
			case SeverityError:
				result.Passed = false
				result.Violations = append(result.Violations, violation)
			case SeverityWarning:
				result.Warnings = append(result.Warnings, violation)
			case SeverityInfo:
				result.Warnings = append(result.Warnings, violation)
			}
		}
	}

	result.Duration = time.Since(start)
	return result
}

// compilePolicy compiles a policy into an evaluable form.
func (e *Engine) compilePolicy(policy *Policy) (*compiledPolicy, error) {
	compiled := &compiledPolicy{
		policy: policy,
		rules:  make([]*compiledRule, 0, len(policy.Rules)),
	}

	for i := range policy.Rules {
		rule := &policy.Rules[i]
		evaluator, err := e.compileCondition(&rule.Condition)
		if err != nil {
			return nil, fmt.Errorf("failed to compile rule %s: %w", rule.ID, err)
		}

		compiled.rules = append(compiled.rules, &compiledRule{
			rule:      rule,
			evaluator: evaluator,
		})
	}

	return compiled, nil
}

// compileCondition compiles a condition into an evaluator function.
func (e *Engine) compileCondition(cond *Condition) (conditionEvaluator, error) {
	// Handle AND conditions
	if len(cond.And) > 0 {
		evaluators := make([]conditionEvaluator, 0, len(cond.And))
		for i := range cond.And {
			eval, err := e.compileCondition(&cond.And[i])
			if err != nil {
				return nil, err
			}
			evaluators = append(evaluators, eval)
		}
		return func(ctx *EvaluationContext) (bool, error) {
			for _, eval := range evaluators {
				passed, err := eval(ctx)
				if err != nil {
					return false, err
				}
				if !passed {
					return false, nil
				}
			}
			return true, nil
		}, nil
	}

	// Handle OR conditions
	if len(cond.Or) > 0 {
		evaluators := make([]conditionEvaluator, 0, len(cond.Or))
		for i := range cond.Or {
			eval, err := e.compileCondition(&cond.Or[i])
			if err != nil {
				return nil, err
			}
			evaluators = append(evaluators, eval)
		}
		return func(ctx *EvaluationContext) (bool, error) {
			for _, eval := range evaluators {
				passed, err := eval(ctx)
				if err != nil {
					return false, err
				}
				if passed {
					return true, nil
				}
			}
			return false, nil
		}, nil
	}

	// Handle NOT condition
	if cond.Not != nil {
		eval, err := e.compileCondition(cond.Not)
		if err != nil {
			return nil, err
		}
		return func(ctx *EvaluationContext) (bool, error) {
			passed, err := eval(ctx)
			if err != nil {
				return false, err
			}
			return !passed, nil
		}, nil
	}

	// Compile regex for matches operator
	var compiledRegex *regexp.Regexp
	if cond.Operator == OpMatches {
		if pattern, ok := cond.Value.(string); ok {
			var err error
			compiledRegex, err = regexp.Compile(pattern)
			if err != nil {
				return nil, fmt.Errorf("invalid regex pattern: %w", err)
			}
		}
	}

	// Simple condition
	return func(ctx *EvaluationContext) (bool, error) {
		if ctx == nil {
			return false, errors.New("nil evaluation context")
		}

		fieldValue, err := getFieldValue(ctx.Data, cond.Field)
		if err != nil {
			// For exists/not_exists, missing field is valid
			if cond.Operator == OpNotExists {
				return true, nil
			}
			if cond.Operator == OpExists {
				return false, nil
			}
			return false, nil // Treat missing field as failure for other operators
		}

		return evaluateOperator(cond.Operator, fieldValue, cond.Value, compiledRegex)
	}, nil
}

// AggregateResult contains the results of evaluating multiple policies.
type AggregateResult struct {
	Passed   bool                `json:"passed"`
	Results  []*EvaluationResult `json:"results"`
	Duration time.Duration       `json:"duration"`
}

// getFieldValue retrieves a value from nested data using dot notation.
func getFieldValue(data map[string]interface{}, field string) (interface{}, error) {
	parts := strings.Split(field, ".")
	current := interface{}(data)

	for _, part := range parts {
		switch v := current.(type) {
		case map[string]interface{}:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("field not found: %s", part)
			}
			current = val
		case map[string]string:
			val, ok := v[part]
			if !ok {
				return nil, fmt.Errorf("field not found: %s", part)
			}
			current = val
		default:
			return nil, fmt.Errorf("cannot traverse into non-map type at %s", part)
		}
	}

	return current, nil
}

// evaluateOperator evaluates a single operator.
func evaluateOperator(op Operator, fieldValue, expectedValue interface{}, compiledRegex *regexp.Regexp) (bool, error) {
	switch op {
	case OpEquals:
		return fmt.Sprintf("%v", fieldValue) == fmt.Sprintf("%v", expectedValue), nil

	case OpNotEquals:
		return fmt.Sprintf("%v", fieldValue) != fmt.Sprintf("%v", expectedValue), nil

	case OpContains:
		return strings.Contains(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", expectedValue)), nil

	case OpNotContains:
		return !strings.Contains(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", expectedValue)), nil

	case OpStartsWith:
		return strings.HasPrefix(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", expectedValue)), nil

	case OpEndsWith:
		return strings.HasSuffix(fmt.Sprintf("%v", fieldValue), fmt.Sprintf("%v", expectedValue)), nil

	case OpMatches:
		if compiledRegex != nil {
			return compiledRegex.MatchString(fmt.Sprintf("%v", fieldValue)), nil
		}
		return false, errors.New("no compiled regex for matches operator")

	case OpIn:
		if list, ok := expectedValue.([]interface{}); ok {
			strVal := fmt.Sprintf("%v", fieldValue)
			for _, item := range list {
				if fmt.Sprintf("%v", item) == strVal {
					return true, nil
				}
			}
		}
		return false, nil

	case OpNotIn:
		if list, ok := expectedValue.([]interface{}); ok {
			strVal := fmt.Sprintf("%v", fieldValue)
			for _, item := range list {
				if fmt.Sprintf("%v", item) == strVal {
					return false, nil
				}
			}
		}
		return true, nil

	case OpExists:
		return fieldValue != nil, nil

	case OpNotExists:
		return fieldValue == nil, nil

	case OpIsEmpty:
		str := fmt.Sprintf("%v", fieldValue)
		return str == "" || str == "<nil>", nil

	case OpIsNotEmpty:
		str := fmt.Sprintf("%v", fieldValue)
		return str != "" && str != "<nil>", nil

	case OpGreaterThan, OpGreaterOrEqual, OpLessThan, OpLessOrEqual:
		return compareNumeric(op, fieldValue, expectedValue)

	default:
		return false, fmt.Errorf("unsupported operator: %s", op)
	}
}

// compareNumeric performs numeric comparison.
func compareNumeric(op Operator, fieldValue, expectedValue interface{}) (bool, error) {
	fv, err := toFloat64(fieldValue)
	if err != nil {
		return false, err
	}

	ev, err := toFloat64(expectedValue)
	if err != nil {
		return false, err
	}

	switch op {
	case OpGreaterThan:
		return fv > ev, nil
	case OpGreaterOrEqual:
		return fv >= ev, nil
	case OpLessThan:
		return fv < ev, nil
	case OpLessOrEqual:
		return fv <= ev, nil
	default:
		return false, fmt.Errorf("unsupported comparison operator: %s", op)
	}
}

// toFloat64 converts a value to float64.
func toFloat64(v interface{}) (float64, error) {
	switch val := v.(type) {
	case float64:
		return val, nil
	case float32:
		return float64(val), nil
	case int:
		return float64(val), nil
	case int64:
		return float64(val), nil
	case int32:
		return float64(val), nil
	case json.Number:
		return val.Float64()
	default:
		return 0, fmt.Errorf("cannot convert %T to float64", v)
	}
}
