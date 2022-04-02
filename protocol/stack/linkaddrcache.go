package stack

import (
	"github.com/qxcheng/net-protocol/pkg/sleep"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"sync"
	"time"
)

const linkAddrCacheSize = 512 // max cache entries

// linkAddrCache is a fixed-sized cache mapping IP addresses to link addresses.
//
// The entries are stored in a ring buffer, oldest entry replaced first.
//
// This struct is safe for concurrent use.
type linkAddrCache struct {
	// ageLimit is how long a cache entry is valid for.
	ageLimit time.Duration

	// resolutionTimeout is the amount of time to wait for a link request to
	// resolve an address.
	resolutionTimeout time.Duration

	// resolutionAttempts is the number of times an address is attempted to be
	// resolved before failing.
	resolutionAttempts int

	mu      sync.Mutex
	cache   map[tcpip.FullAddress]*linkAddrEntry
	next    int // array index of next available entry
	entries [linkAddrCacheSize]linkAddrEntry
}

// entryState controls the state of a single entry in the cache.
type entryState int

// A linkAddrEntry is an entry in the linkAddrCache.
// This struct is thread-compatible.
type linkAddrEntry struct {
	addr       tcpip.FullAddress
	linkAddr   tcpip.LinkAddress
	expiration time.Time
	s          entryState

	// wakers is a set of waiters for address resolution result. Anytime
	// state transitions out of 'incomplete' these waiters are notified.
	wakers map[*sleep.Waker]struct{}

	done chan struct{}
}