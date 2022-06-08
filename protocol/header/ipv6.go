package header

import (
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"strings"
)

const (
	// IPv6MinimumSize is the minimum size of a valid IPv6 packet.
	IPv6MinimumSize = 40

	// IPv6AddressSize is the size, in bytes, of an IPv6 address.
	IPv6AddressSize = 16

	// IPv6ProtocolNumber is IPv6's network protocol number.
	IPv6ProtocolNumber tcpip.NetworkProtocolNumber = 0x86dd

	// IPv6Version is the version of the ipv6 protocol.
	IPv6Version = 6

	// IPv6MinimumMTU is the minimum MTU required by IPv6, per RFC 2460,
	// section 5.
	IPv6MinimumMTU = 1280
)

// IsV4MappedAddress determines if the provided address is an IPv4 mapped
// address by checking if its prefix is 0:0:0:0:0:ffff::/96.
func IsV4MappedAddress(addr tcpip.Address) bool {
	if len(addr) != IPv6AddressSize {
		return false
	}

	return strings.HasPrefix(string(addr), "\x00\x00\x00\x00\x00\x00\x00\x00\x00\x00\xff\xff")
}

// IsV6MulticastAddress determines if the provided address is an IPv6
// multicast address (anything starting with FF).
func IsV6MulticastAddress(addr tcpip.Address) bool {
	if len(addr) != IPv6AddressSize {
		return false
	}
	return addr[0] == 0xff
}