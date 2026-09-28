// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import (
	"testing"

	"github.com/tintoser/mppgo/project"
)

func TestParseXSDDuration(t *testing.T) {
	cases := []struct {
		in   string
		want xsdDurationComponents
		ok   bool
	}{
		{"PT8H0M0S", xsdDurationComponents{hours: 8}, true},
		{"P1DT2H0M0S", xsdDurationComponents{days: 1, hours: 2}, true},
		{"0", xsdDurationComponents{}, true},
		{"-PT8H0M0S", xsdDurationComponents{hours: -8}, true},
		{"PT-8H0M0S", xsdDurationComponents{hours: -8}, true}, // MSPDI's own non-spec negative form
		{"", xsdDurationComponents{}, false},
		{"garbage", xsdDurationComponents{}, false},
	}
	for _, c := range cases {
		got, ok := parseXSDDuration(c.in)
		if ok != c.ok {
			t.Errorf("parseXSDDuration(%q) ok = %v, want %v", c.in, ok, c.ok)
			continue
		}
		if ok && got != c.want {
			t.Errorf("parseXSDDuration(%q) = %+v, want %+v", c.in, got, c.want)
		}
	}
}

func TestXSDDurationAmountAndUnitPicksLargestComponent(t *testing.T) {
	c, ok := parseXSDDuration("P1Y2M3DT4H5M6S")
	if !ok {
		t.Fatal("expected a successful parse")
	}
	_, unit := c.amountAndUnit()
	if unit != project.Years {
		t.Errorf("unit = %v, want Years (the largest non-zero component)", unit)
	}

	c, ok = parseXSDDuration("PT30M0S")
	if !ok {
		t.Fatal("expected a successful parse")
	}
	amount, unit := c.amountAndUnit()
	if unit != project.Minutes || amount != 30 {
		t.Errorf("amount/unit = %v/%v, want 30/Minutes", amount, unit)
	}
}

func TestParseDurationRoundTripsAtSameUnit(t *testing.T) {
	scale := durationScale{minutesPerDay: 480, minutesPerWeek: 2400, daysPerMonth: 20}
	d, ok := parseDuration(scale, "PT8H0M0S", project.Hours)
	if !ok {
		t.Fatal("expected a successful parse")
	}
	if d.Amount != 8 || d.Units != project.Hours {
		t.Errorf("got %v %v, want 8 Hours", d.Amount, d.Units)
	}
}

func TestParseDurationMalformedReturnsNotOK(t *testing.T) {
	scale := durationScale{minutesPerDay: 480, minutesPerWeek: 2400, daysPerMonth: 20}
	if _, ok := parseDuration(scale, "not a duration", project.Hours); ok {
		t.Error("expected ok=false for a malformed duration string")
	}
	if _, ok := parseDuration(scale, "", project.Hours); ok {
		t.Error("expected ok=false for an empty duration string")
	}
}

func TestDurationScaleConvert(t *testing.T) {
	scale := durationScale{minutesPerDay: 480, minutesPerWeek: 2400, daysPerMonth: 20}
	if got, want := scale.convert(1, project.Days, project.Hours), 8.0; got != want {
		t.Errorf("1 day -> hours = %v, want %v", got, want)
	}
	if got, want := scale.convert(60, project.Percent, project.Hours), 60.0; got != want {
		t.Errorf("a percentage should pass through unscaled, got %v want %v", got, want)
	}
}

func TestMSPDIDurationUnit(t *testing.T) {
	if got := mspdiDurationUnit(7, project.Hours); got != project.Days {
		t.Errorf("code 7 = %v, want Days", got)
	}
	if got := mspdiDurationUnit(53, project.Hours); got != project.Days {
		t.Errorf("code 53 (an MSPDI-specific alias) = %v, want Days", got)
	}
	if got := mspdiDurationUnit(9999, project.Weeks); got != project.Weeks {
		t.Errorf("an unrecognised code should fall back to the supplied default, got %v want Weeks", got)
	}
}
