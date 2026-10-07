// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

// MS Project's ten task/resource Outline Code custom fields (Task.OUTLINE_CODE1
// .. OUTLINE_CODE10, and the resource equivalents) don't store their value
// inline: a task or resource carries only an index into one shared,
// project-wide lookup table (TBkndOutlCode), and that index is itself a
// value's unique ID rather than a position — following the field's own
// name literally would be a mistake. Each value also names a parent value,
// so the field a user actually sees is a hierarchical path ("USA | CA | Los
// Angeles"), not a single flat string.
//
// This reader resolves that path but does not attempt to group values by
// which of the ten outline code slots they belong to (MPXJ does, via a
// second GUID-keyed indirection through each entity's own Props stream) —
// a value's unique ID is enough to resolve its own path regardless of which
// field pointed at it, which is all a caller needs.
const (
	outlineCodeUniqueIDOffset = 4
	outlineCodeValueVarType   = 22

	// outlineCodeMaxDepth bounds the parent-chain walk in resolveOutlineCodePath,
	// so a corrupt file with a cyclic parent chain degrades to a truncated
	// path instead of hanging.
	outlineCodeMaxDepth = 64
)

// outlineCodeValue is one node of the shared value tree; see lookupValue.
type outlineCodeValue = lookupValue

// outlineCodeParentOffset returns the byte offset of a value's parent-ID
// field within its FixedData record. Project 2013 and 2016+ share a
// layout; 2010 differs.
func outlineCodeParentOffset(applicationVersion int) int {
	if applicationVersion <= appVersionProject2010 {
		return 8
	}
	return 10
}

// lookupValueLayout returns the offsets, within a value's Fixed2Data
// record, of its value-type code and of the GUID of the lookup table it
// belongs to. The record starts with the value's own GUID.
func lookupValueLayout(applicationVersion int) (typeOffset, tableOffset int) {
	if applicationVersion <= appVersionProject2010 {
		return 32, 16
	}
	return 16, 18
}

// Value-type codes of a lookup table value (MPXJ's CustomFieldValueDataType).
const (
	lookupTypeDate       = 4
	lookupTypeDuration   = 6
	lookupTypeCost       = 9
	lookupTypeNumber     = 15
	lookupTypeText       = 21
	lookupTypeFinishDate = 27

	lookupDescriptionVarType = 8
)

// readOutlineCodeValues reads the TBkndOutlCode storage: the single,
// project-wide table holding every outline code value and every custom
// field lookup table value, keyed by unique ID and by GUID. Returns empty
// maps (not an error) if the storage is absent — an older or minimal file
// may not have one at all.
func readOutlineCodeValues(ctx *readContext, projectDirPath string) (map[int]lookupValue, map[string]lookupValue) {
	values := make(map[int]lookupValue)
	byGUID := make(map[string]lookupValue)
	src := ctx.src

	dir := projectDirPath + "/TBkndOutlCode"
	if !src.has(dir + "/VarMeta") {
		return values, byGUID
	}

	varMetaRaw, err := src.plain(dir + "/VarMeta")
	if err != nil {
		return values, byGUID
	}
	varMeta, err := ParseVarMeta(varMetaRaw)
	if err != nil {
		return values, byGUID
	}
	var2Raw, err := src.plain(dir + "/Var2Data")
	if err != nil {
		return values, byGUID
	}
	varData := ParseVar2Data(varMeta, var2Raw)

	fixedMetaRaw, err := src.plain(dir + "/FixedMeta")
	if err != nil {
		return values, byGUID
	}
	fixedMeta, err := ParseFixedMeta(fixedMetaRaw, 10)
	if err != nil {
		return values, byGUID
	}
	fixedRaw, err := src.decoded(dir + "/FixedData")
	if err != nil {
		return values, byGUID
	}
	fixedData := ParseFixedData(fixedMeta, fixedRaw, 0, 0)

	// The second block carries each value's GUID, type and lookup table.
	// It is optional: without it values still resolve, as text.
	var fixed2 *FixedData
	if raw, err := src.plain(dir + "/Fixed2Meta"); err == nil {
		if meta2, err := ParseFixedMeta(raw, 10); err == nil {
			if raw2, err := src.decoded(dir + "/Fixed2Data"); err == nil {
				fixed2 = ParseFixedData(meta2, raw2, 0, 0)
			}
		}
	}

	parentOffset := outlineCodeParentOffset(ctx.version)
	typeOffset, tableOffset := lookupValueLayout(ctx.version)

	for i := 0; i < fixedData.ItemCount(); i++ {
		rec := fixedData.ByteArrayValue(i)
		if rec == nil {
			continue
		}
		id := getShort(rec, outlineCodeUniqueIDOffset)
		if id <= 0 {
			continue
		}
		raw := varData.ByteArray(id, outlineCodeValueVarType)
		if raw == nil {
			continue
		}
		v := lookupValue{
			text:        getUnicodeString(raw, 0),
			description: varData.UnicodeString(id, lookupDescriptionVarType),
			parentID:    getShort(rec, parentOffset),
		}
		valueType := 0
		if fixed2 != nil {
			if rec2 := fixed2.ByteArrayValue(i); rec2 != nil {
				v.guid = getGUID(rec2, 0)
				v.tableGUID = getGUID(rec2, tableOffset)
				valueType = getShort(rec2, typeOffset)
			}
		}
		v.value = typedLookupValue(ctx, valueType, raw, v.text)
		values[id] = v
		if v.guid != "" {
			byGUID[v.guid] = v
		}
	}
	return values, byGUID
}

// typedLookupValue decodes a lookup value's bytes by its value type,
// falling back to text for a type this reader does not know.
func typedLookupValue(ctx *readContext, valueType int, raw []byte, text string) interface{} {
	switch valueType {
	case lookupTypeDate, lookupTypeFinishDate:
		if t, ok := getTimestamp(raw, 0); ok {
			return t
		}
		return nil
	case lookupTypeDuration:
		return ctx.scale.duration(getInt(raw, 0), durationTimeUnits(getShort(raw, 4), ctx.defaultUnits))
	case lookupTypeCost:
		return getDouble(raw, 0) / 100
	case lookupTypeNumber:
		return getDouble(raw, 0)
	}
	return text
}

// resolveOutlineCodePath builds the full "parent | child | ..." path MS
// Project displays for an outline code value, given the leaf value's unique
// ID (what a task or resource's OUTLINE_CODEn_INDEX field actually stores).
// Returns "" if id does not resolve to a known value.
func resolveOutlineCodePath(values map[int]outlineCodeValue, id int) string {
	var parts []string
	seen := make(map[int]bool, outlineCodeMaxDepth)

	for depth := 0; depth < outlineCodeMaxDepth && id > 0 && !seen[id]; depth++ {
		v, ok := values[id]
		if !ok {
			break
		}
		seen[id] = true
		parts = append(parts, v.text)
		id = v.parentID
	}
	if len(parts) == 0 {
		return ""
	}

	// parts was collected leaf-to-root; reverse it to display root-to-leaf.
	for i, j := 0, len(parts)-1; i < j; i, j = i+1, j-1 {
		parts[i], parts[j] = parts[j], parts[i]
	}

	joined := parts[0]
	for _, p := range parts[1:] {
		joined += " | " + p
	}
	return joined
}
