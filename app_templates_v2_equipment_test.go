package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// makeEquipmentScanSlot builds a synthetic SaveSlot with empty
// ChrAsmEquipment (all 0xFFFFFFFF) and a controllable inventory list.
func makeEquipmentScanSlot() *core.SaveSlot {
	data := make([]byte, core.SlotSize)
	equipOff := 0x10000
	for i := 0; i < core.ChrAsmFieldCount; i++ {
		binary.LittleEndian.PutUint32(data[equipOff+i*4:], 0xFFFFFFFF)
	}
	return &core.SaveSlot{
		Data:               data,
		EquipItemsIDOffset: equipOff,
		GaMap:              map[uint32]uint32{},
	}
}

// writeEquipSlot writes a raw u32 at the given ChrAsmEquipment index.
func writeEquipSlot(slot *core.SaveSlot, idx int, value uint32) {
	off := slot.EquipItemsIDOffset + idx*4
	binary.LittleEndian.PutUint32(slot.Data[off:], value)
}

func TestBuildEquipmentSectionFromSlot_EmptyReturnsNil(t *testing.T) {
	slot := makeEquipmentScanSlot()
	got := buildEquipmentSectionFromSlot(slot, nil)
	if got != nil {
		t.Errorf("empty equipment should return nil section, got %+v", got)
	}
}

func TestBuildEquipmentSectionFromSlot_WeaponMatchesEditableItem(t *testing.T) {
	slot := makeEquipmentScanSlot()
	// Equip a weapon in RH1: encoded form = itemID | 0x80000000.
	writeEquipSlot(slot, 1, 0x80100020)

	items := []editor.EditableItem{{
		BaseItemID:     0x100000,
		ItemID:         0x00100020,
		Name:           "Uchigatana",
		IsWeapon:       true,
		CurrentUpgrade: 25,
		InfusionName:   "Cold",
	}}
	sec := buildEquipmentSectionFromSlot(slot, items)
	if sec == nil || sec.WeaponRightHand1 == nil {
		t.Fatalf("expected WeaponRightHand1 populated, got %+v", sec)
	}
	if sec.WeaponRightHand1.BaseItemID != 0x100000 {
		t.Errorf("baseItemID mismatch: %v", sec.WeaponRightHand1)
	}
	if sec.WeaponRightHand1.Upgrade == nil || *sec.WeaponRightHand1.Upgrade != 25 {
		t.Errorf("upgrade not populated for weapon")
	}
	if sec.WeaponRightHand1.InfusionName != "Cold" {
		t.Errorf("infusion lost: %q", sec.WeaponRightHand1.InfusionName)
	}
}

func TestBuildEquipmentSectionFromSlot_AmmoMatchesGoodsItem(t *testing.T) {
	slot := makeEquipmentScanSlot()
	// Equip Arrows1 (idx 6): goods item ID 0x40100050 (already 0x40-prefixed).
	writeEquipSlot(slot, 6, 0x40100050)
	items := []editor.EditableItem{{
		BaseItemID: 0x40100050,
		ItemID:     0x40100050,
		Name:       "Standard Arrow",
	}}
	sec := buildEquipmentSectionFromSlot(slot, items)
	if sec == nil || sec.Arrows1 == nil {
		t.Fatalf("expected Arrows1 populated, got %+v", sec)
	}
	if sec.Arrows1.BaseItemID != 0x40100050 {
		t.Errorf("arrows baseItemID mismatch: %v", sec.Arrows1)
	}
	if sec.Arrows1.Upgrade != nil {
		t.Errorf("ammo should not carry an upgrade pointer")
	}
}

func TestBuildEquipmentSectionFromSlot_ArmorMatchesEditableItem(t *testing.T) {
	slot := makeEquipmentScanSlot()
	// Armor head idx 12: encoded = itemID | 0x80000000.
	writeEquipSlot(slot, 12, 0x90100040)
	items := []editor.EditableItem{{
		BaseItemID: 0x10100040,
		ItemID:     0x10100040,
		Name:       "Knight Helm",
		IsArmor:    true,
	}}
	sec := buildEquipmentSectionFromSlot(slot, items)
	if sec == nil || sec.ArmorHead == nil {
		t.Fatalf("expected ArmorHead populated, got %+v", sec)
	}
	if sec.ArmorHead.BaseItemID != 0x10100040 {
		t.Errorf("armor baseItemID mismatch")
	}
}

func TestBuildEquipmentSectionFromSlot_TalismansAndGreatRuneNotExported(t *testing.T) {
	slot := makeEquipmentScanSlot()
	writeEquipSlot(slot, 10, 0x80000001) // EquippedGreatRune
	writeEquipSlot(slot, 17, 0x80000002) // Talisman1
	writeEquipSlot(slot, 18, 0x80000003) // Talisman2
	writeEquipSlot(slot, 11, 0x80000004) // unk0x2C
	writeEquipSlot(slot, 16, 0x80000005) // unk0x40

	sec := buildEquipmentSectionFromSlot(slot, nil)
	if sec != nil {
		t.Errorf("section should be nil — none of the Phase 7b.1 slots populated, got %+v", sec)
	}
}

func TestBuildEquipmentSectionFromSlot_UnreadableSlotReturnsNil(t *testing.T) {
	slot := &core.SaveSlot{
		Data:               make([]byte, core.SlotSize),
		EquipItemsIDOffset: 0, // not parsed
	}
	if buildEquipmentSectionFromSlot(slot, nil) != nil {
		t.Error("expected nil section when EquipItemsIDOffset is unparsed")
	}
}

func TestBuildEquipmentSectionFromSlot_UnknownItemEmitsRawBaseID(t *testing.T) {
	slot := makeEquipmentScanSlot()
	// Equip RH1 with an item ID that won't resolve to anything in the
	// editable inventory or the DB; the scanner should still emit a ref
	// rather than silently drop the slot.
	writeEquipSlot(slot, 1, 0xDEADBEEF)
	sec := buildEquipmentSectionFromSlot(slot, nil)
	if sec == nil || sec.WeaponRightHand1 == nil {
		t.Fatalf("unknown equipped item should still emit a ref, got %+v", sec)
	}
	if sec.WeaponRightHand1.BaseItemID == 0 {
		t.Errorf("unknown item ref should carry the decoded itemID as baseItemID")
	}
}

func TestBuildEquipmentSectionFromSlot_MultiSlotPopulated(t *testing.T) {
	slot := makeEquipmentScanSlot()
	writeEquipSlot(slot, 1, 0x80100020)  // RH1 weapon
	writeEquipSlot(slot, 6, 0x40100050)  // Arrows1
	writeEquipSlot(slot, 12, 0x90100040) // Head armor
	items := []editor.EditableItem{
		{BaseItemID: 0x100000, ItemID: 0x00100020, Name: "Uchi", IsWeapon: true, CurrentUpgrade: 0},
		{BaseItemID: 0x40100050, ItemID: 0x40100050, Name: "Arrow"},
		{BaseItemID: 0x10100040, ItemID: 0x10100040, Name: "Helm", IsArmor: true},
	}
	sec := buildEquipmentSectionFromSlot(slot, items)
	if sec == nil {
		t.Fatal("section nil")
	}
	if sec.WeaponRightHand1 == nil || sec.Arrows1 == nil || sec.ArmorHead == nil {
		t.Errorf("expected three slots populated, got %+v", sec)
	}
	if sec.ArmorChest != nil {
		t.Errorf("ArmorChest should remain nil for empty slot")
	}
}
