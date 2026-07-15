package core

import (
	"bytes"
	"encoding/binary"
	"os"
	"testing"
)

type gaItemDataRecord struct {
	id   uint32
	flag uint32
}

func weaponGaItemDataFixture(records []gaItemDataRecord) *SaveSlot {
	const off = 64
	data := make([]byte, off+GaitemGameDataSize+64)
	slot := &SaveSlot{Data: data, GaItemDataOffset: off}
	binary.LittleEndian.PutUint32(data[off:], uint32(len(records)))
	for i, record := range records {
		entryOff := off + GaItemDataArrayOff + i*GaItemDataActiveEntryLen
		binary.LittleEndian.PutUint32(data[entryOff:], record.id)
		binary.LittleEndian.PutUint32(data[entryOff+4:], record.flag)
	}
	return slot
}

func activeGaItemDataRecords(t *testing.T, slot *SaveSlot) []gaItemDataRecord {
	t.Helper()
	off := slot.GaItemDataOffset
	count := int(binary.LittleEndian.Uint32(slot.Data[off:]))
	records := make([]gaItemDataRecord, count)
	for i := range records {
		entryOff := off + GaItemDataArrayOff + i*GaItemDataActiveEntryLen
		records[i] = gaItemDataRecord{
			id:   binary.LittleEndian.Uint32(slot.Data[entryOff:]),
			flag: binary.LittleEndian.Uint32(slot.Data[entryOff+4:]),
		}
	}
	return records
}

func TestUpsertWeaponGaItemData_InsertsEightByteRecordInGameOrderedSegment(t *testing.T) {
	// This is the relevant shape observed in the game-controlled save pair:
	// an older prefix, a final ascending ordinary-ID segment, then unordered AoWs.
	before := []gaItemDataRecord{
		{0x40000064, 1}, {0x401EA7A8, 1},
		{0x0000BD46, 0}, {0x0000BD54, 0},
		{0x000F4240, 1}, {0x0010C8E0, 1}, {0x001E8480, 1}, {0x001ED2A0, 1}, {0x002E6300, 1},
		{0x8000EB28, 1}, {0x8000FEB0, 1},
		{0x0104511D, 1}, // trailing data after the AoW group must not be reordered
	}
	slot := weaponGaItemDataFixture(before)
	oldEnd := slot.GaItemDataOffset + GaItemDataArrayOff + len(before)*GaItemDataActiveEntryLen
	for i := 0; i < 56; i++ {
		slot.Data[oldEnd+i] = byte(i + 1)
	}
	// The first eight tail bytes are consumed by the expanded active list. The
	// remaining reserved bytes must remain at their physical offsets.
	tailBefore := append([]byte(nil), slot.Data[oldEnd+GaItemDataActiveEntryLen:oldEnd+56]...)

	const lordswornsGreatsword = 0x002E3BF0
	if err := upsertWeaponGaItemData(slot, lordswornsGreatsword); err != nil {
		t.Fatalf("upsertWeaponGaItemData: %v", err)
	}

	want := []gaItemDataRecord{
		{0x40000064, 1}, {0x401EA7A8, 1},
		{0x0000BD46, 0}, {0x0000BD54, 0},
		{0x000F4240, 1}, {0x0010C8E0, 1}, {0x001E8480, 1}, {0x001ED2A0, 1},
		{lordswornsGreatsword, 1}, {0x002E6300, 1},
		{0x8000EB28, 1}, {0x8000FEB0, 1}, {0x0104511D, 1},
	}
	if got := activeGaItemDataRecords(t, slot); !bytes.Equal(encodeGaItemDataRecords(got), encodeGaItemDataRecords(want)) {
		t.Fatalf("active records = %#v, want %#v", got, want)
	}
	reservedTailStart := oldEnd + GaItemDataActiveEntryLen
	if got := slot.Data[reservedTailStart : reservedTailStart+len(tailBefore)]; !bytes.Equal(got, tailBefore) {
		t.Fatalf("reserved tail changed: got %x, want %x", got, tailBefore)
	}
}

func TestUpsertWeaponGaItemData_GameControlledSaveFixture(t *testing.T) {
	const path = "../../tmp/save-gaitems/ER0000-kro55-2-back.sl2"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skipf("game-controlled fixture not available: %s", path)
	}
	save, err := LoadSave(path)
	if err != nil {
		t.Fatalf("LoadSave(%q): %v", path, err)
	}
	slot := &save.Slots[0]
	const lordswornsGreatsword = 0x002E3BF0
	off := slot.GaItemDataOffset
	if count := binary.LittleEndian.Uint32(slot.Data[off:]); count != 240 {
		t.Fatalf("fixture count=%d, want 240", count)
	}
	if err := upsertWeaponGaItemData(slot, lordswornsGreatsword); err != nil {
		t.Fatalf("upsertWeaponGaItemData: %v", err)
	}
	if count := binary.LittleEndian.Uint32(slot.Data[off:]); count != 241 {
		t.Fatalf("count=%d, want 241", count)
	}
	entryOff := off + GaItemDataArrayOff + 64*GaItemDataActiveEntryLen
	if id := binary.LittleEndian.Uint32(slot.Data[entryOff:]); id != lordswornsGreatsword {
		t.Fatalf("record[64].id=%#08x, want %#08x", id, lordswornsGreatsword)
	}
	if flag := binary.LittleEndian.Uint32(slot.Data[entryOff+4:]); flag != 1 {
		t.Fatalf("record[64].flag=%d, want 1", flag)
	}
}

func TestUpsertAoWGaItemData_InsertsBeforeGreaterAoW(t *testing.T) {
	const (
		sacredBlade   = 0x80004E84
		determination = 0x8000EA60
	)
	slot := weaponGaItemDataFixture([]gaItemDataRecord{
		{0x400023B6, 1}, {determination, 1},
	})
	if err := upsertAoWGaItemData(slot, sacredBlade); err != nil {
		t.Fatalf("upsertAoWGaItemData: %v", err)
	}
	want := []gaItemDataRecord{{0x400023B6, 1}, {sacredBlade, 1}, {determination, 1}}
	if got := activeGaItemDataRecords(t, slot); !bytes.Equal(encodeGaItemDataRecords(got), encodeGaItemDataRecords(want)) {
		t.Fatalf("active records = %#v, want %#v", got, want)
	}
}

func TestUpsertAoWGaItemData_PreservesUnsortedLegacyGroup(t *testing.T) {
	const (
		sacredBlade   = 0x80004E84
		determination = 0x8000EA60
		groundSlam    = 0x8000C5A8
	)
	slot := weaponGaItemDataFixture([]gaItemDataRecord{
		{0x400023B6, 1}, {determination, 1}, {sacredBlade, 1},
	})
	if err := upsertAoWGaItemData(slot, groundSlam); err != nil {
		t.Fatalf("upsertAoWGaItemData: %v", err)
	}
	want := []gaItemDataRecord{{0x400023B6, 1}, {determination, 1}, {sacredBlade, 1}, {groundSlam, 1}}
	if got := activeGaItemDataRecords(t, slot); !bytes.Equal(encodeGaItemDataRecords(got), encodeGaItemDataRecords(want)) {
		t.Fatalf("active records = %#v, want %#v", got, want)
	}
}

func TestUpsertAoWGaItemData_GameControlledSaveFixture(t *testing.T) {
	const (
		beforePath  = "../../tmp/save-gaitems/ER0000-kro55-aow-3.sl2"
		afterPath   = "../../tmp/save-gaitems/ER0000-kro55-aow-4.sl2"
		sacredBlade = 0x80004E84
	)
	for _, path := range []string{beforePath, afterPath} {
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Skipf("game-controlled fixture not available: %s", path)
		}
	}
	before, err := LoadSave(beforePath)
	if err != nil {
		t.Fatalf("LoadSave(%q): %v", beforePath, err)
	}
	after, err := LoadSave(afterPath)
	if err != nil {
		t.Fatalf("LoadSave(%q): %v", afterPath, err)
	}
	actual, expected := &before.Slots[0], &after.Slots[0]
	if err := upsertAoWGaItemData(actual, sacredBlade); err != nil {
		t.Fatalf("upsertAoWGaItemData: %v", err)
	}
	gotData := actual.Data[actual.GaItemDataOffset : actual.GaItemDataOffset+GaitemGameDataSize]
	wantData := expected.Data[expected.GaItemDataOffset : expected.GaItemDataOffset+GaitemGameDataSize]
	if !bytes.Equal(gotData, wantData) {
		t.Fatal("GaItemData differs from the game-controlled Sacred Blade output")
	}
}

func TestUpsertWeaponGaItemData_ExistingIDIsNoOp(t *testing.T) {
	slot := weaponGaItemDataFixture([]gaItemDataRecord{{0x000F4240, 1}, {0x001E8480, 1}})
	before := append([]byte(nil), slot.Data...)
	if err := upsertWeaponGaItemData(slot, 0x001E8480); err != nil {
		t.Fatalf("upsertWeaponGaItemData: %v", err)
	}
	if !bytes.Equal(slot.Data, before) {
		t.Fatal("existing item changed GaItemData")
	}
}

func TestUpsertWeaponGaItemData_RejectsFullActiveList(t *testing.T) {
	slot := weaponGaItemDataFixture(nil)
	binary.LittleEndian.PutUint32(slot.Data[slot.GaItemDataOffset:], GaItemDataMaxCount)
	before := append([]byte(nil), slot.Data...)
	if err := upsertWeaponGaItemData(slot, 0x002E3BF0); err == nil {
		t.Fatal("upsertWeaponGaItemData succeeded for a full list")
	}
	if !bytes.Equal(slot.Data, before) {
		t.Fatal("full-list error mutated GaItemData")
	}
}

func TestCheckAddCapacity_RecognizesWeaponAtOddActiveRecord(t *testing.T) {
	const weaponID = 0x001E8480
	slot := capSlot(0, make([]GaItemFull, 10))
	slot.Data = make([]byte, 256)
	slot.GaItemDataOffset = 64
	binary.LittleEndian.PutUint32(slot.Data[slot.GaItemDataOffset:], 2)
	arrayBase := slot.GaItemDataOffset + GaItemDataArrayOff
	binary.LittleEndian.PutUint32(slot.Data[arrayBase:], 0x000F4240)
	binary.LittleEndian.PutUint32(slot.Data[arrayBase+4:], 1)
	binary.LittleEndian.PutUint32(slot.Data[arrayBase+GaItemDataActiveEntryLen:], weaponID)
	binary.LittleEndian.PutUint32(slot.Data[arrayBase+GaItemDataActiveEntryLen+4:], 1)

	if report := CheckAddCapacity(slot, []ItemToAdd{{ItemID: weaponID, InvQty: 1}}); report.NeededGaItemData != 0 {
		t.Fatalf("NeededGaItemData=%d, want 0 for an ID in active record 1", report.NeededGaItemData)
	}
}

func encodeGaItemDataRecords(records []gaItemDataRecord) []byte {
	data := make([]byte, len(records)*GaItemDataActiveEntryLen)
	for i, record := range records {
		binary.LittleEndian.PutUint32(data[i*GaItemDataActiveEntryLen:], record.id)
		binary.LittleEndian.PutUint32(data[i*GaItemDataActiveEntryLen+4:], record.flag)
	}
	return data
}
