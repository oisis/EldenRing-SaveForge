package main

import (
	"fmt"
	"sort"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// itemAddPlanItem is one prepared add, holding exactly the fields the mutation
// tail consumes: the base ID (key into the event-flag / tutorial / companion
// maps), the resolved inventory/storage quantities (bolstering pickup count),
// and the in-place KeyItems stack bump (row + target qty, issue 7).
type itemAddPlanItem struct {
	baseID        uint32
	actualInv     int
	actualStorage int
	keyRow        int // KeyItems row to bump in place, -1 if none
	keyTargetQty  int // target stack qty for that row, 0 if no update
}

// itemAddMutationPlan is the immutable, already-computed input to the mutation
// tail of addItemsToCharacter. Building the plan once lets the real Database Add
// path apply it to the real slot while a later diagnostic planner applies the
// identical plan to core.CloneSlot(slot). The plan carries no App state: no
// locking, undo, journal, snapshot or rollback responsibility lives here.
type itemAddMutationPlan struct {
	batch               []core.ItemToAdd
	items               []itemAddPlanItem
	usedContainers      map[uint32]bool
	storageContainers   map[uint32]bool
	existingByContainer map[uint32]int
	// requiredTutorialIDs are TutorialData IDs that must be appended as part
	// of this same mutation, unlike the best-effort data.AboutTutorialID
	// lookup below: a failure here (e.g. a full TutorialData list) rolls
	// back the whole plan instead of only warning. Used by bundles (T090
	// Crimson Crystal Tear) whose confirmed native contract has no partial
	// state — the tutorial entry is not optional cosmetic polish for them.
	requiredTutorialIDs []uint32
}

// applyItemAddMutationPlan performs the batch add, in-place key-item stack
// bumps, post-add event-flag/tutorial side effects, container upserts with
// pickup/vendor flags, storage-header reconciliation and post-mutation
// validation — in the same semantic order as before extraction. It mutates only
// the passed slot, does no locking/undo/journal/rollback, and is safe to call
// against a cloned slot. Non-fatal side-effect failures are reported through
// warn (the real path forwards them to the Wails runtime log); fatal steps
// return the same error messages the inline tail produced, leaving rollback to
// the caller.
func applyItemAddMutationPlan(slot *core.SaveSlot, plan itemAddMutationPlan, warn func(string, ...any)) error {
	// MUTATE: batch add all items (one RebuildSlotFull instead of N).
	if len(plan.batch) > 0 {
		if err := core.AddItemsToSlotBatch(slot, plan.batch); err != nil {
			return fmt.Errorf("rollback after batch add: %w", err)
		}
	}

	// KEY-ITEM STACK BUMPS: raise already-owned stackable key item rows in place
	// (issue 7). Runs after the batch add so it writes into the final slot.Data
	// (RebuildSlotFull may have re-parsed the slot). The KeyItems section is not
	// touched by the batch adder, so the recorded rows stay valid.
	for _, p := range plan.items {
		if p.keyTargetQty <= 0 {
			continue
		}
		if err := setInventoryKeyItemQuantity(slot, p.keyRow, uint32(p.keyTargetQty)); err != nil {
			return fmt.Errorf("rollback after key item qty update: %w", err)
		}
	}

	// POST-FLAGS: event flags, tutorial IDs (safe to set after batch add).
	for _, p := range plan.items {
		if flagID, ok := data.AoWItemToFlagID[p.baseID]; ok {
			if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
				if err := db.SetEventFlag(slot.Data[slot.EventFlagsOffset:], flagID, true); err != nil {
					warn("event flag AoW %d: %v", flagID, err)
				}
			}
		}
		if flagID, ok := data.WorldPickupFlagID[p.baseID]; ok {
			if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
				if err := db.SetEventFlag(slot.Data[slot.EventFlagsOffset:], flagID, true); err != nil {
					warn("event flag pickup %d: %v", flagID, err)
				}
			}
		}
		if flagList, ok := data.BolsteringPickupFlags[p.baseID]; ok {
			if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
				flags := slot.Data[slot.EventFlagsOffset:]
				sorted := make([]uint32, len(flagList))
				copy(sorted, flagList)
				sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
				qty := p.actualInv + p.actualStorage
				set := 0
				for _, f := range sorted {
					if set >= qty {
						break
					}
					if val, err := db.GetEventFlag(flags, f); err == nil && !val {
						if err := db.SetEventFlag(flags, f, true); err != nil {
							warn("bolstering pickup flag %d: %v", f, err)
						} else {
							set++
						}
					}
				}
			}
		}
		if tutorialID, ok := data.AboutTutorialID[p.baseID]; ok {
			if err := core.AppendTutorialID(slot, tutorialID); err != nil {
				warn("tutorial ID %d: %v", tutorialID, err)
			}
		}
		if companions := data.CompanionEventFlagsForItem(p.baseID); len(companions) > 0 {
			if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
				eflags := slot.Data[slot.EventFlagsOffset:]
				for _, f := range companions {
					if err := db.SetEventFlag(eflags, f, true); err != nil {
						warn("companion flag %d for item 0x%08X: %v", f, p.baseID, err)
					}
				}
			}
		}
	}

	// REQUIRED TUTORIAL IDS: unlike the best-effort data.AboutTutorialID loop
	// above, a failure here (e.g. a full TutorialData list) is fatal — it
	// rolls back the entire plan via the caller's snapshot restore, so a
	// bundle's confirmed native contract is never left partially written.
	for _, tutorialID := range plan.requiredTutorialIDs {
		if err := core.AppendTutorialID(slot, tutorialID); err != nil {
			return fmt.Errorf("rollback after required tutorial ID %d: %w", tutorialID, err)
		}
	}

	// Auto-add / update container quantities.
	for cID := range plan.usedContainers {
		desired := plan.existingByContainer[cID]
		current := inventoryContainerQuantity(slot, cID)
		finalQty := desired
		if current > finalQty {
			finalQty = current
		}
		// A Storage-only add consumes no per-unit container but still needs one
		// container present to hold the family; never below the existing count.
		if plan.storageContainers[cID] && finalQty < 1 {
			finalQty = 1
		}
		if finalQty > current {
			if err := upsertInventoryContainerQuantity(slot, cID, uint32(finalQty)); err != nil {
				return fmt.Errorf("rollback after container upsert: %w", err)
			}
		}

		if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
			continue
		}
		flags := slot.Data[slot.EventFlagsOffset:]
		if flagList, ok := data.ContainerPickupFlags[cID]; ok {
			n := finalQty
			if n > len(flagList) {
				n = len(flagList)
			}
			for i := 0; i < n; i++ {
				if err := db.SetEventFlag(flags, flagList[i], true); err != nil {
					warn("container pickup flag %d: %v", flagList[i], err)
				}
			}
		}

		if vendorFlags, ok := data.ContainerVendorPurchaseFlags[cID]; ok {
			for _, f := range vendorFlags {
				if err := db.SetEventFlag(flags, f, true); err != nil {
					warn("vendor purchase flag %d: %v", f, err)
				}
			}
		}
	}

	// RECONCILE: fix storage header count (blind +1 increment may drift).
	core.ReconcileStorageHeader(slot)

	// POST-VALIDATION: check invariants after mutation. The pre-flight guard
	// guarantees the slot was free of duplicate acquisition indices on entry,
	// so any duplicate detected here was introduced by this add and must roll
	// back the entire batch.
	if violations := core.ValidatePostMutation(slot); len(violations) > 0 {
		return fmt.Errorf("rollback: post-mutation validation failed: %s", violations[0].Error())
	}

	return nil
}
