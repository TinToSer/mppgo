// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"fmt"

	"github.com/tintoser/mppgo/project"
)

const (
	// assignmentFixedRecordSize is the size of a TBkndAssn FixedData
	// record. Unlike tasks/resources, MPXJ reads this stream as fixed-size
	// records rather than locating them via FixedMeta offsets — the
	// FixedMeta block is used only for its per-item deleted flag and byte
	// offset (which, divided by this size, gives the FixedData index).
	assignmentFixedRecordSize = 110

	// Field IDs (low 16 bits of an assignment field-map entry). See
	// fieldmap.go.
	assignmentFieldIDUniqueID   = 0
	assignmentFieldIDTaskID     = 1
	assignmentFieldIDResourceID = 2
	assignmentFieldIDUnits      = 7
	assignmentFieldIDWork       = 8
	assignmentFieldIDStart      = 20
	assignmentFieldIDFinish     = 21

	// MPP14 default offsets within a TBkndAssn FixedData record, used when
	// the file carries no field map of its own. See NOTICE.
	assignmentDefaultOffsetUniqueID   = 0
	assignmentDefaultOffsetTaskID     = 4
	assignmentDefaultOffsetResourceID = 8
	assignmentDefaultOffsetUnits      = 46
	assignmentDefaultOffsetWork       = 54
	assignmentDefaultOffsetStart      = 12
	assignmentDefaultOffsetFinish     = 16

	assignmentNullResourceID = -65535

	assignmentNotesVarType = 71

	// Baseline0 (the primary, unnumbered baseline) var-data type keys.
	assignmentBaselineStartVarType  = 146
	assignmentBaselineFinishVarType = 147
	assignmentBaselineWorkVarType   = 16
	assignmentBaselineCostVarType   = 32
)

// assignmentBaselineVarKeys are the var-data type keys for one numbered
// baseline (Baseline1..Baseline10). Sourced from MPXJ's MPP14 field map (see
// NOTICE).
type assignmentBaselineVarKeys struct {
	start, finish, work, cost int
}

var assignmentNumberedBaselineVarKeys = [11]assignmentBaselineVarKeys{
	{}, // index 0 unused: Baseline0 uses the assignmentBaseline* constants above
	{start: 295, finish: 296, work: 289, cost: 290},
	{start: 304, finish: 305, work: 298, cost: 299},
	{start: 313, finish: 314, work: 307, cost: 308},
	{start: 322, finish: 323, work: 316, cost: 317},
	{start: 331, finish: 332, work: 325, cost: 326},
	{start: 340, finish: 341, work: 334, cost: 335},
	{start: 349, finish: 350, work: 343, cost: 344},
	{start: 358, finish: 359, work: 352, cost: 353},
	{start: 367, finish: 368, work: 361, cost: 362},
	{start: 376, finish: 377, work: 370, cost: 371},
}

// readAssignments reads the TBkndAssn storage and returns the resource
// assignments it defines. TaskUniqueID/ResourceUniqueID are returned as
// found in the file; the caller can cross-reference them against
// File.TaskByID/ResourceByID.
func readAssignments(src *streamSource, projectDirPath string, projectProps *Props, pf *project.File) ([]*project.Assignment, error) {
	dir := projectDirPath + "/TBkndAssn"

	varMetaRaw, err := src.plain(dir + "/VarMeta")
	if err != nil {
		return nil, err
	}
	varMeta, err := ParseVarMeta(varMetaRaw)
	if err != nil {
		return nil, fmt.Errorf("mpp: assignment VarMeta: %w", err)
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
	fixedMeta, err := ParseFixedMeta(fixedMetaRaw, 34)
	if err != nil {
		return nil, fmt.Errorf("mpp: assignment FixedMeta: %w", err)
	}
	fixedRaw, err := src.decoded(dir + "/FixedData")
	if err != nil {
		return nil, err
	}
	fixedData := ParseFixedDataFixedSize(fixedRaw, assignmentFixedRecordSize)

	fm := loadFieldMap(projectProps, assignmentFieldMapPropsKey1, assignmentFieldMapPropsKey2)
	off := func(fieldID, defaultOffset int) int {
		return fieldOffset(fm, assignmentFieldBase|fieldID, defaultOffset)
	}
	offUniqueID := off(assignmentFieldIDUniqueID, assignmentDefaultOffsetUniqueID)
	offTaskID := off(assignmentFieldIDTaskID, assignmentDefaultOffsetTaskID)
	offResourceID := off(assignmentFieldIDResourceID, assignmentDefaultOffsetResourceID)
	offUnits := off(assignmentFieldIDUnits, assignmentDefaultOffsetUnits)
	offWork := off(assignmentFieldIDWork, assignmentDefaultOffsetWork)
	offStart := off(assignmentFieldIDStart, assignmentDefaultOffsetStart)
	offFinish := off(assignmentFieldIDFinish, assignmentDefaultOffsetFinish)

	var assignments []*project.Assignment

	for i := 0; i < fixedMeta.AdjustedItemCount; i++ {
		meta := fixedMeta.ByteArrayValue(i)
		if len(meta) == 0 || meta[0] != 0 {
			continue // deleted
		}

		// FixedData items sit at exact multiples of the record size, so an
		// offset that is not one does not name a record at all. Rounding it
		// down would silently attribute another assignment's data to this
		// one, so it is skipped instead.
		byteOffset := getInt(meta, 4)
		if byteOffset < 0 || byteOffset%assignmentFixedRecordSize != 0 {
			continue
		}
		data := fixedData.ByteArrayValue(byteOffset / assignmentFixedRecordSize)
		if data == nil {
			continue
		}

		uniqueID := getInt(data, offUniqueID)
		if len(varMeta.Types(uniqueID)) == 0 {
			continue // no var data: a phantom record left behind by a delete
		}

		resourceID := getInt(data, offResourceID)
		if resourceID == assignmentNullResourceID || resourceID <= 0 {
			continue
		}

		a := &project.Assignment{
			UniqueID:         uniqueID,
			TaskUniqueID:     getInt(data, offTaskID),
			ResourceUniqueID: resourceID,
			Units:            getUnits(data, offUnits),
			Work:             getWork(data, offWork),
		}
		if d, ok := getTimestamp(data, offStart); ok {
			a.Start = d
		}
		if d, ok := getTimestamp(data, offFinish); ok {
			a.Finish = d
		}

		if rtf := varData.String(uniqueID, assignmentNotesVarType); rtf != "" {
			a.RTFNotes = rtf
			a.Notes = stripRTF(rtf)
		}

		a.Baseline = readAssignmentBaseline(varData, uniqueID, assignmentBaselineVarKeys{
			start: assignmentBaselineStartVarType, finish: assignmentBaselineFinishVarType,
			work: assignmentBaselineWorkVarType, cost: assignmentBaselineCostVarType,
		})
		for n := 1; n <= 10; n++ {
			if b := readAssignmentBaseline(varData, uniqueID, assignmentNumberedBaselineVarKeys[n]); b != nil {
				if a.Baselines == nil {
					a.Baselines = make(map[int]*project.Baseline)
				}
				a.Baselines[n] = b
			}
		}

		cal := pf.AssignmentCalendar(a)
		a.TimephasedWork = readTimephasedPlannedWork(cal, a.Start, a.Finish, varData.ByteArray(uniqueID, assignmentTimephasedPlannedWorkVarType))

		if w := readTimephasedBaselineWork(cal, varData.ByteArray(uniqueID, assignmentTimephasedBaselineWorkVarType)); w != nil {
			a.TimephasedBaselineWork = map[int][]project.TimephasedWork{0: w}
		}
		if c := readTimephasedBaselineCost(cal, varData.ByteArray(uniqueID, assignmentTimephasedBaselineCostVarType)); c != nil {
			a.TimephasedBaselineCost = map[int][]project.TimephasedCost{0: c}
		}
		for n := 1; n <= 10; n++ {
			keys := assignmentTimephasedBaselineVarKeys[n]
			if w := readTimephasedBaselineWork(cal, varData.ByteArray(uniqueID, keys[0])); w != nil {
				if a.TimephasedBaselineWork == nil {
					a.TimephasedBaselineWork = make(map[int][]project.TimephasedWork)
				}
				a.TimephasedBaselineWork[n] = w
			}
			if c := readTimephasedBaselineCost(cal, varData.ByteArray(uniqueID, keys[1])); c != nil {
				if a.TimephasedBaselineCost == nil {
					a.TimephasedBaselineCost = make(map[int][]project.TimephasedCost)
				}
				a.TimephasedBaselineCost[n] = c
			}
		}

		assignments = append(assignments, a)
	}

	return assignments, nil
}

// readAssignmentBaseline reads one baseline snapshot (the primary baseline
// or one of the ten numbered ones) from var data, returning nil if the
// assignment has no data for any field of that snapshot.
func readAssignmentBaseline(varData *Var2Data, uniqueID int, k assignmentBaselineVarKeys) *project.Baseline {
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
