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

	DefaultCalendarName string

	// ApplicationName and ApplicationVersion identify the MS Project build
	// that wrote the file (e.g. "Microsoft.Project 16.0", 16). The version
	// determines several field layouts within the file.
	ApplicationName    string
	ApplicationVersion int
}
