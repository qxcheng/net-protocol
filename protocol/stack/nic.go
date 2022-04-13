package stack

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	"github.com/qxcheng/net-protocol/pkg/ilist"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"github.com/qxcheng/net-protocol/protocol/header"
	"strings"
	"sync"
	"sync/atomic"
)

// NIC 代表一个与网络栈关联的网卡对象
type NIC struct {
	stack *Stack
	id tcpip.NICID            // 每个网卡的唯一标识号
	name string               // 网卡名，可有可无
	linkEP LinkEndpoint       // 链路层端点
	demux *transportDemuxer   // 传输层的解复用
	mu sync.RWMutex
	spoofing bool             // 地址欺骗
	promiscuous bool          // 混杂模式
	primary map[tcpip.NetworkProtocolNumber]*ilist.List
	endpoints map[NetworkEndpointID]*referencedNetworkEndpoint  // 网络层端的记录
	subnets []tcpip.Subnet    // 子网的记录
}

// PrimaryEndpointBehavior is an enumeration of an endpoint's primacy behavior.
type PrimaryEndpointBehavior int

const (
	// CanBePrimaryEndpoint indicates the endpoint can be used as a primary
	// endpoint for new connections with no local address. This is the
	// default when calling NIC.AddAddress.
	CanBePrimaryEndpoint PrimaryEndpointBehavior = iota

	// FirstPrimaryEndpoint indicates the endpoint should be the first
	// primary endpoint considered. If there are multiple endpoints with
	// this behavior, the most recently-added one will be first.
	FirstPrimaryEndpoint

	// NeverPrimaryEndpoint indicates the endpoint should never be a
	// primary endpoint.
	NeverPrimaryEndpoint
)

// 根据参数新建一个NIC
func newNIC(stack *Stack, id tcpip.NICID, name string, ep LinkEndpoint) *NIC {
	return &NIC{
		stack:       stack,
		id:          id,
		name:        name,
		linkEP:      ep,
		demux:       newTransportDemuxer(stack),
		primary:     make(map[tcpip.NetworkProtocolNumber]*ilist.List),
		endpoints:   make(map[NetworkEndpointID]*referencedNetworkEndpoint),
	}
}

// 添加当前的NIC到链路层设备，激活该NIC
func (n *NIC) attachLinkEndpoint() {
	n.linkEP.Attach(n)
}

// 设置网卡为混杂模式
func (n *NIC) setPromiscuousMode(enable bool) {
	n.mu.Lock()
	n.promiscuous = enable
	n.mu.Unlock()
}

// 判断网卡是否开启混杂模式
func (n *NIC) isPromiscuousMode() bool {
	n.mu.RLock()
	rv := n.promiscuous
	n.mu.RUnlock()
	return rv
}

// setSpoofing 设置ip地址欺骗
func (n *NIC) setSpoofing(enable bool) {
	n.mu.Lock()
	n.spoofing = enable
	n.mu.Unlock()
}

// ip地址相关 ///////////////////////////////////////////////////////////////

// Addresses 获取网卡已分配的协议和对应的地址
func (n *NIC) Addresses() []tcpip.ProtocolAddress {
	n.mu.RLock()
	defer n.mu.RUnlock()
	addrs := make([]tcpip.ProtocolAddress, 0, len(n.endpoints))
	for nid, ep := range n.endpoints {
		addrs = append(addrs, tcpip.ProtocolAddress{
			Protocol: ep.protocol,
			Address:  nid.LocalAddress,
		})
	}
	return addrs
}

// AddAddress 向n添加一个新地址，以便它开始接受针对给定地址（和网络协议）的数据包
// 最终会生成一个网络端注册到 endpoints
func (n *NIC) AddAddress(protocol tcpip.NetworkProtocolNumber, addr tcpip.Address) *tcpip.Error {
	return n.AddAddressWithOptions(protocol, addr, CanBePrimaryEndpoint)
}

// AddAddressWithOptions 与AddAddress相同，但允许您指定新端点是否可以是主端点。
func (n *NIC) AddAddressWithOptions(protocol tcpip.NetworkProtocolNumber, addr tcpip.Address,
	peb PrimaryEndpointBehavior) *tcpip.Error {
	n.mu.Lock()
	_, err := n.addAddressLocked(protocol, addr, peb, false)
	n.mu.Unlock()
	return err
}

// 在NIC上添加addr地址，注册和初始化网络层协议，相当于给网卡添加ip地址
func (n *NIC) addAddressLocked(protocol tcpip.NetworkProtocolNumber, addr tcpip.Address,
	peb PrimaryEndpointBehavior, replace bool) (*referencedNetworkEndpoint, *tcpip.Error) {
	// 查看是否支持该协议，若不支持则返回错误
	netProto, ok := n.stack.networkProtocols[protocol]
	if !ok {
		return nil, tcpip.ErrUnknownProtocol
	}

	// Create the new network endpoint.
	// 比如netProto为ipv4，会调用ipv4.NewEndpoint，新建一个网络层端
	ep, err := netProto.NewEndpoint(n.id, addr, n.stack, n, n.linkEP)
	if err != nil {
		return nil, err
	}

	// 获取网络层端的id，其实就是结构体包了个ip地址
	id := *ep.ID()
	if ref, ok := n.endpoints[id]; ok {
		// 不是替换，且该id存在，返回错误
		if !replace {
			return nil, tcpip.ErrDuplicateAddress
		}
		n.removeEndpointLocked(ref)
	}

	ref := &referencedNetworkEndpoint{
		refs:           1,
		ep:             ep,
		nic:            n,
		protocol:       protocol,
		holdsInsertRef: true,
	}

	// Set up cache if link address resolution exists for this protocol.
	if n.linkEP.Capabilities() & CapabilityResolutionRequired != 0 {
		if _, ok := n.stack.linkAddrResolvers[protocol]; ok {
			ref.linkCache = n.stack
		}
	}

	// 注册该网络端
	n.endpoints[id] = ref

	l, ok := n.primary[protocol]
	if !ok {
		l = &ilist.List{}
		n.primary[protocol] = l
	}

	switch peb {
	case CanBePrimaryEndpoint:
		l.PushBack(ref)
	case FirstPrimaryEndpoint:
		l.PushFront(ref)
	}

	return ref, nil
}

// RemoveAddress 删除一个地址
func (n *NIC) RemoveAddress(addr tcpip.Address) *tcpip.Error {
	n.mu.Lock()
	r := n.endpoints[NetworkEndpointID{addr}]
	if r == nil || !r.holdsInsertRef {
		n.mu.Unlock()
		return tcpip.ErrBadLocalAddress
	}

	r.holdsInsertRef = false
	n.mu.Unlock()

	r.decRef()

	return nil
}

// 根据网络层协议号找到对应的网络层端
func (n *NIC) primaryEndpoint(protocol tcpip.NetworkProtocolNumber) *referencedNetworkEndpoint {
	n.mu.RLock()
	defer n.mu.RUnlock()

	list := n.primary[protocol]
	if list == nil {
		return nil
	}

	for e := list.Front(); e != nil; e = e.Next() {
		r := e.(*referencedNetworkEndpoint)
		// TODO: allow broadcast address when SO_BROADCAST is set.
		switch r.ep.ID().LocalAddress {
		case header.IPv4Broadcast, header.IPv4Any:
			continue
		}
		if r.tryIncRef() {
			return r
		}
	}
	return nil
}

// 根据address参数查找对应的网络层端
func (n *NIC) findEndpoint(protocol tcpip.NetworkProtocolNumber, address tcpip.Address,
	peb PrimaryEndpointBehavior) *referencedNetworkEndpoint {
	id := NetworkEndpointID{address}

	n.mu.RLock()
	ref := n.endpoints[id]
	if ref != nil && !ref.tryIncRef() {
		ref = nil
	}
	spoofing := n.spoofing
	n.mu.RUnlock()

	if ref != nil || !spoofing {
		return ref
	}

	n.mu.Lock()
	ref = n.endpoints[id]
	if ref == nil || !ref.tryIncRef() {
		ref, _ = n.addAddressLocked(protocol, address, peb, true)
		if ref != nil {
			ref.holdsInsertRef = false
		}
	}
	n.mu.Unlock()
	return ref
}

func (n *NIC) removeEndpoint(r *referencedNetworkEndpoint) {
	n.mu.Lock()
	n.removeEndpointLocked(r)
	n.mu.Unlock()
}

// 删除一个网络端
func (n *NIC) removeEndpointLocked(r *referencedNetworkEndpoint) {
	id := *r.ep.ID()

	if n.endpoints[id] != r {
		return
	}

	if r.holdsInsertRef {
		panic("Reference count dropped to zero before being removed")
	}

	delete(n.endpoints, id)
	wasInList := r.Next() != nil || r.Prev() != nil || r == n.primary[r.protocol].Front()
	if wasInList {
		n.primary[r.protocol].Remove(r)
	}

	r.ep.Close()
}

// 子网相关 /////////////////////////////////////////////////////////////////////

// AddSubnet 向n添加一个新子网，以便它开始接受针对给定地址和网络协议的数据包。
func (n *NIC) AddSubnet(protocol tcpip.NetworkProtocolNumber, subnet tcpip.Subnet) {
	n.mu.Lock()
	n.subnets = append(n.subnets, subnet)
	n.mu.Unlock()
}

// RemoveSubnet 从n中删除一个子网
func (n *NIC) RemoveSubnet(subnet tcpip.Subnet) {
	n.mu.Lock()

	tmp := n.subnets[:0]  // 使用相同的底层数组
	for _, sub := range n.subnets {
		if sub != subnet {
			tmp = append(tmp, sub)
		}
	}
	n.subnets = tmp

	n.mu.Unlock()
}

// ContainsSubnet 判断 subnet 这个子网是否在该网卡下
func (n *NIC) ContainsSubnet(subnet tcpip.Subnet) bool {
	for _, s := range n.subnets {
		if s == subnet {
			return true
		}
	}
	return false
}

// Subnets 获取该网卡的所有子网
func (n *NIC) Subnets() []tcpip.Subnet {
	n.mu.RLock()
	defer n.mu.RUnlock()

	sns := make([]tcpip.Subnet, 0, len(n.subnets)+len(n.endpoints))
	for nid := range n.endpoints {
		sn, err := tcpip.NewSubnet(nid.LocalAddress, tcpip.AddressMask(strings.Repeat("\xff", len(nid.LocalAddress))))
		if err != nil {
			panic("Invalid endpoint subnet: " + err.Error())
		}
		sns = append(sns, sn)
	}
	return append(sns, n.subnets...)
}

// 分发包相关 ////////////////////////////////////////////////////////////////////

// DeliverNetworkPacket 找到适当的网络协议端点并将数据包交给它进一步处理。
// 当NIC从物理接口接收数据包时，将调用此函数。比如protocol是arp协议号， 那么会找到arp.HandlePacket来处理数据报。
// protocol是ipv4协议号， 那么会找到ipv4.HandlePacket来处理数据报。
func (n *NIC) DeliverNetworkPacket(linkEP LinkEndpoint, remoteLinkAddr, localLinkAddr tcpip.LinkAddress,
	protocol tcpip.NetworkProtocolNumber, vv buffer.VectorisedView) {
	netProto, ok := n.stack.networkProtocols[protocol]
	if !ok {
		n.stack.stats.UnknownProtocolRcvdPackets.Increment()
		return
	}

	if netProto.Number() == header.IPv4ProtocolNumber || netProto.Number() == header.IPv6ProtocolNumber {
		n.stack.stats.IP.PacketsReceived.Increment()
	}

	if len(vv.First()) < netProto.MinimumPacketSize() {
		n.stack.stats.MalformedRcvdPackets.Increment()
		return
	}

	src, dst := netProto.ParseAddresses(vv.First())

	// 根据网络协议和数据包的目的地址, 找到网络端, 然后将数据包分发给网络层
	if ref := n.getRef(protocol, dst); ref != nil {
		r := makeRoute(protocol, dst, src, linkEP.LinkAddress(), ref)
		r.RemoteLinkAddress = remoteLinkAddr
		ref.ep.HandlePacket(&r, vv)
		ref.decRef()
		return
	}

	// This NIC doesn't care about the packet. Find a NIC that cares about the
	// packet and forward it to the NIC.
	//
	// TODO: Should we be forwarding the packet even if promiscuous?
	if n.stack.Forwarding() {
		r, err := n.stack.FindRoute(0, "", dst, protocol)
		if err != nil {
			n.stack.stats.IP.InvalidAddressesReceived.Increment()
			return
		}
		defer r.Release()

		r.LocalLinkAddress = n.linkEP.LinkAddress()
		r.RemoteLinkAddress = remoteLinkAddr

		// Found a NIC.
		n := r.ref.nic
		n.mu.RLock()
		ref, ok := n.endpoints[NetworkEndpointID{dst}]
		n.mu.RUnlock()
		if ok && ref.tryIncRef() {
			ref.ep.HandlePacket(&r, vv)
			ref.decRef()
		} else {
			// n doesn't have a destination endpoint.
			// Send the packet out of n.
			hdr := buffer.NewPrependableFromView(vv.First())
			vv.RemoveFirst()
			n.linkEP.WritePacket(&r, hdr, vv, protocol)
		}
		return
	}
	n.stack.stats.IP.InvalidAddressesReceived.Increment()
}

// 根据协议类型和目标地址，找出关联的Endpoint
func (n *NIC) getRef(protocol tcpip.NetworkProtocolNumber, dst tcpip.Address) *referencedNetworkEndpoint {
	id := NetworkEndpointID{dst}

	n.mu.RLock()
	if ref, ok := n.endpoints[id]; ok && ref.tryIncRef() {
		n.mu.RUnlock()
		return ref
	}

	promiscuous := n.promiscuous
	// Check if the packet is for a subnet this NIC cares about.
	if !promiscuous {
		for _, sn := range n.subnets {
			if sn.Contains(dst) {
				promiscuous = true
				break
			}
		}
	}
	n.mu.RUnlock()
	if promiscuous {
		// Try again with the lock in exclusive mode. If we still can't
		// get the endpoint, create a new "temporary" one. It will only
		// exist while there's a route through it.
		n.mu.Lock()
		if ref, ok := n.endpoints[id]; ok && ref.tryIncRef() {
			n.mu.Unlock()
			return ref
		}
		ref, err := n.addAddressLocked(protocol, dst, CanBePrimaryEndpoint, true)
		n.mu.Unlock()
		if err == nil {
			ref.holdsInsertRef = false
			return ref
		}
	}
	return nil
}

// DeliverTransportPacket delivers the packets to the appropriate transport
// protocol endpoint.
// DeliverTransportPacket 按照传输层协议号和传输层ID，将数据包分发给相应的传输层端
// 比如 protocol=6表示为tcp协议，将会给相应的tcp端分发消息。
func (n *NIC) DeliverTransportPacket(r *Route, protocol tcpip.TransportProtocolNumber, vv buffer.VectorisedView) {
	// 先查找协议栈是否注册了该传输层协议
	state, ok := n.stack.transportProtocols[protocol]
	if !ok {
		n.stack.stats.UnknownProtocolRcvdPackets.Increment()
		return
	}

	transProto := state.proto
	// 如果报文长度比该协议最小报文长度还小，那么丢弃它
	if len(vv.First()) < transProto.MinimumPacketSize() {
		n.stack.stats.MalformedRcvdPackets.Increment()
		return
	}

	// 解析报文得到源端口和目的端口
	srcPort, dstPort, err := transProto.ParsePorts(vv.First())
	if err != nil {
		n.stack.stats.MalformedRcvdPackets.Increment()
		return
	}

	id := TransportEndpointID{dstPort, r.LocalAddress, srcPort, r.RemoteAddress}
	// 调用分流器，根据传输层协议和传输层id分发数据报文
	if n.demux.deliverPacket(r, protocol, vv, id) {
		return
	}
	if n.stack.demux.deliverPacket(r, protocol, vv, id) {
		return
	}

	// Try to deliver to per-stack default handler.
	if state.defaultHandler != nil {
		if state.defaultHandler(r, id, vv) {
			return
		}
	}

	// We could not find an appropriate destination for this packet, so
	// deliver it to the global handler.
	if !transProto.HandleUnknownDestinationPacket(r, id, vv) {
		n.stack.stats.MalformedRcvdPackets.Increment()
	}
}

// DeliverTransportControlPacket delivers control packets to the appropriate
// transport protocol endpoint.
func (n *NIC) DeliverTransportControlPacket(
	local, remote tcpip.Address, net tcpip.NetworkProtocolNumber,
	trans tcpip.TransportProtocolNumber, typ ControlType, extra uint32, vv buffer.VectorisedView) {

	state, ok := n.stack.transportProtocols[trans]
	if !ok {
		return
	}

	transProto := state.proto

	// ICMPv4 only guarantees that 8 bytes of the transport protocol will
	// be present in the payload. We know that the ports are within the
	// first 8 bytes for all known transport protocols.
	if len(vv.First()) < 8 {
		return
	}

	srcPort, dstPort, err := transProto.ParsePorts(vv.First())
	if err != nil {
		return
	}

	id := TransportEndpointID{srcPort, local, dstPort, remote}
	if n.demux.deliverControlPacket(net, trans, typ, extra, vv, id) {
		return
	}
	if n.stack.demux.deliverControlPacket(net, trans, typ, extra, vv, id) {
		return
	}
}


type referencedNetworkEndpoint struct {
	ilist.Entry                // 使其可以加入队列
	refs     int32
	ep       NetworkEndpoint   // 网络层端点
	nic      *NIC              // 网卡对象
	protocol tcpip.NetworkProtocolNumber  // 网络层协议号

	// linkCache is set if link address resolution is enabled for this
	// protocol. Set to nil otherwise.
	linkCache LinkAddressCache

	// holdsInsertRef is protected by the NIC's mutex. It indicates whether
	// the reference count is biased by 1 due to the insertion of the
	// endpoint. It is reset to false when RemoveAddress is called on the
	// NIC.
	holdsInsertRef bool
}

// decRef decrements the ref count and cleans up the endpoint once it reaches zero.
func (r *referencedNetworkEndpoint) decRef() {
	if atomic.AddInt32(&r.refs, -1) == 0 {
		r.nic.removeEndpoint(r)
	}
}

// incRef increments the ref count. It must only be called when the caller is
// known to be holding a reference to the endpoint, otherwise tryIncRef should
// be used.
func (r *referencedNetworkEndpoint) incRef() {
	atomic.AddInt32(&r.refs, 1)
}

// tryIncRef attempts to increment the ref count from n to n+1, but only if n is
// not zero. That is, it will increment the count if the endpoint is still
// alive, and do nothing if it has already been clean up.
func (r *referencedNetworkEndpoint) tryIncRef() bool {
	for {
		v := atomic.LoadInt32(&r.refs)
		if v == 0 {
			return false
		}

		if atomic.CompareAndSwapInt32(&r.refs, v, v+1) {
			return true
		}
	}
}