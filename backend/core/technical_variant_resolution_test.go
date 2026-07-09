package core

import "testing"

// Goods handles (prefix 0xB0) for the five technical variant item IDs seen in
// tmp/save/ER0000.sl2 slot 0 "[PL] Jagna". Before the DB entries were added,
// each resolved to unknown_item_id; they are legal game items and must now
// resolve cleanly.
var technicalVariantHandles = []struct {
	handle uint32
	name   string
}{
	{0xB0002AFA, "Crimson Crystal Tear (Variant)"},
	{0xB0002AFC, "Cerulean Crystal Tear (Variant)"},
	{0xB0002B08, "Ruptured Crystal Tear (Variant)"},
	{0xB0001FAD, "Academy Glintstone Key (Variant)"},
	{0xB0001FD2, "Miniature Ranni (Variant)"},
}

// TestResolveRecord_TechnicalVariantsAreKnownDB confirms the scanner-facing
// resolver treats each variant as a resolved DB item, not an unknown_item_id.
func TestResolveRecord_TechnicalVariantsAreKnownDB(t *testing.T) {
	slot := &SaveSlot{GaMap: map[uint32]uint32{}}
	for _, c := range technicalVariantHandles {
		rec := ResolveRecord(slot, repairScopeInventoryCommon, 0, c.handle, 1, 500)
		if rec.Resolution != ResolutionKnownDB {
			t.Errorf("handle 0x%08X: resolution = %q, want known_db (unknown reason %q)", c.handle, rec.Resolution, rec.UnknownReason)
		}
		if rec.Name != c.name {
			t.Errorf("handle 0x%08X: name = %q, want %q", c.handle, rec.Name, c.name)
		}
	}
}

// TestResolveRecord_UnknownGoodsStillFlagged guards that a goods handle whose
// itemID is genuinely absent from the DB still reports missing_db_entry — the
// additive variant entries must not mask real unknowns.
func TestResolveRecord_UnknownGoodsStillFlagged(t *testing.T) {
	slot := &SaveSlot{GaMap: map[uint32]uint32{}}
	rec := ResolveRecord(slot, repairScopeInventoryCommon, 0, 0xB0009999, 1, 500)
	if rec.Resolution != ResolutionUnknown || rec.UnknownReason != UnknownReasonMissingDBEntry {
		t.Errorf("unknown goods handle: resolution = %q reason = %q, want unknown/missing_db_entry", rec.Resolution, rec.UnknownReason)
	}
}
