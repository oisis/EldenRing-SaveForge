package templates

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// Phase 8C tests — backend builder for the v2 items / inventoryLayout /
// storageLayout sections. Apply / writer / UI paths are NOT exercised
// here; this file covers only the export contract.

// ─── fixture builders ──────────────────────────────────────────────────

func editableWeapon(uid string, baseID uint32, name string, container editor.ContainerKind, position int, maxUpgrade, currentUpgrade int) editor.EditableItem {
	return editor.EditableItem{
		UID:            uid,
		Container:      container,
		Position:       position,
		BaseItemID:     baseID,
		ItemID:         baseID,
		Name:           name,
		Category:       ItemCategoryMeleeArmaments,
		Quantity:       1,
		CurrentUpgrade: currentUpgrade,
		MaxUpgrade:     maxUpgrade,
		IsWeapon:       true,
	}
}

func editableArmor(uid string, baseID uint32, name string, container editor.ContainerKind, position int, category string) editor.EditableItem {
	return editor.EditableItem{
		UID:        uid,
		Container:  container,
		Position:   position,
		BaseItemID: baseID,
		ItemID:     baseID,
		Name:       name,
		Category:   category,
		Quantity:   1,
		MaxUpgrade: 0,
		IsArmor:    true,
	}
}

func editableTalisman(uid string, baseID uint32, name string, container editor.ContainerKind, position int) editor.EditableItem {
	return editor.EditableItem{
		UID:        uid,
		Container:  container,
		Position:   position,
		BaseItemID: baseID,
		ItemID:     baseID,
		Name:       name,
		Category:   ItemCategoryTalismans,
		Quantity:   1,
		IsTalisman: true,
	}
}

func selectAllItemsLayouts() *TemplateSelection {
	return &TemplateSelection{
		Items:           &SectionSelection{All: true},
		InventoryLayout: &SectionSelection{All: true},
		StorageLayout:   &SectionSelection{All: true},
	}
}

// ─── A. baseline negative paths ─────────────────────────────────────────

func TestBuildV2_NoItemsOrLayoutSelected_StillBuildsProfileOnly(t *testing.T) {
	tpl, err := BuildV2Template(ExportV2Options{
		Now:       fixedNow(),
		Profile:   &ProfileSource{Level: u32p(50)},
		Selection: &TemplateSelection{Profile: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Items != nil {
		t.Error("Sections.Items should remain nil without selection")
	}
	if tpl.Sections.InventoryLayout != nil {
		t.Error("Sections.InventoryLayout should remain nil without selection")
	}
	if tpl.Sections.StorageLayout != nil {
		t.Error("Sections.StorageLayout should remain nil without selection")
	}
}

func TestBuildV2_ItemsSelectedWithoutSource_Errors(t *testing.T) {
	_, err := BuildV2Template(ExportV2Options{
		Now:       fixedNow(),
		Selection: &TemplateSelection{Items: &SectionSelection{All: true}},
	})
	if err == nil {
		t.Fatal("expected error when selection.items is set but ItemsSource is nil")
	}
	if !strings.Contains(err.Error(), "ItemsSource") {
		t.Errorf("error should mention ItemsSource; got: %v", err)
	}
}

func TestBuildV2_LayoutSelectedWithoutItems_Errors(t *testing.T) {
	_, err := BuildV2Template(ExportV2Options{
		Now:         fixedNow(),
		ItemsSource: &ItemsLayoutSource{},
		Selection:   &TemplateSelection{InventoryLayout: &SectionSelection{All: true}},
	})
	if err == nil {
		t.Fatal("expected error when layout is selected without items selection")
	}
	if !strings.Contains(err.Error(), "selection.items") {
		t.Errorf("error should require selection.items; got: %v", err)
	}
}

// ─── B. items export ────────────────────────────────────────────────────

func TestBuildV2_ItemsOnly_EmitsItemsSectionAndSkipsLayouts(t *testing.T) {
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-w-0", 0x003D9700, "Greatsword", editor.ContainerInventory, 0, 25, 25),
			editableTalisman("uid-t-0", 0x200003E8, "Crimson Amber Medallion", editor.ContainerInventory, 1),
		},
		StorageItems: []editor.EditableItem{
			editableArmor("uid-a-0", 0x10000001, "Iron Helmet", editor.ContainerStorage, 0, ItemCategoryArmorHead),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now:         fixedNow(),
		ItemsSource: src,
		Selection:   &TemplateSelection{Items: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.Items == nil {
		t.Fatal("expected Sections.Items to be set")
	}
	if got, want := len(tpl.Sections.Items.Entries), 3; got != want {
		t.Fatalf("items.entries len = %d, want %d", got, want)
	}
	// Layouts must NOT be emitted when only items is selected.
	if tpl.Sections.InventoryLayout != nil {
		t.Error("InventoryLayout should be nil when only items is selected")
	}
	if tpl.Sections.StorageLayout != nil {
		t.Error("StorageLayout should be nil when only items is selected")
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("validator rejected our own output: %v", err)
	}
}

func TestBuildV2_ItemsSplitInventoryStorage_LocationsCorrect(t *testing.T) {
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-w-0", 0x003D9700, "Greatsword", editor.ContainerInventory, 0, 25, 5),
		},
		StorageItems: []editor.EditableItem{
			editableWeapon("uid-w-1", 0x003D9701, "Longsword", editor.ContainerStorage, 0, 25, 0),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now:         fixedNow(),
		ItemsSource: src,
		Selection:   &TemplateSelection{Items: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	entries := tpl.Sections.Items.Entries
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	// Inventory entries are emitted first.
	if entries[0].Location != ItemLocationInventory {
		t.Errorf("entries[0].Location = %q, want inventory", entries[0].Location)
	}
	if !strings.HasPrefix(entries[0].EntryID, "inv_") {
		t.Errorf("entries[0].EntryID = %q, want inv_* prefix", entries[0].EntryID)
	}
	if entries[1].Location != ItemLocationStorage {
		t.Errorf("entries[1].Location = %q, want storage", entries[1].Location)
	}
	if !strings.HasPrefix(entries[1].EntryID, "sto_") {
		t.Errorf("entries[1].EntryID = %q, want sto_* prefix", entries[1].EntryID)
	}
}

func TestBuildV2_DuplicateSameItemIDGetsDifferentEntryID(t *testing.T) {
	// Two Greatswords, same baseItemID, different upgrade levels.
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-w-0", 0x003D9700, "Greatsword", editor.ContainerInventory, 0, 25, 25),
			editableWeapon("uid-w-1", 0x003D9700, "Greatsword (offhand)", editor.ContainerInventory, 1, 25, 10),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now:         fixedNow(),
		ItemsSource: src,
		Selection:   &TemplateSelection{Items: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	entries := tpl.Sections.Items.Entries
	if len(entries) != 2 {
		t.Fatalf("entries len = %d, want 2", len(entries))
	}
	if entries[0].EntryID == entries[1].EntryID {
		t.Fatalf("entryIDs must differ for duplicate same itemID; got both %q", entries[0].EntryID)
	}
	if entries[0].ItemID != entries[1].ItemID {
		t.Errorf("itemIDs must match (same baseItemID); got %#x and %#x", entries[0].ItemID, entries[1].ItemID)
	}
	if *entries[0].UpgradeLevel == *entries[1].UpgradeLevel {
		t.Errorf("upgrade levels must differ; got both %d", *entries[0].UpgradeLevel)
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("template validation: %v", err)
	}
}

func TestBuildV2_UpgradeStandard_EmitsKindAndLevel(t *testing.T) {
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-w-std", 0x003D9700, "Standard +25", editor.ContainerInventory, 0, 25, 25),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: &TemplateSelection{Items: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	e := tpl.Sections.Items.Entries[0]
	if e.UpgradeKind != UpgradeKindStandard {
		t.Errorf("UpgradeKind = %q, want %q", e.UpgradeKind, UpgradeKindStandard)
	}
	if e.UpgradeLevel == nil || *e.UpgradeLevel != 25 {
		t.Errorf("UpgradeLevel = %v, want 25", e.UpgradeLevel)
	}
}

func TestBuildV2_UpgradeSomber_EmitsKindAndLevel(t *testing.T) {
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-w-som", 0x003D9701, "Somber +10", editor.ContainerInventory, 0, 10, 10),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: &TemplateSelection{Items: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	e := tpl.Sections.Items.Entries[0]
	if e.UpgradeKind != UpgradeKindSomber {
		t.Errorf("UpgradeKind = %q, want %q", e.UpgradeKind, UpgradeKindSomber)
	}
	if e.UpgradeLevel == nil || *e.UpgradeLevel != 10 {
		t.Errorf("UpgradeLevel = %v, want 10", e.UpgradeLevel)
	}
}

func TestBuildV2_NonWeapon_NoUpgradeFields(t *testing.T) {
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableTalisman("uid-t-0", 0x200003E8, "Crimson Amber Medallion", editor.ContainerInventory, 0),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: &TemplateSelection{Items: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	e := tpl.Sections.Items.Entries[0]
	if e.UpgradeKind != "" {
		t.Errorf("non-weapon UpgradeKind = %q, want empty (treated as none)", e.UpgradeKind)
	}
	if e.UpgradeLevel != nil {
		t.Errorf("non-weapon UpgradeLevel = %v, want nil", e.UpgradeLevel)
	}
}

func TestBuildV2_InfusionAndAoW_EmittedForCustomStatus(t *testing.T) {
	aow := uint32(0x80002710)
	w := editableWeapon("uid-w-0", 0x003D9700, "Greatsword", editor.ContainerInventory, 0, 25, 25)
	w.InfusionName = "Heavy"
	w.CurrentAoWItemID = aow
	w.CurrentAoWStatus = editor.AoWStatusCustom
	src := &ItemsLayoutSource{InventoryItems: []editor.EditableItem{w}}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: &TemplateSelection{Items: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	e := tpl.Sections.Items.Entries[0]
	if e.InfusionName != "Heavy" {
		t.Errorf("InfusionName = %q, want Heavy", e.InfusionName)
	}
	if e.AshOfWarItemID == nil || *e.AshOfWarItemID != aow {
		t.Errorf("AshOfWarItemID = %v, want 0x%X", e.AshOfWarItemID, aow)
	}
}

func TestBuildV2_AoWMissingOrShared_NotEmitted(t *testing.T) {
	cases := []struct {
		status string
	}{
		{editor.AoWStatusMissing},
		{editor.AoWStatusShared},
		{editor.AoWStatusNone},
		{""},
	}
	for _, tc := range cases {
		t.Run("status="+tc.status, func(t *testing.T) {
			w := editableWeapon("uid-w-x", 0x003D9700, "Greatsword", editor.ContainerInventory, 0, 25, 25)
			w.CurrentAoWItemID = 0x80002710
			w.CurrentAoWStatus = tc.status
			src := &ItemsLayoutSource{InventoryItems: []editor.EditableItem{w}}
			tpl, err := BuildV2Template(ExportV2Options{
				Now: fixedNow(), ItemsSource: src,
				Selection: &TemplateSelection{Items: &SectionSelection{All: true}},
			})
			if err != nil {
				t.Fatalf("BuildV2Template: %v", err)
			}
			if tpl.Sections.Items.Entries[0].AshOfWarItemID != nil {
				t.Errorf("AshOfWarItemID must be omitted for status=%q", tc.status)
			}
		})
	}
}

// ─── C. per-row skips ───────────────────────────────────────────────────

func TestBuildV2_QuantityZero_IsSkipped(t *testing.T) {
	bad := editableWeapon("uid-w-0", 0x003D9700, "Bad", editor.ContainerInventory, 0, 25, 25)
	bad.Quantity = 0
	good := editableWeapon("uid-w-1", 0x003D9701, "Good", editor.ContainerInventory, 1, 25, 10)
	src := &ItemsLayoutSource{InventoryItems: []editor.EditableItem{bad, good}}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: &TemplateSelection{Items: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	entries := tpl.Sections.Items.Entries
	if len(entries) != 1 {
		t.Fatalf("expected 1 surviving entry; got %d", len(entries))
	}
	if entries[0].ItemID != 0x003D9701 {
		t.Errorf("wrong survivor: itemID=0x%X", entries[0].ItemID)
	}
}

func TestBuildV2_BaseItemIDZero_IsSkipped(t *testing.T) {
	bad := editableWeapon("uid-w-0", 0, "Bad", editor.ContainerInventory, 0, 25, 25)
	good := editableWeapon("uid-w-1", 0x003D9701, "Good", editor.ContainerInventory, 1, 25, 0)
	src := &ItemsLayoutSource{InventoryItems: []editor.EditableItem{bad, good}}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: &TemplateSelection{Items: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if len(tpl.Sections.Items.Entries) != 1 {
		t.Fatalf("expected 1 entry, got %d", len(tpl.Sections.Items.Entries))
	}
}

func TestBuildV2_UnknownCategory_IsSkipped(t *testing.T) {
	bad := editableWeapon("uid-w-0", 0x003D9700, "Bad", editor.ContainerInventory, 0, 25, 25)
	bad.Category = "mystery_box"
	src := &ItemsLayoutSource{InventoryItems: []editor.EditableItem{bad}}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: &TemplateSelection{Items: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if len(tpl.Sections.Items.Entries) != 0 {
		t.Errorf("unknown category must be skipped; got %d entries", len(tpl.Sections.Items.Entries))
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("template with empty items must still validate: %v", err)
	}
}

// ─── D. inventory + storage layout ──────────────────────────────────────

func TestBuildV2_InventoryLayout_ReferencesItemsEntries(t *testing.T) {
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-0", 0x003D9700, "A", editor.ContainerInventory, 0, 25, 25),
			editableWeapon("uid-1", 0x003D9701, "B", editor.ContainerInventory, 1, 10, 5),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: &TemplateSelection{
			Items:           &SectionSelection{All: true},
			InventoryLayout: &SectionSelection{All: true},
		},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.InventoryLayout == nil {
		t.Fatal("InventoryLayout not emitted")
	}
	entries := tpl.Sections.Items.Entries
	layout := tpl.Sections.InventoryLayout.Entries
	if len(layout) != len(entries) {
		t.Fatalf("layout entries=%d, items entries=%d", len(layout), len(entries))
	}
	entryIDs := map[string]bool{}
	for _, e := range entries {
		entryIDs[e.EntryID] = true
	}
	for i, le := range layout {
		if !entryIDs[le.EntryRef] {
			t.Errorf("layout[%d].EntryRef=%q does not resolve to any items entry", i, le.EntryRef)
		}
		if le.Position != i {
			t.Errorf("layout[%d].Position=%d, want compact %d", i, le.Position, i)
		}
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("validation failed: %v", err)
	}
}

func TestBuildV2_StorageLayout_ReferencesOnlyStorageEntries(t *testing.T) {
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-i-0", 0x003D9700, "A", editor.ContainerInventory, 0, 25, 25),
		},
		StorageItems: []editor.EditableItem{
			editableTalisman("uid-s-0", 0x200003E8, "Talisman", editor.ContainerStorage, 0),
			editableTalisman("uid-s-1", 0x200003E9, "Talisman+1", editor.ContainerStorage, 1),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: &TemplateSelection{
			Items:         &SectionSelection{All: true},
			StorageLayout: &SectionSelection{All: true},
		},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.StorageLayout == nil {
		t.Fatal("StorageLayout not emitted")
	}
	if tpl.Sections.InventoryLayout != nil {
		t.Error("InventoryLayout should be nil when only storageLayout is selected")
	}
	for i, le := range tpl.Sections.StorageLayout.Entries {
		if !strings.HasPrefix(le.EntryRef, "sto_") {
			t.Errorf("storageLayout[%d].EntryRef=%q must reference sto_* entry", i, le.EntryRef)
		}
		if le.Position != i {
			t.Errorf("storageLayout[%d].Position=%d, want compact %d", i, le.Position, i)
		}
	}
}

func TestBuildV2_LayoutSkipsKeepLayoutPositionsCompact(t *testing.T) {
	// inv[0] valid, inv[1] skipped (quantity=0), inv[2] valid.
	// Layout must be compact 0..1 with two surviving entries; the
	// skipped row must NOT leave a hole in `position`.
	skipped := editableWeapon("uid-i-1", 0x003D9701, "Skip", editor.ContainerInventory, 1, 25, 0)
	skipped.Quantity = 0
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-i-0", 0x003D9700, "Keep-A", editor.ContainerInventory, 0, 25, 25),
			skipped,
			editableWeapon("uid-i-2", 0x003D9702, "Keep-B", editor.ContainerInventory, 2, 10, 5),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: &TemplateSelection{
			Items:           &SectionSelection{All: true},
			InventoryLayout: &SectionSelection{All: true},
		},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	layout := tpl.Sections.InventoryLayout.Entries
	if len(layout) != 2 {
		t.Fatalf("layout entries = %d, want 2", len(layout))
	}
	if layout[0].Position != 0 || layout[1].Position != 1 {
		t.Errorf("positions must be compact 0,1; got %d,%d", layout[0].Position, layout[1].Position)
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Errorf("validation failed: %v", err)
	}
}

func TestBuildV2_EntryIDsArePerContainerStableAndUnique(t *testing.T) {
	// Two inventory entries + two storage entries; prefixes must
	// disambiguate the namespace.
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-i-0", 0x003D9700, "iw0", editor.ContainerInventory, 0, 25, 25),
			editableWeapon("uid-i-1", 0x003D9701, "iw1", editor.ContainerInventory, 1, 25, 10),
		},
		StorageItems: []editor.EditableItem{
			editableTalisman("uid-s-0", 0x200003E8, "st0", editor.ContainerStorage, 0),
			editableTalisman("uid-s-1", 0x200003E9, "st1", editor.ContainerStorage, 1),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: selectAllItemsLayouts(),
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	seen := map[string]bool{}
	for _, e := range tpl.Sections.Items.Entries {
		if seen[e.EntryID] {
			t.Fatalf("duplicate entryID %q from exporter", e.EntryID)
		}
		seen[e.EntryID] = true
	}
	for i, le := range tpl.Sections.InventoryLayout.Entries {
		if !strings.HasPrefix(le.EntryRef, "inv_") {
			t.Errorf("inventoryLayout[%d] entryRef=%q must start with inv_", i, le.EntryRef)
		}
	}
	for i, le := range tpl.Sections.StorageLayout.Entries {
		if !strings.HasPrefix(le.EntryRef, "sto_") {
			t.Errorf("storageLayout[%d] entryRef=%q must start with sto_", i, le.EntryRef)
		}
	}
}

// ─── E. validation + YAML round-trip ────────────────────────────────────

func TestBuildV2_OutputAlwaysPassesValidator(t *testing.T) {
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-w-0", 0x003D9700, "Greatsword", editor.ContainerInventory, 0, 25, 25),
			editableTalisman("uid-t-0", 0x200003E8, "Talisman", editor.ContainerInventory, 1),
		},
		StorageItems: []editor.EditableItem{
			editableArmor("uid-a-0", 0x10000001, "Helmet", editor.ContainerStorage, 0, ItemCategoryArmorHead),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: selectAllItemsLayouts(),
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	// BuildV2Template already runs the validator internally, but assert
	// once more so an accidental skip would be caught.
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("validator rejected exporter output: %v", err)
	}
}

func TestBuildV2_YAMLRoundTrip_PreservesItemsAndLayout(t *testing.T) {
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-w-0", 0x003D9700, "Greatsword", editor.ContainerInventory, 0, 25, 25),
			editableTalisman("uid-t-0", 0x200003E8, "Talisman", editor.ContainerInventory, 1),
		},
		StorageItems: []editor.EditableItem{
			editableArmor("uid-a-0", 0x10000001, "Helmet", editor.ContainerStorage, 0, ItemCategoryArmorHead),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: selectAllItemsLayouts(),
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	data, err := MarshalBuildTemplateYAML(tpl)
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	got, err := ParseBuildTemplateYAML(data)
	if err != nil {
		t.Fatalf("ParseYAML: %v\npayload:\n%s", err, data)
	}
	if got.Sections.Items == nil || len(got.Sections.Items.Entries) != 3 {
		t.Fatalf("items lost in roundtrip: %+v", got.Sections.Items)
	}
	if got.Sections.InventoryLayout == nil || len(got.Sections.InventoryLayout.Entries) != 2 {
		t.Fatalf("inventoryLayout lost in roundtrip: %+v", got.Sections.InventoryLayout)
	}
	if got.Sections.StorageLayout == nil || len(got.Sections.StorageLayout.Entries) != 1 {
		t.Fatalf("storageLayout lost in roundtrip: %+v", got.Sections.StorageLayout)
	}
}

// ─── F. apply gating regression ─────────────────────────────────────────

func TestBuildV2_TemplateWithItems_RemainsUnsupportedForV1ApplyGate(t *testing.T) {
	// The v1 apply path keys off TemplateSections.InventoryWorkspace
	// (see app_templates.go::applyTemplateItemsToWorkspace). A
	// Phase 8C export populates the NEW v2 items/layout sections but
	// must NOT spuriously populate the v1 InventoryWorkspace section,
	// because the apply path for items/layout does not exist yet
	// (Phase 8D+). This test guards that surface.
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-w-0", 0x003D9700, "Greatsword", editor.ContainerInventory, 0, 25, 25),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: &TemplateSelection{Items: &SectionSelection{All: true}},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	if tpl.Sections.InventoryWorkspace != nil {
		t.Error("v2 items export must NOT populate sections.inventory.workspace (v1 apply gate)")
	}
	if tpl.Selection.InventoryWorkspace.HasAny() {
		t.Error("v2 items export must NOT auto-select inventory.workspace")
	}
}

// ─── G. library save/load preserves new sections ────────────────────────

func TestBuildV2_LibrarySaveLoad_PreservesItemsAndLayout(t *testing.T) {
	src := &ItemsLayoutSource{
		InventoryItems: []editor.EditableItem{
			editableWeapon("uid-w-0", 0x003D9700, "Greatsword", editor.ContainerInventory, 0, 25, 25),
			editableTalisman("uid-t-0", 0x200003E8, "Talisman", editor.ContainerInventory, 1),
		},
	}
	tpl, err := BuildV2Template(ExportV2Options{
		Now: fixedNow(), ItemsSource: src,
		Selection: &TemplateSelection{
			Items:           &SectionSelection{All: true},
			InventoryLayout: &SectionSelection{All: true},
		},
	})
	if err != nil {
		t.Fatalf("BuildV2Template: %v", err)
	}
	dir := t.TempDir()
	lib := NewTemplateLibrary(dir)
	entry, err := lib.SaveTemplate(tpl)
	if err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	loaded, err := lib.LoadTemplate(entry.ID)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if loaded.Sections.Items == nil || len(loaded.Sections.Items.Entries) != 2 {
		t.Fatalf("items lost in library round-trip: %+v", loaded.Sections.Items)
	}
	if loaded.Sections.InventoryLayout == nil || len(loaded.Sections.InventoryLayout.Entries) != 2 {
		t.Fatalf("inventoryLayout lost in library round-trip: %+v", loaded.Sections.InventoryLayout)
	}
}
