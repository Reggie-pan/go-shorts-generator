package worker

import "sync"

type Queue struct {
	ch     chan string
	cancel map[string]bool
	mu     sync.Mutex
}

func NewQueue(size int) *Queue {
	return &Queue{
		ch:     make(chan string, size),
		cancel: map[string]bool{},
	}
}

func (q *Queue) Push(id string) {
	q.ch <- id
}

func (q *Queue) Pop() string {
	return <-q.ch
}

func (q *Queue) Cancel(id string) {
	q.mu.Lock()
	defer q.mu.Unlock()
	q.cancel[id] = true
}

func (q *Queue) IsCanceled(id string) bool {
	q.mu.Lock()
	defer q.mu.Unlock()
	return q.cancel[id]
}
