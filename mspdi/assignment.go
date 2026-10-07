// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import "github.com/tintoser/mppgo/project"

type xmlAssignments struct {
	Assignment []xmlAssignment `xml:"Assignment"`
}

type xmlAssignment struct {
	UID                 int                     `xml:"UID"`
	GUID                string                  `xml:"GUID,omitempty"`
	TaskUID             int                     `xml:"TaskUID"`
	ResourceUID         *int                    `xml:"ResourceUID"`
	PercentWorkComplete float64                 `xml:"PercentWorkComplete"`
	ActualCost          float64                 `xml:"ActualCost"`
	ActualFinish        xmlDateTime             `xml:"ActualFinish"`
	ActualOvertimeWork  string                  `xml:"ActualOvertimeWork,omitempty"`
	ActualStart         xmlDateTime             `xml:"ActualStart"`
	ActualWork          string                  `xml:"ActualWork,omitempty"`
	ACWP                float64                 `xml:"ACWP"`
	Confirmed           bool                    `xml:"Confirmed"`
	Cost                float64                 `xml:"Cost"`
	CostRateTable       int                     `xml:"CostRateTable"`
	Delay               *int                    `xml:"Delay"`
	Finish              xmlDateTime             `xml:"Finish"`
	Hyperlink           string                  `xml:"Hyperlink,omitempty"`
	HyperlinkAddress    string                  `xml:"HyperlinkAddress,omitempty"`
	HyperlinkSubAddress string                  `xml:"HyperlinkSubAddress,omitempty"`
	LevelingDelay       *int                    `xml:"LevelingDelay"`
	LevelingDelayFormat int                     `xml:"LevelingDelayFormat,omitempty"`
	Notes               string                  `xml:"Notes"`
	OvertimeCost        float64                 `xml:"OvertimeCost"`
	OvertimeWork        string                  `xml:"OvertimeWork,omitempty"`
	RegularWork         string                  `xml:"RegularWork,omitempty"`
	RemainingCost       float64                 `xml:"RemainingCost"`
	RemainingWork       string                  `xml:"RemainingWork,omitempty"`
	ResponsePending     bool                    `xml:"ResponsePending"`
	Start               xmlDateTime             `xml:"Start"`
	Stop                xmlDateTime             `xml:"Stop"`
	Resume              xmlDateTime             `xml:"Resume"`
	Units               float64                 `xml:"Units"`
	Work                string                  `xml:"Work"`
	WorkContour         int                     `xml:"WorkContour"`
	BCWS                float64                 `xml:"BCWS"`
	BCWP                float64                 `xml:"BCWP"`
	CreationDate        xmlDateTime             `xml:"CreationDate"`
	ExtendedAttribute   []xmlExtendedAttribute  `xml:"ExtendedAttribute"`
	Baseline            []xmlAssignmentBaseline `xml:"Baseline"`
	Timephased          []xmlTimephasedData     `xml:"TimephasedData"`
}

type xmlAssignmentBaseline struct {
	Number int         `xml:"Number"`
	Start  xmlDateTime `xml:"Start"`
	Finish xmlDateTime `xml:"Finish"`
	Work   string      `xml:"Work"`
	Cost   float64     `xml:"Cost"`
}

func readAssignments(xa *xmlAssignments, scale durationScale, defaultUnits project.TimeUnit) []*project.Assignment {
	if xa == nil {
		return nil
	}

	assignments := make([]*project.Assignment, 0, len(xa.Assignment))
	for _, x := range xa.Assignment {
		if x.ResourceUID == nil {
			continue
		}
		// -65535 marks an assignment with no resource; the model, like the
		// MPP reader, uses 0.
		resourceID := *x.ResourceUID
		if resourceID < 0 {
			resourceID = 0
		}
		a := &project.Assignment{
			UniqueID:         x.UID,
			TaskUniqueID:     x.TaskUID,
			ResourceUniqueID: resourceID,
			Units:            x.Units * 100,
			Notes:            x.Notes,

			GUID:                x.GUID,
			PercentWorkComplete: x.PercentWorkComplete,
			ActualCost:          x.ActualCost,
			ActualStart:         x.ActualStart.Time,
			ActualFinish:        x.ActualFinish.Time,
			ACWP:                x.ACWP,
			BCWS:                x.BCWS,
			BCWP:                x.BCWP,
			Confirmed:           x.Confirmed,
			Cost:                x.Cost,
			CostRateTable:       x.CostRateTable,
			Hyperlink:           x.Hyperlink,
			HyperlinkAddress:    x.HyperlinkAddress,
			HyperlinkSubAddress: x.HyperlinkSubAddress,
			OvertimeCost:        x.OvertimeCost,
			RemainingCost:       x.RemainingCost,
			ResponsePending:     x.ResponsePending,
			Stop:                x.Stop.Time,
			Resume:              x.Resume.Time,
			WorkContour:         workContourName(x.WorkContour),
			Created:             x.CreationDate.Time,
		}
		if x.Delay != nil {
			a.Delay = scale.tenthsToDuration(*x.Delay, project.Days)
		}
		if x.LevelingDelay != nil {
			a.LevelingDelay = scale.tenthsToDuration(*x.LevelingDelay, mspdiDurationUnit(x.LevelingDelayFormat, project.Days))
		}
		for _, w := range []struct {
			dst *project.Duration
			src string
		}{
			{&a.ActualWork, x.ActualWork}, {&a.ActualOvertimeWork, x.ActualOvertimeWork}, {&a.OvertimeWork, x.OvertimeWork},
			{&a.RegularWork, x.RegularWork}, {&a.RemainingWork, x.RemainingWork},
		} {
			if d, ok := parseDuration(scale, w.src, project.Hours); ok {
				*w.dst = d
			}
		}
		for _, attr := range x.ExtendedAttribute {
			a.CustomFields = applyExtendedAttribute(a.CustomFields, assignmentCustomFields, scale, defaultUnits, attr.FieldID, attr.Value, attr.DurationFormat)
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
