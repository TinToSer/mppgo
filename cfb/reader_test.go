// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package cfb_test

import (
	"os"
	"testing"

	"github.com/tintoser/mppgo/cfb"
)

// sampleFile is a real-world fixture, not checked into the repo because
// MPP files carry project-specific data. Place one at this path locally to
// run these tests; otherwise they skip.
const sampleFile = "../testdata/sample.mpp"

func TestOpenAndLookup(t *testing.T) {
	f, err := os.Open(sampleFile)
	if err != nil {
		t.Skipf("sample file unavailable: %v", err)
	}
	defer f.Close()

	cf, err := cfb.Open(f)
	if err != nil {
		t.Fatalf("cfb.Open: %v", err)
	}

	for _, path := range []string{"Props14", "\x01CompObj", "   114/TBkndCal/VarMeta"} {
		e, err := cf.Lookup(path)
		if err != nil {
			t.Errorf("Lookup(%q): %v", path, err)
			continue
		}
		if !e.IsStream() {
			t.Errorf("Lookup(%q): expected a stream", path)
		}
		if _, err := cf.ReadStream(e); err != nil {
			t.Errorf("ReadStream(%q): %v", path, err)
		}
	}

	if _, err := cf.Lookup("DoesNotExist"); err == nil {
		t.Error("Lookup of a missing entry should return an error")
	}
}
