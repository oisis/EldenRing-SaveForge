# 50 — Item Companion Flags

> **Type**: Design doc
> **Status**: ✅ Implemented (v0.14.0)
> **Scope**: Mechanism for setting item-dependent EventFlags when items are added to a character slot.

---

## Problem

Adding certain items via the editor results in the item being physically present in inventory, but the game treats the character as if the item was never obtained through the normal questline. This causes:

- Re-triggering of cutscenes and NPC dialogues on grace rest.
- Game mechanics gated behind EventFlags remaining locked (e.g. Torrent cannot be summoned without flag 60100 even if the Spectral Steed Whistle is in inventory).
- Quest state inconsistency — the game's EMEVD reads EventFlags, not inventory, for mechanic gates.

## Root Cause

The game sets EventFlags during the normal acquisition flow (EMEVD script execution). The editor's `AddItemsToCharacter()` previously only set single-per-item flags (`AoWItemToFlagID`, `WorldPickupFlagID`, `BolsteringPickupFlags`) and tutorial IDs. Items obtained through quest dialogue (rather than world pickup or shop) had no flag coverage.

---

## Design

### Companion flag set

A **companion flag set** is the minimal group of EventFlags that the game co-sets during normal item acquisition, such that:

1. The game's mechanic gate for the item is cleared (item is usable).
2. The game's EMEVD does not re-trigger the acquisition dialogue/cutscene.
3. No transient flags (cleared by engine after use) are included.
4. No area-specific or zone flags out of bounds on PS4 are included.

### Data structure

```go
// backend/db/data/item_companion_flags.go
var itemCompanionEventFlags = map[uint32][]uint32{
    0x40000082: {60100, 4680, 710520, 4681},
}

func CompanionEventFlagsForItem(itemID uint32) []uint32
```

### Hook location

POST-FLAGS block in `AddItemsToCharacter()` (`app.go`), after `AboutTutorialID`. Fires for every item in the `prepared` slice — including items already at max inventory quantity — enabling the mechanism to **repair saves** where the item was previously added without companion flags.

```go
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
```

---

## Spectral Steed Whistle — `0x40000082`

### Companion flags

| Flag | Name | Classification | Source |
|---|---|---|---|
| **60100** | Obtained Spectral Steed Whistle | CONFIRMED_MINIMAL — Torrent mechanic unlock | spec/12, spec/15, er-save-manager `event_flags_db.py`, 5× PC slot |
| **4680** | Melina gave Spectral Steed Whistle | CONFIRMED — quest-give state, prevents dialogue re-trigger | quests.go step 6, 5× PC slot |
| **710520** | Whistle world/map state | CONFIRMED — co-set with 60100 in game | quests.go step 6, 5× PC slot |
| **4681** | Melina accept/refuse popup shown | CONFIRMED — popup prerequisite | quests.go step 5, 5× PC slot |

### Verification

Confirmed in: 5 active slots of `ER0000.sl2` (PC, post-Melina characters, 2026-05-11). All 4 flags SET in every slot. Corroborated by vanilla PS4 save (all 0) and er-save-manager `event_flags_db.py`.

### Runtime Validation — Spectral Steed Whistle ✅ Runtime confirmed

**Date**: 2026-05-11  
**Platform**: PS4  
**Result**: PASS

Confirmed: adding Spectral Steed Whistle with companion flags 60100, 4680, 710520, 4681 prevents the normal Melina whistle-gift scene from replaying. The item works in-game. No additional Gatefront/Melina cleanup flags were required.

Flags 710770, 69090, 69370 (Melina leaves Gatefront) were **not set** and were **not needed** — runtime confirmed. These remain research candidates only.

### Flags NOT included (and why)

| Flag(s) | Reason excluded |
|---|---|
| 710770, 69090, 69370 | Melina leaves Gatefront — research candidates; **runtime confirmed not required** (2026-05-11 PS4 test). Present in all post-Melina PC saves but not needed for correct item mechanics. |
| 4698 | Melina cutscene trigger — transient, cleared by engine after cutscene plays. 0 in all real saves. |
| 4651, 4652, 4653 | Melina dialogue states — transient, cleared after dialogue. 0 in all real saves. |
| 4656 | Level Up performed — separate user action, not part of item acquisition. |
| 1042xxx range | Out of bounds on PS4 (BST offset ~130 MB vs ~2.3 MB flag array). Physically cannot be set. |

### Context: ColosseumGlobalFlags

Flag 60100 is also set by `ApplyPvPPreparation()` via `data.ColosseumGlobalFlags` when `opts.Colosseums = true`. This explains why some users reported Torrent working after editor-adding the whistle — they had previously applied PvP Preparation with Colosseums enabled. Users without that step had 60100=0 and Torrent was unusable.

---

## Adding future companion flag sets

To add companion flags for another item:

1. Research which flags the game sets during normal acquisition (check `quests.go`, compare before/after save pairs, cross-reference er-save-manager `event_flags_db.py`).
2. Verify each flag is present in post-acquisition real saves (both PC and PS4 where possible).
3. Exclude transient flags (values that are 0 in fully-settled post-acquisition saves).
4. Exclude 1042xxx range flags (out of bounds on PS4).
5. Add the item ID and flag list to `itemCompanionEventFlags` in `backend/db/data/item_companion_flags.go`.
6. Add unit tests to `backend/db/data/item_companion_flags_test.go`.
7. Add integration test cases to `tests/item_companion_flags_test.go`.

---

## Sources

- `backend/db/data/item_companion_flags.go` — implementation
- `backend/db/data/item_companion_flags_test.go` — unit tests
- `tests/item_companion_flags_test.go` — integration tests
- `spec/12-torrent.md` — Torrent mechanic flags
- `spec/15-event-flags.md` — EventFlag registry
- `backend/db/data/quests.go` — Melina quest chain flag steps
- `tmp/repos/er-save-manager/src/er_save_manager/data/event_flags_db.py` — community flag database
- `tmp/regulation-bin-debug/spectral-steed-whistle-research.md` — full research report (2026-05-11)
