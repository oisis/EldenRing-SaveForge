package main

import (
	"context"
	"encoding/binary"
	"fmt"
	"os"
	"sort"
	"strconv"
	"strings"
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
	Active         bool
	ProfileSummary core.ProfileSummary
	Slot           core.SlotSnapshot
}

// App struct
type App struct {
	ctx          context.Context
	save         *core.SaveFile
	undoStacks   [10][]slotSnapshot
	lastSavePath string

	// journal is the durable per-session diagnostic log. It is nil in
	// tests and headless runs (NewApp leaves it unset so existing
	// fixtures never create a real session file); production main.go
	// opens a session journal and assigns it before wails.Run. Every
	// access goes through journalLog, which no-ops on a nil journal.
	journal *DiagnosticJournal

	// GaItem repack tokens bind a dry-run to one loaded save and an exact slot
	// state. They are protected by saveMu + the relevant slotMu entry.
	gaItemRepackTokens map[string]gaItemRepackToken
	gaItemDedupTokens  map[string]gaItemDedupToken
	gaItemRepackNextID uint64
	saveGeneration     uint64
	slotRevisions      [maxCharacters]uint64
	deployStore        *deploy.TargetStore
	deploySSH          *deploy.SSHManager
	deployLocal        *deploy.LocalManager
	favSlotNames       map[int]string // preset name written to each Favorites slot; empty = loaded from save (unknown)

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
	// favSlotNames concurrent-map-writes between favorites endpoints).
	//
	// Lock order — taken in increasing order, released in reverse — is:
	//   0. diagnosticScopeMu      (diagnostic scope-marker linearity — outermost)
	//   1. saveMu                 (whole-save lifecycle + metadata)
	//   2. lifecycleMu[i]         (session lifecycle per slot — existing)
	//   3. editSessionsMu         (session registry, short — existing)
	//   4. sess.mu                (per-session workspace state — existing)
	//   5. favMu                  (UserData10 + favSlotNames)
	//   6. slotMu[i]              (per-slot data, ascending i)
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
	// diagnosticScopeMu serialises the diagnostic current-save scope
	// transitions (commitLoadedSave, CloseSave) so the final a.save state and
	// the last save_loaded/save_closed marker can never diverge under
	// concurrent load/close. It is the OUTERMOST lock (level 0): a transition
	// holds it across the whole sequence — take saveMu, mutate a.save, release
	// saveMu, append the scope marker — so two transitions can never interleave
	// their (state change, marker) pairs. It is acquired ONLY by those two
	// methods and ALWAYS before saveMu, so no reverse-lock is possible; the
	// marker append (journal j.mu, a leaf) runs after saveMu is released, so
	// journal Sync never happens under saveMu.
	diagnosticScopeMu sync.Mutex
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
}

// NewApp creates a new App struct
func NewApp() *App {
	return &App{
		favSlotNames:       make(map[int]string),
		editSessions:       make(map[string]*editor.InventoryEditSession),
		editSessionByChar:  make(map[int]string),
		gaItemRepackTokens: make(map[string]gaItemRepackToken),
		gaItemDedupTokens:  make(map[string]gaItemDedupToken),
	}
}

// startup is called when the app starts. The context is saved
// so we can call the runtime methods
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
	a.journalLog(levelInfo, "startup", "app startup begin")

	store, err := deploy.NewTargetStore()
	if err != nil {
		fmt.Printf("Warning: deploy targets unavailable: %v\n", err)
		a.journalLog(levelWarn, "deploy_store_unavailable", "deploy targets unavailable")
		return
	}
	a.deployStore = store
	a.deploySSH = deploy.NewSSHManager(store)
	a.deployLocal = deploy.NewLocalManager(store)
	a.journalLog(levelInfo, "startup", "app startup complete")
}

// shutdown is wired to options.App.OnShutdown. It writes the
// session_closed record and closes the journal on a clean exit; a crash
// skips this path, naturally leaving no session_closed marker. A journal
// close failure is reported to stderr and never blocks shutdown.
func (a *App) shutdown(_ context.Context) {
	if err := a.journal.Close(); err != nil {
		fmt.Fprintf(os.Stderr, "diagnostic journal close: %v\n", err)
	}
}

// journalLog appends one app-sourced diagnostic record. It degrades to
// stderr on a journal write failure and no-ops when no journal is
// attached, so callers never need to guard the journal or handle its
// errors. Values must already be sanitized technical fields — never raw
// save bytes, Steam IDs, credentials, SSH data, tokens, or full paths.
func (a *App) journalLog(level diagnosticLevel, event, message string, fields ...diagnosticField) {
	a.journalLogFrom(level, sourceApp, event, message, fields...)
}

// journalLogFrom is journalLog with an explicit producer source. Only trusted
// in-process producers use it; the frontend bridge supplies a fixed source and
// cannot choose its own event name (see RecordDiagnosticClientError).
func (a *App) journalLogFrom(level diagnosticLevel, source, event, message string, fields ...diagnosticField) {
	if err := a.journal.Log(level, source, event, message, fields...); err != nil {
		fmt.Fprintf(os.Stderr, "diagnostic journal (%s %s): %v\n", source, event, err)
	}
}

// journalDebug records an optional, structured operation detail. DiagnosticJournal
// owns the verbosity policy and drops this call before seq/file/tail work unless
// Debug Mode is enabled; callers must still pass only privacy-safe metadata.
func (a *App) journalDebug(event, message string, fields ...diagnosticField) {
	a.journalLog(levelDebug, event, message, fields...)
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
	a.journalDebug("save_load_requested", "save load requested", field("origin", string(loadOriginFileDialog)))
	path, err := runtime.OpenFileDialog(a.ctx, runtime.OpenDialogOptions{
		Title: "Select Elden Ring Save File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Elden Ring Save (*.sl2;*.dat;*.txt)", Pattern: "*.sl2;*.dat;*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		a.journalDebug("save_load_failed", "save load failed", field("origin", string(loadOriginFileDialog)), field("stage", "dialog"))
		return "", err
	}
	if path == "" {
		a.journalDebug("save_load_cancelled", "save load cancelled", field("origin", string(loadOriginFileDialog)))
		return "", fmt.Errorf("no file selected")
	}

	save, err := core.LoadSave(path)
	if err != nil {
		a.journalDebug("save_load_failed", "save load failed", field("origin", string(loadOriginFileDialog)), field("stage", "parse"))
		return "", err
	}
	a.journalDebug("save_load_parsed", "save load parsed", field("origin", string(loadOriginFileDialog)), field("platform", string(save.Platform)))
	// Commit phase: exclusive saveMu blocks every reader/writer of the old
	// a.save while the pointer swap + derived-state reset run. installLoadedSave
	// also resets favSlotNames; favMu is intentionally NOT taken here — every
	// favorites endpoint enters via saveMu.RLock first, so saveMu.Lock alone
	// guarantees no favorites endpoint is mid-flight. commitLoadedSave appends
	// the save_loaded scope marker after the lock is released.
	a.commitLoadedSave(save, path, loadOriginFileDialog)
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
	a.saveGeneration++
	a.slotRevisions = [maxCharacters]uint64{}
	a.gaItemRepackTokens = make(map[string]gaItemRepackToken)
	a.gaItemDedupTokens = make(map[string]gaItemDedupToken)
	a.favSlotNames = make(map[int]string)
	a.clearAllUndoStacks()
	a.clearAllEditSessions()
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

	// Atomic rollback point captured before any mutation: a failed container
	// sync must not leave an increased pot/perfume without its required Key
	// Items container.
	containerRollback := core.SnapshotSlot(&a.save.Slots[index])

	// Plan the in-scope direct profile/attribute field changes and the Memory
	// Stones semantic fields against the pre-mutation slot, then emit all before
	// records followed by all planned records BEFORE the writer touches the
	// slot. Only fields whose normalized target differs are logged.
	plans := planCharacterSaveChanges(a.save.Slots[index].Player, &charVM)
	msPlans := planMemoryStonesSaveChanges(&a.save.Slots[index], charVM.MemoryStones)
	sePlans := a.planClearCountSideEffects(index, &charVM)
	iqPlans := planItemQuantityChanges(&a.save.Slots[index], &charVM)
	ccPlans := a.planContainerSideEffects(index, &charVM)
	if len(plans) > 0 || len(msPlans) > 0 || len(sePlans) > 0 || len(iqPlans) > 0 || len(ccPlans) > 0 {
		records := append(plannedChangeRecords(plans), memoryStonesPlannedRecords(msPlans)...)
		records = append(records, sideEffectPlannedRecords(sePlans)...)
		records = append(records, itemQuantityPlannedRecords(iqPlans)...)
		records = append(records, containerPlannedRecords(ccPlans)...)
		a.journalCharacterChangeBefore(actionSaveCharacter, index, records)
		a.journalCharacterChangePlanned(actionSaveCharacter, index, records)
	}

	stage, err := func() (string, error) {
		// 1. Update the slot data
		if err := vm.ApplyVMToParsedSlot(&charVM, &a.save.Slots[index]); err != nil {
			return characterStageApplyVM, err
		}

		slot := &a.save.Slots[index]

		// 1b. Keep pot/perfume containers in sync with the edited Inventory/Storage
		// quantities (Character → Inventory → Save Changes). Roll the whole slot back
		// if the container cannot be written.
		if err := syncContainerKeyItems(slot); err != nil {
			core.RestoreSlot(slot, containerRollback)
			return characterStageSyncContainers, fmt.Errorf("SaveCharacter: sync containers: %w", err)
		}

		if err := a.applyMemoryStonesToSlot(slot, charVM.MemoryStones); err != nil {
			return characterStageMemoryStones, err
		}

		// Flush slot.Player → slot.Data so that subsequent operations
		// (AddItemsToSlotBatch, RebuildSlotFull) that re-parse slot.Data
		// see the correct stats instead of the pre-edit binary values.
		slot.SyncPlayerToData()

		// 2. Sync NG+ event flags (50-57) with ClearCount
		if hasEventFlagsRegion(slot) {
			flags := slot.Data[slot.EventFlagsOffset:]
			for i := uint32(0); i <= 7; i++ {
				_ = db.SetEventFlag(flags, 50+i, i == slot.Player.ClearCount)
			}
		}

		// 3. Update ProfileSummary (for the menu)
		a.save.ProfileSummaries[index].Level = a.save.Slots[index].Player.Level
		copy(a.save.ProfileSummaries[index].CharacterName[:], a.save.Slots[index].Player.CharacterName[:])

		return characterStageCompleted, nil
	}()

	// Emit finished records for every planned field using the field values that
	// actually landed in the slot at this exit point (post-rollback on failure).
	if len(plans) > 0 || len(msPlans) > 0 || len(sePlans) > 0 || len(iqPlans) > 0 || len(ccPlans) > 0 {
		outcome := characterChangeSuccess
		if err != nil {
			outcome = characterChangeError
		}
		finished := append(finishedChangeRecords(plans, a.save.Slots[index].Player),
			memoryStonesFinishedRecords(msPlans, &a.save.Slots[index])...)
		finished = append(finished, sideEffectFinishedRecords(sePlans)...)
		finished = append(finished, itemQuantityFinishedRecords(iqPlans, &a.save.Slots[index])...)
		finished = append(finished, containerFinishedRecords(ccPlans, &a.save.Slots[index])...)
		a.journalCharacterChangeFinished(actionSaveCharacter, index, outcome, stage, finished)
	}

	return err
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
func (a *App) WriteSave() error {
	a.saveMu.RLock()
	if a.save == nil {
		a.saveMu.RUnlock()
		a.journalLog(levelError, "save_write_failed", "save write failed", field("stage", "no_active_save"))
		return fmt.Errorf("no save loaded")
	}
	expected := a.save
	platform := string(a.save.Platform)
	a.saveMu.RUnlock()
	a.journalLog(levelInfo, "save_write_requested", "save write requested", field("platform", platform))

	path, err := runtime.SaveFileDialog(a.ctx, runtime.SaveDialogOptions{
		Title: "Save Elden Ring Save File",
		Filters: []runtime.FileFilter{
			{DisplayName: "Elden Ring Save (*.sl2;*.dat;*.txt)", Pattern: "*.sl2;*.dat;*.txt"},
			{DisplayName: "All Files (*.*)", Pattern: "*.*"},
		},
	})
	if err != nil {
		a.journalLog(levelError, "save_write_failed", "save write failed", field("stage", "dialog"))
		return err
	}
	if path == "" {
		a.journalLog(levelInfo, "save_write_cancelled", "save write cancelled")
		return fmt.Errorf("no file selected")
	}

	backupCreated := false
	// Backup only when the target file already exists (nothing to protect otherwise).
	if _, statErr := os.Stat(path); statErr == nil {
		if _, err := core.CreateBackup(path); err != nil {
			a.journalLog(levelError, "save_write_failed", "save write failed", field("stage", "backup"))
			return fmt.Errorf("backup failed, save aborted: %w", err)
		}
		backupCreated = true
		a.journalLog(levelInfo, "save_write_backup_created", "save backup created")
		if err := core.PruneBackups(path, 10); err != nil {
			fmt.Printf("Warning: failed to prune old backups: %v\n", err)
		}
	}

	snapshot, err := a.writeSaveCore(path, expected)
	if len(snapshot) > 0 {
		a.journalDebug(eventSaveStateBeforeWrite, "privacy-safe save state captured before file write",
			diagnosticSnapshotForSerialization(snapshot, "file")...)
	}
	if err != nil {
		a.journalLog(levelError, "save_write_failed", "save write failed", field("stage", "write"))
		return err
	}
	fields := []diagnosticField{
		field("platform", platform),
		field("backup_created", strconv.FormatBool(backupCreated)),
	}
	if name := safeSaveFileName(path); name != "" {
		fields = append(fields, field("save_file", name))
	}
	a.journalLog(levelInfo, "save_write_finished", "save write finished", fields...)
	a.journalDebug(eventSaveStateWritten, "privacy-safe save state captured after successful file write",
		diagnosticSnapshotForSerialization(snapshot, "file")...)
	return nil
}

// writeSaveCore serializes a.save to disk in its current platform and resets
// undo state + lastSavePath. Cross-platform conversion is handled separately
// by ExecuteConversion (app_convert.go), which operates on a local copy and
// never touches a.save.
//
// Acquires a.saveMu.Lock() for the entire critical section. Caller MUST
// have performed dialog / backup work BEFORE invoking this helper, and
// MUST pass `expected` — the *core.SaveFile snapshot taken before the
// dialog. When a concurrent SelectAndOpenSave / DownloadRemoteSave
// replaced a.save during the dialog wait, the identity check fails fast:
// no file is written, the helper returns a user-facing error instead of
// silently writing save B to a path picked for save A. Its first result is a
// privacy-safe snapshot captured immediately before serialization; callers
// must append it only after this method has released saveMu.
func (a *App) writeSaveCore(path string, expected *core.SaveFile) ([]diagnosticField, error) {
	a.saveMu.Lock()
	defer a.saveMu.Unlock()

	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if a.save != expected {
		return nil, fmt.Errorf("active save changed while the save dialog was open; write to %q aborted to avoid overwriting it with the wrong save", path)
	}

	snapshot := diagnosticSaveSnapshot(a.save, a.saveGeneration)
	if err := a.save.SaveFile(path); err != nil {
		return snapshot, err
	}
	a.lastSavePath = path
	a.clearAllUndoStacks()
	return snapshot, nil
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
	Added           int          `json:"added"`
	Requested       int          `json:"requested"`
	Trimmed         []SkippedAdd `json:"trimmed"`
	SkippedExisting []SkippedAdd `json:"skippedExisting"`
	CapHit          string       `json:"capHit"`
	FreeInv         int          `json:"freeInv"`
	FreeStore       int          `json:"freeStore"`
	NeededInv       int          `json:"neededInv"`
	NeededStore     int          `json:"neededStore"`
	// GaItem placement capacity (weapons/armor/AoW). FreeGaItems is the smaller
	// of total empty records and allocator cursor room; NeededGaItems is how many
	// the batch requires — surfaced so the UI can explain a gaitem_full failure.
	FreeGaItems   int `json:"freeGaItems"`
	NeededGaItems int `json:"neededGaItems"`
	// GaItemCapacity and GaItemRepackCTA are present only for a gaitem_full
	// rejection. They carry the backend's raw capacity breakdown and repack
	// eligibility decision so the frontend never derives repack safety itself.
	GaItemCapacity  *GaItemCapacity  `json:"gaItemCapacity,omitempty"`
	GaItemRepackCTA *GaItemRepackCTA `json:"gaItemRepackCTA,omitempty"`
}

const diagnosticItemListMax = 20

// diagnosticItemList resolves only canonical, public game-database names for
// the diagnostic journal. It deliberately never reads item data from the save;
// unknown IDs remain useful technical clues without exposing save content.
func diagnosticItemList(itemIDs []uint32) string {
	items := make([]string, 0, min(len(itemIDs), diagnosticItemListMax))
	for _, id := range itemIDs[:min(len(itemIDs), diagnosticItemListMax)] {
		item, _ := db.GetItemDataFuzzy(id)
		if item.Name == "" {
			items = append(items, fmt.Sprintf("Unknown item (0x%08X)", id))
			continue
		}
		items = append(items, item.Name)
	}
	if extra := len(itemIDs) - len(items); extra > 0 {
		items = append(items, fmt.Sprintf("… +%d more", extra))
	}
	return strings.Join(items, ", ")
}

func diagnosticAddDestination(inventoryQty, storageQty int) string {
	switch {
	case inventoryQty > 0 && storageQty > 0:
		return "inventory + storage"
	case inventoryQty > 0:
		return "inventory"
	case storageQty > 0:
		return "storage"
	default:
		return "none"
	}
}

type diagnosticAddedItem struct {
	id                uint32
	inventoryQty      int
	storageQty        int
	keyItemsTargetQty int
}

// diagnosticAddedItemList reports the quantities the add path actually
// prepared for each location. It is assembled from transient operation state,
// not from save bytes, and is bounded to keep a large batch readable.
func diagnosticAddedItemList(items []diagnosticAddedItem) string {
	parts := make([]string, 0, min(len(items), diagnosticItemListMax))
	accepted := 0
	for _, item := range items {
		if item.inventoryQty == 0 && item.storageQty == 0 && item.keyItemsTargetQty == 0 {
			continue
		}
		accepted++
		if len(parts) == diagnosticItemListMax {
			continue
		}
		itemData, _ := db.GetItemDataFuzzy(item.id)
		name := itemData.Name
		if name == "" {
			name = fmt.Sprintf("Unknown item (0x%08X)", item.id)
		}
		locations := make([]string, 0, 3)
		if item.inventoryQty > 0 {
			locations = append(locations, fmt.Sprintf("inventory=%d", item.inventoryQty))
		}
		if item.storageQty > 0 {
			locations = append(locations, fmt.Sprintf("storage=%d", item.storageQty))
		}
		if item.keyItemsTargetQty > 0 {
			locations = append(locations, fmt.Sprintf("key items target=%d", item.keyItemsTargetQty))
		}
		parts = append(parts, fmt.Sprintf("%s (%s)", name, strings.Join(locations, ", ")))
	}
	if extra := accepted - len(parts); extra > 0 {
		parts = append(parts, fmt.Sprintf("… +%d more", extra))
	}
	if len(parts) == 0 {
		return "none"
	}
	return strings.Join(parts, "; ")
}

func diagnosticContainerList(containers map[uint32]bool) string {
	ids := make([]uint32, 0, len(containers))
	for id := range containers {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	return diagnosticItemList(ids)
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
	return a.addItemsToCharacter(charIdx, itemIDs, upgrade25, upgrade10, infuseOffset, upgradeAsh, invQty, storageQty, false)
}

// AddItemsToCharacterWithGameLimits is the Full Chaos Mode add path. It uses
// regulation-derived technical game limits where known and falls back to the
// conservative Normal Mode cap where game-limit data is unavailable.
func (a *App) AddItemsToCharacterWithGameLimits(charIdx int, itemIDs []uint32, upgrade25, upgrade10, infuseOffset, upgradeAsh, invQty, storageQty int) (AddResult, error) {
	return a.addItemsToCharacter(charIdx, itemIDs, upgrade25, upgrade10, infuseOffset, upgradeAsh, invQty, storageQty, true)
}

func (a *App) addItemsToCharacter(charIdx int, itemIDs []uint32, upgrade25, upgrade10, infuseOffset, upgradeAsh, invQty, storageQty int, useGameLimits bool) (result AddResult, retErr error) {
	result = AddResult{Requested: len(itemIDs)}
	requestedItems := diagnosticItemList(itemIDs)
	requestedDestination := diagnosticAddDestination(invQty, storageQty)
	actualItems := "none"
	containersUpdated := "none"
	a.journalDebug("items_add_requested", "item add requested",
		field("character_index", strconv.Itoa(charIdx)),
		field("requested_count", strconv.Itoa(len(itemIDs))),
		field("requested_items", requestedItems),
		field("requested_destination", requestedDestination),
		field("inventory_quantity", strconv.Itoa(invQty)),
		field("storage_quantity", strconv.Itoa(storageQty)),
		field("game_limits", strconv.FormatBool(useGameLimits)))

	a.saveMu.RLock()
	defer func() {
		a.saveMu.RUnlock()
		outcome := "success"
		if retErr != nil {
			outcome = "error"
		} else if result.CapHit != "" {
			outcome = "capacity_rejected"
		} else if result.Added == 0 {
			outcome = "no_change"
		}
		resultItems := "none"
		updatedContainers := "none"
		if retErr == nil && result.CapHit == "" && result.Added > 0 {
			resultItems = actualItems
			updatedContainers = containersUpdated
		}
		a.journalDebug("items_add_finished", "item add finished",
			field("character_index", strconv.Itoa(charIdx)),
			field("outcome", outcome),
			field("result_items", resultItems),
			field("containers_updated", updatedContainers),
			field("added", strconv.Itoa(result.Added)),
			field("trimmed", strconv.Itoa(len(result.Trimmed))),
			field("skipped_existing", strconv.Itoa(len(result.SkippedExisting))),
			field("capacity_hit", result.CapHit))
	}()
	if a.save == nil {
		return result, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return result, fmt.Errorf("invalid character index")
	}

	// Probe the inventory-workspace registry BEFORE taking slotMu: editSessionsMu
	// (#3) precedes slotMu (#6) in the lock order, so it must not be acquired while
	// slotMu is held. Because the probe runs outside slotMu, a workspace started
	// concurrently after it returns can leave the CTA computed below stale (reported
	// as eligible when a live workspace now exists). That is tolerated here: this
	// path only reports the CTA and never mutates the slot on it, and
	// AnalyzeGaItemRepack re-checks workspace state under its own lifecycle guard
	// before performing any optimization.
	workspaceActive := a.gaItemRepackHasActiveWorkspaceLocked(charIdx)

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
	// Crimson/Cerulean Tears flasks are one-per-family (MaxInventory 1) but every
	// upgrade level is its own DB row, so existingItemQty keys a +12 flask under
	// 0x40000401 — not the +0 picker row 0x400003E9. Track which flask families
	// the save already holds (any level, inventory or storage) so adding the +0
	// base can be skipped instead of creating a duplicate logical flask.
	existingFlaskFamily := make(map[uint32]bool)
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
		if fam, ok := db.FlaskFamilyBaseID(invID); ok {
			existingFlaskFamily[fam] = true
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
		if fam, ok := db.FlaskFamilyBaseID(sID); ok {
			existingFlaskFamily[fam] = true
		}
	}

	// Existing container key item qtys (so we don't lower them). existingKeyItemRow
	// records the physical KeyItems row per base ID so a stackable already-owned
	// key item (Larval Tear, Lost Ashes of War) can have its stack bumped in place
	// for Full Chaos / Game Max instead of being skipped (issue 7).
	existingKeyItemQty := make(map[uint32]int)
	existingKeyItemRow := make(map[uint32]int)
	for i, item := range slot.Inventory.KeyItems {
		if item.GaItemHandle == 0 || item.GaItemHandle == 0xFFFFFFFF {
			continue
		}
		keyID := db.HandleToItemID(item.GaItemHandle)
		_, keyBaseID := db.GetItemDataFuzzy(keyID)
		if _, seen := existingItemQty[keyBaseID]; seen {
			continue // ponytail: item in both sections — CommonItems is canonical, skip double-count
		}
		existingKeyItemQty[keyBaseID] = int(item.Quantity & 0x7FFFFFFF)
		existingKeyItemRow[keyBaseID] = i
	}

	containerCap := func(c uint32) int {
		cData, _ := db.GetItemDataFuzzy(c)
		maxInventory, _ := addItemCaps(cData, useGameLimits)
		return int(maxInventory)
	}

	// Track containers touched by this batch (need auto-update of qty).
	usedContainers := make(map[uint32]bool)
	// Containers required only because a gated item was added to Storage. Storage
	// has no cap and does not consume per-unit containers, but the game still
	// requires at least one container in Key Items to hold the family at all.
	storageContainers := make(map[uint32]bool)

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
		keyRow         int // KeyItems row to bump in place (issue 7), -1 if none
		keyTargetQty   int // target stack qty for that row, 0 if no update
	}
	var prepared []preparedItem
	var trimmed []SkippedAdd
	var skippedExisting []SkippedAdd

	for _, id := range sortedIDs {
		isPhysick := db.IsWondrousPhysick(id)
		if isPhysick {
			id = db.ItemFlaskWondrousPhysickEmpty
		}
		itemData, _ := db.GetItemDataFuzzy(id)
		// A Crimson/Cerulean flask family the save already holds (any level) must
		// not gain a second copy from the +0 picker row. Reuse the family key so
		// there's no duplicated flask ID math; report it via the normal skip list.
		if fam, ok := db.FlaskFamilyBaseID(id); ok && existingFlaskFamily[fam] {
			a.logInfo("flask family already present — skipping %s (0x%08X)", itemData.Name, id)
			skippedExisting = append(skippedExisting, SkippedAdd{ItemID: id})
			continue
		}
		maxInventory, maxStorage := addItemCaps(itemData, useGameLimits)
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

		actualInv := resolveQty(invQty, int(maxInventory))
		actualStorage := resolveQty(storageQty, int(maxStorage))
		if isPhysick {
			actualStorage = 0
			if hasPhysick {
				actualInv = 0
			} else if actualInv > 0 {
				hasPhysick = true
			}
		}

		// Skip stackable items already at max qty. Talismans (0xA0) are handle-encoded
		// but NOT stackable: each copy is a distinct record, so they must bypass the
		// "already at max qty" skip and the per-copy cap-to-1 collapse.
		handlePrefix := db.ItemIDToHandlePrefix(finalID)
		isStackable := handlePrefix == core.ItemTypeItem || db.IsArrowID(finalID)
		keyRow := -1
		keyTargetQty := 0
		if isStackable {
			if (actualInv > 0 || actualStorage > 0) && existingKeyItemQty[id] > 0 {
				// Item already lives in the game-managed KeyItems section. If the
				// requested target exceeds the current stack, bump that existing row
				// in place (issue 7: Full Chaos / Game Max on stackable key items like
				// Larval Tear or Lost Ashes of War). Otherwise keep the historical
				// skip. Never route into CommonItems; that duplicates the record.
				if actualInv > existingKeyItemQty[id] {
					keyRow = existingKeyItemRow[id]
					keyTargetQty = actualInv
				} else {
					a.logInfo("already in Key Items — skipping %s (0x%08X)", itemData.Name, id)
					skippedExisting = append(skippedExisting, SkippedAdd{ItemID: id})
				}
				actualInv = 0
				actualStorage = 0
			}
			if actualInv > 0 && existingItemQty[id] >= int(maxInventory) {
				a.logInfo("already max inv qty %d/%d — skipping %s (0x%08X)", existingItemQty[id], maxInventory, itemData.Name, id)
				actualInv = 0
			}
			if actualStorage > 0 && existingStorageQty[id] >= int(maxStorage) {
				a.logInfo("already max storage qty %d/%d — skipping %s (0x%08X)", existingStorageQty[id], maxStorage, itemData.Name, id)
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
			if actualStorage > 0 {
				storageContainers[cID] = true
			}
		}

		forceStackable := db.IsArrowID(finalID)

		prepared = append(prepared, preparedItem{
			baseID:         id,
			finalID:        finalID,
			actualInv:      actualInv,
			actualStorage:  actualStorage,
			forceStackable: forceStackable,
			keyRow:         keyRow,
			keyTargetQty:   keyTargetQty,
		})
	}
	diagnosticItems := make([]diagnosticAddedItem, 0, len(prepared))
	for _, item := range prepared {
		diagnosticItems = append(diagnosticItems, diagnosticAddedItem{
			id:                item.finalID,
			inventoryQty:      item.actualInv,
			storageQty:        item.actualStorage,
			keyItemsTargetQty: item.keyTargetQty,
		})
	}
	actualItems = diagnosticAddedItemList(diagnosticItems)
	containersUpdated = diagnosticContainerList(usedContainers)

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
		})
	}
	// In-place KeyItems stack bumps (issue 7) do not consume new slots, so they
	// are work even when nothing routes through the batch adder.
	hasKeyUpdates := false
	for _, p := range prepared {
		if p.keyTargetQty > 0 {
			hasKeyUpdates = true
			break
		}
	}

	if len(capacityItems) == 0 && !hasKeyUpdates {
		finalUsage := core.CountSlotUsage(slot)
		result.Trimmed = trimmed
		result.SkippedExisting = skippedExisting
		result.FreeInv = finalUsage.InventoryMax - finalUsage.InventoryUsed
		result.FreeStore = finalUsage.StorageMax - finalUsage.StorageUsed
		return result, nil
	}

	if len(capacityItems) > 0 {
		capReport := core.CheckAddCapacity(slot, capacityItems)
		if !capReport.CanFitAll {
			freeGaItems := capReport.FreeGaItems
			if capReport.FreeGaItemCursor < freeGaItems {
				freeGaItems = capReport.FreeGaItemCursor
			}
			a.logError("[AddItems] %s: need inv=%d store=%d gaitem=%d, free inv=%d store=%d gaitem=%d (empty=%d cursor=%d), requested=%d",
				capReport.CapHit, capReport.NeededInv, capReport.NeededStorage, capReport.NeededGaItems,
				capReport.FreeInv, capReport.FreeStorage, freeGaItems, capReport.FreeGaItems, capReport.FreeGaItemCursor, len(itemIDs))
			result.CapHit = capReport.CapHit
			result.FreeInv = capReport.FreeInv
			result.FreeStore = capReport.FreeStorage
			result.NeededInv = capReport.NeededInv
			result.NeededStore = capReport.NeededStorage
			result.FreeGaItems = freeGaItems
			result.NeededGaItems = capReport.NeededGaItems
			if capReport.CapHit == "gaitem_full" {
				capacity, cta := gaItemFullCTA(slot, capReport, workspaceActive)
				result.GaItemCapacity = &capacity
				result.GaItemRepackCTA = &cta
			}
			return result, nil
		}
	}

	// MUTATE: build the mutation plan once. The plan captures every already-
	// computed input the mutation tail consumes, so the diagnostic planner can
	// apply the identical plan to core.CloneSlot before the real executor runs.
	planItems := make([]itemAddPlanItem, 0, len(prepared))
	for _, p := range prepared {
		planItems = append(planItems, itemAddPlanItem{
			baseID:        p.baseID,
			actualInv:     p.actualInv,
			actualStorage: p.actualStorage,
			keyRow:        p.keyRow,
			keyTargetQty:  p.keyTargetQty,
		})
	}
	plan := itemAddMutationPlan{
		batch:               capacityItems,
		items:               planItems,
		usedContainers:      usedContainers,
		storageContainers:   storageContainers,
		existingByContainer: existingByContainer,
	}

	// DIAGNOSTICS: apply the identical plan to a clone and capture every changed
	// direct core record. before values come from the still-unmutated real slot,
	// planned values from the clone. Emitted before pushUndoLocked and the real
	// executor; only built in Debug Mode, since the clone add duplicates the whole
	// mutation and is pure overhead otherwise.
	var giPlans []gameItemFieldPlan
	var giContainerPlans []containerFieldPlan
	if a.journal.debugEnabled() {
		clone := core.CloneSlot(slot)
		_ = applyItemAddMutationPlan(clone, plan, func(string, ...any) {})
		giPlans = planGameItemsAddRecords(slot, clone, plan)
		giContainerPlans = planGameItemsAddContainerRecords(slot, clone, plan)
		// Direct physical rows first, then container side effects, kept in the
		// same global phase grouping: all before -> all planned -> all finished.
		records := append(gameItemPlannedRecords(giPlans), containerPlannedRecords(giContainerPlans)...)
		a.journalGameItemChangeBefore(actionGameItemsAdd, charIdx, records)
		a.journalGameItemChangePlanned(actionGameItemsAdd, charIdx, records)
	}

	// SNAPSHOT: deep copy slot state before mutation.
	a.pushUndoLocked(charIdx)
	snapshot := core.SnapshotSlot(slot)

	// MUTATE: apply the plan to the real slot. Any executor failure restores the
	// pre-mutation snapshot here; the executor itself owns no locking, undo,
	// journal or rollback.
	if err := applyItemAddMutationPlan(slot, plan, func(format string, args ...any) {
		runtime.LogWarningf(a.ctx, format, args...)
	}); err != nil {
		core.RestoreSlot(slot, snapshot)
		if len(giPlans) > 0 || len(giContainerPlans) > 0 {
			finished := append(gameItemFinishedRecords(giPlans, slot), containerFinishedRecords(giContainerPlans, slot)...)
			a.journalGameItemChangeFinished(actionGameItemsAdd, charIdx, characterChangeError, stageGameItemsApplyAddPlan, finished)
		}
		return result, err
	}
	if len(giPlans) > 0 || len(giContainerPlans) > 0 {
		finished := append(gameItemFinishedRecords(giPlans, slot), containerFinishedRecords(giContainerPlans, slot)...)
		a.journalGameItemChangeFinished(actionGameItemsAdd, charIdx, characterChangeSuccess, characterStageCompleted, finished)
	}

	// SUCCESS: compute final capacity and return.
	finalUsage := core.CountSlotUsage(slot)
	added := 0
	for _, p := range prepared {
		if p.actualInv > 0 || p.actualStorage > 0 || p.keyTargetQty > 0 {
			added++
		}
	}
	result.Added = added
	result.Trimmed = trimmed
	result.SkippedExisting = skippedExisting
	result.FreeInv = finalUsage.InventoryMax - finalUsage.InventoryUsed
	result.FreeStore = finalUsage.StorageMax - finalUsage.StorageUsed
	return result, nil
}

// setInventoryKeyItemQuantity sets an existing KeyItems record's stack quantity
// in memory and in the binary slot.Data. The held-inventory KeyItems section is a
// full KeyItemCount array, so the slice row maps directly to the binary record at
// row*InvRecordLen; the quantity field sits at +4 within that record. Mirrors the
// stackable-update branch of core.addToInventory, which only covers CommonItems.
func setInventoryKeyItemQuantity(slot *core.SaveSlot, row int, qty uint32) error {
	if row < 0 || row >= len(slot.Inventory.KeyItems) {
		return fmt.Errorf("setInventoryKeyItemQuantity: row %d out of range (%d key items)", row, len(slot.Inventory.KeyItems))
	}
	off := slot.MagicOffset + core.InvStartFromMagic +
		core.CommonItemCount*core.InvRecordLen + core.InvKeyCountHeader +
		row*core.InvRecordLen + 4
	if off < 4 || off+4 > len(slot.Data) {
		return fmt.Errorf("setInventoryKeyItemQuantity: quantity offset %d out of bounds (data len %d)", off, len(slot.Data))
	}
	binary.LittleEndian.PutUint32(slot.Data[off:], qty)
	slot.Inventory.KeyItems[row].Quantity = qty
	return nil
}

func setInventoryCommonItemQuantity(slot *core.SaveSlot, row int, qty uint32) error {
	if row < 0 || row >= len(slot.Inventory.CommonItems) {
		return fmt.Errorf("setInventoryCommonItemQuantity: row %d out of range (%d common items)", row, len(slot.Inventory.CommonItems))
	}
	off := slot.MagicOffset + core.InvStartFromMagic + row*core.InvRecordLen + 4
	if off < 4 || off+4 > len(slot.Data) {
		return fmt.Errorf("setInventoryCommonItemQuantity: quantity offset %d out of bounds (data len %d)", off, len(slot.Data))
	}
	binary.LittleEndian.PutUint32(slot.Data[off:], qty)
	slot.Inventory.CommonItems[row].Quantity = qty
	return nil
}

// inventoryContainerQuantity returns the largest physical quantity for the
// container. Normal game saves place these goods records in CommonItems. The
// KeyItems lookup preserves compatibility with saves written by versions
// 1.3.3–1.5.1, which could have created a second legacy record there.
func inventoryContainerQuantity(slot *core.SaveSlot, itemID uint32) int {
	handle := db.ItemIDToHandlePrefix(itemID) | (itemID & 0x0FFFFFFF)
	qty := 0
	for _, item := range slot.Inventory.CommonItems {
		if item.GaItemHandle == handle && int(item.Quantity&0x7FFFFFFF) > qty {
			qty = int(item.Quantity & 0x7FFFFFFF)
		}
	}
	for _, item := range slot.Inventory.KeyItems {
		if item.GaItemHandle == handle && int(item.Quantity&0x7FFFFFFF) > qty {
			qty = int(item.Quantity & 0x7FFFFFFF)
		}
	}
	return qty
}

// upsertInventoryContainerQuantity updates the physical container record
// already present in inventory. Vanilla saves store the goods container in
// CommonItems; a KeyItems record is supported only as a legacy compatibility
// layout. When missing entirely, core.AddItemsToSlot creates the canonical
// CommonItems record.
func upsertInventoryContainerQuantity(slot *core.SaveSlot, itemID, qty uint32) error {
	handle := db.ItemIDToHandlePrefix(itemID) | (itemID & 0x0FFFFFFF)

	for row := range slot.Inventory.CommonItems {
		if slot.Inventory.CommonItems[row].GaItemHandle == handle {
			return setInventoryCommonItemQuantity(slot, row, qty)
		}
	}

	for row := range slot.Inventory.KeyItems {
		if slot.Inventory.KeyItems[row].GaItemHandle == handle {
			return setInventoryKeyItemQuantity(slot, row, qty)
		}
	}

	return core.AddItemsToSlot(slot, []uint32{itemID}, int(qty), 0, false)
}

// containerSyncAction is one container syncContainerKeyItems will raise, together
// with the exact pickup/vendor event flags the writer sets for that raised
// quantity. It is the single shared synchronization plan: both the writer below
// and the SaveCharacter container diagnostics compute it from planContainerSync,
// so the log can never predict a side effect the writer would not perform, nor
// miss one it would.
type containerSyncAction struct {
	itemID      uint32   // canonical container key item ID (key into data maps)
	current     int      // container quantity before the sync (0 when absent)
	finalQty    int      // raised quantity; always > current
	pickupFlags []uint32 // pickup flags [0..finalQty-1], capped at the flag list
	vendorFlags []uint32 // ContainerVendorPurchaseFlags[itemID]
}

// planContainerSync computes, without mutating the slot, every container whose
// quantity must be raised and the flags that must follow. It is the extracted
// heart of the container synchronization semantics: gated units are totalled
// only from Inventory.CommonItems; a family present only in Storage forces at
// least one container; quantities never drop; a container is planned only when
// its final quantity exceeds the current one. Pickup flags run 0..finalQty-1
// (capped at the flag list) and vendor flags come from data; both are attached
// only when the slot exposes a valid Event Flags region, because the writer
// cannot set them otherwise. Actions are ordered by ascending item ID so the
// plan (and the diagnostics built from it) is deterministic.
func planContainerSync(slot *core.SaveSlot) []containerSyncAction {
	itemID := func(h uint32) uint32 {
		if id, ok := slot.GaMap[h]; ok && id != 0 {
			return id
		}
		return db.HandleToItemID(h)
	}
	live := func(h uint32) bool { return h != core.GaHandleEmpty && h != core.GaHandleInvalid }

	invByContainer := make(map[uint32]int)
	storageContainers := make(map[uint32]bool)
	for _, it := range slot.Inventory.CommonItems {
		if !live(it.GaItemHandle) {
			continue
		}
		_, base := db.GetItemDataFuzzy(itemID(it.GaItemHandle))
		if cID, ok := data.GetRequiredContainer(base); ok {
			invByContainer[cID] += int(it.Quantity & 0x7FFFFFFF)
		}
	}
	for _, it := range slot.Storage.CommonItems {
		if !live(it.GaItemHandle) {
			continue
		}
		_, base := db.GetItemDataFuzzy(itemID(it.GaItemHandle))
		if cID, ok := data.GetRequiredContainer(base); ok {
			storageContainers[cID] = true
		}
	}

	needed := make(map[uint32]bool)
	for cID := range invByContainer {
		needed[cID] = true
	}
	for cID := range storageContainers {
		needed[cID] = true
	}

	withFlags := hasEventFlagsRegion(slot)
	var actions []containerSyncAction
	for cID := range needed {
		current := inventoryContainerQuantity(slot, cID)
		finalQty := invByContainer[cID]
		if current > finalQty {
			finalQty = current
		}
		if storageContainers[cID] && finalQty < 1 {
			finalQty = 1
		}
		if finalQty <= current {
			continue
		}
		act := containerSyncAction{itemID: cID, current: current, finalQty: finalQty}
		if withFlags {
			if flagList, ok := data.ContainerPickupFlags[cID]; ok {
				n := finalQty
				if n > len(flagList) {
					n = len(flagList)
				}
				act.pickupFlags = append([]uint32(nil), flagList[:n]...)
			}
			act.vendorFlags = append([]uint32(nil), data.ContainerVendorPurchaseFlags[cID]...)
		}
		actions = append(actions, act)
	}
	sort.Slice(actions, func(i, j int) bool { return actions[i].itemID < actions[j].itemID })
	return actions
}

// syncContainerKeyItems reconciles containers (Cracked/Ritual/Hefty Pot,
// Perfume Bottle) against the current slot state after a manual inventory edit
// (Character → Inventory → Save Changes, via SaveCharacter). Each container's
// quantity is raised to cover the total active Inventory units of its gated
// family; a family present only in Storage forces at least one container.
// Quantities are never lowered. Existing records are updated in their physical
// section; missing records are added to canonical Inventory.CommonItems.
// On a confirmed increase the container's pickup/vendor flags are set for the
// saved quantity. Returns an error if a container record cannot be written.
func syncContainerKeyItems(slot *core.SaveSlot) error {
	for _, act := range planContainerSync(slot) {
		if err := upsertInventoryContainerQuantity(slot, act.itemID, uint32(act.finalQty)); err != nil {
			return err
		}
		// Container written successfully — set pickup/vendor flags for the saved
		// quantity (mirrors the Add path) so the game won't re-offer the world
		// pickups. Best-effort: planContainerSync already leaves the flag lists
		// empty when no Event Flags region is available.
		if hasEventFlagsRegion(slot) {
			flags := slot.Data[slot.EventFlagsOffset:]
			for _, f := range act.pickupFlags {
				_ = db.SetEventFlag(flags, f, true)
			}
			for _, f := range act.vendorFlags {
				_ = db.SetEventFlag(flags, f, true)
			}
		}
	}
	return nil
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

func addItemCaps(item data.ItemData, useGameLimits bool) (inventory, storage uint32) {
	inventory = item.MaxInventory
	storage = item.MaxStorage
	if !useGameLimits {
		return inventory, storage
	}
	if item.GameMaxInventoryKnown {
		inventory = item.GameMaxInventory
	}
	if item.GameMaxStorageKnown {
		storage = item.GameMaxStorage
	}
	return inventory, storage
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
	a.pushUndoSnapshotLocked(idx, a.buildSlotSnapshotLocked(idx))
}

// buildSlotSnapshotLocked deep-copies the current state of slot[idx] into an
// undo snapshot WITHOUT pushing it. The repair apply endpoint captures this
// before a batch and only pushes it (via pushUndoSnapshotLocked) if at least
// one action actually mutated — so a fully-rolled-back batch leaves no undo
// entry. Same locking contract as pushUndoLocked.
func (a *App) buildSlotSnapshotLocked(idx int) slotSnapshot {
	return slotSnapshot{
		Active:         a.save.ActiveSlots[idx],
		ProfileSummary: a.save.ProfileSummaries[idx],
		Slot:           core.SnapshotSlot(&a.save.Slots[idx]),
	}
}

// pushUndoSnapshotLocked appends a pre-built undo snapshot onto the stack,
// enforcing the depth cap. Same locking contract as pushUndoLocked.
func (a *App) pushUndoSnapshotLocked(idx int, snap slotSnapshot) {
	stack := a.undoStacks[idx]
	if len(stack) >= maxUndoDepth {
		stack = stack[1:] // drop oldest
	}
	a.undoStacks[idx] = append(stack, snap)
	a.slotRevisions[idx]++
	a.invalidateGaItemRepackTokensLocked(idx)
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
	core.RestoreSlot(&a.save.Slots[idx], snap.Slot)
	a.slotRevisions[idx]++
	a.invalidateGaItemRepackTokensLocked(idx)

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
	return a.setNetworkParamsWithDiagnostics("network_params_apply", params)
}

// ResetNetworkParams restores vanilla defaults for all network parameters.
//
// Delegates to the internal locked worker (not the public SetNetworkParams)
// to avoid double-acquire of saveMu.Lock (sync.RWMutex.Lock is not
// reentrant).
func (a *App) ResetNetworkParams() error {
	return a.setNetworkParamsWithDiagnostics("network_params_reset", core.NetworkParamDefaults())
}

// setNetworkParamsWithDiagnostics keeps the durable journal writes outside
// saveMu while preserving the public apply/reset contracts. The action name is
// internal and closed, so no network values or renderer-controlled strings are
// included in diagnostics.
func (a *App) setNetworkParamsWithDiagnostics(action string, params core.NetworkParamValues) error {
	a.journalLog(levelInfo, "network_params_requested", "network parameter change requested", field("action", action))
	noActiveSave := false
	err := func() error {
		a.saveMu.Lock()
		defer a.saveMu.Unlock()
		if a.save == nil {
			noActiveSave = true
			return fmt.Errorf("no save loaded")
		}
		return a.setNetworkParamsLocked(params)
	}()
	if err != nil {
		stage := "patch"
		if noActiveSave {
			stage = "no_active_save"
		}
		a.journalLog(levelError, "network_params_finished", "network parameter change failed", field("action", action), field("outcome", "error"), field("stage", stage))
		return err
	}
	a.journalLog(levelInfo, "network_params_finished", "network parameter change finished", field("action", action), field("outcome", "success"))
	return nil
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
	// The requested event records the user's intent and is journalled — and
	// fsync'd, since every journal Log is durable — before the save mutation or
	// any repair lock, so a crash mid-repair still leaves a durable trace that a
	// repair began. It carries intent only: no pre-scan-derived actions, because
	// a pre-scan needs the very locks we must not hold before this event.
	a.journalLog(levelInfo, "inventory_integrity_repair_requested",
		"inventory integrity repair requested",
		field("character_index", strconv.Itoa(charIdx)),
		field("action", "repair_duplicates"))

	report, dupInv, dupPhysick, attemptedReassign, attemptedPhysick, err := a.repairDuplicateInventoryIndicesLocked(charIdx)

	outcome := "applied"
	level := levelInfo
	switch {
	case err != nil:
		outcome = "error"
		level = levelError
	case dupInv == 0 && dupPhysick == 0:
		outcome = "no_changes"
	}

	// The finished event runs after every lock is released (a journal Sync must
	// never run under saveMu or a slot lock) and describes the real result.
	// attempted_actions names the core repair steps the backend actually
	// reached — not what a pre-scan implied — so with outcome it honestly
	// reports which steps started and whether the whole process succeeded. An
	// error logs at levelError so the console's Error filter surfaces it.
	a.journalLog(level, "inventory_integrity_repair_finished",
		"inventory integrity repair finished",
		field("character_index", strconv.Itoa(charIdx)),
		field("duplicate_inventory_entries", strconv.Itoa(dupInv)),
		field("duplicate_physick_entries", strconv.Itoa(dupPhysick)),
		field("outcome", outcome),
		field("changed_inventory_indices", strconv.Itoa(report.Changed)),
		field("attempted_actions", repairAttemptedActions(attemptedReassign, attemptedPhysick)))

	return report, err
}

// repairAttemptedActions renders the set of core repair steps that were
// actually begun: index reassignment when core.RepairDuplicateInventoryIndices
// was invoked, physick removal when core.RepairDuplicateWondrousPhysick was
// invoked. It says nothing about whether a step completed — outcome carries
// that. Both are safe, static action identifiers (no counts, handles, or item
// data), and "none" marks a step that never started (no-op / early error).
func repairAttemptedActions(attemptedReassign, attemptedPhysick bool) string {
	actions := make([]string, 0, 2)
	if attemptedReassign {
		actions = append(actions, "reassign_duplicate_inventory_indices")
	}
	if attemptedPhysick {
		actions = append(actions, "remove_duplicate_physick_entries")
	}
	if len(actions) == 0 {
		return "none"
	}
	return strings.Join(actions, ",")
}

// repairDuplicateInventoryIndicesLocked performs the scan+repair under saveMu
// and slotMu and returns the report, the pre-repair duplicate counts, and which
// core repair steps were actually begun, so the caller can journal the outcome
// after every lock is released. attemptedReassign/attemptedPhysick flip to true
// the moment the corresponding core call is reached — the second only after the
// first returned without error — so a caller never claims a step it skipped.
// It preserves the exact return contract of the endpoint (empty report on the
// no-save/invalid/no-op/repair-error paths, populated report on success and on
// the post-repair-remains errors).
func (a *App) repairDuplicateInventoryIndicesLocked(charIdx int) (report core.InventoryIndexRepairReport, dupInv, dupPhysick int, attemptedReassign, attemptedPhysick bool, err error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return report, 0, 0, false, false, fmt.Errorf("no save loaded")
	}
	if charIdx < 0 || charIdx >= 10 {
		return report, 0, 0, false, false, fmt.Errorf("invalid character index")
	}
	a.slotMu[charIdx].Lock()
	defer a.slotMu[charIdx].Unlock()
	slot := &a.save.Slots[charIdx]

	pre := core.ScanDuplicateInventoryIndices(slot)
	prePhysick := core.ScanDuplicateWondrousPhysick(slot)
	dupInv = len(pre)
	dupPhysick = len(prePhysick)
	if dupInv == 0 && dupPhysick == 0 {
		return report, dupInv, dupPhysick, false, false, nil
	}

	a.pushUndoLocked(charIdx)
	attemptedReassign = true
	report, err = core.RepairDuplicateInventoryIndices(slot)
	if err != nil {
		return core.InventoryIndexRepairReport{}, dupInv, dupPhysick, attemptedReassign, false, err
	}
	attemptedPhysick = true
	if _, err = core.RepairDuplicateWondrousPhysick(slot); err != nil {
		return core.InventoryIndexRepairReport{}, dupInv, dupPhysick, attemptedReassign, attemptedPhysick, err
	}
	if post := core.ScanDuplicateInventoryIndices(slot); len(post) > 0 {
		return report, dupInv, dupPhysick, attemptedReassign, attemptedPhysick, fmt.Errorf("RepairDuplicateInventoryIndices: %d duplicate(s) remain after repair", len(post))
	}
	if post := core.ScanDuplicateWondrousPhysick(slot); len(post) > 0 {
		return report, dupInv, dupPhysick, attemptedReassign, attemptedPhysick, fmt.Errorf("RepairDuplicateInventoryIndices: %d Flask of Wondrous Physick record(s) remain after repair", len(post))
	}
	return report, dupInv, dupPhysick, attemptedReassign, attemptedPhysick, nil
}

// Dummy method to force Wails to export types
func (a *App) _forceExportTypes() (db.GraceEntry, db.BossEntry, db.ItemEntry, db.MapEntry, db.CookbookEntry, db.GestureEntry, db.QuestNPC, db.QuestStep, db.QuestFlagState, core.SlotDiagnostics, core.DiagnosticIssue, SlotCapacity, deploy.Target, PresetInfo, FavoriteSlotInfo, db.BellBearingEntry, db.WhetbladeEntry, core.NetworkParamValues, vm.CharacterPreset, vm.PresetItem, vm.ApplyOptions, vm.PresetApplyResult, vm.WorldPresetData, PvPPreparationOptions, vm.AoWAvailabilityEntry, BuiltinCharacterPresetInfo, InventoryOrderItem, core.TransferResult, core.TransferSkip, core.InventoryIndexRepairReport, core.InventoryIndexRepairChange, SaveInventoryIntegrityReport, SlotInventoryIntegrityReport, InventoryIntegrityConflict, InventoryIntegrityConflictItem) {
	return db.GraceEntry{}, db.BossEntry{}, db.ItemEntry{}, db.MapEntry{}, db.CookbookEntry{}, db.GestureEntry{}, db.QuestNPC{}, db.QuestStep{}, db.QuestFlagState{}, core.SlotDiagnostics{}, core.DiagnosticIssue{}, SlotCapacity{}, deploy.Target{}, PresetInfo{}, FavoriteSlotInfo{}, db.BellBearingEntry{}, db.WhetbladeEntry{}, core.NetworkParamValues{}, vm.CharacterPreset{}, vm.PresetItem{}, vm.ApplyOptions{}, vm.PresetApplyResult{}, vm.WorldPresetData{}, PvPPreparationOptions{}, vm.AoWAvailabilityEntry{}, BuiltinCharacterPresetInfo{}, InventoryOrderItem{}, core.TransferResult{}, core.TransferSkip{}, core.InventoryIndexRepairReport{}, core.InventoryIndexRepairChange{}, SaveInventoryIntegrityReport{}, SlotInventoryIntegrityReport{}, InventoryIntegrityConflict{}, InventoryIntegrityConflictItem{}
}
