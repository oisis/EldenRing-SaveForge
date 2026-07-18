package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

func removeInventoryHandleForItem(t *testing.T, app *App, itemID uint32) uint32 {
	t.Helper()
	slot := &app.save.Slots[0]
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle != 0 && giResolveItemID(slot, item.GaItemHandle) == giHex(itemID) {
			return item.GaItemHandle
		}
	}
	t.Fatalf("inventory handle for item 0x%08X not found", itemID)
	return 0
}

func assertRemoveSuccess(t *testing.T, lc removeLifecycle) {
	t.Helper()
	assertRemovePhaseGrouping(t, lc)
	if lc.outcome != string(characterChangeSuccess) {
		t.Errorf("outcome = %q, want success", lc.outcome)
	}
	if lc.stage != characterStageCompleted {
		t.Errorf("stage = %q, want completed", lc.stage)
	}
}

// A. The existing cleanup restores exactly one highest currently-set Golden Seed
// flag for one removed item. Lower set flags stay true and self-exclude.
func TestGameItemsRemoveFlagDiagnosticBolstering(t *testing.T) {
	app := gameItemAddApp(false)
	withFullEventFlags(app)
	flags := sortedBolsteringFlags(t, goldenSeedID)
	if _, err := app.AddItemsToCharacter(0, []uint32{goldenSeedID}, 0, 0, 0, 0, 3, 0); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}
	for i := 0; i < 3; i++ {
		if got := readContainerFlag(&app.save.Slots[0], flags[i]); got != "true" {
			t.Fatalf("seed flag %d = %q, want true", flags[i], got)
		}
	}

	handle := removeInventoryHandleForItem(t, app, goldenSeedID)
	journal := freshRemoveJournal(app, true)
	if err := app.RemoveItemsFromCharacter(0, []uint32{handle}, true, false); err != nil {
		t.Fatalf("RemoveItemsFromCharacter: %v", err)
	}

	lc := collectRemoveLifecycle(t, journal.Tail(), "0")
	assertRemoveSuccess(t, lc)
	highestSet := bolsteringField(goldenSeedID, flags[2])
	assertRemoveLifecycle(t, lc, highestSet, "true", "false", "false")
	if got := readContainerFlag(&app.save.Slots[0], flags[2]); got != lc.finished[highestSet] {
		t.Errorf("finished %s = %q, want real slot %q", highestSet, lc.finished[highestSet], got)
	}
	assertRemoveNoField(t, lc, bolsteringField(goldenSeedID, flags[0]))
	assertRemoveNoField(t, lc, bolsteringField(goldenSeedID, flags[1]))
}

// B. Removing the last Spectral Steed Whistle clears every mapped companion
// flag. The planner reads the processed clone rather than predicting the flags.
func TestGameItemsRemoveFlagDiagnosticCompanion(t *testing.T) {
	app := gameItemAddApp(false)
	withItemEventFlags(app)
	flags := data.CompanionEventFlagsForItem(data.ItemSpectralSteedWhistle)
	if len(flags) == 0 {
		t.Fatal("Spectral Steed Whistle has no companion flags")
	}
	if _, err := app.AddItemsToCharacter(0, []uint32{data.ItemSpectralSteedWhistle}, 0, 0, 0, 0, 1, 0); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}
	for _, flagID := range flags {
		if got := readContainerFlag(&app.save.Slots[0], flagID); got != "true" {
			t.Fatalf("seed companion flag %d = %q, want true", flagID, got)
		}
	}

	handle := removeInventoryHandleForItem(t, app, data.ItemSpectralSteedWhistle)
	journal := freshRemoveJournal(app, true)
	if err := app.RemoveItemsFromCharacter(0, []uint32{handle}, true, false); err != nil {
		t.Fatalf("RemoveItemsFromCharacter: %v", err)
	}

	lc := collectRemoveLifecycle(t, journal.Tail(), "0")
	assertRemoveSuccess(t, lc)
	for _, flagID := range flags {
		field := "item_0x40000082_companion_flag_" + giDec(flagID)
		assertRemoveLifecycle(t, lc, field, "true", "false", "false")
		if got := readContainerFlag(&app.save.Slots[0], flagID); got != lc.finished[field] {
			t.Errorf("finished %s = %q, want real slot %q", field, lc.finished[field], got)
		}
	}
}

// C. Without a valid Event Flags region, removal still succeeds but no cleanup
// flag lifecycle is emitted.
func TestGameItemsRemoveFlagDiagnosticNoEventFlagsRegion(t *testing.T) {
	app := gameItemAddApp(false)
	if _, err := app.AddItemsToCharacter(0, []uint32{goldenSeedID}, 0, 0, 0, 0, 1, 0); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}
	handle := removeInventoryHandleForItem(t, app, goldenSeedID)
	journal := freshRemoveJournal(app, true)
	if err := app.RemoveItemsFromCharacter(0, []uint32{handle}, true, false); err != nil {
		t.Fatalf("RemoveItemsFromCharacter: %v", err)
	}

	lc := collectRemoveLifecycle(t, journal.Tail(), "0")
	assertRemoveSuccess(t, lc)
	for _, phase := range []map[string]string{lc.before, lc.planned, lc.finished} {
		for field := range phase {
			if strings.Contains(field, "_bolstering_pickup_flag_") || strings.Contains(field, "_companion_flag_") {
				t.Errorf("cleanup flag field emitted without Event Flags region: %q", field)
			}
		}
	}
}

// D. Debug Mode only controls recording: existing bolstering cleanup still
// restores the flag when diagnostics are disabled.
func TestGameItemsRemoveFlagDiagnosticDebugOff(t *testing.T) {
	app := gameItemAddApp(false)
	withFullEventFlags(app)
	flags := sortedBolsteringFlags(t, goldenSeedID)
	if _, err := app.AddItemsToCharacter(0, []uint32{goldenSeedID}, 0, 0, 0, 0, 1, 0); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}
	if got := readContainerFlag(&app.save.Slots[0], flags[0]); got != "true" {
		t.Fatalf("seed flag %d = %q, want true", flags[0], got)
	}

	handle := removeInventoryHandleForItem(t, app, goldenSeedID)
	journal := freshRemoveJournal(app, false)
	if err := app.RemoveItemsFromCharacter(0, []uint32{handle}, true, false); err != nil {
		t.Fatalf("RemoveItemsFromCharacter: %v", err)
	}
	if got := readContainerFlag(&app.save.Slots[0], flags[0]); got != "false" {
		t.Errorf("cleanup flag %d = %q, want false", flags[0], got)
	}
	if lc := collectRemoveLifecycle(t, journal.Tail(), "0"); lc.count != 0 {
		t.Errorf("Debug-off removal emitted %d game_items_change_* records, want 0", lc.count)
	}
}

func assertRemoveNoField(t *testing.T, lc removeLifecycle, field string) {
	t.Helper()
	for _, phase := range []map[string]string{lc.before, lc.planned, lc.finished} {
		if _, ok := phase[field]; ok {
			t.Errorf("unexpected lifecycle record for %q", field)
			return
		}
	}
}
