// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

import "time"

// Properties holds project-level (header) properties. Field set will grow
// as the MPP/MSPDI readers gain coverage.
type Properties struct {
	// Name is the project title. Not yet populated from MPP: the title
	// lives in the standard OLE "\x05SummaryInformation" property set,
	// which this reader does not parse yet.
	Name string

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
}
