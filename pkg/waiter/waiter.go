package waiter

import (
	"github.com/qxcheng/net-protocol/pkg/ilist"
	"sync"
)

// EventMask represents io events as used in the poll() syscall.
type EventMask uint16

type Queue struct {
	list ilist.List
	mu sync.RWMutex
}
