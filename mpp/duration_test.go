// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"math"
	"testing"

	"github.com/tintoser/mppgo/project"
)

func TestDurationTimeUnits(t *testing.T) {
	cases := map[int]project.TimeUnit{
		3:  project.Minutes,
		4:  project.ElapsedMinutes,
		5:  project.Hours,
		6:  project.ElapsedHours,
		7:  project.Days,
		8:  project.ElapsedDays,
		9:  project.Weeks,
		10: project.ElapsedWeeks,
		11: project.Months,
		12: project.ElapsedMonths,
		19: project.Percent,
		20: project.ElapsedPercent,
	}
	for code, want := range cases {
		if got := durationTimeUnits(code, project.Days); got != want {
			t.Errorf("durationTimeUnits(%d) = %v, want %v", code, got, want)
		}
	}

	// Code 21 defers to the project's own default unit.
	if got := durationTimeUnits(21, project.Hours); got != project.Hours {
		t.Errorf("durationTimeUnits(21) = %v, want the supplied default (Hours)", got)
	}
	// Unrecognised codes fall back to days, as MS Project does.
	if got := durationTimeUnits(63&^durationUnitsMask|1, project.Hours); got != project.Days {
		t.Errorf("durationTimeUnits(unknown) = %v, want Days", got)
	}

	// The units field carries flag bits above the unit code, which must be
	// masked off before the code is interpreted.
	if got := durationTimeUnits(0x20|7, project.Days); got != project.Days {
		t.Errorf("durationTimeUnits with high flag bits = %v, want Days", got)
	}
	if got := durationTimeUnits(0x20|5, project.Days); got != project.Hours {
		t.Errorf("durationTimeUnits with high flag bits = %v, want Hours", got)
	}
}

func TestDurationScaleConversions(t *testing.T) {
	// The MS Project defaults: an 8-hour day, a 40-hour week.
	s := newDurationScale(&project.Properties{})

	// Raw durations are counts of tenths of a minute.
	cases := []struct {
		raw   int
		units project.TimeUnit
		want  float64
	}{
		{600, project.Minutes, 60},        // 600 tenths = 60 minutes
		{600, project.Hours, 1},           // 600 tenths = 1 hour
		{4800, project.Days, 1},           // 8h at 480 min/day
		{24000, project.Weeks, 1},         // 40h at 2400 min/week
		{14400, project.ElapsedDays, 1},   // 24h
		{100800, project.ElapsedWeeks, 1}, // 7 * 24h
		{96000, project.Months, 1},        // 20 days of 8h
		{432000, project.ElapsedMonths, 1},
	}
	for _, c := range cases {
		got := s.duration(c.raw, c.units)
		if math.Abs(got.Amount-c.want) > 1e-9 {
			t.Errorf("duration(%d, %v) = %v, want %v", c.raw, c.units, got.Amount, c.want)
		}
		if got.Units != c.units {
			t.Errorf("duration(%d, %v) units = %v, want %v", c.raw, c.units, got.Units, c.units)
		}
	}
}

func TestDurationScaleHonoursProjectSettings(t *testing.T) {
	// A project set to a 12-hour day reports the same stored duration as
	// fewer days than an 8-hour one — this is exactly why the conversion
	// cannot be hardcoded.
	twelve := newDurationScale(&project.Properties{MinutesPerDay: 720})
	if got := twelve.duration(4800, project.Days); math.Abs(got.Amount-(8.0/12.0)) > 1e-9 {
		t.Errorf("8h at 12h/day = %v days, want %v", got.Amount, 8.0/12.0)
	}
}

func TestDurationScaleFallsBackToDefaults(t *testing.T) {
	// A file that records nothing must still convert, using MS Project's
	// own defaults rather than dividing by zero.
	s := newDurationScale(&project.Properties{})
	if s.minutesPerDay != defaultMinutesPerDay {
		t.Errorf("minutesPerDay = %v, want the default %v", s.minutesPerDay, float64(defaultMinutesPerDay))
	}
	for _, u := range []project.TimeUnit{project.Days, project.Weeks, project.Months} {
		if got := s.duration(4800, u); math.IsInf(got.Amount, 0) || math.IsNaN(got.Amount) {
			t.Errorf("duration(4800, %v) = %v, want a finite value", u, got.Amount)
		}
	}
}

func TestDurationScaleNoValueSentinel(t *testing.T) {
	s := newDurationScale(&project.Properties{})
	if got := s.duration(-1, project.Days); got.Amount != 0 || got.Units != project.Minutes {
		t.Errorf("duration(-1) = %+v, want the zero Duration", got)
	}
}

func TestGetWork(t *testing.T) {
	data := make([]byte, 8)
	putFloat64(data, 60000) // one hour
	if got := getWork(data, 0); got.Amount != 1 || got.Units != project.Hours {
		t.Errorf("getWork(60000) = %+v, want 1 hour", got)
	}

	// Under a minute reads as none, matching MS Project's own display.
	putFloat64(data, 999)
	if got := getWork(data, 0); got.Amount != 0 {
		t.Errorf("getWork(999) = %v, want 0", got.Amount)
	}
}

func TestGetCurrencyAndUnits(t *testing.T) {
	data := make([]byte, 8)

	putFloat64(data, 12345) // stored in hundredths
	if got := getCurrency(data, 0); math.Abs(got-123.45) > 1e-9 {
		t.Errorf("getCurrency(12345) = %v, want 123.45", got)
	}
	putFloat64(data, 0.05)
	if got := getCurrency(data, 0); got != 0 {
		t.Errorf("getCurrency(0.05) = %v, want 0 (below the threshold)", got)
	}

	// A full-time resource is stored as 10000 and reported as 100 percent.
	putFloat64(data, 10000)
	if got := getUnits(data, 0); math.Abs(got-100) > 1e-9 {
		t.Errorf("getUnits(10000) = %v, want 100", got)
	}
}
