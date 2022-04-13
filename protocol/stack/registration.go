package stack

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	"github.com/qxcheng/net-protocol/pkg/sleep"
	"github.com/qxcheng/net-protocol/pkg/waiter"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"sync"
)


type ControlType int  // 网络控制信息
const (
	ControlPacketTooBig ControlType = iota
	ControlPortUnreachable
	ControlUnknown
)


// 传输层 ///////////////////////////////////////////////////////////////////

// TransportEndpointID 传输层协议端点的标识
type TransportEndpointID struct {
	LocalPort uint16             // 本地端口
	LocalAddress tcpip.Address   // 本地ip地址
	RemotePort uint16            // 远程端口
	RemoteAddress tcpip.Address  // 远程ip地址
}

// TransportEndpoint 由处理包的传输层协议端点实现(e.g., tcp, udp)
type TransportEndpoint interface {
	// HandlePacket 被调用当包到达此传输层端点
	HandlePacket(r *Route, id TransportEndpointID, vv buffer.VectorisedView)

	// HandleControlPacket 被调用当控制包（ICMP）到达此传输层端点
	HandleControlPacket(id TransportEndpointID, typ ControlType, extra uint32, vv buffer.VectorisedView)
}

// TransportProtocol 由（想成为网络栈一部分的）传输层协议实现的接口(e.g., tcp, udp)
type TransportProtocol interface {
	// Number 返回传输层协议号
	Number() tcpip.TransportProtocolNumber

	// NewEndpoint 创建传输层的端点
	NewEndpoint(stack *Stack, netProto tcpip.NetworkProtocolNumber, waitQueue *waiter.Queue) (tcpip.Endpoint, *tcpip.Error)

	// MinimumPacketSize 返回此传输层协议包的最小值，任何小于此值的包被此协议丢弃
	MinimumPacketSize() int

	// ParsePorts 返回此协议包的源端口和目的端口
	ParsePorts(v buffer.View) (src, dst uint16, err *tcpip.Error)

	// HandleUnknownDestinationPacket handles packets targeted at this
	// protocol but that don't match any existing endpoint. For example,
	// it is targeted at a port that have no listeners.
	//
	// The return value indicates whether the packet was well-formed (for
	// stats purposes only).
	HandleUnknownDestinationPacket(r *Route, id TransportEndpointID, vv buffer.VectorisedView) bool

	// SetOption allows enabling/disabling protocol specific features.
	// SetOption returns an error if the option is not supported or the
	// provided option value is invalid.
	SetOption(option interface{}) *tcpip.Error

	// Option allows retrieving protocol specific option values.
	// Option returns an error if the option is not supported or the
	// provided option value is invalid.
	Option(option interface{}) *tcpip.Error
}

// TransportDispatcher 将包发给合适的传输层端点
type TransportDispatcher interface {
	DeliverTransportPacket(r *Route, protocol tcpip.TransportProtocolNumber, vv buffer.VectorisedView)
	DeliverTransportControlPacket(local, remote tcpip.Address, net tcpip.NetworkProtocolNumber, trans tcpip.TransportProtocolNumber, typ ControlType, extra uint32, vv buffer.VectorisedView)
}


// 网络层 //////////////////////////////////////////////////////////////////////

// NetworkEndpointID 网络层协议的标识，目前本地地址足够标识（IPv4 和 IPv6地址大小不同）
type NetworkEndpointID struct {
	LocalAddress tcpip.Address
}

// NetworkEndpoint 是需要由网络层协议（例如，ipv4，ipv6）的端点实现的接口。
type NetworkEndpoint interface {
	DefaultTTL() uint8  // 默认的time-to-live值 (或 hop limit in ipv6)

	// MTU is the maximum transmission unit for this endpoint. This is
	// generally calculated as the MTU of the underlying data link endpoint
	// minus the network endpoint max header length.
	MTU() uint32

	// Capabilities returns the set of capabilities supported by the underlying link-layer endpoint.
	Capabilities() LinkEndpointCapabilities

	// MaxHeaderLength returns the maximum size the network (and lower
	// level layers combined) headers can have. Higher levels use this
	// information to reserve space in the front of the packets they're
	// building.
	MaxHeaderLength() uint16

	// WritePacket writes a packet to the given destination address and protocol.
	WritePacket(r *Route, hdr buffer.Prependable, payload buffer.VectorisedView, protocol tcpip.TransportProtocolNumber, ttl uint8) *tcpip.Error

	// ID 返回网络层协议端点标识
	ID() *NetworkEndpointID

	// NICID 返回该端点所属网卡的id
	NICID() tcpip.NICID

	// HandlePacket 当包到达此网络层端点时被链路层调用
	HandlePacket(r *Route, vv buffer.VectorisedView)

	// Close 将该端点从栈中移除
	Close()
}

// NetworkProtocol 由（想成为网络栈一部分的）网络层协议实现 (ipv4, ipv6)
type NetworkProtocol interface {
	Number() tcpip.NetworkProtocolNumber  // 返回网络层协议号
	MinimumPacketSize() int               // 返回包的最小值，任何小于此值的包被此协议丢弃
	ParseAddresses(v buffer.View) (src, dst tcpip.Address)  // 返回此协议包的源ip地址和目的ip地址
	// NewEndpoint 创建此协议的端点
	NewEndpoint(nicid tcpip.NICID, addr tcpip.Address, linkAddrCache LinkAddressCache, dispatcher TransportDispatcher, sender LinkEndpoint) (NetworkEndpoint, *tcpip.Error)
	// SetOption allows enabling/disabling protocol specific features.
	// SetOption returns an error if the option is not supported or the
	// provided option value is invalid.
	SetOption(option interface{}) *tcpip.Error
	// Option allows retrieving protocol specific option values.
	// Option returns an error if the option is not supported or the
	// provided option value is invalid.
	Option(option interface{}) *tcpip.Error
}

// NetworkDispatcher 将包发给合适的网络层端点
type NetworkDispatcher interface {
	DeliverNetworkPacket(linkEP LinkEndpoint, dstLinkAddr, srcLinkAddr tcpip.LinkAddress, protocol tcpip.NetworkProtocolNumber, vv buffer.VectorisedView)
}


// 链路层 //////////////////////////////////////////////////////////////////////

type LinkEndpointCapabilities uint
const (
	CapabilityChecksumOffload LinkEndpointCapabilities = 1 << iota
	CapabilityResolutionRequired
	CapabilitySaveRestore
	CapabilityDisconnectOk
	CapabilityLoopback
)

// LinkEndpoint 是由数据链路层协议（例如，以太网，环回，原始）实现的接口，并由网络层协议
// 用于通过实施者的数据链路端点发送数据包。
type LinkEndpoint interface {
	// MTU 是此端点的最大传输单位。这通常由支持物理网络决定;
	// 当这种物理网络不存在时，限制通常为64k，其中包括IP数据包的最大大小。
	MTU() uint32

	// Capabilities 返回链路层端点支持的功能集。
	Capabilities() LinkEndpointCapabilities

	// MaxHeaderLength 返回数据链接（和较低级别的图层组合）标头可以具有的最大大小。
	// 较高级别使用此信息来保留它们正在构建的数据包前面预留空间。
	MaxHeaderLength() uint16

	// LinkAddress 本地链路层地址
	LinkAddress() tcpip.LinkAddress

	// WritePacket 通过给定的路由写入具有给定协议的数据包。
	// 要参与透明桥接，LinkEndpoint实现应调用eth.Encode，并将header.EthernetFields.SrcAddr
	// 设置为r.LocalLinkAddress（如果已提供）。
	WritePacket(r *Route, hdr buffer.Prependable, payload buffer.VectorisedView, protocol tcpip.NetworkProtocolNumber) *tcpip.Error

	// Attach 将数据链路层端点附加到协议栈的网络层调度程序。
	Attach(dispatcher NetworkDispatcher)

	// IsAttached 是否已经添加了网络层调度器
	IsAttached() bool
}

// A LinkAddressResolver 可以处理链路层地址的对网络层协议的扩展
type LinkAddressResolver interface {
	// LinkAddressRequest sends a request for the LinkAddress of addr.
	// The request is sent on linkEP with localAddr as the source.
	//
	// A valid response will cause the discovery protocol's network
	// endpoint to call AddLinkAddress.
	LinkAddressRequest(addr, localAddr tcpip.Address, linkEP LinkEndpoint) *tcpip.Error

	// ResolveStaticAddress attempts to resolve address without sending
	// requests. It either resolves the name immediately or returns the
	// empty LinkAddress.
	//
	// It can be used to resolve broadcast addresses for example.
	ResolveStaticAddress(addr tcpip.Address) (tcpip.LinkAddress, bool)

	// LinkAddressProtocol returns the network protocol of the
	// addresses this this resolver can resolve.
	LinkAddressProtocol() tcpip.NetworkProtocolNumber
}

// LinkAddressCache mac地址缓存
type LinkAddressCache interface {
	// CheckLocalAddress determines if the given local address exists, and if it
	// does not exist.
	CheckLocalAddress(nicid tcpip.NICID, protocol tcpip.NetworkProtocolNumber, addr tcpip.Address) tcpip.NICID

	// AddLinkAddress adds a link address to the cache.
	AddLinkAddress(nicid tcpip.NICID, addr tcpip.Address, linkAddr tcpip.LinkAddress)

	// GetLinkAddress looks up the cache to translate address to link address (e.g. IP -> MAC).
	// If the LinkEndpoint requests address resolution and there is a LinkAddressResolver
	// registered with the network protocol, the cache attempts to resolve the address
	// and returns ErrWouldBlock. Waker is notified when address resolution is
	// complete (success or not).
	//
	// If address resolution is required, ErrNoLinkAddress and a notification channel is
	// returned for the top level caller to block. Channel is closed once address resolution
	// is complete (success or not).
	GetLinkAddress(nicid tcpip.NICID, addr, localAddr tcpip.Address, protocol tcpip.NetworkProtocolNumber, w *sleep.Waker) (tcpip.LinkAddress, <-chan struct{}, *tcpip.Error)

	// RemoveWaker removes a waker that has been added in GetLinkAddress().
	RemoveWaker(nicid tcpip.NICID, addr tcpip.Address, waker *sleep.Waker)
}

// TransportProtocolFactory functions are used by the stack to instantiate
// transport protocols.
type TransportProtocolFactory func() TransportProtocol

// NetworkProtocolFactory provides methods to be used by the stack to
// instantiate network protocols.
type NetworkProtocolFactory func() NetworkProtocol

var (
	// 传输层协议的注册储存结构
	transportProtocols = make(map[string]TransportProtocolFactory)
	// 网络层协议的注册存储结构
	networkProtocols = make(map[string]NetworkProtocolFactory)

	linkEPMu           sync.RWMutex
	nextLinkEndpointID tcpip.LinkEndpointID = 1
	linkEndpoints                           = make(map[tcpip.LinkEndpointID]LinkEndpoint)
)

// 注册一个链路层设备并返回其ID
func RegisterLinkEndpoint(linkEP LinkEndpoint) tcpip.LinkEndpointID {
	linkEPMu.Lock()
	defer linkEPMu.Unlock()

	v := nextLinkEndpointID
	nextLinkEndpointID++

	linkEndpoints[v] = linkEP

	return v
}

// 根据ID找到网卡设备
func FindLinkEndpoint(id tcpip.LinkEndpointID) LinkEndpoint {
	linkEPMu.RLock()
	defer linkEPMu.RUnlock()

	return linkEndpoints[id]
}