package main

import (
	"os"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
)

// TestClampUpgrade_AppPathContract pins the contract the add path relies
// on. The pure-function tests for editor.ClampUpgrade live in
// backend/editor/weapon_test.go; this case set is kept here as a
// regression guard so the relocation doesn't silently change semantics
// on the caller side.
func TestClampUpgrade_AppPathContract(t *testing.T) {
	cases := []struct{ req, max, want int }{
		{25, 10, 10}, // somber weapon: over-cap clamped down
		{25, 25, 25}, // standard weapon: exact max allowed
		{5, 10, 5},   // in range untouched
		{0, 10, 0},   // zero untouched
		{-3, 10, 0},  // negative floored to 0
		{7, 0, 0},    // max 0 (non-upgradeable) clamps to 0
		{12, -1, 0},  // defensive: negative max treated as 0
	}
	for _, c := range cases {
		if got := editor.ClampUpgrade(c.req, c.max); got != c.want {
			t.Errorf("editor.ClampUpgrade(%d, %d) = %d, want %d", c.req, c.max, got, c.want)
		}
	}
}

// TestAddItemsToCharacter_ClampsSomberUpgrade is the regression for the Golem
// Greatbow corruption: requesting +25 on a somber (+10) weapon must store it at
// +10 (base+10), never base+25 — which the editor would flag as upgrade_out_of_range.
func TestAddItemsToCharacter_ClampsSomberUpgrade(t *testing.T) {
	const savePath = "tmp/save/ER0000.sl2"
	if _, err := os.Stat(savePath); os.IsNotExist(err) {
		t.Skipf("test save not found: %s", savePath)
	}
	save, err := core.LoadSave(savePath)
	if err != nil {
		t.Fatalf("LoadSave: %v", err)
	}
	app := NewApp()
	app.save = save

	// Pick an active slot with free capacity.
	slotIdx := -1
	for i := 0; i < 10; i++ {
		s := &save.Slots[i]
		if s.Version == 0 {
			continue
		}
		freeGa, freeInv := 0, 0
		for _, g := range s.GaItems {
			if g.IsEmpty() {
				freeGa++
			}
		}
		for _, it := range s.Inventory.CommonItems {
			if it.GaItemHandle == 0 || it.GaItemHandle == 0xFFFFFFFF {
				freeInv++
			}
		}
		if freeGa >= 4 && freeInv >= 4 {
			slotIdx = i
			break
		}
	}
	if slotIdx < 0 {
		t.Skip("no active slot with free capacity")
	}

	const golemBase = uint32(0x02810590) // Golem Greatbow, somber, MaxUpgrade 10
	const golemPlus25 = golemBase + 25   // 0x028105A9 — the invalid encoding
	const golemPlus10 = golemBase + 10   // 0x0281059A — the clamped, valid encoding

	// Request +25 via BOTH upgrade params; the add path must route by MaxUpgrade
	// and clamp to 10.
	if _, err := app.AddItemsToCharacter(slotIdx, []uint32{golemBase}, 25, 25, 0, 0, 1, 0); err != nil {
		t.Fatalf("AddItemsToCharacter: %v", err)
	}

	slot := &save.Slots[slotIdx]
	has := func(itemID uint32) bool {
		for _, id := range slot.GaMap {
			if id == itemID {
				return true
			}
		}
		return false
	}
	if has(golemPlus25) {
		t.Errorf("add stored Golem Greatbow at +25 (0x%08X) — clamp failed", golemPlus25)
	}
	if !has(golemPlus10) {
		t.Errorf("add did not store Golem Greatbow at clamped +10 (0x%08X)", golemPlus10)
	}
}
