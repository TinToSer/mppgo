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

	// Start and Finish are the dates MS Project shows for the task, summary
	// tasks included.
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

	// Notes is the task's Notes field with RTF formatting stripped down to
	// plain text — what MS Project itself shows in the Notes box. RTFNotes
	// keeps the original RTF for a caller that needs the formatting (or the
	// embedded objects RTFEmbeddedObject-style tools would extract from it).
	// Both are empty if the task has no note.
	Notes    string
	RTFNotes string

	// Baseline is the task's primary baseline snapshot, and Baselines holds
	// the numbered ones MS Project supports (Baseline1..Baseline10), keyed
	// by that number. Both are nil/absent until the corresponding "Set
	// Baseline" has been run at least once.
	Baseline  *Baseline
	Baselines map[int]*Baseline

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

	GUID string

	// Manual reports a manually scheduled task. Its Duration is the manual
	// duration MS Project shows; ManualDuration keeps it separately.
	Manual         bool
	ManualDuration Duration

	// TotalSlack is the smaller of StartSlack and FinishSlack (FinishSlack
	// once the task has started), and Critical is true when it is within
	// the project's critical slack limit — MS Project's own definitions.
	TotalSlack Duration
	Critical   bool
	Estimated  bool

	// Stop and Resume bound a split in progress: work is complete up to
	// Stop and the remaining work resumes at Resume.
	Stop   time.Time
	Resume time.Time

	LevelingDelay Duration

	Contact             string
	Hyperlink           string
	HyperlinkAddress    string
	HyperlinkSubAddress string
	HyperlinkScreenTip  string

	// Earned value: budgeted cost of work scheduled/performed and actual
	// cost of work performed.
	BCWS float64
	BCWP float64
	ACWP float64

	RegularWork           Duration
	OvertimeWork          Duration
	ActualOvertimeWork    Duration
	RemainingOvertimeWork Duration
	OvertimeCost          float64
	ActualOvertimeCost    float64
	RemainingOvertimeCost float64

	FixedCostAccrual        string // "Start", "End" or "Prorated"
	EarnedValueMethod       string
	PhysicalPercentComplete float64

	EffortDriven           bool
	Rollup                 bool
	HideBar                bool
	Marked                 bool
	IgnoreResourceCalendar bool
	LevelAssignments       bool
	LevelingCanSplit       bool

	// SubprojectFile is the inserted project a task stands for, and
	// External marks a task that belongs to another project file (an
	// inserted project's task or a cross-project link).
	SubprojectFile     string
	SubprojectReadOnly bool
	External           bool

	// Recurrence is set for a recurring task's summary row.
	Recurrence *RecurringTask

	// Fields holds every field the file stores for the task, decoded and
	// keyed by MPXJ's field name in CamelCase ("TotalSlack", "Text1",
	// "Baseline3Start", ...): time.Time, Duration, float64, int, bool,
	// string, TimeUnit, ConstraintType or TaskType. It includes the fields
	// above, and fields with no typed counterpart. An absent key means the
	// file stores no value. Populated by the MPP reader.
	Fields map[string]interface{}
}

// RecurringTask describes the pattern of a recurring task.
type RecurringTask struct {
	Start, Finish   time.Time
	Duration        Duration
	Occurrences     int
	Type            string // "Daily", "Weekly", "Monthly" or "Yearly"
	UseEndDate      bool
	WorkingDaysOnly bool
	WeeklyDays      [7]bool // indexed by time.Weekday
	Relative        bool
	Frequency       int
	DayNumber       int
	DayOfWeek       time.Weekday
	MonthNumber     int
	YearlyDate      time.Time
}
