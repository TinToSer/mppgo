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
	return e.coversDay(dayKey(date))
}

func (e *CalendarException) coversDay(day int64) bool {
	return day >= dayKey(e.FromDate) && day <= dayKey(e.ToDate)
}

// dayKey numbers calendar dates consecutively (days since 1970-01-01), each
// read in its own time's location. Exception lookups run once per day
// scanned for every exception in the calendar chain, so this avoids the
// cost of a full calendar-date conversion on that hot path.
func dayKey(t time.Time) int64 {
	_, offset := t.Zone()
	seconds := t.Unix() + int64(offset)
	day := seconds / 86400
	if seconds%86400 < 0 {
		day--
	}
	return day
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

	// WorkWeeks are named weekly patterns that replace this calendar's
	// default week for a date range. Days a work week leaves as DayDefault
	// follow the default week.
	WorkWeeks []*WorkWeek
}

// WorkWeek is a weekly working pattern in force for a date range.
type WorkWeek struct {
	Name     string
	FromDate time.Time
	ToDate   time.Time
	Days     map[time.Weekday]DayType
	Hours    map[time.Weekday][]TimeRange
}

// NewWorkWeek creates a work week with every day set to DayDefault.
func NewWorkWeek() *WorkWeek {
	return &WorkWeek{Days: make(map[time.Weekday]DayType), Hours: make(map[time.Weekday][]TimeRange)}
}

// Covers reports whether the work week applies to the given date.
func (w *WorkWeek) Covers(date time.Time) bool {
	day := dayKey(date)
	return day >= dayKey(w.FromDate) && day <= dayKey(w.ToDate)
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
	day := dayKey(date)
	for cal, depth := c, 0; cal != nil && depth < 32; cal, depth = cal.Parent, depth+1 {
		for i := len(cal.Exceptions) - 1; i >= 0; i-- {
			if cal.Exceptions[i].coversDay(day) {
				return cal.Exceptions[i]
			}
		}
	}
	return nil
}

// workWeekFor finds the work week covering a date, searching this calendar
// then its ancestors.
func (c *Calendar) workWeekFor(date time.Time) *WorkWeek {
	day := dayKey(date)
	for cal, depth := c, 0; cal != nil && depth < 32; cal, depth = cal.Parent, depth+1 {
		for _, w := range cal.WorkWeeks {
			if day >= dayKey(w.FromDate) && day <= dayKey(w.ToDate) {
				return w
			}
		}
	}
	return nil
}

// WorkingOn reports whether a specific date is worked, taking calendar
// exceptions and work weeks into account as well as the weekly pattern.
func (c *Calendar) WorkingOn(date time.Time) bool {
	if exc := c.exceptionFor(date); exc != nil {
		return exc.Working()
	}
	if w := c.workWeekFor(date); w != nil && w.Days[date.Weekday()] != DayDefault {
		return w.Days[date.Weekday()] == DayWorking
	}
	return c.IsWorkingDay(date.Weekday())
}

// HoursOn returns the working periods for a specific date, taking calendar
// exceptions and work weeks into account. Returns nil if the date is not
// worked.
func (c *Calendar) HoursOn(date time.Time) []TimeRange {
	if exc := c.exceptionFor(date); exc != nil {
		return exc.Ranges
	}
	if w := c.workWeekFor(date); w != nil && w.Days[date.Weekday()] != DayDefault {
		if w.Days[date.Weekday()] != DayWorking {
			return nil
		}
		return w.Hours[date.Weekday()]
	}
	return c.HoursFor(date.Weekday())
}

// calendarMaxDaysScan bounds every day-by-day walk below, so a pathological
// or corrupt input (an end date centuries away, a calendar that is
// non-working every single day) degrades to a capped result instead of an
// effectively unbounded loop.
const calendarMaxDaysScan = 20000 // ~55 years

// WorkMinutesBetween returns the total working time in [start, end), in
// minutes, according to this calendar's working days/hours and exceptions.
// Used to turn a stored amount of work into the "per hour" rate MS Project
// itself shows for a timephased span (see the project's Duration model);
// not a general-purpose scheduling primitive.
func (c *Calendar) WorkMinutesBetween(start, end time.Time) float64 {
	if !end.After(start) {
		return 0
	}

	var total float64
	day := truncateToDay(start)
	for i := 0; i < calendarMaxDaysScan && !day.After(end); i++ {
		for _, r := range c.HoursOn(day) {
			rangeStart, rangeEnd := day.Add(r.Start), day.Add(r.End)
			os, oe := rangeStart, rangeEnd
			if start.After(os) {
				os = start
			}
			if end.Before(oe) {
				oe = end
			}
			if oe.After(os) {
				total += oe.Sub(os).Minutes()
			}
		}
		day = day.AddDate(0, 0, 1)
	}
	return total
}

// NextWorkStart returns the next instant at or after t that falls within a
// working period — t itself, if it already does.
func (c *Calendar) NextWorkStart(t time.Time) time.Time {
	cursor := t
	day := truncateToDay(t)
	for i := 0; i < calendarMaxDaysScan; i++ {
		for _, r := range c.HoursOn(day) {
			rangeStart, rangeEnd := day.Add(r.Start), day.Add(r.End)
			if cursor.Before(rangeEnd) {
				if cursor.Before(rangeStart) {
					return rangeStart
				}
				return cursor
			}
		}
		day = day.AddDate(0, 0, 1)
		cursor = day
	}
	return t // gave up: leave the caller with what it started with
}

// AdvanceByWork returns the instant reached after minutes of working time,
// starting from the next working instant at or after start (so a start
// that falls outside working hours snaps forward first, matching how MS
// Project itself schedules from a non-working moment).
func (c *Calendar) AdvanceByWork(start time.Time, minutes float64) time.Time {
	cursor := c.NextWorkStart(start)
	if minutes <= 0 {
		return cursor
	}

	remaining := minutes
	day := truncateToDay(cursor)
	for i := 0; i < calendarMaxDaysScan; i++ {
		for _, r := range c.HoursOn(day) {
			rangeStart, rangeEnd := day.Add(r.Start), day.Add(r.End)
			if !rangeEnd.After(cursor) {
				continue
			}
			segStart := rangeStart
			if cursor.After(segStart) {
				segStart = cursor
			}
			available := rangeEnd.Sub(segStart).Minutes()
			if available <= 0 {
				continue
			}
			if remaining <= available {
				return segStart.Add(time.Duration(remaining * float64(time.Minute)))
			}
			remaining -= available
			cursor = rangeEnd
		}
		day = day.AddDate(0, 0, 1)
		cursor = day
	}
	return cursor // gave up: return as far as the scan got
}
