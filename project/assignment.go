// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

// Assignment links a Resource to a Task. Field set will grow as the
// MPP/MSPDI readers gain coverage.
type Assignment struct {
	UniqueID         int
	TaskUniqueID     int
	ResourceUniqueID int

	Units float64 // e.g. 1.0 = 100%
	Work  Duration
}
