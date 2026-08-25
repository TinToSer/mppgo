// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import "testing"

func TestTaskMilestoneBitLayout(t *testing.T) {
	if offset, mask := taskMilestoneBitLayout(appVersionProject2010); offset != 8 || mask != 0x20 {
		t.Errorf("2010 layout = (%d, %#x), want (8, 0x20)", offset, mask)
	}
	if offset, mask := taskMilestoneBitLayout(appVersionProject2013); offset != 10 || mask != 0x02 {
		t.Errorf("2013 layout = (%d, %#x), want (10, 0x02)", offset, mask)
	}
	if offset, mask := taskMilestoneBitLayout(appVersionProject2016); offset != 10 || mask != 0x02 {
		t.Errorf("2016 layout = (%d, %#x), want (10, 0x02)", offset, mask)
	}
}

func TestTaskActiveBitLayout(t *testing.T) {
	if offset, mask := taskActiveBitLayout(appVersionProject2010); offset != 8 || mask != 0x04 {
		t.Errorf("2010 layout = (%d, %#x), want (8, 0x04)", offset, mask)
	}
	if offset, mask := taskActiveBitLayout(appVersionProject2016); offset != 8 || mask != 0x40 {
		t.Errorf("2016 layout = (%d, %#x), want (8, 0x40)", offset, mask)
	}
}

func TestTaskCalendarUniqueID(t *testing.T) {
	if got := taskCalendarUniqueID(-1); got != 0 {
		t.Errorf("taskCalendarUniqueID(-1) = %d, want 0 (MPXJ's \"unset\" sentinel)", got)
	}
	if got := taskCalendarUniqueID(7); got != 7 {
		t.Errorf("taskCalendarUniqueID(7) = %d, want 7", got)
	}
	if got := taskCalendarUniqueID(0); got != 0 {
		t.Errorf("taskCalendarUniqueID(0) = %d, want 0", got)
	}
}

func TestTaskPercentageClampsOutOfRange(t *testing.T) {
	data := make([]byte, 4)
	copy(data, u16(45))
	if got := taskPercentage(data, 0); got != 45 {
		t.Errorf("taskPercentage(45) = %d, want 45", got)
	}

	copy(data, u16(65535)) // MPXJ's "not applicable" short sentinel
	if got := taskPercentage(data, 0); got != 0 {
		t.Errorf("taskPercentage(out of range) = %d, want 0", got)
	}
}
