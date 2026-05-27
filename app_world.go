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

// SetGraceVisited sets the grace EventFlag (71xxx–76xxx) used for map marker and fast-travel list.
// Does not touch LastRestedGrace — the game updates that automatically on arrival.
// Does not persist the in-world object visual state — EMEVD derives it from the EventFlag at area load.
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
	return core.SetUnlockedRegions(slot, mergeUnlockedRegions(regionIDs, slot.UnlockedRegions))
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
