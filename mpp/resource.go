// Authored By: TinToSer (github.com/tintoser)
// Developed by: Claude Sonnet

package mpp

import (
	"fmt"

	"github.com/tintoser/mppgo/project"
)

const (
	resourceNameVarType     = 1
	resourceInitialsVarType = 2
	resourceGroupVarType    = 3
	resourceCodeVarType     = 10
	resourceEmailVarType    = 35

	// Field IDs (low 16 bits of a resource field-map entry). See fieldmap.go.
	resourceFieldIDUniqueID     = 27
	resourceFieldIDID           = 0
	resourceFieldIDMaxUnits     = 4
	resourceFieldIDStandardRate = 6
	resourceFieldIDWork         = 13
	resourceFieldIDCost         = 12

	// MPP14 default offsets within a TBkndRsc FixedData record, used when
	// the file carries no field map of its own. See NOTICE.
	resourceDefaultOffsetUniqueID     = 0
	resourceDefaultOffsetID           = 4
	resourceDefaultOffsetMaxUnits     = 44
	resourceDefaultOffsetStandardRate = 28
	resourceDefaultOffsetWork         = 52
	resourceDefaultOffsetCost         = 140
)

// resourceTypeBitLayout returns the byte offset and bit mask of the flag
// that marks a resource as a work resource within its FixedMeta record.
// Project 2013 and 2016+ share a layout; 2010 differs.
func resourceTypeBitLayout(applicationVersion int) (offset, mask int) {
	if applicationVersion > appVersionProject2010 {
		return 12, 0x10
	}
	return 9, 0x02
}

// resourceType decides between the three resource kinds. The primary
// FixedMeta record says whether the resource is a work resource at all;
// only if it is not does a second flag, in the Fixed2Meta record, separate
// cost resources from material ones.
func resourceType(metaData, metaData2 []byte, applicationVersion int) project.ResourceType {
	offset, mask := resourceTypeBitLayout(applicationVersion)
	if getByte(metaData, offset)&mask != 0 {
		return project.WorkResource
	}
	if getByte(metaData2, 8)&0x10 != 0 {
		return project.CostResource
	}
	return project.MaterialResource
}

// readResources reads the TBkndRsc storage and returns the resources it
// defines. Each resource's CalendarUniqueID is filled in by the caller from
// the resourceCalendars map readCalendars already produced, since MS
// Project links a resource to its calendar there rather than in TBkndRsc.
func readResources(src *streamSource, projectDirPath string, projectProps *Props, applicationVersion int) ([]*project.Resource, error) {
	dir := projectDirPath + "/TBkndRsc"

	varMetaRaw, err := src.plain(dir + "/VarMeta")
	if err != nil {
		return nil, err
	}
	varMeta, err := ParseVarMeta(varMetaRaw)
	if err != nil {
		return nil, fmt.Errorf("mpp: resource VarMeta: %w", err)
	}
	var2Raw, err := src.plain(dir + "/Var2Data")
	if err != nil {
		return nil, err
	}
	varData := ParseVar2Data(varMeta, var2Raw)

	fixedMetaRaw, err := src.plain(dir + "/FixedMeta")
	if err != nil {
		return nil, err
	}
	fixedMeta, err := ParseFixedMeta(fixedMetaRaw, 37)
	if err != nil {
		return nil, fmt.Errorf("mpp: resource FixedMeta: %w", err)
	}
	fixedRaw, err := src.decoded(dir + "/FixedData")
	if err != nil {
		return nil, err
	}
	fixedData := ParseFixedData(fixedMeta, fixedRaw, 512, 0)

	// Fixed2Meta carries the flag separating cost resources from material
	// ones. It is optional; without it such resources read as material.
	var fixed2Meta *FixedMeta
	if src.has(dir + "/Fixed2Meta") {
		if raw, err := src.plain(dir + "/Fixed2Meta"); err == nil {
			if meta2, err := ParseFixedMetaHeuristic(raw, fixedData.ItemCount(), 50, 51); err == nil {
				fixed2Meta = meta2
			}
		}
	}

	fm := loadFieldMap(projectProps, resourceFieldMapPropsKey1, resourceFieldMapPropsKey2)
	off := func(fieldID, defaultOffset int) int {
		return fieldOffset(fm, resourceFieldBase|fieldID, defaultOffset)
	}
	offUniqueID := off(resourceFieldIDUniqueID, resourceDefaultOffsetUniqueID)
	offID := off(resourceFieldIDID, resourceDefaultOffsetID)
	offMaxUnits := off(resourceFieldIDMaxUnits, resourceDefaultOffsetMaxUnits)
	offStandardRate := off(resourceFieldIDStandardRate, resourceDefaultOffsetStandardRate)
	offWork := off(resourceFieldIDWork, resourceDefaultOffsetWork)
	offCost := off(resourceFieldIDCost, resourceDefaultOffsetCost)
	// A record must be long enough to hold every field read below;
	// admitting a shorter one would yield a half-populated resource whose
	// missing fields silently read as zero.
	minSize := 0
	for _, end := range []int{
		offUniqueID + 4, offID + 4, offMaxUnits + 8,
		offStandardRate + 8, offWork + 8, offCost + 8,
	} {
		if end > minSize {
			minSize = end
		}
	}

	seen := make(map[int]bool)
	var resources []*project.Resource

	for i := 0; i < fixedData.ItemCount(); i++ {
		rec := fixedData.ByteArrayValue(i)
		if len(rec) < minSize {
			continue
		}

		uniqueID := getInt(rec, offUniqueID)
		if uniqueID <= 0 || seen[uniqueID] {
			continue
		}
		seen[uniqueID] = true

		var metaData2 []byte
		if fixed2Meta != nil {
			metaData2 = fixed2Meta.ByteArrayValue(i)
		}

		r := &project.Resource{
			UniqueID:     uniqueID,
			ID:           getInt(rec, offID),
			Name:         varData.UnicodeString(uniqueID, resourceNameVarType),
			Initials:     varData.UnicodeString(uniqueID, resourceInitialsVarType),
			Group:        varData.UnicodeString(uniqueID, resourceGroupVarType),
			Code:         varData.UnicodeString(uniqueID, resourceCodeVarType),
			EmailAddress: varData.UnicodeString(uniqueID, resourceEmailVarType),
			Type:         resourceType(fixedMeta.ByteArrayValue(i), metaData2, applicationVersion),
			MaxUnits:     getUnits(rec, offMaxUnits),
			StandardRate: getDouble(rec, offStandardRate),
			Work:         getWork(rec, offWork),
			Cost:         getCurrency(rec, offCost),
		}
		resources = append(resources, r)
	}

	return resources, nil
}
