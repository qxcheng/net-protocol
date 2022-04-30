package stack

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	tcpip "github.com/qxcheng/net-protocol/protocol"
)

// Route represents a route through the networking stack to a given destination.
// 贯穿整个协议栈的路由，也就是在链路层和网络层都可以路由
// 如果目标地址是链路层地址，那么在链路层路由，
// 如果目标地址是网络层地址，那么在网络层路由。
type Route struct {
	RemoteAddress tcpip.Address           // 远端网络层地址，ipv4或者ipv6地址
	RemoteLinkAddress tcpip.LinkAddress   // 远端网卡MAC地址
	LocalAddress tcpip.Address            // 本地网络层地址，ipv4或者ipv6地址
	LocalLinkAddress tcpip.LinkAddress    // 本地网卡MAC地址

	NextHop tcpip.Address                 // 下一跳网络层地址
	NetProto tcpip.NetworkProtocolNumber  // 网络层协议号

	ref *referencedNetworkEndpoint        // 相关的网络终端
}

// 根据参数新建一个路由，并关联一个网络层端
func makeRoute(netProto tcpip.NetworkProtocolNumber, localAddr, remoteAddr tcpip.Address,
	localLinkAddr tcpip.LinkAddress, ref *referencedNetworkEndpoint) Route {
	return Route{
		RemoteAddress:     remoteAddr,
		LocalAddress:      localAddr,
		LocalLinkAddress:  localLinkAddr,
		NetProto:          netProto,
		ref:               ref,
	}
}

// NICID returns the id of the NIC from which this route originates.
func (r *Route) NICID() tcpip.NICID {
	return r.ref.ep.NICID()
}

// MaxHeaderLength forwards the call to the network endpoint's implementation.
func (r *Route) MaxHeaderLength() uint16 {
	return r.ref.ep.MaxHeaderLength()
}

// Stats returns a mutable copy of current stats.
func (r *Route) Stats() tcpip.Stats {
	return r.ref.nic.stack.Stats()
}

// WritePacket writes the packet through the given route.
func (r *Route) WritePacket(hdr buffer.Prependable, payload buffer.VectorisedView,
	protocol tcpip.TransportProtocolNumber, ttl uint8) *tcpip.Error {
	err := r.ref.ep.WritePacket(r, hdr, payload, protocol, ttl)
	if err == tcpip.ErrNoRoute {
		r.Stats().IP.OutgoingPacketErrors.Increment()
	}
	return err
}


// DefaultTTL returns the default TTL of the underlying network endpoint.
func (r *Route) DefaultTTL() uint8 {
	return r.ref.ep.DefaultTTL()
}

// MTU returns the MTU of the underlying network endpoint.
func (r *Route) MTU() uint32 {
	return r.ref.ep.MTU()
}

// Release frees all resources associated with the route.
func (r *Route) Release() {
	if r.ref != nil {
		r.ref.decRef()
		r.ref = nil
	}
}

// Clone clone a route such that the original one can be released and the new
// one will remain valid.
func (r *Route) Clone() Route {
	r.ref.incRef()
	return *r
}