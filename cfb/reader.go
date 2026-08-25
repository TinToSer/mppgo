// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

// Package cfb reads Microsoft's Compound File Binary (OLE2 / "structured
// storage") container format — the outer envelope used by legacy Office
// documents including .mpp. The format is public (MS-CFB) and unrelated to
// MPXJ's proprietary knowledge of the .mpp payload itself.
package cfb

import (
	"encoding/binary"
	"errors"
	"fmt"
	"io"
	"os"
	"strings"
	"unicode/utf16"
)

const (
	headerSize         = 512
	difatInHeaderCount = 109

	freeSect   = 0xFFFFFFFF
	endOfChain = 0xFFFFFFFE
	fatSect    = 0xFFFFFFFD
	difSect    = 0xFFFFFFFC

	dirEntrySize = 128
	noStream     = 0xFFFFFFFF

	objTypeUnknown = 0
	objTypeStorage = 1
	objTypeStream  = 2
	objTypeRoot    = 5
)

var signature = [8]byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1}

// maxAllocHint caps capacity hints derived from file-supplied counts, so a
// corrupt header cannot trigger a huge speculative allocation. Slices still
// grow to whatever the file legitimately needs.
const maxAllocHint = 1 << 16

func clampAlloc(n int) int {
	if n < 0 {
		return 0
	}
	if n > maxAllocHint {
		return maxAllocHint
	}
	return n
}

// readerSize reports the total length of r when that is cheaply available.
// *os.File, *bytes.Reader and *strings.Reader all qualify.
func readerSize(r io.ReaderAt) (int64, bool) {
	switch v := r.(type) {
	case interface{ Size() int64 }: // *bytes.Reader, *strings.Reader
		return v.Size(), true
	case interface{ Stat() (os.FileInfo, error) }: // *os.File
		info, err := v.Stat()
		if err != nil {
			return 0, false
		}
		return info.Size(), true
	}
	return 0, false
}

// Entry is a single directory entry (a storage or a stream).
type Entry struct {
	Name      string
	Type      byte // objTypeStorage, objTypeStream, objTypeRoot
	StartSect uint32
	Size      uint64
	Children  map[string]*Entry // populated for storages/root
}

// IsStorage reports whether the entry is a storage (directory-like) node.
func (e *Entry) IsStorage() bool { return e.Type == objTypeStorage || e.Type == objTypeRoot }

// IsStream reports whether the entry is a stream (file-like) node.
func (e *Entry) IsStream() bool { return e.Type == objTypeStream }

// File represents an opened compound file.
type File struct {
	r              io.ReaderAt
	sectorSize     int
	miniSectorSize int
	miniCutoff     uint32

	fat     []uint32 // sector id -> next sector id (or END_OF_CHAIN/FREESECT)
	miniFAT []uint32

	miniStream []byte // concatenated mini-stream sectors (read from root entry)

	Root *Entry
}

// Open parses the compound file container from r, which must provide the
// full extent of the file (an *os.File satisfies this).
func Open(r io.ReaderAt) (*File, error) {
	hdr := make([]byte, headerSize)
	if _, err := r.ReadAt(hdr, 0); err != nil {
		return nil, fmt.Errorf("cfb: reading header: %w", err)
	}
	var sig [8]byte
	copy(sig[:], hdr[0:8])
	if sig != signature {
		return nil, errors.New("cfb: not a compound file (bad signature)")
	}

	sectorShift := binary.LittleEndian.Uint16(hdr[30:32])
	miniSectorShift := binary.LittleEndian.Uint16(hdr[32:34])
	numFATSectors := binary.LittleEndian.Uint32(hdr[44:48])
	firstDirSector := binary.LittleEndian.Uint32(hdr[48:52])
	miniCutoff := binary.LittleEndian.Uint32(hdr[56:60])
	firstMiniFATSector := binary.LittleEndian.Uint32(hdr[60:64])
	numMiniFATSectors := binary.LittleEndian.Uint32(hdr[64:68])
	firstDIFATSector := binary.LittleEndian.Uint32(hdr[68:72])
	numDIFATSectors := binary.LittleEndian.Uint32(hdr[72:76])

	// Validate the shift values before using them as allocation sizes: the
	// spec allows 9 (512-byte sectors) or 12 (4096), and 6 for mini
	// sectors. Accepting a small range around those keeps us tolerant of
	// odd-but-readable files while refusing a corrupt header that would
	// otherwise drive a multi-gigabyte allocation.
	if sectorShift < 7 || sectorShift > 20 {
		return nil, fmt.Errorf("cfb: implausible sector shift %d", sectorShift)
	}
	if miniSectorShift < 4 || miniSectorShift > sectorShift {
		return nil, fmt.Errorf("cfb: implausible mini sector shift %d", miniSectorShift)
	}

	f := &File{
		r:              r,
		sectorSize:     1 << sectorShift,
		miniSectorSize: 1 << miniSectorShift,
		miniCutoff:     miniCutoff,
	}

	// Sector counts in the header are used to size reads, so they are
	// cross-checked against the actual file length where it is known. A
	// corrupt count would otherwise drive a huge allocation before the
	// first short read is hit.
	if size, ok := readerSize(r); ok {
		maxSectors := size / int64(f.sectorSize)
		if int64(numFATSectors) > maxSectors+1 {
			return nil, fmt.Errorf("cfb: FAT sector count %d exceeds file size", numFATSectors)
		}
		if int64(numDIFATSectors) > maxSectors+1 {
			return nil, fmt.Errorf("cfb: DIFAT sector count %d exceeds file size", numDIFATSectors)
		}
		if int64(numMiniFATSectors) > maxSectors+1 {
			return nil, fmt.Errorf("cfb: mini FAT sector count %d exceeds file size", numMiniFATSectors)
		}
	}

	// Build the full list of FAT sector locations: 109 in the header,
	// plus any chained through DIFAT sectors. Capacity hints are clamped
	// so that an unvalidated count cannot request an enormous allocation;
	// append grows the slice as genuinely needed.
	fatSectorLocs := make([]uint32, 0, clampAlloc(int(numFATSectors)))
	for i := 0; i < difatInHeaderCount && len(fatSectorLocs) < int(numFATSectors); i++ {
		off := 76 + i*4
		loc := binary.LittleEndian.Uint32(hdr[off : off+4])
		if loc == freeSect {
			break
		}
		fatSectorLocs = append(fatSectorLocs, loc)
	}
	difatSector := firstDIFATSector
	for i := uint32(0); i < numDIFATSectors && difatSector != endOfChain && difatSector != freeSect; i++ {
		buf, err := f.readRawSector(difatSector)
		if err != nil {
			return nil, err
		}
		entriesPerSector := f.sectorSize / 4
		for j := 0; j < entriesPerSector-1 && len(fatSectorLocs) < int(numFATSectors); j++ {
			loc := binary.LittleEndian.Uint32(buf[j*4 : j*4+4])
			if loc == freeSect {
				break
			}
			fatSectorLocs = append(fatSectorLocs, loc)
		}
		difatSector = binary.LittleEndian.Uint32(buf[f.sectorSize-4 : f.sectorSize])
	}

	// Read the FAT itself.
	f.fat = make([]uint32, 0, clampAlloc(len(fatSectorLocs)*f.sectorSize/4))
	for _, loc := range fatSectorLocs {
		buf, err := f.readRawSector(loc)
		if err != nil {
			return nil, err
		}
		for off := 0; off < len(buf); off += 4 {
			f.fat = append(f.fat, binary.LittleEndian.Uint32(buf[off:off+4]))
		}
	}

	// Read directory sectors (chained via the FAT).
	dirData, err := f.readChain(firstDirSector, 0)
	if err != nil {
		return nil, fmt.Errorf("cfb: reading directory stream: %w", err)
	}
	numEntries := len(dirData) / dirEntrySize
	rawEntries := make([]rawDirEntry, numEntries)
	for i := 0; i < numEntries; i++ {
		rawEntries[i] = parseDirEntry(dirData[i*dirEntrySize : (i+1)*dirEntrySize])
	}
	if len(rawEntries) == 0 || rawEntries[0].objType != objTypeRoot {
		return nil, errors.New("cfb: missing root storage entry")
	}

	// Root stream size gives us the mini-stream's total length; the
	// mini-stream itself lives in normal sectors chained from the root's
	// start sector.
	rootSize := rawEntries[0].size
	f.miniStream, err = f.readChain(rawEntries[0].startSect, rootSize)
	if err != nil {
		return nil, fmt.Errorf("cfb: reading mini stream: %w", err)
	}

	// Read the mini FAT.
	if numMiniFATSectors > 0 {
		miniFATData, err := f.readChain(firstMiniFATSector, 0)
		if err != nil {
			return nil, fmt.Errorf("cfb: reading mini FAT: %w", err)
		}
		f.miniFAT = make([]uint32, len(miniFATData)/4)
		for i := range f.miniFAT {
			f.miniFAT[i] = binary.LittleEndian.Uint32(miniFATData[i*4 : i*4+4])
		}
	}

	entries := make([]*Entry, numEntries)
	for i, re := range rawEntries {
		entries[i] = &Entry{Name: re.name, Type: re.objType, StartSect: re.startSect, Size: re.size}
	}
	f.Root = entries[0]
	f.buildTree(entries, 0, rawEntries, make(map[uint32]bool))

	return f, nil
}

// buildTree populates the Children map of the storage at idx, then recurses
// into each child storage. visited guards against a malformed file whose
// directory entries reference each other cyclically, which would otherwise
// recurse until the stack is exhausted.
func (f *File) buildTree(entries []*Entry, idx int, raw []rawDirEntry, visited map[uint32]bool) {
	e := entries[idx]
	if !e.IsStorage() || e.Children != nil {
		return
	}
	e.Children = make(map[string]*Entry)

	childIndexes := make([]uint32, 0, 8)
	f.walkSiblings(entries, raw, raw[idx].childID, e.Children, &childIndexes, visited)

	for _, childIdx := range childIndexes {
		f.buildTree(entries, int(childIdx), raw, visited)
	}
}

// walkSiblings performs an in-order traversal of the red-black tree of
// sibling directory entries rooted at nodeID, collecting them into out by
// name and recording their indexes.
func (f *File) walkSiblings(entries []*Entry, raw []rawDirEntry, nodeID uint32, out map[string]*Entry, indexes *[]uint32, visited map[uint32]bool) {
	if nodeID == noStream || int(nodeID) >= len(entries) || visited[nodeID] {
		return
	}
	visited[nodeID] = true

	r := raw[nodeID]
	f.walkSiblings(entries, raw, r.leftID, out, indexes, visited)
	out[entries[nodeID].Name] = entries[nodeID]
	*indexes = append(*indexes, nodeID)
	f.walkSiblings(entries, raw, r.rightID, out, indexes, visited)
}

type rawDirEntry struct {
	name      string
	objType   byte
	leftID    uint32
	rightID   uint32
	childID   uint32
	startSect uint32
	size      uint64
}

// dirEntryNameSize is the size of the fixed name field at the start of a
// directory entry: 32 UTF-16 code units including the NUL terminator.
const dirEntryNameSize = 64

func parseDirEntry(b []byte) rawDirEntry {
	// The stored length is only a hint and must be clamped to the fixed
	// name field: a corrupt file can otherwise claim a length of up to
	// 65535 and read past the end of the entry.
	nameLen := int(binary.LittleEndian.Uint16(b[64:66]))
	if nameLen > dirEntryNameSize {
		nameLen = dirEntryNameSize
	}
	var name string
	if nameLen >= 2 {
		u16 := make([]uint16, 0, (nameLen-2)/2)
		for i := 0; i+1 < nameLen-1; i += 2 {
			c := binary.LittleEndian.Uint16(b[i : i+2])
			if c == 0 { // defensive: stop at an embedded terminator
				break
			}
			u16 = append(u16, c)
		}
		name = string(utf16.Decode(u16))
	}
	return rawDirEntry{
		name:      name,
		objType:   b[66],
		leftID:    binary.LittleEndian.Uint32(b[68:72]),
		rightID:   binary.LittleEndian.Uint32(b[72:76]),
		childID:   binary.LittleEndian.Uint32(b[76:80]),
		startSect: binary.LittleEndian.Uint32(b[116:120]),
		size:      binary.LittleEndian.Uint64(b[120:128]),
	}
}

func (f *File) readRawSector(id uint32) ([]byte, error) {
	buf := make([]byte, f.sectorSize)
	off := int64(headerSize) + int64(id)*int64(f.sectorSize)
	if _, err := f.r.ReadAt(buf, off); err != nil {
		return nil, fmt.Errorf("cfb: reading sector %d: %w", id, err)
	}
	return buf, nil
}

// readChain follows a chain of normal (non-mini) sectors starting at
// startSect, using the FAT, and returns the concatenated bytes. If size is
// nonzero, the result is truncated to that many bytes; otherwise the full
// chain (padded to sector boundaries) is returned.
func (f *File) readChain(startSect uint32, size uint64) ([]byte, error) {
	var out []byte
	sect := startSect
	seen := map[uint32]bool{}
	for sect != endOfChain && sect != freeSect {
		if seen[sect] {
			return nil, errors.New("cfb: cyclic sector chain")
		}
		seen[sect] = true
		buf, err := f.readRawSector(sect)
		if err != nil {
			return nil, err
		}
		out = append(out, buf...)
		if int(sect) >= len(f.fat) {
			return nil, fmt.Errorf("cfb: sector %d out of FAT range", sect)
		}
		sect = f.fat[sect]
	}
	if size > 0 && uint64(len(out)) > size {
		out = out[:size]
	}
	return out, nil
}

// readMiniChain follows a chain of mini-sectors (within the mini-stream)
// starting at startSect, using the mini FAT.
func (f *File) readMiniChain(startSect uint32, size uint64) ([]byte, error) {
	var out []byte
	sect := startSect
	seen := map[uint32]bool{}
	for sect != endOfChain && sect != freeSect {
		if seen[sect] {
			return nil, errors.New("cfb: cyclic mini sector chain")
		}
		seen[sect] = true
		off := int(sect) * f.miniSectorSize
		if off+f.miniSectorSize > len(f.miniStream) {
			return nil, fmt.Errorf("cfb: mini sector %d out of range", sect)
		}
		out = append(out, f.miniStream[off:off+f.miniSectorSize]...)
		if int(sect) >= len(f.miniFAT) {
			return nil, fmt.Errorf("cfb: mini sector %d out of mini-FAT range", sect)
		}
		sect = f.miniFAT[sect]
	}
	if size > 0 && uint64(len(out)) > size {
		out = out[:size]
	}
	return out, nil
}

// ReadStream returns the full contents of a stream entry.
func (f *File) ReadStream(e *Entry) ([]byte, error) {
	if !e.IsStream() {
		return nil, fmt.Errorf("cfb: %q is not a stream", e.Name)
	}
	if e.Size < uint64(f.miniCutoff) {
		return f.readMiniChain(e.StartSect, e.Size)
	}
	return f.readChain(e.StartSect, e.Size)
}

// Open looks up a stream by a '/'-separated path from the root storage
// (e.g. "TBkndTask/FixedData") and returns its contents.
func (f *File) OpenStream(path string) ([]byte, error) {
	e, err := f.Lookup(path)
	if err != nil {
		return nil, err
	}
	return f.ReadStream(e)
}

// Lookup resolves a '/'-separated path from the root storage to its Entry.
// An empty path returns the root storage.
func (f *File) Lookup(path string) (*Entry, error) {
	cur := f.Root
	for _, part := range strings.Split(path, "/") {
		if part == "" {
			continue
		}
		if !cur.IsStorage() {
			return nil, fmt.Errorf("cfb: %q is not a storage", cur.Name)
		}
		next, ok := cur.Children[part]
		if !ok {
			return nil, fmt.Errorf("cfb: entry not found: %q", path)
		}
		cur = next
	}
	return cur, nil
}
