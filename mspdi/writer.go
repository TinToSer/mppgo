package mspdi

import (
	"bytes"
	"encoding/xml"
	"fmt"
	"io"
	"math"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/tintoser/mppgo/project"
)

const taskAttributeBase = 0x0B400000
const resourceAttributeBase = 0x0C400000

type xmlProjectExtendedAttributes struct {
	Attributes []xmlAttributeDefinition `xml:"ExtendedAttribute"`
}

type xmlAttributeDefinition struct {
	FieldID   int    `xml:"FieldID"`
	FieldName string `xml:"FieldName"`
	Alias     string `xml:"Alias"`
}

func Write(writer io.Writer, file *project.File) error {
	if file == nil || file.Properties == nil {
		return fmt.Errorf("mspdi: project and properties are required")
	}
	properties := file.Properties
	scale := newDurationScale(properties)
	root := xmlProjectRoot{
		XMLName: xml.Name{Space: "http://schemas.microsoft.com/project", Local: "Project"},
		XMLNS:   "http://schemas.microsoft.com/project",
		Name:    properties.Name, Title: properties.Name, Subject: properties.Subject,
		Category: properties.Category, Company: properties.Company, Manager: properties.Manager, Author: properties.Author,
		StartDate: writeDate(properties.StartDate), FinishDate: writeDate(properties.FinishDate), StatusDate: writeDate(properties.StatusDate),
		MinutesPerDay: properties.MinutesPerDay, MinutesPerWeek: properties.MinutesPerWeek, DaysPerMonth: properties.DaysPerMonth,
		DurationFormat: pointer(7), Calendars: &xmlCalendars{}, Tasks: &xmlTasks{}, Resources: &xmlResources{}, Assignments: &xmlAssignments{},
	}
	if file.DefaultCalendar != nil {
		root.CalendarUID = pointer(file.DefaultCalendar.UniqueID)
	}
	ids := make([]int, 0, len(file.CustomFieldAliases))
	for id := range file.CustomFieldAliases {
		ids = append(ids, id)
	}
	sort.Ints(ids)
	if len(ids) > 0 {
		root.ExtendedAttributes = &xmlProjectExtendedAttributes{}
		for _, id := range ids {
			definition := taskCustomFields[id&0xFFFF]
			if id&0xFFFF0000 == resourceAttributeBase {
				definition = resourceCustomFields[id&0xFFFF]
			}
			root.ExtendedAttributes.Attributes = append(root.ExtendedAttributes.Attributes,
				xmlAttributeDefinition{FieldID: id, FieldName: definition.name, Alias: file.CustomFieldAliases[id]})
		}
	}
	for _, calendar := range file.Calendars {
		if calendar == nil {
			return fmt.Errorf("mspdi: nil calendar")
		}
		root.Calendars.Calendar = append(root.Calendars.Calendar, writeCalendar(calendar))
	}
	ordered, levels, err := taskOrder(file.Tasks)
	if err != nil {
		return err
	}
	for _, task := range ordered {
		xmlTask, err := writeTask(task, levels[task.UniqueID], scale, file.CustomFieldAliases)
		if err != nil {
			return err
		}
		for _, relation := range file.Relations {
			if relation.SuccessorUniqueID == task.UniqueID {
				lag := int(math.Round(scale.convert(relation.Lag.Amount, relation.Lag.Units, project.Minutes) * 10))
				if relation.Lag.Units == project.Percent || relation.Lag.Units == project.ElapsedPercent {
					lag = int(math.Round(relation.Lag.Amount))
				}
				xmlTask.PredecessorLink = append(xmlTask.PredecessorLink, xmlPredecessorLink{
					PredecessorUID: relation.PredecessorUniqueID, Type: pointer(int(relation.Type)),
					LinkLag:   lag,
					LagFormat: pointer(writeDurationFormat(relation.Lag.Units)),
				})
			}
		}
		root.Tasks.Task = append(root.Tasks.Task, xmlTask)
	}
	for _, resource := range file.Resources {
		attributes, err := writeAttributes(resource.CustomFields, resourceAttributeBase, resourceCustomFields, file.CustomFieldAliases, scale)
		if err != nil {
			return err
		}
		kind := 1
		if resource.Type == project.MaterialResource {
			kind = 0
		}
		xmlResource := xmlResource{UID: resource.UniqueID, ID: resource.ID, Name: resource.Name, Type: kind,
			IsCostResource: resource.Type == project.CostResource, Initials: resource.Initials, Code: resource.Code,
			Group: resource.Group, EmailAddress: resource.EmailAddress, MaxUnits: resource.MaxUnits / 100,
			StandardRate: resource.StandardRate, OvertimeRate: resource.OvertimeRate, CostPerUse: resource.CostPerUse,
			Cost: resource.Cost, Work: writeDuration(resource.Work, scale), CalendarUID: pointer(resource.CalendarUniqueID),
			Notes: resource.Notes, ExtendedAttribute: attributes}
		for _, numbered := range baselines(resource.Baseline, resource.Baselines) {
			xmlResource.Baseline = append(xmlResource.Baseline, xmlResourceBaseline{
				Number: numbered.number, Start: writeDate(numbered.value.Start), Finish: writeDate(numbered.value.Finish),
				Work: writeDuration(numbered.value.Work, scale), Cost: numbered.value.Cost})
		}
		xmlResource.Rates, xmlResource.Availability = writeResourceTables(resource)
		root.Resources.Resource = append(root.Resources.Resource, xmlResource)
	}
	for _, assignment := range file.Assignments {
		xmlAssignment := xmlAssignment{UID: assignment.UniqueID, TaskUID: assignment.TaskUniqueID,
			ResourceUID: pointer(assignment.ResourceUniqueID), Units: assignment.Units / 100,
			Work: writeDuration(assignment.Work, scale), Start: writeDate(assignment.Start), Finish: writeDate(assignment.Finish), Notes: assignment.Notes}
		for _, numbered := range baselines(assignment.Baseline, assignment.Baselines) {
			xmlAssignment.Baseline = append(xmlAssignment.Baseline, xmlAssignmentBaseline{
				Number: numbered.number, Start: writeDate(numbered.value.Start), Finish: writeDate(numbered.value.Finish),
				Work: writeDuration(numbered.value.Work, scale), Cost: numbered.value.Cost})
		}
		xmlAssignment.Timephased = writeTimephased(assignment, scale)
		root.Assignments.Assignment = append(root.Assignments.Assignment, xmlAssignment)
	}
	var buffer bytes.Buffer
	buffer.WriteString(xml.Header)
	encoder := xml.NewEncoder(&buffer)
	encoder.Indent("", "  ")
	if err := encoder.Encode(root); err != nil {
		return fmt.Errorf("mspdi: encoding: %w", err)
	}
	if err := encoder.Flush(); err != nil {
		return err
	}
	count, err := writer.Write(buffer.Bytes())
	if err == nil && count != buffer.Len() {
		return io.ErrShortWrite
	}
	return err
}

func WriteFile(path string, file *project.File) error {
	var buffer bytes.Buffer
	if err := Write(&buffer, file); err != nil {
		return err
	}
	output, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_EXCL, 0600)
	if err != nil {
		return err
	}
	_, writeErr := output.Write(buffer.Bytes())
	closeErr := output.Close()
	if writeErr != nil {
		return writeErr
	}
	return closeErr
}

func writeTask(task *project.Task, level int, scale durationScale, aliases map[int]string) (xmlTask, error) {
	attributes, err := writeAttributes(task.CustomFields, taskAttributeBase, taskCustomFields, aliases, scale)
	if err != nil {
		return xmlTask{}, err
	}
	result := xmlTask{UID: task.UniqueID, ID: task.ID, Name: task.Name, Active: pointer(!task.Inactive), Type: int(task.Type),
		WBS: task.WBS, OutlineLevel: level, Priority: task.Priority, Start: writeDate(task.Start), Finish: writeDate(task.Finish),
		Duration: writeDuration(task.Duration, scale), DurationFormat: writeDurationFormat(task.Duration.Units),
		Work: writeDuration(task.Work, scale), ActualWork: writeDuration(task.ActualWork, scale), RemainingWork: writeDuration(task.RemainingWork, scale),
		Milestone: task.Milestone, Summary: task.Summary, EarlyStart: writeDate(task.EarlyStart), EarlyFinish: writeDate(task.EarlyFinish),
		LateStart: writeDate(task.LateStart), LateFinish: writeDate(task.LateFinish),
		FreeSlack:   pointer(int(math.Round(scale.convert(task.FreeSlack.Amount, task.FreeSlack.Units, project.Minutes) * 10))),
		StartSlack:  pointer(int(math.Round(scale.convert(task.StartSlack.Amount, task.StartSlack.Units, project.Minutes) * 10))),
		FinishSlack: pointer(int(math.Round(scale.convert(task.FinishSlack.Amount, task.FinishSlack.Units, project.Minutes) * 10))),
		FixedCost:   task.FixedCost, PercentComplete: task.PercentComplete, PercentWorkComplete: task.PercentWorkComplete,
		Cost: task.Cost, ActualStart: writeDate(task.ActualStart), ActualFinish: writeDate(task.ActualFinish),
		ActualDuration: writeDuration(task.ActualDuration, scale), ActualCost: task.ActualCost, RemainingCost: task.RemainingCost,
		RemainingDuration: writeDuration(task.RemainingDuration, scale), ConstraintType: pointer(int(task.ConstraintType)),
		CalendarUID: pointer(task.CalendarUniqueID), ConstraintDate: writeDate(task.ConstraintDate), Deadline: writeDate(task.Deadline),
		Notes: task.Notes, CreateDate: writeDate(task.Created), ExtendedAttribute: attributes}
	for _, numbered := range baselines(task.Baseline, task.Baselines) {
		baseline := numbered.value
		result.Baseline = append(result.Baseline, xmlTaskBaseline{Number: numbered.number,
			Start: writeDate(baseline.Start), Finish: writeDate(baseline.Finish), Duration: writeDuration(baseline.Duration, scale),
			DurationFormat: writeDurationFormat(baseline.Duration.Units), Work: writeDuration(baseline.Work, scale), Cost: baseline.Cost, FixedCost: baseline.FixedCost})
	}
	return result, nil
}

func writeCalendar(calendar *project.Calendar) xmlCalendar {
	result := xmlCalendar{UID: calendar.UniqueID, Name: calendar.Name, GUID: calendar.GUID,
		IsBaseCalendar: calendar.Parent == nil, BaseCalendarUID: pointer(-1), WeekDays: &xmlWeekDays{}, Exceptions: &xmlExceptions{}}
	if calendar.Parent != nil {
		result.BaseCalendarUID = pointer(calendar.Parent.UniqueID)
	}
	for day := time.Sunday; day <= time.Saturday; day++ {
		if calendar.DayType(day) == project.DayDefault {
			continue
		}
		result.WeekDays.WeekDay = append(result.WeekDays.WeekDay, xmlWeekDay{
			DayType: int(day) + 1, DayWorking: calendar.DayType(day) == project.DayWorking, WorkingTimes: writeRanges(calendar.Hours[day])})
	}
	for _, exception := range calendar.Exceptions {
		result.Exceptions.Exception = append(result.Exceptions.Exception, xmlException{Name: exception.Name,
			TimePeriod: &xmlTimePeriod{FromDate: writeDate(exception.FromDate), ToDate: writeDate(exception.ToDate)},
			DayWorking: exception.Working(), WorkingTimes: writeRanges(exception.Ranges)})
	}
	return result
}

func writeRanges(ranges []project.TimeRange) *xmlWorkingTimes {
	if len(ranges) == 0 {
		return nil
	}
	result := &xmlWorkingTimes{}
	for _, period := range ranges {
		result.WorkingTime = append(result.WorkingTime, xmlWorkingTime{
			FromTime: xmlTime{Offset: period.Start, Valid: true}, ToTime: xmlTime{Offset: period.End, Valid: true}})
	}
	return result
}

func writeAttributes(fields map[string]interface{}, base int, definitions map[int]customFieldDef, aliases map[int]string, scale durationScale) ([]xmlExtendedAttribute, error) {
	names := make([]string, 0, len(fields))
	for name := range fields {
		names = append(names, name)
	}
	sort.Strings(names)
	var attributes []xmlExtendedAttribute
	for _, name := range names {
		id := -1
		var definition customFieldDef
		for candidate, current := range definitions {
			if strings.EqualFold(current.name, name) || (aliases[base|candidate] != "" && strings.EqualFold(aliases[base|candidate], name)) {
				id, definition = candidate, current
				break
			}
		}
		if id < 0 {
			return nil, fmt.Errorf("mspdi: custom field %q has no supported field definition; use a known alias or Text/Number/Date/Duration/Cost/Flag slot", name)
		}
		value := fields[name]
		attribute := xmlExtendedAttribute{FieldID: strconv.Itoa(base | id)}
		switch definition.kind {
		case kindText:
			text, ok := value.(string)
			if !ok {
				return nil, fmt.Errorf("mspdi: %s requires text", name)
			}
			attribute.Value = text
		case kindDate:
			date, ok := value.(time.Time)
			if !ok {
				return nil, fmt.Errorf("mspdi: %s requires time.Time", name)
			}
			attribute.Value = date.Format("2006-01-02T15:04:05")
		case kindDuration:
			duration, ok := value.(project.Duration)
			if !ok {
				return nil, fmt.Errorf("mspdi: %s requires project.Duration", name)
			}
			attribute.Value, attribute.DurationFormat = writeDuration(duration, scale), strconv.Itoa(writeDurationFormat(duration.Units))
		case kindFlag:
			flag, ok := value.(bool)
			if !ok {
				return nil, fmt.Errorf("mspdi: %s requires boolean", name)
			}
			attribute.Value = "0"
			if flag {
				attribute.Value = "1"
			}
		default:
			number, err := strconv.ParseFloat(fmt.Sprint(value), 64)
			if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
				return nil, fmt.Errorf("mspdi: %s requires a finite number", name)
			}
			if definition.kind == kindCost {
				number *= 100
			}
			attribute.Value = strconv.FormatFloat(number, 'f', -1, 64)
		}
		attributes = append(attributes, attribute)
	}
	return attributes, nil
}

func readAliases(file *project.File, definitions *xmlProjectExtendedAttributes) {
	if definitions == nil {
		return
	}
	file.CustomFieldAliases = make(map[int]string)
	for _, attribute := range definitions.Attributes {
		if attribute.Alias != "" {
			file.CustomFieldAliases[attribute.FieldID] = attribute.Alias
		}
	}
	apply := func(fields map[string]interface{}, base int, defs map[int]customFieldDef) {
		for id, definition := range defs {
			alias := file.CustomFieldAliases[base|id]
			if value, exists := fields[definition.name]; exists && alias != "" && alias != definition.name {
				fields[alias] = value
				delete(fields, definition.name)
			}
		}
	}
	for _, task := range file.Tasks {
		apply(task.CustomFields, taskAttributeBase, taskCustomFields)
	}
	for _, resource := range file.Resources {
		apply(resource.CustomFields, resourceAttributeBase, resourceCustomFields)
	}
}

func taskOrder(tasks []*project.Task) ([]*project.Task, map[int]int, error) {
	byID := make(map[int]*project.Task)
	children := make(map[int][]*project.Task)
	var roots []*project.Task
	for _, task := range tasks {
		if task == nil || byID[task.UniqueID] != nil {
			return nil, nil, fmt.Errorf("mspdi: nil task or duplicate unique ID")
		}
		byID[task.UniqueID] = task
		if task.ParentUniqueID <= 0 {
			roots = append(roots, task)
		} else {
			children[task.ParentUniqueID] = append(children[task.ParentUniqueID], task)
		}
	}
	var ordered []*project.Task
	levels := make(map[int]int)
	var visit func(*project.Task, int) error
	visit = func(task *project.Task, level int) error {
		if level > 100 || levels[task.UniqueID] != 0 {
			return fmt.Errorf("mspdi: cyclic or excessively deep task hierarchy")
		}
		levels[task.UniqueID] = level
		ordered = append(ordered, task)
		for _, child := range children[task.UniqueID] {
			if err := visit(child, level+1); err != nil {
				return err
			}
		}
		return nil
	}
	for _, task := range roots {
		if err := visit(task, 1); err != nil {
			return nil, nil, err
		}
	}
	if len(ordered) != len(tasks) {
		return nil, nil, fmt.Errorf("mspdi: missing parent or cyclic task hierarchy")
	}
	return ordered, levels, nil
}

type numberedBaseline struct {
	number int
	value  *project.Baseline
}

func baselines(primary *project.Baseline, numbered map[int]*project.Baseline) []numberedBaseline {
	var result []numberedBaseline
	if primary != nil {
		result = append(result, numberedBaseline{number: 0, value: primary})
	}
	for index := 1; index <= 10; index++ {
		if baseline := numbered[index]; baseline != nil {
			result = append(result, numberedBaseline{number: index, value: baseline})
		}
	}
	return result
}

func pointer[Value any](value Value) *Value { return &value }

func writeDate(date time.Time) xmlDateTime { return xmlDateTime{Time: date, Valid: !date.IsZero()} }

func writeDuration(duration project.Duration, scale durationScale) string {
	seconds := scale.convert(duration.Amount, duration.Units, project.Minutes) * 60
	prefix := "PT"
	if seconds < 0 {
		prefix, seconds = "-PT", -seconds
	}
	return prefix + strconv.FormatFloat(seconds, 'f', -1, 64) + "S"
}

func writeDurationFormat(unit project.TimeUnit) int {
	return map[project.TimeUnit]int{
		project.Minutes: 3, project.ElapsedMinutes: 4, project.Hours: 5, project.ElapsedHours: 6,
		project.Days: 7, project.ElapsedDays: 8, project.Weeks: 9, project.ElapsedWeeks: 10,
		project.Months: 11, project.ElapsedMonths: 12, project.Years: 9, project.ElapsedYears: 10,
		project.Percent: 19, project.ElapsedPercent: 20,
	}[unit]
}
