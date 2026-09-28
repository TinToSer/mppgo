// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"encoding/binary"
	"math"
	"testing"
	"time"

	"github.com/tintoser/mppgo/project"
)

// putDouble, putInt and putShort write a value at a byte offset in the
// little-endian layout every MPP fixed/var-data field uses, for building
// synthetic fixtures byte by byte.
func putDouble(b []byte, off int, v float64) {
	binary.LittleEndian.PutUint64(b[off:], math.Float64bits(v))
}
func putInt(b []byte, off int, v int32) {
	binary.LittleEndian.PutUint32(b[off:], uint32(v))
}
func putShort(b []byte, off int, v uint16) {
	binary.LittleEndian.PutUint16(b[off:], v)
}

// tenthsSince encodes a time as the raw "tenths of a minute since epoch"
// count getTimestampFromTenths decodes, the inverse of that function — used
// here only to build fixtures.
func tenthsSince(t time.Time) int32 {
	return int32(t.Sub(epoch).Seconds() / 6)
}

var testScale = durationScale{minutesPerDay: 480, minutesPerWeek: 2400, daysPerMonth: 20}

func TestReadResourceCostRateTableNilFallsBackForTableAOnly(t *testing.T) {
	entries := readResourceCostRateTable(nil, 0, 50, 75, 10, testScale)
	if len(entries) != 1 {
		t.Fatalf("table A with no data: got %d entries, want 1 (the synthesized fallback)", len(entries))
	}
	e := entries[0]
	if e.StandardRate != 50 || e.StandardRateUnits != project.Hours {
		t.Errorf("StandardRate = %v %v, want 50 h", e.StandardRate, e.StandardRateUnits)
	}
	if e.OvertimeRate != 75 || e.CostPerUse != 10 {
		t.Errorf("OvertimeRate/CostPerUse = %v/%v, want 75/10", e.OvertimeRate, e.CostPerUse)
	}
	if !e.Start.IsZero() || !e.End.IsZero() {
		t.Errorf("fallback entry should be unbounded, got Start=%v End=%v", e.Start, e.End)
	}

	if got := readResourceCostRateTable(nil, 1, 50, 75, 10, testScale); got != nil {
		t.Errorf("table B (index 1) with no data: got %v, want nil (never customized, not a fallback)", got)
	}
}

func TestReadResourceCostRateTableSingleOpenEndedEntry(t *testing.T) {
	data := make([]byte, 16+44)
	putDouble(data, 16+0, 50)       // standard rate, $/hour
	putShort(data, 16+8, 0xFFFF)    // format: hours (the "just use hours" sentinel)
	putDouble(data, 16+16, 75)      // overtime rate, $/hour
	putShort(data, 16+24, 0xFFFF)   // format: hours
	putDouble(data, 16+32, 1000)    // cost per use, hundredths of a currency unit
	putInt(data, 16+40, 2000000000) // end date: far enough future to be MPXJ's "no upper bound" sentinel

	entries := readResourceCostRateTable(data, 0, 0, 0, 0, testScale)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	e := entries[0]
	if !e.Start.IsZero() || !e.End.IsZero() {
		t.Errorf("expected an unbounded entry, got Start=%v End=%v", e.Start, e.End)
	}
	if e.StandardRate != 50 {
		t.Errorf("StandardRate = %v, want 50", e.StandardRate)
	}
	if e.OvertimeRate != 75 {
		t.Errorf("OvertimeRate = %v, want 75", e.OvertimeRate)
	}
	if e.CostPerUse != 10 {
		t.Errorf("CostPerUse = %v, want 10 (1000/100)", e.CostPerUse)
	}
}

func TestReadResourceCostRateTableRateUnitsConversion(t *testing.T) {
	data := make([]byte, 16+44)
	putDouble(data, 16+0, 60) // $60/hour standard rate
	putShort(data, 16+8, 3)   // format 3 -> Days (workTimeUnit: code 3 = Days)
	putDouble(data, 16+16, 0)
	putShort(data, 16+24, 0xFFFF)
	putDouble(data, 16+32, 0)
	putInt(data, 16+40, 2000000000)

	entries := readResourceCostRateTable(data, 0, 0, 0, 0, testScale)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	// 480 minutes/day of an 8-hour day: $60/h * 8h = $480/day.
	if entries[0].StandardRateUnits != project.Days {
		t.Errorf("StandardRateUnits = %v, want Days", entries[0].StandardRateUnits)
	}
	if want := 480.0; entries[0].StandardRate != want {
		t.Errorf("StandardRate = %v, want %v", entries[0].StandardRate, want)
	}
}

func TestReadResourceCostRateTableTwoEntriesChainStartToPreviousEnd(t *testing.T) {
	firstEnd := time.Date(2020, 6, 15, 23, 59, 0, 0, time.UTC) // not divisible by 5: kept as-is
	data := make([]byte, 16+44*2)

	putDouble(data, 16+0, 40)
	putShort(data, 16+8, 0xFFFF)
	putDouble(data, 16+16, 0)
	putShort(data, 16+24, 0xFFFF)
	putDouble(data, 16+32, 0)
	putInt(data, 16+40, tenthsSince(firstEnd))

	putDouble(data, 60+0, 45)
	putShort(data, 60+8, 0xFFFF)
	putDouble(data, 60+16, 0)
	putShort(data, 60+24, 0xFFFF)
	putDouble(data, 60+32, 0)
	putInt(data, 60+40, 2000000000)

	entries := readResourceCostRateTable(data, 0, 0, 0, 0, testScale)
	if len(entries) != 2 {
		t.Fatalf("got %d entries, want 2", len(entries))
	}
	if !entries[0].Start.IsZero() {
		t.Errorf("first entry Start = %v, want zero (unbounded)", entries[0].Start)
	}
	if !entries[0].End.Equal(firstEnd) {
		t.Errorf("first entry End = %v, want %v", entries[0].End, firstEnd)
	}
	if want := firstEnd.Add(time.Minute); !entries[1].Start.Equal(want) {
		t.Errorf("second entry Start = %v, want %v (first entry's End + 1 minute)", entries[1].Start, want)
	}
	if !entries[1].End.IsZero() {
		t.Errorf("second entry End = %v, want zero (unbounded)", entries[1].End)
	}
	if entries[0].StandardRate != 40 || entries[1].StandardRate != 45 {
		t.Errorf("StandardRate = %v, %v, want 40, 45", entries[0].StandardRate, entries[1].StandardRate)
	}
}

func TestReadResourceAvailabilityNilOrShort(t *testing.T) {
	if got := readResourceAvailability(nil); got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if got := readResourceAvailability([]byte{1, 2, 3}); got != nil {
		t.Errorf("got %v, want nil for a too-short blob", got)
	}
}

func TestReadResourceAvailabilityOneEntry(t *testing.T) {
	start := time.Date(2024, 3, 1, 8, 0, 0, 0, time.UTC)
	nextStart := time.Date(2024, 6, 1, 8, 0, 0, 0, time.UTC) // stored as this entry's "end", -1 minute

	data := make([]byte, 12+20*2) // one real entry plus the trailing sentinel record
	putShort(data, 0, 1)          // item count

	putInt(data, 12+0, tenthsSince(start))
	putDouble(data, 12+4, 15000) // 150% (hundredths of a percent)

	putInt(data, 32+0, tenthsSince(nextStart))

	entries := readResourceAvailability(data)
	if len(entries) != 1 {
		t.Fatalf("got %d entries, want 1", len(entries))
	}
	if !entries[0].Start.Equal(start) {
		t.Errorf("Start = %v, want %v", entries[0].Start, start)
	}
	if want := nextStart.Add(-time.Minute); !entries[0].End.Equal(want) {
		t.Errorf("End = %v, want %v", entries[0].End, want)
	}
	if entries[0].MaxUnits != 150 {
		t.Errorf("MaxUnits = %v, want 150", entries[0].MaxUnits)
	}
}
