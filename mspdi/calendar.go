// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import (
	"time"

	"github.com/tintoser/mppgo/project"
)

type xmlCalendars struct {
	Calendar []xmlCalendar `xml:"Calendar"`
}

type xmlCalendar struct {
	UID             int            `xml:"UID"`
	Name            string         `xml:"Name"`
	GUID            string         `xml:"GUID,omitempty"`
	IsBaseCalendar  bool           `xml:"IsBaseCalendar"`
	BaseCalendarUID *int           `xml:"BaseCalendarUID"`
	WeekDays        *xmlWeekDays   `xml:"WeekDays"`
	Exceptions      *xmlExceptions `xml:"Exceptions"`
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

type xmlException struct {
	Name         string           `xml:"Name"`
	TimePeriod   *xmlTimePeriod   `xml:"TimePeriod"`
	DayWorking   bool             `xml:"DayWorking"`
	WorkingTimes *xmlWorkingTimes `xml:"WorkingTimes"`
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

		if x.WeekDays != nil {
			for _, wd := range x.WeekDays.WeekDay {
				readWeekDay(c, wd)
			}
		}
		if x.Exceptions != nil {
			for _, exc := range x.Exceptions.Exception {
				readException(c, exc)
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
		readExceptionRange(c, "", wd.TimePeriod.FromDate, wd.TimePeriod.ToDate, wd.WorkingTimes)
		return
	}

	day := time.Weekday(wd.DayType - 1)
	c.SetWorkingDay(day, wd.DayWorking)
	if wd.WorkingTimes != nil {
		c.Hours[day] = toTimeRanges(wd.WorkingTimes)
	}
}

func readException(c *project.Calendar, exc xmlException) {
	if exc.TimePeriod == nil {
		return
	}
	readExceptionRange(c, exc.Name, exc.TimePeriod.FromDate, exc.TimePeriod.ToDate, exc.WorkingTimes)
}

func readExceptionRange(c *project.Calendar, name string, from, to xmlDateTime, times *xmlWorkingTimes) {
	if !from.Valid && !to.Valid {
		// Some non-Microsoft exporters write the range into FromTime/ToTime
		// instead; MS Project ignores that malformed form, and so does this
		// reader.
		return
	}
	c.Exceptions = append(c.Exceptions, &project.CalendarException{
		Name:     name,
		FromDate: from.Time,
		ToDate:   to.Time,
		Ranges:   toTimeRanges(times),
	})
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
