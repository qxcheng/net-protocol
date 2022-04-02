package sleep

import "unsafe"

type Waker struct {
	// s is the sleeper that this waker can wake up. Only one sleeper at a
	// time is allowed. This field can have three classes of values:
	// nil -- the waker is not asserted: it either is not associated with
	//     a sleeper, or is queued to a sleeper due to being previously
	//     asserted. This is the zero value.
	// &assertedSleeper -- the waker is asserted.
	// otherwise -- the waker is not asserted, and is associated with the
	//     given sleeper. Once it transitions to asserted state, the
	//     associated sleeper will be woken.
	s unsafe.Pointer

	// next is used to form a linked list of asserted wakers in a sleeper.
	next *Waker

	// allWakersNext is used to form a linked list of all wakers associated
	// to a given sleeper.
	allWakersNext *Waker

	// id is the value to be returned to sleepers when they wake up due to
	// this waker being asserted.
	id int
}