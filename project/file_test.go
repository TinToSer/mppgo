// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package project

import "testing"

func TestTaskCalendarPrefersTasksOwnCalendar(t *testing.T) {
	f := New()
	own := NewCalendar()
	own.UniqueID = 5
	def := NewCalendar()
	def.UniqueID = 1
	f.AddCalendar(own)
	f.AddCalendar(def)
	f.DefaultCalendar = def

	task := &Task{CalendarUniqueID: 5}
	if got := f.TaskCalendar(task); got != own {
		t.Errorf("TaskCalendar = %v, want the task's own calendar", got)
	}
}

func TestTaskCalendarFallsBackToProjectDefault(t *testing.T) {
	f := New()
	def := NewCalendar()
	def.UniqueID = 1
	f.AddCalendar(def)
	f.DefaultCalendar = def

	// Zero means "no calendar set on the task itself".
	if got := f.TaskCalendar(&Task{}); got != def {
		t.Errorf("TaskCalendar (unset) = %v, want the project default", got)
	}

	// A dangling reference to a calendar that doesn't exist should also
	// fall back rather than returning nil.
	if got := f.TaskCalendar(&Task{CalendarUniqueID: 99}); got != def {
		t.Errorf("TaskCalendar (dangling reference) = %v, want the project default", got)
	}
}
