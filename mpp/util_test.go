// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"testing"
	"time"
)

// MPP encodes several fields as unsigned 16-bit values, and relies on 65535
// as a "not applicable" sentinel. Reading these as signed would turn 65535
// into -1 and silently corrupt every date field, so this is pinned down.
func TestGetShortIsUnsigned(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want int
	}{
		{"zero", []byte{0x00, 0x00}, 0},
		{"one", []byte{0x01, 0x00}, 1},
		{"little endian", []byte{0x34, 0x12}, 0x1234},
		{"high bit set", []byte{0x00, 0x80}, 32768},
		{"max / NA sentinel", []byte{0xFF, 0xFF}, 65535},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getShort(tt.data, 0); got != tt.want {
				t.Errorf("getShort(%v) = %d, want %d", tt.data, got, tt.want)
			}
		})
	}
}

func TestGetIntIsSigned(t *testing.T) {
	if got, want := getInt([]byte{0xFF, 0xFF, 0xFF, 0xFF}, 0), -1; got != want {
		t.Errorf("getInt(FFFFFFFF) = %d, want %d", got, want)
	}
	if got, want := getInt([]byte{0x78, 0x56, 0x34, 0x12}, 0), 0x12345678; got != want {
		t.Errorf("getInt = %#x, want %#x", got, want)
	}
}

// A truncated or malformed file must not panic the parser.
func TestAccessorsAreBoundsSafe(t *testing.T) {
	short := []byte{0x01}
	var empty []byte

	for _, data := range [][]byte{short, empty} {
		if got := getByte(data, 5); got != 0 {
			t.Errorf("getByte out of range = %d, want 0", got)
		}
		if got := getShort(data, 5); got != 0 {
			t.Errorf("getShort out of range = %d, want 0", got)
		}
		if got := getInt(data, 5); got != 0 {
			t.Errorf("getInt out of range = %d, want 0", got)
		}
		if got := getLong(data, 5); got != 0 {
			t.Errorf("getLong out of range = %d, want 0", got)
		}
		if got := getDouble(data, 5); got != 0 {
			t.Errorf("getDouble out of range = %v, want 0", got)
		}
		if got := getUnicodeString(data, 5); got != "" {
			t.Errorf("getUnicodeString out of range = %q, want empty", got)
		}
		if got := getString(data, 5); got != "" {
			t.Errorf("getString out of range = %q, want empty", got)
		}
		if got := getGUID(data, 0); got != "" {
			t.Errorf("getGUID out of range = %q, want empty", got)
		}
		if _, ok := getDate(data, 5); ok {
			t.Error("getDate out of range should report not-ok")
		}
		if _, ok := getTimestamp(data, 5); ok {
			t.Error("getTimestamp out of range should report not-ok")
		}
	}

	// Negative offsets are rejected too.
	if got := getShort([]byte{1, 2, 3, 4}, -2); got != 0 {
		t.Errorf("getShort(-2) = %d, want 0", got)
	}
}

func TestGetDate(t *testing.T) {
	// Days since the 1983-12-31 epoch.
	got, ok := getDate([]byte{0x01, 0x00}, 0)
	if !ok {
		t.Fatal("getDate reported not-ok for a valid date")
	}
	if want := time.Date(1984, 1, 1, 0, 0, 0, 0, time.UTC); !got.Equal(want) {
		t.Errorf("getDate = %v, want %v", got, want)
	}

	if _, ok := getDate([]byte{0xFF, 0xFF}, 0); ok {
		t.Error("65535 is the NA sentinel and must report not-ok")
	}
}

func TestGetTimeAndDuration(t *testing.T) {
	// Times are tenths of a minute since midnight: 4800 => 08:00.
	if got, want := getTime([]byte{0xC0, 0x12}, 0), 8*time.Hour; got != want {
		t.Errorf("getTime = %v, want %v", got, want)
	}
	// Durations use the same units: 2400 => 4h.
	if got, want := getDuration([]byte{0x60, 0x09}, 0), 4*time.Hour; got != want {
		t.Errorf("getDuration = %v, want %v", got, want)
	}
}

func TestGetTimestamp(t *testing.T) {
	// time=480 (units of 6s => 48 min), days=15000.
	data := []byte{0xE0, 0x01, 0x98, 0x3A}
	got, ok := getTimestamp(data, 0)
	if !ok {
		t.Fatal("getTimestamp reported not-ok for a valid timestamp")
	}
	want := epoch.AddDate(0, 0, 15000).Add(48 * time.Minute)
	if !got.Equal(want) {
		t.Errorf("getTimestamp = %v, want %v", got, want)
	}

	// Day counts of 0, 1 and 65535 all mean "no value".
	for _, days := range []byte{0x00, 0x01} {
		if _, ok := getTimestamp([]byte{0x00, 0x00, days, 0x00}, 0); ok {
			t.Errorf("day count %d should report not-ok", days)
		}
	}
	if _, ok := getTimestamp([]byte{0x00, 0x00, 0xFF, 0xFF}, 0); ok {
		t.Error("day count 65535 should report not-ok")
	}
}

func TestGetUnicodeString(t *testing.T) {
	tests := []struct {
		name string
		data []byte
		want string
	}{
		{"terminated", []byte{'H', 0, 'i', 0, 0, 0, 'x', 0}, "Hi"},
		{"unterminated runs to end", []byte{'H', 0, 'i', 0}, "Hi"},
		{"empty", []byte{0, 0}, ""},
		{"non-ascii", []byte{0xE9, 0x00, 0x00, 0x00}, "é"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := getUnicodeString(tt.data, 0); got != tt.want {
				t.Errorf("getUnicodeString = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetString(t *testing.T) {
	if got, want := getString([]byte{'a', 'b', 'c', 0, 'd'}, 0), "abc"; got != want {
		t.Errorf("getString = %q, want %q", got, want)
	}
}

// GUIDs are mixed-endian: the first three groups are little-endian, the
// last two big-endian.
func TestGetGUID(t *testing.T) {
	data := []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13, 14, 15, 16}
	if got, want := getGUID(data, 0), "04030201-0605-0807-090A-0B0C0D0E0F10"; got != want {
		t.Errorf("getGUID = %q, want %q", got, want)
	}
	if got := getGUID(make([]byte, 16), 0); got != "" {
		t.Errorf("all-zero GUID = %q, want empty", got)
	}
}

func TestEncryptionMask(t *testing.T) {
	if got := encryptionMask(0); got != 0 {
		t.Errorf("encryptionMask(0) = %#x, want 0 (no masking)", got)
	}
	// The mask is the one's complement of the stored code.
	for _, code := range []byte{0x01, 0x42, 0x92, 0xFF} {
		want := 0xFF - code
		if got := encryptionMask(code); got != want {
			t.Errorf("encryptionMask(%#x) = %#x, want %#x", code, got, want)
		}
	}
}
