// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

// propsCustomFields is the Props key holding the custom-field definition
// block: the alias a user gave each generic field (Text1, Number3, ...)
// through MS Project's Customize Fields dialog, plus a per-field
// data-type/lookup-table section this reader does not need. Unlike most
// Props keys used elsewhere in this reader, this one is never set on the
// project-wide Props stream — MS Project writes a separate copy of it to
// each entity's own Props stream (TBkndTask/Props, TBkndRsc/Props,
// TBkndAssn/Props) instead, so all three must be read and merged.
const propsCustomFields = 71303169

// customFieldAliases maps a field's full class-prefixed ID (taskFieldBase|N,
// resourceFieldBase|N, ...) to the alias a user gave it. A field with no
// alias set, or not present in the map at all, keeps its generic name.
type customFieldAliases map[int]string

// readCustomFieldAliases reads and merges the custom-field alias block from
// every entity storage that carries one.
func readCustomFieldAliases(src *streamSource, projectDirPath string) customFieldAliases {
	aliases := make(customFieldAliases)
	for _, dir := range [...]string{"TBkndTask", "TBkndRsc", "TBkndAssn"} {
		raw, err := src.decoded(projectDirPath + "/" + dir + "/Props")
		if err != nil {
			continue
		}
		mergeCustomFieldAliases(aliases, ParseProps14(raw).ByteArray(propsCustomFields))
	}
	return aliases
}

// mergeCustomFieldAliases parses one entity's custom-field alias block into
// aliases. Layout: a 4-byte block size (repeated once more at offset 4,
// ignored here), a 4-byte alias count, then that many { fieldID int32,
// aliasOffset int32 } pairs. aliasOffset+4 locates a NUL-terminated
// UTF-16LE string within the same blob — the "+4" is the block's own
// leading size field, which the stored offset is relative to.
func mergeCustomFieldAliases(aliases customFieldAliases, data []byte) {
	if len(data) < 12 {
		return
	}

	aliasBlockSize := getInt(data, 0)
	numberOfAliases := getInt(data, 8)

	offset := 12
	for i := 0; i < numberOfAliases && offset < aliasBlockSize && offset+8 <= len(data); i++ {
		fieldID := getInt(data, offset)
		aliasOffset := getInt(data, offset+4) + 4
		offset += 8

		if aliasOffset < 0 || aliasOffset >= len(data) {
			continue
		}
		if alias := getUnicodeString(data, aliasOffset); alias != "" {
			aliases[fieldID] = alias
		}
	}
}

// name returns the alias registered for a field (by its full class-prefixed
// ID), or fallback if the field has none.
func (a customFieldAliases) name(fullFieldID int, fallback string) string {
	if alias, ok := a[fullFieldID]; ok && alias != "" {
		return alias
	}
	return fallback
}

// customFieldCurrency scales a custom Cost field's raw var-data value (a
// plain double, stored in hundredths of a currency unit like every other
// money field) down to the actual amount. Matches getCurrency's threshold
// for treating a near-zero amount as exactly zero.
func customFieldCurrency(v float64) float64 {
	if v < 0.1 && v > -0.1 {
		return 0
	}
	return v / 100
}
