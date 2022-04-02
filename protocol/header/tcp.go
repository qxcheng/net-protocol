package header

import "github.com/qxcheng/net-protocol/protocol/seqnum"

// SACKBlock represents a single contiguous SACK block.
//
// +stateify savable
// 表示 sack 块的结构体
type SACKBlock struct {
	// Start indicates the lowest sequence number in the block.
	Start seqnum.Value

	// End indicates the sequence number immediately following the last
	// sequence number of this block.
	End seqnum.Value
}