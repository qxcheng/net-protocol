package ports

import (
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"sync"
)

// 端口的唯一标识: 网络层协议-传输层协议-端口号
type portDescriptor struct {
	network   tcpip.NetworkProtocolNumber
	transport tcpip.TransportProtocolNumber
	port      uint16
}

// PortManager 管理端口的对象，由它来保留和释放端口
type PortManager struct {
	mu sync.RWMutex
	// 用一个map接口来保存端口被占用
	allocatedPorts map[portDescriptor]bindAddresses
}

// bindAddresses is a set of IP addresses.
type bindAddresses map[tcpip.Address]struct{}
