package ipv4

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"github.com/qxcheng/net-protocol/protocol/header"
	"github.com/qxcheng/net-protocol/protocol/network/fragmentation"
	"github.com/qxcheng/net-protocol/protocol/network/hash"
	"github.com/qxcheng/net-protocol/protocol/stack"
	"log"
	"sync/atomic"
)

const (
	ProtocolName = "ipv4"                       // 协议名
	ProtocolNumber = header.IPv4ProtocolNumber  // 协议号
	maxTotalSize = 0xffff  // ipv4头中16位的TotalLength字段能编码的最大尺寸
	buckets = 2048  // buckets is the number of identifier buckets.
)

var (
	ids    []uint32
	hashIV uint32
)

// 初始化的时候初始化buckets，用于hash计算
// 并在网络层协议中注册ipv4协议
func init() {
	ids = make([]uint32, buckets)

	r := hash.RandN32(1 + buckets)
	for i := range ids {
		ids[i] = r[i]
	}
	hashIV = r[buckets]

	// 注册网络层协议
	stack.RegisterNetworkProtocolFactory(ProtocolName, func() stack.NetworkProtocol {
		return &protocol{}
	})
}


// ipv4端点
type endpoint struct {
	nicid tcpip.NICID                           // 网卡id
	id stack.NetworkEndpointID                  // 表示该endpoint的id，也是ip地址
	linkEP stack.LinkEndpoint                   // 链路端的表示
	dispatcher stack.TransportDispatcher        // 报文分发器
	echoRequests chan echoRequest               // ping请求报文接收队列
	fragmentation *fragmentation.Fragmentation  // ip报文分片处理器
}

// DefaultTTL 默认的TTL值，TTL每经过路由转发一次就会减1
func (e *endpoint) DefaultTTL() uint8 {
	return 255
}

// MTU 获取去除ipv4头部后的最大报文长度
func (e *endpoint) MTU() uint32 {
	return calculateMTU(e.linkEP.MTU())
}

func calculateMTU(mtu uint32) uint32 {
	if mtu > maxTotalSize {
		mtu = maxTotalSize
	}
	return mtu - header.IPv4MinimumSize
}

func (e *endpoint) Capabilities() stack.LinkEndpointCapabilities {
	return e.linkEP.Capabilities()
}

func (e *endpoint) NICID() tcpip.NICID {
	return e.nicid
}

// ID 获取该网络层端的id，也就是ip地址
func (e *endpoint) ID() *stack.NetworkEndpointID {
	return &e.id
}

// MaxHeaderLength 链路层和网络层的头部长度
func (e *endpoint) MaxHeaderLength() uint16 {
	return e.linkEP.MaxHeaderLength() + header.IPv4MinimumSize
}

// Close cleans up resources associated with the endpoint.
func (e *endpoint) Close() {
	close(e.echoRequests)
}

// hashRoute calculates a hash value for the given route. It uses the source &
// destination address, the transport protocol number, and a random initial
// value (generated once on initialization) to generate the hash.
func hashRoute(r *stack.Route, protocol tcpip.TransportProtocolNumber) uint32 {
	t := r.LocalAddress
	a := uint32(t[0]) | uint32(t[1])<<8 | uint32(t[2])<<16 | uint32(t[3])<<24
	t = r.RemoteAddress
	b := uint32(t[0]) | uint32(t[1])<<8 | uint32(t[2])<<16 | uint32(t[3])<<24
	return hash.Hash3Words(a, b, uint32(protocol), hashIV)
}

// WritePacket 将传输层的数据封装加上IP头，并调用网卡的写入接口，写入IP报文
func (e *endpoint) WritePacket(r *stack.Route, hdr buffer.Prependable, payload buffer.VectorisedView,
	protocol tcpip.TransportProtocolNumber, ttl uint8) *tcpip.Error {

	ip := header.IPv4(hdr.Prepend(header.IPv4MinimumSize))
	length := uint16(hdr.UsedLength() + payload.Size())
	id := uint32(0)
	// 如果报文长度大于68
	if length > header.IPv4MaximumHeaderSize+8 {
		// Packets of 68 bytes or less are required by RFC 791 to not be
		// fragmented, so we only assign ids to larger packets.
		id = atomic.AddUint32(&ids[hashRoute(r, protocol) % buckets], 1)
	}
	// ip首部编码
	ip.Encode(&header.IPv4Fields{
		IHL:         header.IPv4MinimumSize,
		TotalLength: length,
		ID:          uint16(id),
		TTL:         ttl,
		Protocol:    uint8(protocol),
		SrcAddr:     r.LocalAddress,
		DstAddr:     r.RemoteAddress,
	})
	// 计算校验和和设置校验和
	ip.SetChecksum(^ip.CalculateChecksum())
	r.Stats().IP.PacketsSent.Increment()

	// 写入网卡接口
	log.Printf("send ipv4 packet %d bytes, proto: 0x%x", hdr.UsedLength()+payload.Size(), protocol)
	return e.linkEP.WritePacket(r, hdr, payload, ProtocolNumber)
}

// HandlePacket 收到ip包的处理, 由链路层调用
func (e *endpoint) HandlePacket(r *stack.Route, vv buffer.VectorisedView) {
	// 得到ip报文
	h := header.IPv4(vv.First())
	// 检查报文是否有效
	if !h.IsValid(vv.Size()) {
		return
	}

	hlen := int(h.HeaderLength())
	tlen := int(h.TotalLength())
	vv.TrimFront(hlen)
	vv.CapLength(tlen - hlen)

	// 检查ip报文是否有更多的分片（检查MF标志位）
	more := (h.Flags() & header.IPv4FlagMoreFragments) != 0
	// 此包是一个分片，需要ip分片重组
	if more || h.FragmentOffset() != 0 {
		last := h.FragmentOffset() + uint16(vv.Size()) - 1
		var ready bool
		vv, ready = e.fragmentation.Process(hash.IPv4FragmentHash(h), h.FragmentOffset(), last, more, vv)
		if !ready {
			return
		}
	}
	// 得到传输层的协议
	p := h.TransportProtocol()
	// 如果时ICMP协议，则进入ICMP处理函数
	if p == header.ICMPv4ProtocolNumber {
		e.handleICMP(r, vv)
		return
	}
	r.Stats().IP.PacketsDelivered.Increment()
	// 根据协议分发到不通处理函数，比如协议时TCP，会进入tcp.HandlePacket
	log.Printf("recv ipv4 packet %d bytes, proto: 0x%x", tlen, p)
	e.dispatcher.DeliverTransportPacket(r, p, vv)
}


// 实现NetworkProtocol接口
type protocol struct{}

func NewProtocol() stack.NetworkProtocol {
	return &protocol{}
}

// NewEndpoint 新建一个ipv4端
func (p *protocol) NewEndpoint(
	nicid tcpip.NICID, addr tcpip.Address, linkAddrCache stack.LinkAddressCache,
	dispatcher stack.TransportDispatcher, linkEP stack.LinkEndpoint) (stack.NetworkEndpoint, *tcpip.Error) {

	e := &endpoint{
		nicid:         nicid,
		id:            stack.NetworkEndpointID{LocalAddress: addr},
		linkEP:        linkEP,
		dispatcher:    dispatcher,
		echoRequests:  make(chan echoRequest, 10),
		fragmentation: fragmentation.NewFragmentation(fragmentation.HighFragThreshold,
			fragmentation.LowFragThreshold, fragmentation.DefaultReassembleTimeout),
	}

	// 启动一个goroutine给icmp请求服务
	go e.echoReplier()

	return e, nil
}

func (p *protocol) Number() tcpip.NetworkProtocolNumber {
	return ProtocolNumber
}

func (p *protocol) MinimumPacketSize() int {
	return header.IPv4MinimumSize
}

func (*protocol) ParseAddresses(v buffer.View) (src, dst tcpip.Address) {
	h := header.IPv4(v)
	return h.SourceAddress(), h.DestinationAddress()
}

func (p *protocol) SetOption(option interface{}) *tcpip.Error {
	return tcpip.ErrUnknownProtocolOption
}

func (p *protocol) Option(option interface{}) *tcpip.Error {
	return tcpip.ErrUnknownProtocolOption
}
