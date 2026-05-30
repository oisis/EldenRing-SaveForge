package templates

import (
	"fmt"
	"time"
)

// ProfileSource is the neutral DTO consumed by BuildV2Template. Producers
// (the future app layer, test fixtures) populate only the fields they
// intend to share — string fields are "set" when non-empty, pointer
// fields are "set" when non-nil. Producers MUST NOT take addresses of map
// values or other temporaries that may be overwritten between Build and
// the returned template being consumed: the builder copies pointer values
// into independent storage to make the result safe to mutate.
//
// The Class display name maps to schema field profile.class. The builder
// does not perform DB lookups; choosing a stable name string is the
// producer's responsibility.
type ProfileSource struct {
	Name                string
	Level               *uint32
	Runes               *uint32
	SoulMemory          *uint32
	ClassName           string
	ClearCount          *uint32
	ScadutreeBlessing   *uint8
	ShadowRealmBlessing *uint8
	TalismanSlots       *uint8
}

// StatsSource is the neutral DTO for the eight character stats.
type StatsSource struct {
	Vigor        *uint32
	Mind         *uint32
	Endurance    *uint32
	Strength     *uint32
	Dexterity    *uint32
	Intelligence *uint32
	Faith        *uint32
	Arcane       *uint32
}

// ExportV2Options bundles the inputs to BuildV2Template. The builder is a
// pure function over this struct — no save state, no session, no vm
// dependency.
//
// Selection is required for v2 documents and drives which source fields
// are emitted. Each selected section requires a matching source DTO; a
// selected section without a source is a hard error so the caller can
// surface "you asked to share X but did not supply X" up front rather
// than emitting a half-populated template.
type ExportV2Options struct {
	AppVersion string
	Metadata   *TemplateMetadata
	Now        time.Time

	Profile            *ProfileSource
	Stats              *StatsSource
	InventoryWorkspace *InventoryWorkspaceSection

	Selection *TemplateSelection
}

// BuildV2Template assembles a `version: 2` BuildTemplate from neutral
// DTOs. The output Selection follows two normalization rules:
//
//   - boolean shortcut (All=true) survives verbatim — the producer asked
//     to share the whole section and the recipient is told so, even if
//     the source had no populated fields;
//   - per-field selection is reduced to the fields the source actually
//     supplied — a selected-but-unset field is removed from the returned
//     Selection so the v2 invariant "selection ⟺ data present" holds for
//     per-field maps.
//
// When per-field normalization drops a whole section (no field had a
// source value) the section is removed from both Selection and Sections.
// If that leaves no selected section at all, BuildV2Template returns an
// error rather than emit a structurally-empty v2 document.
//
// The result is fed through ValidateBuildTemplate before returning;
// callers therefore never see an internally-inconsistent template.
func BuildV2Template(opts ExportV2Options) (*BuildTemplate, error) {
	if opts.Selection == nil {
		return nil, fmt.Errorf("BuildV2Template: selection is required for v2 templates")
	}
	if !opts.Selection.HasAnySelected() {
		return nil, fmt.Errorf("BuildV2Template: selection has no selected fields")
	}
	if opts.Selection.Profile.HasAny() && opts.Profile == nil {
		return nil, fmt.Errorf("BuildV2Template: selection.profile is selected but no Profile source was provided")
	}
	if opts.Selection.Stats.HasAny() && opts.Stats == nil {
		return nil, fmt.Errorf("BuildV2Template: selection.stats is selected but no Stats source was provided")
	}
	if opts.Selection.InventoryWorkspace.HasAny() && opts.InventoryWorkspace == nil {
		return nil, fmt.Errorf("BuildV2Template: selection.inventory.workspace is selected but no InventoryWorkspace source was provided")
	}

	outSelection := &TemplateSelection{}
	var outSections TemplateSections

	if opts.Selection.Profile.HasAny() {
		profile, emittedFields := buildProfileSection(opts.Profile, opts.Selection.Profile)
		if opts.Selection.Profile.All {
			outSections.Profile = profile
			outSelection.Profile = &SectionSelection{All: true}
		} else if len(emittedFields) > 0 {
			outSections.Profile = profile
			outSelection.Profile = &SectionSelection{Fields: emittedFields}
		}
	}

	if opts.Selection.Stats.HasAny() {
		stats, emittedFields := buildStatsSection(opts.Stats, opts.Selection.Stats)
		if opts.Selection.Stats.All {
			outSections.Stats = stats
			outSelection.Stats = &SectionSelection{All: true}
		} else if len(emittedFields) > 0 {
			outSections.Stats = stats
			outSelection.Stats = &SectionSelection{Fields: emittedFields}
		}
	}

	if opts.Selection.InventoryWorkspace.HasAny() {
		outSections.InventoryWorkspace = opts.InventoryWorkspace
		outSelection.InventoryWorkspace = &SectionSelection{All: true}
	}

	if !outSelection.HasAnySelected() {
		return nil, fmt.Errorf("BuildV2Template: every per-field selection was dropped because the source had no matching values")
	}

	now := opts.Now
	if now.IsZero() {
		now = time.Now()
	}

	tpl := &BuildTemplate{
		Schema:     SchemaKey,
		Version:    2,
		CreatedAt:  now.UTC().Format(time.RFC3339),
		AppVersion: opts.AppVersion,
		Metadata:   opts.Metadata,
		Selection:  outSelection,
		Sections:   outSections,
	}

	if err := ValidateBuildTemplate(tpl); err != nil {
		return nil, fmt.Errorf("BuildV2Template: built template failed validation: %w", err)
	}
	return tpl, nil
}

// buildProfileSection copies set source fields whose selection key is
// included in sel. Pointer values are dereferenced and re-allocated so
// the returned section does not alias the source DTO. The second return
// names the fields that actually landed in the section, used by the
// caller to normalize per-field selections.
func buildProfileSection(src *ProfileSource, sel *SectionSelection) (*ProfileSection, map[string]bool) {
	out := &ProfileSection{}
	emitted := map[string]bool{}

	if sel.Selected("name") && src.Name != "" {
		v := src.Name
		out.Name = &v
		emitted["name"] = true
	}
	if sel.Selected("level") && src.Level != nil {
		v := *src.Level
		out.Level = &v
		emitted["level"] = true
	}
	if sel.Selected("runes") && src.Runes != nil {
		v := *src.Runes
		out.Runes = &v
		emitted["runes"] = true
	}
	if sel.Selected("soulMemory") && src.SoulMemory != nil {
		v := *src.SoulMemory
		out.SoulMemory = &v
		emitted["soulMemory"] = true
	}
	if sel.Selected("class") && src.ClassName != "" {
		v := src.ClassName
		out.Class = &v
		emitted["class"] = true
	}
	if sel.Selected("clearCount") && src.ClearCount != nil {
		v := *src.ClearCount
		out.ClearCount = &v
		emitted["clearCount"] = true
	}
	if sel.Selected("scadutreeBlessing") && src.ScadutreeBlessing != nil {
		v := *src.ScadutreeBlessing
		out.ScadutreeBlessing = &v
		emitted["scadutreeBlessing"] = true
	}
	if sel.Selected("shadowRealmBlessing") && src.ShadowRealmBlessing != nil {
		v := *src.ShadowRealmBlessing
		out.ShadowRealmBlessing = &v
		emitted["shadowRealmBlessing"] = true
	}
	if sel.Selected("talismanSlots") && src.TalismanSlots != nil {
		v := *src.TalismanSlots
		out.TalismanSlots = &v
		emitted["talismanSlots"] = true
	}
	return out, emitted
}

// buildStatsSection mirrors buildProfileSection for the eight stat fields.
func buildStatsSection(src *StatsSource, sel *SectionSelection) (*StatsSection, map[string]bool) {
	out := &StatsSection{}
	emitted := map[string]bool{}

	stats := []struct {
		key string
		src *uint32
		dst **uint32
	}{
		{"vigor", src.Vigor, &out.Vigor},
		{"mind", src.Mind, &out.Mind},
		{"endurance", src.Endurance, &out.Endurance},
		{"strength", src.Strength, &out.Strength},
		{"dexterity", src.Dexterity, &out.Dexterity},
		{"intelligence", src.Intelligence, &out.Intelligence},
		{"faith", src.Faith, &out.Faith},
		{"arcane", src.Arcane, &out.Arcane},
	}
	for _, st := range stats {
		if sel.Selected(st.key) && st.src != nil {
			v := *st.src
			*st.dst = &v
			emitted[st.key] = true
		}
	}
	return out, emitted
}
