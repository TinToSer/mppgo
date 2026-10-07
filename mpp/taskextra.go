package mpp

import (
	"time"
	"unicode/utf16"

	"github.com/tintoser/mppgo/project"
)

// Var-data keys for task data blocks the generic field decoder leaves as
// binary.
const (
	taskRecurringDataVarType = 203
	taskHyperlinkVarType     = 215
)

// durationEstimatedMask marks a duration-units value whose duration was
// entered as an estimate ("5d?").
const durationEstimatedMask = 0x20

// metaFlag is a boolean held as a bit in a FixedMeta or Fixed2Meta record,
// read as a 4-byte little-endian value at offset.
type metaFlag struct {
	offset, mask int
}

func (f metaFlag) set(meta []byte) bool { return getInt(meta, f.offset)&f.mask != 0 }

// taskFlags are the task bit flags outside the field map. Project 2013 and
// 2016+ share a layout; 2010 differs. Sourced from MPXJ (see NOTICE).
type taskFlags struct {
	marked, rollup, hideBar, effortDriven, levelAssignments, levelingCanSplit, ignoreResourceCalendar metaFlag
	manual                                                                                            metaFlag // in Fixed2Meta
}

func taskFlagLayout(applicationVersion int) taskFlags {
	if applicationVersion <= appVersionProject2010 {
		return taskFlags{
			marked: metaFlag{9, 0x40}, ignoreResourceCalendar: metaFlag{10, 0x02}, rollup: metaFlag{10, 0x08},
			hideBar: metaFlag{10, 0x80}, effortDriven: metaFlag{11, 0x10}, levelAssignments: metaFlag{13, 0x04},
			levelingCanSplit: metaFlag{13, 0x02}, manual: metaFlag{8, 0x08},
		}
	}
	return taskFlags{
		marked: metaFlag{12, 0x02}, rollup: metaFlag{12, 0x04}, hideBar: metaFlag{12, 0x80},
		effortDriven: metaFlag{13, 0x08}, levelAssignments: metaFlag{16, 0x04}, levelingCanSplit: metaFlag{16, 0x02},
		ignoreResourceCalendar: metaFlag{17, 0x20}, manual: metaFlag{8, 0x80},
	}
}

// applyTaskFields fills the typed task fields that come from the decoded
// field map, the meta-data flags and the task's binary var-data blocks.
// durationUnitsRaw is the stored duration-units value, whose estimate bit
// the decoded TimeUnit drops.
func applyTaskFields(ctx *readContext, t *project.Task, f map[string]interface{}, durationUnitsRaw int, meta, meta2 []byte, varData *Var2Data) {
	flags := taskFlagLayout(ctx.version)
	t.GUID = fieldString(f, "GUID")
	t.Estimated = durationUnitsRaw&durationEstimatedMask != 0
	t.Stop = fieldTime(f, "Stop")
	t.Resume = fieldTime(f, "Resume")
	t.LevelingDelay = fieldDuration(f, "LevelingDelay")
	t.Contact = fieldString(f, "Contact")
	t.SubprojectFile = fieldString(f, "SubprojectFile")
	t.SubprojectReadOnly = fieldBool(f, "SubprojectReadOnly")
	t.External = fieldBool(f, "ExternalTask")
	t.BCWS = fieldFloat(f, "BCWS")
	t.BCWP = fieldFloat(f, "BCWP")
	t.ACWP = fieldFloat(f, "ACWP")
	t.RegularWork = fieldDuration(f, "RegularWork")
	t.OvertimeWork = fieldDuration(f, "OvertimeWork")
	t.ActualOvertimeWork = fieldDuration(f, "ActualOvertimeWork")
	t.RemainingOvertimeWork = fieldDuration(f, "RemainingOvertimeWork")
	t.OvertimeCost = fieldFloat(f, "OvertimeCost")
	t.ActualOvertimeCost = fieldFloat(f, "ActualOvertimeCost")
	t.RemainingOvertimeCost = fieldFloat(f, "RemainingOvertimeCost")
	t.FixedCostAccrual = fieldString(f, "FixedCostAccrual")
	t.EarnedValueMethod = fieldString(f, "EarnedValueMethod")
	if t.EarnedValueMethod == "" {
		t.EarnedValueMethod = earnedValueMethod(0) // MS Project's default
	}
	t.PhysicalPercentComplete = fieldFloat(f, "PhysicalPercentComplete")

	t.Marked = flags.marked.set(meta)
	t.Rollup = flags.rollup.set(meta)
	t.HideBar = flags.hideBar.set(meta)
	t.EffortDriven = flags.effortDriven.set(meta)
	t.LevelAssignments = flags.levelAssignments.set(meta)
	t.LevelingCanSplit = flags.levelingCanSplit.set(meta)
	t.IgnoreResourceCalendar = flags.ignoreResourceCalendar.set(meta)
	t.Manual = meta2 != nil && flags.manual.set(meta2)

	// MS Project shows a manually scheduled task's manual duration; the
	// ordinary duration field can be stale for these.
	if t.Manual {
		if d, ok := f["ManualDuration"].(project.Duration); ok {
			t.ManualDuration = d
			t.Duration = d
		}
	}

	t.Hyperlink, t.HyperlinkAddress, t.HyperlinkSubAddress, t.HyperlinkScreenTip = readHyperlink(varData.ByteArray(t.UniqueID, taskHyperlinkVarType))
	t.Recurrence = readRecurringTask(ctx, varData.ByteArray(t.UniqueID, taskRecurringDataVarType))

	if d, ok := f["TotalSlack"].(project.Duration); ok {
		t.TotalSlack = d
	} else {
		t.TotalSlack = totalSlack(t)
	}
	t.Critical = critical(ctx, t, f)
}

// totalSlack follows MS Project: finish slack once the task has started,
// otherwise the smaller of start and finish slack, in the duration's units.
func totalSlack(t *project.Task) project.Duration {
	if !t.ActualStart.IsZero() {
		return t.FinishSlack
	}
	if t.StartSlack.Amount < t.FinishSlack.Amount {
		return t.StartSlack
	}
	return t.FinishSlack
}

// critical follows MS Project: an unfinished, not 100% complete task whose
// total slack is within the project's critical slack limit, and which is
// automatically scheduled (or manual with real dates rather than text).
func critical(ctx *readContext, t *project.Task, f map[string]interface{}) bool {
	if !t.ActualFinish.IsZero() || t.PercentComplete == 100 {
		return false
	}
	if t.Manual && (fieldString(f, "StartText") != "" || fieldString(f, "FinishText") != "" || fieldString(f, "DurationText") != "") {
		return false
	}
	limit := ctx.criticalSlackLimit // days
	slack := t.TotalSlack.Amount
	if limit != 0 && slack != 0 {
		slack = ctx.scale.minutes(t.TotalSlack) / ctx.scale.minutesPerDay
	}
	return slack <= limit
}

// readHyperlink decodes a hyperlink block: four NUL-terminated UTF-16
// strings (text, address, sub-address, screen tip), each preceded by 12
// bytes, after a 12-byte header.
func readHyperlink(data []byte) (text, address, subAddress, screenTip string) {
	if data == nil {
		return
	}
	offset := 12
	next := func() string {
		offset += 12
		s := getUnicodeString(data, offset)
		offset += (len(utf16.Encode([]rune(s))) + 1) * 2
		return s
	}
	return next(), next(), next(), next()
}

// readRecurringTask decodes a recurring task's pattern block (MPXJ's
// RecurringTaskReader layout).
func readRecurringTask(ctx *readContext, data []byte) *project.RecurringTask {
	if len(data) < 72 {
		return nil
	}
	r := &project.RecurringTask{
		Occurrences:     getShort(data, 18),
		UseEndDate:      getShort(data, 24) == 1,
		WorkingDaysOnly: getShort(data, 26) == 1,
	}
	r.Start, _ = getDate(data, 6)
	r.Finish, _ = getDate(data, 10)
	r.Duration = ctx.scale.duration(getInt(data, 12), durationTimeUnits(getShort(data, 16), ctx.defaultUnits))
	for day := 0; day < 7; day++ {
		r.WeeklyDays[day] = getShort(data, 28+day*2) == 1
	}
	var frequency, dayOfWeek, dayNumber, monthNumber, date int
	switch getShort(data, 20) {
	case 1:
		r.Type, frequency = "Daily", 46
	case 4:
		r.Type, frequency = "Weekly", 48
	case 8:
		r.Type = "Monthly"
		r.Relative = getShort(data, 42) == 1
		if r.Relative {
			frequency, dayNumber, dayOfWeek = 58, 50, 52
		} else {
			frequency, dayNumber = 54, 56
		}
	case 16:
		r.Type = "Yearly"
		r.Relative = getShort(data, 44) != 1
		if r.Relative {
			dayNumber, dayOfWeek, monthNumber = 60, 62, 64
		} else {
			date = 70
		}
	default:
		return nil
	}
	if frequency != 0 {
		r.Frequency = getShort(data, frequency)
	}
	if dayOfWeek != 0 {
		r.DayOfWeek = time.Weekday(getShort(data, dayOfWeek) % 7)
	}
	if dayNumber != 0 {
		r.DayNumber = getShort(data, dayNumber)
	}
	if monthNumber != 0 {
		r.MonthNumber = getShort(data, monthNumber)
	}
	if date != 0 {
		r.YearlyDate, _ = getDate(data, date)
	}
	return r
}
