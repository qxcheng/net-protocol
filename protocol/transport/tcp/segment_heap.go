package tcp

// tcp段的堆，用来暂存失序的tcp段
// 实现了堆排序
type segmentHeap []*segment

func (h segmentHeap) Len() int {
	return len(h)
}

func (h segmentHeap) Less(i, j int) bool {
	return h[i].sequenceNumber.LessThan(h[j].sequenceNumber)
}

func (h segmentHeap) Swap(i, j int) {
	h[i], h[j] = h[j], h[i]
}

// Push adds x as the last element of h.
func (h *segmentHeap) Push(x interface{}) {
	*h = append(*h, x.(*segment))
}

// Pop removes the last element of h and returns it.
func (h *segmentHeap) Pop() interface{} {
	old := *h
	n := len(old)
	x := old[n-1]
	*h = old[:n-1]
	return x
}