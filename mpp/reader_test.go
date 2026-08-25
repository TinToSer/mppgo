// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp_test

import (
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

	if got, want := pf.Properties.DefaultCalendarName, "Standard"; got != want {
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
	std := pf.CalendarByName("Standard")
	if std == nil {
		t.Fatal(`no "Standard" calendar`)
	}

	if len(std.Exceptions) != 4 {
		t.Fatalf("len(Exceptions) = %d, want 4", len(std.Exceptions))
	}

	first := std.Exceptions[0]
	if first.Name != "Christmas break" {
		t.Errorf("Exceptions[0].Name = %q, want %q", first.Name, "Christmas break")
	}
	if first.Working() {
		t.Error("Christmas break should be non-working")
	}
	if got, want := first.FromDate.Format("2006-01-02"), "2025-12-24"; got != want {
		t.Errorf("FromDate = %s, want %s", got, want)
	}
	if got, want := first.ToDate.Format("2006-01-02"), "2026-01-01"; got != want {
		t.Errorf("ToDate = %s, want %s", got, want)
	}

	// A date inside the Christmas break is a normal working weekday, so
	// only the exception can make it non-working.
	xmasEve := time.Date(2025, 12, 24, 0, 0, 0, 0, time.UTC)
	if xmasEve.Weekday() != time.Wednesday {
		t.Fatalf("fixture assumption broken: 2025-12-24 is %s", xmasEve.Weekday())
	}
	if std.WorkingOn(xmasEve) {
		t.Error("2025-12-24 should be non-working (inside Christmas break)")
	}
	if !std.IsWorkingDay(time.Wednesday) {
		t.Error("Wednesday should otherwise be a working day")
	}

	// The sample also contains a one-off working Saturday.
	sat := time.Date(2026, 6, 20, 0, 0, 0, 0, time.UTC)
	if sat.Weekday() != time.Saturday {
		t.Fatalf("fixture assumption broken: 2026-06-20 is %s", sat.Weekday())
	}
	if !std.WorkingOn(sat) {
		t.Error("2026-06-20 should be working (exception overrides Saturday)")
	}
	if std.IsWorkingDay(time.Saturday) {
		t.Error("Saturday should otherwise be non-working")
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

func TestReadRejectsNonProjectFile(t *testing.T) {
	if _, err := mpp.ReadFile("reader_test.go"); err == nil {
		t.Error("expected an error reading a non-compound file")
	}
}
