package main

import (
	"encoding/json"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// Phase 8C.1 — App-layer wiring of the v2 items / inventoryLayout /
// storageLayout sources. Tests reuse inventoryOrderFixture because it is
// the only fixture that ships a slot.Data layout editor.BuildSnapshot can
// read; that gives us real EditableItem rows the v2 builder can fold into
// a TemplateItemEntryV2 list.

func TestExportBuildTemplateV2_ItemsOnly_PopulatesItemsSection(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	out, err := app.ExportBuildTemplateV2JSONFromCharacter(0, `{"items":true}`, BuildTemplateV2ExportOptions{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var tpl templates.BuildTemplate
	if err := json.Unmarshal([]byte(out), &tpl); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, out)
	}
	if tpl.Version != 2 {
		t.Fatalf("Version = %d, want 2", tpl.Version)
	}
	if tpl.Sections.Items == nil {
		t.Fatal("sections.items missing — App layer did not pass ItemsSource through")
	}
	if len(tpl.Sections.Items.Entries) != len(testWeapons) {
		t.Errorf("items.entries count = %d, want %d", len(tpl.Sections.Items.Entries), len(testWeapons))
	}
	if tpl.Sections.InventoryLayout != nil {
		t.Errorf("inventoryLayout must stay nil when selection.inventoryLayout is not set")
	}
	if tpl.Sections.StorageLayout != nil {
		t.Errorf("storageLayout must stay nil when selection.storageLayout is not set")
	}
	if !tpl.Selection.Items.HasAny() {
		t.Errorf("selection.items must survive into the exported document")
	}
}

func TestExportBuildTemplateV2_ItemsAndInventoryLayout_PopulatesBoth(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	out, err := app.ExportBuildTemplateV2JSONFromCharacter(0,
		`{"items":true,"inventoryLayout":true,"storageLayout":true}`,
		BuildTemplateV2ExportOptions{},
	)
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	var tpl templates.BuildTemplate
	if err := json.Unmarshal([]byte(out), &tpl); err != nil {
		t.Fatalf("decode JSON: %v\n%s", err, out)
	}
	if tpl.Sections.Items == nil || len(tpl.Sections.Items.Entries) == 0 {
		t.Fatalf("sections.items missing or empty")
	}
	if tpl.Sections.InventoryLayout == nil {
		t.Fatal("sections.inventoryLayout missing")
	}
	if len(tpl.Sections.InventoryLayout.Entries) != len(testWeapons) {
		t.Errorf("inventoryLayout entries = %d, want %d", len(tpl.Sections.InventoryLayout.Entries), len(testWeapons))
	}
	if tpl.Sections.StorageLayout == nil {
		t.Fatal("sections.storageLayout missing — empty container is allowed but the section must be emitted when selected")
	}
	// Every layout entryRef must point at an existing items.entries.entryID.
	known := map[string]bool{}
	for _, e := range tpl.Sections.Items.Entries {
		known[e.EntryID] = true
	}
	for _, l := range tpl.Sections.InventoryLayout.Entries {
		if !known[l.EntryRef] {
			t.Errorf("inventoryLayout.entryRef %q has no matching items.entries entry", l.EntryRef)
		}
	}
}

func TestExportBuildTemplateV2_LayoutWithoutItems_Rejected(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	_, err := app.ExportBuildTemplateV2JSONFromCharacter(0,
		`{"inventoryLayout":true}`,
		BuildTemplateV2ExportOptions{},
	)
	if err == nil {
		t.Fatal("expected error: selection.inventoryLayout without selection.items must be rejected")
	}
	if !strings.Contains(err.Error(), "selection.items") {
		t.Errorf("error must mention the missing items selection: %v", err)
	}
}

func TestPreviewBuildTemplateV2_ItemsCountsInSummary(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	preview, err := app.PreviewBuildTemplateV2FromCharacter(0,
		`{"items":true,"inventoryLayout":true,"storageLayout":true}`,
		BuildTemplateV2ExportOptions{},
	)
	if err != nil {
		t.Fatalf("Preview: %v", err)
	}
	if preview.Report.Summary.ItemsEntries != len(testWeapons) {
		t.Errorf("Summary.ItemsEntries = %d, want %d", preview.Report.Summary.ItemsEntries, len(testWeapons))
	}
	if preview.Report.Summary.InventoryLayoutCount != len(testWeapons) {
		t.Errorf("Summary.InventoryLayoutCount = %d, want %d", preview.Report.Summary.InventoryLayoutCount, len(testWeapons))
	}
	if preview.Report.Summary.StorageLayoutCount != 0 {
		t.Errorf("Summary.StorageLayoutCount = %d, want 0 (fixture has no storage)", preview.Report.Summary.StorageLayoutCount)
	}
	wantSections := map[string]bool{"items": false, "inventoryLayout": false, "storageLayout": false}
	for _, s := range preview.Report.Summary.SelectedSections {
		if _, ok := wantSections[s]; ok {
			wantSections[s] = true
		}
	}
	for s, ok := range wantSections {
		if !ok {
			t.Errorf("Summary.SelectedSections missing %q", s)
		}
	}
}

func TestSaveBuildTemplateV2_ItemsLayoutPreservedInLibrary(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	t.Setenv("HOME", t.TempDir())

	entry, err := app.SaveBuildTemplateV2FromCharacterToLibrary(0,
		`{"items":true,"inventoryLayout":true,"storageLayout":true}`,
		BuildTemplateV2ExportOptions{Name: "Phase 8C.1"},
	)
	if err != nil {
		t.Fatalf("Save: %v", err)
	}
	want := map[string]bool{"items": false, "inventoryLayout": false, "storageLayout": false}
	for _, s := range entry.SelectedSections {
		if _, ok := want[s]; ok {
			want[s] = true
		}
	}
	for s, ok := range want {
		if !ok {
			t.Errorf("LibraryTemplateEntry.SelectedSections missing %q (entry=%+v)", s, entry.SelectedSections)
		}
	}

	lib, err := app.ensureTemplateLibrary()
	if err != nil {
		t.Fatalf("ensureTemplateLibrary: %v", err)
	}
	loaded, err := lib.LoadTemplate(entry.ID)
	if err != nil {
		t.Fatalf("LoadTemplate: %v", err)
	}
	if loaded.Sections.Items == nil || len(loaded.Sections.Items.Entries) == 0 {
		t.Errorf("loaded template lost sections.items")
	}
	if loaded.Sections.InventoryLayout == nil || len(loaded.Sections.InventoryLayout.Entries) == 0 {
		t.Errorf("loaded template lost sections.inventoryLayout")
	}
}

// Apply gating stays in place: a template that selects items (and nothing
// else applyable) is rejected by ApplyBuildTemplateV2ToCharacterJSON
// because items/layout apply is still unsupported in this phase. The
// frontend modal mirrors this via V2_APPLY_SUPPORTED_SECTIONS.
func TestApplyBuildTemplateV2_ItemsOnlySelection_StillUnsupported(t *testing.T) {
	app := inventoryOrderFixture(testWeapons)
	app.save.ActiveSlots[0] = true
	jsonText, err := app.ExportBuildTemplateV2JSONFromCharacter(0, `{"items":true}`, BuildTemplateV2ExportOptions{})
	if err != nil {
		t.Fatalf("Export: %v", err)
	}
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for items-only selection — apply for items is not implemented yet")
	}
	if len(res.Preview.Errors) == 0 {
		t.Fatal("expected at least one error explaining that items-only apply is unsupported")
	}
}
