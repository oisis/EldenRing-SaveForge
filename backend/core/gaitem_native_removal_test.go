package core

import "testing"

func TestReprojectNativeGaItemsAfterAoWRemoval(t *testing.T) {
	slot := makeTestSlot(300)
	kick := uint32(ItemTypeAow | gaItemHandleValidBit | 2)
	noSkill := uint32(ItemTypeAow | gaItemHandleValidBit | 245)
	impalingThrust := uint32(ItemTypeAow | gaItemHandleValidBit | 260)
	weapon := uint32(ItemTypeWeapon | gaItemHandleValidBit | 4)

	slot.GaItems[0] = nativeTestRecord(kick, 0x8000C47C)
	slot.GaItems[1] = nativeTestRecord(noSkill, 0x800078B4)
	slot.GaItems[2] = nativeTestRecord(impalingThrust, 0x80002774)
	slot.GaItems[6] = nativeTestRecord(weapon, 0x002E3BF0)

	if _, err := analyzeNativeGaItemLayout(slot); err != nil {
		t.Fatalf("precondition: %v", err)
	}
	slot.GaItems[0] = GaItemFull{}

	if err := reprojectNativeGaItems(slot); err != nil {
		t.Fatalf("reprojectNativeGaItems: %v", err)
	}
	if got := slot.GaItems[0].Handle; got != noSkill {
		t.Errorf("GaItems[0] handle = 0x%08X, want No Skill 0x%08X", got, noSkill)
	}
	if got := slot.GaItems[1].Handle; got != impalingThrust {
		t.Errorf("GaItems[1] handle = 0x%08X, want Impaling Thrust 0x%08X", got, impalingThrust)
	}
	if got := slot.GaItems[6].Handle; got != weapon {
		t.Errorf("weapon handle changed or moved: GaItems[6] = 0x%08X, want 0x%08X", got, weapon)
	}
	if !slot.GaItems[4].IsEmpty() {
		t.Errorf("removed physical index marker GaItems[4] is not empty: %+v", slot.GaItems[4])
	}
}
