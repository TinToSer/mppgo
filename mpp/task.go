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
	taskWBSVarType  = 16
	taskNameVarType = 14

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
	// Start/Finish (block 1/Fixed2Data): the date MS Project currently
	// shows for the task, as opposed to the CPM-computed
	// EARLY_START/EARLY_FINISH pair (block 0), which this reader does not
	// expose.
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

	// Default offsets within a TBkndTask Fixed2Data record (block 1).
	taskDefault2OffsetStart  = 50
	taskDefault2OffsetFinish = 54

	// Deleted and null-placeholder tasks have their ID/UniqueID at these
	// fixed offsets and are skipped rather than modelled.
	taskNullBlockSize   = 16
	taskDeletedFlagMask = 0x02
)

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

// readTasks reads the TBkndTask storage and returns the tasks it defines.
// Summary is derived after the fact: MS Project does not store it directly,
// a task is a summary task exactly when some other task names it as parent.
func readTasks(src *streamSource, projectDirPath string, projectProps *Props, applicationVersion int, scale durationScale, defaultUnits project.TimeUnit) ([]*project.Task, error) {
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

	milestoneOffset, milestoneMask := taskMilestoneBitLayout(applicationVersion)
	activeOffset, activeMask := taskActiveBitLayout(applicationVersion)

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

		t := &project.Task{
			UniqueID:         uniqueID,
			ID:               getInt(rec, offID),
			OutlineLevel:     getShort(rec, offOutlineLevel),
			ParentUniqueID:   getInt(rec, offParentUniqueID),
			PercentComplete:  float64(taskPercentage(rec, offPercentComplete)),
			CalendarUniqueID: taskCalendarUniqueID(getInt(rec, offCalendarUniqueID)),
			WBS:              varData.UnicodeString(uniqueID, taskWBSVarType),
			Name:             varData.UnicodeString(uniqueID, taskNameVarType),
			Duration: scale.duration(
				getInt(rec, offDuration),
				durationTimeUnits(getShort(rec, offDurationUnits), defaultUnits),
			),
			Work:           getWork(rec, offWork),
			ActualWork:     getWork(rec, offActualWork),
			Cost:           getCurrency(rec, offCost),
			ConstraintType: project.ConstraintType(getShort(rec, offConstraintType)),
			Priority:       getShort(rec, offPriority),
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
