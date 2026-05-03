# 17 — Player Coordinates

> **Type**: Binary format spec  
> **Scope**: Player position in the 3D world, map identifier, rotation angle.

---

## Overview

PlayerCoordinates stores the player's exact position at the time of save. Size: 57 bytes (0x39).

---

## Structure (57 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | f32 × 3 | Coordinates (x, y, z) — main position |
| 0x0C | u8[4] | Map ID — current map identifier |
| 0x10 | f32 × 4 | Angle — rotation quaternion (x, y, z, w) |
| 0x20 | u8 | game_man_0xbf0 (unknown) |
| 0x21 | f32 × 3 | Unknown Coordinates (backup? spawn?) |
| 0x2D | f32 × 4 | Unknown Angle |

---

## Map ID Format

Map ID is 4 bytes encoding location in the world hierarchy:

| Byte | Description |
|---|---|
| 0 | Region / Area type |
| 1 | Y coordinate (grid) |
| 2 | X coordinate (grid) |
| 3 | Layer (0x3C=overworld, 0x3D+=underground) |

---

## Known Map ID values

| Byte[3] (Layer) | Description |
|---|---|
| 0x3C | Overworld (surface) |
| 0x3D | Underground level 1 (Siofra, Ainsel, Deeproot) |
| 0x3E | Underground level 2 (Lake of Rot, Mohgwyn) |

| Byte[0] (Region) | Description (approximate) |
|---|---|
| 0x3C | Limgrave, Stormveil |
| 0x3D | Liurnia |
| 0x3E | Altus, Leyndell |
| 0x3F | Caelid |
| 0x40 | Mountaintops |

**Note**: Exact mapping requires verification — the above is approximate from save file observations.

---

## Teleportation via editing — safety rules

1. **Map ID must exist** — invalid ID = infinite loading or crash
2. **Coordinates must be within map bounds** — out-of-bounds position = falling death → respawn at last grace
3. **Y (vertical)** is critical — too low = under the map, too high = falling
4. **Last Rested Grace** (spec/14) is a safer teleportation alternative
5. **Unknown coordinates** (offset 0x21) are likely "stable ground position" — backup for unstuck

---

## Editing implications

- Changing Coordinates = direct player teleportation
- Map ID must correspond to a valid map — wrong ID = crash/infinite loading
- **Safe teleportation**: better to change LastRestedGrace (Game State) than raw coords
- Second set of coordinates: likely spawn point / last stable position
- Quaternion: (0, 0, 0, 1) = no rotation (facing north)
- game_man_0xbf0: likely "on ground" / "in air" flag

---

## Sources

- er-save-manager: `parser/world.py` — class `PlayerCoordinates` (lines 747-776)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — `PlayerCoords` struct (lines 73-119)
- er-save-manager: `parser/user_data_x.py` line 175: `player_coordinates: PlayerCoordinates`
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — coordinates via WorldChrMan (runtime, f32 x/y/z)
- Cheat Engine: `ER_TGA_v1.9.0` — FieldArea MapID at +0x2C
