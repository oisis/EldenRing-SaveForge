package main

import (
	"testing"

	"github.com/oisis/EldenRing-SaveForge/backend/db"
)

func TestAddItemCaps_NormalAndGameModes(t *testing.T) {
	item := db.GetItemData(0x40000FA0) // Glintstone Pebble: safe 1/0, game 99/600

	inventory, storage := addItemCaps(item, false)
	if inventory != 1 || storage != 0 {
		t.Fatalf("Normal Mode caps = %d/%d, want 1/0", inventory, storage)
	}

	inventory, storage = addItemCaps(item, true)
	if inventory != 99 || storage != 600 {
		t.Fatalf("game caps = %d/%d, want 99/600", inventory, storage)
	}
}

func TestAddItemCaps_GameModeFallsBackWhenUnknown(t *testing.T) {
	item := db.GetItemData(0x000F4240) // weapon: no per-record game quantity cap
	if item.Name == "" {
		t.Fatal("weapon fixture did not resolve")
	}
	if item.GameMaxInventoryKnown || item.GameMaxStorageKnown {
		t.Fatal("weapon fixture unexpectedly has generated per-record game limits")
	}

	inventory, storage := addItemCaps(item, true)
	if inventory != item.MaxInventory || storage != item.MaxStorage {
		t.Fatalf("fallback caps = %d/%d, want safe %d/%d", inventory, storage, item.MaxInventory, item.MaxStorage)
	}
}
