package core

import (
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// itemAddKind classifies how adding one item consumes save-slot resources. It is
// the single source of truth shared by CheckAddCapacity (preflight counting) and
// AddItemsToSlotBatch (write branch selection), so the two can never disagree —
// notably about talismans, which occupy N physical inventory/storage records yet
// allocate zero serialized GaItems.
//
// ItemToAdd deliberately carries no caller-provided "is stackable" flag: the kind
// is a pure function of the item ID (plus the explicit arrow override), so every
// caller (app, presets, editor, tests) gets identical semantics for free.
type itemAddKind int

const (
	// addKindStack: fungible goods (0xB0). Quantity folds into at most one
	// physical record per destination; handle-encoded, no serialized GaItem.
	addKindStack itemAddKind = iota
	// addKindTalisman: accessories (0xA0). Each requested copy is a distinct
	// physical record of quantity 1 (copies never merge); handle-encoded, no
	// serialized GaItem.
	addKindTalisman
	// addKindArrow: forced-stackable ammo. A weapon-prefixed item that behaves as
	// a merged stack in-game but is backed by a lazily-allocated GaItem (see the
	// writer stackable path). Counted like a stack for capacity — existing
	// behavior preserved exactly.
	addKindArrow
	// addKindKeyItemStack: the confirmed-native-KeyItems family (see
	// nativeKeyItemFamily). Fungible handle-encoded stack exactly like
	// addKindStack (no serialized GaItem, one active GaItemData entry for a
	// genuinely new ID) — the only difference is the physical destination:
	// Inventory.KeyItems instead of Inventory.CommonItems.
	addKindKeyItemStack
	// addKindGaItem: weapons/armor/AoW. One physical record plus one serialized
	// GaItem per destination; weapons/armor/AoW also consume a GaItemData entry.
	addKindGaItem
)

// craftingKitItemID, crackedPotItemID and crimsonCrystalTearVariantID are the
// three single-ID confirmed-native-KeyItems families that aren't already
// covered by a whole-family DB helper (data.IsCookbookItemID covers
// cookbooks). crimsonCrystalTearVariantID is deliberately the raw save
// variant 0x40002AFA T090 observed on disk, NOT the canonical picker ID
// 0x40002AFB.
const (
	craftingKitItemID           = uint32(0x40002134) // T070 — Crafting Kit
	crackedPotItemID            = uint32(0x4000251C) // T074 — Cracked Pot only (not Ritual/Hefty Pot, Perfume Bottle)
	crimsonCrystalTearVariantID = uint32(0x40002AFA) // T090 — Physick package's raw save variant
)

// nativeKeyItemFamily reports whether id is one of the confirmed families
// (item-save-lab T070/T071/T074/T090) that the native game writes into
// Inventory.KeyItems instead of Inventory.CommonItems: Crafting Kit, the
// cookbook family (data.IsCookbookItemID — the same classifier already used
// to mark cookbook inventory rows read-only, see backend/vm/character_vm.go),
// Cracked Pot, and the Physick package's Crimson Crystal Tear variant.
// Deliberately narrow: no other db Key Items category (Ritual Pot, Hefty
// Cracked Pot, Perfume Bottle, Memory Stone, Bell Bearings, map fragments,
// ...) has this level of direct native save evidence, so none of them route
// here — a batch/preflight item outside this set keeps falling through to
// addKindStack exactly as before.
func nativeKeyItemFamily(id uint32) bool {
	switch id {
	case craftingKitItemID, crackedPotItemID, crimsonCrystalTearVariantID:
		return true
	}
	return data.IsCookbookItemID(id)
}

// classifyItemAdd derives the resource semantics for adding item id.
// forceStackable is the explicit arrow override (arrows are weapon-prefixed
// but stack in-game). forceCommonItems overrides nativeKeyItemFamily back to
// the classic CommonItems goods contract (addKindStack) — used by
// AddItemsToSlot's legacy single-item callers (container auto-sync, cookbook/
// whetblade/map-fragment flag unlocks, DLC map reveal), whose established,
// independently tested "canonical CommonItems, legacy KeyItems compat"
// behavior must not shift just because one of their IDs now also has direct
// native pickup evidence. It does not affect the GaItemData contract, which
// addKindStack and addKindKeyItemStack create identically.
func classifyItemAdd(id uint32, forceStackable, forceCommonItems bool) itemAddKind {
	switch {
	case forceStackable:
		return addKindArrow
	case db.ItemIDToHandlePrefix(id) == ItemTypeAccessory:
		return addKindTalisman
	case !forceCommonItems && nativeKeyItemFamily(id):
		return addKindKeyItemStack
	case db.ItemIDToHandlePrefix(id) == ItemTypeItem:
		return addKindStack
	default:
		return addKindGaItem
	}
}
