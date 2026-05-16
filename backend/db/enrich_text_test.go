package db

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestEnrichItemEntryUsesItemTextsDescription verifies that enrichItemEntry
// prefers the Phase 3B.1 ItemTexts entry for Description.
//
// Anchor: Black Syrup (0x401EA3D3) — a SOTE DLC key item shipped in
// Phase 2B.4. Its FMG description in ItemTexts ("Something Moore asked
// to be delivered to Thiollier") differs from any value descriptions.go
// might supply, so the assertion proves ItemTexts wins.
func TestEnrichItemEntryUsesItemTextsDescription(t *testing.T) {
	const id uint32 = 0x401EA3D3
	want, ok := data.ItemTexts[id]
	if !ok {
		t.Fatalf("ItemTexts[0x%08X] missing; cannot anchor test on Black Syrup", id)
	}
	if want.Description == "" {
		t.Fatalf("ItemTexts[0x%08X].Description is empty; pick a different anchor", id)
	}

	e := &ItemEntry{ID: id}
	enrichItemEntry(e)
	if e.Description != want.Description {
		t.Errorf("enrichItemEntry: Description = %q\nwant ItemTexts value %q",
			e.Description, want.Description)
	}
}

// TestEnrichItemEntryUsesItemTextsLocation verifies that Location after
// enrichment matches the curated descriptions.go Location surfaced via
// the new ItemTexts entry. Lance (0x010450A0) is the canonical anchor
// from Phase 3A — it has a long Fextralife-sourced Location that has
// no FMG equivalent.
func TestEnrichItemEntryUsesItemTextsLocation(t *testing.T) {
	const id uint32 = 0x010450A0
	want, ok := data.ItemTexts[id]
	if !ok {
		t.Fatalf("ItemTexts[0x%08X] missing; cannot anchor test on Lance", id)
	}
	if want.Location == "" {
		t.Fatalf("ItemTexts[0x%08X].Location is empty; pick a different anchor", id)
	}
	if want.LocationSource != data.TextSourceCurated {
		t.Fatalf("ItemTexts[0x%08X].LocationSource = %q, want curated",
			id, want.LocationSource)
	}

	e := &ItemEntry{ID: id}
	enrichItemEntry(e)
	if e.Location != want.Location {
		t.Errorf("enrichItemEntry: Location = %q\nwant ItemTexts value %q",
			e.Location, want.Location)
	}
}

// TestEnrichItemEntryFallsBackToDescriptions verifies the legacy fallback
// path still works for IDs that exist in data.Descriptions but not in
// data.ItemTexts. These are the ~3 110 orphan rows surfaced in the Phase
// 3A audit — items in the curated table without a current app-map entry.
//
// The test discovers a suitable orphan dynamically (one with a non-empty
// Description or Location) so it tolerates churn in either dataset.
func TestEnrichItemEntryFallsBackToDescriptions(t *testing.T) {
	var (
		chosenID    uint32
		chosenDesc  data.ItemDescription
		foundOrphan bool
	)
	for id, desc := range data.Descriptions {
		if _, inTexts := data.ItemTexts[id]; inTexts {
			continue
		}
		if desc.Description == "" && desc.Location == "" {
			continue
		}
		chosenID = id
		chosenDesc = desc
		foundOrphan = true
		break
	}
	if !foundOrphan {
		t.Skip("no Descriptions orphan with text — skipping fallback test (Phase 3D D7 may have already pruned orphans)")
	}

	e := &ItemEntry{ID: chosenID}
	enrichItemEntry(e)
	if chosenDesc.Description != "" && e.Description != chosenDesc.Description {
		t.Errorf("enrichItemEntry(0x%08X): Description = %q, want fallback to Descriptions value %q",
			chosenID, e.Description, chosenDesc.Description)
	}
	if chosenDesc.Location != "" && e.Location != chosenDesc.Location {
		t.Errorf("enrichItemEntry(0x%08X): Location = %q, want fallback to Descriptions value %q",
			chosenID, e.Location, chosenDesc.Location)
	}
}

// TestEnrichItemEntryPreservesLegacyStats verifies that legacy
// Weapon/Armor/Spell pointers and Weight remain sourced from
// data.Descriptions even after ItemTexts wiring. Lance carries
// WeaponStats in descriptions.go; the assertion confirms those values
// survived the Phase 3B.2 change.
func TestEnrichItemEntryPreservesLegacyStats(t *testing.T) {
	const id uint32 = 0x010450A0 // Lance
	want, ok := data.Descriptions[id]
	if !ok {
		t.Fatalf("Descriptions[0x%08X] missing; cannot anchor test on Lance", id)
	}
	if want.Weapon == nil {
		t.Fatalf("Descriptions[0x%08X].Weapon is nil; pick a different anchor", id)
	}

	e := &ItemEntry{ID: id, Category: "melee_armaments"}
	enrichItemEntry(e)

	if e.Weapon == nil {
		t.Fatalf("enrichItemEntry: legacy Weapon was dropped after Phase 3B.2 wiring")
	}
	if *e.Weapon != *want.Weapon {
		t.Errorf("enrichItemEntry: Weapon = %+v\nwant %+v (must equal Descriptions value)",
			*e.Weapon, *want.Weapon)
	}
	if want.Weight > 0 && e.Weight != want.Weight {
		t.Errorf("enrichItemEntry: Weight = %v, want %v", e.Weight, want.Weight)
	}
}

// TestEnrichItemEntryNoPanicMissingText asserts that ItemEntry IDs absent
// from both data.ItemTexts and data.Descriptions enrich without panic
// and leave Description / Location empty.
func TestEnrichItemEntryNoPanicMissingText(t *testing.T) {
	const unknown uint32 = 0xDEADBEEF
	if _, ok := data.ItemTexts[unknown]; ok {
		t.Fatalf("0x%08X collides with a known item; pick a different sentinel", unknown)
	}
	if _, ok := data.Descriptions[unknown]; ok {
		t.Fatalf("0x%08X collides with a known item; pick a different sentinel", unknown)
	}

	e := &ItemEntry{ID: unknown}
	enrichItemEntry(e) // must not panic
	if e.Description != "" {
		t.Errorf("enrichItemEntry(unknown): Description = %q, want empty", e.Description)
	}
	if e.Location != "" {
		t.Errorf("enrichItemEntry(unknown): Location = %q, want empty", e.Location)
	}
}

// TestEnrichItemEntryEmptyItemTextsDoesNotOverwriteDescriptions verifies
// that an ItemTexts entry with empty Description / Location does NOT
// blank out a populated legacy value. We construct the scenario by
// scanning for an ID where ItemTexts.Description == "" but
// Descriptions.Description != "" (rare in practice; Phase 3B.1 generator
// usually copies curated into ItemTexts, but defensive coverage matters
// in case future regenerations diverge).
func TestEnrichItemEntryEmptyItemTextsDoesNotOverwriteDescriptions(t *testing.T) {
	var (
		chosenID   uint32
		chosenDesc data.ItemDescription
		found      bool
	)
	for id, t1 := range data.ItemTexts {
		if t1.Description != "" {
			continue
		}
		desc, ok := data.Descriptions[id]
		if !ok || desc.Description == "" {
			continue
		}
		chosenID = id
		chosenDesc = desc
		found = true
		break
	}
	if !found {
		t.Skip("no ID with empty ItemTexts.Description but populated Descriptions.Description; case may be unreachable in current data")
	}

	e := &ItemEntry{ID: chosenID}
	enrichItemEntry(e)
	if e.Description != chosenDesc.Description {
		t.Errorf("enrichItemEntry(0x%08X): Description = %q, want preserved legacy %q",
			chosenID, e.Description, chosenDesc.Description)
	}
}
