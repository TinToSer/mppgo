// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

import "time"

// Assignment links a Resource to a Task. Field set will grow as the
// MPP/MSPDI readers gain coverage.
type Assignment struct {
	UniqueID         int
	TaskUniqueID     int
	ResourceUniqueID int

	// Units is the proportion of the resource assigned, as a percentage:
	// 100 means 100%, matching how MS Project and MPXJ report it.
	Units float64

	Work Duration

	// Start and Finish are the dates MS Project schedules this specific
	// assignment for — not necessarily identical to the parent task's own
	// Start/Finish when the task has multiple assignments contouring
	// differently.
	Start  time.Time
	Finish time.Time

	// Notes is the assignment's Notes field with RTF formatting stripped to
	// plain text; RTFNotes keeps the original RTF. Both are empty if the
	// assignment has no note.
	Notes    string
	RTFNotes string

	// Baseline is the assignment's primary baseline snapshot, and Baselines
	// holds the numbered ones (Baseline1..Baseline10), keyed by that number.
	// An assignment baseline never carries Duration or FixedCost.
	Baseline  *Baseline
	Baselines map[int]*Baseline

	// TimephasedWork is the assignment's scheduled/remaining work, spread
	// day by day (MS Project's Task Usage/Resource Usage view). Nil if the
	// file has none — a task with no duration, or no remaining work left.
	//
	// TimephasedBaselineWork and TimephasedBaselineCost are the same idea
	// captured at baseline time: key 0 is the primary baseline, 1-10 the
	// numbered ones, mirroring Baseline/Baselines above. A key present in
	// Baselines is not guaranteed to also be present here, or vice versa —
	// each comes from its own data in the file.
	TimephasedWork         []TimephasedWork
	TimephasedActualWork   []TimephasedWork
	TimephasedBaselineWork map[int][]TimephasedWork
	TimephasedBaselineCost map[int][]TimephasedCost
	RawTimephased          []TimephasedData
}
