# 15 — Event Flags

> **Type**: Binary format spec  
> **Scope**: Main game progression mechanism — 1.8 MB bitfield controlling world state, quests, bosses, discoveries.

---

## Overview

Event Flags is a 0x1BF99F byte array (1,833,375 bytes / ~1.75 MB) of bit flags. Each flag is a single bit controlling one aspect of game state:
- Boss defeated
- NPC quest stage
- Item picked up
- Map discovered
- Cutscene watched
- Mechanic unlocked
- and thousands more

After the flag array comes a 4-byte **terminator**.

---

## Flag addressing — BST Lookup

Flags are NOT linearly mapped (flag_id ≠ byte_offset × 8 + bit). Instead, a **Binary Search Tree** (BST) is used for conversion:

### Algorithm: Event ID → (byte_offset, bit_index)

```
BLOCK_SIZE = 125 bytes
FLAG_DIVISOR = 1000

1. block = event_id / 1000  (integer division)
2. index = event_id - (block × 1000)
3. offset = BST_LOOKUP[block] × 125     ← z eventflag_bst.txt
4. byte_index = index / 8
5. bit_index = 7 - (index - byte_index × 8)    ← BIG-ENDIAN bit order!
6. final_byte_pos = offset + byte_index
7. flag_value = (event_flags[final_byte_pos] >> bit_index) & 1
```

### BST Lookup Table

File `eventflag_bst.txt` contains 11,919 entries in format `block,offset`:
- `block` = event_id // 1000
- `offset` = 125-byte block number in the event_flags array

---

## Setting a flag

```
byte_pos = BST_LOOKUP[block] × 125 + (index / 8)
bit_pos = 7 - (index % 8)

SET:   event_flags[byte_pos] |= (1 << bit_pos)
CLEAR: event_flags[byte_pos] &= ~(1 << bit_pos)
```

---

## Flag categories — detailed IDs

### Global progression (0–999)

| ID | Description |
|---|---|
| 20–24 | Game Clear (Normal, Ranni, Frenzied Flame endings) |
| 30 | Transition to next NG+ cycle |
| 50–58 | Completed NG+ cycle tracking |
| 180–197 | Great Rune possession tracking |
| 300–575 | World state (Erdtree, meteorites, gravity) |

### Mechanics — Core Unlocks (60000–60999)

| ID | Mechanic | How to unlock in-game |
|---|---|---|
| 60020 | Flask of Wondrous Physick | Third Church of Marika |
| 60100 | Torrent Whistle (Spectral Steed Ring) | Meeting Melina at Grace |
| 60110 | Spirit Calling Bell | Ranni at Church of Elleh (night) |
| 60120 | Crafting Kit | Purchase from Kale (Merchant) |
| 60130 | Whetstone Knife | Loot in Gatefront Ruins |
| 60140 | Tailoring Tools | Loot in Coastal Cave |
| 60150 | Golden Tailoring Tools | Boss drop (Godfrey's shade?) |

### Mechanics — Whetblades (affinity unlock)

| ID | Whetblade | Affinity unlocked |
|---|---|---|
| 65610 | Iron Whetblade | Heavy, Keen, Quality |
| 65640 | Glintstone Whetblade | Magic, Cold |
| 65660 | Red-Hot Whetblade | Fire, Flame Art |
| 65680 | Sanctified Whetblade | Lightning, Sacred |
| 65720 | Black Whetblade | Poison, Blood, Occult |

### Mechanics — remaining (60200–60849)

| Range | Description |
|---|---|
| 60200–60300 | Multiplayer features (signs, invasions, items) |
| 60400–60590 | Memory slots and talisman slot unlocks |
| 60800–60849 | Gesture unlocks |

### Bosses (61000–61999)

| ID | Description |
|---|---|
| 61100–61135 | Major bosses (Margit, Godrick, Maliketh, Malenia, etc.) |
| 61200–61220 | Catacomb bosses |
| 61230–61248 | Cave bosses |
| 61260–61268 | Mine bosses |

### Map — Visibility (62000–62065)

Map visibility flags — control the visibility of region texture on the map.

| ID | Region | Area |
|---|---|---|
| 62010 | Limgrave West | Western Limgrave |
| 62011 | Limgrave East | Eastern Limgrave |
| 62012 | Weeping Peninsula | Weeping Peninsula |
| 62020 | Liurnia South | Southern Liurnia |
| 62021 | Liurnia North | Northern Liurnia |
| 62022 | Liurnia East | Eastern Liurnia |
| 62030 | Altus Plateau | Altus Plateau |
| 62031 | Leyndell | Leyndell Royal Capital |
| 62032 | Mt. Gelmir | Mt. Gelmir |
| 62040 | Caelid South | Southern Caelid |
| 62041 | Caelid North (Dragonbarrow) | Northern Caelid / Dragonbarrow |
| 62050 | Mountaintops West | Western Mountaintops |
| 62051 | Mountaintops East | Eastern Mountaintops |
| 62052 | Consecrated Snowfield | Consecrated Snowfield |
| 62060 | Siofra River | Siofra River (underground) |
| 62061 | Ainsel River | Ainsel River (underground) |
| 62062 | Deeproot Depths | Deeproot Depths |
| 62063 | Lake of Rot | Lake of Rot |
| 62064 | Mohgwyn Palace | Mohgwyn Palace |
| 82001 | Shadow of the Erdtree (DLC) | Realm of Shadow |

### Map — Fragment Acquisition (63000–63065)

Map fragment pickup flags — whether the player picked up the map fragment.

| ID | Fragment | Location |
|---|---|---|
| 63010 | Limgrave West Map | Gatefront |
| 63011 | Limgrave East Map | Waypoint Ruins area |
| 63012 | Weeping Peninsula Map | Castle Morne approach |
| 63020 | Liurnia South Map | Lake-Facing Cliffs |
| 63021 | Liurnia North Map | Academy Gate area |
| 63022 | Liurnia East Map | Eastern Liurnia |
| 63030 | Altus Plateau Map | Forest Spanning Greatbridge |
| 63031 | Leyndell Map | Capital Outskirts |
| 63032 | Mt. Gelmir Map | Road of Iniquity |
| 63040 | Caelid Map | Caelid Highway |
| 63041 | Dragonbarrow Map | Dragonbarrow |
| 63050 | Mountaintops West Map | Giants area |
| 63051 | Mountaintops East Map | Fire Giant area |
| 63052 | Consecrated Snowfield Map | Hidden Path |
| 63060 | Siofra River Map | Underground |
| 63061 | Ainsel River Map | Underground |
| 63062 | Deeproot Depths Map | Underground |
| 63063 | Lake of Rot Map | Underground |
| 63064 | Mohgwyn Palace Map | Underground |

### Cookbooks (67000–68500)

| Range | Cookbook Type | Count |
|---|---|---|
| 67000–67910 | Nomadic Warrior's Cookbook | ~10 entries |
| 67200–67300 | Armorer's Cookbook | ~7 entries |
| 67400–67480 | Glintstone Craftsman's Cookbook | ~5 entries |
| 67600–67700 | Missionary's Cookbook | ~5 entries |
| 67840–67920 | Perfumer's Cookbook | ~4 entries |
| 68000–68030 | Ancient Dragon Apostle's Cookbook | ~3 entries |
| 68200–68230 | Fevor's Cookbook | ~3 entries |
| 68400–68410 | Frenzied's Cookbook | ~2 entries |

### Items (65000–68999)

| Range | Description |
|---|---|
| 65600–65790 | Ash of War affinity unlocks |
| 65810–65901 | Skill stone possession |
| 67000–68500 | Cookbook/recipe unlocks (powyżej szczegóły) |

### Graces — Event Flag IDs

| Flag ID | Grace | Location |
|---|---|---|
| 71000 | Godrick the Grafted | Stormveil Castle (post-boss) |
| 71001 | Margit, the Fell Omen | Stormveil approach |
| 71190 | Table of Lost Grace | Roundtable Hold |
| 71800 | Cave of Knowledge | Tutorial area |
| 73xxx | Catacomb/Cave/Tunnel graces | Dungeons |
| 76100 | Church of Elleh | Limgrave |
| 76101 | The First Step | Limgrave (start) |
| 76111 | Gatefront | Limgrave |

### Graces — Byte Offsets in bitfield (confirmed from CT)

Offset from EventFlags base (`[EventFlagMan]+0x28` in memory):

| Grace | Byte Offset | Bit | Flag ID |
|---|---|---|---|
| Table of Lost Grace / Roundtable Hold | +0xA58 | 1 | 71190 |
| The First Step | +0xCBE | 2 | 76101 |
| Church of Elleh | +0xCBE | 3 | 76100 |
| Gatefront | +0xCBF | 0 | 76111 |
| Stormhill Shack | +0xCBE | 1 | — |
| Castleward Tunnel | +0xA41 | 5 | — |
| Margit, the Fell Omen | +0xA41 | 6 | 71001 |
| Warmaster's Shack | +0xCC0 | 1 | — |
| Cave of Knowledge | +0xAA5 | 7 | 71800 |
| Stranded Graveyard | +0xAA5 | 6 | — |

### Location maps (30000–60999)

| Prefix | Description |
|---|---|
| 30xxx | Catacomb zone flags |
| 31xxx | Cave system flags |
| 32xxx | Mine network flags |
| 60xxx | Fortress/camp flags |

### Tutorial/Debug (710000+)

| ID | Description |
|---|---|
| 710000–720200 | Tutorial completion tracking |
| 780000–780090 | Cinematic context flags |
| 9990–9999 | Developer test flags |

---

## Bonfire IDs (Grace Entity IDs — for teleportation)

Bonfire IDs are **separate identifiers** from Event Flag IDs. Used in GameMan for teleportation (Last Grace, Target Grace):

| BonfireId | Grace | Format |
|---|---|---|
| 1042362951 | The First Step | 10AABBCCCC |
| 10002951 | Margit, the Fell Omen | |
| 11052950–55 | Leyndell/Capital | |
| 12012950–71 | Underground (Ainsel, Siofra, Nokron) | |
| 13002950–60 | Crumbling Farum Azula | |
| 14002950–53 | Academy of Raya Lucaria | |
| 15002950–58 | Haligtree | |
| 16002950–64 | Volcano Manor | |
| 18002950–51 | Tutorial area | |
| 19002950 | Fractured Marika | |
| 20xxxxxxx | DLC (Shadow of the Erdtree) | |

---

## Known issues / soft-locks fixable by flag editing

1. **Ranni's Tower quest soft-lock** — fixable by resetting specific quest flags
2. **Warp sickness** (Radahn, Morgott, Radagon, Sealing Tree) — fixable by flag editing
3. **Incompatible flag combinations** — e.g. boss killed + quest stage = before boss → NPC confused

---

## Editing implications

- Changing flags does NOT change section size (fixed 0x1BF99F)
- Does not require shifting other sections
- Requires BST lookup — cannot address flags without `eventflag_bst.txt`
- Setting a boss flag without setting related quest flags may cause soft-locks
- **Map visibility** (62xxx) — setting = map visible even without fragment
- **Map acquired** (63xxx) — setting = fragment "picked up"; should also add item to inventory
- **Cookbook flags** — setting = crafting recipes unlocked
- **Whetblade flags** — setting = new affinity available in crafting
- Full flag list: https://soulsmods.github.io/elden-ring-eventparam/
- Spreadsheet: https://docs.google.com/spreadsheets/d/1Nn-d4_mzEtGUSQXscCkQ41AhtqO_wF2Aw3yoTBdW9lk

---

## Sources

- er-save-manager: `parser/event_flags.py` — klasa `EventFlags` (pełny algorytm BST)
- er-save-manager: `src/resources/eventflag_bst.txt` — 11,919 entries BST mapping
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` linie 197-223 — EventFlags (0x1bf99f bytes)
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — Grace byte offsets, mechanic flags, map discovery
- Cheat Engine: `ER_TGA_v1.9.0` — Event flag categories, flag IDs, grace/boss/NPC references
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=er-refmat:event-flag-list
- Event Flags GitHub Pages: https://soulsmods.github.io/elden-ring-eventparam/
- TGA CE Table: https://github.com/The-Grand-Archives/Elden-Ring-CT-TGA
