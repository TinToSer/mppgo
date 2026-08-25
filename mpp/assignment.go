// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"fmt"

	"github.com/tintoser/mppgo/project"
)

const (
	// assignmentFixedRecordSize is the size of a TBkndAssn FixedData
	// record. Unlike tasks/resources, MPXJ reads this stream as fixed-size
	// records rather than locating them via FixedMeta offsets — the
	// FixedMeta block is used only for its per-item deleted flag and byte
	// offset (which, divided by this size, gives the FixedData index).
	assignmentFixedRecordSize = 110

	// Field IDs (low 16 bits of an assignment field-map entry). See
	// fieldmap.go.
	assignmentFieldIDUniqueID   = 0
	assignmentFieldIDTaskID     = 1
	assignmentFieldIDResourceID = 2
	assignmentFieldIDUnits      = 7
	assignmentFieldIDWork       = 8

	// MPP14 default offsets within a TBkndAssn FixedData record, used when
	// the file carries no field map of its own. See NOTICE.
	assignmentDefaultOffsetUniqueID   = 0
	assignmentDefaultOffsetTaskID     = 4
	assignmentDefaultOffsetResourceID = 8
	assignmentDefaultOffsetUnits      = 46
	assignmentDefaultOffsetWork       = 54

	assignmentNullResourceID = -65535
)

// readAssignments reads the TBkndAssn storage and returns the resource
// assignments it defines. TaskUniqueID/ResourceUniqueID are returned as
// found in the file; the caller can cross-reference them against
// File.TaskByID/ResourceByID.
func readAssignments(src *streamSource, projectDirPath string, projectProps *Props) ([]*project.Assignment, error) {
	dir := projectDirPath + "/TBkndAssn"

	varMetaRaw, err := src.plain(dir + "/VarMeta")
	if err != nil {
		return nil, err
	}
	varMeta, err := ParseVarMeta(varMetaRaw)
	if err != nil {
		return nil, fmt.Errorf("mpp: assignment VarMeta: %w", err)
	}

	fixedMetaRaw, err := src.plain(dir + "/FixedMeta")
	if err != nil {
		return nil, err
	}
	fixedMeta, err := ParseFixedMeta(fixedMetaRaw, 34)
	if err != nil {
		return nil, fmt.Errorf("mpp: assignment FixedMeta: %w", err)
	}
	fixedRaw, err := src.decoded(dir + "/FixedData")
	if err != nil {
		return nil, err
	}
	fixedData := ParseFixedDataFixedSize(fixedRaw, assignmentFixedRecordSize)

	fm := loadFieldMap(projectProps, assignmentFieldMapPropsKey1, assignmentFieldMapPropsKey2)
	off := func(fieldID, defaultOffset int) int {
		return fieldOffset(fm, assignmentFieldBase|fieldID, defaultOffset)
	}
	offUniqueID := off(assignmentFieldIDUniqueID, assignmentDefaultOffsetUniqueID)
	offTaskID := off(assignmentFieldIDTaskID, assignmentDefaultOffsetTaskID)
	offResourceID := off(assignmentFieldIDResourceID, assignmentDefaultOffsetResourceID)
	offUnits := off(assignmentFieldIDUnits, assignmentDefaultOffsetUnits)
	offWork := off(assignmentFieldIDWork, assignmentDefaultOffsetWork)

	var assignments []*project.Assignment

	for i := 0; i < fixedMeta.AdjustedItemCount; i++ {
		meta := fixedMeta.ByteArrayValue(i)
		if len(meta) == 0 || meta[0] != 0 {
			continue // deleted
		}

		// FixedData items sit at exact multiples of the record size, so an
		// offset that is not one does not name a record at all. Rounding it
		// down would silently attribute another assignment's data to this
		// one, so it is skipped instead.
		byteOffset := getInt(meta, 4)
		if byteOffset < 0 || byteOffset%assignmentFixedRecordSize != 0 {
			continue
		}
		data := fixedData.ByteArrayValue(byteOffset / assignmentFixedRecordSize)
		if data == nil {
			continue
		}

		uniqueID := getInt(data, offUniqueID)
		if len(varMeta.Types(uniqueID)) == 0 {
			continue // no var data: a phantom record left behind by a delete
		}

		resourceID := getInt(data, offResourceID)
		if resourceID == assignmentNullResourceID || resourceID <= 0 {
			continue
		}

		assignments = append(assignments, &project.Assignment{
			UniqueID:         uniqueID,
			TaskUniqueID:     getInt(data, offTaskID),
			ResourceUniqueID: resourceID,
			Units:            getUnits(data, offUnits),
			Work:             getWork(data, offWork),
		})
	}

	return assignments, nil
}
