package mpp

import (
	"bytes"
	"reflect"
	"testing"
)

func TestRawEnumeration(t *testing.T) {
	props := &Props{values: map[int][]byte{30: {1}, 10: {2}, 20: {3}}}
	if !reflect.DeepEqual(props.Keys(), []int{10, 20, 30}) {
		t.Fatal("property keys must be complete and deterministic")
	}
	metadata := &VarMeta{table: map[int]map[int]int{3: {}, 1: {}, 2: {}}}
	if !reflect.DeepEqual(metadata.UniqueIDs(), []int{1, 2, 3}) {
		t.Fatal("record IDs must be complete and deterministic")
	}
	if _, err := ReadRawStream(bytes.NewReader([]byte("not an MPP")), "Props14", false); err == nil {
		t.Fatal("invalid compound file must be rejected")
	}
}
