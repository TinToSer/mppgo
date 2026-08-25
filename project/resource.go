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

	CalendarUniqueID int

	CustomFields map[string]interface{}
}
