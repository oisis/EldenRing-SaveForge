package core

import "testing"

// TestResolveRecord_AliasKeepsRawID proves the resolver follows a technical
// alias for metadata only: a goods handle whose alias-derived raw itemID
// (0x400000C2) is not itself a DB entry resolves to the canonical Great Rune,
// while ItemID/DisplayID stay the raw alias ID - the save record is not
// normalized to the canonical ID.
func TestResolveRecord_AliasKeepsRawID(t *testing.T) {
	const (
		handle      = uint32(0xB00000C2) // goods handle for alias itemID 0x400000C2
		rawItemID   = uint32(0x400000C2)
		canonicalID = uint32(0x40001FD7)
		wantName    = "Rykard's Great Rune"
	)
	slot := &SaveSlot{GaMap: map[uint32]uint32{}}
	rec := ResolveRecord(slot, repairScopeInventoryCommon, 0, handle, 1, 500)

	if rec.Resolution != ResolutionKnownDB {
		t.Fatalf("resolution = %q, want known_db (reason %q)", rec.Resolution, rec.UnknownReason)
	}
	if rec.Name != wantName {
		t.Errorf("Name = %q, want %q", rec.Name, wantName)
	}
	if rec.BaseID != canonicalID {
		t.Errorf("BaseID = 0x%08X, want canonical 0x%08X", rec.BaseID, canonicalID)
	}
	if rec.ItemID != rawItemID {
		t.Errorf("ItemID = 0x%08X, want raw alias ID 0x%08X (must not normalize)", rec.ItemID, rawItemID)
	}
	if rec.DisplayID != rawItemID {
		t.Errorf("DisplayID = 0x%08X, want raw alias ID 0x%08X (must not normalize)", rec.DisplayID, rawItemID)
	}
}
