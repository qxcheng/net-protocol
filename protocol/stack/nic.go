package stack

import (
	"github.com/qxcheng/net-protocol/pkg/ilist"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"sync"
)

// 代表一个与网络栈关联的网卡对象
type NIC struct {
	stack *Stack
	id tcpip.NICID            // 每个网卡的唯一标识号
	name string               // 网卡名，可有可无
	linkEP LinkEndpoint       // 链路层端点
	demux *transportDemuxer   // 传输层的解复用
	mu sync.RWMutex
	spoofing bool
	promiscuous bool
	primary map[tcpip.NetworkProtocolNumber]*ilist.List
	endpoints map[NetworkEndpointID]*referencedNetworkEndpoint  // 网络层端的记录
	subnets []tcpip.Subnet    // 子网的记录
}

type referencedNetworkEndpoint struct {
	ilist.Entry
	refs     int32
	ep       NetworkEndpoint
	nic      *NIC
	protocol tcpip.NetworkProtocolNumber

	// linkCache is set if link address resolution is enabled for this
	// protocol. Set to nil otherwise.
	linkCache LinkAddressCache

	// holdsInsertRef is protected by the NIC's mutex. It indicates whether
	// the reference count is biased by 1 due to the insertion of the
	// endpoint. It is reset to false when RemoveAddress is called on the
	// NIC.
	holdsInsertRef bool
}