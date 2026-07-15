package main

import (
	"reflect"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// fitCap builds a CapacityReport that, on its own, satisfies every non-GaItem
// eligibility condition for gaItemFullCTA. Individual tests then break exactly
// one condition to prove it is load-bearing.
func fitCap() core.CapacityReport {
	return core.CapacityReport{
		CapHit:           "gaitem_full",
		FreeInv:          10,
		FreeStorage:      10,
		FreeGaItems:      5,
		FreeGaItemCursor: 0,
		FreeGaItemData:   10,
		NeededInv:        1,
		NeededStorage:    0,
		NeededGaItems:    1,
		NeededGaItemData: 0,
	}
}

// fragmentedRepackSlot returns a healthy, synthetically fragmented slot whose
// non-mutating preflight recovers positive capacity. It is built in memory (no
// on-disk save fixture) so the CTA logic is exercised deterministically: a hole
// precedes the single weapon record and the allocator cursor is exhausted, so
// stable compaction can reclaim the leading empties.
func fragmentedRepackSlot(t *testing.T) *core.SaveSlot {
	t.Helper()
	slot := &core.SaveSlot{
		Data:              make([]byte, core.SlotSize),
		Version:           1,
		GaMap:             make(map[uint32]uint32),
		MagicOffset:       1000,
		InventoryEnd:      core.GaItemsStart,
		PlayerDataOffset:  1000,
		FaceDataOffset:    2000,
		StorageBoxOffset:  2000,
		GaItemDataOffset:  0x8000,
		SectionMap:        []core.SectionRange{{Name: "all", Start: 0, End: core.SlotSize}},
		NextAoWIndex:      0,
		NextArmamentIndex: 4, // cursor exhausted: no room to place a new armament
		NextGaItemHandle:  2,
		PartGaItemHandle:  0x80,
	}
	weapon := core.GaItemFull{Handle: core.ItemTypeWeapon | 1, ItemID: 1}
	slot.GaItems = []core.GaItemFull{{}, weapon, {}, {}}
	slot.GaMap[weapon.Handle] = weapon.ItemID
	if pf := core.PreflightGaItemRepack(slot); len(pf.Blockers) != 0 || pf.Analysis.Recovered <= 0 {
		t.Fatalf("synthetic slot is not a recoverable fragment: blockers=%d recovered=%d", len(pf.Blockers), pf.Analysis.Recovered)
	}
	return slot
}

func TestGaItemFullCTA_FragmentedSlotIsEligible(t *testing.T) {
	slot := fragmentedRepackSlot(t)
	pf := core.PreflightGaItemRepack(slot)

	cap := fitCap()
	cap.FreeGaItems = 5
	cap.FreeGaItemCursor = 0
	capacity, cta := gaItemFullCTA(slot, cap, false)

	// Capacity is mapped straight from the report, including usable = min(...).
	if capacity != (GaItemCapacity{PhysicalEmpty: 5, CursorRoom: 0, Usable: 0}) {
		t.Fatalf("capacity=%+v, want physical=5 cursor=0 usable=0", capacity)
	}
	if !cta.Eligible {
		t.Fatalf("cta=%+v, want eligible", cta)
	}
	if cta.Recovered != pf.Analysis.Recovered || cta.Recovered <= 0 {
		t.Fatalf("recovered=%d, want positive preflight recovery %d", cta.Recovered, pf.Analysis.Recovered)
	}
}

func TestGaItemFullCTA_BatchTooLargeAfterRepackIsIneligible(t *testing.T) {
	slot := fragmentedRepackSlot(t)
	pf := core.PreflightGaItemRepack(slot)

	// The batch needs more records than repack could ever free — e.g. no physical
	// empties would remain. Recovered must still be preserved for the safe preflight.
	cap := fitCap()
	cap.NeededGaItems = pf.Analysis.ProjectedAfter.Usable + 1
	_, cta := gaItemFullCTA(slot, cap, false)
	if cta.Eligible {
		t.Fatalf("cta=%+v, want ineligible when batch exceeds projected usable", cta)
	}
	if cta.Recovered != pf.Analysis.Recovered {
		t.Fatalf("recovered=%d, want preserved %d", cta.Recovered, pf.Analysis.Recovered)
	}
}

func TestGaItemFullCTA_NoRecoveryIsIneligible(t *testing.T) {
	// An already-compact slot: the single weapon sits at index 0 and the cursor is
	// not the limiter, so preflight recovers nothing. This also models "no physical
	// empty records to reclaim" — repack cannot help either way.
	slot := fragmentedRepackSlot(t)
	weapon := core.GaItemFull{Handle: core.ItemTypeWeapon | 1, ItemID: 1}
	slot.GaItems = []core.GaItemFull{weapon, {}, {}, {}}
	slot.NextArmamentIndex = 1
	pf := core.PreflightGaItemRepack(slot)
	if len(pf.Blockers) != 0 || pf.Analysis.Recovered != 0 {
		t.Fatalf("compact slot: blockers=%d recovered=%d, want 0/0", len(pf.Blockers), pf.Analysis.Recovered)
	}

	_, cta := gaItemFullCTA(slot, fitCap(), false)
	if cta.Eligible || cta.Recovered != 0 {
		t.Fatalf("cta=%+v, want ineligible with zero recovery", cta)
	}
}

func TestGaItemFullCTA_RemainingContainerLimitsAreIneligible(t *testing.T) {
	slot := fragmentedRepackSlot(t)
	pf := core.PreflightGaItemRepack(slot)

	cases := map[string]func(*core.CapacityReport){
		"inventory":  func(c *core.CapacityReport) { c.NeededInv = c.FreeInv + 1 },
		"storage":    func(c *core.CapacityReport) { c.NeededStorage = c.FreeStorage + 1 },
		"gaitemdata": func(c *core.CapacityReport) { c.NeededGaItemData = c.FreeGaItemData + 1 },
	}
	for name, breakOne := range cases {
		t.Run(name, func(t *testing.T) {
			cap := fitCap()
			breakOne(&cap)
			_, cta := gaItemFullCTA(slot, cap, false)
			if cta.Eligible {
				t.Fatalf("cta=%+v, want ineligible when %s limit blocks the batch", cta, name)
			}
			// A remaining independent limit does not invalidate the safe recovery estimate.
			if cta.Recovered != pf.Analysis.Recovered {
				t.Fatalf("recovered=%d, want preserved %d", cta.Recovered, pf.Analysis.Recovered)
			}
		})
	}
}

func TestGaItemFullCTA_PreflightRefusalIsIneligible(t *testing.T) {
	// A zero slot fails the structural preflight gate: no safe recovery estimate.
	_, cta := gaItemFullCTA(&core.SaveSlot{}, fitCap(), false)
	if cta.Eligible || cta.Recovered != 0 {
		t.Fatalf("cta=%+v, want ineligible with zero recovery on refusal", cta)
	}
}

func TestGaItemFullCTA_ActiveWorkspaceIsIneligible(t *testing.T) {
	slot := fragmentedRepackSlot(t)
	pf := core.PreflightGaItemRepack(slot)

	_, cta := gaItemFullCTA(slot, fitCap(), true)
	if cta.Eligible {
		t.Fatalf("cta=%+v, want ineligible while a workspace is active", cta)
	}
	if cta.Recovered != pf.Analysis.Recovered {
		t.Fatalf("recovered=%d, want preserved %d", cta.Recovered, pf.Analysis.Recovered)
	}
}

func TestGaItemFullCTA_LeavesSlotUnchanged(t *testing.T) {
	slot := fragmentedRepackSlot(t)
	before := core.CloneSlot(slot)
	gaItemFullCTA(slot, fitCap(), false)
	if !reflect.DeepEqual(slot, before) {
		t.Fatal("gaItemFullCTA mutated the input slot")
	}
}

// endpointGaItemWeaponID is a somber armament whose add path allocates a GaItem
// record, so a cursor-exhausted slot rejects it with gaitem_full.
const endpointGaItemWeaponID = uint32(0x02810590) // Golem Greatbow

func TestAddItems_GaItemFullPopulatesCTA(t *testing.T) {
	app := NewApp()
	app.save = &core.SaveFile{}
	app.save.Slots[0] = *fragmentedRepackSlot(t) // cursor exhausted → weapon add fails
	before := core.CloneSlot(&app.save.Slots[0])

	res, err := app.AddItemsToCharacter(0, []uint32{endpointGaItemWeaponID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.CapHit != "gaitem_full" || res.Added != 0 {
		t.Fatalf("result=%+v, want gaitem_full rejection with nothing added", res)
	}
	if res.GaItemCapacity == nil || res.GaItemRepackCTA == nil {
		t.Fatalf("result=%+v, want both CTA fields populated", res)
	}
	if !res.GaItemRepackCTA.Eligible || res.GaItemRepackCTA.Recovered <= 0 {
		t.Fatalf("cta=%+v, want eligible with positive recovery", *res.GaItemRepackCTA)
	}
	if !reflect.DeepEqual(&app.save.Slots[0], before) {
		t.Fatal("rejected gaitem_full add mutated the slot")
	}
}

func TestAddItems_NonGaItemFullOmitsCTA(t *testing.T) {
	app := appTalismanCapacityFixture(t)
	slot := &app.save.Slots[0]
	// Occupy every common inventory slot (distinct indices to avoid the duplicate
	// -index integrity gate) so a talisman add fails with inventory_full — never
	// gaitem_full — and both CTA fields must stay absent.
	for i := range slot.Inventory.CommonItems {
		slot.Inventory.CommonItems[i] = core.InventoryItem{GaItemHandle: 0x90000000 | uint32(i), Index: uint32(i + 1)}
	}

	res, err := app.AddItemsToCharacter(0, []uint32{endpointTalismanID}, 0, 0, 0, 0, 1, 0)
	if err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}
	if res.CapHit != "inventory_full" {
		t.Fatalf("CapHit=%q, want inventory_full", res.CapHit)
	}
	if res.GaItemCapacity != nil || res.GaItemRepackCTA != nil {
		t.Fatalf("result=%+v, want no CTA fields for a non-gaitem_full failure", res)
	}
}
