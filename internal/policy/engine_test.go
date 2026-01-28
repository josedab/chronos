package policy

import (
	"context"
	"testing"
)

func TestEngine_AddPolicy(t *testing.T) {
	engine := NewEngine()

	policy := &Policy{
		ID:       "test-policy",
		Name:     "Test Policy",
		Type:     PolicyTypeJob,
		Severity: SeverityError,
		Action:   ActionDeny,
		Enabled:  true,
		Rules: []Rule{
			{
				ID:   "rule-1",
				Name: "Test Rule",
				Condition: Condition{
					Field:    "name",
					Operator: OpEquals,
					Value:    "test",
				},
				Message: "Name must be 'test'",
			},
		},
	}

	err := engine.AddPolicy(policy)
	if err != nil {
		t.Fatalf("AddPolicy failed: %v", err)
	}

	// Adding same policy again should fail
	err = engine.AddPolicy(policy)
	if err != ErrPolicyExists {
		t.Errorf("Expected ErrPolicyExists, got %v", err)
	}
}

func TestEngine_Evaluate(t *testing.T) {
	engine := NewEngine()

	policy := &Policy{
		ID:       "test-policy",
		Name:     "Test Policy",
		Type:     PolicyTypeJob,
		Severity: SeverityError,
		Action:   ActionDeny,
		Enabled:  true,
		Rules: []Rule{
			{
				ID:   "rule-1",
				Name: "Name Check",
				Condition: Condition{
					Field:    "name",
					Operator: OpEquals,
					Value:    "valid-name",
				},
				Message: "Name must be 'valid-name'",
			},
		},
	}

	engine.AddPolicy(policy)

	tests := []struct {
		name     string
		data     map[string]interface{}
		wantPass bool
	}{
		{
			name:     "valid name",
			data:     map[string]interface{}{"name": "valid-name"},
			wantPass: true,
		},
		{
			name:     "invalid name",
			data:     map[string]interface{}{"name": "invalid"},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := engine.Evaluate(ctx, PolicyTypeJob, &EvaluationContext{Data: tt.data})
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}
			if result.Passed != tt.wantPass {
				t.Errorf("Expected passed=%v, got %v", tt.wantPass, result.Passed)
			}
		})
	}
}

func TestEngine_NestedField(t *testing.T) {
	engine := NewEngine()

	policy := &Policy{
		ID:       "nested-policy",
		Name:     "Nested Field Policy",
		Type:     PolicyTypeJob,
		Severity: SeverityError,
		Action:   ActionDeny,
		Enabled:  true,
		Rules: []Rule{
			{
				ID:   "rule-1",
				Name: "Webhook URL Check",
				Condition: Condition{
					Field:    "webhook.url",
					Operator: OpStartsWith,
					Value:    "https://",
				},
				Message: "Webhook URL must use HTTPS",
			},
		},
	}

	engine.AddPolicy(policy)

	tests := []struct {
		name     string
		data     map[string]interface{}
		wantPass bool
	}{
		{
			name: "https url",
			data: map[string]interface{}{
				"webhook": map[string]interface{}{
					"url": "https://example.com",
				},
			},
			wantPass: true,
		},
		{
			name: "http url",
			data: map[string]interface{}{
				"webhook": map[string]interface{}{
					"url": "http://example.com",
				},
			},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := engine.Evaluate(ctx, PolicyTypeJob, &EvaluationContext{Data: tt.data})
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}
			if result.Passed != tt.wantPass {
				t.Errorf("Expected passed=%v, got %v", tt.wantPass, result.Passed)
			}
		})
	}
}

func TestEngine_OrCondition(t *testing.T) {
	engine := NewEngine()

	policy := &Policy{
		ID:       "or-policy",
		Name:     "OR Condition Policy",
		Type:     PolicyTypeJob,
		Severity: SeverityError,
		Action:   ActionDeny,
		Enabled:  true,
		Rules: []Rule{
			{
				ID:   "rule-1",
				Name: "Environment Check",
				Condition: Condition{
					Or: []Condition{
						{Field: "env", Operator: OpEquals, Value: "prod"},
						{Field: "env", Operator: OpEquals, Value: "staging"},
					},
				},
				Message: "Environment must be 'prod' or 'staging'",
			},
		},
	}

	engine.AddPolicy(policy)

	tests := []struct {
		name     string
		data     map[string]interface{}
		wantPass bool
	}{
		{
			name:     "prod environment",
			data:     map[string]interface{}{"env": "prod"},
			wantPass: true,
		},
		{
			name:     "staging environment",
			data:     map[string]interface{}{"env": "staging"},
			wantPass: true,
		},
		{
			name:     "dev environment",
			data:     map[string]interface{}{"env": "dev"},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := engine.Evaluate(ctx, PolicyTypeJob, &EvaluationContext{Data: tt.data})
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}
			if result.Passed != tt.wantPass {
				t.Errorf("Expected passed=%v, got %v", tt.wantPass, result.Passed)
			}
		})
	}
}

func TestEngine_AndCondition(t *testing.T) {
	engine := NewEngine()

	policy := &Policy{
		ID:       "and-policy",
		Name:     "AND Condition Policy",
		Type:     PolicyTypeJob,
		Severity: SeverityError,
		Action:   ActionDeny,
		Enabled:  true,
		Rules: []Rule{
			{
				ID:   "rule-1",
				Name: "Combined Check",
				Condition: Condition{
					And: []Condition{
						{Field: "enabled", Operator: OpEquals, Value: "true"},
						{Field: "timeout", Operator: OpGreaterThan, Value: 0.0},
					},
				},
				Message: "Job must be enabled with timeout > 0",
			},
		},
	}

	engine.AddPolicy(policy)

	tests := []struct {
		name     string
		data     map[string]interface{}
		wantPass bool
	}{
		{
			name:     "enabled with valid timeout",
			data:     map[string]interface{}{"enabled": "true", "timeout": 30.0},
			wantPass: true,
		},
		{
			name:     "disabled with valid timeout",
			data:     map[string]interface{}{"enabled": "false", "timeout": 30.0},
			wantPass: false,
		},
		{
			name:     "enabled with zero timeout",
			data:     map[string]interface{}{"enabled": "true", "timeout": 0.0},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := engine.Evaluate(ctx, PolicyTypeJob, &EvaluationContext{Data: tt.data})
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}
			if result.Passed != tt.wantPass {
				t.Errorf("Expected passed=%v, got %v", tt.wantPass, result.Passed)
			}
		})
	}
}

func TestEngine_RegexMatches(t *testing.T) {
	engine := NewEngine()

	policy := &Policy{
		ID:       "regex-policy",
		Name:     "Regex Policy",
		Type:     PolicyTypeJob,
		Severity: SeverityError,
		Action:   ActionDeny,
		Enabled:  true,
		Rules: []Rule{
			{
				ID:   "rule-1",
				Name: "Name Pattern Check",
				Condition: Condition{
					Field:    "name",
					Operator: OpMatches,
					Value:    "^[a-z][a-z0-9-]*$",
				},
				Message: "Name must match pattern",
			},
		},
	}

	engine.AddPolicy(policy)

	tests := []struct {
		name     string
		data     map[string]interface{}
		wantPass bool
	}{
		{
			name:     "valid name",
			data:     map[string]interface{}{"name": "my-job-123"},
			wantPass: true,
		},
		{
			name:     "invalid name with uppercase",
			data:     map[string]interface{}{"name": "My-Job"},
			wantPass: false,
		},
		{
			name:     "invalid name starting with number",
			data:     map[string]interface{}{"name": "123-job"},
			wantPass: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			result, err := engine.Evaluate(ctx, PolicyTypeJob, &EvaluationContext{Data: tt.data})
			if err != nil {
				t.Fatalf("Evaluate failed: %v", err)
			}
			if result.Passed != tt.wantPass {
				t.Errorf("Expected passed=%v, got %v", tt.wantPass, result.Passed)
			}
		})
	}
}

func TestBuiltInPolicies(t *testing.T) {
	policies := BuiltInPolicies()
	if len(policies) == 0 {
		t.Error("Expected built-in policies to be non-empty")
	}

	engine := NewEngine()
	for _, policy := range policies {
		if err := engine.AddPolicy(policy); err != nil {
			t.Errorf("Failed to add built-in policy %s: %v", policy.ID, err)
		}
	}
}

func TestCreateWebhookDomainPolicy(t *testing.T) {
	policy := CreateWebhookDomainPolicy([]string{"api.example.com", "internal.example.com"})
	
	if policy.ID == "" {
		t.Error("Expected policy to have an ID")
	}
	if len(policy.Rules) == 0 {
		t.Error("Expected policy to have rules")
	}
	if len(policy.Rules[0].Condition.Or) != 2 {
		t.Errorf("Expected 2 OR conditions, got %d", len(policy.Rules[0].Condition.Or))
	}
}

func TestEngine_ContextCancellation(t *testing.T) {
	engine := NewEngine()

	// Add a simple policy
	policy := &Policy{
		ID:       "test-policy",
		Name:     "Test Policy",
		Type:     PolicyTypeJob,
		Severity: SeverityError,
		Action:   ActionDeny,
		Enabled:  true,
		Rules: []Rule{
			{
				ID:   "rule-1",
				Name: "Test Rule",
				Condition: Condition{
					Field:    "name",
					Operator: OpEquals,
					Value:    "test",
				},
				Message: "Test message",
			},
		},
	}
	engine.AddPolicy(policy)

	// Create cancelled context
	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	_, err := engine.Evaluate(ctx, PolicyTypeJob, &EvaluationContext{Data: map[string]interface{}{"name": "test"}})
	if err != context.Canceled {
		t.Errorf("Expected context.Canceled error, got %v", err)
	}
}

func BenchmarkEngine_Evaluate(b *testing.B) {
	engine := NewEngine()

	// Add multiple policies
	for _, policy := range BuiltInPolicies() {
		engine.AddPolicy(policy)
	}

	ctx := context.Background()
	evalCtx := &EvaluationContext{
		Data: map[string]interface{}{
			"name": "my-job",
			"webhook": map[string]interface{}{
				"url": "https://api.example.com/webhook",
			},
			"timeout_seconds": 300.0,
			"retry_policy": map[string]interface{}{
				"max_attempts": 3.0,
			},
			"tags": map[string]interface{}{
				"team":        "backend",
				"environment": "production",
			},
		},
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		engine.Evaluate(ctx, PolicyTypeJob, evalCtx)
	}
}
