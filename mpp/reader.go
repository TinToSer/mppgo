// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

// Package mpp reads Microsoft Project MPP14 files — the binary format used
// by Project 2010 and later, including Project 2013/2016/2019/365.
//
// The MPP format is proprietary and undocumented; the layouts implemented
// here were derived from the MPXJ project's reverse engineering work. See
// the NOTICE file at the repository root.
package mpp

import (
	"errors"
	"fmt"
	"io"
	"os"

	"github.com/tintoser/mppgo/cfb"
	"github.com/tintoser/mppgo/project"
)

// Props keys for the root Props14 stream.
const (
	propsPasswordFlag           = 893386752
	propsProtectionPasswordHash = 893386756
	propsEncryptionCode         = 893386759
	propsProjectFilePath        = 893386760
)

// Props keys for the per-project Props stream (projectDirPath + "/Props").
// propsDefaultCalendarName/propsDefaultCalendarHours live in calendar.go
// alongside the code that uses them.
const (
	propsProjectStartDate  = 37748738
	propsProjectFinishDate = 37748739
	propsStatusDate        = 37748805
	propsMinutesPerDay     = 37748765
	propsMinutesPerWeek    = 37748766
	propsDaysPerMonth      = 37753743
	propsDurationUnits     = 37748757
)

// MS Project application versions, as reported by the CompObj stream.
const (
	appVersionProject2010 = 14
	appVersionProject2013 = 15
	appVersionProject2016 = 16
)

// fileFormatMPP14 is the CompObj format tag for MPP14 files. Project 2010
// through 365 all write this format.
const fileFormatMPP14 = "MSProject.MPP14"

// projectDirPath is the compound-file storage holding project data. The
// name is three spaces followed by "114" — the trailing digits encode the
// format version.
const projectDirPath = "   114"

var (
	// ErrPasswordProtected is returned when a file needs a read password.
	ErrPasswordProtected = errors.New("mpp: file is password protected")

	// ErrUnsupportedFormat is returned for a compound file that is not
	// MPP14 — an older MPP8/9/12 file, or not a Project file at all.
	ErrUnsupportedFormat = errors.New("mpp: unsupported file format")
)

// ReadFile opens and parses an MPP14 file from disk.
func ReadFile(path string) (*project.File, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	return Read(f)
}

// Read parses an MPP14 file. The reader must expose the whole file; an
// *os.File or a bytes.Reader both satisfy this.
func Read(r io.ReaderAt) (*project.File, error) {
	cf, err := cfb.Open(r)
	if err != nil {
		return nil, fmt.Errorf("mpp: %w", err)
	}

	compObjRaw, err := cf.OpenStream("\x01CompObj")
	if err != nil {
		return nil, fmt.Errorf("mpp: reading CompObj: %w", err)
	}
	compObj := ParseCompObj(compObjRaw)
	if compObj.FileFormat != fileFormatMPP14 {
		return nil, fmt.Errorf("%w: %q (only %s is supported)", ErrUnsupportedFormat, compObj.FileFormat, fileFormatMPP14)
	}

	docPropsRaw, err := cf.OpenStream("Props14")
	if err != nil {
		return nil, fmt.Errorf("mpp: reading Props14: %w", err)
	}
	docProps := ParseProps14(docPropsRaw)

	// The password flag alone does not mean the content is unreadable: it is
	// also set for the XOR obfuscation handled by streamSource. A genuine
	// read password is only in force when the encryption blob is present too.
	passwordRequiredToRead := docProps.Byte(propsPasswordFlag)&0x1 != 0
	encryptionPresent := docProps.ByteArray(propsProtectionPasswordHash) != nil
	if passwordRequiredToRead && encryptionPresent {
		return nil, ErrPasswordProtected
	}

	src := newStreamSource(cf, docProps)

	projectPropsRaw, err := src.decoded(projectDirPath + "/Props")
	if err != nil {
		return nil, err
	}
	projectProps := ParseProps14(projectPropsRaw)

	pf := project.New()
	pf.Properties.FilePath = docProps.UnicodeString(propsProjectFilePath)
	pf.Properties.DefaultCalendarName = projectProps.UnicodeString(propsDefaultCalendarName)
	pf.Properties.ApplicationVersion = compObj.ApplicationVersion
	pf.Properties.ApplicationName = compObj.ApplicationName
	if d, ok := projectProps.Timestamp(propsProjectStartDate); ok {
		pf.Properties.StartDate = d
	}
	if d, ok := projectProps.Timestamp(propsProjectFinishDate); ok {
		pf.Properties.FinishDate = d
	}
	if d, ok := projectProps.Timestamp(propsStatusDate); ok {
		pf.Properties.StatusDate = d
	}
	pf.Properties.MinutesPerDay = projectProps.Int(propsMinutesPerDay)
	pf.Properties.MinutesPerWeek = projectProps.Int(propsMinutesPerWeek)
	pf.Properties.DaysPerMonth = projectProps.Short(propsDaysPerMonth)

	// Durations are stored as a raw count plus a units code; converting one
	// into the days/weeks/months figure MS Project displays depends on the
	// project settings read just above.
	scale := newDurationScale(pf.Properties)
	defaultDurationUnits := durationTimeUnits(projectProps.Short(propsDurationUnits), project.Days)

	calendars, resourceCalendars, err := readCalendars(src, projectDirPath, projectProps, compObj.ApplicationVersion)
	if err != nil {
		return nil, err
	}
	for _, c := range calendars {
		pf.AddCalendar(c)
	}
	pf.ResourceCalendars = resourceCalendars

	if cal := pf.CalendarByName(pf.Properties.DefaultCalendarName); cal != nil {
		pf.DefaultCalendar = cal
	}

	resources, err := readResources(src, projectDirPath, projectProps, compObj.ApplicationVersion)
	if err != nil {
		return nil, err
	}
	for _, r := range resources {
		if cal, ok := pf.ResourceCalendars[r.UniqueID]; ok {
			r.CalendarUniqueID = cal.UniqueID
		}
		pf.AddResource(r)
	}

	tasks, err := readTasks(src, projectDirPath, projectProps, compObj.ApplicationVersion, scale, defaultDurationUnits)
	if err != nil {
		return nil, err
	}
	for _, t := range tasks {
		pf.AddTask(t)
	}

	// Relations are read after tasks so each one can be linked to the tasks
	// it names as it is added.
	relations, err := readRelations(src, projectDirPath, compObj.ApplicationVersion, scale, defaultDurationUnits)
	if err != nil {
		return nil, err
	}
	for _, r := range relations {
		pf.AddRelation(r)
	}

	assignments, err := readAssignments(src, projectDirPath, projectProps)
	if err != nil {
		return nil, err
	}
	for _, a := range assignments {
		pf.AddAssignment(a)
	}

	return pf, nil
}
