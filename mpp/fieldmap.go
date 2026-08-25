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

// loadFieldMap returns the parsed field-map blob for the given candidate
// Props keys (the first one present wins, matching MS Project's own
// fallback order), or nil if neither is present — callers then fall back
// to the MPP14 defaults entirely.
func loadFieldMap(projectProps *Props, key1, key2 int) map[int]int {
	data := projectProps.ByteArray(key1)
	if data == nil {
		data = projectProps.ByteArray(key2)
	}
	if data == nil {
		return nil
	}
	return parseFieldMap(data)
}

// parseFieldMap decodes a field-map blob into a lookup from a field's full
// class-prefixed ID to its byte offset within whichever fixed-data stream
// it belongs to (a detail this reader already knows per field, so it is
// not re-derived here). Entries located in var-data instead of fixed data
// (offset == 65535) are skipped: this reader locates every var-data field
// it needs directly by its (uniqueID, type) key.
func parseFieldMap(data []byte) map[int]int {
	offsets := make(map[int]int)
	for idx := 0; idx+fieldMapRecordSize <= len(data); idx += fieldMapRecordSize {
		dataBlockOffset := getShort(data, idx+4)
		if dataBlockOffset == fieldMapNoFixedDataOffset {
			continue
		}
		typeValue := getInt(data, idx+12)
		offsets[typeValue] = dataBlockOffset
	}
	return offsets
}

// fieldOffset resolves the fixed-data byte offset for a field, preferring
// the file's own field map over the supplied MPP14 default.
func fieldOffset(fieldMap map[int]int, fullFieldID, defaultOffset int) int {
	if fieldMap != nil {
		if off, ok := fieldMap[fullFieldID]; ok {
			return off
		}
	}
	return defaultOffset
}
