package main

import (
	"testing"
	"time"
)

// TestSourceSave_DiffBlocksOnSourceSaveLock proves GetSlotDiff takes
// sourceSaveMu — without it a peer SelectAndOpenSourceSave / install
// could swap a.sourceSave under the diff loop. We park
// sourceSaveMu.Lock from the test and assert the endpoint cannot
// return until we release.
func TestSourceSave_DiffBlocksOnSourceSaveLock(t *testing.T) {
	app, _ := realSaveAppForSave(t)

	// Seed a source save — without one GetSlotDiff fails fast with
	// "no save loaded" before it reaches the lock contention path.
	source := loadFixtureSave(t)
	app.sourceSaveMu.Lock()
	app.installSourceSave(source)
	app.sourceSaveMu.Unlock()

	app.sourceSaveMu.Lock()
	released := false
	release := func() {
		if !released {
			released = true
			app.sourceSaveMu.Unlock()
		}
	}
	defer release()

	done := make(chan error, 1)
	go func() {
		_, e := app.GetSlotDiff(0)
		done <- e
	}()

	select {
	case err := <-done:
		t.Fatalf("GetSlotDiff completed while sourceSaveMu.Lock was held by the test (err=%v) — reader is not taking sourceSaveMu", err)
	case <-time.After(50 * time.Millisecond):
		// expected
	}

	release()

	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("GetSlotDiff: %v", err)
		}
	case <-time.After(5 * time.Second):
		t.Fatal("GetSlotDiff did not complete after sourceSaveMu release")
	}
}

// TestSourceSave_InstallBlocksOnActiveReader proves the source-save
// commit phase (installSourceSave under sourceSaveMu.Lock) drains
// in-flight readers before swapping a.sourceSave. We park a reader
// via sourceSaveMu.RLock and assert the writer cannot complete until
// we release.
func TestSourceSave_InstallBlocksOnActiveReader(t *testing.T) {
	app, _ := realSaveAppForSave(t)

	first := loadFixtureSave(t)
	app.sourceSaveMu.Lock()
	app.installSourceSave(first)
	app.sourceSaveMu.Unlock()

	candidate := loadFixtureSave(t)
	if candidate == first {
		t.Fatal("loadFixtureSave returned the same pointer twice; cannot prove a swap")
	}

	app.sourceSaveMu.RLock()
	released := false
	release := func() {
		if !released {
			released = true
			app.sourceSaveMu.RUnlock()
		}
	}
	defer release()

	done := make(chan struct{})
	go func() {
		defer close(done)
		app.sourceSaveMu.Lock()
		app.installSourceSave(candidate)
		app.sourceSaveMu.Unlock()
	}()

	select {
	case <-done:
		t.Fatal("installSourceSave completed while an active sourceSaveMu.RLock reader was held — drain contract broken")
	case <-time.After(50 * time.Millisecond):
		// expected
	}

	release()

	select {
	case <-done:
		// good
	case <-time.After(5 * time.Second):
		t.Fatal("installSourceSave did not complete after RUnlock — possible deadlock")
	}

	if app.sourceSave != candidate {
		t.Error("a.sourceSave was not swapped to the candidate after install")
	}
}
