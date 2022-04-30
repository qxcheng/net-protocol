package arp

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"github.com/qxcheng/net-protocol/protocol/header"
	"github.com/qxcheng/net-protocol/protocol/stack"
	"log"
)

const (
	ProtocolName = "arp"
	ProtocolNumber = header.ARPProtocolNumber
	ProtocolAddress = tcpip.Address("arp")
)

var broadcastMAC = tcpip.LinkAddress([]byte{0xff, 0xff, 0xff, 0xff, 0xff, 0xff})

func init() {
	stack.RegisterNetworkProtocolFactory(ProtocolName, func() stack.NetworkProtocol {
		return &protocol{}
	})
}

// 表示arp端 实现了stack.NetworkEndpoint
type endpoint struct {
	nicid tcpip.NICID
	addr tcpip.Address
	linkEP stack.LinkEndpoint
	linkAddrCache stack.LinkAddressCache
}

// DefaultTTL ARP未使用
func (e *endpoint) DefaultTTL() uint8 {
	return 0
}

func (e *endpoint) NICID() tcpip.NICID {
	return e.nicid
}

func (e *endpoint) Capabilities() stack.LinkEndpointCapabilities {
	return e.linkEP.Capabilities()
}

func (e *endpoint) ID() *stack.NetworkEndpointID {
	return &stack.NetworkEndpointID{ProtocolAddress}
}

func (e *endpoint) MTU() uint32 {
	lmtu := e.linkEP.MTU()
	return lmtu - uint32(e.MaxHeaderLength())
}

func (e *endpoint) MaxHeaderLength() uint16 {
	return e.linkEP.MaxHeaderLength() + header.ARPSize
}

func (e *endpoint) Close() {}

func (e *endpoint) WritePacket(r *stack.Route, hdr buffer.Prependable, payload buffer.VectorisedView, protocol tcpip.TransportProtocolNumber, ttl uint8) *tcpip.Error {
	return tcpip.ErrNotSupported
}

// HandlePacket arp数据包的处理，包括arp请求和响应
func (e  *endpoint) HandlePacket(r *stack.Route, vv buffer.VectorisedView) {
	v := vv.First()
	h := header.ARP(v)
	if !h.IsValid() {
		return
	}

	// 判断操作码类型
	switch h.Op() {
	case header.ARPRequest:
		// 如果是arp请求
		log.Printf("recv arp request")
		localAddr := tcpip.Address(h.ProtocolAddressTarget())
		if e.linkAddrCache.CheckLocalAddress(e.nicid, header.IPv4ProtocolNumber, localAddr) == 0 {
			return  // 没有缓存直接返回
		}
		hdr := buffer.NewPrependable(int(e.linkEP.MaxHeaderLength()) + header.ARPSize)
		pkt := header.ARP(hdr.Prepend(header.ARPSize))
		pkt.SetIPv4OverEthernet()
		pkt.SetOp(header.ARPReply)
		copy(pkt.HardwareAddressSender(), r.LocalLinkAddress[:])
		copy(pkt.ProtocolAddressSender(), h.ProtocolAddressTarget())
		copy(pkt.ProtocolAddressTarget(), h.ProtocolAddressSender())
		log.Printf("send arp reply")
		e.linkEP.WritePacket(r, hdr, buffer.VectorisedView{}, ProtocolNumber)
		// 当收到 arp 请求需要添加到链路地址缓存中
		fallthrough
	case header.ARPReply:
		// 这里记录ip和mac对应关系，也就是arp表
		addr := tcpip.Address(h.ProtocolAddressSender())
		linkAddr := tcpip.LinkAddress(h.HardwareAddressSender())
		e.linkAddrCache.AddLinkAddress(e.nicid, addr, linkAddr)
	}
}


// 实现了 stack.NetworkProtocol 和 stack.LinkAddressResolver 两个接口
type protocol struct {
}

// implements NetworkProtocol.

func (p *protocol) Number() tcpip.NetworkProtocolNumber {return ProtocolNumber}
func (p *protocol) MinimumPacketSize() int {return header.ARPSize}

func (*protocol) ParseAddresses(v buffer.View) (src, dst tcpip.Address) {
	h := header.ARP(v)
	return tcpip.Address(h.ProtocolAddressSender()), ProtocolAddress
}

func (p *protocol) NewEndpoint(
	nicid tcpip.NICID, addr tcpip.Address, linkAddrCache stack.LinkAddressCache,
	dispatcher stack.TransportDispatcher,
	sender stack.LinkEndpoint) (stack.NetworkEndpoint, *tcpip.Error) {

	if addr != ProtocolAddress {
		return nil, tcpip.ErrBadLocalAddress
	}

	return &endpoint{
		nicid:         nicid,
		addr:          addr,
		linkEP:        sender,
		linkAddrCache: linkAddrCache,
	}, nil
}

func (p *protocol) SetOption(option interface{}) *tcpip.Error {
	return tcpip.ErrUnknownProtocolOption
}

func (p *protocol) Option(option interface{}) *tcpip.Error {
	return tcpip.ErrUnknownProtocolOption
}


// implements stack.LinkAddressResolver.

func (*protocol) LinkAddressProtocol() tcpip.NetworkProtocolNumber {
	return header.IPv4ProtocolNumber
}

func (*protocol) LinkAddressRequest(addr, localAddr tcpip.Address, linkEP stack.LinkEndpoint) *tcpip.Error {
	r := &stack.Route{
		RemoteLinkAddress: broadcastMAC,
	}

	hdr := buffer.NewPrependable(int(linkEP.MaxHeaderLength()) + header.ARPSize)
	h := header.ARP(hdr.Prepend(header.ARPSize))
	h.SetIPv4OverEthernet()
	h.SetOp(header.ARPRequest)
	copy(h.HardwareAddressSender(), linkEP.LinkAddress())
	copy(h.ProtocolAddressSender(), localAddr)
	copy(h.ProtocolAddressTarget(), addr)

	return linkEP.WritePacket(r, hdr, buffer.VectorisedView{}, ProtocolNumber)
}

func (*protocol) ResolveStaticAddress(addr tcpip.Address) (tcpip.LinkAddress, bool) {
	if addr == "\xff\xff\xff\xff" {
		return broadcastMAC, true
	}
	return "", false
}

