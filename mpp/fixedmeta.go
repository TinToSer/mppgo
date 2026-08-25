// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"encoding/binary"
	"fmt"
)

const (
	fixedMetaMagic      = 0xFADFADBA
	fixedMetaHeaderSize = 16
)

// FixedMeta represents a "FixedMeta" stream: a 16-byte header followed by
// fixed-size opaque metadata records, one per item in a companion FixedData
// stream. Byte 4 of each record (as an int32) is the byte offset of the
// corresponding item within the FixedData stream.
type FixedMeta struct {
	ItemCount         int // as reported in the header (not always reliable)
	AdjustedItemCount int // derived from stream size / item size
	items             [][]byte
}

// ParseFixedMeta parses a FixedMeta stream given a known/assumed item size.
func ParseFixedMeta(data []byte, itemSize int) (*FixedMeta, error) {
	return parseFixedMetaWithSize(data, itemSize)
}

// ParseFixedMetaHeuristic parses a FixedMeta stream by picking the best fit
// among candidate item sizes, cross-checking against the item count of an
// already-parsed companion FixedData block (mirroring MPXJ's heuristic
// FixedMeta constructor, used for some "Fixed2Meta" streams).
func ParseFixedMetaHeuristic(data []byte, otherItemCount int, itemSizes ...int) (*FixedMeta, error) {
	if len(itemSizes) == 0 {
		return nil, fmt.Errorf("mpp: no candidate item sizes supplied")
	}
	if len(data) < fixedMetaHeaderSize {
		return nil, fmt.Errorf("mpp: FixedMeta stream too short (%d bytes)", len(data))
	}
	itemCount := int(int32(binary.LittleEndian.Uint32(data[8:12])))
	available := len(data) - fixedMetaHeaderSize

	itemSize := itemSizes[0]
	distance := -1 << 31
	for _, candidate := range itemSizes {
		if candidate <= 0 || available%candidate != 0 {
			continue
		}
		if available/candidate == otherItemCount {
			itemSize = candidate
			distance = 0
			break
		}
		testDistance := itemCount*candidate - available
		if testDistance <= 0 && testDistance > distance {
			itemSize = candidate
			distance = testDistance
		}
	}
	return parseFixedMetaWithSize(data, itemSize)
}

func parseFixedMetaWithSize(data []byte, itemSize int) (*FixedMeta, error) {
	if len(data) < fixedMetaHeaderSize {
		return nil, fmt.Errorf("mpp: FixedMeta stream too short (%d bytes)", len(data))
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != fixedMetaMagic {
		return nil, fmt.Errorf("mpp: FixedMeta bad magic number: %#x", magic)
	}
	itemCount := int(int32(binary.LittleEndian.Uint32(data[8:12])))

	if itemSize <= 0 {
		return nil, fmt.Errorf("mpp: FixedMeta invalid item size %d", itemSize)
	}

	fm := &FixedMeta{ItemCount: itemCount}
	fm.AdjustedItemCount = (len(data) - fixedMetaHeaderSize) / itemSize
	body := data[fixedMetaHeaderSize:]
	fm.items = make([][]byte, fm.AdjustedItemCount)
	for i := 0; i < fm.AdjustedItemCount; i++ {
		start := i * itemSize
		fm.items[i] = body[start : start+itemSize]
	}
	return fm, nil
}

// ByteArrayValue returns the raw metadata record at the given index, or nil
// if the index is out of range.
func (m *FixedMeta) ByteArrayValue(index int) []byte {
	if index < 0 || index >= len(m.items) {
		return nil
	}
	return m.items[index]
}
