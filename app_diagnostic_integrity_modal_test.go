package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// dirtyIntegrityFixture builds an App whose slot 0 carries `dupItems` common
// inventory rows that all share one acquisition Index, so the read-only
// integrity scan reports the slot as dirty. It stamps a character name so the
// privacy assertions can prove it never reaches the journal.
func dirtyIntegrityFixture(t *testing.T, dupItems int) *App {
	t.Helper()
	common := make([]core.InventoryItem, dupItems)
	for i := range common {
		common[i] = core.InventoryItem{
			GaItemHandle: 0xB0000100 + uint32(i),
			Quantity:     1,
			Index:        552, // shared → duplicate acquisition index
		}
	}
	app := withJournal(repairFixture(common, nil))
	name := "SecretHero"
	for i, r := range name {
		app.save.Slots[0].Player.CharacterName[i] = uint16(r)
	}
	return app
}

func TestRecordDiagnosticIntegrityModalShown_DirtyLogsSafeEvent(t *testing.T) {
	app := dirtyIntegrityFixture(t, 3)

	app.RecordDiagnosticIntegrityModalShown()

	rec := operationEvent(t, app.journal.Tail(), "inventory_integrity_modal_shown")
	if got := rec.Level; got != levelInfo {
		t.Errorf("level = %q, want info", got)
	}
	if got := operationField(rec, "affected_slots"); got != "1" {
		t.Errorf("affected_slots = %q, want 1", got)
	}
	if got := operationField(rec, "duplicate_inventory_entries"); got != "2" {
		t.Errorf("duplicate_inventory_entries = %q, want 2", got)
	}
	if got := operationField(rec, "conflicting_indices"); got != "1" {
		t.Errorf("conflicting_indices = %q, want 1", got)
	}
	items := operationField(rec, "affected_items")
	if !strings.Contains(items, "duplicate_acquisition_index") {
		t.Errorf("affected_items = %q, want a conflict kind", items)
	}
	if got := operationField(rec, "additional_items"); got != "0" {
		t.Errorf("additional_items = %q, want 0", got)
	}

	// Privacy: only the approved safe keys, and no character name, handle hex,
	// or item ID may appear anywhere in the record.
	allowed := map[string]bool{
		"affected_slots":              true,
		"duplicate_inventory_entries": true,
		"conflicting_indices":         true,
		"affected_items":              true,
		"additional_items":            true,
	}
	for _, f := range rec.Fields {
		if !allowed[f.Key] {
			t.Errorf("modal_shown leaked unapproved field %q=%q", f.Key, f.Value)
		}
		if strings.Contains(f.Value, "SecretHero") || strings.Contains(strings.ToLower(f.Value), "0x") ||
			strings.Contains(strings.ToLower(f.Value), "b0000") {
			t.Errorf("modal_shown field %q=%q leaked private data", f.Key, f.Value)
		}
	}
}

func TestRecordDiagnosticIntegrityModalShown_ItemsBounded(t *testing.T) {
	// 22 rows share one index → 21 duplicate issues, 22 conflict items. The
	// summary is capped at 20 with the remainder reported as additional_items.
	app := dirtyIntegrityFixture(t, 22)

	app.RecordDiagnosticIntegrityModalShown()

	rec := operationEvent(t, app.journal.Tail(), "inventory_integrity_modal_shown")
	items := operationField(rec, "affected_items")
	if got := strings.Count(items, ";") + 1; got != diagnosticIntegrityModalItemsMax {
		t.Errorf("affected_items entries = %d, want %d", got, diagnosticIntegrityModalItemsMax)
	}
	if got := operationField(rec, "additional_items"); got != "2" {
		t.Errorf("additional_items = %q, want 2", got)
	}
}

func TestRecordDiagnosticIntegrityModalShown_CleanNoEvent(t *testing.T) {
	app := withJournal(repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB0000001, Quantity: 1, Index: 100},
			{GaItemHandle: 0xB0000002, Quantity: 1, Index: 101},
		},
		nil,
	))

	app.RecordDiagnosticIntegrityModalShown()

	for _, rec := range app.journal.Tail() {
		if rec.Event == "inventory_integrity_modal_shown" {
			t.Fatal("clean save must not log the modal_shown event")
		}
	}
}

func TestRecordDiagnosticIntegrityModalShown_NoSaveNoEvent(t *testing.T) {
	app := withJournal(NewApp())

	app.RecordDiagnosticIntegrityModalShown()

	if len(app.journal.Tail()) != 0 {
		t.Fatalf("no-save modal_shown must log nothing, got %d records", len(app.journal.Tail()))
	}
}

func TestRecordDiagnosticIntegrityModalRepairOutcome(t *testing.T) {
	cases := []struct {
		outcome   string
		wantEvent bool
		wantLevel diagnosticLevel
	}{
		{"resolved", true, levelInfo},
		{"unresolved", true, levelInfo},
		{"error", true, levelError},
		{"", false, ""},
		{"bogus", false, ""},
		{"RESOLVED", false, ""},
	}
	for _, tc := range cases {
		t.Run(tc.outcome, func(t *testing.T) {
			app := withJournal(NewApp())
			app.RecordDiagnosticIntegrityModalRepairOutcome(tc.outcome)

			var rec *diagnosticRecord
			tail := app.journal.Tail()
			for i := range tail {
				if tail[i].Event == "inventory_integrity_modal_repair_finished" {
					rec = &tail[i]
					break
				}
			}
			if !tc.wantEvent {
				if rec != nil {
					t.Fatalf("outcome %q must not log an event", tc.outcome)
				}
				return
			}
			if rec == nil {
				t.Fatalf("outcome %q expected an event", tc.outcome)
			}
			if rec.Level != tc.wantLevel {
				t.Errorf("level = %q, want %q", rec.Level, tc.wantLevel)
			}
			if got := operationField(*rec, "outcome"); got != tc.outcome {
				t.Errorf("outcome field = %q, want %q", got, tc.outcome)
			}
			for _, f := range rec.Fields {
				if f.Key != "outcome" {
					t.Errorf("repair_finished leaked unapproved field %q=%q", f.Key, f.Value)
				}
			}
		})
	}
}

// TestRecordDiagnosticIntegrityModalShown_ResolvesRealItemName drives the full
// scan → resolve → journal path with two real goods rows (Golden Seed and
// Sacred Tear) sharing one acquisition index. It proves affected_items carries
// the DB-resolved names — not the unknown_item fallback — each tagged with the
// real conflict kind. Handles use the goods prefix so HandleToItemID resolves
// them straight out of the DB (empty GaMap), no fixture-supplied name.
func TestRecordDiagnosticIntegrityModalShown_ResolvesRealItemName(t *testing.T) {
	app := withJournal(repairFixture(
		[]core.InventoryItem{
			{GaItemHandle: 0xB000271A, Quantity: 1, Index: 700}, // Golden Seed → 0x4000271A
			{GaItemHandle: 0xB0002724, Quantity: 1, Index: 700}, // Sacred Tear → 0x40002724, shared index
		},
		nil,
	))

	app.RecordDiagnosticIntegrityModalShown()

	rec := operationEvent(t, app.journal.Tail(), "inventory_integrity_modal_shown")
	items := operationField(rec, "affected_items")
	for _, want := range []string{
		"Golden Seed (duplicate_acquisition_index)",
		"Sacred Tear (duplicate_acquisition_index)",
	} {
		if !strings.Contains(items, want) {
			t.Errorf("affected_items = %q, want it to contain %q", items, want)
		}
	}
	if strings.Contains(items, "unknown_item") {
		t.Errorf("affected_items = %q, must not fall back to unknown_item for DB-resolved goods", items)
	}
}
