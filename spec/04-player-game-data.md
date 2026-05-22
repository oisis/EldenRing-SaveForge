# 04 — PlayerGameData (Character Stats)

> **Scope**: Main character data structure — 432 bytes (0x1B0). Contains all attributes, level, runes, name, class, online settings.

---

## Overview

PlayerGameData is a fixed 432-byte structure containing the key character information. It is parsed immediately after the GaItem Map.

---

## Full field map (0x1B0 = 432 bytes)

### HP / FP / SP (0x00–0x33)

| Offset | Type | Field | Description |
|---|---|---|---|
| 0x00 | u32 | unk0x0 | Unknown (likely PlayerNo / internal ID) |
| 0x04 | u32 | unk0x4 | Unknown |
| 0x08 | u32 | HP | Current health |
| 0x0C | u32 | MaxHP | Maximum HP (with buffs/talismans) |
| 0x10 | u32 | BaseMaxHP | Base max HP (pure, from attributes alone) |
| 0x14 | u32 | FP | Current Focus Points (mana) |
| 0x18 | u32 | MaxFP | Maximum FP (with buffs) |
| 0x1C | u32 | BaseMaxFP | Base max FP |
| 0x20 | u32 | unk0x20 | Unknown (possibly related to FP regen?) |
| 0x24 | u32 | SP | Current stamina |
| 0x28 | u32 | MaxSP | Maximum SP (with buffs) |
| 0x2C | u32 | BaseMaxSP | Base max SP |
| 0x30 | u32 | unk0x30 | Unknown |

**HP/FP/SP field description**:
- `HP/FP/SP` — current value at the moment of saving (recovers after rest)
- `MaxXX` — current cap including talismans, Great Rune, etc.
- `BaseMaxXX` — cap computed exclusively from attributes (Vigor→HP, Mind→FP, Endurance→SP)

---

### Attributes (0x34–0x5F)

| Offset | Type | Field | Description | Range |
|---|---|---|---|---|
| 0x34 | u32 | Vigor | Vitality — scales HP | 1–99 |
| 0x38 | u32 | Mind | Mind — scales FP and attunement slots | 1–99 |
| 0x3C | u32 | Endurance | Endurance — scales SP and equip load | 1–99 |
| 0x40 | u32 | Strength | Strength — STR weapon scaling | 1–99 |
| 0x44 | u32 | Dexterity | Dexterity — DEX weapon scaling, cast speed | 1–99 |
| 0x48 | u32 | Intelligence | Intelligence — sorcery scaling, magic dmg | 1–99 |
| 0x4C | u32 | Faith | Faith — incantation scaling, holy dmg | 1–99 |
| 0x50 | u32 | Arcane | Arcane — discovery, buildup, dragon scaling | 1–99 |
| 0x54 | u32 | unk0x54 | Unknown (related to attributes? padding?) |  |
| 0x58 | u32 | unk0x58 | Unknown |  |
| 0x5C | u32 | unk0x5c | Unknown |  |

**Level formula**: `Level = Vigor + Mind + Endurance + Strength + Dexterity + Intelligence + Faith + Arcane - 79`
- Minimum: Level 1 (Wretch, all attributes = 10)
- Maximum: Level 713 (all attributes = 99)

---

### Level and Runes (0x60–0x6F)

| Offset | Type | Field | Description |
|---|---|---|---|
| 0x60 | u32 | Level | Current character level (1–713) |
| 0x64 | u32 | Runes | Runes currently held (lost on death) |
| 0x68 | u32 | TotalGetSoul | Total runes earned over the character's lifetime |
| 0x6C | u32 | unk0x6c | Unknown |

---

### Status Buildups / Resistances (0x70–0x93)

Status buildups are **accumulators** for status effects. When a buildup reaches the threshold, the effect activates. These values are normally 0 (no accumulated effect).

| Offset | Type | Field | In-game description |
|---|---|---|---|
| 0x70 | u32 | Immunity | Poison buildup — poison resistance |
| 0x74 | u32 | Immunity2 | Scarlet Rot buildup — scarlet rot resistance |
| 0x78 | u32 | Robustness | Hemorrhage (Bleed) buildup — bleed resistance |
| 0x7C | u32 | Vitality | Deathblight buildup — resistance to instant death |
| 0x80 | u32 | Robustness2 | Frostbite buildup — frostbite resistance |
| 0x84 | u32 | Focus | Sleep buildup — sleep resistance |
| 0x88 | u32 | Focus2 | Madness buildup — madness resistance |
| 0x8C | u32 | unk0x8c | Unknown (possibly: Blight DLC?) |
| 0x90 | u32 | unk0x90 | Unknown |

**Notes**:
- "Immunity" in game = resistance to Poison + Scarlet Rot
- "Robustness" = resistance to Hemorrhage + Frostbite
- "Focus" = resistance to Sleep + Madness
- "Vitality" = resistance to Deathblight
- Setting the value to 0 = no accumulation (safe)
- The game recomputes thresholds on load — overwriting accumulators is safe

---

### Character name (0x94–0xB5)

| Offset | Type | Field | Description |
|---|---|---|---|
| 0x94 | u16[16] | CharacterName | Character name (UTF-16LE, max 16 characters) |
| 0xB4 | u16 | NullTerminator | Terminator (0x0000) |

- Encoding: UTF-16 Little-Endian
- Maximum: 16 characters (32 bytes payload + 2B terminator = 34B)
- Unused characters: filled with 0x0000

---

### Character creation (0xB6–0xBF)

| Offset | Type | Field | Description | Values |
|---|---|---|---|---|
| 0xB6 | u8 | Gender | Gender/body type | 0=Type B (female), 1=Type A (male) |
| 0xB7 | u8 | ArcheType | Starting class | 0–9 (table below) |
| 0xB8 | u8 | unk0xb8 | Unknown (Appearance/VowType?) | |
| 0xB9 | u8 | unk0xb9 | Unknown | |
| 0xBA | u8 | VoiceType | Voice type | 0=Young 1, 1=Young 2, 2=Mature 1, 3=Mature 2, 4=Aged 1, 5=Aged 2 |
| 0xBB | u8 | Gift | Starting Keepsake | 0–9 (table below) |
| 0xBC | u8 | unk0xbc | Unknown | |
| 0xBD | u8 | unk0xbd | Unknown | |
| 0xBE | u8 | TalismanSlotCount | Extra talisman slots (unlocked via quest) | 0–2 |
| 0xBF | u8 | SummonSpiritLevel | Spirit summon level (Scadutree?) | |

---

### Unknown block (0xC0–0xD7)

| Offset | Type | Field | Description |
|---|---|---|---|
| 0xC0 | u8[24] | unk_block | Unknown block (0x18 bytes) — likely additional character state flags |

---

### Online settings (0xD8–0xF8)

| Offset | Type | Field | Description |
|---|---|---|---|
| 0xD8 | u8 | FurlcallingFingerRemedy | Furlcalling Finger Remedy active (0=off, 1=on) |
| 0xD9 | u8 | unk0xd9 | Unknown |
| 0xDA | u8 | MatchmakingWeaponLevel | Weapon level used for multiplayer matchmaking |
| 0xDB | u8 | WhiteCipherRing | White Cipher Ring active (0=off, 1=on) |
| 0xDC | u8 | BlueCipherRing | Blue Cipher Ring active (0=off, 1=on) |
| 0xDD | u8[18] | unk0xdd | Unknown (0x12 bytes) |
| 0xEF | u8 | ReinforceLv | Character reinforce level (internal parameter) |
| 0xF0 | u8[7] | unk0xf0 | Unknown |
| 0xF7 | u8 | GreatRuneActive | Great Rune active (0=off, 1=on — requires Rune Arc) |
| 0xF8 | u8 | unk0xf8 | Unknown |

**Matchmaking Weapon Level**:
- The game tracks the highest weapon level the character has ever possessed
- Affects multiplayer matchmaking (pairing with players of similar weapon level)
- Range: 0–25 (normal weapons) or 0–10 (special/somber weapons) → normalized to a single scale

---

### Flask counts (0xF9–0x10F)

| Offset | Type | Field | Description |
|---|---|---|---|
| 0xF9 | u8 | MaxCrimsonFlask | Max number of Crimson Tears (HP flask) — default 3-14 |
| 0xFA | u8 | MaxCeruleanFlask | Max number of Cerulean Tears (FP flask) — default 0-14 |
| 0xFB | u8[21] | unk0xfb | Unknown (0x15 bytes) — likely contains flask upgrade level, physick data |

**Flask notes**:
- Total pool: Crimson + Cerulean = 14 (max, after collecting all Golden Seeds)
- Flask reinforcement level: determines the amount of HP/FP restored (Sacred Tears)
- The split is variable — the player adjusts the ratio at a grace

---

### Passwords (0x110–0x17B)

Each password: UTF-16LE, max 8 characters + u16 terminator = 0x12 bytes (18 bytes)

| Offset | Type | Field | Description |
|---|---|---|---|
| 0x110 | u16[8]+u16 | MultiplayerPassword | Multiplayer password (limits matchmaking to a group) |
| 0x122 | u16[8]+u16 | GroupPassword1 | Group password 1 (eases visibility of group signs) |
| 0x134 | u16[8]+u16 | GroupPassword2 | Group password 2 |
| 0x146 | u16[8]+u16 | GroupPassword3 | Group password 3 |
| 0x158 | u16[8]+u16 | GroupPassword4 | Group password 4 |
| 0x16A | u16[8]+u16 | GroupPassword5 | Group password 5 |

---

### SwordArt Point Scaling (0x17C–0x17F) — from CT, approximate offset

| Offset (CT) | Type | Field | Description |
|---|---|---|---|
| ~0x17C | u8 | SwordArtPoint_ByStr | Ash of War scaling from Strength |
| ~0x17D | u8 | SwordArtPoint_ByDex | Scaling from Dexterity |
| ~0x17E | u8 | SwordArtPoint_ByInt | Scaling from Intelligence |
| ~0x17F | u8 | SwordArtPoint_ByFaith | Scaling from Faith |

---

### Correction Stats — attributes for respec (offset from CT: 0x288)

A copy of attributes stored for the respec mechanism (Rennala). Should match the current values.

| Field | Description |
|---|---|
| Vigor [For Correction] | Copy of Vigor for recomputation |
| Mind [For Correction] | Copy of Mind |
| Endurance [For Correction] | Copy of Endurance |
| Strength [For Correction] | Copy of Strength |
| Dexterity [For Correction] | Copy of Dexterity |
| Intelligence [For Correction] | Copy of Intelligence |
| Faith [For Correction] | Copy of Faith |
| Arcane [For Correction] | Copy of Arcane |

**Note**: These fields may not be within the 432B PlayerGameData — they may be further along in the slot. The 0x288 offset (from CT) suggests a separate structure after PlayerGameData (0x1B0 = 432 bytes ends before 0x288).

---

### Padding (0x180–0x1AF)

| Offset | Type | Description |
|---|---|---|
| 0x180 | u8[48] | Unknown trailing block (0x30 bytes) — to investigate |

---

## Character classes (Archetype) — full table with base stats

| ID | Class | Start Lvl | Vig | Mnd | End | Str | Dex | Int | Fai | Arc |
|---|---|---|---|---|---|---|---|---|---|---|
| 0 | Vagabond | 9 | 15 | 10 | 11 | 14 | 13 | 9 | 9 | 7 |
| 1 | Warrior | 8 | 11 | 12 | 11 | 10 | 16 | 10 | 8 | 9 |
| 2 | Hero | 7 | 14 | 9 | 12 | 16 | 9 | 7 | 8 | 11 |
| 3 | Bandit | 5 | 10 | 11 | 10 | 9 | 13 | 9 | 8 | 14 |
| 4 | Astrologer | 6 | 9 | 15 | 9 | 8 | 12 | 16 | 7 | 9 |
| 5 | Prophet | 7 | 10 | 14 | 8 | 11 | 10 | 7 | 16 | 10 |
| 6 | Confessor | 10 | 10 | 13 | 10 | 12 | 12 | 9 | 14 | 9 |
| 7 | Samurai | 9 | 12 | 11 | 13 | 12 | 15 | 9 | 8 | 8 |
| 8 | Prisoner | 9 | 11 | 12 | 11 | 11 | 14 | 14 | 6 | 9 |
| 9 | Wretch | 1 | 10 | 10 | 10 | 10 | 10 | 10 | 10 | 10 |

**Numbering note**: CT Hexinton lists Confessor=6, Samurai=7 — reversed compared to some online sources. Confirmed by both CTs.

---

## Starting Keepsake (Gift) — values

| ID | Gift | Description |
|---|---|---|
| 0 | None | No gift |
| 1 | Crimson Amber Medallion | +HP talisman |
| 2 | Lands Between Rune | Consumable — runes |
| 3 | Golden Seed | Flask upgrade |
| 4 | Fanged Imp Ashes | Spirit summon |
| 5 | Cracked Pot | Crafting container |
| 6 | Stonesword Key ×2 | Keys to imp statues |
| 7 | Bewitching Branch | NPC charm |
| 8 | Boiled Prawn | HP buff food |
| 9 | Shabriri's Woe | Aggro talisman |

---

## Great Rune Values (Item IDs from CT)

| Hex ID | Great Rune | Effect (with Rune Arc) |
|---|---|---|
| 0x00000000 | None | — |
| 0xB00000BF | Godrick's Great Rune | +5 to all attributes |
| 0xB00000C0 | Radahn's Great Rune | +HP/FP/SP |
| 0xB00000C1 | Morgott's Great Rune | +Max HP |
| 0xB00000C2 | Rykard's Great Rune | HP recovery on kill |
| 0xB00000C3 | Mohg's Great Rune | Phantom bleed effect |
| 0xB00000C4 | Malenia's Great Rune | HP recovery on attack |

---

## Editing implications

- **Changing attributes** also requires updating Level (formula: sum - 79)
- **Max HP/FP/SP** — the game recomputes after load based on attributes; safe to overwrite
- **Status buildups** — setting to 0 = clears accumulation (safe)
- **Matchmaking Weapon Level** — change affects multiplayer; cannot be lowered through normal gameplay
- **Gender** — 0↔1 swap changes the character model, but Face Data stays the same
- **Class** — change is only a label; does not change starting stats, but affects respec validation
- **Passwords** — change/zero out = immediate effect in multiplayer
- **Great Rune** — must match the one possessed (event flag) and being active requires GreatRuneActive=1
- **Correction Stats** — must stay in sync with the current attributes

---

## Sources

- er-save-manager: `parser/character.py` — class `PlayerGameData` (lines 22-123, full read 124-200)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — PlayerGameData struct referenced
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — PlayerParam offsets, class reset scripts
- Cheat Engine: `ER_TGA_v1.9.0` — PlayerGameData structure, ChrAsm, EquipItemData
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=er-refmat:param:speffectparam
