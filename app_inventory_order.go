package main

import (
	"encoding/binary"
	"fmt"
	"log"
	"sort"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// InventoryOrderItem is the DTO for a single item in the Sort Order view.
type InventoryOrderItem struct {
	Handle           uint32  `json:"handle"`
	ItemID           uint32  `json:"itemId"`
	Name             string  `json:"name"`
	Category         string  `json:"category"`
	AcquisitionIndex uint32  `json:"acquisitionIndex"`
	Weight           float64 `json:"weight,omitempty"`
	SortId           uint32  `json:"sortId,omitempty"`
	SortGroupId      uint8   `json:"sortGroupId,omitempty"`
	CurrentUpgrade   int     `json:"currentUpgrade,omitempty"`
	MaxUpgrade       uint32  `json:"maxUpgrade,omitempty"`
	InfusionName     string  `json:"infusionName,omitempty"`
	IconPath         string  `json:"iconPath,omitempty"`
	IsTechnical      bool    `json:"isTechnical,omitempty"`
}

// inventoryOrderTabs maps Sort Order tab names to their item categories.
var inventoryOrderTabs = map[string][]string{
	"weapons":   {"melee_armaments", "ranged_and_catalysts", "shields"},
	"talismans": {"talismans"},
	"head":      {"head"},
	"chest":     {"chest"},
	"arms":      {"arms"},
	"legs":      {"legs"},
}

// tabLabel maps a Sort Order tab to its singular human-readable label for error messages.
var tabLabel = map[string]string{
	"weapons":   "weapon",
	"talismans": "talisman",
	"head":      "head",
	"chest":     "chest",
	"arms":      "arm",
	"legs":      "leg",
}

// invUnarmedBaseID is the DB base ID of the "Unarmed" placeholder weapon.
// The game keeps exactly 3 Unarmed entries in CommonItems as technical slots
// for the empty-hand weapon state. They must not appear in sort order UIs.
const invUnarmedBaseID = uint32(0x0001ADB0)

func isWeaponOrderTechnical(name string, baseID uint32) bool {
	return name == "Unarmed" || baseID == invUnarmedBaseID
}

// tabCategorySet builds a category→bool lookup for a given tab.
// Returns an error for unknown tab names.
func tabCategorySet(tab string) (map[string]bool, error) {
	cats, ok := inventoryOrderTabs[tab]
	if !ok {
		return nil, fmt.Errorf("unknown sort order tab %q", tab)
	}
	m := make(map[string]bool, len(cats))
	for _, c := range cats {
		m[c] = true
	}
	return m, nil
}

// GetInventoryOrder returns all items in slot charIdx's CommonItems inventory
// for the given Sort Order tab, sorted by AcquisitionIndex ascending.
//
// Valid tab values: "weapons", "talismans", "head", "chest", "arms", "legs".
// The weapons tab excludes technical Unarmed placeholders.
// Storage items are never included.
func (a *App) GetInventoryOrder(charIdx int, tab string) ([]InventoryOrderItem, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return nil, fmt.Errorf("invalid character index %d", charIdx)
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	return a.getInventoryOrderLocked(charIdx, tab)
}

// getInventoryOrderLocked is the internal read-only worker for
// GetInventoryOrder and GetWeaponInventoryOrder.
//
// Contract: caller MUST have validated `a.save != nil` and `charIdx` in
// range, and MUST hold exclusive access to slot[charIdx]. In the upcoming
// lock phase the caller will hold saveMu.RLock + slotMu[charIdx]. The
// helper performs only reads — no slot mutation, no pushUndo.
func (a *App) getInventoryOrderLocked(charIdx int, tab string) ([]InventoryOrderItem, error) {
	categories, err := tabCategorySet(tab)
	if err != nil {
		return nil, err
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
		if !categories[itemData.Category] {
			continue
		}
		if tab == "weapons" && isWeaponOrderTechnical(itemData.Name, baseID) {
			continue
		}

		acqIdx := binary.LittleEndian.Uint32(slot.Data[off+8:])

		var upgradeLevel int
		var infusionName string
		if tab == "weapons" {
			upgradeLevel, infusionName = decodeWeaponUpgradeInfusion(itemID, baseID)
		}

		sk := data.ItemSortKeys[baseID]
		items = append(items, InventoryOrderItem{
			Handle:           h,
			ItemID:           itemID,
			Name:             itemData.Name,
			Category:         itemData.Category,
			AcquisitionIndex: acqIdx,
			Weight:           data.ItemWeights[baseID],
			SortId:           sk.SortId,
			SortGroupId:      sk.SortGroupId,
			CurrentUpgrade:   upgradeLevel,
			MaxUpgrade:       itemData.MaxUpgrade,
			InfusionName:     infusionName,
			IconPath:         itemData.IconPath,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].AcquisitionIndex < items[j].AcquisitionIndex
	})
	return items, nil
}

// GetStorageOrder returns all items in slot charIdx's Storage CommonItems for
// the given Sort Order tab, sorted by record Index (acquisition) ascending.
// Mirrors GetInventoryOrder but operates on Storage. Read-only — there is no
// ReorderStorage / Apply Order for the storage box.
//
// Index is the in-record Index field — for storage records this is assigned
// monotonically by addToInventory/MoveItemsBetweenContainers as records are
// created (NextEquipIndex, NextAcquisitionSortId). Transferred items naturally
// receive the highest Index and therefore appear at the end of Acquisition ↑.
func (a *App) GetStorageOrder(charIdx int, tab string) ([]InventoryOrderItem, error) {
	categories, err := tabCategorySet(tab)
	if err != nil {
		return nil, err
	}
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return nil, fmt.Errorf("invalid character index %d", charIdx)
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	slot := &a.save.Slots[charIdx]
	if slot.Version == 0 {
		return nil, fmt.Errorf("slot %d is empty", charIdx)
	}
	if slot.StorageBoxOffset <= 0 {
		return []InventoryOrderItem{}, nil
	}

	startOff := slot.StorageBoxOffset + core.StorageHeaderSkip
	items := []InventoryOrderItem{}

	for i := 0; i < core.StorageCommonCount; i++ {
		off := startOff + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		// Resolve itemID: prefer GaMap (set by allocateGaItem for weapons/armor/
		// AoW and for rehandled instances). Fall back to HandleToItemID for
		// vanilla handle-encoded talisman/goods records whose GaMap entry was
		// dropped on parseFromData re-scan.
		itemID, ok := slot.GaMap[h]
		if !ok {
			itemID = db.HandleToItemID(h)
		}
		itemData, baseID := db.GetItemDataFuzzy(itemID)
		if itemData.Name == "" {
			continue
		}
		if !categories[itemData.Category] {
			continue
		}
		if tab == "weapons" && isWeaponOrderTechnical(itemData.Name, baseID) {
			continue
		}

		acqIdx := binary.LittleEndian.Uint32(slot.Data[off+8:])

		var upgradeLevel int
		var infusionName string
		if tab == "weapons" {
			upgradeLevel, infusionName = decodeWeaponUpgradeInfusion(itemID, baseID)
		}

		sk := data.ItemSortKeys[baseID]
		items = append(items, InventoryOrderItem{
			Handle:           h,
			ItemID:           itemID,
			Name:             itemData.Name,
			Category:         itemData.Category,
			AcquisitionIndex: acqIdx,
			Weight:           data.ItemWeights[baseID],
			SortId:           sk.SortId,
			SortGroupId:      sk.SortGroupId,
			CurrentUpgrade:   upgradeLevel,
			MaxUpgrade:       itemData.MaxUpgrade,
			InfusionName:     infusionName,
			IconPath:         itemData.IconPath,
		})
	}

	sort.SliceStable(items, func(i, j int) bool {
		return items[i].AcquisitionIndex < items[j].AcquisitionIndex
	})
	return items, nil
}

// ReorderInventory rewrites the acquisition indices of all items in slot charIdx's
// CommonItems inventory for the given tab so that orderedHandles[0] sorts first
// under "Kolejność zakupu / Rosnąco" in-game.
//
// Uses stride-2 indexing: each item at position pos receives index base + pos*2,
// where base is the next available even number above NextAcquisitionSortId. This
// ensures every item has a unique acqIdx>>1 bucket key, matching the game's
// confirmed sort granularity (empirically verified — see spec/52).
//
// orderedHandles must be the COMPLETE list of items for the tab from GetInventoryOrder
// — no omissions, no duplicates. Partial lists are rejected.
//
// Only InventoryItem.Index values are changed. GaItems, handles, quantities,
// equipped slots, AoW handles, KeyItems, and storage are untouched.
func (a *App) ReorderInventory(charIdx int, tab string, orderedHandles []uint32) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return fmt.Errorf("invalid character index %d", charIdx)
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	return a.reorderInventoryLocked(charIdx, tab, orderedHandles)
}

// reorderInventoryLocked is the internal worker for ReorderInventory and
// ReorderWeaponInventory.
//
// Contract: caller MUST have validated `a.save != nil` and `charIdx` in
// range, and MUST hold exclusive access to slot[charIdx]. In the upcoming
// lock phase the caller will hold saveMu.RLock + slotMu[charIdx]. The helper
// takes exactly one pushUndoLocked snapshot per invocation.
func (a *App) reorderInventoryLocked(charIdx int, tab string, orderedHandles []uint32) error {
	categories, err := tabCategorySet(tab)
	if err != nil {
		return err
	}
	slot := &a.save.Slots[charIdx]
	if slot.Version == 0 {
		return fmt.Errorf("slot %d is empty", charIdx)
	}
	if len(orderedHandles) == 0 {
		return fmt.Errorf("orderedHandles must not be empty")
	}

	label := tabLabel[tab]

	// Guard: no duplicates in orderedHandles.
	seen := make(map[uint32]int, len(orderedHandles))
	for i, h := range orderedHandles {
		if prev, dup := seen[h]; dup {
			return fmt.Errorf("duplicate handle 0x%08X at positions %d and %d", h, prev, i)
		}
		seen[h] = i
	}

	startOff := slot.MagicOffset + core.InvStartFromMagic

	// Locate requested handles in CommonItems; validate category and technical.
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
			continue
		}
		itemID, ok := slot.GaMap[h]
		if !ok {
			continue
		}
		itemData, baseID := db.GetItemDataFuzzy(itemID)
		if !categories[itemData.Category] {
			return fmt.Errorf("handle 0x%08X (category %q) does not belong to sort order tab %q", h, itemData.Category, tab)
		}
		if tab == "weapons" && isWeaponOrderTechnical(itemData.Name, baseID) {
			return fmt.Errorf("handle 0x%08X is a technical placeholder (%s) and cannot be used in sort order", h, itemData.Name)
		}
		located[h] = invLoc{off: off}
	}

	// All requested handles must be in inventory.
	for _, h := range orderedHandles {
		if _, ok := located[h]; !ok {
			return fmt.Errorf("handle 0x%08X not found in %s inventory (may be in storage, or not a %s)", h, label, label)
		}
	}

	// Require complete list: count all eligible items for this tab.
	totalItems := 0
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
		if categories[itemData.Category] {
			if tab != "weapons" || !isWeaponOrderTechnical(itemData.Name, baseID) {
				totalItems++
			}
		}
	}
	if len(orderedHandles) != totalItems {
		return fmt.Errorf(
			"orderedHandles has %d %ss but inventory has %d; provide the full list from GetInventoryOrder",
			len(orderedHandles), label, totalItems,
		)
	}

	// Compute stride-2 base: NextAcquisitionSortId rounded up to nearest even number.
	// Must be strictly > InvEquipReservedMax (432). Even base ensures consecutive
	// positions produce distinct acqIdx>>1 keys: (base+2*i)>>1 = base/2+i.
	base := slot.Inventory.NextAcquisitionSortId
	if base <= uint32(core.InvEquipReservedMax) {
		base = uint32(core.InvEquipReservedMax) + 2 // 434 — minimum safe even value
	}
	if base%2 != 0 {
		base++
	}
	expectedMax := base + uint32(len(orderedHandles)-1)*2

	log.Printf("REORDER STRIDE2 range tab=%s count=%d base=%d expectedMax=%d",
		tab, len(orderedHandles), base, expectedMax)

	// Defensive: shiftDuplicates must be 0. With stride-2 + even base,
	// key = (base+2*i)>>1 = base/2+i is unique by construction. This guard
	// detects regressions if the base/stride logic is ever changed.
	shiftKeys := make(map[uint32]int, len(orderedHandles))
	for pos := range orderedHandles {
		key := (base + uint32(pos)*2) >> 1
		if prevPos, dup := shiftKeys[key]; dup {
			return fmt.Errorf("stride-2 reorder: bucket collision at key=%d positions %d and %d; refusing", key, prevPos, pos)
		}
		shiftKeys[key] = pos
	}

	// Push undo before any mutation.
	a.pushUndoLocked(charIdx)

	// Apply stride-2 indices to slot.Data and in-memory CommonItems.
	for pos, h := range orderedHandles {
		newIdx := base + uint32(pos)*2
		loc := located[h]
		binary.LittleEndian.PutUint32(slot.Data[loc.off+8:], newIdx)
		for j := range slot.Inventory.CommonItems {
			if slot.Inventory.CommonItems[j].GaItemHandle == h {
				slot.Inventory.CommonItems[j].Index = newIdx
				break
			}
		}
	}

	// Advance NextAcquisitionSortId and NextEquipIndex — never decrease.
	// Since base ≥ NextAcquisitionSortId, newNextAcq is always strictly larger.
	newNextAcq := expectedMax + 1
	if newNextAcq > slot.Inventory.NextAcquisitionSortId {
		slot.Inventory.NextAcquisitionSortId = newNextAcq
		if slot.Inventory.NextEquipIndex < newNextAcq {
			slot.Inventory.NextEquipIndex = newNextAcq
		}
		// Write to slot.Data immediately. Guard on equipIdxOff > 0: zero means the
		// offset was never set (e.g. test fixture); WriteSave will pick up the
		// in-memory value on next serialization.
		if equipIdxOff := slot.Inventory.NextEquipIndexOff(); equipIdxOff > 0 {
			binary.LittleEndian.PutUint32(slot.Data[equipIdxOff:], slot.Inventory.NextEquipIndex)
			binary.LittleEndian.PutUint32(slot.Data[equipIdxOff+4:], slot.Inventory.NextAcquisitionSortId)
		}
	}

	log.Printf("REORDER STRIDE2 SUMMARY tab=%s count=%d base=%d max=%d nextAcq=%d",
		tab, len(orderedHandles), base, expectedMax, newNextAcq)

	return nil
}

// ReorderStorage rewrites the acquisition indices of all items in slot charIdx's
// Storage CommonItems for the given tab so that orderedHandles[0] sorts first
// under "Acquisition Order ↑" when the storage box is browsed in-game.
//
// Mirrors ReorderInventory but operates on slot.Storage instead of slot.Inventory:
//   - reads/writes Storage.CommonItems and slot.Data[StorageBoxOffset + StorageHeaderSkip..]
//   - advances slot.Storage.NextEquipIndex / NextAcquisitionSortId monotonically
//   - calls ReconcileStorageHeader defensively at the end
//
// Uses the same stride-2 indexing (`base + pos*2`) as Inventory. The game sorts
// "Acquisition Order" by `acqIdx >> 1` for both Inventory and Storage browsers,
// so without stride-2 adjacent items can swap when the bucket collides
// (see spec/52 for the discovery).
//
// orderedHandles must be the COMPLETE list of storage items for the tab as
// returned by GetStorageOrder — no omissions, no duplicates. Partial lists,
// duplicates, or handles outside Storage are rejected with no mutation.
//
// Only InventoryItem.Index values are changed. GaItems, handles, quantities,
// equipped slots, AoW handles, KeyItems, and inventory items are untouched.
func (a *App) ReorderStorage(charIdx int, tab string, orderedHandles []uint32) error {
	categories, err := tabCategorySet(tab)
	if err != nil {
		return err
	}
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return fmt.Errorf("invalid character index %d", charIdx)
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	slot := &a.save.Slots[charIdx]
	if slot.Version == 0 {
		return fmt.Errorf("slot %d is empty", charIdx)
	}
	if slot.StorageBoxOffset <= 0 {
		return fmt.Errorf("slot %d has no storage box", charIdx)
	}
	if len(orderedHandles) == 0 {
		return fmt.Errorf("orderedHandles must not be empty")
	}

	label := tabLabel[tab]

	// Guard: no duplicates in orderedHandles.
	seen := make(map[uint32]int, len(orderedHandles))
	for i, h := range orderedHandles {
		if prev, dup := seen[h]; dup {
			return fmt.Errorf("duplicate handle 0x%08X at positions %d and %d", h, prev, i)
		}
		seen[h] = i
	}

	startOff := slot.StorageBoxOffset + core.StorageHeaderSkip

	// Locate requested handles in Storage.CommonItems; validate category & technical.
	type stoLoc struct{ off int }
	located := make(map[uint32]stoLoc, len(orderedHandles))

	for i := 0; i < core.StorageCommonCount; i++ {
		off := startOff + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if _, want := seen[h]; !want {
			continue
		}
		itemID, ok := slot.GaMap[h]
		if !ok {
			itemID = db.HandleToItemID(h)
		}
		itemData, baseID := db.GetItemDataFuzzy(itemID)
		if !categories[itemData.Category] {
			return fmt.Errorf("handle 0x%08X (category %q) does not belong to sort order tab %q", h, itemData.Category, tab)
		}
		if tab == "weapons" && isWeaponOrderTechnical(itemData.Name, baseID) {
			return fmt.Errorf("handle 0x%08X is a technical placeholder (%s) and cannot be used in sort order", h, itemData.Name)
		}
		located[h] = stoLoc{off: off}
	}

	for _, h := range orderedHandles {
		if _, ok := located[h]; !ok {
			return fmt.Errorf("handle 0x%08X not found in %s storage (may be in inventory, or not a %s)", h, label, label)
		}
	}

	// Require complete list: count all eligible storage items for this tab.
	totalItems := 0
	for i := 0; i < core.StorageCommonCount; i++ {
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
			itemID = db.HandleToItemID(h)
		}
		itemData, baseID := db.GetItemDataFuzzy(itemID)
		if itemData.Name == "" {
			continue
		}
		if categories[itemData.Category] {
			if tab != "weapons" || !isWeaponOrderTechnical(itemData.Name, baseID) {
				totalItems++
			}
		}
	}
	if len(orderedHandles) != totalItems {
		return fmt.Errorf(
			"orderedHandles has %d %ss but storage has %d; provide the full list from GetStorageOrder",
			len(orderedHandles), label, totalItems,
		)
	}

	// Stride-2 base: start at NextAcquisitionSortId rounded up to nearest even.
	// Strictly > InvEquipReservedMax to stay clear of reserved equipment slots.
	base := slot.Storage.NextAcquisitionSortId
	if base <= uint32(core.InvEquipReservedMax) {
		base = uint32(core.InvEquipReservedMax) + 2
	}
	if base%2 != 0 {
		base++
	}
	expectedMax := base + uint32(len(orderedHandles)-1)*2

	log.Printf("REORDER STORAGE STRIDE2 range tab=%s count=%d base=%d expectedMax=%d",
		tab, len(orderedHandles), base, expectedMax)

	// Defensive bucket-collision guard (unreachable with even base + stride-2,
	// but catches regressions if the base/stride logic changes).
	shiftKeys := make(map[uint32]int, len(orderedHandles))
	for pos := range orderedHandles {
		key := (base + uint32(pos)*2) >> 1
		if prevPos, dup := shiftKeys[key]; dup {
			return fmt.Errorf("stride-2 storage reorder: bucket collision at key=%d positions %d and %d; refusing", key, prevPos, pos)
		}
		shiftKeys[key] = pos
	}

	// Push undo before any mutation.
	a.pushUndoLocked(charIdx)

	// Apply stride-2 indices to slot.Data and in-memory Storage.CommonItems.
	for pos, h := range orderedHandles {
		newIdx := base + uint32(pos)*2
		loc := located[h]
		binary.LittleEndian.PutUint32(slot.Data[loc.off+8:], newIdx)
		for j := range slot.Storage.CommonItems {
			if slot.Storage.CommonItems[j].GaItemHandle == h {
				slot.Storage.CommonItems[j].Index = newIdx
				break
			}
		}
	}

	// Advance counters — monotonic, never decrease.
	newNextAcq := expectedMax + 1
	if newNextAcq > slot.Storage.NextAcquisitionSortId {
		slot.Storage.NextAcquisitionSortId = newNextAcq
		if slot.Storage.NextEquipIndex < newNextAcq {
			slot.Storage.NextEquipIndex = newNextAcq
		}
		if equipIdxOff := slot.Storage.NextEquipIndexOff(); equipIdxOff > 0 {
			binary.LittleEndian.PutUint32(slot.Data[equipIdxOff:], slot.Storage.NextEquipIndex)
			binary.LittleEndian.PutUint32(slot.Data[equipIdxOff+4:], slot.Storage.NextAcquisitionSortId)
		}
	}

	// Defensive header reconcile (counts unchanged, but mirrors transfer.go).
	core.ReconcileStorageHeader(slot)

	log.Printf("REORDER STORAGE STRIDE2 SUMMARY tab=%s count=%d base=%d max=%d nextAcq=%d",
		tab, len(orderedHandles), base, expectedMax, newNextAcq)

	return nil
}

// GetWeaponInventoryOrder returns all weapons in slot charIdx's CommonItems inventory,
// sorted by AcquisitionIndex ascending.
//
// Delegates to the internal locked worker (not the public GetInventoryOrder)
// to avoid re-entering the public Wails endpoint — which would double-acquire
// slotMu[charIdx] once the lock phase is introduced.
func (a *App) GetWeaponInventoryOrder(charIdx int) ([]InventoryOrderItem, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return nil, fmt.Errorf("invalid character index %d", charIdx)
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	return a.getInventoryOrderLocked(charIdx, "weapons")
}

// ReorderWeaponInventory rewrites the acquisition indices of all weapons in slot
// charIdx's CommonItems inventory.
//
// Delegates to the internal locked worker (not the public ReorderInventory)
// to avoid re-entering the public Wails endpoint — which would double-acquire
// slotMu[charIdx] once the lock phase is introduced.
func (a *App) ReorderWeaponInventory(charIdx int, orderedHandles []uint32) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return fmt.Errorf("invalid character index %d", charIdx)
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	return a.reorderInventoryLocked(charIdx, "weapons", orderedHandles)
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
