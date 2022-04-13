package sleep

import (
	"sync/atomic"
	"unsafe"
)

const (
	preparingG = 1  // 存储在sleepers, 表明它们准备睡眠
)

var (
	assertedSleeper Sleeper  // 哨兵sleeper，其指针被存储在asserted waker中，即w.s
)

//go:linkname gopark runtime.gopark
func gopark(unlockf func(uintptr, *uintptr) bool, wg *uintptr, reason string, traceEv byte, traceskip int)

//go:linkname goready runtime.goready
func goready(g uintptr, traceskip int)

// Sleeper 可以使一个goroutine睡眠，然后被Wakers唤醒。这个结构体是线程兼容的。
// Sleeper的方法并发不安全，一个Sleeper对应多个Waker，一个Waker被添加到sleeper A后，
// 只能在A.Done()返回后才能被添加到另一个sleeper。
type Sleeper struct {
	// asserted wakers的栈，当waker变成asserted状态时被原子地添加到队头
	sharedList unsafe.Pointer

	// 只有waiter能访问的asserted wakers队列，故不用原子访问。当获取更多waker时，waiter首先
	// 遍历这个队列，只有当其为空时，才从sharedList原子性地获取wakers
	localList *Waker

	allWakers *Waker  // 所有关联的wakers，用于cleanup方法
	waitingG uintptr  // 用于持有正在睡眠的G
}

type Waker struct {
	// s 是waker能唤醒的sleeper，有三种取值：
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

func usleeper(s *Sleeper) unsafe.Pointer {
	return unsafe.Pointer(s)
}

func uwaker(w *Waker) unsafe.Pointer {
	return unsafe.Pointer(w)
}

// AddWaker 关联waker到sleeper. id是sleeper被给定waker唤醒后的返回值
func (s *Sleeper) AddWaker(w *Waker, id int) {
	// 添加waker到allwakers队列
	w.allWakersNext = s.allWakers
	s.allWakers = w
	w.id = id

	// 尝试关联waker到sleeper，如果waker已经asserted, 我们将它入队到"ready" list.
	for {
		p := (*Sleeper)(atomic.LoadPointer(&w.s))
		if p == &assertedSleeper {
			s.enqueueAssertedWaker(w)
			return
		}

		if atomic.CompareAndSwapPointer(&w.s, usleeper(p), usleeper(s)) {
			return
		}
	}
}

// nextWaker 返回通知队列的下一个waker，可能会阻塞
func (s *Sleeper) nextWaker(block bool) *Waker {
	// 如果locallist为空，尝试填充
	if s.localList == nil {
		// 循环如果sharedList为空
		for atomic.LoadPointer(&s.sharedList) == nil {
			// 非阻塞直接返回
			if !block {
				return nil
			}

			// Indicate to wakers that we're about to sleep,
			// this allows them to abort the wait by setting
			// waitingG back to zero (which we'll notice
			// before committing the sleep).
			atomic.StoreUintptr(&s.waitingG, preparingG)

			// Check if something was queued while we were
			// preparing to sleep. We need this interleaving
			// to avoid missing wake ups.
			if atomic.LoadPointer(&s.sharedList) != nil {
				atomic.StoreUintptr(&s.waitingG, 0)
				break
			}

			// Try to commit the sleep and report it to the
			// tracer as a select.
			//
			// gopark puts the caller to sleep and calls
			// commitSleep to decide whether to immediately
			// wake the caller up or to leave it sleeping.
			const traceEvGoBlockSelect = 24
			gopark(commitSleep, &s.waitingG, "sleeper", traceEvGoBlockSelect, 0)
		}

		// Pull the shared list out and reverse it in the local
		// list. Given that wakers push themselves in reverse
		// order, we fix things here.
		v := (*Waker)(atomic.SwapPointer(&s.sharedList, nil))
		for v != nil {
			cur := v
			v = v.next

			cur.next = s.localList
			s.localList = cur
		}
	}

	// Remove the waker in the front of the list.
	w := s.localList
	s.localList = w.next

	return w
}

// Fetch fetches the next wake-up notification. If a notification is immediately
// available, it is returned right away. Otherwise, the behavior depends on the
// value of 'block': if true, the current goroutine blocks until a notification
// arrives, then returns it; if false, returns 'ok' as false.
//
// When 'ok' is true, the value of 'id' corresponds to the id associated with
// the waker; when 'ok' is false, 'id' is undefined.
//
// N.B. This method is *not* thread-safe. Only one goroutine at a time is
//      allowed to call this method.
func (s *Sleeper) Fetch(block bool) (id int, ok bool) {
	for {
		w := s.nextWaker(block)
		if w == nil {
			return -1, false
		}

		// Reassociate the waker with the sleeper. If the waker was
		// still asserted we can return it, otherwise try the next one.
		old := (*Sleeper)(atomic.SwapPointer(&w.s, usleeper(s)))
		if old == &assertedSleeper {
			return w.id, true
		}
	}
}

// Done is used to indicate that the caller won't use this Sleeper anymore. It
// removes the association with all wakers so that they can be safely reused
// by another sleeper after Done() returns.
func (s *Sleeper) Done() {
	// Remove all associations that we can, and build a list of the ones
	// we could not. An association can be removed right away from waker w
	// if w.s has a pointer to the sleeper, that is, the waker is not
	// asserted yet. By atomically switching w.s to nil, we guarantee that
	// subsequent calls to Assert() on the waker will not result in it being
	// queued to this sleeper.
	var pending *Waker
	w := s.allWakers
	for w != nil {
		next := w.allWakersNext
		for {
			t := atomic.LoadPointer(&w.s)
			if t != usleeper(s) {
				w.allWakersNext = pending
				pending = w
				break
			}

			if atomic.CompareAndSwapPointer(&w.s, t, nil) {
				break
			}
		}
		w = next
	}

	// The associations that we could not remove are either asserted, or in
	// the process of being asserted, or have been asserted and cleared
	// before being pulled from the sleeper lists. We must wait for them all
	// to make it to the sleeper lists, so that we know that the wakers
	// won't do any more work towards waking this sleeper up.
	for pending != nil {
		pulled := s.nextWaker(true)

		// Remove the waker we just pulled from the list of associated
		// wakers.
		prev := &pending
		for w := *prev; w != nil; w = *prev {
			if pulled == w {
				*prev = w.allWakersNext
				break
			}
			prev = &w.allWakersNext
		}
	}
	s.allWakers = nil
}

func (s *Sleeper) enqueueAssertedWaker(w *Waker) {
	// 将一个asserted waker添加到sharedList
	for {
		v := (*Waker)(atomic.LoadPointer(&s.sharedList))
		w.next = v
		if atomic.CompareAndSwapPointer(&s.sharedList, uwaker(v), uwaker(w)) {
			break
		}
	}

	for {
		// 如果没有等待的G，则什么也不做
		g := atomic.LoadUintptr(&s.waitingG)
		if g == 0 {
			return
		}

		// Signal to the sleeper that a waker has been asserted.
		if atomic.CompareAndSwapUintptr(&s.waitingG, g, 0) {
			if g != preparingG {
				// We managed to get a G. Wake it up.
				goready(g, 0)
			}
		}
	}
}


// Assert moves the waker to an asserted state, if it isn't asserted yet. When
// asserted, the waker will cause its matching sleeper to wake up.
func (w *Waker) Assert() {
	// Nothing to do if the waker is already asserted. This check allows us
	// to complete this case (already asserted) without any interlocked
	// operations on x86.
	if atomic.LoadPointer(&w.s) == usleeper(&assertedSleeper) {
		return
	}

	// Mark the waker as asserted, and wake up a sleeper if there is one.
	switch s := (*Sleeper)(atomic.SwapPointer(&w.s, usleeper(&assertedSleeper))); s {
	case nil:
	case &assertedSleeper:
	default:
		s.enqueueAssertedWaker(w)
	}
}

// Clear moves the waker to then non-asserted state and returns whether it was
// asserted before being cleared.
//
// N.B. The waker isn't removed from the "ready" list of a sleeper (if it
// happens to be in one), but the sleeper will notice that it is not asserted
// anymore and won't return it to the caller.
func (w *Waker) Clear() bool {
	// Nothing to do if the waker is not asserted. This check allows us to
	// complete this case (already not asserted) without any interlocked
	// operations on x86.
	if atomic.LoadPointer(&w.s) != usleeper(&assertedSleeper) {
		return false
	}

	// Try to store nil in the sleeper, which indicates that the waker is
	// not asserted.
	return atomic.CompareAndSwapPointer(&w.s, usleeper(&assertedSleeper), nil)
}

// IsAsserted returns whether the waker is currently asserted (i.e., if it's
// currently in a state that would cause its matching sleeper to wake up).
func (w *Waker) IsAsserted() bool {
	return (*Sleeper)(atomic.LoadPointer(&w.s)) == &assertedSleeper
}