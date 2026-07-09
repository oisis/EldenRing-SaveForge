package core

import (
	"fmt"
	"strings"
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

// clearSlot is bareSlot at a given NG+ cycle (ClearCount).
func clearSlot(cc uint32) *SaveSlot {
	return &SaveSlot{GaMap: map[uint32]uint32{}, Player: PlayerGameData{ClearCount: cc}}
}

// stoneswordKeyHandle resolves KnownDB to "Stonesword Key": conservative
// Normal Mode MaxInventory 55 / MaxStorage 0, regulation-derived game limits
// 99 / 600, flagged scales_with_ng for Normal Mode only.
const stoneswordKeyHandle = uint32(0xB0001F40)

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

func TestScanQuantity_AcceptsReportedVanillaQuantities(t *testing.T) {
	recs := []ResolvedRecord{
		resolveRec(repairScopeInventoryCommon, 0, 0xB0000401, 12, nil), // Crimson +12
		resolveRec(repairScopeInventoryCommon, 1, 0xB0000433, 2, nil),  // Cerulean +12
		resolveRec(repairScopeInventoryCommon, 2, 0xB000006F, 4, nil),  // Festering Bloody Finger
		resolveRec(repairScopeInventoryCommon, 3, 0xB0000FA0, 2, nil),  // Glintstone Pebble
	}
	issues := ScanRepairIssuesFromRecords(0, bareSlot(), recs)
	if got := countCode(issues, RepairCodeQuantityAboveMax); got != 0 {
		t.Fatalf("vanilla quantities produced %d quantity_above_max issues", got)
	}
}

func TestScanQuantity_AcceptsRegulationStorageCases(t *testing.T) {
	recs := []ResolvedRecord{
		resolveRec(repairScopeStorageCommon, 0, 0xB0000898, 1, nil), // Prattling Pate "Hello"
		resolveRec(repairScopeStorageCommon, 1, 0xB0000B87, 1, nil), // Remembrance of the Starscourge
	}
	issues := ScanRepairIssuesFromRecords(0, bareSlot(), recs)
	if got := countCode(issues, RepairCodeItemNotAllowedInContainer); got != 0 {
		t.Fatalf("regulation-permitted storage records produced %d not-allowed issues", got)
	}
}

func TestScanQuantity_UnknownGameLimitIsNotZero(t *testing.T) {
	rec := ResolvedRecord{
		Scope:      repairScopeInventoryCommon,
		Handle:     0xA0000001,
		Quantity:   50,
		Resolution: ResolutionKnownDB,
	}
	issues, coverage := ScanRepairIssuesWithCoverage(0, bareSlot(), []ResolvedRecord{rec})
	if got := countCode(issues, RepairCodeItemNotAllowedInContainer); got != 0 {
		t.Fatalf("unknown limit was treated as zero: %d issues", got)
	}
	if coverage.CategoryChecksApplied != 0 {
		t.Fatalf("CategoryChecksApplied = %d, want 0 for unknown game limit", coverage.CategoryChecksApplied)
	}
}

func TestScanQuantity_ZeroStorageLimit_NotPermitted(t *testing.T) {
	// Flask of Wondrous Physick: MaxStorage == 0 → not permitted in storage. This
	// is a distinct defect (item_not_allowed_in_container), NOT quantity_above_max:
	// clamping it would drive the quantity to zero.
	recs := []ResolvedRecord{resolveRec(repairScopeStorageCommon, 0, physickHandleQty, 1, nil)}
	if recs[0].Resolution != ResolutionKnownDB || recs[0].MaxStorage != 0 {
		t.Fatalf("fixture not KnownDB/MaxStorage 0: res=%q maxStor=%d", recs[0].Resolution, recs[0].MaxStorage)
	}
	issues := ScanRepairIssuesFromRecords(0, bareSlot(), recs)
	if got := countCode(issues, RepairCodeItemNotAllowedInContainer); got != 1 {
		t.Errorf("zero-limit container must flag item_not_allowed_in_container, got %d", got)
	}
	if got := countCode(issues, RepairCodeQuantityAboveMax); got != 0 {
		t.Errorf("zero-limit container must NOT flag quantity_above_max, got %d", got)
	}
	// Description must name the container, not "quantity above max 0".
	for _, i := range issues {
		if i.Key.Code == RepairCodeItemNotAllowedInContainer {
			if !strings.Contains(i.Description, "not permitted in storage_common") {
				t.Errorf("description = %q, want a 'not permitted in storage_common' message", i.Description)
			}
			// Core must remain removable but default to a non-mutating action,
			// so a direct core consumer never gets a destructive default.
			hasRemove, hasNoAction := false, false
			for _, a := range i.Actions {
				switch a {
				case RepairActionRemoveRecord:
					hasRemove = true
				case RepairActionNoAction:
					hasNoAction = true
				}
			}
			if !hasRemove {
				t.Error("core actions must include remove_record")
			}
			if !hasNoAction {
				t.Error("core actions must include no_action")
			}
			if i.DefaultAction != RepairActionNoAction {
				t.Errorf("core default action = %q, want no_action", i.DefaultAction)
			}
			if i.DefaultAction == RepairActionRemoveRecord {
				t.Error("core default action must not be destructive remove_record")
			}
		}
	}
}

func TestScanQuantity_PositiveCap_NeverEmitsNotAllowed(t *testing.T) {
	recs := []ResolvedRecord{resolveRec(repairScopeInventoryCommon, 0, smithingStoneHandle, 1000, nil)}
	issues := ScanRepairIssuesFromRecords(0, bareSlot(), recs)
	if got := countCode(issues, RepairCodeItemNotAllowedInContainer); got != 0 {
		t.Errorf("positive-cap over-quantity must NOT flag item_not_allowed_in_container, got %d", got)
	}
	if got := countCode(issues, RepairCodeQuantityAboveMax); got != 1 {
		t.Errorf("positive-cap over-quantity must flag quantity_above_max, got %d", got)
	}
	for _, i := range issues {
		if i.Key.Code == RepairCodeQuantityAboveMax && i.DefaultAction != RepairActionClampQuantity {
			t.Errorf("core default action = %q, want clamp_quantity", i.DefaultAction)
		}
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

// ---- Technical game limits vs conservative NG+ policy ----------------------

func TestScanQuantity_UsesGameLimitNotConservativeCap(t *testing.T) {
	// The conservative editor cap remains 55, but scanner integrity truth is the
	// regulation max 99.
	ok := []ResolvedRecord{resolveRec(repairScopeInventoryCommon, 0, stoneswordKeyHandle, 56, nil)}
	if !ok[0].ScalesWithNG {
		t.Fatalf("fixture must be scales_with_ng")
	}
	if got := countCode(ScanRepairIssuesFromRecords(0, clearSlot(0), ok), RepairCodeQuantityAboveMax); got != 0 {
		t.Errorf("qty 56 must not flag below game max 99, got %d", got)
	}
	over := []ResolvedRecord{resolveRec(repairScopeInventoryCommon, 0, stoneswordKeyHandle, 100, nil)}
	if got := countCode(ScanRepairIssuesFromRecords(0, clearSlot(0), over), RepairCodeQuantityAboveMax); got != 1 {
		t.Errorf("qty 100 must flag above game max 99, got %d", got)
	}
}

func TestScanQuantity_NGPlus3_DoesNotScaleGameCap(t *testing.T) {
	atCap := []ResolvedRecord{resolveRec(repairScopeInventoryCommon, 0, stoneswordKeyHandle, 99, nil)}
	if got := countCode(ScanRepairIssuesFromRecords(0, clearSlot(3), atCap), RepairCodeQuantityAboveMax); got != 0 {
		t.Errorf("qty 99 at NG+3 must not flag, got %d", got)
	}
}

func TestScanQuantity_NGPlus3_AboveGameCapFlagsOnce(t *testing.T) {
	over := []ResolvedRecord{resolveRec(repairScopeInventoryCommon, 0, stoneswordKeyHandle, 100, nil)}
	issues := ScanRepairIssuesFromRecords(0, clearSlot(3), over)
	if got := countCode(issues, RepairCodeQuantityAboveMax); got != 1 {
		t.Fatalf("qty 100 at NG+3 must flag exactly once (game max 99), got %d", got)
	}
	for _, i := range issues {
		if i.Key.Code == RepairCodeQuantityAboveMax && !strings.Contains(i.Description, "max 99") {
			t.Errorf("description must report game limit 99, got %q", i.Description)
		}
	}
}

func TestScanQuantity_Unflagged_DoesNotScaleWithClearCount(t *testing.T) {
	// Smithing Stone (no scales_with_ng): cap stays 999 regardless of NG+.
	recs := []ResolvedRecord{resolveRec(repairScopeInventoryCommon, 0, smithingStoneHandle, 1000, nil)}
	if recs[0].ScalesWithNG {
		t.Fatalf("Smithing Stone must NOT be scales_with_ng")
	}
	if got := countCode(ScanRepairIssuesFromRecords(0, clearSlot(3), recs), RepairCodeQuantityAboveMax); got != 1 {
		t.Errorf("unflagged qty 1000 must flag even at NG+3 (cap stays 999), got %d", got)
	}
}

func TestScanQuantity_StorageUsesGameLimitNotConservativeZero(t *testing.T) {
	recs := []ResolvedRecord{resolveRec(repairScopeStorageCommon, 0, stoneswordKeyHandle, 1, nil)}
	if recs[0].MaxStorage != 0 {
		t.Fatalf("fixture MaxStorage = %d, want 0", recs[0].MaxStorage)
	}
	issues := ScanRepairIssuesFromRecords(0, clearSlot(3), recs)
	if got := countCode(issues, RepairCodeItemNotAllowedInContainer); got != 0 {
		t.Errorf("game storage max 600 must override conservative zero, got %d issues", got)
	}
	if got := countCode(issues, RepairCodeQuantityAboveMax); got != 0 {
		t.Errorf("zero-limit storage must NOT flag quantity_above_max, got %d", got)
	}
}

// TestScanQuantity_RescanAfterClamp_ResolvesWithoutZeroDefect exercises the
// full loop: a scanner-reported quantity_above_max, an actual clamp, and a
// rescan that must show neither quantity_above_max nor a new quantity_zero.
func TestScanQuantity_RescanAfterClamp_ResolvesWithoutZeroDefect(t *testing.T) {
	slot := buildInvFixtureNZ(t, []InventoryItem{{GaItemHandle: smithingStoneHandle, Quantity: 1500, Index: 500}})

	before := ScanRepairIssues(0, slot)
	if countCode(before, RepairCodeQuantityAboveMax) != 1 {
		t.Fatalf("pre-clamp scan must report quantity_above_max, got %d", countCode(before, RepairCodeQuantityAboveMax))
	}

	fp, _ := FingerprintRecordAt(slot, repairScopeInventoryCommon, 0)
	if _, err := ClampInventoryQuantityAt(slot, repairScopeInventoryCommon, 0, fp); err != nil {
		t.Fatalf("clamp: %v", err)
	}

	after := ScanRepairIssues(0, slot)
	if got := countCode(after, RepairCodeQuantityAboveMax); got != 0 {
		t.Errorf("rescan still reports quantity_above_max: %d", got)
	}
	if got := countCode(after, RepairCodeQuantityZero); got != 0 {
		t.Errorf("clamp created a quantity_zero defect: %d", got)
	}
}

func TestScanQuantity_DBResolutionSetsScalesWithNG(t *testing.T) {
	if !resolveRec(repairScopeInventoryCommon, 0, stoneswordKeyHandle, 1, nil).ScalesWithNG {
		t.Error("Stonesword Key must resolve ScalesWithNG = true")
	}
	if resolveRec(repairScopeInventoryCommon, 0, smithingStoneHandle, 1, nil).ScalesWithNG {
		t.Error("Smithing Stone must resolve ScalesWithNG = false")
	}
}

func TestScanQuantity_NG_CategoryChecksAppliedCountsKnownDB(t *testing.T) {
	recs := []ResolvedRecord{
		resolveRec(repairScopeInventoryCommon, 0, stoneswordKeyHandle, 300, nil), // KnownDB, flagged, over even NG+3
		resolveRec(repairScopeInventoryCommon, 1, smithingStoneHandle, 10, nil),  // KnownDB
		resolveRec(repairScopeInventoryCommon, 2, 0x10000005, 5, nil),            // unknown
	}
	_, cov := ScanRepairIssuesWithCoverage(0, clearSlot(3), recs)
	if cov.CategoryChecksApplied != 2 {
		t.Errorf("CategoryChecksApplied = %d, want 2 (only KnownDB)", cov.CategoryChecksApplied)
	}
}
