package mspdi

import (
	"bytes"
	"path/filepath"
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
	if !bytes.Contains(output.Bytes(), []byte("<Duration>PT86400S</Duration>")) {
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
