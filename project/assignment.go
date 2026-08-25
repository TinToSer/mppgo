// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

// Assignment links a Resource to a Task. Field set will grow as the
// MPP/MSPDI readers gain coverage.
type Assignment struct {
	UniqueID         int
	TaskUniqueID     int
	ResourceUniqueID int

	// Units is the proportion of the resource assigned, as a percentage:
	// 100 means 100%, matching how MS Project and MPXJ report it.
	Units float64

	Work Duration
}
