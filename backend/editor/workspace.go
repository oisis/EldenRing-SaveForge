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
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
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

// CurrentAoWStatus values surface read-side AoW state on editable
// weapons. Empty means the field is not relevant (non-weapon or never
// populated). All others apply only to weapon-editable items.
const (
	AoWStatusNone    = "none"    // weapon has no custom AoW (sentinel handle)
	AoWStatusCustom  = "custom"  // weapon has a custom AoW that resolves to a known ashes_of_war DB entry
	AoWStatusMissing = "missing" // weapon's AoW handle is non-sentinel but cannot be resolved (orphan / dangling)
	AoWStatusShared  = "shared"  // weapon's AoW handle is referenced by another weapon (save corruption)
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
//
// CurrentAoW* fields (Phase 4A) are READ-ONLY mirrors of the AoW that is
// currently encoded into the weapon's GaItem.AoWGaItemHandle. They are
// populated by BuildSnapshot for editable weapons and never mutated by
// editor mutations — they describe slot state at snapshot time, not the
// user's pending edit. Use Pending* fields to express an unsaved AoW
// swap; the two coexist until Save resolves the pending request.
type EditableItem struct {
	UID              string        `json:"uid"`
	Source           ItemSource    `json:"source"`
	Container        ContainerKind `json:"container"`
	Position         int           `json:"position"`
	OriginalHandle   uint32        `json:"originalHandle"`
	ItemID           uint32        `json:"itemID"`
	BaseItemID       uint32        `json:"baseItemID"`
	Name             string        `json:"name"`
	Category         string        `json:"category"`
	Quantity         uint32        `json:"quantity"`
	AcquisitionIndex uint32        `json:"acquisitionIndex"`
	Weight           float64       `json:"weight,omitempty"`
	SortID           uint32        `json:"sortId,omitempty"`
	SortGroupID      uint8         `json:"sortGroupId,omitempty"`
	CurrentUpgrade   int           `json:"currentUpgrade"`
	MaxUpgrade       int           `json:"maxUpgrade"`
	InfusionName     string        `json:"infusionName,omitempty"`
	IconPath         string        `json:"iconPath,omitempty"`
	HasGaItem        bool          `json:"hasGaItem"`
	IsWeapon         bool          `json:"isWeapon"`
	IsArmor          bool          `json:"isArmor"`
	IsTalisman       bool          `json:"isTalisman"`
	// AoW mounting compatibility metadata, populated for weapon-editable
	// items. WepType / CanMountAoW mirror the DB lookups done by
	// vm.MapParsedSlotToVM so the WeaponEditModal can resolve AoW
	// compatibility directly from the workspace item without falling
	// back to GetCharacter (which can desync for newly-added items, or
	// when the handle has been re-allocated by a prior Save).
	WepType               uint16 `json:"wepType,omitempty"`
	CanMountAoW           bool   `json:"canMountAoW,omitempty"`
	DefaultAoWID          int32  `json:"defaultAoWID,omitempty"`
	DefaultAoWName        string `json:"defaultAoWName,omitempty"`
	CurrentAoWHandle      uint32 `json:"currentAoWHandle,omitempty"`
	CurrentAoWItemID      uint32 `json:"currentAoWItemID,omitempty"`
	CurrentAoWName        string `json:"currentAoWName,omitempty"`
	HasCurrentAoW         bool   `json:"hasCurrentAoW,omitempty"`
	CurrentAoWShared      bool   `json:"currentAoWShared,omitempty"`
	CurrentAoWStatus      string `json:"currentAoWStatus,omitempty"`
	PendingAoWItemID      uint32 `json:"pendingAoWItemID,omitempty"`
	PendingAoWName        string `json:"pendingAoWName,omitempty"`
	PendingAoWClear       bool   `json:"pendingAoWClear,omitempty"`
	HasPendingWeaponPatch bool   `json:"hasPendingWeaponPatch,omitempty"`

	// OriginalSlotIndex is the physical CommonItems slot this item was parsed
	// from (-1 for items added in the workspace). The save path replays an
	// unchanged container to these exact slots so a no-op save stays
	// byte-identical to the loaded file.
	OriginalSlotIndex int `json:"originalSlotIndex"`
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

	// Precompute weapon → AoW handle references and shared-handle
	// counts in one pass over slot.GaItems. classifyRecord uses these
	// to populate CurrentAoW* fields on weapon-editable items without
	// rescanning GaItems per record.
	weaponAoWRefs, aowSharedCount := buildWeaponAoWMaps(slot)

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
		c := classifyRecord(slot, ContainerInventory, i, h, qty, acq, invSeen, weaponAoWRefs, aowSharedCount)
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
			c := classifyRecord(slot, ContainerStorage, i, h, qty, acq, stoSeen, weaponAoWRefs, aowSharedCount)
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
//
// weaponAoWRefs / aowSharedCount carry pre-computed AoW state for
// editable weapons. They are produced once per BuildSnapshot by
// buildWeaponAoWMaps.
func classifyRecord(slot *core.SaveSlot, container ContainerKind, slotIdx int, handle, qty, acq uint32, seen map[uint32]int, weaponAoWRefs map[uint32]uint32, aowSharedCount map[uint32]int) classified {
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
	sortKey := data.ItemSortKeys[baseID]
	defaultAoWID, defaultAoWName := defaultAoWForBaseID(baseID)

	uid := fmt.Sprintf("hnd:0x%08X", handle)

	editable := &EditableItem{
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
		Weight:           data.ItemWeights[baseID],
		SortID:           sortKey.SortId,
		SortGroupID:      sortKey.SortGroupId,
		CurrentUpgrade:   level,
		MaxUpgrade:       int(itemData.MaxUpgrade),
		InfusionName:     infusion,
		IconPath:         itemData.IconPath,
		HasGaItem:        hasGa,
		IsWeapon:         isWeapon,
		IsArmor:          isArmor,
		IsTalisman:       isTalisman,
		DefaultAoWID:     defaultAoWID,
		DefaultAoWName:   defaultAoWName,
	}
	editable.OriginalSlotIndex = slotIdx
	if isWeapon {
		// Weapon AoW-mounting metadata: WepType is the EquipParamWeapon
		// category integer (0 = unknown), CanMountAoW reflects the
		// gemMountType==2 gate. Both come from the DB entry resolved by
		// GetItemDataFuzzy above and are exposed to the workspace so the
		// edit modal does not have to round-trip through GetCharacter
		// (which can desync for handles re-allocated by Save).
		editable.WepType = itemData.WepType
		editable.CanMountAoW = itemData.GemMountType == 2
		populateCurrentAoW(slot, editable, weaponAoWRefs, aowSharedCount)
	}
	return classified{editable: editable}
}

// buildWeaponAoWMaps walks slot.GaItems once and produces:
//   - weaponAoWRefs: weaponHandle → AoWGaItemHandle (only for weapons
//     whose AoWGaItemHandle is NOT the no-custom sentinel)
//   - aowSharedCount: aowHandle → number of weapons referencing it.
//     A count > 1 is a save-corruption indicator (game expects each AoW
//     copy to attach to at most one weapon).
//
// Empty GaItems entries are skipped. Returns nil maps if slot.GaItems
// is empty so callers can safely range over the results.
func buildWeaponAoWMaps(slot *core.SaveSlot) (map[uint32]uint32, map[uint32]int) {
	weaponAoWRefs := map[uint32]uint32{}
	aowSharedCount := map[uint32]int{}
	if slot == nil {
		return weaponAoWRefs, aowSharedCount
	}
	for i := range slot.GaItems {
		g := &slot.GaItems[i]
		if g.IsEmpty() {
			continue
		}
		if (g.Handle & core.GaHandleTypeMask) != core.ItemTypeWeapon {
			continue
		}
		if core.IsNoCustomAoWHandle(g.AoWGaItemHandle) {
			continue
		}
		weaponAoWRefs[g.Handle] = g.AoWGaItemHandle
		aowSharedCount[g.AoWGaItemHandle]++
	}
	return weaponAoWRefs, aowSharedCount
}

// populateCurrentAoW fills the CurrentAoW* fields on an editable weapon
// from precomputed slot scans. No-op for non-weapons.
//
// Resolution:
//   - No entry in weaponAoWRefs → the GaItem either lacks an
//     AoWGaItemHandle (impossible for a weapon — every weapon GaItem has
//     one) or that handle is the no-custom sentinel. Status = "none".
//   - Entry present → look up the AoW itemID via slot.GaMap[aowHandle].
//     If the lookup fails the handle is dangling → status = "missing".
//     Otherwise the DB is queried for the AoW's name/category and:
//   - count > 1 in aowSharedCount → status = "shared" (corruption)
//   - otherwise status = "custom"
//
// The function never errors; it sets enough fields for Validate to emit
// a warning if state is anomalous (missing / shared / non-AoW category).
func populateCurrentAoW(slot *core.SaveSlot, item *EditableItem, weaponAoWRefs map[uint32]uint32, aowSharedCount map[uint32]int) {
	if item == nil || !item.IsWeapon {
		return
	}
	aowHandle, hasRef := weaponAoWRefs[item.OriginalHandle]
	if !hasRef {
		item.CurrentAoWStatus = AoWStatusNone
		return
	}
	item.CurrentAoWHandle = aowHandle
	item.HasCurrentAoW = true
	aowItemID, mapped := uint32(0), false
	if slot != nil {
		aowItemID, mapped = slot.GaMap[aowHandle]
	}
	if !mapped || aowItemID == 0 {
		item.CurrentAoWStatus = AoWStatusMissing
		return
	}
	item.CurrentAoWItemID = aowItemID
	aowData, _ := db.GetItemDataFuzzy(aowItemID)
	item.CurrentAoWName = aowData.Name
	if aowSharedCount[aowHandle] > 1 {
		item.CurrentAoWShared = true
		item.CurrentAoWStatus = AoWStatusShared
		return
	}
	item.CurrentAoWStatus = AoWStatusCustom
}

func defaultAoWForBaseID(baseID uint32) (int32, string) {
	v, ok := data.WeaponStatsV1ByID[baseID]
	if !ok || v.DefaultAoWID == 0 {
		return 0, ""
	}
	if name, ok := data.SwordArtsNames[v.DefaultAoWID]; ok {
		return v.DefaultAoWID, name
	}
	return v.DefaultAoWID, fmt.Sprintf("Skill #%d", v.DefaultAoWID)
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
