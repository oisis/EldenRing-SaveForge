# 19 — Weather & Time

> **Type**: Binary format spec  
> **Scope**: Weather state and in-game time.

---

## Overview

Two small structures of 12 bytes each — weather state and in-game time.

---

## WorldAreaWeather (12 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u16 | Area ID (weather region identifier) |
| 0x02 | u16 | Weather Type |
| 0x04 | u32 | Timer (duration of current weather) |
| 0x08 | u32 | Padding |

### Corruption detection
`Area ID == 0` → weather is likely corrupted. May cause visual glitches.

---

## WorldAreaTime (12 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u32 | Hour (0-23) |
| 0x04 | u32 | Minute (0-59) |
| 0x08 | u32 | Second (0-59) |

### Corruption detection
`Hour == 0 AND Minute == 0 AND Second == 0` → time potentially corrupted (although 00:00:00 is technically valid).

---

## Editing implications

- **Time**: changing the hour = changing time of day (affects NPC and event spawns)
- **Weather**: changing weather type — values map to internal game weather IDs
- Both structures safe to modify — game resets them during gameplay changes
- Weather corruption: zero Weather Type and set a valid Area ID

---

## Sources

- er-save-manager: `parser/world.py` — `WorldAreaWeather` (lines 810-838), `WorldAreaTime` (846-882)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — `WorldAreaWeather`, `WorldAreaTime` (lines 6-70)
