package main

import (
	"sync"
	"testing"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// These tests verify the slotMu contract added in Phase 2: every
// per-character endpoint (slot writers, slot readers, the session
// Save commit, the multi-slot lockAllSlots readers) must serialise
// against the same slotMu[charIdx]. We do not poke production hooks
// — the gating is the lock itself, observed by parking the lock from
// the test goroutine and asserting the endpoint cannot complete until
// the lock is released.
//
// Run with the race detector:
//   go test -race -run TestSlot_ -count=10 .

// runBlocked launches fn on a goroutine, asserts it does not complete
// while the caller holds the slot lock, releases the lock, and waits
// for completion. Returns the error (or nil) the endpoint returned.
// The 50 ms window matches the existing inventory-session helpers and
// is not a timing assumption: the production code uses a blocking
// mutex, so if the worker raced past it the contract is broken
// regardless of timing slack.
func runBlocked(t *testing.T, label string, release func(), fn func() error) error {
	t.Helper()
	done := make(chan error, 1)
	go func() {
		done <- fn()
	}()

	select {
	case err := <-done:
		t.Fatalf("%s completed before the slot lock was released — lock contract broken (err=%v)", label, err)
	case <-time.After(50 * time.Millisecond):
		// expected: still blocked
	}

	release()

	select {
	case err := <-done:
		return err
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not complete after the slot lock was released — possible deadlock", label)
	}
	return nil
}

// TestSlot_AddItemsBlocksOnSlotLock proves AddItemsToCharacter acquires
// slotMu[charIdx]. We park the lock from the test goroutine and assert
// the endpoint cannot return until it is released. The Dagger item
// (0x000F4240) is the same fixture used by the existing inventory
// session concurrency tests.
//
// We do not assert a final item-count delta: the canonical fixture
// is already at inventory capacity (2688 items), so AddItemsToCharacter
// returns a populated AddResult with CapHit set and nil error — this
// is documented "all-or-nothing capacity check, no error on full
// inventory" semantics, not a lock failure. The lock contract is
// proven by the blocked → released → completed sequence.
func TestSlot_AddItemsBlocksOnSlotLock(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	app.slotMu[idx].Lock()
	released := false
	release := func() {
		if !released {
			released = true
			app.slotMu[idx].Unlock()
		}
	}
	defer release()

	err := runBlocked(t, "AddItemsToCharacter", release, func() error {
		_, e := app.AddItemsToCharacter(idx, []uint32{0x000F4240}, 0, 0, 0, 0, 1, 0)
		return e
	})
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
}

// TestSlot_BulkSetCookbooksBlocksOnSlotLock proves the representative
// world/item writer BulkSetCookbooksUnlocked respects slotMu[charIdx].
// Cookbook flag 67000 ("Nomadic Warrior's Cookbook [1]") maps to a
// real Key Item ID via data.CookbookFlagToItemID.
func TestSlot_BulkSetCookbooksBlocksOnSlotLock(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	app.slotMu[idx].Lock()
	released := false
	release := func() {
		if !released {
			released = true
			app.slotMu[idx].Unlock()
		}
	}
	defer release()

	err := runBlocked(t, "BulkSetCookbooksUnlocked", release, func() error {
		return app.BulkSetCookbooksUnlocked(idx, []uint32{67000}, true)
	})
	if err != nil {
		t.Fatalf("BulkSetCookbooksUnlocked: %v", err)
	}
}

// TestSlot_GetInventoryOrderBlocksOnSlotLock proves the reader that
// iterates the realised inventory / GaMap respects the same slotMu.
// "weapons" is one of the canonical Sort Order tabs accepted by the
// endpoint.
func TestSlot_GetInventoryOrderBlocksOnSlotLock(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	app.slotMu[idx].Lock()
	released := false
	release := func() {
		if !released {
			released = true
			app.slotMu[idx].Unlock()
		}
	}
	defer release()

	err := runBlocked(t, "GetInventoryOrder", release, func() error {
		_, e := app.GetInventoryOrder(idx, "weapons")
		return e
	})
	if err != nil {
		t.Fatalf("GetInventoryOrder: %v", err)
	}
}

// TestSlot_StartSessionBlocksOnSlotLock proves StartInventoryEditSession
// — which has to build a snapshot off the real slot bytes — respects
// slotMu[charIdx] when no prior session for this slot exists.
func TestSlot_StartSessionBlocksOnSlotLock(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	app.slotMu[idx].Lock()
	released := false
	release := func() {
		if !released {
			released = true
			app.slotMu[idx].Unlock()
		}
	}
	defer release()

	var snap editor.InventoryWorkspaceSnapshot
	err := runBlocked(t, "StartInventoryEditSession", release, func() error {
		var e error
		snap, e = app.StartInventoryEditSession(idx)
		return e
	})
	if err != nil {
		t.Fatalf("StartInventoryEditSession: %v", err)
	}
	if snap.SessionID == "" {
		t.Error("StartInventoryEditSession returned empty SessionID")
	}
}

// TestSlot_SessionSaveBlocksOnSlotLock is the mandatory cross-mutator
// gate: the real SaveInventoryWorkspaceChanges must take the same
// slotMu[charIdx] that non-session writers / readers take. Without
// it the production race that motivated this phase (GaMap rewrite vs
// AddItemsToCharacter) would re-open.
//
// We seed the workspace with a Move (no capacity dependency) so Save
// has a real commit to run. We do not assert on slot.Inventory content
// changes — the gate is the blocked → released → completed sequence,
// and the existing app_inventory_session_save_test.go suite already
// covers Save's functional correctness on a fresh fixture.
func TestSlot_SessionSaveBlocksOnSlotLock(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.InventoryItems) < 2 {
		t.Skip("need ≥2 inventory items to seed a workspace Move for Save")
	}
	// Seed a workspace mutation so Save has something to commit.
	// Move is capacity-neutral, unlike Add against a 2688-item fixture.
	if _, err := app.MoveInventoryWorkspaceItem(snap.SessionID,
		snap.InventoryItems[0].UID, "inventory", 1); err != nil {
		t.Fatalf("MoveInventoryWorkspaceItem: %v", err)
	}

	app.slotMu[idx].Lock()
	released := false
	release := func() {
		if !released {
			released = true
			app.slotMu[idx].Unlock()
		}
	}
	defer release()

	var updated editor.InventoryWorkspaceSnapshot
	err = runBlocked(t, "SaveInventoryWorkspaceChanges", release, func() error {
		var e error
		updated, e = app.SaveInventoryWorkspaceChanges(snap.SessionID)
		return e
	})
	if err != nil {
		t.Fatalf("SaveInventoryWorkspaceChanges: %v", err)
	}
	if updated.Dirty {
		t.Error("workspace Dirty should be false after Save committed")
	}
}

// TestSlot_TwoMutatorsSameSlotRaceFree is the race-detector gate: two
// non-session writers against the same slot must serialise cleanly
// under slotMu. The success contract is "both endpoints complete with
// no error and no race report" — we do not assert the final item count
// because the two writers are scheduling-dependent (one Add per call,
// plus a cookbook flag set).
func TestSlot_TwoMutatorsSameSlotRaceFree(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	const workers = 2
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, workers)

	wg.Add(workers)
	go func() {
		defer wg.Done()
		<-start
		_, errs[0] = app.AddItemsToCharacter(idx, []uint32{0x000F4240}, 0, 0, 0, 0, 1, 0)
	}()
	go func() {
		defer wg.Done()
		<-start
		errs[1] = app.BulkSetCookbooksUnlocked(idx, []uint32{67000}, true)
	}()
	close(start)
	wg.Wait()

	for i, e := range errs {
		if e != nil {
			t.Errorf("worker %d: %v", i, e)
		}
	}
}

// TestSlot_DifferentCharacterParallelism proves slotMu is per-slot,
// not global: an operation on slot N must succeed while slot M (M≠N)
// is locked. realSaveAppForSave returns the first active slot; we
// pick a second active slot from the same fixture.
func TestSlot_DifferentCharacterParallelism(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	otherIdx := -1
	for i := 0; i < 10; i++ {
		if i == idx {
			continue
		}
		if app.save.Slots[i].Version != 0 {
			otherIdx = i
			break
		}
	}
	if otherIdx < 0 {
		t.Skip("fixture has only one active slot; parallelism cannot be tested without a second slot")
	}

	app.slotMu[idx].Lock()
	defer app.slotMu[idx].Unlock()

	// Operation on the OTHER slot must complete without blocking,
	// because slotMu is per-slot. We wrap the call in a generous
	// timeout — if it deadlocks (slotMu accidentally shared across
	// slots) the test fails fast.
	done := make(chan error, 1)
	go func() {
		_, e := app.GetInventoryOrder(otherIdx, "weapons")
		done <- e
	}()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("GetInventoryOrder on slot %d (with slot %d locked): %v", otherIdx, idx, err)
		}
	case <-time.After(2 * time.Second):
		t.Fatalf("GetInventoryOrder on slot %d blocked while only slot %d was locked — slotMu is not per-slot", otherIdx, idx)
	}
}

// TestSlot_GetActiveSlotsBlocksOnAnySlotLock proves the multi-slot
// reader (which acquires every slotMu via lockAllSlots) blocks on
// any single slotMu held by a peer. We park slotMu[5] and assert
// GetActiveSlots cannot complete until we release it.
func TestSlot_GetActiveSlotsBlocksOnAnySlotLock(t *testing.T) {
	app, _ := realSaveAppForSave(t)

	app.slotMu[5].Lock()
	released := false
	release := func() {
		if !released {
			released = true
			app.slotMu[5].Unlock()
		}
	}
	defer release()

	var result []bool
	err := runBlocked(t, "GetActiveSlots", release, func() error {
		result = app.GetActiveSlots()
		return nil
	})
	if err != nil {
		t.Fatalf("GetActiveSlots: %v", err)
	}
	if len(result) != 10 {
		t.Fatalf("GetActiveSlots returned %d slots, want 10", len(result))
	}
}
