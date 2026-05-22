# 12 — Torrent (Horse)

> **Scope**: Torrent data — position, HP, state.

---

## Overview

RideGameData stores the horse's (Torrent) state: its position in the world, hit points and activity state. Size: 40 bytes (0x28).

---

## Structure (40 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | f32 × 3 | Coordinates (x, y, z) — Torrent's position |
| 0x0C | u8[4] | Map ID (current map identifier) |
| 0x10 | f32 × 4 | Angle / Quaternion (orientation) |
| 0x20 | i32 | HP (hit points) |
| 0x24 | u32 | State |

---

## Horse State

| Value | State | Description |
|---|---|---|
| 1 | INACTIVE | Torrent is not summoned |
| 3 | DEAD | Torrent dead (requires Crimson Flask) |
| 13 | ACTIVE | Torrent summoned, the player is riding |

---

## Known bug: Infinite Loading Screen

**Bug condition**: `HP == 0` AND `State == ACTIVE (13)`

It should be: `HP == 0` AND `State == DEAD (3)`

This bug causes an infinite loading screen when entering the game. Fix: change State to 3 (DEAD) when HP == 0.

---

## Torrent Unlock

Torrent is unlocked by Event Flag **60100** (Spectral Steed Ring, received after meeting Melina at a Site of Grace).

Without that flag the player cannot summon Torrent even if they have the Spectral Steed Ring in their inventory.

---

## Torrent HP Scaling

Torrent's HP scales with the player's level. It is not a fixed value — the game calculates max HP based on player level. The exact formula is unknown, but:
- ~lvl 1: ~400 HP
- ~lvl 50: ~800 HP
- ~lvl 100+: ~1200+ HP

Revered Spirit Ash Blessing (DLC) increases Torrent's HP and damage negation in the Realm of Shadow.

---

## Editing implications

- Safe to modify
- **HP**: Value in the range 0 to Torrent's max HP. Above max = clamped by the game.
- **State**: Use ONLY known values (1, 3, 13). Others = undefined behavior.
- **Coordinates**: Changing teleports Torrent (but the game resets the position when summoning).
- **Bug fix**: ALWAYS correct State=3 (DEAD) when HP=0. This prevents the infinite loading.
- **Unlock check**: Verify event flag 60100 before setting State=ACTIVE.

---

## Sources

- er-save-manager: `parser/world.py` — `RideGameData` class (lines 126-173)
- er-save-manager: `parser/er_types.py` — `HorseState` enum
- er-save-manager: `parser/user_data_x.py` line 130: `horse: RideGameData`
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — Torrent coordinates (WorldChrMan chain)
- Elden Ring Wiki: https://eldenring.wiki.fextralife.com/Torrent
