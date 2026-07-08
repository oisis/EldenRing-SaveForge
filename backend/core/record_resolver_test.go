package core

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// Naked-armor placeholder handles → GaMap itemIDs (EquipParamProtector rows
// 10000/10100/10200/10300). These are the exact records observed in
// tmp/save/ER0000-kro55.sl2 slot 0.
const (
	nakedHeadHandle = uint32(0x908000FB)
	nakedBodyHandle = uint32(0x908002AE)
	nakedArmsHandle = uint32(0x908002AF)
	nakedLegsHandle = uint32(0x908002B0)
)

// Real Dagger base itemID (data.Weapons) and its handle form. Used where a
// genuinely DB-resolvable instance-backed weapon is needed.
const (
	realDaggerItemID = uint32(0x000F4240)
	realDaggerHandle = uint32(0x800F4240)
)

// TestResolveRecord_NakedArmorIsTechnicalPlaceholder confirms the four naked
// armor rows resolve as technical placeholders, not unknown items.
func TestResolveRecord_NakedArmorIsTechnicalPlaceholder(t *testing.T) {
	cases := []struct {
		handle uint32
		itemID uint32
		name   string
	}{
		{nakedHeadHandle, nakedHeadItemID, "Bare Head"},
		{nakedBodyHandle, nakedBodyItemID, "Bare Body"},
		{nakedArmsHandle, nakedArmsItemID, "Bare Arms"},
		{nakedLegsHandle, nakedLegsItemID, "Bare Legs"},
	}
	for _, c := range cases {
		slot := &SaveSlot{GaMap: map[uint32]uint32{c.handle: c.itemID}}
		rec := ResolveRecord(slot, repairScopeInventoryCommon, 0, c.handle, 1, 500)
		if rec.Resolution != ResolutionTechnicalPlaceholder {
			t.Errorf("handle 0x%08X: resolution = %q, want technical_placeholder", c.handle, rec.Resolution)
		}
		if rec.Identity != IdentityTechnicalPlaceholder {
			t.Errorf("handle 0x%08X: identity = %q, want technical_placeholder", c.handle, rec.Identity)
		}
		if rec.Name != c.name {
			t.Errorf("handle 0x%08X: name = %q, want %q", c.handle, rec.Name, c.name)
		}
		if rec.UnknownReason != "" {
			t.Errorf("handle 0x%08X: technical placeholder must not carry an unknown reason, got %q", c.handle, rec.UnknownReason)
		}
	}
}

// TestResolveRecord_UnarmedIsTechnicalPlaceholder confirms the Unarmed slot is a
// resolved technical placeholder, not an unknown item.
func TestResolveRecord_UnarmedIsTechnicalPlaceholder(t *testing.T) {
	// Unarmed weapon handle: HandleToItemID(0x8001ADB0) = 0x0001ADB0 (base).
	const unarmedHandle = uint32(0x8001ADB0)
	slot := &SaveSlot{GaMap: map[uint32]uint32{}}
	rec := ResolveRecord(slot, repairScopeInventoryCommon, 0, unarmedHandle, 1, 500)
	if rec.Resolution != ResolutionTechnicalPlaceholder {
		t.Fatalf("Unarmed resolution = %q, want technical_placeholder (name=%q)", rec.Resolution, rec.Name)
	}
	if rec.Identity != IdentityTechnicalPlaceholder {
		t.Errorf("Unarmed identity = %q, want technical_placeholder", rec.Identity)
	}
}

// TestResolveRecord_WondrousPhysickKnownDB confirms the filled save-state Flask
// of Wondrous Physick resolves as a known DB item via display-ID normalization,
// without rewriting the raw item ID.
func TestResolveRecord_WondrousPhysickKnownDB(t *testing.T) {
	const physickHandle = uint32(0xB00000FA) // → raw itemID 0x400000FA
	slot := &SaveSlot{GaMap: map[uint32]uint32{}}
	rec := ResolveRecord(slot, repairScopeInventoryCommon, 0, physickHandle, 1, 500)

	if rec.Resolution != ResolutionKnownDB {
		t.Fatalf("physick resolution = %q, want known_db", rec.Resolution)
	}
	if rec.ItemID != db.ItemFlaskWondrousPhysickFilled {
		t.Errorf("raw itemID rewritten: got 0x%08X, want 0x%08X (filled)", rec.ItemID, db.ItemFlaskWondrousPhysickFilled)
	}
	if rec.DisplayID != db.ItemFlaskWondrousPhysickEmpty {
		t.Errorf("displayID = 0x%08X, want 0x%08X (normalized)", rec.DisplayID, db.ItemFlaskWondrousPhysickEmpty)
	}
	if rec.Name == "" {
		t.Error("physick must resolve to a DB name")
	}
	if rec.Identity != IdentityHandleEncoded {
		t.Errorf("physick identity = %q, want handle_encoded", rec.Identity)
	}
}

// TestResolveRecord_IdentityClasses confirms goods, talismans and arrows are
// classified into the correct identity class.
func TestResolveRecord_IdentityClasses(t *testing.T) {
	// Goods (handle-encoded): Smithing Stone via 0xB0 handle.
	goods := ResolveRecord(&SaveSlot{GaMap: map[uint32]uint32{}}, repairScopeInventoryCommon, 0, testHandleSmithingStone, 5, 500)
	if goods.Identity != IdentityHandleEncoded {
		t.Errorf("goods identity = %q, want handle_encoded", goods.Identity)
	}

	// Talisman (handle-encoded): Sacrificial Twig 0xA00017B6.
	tal := ResolveRecord(&SaveSlot{GaMap: map[uint32]uint32{}}, repairScopeInventoryCommon, 0, 0xA00017B6, 1, 500)
	if tal.Identity != IdentityHandleEncoded {
		t.Errorf("talisman identity = %q, want handle_encoded (name=%q)", tal.Identity, tal.Name)
	}

	// Arrow (stackable ammo): weapon-prefix handle backed by an arrow GaItem.
	arrowSlot := &SaveSlot{GaMap: map[uint32]uint32{testHandleArrow: testItemIDArrow}}
	arrow := ResolveRecord(arrowSlot, repairScopeInventoryCommon, 0, testHandleArrow, 600, 500)
	if arrow.Identity != IdentityStackableAmmo {
		t.Errorf("arrow identity = %q, want stackable_ammo", arrow.Identity)
	}

	// Weapon (instance-backed): dagger with a GaItem.
	wSlot := &SaveSlot{GaMap: map[uint32]uint32{testHandleDagger: testItemIDDagger}}
	weap := ResolveRecord(wSlot, repairScopeInventoryCommon, 0, testHandleDagger, 1, 500)
	if weap.Identity != IdentityInstanceBacked {
		t.Errorf("weapon identity = %q, want instance_backed", weap.Identity)
	}
}

// TestResolveRecord_UnknownHandlePrefix confirms a handle whose type prefix is
// not a known GaItem type resolves as unknown with the unknown_handle_type
// reason — distinct from a missing DB entry.
func TestResolveRecord_UnknownHandlePrefix(t *testing.T) {
	const badHandle = uint32(0x10000005) // prefix 0x10 is not a GaItem type
	rec := ResolveRecord(&SaveSlot{GaMap: map[uint32]uint32{}}, repairScopeInventoryCommon, 0, badHandle, 1, 500)
	if rec.Resolution != ResolutionUnknown {
		t.Fatalf("resolution = %q, want unknown", rec.Resolution)
	}
	if rec.UnknownReason != UnknownReasonUnknownHandleType {
		t.Errorf("unknown reason = %q, want unknown_handle_type", rec.UnknownReason)
	}
	if rec.Identity != IdentityUnknown {
		t.Errorf("identity = %q, want unknown", rec.Identity)
	}
}

// TestResolveRecord_MissingDBEntryVsUnknownPrefix confirms a known prefix with
// an unresolvable itemID is missing_db_entry (not unknown_handle_type), while
// its identity class is still derived from the prefix.
func TestResolveRecord_MissingDBEntryVsUnknownPrefix(t *testing.T) {
	// Goods handle with an itemID that is not in the DB.
	const orphanGoods = uint32(0xB0FEEDFF)
	rec := ResolveRecord(&SaveSlot{GaMap: map[uint32]uint32{}}, repairScopeInventoryCommon, 0, orphanGoods, 1, 500)
	if rec.Resolution != ResolutionUnknown {
		t.Fatalf("resolution = %q, want unknown", rec.Resolution)
	}
	if rec.UnknownReason != UnknownReasonMissingDBEntry {
		t.Errorf("unknown reason = %q, want missing_db_entry", rec.UnknownReason)
	}
	if rec.Identity != IdentityHandleEncoded {
		t.Errorf("identity = %q, want handle_encoded (derived from prefix even when DB-unknown)", rec.Identity)
	}
}

// TestResolveRecord_UnknownPrefixDBCollision is the Blocker-1 regression: a
// handle whose type prefix is illegal (0x10) but whose raw value collides
// numerically with a real DB itemID (0x10009C40 == Iron Helmet). The illegal
// prefix must win — the record is unknown_handle_type, never a lucky known_db
// match, and the scanner emits unknown_handle_type (not unknown_item_id).
func TestResolveRecord_UnknownPrefixDBCollision(t *testing.T) {
	const collisionHandle = uint32(0x10009C40) // illegal prefix 0x10, == Iron Helmet itemID
	rec := ResolveRecord(&SaveSlot{GaMap: map[uint32]uint32{}}, repairScopeInventoryCommon, 0, collisionHandle, 1, 500)

	if rec.Resolution != ResolutionUnknown {
		t.Fatalf("resolution = %q, want unknown (DB collision must not resolve an illegal prefix)", rec.Resolution)
	}
	if rec.UnknownReason != UnknownReasonUnknownHandleType {
		t.Errorf("unknown reason = %q, want unknown_handle_type", rec.UnknownReason)
	}
	if rec.Identity != IdentityUnknown {
		t.Errorf("identity = %q, want unknown", rec.Identity)
	}
	if rec.Name != "" {
		t.Errorf("name = %q, want empty (must not adopt the colliding DB entry's name)", rec.Name)
	}

	slot := &SaveSlot{
		Inventory: EquipInventoryData{CommonItems: []InventoryItem{{GaItemHandle: collisionHandle, Quantity: 1, Index: 500}}},
	}
	sawHandleType, sawItemID := false, false
	for _, iss := range ScanRepairIssues(0, slot) {
		switch iss.Key.Code {
		case RepairCodeUnknownHandleType:
			sawHandleType = true
		case RepairCodeUnknownItemID:
			sawItemID = true
		}
	}
	if !sawHandleType {
		t.Error("scanner must emit unknown_handle_type for the illegal-prefix collision handle")
	}
	if sawItemID {
		t.Error("scanner must NOT emit unknown_item_id for the illegal-prefix collision handle")
	}
}

// TestResolveRecord_UnknownPrefixBeatsNakedArmorGaMap confirms the handle-prefix
// gate wins over a GaMap entry pointing at a technical-placeholder itemID: an
// illegal prefix whose GaMap maps to a naked-armor row must still be
// unknown_handle_type, never a technical placeholder or unknown_item_id.
func TestResolveRecord_UnknownPrefixBeatsNakedArmorGaMap(t *testing.T) {
	const badHandle = uint32(0x10000005) // illegal prefix 0x10
	slot := &SaveSlot{
		GaMap:     map[uint32]uint32{badHandle: nakedHeadItemID}, // GaMap → Bare Head
		Inventory: EquipInventoryData{CommonItems: []InventoryItem{{GaItemHandle: badHandle, Quantity: 1, Index: 500}}},
	}

	rec := ResolveRecord(slot, repairScopeInventoryCommon, 0, badHandle, 1, 500)
	if rec.Resolution != ResolutionUnknown {
		t.Fatalf("resolution = %q, want unknown (illegal prefix must beat naked-armor GaMap)", rec.Resolution)
	}
	if rec.UnknownReason != UnknownReasonUnknownHandleType {
		t.Errorf("unknown reason = %q, want unknown_handle_type", rec.UnknownReason)
	}
	if rec.Identity != IdentityUnknown {
		t.Errorf("identity = %q, want unknown (not technical_placeholder)", rec.Identity)
	}
	if rec.Identity == IdentityTechnicalPlaceholder || rec.Resolution == ResolutionTechnicalPlaceholder {
		t.Error("illegal-prefix handle must not be classified as a technical placeholder")
	}

	sawHandleType, sawItemID := false, false
	for _, iss := range ScanRepairIssues(0, slot) {
		switch iss.Key.Code {
		case RepairCodeUnknownHandleType:
			sawHandleType = true
		case RepairCodeUnknownItemID:
			sawItemID = true
		}
	}
	if !sawHandleType {
		t.Error("scanner must emit unknown_handle_type for illegal prefix with naked-armor GaMap")
	}
	if sawItemID {
		t.Error("scanner must NOT emit unknown_item_id for illegal prefix with naked-armor GaMap")
	}
}

// TestStoragePhysicalRows_Gap is the Blocker-3 regression: slot.Storage.CommonItems
// is compacted, so a record physically at binary slot 3 sits at compacted row 1.
// Row (used by repair primitives) stays compacted; PhysicalRow exposes the raw
// binary slot. Fingerprint stays bound to the compacted record.
func TestStoragePhysicalRows_Gap(t *testing.T) {
	const boxOff = 100
	const start = boxOff + StorageHeaderSkip // 104
	data := make([]byte, start+StorageCommonCount*InvRecordLen)
	put := func(physical int, handle, qty, idx uint32) {
		off := start + physical*InvRecordLen
		binary.LittleEndian.PutUint32(data[off:], handle)
		binary.LittleEndian.PutUint32(data[off+4:], qty)
		binary.LittleEndian.PutUint32(data[off+8:], idx)
	}
	// Physical slots 0 and 3 filled; 1 and 2 are gaps.
	put(0, testHandleSmithingStone, 5, 700)
	put(3, 0xA00017B6, 1, 701) // talisman
	binary.LittleEndian.PutUint32(data[boxOff:], 2)

	slot := &SaveSlot{
		Data:             data,
		StorageBoxOffset: boxOff,
		GaMap:            map[uint32]uint32{},
		Storage: EquipInventoryData{CommonItems: []InventoryItem{
			{GaItemHandle: testHandleSmithingStone, Quantity: 5, Index: 700},
			{GaItemHandle: 0xA00017B6, Quantity: 1, Index: 701},
		}},
	}

	var storage []ResolvedRecord
	for _, r := range ResolveInventoryRecords(slot) {
		if r.Scope == repairScopeStorageCommon {
			storage = append(storage, r)
		}
	}
	if len(storage) != 2 {
		t.Fatalf("expected 2 storage records, got %d", len(storage))
	}
	if storage[0].Row != 0 || storage[0].PhysicalRow != 0 {
		t.Errorf("record 0: Row=%d PhysicalRow=%d, want 0/0", storage[0].Row, storage[0].PhysicalRow)
	}
	if storage[1].Row != 1 || storage[1].PhysicalRow != 3 {
		t.Errorf("record 1: Row=%d PhysicalRow=%d, want compacted 1 / physical 3", storage[1].Row, storage[1].PhysicalRow)
	}
	// Fingerprint / apply row stay bound to the compacted Row, so an apply
	// endpoint locating storage row 1 finds the same record.
	fp, ok := FingerprintRecordAt(slot, repairScopeStorageCommon, storage[1].Row)
	if !ok || fp != storage[1].Fingerprint {
		t.Errorf("fingerprint at compacted row 1 = %q ok=%v, want %q", fp, ok, storage[1].Fingerprint)
	}
}

// TestStoragePhysicalRows_FallbackNoRawData is the Blocker-3 fallback variant:
// a synthetic slot with a compacted list but no raw storage bytes must fall back
// to PhysicalRow == Row rather than mis-mapping.
func TestStoragePhysicalRows_FallbackNoRawData(t *testing.T) {
	slot := &SaveSlot{
		GaMap:            map[uint32]uint32{},
		StorageBoxOffset: 0, // no raw storage data available
		Storage: EquipInventoryData{CommonItems: []InventoryItem{
			{GaItemHandle: testHandleSmithingStone, Quantity: 5, Index: 700},
			{GaItemHandle: 0xA00017B6, Quantity: 1, Index: 701},
		}},
	}
	for _, r := range ResolveInventoryRecords(slot) {
		if r.Scope != repairScopeStorageCommon {
			continue
		}
		if r.PhysicalRow != r.Row {
			t.Errorf("row %d: PhysicalRow=%d, want == Row (fallback identity mapping)", r.Row, r.PhysicalRow)
		}
	}
}

// TestScanRepairIssuesWithCoverage_StructuralAfterScan is the Blocker-4
// regression: the coverage builder alone must not declare structural checks;
// the pipeline fills StructuralChecksApplied only after the scanner runs, and
// every physical record is counted exactly once.
func TestScanRepairIssuesWithCoverage_StructuralAfterScan(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{nakedHeadHandle: nakedHeadItemID},
		Inventory: EquipInventoryData{CommonItems: []InventoryItem{
			{GaItemHandle: testHandleSmithingStone, Quantity: 5, Index: 500},
			{GaItemHandle: nakedHeadHandle, Quantity: 1, Index: 501},
			{GaItemHandle: 0x10000005, Quantity: 1, Index: 502}, // unknown prefix
		}},
	}
	records := ResolveInventoryRecords(slot)

	// Builder alone: no structural checks claimed.
	pre := BuildCoverageReport(records)
	if pre.StructuralChecksApplied != 0 {
		t.Errorf("builder StructuralChecksApplied = %d, want 0 before scanner runs", pre.StructuralChecksApplied)
	}
	if pre.ResolutionChecksApplied != pre.TotalPhysical {
		t.Errorf("builder ResolutionChecksApplied = %d, want TotalPhysical %d", pre.ResolutionChecksApplied, pre.TotalPhysical)
	}

	// After the scan: structural checks reflect the records actually processed.
	_, cov := ScanRepairIssuesWithCoverage(0, slot, records)
	if cov.StructuralChecksApplied != cov.TotalPhysical {
		t.Errorf("post-scan StructuralChecksApplied = %d, want TotalPhysical %d (each record checked once)",
			cov.StructuralChecksApplied, cov.TotalPhysical)
	}
	if cov.TotalPhysical != len(records) {
		t.Errorf("TotalPhysical = %d, want %d (one count per physical record)", cov.TotalPhysical, len(records))
	}
}

// TestBuildCoverageReport_Partition confirms the coverage invariants and that
// every non-empty physical record is counted exactly once.
func TestBuildCoverageReport_Partition(t *testing.T) {
	slot := &SaveSlot{
		Version: 1,
		GaMap: map[uint32]uint32{
			realDaggerHandle: realDaggerItemID, // known DB weapon
			nakedHeadHandle:  nakedHeadItemID,  // technical placeholder
		},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: realDaggerHandle, Quantity: 1, Index: 500},        // known DB
				{GaItemHandle: testHandleSmithingStone, Quantity: 5, Index: 501}, // known DB (goods)
				{GaItemHandle: nakedHeadHandle, Quantity: 1, Index: 502},         // technical placeholder
				{GaItemHandle: 0x10000005, Quantity: 1, Index: 503},              // unknown prefix
				{GaItemHandle: GaHandleEmpty, Quantity: 0, Index: 0},             // skipped
			},
		},
	}

	records := ResolveInventoryRecords(slot)
	if len(records) != 4 {
		t.Fatalf("expected 4 non-empty records, got %d", len(records))
	}
	cov := BuildCoverageReport(records)

	if cov.TotalPhysical != 4 {
		t.Errorf("TotalPhysical = %d, want 4", cov.TotalPhysical)
	}
	if got := cov.KnownDB + cov.TechnicalPlaceholder + cov.Unknown; got != cov.TotalPhysical {
		t.Errorf("partition broken: KnownDB(%d)+TechnicalPlaceholder(%d)+Unknown(%d) = %d, want TotalPhysical %d",
			cov.KnownDB, cov.TechnicalPlaceholder, cov.Unknown, got, cov.TotalPhysical)
	}
	if cov.Resolved != cov.KnownDB+cov.TechnicalPlaceholder {
		t.Errorf("Resolved = %d, want KnownDB+TechnicalPlaceholder = %d", cov.Resolved, cov.KnownDB+cov.TechnicalPlaceholder)
	}
	// The builder alone must NOT claim structural checks — the scanner has not
	// run yet. It reports only the resolution work it genuinely did.
	if cov.ResolutionChecksApplied != cov.TotalPhysical {
		t.Errorf("ResolutionChecksApplied = %d, want TotalPhysical %d", cov.ResolutionChecksApplied, cov.TotalPhysical)
	}
	if cov.StructuralChecksApplied != 0 {
		t.Errorf("StructuralChecksApplied = %d, want 0 before the scanner runs", cov.StructuralChecksApplied)
	}
	if cov.CategoryChecksApplied != 0 {
		t.Errorf("CategoryChecksApplied = %d, want 0 (no category validator in Prompt 12)", cov.CategoryChecksApplied)
	}
	if cov.KnownDB != 2 || cov.TechnicalPlaceholder != 1 || cov.Unknown != 1 {
		t.Errorf("counts KnownDB=%d TechnicalPlaceholder=%d Unknown=%d, want 2/1/1", cov.KnownDB, cov.TechnicalPlaceholder, cov.Unknown)
	}
}

// TestScanRepairIssues_NakedArmorNotUnknown confirms the scanner no longer emits
// unknown_item_id for naked armor rows (they are technical placeholders).
func TestScanRepairIssues_NakedArmorNotUnknown(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{
			nakedHeadHandle: nakedHeadItemID,
			nakedBodyHandle: nakedBodyItemID,
			nakedArmsHandle: nakedArmsItemID,
			nakedLegsHandle: nakedLegsItemID,
		},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: nakedHeadHandle, Quantity: 1, Index: 500},
				{GaItemHandle: nakedBodyHandle, Quantity: 1, Index: 501},
				{GaItemHandle: nakedArmsHandle, Quantity: 1, Index: 502},
				{GaItemHandle: nakedLegsHandle, Quantity: 1, Index: 503},
			},
		},
	}
	for _, iss := range ScanRepairIssues(0, slot) {
		if iss.Key.Code == RepairCodeUnknownItemID || iss.Key.Code == RepairCodeUnknownHandleType {
			t.Errorf("naked armor must not be reported as %s: %q", iss.Key.Code, iss.Description)
		}
	}
}

// TestScanRepairIssues_MissingGaItemMapping confirms an instance-backed weapon
// handle that resolves in the DB but has no GaItem is flagged with the distinct
// missing_gaitem_mapping code (not unknown_item_id).
func TestScanRepairIssues_MissingGaItemMapping(t *testing.T) {
	// 0x800F4240 → HandleToItemID → 0x000F4240 (Dagger base, in DB), no GaMap.
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: realDaggerHandle, Quantity: 1, Index: 500},
			},
		},
	}
	found := false
	for _, iss := range ScanRepairIssues(0, slot) {
		if iss.Key.Code == RepairCodeMissingGaItemMapping {
			found = true
		}
		if iss.Key.Code == RepairCodeUnknownItemID {
			t.Errorf("resolved weapon without GaItem must not be unknown_item_id: %q", iss.Description)
		}
	}
	if !found {
		t.Error("expected missing_gaitem_mapping issue for instance-backed handle without GaItem")
	}
}

// TestScanRepairIssuesFromRecords_SharesResolution confirms the scanner and the
// coverage report, fed from one resolved collection, never diverge.
func TestScanRepairIssuesFromRecords_SharesResolution(t *testing.T) {
	slot := &SaveSlot{
		GaMap: map[uint32]uint32{nakedHeadHandle: nakedHeadItemID},
		Inventory: EquipInventoryData{
			CommonItems: []InventoryItem{
				{GaItemHandle: nakedHeadHandle, Quantity: 1, Index: 500},
				{GaItemHandle: testHandleSmithingStone, Quantity: 5, Index: 501},
			},
		},
	}
	records := ResolveInventoryRecords(slot)
	cov := BuildCoverageReport(records)
	issues := ScanRepairIssuesFromRecords(0, slot, records)

	// Coverage sees 1 placeholder + 1 known DB, 0 unknown.
	if cov.Unknown != 0 {
		t.Errorf("coverage Unknown = %d, want 0", cov.Unknown)
	}
	// Scanner must not report any unknown/missing issue for these records.
	for _, iss := range issues {
		switch iss.Key.Code {
		case RepairCodeUnknownItemID, RepairCodeUnknownHandleType, RepairCodeMissingGaItemMapping:
			t.Errorf("scanner diverged from coverage: unexpected %s (%q)", iss.Key.Code, iss.Description)
		}
	}
}
