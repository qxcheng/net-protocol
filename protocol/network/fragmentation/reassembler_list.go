package fragmentation

type reassemblerElementMapper struct{}

//go:nosplit
func (reassemblerElementMapper) linkerFor(elem *reassembler) *reassembler { return elem }


type reassemblerList struct {
	head *reassembler
	tail *reassembler
}

func (l *reassemblerList) Reset() {
	l.head = nil
	l.tail = nil
}

func (l *reassemblerList) Empty() bool {
	return l.head == nil
}

func (l *reassemblerList) Front() *reassembler {
	return l.head
}

func (l *reassemblerList) Back() *reassembler {
	return l.tail
}

func (l *reassemblerList) PushFront(e *reassembler) {
	reassemblerElementMapper{}.linkerFor(e).SetNext(l.head)
	reassemblerElementMapper{}.linkerFor(e).SetPrev(nil)

	if l.head != nil {
		reassemblerElementMapper{}.linkerFor(l.head).SetPrev(e)
	} else {
		l.tail = e
	}

	l.head = e
}

func (l *reassemblerList) PushBack(e *reassembler) {
	reassemblerElementMapper{}.linkerFor(e).SetNext(nil)
	reassemblerElementMapper{}.linkerFor(e).SetPrev(l.tail)

	if l.tail != nil {
		reassemblerElementMapper{}.linkerFor(l.tail).SetNext(e)
	} else {
		l.head = e
	}

	l.tail = e
}

func (l *reassemblerList) PushBackList(m *reassemblerList) {
	if l.head == nil {
		l.head = m.head
		l.tail = m.tail
	} else if m.head != nil {
		reassemblerElementMapper{}.linkerFor(l.tail).SetNext(m.head)
		reassemblerElementMapper{}.linkerFor(m.head).SetPrev(l.tail)

		l.tail = m.tail
	}

	m.head = nil
	m.tail = nil
}

// InsertAfter inserts e after b.
func (l *reassemblerList) InsertAfter(b, e *reassembler) {
	a := reassemblerElementMapper{}.linkerFor(b).Next()
	reassemblerElementMapper{}.linkerFor(e).SetNext(a)
	reassemblerElementMapper{}.linkerFor(e).SetPrev(b)
	reassemblerElementMapper{}.linkerFor(b).SetNext(e)

	if a != nil {
		reassemblerElementMapper{}.linkerFor(a).SetPrev(e)
	} else {
		l.tail = e
	}
}

// InsertBefore inserts e before a.
func (l *reassemblerList) InsertBefore(a, e *reassembler) {
	b := reassemblerElementMapper{}.linkerFor(a).Prev()
	reassemblerElementMapper{}.linkerFor(e).SetNext(a)
	reassemblerElementMapper{}.linkerFor(e).SetPrev(b)
	reassemblerElementMapper{}.linkerFor(a).SetPrev(e)

	if b != nil {
		reassemblerElementMapper{}.linkerFor(b).SetNext(e)
	} else {
		l.head = e
	}
}

// Remove removes e from l.
func (l *reassemblerList) Remove(e *reassembler) {
	prev := reassemblerElementMapper{}.linkerFor(e).Prev()
	next := reassemblerElementMapper{}.linkerFor(e).Next()

	if prev != nil {
		reassemblerElementMapper{}.linkerFor(prev).SetNext(next)
	} else {
		l.head = next
	}

	if next != nil {
		reassemblerElementMapper{}.linkerFor(next).SetPrev(prev)
	} else {
		l.tail = prev
	}
}


type reassemblerEntry struct {
	next *reassembler
	prev *reassembler
}

func (e *reassemblerEntry) Next() *reassembler {
	return e.next
}

func (e *reassemblerEntry) Prev() *reassembler {
	return e.prev
}

func (e *reassemblerEntry) SetNext(elem *reassembler) {
	e.next = elem
}

func (e *reassemblerEntry) SetPrev(elem *reassembler) {
	e.prev = elem
}