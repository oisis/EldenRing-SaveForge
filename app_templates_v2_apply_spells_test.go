package main

import (
	"encoding/binary"
	"encoding/json"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// spellsApplyFixture extends applyV2Fixture with a calibrated slot.Data
// buffer so the Phase 7d.3 apply path can dispatch through
// SaveSlot.WriteSpells without tripping the EquippedSpellsOffset /
// hash-block bounds checks. We use the MagicPattern anchor + the
// exported CalculateDynamicOffsets seam so the dynamic offset chain
// (including spellsOff) matches what ComputeSlotHash recomputes.
//
// Without the calibrated buffer, WriteSpells errors out with
// "EquippedSpellsOffset not initialised" and the apply rolls back —
// which would mask the actual semantics we want to test.
func spellsApplyFixture(t *testing.T) *App {
	t.Helper()
	app := applyV2Fixture()
	slot := &app.save.Slots[0]
	slot.Data = make([]byte, core.SlotSize)
	copy(slot.Data[core.FallbackMagicBase:], core.MagicPattern)
	slot.MagicOffset = core.FallbackMagicBase
	if err := slot.CalculateDynamicOffsets(); err != nil {
		t.Fatalf("calibrate fixture: %v", err)
	}
	// Seed every spell slot to the empty-slot sentinel so per-test
	// assertions start from a known-good baseline rather than from
	// whatever residue the buffer happened to ship with.
	for i := 0; i < core.EquippedSpellSlotCount; i++ {
		off := slot.EquippedSpellsOffset + i*core.EquippedSpellSlotSize
		binary.LittleEndian.PutUint32(slot.Data[off:], core.EquippedSpellEmptySentinel)
		binary.LittleEndian.PutUint32(slot.Data[off+4:], 0x00000000)
	}
	return app
}

// readSpellSlotBytes pulls the (spellID, follower) pair stored at the
// 0-indexed save slot. Used by the assertions below.
func readSpellSlotBytes(t *testing.T, app *App, slotIdx int) (uint32, uint32) {
	t.Helper()
	slot := &app.save.Slots[0]
	off := slot.EquippedSpellsOffset + slotIdx*core.EquippedSpellSlotSize
	return binary.LittleEndian.Uint32(slot.Data[off:]),
		binary.LittleEndian.Uint32(slot.Data[off+4:])
}

// readHash10 pulls hash entry [10] from the in-save hash block.
func readHash10(t *testing.T, app *App) uint32 {
	t.Helper()
	slot := &app.save.Slots[0]
	return binary.LittleEndian.Uint32(slot.Data[core.HashOffset+10*4:])
}

// spellsTemplateJSON marshals a hand-built v2 template (selection +
// sections, no extra noise) into the canonical JSON the apply endpoint
// consumes. Kept inline because makeV2Template requires a full Export
// round-trip and spells aren't yet wired into the export path for
// arbitrary fixtures (the export source comes from raw save bytes).
func spellsTemplateJSON(t *testing.T, sel *templates.SectionSelection, sec *templates.SpellsSection) string {
	t.Helper()
	tpl := &templates.BuildTemplate{
		Schema:    templates.SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-02T00:00:00Z",
		Selection: &templates.TemplateSelection{Spells: sel},
		Sections:  templates.TemplateSections{Spells: sec},
	}
	out, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("marshal template: %v", err)
	}
	return string(out)
}

// ─── happy path ────────────────────────────────────────────────────────

func TestApplyV2_Spells_OccupiedSlot_WritesRawIDAndUpdatesHash10(t *testing.T) {
	app := spellsApplyFixture(t)
	jsonText := spellsTemplateJSON(t,
		&templates.SectionSelection{Fields: map[string]bool{"spell1": true}},
		&templates.SpellsSection{
			Spell1: &templates.SpellSlotRef{BaseItemID: 0x40001770, Name: "Catch Flame"},
		},
	)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v warnings=%+v", res.Preview.Errors, res.Preview.Warnings)
	}
	if res.SpellSlotsApplied != 1 {
		t.Errorf("SpellSlotsApplied = %d, want 1", res.SpellSlotsApplied)
	}
	foundSpells := false
	for _, k := range res.AppliedFields {
		if k == "spells" {
			foundSpells = true
		}
	}
	if !foundSpells {
		t.Errorf("AppliedFields = %v, want it to include 'spells'", res.AppliedFields)
	}

	// Raw MagicParam ID landed in the save (prefix stripped to 0x1770).
	gotID, gotFollower := readSpellSlotBytes(t, app, 0)
	if gotID != 0x00001770 {
		t.Errorf("slot 0 spell_id = 0x%08X, want 0x00001770 (raw MagicParam)", gotID)
	}
	if gotFollower != core.EquippedSpellOccupiedFollower {
		t.Errorf("slot 0 follower = 0x%08X, want 0x%08X", gotFollower, core.EquippedSpellOccupiedFollower)
	}

	// hash[10] in slot.Data matches ComputeSlotHash[10] — proves the
	// apply path called WriteSpells (which recomputes hash[10]).
	gotHash := readHash10(t, app)
	full := core.ComputeSlotHash(&app.save.Slots[0])
	wantHash := binary.LittleEndian.Uint32(full[10*4 : 10*4+4])
	if gotHash != wantHash {
		t.Errorf("hash[10] = 0x%08X, want 0x%08X (apply did not recompute)", gotHash, wantHash)
	}
}

func TestApplyV2_Spells_ExplicitClear_WritesEmptySentinel(t *testing.T) {
	app := spellsApplyFixture(t)
	slot := &app.save.Slots[0]
	// Pre-seed slot 3 with an occupied spell so the clear actually
	// flips state (rather than being a no-op).
	off := slot.EquippedSpellsOffset + 3*core.EquippedSpellSlotSize
	binary.LittleEndian.PutUint32(slot.Data[off:], 0x00001234)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], core.EquippedSpellOccupiedFollower)

	jsonText := spellsTemplateJSON(t,
		&templates.SectionSelection{Fields: map[string]bool{"spell4": true}},
		&templates.SpellsSection{
			Spell4: &templates.SpellSlotRef{BaseItemID: 0, Name: "explicit clear"},
		},
	)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false, errors=%+v", res.Preview.Errors)
	}
	if res.SpellSlotsApplied != 1 {
		t.Errorf("SpellSlotsApplied = %d, want 1", res.SpellSlotsApplied)
	}

	gotID, gotFollower := readSpellSlotBytes(t, app, 3)
	if gotID != core.EquippedSpellEmptySentinel {
		t.Errorf("slot 3 spell_id = 0x%08X, want 0xFFFFFFFF (empty sentinel)", gotID)
	}
	if gotFollower != 0 {
		t.Errorf("slot 3 follower = 0x%08X, want 0 (empty sentinel)", gotFollower)
	}
}

func TestApplyV2_Spells_OmittedSlot_LeavesLiveSlotUnchanged(t *testing.T) {
	app := spellsApplyFixture(t)
	slot := &app.save.Slots[0]
	// Pre-seed slot 7 with a recognizable occupied spell. Template
	// selects "spell1" only, so slot 7 must NOT change.
	off := slot.EquippedSpellsOffset + 7*core.EquippedSpellSlotSize
	binary.LittleEndian.PutUint32(slot.Data[off:], 0x000011A1)
	binary.LittleEndian.PutUint32(slot.Data[off+4:], core.EquippedSpellOccupiedFollower)

	jsonText := spellsTemplateJSON(t,
		&templates.SectionSelection{Fields: map[string]bool{"spell1": true}},
		&templates.SpellsSection{
			Spell1: &templates.SpellSlotRef{BaseItemID: 0x40001770},
			// spell8 (index 7) intentionally omitted.
		},
	)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}

	gotID, gotFollower := readSpellSlotBytes(t, app, 7)
	if gotID != 0x000011A1 || gotFollower != core.EquippedSpellOccupiedFollower {
		t.Errorf("slot 7 = (0x%08X, 0x%08X), want pre-apply state (0x000011A1, 0x%08X)",
			gotID, gotFollower, core.EquippedSpellOccupiedFollower)
	}
}

// ─── per-field vs full loadout ─────────────────────────────────────────

func TestApplyV2_Spells_PerFieldSelection_OnlyTargetsListed(t *testing.T) {
	app := spellsApplyFixture(t)
	jsonText := spellsTemplateJSON(t,
		&templates.SectionSelection{Fields: map[string]bool{"spell1": true, "spell14": true}},
		&templates.SpellsSection{
			Spell1:  &templates.SpellSlotRef{BaseItemID: 0x40001770},
			Spell14: &templates.SpellSlotRef{BaseItemID: 0x40000FA0},
		},
	)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.SpellSlotsApplied != 2 {
		t.Errorf("SpellSlotsApplied = %d, want 2", res.SpellSlotsApplied)
	}

	if id, _ := readSpellSlotBytes(t, app, 0); id != 0x00001770 {
		t.Errorf("slot 0 = 0x%08X, want 0x00001770", id)
	}
	if id, _ := readSpellSlotBytes(t, app, 13); id != 0x00000FA0 {
		t.Errorf("slot 13 = 0x%08X, want 0x00000FA0", id)
	}
	// Untouched slot stays at empty sentinel.
	if id, _ := readSpellSlotBytes(t, app, 5); id != core.EquippedSpellEmptySentinel {
		t.Errorf("slot 5 mutated: 0x%08X (expected to remain empty sentinel)", id)
	}
}

func TestApplyV2_Spells_AllSelection_FullLoadout(t *testing.T) {
	app := spellsApplyFixture(t)
	sec := &templates.SpellsSection{
		Spell1:  &templates.SpellSlotRef{BaseItemID: 0x40001770},
		Spell2:  &templates.SpellSlotRef{BaseItemID: 0x40000FA0},
		Spell14: &templates.SpellSlotRef{BaseItemID: 0}, // explicit clear
	}
	jsonText := spellsTemplateJSON(t, &templates.SectionSelection{All: true}, sec)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	// All:true exposes every slot to selection, but only non-nil refs
	// produce SpellWrite entries — sec has 3 set fields.
	if res.SpellSlotsApplied != 3 {
		t.Errorf("SpellSlotsApplied = %d, want 3 (3 non-nil refs in section)", res.SpellSlotsApplied)
	}
}

// ─── rejection / warning paths ─────────────────────────────────────────

func TestApplyV2_Spells_UnknownBaseItemID_WarnsAndSkips(t *testing.T) {
	app := spellsApplyFixture(t)
	// 0x40FFFFFF has the right structural prefix but no entry in the
	// sorcery / incantation DBs → resolver downgrades to a warning and
	// skips the slot. The apply should still succeed for any other
	// valid spells in the same batch.
	jsonText := spellsTemplateJSON(t,
		&templates.SectionSelection{Fields: map[string]bool{"spell1": true, "spell2": true}},
		&templates.SpellsSection{
			Spell1: &templates.SpellSlotRef{BaseItemID: 0x40FFFFFF},
			Spell2: &templates.SpellSlotRef{BaseItemID: 0x40001770},
		},
	)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	// Only spell2 (Catch Flame) was actually written.
	if res.SpellSlotsApplied != 1 {
		t.Errorf("SpellSlotsApplied = %d, want 1 (only the known spell)", res.SpellSlotsApplied)
	}

	// A warning fired for the unknown ID.
	foundWarn := false
	for _, w := range res.Preview.Warnings {
		if strings.Contains(w.Message, "0x40FFFFFF") && strings.Contains(w.Message, "spells.spell1") {
			foundWarn = true
		}
	}
	if !foundWarn {
		t.Errorf("expected a warning citing spells.spell1 / 0x40FFFFFF; got: %+v", res.Preview.Warnings)
	}

	// Slot 0 remained at empty sentinel; slot 1 got Catch Flame.
	if id, _ := readSpellSlotBytes(t, app, 0); id != core.EquippedSpellEmptySentinel {
		t.Errorf("slot 0 should remain untouched, got 0x%08X", id)
	}
	if id, _ := readSpellSlotBytes(t, app, 1); id != 0x00001770 {
		t.Errorf("slot 1 = 0x%08X, want 0x00001770", id)
	}
}

// ─── cross-section coexistence ─────────────────────────────────────────

func TestApplyV2_Spells_CoexistsWithProfile_NoCrossSectionRegression(t *testing.T) {
	app := spellsApplyFixture(t)
	prePlayer := snapPlayer(app.save.Slots[0].Player)

	// Hand-craft a template carrying both profile + spells so we
	// exercise the dispatch ordering (VM flush → equipment → spells).
	tpl := &templates.BuildTemplate{
		Schema:    templates.SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-02T00:00:00Z",
		Selection: &templates.TemplateSelection{
			Profile: &templates.SectionSelection{Fields: map[string]bool{"level": true}},
			Spells:  &templates.SectionSelection{Fields: map[string]bool{"spell1": true}},
		},
		Sections: templates.TemplateSections{
			Profile: &templates.ProfileSection{Level: u32(99)},
			Spells: &templates.SpellsSection{
				Spell1: &templates.SpellSlotRef{BaseItemID: 0x40001770},
			},
		},
	}
	raw, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, string(raw), ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}

	// Profile level took effect.
	if app.save.Slots[0].Player.Level != 99 {
		t.Errorf("Level = %d, want 99", app.save.Slots[0].Player.Level)
	}
	// Spell landed in save.
	if id, _ := readSpellSlotBytes(t, app, 0); id != 0x00001770 {
		t.Errorf("slot 0 = 0x%08X, want 0x00001770", id)
	}
	// Unrelated profile fields untouched (Class/Souls/CharacterName).
	post := app.save.Slots[0].Player
	if post.Class != prePlayer.Class || post.Souls != prePlayer.Souls || post.CharacterName != prePlayer.CharacterName {
		t.Errorf("non-selected profile fields mutated: pre=%+v post=%+v", prePlayer, post)
	}
}

// ─── selection-but-no-section is rejected before reaching the writer ────

func TestApplyV2_Spells_SelectedButSectionMissing_Rejected(t *testing.T) {
	app := spellsApplyFixture(t)
	tpl := &templates.BuildTemplate{
		Schema:    templates.SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-02T00:00:00Z",
		Selection: &templates.TemplateSelection{Spells: &templates.SectionSelection{All: true}},
		// sections.spells deliberately missing — Phase 7d.1 validator
		// must catch this before any byte mutation.
	}
	raw, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, string(raw), ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("expected Applied=false for selection-without-section payload")
	}
	foundErr := false
	for _, e := range res.Preview.Errors {
		if strings.Contains(e.Message, "selection.spells is selected but sections.spells is missing") {
			foundErr = true
		}
	}
	if !foundErr {
		t.Errorf("expected the Phase 7d.1 missing-section error; got: %+v", res.Preview.Errors)
	}
}
