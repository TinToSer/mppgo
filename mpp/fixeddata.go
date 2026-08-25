// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

// FixedData represents a "FixedData" stream: a sequence of items whose
// locations within the stream are described by a companion FixedMeta block
// (byte offset 4 of each FixedMeta record, as an int32).
type FixedData struct {
	items [][]byte
}

// ParseFixedData parses a FixedData stream using offsets described by meta.
// maxExpectedSize (0 = unbounded) caps how large a single item can be taken
// to be, guarding against corrupt/out-of-order offset data; minSize is the
// size used when a computed item size comes out as zero.
//
// Items alias the supplied buffer rather than copying it, so data must not
// be modified afterwards.
func ParseFixedData(meta *FixedMeta, data []byte, maxExpectedSize, minSize int) *FixedData {
	if meta == nil {
		return &FixedData{}
	}
	itemCount := meta.AdjustedItemCount
	fd := &FixedData{items: make([][]byte, itemCount)}

	for i := 0; i < itemCount; i++ {
		metaData := meta.ByteArrayValue(i)
		if metaData == nil || len(metaData) < 8 {
			continue
		}
		itemOffset := getInt(metaData, 4)
		if itemOffset < 0 || itemOffset > len(data) {
			continue
		}

		// An item runs from its own offset to the next item's offset; the
		// last item runs to the end of the block.
		var itemSize int
		if next := meta.ByteArrayValue(i + 1); i+1 < itemCount && len(next) >= 8 {
			itemSize = getInt(next, 4) - itemOffset
		} else {
			itemSize = len(data) - itemOffset
		}

		if itemSize == 0 {
			itemSize = minSize
		}

		available := len(data) - itemOffset
		if itemSize < 0 || itemSize > available {
			if maxExpectedSize == 0 {
				itemSize = available
			} else if maxExpectedSize < available {
				itemSize = maxExpectedSize
			} else {
				itemSize = available
			}
		}
		if maxExpectedSize != 0 && itemSize > maxExpectedSize {
			itemSize = maxExpectedSize
		}

		if itemSize > 0 {
			fd.items[i] = data[itemOffset : itemOffset+itemSize]
		}
	}
	return fd
}

// ParseFixedDataFixedSize parses a FixedData stream whose item size is
// known in advance rather than derived from a FixedMeta block. Used where
// the metadata block is known to be unreliable.
//
// Items alias the supplied buffer rather than copying it.
func ParseFixedDataFixedSize(data []byte, itemSize int) *FixedData {
	if itemSize <= 0 {
		return &FixedData{}
	}
	itemCount := len(data) / itemSize
	fd := &FixedData{items: make([][]byte, itemCount)}
	for i := 0; i < itemCount; i++ {
		start := i * itemSize
		fd.items[i] = data[start : start+itemSize]
	}
	return fd
}

// ByteArrayValue returns the item at the given index, or nil.
func (d *FixedData) ByteArrayValue(index int) []byte {
	if index < 0 || index >= len(d.items) {
		return nil
	}
	return d.items[index]
}

// ItemCount returns the number of items held in this block.
func (d *FixedData) ItemCount() int { return len(d.items) }
