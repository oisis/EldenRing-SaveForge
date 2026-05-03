# 99 — Verification & Discovery Methodology

> **Type**: Research / QA  
> **Scope**: Test procedures for verifying documented offsets and discovering unknown parameters in save files.

---

## Available test resources

| Resource | Path | Description |
|---|---|---|
| PC Save | `tmp/save/ER0000.sl2` | Real PC save (multiple slots) |
| PS4 Save | `tmp/save/oisisk_ps4.txt` | Real PS4 save |
| Reference parser | `tmp/repos/er-save-manager/` | Python parser (ground truth) |
| Cheat Tables | `tmp/cheat_tables/` | Runtime offsets (Hexinton + TGA) |
| BST Lookup | `tmp/repos/er-save-manager/src/resources/eventflag_bst.txt` | Event flag addressing |
| Our parser | `backend/core/reader.go` | Go parser (for comparison) |

---

## Method 1: Hex Dump — Known Value Search

**Goal**: Find a field's offset in the save file based on a known value.

### Procedure:
1. Load save through our parser → extract known values (name, level, stats)
2. Hex dump the slot to file
3. Search for the known value pattern (little-endian)
4. Confirm offset — compare with documentation

### Example — searching for Character Name:
```bash
# Name "Zofia" in UTF-16LE = 5A 00 6F 00 66 00 69 00 61 00
xxd -s $SLOT_START -l 0x280000 save.sl2 | grep "5a00 6f00 6600"
```

### Example — searching for Level (e.g. level 150 = 0x96 = 96 00 00 00 LE):
```bash
# Level 150 in u32 LE
xxd -s $SLOT_START -l 0x280000 save.sl2 | grep "9600 0000"
```

---

## Method 2: Binary Diff — Before/After Action

**Goal**: Discover which bytes change after a specific in-game action.

### Procedure:
1. Backup save BEFORE action
2. Perform one specific action in-game (e.g. level up, pick item, kill boss)
3. Save the game
4. Binary diff the two saves → list of changed bytes
5. Filter out known changes (checksum, timestamps) → new discoveries remain

### Tool:
```bash
# Extract specific slot (e.g. slot 0, PC with MD5)
dd if=save_before.sl2 bs=1 skip=$((0x300 + 0x10)) count=$((0x280000)) of=slot0_before.bin
dd if=save_after.sl2 bs=1 skip=$((0x300 + 0x10)) count=$((0x280000)) of=slot0_after.bin

# Diff
cmp -l slot0_before.bin slot0_after.bin | head -50
# Format: offset byte_before byte_after
```

### Test actions to perform:
- Level up (1 attribute) → change in PlayerGameData
- Pick up item → change in Inventory + Event Flags
- Kill boss → change in Event Flags
- Rest at grace → change in Game State (LastGrace) + HP/FP/SP
- Change time of day → change in WorldAreaTime
- Equip/unequip item → change in Equipment structures

---

## Method 3: Cross-Slot Comparison

**Goal**: Compare the same offsets between different slots (characters) to identify per-character vs constant fields.

### Procedure:
1. Extract N slots from one save file
2. Compare byte-by-byte at the same offsets
3. Identical fields = constant/template
4. Different fields = per-character data

### What to look for:
- Offsets where the value corresponds to level differences between characters
- Offsets where one character's name appears but not another's
- Zero blocks in one slot but non-zero in another (unused vs used features)

---

## Method 4: Parser Comparison (er-save-manager vs ours)

**Goal**: Compare parsing results of the same save by Python reference and our Go parser.

### Procedure:
```bash
# Python parser (reference)
cd tmp/repos/er-save-manager
python -c "
from er_save_manager.parser.save import Save
s = Save.from_file('../../tmp/save/ER0000.sl2')
slot = s.slots[0]
print(f'Version: {slot.version}')
print(f'Level: {slot.player_game_data.soul_lv}')
print(f'Name: {slot.player_game_data.player_name}')
print(f'HP: {slot.player_game_data.hp}')
print(f'Vigor: {slot.player_game_data.vigor}')
# ... etc
"

# Our Go parser
go test -v -run TestParseComparison ./tests/
```

### Compared values:
- All PlayerGameData fields
- GaItem count and first/last entries
- Inventory count + first items
- Event flag spot-checks (known flags)
- Dynamic offsets (MagicOffset, EventFlagsOffset, etc.)

---

## Method 5: Targeted Byte Probing (Unknown Fields)

**Goal**: Discover meaning of unknown fields by systematically modifying and observing in-game effect.

### Procedure:
1. Select an unknown byte/field
2. Note current value
3. Change to an extreme value (0x00, 0xFF, or inverse)
4. Load save in game
5. Observe: crash? different behavior? no effect?
6. Document result

### Safety rules:
- ALWAYS work on a COPY of the save (never original)
- Change ONE field at a time
- Test "safe" values first (0, max) before random ones
- If crash → field is critical, note it
- If no effect → field is likely runtime-only or unused

---

## Method 6: Pattern Recognition (MagicPattern Anchor)

**Goal**: Use the known 192-byte MagicPattern as an anchor to calculate offsets.

### Procedure:
1. Find MagicPattern in slot (192 bytes of known values)
2. Calculate PlayerGameData offset = MagicPattern + 192 + N (where N depends on version)
3. From that point — verify fields sequentially

### MagicPattern (hex):
```
00 FF FF FF FF 00 00 00 00 00 00 00 00 00 00 00 00
FF FF FF FF 00 00 00 00 00 00 00 00 00 00 00 00 00
(repeats 12× = 192 bytes total)
```

---

## Method 7: Event Flag Spot-Check

**Goal**: Verify the BST algorithm on specific, known flags.

### Procedure:
1. Take a save with a defeated boss (e.g. Margit — flag 71001)
2. Calculate offset via BST: block=71, index=1, lookup BST[71], byte=offset×125+0, bit=7-1=6
3. Check if the bit is set in the hex dump of Event Flags section
4. Repeat for several known flags

### Spot-check flags:
- 60100 (Torrent) — mechanic, easy to verify
- 71001 (Margit killed) — boss
- 76101 (The First Step grace) — grace
- 62010 (Limgrave West map) — map

---

---

# VERIFICATION CHECKLIST

## Status: ✅ = verified, ⏳ = in progress, ❌ = to do, ❓ = unknown/to discover

---

## A. PlayerGameData (432 bytes) — spec/04

### HP / FP / SP Block (0x00–0x33)

| Offset | Field | Status | Method | Notes |
|---|---|---|---|---|
| 0x00 | unk0x0 | ❓ | Probe | PlayerNo? Internal ID? |
| 0x04 | unk0x4 | ❓ | Probe | — |
| 0x08 | HP | ❌ | Known Value | Compare with CT value |
| 0x0C | MaxHP | ❌ | Known Value | — |
| 0x10 | BaseMaxHP | ❌ | Known Value | Calculate from Vigor table |
| 0x14 | FP | ❌ | Known Value | — |
| 0x18 | MaxFP | ❌ | Known Value | — |
| 0x1C | BaseMaxFP | ❌ | Known Value | Calculate from Mind table |
| 0x20 | unk0x20 | ❓ | Probe | FP regen? MaxFP2? BaseMaxMP? |
| 0x24 | SP | ❌ | Known Value | — |
| 0x28 | MaxSP | ❌ | Known Value | — |
| 0x2C | BaseMaxSP | ❌ | Known Value | Calculate from Endurance table |
| 0x30 | unk0x30 | ❓ | Probe | SP regen? |

### Attributes (0x34–0x5F)

| Offset | Field | Status | Method | Notes |
|---|---|---|---|---|
| 0x34 | Vigor | ❌ | Known Value | Known value from creator/level up |
| 0x38 | Mind | ❌ | Known Value | — |
| 0x3C | Endurance | ❌ | Known Value | — |
| 0x40 | Strength | ❌ | Known Value | — |
| 0x44 | Dexterity | ❌ | Known Value | — |
| 0x48 | Intelligence | ❌ | Known Value | — |
| 0x4C | Faith | ❌ | Known Value | — |
| 0x50 | Arcane | ❌ | Known Value | — |
| 0x54 | unk0x54 | ❓ | Cross-Slot | Padding? DLC attr? |
| 0x58 | unk0x58 | ❓ | Cross-Slot | — |
| 0x5C | unk0x5c | ❓ | Cross-Slot | — |

### Level & Runes (0x60–0x6F)

| Offset | Field | Status | Method | Notes |
|---|---|---|---|---|
| 0x60 | Level | ❌ | Known Value | Formula: sum(attrs) - 79 |
| 0x64 | Runes | ❌ | Known Value | Currently held |
| 0x68 | TotalGetSoul | ❌ | Known Value | Lifetime — compare high/low level chars |
| 0x6C | unk0x6c | ❓ | Diff | Runes lost? Memory runes? |

### Status Buildups (0x70–0x93)

| Offset | Field | Status | Method | Notes |
|---|---|---|---|---|
| 0x70 | Immunity (Poison) | ❌ | Probe | Should be 0 in clean save |
| 0x74 | Immunity2 (Scarlet Rot) | ❌ | Probe | — |
| 0x78 | Robustness (Bleed) | ❌ | Probe | — |
| 0x7C | Vitality (Death) | ❌ | Probe | — |
| 0x80 | Robustness2 (Frost) | ❌ | Probe | — |
| 0x84 | Focus (Sleep) | ❌ | Probe | — |
| 0x88 | Focus2 (Madness) | ❌ | Probe | — |
| 0x8C | unk0x8c | ❓ | Probe | DLC buildup? |
| 0x90 | unk0x90 | ❓ | Probe | — |

### Character Name (0x94–0xB5)

| Offset | Field | Status | Method | Notes |
|---|---|---|---|---|
| 0x94 | CharacterName[16] | ❌ | Known Value | UTF-16LE — easy anchor |
| 0xB4 | NullTerminator | ❌ | Pattern | Should be 0x0000 |

### Creation Data (0xB6–0xBF)

| Offset | Field | Status | Method | Notes |
|---|---|---|---|---|
| 0xB6 | Gender | ❌ | Known Value | 0=TypeB, 1=TypeA |
| 0xB7 | ArcheType (Class) | ❌ | Known Value | 0-9 |
| 0xB8 | unk0xb8 | ❓ | Cross-Slot | Appearance? VowType? |
| 0xB9 | unk0xb9 | ❓ | Cross-Slot | — |
| 0xBA | VoiceType | ❌ | Probe | 0-5 |
| 0xBB | Gift | ❌ | Known Value | 0-9 (starting keepsake) |
| 0xBC | unk0xbc | ❓ | Probe | — |
| 0xBD | unk0xbd | ❓ | Probe | — |
| 0xBE | TalismanSlotCount | ❌ | Probe | 0-3 |
| 0xBF | SummonSpiritLevel | ❓ | Probe | What does this do exactly? |

### Unknown Block (0xC0–0xD7)

| Offset | Field | Status | Method | Notes |
|---|---|---|---|---|
| 0xC0–0xD7 | 24 bytes unknown | ❓ | Diff + Probe | Compare fresh char vs endgame char |

### Online Settings (0xD8–0xF8)

| Offset | Field | Status | Method | Notes |
|---|---|---|---|---|
| 0xD8 | FurlcallingFinger | ❌ | Probe | 0/1 |
| 0xD9 | unk0xd9 | ❓ | Probe | — |
| 0xDA | MatchmakingWepLvl | ❌ | Known Value | 0-25 |
| 0xDB | WhiteCipherRing | ❌ | Probe | 0/1 |
| 0xDC | BlueCipherRing | ❌ | Probe | 0/1 |
| 0xDD–0xEE | 18 bytes unknown | ❓ | Diff | Online flags? |
| 0xEF | ReinforceLv | ❓ | Probe | Character reinforce? |
| 0xF0–0xF6 | 7 bytes unknown | ❓ | Diff | — |
| 0xF7 | GreatRuneActive | ❌ | Probe | 0/1 |
| 0xF8 | unk0xf8 | ❓ | Probe | — |

### Flask Counts (0xF9–0x10F)

| Offset | Field | Status | Method | Notes |
|---|---|---|---|---|
| 0xF9 | MaxCrimsonFlask | ❌ | Known Value | 0-14 |
| 0xFA | MaxCeruleanFlask | ❌ | Known Value | 0-14 |
| 0xFB | unk | ❓ | Diff | Flask upgrade level? |
| 0xFC–0x10F | 20 bytes unknown | ❓ | Diff | Physick tears? Flask state? |

### Passwords (0x110–0x17B)

| Offset | Field | Status | Method | Notes |
|---|---|---|---|---|
| 0x110 | MultiplayerPassword | ❌ | Known Value | UTF-16LE |
| 0x122 | GroupPassword1 | ❌ | Known Value | — |
| 0x134–0x16A | GroupPasswords 2-5 | ❌ | Known Value | — |

### Trailing Block (0x17C–0x1AF)

| Offset | Field | Status | Method | Notes |
|---|---|---|---|---|
| 0x17C–0x17F | SwordArtPoint? | ❓ | CT Cross-ref | 4 × u8 scaling? |
| 0x180–0x1AF | 48 bytes | ❓ | Diff + Probe | Correction stats? Extended data? |

---

## B. Equipment Structures — spec/06

| Structure | Status | Method | Notes |
|---|---|---|---|
| EquippedItemsEquipIndex (88B) | ❌ | Known Value | Compare with inventory indices |
| ActiveWeaponSlots (28B) | ❌ | Probe | ArmStyle 0-3, slots 0-2 |
| EquippedItemsItemIds (88B) | ❌ | Known Value | Item IDs from database |
| EquippedItemsGaitemHandles (88B) | ❌ | Known Value | Handles from GaItem Map |
| Slot #10 (Arrows 3) | ❓ | Probe | CT says it exists |
| Slot #11 (Bolts 3) | ❓ | Probe | CT says it exists |
| Slot #16 (Hair) | ❓ | Probe | Internal slot |
| Slot #21 (Accessory 5) | ❓ | Probe | Unused — is it zero? |
| Great Rune field | ❓ | Known Value | Where exactly in save? |
| Quick Items (10 × u32) | ❌ | Known Value | — |
| Pouch (6 × u32) | ❌ | Known Value | — |

---

## C. Spells & Gestures — spec/08

| Element | Status | Method | Notes |
|---|---|---|---|
| 14 spell slots (stride 8B) | ❌ | Known Value | Known character spells |
| SelectedSlotIdx | ❓ | Probe | -1 or 0-13 |
| Quick Slots 10 × u32 | ❌ | Known Value | — |
| Pouch 6 × u32 | ❌ | Known Value | — |
| Equipped Gestures (ring) | ❌ | Known Value | ID from table |
| Acquired Projectiles count | ❌ | Known Value | Compare characters |
| Gestures 64 × u32 | ❌ | Cross-Slot | — |
| Equipped Physics (2 × u32) | ❌ | Known Value | Crystal Tear IDs |

---

## D. Face Data (303 bytes) — spec/09

| Element | Status | Method | Notes |
|---|---|---|---|
| Face_Model_Id (u32) | ❌ | Cross-Slot | Compare chars with different faces |
| Hair_Model_Id (u32) | ❌ | Known Value | Known hairstyle |
| Beard_Model_Id (u32) | ❌ | Cross-Slot | — |
| Face shape params (~50 u8) | ❓ | Diff | Compare identical vs different faces |
| Skin colors (RGBA) | ❓ | Diff | — |
| Body Scale (7 values) | ❓ | Probe | float vs u8? |
| Trailing 15 bytes (slot-only) | ❓ | Probe | What is this? |

---

## E. Inventory & Storage — spec/07, spec/10

| Element | Status | Method | Notes |
|---|---|---|---|
| Common item count (2688 max) | ❌ | Known Value | — |
| Key item count (384 max) | ❌ | Known Value | — |
| Item record (12B: handle+qty+idx) | ❌ | Known Value | — |
| NextEquipIndex | ❌ | Monotonic check | Should be increasing |
| NextAcquisitionSortId | ❌ | Monotonic check | — |
| Storage common (1920 max) | ❌ | Known Value | — |
| Storage key (128 max) | ❌ | Known Value | — |
| Empty slot pattern | ❌ | Pattern | 0x00000000 or 0xFFFFFFFF |

---

## F. GaItem Map — spec/03

| Element | Status | Method | Notes |
|---|---|---|---|
| Entry count (5118 vs 5120) | ❌ | Version check | Depends on slot.Version |
| Weapon record (21B) | ❌ | Known Value | Handle + ItemID + extras |
| Armor record (16B) | ❌ | Known Value | — |
| Other record (8B) | ❌ | Known Value | — |
| Type segregation (AoW first) | ❌ | Scan | Check if 0xC0... before 0x80... |
| Empty entry pattern | ❌ | Pattern | 0x00 or 0xFFFFFFFF |

---

## G. Event Flags — spec/15

| Element | Status | Method | Notes |
|---|---|---|---|
| BST algorithm correctness | ❌ | Spot-check | 4-5 known flags |
| Size = 0x1BF99F | ❌ | Measure | — |
| Terminator (4B after) | ❌ | Pattern | — |
| Grace flag 76101 (First Step) | ❌ | BST + hex | — |
| Boss flag (Margit killed?) | ❌ | BST + hex | — |
| Mechanic flag 60100 (Torrent) | ❌ | BST + hex | — |
| Map flag 62010 (Limgrave W) | ❌ | BST + hex | — |

---

## H. Game State — spec/14

| Element | Status | Method | Notes |
|---|---|---|---|
| ClearCount (NG+) | ❌ | Known Value | Check pre/post NG+ characters |
| Death Count | ❌ | Known Value | Known value from menu? |
| Last Rested Grace | ❌ | Known Value | BonfireId — verify with grace list |
| Play Time | ❌ | Known Value | Compare with menu display |
| GaItem Game Data count | ❌ | Known Value | — |
| Tutorial Data count | ❌ | Cross-Slot | Fresh vs endgame |

---

## I. World Data — spec/12, 13, 16, 17, 18, 19

| Element | Status | Method | Notes |
|---|---|---|---|
| Torrent State | ❌ | Probe | 1/3/13 |
| Torrent HP | ❌ | Known Value | — |
| Blood Stain runes | ❌ | Known Value | How many runes were lost |
| Player Coords (x,y,z) | ❌ | Known Value | Compare with CT readout |
| Map ID | ❌ | Known Value | — |
| Weather type | ❓ | Diff | Compare different times of day |
| Time (H/M/S) | ❌ | Known Value | — |
| Regions count | ❌ | Known Value | How many regions discovered |
| FieldArea size | ❌ | Measure | — |
| WorldArea size | ❌ | Measure | — |
| NetMan size (0x20004) | ❌ | Measure | — |

---

## J. Platform & Meta — spec/01, 20, 21, 22, 23

| Element | Status | Method | Notes |
|---|---|---|---|
| BND4 header (PC) | ❌ | Pattern | Magic bytes "BND4" |
| MD5 checksum (PC) | ❌ | Recalculate | Compute and compare |
| PS4 header | ❌ | Pattern | Compare with known magic |
| Active Slots offset PC | ❌ | **CRITICAL** | Spec: 0x1C vs code: 0x310 |
| Active Slots offset PS4 | ❌ | Known Value | 0x300 |
| ProfileSummary offset PC | ❌ | **CRITICAL** | Spec: 0x26 vs code: 0x31A |
| ProfileSummary offset PS4 | ❌ | Known Value | 0x30A |
| SteamID in UserData10 | ❌ | Known Value | 8 bytes, known Steam ID |
| DLC flags (50B) | ❌ | Pattern | Bytes 3-49 = zero? |
| DLC entry flag | ❌ | Probe | 0/1 |
| BaseVersion | ❌ | Known Value | — |
| PlayerGameData Hash | ❌ | Measure | Remainder to end of slot |

---

## K. Dynamic Offset Chain — CRITICAL

Verification of sequentially calculated offset chain:

| Step | From → To | Status | Method |
|---|---|---|---|
| 1 | Slot start → GaItem Map | ❌ | Version + header (32B) |
| 2 | GaItem Map → PlayerGameData | ❌ | Scan all entries, sum sizes |
| 3 | PlayerGameData → SP Effects | ❌ | +432B |
| 4 | SP Effects → EquipIndex | ❌ | Variable (13 entries?) |
| 5 | EquipIndex → ActiveWeapons | ❌ | +88B |
| 6 | ActiveWeapons → ItemIds | ❌ | +28B |
| 7 | ItemIds → GaitemHandles | ❌ | +88B |
| 8 | GaitemHandles → Inventory | ❌ | +88B |
| 9 | Inventory → Spells | ❌ | Count-based (common+key × 12B + 8B counters) |
| 10 | Spells → Projectiles | ❌ | Fixed? |
| 11 | Projectiles → Face Data | ❌ | VARIABLE (count × 8B) |
| 12 | Face Data → Storage | ❌ | +303B |
| 13 | Storage → Gestures | ❌ | Count-based |
| 14 | Gestures → Regions | ❌ | +256B |
| 15 | Regions → Torrent | ❌ | VARIABLE (4 + count×4) |
| 16 | Torrent → Blood Stain | ❌ | +40B + 1B (control byte) |
| 17 | Blood Stain → Game State | ❌ | +68B |
| 18 | Game State → Event Flags | ❌ | VARIABLE (multiple sub-sections) |
| 19 | Event Flags → World State | ❌ | +0x1BF99F + 4B |
| 20 | World State → Coords | ❌ | VARIABLE (5 size-prefixed sections) |
| 21 | Coords → NetMan | ❌ | +57B + spawn bytes |
| 22 | NetMan → Weather | ❌ | +0x20004 |
| 23 | Weather → Time | ❌ | +12B |
| 24 | Time → Version | ❌ | +12B |
| 25 | Version → SteamID | ❌ | +16B |
| 26 | SteamID → PS5Activity | ❌ | +8B |
| 27 | PS5Activity → DLC | ❌ | +32B |
| 28 | DLC → Hash | ❌ | +50B |
| 29 | Hash → Slot End | ❌ | Remainder to 0x280000 |

---

## L. DISCOVERIES — Unknown parameters to investigate

### HIGH priority (likely editable)

| ID | Location | Hypothesis | Investigation plan |
|---|---|---|---|
| L1 | PGD 0x00–0x07 | Runtime-only header? | Check if different between slots |
| L2 | PGD 0x20 | FP-related (between MaxFP and SP) | Compare with Mind value; probe |
| L3 | PGD 0x30 | SP-related | Compare with Endurance value |
| L4 | PGD 0x54–0x5C | Extended attrs? DLC? | Check pre-DLC vs post-DLC save |
| L5 | PGD 0x6C | Runes on bloodstain? | Kill char, compare before/after |
| L6 | PGD 0x8C–0x90 | DLC buildups? | DLC save vs base save |
| L7 | PGD 0xFB–0x10F | Flask upgrade level + Physick | Compare fresh vs 12 Sacred Tears |
| L8 | PGD 0xC0–0xD7 | Character state flags? | Diff between online/offline save |

### MEDIUM priority (likely informational)

| ID | Location | Hypothesis | Investigation plan |
|---|---|---|---|
| L9 | PGD 0xB8–0xB9 | Appearance/VowType? | Cross-ref with CT |
| L10 | PGD 0xBC–0xBD | Starting equip related? | Compare classes |
| L11 | PGD 0xBF | SummonSpiritLevel | What does it do? DLC? |
| L12 | PGD 0xDD–0xF6 | Extended online settings | Diff online vs offline |
| L13 | PGD 0xEF | ReinforceLv | Character reinforce — what is this? |
| L14 | PGD 0x180–0x1AF | Trailing 48B | SwordArt? Correction? Overflow? |

### LOW priority (likely constant/unused)

| ID | Location | Hypothesis | Investigation plan |
|---|---|---|---|
| L15 | Equipment slot #21 | Accessory 5 — unused? | Check if always 0xFFFFFFFF |
| L16 | Face Data trailing 15B | Slot-only extra params? | Diff vs ProfileSummary version |
| L17 | Body Scale format | float vs u8 in save? | Hex dump known proportions |
| L18 | Correction Stats location | In PGD or after PGD? | Search for attribute copies |

---

## Verification session procedure

### Before session:
1. `cp tmp/save/ER0000.sl2 tmp/save/ER0000.sl2.bak` — backup
2. Prepare script to extract slots from save file
3. Prepare `xxd` / `hexdump` commands with correct offsets

### During session:
1. Choose section to verify (e.g. "A. PlayerGameData")
2. Execute appropriate method (Known Value / Diff / Probe)
3. Record results in "Status" and "Notes" columns
4. On discovery — add to section L with full description

### After session:
1. Update this file with statuses
2. Update corresponding spec/ files with confirmed information
3. Add new discoveries to spec/26-parameter-reference.md

---

## Helper tools (to write)

| Tool | Purpose | Status |
|---|---|---|
| `scripts/dump_slot.py` | Extracts raw slot from .sl2 | ❌ To write |
| `scripts/find_pattern.py` | Searches for pattern in hex dump | ❌ To write |
| `scripts/verify_bst.py` | Tests BST algorithm on known flags | ❌ To write |
| `scripts/diff_slots.py` | Compares two slots byte-by-byte | ❌ To write |
| `scripts/parse_pgd.py` | Parses PlayerGameData and prints fields | ❌ To write |
| `tests/offset_chain_test.go` | Verifies the entire offset chain | ❌ To write |

---

## Sources

- Hex editor: `xxd`, `hexdump`, or GUI (HxD, ImHex)
- Python: struct module for binary parsing
- Go test framework: `go test -v -run TestXxx`
- er-save-manager: reference parser for comparisons
- Cheat Engine tables: runtime values for cross-reference
