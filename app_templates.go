package main

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"regexp"
	"strings"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

// BuildTemplateExportOptions is the Wails-facing input for build template
// export. Frontend collects this from the export modal and passes it
// alongside a session ID. Metadata fields are passed through to the
// templates package; container flags drive which sections are emitted.
type BuildTemplateExportOptions struct {
	IncludeInventory bool     `json:"includeInventory"`
	IncludeStorage   bool     `json:"includeStorage"`
	Name             string   `json:"name"`
	Description      string   `json:"description"`
	Author           string   `json:"author"`
	Tags             []string `json:"tags"`
}

// BuildTemplateExportResult is the Wails-facing return. Path is filled
// by the file-writing endpoint; JSON is filled by the JSON-only endpoint
// (used by tests and any future "copy to clipboard" UI affordance).
type BuildTemplateExportResult struct {
	Path         string                    `json:"path,omitempty"`
	JSON         string                    `json:"json,omitempty"`
	Warnings     []templates.ExportWarning `json:"warnings,omitempty"`
	SkippedItems int                       `json:"skippedItems"`
}

// ExportBuildTemplateJSON returns the template payload as a JSON string
// without touching the filesystem. The frontend uses this when it wants
// to preview a payload, paste it into a textbox, or run additional
// validation before asking the user to choose a file path.
//
// Phase B contract:
//   - Operates on the in-memory workspace snapshot of the session ID.
//   - Does NOT call SaveInventoryWorkspaceChanges and does NOT mutate
//     the slot or any session state.
//   - A dirty workspace is a valid export source — exporting before
//     Save is the whole point of the feature.
func (a *App) ExportBuildTemplateJSON(sessionID string, opts BuildTemplateExportOptions) (BuildTemplateExportResult, error) {
	// saveMu.RLock pins a.save for the slot-name read inside
	// sourceCharacterName (called transitively by buildAndValidateTemplate).
	// Released before marshalling — the rest works on local template data
	// and never touches a.save.
	a.saveMu.RLock()
	tpl, report, err := a.buildAndValidateTemplate(sessionID, opts)
	a.saveMu.RUnlock()
	if err != nil {
		return BuildTemplateExportResult{}, err
	}
	data, err := marshalBuildTemplate(tpl)
	if err != nil {
		return BuildTemplateExportResult{}, err
	}
	return BuildTemplateExportResult{
		JSON:     string(data),
		Warnings: report.Warnings,
	}, nil
}

// ExportBuildTemplateToFile shows a native save-file dialog and writes
// the build template JSON to the chosen path. Cancellation surfaces as
// an empty Path + nil error — this matches how the frontend uses
// runtime dialogs elsewhere and is documented in the test cases.
//
// File mode 0644 mirrors the existing ExportCharacterPresetToFile
// pattern; templates are not secrets and may be shared with other
// SaveForge users.
func (a *App) ExportBuildTemplateToFile(sessionID string, opts BuildTemplateExportOptions) (BuildTemplateExportResult, error) {
	// saveMu.RLock around buildAndValidateTemplate only — dialog and
	// disk write run unlocked. See ExportBuildTemplateJSON.
	a.saveMu.RLock()
	tpl, report, err := a.buildAndValidateTemplate(sessionID, opts)
	a.saveMu.RUnlock()
	if err != nil {
		return BuildTemplateExportResult{}, err
	}
	data, err := marshalBuildTemplate(tpl)
	if err != nil {
		return BuildTemplateExportResult{}, err
	}

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export Build Template",
		DefaultFilename: defaultTemplateFilename(tpl),
		Filters: []runtime.FileFilter{
			{DisplayName: "Build Template (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return BuildTemplateExportResult{}, err
	}
	if path == "" {
		// User cancelled the dialog. Surface as a benign empty result
		// rather than an error so the frontend can stay quiet about
		// expected user-initiated cancellation. Warnings are still
		// returned in case the UI wants to display them anyway.
		return BuildTemplateExportResult{Warnings: report.Warnings}, nil
	}

	if err := writeBuildTemplateFile(path, data); err != nil {
		return BuildTemplateExportResult{}, err
	}
	return BuildTemplateExportResult{
		Path:     path,
		Warnings: report.Warnings,
	}, nil
}

// buildAndValidateTemplate is the shared core: resolve session, build
// the template, validate it. Exposed via package-private receiver so
// app_templates_test.go can drive it without going through Wails.
func (a *App) buildAndValidateTemplate(sessionID string, opts BuildTemplateExportOptions) (*templates.BuildTemplate, *templates.ExportReport, error) {
	if !opts.IncludeInventory && !opts.IncludeStorage {
		return nil, nil, fmt.Errorf("ExportBuildTemplate: at least one of includeInventory/includeStorage must be true")
	}
	sess, err := a.acquireSession(sessionID)
	if err != nil {
		return nil, nil, err
	}
	defer sess.Unlock()

	exportOpts := templates.ExportOptions{
		IncludeInventory: opts.IncludeInventory,
		IncludeStorage:   opts.IncludeStorage,
		AppVersion:       appVersion,
		Metadata: &templates.TemplateMetadata{
			Name:                 opts.Name,
			Description:          opts.Description,
			Author:               opts.Author,
			Tags:                 opts.Tags,
			SourceCharacterIndex: sess.CharacterIndex,
			SourceCharacterName:  a.sourceCharacterName(sess.CharacterIndex),
		},
	}

	tpl, report, err := templates.BuildTemplateFromSnapshot(sess.Workspace, exportOpts)
	if err != nil {
		return nil, nil, fmt.Errorf("build template: %w", err)
	}
	if err := templates.ValidateBuildTemplate(tpl); err != nil {
		return nil, nil, fmt.Errorf("validate template: %w", err)
	}
	return tpl, report, nil
}

// sourceCharacterName resolves the in-save display name for a character
// index. Returns empty when no save is loaded or the slot index is out
// of range; the template metadata field is optional and the exporter
// tolerates an empty string.
//
// Contract: caller MUST hold a.saveMu.RLock so the a.save pointer cannot
// be swapped under us. The helper takes slotMu[charIdx] itself for the
// duration of the name read so a concurrent non-session writer (e.g.
// SaveCharacter renaming the character) cannot torn the UTF16 bytes —
// taking slotMu AFTER the caller's saveMu.RLock + sess.mu respects the
// global lock order saveMu → sess.mu → slotMu.
func (a *App) sourceCharacterName(charIdx int) string {
	if a.save == nil || charIdx < 0 || charIdx >= len(a.save.Slots) {
		return ""
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	return core.UTF16ToString(a.save.Slots[charIdx].Player.CharacterName[:])
}

// marshalBuildTemplate is the canonical encoder. Templates ship as
// indented JSON so the file is reasonably diff-friendly for users who
// open it in an editor — these files are meant to be shareable.
func marshalBuildTemplate(tpl *templates.BuildTemplate) ([]byte, error) {
	data, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal template: %w", err)
	}
	return data, nil
}

// writeBuildTemplateFile writes the template payload to disk with the
// same 0644 mode the existing preset exporter uses. Factored out so
// tests can exercise the disk write path without a save dialog.
func writeBuildTemplateFile(path string, data []byte) error {
	if path == "" {
		return fmt.Errorf("writeBuildTemplateFile: empty path")
	}
	if err := os.WriteFile(path, data, 0644); err != nil {
		return fmt.Errorf("write template: %w", err)
	}
	return nil
}

// PreviewBuildTemplateImportJSON runs the Phase C dry-run validator on
// a JSON payload provided directly by the caller (UI textbox, clipboard,
// test harness). It is the building block that
// PreviewBuildTemplateImportFromFile composes with a file read.
//
// Phase C contract:
//   - Does NOT mutate any session, save, or workspace.
//   - Does NOT call AddInventoryWorkspaceItem or
//     SaveInventoryWorkspaceChanges.
//   - Malformed payloads return a non-OK report with a single error
//     coded structure_invalid rather than a flat Go error, so the
//     frontend can render parse failures and validation failures via
//     the same panel.
func (a *App) PreviewBuildTemplateImportJSON(jsonText string) (templates.ImportPreviewReport, error) {
	tpl, err := templates.ParseBuildTemplateJSON([]byte(jsonText))
	if err != nil {
		return templates.ImportPreviewReport{
			OK: false,
			Errors: []templates.ImportPreviewIssue{{
				Severity: "error",
				Code:     templates.IssueCodeStructureInvalid,
				Message:  err.Error(),
			}},
			Warnings: []templates.ImportPreviewIssue{},
			Summary:  templates.ImportPreviewSummary{},
		}, nil
	}
	return templates.PreviewBuildTemplateImport(tpl, templates.ImportPreviewOptions{Mode: "append"}), nil
}

// LoadedTemplatePreview bundles the report with the raw JSON text and
// file path so the UI can pass the same payload to a follow-up Apply
// call without making the user pick the file again. Returned by
// PreviewBuildTemplateImportFromFile; the Apply flow consumes JSON via
// ApplyBuildTemplateToWorkspaceJSON.
type LoadedTemplatePreview struct {
	Report templates.ImportPreviewReport `json:"report"`
	JSON   string                        `json:"json,omitempty"`
	Path   string                        `json:"path,omitempty"`
}

// maxYAMLImportBytes caps the byte length of a public YAML template
// file accepted by PreviewBuildTemplateImportYAMLFromFile. 1 MiB is
// >20× the size of a typical v1 inventory-only template and provides
// a clean prophylactic against malicious or pathological YAML
// payloads. The JSON import path is intentionally NOT capped in this
// phase to avoid changing existing behaviour.
const maxYAMLImportBytes int64 = 1 << 20

// PreviewBuildTemplateImportFromFile opens a native file dialog, reads
// the chosen JSON file, and runs the same dry-run preview as
// PreviewBuildTemplateImportJSON. Cancellation (empty path) is not an
// error: the call returns a sentinel result with empty JSON/Path and
// the zero report so the UI can detect "user backed out" via
// isCancelledPreview on the report's shape.
//
// The JSON text is returned alongside the report so the Apply flow
// (Phase D) can re-use the same payload without re-opening the file
// dialog. Path is informational and used only by the UI for the
// "applied X.json" toast.
func (a *App) PreviewBuildTemplateImportFromFile() (LoadedTemplatePreview, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Preview Build Template",
		Filters: []runtime.FileFilter{
			{DisplayName: "Build Template (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return LoadedTemplatePreview{}, err
	}
	if path == "" {
		return LoadedTemplatePreview{Report: cancelledPreviewReport()}, nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return LoadedTemplatePreview{}, fmt.Errorf("read template: %w", err)
	}
	report, err := a.PreviewBuildTemplateImportJSON(string(data))
	if err != nil {
		return LoadedTemplatePreview{}, err
	}
	return LoadedTemplatePreview{
		Report: report,
		JSON:   string(data),
		Path:   path,
	}, nil
}

// PreviewBuildTemplateImportYAMLFromFile is the YAML twin of
// PreviewBuildTemplateImportFromFile. It opens a native open-file
// dialog filtered to .yaml/.yml, enforces a 1 MiB size cap, parses
// the YAML into a BuildTemplate with strict-mode decoding, and runs
// the same per-item preview validator as the JSON path.
//
// Anti-TOCTOU contract: the returned LoadedTemplatePreview.JSON field
// carries a canonical JSON re-serialisation of the successfully parsed
// template. The frontend is expected to hand exactly those bytes back
// to SaveImportedBuildTemplateJSONToLibrary if the user chooses to
// save the imported template to the local library — there is no
// second file read between Preview and Save, so the file on disk
// cannot be swapped under us.
//
// Cancellation surfaces as a sentinel result with the cancelled
// report and empty JSON/Path, mirroring PreviewBuildTemplateImportFromFile.
// Parse / validation failures are reported as IssueCodeStructureInvalid
// inside the report rather than as a Go error so the UI can render
// "bad YAML" and "bad DB lookup" through the same panel.
func (a *App) PreviewBuildTemplateImportYAMLFromFile() (LoadedTemplatePreview, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Preview Build Template (YAML)",
		Filters: []runtime.FileFilter{
			{DisplayName: "Build Template YAML (*.yaml, *.yml)", Pattern: "*.yaml;*.yml"},
		},
	})
	if err != nil {
		return LoadedTemplatePreview{}, err
	}
	if path == "" {
		return LoadedTemplatePreview{Report: cancelledPreviewReport()}, nil
	}
	data, err := readYAMLFileCapped(path)
	if err != nil {
		return LoadedTemplatePreview{}, err
	}
	return previewYAMLPayload(data, path), nil
}

// previewYAMLPayload runs the YAML parse + structural + per-item
// validators in the same shape as PreviewBuildTemplateImportJSON and
// returns the bundle the frontend stores between Preview and Save.
// Pure function — does not touch any session, save, or filesystem.
func previewYAMLPayload(data []byte, path string) LoadedTemplatePreview {
	tpl, err := templates.ParseBuildTemplateYAML(data)
	if err != nil {
		return LoadedTemplatePreview{
			Report: templates.ImportPreviewReport{
				OK: false,
				Errors: []templates.ImportPreviewIssue{{
					Severity: "error",
					Code:     templates.IssueCodeStructureInvalid,
					Message:  err.Error(),
				}},
				Warnings: []templates.ImportPreviewIssue{},
				Summary:  templates.ImportPreviewSummary{},
			},
			Path: path,
		}
	}
	canonical, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return LoadedTemplatePreview{
			Report: templates.ImportPreviewReport{
				OK: false,
				Errors: []templates.ImportPreviewIssue{{
					Severity: "error",
					Code:     templates.IssueCodeStructureInvalid,
					Message:  fmt.Sprintf("re-encode parsed YAML as canonical JSON: %s", err.Error()),
				}},
				Warnings: []templates.ImportPreviewIssue{},
				Summary:  templates.ImportPreviewSummary{},
			},
			Path: path,
		}
	}
	report := templates.PreviewBuildTemplateImport(tpl, templates.ImportPreviewOptions{Mode: "append"})
	return LoadedTemplatePreview{
		Report: report,
		JSON:   string(canonical),
		Path:   path,
	}
}

// readYAMLFileCapped reads up to maxYAMLImportBytes+1 bytes from path
// and errors out if the file is larger than the cap. The +1 read lets
// us distinguish "exactly at the cap" from "exceeds the cap" without a
// second Stat call.
func readYAMLFileCapped(path string) ([]byte, error) {
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("read template: %w", err)
	}
	defer func() { _ = f.Close() }()
	data, err := readAllUpTo(f, maxYAMLImportBytes+1)
	if err != nil {
		return nil, fmt.Errorf("read template: %w", err)
	}
	if int64(len(data)) > maxYAMLImportBytes {
		return nil, fmt.Errorf("read template: file exceeds %d byte limit for YAML import", maxYAMLImportBytes)
	}
	return data, nil
}

// readAllUpTo drains r until EOF or until limit bytes have been read,
// whichever comes first. Returned slice may be up to limit bytes long.
func readAllUpTo(r io.Reader, limit int64) ([]byte, error) {
	buf := make([]byte, 0, 4096)
	chunk := make([]byte, 4096)
	for int64(len(buf)) < limit {
		remaining := limit - int64(len(buf))
		if remaining < int64(len(chunk)) {
			chunk = chunk[:remaining]
		}
		n, err := r.Read(chunk)
		if n > 0 {
			buf = append(buf, chunk[:n]...)
		}
		if err != nil {
			if err == io.EOF {
				return buf, nil
			}
			return buf, err
		}
	}
	return buf, nil
}

// cancelledPreviewReport is the sentinel returned when the user
// cancelled the open-file dialog. It contains zero items and zero
// issues so the UI can render the "no file chosen" state identically
// to "preview hasn't run yet" without inventing a separate flag.
func cancelledPreviewReport() templates.ImportPreviewReport {
	return templates.ImportPreviewReport{
		OK:       false,
		Errors:   []templates.ImportPreviewIssue{},
		Warnings: []templates.ImportPreviewIssue{},
		Summary:  templates.ImportPreviewSummary{},
	}
}

// ─── Phase D — Apply Build Template to Workspace ────────────────────────

// ApplyTemplateOptions controls the merge strategy when copying template
// items into the workspace. Phase D ships only "append" (the default).
// "replace-inventory" and "replace-all" are reserved.
//
// Phase 6b adds WeaponLevelOverride — an apply-time runtime option that
// overrides upgrade levels for weapons added by this apply (split into
// independent standard / somber selectors). The override is NOT part of
// the template schema; it is the recipient's choice at apply time and is
// applied AFTER the template's own Upgrade/Infusion/AoW patches, only
// inside the active inventory edit session (no slot mutation).
type ApplyTemplateOptions struct {
	Mode                string               `json:"mode,omitempty"`
	WeaponLevelOverride *WeaponLevelOverride `json:"weaponLevelOverride,omitempty"`
}

// WeaponLevelOverride is the Phase 6b apply-time override of upgrade
// levels for weapons added by an inventory template apply. Standard
// (`MaxUpgrade==25`) and Somber (`MaxUpgrade==10`) weapons are addressed
// independently; either pointer may be nil for "leave that class
// unchanged". Enabled=false (the zero value) is a no-op pass-through.
//
// Range is enforced by editor.ClampUpgrade: negative requests collapse
// to 0 and over-max requests clamp to MaxUpgrade with a warning. The
// override only writes the Upgrade field; Infusion and AoW are not
// touched here.
type WeaponLevelOverride struct {
	Enabled       bool `json:"enabled,omitempty"`
	StandardLevel *int `json:"standardLevel,omitempty"`
	SomberLevel   *int `json:"somberLevel,omitempty"`
}

// HasAny reports whether the override would touch any weapon. A nil
// override, an override with Enabled=false, or an enabled override with
// both pointers nil are all no-ops; the apply path short-circuits in
// each case.
func (o *WeaponLevelOverride) HasAny() bool {
	if o == nil || !o.Enabled {
		return false
	}
	return o.StandardLevel != nil || o.SomberLevel != nil
}

// ApplyTemplateResult is the dual-purpose return of an Apply call. When
// the preview reports errors (template invalid, capacity overflow,
// unsupported category), Applied is false and Workspace mirrors the
// pre-apply state. When OK, Applied is true and Workspace is the new
// RAM-only snapshot with Dirty=true. The frontend reads Applied to
// decide between "show preview errors" and "swap workspace state".
type ApplyTemplateResult struct {
	Preview   templates.ImportPreviewReport     `json:"preview"`
	Workspace editor.InventoryWorkspaceSnapshot `json:"workspace"`
	Applied   bool                              `json:"applied"`
}

// ApplyBuildTemplateToWorkspaceJSON parses, validates, and applies a
// build template to the active edit session in-memory.
//
// Phase D contract:
//   - Does NOT call SaveInventoryWorkspaceChanges.
//   - Does NOT mutate slot.Data or the underlying save.
//   - Mutates only sess.Workspace (sets Dirty=true on success).
//   - Failures during apply roll the workspace back to the pre-apply
//     snapshot before returning, so partial states are not observable.
//
// Validation order (each level returns early without mutation when it
// fails):
//  1. Mode whitelist ("" or "append").
//  2. Session exists.
//  3. ParseBuildTemplateJSON (schema/structure).
//  4. PreviewBuildTemplateImport (per-item DB + AoW compat).
//  5. Capacity preflight (inventory + storage container slot caps).
//  6. RAM apply via editor.AddItem and editor.UpdateWeapon.
//
// Returning (ApplyTemplateResult, nil) with Applied=false is the
// expected "blocked by preview" outcome — the Go error channel is
// reserved for unexpected failures (session lookup race, mid-apply
// editor bug). Both shapes carry the current Workspace so the UI can
// always re-render.
func (a *App) ApplyBuildTemplateToWorkspaceJSON(sessionID string, jsonText string, opts ApplyTemplateOptions) (ApplyTemplateResult, error) {
	mode := opts.Mode
	if mode == "" {
		mode = "append"
	}
	if mode != "append" {
		return ApplyTemplateResult{}, fmt.Errorf("ApplyBuildTemplate: unsupported import mode %q (Phase D only ships %q)", mode, "append")
	}
	if err := validateWeaponLevelOverride(opts.WeaponLevelOverride); err != nil {
		return ApplyTemplateResult{}, err
	}

	sess, err := a.acquireSession(sessionID)
	if err != nil {
		return ApplyTemplateResult{}, err
	}
	defer sess.Unlock()

	tpl, err := templates.ParseBuildTemplateJSON([]byte(jsonText))
	if err != nil {
		return ApplyTemplateResult{
			Preview: templates.ImportPreviewReport{
				OK: false,
				Errors: []templates.ImportPreviewIssue{{
					Severity: "error",
					Code:     templates.IssueCodeStructureInvalid,
					Message:  err.Error(),
				}},
				Warnings: []templates.ImportPreviewIssue{},
				Summary:  templates.ImportPreviewSummary{},
			},
			Workspace: sess.Workspace,
			Applied:   false,
		}, nil
	}

	// v2 templates parse cleanly but the apply path below dereferences
	// sec.InventoryItems / sec.StorageItems unconditionally and would
	// panic for profile/stats-only templates. Block until per-section
	// apply ships.
	if tpl.Version > templates.SchemaVersion {
		return ApplyTemplateResult{
			Preview: templates.ImportPreviewReport{
				OK: false,
				Errors: []templates.ImportPreviewIssue{{
					Severity: "error",
					Code:     templates.IssueCodeStructureInvalid,
					Message:  fmt.Sprintf("apply of schema v%d templates is not yet supported in this phase", tpl.Version),
				}},
				Warnings: []templates.ImportPreviewIssue{},
				Summary:  templates.ImportPreviewSummary{},
			},
			Workspace: sess.Workspace,
			Applied:   false,
		}, nil
	}

	report := templates.PreviewBuildTemplateImport(tpl, templates.ImportPreviewOptions{Mode: mode})
	if !report.OK {
		return ApplyTemplateResult{
			Preview:   report,
			Workspace: sess.Workspace,
			Applied:   false,
		}, nil
	}

	sec := tpl.Sections.InventoryWorkspace
	if capacityIssues := capacityPreflight(sess.Workspace, sec); len(capacityIssues) > 0 {
		report.Errors = append(report.Errors, capacityIssues...)
		report.OK = false
		return ApplyTemplateResult{
			Preview:   report,
			Workspace: sess.Workspace,
			Applied:   false,
		}, nil
	}

	// Snapshot for rollback. EditableItem and RawInventoryRecord are
	// value types — a shallow slice copy is enough to make a snapshot
	// the apply path cannot accidentally mutate through.
	backup := deepCopySnapshot(sess.Workspace)

	invWarnings, err := applyTemplateItemsToWorkspace(&sess.Workspace, sec.InventoryItems, editor.ContainerInventory, opts.WeaponLevelOverride)
	if err != nil {
		sess.Workspace = backup
		return ApplyTemplateResult{
			Preview:   buildApplyErrorReport(report, err),
			Workspace: backup,
			Applied:   false,
		}, nil
	}
	stoWarnings, err := applyTemplateItemsToWorkspace(&sess.Workspace, sec.StorageItems, editor.ContainerStorage, opts.WeaponLevelOverride)
	if err != nil {
		sess.Workspace = backup
		return ApplyTemplateResult{
			Preview:   buildApplyErrorReport(report, err),
			Workspace: backup,
			Applied:   false,
		}, nil
	}
	report.Warnings = append(report.Warnings, invWarnings...)
	report.Warnings = append(report.Warnings, stoWarnings...)

	sess.Workspace.Dirty = true
	sess.Workspace.Validation = editor.Validate(sess.Workspace)
	return ApplyTemplateResult{
		Preview:   report,
		Workspace: sess.Workspace,
		Applied:   true,
	}, nil
}

// validateWeaponLevelOverride rejects shape-invalid override payloads
// before the apply path is entered. A nil override, an override with
// Enabled=false, or any well-formed enabled override is accepted; only
// a structurally broken request (enabled with nothing to do, or a
// negative level) returns an error. Out-of-range positive levels are
// allowed at this layer because editor.ClampUpgrade truncates them
// against the per-weapon MaxUpgrade and emits a clamped-warning later.
func validateWeaponLevelOverride(o *WeaponLevelOverride) error {
	if o == nil || !o.Enabled {
		return nil
	}
	if o.StandardLevel == nil && o.SomberLevel == nil {
		return fmt.Errorf("ApplyBuildTemplate: weaponLevelOverride.enabled=true requires at least one of standardLevel / somberLevel")
	}
	if o.StandardLevel != nil && *o.StandardLevel < 0 {
		return fmt.Errorf("ApplyBuildTemplate: weaponLevelOverride.standardLevel = %d (must be >= 0)", *o.StandardLevel)
	}
	if o.SomberLevel != nil && *o.SomberLevel < 0 {
		return fmt.Errorf("ApplyBuildTemplate: weaponLevelOverride.somberLevel = %d (must be >= 0)", *o.SomberLevel)
	}
	return nil
}

// ApplyBuildTemplateToWorkspaceFromFile opens a file dialog and applies
// the chosen template to the workspace. Cancellation (empty path) is a
// non-error sentinel: Applied=false, no preview content. Mirrors the
// cancel UX of PreviewBuildTemplateImportFromFile.
func (a *App) ApplyBuildTemplateToWorkspaceFromFile(sessionID string, opts ApplyTemplateOptions) (ApplyTemplateResult, error) {
	// Cheap existence probe under the registry lock so we can fail
	// before opening the native file dialog. The actual apply path
	// (ApplyBuildTemplateToWorkspaceJSON) re-acquires the session under
	// its own per-session lock; we deliberately do NOT hold either lock
	// across the dialog.
	a.editSessionsMu.Lock()
	_, ok := a.editSessions[sessionID]
	a.editSessionsMu.Unlock()
	if !ok {
		return ApplyTemplateResult{}, fmt.Errorf("inventory edit session %q not found", sessionID)
	}
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Apply Build Template",
		Filters: []runtime.FileFilter{
			{DisplayName: "Build Template (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return ApplyTemplateResult{}, err
	}
	if path == "" {
		return cancelledApplyResult(a, sessionID), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return ApplyTemplateResult{}, fmt.Errorf("read template: %w", err)
	}
	return a.ApplyBuildTemplateToWorkspaceJSON(sessionID, string(data), opts)
}

// cancelledApplyResult is the sentinel for "user backed out of the
// Apply file dialog". Mirrors cancelledPreviewReport's contract:
// Applied=false, empty preview, current workspace echoed so the UI can
// stay in lock-step.
func cancelledApplyResult(a *App, sessionID string) ApplyTemplateResult {
	res := ApplyTemplateResult{
		Preview: cancelledPreviewReport(),
		Applied: false,
	}
	// Read the workspace echo under the per-session lock — Discard or a
	// concurrent Save could otherwise tear the snapshot mid-copy.
	if sess, err := a.acquireSession(sessionID); err == nil {
		res.Workspace = sess.Workspace
		sess.Unlock()
	}
	return res
}

// applyTemplateItemsToWorkspace iterates one section's items and calls
// the existing workspace mutation helpers in order. Append mode: each
// new item lands at len(target_container_after_previous_inserts), so
// the relative order from the template is preserved and existing
// workspace items stay in front.
//
// On the first error this function returns; the caller is responsible
// for rolling back the workspace because partial mutations are visible
// in snap until restoration.
func applyTemplateItemsToWorkspace(snap *editor.InventoryWorkspaceSnapshot, items []templates.TemplateItem, container editor.ContainerKind, override *WeaponLevelOverride) ([]templates.ImportPreviewIssue, error) {
	var warnings []templates.ImportPreviewIssue
	for _, t := range items {
		var targetPos int
		if container == editor.ContainerStorage {
			targetPos = len(snap.StorageItems)
		} else {
			targetPos = len(snap.InventoryItems)
		}

		spec := editor.AddItemSpec{
			BaseItemID: t.BaseItemID,
			Quantity:   t.Quantity,
		}
		if err := editor.AddItem(snap, spec, container, targetPos); err != nil {
			return warnings, fmt.Errorf("AddItem %q (baseItemID=0x%08X): %w", t.Name, t.BaseItemID, err)
		}

		var added *editor.EditableItem
		if container == editor.ContainerStorage {
			added = &snap.StorageItems[targetPos]
		} else {
			added = &snap.InventoryItems[targetPos]
		}
		if !added.IsWeapon {
			continue
		}

		needsUpgrade := t.Upgrade > 0
		needsInfusion := t.InfusionName != ""
		needsAoW := t.AoWItemID != nil && *t.AoWItemID != 0
		if needsUpgrade || needsInfusion || needsAoW {
			patch := editor.WeaponPatch{}
			if needsUpgrade {
				patch.SetUpgrade = true
				patch.Upgrade = t.Upgrade
			}
			if needsInfusion {
				patch.SetInfusionName = true
				patch.InfusionName = t.InfusionName
			}
			if needsAoW {
				patch.SetAoWItemID = true
				patch.AoWItemID = *t.AoWItemID
			}
			if err := editor.UpdateWeapon(snap, added.UID, patch); err != nil {
				return warnings, fmt.Errorf("UpdateWeapon %q (uid=%s): %w", t.Name, added.UID, err)
			}
		}

		if override.HasAny() {
			w, err := applyWeaponLevelOverride(snap, added, override, container)
			if err != nil {
				return warnings, fmt.Errorf("WeaponLevelOverride %q (uid=%s): %w", t.Name, added.UID, err)
			}
			warnings = append(warnings, w...)
		}
	}
	return warnings, nil
}

// applyWeaponLevelOverride is the Phase 6b runtime override step. It
// runs AFTER the template's own Upgrade/Infusion/AoW patch so the
// recipient's choice is the last word on the weapon's upgrade level.
// MaxUpgrade==25 means a standard weapon, MaxUpgrade==10 means a
// somber/special weapon, MaxUpgrade==0 means the weapon is not
// upgradeable at all. Anything else (DLC weirdness, unknown DB entry
// surfacing as a non-canonical cap) is treated as "skip silently" — the
// preview validator already rejects truly unknown items earlier, so by
// the time we reach this point a non-canonical MaxUpgrade is a
// "category we don't address in the override" situation, not a bug
// worth surfacing.
//
// The container argument is purely informational — it is threaded into
// warnings so the UI can show "skipped storage weapon X" with the right
// Container tag, mirroring the existing apply-side issue surface.
func applyWeaponLevelOverride(snap *editor.InventoryWorkspaceSnapshot, added *editor.EditableItem, override *WeaponLevelOverride, container editor.ContainerKind) ([]templates.ImportPreviewIssue, error) {
	var warnings []templates.ImportPreviewIssue
	containerTag := templates.ContainerInventory
	if container == editor.ContainerStorage {
		containerTag = templates.ContainerStorage
	}

	switch added.MaxUpgrade {
	case 25:
		if override.StandardLevel == nil {
			return warnings, nil
		}
		requested := *override.StandardLevel
		clamped := editor.ClampUpgrade(requested, added.MaxUpgrade)
		if err := editor.UpdateWeapon(snap, added.UID, editor.WeaponPatch{SetUpgrade: true, Upgrade: clamped}); err != nil {
			return warnings, err
		}
		if requested != clamped {
			warnings = append(warnings, templates.ImportPreviewIssue{
				Severity:  "warning",
				Code:      templates.IssueCodeWeaponLevelClamped,
				Container: containerTag,
				Message: fmt.Sprintf("standard weapon %q upgrade override clamped from +%d to +%d (max +%d)",
					added.Name, requested, clamped, added.MaxUpgrade),
			})
		}
	case 10:
		if override.SomberLevel == nil {
			return warnings, nil
		}
		requested := *override.SomberLevel
		clamped := editor.ClampUpgrade(requested, added.MaxUpgrade)
		if err := editor.UpdateWeapon(snap, added.UID, editor.WeaponPatch{SetUpgrade: true, Upgrade: clamped}); err != nil {
			return warnings, err
		}
		if requested != clamped {
			warnings = append(warnings, templates.ImportPreviewIssue{
				Severity:  "warning",
				Code:      templates.IssueCodeWeaponLevelClamped,
				Container: containerTag,
				Message: fmt.Sprintf("somber weapon %q upgrade override clamped from +%d to +%d (max +%d)",
					added.Name, requested, clamped, added.MaxUpgrade),
			})
		}
	case 0:
		warnings = append(warnings, templates.ImportPreviewIssue{
			Severity:  "warning",
			Code:      templates.IssueCodeWeaponUnupgradeable,
			Container: containerTag,
			Message:   fmt.Sprintf("weapon %q is not upgradeable; override skipped", added.Name),
		})
	}
	return warnings, nil
}

// deepCopySnapshot returns a snapshot whose mutable slices are
// independent from the source. EditableItem and RawInventoryRecord are
// value structs, so a copy of the slice headers + element copies is a
// full deep copy for rollback purposes. Validation carries its own
// slices; we copy those too so a later editor.Validate() on the rolled
// back snap does not see stale state.
func deepCopySnapshot(snap editor.InventoryWorkspaceSnapshot) editor.InventoryWorkspaceSnapshot {
	cp := snap
	cp.InventoryItems = append([]editor.EditableItem{}, snap.InventoryItems...)
	cp.StorageItems = append([]editor.EditableItem{}, snap.StorageItems...)
	cp.UnsupportedInventoryRecords = append([]editor.RawInventoryRecord{}, snap.UnsupportedInventoryRecords...)
	cp.UnsupportedStorageRecords = append([]editor.RawInventoryRecord{}, snap.UnsupportedStorageRecords...)
	cp.Validation.Errors = append([]editor.WorkspaceValidationIssue{}, snap.Validation.Errors...)
	cp.Validation.Warnings = append([]editor.WorkspaceValidationIssue{}, snap.Validation.Warnings...)
	cp.Validation.DuplicateUIDs = append([]string{}, snap.Validation.DuplicateUIDs...)
	cp.Validation.DuplicateHandles = append([]uint32{}, snap.Validation.DuplicateHandles...)
	return cp
}

// capacityPreflight enforces the per-container slot caps. Phase D only
// checks coarse container capacity (matches the in-save Inventory /
// Storage record-array sizes); the much tighter GaItem-zone capacity
// is still enforced later by SaveInventoryWorkspaceChanges where the
// real handle allocator runs.
func capacityPreflight(snap editor.InventoryWorkspaceSnapshot, sec *templates.InventoryWorkspaceSection) []templates.ImportPreviewIssue {
	var issues []templates.ImportPreviewIssue
	if sec == nil {
		return issues
	}
	invTotal := len(snap.InventoryItems) + len(sec.InventoryItems) + len(snap.UnsupportedInventoryRecords)
	if invTotal > core.CommonItemCount {
		issues = append(issues, templates.ImportPreviewIssue{
			Severity:  "error",
			Code:      templates.IssueCodeCapacityExceeded,
			Container: templates.ContainerInventory,
			Message: fmt.Sprintf("inventory capacity exceeded: %d existing + %d unsupported + %d imported = %d, max %d",
				len(snap.InventoryItems), len(snap.UnsupportedInventoryRecords), len(sec.InventoryItems), invTotal, core.CommonItemCount),
		})
	}
	stoTotal := len(snap.StorageItems) + len(sec.StorageItems) + len(snap.UnsupportedStorageRecords)
	if stoTotal > core.StorageCommonCount {
		issues = append(issues, templates.ImportPreviewIssue{
			Severity:  "error",
			Code:      templates.IssueCodeCapacityExceeded,
			Container: templates.ContainerStorage,
			Message: fmt.Sprintf("storage capacity exceeded: %d existing + %d unsupported + %d imported = %d, max %d",
				len(snap.StorageItems), len(snap.UnsupportedStorageRecords), len(sec.StorageItems), stoTotal, core.StorageCommonCount),
		})
	}
	return issues
}

// buildApplyErrorReport tags a rollback-causing apply failure onto the
// previously-OK preview report. The preview is still useful (the user
// might want to see what would have happened); we just tack on the
// stop-reason as an additional error.
func buildApplyErrorReport(report templates.ImportPreviewReport, applyErr error) templates.ImportPreviewReport {
	report.OK = false
	report.Errors = append(report.Errors, templates.ImportPreviewIssue{
		Severity: "error",
		Code:     templates.IssueCodeUnsupportedCategory,
		Message:  fmt.Sprintf("apply failed mid-way and was rolled back: %s", applyErr.Error()),
	})
	return report
}

// ─── Phase E — Local Build Template Library ─────────────────────────────

// ensureTemplateLibrary lazily creates the on-disk library handle. The
// default rootDir is $UserConfigDir/EldenRing-SaveEditor/templates;
// tests may inject an alternate library by setting a.templateLibrary
// directly before invoking endpoints.
func (a *App) ensureTemplateLibrary() (*templates.TemplateLibrary, error) {
	if a.templateLibrary != nil {
		return a.templateLibrary, nil
	}
	dir, err := templates.DefaultTemplateLibraryDir()
	if err != nil {
		return nil, fmt.Errorf("template library: %w", err)
	}
	a.templateLibrary = templates.NewTemplateLibrary(dir)
	return a.templateLibrary, nil
}

// SaveBuildTemplateToLibrary builds a template from the active workspace
// session (same code path as ExportBuildTemplateJSON) and stores it in
// the local library. Returns the new index entry. Workspace and save
// are untouched.
func (a *App) SaveBuildTemplateToLibrary(sessionID string, opts BuildTemplateExportOptions) (templates.LibraryTemplateEntry, error) {
	// saveMu.RLock around buildAndValidateTemplate only — library write
	// runs unlocked. See ExportBuildTemplateJSON.
	a.saveMu.RLock()
	tpl, _, err := a.buildAndValidateTemplate(sessionID, opts)
	a.saveMu.RUnlock()
	if err != nil {
		return templates.LibraryTemplateEntry{}, err
	}
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return templates.LibraryTemplateEntry{}, err
	}
	return lib.SaveTemplate(tpl)
}

// ListBuildTemplateLibrary returns the index entries sorted newest-first.
func (a *App) ListBuildTemplateLibrary() ([]templates.LibraryTemplateEntry, error) {
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return nil, err
	}
	return lib.ListTemplates()
}

// PreviewBuildTemplateFromLibrary loads a stored template by ID and
// runs the same dry-run validator as PreviewBuildTemplateImportJSON.
// The JSON payload is included so the Apply flow can re-use it without
// a second library read. Path is the on-disk filename relative to the
// library root.
func (a *App) PreviewBuildTemplateFromLibrary(id string) (LoadedTemplatePreview, error) {
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return LoadedTemplatePreview{}, err
	}
	tpl, err := lib.LoadTemplate(id)
	if err != nil {
		return LoadedTemplatePreview{}, fmt.Errorf("PreviewBuildTemplateFromLibrary: %w", err)
	}
	data, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return LoadedTemplatePreview{}, fmt.Errorf("PreviewBuildTemplateFromLibrary: marshal: %w", err)
	}
	report := templates.PreviewBuildTemplateImport(tpl, templates.ImportPreviewOptions{Mode: "append"})
	return LoadedTemplatePreview{
		Report: report,
		JSON:   string(data),
		Path:   id,
	}, nil
}

// ApplyBuildTemplateFromLibrary loads a stored template and applies it
// to the active workspace via the existing JSON-based apply path.
// RAM-only — save state is not touched. The caller still has to invoke
// SaveInventoryWorkspaceChanges separately to persist.
func (a *App) ApplyBuildTemplateFromLibrary(sessionID, id string, opts ApplyTemplateOptions) (ApplyTemplateResult, error) {
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return ApplyTemplateResult{}, err
	}
	tpl, err := lib.LoadTemplate(id)
	if err != nil {
		return ApplyTemplateResult{}, fmt.Errorf("ApplyBuildTemplateFromLibrary: %w", err)
	}
	data, err := json.MarshalIndent(tpl, "", "  ")
	if err != nil {
		return ApplyTemplateResult{}, fmt.Errorf("ApplyBuildTemplateFromLibrary: marshal: %w", err)
	}
	return a.ApplyBuildTemplateToWorkspaceJSON(sessionID, string(data), opts)
}

// DeleteBuildTemplateFromLibrary removes a template from the library.
// Workspace and save are not touched.
func (a *App) DeleteBuildTemplateFromLibrary(id string) error {
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return err
	}
	return lib.DeleteTemplate(id)
}

// RenameBuildTemplateInLibrary updates Name/Description/Tags on a stored
// template's metadata + index entry. Returns the updated entry.
func (a *App) RenameBuildTemplateInLibrary(id, name, description string, tags []string) (templates.LibraryTemplateEntry, error) {
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return templates.LibraryTemplateEntry{}, err
	}
	if tags == nil {
		tags = []string{}
	}
	return lib.RenameTemplate(id, name, description, tags)
}

// RebuildBuildTemplateLibraryIndex rescans the library directory and
// rewrites _index.json from the template files on disk. Files that
// fail to parse or validate are silently skipped — they remain on
// disk but do not appear in the new index. The post-rebuild list is
// returned so the UI can refresh in a single round-trip.
//
// Use case: the user manually dropped a template JSON into the
// library folder, or copied templates from another machine. Rebuild
// makes those visible without restarting the app.
//
// Workspace and save state are untouched.
func (a *App) RebuildBuildTemplateLibraryIndex() ([]templates.LibraryTemplateEntry, error) {
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return nil, err
	}
	if err := lib.RebuildIndex(); err != nil {
		return nil, fmt.Errorf("RebuildBuildTemplateLibraryIndex: %w", err)
	}
	return lib.ListTemplates()
}

// GetBuildTemplateLibraryPath returns the on-disk directory the
// library reads from / writes to. The UI surfaces this in the
// empty-state copy and as a footer so users can find the folder for
// manual file management. Lazy-initialises the library handle so the
// directory exists by the time the path is returned.
func (a *App) GetBuildTemplateLibraryPath() (string, error) {
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return "", err
	}
	return lib.RootDir(), nil
}

// ExportLibraryBuildTemplateToFile loads a stored template, opens a
// native save-file dialog, and writes the chosen path. Cancellation
// surfaces as an empty Path + nil error (mirrors ExportBuildTemplateToFile).
func (a *App) ExportLibraryBuildTemplateToFile(id string) (BuildTemplateExportResult, error) {
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return BuildTemplateExportResult{}, err
	}
	tpl, err := lib.LoadTemplate(id)
	if err != nil {
		return BuildTemplateExportResult{}, fmt.Errorf("ExportLibraryBuildTemplateToFile: %w", err)
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export Build Template",
		DefaultFilename: defaultTemplateFilename(tpl),
		Filters: []runtime.FileFilter{
			{DisplayName: "Build Template (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return BuildTemplateExportResult{}, err
	}
	if path == "" {
		return BuildTemplateExportResult{}, nil
	}
	if err := lib.ExportTemplateToFile(id, path); err != nil {
		return BuildTemplateExportResult{}, err
	}
	return BuildTemplateExportResult{Path: path}, nil
}

// ExportLibraryBuildTemplateAsYAMLToFile is the YAML twin of
// ExportLibraryBuildTemplateToFile. The library entry on disk is not
// touched — this only writes a second, public-share-friendly YAML copy
// to the user-chosen path. Cancellation surfaces as an empty Path + nil
// error, matching the JSON helper.
func (a *App) ExportLibraryBuildTemplateAsYAMLToFile(id string) (BuildTemplateExportResult, error) {
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return BuildTemplateExportResult{}, err
	}
	tpl, err := lib.LoadTemplate(id)
	if err != nil {
		return BuildTemplateExportResult{}, fmt.Errorf("ExportLibraryBuildTemplateAsYAMLToFile: %w", err)
	}
	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title:           "Export Build Template (YAML)",
		DefaultFilename: defaultTemplateFilenameYAML(tpl),
		Filters: []runtime.FileFilter{
			{DisplayName: "Build Template YAML (*.yaml, *.yml)", Pattern: "*.yaml;*.yml"},
		},
	})
	if err != nil {
		return BuildTemplateExportResult{}, err
	}
	if path == "" {
		return BuildTemplateExportResult{}, nil
	}
	if err := lib.ExportTemplateToYAMLFile(id, path); err != nil {
		return BuildTemplateExportResult{}, err
	}
	return BuildTemplateExportResult{Path: path}, nil
}

// SaveImportedBuildTemplateJSONToLibrary persists a previously-previewed
// template payload to the local library. The caller is expected to pass
// the canonical JSON returned by either PreviewBuildTemplateImportJSON,
// PreviewBuildTemplateImportFromFile, or PreviewBuildTemplateImportYAMLFromFile.
//
// Anti-TOCTOU: parsing+validation re-runs against the exact bytes the
// frontend held since Preview, so the file on disk (for the YAML import
// path) cannot be swapped under us. The endpoint also re-runs the
// per-item DB / AoW preview so that a template that became invalid
// between Preview and Save (e.g. DB hotpatch) is refused.
//
// On disk the library stays JSON-internal: the parsed BuildTemplate is
// handed straight to Library.SaveTemplate, which writes JSON via the
// existing atomicWriteFile path. _index.json is updated through the
// same mechanism. No sessionID required — this is a global, workspace-
// independent operation.
func (a *App) SaveImportedBuildTemplateJSONToLibrary(jsonText string) (templates.LibraryTemplateEntry, error) {
	tpl, err := templates.ParseBuildTemplateJSON([]byte(jsonText))
	if err != nil {
		return templates.LibraryTemplateEntry{}, fmt.Errorf("SaveImportedBuildTemplateJSONToLibrary: %w", err)
	}
	report := templates.PreviewBuildTemplateImport(tpl, templates.ImportPreviewOptions{Mode: "append"})
	if !report.OK {
		return templates.LibraryTemplateEntry{}, fmt.Errorf("SaveImportedBuildTemplateJSONToLibrary: imported template has %d blocking issue(s); refusing to save", len(report.Errors))
	}
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return templates.LibraryTemplateEntry{}, err
	}
	return lib.SaveTemplate(tpl)
}

// filenameSanitizer strips anything that would be unsafe in a filename
// across the three desktop OSes. Sequences of unsafe characters collapse
// to a single dash, and surrounding dashes/whitespace are trimmed.
var filenameSanitizer = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

// defaultTemplateFilename picks the default name shown in the system
// save dialog. Preference order: user-provided template name, source
// character name, generic fallback. Always ends with .json.
func defaultTemplateFilename(tpl *templates.BuildTemplate) string {
	var stem string
	if tpl != nil && tpl.Metadata != nil {
		if tpl.Metadata.Name != "" {
			stem = tpl.Metadata.Name
		} else if tpl.Metadata.SourceCharacterName != "" {
			stem = tpl.Metadata.SourceCharacterName + "-build"
		}
	}
	if stem == "" {
		return "saveforge-build-template.json"
	}
	stem = filenameSanitizer.ReplaceAllString(stem, "-")
	stem = strings.Trim(stem, "-_")
	if stem == "" {
		return "saveforge-build-template.json"
	}
	return stem + ".json"
}

// defaultTemplateFilenameYAML mirrors defaultTemplateFilename but
// produces a .yaml suffix for the public YAML export endpoint.
func defaultTemplateFilenameYAML(tpl *templates.BuildTemplate) string {
	base := defaultTemplateFilename(tpl)
	return strings.TrimSuffix(base, ".json") + ".yaml"
}
