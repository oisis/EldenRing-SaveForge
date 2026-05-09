# Elden Ring Save File Format — Specification

> **Purpose**: Complete documentation of the Elden Ring binary save file format (.sl2 / memory.dat).
> Sufficient to implement a save editor from scratch without access to game source code.
>
> **State**: In progress. Each section is verified against real save files via hex analysis.

---

## Platforms

| Platform | File | Encryption | Checksums |
|---|---|---|---|
| PC (Steam) | `ER0000.sl2` | AES-128-CBC (optional) | MD5 per slot |
| PS4 | `memory.dat` | None | None |
| PS5 | `memory.dat` | None | None |

---

## File Structure — Overview

The save file consists of the following main blocks (in sequential order):

```
┌───────────────���─────────────────────────────┐
│  HEADER (platform-specific)                 │  → 01-header.md
├─────────────────────────────────────────────┤
│  SLOT 0: Character Save Data                │
│    ├── Slot Header                          │
│    ├── GaItem Map (item map)                │  → 03-gaitem-map.md
│    ├── PlayerGameData (stats)               │  → 04-player-game-data.md
│    ├── SP Effects (status effects)          │  → 05-sp-effects.md
│    ├── Equipment (equipped items)           │  → 06-equipment.md
│    ├── Inventory (held items)               │  → 07-inventory.md
│    ├── Spells & Gestures                    │  → 08-spells-gestures.md
│    ├── Face Data (character creator)        │  → 09-face-data.md
│    ├── Storage Box (grace chest)            │  → 10-storage.md
│    ├── Regions (unlocked regions)           │  → 11-regions.md
│    ├── Torrent (horse)                      │  → 12-torrent.md
│    ├── Blood Stain (death marker)           │  → 13-blood-stain.md
│    ├── Game State (game state data)         │  → 14-game-state.md
│    ├── Event Flags (event flags)            │  → 15-event-flags.md
│    ├── World State (world state)            │  → 16-world-state.md
│    ├── Player Coordinates (position)        │  → 17-player-coordinates.md
│    ├── Network Manager                      │  → 18-network.md
│    ├── Weather & Time                       │  → 19-weather-time.md
│    ├── Version & Platform Data              │  → 20-version-platform.md
│    ├── DLC                                  │  → 21-dlc.md
│    └── Player Data Hash                     │  → 22-player-hash.md
│  SLOT 1-9: (identical structure)            │
├─────────────────────────────────────────────┤
│  USER_DATA_10 (account profile)             │  → 23-user-data-10.md
├─────────────────────────────────────────────┤
│  USER_DATA_11 (regulation.bin)              │  → 24-user-data-11.md
└─────────────────────────────────────────────┘
```

---

## Documentation Sections

| # | File | Description |
|---|---|---|
| 01 | [Header & File Layout](01-header.md) | Magic bytes, platform detection, BND4 container, slot offsets, MD5 checksums |
| 02 | [Slot — General Structure](02-slot-structure.md) | Slot size, version, sequential parsing, variable-length sections |
| 03 | [GaItem Map](03-gaitem-map.md) | Handle→itemID map, item types, record sizes per type |
| 04 | [PlayerGameData](04-player-game-data.md) | Character stats: HP/FP/SP, attributes, level, runes, name, gender, class, passwords, online settings |
| 05 | [SP Effects](05-sp-effects.md) | Active status effects (buffs/debuffs), buildup values |
| 06 | [Equipment](06-equipment.md) | Equipment: 22 slots (weapons, armor, talismans), active weapon slots, arm style |
| 07 | [Inventory](07-inventory.md) | Character inventory: common items + key items, record format, indices |
| 08 | [Spells & Gestures](08-spells-gestures.md) | Attuned spells, gestures, projectiles |
| 09 | [Face Data](09-face-data.md) | Character creator — appearance parameters (303 bytes) |
| 10 | [Storage Box](10-storage.md) | Grace chest: common + key items, counters |
| 11 | [Regions](11-regions.md) | `unlocked_regions` list (fast travel) — binary format + `core.SetUnlockedRegions` |
| 12 | [Torrent](12-torrent.md) | Horse data: position, HP, state (alive/dead/inactive) |
| 13 | [Blood Stain](13-blood-stain.md) | Death blood stain: position, runes, map |
| 14 | [Game State](14-game-state.md) | Menu profile, tutorial data, GameMan bytes, death count, character type, last grace |
| 15 | [Event Flags](15-event-flags.md) | 1.8 MB bit flags: progression, bosses, quests, items, map — BST lookup |
| 16 | [World State](16-world-state.md) | FieldArea, WorldArea, WorldGeomMan, RendMan — geometry and world state |
| 17 | [Player Coordinates](17-player-coordinates.md) | Player 3D position + map + rotation angle |
| 18 | [Network Manager](18-network.md) | Multiplayer data (131 KB) |
| 19 | [Weather & Time](19-weather-time.md) | In-game weather and time |
| 20 | [Version & Platform](20-version-platform.md) | Save version, Steam ID, PS5 Activity |
| 21 | [DLC](21-dlc.md) | DLC flags: pre-order gestures, Shadow of the Erdtree entry |
| 22 | [Player Data Hash](22-player-hash.md) | Final player data hash |
| 23 | [UserData10](23-user-data-10.md) | Account profile: ProfileSummary ×10, SteamID, active slots |
| 24 | [UserData11](24-user-data-11.md) | regulation.bin — game parameters (params) |
| 25 | [Runtime vs Save](25-runtime-vs-save.md) | Memory↔file offset mapping, warnings |
| 26 | [Parameter Reference](26-parameter-reference.md) | **Complete reference** of all editable parameters |
| 27 | [Map Reveal](27-map-reveal.md) | 4-layer map discovery model: regions / event flags 62xxx + Map Fragments / DLC Cover Layer / FoW bitfield |
| 29 | [DLC Black Tiles](29-dlc-black-tiles.md) | Cover Layer SoE — discovery coordinates in BloodStain section (`afterRegs+0x0088..0x0110`) |
| 30 | [Slot Rebuild Research](30-slot-rebuild-research.md) | Slack analysis + transition from byte-shift to `RebuildSlot` (R-1 Step 13–14) |
| 31 | [Appearance Presets](31-appearance-presets.md) | Mirror Favorites preset slot layout (0x130 bytes), apply algorithm preset → FaceData, cross-gender M↔F handling — RE'd from real saves |
| 32 | [Ban-Risk System](32-ban-risk-system.md) | UI architecture: Tier 0/1/2 + Online Safety Mode + `RISK_INFO` dictionary + `Risk*` components (Phase 6) |
| 33 | [DB Categorization Audit](33-db-categorization-audit.md) | Information tab extraction + reclassification of Multiplayer/Remembrances/Crystal Tears/Materials per Fextralife per-item breadcrumb |
| 34 | [Item Caps Enforcement](34-item-caps.md) | Vanilla-realistic MaxInventory/MaxStorage + `scales_with_ng` flag (effective_cap = base × (ClearCount+1)) + Full Chaos Mode bypass toggle |
| 36 | [Inventory Categories — Game Order](36-inventory-categories-game-order.md) | Canonical 18-tab order + sub-grouping (Tools/Key Items/Melee/etc.) + reclassifications (Larval Tears, Torches, Region Maps, Golden Runes) — extends spec/33 |
| 37 | [Character Presets](37-character-presets.md) | **Design doc** — JSON export/import format, Go structs, phases 1-5 (stats + inventory + appearance + world flags) |
| 38 | [Boss Kill Multi-Flag](38-boss-multiflag.md) | **Design doc** — multi-flag approach for proper boss kill/respawn (arena state + quest + grace flags) |
| 39 | [Inventory Reorder](39-inventory-reorder.md) | **Design doc** — drag & drop grid reorder via `acquisition_index` manipulation, phases 0-4, DnD library |
| 40 | [DB Cleanup Plan](40-db-cleanup-plan.md) | **Design doc** — cut-content registry, multiplayer dedup, empty flask removal, phases A-G |
| 41 | [Info-Tab Ground Drop](41-info-tab-ground-drop.md) | **Investigation** — world pickup flags + TutorialDataChunk tried, EMEVD gating unknown |
| 42 | [Summoning Pools Bug](42-summoning-pools-bug.md) | **Investigation** — UI works, no in-game effect; diagnostic checklist + hypotheses |
| 43 | [Transactional Item Adding](43-transactional-item-adding.md) | **Design doc** (✅ implemented v0.7.2) — ALL-OR-NOTHING architecture, pre-flight + snapshot/rollback |
| 44 | [NetworkParam PvP Tuning](44-network-param-tuning.md) | **Design doc** (✅ partial) — full field reference for NETWORK_PARAM_ST: offsets, vanilla values, ban risk, presets |
| 45 | [Ban Risk Reference](45-ban-risk-reference.md) | Community-reported ban triggers, penalty tiers (180-day softban mechanics, save-embedded flag), safe-editing practices — informs spec/32 risk tiers |
| 46 | [Fast Invasions Research](46-faster-invasions-research.md) | **Investigation** (concluded) — full scan of UD10/NetMan/EventFlags/regulation.bin for invasion timing params; DS3 comparison; Wex Dust mod; verdict: not achievable via save file |
| 47 | [Site of Grace Activation](47-site-of-grace-activation.md) | **Investigation** (✅ resolved) — Hypothesis D confirmed; editor sets identical EventFlag to game; `LastRestedGrace` auto-set by game on arrival; grace object state is runtime-only; Model 3 (UI note) recommended |
| 99 | [Verification Methodology](99-verification-methodology.md) | Testing methods, verification checklist, discovery plan |

---

## Key Format Properties

- **Endianness**: Little-endian (all numeric values)
- **Strings**: UTF-16LE with null terminator
- **Slot size**: 0x280000 (2,621,440 bytes) — fixed
- **Variable-length sections**: inventory projectiles, regions, world areas — require sequential parsing
- **Checksums**: MD5 (PC only), recalculated on write
- **Encryption**: AES-128-CBC (PC only, optional — newer game versions)

---

## Knowledge Sources

### Reference Projects (local copies in `tmp/repos/`)

| Project | Language | Priority | Description |
|---|---|---|---|
| [er-save-manager](https://github.com/Jeius/er-save-manager) | Python | **1 (highest)** | Most recent, full sequential parser with DLC support |
| [ER-Save-Editor](https://github.com/ClayAmore/ER-Save-Editor) | Rust | **2** | Well-typed parser, confirms struct sizes |
| [Elden-Ring-Save-Editor](https://github.com/shalzuth/Elden-Ring-Save-Editor) | Python | **3 (lowest)** | Old, pattern-matching approach, but first offset discoveries |

### Online Documentation

| Source | URL | Contents |
|---|---|---|
| Souls Modding Wiki — SL2 Format | https://www.soulsmodding.com/doku.php?id=format:sl2 | Save container format |
| Souls Modding Wiki — Event Flags | https://www.soulsmodding.com/doku.php?id=er-refmat:event-flag-list | Event flag list |
| Event Flags GitHub Pages | https://soulsmods.github.io/elden-ring-eventparam/ | Full list of 1000+ flags with descriptions |
| Event Flags Spreadsheet | https://docs.google.com/spreadsheets/d/1Nn-d4_mzEtGUSQXscCkQ41AhtqO_wF2Aw3yoTBdW9lk | Detailed flags spreadsheet |
| Steam Guide — Save Structure | https://steamcommunity.com/sharedfiles/filedetails/?id=2797241037 | Slot offsets, MD5 checksums |
| SoulsFormats (C#) | https://github.com/JKAnderson/SoulsFormats | BND4 format parsing library |
| TGA Cheat Engine Table | https://github.com/The-Grand-Archives/Elden-Ring-CT-TGA | Event flags, param scripts, item IDs |
| Souls Modding Wiki — Params | https://www.soulsmodding.com/doku.php?id=er-refmat:param:speffectparam | SpEffect and other param tables |

### Data Files (local)

| File | Path | Description |
|---|---|---|
| eventflag_bst.txt | `tmp/repos/er-save-manager/src/resources/eventflag_bst.txt` | 11919 entries — block→offset mapping for event flags |
| PC Save | `tmp/save/ER0000.sl2` | Real PC save (5 slots) |
| PS4 Save | `tmp/save/oisisk_ps4.txt` | Real PS4 save |

---

## Documentation Conventions

- **Offsets** written as hex: `0x1B0`
- **Sizes** in hex bytes and decimal: `0x12F (303 bytes)`
- **Data types**: u8, u16, u32, i32, f32, u64 (little-endian)
- **Strings**: UTF-16LE, we specify max character count (not bytes)
- **Variable-length sections**: marked as `[VARIABLE]`
- **Unknown fields**: marked as `unk_0xNN` with a note about what is known
- **Verification status**: ✅ hex-verified | ⚠️ cross-reference only | ❓ uncertain

---

## Translations

Polish translations of all spec documents are available in [`spec/lang-pl/`](lang-pl/).
