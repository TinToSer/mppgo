package cfb

import (
	"bytes"
	"fmt"
	"sort"
)

type streamPatch struct {
	offset int
	data   []byte
}

func PatchStreams(original []byte, replacements map[string][]byte) ([]byte, error) {
	file, err := Open(bytes.NewReader(original))
	if err != nil {
		return nil, err
	}
	if file.sectorSize != 512 || file.miniSectorSize != 64 {
		return nil, fmt.Errorf("cfb: patching requires 512-byte sectors and 64-byte mini sectors")
	}
	rootChain, err := file.sectorChain(file.Root.StartSect, file.fat)
	if err != nil {
		return nil, err
	}
	var patches []streamPatch
	for path, replacement := range replacements {
		entry, err := file.Lookup(path)
		if err != nil {
			return nil, err
		}
		if !entry.IsStream() || uint64(len(replacement)) != entry.Size {
			return nil, fmt.Errorf("cfb: %q must be a stream with unchanged size (%d bytes)", path, entry.Size)
		}
		stored, err := file.ReadStream(entry)
		if err != nil || len(stored) != len(replacement) {
			return nil, fmt.Errorf("cfb: truncated or invalid stream %q: %v", path, err)
		}
		unit := file.sectorSize
		allocation := file.fat
		mini := entry.Size < uint64(file.miniCutoff)
		if mini {
			unit, allocation = file.miniSectorSize, file.miniFAT
		}
		chain, err := file.sectorChain(entry.StartSect, allocation)
		if err != nil {
			return nil, err
		}
		position := 0
		for _, sector := range chain {
			if position == len(replacement) {
				break
			}
			count := min(unit, len(replacement)-position)
			offset := int64(headerSize) + int64(sector)*int64(file.sectorSize)
			if mini {
				miniOffset := uint64(sector) * uint64(file.miniSectorSize)
				rootIndex := miniOffset / uint64(file.sectorSize)
				if miniOffset+uint64(count) > file.Root.Size || rootIndex >= uint64(len(rootChain)) {
					return nil, fmt.Errorf("cfb: mini stream %q exceeds root stream", path)
				}
				offset = int64(headerSize) + int64(rootChain[rootIndex])*int64(file.sectorSize) + int64(miniOffset%uint64(file.sectorSize))
			}
			if offset < 0 || offset+int64(count) > int64(len(original)) {
				return nil, fmt.Errorf("cfb: stream %q exceeds file bounds", path)
			}
			patches = append(patches, streamPatch{offset: int(offset), data: replacement[position : position+count]})
			position += count
		}
		if position != len(replacement) {
			return nil, fmt.Errorf("cfb: incomplete allocation chain for %q", path)
		}
	}
	sort.Slice(patches, func(left, right int) bool { return patches[left].offset < patches[right].offset })
	for index := 1; index < len(patches); index++ {
		if patches[index].offset < patches[index-1].offset+len(patches[index-1].data) {
			return nil, fmt.Errorf("cfb: overlapping stream allocations")
		}
	}
	output := bytes.Clone(original)
	for _, patch := range patches {
		copy(output[patch.offset:], patch.data)
	}
	verified, err := Open(bytes.NewReader(output))
	if err != nil {
		return nil, fmt.Errorf("cfb: patched container verification: %w", err)
	}
	for path, expected := range replacements {
		actual, err := verified.OpenStream(path)
		if err != nil || !bytes.Equal(actual, expected) {
			return nil, fmt.Errorf("cfb: patched stream verification failed for %q", path)
		}
	}
	return output, nil
}

func (file *File) sectorChain(start uint32, allocation []uint32) ([]uint32, error) {
	var chain []uint32
	seen := make(map[uint32]bool)
	for sector := start; sector != endOfChain && sector != freeSect; sector = allocation[sector] {
		if uint64(sector) >= uint64(len(allocation)) || seen[sector] {
			return nil, fmt.Errorf("cfb: invalid or cyclic allocation chain at sector %d", sector)
		}
		seen[sector] = true
		chain = append(chain, sector)
	}
	return chain, nil
}
