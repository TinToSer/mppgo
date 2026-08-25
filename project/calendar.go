// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

import "time"

// TimeRange is a working period within a day, expressed as offsets from midnight.
type TimeRange struct {
	Start time.Duration
	End   time.Duration
}

// Duration returns the length of the working period.
func (r TimeRange) Duration() time.Duration { return r.End - r.Start }

// DayType describes how a calendar treats a given day of the week.
type DayType int

const (
	// DayDefault means "inherit from the base (parent) calendar". MPP marks
	// most days of a derived calendar this way; only the days a user has
	// actually overridden carry explicit values.
	DayDefault DayType = iota
	DayWorking
	DayNonWorking
)

// CalendarException overrides the normal working pattern for a date range
// (a holiday, a one-off working Saturday, etc).
type CalendarException struct {
	FromDate time.Time
	ToDate   time.Time
	Name     string
	Ranges   []TimeRange // empty => non-working
}

// Working reports whether this exception represents a working period.
func (e *CalendarException) Working() bool { return len(e.Ranges) > 0 }

// Covers reports whether the exception applies to the given date.
func (e *CalendarException) Covers(date time.Time) bool {
	d := truncateToDay(date)
	from := truncateToDay(e.FromDate)
	to := truncateToDay(e.ToDate)
	return !d.Before(from) && !d.After(to)
}

func truncateToDay(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// Calendar is a working-time calendar: which days of the week are worked,
// the hours worked on each, and date-specific exceptions.
//
// A calendar may derive from a Parent, in which case any day left as
// DayDefault inherits that parent's working pattern. Use the resolving
// accessors (IsWorkingDay, HoursFor, WorkingOn) rather than reading Days and
// Hours directly, unless you specifically want this calendar's own overrides.
type Calendar struct {
	UniqueID int
	Name     string
	GUID     string
	Parent   *Calendar

	Days       map[time.Weekday]DayType
	Hours      map[time.Weekday][]TimeRange
	Exceptions []*CalendarException
}

// NewCalendar creates an empty calendar with every day set to DayDefault.
func NewCalendar() *Calendar {
	return &Calendar{
		Days:  make(map[time.Weekday]DayType),
		Hours: make(map[time.Weekday][]TimeRange),
	}
}

// DayType returns this calendar's own setting for a day, without resolving
// through the parent chain.
func (c *Calendar) DayType(day time.Weekday) DayType { return c.Days[day] }

// SetWorkingDay marks a day as explicitly working or non-working.
func (c *Calendar) SetWorkingDay(day time.Weekday, working bool) {
	if working {
		c.Days[day] = DayWorking
	} else {
		c.Days[day] = DayNonWorking
	}
}

// resolve walks up the parent chain to the calendar that actually defines
// the given day. Returns nil if no calendar in the chain defines it.
// The depth limit guards against a cyclic Parent chain in a malformed file.
func (c *Calendar) resolve(day time.Weekday) *Calendar {
	for cal, depth := c, 0; cal != nil && depth < 32; cal, depth = cal.Parent, depth+1 {
		if cal.Days[day] != DayDefault {
			return cal
		}
	}
	return nil
}

// IsWorkingDay reports whether the given weekday is worked, resolving
// DayDefault through the parent chain. Calendar exceptions are not
// considered; use WorkingOn for a specific date.
func (c *Calendar) IsWorkingDay(day time.Weekday) bool {
	cal := c.resolve(day)
	return cal != nil && cal.Days[day] == DayWorking
}

// HoursFor returns the working periods for the given weekday, resolving
// DayDefault through the parent chain. Returns nil for a non-working day.
func (c *Calendar) HoursFor(day time.Weekday) []TimeRange {
	cal := c.resolve(day)
	if cal == nil || cal.Days[day] != DayWorking {
		return nil
	}
	return cal.Hours[day]
}

// exceptionFor finds the exception covering a date, searching this calendar
// then its ancestors. Later exceptions win over earlier ones.
func (c *Calendar) exceptionFor(date time.Time) *CalendarException {
	for cal, depth := c, 0; cal != nil && depth < 32; cal, depth = cal.Parent, depth+1 {
		for i := len(cal.Exceptions) - 1; i >= 0; i-- {
			if cal.Exceptions[i].Covers(date) {
				return cal.Exceptions[i]
			}
		}
	}
	return nil
}

// WorkingOn reports whether a specific date is worked, taking calendar
// exceptions into account as well as the weekly pattern.
func (c *Calendar) WorkingOn(date time.Time) bool {
	if exc := c.exceptionFor(date); exc != nil {
		return exc.Working()
	}
	return c.IsWorkingDay(date.Weekday())
}

// HoursOn returns the working periods for a specific date, taking calendar
// exceptions into account. Returns nil if the date is not worked.
func (c *Calendar) HoursOn(date time.Time) []TimeRange {
	if exc := c.exceptionFor(date); exc != nil {
		return exc.Ranges
	}
	return c.HoursFor(date.Weekday())
}
