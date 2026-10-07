package mpp

import (
	"encoding/binary"
	"time"
	"unicode/utf16"

	"github.com/tintoser/mppgo/cfb"
	"github.com/tintoser/mppgo/project"
)

// Props keys for project settings held in the per-project Props stream.
// Sourced from MPXJ's PropsKey (see NOTICE).
const (
	propsGUID                  = 37748777
	propsScheduleFrom          = 37748740
	propsStartTime             = 37748764
	propsEndTime               = 37748769
	propsHyperlinkBase         = 37748810
	propsStandardRate          = 37748767
	propsOvertimeRate          = 37748768
	propsWorkUnits             = 37748758
	propsSplitTasks            = 37748762
	propsTaskUpdatesResource   = 37748761
	propsCriticalSlackLimit    = 37748756
	propsCurrencyDigits        = 37748754
	propsCurrencySymbol        = 37748752
	propsCurrencyCode          = 37753787
	propsCurrencyPlacement     = 37748753
	propsDefaultTaskType       = 37748785
	propsWeekStartDay          = 37748773
	propsFiscalYearStartMonth  = 37748780
	propsFiscalYearStart       = 37748801
	propsEditableActualCosts   = 37748802
	propsHonorConstraints      = 37748794
	propsMultipleCriticalPaths = 37748793
	propsNewTasksAreManual     = 37753800
	propsBaselineCalendarName  = 37753747
	propsResourcePool          = 37748747
	propsBaselineDate          = 37753749
	propsBaseline1Date         = 37753750 // Baseline1..10 follow consecutively
)

// readProjectProperties fills the project settings beyond the core ones
// Read sets itself, and the document properties from the OLE summary
// property sets, which take precedence over the Props copies of the same
// values (as in MPXJ: MS Project keeps both, and edits made through
// Windows' file properties only reach the OLE sets).
func readProjectProperties(pf *project.File, props *Props, cf *cfb.File, defaultUnits project.TimeUnit) {
	p := pf.Properties
	if g := props.ByteArray(propsGUID); len(g) >= 16 {
		p.GUID = getGUID(g, 0)
	}
	p.ScheduleFromStart = props.ByteArray(propsScheduleFrom) == nil || props.Short(propsScheduleFrom) == 1
	if v := props.ByteArray(propsStartTime); v != nil {
		p.DefaultStartTime = getTime(v, 0)
	}
	if v := props.ByteArray(propsEndTime); v != nil {
		p.DefaultEndTime = getTime(v, 0)
	}
	p.HyperlinkBase = props.UnicodeString(propsHyperlinkBase)
	p.DefaultDurationUnits = defaultUnits
	p.DefaultWorkUnits = workTimeUnit(props.Short(propsWorkUnits))
	p.DefaultStandardRate = props.Double(propsStandardRate)
	p.DefaultOvertimeRate = props.Double(propsOvertimeRate)
	p.SplitInProgressTasks = props.Boolean(propsSplitTasks)
	p.TaskUpdatesResource = props.Boolean(propsTaskUpdatesResource)
	p.CriticalSlackLimit = project.Duration{Amount: float64(props.Int(propsCriticalSlackLimit)), Units: project.Days}
	p.CurrencyDigits = props.Short(propsCurrencyDigits)
	p.CurrencySymbol = props.UnicodeString(propsCurrencySymbol)
	p.CurrencyCode = props.UnicodeString(propsCurrencyCode)
	p.CurrencySymbolPosition = currencySymbolPosition(props.Short(propsCurrencyPlacement))
	p.DefaultTaskType = taskType(props.Short(propsDefaultTaskType))
	p.WeekStartDay = time.Weekday(props.Short(propsWeekStartDay) % 7)
	p.FiscalYearStartMonth = props.Short(propsFiscalYearStartMonth)
	p.FiscalYearStart = props.Short(propsFiscalYearStart) == 1
	p.EditableActualCosts = props.Boolean(propsEditableActualCosts)
	p.HonorConstraints = !props.Boolean(propsHonorConstraints)
	p.MultipleCriticalPaths = props.Boolean(propsMultipleCriticalPaths)
	p.NewTasksAreManual = props.Boolean(propsNewTasksAreManual)
	p.BaselineCalendarName = props.UnicodeString(propsBaselineCalendarName)
	p.ResourcePoolFile = resourcePoolFile(props.ByteArray(propsResourcePool))
	for n := 0; n <= 10; n++ {
		key := propsBaselineDate
		if n > 0 {
			key = propsBaseline1Date + n - 1
		}
		if d, ok := props.Timestamp(key); ok {
			if p.BaselineDates == nil {
				p.BaselineDates = make(map[int]time.Time)
			}
			p.BaselineDates[n] = d
		}
	}

	if raw, err := cf.OpenStream("\x05SummaryInformation"); err == nil {
		sets := parsePropertySets(raw)
		if len(sets) > 0 {
			s := sets[0]
			setString(&p.Name, s, 2)
			setString(&p.Subject, s, 3)
			setString(&p.Author, s, 4)
			setString(&p.Keywords, s, 5)
			setString(&p.Comments, s, 6)
			p.Template, _ = s.values[7].(string)
			p.LastAuthor, _ = s.values[8].(string)
			if rev, ok := s.values[9].(string); ok {
				p.Revision = atoi(rev)
			}
			if d, ok := s.values[10].(time.Duration); ok {
				p.EditingTime = int(d / time.Minute)
			}
			p.LastPrinted, _ = s.values[11].(time.Time)
			p.CreationDate, _ = s.values[12].(time.Time)
			p.LastSaved, _ = s.values[13].(time.Time)
		}
	}
	if raw, err := cf.OpenStream("\x05DocumentSummaryInformation"); err == nil {
		sets := parsePropertySets(raw)
		if len(sets) > 0 {
			s := sets[0]
			setString(&p.Category, s, 2)
			setString(&p.Manager, s, 14)
			setString(&p.Company, s, 15)
			p.ContentType, _ = s.values[26].(string)
			p.ContentStatus, _ = s.values[27].(string)
			p.Language, _ = s.values[28].(string)
			p.DocumentVersion, _ = s.values[29].(string)
		}
		if len(sets) > 1 {
			for id, name := range sets[1].names {
				if v, ok := sets[1].values[id]; ok && name != "" {
					if p.CustomProperties == nil {
						p.CustomProperties = make(map[string]interface{})
					}
					p.CustomProperties[name] = v
				}
			}
		}
	}
}

// setString overwrites *dst with a non-empty string property.
func setString(dst *string, s propertySet, id uint32) {
	if v, ok := s.values[id].(string); ok && v != "" {
		*dst = v
	}
}

func atoi(s string) int {
	n := 0
	for _, c := range s {
		if c < '0' || c > '9' {
			return 0
		}
		n = n*10 + int(c-'0')
	}
	return n
}

func currencySymbolPosition(code int) string {
	switch code {
	case 1:
		return "After"
	case 2:
		return "Before with space"
	case 3:
		return "After with space"
	}
	return "Before"
}

// resourcePoolFile extracts the path of a shared resource pool file.
func resourcePoolFile(data []byte) string {
	offset := 18
	if offset+4 >= len(data) {
		return ""
	}
	offset += 4 + getInt(data, offset) + 34
	if offset < 0 || offset >= len(data) {
		return ""
	}
	return getUnicodeString(data, offset)
}

// propertySet is one section of an OLE property set stream (MS-OLEPS):
// property values by ID, and for a user-defined section the property names
// from its dictionary.
type propertySet struct {
	values map[uint32]interface{}
	names  map[uint32]string
}

// OLE property value types this reader decodes.
const (
	vtI2       = 2
	vtI4       = 3
	vtR4       = 4
	vtR8       = 5
	vtBool     = 11
	vtLPStr    = 30
	vtLPWStr   = 31
	vtFiletime = 64

	propertyDictionary = 0
	propertyCodePage   = 1
	codePageUTF16      = 1200
	codePageUTF8       = 65001
)

// parsePropertySets decodes an OLE property set stream. Malformed input
// yields whatever sections and values could be read.
func parsePropertySets(data []byte) []propertySet {
	if len(data) < 28 || binary.LittleEndian.Uint16(data) != 0xFFFE {
		return nil
	}
	count := int(binary.LittleEndian.Uint32(data[24:]))
	var sets []propertySet
	for i := 0; i < count && i < 2 && 28+i*20+20 <= len(data); i++ {
		start := int(binary.LittleEndian.Uint32(data[28+i*20+16:]))
		if start < 0 || start+8 > len(data) {
			break
		}
		sets = append(sets, parsePropertySection(data, start))
	}
	return sets
}

func parsePropertySection(data []byte, start int) propertySet {
	set := propertySet{values: make(map[uint32]interface{})}
	count := int(binary.LittleEndian.Uint32(data[start+4:]))
	offsets := make(map[uint32]int)
	for i := 0; i < count && start+8+i*8+8 <= len(data); i++ {
		id := binary.LittleEndian.Uint32(data[start+8+i*8:])
		offsets[id] = start + int(binary.LittleEndian.Uint32(data[start+8+i*8+4:]))
	}
	codePage := 0
	if off, ok := offsets[propertyCodePage]; ok && off+6 <= len(data) {
		codePage = int(binary.LittleEndian.Uint16(data[off+4:]))
	}
	for id, off := range offsets {
		if id == propertyDictionary || id == propertyCodePage || off < 0 || off+4 > len(data) {
			continue
		}
		if v, ok := propertyValue(data, off, codePage); ok {
			set.values[id] = v
		}
	}
	if off, ok := offsets[propertyDictionary]; ok {
		set.names = propertyDictionaryNames(data, off, codePage)
	}
	return set
}

func propertyValue(data []byte, off, codePage int) (interface{}, bool) {
	vt := binary.LittleEndian.Uint16(data[off:])
	v := off + 4
	need := func(n int) bool { return v+n <= len(data) }
	switch vt {
	case vtI2:
		if need(2) {
			return int(int16(binary.LittleEndian.Uint16(data[v:]))), true
		}
	case vtI4:
		if need(4) {
			return int(int32(binary.LittleEndian.Uint32(data[v:]))), true
		}
	case vtR8:
		if need(8) {
			return getDouble(data, v), true
		}
	case vtBool:
		if need(2) {
			return binary.LittleEndian.Uint16(data[v:]) != 0, true
		}
	case vtLPStr:
		if need(4) {
			n := int(binary.LittleEndian.Uint32(data[v:]))
			if n >= 0 && v+4+n <= len(data) {
				return decodeCodePageString(data[v+4:v+4+n], codePage), true
			}
		}
	case vtLPWStr:
		if need(4) {
			n := int(binary.LittleEndian.Uint32(data[v:]))
			if n >= 0 && v+4+n*2 <= len(data) {
				return decodeUTF16String(data[v+4 : v+4+n*2]), true
			}
		}
	case vtFiletime:
		if need(8) {
			ticks := binary.LittleEndian.Uint64(data[v:])
			// A FILETIME below a year is a duration (the edit time), not a
			// point in time.
			if ticks < uint64(365*24*time.Hour/100) {
				return time.Duration(ticks) * 100, true
			}
			if ticks == 0 {
				return nil, false
			}
			const epochDifference = 11644473600 // seconds from 1601 to 1970
			seconds := int64(ticks/10000000) - epochDifference
			return time.Unix(seconds, int64(ticks%10000000)*100).UTC(), true
		}
	}
	return nil, false
}

func propertyDictionaryNames(data []byte, off, codePage int) map[uint32]string {
	names := make(map[uint32]string)
	if off+4 > len(data) {
		return names
	}
	count := int(binary.LittleEndian.Uint32(data[off:]))
	pos := off + 4
	for i := 0; i < count && pos+8 <= len(data); i++ {
		id := binary.LittleEndian.Uint32(data[pos:])
		n := int(binary.LittleEndian.Uint32(data[pos+4:]))
		pos += 8
		if codePage == codePageUTF16 {
			if n < 0 || pos+n*2 > len(data) {
				break
			}
			names[id] = decodeUTF16String(data[pos : pos+n*2])
			pos += n * 2
			pos += (4 - pos%4) % 4 // UTF-16 names are padded to 4 bytes
		} else {
			if n < 0 || pos+n > len(data) {
				break
			}
			names[id] = decodeCodePageString(data[pos:pos+n], codePage)
			pos += n
		}
	}
	return names
}

func decodeUTF16String(b []byte) string {
	u := make([]uint16, 0, len(b)/2)
	for i := 0; i+1 < len(b); i += 2 {
		c := binary.LittleEndian.Uint16(b[i:])
		if c == 0 {
			break
		}
		u = append(u, c)
	}
	return string(utf16.Decode(u))
}

// decodeCodePageString decodes an 8-bit property string. UTF-16 and UTF-8
// code pages are honoured; anything else is read as Windows-1252, the code
// page Western-language Windows writes.
func decodeCodePageString(b []byte, codePage int) string {
	switch codePage {
	case codePageUTF16:
		return decodeUTF16String(b)
	case codePageUTF8:
		for i, c := range b {
			if c == 0 {
				b = b[:i]
				break
			}
		}
		return string(b)
	}
	r := make([]rune, 0, len(b))
	for _, c := range b {
		if c == 0 {
			break
		}
		r = append(r, cp1252ToRune(c))
	}
	return string(r)
}
