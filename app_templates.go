package main

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
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

// PreviewBuildTemplateImportFromFile opens a native file dialog, reads
// the chosen JSON file, and runs the same dry-run preview as
// PreviewBuildTemplateImportJSON. Cancellation (empty path) is not an
// error: the call returns OK=false with no issues so the UI can detect
// "user backed out" via report.Summary.InventoryItems == 0 + zero
// errors. Match the convention of ExportBuildTemplateToFile's silent
// cancel.
func (a *App) PreviewBuildTemplateImportFromFile() (templates.ImportPreviewReport, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Preview Build Template",
		Filters: []runtime.FileFilter{
			{DisplayName: "Build Template (*.json)", Pattern: "*.json"},
		},
	})
	if err != nil {
		return templates.ImportPreviewReport{}, err
	}
	if path == "" {
		return cancelledPreviewReport(), nil
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return templates.ImportPreviewReport{}, fmt.Errorf("read template: %w", err)
	}
	return a.PreviewBuildTemplateImportJSON(string(data))
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

