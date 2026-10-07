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

	assignmentNotesVarType     = 71
	assignmentHyperlinkVarType = 150

	// assignmentFixed2RecordSize is the size of a TBkndAssn Fixed2Data
	// record, which lines up with FixedData record for record.
	assignmentFixed2RecordSize = 48

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
func readAssignments(ctx *readContext, projectDirPath string, pf *project.File) ([]*project.Assignment, error) {
	src, projectProps := ctx.src, ctx.props
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

	// Fixed2Data holds the assignment GUIDs, in fixed 48-byte records
	// parallel to FixedData. It is optional.
	var fixed2Data *FixedData
	if raw2, err := src.decoded(dir + "/Fixed2Data"); err == nil {
		fixed2Data = ParseFixedDataFixedSize(raw2, assignmentFixed2RecordSize)
	}

	fm := loadFieldMap(projectProps, assignmentFieldMapPropsKey1, assignmentFieldMapPropsKey2)
	off := func(fieldID, defaultOffset int) int {
		return fieldOffset(fm, assignmentFieldBase|fieldID, 0, defaultOffset)
	}
	offUniqueID := off(assignmentFieldIDUniqueID, assignmentDefaultOffsetUniqueID)
	offTaskID := off(assignmentFieldIDTaskID, assignmentDefaultOffsetTaskID)
	offResourceID := off(assignmentFieldIDResourceID, assignmentDefaultOffsetResourceID)
	offUnits := off(assignmentFieldIDUnits, assignmentDefaultOffsetUnits)
	offWork := off(assignmentFieldIDWork, assignmentDefaultOffsetWork)
	offStart := off(assignmentFieldIDStart, assignmentDefaultOffsetStart)
	offFinish := off(assignmentFieldIDFinish, assignmentDefaultOffsetFinish)

	decoder := newFieldDecoder(ctx, assignmentFieldDefs, fieldMapData(projectProps, assignmentFieldMapPropsKey1, assignmentFieldMapPropsKey2))
	decoder.alternate = assignmentAlternateFieldDefs
	flags := assignmentFlagLayout(ctx.version)

	var assignments []*project.Assignment
	seen := make(map[[2]int]bool)

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
		index := byteOffset / assignmentFixedRecordSize
		data := fixedData.ByteArrayValue(index)
		if data == nil {
			continue
		}

		uniqueID := getInt(data, offUniqueID)
		if len(varMeta.Types(uniqueID)) == 0 {
			continue // no var data: a phantom record left behind by a delete
		}

		// An assignment must belong to a task that was read. One task is
		// assigned a given resource at most once; a repeat is a stale copy.
		taskID := getInt(data, offTaskID)
		task := pf.TaskByID(taskID)
		if task == nil {
			continue
		}
		// -65535 marks an assignment with no resource: work on the task
		// that nobody is assigned to. It is kept, with ResourceUniqueID 0.
		resourceID := getInt(data, offResourceID)
		if resourceID == assignmentNullResourceID {
			resourceID = 0
		}
		if seen[[2]int{taskID, resourceID}] {
			continue
		}
		seen[[2]int{taskID, resourceID}] = true

		a := &project.Assignment{
			UniqueID:         uniqueID,
			TaskUniqueID:     taskID,
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

		var data2 []byte
		if fixed2Data != nil {
			data2 = fixed2Data.ByteArrayValue(index)
		}
		a.Fields = decoder.decode([][]byte{data, data2}, varData, uniqueID)
		applyAssignmentFields(a, a.Fields, meta, flags, varData)
		a.CustomFields = addAssignmentFlags(customFields(a.Fields, assignmentFieldBase, assignmentFieldIndex, ctx.aliases), meta, flags, ctx.aliases)

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

		readAssignmentTimephased(ctx, pf, a, task, varData)
		assignments = append(assignments, a)
	}

	return assignments, nil
}

// readAssignmentTimephased reads an assignment's timephased data: actual
// (complete) and planned (remaining) work, actual overtime, and baseline
// work and cost, matching MPXJ's ResourceAssignmentFactory. It also fills
// in the assignment's actual start/finish and work contour, which MS
// Project derives from the same data.
func readAssignmentTimephased(ctx *readContext, pf *project.File, a *project.Assignment, task *project.Task, varData *Var2Data) {
	cal := pf.AssignmentCalendar(a)
	baselineCal := pf.CalendarByName(pf.Properties.BaselineCalendarName)
	if baselineCal == nil {
		baselineCal = cal
	}
	taskHasDuration := task.Duration.Amount != 0

	irregular := varData.ByteArray(a.UniqueID, assignmentTimephasedActualIrregularWorkVarType)
	var complete []project.TimephasedWork
	if taskHasDuration {
		complete = readTimephasedCompleteWork(cal, a, varData.ByteArray(a.UniqueID, assignmentTimephasedActualWorkVarType), irregular)
		a.TimephasedActualOvertimeWork = readTimephasedCompleteWork(cal, a, varData.ByteArray(a.UniqueID, assignmentTimephasedActualOvertimeWorkVarType), irregular)
	}

	// MS Project stores no actual start/finish for an assignment: they
	// follow from the task's progress and the assignment's own work.
	if !taskHasDuration {
		if !task.ActualStart.IsZero() {
			a.ActualStart = a.Start
		}
		if !task.ActualFinish.IsZero() {
			a.ActualFinish = a.Finish
		}
	} else {
		if !task.ActualStart.IsZero() && a.RemainingWork.Amount == 0 && a.ResourceUniqueID != 0 {
			a.ActualFinish = a.Finish
		}
		if !a.ActualFinish.IsZero() || len(complete) != 0 {
			a.ActualStart = a.Start
		}
	}

	plannedData := varData.ByteArray(a.UniqueID, assignmentTimephasedPlannedWorkVarType)
	var planned []project.TimephasedWork
	if taskHasDuration {
		start := a.Start
		if len(complete) != 0 && !a.Resume.IsZero() {
			start = a.Resume
		}
		planned = readTimephasedPlannedWork(cal, start, a.Finish, plannedData)
	}
	if len(planned) == 0 && len(complete) == 0 && taskHasDuration {
		planned = flatTimephasedWork(cal, a, pf.ResourceByID(a.ResourceUniqueID))
	}
	a.TimephasedWork = planned
	if len(complete) != 0 {
		a.TimephasedActualWork = complete
	}

	if a.WorkContour == "Flat" && len(plannedData) >= 30 {
		a.WorkContour = workContourName(getShort(plannedData, 28))
	}

	if w := readTimephasedBaselineWork(baselineCal, varData.ByteArray(a.UniqueID, assignmentTimephasedBaselineWorkVarType)); w != nil {
		a.TimephasedBaselineWork = map[int][]project.TimephasedWork{0: w}
	}
	if c := readTimephasedBaselineCost(baselineCal, varData.ByteArray(a.UniqueID, assignmentTimephasedBaselineCostVarType)); c != nil {
		a.TimephasedBaselineCost = map[int][]project.TimephasedCost{0: c}
	}
	for n := 1; n <= 10; n++ {
		keys := assignmentTimephasedBaselineVarKeys[n]
		if w := readTimephasedBaselineWork(baselineCal, varData.ByteArray(a.UniqueID, keys[0])); w != nil {
			if a.TimephasedBaselineWork == nil {
				a.TimephasedBaselineWork = make(map[int][]project.TimephasedWork)
			}
			a.TimephasedBaselineWork[n] = w
		}
		if c := readTimephasedBaselineCost(baselineCal, varData.ByteArray(a.UniqueID, keys[1])); c != nil {
			if a.TimephasedBaselineCost == nil {
				a.TimephasedBaselineCost = make(map[int][]project.TimephasedCost)
			}
			a.TimephasedBaselineCost[n] = c
		}
	}
}

// flatTimephasedWork is the single span MS Project implies for an
// assignment that stores no timephased data of its own: all its work
// spread evenly from start to finish.
func flatTimephasedWork(cal *project.Calendar, a *project.Assignment, r *project.Resource) []project.TimephasedWork {
	totalMinutes := a.Work.Amount * 60
	if totalMinutes == 0 || cal == nil || a.Start.IsZero() || a.Finish.IsZero() {
		return nil
	}
	var perHour float64
	if r == nil || r.Type == project.WorkResource {
		perHour = 60 * a.Units / 100
	} else if hours := cal.WorkMinutesBetween(a.Start, a.Finish) / 60; hours != 0 {
		perHour = a.Units / (hours * 100)
	}
	totalMinutes -= a.OvertimeWork.Amount * 60
	return []project.TimephasedWork{{
		Start:   a.Start,
		Finish:  a.Finish,
		Total:   project.Duration{Amount: totalMinutes, Units: project.Minutes},
		PerHour: project.Duration{Amount: perHour, Units: project.Minutes},
	}}
}

// assignmentFlags are the assignment bit flags in FixedMeta outside the
// field map. Project 2013+ moved them relative to 2010. Sourced from MPXJ.
type assignmentFlags struct {
	flags                                 [20]metaFlag // Flag1..Flag20
	contoured, confirmed, responsePending metaFlag
}

func assignmentFlagLayout(applicationVersion int) assignmentFlags {
	var f assignmentFlags
	if applicationVersion <= appVersionProject2010 {
		masks := [20]int{0x04, 0x08, 0x10, 0x20, 0x40, 0x80, 0x100, 0x200, 0x400, 0x02,
			0x800, 0x1000, 0x2000, 0x4000, 0x8000, 0x10000, 0x20000, 0x40000, 0x80000, 0x100000}
		for i, m := range masks {
			f.flags[i] = metaFlag{28, m}
		}
		f.contoured, f.confirmed, f.responsePending = metaFlag{8, 0x10}, metaFlag{8, 0x80}, metaFlag{8, 0x100}
		return f
	}
	for i := 0; i < 9; i++ {
		f.flags[i] = metaFlag{20, 0x02 << i}
	}
	f.flags[9] = metaFlag{20, 0x01}
	for i := 10; i < 20; i++ {
		f.flags[i] = metaFlag{25, 0x08 << (i - 10)}
	}
	f.contoured, f.confirmed, f.responsePending = metaFlag{8, 0x40000}, metaFlag{8, 0x800000}, metaFlag{8, 0x1000000}
	return f
}

func addAssignmentFlags(fields map[string]interface{}, meta []byte, flags assignmentFlags, aliases customFieldAliases) map[string]interface{} {
	index := assignmentFieldIndex
	for i, flag := range flags.flags {
		if !flag.set(meta) {
			continue
		}
		if fields == nil {
			fields = make(map[string]interface{})
		}
		name := fmt.Sprintf("Flag%d", i+1)
		fields[aliases.name(assignmentFieldBase|index[name], name)] = true
	}
	return fields
}

// applyAssignmentFields fills the typed assignment fields that come from
// the decoded field map, the meta-data flags and the hyperlink block.
func applyAssignmentFields(a *project.Assignment, f map[string]interface{}, meta []byte, flags assignmentFlags, varData *Var2Data) {
	a.GUID = fieldString(f, "GUID")
	a.Cost = fieldFloat(f, "Cost")
	a.ActualCost = fieldFloat(f, "ActualCost")
	a.RemainingCost = fieldFloat(f, "RemainingCost")
	a.OvertimeCost = fieldFloat(f, "OvertimeCost")
	a.BCWS = fieldFloat(f, "BCWS")
	a.BCWP = fieldFloat(f, "BCWP")
	a.ACWP = fieldFloat(f, "ACWP")
	a.ActualWork = fieldDuration(f, "ActualWork")
	a.RemainingWork = fieldDuration(f, "RemainingWork")
	a.RegularWork = fieldDuration(f, "RegularWork")
	a.OvertimeWork = fieldDuration(f, "OvertimeWork")
	a.ActualOvertimeWork = fieldDuration(f, "ActualOvertimeWork")
	a.PercentWorkComplete = fieldFloat(f, "PercentWorkComplete")
	a.Stop = fieldTime(f, "Stop")
	a.Resume = fieldTime(f, "Resume")
	a.Created = fieldTime(f, "Created")
	a.Delay = fieldDuration(f, "AssignmentDelay")
	a.LevelingDelay = fieldDuration(f, "LevelingDelay")
	a.CostRateTable = fieldInt(f, "CostRateTable")
	a.Confirmed = flags.confirmed.set(meta)
	a.ResponsePending = flags.responsePending.set(meta)
	a.WorkContour = "Flat"
	if flags.contoured.set(meta) {
		a.WorkContour = "Contoured"
	}
	a.Hyperlink, a.HyperlinkAddress, a.HyperlinkSubAddress, a.HyperlinkScreenTip = readHyperlink(varData.ByteArray(a.UniqueID, assignmentHyperlinkVarType))
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
