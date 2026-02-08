// Package events provides trigger rule DSL for event-to-job matching.
package events

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Trigger rule errors.
var (
	ErrRuleNotFound     = errors.New("trigger rule not found")
	ErrRuleExists       = errors.New("trigger rule already exists")
	ErrInvalidRule      = errors.New("invalid trigger rule")
	ErrNoMatchingRules  = errors.New("no matching rules for event")
)

// TriggerRule defines how events trigger jobs.
type TriggerRule struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Description string            `json:"description,omitempty"`
	Enabled     bool              `json:"enabled"`

	// Matching criteria
	Source      EventSource       `json:"source"`
	EventTypes  []string          `json:"event_types,omitempty"`   // Event type patterns (glob)
	Condition   *RuleCondition    `json:"condition,omitempty"`     // Complex conditions

	// Target job
	JobID       string            `json:"job_id"`
	JobParams   map[string]string `json:"job_params,omitempty"`    // Parameters to pass

	// Behavior settings
	Throttle    *ThrottleConfig   `json:"throttle,omitempty"`      // Rate limiting
	Delay       time.Duration     `json:"delay,omitempty"`         // Delay before triggering
	Priority    int               `json:"priority"`                // Higher = evaluated first
	Tags        []string          `json:"tags,omitempty"`

	// Timestamps
	CreatedAt   time.Time         `json:"created_at"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// RuleCondition defines complex matching conditions.
type RuleCondition struct {
	// Logical operators (only one should be set)
	And []RuleCondition `json:"and,omitempty"`
	Or  []RuleCondition `json:"or,omitempty"`
	Not *RuleCondition  `json:"not,omitempty"`

	// Field matchers
	Field    string      `json:"field,omitempty"`    // Field path (e.g., "data.bucket", "extensions.region")
	Operator string      `json:"operator,omitempty"` // eq, ne, gt, lt, gte, lte, regex, contains, in, exists
	Value    interface{} `json:"value,omitempty"`
}

// ThrottleConfig defines rate limiting for triggers.
type ThrottleConfig struct {
	MaxEvents   int           `json:"max_events"`   // Max events per window
	Window      time.Duration `json:"window"`       // Time window
	BurstSize   int           `json:"burst_size"`   // Allowed burst
	GroupBy     []string      `json:"group_by"`     // Fields to group by (e.g., ["data.bucket"])
}

// TriggerResult represents the result of rule evaluation.
type TriggerResult struct {
	Rule        *TriggerRule           `json:"rule"`
	Event       *NormalizedEvent       `json:"event"`
	JobID       string                 `json:"job_id"`
	JobParams   map[string]interface{} `json:"job_params"`
	Triggered   bool                   `json:"triggered"`
	Throttled   bool                   `json:"throttled"`
	Delayed     time.Duration          `json:"delayed,omitempty"`
	MatchedAt   time.Time              `json:"matched_at"`
	ExecuteAt   time.Time              `json:"execute_at"`
}

// RuleEngine evaluates trigger rules against events.
type RuleEngine struct {
	mu             sync.RWMutex
	rules          map[string]*TriggerRule
	rulesBySource  map[EventSource][]*TriggerRule
	throttleState  map[string]*throttleWindow
	compiledRules  map[string]*compiledRule
}

type compiledRule struct {
	rule       *TriggerRule
	typeRegexs []*regexp.Regexp
}

type throttleWindow struct {
	count     int
	windowStart time.Time
}

// NewRuleEngine creates a new trigger rule engine.
func NewRuleEngine() *RuleEngine {
	return &RuleEngine{
		rules:         make(map[string]*TriggerRule),
		rulesBySource: make(map[EventSource][]*TriggerRule),
		throttleState: make(map[string]*throttleWindow),
		compiledRules: make(map[string]*compiledRule),
	}
}

// AddRule adds a trigger rule.
func (re *RuleEngine) AddRule(rule *TriggerRule) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	if rule.ID == "" {
		rule.ID = uuid.New().String()
	}

	if _, exists := re.rules[rule.ID]; exists {
		return ErrRuleExists
	}

	if err := re.validateRule(rule); err != nil {
		return err
	}

	now := time.Now().UTC()
	rule.CreatedAt = now
	rule.UpdatedAt = now

	re.rules[rule.ID] = rule
	re.indexRule(rule)
	re.compileRule(rule)

	return nil
}

// UpdateRule updates a trigger rule.
func (re *RuleEngine) UpdateRule(rule *TriggerRule) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	existing, exists := re.rules[rule.ID]
	if !exists {
		return ErrRuleNotFound
	}

	if err := re.validateRule(rule); err != nil {
		return err
	}

	// Remove old index
	re.unindexRule(existing)

	rule.CreatedAt = existing.CreatedAt
	rule.UpdatedAt = time.Now().UTC()

	re.rules[rule.ID] = rule
	re.indexRule(rule)
	re.compileRule(rule)

	return nil
}

// RemoveRule removes a trigger rule.
func (re *RuleEngine) RemoveRule(ruleID string) error {
	re.mu.Lock()
	defer re.mu.Unlock()

	rule, exists := re.rules[ruleID]
	if !exists {
		return ErrRuleNotFound
	}

	re.unindexRule(rule)
	delete(re.rules, ruleID)
	delete(re.compiledRules, ruleID)

	return nil
}

// GetRule returns a rule by ID.
func (re *RuleEngine) GetRule(ruleID string) (*TriggerRule, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	rule, exists := re.rules[ruleID]
	if !exists {
		return nil, ErrRuleNotFound
	}
	return rule, nil
}

// ListRules returns all rules.
func (re *RuleEngine) ListRules() []*TriggerRule {
	re.mu.RLock()
	defer re.mu.RUnlock()

	rules := make([]*TriggerRule, 0, len(re.rules))
	for _, r := range re.rules {
		rules = append(rules, r)
	}
	return rules
}

// Evaluate evaluates all rules against an event.
func (re *RuleEngine) Evaluate(ctx context.Context, event *NormalizedEvent) ([]TriggerResult, error) {
	re.mu.Lock()
	defer re.mu.Unlock()

	var results []TriggerResult

	// Get rules for this source
	rules := re.rulesBySource[event.Source]
	if len(rules) == 0 {
		return results, nil
	}

	// Sort by priority (higher first)
	sortedRules := make([]*TriggerRule, len(rules))
	copy(sortedRules, rules)
	for i := 0; i < len(sortedRules)-1; i++ {
		for j := i + 1; j < len(sortedRules); j++ {
			if sortedRules[j].Priority > sortedRules[i].Priority {
				sortedRules[i], sortedRules[j] = sortedRules[j], sortedRules[i]
			}
		}
	}

	now := time.Now().UTC()

	for _, rule := range sortedRules {
		if !rule.Enabled {
			continue
		}

		matched := re.matchRule(rule, event)
		if !matched {
			continue
		}

		result := TriggerResult{
			Rule:      rule,
			Event:     event,
			JobID:     rule.JobID,
			JobParams: re.resolveParams(rule, event),
			MatchedAt: now,
			ExecuteAt: now,
		}

		// Check throttle
		if rule.Throttle != nil {
			throttleKey := re.getThrottleKey(rule, event)
			if re.isThrottled(throttleKey, rule.Throttle) {
				result.Throttled = true
			}
		}

		// Apply delay
		if rule.Delay > 0 {
			result.Delayed = rule.Delay
			result.ExecuteAt = now.Add(rule.Delay)
		}

		result.Triggered = !result.Throttled

		// Update throttle state
		if result.Triggered && rule.Throttle != nil {
			throttleKey := re.getThrottleKey(rule, event)
			re.recordThrottleEvent(throttleKey, rule.Throttle)
		}

		results = append(results, result)
	}

	return results, nil
}

// matchRule checks if an event matches a rule.
func (re *RuleEngine) matchRule(rule *TriggerRule, event *NormalizedEvent) bool {
	// Match event types (glob patterns)
	if len(rule.EventTypes) > 0 {
		typeMatched := false
		compiled := re.compiledRules[rule.ID]
		if compiled != nil {
			for _, regex := range compiled.typeRegexs {
				if regex.MatchString(event.Type) {
					typeMatched = true
					break
				}
			}
		}
		if !typeMatched {
			return false
		}
	}

	// Match complex condition
	if rule.Condition != nil {
		return re.evaluateCondition(rule.Condition, event)
	}

	return true
}

// evaluateCondition evaluates a complex condition.
func (re *RuleEngine) evaluateCondition(cond *RuleCondition, event *NormalizedEvent) bool {
	// AND
	if len(cond.And) > 0 {
		for _, c := range cond.And {
			if !re.evaluateCondition(&c, event) {
				return false
			}
		}
		return true
	}

	// OR
	if len(cond.Or) > 0 {
		for _, c := range cond.Or {
			if re.evaluateCondition(&c, event) {
				return true
			}
		}
		return false
	}

	// NOT
	if cond.Not != nil {
		return !re.evaluateCondition(cond.Not, event)
	}

	// Field comparison
	if cond.Field != "" {
		fieldValue := re.getFieldValue(event, cond.Field)
		return re.compareValues(fieldValue, cond.Operator, cond.Value)
	}

	return true
}

// getFieldValue extracts a field value from an event using dot notation.
func (re *RuleEngine) getFieldValue(event *NormalizedEvent, path string) interface{} {
	parts := strings.Split(path, ".")
	if len(parts) == 0 {
		return nil
	}

	// Handle special top-level fields
	switch parts[0] {
	case "id":
		return event.ID
	case "source":
		return string(event.Source)
	case "type":
		return event.Type
	case "subject":
		return event.Subject
	case "time":
		return event.Time
	case "extensions":
		if len(parts) > 1 {
			return event.Extensions[parts[1]]
		}
		return event.Extensions
	case "headers":
		if len(parts) > 1 {
			return event.Headers[parts[1]]
		}
		return event.Headers
	case "data":
		return re.traversePath(event.Data, parts[1:])
	}

	return nil
}

// traversePath traverses a nested structure using a path.
func (re *RuleEngine) traversePath(data interface{}, path []string) interface{} {
	if len(path) == 0 || data == nil {
		return data
	}

	switch v := data.(type) {
	case map[string]interface{}:
		return re.traversePath(v[path[0]], path[1:])
	case map[string]string:
		if len(path) == 1 {
			return v[path[0]]
		}
	}

	return nil
}

// compareValues compares a field value against an expected value.
func (re *RuleEngine) compareValues(fieldValue interface{}, operator string, expected interface{}) bool {
	switch operator {
	case "eq", "":
		return fmt.Sprint(fieldValue) == fmt.Sprint(expected)
	case "ne":
		return fmt.Sprint(fieldValue) != fmt.Sprint(expected)
	case "contains":
		return strings.Contains(fmt.Sprint(fieldValue), fmt.Sprint(expected))
	case "regex":
		pattern, ok := expected.(string)
		if !ok {
			return false
		}
		regex, err := regexp.Compile(pattern)
		if err != nil {
			return false
		}
		return regex.MatchString(fmt.Sprint(fieldValue))
	case "in":
		list, ok := expected.([]interface{})
		if !ok {
			return false
		}
		fieldStr := fmt.Sprint(fieldValue)
		for _, item := range list {
			if fmt.Sprint(item) == fieldStr {
				return true
			}
		}
		return false
	case "exists":
		exists := fieldValue != nil
		expectedBool, _ := expected.(bool)
		return exists == expectedBool
	case "gt":
		return compareNumeric(fieldValue, expected) > 0
	case "gte":
		return compareNumeric(fieldValue, expected) >= 0
	case "lt":
		return compareNumeric(fieldValue, expected) < 0
	case "lte":
		return compareNumeric(fieldValue, expected) <= 0
	}

	return false
}

func compareNumeric(a, b interface{}) int {
	aFloat := toFloat64(a)
	bFloat := toFloat64(b)
	if aFloat < bFloat {
		return -1
	}
	if aFloat > bFloat {
		return 1
	}
	return 0
}

func toFloat64(v interface{}) float64 {
	switch val := v.(type) {
	case float64:
		return val
	case float32:
		return float64(val)
	case int:
		return float64(val)
	case int64:
		return float64(val)
	case int32:
		return float64(val)
	case string:
		var f float64
		fmt.Sscanf(val, "%f", &f)
		return f
	}
	return 0
}

// resolveParams resolves job parameters from event data.
func (re *RuleEngine) resolveParams(rule *TriggerRule, event *NormalizedEvent) map[string]interface{} {
	params := make(map[string]interface{})

	// Add event metadata
	params["_event_id"] = event.ID
	params["_event_type"] = event.Type
	params["_event_source"] = string(event.Source)
	params["_event_time"] = event.Time.Format(time.RFC3339)

	// Resolve template parameters
	for key, template := range rule.JobParams {
		params[key] = re.resolveTemplate(template, event)
	}

	return params
}

// resolveTemplate resolves template variables in a parameter value.
func (re *RuleEngine) resolveTemplate(template string, event *NormalizedEvent) interface{} {
	// Replace {{field.path}} with actual values
	result := template
	
	// Find all {{...}} patterns
	re.mu.RLock() // Already locked in Evaluate, but safe for compilation
	defer re.mu.RUnlock()

	pattern := regexp.MustCompile(`\{\{([^}]+)\}\}`)
	matches := pattern.FindAllStringSubmatch(template, -1)

	for _, match := range matches {
		if len(match) == 2 {
			path := strings.TrimSpace(match[1])
			value := re.getFieldValue(event, path)
			result = strings.Replace(result, match[0], fmt.Sprint(value), 1)
		}
	}

	return result
}

// getThrottleKey generates a key for throttle tracking.
func (re *RuleEngine) getThrottleKey(rule *TriggerRule, event *NormalizedEvent) string {
	key := rule.ID

	if rule.Throttle != nil && len(rule.Throttle.GroupBy) > 0 {
		for _, field := range rule.Throttle.GroupBy {
			value := re.getFieldValue(event, field)
			key += ":" + fmt.Sprint(value)
		}
	}

	return key
}

// isThrottled checks if the key is currently throttled.
func (re *RuleEngine) isThrottled(key string, config *ThrottleConfig) bool {
	window, exists := re.throttleState[key]
	if !exists {
		return false
	}

	now := time.Now()
	if now.Sub(window.windowStart) > config.Window {
		// Window expired, reset
		delete(re.throttleState, key)
		return false
	}

	return window.count >= config.MaxEvents
}

// recordThrottleEvent records an event for throttling.
func (re *RuleEngine) recordThrottleEvent(key string, config *ThrottleConfig) {
	now := time.Now()

	window, exists := re.throttleState[key]
	if !exists || now.Sub(window.windowStart) > config.Window {
		re.throttleState[key] = &throttleWindow{
			count:       1,
			windowStart: now,
		}
		return
	}

	window.count++
}

// validateRule validates a trigger rule.
func (re *RuleEngine) validateRule(rule *TriggerRule) error {
	if rule.Name == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidRule)
	}
	if rule.JobID == "" {
		return fmt.Errorf("%w: job_id is required", ErrInvalidRule)
	}
	if rule.Source == "" {
		return fmt.Errorf("%w: source is required", ErrInvalidRule)
	}

	// Validate event type patterns
	for _, pattern := range rule.EventTypes {
		if _, err := globToRegex(pattern); err != nil {
			return fmt.Errorf("%w: invalid event type pattern '%s'", ErrInvalidRule, pattern)
		}
	}

	return nil
}

// indexRule adds a rule to the source index.
func (re *RuleEngine) indexRule(rule *TriggerRule) {
	re.rulesBySource[rule.Source] = append(re.rulesBySource[rule.Source], rule)
}

// unindexRule removes a rule from the source index.
func (re *RuleEngine) unindexRule(rule *TriggerRule) {
	rules := re.rulesBySource[rule.Source]
	for i, r := range rules {
		if r.ID == rule.ID {
			re.rulesBySource[rule.Source] = append(rules[:i], rules[i+1:]...)
			break
		}
	}
}

// compileRule compiles rule patterns for efficient matching.
func (re *RuleEngine) compileRule(rule *TriggerRule) {
	compiled := &compiledRule{
		rule:       rule,
		typeRegexs: make([]*regexp.Regexp, 0, len(rule.EventTypes)),
	}

	for _, pattern := range rule.EventTypes {
		regex, err := globToRegex(pattern)
		if err == nil {
			compiled.typeRegexs = append(compiled.typeRegexs, regex)
		}
	}

	re.compiledRules[rule.ID] = compiled
}

// globToRegex converts a glob pattern to a regex.
func globToRegex(pattern string) (*regexp.Regexp, error) {
	// Escape special regex chars except * and ?
	escaped := regexp.QuoteMeta(pattern)
	// Convert glob wildcards to regex
	escaped = strings.ReplaceAll(escaped, `\*`, ".*")
	escaped = strings.ReplaceAll(escaped, `\?`, ".")
	return regexp.Compile("^" + escaped + "$")
}

// Export exports rules to JSON.
func (re *RuleEngine) Export() ([]byte, error) {
	re.mu.RLock()
	defer re.mu.RUnlock()

	rules := make([]*TriggerRule, 0, len(re.rules))
	for _, r := range re.rules {
		rules = append(rules, r)
	}

	return json.Marshal(rules)
}

// Import imports rules from JSON.
func (re *RuleEngine) Import(data []byte) error {
	var rules []*TriggerRule
	if err := json.Unmarshal(data, &rules); err != nil {
		return err
	}

	for _, rule := range rules {
		if err := re.AddRule(rule); err != nil && !errors.Is(err, ErrRuleExists) {
			return err
		}
	}

	return nil
}

// BuildRule provides a fluent API for building rules.
type RuleBuilder struct {
	rule *TriggerRule
}

// NewRuleBuilder creates a new rule builder.
func NewRuleBuilder(name string) *RuleBuilder {
	return &RuleBuilder{
		rule: &TriggerRule{
			Name:      name,
			Enabled:   true,
			JobParams: make(map[string]string),
		},
	}
}

func (b *RuleBuilder) Source(source EventSource) *RuleBuilder {
	b.rule.Source = source
	return b
}

func (b *RuleBuilder) EventType(patterns ...string) *RuleBuilder {
	b.rule.EventTypes = append(b.rule.EventTypes, patterns...)
	return b
}

func (b *RuleBuilder) Job(jobID string) *RuleBuilder {
	b.rule.JobID = jobID
	return b
}

func (b *RuleBuilder) Param(key, value string) *RuleBuilder {
	b.rule.JobParams[key] = value
	return b
}

func (b *RuleBuilder) Condition(cond *RuleCondition) *RuleBuilder {
	b.rule.Condition = cond
	return b
}

func (b *RuleBuilder) Where(field, operator string, value interface{}) *RuleBuilder {
	b.rule.Condition = &RuleCondition{
		Field:    field,
		Operator: operator,
		Value:    value,
	}
	return b
}

func (b *RuleBuilder) Throttle(maxEvents int, window time.Duration) *RuleBuilder {
	b.rule.Throttle = &ThrottleConfig{
		MaxEvents: maxEvents,
		Window:    window,
	}
	return b
}

func (b *RuleBuilder) Delay(d time.Duration) *RuleBuilder {
	b.rule.Delay = d
	return b
}

func (b *RuleBuilder) Priority(p int) *RuleBuilder {
	b.rule.Priority = p
	return b
}

func (b *RuleBuilder) Build() *TriggerRule {
	return b.rule
}

// And creates an AND condition.
func And(conditions ...*RuleCondition) *RuleCondition {
	conds := make([]RuleCondition, len(conditions))
	for i, c := range conditions {
		conds[i] = *c
	}
	return &RuleCondition{And: conds}
}

// Or creates an OR condition.
func Or(conditions ...*RuleCondition) *RuleCondition {
	conds := make([]RuleCondition, len(conditions))
	for i, c := range conditions {
		conds[i] = *c
	}
	return &RuleCondition{Or: conds}
}

// Not creates a NOT condition.
func Not(condition *RuleCondition) *RuleCondition {
	return &RuleCondition{Not: condition}
}

// Eq creates an equals condition.
func Eq(field string, value interface{}) *RuleCondition {
	return &RuleCondition{Field: field, Operator: "eq", Value: value}
}

// Ne creates a not-equals condition.
func Ne(field string, value interface{}) *RuleCondition {
	return &RuleCondition{Field: field, Operator: "ne", Value: value}
}

// Contains creates a contains condition.
func Contains(field, value string) *RuleCondition {
	return &RuleCondition{Field: field, Operator: "contains", Value: value}
}

// Regex creates a regex condition.
func Regex(field, pattern string) *RuleCondition {
	return &RuleCondition{Field: field, Operator: "regex", Value: pattern}
}

// In creates an in-list condition.
func In(field string, values ...interface{}) *RuleCondition {
	return &RuleCondition{Field: field, Operator: "in", Value: values}
}

// Exists creates an exists condition.
func Exists(field string) *RuleCondition {
	return &RuleCondition{Field: field, Operator: "exists", Value: true}
}
