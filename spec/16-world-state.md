# 16 — World State

> **Type**: Binary format spec + overview chapter
> **Scope**: An index of the World State subsystems in SaveForge — what is a read-only blob, what has write helpers, how it separates from Game State and Event Flags.

Cross-refs: [11-regions.md](11-regions.md), [14-game-state.md](14-game-state.md), [15-event-flags.md](15-event-flags.md), [17-player-coordinates.md](17-player-coordinates.md), [19-weather-time.md](19-weather-time.md), [27-map-reveal.md](27-map-reveal.md), [29-dlc-black-tiles.md](29-dlc-black-tiles.md), [47-site-of-grace-activation.md](47-site-of-grace-activation.md), [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md), [50-item-companion-flags.md](50-item-companion-flags.md).

---

## 1. Chapter purpose

To define unambiguously:

- what SaveForge treats as **World State** (binary sections representing the state of the game's physical world),
- which parts are **read-only blobs** (parsed verbatim, written without editing),
- which parts have **write helpers** in `app_world.go` (mostly indirectly through the event flag bitfield),
- how World State separates from Game State ([14](14-game-state.md)) and Event Flags ([15](15-event-flags.md)),
- where the thin line "read-only blob but we have 1 partial mutator" is — `BloodStain` in the context of DLC tiles.

This chapter is an **overview chapter** — it does not duplicate write path details (those are in 11/15/27/29/47/48/50). It serves as an index of the subsystems.

## 2. Status

| Aspect | Status |
|---|---|
| Binary structure parsers (`backend/core/section_world*.go`) | ✅ implemented — read/write verbatim |
| `WorldGeomBlock` (5 size-prefixed blobs) | 🔒 read-only (`SizePrefixedBlob` verbatim copy) |
| `BloodStain` section | 🔒 read-only **with 1 exception**: L2 DLC cover layer (`revealDLCMap` Phase 3) — see [29](29-dlc-black-tiles.md) |
| `RideGameData` (Torrent horse state) | 🔒 read-only verbatim |
| `WorldAreaWeather` / `WorldAreaTime` | 🔒 read-only verbatim — see [19](19-weather-time.md) |
| Event flag bitfield (caller domain) | ✅ write helpers in `app_world.go` (~34 App methods across ~12 categories: Graces / Bosses / Pools / Regions / Colosseums / Gestures / Quests / Cookbooks / Bells / Whetblades / Ashes / Map) |
| `UnlockedRegions` array | ✅ write helpers (`core.SetUnlockedRegions` + UI) — see [11](11-regions.md) |
| Map reveal orchestration | ✅ `RevealAllMap` / `RemoveFogOfWar` / `ResetMapExploration` — see [27](27-map-reveal.md) |
| PvP preset orchestration | ✅ `ApplyPvPPreparation` (5 modules, 1 placeholder) — see [48](48-pvp-ready-modular-presets.md) |
| Game State (LastRestedGrace, TotalDeathsCount, etc.) | 🔒 NOT modified by `app_world.go` — see [14](14-game-state.md) |

## 3. Source of truth in code

| File / symbol | What it contains | Mode |
|---|---|---|
| `backend/core/section_world_geom.go::WorldGeomBlock` (lines 54-96, struct + Read/Write/ByteSize) | `FieldArea + WorldArea + WorldGeomMan + WorldGeomMan2 + RendMan` — 5 `SizePrefixedBlob`s | read/write verbatim |
| `backend/core/section_world_geom.go::SizePrefixedBlob` (lines 17-48) | `(size: i32, data: bytes[size])` with sanity ceilings | helper |
| `backend/core/section_world.go::RideGameData` | Torrent horse state — `Coordinates + MapID + Angle + HP + State` (40 B) | read/write verbatim |
| `backend/core/section_trailing.go::WorldAreaWeather` / `WorldAreaTime` (12+12 B) | Trailing fixed block, see [19](19-weather-time.md) | read/write verbatim |
| `backend/core/offset_defs.go::DynBloodStain` (`= 0x4C`) | Offset of the `BloodStain` blob | constant |
| `backend/core/offset_defs.go::DLCTile*` (lines 309-322) | Partial mutator for L2 DLC tiles inside `BloodStain` | see [29](29-dlc-black-tiles.md) |
| `app_world.go` (1200+ lines) | All write helpers per subsystem | App-level (Wails binding) |
| `app_pvp.go::ApplyPvPPreparation` | Orchestrator of 5 modules | App-level |
| `frontend/src/components/WorldTab.tsx` | UI: graces / bosses / pools / colosseums / regions / map / gestures / cookbooks / bell bearings / whetblades / quests / Ashes | UI |
| `frontend/src/components/PvPPreparationTab.tsx` | UI for the preset orchestrator | UI |

`needs verification`: the list of 5 blobs in `WorldGeomBlock` is a snapshot; the sanity ceilings (`fieldAreaMaxSize = 0x10000`, `worldAreaMaxSize = 0x10000`, `worldGeomManMaxSize = 0x100000`, `rendManMaxSize = 0x100000`) may change after a game patch.

## 4. Mental model

```
┌────────────────────────────── World State ─────────────────────────────┐
│                                                                          │
│  ┌─ EVENT FLAG BITFIELD (write helpers via app_world.go) ─────────┐      │
│  │   • Graces / Bosses / Pools / Colosseums / Regions / Map       │      │
│  │     / Gestures / Cookbooks / Bell Bearings / Whetblades        │      │
│  │     / Quests / Ashes (Tier 1 / Tier 2 per category)            │      │
│  │   • Everything goes through db.SetEventFlag (15)               │      │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
│  ┌─ UNLOCKED REGIONS array (write helper core.SetUnlockedRegions) ┐      │
│  │   • Variable-length []u32; mutation via RebuildSlot            │      │
│  │   • See [11]                                                   │      │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
│  ┌─ BloodStain section (offset DynBloodStain = 0x4C) ──────────────┐    │
│  │   • Verbatim blob with 1 exception: L2 DLC tile coords          │    │
│  │   • See [29] for DLCTile* constants and the partial mutator       │    │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
│  ┌─ WorldGeomBlock (verbatim blobs, 5 size-prefixed sections) ────┐     │
│  │   FieldArea + WorldArea + WorldGeomMan + WorldGeomMan2 + RendMan│     │
│  │   • Never mutated by SaveForge                                   │     │
│  │   • Read/write through SizePrefixedBlob.Read/Write                │     │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
│  ┌─ RideGameData (Torrent horse) ──────────────────────────────────┐    │
│  │   40 B; verbatim blob — no endpoint modifies it                   │    │
│  └─────────────────────────────────────────────────────────────────┘     │
│                                                                          │
│  ┌─ WorldAreaWeather + WorldAreaTime (trailing) ───────────────────┐    │
│  │   24 B total; verbatim — see [19]                                  │    │
│  └─────────────────────────────────────────────────────────────────┘     │
└──────────────────────────────────────────────────────────────────────────┘

  ─── outside World State ───
  
  ┌─ Game State (PreEventFlagsScalars + LastRestedGrace) ───┐
  │   See [14]; NOT modified by app_world.go                  │
  └────────────────────────────────────────────────────────────┘
  
  ┌─ Player Coordinates ─────────────────────────────────────┐
  │   See [17]; separate section                              │
  └────────────────────────────────────────────────────────────┘
```

## 5. World State subsystem map

A table of subsystems with exactly one pointer to the canonical chapter:

| Subsystem | Canonical chapter | Edit mode | Main endpoints |
|---|---|---|---|
| Event Flags (foundation) | [15](15-event-flags.md) | RW helper API | `db.GetEventFlag` / `db.SetEventFlag` |
| Regions / `UnlockedRegions` | [11](11-regions.md) | RW (RebuildSlot) | `GetUnlockedRegions`, `SetRegionUnlocked`, `BulkSetUnlockedRegions` |
| Map Reveal (4 layers) | [27](27-map-reveal.md) | RW orchestrator | `GetMapProgress`, `SetMapFlag`, `SetMapRegionFlags`, `RevealAllMap`, `ResetMapExploration`, `RemoveFogOfWar` |
| DLC Black Tiles (L2) | [29](29-dlc-black-tiles.md) | RW partial mutator in `BloodStain` | only via `revealDLCMap` Phase 3 |
| Sites of Grace | [47](47-site-of-grace-activation.md) | RW | `GetGraces`, `SetGraceVisited` |
| Item Companion Flags | [50](50-item-companion-flags.md) | RW hooks SET+CLEAR | only via `AddItemsToCharacter` / `RemoveItemsFromCharacter` |
| PvP modular presets | [48](48-pvp-ready-modular-presets.md) | RW orchestrator | `ApplyPvPPreparation` (5 modules, 1 placeholder) |
| Bosses / Pools / Colosseums / Gestures / Cookbooks / Bells / Whetblades / Quests / Ashes | n/a — direct app_world.go | RW per-category | `GetXxx` / `SetXxxYyy` / `BulkSetXxx` |
| Game State (LastRestedGrace, etc.) | [14](14-game-state.md) | RO **from the World State perspective** | `app_world.go` **does not modify** |
| WorldAreaWeather / Time | [19](19-weather-time.md) | RO verbatim | `section_trailing.go` |
| Player Coordinates | [17](17-player-coordinates.md) | RO/RW per chapter | `section_player_coords.go` |

`needs verification`: the categories "Bosses / Pools / Colosseums / Gestures / Cookbooks / Bells / Whetblades / Quests / Ashes" have no dedicated canonical chapters in Phase 4 — they are covered through `15-event-flags.md` (generic API) + per-category snapshots in `backend/db/data/`. The absence of a dedicated chapter is the current state; future chapters may be added.

## 6. Read-only vs write-capable areas

### 6.1 Verbatim read-only blobs (NOT mutated by SaveForge)

| Section | Typical size | Sanity ceiling | Comment |
|---|---|---|---|
| `FieldArea` | variable | `0x10000` (64 KB) | data of the region the player is in |
| `WorldArea` | variable | `0x10000` (64 KB) | NPCs/characters in the world per map (`WorldAreaChrData`) |
| `WorldGeomMan` (instance 1) | variable | `0x100000` (1 MB) | world geometry per map |
| `WorldGeomMan2` (instance 2) | variable | `0x100000` (1 MB) | second instance — probably before/after or two layers |
| `RendMan` | variable | `0x100000` (1 MB) | renderer manager (lighting, particles) |
| `RideGameData` | 40 B (`RideGameDataSize`) | n/a | Torrent horse: `Coordinates + MapID + Angle + HP + State` |
| `WorldAreaWeather` | 12 B | n/a | see [19](19-weather-time.md) |
| `WorldAreaTime` | 12 B | n/a | see [19](19-weather-time.md) |

`needs verification`: the exact offset and order of these sections in the binary save. The current code uses a typed parser (`SizePrefixedBlob.Read/Write`), not per-section hex offsets. See `backend/core/section_world_geom.go` for the order in `WorldGeomBlock.Read`.

### 6.2 Partially mutated (1 exception)

| Section | What we edit | Caller |
|---|---|---|
| `BloodStain` (offset `DynBloodStain = 0x4C`) | only `[0x0088..0x0110)` (DLC tile coords) | `revealDLCMap` Phase 3 — see [29](29-dlc-black-tiles.md) |

This is the **only** point where SaveForge mutates anything inside a World State blob other than `WorldGeomBlock` / `RideGameData` / weather/time. The rest of `BloodStain` (outside the `[0x0088..0x0110)` range) is verbatim.

### 6.3 Write-capable via the event flag bitfield

All the endpoints below from `app_world.go` operate **only** on the Event Flags bitfield (see [15](15-event-flags.md)) — **not** on World State blobs:

```
GetGraces / SetGraceVisited
GetBosses / SetBossDefeated
GetSummoningPools / SetSummoningPoolActivated
GetUnlockedRegions / SetRegionUnlocked / BulkSetUnlockedRegions  ← the only one operating on a NON-bitfield (UnlockedRegions array)
GetColosseums / SetColosseumUnlocked
GetGestures / SetGestureUnlocked / BulkSetGesturesUnlocked
GetQuestNPCs / GetQuestProgress / SetQuestStep
GetCookbooks / SetCookbookUnlocked / BulkSetCookbooksUnlocked
GetBellBearings / SetBellBearingUnlocked / BulkSetBellBearings
GetWhetblades / SetWhetbladeUnlocked
GetAshOfWarFlags / SetAshOfWarFlagUnlocked / BulkSetAshOfWarFlags
GetMapProgress / SetMapFlag / SetMapRegionFlags / RevealAllMap / ResetMapExploration / RemoveFogOfWar
```

All write paths go through `db.SetEventFlag(flags, id, value)` with the generic helper API ([15](15-event-flags.md)). Per-category ID snapshots are in `backend/db/data/`.

## 7. Event Flags relation

The event flag bitfield ([15](15-event-flags.md)) is the **foundation** of most World State mutations. All the subsystems from §5 (except `UnlockedRegions` / DLC tiles / verbatim blobs / Player Coordinates) operate only on flags.

This chapter **does not duplicate** byte/bit indexing, the BST resolver, or the helper API. See [15](15-event-flags.md) §1–§9.

## 8. Map Reveal relation

Map Reveal ([27](27-map-reveal.md)) is a 4-layer model:

- **L0** `UnlockedRegions` — see [11](11-regions.md),
- **L1** event flags 62xxx/63xxx + Map Fragment items,
- **L2** DLC Cover Layer (BloodStain coords) — see [29](29-dlc-black-tiles.md),
- **L3** Fog of War bitfield.

`RevealAllMap` in `app_world.go:1041` is the orchestrator of L1+L2+L3 (excluding L0). See [27](27-map-reveal.md) §11 for the full pseudocode.

This chapter **does not duplicate** the layer model. It links.

## 9. Regions relation

`UnlockedRegions` ([11](11-regions.md)) is the **only** World State subsystem that:

- is not an event flag bitfield,
- has its own variable-length `[]u32` array,
- requires `core.SetUnlockedRegions` + `RebuildSlot` (reallocation of `slot.Data`),
- is the L0 layer of Map Reveal.

A consequence of `RebuildSlot` is recalculation of `EventFlagsOffset` — all the write helpers from §6.3 **must** refresh the slice `flags = slot.Data[slot.EventFlagsOffset:]` after `BulkSetUnlockedRegions` or `SetRegionUnlocked` with a length mutation. In `ApplyPvPPreparation` this happens explicitly (`app_pvp.go:57`).

## 10. DLC black tiles relation

The L2 DLC Cover Layer ([29](29-dlc-black-tiles.md)) is the **only** partial mutation inside the `BloodStain` blob. It is updated only by `revealDLCMap` Phase 3 with 9 coordinate constants (Rec1 X/Y/Flag + Rec2 X/Y/Z/W/Flag).

Stale-after-patch caveat: the coordinates are empirical (a snapshot from 2 reference slots), not game-guaranteed. See [29](29-dlc-black-tiles.md) §13.1 + §15.

## 11. Sites of Grace relation

Sites of Grace ([47](47-site-of-grace-activation.md)) operate on the event flag bitfield in the 71xxx–76xxx bands + an optional `DoorFlag` + companion flags (Gatefront only).

An important distinction with the PvP context:

- **Standalone**: `GetGraces` / `SetGraceVisited` in `app_world.go` are active; UI in `WorldTab.tsx` (per-grace, per-region, Unlock All Tier 1).
- **PvP preset module E** (`opts.SitesOfGrace` in `ApplyPvPPreparation`): **placeholder** — returns a warning without mutation. See [48](48-pvp-ready-modular-presets.md) §7.

Companion flag policy: SET-only asymmetric ([47](47-site-of-grace-activation.md) §9). Contrasts with SET+CLEAR symmetric in item companion flags ([50](50-item-companion-flags.md) §7).

## 12. Item Companion Flags relation

Item companion flags ([50](50-item-companion-flags.md)) are a **bridge** between item ownership and event flag state. SET hook in `AddItemsToCharacter`, CLEAR hook in `RemoveItemsFromCharacter` (after removing the last instance).

Currently 6 entries: Spectral Steed Whistle (4 companion flags) + 5 multiplayer items (1 flag each). Impact on World State: the flags set are in the 60xxx bands (obtained) + 4680/4681 (Melina state) + 710520 (whistle world state).

`MapFragmentItems` (from [27](27-map-reveal.md)) is NOT part of item companion flags — a separate mechanism. See [50](50-item-companion-flags.md) §9 for detail.

## 13. PvP-ready presets relation

PvP modular presets ([48](48-pvp-ready-modular-presets.md)) are an **orchestrator** of several World State subsystems in a single `ApplyPvPPreparation` call:

- Module 1 `MatchmakingRegions` → `core.SetUnlockedRegions` (L0 — [11](11-regions.md)),
- Module 2 `Colosseums` → bulk event flag SET (15 flags total),
- Module 3 `RevealMap` → `revealBaseMap` + `revealDLCMap` (L1+L2+L3 — [27](27-map-reveal.md), [29](29-dlc-black-tiles.md)),
- Module 4 `SummoningPools` → bulk event flag SET,
- Module 5 `SitesOfGrace` → **placeholder** (no mutation).

Single `pushUndo`, fail-fast without auto-restore. See [48](48-pvp-ready-modular-presets.md) §14 for rollback caveats.

## 14. Game State relation

Game State ([14](14-game-state.md)) contains `PreEventFlagsScalars` (a 29 B block directly before the Event Flags bitfield) with fields:

- `LastRestedGrace` (BonfireId — see [47](47-site-of-grace-activation.md) §6.3),
- `TotalDeathsCount`, `ClearCount`, `CharacterType`, `InOnlineSessionFlag`, `InGameCountdownTimer`, etc.

`app_world.go` **does not modify** any of these fields. Game State mutations (if any) go through separate endpoints (`app.go`, `app_character.go`, etc. — see [14](14-game-state.md)).

`needs verification`: the list of write endpoints for Game State in the current code — outside the scope of this chapter.

## 15. Current implemented behavior

### 15.1 Read endpoints

All `GetXxx` endpoints from §6.3 are **tolerant**:

- No `EventFlagsOffset` → returns the full list with `Visited/Defeated/Activated/...=false`.
- No save / invalid slot → error.
- Per-entry error in the bitfield → `fmt.Printf` warning, continues the iteration (NOT propagated).

Result: `GetXxx` may return partial information with 0 errors — the UI must handle this consciously.

### 15.2 Per-category write endpoints

Each `SetXxxYyy` in `app_world.go`:

1. Validates `save != nil`, slot index in range, `EventFlagsOffset > 0`.
2. Calls `a.pushUndo(slotIndex)` — a single snapshot per call.
3. Mutates 1 or several event flag bits.
4. (For some categories) Additionally mutates related flags (e.g., `DoorFlag` in `SetGraceVisited`).

Bulk endpoints (`BulkSetXxx`) iterate over the ID list in **one** call — single `pushUndo`, sequential SET. No rollback on partial failure.

### 15.3 Orchestrators

- `RevealAllMap` ([27](27-map-reveal.md)) — single `pushUndo` + 3 phases (L1 flags → L1 items → L2 BloodStain).
- `ApplyPvPPreparation` ([48](48-pvp-ready-modular-presets.md)) — single `pushUndo` + 5 modules fail-fast.
- `ResetMapExploration` — single `pushUndo` + bulk CLEAR map flags.
- `RemoveFogOfWar` — single `pushUndo` + bulk SET FoW bitfield.

## 16. Planned / placeholder / research-only areas

| Area | Status | Location |
|---|---|---|
| PvP module E (Sites of Grace) | placeholder — warning only | `app_pvp.go:108-110`; see [48](48-pvp-ready-modular-presets.md) §7 |
| Companion flags beyond Whistle + 5 multiplayer items | research candidate (Talisman Pouch, others) | see [50](50-item-companion-flags.md) §6.3 and §14 |
| Companion flags beyond Gatefront | research candidate for the remaining ~418 graces | see [47](47-site-of-grace-activation.md) §8 |
| L2 DLC tile coords cross-version | empirical — `needs verification` after a patch | see [29](29-dlc-black-tiles.md) §13.1 |
| In-world activation per grace category | verified only Church of Elleh | see [47](47-site-of-grace-activation.md) §7 |
| Physical colosseum gates (WorldGeomMan blob) | NOT editable from a save editor | see [48](48-pvp-ready-modular-presets.md) §13 |
| WorldAreaWeather/Time write helpers | none — verbatim blob | see [19](19-weather-time.md) |
| RideGameData (Torrent horse) write helpers | none — verbatim blob | `section_world.go` |
| WorldGeomBlock direct edit | NOT done — corruption risk | see §17.1 |

## 17. Write path and rollback caveats

### 17.1 Cross-subsystem atomicity

`app_world.go` has **no** transactional mechanism for mutations spanning multiple subsystems. Each endpoint does its own `pushUndo`. Bulk endpoints (`BulkSetXxx`) do **one** snapshot for the whole list, not per-item.

Orchestrators (`RevealAllMap`, `ApplyPvPPreparation`, `ResetMapExploration`, `RemoveFogOfWar`) also have one `pushUndo` for the whole operation.

**Consequence**: if an orchestrator fails midway (e.g., module 4 in the PvP prep), the earlier mutations (modules 1–3) remain in `slot.Data`. The snapshot from `pushUndo` is available via Undo, but is **not** automatically popped.

### 17.2 WorldGeomBlock corruption risk

`WorldGeomBlock` (5 size-prefixed sections) is **always** read and written verbatim by `SizePrefixedBlob.Read/Write`. Any manual mutation:

- may corrupt the `Size` header → the parser on the next load returns `out-of-range` or a crash,
- may corrupt the internal layout (`WorldBlockChrData`, `WorldGeomDataChunk`) → NPC respawn in wrong places, missing objects, game crash on area load.

SaveForge **intentionally** does not expose an API to edit `WorldGeomBlock`. Cross-platform conversion (PC ↔ PS4) copies these blobs verbatim.

`needs verification`: whether there are saves from PS4 vs PC where the `WorldGeomBlock` layout differs binarily (other than padding). The current round-trip tests (`tests/roundtrip_test.go`) check identity round-trip, not a cross-platform diff.

### 17.3 BloodStain partial mutator caveats

`revealDLCMap` Phase 3 zeroes `[0x0085..0x0110)` in `BloodStain` and writes 9 synthetic values. This is **destructive** for an authentic DLC exploration by the player (if there was one). See [29](29-dlc-black-tiles.md) §13.2.

### 17.4 Slice invalidation

Any endpoint that does a `slot.Data` realloc (via `RebuildSlot` or `AddItemsToSlot`) must refresh `flags := slot.Data[slot.EventFlagsOffset:]` if it continues operating on the bitfield. The pattern is visible in `ApplyPvPPreparation` (lines 57, 94) and `revealDLCMap` (line 1096).

### 17.5 No per-subsystem audit log

`app_world.go` has no centralized logging of "what was changed in this operation". Individual warnings are emitted via `fmt.Printf` (some endpoints) or `runtime.LogWarningf` (write hooks). The UI in `WorldTab` caches state locally after a mutation (`setGraces(prev => ...)`, etc.) — which creates a stale-cache risk if the backend mutation partially succeeded.

## 18. Validation and safety notes

### 18.1 Stale generated data

Per-category snapshots in `backend/db/data/` (graces.go, summoning_pools.go, maps.go, regions.go, etc.) are **static**. After a DLC patch:

- new entries will not appear in the UI,
- bulk operations will not cover the new IDs,
- `IsKnownXxxID(newID)` will return `false`.

There is no automatic detection of "regulation.bin newer than snapshot". `needs verification` after every major patch.

### 18.2 Wrong flag IDs

Per-category endpoints do not validate `id` against the corresponding `data.Xxx` map — `SetGraceVisited`, `SetBossDefeated`, etc. accept any ID and delegate to `db.SetEventFlag`. A typo in the argument or a direct JS call with an arbitrary ID may SET a completely unrelated flag. See [15](15-event-flags.md) §3.

### 18.3 Read-only vs write confusion

`WorldGeomBlock` / `RideGameData` / weather / time are verbatim — no endpoint in `app_world.go` modifies them. A user may expect that some endpoint does (e.g., "reset world state" → reset all NPC) — it does not exist.

Exception: `BloodStain` has 1 partial mutator for L2 DLC tiles (§17.3).

### 18.4 Non-atomic multi-module mutations

See §17.1. Orchestrators are fail-fast without auto-restore. Partial mutation is possible.

### 18.5 Rollback / manual undo limits

All write endpoints use `a.pushUndo(slotIndex)`. The Undo stack has a depth limit (`needs verification`: the exact value in `app.go`). Bulk operations + Promise.all in the UI may create N separate snapshots, growing the stack linearly.

### 18.6 Platform / version differences

Flag/region/grace/pool IDs are `regulation.bin` snapshots. PS4 vs PC save file offsets differ — see [01-header.md](01-header.md) and [49-ps4-zstd-rawblock-patch.md](49-ps4-zstd-rawblock-patch.md). The bitfield resolver (BST) is cross-platform.

`needs verification`: whether all write endpoints from §6.3 yield identical results on PC and PS4 — the current `tests/roundtrip_test.go` covers I/O round-trip, but per-endpoint platform parity is not an isolated test.

### 18.7 In-game verification gaps

Most helpers have ad-hoc runtime verification (CHANGELOG entries), no automatic in-game test loop. Each game patch requires a manual re-test.

### 18.8 Quest / NPC progression side effects

Some flags (e.g., `4680`/`4681` Melina state, `60100` Whistle obtained, `710520` Whistle world state) are quest-NPC progression flags. Setting them from `SetGraceVisited(76111)` (Gatefront companion fan-out) or `ApplyPvPPreparation` (Colosseums 60100) **may** affect other quests / cutscenes. SaveForge **does not warn** pre-mutation.

## 19. Test coverage

| Subsystem | Test file | Coverage |
|---|---|---|
| Binary parsers | `backend/core/section_world_geom_test.go`, `section_world_test.go`, `section_trailing_test.go` | parse + round-trip |
| Event flag resolver | `tests/map_flags_test.go`, `backend/db/event_flag*_test.go` | BST + precomputed |
| Regions | `backend/core/writer_regions_test.go` (4 tests) | RebuildSlot + identity |
| Companion flags item | `tests/item_companion_flags_test.go` (17), `backend/db/data/item_companion_flags_test.go` (11) | hook SET/CLEAR + cross-contamination |
| Companion flags grace | `tests/grace_companion_flags_test.go` (3), `backend/db/data/grace_companion_flags_test.go` (3) | SET-only contract |
| PvP preset | `pvp_test.go` (10) | validation + warnings + mutation |

`needs verification`: there is no isolated cross-subsystem orchestration test (e.g., `RevealAllMap` + `SetGraceVisited` + `ApplyPvPPreparation` in sequence). No in-game runtime verification in CI.

## 20. Known limits / needs verification

A condensed list of cross-cutting open questions:

1. **Snapshot staleness** — no automatic regeneration of `data.Xxx` from `regulation.bin`.
2. **Read-only verbatim blobs** — no validation of magic bytes / structure integrity on load.
3. **WorldGeomBlock direct edit** — unreachable; physical colosseum gates etc. out of reach.
4. **Cross-platform parity** — no isolated per-endpoint tests.
5. **Patch detection** — no mechanism for "the current constants are stale".
6. **In-game verification CI** — none; each patch requires a manual re-test.
7. **Quest progression side effects pre-warning** — none; a mutation may disrupt quests.
8. **Orchestrator fail-fast auto-restore** — none; partial mutation is possible.
9. **Undo stack depth** — `needs verification` of the exact limit value.
10. **Stale cache risk in UI** — `setXxx(prev => ...)` after a backend mutation may diverge the UI from reality.
11. **`WorldGeomMan` vs `WorldGeomMan2` semantics** — "before/after" or "two layers"? unverified.
12. **RideGameData edit** — impossible; Torrent state reverts on rest.

## 21. Cross-references

- [11-regions.md](11-regions.md) — L0 UnlockedRegions; the only non-bitfield write helper.
- [14-game-state.md](14-game-state.md) — Game State (LastRestedGrace, TotalDeathsCount); a separate domain.
- [15-event-flags.md](15-event-flags.md) — generic event flag helper API; the foundation of write helpers.
- [17-player-coordinates.md](17-player-coordinates.md) — separate player coords section.
- [19-weather-time.md](19-weather-time.md) — WorldAreaWeather + WorldAreaTime (verbatim blobs).
- [27-map-reveal.md](27-map-reveal.md) — 4-layer Map Reveal.
- [29-dlc-black-tiles.md](29-dlc-black-tiles.md) — L2 DLC Cover Layer (BloodStain partial mutator).
- [47-site-of-grace-activation.md](47-site-of-grace-activation.md) — Sites of Grace.
- [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md) — `ApplyPvPPreparation` orchestrator.
- [50-item-companion-flags.md](50-item-companion-flags.md) — item lifecycle ↔ event flag bridge.

## 22. Sources

- `backend/core/section_world_geom.go` — `WorldGeomBlock` (5 blobs), `SizePrefixedBlob`, sanity ceilings.
- `backend/core/section_world.go` — `RideGameData` (40 B verbatim).
- `backend/core/section_trailing.go` — `WorldAreaWeather` + `WorldAreaTime` (12+12 B verbatim), `TrailingFixedBlock`.
- `backend/core/offset_defs.go::DynBloodStain` (`= 0x4C`), `DLCTile*` (partial mutator for L2).
- `app_world.go` — ~34 App methods (Get/Set per ~12 categories, RevealAllMap, ResetMapExploration, RemoveFogOfWar).
- `app_pvp.go` — `ApplyPvPPreparation` orchestrator.
- `frontend/src/components/WorldTab.tsx` + `PvPPreparationTab.tsx` — UI per subsystem.
- Tests: `pvp_test.go`, `tests/map_flags_test.go`, `tests/grace_companion_flags_test.go`, `tests/item_companion_flags_test.go`, `backend/core/writer_regions_test.go`, `backend/core/section_world_geom_test.go`.
