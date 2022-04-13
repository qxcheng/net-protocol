package stack

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	"github.com/qxcheng/net-protocol/pkg/sleep"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"github.com/qxcheng/net-protocol/protocol/header"
	"github.com/qxcheng/net-protocol/protocol/ports"
	"github.com/qxcheng/net-protocol/protocol/seqnum"
	"sync"
	"time"
)

type transportProtocolState struct {
	proto          TransportProtocol
	defaultHandler func(*Route, TransportEndpointID, buffer.VectorisedView) bool
}

// TCPProbeFunc is the expected function type for a TCP probe function to be
// passed to stack.AddTCPProbe.
type TCPProbeFunc func(s TCPEndpointState)

// TCPCubicState is used to hold a copy of the internal cubic state when the
// TCPProbeFunc is invoked.
type TCPCubicState struct {
	WLastMax                float64
	WMax                    float64
	T                       time.Time
	TimeSinceLastCongestion time.Duration
	C                       float64
	K                       float64
	Beta                    float64
	WC                      float64
	WEst                    float64
}

// TCPEndpointID is the unique 4 tuple that identifies a given endpoint.
type TCPEndpointID struct {
	// LocalPort is the local port associated with the endpoint.
	LocalPort uint16

	// LocalAddress is the local [network layer] address associated with
	// the endpoint.
	LocalAddress tcpip.Address

	// RemotePort is the remote port associated with the endpoint.
	RemotePort uint16

	// RemoteAddress it the remote [network layer] address associated with
	// the endpoint.
	RemoteAddress tcpip.Address
}

// TCPFastRecoveryState holds a copy of the internal fast recovery state of a
// TCP endpoint.
type TCPFastRecoveryState struct {
	// Active if true indicates the endpoint is in fast recovery.
	Active bool

	// First is the first unacknowledged sequence number being recovered.
	First seqnum.Value

	// Last is the 'recover' sequence number that indicates the point at
	// which we should exit recovery barring any timeouts etc.
	Last seqnum.Value

	// MaxCwnd is the maximum value we are permitted to grow the congestion
	// window during recovery. This is set at the time we enter recovery.
	MaxCwnd int
}

// TCPReceiverState holds a copy of the internal state of the receiver for
// a given TCP endpoint.
type TCPReceiverState struct {
	// RcvNxt is the TCP variable RCV.NXT.
	RcvNxt seqnum.Value

	// RcvAcc is the TCP variable RCV.ACC.
	RcvAcc seqnum.Value

	// RcvWndScale is the window scaling to use for inbound segments.
	RcvWndScale uint8

	// PendingBufUsed is the number of bytes pending in the receive
	// queue.
	PendingBufUsed seqnum.Size

	// PendingBufSize is the size of the socket receive buffer.
	PendingBufSize seqnum.Size
}

// TCPSenderState holds a copy of the internal state of the sender for
// a given TCP Endpoint.
type TCPSenderState struct {
	// LastSendTime is the time at which we sent the last segment.
	LastSendTime time.Time

	// DupAckCount is the number of Duplicate ACK's received.
	DupAckCount int

	// SndCwnd is the size of the sending congestion window in packets.
	SndCwnd int

	// Ssthresh is the slow start threshold in packets.
	Ssthresh int

	// SndCAAckCount is the number of packets consumed in congestion
	// avoidance mode.
	SndCAAckCount int

	// Outstanding is the number of packets in flight.
	Outstanding int

	// SndWnd is the send window size in bytes.
	SndWnd seqnum.Size

	// SndUna is the next unacknowledged sequence number.
	SndUna seqnum.Value

	// SndNxt is the sequence number of the next segment to be sent.
	SndNxt seqnum.Value

	// RTTMeasureSeqNum is the sequence number being used for the latest RTT
	// measurement.
	RTTMeasureSeqNum seqnum.Value

	// RTTMeasureTime is the time when the RTTMeasureSeqNum was sent.
	RTTMeasureTime time.Time

	// Closed indicates that the caller has closed the endpoint for sending.
	Closed bool

	// SRTT is the smoothed round-trip time as defined in section 2 of
	// RFC 6298.
	SRTT time.Duration

	// RTO is the retransmit timeout as defined in section of 2 of RFC 6298.
	RTO time.Duration

	// RTTVar is the round-trip time variation as defined in section 2 of
	// RFC 6298.
	RTTVar time.Duration

	// SRTTInited if true indicates take a valid RTT measurement has been
	// completed.
	SRTTInited bool

	// MaxPayloadSize is the maximum size of the payload of a given segment.
	// It is initialized on demand.
	MaxPayloadSize int

	// SndWndScale is the number of bits to shift left when reading the send
	// window size from a segment.
	SndWndScale uint8

	// MaxSentAck is the highest acknowledgement number sent till now.
	MaxSentAck seqnum.Value

	// FastRecovery holds the fast recovery state for the endpoint.
	FastRecovery TCPFastRecoveryState

	// Cubic holds the state related to CUBIC congestion control.
	Cubic TCPCubicState
}

// TCPSACKInfo holds TCP SACK related information for a given TCP endpoint.
type TCPSACKInfo struct {
	// Blocks is the list of SACK block currently received by the
	// TCP endpoint.
	Blocks []header.SACKBlock
}

// TCPEndpointState is a copy of the internal state of a TCP endpoint.
type TCPEndpointState struct {
	// ID is a copy of the TransportEndpointID for the endpoint.
	ID TCPEndpointID

	// SegTime denotes the absolute time when this segment was received.
	SegTime time.Time

	// RcvBufSize is the size of the receive socket buffer for the endpoint.
	RcvBufSize int

	// RcvBufUsed is the amount of bytes actually held in the receive socket
	// buffer for the endpoint.
	RcvBufUsed int

	// RcvClosed if true, indicates the endpoint has been closed for reading.
	RcvClosed bool

	// SendTSOk is used to indicate when the TS Option has been negotiated.
	// When sendTSOk is true every non-RST segment should carry a TS as per
	// RFC7323#section-1.1.
	SendTSOk bool

	// RecentTS is the timestamp that should be sent in the TSEcr field of
	// the timestamp for future segments sent by the endpoint. This field is
	// updated if required when a new segment is received by this endpoint.
	RecentTS uint32

	// TSOffset is a randomized offset added to the value of the TSVal field
	// in the timestamp option.
	TSOffset uint32

	// SACKPermitted is set to true if the peer sends the TCPSACKPermitted
	// option in the SYN/SYN-ACK.
	SACKPermitted bool

	// SACK holds TCP SACK related information for this endpoint.
	SACK TCPSACKInfo

	// SndBufSize is the size of the socket send buffer.
	SndBufSize int

	// SndBufUsed is the number of bytes held in the socket send buffer.
	SndBufUsed int

	// SndClosed indicates that the endpoint has been closed for sends.
	SndClosed bool

	// SndBufInQueue is the number of bytes in the send queue.
	SndBufInQueue seqnum.Size

	// PacketTooBigCount is used to notify the main protocol routine how
	// many times a "packet too big" control packet is received.
	PacketTooBigCount int

	// SndMTU is the smallest MTU seen in the control packets received.
	SndMTU int

	// Receiver holds variables related to the TCP receiver for the endpoint.
	Receiver TCPReceiverState

	// Sender holds state related to the TCP Sender for the endpoint.
	Sender TCPSenderState
}

// Stack 一个协议栈, with all supported protocols, NICs, and route table.
type Stack struct {
	transportProtocols map[tcpip.TransportProtocolNumber]*transportProtocolState
	networkProtocols   map[tcpip.NetworkProtocolNumber]NetworkProtocol
	linkAddrResolvers  map[tcpip.NetworkProtocolNumber]LinkAddressResolver
	demux *transportDemuxer
	stats tcpip.Stats
	linkAddrCache *linkAddrCache
	mu         sync.RWMutex
	nics       map[tcpip.NICID]*NIC
	forwarding bool

	// route is the route table passed in by the user via SetRouteTable(),
	// it is used by FindRoute() to build a route for a specific
	// destination.
	routeTable []tcpip.Route

	*ports.PortManager

	// If not nil, then any new endpoints will have this probe function
	// invoked everytime they receive a TCP segment.
	tcpProbeFunc TCPProbeFunc

	// clock is used to generate user-visible times.
	clock tcpip.Clock
}

// Forwarding returns if the packet forwarding between NICs is enabled.
func (s *Stack) Forwarding() bool {
	// TODO: Expose via /proc/sys/net/ipv4/ip_forward.
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.forwarding
}

// FindRoute 路由查找实现，比如当tcp建立连接时，会用该函数得到路由信息
func (s *Stack) FindRoute(id tcpip.NICID, localAddr, remoteAddr tcpip.Address, netProto tcpip.NetworkProtocolNumber) (Route, *tcpip.Error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	// 遍历路由表
	for i := range s.routeTable {
		if (id != 0 && id != s.routeTable[i].NIC) || (len(remoteAddr) != 0 && !s.routeTable[i].Match(remoteAddr)) {
			continue
		}

		nic := s.nics[s.routeTable[i].NIC]
		if nic == nil {
			continue
		}

		var ref *referencedNetworkEndpoint
		if len(localAddr) != 0 {
			ref = nic.findEndpoint(netProto, localAddr, CanBePrimaryEndpoint)
		} else {
			ref = nic.primaryEndpoint(netProto)
		}
		if ref == nil {
			continue
		}

		if len(remoteAddr) == 0 {
			// If no remote address was provided, then the route
			// provided will refer to the link local address.
			remoteAddr = ref.ep.ID().LocalAddress
		}

		r := makeRoute(netProto, ref.ep.ID().LocalAddress, remoteAddr, nic.linkEP.LinkAddress(), ref)
		r.NextHop = s.routeTable[i].Gateway
		return r, nil
	}
	return Route{}, tcpip.ErrNoRoute
}



// 网卡管理相关 //////////////////////////////////////////////////////////////////////////

// 新建一个网卡对象，并且激活它，激活的意思就是准备好从网卡中读取和写入数据。
func (s *Stack) createNIC(id tcpip.NICID, name string, linkEP tcpip.LinkEndpointID, enabled bool) *tcpip.Error  {
	ep := FindLinkEndpoint(linkEP)
	if ep == nil {
		return tcpip.ErrBadLinkEndpoint
	}

	s.mu.Lock()
	defer s.mu.Unlock()

	// 保证网卡ID唯一
	if  _, ok := s.nics[id]; ok {
		return tcpip.ErrDuplicateNICID
	}

	n := newNIC(s, id, name, ep)

	s.nics[id] = n
	if enabled {
		n.attachLinkEndpoint()
	}

	return nil
}

// CreateNIC 根据nic id和linkEP id来创建和注册一个网卡对象
func (s *Stack) CreateNIC(id tcpip.NICID, linkEP tcpip.LinkEndpointID) *tcpip.Error {
	return s.createNIC(id, "", linkEP, true)
}

// LinkAddressCache 接口实现 ///////////////////////////////////////////////////////////

// CheckLocalAddress 检查本地ip地址是否存在，存在返回网卡id，否则返回0
func (s *Stack) CheckLocalAddress(nicid tcpip.NICID, protocol tcpip.NetworkProtocolNumber, addr tcpip.Address) tcpip.NICID {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if nicid != 0 {
		nic := s.nics[nicid]
		if nic == nil {
			return 0
		}

		ref := nic.findEndpoint(protocol, addr, CanBePrimaryEndpoint)
		if ref == nil {
			return 0
		}
		ref.decRef()  // 因为findEndpoint时会增加引用计数

		return nic.id
	}

	// 遍历所有NICs.
	for _, nic := range s.nics {
		ref := nic.findEndpoint(protocol, addr, CanBePrimaryEndpoint)
		if ref != nil {
			ref.decRef()
			return nic.id
		}
	}

	return 0
}

// AddLinkAddress adds a link address to the stack link cache.
func (s *Stack) AddLinkAddress(nicid tcpip.NICID, addr tcpip.Address, linkAddr tcpip.LinkAddress) {
	fullAddr := tcpip.FullAddress{NIC:  nicid, Addr: addr}
	s.linkAddrCache.add(fullAddr, linkAddr)
	// TODO: provide a way for a transport endpoint to receive a signal
	// that AddLinkAddress for a particular address has been called.
}

// GetLinkAddress 获取链路层地址
func (s *Stack) GetLinkAddress(
	nicid tcpip.NICID, addr, localAddr tcpip.Address,
	protocol tcpip.NetworkProtocolNumber,
	waker *sleep.Waker) (tcpip.LinkAddress, <-chan struct{}, *tcpip.Error) {

	s.mu.RLock()
	// 获取网卡对象
	nic := s.nics[nicid]
	if nic == nil {
		s.mu.RUnlock()
		return "", nil, tcpip.ErrUnknownNICID
	}
	s.mu.RUnlock()

	fullAddr := tcpip.FullAddress{NIC: nicid, Addr: addr}
	// 根据网络层协议号找到对应的地址解析协议，
	// 如：IPV4协议会得到ARP协议
	linkRes := s.linkAddrResolvers[protocol]
	return s.linkAddrCache.get(fullAddr, linkRes, localAddr, nic.linkEP, waker)
}

// RemoveWaker implements LinkAddressCache.RemoveWaker.
func (s *Stack) RemoveWaker(nicid tcpip.NICID, addr tcpip.Address, waker *sleep.Waker) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if nic := s.nics[nicid]; nic != nil {
		fullAddr := tcpip.FullAddress{NIC:nicid, Addr:addr}
		s.linkAddrCache.removeWaker(fullAddr, waker)
	}
}