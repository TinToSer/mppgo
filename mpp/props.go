// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

// Props is a decoded "Props"/"Props14" property-set stream: an integer key
// to raw-byte-value map, with typed accessors.
type Props struct {
	values map[int][]byte
}

// ParseProps14 parses a Props14-format stream (used by MPP14 for the
// top-level "Props14" stream and the per-entity "Props" streams under
// TBkndTask/TBkndRsc/TBkndAssn).
//
// Layout: a 16-byte header whose bytes [12:14] give the entry count, then
// that many entries of { length int32, key int32, unused int32, data[length] },
// each padded to a 2-byte boundary.
func ParseProps14(data []byte) *Props {
	p := &Props{values: make(map[int][]byte)}
	if len(data) < 16 {
		return p
	}
	headerCount := getShort(data, 12)
	pos := 16
	found := 0
	for found < headerCount {
		if len(data)-pos < 12 {
			break
		}
		length := getInt(data, pos)
		key := getInt(data, pos+4)
		pos += 12

		if len(data)-pos < length || length < 1 {
			break
		}
		p.values[key] = data[pos : pos+length]
		pos += length
		found++

		if length%2 != 0 {
			pos++
		}
	}
	return p
}

func (p *Props) ByteArray(key int) []byte { return p.values[key] }

func (p *Props) Byte(key int) int {
	v := p.values[key]
	if len(v) < 1 {
		return 0
	}
	return int(v[0])
}

func (p *Props) Short(key int) int {
	v := p.values[key]
	if len(v) < 2 {
		return 0
	}
	return getShort(v, 0)
}

func (p *Props) Int(key int) int {
	v := p.values[key]
	if len(v) < 4 {
		return 0
	}
	return getInt(v, 0)
}

func (p *Props) Double(key int) float64 {
	v := p.values[key]
	if len(v) < 8 {
		return 0
	}
	return getDouble(v, 0)
}

func (p *Props) UnicodeString(key int) string {
	v := p.values[key]
	if v == nil {
		return ""
	}
	return getUnicodeString(v, 0)
}

func (p *Props) Boolean(key int) bool {
	v := p.values[key]
	if len(v) < 2 {
		return false
	}
	return getShort(v, 0) != 0
}
