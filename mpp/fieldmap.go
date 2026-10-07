// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

// MS Project stores task/resource/assignment fields at byte offsets that
// are not fixed across builds: unlike TBkndCal, MS Project also writes an
// explicit field map (a Props value) describing where each field actually
// lives in that specific file. The MPP14 defaults below (sourced from MPXJ,
// see NOTICE) hold for a vanilla file, but a real file — especially one
// saved by a continuously-updated Microsoft 365 build — can shift fields
// after the ones MPXJ's defaults were captured against. Reading the file's
// own field map, when present, is what MPXJ itself does at runtime, and is
// what this reader does too rather than trusting the defaults blindly.
const (
	taskFieldMapPropsKey1       = 131092
	taskFieldMapPropsKey2       = 50331668
	resourceFieldMapPropsKey1   = 131093
	resourceFieldMapPropsKey2   = 50331669
	assignmentFieldMapPropsKey1 = 131095
	assignmentFieldMapPropsKey2 = 50331671

	// Class prefixes ORed onto a field's low-16-bit ID to form the full
	// value stored in a field-map entry and read back out of it.
	taskFieldBase       = 0x0B400000
	resourceFieldBase   = 0x0C400000
	assignmentFieldBase = 0x0F400000

	fieldMapRecordSize        = 28
	fieldMapNoFixedDataOffset = 65535
)

// fieldLocation is where a field-map entry places a field: which fixed-data
// block (0 = FixedData, 1 = Fixed2Data, ...) and the byte offset within it.
type fieldLocation struct {
	block  int
	offset int
}

// loadFieldMap returns the parsed field-map blob for the given candidate
// Props keys (the first one present wins, matching MS Project's own
// fallback order), or nil if neither is present — callers then fall back
// to the MPP14 defaults entirely.
func loadFieldMap(projectProps *Props, key1, key2 int) map[int]fieldLocation {
	data := projectProps.ByteArray(key1)
	if data == nil {
		data = projectProps.ByteArray(key2)
	}
	if data == nil {
		return nil
	}
	return parseFieldMap(data)
}

// fieldMapData returns the raw field-map blob under the first of the two
// Props keys that is present, or nil.
func fieldMapData(projectProps *Props, key1, key2 int) []byte {
	if data := projectProps.ByteArray(key1); data != nil {
		return data
	}
	return projectProps.ByteArray(key2)
}

// parseFieldMap decodes a field-map blob into a lookup from a field's full
// class-prefixed ID to its fixed-data block and byte offset (see
// parseFieldMapEntries for how blocks are told apart). Entries located in
// var data or in the meta-data flags are left out: this reader locates
// var-data fields directly by their (uniqueID, type) key.
func parseFieldMap(data []byte) map[int]fieldLocation {
	locations := make(map[int]fieldLocation)
	for _, e := range parseFieldMapEntries(data) {
		if e.location == fieldLocationFixed {
			locations[e.fullID] = fieldLocation{block: e.block, offset: e.offset}
		}
	}
	return locations
}

// fieldOffset resolves a field's byte offset within the given fixed-data
// block, preferring the file's own field map over the supplied MPP14
// default. When the file has a field map but it places the field anywhere
// else — another block, or var data — the result is -1, which every
// bounds-checked accessor reads as absent. Falling back to the default
// offset in that case would read whatever unrelated field now occupies it.
func fieldOffset(fieldMap map[int]fieldLocation, fullFieldID, block, defaultOffset int) int {
	if fieldMap == nil {
		return defaultOffset
	}
	if loc, ok := fieldMap[fullFieldID]; ok && loc.block == block {
		return loc.offset
	}
	return -1
}
