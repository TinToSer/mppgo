// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"fmt"
	"sort"
	"time"

	"github.com/tintoser/mppgo/project"
)

// costRateEndDateNA is MPXJ's sentinel for "no upper bound" in a cost rate
// table or availability table entry (an arbitrary far-future date MS
// Project itself writes for this case, per its file format — see NOTICE).
// This reader reports it as a zero time.Time instead, matching the
// zero-means-unbounded convention used throughout this codebase.
var costRateEndDateNA = time.Date(2049, 12, 31, 23, 59, 0, 0, time.UTC)

const (
	resourceNameVarType     = 1
	resourceInitialsVarType = 2
	resourceGroupVarType    = 3
	resourceCodeVarType     = 10
	resourceEmailVarType    = 35
	resourceNotesVarType    = 20

	// Var-data type keys for the five cost rate tables (A-E) and the
	// availability table — see readResourceCostRateTables and
	// readResourceAvailability.
	resourceCostRateAVarType    = 61
	resourceAvailabilityVarType = 276

	// Field IDs (low 16 bits of a resource field-map entry). See fieldmap.go.
	resourceFieldIDUniqueID     = 27
	resourceFieldIDID           = 0
	resourceFieldIDMaxUnits     = 4
	resourceFieldIDStandardRate = 6
	resourceFieldIDOvertimeRate = 7
	resourceFieldIDCostPerUse   = 18
	resourceFieldIDWork         = 13
	resourceFieldIDCost         = 12
	resourceFieldIDBaselineWork = 15
	resourceFieldIDBaselineCost = 17

	// MPP14 default offsets within a TBkndRsc FixedData record, used when
	// the file carries no field map of its own. See NOTICE.
	resourceDefaultOffsetUniqueID     = 0
	resourceDefaultOffsetID           = 4
	resourceDefaultOffsetMaxUnits     = 44
	resourceDefaultOffsetStandardRate = 28
	resourceDefaultOffsetOvertimeRate = 36
	resourceDefaultOffsetCostPerUse   = 84
	resourceDefaultOffsetWork         = 52
	resourceDefaultOffsetCost         = 140
	resourceDefaultOffsetBaselineWork = 68
	resourceDefaultOffsetBaselineCost = 148
)

// Baseline0's Start/Finish are, per MS Project's own field map, simply never
// recorded at the resource level — only the numbered baselines (below)
// carry them. Baseline0's Work/Cost live in fixed data, read the same way
// as any other fixed-data field (see readResources); Work/Cost/Start/Finish
// for Baseline1-10 are all var data.

// resourceBaselineVarKeys are the var-data type keys for one numbered
// baseline (Baseline1..Baseline10). Sourced from MPXJ's MPP14 field map (see
// NOTICE).
type resourceBaselineVarKeys struct {
	work, cost, start, finish int
}

var resourceNumberedBaselineVarKeys = [11]resourceBaselineVarKeys{
	{}, // index 0 unused: Baseline0 is read directly, see readResources
	{work: 342, cost: 343, start: 348, finish: 349},
	{work: 352, cost: 353, start: 358, finish: 359},
	{work: 362, cost: 363, start: 368, finish: 369},
	{work: 372, cost: 373, start: 378, finish: 379},
	{work: 382, cost: 383, start: 388, finish: 389},
	{work: 392, cost: 393, start: 398, finish: 399},
	{work: 402, cost: 403, start: 408, finish: 409},
	{work: 412, cost: 413, start: 418, finish: 419},
	{work: 422, cost: 423, start: 428, finish: 429},
	{work: 432, cost: 433, start: 438, finish: 439},
}

// resourceTextVarKeys, resourceNumberVarKeys, resourceDateVarKeys and
// resourceCostVarKeys are the var-data type keys for the resource-level
// Text1..Text30, Number1..Number20, Date1..Date10 and Cost1..Cost10 generic
// custom fields, index 0 = field "1".
var resourceTextVarKeys = [30]int{
	8, 9, 30, 31, 32, 97, 98, 99, 100, 101,
	225, 226, 227, 228, 229, 230, 231, 232, 233, 234,
	235, 236, 237, 238, 239, 240, 241, 242, 243, 244,
}

var resourceNumberVarKeys = [20]int{
	112, 113, 114, 115, 116, 205, 206, 207, 208, 209,
	210, 211, 212, 213, 214, 215, 216, 217, 218, 219,
}

var resourceDateVarKeys = [10]int{173, 174, 175, 176, 177, 178, 179, 180, 181, 182}

var resourceCostVarKeys = [10]int{123, 124, 125, 166, 167, 168, 169, 170, 171, 172}

var resourceDurationVarKeys = [10][2]int{
	{117, 120}, {118, 121}, {119, 122}, {183, 245}, {184, 246},
	{185, 247}, {186, 248}, {187, 249}, {188, 250}, {189, 251},
}

// resourceOutlineCodeIndexVarKeys are the var-data type keys for
// OUTLINE_CODE1_INDEX..OUTLINE_CODE10_INDEX. See taskOutlineCodeIndexVarKeys.
var resourceOutlineCodeIndexVarKeys = [10]int{279, 281, 283, 285, 287, 289, 291, 293, 295, 297}

// resourceFlagBitOffset is the byte offset and bit mask of one Flag1..
// Flag20 custom field within a resource's primary FixedMeta record — not
// covered by the field map, so hardcoded per MS Project version like the
// task flags in task.go.
type resourceFlagBitOffset struct{ offset, mask int }

// resourceFlagBitLayout returns the bit position of Flag1..Flag20, in that
// order. Project 2013 and 2016+ share a layout; 2010 differs. Flag10's bit
// sits out of sequence relative to Flag1-9 in both layouts — a quirk of the
// format, reproduced here rather than smoothed over.
func resourceFlagBitLayout(applicationVersion int) [20]resourceFlagBitOffset {
	if applicationVersion <= appVersionProject2010 {
		return [20]resourceFlagBitOffset{
			{28, 0x0000040}, {28, 0x0000080}, {28, 0x0000100}, {28, 0x0000200}, {28, 0x0000400},
			{28, 0x0000800}, {28, 0x0001000}, {28, 0x0002000}, {28, 0x0004000}, {28, 0x0000020},
			{28, 0x0008000}, {28, 0x0010000}, {28, 0x0020000}, {28, 0x0040000}, {28, 0x0080000},
			{28, 0x0100000}, {28, 0x0200000}, {28, 0x0400000}, {28, 0x0800000}, {28, 0x1000000},
		}
	}
	return [20]resourceFlagBitOffset{
		{19, 0x0080}, {19, 0x0100}, {19, 0x0200}, {19, 0x0400}, {19, 0x0800},
		{19, 0x1000}, {19, 0x2000}, {19, 0x4000}, {19, 0x8000}, {19, 0x0040},
		{24, 0x00080}, {24, 0x00100}, {24, 0x00200}, {24, 0x00400}, {24, 0x00800},
		{24, 0x01000}, {24, 0x02000}, {24, 0x04000}, {24, 0x08000}, {24, 0x10000},
	}
}

// resourceFlagFieldIDs are Flag1..Flag20's field IDs, used only to resolve a
// user-assigned alias for the flag; the bit position itself comes from
// resourceFlagBitLayout.
var resourceFlagFieldIDs = [20]int{
	127, 128, 129, 130, 131, 132, 133, 134, 135, 126,
	195, 196, 197, 198, 199, 200, 201, 202, 203, 204,
}

// resourceTypeBitLayout returns the byte offset and bit mask of the flag
// that marks a resource as a work resource within its FixedMeta record.
// Project 2013 and 2016+ share a layout; 2010 differs.
func resourceTypeBitLayout(applicationVersion int) (offset, mask int) {
	if applicationVersion > appVersionProject2010 {
		return 12, 0x10
	}
	return 9, 0x02
}

// resourceType decides between the three resource kinds. The primary
// FixedMeta record says whether the resource is a work resource at all;
// only if it is not does a second flag, in the Fixed2Meta record, separate
// cost resources from material ones.
func resourceType(metaData, metaData2 []byte, applicationVersion int) project.ResourceType {
	offset, mask := resourceTypeBitLayout(applicationVersion)
	if getByte(metaData, offset)&mask != 0 {
		return project.WorkResource
	}
	if getByte(metaData2, 8)&0x10 != 0 {
		return project.CostResource
	}
	return project.MaterialResource
}

// readResources reads the TBkndRsc storage and returns the resources it
// defines. Each resource's CalendarUniqueID is filled in by the caller from
// the resourceCalendars map readCalendars already produced, since MS
// Project links a resource to its calendar there rather than in TBkndRsc.
func readResources(ctx *readContext, projectDirPath string) ([]*project.Resource, error) {
	src, projectProps, applicationVersion := ctx.src, ctx.props, ctx.version
	scale, aliases := ctx.scale, ctx.aliases
	dir := projectDirPath + "/TBkndRsc"

	varMetaRaw, err := src.plain(dir + "/VarMeta")
	if err != nil {
		return nil, err
	}
	varMeta, err := ParseVarMeta(varMetaRaw)
	if err != nil {
		return nil, fmt.Errorf("mpp: resource VarMeta: %w", err)
	}
	var2Raw, err := src.plain(dir + "/Var2Data")
	if err != nil {
		return nil, err
	}
	varData := ParseVar2Data(varMeta, var2Raw)

	fixedMetaRaw, err := src.plain(dir + "/FixedMeta")
	if err != nil {
		return nil, err
	}
	fixedMeta, err := ParseFixedMeta(fixedMetaRaw, 37)
	if err != nil {
		return nil, fmt.Errorf("mpp: resource FixedMeta: %w", err)
	}
	fixedRaw, err := src.decoded(dir + "/FixedData")
	if err != nil {
		return nil, err
	}
	fixedData := ParseFixedData(fixedMeta, fixedRaw, 512, 0)

	// Fixed2Meta carries the flag separating cost resources from material
	// ones. It is optional; without it such resources read as material.
	var fixed2Meta *FixedMeta
	var fixed2Data *FixedData
	if src.has(dir + "/Fixed2Meta") {
		if raw, err := src.plain(dir + "/Fixed2Meta"); err == nil {
			if meta2, err := ParseFixedMetaHeuristic(raw, fixedData.ItemCount(), 50, 51); err == nil {
				fixed2Meta = meta2
				if raw2, err := src.decoded(dir + "/Fixed2Data"); err == nil {
					fixed2Data = ParseFixedData(meta2, raw2, 0, 0)
				}
			}
		}
	}
	decoder := newFieldDecoder(ctx, resourceFieldDefs, fieldMapData(projectProps, resourceFieldMapPropsKey1, resourceFieldMapPropsKey2))
	flags := resourceFlags(applicationVersion, fixed2Meta)

	fm := loadFieldMap(projectProps, resourceFieldMapPropsKey1, resourceFieldMapPropsKey2)
	off := func(fieldID, defaultOffset int) int {
		return fieldOffset(fm, resourceFieldBase|fieldID, 0, defaultOffset)
	}
	offUniqueID := off(resourceFieldIDUniqueID, resourceDefaultOffsetUniqueID)
	offID := off(resourceFieldIDID, resourceDefaultOffsetID)
	offMaxUnits := off(resourceFieldIDMaxUnits, resourceDefaultOffsetMaxUnits)
	offStandardRate := off(resourceFieldIDStandardRate, resourceDefaultOffsetStandardRate)
	offOvertimeRate := off(resourceFieldIDOvertimeRate, resourceDefaultOffsetOvertimeRate)
	offCostPerUse := off(resourceFieldIDCostPerUse, resourceDefaultOffsetCostPerUse)
	offWork := off(resourceFieldIDWork, resourceDefaultOffsetWork)
	offCost := off(resourceFieldIDCost, resourceDefaultOffsetCost)
	// A record only has to be long enough to identify the resource. The
	// optional fields below are read through bounds-safe accessors, so a
	// short record yields a resource with those left at zero rather than
	// being dropped entirely — losing a real resource is the worse
	// outcome, and matches this reader's degrade-don't-fail stance.
	minSize := offUniqueID + 4
	if offID+4 > minSize {
		minSize = offID + 4
	}

	flagBits := resourceFlagBitLayout(applicationVersion)

	seen := make(map[int]bool)
	var resources []*project.Resource

	for i := 0; i < fixedData.ItemCount(); i++ {
		rec := fixedData.ByteArrayValue(i)
		if len(rec) < minSize {
			continue
		}

		// A resource with no var data at all is a phantom record left by a
		// delete; MPXJ reads only resources present in VarMeta too.
		uniqueID := getInt(rec, offUniqueID)
		if uniqueID <= 0 || seen[uniqueID] || len(varMeta.Types(uniqueID)) == 0 {
			continue
		}
		seen[uniqueID] = true

		metaData := fixedMeta.ByteArrayValue(i)
		var metaData2 []byte
		if fixed2Meta != nil {
			metaData2 = fixed2Meta.ByteArrayValue(i)
		}

		r := &project.Resource{
			UniqueID:     uniqueID,
			ID:           getInt(rec, offID),
			Name:         varData.UnicodeString(uniqueID, resourceNameVarType),
			Initials:     varData.UnicodeString(uniqueID, resourceInitialsVarType),
			Group:        varData.UnicodeString(uniqueID, resourceGroupVarType),
			Code:         varData.UnicodeString(uniqueID, resourceCodeVarType),
			EmailAddress: varData.UnicodeString(uniqueID, resourceEmailVarType),
			Type:         resourceType(metaData, metaData2, applicationVersion),
			MaxUnits:     getUnits(rec, offMaxUnits),
			StandardRate: getDouble(rec, offStandardRate),
			OvertimeRate: getDouble(rec, offOvertimeRate),
			CostPerUse:   getCurrency(rec, offCostPerUse),
			Work:         getWork(rec, offWork),
			Cost:         getCurrency(rec, offCost),
		}

		if rtf := varData.String(uniqueID, resourceNotesVarType); rtf != "" {
			r.RTFNotes = rtf
			r.Notes = stripRTF(rtf)
		}

		var rec2 []byte
		if fixed2Data != nil {
			rec2 = fixed2Data.ByteArrayValue(i)
		}
		r.Fields = decoder.decode([][]byte{rec, rec2}, varData, uniqueID)
		applyResourceFields(r, r.Fields, metaData2, flags, varData)

		// Baseline Work/Cost are fixed data in some files and var data in
		// others; the decoded fields cover both.
		baselineWork := fieldDuration(r.Fields, "BaselineWork")
		baselineCost := fieldFloat(r.Fields, "BaselineCost")
		if baselineWork.Amount != 0 || baselineCost != 0 {
			r.Baseline = &project.Baseline{Work: baselineWork, Cost: baselineCost}
		}
		for n := 1; n <= 10; n++ {
			if b := readResourceBaseline(varData, uniqueID, resourceNumberedBaselineVarKeys[n]); b != nil {
				if r.Baselines == nil {
					r.Baselines = make(map[int]*project.Baseline)
				}
				r.Baselines[n] = b
			}
		}

		r.CustomFields = addResourceFlags(customFields(r.Fields, resourceFieldBase, resourceFieldIndex, aliases), metaData, flagBits, aliases)
		r.CustomFields = addOutlineCodes(r.CustomFields, varData, uniqueID, resourceFieldBase, resourceOutlineCodeIndexVarKeys, aliases, ctx.values)

		for table := 0; table < 5; table++ {
			data := varData.ByteArray(uniqueID, resourceCostRateAVarType+table)
			r.CostRateTables[table] = readResourceCostRateTable(data, table, r.StandardRate, r.OvertimeRate, r.CostPerUse, scale)
		}
		// The rate fields are stored per hour; MS Project shows them in
		// their own rate units, as the cost rate tables already are.
		r.StandardRate = scale.rate(r.StandardRate, r.StandardRateUnits)
		r.OvertimeRate = scale.rate(r.OvertimeRate, r.OvertimeRateUnits)
		r.Availability = readResourceAvailability(varData.ByteArray(uniqueID, resourceAvailabilityVarType))

		resources = append(resources, r)
	}

	return resources, nil
}

// readResourceBaseline reads one numbered baseline (Baseline1..Baseline10)
// from var data, returning nil if the resource has no data for any field of
// that snapshot.
func readResourceBaseline(varData *Var2Data, uniqueID int, k resourceBaselineVarKeys) *project.Baseline {
	b := &project.Baseline{}
	has := false

	if varData.Has(uniqueID, k.work) {
		b.Work = getWork(varData.ByteArray(uniqueID, k.work), 0)
		has = true
	}
	if varData.Has(uniqueID, k.cost) {
		b.Cost = getCurrency(varData.ByteArray(uniqueID, k.cost), 0)
		has = true
	}
	if d, ok := varData.Timestamp(uniqueID, k.start); ok {
		b.Start = d
		has = true
	}
	if d, ok := varData.Timestamp(uniqueID, k.finish); ok {
		b.Finish = d
		has = true
	}

	if !has {
		return nil
	}
	return b
}

// addResourceFlags merges any set Flag1..Flag20 bits into a resource's
// custom-field map. See addTaskFlags for why an unset flag is left out
// rather than added as false.
func addResourceFlags(fields map[string]interface{}, metaData []byte, flagBits [20]resourceFlagBitOffset, aliases customFieldAliases) map[string]interface{} {
	for i, fb := range flagBits {
		if getInt(metaData, fb.offset)&fb.mask == 0 {
			continue
		}
		if fields == nil {
			fields = make(map[string]interface{})
		}
		fields[aliases.name(resourceFieldBase|resourceFlagFieldIDs[i], fmt.Sprintf("Flag%d", i+1))] = true
	}
	return fields
}

// readResourceCostRateTable parses one of a resource's five cost rate
// tables (A-E, table index 0-4) from its raw var-data blob. MS Project
// economises by not writing table A at all when it's just "the resource's
// own rate, always" — a nil blob for table A synthesizes that single
// open-ended entry from standardRate/overtimeRate/costPerUse; a nil blob
// for any other table means the user never customized it, and is left nil.
func readResourceCostRateTable(data []byte, table int, standardRate, overtimeRate, costPerUse float64, scale durationScale) []project.CostRateTableEntry {
	if data == nil {
		if table != 0 {
			return nil
		}
		return []project.CostRateTableEntry{{
			StandardRate: standardRate, StandardRateUnits: project.Hours,
			OvertimeRate: overtimeRate, OvertimeRateUnits: project.Hours,
			CostPerUse: costPerUse,
		}}
	}

	type row struct {
		end   time.Time // zero means unbounded
		entry project.CostRateTableEntry
	}
	var rows []row

	for i := 16; i+44 <= len(data); i += 44 {
		end := getTimestampFromTenths(data, i+40)
		if end.After(costRateEndDateNA) {
			end = time.Time{}
		} else {
			// MPP files only store the end of a range, typically as the
			// last minute of it (07:59, with the next range starting at
			// 08:00), but occasionally as the next range's own start time
			// instead. A minute divisible by 5 looks like a start time, so
			// it is shifted back by one minute to match the usual case.
			if end.Minute()%5 == 0 {
				end = end.Add(-time.Minute)
			}
		}
		// A timestamp with a non-zero seconds component doesn't fit either
		// pattern above and is likely corrupt; skip the entry rather than
		// keep a nonsensical date.
		if end.Second() != 0 {
			continue
		}

		standardUnits := workTimeUnit(getShort(data, i+8))
		overtimeUnits := workTimeUnit(getShort(data, i+24))

		rows = append(rows, row{
			end: end,
			entry: project.CostRateTableEntry{
				End:               end,
				StandardRate:      scale.rate(getDouble(data, i), standardUnits),
				StandardRateUnits: standardUnits,
				OvertimeRate:      scale.rate(getDouble(data, i+16), overtimeUnits),
				OvertimeRateUnits: overtimeUnits,
				CostPerUse:        getDouble(data, i+32) / 100,
			},
		})
	}
	if len(rows) == 0 {
		return nil
	}

	sort.Slice(rows, func(a, b int) bool {
		if rows[a].end.IsZero() != rows[b].end.IsZero() {
			return rows[b].end.IsZero() // an unbounded end date sorts last
		}
		return rows[a].end.Before(rows[b].end)
	})

	entries := make([]project.CostRateTableEntry, len(rows))
	for i, r := range rows {
		e := r.entry
		if i > 0 && !rows[i-1].end.IsZero() {
			e.Start = rows[i-1].end.Add(time.Minute)
		}
		entries[i] = e
	}
	return entries
}

// readResourceAvailability parses a resource's availability table from its
// raw var-data blob. A nil blob means the resource has no table of its own
// (MaxUnits then applies for its entire lifetime).
func readResourceAvailability(data []byte) []project.AvailabilityEntry {
	if len(data) < 12 {
		return nil
	}
	items := getShort(data, 0)

	var entries []project.AvailabilityEntry
	offset := 12
	for i := 0; i < items && offset+24 <= len(data); i++ {
		units := getDouble(data, offset+4)
		if units != 0 {
			start := getTimestampFromTenths(data, offset)
			end := getTimestampFromTenths(data, offset+20).Add(-time.Minute)

			if !start.After(resourceAvailabilityStartNA) {
				start = time.Time{}
			}
			if end.After(costRateEndDateNA) {
				end = time.Time{}
			}

			entries = append(entries, project.AvailabilityEntry{Start: start, End: end, MaxUnits: units / 100})
		}
		offset += 20
	}

	sort.Slice(entries, func(a, b int) bool {
		if entries[a].Start.IsZero() != entries[b].Start.IsZero() {
			return entries[a].Start.IsZero() // an unbounded start date sorts first
		}
		return entries[a].Start.Before(entries[b].Start)
	})
	return entries
}

// resourceAvailabilityStartNA is MPXJ's sentinel for "no lower bound" in an
// availability table entry's start date. See costRateEndDateNA.
var resourceAvailabilityStartNA = time.Date(1984, 1, 1, 0, 0, 0, 0, time.UTC)

// resourceHyperlinkVarType is the var-data key of a resource's hyperlink block.
const resourceHyperlinkVarType = 136

// resourceMetaFlags are the resource bit flags in Fixed2Meta outside the
// field map (MPXJ's resource META_DATA2 flags). Project 2013+ moved the
// Generic bit depending on the record size.
type resourceMetaFlags struct {
	budget, generic metaFlag
}

func resourceFlags(applicationVersion int, fixed2Meta *FixedMeta) resourceMetaFlags {
	if applicationVersion <= appVersionProject2010 {
		return resourceMetaFlags{budget: metaFlag{8, 0x20}, generic: metaFlag{32, 0x04000000}}
	}
	if fixed2Meta != nil && len(fixed2Meta.ByteArrayValue(0)) == 51 {
		return resourceMetaFlags{budget: metaFlag{8, 0x40}, generic: metaFlag{32, -0x80000000}}
	}
	return resourceMetaFlags{budget: metaFlag{8, 0x40}, generic: metaFlag{32, 0x10000000}}
}

// applyResourceFields fills the typed resource fields that come from the
// decoded field map and the meta-data flags.
func applyResourceFields(r *project.Resource, f map[string]interface{}, meta2 []byte, flags resourceMetaFlags, varData *Var2Data) {
	r.GUID = fieldString(f, "GUID")
	r.CanLevel = fieldBool(f, "CanLevel")
	r.AccrueAt = fieldString(f, "AccrueAt")
	r.Phonetics = fieldString(f, "Phonetics")
	r.NTAccount = fieldString(f, "WindowsUserAccount")
	r.MaterialLabel = fieldString(f, "MaterialLabel")
	r.BookingType = fieldString(f, "BookingType")
	r.StandardRateUnits = fieldUnit(f, "StandardRateUnits", project.Hours)
	r.OvertimeRateUnits = fieldUnit(f, "OvertimeRateUnits", project.Hours)
	r.PeakUnits = fieldFloat(f, "Peak")
	r.RegularWork = fieldDuration(f, "RegularWork")
	r.ActualWork = fieldDuration(f, "ActualWork")
	r.RemainingWork = fieldDuration(f, "RemainingWork")
	r.OvertimeWork = fieldDuration(f, "OvertimeWork")
	r.ActualOvertimeWork = fieldDuration(f, "ActualOvertimeWork")
	r.ActualCost = fieldFloat(f, "ActualCost")
	r.RemainingCost = fieldFloat(f, "RemainingCost")
	r.OvertimeCost = fieldFloat(f, "OvertimeCost")
	r.BCWS = fieldFloat(f, "BCWS")
	r.BCWP = fieldFloat(f, "BCWP")
	r.ACWP = fieldFloat(f, "ACWP")
	r.Start = fieldTime(f, "Start")
	r.Finish = fieldTime(f, "Finish")
	r.AvailableFrom = fieldTime(f, "AvailableFrom")
	r.AvailableTo = fieldTime(f, "AvailableTo")
	r.Created = fieldTime(f, "Created")
	r.Budget = meta2 != nil && flags.budget.set(meta2)
	r.Generic = meta2 != nil && flags.generic.set(meta2)
	r.Hyperlink, r.HyperlinkAddress, r.HyperlinkSubAddress, r.HyperlinkScreenTip = readHyperlink(varData.ByteArray(r.UniqueID, resourceHyperlinkVarType))
}
