package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

func testCharacterName(s string) [16]uint16 {
	return encodeCharacterName16(s)
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

func TestCloneSlotAssignsUniqueCharacterName(t *testing.T) {
	app := newSlotStateTestApp()
	app.save.ActiveSlots[0] = true
	app.save.Slots[0].Player.CharacterName = testCharacterName("Niziol")
	app.save.ProfileSummaries[0].CharacterName = testCharacterName("Niziol")
	app.save.ActiveSlots[2] = true
	app.save.Slots[2].Player.CharacterName = testCharacterName("Niziol 2")

	if err := app.CloneSlot(0, 1); err != nil {
		t.Fatalf("CloneSlot: %v", err)
	}
	if got := core.UTF16ToString(app.save.Slots[1].Player.CharacterName[:]); got != "Niziol 3" {
		t.Fatalf("cloned slot name = %q, want Niziol 3", got)
	}
	if got := core.UTF16ToString(app.save.ProfileSummaries[1].CharacterName[:]); got != "Niziol 3" {
		t.Fatalf("cloned profile summary name = %q, want Niziol 3", got)
	}
}

func TestCloneSlotConsidersResidualNamesWhenChoosingSuffix(t *testing.T) {
	app := newSlotStateTestApp()
	app.save.ActiveSlots[0] = true
	app.save.Slots[0].Player.CharacterName = testCharacterName("Name")
	app.save.ActiveSlots[2] = false
	app.save.Slots[2].Player.CharacterName = testCharacterName("Name 2")

	if err := app.CloneSlot(0, 1); err != nil {
		t.Fatalf("CloneSlot: %v", err)
	}
	if got := core.UTF16ToString(app.save.Slots[1].Player.CharacterName[:]); got != "Name 3" {
		t.Fatalf("cloned slot name = %q, want Name 3", got)
	}
}

func TestCloneSlotTrimsNameToFitSuffixUTF16Limit(t *testing.T) {
	app := newSlotStateTestApp()
	sourceName := strings.Repeat("😀", 8)
	want := strings.Repeat("😀", 7) + " 2"
	app.save.ActiveSlots[0] = true
	app.save.Slots[0].Player.CharacterName = encodeCharacterName16(sourceName)

	if err := app.CloneSlot(0, 1); err != nil {
		t.Fatalf("CloneSlot: %v", err)
	}
	got := core.UTF16ToString(app.save.Slots[1].Player.CharacterName[:])
	if got != want {
		t.Fatalf("cloned slot name = %q, want %q", got, want)
	}
	if utf16UnitLen(got) > 16 {
		t.Fatalf("cloned slot name uses %d UTF-16 units, want <= 16", utf16UnitLen(got))
	}
}

func TestRevertSlotRestoresCloneDestinationActivityAndSummary(t *testing.T) {
	app := newSlotStateTestApp()
	app.save.ActiveSlots[0] = true
	app.save.Slots[0].Player.CharacterName = testCharacterName("Source")
	app.save.ProfileSummaries[0].CharacterName = testCharacterName("Source")

	if err := app.CloneSlot(0, 1); err != nil {
		t.Fatalf("CloneSlot: %v", err)
	}
	if err := app.RevertSlot(1); err != nil {
		t.Fatalf("RevertSlot: %v", err)
	}
	state := app.GetSlotStates()[1]
	if !state.Empty || state.Active || state.Residual {
		t.Fatalf("slot 1 after revert = %+v, want empty inactive slot", state)
	}
	if got := core.UTF16ToString(app.save.ProfileSummaries[1].CharacterName[:]); got != "" {
		t.Fatalf("profile summary name after revert = %q, want empty", got)
	}
}
