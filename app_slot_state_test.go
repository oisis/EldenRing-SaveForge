package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

func testCharacterName(s string) [16]uint16 {
	var out [16]uint16
	for i, r := range s {
		if i >= len(out) {
			break
		}
		out[i] = uint16(r)
	}
	return out
}

func newSlotStateTestApp() *App {
	app := NewApp()
	app.save = &core.SaveFile{}
	for i := range app.save.Slots {
		app.save.Slots[i].Data = make([]byte, core.SlotSize)
		app.save.Slots[i].GaMap = make(map[uint32]uint32)
	}
	app.save.UserData10.Data = make([]byte, core.ProfileSummaryOffset+10*core.ProfileSummaryStride)
	return app
}

func TestGetSlotStatesDistinguishesActiveResidualAndEmpty(t *testing.T) {
	app := newSlotStateTestApp()
	app.save.ActiveSlots[0] = true
	app.save.Slots[0].Player.CharacterName = testCharacterName("Active")
	app.save.ActiveSlots[1] = false
	app.save.Slots[1].Player.CharacterName = testCharacterName("Residual")

	states := app.GetSlotStates()
	if len(states) != 10 {
		t.Fatalf("GetSlotStates returned %d states, want 10", len(states))
	}
	if !states[0].Active || states[0].Residual || states[0].Empty || states[0].Name != "Active" {
		t.Fatalf("slot 0 state = %+v, want active named slot", states[0])
	}
	if states[1].Active || !states[1].Residual || states[1].Empty || states[1].Name != "Residual" {
		t.Fatalf("slot 1 state = %+v, want residual named slot", states[1])
	}
	if states[2].Active || states[2].Residual || !states[2].Empty || states[2].Name != "Empty Slot" {
		t.Fatalf("slot 2 state = %+v, want empty slot", states[2])
	}
}

func TestCleanResidualSlotClearsOnlyTargetResidualSlot(t *testing.T) {
	app := newSlotStateTestApp()
	app.save.ActiveSlots[0] = true
	app.save.Slots[0].Player.CharacterName = testCharacterName("Active")
	app.save.ActiveSlots[1] = false
	app.save.Slots[1].Player.CharacterName = testCharacterName("Residual")
	app.save.ProfileSummaries[1] = core.ProfileSummary{CharacterName: testCharacterName("Residual"), Level: 99}

	if err := app.CleanResidualSlot(1); err != nil {
		t.Fatalf("CleanResidualSlot: %v", err)
	}
	states := app.GetSlotStates()
	if !states[1].Empty || states[1].Residual || states[1].Name != "Empty Slot" {
		t.Fatalf("slot 1 after cleanup = %+v, want empty", states[1])
	}
	if !states[0].Active || states[0].Name != "Active" {
		t.Fatalf("active slot disturbed: %+v", states[0])
	}
	if err := app.CleanResidualSlot(0); err == nil {
		t.Fatalf("CleanResidualSlot on active slot succeeded, want error")
	}
}

func TestCloneSlotRejectsResidualDestination(t *testing.T) {
	app := newSlotStateTestApp()
	app.save.ActiveSlots[0] = true
	app.save.Slots[0].Player.CharacterName = testCharacterName("Source")
	app.save.ActiveSlots[1] = false
	app.save.Slots[1].Player.CharacterName = testCharacterName("Residual")

	err := app.CloneSlot(0, 1)
	if err == nil {
		t.Fatalf("CloneSlot into residual destination succeeded, want error")
	}
	if !strings.Contains(err.Error(), "residual character data") {
		t.Fatalf("CloneSlot error = %q, want residual cleanup hint", err.Error())
	}
}
