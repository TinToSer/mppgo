// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"bytes"
	"encoding/binary"
	"math"
	"testing"
	"time"
)

// --- fixture builders -------------------------------------------------

func u16(v int) []byte {
	b := make([]byte, 2)
	binary.LittleEndian.PutUint16(b, uint16(v))
	return b
}

func u32(v int) []byte {
	b := make([]byte, 4)
	binary.LittleEndian.PutUint32(b, uint32(v))
	return b
}

// putFloat64 writes an IEEE-754 double at the start of buf, for building
// the work/cost/units fields those decoders read.
func putFloat64(buf []byte, v float64) {
	binary.LittleEndian.PutUint64(buf, math.Float64bits(v))
}

// buildFixedMeta produces a FixedMeta stream whose records carry the given
// FixedData offsets. Each record is itemSize bytes with the offset at byte 4.
func buildFixedMeta(itemSize int, offsets ...int) []byte {
	var buf bytes.Buffer
	buf.Write(u32(fixedMetaMagic))
	buf.Write(u32(0))
	buf.Write(u32(len(offsets)))
	buf.Write(u32(0))
	for _, off := range offsets {
		rec := make([]byte, itemSize)
		copy(rec[4:], u32(off))
		buf.Write(rec)
	}
	return buf.Bytes()
}

// buildVarMeta produces a VarMeta stream describing the given entries.
func buildVarMeta(dataSize int, entries ...[3]int) []byte {
	var buf bytes.Buffer
	buf.Write(u32(varMetaMagic))
	buf.Write(u32(0))
	buf.Write(u32(len(entries)))
	buf.Write(u32(0))
	buf.Write(u32(0))
	buf.Write(u32(dataSize))
	for _, e := range entries { // {uniqueID, offset, type}
		buf.Write(u32(e[0]))
		buf.Write(u32(e[1]))
		buf.Write(u16(e[2]))
		buf.Write(u16(0))
	}
	return buf.Bytes()
}

// buildProps14 produces a Props14 stream from key/value pairs.
func buildProps14(entries ...[2]interface{}) []byte {
	var body bytes.Buffer
	for _, e := range entries {
		key := e[0].(int)
		data := e[1].([]byte)
		body.Write(u32(len(data)))
		body.Write(u32(key))
		body.Write(u32(0))
		body.Write(data)
		if len(data)%2 != 0 {
			body.WriteByte(0) // pad to a 2-byte boundary
		}
	}
	header := make([]byte, 16)
	copy(header[12:], u16(len(entries)))
	return append(header, body.Bytes()...)
}

// --- FixedMeta / FixedData -------------------------------------------

func TestParseFixedMeta(t *testing.T) {
	data := buildFixedMeta(10, 0, 6)
	fm, err := ParseFixedMeta(data, 10)
	if err != nil {
		t.Fatalf("ParseFixedMeta: %v", err)
	}
	if fm.ItemCount != 2 {
		t.Errorf("ItemCount = %d, want 2", fm.ItemCount)
	}
	if fm.AdjustedItemCount != 2 {
		t.Errorf("AdjustedItemCount = %d, want 2", fm.AdjustedItemCount)
	}
	if got := getInt(fm.ByteArrayValue(1), 4); got != 6 {
		t.Errorf("record 1 offset = %d, want 6", got)
	}
	if fm.ByteArrayValue(99) != nil {
		t.Error("out-of-range index should return nil")
	}
}

func TestParseFixedMetaRejectsBadInput(t *testing.T) {
	good := buildFixedMeta(10, 0)

	bad := append([]byte(nil), good...)
	copy(bad, u32(0xDEADBEEF))
	if _, err := ParseFixedMeta(bad, 10); err == nil {
		t.Error("expected an error for a bad magic number")
	}

	if _, err := ParseFixedMeta(good[:8], 10); err == nil {
		t.Error("expected an error for a truncated header")
	}

	if _, err := ParseFixedMeta(good, 0); err == nil {
		t.Error("expected an error for a zero item size")
	}
}

func TestParseFixedData(t *testing.T) {
	meta, err := ParseFixedMeta(buildFixedMeta(10, 0, 6), 10)
	if err != nil {
		t.Fatalf("ParseFixedMeta: %v", err)
	}
	raw := []byte("AAAAAABBBB")
	fd := ParseFixedData(meta, raw, 0, 0)

	if fd.ItemCount() != 2 {
		t.Fatalf("ItemCount = %d, want 2", fd.ItemCount())
	}
	// An item spans from its offset to the next item's offset; the last
	// item runs to the end of the block.
	if got, want := string(fd.ByteArrayValue(0)), "AAAAAA"; got != want {
		t.Errorf("item 0 = %q, want %q", got, want)
	}
	if got, want := string(fd.ByteArrayValue(1)), "BBBB"; got != want {
		t.Errorf("item 1 = %q, want %q", got, want)
	}
}

func TestParseFixedDataHandlesBadOffsets(t *testing.T) {
	// An offset past the end of the block must be skipped, not panic.
	meta, err := ParseFixedMeta(buildFixedMeta(10, 0, 9999), 10)
	if err != nil {
		t.Fatalf("ParseFixedMeta: %v", err)
	}
	fd := ParseFixedData(meta, []byte("ABCD"), 0, 0)
	if fd.ByteArrayValue(1) != nil {
		t.Error("item with an out-of-range offset should be nil")
	}
}

func TestParseFixedDataRespectsMaxSize(t *testing.T) {
	meta, err := ParseFixedMeta(buildFixedMeta(10, 0), 10)
	if err != nil {
		t.Fatalf("ParseFixedMeta: %v", err)
	}
	fd := ParseFixedData(meta, []byte("ABCDEFGHIJ"), 4, 0)
	if got, want := len(fd.ByteArrayValue(0)), 4; got != want {
		t.Errorf("item length = %d, want %d (capped by maxExpectedSize)", got, want)
	}
}

func TestParseFixedDataFixedSize(t *testing.T) {
	fd := ParseFixedDataFixedSize([]byte("AABBCC"), 2)
	if fd.ItemCount() != 3 {
		t.Fatalf("ItemCount = %d, want 3", fd.ItemCount())
	}
	if got, want := string(fd.ByteArrayValue(2)), "CC"; got != want {
		t.Errorf("item 2 = %q, want %q", got, want)
	}
}

// --- VarMeta / Var2Data ----------------------------------------------

func TestParseVarMeta(t *testing.T) {
	vm, err := ParseVarMeta(buildVarMeta(64, [3]int{7, 0, 1}, [3]int{7, 12, 8}))
	if err != nil {
		t.Fatalf("ParseVarMeta: %v", err)
	}
	if vm.ItemCount != 2 {
		t.Errorf("ItemCount = %d, want 2", vm.ItemCount)
	}
	if vm.DataSize != 64 {
		t.Errorf("DataSize = %d, want 64", vm.DataSize)
	}

	if off, ok := vm.Offset(7, 1); !ok || off != 0 {
		t.Errorf("Offset(7,1) = %d,%v want 0,true", off, ok)
	}
	if off, ok := vm.Offset(7, 8); !ok || off != 12 {
		t.Errorf("Offset(7,8) = %d,%v want 12,true", off, ok)
	}
	if _, ok := vm.Offset(99, 1); ok {
		t.Error("Offset for an unknown id should report not-ok")
	}
	if got := len(vm.Types(7)); got != 2 {
		t.Errorf("len(Types(7)) = %d, want 2", got)
	}
}

func TestParseVarMetaRejectsBadMagic(t *testing.T) {
	data := buildVarMeta(0, [3]int{1, 0, 1})
	copy(data, u32(0xDEADBEEF))
	if _, err := ParseVarMeta(data); err == nil {
		t.Error("expected an error for a bad magic number")
	}
	// Zero is tolerated: some otherwise-valid files use it.
	copy(data, u32(0))
	if _, err := ParseVarMeta(data); err != nil {
		t.Errorf("a zero magic number should be accepted: %v", err)
	}
}

func TestParseVar2Data(t *testing.T) {
	vm, err := ParseVarMeta(buildVarMeta(0, [3]int{7, 0, 1}, [3]int{7, 12, 8}))
	if err != nil {
		t.Fatalf("ParseVarMeta: %v", err)
	}

	var raw bytes.Buffer
	raw.Write(u32(8))                               // offset 0: 8 bytes
	raw.Write([]byte{'H', 0, 'i', 0, '!', 0, 0, 0}) // "Hi!"
	raw.Write(u32(4))                               // offset 12: 4 bytes
	raw.Write(u32(42))                              // int value

	vd := ParseVar2Data(vm, raw.Bytes())
	if got, want := vd.UnicodeString(7, 1), "Hi!"; got != want {
		t.Errorf("UnicodeString(7,1) = %q, want %q", got, want)
	}
	if got, want := vd.Int(7, 8), 42; got != want {
		t.Errorf("Int(7,8) = %d, want %d", got, want)
	}
	if vd.ByteArray(99, 1) != nil {
		t.Error("ByteArray for an unknown id should be nil")
	}
	if got := vd.UnicodeString(99, 1); got != "" {
		t.Errorf("UnicodeString for an unknown id = %q, want empty", got)
	}
}

func TestParseVar2DataHandlesCorruptSizes(t *testing.T) {
	vm, err := ParseVarMeta(buildVarMeta(0, [3]int{1, 0, 1}))
	if err != nil {
		t.Fatalf("ParseVarMeta: %v", err)
	}
	// A declared size far larger than the remaining bytes must be skipped.
	raw := append(u32(9999), []byte("AB")...)
	if got := ParseVar2Data(vm, raw).ByteArray(1, 1); got != nil {
		t.Errorf("oversized entry = %v, want nil", got)
	}
}

// --- Props ------------------------------------------------------------

func TestParseProps14(t *testing.T) {
	props := ParseProps14(buildProps14(
		[2]interface{}{100, []byte{0x2A}},                 // byte
		[2]interface{}{101, u32(1234)},                    // int
		[2]interface{}{102, []byte{'H', 0, 'i', 0, 0, 0}}, // string
		[2]interface{}{103, u16(1)},                       // bool
	))

	if got, want := props.Byte(100), 42; got != want {
		t.Errorf("Byte(100) = %d, want %d", got, want)
	}
	if got, want := props.Int(101), 1234; got != want {
		t.Errorf("Int(101) = %d, want %d", got, want)
	}
	if got, want := props.UnicodeString(102), "Hi"; got != want {
		t.Errorf("UnicodeString(102) = %q, want %q", got, want)
	}
	if !props.Boolean(103) {
		t.Error("Boolean(103) = false, want true")
	}

	// Absent keys yield zero values rather than panicking.
	if got := props.Int(999); got != 0 {
		t.Errorf("Int(absent) = %d, want 0", got)
	}
	if got := props.ByteArray(999); got != nil {
		t.Errorf("ByteArray(absent) = %v, want nil", got)
	}
	if got := props.UnicodeString(999); got != "" {
		t.Errorf("UnicodeString(absent) = %q, want empty", got)
	}
}

func TestParseProps14Truncated(t *testing.T) {
	// A header claiming more entries than the body holds must terminate.
	data := buildProps14([2]interface{}{1, u32(5)})
	copy(data[12:], u16(99))
	props := ParseProps14(data)
	if got, want := props.Int(1), 5; got != want {
		t.Errorf("Int(1) = %d, want %d", got, want)
	}

	if p := ParseProps14(nil); p.Int(1) != 0 {
		t.Error("parsing nil should yield an empty Props")
	}
}

func TestPropsTimestamp(t *testing.T) {
	// time=480 (units of 6s => 48 min), days=15000 — same fixture as
	// TestGetTimestamp in util_test.go, since Props.Timestamp is a thin
	// wrapper around the same decoder.
	props := ParseProps14(buildProps14(
		[2]interface{}{1, []byte{0xE0, 0x01, 0x98, 0x3A}},
	))
	got, ok := props.Timestamp(1)
	if !ok {
		t.Fatal("Timestamp reported not-ok for a valid value")
	}
	want := epoch.AddDate(0, 0, 15000).Add(48 * time.Minute)
	if !got.Equal(want) {
		t.Errorf("Timestamp = %v, want %v", got, want)
	}

	if _, ok := props.Timestamp(999); ok {
		t.Error("Timestamp for an absent key should report not-ok")
	}
}

// --- CompObj ----------------------------------------------------------

func TestParseCompObj(t *testing.T) {
	writeLenString := func(buf *bytes.Buffer, s string) {
		buf.Write(u32(len(s) + 1))
		buf.WriteString(s)
		buf.WriteByte(0)
	}

	var buf bytes.Buffer
	buf.Write(make([]byte, 28))
	writeLenString(&buf, "Microsoft.Project 16.0")
	writeLenString(&buf, "MSProject.MPP14")
	writeLenString(&buf, "MSProject.Project.9")

	co := ParseCompObj(buf.Bytes())
	if got, want := co.ApplicationName, "Microsoft.Project 16.0"; got != want {
		t.Errorf("ApplicationName = %q, want %q", got, want)
	}
	if got, want := co.ApplicationVersion, 16; got != want {
		t.Errorf("ApplicationVersion = %d, want %d", got, want)
	}
	if got, want := co.FileFormat, "MSProject.MPP14"; got != want {
		t.Errorf("FileFormat = %q, want %q", got, want)
	}
	if got, want := co.ApplicationID, "MSProject.Project.9"; got != want {
		t.Errorf("ApplicationID = %q, want %q", got, want)
	}
}

func TestParseCompObjTruncated(t *testing.T) {
	// Must not panic on a stream that ends early.
	for _, n := range []int{0, 8, 28, 30} {
		if co := ParseCompObj(make([]byte, n)); co == nil {
			t.Errorf("ParseCompObj(%d bytes) returned nil", n)
		}
	}
}

func TestParseVarMetaClampsItemCount(t *testing.T) {
	// A header claiming far more entries than the stream can hold must be
	// clamped, not used to size an allocation.
	data := buildVarMeta(0, [3]int{1, 0, 1})
	copy(data[8:], u32(1<<30))

	vm, err := ParseVarMeta(data)
	if err != nil {
		t.Fatalf("ParseVarMeta: %v", err)
	}
	if vm.ItemCount != 1 {
		t.Errorf("ItemCount = %d, want 1 (clamped to what the stream holds)", vm.ItemCount)
	}

	copy(data[8:], u32(0xFFFFFFFF)) // negative when read as int32
	if _, err := ParseVarMeta(data); err == nil {
		t.Error("expected an error for a negative item count")
	}
}

func TestParseFixedDataRejectsBadInput(t *testing.T) {
	// Neither a nil meta block nor a zero item size may panic.
	if fd := ParseFixedData(nil, []byte("ABC"), 0, 0); fd.ItemCount() != 0 {
		t.Error("nil meta should yield an empty block")
	}
	if fd := ParseFixedDataFixedSize([]byte("ABC"), 0); fd.ItemCount() != 0 {
		t.Error("a zero item size should yield an empty block")
	}
}
