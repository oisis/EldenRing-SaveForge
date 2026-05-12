package data

import "testing"

// Known talisman weights from EquipParamAccessory.csv.
// These values are imported by tmp/scripts/import_weights.go and must stay in sync.
//
//	rowID 1000 = Crimson Amber Medallion  → saveID 0x200003E8, weight 0.3
//	rowID 1030 = Arsenal Charm            → saveID 0x20000406, weight 1.5
//	rowID 1040 = Erdtree's Favor          → saveID 0x20000410, weight 0.9
var talismanWeightCases = []struct {
	id     uint32
	name   string
	wantGT float64 // want weight > 0 (exact float comparisons skipped due to CSV float32 repr)
}{
	{0x200003E8, "Crimson Amber Medallion", 0},
	{0x20000406, "Arsenal Charm", 0},
	{0x20000410, "Erdtree's Favor", 0},
}

func TestItemWeights_TalismansPresent(t *testing.T) {
	for _, tc := range talismanWeightCases {
		w, ok := ItemWeights[tc.id]
		if !ok {
			t.Errorf("ItemWeights[0x%08X] (%s): missing entry", tc.id, tc.name)
			continue
		}
		if w <= 0 {
			t.Errorf("ItemWeights[0x%08X] (%s): want weight > 0, got %g", tc.id, tc.name, w)
		}
	}
}

func TestItemWeights_CrimsonAmberMedallionWeight(t *testing.T) {
	const id = uint32(0x200003E8)
	w, ok := ItemWeights[id]
	if !ok {
		t.Fatal("ItemWeights[0x200003E8] (Crimson Amber Medallion): missing")
	}
	// CSV stores float32; 0.3 → 0.30000001192092896. Accept any value in (0.29, 0.31).
	if w < 0.29 || w > 0.31 {
		t.Errorf("want ~0.3 for Crimson Amber Medallion, got %g", w)
	}
}

func TestItemWeights_WeaponEntriesUnaffected(t *testing.T) {
	// Dagger 0x000F4240 weight=1.5 — ensure weapon entries still present after re-gen.
	const daggerID = uint32(0x000F4240)
	w, ok := ItemWeights[daggerID]
	if !ok {
		t.Fatal("ItemWeights[0x000F4240] (Dagger): missing")
	}
	if w <= 0 {
		t.Errorf("Dagger weight want > 0, got %g", w)
	}
}

func TestItemWeights_ArmorEntriesUnaffected(t *testing.T) {
	// Iron Kasa 0x100249F0 (head) — ensure armor entries still present after re-gen.
	const ironKasaID = uint32(0x100249F0)
	w, ok := ItemWeights[ironKasaID]
	if !ok {
		t.Fatal("ItemWeights[0x100249F0] (Iron Kasa): missing")
	}
	if w <= 0 {
		t.Errorf("Iron Kasa weight want > 0, got %g", w)
	}
}
