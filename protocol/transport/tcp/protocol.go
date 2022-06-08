package tcp

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	"github.com/qxcheng/net-protocol/pkg/waiter"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"github.com/qxcheng/net-protocol/protocol/header"
	"github.com/qxcheng/net-protocol/protocol/seqnum"
	"github.com/qxcheng/net-protocol/protocol/stack"
	"strings"
	"sync"
)

func init() {
	// tcp协议在协议栈中的注册
	stack.RegisterTransportProtocolFactory(ProtocolName, func() stack.TransportProtocol {
		return &protocol{
			sendBufferSize:             SendBufferSizeOption{minBufferSize, DefaultBufferSize, maxBufferSize},
			recvBufferSize:             ReceiveBufferSizeOption{minBufferSize, DefaultBufferSize, maxBufferSize},
			congestionControl:          ccReno,
			availableCongestionControl: []string{ccReno, ccCubic},
		}
	})
}

const (
	ProtocolName = "tcp"
	ProtocolNumber = header.TCPProtocolNumber
	minBufferSize = 4 << 10     // 4096 bytes. 接收或发送缓存的最小值
	DefaultBufferSize = 1 << 20 // 1MB 接收或发送缓存的默认值
	maxBufferSize = 4 << 20     // 4MB 接收或发送缓存可以扩展到的最大值
)

type SACKEnabled bool                // 开启SACK功能

type SendBufferSizeOption struct {
	Min     int
	Default int
	Max     int
}

type ReceiveBufferSizeOption struct {
	Min     int
	Default int
	Max     int
}

const (
	ccReno  = "reno"
	ccCubic = "cubic"
)

type CongestionControlOption string  // 设置当前的拥塞控制算法
type AvailableCongestionControlOption string  // 返回支持的拥塞控制算法

// tcp协议传输层接口的实现
type protocol struct {
	mu                         sync.Mutex
	sackEnabled                bool
	sendBufferSize             SendBufferSizeOption
	recvBufferSize             ReceiveBufferSizeOption
	congestionControl          string
	availableCongestionControl []string
	allowedCongestionControl   []string
}

func (*protocol) NewEndpoint(stack *stack.Stack, netProto tcpip.NetworkProtocolNumber, waiterQueue *waiter.Queue) (tcpip.Endpoint, *tcpip.Error) {
	return newEndpoint(stack, netProto, waiterQueue), nil
}

func (*protocol) Number() tcpip.TransportProtocolNumber {
	return ProtocolNumber
}

func (*protocol) MinimumPacketSize() int {
	return header.TCPMinimumSize
}

func (*protocol) ParsePorts(v buffer.View) (src, dst uint16, err *tcpip.Error) {
	h := header.TCP(v)
	return h.SourcePort(), h.DestinationPort(), nil
}

// HandleUnknownDestinationPacket handles packets targeted at this protocol but
// that don't match any existing endpoint.
func (*protocol) HandleUnknownDestinationPacket(r *stack.Route, id stack.TransportEndpointID, vv buffer.VectorisedView) bool {
	s := newSegment(r, id, vv)
	defer s.decRef()

	if s.parse() {
		return false
	}

	// There's nothing to do if this is already a reset packet.
	if s.flagIsSet(flagRst) {
		return true
	}

	replyWithReset(s)
	return true
}

// replyWithReset replies to the given segment with a reset segment.
func replyWithReset(s *segment) {
	// Get the seqnum from the packet if the ack flag is set.
	seq := seqnum.Value(0)
	if s.flagIsSet(flagAck) {
		seq = s.ackNumber
	}

	ack := s.sequenceNumber.Add(s.logicalLen())

	sendTCP(&s.route, s.id, buffer.VectorisedView{}, s.route.DefaultTTL(), flagRst|flagAck, seq, ack, 0, nil)
}

func (p *protocol) SetOption(option interface{}) *tcpip.Error {
	switch v := option.(type) {
	case SACKEnabled:
		p.mu.Lock()
		p.sackEnabled = bool(v)
		p.mu.Unlock()
		return nil

	case SendBufferSizeOption:
		if v.Min <= 0 || v.Default < v.Min || v.Default > v.Max {
			return tcpip.ErrInvalidOptionValue
		}
		p.mu.Lock()
		p.sendBufferSize = v
		p.mu.Unlock()
		return nil

	case ReceiveBufferSizeOption:
		if v.Min <= 0 || v.Default < v.Min || v.Default > v.Max {
			return tcpip.ErrInvalidOptionValue
		}
		p.mu.Lock()
		p.recvBufferSize = v
		p.mu.Unlock()
		return nil

	case CongestionControlOption:
		for _, c := range p.availableCongestionControl {
			if string(v) == c {
				p.mu.Lock()
				p.congestionControl = string(v)
				p.mu.Unlock()
				return nil
			}
		}
		return tcpip.ErrInvalidOptionValue
	default:
		return tcpip.ErrUnknownProtocolOption
	}
}

func (p *protocol) Option(option interface{}) *tcpip.Error {
	switch v := option.(type) {
	case *SACKEnabled:
		p.mu.Lock()
		*v = SACKEnabled(p.sackEnabled)
		p.mu.Unlock()
		return nil

	case *SendBufferSizeOption:
		p.mu.Lock()
		*v = p.sendBufferSize
		p.mu.Unlock()
		return nil

	case *ReceiveBufferSizeOption:
		p.mu.Lock()
		*v = p.recvBufferSize
		p.mu.Unlock()
		return nil
	case *CongestionControlOption:
		p.mu.Lock()
		*v = CongestionControlOption(p.congestionControl)
		p.mu.Unlock()
		return nil
	case *AvailableCongestionControlOption:
		p.mu.Lock()
		*v = AvailableCongestionControlOption(strings.Join(p.availableCongestionControl, " "))
		p.mu.Unlock()
		return nil
	default:
		return tcpip.ErrUnknownProtocolOption
	}
}






