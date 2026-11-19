package application

import (
	"reflect"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// addedWeaponIDs returns the item IDs present in slot.GaMap that were not in
// the before snapshot, i.e. the IDs written by the add under test.
func addedWeaponIDs(before, after *core.SaveSlot) []uint32 {
	var ids []uint32
	for handle, id := range after.GaMap {
		if _, existed := before.GaMap[handle]; !existed {
			ids = append(ids, id)
		}
	}
	return ids
}

// addSingleWeapon adds one weapon through the real add path on the in-memory
// fixture and returns the single item ID that was written.
func addSingleWeapon(t *testing.T, baseID uint32, upgrade25, infuseOffset int) uint32 {
	t.Helper()
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *nativeHoleSlotFixture(t)
	before := core.CloneSlot(&app.save.Slots[0])

	res, err := app.AddItemsToCharacter(0, []uint32{baseID}, upgrade25, 0, infuseOffset, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter(0x%08X): %v", baseID, err)
	}
	if res.Added != 1 {
		t.Fatalf("result=%+v, want one item added", res)
	}
	ids := addedWeaponIDs(before, &app.save.Slots[0])
	if len(ids) != 1 {
		t.Fatalf("added IDs=%v, want exactly one", ids)
	}
	return ids[0]
}

func TestAddNoAffinityWeaponsAffinityGate(t *testing.T) {
	cases := []struct {
		name string
		id   uint32
	}{
		{"Ghostflame Torch", 0x016EF950},
		{"Beast-Repellent Torch", 0x016F2060},
		{"Crystal Knife", 0x00100590},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if db.GetItemData(tc.id).CanChangeAffinity {
				t.Fatalf("0x%08X: fixture expects CanChangeAffinity=false", tc.id)
			}
			got := addSingleWeapon(t, tc.id, 5, 100)
			if got != tc.id+5 {
				t.Fatalf("added ID=0x%08X, want 0x%08X (affinity must be ignored)", got, tc.id+5)
			}
		})
	}
}

func TestAddInfusableWeaponAffinityGate(t *testing.T) {
	const dagger = uint32(0x000F4240)
	if !db.GetItemData(dagger).CanChangeAffinity {
		t.Fatal("Dagger: fixture expects CanChangeAffinity=true")
	}
	if got := addSingleWeapon(t, dagger, 5, 100); got != dagger+100+5 {
		t.Fatalf("added ID=0x%08X, want 0x%08X", got, dagger+100+5)
	}
}

func TestAddZeroOffsetAffinityGate(t *testing.T) {
	const dagger = uint32(0x000F4240)
	if got := addSingleWeapon(t, dagger, 5, 0); got != dagger+5 {
		t.Fatalf("added ID=0x%08X, want 0x%08X", got, dagger+5)
	}
}

func TestAddInvalidOffsetAffinityGate(t *testing.T) {
	const dagger = uint32(0x000F4240)
	for _, offset := range []int{150, -100, 1300} {
		app := NewApp()
		app.save = &core.SaveFile{}
		app.save.Slots[0] = *nativeHoleSlotFixture(t)
		before := core.CloneSlot(&app.save.Slots[0])

		if _, err := app.AddItemsToCharacter(0, []uint32{dagger}, 5, 0, offset, 0, 1, 0); err == nil {
			t.Fatalf("offset %d: expected error", offset)
		}
		if !reflect.DeepEqual(&app.save.Slots[0], before) {
			t.Fatalf("offset %d: slot mutated despite error", offset)
		}
	}
}

// A non-zero offset on a weapon that forbids affinity stays a silent no-op, but
// an offset outside the known affinity list is rejected for every item.
func TestAddInvalidOffsetOnNoAffinityWeaponAffinityGate(t *testing.T) {
	const torch = uint32(0x016EF950)
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *nativeHoleSlotFixture(t)
	if _, err := app.AddItemsToCharacterWithGameLimits(0, []uint32{torch}, 0, 0, 150, 0, 1, 0); err == nil {
		t.Fatal("offset 150 on torch: expected error")
	}
}

// Every weapon row, at level 0 and max upgrade (plus every allowed affinity for
// plain rows), must resolve to the row's own family through the resolver the
// add path uses as its fail-closed guard. Rows that are themselves a catalogued
// infused variant (e.g. "Flame Art Swift Spear") resolve to their parent base
// with a non-zero affinity; for those the family must stay unchanged.
func TestAddAllWeaponBasesResolveAffinityGate(t *testing.T) {
	for _, m := range []map[uint32]data.ItemData{data.Weapons, data.Shields, data.RangedAndCatalysts} {
		for id, item := range m {
			row, ok := db.ResolveWeaponIdentity(id)
			if !ok {
				t.Errorf("%s 0x%08X: row does not resolve", item.Name, id)
				continue
			}
			finals := []uint32{id, id + item.MaxUpgrade}
			if row.AffinityOffset == 0 && db.GetItemData(id).CanChangeAffinity {
				finals = append(finals, id+100, id+100+item.MaxUpgrade)
			}
			for _, finalID := range finals {
				ident, ok := db.ResolveWeaponIdentity(finalID)
				if !ok || ident.BaseID != row.BaseID {
					t.Errorf("%s 0x%08X: final 0x%08X resolves to %+v ok=%v", item.Name, id, finalID, ident, ok)
				}
			}
		}
	}
}

// A catalogued pre-infused row (e.g. "Flame Art Swift Spear") already carries
// its affinity, so a batch offset is ignored for it like for a weapon without
// affinity support, and never fails the batch.
func TestAddPreInfusedRowIgnoresOffsetAffinityGate(t *testing.T) {
	const preInfused = uint32(0x00FC1534)
	if got := addSingleWeapon(t, preInfused, 5, 100); got != preInfused+5 {
		t.Fatalf("added ID=0x%08X, want 0x%08X", got, preInfused+5)
	}
}

func TestAddMixedBatchAffinityGate(t *testing.T) {
	const (
		preInfused = uint32(0x00FC1534)
		torch      = uint32(0x016EF950)
		dagger     = uint32(0x000F4240)
	)
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *nativeHoleSlotFixture(t)
	before := core.CloneSlot(&app.save.Slots[0])

	res, err := app.AddItemsToCharacter(0, []uint32{preInfused, torch, dagger}, 5, 0, 100, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.Added != 3 {
		t.Fatalf("result=%+v, want three items added", res)
	}
	got := map[uint32]bool{}
	for _, id := range addedWeaponIDs(before, &app.save.Slots[0]) {
		got[id] = true
	}
	for _, want := range []uint32{preInfused + 5, torch + 5, dagger + 100 + 5} {
		if !got[want] {
			t.Errorf("missing added ID 0x%08X, got %v", want, got)
		}
	}
}
