package tests

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// TestBSTLookupMatchesEventFlags verifies that BST-computed positions match
// the precomputed entries in the EventFlags lookup table for known map flags.
func TestBSTLookupMatchesEventFlags(t *testing.T) {
	data.LoadBST()

	// All map-related flags that exist in both EventFlags and BST
	knownFlags := []uint32{
		62000, 62001, 62004, 62005, 62006, 62007, 62008, 62009,
		62010, 62011, 62012, 62020, 62021, 62022, 62030, 62031, 62032,
		62040, 62041, 62050, 62051, 62052, 62060, 62061, 62062, 62063, 62064,
		62080, 62081, 62082, 62083, 62084, 62102, 62103,
		63010, 63011, 63012, 63020, 63021, 63022, 63030, 63031, 63032,
		63040, 63041, 63050, 63051, 63052, 63060, 63061, 63062, 63063, 63064,
		63080, 63081, 63082, 63083, 63084,
		82001, 82002,
	}

	for _, flagID := range knownFlags {
		info, ok := data.EventFlags[flagID]
		if !ok {
			t.Errorf("flag %d missing from EventFlags lookup table", flagID)
			continue
		}

		block := flagID / data.BSTFlagDivisor
		bstPos, ok := data.EventFlagBST[block]
		if !ok {
			t.Errorf("block %d (flag %d) missing from BST", block, flagID)
			continue
		}

		idx := flagID % data.BSTFlagDivisor
		bstByte := bstPos*data.BSTBlockSize + idx/8
		bstBit := uint8(7 - (idx % 8))

		if bstByte != info.Byte || bstBit != info.Bit {
			t.Errorf("flag %d: BST gives byte=0x%X bit=%d, EventFlags gives byte=0x%X bit=%d",
				flagID, bstByte, bstBit, info.Byte, info.Bit)
		}
	}
}

// TestGetAllMapEntries verifies the map entry list is populated correctly.
func TestGetAllMapEntries(t *testing.T) {
	entries := db.GetAllMapEntries()
	if len(entries) == 0 {
		t.Fatal("GetAllMapEntries returned empty list")
	}

	visible := 0
	acquired := 0
	system := 0
	unsafe := 0
	for _, e := range entries {
		switch e.Category {
		case "visible":
			visible++
		case "acquired":
			acquired++
		case "system":
			system++
		case "unsafe":
			unsafe++
		default:
			t.Errorf("unexpected category %q for flag %d", e.Category, e.ID)
		}
	}

	t.Logf("Map entries: %d visible, %d acquired, %d system, %d unsafe = %d total",
		visible, acquired, system, unsafe, len(entries))

	if visible < 26 {
		t.Errorf("expected at least 26 visible entries, got %d", visible)
	}
	if acquired < 20 {
		t.Errorf("expected at least 20 acquired entries, got %d", acquired)
	}
	if system != 4 {
		t.Errorf("expected 4 system entries, got %d", system)
	}
	if unsafe != 8 {
		t.Errorf("expected 8 unsafe entries, got %d", unsafe)
	}
}

// TestMapFlagsRoundtrip tests setting and reading map flags on a real save file.
func TestMapFlagsRoundtrip(t *testing.T) {
	save := loadTestSave(t, pcSavePath)

	// Use slot 1 (Zofia - completed game)
	slot := &save.Slots[1]
	if slot.Version == 0 {
		t.Skip("slot 1 is empty")
	}
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		t.Skip("event flags offset not available")
	}

	flags := slot.Data[slot.EventFlagsOffset:]

	// Test reading a known map visible flag (62010 = Limgrave West)
	val, err := db.GetEventFlag(flags, 62010)
	if err != nil {
		t.Fatalf("GetEventFlag(62010): %v", err)
	}
	t.Logf("Slot 1 flag 62010 (Limgrave West visible): %v", val)

	// Test set/get roundtrip on a less critical flag
	testFlag := uint32(62065) // unused map flag
	origVal, err := db.GetEventFlag(flags, testFlag)
	if err != nil {
		t.Fatalf("GetEventFlag(%d): %v", testFlag, err)
	}

	// Toggle
	if err := db.SetEventFlag(flags, testFlag, !origVal); err != nil {
		t.Fatalf("SetEventFlag(%d, %v): %v", testFlag, !origVal, err)
	}
	newVal, err := db.GetEventFlag(flags, testFlag)
	if err != nil {
		t.Fatalf("GetEventFlag(%d) after set: %v", testFlag, err)
	}
	if newVal != !origVal {
		t.Errorf("flag %d: expected %v after toggle, got %v", testFlag, !origVal, newVal)
	}

	// Restore
	if err := db.SetEventFlag(flags, testFlag, origVal); err != nil {
		t.Fatalf("SetEventFlag(%d, %v) restore: %v", testFlag, origVal, err)
	}
}

// TestEventFlagsOffsetCorrectness verifies that EventFlagsOffset points to
// the real event flags section by checking that known graces in a completed
// save (slot 1 / Zofia) read as visited=true.
func TestEventFlagsOffsetCorrectness(t *testing.T) {
	save := loadTestSave(t, pcSavePath)

	slot := &save.Slots[1]
	if slot.Version == 0 {
		t.Skip("slot 1 is empty")
	}
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		t.Skip("event flags offset not available")
	}

	t.Logf("EventFlagsOffset = 0x%X (flags len = %d)", slot.EventFlagsOffset, len(slot.Data)-slot.EventFlagsOffset)

	flags := slot.Data[slot.EventFlagsOffset:]

	// Graces that must be visited in any completed game save:
	// Table of Lost Grace (starting grace), The First Step, Church of Elleh
	knownVisitedGraces := []struct {
		id   uint32
		name string
	}{
		{0x00011616, "Table of Lost Grace / Roundtable Hold"},
		{0x00012945, "The First Step"},
		{0x00012944, "Church of Elleh"},
	}

	visitedCount := 0
	for _, g := range knownVisitedGraces {
		val, err := db.GetEventFlag(flags, g.id)
		if err != nil {
			t.Errorf("GetEventFlag(%s / 0x%X): %v", g.name, g.id, err)
			continue
		}
		t.Logf("Grace %s (0x%X): visited=%v", g.name, g.id, val)
		if val {
			visitedCount++
		}
	}

	if visitedCount == 0 {
		t.Errorf("none of %d known graces read as visited — EventFlagsOffset is likely wrong", len(knownVisitedGraces))
	} else {
		t.Logf("%d/%d known graces read as visited — offset looks correct", visitedCount, len(knownVisitedGraces))
	}
}
