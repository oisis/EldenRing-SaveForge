# 14 — Game State

> **Type**: Binary format spec  
> **Scope**: Menu profile, tutorial data, GameMan bytes, death count, session info, last grace, NG+ cycle, play time.

---

## Overview

The Game State section contains various gameplay state data — from the death counter through character type to the last grace rest. It consists of several smaller sub-structures.

---

## Sub-section order

```
1. Unknown fields (2 × u32)                           8 bytes
2. Menu Profile SaveLoad                               [VARIABLE]
3. Trophy Equip Data                                   (fixed)
4. GaItem Game Data                                    8 + 7000×16 bytes
5. Tutorial Data                                       [VARIABLE]
6. GameMan bytes                                       3 bytes
7. Death/Character/Session state                       ~32 bytes (version-dependent)
```

---

## 1. Unknown Fields (8 bytes)

| Offset | Type | Field | Description (from CT) |
|---|---|---|---|
| 0x00 | u32 | ClearCount | **NG+ cycle** (0=Journey 1, 1=NG+1, ..., 7=NG+7) |
| 0x04 | u32 | unk_gamedataman_0x88 | Unknown (internal GameDataMan field) |

**ClearCount** — confirmed from CT (`GameDataMan -> +0x120`):
- Value 0 = first journey (Journey 1)
- Value 1–7 = NG+1 through NG+7
- Max NG+7 (Journey 8) — mechanics don't change after NG+7

---

## 2. Menu Profile SaveLoad [VARIABLE]

| Offset | Type | Field | Description |
|---|---|---|---|
| 0x00 | u16 | unk0x0 | Unknown |
| 0x02 | u16 | unk0x2 | Unknown |
| 0x04 | u32 | Size | Data size after header |
| 0x08 | u8[size] | Data | Menu profile data (typically 0x1000 = 4096 bytes) |

Total: 8 + size bytes (typically 0x1008)

---

## 3. Trophy Equip Data

Equipment data for trophy/achievement tracking. Fixed structure (size to verify).

---

## 4. GaItem Game Data (8 + 7000 × 16 = 112,008 bytes)

Table of 7000 entries describing the "history" of acquired items. **Critical** — every weapon/AoW must have an entry here, otherwise the game crashes (EXCEPTION_ACCESS_VIOLATION).

### Header:
| Offset | Type | Field | Description |
|---|---|---|---|
| 0x00 | i64 | Count | Number of uniquely acquired items |

### Entry (16 bytes):
| Offset | Type | Field | Description |
|---|---|---|---|
| 0x00 | u32 | ItemID | Item ID |
| 0x04 | u8 | ReinforceType | Reinforcement type (itemID % 100 — related to upgrade path) |
| 0x05 | u8[3] | Padding | Padding |
| 0x08 | u32 | NextItemID | Next ID in chain (linked list structure) |
| 0x0C | u8 | Unk | Unknown |
| 0x0D | u8[3] | Padding | Padding |

---

## 5. Tutorial Data [VARIABLE]

| Offset | Type | Field | Description |
|---|---|---|---|
| 0x00 | u16 | unk0x0 | Unknown |
| 0x02 | u16 | unk0x2 | Unknown |
| 0x04 | u32 | Size | Section size |
| 0x08 | u32 | Count | Number of completed tutorials |
| 0x0C | u32[Count] | TutorialIDs | IDs of completed tutorials |

---

## 6. GameMan Bytes (3 bytes)

| Offset | Type | Field | Description |
|---|---|---|---|
| 0x00 | u8 | gameman_0x8c | Unknown (state flag?) |
| 0x01 | u8 | gameman_0x8d | Unknown |
| 0x02 | u8 | gameman_0x8e | Unknown |

---

## 7. Death/Character/Session State

| Offset | Type | Field | Description |
|---|---|---|---|
| 0x00 | u32 | TotalDeathCount | **Total death count** for the character |
| 0x04 | u32 | CharacterType | Character type (0=Host, 1=WhitePhantom, 2=DarkSpirit, 3=Ghost) |
| 0x08 | u32 | InOnlineSession | Online session flag (0=offline, 1=in session) |
| 0x0C | u32 | CharacterTypeOnline | Character type in online (same as CharacterType) |
| 0x10 | u32 | LastRestedGrace | **Grace entity ID** (BonfireId) of last rest |
| 0x14 | u32 | NotAloneFlag | Not alone (1 = in co-op/invasion) |
| 0x18 | u32 | InGameTimer | Gameplay timer (countdown?) |
| 0x1C | u32 | unk_gamedataman | Unknown GameDataMan field |

### Version-dependent extra fields:
- `version >= 65`: + `temp_spawn_point_entity_id` (u32) — temporary spawn point
- `version >= 66`: + `game_man_0xcb3` (u8)

---

## Play Time (separate section in slot)

**Play Time** in milliseconds is stored in a separate slot field (not in this Game State section):
- CT offset: `GameDataMan -> +0xA0`
- Type: u32 (milliseconds from game start)
- Conversion: `hours = value / 3,600,000`

---

## Fields from CT confirmed as save-relevant

| Field | CT Offset | Save section | Editable | Effect |
|---|---|---|---|---|
| Death Count | GameDataMan+0x94 | Game State 7 [0x00] | Yes | Cosmetic (statistic) |
| Play Time | GameDataMan+0xA0 | Separate section | Yes | Time displayed in menu |
| NG+ Cycle | GameDataMan+0x120 | Game State 1 [0x00] | Yes | Journey number, enemy scaling |
| Last Grace | GameMan+0xB30 | Game State 7 [0x10] | Yes | Respawn point after death / fast travel |
| Target Grace | GameMan+0xB3C | — | To verify | Target grace (warp in progress?) |
| Save Slot Index | GameMan+0xAC0 | — | Runtime only | Current profile (not in per-slot save) |

---

## Editing implications

- **Death Count**: can be zeroed (purely cosmetic, doesn't affect gameplay)
- **Last Rested Grace**: change = player spawns at a different grace after loading (teleportation!)
- **NG+ Cycle**: changing 0→N = transition to Journey N+1 (enemy scaling, boss loot reset)
- **Play Time**: changes time displayed in menu (cosmetic)
- **Tutorial Data**: zeroing = tutorials shown again
- **GaItem Game Data**: MUST contain an entry for every owned weapon/AoW — missing = crash
- **CharacterType**: should be 0 (Host) in a normal save. Other values = multiplayer state
- **InOnlineSession**: should be 0 in an offline save

---

## Sources

- er-save-manager: `parser/world.py` — `MenuSaveLoad` (lines 237-267), `GaitemGameData` (275-335), `TutorialData` (372-402)
- er-save-manager: `parser/user_data_x.py` lines 139-165
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — Death Num, PlayTime, NG+ (ClearCount), LastGrace
- Cheat Engine: `ER_TGA_v1.9.0` — GameDataMan offsets, GameMan offsets, Save Slot Index
