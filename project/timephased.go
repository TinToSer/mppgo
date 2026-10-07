// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

import "time"

type TimephasedData struct {
	Type   int
	UID    int
	Start  time.Time
	Finish time.Time
	Unit   int
	Value  string
}

// TimephasedWork is one span of an assignment's work spread across time —
// MS Project's "Task Usage"/"Resource Usage" view, one column per day (or
// week/month, depending on the view's timescale). PerHour is the rate MS
// Project displays for the span (e.g. "4h/d"); Total is that span's actual
// total, which is not always exactly PerHour times the span's calendar
// length — MS Project itself rounds and adjusts both independently.
type TimephasedWork struct {
	Start   time.Time
	Finish  time.Time
	Total   Duration
	PerHour Duration
}

// TimephasedCost is TimephasedWork's cost equivalent.
type TimephasedCost struct {
	Start   time.Time
	Finish  time.Time
	Total   float64
	PerHour float64
}
