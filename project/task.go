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

	// Start and Finish are the dates MS Project shows for the task, and are
	// stored only for leaf tasks that are active. They are zero for summary
	// tasks (MS Project rolls those up from the children rather than
	// storing them) and for inactive tasks. EarlyStart/EarlyFinish below
	// are populated for every task, so prefer those when a date is needed
	// for a summary row.
	Start  time.Time
	Finish time.Time

	Duration Duration

	// EarlyStart/EarlyFinish and LateStart/LateFinish are the critical-path
	// bounds: the earliest and latest the task can run without moving the
	// project finish. Start/Finish above are the dates MS Project actually
	// shows for the task.
	EarlyStart  time.Time
	EarlyFinish time.Time
	LateStart   time.Time
	LateFinish  time.Time

	// ActualStart/ActualFinish are zero until progress is recorded.
	ActualStart  time.Time
	ActualFinish time.Time

	// Deadline is a target date that does not constrain scheduling but
	// which MS Project flags when missed. Created records when the task was
	// added. Both are zero if unset.
	Deadline time.Time
	Created  time.Time

	// Slack is how far the task can move before it affects the project
	// finish. FreeSlack is the amount that does not affect any successor.
	FreeSlack   Duration
	StartSlack  Duration
	FinishSlack Duration

	ActualDuration    Duration
	RemainingDuration Duration

	// Work is the effort assigned to the task, Cost its total cost.
	Work          Duration
	ActualWork    Duration
	RemainingWork Duration
	Cost          float64
	FixedCost     float64
	ActualCost    float64
	RemainingCost float64

	// PercentWorkComplete tracks completion by effort, where
	// PercentComplete above tracks it by duration; the two differ whenever
	// effort is not spread evenly across the task.
	PercentWorkComplete float64

	// Type decides which of duration, work and units MS Project holds fixed
	// when rescheduling.
	Type TaskType

	// ConstraintType defaults to AsSoonAsPossible, the unconstrained case.
	// ConstraintDate is zero for constraint types that do not use a date.
	ConstraintType ConstraintType
	ConstraintDate time.Time

	// Priority is 0-1000, where 500 is MS Project's normal default.
	Priority int

	PercentComplete float64
	Milestone       bool
	Summary         bool

	// Inactive is true for a task the user has explicitly deactivated in
	// MS Project (available since Project 2010). MS Project blanks such a
	// task's Start/Finish while leaving LateStart/LateFinish as whatever
	// they were before deactivation, so a zero Start/Finish alongside
	// Inactive == true means "deliberately inactive", not missing data.
	Inactive bool

	ParentUniqueID int // 0 if top-level

	// Predecessors are the dependencies this task waits on; Successors are
	// the dependencies that wait on it. Both point at the same Relation
	// values held in File.Relations, so a relation is never duplicated.
	//
	// Both ends of every relation listed here resolve to a task that was
	// read, so File.TaskByID on either end returns non-nil.
	Predecessors []*Relation
	Successors   []*Relation

	// CalendarUniqueID is the calendar set directly on this task, or 0 if
	// the task has none of its own and should resolve through the project
	// default. See File.TaskCalendar.
	CalendarUniqueID int

	CustomFields map[string]interface{}
}
