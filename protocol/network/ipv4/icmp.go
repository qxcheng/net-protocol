package ipv4

import (
	"encoding/binary"
	"github.com/qxcheng/net-protocol/pkg/buffer"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"github.com/qxcheng/net-protocol/protocol/header"
	"github.com/qxcheng/net-protocol/protocol/stack"
	"log"
)

// handleControl处理ICMP数据包包含导致ICMP发送的原始数据包的标头的情况。
// 此信息用于确定必须通知哪个传输端点有关ICMP数据包。
func (e *endpoint) handleControl(typ stack.ControlType, extra uint32, vv buffer.VectorisedView) {
	// vv 去除ICMP头后的数据部分：一般存放的是IP头和8字节的IP数据（即传输层的头）
	h := header.IPv4(vv.First())

	// 如果ipv4头小于最小值 || 包头中的原地址不匹配本机
	if len(h) < header.IPv4MinimumSize || h.SourceAddress() != e.id.LocalAddress {
		return
	}

	hlen := int(h.HeaderLength())
	// 如果头不完整 || 不是第一个分片（因为其他分片没有传输层的8字节头）
	if vv.Size() < hlen || h.FragmentOffset() != 0 {
		return
	}

	vv.TrimFront(hlen)
	p := h.TransportProtocol()
	e.dispatcher.DeliverTransportControlPacket(e.id.LocalAddress, h.DestinationAddress(), ProtocolNumber, p, typ, extra, vv)
}

// 处理ICMP报文
func (e *endpoint) handleICMP(r *stack.Route, vv buffer.VectorisedView) {
	v := vv.First()
	if len(v) < header.ICMPv4MinimumSize {
		return
	}
	h := header.ICMPv4(v)

	// 根据icmp的类型来进行相应的处理
	switch h.Type() {
	case header.ICMPv4Echo:
		if len(v) < header.ICMPv4EchoMinimumSize {
			return
		}
		log.Printf("icmp echo")
		vv.TrimFront(header.ICMPv4MinimumSize)
		req := echoRequest{r: r.Clone(), v: vv.ToView()}
		select {
		// 发送给echoReplier处理
		case e.echoRequests <- req:
		default:
			req.r.Release()
		}

	case header.ICMPv4EchoReply:  // icmp echo响应
		if len(v) < header.ICMPv4EchoMinimumSize {
			return
		}
		e.dispatcher.DeliverTransportPacket(r, header.ICMPv4ProtocolNumber, vv)

	case header.ICMPv4DstUnreachable:  // 目标不可达
		if len(v) < header.ICMPv4DstUnreachableMinimumSize {
			return
		}
		vv.TrimFront(header.ICMPv4DstUnreachableMinimumSize)
		switch h.Code() {
		case header.ICMPv4PortUnreachable: // 端口不可达
			e.handleControl(stack.ControlPortUnreachable, 0, vv)
		case header.ICMPv4FragmentationNeeded: // 需要进行分片但设置不分片标志
			mtu := uint32(binary.BigEndian.Uint16(v[header.ICMPv4DstUnreachableMinimumSize-2:]))
			e.handleControl(stack.ControlPacketTooBig, calculateMTU(mtu), vv)
		}
	}
	// TODO: Handle other ICMP types.
}


type echoRequest struct {
	r stack.Route
	v buffer.View
}

// 处理icmp echo请求的goroutine
func (e *endpoint) echoReplier() {
	for req := range e.echoRequests {
		sendPing4(&req.r, 0, req.v)
		req.r.Release()
	}
}

// 根据icmp echo请求，封装icmp echo响应报文，并传给ip层处理
func sendPing4(r *stack.Route, code byte, data buffer.View) *tcpip.Error {
	hdr := buffer.NewPrependable(header.ICMPv4EchoMinimumSize +int(r.MaxHeaderLength()))

	icmpv4 := header.ICMPv4(hdr.Prepend(header.ICMPv4EchoMinimumSize)) // 6 字节
	icmpv4.SetType(header.ICMPv4EchoReply)  // 第1字节
	icmpv4.SetCode(code)  // 第2字节
	copy(icmpv4[header.ICMPv4MinimumSize:], data)  // 第5、6字节放的data前2字节
	data = data[header.ICMPv4EchoMinimumSize-header.ICMPv4MinimumSize:] // data裁掉前2字节
	icmpv4.SetChecksum(^header.Checksum(icmpv4, header.Checksum(data, 0)))  // 第3、4字节

	log.Printf("icmp reply")
	// 传给ip层处理
	return r.WritePacket(hdr, data.ToVectorisedView(), header.ICMPv4ProtocolNumber, r.DefaultTTL())

}