// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

// Resource is a project resource (work, material, or cost). Field set will
// grow as the MPP/MSPDI readers gain coverage.
type Resource struct {
	UniqueID int
	ID       int
	Name     string
	Initials string

	// Type decides how the resource is costed and scheduled. It defaults to
	// WorkResource, which is also the common case.
	Type ResourceType

	Group        string
	Code         string
	EmailAddress string

	// MaxUnits is the resource's available capacity, as a percentage: 100
	// means one full-time equivalent, matching how MS Project and MPXJ
	// report it. Not meaningful for material resources.
	MaxUnits float64

	// StandardRate and OvertimeRate are the cost per hour for regular and
	// overtime work; CostPerUse is a flat charge applied once per
	// assignment regardless of how much work is done. These are the
	// resource's own rate — see CostRateTables below for the date-ranged
	// tables MS Project derives from (or falls back to) these.
	StandardRate float64
	OvertimeRate float64
	CostPerUse   float64

	// Work is the total effort assigned to this resource across all tasks,
	// Cost its total cost.
	Work Duration
	Cost float64

	CalendarUniqueID int

	// Notes is the resource's Notes field with RTF formatting stripped to
	// plain text; RTFNotes keeps the original RTF. Both are empty if the
	// resource has no note.
	Notes    string
	RTFNotes string

	// Baseline is the resource's primary baseline snapshot, and Baselines
	// holds the numbered ones (Baseline1..Baseline10), keyed by that number.
	// A resource baseline carries Work/Cost always; Start/Finish are only
	// ever recorded for the numbered baselines, never the primary one — a
	// quirk of the format, not of this reader.
	Baseline  *Baseline
	Baselines map[int]*Baseline

	// CostRateTables holds the resource's five cost rate tables (MS
	// Project's Resource Information > Costs tab, tables A-E — index 0 is
	// table A, index 4 is table E), each a set of date ranges with their
	// own standard/overtime rate and cost-per-use. Table A, index 0, is
	// always populated (falling back to StandardRate/CostPerUse as a single
	// open-ended entry if the file doesn't store one explicitly); B-E are
	// nil until the user actually defines them.
	CostRateTables [5][]CostRateTableEntry

	// Availability is the resource's availability table (Resource
	// Information > General tab, "Resource Availability" grid): date ranges
	// over which MaxUnits above does not apply, replaced by each entry's
	// own MaxUnits. Nil if the resource has no availability table of its
	// own, in which case MaxUnits applies for its entire lifetime.
	Availability []AvailabilityEntry

	CustomFields map[string]interface{}
}
