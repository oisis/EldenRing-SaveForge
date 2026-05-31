package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// ApplyTemplateV2Options controls how a v2 template applies to a character.
// Mode is a forward-compat string; Phase 5A only accepts "append" (the
// default and only mode). Replace / merge modes are intentionally rejected
// until later phases ship the corresponding semantics.
type ApplyTemplateV2Options struct {
	Mode string `json:"mode,omitempty"`
}

// ApplyTemplateV2Result is the dual-purpose return of
// ApplyBuildTemplateV2ToCharacterJSON. It mirrors the
// (Preview, Applied) shape used by the v1 workspace apply path so the
// frontend can render either an error report or a success summary from a
// single struct.
//
// CharIndex echoes the target slot so the UI can correlate the result
// even when the apply was rejected before mutation. AppliedFields /
// SkippedFields list canonical paths ("profile.level", "stats.vigor")
// in a stable order — see profileApplyOrder / statsApplyOrder. Character
// is the freshly-mapped post-apply ViewModel; it is nil on failure.
type ApplyTemplateV2Result struct {
	Preview       templates.ImportPreviewReport `json:"preview"`
	Applied       bool                          `json:"applied"`
	CharIndex     int                           `json:"charIndex"`
	AppliedFields []string                      `json:"appliedFields"`
	SkippedFields []string                      `json:"skippedFields"`
	Character     *vm.CharacterViewModel        `json:"character,omitempty"`
}

// Canonical apply ordering. Defined once so the AppliedFields /
// SkippedFields lists on the wire are deterministic regardless of source
// map iteration. New v2 sections (equipment, spells, ...) slot in after
// stats when their phases ship.
var (
	profileApplyOrder = []string{
		"name",
		"level",
		"runes",
		"soulMemory",
		"clearCount",
		"scadutreeBlessing",
		"shadowRealmBlessing",
		"talismanSlots",
	}
	statsApplyOrder = []string{
		"vigor",
		"mind",
		"endurance",
		"strength",
		"dexterity",
		"intelligence",
		"faith",
		"arcane",
	}
)

// ApplyBuildTemplateV2ToCharacterJSON parses a schema v2 build template
// and applies its profile / stats sections to slot charIdx of the loaded
// save. Mutation is RAM-only — the user still has to call WriteSave to
// persist to disk, matching SaveCharacter semantics.
//
// Phase 5A scope (see spec/56 §17a and the Phase 5 preflight):
//   - only sections.profile and sections.stats are applied;
//   - sections.inventory.workspace is rejected (Phase 5 carves it out);
//   - profile.class is intentionally selected-but-skipped — no confirmed
//     reverse mapping from display name to class ID yet; the field lands
//     in SkippedFields and slot.Player.Class is not modified;
//   - equipment / equipped talismans / spell loadout / appearance / URL
//     import / multi-character pack remain unsupported.
//
// Locking and side effects mirror SaveCharacter as closely as possible:
//   - saveMu.RLock for the lifetime of the call;
//   - editSessionsMu (short) to detect an inventory edit session on the
//     same character — if present, the apply is rejected (the caller is
//     asked to close the session) rather than racing the session's
//     workspace snapshot;
//   - slotMu[charIdx].Lock for the mutation;
//   - core.SnapshotSlot before mutation; core.RestoreSlot on failure;
//   - pushUndoLocked(charIdx) so the standard Undo button reverts the
//     apply;
//   - NG+ event flags 50..57 are synchronised to slot.Player.ClearCount
//     ONLY when profile.clearCount actually landed;
//   - ProfileSummaries[charIdx].Level / CharacterName is updated ONLY
//     when profile.level / profile.name actually landed.
//
// Failure model mirrors ApplyBuildTemplateToWorkspaceJSON: parse /
// schema / scope / validation errors return
// (ApplyTemplateV2Result{Applied:false, Preview:{Errors}}, nil error).
// Infrastructure failures (no save loaded, invalid character index,
// internal VM mapping crash) return non-nil error.
func (a *App) ApplyBuildTemplateV2ToCharacterJSON(charIdx int, jsonText string, opts ApplyTemplateV2Options) (ApplyTemplateV2Result, error) {
	mode := opts.Mode
	if mode == "" {
		mode = "append"
	}
	if mode != "append" {
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   singleErrorPreview(templates.IssueCodeUnknownMode, fmt.Sprintf("ApplyBuildTemplateV2: unsupported import mode %q (Phase 5A only ships %q)", mode, "append")),
			Applied:   false,
		}, nil
	}

	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return ApplyTemplateV2Result{CharIndex: charIdx}, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return ApplyTemplateV2Result{CharIndex: charIdx}, fmt.Errorf("invalid character index %d", charIdx)
	}

	tpl, err := templates.ParseBuildTemplateJSON([]byte(jsonText))
	if err != nil {
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   singleErrorPreview(templates.IssueCodeStructureInvalid, err.Error()),
			Applied:   false,
		}, nil
	}

	if tpl.Version == 1 {
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   singleErrorPreview(templates.IssueCodeStructureInvalid, "this endpoint accepts schema v2 templates only; use ApplyBuildTemplateToWorkspaceJSON for schema v1"),
			Applied:   false,
		}, nil
	}
	if tpl.Version != 2 {
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   singleErrorPreview(templates.IssueCodeSchemaInvalid, fmt.Sprintf("ApplyBuildTemplateV2: unsupported schema version %d (Phase 5A only accepts version 2)", tpl.Version)),
			Applied:   false,
		}, nil
	}

	// PreviewBuildTemplateImport runs ValidateBuildTemplate + per-item
	// preview. It is v2-aware (Summary.SelectedSections,
	// ProfileFieldsPresent, StatFieldsPresent populated; v1 InventoryWorkspace
	// preview skipped when the section is nil).
	report := templates.PreviewBuildTemplateImport(tpl, templates.ImportPreviewOptions{Mode: mode})
	if !report.OK {
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   report,
			Applied:   false,
		}, nil
	}

	// Phase 5A scope check — only profile / stats may apply.
	if tpl.Selection != nil && tpl.Selection.InventoryWorkspace.HasAny() {
		report.OK = false
		report.Errors = append(report.Errors, templates.ImportPreviewIssue{
			Severity: "error",
			Code:     templates.IssueCodeUnsupportedCategory,
			Message:  "schema v2 inventory.workspace apply is not supported in Phase 5",
		})
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   report,
			Applied:   false,
		}, nil
	}
	hasProfile := tpl.Selection != nil && tpl.Selection.Profile.HasAny()
	hasStats := tpl.Selection != nil && tpl.Selection.Stats.HasAny()
	if !hasProfile && !hasStats {
		// ValidateBuildTemplate already rejects an empty selection
		// (HasAnySelected == false) — this branch defends against future
		// validator drift.
		report.OK = false
		report.Errors = append(report.Errors, templates.ImportPreviewIssue{
			Severity: "error",
			Code:     templates.IssueCodeStructureInvalid,
			Message:  "schema v2 template selects neither profile nor stats",
		})
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   report,
			Applied:   false,
		}, nil
	}

	// Edit session conflict — short read on the registry lock so the
	// rejection happens before we take slotMu and the caller sees a
	// fast, clear error.
	a.editSessionsMu.Lock()
	_, sessionConflict := a.editSessionByChar[charIdx]
	a.editSessionsMu.Unlock()
	if sessionConflict {
		report.OK = false
		report.Errors = append(report.Errors, templates.ImportPreviewIssue{
			Severity: "error",
			Code:     templates.IssueCodeStructureInvalid,
			Message:  fmt.Sprintf("close the inventory edit session for slot %d before applying a schema v2 character template", charIdx),
		})
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   report,
			Applied:   false,
		}, nil
	}

	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()

	if !a.save.ActiveSlots[charIdx] {
		report.OK = false
		report.Errors = append(report.Errors, templates.ImportPreviewIssue{
			Severity: "error",
			Code:     templates.IssueCodeStructureInvalid,
			Message:  fmt.Sprintf("character slot %d is inactive", charIdx),
		})
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   report,
			Applied:   false,
		}, nil
	}

	slot := &a.save.Slots[charIdx]
	snapshot := core.SnapshotSlot(slot)

	charVM, err := vm.MapParsedSlotToVM(slot)
	if err != nil {
		core.RestoreSlot(slot, snapshot)
		return ApplyTemplateV2Result{CharIndex: charIdx, Preview: report, Applied: false}, fmt.Errorf("ApplyBuildTemplateV2: map slot to VM: %w", err)
	}

	var applied, skipped []string
	if hasProfile {
		ap, sk := applyTemplateV2ProfileToVM(charVM, tpl.Selection.Profile, tpl.Sections.Profile)
		applied = append(applied, ap...)
		skipped = append(skipped, sk...)
	}
	if hasStats {
		ap, sk := applyTemplateV2StatsToVM(charVM, tpl.Selection.Stats, tpl.Sections.Stats)
		applied = append(applied, ap...)
		skipped = append(skipped, sk...)
	}

	if len(applied) == 0 {
		// Selection nominated fields but the corresponding section was
		// nil for all of them — nothing to write. Returning Applied=false
		// without taking pushUndoLocked / mutating slot keeps the no-op
		// audit-clean.
		return ApplyTemplateV2Result{
			CharIndex:     charIdx,
			Preview:       report,
			Applied:       false,
			AppliedFields: applied,
			SkippedFields: skipped,
		}, nil
	}

	clearCountApplied := containsString(applied, "profile.clearCount")
	nameApplied := containsString(applied, "profile.name")
	levelApplied := containsString(applied, "profile.level")

	a.pushUndoLocked(charIdx)

	if err := vm.ApplyVMToParsedSlot(charVM, slot); err != nil {
		core.RestoreSlot(slot, snapshot)
		return ApplyTemplateV2Result{CharIndex: charIdx, Preview: report, Applied: false}, fmt.Errorf("ApplyBuildTemplateV2: apply VM: %w", err)
	}
	slot.SyncPlayerToData()

	// NG+ event flag sync — mirror SaveCharacter (app.go:339-345). The
	// per-slot offset / buffer guards match the existing pattern; we only
	// touch flags 50..57 and only when clearCount actually landed.
	if clearCountApplied && slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for i := uint32(0); i <= 7; i++ {
			_ = db.SetEventFlag(flags, 50+i, i == slot.Player.ClearCount)
		}
	}

	// ProfileSummary update — only the menu fields the apply actually
	// changed. Mirrors SaveCharacter at app.go:347-349 but split so a
	// stats-only apply does not pointlessly rewrite the summary.
	if levelApplied {
		a.save.ProfileSummaries[charIdx].Level = a.save.Slots[charIdx].Player.Level
	}
	if nameApplied {
		copy(a.save.ProfileSummaries[charIdx].CharacterName[:], a.save.Slots[charIdx].Player.CharacterName[:])
	}

	freshVM, err := vm.MapParsedSlotToVM(slot)
	if err != nil {
		core.RestoreSlot(slot, snapshot)
		return ApplyTemplateV2Result{CharIndex: charIdx, Preview: report, Applied: false}, fmt.Errorf("ApplyBuildTemplateV2: re-read VM: %w", err)
	}

	return ApplyTemplateV2Result{
		CharIndex:     charIdx,
		Preview:       report,
		Applied:       true,
		AppliedFields: applied,
		SkippedFields: skipped,
		Character:     freshVM,
	}, nil
}

// applyTemplateV2ProfileToVM mutates charVM with selected+present profile
// fields and returns (applied, skipped) lists of canonical paths
// ("profile.level"). profile.class is intentionally added to skipped
// when both selected and present — see Phase 5A scope notes on
// ApplyBuildTemplateV2ToCharacterJSON.
func applyTemplateV2ProfileToVM(charVM *vm.CharacterViewModel, sel *templates.SectionSelection, sec *templates.ProfileSection) ([]string, []string) {
	var applied, skipped []string
	if sec == nil {
		return applied, skipped
	}
	// profile.class is skipped by design in Phase 5A.
	if sel.Selected("class") && sec.Class != nil {
		skipped = append(skipped, "profile.class")
	}
	for _, field := range profileApplyOrder {
		if !sel.Selected(field) {
			continue
		}
		if applyProfileFieldToVM(charVM, field, sec) {
			applied = append(applied, "profile."+field)
		} else {
			skipped = append(skipped, "profile."+field)
		}
	}
	return applied, skipped
}

func applyProfileFieldToVM(charVM *vm.CharacterViewModel, field string, sec *templates.ProfileSection) bool {
	switch field {
	case "name":
		if sec.Name == nil {
			return false
		}
		charVM.Name = *sec.Name
	case "level":
		if sec.Level == nil {
			return false
		}
		charVM.Level = *sec.Level
	case "runes":
		if sec.Runes == nil {
			return false
		}
		charVM.Souls = *sec.Runes
	case "soulMemory":
		if sec.SoulMemory == nil {
			return false
		}
		charVM.SoulMemory = *sec.SoulMemory
	case "clearCount":
		if sec.ClearCount == nil {
			return false
		}
		charVM.ClearCount = *sec.ClearCount
	case "scadutreeBlessing":
		if sec.ScadutreeBlessing == nil {
			return false
		}
		charVM.ScadutreeBlessing = *sec.ScadutreeBlessing
	case "shadowRealmBlessing":
		if sec.ShadowRealmBlessing == nil {
			return false
		}
		charVM.ShadowRealmBlessing = *sec.ShadowRealmBlessing
	case "talismanSlots":
		if sec.TalismanSlots == nil {
			return false
		}
		charVM.TalismanSlots = *sec.TalismanSlots
	default:
		return false
	}
	return true
}

// applyTemplateV2StatsToVM mirrors the profile helper for the eight
// stat fields. Stats have no "skipped-by-design" branch — every selected
// stat with a non-nil pointer is applied.
func applyTemplateV2StatsToVM(charVM *vm.CharacterViewModel, sel *templates.SectionSelection, sec *templates.StatsSection) ([]string, []string) {
	var applied, skipped []string
	if sec == nil {
		return applied, skipped
	}
	for _, field := range statsApplyOrder {
		if !sel.Selected(field) {
			continue
		}
		if applyStatFieldToVM(charVM, field, sec) {
			applied = append(applied, "stats."+field)
		} else {
			skipped = append(skipped, "stats."+field)
		}
	}
	return applied, skipped
}

func applyStatFieldToVM(charVM *vm.CharacterViewModel, field string, sec *templates.StatsSection) bool {
	switch field {
	case "vigor":
		if sec.Vigor == nil {
			return false
		}
		charVM.Vigor = *sec.Vigor
	case "mind":
		if sec.Mind == nil {
			return false
		}
		charVM.Mind = *sec.Mind
	case "endurance":
		if sec.Endurance == nil {
			return false
		}
		charVM.Endurance = *sec.Endurance
	case "strength":
		if sec.Strength == nil {
			return false
		}
		charVM.Strength = *sec.Strength
	case "dexterity":
		if sec.Dexterity == nil {
			return false
		}
		charVM.Dexterity = *sec.Dexterity
	case "intelligence":
		if sec.Intelligence == nil {
			return false
		}
		charVM.Intelligence = *sec.Intelligence
	case "faith":
		if sec.Faith == nil {
			return false
		}
		charVM.Faith = *sec.Faith
	case "arcane":
		if sec.Arcane == nil {
			return false
		}
		charVM.Arcane = *sec.Arcane
	default:
		return false
	}
	return true
}

// singleErrorPreview builds an ImportPreviewReport carrying exactly one
// error issue. Used by every early-exit branch of
// ApplyBuildTemplateV2ToCharacterJSON so the wire shape matches what the
// frontend already renders for v1 apply errors.
func singleErrorPreview(code, message string) templates.ImportPreviewReport {
	return templates.ImportPreviewReport{
		OK: false,
		Errors: []templates.ImportPreviewIssue{{
			Severity: "error",
			Code:     code,
			Message:  message,
		}},
		Warnings: []templates.ImportPreviewIssue{},
		Summary:  templates.ImportPreviewSummary{},
	}
}

func containsString(haystack []string, needle string) bool {
	for _, s := range haystack {
		if s == needle {
			return true
		}
	}
	return false
}
