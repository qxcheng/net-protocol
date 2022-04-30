// +build go1.9

package tcpip

import (
	_ "time"   // Used with go:linkname.
	_ "unsafe" // Required for go:linkname.
)

// StdClock implements Clock with the time package.
type StdClock struct {}

var _ Clock = (*StdClock)(nil)

//go:linkname now time.now
func now() (sec int64, nsec int32, mono int64)

// NowNanoseconds implements Clock.NowNanoseconds.
func (*StdClock) NowNanoseconds() int64 {
	sec, nsec, _ := now()
	return sec*1e9 + int64(nsec)
}

// NowMonotonic implements Clock.NowMonotonic.
func (*StdClock) NowMonotonic() int64 {
	_, _, mono := now()
	return mono
}