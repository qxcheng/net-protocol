package rawfile

import (
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"syscall"
	"unsafe"
)

type pollEvent struct {
	fd      int32
	events  int16
	revents int16
}

func blockingPoll(fds *pollEvent, nfds int, timeout int64) (int, syscall.Errno) {
	n, _, e := syscall.Syscall(syscall.SYS_POLL, uintptr(unsafe.Pointer(fds)), uintptr(nfds), uintptr(timeout))
	return int(n), e
}

// BlockingRead reads from a file descriptor that is set up as non-blocking. If
// no data is available, it will block in a poll() syscall until the file
// descirptor becomes readable.
func BlockingRead(fd int, b []byte) (int, *tcpip.Error) {
	for {
		n, _, e := syscall.RawSyscall(syscall.SYS_READ, uintptr(fd), uintptr(unsafe.Pointer(&b[0])), uintptr(len(b)))
		if e == 0 {
			return int(n), nil
		}

		event := pollEvent{
			fd: int32(fd),
			events: 1,
		}

		_, e = blockingPoll(&event, 1, -1)
		if e != 0 && e != syscall.EINTR {
			return 0, TranslateErrno(e)
		}
	}
}