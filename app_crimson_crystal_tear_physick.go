package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// Confirmed by item-save-lab T090 (tmp/item-save-lab/WYNIKI_I_IMPLEMENTACJA.md,
// "Analiza T090 — pakiet Physick i Święta Łza"): a single native pickup at the
// Third Church basin writes an inseparable three-record bundle — it is never a
// standalone Crimson Crystal Tear pickup. The Database picker can only ever
// request the canonical ID 0x40002AFB (0x40002AFA is flagged "no_database" and
// is filtered out of db.GetItemsByCategory("key_items", ...) — see db.go's
// key_items case), yet T090 shows the native save uses the raw 0x40002AFA
// variant, never the canonical picker ID. Writing 0x40002AFB directly (the old
// behavior) is a contract the lab never confirms. This file converts a
// 0x40002AFB pick into the confirmed bundle instead.
const (
	// crimsonCrystalTearPickerID is the only Crimson Crystal Tear ID the
	// Database picker can ever send.
	crimsonCrystalTearPickerID = uint32(0x40002AFB)
	// crimsonCrystalTearVariantID is the raw save ID T090 shows the native
	// pickup actually writes to Inventory.KeyItems (handle 0xB0002AFA).
	// classifyItemAdd's nativeKeyItemFamily already routes this ID natively.
	crimsonCrystalTearVariantID = uint32(0x40002AFA)
	// aboutWondrousPhysickInfoItemID is the Info Item T090 shows accompanies
	// the bundle in CommonItems (handle 0xB000239B).
	aboutWondrousPhysickInfoItemID = uint32(0x4000239B)
	// crimsonCrystalTearBundleTutorialID is the TutorialData row T090 shows
	// the pickup appends.
	crimsonCrystalTearBundleTutorialID = uint32(1590)
)

// crimsonCrystalTearBundleAction is the result of checking the bundle's
// current physical state against the confirmed T090 contract.
type crimsonCrystalTearBundleAction int

const (
	// crimsonBundleCreate: none of the three confirmed records exist yet —
	// safe to create the full bundle from scratch.
	crimsonBundleCreate crimsonCrystalTearBundleAction = iota
	// crimsonBundleAlreadyComplete: all three records already exist exactly
	// as T090 specifies — re-requesting the pick is a safe no-op.
	crimsonBundleAlreadyComplete
)

// hasInventoryHandle reports whether any record in items carries handle.
func hasInventoryHandle(items []core.InventoryItem, handle uint32) bool {
	for _, it := range items {
		if it.GaItemHandle == handle {
			return true
		}
	}
	return false
}

// stackableHandle computes the id-derived handle-encoded goods/KeyItems
// handle (0xB0... for these item IDs), matching the formula
// core.AddItemsToSlotBatch's addKindStack/addKindKeyItemStack branches use.
func stackableHandle(id uint32) uint32 {
	return db.ItemIDToHandlePrefix(id) | (id & 0x0FFFFFFF)
}

// crimsonCrystalTearBundleState inspects the slot's CURRENT physical state
// against the confirmed T090 contract (KeyItems 0x40002AFA + CommonItems
// 0x400000FA + CommonItems 0x4000239B + TutorialData 1590) and reports
// whether it is safe to create the bundle or already fully present. Every
// partial or conflicting state — including any relevant record in Storage,
// an absent required tutorial, the empty Physick variant (0x400000FB), or the
// canonical picker handle (0xB0002AFB) — has no lab evidence for how the game
// would reconcile it, so it is reported as an error rather than silently
// merged, duplicated, or overwritten.
func crimsonCrystalTearBundleState(slot *core.SaveSlot) (crimsonCrystalTearBundleAction, error) {
	variantHandle := stackableHandle(crimsonCrystalTearVariantID)
	canonicalHandle := stackableHandle(crimsonCrystalTearPickerID)
	filledHandle := stackableHandle(db.ItemFlaskWondrousPhysickFilled)
	emptyHandle := stackableHandle(db.ItemFlaskWondrousPhysickEmpty)
	infoHandle := stackableHandle(aboutWondrousPhysickInfoItemID)

	hasVariantInKeyItems := hasInventoryHandle(slot.Inventory.KeyItems, variantHandle)
	hasVariantOutsideKeyItems := hasInventoryHandle(slot.Inventory.CommonItems, variantHandle) ||
		hasInventoryHandle(slot.Storage.CommonItems, variantHandle)
	hasCanonicalStray := hasInventoryHandle(slot.Inventory.KeyItems, canonicalHandle) ||
		hasInventoryHandle(slot.Inventory.CommonItems, canonicalHandle) ||
		hasInventoryHandle(slot.Storage.CommonItems, canonicalHandle)
	hasFilledInInventory := hasInventoryHandle(slot.Inventory.CommonItems, filledHandle)
	hasFilledOutsideInventory := hasInventoryHandle(slot.Inventory.KeyItems, filledHandle) ||
		hasInventoryHandle(slot.Storage.CommonItems, filledHandle)
	hasEmpty := hasInventoryHandle(slot.Inventory.KeyItems, emptyHandle) ||
		hasInventoryHandle(slot.Inventory.CommonItems, emptyHandle) ||
		hasInventoryHandle(slot.Storage.CommonItems, emptyHandle)
	hasInfoInInventory := hasInventoryHandle(slot.Inventory.CommonItems, infoHandle)
	hasInfoOutsideInventory := hasInventoryHandle(slot.Inventory.KeyItems, infoHandle) ||
		hasInventoryHandle(slot.Storage.CommonItems, infoHandle)
	hasTutorial, err := core.HasTutorialID(slot, crimsonCrystalTearBundleTutorialID)
	if err != nil {
		return crimsonBundleCreate, fmt.Errorf("cannot inspect Crimson Crystal Tear bundle tutorial state: %w", err)
	}

	noConflicts := !hasVariantOutsideKeyItems && !hasCanonicalStray &&
		!hasFilledOutsideInventory && !hasEmpty && !hasInfoOutsideInventory
	emptyBundle := !hasVariantInKeyItems && !hasVariantOutsideKeyItems && !hasCanonicalStray &&
		!hasFilledInInventory && !hasFilledOutsideInventory && !hasEmpty &&
		!hasInfoInInventory && !hasInfoOutsideInventory && !hasTutorial

	switch {
	case hasVariantInKeyItems && hasFilledInInventory && hasInfoInInventory && hasTutorial && noConflicts:
		return crimsonBundleAlreadyComplete, nil
	case emptyBundle:
		return crimsonBundleCreate, nil
	default:
		return crimsonBundleCreate, fmt.Errorf(
			"Crimson Crystal Tear / Flask of Wondrous Physick bundle is in a partial or conflicting state "+
				"(variant in KeyItems=%v, variant outside KeyItems=%v, filled Physick in Inventory=%v, "+
				"filled Physick outside Inventory=%v, empty Physick present=%v, Info Item in Inventory=%v, "+
				"Info Item outside Inventory=%v, TutorialData 1590=%v, stray canonical Crystal Tear=%v) — "+
				"refusing to guess a merge; the confirmed T090 bundle requires all records and tutorial together "+
				"in their native Inventory locations or none of them",
			hasVariantInKeyItems, hasVariantOutsideKeyItems, hasFilledInInventory, hasFilledOutsideInventory,
			hasEmpty, hasInfoInInventory, hasInfoOutsideInventory, hasTutorial, hasCanonicalStray)
	}
}

// validateCrimsonCrystalTearBundleQuantity checks the whole-request inv/
// storage quantities against the confirmed T090 contract before any mutation
// runs: the bundle is a single native Inventory pickup at the Third Church
// basin — it has no Storage variant and no lab evidence for any quantity
// other than exactly one. addItemsToCharacter has no per-item quantity (inv/
// storageQty apply to the whole request), so a Storage-only request, a
// combined Inventory+Storage request, or an unsupported inventory count must
// fail closed here rather than have the caller silently redirect the bundle's
// container or clamp its quantity.
func validateCrimsonCrystalTearBundleQuantity(invQty, storageQty int) error {
	if storageQty != 0 {
		return fmt.Errorf(
			"Crimson Crystal Tear / Flask of Wondrous Physick bundle is Inventory-only (T090 confirmed native pickup); "+
				"requested storage quantity %d is not supported", storageQty)
	}
	if invQty != 1 && invQty != -1 {
		return fmt.Errorf(
			"Crimson Crystal Tear / Flask of Wondrous Physick bundle only supports a single native pickup "+
				"(inventory quantity 1, or -1 for game max); requested inventory quantity %d is not supported", invQty)
	}
	return nil
}

// appendCrimsonCrystalTearBundleItems appends the three confirmed T090
// records to items. It deliberately bypasses addItemsToCharacter's generic
// per-item Physick handling (the isPhysick id-rewrite, which would otherwise
// force any Wondrous Physick pick — including this bundle's raw filled ID —
// down to the empty variant 0x400000FB): these three ItemToAdd entries are
// added directly to the capacity/batch list, never to the sortedIDs/prepared
// per-item pipeline. Capacity for the three additions is validated by the
// caller's normal CheckAddCapacity pass over the returned slice, and their
// GaItemData/routing (KeyItems for the variant, CommonItems for the other
// two) comes from the existing classifyItemAdd contract — no bundle-specific
// writer logic is needed for any of that.
func appendCrimsonCrystalTearBundleItems(items []core.ItemToAdd) []core.ItemToAdd {
	return append(items,
		core.ItemToAdd{ItemID: crimsonCrystalTearVariantID, InvQty: 1},
		core.ItemToAdd{ItemID: db.ItemFlaskWondrousPhysickFilled, InvQty: 1},
		core.ItemToAdd{ItemID: aboutWondrousPhysickInfoItemID, InvQty: 1},
	)
}
