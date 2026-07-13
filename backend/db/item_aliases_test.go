package db

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// aliasCases pairs a technical alias ID with the canonical name it must resolve
// to via GetItemDataFuzzy.
var aliasCases = []struct {
	alias uint32
	name  string
}{
	{0x400000BF, "Godrick's Great Rune"},
	{0x400000C0, "Radahn's Great Rune"},
	{0x400000C1, "Morgott's Great Rune"},
	{0x400000C2, "Rykard's Great Rune"},
	{0x400000C3, "Mohg's Great Rune"},
	{0x400000C4, "Malenia's Great Rune"},
	{0x40001FDA, "Lord of Blood's Favor"},
	{0x40002004, "Unalloyed Gold Needle"},
	{0x4000230F, "Unalloyed Gold Needle"},
	{0x40000400, "Flask of Crimson Tears +12"},
	{0x40000432, "Flask of Cerulean Tears +12"},
}

// TestItemAliases_ResolveToCanonical verifies each alias resolves to its
// canonical metadata via the fuzzy lookup, returning the canonical baseID.
func TestItemAliases_ResolveToCanonical(t *testing.T) {
	for _, c := range aliasCases {
		canonical, ok := data.TechnicalItemAliases[c.alias]
		if !ok {
			t.Fatalf("alias 0x%08X missing from TechnicalItemAliases", c.alias)
		}
		got, baseID := GetItemDataFuzzy(c.alias)
		if got.Name != c.name {
			t.Errorf("GetItemDataFuzzy(0x%08X).Name = %q, want %q", c.alias, got.Name, c.name)
		}
		if baseID != canonical {
			t.Errorf("GetItemDataFuzzy(0x%08X) baseID = 0x%08X, want canonical 0x%08X", c.alias, baseID, canonical)
		}
	}
}

// TestItemAliases_RawNotNormalized proves the lookup does not inject the alias
// as a DB entry: the exact lookup stays empty, so nothing rewrites the save's
// raw ID and the alias never pollutes globalItemIndex.
func TestItemAliases_RawNotNormalized(t *testing.T) {
	for _, c := range aliasCases {
		if exact := GetItemData(c.alias); exact.Name != "" {
			t.Errorf("GetItemData(0x%08X) exact = %q, want empty (alias must not be a DB entry)", c.alias, exact.Name)
		}
	}
}

// TestItemAliases_NotInPicker ensures alias IDs are never returned as separate
// addable items by the category/picker API.
func TestItemAliases_NotInPicker(t *testing.T) {
	aliasSet := map[uint32]bool{}
	for a := range data.TechnicalItemAliases {
		aliasSet[a] = true
	}
	for _, cat := range []string{"key_items", "tools", "all"} {
		for _, e := range GetItemsByCategory(cat, "PC") {
			if aliasSet[e.ID] {
				t.Errorf("category %q lists alias ID 0x%08X (%q) as a separate item", cat, e.ID, e.Name)
			}
		}
	}
}

// TestItemAliases_HandlePrefixed proves goods handles (prefix 0xB0) resolve
// through the alias map after Handle-to-ItemID conversion.
func TestItemAliases_HandlePrefixed(t *testing.T) {
	cases := []struct {
		handle    uint32
		name      string
		canonical uint32
	}{
		{0xB00000C2, "Rykard's Great Rune", 0x40001FD7},
		{0xB0000400, "Flask of Crimson Tears +12", 0x40000401},
	}
	for _, c := range cases {
		got, baseID := GetItemDataFuzzy(c.handle)
		if got.Name != c.name {
			t.Errorf("GetItemDataFuzzy(0x%08X).Name = %q, want %q", c.handle, got.Name, c.name)
		}
		if baseID != c.canonical {
			t.Errorf("GetItemDataFuzzy(0x%08X) baseID = 0x%08X, want canonical 0x%08X", c.handle, baseID, c.canonical)
		}
	}
}

// TestItemAliases_SingleSourceOfTruth is the invariant guard: for every entry
// the alias must differ from its canonical, the alias must NOT be an exact DB
// entry (so it can never become a second metadata source), and the canonical
// must be a real DB entry (so resolution actually lands somewhere).
func TestItemAliases_SingleSourceOfTruth(t *testing.T) {
	for alias, canonical := range data.TechnicalItemAliases {
		if alias == canonical {
			t.Errorf("alias 0x%08X equals its canonical ID", alias)
		}
		if got := GetItemData(alias); got.Name != "" {
			t.Errorf("alias 0x%08X resolves via exact DB lookup to %q, want empty (must not be a DB entry)", alias, got.Name)
		}
		if got := GetItemData(canonical); got.Name == "" {
			t.Errorf("canonical 0x%08X (for alias 0x%08X) has no exact DB entry, want a real item", canonical, alias)
		}
	}
}

// TestItemAliases_UnrelatedStillUnknown guards that adding aliases did not turn
// a genuinely absent ID into a resolvable one.
func TestItemAliases_UnrelatedStillUnknown(t *testing.T) {
	const bogus = uint32(0x40009999)
	if got, _ := GetItemDataFuzzy(bogus); got.Name != "" {
		t.Errorf("GetItemDataFuzzy(0x%08X).Name = %q, want empty", bogus, got.Name)
	}
}
