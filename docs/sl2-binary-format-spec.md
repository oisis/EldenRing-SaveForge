# Full Technical Audit — EldenRing-SaveEditor

**Date**: 2026-04-11
**Scope**: Analysis of the `.sl2` binary format, read/write/edit algorithm, comparison with 3 reference editors, crash logs, binary comparison of save files.

---

## Table of Contents

1. [`.sl2` file format specification](#1-sl2-file-format-specification)
2. [File header structure](#2-file-header-structure)
3. [Character slot structure (UserDataX)](#3-character-slot-structure-userdatax)
4. [GaItems section — item registry](#4-gaitems-section--item-registry)
5. [Dynamic offset chain](#5-dynamic-offset-chain)
6. [Inventory system](#6-inventory-system)
7. [UserData10 — global data](#7-userdata10--global-data)
8. [UserData11 — game regulation](#8-userdata11--game-regulation)
9. [Checksums (MD5)](#9-checksums-md5)
10. [Encryption (AES)](#10-encryption-aes)
11. [PC vs PS4 differences](#11-pc-vs-ps4-differences)
12. [Slot integrity hash (PlayerGameDataHash)](#12-slot-integrity-hash-playergamedatahash)
13. [Save file read algorithm](#13-save-file-read-algorithm)
14. [Save file write algorithm](#14-save-file-write-algorithm)
15. [Character data editing algorithm](#15-character-data-editing-algorithm)
16. [Our code vs references analysis](#16-our-code-vs-references-analysis)
17. [Bugs found in our code](#17-bugs-found-in-our-code)
18. [Game crash analysis](#18-game-crash-analysis)
19. [Binary comparison of save files](#19-binary-comparison-of-save-files)
20. [Fix recommendations](#20-fix-recommendations)

---

## 1. `.sl2` file format specification

### Overview

The `.sl2` file (Elden Ring save file) is a binary container holding up to 10 character slots + global data + game regulation data. The format is **little-endian** throughout (except the DCX header in regulation, which is big-endian).

### Sizes

| Component | Size |
|---|---|
| Character slot (data) | `0x280000` (2,621,440 B) |
| MD5 checksum (PC only) | `0x10` (16 B) |
| PC slot total | `0x280010` (2,621,456 B) |
| PS4 slot total | `0x280000` (2,621,440 B) |
| PC header (BND4) | `0x300` (768 B) |
| PS4 header | `0x70` (112 B) |
| UserData10 (data) | `0x60000` (393,216 B) |
| UserData11 (data) | `0x240000` (2,359,296 B) |
| Number of slots | 10 |

### PC file layout

```
Offset          Size         Section
─────────────────────────────────────────────────────
0x000           0x300        BND4 Header
0x300           0x10         MD5 checksum slot 0
0x310           0x280000     Slot 0 data
0x280310        0x10         MD5 checksum slot 1
0x280320        0x280000     Slot 1 data
  ...           ...          ... (repeat for slots 2-9)
0x19003A0       0x10         MD5 checksum UserData10
0x19003B0       0x60000      UserData10 data
0x1F003B0       0x10         MD5 checksum UserData11
0x1F003C0       0x240000     UserData11 data
─────────────────────────────────────────────────────
```

**Formula for slot N offset (PC)**:
- Checksum: `0x300 + N * 0x280010`
- Data: `0x310 + N * 0x280010`

### PS4 file layout

```
Offset          Size         Section
─────────────────────────────────────────────────────
0x000           0x70         PS4 Header
0x70            0x280000     Slot 0 data
0x280070        0x280000     Slot 1 data
  ...           ...          ... (repeat for slots 2-9)
0x1900070       0x60000      UserData10 data (no MD5)
0x1960070       0x240000     UserData11 data (no MD5)
─────────────────────────────────────────────────────
```

---

## 2. File header structure

### PC — BND4 header (0x300 bytes)

Magic bytes at offset 0: `42 4E 44 34` = ASCII `"BND4"`

The BND4 header is a standard FromSoftware container with an entry table (12 entries):
- 10 × UserData000..009 (character slots)
- 1 × UserData010 (global data)
- 1 × UserData011 (regulation)

Each entry contains: data offset, data size, ID, name hash. File names in UTF-16LE.

### PS4 — simple header (0x70 bytes)

Magic bytes: `CB 01 9C 2C`

Fixed 112-byte header containing `{id, 0x7F7F7F7F}` pairs for entries 7-18.

---

## 3. Character slot structure (UserDataX)

Each slot is exactly `0x280000` bytes. The structure is sequential — fields are read in order, and each field's position depends on the size of preceding fields (including variable-length fields).

### Slot fields (sequential)

| Offset | Size | Field | Description |
|---|---|---|---|
| 0x00 | 4 | `version` | Format version (0 = empty slot) |
| 0x04 | 4 | `map_id` | Current map location |
| 0x08 | 24 | padding | Unknown |
| **0x20** | **variable** | **`gaitem_map`** | **GaItem table (5118 or 5120 entries)** |
| variable | 0x1B0 (432) | `player_game_data` | Stats, name, level |
| +0xD0 | 0xD0 (208) | `sp_effects` | 13 active effects |
| +0x58 | 0x58 (88) | `equipped_items_equip_index` | Equipment indices |
| +0x1C | 0x1C (28) | `active_weapon_slots` | Active weapon slots |
| +0x58 | 0x58 (88) | `equipped_items_item_id` | Item IDs |
| +0x58 | 0x58 (88) | `equipped_items_gaitem_handle` | GaItem handles |
| variable | variable | `inventory_held` | Inventory (common: 2688, key: 384 slots) |
| +0x74 | 0x74 (116) | `equipped_spells` | 14 spell slots + active |
| +0x8C | 0x8C (140) | `equipped_items` | 10 quick + 6 pouch |
| +0x18 | 0x18 (24) | `equipped_gestures` | 6 gestures |
| variable | variable | `acquired_projectiles` | count + count×8 bytes |
| +0x9C | 0x9C (156) | `equipped_armaments` | Full equipment state |
| +0x0C | 0x0C (12) | `equipped_physics` | Physick tears |
| +0x12F | 0x12F (303) | `face_data` | Character appearance |
| variable | variable | `inventory_storage_box` | Storage box (common: 1920, key: 128) |
| +0x100 | 0x100 (256) | `gesture_game_data` | 64 gestures |
| variable | variable | `unlocked_regions` | count + count×4 |
| +0x28 | 0x28 (40) | `ride_game_data` | Torrent (position, HP) |
| +1 | 1 | `control_byte` | |
| +0x44 | 0x44 (68) | `blood_stain` | Death location + runes |
| +8 | 8 | padding | |
| variable | variable | `menu_profile_save_load` | H+H+I header + data |
| +0x34 | 0x34 (52) | `trophy_equip_data` | |
| variable | variable | `gaitem_game_data` | i64 count + 7000 × 16B |
| variable | variable | `tutorial_data` | H+H+I header + data |
| +3 | 3 | `gameman_flags` | |
| +4 | 4 | `total_deaths` | Death counter |
| +4 | 4 | `character_type` | |
| +1 | 1 | `in_online_session` | |
| +4 | 4 | `character_type_online` | |
| +4 | 4 | `last_rested_grace` | |
| +1 | 1 | `not_alone_flag` | |
| +4 | 4 | `in_game_timer` | |
| +4 | 4 | padding | |
| **+0x1BF99F** | **0x1BF99F** | **`event_flags`** | **1,833,375 bytes of bitwise flags** |
| +1 | 1 | terminator | |
| variable | variable | 5× `UnknownList` | Size prefixes + data |
| +0x39 | 0x39 (57) | `player_coordinates` | Position + angle |
| +2 | 2 | padding | |
| +4 | 4 | `spawn_point_entity_id` | |
| +4 | 4 | padding | |
| +4 | 4 | `temp_spawn_point` | (version >= 65) |
| +1 | 1 | padding | (version >= 66) |
| +0x20004 | 0x20004 | `net_man` | Network data |
| +0x0C | 0x0C | `world_area_weather` | |
| +0x0C | 0x0C | `world_area_time` | |
| +0x10 | 0x10 | `base_version` | |
| +8 | 8 | `steam_id` | SteamID per-slot |
| +0x20 | 0x20 | `ps5_activity` | |
| +0x32 | 0x32 (50) | `dlc` | DLC flags |
| +0x80 | 0x80 (128) | `player_data_hash` | Integrity hash |
| variable | variable | padding | Padding to 0x280000 |

### PlayerGameData (432 bytes / 0x1B0)

Stat fields relative to **MagicOffset** (= `PlayerDataOffset`):

| Offset from MagicOffset | Type | Field |
|---|---|---|
| -379 | u32 | Vigor |
| -375 | u32 | Mind |
| -371 | u32 | Endurance |
| -367 | u32 | Strength |
| -363 | u32 | Dexterity |
| -359 | u32 | Intelligence |
| -355 | u32 | Faith |
| -351 | u32 | Arcane |
| -347 | u32 | Humanity (internal) |
| -335 | u32 | Level |
| -331 | u32 | Souls (Runes) |
| -327 | u32 | SoulMemory |
| -283 (0x11B) | 16×u16 | CharacterName (UTF-16LE) |
| -249 | u8 | Gender |
| -248 | u8 | Class (starting class) |
| -187 | u8 | ScadutreeBlessing (DLC) |
| -186 | u8 | ShadowRealmBlessing (DLC) |

**Level formula**: `level = vigor + mind + endurance + strength + dexterity + intelligence + faith + arcane - 79`

### Slot versioning

| Version | Change |
|---|---|
| ≤ 81 | GaItem count = 5118 |
| > 81 | GaItem count = 5120 |
| ≥ 65 | Added `temp_spawn_point_entity_id` (4B) |
| ≥ 66 | Added `game_man_0xcb3` (1B) |

---

## 4. GaItems section — item registry

### Location

The GaItems section starts at offset **0x20** in slot data and ends just before `PlayerGameData` (432 bytes = 0x1B0 before MagicOffset).

### Record format (variable length!)

This is the **most critical** structure in the save format. Incorrect parsing desynchronizes all subsequent data.

```
Base record: 8 bytes
┌──────────────────┬──────────────────┐
│ gaitem_handle (4B)│ item_id (4B)     │
└──────────────────┴──────────────────┘

If handle != 0 and type != AoW (0xC0):
  +8 bytes: unk2 (i32) + unk3 (i32) = 16 bytes total

  If type == Weapon (0x80):
    +5 bytes: aow_handle (i32) + unk5 (u8) = 21 bytes total
```

### Record sizes per type

| Type | Handle mask | Record size | Additional fields |
|---|---|---|---|
| **Weapon** | `0x80000000` | **21 B** | unk2, unk3, aow_handle, unk5 |
| **Armor** | `0x90000000` | **16 B** | unk2, unk3 |
| **Accessory** | `0xA0000000` | **8 B** | none |
| **Item/Goods** | `0xB0000000` | **8 B** | none |
| **Ash of War** | `0xC0000000` | **8 B** | none |
| **Empty** | `0x00000000` | **8 B** | (id = 0xFFFFFFFF) |

### Record type detection

Record type is determined by the **upper nibble** (4 bits) of the `gaitem_handle` field:

```
handle & 0xF0000000:
  0x80000000 → Weapon (21B)
  0x90000000 → Armor (16B)
  0xA0000000 → Accessory (8B)
  0xB0000000 → Item/Goods (8B)
  0xC0000000 → Ash of War (8B)
  0x00000000 → Empty (8B, id = 0xFFFFFFFF)
```

### Reference weapon record format (21B)

```
Offset  Size     Field
0x00    4        gaitem_handle     (e.g. 0x80010001)
0x04    4        item_id           (e.g. 0x003D0900 = Moonveil+0)
0x08    4        unk2              (usually -1 = 0xFFFFFFFF)
0x0C    4        unk3              (usually -1 = 0xFFFFFFFF)
0x10    4        aow_gaitem_handle (AoW handle or 0xFFFFFFFF)
0x14    1        unk5              (usually 0x00)
```

### Prefix system — Handle vs ItemID

**IMPORTANT**: There are **two independent prefix systems**:

| Type | Handle prefix | ItemID prefix |
|---|---|---|
| Weapon | `0x80xxxxxx` | `0x00xxxxxx` |
| Armor | `0x90xxxxxx` | `0x10xxxxxx` |
| Accessory | `0xA0xxxxxx` | `0x20xxxxxx` |
| Item/Goods | `0xB0xxxxxx` | `0x40xxxxxx` |
| Ash of War | `0xC0xxxxxx` | `0x60xxxxxx` |

### Entry count

- **Slot version ≤ 81**: 5118 entries (0x13FE)
- **Slot version > 81**: 5120 entries (0x1400)

Reference editors (Rust, Python) read a **fixed number of entries** rather than scanning to the end of the section. Our editor scans to MagicOffset, which is less safe.

---

## 5. Dynamic offset chain

### Anchor point — MagicPattern

64-byte pattern serving as an anchor in each slot:

```hex
00 FFFFFFFF 000000000000000000000000
   FFFFFFFF 000000000000000000000000
   FFFFFFFF 000000000000000000000000
   FFFFFFFF 000000000000000000000000
```

Pattern: 4× repetition of `[0x00, 0xFF,0xFF,0xFF,0xFF, 12×0x00]` (16 bytes × 4 = 64 bytes).

**MagicOffset** = position of this pattern in slot data. This is simultaneously the `PlayerDataOffset`.

### Offset chain (from MagicOffset)

The following chain defines positions of all data sections. Each offset is **additive** from the previous one:

```
PlayerDataOffset    = MagicOffset
                      │
                      ├── + 0xD0 ──→ SpEffect
                      ├── + 0x58 ──→ EquipedItemIndex
                      ├── + 0x1C ──→ ActiveEquipedItems
                      ├── + 0x58 ──→ EquipedItemsID
                      ├── + 0x58 ──→ ActiveEquipedItemsGa
                      ├── + 0x9010 ─→ InventoryHeld
                      ├── + 0x74 ──→ EquipedSpells
                      ├── + 0x8C ──→ EquipedItems
                      ├── + 0x18 ──→ EquipedGestures
                      │
                      ├── + DYNAMIC: projSize × 8 + 4  ← read from save data!
                      │
                      ├── + 0x9C ──→ EquipedArmaments
                      ├── + 0x0C ──→ EquipPhysics
                      ├── + 0x12F ─→ FaceData
                      ├── + 0x6010 ─→ StorageBox
                      ├── + 0x100 ─→ GestureGameData
                      │
                      ├── + DYNAMIC: unlockedRegSz × 4 + 4  ← read from save data!
                      │
                      ├── + 0x29 ──→ Horse (Torrent)
                      ├── + 0x4C ──→ BloodStain
                      ├── + 0x103C ─→ MenuProfile
                      ├── + 0x1B588 → GaItemDataOther
                      ├── + 0x40B ─→ TutorialData
                      ├── + 0x1A ──→ IngameTimer
                      └── + 0x1C0000 → EventFlags
```

### Two dynamic fields

1. **`projSize`** — read `uint32` from current position. Actual size = `projSize × 8 + 4`. Max: 256.
2. **`unlockedRegSz`** — read `uint32` from current position. Actual size = `unlockedRegSz × 4 + 4`. Max: 1024.

**CRITICAL**: Any error in calculating a position in the chain cascades and shifts ALL subsequent offsets, leading to reading/writing garbage.

---

## 6. Inventory system

### Inventory record (12 bytes)

```
┌──────────────────┬──────────────────┬──────────────────┐
│ gaitem_handle (4B)│ quantity (4B)    │ index (4B)       │
└──────────────────┴──────────────────┴──────────────────┘
```

### Capacities

| List | Common | Key | Total slots |
|---|---|---|---|
| **Inventory (held)** | 0xA80 (2688) | 0x180 (384) | 3072 |
| **Storage Box** | 0x780 (1920) | 0x80 (128) | 2048 |

### Inventory layout

```
common_item_count:  u32
common_items:       InventoryItem[capacity]  ← ALL slots always written
key_item_count:     u32
key_items:          InventoryItem[capacity]
equip_index_ctr:    u32                      ← NextEquipIndex
acquisition_ctr:    u32                      ← NextAcquisitionSortId
```

### Reserved indices

Indices `0–432` (`InvEquipReservedMax`) in the `CSGaItemIns` field are reserved for character equipment. New items must have an index > 432 to avoid collisions with the game's equipment system.

---

## 7. UserData10 — global data

### Layout (after MD5 checksum for PC)

```
Offset in UD10   Size        Field                    Platform
──────────────────────────────────────────────────────────────────
0x00             4           version                  Both
0x04             8           steam_id                 PC only
0x0C             0x140       Settings                 Both
0x14C            0x1808      MenuSystemSaveLoad       Both (15 presets)
                 ---         (different offsets for active_slots) ---
PC: 0x310        10          active_slots[10]         PC
PS4: 0x300       10          active_slots[10]         PS4
PC: 0x31A        10×0x24C    profile_summaries[10]    PC
PS4: 0x30A       10×0x24C    profile_summaries[10]    PS4
                 5           gamedataman fields       Both
PC only          0xB2        PCOptionData             PC only
                 variable    KeyConfigSaveLoad        Both
                 8           game_man field           Both
                 variable    padding to 0x60000       Both
```

### ProfileSummary (0x24C = 588 bytes)

```
character_name:  16×u16 (32B, UTF-16LE) + 2B terminator
level:           u32
seconds_played:  u32
runes_memory:    u32
map_id:          4B
unk:             u32
face_data:       0x124B (292B, shortened version)
equipment:       0xE8B (232B)
body_type:       u8
archetype:       u8
starting_gift:   u8
padding:         7B
```

### DLC in slot (50 bytes)

| Byte | Meaning |
|---|---|
| [0] | Pre-order gesture "The Ring" |
| [1] | Shadow of the Erdtree (non-zero = DLC entry, causes infinite loading without DLC) |
| [2] | Pre-order gesture "Ring of Miquella" |
| [3-49] | Unused (MUST be 0x00, non-zero = corruption) |

---

## 8. UserData11 — game regulation

### Layout

```
[16B unk header]
[0x1C5F70 regulation data]  ← encrypted AES-256-CBC + compressed DCX/zlib
[0x7A090 rest data]
```

Total size: `0x240000` bytes.

### Regulation decryption (AES-256-CBC)

**Note**: This is encryption **within regulation**, NOT `.sl2` file encryption.

- **Key (32 bytes)**: `99 BF FC 36 6A 6B C8 C6 F5 82 7D 09 36 02 D6 76 C4 28 92 A0 1C 20 7F B0 24 D3 AF 4E 49 3F EF 99`
- **IV**: first 16 bytes of ciphertext
- **Mode**: CBC, no padding
- After decryption: DCX container (FromSoftware compression) → zlib deflate → BND4 archive with `.param` files

---

## 9. Checksums (MD5)

### Applies ONLY to the PC platform

PS4 saves **have no checksums**.

### Algorithm

Standard MD5 (`crypto/md5` in Go, `hashlib.md5` in Python).

### What is checksummed

| Section | MD5 input data | Data size | MD5 location |
|---|---|---|---|
| Slot N | Slot data (0x280000 B) | 2,621,440 B | 16B immediately before slot data |
| UserData10 | UD10 data (0x60000 B) | 393,216 B | 16B immediately before UD10 data |
| UserData11 | UD11 data | variable | 16B immediately before UD11 data |

### Empty slot detection

If all 16 checksum bytes = 0x00 → slot is empty.

### Recalculation

After **every modification of slot data**, the MD5 must be recalculated and the new checksum written. All 3 reference editors do this.

---

## 10. Encryption (AES)

### Key finding

**None of the 3 reference editors encrypts/decrypts the `.sl2` file.**

The `.sl2` file on PC is normally saved as plaintext BND4. The AES-128-CBC encryption that our editor implements applies to the **Steam layer** — Steam on some platforms (Windows desktop) encrypts the file before writing to disk. On **Steam Deck** the file is NOT encrypted.

### Our AES-128-CBC code (crypto.go)

- **Key (16 bytes)**: `99 AD 2D 50 ED F2 FB 01 C5 F3 EC 3A 2B CA B6 9D`
- **IV**: first 16 bytes of the file
- **Mode**: CBC
- **Detection**: If the file starts with `"BND4"` → unencrypted. If after decryption it starts with `"BND4"` → encrypted.

### When NOT to encrypt

- PS4: never
- Steam Deck: save is plaintext `.sl2`
- PS4→PC conversion: our editor generates a random IV and encrypts — this is correct for Windows Steam, but unnecessary for Steam Deck

---

## 11. PC vs PS4 differences

| Aspect | PC | PS4 |
|---|---|---|
| Header | BND4, 0x300 B | Simple, 0x70 B |
| Magic bytes | `42 4E 44 34` ("BND4") | `CB 01 9C 2C` |
| File encryption | Optional (AES-128-CBC) | None |
| MD5 per slot | Yes (16B prefix) | No |
| MD5 on UD10/UD11 | Yes | No |
| SteamID in UD10 | Yes (offset 0x04) | No |
| SteamID in slot | Yes (last 8B) | No |
| ActiveSlots offset in UD10 | `0x310` | `0x300` |
| ProfileSummaries offset | `0x31A` | `0x30A` |
| PCOptionData in UD10 | Yes (0xB2 B) | No |
| Slot data | Identical `0x280000` B | Identical `0x280000` B |
| Internal slot structure | **Identical** | **Identical** |

**Conclusion**: Differences apply ONLY to the container (header, checksums, encryption). Character data inside the slot has an identical format on both platforms.

---

## 12. Slot integrity hash (PlayerGameDataHash)

### Location

Last 0x80 (128) bytes of each slot, offset `SlotSize - 0x80 = 0x27FF80`.

### Algorithm (Adler-like)

32 `uint32` fields (only 12 used), rest zeroed.

**Base function `computeHashedValue(input)`**:
```
product = uint64(0x80078071) × uint64(input)
upper = uint32(product >> 32)
shifted = upper >> 15
mod = int32(shifted) × (-0xFFF1)    // -65521 = Adler-32 modulus
return input + uint32(mod)
```

This is an `input mod 65521` operation (Adler-32 constant).

**Function `bytesHash(data)`**:
```
lo = 1, hi = 0
for each byte b:
    lo += b
    hi += lo
loH = computeHashedValue(lo)
hiH = computeHashedValue(hi)
return (loH | (hiH << 16)) × 2
```

### Hash entries

| Index | Contents |
|---|---|
| 0 | valueHash(Level) |
| 1 | statsHash(9 stats + Humanity) — **Int and Faith swapped!** |
| 2 | valueHash(Class) |
| 3 | bytesHash(PGD+0xB8 byte) |
| 4 | 0 (padding) |
| 5 | valueHash(Souls) |
| 6 | valueHash(SoulMemory) |
| 7 | equipmentHash(10 weapon slot IDs) |
| 8 | equipmentHash(4 armor + 5 talisman IDs) |
| 9 | quickItemsHash(16 quick/pouch IDs, & 0x0FFFFFFF) |
| 10 | equipmentHash(14 spell IDs) |
| 11 | 0 (padding) |

### Does the game validate the hash?

**Ambiguous**. Our code contains a comment "game doesn't validate", but simultaneously implements `RecalculateSlotHash()`. Reference editors (Rust) read the hash and can recalculate it. Our round-trip test excludes the hash region from comparison, suggesting the hash is incorrectly recalculated or zeroed.

---

## 13. Save file read algorithm

### Step by step (based on 3 reference editors)

```
1. Load entire file into memory (bytearray)

2. PLATFORM DETECTION:
   - Bytes 0-3 == "BND4" → PC (unencrypted)
   - After AES-128 decryption bytes 0-3 == "BND4" → PC (encrypted)
   - Bytes 0-3 == 0xCB019C2C → PS4
   
3. HEADER:
   PC:  read 0x300 bytes (BND4 header)
   PS4: read 0x70 bytes
   
4. FOR EACH SLOT (0-9):
   PC:  skip 0x10 (MD5), read 0x280000 bytes
   PS4: read 0x280000 bytes (no MD5)
   
   4a. Check version (u32 @ offset 0) — if 0, slot is empty
   
   4b. Parse GaItems:
       - Start: offset 0x20
       - Read handle (u32), check type, read appropriate number of bytes
       - Repeat for 5118/5120 entries (OR to end of section)
       - Remember handle → itemID map
   
   4c. Find MagicPattern (64B pattern) → set MagicOffset
   
   4d. Read PlayerGameData from MagicOffset + negative offsets
   
   4e. Calculate dynamic offset chain (2 dynamic fields)
   
   4f. Parse inventory, storage, event flags, coordinates, etc.

5. USERDATA10:
   PC:  skip 0x10 (MD5), read 0x60000 bytes
   PS4: read 0x60000 bytes
   - Parse active_slots, profile_summaries, SteamID

6. USERDATA11:
   PC:  skip 0x10 (MD5), read rest
   PS4: read rest
```

---

## 14. Save file write algorithm

### Step by step (based on references)

```
1. PRE-SAVE VALIDATION:
   - Check len(slot.Data) == 0x280000
   - Verify offset chain correctness
   - Check stat bounds (level 1-713, attributes 1-99)

2. FLUSH METADATA:
   - Write active_slots to UserData10
   - Write profile_summaries to UserData10
   - Write SteamID to UserData10 (PC only)

3. FOR EACH SLOT:
   - Write stats to slot.Data (MagicOffset + offsets)
   - Write SteamID at end of slot (PC only, offset SlotSize - 8)
   
4. SERIALIZATION:
   PC:
   - BND4 header (0x300)
   - For each slot: MD5(slot.Data) + slot.Data
   - MD5(UD10.Data) + UD10.Data
   - MD5(UD11) + UD11
   
   PS4:
   - PS4 header (0x70)
   - For each slot: slot.Data (no MD5)
   - UD10.Data (no MD5)
   - UD11

5. ENCRYPTION (optional, PC only):
   - If save was encrypted: AES-128-CBC encrypt with preserved IV

6. ATOMIC WRITE:
   - Write to .tmp file
   - Rename .tmp → target file
```

---

## 15. Character data editing algorithm

### Changing stats

```
1. Modify values in PlayerGameData structure
2. Write new values to slot.Data[MagicOffset + offset]
3. Recalculate level: sum(all_stats) - 79
4. Update ProfileSummary in UserData10 (level, name)
```

### Adding items

```
1. ADD GaItem RECORD (section 0x20+):
   a. Determine type from itemID prefix → handle prefix
   b. Generate unique handle (or use existing for stackable)
   c. Write record of appropriate length (8/16/21B) at InventoryEnd
   d. Update InventoryEnd
   e. CLEAR remaining bytes to section boundary (prevent desync)

2. ADD TO GaItemData (GaItemDataOther section):
   a. For weapons/AoW: add entry (id, unk, reinforce_type, unk1)
   b. Increment counter

3. ADD TO INVENTORY:
   a. Find empty slot (handle == 0 or 0xFFFFFFFF)
   b. Set handle, quantity, index
   c. Index MUST be > 432 (InvEquipReservedMax)
   d. Increment NextAcquisitionSortId
```

### Removing items

```
1. Zero out matching entries in inventory/storage
2. If handle doesn't exist in any list → remove from GaMap
```

---

## 16. Our code vs references analysis

### Correct implementations

| Aspect | Status | Notes |
|---|---|---|
| Platform detection (magic bytes) | ✅ | PC/PS4 correctly identified |
| BND4/PS4 header | ✅ | Correct sizes and structure |
| AES-128-CBC decrypt/encrypt | ✅ | Key and mode correct |
| MD5 per-slot checksums | ✅ | Correctly recalculated |
| MagicPattern search | ✅ | 64B pattern, fallback offset |
| Stat offsets (negative from Magic) | ✅ | Consistent with references |
| Inventory held/storage layout | ✅ | Capacities and format match |
| GaItem record sizes | ✅ | 21/16/8 bytes |
| Dynamic offset chain | ✅ | Fixed offsets match references |
| Profile summaries | ✅ | Correct PC/PS4 offsets |
| Atomic write (tmp + rename) | ✅ | Correct implementation |
| Pre-save validation | ✅ | Checks bounds, offsets |

### Key differences

| Aspect | Our code | References |
|---|---|---|
| **GaItem scan** | Scans from 0x20 to MagicOffset-0x1B0 | Reads fixed entry count (5118/5120) |
| **ReadBytes** | Returns slice (aliasing) | Data copy |
| **Hash recalculation** | NOT called in write path | Some references recalculate |
| **Version field check** | We don't check version per slot | References check (0 = empty, >81 = new format) |
| **Write strategy** | Modify buffer in-place | er-save-manager: raw-data patching with optional full rebuild |

---

## 17. Bugs found in our code

### 🔴 CRITICAL (cause game crashes)

#### BUG-1: No data shift after adding GaItems

**File**: `backend/core/writer.go` → `writeGaItem()`

**Problem**: Adding new GaItem records advances `InventoryEnd` forward, but data after the GaItems section (PlayerGameData, equipment, event flags, etc.) **is not shifted** in the buffer. The game calculates offsets dynamically based on GaItems section size, so after adding items the game looks for stats/equipment data at wrong offsets.

**Effect**: The game reads garbage as pointers and dereferences address `0xFFFFFFFFFFFFFFFF` → crash `page fault on read access`.

**Note**: This is the main cause of crashes from the `analiza/` directory. The stack shows value `0x808000ba` — an invalid GaItemHandle.

#### BUG-2: No GaItem count versioning

**File**: `backend/core/structures.go` → `scanGaItems()`

**Problem**: Our scanner does not check the `version` field (offset 0x00 in slot) and always scans to MagicOffset. References read a fixed number of entries: 5118 (version ≤ 81) or 5120 (version > 81). Scanning "to end" is risky — garbage bytes may be misinterpreted as records.

#### BUG-3: Incorrect handles for arrows/bolts

**File**: `backend/core/writer.go` → `AddItemsToSlot()`

**Problem**: Arrows/bolts (item ID prefix 0x02/0x03) may be treated as weapons (`ItemTypeWeapon`), producing 21-byte GaItem records instead of 8-byte. This desynchronizes the entire GaItems scanner.

### 🟡 IMPORTANT (may cause issues)

#### BUG-4: Hash offset chain doesn't account for dynamic projSize

**File**: `backend/core/hash.go` → `ComputeSlotHash()`

**Problem**: Calculating offsets for hash entries [9] (quickItems) and [10] (spells) skips the dynamic `projSize`. Uses a simplified offset chain that doesn't read `projSize` from save data. The hash will be computed from incorrect data.

**Impact**: Currently no impact, since `RecalculateSlotHash()` is not called in the write path. But will become critical when hash is enabled.

#### BUG-5: quickItemsHash reads from wrong offset

**File**: `backend/core/hash.go` → `readQuickItemIDs()`

**Problem**: Reads 16 × u32 from the beginning of the `equipedItems` section without skipping the ChrAsmEquipment header. Reads equipment data instead of quick items.

#### BUG-6: upsertGaItemData always sets reinforce_type = 0

**File**: `backend/core/writer.go` → `upsertGaItemData()`

**Problem**: New GaItemData entries have `reinforce_type = 0` regardless of weapon upgrade level. If the game checks this field, a +10 weapon may behave as +0.

#### BUG-7: Undo doesn't preserve unexported offset fields

**File**: `app.go` → `pushUndo()`

**Problem**: Deep-copy of `Inventory`/`Storage` creates new structs without setting `nextEquipIndexOff`/`nextAcqSortIdOff`. After revert, `addToInventory` checks `if slot.Inventory.nextAcqSortIdOff > 0` — condition is false → counter write-back doesn't work → stale acquisition sort IDs.

### 🟢 MINOR

#### BUG-8: ReadStorage breaks on first empty handle

**Problem**: If storage has sparse slots (handle=0 in the middle), items after the gap are lost.

#### BUG-9: EquipInventoryData.Read ignores errors

**Problem**: All `_ = err` patterns in `ReadU32`. A truncated slot silently produces incorrect data.

#### BUG-10: ComputeSHA256 is dead code

**File**: `backend/core/crypto.go`

**Problem**: Declared but never used anywhere.

---

## 18. Game crash analysis

### Source: `analiza/steam-1245620.log.*.txt`

Crashes occur on **Steam Deck** (Linux, Proton 10.0-4).

### Crash signature

```
wine: Unhandled page fault on read access to FFFFFFFFFFFFFFFF
at address 000000014067150A (thread 01b0/01b4)

rip: 0x14067150a (eldenring.exe + 0x67150a)
r14: 0xffffffff
```

### Diagnosis

1. The value `0xFFFFFFFF` in register r14 is the "invalid handle" sentinel from the GaItems section
2. The game tries to dereference this handle as a pointer → page fault
3. Stack shows `0x808000ba` — invalid handle format (correct is `0x80xxxxxx` with sequential lower word)

### Root cause

**BUG-1** (see section 17): Adding items to GaItems advances the section boundary, but doesn't shift data after it. The game calculates `PlayerDataOffset = GaItems_End + 0x1B0` dynamically and hits garbage data.

### Files without crash

`steam-1245620.log.7.txt` — normal game startup. Likely used an unmodified save.

---

## 19. Binary comparison of save files

### Files

- **Original**: `tmp/save/ER0000.sl2` (28,967,888 B)
- **Edited**: `tmp/save/ER0000-out.sl2` (28,967,888 B)

### Results

| Metric | Value |
|---|---|
| Size | Identical (28,967,888 B) |
| Total differences | **20,249 bytes** (0.07%) |
| Modified slots | **Slot 0** and **Slot 4** |
| BND4 header | Identical |
| UserData10 | Identical |
| UserData11 | Identical |

### Change distribution

#### Slot 0 (6 clusters, ~650 B of data changes)

| File offset | Slot offset | Size | Description |
|---|---|---|---|
| 0x300-0x30F | — | 16B | MD5 checksum (recalculated) |
| 0xBBEE-0xC0C7 | 0xB8DE | 630B | Inventory region (added items) |
| 0xCD46-0xCD4D | 0xCA36 | 5B | Small edit |
| 0x1547C-0x1547D | 0x1516C | 2B | Small edit |
| 0x1B782-0x1B78F | 0x1B472 | 7B | Small edit |
| 0x2178E-0x2178F | 0x2147E | 2B | Small edit |
| 0x37EBF-0x37EC9 | 0x37BAF | 4B | Small edit |

#### Slot 4 (6 clusters, ~19,570 B of data changes)

| File offset | Slot offset | Size | Description |
|---|---|---|---|
| 0xA00340-0xA0034F | — | 16B | MD5 checksum (recalculated) |
| **0xA00C9A-0xA0A53B** | **0x94A** | **19,546B** | **Bulk item addition** |
| 0xA0AA22-0xA0AA29 | 0xA6D2 | 5B | Small edit |
| 0xA138F0-0xA138F1 | 0x135A0 | 2B | Small edit |
| 0xA19BF6-0xA19C05 | 0x198A6 | 9B | Small edit |
| 0xA1FC02-0xA1FC02 | 0x1F8B2 | 1B | Single byte |
| 0xA36333-0xA3633D | 0x35FE3 | 4B | Small edit |

### Conclusions from comparison

1. **MD5 checksums recalculated correctly** — 16B change at the beginning of each modified slot
2. **Main change in Slot 4** (19,546B) is the GaItems region (offset 0x94A in slot, starts close to 0x20) — visible pattern `00000000 FFFFFFFF` (empty slots) replaced with structured item data
3. **Data does NOT exceed slot boundaries** — no header/UD10 corruption
4. **However**: added items advance `InventoryEnd` without shifting subsequent sections — this is BUG-1

---

## 20. Fix recommendations

### Priority 1 — CRITICAL (fix crashes)

#### R-1: Implement full slot rebuild instead of in-place patching

Modeled after `er-save-manager/parser/slot_rebuild.py`:

```
Instead of: modify buffer[InventoryEnd] and hope the rest aligns
Do:    
  1. Parse entire slot into data structures
  2. Modify structures (add/remove items)
  3. Serialize EVERYTHING fresh into a 0x280000 buffer
  4. Fill remainder with zeros
```

This eliminates BUG-1 fundamentally — there's no data shift problem because the entire slot is written from scratch.

**Cost**: Major refactor. Requires full parsing and serialization of every section.

#### R-2: Alternative — shift data after GaItems

If a full rebuild is too costly:

```
1. Calculate delta = newInventoryEnd - oldInventoryEnd
2. Shift bytes[oldInventoryEnd:] by delta to the right
3. Update ALL offsets in the dynamic chain
4. Recalculate MagicOffset (MagicPattern has moved!)
```

**Risk**: Every missed offset = corruption. The chain has 20+ fields + 2 dynamic ones.

#### R-3: Fixed GaItem count instead of scanning

```go
// Instead of:
func (s *SaveSlot) scanGaItems(start int) { ... scan to magic ... }

// Do:
version := binary.LittleEndian.Uint32(s.Data[0:4])
count := 5120
if version <= 81 { count = 5118 }
for i := 0; i < count; i++ { ... read fixed count ... }
```

### Priority 2 — IMPORTANT

#### R-4: Fix record size for arrows/bolts

Arrows/bolts are `ItemTypeItem` (goods), not `ItemTypeWeapon`. They should have 8B GaItem records.

#### R-5: Enable RecalculateSlotHash with correct offset chain

Fix `ComputeSlotHash()` to account for dynamic `projSize`, then enable in the write path.

#### R-6: Fix undo snapshot

Copy `nextEquipIndexOff` and `nextAcqSortIdOff` during deep-copy. Requires exporting these fields or adding a `Clone()` method.

### Priority 3 — IMPROVEMENTS

#### R-7: Raw-data patching strategy (like er-save-manager)

Instead of building the entire file from scratch, keep the original `[]byte` and modify in-place with tracked offsets. Safer than full serialization because you don't need to know the format of every section — you only change what's needed.

#### R-8: Version field validation

Check `version` at offset 0 of each slot. Version 0 = empty slot. Version > 81 = new format (5120 GaItems).

#### R-9: DLC flags handling during conversion

DLC byte[1] (Shadow of Erdtree entry flag) should be zeroed during conversion if the target platform doesn't have DLC. Non-zero = infinite loading.

---

## Appendix A: Constants and magic numbers (full list)

```
// Sizes
SlotSize                = 0x280000  // 2,621,440
PCHeaderSize            = 0x300     // 768
PSHeaderSize            = 0x70      // 112
MD5Size                 = 0x10      // 16
UserData10Size          = 0x60000   // 393,216
UserData11Size          = 0x240000  // 2,359,296
EventFlagsSize          = 0x1BF99F  // 1,833,375
NetManDataSize          = 0x20000   // 131,072
FaceDataSize            = 0x12F     // 303
FaceDataProfileSize     = 0x120     // 288 (shortened version in profile summary)
ProfileSummarySize      = 0x24C     // 588

// GaItems
GaItemsStart            = 0x20
GaItemCountOld          = 0x13FE    // 5118 (version ≤ 81)
GaItemCountNew          = 0x1400    // 5120 (version > 81)
GaItemGameDataEntries   = 7000      // 0x1B58
GaItemDataEntrySize     = 16        // id(4) + unk(4) + reinforce(4) + unk1(4)
WeaponRecordSize        = 21
ArmorRecordSize         = 16
DefaultRecordSize       = 8

// Inventory
HeldCommonCapacity      = 0xA80     // 2688
HeldKeyCapacity         = 0x180     // 384
StorageCommonCapacity   = 0x780     // 1920
StorageKeyCapacity      = 0x80      // 128
InvRecordSize           = 12        // handle(4) + qty(4) + idx(4)
InvEquipReservedMax     = 432

// Handle masks
HandleWeapon            = 0x80000000
HandleArmor             = 0x90000000
HandleAccessory         = 0xA0000000
HandleItem              = 0xB0000000
HandleAoW               = 0xC0000000
HandleEmpty             = 0x00000000
HandleInvalid           = 0xFFFFFFFF
HandleTypeMask          = 0xF0000000

// ItemID prefixes
ItemIDWeapon            = 0x00000000
ItemIDArmor             = 0x10000000
ItemIDAccessory         = 0x20000000
ItemIDGoods             = 0x40000000
ItemIDAoW               = 0x60000000

// Crypto
AES128Key               = [16]byte{0x99, 0xAD, 0x2D, 0x50, 0xED, 0xF2, 0xFB, 0x01,
                                    0xC5, 0xF3, 0xEC, 0x3A, 0x2B, 0xCA, 0xB6, 0x9D}
RegulationAES256Key     = [32]byte{0x99, 0xBF, 0xFC, 0x36, ...}

// Hash
HashMagic               = 0x80078071
HashSize                = 0x80      // 128
HashOffset              = SlotSize - HashSize  // 0x27FF80
HashEntries             = 12
AdlerModulus            = 65521

// Platform magic bytes
BND4Magic               = "BND4"    // 0x42 0x4E 0x44 0x34
PS4Magic                = []byte{0xCB, 0x01, 0x9C, 0x2C}

// Dynamic chain (fixed offsets)
DynSpEffect             = 0xD0
DynEquipedItemIndex     = 0x58
DynActiveEquipedItems   = 0x1C
DynEquipedItemsID       = 0x58
DynActiveEquipedItemsGa = 0x58
DynInventoryHeld        = 0x9010
DynEquipedSpells        = 0x74
DynEquipedItems         = 0x8C
DynEquipedGestures      = 0x18
DynEquipedArmaments     = 0x9C
DynEquipePhysics        = 0x0C
DynFaceData             = 0x12F
DynStorageBox           = 0x6010
DynGestureGameData      = 0x100
DynHorse                = 0x29
DynBloodStain           = 0x4C
DynMenuProfile          = 0x103C
DynGaItemsOther         = 0x1B588
DynTutorialData         = 0x40B
DynIngameTimer          = 0x1A
DynEventFlags           = 0x1C0000

// Dynamic field limits
MaxProjSize             = 256
MaxUnlockedRegSz        = 1024
```

---

## Appendix B: Reference editors — summary

| Editor | Language | .sl2 encryption | Checksums | GaItem parsing | Slot rebuild |
|---|---|---|---|---|---|
| **Elden-Ring-Save-Editor** | Python | ❌ None | ✅ MD5 | Pattern scanning | ❌ |
| **ER-Save-Editor** | Rust | ❌ None | ✅ MD5 | Fixed count (5118/5120) | ✅ Full |
| **er-save-manager** | Python | ❌ None | ✅ MD5 | Fixed count (5118/5120) | ✅ Full |
| **Our editor** | Go | ✅ AES-128 | ✅ MD5 | Scan to Magic | ❌ In-place |

### Key lessons from references

1. **No reference editor encrypts/decrypts the .sl2 file** — all operate on plaintext BND4
2. **Rust and Python (er-save-manager) use full slot rebuild** — they serialize all sections from scratch
3. **Fixed GaItem entry count** (not scanning) is safer
4. **Version field** (offset 0) is checked — 0 = empty slot, value affects GaItem count

---

*Document generated based on analysis of project source code, 3 reference editors (Elden-Ring-Save-Editor/Python, ER-Save-Editor/Rust, er-save-manager/Python), game crash logs, and binary comparison of save files.*
