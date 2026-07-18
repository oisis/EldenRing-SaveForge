package main

import (
	"encoding/binary"
	"fmt"
	"math"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// GetGraces returns all Sites of Grace with visited state from the specified character slot
func (a *App) GetGraces(slotIndex int) ([]db.GraceEntry, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

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

// SetGraceVisited sets the grace EventFlag (71xxx–76xxx) used for map marker and fast-travel list.
// Does not touch LastRestedGrace — the game updates that automatically on arrival.
// Does not persist the in-world object visual state — EMEVD derives it from the EventFlag at area load.
func (a *App) SetGraceVisited(slotIndex int, graceID uint32, visited bool) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	a.pushUndoLocked(slotIndex)

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

	// SET-only: companion flags are set on activation but never cleared on deactivation.
	// They may also be set by item companion flags or normal game progression — clearing
	// them on visited=false would regress saves that obtained the flags through other paths.
	if visited {
		if companions := data.CompanionEventFlagsForGrace(graceID); len(companions) > 0 {
			for _, f := range companions {
				if err := db.SetEventFlag(flags, f, true); err != nil {
					fmt.Printf("Warning: companion flag %d for grace %d: %v\n", f, graceID, err)
				}
			}
		}
	}

	return nil
}

// GetBosses returns all boss encounters with defeated state from the specified character slot
func (a *App) GetBosses(slotIndex int) ([]db.BossEntry, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	a.pushUndoLocked(slotIndex)

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	a.pushUndoLocked(slotIndex)

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()
	a.pushUndoLocked(slotIndex)

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

// mergeUnlockedRegions computes the new raw unlocked_regions list for a World-tab
// bulk operation. It sets the curated invasion-region membership to exactly
// curatedIDs while preserving every existing raw ID that is NOT part of the
// curated allowlist (db.IsKnownRegionID).
//
// The World tab only ever surfaces curated regions, so a bare list replacement
// would silently drop advanced / internal region IDs a real save carries (a
// late-game DLC save can hold ~395 raw IDs vs. the curated allowlist's ~274).
// Preserving the non-curated remainder makes every bulk action non-destructive:
//
//	Unlock All  → curatedIDs = full allowlist  ⇒ result = rawNonCurated ∪ allowlist
//	Lock All    → curatedIDs = {}              ⇒ result = rawNonCurated
//	per-area ±  → curatedIDs = curated subset  ⇒ rawNonCurated always retained
//
// core.SetUnlockedRegions dedupes + sorts the result, so overlaps are harmless.
func mergeUnlockedRegions(curatedIDs, existingRaw []uint32) []uint32 {
	out := make([]uint32, 0, len(curatedIDs)+len(existingRaw))
	out = append(out, curatedIDs...)
	for _, id := range existingRaw {
		if !db.IsKnownRegionID(id) {
			out = append(out, id)
		}
	}
	return out
}

// BulkSetUnlockedRegions sets the curated invasion-region membership of the slot
// to regionIDs while preserving any raw unlocked_regions entries outside the
// curated allowlist (see mergeUnlockedRegions). Used by the World tab for
// per-area Unlock/Lock and the global Unlock All / Lock All actions. The result
// is deduped + sorted by core.SetUnlockedRegions.
func (a *App) BulkSetUnlockedRegions(slotIndex int, regionIDs []uint32) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()
	a.pushUndoLocked(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.Version == 0 {
		return fmt.Errorf("slot %d is empty", slotIndex)
	}
	return core.SetUnlockedRegions(slot, mergeUnlockedRegions(regionIDs, slot.UnlockedRegions))
}

// GetColosseums returns all colosseums with unlock state from the specified character slot
func (a *App) GetColosseums(slotIndex int) ([]db.ColosseumEntry, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	a.pushUndoLocked(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	flags := slot.Data[slot.EventFlagsOffset:]

	// Set the primary Activate flag plus every per-colosseum derivative
	// (MapPOI, NPC, Gate). Without the gate flag the matchmaking menu
	// reports the arena as unlocked but the physical entrance stays closed
	// (see tmp/coloseum-debug/ for the RE that established this set).
	flagSet, ok := data.ColosseumFlagSets[colosseumID]
	if !ok {
		flagSet = data.ColosseumFlagSet{Activate: colosseumID}
	}
	for _, id := range flagSet.AllFlags() {
		if id == 0 {
			continue
		}
		if err := db.SetEventFlag(flags, id, unlocked); err != nil {
			return fmt.Errorf("failed to set colosseum flag %d: %w", id, err)
		}
	}

	// Globals fire once any colosseum is unlocked; we set them on unlock
	// but never clear them, because they double as broader progression
	// markers and clearing risks regressing unrelated systems.
	if unlocked {
		for _, id := range data.ColosseumGlobalFlags {
			if err := db.SetEventFlag(flags, id, true); err != nil {
				return fmt.Errorf("failed to set colosseum global flag %d: %w", id, err)
			}
		}
	}
	return nil
}

// GetGestures returns all gestures with unlock state from the specified character slot.
// GestureGameData is 64×u32 at StorageBoxOffset + DynStorageBox (immediately after storage).
// Gesture slot IDs vary by body type — some gestures have even/odd variants.
func (a *App) GetGestures(slotIndex int) ([]db.GestureEntry, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	a.pushUndoLocked(slotIndex)

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()
	if len(gestureIDs) == 0 {
		return nil
	}

	a.pushUndoLocked(slotIndex)

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

// GetQuestNPCs returns the list of NPC names with quest data.
func (a *App) GetQuestNPCs() []string {
	return db.GetAllQuestNPCs()
}

// GetQuestProgress returns the quest progression for a specific NPC in a character slot.
func (a *App) GetQuestProgress(slotIndex int, npcName string) (*db.QuestNPC, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	questSteps, ok := data.QuestData[npcName]
	if !ok {
		return fmt.Errorf("unknown NPC: %s", npcName)
	}
	if stepIndex < 0 || stepIndex >= len(questSteps) {
		return fmt.Errorf("invalid step index %d for %s", stepIndex, npcName)
	}

	a.pushUndoLocked(slotIndex)

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	slot := &a.save.Slots[slotIndex]
	cookbooks := db.GetAllCookbooks()

	// Build set of cookbook item IDs present in inventory (flag may be unset even when item exists)
	ownedCookbooks := make(map[uint32]bool)
	checkItem := func(handle, qty uint32) {
		if handle == 0 || handle == 0xFFFFFFFF || qty == 0 {
			return
		}
		itemID := db.HandleToItemID(handle)
		if _, ok := data.CookbookItemToFlagID[itemID]; ok {
			ownedCookbooks[itemID] = true
		}
	}
	for _, it := range slot.Inventory.CommonItems {
		checkItem(it.GaItemHandle, it.Quantity)
	}
	for _, it := range slot.Inventory.KeyItems {
		checkItem(it.GaItemHandle, it.Quantity)
	}

	var flags []byte
	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags = slot.Data[slot.EventFlagsOffset:]
	}
	for i, cb := range cookbooks {
		flagOn, _ := db.GetEventFlag(flags, cb.ID)
		itemID := data.CookbookFlagToItemID[cb.ID]
		cookbooks[i].Unlocked = flagOn || ownedCookbooks[itemID]
	}

	return cookbooks, nil
}

// journalUnlockMutation runs a Database-tab single-item unlock through the Game
// Items before -> planned -> finished lifecycle when Debug Mode is on, and always
// performs exactly one real mutation. apply is a slot-only writer that performs
// precisely the operation's mutation; it runs first on a clone to project the
// planned diff and then on the real slot, so the two share one implementation and
// planned can never drift from what actually lands. flagIDs are the Event Flags
// the operation owns. With Debug Mode off no clone is taken and no records are
// emitted — a single real mutation runs.
//
// The caller holds saveMu.RLock + slotMu[slotIndex], has already taken its single
// pushUndoLocked snapshot, and has validated the slot's Event Flags region. On a
// real mutation error after work has begun, the finished phase reports the actual
// post-error slot under stage apply_unlock; on success it reports stage completed.
func (a *App) journalUnlockMutation(action string, slotIndex int, slot *core.SaveSlot, flagIDs []uint32, apply func(*core.SaveSlot) error) error {
	if !a.journal.debugEnabled() {
		return apply(slot)
	}
	clone := core.CloneSlot(slot)
	_ = apply(clone)
	plans := planGameItemsMutation(slot, clone, flagIDs)
	a.journalGameItemsMutationBefore(action, slotIndex, plans)
	if err := apply(slot); err != nil {
		a.journalGameItemsMutationFinished(action, slotIndex, characterChangeError, stageGameItemsUnlock, plans, slot)
		return err
	}
	a.journalGameItemsMutationFinished(action, slotIndex, characterChangeSuccess, characterStageCompleted, plans, slot)
	return nil
}

// applyCookbookUnlock performs the cookbook mutation on a single slot: a
// best-effort event flag per cookbook plus the batched inventory add (unlock) or
// per-cookbook removal (lock). Shared by the bulk worker, the single-item entry
// point and Debug Mode's clone so a planned diff cannot drift from the applied
// mutation. Caller holds the slot lock and has validated EventFlagsOffset.
func applyCookbookUnlock(slot *core.SaveSlot, cookbookIDs []uint32, unlocked bool) {
	flags := slot.Data[slot.EventFlagsOffset:]

	// Collect item IDs to add in batch.
	var itemsToAdd []uint32

	for _, cookbookID := range cookbookIDs {
		_ = db.SetEventFlag(flags, cookbookID, unlocked) // best-effort; inventory op is independent

		if itemID, ok := data.CookbookFlagToItemID[cookbookID]; ok {
			if unlocked {
				itemsToAdd = append(itemsToAdd, itemID)
			} else {
				core.RemoveItemByBaseID(slot, itemID)
			}
		}
	}

	if len(itemsToAdd) > 0 {
		_ = core.AddItemsToSlot(slot, itemsToAdd, 1, 0, false)
	}
}

// SetCookbookUnlocked sets or clears the unlock flag for a single cookbook and
// keeps the matching inventory item in sync. It performs its own single
// pushUndoLocked and validation rather than re-entering the public
// BulkSetCookbooksUnlocked (which would double-acquire the per-slot lock), and
// routes the mutation through the Game Items unlock lifecycle so Debug Mode
// captures its before -> planned -> finished records.
func (a *App) SetCookbookUnlocked(slotIndex int, cookbookID uint32, unlocked bool) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	a.pushUndoLocked(slotIndex)
	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}
	cookbookIDs := []uint32{cookbookID}
	return a.journalUnlockMutation(actionGameItemsUnlockCookbook, slotIndex, slot, cookbookIDs, func(s *core.SaveSlot) error {
		applyCookbookUnlock(s, cookbookIDs, unlocked)
		return nil
	})
}

// BulkSetCookbooksUnlocked sets event flags AND adds/removes inventory items for multiple cookbooks.
// Single pushUndo — safe for concurrent Wails calls.
func (a *App) BulkSetCookbooksUnlocked(slotIndex int, cookbookIDs []uint32, unlocked bool) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()
	return a.bulkSetCookbooksUnlockedLocked(slotIndex, cookbookIDs, unlocked)
}

// bulkSetCookbooksUnlockedLocked is the shared worker for SetCookbookUnlocked
// and BulkSetCookbooksUnlocked.
//
// Contract: caller MUST have validated `a.save != nil` and `slotIndex` in
// range, and MUST hold exclusive access to slot[slotIndex]. In the upcoming
// lock phase the caller will hold saveMu.RLock + slotMu[slotIndex]. The
// helper takes exactly one pushUndoLocked snapshot per invocation.
func (a *App) bulkSetCookbooksUnlockedLocked(slotIndex int, cookbookIDs []uint32, unlocked bool) error {
	a.pushUndoLocked(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	applyCookbookUnlock(slot, cookbookIDs, unlocked)
	return nil
}

// GetBellBearings returns all bell bearings with unlock state from the specified character slot
func (a *App) GetBellBearings(slotIndex int) ([]db.BellBearingEntry, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()
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
//
// NOTE: this entry point uses its own inline implementation (not the bulk
// worker) because it propagates db.SetEventFlag errors to the caller, while
// the bulk path silently tolerates per-flag failures. Keeping the inline
// implementation preserves the existing single-flag error contract; the
// upcoming lock phase will wrap this body directly with slotMu[slotIndex].
func (a *App) SetBellBearingUnlocked(slotIndex int, flagID uint32, unlocked bool) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()
	a.pushUndoLocked(slotIndex)
	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}
	return a.journalUnlockMutation(actionGameItemsUnlockBellBearing, slotIndex, slot, []uint32{flagID}, func(s *core.SaveSlot) error {
		return applyBellBearingUnlock(s, flagID, unlocked)
	})
}

// applyBellBearingUnlock performs the bell bearing mutation on a single slot: set
// the acquisition flag (propagating a SetEventFlag error exactly as the original
// inline path did), then apply the inventory side effect. Shared by the real
// slot and Debug Mode's clone so a planned diff cannot drift from the applied
// mutation. Caller holds the slot lock and has validated EventFlagsOffset.
func applyBellBearingUnlock(slot *core.SaveSlot, flagID uint32, unlocked bool) error {
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
	handlePrefix := db.ItemIDToHandlePrefix(itemID)
	computedHandle := (itemID & 0x0FFFFFFF) | handlePrefix
	// Try both computed handle (editor-added) and raw itemID (game-placed key items)
	_ = core.RemoveItemFromSlot(slot, computedHandle, true, true)
	_ = core.RemoveItemFromSlot(slot, itemID, true, true)
}

// GetWhetblades returns all whetblades with unlock state from the specified character slot
func (a *App) GetWhetblades(slotIndex int) ([]db.WhetbladeEntry, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()
	slot := &a.save.Slots[slotIndex]
	entries := db.GetAllWhetblades()

	ownedWhetblades := make(map[uint32]bool)
	checkItem := func(handle, qty uint32) {
		if handle == 0 || handle == 0xFFFFFFFF || qty == 0 {
			return
		}
		itemID := db.HandleToItemID(handle)
		if _, ok := data.WhetbladeItemToFlagID[itemID]; ok {
			ownedWhetblades[itemID] = true
		}
	}
	for _, it := range slot.Inventory.CommonItems {
		checkItem(it.GaItemHandle, it.Quantity)
	}
	for _, it := range slot.Inventory.KeyItems {
		checkItem(it.GaItemHandle, it.Quantity)
	}

	var flags []byte
	if slot.EventFlagsOffset > 0 && slot.EventFlagsOffset < len(slot.Data) {
		flags = slot.Data[slot.EventFlagsOffset:]
	}
	for i, e := range entries {
		flagOn, _ := db.GetEventFlag(flags, e.ID)
		itemID := data.WhetbladeFlagToItemID[e.ID]
		entries[i].Unlocked = flagOn || ownedWhetblades[itemID]
	}
	return entries, nil
}

// whetbladeAlreadyOwned reports true if the whetblade is already considered
// unlocked: event flag is set OR the item is present in any inventory section.
func whetbladeAlreadyOwned(slot *core.SaveSlot, flagID uint32, flags []byte) bool {
	if flagOn, err := db.GetEventFlag(flags, flagID); err == nil && flagOn {
		return true
	}
	itemID, ok := data.WhetbladeFlagToItemID[flagID]
	if !ok {
		return false
	}
	check := func(items []core.InventoryItem) bool {
		for _, it := range items {
			if it.Quantity > 0 && db.HandleToItemID(it.GaItemHandle) == itemID {
				return true
			}
		}
		return false
	}
	return check(slot.Inventory.CommonItems) || check(slot.Inventory.KeyItems)
}

// applyWhetbladeUnlock performs the whetblade mutation on a single slot: the main
// whetblade flag (whose error aborts before any further change, matching the
// original single-item path), related affinity flags, the inventory item, the
// Whetstone Knife Storm Stomp bonus + its duplication flag, and the AoW menu flag.
// Shared by the single-item entry point and Debug Mode's clone so a planned diff
// cannot drift from the applied mutation. Caller holds the slot lock and has
// validated EventFlagsOffset.
func applyWhetbladeUnlock(slot *core.SaveSlot, flagID uint32, unlocked bool) error {
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
			core.RemoveItemByBaseID(slot, itemID)
		}
	}

	// 4. Whetstone Knife bonus: add/remove Storm Stomp AoW + duplication flag.
	if flagID == data.WhetstoneKnifeFlag {
		if unlocked {
			_ = core.AddItemsToSlot(slot, []uint32{data.StormStompItemID}, 1, 0, false)
			_ = db.SetEventFlag(flags, data.StormStompDupFlag, true)
		} else {
			core.RemoveItemByBaseID(slot, data.StormStompItemID)
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

// whetbladeUnlockFlagCandidates is the full set of Event Flags the whetblade
// mutation may touch: the main flag, its related affinity flags, the Storm Stomp
// duplication flag (Whetstone Knife only) and the AoW menu flag. Only flags the
// operation actually owns are listed; planGameItemsMutation self-excludes any
// whose before == planned (e.g. the menu flag on a lock while another whetblade
// stays active).
func whetbladeUnlockFlagCandidates(flagID uint32) []uint32 {
	flagIDs := []uint32{flagID}
	flagIDs = append(flagIDs, data.WhetbladeRelatedFlags[flagID]...)
	if flagID == data.WhetstoneKnifeFlag {
		flagIDs = append(flagIDs, data.StormStompDupFlag)
	}
	flagIDs = append(flagIDs, data.AoWMenuUnlockedFlag)
	return flagIDs
}

// SetWhetbladeUnlocked sets or clears the unlock flag for a whetblade,
// manages the inventory item, related affinity flags, and the AoW menu flag.
// Returns true if the whetblade was already unlocked/owned before this call.
// The mutation runs through the Game Items unlock lifecycle so Debug Mode
// captures its before -> planned -> finished records without changing the
// returned (alreadyOwned, error) contract.
func (a *App) SetWhetbladeUnlocked(slotIndex int, flagID uint32, unlocked bool) (bool, error) {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return false, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return false, fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	a.pushUndoLocked(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return false, fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	flags := slot.Data[slot.EventFlagsOffset:]

	// alreadyOwned is read from the real slot before any mutation, exactly as the
	// original single-item path did.
	alreadyOwned := whetbladeAlreadyOwned(slot, flagID, flags)

	if err := a.journalUnlockMutation(actionGameItemsUnlockWhetblade, slotIndex, slot, whetbladeUnlockFlagCandidates(flagID), func(s *core.SaveSlot) error {
		return applyWhetbladeUnlock(s, flagID, unlocked)
	}); err != nil {
		return false, err
	}

	return alreadyOwned, nil
}

// BulkSetBellBearings sets multiple bell bearing flags at once (single undo)
// and keeps the matching inventory items in sync via syncBellBearingItem.
func (a *App) BulkSetBellBearings(slotIndex int, flagIDs []uint32, unlocked bool) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()
	return a.bulkSetBellBearingsLocked(slotIndex, flagIDs, unlocked)
}

// bulkSetBellBearingsLocked is the internal worker for BulkSetBellBearings.
//
// Contract: caller MUST have validated `a.save != nil` and `slotIndex` in
// range, and MUST hold exclusive access to slot[slotIndex]. In the upcoming
// lock phase the caller will hold saveMu.RLock + slotMu[slotIndex]. The
// helper takes exactly one pushUndoLocked snapshot per invocation and
// silently tolerates per-flag SetEventFlag failures (bulk semantics).
func (a *App) bulkSetBellBearingsLocked(slotIndex int, flagIDs []uint32, unlocked bool) error {
	a.pushUndoLocked(slotIndex)
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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	a.pushUndoLocked(slotIndex)

	slot := &a.save.Slots[slotIndex]
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	return a.journalUnlockMutation(actionGameItemsUnlockMapFragment, slotIndex, slot, []uint32{visibleFlagID}, func(s *core.SaveSlot) error {
		return applyMapRegionUnlock(s, visibleFlagID, enabled)
	})
}

// applyMapRegionUnlock performs the map region mutation on a single slot: set the
// visible flag (wrapping a SetEventFlag error exactly as the original inline path
// did) and add/remove the matching map fragment item. Only the visible flag is
// touched — the acquired (63xxx) flag is not written by this operation. Shared by
// the real slot and Debug Mode's clone so a planned diff cannot drift from the
// applied mutation. Caller holds the slot lock and has validated EventFlagsOffset.
func applyMapRegionUnlock(slot *core.SaveSlot, visibleFlagID uint32, enabled bool) error {
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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	a.pushUndoLocked(slotIndex)

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	a.pushUndoLocked(slotIndex)

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
	afterRegs, err := resolveAfterRegs(slot)
	if err != nil {
		return err
	}

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

// resolveAfterRegs computes the byte offset immediately after the UnlockedRegions array.
// Layout: StorageBox → gesturesOff (count u32) → count × 4-byte region IDs → afterRegs.
func resolveAfterRegs(slot *core.SaveSlot) (int, error) {
	storageEnd := slot.StorageBoxOffset + core.DynStorageBox
	gesturesOff := storageEnd + core.DynStorageToGestures
	if gesturesOff+4 > len(slot.Data) {
		return 0, fmt.Errorf("gesturesOff 0x%X out of bounds", gesturesOff)
	}
	regCount := int(binary.LittleEndian.Uint32(slot.Data[gesturesOff : gesturesOff+4]))
	return gesturesOff + 4 + regCount*4, nil
}

// putF32 writes a float32 value at the given offset in little-endian format.
func putF32(d []byte, off int, v float32) {
	binary.LittleEndian.PutUint32(d[off:], math.Float32bits(v))
}

// ResetMapExploration clears all map visibility, acquisition, and POI discovery flags
func (a *App) ResetMapExploration(slotIndex int) error {
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	a.pushUndoLocked(slotIndex)

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
	a.saveMu.RLock()
	defer a.saveMu.RUnlock()
	if a.save == nil {
		return fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return fmt.Errorf("invalid slot index")
	}
	a.slotMu[slotIndex].Lock()
	defer a.slotMu[slotIndex].Unlock()

	a.pushUndoLocked(slotIndex)

	slot := &a.save.Slots[slotIndex]
	afterRegs, err := resolveAfterRegs(slot)
	if err != nil {
		return err
	}

	fowStart := afterRegs + core.FoWBlobStart
	fowEnd := afterRegs + core.FoWBlobEnd

	if fowEnd >= len(slot.Data)-0x80 {
		return fmt.Errorf("FoW bitfield range out of bounds (0x%X)", fowEnd)
	}

	for i := fowStart; i <= fowEnd; i++ {
		slot.Data[i] = 0xFF
	}

	return nil
}
