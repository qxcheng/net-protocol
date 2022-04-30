package fragmentation

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	"log"
	"sync"
	"time"
)

const DefaultReassembleTimeout = 30 * time.Second  // 默认的重组时间

const HighFragThreshold = 4 << 20  // 4MB 开始丢弃旧分片包的阈值
const LowFragThreshold = 3 << 20   // 3MB 丢弃旧分片包需要减小到的阈值


type Fragmentation struct {
	mu           sync.Mutex
	highLimit    int
	lowLimit     int
	reassemblers map[uint32]*reassembler
	rList        reassemblerList
	size         int
	timeout      time.Duration
}

func NewFragmentation(highMemoryLimit, lowMemoryLimit int, reassemblingTimeout time.Duration) *Fragmentation {
	if lowMemoryLimit >= highMemoryLimit {
		lowMemoryLimit = highMemoryLimit
	}

	if lowMemoryLimit < 0 {
		lowMemoryLimit = 0
	}

	return &Fragmentation{
		reassemblers: make(map[uint32]*reassembler),
		highLimit:    highMemoryLimit,
		lowLimit:     lowMemoryLimit,
		timeout:      reassemblingTimeout,
	}
}

// Process 完成分片重组功能
func (f *Fragmentation) Process(id uint32, first, last uint16,
	more bool, vv buffer.VectorisedView) (buffer.VectorisedView, bool) {

	f.mu.Lock()
	r, ok := f.reassemblers[id]
	// 如果已过期
	if ok && r.tooOld(f.timeout) {
		f.release(r)
		ok = false
	}
	if !ok {
		r = newReassembler(id)
		f.reassemblers[id] = r
		f.rList.PushFront(r)
	}
	f.mu.Unlock()

	res, done, consumed := r.process(first, last, more, vv)

	f.mu.Lock()
	f.size += consumed
	if done {
		f.release(r)
	}
	// 开始缓存淘汰
	if f.size > f.highLimit {
		tail := f.rList.Back()
		for f.size > f.lowLimit && tail != nil {
			f.release(tail)
			tail = tail.Prev()
		}
	}
	f.mu.Unlock()
	return res, done
}

func (f *Fragmentation) release(r *reassembler) {
	if r.checkDoneOrMark() {
		return
	}

	delete(f.reassemblers, r.id)
	f.rList.Remove(r)
	f.size -= r.size
	if f.size < 0 {
		log.Printf("memory counter < 0 (%d), this is an accounting bug that requires investigation", f.size)
		f.size = 0
	}
}
