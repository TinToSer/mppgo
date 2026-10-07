package mspdi

import (
	"bytes"
	"math"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/tintoser/mppgo/mpp"
	"github.com/tintoser/mppgo/project"
)

// Every real MPP fixture survives MPP -> Project XML -> model: tasks,
// resources, assignments, custom fields, timephased spans, project
// settings and calendar working days. The fixtures are not committed, so
// this skips without them.
func TestFixtureXMLRoundTrip(t *testing.T) {
	paths, _ := filepath.Glob("../testdata/*.mpp")
	if len(paths) == 0 {
		t.Skip("no MPP fixtures in ../testdata")
	}
	near := func(a, b float64) bool { return math.Abs(a-b) < 1e-6 }
	durationEqual := func(a, b project.Duration) bool {
		return (a.Amount == 0 && b.Amount == 0) || (near(a.Amount, b.Amount) && a.Units == b.Units)
	}
	for _, path := range paths {
		t.Run(filepath.Base(path), func(t *testing.T) {
			original, err := mpp.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			var xml bytes.Buffer
			if err := Write(&xml, original); err != nil {
				t.Fatal(err)
			}
			result, err := Read(bytes.NewReader(xml.Bytes()))
			if err != nil {
				t.Fatal(err)
			}

			for _, a := range original.Tasks {
				b := result.TaskByID(a.UniqueID)
				switch {
				case b == nil:
					t.Fatalf("task %d lost", a.UniqueID)
				case !a.Start.Equal(b.Start) || !a.Finish.Equal(b.Finish) || !durationEqual(a.Duration, b.Duration):
					t.Errorf("task %d schedule changed", a.UniqueID)
				case a.Critical != b.Critical || !durationEqual(a.TotalSlack, b.TotalSlack) || a.GUID != b.GUID ||
					!a.Stop.Equal(b.Stop) || !a.Resume.Equal(b.Resume) || a.Manual != b.Manual || a.EarnedValueMethod != b.EarnedValueMethod:
					t.Errorf("task %d extended fields changed", a.UniqueID)
				case !reflect.DeepEqual(a.CustomFields, b.CustomFields):
					t.Errorf("task %d custom fields: %v vs %v", a.UniqueID, a.CustomFields, b.CustomFields)
				}
			}
			if result.ProjectSummaryTask == nil {
				t.Error("project summary task lost")
			}
			for _, a := range original.Resources {
				b := result.ResourceByID(a.UniqueID)
				if b == nil || a.GUID != b.GUID || a.AccrueAt != b.AccrueAt || !near(a.StandardRate, b.StandardRate) ||
					!durationEqual(a.ActualWork, b.ActualWork) || !reflect.DeepEqual(a.CustomFields, b.CustomFields) {
					t.Errorf("resource %d changed", a.UniqueID)
				}
			}
			byID := make(map[int]*project.Assignment)
			for _, a := range result.Assignments {
				byID[a.UniqueID] = a
			}
			for _, a := range original.Assignments {
				b := byID[a.UniqueID]
				if b == nil || a.ResourceUniqueID != b.ResourceUniqueID || !durationEqual(a.ActualWork, b.ActualWork) ||
					!durationEqual(a.RemainingWork, b.RemainingWork) || !a.ActualStart.Equal(b.ActualStart) ||
					a.WorkContour != b.WorkContour || len(a.TimephasedWork) != len(b.TimephasedWork) ||
					len(a.TimephasedActualWork) != len(b.TimephasedActualWork) {
					t.Errorf("assignment %d changed", a.UniqueID)
				}
			}
			pa, pb := original.Properties, result.Properties
			if pa.GUID != pb.GUID || pa.CurrencySymbol != pb.CurrencySymbol || pa.CurrencySymbolPosition != pb.CurrencySymbolPosition ||
				pa.WeekStartDay != pb.WeekStartDay || pa.DefaultStartTime != pb.DefaultStartTime || !pa.CreationDate.Equal(pb.CreationDate) {
				t.Error("project properties changed")
			}
			for _, a := range original.Calendars {
				b := result.CalendarByID(a.UniqueID)
				if b == nil {
					t.Fatalf("calendar %d lost", a.UniqueID)
				}
				for d := time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC); d.Year() < 2027; d = d.AddDate(0, 0, 1) {
					if a.WorkingOn(d) != b.WorkingOn(d) {
						t.Errorf("calendar %q differs on %s", a.Name, d.Format("2006-01-02"))
						break
					}
				}
			}
		})
	}
}
