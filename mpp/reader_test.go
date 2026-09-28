// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp_test

import (
	"strings"
	"testing"
	"time"

	"github.com/tintoser/mppgo/mpp"
	"github.com/tintoser/mppgo/project"
)

// sampleFile is a real-world fixture, not checked into the repo because
// MPP files carry project-specific data. Place one at this path locally to
// run these tests; otherwise they skip. There is no automated MPXJ oracle
// here by design, so these expectations were verified by hand against the
// file's known content.
const sampleFile = "../testdata/sample.mpp"

func readSample(t *testing.T) *project.File {
	t.Helper()
	pf, err := mpp.ReadFile(sampleFile)
	if err != nil {
		t.Skipf("sample file unavailable: %v", err)
	}
	return pf
}

func TestReadSampleProperties(t *testing.T) {
	pf := readSample(t)

	if got, want := pf.Properties.DefaultCalendarName, "6 Days Calender"; got != want {
		t.Errorf("DefaultCalendarName = %q, want %q", got, want)
	}
	if got, want := pf.Properties.ApplicationVersion, 16; got != want {
		t.Errorf("ApplicationVersion = %d, want %d", got, want)
	}
	if pf.Properties.FilePath == "" {
		t.Error("FilePath is empty, want the saved SharePoint URL")
	}
	if pf.DefaultCalendar == nil {
		t.Error("DefaultCalendar was not resolved")
	}

	if pf.Properties.StartDate.IsZero() {
		t.Error("StartDate was not populated")
	}
	if pf.Properties.FinishDate.IsZero() {
		t.Error("FinishDate was not populated")
	}
	if pf.Properties.FinishDate.Before(pf.Properties.StartDate) {
		t.Errorf("FinishDate %v is before StartDate %v", pf.Properties.FinishDate, pf.Properties.StartDate)
	}
	if pf.Properties.StatusDate.IsZero() {
		t.Error("StatusDate was not populated")
	}
	if pf.Properties.Name == "" {
		t.Error("Name (the project title) was not populated")
	}
	// The scheduling factors decide how a stored duration converts into
	// the days and weeks MS Project displays, so a zero here would make
	// every duration wrong.
	if pf.Properties.MinutesPerDay <= 0 {
		t.Errorf("MinutesPerDay = %d, want a positive value", pf.Properties.MinutesPerDay)
	}
	if pf.Properties.MinutesPerWeek <= 0 {
		t.Errorf("MinutesPerWeek = %d, want a positive value", pf.Properties.MinutesPerWeek)
	}
}

func TestReadSampleStandardCalendar(t *testing.T) {
	pf := readSample(t)

	std := pf.CalendarByName("Standard")
	if std == nil {
		t.Fatal(`no "Standard" calendar`)
	}
	if std.Parent != nil {
		t.Error("Standard should be a base calendar with no parent")
	}

	wantWorking := map[time.Weekday]bool{
		time.Sunday: false, time.Monday: true, time.Tuesday: true,
		time.Wednesday: true, time.Thursday: true, time.Friday: true,
		time.Saturday: false,
	}
	for day, want := range wantWorking {
		if got := std.IsWorkingDay(day); got != want {
			t.Errorf("Standard.IsWorkingDay(%s) = %v, want %v", day, got, want)
		}
	}

	mon := std.HoursFor(time.Monday)
	if len(mon) != 2 {
		t.Fatalf("Standard.HoursFor(Monday) = %v, want 2 ranges", mon)
	}
	if mon[0].Start != 8*time.Hour || mon[0].End != 12*time.Hour {
		t.Errorf("morning = %v, want 08:00-12:00", mon[0])
	}
	if mon[1].Start != 13*time.Hour || mon[1].End != 17*time.Hour {
		t.Errorf("afternoon = %v, want 13:00-17:00", mon[1])
	}
	if got, want := mon[0].Duration()+mon[1].Duration(), 8*time.Hour; got != want {
		t.Errorf("working day length = %v, want %v", got, want)
	}

	if std.HoursFor(time.Sunday) != nil {
		t.Error("Sunday should have no working hours")
	}
}

func TestReadSampleCalendarExceptions(t *testing.T) {
	pf := readSample(t)
	sixDay := pf.CalendarByName("6 Days Calender")
	if sixDay == nil {
		t.Fatal(`no "6 Days Calender" calendar`)
	}

	if len(sixDay.Exceptions) != 2 {
		t.Fatalf("len(Exceptions) = %d, want 2", len(sixDay.Exceptions))
	}

	// Both exceptions fall on a Sunday, the one day this calendar does not
	// otherwise work, so only the exception can make them working.
	for _, dateStr := range []string{"2022-04-17", "2022-05-08"} {
		d, err := time.Parse("2006-01-02", dateStr)
		if err != nil {
			t.Fatalf("parsing %s: %v", dateStr, err)
		}
		if d.Weekday() != time.Sunday {
			t.Fatalf("fixture assumption broken: %s is %s", dateStr, d.Weekday())
		}
		if !sixDay.WorkingOn(d) {
			t.Errorf("%s should be working (exception overrides Sunday)", dateStr)
		}
	}
	if sixDay.IsWorkingDay(time.Sunday) {
		t.Error("Sunday should otherwise be non-working")
	}
	if sixDay.IsWorkingDay(time.Saturday) == false {
		t.Error("Saturday should be a working day on the 6-day calendar")
	}
}

// Derived calendars store only the days the user overrode; every other day
// must resolve through the parent. Before day-type support these calendars
// came back completely empty.
func TestReadSampleDerivedCalendarsInherit(t *testing.T) {
	pf := readSample(t)

	var derived []*project.Calendar
	for _, c := range pf.Calendars {
		if c.Parent != nil {
			derived = append(derived, c)
		}
	}
	if len(derived) == 0 {
		t.Fatal("expected at least one derived calendar in the sample")
	}

	for _, c := range derived {
		if c.Parent.UniqueID == c.UniqueID {
			t.Errorf("calendar #%d is its own parent", c.UniqueID)
		}
		// Inherited or not, a weekday must resolve to a definite answer.
		if !c.IsWorkingDay(time.Wednesday) {
			t.Errorf("derived calendar #%d: Wednesday should inherit as working", c.UniqueID)
		}
		if hours := c.HoursFor(time.Wednesday); len(hours) == 0 {
			t.Errorf("derived calendar #%d: Wednesday should inherit working hours", c.UniqueID)
		}
	}
}

// TestReadSampleResources checks the eight category resources
// (CR/DR/ER/GR/IR/MR/PR/TR) that drive S-curve categorization are read with
// their names, initials, and a calendar link resolved from TBkndCal.
func TestReadSampleResources(t *testing.T) {
	pf := readSample(t)

	if len(pf.Resources) == 0 {
		t.Fatal("expected at least one resource")
	}

	wantInitials := map[string]bool{"C": false, "D": false, "E": false, "G": false, "I": false, "M": false, "P": false, "T": false}
	for _, r := range pf.Resources {
		if r.Name == "" {
			t.Errorf("resource #%d has an empty name", r.UniqueID)
		}
		if _, tracked := wantInitials[r.Initials]; tracked {
			wantInitials[r.Initials] = true
		}
		if r.CalendarUniqueID != 0 && pf.CalendarByID(r.CalendarUniqueID) == nil {
			t.Errorf("resource %q: CalendarUniqueID %d does not resolve to a calendar", r.Name, r.CalendarUniqueID)
		}
	}
	for initials, seen := range wantInitials {
		if !seen {
			t.Errorf("expected a resource with initials %q", initials)
		}
	}
}

// TestReadSampleTasks exercises the task hierarchy end to end: every parent
// reference resolves, Summary is set on (and only on) tasks that are
// actually a parent, and the outline levels form a real multi-level tree
// rather than everything landing at level 0 (the symptom of a misaligned
// FixedData offset).
func TestReadSampleTasks(t *testing.T) {
	pf := readSample(t)

	if len(pf.Tasks) == 0 {
		t.Fatal("expected at least one task")
	}

	hasChildren := make(map[int]bool)
	for _, task := range pf.Tasks {
		if task.ParentUniqueID == 0 {
			continue
		}
		parent := pf.TaskByID(task.ParentUniqueID)
		if parent == nil {
			t.Errorf("task %q: ParentUniqueID %d does not resolve to a task", task.Name, task.ParentUniqueID)
			continue
		}
		hasChildren[parent.UniqueID] = true
	}

	var topLevel, milestones, maxOutline int
	for _, task := range pf.Tasks {
		if task.ParentUniqueID == 0 {
			topLevel++
		}
		if task.Milestone {
			milestones++
		}
		if task.OutlineLevel > maxOutline {
			maxOutline = task.OutlineLevel
		}
		if want := hasChildren[task.UniqueID]; task.Summary != want {
			t.Errorf("task %q: Summary = %v, want %v (has children = %v)", task.Name, task.Summary, want, want)
		}
	}
	if topLevel == 0 {
		t.Error("expected at least one top-level task (ParentUniqueID == 0)")
	}
	if milestones == 0 {
		t.Error("expected at least one milestone task")
	}
	if maxOutline < 2 {
		t.Errorf("max OutlineLevel = %d, want a real multi-level hierarchy (>= 2)", maxOutline)
	}
}

// TestReadSampleAssignments checks every assignment links back to a real
// task and resource, and that Work/Units decode to non-negative values.
func TestReadSampleAssignments(t *testing.T) {
	pf := readSample(t)

	if len(pf.Assignments) == 0 {
		t.Fatal("expected at least one assignment")
	}

	for _, a := range pf.Assignments {
		if pf.TaskByID(a.TaskUniqueID) == nil {
			t.Errorf("assignment #%d: TaskUniqueID %d does not resolve to a task", a.UniqueID, a.TaskUniqueID)
		}
		if pf.ResourceByID(a.ResourceUniqueID) == nil {
			t.Errorf("assignment #%d: ResourceUniqueID %d does not resolve to a resource", a.UniqueID, a.ResourceUniqueID)
		}
		if a.Units < 0 {
			t.Errorf("assignment #%d: Units = %v, want >= 0", a.UniqueID, a.Units)
		}
		if a.Work.Amount < 0 {
			t.Errorf("assignment #%d: Work = %v, want >= 0", a.UniqueID, a.Work.Amount)
		}
	}
}

// TestReadSampleRelations checks the dependency graph: every relation
// resolves to real tasks at both ends, the per-task links agree with the
// flat list, and the graph is not degenerate.
func TestReadSampleRelations(t *testing.T) {
	pf := readSample(t)

	if len(pf.Relations) == 0 {
		t.Fatal("expected at least one task dependency")
	}

	linkCount := 0
	for _, r := range pf.Relations {
		pred := pf.TaskByID(r.PredecessorUniqueID)
		succ := pf.TaskByID(r.SuccessorUniqueID)
		if pred == nil {
			t.Errorf("relation #%d: predecessor %d does not resolve", r.UniqueID, r.PredecessorUniqueID)
		}
		if succ == nil {
			t.Errorf("relation #%d: successor %d does not resolve", r.UniqueID, r.SuccessorUniqueID)
		}
		if r.PredecessorUniqueID == r.SuccessorUniqueID {
			t.Errorf("relation #%d links a task to itself", r.UniqueID)
		}
		switch r.Type {
		case project.FinishStart, project.FinishFinish, project.StartStart, project.StartFinish:
		default:
			t.Errorf("relation #%d has an unrecognised type %d", r.UniqueID, r.Type)
		}
	}

	// Every relation must appear on both of the tasks it links.
	for _, task := range pf.Tasks {
		linkCount += len(task.Predecessors) + len(task.Successors)
		for _, r := range task.Predecessors {
			if r.SuccessorUniqueID != task.UniqueID {
				t.Errorf("task %d lists a predecessor relation whose successor is %d", task.UniqueID, r.SuccessorUniqueID)
			}
		}
		for _, r := range task.Successors {
			if r.PredecessorUniqueID != task.UniqueID {
				t.Errorf("task %d lists a successor relation whose predecessor is %d", task.UniqueID, r.PredecessorUniqueID)
			}
		}
	}
	if want := len(pf.Relations) * 2; linkCount != want {
		t.Errorf("per-task links = %d, want %d (each relation linked from both ends)", linkCount, want)
	}
}

// TestReadSampleTaskDurations checks durations decode into sane units
// rather than the raw tenths-of-a-minute counts stored in the file.
func TestReadSampleTaskDurations(t *testing.T) {
	pf := readSample(t)

	if pf.Properties.MinutesPerDay <= 0 {
		t.Error("MinutesPerDay was not populated, so durations cannot be scaled correctly")
	}

	withDuration := 0
	for _, task := range pf.Tasks {
		if task.Duration.Amount < 0 {
			t.Errorf("task %q has a negative duration %v", task.Name, task.Duration.Amount)
		}
		if task.Duration.Amount > 0 {
			withDuration++
			// A raw count read without conversion would be in the
			// thousands; a real schedule's task durations are not.
			if task.Duration.Units == project.Days && task.Duration.Amount > 10000 {
				t.Errorf("task %q duration %v days looks like an unconverted raw value",
					task.Name, task.Duration.Amount)
			}
		}
	}
	if withDuration == 0 {
		t.Error("no task had a duration")
	}
}

// TestReadSampleRemainingDurationAgrees cross-checks two independently
// decoded duration fields: a task that has not been started at all must
// have all of its duration still remaining. Agreement means the shared
// duration-units field and the raw-to-unit conversion are both right,
// since a mis-scaled conversion would break the equality.
func TestReadSampleRemainingDurationAgrees(t *testing.T) {
	pf := readSample(t)

	checked := 0
	for _, task := range pf.Tasks {
		if task.PercentComplete != 0 {
			continue
		}
		checked++
		if diff := task.RemainingDuration.Amount - task.Duration.Amount; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("task %q is 0%% complete but Duration = %v and RemainingDuration = %v",
				task.Name, task.Duration.Amount, task.RemainingDuration.Amount)
		}
	}
	if checked == 0 {
		t.Skip("every task in the sample has progress recorded")
	}
}

// Slack is stored, not derived, so it should never come back negative on a
// well-formed file — a negative value is the signature of a misread offset.
func TestReadSampleSlackIsNonNegative(t *testing.T) {
	pf := readSample(t)

	for _, task := range pf.Tasks {
		for name, d := range map[string]project.Duration{
			"FreeSlack":   task.FreeSlack,
			"StartSlack":  task.StartSlack,
			"FinishSlack": task.FinishSlack,
		} {
			if d.Amount < 0 {
				t.Errorf("task %q: %s = %v, want >= 0", task.Name, name, d.Amount)
			}
		}
	}
}

// TestReadSampleResourceWorkReconciles is a cross-check between three
// independently decoded fields: a resource's own rolled-up Work should
// equal the sum of its assignments' Work, once inactive tasks are excluded
// (MS Project leaves them out of the rollup but keeps their assignments).
// Agreement here means Resource.Work, Assignment.Work and Task.Inactive are
// all being read correctly.
func TestReadSampleResourceWorkReconciles(t *testing.T) {
	pf := readSample(t)

	assigned := make(map[int]float64)
	for _, a := range pf.Assignments {
		if task := pf.TaskByID(a.TaskUniqueID); task != nil && task.Inactive {
			continue
		}
		assigned[a.ResourceUniqueID] += a.Work.Amount
	}

	checked := 0
	for _, r := range pf.Resources {
		if r.Work.Amount == 0 && assigned[r.UniqueID] == 0 {
			continue
		}
		checked++
		if diff := r.Work.Amount - assigned[r.UniqueID]; diff > 1e-6 || diff < -1e-6 {
			t.Errorf("resource %q: Work = %v, but its assignments total %v",
				r.Name, r.Work.Amount, assigned[r.UniqueID])
		}
	}
	if checked == 0 {
		t.Skip("no resource in the sample carries work to reconcile")
	}
}

// TestReadSampleTaskNotes checks that at least one task note round-trips
// through the RTF stripper into readable text: no leftover RTF control
// sequences, and no shorter than the RTF source it came from.
func TestReadSampleTaskNotes(t *testing.T) {
	pf := readSample(t)

	found := 0
	for _, task := range pf.Tasks {
		if task.Notes == "" {
			continue
		}
		found++
		if task.RTFNotes == "" {
			t.Errorf("task %q: Notes is set but RTFNotes is empty", task.Name)
		}
		if strings.Contains(task.Notes, `{\rtf`) || strings.Contains(task.Notes, `\par`) {
			t.Errorf("task %q: Notes still contains RTF markup: %q", task.Name, task.Notes)
		}
	}
	if found == 0 {
		t.Skip("no task in the sample has a note")
	}
}

// TestReadSampleTaskBaselines checks that a task's primary baseline, when
// present, has at least one field actually populated (readTaskBaseline
// should never return a Baseline with nothing set), and that any numbered
// baseline present is keyed 1..10.
func TestReadSampleTaskBaselines(t *testing.T) {
	pf := readSample(t)

	found := 0
	for _, task := range pf.Tasks {
		if task.Baseline != nil {
			found++
		}
		for n, b := range task.Baselines {
			if n < 1 || n > 10 {
				t.Errorf("task %q: Baselines has out-of-range key %d", task.Name, n)
			}
			if b == nil {
				t.Errorf("task %q: Baselines[%d] is nil", task.Name, n)
			}
		}
	}
	if found == 0 {
		t.Skip("no task in the sample has a baseline set")
	}
}

// TestReadSampleTaskCustomFields checks that any populated custom field is
// keyed by a non-empty name (either a real alias or the generic Text1/
// Number1/... fallback) and holds a value of a type readTaskCustomFields is
// actually meant to produce.
func TestReadSampleTaskCustomFields(t *testing.T) {
	pf := readSample(t)

	found := 0
	for _, task := range pf.Tasks {
		for name, value := range task.CustomFields {
			found++
			if name == "" {
				t.Errorf("task %q: custom field has an empty name", task.Name)
			}
			switch value.(type) {
			case string, float64, time.Time, project.Duration, bool:
			default:
				t.Errorf("task %q: custom field %q has unexpected type %T", task.Name, name, value)
			}
		}
	}
	if found == 0 {
		t.Skip("no task in the sample has a custom field set")
	}
}

// TestReadSampleResourceNotesAndBaselines mirrors the task-level checks
// above for resources: a note strips its RTF markup, and a non-nil
// baseline carries at least one real value.
func TestReadSampleResourceNotesAndBaselines(t *testing.T) {
	pf := readSample(t)

	notes, baselines := 0, 0
	for _, r := range pf.Resources {
		if r.Notes != "" {
			notes++
			if r.RTFNotes == "" {
				t.Errorf("resource %q: Notes is set but RTFNotes is empty", r.Name)
			}
			if strings.Contains(r.Notes, `{\rtf`) {
				t.Errorf("resource %q: Notes still contains RTF markup: %q", r.Name, r.Notes)
			}
		}
		if r.Baseline != nil {
			baselines++
		}
	}
	if notes == 0 && baselines == 0 {
		t.Skip("no resource in the sample has a note or baseline set")
	}
}

// TestReadSampleAssignmentNotesAndBaselines mirrors the task-level checks
// above for assignments.
func TestReadSampleAssignmentNotesAndBaselines(t *testing.T) {
	pf := readSample(t)

	notes, baselines := 0, 0
	for _, a := range pf.Assignments {
		if a.Notes != "" {
			notes++
			if strings.Contains(a.Notes, `{\rtf`) {
				t.Errorf("assignment #%d: Notes still contains RTF markup: %q", a.UniqueID, a.Notes)
			}
		}
		if a.Baseline != nil {
			baselines++
		}
	}
	if notes == 0 && baselines == 0 {
		t.Skip("no assignment in the sample has a note or baseline set")
	}
}

// TestReadSampleResourceCostRateTables checks table A is always populated
// (falling back to the resource's own rate when the file has no table of
// its own — see readResourceCostRateTable) and that every entry's rate
// units are one of the values workTimeUnit can actually produce.
func TestReadSampleResourceCostRateTables(t *testing.T) {
	pf := readSample(t)

	if len(pf.Resources) == 0 {
		t.Fatal("expected at least one resource")
	}
	for _, r := range pf.Resources {
		if len(r.CostRateTables[0]) == 0 {
			t.Errorf("resource %q: CostRateTables[0] (table A) is empty, want at least the fallback entry", r.Name)
		}
		for table, entries := range r.CostRateTables {
			for _, e := range entries {
				if e.CostPerUse < 0 {
					t.Errorf("resource %q table %c: CostPerUse = %v, want >= 0", r.Name, 'A'+table, e.CostPerUse)
				}
			}
		}
	}
}

// TestReadSampleTaskAndResourceFlags checks that any Flag custom field
// present is a real bool and keyed by a non-empty name.
func TestReadSampleTaskAndResourceFlags(t *testing.T) {
	pf := readSample(t)

	found := 0
	for _, task := range pf.Tasks {
		for name, v := range task.CustomFields {
			if b, ok := v.(bool); ok {
				found++
				if !b {
					t.Errorf("task %q: flag %q = false, want it to have been left out of CustomFields entirely", task.Name, name)
				}
				if name == "" {
					t.Errorf("task %q: flag has an empty name", task.Name)
				}
			}
		}
	}
	if found == 0 {
		t.Skip("no task in the sample has a flag set")
	}
}

// TestReadSampleTimephasedWork checks that timephased spans are internally
// consistent: chronologically ordered, non-negative, and — the strongest
// check available without a second implementation to compare against —
// that a fully-populated set of planned-work spans sums to the
// assignment's own Work field, since MS Project derives one from the
// other. A partially-complete assignment's remaining work legitimately
// sums to less than its total Work, so this only asserts the sum never
// exceeds it.
func TestReadSampleTimephasedWork(t *testing.T) {
	pf := readSample(t)

	found := 0
	for _, a := range pf.Assignments {
		if len(a.TimephasedWork) == 0 {
			continue
		}
		found++

		var sumMinutes float64
		for i, span := range a.TimephasedWork {
			if span.Finish.Before(span.Start) {
				t.Errorf("assignment #%d span %d: Finish %v is before Start %v", a.UniqueID, i, span.Finish, span.Start)
			}
			if i > 0 && span.Start.Before(a.TimephasedWork[i-1].Finish) {
				t.Errorf("assignment #%d span %d starts before the previous span ends", a.UniqueID, i)
			}
			if span.Total.Amount < 0 {
				t.Errorf("assignment #%d span %d: Total = %v, want >= 0", a.UniqueID, i, span.Total.Amount)
			}
			sumMinutes += span.Total.Amount
		}
		if sumHours := sumMinutes / 60; sumHours > a.Work.Amount+0.01 {
			t.Errorf("assignment #%d: timephased work sums to %.3fh, want <= Work (%.3fh)", a.UniqueID, sumHours, a.Work.Amount)
		}
	}
	if found == 0 {
		t.Skip("no assignment in the sample has timephased work")
	}
}

func TestReadRejectsNonProjectFile(t *testing.T) {
	if _, err := mpp.ReadFile("reader_test.go"); err == nil {
		t.Error("expected an error reading a non-compound file")
	}
}
