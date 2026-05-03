# 38 — Boss Kill Multi-Flag Rework

> **Type**: Design doc
> **Extracted from**: docs/ROADMAP.md (2026-05-03 cleanup)
> **Status**: 🔲 Planned

---

## Problem

Current implementation sets only 1 event flag per boss (9xxx defeat flag). This grants runes but the boss remains alive in-game. Proper kill/respawn requires setting multiple flags per boss.

## Required flags per boss

Each boss kill needs a combination of:
- **Arena state flag** — marks the arena as "boss defeated" (prevents fog wall)
- **Defeat flag** (9xxx) — grants runes, marks kill in save
- **Quest progression flags** — NPCs that react to boss death
- **Grace activation flags** — post-boss Site of Grace
- **Item drop flags** — Remembrance, unique drops
- **World state flags** — map changes, NPC relocations

## Reference data

`tmp/repos/er-save-manager/src/er_save_manager/data/boss_data.py` contains 208 bosses with complete flag lists. Structure:

```python
boss_data = {
    "arena_state_flag": {
        "name": "Boss Name",
        "event_flags": [flag1, flag2, flag3, ...]
    }
}
```

Key: arena state flag (primary identifier, NOT the 9xxx defeat flag).

## Implementation plan

### Data structure change

```go
type BossData struct {
    ID          uint32   `json:"id"`          // arena state flag (primary key)
    Name        string   `json:"name"`
    Region      string   `json:"region"`
    Type        string   `json:"type"`        // "main" or "field"
    Remembrance bool     `json:"remembrance"`
    EventFlags  []uint32 `json:"eventFlags"`  // ALL flags to set on kill
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

- Re-key `bosses.go` map from defeat flag to arena state flag
- Import all 208 entries from `boss_data.py` (currently only ~120)
- Add DLC bosses with full flag lists
- Existing `SetBossDefeated(slotIndex, bossID, defeated)` keeps same API, just iterates all flags internally

### Testing

- Per-boss in-game verification needed (boss should not appear after kill)
- Priority: main bosses first (Godrick, Rennala, Radahn, Morgott, Fire Giant, Godfrey, Radagon/EB)
- Field bosses second (Tree Sentinel, Agheel, Tibia Mariner, etc.)

### Known complications

- Some bosses have conditional flags (e.g., Radahn requires festival flags)
- Multi-phase bosses (Godfrey/Hoarah Loux) may need intermediate state flags
- Boss respawn may not work for bosses that trigger irreversible world changes (Rykard → Volcano Manor)
