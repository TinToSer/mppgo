// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"math"
	"time"

	"github.com/tintoser/mppgo/project"
)

// Var-data type keys for an assignment's timephased data. Only the planned
// (scheduled/remaining) work, and the baseline work/cost for the primary
// baseline plus Baseline1-10, are read — see readTimephasedPlannedWork's
// doc comment for what is deliberately left out and why.
const (
	assignmentTimephasedPlannedWorkVarType  = 49
	assignmentTimephasedBaselineWorkVarType = 52
	assignmentTimephasedBaselineCostVarType = 53
)

// assignmentTimephasedBaselineVarKeys are the {work, cost} var-data type
// keys for Baseline1..Baseline10's timephased data. Sourced from MPXJ's
// MPP14 field map (see NOTICE).
var assignmentTimephasedBaselineVarKeys = [11][2]int{
	{}, // index 0 unused: the primary baseline uses the constants above
	{291, 292}, {300, 301}, {309, 310}, {318, 319}, {327, 328},
	{336, 337}, {345, 346}, {354, 355}, {363, 364}, {372, 373},
}

// readTimephasedPlannedWork decodes an assignment's planned (scheduled, or
// once work has started, remaining) work, spread across time — the day by
// day breakdown MS Project's Task Usage/Resource Usage views show.
//
// This only covers the "planned" block. It deliberately does not attempt
// the "complete" (actual) block MPXJ also reads: actual work distinguishes
// work done inside normal working hours from work done outside it via a
// second, "irregular ranges" data block that must be spliced into the
// first through a fairly intricate interval-splitting algorithm, all of it
// working entirely in raw elapsed minutes with no fixed record layout to
// anchor bounds-checking the way every other reader in this package does.
// The risk of a subtly wrong implementation going unnoticed was judged too
// high for the value it adds on top of planned/baseline work, which cover
// the common reporting cases (resource loading, EVM against baseline).
//
// data is the assignment's own var-data blob (a nil blob, or one too short
// to contain a header, yields no items — a task with no duration, or with
// no remaining work, has neither). assignmentStart/assignmentFinish are the
// assignment's own Start/Finish fields, and calendar is the calendar to
// resolve elapsed-time spans against (see project.File.AssignmentCalendar).
func readTimephasedPlannedWork(calendar *project.Calendar, assignmentStart, assignmentFinish time.Time, data []byte) []project.TimephasedWork {
	if calendar == nil || len(data) < 24 {
		return nil
	}

	blockCount := getShort(data, 0)
	var items []project.TimephasedWork

	if blockCount == 0 {
		// A block count of zero means the whole assignment is one span,
		// described by the header (summary) block alone.
		totalWorkSeconds := round(getDouble(data, 16) * 60 / 1000)
		if totalWorkSeconds == 0 {
			return nil
		}
		elapsedSeconds := round(calendar.WorkMinutesBetween(assignmentStart, assignmentFinish) * 60)
		perHour := workPerHourMinutes(totalWorkSeconds, elapsedSeconds)
		items = append(items, project.TimephasedWork{
			Start:   assignmentStart,
			Finish:  assignmentFinish,
			Total:   project.Duration{Amount: totalWorkSeconds / 60, Units: project.Minutes},
			PerHour: project.Duration{Amount: perHour, Units: project.Minutes},
		})
		return items
	}

	// The regular case: a summary block (skipped) followed by blockCount
	// 28-byte blocks, each carrying the cumulative work and cumulative
	// elapsed time as of the end of that block.
	const blockSize = 28
	offset := 16 + blockSize
	var previousWorkSeconds, previousElapsedSeconds float64
	start := assignmentStart

	for count := 0; count < blockCount && offset+blockSize <= len(data); count++ {
		cumulativeWorkSeconds := round(getDouble(data, offset) * 60 / 1000)
		workSecondsThisPeriod := cumulativeWorkSeconds - previousWorkSeconds
		cumulativeElapsedSeconds := round(float64(getInt(data, offset+24)) * 60 / 80)
		elapsedSecondsThisPeriod := cumulativeElapsedSeconds - previousElapsedSeconds

		end := calendar.AdvanceByWork(start, elapsedSecondsThisPeriod/60)
		perHour := workPerHourMinutes(workSecondsThisPeriod, elapsedSecondsThisPeriod)

		if workSecondsThisPeriod >= 1 || !start.Equal(end) {
			items = append(items, project.TimephasedWork{
				Start:   start,
				Finish:  end,
				Total:   project.Duration{Amount: workSecondsThisPeriod / 60, Units: project.Minutes},
				PerHour: project.Duration{Amount: perHour, Units: project.Minutes},
			})
		}

		start = calendar.NextWorkStart(end)
		previousWorkSeconds = cumulativeWorkSeconds
		previousElapsedSeconds = cumulativeElapsedSeconds
		offset += blockSize
	}

	return removeZeroLengthWork(items)
}

// readTimephasedBaselineWork decodes one baseline snapshot's timephased
// work — data captured when "Set Baseline" was last run, not MS Project's
// live/current schedule, so it is read against calendar rather than
// recomputed from it.
func readTimephasedBaselineWork(calendar *project.Calendar, data []byte) []project.TimephasedWork {
	if calendar == nil || len(data) < 48 {
		return nil
	}

	blockCount := getShort(data, 0)
	start := getTimestampFromTenths(data, 44)
	const blockSize = 20
	offset := 48
	var cumulativeWork float64
	var items []project.TimephasedWork

	// The first and last blocks are summaries, not data; blockCount counts
	// every block including those two.
	for i := 0; i < blockCount-2 && offset+blockSize <= len(data); i++ {
		currentCumulativeWork := getDouble(data, offset) / 1000
		workThisPeriod := currentCumulativeWork - cumulativeWork
		end := getTimestampFromTenths(data, offset+16)

		var perHour float64
		if workThisPeriod != 0 {
			calendarMinutes := calendar.WorkMinutesBetween(start, end)
			if calendarMinutes == 0 {
				calendarMinutes = end.Sub(start).Minutes()
			}
			if calendarMinutes != 0 {
				perHour = (workThisPeriod * 60) / calendarMinutes
			}
		}

		items = append(items, project.TimephasedWork{
			Start:   start,
			Finish:  end,
			Total:   project.Duration{Amount: workThisPeriod, Units: project.Minutes},
			PerHour: project.Duration{Amount: perHour, Units: project.Minutes},
		})

		start = end
		cumulativeWork = currentCumulativeWork
		offset += blockSize
	}

	if cumulativeWork == 0 {
		return nil
	}
	return removeZeroLengthWork(items)
}

// readTimephasedBaselineCost decodes one baseline snapshot's timephased
// cost. See readTimephasedBaselineWork.
func readTimephasedBaselineCost(calendar *project.Calendar, data []byte) []project.TimephasedCost {
	if calendar == nil || len(data) < 36 {
		return nil
	}

	blockCount := getShort(data, 0)
	start := getTimestampFromTenths(data, 32)
	const blockSize = 20
	offset := 36
	var cumulativeCost float64
	var items []project.TimephasedCost

	for i := 0; i < blockCount-2 && offset+blockSize <= len(data); i++ {
		end := getTimestampFromTenths(data, offset+16)
		cumulativeCostThisEnd := getDouble(data, offset+8)
		costThisPeriod := cumulativeCostThisEnd - cumulativeCost

		var perHour float64
		if costThisPeriod != 0 {
			calendarMinutes := calendar.WorkMinutesBetween(start, end)
			if calendarMinutes == 0 {
				calendarMinutes = end.Sub(start).Minutes()
			}
			if calendarMinutes != 0 {
				perHour = (costThisPeriod * 60) / calendarMinutes
			}
		}

		items = append(items, project.TimephasedCost{
			Start:   start,
			Finish:  end,
			Total:   costThisPeriod / 100,
			PerHour: perHour / 100,
		})

		cumulativeCost = cumulativeCostThisEnd
		start = end
		offset += blockSize
	}

	if cumulativeCost == 0 {
		return nil
	}
	return items
}

// workPerHourMinutes computes minutes-of-work-per-elapsed-hour, MS
// Project's own "per hour" figure for a timephased span, guarding the
// elapsed side against zero (a valid, if unhelpful, input rather than
// something to divide by).
func workPerHourMinutes(workSeconds, elapsedSeconds float64) float64 {
	if elapsedSeconds == 0 {
		return 0
	}
	return (workSeconds * 60) / elapsedSeconds
}

// removeZeroLengthWork drops spans whose start equals their finish —
// bookkeeping artifacts in the source data, not real (if empty) periods of
// work.
func removeZeroLengthWork(items []project.TimephasedWork) []project.TimephasedWork {
	out := items[:0]
	for _, it := range items {
		if !it.Start.Equal(it.Finish) {
			out = append(out, it)
		}
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

// round matches Java's Math.round(double): floor(v + 0.5), which for a
// negative tie (-2.5) rounds toward positive infinity (-2), not away from
// zero. Timephased amounts are cumulative and effectively always
// non-negative in practice, but this keeps the arithmetic bit-for-bit
// consistent with the reference implementation regardless.
func round(v float64) float64 {
	return math.Floor(v + 0.5)
}
