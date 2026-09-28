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

// outlineCodeValue is one node of the shared outline-code value tree.
type outlineCodeValue struct {
	text     string
	parentID int
}

// outlineCodeParentOffset returns the byte offset of a value's parent-ID
// field within its FixedData record. Project 2013 and 2016+ share a
// layout; 2010 differs.
func outlineCodeParentOffset(applicationVersion int) int {
	if applicationVersion <= appVersionProject2010 {
		return 8
	}
	return 10
}

// readOutlineCodeValues reads the TBkndOutlCode storage: the single,
// project-wide table backing every task and resource Outline Code field.
// Returns an empty map (not an error) if the storage is absent — an older
// or minimal file may not have one at all.
func readOutlineCodeValues(src *streamSource, projectDirPath string, applicationVersion int) map[int]outlineCodeValue {
	values := make(map[int]outlineCodeValue)

	dir := projectDirPath + "/TBkndOutlCode"
	if !src.has(dir + "/VarMeta") {
		return values
	}

	varMetaRaw, err := src.plain(dir + "/VarMeta")
	if err != nil {
		return values
	}
	varMeta, err := ParseVarMeta(varMetaRaw)
	if err != nil {
		return values
	}
	var2Raw, err := src.plain(dir + "/Var2Data")
	if err != nil {
		return values
	}
	varData := ParseVar2Data(varMeta, var2Raw)

	fixedMetaRaw, err := src.plain(dir + "/FixedMeta")
	if err != nil {
		return values
	}
	fixedMeta, err := ParseFixedMeta(fixedMetaRaw, 10)
	if err != nil {
		return values
	}
	fixedRaw, err := src.decoded(dir + "/FixedData")
	if err != nil {
		return values
	}
	fixedData := ParseFixedData(fixedMeta, fixedRaw, 512, 0)

	parentOffset := outlineCodeParentOffset(applicationVersion)

	for i := 0; i < fixedData.ItemCount(); i++ {
		rec := fixedData.ByteArrayValue(i)
		if rec == nil {
			continue
		}
		id := getShort(rec, outlineCodeUniqueIDOffset)
		if id <= 0 {
			continue
		}
		if !varData.Has(id, outlineCodeValueVarType) {
			continue
		}

		values[id] = outlineCodeValue{
			text:     varData.UnicodeString(id, outlineCodeValueVarType),
			parentID: getShort(rec, parentOffset),
		}
	}

	return values
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
