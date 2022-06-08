package udp

import (
	"github.com/qxcheng/net-protocol/pkg/buffer"
	"github.com/qxcheng/net-protocol/pkg/waiter"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"github.com/qxcheng/net-protocol/protocol/header"
	"github.com/qxcheng/net-protocol/protocol/stack"
)

const (
	ProtocolName = "udp"
	ProtocolNumber = header.UDPProtocolNumber
)

type protocol struct{}

func init() {
	stack.RegisterTransportProtocolFactory(ProtocolName, func() stack.TransportProtocol {
		return &protocol{}
	})
}

func (*protocol) NewEndpoint(
	stack *stack.Stack, netProto tcpip.NetworkProtocolNumber,
	waiterQueue *waiter.Queue) (tcpip.Endpoint, *tcpip.Error) {

	return newEndpoint(stack, netProto, waiterQueue), nil
}

func (*protocol) Number() tcpip.TransportProtocolNumber {
	return ProtocolNumber
}

func (*protocol) MinimumPacketSize() int {
	return header.UDPMinimumSize
}

func (*protocol) ParsePorts(v buffer.View) (src, dst uint16, err *tcpip.Error) {
	h := header.UDP(v)
	return h.SourcePort(), h.DestinationPort(), nil
}

// HandleUnknownDestinationPacket handles packets targeted at this protocol but
// that don't match any existing endpoint.
func (p *protocol) HandleUnknownDestinationPacket(*stack.Route, stack.TransportEndpointID, buffer.VectorisedView) bool {
	return true
}

func (p *protocol) SetOption(option interface{}) *tcpip.Error {
	return tcpip.ErrUnknownProtocolOption
}

func (p *protocol) Option(option interface{}) *tcpip.Error {
	return tcpip.ErrUnknownProtocolOption
}


