package main

import (
	"sort"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// fixtureRawNonCurated is a real regulation.bin PlayRegionParam Row ID that is
// deliberately NOT part of the curated invasion allowlist (db.IsKnownRegionID).
// It stands in for the advanced / internal region IDs a real save carries.
// It is used ONLY to prove the World-tab bulk operations are non-destructive;
// the editor does not claim it is a legal standard-PvP region.
const fixtureRawNonCurated uint32 = 1001000

func TestFixtureIsRealButNonCurated(t *testing.T) {
	if db.IsKnownRegionID(fixtureRawNonCurated) {
		t.Fatalf("fixture %d must be outside the curated allowlist", fixtureRawNonCurated)
	}
}

func curatedSampleIDs(t *testing.T, n int) []uint32 {
	t.Helper()
	entries := db.GetAllRegions()
	if len(entries) < n {
		t.Fatalf("only %d curated regions, need %d", len(entries), n)
	}
	ids := make([]uint32, 0, n)
	for i := 0; i < n; i++ {
		ids = append(ids, entries[i].ID)
	}
	return ids
}

func contains(ids []uint32, id uint32) bool {
	for _, x := range ids {
		if x == id {
			return true
		}
	}
	return false
}

func hasDup(ids []uint32) bool {
	s := append([]uint32(nil), ids...)
	sort.Slice(s, func(i, j int) bool { return s[i] < s[j] })
	for i := 1; i < len(s); i++ {
		if s[i] == s[i-1] {
			return true
		}
	}
	return false
}

// TC-05 — Unlock All preserves non-curated raw IDs.
// Input: full curated allowlist as curatedIDs, existing raw = some curated +
// the non-curated fixture. After merge: all curated present, fixture still
// present, no duplicates.
func TestUnlockAllPreservesNonCuratedRaw(t *testing.T) {
	curatedAll := make([]uint32, 0, len(db.GetAllRegions()))
	for _, e := range db.GetAllRegions() {
		curatedAll = append(curatedAll, e.ID)
	}
	existingRaw := append(curatedSampleIDs(t, 3), fixtureRawNonCurated)

	got := mergeUnlockedRegions(curatedAll, existingRaw)

	for _, id := range curatedAll {
		if !contains(got, id) {
			t.Fatalf("Unlock All dropped curated region %d", id)
		}
	}
	if !contains(got, fixtureRawNonCurated) {
		t.Fatalf("Unlock All dropped non-curated raw fixture %d", fixtureRawNonCurated)
	}
	if hasDup(got) {
		t.Errorf("Unlock All produced duplicates (core.SetUnlockedRegions also dedupes, but the merge must not rely on it for correctness)")
	}
}

// TC-06 — Lock All preserves non-curated raw IDs.
// Input: curatedIDs = {} (lock everything curated), existing raw = curated
// sample + fixture. After merge: only the fixture survives.
func TestLockAllPreservesNonCuratedRaw(t *testing.T) {
	existingRaw := append(curatedSampleIDs(t, 5), fixtureRawNonCurated)

	got := mergeUnlockedRegions([]uint32{}, existingRaw)

	if !contains(got, fixtureRawNonCurated) {
		t.Fatalf("Lock All dropped non-curated raw fixture %d", fixtureRawNonCurated)
	}
	for _, id := range existingRaw {
		if db.IsKnownRegionID(id) && contains(got, id) {
			t.Errorf("Lock All left curated region %d unlocked", id)
		}
	}
	if len(got) != 1 {
		t.Errorf("Lock All result = %v, want only the non-curated fixture", got)
	}
}

// TC-07 — per-area toggle preserves non-curated raw IDs.
// Simulates the frontend handleUnlockAreaRegions / handleLockAreaRegions
// payload (a curated subset) and asserts the non-curated fixture is retained.
func TestPerAreaTogglePreservesNonCuratedRaw(t *testing.T) {
	areaCurated := curatedSampleIDs(t, 4) // pretend "one area" worth of curated IDs
	existingRaw := []uint32{areaCurated[0], fixtureRawNonCurated}

	// Unlock-area: curatedIDs = currently-unlocked-curated ∪ area.
	gotUnlock := mergeUnlockedRegions(areaCurated, existingRaw)
	if !contains(gotUnlock, fixtureRawNonCurated) {
		t.Fatalf("per-area Unlock dropped non-curated raw fixture %d", fixtureRawNonCurated)
	}
	for _, id := range areaCurated {
		if !contains(gotUnlock, id) {
			t.Errorf("per-area Unlock missing curated %d", id)
		}
	}

	// Lock-area: curatedIDs = unlocked-curated minus area (here: empty).
	gotLock := mergeUnlockedRegions([]uint32{}, existingRaw)
	if !contains(gotLock, fixtureRawNonCurated) {
		t.Fatalf("per-area Lock dropped non-curated raw fixture %d", fixtureRawNonCurated)
	}
}
