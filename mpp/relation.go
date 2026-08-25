// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"fmt"

	"github.com/tintoser/mppgo/project"
)

const (
	// Each TBkndCons FixedData record is 20 bytes.
	relationRecordSize = 20

	// Offsets within a TBkndCons FixedData record. Unlike tasks and
	// resources, constraints are not covered by a per-file field map — MS
	// Project writes them at fixed positions, so these are used directly.
	relationOffsetUniqueID    = 0
	relationOffsetPredecessor = 4
	relationOffsetSuccessor   = 8
	relationOffsetType        = 12

	// A record shorter than this cannot hold the type field and is skipped.
	relationMinRecordSize = 14
)

// relationLagLayout returns the byte offsets of the lag value and its units
// field. Project 2013+ swapped them relative to Project 2010.
func relationLagLayout(applicationVersion int) (lagOffset, unitsOffset int) {
	if applicationVersion > appVersionProject2010 {
		return 14, 18
	}
	return 16, 14
}

// readRelations reads the TBkndCons storage and returns the task
// dependencies it defines. The storage is optional: a file with no
// dependencies at all may omit it, which is not an error.
//
// Relations are returned as read; the caller links them to tasks (see
// project.File.AddRelation), which is also what drops any relation naming a
// task that was not read.
func readRelations(src *streamSource, projectDirPath string, applicationVersion int, scale durationScale, defaultUnits project.TimeUnit) ([]*project.Relation, error) {
	dir := projectDirPath + "/TBkndCons"
	if !src.has(dir + "/FixedMeta") {
		return nil, nil
	}

	fixedMetaRaw, err := src.plain(dir + "/FixedMeta")
	if err != nil {
		return nil, err
	}
	fixedMeta, err := ParseFixedMeta(fixedMetaRaw, 10)
	if err != nil {
		return nil, fmt.Errorf("mpp: constraint FixedMeta: %w", err)
	}
	fixedRaw, err := src.decoded(dir + "/FixedData")
	if err != nil {
		return nil, err
	}
	fixedData := ParseFixedData(fixedMeta, fixedRaw, relationRecordSize, 0)

	lagOffset, lagUnitsOffset := relationLagLayout(applicationVersion)

	var relations []*project.Relation
	for i := 0; i < fixedMeta.AdjustedItemCount; i++ {
		meta := fixedMeta.ByteArrayValue(i)
		if meta == nil {
			continue
		}
		// The deleted flag is a short, not an int: reading four bytes here
		// picks up unrelated neighbouring data and discards live records.
		if getShort(meta, 0) != 0 {
			continue
		}

		data := fixedData.ByteArrayValue(i)
		if len(data) < relationMinRecordSize {
			continue
		}

		predecessor := getInt(data, relationOffsetPredecessor)
		successor := getInt(data, relationOffsetSuccessor)

		// Unique ID 0 is the hidden project summary task, which cannot
		// take part in a dependency, and a task cannot depend on itself.
		if predecessor <= 0 || successor <= 0 || predecessor == successor {
			continue
		}

		relations = append(relations, &project.Relation{
			UniqueID:            getInt(data, relationOffsetUniqueID),
			PredecessorUniqueID: predecessor,
			SuccessorUniqueID:   successor,
			Type:                project.RelationType(getShort(data, relationOffsetType)),
			Lag: scale.duration(
				getInt(data, lagOffset),
				durationTimeUnits(getShort(data, lagUnitsOffset), defaultUnits),
			),
		})
	}

	return relations, nil
}
