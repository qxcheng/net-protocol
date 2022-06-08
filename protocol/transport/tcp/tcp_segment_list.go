package tcp

type segmentElementMapper struct{}

//go:nosplit
func (segmentElementMapper) linkerFor(elem *segment) *segment { return elem }

type segmentList struct {
	head *segment
	tail *segment
}

func (l *segmentList) Reset() {
	l.head = nil
	l.tail = nil
}

func (l *segmentList) Empty() bool {
	return l.head == nil
}

func (l *segmentList) Front() *segment {
	return l.head
}

func (l *segmentList) Back() *segment {
	return l.tail
}

func (l *segmentList) PushFront(e *segment) {
	segmentElementMapper{}.linkerFor(e).SetNext(l.head)
	segmentElementMapper{}.linkerFor(e).SetPrev(nil)

	if l.head != nil {
		segmentElementMapper{}.linkerFor(l.head).SetPrev(e)
	} else {
		l.tail = e
	}

	l.head = e
}

func (l *segmentList) PushBack(e *segment) {
	segmentElementMapper{}.linkerFor(e).SetNext(nil)
	segmentElementMapper{}.linkerFor(e).SetPrev(l.tail)

	if l.tail != nil {
		segmentElementMapper{}.linkerFor(l.tail).SetNext(e)
	} else {
		l.head = e
	}

	l.tail = e
}

func (l *segmentList) PushBackList(m *segmentList) {
	if l.head == nil {
		l.head = m.head
		l.tail = m.tail
	} else if m.head != nil {
		segmentElementMapper{}.linkerFor(l.tail).SetNext(m.head)
		segmentElementMapper{}.linkerFor(m.head).SetPrev(l.tail)

		l.tail = m.tail
	}

	m.head = nil
	m.tail = nil
}

// InsertAfter inserts e after b.
func (l *segmentList) InsertAfter(b, e *segment) {
	a := segmentElementMapper{}.linkerFor(b).Next()
	segmentElementMapper{}.linkerFor(e).SetNext(a)
	segmentElementMapper{}.linkerFor(e).SetPrev(b)
	segmentElementMapper{}.linkerFor(b).SetNext(e)

	if a != nil {
		segmentElementMapper{}.linkerFor(a).SetPrev(e)
	} else {
		l.tail = e
	}
}

// InsertBefore inserts e before a.
func (l *segmentList) InsertBefore(a, e *segment) {
	b := segmentElementMapper{}.linkerFor(a).Prev()
	segmentElementMapper{}.linkerFor(e).SetNext(a)
	segmentElementMapper{}.linkerFor(e).SetPrev(b)
	segmentElementMapper{}.linkerFor(a).SetPrev(e)

	if b != nil {
		segmentElementMapper{}.linkerFor(b).SetNext(e)
	} else {
		l.head = e
	}
}

func (l *segmentList) Remove(e *segment) {
	prev := segmentElementMapper{}.linkerFor(e).Prev()
	next := segmentElementMapper{}.linkerFor(e).Next()

	if prev != nil {
		segmentElementMapper{}.linkerFor(prev).SetNext(next)
	} else {
		l.head = next
	}

	if next != nil {
		segmentElementMapper{}.linkerFor(next).SetPrev(prev)
	} else {
		l.tail = prev
	}
}


type segmentEntry struct {
	next *segment
	prev *segment
}

// Next returns the entry that follows e in the list.
func (e *segmentEntry) Next() *segment {
	return e.next
}

// Prev returns the entry that precedes e in the list.
func (e *segmentEntry) Prev() *segment {
	return e.prev
}

// SetNext assigns 'entry' as the entry that follows e in the list.
func (e *segmentEntry) SetNext(elem *segment) {
	e.next = elem
}

// SetPrev assigns 'entry' as the entry that precedes e in the list.
func (e *segmentEntry) SetPrev(elem *segment) {
	e.prev = elem
}