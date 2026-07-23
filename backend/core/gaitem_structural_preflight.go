package core

import (
	"encoding/binary"
	"fmt"
	"sort"
	"strings"
)

// GaItemStructuralIssue is one fail-closed GaItem integrity problem. Code is
// stable for callers; Message is concise and user-facing.
// Handle carries the offending GaItem handle for issues that identify one
// structurally (currently only "duplicate_handle" for a physical GaItem
// duplicate); it is 0 for issues that do not name a handle.
type GaItemStructuralIssue struct {
	Code    string
	Message string
	Handle  uint32
	order   int
}

// GaItemStructuralReport is the read-only result of validating persisted
// GaItem records and every reference needed by duplicate repair.
type GaItemStructuralReport struct {
	Issues []GaItemStructuralIssue
}

// ScanGaItemStructuralIssues validates the invariants required to repair a
// physical duplicate safely. It never repairs or mutates slot. Structural and
// record-identity failures stop later phases; reference failures are aggregated
// in deterministic order for a useful refusal report.
func ScanGaItemStructuralIssues(slot *SaveSlot) GaItemStructuralReport {
	if issues := gaItemStructureIssues(slot); len(issues) != 0 {
		return GaItemStructuralReport{Issues: sortGaItemStructuralIssues(issues)}
	}

	records, issues := scanGaItemStructuralRecords(slot)
	if len(issues) != 0 {
		return GaItemStructuralReport{Issues: sortGaItemStructuralIssues(issues)}
	}

	issues = gaItemReferenceIssues(slot, records)
	if len(issues) != 0 {
		return GaItemStructuralReport{Issues: sortGaItemStructuralIssues(issues)}
	}

	return GaItemStructuralReport{}
}

type gaItemStructuralRecord struct {
	index  int
	handle uint32
	itemID uint32
	typeID uint32
	entry  GaItemFull
}

func gaItemStructureIssues(slot *SaveSlot) []GaItemStructuralIssue {
	if slot == nil || len(slot.Data) != SlotSize {
		length := 0
		if slot != nil {
			length = len(slot.Data)
		}
		return []GaItemStructuralIssue{newGaItemStructuralIssue("slot_data_size", fmt.Sprintf("slot data length %d, want %d", length, SlotSize), 0)}
	}

	var issues []GaItemStructuralIssue
	if err := validateGaItemOffsetChain(slot); err != nil {
		issues = append(issues, newGaItemStructuralIssue("offset_chain", err.Error(), 0))
	}
	if slot.GaItemDataOffset <= 0 || slot.GaItemDataOffset+4 > len(slot.Data) {
		issues = append(issues, newGaItemStructuralIssue("offset_chain", "GaItemData header is outside slot data", 1))
	}
	storageEnd := slot.StorageBoxOffset + StorageHeaderSkip + StorageCommonCount*InvRecordLen
	if slot.StorageBoxOffset <= 0 || storageEnd > len(slot.Data) {
		issues = append(issues, newGaItemStructuralIssue("offset_chain", "Storage records are outside slot data", 2))
	}
	if err := validateSectionMap(slot.SectionMap); err != nil {
		issues = append(issues, newGaItemStructuralIssue("section_map", err.Error(), 0))
	}
	return issues
}

// validateGaItemOffsetChain mirrors the read-only safety-relevant checks of
// SaveSlot.validateOffsetChain. The latter may normalize EventFlagsOffset and
// append a warning, which would violate the dry-run contract.
func validateGaItemOffsetChain(slot *SaveSlot) error {
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

func scanGaItemStructuralRecords(slot *SaveSlot) ([]gaItemStructuralRecord, []GaItemStructuralIssue) {
	var records []gaItemStructuralRecord
	var issues []GaItemStructuralIssue
	seenHandles := make(map[uint32]int)

	for i, entry := range slot.GaItems {
		if entry.IsEmpty() {
			continue
		}
		typeID := entry.Handle & GaHandleTypeMask
		switch typeID {
		case ItemTypeWeapon, ItemTypeArmor, ItemTypeAccessory, ItemTypeItem, ItemTypeAow:
		default:
			issues = append(issues, newGaItemStructuralIssue(
				"unknown_handle_type",
				fmt.Sprintf("GaItem[%d] has unknown handle type 0x%X", i, typeID),
				i,
			))
			continue
		}
		if first, exists := seenHandles[entry.Handle]; exists {
			issue := newGaItemStructuralIssue(
				"duplicate_handle",
				fmt.Sprintf("GaItem[%d] reuses handle 0x%08X from GaItem[%d]", i, entry.Handle, first),
				i,
			)
			issue.Handle = entry.Handle
			issues = append(issues, issue)
			continue
		}
		seenHandles[entry.Handle] = i
		records = append(records, gaItemStructuralRecord{index: i, handle: entry.Handle, itemID: entry.ItemID, typeID: typeID, entry: entry})
	}

	if !validGaItemIndices(slot, len(records)) {
		issues = append(issues, newGaItemStructuralIssue(
			"gaitem_indices",
			fmt.Sprintf("NextAoWIndex=%d NextArmamentIndex=%d len(GaItems)=%d", slot.NextAoWIndex, slot.NextArmamentIndex, len(slot.GaItems)),
			len(slot.GaItems),
		))
	}
	return records, issues
}

func validGaItemIndices(slot *SaveSlot, recordCount int) bool {
	if len(slot.GaItems) == 0 && recordCount == 0 {
		return slot.NextAoWIndex >= 0 && slot.NextAoWIndex <= 1 && slot.NextArmamentIndex == 1
	}
	return slot.NextAoWIndex >= 0 &&
		slot.NextArmamentIndex >= 0 &&
		slot.NextAoWIndex <= slot.NextArmamentIndex &&
		slot.NextArmamentIndex <= len(slot.GaItems)
}

func gaItemReferenceIssues(slot *SaveSlot, records []gaItemStructuralRecord) []GaItemStructuralIssue {
	physical := make(map[uint32]gaItemStructuralRecord, len(records))
	aowRecords := make(map[uint32]struct{})
	for _, record := range records {
		physical[record.handle] = record
		if record.typeID == ItemTypeAow {
			aowRecords[record.handle] = struct{}{}
		}
	}

	var issues []GaItemStructuralIssue
	for handle, itemID := range slot.GaMap {
		if itemID == 0 {
			issues = append(issues, newGaItemStructuralIssue("gamap_zero_id", fmt.Sprintf("GaMap handle 0x%08X maps to itemID=0", handle), int(handle)))
		}
	}
	for _, record := range records {
		itemID, ok := slot.GaMap[record.handle]
		if !ok || itemID != record.itemID {
			issues = append(issues, newGaItemStructuralIssue(
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
			issues = append(issues, newGaItemStructuralIssue(
				"orphan_gamap_entry",
				fmt.Sprintf("GaMap handle 0x%08X has no physical GaItem record", handle),
				int(handle),
			))
		}
	}

	issues = append(issues, gaItemContainerIssues(slot.Inventory.CommonItems, "inventory", physical, slot.GaMap)...)
	issues = append(issues, gaItemContainerIssues(slot.Inventory.KeyItems, "inventory key items", physical, slot.GaMap)...)
	issues = append(issues, gaItemContainerIssues(slot.Storage.CommonItems, "storage", physical, slot.GaMap)...)
	issues = append(issues, gaItemAoWIssues(records, aowRecords)...)

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
		issues = append(issues, newGaItemStructuralIssue(
			"storage_count",
			fmt.Sprintf("storage header count %d != actual items %d", headerCount, actualStorageCount),
			0,
		))
	}

	gaItemDataCount := binary.LittleEndian.Uint32(slot.Data[slot.GaItemDataOffset:])
	if gaItemDataCount > GaItemDataMaxCount {
		issues = append(issues, newGaItemStructuralIssue(
			"gaitemdata_count",
			fmt.Sprintf("GaItemData count %d > max %d", gaItemDataCount, GaItemDataMaxCount),
			0,
		))
	}
	return issues
}

func gaItemContainerIssues(items []InventoryItem, scope string, physical map[uint32]gaItemStructuralRecord, gaMap map[uint32]uint32) []GaItemStructuralIssue {
	var issues []GaItemStructuralIssue
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
				issues = append(issues, newGaItemStructuralIssue(
					code,
					fmt.Sprintf("%s[%d] handle 0x%08X does not resolve to a matching GaItem", scope, i, handle),
					i,
				))
			}
		case ItemTypeAccessory, ItemTypeItem:
			// Talismans and goods are handle-encoded and normally have no
			// physical GaItem or GaMap entry. A present zero-valued GaMap entry
			// is still rejected by the global gamap_zero_id preflight check.
		}
	}
	return issues
}

func gaItemAoWIssues(records []gaItemStructuralRecord, aowRecords map[uint32]struct{}) []GaItemStructuralIssue {
	refs := make(map[uint32][]int)
	for _, record := range records {
		if record.typeID != ItemTypeWeapon || IsNoCustomAoWHandle(record.entry.AoWGaItemHandle) {
			continue
		}
		handle := record.entry.AoWGaItemHandle
		refs[handle] = append(refs[handle], record.index)
	}

	var issues []GaItemStructuralIssue
	for handle, weaponIndices := range refs {
		if _, exists := aowRecords[handle]; !exists {
			for _, weaponIndex := range weaponIndices {
				issues = append(issues, newGaItemStructuralIssue(
					"dangling_aow_handle",
					fmt.Sprintf("GaItem[%d] references missing Ash of War handle 0x%08X", weaponIndex, handle),
					weaponIndex,
				))
			}
		}
		if len(weaponIndices) > 1 {
			for _, weaponIndex := range weaponIndices[1:] {
				issues = append(issues, newGaItemStructuralIssue(
					"shared_aow_handle",
					fmt.Sprintf("Ash of War handle 0x%08X is referenced by multiple weapons", handle),
					weaponIndex,
				))
			}
		}
	}
	return issues
}

func newGaItemStructuralIssue(code, message string, order int) GaItemStructuralIssue {
	return GaItemStructuralIssue{Code: code, Message: message, order: order}
}

func sortGaItemStructuralIssues(issues []GaItemStructuralIssue) []GaItemStructuralIssue {
	sort.Slice(issues, func(i, j int) bool {
		if issues[i].Code != issues[j].Code {
			return issues[i].Code < issues[j].Code
		}
		if issues[i].order != issues[j].order {
			return issues[i].order < issues[j].order
		}
		return issues[i].Message < issues[j].Message
	})
	return issues
}

func formatGaItemStructuralIssues(issues []GaItemStructuralIssue) string {
	parts := make([]string, len(issues))
	for i, issue := range issues {
		parts[i] = issue.Code + ": " + issue.Message
	}
	return strings.Join(parts, "; ")
}
