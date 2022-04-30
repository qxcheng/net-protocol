package header

import (
	"encoding/binary"
	tcpip "github.com/qxcheng/net-protocol/protocol"
)

const (
	versIHL  = 0
	tos      = 1
	totalLen = 2
	id       = 4
	flagsFO  = 6
	ttl      = 8
	protocol = 9
	checksum = 10
	srcAddr  = 12
	dstAddr  = 16
)

const (
	IPv4MinimumSize = 20        // ipv4包的最小尺寸
	IPv4MaximumHeaderSize = 60  // ipv4头的最大长度，用4个二进制位(1111=15)、单位4字节表示头长度
	IPv4AddressSize = 4         // ipv4地址的长度（字节）
	IPv4ProtocolNumber tcpip.NetworkProtocolNumber = 0x0800  // ipv4网络层协议号
	IPv4Version = 4  // ipv4协议的版本
	IPv4Broadcast tcpip.Address = "\xff\xff\xff\xff"         // ipv4协议的广播地址

	// IPv4Any is the non-routable IPv4 "any" meta address.
	IPv4Any tcpip.Address = "\x00\x00\x00\x00"
)

// Flags that may be set in an IPv4 packet.
const (
	IPv4FlagMoreFragments = 1 << iota
	IPv4FlagDontFragment
)

// IPv4Fields 表示IPv4头部信息的结构体
type IPv4Fields struct {
	IHL uint8              // 头部长度
	TOS uint8              // 服务区分的表示
	TotalLength uint16     // 数据报文总长
	ID uint16              // 标识符
	Flags uint8            // 标签
	FragmentOffset uint16  // 分片偏移
	TTL uint8              // 存活时间
	Protocol uint8         // 表示的传输层协议
	Checksum uint16        // 首部校验和
	SrcAddr tcpip.Address  // 源IP地址
	DstAddr tcpip.Address  // 目的IP地址
}

// IPv4 表示ipv4头
type IPv4 []byte


// IPVersion 返回ipv4版本
func IPVersion(b []byte) int {
	if len(b) < versIHL+1 {
		return -1
	}
	return int(b[versIHL] >> 4)
}


// HeaderLength 返回头长度（单位字节）
func (b IPv4) HeaderLength() uint8 {
	return (b[versIHL] & 0xf) * 4
}

// TOS 返回服务类型
func (b IPv4) TOS() (uint8, uint32) {
	return b[tos], 0
}

// TotalLength 包的总长度
func (b IPv4) TotalLength() uint16 {
	return binary.BigEndian.Uint16(b[totalLen:])
}

// ID 返回标识符Identifier
func (b IPv4) ID() uint16 {
	return binary.BigEndian.Uint16(b[id:])
}

// Flags 返回标志位
func (b IPv4) Flags() uint8 {
	return uint8(binary.BigEndian.Uint16(b[flagsFO:]) >> 13)
}

// FragmentOffset 返回分片的偏移量
func (b IPv4) FragmentOffset() uint16 {
	return binary.BigEndian.Uint16(b[flagsFO:]) << 3
}

// TTL 返回TTL
func (b IPv4) TTL() uint8 {
	return b[ttl]
}

// Protocol 返回上层协议
func (b IPv4) Protocol() uint8 {
	return b[protocol]
}

// Checksum 头部的校验和
func (b IPv4) Checksum() uint16 {
	return binary.BigEndian.Uint16(b[checksum:])
}

// SourceAddress 源地址
func (b IPv4) SourceAddress() tcpip.Address {
	return tcpip.Address(b[srcAddr : srcAddr+IPv4AddressSize])
}

// DestinationAddress 目的地址
func (b IPv4) DestinationAddress() tcpip.Address {
	return tcpip.Address(b[dstAddr : dstAddr+IPv4AddressSize])
}


// TransportProtocol 返回传输层协议号
func (b IPv4) TransportProtocol() tcpip.TransportProtocolNumber {
	return tcpip.TransportProtocolNumber(b.Protocol())
}

// Payload 返回IP包的数据
func (b IPv4) Payload() []byte {
	return b[b.HeaderLength():][:b.PayloadLength()]
}

// PayloadLength 返回ip包数据的长度
func (b IPv4) PayloadLength() uint16 {
	return b.TotalLength() - uint16(b.HeaderLength())
}


func (b IPv4) SetTOS(v uint8, _ uint32) {
	b[tos] = v
}

func (b IPv4) SetTotalLength(totalLength uint16) {
	binary.BigEndian.PutUint16(b[totalLen:], totalLength)
}

// CalculateChecksum 计算ip头的检验和
func (b IPv4) CalculateChecksum() uint16 {
	return Checksum(b[:b.HeaderLength()], 0)
}

func (b IPv4) SetChecksum(v uint16) {
	binary.BigEndian.PutUint16(b[checksum:], v)
}

func (b IPv4) SetFlagsFragmentOffset(flags uint8, offset uint16) {
	v := (uint16(flags) << 13) | (offset >> 3)  // offset应该需要先左移3位
	binary.BigEndian.PutUint16(b[flagsFO:], v)
}

func (b IPv4) SetSourceAddress(addr tcpip.Address) {
	copy(b[srcAddr:srcAddr+IPv4AddressSize], addr)
}

func (b IPv4) SetDestinationAddress(addr tcpip.Address) {
	copy(b[dstAddr:dstAddr+IPv4AddressSize], addr)
}

// Encode 组装ip头
func (b IPv4) Encode(i *IPv4Fields) {
	b[versIHL] = (4 << 4) | ((i.IHL / 4) & 0xf)
	b[tos] = i.TOS
	b.SetTotalLength(i.TotalLength)
	binary.BigEndian.PutUint16(b[id:], i.ID)
	b.SetFlagsFragmentOffset(i.Flags, i.FragmentOffset)
	b[ttl] = i.TTL
	b[protocol] = i.Protocol
	b.SetChecksum(i.Checksum)
	copy(b[srcAddr:srcAddr+IPv4AddressSize], i.SrcAddr)
	copy(b[dstAddr:dstAddr+IPv4AddressSize], i.DstAddr)
}

// EncodePartial updates the total length and checksum fields of ipv4 header,
// taking in the partial checksum, which is the checksum of the header without
// the total length and checksum fields. It is useful in cases when similar
// packets are produced.
func (b IPv4) EncodePartial(partialChecksum, totalLength uint16) {
	b.SetTotalLength(totalLength)
	checksum := Checksum(b[totalLen:totalLen+2], partialChecksum)
	b.SetChecksum(^checksum)
}

// IsValid 校验包是否合法
func (b IPv4) IsValid(pktSize int) bool {
	if len(b) < IPv4MinimumSize {
		return false
	}

	hlen := int(b.HeaderLength())
	tlen := int(b.TotalLength())
	if hlen > tlen || tlen > pktSize {
		return false
	}

	return true
}

// IsV4MulticastAddress determines if the provided address is an IPv4 multicast
// address (range 224.0.0.0 to 239.255.255.255). The four most significant bits
// will be 1110 = 0xe0.
func IsV4MulticastAddress(addr tcpip.Address) bool {
	if len(addr) != IPv4AddressSize {
		return false
	}
	return (addr[0] & 0xf0) == 0xe0
}








