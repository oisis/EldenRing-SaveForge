package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/templates"
)

// PreviewBuildTemplateImportYAMLFromURL is a thin Wails handler: it
// wraps the production FetchYAMLFromURL (covered by SSRF / scheme /
// redirect / content-type / body-cap tests in backend/templates), then
// hands the bytes to previewYAMLPayload (covered by the file import
// tests). The integration tests here only need to confirm the wiring:
//   - an empty URL surfaces IssueCodeURLEmpty in the report,
//   - a guard violation (e.g. http scheme) maps to an
//     IssueCodeURLDisallowedScheme report with sourceURL echoed back,
//   - a forbidden-IP literal surfaces IssueCodeURLForbiddenIP.
//
// We do NOT exercise the full happy-path here — that would re-test the
// backend/templates fetcher. The handler's job is to translate
// templates.FetchError → LoadedTemplatePreview.

func TestPreviewBuildTemplateImportYAMLFromURL_EmptyURL(t *testing.T) {
	a := NewApp()
	preview, err := a.PreviewBuildTemplateImportYAMLFromURL("")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preview.Report.OK {
		t.Errorf("report.OK = true, want false")
	}
	if len(preview.Report.Errors) != 1 {
		t.Fatalf("expected 1 error, got %d: %+v", len(preview.Report.Errors), preview.Report.Errors)
	}
	if preview.Report.Errors[0].Code != templates.IssueCodeURLEmpty {
		t.Errorf("code = %q, want %q", preview.Report.Errors[0].Code, templates.IssueCodeURLEmpty)
	}
	if preview.JSON != "" {
		t.Errorf("JSON should be empty on guard rejection, got %q", preview.JSON)
	}
}

func TestPreviewBuildTemplateImportYAMLFromURL_RejectsHTTPScheme(t *testing.T) {
	a := NewApp()
	preview, err := a.PreviewBuildTemplateImportYAMLFromURL("http://example.com/x.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preview.Report.OK {
		t.Errorf("report.OK = true, want false")
	}
	if len(preview.Report.Errors) == 0 ||
		preview.Report.Errors[0].Code != templates.IssueCodeURLDisallowedScheme {
		t.Errorf("expected url_disallowed_scheme, got %+v", preview.Report.Errors)
	}
	// Source URL must be echoed back so the UI can show it in the
	// preview panel even on rejection.
	if preview.Path != "http://example.com/x.yaml" {
		t.Errorf("Path = %q, want the user-supplied URL", preview.Path)
	}
}

func TestPreviewBuildTemplateImportYAMLFromURL_RejectsLoopbackLiteral(t *testing.T) {
	a := NewApp()
	preview, err := a.PreviewBuildTemplateImportYAMLFromURL("https://127.0.0.1/x.yaml")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preview.Report.OK {
		t.Errorf("report.OK = true, want false")
	}
	if len(preview.Report.Errors) == 0 ||
		preview.Report.Errors[0].Code != templates.IssueCodeURLForbiddenIP {
		t.Errorf("expected url_forbidden_ip, got %+v", preview.Report.Errors)
	}
}

func TestPreviewBuildTemplateImportYAMLFromURL_RejectsCloudMetadataIP(t *testing.T) {
	a := NewApp()
	preview, err := a.PreviewBuildTemplateImportYAMLFromURL("https://169.254.169.254/latest/meta-data/")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preview.Report.OK {
		t.Errorf("report.OK = true, want false")
	}
	if len(preview.Report.Errors) == 0 ||
		preview.Report.Errors[0].Code != templates.IssueCodeURLForbiddenIP {
		t.Errorf("expected url_forbidden_ip, got %+v", preview.Report.Errors)
	}
}

func TestPreviewBuildTemplateImportYAMLFromURL_TrimsWhitespace(t *testing.T) {
	a := NewApp()
	preview, err := a.PreviewBuildTemplateImportYAMLFromURL("   \t  ")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preview.Report.OK {
		t.Errorf("report.OK = true, want false")
	}
	if len(preview.Report.Errors) == 0 ||
		preview.Report.Errors[0].Code != templates.IssueCodeURLEmpty {
		t.Errorf("expected url_empty after trim, got %+v", preview.Report.Errors)
	}
}

func TestPreviewBuildTemplateImportYAMLFromURL_RejectsDataScheme(t *testing.T) {
	a := NewApp()
	preview, err := a.PreviewBuildTemplateImportYAMLFromURL("data:application/yaml;base64,abc")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if preview.Report.OK {
		t.Errorf("report.OK = true, want false")
	}
	if preview.Report.Errors[0].Code != templates.IssueCodeURLDisallowedScheme {
		t.Errorf("expected url_disallowed_scheme, got %q", preview.Report.Errors[0].Code)
	}
}
