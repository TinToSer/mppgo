// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import (
	"strconv"
	"strings"

	"github.com/tintoser/mppgo/project"
)

// xsdDurationComponents is a parsed xsd:duration value: PnYnMnDTnHnMnS. M
// means months before the "T" time separator and minutes after it — the
// same ambiguity the XSD spec itself resolves positionally, not by a
// different letter.
type xsdDurationComponents struct {
	years, months, days, hours, minutes float64
	seconds                             float64
}

// parseXSDDuration parses an xsd:duration string as MSPDI actually writes
// it, which departs from the spec in two ways this reader tolerates
// because MS Project itself does: a bare "0" for a zero duration (seen
// from at least one third-party exporter), and a negative sign on each
// component instead of one leading the whole value.
func parseXSDDuration(s string) (xsdDurationComponents, bool) {
	var c xsdDurationComponents
	if s == "" {
		return c, false
	}
	if s == "0" {
		return c, true
	}

	negative := false
	i := 0
	if strings.HasPrefix(s, "-P") {
		negative = true
		i = 1
	} else if strings.HasPrefix(s, "P") {
		i = 0
	} else {
		return c, false
	}
	i++ // skip 'P'

	haveTime := false
	n := len(s)
	for i < n {
		start := i
		for i < n && (s[i] == '-' || s[i] == '.' || (s[i] >= '0' && s[i] <= '9')) {
			i++
		}
		if i >= n {
			return c, false
		}
		unit := s[i]
		if unit == 'T' {
			haveTime = true
			i++
			continue
		}
		if start == i {
			return c, false
		}
		value, err := strconv.ParseFloat(s[start:i], 64)
		if err != nil {
			return c, false
		}
		switch unit {
		case 'Y':
			c.years = value
		case 'M':
			if haveTime {
				c.minutes = value
			} else {
				c.months = value
			}
		case 'D':
			c.days = value
		case 'H':
			c.hours = value
		case 'S':
			c.seconds = value
		default:
			return c, false
		}
		i++
	}

	if negative {
		c.years, c.months, c.days = -c.years, -c.months, -c.days
		c.hours, c.minutes, c.seconds = -c.hours, -c.minutes, -c.seconds
	}
	return c, true
}

// amountAndUnit picks the display unit MS Project itself uses when a
// duration's DurationFormat sibling is absent: the largest non-zero
// component, and expresses the whole value in that unit. Mirrors MPXJ's
// DatatypeConverter.parseDuration exactly, including its approximations
// (a month is always 30 days, a year always 365, regardless of the
// project's own MinutesPerDay/DaysPerMonth — those only apply once this
// value is converted to the field's real target unit).
func (c xsdDurationComponents) amountAndUnit() (float64, project.TimeUnit) {
	unit := project.Days
	if c.seconds != 0 || c.minutes != 0 {
		unit = project.Minutes
	}
	if c.hours != 0 {
		unit = project.Hours
	}
	if c.days != 0 {
		unit = project.Days
	}
	if c.months != 0 {
		unit = project.Months
	}
	if c.years != 0 {
		unit = project.Years
	}

	var amount float64
	switch unit {
	case project.Years:
		amount = c.years + c.months/12 + c.days/365 + c.hours/(365*24) + c.minutes/(365*24*60) + c.seconds/(365*24*60*60)
	case project.Months:
		amount = c.years*12 + c.months + c.days/30 + c.hours/(30*24) + c.minutes/(30*24*60) + c.seconds/(30*24*60*60)
	case project.Days:
		amount = c.years*365 + c.months*30 + c.days + c.hours/24 + c.minutes/(24*60) + c.seconds/(24*60*60)
	case project.Hours:
		amount = c.years*365*24 + c.months*30*24 + c.days*24 + c.hours + c.minutes/60 + c.seconds/3600
	case project.Minutes:
		amount = c.years*365*24*60 + c.months*30*24*60 + c.days*24*60 + c.hours*60 + c.minutes + c.seconds/60
	}
	return amount, unit
}

// parseDuration parses an xsd:duration string and converts it to target
// (the field's own DurationFormat, or the project's default duration
// units if the field has none), matching MPXJ's DatatypeConverter.
// parseDuration. Returns the zero Duration, ok=false if s is empty or
// malformed — MS Project itself simply ignores a duration it can't parse.
func parseDuration(scale durationScale, s string, target project.TimeUnit) (project.Duration, bool) {
	c, ok := parseXSDDuration(s)
	if !ok {
		return project.Duration{}, false
	}
	amount, unit := c.amountAndUnit()
	return project.Duration{Amount: scale.convert(amount, unit, target), Units: target}, true
}

// mspdiDurationUnit decodes MSPDI's own DurationFormat/LagFormat integer
// code — not the bit-masked encoding the MPP binary format uses for the
// same idea, so this reader keeps a completely separate table for it
// rather than risk conflating the two. code 0 (absent) and any
// unrecognised value fall back to defaultUnit, matching MS Project's own
// tolerance for a missing or unknown value here.
func mspdiDurationUnit(code int, defaultUnit project.TimeUnit) project.TimeUnit {
	switch code {
	case 3, 35:
		return project.Minutes
	case 4, 36:
		return project.ElapsedMinutes
	case 5, 37:
		return project.Hours
	case 6, 38:
		return project.ElapsedHours
	case 7, 39, 53:
		return project.Days
	case 8, 40:
		return project.ElapsedDays
	case 9, 41:
		return project.Weeks
	case 10, 42:
		return project.ElapsedWeeks
	case 11, 43:
		return project.Months
	case 12, 44:
		return project.ElapsedMonths
	case 19, 51:
		return project.Percent
	case 20, 52:
		return project.ElapsedPercent
	default:
		return defaultUnit
	}
}

// durationScale carries the project settings needed to convert a duration
// between units, mirroring mpp's package-internal equivalent but exported
// within this package only (MSPDI durations arrive as a plain amount in a
// detected unit, not a raw tenths-of-a-minute count, so the conversion
// itself is simpler: a straight amount-to-minutes-to-amount rescale).
type durationScale struct {
	minutesPerDay  float64
	minutesPerWeek float64
	daysPerMonth   float64
}

func newDurationScale(p *project.Properties) durationScale {
	s := durationScale{
		minutesPerDay:  float64(p.MinutesPerDay),
		minutesPerWeek: float64(p.MinutesPerWeek),
		daysPerMonth:   float64(p.DaysPerMonth),
	}
	if s.minutesPerDay <= 0 {
		s.minutesPerDay = 8 * 60
	}
	if s.minutesPerWeek <= 0 {
		s.minutesPerWeek = 5 * 8 * 60
	}
	if s.daysPerMonth <= 0 {
		s.daysPerMonth = 20
	}
	return s
}

func (s durationScale) minutesPerUnit(u project.TimeUnit) float64 {
	switch u {
	case project.Minutes, project.ElapsedMinutes:
		return 1
	case project.Hours, project.ElapsedHours:
		return 60
	case project.Days:
		return s.minutesPerDay
	case project.ElapsedDays:
		return 24 * 60
	case project.Weeks:
		return s.minutesPerWeek
	case project.ElapsedWeeks:
		return 7 * 24 * 60
	case project.Months:
		return s.minutesPerDay * s.daysPerMonth
	case project.ElapsedMonths:
		return 30 * 24 * 60
	case project.Years:
		return s.minutesPerWeek * 52
	case project.ElapsedYears:
		return 365 * 24 * 60
	default:
		return 1
	}
}

// convert rescales amount from one duration unit to another via minutes.
// Percentage units are not time-based, so a percentage passes through
// unscaled rather than being (nonsensically) treated as a minute count.
func (s durationScale) convert(amount float64, from, to project.TimeUnit) float64 {
	if from == project.Percent || from == project.ElapsedPercent || to == project.Percent || to == project.ElapsedPercent {
		return amount
	}
	return amount * s.minutesPerUnit(from) / s.minutesPerUnit(to)
}
