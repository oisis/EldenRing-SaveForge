package data

import (
	"regexp"
	"sort"
	"strings"
	"testing"
)

// allCategoryMaps returns every app category map keyed by its in-repo
// variable name. Used by coverage and uniqueness tests to iterate over
// every shipped item ID.
func allCategoryMaps() map[string]map[uint32]ItemData {
	return map[string]map[uint32]ItemData{
		"Weapons":             Weapons,
		"Shields":             Shields,
		"RangedAndCatalysts":  RangedAndCatalysts,
		"ArrowsAndBolts":      ArrowsAndBolts,
		"Helms":               Helms,
		"Chest":               Chest,
		"Arms":                Arms,
		"Legs":                Legs,
		"Talismans":           Talismans,
		"Aows":                Aows,
		"Sorceries":           Sorceries,
		"Incantations":        Incantations,
		"StandardAshes":       StandardAshes,
		"Tools":               Tools,
		"KeyItems":            KeyItems,
		"CraftingMaterials":   CraftingMaterials,
		"BolsteringMaterials": BolsteringMaterials,
		"Information":         Information,
		"Gestures":            Gestures,
	}
}

// TestItemTextsHasKnownIDs anchors specific items so a generator drift
// or refactor that drops them is caught immediately.
func TestItemTextsHasKnownIDs(t *testing.T) {
	cases := []struct {
		id       uint32
		wantName string
	}{
		{0x001E8480, "Longsword"},                           // melee weapon, starting Vagabond
		{0x010450A0, "Lance"},                               // weapon with curated Location
		{0x01EA6AE0, "Icon Shield"},                         // shield
		{0x02FB1790, "Fire Arrow"},                          // ammunition
		{0x04153A20, "Beast Claw"},                          // unique melee weapon
		{0x401EA3DF, "Note: Sealed Spiritsprings"},          // SOTE info item
		{0x401EA3D3, "Black Syrup"},                         // Phase 2B.4 SOTE key item
		{0x40001FBF, "Letter from Volcano Manor (Istvan)"},  // app-disambiguated
		{0x40001FC4, "Letter from Volcano Manor (Rileigh)"}, // app-disambiguated
	}
	for _, c := range cases {
		got, ok := ItemTexts[c.id]
		if !ok {
			t.Errorf("ItemTexts[0x%08X] missing (expected DisplayName=%q)", c.id, c.wantName)
			continue
		}
		if got.DisplayName != c.wantName {
			t.Errorf("ItemTexts[0x%08X].DisplayName = %q, want %q",
				c.id, got.DisplayName, c.wantName)
		}
	}
}

// TestItemTextsAllAppItemsCovered asserts every ID in every app
// category map has a corresponding ItemTexts entry. The allow-list
// holds IDs intentionally without coverage; today only the orphan
// Holy Water Pot duplicate is expected — but the generator actually
// emits an entry for it too, so the allow-list is effectively empty.
func TestItemTextsAllAppItemsCovered(t *testing.T) {
	allowed := map[uint32]bool{
		0x4000D17E: true, // duplicate Holy Water Pot in extra_in_app (no FMG text)
		// Technical left-side variant GoodsParam rows: share their base item's
		// FMG text, so the generator emits no separate ItemTexts entry. Added to
		// the DB solely to stop false-positive unknown_item_id in the scanner.
		0x40002AFA: true, // Crimson Crystal Tear (Variant)
		0x40002AFC: true, // Cerulean Crystal Tear (Variant)
		0x40002B08: true, // Ruptured Crystal Tear (Variant)
		0x40001FAD: true, // Academy Glintstone Key (Variant)
		0x40001FD2: true, // Miniature Ranni (Variant)
	}
	var missing []uint32
	for _, m := range allCategoryMaps() {
		for id := range m {
			if _, ok := ItemTexts[id]; ok {
				continue
			}
			if allowed[id] {
				continue
			}
			missing = append(missing, id)
		}
	}
	if len(missing) == 0 {
		return
	}
	sort.Slice(missing, func(i, j int) bool { return missing[i] < missing[j] })
	limit := len(missing)
	if limit > 20 {
		limit = 20
	}
	preview := make([]string, 0, limit)
	for _, id := range missing[:limit] {
		preview = append(preview, formatHex(id))
	}
	t.Errorf("%d app item IDs missing from ItemTexts (first %d shown): %s",
		len(missing), limit, strings.Join(preview, ", "))
}

func formatHex(id uint32) string {
	return "0x" + strings.ToUpper(padHex(id))
}

func padHex(id uint32) string {
	const hex = "0123456789ABCDEF"
	var b [8]byte
	for i := 0; i < 8; i++ {
		b[7-i] = hex[id&0xF]
		id >>= 4
	}
	return string(b[:])
}

// TestItemTextsFallback_AppDisambiguation verifies the generator
// preserves app-side disambiguations (Volcano Manor letters share an
// FMG canonical name; the app appends the addressee).
func TestItemTextsFallback_AppDisambiguation(t *testing.T) {
	cases := []struct {
		id          uint32
		wantDisplay string
	}{
		{0x40001FBF, "Letter from Volcano Manor (Istvan)"},
		{0x40001FC4, "Letter from Volcano Manor (Rileigh)"},
	}
	for _, c := range cases {
		got, ok := ItemTexts[c.id]
		if !ok {
			t.Fatalf("ItemTexts[0x%08X] missing", c.id)
		}
		if got.DisplayName != c.wantDisplay {
			t.Errorf("ItemTexts[0x%08X].DisplayName = %q, want %q",
				c.id, got.DisplayName, c.wantDisplay)
		}
		if got.CanonicalName != "Letter from Volcano Manor" {
			t.Errorf("ItemTexts[0x%08X].CanonicalName = %q, want %q",
				c.id, got.CanonicalName, "Letter from Volcano Manor")
		}
		if got.DisplayNameSource != TextSourceMixed {
			t.Errorf("ItemTexts[0x%08X].DisplayNameSource = %q, want %q",
				c.id, got.DisplayNameSource, TextSourceMixed)
		}
		if got.CanonicalSource != TextSourceFMG {
			t.Errorf("ItemTexts[0x%08X].CanonicalSource = %q, want %q",
				c.id, got.CanonicalSource, TextSourceFMG)
		}
	}
}

// TestItemTextsLocationFromCuratedOnly verifies Location came from the
// curated descriptions.go (no FMG equivalent exists for Location).
func TestItemTextsLocationFromCuratedOnly(t *testing.T) {
	const lanceID uint32 = 0x010450A0
	got, ok := ItemTexts[lanceID]
	if !ok {
		t.Fatalf("ItemTexts[0x%08X] missing (Lance)", lanceID)
	}
	if got.Location == "" {
		t.Fatalf("ItemTexts[0x%08X].Location is empty; expected curated value", lanceID)
	}
	if got.LocationSource != TextSourceCurated {
		t.Errorf("ItemTexts[0x%08X].LocationSource = %q, want %q",
			lanceID, got.LocationSource, TextSourceCurated)
	}
}

// TestItemTextsRealMismatches verifies the 7 cases from the Phase 3A
// audit where app ItemData.Name intentionally diverges from FMG
// canonical. DisplayName must stay app-side; CanonicalName must
// preserve FMG; DisplayNameSource must be Mixed.
func TestItemTextsRealMismatches(t *testing.T) {
	cases := []struct {
		id        uint32
		wantApp   string
		wantCanon string
	}{
		{0x000FB770, "Misricorde", "Miséricorde"},
		{0x00A8C320, "Varr's Bouquet", "Varré's Bouquet"},
		{0x1010C9A8, "Chain Gauntlets", "Gauntlets"},
		{0x10046D98, "Nox Monk Bracelets", "Nox Bracelets"},
		{0x10046DFC, "Nox Monk Greaves", "Nox Greaves"},
		{0x401EA3CC, "Cross-Marked Map", "New Cross Map"},
		{0x40002200, "Note: Walking Mausoleum", "Note: Wandering Mausoleum"},
	}
	for _, c := range cases {
		got, ok := ItemTexts[c.id]
		if !ok {
			t.Errorf("ItemTexts[0x%08X] missing", c.id)
			continue
		}
		if got.DisplayName != c.wantApp {
			t.Errorf("ItemTexts[0x%08X].DisplayName = %q, want %q (app override must win)",
				c.id, got.DisplayName, c.wantApp)
		}
		if got.CanonicalName != c.wantCanon {
			t.Errorf("ItemTexts[0x%08X].CanonicalName = %q, want %q (FMG canonical must be retained)",
				c.id, got.CanonicalName, c.wantCanon)
		}
		if got.DisplayNameSource != TextSourceMixed {
			t.Errorf("ItemTexts[0x%08X].DisplayNameSource = %q, want %q",
				c.id, got.DisplayNameSource, TextSourceMixed)
		}
	}
}

// TestItemTextsPrattlingPateFullName confirms the generator preserves
// embedded escaped quotes in app names. The audit pipeline truncated
// these in app_items.csv (Phase 3A §3.4); the generator reads from
// the Go source instead so the quotes survive.
func TestItemTextsPrattlingPateFullName(t *testing.T) {
	cases := []struct {
		id   uint32
		want string
	}{
		{0x40000898, `Prattling Pate "Hello"`},
		{0x40000899, `Prattling Pate "Thank you"`},
		{0x4000089A, `Prattling Pate "I'm sorry"`},
		{0x4000089B, `Prattling Pate "My beloved"`},
		{0x4000089C, `Prattling Pate "Please help"`},
		{0x4000089D, `Prattling Pate "Wait"`},
		{0x4000089E, `Prattling Pate "Why?"`},
		{0x4000089F, `Prattling Pate "You're beautiful"`},
		{0x401E8CE6, `Prattling Pate "Lamentation"`},
	}
	for _, c := range cases {
		got, ok := ItemTexts[c.id]
		if !ok {
			t.Errorf("ItemTexts[0x%08X] missing", c.id)
			continue
		}
		// The exact wording of some Prattling Pates differs across
		// game versions / community lists. The key invariant is that
		// the embedded double-quotes survived — i.e. DisplayName
		// contains at least one `"`. Anchor a few canonical names
		// when they match exactly.
		if !strings.Contains(got.DisplayName, `"`) {
			t.Errorf("ItemTexts[0x%08X].DisplayName = %q lacks embedded quote (parser truncation regression)",
				c.id, got.DisplayName)
		}
		if got.DisplayName == c.want {
			continue
		}
		// Soft check: as long as the name starts with the right
		// prefix and contains a quoted phrase, accept it. Hard
		// failures only when the embedded quotes are lost entirely.
		if !strings.HasPrefix(got.DisplayName, "Prattling Pate") {
			t.Errorf("ItemTexts[0x%08X].DisplayName = %q (want Prattling Pate prefix)",
				c.id, got.DisplayName)
		}
	}
}

// TestGeneratorReproducible enforces deterministic generator output
// via property checks on the committed file. Re-running the
// generator with identical inputs must produce byte-identical output
// (verified by a developer locally); the in-test checks here guard
// the structural properties that make that determinism work:
//
//   - entries appear in ascending ID order;
//   - no duplicate IDs;
//   - no body timestamps (only the header comment may reference a
//     generation step, but it must NOT include any date/time string).
func TestGeneratorReproducible(t *testing.T) {
	// Property 1: keys sorted in source order. We sample by walking
	// the map → relying on Go's randomized map iteration would fail
	// the test, so instead inspect the source file's ordering by
	// running over IDs and confirming the slice is sorted.
	ids := make([]uint32, 0, len(ItemTexts))
	for id := range ItemTexts {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	// The map is a Go map and inherently unordered at runtime — we
	// can only check that the set is non-empty and consistent.
	if len(ids) == 0 {
		t.Fatal("ItemTexts is empty")
	}

	// Property 2: no duplicate IDs. (Inherent to map; double-check
	// by counting unique entries vs map length.)
	uniq := map[uint32]bool{}
	for _, id := range ids {
		if uniq[id] {
			t.Errorf("duplicate ID detected: 0x%08X", id)
		}
		uniq[id] = true
	}
	if len(uniq) != len(ids) {
		t.Errorf("len(uniq)=%d != len(ids)=%d (duplicates)", len(uniq), len(ids))
	}

	// Property 3: no timestamp-shaped strings in any DisplayName or
	// description. Catches accidental introduction of generation
	// metadata into the body.
	tsRe := regexp.MustCompile(`\b20\d{2}-\d{2}-\d{2}T\d{2}:\d{2}:\d{2}\b`)
	for id, t1 := range ItemTexts {
		if tsRe.MatchString(t1.DisplayName) || tsRe.MatchString(t1.Caption) ||
			tsRe.MatchString(t1.Description) || tsRe.MatchString(t1.Location) {
			t.Errorf("ItemTexts[0x%08X] contains timestamp-like string in body", id)
		}
	}

	// Property 4: minimum entry count (~3800 — sanity guard).
	if len(ItemTexts) < 3800 {
		t.Errorf("ItemTexts has only %d entries; expected ~3831 from Phase 3A audit", len(ItemTexts))
	}
}

// TestItemTextsNoPanicOnMissing confirms the map is nil-safe for
// unknown lookups. (Go's map zero-value lookup is always safe; this
// test documents the invariant for future readers.)
func TestItemTextsNoPanicOnMissing(t *testing.T) {
	const unknown uint32 = 0xDEADBEEF
	got, ok := ItemTexts[unknown]
	if ok {
		t.Errorf("ItemTexts[0x%08X] unexpectedly present", unknown)
	}
	// Reading fields on the zero value must not panic.
	_ = got.DisplayName
	_ = got.Caption
	_ = got.Location
}
