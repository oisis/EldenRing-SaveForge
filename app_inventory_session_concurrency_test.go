package main

import (
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// These tests cover the concurrency fix for the Inventory Edit Session
// subsystem. They drive realistic call patterns through the Wails-bound
// endpoints to reproduce — and gate against — the historical
// `fatal error: concurrent map writes` crash at StartInventoryEditSession
// plus the related per-session data races on Workspace /
// BaselineEditableHandles.
//
// Run with the race detector for the full guarantee, e.g.:
//   go test -race -run TestInventorySession_Concurrent -count=10 .
//
// Tests that need a populated save reuse realSaveAppForSave and skip
// when the tmp/ fixture isn't checked in.

// gate is a deterministic concurrency barrier: every worker calls
// wait() once it is ready to go and then blocks on the same channel,
// so all workers fire in the same scheduler tick after release().
// done() must be called after each worker finishes its critical
// section so the main goroutine's finish() can wait for all of them
// to return before reading their result slots.
type gate struct {
	ready sync.WaitGroup
	done  sync.WaitGroup
	ch    chan struct{}
}

func newGate(n int) *gate {
	g := &gate{ch: make(chan struct{})}
	g.ready.Add(n)
	g.done.Add(n)
	return g
}

// wait announces that the worker has reached the barrier and blocks
// until release() opens it.
func (g *gate) wait() {
	g.ready.Done()
	<-g.ch
}

// release waits for every worker to reach wait() then opens the
// barrier. Workers run after this call returns.
func (g *gate) release() {
	g.ready.Wait()
	close(g.ch)
}

// finish reports that the calling worker has completed.
func (g *gate) finish() {
	g.done.Done()
}

// join blocks until every worker has called finish().
func (g *gate) join() {
	g.done.Wait()
}

// TestInventorySession_ConcurrentStartSameChar reproduces the original
// crash signature: many goroutines fire StartInventoryEditSession against
// the same charIdx, racing on the editSessions and editSessionByChar
// map writes. With the fix the call is atomic — every worker observes a
// valid snapshot, no panic, and the registry settles with exactly one
// session that matches editSessionByChar[charIdx].
func TestInventorySession_ConcurrentStartSameChar(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	const workers = 50
	g := newGate(workers)
	results := make([]editor.InventoryWorkspaceSnapshot, workers)
	errs := make([]error, workers)

	for i := 0; i < workers; i++ {
		go func(i int) {
			defer g.finish()
			g.wait()
			snap, err := app.StartInventoryEditSession(idx)
			results[i] = snap
			errs[i] = err
		}(i)
	}
	g.release()
	g.join()

	// Issue one extra Start so the registry's "current" entry is
	// deterministic for the assertion below.
	finalSnap, finalErr := app.StartInventoryEditSession(idx)
	if finalErr != nil {
		t.Fatalf("final Start: %v", finalErr)
	}

	for i, err := range errs {
		if err != nil {
			t.Errorf("worker %d: %v", i, err)
		}
		if results[i].SessionID == "" {
			t.Errorf("worker %d: empty SessionID", i)
		}
	}

	// Registry consistency: exactly one session for this char, and the
	// reverse map points at the most recent Start.
	app.editSessionsMu.Lock()
	defer app.editSessionsMu.Unlock()
	if len(app.editSessions) != 1 {
		t.Errorf("editSessions: want 1 entry after concurrent Starts, got %d", len(app.editSessions))
	}
	gotID, ok := app.editSessionByChar[idx]
	if !ok {
		t.Fatalf("editSessionByChar[%d] missing", idx)
	}
	if gotID != finalSnap.SessionID {
		t.Errorf("editSessionByChar[%d] = %q, want %q (last Start)", idx, gotID, finalSnap.SessionID)
	}
	if _, ok := app.editSessions[gotID]; !ok {
		t.Errorf("editSessions[%q] missing — registry maps out of sync", gotID)
	}
}

// TestInventorySession_ConcurrentMutations fans out a representative
// mix of mutation and read endpoints against a single live session.
// The per-session lock must serialise them — without it the slice
// reorder / append paths in editor.MoveItem and editor.AddItem race on
// sess.Workspace.InventoryItems. The race detector is the gating check;
// the post-mortem assertions just sanity-check that the workspace did
// not lose every item.
func TestInventorySession_ConcurrentMutations(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.InventoryItems) < 3 {
		t.Skip("need ≥3 inventory items to exercise concurrent move + validate")
	}
	sessionID := snap.SessionID
	uid := snap.InventoryItems[0].UID

	const workers = 32
	g := newGate(workers)
	errs := make([]error, workers)

	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer g.finish()
			g.wait()
			switch i % 4 {
			case 0:
				// Reorder the same item — every call is safe in isolation
				// and the lock must keep the slice consistent under
				// concurrent reorders.
				_, errs[i] = app.MoveInventoryWorkspaceItem(sessionID, uid, "inventory", (i%3)+1)
			case 1:
				// Pure read — must not tear the snapshot.
				_, errs[i] = app.GetInventoryEditSession(sessionID)
			case 2:
				// Validation rewrites sess.Workspace.Validation — covers
				// the read+write hot path that historically raced reads.
				_, errs[i] = app.ValidateInventoryWorkspace(sessionID)
			case 3:
				// Add a Dagger to grow the inventory slice; covers the
				// append path on sess.Workspace.InventoryItems.
				_, errs[i] = app.AddInventoryWorkspaceItem(sessionID,
					editor.AddItemSpec{ItemID: 0x000F4240}, "inventory", 0)
			}
		}()
	}
	g.release()
	g.join()

	final, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("final Get: %v", err)
	}
	for i, e := range errs {
		if e != nil {
			t.Errorf("worker %d: %v", i, e)
		}
	}
	if len(final.InventoryItems) == 0 {
		t.Error("workspace lost every inventory item — concurrent mutations corrupted state")
	}
}

// TestInventorySession_ConcurrentSaveAndMutation runs
// SaveInventoryWorkspaceChanges in parallel with a mutator on the same
// session. The mutator must NOT observe a half-replaced Workspace or a
// baseline map in the middle of regeneration — both were possible
// before the fix because Save swaps sess.Workspace and rebuilds
// sess.BaselineEditableHandles in non-atomic steps.
//
// Functional outcome: at the end the workspace must be coherent
// (Save succeeded, baseline regenerated) and a follow-up Add must not
// crash.
func TestInventorySession_ConcurrentSaveAndMutation(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.InventoryItems) < 2 {
		t.Skip("need ≥2 inventory items to exercise reorder + concurrent save")
	}
	sessionID := snap.SessionID

	// Seed a pending mutation so Save has something to commit.
	if _, err := app.MoveInventoryWorkspaceItem(sessionID, snap.InventoryItems[0].UID, "inventory", 1); err != nil {
		t.Fatalf("seed Move: %v", err)
	}

	g := newGate(2)
	var saveErr, addErr error

	go func() {
		defer g.finish()
		g.wait()
		_, saveErr = app.SaveInventoryWorkspaceChanges(sessionID)
	}()
	go func() {
		defer g.finish()
		g.wait()
		// Add will either land in the pre-save workspace (committed to
		// disk as part of Save) or in the post-save workspace
		// (subsequent dirty state). Either ordering is fine — the test
		// is gating on no panic / no torn state.
		_, addErr = app.AddInventoryWorkspaceItem(sessionID,
			editor.AddItemSpec{ItemID: 0x000F4240}, "inventory", 0)
	}()
	g.release()
	g.join()

	final, err := app.GetInventoryEditSession(sessionID)
	if err != nil {
		t.Fatalf("final Get: %v", err)
	}
	if saveErr != nil {
		t.Errorf("Save: %v", saveErr)
	}
	if addErr != nil {
		t.Errorf("Add: %v", addErr)
	}
	if final.SessionID != sessionID {
		t.Errorf("final SessionID = %q, want %q", final.SessionID, sessionID)
	}
	// Confirm baseline is internally consistent: another Add must not
	// panic on a torn baseline map.
	if _, err := app.AddInventoryWorkspaceItem(sessionID,
		editor.AddItemSpec{ItemID: 0x000F4240}, "inventory", 0); err != nil {
		t.Errorf("post-race Add: %v", err)
	}
}

// TestInventorySession_DiscardDuringMutation drives Discard against a
// session that has a mutation in flight. The mutator must either
// complete cleanly (it acquired the lock before Discard) or fail with
// "not found" (Discard won the race). After Discard returns no further
// endpoint may write to the orphan session.
func TestInventorySession_DiscardDuringMutation(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.InventoryItems) < 2 {
		t.Skip("need ≥2 inventory items to exercise concurrent move + discard")
	}
	sessionID := snap.SessionID
	uid := snap.InventoryItems[0].UID

	g := newGate(2)
	var moveErr, discardErr error

	go func() {
		defer g.finish()
		g.wait()
		_, moveErr = app.MoveInventoryWorkspaceItem(sessionID, uid, "inventory", 1)
	}()
	go func() {
		defer g.finish()
		g.wait()
		discardErr = app.DiscardInventoryEditSession(sessionID)
	}()
	g.release()
	g.join()

	// Both racers have returned; the next Move must fail because
	// Discard removed the session from the registry.
	_, postErr := app.MoveInventoryWorkspaceItem(sessionID, uid, "inventory", 0)
	if discardErr != nil {
		t.Errorf("Discard: %v", discardErr)
	}
	if moveErr != nil && !strings.Contains(moveErr.Error(), "not found") {
		t.Errorf("racing Move: want nil or 'not found', got %v", moveErr)
	}
	if postErr == nil || !strings.Contains(postErr.Error(), "not found") {
		t.Errorf("post-Discard Move: want 'not found', got %v", postErr)
	}

	// Registry must be empty for this charIdx.
	app.editSessionsMu.Lock()
	defer app.editSessionsMu.Unlock()
	if _, ok := app.editSessions[sessionID]; ok {
		t.Error("editSessions still contains discarded session ID")
	}
	if got, ok := app.editSessionByChar[idx]; ok {
		t.Errorf("editSessionByChar[%d] = %q after Discard, want missing", idx, got)
	}
}

// TestInventorySession_ClearAllDuringMutation races
// clearAllEditSessions (used by SelectAndOpenSave / Reload) against a
// live mutator. After clearAll returns the registry is empty and any
// subsequent endpoint that uses the old session ID fails with
// "not found"; no panic, no map write race.
func TestInventorySession_ClearAllDuringMutation(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start: %v", err)
	}
	if len(snap.InventoryItems) < 2 {
		t.Skip("need ≥2 inventory items to exercise concurrent mutation + clearAll")
	}
	sessionID := snap.SessionID
	uid := snap.InventoryItems[0].UID

	g := newGate(2)
	var mutationErr error

	go func() {
		defer g.finish()
		g.wait()
		_, mutationErr = app.MoveInventoryWorkspaceItem(sessionID, uid, "inventory", 1)
	}()
	go func() {
		defer g.finish()
		g.wait()
		app.clearAllEditSessions()
	}()
	g.release()
	g.join()

	_, postErr := app.GetInventoryEditSession(sessionID)
	if mutationErr != nil && !strings.Contains(mutationErr.Error(), "not found") {
		t.Errorf("racing mutation: want nil or 'not found', got %v", mutationErr)
	}
	if postErr == nil || !strings.Contains(postErr.Error(), "not found") {
		t.Errorf("post-clearAll Get: want 'not found', got %v", postErr)
	}

	app.editSessionsMu.Lock()
	defer app.editSessionsMu.Unlock()
	if len(app.editSessions) != 0 {
		t.Errorf("editSessions: want 0 entries after clearAll, got %d", len(app.editSessions))
	}
	if len(app.editSessionByChar) != 0 {
		t.Errorf("editSessionByChar: want 0 entries after clearAll, got %d", len(app.editSessionByChar))
	}
}

// ─── Lifecycle Save × Start/Discard/Clear tests ───────────────────────
//
// These three tests close the cross-session slot race that the per-session
// mutex alone could not cover. Each one parks a fake "Save in flight" by
// holding the per-session lock directly (editor.InventoryEditSession.Acquire
// is the same primitive SaveInventoryWorkspaceChanges takes), then drives
// the conflicting lifecycle endpoint in a goroutine and asserts the
// endpoint cannot complete until the Save lock is released. We rely on the
// production code's own lifecycleMu serialisation, not on test hooks — the
// only test-only manipulation is acquiring the per-session mutex via the
// exported Acquire/Unlock API.

// expectStillBlocked asserts the worker has not signalled completion
// within a short observation window. This is a bounded sleep, not a
// timing assumption — the production code uses a blocking lock; if the
// worker raced past it, the lock contract is broken regardless of
// timing slack.
func expectStillBlocked(t *testing.T, label string, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
		t.Fatalf("%s completed before the simulated Save released its lock — lifecycle race", label)
	case <-time.After(50 * time.Millisecond):
		// expected: still blocked
	}
}

// expectCompleted waits up to a generous timeout for the worker to
// finish after we release the simulated Save lock. If it doesn't, the
// blocking endpoint never woke up — either we leaked a lock or the lock
// chain is broken.
func expectCompleted(t *testing.T, label string, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
		// good
	case <-time.After(5 * time.Second):
		t.Fatalf("%s did not complete after the simulated Save lock was released", label)
	}
}

// TestInventorySession_StartReplacementDrainsActiveSave parks a fake
// Save (by directly holding the session's per-session lock) and then
// fires a replacement Start for the same charIdx. The Start must NOT
// build the new snapshot or publish the new session ID until the lock
// is released — otherwise editor.StartSession would race
// ApplyWorkspaceSave on a.save.Slots[charIdx].
func TestInventorySession_StartReplacementDrainsActiveSave(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap1, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start #1: %v", err)
	}

	// Simulate "Save in flight" on the first session by holding its
	// per-session lock — this is the exact mutex SaveInventoryWorkspaceChanges
	// holds across the whole apply / rebuild / regenerate sequence.
	sess1 := app.editSessions[snap1.SessionID]
	if sess1 == nil {
		t.Fatal("session #1 missing from registry")
	}
	if err := sess1.Acquire(); err != nil {
		t.Fatalf("Acquire session #1 lock: %v", err)
	}

	done := make(chan struct{})
	var startErr error
	var snap2 editor.InventoryWorkspaceSnapshot
	go func() {
		defer close(done)
		snap2, startErr = app.StartInventoryEditSession(idx)
	}()

	// Start must block at closeSession(prior) waiting for our lock.
	expectStillBlocked(t, "replacement Start", done)

	// Inspect the registry: the prior session ID must have been evicted
	// even though closeSession is still pending. This proves Start
	// progressed past the registry-delete step but is parked at the
	// drain — i.e. peer endpoints already see the prior session as
	// gone, but the new snapshot has not been built yet.
	app.editSessionsMu.Lock()
	if _, stillThere := app.editSessions[snap1.SessionID]; stillThere {
		app.editSessionsMu.Unlock()
		t.Fatal("prior session ID still in registry while replacement Start was parked")
	}
	app.editSessionsMu.Unlock()

	// Release the simulated Save — Start should now drain, build the
	// new snapshot off the quiesced slot, publish, and return.
	sess1.Unlock()
	expectCompleted(t, "replacement Start", done)

	if startErr != nil {
		t.Fatalf("replacement Start: %v", startErr)
	}
	if snap2.SessionID == "" || snap2.SessionID == snap1.SessionID {
		t.Fatalf("replacement Start returned suspicious SessionID: prior=%q new=%q", snap1.SessionID, snap2.SessionID)
	}

	app.editSessionsMu.Lock()
	defer app.editSessionsMu.Unlock()
	if got := app.editSessionByChar[idx]; got != snap2.SessionID {
		t.Errorf("editSessionByChar[%d] = %q, want %q", idx, got, snap2.SessionID)
	}
	if _, ok := app.editSessions[snap2.SessionID]; !ok {
		t.Errorf("editSessions missing the new session %q", snap2.SessionID)
	}
}

// TestInventorySession_DiscardThenStartDrainsActiveSave parks a fake
// Save, then races Discard + new Start for the same charIdx. Discard
// must wait for the per-session lock, and the new Start must wait for
// the lifecycleMu Discard holds — so the new Start cannot read or
// publish anything for this slot until Discard's drain completes.
func TestInventorySession_DiscardThenStartDrainsActiveSave(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap1, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start #1: %v", err)
	}
	sess1 := app.editSessions[snap1.SessionID]
	if sess1 == nil {
		t.Fatal("session #1 missing from registry")
	}
	if err := sess1.Acquire(); err != nil {
		t.Fatalf("Acquire session #1 lock: %v", err)
	}

	// Discard goroutine — will block at closeSession on our held lock.
	discardDone := make(chan struct{})
	var discardErr error
	go func() {
		defer close(discardDone)
		discardErr = app.DiscardInventoryEditSession(snap1.SessionID)
	}()
	expectStillBlocked(t, "Discard", discardDone)

	// New Start goroutine for the SAME charIdx — must block on
	// lifecycleMu[idx] which Discard holds.
	startDone := make(chan struct{})
	var startErr error
	var snap2 editor.InventoryWorkspaceSnapshot
	go func() {
		defer close(startDone)
		snap2, startErr = app.StartInventoryEditSession(idx)
	}()
	expectStillBlocked(t, "new Start during Discard", startDone)

	// Release the simulated Save. Discard's drain completes first;
	// then the new Start gets the lifecycle lock, sees no prior, and
	// builds + publishes its snapshot.
	sess1.Unlock()
	expectCompleted(t, "Discard", discardDone)
	expectCompleted(t, "new Start", startDone)

	if discardErr != nil {
		t.Errorf("Discard: %v", discardErr)
	}
	if startErr != nil {
		t.Fatalf("new Start: %v", startErr)
	}
	if snap2.SessionID == "" || snap2.SessionID == snap1.SessionID {
		t.Fatalf("new Start SessionID prior=%q new=%q", snap1.SessionID, snap2.SessionID)
	}

	app.editSessionsMu.Lock()
	defer app.editSessionsMu.Unlock()
	if _, stillThere := app.editSessions[snap1.SessionID]; stillThere {
		t.Error("Discarded session ID still in registry after both ops completed")
	}
	if got := app.editSessionByChar[idx]; got != snap2.SessionID {
		t.Errorf("editSessionByChar[%d] = %q, want %q", idx, got, snap2.SessionID)
	}
}

// TestInventorySession_ClearAllThenStartDrainsActiveSave parks a fake
// Save, then races clearAllEditSessions + new Start for the same
// charIdx. clearAll acquires every lifecycleMu in order, so the new
// Start cannot enter while clearAll is still draining the parked
// session. After we release the lock, clearAll completes and Start
// proceeds against the cleared registry.
func TestInventorySession_ClearAllThenStartDrainsActiveSave(t *testing.T) {
	app, idx := realSaveAppForSave(t)
	snap1, err := app.StartInventoryEditSession(idx)
	if err != nil {
		t.Fatalf("Start #1: %v", err)
	}
	sess1 := app.editSessions[snap1.SessionID]
	if sess1 == nil {
		t.Fatal("session #1 missing from registry")
	}
	if err := sess1.Acquire(); err != nil {
		t.Fatalf("Acquire session #1 lock: %v", err)
	}

	clearDone := make(chan struct{})
	go func() {
		defer close(clearDone)
		app.clearAllEditSessions()
	}()
	expectStillBlocked(t, "clearAllEditSessions", clearDone)

	startDone := make(chan struct{})
	var startErr error
	var snap2 editor.InventoryWorkspaceSnapshot
	go func() {
		defer close(startDone)
		snap2, startErr = app.StartInventoryEditSession(idx)
	}()
	expectStillBlocked(t, "new Start during clearAll", startDone)

	sess1.Unlock()
	expectCompleted(t, "clearAllEditSessions", clearDone)
	expectCompleted(t, "new Start", startDone)

	if startErr != nil {
		t.Fatalf("new Start: %v", startErr)
	}
	if snap2.SessionID == "" || snap2.SessionID == snap1.SessionID {
		t.Fatalf("new Start SessionID prior=%q new=%q", snap1.SessionID, snap2.SessionID)
	}

	app.editSessionsMu.Lock()
	defer app.editSessionsMu.Unlock()
	if len(app.editSessions) != 1 {
		t.Errorf("editSessions: want 1 (the post-clear Start), got %d", len(app.editSessions))
	}
	if got := app.editSessionByChar[idx]; got != snap2.SessionID {
		t.Errorf("editSessionByChar[%d] = %q, want %q", idx, got, snap2.SessionID)
	}
}

// TestInventorySession_ClosedSessionFailsAcquire is a unit-level
// sanity check on the editor.InventoryEditSession Close/Acquire pair.
// Together with the integration tests above it documents the contract
// the App endpoints depend on.
func TestInventorySession_ClosedSessionFailsAcquire(t *testing.T) {
	sess := &editor.InventoryEditSession{}
	if err := sess.Acquire(); err != nil {
		t.Fatalf("first Acquire: %v", err)
	}
	sess.Close()
	sess.Unlock()
	if err := sess.Acquire(); err == nil {
		t.Fatal("Acquire after Close: want ErrSessionClosed, got nil")
	} else if err != editor.ErrSessionClosed {
		t.Errorf("Acquire after Close: want ErrSessionClosed, got %v", err)
	}
	if !sess.IsClosed() {
		t.Error("IsClosed: want true after Close")
	}
}
