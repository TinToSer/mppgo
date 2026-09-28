// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mspdi

import (
	"encoding/xml"
	"strconv"
	"strings"
	"time"
)

// mspdiDateTimeLayouts are the xsd:dateTime lexical forms this reader
// accepts, tried in order. MS Project itself never writes a timezone
// offset, but the format is documented to allow one, and a third-party
// exporter might.
var mspdiDateTimeLayouts = []string{
	"2006-01-02T15:04:05",
	"2006-01-02T15:04:05.999999999",
	time.RFC3339,
	"2006-01-02T15:04:05Z07:00",
}

// xmlDateTime decodes an optional xsd:dateTime element into a time.Time,
// leaving it as the zero value (Valid false) when the element is absent
// or empty — MS Project itself omits the element entirely for "not set"
// rather than writing a sentinel value.
type xmlDateTime struct {
	Time  time.Time
	Valid bool
}

func (d *xmlDateTime) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var s string
	if err := dec.DecodeElement(&s, &start); err != nil {
		return err
	}
	// A date this reader can't parse is treated as absent rather than
	// failing the whole file — the same degrade-don't-fail stance the mpp
	// package takes with malformed binary fields.
	d.Time, d.Valid = parseMSPDIDateTime(s)
	return nil
}

// parseMSPDIDateTime parses an xsd:dateTime string using every layout MSPDI
// is known to write (see mspdiDateTimeLayouts), reporting ok=false rather
// than an error for anything empty or unrecognised.
func parseMSPDIDateTime(s string) (time.Time, bool) {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}, false
	}
	for _, layout := range mspdiDateTimeLayouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, true
		}
	}
	return time.Time{}, false
}

// xmlTime decodes an optional xsd:time element (a calendar working-hours
// boundary, e.g. "08:00:00") into an offset from midnight.
type xmlTime struct {
	Offset time.Duration
	Valid  bool
}

func (t *xmlTime) UnmarshalXML(dec *xml.Decoder, start xml.StartElement) error {
	var s string
	if err := dec.DecodeElement(&s, &start); err != nil {
		return err
	}
	s = strings.TrimSpace(s)
	if s == "" {
		return nil
	}
	// Strip an optional timezone/fractional-second suffix; only the plain
	// HH:MM:SS (or HH:MM) prefix is used.
	if i := strings.IndexAny(s, "Z+"); i > 0 {
		s = s[:i]
	}
	if i := strings.Index(s, "-"); i > 8 { // a date-less time won't have a '-' this late; guards a trailing zone offset
		s = s[:i]
	}
	parts := strings.SplitN(s, ":", 3)
	if len(parts) < 2 {
		return nil
	}
	hour, err1 := strconv.Atoi(parts[0])
	minute, err2 := strconv.Atoi(parts[1])
	if err1 != nil || err2 != nil {
		return nil
	}
	seconds := 0.0
	if len(parts) == 3 {
		seconds, _ = strconv.ParseFloat(parts[2], 64)
	}
	t.Offset = time.Duration(hour)*time.Hour + time.Duration(minute)*time.Minute + time.Duration(seconds*float64(time.Second))
	t.Valid = true
	return nil
}
