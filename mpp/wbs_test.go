// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"testing"

	"github.com/tintoser/mppgo/project"
)

func wbsTask(id, outlineLevel int) *project.Task {
	return &project.Task{ID: id, OutlineLevel: outlineLevel}
}

func TestSynthesizeWBSSimpleHierarchy(t *testing.T) {
	// Outline nesting is positional (display order + level), like MS
	// Project itself: d directly follows b, so it nests under b; c then
	// follows d and pops back up to being a's second child.
	a := wbsTask(1, 1) // root
	b := wbsTask(2, 2) // child of a
	d := wbsTask(3, 3) // child of b
	c := wbsTask(4, 2) // second child of a
	tasks := []*project.Task{a, b, d, c}

	synthesizeWBS(tasks)

	cases := map[string]string{"a": a.WBS, "b": b.WBS, "c": c.WBS, "d": d.WBS}
	want := map[string]string{"a": "1", "b": "1.1", "c": "1.2", "d": "1.1.1"}
	for name, got := range cases {
		if got != want[name] {
			t.Errorf("%s.WBS = %q, want %q", name, got, want[name])
		}
	}
}

func TestSynthesizeWBSMultipleRoots(t *testing.T) {
	a := wbsTask(1, 1)
	b := wbsTask(2, 2)
	e := wbsTask(3, 1) // a second root
	tasks := []*project.Task{a, b, e}

	synthesizeWBS(tasks)

	if a.WBS != "1" || b.WBS != "1.1" || e.WBS != "2" {
		t.Errorf("got a=%q b=%q e=%q, want a=1 b=1.1 e=2", a.WBS, b.WBS, e.WBS)
	}
}

// TestSynthesizeWBSPopsBackUpTheStack exercises the case a flat FieldItem
// offset table can't get wrong but a naive "always attach to the previous
// task" implementation would: dropping from level 3 back to level 2 must
// reattach to the level-1 ancestor, not stay under the level-3 task.
func TestSynthesizeWBSPopsBackUpTheStack(t *testing.T) {
	a := wbsTask(1, 1) // root
	b := wbsTask(2, 2) // child of a
	c := wbsTask(3, 3) // child of b
	d := wbsTask(4, 2) // back up: second child of a, sibling of b
	tasks := []*project.Task{a, b, c, d}

	synthesizeWBS(tasks)

	if got, want := d.WBS, "1.2"; got != want {
		t.Errorf("d.WBS = %q, want %q (should pop back to be a's second child)", got, want)
	}
	if got, want := c.WBS, "1.1.1"; got != want {
		t.Errorf("c.WBS = %q, want %q", got, want)
	}
}

func TestSynthesizeWBSPreservesRealValues(t *testing.T) {
	a := wbsTask(1, 1)
	a.WBS = "CUSTOM"
	b := wbsTask(2, 2) // child of a, no real WBS of its own

	tasks := []*project.Task{a, b}
	synthesizeWBS(tasks)

	if a.WBS != "CUSTOM" {
		t.Errorf("a.WBS = %q, want the preserved real value %q", a.WBS, "CUSTOM")
	}
	// b still gets a generated value built on the parent's real WBS.
	if got, want := b.WBS, "CUSTOM.1"; got != want {
		t.Errorf("b.WBS = %q, want %q", got, want)
	}
}

func TestSynthesizeWBSSortsByIDNotSliceOrder(t *testing.T) {
	// Deliberately supplied out of ID order; synthesizeWBS must sort by
	// ID (MS Project's row/display order) before walking the hierarchy,
	// matching MPXJ's Collections.sort(this) in TaskContainer.updateStructure.
	b := wbsTask(2, 2)
	a := wbsTask(1, 1)
	tasks := []*project.Task{b, a}

	synthesizeWBS(tasks)

	if a.WBS != "1" || b.WBS != "1.1" {
		t.Errorf("got a=%q b=%q, want a=1 b=1.1 regardless of input slice order", a.WBS, b.WBS)
	}
}

func TestSynthesizeWBSEmpty(t *testing.T) {
	// Must not panic on an empty task list.
	synthesizeWBS(nil)
}
