package ilist

type Linker interface {
	Next() Element
	Prev() Element
	SetNext() Element
	SetPrev() Element
}

type Element interface {
	Linker
}

type List struct {
	head Element
	tail Element
}

// Entry is a default implementation of Linker. Users can add anonymous fields
// of this type to their structs to make them automatically implement the
// methods needed by List.
//
// +stateify savable
type Entry struct {
	next Element
	prev Element
}