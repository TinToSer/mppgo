package mpp

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

func TestNativeVariableWrite(t *testing.T) {
	raw := make([]byte, 36)
	binary.LittleEndian.PutUint32(raw, 32)
	metadata := &VarMeta{table: map[int]map[int]int{1: {14: 0}}, offsets: []int{0}}
	table := &nativeTable{varRaw: raw, variable: ParseVar2Data(metadata, raw)}
	if err := table.writeVariable(1, 14, nativeUTF16("MCP")); err != nil {
		t.Fatal(err)
	}
	if got := table.variable.UnicodeString(1, 14); got != "MCP" {
		t.Fatalf("invalid text: %q", got)
	}
	if err := table.writeVariable(1, 14, make([]byte, 33)); err == nil {
		t.Fatal("must reject growing a field")
	}
	if err := table.writeVariable(1, 15, []byte{0}); err == nil {
		t.Fatal("must reject a missing field")
	}
	metadata.table[2] = map[int]int{14: 0}
	if err := table.writeVariable(1, 14, nativeUTF16("Other")); err == nil {
		t.Fatal("must reject shared records")
	}
}

func TestNativeRTF(t *testing.T) {
	for _, text := range []string{"Plain notes", "Braces {x} and \\path", "First\nSecond", "Value \u00e9", "Unicode \U0001F680"} {
		if got := stripRTF(nativeRTF(text)); got != text {
			t.Errorf("RTF round trip: got %q, want %q", got, text)
		}
	}
}

func TestPatchMPPRealFile(t *testing.T) {
	path := os.Getenv("MPPGO_TEST_FILE")
	if path == "" {
		path = "../testdata/sample.mpp"
	}
	original, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		t.Skip("real MPP fixture is not present")
	}
	if err != nil {
		t.Fatal(err)
	}
	file, err := Read(bytes.NewReader(original))
	if err != nil {
		t.Fatal(err)
	}
	if len(file.Tasks) == 0 {
		t.Fatal("fixture must contain tasks")
	}
	task := file.Tasks[0]
	priority := (task.Priority + 1) % 1001
	unchanged := bytes.Clone(original)
	patched, err := Patch(original, []NativeEdit{{Entity: "tasks", UniqueID: task.UniqueID, Field: "priority", Value: priority}})
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(original, unchanged) || len(patched) != len(original) {
		t.Fatal("source changed or output grew")
	}
	result, err := Read(bytes.NewReader(patched))
	if err != nil || result.TaskByID(task.UniqueID).Priority != priority {
		t.Fatalf("priority was not persisted: %v", err)
	}
	if _, err := Patch(original, []NativeEdit{{Entity: "tasks", UniqueID: task.UniqueID, Field: "priority", Value: 1001}}); err == nil {
		t.Fatal("must reject invalid priority")
	}
	if _, err := Patch(original, []NativeEdit{{Entity: "tasks", UniqueID: task.UniqueID, Field: "start", Value: "2026-10-07"}}); err == nil {
		t.Fatal("must reject unsupported scheduling edits")
	}
}
