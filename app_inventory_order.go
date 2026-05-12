package main

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// InventoryOrderItem is the DTO for a single weapon in the Sort Order view.
type InventoryOrderItem struct {
	Handle           uint32 `json:"handle"`
	ItemID           uint32 `json:"itemId"`
	Name             string `json:"name"`
	Category         string `json:"category"`
	AcquisitionIndex uint32 `json:"acquisitionIndex"`
	CurrentUpgrade   int    `json:"currentUpgrade,omitempty"`
	InfusionName     string `json:"infusionName,omitempty"`
	IsTechnical      bool   `json:"isTechnical,omitempty"`
}

// weaponOrderCategories is the set of item categories included in Sort Order MVP.
// Shields share handle prefix 0x80 with melee/ranged weapons, support
// infusion/AoW, and are treated as weapons throughout the codebase.
var weaponOrderCategories = map[string]bool{
	"melee_armaments":      true,
	"ranged_and_catalysts": true,
	"shields":              true,
}

// invUnarmedBaseID is the DB base ID of the "Unarmed" placeholder weapon.
// The game keeps exactly 3 Unarmed entries in CommonItems as technical slots
// for the empty-hand weapon state. They must not appear in sort order UIs.
const invUnarmedBaseID = uint32(0x0001ADB0)

func isWeaponOrderTechnical(name string, baseID uint32) bool {
	return name == "Unarmed" || baseID == invUnarmedBaseID
}

// GetWeaponInventoryOrder returns all weapons in slot charIdx's CommonItems
// inventory (not storage), sorted by AcquisitionIndex ascending.
// Categories returned: melee_armaments, ranged_and_catalysts, shields.
// Technical placeholders (Unarmed) are excluded.
func (a *App) GetWeaponInventoryOrder(charIdx int) ([]InventoryOrderItem, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return nil, fmt.Errorf("invalid character index %d", charIdx)
	}
	slot := &a.save.Slots[charIdx]
	if slot.Version == 0 {
		return nil, fmt.Errorf("slot %d is empty", charIdx)
	}

	startOff := slot.MagicOffset + core.InvStartFromMagic
	items := []InventoryOrderItem{}

	for i := 0; i < core.CommonItemCount; i++ {
		off := startOff + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		itemID, ok := slot.GaMap[h]
		if !ok {
			continue
		}
		itemData, baseID := db.GetItemDataFuzzy(itemID)
		if !weaponOrderCategories[itemData.Category] {
			continue
		}
		if isWeaponOrderTechnical(itemData.Name, baseID) {
			continue
		}

		acqIdx := binary.LittleEndian.Uint32(slot.Data[off+8:])
		upgradeLevel, infusionName := decodeWeaponUpgradeInfusion(itemID, baseID)

		items = append(items, InventoryOrderItem{
			Handle:           h,
			ItemID:           itemID,
			Name:             itemData.Name,
			Category:         itemData.Category,
			AcquisitionIndex: acqIdx,
			CurrentUpgrade:   upgradeLevel,
			InfusionName:     infusionName,
		})
	}

	sort.Slice(items, func(i, j int) bool {
		return items[i].AcquisitionIndex < items[j].AcquisitionIndex
	})

	return items, nil
}

// ReorderWeaponInventory rewrites the acquisition indices of all weapons in slot
// charIdx's CommonItems inventory so that orderedHandles[0] sorts first under
// "Kolejność zakupu / Rosnąco" in-game.
//
// orderedHandles must be the COMPLETE list of weapons from GetWeaponInventoryOrder
// — no omissions, no duplicates. Partial lists are rejected to prevent leaving
// some weapons interleaved with stale indices from the old order.
//
// Only InventoryItem.Index values are changed. GaItems, handles, quantities,
// equipped slots, AoW handles, KeyItems, and storage are untouched.
func (a *App) ReorderWeaponInventory(charIdx int, orderedHandles []uint32) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return fmt.Errorf("invalid character index %d", charIdx)
	}
	slot := &a.save.Slots[charIdx]
	if slot.Version == 0 {
		return fmt.Errorf("slot %d is empty", charIdx)
	}
	if len(orderedHandles) == 0 {
		return fmt.Errorf("orderedHandles must not be empty")
	}

	// --- Guard: no duplicates in orderedHandles ---
	seen := make(map[uint32]int, len(orderedHandles))
	for i, h := range orderedHandles {
		if prev, dup := seen[h]; dup {
			return fmt.Errorf("duplicate handle 0x%08X at positions %d and %d", h, prev, i)
		}
		seen[h] = i
	}

	startOff := slot.MagicOffset + core.InvStartFromMagic

	// --- Locate requested handles in CommonItems; validate each is a non-technical weapon ---
	type invLoc struct{ off int }
	located := make(map[uint32]invLoc, len(orderedHandles))

	for i := 0; i < core.CommonItemCount; i++ {
		off := startOff + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if _, want := seen[h]; !want {
			continue // skip slots not in the requested set
		}
		itemID, ok := slot.GaMap[h]
		if !ok {
			continue
		}
		itemData, baseID := db.GetItemDataFuzzy(itemID)
		if !weaponOrderCategories[itemData.Category] {
			return fmt.Errorf("handle 0x%08X (category %q) is not in the weapon sort order scope", h, itemData.Category)
		}
		if isWeaponOrderTechnical(itemData.Name, baseID) {
			return fmt.Errorf("handle 0x%08X is a technical placeholder (%s) and cannot be used in sort order", h, itemData.Name)
		}
		located[h] = invLoc{off: off}
	}

	// --- All requested handles must be in inventory ---
	for _, h := range orderedHandles {
		if _, ok := located[h]; !ok {
			return fmt.Errorf("handle 0x%08X not found in weapon inventory (may be in storage, or not a weapon)", h)
		}
	}

	// --- Require complete list: count all non-technical weapons in inventory ---
	totalWeapons := 0
	for i := 0; i < core.CommonItemCount; i++ {
		off := startOff + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		itemID, ok := slot.GaMap[h]
		if !ok {
			continue
		}
		itemData, baseID := db.GetItemDataFuzzy(itemID)
		if weaponOrderCategories[itemData.Category] && !isWeaponOrderTechnical(itemData.Name, baseID) {
			totalWeapons++
		}
	}
	if len(orderedHandles) != totalWeapons {
		return fmt.Errorf(
			"orderedHandles has %d weapons but inventory has %d; provide the full list from GetWeaponInventoryOrder",
			len(orderedHandles), totalWeapons,
		)
	}

	// --- Compute base index ---
	// NextAcquisitionSortId is the raw next InventoryItem.Index (writer.go assigns
	// Index = NextAcquisitionSortId, then increments by 1). Reconciled on load to
	// maxExistingIndex+1, so it is safe to use directly without multiplication.
	// Must be strictly > InvEquipReservedMax (432): equipment slots occupy 0–432.
	base := slot.Inventory.NextAcquisitionSortId
	if base <= uint32(core.InvEquipReservedMax) {
		base = uint32(core.InvEquipReservedMax) + 1
	}
	topIdx := base + uint32(len(orderedHandles)-1)

	// --- Push undo before any mutation ---
	a.pushUndo(charIdx)

	// --- Apply new indices to slot.Data and in-memory CommonItems ---
	for i, h := range orderedHandles {
		newIdx := base + uint32(i)
		loc := located[h]
		binary.LittleEndian.PutUint32(slot.Data[loc.off+8:], newIdx)
		for j := range slot.Inventory.CommonItems {
			if slot.Inventory.CommonItems[j].GaItemHandle == h {
				slot.Inventory.CommonItems[j].Index = newIdx
				break
			}
		}
	}

	// --- Update NextAcquisitionSortId and NextEquipIndex in-memory ---
	newNextAcq := topIdx + 1
	slot.Inventory.NextAcquisitionSortId = newNextAcq
	if slot.Inventory.NextEquipIndex < newNextAcq {
		slot.Inventory.NextEquipIndex = newNextAcq
	}
	// Write to slot.Data immediately; nextAcqSortIdOff is always nextEquipIndexOff+4.
	// Guard on equipIdxOff > 0: zero means the offset was never set (e.g. test fixture),
	// in which case WriteSave will pick up the in-memory value on the next serialization.
	if equipIdxOff := slot.Inventory.NextEquipIndexOff(); equipIdxOff > 0 {
		binary.LittleEndian.PutUint32(slot.Data[equipIdxOff:], slot.Inventory.NextEquipIndex)
		binary.LittleEndian.PutUint32(slot.Data[equipIdxOff+4:], slot.Inventory.NextAcquisitionSortId)
	}

	return nil
}

// decodeWeaponUpgradeInfusion extracts upgrade level and infusion name from
// the offset between itemID and baseID. Returns (0, "") for standard +0 weapons.
func decodeWeaponUpgradeInfusion(itemID, baseID uint32) (level int, infusionName string) {
	if itemID == baseID {
		return 0, ""
	}
	offset := itemID - baseID
	level = int(offset % 100)
	infIdx := int(offset / 100)
	for _, t := range db.InfuseTypes {
		if t.Offset == infIdx*100 {
			if t.Name != "Standard" {
				infusionName = t.Name
			}
			break
		}
	}
	return level, infusionName
}
