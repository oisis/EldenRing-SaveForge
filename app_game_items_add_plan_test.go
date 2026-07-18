package main

import (
	"bytes"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// noWarn is the diagnostic/clone-side warn sink: the executor forwards non-fatal
// side-effect failures to it, but these fixture runs expect none.
func noWarn(string, ...any) {}

// firePotInventoryPlan mirrors the plan addItemsToCharacter builds for adding
// `inv` Fire Pots to inventory: the batch entry, the post-flag item, and the
// Cracked Pot container whose occupancy ApplyContainerCap has already totalled.
func firePotInventoryPlan(t *testing.T, inv int) itemAddMutationPlan {
	t.Helper()
	cID, gated := data.GetRequiredContainer(firePotID)
	if !gated {
		t.Fatalf("Fire Pot 0x%08X expected to be a gated (container) item", firePotID)
	}
	return itemAddMutationPlan{
		batch:               []core.ItemToAdd{{ItemID: firePotID, InvQty: inv}},
		items:               []itemAddPlanItem{{baseID: firePotID, actualInv: inv, keyRow: -1}},
		usedContainers:      map[uint32]bool{cID: true},
		storageContainers:   map[uint32]bool{},
		existingByContainer: map[uint32]int{cID: inv},
	}
}

// TestItemAddMutationPlanCloneIsolation proves the extracted executor is safe to
// run against core.CloneSlot(slot): the clone gains the add while the original
// slot stays byte-for-byte unchanged. This is the property the future diagnostic
// planner depends on.
func TestItemAddMutationPlanCloneIsolation(t *testing.T) {
	app := remembranceGameLimitsFixture()
	withContainerEventFlags(app)
	slot := &app.save.Slots[0]

	plan := firePotInventoryPlan(t, 2)
	origData := append([]byte(nil), slot.Data...)

	clone := core.CloneSlot(slot)
	if err := applyItemAddMutationPlan(clone, plan, noWarn); err != nil {
		t.Fatalf("applyItemAddMutationPlan on clone: %v", err)
	}

	// Clone changed: Fire Pot and its container are now present.
	cloneInvStart := clone.MagicOffset + core.InvStartFromMagic
	if _, ok := quantityInRecords(clone.Data, cloneInvStart, core.CommonItemCount, firePotHandle); !ok {
		t.Error("clone missing Fire Pot after executor")
	}
	if row, _ := findCommonItem(clone, crackedPotHandle); row < 0 {
		t.Error("clone missing Cracked Pot container after executor")
	}

	// Original untouched — both in the binary buffer and the parsed inventory.
	if !bytes.Equal(slot.Data, origData) {
		t.Error("original slot.Data mutated by executor run on clone")
	}
	if row, _ := findCommonItem(slot, firePotHandle); row >= 0 {
		t.Error("original slot gained a Fire Pot from a clone-only mutation")
	}
}

// TestItemAddMutationPlanGatedAdd proves the executor preserves the gated-add
// behavior: item addition, required container update, and pickup flags when an
// Event Flags region exists.
func TestItemAddMutationPlanGatedAdd(t *testing.T) {
	app := remembranceGameLimitsFixture()
	withContainerEventFlags(app)
	slot := &app.save.Slots[0]

	if err := applyItemAddMutationPlan(slot, firePotInventoryPlan(t, 2), noWarn); err != nil {
		t.Fatalf("applyItemAddMutationPlan: %v", err)
	}

	invStart := slot.MagicOffset + core.InvStartFromMagic
	if _, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, firePotHandle); !ok {
		t.Error("Fire Pot missing from CommonItems")
	}
	if got, ok := quantityInRecords(slot.Data, invStart, core.CommonItemCount, crackedPotHandle); !ok || got != 2 {
		t.Errorf("Cracked Pot container qty = %d, want 2", got)
	}
	if keyRow, _ := findKeyItem(slot, crackedPotHandle); keyRow >= 0 {
		t.Error("Cracked Pot leaked into KeyItems")
	}

	pickup := data.ContainerPickupFlags[data.CrackedPotKeyItemID]
	flags := slot.Data[slot.EventFlagsOffset:]
	for i := 0; i < 2; i++ {
		if set, err := db.GetEventFlag(flags, pickup[i]); err != nil || !set {
			t.Errorf("pickup flag[%d] set=%v err=%v, want set", i, set, err)
		}
	}
}

// TestItemAddMutationPlanStorageOnly proves the executor keeps the storage-only
// minimum-one-container behavior: a Storage add consumes no per-unit container,
// so the container is forced to exactly 1 and only its first pickup flag is set.
func TestItemAddMutationPlanStorageOnly(t *testing.T) {
	app := remembranceGameLimitsFixture()
	slot := &app.save.Slots[0]
	slot.Inventory.KeyItems = make([]core.InventoryItem, core.KeyItemCount)
	withContainerEventFlags(app)

	cID, gated := data.GetRequiredContainer(sparkAromaticID)
	if !gated {
		t.Fatalf("Spark Aromatic 0x%08X expected to be a gated item", sparkAromaticID)
	}
	plan := itemAddMutationPlan{
		batch:               []core.ItemToAdd{{ItemID: sparkAromaticID, StorageQty: 2}},
		items:               []itemAddPlanItem{{baseID: sparkAromaticID, actualStorage: 2, keyRow: -1}},
		usedContainers:      map[uint32]bool{cID: true},
		storageContainers:   map[uint32]bool{cID: true},
		existingByContainer: map[uint32]int{},
	}
	if err := applyItemAddMutationPlan(slot, plan, noWarn); err != nil {
		t.Fatalf("applyItemAddMutationPlan: %v", err)
	}

	row, qty := findCommonItem(slot, perfumeBottleHandle)
	if row < 0 {
		t.Fatal("Perfume Bottle not created from a Storage-only add")
	}
	if qty != 1 {
		t.Errorf("Perfume Bottle qty = %d, want 1 (Storage consumes no per-unit container)", qty)
	}
	if keyRow, _ := findKeyItem(slot, perfumeBottleHandle); keyRow >= 0 {
		t.Error("Perfume Bottle leaked into KeyItems")
	}

	pickup := data.ContainerPickupFlags[data.PerfumeBottleKeyItemID]
	flags := slot.Data[slot.EventFlagsOffset:]
	if set, err := db.GetEventFlag(flags, pickup[0]); err != nil || !set {
		t.Errorf("first pickup flag set=%v err=%v, want set", set, err)
	}
	if set, err := db.GetEventFlag(flags, pickup[1]); err != nil || set {
		t.Errorf("second pickup flag set=%v err=%v, want unset", set, err)
	}
}
