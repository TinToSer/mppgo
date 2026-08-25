// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

// Package project is the format-agnostic core data model — the Go
// equivalent of MPXJ's org.mpxj root package. MPP and MSPDI readers and
// writers both target this model.
package project

// File is a parsed project file: properties, calendars, tasks, resources,
// and assignments.
type File struct {
	Properties  *Properties
	Calendars   []*Calendar
	Tasks       []*Task
	Resources   []*Resource
	Assignments []*Assignment

	// DefaultCalendar is the project's default calendar, if it could be
	// identified.
	DefaultCalendar *Calendar

	// ResourceCalendars maps a resource unique ID to that resource's
	// calendar. Populated by the MPP reader ahead of resource support, so
	// the link survives once resources are read.
	ResourceCalendars map[int]*Calendar

	calendarsByID map[int]*Calendar
	tasksByID     map[int]*Task
	resourcesByID map[int]*Resource
}

// New creates an empty project File.
func New() *File {
	return &File{
		Properties:        &Properties{},
		ResourceCalendars: make(map[int]*Calendar),
		calendarsByID:     make(map[int]*Calendar),
		tasksByID:         make(map[int]*Task),
		resourcesByID:     make(map[int]*Resource),
	}
}

// AddCalendar registers a calendar, indexing it by UniqueID.
func (f *File) AddCalendar(c *Calendar) {
	f.Calendars = append(f.Calendars, c)
	if c.UniqueID != 0 {
		f.calendarsByID[c.UniqueID] = c
	}
}

// AddTask registers a task, indexing it by UniqueID.
func (f *File) AddTask(t *Task) {
	f.Tasks = append(f.Tasks, t)
	if t.UniqueID != 0 {
		f.tasksByID[t.UniqueID] = t
	}
}

// AddResource registers a resource, indexing it by UniqueID.
func (f *File) AddResource(r *Resource) {
	f.Resources = append(f.Resources, r)
	if r.UniqueID != 0 {
		f.resourcesByID[r.UniqueID] = r
	}
}

// AddAssignment registers a resource assignment.
func (f *File) AddAssignment(a *Assignment) {
	f.Assignments = append(f.Assignments, a)
}

// CalendarByID looks up a calendar by its unique ID.
func (f *File) CalendarByID(id int) *Calendar { return f.calendarsByID[id] }

// TaskByID looks up a task by its unique ID.
func (f *File) TaskByID(id int) *Task { return f.tasksByID[id] }

// ResourceByID looks up a resource by its unique ID.
func (f *File) ResourceByID(id int) *Resource { return f.resourcesByID[id] }

// CalendarByName returns the first calendar with the given name. Names are
// not guaranteed unique, and derived resource calendars are often unnamed.
func (f *File) CalendarByName(name string) *Calendar {
	if name == "" {
		return nil
	}
	for _, c := range f.Calendars {
		if c.Name == name {
			return c
		}
	}
	return nil
}

// BaseCalendars returns the calendars that are not derived from another —
// the named calendars a user would pick from in MS Project.
func (f *File) BaseCalendars() []*Calendar {
	var result []*Calendar
	for _, c := range f.Calendars {
		if c.Parent == nil {
			result = append(result, c)
		}
	}
	return result
}
