package main

import (
	"context"
	"crypto/rand"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strconv"
	"sync"
	"unicode/utf16"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
	"github.com/oisis/EldenRing-SaveForge/backend/deploy"
	"github.com/oisis/EldenRing-SaveForge/backend/editor"
	"github.com/oisis/EldenRing-SaveForge/backend/templates"
	"github.com/oisis/EldenRing-SaveForge/backend/vm"
	"github.com/wailsapp/wails/v2/pkg/runtime"
)

const maxUndoDepth = 5

// maxCharacters mirrors the canonical fixed slot count baked into
// core.SaveFile (Slots [10]SaveSlot — backend/core/save_manager.go) and
// the validation literal `charIdx >= 10` used throughout this file. It
// exists so the lifecycle mutex array has a single symbolic source; we
// deliberately do NOT refactor every `10` in the codebase here.
const maxCharacters = 10

// slotSnapshot holds a deep copy of a SaveSlot for undo purposes.
type slotSnapshot struct {
	Active             bool
	ProfileSummary     core.ProfileSummary
	Data               []byte
	Version            uint32
	Player             core.PlayerGameData
	GaMap              map[uint32]uint32
	GaItems            []core.GaItemFull
	Inventory          core.EquipInventoryData
	Storage            core.EquipInventoryData
	Warnings           []string
	MagicOffset        int
	InventoryEnd       int
	EventFlagsOffset   int
	PlayerDataOffset   int
	FaceDataOffset     int
	StorageBoxOffset   int
	IngameTimerOffset  int
	GaItemDataOffset   int
	TutorialDataOffset int
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
	//
	// editSessionsMu serializes every lifecycle operation on the two maps
	// above (insert, replace, lookup, delete, full clear). Wails dispatches
	// each bound endpoint in its own goroutine, so without this mutex two
	// concurrent StartInventoryEditSession calls would race on the map writes
	// at app_inventory_session.go and crash the process with
	// `fatal error: concurrent map writes`. The mutex is held ONLY around
	// registry-touching code — long-running mutations and slot rebuilds use
	// the per-session lock on InventoryEditSession instead, so two sessions
	// for two different characters can still run in parallel.
	editSessionsMu sync.Mutex
	// lifecycleMu serialises session lifecycle transitions per character
	// slot (Start replacement, Discard, clearAllEditSessions). It is
	// SEPARATE from editSessionsMu and from sess.mu and exists to close
	// the cross-session slot race: without it a new StartInventoryEditSession
	// could call editor.StartSession on a.save.Slots[charIdx] while the
	// PREVIOUS session for the same slot was still inside
	// SaveInventoryWorkspaceChanges mutating the same slot. Lock order is
	// strict: lifecycleMu[charIdx] (long-held) → editSessionsMu (short,
	// registry only) → sess.Acquire() (per-session, may block on a peer's
	// Save). Never take a lifecycleMu while already holding sess.mu.
	// Different characters use independent entries so two characters can
	// still be edited in parallel.
	lifecycleMu       [maxCharacters]sync.Mutex
	editSessions      map[string]*editor.InventoryEditSession
	editSessionByChar map[int]string

	// templateLibrary is the lazily-initialised handle to the per-user
	// build template store on disk. nil until the first endpoint that
	// needs it calls ensureTemplateLibrary. Tests may pre-populate this
	// field via a non-default rootDir.
	templateLibrary *templates.TemplateLibrary

	// --- Save / slot / favorites / source-save locks (Phase 2) ---
	//
	// These mutexes were introduced together to close the cross-endpoint
	// concurrency holes that the previous inventory-session fix left open
	// (whole-save replacement vs in-flight slot operations, GaMap
	// concurrent-map-writes between session Save and non-session mutators,
	// favSlotNames concurrent-map-writes between favorites endpoints,
	// sourceSave use-after-replace).
	//
	// Lock order — taken in increasing order, released in reverse — is:
	//   1. saveMu                 (whole-save lifecycle + metadata)
	//   2. lifecycleMu[i]         (session lifecycle per slot — existing)
	//   3. editSessionsMu         (session registry, short — existing)
	//   4. sess.mu                (per-session workspace state — existing)
	//   5. favMu                  (UserData10 + favSlotNames)
	//   6. sourceSaveMu           (a.sourceSave pointer + content)
	//   7. slotMu[i]              (per-slot data, ascending i)
	// Multi-slot operations take slotMu in strictly ascending index order
	// and release in descending order (see lockAllSlots / lockSlotPair).
	// sync.RWMutex is NOT treated as reentrant: never call RLock while
	// already holding RLock on the same RWMutex on the same goroutine.
	//
	// saveMu protects:
	//   - the a.save pointer itself (replaced by installLoadedSave under Lock);
	//   - whole-save metadata mutated outside of a single slot:
	//     UserData11 (network params), SteamID, Platform, IV, Encrypted,
	//     lastSavePath. Writers take saveMu.Lock(); readers RLock().
	saveMu sync.RWMutex
	// slotMu[i] protects the per-character data of Slots[i] as well as
	// ActiveSlots[i], ProfileSummaries[i] and undoStacks[i] — every field
	// keyed by character index. Always held in ADDITION to saveMu.RLock
	// (or under saveMu.Lock for multi-slot lifecycle), never alone.
	slotMu [maxCharacters]sync.Mutex
	// favMu serialises access to the favorites blob inside the current
	// save: a.save.UserData10 (raw bytes mutated element-level by the
	// favorites endpoints) and a.favSlotNames (Go map indexed by the
	// favorite slot id). Concurrent map writes on favSlotNames would
	// otherwise crash the process. Taken AFTER sess.mu, BEFORE slotMu[i]
	// when both apply (see ApplyMirrorFavoriteToCharacter,
	// WriteSelectedToFavorites).
	favMu sync.RWMutex
	// sourceSaveMu protects the a.sourceSave pointer (installed by
	// SelectAndOpenSourceSave) and every dereference of its slots from the
	// diff / source-active-slots endpoints. Independent of saveMu because
	// the source save is loaded only for diff/import preview — it is never
	// mutated by user actions on the active save.
	sourceSaveMu sync.RWMutex
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
	if a.ctx == nil {
		return // headless / test: no Wails runtime context to log or emit to
	}
	msg := fmt.Sprintf(format, args...)
	runtime.LogInfof(a.ctx, "%s", msg)
	runtime.EventsEmit(a.ctx, "app:log", "info", msg)
}

func (a *App) logError(format string, args ...interface{}) {
	if a.ctx == nil {
		return // headless / test: no Wails runtime context to log or emit to
	}
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
	// Commit phase: exclusive saveMu blocks every reader/writer of the old
	// a.save while the pointer swap + derived-state reset run. installLoadedSave
	// also resets favSlotNames; favMu is intentionally NOT taken here — every
	// favorites endpoint enters via saveMu.RLock first, so saveMu.Lock alone
	// guarantees no favorites endpoint is mid-flight.
	a.saveMu.Lock()
	a.installLoadedSave(save, path)
	a.saveMu.Unlock()
	return string(save.Platform), nil
}

// installLoadedSave swaps the active save pointer to candidate and resets
// every piece of derived state that was scoped to the previous save:
// lastSavePath, favSlotNames, undo stacks, edit sessions.
//
// Contract: caller MUST run dialog / file I/O / decrypt / parse OUTSIDE
// this helper — pass a fully-prepared *core.SaveFile and only invoke the
// helper for the atomic commit phase. Caller MUST hold a.saveMu.Lock for
// the entire call; the helper itself does not acquire it. Holding saveMu
// exclusively is sufficient to also serialise the favSlotNames reset
// against favorites endpoints (which all enter via saveMu.RLock first),
// so favMu is intentionally not re-acquired here.
func (a *App) installLoadedSave(candidate *core.SaveFile, path string) {
	a.save = candidate
	a.lastSavePath = path
	a.favSlotNames = make(map[int]string)
	a.clearAllUndoStacks()
	a.clearAllEditSessions()
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
	// Commit phase: sourceSaveMu.Lock blocks every reader of the old
	// a.sourceSave (GetSourceActiveSlots, GetSlotDiff, GetSaveDiffSummary)
	// while the pointer is swapped. We do NOT take saveMu.RLock here
	// because the endpoint touches only a.sourceSave; the active save is
	// untouched.
	a.sourceSaveMu.Lock()
	a.installSourceSave(save)
	a.sourceSaveMu.Unlock()
	return string(save.Platform), nil
}

// installSourceSave swaps the source save pointer to candidate.
//
// Contract: caller MUST run dialog / file I/O / decrypt / parse OUTSIDE
// this helper — pass a fully-prepared *core.SaveFile and only invoke the
// helper for the atomic commit phase. Caller MUST hold a.sourceSaveMu.Lock
// for the entire call; the helper itself does not acquire it.
func (a *App) installSourceSave(candidate *core.SaveFile) {
	a.sourceSave = candidate
}

// GetCharacter returns the ViewModel for a specific slot
func (a *App) GetCharacter(index int) (*vm.CharacterViewModel, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if index < 0 || index >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[index].Lock()
	defer a.slotMu[index].Unlock()

	slot := a.save.Slots[index]
	return vm.MapParsedSlotToVM(&slot)
}

// SaveCharacter updates the raw slot data from the ViewModel
func (a *App) SaveCharacter(index int, charVM vm.CharacterViewModel) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if index < 0 || index >= 10 {
		return fmt.Errorf("invalid slot index")
	}

	a.slotMu[index].Lock()
	defer a.slotMu[index].Unlock()

	a.pushUndoLocked(index)

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

// WriteSave writes the current save state to a file.
//
// The dialog and on-disk backup happen OUTSIDE saveMu so that blocking UX
// (file dialog, backup I/O) never holds the whole-save write lock. Before
// the dialog we snapshot the active save pointer under saveMu.RLock; the
// snapshot is then passed to writeSaveCore, which under saveMu.Lock
// verifies that a.save still refers to the same instance. If a concurrent
// SelectAndOpenSave / DownloadRemoteSave replaced a.save while the user
// was choosing the destination path, writeSaveCore aborts with a clear
// error WITHOUT touching the file or mutating any metadata — i.e. it
// refuses to silently serialise save B under a path picked for save A.
func (a *App) WriteSave(platform string) error {
	a.saveMu.RLock()
	if a.save == nil {
		a.saveMu.RUnlock()
		return fmt.Errorf("no save loaded")
	}
	expected := a.save
	a.saveMu.RUnlock()

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

	return a.writeSaveCore(path, platform, expected)
}

// writeSaveCore performs the in-memory mutation of a.save (Platform / IV /
// Encrypted), serializes it to disk and resets undo state + lastSavePath.
//
// Acquires a.saveMu.Lock() for the entire critical section. Caller MUST
// have performed dialog / backup work BEFORE invoking this helper, and
// MUST pass `expected` — the *core.SaveFile snapshot taken before the
// dialog. When a concurrent SelectAndOpenSave / DownloadRemoteSave
// replaced a.save during the dialog wait, the identity check fails fast:
// no metadata is mutated, no file is written, the helper returns a
// user-facing error instead of silently writing save B to a path picked
// for save A.
func (a *App) writeSaveCore(path string, platform string, expected *core.SaveFile) error {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()

	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if a.save != expected {
		return fmt.Errorf("active save changed while the save dialog was open; write to %q aborted to avoid overwriting it with the wrong save", path)
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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	return a.getItemListLocked(category)
}

// GetItemListChunk is the progressive-load alias for GetItemList.
//
// Delegates to the internal locked worker (not the public GetItemList) to
// avoid nested saveMu.RLock on a single goroutine — sync.RWMutex is not
// treated as reentrant.
func (a *App) GetItemListChunk(category string) []db.ItemEntry {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	return a.getItemListLocked(category)
}

// getItemListLocked is the internal read-only worker for GetItemList and
// GetItemListChunk.
//
// Contract: caller MUST hold a.saveMu.RLock for the duration of the call so
// the a.save / a.save.Platform read is stable. The database lookup itself
// is on static data and needs no lock.
func (a *App) getItemListLocked(category string) []db.ItemEntry {
	platform := "PS4"
	if a.save != nil {
		platform = string(a.save.Platform)
	}
	return db.GetItemsByCategory(category, platform)
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

// SlotState is the frontend's single source of truth for character-slot
// occupancy. Residual means the game has cleared the active flag, but stale
// character data still occupies the raw slot/profile-summary region.
type SlotState struct {
	Index    int    `json:"index"`
	Name     string `json:"name"`
	Active   bool   `json:"active"`
	Residual bool   `json:"residual"`
	Empty    bool   `json:"empty"`
}

func utf16UnitLen(s string) int {
	return len(utf16.Encode([]rune(s)))
}

func trimToUTF16Units(s string, maxUnits int) string {
	if maxUnits <= 0 {
		return ""
	}
	units := 0
	out := make([]rune, 0, len(s))
	for _, r := range s {
		need := 1
		if r > 0xFFFF {
			need = 2
		}
		if units+need > maxUnits {
			break
		}
		out = append(out, r)
		units += need
	}
	return string(out)
}

func encodeCharacterName16(name string) [16]uint16 {
	var out [16]uint16
	encoded := utf16.Encode([]rune(trimToUTF16Units(name, 16)))
	copy(out[:], encoded)
	return out
}

func uniqueClonedCharacterName(base string, used map[string]struct{}) string {
	base = trimToUTF16Units(base, 16)
	for suffixNum := 2; suffixNum < 10000; suffixNum++ {
		suffix := fmt.Sprintf(" %d", suffixNum)
		maxBaseUnits := 16 - utf16UnitLen(suffix)
		candidate := trimToUTF16Units(base, maxBaseUnits) + suffix
		if _, exists := used[candidate]; !exists {
			return candidate
		}
	}
	return trimToUTF16Units(base, 14) + " 2"
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

	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return result, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return result, fmt.Errorf("invalid character index")
	}

	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()

	slot := &a.save.Slots[charIdx]

	// PRE-FLIGHT (fail-closed): refuse to mutate a slot that already holds
	// duplicate acquisition indices. The duplicates almost certainly originate
	// from an older SaveForge revision; whether the game would silently accept
	// such a save is unverified, so we treat the state as a save-integrity
	// issue and require an explicit repair (RepairDuplicateInventoryIndices)
	// before any further inventory mutation. Never auto-repairs here.
	if dups := core.ScanDuplicateInventoryIndices(slot); len(dups) > 0 {
		return result, fmt.Errorf(
			"inventory integrity issue: slot %d contains %d duplicate acquisition entries; repair is required before adding items",
			charIdx, len(dups))
	}
	if physickDupes := core.ScanDuplicateWondrousPhysick(slot); len(physickDupes) > 0 {
		return result, fmt.Errorf(
			"inventory integrity issue: slot %d contains %d Flask of Wondrous Physick records; repair is required before adding items",
			charIdx, len(physickDupes))
	}

	// Build maps from current inventory/storage state:
	// - existingItemQty: per-item stack qty in inventory (used to compute SET delta)
	// - existingByContainer: total pot/aromatic units per container
	// - existingStorageQty: per-item stack qty in storage
	existingItemQty := make(map[uint32]int)
	existingByContainer := make(map[uint32]int)
	hasPhysick := false
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == 0 || item.GaItemHandle == 0xFFFFFFFF {
			continue
		}
		invID, mapped := slot.GaMap[item.GaItemHandle]
		if !mapped || invID == 0 {
			invID = db.HandleToItemID(item.GaItemHandle)
		}
		if db.IsWondrousPhysick(invID) {
			hasPhysick = true
		}
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
		if _, seen := existingItemQty[keyBaseID]; seen {
			continue // ponytail: item in both sections — CommonItems is canonical, skip double-count
		}
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
		isPhysick := db.IsWondrousPhysick(id)
		if isPhysick {
			id = db.ItemFlaskWondrousPhysickEmpty
		}
		itemData, _ := db.GetItemDataFuzzy(id)
		finalID := id
		// Clamp the requested upgrade to the item's real MaxUpgrade so an add can
		// never encode an out-of-range level (e.g. base+25 for a somber +10 weapon
		// like Golem Greatbow). The encoded ID embeds the level, so an over-cap
		// value would produce a permanently invalid item the editor then rejects.
		switch {
		case itemData.Category == "ashes":
			finalID = id + uint32(editor.ClampUpgrade(upgradeAsh, int(itemData.MaxUpgrade)))
		case itemData.MaxUpgrade == 25:
			lvl := editor.ClampUpgrade(upgrade25, int(itemData.MaxUpgrade))
			if infuseOffset != 0 && weaponCategorySupportsInfusion(itemData.Category) {
				finalID = id + uint32(infuseOffset) + uint32(lvl)
			} else {
				finalID = id + uint32(lvl)
			}
		case itemData.MaxUpgrade == 10:
			finalID = id + uint32(editor.ClampUpgrade(upgrade10, int(itemData.MaxUpgrade)))
		}

		actualInv := resolveQty(invQty, int(itemData.MaxInventory))
		actualStorage := resolveQty(storageQty, int(itemData.MaxStorage))
		if isPhysick {
			actualStorage = 0
			if hasPhysick {
				actualInv = 0
			} else if actualInv > 0 {
				hasPhysick = true
			}
		}

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
	a.pushUndoLocked(charIdx)
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

	// POST-VALIDATION: check invariants after mutation. The pre-flight guard
	// guarantees the slot was free of duplicate acquisition indices on entry,
	// so any duplicate detected here was introduced by this add and must roll
	// back the entire batch.
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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return fmt.Errorf("invalid character index")
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	a.pushUndoLocked(charIdx)

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return empty, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= len(a.save.Slots) {
		return empty, fmt.Errorf("invalid character index %d", charIdx)
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
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
		a.pushUndoLocked(charIdx)
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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if srcIdx < 0 || srcIdx >= 10 || destIdx < 0 || destIdx >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	if srcIdx == destIdx {
		return fmt.Errorf("source and destination must differ")
	}
	// Clone reads every slot name to choose a unique destination name, so take
	// the same multi-slot lock used by other all-slot readers.
	a.lockAllSlots()
	defer a.unlockAllSlots()

	if !a.save.ActiveSlots[srcIdx] {
		return fmt.Errorf("source slot %d is not active", srcIdx)
	}
	srcName := core.UTF16ToString(a.save.Slots[srcIdx].Player.CharacterName[:])
	if srcName == "" {
		return fmt.Errorf("source slot %d is empty", srcIdx)
	}
	if a.save.ActiveSlots[destIdx] {
		return fmt.Errorf("destination slot %d is not empty", destIdx)
	}
	if a.save.SlotHasResidualData(destIdx) {
		return fmt.Errorf("destination slot %d contains residual character data; clean it first", destIdx)
	}

	usedNames := make(map[string]struct{}, 10)
	for i := 0; i < 10; i++ {
		if !a.save.ActiveSlots[i] && !a.save.SlotHasResidualData(i) {
			continue
		}
		name := a.slotDisplayNameLocked(i)
		if name != "" {
			usedNames[name] = struct{}{}
		}
	}
	cloneName := uniqueClonedCharacterName(srcName, usedNames)

	a.pushUndoLocked(destIdx)

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
	src.Player.CharacterName = encodeCharacterName16(cloneName)

	a.save.Slots[destIdx] = src
	a.save.ActiveSlots[destIdx] = true
	a.save.ProfileSummaries[destIdx] = a.save.ProfileSummaries[srcIdx]
	a.save.ProfileSummaries[destIdx].CharacterName = src.Player.CharacterName

	return nil
}

// DeleteSlot clears a character slot IN PLACE — matching the game's positional
// model. Slots 0-9 keep their positions; only the target slot is zeroed (data
// block, active flag, full ProfileSummary region). Subsequent slots are NOT
// shifted down (the game uses independent per-slot active flags; gaps are valid).
func (a *App) DeleteSlot(idx int) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if idx < 0 || idx >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[idx].Lock()
	defer a.slotMu[idx].Unlock()
	// Occupied = active flag set OR residual data left by an in-game deletion.
	// Including residual lets the user clear a phantom slot the game ignores.
	if !a.save.ActiveSlots[idx] && !a.save.SlotHasResidualData(idx) {
		return fmt.Errorf("slot %d is already empty", idx)
	}

	a.pushUndoLocked(idx)
	a.save.ClearSlot(idx)
	return nil
}

// CleanResidualSlots zeroes every inactive slot that still carries leftover
// character data (a phantom produced when a character was deleted in-game but its
// data block / summary were never cleared). Returns the number of slots cleaned.
func (a *App) CleanResidualSlots() (int, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return 0, fmt.Errorf("no save loaded")
	}
	// Multi-slot writer — take every slotMu rosnąco. core.CleanResidualSlots
	// may touch any subset of slots so we need them all under lock.
	a.lockAllSlots()
	defer a.unlockAllSlots()
	for i := 0; i < 10; i++ {
		if a.save.SlotHasResidualData(i) {
			a.pushUndoLocked(i)
		}
	}
	return a.save.CleanResidualSlots(), nil
}

// CleanResidualSlot explicitly zeroes one inactive slot that still carries
// stale character data. Active slots and already-empty slots are rejected so the
// UI can safely expose this as a targeted cleanup action.
func (a *App) CleanResidualSlot(idx int) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if idx < 0 || idx >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[idx].Lock()
	defer a.slotMu[idx].Unlock()
	if a.save.ActiveSlots[idx] {
		return fmt.Errorf("slot %d is active", idx)
	}
	if !a.save.SlotHasResidualData(idx) {
		return fmt.Errorf("slot %d has no residual data", idx)
	}
	a.pushUndoLocked(idx)
	a.save.ClearSlot(idx)
	return nil
}

// GetActiveSlots returns the activity status of all 10 slots. Source of truth is
// the per-slot active flag (0x1954) — exactly what the game's character-select
// reads. A slot with residual data but a cleared flag (phantom) reports inactive,
// so the UI roster matches the console.
//
// Multi-slot reader: takes saveMu.RLock + every slotMu[0..9] (ascending) so
// the returned snapshot is consistent with the slotMu policy that protects
// ActiveSlots[i] writes (SetSlotActivity, CloneSlot, DeleteSlot,
// CleanResidualSlots).
func (a *App) GetActiveSlots() []bool {
	active := make([]bool, 10)
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return active
	}
	a.lockAllSlots()
	defer a.unlockAllSlots()
	copy(active, a.save.ActiveSlots[:])
	return active
}

func (a *App) slotDisplayNameLocked(idx int) string {
	name := core.UTF16ToString(a.save.Slots[idx].Player.CharacterName[:])
	if name == "" {
		name = core.UTF16ToString(a.save.ProfileSummaries[idx].CharacterName[:])
	}
	return name
}

// GetSlotStates returns active, residual and empty state for all character
// slots. Prefer this over combining GetActiveSlots and GetCharacterNames in the
// frontend: inactive residual slots need a distinct UX from truly empty slots.
func (a *App) GetSlotStates() []SlotState {
	states := make([]SlotState, 10)
	for i := range states {
		states[i] = SlotState{Index: i, Name: "Empty Slot", Empty: true}
	}
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return states
	}
	a.lockAllSlots()
	defer a.unlockAllSlots()
	for i := 0; i < 10; i++ {
		active := a.save.ActiveSlots[i]
		residual := a.save.SlotHasResidualData(i)
		name := a.slotDisplayNameLocked(i)
		if name == "" {
			name = "Empty Slot"
		}
		states[i] = SlotState{
			Index:    i,
			Name:     name,
			Active:   active,
			Residual: residual,
			Empty:    !active && !residual,
		}
	}
	return states
}

// GetCharacterNames returns the names of all 10 characters. Inactive slots
// (active flag cleared) report "Empty Slot" regardless of residual data, so a
// phantom slot is not shown as a duplicate. The name is read from the slot-data
// block, falling back to the ProfileSummary name if the block is empty.
func (a *App) GetCharacterNames() []string {
	names := make([]string, 10)
	for i := range names {
		names[i] = "Empty Slot"
	}
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return names
	}
	// Multi-slot reader — every Slots[i].Player.CharacterName,
	// ProfileSummaries[i].CharacterName and ActiveSlots[i] are protected
	// by their respective slotMu[i]. Take all 10 ascending.
	a.lockAllSlots()
	defer a.unlockAllSlots()
	for i := 0; i < 10; i++ {
		if !a.save.ActiveSlots[i] {
			continue
		}
		name := core.UTF16ToString(a.save.Slots[i].Player.CharacterName[:])
		if name == "" {
			name = core.UTF16ToString(a.save.ProfileSummaries[i].CharacterName[:])
		}
		if name != "" {
			names[i] = name
		}
	}
	return names
}

// GetSourceActiveSlots returns the activity status of all 10 slots in the source file
func (a *App) GetSourceActiveSlots() []bool {
	active := make([]bool, 10)
	a.sourceSaveMu.RLock()
	defer a.sourceSaveMu.RUnlock()
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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if index < 0 || index >= 10 {
		return fmt.Errorf("invalid slot index %d", index)
	}
	a.slotMu[index].Lock()
	defer a.slotMu[index].Unlock()
	a.save.ActiveSlots[index] = active
	return nil
}

// GetSteamIDString returns the SteamID as a decimal string to avoid JS float64 precision loss.
func (a *App) GetSteamIDString() string {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return ""
	}
	return strconv.FormatUint(a.save.SteamID, 10)
}

// SetSteamIDFromString parses a decimal string and updates the SteamID.
func (a *App) SetSteamIDFromString(s string) error {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
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

// pushUndoLocked takes a deep-copy snapshot of the given slot and pushes it
// onto the undo stack.
//
// Contract: caller MUST hold exclusive access to slot[idx] for the duration
// of this call. Today serialization is provided implicitly by the Wails
// dispatch model + per-endpoint defensive checks; in the upcoming slot-lock
// phase the caller will hold slotMu[idx]. The "Locked" suffix marks that
// this helper neither acquires that lock nor takes a snapshot of the undo
// stack itself — so it must not be called concurrently for the same idx.
func (a *App) pushUndoLocked(idx int) {
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
		Active:             a.save.ActiveSlots[idx],
		ProfileSummary:     a.save.ProfileSummaries[idx],
		Data:               dataCopy,
		Version:            slot.Version,
		Player:             slot.Player,
		GaMap:              gaMapCopy,
		GaItems:            gaItemsCopy,
		Inventory:          slot.Inventory.Clone(),
		Storage:            slot.Storage.Clone(),
		Warnings:           append([]string{}, slot.Warnings...),
		MagicOffset:        slot.MagicOffset,
		InventoryEnd:       slot.InventoryEnd,
		EventFlagsOffset:   slot.EventFlagsOffset,
		PlayerDataOffset:   slot.PlayerDataOffset,
		FaceDataOffset:     slot.FaceDataOffset,
		StorageBoxOffset:   slot.StorageBoxOffset,
		IngameTimerOffset:  slot.IngameTimerOffset,
		GaItemDataOffset:   slot.GaItemDataOffset,
		TutorialDataOffset: slot.TutorialDataOffset,
		NextAoWIndex:       slot.NextAoWIndex,
		NextArmamentIndex:  slot.NextArmamentIndex,
		NextGaItemHandle:   slot.NextGaItemHandle,
		PartGaItemHandle:   slot.PartGaItemHandle,
	}

	stack := a.undoStacks[idx]
	if len(stack) >= maxUndoDepth {
		stack = stack[1:] // drop oldest
	}
	a.undoStacks[idx] = append(stack, snap)
}

// lockAllSlots takes every slotMu[0..maxCharacters-1] in strictly ascending
// index order. Caller MUST call unlockAllSlots (typically via defer) on the
// same goroutine to release them in descending order. Used by multi-slot
// operations (CleanResidualSlots, GetActiveSlots, GetCharacterNames,
// AuditLoadedSaveIssues, writeTempSave) so they cannot deadlock with each
// other — the rosnąco rule is invariant across every multi-slot caller.
func (a *App) lockAllSlots() {
	for i := 0; i < maxCharacters; i++ {
		a.slotMu[i].Lock()
	}
}

// unlockAllSlots releases every slotMu acquired by lockAllSlots, in
// descending order. Safe to defer immediately after lockAllSlots().
func (a *App) unlockAllSlots() {
	for i := maxCharacters - 1; i >= 0; i-- {
		a.slotMu[i].Unlock()
	}
}

// lockSlotPair takes slotMu[a] and slotMu[b] in ascending order. When the
// two indices are equal it acquires only one lock — Go's sync.Mutex is not
// reentrant, so double-acquiring would deadlock. Returns an unlock closure
// that releases in the correct (descending) order. Used by CloneSlot.
func (a *App) lockSlotPair(idx1, idx2 int) (unlock func()) {
	if idx1 == idx2 {
		a.slotMu[idx1].Lock()
		return func() { a.slotMu[idx1].Unlock() }
	}
	lo, hi := idx1, idx2
	if lo > hi {
		lo, hi = hi, lo
	}
	a.slotMu[lo].Lock()
	a.slotMu[hi].Lock()
	return func() {
		a.slotMu[hi].Unlock()
		a.slotMu[lo].Unlock()
	}
}

// RevertSlot pops the last snapshot from the undo stack and restores the slot.
func (a *App) RevertSlot(idx int) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if idx < 0 || idx >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[idx].Lock()
	defer a.slotMu[idx].Unlock()
	stack := a.undoStacks[idx]
	if len(stack) == 0 {
		return fmt.Errorf("nothing to undo for slot %d", idx)
	}

	snap := stack[len(stack)-1]
	a.undoStacks[idx] = stack[:len(stack)-1]
	a.save.ActiveSlots[idx] = snap.Active
	a.save.ProfileSummaries[idx] = snap.ProfileSummary

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil || idx < 0 || idx >= 10 {
		return 0
	}
	a.slotMu[idx].Lock()
	defer a.slotMu[idx].Unlock()
	return len(a.undoStacks[idx])
}

// clearAllUndoStacks resets all undo history (called on file load/save).
func (a *App) clearAllUndoStacks() {
	for i := range a.undoStacks {
		a.undoStacks[i] = nil
	}
}

// clearAllEditSessions drops every in-memory inventory edit session. Called when
// a new save is loaded so stale sessions (which reference the previous save's
// slots) cannot linger; the frontend re-creates a fresh session on demand.
//
// Concurrency:
//   - Acquires every lifecycleMu[0..maxCharacters-1] in ascending order
//     BEFORE touching the registry. This blocks any concurrent
//     StartInventoryEditSession / DiscardInventoryEditSession for any
//     character until the bulk drain has completed — a new Start cannot
//     sneak in between the registry reset and the per-session close and
//     race the old Save on the freshly-replaced a.save.
//   - Then under editSessionsMu the registry maps are reset and drained
//     atomically. editSessionsMu is released before any per-session
//     close so a long-running peer cannot indirectly hold up the
//     registry probe path.
//   - Each drained session is then Close()-d. closeSession blocks on the
//     per-session mutex, draining any in-flight Save. By the time
//     clearAllEditSessions returns no orphan session is still writing.
//   - lifecycleMu locks are released in descending order; the ordering
//     is deterministic so concurrent Start/Discard cannot construct a
//     reverse cycle and deadlock.
func (a *App) clearAllEditSessions() {
	for i := 0; i < maxCharacters; i++ {
		a.lifecycleMu[i].Lock()
	}
	defer func() {
		for i := maxCharacters - 1; i >= 0; i-- {
			a.lifecycleMu[i].Unlock()
		}
	}()

	a.editSessionsMu.Lock()
	drained := make([]*editor.InventoryEditSession, 0, len(a.editSessions))
	for _, s := range a.editSessions {
		drained = append(drained, s)
	}
	a.editSessions = make(map[string]*editor.InventoryEditSession)
	a.editSessionByChar = make(map[int]string)
	a.editSessionsMu.Unlock()

	for _, s := range drained {
		closeSession(s)
	}
}

// GetNetworkParams reads the current invasion matchmaking parameters from the save's regulation.
func (a *App) GetNetworkParams() (*core.NetworkParamValues, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
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
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	return a.setNetworkParamsLocked(params)
}

// ResetNetworkParams restores vanilla defaults for all network parameters.
//
// Delegates to the internal locked worker (not the public SetNetworkParams)
// to avoid double-acquire of saveMu.Lock (sync.RWMutex.Lock is not
// reentrant).
func (a *App) ResetNetworkParams() error {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	return a.setNetworkParamsLocked(core.NetworkParamDefaults())
}

// setNetworkParamsLocked is the internal worker for SetNetworkParams and
// ResetNetworkParams.
//
// Contract: caller MUST have validated `a.save != nil` and MUST hold
// a.saveMu.Lock for the entire call. The helper reassigns the
// a.save.UserData11 slice header — not safe for concurrent readers without
// that exclusive lock.
func (a *App) setNetworkParamsLocked(params core.NetworkParamValues) error {
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

// GetNetworkPreset returns preset values by name without applying them.
func (a *App) GetNetworkPreset(name string) (*core.NetworkParamValues, error) {
	var p core.NetworkParamValues
	switch name {
	// Functional presets (active unified UI — single source of truth).
	case "faster-reds":
		p = core.NetworkParamFasterReds()
	case "faster-summons":
		p = core.NetworkParamFasterSummons()
	case "faster-blue":
		p = core.NetworkParamFasterBlue()
	case "aggressive-reds":
		p = core.NetworkParamAggressiveReds()
	case "aggressive-summons":
		p = core.NetworkParamAggressiveSummons()
	case "aggressive-blue":
		p = core.NetworkParamAggressiveBlue()
	case "vanilla":
		p = core.NetworkParamDefaults()
	// Summon sections presets (v1.0-beta5+).
	case "faster-summon-host":
		p = core.NetworkParamFasterSummonHost()
	case "aggressive-summon-host":
		p = core.NetworkParamAggressiveSummonHost()
	case "faster-summon-guest":
		p = core.NetworkParamFasterSummonGuest()
	case "aggressive-summon-guest":
		p = core.NetworkParamAggressiveSummonGuest()
	case "faster-hunter":
		p = core.NetworkParamFasterHunter()
	case "aggressive-hunter":
		p = core.NetworkParamAggressiveHunter()
	// Legacy preset keys (consumed only by the orphaned NetworkSpeedPanel).
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

// GetAoWAvailability scans GaItems for character charIdx and returns per-itemID
// availability stats for every Ash of War present in the slot.
// AoW items absent from the slot are not included — the frontend treats absence as missing.
func (a *App) GetAoWAvailability(charIdx int) ([]vm.AoWAvailabilityEntry, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return nil, fmt.Errorf("invalid character index %d", charIdx)
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return empty, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return empty, fmt.Errorf("invalid character index")
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	slot := &a.save.Slots[charIdx]

	pre := core.ScanDuplicateInventoryIndices(slot)
	prePhysick := core.ScanDuplicateWondrousPhysick(slot)
	if len(pre) == 0 && len(prePhysick) == 0 {
		return empty, nil
	}

	a.pushUndoLocked(charIdx)
	report, err := core.RepairDuplicateInventoryIndices(slot)
	if err != nil {
		return empty, err
	}
	if _, err := core.RepairDuplicateWondrousPhysick(slot); err != nil {
		return empty, err
	}
	if post := core.ScanDuplicateInventoryIndices(slot); len(post) > 0 {
		return report, fmt.Errorf("RepairDuplicateInventoryIndices: %d duplicate(s) remain after repair", len(post))
	}
	if post := core.ScanDuplicateWondrousPhysick(slot); len(post) > 0 {
		return report, fmt.Errorf("RepairDuplicateInventoryIndices: %d Flask of Wondrous Physick record(s) remain after repair", len(post))
	}
	return report, nil
}

// Dummy method to force Wails to export types
func (a *App) _forceExportTypes() (db.GraceEntry, db.BossEntry, db.ItemEntry, db.MapEntry, db.CookbookEntry, db.GestureEntry, db.QuestNPC, db.QuestStep, db.QuestFlagState, core.SlotDiagnostics, core.DiagnosticIssue, DiffEntry, SlotDiffSummary, SlotCapacity, deploy.Target, PresetInfo, FavoriteSlotInfo, db.BellBearingEntry, db.WhetbladeEntry, core.NetworkParamValues, vm.CharacterPreset, vm.PresetItem, vm.ApplyOptions, vm.PresetApplyResult, vm.WorldPresetData, PvPPreparationOptions, vm.AoWAvailabilityEntry, BuiltinCharacterPresetInfo, InventoryOrderItem, core.TransferResult, core.TransferSkip, core.InventoryIndexRepairReport, core.InventoryIndexRepairChange, SaveInventoryIntegrityReport, SlotInventoryIntegrityReport, InventoryIntegrityConflict, InventoryIntegrityConflictItem) {
	return db.GraceEntry{}, db.BossEntry{}, db.ItemEntry{}, db.MapEntry{}, db.CookbookEntry{}, db.GestureEntry{}, db.QuestNPC{}, db.QuestStep{}, db.QuestFlagState{}, core.SlotDiagnostics{}, core.DiagnosticIssue{}, DiffEntry{}, SlotDiffSummary{}, SlotCapacity{}, deploy.Target{}, PresetInfo{}, FavoriteSlotInfo{}, db.BellBearingEntry{}, db.WhetbladeEntry{}, core.NetworkParamValues{}, vm.CharacterPreset{}, vm.PresetItem{}, vm.ApplyOptions{}, vm.PresetApplyResult{}, vm.WorldPresetData{}, PvPPreparationOptions{}, vm.AoWAvailabilityEntry{}, BuiltinCharacterPresetInfo{}, InventoryOrderItem{}, core.TransferResult{}, core.TransferSkip{}, core.InventoryIndexRepairReport{}, core.InventoryIndexRepairChange{}, SaveInventoryIntegrityReport{}, SlotInventoryIntegrityReport{}, InventoryIntegrityConflict{}, InventoryIntegrityConflictItem{}
}
