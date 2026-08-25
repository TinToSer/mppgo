// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

import "testing"

func TestRelationTypeString(t *testing.T) {
	cases := map[RelationType]string{
		FinishFinish: "FF",
		FinishStart:  "FS",
		StartFinish:  "SF",
		StartStart:   "SS",
	}
	for typ, want := range cases {
		if got := typ.String(); got != want {
			t.Errorf("RelationType(%d).String() = %q, want %q", typ, got, want)
		}
	}
	if got := RelationType(99).String(); got != "?" {
		t.Errorf("unknown RelationType = %q, want %q", got, "?")
	}
}

func TestAddRelationLinksBothTasks(t *testing.T) {
	f := New()
	pred := &Task{UniqueID: 1, Name: "submit"}
	succ := &Task{UniqueID: 2, Name: "approve"}
	f.AddTask(pred)
	f.AddTask(succ)

	r := &Relation{PredecessorUniqueID: 1, SuccessorUniqueID: 2, Type: FinishStart}
	f.AddRelation(r)

	if len(f.Relations) != 1 {
		t.Fatalf("len(Relations) = %d, want 1", len(f.Relations))
	}
	if len(pred.Successors) != 1 || pred.Successors[0] != r {
		t.Error("the predecessor should list the relation as a successor link")
	}
	if len(succ.Predecessors) != 1 || succ.Predecessors[0] != r {
		t.Error("the successor should list the relation as a predecessor link")
	}
	if len(pred.Predecessors) != 0 || len(succ.Successors) != 0 {
		t.Error("the relation was linked in the wrong direction")
	}
}

// A relation naming a task that was never read must still be recorded, but
// must not create a link to a task that does not exist.
func TestAddRelationToleratesUnknownTasks(t *testing.T) {
	f := New()
	known := &Task{UniqueID: 1}
	f.AddTask(known)

	f.AddRelation(&Relation{PredecessorUniqueID: 1, SuccessorUniqueID: 999})

	if len(f.Relations) != 1 {
		t.Errorf("len(Relations) = %d, want 1", len(f.Relations))
	}
	if len(known.Successors) != 1 {
		t.Errorf("len(known.Successors) = %d, want 1", len(known.Successors))
	}
}

func TestConstraintTypeString(t *testing.T) {
	if got := AsSoonAsPossible.String(); got != "As Soon As Possible" {
		t.Errorf("AsSoonAsPossible = %q", got)
	}
	if got := MustFinishOn.String(); got != "Must Finish On" {
		t.Errorf("MustFinishOn = %q", got)
	}
	if got := ConstraintType(42).String(); got != "Unknown" {
		t.Errorf("unknown ConstraintType = %q, want %q", got, "Unknown")
	}
}

func TestResourceTypeString(t *testing.T) {
	cases := map[ResourceType]string{
		WorkResource:     "Work",
		MaterialResource: "Material",
		CostResource:     "Cost",
	}
	for typ, want := range cases {
		if got := typ.String(); got != want {
			t.Errorf("ResourceType(%d).String() = %q, want %q", typ, got, want)
		}
	}
}

func TestTimeUnitString(t *testing.T) {
	cases := map[TimeUnit]string{
		Minutes: "m", Hours: "h", Days: "d", Weeks: "w",
		ElapsedDays: "ed", Percent: "%",
	}
	for unit, want := range cases {
		if got := unit.String(); got != want {
			t.Errorf("TimeUnit(%d).String() = %q, want %q", unit, got, want)
		}
	}
}
