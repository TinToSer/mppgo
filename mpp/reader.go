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
//
// Currently populated on the returned project.File: Properties (partial)
// and Calendars. Tasks, Resources and Assignments are not yet implemented.
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

	return pf, nil
}
