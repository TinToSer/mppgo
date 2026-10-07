package cfb_test

import (
	"bytes"
	"encoding/binary"
	"testing"
	"unicode/utf16"

	"github.com/tintoser/mppgo/cfb"
)

func patchFixture() []byte {
	data := make([]byte, 7*512)
	copy(data, []byte{0xD0, 0xCF, 0x11, 0xE0, 0xA1, 0xB1, 0x1A, 0xE1})
	put16 := func(offset int, value uint16) { binary.LittleEndian.PutUint16(data[offset:], value) }
	put32 := func(offset int, value uint32) { binary.LittleEndian.PutUint32(data[offset:], value) }
	put16(26, 3)
	put16(28, 0xFFFE)
	put16(30, 9)
	put16(32, 6)
	put32(44, 1)
	put32(48, 1)
	put32(56, 512)
	put32(60, 4)
	put32(64, 1)
	put32(68, 0xFFFFFFFE)
	for index := 0; index < 109; index++ {
		put32(76+index*4, 0xFFFFFFFF)
	}
	put32(76, 0)
	for index := 0; index < 128; index++ {
		put32(512+index*4, 0xFFFFFFFF)
		put32(512+4*512+index*4, 0xFFFFFFFF)
	}
	for index, next := range []uint32{0xFFFFFFFD, 0xFFFFFFFE, 3, 0xFFFFFFFE, 0xFFFFFFFE, 0xFFFFFFFE} {
		put32(512+index*4, next)
	}
	put32(512+4*512, 1)
	put32(512+4*512+4, 0xFFFFFFFE)
	entry := func(index int, name string, kind byte, start uint32, size uint64, child, right uint32) {
		offset := 1024 + index*128
		letters := utf16.Encode([]rune(name))
		for index, letter := range letters {
			put16(offset+index*2, letter)
		}
		put16(offset+64, uint16((len(letters)+1)*2))
		data[offset+66], data[offset+67] = kind, 1
		put32(offset+68, 0xFFFFFFFF)
		put32(offset+72, right)
		put32(offset+76, child)
		put32(offset+116, start)
		binary.LittleEndian.PutUint64(data[offset+120:], size)
	}
	entry(0, "Root Entry", 5, 5, 128, 1, 0xFFFFFFFF)
	entry(1, "Normal", 2, 2, 700, 0xFFFFFFFF, 2)
	entry(2, "Mini", 2, 0, 80, 0xFFFFFFFF, 0xFFFFFFFF)
	copy(data[1536:], bytes.Repeat([]byte{0x41}, 700))
	copy(data[3072:], bytes.Repeat([]byte{0x42}, 80))
	return data
}

func TestPatchStreams(t *testing.T) {
	original := patchFixture()
	unchanged := bytes.Clone(original)
	updates := map[string][]byte{
		"Normal": bytes.Repeat([]byte{0x55}, 700), "Mini": bytes.Repeat([]byte{0x66}, 80),
	}
	patched, err := cfb.PatchStreams(original, updates)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, unchanged) || len(patched) != len(original) {
		t.Fatal("patching mutated the source or changed the container size")
	}
	file, err := cfb.Open(bytes.NewReader(patched))
	if err != nil {
		t.Fatal(err)
	}
	for path, expected := range updates {
		actual, err := file.OpenStream(path)
		if err != nil || !bytes.Equal(actual, expected) {
			t.Fatalf("invalid patched %s: %v", path, err)
		}
	}
	for index := range original {
		if (index >= 1536 && index < 2236) || (index >= 3072 && index < 3152) {
			continue
		}
		if original[index] != patched[index] {
			t.Fatalf("unrelated byte changed at %d", index)
		}
	}
}

func TestPatchStreamsRejectsInvalidEdits(t *testing.T) {
	for _, updates := range []map[string][]byte{
		{"Normal": {1}}, {"Missing": {}}, {"": {}},
	} {
		if _, err := cfb.PatchStreams(patchFixture(), updates); err == nil {
			t.Fatalf("expected rejection for %v", updates)
		}
	}
	corrupt := patchFixture()
	binary.LittleEndian.PutUint32(corrupt[512+2*4:], 2)
	if _, err := cfb.PatchStreams(corrupt, map[string][]byte{"Normal": make([]byte, 700)}); err == nil {
		t.Fatal("expected rejection of cyclic stream")
	}
}
