// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"encoding/binary"
	"fmt"
	"sort"
)

const varMetaMagic = 0xFADFADBA

// VarMeta represents a "VarMeta" stream (the VarMeta12 layout, used
// throughout MPP12/MPP14): metadata describing where each variable-length
// field lives in a companion Var2Data stream, keyed by (uniqueID, type).
type VarMeta struct {
	ItemCount int
	DataSize  int

	table   map[int]map[int]int // uniqueID -> type -> offset
	offsets []int               // all offsets, in file order (dupes possible)
}

// ParseVarMeta parses a VarMeta12-format stream.
func ParseVarMeta(data []byte) (*VarMeta, error) {
	const headerSize = 24
	if len(data) < headerSize {
		return nil, fmt.Errorf("mpp: VarMeta stream too short (%d bytes)", len(data))
	}
	magic := binary.LittleEndian.Uint32(data[0:4])
	if magic != 0 && magic != varMetaMagic {
		return nil, fmt.Errorf("mpp: VarMeta bad magic number: %#x", magic)
	}
	itemCount := int(int32(binary.LittleEndian.Uint32(data[8:12])))
	dataSize := int(int32(binary.LittleEndian.Uint32(data[20:24])))
	if itemCount < 0 {
		return nil, fmt.Errorf("mpp: VarMeta negative item count %d", itemCount)
	}

	// The declared item count cannot exceed what the stream can actually
	// hold; trusting it would let a corrupt header drive a huge allocation.
	if max := (len(data) - headerSize) / 12; itemCount > max {
		itemCount = max
	}

	vm := &VarMeta{
		ItemCount: itemCount,
		DataSize:  dataSize,
		table:     make(map[int]map[int]int),
		offsets:   make([]int, 0, itemCount),
	}

	pos := headerSize
	for i := 0; i < itemCount; i++ {
		if len(data)-pos < 12 {
			break
		}
		uniqueID := getInt(data, pos)
		offset := getInt(data, pos+4)
		typ := getShort(data, pos+8)
		pos += 12

		m, ok := vm.table[uniqueID]
		if !ok {
			m = make(map[int]int)
			vm.table[uniqueID] = m
		}
		m[typ] = offset
		vm.offsets = append(vm.offsets, offset)
	}
	sort.Ints(vm.offsets)
	return vm, nil
}

// Offset returns the Var2Data stream offset for the given (uniqueID, type)
// pair, and whether it was found.
func (m *VarMeta) Offset(id, typ int) (int, bool) {
	sub, ok := m.table[id]
	if !ok {
		return 0, false
	}
	off, ok := sub[typ]
	return off, ok
}

// Types returns the set of field-type identifiers stored for a given unique ID.
func (m *VarMeta) Types(id int) []int {
	sub, ok := m.table[id]
	if !ok {
		return nil
	}
	result := make([]int, 0, len(sub))
	for t := range sub {
		result = append(result, t)
	}
	return result
}

// Offsets returns all item offsets in ascending order (may contain duplicates
// where entries share deduplicated var data).
func (m *VarMeta) Offsets() []int { return m.offsets }
