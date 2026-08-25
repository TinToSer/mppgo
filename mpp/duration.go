// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import "github.com/tintoser/mppgo/project"

// MPP stores most durations as a count of tenths of a minute, paired with a
// separate units field saying which unit MS Project displays them in. The
// units field carries flag bits above the unit code, so it is masked before
// being interpreted.
const durationUnitsMask = 0x1F

// MS Project's own defaults, used when a file does not record these
// project settings. Without them a "days" duration cannot be converted at
// all, and defaulting is much better than reporting zero.
const (
	defaultMinutesPerDay  = 8 * 60
	defaultMinutesPerWeek = 5 * 8 * 60
	defaultDaysPerMonth   = 20
)

// durationTimeUnits maps an MPP duration-units code to a TimeUnit. Codes
// this reader does not recognise fall back to days, as MS Project does.
// Code 21 means "whatever the project's default duration unit is".
func durationTimeUnits(code int, projectDefault project.TimeUnit) project.TimeUnit {
	switch code & durationUnitsMask {
	case 3:
		return project.Minutes
	case 4:
		return project.ElapsedMinutes
	case 5:
		return project.Hours
	case 6:
		return project.ElapsedHours
	case 7:
		return project.Days
	case 8:
		return project.ElapsedDays
	case 9:
		return project.Weeks
	case 10:
		return project.ElapsedWeeks
	case 11:
		return project.Months
	case 12:
		return project.ElapsedMonths
	case 19:
		return project.Percent
	case 20:
		return project.ElapsedPercent
	case 21:
		return projectDefault
	default:
		return project.Days
	}
}

// durationScale carries the project settings that decide how a raw stored
// duration converts into displayed days, weeks and months. Elapsed units
// use fixed calendar arithmetic instead and ignore these.
type durationScale struct {
	minutesPerDay  float64
	minutesPerWeek float64
	daysPerMonth   float64
}

// newDurationScale reads the conversion factors from project properties,
// substituting MS Project's defaults for anything the file omits.
func newDurationScale(props *project.Properties) durationScale {
	s := durationScale{
		minutesPerDay:  float64(props.MinutesPerDay),
		minutesPerWeek: float64(props.MinutesPerWeek),
		daysPerMonth:   float64(props.DaysPerMonth),
	}
	if s.minutesPerDay <= 0 {
		s.minutesPerDay = defaultMinutesPerDay
	}
	if s.minutesPerWeek <= 0 {
		s.minutesPerWeek = defaultMinutesPerWeek
	}
	if s.daysPerMonth <= 0 {
		s.daysPerMonth = defaultDaysPerMonth
	}
	return s
}

// duration converts a raw stored duration (tenths of a minute) into the
// supplied units. A raw value of -1 is MS Project's "no value" encoding and
// yields the zero Duration.
func (s durationScale) duration(raw int, units project.TimeUnit) project.Duration {
	if raw == -1 {
		return project.Duration{}
	}
	v := float64(raw)

	var amount float64
	switch units {
	case project.Minutes, project.ElapsedMinutes:
		amount = v / 10
	case project.Hours, project.ElapsedHours:
		amount = v / 600
	case project.Days:
		amount = v / (s.minutesPerDay * 10)
	case project.ElapsedDays:
		amount = v / (24 * 600)
	case project.Weeks:
		amount = v / (s.minutesPerWeek * 10)
	case project.ElapsedWeeks:
		amount = v / (7 * 24 * 600)
	case project.Months:
		amount = v / (s.minutesPerDay * s.daysPerMonth * 10)
	case project.ElapsedMonths:
		amount = v / (30 * 24 * 600)
	default:
		// Percentages and anything unrecognised pass through unscaled.
		amount = v
	}
	return project.Duration{Amount: amount, Units: units}
}

// Work, cost and unit fields are all stored as 8-byte doubles, but each
// with its own scale and its own "treat as nothing" threshold below which
// MS Project itself shows an empty cell. The helpers below apply both.

// getWork reads a work amount, stored in thousandths of a minute (60000
// per hour), and returns it in hours. Anything under a minute reads as none.
func getWork(data []byte, offset int) project.Duration {
	v := getDouble(data, offset)
	if v < 1000 && v > -1000 {
		return project.Duration{Units: project.Hours}
	}
	return project.Duration{Amount: v / 60000, Units: project.Hours}
}

// getCurrency reads a money amount, stored in hundredths of a currency
// unit. Anything under a tenth of a cent reads as zero.
func getCurrency(data []byte, offset int) float64 {
	v := getDouble(data, offset)
	if v < 0.1 && v > -0.1 {
		return 0
	}
	return v / 100
}

// getUnits reads a resource-units value. The stored figure is scaled by
// 100 relative to the percentage MS Project reports, so a full-time 100%
// is stored as 10000 and read back here as 100. Anything under a tenth of
// a percent reads as zero.
func getUnits(data []byte, offset int) float64 {
	v := getDouble(data, offset)
	if v < 0.1 && v > -0.1 {
		return 0
	}
	return v / 100
}
