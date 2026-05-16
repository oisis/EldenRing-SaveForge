package main

import (
	"os"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// somberGreatshieldFixture loads the PC test save and returns an App wired to
// it plus the first non-empty slot index with enough free capacity to add the
// full somber-greatshield set. The integration path exercises
// AddItemsToCharacter end-to-end: prepared ID computation, GaItem allocation,
// RebuildSlotFull, re-parse, and final placement in inventory.
//
// Tests skip when the save file is absent (CI without `tmp/save/`).
func somberGreatshieldFixture(t *testing.T) (*App, int) {
	t.Helper()
	const savePath = "tmp/save/ER0000.sl2"
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		t.Skipf("test save not found: %s", savePath)
	}
	save, err := core.LoadSave(savePath)
	if err != nil {
		t.Fatalf("LoadSave: %v", err)
	}
	app := NewApp()
	app.save = save

	for i := 0; i < 10; i++ {
		s := &save.Slots[i]
		if s.Version == 0 {
			continue
		}
		emptyGa := 0
		for _, g := range s.GaItems {
			if g.IsEmpty() {
				emptyGa++
			}
		}
		emptyInv := 0
		for _, it := range s.Inventory.CommonItems {
			if it.GaItemHandle == 0 || it.GaItemHandle == 0xFFFFFFFF {
				emptyInv++
			}
		}
		// Need ≥20 free GaItems (10 shields × inv-only) and ≥20 free CommonItem
		// rows — large headroom guards against vanilla-save variation.
		if emptyGa >= 20 && emptyInv >= 20 {
			return app, i
		}
	}
	t.Skip("no active slot with sufficient free GaItem/CommonItem capacity")
	return nil, 0
}

// gaMapHas returns true if itemID is present anywhere in slot.GaMap.
func gaMapHas(slot *core.SaveSlot, itemID uint32) bool {
	for _, id := range slot.GaMap {
		if id == itemID {
			return true
		}
	}
	return false
}

const iconShieldID = uint32(0x01EA6AE0) // 32140000 — somber Greatshield

// TestAddIconShield_DefaultUsesBaseID confirms that calling AddItemsToCharacter
// for Icon Shield with all upgrade/infuse parameters at zero stores the bare
// base ID 0x01EA6AE0 in GaItems and GaMap — i.e. no affinity, no upgrade.
func TestAddIconShield_DefaultUsesBaseID(t *testing.T) {
	app, slotIdx := somberGreatshieldFixture(t)
	slot := &app.save.Slots[slotIdx]

	res, err := app.AddItemsToCharacter(slotIdx, []uint32{iconShieldID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.CapHit != "" {
		t.Fatalf("capacity hit on default add: %+v", res)
	}
	if !gaMapHas(slot, iconShieldID) {
		t.Fatalf("Icon Shield base ID 0x%08X not in GaMap after default add", iconShieldID)
	}
	// Guard: no spurious Heavy variant got written (would indicate the somber
	// branch was bypassed and the infuse path ran with infuseOffset=0+100=100).
	if gaMapHas(slot, iconShieldID+100) {
		t.Errorf("unexpected affinity variant 0x%08X present in GaMap", iconShieldID+100)
	}
}

// TestAddIconShield_InfuseOffsetIsIgnored is the core regression: with MaxUpgrade=10
// the somber branch must run regardless of infuseOffset, so even an explicit
// Heavy offset (+100) leaves the final ID at the base value. Before the fix
// MaxUpgrade was 25, weaponCategorySupportsInfusion("shields")==true ran the
// infusion path, and the save received 0x01EA6B44 — an ID with no row in
// EquipParamWeapon → invisible in-game.
func TestAddIconShield_InfuseOffsetIsIgnored(t *testing.T) {
	app, slotIdx := somberGreatshieldFixture(t)
	slot := &app.save.Slots[slotIdx]

	res, err := app.AddItemsToCharacter(slotIdx, []uint32{iconShieldID}, 0, 0, 100, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.CapHit != "" {
		t.Fatalf("capacity hit: %+v", res)
	}
	if !gaMapHas(slot, iconShieldID) {
		t.Errorf("Icon Shield base ID 0x%08X missing — infuse path was taken", iconShieldID)
	}
	if gaMapHas(slot, iconShieldID+100) {
		t.Errorf("affinity variant 0x%08X written — infuse offset was not ignored", iconShieldID+100)
	}
}

// TestAddIconShield_Upgrade10MaxAdds10 checks that the upgrade10 slider is
// applied raw (+1 stride per level, like every weapon ID in Elden Ring) on top
// of the base somber ID — so +10 produces 0x01EA6AE0 + 10 = 0x01EA6AEA.
func TestAddIconShield_Upgrade10MaxAdds10(t *testing.T) {
	app, slotIdx := somberGreatshieldFixture(t)
	slot := &app.save.Slots[slotIdx]

	wantID := iconShieldID + 10 // Icon Shield +10
	res, err := app.AddItemsToCharacter(slotIdx, []uint32{iconShieldID}, 0, 10, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.CapHit != "" {
		t.Fatalf("capacity hit: %+v", res)
	}
	if !gaMapHas(slot, wantID) {
		t.Errorf("Icon Shield +10 (0x%08X) missing from GaMap", wantID)
	}
	if gaMapHas(slot, iconShieldID) {
		t.Errorf("unexpected base-level Icon Shield 0x%08X present alongside +10", iconShieldID)
	}
}

// somberGreatshieldCases mirrors backend/db/data/shields_somber_test.go so the
// integration suite covers every shield the fix touched. Decimal IDs map
// back to EquipParamWeapon rows with reinforceTypeId=8300 and gemMountType=0.
var somberGreatshieldCases = []struct {
	id    uint32
	decID uint32
	name  string
}{
	{0x01E8BD30, 32030000, "Crucible Hornshield"},
	{0x01E8E440, 32040000, "Dragonclaw Shield"},
	{0x01E98080, 32080000, "Erdtree Greatshield"},
	{0x01EA1CC0, 32120000, "Jellyfish Shield"},
	{0x01EA6AE0, 32140000, "Icon Shield"},
	{0x01EA91F0, 32150000, "One-Eyed Shield"},
	{0x01EAB900, 32160000, "Visage Shield"},
	{0x01EBA360, 32220000, "Ant's Skull Plate"},
	{0x01F03740, 32520000, "Verdigris Greatshield"},
}

// TestAddSomberGreatshields_InfuseOffsetIgnoredForAll batches the whole set
// with Heavy (+100) infusion and confirms each one lands at its base ID and
// not at the affinity variant.
func TestAddSomberGreatshields_InfuseOffsetIgnoredForAll(t *testing.T) {
	app, slotIdx := somberGreatshieldFixture(t)
	slot := &app.save.Slots[slotIdx]

	ids := make([]uint32, len(somberGreatshieldCases))
	for i, c := range somberGreatshieldCases {
		ids[i] = c.id
	}

	res, err := app.AddItemsToCharacter(slotIdx, ids, 0, 0, 100, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.CapHit != "" {
		t.Fatalf("capacity hit: %+v", res)
	}

	for _, c := range somberGreatshieldCases {
		if c.id != c.decID {
			t.Errorf("table-row sanity: 0x%08X (%s) != %d decimal", c.id, c.name, c.decID)
		}
		item, _ := db.GetItemDataFuzzy(c.id)
		if item.MaxUpgrade != 10 {
			t.Errorf("%s (0x%08X): DB MaxUpgrade=%d, want 10", c.name, c.id, item.MaxUpgrade)
		}
		if !gaMapHas(slot, c.id) {
			t.Errorf("%s base ID 0x%08X missing — infuse path ran", c.name, c.id)
		}
		if gaMapHas(slot, c.id+100) {
			t.Errorf("%s affinity variant 0x%08X written — infuse offset not ignored", c.name, c.id+100)
		}
	}
}

// TestAddSomberGreatshields_Upgrade10AppliesToAll batches the whole set with
// upgrade10=10 and confirms each one lands at baseID+10.
func TestAddSomberGreatshields_Upgrade10AppliesToAll(t *testing.T) {
	app, slotIdx := somberGreatshieldFixture(t)
	slot := &app.save.Slots[slotIdx]

	ids := make([]uint32, len(somberGreatshieldCases))
	for i, c := range somberGreatshieldCases {
		ids[i] = c.id
	}

	res, err := app.AddItemsToCharacter(slotIdx, ids, 0, 10, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.CapHit != "" {
		t.Fatalf("capacity hit: %+v", res)
	}

	for _, c := range somberGreatshieldCases {
		want := c.id + 10
		if !gaMapHas(slot, want) {
			t.Errorf("%s +10 (0x%08X) missing from GaMap", c.name, want)
		}
	}
}
