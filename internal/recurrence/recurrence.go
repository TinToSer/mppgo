// Package recurrence expands a recurring calendar exception ("every 1
// January", "the last Friday of each month") into its individual
// occurrences. MPP and MSPDI both store such an exception as a pattern plus
// its first and last occurrence dates, with the same type codes, so both
// readers share this expansion.
package recurrence

import "time"

// Type is a recurrence type. The codes are the ones both the MPP binary
// format and MSPDI's Exception/Type element use.
type Type int

const (
	Daily             Type = 1
	YearlyByDate      Type = 2
	YearlyByPosition  Type = 3
	MonthlyByDate     Type = 4
	MonthlyByPosition Type = 5
	Weekly            Type = 6
)

// LastOrdinal is the Ordinal meaning "the last <weekday> of the month".
const LastOrdinal = 5

// MaxOccurrences bounds the expansion of one pattern from a corrupt file.
const MaxOccurrences = 10000

// ScanBudget is a sensible budget for Pattern.Spans across one calendar:
// fifty yearly holidays over thirty years scan about half a million days,
// so it only ever stops a corrupt file whose many patterns each span
// centuries.
const ScanBudget = 5_000_000

// Pattern describes when a recurring exception occurs. Only the fields its
// Type uses are read.
type Pattern struct {
	Type        Type
	Period      int // every Period days/weeks/months; < 1 means 1
	Occurrences int // stop after this many; <= 0 means no stated limit

	Month      time.Month   // yearly types
	DayOfMonth int          // the "by date" types
	Ordinal    int          // the "by position" types: 1-4, or LastOrdinal
	Weekday    time.Weekday // the "by position" types
	Weekdays   uint8        // Weekly: bit 0 = Sunday ... bit 6 = Saturday
}

// Spans returns the [from, to] date spans the pattern covers between its
// first occurrence from and last occurrence to, both midnight dates. A
// plain range (a daily pattern recurring every day, or an unknown type) is
// the single span itself; anything else is one single-day span per
// occurrence. A pattern whose parameters are invalid yields just from and
// to, the two dates the file states outright.
//
// budget is the number of days the caller is still willing to scan, shared
// across a calendar's exceptions and decremented here.
func (p Pattern) Spans(from, to time.Time, budget *int) [][2]time.Time {
	if to.Before(from) {
		return nil
	}
	period := max(p.Period, 1)
	limit := p.Occurrences
	if limit <= 0 || limit > MaxOccurrences {
		limit = MaxOccurrences
	}

	var matches func(day time.Time) bool
	switch p.Type {
	case Daily:
		if period == 1 {
			return [][2]time.Time{{from, to}}
		}
		matches = func(day time.Time) bool { return daysBetween(from, day)%period == 0 }
	case YearlyByDate:
		if validMonth(p.Month) && validDay(p.DayOfMonth) {
			matches = func(day time.Time) bool { return day.Month() == p.Month && day.Day() == p.DayOfMonth }
		}
	case YearlyByPosition:
		if validMonth(p.Month) && validPosition(p.Ordinal, p.Weekday) {
			matches = func(day time.Time) bool { return day.Month() == p.Month && isNthWeekday(day, p.Ordinal, p.Weekday) }
		}
	case MonthlyByDate:
		if validDay(p.DayOfMonth) {
			matches = func(day time.Time) bool {
				return day.Day() == p.DayOfMonth && monthsBetween(from, day)%period == 0
			}
		}
	case MonthlyByPosition:
		if validPosition(p.Ordinal, p.Weekday) {
			matches = func(day time.Time) bool {
				return isNthWeekday(day, p.Ordinal, p.Weekday) && monthsBetween(from, day)%period == 0
			}
		}
	case Weekly:
		if days := p.Weekdays & 0x7F; days != 0 {
			firstWeek := from.AddDate(0, 0, -int(from.Weekday()))
			matches = func(day time.Time) bool {
				return days&(1<<day.Weekday()) != 0 && (daysBetween(firstWeek, day)/7)%period == 0
			}
		}
	default:
		// Not a recurrence: the stored range is exactly right.
		return [][2]time.Time{{from, to}}
	}
	if matches == nil {
		if from.Equal(to) {
			return [][2]time.Time{{from, from}}
		}
		return [][2]time.Time{{from, from}, {to, to}}
	}

	var spans [][2]time.Time
	for day := from; !day.After(to) && len(spans) < limit; day = day.AddDate(0, 0, 1) {
		if *budget <= 0 {
			// Only a corrupt file gets here: keep the last occurrence, which
			// the file states outright, and stop scanning.
			spans = append(spans, [2]time.Time{to, to})
			break
		}
		*budget--
		if matches(day) {
			spans = append(spans, [2]time.Time{day, day})
		}
	}
	return spans
}

func validMonth(m time.Month) bool { return m >= time.January && m <= time.December }

func validDay(d int) bool { return d >= 1 && d <= 31 }

func validPosition(ordinal int, weekday time.Weekday) bool {
	return ordinal >= 1 && ordinal <= LastOrdinal && weekday >= time.Sunday && weekday <= time.Saturday
}

// daysBetween counts whole days from a to b, both midnight dates in the
// same location.
func daysBetween(a, b time.Time) int {
	return int(b.Sub(a).Hours() / 24)
}

func monthsBetween(a, b time.Time) int {
	return (b.Year()-a.Year())*12 + int(b.Month()) - int(a.Month())
}

// isNthWeekday reports whether day is the ordinal-th (1-4) given weekday of
// its month, or the last one when ordinal is LastOrdinal.
func isNthWeekday(day time.Time, ordinal int, weekday time.Weekday) bool {
	if day.Weekday() != weekday {
		return false
	}
	if ordinal == LastOrdinal {
		return day.AddDate(0, 0, 7).Month() != day.Month()
	}
	return (day.Day()-1)/7+1 == ordinal
}
