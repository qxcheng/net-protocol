package tcpip

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	"github.com/qxcheng/net-protocol/pkg/waiter"
)

// Error 自定义错误相关 ///////////////
type Error struct {
	msg string
	ignoreStats bool
}

func (e *Error) String() string {
	return e.msg
}

func (e *Error) IgnoreStats() bool {
	return e.ignoreStats
}

var (
	ErrUnknownProtocol       = &Error{msg: "unknown protocol"}
	ErrUnknownNICID          = &Error{msg: "unknown nic id"}
	ErrUnknownProtocolOption = &Error{msg: "unknown option for protocol"}
	ErrDuplicateNICID        = &Error{msg: "duplicate nic id"}
	ErrDuplicateAddress      = &Error{msg: "duplicate address"}
	ErrNoRoute               = &Error{msg: "no route"}
	ErrBadLinkEndpoint       = &Error{msg: "bad link layer endpoint"}
	ErrAlreadyBound          = &Error{msg: "endpoint already bound", ignoreStats: true}
	ErrInvalidEndpointState  = &Error{msg: "endpoint is in invalid state"}
	ErrAlreadyConnecting     = &Error{msg: "endpoint is already connecting", ignoreStats: true}
	ErrAlreadyConnected      = &Error{msg: "endpoint is already connected", ignoreStats: true}
	ErrNoPortAvailable       = &Error{msg: "no ports are available"}
	ErrPortInUse             = &Error{msg: "port is in use"}
	ErrBadLocalAddress       = &Error{msg: "bad local address"}
	ErrClosedForSend         = &Error{msg: "endpoint is closed for send"}
	ErrClosedForReceive      = &Error{msg: "endpoint is closed for receive"}
	ErrWouldBlock            = &Error{msg: "operation would block", ignoreStats: true}
	ErrConnectionRefused     = &Error{msg: "connection was refused"}
	ErrTimeout               = &Error{msg: "operation timed out"}
	ErrAborted               = &Error{msg: "operation aborted"}
	ErrConnectStarted        = &Error{msg: "connection attempt started", ignoreStats: true}
	ErrDestinationRequired   = &Error{msg: "destination address is required"}
	ErrNotSupported          = &Error{msg: "operation not supported"}
	ErrQueueSizeNotSupported = &Error{msg: "queue size querying not supported"}
	ErrNotConnected          = &Error{msg: "endpoint not connected"}
	ErrConnectionReset       = &Error{msg: "connection reset by peer"}
	ErrConnectionAborted     = &Error{msg: "connection aborted"}
	ErrNoSuchFile            = &Error{msg: "no such file"}
	ErrInvalidOptionValue    = &Error{msg: "invalid option value specified"}
	ErrNoLinkAddress         = &Error{msg: "no remote link address"}
	ErrBadAddress            = &Error{msg: "bad address"}
	ErrNetworkUnreachable    = &Error{msg: "network is unreachable"}
	ErrMessageTooLong        = &Error{msg: "message too long"}
	ErrNoBufferSpace         = &Error{msg: "no buffer space available"}
)

// 工具相关 ////////////////////////////////////////////////////////////

// A Clock provides the current time.
//
// Times returned by a Clock should always be used for application-visible
// time, but never for netstack internal timekeeping.
type Clock interface {
	// NowNanoseconds returns the current real time as a number of
	// nanoseconds since the Unix epoch.
	NowNanoseconds() int64

	// NowMonotonic returns a monotonic time value.
	NowMonotonic() int64
}

// Payload provides an interface around data that is being sent to an endpoint.
// This allows the endpoint to request the amount of data it needs based on
// internal buffers without exposing them. 'p.Get(p.Size())' reads all the data.
type Payload interface {
	// Get returns a slice containing exactly 'min(size, p.Size())' bytes.
	Get(size int) ([]byte, *Error)

	// Size returns the payload size.
	Size() int
}

// Options 相关

// WriteOptions contains options for Endpoint.Write.
type WriteOptions struct {
	// If To is not nil, write to the given address instead of the endpoint's
	// peer.
	To *FullAddress

	// More has the same semantics as Linux's MSG_MORE.
	More bool

	// EndOfRecord has the same semantics as Linux's MSG_EOR.
	EndOfRecord bool
}

// 链路层 //////////////////////////////////////////////////////////////

type LinkAddress string
type LinkEndpointID uint64  // 链路层端点标识
type NICID int32  // NIC网卡唯一标识

// FullAddress 代表完整的传输层节点地址，用于 Connect() and Bind()
type FullAddress struct {
	NIC NICID  // This may not be used by all endpoint types.
	Addr Address  // 网络层地址
	Port uint16  // 传输层端口  This may not be used by all endpoint types.
}


// 网络层 /////////////////////////////////////////////////////////////

type NetworkProtocolNumber uint32
type Address string  // 网络层地址
type AddressMask string  // 网络层地址的子网掩码
// Subnet is a subnet defined by its address and mask.
type Subnet struct {
	address Address
	mask    AddressMask
}

// Route is a row in the routing table. It specifies through which NIC (and
// gateway) sets of packets should be routed. A row is considered viable if the
// masked target address matches the destination adddress in the row.
type Route struct {
	// Destination is the address that must be matched against the masked
	// target address to check if this row is viable.
	Destination Address

	// Mask specifies which bits of the Destination and the target address
	// must match for this row to be viable.
	Mask AddressMask

	// Gateway is the gateway to be used if this row is viable.
	Gateway Address

	// NIC is the id of the nic to be used if this row is viable.
	NIC NICID
}

// A ControlMessages contains socket control messages for IP sockets.
//
// +stateify savable
type ControlMessages struct {
	// HasTimestamp indicates whether Timestamp is valid/set.
	HasTimestamp bool

	// Timestamp is the time (in ns) that the last packed used to create
	// the read data was received.
	Timestamp int64
}


// 传输层 ////////////////////////////////////////////////////////////

type TransportProtocolNumber uint32  // 传输层协议号

// ShutdownFlags represents flags that can be passed to the Shutdown() method
// of the Endpoint interface.
type ShutdownFlags int

// Endpoint 由传输层协议实现 (e.g., tcp, udp)，向用户暴露网络栈的read, write, connect方法等
type Endpoint interface {
	// Close puts the endpoint in a closed state and frees all resources
	// associated with it.
	Close()

	// Read reads data from the endpoint and optionally returns the sender.
	//
	// This method does not block if there is no data pending. It will also
	// either return an error or data, never both.
	//
	// A timestamp (in ns) is optionally returned. A zero value indicates
	// that no timestamp was available.
	Read(*FullAddress) (buffer.View, ControlMessages, *Error)

	// Write writes data to the endpoint's peer. This method does not block if
	// the data cannot be written.
	//
	// Unlike io.Writer.Write, Endpoint.Write transfers ownership of any bytes
	// successfully written to the Endpoint. That is, if a call to
	// Write(SlicePayload{data}) returns (n, err), it may retain data[:n], and
	// the caller should not use data[:n] after Write returns.
	//
	// Note that unlike io.Writer.Write, it is not an error for Write to
	// perform a partial write.
	//
	// For UDP and Ping sockets if address resolution is required,
	// ErrNoLinkAddress and a notification channel is returned for the caller to
	// block. Channel is closed once address resolution is complete (success or
	// not). The channel is only non-nil in this case.
	Write(Payload, WriteOptions) (uintptr, <-chan struct{}, *Error)

	// Peek reads data without consuming it from the endpoint.
	//
	// This method does not block if there is no data pending.
	//
	// A timestamp (in ns) is optionally returned. A zero value indicates
	// that no timestamp was available.
	Peek([][]byte) (uintptr, ControlMessages, *Error)

	// Connect connects the endpoint to its peer. Specifying a NIC is
	// optional.
	//
	// There are three classes of return values:
	//	nil -- the attempt to connect succeeded.
	//	ErrConnectStarted/ErrAlreadyConnecting -- the connect attempt started
	//		but hasn't completed yet. In this case, the caller must call Connect
	//		or GetSockOpt(ErrorOption) when the endpoint becomes writable to
	//		get the actual result. The first call to Connect after the socket has
	//		connected returns nil. Calling connect again results in ErrAlreadyConnected.
	//	Anything else -- the attempt to connect failed.
	Connect(address FullAddress) *Error

	// Shutdown closes the read and/or write end of the endpoint connection
	// to its peer.
	Shutdown(flags ShutdownFlags) *Error

	// Listen puts the endpoint in "listen" mode, which allows it to accept
	// new connections.
	Listen(backlog int) *Error

	// Accept returns a new endpoint if a peer has established a connection
	// to an endpoint previously set to listen mode. This method does not
	// block if no new connections are available.
	//
	// The returned Queue is the wait queue for the newly created endpoint.
	Accept() (Endpoint, *waiter.Queue, *Error)

	// Bind binds the endpoint to a specific local address and port.
	// Specifying a NIC is optional.
	//
	// An optional commit function will be executed atomically with respect
	// to binding the endpoint. If this returns an error, the bind will not
	// occur and the error will be propagated back to the caller.
	Bind(address FullAddress, commit func() *Error) *Error

	// GetLocalAddress returns the address to which the endpoint is bound.
	GetLocalAddress() (FullAddress, *Error)

	// GetRemoteAddress returns the address to which the endpoint is
	// connected.
	GetRemoteAddress() (FullAddress, *Error)

	// Readiness returns the current readiness of the endpoint. For example,
	// if waiter.EventIn is set, the endpoint is immediately readable.
	Readiness(mask waiter.EventMask) waiter.EventMask

	// SetSockOpt sets a socket option. opt should be one of the *Option types.
	SetSockOpt(opt interface{}) *Error

	// GetSockOpt gets a socket option. opt should be a pointer to one of the
	// *Option types.
	GetSockOpt(opt interface{}) *Error
}

// 统计相关 //////////////////////////////////////////////////////////////////////

// A StatCounter keeps track of a statistic.
type StatCounter struct {
	count uint64
}

// IPStats collects IP-specific stats (both v4 and v6).
type IPStats struct {
	// PacketsReceived is the total number of IP packets received from the link
	// layer in nic.DeliverNetworkPacket.
	PacketsReceived *StatCounter

	// InvalidAddressesReceived is the total number of IP packets received
	// with an unknown or invalid destination address.
	InvalidAddressesReceived *StatCounter

	// PacketsDelivered is the total number of incoming IP packets that
	// are successfully delivered to the transport layer via HandlePacket.
	PacketsDelivered *StatCounter

	// PacketsSent is the total number of IP packets sent via WritePacket.
	PacketsSent *StatCounter

	// OutgoingPacketErrors is the total number of IP packets which failed
	// to write to a link-layer endpoint.
	OutgoingPacketErrors *StatCounter
}

// TCPStats collects TCP-specific stats.
type TCPStats struct {
	// ActiveConnectionOpenings is the number of connections opened successfully
	// via Connect.
	ActiveConnectionOpenings *StatCounter

	// PassiveConnectionOpenings is the number of connections opened
	// successfully via Listen.
	PassiveConnectionOpenings *StatCounter

	// FailedConnectionAttempts is the number of calls to Connect or Listen
	// (active and passive openings, respectively) that end in an error.
	FailedConnectionAttempts *StatCounter

	// ValidSegmentsReceived is the number of TCP segments received that the
	// transport layer successfully parsed.
	ValidSegmentsReceived *StatCounter

	// InvalidSegmentsReceived is the number of TCP segments received that
	// the transport layer could not parse.
	InvalidSegmentsReceived *StatCounter

	// SegmentsSent is the number of TCP segments sent.
	SegmentsSent *StatCounter

	// ResetsSent is the number of TCP resets sent.
	ResetsSent *StatCounter

	// ResetsReceived is the number of TCP resets received.
	ResetsReceived *StatCounter
}

// UDPStats collects UDP-specific stats.
type UDPStats struct {
	// PacketsReceived is the number of UDP datagrams received via
	// HandlePacket.
	PacketsReceived *StatCounter

	// UnknownPortErrors is the number of incoming UDP datagrams dropped
	// because they did not have a known destination port.
	UnknownPortErrors *StatCounter

	// ReceiveBufferErrors is the number of incoming UDP datagrams dropped
	// due to the receiving buffer being in an invalid state.
	ReceiveBufferErrors *StatCounter

	// MalformedPacketsReceived is the number of incoming UDP datagrams
	// dropped due to the UDP header being in a malformed state.
	MalformedPacketsReceived *StatCounter

	// PacketsSent is the number of UDP datagrams sent via sendUDP.
	PacketsSent *StatCounter
}

// Stats 网络栈的统计数据，所有字段都是可选的
type Stats struct {
	// UnknownProtocolRcvdPackets is the number of packets received by the
	// stack that were for an unknown or unsupported protocol.
	UnknownProtocolRcvdPackets *StatCounter

	// MalformedRcvPackets is the number of packets received by the stack
	// that were deemed malformed.
	MalformedRcvdPackets *StatCounter

	// DroppedPackets is the number of packets dropped due to full queues.
	DroppedPackets *StatCounter

	// IP breaks out IP-specific stats (both v4 and v6).
	IP IPStats

	// TCP breaks out TCP-specific stats.
	TCP TCPStats

	// UDP breaks out UDP-specific stats.
	UDP UDPStats
}