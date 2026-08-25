// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

// RelationType is the kind of dependency between two tasks: which end of
// the predecessor the successor is tied to, and which end of the successor
// is tied to it.
type RelationType int

// The numeric values match how MS Project stores them, so they can be read
// straight out of a file without a translation table.
const (
	FinishFinish RelationType = 0
	FinishStart  RelationType = 1
	StartFinish  RelationType = 2
	StartStart   RelationType = 3
)

// String returns the two-letter abbreviation MS Project displays ("FS",
// "SS", ...).
func (t RelationType) String() string {
	switch t {
	case FinishFinish:
		return "FF"
	case FinishStart:
		return "FS"
	case StartFinish:
		return "SF"
	case StartStart:
		return "SS"
	default:
		return "?"
	}
}

// Relation is a dependency between two tasks: the successor cannot be
// scheduled purely on its own, but relative to the predecessor according to
// Type, offset by Lag.
//
// A negative Lag is a lead (the successor may start before the predecessor
// reaches the linked point), which MS Project shows as a negative value.
type Relation struct {
	UniqueID int

	PredecessorUniqueID int
	SuccessorUniqueID   int

	Type RelationType
	Lag  Duration
}
