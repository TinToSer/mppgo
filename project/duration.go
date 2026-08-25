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
)

// Duration is an amount of work/time expressed in a particular TimeUnit.
type Duration struct {
	Amount float64
	Units  TimeUnit
}
