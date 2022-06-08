package tcp

import (
	"github.com/qxcheng/net-protocol/protocol/header"
	"sync"
)

type segmentQueue struct {
	mu    sync.Mutex
	list  segmentList
	limit int
	used  int
}

func (q *segmentQueue) empty() bool {
	q.mu.Lock()
	r := q.used == 0
	q.mu.Unlock()

	return r
}

func (q *segmentQueue) setLimit(limit int) {
	q.mu.Lock()
	q.limit = limit
	q.mu.Unlock()
}

func (q *segmentQueue) enqueue(s *segment) bool {
	q.mu.Lock()
	r := q.used < q.limit
	if r {
		q.list.PushBack(s)
		q.used += s.data.Size() + header.TCPMinimumSize
	}
	q.mu.Unlock()

	return r
}

func (q *segmentQueue) dequeue() *segment {
	q.mu.Lock()
	s := q.list.Front()
	if s != nil {
		q.list.Remove(s)
		q.used -= s.data.Size() + header.TCPMinimumSize
	}
	q.mu.Unlock()

	return s
}