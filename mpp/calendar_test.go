package mpp

import (
	"encoding/binary"
	"testing"
	"time"

	"github.com/tintoser/mppgo/internal/recurrence"
)

// exceptionRecord builds the fixed 92-byte part of a calendar exception:
// occurrence count at 4, recurrence type at 72 and the type-specific
// parameter bytes from 76.
func exceptionRecord(kind int, occurrences int, params ...byte) []byte {
	rec := make([]byte, calendarExceptionSize)
	binary.LittleEndian.PutUint16(rec[4:], uint16(occurrences))
	binary.LittleEndian.PutUint16(rec[72:], uint16(kind))
	copy(rec[76:], params)
	return rec
}

func date(s string) time.Time {
	d, err := time.Parse("2006-01-02", s)
	if err != nil {
		panic(err)
	}
	return d
}

func occurrenceDates(spans [][2]time.Time) []string {
	var out []string
	for _, span := range spans {
		out = append(out, span[0].Format("2006-01-02")+".."+span[1].Format("2006-01-02"))
	}
	return out
}

func TestExceptionPattern(t *testing.T) {
	for _, tc := range []struct {
		name     string
		rec      []byte
		from, to string
		want     []string
	}{
		// Byte layout taken from a real Project 2016 file: "Neujahr", every
		// 1 January 2017-2046, stored with its first and last occurrence.
		{"yearly by date", exceptionRecord(2, 3, 0, 1), "2017-01-01", "2019-01-01",
			[]string{"2017-01-01..2017-01-01", "2018-01-01..2018-01-01", "2019-01-01..2019-01-01"}},
		{"plain range", exceptionRecord(1, 4), "2017-04-14", "2017-04-17",
			[]string{"2017-04-14..2017-04-17"}},
		{"every other day", exceptionRecord(7, 3, 2, 0), "2026-03-02", "2026-03-06",
			[]string{"2026-03-02..2026-03-02", "2026-03-04..2026-03-04", "2026-03-06..2026-03-06"}},
		{"weekly Mon+Fri", exceptionRecord(6, 4, 0x22, 0, 1, 0), "2026-03-02", "2026-03-13",
			[]string{"2026-03-02..2026-03-02", "2026-03-06..2026-03-06", "2026-03-09..2026-03-09", "2026-03-13..2026-03-13"}},
		{"fortnightly Monday", exceptionRecord(6, 2, 0x02, 0, 2, 0), "2026-03-02", "2026-03-16",
			[]string{"2026-03-02..2026-03-02", "2026-03-16..2026-03-16"}},
		{"monthly day 15", exceptionRecord(4, 2, 15, 0, 1), "2026-01-15", "2026-02-15",
			[]string{"2026-01-15..2026-01-15", "2026-02-15..2026-02-15"}},
		{"last Friday monthly", exceptionRecord(5, 2, 4, 8, 1, 0), "2026-01-30", "2026-02-27",
			[]string{"2026-01-30..2026-01-30", "2026-02-27..2026-02-27"}},
		{"third Monday of January", exceptionRecord(3, 2, 0, 2, 4), "2026-01-19", "2027-01-18",
			[]string{"2026-01-19..2026-01-19", "2027-01-18..2027-01-18"}},
		{"occurrence count bounds expansion", exceptionRecord(2, 1, 0, 1), "2017-01-01", "2019-01-01",
			[]string{"2017-01-01..2017-01-01"}},
		// Parameters that do not decode (month 13) yield only the two dates
		// the record states outright, never the whole range.
		{"undecodable parameters", exceptionRecord(2, 3, 12, 1), "2017-01-01", "2019-01-01",
			[]string{"2017-01-01..2017-01-01", "2019-01-01..2019-01-01"}},
	} {
		budget := recurrence.ScanBudget
		got := occurrenceDates(exceptionPattern(tc.rec).Spans(date(tc.from), date(tc.to), &budget))
		if len(got) != len(tc.want) {
			t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
			continue
		}
		for i := range got {
			if got[i] != tc.want[i] {
				t.Errorf("%s: got %v, want %v", tc.name, got, tc.want)
				break
			}
		}
	}
}

func TestExceptionPatternRespectsScanBudget(t *testing.T) {
	budget := 40 // enough to scan into February, not to reach 2019
	got := occurrenceDates(exceptionPattern(exceptionRecord(2, 3, 0, 1)).Spans(date("2017-01-01"), date("2019-01-01"), &budget))
	want := []string{"2017-01-01..2017-01-01", "2019-01-01..2019-01-01"}
	if len(got) != len(want) || got[0] != want[0] || got[1] != want[1] {
		t.Fatalf("got %v, want %v", got, want)
	}
	if budget != 0 {
		t.Fatalf("budget = %d, want it spent", budget)
	}
}

func TestReadTimeRangesCapsPeriodCount(t *testing.T) {
	data := make([]byte, calendarDayRecordSize)
	for p := 0; p < calendarMaxWorkingPeriods; p++ {
		binary.LittleEndian.PutUint16(data[8+p*2:], uint16(4800+p*600))
		binary.LittleEndian.PutUint32(data[20+p*4:], 600)
	}
	if got := len(readTimeRanges(data, 9, 8, 20)); got != calendarMaxWorkingPeriods {
		t.Fatalf("got %d periods, want %d (a larger count would read durations as start times)", got, calendarMaxWorkingPeriods)
	}
}
