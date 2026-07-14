package core

import (
	"encoding/binary"
	"fmt"
	"sort"
)

// GaItemRepackBlocker is one fail-closed reason why a GaItem repack cannot be
// attempted. Code is stable for callers; Message is concise and user-facing.
type GaItemRepackBlocker struct {
	Code    string
	Message string
	order   int
}

// GaItemRepackPreflight is the read-only result of checking whether a slot is
// safe to repack. Analysis is populated only when all refusal gates pass.
type GaItemRepackPreflight struct {
	Analysis GaItemRepackAnalysis
	Blockers []GaItemRepackBlocker
}

// PreflightGaItemRepack validates only invariants required to preserve a slot
// during stable GaItem compaction. It never repairs or mutates slot. Structural
// and record-identity failures stop later phases; reference failures are
// aggregated in deterministic order for a useful refusal report.
func PreflightGaItemRepack(slot *SaveSlot) GaItemRepackPreflight {
	if blockers := repackStructureBlockers(slot); len(blockers) != 0 {
		return GaItemRepackPreflight{Blockers: sortRepackBlockers(blockers)}
	}

	records, blockers := scanRepackRecords(slot)
	if len(blockers) != 0 {
		return GaItemRepackPreflight{Blockers: sortRepackBlockers(blockers)}
	}

	blockers = repackReferenceBlockers(slot, records)
	if len(blockers) != 0 {
		return GaItemRepackPreflight{Blockers: sortRepackBlockers(blockers)}
	}

	return GaItemRepackPreflight{Analysis: AnalyzeGaItemRepack(slot)}
}

type repackRecord struct {
	index  int
	handle uint32
	itemID uint32
	typeID uint32
	entry  GaItemFull
}

func repackStructureBlockers(slot *SaveSlot) []GaItemRepackBlocker {
	if slot == nil || len(slot.Data) != SlotSize {
		length := 0
		if slot != nil {
			length = len(slot.Data)
		}
		return []GaItemRepackBlocker{newRepackBlocker("slot_data_size", fmt.Sprintf("slot data length %d, want %d", length, SlotSize), 0)}
	}

	var blockers []GaItemRepackBlocker
	if err := validateRepackOffsetChain(slot); err != nil {
		blockers = append(blockers, newRepackBlocker("offset_chain", err.Error(), 0))
	}
	if slot.GaItemDataOffset <= 0 || slot.GaItemDataOffset+4 > len(slot.Data) {
		blockers = append(blockers, newRepackBlocker("offset_chain", "GaItemData header is outside slot data", 1))
	}
	storageEnd := slot.StorageBoxOffset + StorageHeaderSkip + StorageCommonCount*InvRecordLen
	if slot.StorageBoxOffset <= 0 || storageEnd > len(slot.Data) {
		blockers = append(blockers, newRepackBlocker("offset_chain", "Storage records are outside slot data", 2))
	}
	if err := validateSectionMap(slot.SectionMap); err != nil {
		blockers = append(blockers, newRepackBlocker("section_map", err.Error(), 0))
	}
	return blockers
}

// validateRepackOffsetChain mirrors the read-only safety-relevant checks of
// SaveSlot.validateOffsetChain. The latter may normalize EventFlagsOffset and
// append a warning, which would violate the dry-run contract.
func validateRepackOffsetChain(slot *SaveSlot) error {
	type check struct {
		name   string
		offset int
		minVal int
		maxVal int
	}

	checks := []check{
		{"MagicOffset", slot.MagicOffset, MinMagicOffset, SlotSize},
		{"InventoryEnd", slot.InventoryEnd, GaItemsStart, slot.MagicOffset - DynPlayerData + 2},
		{"PlayerDataOffset", slot.PlayerDataOffset, slot.MagicOffset, SlotSize},
		{"FaceDataOffset", slot.FaceDataOffset, slot.PlayerDataOffset, SlotSize},
		{"StorageBoxOffset", slot.StorageBoxOffset, slot.FaceDataOffset, SlotSize},
	}
	for _, check := range checks {
		if check.offset < check.minVal || check.offset >= check.maxVal {
			return fmt.Errorf("offset %s = 0x%X out of expected range [0x%X, 0x%X)",
				check.name, check.offset, check.minVal, check.maxVal)
		}
	}
	if !(slot.InventoryEnd <= slot.MagicOffset &&
		slot.MagicOffset <= slot.PlayerDataOffset &&
		slot.PlayerDataOffset < slot.FaceDataOffset &&
		slot.FaceDataOffset <= slot.StorageBoxOffset) {
		return fmt.Errorf("offset chain order violated: InventoryEnd=0x%X MagicOffset=0x%X PlayerData=0x%X FaceData=0x%X StorageBox=0x%X",
			slot.InventoryEnd, slot.MagicOffset, slot.PlayerDataOffset, slot.FaceDataOffset, slot.StorageBoxOffset)
	}
	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset >= SlotSize {
		return fmt.Errorf("EventFlagsOffset 0x%X >= SlotSize", slot.EventFlagsOffset)
	}
	if slot.GaItemDataOffset > 0 && slot.GaItemDataOffset >= SlotSize {
		return fmt.Errorf("GaItemDataOffset 0x%X >= SlotSize", slot.GaItemDataOffset)
	}
	return nil
}

func scanRepackRecords(slot *SaveSlot) ([]repackRecord, []GaItemRepackBlocker) {
	var records []repackRecord
	var blockers []GaItemRepackBlocker
	seenHandles := make(map[uint32]int)

	for i, entry := range slot.GaItems {
		if entry.IsEmpty() {
			continue
		}
		typeID := entry.Handle & GaHandleTypeMask
		switch typeID {
		case ItemTypeWeapon, ItemTypeArmor, ItemTypeAccessory, ItemTypeItem, ItemTypeAow:
		default:
			blockers = append(blockers, newRepackBlocker(
				"unknown_handle_type",
				fmt.Sprintf("GaItem[%d] has unknown handle type 0x%X", i, typeID),
				i,
			))
			continue
		}
		if first, exists := seenHandles[entry.Handle]; exists {
			blockers = append(blockers, newRepackBlocker(
				"duplicate_handle",
				fmt.Sprintf("GaItem[%d] reuses handle 0x%08X from GaItem[%d]", i, entry.Handle, first),
				i,
			))
			continue
		}
		seenHandles[entry.Handle] = i
		records = append(records, repackRecord{index: i, handle: entry.Handle, itemID: entry.ItemID, typeID: typeID, entry: entry})
	}

	if !validRepackGaItemIndices(slot, len(records)) {
		blockers = append(blockers, newRepackBlocker(
			"gaitem_indices",
			fmt.Sprintf("NextAoWIndex=%d NextArmamentIndex=%d len(GaItems)=%d", slot.NextAoWIndex, slot.NextArmamentIndex, len(slot.GaItems)),
			len(slot.GaItems),
		))
	}
	return records, blockers
}

func validRepackGaItemIndices(slot *SaveSlot, recordCount int) bool {
	if len(slot.GaItems) == 0 && recordCount == 0 {
		return slot.NextAoWIndex >= 0 && slot.NextAoWIndex <= 1 && slot.NextArmamentIndex == 1
	}
	return slot.NextAoWIndex >= 0 &&
		slot.NextArmamentIndex >= 0 &&
		slot.NextAoWIndex <= slot.NextArmamentIndex &&
		slot.NextArmamentIndex <= len(slot.GaItems)
}

func repackReferenceBlockers(slot *SaveSlot, records []repackRecord) []GaItemRepackBlocker {
	physical := make(map[uint32]repackRecord, len(records))
	aowRecords := make(map[uint32]struct{})
	for _, record := range records {
		physical[record.handle] = record
		if record.typeID == ItemTypeAow {
			aowRecords[record.handle] = struct{}{}
		}
	}

	var blockers []GaItemRepackBlocker
	for handle, itemID := range slot.GaMap {
		if itemID == 0 {
			blockers = append(blockers, newRepackBlocker("gamap_zero_id", fmt.Sprintf("GaMap handle 0x%08X maps to itemID=0", handle), int(handle)))
		}
	}
	for _, record := range records {
		itemID, ok := slot.GaMap[record.handle]
		if !ok || itemID != record.itemID {
			blockers = append(blockers, newRepackBlocker(
				"gamap_record_mismatch",
				fmt.Sprintf("GaItem[%d] handle 0x%08X does not match GaMap", record.index, record.handle),
				record.index,
			))
		}
	}
	for handle := range slot.GaMap {
		if _, exists := physical[handle]; exists {
			continue
		}
		typeID := handle & GaHandleTypeMask
		if typeID != ItemTypeAccessory && typeID != ItemTypeItem {
			blockers = append(blockers, newRepackBlocker(
				"orphan_gamap_entry",
				fmt.Sprintf("GaMap handle 0x%08X has no physical GaItem record", handle),
				int(handle),
			))
		}
	}

	blockers = append(blockers, repackContainerBlockers(slot.Inventory.CommonItems, "inventory", physical, slot.GaMap)...)
	blockers = append(blockers, repackContainerBlockers(slot.Inventory.KeyItems, "inventory key items", physical, slot.GaMap)...)
	blockers = append(blockers, repackContainerBlockers(slot.Storage.CommonItems, "storage", physical, slot.GaMap)...)
	blockers = append(blockers, repackAoWBlockers(records, aowRecords)...)

	for _, issue := range ScanDuplicateInventoryIndices(slot) {
		blockers = append(blockers, newRepackBlocker(
			"duplicate_index",
			fmt.Sprintf("duplicate inventory index %d in %s", issue.Index, issue.Scope),
			issue.DuplicateRow,
		))
	}

	headerCount := binary.LittleEndian.Uint32(slot.Data[slot.StorageBoxOffset:])
	actualStorageCount := uint32(0)
	storageStart := slot.StorageBoxOffset + StorageHeaderSkip
	for i := 0; i < StorageCommonCount; i++ {
		off := storageStart + i*InvRecordLen
		handle := binary.LittleEndian.Uint32(slot.Data[off:])
		if handle != GaHandleEmpty && handle != GaHandleInvalid {
			actualStorageCount++
		}
	}
	if headerCount != actualStorageCount {
		blockers = append(blockers, newRepackBlocker(
			"storage_count",
			fmt.Sprintf("storage header count %d != actual items %d", headerCount, actualStorageCount),
			0,
		))
	}

	gaItemDataCount := binary.LittleEndian.Uint32(slot.Data[slot.GaItemDataOffset:])
	if gaItemDataCount > GaItemDataMaxCount {
		blockers = append(blockers, newRepackBlocker(
			"gaitemdata_count",
			fmt.Sprintf("GaItemData count %d > max %d", gaItemDataCount, GaItemDataMaxCount),
			0,
		))
	}
	return blockers
}

func repackContainerBlockers(items []InventoryItem, scope string, physical map[uint32]repackRecord, gaMap map[uint32]uint32) []GaItemRepackBlocker {
	var blockers []GaItemRepackBlocker
	for i, item := range items {
		handle := item.GaItemHandle
		if handle == GaHandleEmpty || handle == GaHandleInvalid {
			continue
		}
		switch handle & GaHandleTypeMask {
		case ItemTypeWeapon, ItemTypeArmor, ItemTypeAow:
			record, physicalOK := physical[handle]
			mappedID, mapOK := gaMap[handle]
			if !physicalOK || !mapOK || mappedID != record.itemID {
				code := "orphan_inventory_handle"
				if scope == "storage" {
					code = "orphan_storage_handle"
				}
				blockers = append(blockers, newRepackBlocker(
					code,
					fmt.Sprintf("%s[%d] handle 0x%08X does not resolve to a matching GaItem", scope, i, handle),
					i,
				))
			}
		case ItemTypeAccessory, ItemTypeItem:
			if itemID, ok := gaMap[handle]; !ok || itemID == 0 {
				blockers = append(blockers, newRepackBlocker(
					"unresolved_stackable_handle",
					fmt.Sprintf("%s[%d] stackable handle 0x%08X is not in GaMap", scope, i, handle),
					i,
				))
			}
		}
	}
	return blockers
}

func repackAoWBlockers(records []repackRecord, aowRecords map[uint32]struct{}) []GaItemRepackBlocker {
	refs := make(map[uint32][]int)
	for _, record := range records {
		if record.typeID != ItemTypeWeapon || IsNoCustomAoWHandle(record.entry.AoWGaItemHandle) {
			continue
		}
		handle := record.entry.AoWGaItemHandle
		refs[handle] = append(refs[handle], record.index)
	}

	var blockers []GaItemRepackBlocker
	for handle, weaponIndices := range refs {
		if _, exists := aowRecords[handle]; !exists {
			for _, weaponIndex := range weaponIndices {
				blockers = append(blockers, newRepackBlocker(
					"dangling_aow_handle",
					fmt.Sprintf("GaItem[%d] references missing Ash of War handle 0x%08X", weaponIndex, handle),
					weaponIndex,
				))
			}
		}
		if len(weaponIndices) > 1 {
			for _, weaponIndex := range weaponIndices[1:] {
				blockers = append(blockers, newRepackBlocker(
					"shared_aow_handle",
					fmt.Sprintf("Ash of War handle 0x%08X is referenced by multiple weapons", handle),
					weaponIndex,
				))
			}
		}
	}
	return blockers
}

func newRepackBlocker(code, message string, order int) GaItemRepackBlocker {
	return GaItemRepackBlocker{Code: code, Message: message, order: order}
}

func sortRepackBlockers(blockers []GaItemRepackBlocker) []GaItemRepackBlocker {
	sort.Slice(blockers, func(i, j int) bool {
		if blockers[i].Code != blockers[j].Code {
			return blockers[i].Code < blockers[j].Code
		}
		if blockers[i].order != blockers[j].order {
			return blockers[i].order < blockers[j].order
		}
		return blockers[i].Message < blockers[j].Message
	})
	return blockers
}
