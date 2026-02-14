// Package dispatcher provides webhook response assertion evaluation.
package dispatcher

import (
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/chronos/chronos/internal/models"
)

// EvaluateAssertions checks webhook response against defined assertions.
func EvaluateAssertions(assertions *models.ResponseAssertions, result *models.ExecutionResult, responseTime time.Duration) *models.AssertionResult {
	if assertions == nil {
		return nil
	}

	ar := &models.AssertionResult{Passed: true}

	// Response time assertion
	if maxRT := assertions.MaxResponseTime.Duration(); maxRT > 0 && responseTime > maxRT {
		ar.Violations = append(ar.Violations, models.AssertionViolation{
			Type:     "response_time",
			Expected: fmt.Sprintf("<= %s", maxRT),
			Actual:   responseTime.String(),
			Message:  fmt.Sprintf("response time %s exceeded max %s", responseTime, maxRT),
		})
	}

	body := result.Response

	// Body contains assertions
	for _, needle := range assertions.BodyContains {
		if !strings.Contains(body, needle) {
			ar.Violations = append(ar.Violations, models.AssertionViolation{
				Type:     "body_contains",
				Expected: needle,
				Actual:   truncate(body, 200),
				Message:  fmt.Sprintf("response body does not contain %q", needle),
			})
		}
	}

	// Body not-contains assertions
	for _, needle := range assertions.BodyNotContains {
		if strings.Contains(body, needle) {
			ar.Violations = append(ar.Violations, models.AssertionViolation{
				Type:     "body_not_contains",
				Expected: fmt.Sprintf("must not contain %q", needle),
				Actual:   truncate(body, 200),
				Message:  fmt.Sprintf("response body contains forbidden string %q", needle),
			})
		}
	}

	// JSON path assertions
	if len(assertions.JSONPathMatchers) > 0 {
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(body), &parsed); err == nil {
			for _, matcher := range assertions.JSONPathMatchers {
				evaluateJSONPathMatcher(ar, parsed, matcher)
			}
		} else if len(assertions.JSONPathMatchers) > 0 {
			ar.Violations = append(ar.Violations, models.AssertionViolation{
				Type:    "json_path",
				Message: fmt.Sprintf("response is not valid JSON: %v", err),
			})
		}
	}

	if len(ar.Violations) > 0 {
		ar.Passed = false
	}

	return ar
}

// evaluateJSONPathMatcher checks a single JSON path matcher.
func evaluateJSONPathMatcher(ar *models.AssertionResult, data map[string]interface{}, matcher models.JSONPathMatcher) {
	value, found := resolveJSONPath(data, matcher.Path)

	op := matcher.Operator
	if op == "" {
		op = "equals"
	}

	actualStr := fmt.Sprintf("%v", value)

	switch op {
	case "equals":
		if !found || actualStr != matcher.Expected {
			ar.Violations = append(ar.Violations, models.AssertionViolation{
				Type:     "json_path",
				Expected: fmt.Sprintf("%s == %q", matcher.Path, matcher.Expected),
				Actual:   actualStr,
				Message:  fmt.Sprintf("JSON path %q: expected %q, got %q", matcher.Path, matcher.Expected, actualStr),
			})
		}
	case "contains":
		if !found || !strings.Contains(actualStr, matcher.Expected) {
			ar.Violations = append(ar.Violations, models.AssertionViolation{
				Type:     "json_path",
				Expected: fmt.Sprintf("%s contains %q", matcher.Path, matcher.Expected),
				Actual:   actualStr,
				Message:  fmt.Sprintf("JSON path %q: expected to contain %q", matcher.Path, matcher.Expected),
			})
		}
	case "not_empty":
		if !found || actualStr == "" || actualStr == "<nil>" {
			ar.Violations = append(ar.Violations, models.AssertionViolation{
				Type:     "json_path",
				Expected: fmt.Sprintf("%s is not empty", matcher.Path),
				Actual:   actualStr,
				Message:  fmt.Sprintf("JSON path %q: expected non-empty value", matcher.Path),
			})
		}
	}
}

// resolveJSONPath resolves a simple dot-notation path against a map.
func resolveJSONPath(data map[string]interface{}, path string) (interface{}, bool) {
	parts := strings.Split(path, ".")
	var current interface{} = data

	for _, part := range parts {
		m, ok := current.(map[string]interface{})
		if !ok {
			return nil, false
		}
		current, ok = m[part]
		if !ok {
			return nil, false
		}
	}

	return current, true
}

func truncate(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + "..."
}
