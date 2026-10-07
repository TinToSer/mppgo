package mspdi

import (
	"bytes"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/tintoser/mppgo/project"
)

func TestWriteRoundTrip(t *testing.T) {
	file := project.New()
	file.Properties.Name = "Schedule & verification"
	file.Properties.MinutesPerDay, file.Properties.MinutesPerWeek, file.Properties.DaysPerMonth = 480, 2400, 20
	calendar := project.NewCalendar()
	calendar.UniqueID, calendar.Name = 1, "Standard"
	calendar.SetWorkingDay(time.Monday, true)
	calendar.Hours[time.Monday] = []project.TimeRange{{Start: 8 * time.Hour, End: 12 * time.Hour}}
	file.AddCalendar(calendar)
	file.DefaultCalendar = calendar
	child := project.NewCalendar()
	child.UniqueID, child.Parent = 2, calendar
	file.AddCalendar(child)
	start := time.Date(2026, 10, 7, 8, 0, 0, 0, time.UTC)
	file.AddTask(&project.Task{UniqueID: 1, ID: 1, Name: "Summary", Summary: true})
	file.AddTask(&project.Task{UniqueID: 2, ID: 2, Name: "Task <two>", ParentUniqueID: 1,
		Start: start, Duration: project.Duration{Amount: 2, Units: project.Days}, PercentComplete: 25,
		Notes: "Notes & details", Priority: 700,
		CustomFields: map[string]interface{}{"Owner": "Alice", "Number1": 2.5, "Cost1": 123.45, "Flag1": true},
		Baseline:     &project.Baseline{Start: start, Cost: 100}, Baselines: map[int]*project.Baseline{3: {Cost: 90}}})
	file.CustomFieldAliases = map[int]string{taskAttributeBase | 51: "Owner"}
	file.AddResource(&project.Resource{UniqueID: 3, Name: "Engineer", Type: project.WorkResource, MaxUnits: 100})
	file.AddAssignment(&project.Assignment{UniqueID: 4, TaskUniqueID: 2, ResourceUniqueID: 3, Units: 50,
		Work: project.Duration{Amount: 8, Units: project.Hours}})
	file.AddRelation(&project.Relation{UniqueID: 1, PredecessorUniqueID: 1, SuccessorUniqueID: 2,
		Type: project.FinishStart, Lag: project.Duration{Amount: 2, Units: project.Hours}})
	var output bytes.Buffer
	if err := Write(&output, file); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte(`xmlns="http://schemas.microsoft.com/project"`)) {
		t.Fatal("missing MSPDI namespace")
	}
	result, err := Read(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	task := result.TaskByID(2)
	if task == nil || task.ParentUniqueID != 1 || task.Duration.Amount != 2 || task.Duration.Units != project.Days ||
		!task.Start.Equal(start) || task.Priority != 700 || task.CustomFields["Owner"] != "Alice" ||
		task.CustomFields["Cost1"] != 123.45 || task.Baselines[3].Cost != 90 || task.Notes != "Notes & details" {
		t.Fatalf("task fields did not round trip: %+v", task)
	}
	if result.CalendarByID(2).Parent != result.CalendarByID(1) || !result.CalendarByID(2).IsWorkingDay(time.Monday) {
		t.Fatal("calendar inheritance was lost")
	}
	if result.Assignments[0].Units != 50 || result.Resources[0].MaxUnits != 100 || result.Relations[0].Lag.Amount != 2 {
		t.Fatal("assignment percentages or dependency lag were lost")
	}
}

func TestWriteFileDoesNotOverwrite(t *testing.T) {
	path := filepath.Join(t.TempDir(), "project.xml")
	file := project.New()
	if err := WriteFile(path, file); err != nil {
		t.Fatal(err)
	}
	if err := WriteFile(path, file); err == nil {
		t.Fatal("must not overwrite an existing file")
	}
}

func TestWriteRejectsUnmappedCustomField(t *testing.T) {
	file := project.New()
	file.AddTask(&project.Task{UniqueID: 1, CustomFields: map[string]interface{}{"Unmapped": "value"}})
	var output bytes.Buffer
	if err := Write(&output, file); err == nil || output.Len() != 0 {
		t.Fatal("must reject unmapped custom fields before emitting output")
	}
}

func TestWriteRichResourceAndTimephasedRoundTrip(t *testing.T) {
	file := project.New()
	start := time.Date(2026, 10, 5, 8, 0, 0, 0, time.UTC)
	finish := start.Add(8 * time.Hour)
	file.AddTask(&project.Task{UniqueID: 1})
	resource := &project.Resource{UniqueID: 2, Type: project.WorkResource, MaxUnits: 100,
		Availability: []project.AvailabilityEntry{{Start: start, End: finish, MaxUnits: 50}},
		Baselines:    map[int]*project.Baseline{10: {Start: start, Finish: finish, Cost: 30}}}
	resource.CostRateTables[4] = []project.CostRateTableEntry{{Start: start, End: finish,
		StandardRate: 100, StandardRateUnits: project.Hours, OvertimeRate: 150, OvertimeRateUnits: project.Hours, CostPerUse: 10}}
	file.AddResource(resource)
	file.AddAssignment(&project.Assignment{UniqueID: 3, TaskUniqueID: 1, ResourceUniqueID: 2,
		TimephasedWork:         []project.TimephasedWork{{Start: start, Finish: finish, Total: project.Duration{Amount: 8, Units: project.Hours}}},
		TimephasedActualWork:   []project.TimephasedWork{{Start: start, Finish: finish, Total: project.Duration{Amount: 2, Units: project.Hours}}},
		TimephasedBaselineWork: map[int][]project.TimephasedWork{10: {{Start: start, Finish: finish, Total: project.Duration{Amount: 7, Units: project.Hours}}}},
		TimephasedBaselineCost: map[int][]project.TimephasedCost{10: {{Start: start, Finish: finish, Total: 70}}}})
	var output bytes.Buffer
	if err := Write(&output, file); err != nil {
		t.Fatal(err)
	}
	result, err := Read(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	actualResource := result.ResourceByID(2)
	if actualResource.CostRateTables[4][0].StandardRate != 100 || actualResource.CostRateTables[4][0].StandardRateUnits != project.Hours ||
		actualResource.Availability[0].MaxUnits != 50 || !actualResource.Baselines[10].Finish.Equal(finish) {
		t.Fatal("resource tables, percentages or numbered baseline dates were lost")
	}
	assignment := result.Assignments[0]
	if assignment.TimephasedWork[0].Total.Amount != 8 || assignment.TimephasedActualWork[0].Total.Amount != 2 ||
		assignment.TimephasedBaselineWork[10][0].Total.Amount != 7 || assignment.TimephasedBaselineCost[10][0].Total != 70 {
		t.Fatal("timephased series or baseline type mappings were lost")
	}
	assignment.RawTimephased = append(assignment.RawTimephased, project.TimephasedData{Type: 999, UID: 42, Start: start, Finish: finish, Unit: 1, Value: "opaque"})
	output.Reset()
	if err := Write(&output, result); err != nil {
		t.Fatal(err)
	}
	preserved, err := Read(bytes.NewReader(output.Bytes()))
	if err != nil || preserved.Assignments[0].RawTimephased[4].Value != "opaque" {
		t.Fatalf("unknown XML timephased records must be retained: %v", err)
	}
}

func TestWriteElapsedDaysAndPercentLag(t *testing.T) {
	file := project.New()
	file.Properties.MinutesPerDay, file.Properties.MinutesPerWeek, file.Properties.DaysPerMonth = 480, 2400, 20
	file.AddTask(&project.Task{UniqueID: 1, Duration: project.Duration{Amount: 1, Units: project.ElapsedDays}})
	file.AddTask(&project.Task{UniqueID: 2})
	file.AddRelation(&project.Relation{UniqueID: 1, PredecessorUniqueID: 1, SuccessorUniqueID: 2,
		Type: project.FinishStart, Lag: project.Duration{Amount: 50, Units: project.Percent}})
	var output bytes.Buffer
	if err := Write(&output, file); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte("<Duration>PT24H0M0S</Duration>")) {
		t.Error("one elapsed day must serialize as 24 hours, not an eight-hour working day")
	}
	if !bytes.Contains(output.Bytes(), []byte("<LinkLag>50</LinkLag>")) {
		t.Error("percentage lag must not be scaled into tenths of a minute")
	}
	parsed, err := Read(bytes.NewReader(output.Bytes()))
	if err != nil || parsed.TaskByID(1).Duration.Amount != 1 || parsed.Relations[0].Lag.Amount != 50 {
		t.Fatalf("elapsed/percentage values failed to round trip: %v", err)
	}
}

// MSPDI stores rates per hour whatever unit Project displays them in; the
// model holds the displayed amount, as the MPP reader produces it.
func TestWriteRateInDisplayedUnits(t *testing.T) {
	file := project.New()
	file.Properties.MinutesPerDay, file.Properties.MinutesPerWeek, file.Properties.DaysPerMonth = 480, 2400, 20
	resource := &project.Resource{UniqueID: 1, Type: project.WorkResource}
	resource.CostRateTables[0] = []project.CostRateTableEntry{{StandardRate: 400, StandardRateUnits: project.Days,
		OvertimeRate: 60, OvertimeRateUnits: project.Hours}}
	file.AddResource(resource)
	var output bytes.Buffer
	if err := Write(&output, file); err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(output.Bytes(), []byte("<StandardRate>50</StandardRate>")) {
		t.Errorf("400/day must be written as the per-hour figure 50:\n%s", output.Bytes())
	}
	result, err := Read(bytes.NewReader(output.Bytes()))
	if err != nil {
		t.Fatal(err)
	}
	entry := result.ResourceByID(1).CostRateTables[0][0]
	if entry.StandardRate != 400 || entry.StandardRateUnits != project.Days || entry.OvertimeRate != 60 {
		t.Fatalf("rate did not round trip in displayed units: %+v", entry)
	}
}

func TestWriteDurationFormat(t *testing.T) {
	scale := newDurationScale(&project.Properties{})
	for _, tc := range []struct {
		in   project.Duration
		want string
	}{
		{project.Duration{Amount: 1.5, Units: project.Hours}, "PT1H30M0S"},
		{project.Duration{Amount: 2, Units: project.Days}, "PT16H0M0S"},
		{project.Duration{Amount: 0.1, Units: project.Minutes}, "PT0H0M6S"},
		{project.Duration{Amount: -1, Units: project.Hours}, "-PT1H0M0S"},
		{project.Duration{Amount: 1.0 / 3, Units: project.Hours}, "PT0H20M0S"},
	} {
		if got := writeDuration(tc.in, scale); got != tc.want {
			t.Errorf("writeDuration(%v) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

// The writer emits elements in MSPDI schema order (an xsd:sequence), and
// does not claim calendar UID 0 for tasks or resources without their own.
func TestWriteSchemaOrderAndCalendarDefaults(t *testing.T) {
	file := project.New()
	file.Properties.StatusDate = time.Date(2026, 10, 7, 17, 0, 0, 0, time.UTC)
	file.AddTask(&project.Task{UniqueID: 1, ID: 1, Name: "Task", Created: time.Date(2026, 1, 1, 8, 0, 0, 0, time.UTC),
		RemainingCost: 1, RemainingDuration: project.Duration{Amount: 1, Units: project.Days}})
	file.AddResource(&project.Resource{UniqueID: 1, Name: "R", Type: project.CostResource})
	file.CustomFieldAliases = map[int]string{taskAttributeBase | 416: "Outline alias", taskAttributeBase | 51: "Owner"}
	var output bytes.Buffer
	if err := Write(&output, file); err != nil {
		t.Fatal(err)
	}
	xml := output.String()
	for _, pair := range [][2]string{
		{"<DurationFormat>", "<StatusDate>"},
		{"<CreateDate>", "<WBS>"},
		{"<RemainingDuration>", "<RemainingCost>"},
		{"<Notes>", "<IsCostResource>"},
	} {
		first, second := strings.Index(xml, pair[0]), strings.Index(xml, pair[1])
		if first < 0 || second < 0 || first > second {
			t.Errorf("%s must precede %s", pair[0], pair[1])
		}
	}
	if !strings.Contains(xml, "<CalendarUID>-1</CalendarUID>") || strings.Contains(xml, "<CalendarUID>0</CalendarUID>") {
		t.Error("a task without its own calendar must be written as CalendarUID -1, and no element may claim UID 0")
	}
	if strings.Contains(xml, "Outline alias") || !strings.Contains(xml, "<Alias>Owner</Alias>") {
		t.Error("only aliases for fields with a known ExtendedAttribute definition may be written")
	}
	result, err := Read(strings.NewReader(xml))
	if err != nil || result.TaskByID(1).CalendarUniqueID != 0 {
		t.Fatalf("CalendarUID -1 must read back as no task calendar: %v", err)
	}
}

func TestReadSkipsProjectSummaryTask(t *testing.T) {
	doc := `<Project xmlns="http://schemas.microsoft.com/project"><Tasks>
<Task><UID>0</UID><ID>0</ID><Name>Project</Name><OutlineLevel>0</OutlineLevel><Summary>1</Summary></Task>
<Task><UID>1</UID><ID>1</ID><Name>Phase</Name><OutlineLevel>1</OutlineLevel></Task>
<Task><UID>2</UID><ID>2</ID><Name>Work</Name><OutlineLevel>2</OutlineLevel></Task>
</Tasks></Project>`
	file, err := Read(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Tasks) != 2 || file.TaskByID(1).ParentUniqueID != 0 || file.TaskByID(2).ParentUniqueID != 1 || !file.TaskByID(1).Summary {
		t.Fatalf("project summary task must be left out like the MPP reader does: %+v", file.Tasks)
	}
}

// A recurring exception's TimePeriod spans only its first to last
// occurrence, as Project writes it; it must not read as one long holiday.
func TestReadRecurringException(t *testing.T) {
	doc := `<Project xmlns="http://schemas.microsoft.com/project"><Calendars><Calendar>
<UID>1</UID><Name>Standard</Name><IsBaseCalendar>1</IsBaseCalendar><BaseCalendarUID>-1</BaseCalendarUID>
<WeekDays>` + func() string {
		var days string
		for day := 1; day <= 7; day++ {
			working := "0"
			if day >= 2 && day <= 6 {
				working = "1"
			}
			days += "<WeekDay><DayType>" + strconv.Itoa(day) + "</DayType><DayWorking>" + working + "</DayWorking></WeekDay>"
		}
		return days
	}() + `</WeekDays><Exceptions>
<Exception><EnteredByOccurrences>1</EnteredByOccurrences><TimePeriod><FromDate>2017-01-01T00:00:00</FromDate><ToDate>2046-01-01T23:59:00</ToDate></TimePeriod>
<Occurrences>30</Occurrences><Name>Neujahr</Name><Type>2</Type><PeriodNumber>1</PeriodNumber><Month>0</Month><MonthDay>1</MonthDay><DayWorking>0</DayWorking></Exception>
<Exception><TimePeriod><FromDate>2026-01-01T00:00:00</FromDate><ToDate>2026-12-31T23:59:00</ToDate></TimePeriod>
<Occurrences>12</Occurrences><Name>Last Friday</Name><Type>5</Type><PeriodNumber>1</PeriodNumber><MonthItem>8</MonthItem><MonthPosition>4</MonthPosition><DayWorking>0</DayWorking></Exception>
<Exception><TimePeriod><FromDate>2026-04-03T00:00:00</FromDate><ToDate>2026-04-06T23:59:00</ToDate></TimePeriod>
<Occurrences>4</Occurrences><Name>Ostern</Name><Type>1</Type><PeriodNumber>1</PeriodNumber><DayWorking>0</DayWorking></Exception>
</Exceptions></Calendar></Calendars></Project>`
	file, err := Read(strings.NewReader(doc))
	if err != nil {
		t.Fatal(err)
	}
	calendar := file.CalendarByID(1)
	for day, working := range map[string]bool{
		"2030-01-01": false, "2026-03-10": true, "2026-01-30": false, "2026-01-23": true,
		"2026-04-03": false, "2026-04-06": false, "2026-04-07": true,
	} {
		date, _ := time.Parse("2006-01-02", day)
		if got := calendar.WorkingOn(date); got != working {
			t.Errorf("WorkingOn(%s) = %v, want %v", day, got, working)
		}
	}
}
