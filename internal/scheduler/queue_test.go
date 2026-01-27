package scheduler

import (
	"container/heap"
	"testing"
	"time"
)

func TestPriorityQueue(t *testing.T) {
	pq := &PriorityQueue{}
	heap.Init(pq)

	// Add items out of order
	now := time.Now()
	items := []*ScheduledRun{
		{JobID: "job-3", NextRun: now.Add(3 * time.Hour)},
		{JobID: "job-1", NextRun: now.Add(1 * time.Hour)},
		{JobID: "job-2", NextRun: now.Add(2 * time.Hour)},
	}

	for _, item := range items {
		heap.Push(pq, item)
	}

	if pq.Len() != 3 {
		t.Errorf("expected length 3, got %d", pq.Len())
	}

	// Pop should return in order (earliest first)
	first := heap.Pop(pq).(*ScheduledRun)
	if first.JobID != "job-1" {
		t.Errorf("expected job-1 first, got %s", first.JobID)
	}

	second := heap.Pop(pq).(*ScheduledRun)
	if second.JobID != "job-2" {
		t.Errorf("expected job-2 second, got %s", second.JobID)
	}

	third := heap.Pop(pq).(*ScheduledRun)
	if third.JobID != "job-3" {
		t.Errorf("expected job-3 third, got %s", third.JobID)
	}

	if pq.Len() != 0 {
		t.Errorf("expected empty queue, got %d items", pq.Len())
	}
}

func TestPriorityQueue_Peek(t *testing.T) {
	pq := &PriorityQueue{}
	heap.Init(pq)

	// Peek on empty queue
	item, ok := pq.Peek()
	if ok {
		t.Error("expected Peek to return false on empty queue")
	}
	if item != nil {
		t.Error("expected nil item on empty queue")
	}

	// Add items
	now := time.Now()
	heap.Push(pq, &ScheduledRun{JobID: "job-1", NextRun: now.Add(1 * time.Hour)})
	heap.Push(pq, &ScheduledRun{JobID: "job-2", NextRun: now.Add(30 * time.Minute)})

	// Peek should return earliest without removing
	item, ok = pq.Peek()
	if !ok {
		t.Error("expected Peek to return true")
	}
	if item.JobID != "job-2" {
		t.Errorf("expected job-2 from Peek, got %s", item.JobID)
	}

	// Queue should still have 2 items
	if pq.Len() != 2 {
		t.Errorf("expected 2 items after Peek, got %d", pq.Len())
	}
}

func TestPriorityQueue_SameTime(t *testing.T) {
	pq := &PriorityQueue{}
	heap.Init(pq)

	now := time.Now()
	heap.Push(pq, &ScheduledRun{JobID: "job-1", NextRun: now})
	heap.Push(pq, &ScheduledRun{JobID: "job-2", NextRun: now})
	heap.Push(pq, &ScheduledRun{JobID: "job-3", NextRun: now})

	// All should be poppable
	if pq.Len() != 3 {
		t.Errorf("expected 3 items, got %d", pq.Len())
	}

	seen := make(map[string]bool)
	for pq.Len() > 0 {
		item := heap.Pop(pq).(*ScheduledRun)
		seen[item.JobID] = true
	}

	if len(seen) != 3 {
		t.Errorf("expected 3 unique jobs, got %d", len(seen))
	}
}
