package main

import (
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
	"unicode/utf16"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// applyV2Fixture extends profileStatsFixture with the slot-active bit set
// so the Phase 5A scope checks pass. Phase 5A rejects inactive slots; the
// dedicated inactive-slot test resets the flag itself.
func applyV2Fixture() *App {
	app := profileStatsFixture()
	app.save.ActiveSlots[0] = true
	return app
}

// applyV2FixtureWithEventFlags builds the apply-v2 fixture with a slot.Data
// buffer and an EventFlagsOffset positioned so NG+ flag writes land inside
// the buffer. The buffer is intentionally sized just enough to cover the
// NG+ region (8 flags around event flag 50) — there is no MagicOffset
// configuration here, so the SyncPlayerToData writes via SlotAccessor land
// at bounds-checked offsets that silently fail when the buffer is shorter.
// That is intentional: this fixture exercises the NG+ flag pipeline, not
// the full Player → Data flush.
func applyV2FixtureWithEventFlags(t *testing.T) *App {
	t.Helper()
	app := applyV2Fixture()
	slot := &app.save.Slots[0]
	// 4 KiB buffer with EventFlagsOffset at 64 mirrors the SaveCharacter
	// guard `EventFlagsOffset > 0 && < len(slot.Data)`. SetEventFlag(50..57)
	// writes at flags[6] (= absolute byte 70), well inside the buffer.
	slot.Data = make([]byte, 4096)
	slot.EventFlagsOffset = 64
	return app
}

func mustExportV2(t *testing.T, app *App, selectionJSON string, opts BuildTemplateV2ExportOptions) string {
	t.Helper()
	out, err := app.ExportBuildTemplateV2JSONFromCharacter(0, selectionJSON, opts)
	if err != nil {
		t.Fatalf("ExportBuildTemplateV2JSONFromCharacter: %v", err)
	}
	if out == "" {
		t.Fatal("empty export JSON")
	}
	return out
}

// snapshotPlayer captures every Player field Phase 5A may touch so tests
// can assert "unchanged" with a single equality check.
type playerSnapshot struct {
	Level               uint32
	Class               uint8
	Souls               uint32
	SoulMemory          uint32
	Vigor               uint32
	Mind                uint32
	Endurance           uint32
	Strength            uint32
	Dexterity           uint32
	Intelligence        uint32
	Faith               uint32
	Arcane              uint32
	TalismanSlots       uint8
	ClearCount          uint32
	ScadutreeBlessing   uint8
	ShadowRealmBlessing uint8
	CharacterName       [16]uint16
}

func snapPlayer(p core.PlayerGameData) playerSnapshot {
	return playerSnapshot{
		Level:               p.Level,
		Class:               p.Class,
		Souls:               p.Souls,
		SoulMemory:          p.SoulMemory,
		Vigor:               p.Vigor,
		Mind:                p.Mind,
		Endurance:           p.Endurance,
		Strength:            p.Strength,
		Dexterity:           p.Dexterity,
		Intelligence:        p.Intelligence,
		Faith:               p.Faith,
		Arcane:              p.Arcane,
		TalismanSlots:       p.TalismanSlots,
		ClearCount:          p.ClearCount,
		ScadutreeBlessing:   p.ScadutreeBlessing,
		ShadowRealmBlessing: p.ShadowRealmBlessing,
		CharacterName:       p.CharacterName,
	}
}

// makeV2Template builds a canonical v2 JSON document from the fixture
// character — selection (boolean shortcuts or per-field) plus overrides
// applied to the in-memory template before re-marshalling. Tests use this
// helper to construct payloads that exercise specific selection shapes
// without hand-crafting JSON.
func makeV2Template(t *testing.T, app *App, selectionJSON string, override func(*templates.BuildTemplate)) string {
	t.Helper()
	raw := mustExportV2(t, app, selectionJSON, BuildTemplateV2ExportOptions{Name: "test"})
	var tpl templates.BuildTemplate
	if err := json.Unmarshal([]byte(raw), &tpl); err != nil {
		t.Fatalf("decode exported template: %v", err)
	}
	if override != nil {
		override(&tpl)
	}
	out, err := json.Marshal(&tpl)
	if err != nil {
		t.Fatalf("re-marshal template: %v", err)
	}
	return string(out)
}

func u32(v uint32) *uint32 { return &v }
func u8val(v uint8) *uint8 { return &v }

// ─── 1. Profile-only selected fields only ─────────────────────────────

func TestApplyBuildTemplateV2_ProfileOnly_SelectedFieldsOnly(t *testing.T) {
	app := applyV2Fixture()
	pre := snapPlayer(app.save.Slots[0].Player)

	jsonText := makeV2Template(t, app, `{"profile":{"level":true,"runes":true}}`, func(tpl *templates.BuildTemplate) {
		tpl.Sections.Profile.Level = u32(80)
		tpl.Sections.Profile.Runes = u32(50000)
	})

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	if !reflect.DeepEqual(res.AppliedFields, []string{"profile.level", "profile.runes"}) {
		t.Errorf("AppliedFields = %v, want [profile.level profile.runes]", res.AppliedFields)
	}
	if len(res.SkippedFields) != 0 {
		t.Errorf("SkippedFields = %v, want empty", res.SkippedFields)
	}

	post := app.save.Slots[0].Player
	if post.Level != 80 {
		t.Errorf("Level = %d, want 80", post.Level)
	}
	if post.Souls != 50000 {
		t.Errorf("Souls = %d, want 50000", post.Souls)
	}
	// Unrelated profile fields unchanged.
	if post.Class != pre.Class {
		t.Errorf("Class changed: %d → %d", pre.Class, post.Class)
	}
	if post.ClearCount != pre.ClearCount {
		t.Errorf("ClearCount changed: %d → %d", pre.ClearCount, post.ClearCount)
	}
	if post.ScadutreeBlessing != pre.ScadutreeBlessing {
		t.Errorf("ScadutreeBlessing changed")
	}
	if post.TalismanSlots != pre.TalismanSlots {
		t.Errorf("TalismanSlots changed")
	}
	if post.CharacterName != pre.CharacterName {
		t.Errorf("CharacterName changed")
	}
	// Stats untouched.
	if post.Vigor != pre.Vigor || post.Mind != pre.Mind || post.Endurance != pre.Endurance ||
		post.Strength != pre.Strength || post.Dexterity != pre.Dexterity ||
		post.Intelligence != pre.Intelligence || post.Faith != pre.Faith || post.Arcane != pre.Arcane {
		t.Errorf("stats mutated: pre=%+v post=%+v", pre, post)
	}
}

// ─── 2. Stats-only selected fields only ───────────────────────────────

func TestApplyBuildTemplateV2_StatsOnly_SelectedFieldsOnly(t *testing.T) {
	app := applyV2Fixture()
	pre := snapPlayer(app.save.Slots[0].Player)

	jsonText := makeV2Template(t, app, `{"stats":{"vigor":true,"faith":true}}`, func(tpl *templates.BuildTemplate) {
		tpl.Sections.Stats.Vigor = u32(60)
		tpl.Sections.Stats.Faith = u32(50)
	})

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	if !reflect.DeepEqual(res.AppliedFields, []string{"stats.vigor", "stats.faith"}) {
		t.Errorf("AppliedFields = %v, want [stats.vigor stats.faith]", res.AppliedFields)
	}

	post := app.save.Slots[0].Player
	if post.Vigor != 60 || post.Faith != 50 {
		t.Errorf("Vigor=%d Faith=%d, want 60 / 50", post.Vigor, post.Faith)
	}
	if post.Mind != pre.Mind || post.Endurance != pre.Endurance || post.Strength != pre.Strength ||
		post.Dexterity != pre.Dexterity || post.Intelligence != pre.Intelligence || post.Arcane != pre.Arcane {
		t.Errorf("untouched stats mutated: pre=%+v post=%+v", pre, post)
	}
	if post.Level != pre.Level || post.Souls != pre.Souls || post.ClearCount != pre.ClearCount ||
		post.CharacterName != pre.CharacterName {
		t.Errorf("profile fields mutated: pre=%+v post=%+v", pre, post)
	}
}

// ─── 3. Profile + stats combined ──────────────────────────────────────

func TestApplyBuildTemplateV2_ProfileAndStats_Combined(t *testing.T) {
	app := applyV2Fixture()
	jsonText := makeV2Template(t, app,
		`{"profile":{"name":true,"level":true},"stats":{"vigor":true,"mind":true}}`,
		func(tpl *templates.BuildTemplate) {
			n := "RenamedHero"
			tpl.Sections.Profile.Name = &n
			tpl.Sections.Profile.Level = u32(120)
			tpl.Sections.Stats.Vigor = u32(55)
			tpl.Sections.Stats.Mind = u32(40)
		},
	)

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	wantApplied := []string{"profile.name", "profile.level", "stats.vigor", "stats.mind"}
	if !reflect.DeepEqual(res.AppliedFields, wantApplied) {
		t.Errorf("AppliedFields = %v, want %v", res.AppliedFields, wantApplied)
	}
	post := app.save.Slots[0].Player
	if core.UTF16ToString(post.CharacterName[:]) != "RenamedHero" {
		t.Errorf("Name = %q, want RenamedHero", core.UTF16ToString(post.CharacterName[:]))
	}
	if post.Level != 120 || post.Vigor != 55 || post.Mind != 40 {
		t.Errorf("Level=%d Vigor=%d Mind=%d", post.Level, post.Vigor, post.Mind)
	}
}

// ─── 4. Unselected fields unchanged when template carries values ──────

func TestApplyBuildTemplateV2_UnselectedFieldsUnchanged(t *testing.T) {
	app := applyV2Fixture()
	pre := snapPlayer(app.save.Slots[0].Player)

	// Build a template carrying many values but selecting only `level`.
	// Manually construct so the section carries values for fields the
	// selection deliberately omits.
	tpl := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    2,
		AppVersion: "test",
		CreatedAt:  "2026-05-31T12:00:00Z",
		Selection: &templates.TemplateSelection{
			Profile: &templates.SectionSelection{Fields: map[string]bool{"level": true}},
		},
		Sections: templates.TemplateSections{
			Profile: &templates.ProfileSection{
				Level:               u32(77),
				Runes:               u32(99999),
				ClearCount:          u32(5),
				ScadutreeBlessing:   u8val(15), // within [0, 20]
				ShadowRealmBlessing: u8val(8),  // within [0, 10]
				TalismanSlots:       u8val(3),  // within [0, 3]
			},
		},
	}
	if err := templates.ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("fixture template fails validation: %v", err)
	}
	payload, err := json.Marshal(tpl)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}

	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, string(payload), ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	if !reflect.DeepEqual(res.AppliedFields, []string{"profile.level"}) {
		t.Errorf("AppliedFields = %v, want [profile.level]", res.AppliedFields)
	}

	post := app.save.Slots[0].Player
	if post.Level != 77 {
		t.Errorf("Level = %d, want 77", post.Level)
	}
	// Every other field — Runes / ClearCount / Blessings / TalismanSlots —
	// carried a template value but was unselected; must remain at pre-apply
	// values.
	if post.Souls != pre.Souls {
		t.Errorf("Souls changed despite unselected: %d → %d", pre.Souls, post.Souls)
	}
	if post.ClearCount != pre.ClearCount {
		t.Errorf("ClearCount changed despite unselected: %d → %d", pre.ClearCount, post.ClearCount)
	}
	if post.ScadutreeBlessing != pre.ScadutreeBlessing {
		t.Errorf("ScadutreeBlessing changed despite unselected: %d → %d", pre.ScadutreeBlessing, post.ScadutreeBlessing)
	}
	if post.ShadowRealmBlessing != pre.ShadowRealmBlessing {
		t.Errorf("ShadowRealmBlessing changed despite unselected: %d → %d", pre.ShadowRealmBlessing, post.ShadowRealmBlessing)
	}
	if post.TalismanSlots != pre.TalismanSlots {
		t.Errorf("TalismanSlots changed despite unselected: %d → %d", pre.TalismanSlots, post.TalismanSlots)
	}
}

// ─── 5. Reject v1 templates at the v2 endpoint ────────────────────────

func TestApplyBuildTemplateV2_RejectsV1Template(t *testing.T) {
	app := applyV2Fixture()
	// Igon's Furled Finger (0x401EA3C3) is a real key_items entry with
	// MaxUpgrade=0; using a known good item makes the v1 payload pass
	// ValidateBuildTemplate inside ParseBuildTemplateJSON so the test
	// actually reaches the version-gate check inside the v2 endpoint.
	v1 := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    1,
		AppVersion: "test",
		CreatedAt:  "2026-05-31T12:00:00Z",
		Sections: templates.TemplateSections{
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{{
					BaseItemID: 0x401EA3C3,
					Name:       "Igon's Furled Finger",
					Category:   "key_items",
					Quantity:   1,
					Container:  templates.ContainerInventory,
					Position:   0,
				}},
				StorageItems: []templates.TemplateItem{},
			},
		},
	}
	if err := templates.ValidateBuildTemplate(v1); err != nil {
		t.Fatalf("v1 fixture fails validation: %v", err)
	}
	payload, _ := json.Marshal(v1)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, string(payload), ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply (unexpected non-nil error): %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true, want false")
	}
	if len(res.Preview.Errors) == 0 {
		t.Fatal("expected error issue in preview")
	}
	msg := res.Preview.Errors[0].Message
	if !strings.Contains(msg, "schema v2") || !strings.Contains(msg, "v1") {
		t.Errorf("error message %q does not mention v2 vs v1 routing", msg)
	}
}

// ─── 6. Reject inventory.workspace selection in v2 ────────────────────

// TestApplyBuildTemplateV2_InventoryWorkspaceWithoutSessionRejected — Phase 7a
// flips the original Phase 5 hard-reject into a session-gated reject. A v2
// template selecting inventory.workspace WITHOUT an active session ID still
// must not mutate the slot, but it now surfaces IssueCodeInventorySessionRequired
// so the Templates shell can render the "Open the Sort Order workspace first"
// guidance instead of the generic unsupported-category message.
func TestApplyBuildTemplateV2_InventoryWorkspaceWithoutSessionRejected(t *testing.T) {
	app := applyV2Fixture()
	tpl := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    2,
		AppVersion: "test",
		CreatedAt:  "2026-05-31T12:00:00Z",
		Selection: &templates.TemplateSelection{
			Stats:              &templates.SectionSelection{All: true},
			InventoryWorkspace: &templates.SectionSelection{All: true},
		},
		Sections: templates.TemplateSections{
			Stats: &templates.StatsSection{Vigor: u32(40), Mind: u32(30), Endurance: u32(30), Strength: u32(20), Dexterity: u32(20), Intelligence: u32(20), Faith: u32(20), Arcane: u32(20)},
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{},
				StorageItems:   []templates.TemplateItem{},
			},
		},
	}
	if err := templates.ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("fixture fails validation: %v", err)
	}
	pre := snapPlayer(app.save.Slots[0].Player)
	payload, _ := json.Marshal(tpl)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, string(payload), ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for inventory.workspace selection without session — must reject")
	}
	found := false
	for _, issue := range res.Preview.Errors {
		if issue.Code == templates.IssueCodeInventorySessionRequired {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected IssueCodeInventorySessionRequired, got %+v", res.Preview.Errors)
	}
	post := app.save.Slots[0].Player
	if snapPlayer(post) != pre {
		t.Errorf("slot mutated despite rejection")
	}
}

// ─── 7. className is rejected by schema validation ────────────────────

func TestApplyBuildTemplateV2_RejectsClassNameSelectionKey(t *testing.T) {
	app := applyV2Fixture()
	// Hand-crafted selection with the non-canonical key. ParseBuildTemplateJSON
	// → ValidateBuildTemplate must reject `className` because the per-section
	// allowlist contains only `class`.
	payload := `{
		"schema": "saveforge.build-template",
		"version": 2,
		"appVersion": "test",
		"createdAt": "2026-05-31T12:00:00Z",
		"selection": { "profile": { "className": true } },
		"sections": { "profile": { "level": 1 } }
	}`
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, payload, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true; className must be rejected")
	}
	if len(res.Preview.Errors) == 0 {
		t.Fatal("expected schema/structure error for className")
	}
	if !strings.Contains(strings.ToLower(res.Preview.Errors[0].Message), "classname") &&
		!strings.Contains(strings.ToLower(res.Preview.Errors[0].Message), "unknown") &&
		!strings.Contains(strings.ToLower(res.Preview.Errors[0].Message), "profile") {
		t.Errorf("error message %q does not point at className", res.Preview.Errors[0].Message)
	}
}

// ─── 8. profile.class selected → skipped, Class unchanged ─────────────

func TestApplyBuildTemplateV2_ClassSelectedIsSkipped(t *testing.T) {
	app := applyV2Fixture()
	preClass := app.save.Slots[0].Player.Class

	jsonText := makeV2Template(t, app,
		`{"profile":{"class":true,"level":true}}`,
		func(tpl *templates.BuildTemplate) {
			n := "Different"
			tpl.Sections.Profile.Class = &n
			tpl.Sections.Profile.Level = u32(42)
		},
	)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	if !containsString(res.AppliedFields, "profile.level") {
		t.Errorf("profile.level should be applied, AppliedFields=%v", res.AppliedFields)
	}
	if containsString(res.AppliedFields, "profile.class") {
		t.Errorf("profile.class must NOT be in AppliedFields in Phase 5A, got %v", res.AppliedFields)
	}
	if !containsString(res.SkippedFields, "profile.class") {
		t.Errorf("profile.class must be in SkippedFields, got %v", res.SkippedFields)
	}
	if app.save.Slots[0].Player.Class != preClass {
		t.Errorf("Class changed from %d to %d despite skip", preClass, app.save.Slots[0].Player.Class)
	}
	if app.save.Slots[0].Player.Level != 42 {
		t.Errorf("Level not applied: %d", app.save.Slots[0].Player.Level)
	}
}

// ─── 9. Invalid character index ───────────────────────────────────────

func TestApplyBuildTemplateV2_InvalidCharIndex(t *testing.T) {
	app := applyV2Fixture()
	jsonText := makeV2Template(t, app, `{"stats":true}`, nil)

	for _, idx := range []int{-1, 10, 99} {
		res, err := app.ApplyBuildTemplateV2ToCharacterJSON(idx, jsonText, ApplyTemplateV2Options{})
		if err == nil {
			t.Errorf("charIdx=%d: expected error, got Applied=%v", idx, res.Applied)
		}
		if res.Applied {
			t.Errorf("charIdx=%d: Applied=true, want false", idx)
		}
		if res.CharIndex != idx {
			t.Errorf("charIdx=%d: result.CharIndex=%d", idx, res.CharIndex)
		}
	}
}

// ─── 10. No save loaded ───────────────────────────────────────────────

func TestApplyBuildTemplateV2_NoSaveLoaded(t *testing.T) {
	app := NewApp()
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, `{}`, ApplyTemplateV2Options{})
	if err == nil {
		t.Fatalf("expected error, got Applied=%v", res.Applied)
	}
	if !strings.Contains(err.Error(), "no save loaded") {
		t.Errorf("unexpected error: %v", err)
	}
}

// ─── 11. Inactive slot is rejected ────────────────────────────────────

func TestApplyBuildTemplateV2_InactiveSlotRejected(t *testing.T) {
	app := applyV2Fixture()
	app.save.ActiveSlots[0] = false
	jsonText := makeV2Template(t, app, `{"stats":true}`, nil)
	pre := snapPlayer(app.save.Slots[0].Player)

	// makeV2Template uses ExportBuildTemplateV2JSONFromCharacter which
	// reads via GetCharacter; it does not require an active slot. The
	// rejection happens in the apply endpoint after slotMu is taken.
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true on inactive slot")
	}
	if len(res.Preview.Errors) == 0 || !strings.Contains(res.Preview.Errors[0].Message, "inactive") {
		t.Errorf("expected inactive-slot error, got %+v", res.Preview.Errors)
	}
	if snapPlayer(app.save.Slots[0].Player) != pre {
		t.Errorf("slot mutated despite rejection")
	}
}

// ─── 12. TalismanSlots applies at the validator-allowed maximum ───────
//
// ValidateBuildTemplate rejects profile.talismanSlots > 3 at parse time
// (MaxProfileTalismanSlots=3), so an out-of-range value can never reach
// the apply layer through this endpoint. The defensive clamp inside
// ApplyVMToParsedSlot is exercised here at its top boundary: setting
// talismanSlots=3 must land cleanly without being silently coerced.
func TestApplyBuildTemplateV2_TalismanSlotsApplyMax(t *testing.T) {
	app := applyV2Fixture()
	// Pre-state: fixture has TalismanSlots=2.
	jsonText := makeV2Template(t, app, `{"profile":{"talismanSlots":true}}`, func(tpl *templates.BuildTemplate) {
		tpl.Sections.Profile.TalismanSlots = u8val(3)
	})
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	if app.save.Slots[0].Player.TalismanSlots != 3 {
		t.Errorf("TalismanSlots = %d, want 3", app.save.Slots[0].Player.TalismanSlots)
	}
}

// ─── 13. ClearCount applies at max + NG+ flag sync ────────────────────
//
// ValidateBuildTemplate rejects profile.clearCount > 7 at parse time
// (MaxProfileClearCount=7), so this test exercises the max-boundary apply
// and verifies that the NG+ flag sync side-effect fires exactly when
// clearCount lands.
func TestApplyBuildTemplateV2_ClearCountApplyMaxAndFlagsSynced(t *testing.T) {
	app := applyV2FixtureWithEventFlags(t)
	jsonText := makeV2Template(t, app, `{"profile":{"clearCount":true}}`, func(tpl *templates.BuildTemplate) {
		tpl.Sections.Profile.ClearCount = u32(7)
	})
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	got := app.save.Slots[0].Player.ClearCount
	if got != 7 {
		t.Errorf("ClearCount = %d, want 7", got)
	}
	// Flag 50+7=57 must be set; 50..56 must be cleared.
	flags := app.save.Slots[0].Data[app.save.Slots[0].EventFlagsOffset:]
	for i := uint32(0); i <= 7; i++ {
		want := i == 7
		got, err := db.GetEventFlag(flags, 50+i)
		if err != nil {
			t.Fatalf("GetEventFlag(%d): %v", 50+i, err)
		}
		if got != want {
			t.Errorf("flag %d = %v, want %v", 50+i, got, want)
		}
	}
}

// ─── 13b. ClearCount NOT touched when unselected ──────────────────────

func TestApplyBuildTemplateV2_ClearCountNotSelected_NoFlagSync(t *testing.T) {
	app := applyV2FixtureWithEventFlags(t)
	// Pre-set flag 53 to true so we can verify the apply did NOT touch it.
	flags := app.save.Slots[0].Data[app.save.Slots[0].EventFlagsOffset:]
	if err := db.SetEventFlag(flags, 53, true); err != nil {
		t.Fatalf("seed flag: %v", err)
	}
	jsonText := makeV2Template(t, app, `{"profile":{"level":true}}`, func(tpl *templates.BuildTemplate) {
		tpl.Sections.Profile.Level = u32(40)
	})
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	flags = app.save.Slots[0].Data[app.save.Slots[0].EventFlagsOffset:]
	got, err := db.GetEventFlag(flags, 53)
	if err != nil {
		t.Fatalf("GetEventFlag(53): %v", err)
	}
	if !got {
		t.Error("flag 53 was cleared even though clearCount was not in selection")
	}
}

// ─── 14. SoulMemory bump-by-level (existing ApplyVMToParsedSlot rule) ─

func TestApplyBuildTemplateV2_SoulMemoryBumpedByRunesCost(t *testing.T) {
	app := applyV2Fixture()
	// Drive Level up but leave SoulMemory deliberately low; the
	// runesCostForLevel clamp inside ApplyVMToParsedSlot must push it up.
	jsonText := makeV2Template(t, app, `{"profile":{"level":true,"soulMemory":true}}`, func(tpl *templates.BuildTemplate) {
		tpl.Sections.Profile.Level = u32(150)
		tpl.Sections.Profile.SoulMemory = u32(0)
	})
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	if app.save.Slots[0].Player.SoulMemory == 0 {
		t.Error("SoulMemory not bumped by runesCostForLevel (still 0)")
	}
}

// ─── 15a. ProfileSummary updated for name/level ───────────────────────

func TestApplyBuildTemplateV2_ProfileSummaryUpdatedForName(t *testing.T) {
	app := applyV2Fixture()
	jsonText := makeV2Template(t, app, `{"profile":{"name":true,"level":true}}`, func(tpl *templates.BuildTemplate) {
		n := "MenuName"
		tpl.Sections.Profile.Name = &n
		tpl.Sections.Profile.Level = u32(88)
	})
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	if app.save.ProfileSummaries[0].Level != 88 {
		t.Errorf("ProfileSummaries[0].Level = %d, want 88", app.save.ProfileSummaries[0].Level)
	}
	wantName := utf16.Encode([]rune("MenuName"))
	for i, c := range wantName {
		if app.save.ProfileSummaries[0].CharacterName[i] != c {
			t.Errorf("ProfileSummaries[0].CharacterName[%d] = %d, want %d", i, app.save.ProfileSummaries[0].CharacterName[i], c)
		}
	}
}

// ─── 15b. Stats-only apply does NOT touch ProfileSummary ──────────────

func TestApplyBuildTemplateV2_StatsOnly_DoesNotTouchProfileSummary(t *testing.T) {
	app := applyV2Fixture()
	// Seed a recognisable summary state so we can confirm nothing
	// overwrites it during a stats apply.
	app.save.ProfileSummaries[0].Level = 999
	app.save.ProfileSummaries[0].CharacterName[0] = 0x5A5A
	jsonText := makeV2Template(t, app, `{"stats":{"vigor":true}}`, func(tpl *templates.BuildTemplate) {
		tpl.Sections.Stats.Vigor = u32(50)
	})
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	if app.save.ProfileSummaries[0].Level != 999 {
		t.Errorf("ProfileSummaries[0].Level mutated by stats-only apply: %d", app.save.ProfileSummaries[0].Level)
	}
	if app.save.ProfileSummaries[0].CharacterName[0] != 0x5A5A {
		t.Errorf("ProfileSummaries[0].CharacterName mutated by stats-only apply")
	}
}

// ─── 16. No inventory mutation ────────────────────────────────────────

func TestApplyBuildTemplateV2_NoInventoryMutation(t *testing.T) {
	app := applyV2Fixture()
	preInv := app.save.Slots[0].Inventory.Clone()
	preStg := app.save.Slots[0].Storage.Clone()
	jsonText := makeV2Template(t, app, `{"profile":{"level":true},"stats":{"vigor":true}}`, func(tpl *templates.BuildTemplate) {
		tpl.Sections.Profile.Level = u32(72)
		tpl.Sections.Stats.Vigor = u32(50)
	})
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	if !reflect.DeepEqual(app.save.Slots[0].Inventory, preInv) {
		t.Errorf("Inventory mutated by Phase 5A apply")
	}
	if !reflect.DeepEqual(app.save.Slots[0].Storage, preStg) {
		t.Errorf("Storage mutated by Phase 5A apply")
	}
}

// ─── 17. No memory-stones mutation ────────────────────────────────────

func TestApplyBuildTemplateV2_NoMemoryStonesMutation(t *testing.T) {
	app := applyV2Fixture()
	// ApplyVMToParsedSlot always runs updateItemsAndSync against
	// slot.Inventory.CommonItems and writes each item's quantity back
	// into slot.Data at MagicOffset + 505 + i*12 + 4. With one item at
	// index 0 and MagicOffset=0, the highest write offset is 513 — so
	// allocate a 1 KiB buffer to keep the bounds check happy.
	slot := &app.save.Slots[0]
	slot.Data = make([]byte, 1024)
	// Stage a memory-stones inventory entry (handle 0xB000272E is the
	// memory-stones marker recognised by MapParsedSlotToVM). Apply must
	// NOT touch its quantity even though SaveCharacter would, because
	// Phase 5A intentionally skips applyMemoryStonesToSlot.
	slot.Inventory.CommonItems = []core.InventoryItem{
		{GaItemHandle: 0xB000272E, Quantity: 4},
	}
	preQty := slot.Inventory.CommonItems[0].Quantity
	jsonText := makeV2Template(t, app, `{"profile":{"level":true}}`, func(tpl *templates.BuildTemplate) {
		tpl.Sections.Profile.Level = u32(99)
	})
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	got := slot.Inventory.CommonItems[0].Quantity
	if got != preQty {
		t.Errorf("MemoryStones quantity changed: %d → %d (apply must not call applyMemoryStonesToSlot)", preQty, got)
	}
}

// ─── 18. Undo pushed before mutation ──────────────────────────────────

func TestApplyBuildTemplateV2_UndoPushedBeforeMutation(t *testing.T) {
	app := applyV2Fixture()
	preStackDepth := len(app.undoStacks[0])
	jsonText := makeV2Template(t, app, `{"profile":{"level":true}}`, func(tpl *templates.BuildTemplate) {
		tpl.Sections.Profile.Level = u32(60)
	})
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false")
	}
	if got := len(app.undoStacks[0]); got != preStackDepth+1 {
		t.Errorf("undo stack depth = %d, want %d (pushUndoLocked not called)", got, preStackDepth+1)
	}
}

// ─── 19. Existing v1 apply path unchanged ─────────────────────────────

func TestApplyBuildTemplateV2_V1WorkspaceApplyPathStillRejectsV2(t *testing.T) {
	// The v1 workspace endpoint (ApplyBuildTemplateToWorkspaceJSON) must
	// continue to reject v2 payloads with its own guard. This narrow
	// regression case keeps Phase 5A from accidentally removing the
	// existing v2-block in the workspace endpoint.
	app := applyV2Fixture()
	v2 := makeV2Template(t, app, `{"profile":{"level":true}}`, nil)
	res, err := app.ApplyBuildTemplateToWorkspaceJSON("nonexistent-session", v2, ApplyTemplateOptions{})
	// The endpoint rejects either on session lookup or on the v2 guard.
	// Both are acceptable as long as Applied stays false.
	if err == nil && res.Applied {
		t.Fatal("ApplyBuildTemplateToWorkspaceJSON applied a v2 template")
	}
}

// ─── 20. Edit session conflict ────────────────────────────────────────

func TestApplyBuildTemplateV2_EditSessionConflict(t *testing.T) {
	app := applyV2Fixture()
	app.editSessionsMu.Lock()
	app.editSessionByChar[0] = "fake-session-id"
	app.editSessionsMu.Unlock()
	jsonText := makeV2Template(t, app, `{"stats":true}`, nil)
	pre := snapPlayer(app.save.Slots[0].Player)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true with active edit session — Phase 5A must reject")
	}
	if len(res.Preview.Errors) == 0 || !strings.Contains(res.Preview.Errors[0].Message, "edit session") {
		t.Errorf("expected edit session error, got %+v", res.Preview.Errors)
	}
	if snapPlayer(app.save.Slots[0].Player) != pre {
		t.Errorf("slot mutated despite session conflict")
	}
}

// ─── 21. Unknown mode rejected ────────────────────────────────────────

func TestApplyBuildTemplateV2_RejectsUnknownMode(t *testing.T) {
	app := applyV2Fixture()
	jsonText := makeV2Template(t, app, `{"stats":true}`, nil)
	res, err := app.ApplyBuildTemplateV2ToCharacterJSON(0, jsonText, ApplyTemplateV2Options{Mode: "replace"})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true with mode=replace; only append is supported")
	}
	if len(res.Preview.Errors) == 0 || res.Preview.Errors[0].Code != templates.IssueCodeUnknownMode {
		t.Errorf("expected IssueCodeUnknownMode, got %+v", res.Preview.Errors)
	}
}

// ─── Phase 5B — library sibling endpoint ──────────────────────────────

// saveV2TemplateInLibrary builds a v2 template via the existing Phase 3B
// pipeline (so we exercise the real schema/selection invariants the
// frontend would produce) and persists it through Library.SaveTemplate.
// Returns the entry id used to load the template back via the library
// endpoint under test.
func saveV2TemplateInLibrary(t *testing.T, app *App, selectionJSON string, override func(*templates.BuildTemplate)) string {
	t.Helper()
	if app.templateLibrary == nil {
		app.templateLibrary = templates.NewTemplateLibrary(t.TempDir())
	}
	// Reuse makeV2Template to build a valid canonical v2 JSON; then parse
	// it back and persist via SaveTemplate so the library entry carries
	// the exact bytes the Phase 5B endpoint will load.
	jsonText := makeV2Template(t, app, selectionJSON, override)
	tpl, err := templates.ParseBuildTemplateJSON([]byte(jsonText))
	if err != nil {
		t.Fatalf("re-parse exported v2 template: %v", err)
	}
	entry, err := app.templateLibrary.SaveTemplate(tpl)
	if err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	return entry.ID
}

func TestApplyBuildTemplateV2FromLibrary_Success(t *testing.T) {
	app := applyV2Fixture()
	id := saveV2TemplateInLibrary(t, app, `{"profile":{"level":true},"stats":{"vigor":true}}`, func(tpl *templates.BuildTemplate) {
		tpl.Sections.Profile.Level = u32(91)
		tpl.Sections.Stats.Vigor = u32(58)
	})

	res, err := app.ApplyBuildTemplateV2FromLibraryToCharacter(0, id, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply from library: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	if !reflect.DeepEqual(res.AppliedFields, []string{"profile.level", "stats.vigor"}) {
		t.Errorf("AppliedFields = %v, want [profile.level stats.vigor]", res.AppliedFields)
	}
	if app.save.Slots[0].Player.Level != 91 {
		t.Errorf("Level = %d, want 91", app.save.Slots[0].Player.Level)
	}
	if app.save.Slots[0].Player.Vigor != 58 {
		t.Errorf("Vigor = %d, want 58", app.save.Slots[0].Player.Vigor)
	}
	if res.Character == nil || res.Character.Level != 91 {
		t.Errorf("Character missing or stale: %+v", res.Character)
	}
}

func TestApplyBuildTemplateV2FromLibrary_NotFound(t *testing.T) {
	app := applyV2Fixture()
	app.templateLibrary = templates.NewTemplateLibrary(t.TempDir())
	pre := snapPlayer(app.save.Slots[0].Player)

	res, err := app.ApplyBuildTemplateV2FromLibraryToCharacter(0, "does-not-exist", ApplyTemplateV2Options{})
	if err == nil {
		t.Fatalf("expected error for unknown id, got Applied=%v", res.Applied)
	}
	if res.Applied {
		t.Fatal("Applied=true for unknown id")
	}
	if res.CharIndex != 0 {
		t.Errorf("CharIndex = %d, want 0", res.CharIndex)
	}
	if snapPlayer(app.save.Slots[0].Player) != pre {
		t.Errorf("slot mutated despite not-found rejection")
	}
}

func TestApplyBuildTemplateV2FromLibrary_EmptyID(t *testing.T) {
	app := applyV2Fixture()
	app.templateLibrary = templates.NewTemplateLibrary(t.TempDir())

	res, err := app.ApplyBuildTemplateV2FromLibraryToCharacter(0, "", ApplyTemplateV2Options{})
	if err == nil {
		t.Fatalf("expected error for empty id, got Applied=%v", res.Applied)
	}
	if res.Applied {
		t.Fatal("Applied=true for empty id")
	}
}

func TestApplyBuildTemplateV2FromLibrary_V1EntryRejected(t *testing.T) {
	app := applyV2Fixture()
	app.templateLibrary = templates.NewTemplateLibrary(t.TempDir())

	// Persist a valid v1 template directly via SaveTemplate (the library
	// stores both schema versions; the v2 endpoint must refuse v1 via
	// the Phase 5A routing message).
	v1 := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    1,
		AppVersion: "test",
		CreatedAt:  "2026-05-31T12:00:00Z",
		Sections: templates.TemplateSections{
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{{
					BaseItemID: 0x401EA3C3,
					Name:       "Igon's Furled Finger",
					Category:   "key_items",
					Quantity:   1,
					Container:  templates.ContainerInventory,
					Position:   0,
				}},
				StorageItems: []templates.TemplateItem{},
			},
		},
	}
	entry, err := app.templateLibrary.SaveTemplate(v1)
	if err != nil {
		t.Fatalf("SaveTemplate v1: %v", err)
	}
	pre := snapPlayer(app.save.Slots[0].Player)

	res, err := app.ApplyBuildTemplateV2FromLibraryToCharacter(0, entry.ID, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply from library: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for v1 library entry")
	}
	if len(res.Preview.Errors) == 0 {
		t.Fatal("expected preview error")
	}
	msg := res.Preview.Errors[0].Message
	if !strings.Contains(msg, "schema v2") || !strings.Contains(msg, "v1") {
		t.Errorf("error message %q does not mention v2 vs v1 routing", msg)
	}
	if snapPlayer(app.save.Slots[0].Player) != pre {
		t.Errorf("slot mutated despite v1 rejection")
	}
}

// TestApplyBuildTemplateV2FromLibrary_InventoryWorkspaceWithoutSessionRejected —
// library wrapper preserves the Phase 7a gate verbatim. Same library entry,
// no SessionID option supplied → IssueCodeInventorySessionRequired surfaces
// through the delegation without mutating the slot.
func TestApplyBuildTemplateV2FromLibrary_InventoryWorkspaceWithoutSessionRejected(t *testing.T) {
	app := applyV2Fixture()
	app.templateLibrary = templates.NewTemplateLibrary(t.TempDir())

	tpl := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    2,
		AppVersion: "test",
		CreatedAt:  "2026-05-31T12:00:00Z",
		Selection: &templates.TemplateSelection{
			Stats:              &templates.SectionSelection{All: true},
			InventoryWorkspace: &templates.SectionSelection{All: true},
		},
		Sections: templates.TemplateSections{
			Stats: &templates.StatsSection{Vigor: u32(40), Mind: u32(30), Endurance: u32(30), Strength: u32(20), Dexterity: u32(20), Intelligence: u32(20), Faith: u32(20), Arcane: u32(20)},
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{},
				StorageItems:   []templates.TemplateItem{},
			},
		},
	}
	entry, err := app.templateLibrary.SaveTemplate(tpl)
	if err != nil {
		t.Fatalf("SaveTemplate: %v", err)
	}
	pre := snapPlayer(app.save.Slots[0].Player)

	res, err := app.ApplyBuildTemplateV2FromLibraryToCharacter(0, entry.ID, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for v2 entry with inventory.workspace selection and no session")
	}
	found := false
	for _, issue := range res.Preview.Errors {
		if issue.Code == templates.IssueCodeInventorySessionRequired {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected IssueCodeInventorySessionRequired, got %+v", res.Preview.Errors)
	}
	if snapPlayer(app.save.Slots[0].Player) != pre {
		t.Errorf("slot mutated despite scope rejection")
	}
}

// ─── Phase 5B — file sibling endpoint (YAML, dialog-less core) ────────
//
// runtime.OpenFileDialog requires a Wails app context (a.ctx) and is not
// mockable in unit tests, so these tests drive applyV2TemplateFromYAMLPath
// — the dialog-less core of ApplyBuildTemplateV2FromFileToCharacter. The
// dialog wrapper itself is a thin path-fetch + delegate that is exercised
// indirectly: every reachable behaviour after path acquisition is covered
// by these tests.

func writeTempYAMLFile(t *testing.T, contents []byte) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "template.yaml")
	if err := os.WriteFile(path, contents, 0o644); err != nil {
		t.Fatalf("write temp yaml: %v", err)
	}
	return path
}

func TestApplyBuildTemplateV2FromFileCore_Success(t *testing.T) {
	app := applyV2Fixture()
	jsonText := makeV2Template(t, app, `{"profile":{"level":true},"stats":{"faith":true}}`, func(tpl *templates.BuildTemplate) {
		tpl.Sections.Profile.Level = u32(85)
		tpl.Sections.Stats.Faith = u32(45)
	})
	// Convert the canonical JSON to YAML so the file path exercises the
	// real YAML decode + canonical re-encode pipeline.
	tpl, err := templates.ParseBuildTemplateJSON([]byte(jsonText))
	if err != nil {
		t.Fatalf("parse JSON: %v", err)
	}
	yamlBytes, err := templates.MarshalBuildTemplateYAML(tpl)
	if err != nil {
		t.Fatalf("marshal YAML: %v", err)
	}
	path := writeTempYAMLFile(t, yamlBytes)

	res, err := app.applyV2TemplateFromYAMLPath(0, path, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply from yaml path: %v", err)
	}
	if !res.Applied {
		t.Fatalf("Applied=false: %+v", res.Preview.Errors)
	}
	if app.save.Slots[0].Player.Level != 85 {
		t.Errorf("Level = %d, want 85", app.save.Slots[0].Player.Level)
	}
	if app.save.Slots[0].Player.Faith != 45 {
		t.Errorf("Faith = %d, want 45", app.save.Slots[0].Player.Faith)
	}
}

func TestApplyBuildTemplateV2FromFileCore_InvalidYAMLRejected(t *testing.T) {
	app := applyV2Fixture()
	path := writeTempYAMLFile(t, []byte("this is: not: valid: yaml: at: all\n\t- malformed\n"))
	pre := snapPlayer(app.save.Slots[0].Player)

	res, err := app.applyV2TemplateFromYAMLPath(0, path, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for malformed YAML")
	}
	if len(res.Preview.Errors) == 0 || res.Preview.Errors[0].Code != templates.IssueCodeStructureInvalid {
		t.Errorf("expected IssueCodeStructureInvalid, got %+v", res.Preview.Errors)
	}
	if snapPlayer(app.save.Slots[0].Player) != pre {
		t.Errorf("slot mutated despite parse failure")
	}
}

func TestApplyBuildTemplateV2FromFileCore_V1YAMLRejected(t *testing.T) {
	app := applyV2Fixture()
	v1 := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    1,
		AppVersion: "test",
		CreatedAt:  "2026-05-31T12:00:00Z",
		Sections: templates.TemplateSections{
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{{
					BaseItemID: 0x401EA3C3,
					Name:       "Igon's Furled Finger",
					Category:   "key_items",
					Quantity:   1,
					Container:  templates.ContainerInventory,
					Position:   0,
				}},
				StorageItems: []templates.TemplateItem{},
			},
		},
	}
	yamlBytes, err := templates.MarshalBuildTemplateYAML(v1)
	if err != nil {
		t.Fatalf("marshal v1 YAML: %v", err)
	}
	path := writeTempYAMLFile(t, yamlBytes)
	pre := snapPlayer(app.save.Slots[0].Player)

	res, err := app.applyV2TemplateFromYAMLPath(0, path, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for v1 YAML through v2 endpoint")
	}
	msg := res.Preview.Errors[0].Message
	if !strings.Contains(msg, "schema v2") || !strings.Contains(msg, "v1") {
		t.Errorf("error message %q does not mention v2 vs v1 routing", msg)
	}
	if snapPlayer(app.save.Slots[0].Player) != pre {
		t.Errorf("slot mutated despite v1 rejection")
	}
}

func TestApplyBuildTemplateV2FromFileCore_MissingFile(t *testing.T) {
	app := applyV2Fixture()
	missing := filepath.Join(t.TempDir(), "does-not-exist.yaml")

	res, err := app.applyV2TemplateFromYAMLPath(0, missing, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for missing file")
	}
	if len(res.Preview.Errors) == 0 || res.Preview.Errors[0].Code != templates.IssueCodeStructureInvalid {
		t.Errorf("expected IssueCodeStructureInvalid, got %+v", res.Preview.Errors)
	}
}

// TestApplyBuildTemplateV2FromFileCore_V2InventoryWorkspaceWithoutSessionRejected —
// YAML-file wrapper preserves the Phase 7a gate verbatim. The dialog-less
// core delegates to the JSON endpoint, so the session-required reject
// surfaces unchanged across both source paths.
func TestApplyBuildTemplateV2FromFileCore_V2InventoryWorkspaceWithoutSessionRejected(t *testing.T) {
	app := applyV2Fixture()
	tpl := &templates.BuildTemplate{
		Schema:     templates.SchemaKey,
		Version:    2,
		AppVersion: "test",
		CreatedAt:  "2026-05-31T12:00:00Z",
		Selection: &templates.TemplateSelection{
			Stats:              &templates.SectionSelection{All: true},
			InventoryWorkspace: &templates.SectionSelection{All: true},
		},
		Sections: templates.TemplateSections{
			Stats: &templates.StatsSection{Vigor: u32(40), Mind: u32(30), Endurance: u32(30), Strength: u32(20), Dexterity: u32(20), Intelligence: u32(20), Faith: u32(20), Arcane: u32(20)},
			InventoryWorkspace: &templates.InventoryWorkspaceSection{
				InventoryItems: []templates.TemplateItem{},
				StorageItems:   []templates.TemplateItem{},
			},
		},
	}
	yamlBytes, err := templates.MarshalBuildTemplateYAML(tpl)
	if err != nil {
		t.Fatalf("marshal YAML: %v", err)
	}
	path := writeTempYAMLFile(t, yamlBytes)
	pre := snapPlayer(app.save.Slots[0].Player)

	res, err := app.applyV2TemplateFromYAMLPath(0, path, ApplyTemplateV2Options{})
	if err != nil {
		t.Fatalf("apply: %v", err)
	}
	if res.Applied {
		t.Fatal("Applied=true for v2 YAML with inventory.workspace selection and no session")
	}
	found := false
	for _, issue := range res.Preview.Errors {
		if issue.Code == templates.IssueCodeInventorySessionRequired {
			found = true
			break
		}
	}
	if !found {
		t.Errorf("expected IssueCodeInventorySessionRequired, got %+v", res.Preview.Errors)
	}
	if snapPlayer(app.save.Slots[0].Player) != pre {
		t.Errorf("slot mutated despite scope rejection")
	}
}

// ─── Phase 5B — cancellation sentinel shape ───────────────────────────
//
// runtime.OpenFileDialog cannot be invoked without a Wails app context,
// so the cancellation branch of ApplyBuildTemplateV2FromFileToCharacter
// is exercised via the helper directly: confirms that the sentinel
// preview matches cancelledPreviewReport's shape and Applied stays
// false. No filesystem touch.

func TestApplyBuildTemplateV2FromFile_CancelledSentinel(t *testing.T) {
	res := cancelledApplyV2Result(0)
	if res.Applied {
		t.Fatal("cancelled sentinel has Applied=true")
	}
	if res.CharIndex != 0 {
		t.Errorf("CharIndex = %d, want 0", res.CharIndex)
	}
	if res.Preview.OK {
		t.Error("cancelled preview should have OK=false")
	}
	if len(res.Preview.Errors) != 0 {
		t.Errorf("cancelled preview should carry no errors, got %d", len(res.Preview.Errors))
	}
}
