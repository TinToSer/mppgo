// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import "github.com/tintoser/mppgo/project"

type xmlAssignments struct {
	Assignment []xmlAssignment `xml:"Assignment"`
}

type xmlAssignment struct {
	UID         int                     `xml:"UID"`
	TaskUID     int                     `xml:"TaskUID"`
	ResourceUID *int                    `xml:"ResourceUID"`
	Units       float64                 `xml:"Units"`
	Work        string                  `xml:"Work"`
	Start       xmlDateTime             `xml:"Start"`
	Finish      xmlDateTime             `xml:"Finish"`
	Notes       string                  `xml:"Notes"`
	Baseline    []xmlAssignmentBaseline `xml:"Baseline"`
	Timephased  []xmlTimephasedData     `xml:"TimephasedData"`
}

type xmlAssignmentBaseline struct {
	Number int         `xml:"Number"`
	Start  xmlDateTime `xml:"Start"`
	Finish xmlDateTime `xml:"Finish"`
	Work   string      `xml:"Work"`
	Cost   float64     `xml:"Cost"`
}

func readAssignments(xa *xmlAssignments, scale durationScale) []*project.Assignment {
	if xa == nil {
		return nil
	}

	assignments := make([]*project.Assignment, 0, len(xa.Assignment))
	for _, x := range xa.Assignment {
		if x.ResourceUID == nil {
			continue
		}
		a := &project.Assignment{
			UniqueID:         x.UID,
			TaskUniqueID:     x.TaskUID,
			ResourceUniqueID: *x.ResourceUID,
			Units:            x.Units * 100,
			Notes:            x.Notes,
		}
		if w, ok := parseDuration(scale, x.Work, project.Hours); ok {
			a.Work = w
		}
		if x.Start.Valid {
			a.Start = x.Start.Time
		}
		if x.Finish.Valid {
			a.Finish = x.Finish.Time
		}

		for _, b := range x.Baseline {
			bl := &project.Baseline{Cost: b.Cost}
			if b.Start.Valid {
				bl.Start = b.Start.Time
			}
			if b.Finish.Valid {
				bl.Finish = b.Finish.Time
			}
			if w, ok := parseDuration(scale, b.Work, project.Hours); ok {
				bl.Work = w
			}
			if b.Number == 0 {
				a.Baseline = bl
			} else if b.Number >= 1 && b.Number <= 10 {
				if a.Baselines == nil {
					a.Baselines = make(map[int]*project.Baseline)
				}
				a.Baselines[b.Number] = bl
			}
		}

		assignments = append(assignments, a)
	}
	return assignments
}
