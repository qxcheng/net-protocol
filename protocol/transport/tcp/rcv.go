package tcp

import (
	"container/heap"
	"github.com/qxcheng/net-protocol/protocol/seqnum"
)

// receiver 维护接收TCP段的状态信息，并把它们转换为流式的字节
type receiver struct {
	ep *endpoint

	rcvNxt seqnum.Value  // 下一个应该接收的序列号

	// rcvAcc 最后一个可接收的序列号的下一位
	// 如果接收窗口减少，这可能与rcvNxt + rcvWnd不同;在这种情况下，我们必须减少窗口，因为我们收到更多数据而不是缩小它。
	rcvAcc seqnum.Value

	rcvWndScale uint8

	closed bool

	pendingRcvdSegments segmentHeap
	pendingBufUsed      seqnum.Size
	pendingBufSize      seqnum.Size
}

// 初始化接收器
func newReceiver(ep *endpoint, irs seqnum.Value, rcvWnd seqnum.Size, rcvWndScale uint8) *receiver {
	return &receiver{
		ep:             ep,
		rcvNxt:         irs + 1,
		rcvAcc:         irs.Add(rcvWnd + 1),
		rcvWndScale:    rcvWndScale,
		pendingBufSize: rcvWnd,
	}
}

// tcp流量控制：判断序列号 segSeq 是否在窗口內
func (r *receiver) acceptable(segSeq seqnum.Value, segLen seqnum.Size) bool {
	rcvWnd := r.rcvNxt.Size(r.rcvAcc)  // 窗口大小
	if rcvWnd == 0 {
		return segLen == 0 && segSeq == r.rcvNxt
	}
	// 序列号在窗口内 || 数据段与窗口重叠
	return segSeq.InWindow(r.rcvNxt, rcvWnd) || seqnum.Overlap(r.rcvNxt, rcvWnd, segSeq, segLen)
}

// getSendParams 在构建要发送的段时，返回 sender 所需的参数。
// 并且更新接收窗口的指标 rcvAcc
func (r *receiver) getSendParams() (rcvNxt seqnum.Value, rcvWnd seqnum.Size) {
	n := r.ep.receiveBufferAvailable()
	acc := r.rcvNxt.Add(seqnum.Size(n))
	if r.rcvAcc.LessThan(acc) {
		r.rcvAcc = acc
	}
	return r.rcvNxt, r.rcvNxt.Size(r.rcvAcc) >> r.rcvWndScale
}

// tcp流量控制：当接收窗口从零增长到非零时，调用 nonZeroWindow;在这种情况下，
// 我们可能需要发送一个 ack，以便向对端表明它可以恢复发送数据。
func (r *receiver) nonZeroWindow() {
	if (r.rcvAcc-r.rcvNxt)>>r.rcvWndScale != 0 {
		// We never got around to announcing a zero window size, so we
		// don't need to immediately announce a nonzero one.
		return
	}

	// Immediately send an ack.
	r.ep.snd.sendAck()
}

// tcp可靠性：consumeSegment 尝试使用r接收tcp段。该数据段可能刚刚收到或可能已经收到，但尚未准备好被消费。
// 如果数据段被消耗则返回true，如果由于缺少段而无法消耗，则返回false。
func (r *receiver) consumeSegment(s *segment, segSeq seqnum.Value, segLen seqnum.Size) bool {
	if segLen > 0 {
		// 我们期望接收到的序列号范围应该是 seqStart <= rcvNxt < seqEnd，
		// 如果不在这个范围内说明我们少了数据段，返回false，表示不能立马消费
		if !r.rcvNxt.InWindow(segSeq, segLen) {
			return false
		}

		// 去除已经确认过的数据
		if segSeq.LessThan(r.rcvNxt) {
			diff := segSeq.Size(r.rcvNxt)
			segLen -= diff
			segSeq.UpdateForward(diff)
			s.sequenceNumber.UpdateForward(diff)
			s.data.TrimFront(int(diff))
		}

		// 将tcp段插入接收链表，并通知应用层用数据来了
		r.ep.readyToRead(s)

	} else if segSeq != r.rcvNxt {
		return false
	}

	// 更新期望下次收到的序列号
	r.rcvNxt = segSeq.Add(segLen)

	// 修剪SACK块以删除任何涵盖已消耗序列号的SACK信息。
	TrimSACKBlockList(&r.ep.sack, r.rcvNxt)

	// 如果收到 fin 报文
	if s.flagIsSet(flagFin) {
		// 控制报文消耗一个字节的序列号，因此这边期望下次收到的序列号加1
		r.rcvNxt++

		// 收到 fin，立即回复 ack
		r.ep.snd.sendAck()

		// Tell any readers that no more data will come.
		// 标记接收器关闭
		// 触发上层应用可以读取
		r.closed = true
		r.ep.readyToRead(nil)

		// 清除未使用的等待状态的包
		first := 0
		if len(r.pendingRcvdSegments) != 0 && r.pendingRcvdSegments[0] == s {
			first = 1
		}

		for i := first; i < len(r.pendingRcvdSegments); i++ {
			r.pendingRcvdSegments[i].decRef()
		}
		r.pendingRcvdSegments = r.pendingRcvdSegments[:first]
	}

	return true
}

// handleRcvdSegment 接收tcp段，然后进行处理消费，所谓的消费就是将负载内容插入到接收队列中
func (r *receiver) handleRcvdSegment(s *segment) {
	if r.closed {
		return
	}

	segLen := seqnum.Size(s.data.Size())
	segSeq := s.sequenceNumber

	// tcp流量控制：判断该数据段的序列号是否在接收窗口内，如果不在，立即返回ack给对端。
	if !r.acceptable(segSeq, segLen) {
		r.ep.snd.sendAck()
		return
	}

	// tcp可靠性：true表示已经消费该数据段
	// 如果不是，那么进行下面的处理，插入到 pendingRcvdSegments，且进行堆排序。
	if !r.consumeSegment(s, segSeq, segLen) {
		// 如果有负载数据或者是 fin 报文，立即回复一个 ack 报文
		if segLen > 0 || s.flagIsSet(flagFin) {
			// tcp可靠性：对于乱序的tcp段，应该在等待处理段中缓存 (只在buffer够用时)
			if r.pendingBufUsed < r.pendingBufSize {
				r.pendingBufUsed += s.logicalLen()
				s.incRef()
				// 插入堆中，且进行排序
				heap.Push(&r.pendingRcvdSegments, s)
			}

			// tcp的可靠性：更新 sack 块信息
			UpdateSACKBlocks(&r.ep.sack, segSeq, segSeq.Add(segLen), r.rcvNxt)

			// 马上发送ACK报文，让对端知道可能需要重传了
			r.ep.snd.sendAck()
		}
		return
	}

	// tcp的可靠性：通过消费当前段，我们可能填补了序列号域中的间隙，所以试着去消费等待处理段。
	for !r.closed && r.pendingRcvdSegments.Len() > 0 {
		s := r.pendingRcvdSegments[0]
		segLen := seqnum.Size(s.data.Size())
		segSeq := s.sequenceNumber

		// Skip segment altogether if it has already been acknowledged.
		if !segSeq.Add(segLen-1).LessThan(r.rcvNxt) && !r.consumeSegment(s, segSeq, segLen) {
			break
		}

		// 如果该tcp段，已经被正确消费，那么中等待处理段中删除
		heap.Pop(&r.pendingRcvdSegments)
		r.pendingBufUsed -= s.logicalLen()
		s.decRef()
	}
}