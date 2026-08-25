// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

// TimeUnit is the unit a Duration is expressed in, mirroring MPXJ's TimeUnit.
type TimeUnit int

const (
	Minutes TimeUnit = iota
	Hours
	Days
	Weeks
	Months
	Years
	Percent
	ElapsedMinutes
	ElapsedHours
	ElapsedDays
	ElapsedWeeks
	ElapsedMonths
	ElapsedYears
	ElapsedPercent
)

// String returns the short name MS Project uses for the unit ("d", "h",
// "ed", ...), with elapsed units carrying the "e" prefix.
func (u TimeUnit) String() string {
	switch u {
	case Minutes:
		return "m"
	case Hours:
		return "h"
	case Days:
		return "d"
	case Weeks:
		return "w"
	case Months:
		return "mo"
	case Years:
		return "y"
	case Percent:
		return "%"
	case ElapsedMinutes:
		return "em"
	case ElapsedHours:
		return "eh"
	case ElapsedDays:
		return "ed"
	case ElapsedWeeks:
		return "ew"
	case ElapsedMonths:
		return "emo"
	case ElapsedYears:
		return "ey"
	case ElapsedPercent:
		return "e%"
	default:
		return "?"
	}
}

// Duration is an amount of work/time expressed in a particular TimeUnit.
type Duration struct {
	Amount float64
	Units  TimeUnit
}
