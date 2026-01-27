package scheduler

import "time"

// ScheduledRun represents a scheduled job run.
type ScheduledRun struct {
	JobID   string
	NextRun time.Time
	index   int
}

// PriorityQueue implements a min-heap for scheduled runs.
type PriorityQueue []*ScheduledRun

func (pq PriorityQueue) Len() int { return len(pq) }

func (pq PriorityQueue) Less(i, j int) bool {
	return pq[i].NextRun.Before(pq[j].NextRun)
}

func (pq PriorityQueue) Swap(i, j int) {
	pq[i], pq[j] = pq[j], pq[i]
	pq[i].index = i
	pq[j].index = j
}

func (pq *PriorityQueue) Push(x interface{}) {
	n := len(*pq)
	item := x.(*ScheduledRun)
	item.index = n
	*pq = append(*pq, item)
}

func (pq *PriorityQueue) Pop() interface{} {
	old := *pq
	n := len(old)
	item := old[n-1]
	old[n-1] = nil  // avoid memory leak
	item.index = -1 // for safety
	*pq = old[0 : n-1]
	return item
}

// Remove removes an item at index i from the heap.
func (pq *PriorityQueue) Remove(i int) *ScheduledRun {
	n := pq.Len() - 1
	if n != i {
		pq.Swap(i, n)
		item := (*pq)[n]
		*pq = (*pq)[:n]
		if i < n {
			pq.fix(i)
		}
		return item
	}
	item := (*pq)[n]
	*pq = (*pq)[:n]
	return item
}

// fix re-establishes the heap ordering after the element at index i has changed its value.
func (pq *PriorityQueue) fix(i int) {
	// Bubble up
	for i > 0 {
		parent := (i - 1) / 2
		if !pq.Less(i, parent) {
			break
		}
		pq.Swap(i, parent)
		i = parent
	}

	// Bubble down
	n := pq.Len()
	for {
		left := 2*i + 1
		if left >= n {
			break
		}
		j := left
		if right := left + 1; right < n && pq.Less(right, left) {
			j = right
		}
		if !pq.Less(j, i) {
			break
		}
		pq.Swap(i, j)
		i = j
	}
}

// FindByJobID returns the index of the first item with the given job ID, or -1 if not found.
func (pq *PriorityQueue) FindByJobID(jobID string) int {
	for i, item := range *pq {
		if item.JobID == jobID {
			return i
		}
	}
	return -1
}

// Peek returns the next scheduled run without removing it.
func (pq *PriorityQueue) Peek() (*ScheduledRun, bool) {
	if len(*pq) == 0 {
		return nil, false
	}
	return (*pq)[0], true
}
