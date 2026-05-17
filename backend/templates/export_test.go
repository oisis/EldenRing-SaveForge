package templates

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// fixedTime keeps CreatedAt deterministic across runs.
var fixedTime = time.Date(2026, time.May, 17, 12, 34, 56, 0, time.UTC)

func defaultOpts() ExportOptions {
	return ExportOptions{
		IncludeInventory: true,
		IncludeStorage:   true,
		AppVersion:       "0.15.0-beta",
		Now:              fixedTime,
	}
}

// weapon returns an editable weapon that BuildTemplateFromSnapshot can
// project into a TemplateItem. AoW state is parameterised via the status
// argument to exercise the four AoWStatus* code paths in one helper.
func weapon(uid string, baseID, aowItemID uint32, status string, position int) editor.EditableItem {
	it := editor.EditableItem{
		UID:              uid,
		Source:           editor.ItemSourceOriginal,
		Container:        editor.ContainerInventory,
		Position:         position,
		OriginalHandle:   0xA0000001,
		ItemID:           baseID,
		BaseItemID:       baseID,
		Name:             "Greatsword",
		Category:         "melee_armaments",
		Quantity:         1,
		AcquisitionIndex: uint32(position),
		CurrentUpgrade:   25,
		MaxUpgrade:       25,
		InfusionName:     "Heavy",
		HasGaItem:        true,
		IsWeapon:         true,
		CurrentAoWStatus: status,
		CurrentAoWItemID: aowItemID,
		CurrentAoWName:   "Wild Strikes",
		HasCurrentAoW:    status != editor.AoWStatusNone && status != "",
	}
	if status == editor.AoWStatusShared {
		it.CurrentAoWShared = true
	}
	return it
}

func armor(uid string, baseID uint32, position int) editor.EditableItem {
	return editor.EditableItem{
		UID:              uid,
		Source:           editor.ItemSourceOriginal,
		Container:        editor.ContainerInventory,
		Position:         position,
		OriginalHandle:   0xA0000010,
		ItemID:           baseID,
		BaseItemID:       baseID,
		Name:             "Carian Knight Helm",
		Category:         "head",
		Quantity:         1,
		AcquisitionIndex: uint32(position),
		HasGaItem:        true,
		IsArmor:          true,
	}
}

func stackable(uid string, baseID uint32, qty uint32, position int, container editor.ContainerKind) editor.EditableItem {
	return editor.EditableItem{
		UID:              uid,
		Source:           editor.ItemSourceOriginal,
		Container:        container,
		Position:         position,
		ItemID:           baseID,
		BaseItemID:       baseID,
		Name:             "Standard Arrow",
		Category:         "ammo",
		Quantity:         qty,
		AcquisitionIndex: uint32(position),
	}
}

func TestExport_HappyPath_InventoryAndStorage(t *testing.T) {
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{
			weapon("hnd:0x01", 0x003D9700, 0x80002710, editor.AoWStatusCustom, 0),
			armor("hnd:0x02", 0x10A53880, 1),
		},
		StorageItems: []editor.EditableItem{
			stackable("hnd:0x03", 0x02FAF080, 99, 0, editor.ContainerStorage),
		},
	}

	tpl, report, err := BuildTemplateFromSnapshot(snap, defaultOpts())
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if tpl.Schema != SchemaKey || tpl.Version != SchemaVersion {
		t.Fatalf("schema header wrong: %q v%d", tpl.Schema, tpl.Version)
	}
	if tpl.CreatedAt != "2026-05-17T12:34:56Z" {
		t.Fatalf("CreatedAt not deterministic: %q", tpl.CreatedAt)
	}
	if tpl.AppVersion != "0.15.0-beta" {
		t.Fatalf("AppVersion lost: %q", tpl.AppVersion)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("expected zero warnings, got %+v", report.Warnings)
	}
	sec := tpl.Sections.InventoryWorkspace
	if sec == nil || len(sec.InventoryItems) != 2 || len(sec.StorageItems) != 1 {
		t.Fatalf("section payload wrong: %+v", sec)
	}

	if got := sec.InventoryItems[0]; got.BaseItemID != 0x003D9700 || got.Upgrade != 25 || got.InfusionName != "Heavy" {
		t.Fatalf("weapon fields wrong: %+v", got)
	}
	if got := sec.InventoryItems[0]; got.AoWItemID == nil || *got.AoWItemID != 0x80002710 {
		t.Fatalf("weapon AoWItemID not exported: %v", got.AoWItemID)
	}
	if got := sec.StorageItems[0]; got.Container != ContainerStorage || got.Quantity != 99 {
		t.Fatalf("storage stackable wrong: %+v", got)
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("exported template fails validation: %v", err)
	}
}

func TestExport_InventoryOnly(t *testing.T) {
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{armor("hnd:0x01", 0x10A53880, 0)},
		StorageItems:   []editor.EditableItem{stackable("hnd:0x02", 0x02FAF080, 50, 0, editor.ContainerStorage)},
	}
	opts := defaultOpts()
	opts.IncludeStorage = false

	tpl, _, err := BuildTemplateFromSnapshot(snap, opts)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	sec := tpl.Sections.InventoryWorkspace
	if len(sec.InventoryItems) != 1 {
		t.Fatalf("inventory not exported: %+v", sec.InventoryItems)
	}
	if len(sec.StorageItems) != 0 {
		t.Fatalf("storage must be empty when IncludeStorage=false, got %+v", sec.StorageItems)
	}
}

func TestExport_StorageOnly(t *testing.T) {
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{armor("hnd:0x01", 0x10A53880, 0)},
		StorageItems:   []editor.EditableItem{stackable("hnd:0x02", 0x02FAF080, 50, 0, editor.ContainerStorage)},
	}
	opts := defaultOpts()
	opts.IncludeInventory = false

	tpl, _, err := BuildTemplateFromSnapshot(snap, opts)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	sec := tpl.Sections.InventoryWorkspace
	if len(sec.InventoryItems) != 0 {
		t.Fatalf("inventory must be empty when IncludeInventory=false, got %+v", sec.InventoryItems)
	}
	if len(sec.StorageItems) != 1 {
		t.Fatalf("storage not exported: %+v", sec.StorageItems)
	}
}

func TestExport_RejectsBothExcluded(t *testing.T) {
	snap := editor.InventoryWorkspaceSnapshot{}
	opts := ExportOptions{IncludeInventory: false, IncludeStorage: false}
	if _, _, err := BuildTemplateFromSnapshot(snap, opts); err == nil {
		t.Fatal("expected error when both containers excluded")
	}
}

func TestExport_OrderPreservedByPosition(t *testing.T) {
	// Provided items out of order — exporter must stable-sort by Position
	// and emit array indices that match.
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{
			armor("hnd:0xB", 0x10A53880, 2),
			armor("hnd:0xA", 0x10A53881, 0),
			armor("hnd:0xC", 0x10A53882, 1),
		},
	}
	opts := defaultOpts()
	opts.IncludeStorage = false

	tpl, report, err := BuildTemplateFromSnapshot(snap, opts)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	got := tpl.Sections.InventoryWorkspace.InventoryItems
	if len(got) != 3 {
		t.Fatalf("expected 3 items, got %d", len(got))
	}
	if got[0].BaseItemID != 0x10A53881 || got[1].BaseItemID != 0x10A53882 || got[2].BaseItemID != 0x10A53880 {
		t.Fatalf("sort by Position broken: %+v", got)
	}
	for i, it := range got {
		if it.Position != i {
			t.Errorf("Position not renormalized: index %d → Position %d", i, it.Position)
		}
	}
	// Positions matched array indices after sort, so no normalization
	// warnings expected here.
	if len(report.Warnings) != 0 {
		t.Fatalf("expected no warnings when positions are unique and sortable, got %+v", report.Warnings)
	}
}

func TestExport_InconsistentPositionsEmitWarning(t *testing.T) {
	// Two items both reporting Position=0 — stable sort keeps insertion
	// order, exporter renormalises to 0,1 and emits one warning for the
	// second one whose final array index (1) doesn't match its claimed
	// position (0).
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{
			armor("hnd:0xA", 0x10A53880, 0),
			armor("hnd:0xB", 0x10A53881, 0),
		},
	}
	opts := defaultOpts()
	opts.IncludeStorage = false

	tpl, report, err := BuildTemplateFromSnapshot(snap, opts)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	if got := tpl.Sections.InventoryWorkspace.InventoryItems[1].Position; got != 1 {
		t.Fatalf("second item not renormalized to 1, got %d", got)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Code != WarnCodePositionNormalized {
		t.Fatalf("expected exactly one position_normalized warning, got %+v", report.Warnings)
	}
}

func TestExport_CustomAoWEmitsItemID(t *testing.T) {
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{
			weapon("hnd:0x01", 0x003D9700, 0x80002710, editor.AoWStatusCustom, 0),
		},
	}
	opts := defaultOpts()
	opts.IncludeStorage = false

	tpl, report, err := BuildTemplateFromSnapshot(snap, opts)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	got := tpl.Sections.InventoryWorkspace.InventoryItems[0]
	if got.AoWItemID == nil || *got.AoWItemID != 0x80002710 {
		t.Fatalf("custom AoW not exported: %v", got.AoWItemID)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("custom AoW must not warn, got %+v", report.Warnings)
	}
}

func TestExport_NoCustomAoWOmitsItemID(t *testing.T) {
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{
			weapon("hnd:0x01", 0x003D9700, 0, editor.AoWStatusNone, 0),
		},
	}
	opts := defaultOpts()
	opts.IncludeStorage = false

	tpl, report, err := BuildTemplateFromSnapshot(snap, opts)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	got := tpl.Sections.InventoryWorkspace.InventoryItems[0]
	if got.AoWItemID != nil {
		t.Fatalf("status=none must omit aowItemID, got %v", *got.AoWItemID)
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("status=none must not warn, got %+v", report.Warnings)
	}

	data, _ := json.Marshal(tpl)
	if strings.Contains(string(data), "aowItemID") {
		t.Errorf("aowItemID must be absent from JSON for status=none, got:\n%s", string(data))
	}
}

func TestExport_MissingAoWWarnsAndOmits(t *testing.T) {
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{
			weapon("hnd:0x01", 0x003D9700, 0, editor.AoWStatusMissing, 0),
		},
	}
	opts := defaultOpts()
	opts.IncludeStorage = false

	tpl, report, err := BuildTemplateFromSnapshot(snap, opts)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	got := tpl.Sections.InventoryWorkspace.InventoryItems[0]
	if got.AoWItemID != nil {
		t.Fatalf("missing AoW must omit aowItemID, got %v", *got.AoWItemID)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Code != WarnCodeAoWMissingSkipped {
		t.Fatalf("expected one aow_missing_skipped warning, got %+v", report.Warnings)
	}
}

func TestExport_SharedAoWWarnsAndOmits(t *testing.T) {
	// AoW status=shared even though CurrentAoWItemID is populated — the
	// exporter must NOT emit the ID because shared handles are corruption
	// indicators and the importer cannot recreate the sharing safely.
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{
			weapon("hnd:0x01", 0x003D9700, 0x80002710, editor.AoWStatusShared, 0),
		},
	}
	opts := defaultOpts()
	opts.IncludeStorage = false

	tpl, report, err := BuildTemplateFromSnapshot(snap, opts)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	got := tpl.Sections.InventoryWorkspace.InventoryItems[0]
	if got.AoWItemID != nil {
		t.Fatalf("shared AoW must omit aowItemID, got %v", *got.AoWItemID)
	}
	if len(report.Warnings) != 1 || report.Warnings[0].Code != WarnCodeAoWSharedSkipped {
		t.Fatalf("expected one aow_shared_skipped warning, got %+v", report.Warnings)
	}
}

func TestExport_ZeroBaseItemIDIsError(t *testing.T) {
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{
			{
				UID:        "hnd:0x01",
				Container:  editor.ContainerInventory,
				Position:   0,
				BaseItemID: 0,
				Quantity:   1,
			},
		},
	}
	if _, _, err := BuildTemplateFromSnapshot(snap, defaultOpts()); err == nil {
		t.Fatal("expected error for baseItemID=0")
	}
}

func TestExport_ZeroQuantityIsError(t *testing.T) {
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{
			{
				UID:        "hnd:0x01",
				Container:  editor.ContainerInventory,
				Position:   0,
				BaseItemID: 0x10A53880,
				Quantity:   0,
			},
		},
	}
	if _, _, err := BuildTemplateFromSnapshot(snap, defaultOpts()); err == nil {
		t.Fatal("expected error for quantity=0")
	}
}

// TestExport_IgnoresPendingFieldsAndPassThroughRecords confirms two
// Phase A invariants in one fixture:
//   - Pending* AoW edits on EditableItem do not leak into the template
//     (we export "current saved/applied state", not unsaved edits).
//   - UnsupportedInventoryRecords / UnsupportedStorageRecords are
//     excluded from the MVP scope.
func TestExport_IgnoresPendingFieldsAndPassThroughRecords(t *testing.T) {
	w := weapon("hnd:0x01", 0x003D9700, 0x80002710, editor.AoWStatusCustom, 0)
	// Simulate an unsaved AoW edit: pending fields set but not yet
	// committed. Template must reflect the *current* AoW item ID
	// (0x80002710), not the pending one.
	w.PendingAoWItemID = 0x80009999
	w.PendingAoWName = "Carian Retaliation"
	w.HasPendingWeaponPatch = true

	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{w},
		UnsupportedInventoryRecords: []editor.RawInventoryRecord{
			{Container: editor.ContainerInventory, SlotIndex: 5, Handle: 0xB0000005, ItemID: 0x12345678, Quantity: 1, Reason: editor.ReasonUnsupportedCategory, Name: "Test Spell"},
		},
		UnsupportedStorageRecords: []editor.RawInventoryRecord{
			{Container: editor.ContainerStorage, SlotIndex: 7, Handle: 0xB0000006, ItemID: 0x12345679, Quantity: 1, Reason: editor.ReasonUnknownItem},
		},
	}

	tpl, _, err := BuildTemplateFromSnapshot(snap, defaultOpts())
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	got := tpl.Sections.InventoryWorkspace.InventoryItems[0]
	if got.AoWItemID == nil || *got.AoWItemID != 0x80002710 {
		t.Fatalf("export must mirror current AoW, not pending: %v", got.AoWItemID)
	}

	data, _ := json.Marshal(tpl)
	body := string(data)
	if strings.Contains(body, "Carian Retaliation") || strings.Contains(body, "pendingAoW") {
		t.Errorf("pending AoW edit leaked into template JSON:\n%s", body)
	}
	// Unsupported records must not appear in the template payload.
	if strings.Contains(body, "0x12345678") || strings.Contains(body, "Test Spell") {
		t.Errorf("pass-through record leaked into template JSON:\n%s", body)
	}
}

// TestExport_EmptySnapshotProducesEmptyButValidStructure documents the
// edge case: an empty workspace still produces a well-formed template
// struct, but ValidateBuildTemplate will reject it because both
// containers are empty. The contract is "export succeeds, validation
// fails downstream" — keeping export non-fatal lets the UI report the
// empty state cleanly via the validator.
func TestExport_EmptySnapshotProducesEmptyButValidStructure(t *testing.T) {
	snap := editor.InventoryWorkspaceSnapshot{}
	tpl, report, err := BuildTemplateFromSnapshot(snap, defaultOpts())
	if err != nil {
		t.Fatalf("export of empty snapshot must not error: %v", err)
	}
	if tpl.Sections.InventoryWorkspace == nil {
		t.Fatal("section pointer must always be non-nil")
	}
	if len(report.Warnings) != 0 {
		t.Fatalf("empty snapshot must not warn, got %+v", report.Warnings)
	}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("validation must reject empty template")
	}
}

func TestExport_DefaultNowFallsBackToTimeNow(t *testing.T) {
	snap := editor.InventoryWorkspaceSnapshot{
		InventoryItems: []editor.EditableItem{armor("hnd:0x01", 0x10A53880, 0)},
	}
	opts := defaultOpts()
	opts.Now = time.Time{}
	before := time.Now().UTC()
	tpl, _, err := BuildTemplateFromSnapshot(snap, opts)
	if err != nil {
		t.Fatalf("export failed: %v", err)
	}
	parsed, err := time.Parse(time.RFC3339, tpl.CreatedAt)
	if err != nil {
		t.Fatalf("CreatedAt not RFC3339: %q", tpl.CreatedAt)
	}
	if parsed.Before(before.Add(-time.Second)) || parsed.After(time.Now().UTC().Add(time.Second)) {
		t.Fatalf("CreatedAt fallback far from time.Now(): parsed=%v before=%v", parsed, before)
	}
}
