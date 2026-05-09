package main

import (
	"fmt"

	"github.com/oisis/EldenRing-SaveForge/backend/core"
	"github.com/oisis/EldenRing-SaveForge/backend/db"
	"github.com/oisis/EldenRing-SaveForge/backend/db/data"
)

// PvPPreparationOptions selects which world modules are applied in a single
// ApplyPvPPreparation call. Each field maps to one spec/48 module.
type PvPPreparationOptions struct {
	MatchmakingRegions bool `json:"matchmakingRegions"`
	Colosseums         bool `json:"colosseums"`
	RevealMap          bool `json:"revealMap"`
	SummoningPools     bool `json:"summoningPools"`
	SitesOfGrace       bool `json:"sitesOfGrace"`
}

// ApplyPvPPreparation applies selected PvP-preparation modules to the given
// character slot in a single undo-able operation. It uses internal core and DB
// functions to avoid stacking multiple undo entries per module.
// Returns informational warnings on success; an error if any module fails.
func (a *App) ApplyPvPPreparation(slotIndex int, opts PvPPreparationOptions) ([]string, error) {
	if a.save == nil {
		return nil, fmt.Errorf("no save loaded")
	}
	if slotIndex < 0 || slotIndex >= 10 {
		return nil, fmt.Errorf("invalid slot index")
	}

	slot := &a.save.Slots[slotIndex]
	if slot.Version == 0 {
		return nil, fmt.Errorf("slot %d is empty", slotIndex)
	}
	if slot.EventFlagsOffset <= 0 || slot.EventFlagsOffset >= len(slot.Data) {
		return nil, fmt.Errorf("event flags offset not computed for slot %d", slotIndex)
	}

	a.pushUndo(slotIndex)

	flags := slot.Data[slot.EventFlagsOffset:]
	var warnings []string

	if opts.MatchmakingRegions {
		allRegions := db.GetAllRegions()
		ids := make([]uint32, len(allRegions))
		for i, r := range allRegions {
			ids[i] = r.ID
		}
		if err := core.SetUnlockedRegions(slot, ids); err != nil {
			return nil, fmt.Errorf("matchmaking regions: %w", err)
		}
		// SetUnlockedRegions calls RebuildSlot which replaces slot.Data and
		// recalculates EventFlagsOffset — refresh the slice to avoid stale writes.
		flags = slot.Data[slot.EventFlagsOffset:]
		warnings = append(warnings, fmt.Sprintf("Applied %d matchmaking regions.", len(ids)))
	}

	if opts.Colosseums {
		colosseums := db.GetAllColosseums()
		for _, c := range colosseums {
			flagSet, ok := data.ColosseumFlagSets[c.ID]
			if !ok {
				flagSet = data.ColosseumFlagSet{Activate: c.ID}
			}
			for _, id := range flagSet.AllFlags() {
				if id == 0 {
					continue
				}
				if err := db.SetEventFlag(flags, id, true); err != nil {
					return nil, fmt.Errorf("colosseum %s flag %d: %w", c.Name, id, err)
				}
			}
		}
		for _, id := range data.ColosseumGlobalFlags {
			if err := db.SetEventFlag(flags, id, true); err != nil {
				return nil, fmt.Errorf("colosseum global flag %d: %w", id, err)
			}
		}
		warnings = append(warnings, "Colosseum matchmaking flags set. Physical gates may still need to be opened once in-game.")
	}

	if opts.RevealMap {
		if err := revealBaseMap(slot); err != nil {
			return nil, fmt.Errorf("map reveal (base): %w", err)
		}
		if err := revealDLCMap(slot); err != nil {
			return nil, fmt.Errorf("map reveal (DLC): %w", err)
		}
		// revealBaseMap/revealDLCMap call AddItemsToSlot which shifts slot.Data —
		// refresh the slice so subsequent modules write to the correct array.
		flags = slot.Data[slot.EventFlagsOffset:]
		warnings = append(warnings, "Map revealed (base game + DLC).")
	}

	if opts.SummoningPools {
		pools := db.GetAllSummoningPools()
		for _, p := range pools {
			if err := db.SetEventFlag(flags, p.ID, true); err != nil {
				return nil, fmt.Errorf("summoning pool %s: %w", p.Name, err)
			}
		}
		warnings = append(warnings, fmt.Sprintf("Activated %d summoning pools. Bloody Finger invasion impact is unconfirmed.", len(pools)))
	}

	if opts.SitesOfGrace {
		warnings = append(warnings, "Sites of Grace module is planned but not enabled in this version.")
	}

	return warnings, nil
}
