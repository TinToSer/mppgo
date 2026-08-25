// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import "regexp"

var applicationVersionPattern = regexp.MustCompile(`Microsoft.Project.(\d+).0`)

// CompObj is the decoded contents of the root "\x01CompObj" stream, which
// identifies the application and file format that wrote the file.
type CompObj struct {
	ApplicationName    string
	ApplicationVersion int // 0 if not determinable
	FileFormat         string
	ApplicationID      string
}

// ParseCompObj parses a CompObj stream.
func ParseCompObj(data []byte) *CompObj {
	c := &CompObj{}
	pos := 28
	if len(data)-pos < 4 {
		return c
	}
	length := getInt(data, pos)
	pos += 4
	if length < 1 || pos+length > len(data) {
		return c
	}
	c.ApplicationName = string(data[pos : pos+length-1]) // drop trailing NUL
	pos += length

	if m := applicationVersionPattern.FindStringSubmatch(c.ApplicationName); m != nil {
		v := 0
		for _, ch := range m[1] {
			v = v*10 + int(ch-'0')
		}
		c.ApplicationVersion = v
	}

	if c.ApplicationName == "Microsoft Project 4.0" {
		c.FileFormat = "MSProject.MPP4"
		c.ApplicationID = "MSProject.Project.4"
		return c
	}

	if len(data)-pos < 4 {
		return c
	}
	length = getInt(data, pos)
	pos += 4
	if length > 0 && pos+length <= len(data) {
		c.FileFormat = string(data[pos : pos+length-1])
		pos += length

		if len(data)-pos >= 4 {
			length = getInt(data, pos)
			pos += 4
			if length > 0 && pos+length <= len(data) {
				c.ApplicationID = string(data[pos : pos+length-1])
			}
		}
	}
	return c
}
