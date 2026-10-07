package mspdi

import (
	"math"
	"strconv"

	"github.com/tintoser/mppgo/project"
)

type xmlTimephasedData struct {
	Type   int         `xml:"Type"`
	UID    int         `xml:"UID"`
	Start  xmlDateTime `xml:"Start"`
	Finish xmlDateTime `xml:"Finish"`
	Unit   int         `xml:"Unit"`
	Value  string      `xml:"Value"`
}

func readTimephased(file *project.File, assignments *xmlAssignments, scale durationScale) {
	if assignments == nil {
		return
	}
	byID := make(map[int]*project.Assignment)
	for _, assignment := range file.Assignments {
		byID[assignment.UniqueID] = assignment
	}
	for _, value := range assignments.Assignment {
		assignment := byID[value.UID]
		if assignment == nil {
			continue
		}
		for _, span := range value.Timephased {
			assignment.RawTimephased = append(assignment.RawTimephased, project.TimephasedData{
				Type: span.Type, UID: span.UID, Start: span.Start.Time, Finish: span.Finish.Time, Unit: span.Unit, Value: span.Value})
			if !span.Start.Valid || !span.Finish.Valid || span.Finish.Time.Before(span.Start.Time) {
				continue
			}
			baseline, cost, supported := timephasedKind(span.Type)
			if !supported {
				continue
			}
			minutes := span.Finish.Time.Sub(span.Start.Time).Minutes()
			if calendar := file.AssignmentCalendar(assignment); calendar != nil {
				minutes = calendar.WorkMinutesBetween(span.Start.Time, span.Finish.Time)
			}
			if cost {
				number, err := strconv.ParseFloat(span.Value, 64)
				if err != nil || math.IsInf(number, 0) || math.IsNaN(number) {
					continue
				}
				period := project.TimephasedCost{Start: span.Start.Time, Finish: span.Finish.Time, Total: number}
				if minutes > 0 {
					period.PerHour = number / (minutes / 60)
				}
				if assignment.TimephasedBaselineCost == nil {
					assignment.TimephasedBaselineCost = make(map[int][]project.TimephasedCost)
				}
				assignment.TimephasedBaselineCost[baseline] = append(assignment.TimephasedBaselineCost[baseline], period)
				continue
			}
			work, ok := parseDuration(scale, span.Value, project.Hours)
			if !ok {
				continue
			}
			period := project.TimephasedWork{Start: span.Start.Time, Finish: span.Finish.Time, Total: work, PerHour: project.Duration{Units: project.Hours}}
			if minutes > 0 {
				period.PerHour.Amount = work.Amount / (minutes / 60)
			}
			switch span.Type {
			case 1:
				assignment.TimephasedWork = append(assignment.TimephasedWork, period)
			case 2:
				assignment.TimephasedActualWork = append(assignment.TimephasedActualWork, period)
			default:
				if assignment.TimephasedBaselineWork == nil {
					assignment.TimephasedBaselineWork = make(map[int][]project.TimephasedWork)
				}
				assignment.TimephasedBaselineWork[baseline] = append(assignment.TimephasedBaselineWork[baseline], period)
			}
		}
	}
}

func timephasedKind(kind int) (int, bool, bool) {
	switch kind {
	case 1, 2, 4:
		return 0, false, true
	case 5:
		return 0, true, true
	}
	if kind >= 16 && kind <= 71 {
		position := (kind - 16) % 6
		if position <= 1 {
			return (kind-16)/6 + 1, position == 1, true
		}
	}
	return 0, false, false
}

func writeTimephased(assignment *project.Assignment, scale durationScale) []xmlTimephasedData {
	var result []xmlTimephasedData
	if len(assignment.RawTimephased) != 0 {
		for _, span := range assignment.RawTimephased {
			result = append(result, xmlTimephasedData{Type: span.Type, UID: span.UID, Start: writeDate(span.Start),
				Finish: writeDate(span.Finish), Unit: span.Unit, Value: span.Value})
		}
		return result
	}
	work := func(kind int, spans []project.TimephasedWork) {
		for _, span := range spans {
			result = append(result, xmlTimephasedData{Type: kind, UID: assignment.UniqueID,
				Start: writeDate(span.Start), Finish: writeDate(span.Finish), Unit: 2, Value: writeDuration(span.Total, scale)})
		}
	}
	work(1, assignment.TimephasedWork)
	work(2, assignment.TimephasedActualWork)
	for baseline := 0; baseline <= 10; baseline++ {
		workType, costType := 4, 5
		if baseline > 0 {
			workType = 16 + (baseline-1)*6
			costType = workType + 1
		}
		work(workType, assignment.TimephasedBaselineWork[baseline])
		for _, span := range assignment.TimephasedBaselineCost[baseline] {
			result = append(result, xmlTimephasedData{Type: costType, UID: assignment.UniqueID,
				Start: writeDate(span.Start), Finish: writeDate(span.Finish), Unit: 2, Value: strconv.FormatFloat(span.Total, 'f', -1, 64)})
		}
	}
	return result
}
