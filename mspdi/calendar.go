// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import (
	"time"

	"github.com/tintoser/mppgo/internal/recurrence"
	"github.com/tintoser/mppgo/project"
)

type xmlCalendars struct {
	Calendar []xmlCalendar `xml:"Calendar"`
}

type xmlCalendar struct {
	UID             int            `xml:"UID"`
	GUID            string         `xml:"GUID,omitempty"`
	Name            string         `xml:"Name"`
	IsBaseCalendar  bool           `xml:"IsBaseCalendar"`
	BaseCalendarUID *int           `xml:"BaseCalendarUID"`
	WeekDays        *xmlWeekDays   `xml:"WeekDays"`
	Exceptions      *xmlExceptions `xml:"Exceptions"`
	WorkWeeks       *xmlWorkWeeks  `xml:"WorkWeeks"`
}

type xmlWorkWeeks struct {
	WorkWeek []xmlWorkWeek `xml:"WorkWeek"`
}

type xmlWorkWeek struct {
	TimePeriod *xmlTimePeriod `xml:"TimePeriod"`
	Name       string         `xml:"Name,omitempty"`
	WeekDays   *xmlWeekDays   `xml:"WeekDays"`
}

type xmlWeekDays struct {
	WeekDay []xmlWeekDay `xml:"WeekDay"`
}

type xmlWeekDay struct {
	DayType      int              `xml:"DayType"`
	DayWorking   bool             `xml:"DayWorking"`
	TimePeriod   *xmlTimePeriod   `xml:"TimePeriod"`
	WorkingTimes *xmlWorkingTimes `xml:"WorkingTimes"`
}

type xmlTimePeriod struct {
	FromDate xmlDateTime `xml:"FromDate"`
	ToDate   xmlDateTime `xml:"ToDate"`
}

type xmlWorkingTimes struct {
	WorkingTime []xmlWorkingTime `xml:"WorkingTime"`
}

type xmlWorkingTime struct {
	FromTime xmlTime `xml:"FromTime"`
	ToTime   xmlTime `xml:"ToTime"`
}

type xmlExceptions struct {
	Exception []xmlException `xml:"Exception"`
}

// xmlException's recurrence elements (Occurrences .. MonthDay) are read
// only; the writer emits every exception as a plain date range, which is
// how the model holds them once expanded.
type xmlException struct {
	TimePeriod    *xmlTimePeriod   `xml:"TimePeriod"`
	Occurrences   int              `xml:"Occurrences,omitempty"`
	Name          string           `xml:"Name"`
	Type          int              `xml:"Type,omitempty"`
	PeriodNumber  int              `xml:"PeriodNumber,omitempty"`
	DaysOfWeek    int              `xml:"DaysOfWeek,omitempty"`
	MonthItem     *int             `xml:"MonthItem"`
	MonthPosition *int             `xml:"MonthPosition"`
	Month         *int             `xml:"Month"`
	MonthDay      int              `xml:"MonthDay,omitempty"`
	DayWorking    bool             `xml:"DayWorking"`
	WorkingTimes  *xmlWorkingTimes `xml:"WorkingTimes"`
}

// readCalendars converts every xmlCalendar into a project.Calendar, wiring
// up Parent via BaseCalendarUID once every calendar has been created (a
// derived calendar can appear before the base calendar it names, so parent
// linking is deliberately a second pass). Work weeks (a named, date-ranged
// weekly pattern override distinct from a plain exception) are not read —
// the same scope cut this reader's MPP side makes.
func readCalendars(xc *xmlCalendars) []*project.Calendar {
	if xc == nil {
		return nil
	}

	calendars := make([]*project.Calendar, 0, len(xc.Calendar))
	byID := make(map[int]*project.Calendar, len(xc.Calendar))
	parentOf := make(map[int]int)

	for _, x := range xc.Calendar {
		c := project.NewCalendar()
		c.UniqueID = x.UID
		c.Name = x.Name
		c.GUID = x.GUID

		budget := recurrence.ScanBudget
		if x.WeekDays != nil {
			for _, wd := range x.WeekDays.WeekDay {
				readWeekDay(c, wd)
			}
		}
		if x.Exceptions != nil {
			for _, exc := range x.Exceptions.Exception {
				readException(c, exc, &budget)
			}
		}
		if x.WorkWeeks != nil {
			for _, ww := range x.WorkWeeks.WorkWeek {
				w := project.NewWorkWeek()
				w.Name = ww.Name
				if ww.TimePeriod != nil {
					w.FromDate, w.ToDate = ww.TimePeriod.FromDate.Time, ww.TimePeriod.ToDate.Time
				}
				if ww.WeekDays != nil {
					for _, wd := range ww.WeekDays.WeekDay {
						if wd.DayType < 1 || wd.DayType > 7 {
							continue
						}
						day := time.Weekday(wd.DayType - 1)
						w.Days[day] = project.DayNonWorking
						if wd.DayWorking {
							w.Days[day] = project.DayWorking
							w.Hours[day] = toTimeRanges(wd.WorkingTimes)
						}
					}
				}
				c.WorkWeeks = append(c.WorkWeeks, w)
			}
		}

		if x.BaseCalendarUID != nil {
			parentOf[x.UID] = *x.BaseCalendarUID
		}

		calendars = append(calendars, c)
		byID[x.UID] = c
	}

	for childID, parentID := range parentOf {
		child := byID[childID]
		parent := byID[parentID]
		if child == nil || parent == nil || child == parent {
			continue
		}
		child.Parent = parent
	}

	return calendars
}

// readWeekDay handles both layouts a WeekDay element can carry: DayType 0
// is a legacy (pre-2007) exception smuggled into the WeekDays list rather
// than the dedicated Exceptions block; DayType 1-7 is an ordinary weekly
// pattern entry for that day (1 = Sunday, matching time.Weekday's own
// numbering once shifted down by one).
func readWeekDay(c *project.Calendar, wd xmlWeekDay) {
	if wd.DayType == 0 {
		if wd.TimePeriod == nil {
			return
		}
		readExceptionRange(c, "", wd.TimePeriod.FromDate, wd.TimePeriod.ToDate, wd.WorkingTimes, recurrence.Pattern{}, nil)
		return
	}

	day := time.Weekday(wd.DayType - 1)
	c.SetWorkingDay(day, wd.DayWorking)
	if wd.WorkingTimes != nil {
		c.Hours[day] = toTimeRanges(wd.WorkingTimes)
	}
}

func readException(c *project.Calendar, exc xmlException, budget *int) {
	if exc.TimePeriod == nil {
		return
	}
	readExceptionRange(c, exc.Name, exc.TimePeriod.FromDate, exc.TimePeriod.ToDate, exc.WorkingTimes, exceptionPattern(exc), budget)
}

// exceptionPattern decodes an Exception's recurrence elements. Month and
// MonthPosition are 0-based (MonthPosition 4 = last); MonthItem 3-9 names
// Sunday-Saturday (0-2, "day", "weekday" and "weekend day", have no single
// weekday and so fail validation, falling back to the stated first and last
// occurrence); DaysOfWeek is a bitmap with 1 = Sunday.
func exceptionPattern(exc xmlException) recurrence.Pattern {
	p := recurrence.Pattern{
		Type:        recurrence.Type(exc.Type),
		Period:      exc.PeriodNumber,
		Occurrences: exc.Occurrences,
		DayOfMonth:  exc.MonthDay,
		Weekdays:    uint8(exc.DaysOfWeek),
		Weekday:     -1,
	}
	if exc.Month != nil {
		p.Month = time.Month(*exc.Month + 1)
	}
	if exc.MonthPosition != nil {
		p.Ordinal = *exc.MonthPosition + 1
	}
	if exc.MonthItem != nil && *exc.MonthItem >= 3 && *exc.MonthItem <= 9 {
		p.Weekday = time.Weekday(*exc.MonthItem - 3)
	}
	return p
}

// readExceptionRange adds the exception, expanded into one exception per
// occurrence when pattern recurs. A recurring exception's TimePeriod spans
// only its first to last occurrence; reading it as one range would make
// every day in between an exception. budget is nil for a legacy WeekDay
// exception, which cannot recur.
func readExceptionRange(c *project.Calendar, name string, from, to xmlDateTime, times *xmlWorkingTimes, pattern recurrence.Pattern, budget *int) {
	if !from.Valid && !to.Valid {
		// Some non-Microsoft exporters write the range into FromTime/ToTime
		// instead; MS Project ignores that malformed form, and so does this
		// reader.
		return
	}
	ranges := toTimeRanges(times)
	if budget == nil || pattern.Type == 0 || !from.Valid || !to.Valid {
		c.Exceptions = append(c.Exceptions, &project.CalendarException{Name: name, FromDate: from.Time, ToDate: to.Time, Ranges: ranges})
		return
	}
	first := time.Date(from.Time.Year(), from.Time.Month(), from.Time.Day(), 0, 0, 0, 0, from.Time.Location())
	for _, span := range pattern.Spans(first, to.Time, budget) {
		exc := &project.CalendarException{Name: name, FromDate: span[0], ToDate: span[1], Ranges: append([]project.TimeRange(nil), ranges...)}
		if pattern.Type == recurrence.Daily && max(pattern.Period, 1) == 1 {
			exc.FromDate, exc.ToDate = from.Time, to.Time // the plain range, kept exactly as written
		}
		c.Exceptions = append(c.Exceptions, exc)
	}
}

func toTimeRanges(times *xmlWorkingTimes) []project.TimeRange {
	if times == nil {
		return nil
	}
	var ranges []project.TimeRange
	for _, wt := range times.WorkingTime {
		if !wt.FromTime.Valid || !wt.ToTime.Valid {
			continue
		}
		ranges = append(ranges, project.TimeRange{Start: wt.FromTime.Offset, End: wt.ToTime.Offset})
	}
	return ranges
}
