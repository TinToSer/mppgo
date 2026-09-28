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

	"github.com/tintoser/mppgo/project"
)

type xmlProjectRoot struct {
	XMLName        xml.Name        `xml:"Project"`
	Name           string          `xml:"Name"`
	Title          string          `xml:"Title"`
	Subject        string          `xml:"Subject"`
	Category       string          `xml:"Category"`
	Company        string          `xml:"Company"`
	Manager        string          `xml:"Manager"`
	Author         string          `xml:"Author"`
	StartDate      xmlDateTime     `xml:"StartDate"`
	FinishDate     xmlDateTime     `xml:"FinishDate"`
	StatusDate     xmlDateTime     `xml:"StatusDate"`
	CalendarUID    *int            `xml:"CalendarUID"`
	MinutesPerDay  int             `xml:"MinutesPerDay"`
	MinutesPerWeek int             `xml:"MinutesPerWeek"`
	DaysPerMonth   int             `xml:"DaysPerMonth"`
	DurationFormat *int            `xml:"DurationFormat"`
	Calendars      *xmlCalendars   `xml:"Calendars"`
	Tasks          *xmlTasks       `xml:"Tasks"`
	Resources      *xmlResources   `xml:"Resources"`
	Assignments    *xmlAssignments `xml:"Assignments"`
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
	for _, t := range readTasks(root.Tasks, scale, defaultUnits) {
		pf.AddTask(t)
	}
	readPredecessors(pf, root.Tasks, scale)
	for _, a := range readAssignments(root.Assignments, scale) {
		pf.AddAssignment(a)
	}

	return pf, nil
}

func firstNonEmpty(values ...string) string {
	for _, v := range values {
		if v != "" {
			return v
		}
	}
	return ""
}
