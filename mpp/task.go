// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"fmt"
	"sort"
	"time"

	"github.com/tintoser/mppgo/project"
)

const (
	taskWBSVarType   = 16
	taskNameVarType  = 14
	taskNotesVarType = 15

	// Baseline0 (the primary, unnumbered baseline) var-data type keys.
	taskBaselineStartVarType         = 43
	taskBaselineFinishVarType        = 44
	taskBaselineDurationVarType      = 27
	taskBaselineDurationUnitsVarType = 179
	taskBaselineWorkVarType          = 1
	taskBaselineCostVarType          = 6

	// Field IDs (low 16 bits of a task field-map entry), used to look up
	// each field's real per-file offset. See fieldmap.go.
	taskFieldIDUniqueID         = 86
	taskFieldIDID               = 23
	taskFieldIDLateStart        = 39
	taskFieldIDParentUniqueID   = 160
	taskFieldIDOutlineLevel     = 249
	taskFieldIDPercentComplete  = 32
	taskFieldIDCalendarUniqueID = 401
	taskFieldIDLateFinish       = 40
	taskFieldIDDuration         = 29
	taskFieldIDDurationUnits    = 181
	taskFieldIDEarlyStart       = 37
	taskFieldIDEarlyFinish      = 38
	taskFieldIDActualStart      = 41
	taskFieldIDActualFinish     = 42
	taskFieldIDWork             = 0
	taskFieldIDActualWork       = 2
	taskFieldIDCost             = 5
	taskFieldIDConstraintType   = 17
	taskFieldIDConstraintDate   = 18
	taskFieldIDPriority         = 25
	taskFieldIDFreeSlack        = 21
	taskFieldIDStartSlack       = 438
	taskFieldIDFinishSlack      = 439
	taskFieldIDActualDuration   = 28
	taskFieldIDRemainingDur     = 31
	taskFieldIDPercentWorkDone  = 33
	taskFieldIDType             = 128
	taskFieldIDCreated          = 93
	taskFieldIDDeadline         = 437
	taskFieldIDRemainingWork    = 4
	taskFieldIDFixedCost        = 8
	taskFieldIDActualCost       = 7
	taskFieldIDRemainingCost    = 10
	// Start/Finish live in block 1 (Fixed2Data), unlike everything above:
	// they are the dates MS Project currently shows for the task, as
	// opposed to the critical-path EARLY_START/EARLY_FINISH pair in
	// block 0.
	taskFieldIDStart  = 1283
	taskFieldIDFinish = 1284

	// MPP14 default offsets within a TBkndTask FixedData record (block 0),
	// used when the file carries no field map of its own. See NOTICE.
	taskDefaultOffsetUniqueID         = 0
	taskDefaultOffsetID               = 4
	taskDefaultOffsetLateStart        = 12
	taskDefaultOffsetParentUniqueID   = 36
	taskDefaultOffsetOutlineLevel     = 40
	taskDefaultOffsetPercentComplete  = 90
	taskDefaultOffsetLateFinish       = 110
	taskDefaultOffsetCalendarUniqueID = 118
	taskDefaultOffsetDuration         = 42
	taskDefaultOffsetDurationUnits    = 46
	taskDefaultOffsetEarlyStart       = 106
	taskDefaultOffsetEarlyFinish      = 8
	taskDefaultOffsetActualStart      = 72
	taskDefaultOffsetActualFinish     = 76
	taskDefaultOffsetWork             = 126
	taskDefaultOffsetActualWork       = 134
	taskDefaultOffsetCost             = 150
	taskDefaultOffsetConstraintType   = 56
	taskDefaultOffsetConstraintDate   = 80
	taskDefaultOffsetPriority         = 88
	taskDefaultOffsetFreeSlack        = 24
	taskDefaultOffsetStartSlack       = 28
	taskDefaultOffsetFinishSlack      = 32
	taskDefaultOffsetActualDuration   = 48
	taskDefaultOffsetRemainingDur     = 52
	taskDefaultOffsetPercentWorkDone  = 92
	taskDefaultOffsetType             = 94
	taskDefaultOffsetCreated          = 98
	taskDefaultOffsetDeadline         = 122
	taskDefaultOffsetRemainingWork    = 142
	taskDefaultOffsetFixedCost        = 158
	taskDefaultOffsetActualCost       = 166
	taskDefaultOffsetRemainingCost    = 174

	// Default offsets within a TBkndTask Fixed2Data record (block 1).
	taskDefault2OffsetStart  = 50
	taskDefault2OffsetFinish = 54

	// Deleted and null-placeholder tasks have their ID/UniqueID at these
	// fixed offsets and are skipped rather than modelled.
	taskNullBlockSize   = 16
	taskDeletedFlagMask = 0x02
)

// taskBaselineVarKeys are the var-data type keys for one numbered baseline
// (Baseline1..Baseline10), in the same {start, finish, duration,
// durationUnits, work, cost} order as the taskBaseline* constants above,
// which cover Baseline0 (the primary baseline). Sourced from MPXJ's MPP14
// field map (see NOTICE); the numbering is not a fixed stride, so each
// baseline's keys are listed explicitly rather than computed.
type taskBaselineVarKeys struct {
	start, finish, duration, durationUnits, work, cost int
}

var taskNumberedBaselineVarKeys = [11]taskBaselineVarKeys{
	{}, // index 0 unused: Baseline0 uses the taskBaseline* constants above
	{start: 482, finish: 483, duration: 487, durationUnits: 488, work: 485, cost: 484},
	{start: 493, finish: 494, duration: 498, durationUnits: 499, work: 496, cost: 495},
	{start: 504, finish: 505, duration: 509, durationUnits: 510, work: 507, cost: 506},
	{start: 515, finish: 516, duration: 520, durationUnits: 521, work: 518, cost: 517},
	{start: 526, finish: 527, duration: 531, durationUnits: 532, work: 529, cost: 528},
	{start: 544, finish: 545, duration: 549, durationUnits: 550, work: 547, cost: 546},
	{start: 555, finish: 556, duration: 560, durationUnits: 561, work: 558, cost: 557},
	{start: 566, finish: 567, duration: 571, durationUnits: 572, work: 569, cost: 568},
	{start: 577, finish: 578, duration: 582, durationUnits: 583, work: 580, cost: 579},
	{start: 588, finish: 589, duration: 593, durationUnits: 594, work: 591, cost: 590},
}

// taskTextVarKeys, taskNumberVarKeys, taskDateVarKeys and
// taskDurationVarKeys are the var-data type keys for the Text1..Text30,
// Number1..Number20, Date1..Date10 and Duration1..Duration10 (paired with
// their own units field) generic custom fields, index 0 = field "1". A
// field with no data in a given file (the common case: MS Project only
// writes an entry once a value is set) is simply absent, not zero.
var taskTextVarKeys = [30]int{
	51, 54, 57, 60, 63, 66, 67, 68, 69, 70,
	317, 318, 319, 320, 321, 322, 323, 324, 325, 326,
	327, 328, 329, 330, 331, 332, 333, 334, 335, 336,
}

var taskNumberVarKeys = [20]int{
	87, 88, 89, 90, 91, 302, 303, 304, 305, 306,
	307, 308, 309, 310, 311, 312, 313, 314, 315, 316,
}

var taskDateVarKeys = [10]int{265, 266, 267, 268, 269, 270, 271, 272, 273, 274}

var taskCostVarKeys = [10]int{106, 107, 108, 258, 259, 260, 261, 262, 263, 264}

var taskDurationVarKeys = [10][2]int{
	{103, 183}, {104, 184}, {105, 185}, {275, 337}, {276, 338},
	{277, 339}, {278, 340}, {279, 341}, {280, 342}, {281, 343},
}

// taskOutlineCodeIndexVarKeys are the var-data type keys for
// OUTLINE_CODE1_INDEX..OUTLINE_CODE10_INDEX — the value actually stored on
// a task is a unique ID into the project-wide outline code value table
// (see outlinecodes.go), resolved to a path via resolveOutlineCodePath. The
// base OUTLINE_CODEn field (for alias lookups) is always one less than its
// own _INDEX field's ID.
var taskOutlineCodeIndexVarKeys = [10]int{417, 419, 421, 423, 425, 427, 429, 431, 433, 435}

// taskMilestoneBitLayout returns the byte offset and bit mask of the
// MILESTONE flag within a task's 47-byte FixedMeta record. Project 2013
// and 2016+ share a layout; 2010 differs. Unlike the fields above, this bit
// flag is not covered by the field map: MPXJ hardcodes its location too.
func taskMilestoneBitLayout(applicationVersion int) (offset, mask int) {
	if applicationVersion <= appVersionProject2010 {
		return 8, 0x20
	}
	return 10, 0x02
}

// taskActiveBitLayout returns the byte offset and bit mask of the ACTIVE
// flag within a task's Fixed2Meta record (distinct from the MILESTONE
// flag's home in the primary FixedMeta record above). A clear bit means
// the task has been explicitly deactivated in MS Project (available since
// Project 2010) — MS Project then blanks that task's Start/Finish while
// leaving LateStart/LateFinish as whatever they were before deactivation.
// Project 2013 and 2016+ share a layout; 2010 differs.
func taskActiveBitLayout(applicationVersion int) (offset, mask int) {
	if applicationVersion <= appVersionProject2010 {
		return 8, 0x04
	}
	return 8, 0x40
}

// taskFlagBitOffset is the byte offset and bit mask of one Flag1..Flag20
// custom field within a task's primary FixedMeta record — like MILESTONE
// and ACTIVE above, these bits are not covered by the field map and must be
// hardcoded per MS Project version.
type taskFlagBitOffset struct{ offset, mask int }

// taskFlagBitLayout returns the bit position of Flag1..Flag20, in that
// order. Project 2013 and 2016+ share a layout; 2010 differs.
func taskFlagBitLayout(applicationVersion int) [20]taskFlagBitOffset {
	if applicationVersion <= appVersionProject2010 {
		return [20]taskFlagBitOffset{
			{35, 0x0000040}, {35, 0x0000080}, {35, 0x0000100}, {35, 0x0000200}, {35, 0x0000400},
			{35, 0x0000800}, {35, 0x0001000}, {35, 0x0002000}, {35, 0x0004000}, {35, 0x0008000},
			{35, 0x0010000}, {35, 0x0020000}, {35, 0x0040000}, {35, 0x0080000}, {35, 0x0100000},
			{35, 0x0200000}, {35, 0x0400000}, {35, 0x0800000}, {35, 0x1000000}, {35, 0x2000000},
		}
	}
	return [20]taskFlagBitOffset{
		{24, 0x0002}, {24, 0x0004}, {24, 0x0008}, {24, 0x0010}, {24, 0x0020},
		{24, 0x0040}, {24, 0x0080}, {24, 0x0100}, {24, 0x0200}, {24, 0x0400},
		{33, 0x002}, {33, 0x004}, {33, 0x008}, {33, 0x010}, {33, 0x020},
		{33, 0x040}, {33, 0x080}, {33, 0x100}, {33, 0x200}, {33, 0x400},
	}
}

// taskFlagFieldIDs are Flag1..Flag20's field IDs, used only to resolve a
// user-assigned alias for the flag (see readCustomFieldAliases); the bit
// position itself comes from taskFlagBitLayout, not the field map.
var taskFlagFieldIDs = [20]int{
	72, 73, 74, 75, 76, 77, 78, 79, 80, 81,
	292, 293, 294, 295, 296, 297, 298, 299, 300, 301,
}

// readTasks reads the TBkndTask storage and returns the tasks it defines.
// Summary is derived after the fact: MS Project does not store it directly,
// a task is a summary task exactly when some other task names it as parent.
func readTasks(src *streamSource, projectDirPath string, projectProps *Props, applicationVersion int, scale durationScale, defaultUnits project.TimeUnit, aliases customFieldAliases, outlineCodeValues map[int]outlineCodeValue) ([]*project.Task, error) {
	dir := projectDirPath + "/TBkndTask"

	varMetaRaw, err := src.plain(dir + "/VarMeta")
	if err != nil {
		return nil, err
	}
	varMeta, err := ParseVarMeta(varMetaRaw)
	if err != nil {
		return nil, fmt.Errorf("mpp: task VarMeta: %w", err)
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
	fixedMeta, err := ParseFixedMeta(fixedMetaRaw, 47)
	if err != nil {
		return nil, fmt.Errorf("mpp: task FixedMeta: %w", err)
	}
	fixedRaw, err := src.decoded(dir + "/FixedData")
	if err != nil {
		return nil, err
	}
	fixedData := ParseFixedData(fixedMeta, fixedRaw, 512, 0)

	// Fixed2Data carries the Start/Finish pair, and Fixed2Meta itself (kept
	// alongside it, not just used to locate it) carries the ACTIVE bit.
	// Fixed2Meta's item size varies by file version, so it is picked
	// heuristically against the FixedData item count already established
	// above, the same way calendar GUIDs' Fixed2Meta is handled in
	// calendar.go.
	var fixed2Meta *FixedMeta
	var fixed2Data *FixedData
	if src.has(dir + "/Fixed2Meta") {
		if raw, err := src.plain(dir + "/Fixed2Meta"); err == nil {
			if meta2, err := ParseFixedMetaHeuristic(raw, fixedData.ItemCount(), 92, 93, 94, 95, 96); err == nil {
				fixed2Meta = meta2
				if raw2, err := src.decoded(dir + "/Fixed2Data"); err == nil {
					fixed2Data = ParseFixedData(meta2, raw2, 128, 0)
				}
			}
		}
	}

	fm := loadFieldMap(projectProps, taskFieldMapPropsKey1, taskFieldMapPropsKey2)
	off := func(fieldID, defaultOffset int) int {
		return fieldOffset(fm, taskFieldBase|fieldID, defaultOffset)
	}
	offUniqueID := off(taskFieldIDUniqueID, taskDefaultOffsetUniqueID)
	offID := off(taskFieldIDID, taskDefaultOffsetID)
	offLateStart := off(taskFieldIDLateStart, taskDefaultOffsetLateStart)
	offParentUniqueID := off(taskFieldIDParentUniqueID, taskDefaultOffsetParentUniqueID)
	offOutlineLevel := off(taskFieldIDOutlineLevel, taskDefaultOffsetOutlineLevel)
	offPercentComplete := off(taskFieldIDPercentComplete, taskDefaultOffsetPercentComplete)
	offLateFinish := off(taskFieldIDLateFinish, taskDefaultOffsetLateFinish)
	offCalendarUniqueID := off(taskFieldIDCalendarUniqueID, taskDefaultOffsetCalendarUniqueID)
	off2Start := off(taskFieldIDStart, taskDefault2OffsetStart)
	off2Finish := off(taskFieldIDFinish, taskDefault2OffsetFinish)
	offDuration := off(taskFieldIDDuration, taskDefaultOffsetDuration)
	offDurationUnits := off(taskFieldIDDurationUnits, taskDefaultOffsetDurationUnits)
	offEarlyStart := off(taskFieldIDEarlyStart, taskDefaultOffsetEarlyStart)
	offEarlyFinish := off(taskFieldIDEarlyFinish, taskDefaultOffsetEarlyFinish)
	offActualStart := off(taskFieldIDActualStart, taskDefaultOffsetActualStart)
	offActualFinish := off(taskFieldIDActualFinish, taskDefaultOffsetActualFinish)
	offWork := off(taskFieldIDWork, taskDefaultOffsetWork)
	offActualWork := off(taskFieldIDActualWork, taskDefaultOffsetActualWork)
	offCost := off(taskFieldIDCost, taskDefaultOffsetCost)
	offConstraintType := off(taskFieldIDConstraintType, taskDefaultOffsetConstraintType)
	offConstraintDate := off(taskFieldIDConstraintDate, taskDefaultOffsetConstraintDate)
	offPriority := off(taskFieldIDPriority, taskDefaultOffsetPriority)
	offFreeSlack := off(taskFieldIDFreeSlack, taskDefaultOffsetFreeSlack)
	offStartSlack := off(taskFieldIDStartSlack, taskDefaultOffsetStartSlack)
	offFinishSlack := off(taskFieldIDFinishSlack, taskDefaultOffsetFinishSlack)
	offActualDuration := off(taskFieldIDActualDuration, taskDefaultOffsetActualDuration)
	offRemainingDur := off(taskFieldIDRemainingDur, taskDefaultOffsetRemainingDur)
	offPercentWorkDone := off(taskFieldIDPercentWorkDone, taskDefaultOffsetPercentWorkDone)
	offType := off(taskFieldIDType, taskDefaultOffsetType)
	offCreated := off(taskFieldIDCreated, taskDefaultOffsetCreated)
	offDeadline := off(taskFieldIDDeadline, taskDefaultOffsetDeadline)
	offRemainingWork := off(taskFieldIDRemainingWork, taskDefaultOffsetRemainingWork)
	offFixedCost := off(taskFieldIDFixedCost, taskDefaultOffsetFixedCost)
	offActualCost := off(taskFieldIDActualCost, taskDefaultOffsetActualCost)
	offRemainingCost := off(taskFieldIDRemainingCost, taskDefaultOffsetRemainingCost)

	milestoneOffset, milestoneMask := taskMilestoneBitLayout(applicationVersion)
	activeOffset, activeMask := taskActiveBitLayout(applicationVersion)
	flagBits := taskFlagBitLayout(applicationVersion)

	byID := make(map[int]*project.Task)
	var order []int

	itemCount := fixedData.ItemCount()
	// The first three items are header/reserved records, not tasks.
	for i := 3; i < itemCount; i++ {
		metaData := fixedMeta.ByteArrayValue(i)
		rec := fixedData.ByteArrayValue(i)
		if metaData == nil || rec == nil {
			continue
		}
		if getInt(metaData, 0)&taskDeletedFlagMask != 0 {
			continue
		}
		if len(rec) == taskNullBlockSize {
			continue // placeholder task; not modelled
		}

		uniqueID := getInt(rec, offUniqueID)
		if uniqueID <= 0 {
			continue
		}

		// One units field governs the task's duration and every duration
		// derived from it (slack, actual, remaining).
		durationUnits := durationTimeUnits(getShort(rec, offDurationUnits), defaultUnits)

		t := &project.Task{
			UniqueID:            uniqueID,
			ID:                  getInt(rec, offID),
			OutlineLevel:        getShort(rec, offOutlineLevel),
			ParentUniqueID:      getInt(rec, offParentUniqueID),
			PercentComplete:     float64(taskPercentage(rec, offPercentComplete)),
			CalendarUniqueID:    taskCalendarUniqueID(getInt(rec, offCalendarUniqueID)),
			WBS:                 varData.UnicodeString(uniqueID, taskWBSVarType),
			Name:                varData.UnicodeString(uniqueID, taskNameVarType),
			Duration:            scale.duration(getInt(rec, offDuration), durationUnits),
			Work:                getWork(rec, offWork),
			ActualWork:          getWork(rec, offActualWork),
			RemainingWork:       getWork(rec, offRemainingWork),
			Cost:                getCurrency(rec, offCost),
			FixedCost:           getCurrency(rec, offFixedCost),
			ActualCost:          getCurrency(rec, offActualCost),
			RemainingCost:       getCurrency(rec, offRemainingCost),
			ConstraintType:      project.ConstraintType(getShort(rec, offConstraintType)),
			Priority:            getShort(rec, offPriority),
			PercentWorkComplete: float64(taskPercentage(rec, offPercentWorkDone)),
			Type:                taskType(getShort(rec, offType)),
			// Slack and the actual/remaining durations share the task's
			// own duration-units field.
			FreeSlack:         scale.duration(getInt(rec, offFreeSlack), durationUnits),
			StartSlack:        scale.duration(getInt(rec, offStartSlack), durationUnits),
			FinishSlack:       scale.duration(getInt(rec, offFinishSlack), durationUnits),
			ActualDuration:    scale.duration(getInt(rec, offActualDuration), durationUnits),
			RemainingDuration: scale.duration(getInt(rec, offRemainingDur), durationUnits),
		}
		for _, f := range []struct {
			dst    *time.Time
			offset int
		}{
			{&t.LateStart, offLateStart},
			{&t.LateFinish, offLateFinish},
			{&t.EarlyStart, offEarlyStart},
			{&t.EarlyFinish, offEarlyFinish},
			{&t.ActualStart, offActualStart},
			{&t.ActualFinish, offActualFinish},
			{&t.ConstraintDate, offConstraintDate},
			{&t.Deadline, offDeadline},
			{&t.Created, offCreated},
		} {
			if d, ok := getTimestamp(rec, f.offset); ok {
				*f.dst = d
			}
		}
		if fixed2Data != nil {
			if rec2 := fixed2Data.ByteArrayValue(i); rec2 != nil {
				if d, ok := getTimestamp(rec2, off2Start); ok {
					t.Start = d
				}
				if d, ok := getTimestamp(rec2, off2Finish); ok {
					t.Finish = d
				}
			}
		}
		if fixed2Meta != nil {
			if metaData2 := fixed2Meta.ByteArrayValue(i); metaData2 != nil {
				t.Inactive = getInt(metaData2, activeOffset)&activeMask == 0
			}
		}
		if getInt(metaData, milestoneOffset)&milestoneMask != 0 {
			t.Milestone = true
		}

		if rtf := varData.String(uniqueID, taskNotesVarType); rtf != "" {
			t.RTFNotes = rtf
			t.Notes = stripRTF(rtf)
		}

		t.Baseline = readTaskBaseline(varData, uniqueID, taskBaselineVarKeys{
			start: taskBaselineStartVarType, finish: taskBaselineFinishVarType,
			duration: taskBaselineDurationVarType, durationUnits: taskBaselineDurationUnitsVarType,
			work: taskBaselineWorkVarType, cost: taskBaselineCostVarType,
		}, scale, defaultUnits)
		for n := 1; n <= 10; n++ {
			if b := readTaskBaseline(varData, uniqueID, taskNumberedBaselineVarKeys[n], scale, defaultUnits); b != nil {
				if t.Baselines == nil {
					t.Baselines = make(map[int]*project.Baseline)
				}
				t.Baselines[n] = b
			}
		}

		t.CustomFields = addTaskFlags(readTaskCustomFields(varData, uniqueID, aliases, scale, defaultUnits, outlineCodeValues), metaData, flagBits, aliases)

		if _, exists := byID[uniqueID]; !exists {
			order = append(order, uniqueID)
		}
		byID[uniqueID] = t // a later duplicate record is the correct one
	}

	tasks := make([]*project.Task, 0, len(order))
	for _, id := range order {
		tasks = append(tasks, byID[id])
	}

	// Return tasks in ID order — the row order MS Project displays, and
	// what MPXJ's task container settles on too. The raw FixedData order
	// they are read in carries no meaning for a caller.
	sort.SliceStable(tasks, func(i, j int) bool { return tasks[i].ID < tasks[j].ID })

	// A task is a summary task exactly when some other task names it as
	// parent; MS Project does not store this as its own flag.
	for _, t := range tasks {
		if t.ParentUniqueID <= 0 {
			continue
		}
		if parent, ok := byID[t.ParentUniqueID]; ok {
			parent.Summary = true
		}
	}

	synthesizeWBS(tasks)

	return tasks, nil
}

// taskCalendarUniqueID normalizes MPXJ's "no calendar set on this task"
// sentinel (-1) to 0, matching this reader's zero-value convention (see
// project.File.TaskCalendar).
func taskCalendarUniqueID(raw int) int {
	if raw == -1 {
		return 0
	}
	return raw
}

// taskType maps the stored task-type code. MS Project writes only three
// values here; anything else falls back to fixed work, as MPXJ does.
func taskType(code int) project.TaskType {
	switch code {
	case 0:
		return project.FixedUnits
	case 1:
		return project.FixedDuration
	default:
		return project.FixedWork
	}
}

// taskPercentage decodes a percentage stored as a raw short 0..100,
// matching MPXJ's MPPUtility.getPercentage. Out-of-range values (the "not
// applicable" case) read as 0.
func taskPercentage(data []byte, offset int) int {
	v := getShort(data, offset)
	if v < 0 || v > 100 {
		return 0
	}
	return v
}

// readTaskBaseline reads one baseline snapshot (the primary baseline or one
// of the ten numbered ones) from var data, returning nil if the task has no
// data for any field of that snapshot — the common case for a task whose
// baseline was never set (or never re-set after this baseline number was
// introduced).
func readTaskBaseline(varData *Var2Data, uniqueID int, k taskBaselineVarKeys, scale durationScale, defaultUnits project.TimeUnit) *project.Baseline {
	b := &project.Baseline{}
	has := false

	if d, ok := varData.Timestamp(uniqueID, k.start); ok {
		b.Start = d
		has = true
	}
	if d, ok := varData.Timestamp(uniqueID, k.finish); ok {
		b.Finish = d
		has = true
	}
	if varData.Has(uniqueID, k.duration) {
		units := durationTimeUnits(varData.Short(uniqueID, k.durationUnits), defaultUnits)
		b.Duration = scale.duration(varData.Int(uniqueID, k.duration), units)
		has = true
	}
	if varData.Has(uniqueID, k.work) {
		b.Work = getWork(varData.ByteArray(uniqueID, k.work), 0)
		has = true
	}
	if varData.Has(uniqueID, k.cost) {
		b.Cost = getCurrency(varData.ByteArray(uniqueID, k.cost), 0)
		has = true
	}

	if !has {
		return nil
	}
	return b
}

// readTaskCustomFields collects the task's generic custom fields (Text1-30,
// Number1-20, Date1-10, Duration1-10, Cost1-10, Outline Code1-10) into the
// map exposed as Task.CustomFields, keyed by the field's user-assigned
// alias if it has one (see readCustomFieldAliases) or its generic name
// otherwise. A field the file has no data for is left out of the map
// entirely, rather than being present with a zero value indistinguishable
// from a real zero.
func readTaskCustomFields(varData *Var2Data, uniqueID int, aliases customFieldAliases, scale durationScale, defaultUnits project.TimeUnit, outlineCodeValues map[int]outlineCodeValue) map[string]interface{} {
	fields := make(map[string]interface{})

	for i, key := range taskTextVarKeys {
		if v := varData.UnicodeString(uniqueID, key); v != "" {
			fields[aliases.name(taskFieldBase|key, fmt.Sprintf("Text%d", i+1))] = v
		}
	}
	for i, key := range taskNumberVarKeys {
		if varData.Has(uniqueID, key) {
			fields[aliases.name(taskFieldBase|key, fmt.Sprintf("Number%d", i+1))] = varData.Double(uniqueID, key)
		}
	}
	for i, key := range taskDateVarKeys {
		if d, ok := varData.Timestamp(uniqueID, key); ok {
			fields[aliases.name(taskFieldBase|key, fmt.Sprintf("Date%d", i+1))] = d
		}
	}
	for i, key := range taskCostVarKeys {
		if varData.Has(uniqueID, key) {
			fields[aliases.name(taskFieldBase|key, fmt.Sprintf("Cost%d", i+1))] = customFieldCurrency(varData.Double(uniqueID, key))
		}
	}
	for i, keys := range taskDurationVarKeys {
		valueKey, unitsKey := keys[0], keys[1]
		if varData.Has(uniqueID, valueKey) {
			units := durationTimeUnits(varData.Short(uniqueID, unitsKey), defaultUnits)
			fields[aliases.name(taskFieldBase|valueKey, fmt.Sprintf("Duration%d", i+1))] = scale.duration(varData.Int(uniqueID, valueKey), units)
		}
	}
	for i, key := range taskOutlineCodeIndexVarKeys {
		if !varData.Has(uniqueID, key) {
			continue
		}
		if path := resolveOutlineCodePath(outlineCodeValues, varData.Int(uniqueID, key)); path != "" {
			fields[aliases.name(taskFieldBase|(key-1), fmt.Sprintf("Outline Code%d", i+1))] = path
		}
	}

	if len(fields) == 0 {
		return nil
	}
	return fields
}

// addTaskFlags merges any set Flag1..Flag20 bits into a task's custom-field
// map, allocating it if necessary. Unlike the other custom field kinds, a
// flag is a physical bit that always has a value (MS Project defaults it to
// No); to keep CustomFields sparse and consistent with "absent means never
// set" for every other kind, an unset (false) flag is simply left out
// rather than added as false.
func addTaskFlags(fields map[string]interface{}, metaData []byte, flagBits [20]taskFlagBitOffset, aliases customFieldAliases) map[string]interface{} {
	for i, fb := range flagBits {
		if getInt(metaData, fb.offset)&fb.mask == 0 {
			continue
		}
		if fields == nil {
			fields = make(map[string]interface{})
		}
		fields[aliases.name(taskFieldBase|taskFlagFieldIDs[i], fmt.Sprintf("Flag%d", i+1))] = true
	}
	return fields
}
