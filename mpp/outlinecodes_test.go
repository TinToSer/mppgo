// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import "testing"

func TestResolveOutlineCodePathBuildsRootToLeaf(t *testing.T) {
	values := map[int]outlineCodeValue{
		1: {text: "USA", parentID: 0},
		2: {text: "California", parentID: 1},
		3: {text: "Los Angeles", parentID: 2},
	}
	if got, want := resolveOutlineCodePath(values, 3), "USA | California | Los Angeles"; got != want {
		t.Errorf("resolveOutlineCodePath = %q, want %q", got, want)
	}
}

func TestResolveOutlineCodePathSingleLevel(t *testing.T) {
	values := map[int]outlineCodeValue{1: {text: "Standalone", parentID: 0}}
	if got, want := resolveOutlineCodePath(values, 1), "Standalone"; got != want {
		t.Errorf("resolveOutlineCodePath = %q, want %q", got, want)
	}
}

func TestResolveOutlineCodePathUnknownID(t *testing.T) {
	values := map[int]outlineCodeValue{1: {text: "USA"}}
	if got := resolveOutlineCodePath(values, 99); got != "" {
		t.Errorf("resolveOutlineCodePath(unknown id) = %q, want \"\"", got)
	}
	if got := resolveOutlineCodePath(values, 0); got != "" {
		t.Errorf("resolveOutlineCodePath(0) = %q, want \"\"", got)
	}
}

// A cyclic parent chain (corrupt data, or a bug elsewhere) must terminate
// rather than loop forever — matching this reader's stance elsewhere (see
// project.TestCyclicParentChainTerminates for the calendar equivalent).
func TestResolveOutlineCodePathCyclicParentChainTerminates(t *testing.T) {
	values := map[int]outlineCodeValue{
		1: {text: "A", parentID: 2},
		2: {text: "B", parentID: 1},
	}
	got := resolveOutlineCodePath(values, 1)
	if got != "A | B" && got != "B | A" {
		t.Errorf("resolveOutlineCodePath(cycle) = %q, want a two-element path built from the cycle before it repeats", got)
	}
}
