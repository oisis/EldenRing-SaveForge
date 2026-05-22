# 02 — Slot — General Structure

> **Scope**: General layout of a single character slot (SaveSlot / UserDataX). Section ordering, fixed vs variable-length sections.

---

## Basic facts

- **Slot size**: 0x280000 bytes (2,621,440 bytes) — fixed, independent of content
- **Endianness**: Little-endian
- **Parsing**: Sequential — sections follow one after another, many have variable length
- **Version**: The first u32 of the slot. Determines the parsing variant (e.g., 5118 vs 5120 GaItems, additional fields in newer versions)

---

## Section order within a slot

The list below shows the **exact sequence** in which data is stored in the slot.
Sections marked `[VARIABLE]` have variable length — subsequent sections do not have fixed offsets.

```
Offset (approximate)   Section                              Size
──────────────────────────────────────────────────────────────────────────
0x00                    Slot Header (version + map + unk)    32 bytes
0x20                    GaItem Map                           [VARIABLE: 5118×var or 5120×var]
(dynamic)               PlayerGameData                       0x1B0 (432 bytes)
(dynamic)               SP Effects                           [VARIABLE: 13 entries]
(dynamic)               EquippedItems EquipIndex             0x58 (88 bytes)
(dynamic)               ActiveWeaponSlots + ArmStyle         0x1C (28 bytes)
(dynamic)               EquippedItems ItemIDs                0x58 (88 bytes)
(dynamic)               EquippedItems GaitemHandles          0x58 (88 bytes)
(dynamic)               Inventory Held                       [VARIABLE]
(dynamic)               Equipped Spells                      (fixed)
(dynamic)               Equipped Items (quick/pouch)         (fixed)
(dynamic)               Equipped Gestures                    (fixed)
(dynamic)               Acquired Projectiles                 [VARIABLE: count × 8]
(dynamic)               Equipped Armaments & Items           (fixed)
(dynamic)               Equipped Physics                     (fixed)
(dynamic)               Face Data                            0x12F (303 bytes)
(dynamic)               Inventory Storage Box                [VARIABLE]
(dynamic)               Gestures                             0x100 (256 bytes = 64 × u32)
(dynamic)               Unlocked Regions                     [VARIABLE: 4 + count×4]
(dynamic)               Torrent / RideGameData               0x28 (40 bytes)
(dynamic)               Control Byte                         1 byte
(dynamic)               Blood Stain                          0x44 (68 bytes)
(dynamic)               Unknown fields (2 × u32)            8 bytes
(dynamic)               Menu Profile SaveLoad                [VARIABLE: 8 + size]
(dynamic)               Trophy Equip Data                    (fixed)
(dynamic)               GaItem Game Data                     [VARIABLE: 8 + 7000×16]
(dynamic)               Tutorial Data                        [VARIABLE: 8 + size]
(dynamic)               GameMan bytes                        3 bytes
(dynamic)               Death/Character/Session state        [VARIABLE: ~32 bytes]
(dynamic)               Event Flags                          0x1BF99F (1,833,375 bytes)
(dynamic)               Event Flags Terminator               4 bytes
(dynamic)               FieldArea                            [VARIABLE: 4 + size]
(dynamic)               WorldArea                            [VARIABLE: 4 + size]
(dynamic)               WorldGeomMan (×2)                    [VARIABLE: 4 + size]
(dynamic)               RendMan                              [VARIABLE: 4 + size]
(dynamic)               Player Coordinates                   0x39 (57 bytes)
(dynamic)               GameMan spawn bytes                  ~12-20 bytes (version-dependent)
(dynamic)               NetMan                               0x20004 (131,076 bytes)
(dynamic)               WorldAreaWeather                     0x0C (12 bytes)
(dynamic)               WorldAreaTime                        0x0C (12 bytes)
(dynamic)               BaseVersion                          0x10 (16 bytes)
(dynamic)               Steam ID                             0x08 (8 bytes)
(dynamic)               PS5 Activity                         0x20 (32 bytes)
(dynamic)               DLC                                  0x32 (50 bytes)
(dynamic)               PlayerGameData Hash                  [remainder until slot end]
──────────────────────────────────────────────────────────────────────────
```

---

## Slot Header (32 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u32 | Version — save format version number |
| 0x04 | u8[4] | Map ID — identifier of the current map |
| 0x08 | u8[8] | Unknown |
| 0x10 | u8[16] | Unknown |

### Slot version

- `version == 0` → slot is empty
- `version <= 81` → old format (5118 GaItems)
- `version > 81` → new format (5120 GaItems)
- `version >= 65` → additional field `temp_spawn_point_entity_id`
- `version >= 66` → additional field `game_man_0xcb3`

---

## Variable-length sections — the key problem

Many sections have a size that depends on character state. This means **you cannot use fixed offsets** for sections after GaItem Map. They must be parsed sequentially.

Main sources of variability:
1. **GaItem Map** — record size depends on item type (weapon=21B, armor=16B, others=8B)
2. **Acquired Projectiles** — count × 8 bytes
3. **Unlocked Regions** — count × 4 bytes
4. **Inventory** — fixed slot count, but related counters
5. **World areas** — size-prefixed, variable
6. **GaItem Game Data** — 7000 entries but an 8-byte header

---

## Editing implications

1. **Modifying a fixed section** (e.g., PlayerGameData) — it is enough to write the new bytes in the same place
2. **Modifying a variable section** (e.g., Regions) — a size change requires shifting ALL subsequent sections
3. **Checksum** (PC) — after every modification the MD5 of the whole slot MUST be recomputed

---

## Starting classes — Base Stats Reference

| ID | Class | Start Lvl | Vig | Mnd | End | Str | Dex | Int | Fai | Arc | Sum |
|---|---|---|---|---|---|---|---|---|---|---|---|
| 0 | Vagabond | 9 | 15 | 10 | 11 | 14 | 13 | 9 | 9 | 7 | 88 |
| 1 | Warrior | 8 | 11 | 12 | 11 | 10 | 16 | 10 | 8 | 9 | 87 |
| 2 | Hero | 7 | 14 | 9 | 12 | 16 | 9 | 7 | 8 | 11 | 86 |
| 3 | Bandit | 5 | 10 | 11 | 10 | 9 | 13 | 9 | 8 | 14 | 84 |
| 4 | Astrologer | 6 | 9 | 15 | 9 | 8 | 12 | 16 | 7 | 9 | 85 |
| 5 | Prophet | 7 | 10 | 14 | 8 | 11 | 10 | 7 | 16 | 10 | 86 |
| 6 | Confessor | 10 | 10 | 13 | 10 | 12 | 12 | 9 | 14 | 9 | 89 |
| 7 | Samurai | 9 | 12 | 11 | 13 | 12 | 15 | 9 | 8 | 8 | 88 |
| 8 | Prisoner | 9 | 11 | 12 | 11 | 11 | 14 | 14 | 6 | 9 | 88 |
| 9 | Wretch | 1 | 10 | 10 | 10 | 10 | 10 | 10 | 10 | 10 | 80 |

**Formula**: `Level = Sum(all_attributes) - 79`

Values confirmed against two independent Cheat Engine tables (Hexinton + TGA).

---

## Sources

- er-save-manager: `parser/user_data_x.py` — class `UserDataX` with the full sequence of fields (lines 54-198)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — structs in parsing order
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — Class reset scripts (base stats)
- Cheat Engine: `ER_TGA_v1.9.0` — Class definitions
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=format:sl2
