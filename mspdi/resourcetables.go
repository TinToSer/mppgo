package mspdi

import "github.com/tintoser/mppgo/project"

type xmlRates struct {
	Rates []xmlRate `xml:"Rate"`
}

type xmlRate struct {
	From         xmlDateTime `xml:"RatesFrom"`
	To           xmlDateTime `xml:"RatesTo"`
	Table        int         `xml:"RateTable"`
	Standard     float64     `xml:"StandardRate"`
	StandardUnit int         `xml:"StandardRateFormat"`
	Overtime     float64     `xml:"OvertimeRate"`
	OvertimeUnit int         `xml:"OvertimeRateFormat"`
	PerUse       float64     `xml:"CostPerUse"`
}

type xmlAvailabilityPeriods struct {
	Periods []xmlAvailabilityPeriod `xml:"AvailabilityPeriod"`
}

type xmlAvailabilityPeriod struct {
	From  xmlDateTime `xml:"AvailableFrom"`
	To    xmlDateTime `xml:"AvailableTo"`
	Units float64     `xml:"AvailableUnits"`
}

func readResourceTables(resource *project.Resource, value xmlResource) {
	if value.Rates != nil {
		for _, rate := range value.Rates.Rates {
			if rate.Table < 0 || rate.Table >= len(resource.CostRateTables) {
				continue
			}
			entry := project.CostRateTableEntry{StandardRate: rate.Standard, StandardRateUnits: readRateUnit(rate.StandardUnit),
				OvertimeRate: rate.Overtime, OvertimeRateUnits: readRateUnit(rate.OvertimeUnit), CostPerUse: rate.PerUse}
			if rate.From.Valid {
				entry.Start = rate.From.Time
			}
			if rate.To.Valid {
				entry.End = rate.To.Time
			}
			resource.CostRateTables[rate.Table] = append(resource.CostRateTables[rate.Table], entry)
		}
	}
	if value.Availability != nil {
		for _, availability := range value.Availability.Periods {
			entry := project.AvailabilityEntry{MaxUnits: availability.Units * 100}
			if availability.From.Valid {
				entry.Start = availability.From.Time
			}
			if availability.To.Valid {
				entry.End = availability.To.Time
			}
			resource.Availability = append(resource.Availability, entry)
		}
	}
}

func writeResourceTables(resource *project.Resource) (*xmlRates, *xmlAvailabilityPeriods) {
	var rates *xmlRates
	for table, entries := range resource.CostRateTables {
		for _, entry := range entries {
			if rates == nil {
				rates = &xmlRates{}
			}
			rates.Rates = append(rates.Rates, xmlRate{From: writeDate(entry.Start), To: writeDate(entry.End), Table: table,
				Standard: entry.StandardRate, StandardUnit: writeRateUnit(entry.StandardRateUnits),
				Overtime: entry.OvertimeRate, OvertimeUnit: writeRateUnit(entry.OvertimeRateUnits), PerUse: entry.CostPerUse})
		}
	}
	var periods *xmlAvailabilityPeriods
	for _, entry := range resource.Availability {
		if periods == nil {
			periods = &xmlAvailabilityPeriods{}
		}
		periods.Periods = append(periods.Periods, xmlAvailabilityPeriod{
			From: writeDate(entry.Start), To: writeDate(entry.End), Units: entry.MaxUnits / 100})
	}
	return rates, periods
}

func readRateUnit(code int) project.TimeUnit {
	unit, exists := map[int]project.TimeUnit{1: project.Minutes, 2: project.Hours, 3: project.Days, 4: project.Weeks, 5: project.Months, 7: project.Years, 8: project.Hours}[code]
	if !exists {
		return project.Hours
	}
	return unit
}

func writeRateUnit(unit project.TimeUnit) int {
	return map[project.TimeUnit]int{project.Minutes: 1, project.Hours: 2, project.Days: 3, project.Weeks: 4, project.Months: 5, project.Years: 7}[unit]
}
