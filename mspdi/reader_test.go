// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import (
	"strings"
	"testing"
	"time"

	"github.com/tintoser/mppgo/project"
)

// sampleMSPDI is a small, hand-built MSPDI document exercising every area
// this reader covers. There is no MSPDI equivalent of the real MPP
// fixtures the mpp package tests against (see mpp/reader_test.go) — MSPDI
// is a published, documented format, not one reverse-engineered from a
// user's own file — so this synthetic document is the primary regression
// coverage for the whole package, not a supplement to it.
const sampleMSPDI = `<?xml version="1.0" encoding="UTF-8"?>
<Project>
  <Title>Sample Project</Title>
  <Subject>A test document</Subject>
  <Author>Author Name</Author>
  <Manager>Manager Name</Manager>
  <Company>Acme Co</Company>
  <Category>Testing</Category>
  <StartDate>2026-05-04T08:00:00</StartDate>
  <FinishDate>2026-05-15T17:00:00</FinishDate>
  <StatusDate>2026-05-10T08:00:00</StatusDate>
  <CalendarUID>1</CalendarUID>
  <MinutesPerDay>480</MinutesPerDay>
  <MinutesPerWeek>2400</MinutesPerWeek>
  <DaysPerMonth>20</DaysPerMonth>
  <DurationFormat>7</DurationFormat>
  <Calendars>
    <Calendar>
      <UID>1</UID>
      <Name>Standard</Name>
      <WeekDays>
        <WeekDay>
          <DayType>1</DayType>
          <DayWorking>0</DayWorking>
        </WeekDay>
        <WeekDay>
          <DayType>2</DayType>
          <DayWorking>1</DayWorking>
          <WorkingTimes>
            <WorkingTime>
              <FromTime>09:00:00</FromTime>
              <ToTime>17:00:00</ToTime>
            </WorkingTime>
          </WorkingTimes>
        </WeekDay>
      </WeekDays>
      <Exceptions>
        <Exception>
          <Name>Holiday</Name>
          <TimePeriod>
            <FromDate>2026-05-04T00:00:00</FromDate>
            <ToDate>2026-05-04T00:00:00</ToDate>
          </TimePeriod>
          <DayWorking>0</DayWorking>
        </Exception>
      </Exceptions>
    </Calendar>
    <Calendar>
      <UID>2</UID>
      <Name>Derived</Name>
      <BaseCalendarUID>1</BaseCalendarUID>
    </Calendar>
  </Calendars>
  <Tasks>
    <Task>
      <UID>1</UID>
      <ID>1</ID>
      <Name>Phase 1</Name>
      <OutlineLevel>1</OutlineLevel>
      <Summary>1</Summary>
      <Start>2026-05-04T08:00:00</Start>
      <Finish>2026-05-15T17:00:00</Finish>
    </Task>
    <Task>
      <UID>2</UID>
      <ID>2</ID>
      <Name>Design</Name>
      <OutlineLevel>2</OutlineLevel>
      <Type>1</Type>
      <Priority>500</Priority>
      <Start>2026-05-04T08:00:00</Start>
      <Finish>2026-05-06T17:00:00</Finish>
      <Duration>PT24H0M0S</Duration>
      <DurationFormat>7</DurationFormat>
      <Work>PT16H0M0S</Work>
      <PercentComplete>50</PercentComplete>
      <Notes>Design notes here</Notes>
      <FreeSlack>0</FreeSlack>
      <ExtendedAttribute>
        <FieldID>51</FieldID>
        <Value>Custom text value</Value>
      </ExtendedAttribute>
      <ExtendedAttribute>
        <FieldID>72</FieldID>
        <Value>1</Value>
      </ExtendedAttribute>
      <Baseline>
        <Number>0</Number>
        <Start>2026-05-04T08:00:00</Start>
        <Finish>2026-05-06T17:00:00</Finish>
        <Duration>PT24H0M0S</Duration>
        <DurationFormat>7</DurationFormat>
        <Work>PT16H0M0S</Work>
        <Cost>500</Cost>
      </Baseline>
    </Task>
    <Task>
      <UID>3</UID>
      <ID>3</ID>
      <Name>Build</Name>
      <OutlineLevel>2</OutlineLevel>
      <Milestone>1</Milestone>
      <Active>0</Active>
      <Start>2026-05-07T08:00:00</Start>
      <Finish>2026-05-07T08:00:00</Finish>
      <PredecessorLink>
        <PredecessorUID>2</PredecessorUID>
        <Type>1</Type>
        <LinkLag>0</LinkLag>
      </PredecessorLink>
    </Task>
  </Tasks>
  <Resources>
    <Resource>
      <UID>1</UID>
      <ID>1</ID>
      <Name>Alice</Name>
      <Type>1</Type>
      <Initials>A</Initials>
      <MaxUnits>1</MaxUnits>
      <StandardRate>50</StandardRate>
      <Notes>Resource notes</Notes>
    </Resource>
    <Resource>
      <UID>2</UID>
      <ID>2</ID>
      <Name>Concrete</Name>
      <Type>0</Type>
    </Resource>
  </Resources>
  <Assignments>
    <Assignment>
      <UID>1</UID>
      <TaskUID>2</TaskUID>
      <ResourceUID>1</ResourceUID>
      <Units>1</Units>
      <Work>PT16H0M0S</Work>
      <Start>2026-05-04T08:00:00</Start>
      <Finish>2026-05-06T17:00:00</Finish>
    </Assignment>
  </Assignments>
</Project>`

func readSample(t *testing.T) *project.File {
	t.Helper()
	pf, err := Read(strings.NewReader(sampleMSPDI))
	if err != nil {
		t.Fatalf("Read: %v", err)
	}
	return pf
}

func TestReadProperties(t *testing.T) {
	pf := readSample(t)
	p := pf.Properties

	if p.Name != "Sample Project" {
		t.Errorf("Name = %q, want %q", p.Name, "Sample Project")
	}
	if p.Author != "Author Name" || p.Manager != "Manager Name" || p.Company != "Acme Co" {
		t.Errorf("Author/Manager/Company = %q/%q/%q, want Author Name/Manager Name/Acme Co", p.Author, p.Manager, p.Company)
	}
	wantStart := time.Date(2026, 5, 4, 8, 0, 0, 0, time.UTC)
	if !p.StartDate.Equal(wantStart) {
		t.Errorf("StartDate = %v, want %v", p.StartDate, wantStart)
	}
	if p.MinutesPerDay != 480 || p.MinutesPerWeek != 2400 || p.DaysPerMonth != 20 {
		t.Errorf("MinutesPerDay/Week/DaysPerMonth = %d/%d/%d, want 480/2400/20", p.MinutesPerDay, p.MinutesPerWeek, p.DaysPerMonth)
	}
	if pf.DefaultCalendar == nil || pf.DefaultCalendar.Name != "Standard" {
		t.Errorf("DefaultCalendar = %v, want the Standard calendar", pf.DefaultCalendar)
	}
}

func TestReadCalendars(t *testing.T) {
	pf := readSample(t)
	if len(pf.Calendars) != 2 {
		t.Fatalf("got %d calendars, want 2", len(pf.Calendars))
	}

	std := pf.CalendarByID(1)
	if std == nil {
		t.Fatal("calendar 1 not found")
	}
	if std.IsWorkingDay(time.Sunday) {
		t.Error("Sunday should be non-working per the WeekDay entry")
	}
	if !std.IsWorkingDay(time.Monday) {
		t.Error("Monday should be working per the WeekDay entry")
	}
	hours := std.HoursFor(time.Monday)
	if len(hours) != 1 || hours[0].Start != 9*time.Hour || hours[0].End != 17*time.Hour {
		t.Errorf("HoursFor(Monday) = %v, want 09:00-17:00", hours)
	}
	if std.WorkingOn(time.Date(2026, 5, 4, 0, 0, 0, 0, time.UTC)) {
		t.Error("2026-05-04 should be a non-working holiday exception")
	}

	derived := pf.CalendarByID(2)
	if derived == nil {
		t.Fatal("calendar 2 not found")
	}
	if derived.Parent != std {
		t.Errorf("derived calendar's Parent = %v, want the Standard calendar", derived.Parent)
	}
}

func TestReadTaskHierarchy(t *testing.T) {
	pf := readSample(t)
	if len(pf.Tasks) != 3 {
		t.Fatalf("got %d tasks, want 3", len(pf.Tasks))
	}

	phase := pf.TaskByID(1)
	design := pf.TaskByID(2)
	build := pf.TaskByID(3)
	if phase == nil || design == nil || build == nil {
		t.Fatal("expected all three tasks to resolve by unique ID")
	}

	if design.ParentUniqueID != 1 || build.ParentUniqueID != 1 {
		t.Errorf("ParentUniqueID = %d/%d, want 1/1", design.ParentUniqueID, build.ParentUniqueID)
	}
	if !phase.Summary {
		t.Error("Phase 1 should be a summary task (it has children)")
	}
	if design.Summary {
		t.Error("Design should not be a summary task")
	}
}

func TestReadTaskFields(t *testing.T) {
	pf := readSample(t)
	design := pf.TaskByID(2)

	// PT24H0M0S is 24 elapsed hours; at 480 minutes (8 hours) per working
	// day that's 3 working days — matching the task's own Start (May 4
	// 08:00) to Finish (May 6 17:00) span.
	if design.Duration.Amount != 3 || design.Duration.Units != project.Days {
		t.Errorf("Duration = %v %v, want 3 Days", design.Duration.Amount, design.Duration.Units)
	}
	if design.Work.Amount != 16 || design.Work.Units != project.Hours {
		t.Errorf("Work = %v %v, want 16 Hours", design.Work.Amount, design.Work.Units)
	}
	if design.PercentComplete != 50 {
		t.Errorf("PercentComplete = %v, want 50", design.PercentComplete)
	}
	if design.Notes != "Design notes here" {
		t.Errorf("Notes = %q, want %q", design.Notes, "Design notes here")
	}
	if design.Type != project.FixedDuration {
		t.Errorf("Type = %v, want FixedDuration", design.Type)
	}

	build := pf.TaskByID(3)
	if !build.Milestone {
		t.Error("Build should be a milestone")
	}
	if !build.Inactive {
		t.Error("Build has Active=0 and should be Inactive")
	}
	// Design has no Active element at all; absence must default to active.
	if design.Inactive {
		t.Error("Design has no Active element and should default to active (not inactive)")
	}
}

func TestReadTaskCustomFields(t *testing.T) {
	pf := readSample(t)
	design := pf.TaskByID(2)

	if design.CustomFields["Text1"] != "Custom text value" {
		t.Errorf("CustomFields[Text1] = %v, want %q", design.CustomFields["Text1"], "Custom text value")
	}
	if design.CustomFields["Flag1"] != true {
		t.Errorf("CustomFields[Flag1] = %v, want true", design.CustomFields["Flag1"])
	}
}

func TestReadTaskBaseline(t *testing.T) {
	pf := readSample(t)
	design := pf.TaskByID(2)

	if design.Baseline == nil {
		t.Fatal("expected a primary baseline")
	}
	if design.Baseline.Cost != 500 {
		t.Errorf("Baseline.Cost = %v, want 500", design.Baseline.Cost)
	}
	if design.Baseline.Work.Amount != 16 {
		t.Errorf("Baseline.Work = %v, want 16", design.Baseline.Work.Amount)
	}
}

func TestReadPredecessors(t *testing.T) {
	pf := readSample(t)
	build := pf.TaskByID(3)

	if len(build.Predecessors) != 1 {
		t.Fatalf("got %d predecessors, want 1", len(build.Predecessors))
	}
	r := build.Predecessors[0]
	if r.PredecessorUniqueID != 2 || r.SuccessorUniqueID != 3 {
		t.Errorf("relation = %d -> %d, want 2 -> 3", r.PredecessorUniqueID, r.SuccessorUniqueID)
	}
	if r.Type != project.FinishStart {
		t.Errorf("Type = %v, want FinishStart", r.Type)
	}
}

func TestReadResources(t *testing.T) {
	pf := readSample(t)
	if len(pf.Resources) != 2 {
		t.Fatalf("got %d resources, want 2", len(pf.Resources))
	}

	alice := pf.ResourceByID(1)
	if alice == nil {
		t.Fatal("resource 1 not found")
	}
	if alice.Type != project.WorkResource {
		t.Errorf("Alice's Type = %v, want WorkResource", alice.Type)
	}
	if alice.MaxUnits != 100 {
		t.Errorf("MaxUnits = %v, want 100 (1.0 scaled to a percentage)", alice.MaxUnits)
	}
	if alice.Notes != "Resource notes" {
		t.Errorf("Notes = %q, want %q", alice.Notes, "Resource notes")
	}

	concrete := pf.ResourceByID(2)
	if concrete == nil {
		t.Fatal("resource 2 not found")
	}
	if concrete.Type != project.MaterialResource {
		t.Errorf("Concrete's Type = %v, want MaterialResource", concrete.Type)
	}
}

func TestReadAssignments(t *testing.T) {
	pf := readSample(t)
	if len(pf.Assignments) != 1 {
		t.Fatalf("got %d assignments, want 1", len(pf.Assignments))
	}
	a := pf.Assignments[0]
	if a.TaskUniqueID != 2 || a.ResourceUniqueID != 1 {
		t.Errorf("assignment links task=%d resource=%d, want 2/1", a.TaskUniqueID, a.ResourceUniqueID)
	}
	if a.Units != 100 {
		t.Errorf("Units = %v, want 100", a.Units)
	}
	if a.Work.Amount != 16 {
		t.Errorf("Work = %v, want 16", a.Work.Amount)
	}
}

func TestReadRejectsGarbage(t *testing.T) {
	if _, err := Read(strings.NewReader("not xml at all")); err == nil {
		t.Error("expected an error reading non-XML input")
	}
}
