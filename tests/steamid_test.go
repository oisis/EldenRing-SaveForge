package tests

import (
	"encoding/binary"
	"os"
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
)

func TestSteamIDFlush(t *testing.T) {
	path := "../tmp/save/ER0000.sl2"
	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Skip("Test save file not found in tmp/save/")
	}

	save, err := core.LoadSave(path)
	if err != nil {
		t.Fatalf("Failed to load save: %v", err)
	}

	originalSteamID := save.SteamID
	newSteamID := originalSteamID + 1
	save.SteamID = newSteamID

	tmpPath := "data/pc/steamid_test.sl2"
	os.MkdirAll("data/pc", 0755)
	if err := save.SaveFile(tmpPath); err != nil {
		t.Fatalf("Failed to write save: %v", err)
	}
	defer os.Remove(tmpPath)

	reloaded, err := core.LoadSave(tmpPath)
	if err != nil {
		t.Fatalf("Failed to reload save: %v", err)
	}

	if reloaded.SteamID != newSteamID {
		t.Errorf("SteamID not persisted: expected %d, got %d", newSteamID, reloaded.SteamID)
	}

	// Verify it's flushed into the raw UserData10.Data
	flushed := binary.LittleEndian.Uint64(reloaded.UserData10.Data[0:8])
	if flushed != newSteamID {
		t.Errorf("SteamID not in UserData10.Data: expected %d, got %d", newSteamID, flushed)
	}
}
