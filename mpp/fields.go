package mpp

import (
	"sort"
	"time"

	"github.com/tintoser/mppgo/project"
)

// fieldKind is a field's data type, mirroring MPXJ's DataType for the kinds
// MPP fields use. The per-field kinds live in fielddefs_gen.go.
type fieldKind int

const (
	kindString fieldKind = iota
	kindDate
	kindCurrency
	kindBoolean
	kindNumeric
	kindDuration
	kindUnits
	kindPercentage
	kindAccrue
	kindConstraint
	kindRate
	kindPriority
	kindTaskType
	kindResourceType
	kindWork
	kindInteger
	kindNotes
	kindShort
	kindBinary
	kindDelay
	kindWorkUnits
	kindWorkgroup
	kindGUID
	kindRateUnits
	kindEarnedValueMethod
	kindResourceRequestType
	kindBookingType
	kindTimeUnits
	kindWorkContour
)

type fieldDef struct {
	name  string
	kind  fieldKind
	units int // index of the field holding this duration's units, or -1
}

// Field-map entry locations. Category 0x0B and 0x64 entries are boolean
// flags held in the FixedMeta/Fixed2Meta records; their bit positions are
// not described by the map, so they are read from fixed per-version tables
// instead (see the *BitLayout functions).
const (
	fieldLocationFixed = iota
	fieldLocationVar
	fieldLocationMeta
)

const (
	fieldMapCategoryMeta0 = 0x0B
	fieldMapCategoryMeta1 = 0x64
)

type fieldMapEntry struct {
	fullID   int // the class-prefixed field ID as stored
	id       int // field index: the low 16 bits of fullID
	location int
	block    int
	offset   int
}

// parseFieldMapEntries decodes every 28-byte field-map record: field ID at
// 12, fixed-data offset at 4 (65535 = not in fixed data) and category at
// 20. Offsets within a block ascend, so a lower one starts the next block;
// meta-data flags do not take part in that sequence. Matches MPXJ's
// FieldMap.createFieldMap.
func parseFieldMapEntries(data []byte) []fieldMapEntry {
	var entries []fieldMapEntry
	block, lastOffset := 0, 0
	for idx := 0; idx+fieldMapRecordSize <= len(data); idx += fieldMapRecordSize {
		full := getInt(data, idx+12)
		e := fieldMapEntry{fullID: full, id: full & 0xFFFF, offset: getShort(data, idx+4)}
		switch getShort(data, idx+20) {
		case fieldMapCategoryMeta0, fieldMapCategoryMeta1:
			e.location = fieldLocationMeta
		default:
			if e.offset == fieldMapNoFixedDataOffset {
				e.location = fieldLocationVar
			} else {
				if e.offset < lastOffset {
					block++
				}
				lastOffset = e.offset
				e.location, e.block = fieldLocationFixed, block
			}
		}
		entries = append(entries, e)
	}
	return entries
}

// lookupValue is one entry of a custom field lookup table or outline code
// value list (the shared TBkndOutlCode table).
type lookupValue struct {
	text        string      // the value as text, used for outline code paths
	value       interface{} // the value in its own type
	description string
	parentID    int
	guid        string
	tableGUID   string // the lookup table (custom field) the value belongs to
}

// readContext carries what every entity reader needs: the file's streams,
// project settings and the tables shared across entities.
type readContext struct {
	src          *streamSource
	props        *Props
	version      int
	scale        durationScale
	defaultUnits project.TimeUnit
	aliases      customFieldAliases
	values       map[int]lookupValue
	valuesByGUID map[string]lookupValue

	// criticalSlackLimit is the project's critical slack limit in days.
	criticalSlackLimit float64
}

// fieldDecoder reads every field one entity class stores, guided by the
// file's field map for fixed data and by the var-data keys themselves (in
// MPP14 a var-data key is the field index).
type fieldDecoder struct {
	ctx  *readContext
	defs map[int]fieldDef
	// alternate covers var-data keys with the 0x4000 bit, where some MPP14
	// files keep assignment custom field values; they are read only when
	// the field's usual key holds nothing.
	alternate map[int]fieldDef
	fixed     []fieldMapEntry // fixed-data entries, ordered by field index
	byID      map[int]fieldMapEntry
}

func newFieldDecoder(ctx *readContext, defs map[int]fieldDef, mapData []byte) *fieldDecoder {
	d := &fieldDecoder{ctx: ctx, defs: defs, byID: make(map[int]fieldMapEntry)}
	for _, e := range parseFieldMapEntries(mapData) {
		if _, known := defs[e.id]; !known {
			continue
		}
		d.byID[e.id] = e
		if e.location == fieldLocationFixed {
			d.fixed = append(d.fixed, e)
		}
	}
	sort.Slice(d.fixed, func(i, j int) bool { return d.fixed[i].id < d.fixed[j].id })
	return d
}

// decode returns every field value stored for the entity uid, keyed by
// MPXJ's field name in CamelCase ("TotalSlack", "Text1", ...). A field with
// no stored value (an "NA" date, a var-data field never set) is absent.
// Where two field indexes share a name, the lower index wins.
func (d *fieldDecoder) decode(blocks [][]byte, varData *Var2Data, uid int) map[string]interface{} {
	fields := make(map[string]interface{})
	for _, e := range d.fixed {
		def := d.defs[e.id]
		if _, set := fields[def.name]; set || e.block >= len(blocks) {
			continue
		}
		if v, ok := d.fixedValue(def, e, blocks, varData, uid); ok {
			fields[def.name] = v
		}
	}
	if varData != nil {
		types := varData.meta.Types(uid)
		sort.Ints(types)
		for _, key := range types {
			def, known := d.defs[key]
			if key&alternateFieldFlag != 0 && d.alternate != nil {
				def, known = d.alternate[key&^alternateFieldFlag]
			}
			if !known {
				continue
			}
			if _, set := fields[def.name]; set {
				continue
			}
			if v, ok := d.varValue(def, blocks, varData, uid, key); ok {
				fields[def.name] = v
			}
		}
	}
	return fields
}

// units resolves the time unit a duration field is expressed in, from its
// companion units field wherever that is stored.
func (d *fieldDecoder) units(def fieldDef, blocks [][]byte, varData *Var2Data, uid int, fallback project.TimeUnit) project.TimeUnit {
	if def.units < 0 {
		return fallback
	}
	if e, ok := d.byID[def.units]; ok && e.location == fieldLocationFixed && e.block < len(blocks) {
		if rec := blocks[e.block]; e.offset+2 <= len(rec) {
			return durationTimeUnits(getShort(rec, e.offset), d.ctx.defaultUnits)
		}
	}
	if varData != nil && varData.Has(uid, def.units) {
		return durationTimeUnits(varData.Short(uid, def.units), d.ctx.defaultUnits)
	}
	return fallback
}

func (d *fieldDecoder) fixedValue(def fieldDef, e fieldMapEntry, blocks [][]byte, varData *Var2Data, uid int) (interface{}, bool) {
	rec := blocks[e.block]
	size := fixedFieldSize(def.kind)
	if size == 0 || rec == nil || e.offset+size > len(rec) {
		return nil, false
	}
	off := e.offset
	switch def.kind {
	case kindDate:
		t, ok := getTimestamp(rec, off)
		return t, ok
	case kindInteger:
		return getInt(rec, off), true
	case kindDuration:
		raw := getInt(rec, off)
		if raw == -1 {
			return nil, false
		}
		return d.ctx.scale.duration(raw, d.units(def, blocks, varData, uid, d.ctx.defaultUnits)), true
	case kindTimeUnits:
		return durationTimeUnits(getShort(rec, off), d.ctx.defaultUnits), true
	case kindConstraint:
		return project.ConstraintType(getShort(rec, off)), true
	case kindPriority, kindShort, kindWorkgroup:
		return getShort(rec, off), true
	case kindPercentage:
		return float64(taskPercentage(rec, off)), true
	case kindTaskType:
		return taskType(getShort(rec, off)), true
	case kindAccrue:
		return accrueType(getShort(rec, off)), true
	case kindCurrency, kindUnits:
		return getCurrency(rec, off), true
	case kindRate:
		return getDouble(rec, off), true
	case kindWork:
		return getWork(rec, off), true
	case kindBoolean:
		return getShort(rec, off) != 0, true
	case kindDelay:
		return project.Duration{Amount: float64(getShort(rec, off)) / 600, Units: project.Hours}, true
	case kindWorkUnits:
		v := getByte(rec, off)
		return workTimeUnit(v), v != 0
	case kindRateUnits:
		return workTimeUnit(getShort(rec, off)), true
	case kindEarnedValueMethod:
		return earnedValueMethod(getShort(rec, off)), true
	case kindResourceRequestType:
		return resourceRequestType(getShort(rec, off)), true
	case kindGUID:
		g := getGUID(rec, off)
		return g, g != ""
	}
	return nil, false
}

func (d *fieldDecoder) varValue(def fieldDef, blocks [][]byte, varData *Var2Data, uid, key int) (interface{}, bool) {
	data := varData.ByteArray(uid, key)
	if data == nil {
		return nil, false
	}
	switch def.kind {
	case kindString, kindDate, kindNumeric:
		if v, listed, ok := d.listValue(data); listed {
			return v, ok
		}
	}
	switch def.kind {
	case kindString:
		s := getUnicodeString(data, 0)
		return s, s != ""
	case kindNotes:
		s := getString(data, 0)
		return stripRTF(s), s != ""
	case kindDate:
		if len(data) == 512 || len(data) < 4 {
			return nil, false
		}
		t, ok := getTimestamp(data, 0)
		return t, ok
	case kindNumeric:
		return getDouble(data, 0), len(data) >= 8
	case kindInteger:
		return getInt(data, 0), len(data) >= 4
	case kindDuration:
		if len(data) == 512 || len(data) < 4 || getInt(data, 0) == -1 {
			return nil, false
		}
		return d.ctx.scale.duration(getInt(data, 0), d.units(def, blocks, varData, uid, project.Hours)), true
	case kindTimeUnits:
		return durationTimeUnits(getShort(data, 0), d.ctx.defaultUnits), len(data) >= 2
	case kindCurrency:
		return getCurrency(data, 0), len(data) >= 8
	case kindWork:
		return getWork(data, 0), len(data) >= 8
	case kindDelay:
		return project.Duration{Amount: float64(getShort(data, 0)) / 600, Units: project.Hours}, len(data) >= 2
	case kindWorkUnits:
		v := getByte(data, 0)
		return workTimeUnit(v), v != 0
	case kindRateUnits:
		return workTimeUnit(getShort(data, 0)), len(data) >= 2
	case kindEarnedValueMethod:
		return earnedValueMethod(getShort(data, 0)), len(data) >= 2
	case kindResourceRequestType:
		return resourceRequestType(getShort(data, 0)), len(data) >= 2
	case kindAccrue:
		return accrueType(getShort(data, 0)), len(data) >= 2
	case kindPercentage, kindShort, kindWorkgroup:
		return getShort(data, 0), len(data) >= 2
	case kindBoolean:
		return getShort(data, 0) != 0, len(data) >= 2
	case kindBookingType:
		return bookingType(getShort(data, 0)), len(data) >= 2
	case kindGUID:
		g := getGUID(data, 0)
		return g, g != ""
	}
	return nil, false
}

const alternateFieldFlag = 0x4000

// Var-data custom field values that come from a lookup table store a
// reference instead of the value: a 2-byte marker, the value's unique ID
// (-1 when only the GUID identifies it) and its GUID.
const (
	valueListWithIDMask    = 0x0701
	valueListWithoutIDMask = 0x0401
)

// listValue resolves a lookup-table reference. listed reports whether data
// is one at all; ok whether it resolved.
func (d *fieldDecoder) listValue(data []byte) (value interface{}, listed, ok bool) {
	flag := getShort(data, 0)
	if flag != valueListWithIDMask && flag != valueListWithoutIDMask {
		return nil, false, false
	}
	if len(data) == 4 {
		t, ok := getDate(data, 2)
		return t, true, ok
	}
	item, found := d.ctx.values[getInt(data, 2)]
	if !found || getInt(data, 2) == -1 {
		item, found = d.ctx.valuesByGUID[getGUID(data, 6)]
	}
	if !found || item.value == nil {
		return nil, true, false
	}
	return item.value, true, true
}

func fixedFieldSize(kind fieldKind) int {
	switch kind {
	case kindDate, kindInteger, kindDuration:
		return 4
	case kindTimeUnits, kindConstraint, kindPriority, kindPercentage, kindTaskType, kindAccrue, kindShort,
		kindBoolean, kindDelay, kindWorkgroup, kindRateUnits, kindEarnedValueMethod, kindResourceRequestType:
		return 2
	case kindCurrency, kindUnits, kindRate, kindWork:
		return 8
	case kindWorkUnits:
		return 1
	case kindGUID:
		return 16
	}
	return 0
}

func accrueType(code int) string {
	switch code {
	case 1:
		return "Start"
	case 2:
		return "End"
	case 3:
		return "Prorated"
	}
	return ""
}

func earnedValueMethod(code int) string {
	if code == 1 {
		return "Physical % Complete"
	}
	return "% Complete"
}

func resourceRequestType(code int) string {
	switch code {
	case 1:
		return "Request"
	case 2:
		return "Demand"
	}
	return "None"
}

func bookingType(code int) string {
	if code == 1 {
		return "Proposed"
	}
	return "Committed"
}

// workContourName names an MPP work contour code (0 = Flat ... 8 =
// Contoured); anything else reads as Flat, as MPXJ's WorkContourHelper.
func workContourName(code int) string {
	names := [...]string{"Flat", "Back Loaded", "Front Loaded", "Double Peak", "Early Peak", "Late Peak", "Bell", "Turtle", "Contoured"}
	if code >= 0 && code < len(names) {
		return names[code]
	}
	return "Flat"
}

// Typed accessors over a decoded field map, returning the zero value when
// the field is absent or of another type.

func fieldString(f map[string]interface{}, name string) string {
	v, _ := f[name].(string)
	return v
}

func fieldTime(f map[string]interface{}, name string) time.Time {
	v, _ := f[name].(time.Time)
	return v
}

func fieldDuration(f map[string]interface{}, name string) project.Duration {
	v, _ := f[name].(project.Duration)
	return v
}

func fieldFloat(f map[string]interface{}, name string) float64 {
	switch v := f[name].(type) {
	case float64:
		return v
	case int:
		return float64(v)
	}
	return 0
}

func fieldInt(f map[string]interface{}, name string) int {
	v, _ := f[name].(int)
	return v
}

func fieldBool(f map[string]interface{}, name string) bool {
	v, _ := f[name].(bool)
	return v
}

func fieldUnit(f map[string]interface{}, name string, fallback project.TimeUnit) project.TimeUnit {
	if v, ok := f[name].(project.TimeUnit); ok {
		return v
	}
	return fallback
}
