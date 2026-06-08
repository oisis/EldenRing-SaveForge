package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// ApplyTemplateV2Options controls how a v2 template applies to a character.
// Mode is a forward-compat string; Phase 5A only accepts "append" (the
// default and only mode). Replace / merge modes are intentionally rejected
// until later phases ship the corresponding semantics.
//
// Phase 7a — SessionID. When the v2 template's selection nominates
// inventory.workspace, the caller MUST provide the ID of an active
// Inventory Edit Session that targets the same charIdx. The apply path
// resolves the session via App.acquireSession (long-lived per-session
// mutex held across the workspace mutation) and rolls the workspace
// snapshot back on any error. Profile/stats-only applies ignore the
// field — passing it for a non-inventory template is accepted silently
// so the frontend may unconditionally send the active session ID.
//
// Phase 7a.2 — WeaponLevelOverride. Optional apply-time runtime override
// of upgrade levels for weapons added by the inventory.workspace section.
// Reuses the v1 WeaponLevelOverride type and validateWeaponLevelOverride
// pre-check verbatim. Threaded into applyTemplateItemsToWorkspace for
// both inventory and storage containers. Profile/stats-only templates
// silently ignore a structurally valid override (no items → no-op).
type ApplyTemplateV2Options struct {
	Mode                string               `json:"mode,omitempty"`
	SessionID           string               `json:"sessionID,omitempty"`
	WeaponLevelOverride *WeaponLevelOverride `json:"weaponLevelOverride,omitempty"`
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

	// Phase 7a — inventory.workspace apply counters. Populated only
	// when sections.inventory.workspace was actually applied (selection
	// present + active session). Profile/stats-only applies leave both
	// counters at zero. Workspace is the post-apply snapshot of the
	// active session; nil when no workspace mutation happened.
	InventoryItemsApplied int                                `json:"inventoryItemsApplied"`
	StorageItemsApplied   int                                `json:"storageItemsApplied"`
	Workspace             *editor.InventoryWorkspaceSnapshot `json:"workspace,omitempty"`

	// Phase 7b.1 — equipment apply counter. Number of ChrAsmEquipment
	// slots actually written to slot.Data by SaveSlot.WriteEquipment.
	// Slots reported as not-in-inventory warnings are NOT counted; the
	// counter reflects successful writer dispatch only.
	EquipmentSlotsApplied int `json:"equipmentSlotsApplied"`

	// Phase 7d.3 — spells apply counter. Number of EquippedSpells slots
	// actually written to slot.Data by SaveSlot.WriteSpells. Slots
	// downgraded to warnings (unknown spell ID, wrong prefix) are NOT
	// counted; the counter reflects successful writer dispatch only.
	SpellSlotsApplied int `json:"spellSlotsApplied"`
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
// Failure model mirrors applyBuildTemplateToWorkspaceFromJSON: parse /
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

	// Phase 7a.2 — weaponLevelOverride structural validation. Mirrors v1
	// applyBuildTemplateToWorkspaceFromJSON: runs BEFORE acquireSession /
	// snapshot / mutation so a broken request bounces with zero side
	// effects. Out-of-range positive values pass here and are clamped by
	// editor.ClampUpgrade later, surfacing as warnings.
	if err := validateWeaponLevelOverride(opts.WeaponLevelOverride); err != nil {
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   singleErrorPreview(templates.IssueCodeStructureInvalid, err.Error()),
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
			Preview:   singleErrorPreview(templates.IssueCodeStructureInvalid, "this endpoint accepts schema v2 templates only; schema v1 payloads are supported only via ApplyBuildTemplateFromLibrary for already-stored library entries"),
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

	hasProfile := tpl.Selection != nil && tpl.Selection.Profile.HasAny()
	hasStats := tpl.Selection != nil && tpl.Selection.Stats.HasAny()
	hasInventory := tpl.Selection != nil && tpl.Selection.InventoryWorkspace.HasAny()
	hasEquipment := tpl.Selection != nil && tpl.Selection.Equipment.HasAny()
	hasSpells := tpl.Selection != nil && tpl.Selection.Spells.HasAny()
	// Phase 8D.1 — v2 items / layout selection.
	hasItems := tpl.Selection != nil && tpl.Selection.Items.HasAny()
	hasInvLayout := tpl.Selection != nil && tpl.Selection.InventoryLayout.HasAny()
	hasStoLayout := tpl.Selection != nil && tpl.Selection.StorageLayout.HasAny()
	if !hasProfile && !hasStats && !hasInventory && !hasEquipment && !hasSpells && !hasItems {
		// ValidateBuildTemplate already rejects an empty selection
		// (HasAnySelected == false) — this branch defends against future
		// validator drift. Layout-only selection is not a supported
		// apply target in Phase 8D.1 (items must be selected for layout
		// to be meaningful); the validator catches a layout-without-items
		// selection at parse time so it cannot reach here.
		report.OK = false
		report.Errors = append(report.Errors, templates.ImportPreviewIssue{
			Severity: "error",
			Code:     templates.IssueCodeStructureInvalid,
			Message:  "schema v2 template selects no applyable section (profile / stats / inventory.workspace / equipment / spells / items)",
		})
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   report,
			Applied:   false,
		}, nil
	}

	// Phase 7b.1 — hard reject equipment + inventory.workspace combo at the
	// apply boundary too. The preview already injects this error in
	// PreviewBuildTemplateImport, but the apply double-checks here so direct
	// callers of the JSON endpoint that bypass the preview cannot smuggle a
	// combo through. See IssueCodeEquipmentInventoryComboUnsupported for
	// the GaMap-freshness rationale.
	if hasEquipment && hasInventory {
		report.OK = false
		report.Errors = append(report.Errors, templates.ImportPreviewIssue{
			Severity: "error",
			Code:     templates.IssueCodeEquipmentInventoryComboUnsupported,
			Message:  "sections.equipment cannot be applied together with sections.inventory.workspace in the same template (Phase 7b.1 limitation).",
		})
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   report,
			Applied:   false,
		}, nil
	}

	// Phase 8D.1 — v1 inventory.workspace + v2 items in the same
	// template is fail-closed. The two address overlapping state
	// (sess.Workspace) through different schemas; merging them in one
	// pass would silently double-add or skip depending on order. The
	// MVP applies only one of the two systems per call.
	if hasItems && hasInventory {
		report.OK = false
		report.Errors = append(report.Errors, templates.ImportPreviewIssue{
			Severity: "error",
			Code:     templates.IssueCodeItemsV1V2Mix,
			Message:  "schema v2 sections.items cannot be applied together with v1 sections.inventory.workspace in the same template (Phase 8D.1 limitation).",
		})
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   report,
			Applied:   false,
		}, nil
	}

	// Phase 8D.1 — items apply mode gate. Phase 8D.1 only ships
	// addMissing; tpl.ApplyOptions.Items==nil defaults to addMissing
	// when items selection is present. Any other mode (merge,
	// updateExisting, replace) is rejected before mutation.
	if hasItems {
		mode := templates.ItemApplyModeAddMissing
		if tpl.ApplyOptions != nil && tpl.ApplyOptions.Items != nil && tpl.ApplyOptions.Items.Mode != "" {
			mode = tpl.ApplyOptions.Items.Mode
		}
		if mode != templates.ItemApplyModeAddMissing {
			report.OK = false
			report.Errors = append(report.Errors, templates.ImportPreviewIssue{
				Severity: "error",
				Code:     templates.IssueCodeItemsModeUnsupported,
				Message:  fmt.Sprintf("schema v2 sections.items mode %q is not supported in Phase 8D.1 (only addMissing).", mode),
			})
			return ApplyTemplateV2Result{
				CharIndex: charIdx,
				Preview:   report,
				Applied:   false,
			}, nil
		}
	}

	// Phase 7a — v2 inventory.workspace session gating.
	//
	// Mode A: hasInventory=false → preserve the Phase 5 behaviour. An
	// active session for this character is a CONFLICT (we would race
	// it), so we refuse fast with the existing "close the session"
	// hint. opts.SessionID is silently ignored — the frontend may pass
	// it speculatively whenever a session happens to exist.
	//
	// Mode B: hasInventory=true → the apply REQUIRES a matching active
	// session. Empty SessionID → hard reject (IssueCodeInventorySessionRequired).
	// Unknown ID, closed session, or session targeting a different
	// character → hard reject (IssueCodeInventorySessionInvalid). On
	// success, sess.Acquire() returns with the per-session mutex held;
	// we defer Unlock so every error path releases it.
	//
	// Lock order matches SaveInventoryWorkspaceChanges:
	//   saveMu.RLock (already held) → editSessionsMu (short) →
	//   sess.mu (long, via acquireSession) → slotMu[charIdx].
	// Phase 8D.1 — hasItems shares the session-required path with
	// hasInventory. Both write into sess.Workspace; both need the same
	// "open the Sort Order workspace first" guard.
	needsSession := hasInventory || hasItems
	var sess *editor.InventoryEditSession
	if needsSession {
		if opts.SessionID == "" {
			report.OK = false
			report.Errors = append(report.Errors, templates.ImportPreviewIssue{
				Severity: "error",
				Code:     templates.IssueCodeInventorySessionRequired,
				Message:  "Open the Sort Order workspace before applying inventory / items templates.",
			})
			return ApplyTemplateV2Result{CharIndex: charIdx, Preview: report, Applied: false}, nil
		}
		s, err := a.acquireSession(opts.SessionID)
		if err != nil {
			report.OK = false
			report.Errors = append(report.Errors, templates.ImportPreviewIssue{
				Severity: "error",
				Code:     templates.IssueCodeInventorySessionInvalid,
				Message:  fmt.Sprintf("inventory edit session %q is not active; reopen the Sort Order workspace.", opts.SessionID),
			})
			return ApplyTemplateV2Result{CharIndex: charIdx, Preview: report, Applied: false}, nil
		}
		if s.CharacterIndex != charIdx {
			s.Unlock()
			report.OK = false
			report.Errors = append(report.Errors, templates.ImportPreviewIssue{
				Severity: "error",
				Code:     templates.IssueCodeInventorySessionInvalid,
				Message:  fmt.Sprintf("inventory edit session %q targets character slot %d, not slot %d.", opts.SessionID, s.CharacterIndex+1, charIdx+1),
			})
			return ApplyTemplateV2Result{CharIndex: charIdx, Preview: report, Applied: false}, nil
		}
		sess = s
		defer sess.Unlock()
	} else {
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

	// Phase 7a — workspace snapshot for atomic rollback in the
	// mixed-apply case. Only taken when the apply will touch the
	// workspace; otherwise we keep the profile/stats-only fast path
	// allocation-free. The session lock is already held above.
	//
	// Phase 8D.1 — hasItems also touches sess.Workspace, so the same
	// backup gate covers both v1 inventory.workspace and v2 items
	// apply paths.
	var workspaceBackup editor.InventoryWorkspaceSnapshot
	if needsSession {
		workspaceBackup = deepCopySnapshot(sess.Workspace)
	}

	// rollbackBoth restores both slot bytes and (when held) the
	// workspace snapshot. Used by every error exit below so a partial
	// profile/stats write never leaves the workspace dirty, and a
	// partial inventory / items write never leaves slot.Data modified.
	rollbackBoth := func() {
		core.RestoreSlot(slot, snapshot)
		if needsSession {
			sess.Workspace = workspaceBackup
		}
	}

	// Phase 7a — capacity preflight for inventory.workspace BEFORE any
	// mutation. Mirrors the v1 applyBuildTemplateToWorkspaceFromJSON guard so
	// a "would not fit" diagnosis surfaces as a preview error without
	// touching either snapshot.
	if hasInventory {
		sec := tpl.Sections.InventoryWorkspace
		if sec != nil {
			if capacityIssues := capacityPreflight(sess.Workspace, sec); len(capacityIssues) > 0 {
				report.Errors = append(report.Errors, capacityIssues...)
				report.OK = false
				return ApplyTemplateV2Result{
					CharIndex: charIdx,
					Preview:   report,
					Applied:   false,
				}, nil
			}
		}
	}

	// Phase 8D.1 — informational warnings for the items apply path.
	// Emitted BEFORE the items plan / preflight / mutation so the user
	// sees them even when the apply later fails preflight.
	//
	//   - layout selection / sections: dropped silently (Phase 8D.1
	//     does not apply layout), warning per affected container.
	//   - tpl.ApplyOptions.WeaponLevelOverride: dropped silently
	//     (Phase 8D.1 only honours the runtime override in
	//     opts.WeaponLevelOverride); single warning.
	if hasItems {
		if hasInvLayout || tpl.Sections.InventoryLayout != nil {
			report.Warnings = append(report.Warnings, templates.ImportPreviewIssue{
				Severity:  "warning",
				Code:      templates.IssueCodeItemsLayoutIgnored,
				Container: templates.ContainerInventory,
				Message:   "sections.inventoryLayout is present but layout apply is not supported in Phase 8D.1; items apply proceeds.",
			})
		}
		if hasStoLayout || tpl.Sections.StorageLayout != nil {
			report.Warnings = append(report.Warnings, templates.ImportPreviewIssue{
				Severity:  "warning",
				Code:      templates.IssueCodeItemsLayoutIgnored,
				Container: templates.ContainerStorage,
				Message:   "sections.storageLayout is present but layout apply is not supported in Phase 8D.1; items apply proceeds.",
			})
		}
		if tpl.ApplyOptions != nil && tpl.ApplyOptions.WeaponLevelOverride != nil {
			report.Warnings = append(report.Warnings, templates.ImportPreviewIssue{
				Severity: "warning",
				Code:     templates.IssueCodeItemsTemplateOverrideIgnored,
				Message:  "applyOptions.weaponLevelOverride is present but Phase 8D.1 only honours the runtime weapon level override; template option ignored.",
			})
		}
	}

	// Phase 8D.1 — plan v2 items additions and preflight capacity
	// BEFORE any mutation. Planning is the same data the apply step
	// consumes; preflight runs against snap.{Inventory,Storage}Items
	// plus planned counts plus pre-existing pass-through records so a
	// "would not fit" diagnosis surfaces without touching the
	// workspace.
	var itemsPlans []v2ItemsPlannedAdd
	var itemsPlanWarnings []templates.ImportPreviewIssue
	if hasItems && tpl.Sections.Items != nil {
		itemsPlans, itemsPlanWarnings = planV2ItemsAddMissing(sess.Workspace, tpl.Sections.Items.Entries)
		invPlanned, stoPlanned := 0, 0
		for _, p := range itemsPlans {
			if p.Container == editor.ContainerStorage {
				stoPlanned++
			} else {
				invPlanned++
			}
		}
		if issues := capacityPreflightV2Items(sess.Workspace, invPlanned, stoPlanned); len(issues) > 0 {
			report.Errors = append(report.Errors, issues...)
			report.OK = false
			return ApplyTemplateV2Result{
				CharIndex: charIdx,
				Preview:   report,
				Applied:   false,
			}, nil
		}
	}

	charVM, err := vm.MapParsedSlotToVM(slot)
	if err != nil {
		rollbackBoth()
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

	// Phase 7b.1 — equipment resolver runs before the inventory.workspace
	// branch. It reads slot.Inventory.CommonItems + slot.GaMap (fresh
	// from LoadSave; combo with inventory.workspace is hard-rejected
	// above so we never see a half-committed workspace here), matches
	// each EquipmentItemRef against the inventory and produces a
	// []core.EquipmentWrite batch. Missing items become warnings; the
	// resolver returns Go errors only for infrastructure failures.
	//
	// The actual SaveSlot.WriteEquipment call happens after the
	// profile/stats slot.Data mutation so the rollback snapshot taken
	// above covers any partial equipment write the same way it covers
	// partial profile/stats writes.
	var equipmentWrites []core.EquipmentWrite
	var equipmentSlotsApplied int
	if hasEquipment {
		activeTalismanSlots := computeActiveTalismanSlots(slot, tpl)
		writes, equipWarn, equipErr := resolveEquipmentWrites(slot, tpl.Selection.Equipment, tpl.Sections.Equipment, activeTalismanSlots)
		if equipErr != nil {
			rollbackBoth()
			return ApplyTemplateV2Result{
				CharIndex: charIdx,
				Preview:   buildApplyErrorReport(report, fmt.Errorf("ApplyBuildTemplateV2: equipment resolver: %w", equipErr)),
				Applied:   false,
			}, nil
		}
		report.Warnings = append(report.Warnings, equipWarn...)
		if len(writes) > 0 {
			equipmentWrites = writes
			applied = append(applied, "equipment")
		}
	}

	// Phase 7d.3 — resolve spell writes ahead of the VM flush, so the
	// resolver's structural / DB-membership warnings land in the report
	// in the same pre-mutation phase as equipment. The actual
	// slot.WriteSpells call happens later, after vm.MapViewModelToSlot
	// and after slot.WriteEquipment, so the spell bytes and hash[10]
	// recompute sit on top of the freshest VM-flushed state.
	var spellWrites []core.SpellWrite
	var spellSlotsApplied int
	if hasSpells {
		writes, spellWarn, spellErr := resolveSpellWrites(slot, tpl.Selection.Spells, tpl.Sections.Spells)
		if spellErr != nil {
			rollbackBoth()
			return ApplyTemplateV2Result{
				CharIndex: charIdx,
				Preview:   buildApplyErrorReport(report, fmt.Errorf("ApplyBuildTemplateV2: spells resolver: %w", spellErr)),
				Applied:   false,
			}, nil
		}
		report.Warnings = append(report.Warnings, spellWarn...)
		if len(writes) > 0 {
			spellWrites = writes
			applied = append(applied, "spells")
		}
	}

	// Phase 7a — inventory.workspace apply runs against the workspace
	// snapshot through the same applyTemplateItemsToWorkspace helper
	// the v1 path uses. Phase 7a.2 threads opts.WeaponLevelOverride into
	// both container calls; a nil / disabled override is a no-op inside
	// applyTemplateItemsToWorkspace (WeaponLevelOverride.HasAny gate).
	var inventoryItemsApplied, storageItemsApplied int
	if hasInventory {
		sec := tpl.Sections.InventoryWorkspace
		if sec != nil {
			invWarn, applyErr := applyTemplateItemsToWorkspace(&sess.Workspace, sec.InventoryItems, editor.ContainerInventory, opts.WeaponLevelOverride)
			if applyErr != nil {
				rollbackBoth()
				return ApplyTemplateV2Result{
					CharIndex: charIdx,
					Preview:   buildApplyErrorReport(report, applyErr),
					Applied:   false,
				}, nil
			}
			stoWarn, applyErr := applyTemplateItemsToWorkspace(&sess.Workspace, sec.StorageItems, editor.ContainerStorage, opts.WeaponLevelOverride)
			if applyErr != nil {
				rollbackBoth()
				return ApplyTemplateV2Result{
					CharIndex: charIdx,
					Preview:   buildApplyErrorReport(report, applyErr),
					Applied:   false,
				}, nil
			}
			report.Warnings = append(report.Warnings, invWarn...)
			report.Warnings = append(report.Warnings, stoWarn...)
			inventoryItemsApplied = len(sec.InventoryItems)
			storageItemsApplied = len(sec.StorageItems)
			if inventoryItemsApplied > 0 || storageItemsApplied > 0 {
				applied = append(applied, "inventory.workspace")
			}
		}
	}

	// Phase 8D.1 — v2 items addMissing apply. Plans were built and
	// preflighted above; here we just execute them through the same
	// editor.AddItem + editor.UpdateWeapon helpers the v1 path uses,
	// then run the runtime weapon level override on every newly added
	// weapon. Skip warnings produced during planning (already-present,
	// unsupported category, "both" location, unknown location) are
	// appended to the report.
	if hasItems && tpl.Sections.Items != nil {
		report.Warnings = append(report.Warnings, itemsPlanWarnings...)
		applyRes, applyErr := executeV2ItemsPlans(&sess.Workspace, itemsPlans, opts.WeaponLevelOverride)
		if applyErr != nil {
			rollbackBoth()
			return ApplyTemplateV2Result{
				CharIndex: charIdx,
				Preview:   buildApplyErrorReport(report, applyErr),
				Applied:   false,
			}, nil
		}
		report.Warnings = append(report.Warnings, applyRes.warnings...)
		inventoryItemsApplied += applyRes.addedInventory
		storageItemsApplied += applyRes.addedStorage
		if applyRes.addedInventory+applyRes.addedStorage > 0 {
			applied = append(applied, "items")
		}
	}

	if len(applied) == 0 {
		// Selection nominated sections but none had anything to write.
		// Workspace untouched (capacity preflight passed for empty
		// items; applyTemplateItemsToWorkspace was a no-op). Profile/
		// stats sections were nil for every selected field. No undo
		// push, no mutation — audit-clean no-op.
		return ApplyTemplateV2Result{
			CharIndex:     charIdx,
			Preview:       report,
			Applied:       false,
			AppliedFields: applied,
			SkippedFields: skipped,
		}, nil
	}

	profileOrStatsApplied := false
	for _, f := range applied {
		if f == "inventory.workspace" || f == "equipment" || f == "spells" || f == "items" {
			continue
		}
		profileOrStatsApplied = true
		break
	}

	clearCountApplied := containsString(applied, "profile.clearCount")
	nameApplied := containsString(applied, "profile.name")
	levelApplied := containsString(applied, "profile.level")

	a.pushUndoLocked(charIdx)

	if profileOrStatsApplied {
		if err := vm.ApplyVMToParsedSlot(charVM, slot); err != nil {
			rollbackBoth()
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
	}

	// Phase 7b.1 — equipment apply. Runs AFTER profile/stats have
	// flushed to slot.Data so any failure here is rolled back by the
	// existing core.SnapshotSlot taken at the top of the slot lock.
	// WriteEquipment writes the 14 supported ChrAsmEquipment slots
	// directly to slot.Data and recomputes the touched hash 7 / 8
	// entries inline; the rollback snapshot covers both.
	if len(equipmentWrites) > 0 {
		if err := slot.WriteEquipment(equipmentWrites); err != nil {
			rollbackBoth()
			report.OK = false
			report.Errors = append(report.Errors, templates.ImportPreviewIssue{
				Severity: "error",
				Code:     templates.IssueCodeEquipmentSlotInvalid,
				Message:  fmt.Sprintf("equipment write rolled back: %s", err.Error()),
			})
			return ApplyTemplateV2Result{
				CharIndex: charIdx,
				Preview:   report,
				Applied:   false,
			}, nil
		}
		equipmentSlotsApplied = len(equipmentWrites)
	}

	// Phase 7d.3 — apply spell writes AFTER vm.MapViewModelToSlot
	// (which already ran above) and AFTER slot.WriteEquipment, so the
	// spell bytes and hash[10] recompute land on top of fully
	// VM-flushed + equipment-written state. Any earlier placement
	// would be overwritten by MapViewModelToSlot. WriteSpells batches
	// PatchEquippedSpell with pre-validation; a non-nil error means no
	// byte was mutated.
	if len(spellWrites) > 0 {
		if err := slot.WriteSpells(spellWrites); err != nil {
			rollbackBoth()
			report.OK = false
			report.Errors = append(report.Errors, templates.ImportPreviewIssue{
				Severity: "error",
				Code:     templates.IssueCodeStructureInvalid,
				Message:  fmt.Sprintf("spell write rolled back: %s", err.Error()),
			})
			return ApplyTemplateV2Result{
				CharIndex: charIdx,
				Preview:   report,
				Applied:   false,
			}, nil
		}
		spellSlotsApplied = len(spellWrites)
	}

	// Phase 7a — mark the workspace dirty + revalidate when items
	// actually landed. Mirrors the v1 applyBuildTemplateToWorkspaceFromJSON
	// success tail so the user still commits by clicking Save changes.
	if hasInventory && (inventoryItemsApplied > 0 || storageItemsApplied > 0) {
		sess.Workspace.Dirty = true
		sess.Workspace.Validation = editor.Validate(sess.Workspace)
	}

	freshVM, err := vm.MapParsedSlotToVM(slot)
	if err != nil {
		rollbackBoth()
		return ApplyTemplateV2Result{CharIndex: charIdx, Preview: report, Applied: false}, fmt.Errorf("ApplyBuildTemplateV2: re-read VM: %w", err)
	}

	result := ApplyTemplateV2Result{
		CharIndex:             charIdx,
		Preview:               report,
		Applied:               true,
		AppliedFields:         applied,
		SkippedFields:         skipped,
		Character:             freshVM,
		InventoryItemsApplied: inventoryItemsApplied,
		SpellSlotsApplied:     spellSlotsApplied,
		StorageItemsApplied:   storageItemsApplied,
		EquipmentSlotsApplied: equipmentSlotsApplied,
	}
	if hasInventory || hasItems {
		ws := sess.Workspace
		result.Workspace = &ws
	}
	return result, nil
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

// ─── Phase 5B — sibling endpoints: from library / from file ───────────

// ApplyBuildTemplateV2FromLibraryToCharacter loads a stored template by
// id from the local library and applies it to slot charIdx via the
// canonical Phase 5A JSON endpoint. Mirrors the v1 ApplyBuildTemplateFromLibrary
// delegation shape (load → marshalBuildTemplate → applyBuildTemplateToWorkspaceFromJSON)
// so all v2 validation, scope guards, locking and rollback live in exactly
// one place.
//
// Behaviour:
//   - Empty / unknown id → non-nil error wrapped by the endpoint name;
//     the library's findEntryLocked is the source of truth.
//   - v1 library entry → delegation rejects with the Phase 5A v1-routing
//     preview error (the v2 endpoint detects tpl.Version == 1).
//   - v2 entry with inventory.workspace in selection → delegation rejects
//     with the Phase 5A scope preview error.
//   - Library load failures are reported as Go errors (consistent with v1).
func (a *App) ApplyBuildTemplateV2FromLibraryToCharacter(charIdx int, id string, opts ApplyTemplateV2Options) (ApplyTemplateV2Result, error) {
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return ApplyTemplateV2Result{CharIndex: charIdx}, fmt.Errorf("ApplyBuildTemplateV2FromLibraryToCharacter: %w", err)
	}
	tpl, err := lib.LoadTemplate(id)
	if err != nil {
		return ApplyTemplateV2Result{CharIndex: charIdx}, fmt.Errorf("ApplyBuildTemplateV2FromLibraryToCharacter: %w", err)
	}
	data, err := marshalBuildTemplate(tpl)
	if err != nil {
		return ApplyTemplateV2Result{CharIndex: charIdx}, fmt.Errorf("ApplyBuildTemplateV2FromLibraryToCharacter: marshal: %w", err)
	}
	return a.ApplyBuildTemplateV2ToCharacterJSON(charIdx, string(data), opts)
}

// ApplyBuildTemplateV2FromFileToCharacter opens a native open-file dialog
// filtered to .yaml/.yml, parses the chosen file as a public YAML
// template, transcodes to canonical JSON, and applies it through the
// Phase 5A JSON endpoint. The dialog and file I/O happen here; all
// validation / scope / locking / rollback is delegated.
//
// Cancellation (empty path) returns a sentinel result (Applied=false,
// cancelledPreviewReport) mirroring ApplyBuildTemplateToWorkspaceFromFile.
// Parse / validation failures land as preview errors rather than Go
// errors so the UI can render "bad YAML" through the same panel as
// "scope-rejected v2 template".
func (a *App) ApplyBuildTemplateV2FromFileToCharacter(charIdx int, opts ApplyTemplateV2Options) (ApplyTemplateV2Result, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Apply Build Template (YAML)",
		Filters: []runtime.FileFilter{
			{DisplayName: "Build Template YAML (*.yaml, *.yml)", Pattern: "*.yaml;*.yml"},
		},
	})
	if err != nil {
		return ApplyTemplateV2Result{CharIndex: charIdx}, err
	}
	if path == "" {
		return cancelledApplyV2Result(charIdx), nil
	}
	return a.applyV2TemplateFromYAMLPath(charIdx, path, opts)
}

// applyV2TemplateFromYAMLPath is the dialog-less core of the file
// endpoint. Split out so tests can drive it with a real path from
// t.TempDir() without going through runtime.OpenFileDialog (which is
// unmockable in a unit test). Behaviour rules:
//
//   - File read errors (missing, too large per maxYAMLImportBytes,
//     unreadable) surface as preview errors with IssueCodeStructureInvalid,
//     not Go errors.
//   - YAML parse / strict-decode / multi-document failures surface as
//     preview errors via the shared ParseBuildTemplateYAML path
//     (anti-TOCTOU: we re-encode the parsed template, never re-read
//     the file).
//   - The canonical JSON is fed verbatim to ApplyBuildTemplateV2ToCharacterJSON
//     so every v2 invariant (version gate, scope, edit-session conflict,
//     inactive slot, snapshot/rollback) is enforced by exactly the same
//     code path as ApplyBuildTemplateV2ToCharacterJSON callers.
func (a *App) applyV2TemplateFromYAMLPath(charIdx int, path string, opts ApplyTemplateV2Options) (ApplyTemplateV2Result, error) {
	data, err := readYAMLFileCapped(path)
	if err != nil {
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   singleErrorPreview(templates.IssueCodeStructureInvalid, err.Error()),
			Applied:   false,
		}, nil
	}
	tpl, err := templates.ParseBuildTemplateYAML(data)
	if err != nil {
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   singleErrorPreview(templates.IssueCodeStructureInvalid, err.Error()),
			Applied:   false,
		}, nil
	}
	canonical, err := marshalBuildTemplate(tpl)
	if err != nil {
		return ApplyTemplateV2Result{
			CharIndex: charIdx,
			Preview:   singleErrorPreview(templates.IssueCodeStructureInvalid, fmt.Sprintf("re-encode parsed YAML as canonical JSON: %s", err.Error())),
			Applied:   false,
		}, nil
	}
	return a.ApplyBuildTemplateV2ToCharacterJSON(charIdx, string(canonical), opts)
}

// cancelledApplyV2Result is the sentinel for "user backed out of the
// Apply file dialog". Mirrors cancelledApplyResult from the v1 path:
// Applied=false, cancelledPreviewReport, CharIndex echoed so the UI
// can correlate the cancel back to the originating request.
func cancelledApplyV2Result(charIdx int) ApplyTemplateV2Result {
	return ApplyTemplateV2Result{
		CharIndex: charIdx,
		Preview:   cancelledPreviewReport(),
		Applied:   false,
	}
}

// computeActiveTalismanSlots returns the effective talisman pouch
// capacity (1..4) the equipment resolver should gate talisman slots
// against. When the template selects profile.talismanSlots and ships a
// value in sections.profile, that value wins because profile apply
// later lifts the persisted pouch count before equipment apply runs;
// otherwise the slot's current persisted Player.TalismanSlots is used.
// Both branches clamp to [0, 3] (MaxProfileTalismanSlots) and add 1 to
// derive the active slot count (the base talisman slot is always
// available even with zero Pouch upgrades).
func computeActiveTalismanSlots(slot *core.SaveSlot, tpl *templates.BuildTemplate) uint8 {
	var base uint8
	if slot != nil {
		base = slot.Player.TalismanSlots
	}
	if base > templates.MaxProfileTalismanSlots {
		base = templates.MaxProfileTalismanSlots
	}
	if tpl != nil && tpl.Selection != nil &&
		tpl.Selection.Profile.Selected("talismanSlots") &&
		tpl.Sections.Profile != nil && tpl.Sections.Profile.TalismanSlots != nil {
		v := *tpl.Sections.Profile.TalismanSlots
		if v > templates.MaxProfileTalismanSlots {
			v = templates.MaxProfileTalismanSlots
		}
		base = v
	}
	return 1 + base
}

// ─── Phase 8D.1 — v2 items addMissing apply helpers ─────────────────────

// v2ItemsMatchKey is the live-identity tuple used by addMissing.
// EntryID is intentionally NOT part of the tuple — it is a portable
// layout reference, not a stable live identity. Two templates with
// different EntryIDs that describe the same baseItemID+upgrade+infusion
// +AoW must collide on the live side.
type v2ItemsMatchKey struct {
	baseItemID     uint32
	currentUpgrade int
	infusionName   string
	aowItemID      uint32
}

// v2ItemsPlannedAdd is one (entry, target container) request produced
// by planV2ItemsAddMissing. A single template entry can yield up to two
// plans when Location="both".
type v2ItemsPlannedAdd struct {
	Entry     templates.TemplateItemEntryV2
	Container editor.ContainerKind
}

// v2ItemsApplyResult collects the runtime counters and post-apply
// warnings (weapon level override clamped / unupgradeable). Planning-
// time skip warnings (already-present, unsupported category, unknown
// location) are returned separately by planV2ItemsAddMissing so the
// caller can emit them even when execution rolls back.
type v2ItemsApplyResult struct {
	addedInventory int
	addedStorage   int
	warnings       []templates.ImportPreviewIssue
}

// v2ItemsContainerTag maps an editor container into the templates
// package's container string (used in ImportPreviewIssue.Container).
func v2ItemsContainerTag(c editor.ContainerKind) string {
	if c == editor.ContainerStorage {
		return templates.ContainerStorage
	}
	return templates.ContainerInventory
}

// v2ItemsKeyForEntry derives the addMissing identity tuple from a
// template entry. UpgradeLevel==nil means "kind=none / unupgradable" →
// level 0. AshOfWarItemID==nil or *0 means "no custom AoW" → 0.
func v2ItemsKeyForEntry(e templates.TemplateItemEntryV2) v2ItemsMatchKey {
	k := v2ItemsMatchKey{
		baseItemID:   e.ItemID,
		infusionName: e.InfusionName,
	}
	if e.UpgradeLevel != nil {
		k.currentUpgrade = int(*e.UpgradeLevel)
	}
	if e.AshOfWarItemID != nil {
		k.aowItemID = *e.AshOfWarItemID
	}
	return k
}

// v2ItemsKeyForLive derives the addMissing identity tuple from a live
// EditableItem. CurrentAoWItemID is only included when the live AoW
// state is "custom" — "missing" and "shared" are abnormal states and
// must not pretend to match a template entry's clean AoW selection.
func v2ItemsKeyForLive(it editor.EditableItem) v2ItemsMatchKey {
	k := v2ItemsMatchKey{
		baseItemID:     it.BaseItemID,
		currentUpgrade: it.CurrentUpgrade,
		infusionName:   it.InfusionName,
	}
	if it.CurrentAoWStatus == editor.AoWStatusCustom {
		k.aowItemID = it.CurrentAoWItemID
	}
	return k
}

// v2ItemsContainersForLocation maps the v2 location string into the
// editor.ContainerKind list the entry will target. Returns nil for
// any value not in the v2 schema allowlist; the validator should have
// caught those earlier, so a nil here is a defense-in-depth path.
func v2ItemsContainersForLocation(loc string) []editor.ContainerKind {
	switch loc {
	case templates.ItemLocationInventory:
		return []editor.ContainerKind{editor.ContainerInventory}
	case templates.ItemLocationStorage:
		return []editor.ContainerKind{editor.ContainerStorage}
	case templates.ItemLocationBoth:
		return []editor.ContainerKind{editor.ContainerInventory, editor.ContainerStorage}
	default:
		return nil
	}
}

// planV2ItemsAddMissing walks the template entries against a static
// snapshot of the live workspace and produces:
//
//   - a list of (entry, container) plans for editor.AddItem;
//   - a list of skip warnings (already-present, unsupported category,
//     unknown location) ready to append to the preview report.
//
// Match set evolves as plans accumulate so two template entries that
// resolve to the same identity tuple collide on the second one (the
// first plan blocks the second from being added). Live snap is read-
// only.
func planV2ItemsAddMissing(snap editor.InventoryWorkspaceSnapshot, entries []templates.TemplateItemEntryV2) ([]v2ItemsPlannedAdd, []templates.ImportPreviewIssue) {
	plans := make([]v2ItemsPlannedAdd, 0, len(entries))
	warnings := make([]templates.ImportPreviewIssue, 0)

	type containerKey struct {
		c editor.ContainerKind
		k v2ItemsMatchKey
	}
	live := map[containerKey]bool{}
	for _, it := range snap.InventoryItems {
		live[containerKey{editor.ContainerInventory, v2ItemsKeyForLive(it)}] = true
	}
	for _, it := range snap.StorageItems {
		live[containerKey{editor.ContainerStorage, v2ItemsKeyForLive(it)}] = true
	}

	for _, e := range entries {
		containers := v2ItemsContainersForLocation(e.Location)
		if len(containers) == 0 {
			warnings = append(warnings, templates.ImportPreviewIssue{
				Severity: "warning",
				Code:     templates.IssueCodeStructureInvalid,
				Message:  fmt.Sprintf("v2 items entry %q: unknown location %q; skipped", e.EntryID, e.Location),
			})
			continue
		}
		key := v2ItemsKeyForEntry(e)
		for _, c := range containers {
			tag := v2ItemsContainerTag(c)
			ck := containerKey{c, key}
			if live[ck] {
				warnings = append(warnings, templates.ImportPreviewIssue{
					Severity:  "warning",
					Code:      templates.IssueCodeItemsAlreadyPresent,
					Container: tag,
					Message: fmt.Sprintf("entry %q (baseItemID=0x%08X, upgrade=%d, infusion=%q, aow=0x%08X) already present; skipped (addMissing)",
						e.EntryID, e.ItemID, key.currentUpgrade, key.infusionName, key.aowItemID),
				})
				continue
			}
			if !editor.SupportedCategories[e.Category] {
				warnings = append(warnings, templates.ImportPreviewIssue{
					Severity:  "warning",
					Code:      templates.IssueCodeUnsupportedCategory,
					Container: tag,
					Message:   fmt.Sprintf("entry %q (category=%q) is not in the Phase 8D.1 editable category allow-list; skipped", e.EntryID, e.Category),
				})
				continue
			}
			plans = append(plans, v2ItemsPlannedAdd{Entry: e, Container: c})
			live[ck] = true
		}
	}
	return plans, warnings
}

// executeV2ItemsPlans runs each planned addition through editor.AddItem,
// applies the per-entry weapon patch (upgrade / infusion / AoW), and
// then runs the runtime weapon level override. Returns a hard error on
// the first editor.* failure so the caller can roll back; soft-skip
// conditions (unsupported category, already present) were already
// resolved during planning.
func executeV2ItemsPlans(snap *editor.InventoryWorkspaceSnapshot, plans []v2ItemsPlannedAdd, override *WeaponLevelOverride) (v2ItemsApplyResult, error) {
	res := v2ItemsApplyResult{warnings: []templates.ImportPreviewIssue{}}
	for _, p := range plans {
		var targetPos int
		if p.Container == editor.ContainerStorage {
			targetPos = len(snap.StorageItems)
		} else {
			targetPos = len(snap.InventoryItems)
		}
		spec := editor.AddItemSpec{
			BaseItemID: p.Entry.ItemID,
			Quantity:   p.Entry.Quantity,
		}
		if err := editor.AddItem(snap, spec, p.Container, targetPos); err != nil {
			return res, fmt.Errorf("AddItem v2 entry %q (baseItemID=0x%08X): %w", p.Entry.EntryID, p.Entry.ItemID, err)
		}
		var added *editor.EditableItem
		if p.Container == editor.ContainerStorage {
			added = &snap.StorageItems[targetPos]
			res.addedStorage++
		} else {
			added = &snap.InventoryItems[targetPos]
			res.addedInventory++
		}
		if added.IsWeapon {
			patch := editor.WeaponPatch{}
			if p.Entry.UpgradeLevel != nil && *p.Entry.UpgradeLevel > 0 {
				patch.SetUpgrade = true
				patch.Upgrade = int(*p.Entry.UpgradeLevel)
			}
			if p.Entry.InfusionName != "" {
				patch.SetInfusionName = true
				patch.InfusionName = p.Entry.InfusionName
			}
			if p.Entry.AshOfWarItemID != nil && *p.Entry.AshOfWarItemID != 0 {
				patch.SetAoWItemID = true
				patch.AoWItemID = *p.Entry.AshOfWarItemID
			}
			if patch.SetUpgrade || patch.SetInfusionName || patch.SetAoWItemID {
				if err := editor.UpdateWeapon(snap, added.UID, patch); err != nil {
					return res, fmt.Errorf("UpdateWeapon v2 entry %q (uid=%s): %w", p.Entry.EntryID, added.UID, err)
				}
			}
			if override.HasAny() {
				w, err := applyWeaponLevelOverride(snap, added, override, p.Container)
				if err != nil {
					return res, fmt.Errorf("WeaponLevelOverride v2 entry %q (uid=%s): %w", p.Entry.EntryID, added.UID, err)
				}
				res.warnings = append(res.warnings, w...)
			}
		}
	}
	return res, nil
}

// capacityPreflightV2Items mirrors capacityPreflight but takes
// already-counted planned additions per container. v2 items planning
// happens BEFORE mutation so we can run the preflight without any
// half-built section struct; the counts come straight from
// planV2ItemsAddMissing.
func capacityPreflightV2Items(snap editor.InventoryWorkspaceSnapshot, invPlanned, stoPlanned int) []templates.ImportPreviewIssue {
	var issues []templates.ImportPreviewIssue
	invTotal := len(snap.InventoryItems) + invPlanned + len(snap.UnsupportedInventoryRecords)
	if invTotal > core.CommonItemCount {
		issues = append(issues, templates.ImportPreviewIssue{
			Severity:  "error",
			Code:      templates.IssueCodeCapacityExceeded,
			Container: templates.ContainerInventory,
			Message: fmt.Sprintf("inventory capacity exceeded: %d existing + %d unsupported + %d v2 items = %d, max %d",
				len(snap.InventoryItems), len(snap.UnsupportedInventoryRecords), invPlanned, invTotal, core.CommonItemCount),
		})
	}
	stoTotal := len(snap.StorageItems) + stoPlanned + len(snap.UnsupportedStorageRecords)
	if stoTotal > core.StorageCommonCount {
		issues = append(issues, templates.ImportPreviewIssue{
			Severity:  "error",
			Code:      templates.IssueCodeCapacityExceeded,
			Container: templates.ContainerStorage,
			Message: fmt.Sprintf("storage capacity exceeded: %d existing + %d unsupported + %d v2 items = %d, max %d",
				len(snap.StorageItems), len(snap.UnsupportedStorageRecords), stoPlanned, stoTotal, core.StorageCommonCount),
		})
	}
	return issues
}
