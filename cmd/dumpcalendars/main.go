// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

// Command dumpcalendars reads an MPP file and prints its project
// properties and calendars, for manual inspection.
package main

import (
	"fmt"
	"os"
	"time"

	"github.com/tintoser/mppgo/mpp"
	"github.com/tintoser/mppgo/project"
)

func main() {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "usage: dumpcalendars <file.mpp>")
		os.Exit(2)
	}

	pf, err := mpp.ReadFile(os.Args[1])
	if err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}

	fmt.Printf("Application:      %s (version %d)\n", pf.Properties.ApplicationName, pf.Properties.ApplicationVersion)
	fmt.Printf("File path:        %s\n", pf.Properties.FilePath)
	fmt.Printf("Default calendar: %q\n", pf.Properties.DefaultCalendarName)
	fmt.Printf("Calendars:        %d (%d base)\n\n", len(pf.Calendars), len(pf.BaseCalendars()))

	for _, c := range pf.Calendars {
		describe(c)
	}
}

func describe(c *project.Calendar) {
	name := c.Name
	if name == "" {
		name = "(unnamed)"
	}
	fmt.Printf("Calendar #%d %s", c.UniqueID, name)
	if c.Parent != nil {
		parent := c.Parent.Name
		if parent == "" {
			parent = fmt.Sprintf("#%d", c.Parent.UniqueID)
		}
		fmt.Printf("  [derived from %s]", parent)
	}
	if c.GUID != "" {
		fmt.Printf("  guid=%s", c.GUID)
	}
	fmt.Println()

	for d := time.Sunday; d <= time.Saturday; d++ {
		// Show the resolved view, noting whether it is inherited.
		origin := ""
		if c.DayType(d) == project.DayDefault {
			origin = " (inherited)"
		}
		if c.IsWorkingDay(d) {
			fmt.Printf("  %-9s %v%s\n", d, c.HoursFor(d), origin)
		} else {
			fmt.Printf("  %-9s non-working%s\n", d, origin)
		}
	}

	if len(c.Exceptions) > 0 {
		fmt.Printf("  exceptions: %d\n", len(c.Exceptions))
		for _, e := range c.Exceptions {
			kind := "non-working"
			if e.Working() {
				kind = fmt.Sprintf("working %v", e.Ranges)
			}
			label := e.Name
			if label == "" {
				label = "(unnamed)"
			}
			fmt.Printf("    %s .. %s  %-24s %s\n",
				e.FromDate.Format("2006-01-02"), e.ToDate.Format("2006-01-02"), kind, label)
		}
	}
	fmt.Println()
}
