package udp

type udpPacketElementMapper struct{}

//go:nosplit
func (udpPacketElementMapper) linkerFor(elem *udpPacket) *udpPacket { return elem }

// udp数据报的双向链表结构
type udpPacketList struct {
	head *udpPacket
	tail *udpPacket
}

func (l *udpPacketList) Reset() {
	l.head = nil
	l.tail = nil
}

func (l *udpPacketList) Empty() bool {
	return l.head == nil
}

func (l *udpPacketList) Front() *udpPacket {
	return l.head
}

func (l *udpPacketList) Back() *udpPacket {
	return l.tail
}

func (l *udpPacketList) PushFront(e *udpPacket) {
	udpPacketElementMapper{}.linkerFor(e).SetNext(l.head)
	udpPacketElementMapper{}.linkerFor(e).SetPrev(nil)

	if l.head != nil {
		udpPacketElementMapper{}.linkerFor(l.head).SetPrev(e)
	} else {
		l.tail = e
	}

	l.head = e
}

func (l *udpPacketList) PushBack(e *udpPacket) {
	udpPacketElementMapper{}.linkerFor(e).SetNext(nil)
	udpPacketElementMapper{}.linkerFor(e).SetPrev(l.tail)

	if l.tail != nil {
		udpPacketElementMapper{}.linkerFor(l.tail).SetNext(e)
	} else {
		l.head = e
	}

	l.tail = e
}

func (l *udpPacketList) PushBackList(m *udpPacketList) {
	if l.head == nil {
		l.head = m.head
		l.tail = m.tail
	} else if m.head != nil {
		udpPacketElementMapper{}.linkerFor(l.tail).SetNext(m.head)
		udpPacketElementMapper{}.linkerFor(m.head).SetPrev(l.tail)

		l.tail = m.tail
	}

	m.head = nil
	m.tail = nil
}

// InsertAfter inserts e after b.
func (l *udpPacketList) InsertAfter(b, e *udpPacket) {
	a := udpPacketElementMapper{}.linkerFor(b).Next()
	udpPacketElementMapper{}.linkerFor(e).SetNext(a)
	udpPacketElementMapper{}.linkerFor(e).SetPrev(b)
	udpPacketElementMapper{}.linkerFor(b).SetNext(e)

	if a != nil {
		udpPacketElementMapper{}.linkerFor(a).SetPrev(e)
	} else {
		l.tail = e
	}
}

// InsertBefore inserts e before a.
func (l *udpPacketList) InsertBefore(a, e *udpPacket) {
	b := udpPacketElementMapper{}.linkerFor(a).Prev()
	udpPacketElementMapper{}.linkerFor(e).SetNext(a)
	udpPacketElementMapper{}.linkerFor(e).SetPrev(b)
	udpPacketElementMapper{}.linkerFor(a).SetPrev(e)

	if b != nil {
		udpPacketElementMapper{}.linkerFor(b).SetNext(e)
	} else {
		l.head = e
	}
}

// Remove removes e from l.
func (l *udpPacketList) Remove(e *udpPacket) {
	prev := udpPacketElementMapper{}.linkerFor(e).Prev()
	next := udpPacketElementMapper{}.linkerFor(e).Next()

	if prev != nil {
		udpPacketElementMapper{}.linkerFor(prev).SetNext(next)
	} else {
		l.head = next
	}

	if next != nil {
		udpPacketElementMapper{}.linkerFor(next).SetPrev(prev)
	} else {
		l.tail = prev
	}
}

type udpPacketEntry struct {
	next *udpPacket
	prev *udpPacket
}

// Next returns the entry that follows e in the list.
func (e *udpPacketEntry) Next() *udpPacket {
	return e.next
}

// Prev returns the entry that precedes e in the list.
func (e *udpPacketEntry) Prev() *udpPacket {
	return e.prev
}

// SetNext assigns 'entry' as the entry that follows e in the list.
func (e *udpPacketEntry) SetNext(elem *udpPacket) {
	e.next = elem
}

// SetPrev assigns 'entry' as the entry that precedes e in the list.
func (e *udpPacketEntry) SetPrev(elem *udpPacket) {
	e.prev = elem
}