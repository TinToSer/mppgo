// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

// ConstraintType is a scheduling constraint placed on a task, limiting
// where the scheduler may move it.
type ConstraintType int

// The numeric values match how MS Project stores them.
const (
	AsSoonAsPossible    ConstraintType = 0
	AsLateAsPossible    ConstraintType = 1
	MustStartOn         ConstraintType = 2
	MustFinishOn        ConstraintType = 3
	StartNoEarlierThan  ConstraintType = 4
	StartNoLaterThan    ConstraintType = 5
	FinishNoEarlierThan ConstraintType = 6
	FinishNoLaterThan   ConstraintType = 7
	StartOn             ConstraintType = 8
	FinishOn            ConstraintType = 9
)

// String returns the name MS Project displays for the constraint.
func (c ConstraintType) String() string {
	switch c {
	case AsSoonAsPossible:
		return "As Soon As Possible"
	case AsLateAsPossible:
		return "As Late As Possible"
	case MustStartOn:
		return "Must Start On"
	case MustFinishOn:
		return "Must Finish On"
	case StartNoEarlierThan:
		return "Start No Earlier Than"
	case StartNoLaterThan:
		return "Start No Later Than"
	case FinishNoEarlierThan:
		return "Finish No Earlier Than"
	case FinishNoLaterThan:
		return "Finish No Later Than"
	case StartOn:
		return "Start On"
	case FinishOn:
		return "Finish On"
	default:
		return "Unknown"
	}
}

// ResourceType distinguishes the three kinds of resource MS Project
// supports, which are costed and scheduled differently.
type ResourceType int

const (
	// WorkResource is a person or piece of equipment, assigned in units of
	// time and constrained by a calendar.
	WorkResource ResourceType = iota
	// MaterialResource is a consumable, assigned as a quantity rather than
	// over a span of time.
	MaterialResource
	// CostResource carries a cost with no work or duration of its own.
	CostResource
)

// String returns the name MS Project displays for the resource type.
func (t ResourceType) String() string {
	switch t {
	case WorkResource:
		return "Work"
	case MaterialResource:
		return "Material"
	case CostResource:
		return "Cost"
	default:
		return "Unknown"
	}
}
