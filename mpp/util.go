// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"encoding/binary"
	"fmt"
	"math"
	"time"
	"unicode/utf16"
)

// epoch is the reference date MPP binary date/time fields are encoded
// relative to: 1983-12-31T00:00:00.
var epoch = time.Date(1983, 12, 31, 0, 0, 0, 0, time.UTC)

// The accessors below are deliberately bounds-checked, returning a zero
// value rather than panicking when a field runs past the end of the buffer.
// MPP files in the wild are frequently truncated or internally inconsistent,
// and a parser for untrusted binary input should degrade rather than crash.
// Callers that need to distinguish "absent" from "zero" should check the
// buffer length themselves before reading.

func getByte(data []byte, offset int) int {
	if offset < 0 || offset >= len(data) {
		return 0
	}
	return int(data[offset])
}

// getShort reads a 16-bit little-endian value. Note this is UNSIGNED
// (0..65535), matching the semantics of MPXJ's ByteArrayHelper.getShort,
// which several MPP field encodings depend on — notably the 65535 "not
// applicable" sentinel used by date fields.
func getShort(data []byte, offset int) int {
	if offset < 0 || offset+2 > len(data) {
		return 0
	}
	return int(binary.LittleEndian.Uint16(data[offset : offset+2]))
}

// getInt reads a 32-bit little-endian signed value.
func getInt(data []byte, offset int) int {
	if offset < 0 || offset+4 > len(data) {
		return 0
	}
	return int(int32(binary.LittleEndian.Uint32(data[offset : offset+4])))
}

// getLong reads a 64-bit little-endian signed value.
func getLong(data []byte, offset int) int64 {
	if offset < 0 || offset+8 > len(data) {
		return 0
	}
	return int64(binary.LittleEndian.Uint64(data[offset : offset+8]))
}

// getDouble reads an IEEE-754 double. NaN is normalised to 0, matching how
// MS Project encodes "no value" in some numeric fields.
func getDouble(data []byte, offset int) float64 {
	if offset < 0 || offset+8 > len(data) {
		return 0
	}
	v := math.Float64frombits(binary.LittleEndian.Uint64(data[offset : offset+8]))
	if math.IsNaN(v) {
		return 0
	}
	return v
}

// getDate reads a date stored as a count of days since the MPP epoch.
// Returns ok=false for the "N/A" sentinel (65535).
func getDate(data []byte, offset int) (t time.Time, ok bool) {
	if offset < 0 || offset+2 > len(data) {
		return time.Time{}, false
	}
	days := getShort(data, offset)
	if days == 65535 {
		return time.Time{}, false
	}
	return epoch.AddDate(0, 0, days), true
}

// getTime reads a time-of-day value stored as tenths of a minute since
// midnight, returned as an offset from midnight.
func getTime(data []byte, offset int) time.Duration {
	seconds := int64(getShort(data, offset)/10) * 60
	if seconds > 86399 {
		seconds %= 86400
	}
	return time.Duration(seconds) * time.Second
}

// getDuration reads a duration encoded as tenths of a minute.
func getDuration(data []byte, offset int) time.Duration {
	return time.Duration(getShort(data, offset)) * time.Minute / 10
}

// getTimestamp reads a combined date+time value: a 2-byte time (units of 6
// seconds) at offset, followed by a 2-byte day count at offset+2.
func getTimestamp(data []byte, offset int) (time.Time, bool) {
	if offset < 0 || offset+4 > len(data) {
		return time.Time{}, false
	}
	days := getShort(data, offset+2)
	if days <= 1 || days == 65535 {
		return time.Time{}, false
	}
	t := getShort(data, offset)
	if t == 65535 {
		t = 0
	}
	result := epoch.AddDate(0, 0, days).Add(time.Duration(t*6) * time.Second)
	// Very small day counts show as "NA" in MS Project. Use the absence of a
	// seconds component as a heuristic to tell real values from junk.
	if days < 100 && result.Second() != 0 {
		return time.Time{}, false
	}
	return result, true
}

// getUnicodeString reads a NUL-terminated UTF-16LE string starting at offset.
func getUnicodeString(data []byte, offset int) string {
	if offset < 0 || offset >= len(data) {
		return ""
	}
	end := len(data)
	for i := offset; i+1 < len(data); i += 2 {
		if data[i] == 0 && data[i+1] == 0 {
			end = i
			break
		}
	}
	if end <= offset {
		return ""
	}
	u16 := make([]uint16, 0, (end-offset)/2)
	for i := offset; i+1 < end; i += 2 {
		u16 = append(u16, binary.LittleEndian.Uint16(data[i:i+2]))
	}
	return string(utf16.Decode(u16))
}

// getString reads a NUL-terminated single-byte-per-char string at offset.
func getString(data []byte, offset int) string {
	if offset < 0 || offset >= len(data) {
		return ""
	}
	end := offset
	for end < len(data) && data[end] != 0 {
		end++
	}
	return string(data[offset:end])
}

// getGUID reads a 16-byte Microsoft-style GUID (mixed-endian: the first
// three groups are little-endian, the last two big-endian) and formats it as
// a standard hyphenated UUID string. Returns "" if the bytes are all zero or
// the buffer is too short.
func getGUID(data []byte, offset int) string {
	if offset < 0 || len(data) < offset+16 {
		return ""
	}
	b := data[offset : offset+16]
	allZero := true
	for _, v := range b {
		if v != 0 {
			allZero = false
			break
		}
	}
	if allZero {
		return ""
	}
	return fmt.Sprintf("%02X%02X%02X%02X-%02X%02X-%02X%02X-%02X%02X-%02X%02X%02X%02X%02X%02X",
		b[3], b[2], b[1], b[0],
		b[5], b[4],
		b[7], b[6],
		b[8], b[9],
		b[10], b[11], b[12], b[13], b[14], b[15])
}
