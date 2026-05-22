# 18 — Network Manager (Network Data)

> **Scope**: Multiplayer data — 131 KB opaque blob.

---

## Overview

NetMan stores multiplayer session data. A large block (131,076 bytes = 0x20004), largely unexplored.

---

## Structure (131,076 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u32 | Unknown (unk0x0) — probably a state flag |
| 0x04 | u8[0x20000] | Data (131,072 bytes) — opaque network state |

---

## Spawn Point Data (immediately after RendMan, before NetMan)

Between Player Coordinates and NetMan there are additional GameMan fields:

| Offset | Type | Description |
|---|---|---|
| +0 | u8 | game_man_0x5be |
| +1 | u8 | game_man_0x5bf |
| +2 | u32 | spawn_point_entity_id (Grace entity ID for respawn) |
| +6 | u32 | game_man_0xb64 |
| +10 | u32 | temp_spawn_point_entity_id (version >= 65) |
| +14 | u8 | game_man_0xcb3 (version >= 66) |

---

## Editing implications

- **spawn_point_entity_id**: changing = the player respawns at a different Site of Grace
- Network data: typically not edited — session data is ephemeral
- The whole blob can be zeroed out safely (multiplayer state reset)

---

## Sources

- er-save-manager: `parser/world.py` — `NetMan` class (lines 785-802)
- er-save-manager: `parser/user_data_x.py` lines 178-186 (GameMan spawn fields)
