package cfb_test

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"github.com/tintoser/mppgo/cfb"
)

// An empty stream owns no sectors; writers commonly leave its start sector
// as 0, which also names real data. Reading it must return nothing rather
// than whatever chain sector 0 happens to start.
func TestReadEmptyStreamWithZeroStartSector(t *testing.T) {
	data := patchFixture()
	offset := 1024 + 3*128 // fourth directory entry, unused by the fixture
	for index, letter := range utf16.Encode([]rune("Empty")) {
		binary.LittleEndian.PutUint16(data[offset+index*2:], letter)
	}
	binary.LittleEndian.PutUint16(data[offset+64:], uint16((len("Empty")+1)*2))
	data[offset+66], data[offset+67] = 2, 1
	binary.LittleEndian.PutUint32(data[offset+68:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(data[offset+72:], 0xFFFFFFFF)
	binary.LittleEndian.PutUint32(data[offset+76:], 0xFFFFFFFF)
	// "Mini" (entry 2) gains "Empty" as its right sibling.
	binary.LittleEndian.PutUint32(data[1024+2*128+72:], 3)

	file, err := cfb.Open(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got, err := file.OpenStream("Empty")
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 0 {
		t.Fatalf("empty stream returned %d bytes", len(got))
	}
}

// Version 4 compound files use 4096-byte sectors, and sector 0 starts
// 4096 bytes in: the 512-byte header is padded to a full sector.
func TestOpen4096ByteSectors(t *testing.T) {
	const sector = 4096
	data := make([]byte, 4*sector)
	copy(data, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	put16 := func(offset int, value uint16) { binary.LittleEndian.PutUint16(data[offset:], value) }
	put32 := func(offset int, value uint32) { binary.LittleEndian.PutUint32(data[offset:], value) }
	put16(26, 4)
	put16(28, 0xFFFE)
	put16(30, 12)
	put16(32, 6)
	put32(44, 1)          // one FAT sector
	put32(48, 1)          // directory at sector 1
	put32(56, 4096)       // mini stream cutoff
	put32(60, 0xFFFFFFFE) // no mini FAT
	put32(68, 0xFFFFFFFE) // no DIFAT sectors
	for index := 0; index < 109; index++ {
		put32(76+index*4, 0xFFFFFFFF)
	}
	put32(76, 0) // FAT at sector 0

	fat := sector // sector 0
	for index := 0; index < sector/4; index++ {
		put32(fat+index*4, 0xFFFFFFFF)
	}
	put32(fat, 0xFFFFFFFD)   // sector 0: FAT
	put32(fat+4, 0xFFFFFFFE) // sector 1: directory
	put32(fat+8, 0xFFFFFFFE) // sector 2: stream data

	entry := func(index int, name string, kind byte, start uint32, size uint64, child uint32) {
		offset := 2*sector + index*128 // directory is sector 1
		for i, letter := range utf16.Encode([]rune(name)) {
			put16(offset+i*2, letter)
		}
		put16(offset+64, uint16((len(name)+1)*2))
		data[offset+66], data[offset+67] = kind, 1
		put32(offset+68, 0xFFFFFFFF)
		put32(offset+72, 0xFFFFFFFF)
		put32(offset+76, child)
		put32(offset+116, start)
		binary.LittleEndian.PutUint64(data[offset+120:], size)
	}
	entry(0, "Root Entry", 5, 0xFFFFFFFE, 0, 1)
	entry(1, "Big", 2, 2, sector, 0xFFFFFFFF)
	want := bytes.Repeat([]byte{0x5A}, sector)
	copy(data[3*sector:], want) // sector 2

	file, err := cfb.Open(bytes.NewReader(data))
	if err != nil {
		t.Fatal(err)
	}
	got, err := file.OpenStream("Big")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(got, want) {
		t.Fatal("4096-byte-sector stream read from the wrong file offset")
	}
}
