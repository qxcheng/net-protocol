package ipv6

import "github.com/qxcheng/net-protocol/protocol/header"

const (
	// ProtocolName is the string representation of the ipv6 protocol name.
	ProtocolName = "ipv6"

	// ProtocolNumber is the ipv6 protocol number.
	ProtocolNumber = header.IPv6ProtocolNumber

	// maxTotalSize is maximum size that can be encoded in the 16-bit
	// PayloadLength field of the ipv6 header.
	maxPayloadSize = 0xffff

	// defaultIPv6HopLimit is the default hop limit for IPv6 Packets
	// egressed by Netstack.
	defaultIPv6HopLimit = 255
)
