package fragmentation

import (
	"container/heap"
	"fmt"
	"github.com/qxcheng/net-protocol/pkg/buffer"
	"math"
	"sync"
	"time"
)

type hole struct {
	first   uint16
	last    uint16
	deleted bool
}

type reassembler struct {
	reassemblerEntry
	id           uint32
	size         int
	mu           sync.Mutex
	holes        []hole
	deleted      int
	heap         fragHeap
	done         bool
	creationTime time.Time
}

func newReassembler(id uint32) *reassembler {
	r := &reassembler{
		id:           id,
		holes:        make([]hole, 0, 16),
		deleted:      0,
		heap:         make(fragHeap, 0, 8),
		creationTime: time.Now(),
	}
	r.holes = append(r.holes, hole{
		first:   0,
		last:    math.MaxUint16,
		deleted: false})
	return r
}

func (r *reassembler) updateHoles(first, last uint16, more bool) bool {
	used := false
	for i := range r.holes {
		if r.holes[i].deleted || first > r.holes[i].last || last < r.holes[i].first {
			continue
		}
		used = true
		r.deleted++
		r.holes[i].deleted = true
		if first > r.holes[i].first {
			r.holes = append(r.holes, hole{r.holes[i].first, first-1, false})
		}
		if last < r.holes[i].last && more {
			r.holes = append(r.holes, hole{last+1, r.holes[i].last, false})
		}
	}
	return used
}

func (r *reassembler) process(first, last uint16, more bool,
	vv buffer.VectorisedView) (buffer.VectorisedView, bool, int) {

	r.mu.Lock()
	defer r.mu.Unlock()
	consumed := 0
	if r.done {
		return buffer.VectorisedView{}, false, consumed
	}
	if r.updateHoles(first, last, more) {
		heap.Push(&r.heap, fragment{offset: first, vv: vv.Clone(nil)})
		consumed = vv.Size()
		r.size += consumed
	}
	if r.deleted < len(r.holes) {
		return buffer.VectorisedView{}, false, consumed
	}
	res, err := r.heap.reassemble()
	if err != nil {
		panic(fmt.Sprintf("reassemble failed with: %v. There is probably a bug in the code handling the holes.", err))
	}
	return res, true, consumed
}

func (r *reassembler) tooOld(timeout time.Duration) bool {
	return time.Now().Sub(r.creationTime) > timeout
}

func (r *reassembler) checkDoneOrMark() bool {
	r.mu.Lock()
	prev := r.done
	r.done = true
	r.mu.Unlock()
	return prev
}