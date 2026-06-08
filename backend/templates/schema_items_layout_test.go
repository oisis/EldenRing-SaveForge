package templates

import (
	"strings"
	"testing"
)

// Phase 8B — structural tests for items / inventoryLayout / storageLayout
// / applyOptions / weaponLevelOverride. The apply layer does not exist
// yet (Phase 8C+), so these tests cover ONLY the schema contract —
// shape, allowlists, ranges, cross-references.

// ─── fixture builders ──────────────────────────────────────────────────

// emptySelectionV2 returns a v2 template skeleton that satisfies "have a
// selection object" but does NOT select anything yet. Tests add a
// selection + matching section themselves so each assertion is self-
// contained.
func emptySelectionV2() *BuildTemplate {
	return &BuildTemplate{
		Schema:    SchemaKey,
		Version:   2,
		CreatedAt: "2026-06-08T00:00:00Z",
		Selection: &TemplateSelection{},
		Sections:  TemplateSections{},
	}
}

// itemsOnlyV2 returns a v2 template that selects sections.items and
// pre-populates one valid melee weapon entry. Tests mutate the returned
// template to exercise individual fields.
func itemsOnlyV2() *BuildTemplate {
	tpl := emptySelectionV2()
	tpl.Selection.Items = &SectionSelection{All: true}
	tpl.Sections.Items = &ItemsSection{
		Entries: []TemplateItemEntryV2{
			validMeleeEntry("weapon_main"),
		},
	}
	return tpl
}

func validMeleeEntry(id string) TemplateItemEntryV2 {
	return TemplateItemEntryV2{
		EntryID:      id,
		ItemID:       0x003D9700,
		Name:         "Greatsword",
		Category:     ItemCategoryMeleeArmaments,
		Quantity:     1,
		Location:     ItemLocationInventory,
		UpgradeKind:  UpgradeKindStandard,
		UpgradeLevel: u8p(25),
	}
}

func validSomberEntry(id string) TemplateItemEntryV2 {
	return TemplateItemEntryV2{
		EntryID:      id,
		ItemID:       0x003D9701,
		Name:         "Some Somber",
		Category:     ItemCategoryMeleeArmaments,
		Quantity:     1,
		Location:     ItemLocationInventory,
		UpgradeKind:  UpgradeKindSomber,
		UpgradeLevel: u8p(10),
	}
}

func validTalismanEntry(id string) TemplateItemEntryV2 {
	return TemplateItemEntryV2{
		EntryID:  id,
		ItemID:   0x200003E8,
		Name:     "Crimson Amber Medallion",
		Category: ItemCategoryTalismans,
		Quantity: 1,
		Location: ItemLocationInventory,
		// no upgrade fields — talismans are non-upgradable here
	}
}

// ─── A. valid templates ────────────────────────────────────────────────

func TestValidate_AcceptsV2_NoItemsOrLayoutStillValid(t *testing.T) {
	// A v2 template selecting only profile (existing minimal helper)
	// must still pass — Phase 8B sections are purely additive.
	tpl := minimalValidV2("profile")
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("v2 profile-only template must remain valid after Phase 8B: %v", err)
	}
}

func TestValidate_AcceptsV2_ItemsAcrossCategories(t *testing.T) {
	aow := uint32(0x80002710)
	tpl := emptySelectionV2()
	tpl.Selection.Items = &SectionSelection{All: true}
	tpl.Sections.Items = &ItemsSection{
		Entries: []TemplateItemEntryV2{
			{EntryID: "w1", ItemID: 0x003D9700, Category: ItemCategoryMeleeArmaments, Quantity: 1, Location: ItemLocationInventory, UpgradeKind: UpgradeKindStandard, UpgradeLevel: u8p(25), InfusionName: "Heavy", AshOfWarItemID: &aow},
			{EntryID: "w2", ItemID: 0x003D9701, Category: ItemCategoryRangedAndCatalysts, Quantity: 1, Location: ItemLocationInventory, UpgradeKind: UpgradeKindSomber, UpgradeLevel: u8p(10)},
			{EntryID: "sh1", ItemID: 0x004C4B40, Category: ItemCategoryShields, Quantity: 1, Location: ItemLocationInventory, UpgradeKind: UpgradeKindStandard, UpgradeLevel: u8p(0)},
			{EntryID: "aow1", ItemID: 0x80002710, Category: ItemCategoryAshesOfWar, Quantity: 1, Location: ItemLocationInventory},
			{EntryID: "h1", ItemID: 0x10000001, Category: ItemCategoryArmorHead, Quantity: 1, Location: ItemLocationInventory},
			{EntryID: "c1", ItemID: 0x10000002, Category: ItemCategoryArmorChest, Quantity: 1, Location: ItemLocationInventory},
			{EntryID: "a1", ItemID: 0x10000003, Category: ItemCategoryArmorArms, Quantity: 1, Location: ItemLocationInventory},
			{EntryID: "l1", ItemID: 0x10000004, Category: ItemCategoryArmorLegs, Quantity: 1, Location: ItemLocationInventory},
			{EntryID: "t1", ItemID: 0x200003E8, Category: ItemCategoryTalismans, Quantity: 1, Location: ItemLocationInventory},
			{EntryID: "sp1", ItemID: 0x40001770, Category: ItemCategorySorceries, Quantity: 1, Location: ItemLocationStorage},
			{EntryID: "in1", ItemID: 0x40001771, Category: ItemCategoryIncantations, Quantity: 1, Location: ItemLocationStorage},
			{EntryID: "tool1", ItemID: 0x40000001, Category: ItemCategoryTools, Quantity: 99, Location: ItemLocationBoth},
			{EntryID: "cm1", ItemID: 0x40000002, Category: ItemCategoryCraftingMaterials, Quantity: 50, Location: ItemLocationStorage},
			{EntryID: "bm1", ItemID: 0x40000003, Category: ItemCategoryBolsteringMaterials, Quantity: 12, Location: ItemLocationStorage},
			{EntryID: "ab1", ItemID: 0x02FAF080, Category: ItemCategoryArrowsAndBolts, Quantity: 99, Location: ItemLocationInventory},
			{EntryID: "ki1", ItemID: 0x40000004, Category: ItemCategoryKeyItems, Quantity: 1, Location: ItemLocationInventory},
			{EntryID: "g1", ItemID: 0x90000000, Category: ItemCategoryGestures, Quantity: 1, Location: ItemLocationInventory},
			{EntryID: "dlc1", ItemID: 0x40000005, Category: ItemCategoryDLC, Quantity: 1, Location: ItemLocationStorage},
		},
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("expected items-across-categories template to validate: %v", err)
	}
}

func TestValidate_AcceptsV2_StandardUpgradeAtMax(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries[0].UpgradeLevel = u8p(MaxItemUpgradeStandard)
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("standard +25 must validate: %v", err)
	}
}

func TestValidate_RejectsV2_StandardUpgradeAboveMax(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries[0].UpgradeLevel = u8p(MaxItemUpgradeStandard + 1)
	err := ValidateBuildTemplate(tpl)
	if err == nil {
		t.Fatal("expected error for standard +26")
	}
	if !strings.Contains(err.Error(), "out of range") {
		t.Fatalf("error should mention range; got: %v", err)
	}
}

func TestValidate_AcceptsV2_SomberUpgradeAtMax(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries[0] = validSomberEntry("weapon_somber")
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("somber +10 must validate: %v", err)
	}
}

func TestValidate_RejectsV2_SomberUpgradeAboveMax(t *testing.T) {
	tpl := itemsOnlyV2()
	e := validSomberEntry("weapon_somber")
	e.UpgradeLevel = u8p(MaxItemUpgradeSomber + 1)
	tpl.Sections.Items.Entries[0] = e
	err := ValidateBuildTemplate(tpl)
	if err == nil {
		t.Fatal("expected error for somber +11")
	}
	if !strings.Contains(err.Error(), "somber") {
		t.Fatalf("error should mention somber kind; got: %v", err)
	}
}

func TestValidate_RejectsV2_StandardWithoutUpgradeLevel(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries[0].UpgradeLevel = nil
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("standard kind without upgradeLevel must fail")
	}
}

func TestValidate_RejectsV2_NoneWithUpgradeLevel(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries[0] = TemplateItemEntryV2{
		EntryID:      "tally",
		ItemID:       0x200003E8,
		Category:     ItemCategoryTalismans,
		Quantity:     1,
		Location:     ItemLocationInventory,
		UpgradeKind:  UpgradeKindNone,
		UpgradeLevel: u8p(0),
	}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("upgradeKind=none with upgradeLevel must fail")
	}
}

func TestValidate_AcceptsV2_EmptyUpgradeKindIsNone(t *testing.T) {
	// An empty UpgradeKind is the shorthand for "none" — used by
	// producers that do not care about upgrades (talismans, key items,
	// consumables). validTalismanEntry leaves UpgradeKind empty.
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries[0] = validTalismanEntry("tal1")
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("empty upgradeKind on non-upgradable item must validate: %v", err)
	}
}

func TestValidate_RejectsV2_UnknownUpgradeKind(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries[0].UpgradeKind = "legendary"
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("unknown upgradeKind must fail-closed")
	}
}

func TestValidate_RejectsV2_NegativeOrZeroQuantity(t *testing.T) {
	// uint32 disallows negative at the type level; the validator
	// rejects zero (Phase 8B fail-closed decision).
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries[0].Quantity = 0
	err := ValidateBuildTemplate(tpl)
	if err == nil {
		t.Fatal("quantity=0 must fail")
	}
	if !strings.Contains(err.Error(), "quantity=0") {
		t.Fatalf("error should mention quantity=0; got: %v", err)
	}
}

func TestValidate_RejectsV2_UnknownCategory(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries[0].Category = "mystery_box"
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("unknown category must fail-closed")
	}
}

func TestValidate_RejectsV2_UnknownLocation(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries[0].Location = "vault"
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("unknown location must fail-closed")
	}
}

func TestValidate_RejectsV2_EmptyEntryID(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries[0].EntryID = ""
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("empty entryID must fail")
	}
}

func TestValidate_RejectsV2_DuplicateEntryID(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries = append(tpl.Sections.Items.Entries, validMeleeEntry("weapon_main"))
	err := ValidateBuildTemplate(tpl)
	if err == nil {
		t.Fatal("duplicate entryID must fail")
	}
	if !strings.Contains(err.Error(), "already used") {
		t.Fatalf("error should mention dup; got: %v", err)
	}
}

func TestValidate_AcceptsV2_DuplicateItemIDDifferentEntryID(t *testing.T) {
	// Two Greatswords, one Heavy +25, one Cold +20 — same ItemID,
	// different entryID. Must validate.
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries = append(tpl.Sections.Items.Entries, TemplateItemEntryV2{
		EntryID:      "weapon_offhand",
		ItemID:       0x003D9700,
		Name:         "Greatsword (Cold +20)",
		Category:     ItemCategoryMeleeArmaments,
		Quantity:     1,
		Location:     ItemLocationStorage,
		UpgradeKind:  UpgradeKindStandard,
		UpgradeLevel: u8p(20),
		InfusionName: "Cold",
	})
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("two copies of same itemID with different entryID/upgrade/infusion must validate: %v", err)
	}
}

func TestValidate_RejectsV2_AshOfWarItemIDZero(t *testing.T) {
	tpl := itemsOnlyV2()
	zero := uint32(0)
	tpl.Sections.Items.Entries[0].AshOfWarItemID = &zero
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("ashOfWarItemID=0 must fail (omit field for any-AoW)")
	}
}

func TestValidate_RejectsV2_ItemIDZero(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries[0].ItemID = 0
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("itemID=0 must fail")
	}
}

// ─── B. inventoryLayout / storageLayout ────────────────────────────────

func TestValidate_AcceptsV2_InventoryLayoutReferencingItems(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries = append(tpl.Sections.Items.Entries, validTalismanEntry("tal1"))
	tpl.Selection.InventoryLayout = &SectionSelection{All: true}
	tpl.Sections.InventoryLayout = &InventoryLayoutSection{
		Entries: []LayoutEntry{
			{EntryRef: "weapon_main", Position: 0},
			{EntryRef: "tal1", Position: 1},
		},
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("inventoryLayout referencing existing entries must validate: %v", err)
	}
}

func TestValidate_AcceptsV2_StorageLayoutReferencingItems(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries = append(tpl.Sections.Items.Entries, validTalismanEntry("tal1"))
	tpl.Selection.StorageLayout = &SectionSelection{All: true}
	tpl.Sections.StorageLayout = &StorageLayoutSection{
		Entries: []LayoutEntry{
			{EntryRef: "tal1", Position: 0},
		},
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("storageLayout referencing existing entry must validate: %v", err)
	}
}

func TestValidate_RejectsV2_LayoutReferencesMissingItem(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Selection.InventoryLayout = &SectionSelection{All: true}
	tpl.Sections.InventoryLayout = &InventoryLayoutSection{
		Entries: []LayoutEntry{
			{EntryRef: "ghost_id_xyz", Position: 0},
		},
	}
	err := ValidateBuildTemplate(tpl)
	if err == nil {
		t.Fatal("layout entryRef to missing item must fail")
	}
	if !strings.Contains(err.Error(), "does not match") {
		t.Fatalf("error should mention missing entryID; got: %v", err)
	}
}

func TestValidate_RejectsV2_LayoutEmptyEntryRef(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Selection.InventoryLayout = &SectionSelection{All: true}
	tpl.Sections.InventoryLayout = &InventoryLayoutSection{
		Entries: []LayoutEntry{
			{EntryRef: "", Position: 0},
		},
	}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("empty entryRef must fail")
	}
}

func TestValidate_RejectsV2_LayoutDuplicateEntryRef(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Selection.InventoryLayout = &SectionSelection{All: true}
	tpl.Sections.InventoryLayout = &InventoryLayoutSection{
		Entries: []LayoutEntry{
			{EntryRef: "weapon_main", Position: 0},
			{EntryRef: "weapon_main", Position: 1},
		},
	}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("duplicate entryRef in same layout must fail")
	}
}

func TestValidate_RejectsV2_LayoutDuplicatePosition(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Sections.Items.Entries = append(tpl.Sections.Items.Entries, validTalismanEntry("tal1"))
	tpl.Selection.InventoryLayout = &SectionSelection{All: true}
	tpl.Sections.InventoryLayout = &InventoryLayoutSection{
		Entries: []LayoutEntry{
			{EntryRef: "weapon_main", Position: 0},
			{EntryRef: "tal1", Position: 0},
		},
	}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("duplicate position in same layout must fail")
	}
}

func TestValidate_RejectsV2_LayoutSelectedWithoutLayoutSection(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Selection.InventoryLayout = &SectionSelection{All: true}
	// Sections.InventoryLayout intentionally nil.
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("selection.inventoryLayout=true without sections.inventoryLayout must fail")
	}
}

func TestValidate_RejectsV2_LayoutSelectionWithFieldMap(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.Selection.InventoryLayout = &SectionSelection{Fields: map[string]bool{"foo": true}}
	tpl.Sections.InventoryLayout = &InventoryLayoutSection{Entries: nil}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("selection.inventoryLayout with Fields map must fail (boolean-only)")
	}
}

func TestValidate_RejectsV2_ItemsSelectionWithFieldMap(t *testing.T) {
	tpl := emptySelectionV2()
	tpl.Selection.Items = &SectionSelection{Fields: map[string]bool{"foo": true}}
	tpl.Sections.Items = &ItemsSection{Entries: []TemplateItemEntryV2{validMeleeEntry("x")}}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("selection.items with Fields map must fail (boolean-only)")
	}
}

// ─── C. apply options ───────────────────────────────────────────────────

func TestValidate_AcceptsV2_ApplyOptionsAllModes(t *testing.T) {
	for _, mode := range []string{ItemApplyModeAddMissing, ItemApplyModeUpdateExisting, ItemApplyModeMerge, ItemApplyModeReplace} {
		t.Run("item_mode="+mode, func(t *testing.T) {
			tpl := itemsOnlyV2()
			tpl.ApplyOptions = &ApplyOptions{
				Items: &ItemApplyOptions{Mode: mode},
			}
			if err := ValidateBuildTemplate(tpl); err != nil {
				t.Fatalf("item apply mode %q must validate: %v", mode, err)
			}
		})
	}
	for _, mode := range []string{LayoutApplyModeIgnore, LayoutApplyModeAppend, LayoutApplyModeReorderOnly, LayoutApplyModeReplace} {
		t.Run("layout_mode="+mode, func(t *testing.T) {
			tpl := itemsOnlyV2()
			tpl.ApplyOptions = &ApplyOptions{
				InventoryLayout: &LayoutApplyOptions{Mode: mode},
				StorageLayout:   &LayoutApplyOptions{Mode: mode},
			}
			if err := ValidateBuildTemplate(tpl); err != nil {
				t.Fatalf("layout apply mode %q must validate: %v", mode, err)
			}
		})
	}
}

func TestValidate_RejectsV2_ApplyOptionsBadItemMode(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.ApplyOptions = &ApplyOptions{Items: &ItemApplyOptions{Mode: "nuke"}}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("invalid item apply mode must fail")
	}
}

func TestValidate_RejectsV2_ApplyOptionsBadLayoutMode(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.ApplyOptions = &ApplyOptions{InventoryLayout: &LayoutApplyOptions{Mode: "scramble"}}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("invalid inventory layout apply mode must fail")
	}
	tpl.ApplyOptions = &ApplyOptions{StorageLayout: &LayoutApplyOptions{Mode: "scramble"}}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("invalid storage layout apply mode must fail")
	}
}

func TestValidate_RejectsV2_ApplyOptionsEmptyItemMode(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.ApplyOptions = &ApplyOptions{Items: &ItemApplyOptions{Mode: ""}}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("empty item apply mode must fail")
	}
}

// ─── D. weapon level override ───────────────────────────────────────────

func TestValidate_AcceptsV2_WeaponLevelOverride_UseTemplateLevels(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.ApplyOptions = &ApplyOptions{
		WeaponLevelOverride: &WeaponLevelOverride{UseTemplateLevels: true},
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("useTemplateLevels=true (no overrides) must validate: %v", err)
	}
}

func TestValidate_AcceptsV2_WeaponLevelOverride_StandardSomberBothSet(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.ApplyOptions = &ApplyOptions{
		WeaponLevelOverride: &WeaponLevelOverride{
			StandardOverride: u8p(MaxItemUpgradeStandard),
			SomberOverride:   u8p(MaxItemUpgradeSomber),
		},
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("standard=25 / somber=10 overrides must validate: %v", err)
	}
}

func TestValidate_AcceptsV2_WeaponLevelOverride_AllNilLeavesLevelsUntouched(t *testing.T) {
	// useTemplateLevels=false, both overrides nil → semantically "do
	// not touch upgrade levels at apply time". Must validate.
	tpl := itemsOnlyV2()
	tpl.ApplyOptions = &ApplyOptions{
		WeaponLevelOverride: &WeaponLevelOverride{},
	}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("empty WeaponLevelOverride must validate (means no change): %v", err)
	}
}

func TestValidate_RejectsV2_WeaponLevelOverride_UseTemplatePlusOverride(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.ApplyOptions = &ApplyOptions{
		WeaponLevelOverride: &WeaponLevelOverride{
			UseTemplateLevels: true,
			StandardOverride:  u8p(5),
		},
	}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("useTemplateLevels=true with standardOverride must fail (mutually exclusive)")
	}
}

func TestValidate_RejectsV2_WeaponLevelOverride_StandardOutOfRange(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.ApplyOptions = &ApplyOptions{
		WeaponLevelOverride: &WeaponLevelOverride{
			StandardOverride: u8p(MaxItemUpgradeStandard + 1),
		},
	}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("standardOverride=26 must fail")
	}
}

func TestValidate_RejectsV2_WeaponLevelOverride_SomberOutOfRange(t *testing.T) {
	tpl := itemsOnlyV2()
	tpl.ApplyOptions = &ApplyOptions{
		WeaponLevelOverride: &WeaponLevelOverride{
			SomberOverride: u8p(MaxItemUpgradeSomber + 1),
		},
	}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("somberOverride=11 must fail")
	}
}

// ─── E. selection / sections wiring ─────────────────────────────────────

func TestValidate_RejectsV2_ItemsSelectedWithoutItemsSection(t *testing.T) {
	tpl := emptySelectionV2()
	tpl.Selection.Items = &SectionSelection{All: true}
	if err := ValidateBuildTemplate(tpl); err == nil {
		t.Fatal("selection.items=true with no sections.items must fail")
	}
}

func TestValidate_RejectsV2_LayoutWithoutItemsSection(t *testing.T) {
	tpl := emptySelectionV2()
	tpl.Selection.InventoryLayout = &SectionSelection{All: true}
	tpl.Sections.InventoryLayout = &InventoryLayoutSection{
		Entries: []LayoutEntry{{EntryRef: "missing", Position: 0}},
	}
	err := ValidateBuildTemplate(tpl)
	if err == nil {
		t.Fatal("layout entries must fail when there is no items section to reference")
	}
}

func TestValidate_AcceptsV2_StorageLayoutOnlyWithSharedItems(t *testing.T) {
	// Selecting only storageLayout (not items) is unusual but legal
	// — provided the items section is still present so the validator
	// can resolve entryRefs. This documents that "selection drives
	// apply intent, sections drive validation reachability."
	tpl := emptySelectionV2()
	tpl.Sections.Items = &ItemsSection{Entries: []TemplateItemEntryV2{validTalismanEntry("tal1")}}
	tpl.Selection.StorageLayout = &SectionSelection{All: true}
	tpl.Sections.StorageLayout = &StorageLayoutSection{Entries: []LayoutEntry{{EntryRef: "tal1", Position: 0}}}
	if err := ValidateBuildTemplate(tpl); err != nil {
		t.Fatalf("layout selected without items selection (items section still present) must validate: %v", err)
	}
}

// ─── F. YAML strict-mode / round-trip ───────────────────────────────────

func TestParseYAML_RejectsUnknownTopLevelField(t *testing.T) {
	// KnownFields(true) on the YAML decoder must reject any top-level
	// key we have not declared on BuildTemplate. Phase 8B adds
	// `applyOptions` to the allowlist — this test is the regression
	// guard that an unknown key like `applyOptionsXYZ` still fails.
	yamlPayload := []byte(`schema: saveforge.build-template
version: 2
createdAt: "2026-06-08T00:00:00Z"
selection:
  profile: true
sections:
  profile:
    level: 100
applyOptionsXYZ:
  items:
    mode: addMissing
`)
	if _, err := ParseBuildTemplateYAML(yamlPayload); err == nil {
		t.Fatal("unknown top-level YAML field must be rejected by KnownFields(true)")
	}
}

func TestParseYAML_AcceptsApplyOptionsTopLevelField(t *testing.T) {
	// Sanity: the new `applyOptions` key is part of the schema and
	// must NOT be rejected as unknown.
	yamlPayload := []byte(`schema: saveforge.build-template
version: 2
createdAt: "2026-06-08T00:00:00Z"
selection:
  items: true
sections:
  items:
    entries:
      - entryID: w1
        itemID: 0x003D9700
        category: melee_armaments
        quantity: 1
        location: inventory
        upgradeKind: standard
        upgradeLevel: 25
applyOptions:
  items:
    mode: addMissing
    preserveExtraItems: true
  inventoryLayout:
    mode: ignore
  weaponLevelOverride:
    useTemplateLevels: true
`)
	tpl, err := ParseBuildTemplateYAML(yamlPayload)
	if err != nil {
		t.Fatalf("YAML with applyOptions must decode + validate: %v", err)
	}
	if tpl.ApplyOptions == nil {
		t.Fatal("ApplyOptions must be present after decode")
	}
	if tpl.ApplyOptions.Items == nil || tpl.ApplyOptions.Items.Mode != ItemApplyModeAddMissing {
		t.Fatalf("items.mode lost in decode: %+v", tpl.ApplyOptions.Items)
	}
	if tpl.ApplyOptions.WeaponLevelOverride == nil || !tpl.ApplyOptions.WeaponLevelOverride.UseTemplateLevels {
		t.Fatalf("weaponLevelOverride lost in decode: %+v", tpl.ApplyOptions.WeaponLevelOverride)
	}
}

func TestMarshalParseYAML_ItemsLayoutRoundTrip(t *testing.T) {
	src := itemsOnlyV2()
	src.Sections.Items.Entries = append(src.Sections.Items.Entries, validTalismanEntry("tal1"))
	src.Selection.InventoryLayout = &SectionSelection{All: true}
	src.Sections.InventoryLayout = &InventoryLayoutSection{
		Entries: []LayoutEntry{
			{EntryRef: "weapon_main", Position: 0},
			{EntryRef: "tal1", Position: 1},
		},
	}
	src.ApplyOptions = &ApplyOptions{
		Items:               &ItemApplyOptions{Mode: ItemApplyModeMerge, PreserveExtraItems: true},
		InventoryLayout:     &LayoutApplyOptions{Mode: LayoutApplyModeReorderOnly},
		WeaponLevelOverride: &WeaponLevelOverride{StandardOverride: u8p(10)},
	}

	data, err := MarshalBuildTemplateYAML(src)
	if err != nil {
		t.Fatalf("MarshalYAML: %v", err)
	}
	got, err := ParseBuildTemplateYAML(data)
	if err != nil {
		t.Fatalf("ParseYAML: %v\nPayload:\n%s", err, data)
	}
	if got.Sections.Items == nil || len(got.Sections.Items.Entries) != 2 {
		t.Fatalf("items lost in roundtrip: %+v", got.Sections.Items)
	}
	if got.Sections.InventoryLayout == nil || len(got.Sections.InventoryLayout.Entries) != 2 {
		t.Fatalf("inventoryLayout lost in roundtrip: %+v", got.Sections.InventoryLayout)
	}
	if got.ApplyOptions == nil || got.ApplyOptions.Items == nil || got.ApplyOptions.Items.Mode != ItemApplyModeMerge {
		t.Fatalf("applyOptions.items lost in roundtrip: %+v", got.ApplyOptions)
	}
	if got.ApplyOptions.WeaponLevelOverride == nil || got.ApplyOptions.WeaponLevelOverride.StandardOverride == nil || *got.ApplyOptions.WeaponLevelOverride.StandardOverride != 10 {
		t.Fatalf("weaponLevelOverride.standardOverride lost in roundtrip: %+v", got.ApplyOptions.WeaponLevelOverride)
	}
}
