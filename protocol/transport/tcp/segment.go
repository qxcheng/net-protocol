package tcp

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	"github.com/qxcheng/net-protocol/protocol/header"
	"github.com/qxcheng/net-protocol/protocol/seqnum"
	"github.com/qxcheng/net-protocol/protocol/stack"
	"strings"
	"sync/atomic"
)

// Flags that may be set in a TCP segment.
const (
	flagFin = 1 << iota
	flagSyn
	flagRst
	flagPsh
	flagAck
	flagUrg
)

func flagString(flags uint8) string {
	var s []string
	if (flags & flagAck) != 0 {
		s = append(s, "ack")
	}
	if (flags & flagFin) != 0 {
		s = append(s, "fin")
	}
	if (flags & flagPsh) != 0 {
		s = append(s, "psh")
	}
	if (flags & flagRst) != 0 {
		s = append(s, "rst")
	}
	if (flags & flagSyn) != 0 {
		s = append(s, "syn")
	}
	if (flags & flagUrg) != 0 {
		s = append(s, "urg")
	}
	return strings.Join(s, "|")
}

type segment struct {
	segmentEntry
	refCnt int32
	id     stack.TransportEndpointID
	route  stack.Route
	data   buffer.VectorisedView
	// views is used as buffer for data when its length is large
	// enough to store a VectorisedView.
	views [8]buffer.View
	// viewToDeliver keeps track of the next View that should be
	// delivered by the Read endpoint.
	viewToDeliver  int
	sequenceNumber seqnum.Value
	ackNumber      seqnum.Value
	flags          uint8
	window         seqnum.Size

	// parsedOptions stores the parsed values from the options in the segment.
	parsedOptions header.TCPOptions
	options       []byte
}

func newSegment(r *stack.Route, id stack.TransportEndpointID, vv buffer.VectorisedView) *segment {
	s := &segment{
		refCnt: 1,
		id:     id,
		route:  r.Clone(),
	}
	s.data = vv.Clone(s.views[:])
	return s
}

func newSegmentFromView(r *stack.Route, id stack.TransportEndpointID, v buffer.View) *segment {
	s := &segment{
		refCnt: 1,
		id:     id,
		route:  r.Clone(),
	}
	s.views[0] = v
	s.data = buffer.NewVectorisedView(len(v), s.views[:1])
	return s
}

func (s *segment) clone() *segment {
	t := &segment{
		refCnt:         1,
		id:             s.id,
		sequenceNumber: s.sequenceNumber,
		ackNumber:      s.ackNumber,
		flags:          s.flags,
		window:         s.window,
		route:          s.route.Clone(),
		viewToDeliver:  s.viewToDeliver,
	}
	t.data = s.data.Clone(t.views[:])
	return t
}

func (s *segment) flagIsSet(flag uint8) bool {
	return (s.flags & flag) != 0
}

func (s *segment) decRef() {
	if atomic.AddInt32(&s.refCnt, -1) == 0 {
		s.route.Release()
	}
}

func (s *segment) incRef() {
	atomic.AddInt32(&s.refCnt, 1)
}

// logicalLen 计算tcp段的逻辑长度，包括负载数据的长度，如果有控制标记，需要加1
func (s *segment) logicalLen() seqnum.Size {
	l := seqnum.Size(s.data.Size())
	if s.flagIsSet(flagSyn) {
		l++
	}
	if s.flagIsSet(flagFin) {
		l++
	}
	return l
}

// parse 解析存储在数据中的TCP头，填充tcp段的序列和确认号，标志和窗口字段。
// 返回 bool值，指示解析是否成功。
func (s *segment) parse() bool {
	h := header.TCP(s.data.First())

	offset := int(h.DataOffset())
	if offset < header.TCPMinimumSize || offset > len(h) {
		return false
	}

	s.options = []byte(h[header.TCPMinimumSize:offset])
	s.parsedOptions = header.ParseTCPOptions(s.options)
	s.data.TrimFront(offset)

	s.sequenceNumber = seqnum.Value(h.SequenceNumber())
	s.ackNumber = seqnum.Value(h.AckNumber())
	s.flags = h.Flags()
	s.window = seqnum.Size(h.WindowSize())

	return true
}