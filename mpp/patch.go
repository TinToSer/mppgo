package mpp

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"math"
	"reflect"
	"strconv"
	"strings"
	"unicode/utf16"

	"github.com/tintoser/mppgo/cfb"
	"github.com/tintoser/mppgo/project"
)

type NativeEdit struct {
	Entity   string      `json:"entity"`
	UniqueID int         `json:"unique_id"`
	Field    string      `json:"field"`
	Value    interface{} `json:"value"`
}

func NativeWritableFields() map[string][]string {
	return map[string][]string{
		"tasks":     {"name", "wbs", "notes", "priority", "custom_fields.Text1..30", "custom_fields.Number1..20", "custom_fields.Cost1..10", "custom_fields.Flag1..20"},
		"resources": {"name", "initials", "group", "code", "email_address", "notes", "custom_fields.Text1..30", "custom_fields.Number1..20", "custom_fields.Cost1..10", "custom_fields.Flag1..20"},
	}
}

type nativeTable struct {
	dir       string
	base      int
	metaRaw   []byte
	fixedRaw  []byte
	varRaw    []byte
	meta      *FixedMeta
	fixed     *FixedData
	variable  *Var2Data
	fieldMap  map[int]fieldLocation
	uidOffset int
}

func Patch(original []byte, edits []NativeEdit) ([]byte, error) {
	expected, err := Read(bytes.NewReader(original))
	if err != nil {
		return nil, err
	}
	container, err := cfb.Open(bytes.NewReader(original))
	if err != nil {
		return nil, err
	}
	docRaw, err := container.OpenStream("Props14")
	if err != nil {
		return nil, err
	}
	source := newStreamSource(container, ParseProps14(docRaw))
	propsRaw, err := source.decoded(projectDirPath + "/Props")
	if err != nil {
		return nil, err
	}
	props := ParseProps14(propsRaw)
	aliases := readCustomFieldAliases(source, projectDirPath)
	tables := make(map[string]*nativeTable)
	for index, edit := range edits {
		table := tables[edit.Entity]
		if table == nil {
			table, err = loadNativeTable(source, props, edit.Entity)
			if err != nil {
				return nil, fmt.Errorf("mpp: edit %d: %w", index, err)
			}
			tables[edit.Entity] = table
		}
		if err := table.apply(edit, expected, aliases); err != nil {
			return nil, fmt.Errorf("mpp: edit %d (%s %d %s): %w", index, edit.Entity, edit.UniqueID, edit.Field, err)
		}
	}
	replacements := make(map[string][]byte)
	for _, table := range tables {
		fixedRaw := bytes.Clone(table.fixedRaw)
		if source.obfuscate && source.mask != 0 {
			for index := range fixedRaw {
				fixedRaw[index] ^= source.mask
			}
		}
		for name, data := range map[string][]byte{"FixedMeta": table.metaRaw, "FixedData": fixedRaw, "Var2Data": table.varRaw} {
			path := table.dir + "/" + name
			stored, err := container.OpenStream(path)
			if err != nil {
				return nil, err
			}
			if !bytes.Equal(stored, data) {
				replacements[path] = data
			}
		}
	}
	output, err := cfb.PatchStreams(original, replacements)
	if err != nil {
		return nil, err
	}
	actual, err := Read(bytes.NewReader(output))
	if err != nil {
		return nil, fmt.Errorf("mpp: patched file could not be reopened: %w", err)
	}
	if !reflect.DeepEqual(expected, actual) {
		return nil, fmt.Errorf("mpp: read-back differs from intended edits; no output returned")
	}
	return output, nil
}

func loadNativeTable(source *streamSource, props *Props, entity string) (*nativeTable, error) {
	var storage string
	var base, metaSize, mapKey1, mapKey2, uidField, uidDefault int
	switch entity {
	case "tasks":
		storage, base, metaSize = "TBkndTask", taskFieldBase, 47
		mapKey1, mapKey2, uidField, uidDefault = taskFieldMapPropsKey1, taskFieldMapPropsKey2, taskFieldIDUniqueID, taskDefaultOffsetUniqueID
	case "resources":
		storage, base, metaSize = "TBkndRsc", resourceFieldBase, 37
		mapKey1, mapKey2, uidField, uidDefault = resourceFieldMapPropsKey1, resourceFieldMapPropsKey2, resourceFieldIDUniqueID, resourceDefaultOffsetUniqueID
	default:
		return nil, fmt.Errorf("native edits support tasks and resources only; export XML for other edits")
	}
	table := &nativeTable{dir: projectDirPath + "/" + storage, base: base}
	var err error
	if table.metaRaw, err = source.plain(table.dir + "/FixedMeta"); err != nil {
		return nil, err
	}
	if table.fixedRaw, err = source.decoded(table.dir + "/FixedData"); err != nil {
		return nil, err
	}
	if table.varRaw, err = source.plain(table.dir + "/Var2Data"); err != nil {
		return nil, err
	}
	varMetaRaw, err := source.plain(table.dir + "/VarMeta")
	if err != nil {
		return nil, err
	}
	varMeta, err := ParseVarMeta(varMetaRaw)
	if err != nil {
		return nil, err
	}
	table.variable = ParseVar2Data(varMeta, table.varRaw)
	if table.meta, err = ParseFixedMeta(table.metaRaw, metaSize); err != nil {
		return nil, err
	}
	table.fixed = ParseFixedData(table.meta, table.fixedRaw, 512, 0)
	table.fieldMap = loadFieldMap(props, mapKey1, mapKey2)
	table.uidOffset = fieldOffset(table.fieldMap, base|uidField, 0, uidDefault)
	return table, nil
}

func (table *nativeTable) apply(edit NativeEdit, expected *project.File, aliases customFieldAliases) error {
	var entity interface{}
	var custom *map[string]interface{}
	var textKeys []int
	var numberKeys []int
	var costKeys []int
	var flagIDs []int
	flagOffsets := make([]taskFlagBitOffset, 20)
	version := expected.Properties.ApplicationVersion
	if edit.Entity == "tasks" {
		task := expected.TaskByID(edit.UniqueID)
		if task == nil {
			return fmt.Errorf("task does not exist")
		}
		entity, custom = task, &task.CustomFields
		textKeys, numberKeys, costKeys, flagIDs = taskTextVarKeys[:], taskNumberVarKeys[:], taskCostVarKeys[:], taskFlagFieldIDs[:]
		layout := taskFlagBitLayout(version)
		copy(flagOffsets, layout[:])
	} else {
		resource := expected.ResourceByID(edit.UniqueID)
		if resource == nil {
			return fmt.Errorf("resource does not exist")
		}
		entity, custom = resource, &resource.CustomFields
		textKeys, numberKeys, costKeys, flagIDs = resourceTextVarKeys[:], resourceNumberVarKeys[:], resourceCostVarKeys[:], resourceFlagFieldIDs[:]
		for index, flag := range resourceFlagBitLayout(version) {
			flagOffsets[index] = taskFlagBitOffset{offset: flag.offset, mask: flag.mask}
		}
	}
	rowIndex := -1
	for index := 0; index < table.fixed.ItemCount(); index++ {
		record := table.fixed.ByteArrayValue(index)
		if edit.Entity == "tasks" && (index < 3 || len(record) == taskNullBlockSize || getInt(table.meta.ByteArrayValue(index), 0)&taskDeletedFlagMask != 0) {
			continue
		}
		if len(record) < table.uidOffset+4 || getInt(record, table.uidOffset) != edit.UniqueID {
			continue
		}
		rowIndex = index
		if edit.Entity == "resources" {
			break
		}
	}
	if rowIndex < 0 {
		return fmt.Errorf("no valid fixed record for entity")
	}
	field := strings.ToLower(edit.Field)
	if field == "priority" && edit.Entity == "tasks" {
		number, err := nativeNumber(edit.Value)
		if err != nil || number < 0 || number > 1000 || number != math.Trunc(number) {
			return fmt.Errorf("priority must be an integer from 0 to 1000")
		}
		offset := fieldOffset(table.fieldMap, taskFieldBase|taskFieldIDPriority, 0, taskDefaultOffsetPriority)
		record := table.fixed.ByteArrayValue(rowIndex)
		if offset < 0 || offset+2 > len(record) {
			return fmt.Errorf("priority field is absent from this record")
		}
		binary.LittleEndian.PutUint16(record[offset:], uint16(number))
		entity.(*project.Task).Priority = int(number)
		setDecodedField(entity, "Priority", int(number))
		return nil
	}
	direct := map[string]struct {
		key  int
		name string
	}{"name": {taskNameVarType, "Name"}, "wbs": {taskWBSVarType, "WBS"}, "notes": {taskNotesVarType, "Notes"}}
	if edit.Entity == "resources" {
		direct = map[string]struct {
			key  int
			name string
		}{"name": {resourceNameVarType, "Name"}, "initials": {resourceInitialsVarType, "Initials"},
			"group": {resourceGroupVarType, "Group"}, "code": {resourceCodeVarType, "Code"},
			"email_address": {resourceEmailVarType, "EmailAddress"}, "notes": {resourceNotesVarType, "Notes"}}
	}
	if descriptor, exists := direct[field]; exists {
		text, ok := edit.Value.(string)
		if !ok || strings.ContainsRune(text, 0) {
			return fmt.Errorf("%s must be a string without NUL characters", field)
		}
		var encoded []byte
		if field == "notes" {
			rtf := nativeRTF(text)
			encoded = append([]byte(rtf), 0)
			if err := table.writeVariable(edit.UniqueID, descriptor.key, encoded); err != nil {
				return err
			}
			reflect.ValueOf(entity).Elem().FieldByName("RTFNotes").SetString(rtf)
			text = stripRTF(rtf)
			setDecodedField(entity, "Notes", text)
		} else {
			if field == "wbs" && text == "" {
				return fmt.Errorf("cannot clear WBS: Project synthesizes it when absent")
			}
			encoded = nativeUTF16(text)
			if err := table.writeVariable(edit.UniqueID, descriptor.key, encoded); err != nil {
				return err
			}
			if text == "" {
				setDecodedField(entity, descriptor.name, nil)
			} else {
				setDecodedField(entity, descriptor.name, text)
			}
		}
		reflect.ValueOf(entity).Elem().FieldByName(descriptor.name).SetString(text)
		return nil
	}
	if !strings.HasPrefix(field, "custom_fields.") {
		return fmt.Errorf("field %q is not native-writable; export XML for wider edits", edit.Field)
	}
	requested := edit.Field[len("custom_fields."):]
	setCustom := func(name string, value interface{}) {
		if value == nil {
			delete(*custom, name)
			if len(*custom) == 0 {
				*custom = nil
			}
		} else {
			if *custom == nil {
				*custom = make(map[string]interface{})
			}
			(*custom)[name] = value
		}
	}
	for _, group := range []struct {
		prefix string
		keys   []int
	}{{"Text", textKeys}, {"Number", numberKeys}, {"Cost", costKeys}, {"Flag", flagIDs}} {
		for index, key := range group.keys {
			fallback := fmt.Sprintf("%s%d", group.prefix, index+1)
			name := aliases.name(table.base|key, fallback)
			if !strings.EqualFold(requested, name) && !strings.EqualFold(requested, fallback) {
				continue
			}
			switch group.prefix {
			case "Flag":
				value, ok := edit.Value.(bool)
				if !ok {
					return fmt.Errorf("flag value must be boolean")
				}
				layout := flagOffsets[index]
				record := table.meta.ByteArrayValue(rowIndex)
				if layout.offset < 0 || layout.offset+4 > len(record) {
					return fmt.Errorf("flag field is absent from metadata")
				}
				bits := binary.LittleEndian.Uint32(record[layout.offset:])
				bits &^= uint32(layout.mask)
				if value {
					bits |= uint32(layout.mask)
				}
				binary.LittleEndian.PutUint32(record[layout.offset:], bits)
				if value {
					setCustom(name, true)
				} else {
					setCustom(name, nil)
				}
			case "Text":
				text, ok := edit.Value.(string)
				if !ok || strings.ContainsRune(text, 0) {
					return fmt.Errorf("custom text must be a string without NUL characters")
				}
				if err := table.writeVariable(edit.UniqueID, key, nativeUTF16(text)); err != nil {
					return err
				}
				if text == "" {
					setCustom(name, nil)
					setDecodedField(entity, fallback, nil)
				} else {
					setCustom(name, text)
					setDecodedField(entity, fallback, text)
				}
			default:
				number, err := nativeNumber(edit.Value)
				if err != nil {
					return err
				}
				stored := number
				if group.prefix == "Cost" {
					stored *= 100
				}
				if math.IsInf(stored, 0) {
					return fmt.Errorf("custom number is outside the supported range")
				}
				encoded := make([]byte, 8)
				binary.LittleEndian.PutUint64(encoded, math.Float64bits(stored))
				if err := table.writeVariable(edit.UniqueID, key, encoded); err != nil {
					return err
				}
				setCustom(name, number)
				decoded := number
				if group.prefix == "Cost" {
					decoded = customFieldCurrency(stored)
				}
				setDecodedField(entity, fallback, decoded)
			}
			return nil
		}
	}
	return fmt.Errorf("custom field %q is not supported for native editing", requested)
}

func (table *nativeTable) writeVariable(uniqueID, key int, encoded []byte) error {
	stored := table.variable.ByteArray(uniqueID, key)
	if stored == nil {
		return fmt.Errorf("field has no existing variable record; export XML to add it")
	}
	if len(encoded) > len(stored) {
		return fmt.Errorf("value needs %d bytes but existing capacity is %d; export XML for larger values", len(encoded), len(stored))
	}
	offset, _ := table.variable.meta.Offset(uniqueID, key)
	references := 0
	for _, fields := range table.variable.meta.table {
		for _, candidate := range fields {
			if candidate == offset {
				references++
			}
		}
	}
	if references != 1 {
		return fmt.Errorf("variable record is shared by %d fields; independent edit requires XML export", references)
	}
	clear(stored)
	copy(stored, encoded)
	return nil
}

func nativeUTF16(text string) []byte {
	letters := utf16.Encode([]rune(text))
	encoded := make([]byte, (len(letters)+1)*2)
	for index, letter := range letters {
		binary.LittleEndian.PutUint16(encoded[index*2:], letter)
	}
	return encoded
}

func nativeRTF(text string) string {
	var encoded strings.Builder
	encoded.WriteString("{\\rtf1\\ansi\\uc1 ")
	for _, letter := range utf16.Encode([]rune(strings.ReplaceAll(text, "\r\n", "\n"))) {
		switch letter {
		case '\\', '{', '}':
			encoded.WriteByte('\\')
			encoded.WriteRune(rune(letter))
		case '\n':
			encoded.WriteString("\\par ")
		case '\r':
		case '\t':
			encoded.WriteString("\\tab ")
		default:
			if letter >= 32 && letter < 127 {
				encoded.WriteRune(rune(letter))
			} else {
				fmt.Fprintf(&encoded, "\\u%d?", int16(letter))
			}
		}
	}
	encoded.WriteByte('}')
	return encoded.String()
}

func nativeNumber(value interface{}) (float64, error) {
	number, err := strconv.ParseFloat(fmt.Sprint(value), 64)
	if err != nil || math.IsNaN(number) || math.IsInf(number, 0) {
		return 0, fmt.Errorf("value must be a finite number")
	}
	return number, nil
}

// setDecodedField mirrors an edit into the entity's Fields map, which the
// read-back comparison checks along with the typed fields. A nil value
// removes the field, as the reader leaves out a field with no value.
func setDecodedField(entity interface{}, name string, value interface{}) {
	fields := reflect.ValueOf(entity).Elem().FieldByName("Fields")
	if !fields.IsValid() {
		return
	}
	if fields.IsNil() {
		if value == nil {
			return
		}
		fields.Set(reflect.ValueOf(map[string]interface{}{}))
	}
	m := fields.Interface().(map[string]interface{})
	if value == nil {
		delete(m, name)
	} else {
		m[name] = value
	}
}
