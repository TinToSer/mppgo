// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

// Package mspdi reads Microsoft Project Data Interchange files — the XML
// interchange format Microsoft publishes and documents (unlike the MPP
// binary format the mpp package reverse-engineers). It targets the same
// format-agnostic project.File model the mpp package does, covering the
// same core schedule: project properties, calendars, tasks, resources,
// assignments, dependencies, notes, baselines and custom fields.
//
// Scope is read-only, and deliberately narrower than the mpp package in a
// few places where MSPDI itself supports something considerably more
// complex to do justice to in a first pass: work weeks (a named,
// date-ranged weekly pattern distinct from a plain calendar exception),
// resource cost rate tables and availability tables, Outline Code fields
// (MSPDI resolves these through a separate global lookup-table block this
// reader does not parse), and timephased data are all not yet read.
package mspdi

import (
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/tintoser/mppgo/project"
)

// The xml structs in this package list their fields in the element order
// of Microsoft's MSPDI schema, whose types are xsd:sequence: the writer
// emits fields in declaration order, and Project's XML import validates
// against that schema. The order makes no difference when reading.
type xmlProjectRoot struct {
	XMLName                xml.Name                      `xml:"Project"`
	XMLNS                  string                        `xml:"xmlns,attr,omitempty"`
	Name                   string                        `xml:"Name"`
	GUID                   string                        `xml:"GUID,omitempty"`
	Title                  string                        `xml:"Title"`
	Subject                string                        `xml:"Subject"`
	Category               string                        `xml:"Category"`
	Company                string                        `xml:"Company"`
	Manager                string                        `xml:"Manager"`
	Author                 string                        `xml:"Author"`
	CreationDate           xmlDateTime                   `xml:"CreationDate"`
	Revision               int                           `xml:"Revision,omitempty"`
	LastSaved              xmlDateTime                   `xml:"LastSaved"`
	ScheduleFromStart      *bool                         `xml:"ScheduleFromStart"`
	StartDate              xmlDateTime                   `xml:"StartDate"`
	FinishDate             xmlDateTime                   `xml:"FinishDate"`
	FYStartDate            int                           `xml:"FYStartDate,omitempty"`
	CriticalSlackLimit     *int                          `xml:"CriticalSlackLimit"`
	CurrencyDigits         *int                          `xml:"CurrencyDigits"`
	CurrencySymbol         string                        `xml:"CurrencySymbol,omitempty"`
	CurrencyCode           string                        `xml:"CurrencyCode,omitempty"`
	CurrencySymbolPosition *int                          `xml:"CurrencySymbolPosition"`
	CalendarUID            *int                          `xml:"CalendarUID"`
	BaselineCalendar       string                        `xml:"BaselineCalendar,omitempty"`
	DefaultStartTime       xmlTime                       `xml:"DefaultStartTime"`
	DefaultFinishTime      xmlTime                       `xml:"DefaultFinishTime"`
	MinutesPerDay          int                           `xml:"MinutesPerDay"`
	MinutesPerWeek         int                           `xml:"MinutesPerWeek"`
	DaysPerMonth           int                           `xml:"DaysPerMonth"`
	DefaultTaskType        *int                          `xml:"DefaultTaskType"`
	DefaultStandardRate    float64                       `xml:"DefaultStandardRate,omitempty"`
	DefaultOvertimeRate    float64                       `xml:"DefaultOvertimeRate,omitempty"`
	DurationFormat         *int                          `xml:"DurationFormat"`
	WorkFormat             *int                          `xml:"WorkFormat"`
	EditableActualCosts    bool                          `xml:"EditableActualCosts"`
	HonorConstraints       bool                          `xml:"HonorConstraints"`
	MultipleCriticalPaths  bool                          `xml:"MultipleCriticalPaths"`
	SplitsInProgressTasks  bool                          `xml:"SplitsInProgressTasks"`
	TaskUpdatesResource    bool                          `xml:"TaskUpdatesResource"`
	FiscalYearStart        bool                          `xml:"FiscalYearStart"`
	WeekStartDay           *int                          `xml:"WeekStartDay"`
	StatusDate             xmlDateTime                   `xml:"StatusDate"`
	NewTasksAreManual      bool                          `xml:"NewTasksAreManual"`
	ExtendedAttributes     *xmlProjectExtendedAttributes `xml:"ExtendedAttributes"`
	Calendars              *xmlCalendars                 `xml:"Calendars"`
	Tasks                  *xmlTasks                     `xml:"Tasks"`
	Resources              *xmlResources                 `xml:"Resources"`
	Assignments            *xmlAssignments               `xml:"Assignments"`
}

// ReadFile opens and parses an MSPDI (Project XML) file from disk.
func ReadFile(path string) (*project.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Read(f)
}

// Read parses an MSPDI document from r.
func Read(r io.Reader) (*project.File, error) {
	var root xmlProjectRoot
	dec := xml.NewDecoder(r)
	// MSPDI files commonly declare windows-1252 even though every sample
	// this reader has been tested against is plain ASCII/UTF-8 in
	// practice; a permissive charset reader costs nothing when that holds
	// and avoids a hard failure when it doesn't.
	dec.CharsetReader = permissiveCharsetReader
	if err := dec.Decode(&root); err != nil {
		return nil, fmt.Errorf("mspdi: %w", err)
	}

	pf := project.New()
	pf.Properties.Name = firstNonEmpty(root.Title, root.Name)
	pf.Properties.Subject = root.Subject
	pf.Properties.Author = root.Author
	pf.Properties.Manager = root.Manager
	pf.Properties.Company = root.Company
	pf.Properties.Category = root.Category
	if root.StartDate.Valid {
		pf.Properties.StartDate = root.StartDate.Time
	}
	if root.FinishDate.Valid {
		pf.Properties.FinishDate = root.FinishDate.Time
	}
	if root.StatusDate.Valid {
		pf.Properties.StatusDate = root.StatusDate.Time
	}
	pf.Properties.MinutesPerDay = root.MinutesPerDay
	pf.Properties.MinutesPerWeek = root.MinutesPerWeek
	pf.Properties.DaysPerMonth = root.DaysPerMonth

	scale := newDurationScale(pf.Properties)
	defaultUnits := project.Hours
	if root.DurationFormat != nil {
		defaultUnits = mspdiDurationUnit(*root.DurationFormat, project.Hours)
	}
	readProperties(pf.Properties, &root, defaultUnits)

	for _, c := range readCalendars(root.Calendars) {
		pf.AddCalendar(c)
	}
	if root.CalendarUID != nil {
		if cal := pf.CalendarByID(*root.CalendarUID); cal != nil {
			pf.DefaultCalendar = cal
			pf.Properties.DefaultCalendarName = cal.Name
		}
	}

	for _, r := range readResources(root.Resources, scale, defaultUnits) {
		pf.AddResource(r)
	}
	tasks, summary := readTasks(root.Tasks, scale, defaultUnits)
	pf.ProjectSummaryTask = summary
	for _, t := range tasks {
		pf.AddTask(t)
	}
	readPredecessors(pf, root.Tasks, scale)
	for _, a := range readAssignments(root.Assignments, scale, defaultUnits) {
		pf.AddAssignment(a)
	}
	readTimephased(pf, root.Assignments, scale)
	readAliases(pf, root.ExtendedAttributes)

	return pf, nil
}

// readProperties maps the project-level settings beyond the core ones.
func readProperties(p *project.Properties, root *xmlProjectRoot, defaultUnits project.TimeUnit) {
	p.GUID = root.GUID
	p.CreationDate = root.CreationDate.Time
	p.Revision = root.Revision
	p.LastSaved = root.LastSaved.Time
	p.ScheduleFromStart = root.ScheduleFromStart == nil || *root.ScheduleFromStart
	p.FiscalYearStartMonth = root.FYStartDate
	p.CriticalSlackLimit = project.Duration{Amount: float64(intOrZero(root.CriticalSlackLimit)), Units: project.Days}
	p.CurrencyDigits = intOrZero(root.CurrencyDigits)
	p.CurrencySymbol = root.CurrencySymbol
	p.CurrencyCode = root.CurrencyCode
	p.CurrencySymbolPosition = currencySymbolPositionName(intOrZero(root.CurrencySymbolPosition))
	p.BaselineCalendarName = root.BaselineCalendar
	p.DefaultStartTime = root.DefaultStartTime.Offset
	p.DefaultEndTime = root.DefaultFinishTime.Offset
	p.DefaultTaskType = taskType(intOrZero(root.DefaultTaskType))
	p.DefaultStandardRate = root.DefaultStandardRate
	p.DefaultOvertimeRate = root.DefaultOvertimeRate
	p.DefaultDurationUnits = defaultUnits
	p.DefaultWorkUnits = project.Hours
	if root.WorkFormat != nil {
		p.DefaultWorkUnits = mspdiDurationUnit(*root.WorkFormat, project.Hours)
	}
	p.EditableActualCosts = root.EditableActualCosts
	p.HonorConstraints = root.HonorConstraints
	p.MultipleCriticalPaths = root.MultipleCriticalPaths
	p.SplitInProgressTasks = root.SplitsInProgressTasks
	p.TaskUpdatesResource = root.TaskUpdatesResource
	p.FiscalYearStart = root.FiscalYearStart
	p.WeekStartDay = time.Weekday(intOrZero(root.WeekStartDay) % 7)
	p.NewTasksAreManual = root.NewTasksAreManual
}

var currencySymbolPositions = [...]string{"Before", "After", "Before with space", "After with space"}

func currencySymbolPositionName(code int) string {
	if code >= 0 && code < len(currencySymbolPositions) {
		return currencySymbolPositions[code]
	}
	return "Before"
}

func currencySymbolPositionCode(name string) int {
	for i, n := range currencySymbolPositions {
		if n == name {
			return i
		}
	}
	return 0
}

// MSPDI accrual codes (FixedCostAccrual, AccrueAt).
func accrueName(code int) string {
	switch code {
	case 1:
		return "Start"
	case 2:
		return "Prorated"
	case 3:
		return "End"
	}
	return ""
}

func accrueCode(name string) int {
	switch name {
	case "Start":
		return 1
	case "End":
		return 3
	}
	return 2
}

var workContours = [...]string{"Flat", "Back Loaded", "Front Loaded", "Double Peak", "Early Peak", "Late Peak", "Bell", "Turtle", "Contoured"}

func workContourName(code int) string {
	if code >= 0 && code < len(workContours) {
		return workContours[code]
	}
	return "Flat"
}

func workContourCode(name string) int {
	for i, n := range workContours {
		if n == name {
			return i
		}
	}
	return 0
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
