package main

import (
	"strings"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

func snapshotTestSave() *core.SaveFile {
	save := &core.SaveFile{Platform: core.PlatformPC}
	save.ActiveSlots[0] = true
	save.Slots[0].Version = 7
	save.Slots[0].GaItems = make([]core.GaItemFull, 2)
	save.Slots[0].Warnings = []string{"private parser context must not be logged"}
	save.Slots[0].Player.CharacterName[0] = 'A'
	save.Slots[0].Player.CharacterName[1] = 'l'
	save.Slots[0].Player.CharacterName[2] = 'i'
	save.Slots[0].Player.CharacterName[3] = 'c'
	save.Slots[0].Player.CharacterName[4] = 'e'
	save.Slots[1].Player.CharacterName[0] = 'B' // inactive residual slot
	return save
}

func TestDiagnosticSaveSnapshotIsStructuredAndPrivate(t *testing.T) {
	fields := diagnosticSaveSnapshot(snapshotTestSave(), 12)
	byKey := make(map[string]string, len(fields))
	var serialized strings.Builder
	for _, f := range fields {
		byKey[f.Key] = f.Value
		serialized.WriteString(f.Key)
		serialized.WriteString("=")
		serialized.WriteString(f.Value)
		serialized.WriteString(" ")
	}

	for key, want := range map[string]string{
		"platform":        "PC",
		"save_generation": "12",
		"active_slots":    "1",
		"populated_slots": "1",
		"residual_slots":  "1",
		"parse_warnings":  "1",
	} {
		if got := byKey[key]; got != want {
			t.Errorf("field %q = %q, want %q", key, got, want)
		}
	}
	if got := byKey["slot_0"]; !strings.Contains(got, "state=active") || !strings.Contains(got, "ga_items=0/2") {
		t.Errorf("slot_0 summary = %q, want active state and GaItem capacity", got)
	}
	if got := byKey["slot_1"]; !strings.Contains(got, "state=residual") {
		t.Errorf("slot_1 summary = %q, want residual state", got)
	}
	for _, forbidden := range []string{"Alice", "private parser context"} {
		if strings.Contains(serialized.String(), forbidden) {
			t.Errorf("snapshot leaked %q: %s", forbidden, serialized.String())
		}
	}
}

func TestCommitLoadedSaveRecordsDebugSnapshot(t *testing.T) {
	app := newDebugOperationApp(t)
	app.commitLoadedSave(snapshotTestSave(), "/Users/alice/private/ER0000.sl2", loadOriginFileDialog)

	record := operationEvent(t, app.journal.Tail(), eventSaveStateLoaded)
	if got := operationField(record, "platform"); got != "PC" {
		t.Errorf("snapshot platform = %q, want PC", got)
	}
	if got := operationField(record, "active_slots"); got != "1" {
		t.Errorf("snapshot active_slots = %q, want 1", got)
	}
	if got := operationField(record, "slot_0"); !strings.Contains(got, "state=active") {
		t.Errorf("snapshot slot_0 = %q, want active state", got)
	}

	for _, rec := range app.journal.Tail() {
		for _, f := range rec.Fields {
			if strings.Contains(f.Value, "alice") || strings.Contains(f.Value, "ER0000.sl2") {
				t.Errorf("event %q leaked load path through %q", rec.Event, f.Value)
			}
		}
	}
}

func TestWriteSaveCoreCapturesStateBeforeSerializationFailure(t *testing.T) {
	app := NewApp()
	save := snapshotTestSave()
	app.save = save

	fields, err := app.writeSaveCore("ignored.sl2", save)
	if err == nil {
		t.Fatal("writeSaveCore unexpectedly succeeded with an invalid active slot")
	}
	if got := operationField(diagnosticRecord{Fields: fields}, "platform"); got != "PC" {
		t.Errorf("captured platform = %q, want PC", got)
	}
	if got := operationField(diagnosticRecord{Fields: fields}, "slot_0"); !strings.Contains(got, "state=active") {
		t.Errorf("captured slot_0 = %q, want active state", got)
	}
}
