// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

import "time"

// CostRateTableEntry is one date range within a resource's cost rate table.
// Work performed in [Start, End) costs StandardRate per StandardRateUnits of
// work, overtime costs OvertimeRate per OvertimeRateUnits, and CostPerUse is
// a flat charge added regardless of how much work is done. Start and End
// are zero for "no lower/upper bound" — the common case for a table with
// only one entry, which then applies for the resource's entire lifetime.
type CostRateTableEntry struct {
	Start time.Time
	End   time.Time

	StandardRate      float64
	StandardRateUnits TimeUnit
	OvertimeRate      float64
	OvertimeRateUnits TimeUnit
	CostPerUse        float64
}

// AvailabilityEntry is one date range within a resource's availability
// table: MaxUnits is only meaningful during [Start, End). Start and End are
// zero for "no lower/upper bound".
type AvailabilityEntry struct {
	Start    time.Time
	End      time.Time
	MaxUnits float64
}
