package ilist

type Linker interface {
	Next() Element
	Prev() Element
	SetNext(Element)
	SetPrev(Element)
}

type Element interface {
	Linker
}


type ElementMapper struct{}

//go:nosplit
func (ElementMapper) linkerFor(elem Element) Linker {return elem}


type List struct {
	head Element
	tail Element
}

// Reset 清空List
func (l *List) Reset() {
	l.head = nil
	l.tail = nil
}

// Empty 判断List是否为空
func (l *List) Empty() bool {
	return l.head == nil
}

// Front 返回第一个元素
func (l *List) Front() Element {
	return l.head
}

// Back 返回最后一个元素
func (l *List) Back() Element {
	return l.tail
}

// PushFront 插入元素到列头
func (l *List) PushFront(e Element) {
	ElementMapper{}.linkerFor(e).SetNext(l.head)
	ElementMapper{}.linkerFor(e).SetPrev(nil)

	if l.head != nil {
		ElementMapper{}.linkerFor(l.head).SetPrev(e)
	} else {
		l.tail = e
	}

	l.head = e
}

// PushBack 插入元素到列尾
func (l *List) PushBack(e Element) {
	ElementMapper{}.linkerFor(e).SetNext(nil)
	ElementMapper{}.linkerFor(e).SetPrev(l.tail)

	if l.tail != nil {
		ElementMapper{}.linkerFor(l.tail).SetNext(e)
	} else {
		l.head = e
	}

	l.tail = e
}

// PushBackList 插入一个list到列尾，并清空这个list
func (l *List) PushBackList(m *List) {
	if l.head == nil {
		l.head = m.head
		l.tail = m.tail
	} else if m.head != nil {
		ElementMapper{}.linkerFor(l.tail).SetNext(m.head)
		ElementMapper{}.linkerFor(m.head).SetPrev(l.tail)

		l.tail = m.tail
	}

	m.head = nil
	m.tail = nil
}

// InsertAfter 在b后插入e
func (l *List) InsertAfter(b, e Element) {
	a := ElementMapper{}.linkerFor(b).Next()
	ElementMapper{}.linkerFor(e).SetNext(a)
	ElementMapper{}.linkerFor(e).SetPrev(b)
	ElementMapper{}.linkerFor(b).SetNext(e)

	if a != nil {
		ElementMapper{}.linkerFor(a).SetPrev(e)
	} else {
		l.tail = e
	}
}

// InsertBefore 在a前插入e
func (l *List) InsertBefore(a, e Element) {
	b := ElementMapper{}.linkerFor(a).Prev()
	ElementMapper{}.linkerFor(e).SetNext(a)
	ElementMapper{}.linkerFor(e).SetPrev(b)
	ElementMapper{}.linkerFor(a).SetPrev(e)

	if b != nil {
		ElementMapper{}.linkerFor(b).SetNext(e)
	} else {
		l.head = e
	}
}

// Remove 移除e
func (l *List) Remove(e Element) {
	prev := ElementMapper{}.linkerFor(e).Prev()
	next := ElementMapper{}.linkerFor(e).Next()

	if prev != nil {
		ElementMapper{}.linkerFor(prev).SetNext(next)
	} else {
		l.head = next
	}

	if next != nil {
		ElementMapper{}.linkerFor(next).SetPrev(prev)
	} else {
		l.tail = prev
	}
}


// Entry is a default implementation of Linker. Users can add anonymous fields
// of this type to their structs to make them automatically implement the
// methods needed by List.
type Entry struct {
	next Element
	prev Element
}

func (e *Entry) Next() Element {
	return e.next
}

func (e *Entry) Prev() Element {
	return e.prev
}

func (e *Entry) SetNext(elem Element) {
	e.next = elem
}

func (e *Entry) SetPrev(elem Element) {
	e.prev = elem
}