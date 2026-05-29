package main

import (
	"os"
	"strings"
	"sync"
	"testing"
	"time"
)

// TestFavorites_WriteBlocksOnFavoritesLock proves the favorites writer
// (RemoveFavoritePreset) takes favMu.Lock — without it the favorites
// blob in UserData10 and favSlotNames could race with peer writers.
// We park favMu.Lock from the test and assert the endpoint cannot
// return until we release.
func TestFavorites_WriteBlocksOnFavoritesLock(t *testing.T) {
	app, _ := realSaveAppForSave(t)

	app.favMu.Lock()
	released := false
	release := func() {
		if !released {
			released = true
			app.favMu.Unlock()
		}
	}
	defer release()

	done := make(chan error, 1)
	go func() {
		done <- app.RemoveFavoritePreset(0)
	}()

	select {
	case err := <-done:
		t.Fatalf("RemoveFavoritePreset completed while favMu.Lock was held by the test (err=%v) — writer is not taking favMu", err)
	case <-time.After(50 * time.Millisecond):
		// expected
	}

	release()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("RemoveFavoritePreset: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RemoveFavoritePreset did not complete after favMu release")
	}
}

// TestFavorites_ConcurrentRemoveAndWriteRaceFree is the race-detector
// stress test: parallel Remove/Write/Read endpoints must not corrupt
// favSlotNames or UserData10 and must not panic with "concurrent map
// writes". We do not assert a single winner — the test gate is "no
// panic, no race report, endpoints all return one of the documented
// outcomes" (nil or a bounded validation error).
func TestFavorites_ConcurrentRemoveAndWriteRaceFree(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	const workers = 32
	var wg sync.WaitGroup
	start := make(chan struct{})
	errs := make([]error, workers)

	wg.Add(workers)
	for i := 0; i < workers; i++ {
		i := i
		go func() {
			defer wg.Done()
			<-start
			switch i % 3 {
			case 0:
				// Remove may legitimately succeed or fail depending on
				// scheduling — the test gate is "no panic, no race".
				errs[i] = app.RemoveFavoritePreset(i % 15)
			case 1:
				// Write a real preset (registered name from data.Presets).
				_, errs[i] = app.WriteSelectedToFavorites(idx, []string{"Geralt of Rivia, the Witcher"})
			case 2:
				_ = app.GetFavoritesStatus()
			}
		}()
	}
	close(start)
	wg.Wait()

	// We do not assert a final state — favorites endpoints can fail
	// (e.g. "not enough free slots") in a scheduling-dependent way,
	// and that is allowed. The race detector is the gating check.
	for i, e := range errs {
		if e == nil {
			continue
		}
		// Allow expected bounded errors; flag anything unrecognised.
		msg := e.Error()
		switch {
		case strings.Contains(msg, "not enough free slots"),
			strings.Contains(msg, "no save loaded"),
			strings.Contains(msg, "invalid favorites slot index"),
			strings.Contains(msg, "slot offset out of bounds"):
			// expected scheduling-dependent outcome
		default:
			t.Errorf("worker %d returned unexpected error: %v", i, e)
		}
	}
}

// TestDeploy_WriteTempSaveBlocksFavoritesWriter is the mandatory gate
// for the Phase 2 blocker fix: writeTempSave now takes favMu.RLock
// across the full SaveFile / MD5 pass, so a parallel favorites writer
// (which needs favMu.Lock) cannot mutate UserData10 mid-serialisation.
//
// Deterministic scheme:
//
//  1. Park slotMu[0] in the test — writeTempSave will block at
//     lockAllSlots() AFTER acquiring saveMu.RLock + favMu.RLock.
//  2. Probe favMu.TryLock() in a tight bounded loop. TryLock returns
//     false as soon as any RLock holder exists; success forces an
//     immediate Unlock so we never race future readers. When TryLock
//     fails we know writeTempSave is parked at lockAllSlots holding
//     favMu.RLock.
//  3. Fire RemoveFavoritePreset — must block on favMu.Lock.
//  4. Assert blocked for the standard observation window.
//  5. Release slotMu[0]. writeTempSave finishes → favMu.RUnlock →
//     RemoveFavoritePreset unblocks. Both must return cleanly.
//
// This proves both (a) writeTempSave really takes favMu (not just the
// static contract) and (b) the writer really waits on it. Without
// this gate the production race re-opens: writeTempSave's MD5 pass
// would see a half-zeroed favorites slot region.
func TestDeploy_WriteTempSaveBlocksFavoritesWriter(t *testing.T) {
	app, _ := realSaveAppForSave(t)

	app.slotMu[0].Lock()
	slotReleased := false
	releaseSlot := func() {
		if !slotReleased {
			slotReleased = true
			app.slotMu[0].Unlock()
		}
	}
	defer releaseSlot()

	type writeResult struct {
		path string
		err  error
	}
	tempDone := make(chan writeResult, 1)
	go func() {
		p, e := app.writeTempSave()
		tempDone <- writeResult{p, e}
	}()

	// Best-effort cleanup of whatever temp file writeTempSave creates.
	defer func() {
		select {
		case r := <-tempDone:
			if r.path != "" {
				os.Remove(r.path)
			}
		default:
			// writeTempSave still pending — captured by the wait below.
		}
	}()

	// Wait until writeTempSave has acquired favMu.RLock. TryLock on
	// an RWMutex returns false iff a writer or a reader holds the
	// lock; we are the only goroutine that could call TryLock, so a
	// failure here must come from writeTempSave's RLock.
	deadline := time.Now().Add(2 * time.Second)
	for {
		if !app.favMu.TryLock() {
			// favMu is held — by writeTempSave (only candidate).
			break
		}
		app.favMu.Unlock()
		if time.Now().After(deadline) {
			t.Fatal("writeTempSave did not take favMu.RLock within 2s — lock order is wrong or writeTempSave is not blocking on slotMu")
		}
		time.Sleep(5 * time.Millisecond)
	}

	favDone := make(chan error, 1)
	go func() {
		favDone <- app.RemoveFavoritePreset(0)
	}()

	select {
	case err := <-favDone:
		t.Fatalf("RemoveFavoritePreset completed while writeTempSave held favMu.RLock (err=%v) — favMu.RLock in writeTempSave is missing", err)
	case <-time.After(50 * time.Millisecond):
		// expected
	}

	releaseSlot()

	var tempRes writeResult
	select {
	case tempRes = <-tempDone:
		if tempRes.err != nil {
			t.Errorf("writeTempSave: %v", tempRes.err)
		}
		if tempRes.path != "" {
			os.Remove(tempRes.path)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("writeTempSave did not complete after slotMu[0] was released")
	}

	select {
	case e := <-favDone:
		if e != nil {
			t.Errorf("RemoveFavoritePreset: %v", e)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("RemoveFavoritePreset did not complete after writeTempSave released favMu")
	}
}
