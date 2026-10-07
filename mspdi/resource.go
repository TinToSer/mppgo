// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import "github.com/tintoser/mppgo/project"

type xmlResources struct {
	Resource []xmlResource `xml:"Resource"`
}

type xmlResource struct {
	UID                 int                     `xml:"UID"`
	GUID                string                  `xml:"GUID,omitempty"`
	ID                  int                     `xml:"ID"`
	Name                string                  `xml:"Name"`
	Type                int                     `xml:"Type"`
	Initials            string                  `xml:"Initials"`
	Phonetics           string                  `xml:"Phonetics,omitempty"`
	NTAccount           string                  `xml:"NTAccount,omitempty"`
	MaterialLabel       string                  `xml:"MaterialLabel,omitempty"`
	Code                string                  `xml:"Code"`
	Group               string                  `xml:"Group"`
	EmailAddress        string                  `xml:"EmailAddress"`
	Hyperlink           string                  `xml:"Hyperlink,omitempty"`
	HyperlinkAddress    string                  `xml:"HyperlinkAddress,omitempty"`
	HyperlinkSubAddress string                  `xml:"HyperlinkSubAddress,omitempty"`
	MaxUnits            float64                 `xml:"MaxUnits"`
	PeakUnits           float64                 `xml:"PeakUnits,omitempty"`
	AvailableFrom       xmlDateTime             `xml:"AvailableFrom"`
	AvailableTo         xmlDateTime             `xml:"AvailableTo"`
	Start               xmlDateTime             `xml:"Start"`
	Finish              xmlDateTime             `xml:"Finish"`
	CanLevel            bool                    `xml:"CanLevel"`
	AccrueAt            *int                    `xml:"AccrueAt"`
	Work                string                  `xml:"Work"`
	RegularWork         string                  `xml:"RegularWork,omitempty"`
	OvertimeWork        string                  `xml:"OvertimeWork,omitempty"`
	ActualWork          string                  `xml:"ActualWork,omitempty"`
	RemainingWork       string                  `xml:"RemainingWork,omitempty"`
	ActualOvertimeWork  string                  `xml:"ActualOvertimeWork,omitempty"`
	StandardRate        float64                 `xml:"StandardRate"`
	StandardRateFormat  int                     `xml:"StandardRateFormat,omitempty"`
	Cost                float64                 `xml:"Cost"`
	OvertimeRate        float64                 `xml:"OvertimeRate"`
	OvertimeRateFormat  int                     `xml:"OvertimeRateFormat,omitempty"`
	OvertimeCost        float64                 `xml:"OvertimeCost"`
	CostPerUse          float64                 `xml:"CostPerUse"`
	ActualCost          float64                 `xml:"ActualCost"`
	RemainingCost       float64                 `xml:"RemainingCost"`
	ACWP                float64                 `xml:"ACWP"`
	CalendarUID         *int                    `xml:"CalendarUID"`
	Notes               string                  `xml:"Notes"`
	BCWS                float64                 `xml:"BCWS"`
	BCWP                float64                 `xml:"BCWP"`
	IsGeneric           bool                    `xml:"IsGeneric"`
	BookingType         *int                    `xml:"BookingType"`
	CreationDate        xmlDateTime             `xml:"CreationDate"`
	IsCostResource      bool                    `xml:"IsCostResource"`
	IsBudget            bool                    `xml:"IsBudget"`
	ExtendedAttribute   []xmlExtendedAttribute  `xml:"ExtendedAttribute"`
	Baseline            []xmlResourceBaseline   `xml:"Baseline"`
	Availability        *xmlAvailabilityPeriods `xml:"AvailabilityPeriods"`
	Rates               *xmlRates               `xml:"Rates"`
}

type xmlResourceBaseline struct {
	Number int         `xml:"Number"`
	Start  xmlDateTime `xml:"Start"`
	Finish xmlDateTime `xml:"Finish"`
	Work   string      `xml:"Work"`
	Cost   float64     `xml:"Cost"`
}

// resourceType decides between MS Project's three resource kinds. The
// <Type> element only ever distinguishes Material (0) from Work (1); a
// cost resource is signalled by the separate IsCostResource flag
// overriding whatever <Type> says — the same layering MSPDI uses
// throughout (a boolean flag refining a more basic type code) rather than
// a single three-way field.
func resourceType(xmlType int, isCostResource bool) project.ResourceType {
	if isCostResource {
		return project.CostResource
	}
	if xmlType == 0 {
		return project.MaterialResource
	}
	return project.WorkResource
}

func readResources(xr *xmlResources, scale durationScale, defaultUnits project.TimeUnit) []*project.Resource {
	if xr == nil {
		return nil
	}

	resources := make([]*project.Resource, 0, len(xr.Resource))
	for _, x := range xr.Resource {
		r := &project.Resource{
			UniqueID:     x.UID,
			ID:           x.ID,
			Name:         x.Name,
			Initials:     x.Initials,
			Type:         resourceType(x.Type, x.IsCostResource),
			Group:        x.Group,
			Code:         x.Code,
			EmailAddress: x.EmailAddress,
			MaxUnits:     x.MaxUnits * 100,
			CostPerUse:   x.CostPerUse,
			Cost:         x.Cost,
			Notes:        x.Notes,

			GUID:                x.GUID,
			Phonetics:           x.Phonetics,
			NTAccount:           x.NTAccount,
			MaterialLabel:       x.MaterialLabel,
			Hyperlink:           x.Hyperlink,
			HyperlinkAddress:    x.HyperlinkAddress,
			HyperlinkSubAddress: x.HyperlinkSubAddress,
			PeakUnits:           x.PeakUnits * 100,
			AvailableFrom:       x.AvailableFrom.Time,
			AvailableTo:         x.AvailableTo.Time,
			Start:               x.Start.Time,
			Finish:              x.Finish.Time,
			CanLevel:            x.CanLevel,
			AccrueAt:            accrueName(intOrZero(x.AccrueAt)),
			OvertimeCost:        x.OvertimeCost,
			ActualCost:          x.ActualCost,
			RemainingCost:       x.RemainingCost,
			ACWP:                x.ACWP,
			BCWS:                x.BCWS,
			BCWP:                x.BCWP,
			Generic:             x.IsGeneric,
			Budget:              x.IsBudget,
			Created:             x.CreationDate.Time,
			StandardRateUnits:   readRateUnit(x.StandardRateFormat),
			OvertimeRateUnits:   readRateUnit(x.OvertimeRateFormat),
		}
		// MSPDI rates are per hour; the model holds the displayed amount.
		r.StandardRate = x.StandardRate * scale.minutesPerUnit(r.StandardRateUnits) / 60
		r.OvertimeRate = x.OvertimeRate * scale.minutesPerUnit(r.OvertimeRateUnits) / 60
		if x.BookingType != nil && *x.BookingType == 1 {
			r.BookingType = "Proposed"
		} else {
			r.BookingType = "Committed"
		}
		for _, w := range []struct {
			dst *project.Duration
			src string
		}{
			{&r.RegularWork, x.RegularWork}, {&r.OvertimeWork, x.OvertimeWork}, {&r.ActualWork, x.ActualWork},
			{&r.RemainingWork, x.RemainingWork}, {&r.ActualOvertimeWork, x.ActualOvertimeWork},
		} {
			if d, ok := parseDuration(scale, w.src, project.Hours); ok {
				*w.dst = d
			}
		}
		if x.CalendarUID != nil && *x.CalendarUID > 0 {
			r.CalendarUniqueID = *x.CalendarUID
		}
		if w, ok := parseDuration(scale, x.Work, project.Hours); ok {
			r.Work = w
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
				r.Baseline = bl
			} else if b.Number >= 1 && b.Number <= 10 {
				if r.Baselines == nil {
					r.Baselines = make(map[int]*project.Baseline)
				}
				r.Baselines[b.Number] = bl
			}
		}

		for _, attr := range x.ExtendedAttribute {
			r.CustomFields = applyExtendedAttribute(r.CustomFields, resourceCustomFields, scale, defaultUnits, attr.FieldID, attr.Value, attr.DurationFormat)
		}
		readResourceTables(r, x, scale)

		resources = append(resources, r)
	}
	return resources
}
