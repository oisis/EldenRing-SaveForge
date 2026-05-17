// Package editor implements an in-memory inventory edit session — a
// read-only workspace built from a parsed SaveSlot that mirrors the
// inventory and storage views.
//
// Phase 1 is intentionally non-mutating: the workspace exists only to
// support future reorder/add/transfer/weapon-edit flows which will rebuild
// the slot in a single Save step. Nothing in this package writes to
// slot.Data or to the in-memory slot structs.
package editor

import (
	"encoding/binary"
	"fmt"
	"sort"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// ContainerKind identifies which container an item lives in.
type ContainerKind string

const (
	ContainerInventory ContainerKind = "inventory"
	ContainerStorage   ContainerKind = "storage"
)

// ItemSource records how an item entered the workspace. Phase 1 only sees
// ItemSourceOriginal — ItemSourceAdded is reserved for future
// AddInventoryWorkspaceItem mutations.
type ItemSource string

const (
	ItemSourceOriginal ItemSource = "original"
	ItemSourceAdded    ItemSource = "added"
)

// SupportedCategories is the conservative Phase 1 allow-list. Everything
// outside this set is preserved as a RawInventoryRecord (pass-through) and
// cannot be edited.
var SupportedCategories = map[string]bool{
	"melee_armaments":      true,
	"ranged_and_catalysts": true,
	"shields":              true,
	"talismans":            true,
	"head":                 true,
	"chest":                true,
	"arms":                 true,
	"legs":                 true,
}

// Stable Reason codes attached to RawInventoryRecord. Tests assert against
// these so they must be kept stable.
const (
	ReasonUnsupportedCategory  = "unsupported_category"
	ReasonUnknownItem          = "unknown_item"
	ReasonTechnicalPlaceholder = "technical_placeholder"
	ReasonDuplicateHandle      = "duplicate_handle"
)

// invUnarmedBaseID mirrors the legacy ReorderInventory constant — the
// "Unarmed" placeholder weapon kept by the game as a technical slot.
const invUnarmedBaseID = uint32(0x0001ADB0)

// InventoryWorkspaceSnapshot is the read-only DTO returned across the
// Wails boundary. It is a single point-in-time view of an
// InventoryEditSession's Workspace field.
type InventoryWorkspaceSnapshot struct {
	SessionID                   string                    `json:"sessionID"`
	CharacterIndex              int                       `json:"characterIndex"`
	InventoryItems              []EditableItem            `json:"inventoryItems"`
	StorageItems                []EditableItem            `json:"storageItems"`
	UnsupportedInventoryRecords []RawInventoryRecord      `json:"unsupportedInventoryRecords"`
	UnsupportedStorageRecords   []RawInventoryRecord      `json:"unsupportedStorageRecords"`
	Dirty                       bool                      `json:"dirty"`
	Validation                  WorkspaceValidationReport `json:"validation"`
}

// EditableItem is the user-editable representation of an inventory record.
// All numeric IDs match the DB / game representation.
//
// Pending* fields (Phase 1.7) carry RAM-only requests for weapon edits
// (Ash of War swap) that have NOT yet been encoded into the binary save.
// The eventual Save step (Phase 3+) reads these fields to drive the real
// GaItem patch / handle allocation. Until then they coexist with the
// existing ItemID/CurrentUpgrade/InfusionName fields, which DO get
// updated in place by UpdateWeapon when upgrade/infusion change (because
// those are pure ItemID re-encodings and don't need a handle).
type EditableItem struct {
	UID                   string        `json:"uid"`
	Source                ItemSource    `json:"source"`
	Container             ContainerKind `json:"container"`
	Position              int           `json:"position"`
	OriginalHandle        uint32        `json:"originalHandle"`
	ItemID                uint32        `json:"itemID"`
	BaseItemID            uint32        `json:"baseItemID"`
	Name                  string        `json:"name"`
	Category              string        `json:"category"`
	Quantity              uint32        `json:"quantity"`
	AcquisitionIndex      uint32        `json:"acquisitionIndex"`
	CurrentUpgrade        int           `json:"currentUpgrade"`
	MaxUpgrade            int           `json:"maxUpgrade"`
	InfusionName          string        `json:"infusionName,omitempty"`
	IconPath              string        `json:"iconPath,omitempty"`
	HasGaItem             bool          `json:"hasGaItem"`
	IsWeapon              bool          `json:"isWeapon"`
	IsArmor               bool          `json:"isArmor"`
	IsTalisman            bool          `json:"isTalisman"`
	PendingAoWItemID      uint32        `json:"pendingAoWItemID,omitempty"`
	PendingAoWName        string        `json:"pendingAoWName,omitempty"`
	HasPendingWeaponPatch bool          `json:"hasPendingWeaponPatch,omitempty"`
}

// RawInventoryRecord captures a record the workspace cannot or should not
// edit. It carries enough metadata for a future rebuild step to write the
// bytes back into the same slot index without loss.
type RawInventoryRecord struct {
	Container        ContainerKind `json:"container"`
	SlotIndex        int           `json:"slotIndex"`
	Handle           uint32        `json:"handle"`
	Quantity         uint32        `json:"quantity"`
	AcquisitionIndex uint32        `json:"acquisitionIndex"`
	ItemID           uint32        `json:"itemID"`
	Name             string        `json:"name,omitempty"`
	Category         string        `json:"category,omitempty"`
	Reason           string        `json:"reason"`
	HasGaItem        bool          `json:"hasGaItem"`
}

// rawRecord is the internal carrier used during scanning to express
// "either editable or pass-through" without losing slot context.
type classified struct {
	editable *EditableItem
	raw      *RawInventoryRecord
}

// BuildSnapshot scans the slot's CommonItems for both containers and
// produces a sorted, classified InventoryWorkspaceSnapshot. The slot is
// not mutated.
//
// Ordering matches the legacy GetInventoryOrder/GetStorageOrder behavior:
// editable items are sorted by AcquisitionIndex ascending and assigned
// sequential Position values starting at 0. Pass-through records keep
// their physical SlotIndex.
func BuildSnapshot(slot *core.SaveSlot, sessionID string, charIdx int) (InventoryWorkspaceSnapshot, error) {
	snap := InventoryWorkspaceSnapshot{
		SessionID:                   sessionID,
		CharacterIndex:              charIdx,
		InventoryItems:              []EditableItem{},
		StorageItems:                []EditableItem{},
		UnsupportedInventoryRecords: []RawInventoryRecord{},
		UnsupportedStorageRecords:   []RawInventoryRecord{},
	}
	if slot == nil {
		return snap, fmt.Errorf("BuildSnapshot: nil slot")
	}
	if slot.Version == 0 {
		// Empty slot — nothing to scan, return empty snapshot.
		return snap, nil
	}

	invSeen := make(map[uint32]int)
	stoSeen := make(map[uint32]int)

	// Inventory scan.
	invStart := slot.MagicOffset + core.InvStartFromMagic
	for i := 0; i < core.CommonItemCount; i++ {
		off := invStart + i*core.InvRecordLen
		if off+core.InvRecordLen > len(slot.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slot.Data[off:])
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		qty := binary.LittleEndian.Uint32(slot.Data[off+4:])
		acq := binary.LittleEndian.Uint32(slot.Data[off+8:])
		c := classifyRecord(slot, ContainerInventory, i, h, qty, acq, invSeen)
		if c.editable != nil {
			snap.InventoryItems = append(snap.InventoryItems, *c.editable)
		}
		if c.raw != nil {
			snap.UnsupportedInventoryRecords = append(snap.UnsupportedInventoryRecords, *c.raw)
		}
	}

	// Storage scan.
	if slot.StorageBoxOffset > 0 {
		stoStart := slot.StorageBoxOffset + core.StorageHeaderSkip
		for i := 0; i < core.StorageCommonCount; i++ {
			off := stoStart + i*core.InvRecordLen
			if off+core.InvRecordLen > len(slot.Data) {
				break
			}
			h := binary.LittleEndian.Uint32(slot.Data[off:])
			if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
				continue
			}
			qty := binary.LittleEndian.Uint32(slot.Data[off+4:])
			acq := binary.LittleEndian.Uint32(slot.Data[off+8:])
			c := classifyRecord(slot, ContainerStorage, i, h, qty, acq, stoSeen)
			if c.editable != nil {
				snap.StorageItems = append(snap.StorageItems, *c.editable)
			}
			if c.raw != nil {
				snap.UnsupportedStorageRecords = append(snap.UnsupportedStorageRecords, *c.raw)
			}
		}
	}

	sort.SliceStable(snap.InventoryItems, func(i, j int) bool {
		return snap.InventoryItems[i].AcquisitionIndex < snap.InventoryItems[j].AcquisitionIndex
	})
	for i := range snap.InventoryItems {
		snap.InventoryItems[i].Position = i
	}
	sort.SliceStable(snap.StorageItems, func(i, j int) bool {
		return snap.StorageItems[i].AcquisitionIndex < snap.StorageItems[j].AcquisitionIndex
	})
	for i := range snap.StorageItems {
		snap.StorageItems[i].Position = i
	}

	return snap, nil
}

// classifyRecord inspects a single 12-byte record and decides whether it
// becomes an EditableItem or a pass-through RawInventoryRecord.
//
// Resolution order matches the legacy GetStorageOrder fallback:
//  1. GaMap[handle] — set by allocateGaItem for weapons/armor/AoW and
//     rehandled instances.
//  2. db.HandleToItemID(handle) — vanilla handle-encoded goods/talisman.
//
// The `seen` map tracks the first slot index a handle appeared at within
// a container; a second appearance is forced to pass-through with
// ReasonDuplicateHandle so the editable list never carries duplicate
// handles (validator catches it separately).
func classifyRecord(slot *core.SaveSlot, container ContainerKind, slotIdx int, handle, qty, acq uint32, seen map[uint32]int) classified {
	hasGa := false
	itemID, ok := slot.GaMap[handle]
	if ok {
		hasGa = true
	} else {
		itemID = db.HandleToItemID(handle)
	}

	itemData, baseID := db.GetItemDataFuzzy(itemID)

	raw := func(reason, name, cat string) classified {
		return classified{raw: &RawInventoryRecord{
			Container:        container,
			SlotIndex:        slotIdx,
			Handle:           handle,
			Quantity:         qty,
			AcquisitionIndex: acq,
			ItemID:           itemID,
			Name:             name,
			Category:         cat,
			Reason:           reason,
			HasGaItem:        hasGa,
		}}
	}

	// Duplicate handle in the same container.
	if _, dup := seen[handle]; dup {
		return raw(ReasonDuplicateHandle, itemData.Name, itemData.Category)
	}
	seen[handle] = slotIdx

	if itemData.Name == "" {
		return raw(ReasonUnknownItem, "", "")
	}

	// Unarmed placeholder — physical slot kept, but not editable.
	if baseID == invUnarmedBaseID || itemData.Name == "Unarmed" {
		return raw(ReasonTechnicalPlaceholder, itemData.Name, itemData.Category)
	}

	if !SupportedCategories[itemData.Category] {
		return raw(ReasonUnsupportedCategory, itemData.Name, itemData.Category)
	}

	level, infusion := decodeWeaponUpgradeInfusion(itemID, baseID)
	isWeapon := isWeaponCategory(itemData.Category)
	isArmor := isArmorCategory(itemData.Category)
	isTalisman := itemData.Category == "talismans"

	uid := fmt.Sprintf("hnd:0x%08X", handle)

	return classified{editable: &EditableItem{
		UID:              uid,
		Source:           ItemSourceOriginal,
		Container:        container,
		OriginalHandle:   handle,
		ItemID:           itemID,
		BaseItemID:       baseID,
		Name:             itemData.Name,
		Category:         itemData.Category,
		Quantity:         qty,
		AcquisitionIndex: acq,
		CurrentUpgrade:   level,
		MaxUpgrade:       int(itemData.MaxUpgrade),
		InfusionName:     infusion,
		IconPath:         itemData.IconPath,
		HasGaItem:        hasGa,
		IsWeapon:         isWeapon,
		IsArmor:          isArmor,
		IsTalisman:       isTalisman,
	}}
}

// decodeWeaponUpgradeInfusion mirrors the legacy helper in
// app_inventory_order.go — kept duplicated here to avoid an import cycle
// (main → backend/editor would forbid backend/editor → main).
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

func isWeaponCategory(cat string) bool {
	switch cat {
	case "melee_armaments", "ranged_and_catalysts", "shields":
		return true
	}
	return false
}

func isArmorCategory(cat string) bool {
	switch cat {
	case "head", "chest", "arms", "legs":
		return true
	}
	return false
}

