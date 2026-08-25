// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"testing"

	"github.com/tintoser/mppgo/project"
)

func TestResourceTypeBitLayout(t *testing.T) {
	if offset, mask := resourceTypeBitLayout(appVersionProject2010); offset != 9 || mask != 0x02 {
		t.Errorf("2010 layout = (%d, %#x), want (9, 0x02)", offset, mask)
	}
	if offset, mask := resourceTypeBitLayout(appVersionProject2016); offset != 12 || mask != 0x10 {
		t.Errorf("2016 layout = (%d, %#x), want (12, 0x10)", offset, mask)
	}
}

// resourceType consults two separate records: the work flag lives in the
// primary FixedMeta record, and only when it is clear does the Fixed2Meta
// record decide between cost and material.
func TestResourceType(t *testing.T) {
	meta := make([]byte, 37)
	meta2 := make([]byte, 96)

	meta[12] = 0x10
	if got := resourceType(meta, meta2, appVersionProject2016); got != project.WorkResource {
		t.Errorf("work flag set = %v, want WorkResource", got)
	}

	// With the work flag clear, the second record separates cost from
	// material — and the work flag must win even when both are set.
	meta[12] = 0x10
	meta2[8] = 0x10
	if got := resourceType(meta, meta2, appVersionProject2016); got != project.WorkResource {
		t.Errorf("work flag set (with cost flag also set) = %v, want WorkResource", got)
	}

	meta[12] = 0
	if got := resourceType(meta, meta2, appVersionProject2016); got != project.CostResource {
		t.Errorf("cost flag set = %v, want CostResource", got)
	}

	meta2[8] = 0
	if got := resourceType(meta, meta2, appVersionProject2016); got != project.MaterialResource {
		t.Errorf("neither flag set = %v, want MaterialResource", got)
	}

	// A file without the optional Fixed2Meta record still resolves, rather
	// than panicking on the missing second record.
	if got := resourceType(meta, nil, appVersionProject2016); got != project.MaterialResource {
		t.Errorf("absent Fixed2Meta = %v, want MaterialResource", got)
	}

	// The 2010 layout reads a different byte entirely.
	meta = make([]byte, 37)
	meta[9] = 0x02
	if got := resourceType(meta, nil, appVersionProject2010); got != project.WorkResource {
		t.Errorf("2010 work flag = %v, want WorkResource", got)
	}
	if got := resourceType(meta, nil, appVersionProject2016); got != project.MaterialResource {
		t.Error("the 2010 flag byte must not be consulted for a 2016 file")
	}
}
