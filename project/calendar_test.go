// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

import (
	"testing"
	"time"
)

func date(y int, m time.Month, d int) time.Time {
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

func baseCalendar() *Calendar {
	c := NewCalendar()
	c.Name = "Standard"
	for _, d := range []time.Weekday{time.Monday, time.Tuesday, time.Wednesday, time.Thursday, time.Friday} {
		c.SetWorkingDay(d, true)
		c.Hours[d] = []TimeRange{{Start: 9 * time.Hour, End: 17 * time.Hour}}
	}
	c.SetWorkingDay(time.Saturday, false)
	c.SetWorkingDay(time.Sunday, false)
	return c
}

func TestDayDefaultsToInherited(t *testing.T) {
	base := baseCalendar()
	derived := NewCalendar()
	derived.Parent = base

	// The derived calendar overrides nothing, so every day resolves to the
	// parent's pattern.
	if !derived.IsWorkingDay(time.Monday) {
		t.Error("Monday should inherit as working")
	}
	if derived.IsWorkingDay(time.Sunday) {
		t.Error("Sunday should inherit as non-working")
	}
	hours := derived.HoursFor(time.Monday)
	if len(hours) != 1 || hours[0].Start != 9*time.Hour {
		t.Errorf("HoursFor(Monday) = %v, want the parent's 09:00-17:00", hours)
	}
	if derived.DayType(time.Monday) != DayDefault {
		t.Error("the derived calendar should still record Monday as DayDefault")
	}
}

func TestDayOverrideBeatsParent(t *testing.T) {
	base := baseCalendar()
	derived := NewCalendar()
	derived.Parent = base

	// A resource who does not work Mondays but does work Saturdays.
	derived.SetWorkingDay(time.Monday, false)
	derived.SetWorkingDay(time.Saturday, true)
	derived.Hours[time.Saturday] = []TimeRange{{Start: 10 * time.Hour, End: 14 * time.Hour}}

	if derived.IsWorkingDay(time.Monday) {
		t.Error("the Monday override should win over the parent")
	}
	if derived.HoursFor(time.Monday) != nil {
		t.Error("a non-working day should have no hours")
	}
	if !derived.IsWorkingDay(time.Saturday) {
		t.Error("the Saturday override should win over the parent")
	}
	if got := derived.HoursFor(time.Saturday); len(got) != 1 || got[0].End != 14*time.Hour {
		t.Errorf("HoursFor(Saturday) = %v, want the override 10:00-14:00", got)
	}
	// Untouched days still inherit.
	if !derived.IsWorkingDay(time.Tuesday) {
		t.Error("Tuesday should still inherit as working")
	}
}

func TestMultiLevelInheritance(t *testing.T) {
	base := baseCalendar()
	middle := NewCalendar()
	middle.Parent = base
	middle.SetWorkingDay(time.Friday, false)

	leaf := NewCalendar()
	leaf.Parent = middle

	if leaf.IsWorkingDay(time.Friday) {
		t.Error("Friday should resolve through two levels to non-working")
	}
	if !leaf.IsWorkingDay(time.Monday) {
		t.Error("Monday should resolve through two levels to working")
	}
}

// A corrupt file could produce a cyclic parent chain; resolution must
// terminate rather than hang.
func TestCyclicParentChainTerminates(t *testing.T) {
	a := NewCalendar()
	b := NewCalendar()
	a.Parent = b
	b.Parent = a

	done := make(chan bool, 1)
	go func() {
		a.IsWorkingDay(time.Monday)
		a.HoursFor(time.Monday)
		a.WorkingOn(date(2026, 1, 5))
		done <- true
	}()

	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("resolution did not terminate on a cyclic parent chain")
	}
}

func TestExceptionsOverrideWeeklyPattern(t *testing.T) {
	c := baseCalendar()
	c.Exceptions = append(c.Exceptions,
		&CalendarException{ // a week off over a normal working period
			Name:     "Christmas break",
			FromDate: date(2025, 12, 24),
			ToDate:   date(2026, 1, 1),
		},
		&CalendarException{ // a one-off working Saturday
			FromDate: date(2026, 6, 20),
			ToDate:   date(2026, 6, 20),
			Ranges:   []TimeRange{{Start: 9 * time.Hour, End: 13 * time.Hour}},
		},
	)

	// Inside the break: a Wednesday that is normally worked.
	xmas := date(2025, 12, 25)
	if xmas.Weekday() != time.Thursday {
		t.Fatalf("fixture assumption broken: %s", xmas.Weekday())
	}
	if c.WorkingOn(xmas) {
		t.Error("a date inside the break should be non-working")
	}
	if c.HoursOn(xmas) != nil {
		t.Error("a non-working exception should yield no hours")
	}

	// The one-off working Saturday.
	sat := date(2026, 6, 20)
	if !c.WorkingOn(sat) {
		t.Error("the exception should make this Saturday working")
	}
	if got := c.HoursOn(sat); len(got) != 1 || got[0].End != 13*time.Hour {
		t.Errorf("HoursOn(Saturday) = %v, want 09:00-13:00", got)
	}

	// Boundary dates are inclusive.
	if c.WorkingOn(date(2025, 12, 24)) || c.WorkingOn(date(2026, 1, 1)) {
		t.Error("exception date ranges should be inclusive at both ends")
	}
	// A date outside any exception follows the weekly pattern.
	if !c.WorkingOn(date(2026, 3, 4)) {
		t.Error("an ordinary Wednesday should be working")
	}
}

func TestExceptionsInheritFromParent(t *testing.T) {
	base := baseCalendar()
	base.Exceptions = append(base.Exceptions, &CalendarException{
		Name:     "Company holiday",
		FromDate: date(2026, 5, 4),
		ToDate:   date(2026, 5, 4),
	})

	derived := NewCalendar()
	derived.Parent = base

	if derived.WorkingOn(date(2026, 5, 4)) {
		t.Error("a derived calendar should observe the parent's exceptions")
	}
}

func TestTimeRangeDuration(t *testing.T) {
	r := TimeRange{Start: 9 * time.Hour, End: 17*time.Hour + 30*time.Minute}
	if got, want := r.Duration(), 8*time.Hour+30*time.Minute; got != want {
		t.Errorf("Duration = %v, want %v", got, want)
	}
}
