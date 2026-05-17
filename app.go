package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strconv"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
	"github.com/oisis/EldenRing-SaveForge/backend/deploy"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
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
	GaItemDataOffset      int
	TutorialDataOffset    int
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
	favSlotNames map[int]string // preset name written to each Favorites slot; empty = loaded from save (unknown)

	// Phase 1 inventory edit session state. One active session per character.
	// editSessions is keyed by session ID; editSessionByChar maps charIdx → ID
	// so callers can look up the current session for a character without
	// scanning. Sessions are pure RAM — never persisted, never carry mutations
	// in Phase 1.
	editSessions      map[string]*editor.InventoryEditSession
	editSessionByChar map[int]string
}

// NewApp creates a new App struct
func NewApp() *App {
	return &App{
		favSlotNames:      make(map[int]string),
		editSessions:      make(map[string]*editor.InventoryEditSession),
		editSessionByChar: make(map[int]string),
	}
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
	a.favSlotNames = make(map[int]string)
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

	slot := &a.save.Slots[index]

	if err := a.applyMemoryStonesToSlot(slot, charVM.MemoryStones); err != nil {
		return err
	}

	// Flush slot.Player → slot.Data so that subsequent operations
	// (AddItemsToSlotBatch, RebuildSlotFull) that re-parse slot.Data
	// see the correct stats instead of the pre-edit binary values.
	slot.SyncPlayerToData()

	// 2. Sync NG+ event flags (50-57) with ClearCount
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

// GetItemListChunk is the progressive-load alias for GetItemList.
func (a *App) GetItemListChunk(category string) []db.ItemEntry {
	return a.GetItemList(category)
}

// GetItemDetail returns full item data (description, stats) for a single base item ID.
func (a *App) GetItemDetail(baseId uint32) *db.ItemEntry {
	return db.GetItemEntryByID(baseId)
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

// weaponCategorySupportsInfusion returns true for categories whose weapons can
// receive affinities. Ranged weapons (bows, crossbows, greatbows) and catalysts
// (staves, seals) cannot be infused in Elden Ring — applying an infuse offset to
// their ID produces an ID the game does not recognise, making the item invisible.
func weaponCategorySupportsInfusion(category string) bool {
	return category == "melee_armaments" || category == "shields"
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

	// PRE-FLIGHT: refuse to mutate a save that already has duplicate inventory
	// acquisition indices. Without this guard the batch mutates the slot, then the
	// post-mutation validator catches the pre-existing duplicate and rolls back —
	// surfacing a misleading "post-mutation validation failed" error to the user.
	// No mutation, no snapshot needed; the slot is untouched on error.
	if dups := core.ScanDuplicateInventoryIndices(slot); len(dups) > 0 {
		d := dups[0]
		return result, fmt.Errorf(
			"save inventory has %d duplicate acquisition index issue(s) before adding items: duplicate Index %d in %s (handle 0x%08X, also at row %d handle 0x%08X). Run inventory index repair before adding items",
			len(dups), d.Index, d.Scope, d.DuplicateHandle, d.FirstRow, d.FirstHandle,
		)
	}

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
			if infuseOffset != 0 && weaponCategorySupportsInfusion(itemData.Category) {
				finalID = id + uint32(infuseOffset) + uint32(upgrade25)
			} else {
				finalID = id + uint32(upgrade25)
			}
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
		if flagList, ok := data.BolsteringPickupFlags[p.baseID]; ok {
			if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
				flags := slot.Data[slot.EventFlagsOffset:]
				sorted := make([]uint32, len(flagList))
				copy(sorted, flagList)
				sort.Slice(sorted, func(i, j int) bool { return sorted[i] < sorted[j] })
				qty := p.actualInv + p.actualStorage
				set := 0
				for _, f := range sorted {
					if set >= qty {
						break
					}
					if val, err := db.GetEventFlag(flags, f); err == nil && !val {
						if err := db.SetEventFlag(flags, f, true); err != nil {
							runtime.LogWarningf(a.ctx, "bolstering pickup flag %d: %v", f, err)
						} else {
							set++
						}
					}
				}
			}
		}
		if tutorialID, ok := data.AboutTutorialID[p.baseID]; ok {
			if err := core.AppendTutorialID(slot, tutorialID); err != nil {
				runtime.LogWarningf(a.ctx, "tutorial ID %d: %v", tutorialID, err)
			}
		}
		if companions := data.CompanionEventFlagsForItem(p.baseID); len(companions) > 0 {
			if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
				eflags := slot.Data[slot.EventFlagsOffset:]
				for _, f := range companions {
					if err := db.SetEventFlag(eflags, f, true); err != nil {
						runtime.LogWarningf(a.ctx, "companion flag %d for item 0x%08X: %v", f, p.baseID, err)
					}
				}
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

	// Pre-scan: collect item IDs with companion flags being removed.
	companionRemovals := make(map[uint32]bool)
	for _, handle := range handles {
		if itemID, ok := slot.GaMap[handle]; ok {
			if len(data.CompanionEventFlagsForItem(itemID)) > 0 {
				companionRemovals[itemID] = true
			}
		}
	}

	// Count removals per bolstering material baseID to restore pickup flags.
	bolsteringRemovals := make(map[uint32]int)
	for _, handle := range handles {
		if itemID, ok := slot.GaMap[handle]; ok {
			if _, isBolstering := data.BolsteringPickupFlags[itemID]; isBolstering {
				bolsteringRemovals[itemID]++
			}
		}
	}

	for _, handle := range handles {
		if err := core.RemoveItemFromSlot(slot, handle, fromInventory, fromStorage); err != nil {
			return err
		}
	}

	// Restore pickup flags for removed bolstering materials (highest flag ID first).
	if len(bolsteringRemovals) > 0 && slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags := slot.Data[slot.EventFlagsOffset:]
		for itemID, qty := range bolsteringRemovals {
			flagList := data.BolsteringPickupFlags[itemID]
			sorted := make([]uint32, len(flagList))
			copy(sorted, flagList)
			sort.Slice(sorted, func(i, j int) bool { return sorted[i] > sorted[j] })
			restored := 0
			for _, f := range sorted {
				if restored >= qty {
					break
				}
				if val, err := db.GetEventFlag(flags, f); err == nil && val {
					if err := db.SetEventFlag(flags, f, false); err == nil {
						restored++
					}
				}
			}
		}
	}

	// Clear companion flags for items no longer present in slot after removal.
	if len(companionRemovals) > 0 && slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		eflags := slot.Data[slot.EventFlagsOffset:]
		for itemID := range companionRemovals {
			remaining := false
			for _, g := range slot.GaItems {
				if !g.IsEmpty() && g.ItemID == itemID {
					remaining = true
					break
				}
			}
			if !remaining {
				for _, f := range data.CompanionEventFlagsForItem(itemID) {
					if err := db.SetEventFlag(eflags, f, false); err != nil {
						runtime.LogWarningf(a.ctx, "clear companion flag %d for item 0x%08X: %v", f, itemID, err)
					}
				}
			}
		}
	}

	return nil
}

// MoveItemsBetweenInventoryAndStorage relocates inventory records between
// CommonItems Inventory and Storage for the given character slot. The direction
// string must be "to-storage" (Inventory → Storage) or "to-inventory"
// (Storage → Inventory). Returns per-handle outcome; invalid handles are
// reported in Skipped, not raised as errors.
//
// Equipped items (handles referenced by ChrAsmEquipment) are skipped with
// reason "equipped" only for the to-storage direction. Other skip reasons:
// "not_found", "dest_full", "invalid_handle".
//
// Undo snapshot is pushed only when at least one handle was actually moved.
func (a *App) MoveItemsBetweenInventoryAndStorage(charIdx int, handles []uint32, direction string) (core.TransferResult, error) {
	empty := core.TransferResult{}
	if a.save == nil {
		return empty, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return empty, fmt.Errorf("invalid character index %d", charIdx)
	}
	var dir core.TransferDirection
	switch direction {
	case "to-storage":
		dir = core.TransferToStorage
	case "to-inventory":
		dir = core.TransferToInventory
	default:
		return empty, fmt.Errorf("invalid direction %q (expected \"to-storage\" or \"to-inventory\")", direction)
	}

	slot := &a.save.Slots[charIdx]
	if slot.Version == 0 {
		return empty, fmt.Errorf("slot %d is empty", charIdx)
	}

	// Snapshot for undo. pushUndo only if we will actually mutate state — peek
	// first by running a dry-check on whether at least one handle resolves to a
	// real source record. We push undo before the real call so a partially-
	// failed batch is still recoverable.
	willMutate := false
	for _, h := range handles {
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		var srcStart, slots int
		if dir == core.TransferToStorage {
			srcStart = slot.MagicOffset + core.InvStartFromMagic
			slots = core.CommonItemCount
		} else {
			srcStart = slot.StorageBoxOffset + core.StorageHeaderSkip
			slots = core.StorageCommonCount
		}
		if srcStart <= 0 {
			continue
		}
		for i := 0; i < slots; i++ {
			off := srcStart + i*core.InvRecordLen
			if off+core.InvRecordLen > len(slot.Data) {
				break
			}
			if binary.LittleEndian.Uint32(slot.Data[off:]) == h {
				willMutate = true
				break
			}
		}
		if willMutate {
			break
		}
	}
	if willMutate {
		a.pushUndo(charIdx)
	}

	// Resolve per-handle destination caps for quantity-merge items only
	// (goods 0xB0). Instance-move handles (weapons 0x80, armor 0x90,
	// talismans 0xA0, AoW 0xC0) use physical relocation and never consult
	// the cap; including their MaxInventory=1 in the map would be harmless
	// but is omitted to keep the contract explicit — for those handles, the
	// caps map is intentionally empty.
	caps := make(map[uint32]uint32, len(handles))
	for _, h := range handles {
		if h == core.GaHandleEmpty || h == core.GaHandleInvalid {
			continue
		}
		if h&core.GaHandleTypeMask != core.ItemTypeItem {
			continue
		}
		itemID, ok := slot.GaMap[h]
		if !ok {
			// Goods use handle = itemID|prefix; recover the DB-compatible
			// item ID when GaMap is missing the entry.
			itemID = db.HandleToItemID(h)
		}
		itemData, _ := db.GetItemDataFuzzy(itemID)
		var cap uint32
		if dir == core.TransferToStorage {
			cap = itemData.MaxStorage
		} else {
			cap = itemData.MaxInventory
		}
		caps[h] = cap
	}

	return core.MoveItemsBetweenContainers(slot, handles, dir, &core.TransferOptions{DestCaps: caps})
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

func (a *App) GetStartingClasses() []db.ClassStats {
	return db.GetAllClassStats()
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
		GaItemDataOffset:   slot.GaItemDataOffset,
		TutorialDataOffset: slot.TutorialDataOffset,
		NextAoWIndex:       slot.NextAoWIndex,
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
	slot.TutorialDataOffset = snap.TutorialDataOffset
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

// ResetNetworkParams restores vanilla defaults for all network parameters.
func (a *App) ResetNetworkParams() error {
	return a.SetNetworkParams(core.NetworkParamDefaults())
}

// GetNetworkPreset returns preset values by name without applying them.
func (a *App) GetNetworkPreset(name string) (*core.NetworkParamValues, error) {
	var p core.NetworkParamValues
	switch name {
	case "fast-invasions":
		p = core.NetworkParamFastInvasions()
	case "light-invasions":
		p = core.NetworkParamLightInvasions()
	case "fast-summons":
		p = core.NetworkParamFastSummons()
	case "fast-blue":
		p = core.NetworkParamFastBlue()
	case "aggressive-host":
		p = core.NetworkParamAggressiveHost()
	case "defaults":
		p = core.NetworkParamDefaults()
	default:
		return nil, fmt.Errorf("unknown preset: %s", name)
	}
	return &p, nil
}


// ApplyWeaponInfusion changes the infusion (affinity) of a specific weapon instance
// identified by its GaItemHandle. Only the weapon's ItemID is patched — upgrade level,
// AoW gem, quantity, and location are preserved.
//
// expectedCurrentItemID is a stale-data guard: the request is rejected if the weapon's
// ItemID has already changed since the frontend loaded the character data.
// newItemID must encode the same base weapon and the same upgrade level; only the
// infusion offset portion may differ.
func (a *App) ApplyWeaponInfusion(charIdx int, handle uint32, expectedCurrentItemID, newItemID uint32) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return fmt.Errorf("invalid character index %d", charIdx)
	}

	// Both IDs must be in the weapon range (upper nibble 0x0).
	if expectedCurrentItemID>>28 != 0 {
		return fmt.Errorf("expectedCurrentItemID 0x%08X is not a weapon ID", expectedCurrentItemID)
	}
	if newItemID>>28 != 0 {
		return fmt.Errorf("newItemID 0x%08X is not a weapon ID", newItemID)
	}

	// Resolve base weapon from expectedCurrentItemID. DB lookup is authoritative —
	// do not trust the frontend's baseId / maxUpgrade values.
	baseData, baseID := db.GetItemDataFuzzy(expectedCurrentItemID)
	if baseData.Name == "" {
		return fmt.Errorf("unknown weapon 0x%08X", expectedCurrentItemID)
	}
	if baseData.MaxUpgrade != 25 {
		return fmt.Errorf("weapon %q (0x%08X) is not infusable (maxUpgrade=%d)", baseData.Name, baseID, baseData.MaxUpgrade)
	}
	if !weaponCategorySupportsInfusion(baseData.Category) {
		return fmt.Errorf("weapon category %q does not support infusion", baseData.Category)
	}

	expectedDiff := expectedCurrentItemID - baseID
	expectedUpgrade := expectedDiff % 100

	// Validate newItemID resolves to the same base weapon and same upgrade level.
	_, newBaseID := db.GetItemDataFuzzy(newItemID)
	if newBaseID != baseID {
		return fmt.Errorf("newItemID 0x%08X resolves to a different base weapon (got 0x%08X, expected 0x%08X)",
			newItemID, newBaseID, baseID)
	}
	newDiff := newItemID - baseID
	newUpgrade := newDiff % 100
	if newUpgrade != expectedUpgrade {
		return fmt.Errorf("newItemID 0x%08X changes the upgrade level (%d→%d); only the infusion offset may change",
			newItemID, expectedUpgrade, newUpgrade)
	}

	// Validate the resulting infusion offset is one of the known types.
	newInfuseOffset := int(newDiff - newUpgrade)
	validOffset := false
	for _, t := range db.InfuseTypes {
		if t.Offset == newInfuseOffset {
			validOffset = true
			break
		}
	}
	if !validOffset {
		return fmt.Errorf("infusion offset %d is not a recognised infusion type", newInfuseOffset)
	}

	a.pushUndo(charIdx)
	slot := &a.save.Slots[charIdx]
	return core.PatchWeaponItemID(slot, handle, expectedCurrentItemID, newItemID)
}

// ApplyWeaponUpgradeLevel changes the upgrade (smithing-stone reinforcement) level
// of a specific weapon instance identified by its GaItemHandle. Only the level
// portion of the ItemID is patched — base weapon, infusion offset, AoW gem,
// quantity, and location are preserved.
//
// expectedCurrentItemID is a stale-data guard: the request is rejected if the
// weapon's ItemID has already changed since the frontend loaded the character data.
// newItemID must encode the same base weapon and the same infusion offset; only
// the upgrade level portion may differ. The new level must be in [0, MaxUpgrade].
// Weapons with MaxUpgrade==0 (e.g. Unarmed) cannot be upgraded.
func (a *App) ApplyWeaponUpgradeLevel(charIdx int, handle uint32, expectedCurrentItemID, newItemID uint32) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return fmt.Errorf("invalid character index %d", charIdx)
	}

	// Both IDs must be in the weapon range (upper nibble 0x0).
	if expectedCurrentItemID>>28 != 0 {
		return fmt.Errorf("expectedCurrentItemID 0x%08X is not a weapon ID", expectedCurrentItemID)
	}
	if newItemID>>28 != 0 {
		return fmt.Errorf("newItemID 0x%08X is not a weapon ID", newItemID)
	}

	// Resolve base weapon from expectedCurrentItemID. DB lookup is authoritative.
	baseData, baseID := db.GetItemDataFuzzy(expectedCurrentItemID)
	if baseData.Name == "" {
		return fmt.Errorf("unknown weapon 0x%08X", expectedCurrentItemID)
	}
	if baseData.MaxUpgrade == 0 {
		return fmt.Errorf("weapon %q (0x%08X) cannot be upgraded (maxUpgrade=0)", baseData.Name, baseID)
	}

	expectedDiff := expectedCurrentItemID - baseID
	expectedInfuse := expectedDiff - (expectedDiff % 100)

	// Validate newItemID resolves to the same base weapon.
	_, newBaseID := db.GetItemDataFuzzy(newItemID)
	if newBaseID != baseID {
		return fmt.Errorf("cannot change weapon base ID with upgrade endpoint (newItemID 0x%08X resolves to base 0x%08X, expected 0x%08X)",
			newItemID, newBaseID, baseID)
	}
	newDiff := newItemID - baseID
	newLevel := newDiff % 100
	newInfuse := newDiff - newLevel

	// Infusion offset must be unchanged.
	if newInfuse != expectedInfuse {
		return fmt.Errorf("cannot change infusion with upgrade endpoint (infuseOffset %d→%d); use ApplyWeaponInfusion instead",
			expectedInfuse, newInfuse)
	}

	// Level must be within [0, MaxUpgrade]. newLevel is unsigned so >=0 is automatic.
	if newLevel > uint32(baseData.MaxUpgrade) {
		return fmt.Errorf("upgrade level %d out of range (max %d) for weapon %q (0x%08X)",
			newLevel, baseData.MaxUpgrade, baseData.Name, baseID)
	}

	a.pushUndo(charIdx)
	slot := &a.save.Slots[charIdx]
	return core.PatchWeaponItemID(slot, handle, expectedCurrentItemID, newItemID)
}

// ApplyWeaponAoW sets or removes the Ash of War on a specific weapon instance.
// newAoWItemID == 0 removes the AoW; any other value sets it.
//
// NOTE: currently invoked only by the hidden legacy Weapon Edit tab; do not remove without
// updating tests (app_weapon_aow_editor_test.go, app_weapon_aow_dlc_test.go) and the cleanup plan.
// Sort Order's weapon editor modal uses ApplyWeaponAoWStrict instead.
func (a *App) ApplyWeaponAoW(charIdx int, weaponHandle uint32, newAoWItemID uint32) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return fmt.Errorf("invalid character index %d", charIdx)
	}

	slot := &a.save.Slots[charIdx]
	currentItemID, ok := slot.GaMap[weaponHandle]
	if !ok {
		return fmt.Errorf("weapon handle 0x%08X not found in save", weaponHandle)
	}
	if currentItemID>>28 != 0 {
		return fmt.Errorf("handle 0x%08X (itemID 0x%08X) is not a weapon", weaponHandle, currentItemID)
	}

	baseData, baseID := db.GetItemDataFuzzy(currentItemID)
	if baseData.Name == "" {
		return fmt.Errorf("unknown weapon 0x%08X", currentItemID)
	}
	if !db.CanWeaponMountAoW(baseID) {
		return fmt.Errorf("weapon %q does not support Ash of War (gemMountType=%d)", baseData.Name, baseData.GemMountType)
	}

	if newAoWItemID != 0 {
		if newAoWItemID>>28 != 8 {
			return fmt.Errorf("newAoWItemID 0x%08X is not an Ash of War item ID", newAoWItemID)
		}
		aowData, _ := db.GetItemDataFuzzy(newAoWItemID)
		if aowData.Name == "" {
			return fmt.Errorf("unknown Ash of War item 0x%08X", newAoWItemID)
		}
		compatible, known := db.IsAshOfWarCompatibleWithWeapon(newAoWItemID, currentItemID)
		// known==false means no compatibility data (DLC/unmapped weapon type).
		// CanWeaponMountAoW already confirmed GemMountType==2 above — allow the operation.
		if known && !compatible {
			return fmt.Errorf("selected Ash of War is not compatible with this weapon")
		}
	}

	a.pushUndo(charIdx)
	return core.PatchWeaponAoW(slot, weaponHandle, newAoWItemID)
}

// GetAoWAvailability scans GaItems for character charIdx and returns per-itemID
// availability stats for every Ash of War present in the slot.
// AoW items absent from the slot are not included — the frontend treats absence as missing.
func (a *App) GetAoWAvailability(charIdx int) ([]vm.AoWAvailabilityEntry, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return nil, fmt.Errorf("invalid character index %d", charIdx)
	}
	slot := &a.save.Slots[charIdx]
	rawCopies := core.ScanAoWAvailability(slot)

	type aggregate struct {
		total    int
		used     int
		handles  []uint32
		conflict bool
	}
	agg := make(map[uint32]*aggregate)
	for _, c := range rawCopies {
		ag, ok := agg[c.ItemID]
		if !ok {
			ag = &aggregate{}
			agg[c.ItemID] = ag
		}
		ag.total++
		if c.UsedByWeaponHandle != 0 {
			ag.used++
			ag.handles = append(ag.handles, c.UsedByWeaponHandle)
		}
		if c.HasSharedHandleConflict {
			ag.conflict = true
		}
	}

	result := make([]vm.AoWAvailabilityEntry, 0, len(agg))
	for itemID, ag := range agg {
		result = append(result, vm.AoWAvailabilityEntry{
			ItemID:                  itemID,
			TotalCopies:             ag.total,
			AvailableCopies:         ag.total - ag.used,
			UsedCopies:              ag.used,
			UsedByWeaponHandles:     ag.handles,
			IsMissing:               false,
			HasSharedHandleConflict: ag.conflict,
		})
	}
	return result, nil
}

// ApplyWeaponAoWStrict sets or removes the AoW on a weapon using only pre-existing free copies.
// newAoWItemID == 0: removes the AoW (patches AoWGaItemHandle to the canonical
// core.NoCustomAoWHandle (0x00000000) in-place).
// newAoWItemID != 0: finds the first free copy of that AoW in the slot and attaches it.
//   Returns an error if no free copy exists, if a shared-handle conflict is detected,
//   or if any standard validation fails. Unlike ApplyWeaponAoW, never allocates new GaItem records.
func (a *App) ApplyWeaponAoWStrict(charIdx int, weaponHandle uint32, newAoWItemID uint32) error {
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return fmt.Errorf("invalid character index %d", charIdx)
	}

	slot := &a.save.Slots[charIdx]
	currentItemID, ok := slot.GaMap[weaponHandle]
	if !ok {
		return fmt.Errorf("weapon handle 0x%08X not found in save", weaponHandle)
	}
	if currentItemID>>28 != 0 {
		return fmt.Errorf("handle 0x%08X (itemID 0x%08X) is not a weapon", weaponHandle, currentItemID)
	}

	baseData, baseID := db.GetItemDataFuzzy(currentItemID)
	if baseData.Name == "" {
		return fmt.Errorf("unknown weapon 0x%08X", currentItemID)
	}
	if !db.CanWeaponMountAoW(baseID) {
		return fmt.Errorf("weapon %q does not support Ash of War changes", baseData.Name)
	}

	var newAoWHandle uint32
	if newAoWItemID == 0 {
		newAoWHandle = core.NoCustomAoWHandle
	} else {
		if newAoWItemID>>28 != 8 {
			return fmt.Errorf("newAoWItemID 0x%08X is not an Ash of War item ID", newAoWItemID)
		}
		aowData, _ := db.GetItemDataFuzzy(newAoWItemID)
		if aowData.Name == "" {
			return fmt.Errorf("unknown Ash of War item 0x%08X", newAoWItemID)
		}
		compatible, known := db.IsAshOfWarCompatibleWithWeapon(newAoWItemID, currentItemID)
		// known==false means no compatibility data (DLC/unmapped weapon type).
		// CanWeaponMountAoW already confirmed GemMountType==2 above — allow the operation.
		if known && !compatible {
			return fmt.Errorf("selected Ash of War is not compatible with this weapon")
		}

		rawCopies := core.ScanAoWAvailability(slot)
		hasConflict := false
		hasCopies := false
		newAoWHandle = 0
		for _, c := range rawCopies {
			if c.ItemID != newAoWItemID {
				continue
			}
			hasCopies = true
			if c.HasSharedHandleConflict {
				hasConflict = true
			}
			if c.UsedByWeaponHandle == 0 && newAoWHandle == 0 {
				newAoWHandle = c.Handle
			}
		}
		if hasConflict {
			return fmt.Errorf("Ash of War handle conflict detected — cannot safely apply in strict mode")
		}
		if !hasCopies {
			return fmt.Errorf("selected Ash of War is not present in save")
		}
		if newAoWHandle == 0 {
			return fmt.Errorf("no free copy of selected Ash of War is available")
		}
	}

	a.pushUndo(charIdx)
	return core.PatchWeaponAoWHandle(slot, weaponHandle, newAoWHandle)
}

// RepairDuplicateInventoryIndices rewrites every duplicate acquisition Index in
// Inventory.CommonItems + KeyItems on the given character slot. The first
// occurrence of each Index is preserved; later occurrences receive fresh
// Indices greater than every existing value, plus the matching counter
// (NextAcquisitionSortId / NextEquipIndex) advances accordingly.
//
// Pushes an undo snapshot only when at least one duplicate is detected, so a
// clean slot is a true no-op (no undo entry, no slot mutation). Never writes
// the save to disk — the caller decides when to persist via the regular
// save/upload flow.
func (a *App) RepairDuplicateInventoryIndices(charIdx int) (core.InventoryIndexRepairReport, error) {
	var empty core.InventoryIndexRepairReport
	if a.save == nil {
		return empty, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return empty, fmt.Errorf("invalid character index")
	}
	slot := &a.save.Slots[charIdx]

	if pre := core.ScanDuplicateInventoryIndices(slot); len(pre) == 0 {
		return empty, nil
	}

	a.pushUndo(charIdx)
	report, err := core.RepairDuplicateInventoryIndices(slot)
	if err != nil {
		return empty, err
	}
	if post := core.ScanDuplicateInventoryIndices(slot); len(post) > 0 {
		return report, fmt.Errorf("RepairDuplicateInventoryIndices: %d duplicate(s) remain after repair", len(post))
	}
	return report, nil
}

// Dummy method to force Wails to export types
func (a *App) _forceExportTypes() (db.GraceEntry, db.BossEntry, db.ItemEntry, db.MapEntry, db.CookbookEntry, db.GestureEntry, db.QuestNPC, db.QuestStep, db.QuestFlagState, core.SlotDiagnostics, core.DiagnosticIssue, DiffEntry, SlotDiffSummary, SlotCapacity, deploy.Target, PresetInfo, FavoriteSlotInfo, db.BellBearingEntry, db.WhetbladeEntry, db.AshOfWarFlagEntry, core.NetworkParamValues, vm.CharacterPreset, vm.PresetItem, vm.ApplyOptions, vm.PresetApplyResult, vm.WorldPresetData, PvPPreparationOptions, vm.AoWAvailabilityEntry, BuiltinCharacterPresetInfo, InventoryOrderItem, core.TransferResult, core.TransferSkip, core.InventoryIndexRepairReport, core.InventoryIndexRepairChange) {
	return db.GraceEntry{}, db.BossEntry{}, db.ItemEntry{}, db.MapEntry{}, db.CookbookEntry{}, db.GestureEntry{}, db.QuestNPC{}, db.QuestStep{}, db.QuestFlagState{}, core.SlotDiagnostics{}, core.DiagnosticIssue{}, DiffEntry{}, SlotDiffSummary{}, SlotCapacity{}, deploy.Target{}, PresetInfo{}, FavoriteSlotInfo{}, db.BellBearingEntry{}, db.WhetbladeEntry{}, db.AshOfWarFlagEntry{}, core.NetworkParamValues{}, vm.CharacterPreset{}, vm.PresetItem{}, vm.ApplyOptions{}, vm.PresetApplyResult{}, vm.WorldPresetData{}, PvPPreparationOptions{}, vm.AoWAvailabilityEntry{}, BuiltinCharacterPresetInfo{}, InventoryOrderItem{}, core.TransferResult{}, core.TransferSkip{}, core.InventoryIndexRepairReport{}, core.InventoryIndexRepairChange{}
}
