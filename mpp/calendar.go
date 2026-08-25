// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"fmt"
	"time"

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

// readTimeRanges decodes count working periods, reading 2-byte start times
// from startBase and 4-byte durations from durationBase.
//
// Zero-length periods are dropped: they contribute no working time, and
// keeping them would let a day with nothing but degenerate periods count as
// a working day. This differs slightly from MPXJ, which keeps them.
func readTimeRanges(data []byte, count, startBase, durationBase int) []project.TimeRange {
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
// working-hours section.
//
// Recurring exceptions are currently flattened to their overall from/to date
// range, and MPP14 "work weeks" (which follow the exception list) are not yet
// decoded.
func applyCalendarExceptions(data []byte, cal *project.Calendar) {
	if len(data) <= calendarHoursSize {
		return
	}
	offset := calendarHoursSize
	count := getShort(data, offset)
	if count == 0 {
		return
	}
	offset += 4

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

		if fromOK && toOK {
			exc := &project.CalendarException{FromDate: fromDate, ToDate: toDate}
			exc.Ranges = readTimeRanges(data, getShort(data, offset+14), offset+20, offset+32)
			if nameLength != 0 {
				exc.Name = getUnicodeString(data, offset+calendarExceptionSize)
			}
			cal.Exceptions = append(cal.Exceptions, exc)
		}

		offset += calendarExceptionSize + nameLength
	}
}
