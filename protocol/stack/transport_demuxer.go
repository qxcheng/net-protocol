package stack

import (
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
