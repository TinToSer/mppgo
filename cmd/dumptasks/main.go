// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

// Command dumptasks reads an MPP or MSPDI (Project XML) file and prints its
// task hierarchy, schedule dates and dependencies, for manual inspection.
package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"github.com/tintoser/mppgo/mpp"
	"github.com/tintoser/mppgo/mspdi"
	"github.com/tintoser/mppgo/project"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dumptasks <file.mpp|file.xml>")
		os.Exit(2)
	}

	path := os.Args[1]
	var pf *project.File
	var err error
	if strings.EqualFold(filepath.Ext(path), ".xml") {
		pf, err = mspdi.ReadFile(path)
	} else {
		pf, err = mpp.ReadFile(path)
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	p := pf.Properties
	fmt.Printf("Project:     %s (version %d)\n", p.ApplicationName, p.ApplicationVersion)
	fmt.Printf("Schedule:    %s .. %s\n", date(p.StartDate), date(p.FinishDate))
	fmt.Printf("Status date: %s\n", date(p.StatusDate))
	fmt.Printf("Day/week:    %d min/day, %d min/week\n", p.MinutesPerDay, p.MinutesPerWeek)
	fmt.Printf("Counts:      %d tasks, %d resources, %d assignments, %d dependencies\n\n",
		len(pf.Tasks), len(pf.Resources), len(pf.Assignments), len(pf.Relations))

	for _, t := range pf.Tasks {
		describe(pf, t)
	}
}

func describe(pf *project.File, t *project.Task) {
	indent := strings.Repeat("  ", max(t.OutlineLevel-1, 0))

	name := t.Name
	if name == "" {
		name = "(unnamed)"
	}
	fmt.Printf("%s%s %s", indent, t.WBS, name)

	var tags []string
	if t.Summary {
		tags = append(tags, "summary")
	}
	if t.Milestone {
		tags = append(tags, "milestone")
	}
	if t.Inactive {
		tags = append(tags, "inactive")
	}
	if len(tags) > 0 {
		fmt.Printf("  [%s]", strings.Join(tags, ", "))
	}
	fmt.Println()

	fmt.Printf("%s    %s .. %s", indent, date(t.Start), date(t.Finish))
	if t.Duration.Amount != 0 {
		fmt.Printf("  %.1f%s", t.Duration.Amount, t.Duration.Units)
	}
	if t.PercentComplete != 0 {
		fmt.Printf("  %.0f%% complete", t.PercentComplete)
	}
	if cal := pf.TaskCalendar(t); cal != nil && cal.Name != "" {
		fmt.Printf("  calendar=%q", cal.Name)
	}
	fmt.Println()

	if b := t.Baseline; b != nil {
		fmt.Printf("%s    baseline %s .. %s", indent, date(b.Start), date(b.Finish))
		if b.Duration.Amount != 0 {
			fmt.Printf("  %.1f%s", b.Duration.Amount, b.Duration.Units)
		}
		if b.Work.Amount != 0 {
			fmt.Printf("  work=%.1f%s", b.Work.Amount, b.Work.Units)
		}
		if b.Cost != 0 {
			fmt.Printf("  cost=%.2f", b.Cost)
		}
		fmt.Println()
	}

	if len(t.CustomFields) > 0 {
		names := make([]string, 0, len(t.CustomFields))
		for name := range t.CustomFields {
			names = append(names, name)
		}
		sort.Strings(names)
		fmt.Printf("%s    fields:", indent)
		for _, name := range names {
			fmt.Printf("  %s=%v", name, t.CustomFields[name])
		}
		fmt.Println()
	}

	if t.Notes != "" {
		fmt.Printf("%s    note: %s\n", indent, strings.ReplaceAll(t.Notes, "\n", "\n"+indent+"          "))
	}

	for _, r := range t.Predecessors {
		pred := pf.TaskByID(r.PredecessorUniqueID)
		predName := "(unknown)"
		if pred != nil {
			predName = pred.Name
		}
		fmt.Printf("%s    after %q (%s", indent, predName, r.Type)
		if r.Lag.Amount != 0 {
			fmt.Printf(", lag %.1f%s", r.Lag.Amount, r.Lag.Units)
		}
		fmt.Println(")")
	}

	for _, a := range pf.Assignments {
		if a.TaskUniqueID != t.UniqueID {
			continue
		}
		res := pf.ResourceByID(a.ResourceUniqueID)
		if res == nil {
			continue
		}
		fmt.Printf("%s    resource %s (%.0f%%", indent, res.Name, a.Units)
		if a.Work.Amount != 0 {
			fmt.Printf(", %.1f%s", a.Work.Amount, a.Work.Units)
		}
		if len(a.TimephasedWork) > 0 {
			fmt.Printf(", %d timephased span(s)", len(a.TimephasedWork))
		}
		fmt.Println(")")
	}
}

// date formats a date, showing an unset one as "n/a" rather than as the
// zero time.
func date(t time.Time) string {
	if t.IsZero() {
		return "n/a"
	}
	return t.Format("2006-01-02")
}

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
