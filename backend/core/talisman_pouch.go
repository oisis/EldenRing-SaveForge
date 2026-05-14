package core

import (
	"encoding/binary"
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

// Talisman Pouch sync — the bonus key item that maps 1:1 to additional
// talisman slots in-game. `slot.Player.TalismanSlots` (u8, 0..3) is the
// number of ADDITIONAL pouches the character owns; the base talisman slot
// every character starts with is implicit, has no inventory representation,
// and is intentionally never touched by this helper.
const (
	// TalismanPouchItemID is the Good (item ID prefix 0x40) for "Talisman Pouch".
	TalismanPouchItemID uint32 = 0x40002738

	// TalismanPouchHandle is the GaItem handle for a stackable Talisman Pouch
	// in the held inventory (goods handle prefix 0xB0 + lower 28 bits of the ID).
	TalismanPouchHandle uint32 = 0xB0002738

	// "Obtained Talisman Pouch N" event flags. Vanilla Elden Ring caps pouches
	// at 3; flag IDs 60530+ are unused and deliberately not synced.
	TalismanPouchFlag0 uint32 = 60500
	TalismanPouchFlag1 uint32 = 60510
	TalismanPouchFlag2 uint32 = 60520

	talismanPouchMaxVanilla = 3
)

// SyncTalismanPouchCount reconciles the Talisman Pouch inventory entry and the
// three obtained-pouch event flags (60500/60510/60520) with the requested
// additional-pouch count. `count` is the number of ADDITIONAL pouches (0..3);
// the base talisman slot is not modelled here.
//
// Semantics (idempotent across repeated applies):
//   - count <= 0: removes the Talisman Pouch entry (handle 0xB0002738) from
//     inventory if present; clears flags 60500/60510/60520. The base slot is
//     untouched.
//   - count == 1: leaves exactly one inventory entry with Quantity=1; sets
//     60500 only.
//   - count == 2: leaves exactly one inventory entry with Quantity=2; sets
//     60500+60510.
//   - count == 3: leaves exactly one inventory entry with Quantity=3; sets
//     60500+60510+60520.
//   - count >  3: clamped to 3 (vanilla limit; 60530+ flags remain untouched).
//
// The helper targets ONLY the Talisman Pouch handle: no other CommonItems,
// KeyItems, GaMap entries, or unrelated event flags are scanned or mutated.
func SyncTalismanPouchCount(slot *SaveSlot, count int) error {
	if slot == nil {
		return fmt.Errorf("SyncTalismanPouchCount: nil slot")
	}
	if count < 0 {
		count = 0
	}
	if count > talismanPouchMaxVanilla {
		count = talismanPouchMaxVanilla
	}

	if err := syncTalismanPouchInventory(slot, count); err != nil {
		return fmt.Errorf("SyncTalismanPouchCount: inventory: %w", err)
	}
	if err := syncTalismanPouchFlags(slot, count); err != nil {
		return fmt.Errorf("SyncTalismanPouchCount: flags: %w", err)
	}
	return nil
}

// syncTalismanPouchInventory adjusts the single CommonItems entry for
// handle == TalismanPouchHandle. Lookup is by-handle only — no other entries
// are ever inspected for "similar" item IDs, and KeyItems / Storage are not
// touched.
func syncTalismanPouchInventory(slot *SaveSlot, count int) error {
	existingIdx := -1
	for i, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == TalismanPouchHandle {
			existingIdx = i
			break
		}
	}

	if count == 0 {
		if existingIdx < 0 {
			return nil
		}
		// RemoveItemFromSlot scans only for records whose GaItemHandle equals
		// the supplied handle (writer.go:582). With handle==TalismanPouchHandle
		// and fromInventory=true / fromStorage=false the call is strictly
		// scoped to the Talisman Pouch entry; no other Goods or KeyItems are
		// affected.
		return RemoveItemFromSlot(slot, TalismanPouchHandle, true, false)
	}

	if existingIdx >= 0 {
		sa := NewSlotAccessor(slot.Data)
		invStart := slot.MagicOffset + InvStartFromMagic
		off := invStart + existingIdx*InvRecordLen + 4
		if err := sa.CheckBounds(off, 4, "SyncTalismanPouchCount/qty"); err != nil {
			return err
		}
		binary.LittleEndian.PutUint32(slot.Data[off:], uint32(count))
		slot.Inventory.CommonItems[existingIdx].Quantity = uint32(count)
		return nil
	}

	// No existing entry — add one via the stackable goods path. handlePrefix
	// derived from TalismanPouchItemID resolves to ItemTypeItem (0xB0); the
	// shared-handle stackable branch in AddItemsToSlot registers
	// GaMap[0xB0002738]=0x40002738 (if absent) and inserts a single
	// CommonItems record with Quantity=count.
	return AddItemsToSlot(slot, []uint32{TalismanPouchItemID}, count, 0, false)
}

// syncTalismanPouchFlags reconciles flags 60500/60510/60520 with count.
// Higher flag IDs (60530+) are not vanilla and are intentionally not touched.
func syncTalismanPouchFlags(slot *SaveSlot, count int) error {
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("invalid EventFlagsOffset (%d)", slot.EventFlagsOffset)
	}
	flags := slot.Data[slot.EventFlagsOffset:]
	pouchFlags := [3]uint32{TalismanPouchFlag0, TalismanPouchFlag1, TalismanPouchFlag2}
	for i, flag := range pouchFlags {
		if err := db.SetEventFlag(flags, flag, i < count); err != nil {
			return fmt.Errorf("SetEventFlag(%d): %w", flag, err)
		}
	}
	return nil
}
