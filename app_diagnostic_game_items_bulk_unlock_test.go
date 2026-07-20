package main

import (
	"encoding/binary"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

func TestGameItemsBulkCookbookLifecycle(t *testing.T) {
	app := gameItemAddApp(false)
	withUnlockEventFlags(app, itemEventFlagsRegionSize)
	flags := []uint32{67000, 67110}

	journal := freshRemoveJournal(app, true)
	if err := app.BulkSetCookbooksUnlocked(0, flags, true); err != nil {
		t.Fatalf("BulkSetCookbooksUnlocked: %v", err)
	}
	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsBulkSetCookbooks, "0")
	assertUnlockSuccess(t, lc)
	for _, flagID := range flags {
		assertUnlockLifecycle(t, lc, "event_flag_"+giDec(flagID), "false", "true", "true")
		itemID := data.CookbookFlagToItemID[flagID]
		prefix := findInvField(t, &app.save.Slots[0], itemID)
		if prefix[:len("inventory_key")] != "inventory_key" {
			t.Fatalf("cookbook 0x%08X field prefix = %q, want inventory_key", itemID, prefix)
		}
		assertUnlockLifecycle(t, lc, prefix+"_item_id", giAbsent, giHex(itemID), giHex(itemID))
	}
	slot := &app.save.Slots[0]
	if row, _ := firstPopulatedInvRow(slot); row >= 0 {
		t.Fatalf("bulk cookbook unlock added CommonItems row %d, want KeyItems only", row)
	}
	keyCountOff := slot.MagicOffset + core.InvStartFromMagic + core.CommonItemCount*core.InvRecordLen
	if got := binary.LittleEndian.Uint32(slot.Data[keyCountOff:]); got != uint32(len(flags)) {
		t.Errorf("key_count = %d, want %d for T071's two Merchant Kale cookbooks", got, len(flags))
	}
}

func TestGameItemsBulkBellBearingLifecycle(t *testing.T) {
	app := gameItemAddApp(false)
	withUnlockEventFlags(app, 0x160000)
	flags := []uint32{11109710, 11109711}
	items := []uint32{data.BellBearingFlagToItemID[flags[0]], data.BellBearingFlagToItemID[flags[1]]}
	if _, err := app.AddItemsToCharacter(0, items, 0, 0, 0, 0, 1, 0); err != nil {
		t.Fatalf("seed AddItemsToCharacter: %v", err)
	}

	journal := freshRemoveJournal(app, true)
	if err := app.BulkSetBellBearings(0, flags, true); err != nil {
		t.Fatalf("BulkSetBellBearings: %v", err)
	}
	lc := collectUnlockLifecycle(t, journal.Tail(), actionGameItemsBulkSetBellBearings, "0")
	assertUnlockSuccess(t, lc)
	for _, flagID := range flags {
		assertUnlockLifecycle(t, lc, "event_flag_"+giDec(flagID), "false", "true", "true")
	}
	assertUnlockLifecycle(t, lc, "inventory_common_header_count", "2", "0", "0")
}
