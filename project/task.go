// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

import "time"

// Task is a project task/activity. Field set will grow as the MPP/MSPDI
// readers gain coverage; this is intentionally not yet field-complete.
type Task struct {
	UniqueID     int
	ID           int
	Name         string
	WBS          string
	OutlineLevel int

	Start    time.Time
	Finish   time.Time
	Duration Duration

	PercentComplete float64
	Milestone       bool
	Summary         bool

	ParentUniqueID int // 0 if top-level

	CustomFields map[string]interface{}
}
