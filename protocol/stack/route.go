package stack

import tcpip "github.com/qxcheng/net-protocol/protocol"

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

// Release frees all resources associated with the route.
func (r *Route) Release() {
	if r.ref != nil {
		r.ref.decRef()
		r.ref = nil
	}
}