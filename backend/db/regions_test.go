package db

import (
	"sort"
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// Expected counts for the CURATED invasion/blue-summon allowlist (data.Regions).
// This is NOT the full PlayRegionParam table (594 rows) and NOT the full set of
// raw region IDs a save may carry — it is the subset confirmed as standard
// invasion / blue targets, sourced 1:1 from the Elden-Ring-CT-TGA dedicated
// "Invasion Regions" list (208 base + 66 DLC = 274). Multiplayer hubs and
// colosseums are excluded by construction.
const (
	wantRegionsTotal = 274
	wantRegionsBase  = 208
	wantRegionsDLC   = 66
)

func TestRegionsCompleteness(t *testing.T) {
	if got := len(data.Regions); got != wantRegionsTotal {
		t.Fatalf("len(data.Regions) = %d, want %d", got, wantRegionsTotal)
	}
	var base, dlc int
	for id, r := range data.Regions {
		if r.DLC {
			dlc++
		} else {
			base++
		}
		if r.Name == "" {
			t.Errorf("region %d has empty Name", id)
		}
		if r.Area == "" {
			t.Errorf("region %d has empty Area", id)
		}
	}
	if base != wantRegionsBase {
		t.Errorf("base regions = %d, want %d", base, wantRegionsBase)
	}
	if dlc != wantRegionsDLC {
		t.Errorf("DLC regions = %d, want %d", dlc, wantRegionsDLC)
	}
}

// TestRegionsKeyDLCPresent ensures representative DLC region IDs from every DLC
// sub-system are present, so "Unlock All" prepares the character for SotE PvP.
func TestRegionsKeyDLCPresent(t *testing.T) {
	keyDLC := map[uint32]string{
		6800000: "Gravesite Plain",         // DLC open-world (overworld 68xxxxx)
		6900000: "Scadu Altus",             // DLC open-world (overworld 69xxxxx)
		2000000: "Belurat",                 // DLC legacy dungeon
		2001000: "Enir-Ilim",               // DLC legacy dungeon
		2100010: "Scaduview",               // DLC Shadow Keep / Scaduview
		2101000: "Storehouse",              // DLC Specimen Storehouse
		2200000: "Stone Coffin Fissure",    // DLC special zone
		2500000: "Finger Birthing Grounds", // DLC special zone
		4000001: "Fog Rift Catacombs",      // DLC catacomb (4xxxxxx)
	}
	for id, sub := range keyDLC {
		r, ok := data.Regions[id]
		if !ok {
			t.Errorf("DLC region %d (%s) missing", id, sub)
			continue
		}
		if !r.DLC {
			t.Errorf("region %d (%s) not flagged DLC", id, r.Name)
		}
		if !data.IsDLCRegion(id) {
			t.Errorf("IsDLCRegion(%d) = false, want true", id)
		}
	}
}

// TestRegionConflictResolved locks in the BonfireFlag-verified correction of the
// 6800000 / 6900000 mapping conflict against the TGA game-data table.
func TestRegionConflictResolved(t *testing.T) {
	cases := []struct {
		id   uint32
		name string
		area string
	}{
		{6800000, "Gravesite Plain", "Land of Shadow"},
		{6900000, "Scadu Altus", "Land of Shadow"},
	}
	for _, c := range cases {
		r, ok := data.Regions[c.id]
		if !ok {
			t.Fatalf("region %d missing", c.id)
		}
		if r.Name != c.name {
			t.Errorf("region %d Name = %q, want %q", c.id, r.Name, c.name)
		}
		if r.Area != c.area {
			t.Errorf("region %d Area = %q, want %q", c.id, r.Area, c.area)
		}
		if !r.DLC {
			t.Errorf("region %d should be DLC", c.id)
		}
	}
	// The real Haligtree interior (1500000, Malenia) must exist as a base region —
	// it is what 6800000 previously (incorrectly) labelled.
	if r, ok := data.Regions[1500000]; !ok || r.DLC {
		t.Errorf("Haligtree base region 1500000 missing or wrongly DLC: %+v", r)
	}
}

// TestIsDLCRegion verifies the data-driven DLC predicate (the old numeric range
// 6900000–6999999 is wrong: DLC IDs are non-contiguous).
func TestIsDLCRegion(t *testing.T) {
	if !data.IsDLCRegion(2000000) {
		t.Error("IsDLCRegion(2000000) = false, want true (DLC legacy dungeon outside 69xxxxx)")
	}
	if data.IsDLCRegion(6100000) {
		t.Error("IsDLCRegion(6100000) = true, want false (Limgrave base)")
	}
	if data.IsDLCRegion(9999999) {
		t.Error("IsDLCRegion(9999999) = true, want false (unknown ID)")
	}
}

// TestRegionsNoFabricatedIDs guards against re-introducing the legacy SaveForge-only
// IDs that game data (PlayRegionParam) proved are not real PlayRegion IDs: the
// underground 6600xxx / Farum 6700xxx blocks, 6502001/2, Roundtable Hold 1101000,
// and the fabricated DLC block 6900001–6900006 (the real Scadu Altus is 6900000).
func TestRegionsNoFabricatedIDs(t *testing.T) {
	fabricated := []uint32{
		1101000, 6502001, 6502002,
		6600000, 6600001, 6600002, 6600003, 6600004, 6600005, 6600006, 6600007,
		6700000, 6700001, 6700002,
		6900001, 6900002, 6900003, 6900004, 6900005, 6900006,
	}
	for _, id := range fabricated {
		if _, ok := data.Regions[id]; ok {
			t.Errorf("fabricated region ID %d present (not a PlayRegionParam Row ID)", id)
		}
	}
}

// TestNoForbiddenPvPLocations guards the curated allowlist against ever gaining a
// multiplayer hub, colosseum, or other non-standard-PvP location by name. The TGA
// source already excludes these (Roundtable Hold has no PlayRegion row; colosseums
// use separate matchmaking), so this is a regression fence for future edits.
// Concrete IDs are NOT asserted for categories that have no PlayRegion row — we do
// not guess IDs; a name-substring scan is the strongest evidence-based guard.
func TestNoForbiddenPvPLocations(t *testing.T) {
	forbidden := []string{
		"roundtable", "table of lost grace", "colosseum", "coliseum",
		"chapel of anticipation", "tutorial",
	}
	for id, r := range data.Regions {
		name := strings.ToLower(r.Name)
		area := strings.ToLower(r.Area)
		for _, f := range forbidden {
			if strings.Contains(name, f) || strings.Contains(area, f) {
				t.Errorf("region %d (%q / %q) matches forbidden PvP-location pattern %q", id, r.Name, r.Area, f)
			}
		}
	}
}

// TestGetAllRegionsUniqueSorted covers the data feeding the WorldTab "Unlock All"
// action: every region ID is unique, the entry list matches the map size, and it
// is ordered by Area then Name (the GetAllRegions contract).
func TestGetAllRegionsUniqueSorted(t *testing.T) {
	entries := GetAllRegions()
	if len(entries) != len(data.Regions) {
		t.Fatalf("GetAllRegions len = %d, want %d", len(entries), len(data.Regions))
	}
	seen := make(map[uint32]bool, len(entries))
	for _, e := range entries {
		if seen[e.ID] {
			t.Errorf("duplicate region ID %d", e.ID)
		}
		seen[e.ID] = true
	}
	// Ordering: Area asc, then Name asc.
	ordered := sort.SliceIsSorted(entries, func(i, j int) bool {
		if entries[i].Area != entries[j].Area {
			return entries[i].Area < entries[j].Area
		}
		return entries[i].Name < entries[j].Name
	})
	if !ordered {
		t.Error("GetAllRegions not sorted by Area then Name")
	}
}

// TestUnlockAllRegionIDsNoDuplicates simulates the ID slice the WorldTab
// "Unlock All" button passes to BulkSetUnlockedRegions: it must be duplicate-free
// (core.SetUnlockedRegions also dedupes + sorts downstream).
func TestUnlockAllRegionIDsNoDuplicates(t *testing.T) {
	ids := make([]uint32, 0, len(data.Regions))
	for id := range data.Regions {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	for i := 1; i < len(ids); i++ {
		if ids[i] == ids[i-1] {
			t.Fatalf("duplicate region ID in Unlock All set: %d", ids[i])
		}
	}
	if len(ids) != wantRegionsTotal {
		t.Errorf("Unlock All set size = %d, want %d", len(ids), wantRegionsTotal)
	}
}
