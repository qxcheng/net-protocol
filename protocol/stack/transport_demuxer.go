package stack

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"sync"
)

// 网络层协议号和传输层协议号的组合，当作分流器的key值
type protocolIDs struct {
	network   tcpip.NetworkProtocolNumber
	transport tcpip.TransportProtocolNumber
}

// transportEndpoints manages all endpoints of a given protocol. It has its own
// mutex so as to reduce interference between protocols.
// transportEndpoints 管理给定协议的所有端点。
type transportEndpoints struct {
	mu        sync.RWMutex
	endpoints map[TransportEndpointID]TransportEndpoint
}

// transportDemuxer demultiplexes packets targeted at a transport endpoint
// (i.e., after they've been parsed by the network layer). It does two levels
// of demultiplexing: first based on the network and transport protocols, then
// based on endpoints IDs.
// transportDemuxer 解复用针对传输端点的数据包（即，在它们被网络层解析之后）。
// 它执行两级解复用：首先基于网络层协议和传输协议，然后基于端点ID。
type transportDemuxer struct {
	protocol map[protocolIDs]*transportEndpoints
}

// 新建一个分流器
func newTransportDemuxer(stack *Stack) *transportDemuxer {
	d := &transportDemuxer{protocol: make(map[protocolIDs]*transportEndpoints)}
	// Add each network and transport pair to the demuxer.
	for netProto := range stack.networkProtocols {
		for proto := range stack.transportProtocols {
			d.protocol[protocolIDs{netProto, proto}] = &transportEndpoints{endpoints: make(map[TransportEndpointID]TransportEndpoint)}
		}
	}
	return d
}

// deliverPacket attempts to deliver the given packet. Returns true if it found
// an endpoint, false otherwise.
// 根据传输层的id来找到对应的传输端，再将数据包交给这个传输端处理
func (d *transportDemuxer) deliverPacket(
	r *Route, protocol tcpip.TransportProtocolNumber,
	vv buffer.VectorisedView, id TransportEndpointID) bool {

	// 先看看分流器里有没有注册相关协议端，如果没有则返回false

	return true
}

// deliverControlPacket attempts to deliver the given control packet. Returns
// true if it found an endpoint, false otherwise.
func (d *transportDemuxer) deliverControlPacket(
	net tcpip.NetworkProtocolNumber, trans tcpip.TransportProtocolNumber,
	typ ControlType, extra uint32, vv buffer.VectorisedView, id TransportEndpointID) bool {

	return true
}