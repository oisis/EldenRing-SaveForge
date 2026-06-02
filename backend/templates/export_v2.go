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
	// Equipment is the Phase 7b.1 source. Already in schema shape (no
	// DTO conversion) because the section is small (14 slot pointers)
	// and producers naturally walk the 14 ChrAsmEquipment slots —
	// adding a parallel EquipmentSource DTO would just duplicate the
	// pointer-soup.
	Equipment *EquipmentSection

	// EquippedSpellsRaw is the Phase 7d.2 source: the 14-slot raw spell
	// loadout as it sits in the save (i.e. the slice returned by
	// core.readSpellIDs(slot.Data, slot.EquippedSpellsOffset)).
	//
	// Each entry is a raw MagicParam ID, with the save's empty-slot
	// sentinel 0xFFFFFFFF marking unused slots. BuildV2Template
	// translates this into the template schema's user-facing form:
	//   - raw 0xFFFFFFFF → SpellSlotRef{BaseItemID: 0} (explicit clear),
	//     consistent with Phase 7d.1's empty-slot convention;
	//   - any other raw ID → SpellSlotRef{BaseItemID: 0x40000000 | raw},
	//     producing the full DB-style item ID stored in templates.
	//
	// Producers MUST supply exactly SpellSlotCount entries when
	// selection.spells is selected. A wrong-length slice is a hard
	// error — the builder refuses to silently truncate or pad because
	// either would mis-bind slot indices to spell IDs.
	EquippedSpellsRaw []uint32

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
	if opts.Selection.Equipment.HasAny() && opts.Equipment == nil {
		return nil, fmt.Errorf("BuildV2Template: selection.equipment is selected but no Equipment source was provided")
	}
	if opts.Selection.Spells.HasAny() {
		if opts.EquippedSpellsRaw == nil {
			return nil, fmt.Errorf("BuildV2Template: selection.spells is selected but no EquippedSpellsRaw source was provided")
		}
		if len(opts.EquippedSpellsRaw) != SpellSlotCount {
			return nil, fmt.Errorf("BuildV2Template: EquippedSpellsRaw length is %d, want %d", len(opts.EquippedSpellsRaw), SpellSlotCount)
		}
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

	if opts.Selection.Spells.HasAny() {
		spells, emittedSlots := buildSpellsSection(opts.EquippedSpellsRaw, opts.Selection.Spells)
		if opts.Selection.Spells.All {
			outSections.Spells = spells
			outSelection.Spells = &SectionSelection{All: true}
		} else if len(emittedSlots) > 0 {
			outSections.Spells = spells
			outSelection.Spells = &SectionSelection{Fields: emittedSlots}
		}
	}

	if opts.Selection.Equipment.HasAny() {
		equipment, emittedSlots := buildEquipmentSection(opts.Equipment, opts.Selection.Equipment)
		if opts.Selection.Equipment.All {
			outSections.Equipment = equipment
			outSelection.Equipment = &SectionSelection{All: true}
		} else if len(emittedSlots) > 0 {
			outSections.Equipment = equipment
			outSelection.Equipment = &SectionSelection{Fields: emittedSlots}
		}
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

// buildEquipmentSection copies set source slots whose key is included in
// sel. Each EquipmentItemRef value is deep-cloned so the returned
// section does not alias the source's pointer fields — important
// because callers reuse EquipmentSection structs across multiple Build
// calls. The second return names the slot keys that actually landed,
// used by BuildV2Template to normalise per-field selections.
//
// A slot whose source ref is nil is treated as "not in source" and
// dropped (the boolean shortcut keeps it nil → the output likewise
// stays nil for that slot). A ref with BaseItemID == 0 is kept verbatim
// (explicit-clear sentinel).
func buildEquipmentSection(src *EquipmentSection, sel *SectionSelection) (*EquipmentSection, map[string]bool) {
	out := &EquipmentSection{}
	emitted := map[string]bool{}
	for _, key := range EquipmentSlotOrder {
		if !sel.Selected(key) {
			continue
		}
		ref := EquipmentSlotRef(src, key)
		if ref == nil {
			continue
		}
		clone := *ref
		if ref.Upgrade != nil {
			v := *ref.Upgrade
			clone.Upgrade = &v
		}
		if ref.AoWItemID != nil {
			v := *ref.AoWItemID
			clone.AoWItemID = &v
		}
		SetEquipmentSlotRef(out, key, &clone)
		emitted[key] = true
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

// buildSpellsSection translates the 14-slot raw spell loadout from the
// save into the template's user-facing SpellsSection shape. Length is
// pre-validated by BuildV2Template, so the index lookup is always safe.
//
// Per-slot translation:
//   - rawID == 0xFFFFFFFF (save empty-slot sentinel) → BaseItemID = 0
//     (template explicit-clear). The save-format sentinel never leaks
//     into the public template, matching the Phase 7d.1 schema rule.
//   - rawID != 0xFFFFFFFF → BaseItemID = SpellItemIDPrefix | rawID,
//     producing the full DB-style item ID (e.g. raw 0x1770 →
//     0x40001770 = Catch Flame).
//
// Per-selection behaviour mirrors buildEquipmentSection: the boolean
// shortcut emits all 14 slots; per-field selection emits only the
// listed slot keys.
//
// Name is intentionally left empty: the DB lookup that would resolve
// raw IDs to human names belongs in a higher layer (the app endpoint
// that already has the DB handy in 7d.3+), and ImportPreviewSummary
// already documents Name as debug-only metadata. Keeping this builder
// dependency-free preserves the package's lookup-free invariant.
func buildSpellsSection(rawIDs []uint32, sel *SectionSelection) (*SpellsSection, map[string]bool) {
	const saveEmptySentinel uint32 = 0xFFFFFFFF

	out := &SpellsSection{}
	emitted := map[string]bool{}
	for i, key := range SpellSlotOrder {
		if !sel.Selected(key) {
			continue
		}
		raw := rawIDs[i]
		ref := &SpellSlotRef{}
		if raw != saveEmptySentinel {
			ref.BaseItemID = SpellItemIDPrefix | raw
		}
		setSpellSlotRef(out, key, ref)
		emitted[key] = true
	}
	return out, emitted
}
