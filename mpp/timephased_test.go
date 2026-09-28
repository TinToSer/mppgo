// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"testing"
	"time"

	"github.com/tintoser/mppgo/project"
)

// fullTimeCalendar is a 24/7 working calendar: every elapsed minute is a
// working minute, so expected values in these tests can be computed by
// plain wall-clock arithmetic rather than also modelling a calendar.
func fullTimeCalendar() *project.Calendar {
	c := project.NewCalendar()
	for d := time.Sunday; d <= time.Saturday; d++ {
		c.SetWorkingDay(d, true)
		c.Hours[d] = []project.TimeRange{{Start: 0, End: 24 * time.Hour}}
	}
	return c
}

func TestReadTimephasedPlannedWorkSummaryBlockOnly(t *testing.T) {
	cal := fullTimeCalendar()
	start := time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)
	finish := start.AddDate(0, 0, 5)

	data := make([]byte, 26)
	// blockCount = 0 at offset 0: everything comes from the summary block.
	putDouble(data, 16, 480*1000) // 480 minutes of work, in 1000ths/minute

	items := readTimephasedPlannedWork(cal, start, finish, data)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if !items[0].Start.Equal(start) || !items[0].Finish.Equal(finish) {
		t.Errorf("Start/Finish = %v/%v, want %v/%v", items[0].Start, items[0].Finish, start, finish)
	}
	if items[0].Total.Amount != 480 {
		t.Errorf("Total = %v, want 480 minutes", items[0].Total.Amount)
	}
}

func TestReadTimephasedPlannedWorkNilOrTooShort(t *testing.T) {
	cal := fullTimeCalendar()
	if got := readTimephasedPlannedWork(cal, time.Time{}, time.Time{}, nil); got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if got := readTimephasedPlannedWork(cal, time.Time{}, time.Time{}, []byte{1, 2, 3}); got != nil {
		t.Errorf("got %v, want nil for too-short data", got)
	}
	if got := readTimephasedPlannedWork(nil, time.Time{}, time.Time{}, make([]byte, 26)); got != nil {
		t.Errorf("got %v, want nil for a nil calendar", got)
	}
}

func TestReadTimephasedPlannedWorkTwoBlocks(t *testing.T) {
	cal := fullTimeCalendar()
	start := time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)

	// Header (16 bytes) + one skipped summary block (28 bytes) + two real
	// 28-byte blocks, each describing 8 hours of work over 8 elapsed hours
	// (480 minutes of each, cumulative).
	data := make([]byte, 16+28+28*2)
	putShort(data, 0, 2) // blockCount

	block1 := 16 + 28
	putDouble(data, block1+0, 480*1000) // cumulative work: 480 min
	putInt(data, block1+24, 480*80)     // cumulative elapsed: 480 min, in 80ths/minute

	block2 := block1 + 28
	putDouble(data, block2+0, 960*1000) // cumulative work: 960 min
	putInt(data, block2+24, 960*80)     // cumulative elapsed: 960 min

	items := readTimephasedPlannedWork(cal, start, time.Time{}, data)
	if len(items) != 2 {
		t.Fatalf("got %d items, want 2", len(items))
	}

	want0Finish := start.Add(8 * time.Hour)
	if !items[0].Start.Equal(start) {
		t.Errorf("item 0 Start = %v, want %v", items[0].Start, start)
	}
	if !items[0].Finish.Equal(want0Finish) {
		t.Errorf("item 0 Finish = %v, want %v", items[0].Finish, want0Finish)
	}
	if items[0].Total.Amount != 480 {
		t.Errorf("item 0 Total = %v, want 480", items[0].Total.Amount)
	}
	if items[0].PerHour.Amount != 60 {
		t.Errorf("item 0 PerHour = %v, want 60 (8h of work over 8 elapsed hours: full rate)", items[0].PerHour.Amount)
	}

	want1Finish := want0Finish.Add(8 * time.Hour)
	if !items[1].Start.Equal(want0Finish) {
		t.Errorf("item 1 Start = %v, want %v (immediately after item 0, 24/7 calendar)", items[1].Start, want0Finish)
	}
	if !items[1].Finish.Equal(want1Finish) {
		t.Errorf("item 1 Finish = %v, want %v", items[1].Finish, want1Finish)
	}
	if items[1].Total.Amount != 480 {
		t.Errorf("item 1 Total = %v, want 480", items[1].Total.Amount)
	}
}

func TestReadTimephasedBaselineWorkNilOrEmpty(t *testing.T) {
	cal := fullTimeCalendar()
	if got := readTimephasedBaselineWork(cal, nil); got != nil {
		t.Errorf("got %v, want nil", got)
	}
	if got := readTimephasedBaselineWork(nil, make([]byte, 60)); got != nil {
		t.Errorf("got %v, want nil for a nil calendar", got)
	}
}

func TestReadTimephasedBaselineCostOneBlock(t *testing.T) {
	cal := fullTimeCalendar()
	start := time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)
	end := start.AddDate(0, 0, 1)

	// Header (16 bytes) + first summary block (20 bytes, carries the start
	// timestamp at its own offset 16) + one real 20-byte block + implicit
	// trailing summary (not read: blockCount-2 stops before it).
	data := make([]byte, 16+20+20)
	putShort(data, 0, 3) // blockCount: 1 leading summary + 1 real + 1 trailing summary
	putInt(data, 32, tenthsSince(start))

	block := 16 + 20
	putDouble(data, block+8, 50000) // cumulative cost: 500.00 (hundredths of a currency unit)
	putInt(data, block+16, tenthsSince(end))

	items := readTimephasedBaselineCost(cal, data)
	if len(items) != 1 {
		t.Fatalf("got %d items, want 1", len(items))
	}
	if !items[0].Start.Equal(start) || !items[0].Finish.Equal(end) {
		t.Errorf("Start/Finish = %v/%v, want %v/%v", items[0].Start, items[0].Finish, start, end)
	}
	if items[0].Total != 500 {
		t.Errorf("Total = %v, want 500", items[0].Total)
	}
	// 500 cost over a full 24/7 day (1440 minutes): 500*60/1440 per hour.
	if want, diff := 500.0*60/1440, items[0].PerHour-500.0*60/1440; diff > 1e-9 || diff < -1e-9 {
		t.Errorf("PerHour = %v, want %v", items[0].PerHour, want)
	}
}
