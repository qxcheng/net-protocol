package header

import (
	"encoding/binary"
	tcpip "github.com/qxcheng/net-protocol/protocol"
	"github.com/qxcheng/net-protocol/protocol/seqnum"
)

const (
	srcPort     = 0
	dstPort     = 2
	seqNum      = 4
	ackNum      = 8
	dataOffset  = 12
	tcpFlags    = 13
	winSize     = 14
	tcpChecksum = 16
	urgentPtr   = 18
)

const (
	// MaxWndScale is maximum allowed window scaling, as described in
	// RFC 1323, section 2.3, page 11.
	MaxWndScale = 14

	// TCPMaxSACKBlocks is the maximum number of SACK blocks that can
	// be encoded in a TCP option field.
	TCPMaxSACKBlocks = 4
)

// Flags标志位
const (
	TCPFlagFin = 1 << iota
	TCPFlagSyn
	TCPFlagRst
	TCPFlagPsh
	TCPFlagAck
	TCPFlagUrg
)

// TCP选项
const (
	TCPOptionEOL           = 0
	TCPOptionNOP           = 1
	TCPOptionMSS           = 2
	TCPOptionWS            = 3
	TCPOptionTS            = 8
	TCPOptionSACKPermitted = 4
	TCPOptionSACK          = 5
)

// TCPFields tcp首部字段
type TCPFields struct {
	SrcPort uint16
	DstPort uint16
	SeqNum uint32
	AckNum uint32
	DataOffset uint8  // 即首部长度
	Flags uint8
	WindowSize uint16
	Checksum uint16
	UrgentPointer uint16
}

// TCPSynOptions 用于返回解析后的syn报文的选项
type TCPSynOptions struct {
	// MSS is the maximum segment size provided by the peer in the SYN.
	MSS uint16

	// WS is the window scale option provided by the peer in the SYN.
	// Set to -1 if no window scale option was provided.
	WS int

	// TS is true if the timestamp option was provided in the syn/syn-ack.
	TS bool
	TSVal uint32
	TSEcr uint32

	// SACKPermitted is true if the SACK option was provided in the SYN/SYN-ACK.
	SACKPermitted bool
}

// SACKBlock 表示一个连续的 sack 块
type SACKBlock struct {
	// Start indicates the lowest sequence number in the block.
	Start seqnum.Value

	// End indicates the sequence number immediately following the last
	// sequence number of this block.
	End seqnum.Value
}

// TCPOptions tcp选项结构，不用于 syn/syn-ack 报文
type TCPOptions struct {
	// TS is true if the TimeStamp option is enabled.
	TS bool
	TSVal uint32
	TSEcr uint32
	SACKBlocks []SACKBlock
}

// TCP represents a TCP header stored in a byte array.
type TCP []byte

const (
	TCPMinimumSize = 20
	TCPProtocolNumber tcpip.TransportProtocolNumber = 6
)

func (b TCP) SourcePort() uint16 {
	return binary.BigEndian.Uint16(b[srcPort:])
}

func (b TCP) DestinationPort() uint16 {
	return binary.BigEndian.Uint16(b[dstPort:])
}

func (b TCP) SequenceNumber() uint32 {
	return binary.BigEndian.Uint32(b[seqNum:])
}

func (b TCP) AckNumber() uint32 {
	return binary.BigEndian.Uint32(b[ackNum:])
}

func (b TCP) DataOffset() uint8 {
	return (b[dataOffset] >> 4) * 4  // 返回的单位是字节
}

func (b TCP) Payload() []byte {
	return b[b.DataOffset():]
}

func (b TCP) Flags() uint8 {
	return b[tcpFlags]
}

func (b TCP) WindowSize() uint16 {
	return binary.BigEndian.Uint16(b[winSize:])
}

func (b TCP) Checksum() uint16 {
	return binary.BigEndian.Uint16(b[tcpChecksum:])
}

func (b TCP) Options() []byte {
	return b[TCPMinimumSize:b.DataOffset()]
}

func (b TCP) ParsedOptions() TCPOptions {
	return ParseTCPOptions(b.Options())
}

// ParseSynOptions 解析SYN报文的选项字段
func ParseSynOptions(opts []byte, isAck bool) TCPSynOptions {
	limit := len(opts)

	synOpts := TCPSynOptions{
		MSS: 536,
		WS: -1,
	}

	for i := 0; i < limit; {
		switch opts[i] {
		case TCPOptionEOL: // 直接跳出循环
			i = limit
		case TCPOptionNOP: // 指向下一个字节
			i++
		case TCPOptionMSS:
			if i+4 > limit || opts[i+1] != 4 {
				return synOpts
			}
			mss := uint16(opts[i+2])<<8 | uint16(opts[i+3])
			if mss == 0 {
				return synOpts
			}
			synOpts.MSS = mss
			i += 4

		case TCPOptionWS:
			if i+3 > limit || opts[i+1] != 3 {
				return synOpts
			}
			ws := int(opts[i+2])
			if ws > MaxWndScale {
				ws = MaxWndScale
			}
			synOpts.WS = ws
			i += 3

		case TCPOptionTS:
			if i+10 > limit || opts[i+1] != 10 {
				return synOpts
			}
			synOpts.TSVal = binary.BigEndian.Uint32(opts[i+2:])
			if isAck {
				// If the segment is a SYN-ACK then store the Timestamp Echo Reply
				// in the segment.
				synOpts.TSEcr = binary.BigEndian.Uint32(opts[i+6:])
			}
			synOpts.TS = true
			i += 10
		case TCPOptionSACKPermitted:
			if i+2 > limit || opts[i+1] != 2 {
				return synOpts
			}
			synOpts.SACKPermitted = true
			i += 2

		default:
			// We don't recognize this option, just skip over it.
			if i+2 > limit {
				return synOpts
			}
			l := int(opts[i+1])
			// If the length is incorrect or if l+i overflows the
			// total options length then return false.
			if l < 2 || i+l > limit {
				return synOpts
			}
			i += l
		}
	}

	return synOpts
}

// ParseTCPOptions 解析TCP报文的选项
func ParseTCPOptions(b []byte) TCPOptions {
	opts := TCPOptions{}
	limit := len(b)
	for i := 0; i < limit; {
		switch b[i] {
		case TCPOptionEOL:
			i = limit
		case TCPOptionNOP:
			i++
		case TCPOptionTS:
			if i+10 > limit || (b[i+1] != 10) {
				return opts
			}
			opts.TS = true
			opts.TSVal = binary.BigEndian.Uint32(b[i+2:])
			opts.TSEcr = binary.BigEndian.Uint32(b[i+6:])
			i += 10
		case TCPOptionSACK:
			if i+2 > limit {
				// Malformed SACK block, just return and stop parsing.
				return opts
			}
			sackOptionLen := int(b[i+1])
			if i+sackOptionLen > limit || (sackOptionLen-2)%8 != 0 {
				// Malformed SACK block, just return and stop parsing.
				return opts
			}
			numBlocks := (sackOptionLen - 2) / 8
			opts.SACKBlocks = []SACKBlock{}
			for j := 0; j < numBlocks; j++ {
				start := binary.BigEndian.Uint32(b[i+2+j*8:])
				end := binary.BigEndian.Uint32(b[i+2+j*8+4:])
				opts.SACKBlocks = append(opts.SACKBlocks, SACKBlock{
					Start: seqnum.Value(start),
					End:   seqnum.Value(end),
				})
			}
			i += sackOptionLen
		default:
			// We don't recognize this option, just skip over it.
			if i+2 > limit {
				return opts
			}
			l := int(b[i+1])
			// If the length is incorrect or if l+i overflows the
			// total options length then return false.
			if l < 2 || i+l > limit {
				return opts
			}
			i += l
		}
	}
	return opts
}


func (b TCP) SetSourcePort(port uint16) {
	binary.BigEndian.PutUint16(b[srcPort:], port)
}

func (b TCP) SetDestinationPort(port uint16) {
	binary.BigEndian.PutUint16(b[dstPort:], port)
}

func (b TCP) SetChecksum(checksum uint16) {
	binary.BigEndian.PutUint16(b[tcpChecksum:], checksum)
}

func (b TCP) CalculateChecksum(partialChecksum uint16, totalLen uint16) uint16 {
	// Add the length portion of the checksum to the pseudo-checksum.
	tmp := make([]byte, 2)
	binary.BigEndian.PutUint16(tmp, totalLen)
	checksum := Checksum(tmp, partialChecksum)

	// Calculate the rest of the checksum.
	return Checksum(b[:b.DataOffset()], checksum)
}

func (b TCP) encodeSubset(seq, ack uint32, flags uint8, rcvwnd uint16) {
	binary.BigEndian.PutUint32(b[seqNum:], seq)
	binary.BigEndian.PutUint32(b[ackNum:], ack)
	b[tcpFlags] = flags
	binary.BigEndian.PutUint16(b[winSize:], rcvwnd)
}

func (b TCP) Encode(t *TCPFields) {
	b.encodeSubset(t.SeqNum, t.AckNum, t.Flags, t.WindowSize)
	binary.BigEndian.PutUint16(b[srcPort:], t.SrcPort)
	binary.BigEndian.PutUint16(b[dstPort:], t.DstPort)
	b[dataOffset] = (t.DataOffset / 4) << 4
	binary.BigEndian.PutUint16(b[tcpChecksum:], t.Checksum)
	binary.BigEndian.PutUint16(b[urgentPtr:], t.UrgentPointer)
}

// EncodePartial updates a subset of the fields of the tcp header. It is useful
// in cases when similar segments are produced.
func (b TCP) EncodePartial(partialChecksum, length uint16, seqnum, acknum uint32, flags byte, rcvwnd uint16) {
	// Add the total length and "flags" field contributions to the checksum.
	// We don't use the flags field directly from the header because it's a
	// one-byte field with an odd offset, so it would be accounted for
	// incorrectly by the Checksum routine.
	tmp := make([]byte, 4)
	binary.BigEndian.PutUint16(tmp, length)
	binary.BigEndian.PutUint16(tmp[2:], uint16(flags))
	checksum := Checksum(tmp, partialChecksum)

	// Encode the passed-in fields.
	b.encodeSubset(seqnum, acknum, flags, rcvwnd)

	// Add the contributions of the passed-in fields to the checksum.
	checksum = Checksum(b[seqNum:seqNum+8], checksum)
	checksum = Checksum(b[winSize:winSize+2], checksum)

	// Encode the checksum.
	b.SetChecksum(^checksum)
}

func EncodeMSSOption(mss uint32, b []byte) int {
	// mssOptionSize is the number of bytes in a valid MSS option.
	const mssOptionSize = 4

	if len(b) < mssOptionSize {
		return 0
	}
	b[0], b[1], b[2], b[3] = TCPOptionMSS, mssOptionSize, byte(mss>>8), byte(mss)
	return mssOptionSize
}

func EncodeWSOption(ws int, b []byte) int {
	if len(b) < 3 {
		return 0
	}
	b[0], b[1], b[2] = TCPOptionWS, 3, uint8(ws)
	return int(b[1])
}

func EncodeTSOption(tsVal, tsEcr uint32, b []byte) int {
	if len(b) < 10 {
		return 0
	}
	b[0], b[1] = TCPOptionTS, 10
	binary.BigEndian.PutUint32(b[2:], tsVal)
	binary.BigEndian.PutUint32(b[6:], tsEcr)
	return int(b[1])
}

func EncodeSACKPermittedOption(b []byte) int {
	if len(b) < 2 {
		return 0
	}

	b[0], b[1] = TCPOptionSACKPermitted, 2
	return int(b[1])
}

func EncodeSACKBlocks(sackBlocks []SACKBlock, b []byte) int {
	if len(sackBlocks) == 0 {
		return 0
	}
	l := len(sackBlocks)
	if l > TCPMaxSACKBlocks {
		l = TCPMaxSACKBlocks
	}
	if ll := (len(b) - 2) / 8; ll < l {
		l = ll
	}
	if l == 0 {
		// There is not enough space in the provided buffer to add
		// any SACK blocks.
		return 0
	}
	b[0] = TCPOptionSACK
	b[1] = byte(l*8 + 2)
	for i := 0; i < l; i++ {
		binary.BigEndian.PutUint32(b[i*8+2:], uint32(sackBlocks[i].Start))
		binary.BigEndian.PutUint32(b[i*8+6:], uint32(sackBlocks[i].End))
	}
	return int(b[1])
}

func EncodeNOP(b []byte) int {
	if len(b) == 0 {
		return 0
	}
	b[0] = TCPOptionNOP
	return 1
}

func AddTCPOptionPadding(options []byte, offset int) int {
	paddingToAdd := -offset & 3
	// Now add any padding bytes that might be required to quad align the
	// options.
	for i := offset; i < offset+paddingToAdd; i++ {
		options[i] = TCPOptionNOP
	}
	return paddingToAdd
}

