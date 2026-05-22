# Elden Ring Save File Format — Modder's Handbook

> **Purpose**: Complete documentation of the Elden Ring binary save file format (`.sl2` / `memory.dat`) and the SaveForge editor systems. Sufficient for implementing an independent save editor from scratch.
>
> **State**: 🚧 **Work in progress — book cleanup**.
> Phase 1 (directory reorganization) ✅ completed. Phase 2 (GaItem + Inventory + Storage + Transfer + Sort Order + Categories + Equipment) ✅ completed for main chapters (03, 06, 07, 10, 35, 36, 39, 43, 52, 53). Phase 3 (Ash of War + Build Template) ✅ completed for main chapters (54, 55). Phase 4 (Map / World / Event Flags / Game State) ✅ completed for main chapters (11, 14, 15, 16, 17, 19, 27, 29, 47, 48, 50). Next: Phase 5 — Ban-risk / unsafe edits / validation / safety consolidation. Further phases (5–6) — see `BOOK_PLAN.md`.
>
> **Plan for further work**: see [`BOOK_PLAN.md`](BOOK_PLAN.md). Source audit result: [`tmp/docs-book-audit.md`](../tmp/docs-book-audit.md) (local, gitignored).

---

## How to read this documentation

All documents live directly in `spec/`. Most are **canonical handbook chapters** — verified knowledge about the binary format and implemented editor systems. A few documents carry `research` or `planned` status — clearly marked in the table of contents — and remain in the main directory as supplementary reference material.

**Status legend** used in the table of contents below:

| Status | Meaning |
|---|---|
| `canonical` | Current, aligned with code, candidate for the final book chapter |
| `implemented, needs rewrite` | Code exists and works, but the document needs rewriting in canonical template |
| `partial` | Partially verified / partially implemented — needs supplementation |
| `needs verification` | Doc vs code conflict — requires manual per-section verification |
| `research` | Negative result or paused investigation |
| `planned` | Design without implementation in code |

---

## Platforms

| Platform | File | Encryption | Checksums |
|---|---|---|---|
| PC (Steam) | `ER0000.sl2` | AES-128-CBC (optional) | MD5 per slot |
| PS4 | `memory.dat` | None | None |
| PS5 | `memory.dat` | None | None |

---

## File structure — overview

The save file consists of the following main blocks (in sequential order):

```
┌─────────────────────────────────────────────┐
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
│    ├── Storage Box (chest)                  │  → 10-storage.md
│    ├── Regions (unlocked regions)           │  → 11-regions.md
│    ├── Torrent (horse)                      │  → 12-torrent.md
│    ├── Blood Stain (death marker)           │  → 13-blood-stain.md
│    ├── Game State                           │  → 14-game-state.md
│    ├── Event Flags                          │  → 15-event-flags.md
│    ├── World State                          │  → 16-world-state.md
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

## Phase 2 — cross-cutting gaps

After Phase 2 (canonical rewrites: 03, 06, 07, 10, 35, 36, 39, 43, 52, 53), the following `needs verification` items remain scattered across chapters. They are **not** blockers for Phase 3+, but should be addressed in future iterations:

- **Storage Apply in-game / Steam Deck verification** — common gap for [52](52-acquisition-sort-stride2.md) and [53](53-inventory-storage-transfer.md). No fresh report from PS4 fixture after reorder/transfer.
- **Workspace path equipped guard** — `editor.ApplyWorkspaceSave` has no explicit `IsHandleEquipped` check (see [53](53-inventory-storage-transfer.md) "Equipped guard / Workspace path", [06](06-equipment.md) "Workspace path gap").
- **Workspace post-mutation validation** — unlike [43](43-transactional-item-adding.md), workspace save has no `ValidatePostMutation` (see [53](53-inventory-storage-transfer.md) "Validation and rollback caveats").
- **UI counters vs allocator capacity** — `SortOrderTab` per-tab counter is `view.length`, not container total; allocator capacity ([35](35-gaitem-allocator-invariants.md)) operates at a different level. No end-to-end cross-check test.
- **DLC sub-mapping completeness** — whether every DLC item in DB has an assigned sub-group (see [36](36-inventory-categories-game-order.md) "DLC flag mechanism"). Best-effort `melee_subcat.go` curated lookup.
- **Equipment write API not implemented** — the editor is read-only for ChrAsmEquipment (see [06](06-equipment.md) "What SaveForge does not implement"). `EquippedGreatRune` round-trips, but no public setter from UI.
- **Hash recompute discipline** — `RecalculateSlotHash` is called **only in tests** (see [06](06-equipment.md) "Hash recompute discipline"). `needs verification` end-to-end.
- **Game order in-game verification for the current game version** — last verification April 2026 (see [36](36-inventory-categories-game-order.md) "Status"). FromSoftware patches may reorganize the menu.

---

## Phase 3 — cross-cutting gaps

After Phase 3 (canonical rewrites: 54, 55), the following `needs verification` items remain scattered across chapters. They are **not** blockers for Phase 4+, but should be addressed in future iterations:

- **AoW affinity gating** — `EquipParamWeapon.defaultWepAttr` / `configurableWepAttr00..23` are not imported into `WeaponGemMounts`; Build Template preview validates compat only by `wepType`. `needs verification` in [54 §22.L1](54-ash-of-war.md) and [55 §21.L1](55-build-template.md).
- **DLC wepType 69/94/95 user-facing behavior** — backend allow-passthrough, UI treats them as `unknown` and fail-closes AoW section visibility; no information for the user that this is DLC with unknown compatibility. `needs verification` in [54 §22.L2](54-ash-of-war.md).
- **Frontend/backend `WEP_TYPE_TO_BIT` drift** — two frontend mirrors (`WeaponEditModal.tsx`, `WeaponEditTab.tsx`), both manually maintained; identical to backend but no CI / generator guard. `needs verification` on every backend change. See [54 §17 / §22.L4](54-ash-of-war.md).
- **`gemMountType == 1` (somber) edge cases in UI** — `CanMountAoW = false` disables the AoW section, but documentation does not confirm a placeholder/explanation. `needs verification` in [54 §22.L3](54-ash-of-war.md).
- **`AoWCompatMasks` completeness after regulation update** — bitmask generated from `EquipParamGem`; new DLC rows may not be re-imported. `needs verification` in [54 §22.L5](54-ash-of-war.md).
- **Orphan AoW GaItem GC / save bloat** — the allocator does not release handles after AoW reset; save grows linearly with the number of AoW edits. `needs verification` in [54 §22.L6](54-ash-of-war.md).
- **Build Template equipment write API** — ❌ not implemented; apply leaves weapons unequipped. `needs verification` for a future Phase in [55 §12 / §21.L3](55-build-template.md).
- **Build Template spell loadout / character stats / affinity / DLC presence cross-check** — schema v1 does not export attunement slots or stats; DLC presence at apply flags individual items without a global "needs DLC X" warning. `needs verification` in [55 §6 / §21.L6](55-build-template.md).
- **Build Template `replace-*` modes** — `replace-weapons`, `replace-armors`, etc., are reserved in the schema but not implemented; v1 supports only `merge`. See [55 §6](55-build-template.md).
- **Build Template forward-compat `version=2` tests** — `SchemaVersion=1` is the only accepted version; no tests for unknown-future-fields scenarios. `needs verification` in [55 §18 / §21.L8](55-build-template.md).
- **Cross-platform PS4 vs PC portability for Build Template** — the schema is portable by design (no save-local handles), but no E2E test for PS4 → PC export and vice versa. `needs verification`.

---

## Phase 4 — cross-cutting gaps

After Phase 4 (canonical rewrites and refresh: 11, 14, 15, 16, 17, 19, 27, 29, 47, 48, 50), the following `needs verification` items remain scattered across chapters. They are **not** blockers for Phase 5+, but should be addressed in future iterations:

- **Stale generated snapshots after game / regulation.bin patch** — `data.Graces` (419), `data.Regions` (104), `data.MapVisible` (263), `data.SummoningPools` (~213), `data.ColosseumFlagSets` (3), `itemCompanionEventFlags` (6) are static snapshots; no auto-detection of "regulation.bin newer than snapshot" (see [11 §16](11-regions.md), [15](15-event-flags.md), [27 §13](27-map-reveal.md), [47 §16.1](47-site-of-grace-activation.md), [50 §12.1](50-item-companion-flags.md)).
- **PS4 ↔ PC parity tests** — `tests/roundtrip_test.go` covers I/O round-trip, but no per-endpoint platform parity (e.g., `SetGraceVisited` PC vs PS4 effect identical). `needs verification` in [11](11-regions.md), [16 §18.6](16-world-state.md), [14 §16.5](14-game-state.md).
- **No cross-subsystem atomic transaction** — orchestrators (`RevealAllMap`, `ApplyPvPPreparation`, `SaveCharacter`) use a single `pushUndo`, but per-phase / per-module rollback does not exist (see [16 §17](16-world-state.md), [27 §12](27-map-reveal.md), [48 §14](48-pvp-ready-modular-presets.md), [14 §15](14-game-state.md)).
- **Manual undo / rollback limits** — undo stack depth limit is unknown (`needs verification`); bulk operations (UI Promise.all) create N separate snapshots; after `WriteSave` the only rollback path is the backup `.sl2.YYYYMMDD_HHMMSS.bkp` (see [47 §15.2](47-site-of-grace-activation.md), [50 §11](50-item-companion-flags.md), [16 §18.5](16-world-state.md)).
- **In-game runtime verification gaps** — most helpers have ad-hoc CHANGELOG entries, with no automated in-game loop in CI for map reveal / DLC tiles / Sites of Grace / PvP matchmaking / NG+ flag sync (see [27 §13](27-map-reveal.md), [29 §13.6](29-dlc-black-tiles.md), [47 §16](47-site-of-grace-activation.md), [48 §16.7](48-pvp-ready-modular-presets.md), [14 §16.6](14-game-state.md)).
- **Event flag ID correctness / stale precomputed caches** — the 3-tier resolver (precomputed → BST → fallback formula) is a snapshot; new flags from patches may fall back to unintended places (see [15](15-event-flags.md)).
- **Map reveal visual vs gameplay/progression effects** — UI note `WorldTab.tsx:406` says "Some graces may still play their in-world activation sequence"; no isolated test of whether removing L2 affects trophies / achievements (see [27 §13](27-map-reveal.md), [29 §13.5](29-dlc-black-tiles.md), [47 §7](47-site-of-grace-activation.md)).
- **DLC black tile coordinates stale-after-patch risk** — synthetic values `9648/9124` and `3037/1869/7880/7803` are empirical (snapshot from 2 slots); not game-guaranteed (see [29 §13.1](29-dlc-black-tiles.md)).
- **Sites of Grace SET-only intent + PvP module E placeholder** — companion flag SET-only contract in grace lifecycle is intentional; PvP module E (`opts.SitesOfGrace`) is a placeholder without mutation (see [47 §9](47-site-of-grace-activation.md), [48 §7](48-pvp-ready-modular-presets.md)).
- **Item companion flag IDs stale-after-patch** — 6 entries in `itemCompanionEventFlags` with hardcoded literals; no detection mechanism (see [50 §12.1](50-item-companion-flags.md)).
- **PvP "ready" scope limited** — Matchmaking Regions ✅; Colosseums with physical gates in the `WorldGeomMan` blob (non-editable); Summoning Pools impact "Bloody Finger invasion impact is unconfirmed"; Sites of Grace module **disabled** (see [48 §16.1](48-pvp-ready-modular-presets.md)).
- **Player Coordinates / Weather-Time read-only** — no public setters; `grep "Set..." → 0`; any mutation via direct hex edit is at the user's responsibility (see [17 §6](17-player-coordinates.md), [19 §6](19-weather-time.md)).
- **Game State: LastRestedGrace read-only, ClearCount as the only write path** — `LastRestedGrace` BonfireId is read-only (the game manages it at runtime); `ClearCount` has a write path via `SaveCharacter` + event flag sync 50-57, but no progression consistency validator (see [14 §8.3](14-game-state.md), [14 §9.2](14-game-state.md)).
- **Boss multi-flag editor remains planned** — current `SetBossDefeated` is single-flag; multi-flag design in [38-boss-multiflag.md](38-boss-multiflag.md) (see [14 §12](14-game-state.md)).

---

## Book table of contents

### Part I — Save File Format Fundamentals

Binary format of the save file — container, slots, section layout.

| Doc | Title | Status | Note |
|---|---|---|---|
| 01 | [Header and file layout](01-header.md) | `canonical` | Magic bytes, platform detection, BND4, MD5 |
| 02 | [Slot — general structure](02-slot-structure.md) | `canonical` | Slot size, sequential parsing |
| 03 | [GaItem Map](03-gaitem-map.md) | `canonical` | GaItem layout + GaMap; AoW semantics in 54 (cross-ref, no duplication) — Phase 2 Step 2 |
| 04 | [PlayerGameData](04-player-game-data.md) | `canonical` | 432 B, attributes, runes, online settings |
| 05 | [SP Effects](05-sp-effects.md) | `needs verification` | Short section, "needs verification" noted in content; no parser in `backend/core/` |
| 06 | [Equipment](06-equipment.md) | `canonical` (read-only) | `EquippedItemsItemIds` (88B) + `EquippedGreatRune`; **no public write API** for equipment — Phase 2 Step 8 |
| 07 | [Inventory](07-inventory.md) | `canonical` | Read-side 12B record + CommonItems offsets — Phase 2 Step 3 |
| 08 | [Spells & Gestures](08-spells-gestures.md) | `canonical` | 14 attunement + 8B gesture stride |
| 09 | [Face Data](09-face-data.md) | `partial` | 303 B, fields 0x20-0x5F "approximate" — code (`app_appearance.go`) knows more |
| 10 | [Storage Box](10-storage.md) | `canonical` | Read-side: `StorageBoxOffset` + `StorageHeaderSkip`, sparse list — Phase 2 Step 3 |
| 11 | [Regions](11-regions.md) | `canonical` | `core.SetUnlockedRegions`, L0 Map Reveal detail (cross-link to 27) — Phase 4 Step 3 |
| 12 | [Torrent](12-torrent.md) | `canonical` | State enum 1/3/13; bug HP=0+State=13 |
| 13 | [Blood Stain](13-blood-stain.md) | `partial` | unk_0x1c..0x40 — in spec/29 these offsets are the DLC Cover Layer (conflict to resolve) |
| 14 | [Game State](14-game-state.md) | `canonical` | `PreEventFlagsScalars` (29 B), ClearCount/NG+ write path, LastRestedGrace read-only — Phase 4 Step 10 |
| 15 | [Event Flags](15-event-flags.md) | `canonical` | 3-tier resolver (precomputed → BST → fallback), helper API — Phase 4 Step 1 |
| 16 | [World State](16-world-state.md) | `canonical` (overview/index) | Subsystem map, read-only vs write-capable; links 11/15/27/29/47/48/50/14 — Phase 4 Step 8 |
| 17 | [Player Coordinates](17-player-coordinates.md) | `canonical` (read-only) | `PlayerCoordinates` (**61 B**, NOT 57 B), `SpawnPointBlock` version-gated, no setters — Phase 4 Step 9 |
| 18 | [Network Manager](18-network.md) | `partial` (merge candidate) | 131 KB opaque — slot-local, not regulation NetworkParam |
| 19 | [Weather & Time](19-weather-time.md) | `canonical` (read-only) | `WorldAreaWeather` + `WorldAreaTime` (12+12 B), no setters — Phase 4 Step 9 |
| 20 | [Version & Platform](20-version-platform.md) | `canonical` | Steam ID + BaseVersion + PS5Activity |
| 21 | [DLC](21-dlc.md) | `canonical` | 50 B, invariant bytes 3-49 = 0 |
| 22 | [Player Data Hash](22-player-hash.md) | `canonical` | "The game ignores the hash" — `backend/core/hash.go` confirms |
| 23 | [UserData10](23-user-data-10.md) | `canonical` | PC + PS4 offsets verified Apr 2026 |
| 24 | [UserData11](24-user-data-11.md) | `canonical` (read-only) | HARD RULE — do not modify |
| 25 | [Runtime vs Save](25-runtime-vs-save.md) | `canonical` | CT memory vs save offsets — educational value |
| 26 | [Parameter Reference](26-parameter-reference.md) | `partial` (needs rewrite) | Title promises more than it delivers — attributes + softcaps |

### Part II — Implemented SaveForge Systems

Implemented editor mechanisms — working in the current code.

| Doc | Title | Status | Note |
|---|---|---|---|
| 32 | [Ban-Risk System (UI)](32-ban-risk-system.md) | `canonical` | SafetyMode, RISK_INFO, `Risk*` components |
| 34 | [Item Caps Enforcement](34-item-caps.md) | `canonical` | `scales_with_ng` + NG+ scaling — TODO on ClearCount open |
| 35 | [GaItem Allocator & Invariants](35-gaitem-allocator-invariants.md) | `canonical` | Handle allocation, capacity invariants, `generateUniqueHandle`/`allocateGaItem` — Phase 2 Step 1 |
| 36 | [Inventory Categories — Game Order](36-inventory-categories-game-order.md) | `canonical` | 18 DB tabs + handle prefix bridge + sub-categories (76) + DLC flag mechanism — Phase 2 Step 7 (supersedes 33) |
| 39 | [Inventory Reorder](39-inventory-reorder.md) | `historical / superseded` | Design note from project phase; **superseded by 52** for acquisition/stride mechanics, **covered by 53** for transfer UX — Phase 2 Step 5 |
| 43 | [Transactional Item Adding](43-transactional-item-adding.md) | `canonical` | Pre-flight + `SnapshotSlot`/`RestoreSlot` + `ValidatePostMutation` — Phase 2 Step 4 |
| 44 | [NetworkParam Tuning](44-network-param-tuning.md) | `canonical` | `regulation.go::PatchNetworkParams` |
| 50 | [Item Companion Flags](50-item-companion-flags.md) | `canonical` | SET+CLEAR symmetric mechanism, 6 entries (Whistle + 5 multiplayer items); separate from grace SET-only ([47](47-site-of-grace-activation.md)) and `MapFragmentItems` ([27](27-map-reveal.md)) — Phase 4 Step 6 |
| 52 | [Acquisition Sort — Stride-2](52-acquisition-sort-stride2.md) | `canonical` | Stride-2 algorithm, bucket-collision guard, 3 write paths (`ReorderInventory`/`ReorderStorage`/`writeContainerLayout`) — Phase 2 Step 5 |
| 53 | [Inventory ↔ Storage Transfer](53-inventory-storage-transfer.md) | `canonical` | Two transfer paths (legacy core + workspace), rehandle, equipped guard, SortOrderTab workspace UI — Phase 2 Step 6 |
| 54 | [Ash of War](54-ash-of-war.md) | `canonical` | Sentinels 0x00/0xFFFFFFFF + uniqueness invariant + AoW guard (commit `6881cb9`), workspace/strict write paths, fail-closed compat — Phase 3 Step 1 |
| 55 | [Build Template](55-build-template.md) | `canonical` | JSON v1, portable export without save-local handles, capacity preflight, RAM-only apply with rollback, Phase E local library — Phase 3 Step 2 |

### Part III — Verified Game Mechanics

Game mechanics verified via RE / in-game tests.

| Doc | Title | Status | Note |
|---|---|---|---|
| 27 | [Map Reveal](27-map-reveal.md) | `canonical` | 4-layer model (L0–L3); `RevealAllMap` / `revealBaseMap` / `revealDLCMap` / `RemoveFogOfWar` / `ResetMapExploration` — Phase 4 Step 2 |
| 29 | [DLC Black Tiles](29-dlc-black-tiles.md) | `canonical` (detail) | L2 DLC Cover Layer in `BloodStain`; `DLCTile*` constants + synthetic coords + `revealDLCMap` Phase 3 — Phase 4 Step 4 |
| 31 | [Appearance Presets](31-appearance-presets.md) | `canonical` | Apply algorithm + Mirror Favorites; PC verified |
| 47 | [Sites of Grace — Activation](47-site-of-grace-activation.md) | `canonical` | Grace EventFlag 71xxx-76xxx + DoorFlag + companion flags SET-only (Gatefront); 419 entries — Phase 4 Step 5 |
| 48 | [PvP Modular Presets](48-pvp-ready-modular-presets.md) | `current reference` | `ApplyPvPPreparation` with 5 modules (4 active + 1 placeholder); Sites of Grace module E returns a warning without mutation (`app_pvp.go:109`) — Phase 4 Step 7 |
| 49 | [PS4 ZSTD Raw-Block Patch](49-ps4-zstd-rawblock-patch.md) | `canonical` | Critical PS4 knowledge — `regulation.go:604` |

### Part IV — Research Archive / Negative Results

Research history, paused investigations, negative results.

| Doc | Title | Status | Note |
|---|---|---|---|
| 30 | [Slot Rebuild — Research](30-slot-rebuild-research.md) | `research` | Slack measurement log across 11 slots; final implementation: `RebuildSlot` |
| 42 | [Summoning Pools Bug](42-summoning-pools-bug.md) | `research` | 🐛 Paused — UI works, no in-game effect |

### Planned

Design docs without implementation in code.

| Doc | Title | Status | Note |
|---|---|---|---|
| 37 | [Character Presets (JSON)](37-character-presets.md) | `needs verification` ⚠️ | `backend/vm/preset.go` has `CharacterPreset/VMToPreset/PresetToVM/ValidatePreset`, but the doc declares "Planned". Requires per-phase verification. |
| 38 | [Boss Multi-Flag](38-boss-multiflag.md) | `planned` | Code has a 1-flag model; design requires `EventFlags []uint32` (not implemented) |

### Appendix (planned)

| Doc | Title | Status | Note |
|---|---|---|---|
| 45 | [Ban Risk Reference](45-ban-risk-reference.md) | `canonical` (App. A) | Community triggers — knowledge base, basis for tiers in 32 |
| 99 | [Verification Methodology](99-verification-methodology.md) | `canonical` (App. B) | Research methodology |

---

## Key format properties

- **Endianness**: Little-endian (all numeric values)
- **Strings**: UTF-16LE with null terminator
- **Slot size**: 0x280000 (2,621,440 bytes) — fixed
- **Variable-length sections**: inventory projectiles, regions, world areas — require sequential parsing
- **Checksums**: MD5 (PC only), recomputed on write
- **Encryption**: AES-128-CBC (PC only, optional — newer game versions)

---

## Knowledge sources

### Reference projects (local copies in `tmp/repos/`)

| Project | Language | Priority | Description |
|---|---|---|---|
| [er-save-manager](https://github.com/Jeius/er-save-manager) | Python | **1 (highest)** | Most recent, complete sequential parser with DLC support |
| [ER-Save-Editor](https://github.com/ClayAmore/ER-Save-Editor) | Rust | **2** | Well-typed parser, confirms struct sizes |
| [Elden-Ring-Save-Editor](https://github.com/shalzuth/Elden-Ring-Save-Editor) | Python | **3 (lowest)** | Old, pattern-matching approach, but first offset discoveries |

### Online documentation

| Source | URL | Content |
|---|---|---|
| Souls Modding Wiki — SL2 Format | https://www.soulsmodding.com/doku.php?id=format:sl2 | Save container format |
| Souls Modding Wiki — Event Flags | https://www.soulsmodding.com/doku.php?id=er-refmat:event-flag-list | Event flag list |
| Event Flags GitHub Pages | https://soulsmods.github.io/elden-ring-eventparam/ | Full list of 1000+ flags with descriptions |
| Event Flags Spreadsheet | https://docs.google.com/spreadsheets/d/1Nn-d4_mzEtGUSQXscCkQ41AhtqO_wF2Aw3yoTBdW9lk | Detailed flag spreadsheet |
| Steam Guide — Save Structure | https://steamcommunity.com/sharedfiles/filedetails/?id=2797241037 | Slot offsets, MD5 checksums |
| SoulsFormats (C#) | https://github.com/JKAnderson/SoulsFormats | BND4 format parsing library |
| TGA Cheat Engine Table | https://github.com/The-Grand-Archives/Elden-Ring-CT-TGA | Event flags, param scripts, item IDs |
| Souls Modding Wiki — Params | https://www.soulsmodding.com/doku.php?id=er-refmat:param:speffectparam | SpEffect and other param tables |

### Data files (local)

| File | Path | Description |
|---|---|---|
| eventflag_bst.txt | `tmp/repos/er-save-manager/src/resources/eventflag_bst.txt` | 11919 entries — block→offset mapping for event flags |
| PC Save | `tmp/save/ER0000.sl2` | Real PC save (5 slots) |
| PS4 Save | `tmp/save/oisisk_ps4.txt` | Real PS4 save |

---

## Documentation conventions

- **Offsets** written as hex: `0x1B0`
- **Sizes** in hex bytes and decimal: `0x12F (303 bytes)`
- **Data types**: u8, u16, u32, i32, f32, u64 (little-endian)
- **Strings**: UTF-16LE, we quote the max character count (not bytes)
- **Variable-length sections**: marked as `[VARIABLE]`
- **Unknown fields**: marked as `unk_0xNN` with a note on what we know
- **Verification status**: ✅ hex-verified | ⚠️ cross-reference only | ❓ uncertain

---

## Translations

Polish translations of all specification documents live in the Polish documentation subdirectory. **Note**: the Polish documentation is the current source of truth; the English mirror is rebuilt from PL after the Phase 1–4 cleanup.
