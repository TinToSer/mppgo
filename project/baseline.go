// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

import "time"

// Baseline is a snapshot of a task, resource or assignment's schedule taken
// with MS Project's "Set Baseline" command. MS Project keeps one primary
// baseline plus up to ten numbered ones (Baseline1..Baseline10), each an
// independent snapshot rather than a rolling history.
//
// Not every field applies to every entity: a resource baseline has no
// Duration, and neither resource nor assignment baselines carry a separate
// FixedCost. Fields that do not apply, or that MS Project never recorded a
// value for, are left at their zero value.
type Baseline struct {
	Start     time.Time
	Finish    time.Time
	Duration  Duration
	Work      Duration
	Cost      float64
	FixedCost float64
}
