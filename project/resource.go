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

	// Type decides how the resource is costed and scheduled. It defaults to
	// WorkResource, which is also the common case.
	Type ResourceType

	Group        string
	Code         string
	EmailAddress string

	// MaxUnits is the resource's available capacity, as a percentage: 100
	// means one full-time equivalent, matching how MS Project and MPXJ
	// report it. Not meaningful for material resources.
	MaxUnits float64

	// StandardRate is the cost per hour.
	StandardRate float64

	// Work is the total effort assigned to this resource across all tasks,
	// Cost its total cost.
	Work Duration
	Cost float64

	CalendarUniqueID int

	CustomFields map[string]interface{}
}
