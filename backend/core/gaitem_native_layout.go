package core

import (
	"fmt"
	"sort"
)

const gaItemHandleValidBit uint32 = 0x00800000

type nativeGaItemLayout struct {
	used       []bool
	aowIndices []int
	partID     uint8
}

func isPhysicalGaItemPrefix(prefix uint32) bool {
	return prefix == ItemTypeWeapon || prefix == ItemTypeArmor || prefix == ItemTypeAow
}

func physicalGaItemTypeMatchesItemID(prefix, itemID uint32) bool {
	itemClass := itemID >> 28
	return (prefix == ItemTypeWeapon && itemClass == 0) ||
		(prefix == ItemTypeArmor && itemClass == 1) ||
		(prefix == ItemTypeAow && itemClass == 8)
}

// analyzeNativeGaItemLayout verifies the exact two-pass projection used by the
// game writer. It deliberately derives placement from physical handle indices,
// never from SaveForge's legacy cursor fields.
func analyzeNativeGaItemLayout(slot *SaveSlot) (nativeGaItemLayout, error) {
	if slot == nil {
		return nativeGaItemLayout{}, fmt.Errorf("native GaItem projection: nil slot")
	}

	layout := nativeGaItemLayout{used: make([]bool, len(slot.GaItems))}
	itemByHandle := make(map[uint32]uint32)
	hasPartID := false

	for position, record := range slot.GaItems {
		if record.IsEmpty() {
			continue
		}
		prefix := record.Handle & GaHandleTypeMask
		if !isPhysicalGaItemPrefix(prefix) {
			return nativeGaItemLayout{}, fmt.Errorf("native GaItem projection: record %d has non-physical handle 0x%08X", position, record.Handle)
		}
		if !physicalGaItemTypeMatchesItemID(prefix, record.ItemID) {
			return nativeGaItemLayout{}, fmt.Errorf("native GaItem projection: record %d handle type 0x%X conflicts with item ID 0x%08X", position, prefix, record.ItemID)
		}
		if record.Handle&gaItemHandleValidBit == 0 {
			return nativeGaItemLayout{}, fmt.Errorf("native GaItem projection: record %d handle 0x%08X lacks validity bit", position, record.Handle)
		}
		index := int(record.Handle & 0xFFFF)
		if index >= len(slot.GaItems) {
			return nativeGaItemLayout{}, fmt.Errorf("native GaItem projection: record %d physical index %d outside table length %d", position, index, len(slot.GaItems))
		}
		if layout.used[index] {
			return nativeGaItemLayout{}, fmt.Errorf("native GaItem projection: duplicate physical index %d", index)
		}
		layout.used[index] = true
		itemByHandle[record.Handle] = record.ItemID

		partID := uint8(record.Handle >> 16)
		if !hasPartID {
			layout.partID = partID
			hasPartID = true
		}
		if prefix == ItemTypeAow {
			layout.aowIndices = append(layout.aowIndices, index)
		}
	}

	sort.Ints(layout.aowIndices)
	for position, record := range slot.GaItems {
		if record.IsEmpty() {
			continue
		}
		index := int(record.Handle & 0xFFFF)
		isAoW := record.Handle&GaHandleTypeMask == ItemTypeAow
		expected := layout.position(index, isAoW)
		if position != expected {
			return nativeGaItemLayout{}, fmt.Errorf("native GaItem projection: handle 0x%08X is at record %d, want %d", record.Handle, position, expected)
		}
	}

	if !hasPartID {
		layout.partID = slot.PartGaItemHandle
		if layout.partID == 0 {
			layout.partID = uint8(gaItemHandleValidBit >> 16)
		}
	}
	if uint32(layout.partID)<<16&gaItemHandleValidBit == 0 {
		return nativeGaItemLayout{}, fmt.Errorf("native GaItem projection: generation byte 0x%02X lacks validity bit", layout.partID)
	}

	for position, record := range slot.GaItems {
		if record.IsEmpty() || record.Handle&GaHandleTypeMask != ItemTypeWeapon || IsNoCustomAoWHandle(record.AoWGaItemHandle) {
			continue
		}
		if record.AoWGaItemHandle&GaHandleTypeMask != ItemTypeAow {
			return nativeGaItemLayout{}, fmt.Errorf("native GaItem projection: weapon record %d embeds non-AoW handle 0x%08X", position, record.AoWGaItemHandle)
		}
		if _, ok := itemByHandle[record.AoWGaItemHandle]; !ok {
			return nativeGaItemLayout{}, fmt.Errorf("native GaItem projection: weapon record %d embeds missing AoW handle 0x%08X", position, record.AoWGaItemHandle)
		}
	}

	if err := validatePhysicalGaItemReferences(slot, itemByHandle); err != nil {
		return nativeGaItemLayout{}, err
	}
	return layout, nil
}

func (layout nativeGaItemLayout) position(index int, isAoW bool) int {
	aowBefore := sort.SearchInts(layout.aowIndices, index)
	if isAoW {
		return aowBefore
	}
	return len(layout.aowIndices) + index - aowBefore
}

func (layout nativeGaItemLayout) firstFreeIndex() (int, bool) {
	for index, used := range layout.used {
		if !used {
			return index, true
		}
	}
	return 0, false
}

// NativeGaItemCapacity validates the persisted two-pass layout and reports the
// capacity available to the hole allocator. All physical holes are usable;
// there is no separate monotonic cursor limit in the native model.
func NativeGaItemCapacity(slot *SaveSlot) (int, error) {
	layout, err := analyzeNativeGaItemLayout(slot)
	if err != nil {
		return 0, err
	}
	free := 0
	for _, used := range layout.used {
		if !used {
			free++
		}
	}
	return free, nil
}

func validatePhysicalGaItemReferences(slot *SaveSlot, items map[uint32]uint32) error {
	check := func(scope string, row int, handle uint32) error {
		if handle == GaHandleEmpty || handle == GaHandleInvalid || !isPhysicalGaItemPrefix(handle&GaHandleTypeMask) {
			return nil
		}
		if _, ok := items[handle]; !ok {
			return fmt.Errorf("native GaItem projection: %s[%d] references physical handle 0x%08X without a GaItem record", scope, row, handle)
		}
		return nil
	}
	for row, item := range slot.Inventory.CommonItems {
		if err := check("inventory_common", row, item.GaItemHandle); err != nil {
			return err
		}
	}
	for row, item := range slot.Inventory.KeyItems {
		if err := check("inventory_key", row, item.GaItemHandle); err != nil {
			return err
		}
	}
	for row, item := range slot.Storage.CommonItems {
		if err := check("storage_common", row, item.GaItemHandle); err != nil {
			return err
		}
	}
	for handle, itemID := range slot.GaMap {
		if physicalItemID, ok := items[handle]; isPhysicalGaItemPrefix(handle&GaHandleTypeMask) && !ok {
			return fmt.Errorf("native GaItem projection: GaMap references physical handle 0x%08X without a GaItem record", handle)
		} else if ok && itemID != physicalItemID {
			return fmt.Errorf("native GaItem projection: GaMap item ID 0x%08X for handle 0x%08X differs from GaItem 0x%08X", itemID, handle, physicalItemID)
		}
	}
	return nil
}

func refreshGaItemTracking(slot *SaveSlot) {
	slot.NextAoWIndex = 0
	slot.NextArmamentIndex = 0
	slot.NextGaItemHandle = 0
	slot.PartGaItemHandle = uint8(gaItemHandleValidBit >> 16)
	maxIndex := uint32(0)
	maxPosition := 0
	haveRecord := false

	for position, record := range slot.GaItems {
		if record.IsEmpty() {
			continue
		}
		if !haveRecord {
			slot.PartGaItemHandle = uint8(record.Handle >> 16)
		}
		haveRecord = true
		if record.Handle&GaHandleTypeMask == ItemTypeAow {
			slot.NextAoWIndex = position + 1
		}
		index := record.Handle & 0xFFFF
		if index >= maxIndex {
			maxIndex = index
			maxPosition = position
		}
	}
	if haveRecord {
		slot.NextArmamentIndex = maxPosition + 1
		slot.NextGaItemHandle = maxIndex + 1
	}
}
