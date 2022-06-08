package tmutex

import "sync/atomic"

// Mutex is a mutual exclusion primitive that implements TryLock in addition
// to Lock and Unlock.
type Mutex struct {
	v  int32
	ch chan struct{}
}

func (m *Mutex) Init() {
	m.v = 1
	m.ch = make(chan struct{}, 1)
}

func (m *Mutex) Lock() {
	if atomic.AddInt32(&m.v, -1) == 0 {
		return
	}

	for {
		if v := atomic.LoadInt32(&m.v); v >= 0 && atomic.SwapInt32(&m.v, -1) == 1 {
			return
		}

		<-m.ch
	}
}

func (m *Mutex) TryLock() bool {
	v := atomic.LoadInt32(&m.v)
	if v <= 0 {
		return false
	}
	return atomic.CompareAndSwapInt32(&m.v, 1, 0)
}

func (m *Mutex) Unlock() {
	if atomic.SwapInt32(&m.v, 1) == 0 {
		return
	}

	select {
	case m.ch <- struct{}{}:
	default:
	}
}
