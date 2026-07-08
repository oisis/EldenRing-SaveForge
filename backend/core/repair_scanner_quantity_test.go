package core

import (
	"fmt"
	"testing"
)

// Prompt 13 — category-aware per-record quantity validation. These tests drive
// the scanner through pre-resolved records so resolution and validation share
// one ResolvedRecord collection (no second identity pass in the scanner).

// smithingStoneHandle resolves KnownDB to "Smithing Stone [1]" with
// MaxInventory == MaxStorage == 999 (no GaMap entry needed — handle-encoded).
const smithingStoneHandle = uint32(0xB0002774)

// physickHandleQty resolves KnownDB to the Flask of Wondrous Physick with
// MaxInventory == 1 and MaxStorage == 0 (not permitted in storage).
const physickHandleQty = uint32(0xB00000FA)

// resolveRec is a tiny helper: resolve one record under a scope with an optional
// GaMap so tests can build realistic ResolvedRecord collections.
func resolveRec(scope string, row int, handle, qty uint32, gaMap map[uint32]uint32) ResolvedRecord {
	if gaMap == nil {
		gaMap = map[uint32]uint32{}
	}
	slot := &SaveSlot{GaMap: gaMap}
	return ResolveRecord(slot, scope, row, handle, qty, 500)
}

func countCode(issues []RepairIssue, code string) int {
	n := 0
	for _, i := range issues {
		if i.Key.Code == code {
			n++
		}
	}
	return n
}

// bareSlot is a minimal slot: empty GaItems (no AoW issues) and Level 0 (no
// stats issue), so only the inventory scanner produces output.
func bareSlot() *SaveSlot { return &SaveSlot{GaMap: map[uint32]uint32{}} }

func TestScanQuantity_InventoryBoundary_NoIssue(t *testing.T) {
	recs := []ResolvedRecord{resolveRec(repairScopeInventoryCommon, 0, smithingStoneHandle, 999, nil)}
	if recs[0].Resolution != ResolutionKnownDB || recs[0].MaxInventory != 999 {
		t.Fatalf("fixture not KnownDB/999: res=%q maxInv=%d", recs[0].Resolution, recs[0].MaxInventory)
	}
	issues := ScanRepairIssuesFromRecords(0, bareSlot(), recs)
	if got := countCode(issues, RepairCodeQuantityAboveMax); got != 0 {
		t.Errorf("qty at exact MaxInventory must not flag, got %d", got)
	}
}

func TestScanQuantity_InventoryAboveMax_Flags(t *testing.T) {
	recs := []ResolvedRecord{resolveRec(repairScopeInventoryCommon, 0, smithingStoneHandle, 1000, nil)}
	issues := ScanRepairIssuesFromRecords(0, bareSlot(), recs)
	if got := countCode(issues, RepairCodeQuantityAboveMax); got != 1 {
		t.Fatalf("qty above MaxInventory must flag exactly once, got %d", got)
	}
}

func TestScanQuantity_StorageBoundary_NoIssue(t *testing.T) {
	recs := []ResolvedRecord{resolveRec(repairScopeStorageCommon, 0, smithingStoneHandle, 999, nil)}
	if recs[0].MaxStorage != 999 {
		t.Fatalf("fixture MaxStorage = %d, want 999", recs[0].MaxStorage)
	}
	issues := ScanRepairIssuesFromRecords(0, bareSlot(), recs)
	if got := countCode(issues, RepairCodeQuantityAboveMax); got != 0 {
		t.Errorf("qty at exact MaxStorage must not flag, got %d", got)
	}
}

func TestScanQuantity_StorageAboveMax_Flags(t *testing.T) {
	recs := []ResolvedRecord{resolveRec(repairScopeStorageCommon, 0, smithingStoneHandle, 1000, nil)}
	issues := ScanRepairIssuesFromRecords(0, bareSlot(), recs)
	if got := countCode(issues, RepairCodeQuantityAboveMax); got != 1 {
		t.Errorf("qty above MaxStorage must flag once, got %d", got)
	}
}

func TestScanQuantity_ZeroStorageLimit_NotPermitted(t *testing.T) {
	// Flask of Wondrous Physick: MaxStorage == 0 → not permitted in storage, so
	// even quantity 1 exceeds the container limit.
	recs := []ResolvedRecord{resolveRec(repairScopeStorageCommon, 0, physickHandleQty, 1, nil)}
	if recs[0].Resolution != ResolutionKnownDB || recs[0].MaxStorage != 0 {
		t.Fatalf("fixture not KnownDB/MaxStorage 0: res=%q maxStor=%d", recs[0].Resolution, recs[0].MaxStorage)
	}
	issues := ScanRepairIssuesFromRecords(0, bareSlot(), recs)
	if got := countCode(issues, RepairCodeQuantityAboveMax); got != 1 {
		t.Errorf("record in a zero-limit container must flag, got %d", got)
	}
}

func TestScanQuantity_HighBitFlagMasked(t *testing.T) {
	const flag = uint32(0x80000000)
	// Effective 500 (≤ 999) with the high bit set must NOT flag.
	below := []ResolvedRecord{resolveRec(repairScopeInventoryCommon, 0, smithingStoneHandle, flag|500, nil)}
	if got := countCode(ScanRepairIssuesFromRecords(0, bareSlot(), below), RepairCodeQuantityAboveMax); got != 0 {
		t.Errorf("masked effective 500 must not flag, got %d", got)
	}
	// Effective 1000 (> 999) with the high bit set must flag, and report the
	// masked value 1000 (not the raw quantity with the flag).
	above := []ResolvedRecord{resolveRec(repairScopeInventoryCommon, 0, smithingStoneHandle, flag|1000, nil)}
	issues := ScanRepairIssuesFromRecords(0, bareSlot(), above)
	if got := countCode(issues, RepairCodeQuantityAboveMax); got != 1 {
		t.Fatalf("masked effective 1000 must flag, got %d", got)
	}
	for _, i := range issues {
		if i.Key.Code == RepairCodeQuantityAboveMax && i.Key.Value != "1000" {
			t.Errorf("issue Value = %q, want masked effective %q", i.Key.Value, "1000")
		}
	}
}

func TestScanQuantity_UnknownRecordExcluded(t *testing.T) {
	// Illegal handle prefix → unknown; never category-checked even if the raw
	// quantity is enormous.
	recs := []ResolvedRecord{resolveRec(repairScopeInventoryCommon, 0, 0x10000005, 9999, nil)}
	if recs[0].Resolution != ResolutionUnknown {
		t.Fatalf("fixture resolution = %q, want unknown", recs[0].Resolution)
	}
	_, cov := ScanRepairIssuesWithCoverage(0, bareSlot(), recs)
	if got := countCode(ScanRepairIssuesFromRecords(0, bareSlot(), recs), RepairCodeQuantityAboveMax); got != 0 {
		t.Errorf("unknown record must not be category-checked, got %d", got)
	}
	if cov.CategoryChecksApplied != 0 {
		t.Errorf("CategoryChecksApplied = %d, want 0 (no KnownDB records)", cov.CategoryChecksApplied)
	}
}

func TestScanQuantity_TechnicalPlaceholderExcluded(t *testing.T) {
	// Naked-armor placeholder → technical placeholder; no authoritative cap, so
	// never category-checked.
	recs := []ResolvedRecord{resolveRec(repairScopeStorageCommon, 0, nakedHeadHandle, 5,
		map[uint32]uint32{nakedHeadHandle: nakedHeadItemID})}
	if recs[0].Resolution != ResolutionTechnicalPlaceholder {
		t.Fatalf("fixture resolution = %q, want technical_placeholder", recs[0].Resolution)
	}
	_, cov := ScanRepairIssuesWithCoverage(0, bareSlot(), recs)
	if got := countCode(ScanRepairIssuesFromRecords(0, bareSlot(), recs), RepairCodeQuantityAboveMax); got != 0 {
		t.Errorf("technical placeholder must not be category-checked, got %d", got)
	}
	if cov.CategoryChecksApplied != 0 {
		t.Errorf("CategoryChecksApplied = %d, want 0 (placeholder excluded)", cov.CategoryChecksApplied)
	}
}

func TestScanQuantity_DeterministicIdentityAndFingerprint(t *testing.T) {
	recs := []ResolvedRecord{resolveRec(repairScopeInventoryCommon, 3, smithingStoneHandle, 1000, nil)}
	a := ScanRepairIssuesFromRecords(0, bareSlot(), recs)
	b := ScanRepairIssuesFromRecords(0, bareSlot(), recs)

	var ia *RepairIssue
	for i := range a {
		if a[i].Key.Code == RepairCodeQuantityAboveMax {
			ia = &a[i]
		}
	}
	if ia == nil {
		t.Fatal("expected a quantity_above_max issue")
	}
	// Stable across scans.
	if a[len(a)-1].IssueID != b[len(b)-1].IssueID {
		t.Errorf("IssueID not deterministic across scans")
	}
	// IssueID matches the canonical key hash.
	wantID := IssueKeyID(IssueKey{Slot: 0, Domain: repairDomainInventory, Code: RepairCodeQuantityAboveMax,
		Scope: repairScopeInventoryCommon, Row: 3, Handle: smithingStoneHandle,
		Field: "quantity", Value: fmt.Sprintf("%d", 1000)})
	if ia.IssueID != wantID {
		t.Errorf("IssueID = %s, want canonical %s", ia.IssueID, wantID)
	}
	// Fingerprint matches the record's own fingerprint.
	if ia.Fingerprint != recs[0].Fingerprint {
		t.Errorf("Fingerprint = %s, want record fingerprint %s", ia.Fingerprint, recs[0].Fingerprint)
	}
}

func TestScanQuantity_CategoryChecksApplied_CountsOnlyExecuted(t *testing.T) {
	recs := []ResolvedRecord{
		resolveRec(repairScopeInventoryCommon, 0, smithingStoneHandle, 10, nil),                                          // KnownDB
		resolveRec(repairScopeInventoryCommon, 1, physickHandleQty, 1, nil),                                              // KnownDB
		resolveRec(repairScopeInventoryCommon, 2, 0x10000005, 5, nil),                                                    // unknown
		resolveRec(repairScopeStorageCommon, 0, nakedHeadHandle, 1, map[uint32]uint32{nakedHeadHandle: nakedHeadItemID}), // placeholder
	}
	_, cov := ScanRepairIssuesWithCoverage(0, bareSlot(), recs)
	if cov.CategoryChecksApplied != 2 {
		t.Errorf("CategoryChecksApplied = %d, want 2 (only the two KnownDB records)", cov.CategoryChecksApplied)
	}
	// Prompt-12 invariants must survive alongside the new category coverage.
	if cov.TotalPhysical != cov.KnownDB+cov.TechnicalPlaceholder+cov.Unknown {
		t.Errorf("partition broken: total=%d != known+tech+unknown", cov.TotalPhysical)
	}
	if cov.ResolutionChecksApplied != cov.TotalPhysical {
		t.Errorf("ResolutionChecksApplied = %d, want TotalPhysical %d", cov.ResolutionChecksApplied, cov.TotalPhysical)
	}
	if cov.StructuralChecksApplied != cov.TotalPhysical {
		t.Errorf("StructuralChecksApplied = %d, want TotalPhysical %d", cov.StructuralChecksApplied, cov.TotalPhysical)
	}
	if cov.CategoryChecksApplied > cov.KnownDB {
		t.Errorf("CategoryChecksApplied %d must not exceed KnownDB %d", cov.CategoryChecksApplied, cov.KnownDB)
	}
}
