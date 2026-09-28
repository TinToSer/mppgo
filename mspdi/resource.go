// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import "github.com/tintoser/mppgo/project"

type xmlResources struct {
	Resource []xmlResource `xml:"Resource"`
}

type xmlResource struct {
	UID               int                    `xml:"UID"`
	ID                int                    `xml:"ID"`
	Name              string                 `xml:"Name"`
	Type              int                    `xml:"Type"`
	IsCostResource    bool                   `xml:"IsCostResource"`
	Initials          string                 `xml:"Initials"`
	Code              string                 `xml:"Code"`
	Group             string                 `xml:"Group"`
	EmailAddress      string                 `xml:"EmailAddress"`
	MaxUnits          float64                `xml:"MaxUnits"`
	StandardRate      float64                `xml:"StandardRate"`
	OvertimeRate      float64                `xml:"OvertimeRate"`
	CostPerUse        float64                `xml:"CostPerUse"`
	Cost              float64                `xml:"Cost"`
	Work              string                 `xml:"Work"`
	CalendarUID       *int                   `xml:"CalendarUID"`
	Notes             string                 `xml:"Notes"`
	ExtendedAttribute []xmlExtendedAttribute `xml:"ExtendedAttribute"`
	Baseline          []xmlResourceBaseline  `xml:"Baseline"`
}

type xmlResourceBaseline struct {
	Number int     `xml:"Number"`
	Work   string  `xml:"Work"`
	Cost   float64 `xml:"Cost"`
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
			StandardRate: x.StandardRate,
			OvertimeRate: x.OvertimeRate,
			CostPerUse:   x.CostPerUse,
			Cost:         x.Cost,
			Notes:        x.Notes,
		}
		if x.CalendarUID != nil {
			r.CalendarUniqueID = *x.CalendarUID
		}
		if w, ok := parseDuration(scale, x.Work, project.Hours); ok {
			r.Work = w
		}

		for _, b := range x.Baseline {
			bl := &project.Baseline{Cost: b.Cost}
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

		resources = append(resources, r)
	}
	return resources
}
