// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"bytes"
	"testing"
)

// buildFieldMapEntry produces one 28-byte field-map record: a mask (unused
// by this reader), the fixed-data byte offset (or 65535 for a var-data-only
// field), an unused var-data key byte, padding, the full class-prefixed
// field ID, and a category (also unused here).
func buildFieldMapEntry(dataBlockOffset, fullFieldID int) []byte {
	rec := make([]byte, 28)
	copy(rec[4:], u16(dataBlockOffset))
	copy(rec[12:], u32(fullFieldID))
	return rec
}

func TestParseFieldMap(t *testing.T) {
	var buf bytes.Buffer
	buf.Write(buildFieldMapEntry(4, taskFieldBase|taskFieldIDUniqueID))
	buf.Write(buildFieldMapEntry(0, taskFieldBase|taskFieldIDID))
	buf.Write(buildFieldMapEntry(fieldMapNoFixedDataOffset, taskFieldBase|taskFieldIDLateStart)) // var-data only

	fm := parseFieldMap(buf.Bytes())
	if got, want := len(fm), 2; got != want {
		t.Fatalf("len(fm) = %d, want %d (the var-data-only entry should be skipped)", got, want)
	}
	if got, want := fm[taskFieldBase|taskFieldIDUniqueID], 4; got != want {
		t.Errorf("UNIQUE_ID offset = %d, want %d", got, want)
	}
	if got, want := fm[taskFieldBase|taskFieldIDID], 0; got != want {
		t.Errorf("ID offset = %d, want %d", got, want)
	}
	if _, ok := fm[taskFieldBase|taskFieldIDLateStart]; ok {
		t.Error("a var-data-only entry should not appear in the offset map")
	}
}

func TestParseFieldMapIgnoresTrailingPartialRecord(t *testing.T) {
	data := append(buildFieldMapEntry(4, taskFieldBase|taskFieldIDUniqueID), []byte{1, 2, 3}...)
	fm := parseFieldMap(data)
	if len(fm) != 1 {
		t.Errorf("len(fm) = %d, want 1 (trailing partial record ignored)", len(fm))
	}
}

func TestFieldOffsetPrefersFileMapOverDefault(t *testing.T) {
	fm := map[int]int{taskFieldBase | taskFieldIDUniqueID: 4}

	if got, want := fieldOffset(fm, taskFieldBase|taskFieldIDUniqueID, 0), 4; got != want {
		t.Errorf("fieldOffset (present) = %d, want %d", got, want)
	}
	if got, want := fieldOffset(fm, taskFieldBase|taskFieldIDID, 4), 4; got != want {
		t.Errorf("fieldOffset (absent, falls back to default) = %d, want %d", got, want)
	}
	if got, want := fieldOffset(nil, taskFieldBase|taskFieldIDUniqueID, 0), 0; got != want {
		t.Errorf("fieldOffset (nil map, falls back to default) = %d, want %d", got, want)
	}
}

func TestLoadFieldMapFallsBackToSecondKey(t *testing.T) {
	entry := buildFieldMapEntry(4, taskFieldBase|taskFieldIDUniqueID)

	props := ParseProps14(buildProps14([2]interface{}{taskFieldMapPropsKey2, entry}))
	fm := loadFieldMap(props, taskFieldMapPropsKey1, taskFieldMapPropsKey2)
	if got, want := fm[taskFieldBase|taskFieldIDUniqueID], 4; got != want {
		t.Errorf("offset via key2 = %d, want %d", got, want)
	}

	if fm := loadFieldMap(ParseProps14(nil), taskFieldMapPropsKey1, taskFieldMapPropsKey2); fm != nil {
		t.Error("loadFieldMap should return nil when neither key is present")
	}
}
