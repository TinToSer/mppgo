// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import "testing"

// The lag value and its units field swapped places in Project 2013.
// Reading them the wrong way round yields a plausible-looking but wrong
// lag rather than an obvious failure, so the split is pinned here.
func TestRelationLagLayout(t *testing.T) {
	lag, units := relationLagLayout(appVersionProject2010)
	if lag != 16 || units != 14 {
		t.Errorf("2010 layout = (lag %d, units %d), want (16, 14)", lag, units)
	}

	for _, v := range []int{appVersionProject2013, appVersionProject2016} {
		lag, units := relationLagLayout(v)
		if lag != 14 || units != 18 {
			t.Errorf("version %d layout = (lag %d, units %d), want (14, 18)", v, lag, units)
		}
	}
}
