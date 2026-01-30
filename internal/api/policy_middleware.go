// Package api provides policy enforcement middleware for the REST API.
package api

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"strings"

	"github.com/chronos/chronos/internal/policy"
	"github.com/go-chi/chi/v5"
	"github.com/rs/zerolog"
)

// PolicyEnforcer provides policy enforcement for API operations.
type PolicyEnforcer struct {
	engine *policy.Engine
	logger zerolog.Logger
}

// NewPolicyEnforcer creates a new policy enforcer.
func NewPolicyEnforcer(engine *policy.Engine, logger zerolog.Logger) *PolicyEnforcer {
	if engine == nil {
		engine = policy.NewEngine()
	}
	return &PolicyEnforcer{
		engine: engine,
		logger: logger.With().Str("component", "policy-enforcer").Logger(),
	}
}

// Engine returns the underlying policy engine.
func (p *PolicyEnforcer) Engine() *policy.Engine {
	return p.engine
}

// Middleware returns HTTP middleware that enforces policies on job operations.
func (p *PolicyEnforcer) Middleware() func(next http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Only check policies for mutating job operations
			if !p.shouldEnforcePolicy(r) {
				next.ServeHTTP(w, r)
				return
			}

			// Read and restore the body for downstream handlers
			bodyBytes, err := io.ReadAll(r.Body)
			if err != nil {
				p.writeError(w, http.StatusBadRequest, "BODY_READ_ERROR", "Failed to read request body")
				return
			}
			r.Body = io.NopCloser(bytes.NewBuffer(bodyBytes))

			// Parse the job request for policy evaluation
			var jobData map[string]interface{}
			if len(bodyBytes) > 0 {
				if err := json.Unmarshal(bodyBytes, &jobData); err != nil {
					// Let the handler deal with JSON errors
					next.ServeHTTP(w, r)
					return
				}
			}

			// Build evaluation context
			evalCtx := p.buildEvaluationContext(r, jobData)

			// Evaluate policies
			result, err := p.engine.Evaluate(r.Context(), policy.PolicyTypeJob, evalCtx)
			if err != nil {
				p.logger.Error().Err(err).Msg("Policy evaluation failed")
				// Don't block on evaluation errors - log and continue
				next.ServeHTTP(w, r)
				return
			}

			// Check for policy violations
			if !result.Passed {
				violations := p.collectViolations(result)
				p.logger.Warn().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Interface("violations", violations).
					Msg("Request blocked by policy")

				p.writePolicyViolation(w, violations)
				return
			}

			// Log warnings if any
			warnings := p.collectWarnings(result)
			if len(warnings) > 0 {
				p.logger.Info().
					Str("method", r.Method).
					Str("path", r.URL.Path).
					Interface("warnings", warnings).
					Msg("Policy warnings for request")
			}

			next.ServeHTTP(w, r)
		})
	}
}

// shouldEnforcePolicy determines if policy should be checked for this request.
func (p *PolicyEnforcer) shouldEnforcePolicy(r *http.Request) bool {
	// Enforce policies on job creation, updates, and triggers
	path := r.URL.Path
	method := r.Method

	// POST /api/v1/jobs - create job
	if method == http.MethodPost && strings.HasSuffix(path, "/jobs") {
		return true
	}

	// PUT /api/v1/jobs/{id} - update job
	if method == http.MethodPut && strings.Contains(path, "/jobs/") {
		return true
	}

	// POST /api/v1/jobs/{id}/trigger - trigger job
	if method == http.MethodPost && strings.HasSuffix(path, "/trigger") {
		return true
	}

	return false
}

// buildEvaluationContext creates the evaluation context from the request.
func (p *PolicyEnforcer) buildEvaluationContext(r *http.Request, jobData map[string]interface{}) *policy.EvaluationContext {
	ctx := &policy.EvaluationContext{
		Data:     make(map[string]interface{}),
		Metadata: make(map[string]string),
	}

	// Add job data
	if jobData != nil {
		ctx.Data["job"] = jobData
		// Flatten top-level fields for easier policy access
		for k, v := range jobData {
			ctx.Data[k] = v
		}
	}

	// Add request metadata
	ctx.Metadata["method"] = r.Method
	ctx.Metadata["path"] = r.URL.Path
	ctx.Metadata["remote_addr"] = r.RemoteAddr

	// Add job ID if present
	if jobID := chi.URLParam(r, "id"); jobID != "" {
		ctx.Data["job_id"] = jobID
		ctx.Metadata["job_id"] = jobID
	}

	// Add user info from context if available
	if user := r.Context().Value("user"); user != nil {
		ctx.Data["user"] = user
	}

	return ctx
}

// collectViolations extracts violations from evaluation results.
func (p *PolicyEnforcer) collectViolations(result *policy.AggregateResult) []policyViolationInfo {
	var violations []policyViolationInfo
	for _, r := range result.Results {
		for _, v := range r.Violations {
			violations = append(violations, policyViolationInfo{
				PolicyID:   r.PolicyID,
				PolicyName: r.PolicyName,
				RuleID:     v.RuleID,
				RuleName:   v.RuleName,
				Message:    v.Message,
				Field:      v.Field,
			})
		}
	}
	return violations
}

// collectWarnings extracts warnings from evaluation results.
func (p *PolicyEnforcer) collectWarnings(result *policy.AggregateResult) []policyViolationInfo {
	var warnings []policyViolationInfo
	for _, r := range result.Results {
		for _, w := range r.Warnings {
			warnings = append(warnings, policyViolationInfo{
				PolicyID:   r.PolicyID,
				PolicyName: r.PolicyName,
				RuleID:     w.RuleID,
				RuleName:   w.RuleName,
				Message:    w.Message,
				Field:      w.Field,
			})
		}
	}
	return warnings
}

type policyViolationInfo struct {
	PolicyID   string `json:"policy_id"`
	PolicyName string `json:"policy_name"`
	RuleID     string `json:"rule_id"`
	RuleName   string `json:"rule_name"`
	Message    string `json:"message"`
	Field      string `json:"field,omitempty"`
}

func (p *PolicyEnforcer) writePolicyViolation(w http.ResponseWriter, violations []policyViolationInfo) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusForbidden)
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    "POLICY_VIOLATION",
			Message: "Request violates configured policies",
		},
		Data: map[string]interface{}{
			"violations": violations,
		},
	})
}

func (p *PolicyEnforcer) writeError(w http.ResponseWriter, status int, code, message string) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(Response{
		Success: false,
		Error: &ErrorInfo{
			Code:    code,
			Message: message,
		},
	})
}

// CheckPolicy evaluates a policy against provided data.
// This can be used for direct policy checks outside of HTTP middleware.
func (p *PolicyEnforcer) CheckPolicy(ctx context.Context, policyType policy.PolicyType, data map[string]interface{}) (*policy.AggregateResult, error) {
	evalCtx := &policy.EvaluationContext{
		Data: data,
	}
	return p.engine.Evaluate(ctx, policyType, evalCtx)
}

// LoadBuiltinPolicies loads the default built-in policies.
func (p *PolicyEnforcer) LoadBuiltinPolicies() error {
	builtins := policy.BuiltInPolicies()
	for _, pol := range builtins {
		// Enable built-in policies by default
		pol.Enabled = true
		if err := p.engine.AddPolicy(pol); err != nil {
			if err == policy.ErrPolicyExists {
				// Already loaded, skip
				continue
			}
			return err
		}
		p.logger.Info().Str("policy_id", pol.ID).Str("policy_name", pol.Name).Msg("Loaded built-in policy")
	}
	return nil
}
