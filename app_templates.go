package main

import (
	"encoding/json"
	"fmt"
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
	Path         string                     `json:"path,omitempty"`
	JSON         string                     `json:"json,omitempty"`
	Warnings     []templates.ExportWarning  `json:"warnings,omitempty"`
	SkippedItems int                        `json:"skippedItems"`
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
	tpl, report, err := a.buildAndValidateTemplate(sessionID, opts)
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
	tpl, report, err := a.buildAndValidateTemplate(sessionID, opts)
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
	sess, ok := a.editSessions[sessionID]
	if !ok {
		return nil, nil, fmt.Errorf("inventory edit session %q not found", sessionID)
	}

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
func (a *App) sourceCharacterName(charIdx int) string {
	if a.save == nil || charIdx < 0 || charIdx >= len(a.save.Slots) {
		return ""
	}
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
type ApplyTemplateOptions struct {
	Mode string `json:"mode,omitempty"`
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
//   1. Mode whitelist ("" or "append").
//   2. Session exists.
//   3. ParseBuildTemplateJSON (schema/structure).
//   4. PreviewBuildTemplateImport (per-item DB + AoW compat).
//   5. Capacity preflight (inventory + storage container slot caps).
//   6. RAM apply via editor.AddItem and editor.UpdateWeapon.
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

	sess, ok := a.editSessions[sessionID]
	if !ok {
		return ApplyTemplateResult{}, fmt.Errorf("inventory edit session %q not found", sessionID)
	}

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

	if err := applyTemplateItemsToWorkspace(&sess.Workspace, sec.InventoryItems, editor.ContainerInventory); err != nil {
		sess.Workspace = backup
		return ApplyTemplateResult{
			Preview:   buildApplyErrorReport(report, err),
			Workspace: backup,
			Applied:   false,
		}, nil
	}
	if err := applyTemplateItemsToWorkspace(&sess.Workspace, sec.StorageItems, editor.ContainerStorage); err != nil {
		sess.Workspace = backup
		return ApplyTemplateResult{
			Preview:   buildApplyErrorReport(report, err),
			Workspace: backup,
			Applied:   false,
		}, nil
	}

	sess.Workspace.Dirty = true
	sess.Workspace.Validation = editor.Validate(sess.Workspace)
	return ApplyTemplateResult{
		Preview:   report,
		Workspace: sess.Workspace,
		Applied:   true,
	}, nil
}

// ApplyBuildTemplateToWorkspaceFromFile opens a file dialog and applies
// the chosen template to the workspace. Cancellation (empty path) is a
// non-error sentinel: Applied=false, no preview content. Mirrors the
// cancel UX of PreviewBuildTemplateImportFromFile.
func (a *App) ApplyBuildTemplateToWorkspaceFromFile(sessionID string, opts ApplyTemplateOptions) (ApplyTemplateResult, error) {
	if _, ok := a.editSessions[sessionID]; !ok {
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
	if sess, ok := a.editSessions[sessionID]; ok {
		res.Workspace = sess.Workspace
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
func applyTemplateItemsToWorkspace(snap *editor.InventoryWorkspaceSnapshot, items []templates.TemplateItem, container editor.ContainerKind) error {
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
			return fmt.Errorf("AddItem %q (baseItemID=0x%08X): %w", t.Name, t.BaseItemID, err)
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
		if !needsUpgrade && !needsInfusion && !needsAoW {
			continue
		}
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
			return fmt.Errorf("UpdateWeapon %q (uid=%s): %w", t.Name, added.UID, err)
		}
	}
	return nil
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
	tpl, _, err := a.buildAndValidateTemplate(sessionID, opts)
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

