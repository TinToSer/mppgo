// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import (
	"github.com/tintoser/mppgo/project"
)

type xmlTasks struct {
	Task []xmlTask `xml:"Task"`
}

type xmlTask struct {
	UID                     int                    `xml:"UID"`
	GUID                    string                 `xml:"GUID,omitempty"`
	ID                      int                    `xml:"ID"`
	Name                    string                 `xml:"Name"`
	Active                  *bool                  `xml:"Active"`
	Manual                  bool                   `xml:"Manual"`
	Type                    int                    `xml:"Type"`
	CreateDate              xmlDateTime            `xml:"CreateDate"`
	Contact                 string                 `xml:"Contact,omitempty"`
	WBS                     string                 `xml:"WBS"`
	OutlineLevel            int                    `xml:"OutlineLevel"`
	Priority                int                    `xml:"Priority"`
	Start                   xmlDateTime            `xml:"Start"`
	Finish                  xmlDateTime            `xml:"Finish"`
	Duration                string                 `xml:"Duration"`
	ManualDuration          string                 `xml:"ManualDuration,omitempty"`
	DurationFormat          int                    `xml:"DurationFormat"`
	Work                    string                 `xml:"Work"`
	Stop                    xmlDateTime            `xml:"Stop"`
	Resume                  xmlDateTime            `xml:"Resume"`
	EffortDriven            bool                   `xml:"EffortDriven"`
	Recurring               bool                   `xml:"Recurring"`
	Estimated               bool                   `xml:"Estimated"`
	Milestone               bool                   `xml:"Milestone"`
	Summary                 bool                   `xml:"Summary"`
	Critical                bool                   `xml:"Critical"`
	IsSubproject            bool                   `xml:"IsSubproject"`
	IsSubprojectReadOnly    bool                   `xml:"IsSubprojectReadOnly"`
	SubprojectName          string                 `xml:"SubprojectName,omitempty"`
	ExternalTask            bool                   `xml:"ExternalTask"`
	EarlyStart              xmlDateTime            `xml:"EarlyStart"`
	EarlyFinish             xmlDateTime            `xml:"EarlyFinish"`
	LateStart               xmlDateTime            `xml:"LateStart"`
	LateFinish              xmlDateTime            `xml:"LateFinish"`
	FreeSlack               *int                   `xml:"FreeSlack"`
	TotalSlack              *int                   `xml:"TotalSlack"`
	StartSlack              *int                   `xml:"StartSlack"`
	FinishSlack             *int                   `xml:"FinishSlack"`
	FixedCost               float64                `xml:"FixedCost"`
	FixedCostAccrual        *int                   `xml:"FixedCostAccrual"`
	PercentComplete         float64                `xml:"PercentComplete"`
	PercentWorkComplete     float64                `xml:"PercentWorkComplete"`
	Cost                    float64                `xml:"Cost"`
	OvertimeCost            float64                `xml:"OvertimeCost"`
	OvertimeWork            string                 `xml:"OvertimeWork,omitempty"`
	ActualStart             xmlDateTime            `xml:"ActualStart"`
	ActualFinish            xmlDateTime            `xml:"ActualFinish"`
	ActualDuration          string                 `xml:"ActualDuration"`
	ActualCost              float64                `xml:"ActualCost"`
	ActualOvertimeCost      float64                `xml:"ActualOvertimeCost"`
	ActualWork              string                 `xml:"ActualWork"`
	ActualOvertimeWork      string                 `xml:"ActualOvertimeWork,omitempty"`
	RegularWork             string                 `xml:"RegularWork,omitempty"`
	RemainingDuration       string                 `xml:"RemainingDuration"`
	RemainingCost           float64                `xml:"RemainingCost"`
	RemainingWork           string                 `xml:"RemainingWork"`
	RemainingOvertimeCost   float64                `xml:"RemainingOvertimeCost"`
	RemainingOvertimeWork   string                 `xml:"RemainingOvertimeWork,omitempty"`
	ACWP                    float64                `xml:"ACWP"`
	ConstraintType          *int                   `xml:"ConstraintType"`
	CalendarUID             *int                   `xml:"CalendarUID"`
	ConstraintDate          xmlDateTime            `xml:"ConstraintDate"`
	Deadline                xmlDateTime            `xml:"Deadline"`
	LevelAssignments        bool                   `xml:"LevelAssignments"`
	LevelingCanSplit        bool                   `xml:"LevelingCanSplit"`
	LevelingDelay           *int                   `xml:"LevelingDelay"`
	LevelingDelayFormat     int                    `xml:"LevelingDelayFormat,omitempty"`
	Hyperlink               string                 `xml:"Hyperlink,omitempty"`
	HyperlinkAddress        string                 `xml:"HyperlinkAddress,omitempty"`
	HyperlinkSubAddress     string                 `xml:"HyperlinkSubAddress,omitempty"`
	IgnoreResourceCalendar  bool                   `xml:"IgnoreResourceCalendar"`
	Notes                   string                 `xml:"Notes"`
	HideBar                 bool                   `xml:"HideBar"`
	Rollup                  bool                   `xml:"Rollup"`
	BCWS                    float64                `xml:"BCWS"`
	BCWP                    float64                `xml:"BCWP"`
	PhysicalPercentComplete float64                `xml:"PhysicalPercentComplete"`
	EarnedValueMethod       int                    `xml:"EarnedValueMethod"`
	PredecessorLink         []xmlPredecessorLink   `xml:"PredecessorLink"`
	ExtendedAttribute       []xmlExtendedAttribute `xml:"ExtendedAttribute"`
	Baseline                []xmlTaskBaseline      `xml:"Baseline"`
}

type xmlPredecessorLink struct {
	PredecessorUID int  `xml:"PredecessorUID"`
	Type           *int `xml:"Type"`
	LinkLag        int  `xml:"LinkLag"`
	LagFormat      *int `xml:"LagFormat"`
}

type xmlExtendedAttribute struct {
	FieldID        string `xml:"FieldID"`
	Value          string `xml:"Value"`
	DurationFormat string `xml:"DurationFormat"`
}

type xmlTaskBaseline struct {
	Number         int         `xml:"Number"`
	Start          xmlDateTime `xml:"Start"`
	Finish         xmlDateTime `xml:"Finish"`
	Duration       string      `xml:"Duration"`
	DurationFormat int         `xml:"DurationFormat"`
	Work           string      `xml:"Work"`
	Cost           float64     `xml:"Cost"`
	FixedCost      float64     `xml:"FixedCost"`
}

// taskType maps MSPDI's Type code the same way the MPP reader does (see
// mpp/task.go's taskType) — the values are identical because both formats
// serialize the same underlying enum.
func taskType(code int) project.TaskType {
	switch code {
	case 0:
		return project.FixedUnits
	case 1:
		return project.FixedDuration
	default:
		return project.FixedWork
	}
}

// readTasks converts every xmlTask into a project.Task and derives each
// one's ParentUniqueID from the OutlineLevel sequence — MSPDI, unlike the
// MPP binary format, does not store a parent unique ID directly, only the
// flat document-order list with each task's outline depth.
func readTasks(xt *xmlTasks, scale durationScale, defaultUnits project.TimeUnit) (tasks []*project.Task, summary *project.Task) {
	if xt == nil {
		return nil, nil
	}

	tasks = make([]*project.Task, 0, len(xt.Task))
	byUID := make(map[int]*project.Task, len(xt.Task))

	// stack[i] is the most recently seen task at outline level i+1.
	var stack []*project.Task

	for _, x := range xt.Task {
		// UID 0 is the project summary task MS Project writes at outline
		// level 0. Like the MPP reader, keep it apart from Tasks: it is
		// not part of the outline.
		if x.UID == 0 {
			summary = readTask(x, scale, defaultUnits)
			continue
		}
		t := readTask(x, scale, defaultUnits)

		level := x.OutlineLevel
		if level < 1 {
			level = 1
		}
		for len(stack) >= level {
			stack = stack[:len(stack)-1]
		}
		if len(stack) > 0 {
			t.ParentUniqueID = stack[len(stack)-1].UniqueID
		}
		stack = append(stack, t)

		tasks = append(tasks, t)
		byUID[t.UniqueID] = t
	}

	for _, t := range tasks {
		if t.ParentUniqueID != 0 {
			if parent := byUID[t.ParentUniqueID]; parent != nil {
				parent.Summary = true
			}
		}
	}

	return tasks, summary
}

func readTask(x xmlTask, scale durationScale, defaultUnits project.TimeUnit) *project.Task {
	t := &project.Task{
		UniqueID:            x.UID,
		ID:                  x.ID,
		Name:                x.Name,
		WBS:                 x.WBS,
		OutlineLevel:        x.OutlineLevel,
		Type:                taskType(x.Type),
		Priority:            x.Priority,
		Milestone:           x.Milestone,
		Summary:             x.Summary,
		PercentComplete:     x.PercentComplete,
		PercentWorkComplete: x.PercentWorkComplete,
		Cost:                x.Cost,
		FixedCost:           x.FixedCost,
		ActualCost:          x.ActualCost,
		RemainingCost:       x.RemainingCost,
		Inactive:            x.Active != nil && !*x.Active,

		GUID:                    x.GUID,
		Manual:                  x.Manual,
		Contact:                 x.Contact,
		Stop:                    x.Stop.Time,
		Resume:                  x.Resume.Time,
		EffortDriven:            x.EffortDriven,
		Estimated:               x.Estimated,
		Critical:                x.Critical,
		SubprojectFile:          x.SubprojectName,
		SubprojectReadOnly:      x.IsSubprojectReadOnly,
		External:                x.ExternalTask,
		FixedCostAccrual:        accrueName(intOrZero(x.FixedCostAccrual)),
		OvertimeCost:            x.OvertimeCost,
		ActualOvertimeCost:      x.ActualOvertimeCost,
		RemainingOvertimeCost:   x.RemainingOvertimeCost,
		ACWP:                    x.ACWP,
		BCWS:                    x.BCWS,
		BCWP:                    x.BCWP,
		LevelAssignments:        x.LevelAssignments,
		LevelingCanSplit:        x.LevelingCanSplit,
		Hyperlink:               x.Hyperlink,
		HyperlinkAddress:        x.HyperlinkAddress,
		HyperlinkSubAddress:     x.HyperlinkSubAddress,
		IgnoreResourceCalendar:  x.IgnoreResourceCalendar,
		HideBar:                 x.HideBar,
		Rollup:                  x.Rollup,
		PhysicalPercentComplete: x.PhysicalPercentComplete,
		EarnedValueMethod:       earnedValueMethodName(x.EarnedValueMethod),
	}
	if x.Recurring {
		t.Recurrence = &project.RecurringTask{}
	}
	for _, w := range []struct {
		dst *project.Duration
		src string
	}{
		{&t.OvertimeWork, x.OvertimeWork}, {&t.ActualOvertimeWork, x.ActualOvertimeWork},
		{&t.RegularWork, x.RegularWork}, {&t.RemainingOvertimeWork, x.RemainingOvertimeWork},
	} {
		if d, ok := parseDuration(scale, w.src, project.Hours); ok {
			*w.dst = d
		}
	}

	if x.Start.Valid {
		t.Start = x.Start.Time
	}
	if x.Finish.Valid {
		t.Finish = x.Finish.Time
	}
	if x.EarlyStart.Valid {
		t.EarlyStart = x.EarlyStart.Time
	}
	if x.EarlyFinish.Valid {
		t.EarlyFinish = x.EarlyFinish.Time
	}
	if x.LateStart.Valid {
		t.LateStart = x.LateStart.Time
	}
	if x.LateFinish.Valid {
		t.LateFinish = x.LateFinish.Time
	}
	if x.ActualStart.Valid {
		t.ActualStart = x.ActualStart.Time
	}
	if x.ActualFinish.Valid {
		t.ActualFinish = x.ActualFinish.Time
	}
	if x.ConstraintDate.Valid {
		t.ConstraintDate = x.ConstraintDate.Time
	}
	if x.Deadline.Valid {
		t.Deadline = x.Deadline.Time
	}
	if x.CreateDate.Valid {
		t.Created = x.CreateDate.Time
	}
	if x.ConstraintType != nil {
		t.ConstraintType = project.ConstraintType(*x.ConstraintType)
	}
	// -1 is MSPDI's "no task calendar"; the model uses 0 for that.
	if x.CalendarUID != nil && *x.CalendarUID > 0 {
		t.CalendarUniqueID = *x.CalendarUID
	}

	durationUnit := mspdiDurationUnit(x.DurationFormat, defaultUnits)
	if d, ok := parseDuration(scale, x.Duration, durationUnit); ok {
		t.Duration = d
	}
	if d, ok := parseDuration(scale, x.ActualDuration, durationUnit); ok {
		t.ActualDuration = d
	}
	if d, ok := parseDuration(scale, x.RemainingDuration, durationUnit); ok {
		t.RemainingDuration = d
	}
	if w, ok := parseDuration(scale, x.Work, project.Hours); ok {
		t.Work = w
	}
	if w, ok := parseDuration(scale, x.ActualWork, project.Hours); ok {
		t.ActualWork = w
	}
	if w, ok := parseDuration(scale, x.RemainingWork, project.Hours); ok {
		t.RemainingWork = w
	}

	// Slack is stored as a plain tenths-of-a-minute integer, not an
	// xsd:duration string, sharing the task's own duration format.
	if x.FreeSlack != nil {
		t.FreeSlack = scale.tenthsToDuration(*x.FreeSlack, durationUnit)
	}
	if x.TotalSlack != nil {
		t.TotalSlack = scale.tenthsToDuration(*x.TotalSlack, durationUnit)
	}
	if x.LevelingDelay != nil {
		t.LevelingDelay = scale.tenthsToDuration(*x.LevelingDelay, mspdiDurationUnit(x.LevelingDelayFormat, project.Days))
	}
	if d, ok := parseDuration(scale, x.ManualDuration, durationUnit); ok {
		t.ManualDuration = d
	}
	if x.StartSlack != nil {
		t.StartSlack = scale.tenthsToDuration(*x.StartSlack, durationUnit)
	}
	if x.FinishSlack != nil {
		t.FinishSlack = scale.tenthsToDuration(*x.FinishSlack, durationUnit)
	}

	if x.Notes != "" {
		t.Notes = x.Notes
	}

	for _, b := range x.Baseline {
		bl := readTaskBaseline(b, scale, defaultUnits)
		if b.Number == 0 {
			t.Baseline = bl
		} else if b.Number >= 1 && b.Number <= 10 {
			if t.Baselines == nil {
				t.Baselines = make(map[int]*project.Baseline)
			}
			t.Baselines[b.Number] = bl
		}
	}

	for _, attr := range x.ExtendedAttribute {
		t.CustomFields = applyExtendedAttribute(t.CustomFields, taskCustomFields, scale, defaultUnits, attr.FieldID, attr.Value, attr.DurationFormat)
	}

	return t
}

func readTaskBaseline(b xmlTaskBaseline, scale durationScale, defaultUnits project.TimeUnit) *project.Baseline {
	bl := &project.Baseline{Cost: b.Cost, FixedCost: b.FixedCost}
	if b.Start.Valid {
		bl.Start = b.Start.Time
	}
	if b.Finish.Valid {
		bl.Finish = b.Finish.Time
	}
	unit := mspdiDurationUnit(b.DurationFormat, defaultUnits)
	if d, ok := parseDuration(scale, b.Duration, unit); ok {
		bl.Duration = d
	}
	if w, ok := parseDuration(scale, b.Work, project.Hours); ok {
		bl.Work = w
	}
	return bl
}

// readPredecessors adds every task's PredecessorLink elements as relations
// to f, resolving predecessor/successor by unique ID. A link whose
// predecessor isn't a task in this file (a cross-project link) is skipped
// — this reader does not create the external-task placeholder MPXJ does.
func readPredecessors(f *project.File, xt *xmlTasks, scale durationScale) {
	if xt == nil {
		return
	}
	relationID := 0
	for _, x := range xt.Task {
		for _, link := range x.PredecessorLink {
			if f.TaskByID(link.PredecessorUID) == nil || f.TaskByID(x.UID) == nil {
				continue
			}
			relationID++
			relationType := project.FinishStart
			if link.Type != nil {
				relationType = project.RelationType(*link.Type)
			}

			lagUnit := mspdiDurationUnit(intOrZero(link.LagFormat), project.Hours)
			lag := scale.tenthsToDuration(link.LinkLag, lagUnit)

			f.AddRelation(&project.Relation{
				UniqueID:            relationID,
				PredecessorUniqueID: link.PredecessorUID,
				SuccessorUniqueID:   x.UID,
				Type:                relationType,
				Lag:                 lag,
			})
		}
	}
}

func earnedValueMethodName(code int) string {
	if code == 1 {
		return "Physical % Complete"
	}
	return "% Complete"
}

func intOrZero(p *int) int {
	if p == nil {
		return 0
	}
	return *p
}

// tenthsToDuration converts a raw tenths-of-a-minute integer (MSPDI's
// encoding for slack and lag, as opposed to the xsd:duration string used
// for most other durations) into unit.
func (s durationScale) tenthsToDuration(raw int, unit project.TimeUnit) project.Duration {
	if unit == project.Percent || unit == project.ElapsedPercent {
		return project.Duration{Amount: float64(raw), Units: unit}
	}
	return project.Duration{Amount: s.convert(float64(raw)/10, project.Minutes, unit), Units: unit}
}
