package main

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/oisis/EldenRing-SaveForge/backend/templates"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
)

// BuildTemplateV2ExportOptions is the Wails-facing input for v2 (profile +
// stats) template export from a character slot. Selection is passed
// separately as a JSON string so the boolean-or-map shape of
// templates.SectionSelection survives unchanged through Wails bindings.
//
// Unlike BuildTemplateExportOptions (v1, inventory.workspace), there is no
// IncludeInventory / IncludeStorage — section inclusion is driven entirely
// by the selection JSON.
type BuildTemplateV2ExportOptions struct {
	Name        string   `json:"name"`
	Description string   `json:"description"`
	Author      string   `json:"author"`
	Tags        []string `json:"tags"`
}

func u32p(v uint32) *uint32 { return &v }
func u8p(v uint8) *uint8    { return &v }

// parseTemplateSelectionJSON decodes the wire selection payload into a
// typed TemplateSelection. Top-level field names are restricted by
// DisallowUnknownFields so a typo like "profiel" surfaces as a hard error
// before BuildV2Template is reached. Per-section keys (level, vigor, ...)
// land in the open SectionSelection.Fields map and are validated downstream
// by templates.ValidateBuildTemplate's per-section allowlist.
func parseTemplateSelectionJSON(selectionJSON string) (*templates.TemplateSelection, error) {
	if strings.TrimSpace(selectionJSON) == "" {
		return nil, fmt.Errorf("selection JSON is empty")
	}
	dec := json.NewDecoder(strings.NewReader(selectionJSON))
	dec.DisallowUnknownFields()
	var sel templates.TemplateSelection
	if err := dec.Decode(&sel); err != nil {
		return nil, fmt.Errorf("parse selection: %w", err)
	}
	return &sel, nil
}

// buildTemplateV2SourcesFromCharacter maps a CharacterViewModel into the
// neutral DTOs consumed by templates.BuildV2Template. Mapping is dumb:
// every available field is copied for sections that have any selection;
// the builder then drops fields the user did not select (per-field
// selection) or keeps the whole section verbatim (boolean shortcut).
// Values are NOT clamped — out-of-range numbers surface as a builder /
// validator error rather than being silently coerced.
//
// vm.Souls maps to ProfileSource.Runes — the VM still carries the legacy
// "souls" naming from Dark Souls; the public template schema renamed the
// field to the Elden Ring term ("runes").
//
// ClassName is dropped when it begins with "Unknown (" — that prefix is
// the fallback emitted by vm.MapParsedSlotToVM when db.GetClassStats
// returns nil. A literal "Unknown (5)" string would round-trip through
// validation but is meaningless in a shareable template.
func buildTemplateV2SourcesFromCharacter(charVM *vm.CharacterViewModel, selection *templates.TemplateSelection) (*templates.ProfileSource, *templates.StatsSource) {
	var profile *templates.ProfileSource
	if selection.Profile.HasAny() {
		profile = &templates.ProfileSource{
			Name:                charVM.Name,
			Level:               u32p(charVM.Level),
			Runes:               u32p(charVM.Souls),
			SoulMemory:          u32p(charVM.SoulMemory),
			ClearCount:          u32p(charVM.ClearCount),
			ScadutreeBlessing:   u8p(charVM.ScadutreeBlessing),
			ShadowRealmBlessing: u8p(charVM.ShadowRealmBlessing),
			TalismanSlots:       u8p(charVM.TalismanSlots),
		}
		if !strings.HasPrefix(charVM.ClassName, "Unknown (") {
			profile.ClassName = charVM.ClassName
		}
	}
	var stats *templates.StatsSource
	if selection.Stats.HasAny() {
		stats = &templates.StatsSource{
			Vigor:        u32p(charVM.Vigor),
			Mind:         u32p(charVM.Mind),
			Endurance:    u32p(charVM.Endurance),
			Strength:     u32p(charVM.Strength),
			Dexterity:    u32p(charVM.Dexterity),
			Intelligence: u32p(charVM.Intelligence),
			Faith:        u32p(charVM.Faith),
			Arcane:       u32p(charVM.Arcane),
		}
	}
	return profile, stats
}

// buildAndValidateTemplateV2FromCharacter is the shared core for the v2
// charIndex-source endpoints. Phase 3C.1 ships only JSON export and
// preview; later phases (3C.2) reuse this helper for YAML export and
// library save without re-deriving sources.
//
// Source acquisition delegates to GetCharacter, which already handles
// saveMu / slotMu locking and "no save loaded" / "invalid slot index"
// error messages. SourceCharacterName is taken from charVM.Name rather
// than sourceCharacterName(charIndex) so we never reach for saveMu a
// second time outside the GetCharacter scope.
func (a *App) buildAndValidateTemplateV2FromCharacter(charIndex int, selectionJSON string, opts BuildTemplateV2ExportOptions) (*templates.BuildTemplate, string, error) {
	selection, err := parseTemplateSelectionJSON(selectionJSON)
	if err != nil {
		return nil, "", err
	}
	charVM, err := a.GetCharacter(charIndex)
	if err != nil {
		return nil, "", err
	}
	profile, stats := buildTemplateV2SourcesFromCharacter(charVM, selection)
	tags := opts.Tags
	if tags == nil {
		tags = []string{}
	}
	tpl, err := templates.BuildV2Template(templates.ExportV2Options{
		AppVersion: appVersion,
		Metadata: &templates.TemplateMetadata{
			Name:                 opts.Name,
			Description:          opts.Description,
			Author:               opts.Author,
			Tags:                 tags,
			SourceCharacterIndex: charIndex,
			SourceCharacterName:  charVM.Name,
		},
		Profile:   profile,
		Stats:     stats,
		Selection: selection,
	})
	if err != nil {
		return nil, "", fmt.Errorf("build v2 template: %w", err)
	}
	data, err := marshalBuildTemplate(tpl)
	if err != nil {
		return nil, "", err
	}
	return tpl, string(data), nil
}

// ExportBuildTemplateV2JSONFromCharacter returns the canonical JSON for a
// v2 template built from slot charIndex. Selection is a JSON-encoded
// templates.TemplateSelection — see parseTemplateSelectionJSON for the
// accepted shape.
//
// Dialog-less and filesystem-less. Use it from a UI that has its own
// preview / save UX (Phase 3D) or from tests.
func (a *App) ExportBuildTemplateV2JSONFromCharacter(charIndex int, selectionJSON string, opts BuildTemplateV2ExportOptions) (string, error) {
	_, jsonText, err := a.buildAndValidateTemplateV2FromCharacter(charIndex, selectionJSON, opts)
	return jsonText, err
}

// PreviewBuildTemplateV2FromCharacter builds a v2 template from slot
// charIndex and runs the preview validator. The returned LoadedTemplatePreview
// carries the canonical JSON alongside the report so a follow-up "save to
// library" call (Phase 3C.2) can reuse the exact same bytes — the
// anti-TOCTOU pattern already used by the YAML import path.
func (a *App) PreviewBuildTemplateV2FromCharacter(charIndex int, selectionJSON string, opts BuildTemplateV2ExportOptions) (LoadedTemplatePreview, error) {
	tpl, jsonText, err := a.buildAndValidateTemplateV2FromCharacter(charIndex, selectionJSON, opts)
	if err != nil {
		return LoadedTemplatePreview{}, err
	}
	report := templates.PreviewBuildTemplateImport(tpl, templates.ImportPreviewOptions{Mode: "append"})
	return LoadedTemplatePreview{Report: report, JSON: jsonText}, nil
}

// ExportBuildTemplateV2YAMLFromCharacter returns the canonical YAML payload
// for a v2 template built from slot charIndex. Dialog-less and
// filesystem-less — the file save dialog ships in Phase 3D alongside the
// bindings regen and UI.
//
// The output round-trips through templates.ParseBuildTemplateYAML: schema,
// version: 2, selection (boolean shortcut or per-field map per the
// requested selection), and sections.profile / sections.stats survive
// untouched.
func (a *App) ExportBuildTemplateV2YAMLFromCharacter(charIndex int, selectionJSON string, opts BuildTemplateV2ExportOptions) (string, error) {
	tpl, _, err := a.buildAndValidateTemplateV2FromCharacter(charIndex, selectionJSON, opts)
	if err != nil {
		return "", err
	}
	data, err := templates.MarshalBuildTemplateYAML(tpl)
	if err != nil {
		return "", fmt.Errorf("marshal v2 yaml: %w", err)
	}
	return string(data), nil
}

// SaveBuildTemplateV2FromCharacterToLibrary builds a v2 template from slot
// charIndex and persists it in the local library. Returns the new index
// entry (Version=2, SelectedSections populated by Phase 3C.0). Library
// re-validates and re-marshals the template internally; the canonical JSON
// from buildAndValidateTemplateV2FromCharacter is intentionally discarded
// to keep on-disk encoding centralised in Library.SaveTemplate.
//
// opts.Name may be empty — Library falls back to a "template-" filename
// stem when no display name is provided.
func (a *App) SaveBuildTemplateV2FromCharacterToLibrary(charIndex int, selectionJSON string, opts BuildTemplateV2ExportOptions) (templates.LibraryTemplateEntry, error) {
	tpl, _, err := a.buildAndValidateTemplateV2FromCharacter(charIndex, selectionJSON, opts)
	if err != nil {
		return templates.LibraryTemplateEntry{}, err
	}
	lib, err := a.ensureTemplateLibrary()
	if err != nil {
		return templates.LibraryTemplateEntry{}, err
	}
	return lib.SaveTemplate(tpl)
}
