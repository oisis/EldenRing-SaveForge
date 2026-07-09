package core

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// TestBinaryAddAndReload verifies that adding a smithing stone to inventory
// produces correct binary output that survives a reload (re-parse).
func TestBinaryAddAndReload(t *testing.T) {
	const saveFile = "../../tmp/save/ER0000.sl2"
	const stoneID = uint32(0x40002776) // Smithing Stone [3]
	const stoneHandle = uint32(0xB0002776)

	save, err := LoadSave(saveFile)
	if err != nil {
		t.Skipf("cannot load %s: %v", saveFile, err)
	}

	// Find first slot that can accept a new smithing stone [3]
	targetSlot := -1
	for i := range save.Slots {
		if !save.ActiveSlots[i] {
			continue
		}
		slot := &save.Slots[i]
		found := false
		for _, item := range slot.Inventory.CommonItems {
			if item.GaItemHandle == stoneHandle {
				found = true
				break
			}
		}
		if !found {
			targetSlot = i
			break
		}
	}
	if targetSlot < 0 {
		t.Skip("no slot found without smithing stone [3]")
	}

	slot := &save.Slots[targetSlot]
	t.Logf("Using slot %d", targetSlot)

	// Snapshot counters BEFORE add
	invStart := slot.MagicOffset + InvStartFromMagic
	countOffBefore := invStart - 4
	countBefore := binary.LittleEndian.Uint32(slot.Data[countOffBefore:])
	acqSortIdBefore := slot.Inventory.NextAcquisitionSortId
	nextEquipIdxBefore := slot.Inventory.NextEquipIndex
	nextEquipIdxOff := slot.Inventory.NextEquipIndexOff()

	t.Logf("BEFORE: common_item_count=%d NextAcqSortId=%d NextEquipIdx=%d (off=0x%X)",
		countBefore, acqSortIdBefore, nextEquipIdxBefore, nextEquipIdxOff)

	// Deep-copy slot.Data so original is unchanged (RO guarantee)
	dataCopy := make([]byte, len(slot.Data))
	copy(dataCopy, slot.Data)
	slotCopy := *slot
	slotCopy.Data = dataCopy
	// Re-link EquipInventoryData offsets to the copy
	slotCopy.Inventory = slot.Inventory.Clone()

	// ADD: smithing stone [3], qty=1 to inventory
	_ = db.GetItemDataFuzzy // ensure db imported
	if err := addToInventory(&slotCopy, stoneHandle, 1, false, false); err != nil {
		t.Fatalf("addToInventory: %v", err)
	}

	// Snapshot counters AFTER add (from binary)
	countAfterBin := binary.LittleEndian.Uint32(slotCopy.Data[countOffBefore:])
	acqSortIdAfterBin := binary.LittleEndian.Uint32(slotCopy.Data[slotCopy.Inventory.nextAcqSortIdOff:])
	nextEquipIdxAfterBin := uint32(0)
	if nextEquipIdxOff > 0 {
		nextEquipIdxAfterBin = binary.LittleEndian.Uint32(slotCopy.Data[nextEquipIdxOff:])
	}

	t.Logf("AFTER (binary): common_item_count=%d NextAcqSortId=%d NextEquipIdx=%d",
		countAfterBin, acqSortIdAfterBin, nextEquipIdxAfterBin)

	// Verify count incremented
	if countAfterBin != countBefore+1 {
		t.Errorf("common_item_count: got %d, want %d", countAfterBin, countBefore+1)
	}

	// Verify NextAcqSortId incremented
	if acqSortIdAfterBin != acqSortIdBefore+1 {
		t.Errorf("NextAcqSortId: got %d, want %d", acqSortIdAfterBin, acqSortIdBefore+1)
	}

	// Verify NextEquipIndex bumped when acqIdx >= NextEquipIndex (regression fix for issue #3)
	acqIdxUsed := acqSortIdBefore // the value written to the new record
	if acqIdxUsed >= nextEquipIdxBefore {
		wantNextEquip := acqIdxUsed + 1
		if nextEquipIdxAfterBin != wantNextEquip {
			t.Errorf("NextEquipIndex: got %d, want %d (acqIdx=%d was >= old NextEquipIdx=%d)",
				nextEquipIdxAfterBin, wantNextEquip, acqIdxUsed, nextEquipIdxBefore)
		}
	} else {
		// acqIdx < NextEquipIndex — should be untouched
		if nextEquipIdxAfterBin != nextEquipIdxBefore {
			t.Errorf("NextEquipIndex changed unexpectedly: %d → %d (acqIdx=%d was < NextEquipIdx=%d)",
				nextEquipIdxBefore, nextEquipIdxAfterBin, acqIdxUsed, nextEquipIdxBefore)
		}
	}

	// Find the new item in binary
	newItemFound := false
	newItemOff := 0
	for i := 0; i < CommonItemCount; i++ {
		off := invStart + i*InvRecordLen
		if off+InvRecordLen > len(slotCopy.Data) {
			break
		}
		h := binary.LittleEndian.Uint32(slotCopy.Data[off:])
		if h == stoneHandle {
			qty := binary.LittleEndian.Uint32(slotCopy.Data[off+4:])
			idx := binary.LittleEndian.Uint32(slotCopy.Data[off+8:])
			t.Logf("  Found stone[3] at slot %d (off=0x%X): handle=0x%08X qty=%d index=%d",
				i, off, h, qty, idx)
			newItemFound = true
			newItemOff = off
			// Verify index == acqSortIdBefore (the value used, not post-increment)
			if idx != acqSortIdBefore {
				t.Errorf("item Index: got %d, want %d (NextAcqSortId before add)", idx, acqSortIdBefore)
			}
			// Verify slot number <= countAfterBin-1 (game reads only countAfterBin entries)
			if uint32(i) >= countAfterBin {
				t.Errorf("item at slot %d >= count %d — game won't read it!", i, countAfterBin)
			}
			break
		}
	}
	if !newItemFound {
		t.Errorf("stone[3] NOT found in binary after add")
	}
	_ = newItemOff

	// RELOAD: re-parse the modified data to verify re-read finds the item
	reloaded := &SaveSlot{}
	reloaded.Data = slotCopy.Data
	if err := reloaded.parseFromData(); err != nil {
		t.Fatalf("parseFromData on modified slot: %v", err)
	}

	reloadedCount := uint32(0)
	reloadedFound := false
	for _, item := range reloaded.Inventory.CommonItems {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		reloadedCount++
		if item.GaItemHandle == stoneHandle {
			reloadedFound = true
			t.Logf("  After reload: stone[3] qty=%d index=%d", item.Quantity, item.Index)
		}
	}
	t.Logf("RELOAD: actual non-empty items=%d (header=%d)", reloadedCount,
		binary.LittleEndian.Uint32(slotCopy.Data[countOffBefore:]))

	if !reloadedFound {
		t.Errorf("stone[3] NOT found in reloaded inventory")
	}
	if reloadedCount != countAfterBin {
		t.Errorf("reload count %d != binary header %d", reloadedCount, countAfterBin)
	}

	// Compare NextEquipIndex across load → add → reload
	t.Logf("NextEquipIndex: before=%d after-add=%d after-reload=%d",
		nextEquipIdxBefore, nextEquipIdxAfterBin, reloaded.Inventory.NextEquipIndex)

	// Also check what the game-written item indices look like vs our new item
	var existingIndices []uint32
	for _, item := range reloaded.Inventory.CommonItems {
		if item.GaItemHandle != GaHandleEmpty && item.GaItemHandle != GaHandleInvalid {
			existingIndices = append(existingIndices, item.Index)
		}
	}
	maxExisting := uint32(0)
	minExisting := uint32(0xFFFFFFFF)
	for _, idx := range existingIndices {
		if idx > maxExisting {
			maxExisting = idx
		}
		if idx < minExisting {
			minExisting = idx
		}
	}
	t.Logf("Existing item indices: min=%d max=%d (NextEquipIdx=%d)",
		minExisting, maxExisting, reloaded.Inventory.NextEquipIndex)
	t.Logf("New item index: %d", acqSortIdBefore)
	t.Logf("Summary: new item index %s NextEquipIndex(%d)",
		func() string {
			if acqSortIdBefore >= reloaded.Inventory.NextEquipIndex {
				return ">="
			}
			return "<"
		}(), reloaded.Inventory.NextEquipIndex)

	// Check for duplicate indices
	seen := make(map[uint32]bool)
	for _, item := range reloaded.Inventory.CommonItems {
		if item.GaItemHandle == GaHandleEmpty || item.GaItemHandle == GaHandleInvalid {
			continue
		}
		if seen[item.Index] {
			t.Errorf("duplicate index %d in reloaded inventory", item.Index)
		}
		seen[item.Index] = true
	}

	// Also dump a few bytes around the new item for manual inspection
	if newItemFound {
		off := newItemOff
		if off >= 8 && off+InvRecordLen+8 <= len(slotCopy.Data) {
			t.Logf("Binary around new item (off-8 to off+20):\n  %s",
				hexDump(slotCopy.Data[off-8:off+InvRecordLen+8]))
		}
	}
}

func hexDump(b []byte) string {
	var buf bytes.Buffer
	for i, v := range b {
		fmt.Fprintf(&buf, "%02X ", v)
		if (i+1)%4 == 0 {
			buf.WriteString("| ")
		}
	}
	return buf.String()
}
