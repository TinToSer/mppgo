package mpp

import (
	"bytes"
	"encoding/binary"
	"testing"
	"time"
	"unicode/utf16"

	"github.com/tintoser/mppgo/project"
)

// varDataOf builds a Var2Data holding the given values, keyed by
// {uniqueID, var-data key}.
func varDataOf(t *testing.T, values map[[2]int][]byte) *Var2Data {
	t.Helper()
	var raw bytes.Buffer
	var entries [][3]int
	for k, v := range values {
		entries = append(entries, [3]int{k[0], raw.Len(), k[1]})
		raw.Write(u32(len(v)))
		raw.Write(v)
	}
	meta, err := ParseVarMeta(buildVarMeta(raw.Len(), entries...))
	if err != nil {
		t.Fatal(err)
	}
	return ParseVar2Data(meta, raw.Bytes())
}

func utf16z(s string) []byte {
	var b []byte
	for _, u := range utf16.Encode([]rune(s)) {
		b = append(b, byte(u), byte(u>>8))
	}
	return append(b, 0, 0)
}

// fieldMapRecord is one 28-byte field-map entry.
func fieldMapRecord(offset, fullID, category int) []byte {
	rec := make([]byte, fieldMapRecordSize)
	binary.LittleEndian.PutUint16(rec[4:], uint16(offset))
	binary.LittleEndian.PutUint32(rec[12:], uint32(fullID))
	binary.LittleEndian.PutUint16(rec[20:], uint16(category))
	return rec
}

func TestFieldDecoder(t *testing.T) {
	var fieldMap bytes.Buffer
	fieldMap.Write(fieldMapRecord(0, taskFieldBase|23, 0))                      // ID, int
	fieldMap.Write(fieldMapRecord(4, taskFieldBase|29, 0))                      // Duration
	fieldMap.Write(fieldMapRecord(8, taskFieldBase|181, 0))                     // its units
	fieldMap.Write(fieldMapRecord(10, taskFieldBase|35, 0))                     // Start
	fieldMap.Write(fieldMapRecord(14, taskFieldBase|5, 0))                      // Cost
	fieldMap.Write(fieldMapRecord(30, taskFieldBase|71, fieldMapCategoryMeta0)) // Marked: a meta flag, not in fixed data
	fieldMap.Write(fieldMapRecord(0, taskFieldBase|1172, 0))                    // BudgetCost; a lower offset: block 1

	rec := make([]byte, 22)
	binary.LittleEndian.PutUint32(rec[0:], 7)
	binary.LittleEndian.PutUint32(rec[4:], 4800*3) // 3 days
	binary.LittleEndian.PutUint16(rec[8:], 7)      // days
	binary.LittleEndian.PutUint16(rec[10:], 4800)  // 08:00
	binary.LittleEndian.PutUint16(rec[12:], 15000)
	putFloat64(rec[14:], 12345)
	rec2 := make([]byte, 8)
	putFloat64(rec2, 250)

	listRef := make([]byte, 26)
	binary.LittleEndian.PutUint16(listRef, valueListWithIDMask)
	binary.LittleEndian.PutUint32(listRef[2:], 42)
	varData := varDataOf(t, map[[2]int][]byte{
		{1, 14}: utf16z("Task name"),
		{1, 51}: listRef, // Text1 from a lookup table
	})

	ctx := &readContext{
		scale:        newDurationScale(&project.Properties{}),
		defaultUnits: project.Days,
		values:       map[int]lookupValue{42: {text: "Approved", value: "Approved"}},
	}
	d := newFieldDecoder(ctx, taskFieldDefs, fieldMap.Bytes())
	f := d.decode([][]byte{rec, rec2}, varData, 1)

	want := map[string]interface{}{
		"ID":                  7,
		"Duration":            project.Duration{Amount: 3, Units: project.Days},
		"ActualDurationUnits": project.Days,
		"Start":               time.Date(2025, 1, 24, 8, 0, 0, 0, time.UTC),
		"Cost":                123.45,
		"Name":                "Task name",
		"Text1":               "Approved",
	}
	for name, v := range want {
		if f[name] != v {
			t.Errorf("%s = %#v, want %#v", name, f[name], v)
		}
	}
	if _, ok := f["Marked"]; ok {
		t.Error("a meta-data flag must not be decoded from fixed data")
	}
	if got := f["BudgetCost"]; got != 2.5 {
		t.Errorf("block-1 BudgetCost = %v, want 2.5", got)
	}
}

func TestFieldDecoderAlternateAssignmentKeys(t *testing.T) {
	text1 := fieldIndexByName(assignmentAlternateFieldDefs)["Text1"]
	varData := varDataOf(t, map[[2]int][]byte{{9, alternateFieldFlag | text1}: utf16z("alternate")})
	d := newFieldDecoder(&readContext{scale: newDurationScale(&project.Properties{})}, assignmentFieldDefs, nil)
	d.alternate = assignmentAlternateFieldDefs
	if got := d.decode(nil, varData, 9)["Text1"]; got != "alternate" {
		t.Fatalf("Text1 = %#v, want the value stored under the alternate key", got)
	}
}

func TestApplyWorkWeeks(t *testing.T) {
	data := make([]byte, calendarHoursSize+4) // default week + empty exception list
	data = append(data, make([]byte, 4)...)   // work-week block header
	week := make([]byte, calendarHoursSize)
	for day := 0; day < calendarDayCount; day++ {
		rec := week[day*calendarDayRecordSize:]
		switch day {
		case int(time.Monday):
			binary.LittleEndian.PutUint16(rec[2:], 1)
			binary.LittleEndian.PutUint16(rec[8:], 4200) // 07:00
			binary.LittleEndian.PutUint32(rec[20:], 3000)
		case int(time.Saturday):
			// working-period count 0: non-working
		default:
			binary.LittleEndian.PutUint16(rec, 1) // default
		}
	}
	data = append(data, week...)
	dates := make([]byte, 4+8)
	binary.LittleEndian.PutUint16(dates, 15000)
	binary.LittleEndian.PutUint16(dates[2:], 15006)
	data = append(data, dates...)
	name := utf16z("Summer")
	data = append(data, u32(len(name))...)
	data = append(data, name...)

	cal := project.NewCalendar()
	applyWorkWeeks(data, calendarHoursSize+4, cal)
	if len(cal.WorkWeeks) != 1 {
		t.Fatalf("got %d work weeks, want 1", len(cal.WorkWeeks))
	}
	w := cal.WorkWeeks[0]
	if w.Name != "Summer" || w.Days[time.Monday] != project.DayWorking || w.Days[time.Saturday] != project.DayNonWorking || w.Days[time.Tuesday] != project.DayDefault {
		t.Fatalf("unexpected work week %+v", w)
	}
	if got := w.Hours[time.Monday]; len(got) != 1 || got[0].Start != 7*time.Hour || got[0].End != 12*time.Hour {
		t.Fatalf("Monday hours = %v", got)
	}

	cal.SetWorkingDay(time.Monday, true)
	cal.Hours[time.Monday] = []project.TimeRange{{Start: 8 * time.Hour, End: 17 * time.Hour}}
	cal.SetWorkingDay(time.Saturday, true)
	inWeek := w.FromDate.AddDate(0, 0, (int(time.Monday)-int(w.FromDate.Weekday())+7)%7)
	if got := cal.HoursOn(inWeek); len(got) != 1 || got[0].Start != 7*time.Hour {
		t.Errorf("work week must override the default week on %v: %v", inWeek, got)
	}
	if got := cal.HoursOn(inWeek.AddDate(0, 0, 7)); len(got) != 1 || got[0].Start != 8*time.Hour {
		t.Errorf("outside the work week the default week applies: %v", got)
	}
}

func TestParsePropertySets(t *testing.T) {
	// One section: code page 1252, title (VT_LPSTR), revision (VT_LPSTR)
	// and a creation time (VT_FILETIME).
	var section bytes.Buffer
	props := []struct {
		id    uint32
		value []byte
	}{
		{1, append(u32(vtI2), u16(1252)...)},
		{2, append(append(u32(vtLPStr), u32(6)...), []byte("Plan\xe9\x00")...)},
		{12, append(u32(vtFiletime), func() []byte {
			b := make([]byte, 8)
			binary.LittleEndian.PutUint64(b, uint64(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC).Unix()+11644473600)*10000000)
			return b
		}()...)},
	}
	header := 8 + len(props)*8
	var values bytes.Buffer
	var index bytes.Buffer
	for _, p := range props {
		index.Write(u32(int(p.id)))
		index.Write(u32(header + values.Len()))
		values.Write(p.value)
		for values.Len()%4 != 0 {
			values.WriteByte(0)
		}
	}
	section.Write(u32(header + values.Len()))
	section.Write(u32(len(props)))
	section.Write(index.Bytes())
	section.Write(values.Bytes())

	stream := make([]byte, 48)
	binary.LittleEndian.PutUint16(stream, 0xFFFE)
	binary.LittleEndian.PutUint32(stream[24:], 1)
	binary.LittleEndian.PutUint32(stream[44:], 48)
	stream = append(stream, section.Bytes()...)

	sets := parsePropertySets(stream)
	if len(sets) != 1 {
		t.Fatalf("got %d sections", len(sets))
	}
	if got := sets[0].values[2]; got != "Plané" {
		t.Errorf("title = %#v", got)
	}
	if got, _ := sets[0].values[12].(time.Time); !got.Equal(time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)) {
		t.Errorf("created = %v", sets[0].values[12])
	}
	if parsePropertySets([]byte("garbage")) != nil {
		t.Error("garbage must yield no sections")
	}
}

func TestReadHyperlink(t *testing.T) {
	data := make([]byte, 12)
	for _, s := range []string{"Spec", "http://example.com", "Section 2", "Open spec"} {
		data = append(data, make([]byte, 12)...)
		data = append(data, utf16z(s)...)
	}
	text, address, sub, tip := readHyperlink(data)
	if text != "Spec" || address != "http://example.com" || sub != "Section 2" || tip != "Open spec" {
		t.Fatalf("got %q %q %q %q", text, address, sub, tip)
	}
	if a, b, c, d := readHyperlink(nil); a+b+c+d != "" {
		t.Fatal("nil block must decode to nothing")
	}
}

func TestReadRecurringTask(t *testing.T) {
	data := make([]byte, 72)
	binary.LittleEndian.PutUint16(data[6:], 15000)
	binary.LittleEndian.PutUint16(data[10:], 15100)
	binary.LittleEndian.PutUint32(data[12:], 4800) // 1 day
	binary.LittleEndian.PutUint16(data[16:], 7)
	binary.LittleEndian.PutUint16(data[18:], 15)    // occurrences
	binary.LittleEndian.PutUint16(data[20:], 4)     // weekly
	binary.LittleEndian.PutUint16(data[28+2*1:], 1) // Monday
	binary.LittleEndian.PutUint16(data[28+2*3:], 1) // Wednesday
	binary.LittleEndian.PutUint16(data[48:], 2)     // every 2 weeks
	ctx := &readContext{scale: newDurationScale(&project.Properties{}), defaultUnits: project.Days}
	r := readRecurringTask(ctx, data)
	if r == nil || r.Type != "Weekly" || r.Occurrences != 15 || r.Frequency != 2 || !r.WeeklyDays[time.Monday] || !r.WeeklyDays[time.Wednesday] || r.WeeklyDays[time.Tuesday] {
		t.Fatalf("got %+v", r)
	}
	if r.Duration != (project.Duration{Amount: 1, Units: project.Days}) {
		t.Errorf("duration = %v", r.Duration)
	}
	if readRecurringTask(ctx, data[:20]) != nil {
		t.Error("a short block must be rejected")
	}
}
