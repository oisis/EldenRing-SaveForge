package main

import (
	"bytes"
	"encoding/binary"
	"os"
	"path/filepath"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

// melinaWantModels is Melina's verified UI→PartsId tuple:
// Face 6→50, Hair 22→106, Eye 0→0, Eyebrow 16→15, Beard 1→0, Eyepatch 1→0,
// Decal 29→29, Eyelash 4→3.
var melinaWantModels = [8]uint32{50, 106, 0, 15, 0, 0, 29, 3}

const melinaName = "Melina, the Tarnished Finger Maiden"

// TestWriteSelectedToFavorites_TypeB_Melina is the A5 Mirror-writer regression:
// a fully-mapped Type B preset written to a Mirror slot must carry the female
// body-type byte, the exact verified raw model IDs (tattoo included), and the
// verbatim FaceShape/Body/Skin — fixture-free, in-memory.
func TestWriteSelectedToFavorites_TypeB_Melina(t *testing.T) {
	const charIdx = 0
	preset := findPresetByName(melinaName)
	if preset == nil || preset.BodyType != 0 {
		t.Fatalf("fixture assumption broken: %q must be a known Type B preset", melinaName)
	}

	app := &App{save: &core.SaveFile{}, favSlotNames: make(map[int]string)}
	app.save.UserData10.Data = make([]byte, 0x60000)
	app.save.Slots[charIdx] = core.SaveSlot{
		Data:           make([]byte, core.FaceDataBlobSize),
		FaceDataOffset: core.FaceDataBlobSize, // → FaceDataStart() == 0
	}

	written, err := app.WriteSelectedToFavorites(charIdx, []string{melinaName})
	if err != nil {
		t.Fatalf("WriteSelectedToFavorites: %v", err)
	}
	if written != 1 {
		t.Fatalf("written = %d, want 1", written)
	}

	slotOff := core.FavBaseOffset // first free slot is 0
	ud := app.save.UserData10.Data

	if magic := string(ud[slotOff+core.FavOffMagic : slotOff+core.FavOffMagic+4]); magic != "FACE" {
		t.Fatalf("slot magic = %q, want FACE", magic)
	}
	// Type B → body type byte 1 (female, inverted vs gender).
	if bt := ud[slotOff+core.FavOffBodyType]; bt != 1 {
		t.Errorf("body type byte = %d, want 1 (Type B female)", bt)
	}
	for i, exp := range melinaWantModels {
		got := binary.LittleEndian.Uint32(ud[slotOff+core.FavOffModelIDs+i*4:])
		if got != exp {
			t.Errorf("model[%d] = %d, want %d", i, got, exp)
		}
	}
	// Decal (index 6) is explicitly preserved at 29 — never zeroed.
	if decal := binary.LittleEndian.Uint32(ud[slotOff+core.FavOffModelIDs+6*4:]); decal != 29 {
		t.Errorf("Decal = %d, want 29", decal)
	}
	if got := ud[slotOff+core.FavOffFaceShape : slotOff+core.FavOffFaceShape+64]; !bytes.Equal(got, preset.FaceShape[:]) {
		t.Error("FaceShape not copied")
	}
	if got := ud[slotOff+core.FavOffBody : slotOff+core.FavOffBody+7]; !bytes.Equal(got, preset.Body[:]) {
		t.Error("Body not copied")
	}
	if got := ud[slotOff+core.FavOffSkin : slotOff+core.FavOffSkin+91]; !bytes.Equal(got, preset.Skin[:]) {
		t.Error("Skin not copied")
	}
}

// TestWriteSelectedToFavorites_TypeB_RoundTrip writes a mapped Type B preset to
// the real fixture save, serializes to a temp copy ONLY, and reloads it. It
// proves the PC UserData10 checksum is consistent and the Type B Mirror entry's
// eight models (incl. Decal) survive a full save/reload cycle. Skips cleanly
// when tmp/save/ER0000.sl2 is absent (realSaveAppForSave handles that).
func TestWriteSelectedToFavorites_TypeB_RoundTrip(t *testing.T) {
	app, idx := realSaveAppForSave(t)

	if findPresetByName(melinaName) == nil {
		t.Fatalf("preset %q not found", melinaName)
	}

	written, err := app.WriteSelectedToFavorites(idx, []string{melinaName})
	if err != nil {
		t.Fatalf("WriteSelectedToFavorites: %v", err)
	}
	if written != 1 {
		t.Fatalf("written = %d, want 1", written)
	}
	slotIdx := -1
	for i, n := range app.favSlotNames {
		if n == melinaName {
			slotIdx = i
			break
		}
	}
	if slotIdx < 0 {
		t.Fatal("written preset not found in favSlotNames")
	}

	// Serialize to a throwaway path — never touch tmp/save/.
	outPath := filepath.Join(t.TempDir(), "roundtrip.sl2")
	if err := app.save.SaveFile(outPath); err != nil {
		t.Fatalf("SaveFile: %v", err)
	}

	// PC layout: 0x300 header + 10 × (0x10 MD5 + 0x280000 slot), then the
	// UserData10 block (0x10 MD5 + 0x60000 data). Verify the written MD5 prefix
	// matches ComputeMD5 of the data it guards.
	if app.save.Platform == core.PlatformPC {
		raw, err := os.ReadFile(outPath)
		if err != nil {
			t.Fatalf("read back: %v", err)
		}
		const ud10MD5Off = 0x300 + 10*0x280010
		if len(raw) < ud10MD5Off+0x10+0x60000 {
			t.Fatalf("saved file too small: %d bytes", len(raw))
		}
		prefix := raw[ud10MD5Off : ud10MD5Off+0x10]
		udData := raw[ud10MD5Off+0x10 : ud10MD5Off+0x10+0x60000]
		want := core.ComputeMD5(udData)
		if !bytes.Equal(prefix, want[:]) {
			t.Errorf("UserData10 MD5 prefix != ComputeMD5(UserData10.Data)")
		}
	}

	// Reload the temp copy and confirm the Mirror entry survived intact.
	save2, err := core.LoadSave(outPath)
	if err != nil {
		t.Fatalf("LoadSave(temp): %v", err)
	}
	ud := save2.UserData10.Data
	slotOff := core.FavBaseOffset + slotIdx*core.FavSlotSize
	if magic := string(ud[slotOff+core.FavOffMagic : slotOff+core.FavOffMagic+4]); magic != "FACE" {
		t.Fatalf("reloaded slot magic = %q, want FACE", magic)
	}
	for i, exp := range melinaWantModels {
		got := binary.LittleEndian.Uint32(ud[slotOff+core.FavOffModelIDs+i*4:])
		if got != exp {
			t.Errorf("reloaded model[%d] = %d, want %d", i, got, exp)
		}
	}
	if decal := binary.LittleEndian.Uint32(ud[slotOff+core.FavOffModelIDs+6*4:]); decal != 29 {
		t.Errorf("reloaded Decal = %d, want 29", decal)
	}
}
