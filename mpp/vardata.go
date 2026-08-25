// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import "time"

// Var2Data represents a "Var2Data" stream: a set of variable-length values,
// each stored as a 4-byte little-endian size followed by that many bytes,
// located via offsets supplied by a companion VarMeta block.
type Var2Data struct {
	meta *VarMeta
	data map[int][]byte // offset -> value bytes
}

// ParseVar2Data parses a Var2Data stream given its companion VarMeta.
func ParseVar2Data(meta *VarMeta, raw []byte) *Var2Data {
	vd := &Var2Data{meta: meta, data: make(map[int][]byte)}
	for _, offset := range meta.Offsets() {
		if offset < 0 || offset >= len(raw) {
			continue
		}
		if _, seen := vd.data[offset]; seen {
			continue
		}
		if len(raw)-offset < 4 {
			continue
		}
		size := getInt(raw, offset)
		available := len(raw) - offset - 4
		if size < 0 || size > available {
			continue
		}
		vd.data[offset] = raw[offset+4 : offset+4+size]
	}
	return vd
}

// ByteArray returns the raw value stored for (id, type), or nil if no such
// value exists. The returned slice aliases the underlying stream and must
// not be modified.
func (d *Var2Data) ByteArray(id, typ int) []byte {
	off, ok := d.meta.Offset(id, typ)
	if !ok {
		return nil
	}
	return d.data[off]
}

// Has reports whether a value is stored for (id, type).
func (d *Var2Data) Has(id, typ int) bool {
	return d.ByteArray(id, typ) != nil
}

// UnicodeString returns the value for (id, type) decoded as a NUL-terminated
// UTF-16LE string.
func (d *Var2Data) UnicodeString(id, typ int) string {
	return getUnicodeString(d.ByteArray(id, typ), 0)
}

// Int returns the value for (id, type) decoded as a 32-bit little-endian
// signed value.
func (d *Var2Data) Int(id, typ int) int {
	return getInt(d.ByteArray(id, typ), 0)
}

// Short returns the value for (id, type) decoded as an unsigned 16-bit
// little-endian value.
func (d *Var2Data) Short(id, typ int) int {
	return getShort(d.ByteArray(id, typ), 0)
}

// Byte returns the value for (id, type) decoded as a single byte.
func (d *Var2Data) Byte(id, typ int) int {
	return getByte(d.ByteArray(id, typ), 0)
}

// Long returns the value for (id, type) decoded as a 64-bit little-endian
// signed value.
func (d *Var2Data) Long(id, typ int) int64 {
	return getLong(d.ByteArray(id, typ), 0)
}

// Double returns the value for (id, type) decoded as an IEEE-754 double.
func (d *Var2Data) Double(id, typ int) float64 {
	return getDouble(d.ByteArray(id, typ), 0)
}

// IntAt returns a 32-bit value read from an arbitrary offset within the
// value stored for (id, type). Several MPP fields pack multiple integers
// into a single variable-length entry.
func (d *Var2Data) IntAt(id, typ, offset int) int {
	return getInt(d.ByteArray(id, typ), offset)
}

// Timestamp returns the value for (id, type) decoded as a date and time,
// reporting ok=false when the field holds the "no value" encoding.
func (d *Var2Data) Timestamp(id, typ int) (time.Time, bool) {
	return getTimestamp(d.ByteArray(id, typ), 0)
}

// String returns the value for (id, type) decoded as a NUL-terminated
// single-byte-per-character string.
func (d *Var2Data) String(id, typ int) string {
	return getString(d.ByteArray(id, typ), 0)
}
