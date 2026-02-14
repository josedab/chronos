package dispatcher

import (
	"sync"
	"testing"
	"time"

	"github.com/chronos/chronos/internal/models"
)

func TestEventBus_PublishSubscribe(t *testing.T) {
	bus := NewEventBus(100)
	var received []ExecutionEvent
	var mu sync.Mutex

	bus.Subscribe("", func(e ExecutionEvent) {
		mu.Lock()
		received = append(received, e)
		mu.Unlock()
	})

	bus.Publish(ExecutionEvent{
		Type:        EventExecutionStarted,
		ExecutionID: "exec-1",
		JobID:       "job-1",
		Timestamp:   time.Now(),
	})

	mu.Lock()
	if len(received) != 1 {
		t.Errorf("expected 1 event, got %d", len(received))
	}
	if received[0].Type != EventExecutionStarted {
		t.Errorf("expected started event, got %s", received[0].Type)
	}
	mu.Unlock()
}

func TestEventBus_JobSpecificSubscriber(t *testing.T) {
	bus := NewEventBus(100)
	var job1Events, job2Events int
	var mu sync.Mutex

	bus.Subscribe("job-1", func(e ExecutionEvent) {
		mu.Lock()
		job1Events++
		mu.Unlock()
	})
	bus.Subscribe("job-2", func(e ExecutionEvent) {
		mu.Lock()
		job2Events++
		mu.Unlock()
	})

	bus.Publish(ExecutionEvent{JobID: "job-1", Type: EventExecutionStarted})
	bus.Publish(ExecutionEvent{JobID: "job-1", Type: EventExecutionCompleted})
	bus.Publish(ExecutionEvent{JobID: "job-2", Type: EventExecutionStarted})

	mu.Lock()
	if job1Events != 2 {
		t.Errorf("expected 2 events for job-1, got %d", job1Events)
	}
	if job2Events != 1 {
		t.Errorf("expected 1 event for job-2, got %d", job2Events)
	}
	mu.Unlock()
}

func TestEventBus_Unsubscribe(t *testing.T) {
	bus := NewEventBus(100)
	count := 0

	unsub := bus.Subscribe("", func(e ExecutionEvent) {
		count++
	})

	bus.Publish(ExecutionEvent{Type: EventExecutionStarted})
	if count != 1 {
		t.Errorf("expected 1, got %d", count)
	}

	unsub()
	bus.Publish(ExecutionEvent{Type: EventExecutionStarted})
	if count != 1 {
		t.Errorf("expected still 1 after unsub, got %d", count)
	}
}

func TestEventBus_EmitHelpers(t *testing.T) {
	bus := NewEventBus(100)
	var events []ExecutionEvent
	bus.Subscribe("", func(e ExecutionEvent) {
		events = append(events, e)
	})

	exec := &models.Execution{
		ID:      "exec-1",
		JobID:   "job-1",
		JobName: "Test",
		TraceID: "trace-abc",
		NodeID:  "node-1",
	}

	bus.EmitQueued(exec)
	bus.EmitStarted(exec)
	bus.EmitAttempt(exec, 1, 500, "server error")
	exec.Status = models.ExecutionSuccess
	exec.Duration = time.Second
	bus.EmitCompleted(exec)

	if len(events) != 4 {
		t.Fatalf("expected 4 events, got %d", len(events))
	}
	if events[0].Type != EventExecutionQueued {
		t.Errorf("expected queued, got %s", events[0].Type)
	}
	if events[1].Type != EventExecutionStarted {
		t.Errorf("expected started, got %s", events[1].Type)
	}
	if events[2].Type != EventExecutionAttempt {
		t.Errorf("expected attempt, got %s", events[2].Type)
	}
	if events[3].Type != EventExecutionCompleted {
		t.Errorf("expected completed, got %s", events[3].Type)
	}
	if events[0].TraceID != "trace-abc" {
		t.Errorf("expected trace ID propagation")
	}
}

func TestEventBus_SubscriberCount(t *testing.T) {
	bus := NewEventBus(100)
	if bus.SubscriberCount() != 0 {
		t.Error("expected 0 subscribers")
	}

	bus.Subscribe("", func(e ExecutionEvent) {})
	bus.Subscribe("job-1", func(e ExecutionEvent) {})
	if bus.SubscriberCount() != 2 {
		t.Errorf("expected 2 subscribers, got %d", bus.SubscriberCount())
	}
}
