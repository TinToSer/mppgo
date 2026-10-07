// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

import "time"

// Properties holds project-level (header) properties. Field set will grow
// as the MPP/MSPDI readers gain coverage.
type Properties struct {
	// Name is the project title, and the fields below it are the rest of
	// the document metadata a user can set in MS Project's project
	// information dialog. Any of them may be empty.
	//
	// A file created from a template often carries the template's title
	// rather than one describing this particular project, so treat Name as
	// what the file says rather than as authoritative.
	Name     string
	Subject  string
	Author   string
	Manager  string
	Company  string
	Category string
	Keywords string
	Comments string

	// FilePath is the path the file was last saved to, which for
	// SharePoint-hosted plans is a URL.
	FilePath string

	StartDate  time.Time
	FinishDate time.Time

	// StatusDate is the single project-level date MS Project's own status
	// reporting places the current schedule snapshot against — it is not
	// derivable from any task field. Zero if the project has none set.
	StatusDate time.Time

	DefaultCalendarName string

	// MinutesPerDay, MinutesPerWeek and DaysPerMonth are the conversion
	// factors MS Project uses to turn a stored duration into the day/week/
	// month figure it displays. They are project settings, not derived from
	// the calendar, and a file that sets "8 hours = 1 day" reports a
	// different number of days for the same stored duration than one that
	// sets 12. Zero if the file did not record them.
	MinutesPerDay  int
	MinutesPerWeek int
	DaysPerMonth   int

	// ApplicationName and ApplicationVersion identify the MS Project build
	// that wrote the file (e.g. "Microsoft.Project 16.0", 16). The version
	// determines several field layouts within the file.
	ApplicationName    string
	ApplicationVersion int

	GUID string

	// Document properties MS Project writes to the OLE summary property
	// sets rather than its own Props stream.
	Template        string
	LastAuthor      string
	Revision        int
	CreationDate    time.Time
	LastSaved       time.Time
	LastPrinted     time.Time
	EditingTime     int // minutes, as recorded by the file
	ContentType     string
	ContentStatus   string
	Language        string
	DocumentVersion string
	// CustomProperties are the File > Properties > Custom entries: string,
	// int, float64, bool or time.Time values.
	CustomProperties map[string]interface{}

	ScheduleFromStart bool
	// DefaultStartTime/DefaultEndTime are offsets from midnight.
	DefaultStartTime time.Duration
	DefaultEndTime   time.Duration

	DefaultDurationUnits TimeUnit
	DefaultWorkUnits     TimeUnit
	DefaultTaskType      TaskType
	DefaultStandardRate  float64 // per hour
	DefaultOvertimeRate  float64 // per hour

	CriticalSlackLimit    Duration
	MultipleCriticalPaths bool
	HonorConstraints      bool
	SplitInProgressTasks  bool
	TaskUpdatesResource   bool
	EditableActualCosts   bool
	NewTasksAreManual     bool

	CurrencySymbol         string
	CurrencyCode           string
	CurrencyDigits         int
	CurrencySymbolPosition string // "Before", "After", "Before with space" or "After with space"

	WeekStartDay         time.Weekday
	FiscalYearStartMonth int
	FiscalYearStart      bool

	HyperlinkBase        string
	BaselineCalendarName string
	ResourcePoolFile     string

	// BaselineDates records when each baseline (0 = the primary baseline,
	// 1-10) was last saved.
	BaselineDates map[int]time.Time
}
