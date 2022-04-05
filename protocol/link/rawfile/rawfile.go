package rawfile

import (
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"syscall"
	"unsafe"
)

// NonBlockingWrite writes the given buffer to a file descriptor. It fails if
// partial data is written.
func NonBlockingWrite(fd int, buf []byte) *tcpip.Error {
	var ptr unsafe.Pointer
	if len(buf) > 0 {
		ptr = unsafe.Pointer(&buf[0])
	}
	_, _, e := syscall.RawSyscall(syscall.SYS_WRITE, uintptr(fd), uintptr(ptr), uintptr(len(buf)))
	if e != 0 {
		return TranslateErrno(e)
	}
	return nil
}

// NonBlockingWrite2 writes up to two byte slices to a file descriptor in a
// single syscall. It fails if partial data is written.
func NonBlockingWrite2(fd int, b1, b2 []byte) *tcpip.Error {
	if len(b2) == 0 {
		return NonBlockingWrite(fd, b1)
	}
	// We have two buffers. Build the iovec that represents them and issue
	// a writev syscall.
	iovec := [...]syscall.Iovec{
		{
			Base: &b1[0],
			Len:  uint64(len(b1)),
		},
		{
			Base: &b2[0],
			Len:  uint64(len(b2)),
		},
	}
	_, _, e := syscall.RawSyscall(syscall.SYS_WRITEV, uintptr(fd), uintptr(unsafe.Pointer(&iovec[0])), uintptr(len(iovec)))
	if e != 0 {
		return TranslateErrno(e)
	}

	return nil
}


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

// BlockingReadv reads from a file descriptor that is set up as non-blocking and
// stores the data in a list of iovecs buffers. If no data is available, it will
// block in a poll() syscall until the file descirptor becomes readable.
func BlockingReadv(fd int, iovecs []syscall.Iovec) (int, *tcpip.Error) {
	for {
		n, _, e := syscall.RawSyscall(syscall.SYS_READV, uintptr(fd), uintptr(unsafe.Pointer(&iovecs[0])), uintptr(len(iovecs)))
		if e == 0 {
			return int(n), nil
		}

		event := pollEvent{
			fd:      int32(fd),
			events:  1,
		}

		_, e = blockingPoll(&event, 1, -1)
		if e != 0 && e != syscall.EINTR {
			return 0, TranslateErrno(e)
		}

	}
}

