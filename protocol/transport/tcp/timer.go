package tcp

import (
	"github.com/qxcheng/net-protocol/pkg/sleep"
	"time"
)

type timerState int
const (
	timerStateDisabled timerState = iota
	timerStateEnabled
	timerStateOrphaned
)

// 定时器的实现
type timer struct {
	// state is the current state of the timer:
	//     disabled - the timer is disabled.
	//     orphaned - the timer is disabled, but the runtime timer is
	//                enabled, which means that it will evetually cause a
	//                spurious wake (unless it gets enabled again before
	//                then).
	//     enabled  - the timer is enabled, but the runtime timer may be set
	//                to an earlier expiration time due to a previous
	//                orphaned state.
	state timerState

	// target is the expiration time of the current timer. It is only
	// meaningful in the enabled state.
	target time.Time

	// runtimeTarget is the expiration time of the runtime timer. It is
	// meaningful in the enabled and orphaned states.
	runtimeTarget time.Time

	// timer is the runtime timer used to wait on.
	timer *time.Timer
}

// 初始化 timer, 到期时执行waker.Assert()
func (t *timer) init(w *sleep.Waker) {
	t.state = timerStateDisabled

	t.timer = time.AfterFunc(time.Hour, func() {
		w.Assert()
	})
	t.timer.Stop()
}

func (t *timer) cleanup() {
	t.timer.Stop()
}

// checkExpiration checks if the given timer has actually expired, it should be
// called whenever a sleeper wakes up due to the waker being asserted, and is
// used to check if it's a supurious wake (due to a previously orphaned timer)
// or a legitimate one.
func (t *timer) checkExpiration() bool {
	// Transition to fully disabled state if we're just consuming an
	// orphaned timer.
	if t.state == timerStateOrphaned {
		t.state = timerStateDisabled
		return false
	}

	// The timer is enabled, but it may have expired early. Check if that's
	// the case, and if so, reset the runtime timer to the correct time.
	now := time.Now()
	if now.Before(t.target) {
		t.runtimeTarget = t.target
		t.timer.Reset(t.target.Sub(now))
		return false
	}

	// The timer has actually expired, disable it for now and inform the
	// caller.
	t.state = timerStateDisabled
	return true
}

// disable disables the timer, leaving it in an orphaned state if it wasn't
// already disabled.
func (t *timer) disable() {
	if t.state != timerStateDisabled {
		t.state = timerStateOrphaned
	}
}

// enabled returns true if the timer is currently enabled, false otherwise.
func (t *timer) enabled() bool {
	return t.state == timerStateEnabled
}

// enable enables the timer, programming the runtime timer if necessary.
func (t *timer) enable(d time.Duration) {
	t.target = time.Now().Add(d)

	// Check if we need to set the runtime timer.
	if t.state == timerStateDisabled || t.target.Before(t.runtimeTarget) {
		t.runtimeTarget = t.target
		t.timer.Reset(d)
	}

	t.state = timerStateEnabled
}
