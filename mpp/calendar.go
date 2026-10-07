// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"fmt"
	"time"

	"github.com/tintoser/mppgo/internal/recurrence"
	"github.com/tintoser/mppgo/project"
)

const (
	calendarNameVarType = 1
	calendarDataVarType = 8

	propsDefaultCalendarHours = 37753736
	propsDefaultCalendarName  = 37748750

	// A calendar's Var2Data block holds 7 fixed-size day records before any
	// exception data.
	calendarDayRecordSize = 60
	calendarDayCount      = 7
	calendarHoursSize     = calendarDayRecordSize * calendarDayCount // 420

	// Each exception is a fixed 92-byte record followed by a variable-length
	// name.
	calendarExceptionSize = 92
)

// defaultWorkingWeek indexed by time.Weekday (Sunday=0 .. Saturday=6),
// matching MPXJ's DayOfWeekHelper.ORDERED_DAYS convention.
var defaultWorkingWeek = [calendarDayCount]bool{false, true, true, true, true, true, false}

var (
	defaultWorkingMorning   = project.TimeRange{Start: 8 * time.Hour, End: 12 * time.Hour}
	defaultWorkingAfternoon = project.TimeRange{Start: 13 * time.Hour, End: 17 * time.Hour}
)

// calendarIDLayout captures the version-dependent byte offsets within each
// 12-byte TBkndCal FixedData record. Project 2013+ reordered these fields.
type calendarIDLayout struct {
	calendarID int
	baseID     int
	resourceID int
}

func newCalendarIDLayout(applicationVersion int) calendarIDLayout {
	if applicationVersion > appVersionProject2010 {
		return calendarIDLayout{calendarID: 8, baseID: 0, resourceID: 4}
	}
	return calendarIDLayout{calendarID: 0, baseID: 4, resourceID: 8}
}

// readCalendars reads the TBkndCal storage and returns the calendars it
// defines, plus a map from resource unique ID to that resource's calendar.
func readCalendars(src *streamSource, projectDirPath string, projectProps *Props, applicationVersion int) ([]*project.Calendar, map[int]*project.Calendar, error) {
	dir := projectDirPath + "/TBkndCal"
	layout := newCalendarIDLayout(applicationVersion)

	varMetaRaw, err := src.plain(dir + "/VarMeta")
	if err != nil {
		return nil, nil, err
	}
	varMeta, err := ParseVarMeta(varMetaRaw)
	if err != nil {
		return nil, nil, fmt.Errorf("mpp: calendar VarMeta: %w", err)
	}
	var2Raw, err := src.plain(dir + "/Var2Data")
	if err != nil {
		return nil, nil, err
	}
	varData := ParseVar2Data(varMeta, var2Raw)

	fixedMetaRaw, err := src.plain(dir + "/FixedMeta")
	if err != nil {
		return nil, nil, err
	}
	fixedMeta, err := ParseFixedMeta(fixedMetaRaw, 10)
	if err != nil {
		return nil, nil, fmt.Errorf("mpp: calendar FixedMeta: %w", err)
	}
	fixedRaw, err := src.decoded(dir + "/FixedData")
	if err != nil {
		return nil, nil, err
	}
	fixedData := ParseFixedData(fixedMeta, fixedRaw, 12, 0)

	// Fixed2Data carries calendar GUIDs. It is optional.
	var fixed2Data *FixedData
	if src.has(dir + "/Fixed2Meta") {
		if raw, err := src.plain(dir + "/Fixed2Meta"); err == nil {
			if meta2, err := ParseFixedMeta(raw, 9); err == nil {
				if raw2, err := src.decoded(dir + "/Fixed2Data"); err == nil {
					fixed2Data = ParseFixedData(meta2, raw2, 48, 0)
				}
			}
		}
	}

	defaultCalendarData := projectProps.ByteArray(propsDefaultCalendarHours)
	defaultCalendar := project.NewCalendar()
	applyCalendarHours(defaultCalendarData, nil, defaultCalendar, true)

	byID := make(map[int]*project.Calendar)
	resourceMap := make(map[int]*project.Calendar)
	var calendars []*project.Calendar
	baseLinks := make(map[*project.Calendar]int)

	for i := 0; i < fixedData.ItemCount(); i++ {
		rec := fixedData.ByteArrayValue(i)
		if len(rec) < 12 {
			continue
		}
		var rec2 []byte
		if fixed2Data != nil {
			rec2 = fixed2Data.ByteArrayValue(i)
		}

		// A single FixedData record can pack several 12-byte calendar entries.
		for offset := 0; offset+12 <= len(rec); offset += 12 {
			calendarID := getInt(rec, offset+layout.calendarID)
			baseCalendarID := getInt(rec, offset+layout.baseID)
			resourceID := getInt(rec, offset+layout.resourceID)

			if calendarID <= 0 {
				continue
			}
			if _, exists := byID[calendarID]; exists {
				continue
			}

			data := varData.ByteArray(calendarID, calendarDataVarType)
			isBase := baseCalendarID <= 0 || baseCalendarID == calendarID

			cal := project.NewCalendar()
			cal.UniqueID = calendarID
			cal.Name = varData.UnicodeString(calendarID, calendarNameVarType)
			cal.GUID = getGUID(rec2, 0)

			if isBase {
				// A base calendar with no data of its own falls back to the
				// project default working week.
				if data == nil {
					data = defaultCalendarData
				}
				if data == nil {
					applyDefaultWorkingWeek(cal)
				} else {
					applyCalendarHours(data, defaultCalendar, cal, true)
					applyCalendarExceptions(data, cal)
				}
				// Base calendars should not carry a resource ID, but some
				// files do. Honour it only if that resource is unclaimed.
				if resourceID > 0 {
					if _, taken := resourceMap[resourceID]; !taken {
						resourceMap[resourceID] = cal
					}
				}
			} else {
				// Derived calendar: days it does not override stay
				// DayDefault and resolve through Parent, linked up below.
				if data != nil {
					applyCalendarHours(data, defaultCalendar, cal, false)
					applyCalendarExceptions(data, cal)
				}
				baseLinks[cal] = baseCalendarID
				if resourceID > 0 {
					resourceMap[resourceID] = cal
				}
			}

			byID[calendarID] = cal
			calendars = append(calendars, cal)
		}
	}

	// Base calendar IDs can forward-reference, so links are resolved only
	// once every calendar has been seen.
	for cal, baseID := range baseLinks {
		if base, ok := byID[baseID]; ok && base != cal {
			cal.Parent = base
		}
	}
	breakCalendarCycles(calendars)

	return calendars, resourceMap, nil
}

// breakCalendarCycles severs any Parent link that would form a cycle, which
// a corrupt file could otherwise use to make day resolution loop forever.
func breakCalendarCycles(calendars []*project.Calendar) {
	for _, cal := range calendars {
		slow, fast := cal, cal
		for fast != nil && fast.Parent != nil {
			slow = slow.Parent
			fast = fast.Parent.Parent
			if slow == fast {
				slow.Parent = nil
				break
			}
		}
	}
}

func applyDefaultWorkingWeek(cal *project.Calendar) {
	for i := 0; i < calendarDayCount; i++ {
		day := time.Weekday(i)
		working := defaultWorkingWeek[i]
		cal.SetWorkingDay(day, working)
		if working {
			cal.Hours[day] = []project.TimeRange{defaultWorkingMorning, defaultWorkingAfternoon}
		}
	}
}

// applyCalendarHours decodes the seven 60-byte day records at the start of a
// calendar's Var2Data block.
//
// Each record is: a 2-byte flag (1 = "use the default for this day"), a
// 2-byte count of working periods, then the period start times (2 bytes
// each, from offset 8) and durations (4 bytes each, from offset 20).
func applyCalendarHours(data []byte, defaultCalendar, cal *project.Calendar, isBaseCalendar bool) {
	for i := 0; i < calendarDayCount; i++ {
		day := time.Weekday(i)
		offset := calendarDayRecordSize * i

		usesDefault := data == nil || getShort(data, offset) == 1
		if usesDefault {
			if !isBaseCalendar {
				// Leave as DayDefault so it resolves through the parent.
				continue
			}
			if defaultCalendar == nil {
				working := defaultWorkingWeek[i]
				cal.SetWorkingDay(day, working)
				if working {
					cal.Hours[day] = []project.TimeRange{defaultWorkingMorning, defaultWorkingAfternoon}
				}
				continue
			}
			working := defaultCalendar.IsWorkingDay(day)
			cal.SetWorkingDay(day, working)
			if working {
				cal.Hours[day] = append([]project.TimeRange(nil), defaultCalendar.HoursFor(day)...)
			}
			continue
		}

		ranges := readTimeRanges(data, getShort(data, offset+2), offset+8, offset+20)
		if len(ranges) == 0 {
			cal.SetWorkingDay(day, false)
			continue
		}
		cal.SetWorkingDay(day, true)
		cal.Hours[day] = ranges
	}
}

// calendarMaxWorkingPeriods is the number of working-period slots a day or
// exception record has room for: five 2-byte start times are immediately
// followed by the 4-byte durations, so a larger stored count would read
// durations as start times.
const calendarMaxWorkingPeriods = 5

// readTimeRanges decodes count working periods, reading 2-byte start times
// from startBase and 4-byte durations from durationBase.
//
// Zero-length periods are dropped: they contribute no working time, and
// keeping them would let a day with nothing but degenerate periods count as
// a working day. This differs slightly from MPXJ, which keeps them.
func readTimeRanges(data []byte, count, startBase, durationBase int) []project.TimeRange {
	count = min(count, calendarMaxWorkingPeriods)
	var ranges []project.TimeRange
	for p := 0; p < count; p++ {
		startOffset := startBase + p*2
		durationOffset := durationBase + p*4
		if durationOffset+4 > len(data) {
			break
		}
		start := getTime(data, startOffset)
		duration := getDuration(data, durationOffset)
		if duration <= 0 {
			continue
		}
		ranges = append(ranges, project.TimeRange{Start: start, End: start + duration})
	}
	return ranges
}

// applyCalendarExceptions decodes the date-range exceptions that follow the
// working-hours section. A recurring exception ("every 1 January") is
// expanded into one exception per occurrence (see exceptionPattern):
// its stored from/to dates are only the first and last occurrence, and
// treating them as one continuous range would make every day in between a
// holiday. MPP14 "work weeks" (which follow the exception list) are not yet
// decoded.
func applyCalendarExceptions(data []byte, cal *project.Calendar) {
	if len(data) <= calendarHoursSize {
		return
	}
	offset := calendarHoursSize
	count := getShort(data, offset)
	offset += 4
	defer func() { applyWorkWeeks(data, offset, cal) }()

	budget := recurrence.ScanBudget
	for i := 0; i < count; i++ {
		if offset+calendarExceptionSize > len(data) {
			break
		}

		fromDate, fromOK := getDate(data, offset)
		toDate, toOK := getDate(data, offset+2)

		// Name length is stored at the end of the record and padded to a
		// 4-byte boundary; the name itself follows the fixed part.
		nameLength := getInt(data, offset+88)
		if nameLength < 0 {
			break
		}
		if nameLength%4 != 0 {
			nameLength = (nameLength/4 + 1) * 4
		}

		if fromOK && toOK && !toDate.Before(fromDate) {
			ranges := readTimeRanges(data, getShort(data, offset+14), offset+20, offset+32)
			var name string
			if nameLength != 0 {
				name = getUnicodeString(data, offset+calendarExceptionSize)
			}
			pattern := exceptionPattern(data[offset : offset+calendarExceptionSize])
			for _, span := range pattern.Spans(fromDate, toDate, &budget) {
				cal.Exceptions = append(cal.Exceptions, &project.CalendarException{
					FromDate: span[0],
					ToDate:   span[1],
					Name:     name,
					Ranges:   append([]project.TimeRange(nil), ranges...),
				})
			}
		}

		offset += calendarExceptionSize + nameLength
	}
}

// applyWorkWeeks decodes the named work weeks that follow the exception
// list: a 4-byte header, then per week seven 60-byte day records (Sunday
// first, laid out like the default week's), the week's from and to dates,
// 8 unknown bytes, and a name length (padded to 4 bytes) followed by the
// name. Matches MPXJ's processWorkWeeks.
func applyWorkWeeks(data []byte, offset int, cal *project.Calendar) {
	const fixedSize = calendarHoursSize + 2 + 2 + 8 + 4
	offset += 4
	for offset >= 0 && len(data) >= offset+fixedSize {
		w := project.NewWorkWeek()
		for i := 0; i < calendarDayCount; i++ {
			day := time.Weekday(i)
			if getShort(data, offset) != 1 {
				ranges := readTimeRanges(data, getShort(data, offset+2), offset+8, offset+20)
				w.Days[day] = project.DayNonWorking
				if len(ranges) != 0 {
					w.Days[day] = project.DayWorking
					w.Hours[day] = ranges
				}
			}
			offset += calendarDayRecordSize
		}
		w.FromDate, _ = getDate(data, offset)
		w.ToDate, _ = getDate(data, offset+2)
		offset += 4 + 8
		nameLength := getInt(data, offset)
		offset += 4
		if nameLength < 0 {
			return
		}
		if nameLength%4 != 0 {
			nameLength = (nameLength/4 + 1) * 4
		}
		if nameLength != 0 {
			w.Name = getUnicodeString(data[:min(len(data), offset+nameLength)], offset)
			offset += nameLength
		}
		cal.WorkWeeks = append(cal.WorkWeeks, w)
	}
}

// exceptionPattern decodes the recurrence pattern of one 92-byte exception
// record: 4 occurrence count, 72 recurrence type, and from 76 the type's
// own parameters (MPXJ's AbstractCalendarAndExceptionFactory layout; see
// NOTICE):
//
//	1 daily, every day       (the stored range itself)
//	7 daily, every N days    76 N (short)
//	6 weekly                 76 weekday bitmap (bit 0 = Sunday), 78 every N weeks (short)
//	4 monthly by date        76 day of month, 78 every N months (byte)
//	5 monthly by position    76 ordinal (0-3, 4 = last), 77 weekday (3 = Sunday), 78 every N months (short)
//	2 yearly by date         76 month (0-11), 77 day of month
//	3 yearly by position     76 month (0-11), 77 ordinal, 78 weekday (3 = Sunday)
//
// Types 1 and 2 are verified against files saved by Project 2016+; a value
// that does not decode is caught by Pattern.Spans.
func exceptionPattern(rec []byte) recurrence.Pattern {
	p := recurrence.Pattern{
		Type:        recurrence.Type(getShort(rec, 72)),
		Occurrences: getShort(rec, 4),
		Weekday:     -1,
	}
	switch getShort(rec, 72) {
	case 7:
		p.Type, p.Period = recurrence.Daily, getShort(rec, 76)
	case int(recurrence.Weekly):
		p.Weekdays, p.Period = uint8(getByte(rec, 76)), getShort(rec, 78)
	case int(recurrence.MonthlyByDate):
		p.DayOfMonth, p.Period = getByte(rec, 76), getByte(rec, 78)
	case int(recurrence.MonthlyByPosition):
		p.Ordinal, p.Weekday, p.Period = getByte(rec, 76)+1, time.Weekday(getByte(rec, 77)-3), getShort(rec, 78)
	case int(recurrence.YearlyByDate):
		p.Month, p.DayOfMonth = time.Month(getByte(rec, 76)+1), getByte(rec, 77)
	case int(recurrence.YearlyByPosition):
		p.Month, p.Ordinal, p.Weekday = time.Month(getByte(rec, 76)+1), getByte(rec, 77)+1, time.Weekday(getByte(rec, 78)-3)
	}
	return p
}
