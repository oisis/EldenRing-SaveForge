package core

import "github.com/oisis/EldenRing-SaveForge/backend/db"

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
	// addKindGaItem: weapons/armor/AoW. One physical record plus one serialized
	// GaItem per destination; weapons/AoW also consume a GaItemData entry.
	addKindGaItem
)

// classifyItemAdd derives the resource semantics for adding item id. forceStackable
// is the explicit arrow override (arrows are weapon-prefixed but stack in-game).
func classifyItemAdd(id uint32, forceStackable bool) itemAddKind {
	switch {
	case forceStackable:
		return addKindArrow
	case db.ItemIDToHandlePrefix(id) == ItemTypeAccessory:
		return addKindTalisman
	case db.ItemIDToHandlePrefix(id) == ItemTypeItem:
		return addKindStack
	default:
		return addKindGaItem
	}
}
