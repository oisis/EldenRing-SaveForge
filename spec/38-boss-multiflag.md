# 38 — Boss Multi-Flag Kill Rework

> **Type**: Design doc
> **Extracted from**: docs/ROADMAP.md (2026-05-03 cleanup)
> **Status**: 🔲 Planned

---

## Problem

The current implementation sets only 1 event flag per boss (the defeat flag 9xxx). This awards runes, but the boss remains alive in the game. A proper kill/respawn requires setting multiple flags per boss.

## Required flags per boss

Each boss kill requires a combination of:
- **Arena state flag** — marks the arena as "boss defeated" (prevents the fog wall)
- **Defeat flag** (9xxx) — awards runes, records the kill in the save
- **Quest progress flags** — NPCs reacting to the boss's death
- **Site of Grace activation flags** — the Site of Grace after the boss
- **Drop flags** — Remembrance, unique drops
- **World state flags** — map changes, NPC relocations

## Reference data

`tmp/repos/er-save-manager/src/er_save_manager/data/boss_data.py` contains 208 bosses with full flag lists. Structure:

```python
boss_data = {
    "arena_state_flag": {
        "name": "Boss Name",
        "event_flags": [flag1, flag2, flag3, ...]
    }
}
```

Key: the arena state flag (the primary identifier, NOT the defeat flag 9xxx).

## Implementation plan

### Data structure change

```go
type BossData struct {
    ID          uint32   `json:"id"`          // arena state flag (primary key)
    Name        string   `json:"name"`
    Region      string   `json:"region"`
    Type        string   `json:"type"`        // "main" or "field"
    Remembrance bool     `json:"remembrance"`
    EventFlags  []uint32 `json:"eventFlags"`  // ALL flags to set on a kill
}
```

### Algorithm

**Kill:**
```
for each flag in boss.EventFlags:
    SetEventFlag(slot.Data[EventFlagsOffset:], flag, true)
```

**Respawn:**
```
for each flag in boss.EventFlags:
    SetEventFlag(slot.Data[EventFlagsOffset:], flag, false)
```

### Migration

- Change the key of the `bosses.go` map from the defeat flag to the arena state flag
- Import all 208 entries from `boss_data.py` (currently only ~120)
- Add DLC bosses with full flag lists
- The existing method `SetBossDefeated(slotIndex, bossID, defeated)` keeps the same API, it only iterates internally over all flags

### Testing

- In-game verification per boss required (the boss should not appear after a kill)
- Priority: main bosses first (Godrick, Rennala, Radahn, Morgott, Fire Giant, Godfrey, Radagon/EB)
- Field bosses second (Tree Sentinel, Agheel, Tibia Mariner, etc.)

### Known complications

- Some bosses have conditional flags (e.g., Radahn requires festival flags)
- Multi-phase bosses (Godfrey/Hoarah Loux) may require intermediate-state flags
- A boss respawn may not work for bosses causing irreversible world changes (Rykard → Volcano Manor)
