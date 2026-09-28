// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import (
	"fmt"
	"strconv"
	"strings"

	"github.com/tintoser/mppgo/project"
)

// customFieldKind decides how an ExtendedAttribute's plain-text Value is
// parsed — MSPDI does not tag the value with its own type inline; the
// field ID says what kind of field it is, the same way it does in the MPP
// binary format (see mpp/customfields.go), but as a flat text ID rather
// than a var-data key.
type customFieldKind byte

const (
	kindText customFieldKind = iota
	kindNumber
	kindDate
	kindDuration
	kindCost
	kindFlag
)

type customFieldDef struct {
	name string
	kind customFieldKind
}

// taskCustomFields and resourceCustomFields map a task/resource
// ExtendedAttribute's FieldID to its generic name and value kind. The IDs
// are MPXJ's universal field identifiers — the same ones the MPP binary
// reader uses (see mpp/task.go, mpp/resource.go) — not anything specific to
// XML; MSPDI simply exposes them as plain decimal text instead of a
// class-prefixed binary field-map entry.
var taskCustomFields = buildCustomFieldDefs(
	[]int{51, 54, 57, 60, 63, 66, 67, 68, 69, 70, 317, 318, 319, 320, 321, 322, 323, 324, 325, 326, 327, 328, 329, 330, 331, 332, 333, 334, 335, 336}, "Text", kindText,
	[]int{87, 88, 89, 90, 91, 302, 303, 304, 305, 306, 307, 308, 309, 310, 311, 312, 313, 314, 315, 316}, "Number", kindNumber,
	[]int{265, 266, 267, 268, 269, 270, 271, 272, 273, 274}, "Date", kindDate,
	[]int{103, 104, 105, 275, 276, 277, 278, 279, 280, 281}, "Duration", kindDuration,
	[]int{106, 107, 108, 258, 259, 260, 261, 262, 263, 264}, "Cost", kindCost,
	[]int{72, 73, 74, 75, 76, 77, 78, 79, 80, 81, 292, 293, 294, 295, 296, 297, 298, 299, 300, 301}, "Flag", kindFlag,
)

var resourceCustomFields = buildCustomFieldDefs(
	[]int{8, 9, 30, 31, 32, 97, 98, 99, 100, 101, 225, 226, 227, 228, 229, 230, 231, 232, 233, 234, 235, 236, 237, 238, 239, 240, 241, 242, 243, 244}, "Text", kindText,
	[]int{112, 113, 114, 115, 116, 205, 206, 207, 208, 209, 210, 211, 212, 213, 214, 215, 216, 217, 218, 219}, "Number", kindNumber,
	[]int{173, 174, 175, 176, 177, 178, 179, 180, 181, 182}, "Date", kindDate,
	[]int{117, 118, 119, 183, 184, 185, 186, 187, 188, 189}, "Duration", kindDuration,
	[]int{123, 124, 125, 166, 167, 168, 169, 170, 171, 172}, "Cost", kindCost,
	[]int{127, 128, 129, 130, 131, 132, 133, 134, 135, 126, 195, 196, 197, 198, 199, 200, 201, 202, 203, 204}, "Flag", kindFlag,
)

// buildCustomFieldDefs assembles a field-ID lookup from repeated
// (ids, namePrefix, kind) groups, naming each id by its 1-based position
// within its own group (namePrefix + "1", "2", ...).
func buildCustomFieldDefs(groups ...interface{}) map[int]customFieldDef {
	defs := make(map[int]customFieldDef)
	for i := 0; i+2 < len(groups); i += 3 {
		ids := groups[i].([]int)
		prefix := groups[i+1].(string)
		kind := groups[i+2].(customFieldKind)
		for n, id := range ids {
			defs[id] = customFieldDef{name: fmt.Sprintf("%s%d", prefix, n+1), kind: kind}
		}
	}
	return defs
}

// applyExtendedAttribute decodes one ExtendedAttribute element into fields,
// allocating it if necessary, using defs to look up the field's name and
// value kind by its FieldID. A field ID not present in defs (an Enterprise
// custom field, an outline code, or any field this reader doesn't already
// expose by name elsewhere) is skipped rather than guessed at.
func applyExtendedAttribute(fields map[string]interface{}, defs map[int]customFieldDef, scale durationScale, defaultUnits project.TimeUnit, fieldID, value, durationFormat string) map[string]interface{} {
	if fieldID == "" || value == "" {
		return fields
	}
	id, err := strconv.Atoi(strings.TrimSpace(fieldID))
	if err != nil {
		return fields
	}
	def, ok := defs[id]
	if !ok {
		return fields
	}

	var v interface{}
	switch def.kind {
	case kindText:
		v = value

	case kindNumber:
		n, err := strconv.ParseFloat(correctNumberFormat(value), 64)
		if err != nil {
			return fields
		}
		v = n

	case kindCost:
		n, err := strconv.ParseFloat(correctNumberFormat(value), 64)
		if err != nil {
			return fields
		}
		v = n / 100 // custom Cost fields are stored in hundredths, unlike the plain decimal core Cost fields

	case kindFlag:
		if value != "1" {
			return fields
		}
		v = true

	case kindDate:
		t, ok := parseMSPDIDateTime(value)
		if !ok {
			return fields
		}
		v = t

	case kindDuration:
		unit := defaultUnits
		if durationFormat != "" {
			if code, err := strconv.Atoi(durationFormat); err == nil {
				unit = mspdiDurationUnit(code, defaultUnits)
			}
		}
		d, ok := parseDuration(scale, value, unit)
		if !ok {
			return fields
		}
		v = d
	}

	if fields == nil {
		fields = make(map[string]interface{})
	}
	fields[def.name] = v
	return fields
}

// correctNumberFormat trims a leading "+" some exporters write, which
// strconv.ParseFloat otherwise accepts fine — kept only for symmetry with
// MPXJ's own equivalent and as a single place to fix up any future
// oddities found in real files.
func correctNumberFormat(s string) string {
	return strings.TrimSpace(s)
}
