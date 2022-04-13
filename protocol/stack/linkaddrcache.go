package stack

import (
	"fmt"
	"github.com/qxcheng/net-protocol/pkg/sleep"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"log"
	"sync"
	"time"
)

const linkAddrCacheSize = 512 // 最大缓存数

// linkAddrCache ip-mac的环形缓存，最久的被替换，并发安全
type linkAddrCache struct {
	ageLimit time.Duration           // 缓存有效时间
	resolutionTimeout time.Duration  // 等待一个链路层请求解析ip地址的时间
	resolutionAttempts int           // 解析ip地址的最大尝试次数
	mu      sync.Mutex
	cache   map[tcpip.FullAddress]*linkAddrEntry
	next    int                      // 下一个可用条目的数组索引
	entries [linkAddrCacheSize]linkAddrEntry
}

type entryState int  // 单个条目的状态控制

const (
	incomplete entryState = iota  // 已发出解析请求，初始状态
	ready                         // 解析成功
	failed                        // 解析超时
	expired                       // 缓存已过期需要重新解析
)

// linkAddrEntry 一个缓存条目，线程兼容的
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


func newLinkAddrCache(ageLimit, resolutionTimeout time.Duration, resolutionAttempts int) *linkAddrCache {
	return &linkAddrCache{
		ageLimit:           ageLimit,
		resolutionTimeout:  resolutionTimeout,
		resolutionAttempts: resolutionAttempts,
		cache:              make(map[tcpip.FullAddress]*linkAddrEntry, linkAddrCacheSize),
	}
}


// String implements Stringer.
func (s entryState) String() string {
	switch s {
	case incomplete:
		return "incomplete"
	case ready:
		return "ready"
	case failed:
		return "failed"
	case expired:
		return "expired"
	default:
		return fmt.Sprintf("unknown(%d)", s)
	}
}

func (e *linkAddrEntry) state() entryState {
	if e.s != expired && time.Now().After(e.expiration) {
		// Force the transition to ensure waiters are notified.
		e.changeState(expired)
	}
	return e.s
}

func (e *linkAddrEntry) changeState(ns entryState) {
	if e.s == ns {
		return
	}

	// 校验状态转换合法性
	switch e.s {
	case incomplete:
		// All transitions are valid.
	case ready, failed:
		if ns != expired {
			panic(fmt.Sprintf("invalid state transition from %s to %s", e.s, ns))
		}
	case expired:
		// Terminal state.
		panic(fmt.Sprintf("invalid state transition from %s to %s", e.s, ns))
	default:
		panic(fmt.Sprintf("invalid state: %s", e.s))
	}

	// Notify whoever is waiting on address resolution when transitioning
	// out of 'incomplete'.
	if e.s == incomplete {
		for w := range e.wakers {
			w.Assert()
		}
		e.wakers = nil
		if e.done != nil {
			close(e.done)
		}
	}
	e.s = ns
}

func (e *linkAddrEntry) addWaker(w *sleep.Waker) {
	e.wakers[w] = struct{}{}
}

func (e *linkAddrEntry) removeWaker(w *sleep.Waker) {
	delete(e.wakers, w)
}


// 添加一条缓存条目
func (c *linkAddrCache) add(k tcpip.FullAddress, v tcpip.LinkAddress) {
	log.Printf("add link cache: %v-%v", k, v)
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.cache[k]
	// 如果已缓存
	if ok {
		s := entry.state()
		// 未过期且已存在的k-v对
		if s != expired && entry.linkAddr == v {
			return
		}
		// 如果正在等待地址解析
		if s == incomplete {
			entry.linkAddr = v
		} else {
			entry = c.makeAndAddEntry(k, v)
		}
	} else {
		entry = c.makeAndAddEntry(k, v)
	}

	entry.changeState(ready)
}

// makeAndAddEntry 创建和添加缓存条目，淘汰旧的条目
func (c *linkAddrCache) makeAndAddEntry(k tcpip.FullAddress, v tcpip.LinkAddress) *linkAddrEntry {
	entry := &c.entries[c.next]
	// 下一个可用位置已被缓存，清除掉
	if c.cache[entry.addr] == entry {
		delete(c.cache, entry.addr)
	}

	// Mark the soon-to-be-replaced entry as expired, just in case there is
	// someone waiting for address resolution on it.
	entry.changeState(expired)

	*entry = linkAddrEntry{
		addr:       k,
		linkAddr:   v,
		expiration: time.Now().Add(c.ageLimit),
		wakers:     make(map[*sleep.Waker]struct{}),
		done:       make(chan struct{}),
	}

	c.cache[k] = entry
	c.next = (c.next + 1) % len(c.entries)
	return entry
}

// get reports any known link address for k.
func (c *linkAddrCache) get(
	k tcpip.FullAddress, linkRes LinkAddressResolver,
	localAddr tcpip.Address, linkEP LinkEndpoint,
	waker *sleep.Waker) (tcpip.LinkAddress, <-chan struct{}, *tcpip.Error) {

	log.Printf("link addr get linkRes: %#v, addr: %+v", linkRes, k)
	if linkRes != nil {
		if addr, ok := linkRes.ResolveStaticAddress(k.Addr); ok {
			return addr, nil, nil
		}
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	// 尝试从缓存中得到MAC地址
	if entry, ok := c.cache[k]; ok {
		switch s := entry.state(); s {
		case expired:
		case ready:
			return entry.linkAddr, nil, nil
		case failed:
			return "", nil, tcpip.ErrNoLinkAddress
		case incomplete:
			// 正在解析地址
			entry.addWaker(waker)
			return "", entry.done, tcpip.ErrWouldBlock
		default:
			panic(fmt.Sprintf("invalid cache entry state: %s", s))
		}
	}

	if linkRes == nil {
		return "", nil, tcpip.ErrNoLinkAddress
	}

	// Add 'incomplete' entry in the cache to mark that resolution is in progress.
	e := c.makeAndAddEntry(k, "")
	e.addWaker(waker)

	go c.startAddressResolution(k, linkRes, localAddr, linkEP, e.done)

	return "", e.done, tcpip.ErrWouldBlock
}

// removeWaker removes a waker previously added through get().
func (c *linkAddrCache) removeWaker(k tcpip.FullAddress, waker *sleep.Waker) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if entry, ok := c.cache[k]; ok {
		entry.removeWaker(waker)
	}
}

func (c *linkAddrCache) startAddressResolution(
	k tcpip.FullAddress, linkRes LinkAddressResolver,
	localAddr tcpip.Address, linkEP LinkEndpoint, done <-chan struct{}) {

	for i := 0; ; i++ {
		// Send link request, then wait for the timeout limit and check
		// whether the request succeeded.
		linkRes.LinkAddressRequest(k.Addr, localAddr, linkEP)

		select {
		case <-time.After(c.resolutionTimeout):
			if stop := c.checkLinkRequest(k, i); stop {
				return
			}
		case <-done:
			return
		}
	}
}

// checkLinkRequest checks whether previous attempt to resolve address has succeeded
// and mark the entry accordingly, e.g. ready, failed, etc. Return true if request
// can stop, false if another request should be sent.
func (c *linkAddrCache) checkLinkRequest(k tcpip.FullAddress, attempt int) bool {
	c.mu.Lock()
	defer c.mu.Unlock()

	entry, ok := c.cache[k]
	if !ok {
		// Entry was evicted from the cache.
		return true
	}

	switch s := entry.state(); s {
	case ready, failed, expired:
		// Entry was made ready by resolver or failed. Either way we're done.
		return true
	case incomplete:
		if attempt+1 >= c.resolutionAttempts {
			// Max number of retries reached, mark entry as failed.
			entry.changeState(failed)
			return true
		}
		// No response yet, need to send another ARP request.
		return false
	default:
		panic(fmt.Sprintf("invalid cache entry state: %s", s))
	}
}
