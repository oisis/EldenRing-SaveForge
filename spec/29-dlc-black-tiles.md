# 29 — DLC Black Tiles (DLC map cover layer)

> **Type**: Binary format spec + design doc (detail chapter)
> **Scope**: Layer L2 of the Map Reveal model — the black tiles covering the "Shadow of the Erdtree" DLC map, and the synthetic BloodStain coordinates that the editor writes in order to lift this layer.

This chapter is a **detail chapter** for layer L2 from [27-map-reveal.md](27-map-reveal.md). It does not duplicate the master 4-layer model — it describes only:

- where the DLC black tiles come from,
- which data in `BloodStain` controls this layer,
- which constants and values the current code writes (`app_world.go::revealDLCMap`),
- what pitfalls/risks are associated with this solution.

Cross-refs: [11-regions.md](11-regions.md) (L0 — UnlockedRegions), [15-event-flags.md](15-event-flags.md) (event flag helper API), [27-map-reveal.md](27-map-reveal.md) (master 4-layer model), [50-item-companion-flags.md](50-item-companion-flags.md) (item companion flag policy for Map Fragment items).

---

## 1. Chapter purpose

Unlike the base game, the DLC map covers the entire area by default with a **hard blackout** — full black tiles that are not lifted by:

- event flags `62080..62084` or `628xx..629xx` (visible flags, layer L1),
- the DLC Map Fragment items (`0x401EA618..0x401EA61C`),
- the Fog of War bitfield (layer L3, a separate system — see §17/27),
- the system flags `62002` / `82002`.

This layer physically exists above layer L1 and requires writing "visited" coordinates into the `BloodStain` section so that the game renders the DLC map as revealed. This is the L2 of the Map Reveal model (27 §4 / §17.2).

`needs verification`: the negative list above reflects the research results in `docs/CHANGELOG.md` (branch `fix/dlc-map-reveal-v2`). The result was confirmed in-game by the author; it is not covered by an automatic test.

## 2. Status

| Aspect | Status |
|---|---|
| Implementation in the editor | ✅ `app_world.go::revealDLCMap` Phase 3 |
| Offset constants | ✅ `backend/core/offset_defs.go` (`DLCTile*`) |
| Required user action | Tier 1 risk gate (`RiskActionButton riskKey="map_reveal_full"`) |
| Automatic test coverage | ❌ no dedicated DLCTile test (`needs verification` after game patches) |
| In-game verification | ✅ historical confirmation (CHANGELOG, branch SOLVED) |
| Stability after patches | ❓ `needs verification` — the values are empirical, not game-guaranteed |

## 3. Source of truth in code

| File | What it contains |
|---|---|
| `backend/core/offset_defs.go` lines 303-323 | Constants `DLCTileZeroStart/End`, `DLCTileRec1{X,Y,Flag}`, `DLCTileRec2{X,Y,Z,W,Flag}` |
| `app_world.go::revealDLCMap` (Phase 3) | The procedure that writes the synthetic coordinates |
| `app_world.go::resolveAfterRegs` | Determines `afterRegs` (end of UnlockedRegions) — the relative base of the offsets |
| `app_world.go::RevealAllMap` | Public API; calls `revealBaseMap` + `revealDLCMap` |
| `app_pvp.go::PrepPvP` (`opts.RevealMap`) | Second caller for `revealDLCMap` |
| `backend/db/data/maps.go::IsDLCMapFlag` | Defines which visible flags are DLC (`62080..62084 || 62800..62999`) |
| `backend/db/data/maps.go::MapFragmentItems` | 5 DLC fragments (`62080..62084` → `0x401EA618..0x401EA61C`) |

Document 29 is a complement to 27 — it does not duplicate `RevealAllMap` nor the Phase 1/Phase 2 logic, it focuses only on Phase 3.

## 4. Mental model

The DLC map in the game has three rendering "filters" that must be lifted in parallel:

```
┌──────────────────────────────────────────────┐
│  L3  Fog of War overlay (afterRegs+0x087E..) │  ← separate endpoint RemoveFogOfWar
├──────────────────────────────────────────────┤
│  L2  DLC Cover Layer  (black tiles)          │  ← this chapter: BloodStain coords
├──────────────────────────────────────────────┤
│  L1  Detailed bitmap (event flags 62xxx +    │  ← revealDLCMap Phase 1+2
│      Map Fragment items)                      │
├──────────────────────────────────────────────┤
│  L0  UnlockedRegions ([]u32)                  │  ← spec/11
└──────────────────────────────────────────────┘
```

L2 is driven by `BloodStain` data, not event flags. It is a single layer characteristic of the DLC — in the base game the Cover Layer is transparent by default (this chapter does not modify base game data).

`needs verification`: the assumption "the base game Cover Layer is transparent by default" comes from an empirical comparison of fresh base game saves with DLC saves — not from reverse engineering of the game mechanics.

## 5. Relation to Map Reveal L2

| Aspect | `27-map-reveal.md` (master) | `29` (this chapter) |
|---|---|---|
| 4-layer overview | ✅ master | links |
| L1 event flags 62xxx semantics | ✅ master | links |
| L1 Map Fragment items | ✅ master | links (5 DLC) |
| L2 — existence of the layer | ✅ master | ✅ develops the mechanics |
| L2 — `DLCTile*` constants | ❌ (links) | ✅ hex details |
| L2 — synthetic coordinates | ❌ (links) | ✅ values and procedure |
| L3 FoW | ✅ master | links |

Rule: 27 = what happens layer by layer; 29 = how the code concretely implements layer L2.

## 6. DLC tile data model

Layer L2 is driven by **two position records** located inside the `BloodStain` section in the data area between the end of `UnlockedRegions` and the start of the Fog of War bitfield.

```
afterRegs + 0x0000  ← end of UnlockedRegions (size depends on the slot)
afterRegs + 0x0075  ← start of BloodStain
┃
┣━ afterRegs + 0x0088..0x0110  ← DLC position data field range (136 B)
┃    ┃
┃    ┣━ Rec1 (DLC map center):    2× f32 + 1× u8
┃    ┗━ Rec2 (DLC area anchor):   4× f32 + 1× u8
┃
afterRegs + 0x087E  ← start of the FoW bitfield (L3, separate)
afterRegs + 0x10B0  ← end of the FoW bitfield
afterRegs + 0x10B1  ← start of menuProfile
```

`afterRegs` is determined by `resolveAfterRegs(slot)` in `app_world.go`:

```
storageEnd  = StorageBoxOffset + core.DynStorageBox
gesturesOff = storageEnd + core.DynStorageToGestures
regCount    = u32 LE @ gesturesOff
afterRegs   = gesturesOff + 4 + regCount * 4
```

That is — any length mutation of `UnlockedRegions` (`SetUnlockedRegions` / `RebuildSlot`) also shifts `BloodStain` and thereby **all** DLCTile offsets. The code always resolves `afterRegs` before writing.

## 7. DLC tile constants and synthetic coordinates

### 7.1 Offset constants

From `backend/core/offset_defs.go:307-323` (offsets relative to `afterRegs`):

| Constant | Offset | Type | Code comment |
|---|---|---|---|
| `DLCTileZeroStart` | `0x0088` | — | start of range to zero out before writing coords |
| `DLCTileZeroEnd`   | `0x0110` | — | end of range (exclusive) |
| `DLCTileRec1X`     | `0x008D` | f32 | X coordinate |
| `DLCTileRec1Y`     | `0x0091` | f32 | Y coordinate |
| `DLCTileRec1Flag`  | `0x0095` | u8  | visited flag |
| `DLCTileRec2X`     | `0x00C5` | f32 | X |
| `DLCTileRec2Y`     | `0x00C9` | f32 | Y |
| `DLCTileRec2Z`     | `0x00CD` | f32 | Z |
| `DLCTileRec2W`     | `0x00D1` | f32 | W |
| `DLCTileRec2Flag`  | `0x00D5` | u8  | visited flag |

### 7.2 Synthetic values

From `app_world.go::revealDLCMap` Phase 3:

| Field | Value | Origin |
|---|---|---|
| `Rec1.X` | `9648.0` (f32) | empirical — corresponds to the DLC map center in slot 0/1 of a reference save |
| `Rec1.Y` | `9124.0` (f32) | same |
| `Rec1.Flag` | `0x01` (u8) | "visited" marker (Rec1) |
| `Rec2.X` | `3037.0` (f32) | empirical — DLC area anchor |
| `Rec2.Y` | `1869.0` (f32) | same |
| `Rec2.Z` | `7880.0` (f32) | same |
| `Rec2.W` | `7803.0` (f32) | same |
| `Rec2.Flag` | `0x01` (u8) | "visited" marker (Rec2) |

`needs verification` — the values come from two reference slots (slot 0 and slot 1) with the DLC completed; the CHANGELOG notes the observation "slot 0 and slot 1 have identical values in this range". The nature of the coordinates (whether it is the last Bloodstain? a respawn point? a discovery anchor?) **remains unverified**. The values are NOT game-guaranteed — a future game patch may change them.

`needs verification` — whether the game accepts other arbitrary coordinates from the DLC area, or requires specifically these values, **has not been resolved**. The CHANGELOG flags this as an open question.

### 7.3 Endianness and layout

The f32 fields are written by `putF32` in **little-endian** (`binary.LittleEndian.PutUint32` + `math.Float32bits`). The flag byte `0x01` is a literal write. There is no variable length — the entire `[0x0088..0x0110)` is 136 bytes of fixed layout.

## 8. DLC event flags

L2 is NOT driven by event flags. The DLC 62xxx event flags (`62080..62084` + `62800..62999`) control layer **L1** (detailed bitmap), not L2. `IsDLCMapFlag` (maps.go:378) is a classifying predicate for which branch (`revealBaseMap` vs `revealDLCMap`) the flags belong to — it does not represent the physical presence of a tile.

This document intentionally does NOT duplicate the semantics of the helper API `db.GetEventFlag` / `db.SetEventFlag` — see [15-event-flags.md](15-event-flags.md).

`needs verification`: the range `62800..62999` in `IsDLCMapFlag` covers 200 IDs; in `MapVisible` there really exist ~39 DLC sub-regions (`62800..62999`) + 5 region anchors (`62080..62084`) = 44 DLC visible flags. The remaining IDs in the `62800..62999` band are treated as DLC if they appear in `MapVisible` in the future — the code does not assume density.

## 9. 62002 / 82002 divergence

`revealDLCMap` Phase 1 sets inline (`app_world.go:1099-1100`):

```go
_ = db.SetEventFlag(flags, 62002, true) // Allow Shadow Realm Map Display
_ = db.SetEventFlag(flags, 82002, true) // Show Shadow Realm Map
```

Meanwhile `MapSystem` (`backend/db/data/maps.go:10`) contains **only** `62000`, `62001`, `82001`, `82002` — `62002` is not in `MapSystem`. This means that:

- `82002` is both in `MapSystem` (set by `revealBaseMap` in the loop) and set inline in `revealDLCMap` — effectively set twice. Idempotent, no side effects.
- `62002` is set **only** inline in `revealDLCMap` and nowhere else.

The reason for this split (whether `62002` is intentionally outside `MapSystem` because it is a "DLC-only allow", or whether it is a historically overlooked omission) — **is not documented in the code**. The master analysis of this divergence is in [27-map-reveal.md](27-map-reveal.md) §9; here we only state the fact that Phase 3 (L2) does not depend on either of these flags.

`needs verification` — whether setting `62002`+`82002` without Phase 3 is enough to lift the hard blackout. The results in the CHANGELOG suggest "no" (62xxx event flags as a category were tested and do not remove the tiles), but there is no isolated test of "set only 62002+82002, leave Phase 3 untouched, check in-game".

## 10. DLC ownership and map fragments

`revealDLCMap` Phase 2 adds up to 5 DLC Map Fragment items (`62080..62084` → `0x401EA618..0x401EA61C` from `MapFragmentItems`) to the inventory. Each item is added via `core.AddItemsToSlot` with `quantity=1, durability=0`.

**What the code does NOT do**:

- it does not check whether the player owns the DLC (no DLC ownership gate on the save side — see `backend/core/offset_defs.go:325-335` about `DlcSectionOffset` / `DlcEntryFlagByte`; this section is not consulted by `revealDLCMap`),
- it does not check whether the DLC item already exists (item companion flag policy looks at the pickup flag — see [50-item-companion-flags.md](50-item-companion-flags.md)),
- it does not validate whether the synthetic coordinates make sense for the game version the save is launched on.

`needs verification` — whether an attempt to write a DLC reveal in a save of a player who does **not** have the DLC causes a crash, a soft-lock loading, or a graceful no-op. The historical CHANGELOG note about `DlcEntryFlagByte = 1` warns that "non-zero = entered DLC; causes infinite loading without DLC" — this concerns `CSDlc[1]`, is NOT touched by `revealDLCMap`, but the cross-section risk has not been isolated by a test.

Companion flag policy for DLC Map Fragment items (whether `AddItemsToSlot` sets the pickup flag, or leaves the decision to the game) — see [50-item-companion-flags.md](50-item-companion-flags.md). This chapter does not duplicate that policy.

## 11. Current implemented behavior

`app_world.go::revealDLCMap` performs three phases in a fixed order:

| Phase | Action | Reason for the order |
|---|---|---|
| Phase 1 | Inline `SetEventFlag(62002, true)` + `SetEventFlag(82002, true)`. Then a loop over `MapVisible`: if `IsDLCMapFlag(id)`, set the event flag + collect the corresponding item from `MapFragmentItems`. | Event flags are set BEFORE `AddItemsToSlot`, because adding items shifts `slot.Data` and invalidates `flags := slot.Data[slot.EventFlagsOffset:]`. |
| Phase 2 | For each collected DLC fragment: `core.AddItemsToSlot(slot, []uint32{itemID}, 1, 0, false)`. | Performed BEFORE Phase 3, because `AddItemsToSlot` shifts `slot.Data` again and invalidates the `afterRegs` computed earlier. |
| Phase 3 | `resolveAfterRegs(slot)` → zero `[0x0088..0x0110)` → write Rec1/Rec2 + flags. | Performed AFTER Phase 2, so that `afterRegs` is current after the shifts. |

Phase 3 is the **sole writer** of data in `[0x0088..0x0110)` from `revealDLCMap`. The `ResetMapExploration` endpoint (`app_world.go:1162`) **does not revert** Phase 3 — it clears only the `MapVisible` / `MapAcquired` / `MapUnsafe` event flags. Reverting the synthetic coordinates requires `Undo` (a pre-reveal snapshot) or manual restoration.

Callers:

- `app_world.go::RevealAllMap(slotIndex)` — public Wails endpoint, combines `revealBaseMap` + `revealDLCMap`.
- `app_pvp.go::PrepPvP(opts.RevealMap)` — a module in the PvP prep flow.

UI:

- `frontend/src/components/WorldTab.tsx:219` — `handleRevealAllMap` calls `RevealAllMap` + `RemoveFogOfWar` in one confirmation; protected by `RiskActionButton riskKey="map_reveal_full"` (Tier 1 risk).

`needs verification` — whether `handleRevealAllMap` in the UI actually caches the state as "everything revealed" without a round-trip through `GetMapProgress`. In the code line 219 sets `setMapEntries(prev => prev.map(e => ({...e, enabled: true})))` locally — if `revealDLCMap` failed for some flags for any reason, the UI will show a state diverging from reality until the next `GetMapProgress`. Master note on the cache stale risk: [15-event-flags.md](15-event-flags.md) §L10.

## 12. Write path and rollback caveats

### 12.1 Atomicity

`RevealAllMap` / `PrepPvP(RevealMap)` are **not atomic per layer**. The sequence `revealBaseMap` → `revealDLCMap` → (Phase 1 flags → Phase 2 items → Phase 3 BloodStain) contains 3 `slot.Data` mutation points. If an error occurs in any of the phases, there is no per-phase rollback — the changes made so far remain in `slot.Data`. Test coverage of this missing path → see §14.

### 12.2 Snapshot undo (save-level)

`RevealAllMap` calls `a.pushUndo(slotIndex)` BEFORE `revealBaseMap`. This is a snapshot of the entire slot — `Undo` will restore the state before Phase 1, including the raw BloodStain data. This is the **only** rollback path for Phase 3.

`PrepPvP` (`app_pvp.go:41`) calls `a.pushUndo(slotIndex)` once at the start, by the same mechanism — Phase 3 is subject to the same single-snapshot rollback (see [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md)).

### 12.3 No per-slot "already revealed" detection

`revealDLCMap` does not detect whether `[0x0088..0x0110)` already contains synthetic coordinates. Every call overwrites with a full zeroing + a write of the constant values. In practice idempotent, but **destructive** for any authentic BloodStain data from a real DLC exploration by the player — see §13.

### 12.4 Refresh of `flags` after Phase 2

Inside `revealDLCMap` (`app_world.go:1096`) `flags` is derived once on function entry and used only in Phase 1. Phase 2 mutates `slot.Data` via `AddItemsToSlot` (the `flags` slice becomes stale), but Phase 3 does not reference `flags` — it operates on `slot.Data` after a fresh `resolveAfterRegs`, so a refresh inside `revealDLCMap` is not needed.

A caller that wants to use `flags` after `revealDLCMap` must do the refresh itself — an example is in `app_pvp.go:94` (`flags = slot.Data[slot.EventFlagsOffset:]` after `revealBaseMap`+`revealDLCMap`, before the `SummoningPools` / `Colosseum` modules). `RevealAllMap` does not need a refresh, because it does not use `flags` after delegation.

## 13. Validation and safety notes

### 13.1 Risk of stale coordinates after a patch

The values `9648/9124` and `3037/1869/7880/7803` are empirical — copied from two reference slots. Nothing in the code or in `regulation.bin` validates them. If a future game patch changes the DLC map geometry or the logic of interpreting the BloodStain coordinates, Phase 3 may:

- stop removing the black tiles (no-op behavior),
- place the player in an incorrect location if the coords are also treated as the last-bloodstain respawn position,
- cause visual artifacts (partial reveal).

Mitigation: mark the `DLCTile*` constants as game-version dependent with patch detection (there is no such detection in the current code). `needs verification`.

### 13.2 Risk of "overwritten real exploration"

If the player has a save with the DLC **partially** explored (authentic exploration), Phase 3 zeroes `[0x0085..0x0110)` (the range from `DLCTileZeroStart=0x0088` upward) and overwrites with the synthetic values from reference slot 0/1. The original position of the last Bloodstain / the exploration anchor is **lost without warning**.

`needs verification` — whether the overwritten BloodStain section has effects other than visual (e.g., respawn point, last-grace anchor). The CHANGELOG records the hypothesis "last bloodstain? respawn point? map discovery anchor?" as unresolved.

### 13.3 Risk of DLC ownership mismatch

See §10. The editor writes the DLC reveal without checking `CSDlc` (`DlcSectionOffset = SlotSize - 0xB2`). A player who does not have the DLC will get a save with DLC flags set + 5 DLC items + a synthesized BloodStain. The impact on launching the game without DLC — **untested**.

### 13.4 Risk of platform/version differences

The `BloodStain` layout (and thus the `DLCTile*` offsets) is assumed identical for:

- the PC and PS4 versions of the save (different magic bytes, but the same slot layout — see [01-header.md](01-header.md) and [49-ps4-zstd-rawblock-patch.md](49-ps4-zstd-rawblock-patch.md)),
- different versions of `regulation.bin`.

`needs verification` — there is no isolated test `TestDLCTileLayoutPS4` nor `TestDLCTileLayoutAcrossVersions`.

### 13.5 Visual reveal vs gameplay progression

Phase 3 lifts the **visual** layer L2 (black tiles) without affecting DLC gameplay progress:

- it does not set DLC quest progress flags,
- it does not unlock DLC Sites of Grace (see [47-site-of-grace-activation.md](47-site-of-grace-activation.md)),
- it does not modify `UnlockedRegions` for DLC regions (layer L0 — see [11-regions.md](11-regions.md)).

`needs verification` — whether, after lifting L2, the game treats the DLC region as "visited" in the context of trophies / achievements. The default assumption: no, because trophies usually rely on separate progress flags, but there is no isolated test.

### 13.6 No in-game verification in CI

Phase 3 was verified in-game manually (the author + branch SOLVED). There is no automatic verification of "editor save → load in-game → DLC map screenshot → diff" in the tests. Every nontrivial patch of this flow requires a manual in-game re-test.

## 14. Test coverage

| Test | File | What it covers |
|---|---|---|
| `TestBSTLookupMatchesEventFlags` | `tests/map_flags_test.go:12` | Indirectly touches DLC visible flags (`62080..62084`); verifies the BST lookup, NOT Phase 3 BloodStain |
| `TestGetAllMapEntries` | `tests/map_flags_test.go:53` | Counts map entries; nothing about DLCTile |
| `TestMapFlagsRoundtrip` | `tests/map_flags_test.go:96` | Roundtrip set/get of a single flag; nothing about BloodStain |
| (`pvp_test.go`) | `pvp_test.go` | Comment explicitly: "revealBaseMap/revealDLCMap (RevealMap) will fail — those are tested via …" → indicates the LACK of coverage in pvp_test |

**Conclusion**: there is no isolated unit/integration test for `revealDLCMap` Phase 3. There is no test that would load a reference save, call `revealDLCMap`, and verify that the bytes in `[0x0088..0x0110)` match the specification.

`needs verification`: adding a test of the kind `TestRevealDLCMapPhase3WritesExpectedBytes` (load save → `revealDLCMap` → assert on the `DLCTileRec1/Rec2` bytes after `afterRegs`) would be desirable, but does not exist on the `main` branch.

## 15. Known limits / needs verification

A list of open questions (condensed from CHANGELOG + own analysis):

1. **Nature of the coordinates** — do `9648/9124` and `3037/1869/7880/7803` represent the last-bloodstain, a respawn anchor, or a "discovery anchor"? `needs verification`.
2. **Value tolerance** — does the game accept any coords from the DLC area, or does it require specifically these? `needs verification`.
3. **Stability after a game patch** — no gameVersion-gating. `needs verification`.
4. **DLC ownership without DLC installed** — `needs verification` of how the game reacts to `revealDLCMap` in the save of a player without DLC.
5. **Cross-platform layout** — `needs verification` whether the `DLCTile*` offsets are identical in PS4 and PC slots.
6. **DLC partially explored by the player** — Phase 3 overwrites authentic BloodStain data. `needs verification` whether the original Bloodstain location has effects beyond the visual.
7. **62002 isolated effect** — `needs verification` whether setting only `62002`+`82002` (without Phase 3) removes the hard blackout. Historically ❌, but no isolated test.
8. **Trophies / achievements** — `needs verification` whether lifting L2 affects DLC map trophy progress.
9. **Per-layer rollback** — there is no per-Phase atomic rollback; the only revert path is a snapshot-undo of the whole slot. `as designed`, but worth documenting.
10. **Automatic patch detection** — the code has no mechanism for "the current constants are stale". `needs verification` after every major game patch.

## 16. Cross-references

- [11-regions.md](11-regions.md) — L0 UnlockedRegions; the `afterRegs` base depends on the length of this array.
- [15-event-flags.md](15-event-flags.md) — generic event flag helper API; `IsDLCMapFlag`, `SetEventFlag` semantics.
- [27-map-reveal.md](27-map-reveal.md) — master 4-layer Map Reveal; this chapter is the detail chapter for L2.
- [47-site-of-grace-activation.md](47-site-of-grace-activation.md) — DLC graces; **not** set by `revealDLCMap`.
- [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md) — the second caller of `revealDLCMap` via `opts.RevealMap`.
- [50-item-companion-flags.md](50-item-companion-flags.md) — companion flag policy for Map Fragment items.

## 17. Sources

- `app_world.go` — `RevealAllMap`, `revealBaseMap`, `revealDLCMap`, `resolveAfterRegs`, `putF32`, `ResetMapExploration`.
- `app_pvp.go` — `PrepPvP` (`opts.RevealMap`).
- `backend/core/offset_defs.go:303-323` — constants `DLCTile*`, `DlcSectionOffset`, `DlcEntryFlagByte`.
- `backend/db/data/maps.go` — `MapSystem`, `MapVisible`, `MapFragmentItems`, `IsDLCMapFlag`.
- `frontend/src/components/WorldTab.tsx` — `handleRevealAllMap`, `RiskActionButton riskKey="map_reveal_full"`.
- `tests/map_flags_test.go` — event flag tests (NOT Phase 3).
- `docs/CHANGELOG.md` — entry "Branch: fix/dlc-map-reveal-v2 — DLC black tile removal (SOLVED)" and the binary search history (2025-04-25).
