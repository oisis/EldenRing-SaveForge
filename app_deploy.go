package main

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/deploy"
)

// DiffEntry represents a single change between original and current save state.
type DiffEntry struct {
	Category string `json:"category"` // "stat", "item", "storage", "grace"
	Action   string `json:"action"`   // "changed", "added", "removed"
	Field    string `json:"field"`    // field or item name
	OldValue string `json:"oldValue"`
	NewValue string `json:"newValue"`
}

// SlotDiffSummary is a quick overview for one slot.
type SlotDiffSummary struct {
	SlotIndex   int    `json:"slotIndex"`
	CharName    string `json:"charName"`
	ChangeCount int    `json:"changeCount"`
}

// SlotCapacity reports used vs max counts for GaItems, Inventory, and Storage.
type SlotCapacity struct {
	GaItemsUsed   int `json:"gaItemsUsed"`
	GaItemsMax    int `json:"gaItemsMax"`
	InventoryUsed int `json:"inventoryUsed"`
	InventoryMax  int `json:"inventoryMax"`
	StorageUsed   int `json:"storageUsed"`
	StorageMax    int `json:"storageMax"`
}

// GetSlotDiff compares the current state of a slot against the original loaded state.
func (a *App) GetSlotDiff(idx int) ([]DiffEntry, error) {
	if a.save == nil || a.sourceSave == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if idx < 0 || idx >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	cur := &a.save.Slots[idx]
	orig := &a.sourceSave.Slots[idx]
	var diffs []DiffEntry

	// --- Stats ---
	type statField struct {
		name string
		cur  uint32
		orig uint32
	}
	stats := []statField{
		{"Level", cur.Player.Level, orig.Player.Level},
		{"Vigor", cur.Player.Vigor, orig.Player.Vigor},
		{"Mind", cur.Player.Mind, orig.Player.Mind},
		{"Endurance", cur.Player.Endurance, orig.Player.Endurance},
		{"Strength", cur.Player.Strength, orig.Player.Strength},
		{"Dexterity", cur.Player.Dexterity, orig.Player.Dexterity},
		{"Intelligence", cur.Player.Intelligence, orig.Player.Intelligence},
		{"Faith", cur.Player.Faith, orig.Player.Faith},
		{"Arcane", cur.Player.Arcane, orig.Player.Arcane},
		{"Souls", cur.Player.Souls, orig.Player.Souls},
	}
	for _, s := range stats {
		if s.cur != s.orig {
			diffs = append(diffs, DiffEntry{
				Category: "stat",
				Action:   "changed",
				Field:    s.name,
				OldValue: strconv.FormatUint(uint64(s.orig), 10),
				NewValue: strconv.FormatUint(uint64(s.cur), 10),
			})
		}
	}

	curName := core.UTF16ToString(cur.Player.CharacterName[:])
	origName := core.UTF16ToString(orig.Player.CharacterName[:])
	if curName != origName {
		diffs = append(diffs, DiffEntry{
			Category: "stat",
			Action:   "changed",
			Field:    "Name",
			OldValue: origName,
			NewValue: curName,
		})
	}

	// --- Inventory diff ---
	diffs = append(diffs, diffInventory("item", cur.Inventory, orig.Inventory)...)

	// --- Storage diff ---
	diffs = append(diffs, diffInventory("storage", cur.Storage, orig.Storage)...)

	// --- Graces diff ---
	diffs = append(diffs, a.diffGraces(idx)...)

	// --- Boss diff ---
	diffs = append(diffs, a.diffBosses(idx)...)

	return diffs, nil
}

// diffInventory compares two EquipInventoryData and returns DiffEntries.
func diffInventory(category string, cur, orig core.EquipInventoryData) []DiffEntry {
	var diffs []DiffEntry

	// Build maps: GaItemHandle → item for quick lookup
	type itemInfo struct {
		qty  uint32
		name string
	}
	buildMap := func(items []core.InventoryItem) map[uint32]itemInfo {
		m := make(map[uint32]itemInfo, len(items))
		for _, it := range items {
			if it.GaItemHandle == 0 {
				continue
			}
			name := resolveItemName(it.GaItemHandle)
			existing, ok := m[it.GaItemHandle]
			if ok {
				existing.qty += it.Quantity
				m[it.GaItemHandle] = existing
			} else {
				m[it.GaItemHandle] = itemInfo{qty: it.Quantity, name: name}
			}
		}
		return m
	}

	origAll := append(orig.CommonItems, orig.KeyItems...)
	curAll := append(cur.CommonItems, cur.KeyItems...)
	origMap := buildMap(origAll)
	curMap := buildMap(curAll)

	// Added or changed
	for handle, ci := range curMap {
		oi, existed := origMap[handle]
		if !existed {
			diffs = append(diffs, DiffEntry{
				Category: category,
				Action:   "added",
				Field:    ci.name,
				NewValue: "×" + strconv.FormatUint(uint64(ci.qty), 10),
			})
		} else if ci.qty != oi.qty {
			diffs = append(diffs, DiffEntry{
				Category: category,
				Action:   "changed",
				Field:    ci.name,
				OldValue: "×" + strconv.FormatUint(uint64(oi.qty), 10),
				NewValue: "×" + strconv.FormatUint(uint64(ci.qty), 10),
			})
		}
	}

	// Removed
	for handle, oi := range origMap {
		if _, exists := curMap[handle]; !exists {
			diffs = append(diffs, DiffEntry{
				Category: category,
				Action:   "removed",
				Field:    oi.name,
				OldValue: "×" + strconv.FormatUint(uint64(oi.qty), 10),
			})
		}
	}

	return diffs
}

// resolveItemName tries to get a human-readable name for an inventory item handle.
func resolveItemName(gaItemHandle uint32) string {
	entry, _ := db.GetItemDataFuzzy(gaItemHandle)
	if entry.Name != "" {
		return entry.Name
	}
	return fmt.Sprintf("Item 0x%X", gaItemHandle)
}

// diffGraces compares grace event flags between source and current save.
func (a *App) diffGraces(idx int) []DiffEntry {
	cur := &a.save.Slots[idx]
	orig := &a.sourceSave.Slots[idx]

	if cur.EventFlagsOffset <= 0 || orig.EventFlagsOffset <= 0 {
		return nil
	}
	if cur.EventFlagsOffset >= len(cur.Data) || orig.EventFlagsOffset >= len(orig.Data) {
		return nil
	}

	curFlags := cur.Data[cur.EventFlagsOffset:]
	origFlags := orig.Data[orig.EventFlagsOffset:]
	graces := db.GetAllGraces()

	var diffs []DiffEntry
	for _, g := range graces {
		curVisited, err1 := db.GetEventFlag(curFlags, g.ID)
		origVisited, err2 := db.GetEventFlag(origFlags, g.ID)
		if err1 != nil || err2 != nil {
			continue
		}
		if curVisited != origVisited {
			action := "added"
			if !curVisited {
				action = "removed"
			}
			diffs = append(diffs, DiffEntry{
				Category: "grace",
				Action:   action,
				Field:    g.Name,
			})
		}
	}
	return diffs
}

// diffBosses compares boss defeat event flags between source and current save.
func (a *App) diffBosses(idx int) []DiffEntry {
	cur := &a.save.Slots[idx]
	orig := &a.sourceSave.Slots[idx]

	if cur.EventFlagsOffset <= 0 || orig.EventFlagsOffset <= 0 {
		return nil
	}
	if cur.EventFlagsOffset >= len(cur.Data) || orig.EventFlagsOffset >= len(orig.Data) {
		return nil
	}

	curFlags := cur.Data[cur.EventFlagsOffset:]
	origFlags := orig.Data[orig.EventFlagsOffset:]
	bosses := db.GetAllBosses()

	var diffs []DiffEntry
	for _, b := range bosses {
		curDefeated, err1 := db.GetEventFlag(curFlags, b.ID)
		origDefeated, err2 := db.GetEventFlag(origFlags, b.ID)
		if err1 != nil || err2 != nil {
			continue
		}
		if curDefeated != origDefeated {
			action := "added"
			if !curDefeated {
				action = "removed"
			}
			diffs = append(diffs, DiffEntry{
				Category: "boss",
				Action:   action,
				Field:    b.Name + " (" + b.Region + ")",
			})
		}
	}
	return diffs
}

// GetSaveDiffSummary returns a quick change-count overview for all active slots.
func (a *App) GetSaveDiffSummary() ([]SlotDiffSummary, error) {
	if a.save == nil || a.sourceSave == nil {
		return nil, fmt.Errorf("no save loaded")
	}

	var summaries []SlotDiffSummary
	for i := 0; i < 10; i++ {
		if !a.save.ActiveSlots[i] {
			continue
		}
		diffs, err := a.GetSlotDiff(i)
		if err != nil {
			continue
		}
		name := core.UTF16ToString(a.save.Slots[i].Player.CharacterName[:])
		summaries = append(summaries, SlotDiffSummary{
			SlotIndex:   i,
			CharName:    name,
			ChangeCount: len(diffs),
		})
	}
	return summaries, nil
}

// GetSlotCapacity returns capacity usage for a character slot.
func (a *App) GetSlotCapacity(charIdx int) (*SlotCapacity, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	usage := core.CountSlotUsage(&a.save.Slots[charIdx])
	return &SlotCapacity{
		GaItemsUsed:   usage.GaItemsUsed,
		GaItemsMax:    usage.GaItemsMax,
		InventoryUsed: usage.InventoryUsed,
		InventoryMax:  usage.InventoryMax,
		StorageUsed:   usage.StorageUsed,
		StorageMax:    usage.StorageMax,
	}, nil
}

// GetDeployTargets returns all configured deploy targets.
func (a *App) GetDeployTargets() []deploy.Target {
	if a.deployStore == nil {
		return nil
	}
	return a.deployStore.List()
}

// SaveDeployTarget adds or updates a deploy target.
func (a *App) SaveDeployTarget(t deploy.Target) error {
	if a.deployStore == nil {
		return fmt.Errorf("deploy not initialized")
	}
	return a.deployStore.Save(t)
}

// DeleteDeployTarget removes a deploy target by name.
func (a *App) DeleteDeployTarget(name string) error {
	if a.deployStore == nil {
		return fmt.Errorf("deploy not initialized")
	}
	return a.deployStore.Delete(name)
}

// isLocalTarget returns true if the named target is configured as local.
func (a *App) isLocalTarget(name string) bool {
	if a.deployStore == nil {
		return false
	}
	t, ok := a.deployStore.Get(name)
	return ok && t.IsLocal()
}

// TestSSHConnection tests connectivity to a target (SSH or local path).
func (a *App) TestSSHConnection(targetName string) (string, error) {
	if a.deployStore == nil {
		return "", fmt.Errorf("deploy not initialized")
	}
	if a.isLocalTarget(targetName) {
		return a.deployLocal.TestConnection(targetName)
	}
	return a.deploySSH.TestConnection(targetName)
}

// DeploySave writes the current in-memory save to a temp file and uploads/copies it to a target.
// Returns a human-readable success message with file size.
func (a *App) DeploySave(targetName string) (string, error) {
	if a.deployStore == nil {
		return "", fmt.Errorf("deploy not initialized")
	}
	if a.save == nil {
		return "", fmt.Errorf("no save loaded")
	}

	// Write current working state to a temp file for upload
	tmpPath, err := a.writeTempSave()
	if err != nil {
		return "", err
	}
	defer os.Remove(tmpPath)

	info, _ := os.Stat(tmpPath)
	sizeMB := float64(info.Size()) / (1024 * 1024)

	if a.isLocalTarget(targetName) {
		if err := a.deployLocal.UploadSave(targetName, tmpPath); err != nil {
			return "", err
		}
	} else {
		if err := a.deploySSH.UploadSave(targetName, tmpPath); err != nil {
			return "", err
		}
	}

	t, _ := a.deployStore.Get(targetName)
	return fmt.Sprintf("Uploaded %.1f MB to %s", sizeMB, t.Name), nil
}

// DownloadRemoteSave downloads/copies a save file from a target and loads it.
// The temp file is removed after loading into memory.
func (a *App) DownloadRemoteSave(targetName string) (string, error) {
	if a.deployStore == nil {
		return "", fmt.Errorf("deploy not initialized")
	}

	tmpDir, err := os.MkdirTemp("", "er-save-download-")
	if err != nil {
		return "", fmt.Errorf("cannot create temp dir: %w", err)
	}
	defer os.RemoveAll(tmpDir)

	localPath := tmpDir + "/ER0000.sl2"

	if a.isLocalTarget(targetName) {
		if err := a.deployLocal.DownloadSave(targetName, localPath); err != nil {
			return "", err
		}
	} else {
		if err := a.deploySSH.DownloadSave(targetName, localPath); err != nil {
			return "", err
		}
	}

	save, err := core.LoadSave(localPath)
	if err != nil {
		return "", fmt.Errorf("downloaded file is not a valid save: %w", err)
	}
	a.save = save
	a.lastSavePath = ""
	a.favSlotNames = make(map[int]string)
	a.clearAllUndoStacks()
	a.clearAllEditSessions()
	return string(save.Platform), nil
}

// LaunchRemoteGame starts the game on a target (SSH or local).
func (a *App) LaunchRemoteGame(targetName string) (string, error) {
	if a.deployStore == nil {
		return "", fmt.Errorf("deploy not initialized")
	}
	if a.isLocalTarget(targetName) {
		return a.deployLocal.LaunchGame(targetName)
	}
	return a.deploySSH.LaunchGame(targetName)
}

// CloseRemoteGame stops the game on a target (SSH or local).
func (a *App) CloseRemoteGame(targetName string) (string, error) {
	if a.deployStore == nil {
		return "", fmt.Errorf("deploy not initialized")
	}
	if a.isLocalTarget(targetName) {
		return a.deployLocal.CloseGame(targetName)
	}
	return a.deploySSH.CloseGame(targetName)
}

// DeployAndLaunch performs: write temp → upload → launch (no close).
func (a *App) DeployAndLaunch(targetName string) error {
	if a.deployStore == nil {
		return fmt.Errorf("deploy not initialized")
	}
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}

	tmpPath, err := a.writeTempSave()
	if err != nil {
		return err
	}
	defer os.Remove(tmpPath)

	// Upload save
	if a.isLocalTarget(targetName) {
		if err := a.deployLocal.UploadSave(targetName, tmpPath); err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}
	} else {
		if err := a.deploySSH.UploadSave(targetName, tmpPath); err != nil {
			return fmt.Errorf("upload failed: %w", err)
		}
	}

	// Launch game
	if a.isLocalTarget(targetName) {
		if _, err := a.deployLocal.LaunchGame(targetName); err != nil {
			return fmt.Errorf("launch failed: %w", err)
		}
	} else {
		if _, err := a.deploySSH.LaunchGame(targetName); err != nil {
			return fmt.Errorf("launch failed: %w", err)
		}
	}

	return nil
}

// CloseAndDownload performs: close game → wait for save flush → download → load.
// The temp file is removed after loading into memory.
func (a *App) CloseAndDownload(targetName string) (string, error) {
	if a.deployStore == nil {
		return "", fmt.Errorf("deploy not initialized")
	}

	// Close the game (ignore errors — game might not be running)
	if a.isLocalTarget(targetName) {
		a.deployLocal.CloseGame(targetName)
	} else {
		a.deploySSH.CloseGame(targetName)
	}

	// Wait for graceful shutdown and save file flush
	time.Sleep(5 * time.Second)

	// Download save
	return a.DownloadRemoteSave(targetName)
}

// writeTempSave serializes the current in-memory save to a temp file, preserving target platform.
func (a *App) writeTempSave() (string, error) {
	tmpFile, err := os.CreateTemp("", "er-deploy-*.sl2")
	if err != nil {
		return "", fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmpFile.Name()
	tmpFile.Close()

	if err := a.save.SaveFile(tmpPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("failed to write temp save: %w", err)
	}
	return tmpPath, nil
}
