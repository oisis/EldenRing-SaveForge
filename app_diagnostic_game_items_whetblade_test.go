package main

import (
	"fmt"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// Whetblade single-item unlock lifecycle coverage for SetWhetbladeUnlocked.
// Fixture-free: state is built and mutated only through the public App API, and
// every record is read back from the in-memory journal.
//
// The system affinity flag 1042378601 (a related flag of the Whetstone Knife)
// resolves to byte ~658700, so the carved event-flags region is sized well past
// it; the smaller shared itemEventFlagsRegionSize would silently drop that flag.
const whetbladeEventFlagsRegionSize = 0x100000

// findInvField locates the item across the physical inventory/storage sections
// and returns the lifecycle field prefix ("<section>_row_<n>") for that row.
func findInvField(t *testing.T, slot *core.SaveSlot, itemID uint32) string {
	t.Helper()
	sections := []struct {
		sec  giSection
		name string
	}{
		{giSecInventoryCommon, "inventory_common"},
		{giSecInventoryKey, "inventory_key"},
		{giSecStorageCommon, "storage_common"},
	}
	want := giHex(itemID)
	for _, s := range sections {
		for i := range giInvRows(slot, s.sec) {
			if readGameItemField(slot, s.sec, i, giItemID) == want {
				return fmt.Sprintf("%s_row_%d", s.name, i)
			}
		}
	}
	t.Fatalf("item 0x%08X not found in any inventory section", itemID)
	return ""
}

// A. Whetstone Knife unlock from a clean slot: the main flag, both related
// flags, the Storm Stomp duplication flag and the AoW menu flag all flip
// false -> true, and the Whetstone Knife item is added.
//
// The Storm Stomp Ash of War (0x8000C418) is a best-effort add the writer makes
// alongside its duplication flag; it is not added in this inventory fixture, so
// only the flags it actually owns (65841) are asserted — the planner logs solely
// what lands, never a flag or item the operation failed to write.
func TestGameItemsUnlockWhetbladeLifecycle(t *testing.T) {
	const whetItem = uint32(0x4000218E)

	app := gameItemAddApp(false)
	withUnlockEventFlags(app, whetbladeEventFlagsRegionSize)

	journal := freshRemoveJournal(app, true)
	owned, err := app.SetWhetbladeUnlocked(0, data.WhetstoneKnifeFlag, true)
	if err != nil {
		t.Fatalf("SetWhetbladeUnlocked: %v", err)
	}
	if owned {
		t.Errorf("alreadyOwned = true, want false on a clean slot")
	}

	slot := &app.save.Slots[0]
	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsUnlockWhetblade, "0")
	assertUnlockSuccess(t, lc)

	// Every owned flag flips false -> true; finished == real slot.
	flags := []uint32{data.WhetstoneKnifeFlag, 65600, 1042378601, data.StormStompDupFlag, data.AoWMenuUnlockedFlag}
	for _, f := range flags {
		assertUnlockLifecycle(t, lc, "event_flag_"+giDec(f), "false", "true", "true")
		if got := readContainerFlag(slot, f); got != lc.finished["event_flag_"+giDec(f)] {
			t.Errorf("finished flag %d %q != real slot %q", f, lc.finished["event_flag_"+giDec(f)], got)
		}
	}

	// The Whetstone Knife item is added with a full lifecycle; finished == real slot.
	p := findInvField(t, slot, whetItem)
	assertUnlockLifecycle(t, lc, p+"_item_id", giAbsent, giHex(whetItem), giHex(whetItem))
	if got := readGameItemField(slot, giSecInventoryCommon, 0, giItemID); got != lc.finished[p+"_item_id"] {
		t.Errorf("finished item_id %q != real slot %q", lc.finished[p+"_item_id"], got)
	}
}

// B. Lock the last active Whetstone Knife: seeded Debug-off so only the lock is
// captured. The Whetstone Knife item is removed and every owned flag — including
// the AoW menu flag — is cleared, and alreadyOwned reports true before the lock.
func TestGameItemsLockWhetbladeLastActive(t *testing.T) {
	const whetItem = uint32(0x4000218E)

	app := gameItemAddApp(false)
	withUnlockEventFlags(app, whetbladeEventFlagsRegionSize)

	// Seed the Whetstone Knife (Debug off: no records leak into the lock test).
	if _, err := app.SetWhetbladeUnlocked(0, data.WhetstoneKnifeFlag, true); err != nil {
		t.Fatalf("seed SetWhetbladeUnlocked: %v", err)
	}
	slot := &app.save.Slots[0]
	whetField := findInvField(t, slot, whetItem)

	journal := freshRemoveJournal(app, true)
	owned, err := app.SetWhetbladeUnlocked(0, data.WhetstoneKnifeFlag, false)
	if err != nil {
		t.Fatalf("lock SetWhetbladeUnlocked: %v", err)
	}
	if !owned {
		t.Errorf("alreadyOwned = false, want true before locking a seeded whetblade")
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsUnlockWhetblade, "0")
	assertUnlockSuccess(t, lc)

	// Every owned flag flips true -> false, including the AoW menu flag.
	flags := []uint32{data.WhetstoneKnifeFlag, 65600, 1042378601, data.StormStompDupFlag, data.AoWMenuUnlockedFlag}
	for _, f := range flags {
		assertUnlockLifecycle(t, lc, "event_flag_"+giDec(f), "true", "false", "false")
	}

	// The Whetstone Knife item is removed: its row loses the resolved item_id.
	assertUnlockLifecycle(t, lc, whetField+"_item_id", giHex(whetItem), giAbsent, giAbsent)
}

// C. Lock one whetblade while another stays active: the AoW menu flag must not
// appear in any phase (before == planned == true), while the locked whetblade's
// own flags still have a full lifecycle.
func TestGameItemsLockWhetbladeOtherActive(t *testing.T) {
	app := gameItemAddApp(false)
	withUnlockEventFlags(app, whetbladeEventFlagsRegionSize)

	// Seed two whetblades Debug-off; lock only the Iron Whetblade.
	if _, err := app.SetWhetbladeUnlocked(0, data.WhetstoneKnifeFlag, true); err != nil {
		t.Fatalf("seed Whetstone Knife: %v", err)
	}
	if _, err := app.SetWhetbladeUnlocked(0, 65610, true); err != nil {
		t.Fatalf("seed Iron Whetblade: %v", err)
	}

	journal := freshRemoveJournal(app, true)
	if _, err := app.SetWhetbladeUnlocked(0, 65610, false); err != nil {
		t.Fatalf("lock Iron Whetblade: %v", err)
	}

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsUnlockWhetblade, "0")
	assertUnlockSuccess(t, lc)

	// AoW menu flag stays true because the Whetstone Knife remains active.
	assertFieldAbsent(t, lc, "event_flag_"+giDec(data.AoWMenuUnlockedFlag))

	// Iron Whetblade's own flags (main + Keen + Quality) flip true -> false.
	for _, f := range []uint32{65610, 65620, 65630} {
		assertUnlockLifecycle(t, lc, "event_flag_"+giDec(f), "true", "false", "false")
	}
}

// D. Debug off: the whetblade unlock still mutates the slot, but no lifecycle
// records are retained.
func TestGameItemsUnlockWhetbladeDebugOff(t *testing.T) {
	app := gameItemAddApp(false)
	withUnlockEventFlags(app, whetbladeEventFlagsRegionSize)

	journal := freshRemoveJournal(app, false) // Debug Mode disabled
	if _, err := app.SetWhetbladeUnlocked(0, data.WhetstoneKnifeFlag, true); err != nil {
		t.Fatalf("SetWhetbladeUnlocked: %v", err)
	}

	slot := &app.save.Slots[0]
	if got := readContainerFlag(slot, data.WhetstoneKnifeFlag); got != "true" {
		t.Errorf("whetblade flag not set: %q", got)
	}
	findInvField(t, slot, 0x4000218E) // Whetstone Knife item added

	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsUnlockWhetblade, "0")
	if lc.count != 0 {
		t.Errorf("Debug-off unlock emitted %d game_items_change_* records, want 0", lc.count)
	}
}
