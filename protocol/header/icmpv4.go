package header

import (
	"encoding/binary"
	tcpip "github.com/qxcheng/net-protocol/protocol"
)

type  ICMPv4 []byte

const (
	ICMPv4MinimumSize = 4      // ICMP包最小尺寸
	ICMPv4EchoMinimumSize = 6  // ICMP echo包的最小尺寸
	ICMPv4DstUnreachableMinimumSize = ICMPv4MinimumSize + 4  // ICMP目的地不可达包最小尺寸
	ICMPv4ProtocolNumber tcpip.TransportProtocolNumber = 1   //  ICMP传输层协议号
)

// ICMPv4Type is the ICMP type field described in RFC 792.
type ICMPv4Type byte

// Typical values of ICMPv4Type defined in RFC 792.
const (
	ICMPv4EchoReply      ICMPv4Type = 0
	ICMPv4DstUnreachable ICMPv4Type = 3
	ICMPv4SrcQuench      ICMPv4Type = 4
	ICMPv4Redirect       ICMPv4Type = 5
	ICMPv4Echo           ICMPv4Type = 8
	ICMPv4TimeExceeded   ICMPv4Type = 11
	ICMPv4ParamProblem   ICMPv4Type = 12
	ICMPv4Timestamp      ICMPv4Type = 13
	ICMPv4TimestampReply ICMPv4Type = 14
	ICMPv4InfoRequest    ICMPv4Type = 15
	ICMPv4InfoReply      ICMPv4Type = 16
)

// Values for ICMP code as defined in RFC 792.
const (
	ICMPv4PortUnreachable     = 3
	ICMPv4FragmentationNeeded = 4
)

// Type is the ICMP type field.
func (b ICMPv4) Type() ICMPv4Type { return ICMPv4Type(b[0]) }

// SetType sets the ICMP type field.
func (b ICMPv4) SetType(t ICMPv4Type) { b[0] = byte(t) }

// Code is the ICMP code field. Its meaning depends on the value of Type.
func (b ICMPv4) Code() byte { return b[1] }

// SetCode sets the ICMP code field.
func (b ICMPv4) SetCode(c byte) { b[1] = c }

// Checksum is the ICMP checksum field.
func (b ICMPv4) Checksum() uint16 {
	return binary.BigEndian.Uint16(b[2:])
}

// SetChecksum sets the ICMP checksum field.
func (b ICMPv4) SetChecksum(checksum uint16) {
	binary.BigEndian.PutUint16(b[2:], checksum)
}

// SourcePort implements Transport.SourcePort.
func (ICMPv4) SourcePort() uint16 {
	return 0
}

// DestinationPort implements Transport.DestinationPort.
func (ICMPv4) DestinationPort() uint16 {
	return 0
}

// SetSourcePort implements Transport.SetSourcePort.
func (ICMPv4) SetSourcePort(uint16) {
}

// SetDestinationPort implements Transport.SetDestinationPort.
func (ICMPv4) SetDestinationPort(uint16) {
}

// Payload implements Transport.Payload.
func (b ICMPv4) Payload() []byte {
	return b[ICMPv4MinimumSize:]
}

