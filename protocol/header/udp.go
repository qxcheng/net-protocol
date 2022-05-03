package header

import (
	"encoding/binary"
	tcpip "github.com/qxcheng/net-protocol/protocol"
)

const (
	udpSrcPort  = 0
	udpDstPort  = 2
	udpLength   = 4
	udpChecksum = 6
)

// UDPFields udp首部字段
type UDPFields struct {
	SrcPort uint16
	DstPort uint16
	Length uint16    // UDP数据包的长度
	Checksum uint16  // UDP数据包的校验和
}

// UDP represents a UDP header stored in a byte array.
type UDP []byte

const (
	UDPMinimumSize = 8  // UDP数据包的最小长苏
	UDPProtocolNumber tcpip.TransportProtocolNumber = 17  // UDP的传输层协议号
)

func (b UDP) SourcePort() uint16 {
	return binary.BigEndian.Uint16(b[udpSrcPort:])
}

func (b UDP) DestinationPort() uint16 {
	return binary.BigEndian.Uint16(b[udpDstPort:])
}

func (b UDP) Length() uint16 {
	return binary.BigEndian.Uint16(b[udpLength:])
}

func (b UDP) Checksum() uint16 {
	return binary.BigEndian.Uint16(b[udpChecksum:])
}

func (b UDP) Payload() []byte {
	return b[UDPMinimumSize:]
}

func (b UDP) SetSourcePort(port uint16) {
	binary.BigEndian.PutUint16(b[udpSrcPort:], port)
}

func (b UDP) SetDestinationPort(port uint16) {
	binary.BigEndian.PutUint16(b[udpDstPort:], port)
}

func (b UDP) SetChecksum(checksum uint16) {
	binary.BigEndian.PutUint16(b[udpChecksum:], checksum)
}

// CalculateChecksum calculates the checksum of the udp packet, given the total
// length of the packet and the checksum of the network-layer pseudo-header
// (excluding the total length) and the checksum of the payload.
func (b UDP) CalculateChecksum(partialChecksum uint16, totalLen uint16) uint16 {
	// Add the length portion of the checksum to the pseudo-checksum.
	tmp := make([]byte, 2)
	binary.BigEndian.PutUint16(tmp, totalLen)
	checksum := Checksum(tmp, partialChecksum)

	// Calculate the rest of the checksum.
	return Checksum(b[:UDPMinimumSize], checksum)
}

func (b UDP) Encode(u *UDPFields) {
	binary.BigEndian.PutUint16(b[udpSrcPort:], u.SrcPort)
	binary.BigEndian.PutUint16(b[udpDstPort:], u.DstPort)
	binary.BigEndian.PutUint16(b[udpLength:], u.Length)
	binary.BigEndian.PutUint16(b[udpChecksum:], u.Checksum)
}
