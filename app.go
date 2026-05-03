package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"math"
	"os"
	"sort"
	"strconv"
	"time"

	"github.com/oisis/EldenRing-SaveEditor/backend/core"
	"github.com/oisis/EldenRing-SaveEditor/backend/db"
	"github.com/oisis/EldenRing-SaveEditor/backend/db/data"
	"github.com/oisis/EldenRing-SaveEditor/backend/deploy"
	"github.com/oisis/EldenRing-SaveEditor/backend/vm"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxUndoDepth = 5

// slotSnapshot holds a deep copy of a SaveSlot for undo purposes.
type slotSnapshot struct {
	Data              []byte
	Version           uint32
	Player            core.PlayerGameData
	GaMap             map[uint32]uint32
	GaItems           []core.GaItemFull
	Inventory         core.EquipInventoryData
	Storage           core.EquipInventoryData
	Warnings          []string
	MagicOffset       int
	InventoryEnd      int
	EventFlagsOffset  int
	PlayerDataOffset  int
	FaceDataOffset    int
	StorageBoxOffset  int
	IngameTimerOffset int
	GaItemDataOffset  int
	// GaItem tracked indices
	NextAoWIndex      int
	NextArmamentIndex int
	NextGaItemHandle  uint32
	PartGaItemHandle  uint8
}

// App struct
type App struct {
	ctx          context.Context
	save         *core.SaveFile
	sourceSave   *core.SaveFile
	undoStacks   [10][]slotSnapshot
	lastSavePath string
	deployStore  *deploy.TargetStore
	deploySSH    *deploy.SSHManager
	deployLocal  *deploy.LocalManager
}

// NewApp creates a new App struct
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx

	store, err := deploy.NewTargetStore()
	if err != nil {
		fmt.Printf("Warning: deploy targets unavailable: %v\n", err)
		return
	}
	a.deployStore = store
	a.deploySSH = deploy.NewSSHManager(store)
	a.deployLocal = deploy.NewLocalManager(store)
}

func (a *App) logInfo(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	runtime.LogInfof(a.ctx, "%s", msg)
	runtime.EventsEmit(a.ctx, "app:log", "info", msg)
}

func (a *App) logError(format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	runtime.LogErrorf(a.ctx, "%s", msg)
	runtime.EventsEmit(a.ctx, "app:log", "error", msg)
}

// SelectAndOpenSave opens a native file dialog and loads the selected save
func (a *App) SelectAndOpenSave() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Elden Ring Save File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Elden Ring Save (*.sl2;*.dat;*.txt)", Pattern: "*.sl2;*.dat;*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("no file selected")
	}

	save, err := core.LoadSave(path)
	if err != nil {
		return "", err
	}
	a.save = save
	a.lastSavePath = path
	a.clearAllUndoStacks()
	return string(save.Platform), nil
}

// SelectAndOpenSourceSave opens a native file dialog and loads the selected source save for import
func (a *App) SelectAndOpenSourceSave() (string, error) {
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select SOURCE Elden Ring Save File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Elden Ring Save (*.sl2;*.dat;*.txt)", Pattern: "*.sl2;*.dat;*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", fmt.Errorf("no file selected")
	}

	save, err := core.LoadSave(path)
	if err != nil {
		return "", err
	}
	a.sourceSave = save
	return string(save.Platform), nil
}

// GetCharacter returns the ViewModel for a specific slot
func (a *App) GetCharacter(index int) (*vm.CharacterViewModel, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if index < 0 || index >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	slot := a.save.Slots[index]
	return vm.MapParsedSlotToVM(&slot)
}

// SaveCharacter updates the raw slot data from the ViewModel
func (a *App) SaveCharacter(index int, charVM vm.CharacterViewModel) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if index < 0 || index >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(index)

	// 1. Update the slot data
	if err := vm.ApplyVMToParsedSlot(&charVM, &a.save.Slots[index]); err != nil {
		return err
	}

	// 2. Sync NG+ event flags (50-57) with ClearCount
	slot := &a.save.Slots[index]
	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for i := uint32(0); i <= 7; i++ {
			_ = db.SetEventFlag(flags, 50+i, i == slot.Player.ClearCount)
		}
	}

	// 3. Update ProfileSummary (for the menu)
	a.save.ProfileSummaries[index].Level = a.save.Slots[index].Player.Level
	copy(a.save.ProfileSummaries[index].CharacterName[:], a.save.Slots[index].Player.CharacterName[:])

	return nil
}

// WriteSave writes the current save state to a file
func (a *App) WriteSave(platform string) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title: "Save Elden Ring Save File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Elden Ring Save (*.sl2;*.dat;*.txt)", Pattern: "*.sl2;*.dat;*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		return err
	}
	if path == "" {
		return fmt.Errorf("no file selected")
	}

	// Backup only when the target file already exists (nothing to protect otherwise).
	if _, statErr := os.Stat(path); statErr == nil {
		if _, err := core.CreateBackup(path); err != nil {
			return fmt.Errorf("backup failed, save aborted: %w", err)
		}
		if err := core.PruneBackups(path, 10); err != nil {
			fmt.Printf("Warning: failed to prune old backups: %v\n", err)
		}
	}

	// Apply target platform — enables cross-platform conversion.
	origPlatform := a.save.Platform
	a.save.Platform = core.Platform(platform)
	if platform == "PC" && origPlatform == core.PlatformPS {
		// PS4 → PC: enable AES encryption with a fresh random IV.
		iv := make([]byte, 16)
		if _, err := rand.Read(iv); err != nil {
			return fmt.Errorf("failed to generate IV for encryption: %w", err)
		}
		a.save.IV = iv
		a.save.Encrypted = true
	}
	if platform == "PS4" {
		a.save.Encrypted = false
	}

	if err := a.save.SaveFile(path); err != nil {
		return err
	}
	a.lastSavePath = path
	a.clearAllUndoStacks()
	return nil
}

// GetItemList returns a list of items for a given category, filtered by the loaded save's platform.
func (a *App) GetItemList(category string) []db.ItemEntry {
	platform := "PS4"
	if a.save != nil {
		platform = string(a.save.Platform)
	}
	return db.GetItemsByCategory(category, platform)
}

// GetItemListChunk returns items for a single category. Used by the frontend
// to load the "All Categories" view progressively (one chunk per category)
// instead of blocking on a single large IPC roundtrip.
func (a *App) GetItemListChunk(category string) []db.ItemEntry {
	platform := "PS4"
	if a.save != nil {
		platform = string(a.save.Platform)
	}
	return db.GetItemsByCategory(category, platform)
}

// SkippedAdd reports an item whose requested inventory qty was reduced because
// its container's total-quantity cap was exhausted. CutQty is the number of
// units removed from the requested add (e.g. asked for 12, got 8 → CutQty=4).
type SkippedAdd struct {
	ItemID uint32 `json:"itemID"`
	CutQty int    `json:"cutQty"`
}

// AddResult reports the outcome of an AddItemsToCharacter operation.
type AddResult struct {
	Added       int          `json:"added"`
	Requested   int          `json:"requested"`
	Trimmed     []SkippedAdd `json:"trimmed"`
	CapHit      string       `json:"capHit"`
	FreeInv     int          `json:"freeInv"`
	FreeStore   int          `json:"freeStore"`
	NeededInv   int          `json:"neededInv"`
	NeededStore int          `json:"neededStore"`
}

// AddItemsToCharacter adds multiple items from the database to a character slot.
// ALL-OR-NOTHING for capacity: either all items are added or none. If capacity is
// insufficient, returns AddResult with CapHit set and Added=0 — no mutation occurs.
//
// Container-gated items (Throwing Pots, Aromatics) are best-effort: qty is trimmed
// to fit the container cap without blocking the batch. Trimmed items are reported
// in AddResult.Trimmed.
func (a *App) AddItemsToCharacter(charIdx int, itemIDs []uint32, upgrade25, upgrade10, infuseOffset, upgradeAsh, invQty, storageQty int) (AddResult, error) {
	result := AddResult{Requested: len(itemIDs)}

	if a.save == nil {
		return result, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return result, fmt.Errorf("invalid character index")
	}

	slot := &a.save.Slots[charIdx]

	// Build maps from current inventory/storage state:
	// - existingItemQty: per-item stack qty in inventory (used to compute SET delta)
	// - existingByContainer: total pot/aromatic units per container
	// - existingStorageQty: per-item stack qty in storage
	existingItemQty := make(map[uint32]int)
	existingByContainer := make(map[uint32]int)
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == 0 || item.GaItemHandle == 0xFFFFFFFF {
			continue
		}
		invID := db.HandleToItemID(item.GaItemHandle)
		_, baseID := db.GetItemDataFuzzy(invID)
		qty := int(item.Quantity & 0x7FFFFFFF)
		existingItemQty[baseID] += qty
		if cID, ok := data.GetRequiredContainer(baseID); ok {
			existingByContainer[cID] += qty
		}
	}

	existingStorageQty := make(map[uint32]int)
	for _, item := range slot.Storage.CommonItems {
		if item.GaItemHandle == 0 || item.GaItemHandle == 0xFFFFFFFF {
			continue
		}
		sID := db.HandleToItemID(item.GaItemHandle)
		_, sBaseID := db.GetItemDataFuzzy(sID)
		existingStorageQty[sBaseID] += int(item.Quantity & 0x7FFFFFFF)
	}

	// Existing container key item qtys (so we don't lower them).
	existingKeyItemQty := make(map[uint32]int)
	for _, item := range slot.Inventory.KeyItems {
		if item.GaItemHandle == 0 || item.GaItemHandle == 0xFFFFFFFF {
			continue
		}
		keyID := db.HandleToItemID(item.GaItemHandle)
		_, keyBaseID := db.GetItemDataFuzzy(keyID)
		existingKeyItemQty[keyBaseID] = int(item.Quantity & 0x7FFFFFFF)
	}

	containerCap := func(c uint32) int {
		cData, _ := db.GetItemDataFuzzy(c)
		return int(cData.MaxInventory)
	}

	// Track containers touched by this batch (need auto-update of qty).
	usedContainers := make(map[uint32]bool)

	// FCFS distribution for container caps must be deterministic and intuitive:
	// gated items are processed in ascending ID order so canonical first-of-group
	// (e.g. Fire Pot 0x4000012C) wins the cap, regardless of frontend sort. Non-
	// gated items keep their original order — cap doesn't affect them.
	sortedIDs := make([]uint32, len(itemIDs))
	copy(sortedIDs, itemIDs)
	sort.SliceStable(sortedIDs, func(i, j int) bool {
		_, gI := data.GetRequiredContainer(sortedIDs[i])
		_, gJ := data.GetRequiredContainer(sortedIDs[j])
		if gI != gJ {
			return gI // gated items first
		}
		if gI {
			return sortedIDs[i] < sortedIDs[j] // gated by ID asc
		}
		return false // non-gated stable
	})

	// Pre-compute all items to add (finalIDs, quantities, container caps).
	type preparedItem struct {
		baseID         uint32
		finalID        uint32
		actualInv      int
		actualStorage  int
		forceStackable bool
		isStackable    bool
	}
	var prepared []preparedItem
	var trimmed []SkippedAdd

	for _, id := range sortedIDs {
		itemData, _ := db.GetItemDataFuzzy(id)
		finalID := id
		switch {
		case itemData.Category == "ashes":
			finalID = id + uint32(upgradeAsh)
		case itemData.MaxUpgrade == 25:
			finalID = id + uint32(infuseOffset) + uint32(upgrade25)
		case itemData.MaxUpgrade == 10:
			finalID = id + uint32(upgrade10)
		}

		actualInv := resolveQty(invQty, int(itemData.MaxInventory))
		actualStorage := resolveQty(storageQty, int(itemData.MaxStorage))

		// Skip stackable items already at max qty.
		handlePrefix := db.ItemIDToHandlePrefix(finalID)
		isStackable := handlePrefix == core.ItemTypeItem || handlePrefix == core.ItemTypeAccessory || db.IsArrowID(finalID)
		if isStackable {
			if actualInv > 0 && existingItemQty[id] >= int(itemData.MaxInventory) {
				a.logInfo("already max inv qty %d/%d — skipping %s (0x%08X)", existingItemQty[id], itemData.MaxInventory, itemData.Name, id)
				actualInv = 0
			}
			if actualStorage > 0 && existingStorageQty[id] >= int(itemData.MaxStorage) {
				a.logInfo("already max storage qty %d/%d — skipping %s (0x%08X)", existingStorageQty[id], itemData.MaxStorage, itemData.Name, id)
				actualStorage = 0
			}
		}

		// Container enforcement (inventory only — storage has no cap).
		// Best-effort: trim qty to fit container, don't block the batch.
		if _, gated := data.GetRequiredContainer(id); gated && actualInv > 0 {
			d := data.ApplyContainerCap(id, actualInv, existingItemQty, existingByContainer, containerCap)
			actualInv = d.EffectiveQty
			if d.CutQty > 0 {
				trimmed = append(trimmed, SkippedAdd{ItemID: id, CutQty: d.CutQty})
			}
		}

		if cID, gated := data.GetRequiredContainer(id); gated && (actualInv > 0 || actualStorage > 0) {
			usedContainers[cID] = true
		}

		forceStackable := db.IsArrowID(finalID)

		prepared = append(prepared, preparedItem{
			baseID:         id,
			finalID:        finalID,
			actualInv:      actualInv,
			actualStorage:  actualStorage,
			forceStackable: forceStackable,
			isStackable:    isStackable || forceStackable,
		})
	}

	// PRE-FLIGHT: check if all items fit.
	var capacityItems []core.ItemToAdd
	for _, p := range prepared {
		if p.actualInv == 0 && p.actualStorage == 0 {
			continue
		}
		capacityItems = append(capacityItems, core.ItemToAdd{
			ItemID:         p.finalID,
			InvQty:         p.actualInv,
			StorageQty:     p.actualStorage,
			ForceStackable: p.forceStackable,
			IsStackable:    p.isStackable,
		})
	}

	capReport := core.CheckAddCapacity(slot, capacityItems)
	if !capReport.CanFitAll {
		a.logError("[AddItems] %s: need inv=%d store=%d, free inv=%d store=%d, requested=%d",
			capReport.CapHit, capReport.NeededInv, capReport.NeededStorage, capReport.FreeInv, capReport.FreeStorage, len(itemIDs))
		result.CapHit = capReport.CapHit
		result.FreeInv = capReport.FreeInv
		result.FreeStore = capReport.FreeStorage
		result.NeededInv = capReport.NeededInv
		result.NeededStore = capReport.NeededStorage
		return result, nil
	}

	// SNAPSHOT: deep copy slot state before mutation.
	a.pushUndo(charIdx)
	snapshot := core.SnapshotSlot(slot)

	// MUTATE: batch add all items (one RebuildSlotFull instead of N).
	if err := core.AddItemsToSlotBatch(slot, capacityItems); err != nil {
		core.RestoreSlot(slot, snapshot)
		return result, fmt.Errorf("rollback after batch add: %w", err)
	}

	// POST-FLAGS: event flags, tutorial IDs (safe to set after batch add).
	for _, p := range prepared {
		if flagID, ok := data.AoWItemToFlagID[p.baseID]; ok {
			if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
				if err := db.SetEventFlag(slot.Data[slot.EventFlagsOffset:], flagID, true); err != nil {
					runtime.LogWarningf(a.ctx, "event flag AoW %d: %v", flagID, err)
				}
			}
		}
		if flagID, ok := data.WorldPickupFlagID[p.baseID]; ok {
			if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
				if err := db.SetEventFlag(slot.Data[slot.EventFlagsOffset:], flagID, true); err != nil {
					runtime.LogWarningf(a.ctx, "event flag pickup %d: %v", flagID, err)
				}
			}
		}
		if tutorialID, ok := data.AboutTutorialID[p.baseID]; ok {
			if err := core.AppendTutorialID(slot, tutorialID); err != nil {
				runtime.LogWarningf(a.ctx, "tutorial ID %d: %v", tutorialID, err)
			}
		}
	}

	// Auto-add / update container key item quantities.
	for cID := range usedContainers {
		desired := existingByContainer[cID]
		current := existingKeyItemQty[cID]
		finalQty := desired
		if current > finalQty {
			finalQty = current
		}
		if desired > current {
			if err := core.AddItemsToSlot(slot, []uint32{cID}, desired, 0, false); err != nil {
				core.RestoreSlot(slot, snapshot)
				return result, fmt.Errorf("rollback after container add: %w", err)
			}
			existingKeyItemQty[cID] = desired
		}

		if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
			continue
		}
		flags := slot.Data[slot.EventFlagsOffset:]
		if flagList, ok := data.ContainerPickupFlags[cID]; ok {
			n := finalQty
			if n > len(flagList) {
				n = len(flagList)
			}
			for i := 0; i < n; i++ {
				if err := db.SetEventFlag(flags, flagList[i], true); err != nil {
					runtime.LogWarningf(a.ctx, "container pickup flag %d: %v", flagList[i], err)
				}
			}
		}

		if vendorFlags, ok := data.ContainerVendorPurchaseFlags[cID]; ok {
			for _, f := range vendorFlags {
				if err := db.SetEventFlag(flags, f, true); err != nil {
					runtime.LogWarningf(a.ctx, "vendor purchase flag %d: %v", f, err)
				}
			}
		}
	}

	// RECONCILE: fix storage header count (blind +1 increment may drift).
	core.ReconcileStorageHeader(slot)

	// POST-VALIDATION: check invariants after mutation.
	if violations := core.ValidatePostMutation(slot); len(violations) > 0 {
		core.RestoreSlot(slot, snapshot)
		return result, fmt.Errorf("rollback: post-mutation validation failed: %s", violations[0].Error())
	}

	// SUCCESS: compute final capacity and return.
	finalUsage := core.CountSlotUsage(slot)
	added := 0
	for _, p := range prepared {
		if p.actualInv > 0 || p.actualStorage > 0 {
			added++
		}
	}
	result.Added = added
	result.Trimmed = trimmed
	result.FreeInv = finalUsage.InventoryMax - finalUsage.InventoryUsed
	result.FreeStore = finalUsage.StorageMax - finalUsage.StorageUsed
	return result, nil
}

// RemoveItemsFromCharacter removes items by handle from inventory, storage, or both.
func (a *App) RemoveItemsFromCharacter(charIdx int, handles []uint32, fromInventory, fromStorage bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return fmt.Errorf("invalid character index")
	}
	a.pushUndo(charIdx)

	slot := &a.save.Slots[charIdx]
	for _, handle := range handles {
		if err := core.RemoveItemFromSlot(slot, handle, fromInventory, fromStorage); err != nil {
			return err
		}
	}
	return nil
}

// resolveQty converts a qty directive into an actual quantity.
// qty=0 → 0 (skip); qty=-1 → max; qty>0 → min(qty, max).
func resolveQty(qty, max int) int {
	if qty == 0 || max == 0 {
		return 0
	}
	if qty < 0 {
		return max
	}
	if qty > max {
		return max
	}
	return qty
}

// GetInfuseTypes returns all weapon infusion types with their ID offsets
func (a *App) GetInfuseTypes() []db.InfuseType {
	return db.GetInfuseTypes()
}

// GetAllGraces returns all Sites of Grace (no visited state)
func (a *App) GetAllGraces() []db.GraceEntry {
	return db.GetAllGraces()
}

// GetGraces returns all Sites of Grace with visited state from the specified character slot
func (a *App) GetGraces(slotIndex int) ([]db.GraceEntry, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	slot := &a.save.Slots[slotIndex]
	graces := db.GetAllGraces()

	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for i := range graces {
			visited, err := db.GetEventFlag(flags, graces[i].ID)
			if err != nil {
				fmt.Printf("Warning: grace %d (%s): %v\n", graces[i].ID, graces[i].Name, err)
				continue
			}
			graces[i].Visited = visited
		}
	}

	return graces, nil
}

// SetGraceVisited sets or clears the visited flag for a Site of Grace in the specified character slot
func (a *App) SetGraceVisited(slotIndex int, graceID uint32, visited bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	flags := slot.Data[slot.EventFlagsOffset:]
	if err := db.SetEventFlag(flags, graceID, visited); err != nil {
		return fmt.Errorf("failed to set grace %d: %w", graceID, err)
	}

	// Automatically open/close dungeon entrance door when toggling catacomb/hero's grave graces
	if gd, ok := data.Graces[graceID]; ok && gd.DoorFlag != 0 {
		if err := db.SetEventFlag(flags, gd.DoorFlag, visited); err != nil {
			return fmt.Errorf("failed to set door flag %d: %w", gd.DoorFlag, err)
		}
	}

	return nil
}

// GetBosses returns all boss encounters with defeated state from the specified character slot
func (a *App) GetBosses(slotIndex int) ([]db.BossEntry, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	slot := &a.save.Slots[slotIndex]
	bosses := db.GetAllBosses()

	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for i := range bosses {
			defeated, err := db.GetEventFlag(flags, bosses[i].ID)
			if err != nil {
				continue
			}
			bosses[i].Defeated = defeated
		}
	}

	return bosses, nil
}

// SetBossDefeated sets or clears the defeated flag for a boss in the specified character slot
func (a *App) SetBossDefeated(slotIndex int, bossID uint32, defeated bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	flags := slot.Data[slot.EventFlagsOffset:]
	if err := db.SetEventFlag(flags, bossID, defeated); err != nil {
		return fmt.Errorf("failed to set boss %d: %w", bossID, err)
	}
	return nil
}

// GetSummoningPools returns all summoning pools with activation state from the specified character slot
func (a *App) GetSummoningPools(slotIndex int) ([]db.SummoningPoolEntry, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	slot := &a.save.Slots[slotIndex]
	pools := db.GetAllSummoningPools()

	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for i := range pools {
			activated, err := db.GetEventFlag(flags, pools[i].ID)
			if err != nil {
				continue
			}
			pools[i].Activated = activated
		}
	}

	return pools, nil
}

// SetSummoningPoolActivated sets or clears the activation flag for a summoning pool
func (a *App) SetSummoningPoolActivated(slotIndex int, poolID uint32, activated bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	flags := slot.Data[slot.EventFlagsOffset:]
	if err := db.SetEventFlag(flags, poolID, activated); err != nil {
		return fmt.Errorf("failed to set summoning pool %d: %w", poolID, err)
	}
	return nil
}

// GetUnlockedRegions returns every known invasion region annotated with the unlock state
// from the specified character slot's UnlockedRegions list.
func (a *App) GetUnlockedRegions(slotIndex int) ([]db.RegionEntry, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	slot := &a.save.Slots[slotIndex]
	entries := db.GetAllRegions()

	unlocked := make(map[uint32]bool, len(slot.UnlockedRegions))
	for _, id := range slot.UnlockedRegions {
		unlocked[id] = true
	}
	for i := range entries {
		entries[i].Unlocked = unlocked[entries[i].ID]
	}
	return entries, nil
}

// SetRegionUnlocked toggles a single invasion region. The slot is rebuilt
// from scratch via core.SetUnlockedRegions, which dedupes + sorts the IDs
// and produces a fresh 0x280000-byte buffer that absorbs the size delta in
// the trailing zero padding.
func (a *App) SetRegionUnlocked(slotIndex int, regionID uint32, unlocked bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.Version == 0 {
		return fmt.Errorf("slot %d is empty", slotIndex)
	}

	// Build the new list: existing IDs ± regionID.
	next := make([]uint32, 0, len(slot.UnlockedRegions)+1)
	already := false
	for _, id := range slot.UnlockedRegions {
		if id == regionID {
			already = true
			if unlocked {
				next = append(next, id)
			}
			continue
		}
		next = append(next, id)
	}
	if unlocked && !already {
		next = append(next, regionID)
	}
	return core.SetUnlockedRegions(slot, next)
}

// BulkSetUnlockedRegions replaces the slot's region list with the given
// IDs (deduped + sorted by core.SetUnlockedRegions). Used by the World tab
// for per-area Unlock/Lock and the global Unlock All / Lock All actions.
func (a *App) BulkSetUnlockedRegions(slotIndex int, regionIDs []uint32) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.Version == 0 {
		return fmt.Errorf("slot %d is empty", slotIndex)
	}
	return core.SetUnlockedRegions(slot, regionIDs)
}

// GetColosseums returns all colosseums with unlock state from the specified character slot
func (a *App) GetColosseums(slotIndex int) ([]db.ColosseumEntry, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	slot := &a.save.Slots[slotIndex]
	colosseums := db.GetAllColosseums()

	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for i := range colosseums {
			unlocked, err := db.GetEventFlag(flags, colosseums[i].ID)
			if err != nil {
				continue
			}
			colosseums[i].Unlocked = unlocked
		}
	}

	return colosseums, nil
}

// SetColosseumUnlocked sets or clears the unlock flag for a colosseum
func (a *App) SetColosseumUnlocked(slotIndex int, colosseumID uint32, unlocked bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	flags := slot.Data[slot.EventFlagsOffset:]
	if err := db.SetEventFlag(flags, colosseumID, unlocked); err != nil {
		return fmt.Errorf("failed to set colosseum %d: %w", colosseumID, err)
	}
	return nil
}

// GetGestures returns all gestures with unlock state from the specified character slot.
// GestureGameData is 64×u32 at StorageBoxOffset + DynStorageBox (immediately after storage).
// Gesture slot IDs vary by body type — some gestures have even/odd variants.
func (a *App) GetGestures(slotIndex int) ([]db.GestureEntry, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	slot := &a.save.Slots[slotIndex]
	gestures := db.GetAllGestureSlots()

	gestureDataOff := slot.StorageBoxOffset + core.DynStorageBox
	if gestureDataOff+core.DynStorageToGestures > len(slot.Data) {
		return gestures, nil // return without unlock state
	}

	// Read 64 gesture IDs. Only canonical (odd) IDs are recognised — any
	// even garbage left over from older builds is intentionally not counted
	// as "unlocked" because the game itself does not see those entries.
	gestureSlots := readGestureSlots(slot.Data, gestureDataOff)
	unlockedIDs := make(map[uint32]bool, 64)
	for _, gID := range gestureSlots {
		if gID == data.GestureEmptySentinel || gID == 0 {
			continue
		}
		unlockedIDs[gID] = true
	}
	for i := range gestures {
		gestures[i].Unlocked = unlockedIDs[gestures[i].ID]
	}

	return gestures, nil
}

// readGestureSlots reads the 64 u32 gesture slot values from the save data.
func readGestureSlots(slotData []byte, gestureDataOff int) []uint32 {
	slots := make([]uint32, 64)
	for i := 0; i < 64; i++ {
		off := gestureDataOff + i*4
		slots[i] = binary.LittleEndian.Uint32(slotData[off : off+4])
	}
	return slots
}

// SetGestureUnlocked adds or removes a gesture from the GestureGameData array.
// gestureID is the canonical odd save-slot ID.
//
// On the lock path we ALSO clear the (id-1) even slot — older builds wrote
// even "body type B" IDs that the game silently ignored, so without this the
// garbage entries would linger and eat sentinel slots needed for re-unlock.
func (a *App) SetGestureUnlocked(slotIndex int, gestureID uint32, unlocked bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	gestureDataOff := slot.StorageBoxOffset + core.DynStorageBox
	if gestureDataOff+core.DynStorageToGestures > len(slot.Data) {
		return fmt.Errorf("gesture data offset 0x%X out of bounds", gestureDataOff)
	}

	gestureSlots := readGestureSlots(slot.Data, gestureDataOff)
	purgeUnknownGestures(gestureSlots)

	if unlocked {
		for _, gID := range gestureSlots {
			if gID == gestureID {
				writeGestureSlots(slot.Data, gestureDataOff, gestureSlots)
				return nil // already present (still flush purge)
			}
		}
		for i, gID := range gestureSlots {
			if gID == data.GestureEmptySentinel {
				gestureSlots[i] = gestureID
				writeGestureSlots(slot.Data, gestureDataOff, gestureSlots)
				return nil
			}
		}
		return fmt.Errorf("no empty gesture slot available")
	}

	for i, gID := range gestureSlots {
		if gID == gestureID {
			gestureSlots[i] = data.GestureEmptySentinel
		}
	}
	writeGestureSlots(slot.Data, gestureDataOff, gestureSlots)
	return nil
}

// purgeUnknownGestures replaces any slot value that is not the empty sentinel
// AND not a known canonical (odd) gesture ID with the sentinel. This drops
// legacy "even body-type B" garbage written by older builds and frees space
// for legitimate Unlock operations.
func purgeUnknownGestures(slots []uint32) {
	for i, gID := range slots {
		if gID == data.GestureEmptySentinel {
			continue
		}
		if _, ok := data.LookupGestureBySlotID(gID); !ok {
			slots[i] = data.GestureEmptySentinel
		}
	}
}

// writeGestureSlots writes 64 u32 gesture IDs back to the save data.
func writeGestureSlots(slotData []byte, gestureDataOff int, slots []uint32) {
	for i, v := range slots {
		off := gestureDataOff + i*4
		binary.LittleEndian.PutUint32(slotData[off:off+4], v)
	}
}

// BulkSetGesturesUnlocked adds or removes multiple gestures in a single call.
// gestureIDs are canonical EvenIDs from the UI; body-type variants are resolved automatically.
func (a *App) BulkSetGesturesUnlocked(slotIndex int, gestureIDs []uint32, unlocked bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	if len(gestureIDs) == 0 {
		return nil
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	gestureDataOff := slot.StorageBoxOffset + core.DynStorageBox
	if gestureDataOff+core.DynStorageToGestures > len(slot.Data) {
		return fmt.Errorf("gesture data offset 0x%X out of bounds", gestureDataOff)
	}

	gestureSlots := readGestureSlots(slot.Data, gestureDataOff)
	purgeUnknownGestures(gestureSlots)

	if unlocked {
		present := make(map[uint32]bool, 64)
		for _, gID := range gestureSlots {
			if gID == data.GestureEmptySentinel || gID == 0 {
				continue
			}
			present[gID] = true
		}
		for _, gestureID := range gestureIDs {
			if present[gestureID] {
				continue
			}
			placed := false
			for i, gID := range gestureSlots {
				if gID == data.GestureEmptySentinel {
					gestureSlots[i] = gestureID
					present[gestureID] = true
					placed = true
					break
				}
			}
			if !placed {
				return fmt.Errorf("no empty gesture slot available (tried to unlock %d gestures)", len(gestureIDs))
			}
		}
	} else {
		// Even legacy IDs were already cleared by purgeUnknownGestures above,
		// so Lock All only needs to wipe the canonical IDs the caller listed.
		removeSet := make(map[uint32]bool, len(gestureIDs))
		for _, id := range gestureIDs {
			removeSet[id] = true
		}
		for i, gID := range gestureSlots {
			if removeSet[gID] {
				gestureSlots[i] = data.GestureEmptySentinel
			}
		}
	}

	writeGestureSlots(slot.Data, gestureDataOff, gestureSlots)
	return nil
}

// DiagnoseSlot performs comprehensive corruption detection on a character slot.
func (a *App) DiagnoseSlot(slotIndex int) (*core.SlotDiagnostics, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	name := core.UTF16ToString(a.save.Slots[slotIndex].Player.CharacterName[:])
	if name == "" {
		return nil, fmt.Errorf("slot %d is empty", slotIndex)
	}

	result := core.DiagnoseSaveCorruption(&a.save.Slots[slotIndex], slotIndex)
	return &result, nil
}

// DiagnoseAllSlots runs corruption detection on all active slots.
func (a *App) DiagnoseAllSlots() ([]core.SlotDiagnostics, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}

	var results []core.SlotDiagnostics
	for i := 0; i < 10; i++ {
		name := core.UTF16ToString(a.save.Slots[i].Player.CharacterName[:])
		if name == "" {
			continue
		}
		result := core.DiagnoseSaveCorruption(&a.save.Slots[i], i)
		results = append(results, result)
	}
	return results, nil
}

// GetQuestNPCs returns the list of NPC names with quest data.
func (a *App) GetQuestNPCs() []string {
	return db.GetAllQuestNPCs()
}

// GetQuestProgress returns the quest progression for a specific NPC in a character slot.
func (a *App) GetQuestProgress(slotIndex int, npcName string) (*db.QuestNPC, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	questSteps, ok := data.QuestData[npcName]
	if !ok {
		return nil, fmt.Errorf("unknown NPC: %s", npcName)
	}

	slot := &a.save.Slots[slotIndex]
	var flags []byte
	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags = slot.Data[slot.EventFlagsOffset:]
	}

	result := &db.QuestNPC{
		Name:  npcName,
		Steps: make([]db.QuestStep, len(questSteps)),
	}

	for i, step := range questSteps {
		qs := db.QuestStep{
			Description: step.Description,
			Location:    step.Location,
			Flags:       make([]db.QuestFlagState, len(step.Flags)),
			Complete:    true,
		}
		for j, flag := range step.Flags {
			var current bool
			if flags != nil {
				current, _ = db.GetEventFlag(flags, flag.ID)
			}
			qs.Flags[j] = db.QuestFlagState{
				ID:      flag.ID,
				Target:  flag.Value,
				Current: current,
			}
			targetBool := flag.Value == 1
			if current != targetBool {
				qs.Complete = false
			}
		}
		result.Steps[i] = qs
	}

	return result, nil
}

// SetQuestStep sets all flags for a quest step to their target values.
func (a *App) SetQuestStep(slotIndex int, npcName string, stepIndex int) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	questSteps, ok := data.QuestData[npcName]
	if !ok {
		return fmt.Errorf("unknown NPC: %s", npcName)
	}
	if stepIndex < 0 || stepIndex >= len(questSteps) {
		return fmt.Errorf("invalid step index %d for %s", stepIndex, npcName)
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	flags := slot.Data[slot.EventFlagsOffset:]
	step := questSteps[stepIndex]
	for _, flag := range step.Flags {
		if err := db.SetEventFlag(flags, flag.ID, flag.Value == 1); err != nil {
			return fmt.Errorf("failed to set flag %d: %w", flag.ID, err)
		}
	}

	return nil
}

// GetCookbooks returns all cookbooks with unlock state from the specified character slot
func (a *App) GetCookbooks(slotIndex int) ([]db.CookbookEntry, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	slot := &a.save.Slots[slotIndex]
	cookbooks := db.GetAllCookbooks()

	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for i := range cookbooks {
			unlocked, err := db.GetEventFlag(flags, cookbooks[i].ID)
			if err != nil {
				continue
			}
			cookbooks[i].Unlocked = unlocked
		}
	}

	return cookbooks, nil
}

// SetCookbookUnlocked sets or clears the unlock flag for a cookbook
func (a *App) SetCookbookUnlocked(slotIndex int, cookbookID uint32, unlocked bool) error {
	return a.BulkSetCookbooksUnlocked(slotIndex, []uint32{cookbookID}, unlocked)
}

// BulkSetCookbooksUnlocked sets event flags AND adds/removes inventory items for multiple cookbooks.
// Single pushUndo — safe for concurrent Wails calls.
func (a *App) BulkSetCookbooksUnlocked(slotIndex int, cookbookIDs []uint32, unlocked bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	flags := slot.Data[slot.EventFlagsOffset:]

	// Collect item IDs to add in batch.
	var itemsToAdd []uint32

	for _, cookbookID := range cookbookIDs {
		if err := db.SetEventFlag(flags, cookbookID, unlocked); err != nil {
			continue
		}

		if itemID, ok := data.CookbookFlagToItemID[cookbookID]; ok {
			if unlocked {
				itemsToAdd = append(itemsToAdd, itemID)
			} else {
				for handle, gID := range slot.GaMap {
					if gID == itemID {
						_ = core.RemoveItemFromSlot(slot, handle, true, false)
						break
					}
				}
			}
		}
	}

	if len(itemsToAdd) > 0 {
		_ = core.AddItemsToSlot(slot, itemsToAdd, 1, 0, false)
	}

	return nil
}

// GetBellBearings returns all bell bearings with unlock state from the specified character slot
func (a *App) GetBellBearings(slotIndex int) ([]db.BellBearingEntry, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	slot := &a.save.Slots[slotIndex]
	entries := db.GetAllBellBearings()
	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for i := range entries {
			unlocked, err := db.GetEventFlag(flags, entries[i].ID)
			if err != nil {
				continue
			}
			entries[i].Unlocked = unlocked
		}
	}
	return entries, nil
}

// SetBellBearingUnlocked sets or clears the acquisition flag for a bell bearing.
// "Unlocked" represents the post-give state — the BB has been turned in to the
// Twin Maiden Husks, which sets the flag AND consumes the key item from inventory.
//   - unlocked=true  → set flag, remove the BB key item from inventory/storage
//   - unlocked=false → clear flag, leave inventory untouched
func (a *App) SetBellBearingUnlocked(slotIndex int, flagID uint32, unlocked bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.pushUndo(slotIndex)
	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}
	if err := db.SetEventFlag(slot.Data[slot.EventFlagsOffset:], flagID, unlocked); err != nil {
		return err
	}
	syncBellBearingItem(slot, flagID, unlocked)
	return nil
}

// syncBellBearingItem applies the inventory-side effect of the acquisition flag.
// "Unlocked" represents giving the BB to the Twin Maidens, which consumes the
// key item — so unlocked=true removes it. unlocked=false (re-locking the BB)
// leaves inventory untouched: the user can spawn the BB key item separately
// via the Item Database if they want to model the pre-give state.
// No-op when the flag has no item mapping.
func syncBellBearingItem(slot *core.SaveSlot, flagID uint32, unlocked bool) {
	itemID, ok := data.BellBearingFlagToItemID[flagID]
	if !ok {
		return
	}
	if !unlocked {
		return
	}
	for handle, gID := range slot.GaMap {
		if gID == itemID {
			_ = core.RemoveItemFromSlot(slot, handle, true, true)
		}
	}
}

// GetWhetblades returns all whetblades with unlock state from the specified character slot
func (a *App) GetWhetblades(slotIndex int) ([]db.WhetbladeEntry, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	slot := &a.save.Slots[slotIndex]
	entries := db.GetAllWhetblades()
	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for i := range entries {
			unlocked, err := db.GetEventFlag(flags, entries[i].ID)
			if err != nil {
				continue
			}
			entries[i].Unlocked = unlocked
		}
	}
	return entries, nil
}

// SetWhetbladeUnlocked sets or clears the unlock flag for a whetblade,
// manages the inventory item, related affinity flags, and the AoW menu flag.
func (a *App) SetWhetbladeUnlocked(slotIndex int, flagID uint32, unlocked bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	flags := slot.Data[slot.EventFlagsOffset:]

	// 1. Set the whetblade event flag.
	if err := db.SetEventFlag(flags, flagID, unlocked); err != nil {
		return err
	}

	// 2. Set related affinity flags (e.g., Keen, Quality for Iron Whetblade).
	if related, ok := data.WhetbladeRelatedFlags[flagID]; ok {
		for _, rf := range related {
			_ = db.SetEventFlag(flags, rf, unlocked)
		}
	}

	// 3. Add/remove inventory item.
	if itemID, ok := data.WhetbladeFlagToItemID[flagID]; ok {
		if unlocked {
			_ = core.AddItemsToSlot(slot, []uint32{itemID}, 1, 0, false)
		} else {
			for handle, gID := range slot.GaMap {
				if gID == itemID {
					_ = core.RemoveItemFromSlot(slot, handle, true, false)
					break
				}
			}
		}
	}

	// 4. Whetstone Knife bonus: add/remove Storm Stomp AoW + duplication flag.
	if flagID == data.WhetstoneKnifeFlag {
		if unlocked {
			_ = core.AddItemsToSlot(slot, []uint32{data.StormStompItemID}, 1, 0, false)
			_ = db.SetEventFlag(flags, data.StormStompDupFlag, true)
		} else {
			for handle, gID := range slot.GaMap {
				if gID == data.StormStompItemID {
					_ = core.RemoveItemFromSlot(slot, handle, true, false)
					break
				}
			}
			_ = db.SetEventFlag(flags, data.StormStompDupFlag, false)
		}
	}

	// 5. Manage AoW menu flag (65800):
	//    - unlock: always set (at least one whetblade is now active)
	//    - lock: clear only if no other whetblades remain unlocked
	if unlocked {
		_ = db.SetEventFlag(flags, data.AoWMenuUnlockedFlag, true)
	} else {
		anyUnlocked := false
		for wbFlag := range data.Whetblades {
			if wbFlag == flagID {
				continue
			}
			if on, err := db.GetEventFlag(flags, wbFlag); err == nil && on {
				anyUnlocked = true
				break
			}
		}
		if !anyUnlocked {
			_ = db.SetEventFlag(flags, data.AoWMenuUnlockedFlag, false)
		}
	}

	return nil
}

// GetAshOfWarFlags returns all Ash of War duplication flags with unlock state
func (a *App) GetAshOfWarFlags(slotIndex int) ([]db.AshOfWarFlagEntry, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	slot := &a.save.Slots[slotIndex]
	entries := db.GetAllAshOfWarFlags()
	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for i := range entries {
			unlocked, err := db.GetEventFlag(flags, entries[i].ID)
			if err != nil {
				continue
			}
			entries[i].Unlocked = unlocked
		}
	}
	return entries, nil
}

// SetAshOfWarFlagUnlocked sets or clears an Ash of War duplication flag
func (a *App) SetAshOfWarFlagUnlocked(slotIndex int, flagID uint32, unlocked bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.pushUndo(slotIndex)
	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}
	return db.SetEventFlag(slot.Data[slot.EventFlagsOffset:], flagID, unlocked)
}

// BulkSetAshOfWarFlags sets multiple AoW duplication flags at once (single undo).
func (a *App) BulkSetAshOfWarFlags(slotIndex int, flagIDs []uint32, unlocked bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.pushUndo(slotIndex)
	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}
	flags := slot.Data[slot.EventFlagsOffset:]
	for _, id := range flagIDs {
		_ = db.SetEventFlag(flags, id, unlocked)
	}
	return nil
}

// BulkSetBellBearings sets multiple bell bearing flags at once (single undo)
// and keeps the matching inventory items in sync via syncBellBearingItem.
func (a *App) BulkSetBellBearings(slotIndex int, flagIDs []uint32, unlocked bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.pushUndo(slotIndex)
	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}
	flags := slot.Data[slot.EventFlagsOffset:]
	for _, id := range flagIDs {
		_ = db.SetEventFlag(flags, id, unlocked)
		syncBellBearingItem(slot, id, unlocked)
	}
	return nil
}

// GetMapProgress returns all map region flags with their current state
func (a *App) GetMapProgress(slotIndex int) ([]db.MapEntry, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	slot := &a.save.Slots[slotIndex]
	entries := db.GetAllMapEntries()

	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for i := range entries {
			enabled, err := db.GetEventFlag(flags, entries[i].ID)
			if err != nil {
				continue
			}
			entries[i].Enabled = enabled
		}
	}

	return entries, nil
}

// SetMapRegionFlags sets or clears both the visible and acquired flags for a map region.
// Visible flag IDs (62xxx) map to acquired flag IDs (63xxx) via +1000 offset.
func (a *App) SetMapRegionFlags(slotIndex int, visibleFlagID uint32, enabled bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	flags := slot.Data[slot.EventFlagsOffset:]

	// Set visible flag
	if err := db.SetEventFlag(flags, visibleFlagID, enabled); err != nil {
		return fmt.Errorf("failed to set visible flag %d: %w", visibleFlagID, err)
	}

	// Add/remove map fragment item in inventory
	if itemID, ok := data.MapFragmentItems[visibleFlagID]; ok {
		if enabled {
			_ = core.AddItemsToSlot(slot, []uint32{itemID}, 1, 0, false)
		} else {
			core.RemoveItemByBaseID(slot, itemID)
		}
	}

	return nil
}

// SetMapFlag sets or clears a single map flag
func (a *App) SetMapFlag(slotIndex int, flagID uint32, enabled bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	flags := slot.Data[slot.EventFlagsOffset:]
	if err := db.SetEventFlag(flags, flagID, enabled); err != nil {
		return fmt.Errorf("failed to set map flag %d: %w", flagID, err)
	}
	return nil
}

// RevealAllMap reveals all map regions (base game + DLC).
// Internally delegates to revealBaseMap and revealDLCMap to keep the logic separate.
func (a *App) RevealAllMap(slotIndex int) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	if err := revealBaseMap(slot); err != nil {
		return fmt.Errorf("base map: %w", err)
	}
	if err := revealDLCMap(slot); err != nil {
		return fmt.Errorf("DLC map: %w", err)
	}

	return nil
}

// revealBaseMap sets base game system flags, visible flags, and adds map fragment items.
func revealBaseMap(slot *core.SaveSlot) error {
	// Phase 1: Set all event flags (before any data shifts from AddItemsToSlot).
	flags := slot.Data[slot.EventFlagsOffset:]
	for id := range data.MapSystem {
		_ = db.SetEventFlag(flags, id, true)
	}
	var items []uint32
	for id := range data.MapVisible {
		if data.IsDLCMapFlag(id) {
			continue
		}
		_ = db.SetEventFlag(flags, id, true)
		if itemID, ok := data.MapFragmentItems[id]; ok {
			items = append(items, itemID)
		}
	}

	// Phase 2: Add map fragment items (shifts slot.Data — flags slice invalidated).
	for _, itemID := range items {
		_ = core.AddItemsToSlot(slot, []uint32{itemID}, 1, 0, false)
	}
	return nil
}

// revealDLCMap sets Shadow of the Erdtree flags, adds DLC map fragment items,
// and removes the DLC black tile overlay by writing discovery coordinates.
// See spec/29-dlc-black-tiles.md for details.
func revealDLCMap(slot *core.SaveSlot) error {
	// Phase 1: Set DLC event flags (before item insertion — offsets are stable).
	flags := slot.Data[slot.EventFlagsOffset:]

	// System flags for Shadow Realm display
	_ = db.SetEventFlag(flags, 62002, true) // Allow Shadow Realm Map Display
	_ = db.SetEventFlag(flags, 82002, true) // Show Shadow Realm Map

	var items []uint32
	for id := range data.MapVisible {
		if !data.IsDLCMapFlag(id) {
			continue
		}
		_ = db.SetEventFlag(flags, id, true)
		if itemID, ok := data.MapFragmentItems[id]; ok {
			items = append(items, itemID)
		}
	}

	// Phase 2: Add DLC map fragment items (shifts slot.Data).
	for _, itemID := range items {
		_ = core.AddItemsToSlot(slot, []uint32{itemID}, 1, 0, false)
	}

	// Phase 3: Remove DLC black tiles.
	// Write DLC-area coordinates into the BloodStain section so the game
	// treats the DLC map cover layer as discovered.
	storageEnd := slot.StorageBoxOffset + core.DynStorageBox
	gesturesOff := storageEnd + core.DynStorageToGestures
	if gesturesOff+4 > len(slot.Data) {
		return fmt.Errorf("gesturesOff 0x%X out of bounds", gesturesOff)
	}
	regCount := int(binary.LittleEndian.Uint32(slot.Data[gesturesOff : gesturesOff+4]))
	afterRegs := gesturesOff + 4 + regCount*4

	// Zero out the position data range
	for i := afterRegs + core.DLCTileZeroStart; i < afterRegs+core.DLCTileZeroEnd; i++ {
		slot.Data[i] = 0x00
	}
	// Record 1: DLC map center coordinates
	putF32(slot.Data, afterRegs+core.DLCTileRec1X, 9648.0)
	putF32(slot.Data, afterRegs+core.DLCTileRec1Y, 9124.0)
	slot.Data[afterRegs+core.DLCTileRec1Flag] = 0x01
	// Record 2: DLC area anchor coordinates
	putF32(slot.Data, afterRegs+core.DLCTileRec2X, 3037.0)
	putF32(slot.Data, afterRegs+core.DLCTileRec2Y, 1869.0)
	putF32(slot.Data, afterRegs+core.DLCTileRec2Z, 7880.0)
	putF32(slot.Data, afterRegs+core.DLCTileRec2W, 7803.0)
	slot.Data[afterRegs+core.DLCTileRec2Flag] = 0x01

	return nil
}

// putF32 writes a float32 value at the given offset in little-endian format.
func putF32(d []byte, off int, v float32) {
	binary.LittleEndian.PutUint32(d[off:], math.Float32bits(v))
}

// ResetMapExploration clears all map visibility, acquisition, and POI discovery flags
func (a *App) ResetMapExploration(slotIndex int) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	flags := slot.Data[slot.EventFlagsOffset:]

	// Clear visible flags + remove map fragment items
	for id := range data.MapVisible {
		_ = db.SetEventFlag(flags, id, false)
		if itemID, ok := data.MapFragmentItems[id]; ok {
			core.RemoveItemByBaseID(slot, itemID)
		}
	}
	// Clear acquired flags
	for id := range data.MapAcquired {
		_ = db.SetEventFlag(flags, id, false)
	}
	// Clear unsafe sub-region flags
	for id := range data.MapUnsafe {
		_ = db.SetEventFlag(flags, id, false)
	}
	// Note: system flags (62000, 62001, 82001, 82002) are preserved

	return nil
}

// RemoveFogOfWar fills the exploration bitfield with 0xFF, removing all Fog of War.
// See spec/27-map-reveal.md §4 for details.
func (a *App) RemoveFogOfWar(slotIndex int) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.pushUndo(slotIndex)

	slot := &a.save.Slots[slotIndex]
	storageEnd := slot.StorageBoxOffset + core.DynStorageBox
	gesturesOff := storageEnd + core.DynStorageToGestures

	if gesturesOff+4 > len(slot.Data) {
		return fmt.Errorf("gesturesOff 0x%X out of bounds", gesturesOff)
	}

	regCount := int(binary.LittleEndian.Uint32(slot.Data[gesturesOff : gesturesOff+4]))
	afterRegs := gesturesOff + 4 + regCount*4

	fowStart := afterRegs + 0x087E
	fowEnd := afterRegs + 0x10B0

	if fowEnd >= len(slot.Data)-0x80 {
		return fmt.Errorf("FoW bitfield range out of bounds (0x%X)", fowEnd)
	}

	for i := fowStart; i <= fowEnd; i++ {
		slot.Data[i] = 0xFF
	}

	return nil
}

// ImportCharacter copies a slot from the source save file to the destination save file
func (a *App) ImportCharacter(srcIdx, destIdx int) error {
	return fmt.Errorf("ImportCharacter is temporarily disabled during architecture refactor")
}

// CloneSlot copies an existing character slot to an empty destination slot within the same save.
func (a *App) CloneSlot(srcIdx, destIdx int) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if srcIdx < 0 || srcIdx >= 10 || destIdx < 0 || destIdx >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	if srcIdx == destIdx {
		return fmt.Errorf("source and destination must differ")
	}
	srcName := core.UTF16ToString(a.save.Slots[srcIdx].Player.CharacterName[:])
	if srcName == "" {
		return fmt.Errorf("source slot %d is empty", srcIdx)
	}
	destName := core.UTF16ToString(a.save.Slots[destIdx].Player.CharacterName[:])
	if destName != "" {
		return fmt.Errorf("destination slot %d is not empty", destIdx)
	}

	a.pushUndo(destIdx)

	src := a.save.Slots[srcIdx]

	// Deep copy Data
	newData := make([]byte, len(src.Data))
	copy(newData, src.Data)
	src.Data = newData

	// Deep copy GaMap
	newGaMap := make(map[uint32]uint32, len(src.GaMap))
	for k, v := range src.GaMap {
		newGaMap[k] = v
	}
	src.GaMap = newGaMap

	a.save.Slots[destIdx] = src
	a.save.ActiveSlots[destIdx] = true
	a.save.ProfileSummaries[destIdx] = a.save.ProfileSummaries[srcIdx]

	return nil
}

// DeleteSlot removes a character from a slot and shifts all subsequent slots down by one.
func (a *App) DeleteSlot(idx int) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if idx < 0 || idx >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	name := core.UTF16ToString(a.save.Slots[idx].Player.CharacterName[:])
	if name == "" {
		return fmt.Errorf("slot %d is already empty", idx)
	}

	// Snapshot all affected slots (idx..9) since delete shifts them down
	for i := idx; i < 10; i++ {
		a.pushUndo(i)
	}

	for i := idx; i < 9; i++ {
		a.save.Slots[i] = a.save.Slots[i+1]
		a.save.ActiveSlots[i] = a.save.ActiveSlots[i+1]
		a.save.ProfileSummaries[i] = a.save.ProfileSummaries[i+1]
	}

	// Zero out the last slot with a valid MagicOffset to prevent Write() from panicking
	a.save.Slots[9] = core.SaveSlot{
		Data:        make([]byte, 0x280000),
		GaMap:       make(map[uint32]uint32),
		MagicOffset: 0x15420 + 432,
	}
	a.save.ActiveSlots[9] = false
	a.save.ProfileSummaries[9] = core.ProfileSummary{}

	return nil
}

// GetActiveSlots returns the activity status of all 10 slots
func (a *App) GetActiveSlots() []bool {
	active := make([]bool, 10)
	if a.save == nil {
		return active
	}
	for i := 0; i < 10; i++ {
		// Slot is active if it has a name (Python method)
		name := core.UTF16ToString(a.save.Slots[i].Player.CharacterName[:])
		active[i] = name != ""
	}
	return active
}

// GetCharacterNames returns the names of all 10 characters
func (a *App) GetCharacterNames() []string {
	names := make([]string, 10)
	if a.save == nil {
		for i := 0; i < 10; i++ {
			names[i] = "Empty Slot"
		}
		return names
	}
	for i := 0; i < 10; i++ {
		// Get name directly from the character slot (Python method)
		name := core.UTF16ToString(a.save.Slots[i].Player.CharacterName[:])
		if name == "" {
			names[i] = "Empty Slot"
		} else {
			names[i] = name
		}
	}
	return names
}

// GetSourceActiveSlots returns the activity status of all 10 slots in the source file
func (a *App) GetSourceActiveSlots() []bool {
	active := make([]bool, 10)
	if a.sourceSave == nil {
		return active
	}
	for i := 0; i < 10; i++ {
		name := core.UTF16ToString(a.sourceSave.Slots[i].Player.CharacterName[:])
		active[i] = name != ""
	}
	return active
}

// SetSlotActivity toggles a slot's active status
func (a *App) SetSlotActivity(index int, active bool) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	a.save.ActiveSlots[index] = active
	return nil
}

// GetSteamID returns the global SteamID from UserData10
func (a *App) GetSteamID() uint64 {
	if a.save == nil {
		return 0
	}
	return a.save.SteamID
}

// SetSteamID updates the global SteamID
func (a *App) SetSteamID(id uint64) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	a.save.SteamID = id
	return nil
}

// GetSteamIDString returns the SteamID as a decimal string to avoid JS float64 precision loss.
func (a *App) GetSteamIDString() string {
	if a.save == nil {
		return ""
	}
	return strconv.FormatUint(a.save.SteamID, 10)
}

// SetSteamIDFromString parses a decimal string and updates the SteamID.
func (a *App) SetSteamIDFromString(s string) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	id, err := strconv.ParseUint(s, 10, 64)
	if err != nil {
		return fmt.Errorf("invalid SteamID: %w", err)
	}
	a.save.SteamID = id
	return nil
}

// pushUndo takes a deep-copy snapshot of the given slot and pushes it onto the undo stack.
func (a *App) pushUndo(idx int) {
	slot := &a.save.Slots[idx]

	// Deep copy Data
	dataCopy := make([]byte, len(slot.Data))
	copy(dataCopy, slot.Data)

	// Deep copy GaMap
	gaMapCopy := make(map[uint32]uint32, len(slot.GaMap))
	for k, v := range slot.GaMap {
		gaMapCopy[k] = v
	}

	// Deep copy GaItems
	var gaItemsCopy []core.GaItemFull
	if slot.GaItems != nil {
		gaItemsCopy = make([]core.GaItemFull, len(slot.GaItems))
		copy(gaItemsCopy, slot.GaItems)
	}

	snap := slotSnapshot{
		Data:              dataCopy,
		Version:           slot.Version,
		Player:            slot.Player,
		GaMap:             gaMapCopy,
		GaItems:           gaItemsCopy,
		Inventory:         slot.Inventory.Clone(),
		Storage:           slot.Storage.Clone(),
		Warnings:          append([]string{}, slot.Warnings...),
		MagicOffset:       slot.MagicOffset,
		InventoryEnd:      slot.InventoryEnd,
		EventFlagsOffset:  slot.EventFlagsOffset,
		PlayerDataOffset:  slot.PlayerDataOffset,
		FaceDataOffset:    slot.FaceDataOffset,
		StorageBoxOffset:  slot.StorageBoxOffset,
		IngameTimerOffset: slot.IngameTimerOffset,
		GaItemDataOffset:  slot.GaItemDataOffset,
		NextAoWIndex:      slot.NextAoWIndex,
		NextArmamentIndex: slot.NextArmamentIndex,
		NextGaItemHandle:  slot.NextGaItemHandle,
		PartGaItemHandle:  slot.PartGaItemHandle,
	}

	stack := a.undoStacks[idx]
	if len(stack) >= maxUndoDepth {
		stack = stack[1:] // drop oldest
	}
	a.undoStacks[idx] = append(stack, snap)
}

// RevertSlot pops the last snapshot from the undo stack and restores the slot.
func (a *App) RevertSlot(idx int) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if idx < 0 || idx >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	stack := a.undoStacks[idx]
	if len(stack) == 0 {
		return fmt.Errorf("nothing to undo for slot %d", idx)
	}

	snap := stack[len(stack)-1]
	a.undoStacks[idx] = stack[:len(stack)-1]

	slot := &a.save.Slots[idx]
	slot.Data = snap.Data
	slot.Version = snap.Version
	slot.Player = snap.Player
	slot.GaMap = snap.GaMap
	slot.GaItems = snap.GaItems
	slot.Inventory = snap.Inventory
	slot.Storage = snap.Storage
	slot.Warnings = snap.Warnings
	slot.MagicOffset = snap.MagicOffset
	slot.InventoryEnd = snap.InventoryEnd
	slot.EventFlagsOffset = snap.EventFlagsOffset
	slot.PlayerDataOffset = snap.PlayerDataOffset
	slot.FaceDataOffset = snap.FaceDataOffset
	slot.StorageBoxOffset = snap.StorageBoxOffset
	slot.IngameTimerOffset = snap.IngameTimerOffset
	slot.GaItemDataOffset = snap.GaItemDataOffset
	slot.NextAoWIndex = snap.NextAoWIndex
	slot.NextArmamentIndex = snap.NextArmamentIndex
	slot.NextGaItemHandle = snap.NextGaItemHandle
	slot.PartGaItemHandle = snap.PartGaItemHandle

	return nil
}

// GetUndoDepth returns the number of undo snapshots available for a slot.
func (a *App) GetUndoDepth(idx int) int {
	if a.save == nil || idx < 0 || idx >= 10 {
		return 0
	}
	return len(a.undoStacks[idx])
}

// clearAllUndoStacks resets all undo history (called on file load/save).
func (a *App) clearAllUndoStacks() {
	for i := range a.undoStacks {
		a.undoStacks[i] = nil
	}
}

// ---------- 21.4 Save file diffing ----------

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

// SlotCapacity reports used vs max counts for GaItems, Inventory, and Storage.
type SlotCapacity struct {
	GaItemsUsed   int `json:"gaItemsUsed"`
	GaItemsMax    int `json:"gaItemsMax"`
	InventoryUsed int `json:"inventoryUsed"`
	InventoryMax  int `json:"inventoryMax"`
	StorageUsed   int `json:"storageUsed"`
	StorageMax    int `json:"storageMax"`
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

// ---------- Deploy ----------

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
	a.clearAllUndoStacks()
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

// PresetInfo is the frontend-facing summary of an appearance preset.
type PresetInfo struct {
	Name     string `json:"name"`
	Image    string `json:"image"`    // filename in presets/ dir (e.g. "geralt.jpg")
	BodyType string `json:"bodyType"` // "Type A" or "Type B"
}

// ListAppearancePresets returns the list of available character appearance presets.
func (a *App) ListAppearancePresets() []PresetInfo {
	result := make([]PresetInfo, len(data.Presets))
	for i, p := range data.Presets {
		bt := "Type A"
		if p.BodyType == 0 {
			bt = "Type B"
		}
		result[i] = PresetInfo{Name: p.Name, Image: p.Image, BodyType: bt}
	}
	return result
}

// FavoriteSlotInfo describes a single Favorites slot in the Mirror.
type FavoriteSlotInfo struct {
	Index  int    `json:"index"`  // absolute slot index (0-14)
	Active bool   `json:"active"` // true if slot has FACE magic
	Safe   bool   `json:"safe"`   // true if not colliding with ProfileSummary
	Name   string `json:"name"`   // preset name if we wrote it, empty otherwise
}

// GetFavoritesStatus returns the state of all 15 Favorites slots.
func (a *App) GetFavoritesStatus() []FavoriteSlotInfo {
	result := make([]FavoriteSlotInfo, core.FavSlotCount)
	if a.save == nil {
		return result
	}

	ud := a.save.UserData10.Data
	safeSet := make(map[int]bool, len(core.FavSafeSlots))
	for _, s := range core.FavSafeSlots {
		safeSet[s] = true
	}

	for i := 0; i < core.FavSlotCount; i++ {
		off := core.FavBaseOffset + i*core.FavSlotSize
		active := off+0x1C <= len(ud) && string(ud[off+0x18:off+0x1C]) == "FACE"
		result[i] = FavoriteSlotInfo{
			Index:  i,
			Active: active,
			Safe:   safeSet[i],
		}
	}
	return result
}

// RemoveFavoritePreset clears a Favorites slot in UserData10.
func (a *App) RemoveFavoritePreset(slotIndex int) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= core.FavSlotCount {
		return fmt.Errorf("invalid favorites slot index")
	}

	ud := a.save.UserData10.Data
	off := core.FavBaseOffset + slotIndex*core.FavSlotSize
	if off+core.FavSlotSize > len(ud) {
		return fmt.Errorf("slot offset out of bounds")
	}

	// Zero out the entire slot
	for i := 0; i < core.FavSlotSize; i++ {
		ud[off+i] = 0
	}
	return nil
}

// ApplyMirrorFavoriteToCharacter copies the appearance from a Mirror Favorites slot
// onto a character's FaceData blob, replicating the in-game "Apply" action.
//
// Algorithm (per spec/31):
//   - Model IDs (32 B), face shape (64 B), body proportions (7 B), and skin & cosmetics
//     (91 B) are copied verbatim from the preset slot to the character's FaceData blob.
//   - The unk0x6c block (64 B at FaceData offset 0x70) is preserved; the game does NOT
//     overwrite it on apply.
//   - slot.Player.Gender is set from preset body_type (Mirror stores body_type inverted
//     from the slot's Gender field: body_type=0 → Gender=1 male, body_type=1 → Gender=0 female).
//   - Trailing flags at FaceData offset 0x12D..0x12E are zeroed (game does this on apply).
//
// Equipment handles are NOT cleared. The game zeroes gender-specific equipment to avoid
// model mismatches; we leave gear intact and let the user decide.
func (a *App) ApplyMirrorFavoriteToCharacter(charIndex, mirrorSlotIndex int) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIndex < 0 || charIndex >= 10 {
		return fmt.Errorf("invalid character index")
	}
	if mirrorSlotIndex < 0 || mirrorSlotIndex >= core.FavSlotCount {
		return fmt.Errorf("invalid mirror slot index")
	}

	ud := a.save.UserData10.Data
	mirrorOff := core.FavBaseOffset + mirrorSlotIndex*core.FavSlotSize
	if mirrorOff+core.FavSlotSize > len(ud) {
		return fmt.Errorf("mirror slot offset out of bounds")
	}
	if string(ud[mirrorOff+core.FavOffMagic:mirrorOff+core.FavOffMagic+4]) != "FACE" {
		return fmt.Errorf("mirror slot %d is empty", mirrorSlotIndex)
	}

	slot := &a.save.Slots[charIndex]
	fd := slot.FaceDataStart()
	if fd < 0 || fd+core.FaceDataBlobSize > len(slot.Data) {
		return fmt.Errorf("FaceData blob out of bounds: start=0x%X", fd)
	}

	a.pushUndo(charIndex)

	// Model IDs (32 B): preset[0x24..0x44] → slot[fd+0x10..0x30]
	copy(slot.Data[fd+core.FDOffFaceModel:fd+core.FDOffFaceModel+32],
		ud[mirrorOff+core.FavOffModelIDs:mirrorOff+core.FavOffModelIDs+32])

	// FaceShape (64 B): preset[0x44..0x84] → slot[fd+0x30..0x70]
	copy(slot.Data[fd+core.FDOffFaceShape:fd+core.FDOffFaceShape+64],
		ud[mirrorOff+core.FavOffFaceShape:mirrorOff+core.FavOffFaceShape+64])

	// PRESERVE unk0x6c at slot[fd+0x70..0xB0] — game does NOT touch this.

	// Body proportions (7 B): preset[0xC4..0xCB] → slot[fd+0xB0..0xB7]
	copy(slot.Data[fd+core.FDOffHead:fd+core.FDOffHead+7],
		ud[mirrorOff+core.FavOffBody:mirrorOff+core.FavOffBody+7])

	// Skin & cosmetics (91 B): preset[0xCB..0x126] → slot[fd+0xB7..0x112]
	copy(slot.Data[fd+core.FDOffSkinR:fd+core.FDOffSkinR+91],
		ud[mirrorOff+core.FavOffSkin:mirrorOff+core.FavOffSkin+91])

	// Trailing flags: observed game behavior is to zero bytes 0x125 and 0x126 on apply
	// (byte 0x124 stays 0x01). Semantics unknown — see tmp/re-character/facedata_dump.txt.
	slot.Data[fd+0x125] = 0
	slot.Data[fd+0x126] = 0

	// Gender flip — Mirror body_type is INVERTED relative to slot.Gender.
	if ud[mirrorOff+core.FavOffBodyType] == 0 {
		slot.Player.Gender = 1 // male
	} else {
		slot.Player.Gender = 0 // female
	}

	return nil
}

// WriteSelectedToFavorites writes selected presets to the next available safe Favorites slots.
// Returns the number of presets written.
func (a *App) WriteSelectedToFavorites(charIndex int, presetNames []string) (int, error) {
	if a.save == nil {
		return 0, fmt.Errorf("no save loaded")
	}
	if charIndex < 0 || charIndex >= 10 {
		return 0, fmt.Errorf("invalid character index")
	}
	if len(presetNames) == 0 {
		return 0, nil
	}

	ud := a.save.UserData10.Data
	slot := &a.save.Slots[charIndex]

	// Find available safe slots
	var freeSlots []int
	for _, s := range core.FavSafeSlots {
		off := core.FavBaseOffset + s*core.FavSlotSize
		if off+0x1C > len(ud) {
			continue
		}
		magic := string(ud[off+0x18 : off+0x1C])
		if magic != "FACE" {
			freeSlots = append(freeSlots, s)
		}
	}

	if len(presetNames) > len(freeSlots) {
		return 0, fmt.Errorf("not enough free slots: need %d, have %d", len(presetNames), len(freeSlots))
	}

	// Read unknown block from character slot
	var unkBlock [64]byte
	fd := slot.FaceDataStart()
	if fd >= 0 && fd+core.FaceDataBlobSize <= len(slot.Data) {
		copy(unkBlock[:], slot.Data[fd+core.FDOffUnknownBlock:fd+core.FDOffUnknownBlock+64])
	}

	written := 0
	for i, name := range presetNames {
		var preset *data.AppearancePreset
		for j := range data.Presets {
			if data.Presets[j].Name == name {
				preset = &data.Presets[j]
				break
			}
		}
		if preset == nil {
			continue
		}

		safeIdx := freeSlots[i]
		slotOff := core.FavBaseOffset + safeIdx*core.FavSlotSize

		buf := make([]byte, core.FavSlotSize)

		// Slot header
		binary.LittleEndian.PutUint16(buf[0x00:], 0xFACE)
		binary.LittleEndian.PutUint32(buf[0x04:], core.FavHeaderUnk)
		buf[core.FavOffBodyFlag] = 1
		if preset.BodyType == 1 {
			buf[core.FavOffBodyType] = 0 // male in Favorites
		} else {
			buf[core.FavOffBodyType] = 1 // female in Favorites
		}

		// FACE block header
		copy(buf[core.FavOffMagic:], []byte("FACE"))
		binary.LittleEndian.PutUint32(buf[core.FavOffAlignment:], 4)
		binary.LittleEndian.PutUint32(buf[core.FavOffInnerSize:], 0x120)

		// Model IDs — male only, female skipped (no mapping)
		if preset.BodyType == 1 {
			writeModel := func(off int, uiVal uint8) {
				val := uiVal
				if val > 0 {
					val--
				}
				binary.LittleEndian.PutUint32(buf[core.FavOffModelIDs+(off*4):], uint32(val))
			}
			// Hair: lookup table first, fallback to UI-1 for unmapped styles
			if partsId, ok := data.LookupMaleHairPartsID(preset.HairModel); ok {
				binary.LittleEndian.PutUint32(buf[core.FavOffModelIDs+1*4:], uint32(partsId))
			} else if preset.HairModel > 0 {
				binary.LittleEndian.PutUint32(buf[core.FavOffModelIDs+1*4:], uint32(preset.HairModel-1))
			}
			writeModel(2, preset.EyeModel)
			writeModel(3, preset.EyebrowModel)
			writeModel(4, preset.BeardModel)
			writeModel(5, preset.EyepatchModel)
			writeModel(6, preset.DecalModel)
			writeModel(7, preset.EyelashModel)
		}

		copy(buf[core.FavOffFaceShape:], preset.FaceShape[:])
		copy(buf[core.FavOffUnkBlock:], unkBlock[:])
		copy(buf[core.FavOffBody:], preset.Body[:])
		copy(buf[core.FavOffSkin:], preset.Skin[:])

		copy(ud[slotOff:], buf)
		written++
	}

	return written, nil
}

// GetNetworkParams reads the current invasion matchmaking parameters from the save's regulation.
func (a *App) GetNetworkParams() (*core.NetworkParamValues, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if len(a.save.UserData11) == 0 {
		return nil, fmt.Errorf("save has no UserData11 (regulation)")
	}
	return core.ReadNetworkParams(a.save.UserData11)
}

// SetNetworkParams patches the invasion matchmaking parameters in the save's regulation.
func (a *App) SetNetworkParams(params core.NetworkParamValues) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if len(a.save.UserData11) == 0 {
		return fmt.Errorf("save has no UserData11 (regulation)")
	}

	patched, err := core.PatchNetworkParams(a.save.UserData11, params)
	if err != nil {
		return fmt.Errorf("patch network params: %w", err)
	}
	a.save.UserData11 = patched
	return nil
}

// ResetNetworkParams restores vanilla invasion matchmaking parameters.
func (a *App) ResetNetworkParams() error {
	return a.SetNetworkParams(core.NetworkParamDefaults())
}

// Dummy method to force Wails to export types
func (a *App) _forceExportTypes() (db.GraceEntry, db.BossEntry, db.ItemEntry, db.MapEntry, db.CookbookEntry, db.GestureEntry, db.QuestNPC, db.QuestStep, db.QuestFlagState, core.SlotDiagnostics, core.DiagnosticIssue, DiffEntry, SlotDiffSummary, SlotCapacity, deploy.Target, PresetInfo, FavoriteSlotInfo, db.BellBearingEntry, db.WhetbladeEntry, db.AshOfWarFlagEntry, core.NetworkParamValues) {
	return db.GraceEntry{}, db.BossEntry{}, db.ItemEntry{}, db.MapEntry{}, db.CookbookEntry{}, db.GestureEntry{}, db.QuestNPC{}, db.QuestStep{}, db.QuestFlagState{}, core.SlotDiagnostics{}, core.DiagnosticIssue{}, DiffEntry{}, SlotDiffSummary{}, SlotCapacity{}, deploy.Target{}, PresetInfo{}, FavoriteSlotInfo{}, db.BellBearingEntry{}, db.WhetbladeEntry{}, db.AshOfWarFlagEntry{}, core.NetworkParamValues{}
}
