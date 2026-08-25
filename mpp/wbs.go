// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"sort"
	"strconv"

	"github.com/tintoser/mppgo/project"
)

// synthesizeWBS fills in Task.WBS for any task that has none, using MS
// Project's own auto-numbering algorithm rather than leaving the field
// blank. Most real MPP files never populate a custom WBS and rely on this
// auto-numbering entirely (MS Project only persists a WBS value when the
// user actually customizes it), so leaving the field blank would make it
// blank on the common case, not just an edge case.
//
// This mirrors MPXJ's Task.generateWBS/TaskContainer.updateStructure
// exactly: tasks are walked in ID order (the row/display order, not
// UniqueID) reconstructing parent/child structure from OutlineLevel via a
// stack, and each task's WBS becomes its parent's WBS plus its 1-based
// position among that parent's children — a root task's WBS is just its
// position among the other roots. A task that already carries a real,
// file-stored WBS is left untouched, though it still counts toward its
// siblings' positions.
func synthesizeWBS(tasks []*project.Task) {
	if len(tasks) == 0 {
		return
	}

	sorted := append([]*project.Task(nil), tasks...)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].ID < sorted[j].ID })

	// node tracks the reconstructed parent link and running child count
	// alongside each task, without touching the task's own ParentUniqueID
	// (which this reader already populates directly from the file and
	// which is not what MPXJ uses to generate WBS).
	type node struct {
		task       *project.Task
		parent     *node
		childCount int
	}

	var lastNode *node
	lastLevel := -1
	rootChildCount := 0

	for _, t := range sorted {
		level := t.OutlineLevel
		var parent *node

		switch {
		case lastNode == nil:
			// First task in ID order: no parent.
		case level == lastLevel:
			parent = lastNode.parent
		case level > lastLevel:
			parent = lastNode
		default:
			// Walk back up the stack to the ancestor this task actually
			// descends from. Uses local scratch state: the outer
			// lastNode/lastLevel always reset to this task's own values
			// right after, regardless of what this walk did to them.
			walk, walkLevel := lastNode, lastLevel
			for level <= walkLevel {
				parent = walk.parent
				if parent == nil {
					break
				}
				walkLevel = parent.task.OutlineLevel
				walk = parent
			}
		}

		n := &node{task: t, parent: parent}
		lastNode = n
		lastLevel = level

		var childIndex int
		if parent == nil {
			rootChildCount++
			childIndex = rootChildCount
		} else {
			parent.childCount++
			childIndex = parent.childCount
		}

		if t.WBS != "" {
			continue // a real, file-stored value; still counted above
		}
		switch {
		case parent == nil:
			t.WBS = strconv.Itoa(childIndex)
		case parent.task.WBS == "0":
			t.WBS = strconv.Itoa(childIndex)
		default:
			t.WBS = parent.task.WBS + "." + strconv.Itoa(childIndex)
		}
	}
}
