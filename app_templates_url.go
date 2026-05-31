package main

import (
	"context"
	"strings"

	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// PreviewBuildTemplateImportYAMLFromURL fetches a public Templates v2
// YAML from rawURL under the SSRF guards documented in spec/56-templates-v2.md §12
// (Phase 9) and returns the same LoadedTemplatePreview bundle the
// file-based path returns. The frontend then:
//   - displays the canonical final URL as the source label,
//   - reuses the existing ImportTemplatePreviewModal,
//   - drives Save to Library / Apply to character / Apply with
//     overrides through the same canonical-JSON path that the file
//     import already uses.
//
// Anti-mutation contract: this call never writes to the library and
// never touches the loaded save. On any guard violation the result is
// a LoadedTemplatePreview with Report.OK=false and a single Error
// carrying the IssueCodeURL* code the UI can render — the function
// itself does not return a Go error for guard violations (mirroring
// the file import contract for parse / validation failures).
//
// Errors returned by Go's error channel are reserved for runtime
// programming bugs the UI cannot meaningfully react to (today: none —
// the function returns nil error in every path).
func (a *App) PreviewBuildTemplateImportYAMLFromURL(rawURL string) (LoadedTemplatePreview, error) {
	trimmed := strings.TrimSpace(rawURL)
	if trimmed == "" {
		return urlImportErrorPreview(rawURL, templates.IssueCodeURLEmpty, "URL is required."), nil
	}

	body, finalURL, fe := templates.FetchYAMLFromURL(context.Background(), trimmed)
	if fe != nil {
		return urlImportErrorPreview(trimmed, fe.Code, fe.Message), nil
	}
	return previewYAMLPayload(body, finalURL), nil
}

// urlImportErrorPreview wraps a fetch-side rejection in the same
// LoadedTemplatePreview shape the file path returns on validation
// errors. Path is set to the user-supplied URL so the modal can still
// echo it back as the source label even when the fetch itself failed.
func urlImportErrorPreview(sourceURL, code, message string) LoadedTemplatePreview {
	return LoadedTemplatePreview{
		Report: templates.ImportPreviewReport{
			OK:       false,
			Errors:   []templates.ImportPreviewIssue{{Severity: "error", Code: code, Message: message}},
			Warnings: []templates.ImportPreviewIssue{},
			Summary:  templates.ImportPreviewSummary{},
		},
		Path: sourceURL,
	}
}
