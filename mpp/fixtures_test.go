package mpp_test

import (
	"math"
	"path/filepath"
	"testing"
	"time"

	"github.com/tintoser/mppgo/mpp"
	"github.com/tintoser/mppgo/project"
)

// readEachFixture runs check against every MPP file in ../testdata. Like
// sampleFile, these are real-world files that are never committed, so the
// test skips when none are present.
func readEachFixture(t *testing.T, check func(t *testing.T, pf *project.File)) {
	t.Helper()
	paths, _ := filepath.Glob("../testdata/*.mpp")
	if len(paths) == 0 {
		t.Skip("no MPP fixtures in ../testdata")
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			pf, err := mpp.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			check(t, pf)
		})
	}
}

// Every task, summary tasks included, carries the Start/Finish MS Project
// shows; a summary task spans its active children exactly; and every
// assignment falls within its task. Reading Start/Finish from the secondary
// Fixed2Data copy left them blank for summary tasks, and for every task in
// some files.
func TestReadFixtureTaskDates(t *testing.T) {
	readEachFixture(t, func(t *testing.T, pf *project.File) {
		for _, task := range pf.Tasks {
			if task.Start.IsZero() || task.Finish.IsZero() {
				t.Errorf("task %d %q: Start/Finish missing", task.UniqueID, task.Name)
			} else if task.Finish.Before(task.Start) {
				t.Errorf("task %d %q: Finish %v before Start %v", task.UniqueID, task.Name, task.Finish, task.Start)
			}
		}
		for _, parent := range pf.Tasks {
			if !parent.Summary {
				continue
			}
			var first, last time.Time
			for _, child := range pf.Tasks {
				if child.ParentUniqueID != parent.UniqueID || child.Inactive {
					continue
				}
				if first.IsZero() || child.Start.Before(first) {
					first = child.Start
				}
				if child.Finish.After(last) {
					last = child.Finish
				}
			}
			if !first.IsZero() && (!parent.Start.Equal(first) || !parent.Finish.Equal(last)) {
				t.Errorf("summary %d %q spans %v..%v, children span %v..%v", parent.UniqueID, parent.Name, parent.Start, parent.Finish, first, last)
			}
		}
		for _, a := range pf.Assignments {
			task := pf.TaskByID(a.TaskUniqueID)
			if task != nil && (a.Start.Before(task.Start) || a.Finish.After(task.Finish)) {
				t.Errorf("assignment %d %v..%v lies outside task %d %v..%v", a.UniqueID, a.Start, a.Finish, task.UniqueID, task.Start, task.Finish)
			}
		}
	})
}

// A yearly holiday is stored as its first and last occurrence. Reading that
// pair as one continuous exception made every day of the intervening
// decades non-working.
func TestReadFixtureRecurringHolidays(t *testing.T) {
	readEachFixture(t, func(t *testing.T, pf *project.File) {
		cal := pf.CalendarByName("Standard_BaWü_Feiertage_5_Tage_Woche")
		if cal == nil {
			t.Skip("fixture has no recurring-holiday calendar")
		}
		for day, working := range map[string]bool{
			"2026-01-01": false, // Neujahr, a yearly exception
			"2026-03-10": true,  // an ordinary Tuesday
			"2030-05-01": false, // Tag der Arbeit
			"2030-05-02": true,
			"2026-12-28": false, // Weihnachtsferien 2026, a genuine multi-day range
		} {
			d, _ := time.Parse("2006-01-02", day)
			if got := cal.WorkingOn(d); got != working {
				t.Errorf("WorkingOn(%s) = %v, want %v", day, got, working)
			}
		}
	})
}

// Actual-work timephased data sums to the assignment's actual work, and
// planned data to what remains (or to all of its work when nothing is
// done yet).
func TestReadFixtureTimephasedTotals(t *testing.T) {
	readEachFixture(t, func(t *testing.T, pf *project.File) {
		sum := func(spans []project.TimephasedWork) float64 {
			var hours float64
			for _, s := range spans {
				hours += s.Total.Amount / 60
			}
			return hours
		}
		for _, a := range pf.Assignments {
			if len(a.TimephasedActualWork) != 0 {
				if got := sum(a.TimephasedActualWork); math.Abs(got-a.ActualWork.Amount) > 0.05 {
					t.Errorf("assignment %d: actual timephased work %.2fh, ActualWork %.2fh", a.UniqueID, got, a.ActualWork.Amount)
				}
			}
			if len(a.TimephasedWork) != 0 {
				want := a.Work.Amount
				if len(a.TimephasedActualWork) != 0 {
					want = a.RemainingWork.Amount
				}
				if got := sum(a.TimephasedWork); math.Abs(got-want) > 0.05 {
					t.Errorf("assignment %d: planned timephased work %.2fh, want %.2fh", a.UniqueID, got, want)
				}
			}
		}
	})
}

// The decoded field map and the typed fields describe the same task, and
// the project summary task rolls up the top-level tasks.
func TestReadFixtureFieldsAndSummary(t *testing.T) {
	readEachFixture(t, func(t *testing.T, pf *project.File) {
		for _, task := range pf.Tasks {
			if task.Fields["Name"] != task.Name || task.Fields["UniqueID"] != task.UniqueID {
				t.Fatalf("task %d: Fields disagree with typed fields", task.UniqueID)
			}
			if d, _ := task.Fields["Start"].(time.Time); !d.Equal(task.Start) {
				t.Fatalf("task %d: Fields[Start] %v, Start %v", task.UniqueID, d, task.Start)
			}
		}
		summary := pf.ProjectSummaryTask
		if summary == nil {
			t.Fatal("no project summary task")
		}
		var work float64
		for _, task := range pf.Tasks {
			if task.ParentUniqueID == 0 {
				work += task.Work.Amount
			}
		}
		if math.Abs(summary.Work.Amount-work) > 0.05 {
			t.Errorf("summary work %.2fh, top-level tasks %.2fh", summary.Work.Amount, work)
		}
	})
}
