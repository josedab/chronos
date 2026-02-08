// Package events provides deduplication and exactly-once semantics for event processing.
package events

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"sync"
	"time"

	"github.com/google/uuid"
)

// Deduplication errors.
var (
	ErrDuplicateEvent   = errors.New("duplicate event detected")
	ErrCheckpointFailed = errors.New("checkpoint failed")
)

// DeduplicationConfig configures the deduplication engine.
type DeduplicationConfig struct {
	// TTL is how long to remember processed events.
	TTL time.Duration `json:"ttl" yaml:"ttl"`
	// MaxSize is the maximum number of event IDs to track.
	MaxSize int `json:"max_size" yaml:"max_size"`
	// EnablePersistence enables checkpoint persistence.
	EnablePersistence bool `json:"enable_persistence" yaml:"enable_persistence"`
	// CheckpointInterval is how often to save checkpoints.
	CheckpointInterval time.Duration `json:"checkpoint_interval" yaml:"checkpoint_interval"`
	// IdempotencyKeyFields are fields used to generate idempotency keys.
	IdempotencyKeyFields []string `json:"idempotency_key_fields" yaml:"idempotency_key_fields"`
}

// DefaultDeduplicationConfig returns sensible defaults.
func DefaultDeduplicationConfig() DeduplicationConfig {
	return DeduplicationConfig{
		TTL:                  24 * time.Hour,
		MaxSize:              1000000,
		EnablePersistence:    true,
		CheckpointInterval:   5 * time.Minute,
		IdempotencyKeyFields: []string{"id", "source", "type"},
	}
}

// ProcessedEvent tracks a processed event.
type ProcessedEvent struct {
	EventID        string    `json:"event_id"`
	IdempotencyKey string    `json:"idempotency_key"`
	ProcessedAt    time.Time `json:"processed_at"`
	JobID          string    `json:"job_id,omitempty"`
	ExecutionID    string    `json:"execution_id,omitempty"`
	Success        bool      `json:"success"`
}

// SourceCheckpoint tracks processing position for a source.
type SourceCheckpoint struct {
	Source      EventSource       `json:"source"`
	Partition   string            `json:"partition,omitempty"`
	Offset      int64             `json:"offset,omitempty"`
	Timestamp   time.Time         `json:"timestamp"`
	Metadata    map[string]string `json:"metadata,omitempty"`
	UpdatedAt   time.Time         `json:"updated_at"`
}

// Deduplicator provides event deduplication and exactly-once semantics.
type Deduplicator struct {
	mu             sync.RWMutex
	config         DeduplicationConfig
	processed      map[string]*ProcessedEvent
	processedList  []string // For LRU eviction
	checkpoints    map[string]*SourceCheckpoint
	pendingAcks    map[string]*PendingAck
	store          CheckpointStore
	stopCh         chan struct{}
}

// PendingAck tracks events awaiting acknowledgment.
type PendingAck struct {
	Event       *NormalizedEvent
	ReceivedAt  time.Time
	Timeout     time.Duration
	Callback    func(success bool)
}

// CheckpointStore persists checkpoints.
type CheckpointStore interface {
	SaveCheckpoint(ctx context.Context, checkpoint *SourceCheckpoint) error
	LoadCheckpoint(ctx context.Context, source EventSource, partition string) (*SourceCheckpoint, error)
	SaveProcessedEvents(ctx context.Context, events []*ProcessedEvent) error
	LoadProcessedEvents(ctx context.Context, since time.Time) ([]*ProcessedEvent, error)
}

// NewDeduplicator creates a new deduplication engine.
func NewDeduplicator(config DeduplicationConfig, store CheckpointStore) *Deduplicator {
	d := &Deduplicator{
		config:        config,
		processed:     make(map[string]*ProcessedEvent),
		processedList: make([]string, 0),
		checkpoints:   make(map[string]*SourceCheckpoint),
		pendingAcks:   make(map[string]*PendingAck),
		store:         store,
		stopCh:        make(chan struct{}),
	}

	// Start background cleanup
	go d.cleanupLoop()

	// Load persisted state
	if store != nil && config.EnablePersistence {
		go d.loadPersistedState()
	}

	return d
}

// IsDuplicate checks if an event is a duplicate.
func (d *Deduplicator) IsDuplicate(event *NormalizedEvent) bool {
	key := d.generateIdempotencyKey(event)

	d.mu.RLock()
	_, exists := d.processed[key]
	d.mu.RUnlock()

	return exists
}

// MarkProcessing marks an event as being processed (for at-most-once).
func (d *Deduplicator) MarkProcessing(event *NormalizedEvent) (string, error) {
	key := d.generateIdempotencyKey(event)

	d.mu.Lock()
	defer d.mu.Unlock()

	if _, exists := d.processed[key]; exists {
		return "", ErrDuplicateEvent
	}

	// Add to pending
	d.pendingAcks[key] = &PendingAck{
		Event:      event,
		ReceivedAt: time.Now().UTC(),
		Timeout:    5 * time.Minute,
	}

	return key, nil
}

// AcknowledgeSuccess marks an event as successfully processed.
func (d *Deduplicator) AcknowledgeSuccess(key string, jobID, executionID string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	pending, exists := d.pendingAcks[key]
	if !exists {
		return
	}

	processed := &ProcessedEvent{
		EventID:        pending.Event.ID,
		IdempotencyKey: key,
		ProcessedAt:    time.Now().UTC(),
		JobID:          jobID,
		ExecutionID:    executionID,
		Success:        true,
	}

	d.processed[key] = processed
	d.processedList = append(d.processedList, key)
	delete(d.pendingAcks, key)

	// Evict if over max size
	d.evictIfNeeded()

	// Persist asynchronously
	if d.store != nil && d.config.EnablePersistence {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			d.store.SaveProcessedEvents(ctx, []*ProcessedEvent{processed})
		}()
	}
}

// AcknowledgeFailure marks an event as failed (allows retry).
func (d *Deduplicator) AcknowledgeFailure(key string, allowRetry bool) {
	d.mu.Lock()
	defer d.mu.Unlock()

	pending, exists := d.pendingAcks[key]
	if !exists {
		return
	}

	if !allowRetry {
		// Mark as processed (won't retry)
		d.processed[key] = &ProcessedEvent{
			EventID:        pending.Event.ID,
			IdempotencyKey: key,
			ProcessedAt:    time.Now().UTC(),
			Success:        false,
		}
		d.processedList = append(d.processedList, key)
	}

	delete(d.pendingAcks, key)
}

// GetIdempotencyKey generates and returns the idempotency key for an event.
func (d *Deduplicator) GetIdempotencyKey(event *NormalizedEvent) string {
	return d.generateIdempotencyKey(event)
}

// generateIdempotencyKey generates a unique key for deduplication.
func (d *Deduplicator) generateIdempotencyKey(event *NormalizedEvent) string {
	// Check for explicit idempotency key in extensions
	if key, ok := event.Extensions["idempotency_key"].(string); ok && key != "" {
		return key
	}

	// Generate from configured fields
	var parts []string
	for _, field := range d.config.IdempotencyKeyFields {
		var value string
		switch field {
		case "id":
			value = event.ID
		case "source":
			value = string(event.Source)
		case "type":
			value = event.Type
		case "subject":
			value = event.Subject
		case "time":
			value = event.Time.Format(time.RFC3339Nano)
		default:
			if v, ok := event.Extensions[field]; ok {
				value = toString(v)
			}
		}
		if value != "" {
			parts = append(parts, value)
		}
	}

	if len(parts) == 0 {
		// Fallback: hash the entire event
		data, _ := json.Marshal(event)
		hash := sha256.Sum256(data)
		return hex.EncodeToString(hash[:])
	}

	// Hash the combined parts
	combined := ""
	for i, part := range parts {
		if i > 0 {
			combined += ":"
		}
		combined += part
	}

	hash := sha256.Sum256([]byte(combined))
	return hex.EncodeToString(hash[:16]) // Use first 16 bytes
}

// SetCheckpoint sets the checkpoint for a source/partition.
func (d *Deduplicator) SetCheckpoint(source EventSource, partition string, offset int64, metadata map[string]string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := checkpointKey(source, partition)
	now := time.Now().UTC()

	checkpoint := &SourceCheckpoint{
		Source:    source,
		Partition: partition,
		Offset:    offset,
		Timestamp: now,
		Metadata:  metadata,
		UpdatedAt: now,
	}

	d.checkpoints[key] = checkpoint

	// Persist asynchronously
	if d.store != nil && d.config.EnablePersistence {
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer cancel()
			if err := d.store.SaveCheckpoint(ctx, checkpoint); err != nil {
				// Log error
				_ = err
			}
		}()
	}

	return nil
}

// GetCheckpoint returns the checkpoint for a source/partition.
func (d *Deduplicator) GetCheckpoint(source EventSource, partition string) (*SourceCheckpoint, error) {
	d.mu.RLock()
	checkpoint, exists := d.checkpoints[checkpointKey(source, partition)]
	d.mu.RUnlock()

	if exists {
		return checkpoint, nil
	}

	// Try to load from store
	if d.store != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return d.store.LoadCheckpoint(ctx, source, partition)
	}

	return nil, nil
}

// GetProcessedEvent returns a processed event by key.
func (d *Deduplicator) GetProcessedEvent(key string) (*ProcessedEvent, bool) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	event, exists := d.processed[key]
	return event, exists
}

// evictIfNeeded removes old entries if over max size.
func (d *Deduplicator) evictIfNeeded() {
	for len(d.processedList) > d.config.MaxSize {
		oldestKey := d.processedList[0]
		delete(d.processed, oldestKey)
		d.processedList = d.processedList[1:]
	}
}

// cleanupLoop periodically cleans up expired entries.
func (d *Deduplicator) cleanupLoop() {
	ticker := time.NewTicker(d.config.TTL / 10)
	defer ticker.Stop()

	for {
		select {
		case <-ticker.C:
			d.cleanup()
		case <-d.stopCh:
			return
		}
	}
}

// cleanup removes expired entries.
func (d *Deduplicator) cleanup() {
	d.mu.Lock()
	defer d.mu.Unlock()

	cutoff := time.Now().Add(-d.config.TTL)
	newList := make([]string, 0, len(d.processedList))

	for _, key := range d.processedList {
		if event, exists := d.processed[key]; exists {
			if event.ProcessedAt.After(cutoff) {
				newList = append(newList, key)
			} else {
				delete(d.processed, key)
			}
		}
	}

	d.processedList = newList

	// Clean up expired pending acks
	now := time.Now()
	for key, pending := range d.pendingAcks {
		if now.Sub(pending.ReceivedAt) > pending.Timeout {
			delete(d.pendingAcks, key)
		}
	}
}

// loadPersistedState loads state from the store.
func (d *Deduplicator) loadPersistedState() {
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	since := time.Now().Add(-d.config.TTL)
	events, err := d.store.LoadProcessedEvents(ctx, since)
	if err != nil {
		return
	}

	d.mu.Lock()
	defer d.mu.Unlock()

	for _, event := range events {
		d.processed[event.IdempotencyKey] = event
		d.processedList = append(d.processedList, event.IdempotencyKey)
	}
}

// Stop stops the deduplicator.
func (d *Deduplicator) Stop() {
	close(d.stopCh)
}

// Stats returns deduplication statistics.
func (d *Deduplicator) Stats() DeduplicationStats {
	d.mu.RLock()
	defer d.mu.RUnlock()

	return DeduplicationStats{
		TotalTracked:   len(d.processed),
		PendingAcks:    len(d.pendingAcks),
		Checkpoints:    len(d.checkpoints),
		MaxSize:        d.config.MaxSize,
		TTL:            d.config.TTL,
	}
}

// DeduplicationStats contains deduplication statistics.
type DeduplicationStats struct {
	TotalTracked int           `json:"total_tracked"`
	PendingAcks  int           `json:"pending_acks"`
	Checkpoints  int           `json:"checkpoints"`
	MaxSize      int           `json:"max_size"`
	TTL          time.Duration `json:"ttl"`
}

func checkpointKey(source EventSource, partition string) string {
	return string(source) + ":" + partition
}

func toString(v interface{}) string {
	switch val := v.(type) {
	case string:
		return val
	default:
		data, _ := json.Marshal(v)
		return string(data)
	}
}

// InMemoryCheckpointStore is an in-memory implementation of CheckpointStore.
type InMemoryCheckpointStore struct {
	mu          sync.RWMutex
	checkpoints map[string]*SourceCheckpoint
	events      []*ProcessedEvent
}

// NewInMemoryCheckpointStore creates a new in-memory checkpoint store.
func NewInMemoryCheckpointStore() *InMemoryCheckpointStore {
	return &InMemoryCheckpointStore{
		checkpoints: make(map[string]*SourceCheckpoint),
		events:      make([]*ProcessedEvent, 0),
	}
}

func (s *InMemoryCheckpointStore) SaveCheckpoint(ctx context.Context, checkpoint *SourceCheckpoint) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	key := checkpointKey(checkpoint.Source, checkpoint.Partition)
	s.checkpoints[key] = checkpoint
	return nil
}

func (s *InMemoryCheckpointStore) LoadCheckpoint(ctx context.Context, source EventSource, partition string) (*SourceCheckpoint, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	key := checkpointKey(source, partition)
	return s.checkpoints[key], nil
}

func (s *InMemoryCheckpointStore) SaveProcessedEvents(ctx context.Context, events []*ProcessedEvent) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	s.events = append(s.events, events...)
	return nil
}

func (s *InMemoryCheckpointStore) LoadProcessedEvents(ctx context.Context, since time.Time) ([]*ProcessedEvent, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var result []*ProcessedEvent
	for _, e := range s.events {
		if e.ProcessedAt.After(since) {
			result = append(result, e)
		}
	}
	return result, nil
}

// EventProcessor combines ingestion, deduplication, and trigger evaluation.
type EventProcessor struct {
	ingestor     *Ingestor
	dedup        *Deduplicator
	rules        *RuleEngine
	jobTrigger   JobTriggerFunc
}

// JobTriggerFunc is called to trigger a job.
type JobTriggerFunc func(ctx context.Context, jobID string, params map[string]interface{}) (executionID string, err error)

// NewEventProcessor creates a new event processor.
func NewEventProcessor(jobTrigger JobTriggerFunc) *EventProcessor {
	dedupConfig := DefaultDeduplicationConfig()
	store := NewInMemoryCheckpointStore()

	ep := &EventProcessor{
		dedup:      NewDeduplicator(dedupConfig, store),
		rules:      NewRuleEngine(),
		jobTrigger: jobTrigger,
	}

	// Create ingestor with handler
	ingestorConfig := DefaultIngestorConfig()
	ep.ingestor = NewIngestor(ingestorConfig, ep.handleEvent)

	return ep
}

// handleEvent processes an incoming event.
func (ep *EventProcessor) handleEvent(ctx context.Context, event *NormalizedEvent) error {
	// Check for duplicate
	if ep.dedup.IsDuplicate(event) {
		return ErrDuplicateEvent
	}

	// Mark as processing
	key, err := ep.dedup.MarkProcessing(event)
	if err != nil {
		return err
	}

	// Evaluate trigger rules
	results, err := ep.rules.Evaluate(ctx, event)
	if err != nil {
		ep.dedup.AcknowledgeFailure(key, true) // Allow retry
		return err
	}

	if len(results) == 0 {
		// No matching rules, but not an error
		ep.dedup.AcknowledgeSuccess(key, "", "")
		return nil
	}

	// Trigger jobs
	var lastExecID string
	for _, result := range results {
		if !result.Triggered {
			continue
		}

		// Apply delay if needed
		if result.Delayed > 0 {
			time.Sleep(result.Delayed)
		}

		execID, err := ep.jobTrigger(ctx, result.JobID, result.JobParams)
		if err != nil {
			// Log but continue with other triggers
			continue
		}
		lastExecID = execID
	}

	// Mark as processed
	ep.dedup.AcknowledgeSuccess(key, results[0].JobID, lastExecID)

	return nil
}

// Ingest ingests an event.
func (ep *EventProcessor) Ingest(ctx context.Context, source EventSource, data []byte, headers map[string]string) error {
	return ep.ingestor.Ingest(ctx, source, data, headers)
}

// AddRule adds a trigger rule.
func (ep *EventProcessor) AddRule(rule *TriggerRule) error {
	return ep.rules.AddRule(rule)
}

// GetStats returns combined statistics.
func (ep *EventProcessor) GetStats() map[string]interface{} {
	return map[string]interface{}{
		"ingestor":      ep.ingestor.GetMetrics(),
		"deduplication": ep.dedup.Stats(),
		"rules":         len(ep.rules.ListRules()),
	}
}

// Stop stops the event processor.
func (ep *EventProcessor) Stop() {
	ep.ingestor.Stop()
	ep.dedup.Stop()
}

// Ensure uuid is used
var _ = uuid.New
