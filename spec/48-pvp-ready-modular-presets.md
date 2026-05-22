# 48 — PvP-ready Modular Presets

> **Type**: Binary format spec + design doc (canonical chapter)
> **Scope**: The current state of `ApplyPvPPreparation` in SaveForge — 5 modules (4 active + 1 placeholder), single snapshot undo, write path optionally per module, UI status in `PvPPreparationTab.tsx`.

Cross-refs: [11-regions.md](11-regions.md), [14-game-state.md](14-game-state.md), [15-event-flags.md](15-event-flags.md), [16-world-state.md](16-world-state.md), [27-map-reveal.md](27-map-reveal.md), [29-dlc-black-tiles.md](29-dlc-black-tiles.md), [47-site-of-grace-activation.md](47-site-of-grace-activation.md), [50-item-companion-flags.md](50-item-companion-flags.md).

---

## 1. Chapter purpose

To define unambiguously:

- what `ApplyPvPPreparation` (`app_pvp.go`) actually does,
- which modules are **active** (4): Matchmaking Regions, Colosseums, Reveal Map, Summoning Pools,
- which module is a **placeholder** (1): Sites of Grace — returns a warning without mutation,
- how the single-snapshot undo and error propagation work (fail-fast after the first module),
- where the implementation ends and `needs verification` begins.

It does not duplicate the event flag helper API (see [15-event-flags.md](15-event-flags.md)), the Map Reveal model (see [27-map-reveal.md](27-map-reveal.md)), regions (see [11-regions.md](11-regions.md)), graces (see [47-site-of-grace-activation.md](47-site-of-grace-activation.md)), item companion flags (see [50-item-companion-flags.md](50-item-companion-flags.md)).

## 2. Status

| Aspect | Status |
|---|---|
| Backend endpoint `ApplyPvPPreparation(slotIndex, opts)` | ✅ `app_pvp.go:25` |
| Struct `PvPPreparationOptions` (5 bool fields) | ✅ `app_pvp.go:13-19` |
| Module: Matchmaking Regions | ✅ active — `core.SetUnlockedRegions` |
| Module: Colosseums | ✅ active — `ColosseumFlagSets` + `ColosseumGlobalFlags` |
| Module: Reveal Map | ✅ active — `revealBaseMap` + `revealDLCMap` |
| Module: Summoning Pools | ✅ active — bulk `SetEventFlag` |
| Module: Sites of Grace | ❌ **placeholder** — warning only, no mutation |
| Single snapshot undo (`pushUndo`) | ✅ once at the start |
| Frontend `PvPPreparationTab.tsx` | ✅ 5 modules + 3 profiles (minimal/full/coop) + custom |
| Sites of Grace in UI | 🔒 `disabled: true` with the note "Coming soon — broad QoL module, needs UX confirmation" |
| Test coverage | ✅ 10 tests in `pvp_test.go` (4 validation + 3 warnings + 3 mutation) |

## 3. Source of truth in code

| File / symbol | What it contains |
|---|---|
| `app_pvp.go::PvPPreparationOptions` (lines 13-19) | Struct with 5 `bool` fields |
| `app_pvp.go::ApplyPvPPreparation` (lines 25-113) | Single endpoint, single `pushUndo`, sequential `if opts.X { ... }` per module |
| `app_pvp.go::revealBaseMap` / `revealDLCMap` (called from the `RevealMap` module) | See [27-map-reveal.md](27-map-reveal.md) — the same functions are used by `RevealAllMap` in `app_world.go` |
| `backend/db/data/summoning_pools.go::ColosseumFlagSet` (lines 335-367) | Struct `{Activate, MapPOI, NPC, Gate}` + `AllFlags()` returns in a stable order |
| `backend/db/data/summoning_pools.go::ColosseumFlagSets` (line 349) | 3 entries: Caelid `60350`, Limgrave `60360`, Royal `60370` |
| `backend/db/data/summoning_pools.go::ColosseumGlobalFlags` (line 357) | `[6080, 60100, 69480]` — global "any colosseum unlocked" flags |
| `backend/db/db.go::GetAllRegions` / `GetAllColosseums` / `GetAllSummoningPools` | Static DB lookups (sync.OnceValue) |
| `frontend/src/components/PvPPreparationTab.tsx` | UI: 5 modules `MODULES`, 3 profiles `PROFILE_OPTS` (minimal/full/coop), custom resolver |
| `pvp_test.go` | 10 tests: 4 validation, 3 warning, 3 mutation + roundtrip |

## 4. Mental model

```
ApplyPvPPreparation(slotIndex, opts)
  ├─ validation: save loaded, slot index, slot non-empty, EventFlagsOffset valid
  │            → all errors: return nil, error (before pushUndo)
  ├─ a.pushUndo(slotIndex)                              ← single snapshot
  ├─ flags := slot.Data[slot.EventFlagsOffset:]
  │
  ├─ if opts.MatchmakingRegions:
  │     core.SetUnlockedRegions(slot, allRegionIDs)     ← realloc slot.Data + RebuildSlot
  │     flags = slot.Data[slot.EventFlagsOffset:]       ← REFRESH
  │     warnings += "Applied %d matchmaking regions."
  │
  ├─ if opts.Colosseums:
  │     for c in GetAllColosseums():
  │       for f in ColosseumFlagSets[c.ID].AllFlags():
  │         SetEventFlag(flags, f, true)                ← error → return err
  │     for f in ColosseumGlobalFlags:
  │       SetEventFlag(flags, f, true)                  ← error → return err
  │     warnings += "Colosseum matchmaking flags set. Physical gates may still need..."
  │
  ├─ if opts.RevealMap:
  │     revealBaseMap(slot)                              ← Phase 1 flags + Phase 2 items (see 27)
  │     revealDLCMap(slot)                               ← Phase 1+2+3 (see 27/29)
  │     flags = slot.Data[slot.EventFlagsOffset:]       ← REFRESH
  │     warnings += "Map revealed (base game + DLC)."
  │
  ├─ if opts.SummoningPools:
  │     for p in GetAllSummoningPools():
  │       SetEventFlag(flags, p.ID, true)               ← error → return err
  │     warnings += "Activated %d summoning pools. Bloody Finger invasion impact is unconfirmed."
  │
  └─ if opts.SitesOfGrace:
        warnings += "Sites of Grace module is planned but not enabled in this version."
        ← NO MUTATION
```

**Fail-fast**: the first error in module 1–4 causes `return nil, err` — subsequent modules **do not execute**, but the snapshot from `pushUndo` is *not* automatically recovered. The user must press Undo manually.

## 5. Module status table

| # | Field in `opts` | Tier (UI) | Backend status | What it does | Main flags/sections |
|---|---|---|---|---|---|
| 1 | `MatchmakingRegions` | Recommended · Tier 1 | ✅ active | `core.SetUnlockedRegions` with all 104 region IDs | `slot.UnlockedRegions` (see [11](11-regions.md)) |
| 2 | `Colosseums` | Optional · Tier 1 | ✅ active | SET 12 flags (3 × 4 per-colosseum) + 3 global | event flags in the `60xxx`, `62xxx`, `69xxx`, `710xxx` bands |
| 3 | `RevealMap` | QoL · Tier 0 | ✅ active | `revealBaseMap` + `revealDLCMap` | event flags `62xxx`/`63xxx`/`82xxx` + map fragment items + DLC BloodStain (see [27](27-map-reveal.md)/[29](29-dlc-black-tiles.md)) |
| 4 | `SummoningPools` | Co-op/Summon · Tier 1 | ✅ active | SET all pool IDs (`670xxx`) | event flags `670xxx` |
| 5 | `SitesOfGrace` | QoL · Tier 0 · planned | ❌ **placeholder** | NO-OP — appendWarning only | none (see §7) |

`needs verification`: the exact number of activated pools and regions at runtime depends on the `GetAllSummoningPools`/`GetAllRegions` snapshot. See [11-regions.md](11-regions.md) for 104 regions; pools ~213 (see CHANGELOG, no isolated test of the count).

## 6. Current implemented behavior

### 6.1 Pre-mutation validation

Before `pushUndo`:

- `a.save == nil` → `"no save loaded"`.
- `slotIndex < 0 || slotIndex >= 10` → `"invalid slot index"`.
- `slot.Version == 0` → `"slot %d is empty"`.
- `slot.EventFlagsOffset <= 0 || >= len(slot.Data)` → `"event flags offset not computed for slot %d"`.

All 4 errors are returned **before** `pushUndo` — i.e., they do not create a snapshot.

### 6.2 Single snapshot undo

`a.pushUndo(slotIndex)` (line 41) creates **one** snapshot for the entire `ApplyPvPPreparation` operation. All modules mutate under this single snapshot. There are no per-module snapshots — Undo reverts **everything**, not a specific module.

### 6.3 Module 1 — Matchmaking Regions

`core.SetUnlockedRegions(slot, ids)` internally calls `RebuildSlot`, which:

- reallocates `slot.Data` (changes the size of the `UnlockedRegions` array),
- recalculates `EventFlagsOffset`,
- may change `StorageBoxOffset`, etc.

After this the `flags` slice is stale — the code **explicitly refreshes** `flags = slot.Data[slot.EventFlagsOffset:]` on line 57.

`needs verification`: whether `core.SetUnlockedRegions` returns an error in any realistic scenarios (e.g., large region lists). The test `TestApplyPvPPreparation_BadFlagsOffset` (`pvp_test.go:54`) covers the bad-offset scenario, but not the failure path from `SetUnlockedRegions`.

### 6.4 Module 2 — Colosseums

Per-colosseum:

```go
flagSet, ok := data.ColosseumFlagSets[c.ID]
if !ok {
    flagSet = data.ColosseumFlagSet{Activate: c.ID}
}
for _, id := range flagSet.AllFlags() {
    if id == 0 { continue }
    SetEventFlag(flags, id, true)
}
```

That is:

- If `c.ID` (from `GetAllColosseums`) is in `ColosseumFlagSets` → SET 4 flags (`Activate`, `MapPOI`, `NPC`, `Gate`).
- If not — fallback: SET only `Activate` (i.e., `c.ID`).
- After the per-colosseum loop: SET 3 global flags from `ColosseumGlobalFlags` (`6080`, `60100`, `69480`).

Currently `ColosseumFlagSets` has 3 entries (Caelid/Limgrave/Royal). `GetAllColosseums` returns exactly those 3 — the fallback is not actually used. `needs verification` in case of a future DLC with a new colosseum: `GetAllColosseums` must be synchronized with `ColosseumFlagSets`, otherwise only `Activate` will be SET.

**Note on `60100` — Spectral Steed Whistle flag** — see [50-item-companion-flags.md](50-item-companion-flags.md): the flag `60100` is **shared** between `ColosseumGlobalFlags` (PvP prep) and the item companion set for the Spectral Steed Whistle. Enabling the Colosseums module **sets the Torrent-unlock flag** as a side effect.

### 6.5 Module 3 — Reveal Map

Calls **the same** functions as `RevealAllMap` from `app_world.go`:

```go
revealBaseMap(slot)   // Phase 1: 4 system flags + 219 non-DLC visible + 19 base fragment items
revealDLCMap(slot)    // Phase 1: 62002+82002 + 44 DLC visible. Phase 2: 5 DLC fragment items. Phase 3: BloodStain coords (L2).
```

After both: `flags = slot.Data[slot.EventFlagsOffset:]` (line 94) because `AddItemsToSlot` (inside Phase 2) reallocates `slot.Data`.

See [27-map-reveal.md](27-map-reveal.md) §11 + [29-dlc-black-tiles.md](29-dlc-black-tiles.md) §11 for details. This chapter **does not duplicate** the 4-layer model.

`needs verification`: whether `revealBaseMap` / `revealDLCMap` in the PvP prep context have a different side-effect surface than when called by `RevealAllMap` — the code analysis indicates they do not (the same functions, the same phases), but there is no isolated test of the PvP context.

### 6.6 Module 4 — Summoning Pools

```go
pools := db.GetAllSummoningPools()
for _, p := range pools {
    SetEventFlag(flags, p.ID, true)  // error → return err
}
```

Each pool has 1 flag (its ID). No fan-out to other flags. `needs verification`: whether activating the pool flag is enough for the Martyr Effigy in the game to actually appear as a co-op summon — an ad-hoc runtime test (CHANGELOG), no CI.

**Warning literal**: "Activated %d summoning pools. **Bloody Finger invasion impact is unconfirmed.**" — explicit that we do not promise that pool activation helps PvP/Bloody Finger.

### 6.7 Module 5 — Sites of Grace (PLACEHOLDER)

```go
if opts.SitesOfGrace {
    warnings = append(warnings, "Sites of Grace module is planned but not enabled in this version.")
}
```

This is the **only** line. There is no:

- reading of `data.Graces`,
- call to `SetGraceVisited`,
- setting of any flag,
- mutation of `slot.Data`.

See §7.

## 7. Sites of Grace module E status

| Aspect | Status |
|---|---|
| Backend `app_pvp.go::ApplyPvPPreparation` | 🔒 **placeholder** — `warning` literal, no mutation |
| Frontend `PvPPreparationTab.tsx` `MODULES[4]` | 🔒 `disabled: true`, `disabledNote: "Coming soon — broad QoL module, needs UX confirmation"` |
| Standalone grace endpoints (`GetGraces`/`SetGraceVisited`) | ✅ available **independently** in `WorldTab.tsx` — see [47-site-of-grace-activation.md](47-site-of-grace-activation.md) §10.2 |
| Bulk unlock in `WorldTab` | ✅ Tier 1 `RiskActionButton riskKey="bulk_grace_unlock"` |
| Test `TestApplyPvPPreparation_SitesOfGraceWarning` | ✅ `pvp_test.go:66` — assert warning string |

**Consequence**: a user who wants to unlock all graces as part of PvP prep must **separately** use Unlock All in `WorldTab` — selecting `sitesOfGrace=true` in `ApplyPvPPreparation` is a **no-op**.

`needs verification`: the target semantics of the module (bulk unlock of all graces vs only boss arenas vs selected by region) is not documented in the code. The `disabledNote` says "needs UX confirmation".

## 8. Disabled / placeholder modules

Only `SitesOfGrace` is marked as a placeholder. The other 4 modules are active backend + UI. The UI for Sites of Grace is **rendered as a disabled checkbox** with different styling (`opacity-60`, `cursor-not-allowed`, gray `tierStyle`).

`needs verification`: whether additional modules will appear in the future (e.g., "Bosses defeated", "Item bundle") — `PvPPreparationOptions` has a fixed 5 fields, adding one requires changing the struct + UI + tests.

## 9. Relation to Event Flags

Modules 2 (Colosseums) and 4 (Summoning Pools) use **only** `db.SetEventFlag` from the generic helper API ([15-event-flags.md](15-event-flags.md)):

- Per-colosseum: 4 flags `Activate/MapPOI/NPC/Gate` + 3 global — a total of 12 + 3 = **15 flags** (for 3 colosseums).
- Per pool: 1 flag = the pool ID.

Modules 1 (Regions) and 3 (RevealMap) **also** ultimately operate on the bitfield, but through a higher API:

- Module 1 — `core.SetUnlockedRegions` mutates a **separate structure** `UnlockedRegions`, not the bitfield. See [11-regions.md](11-regions.md) §15.
- Module 3 — `revealBaseMap`/`revealDLCMap` set event flags `62xxx`/`63xxx`/`82xxx` + add items + Phase 3 BloodStain. See [27-map-reveal.md](27-map-reveal.md).

The generic helper API (`GetEventFlag`/`SetEventFlag`/BST resolver) is documented in 15 — this chapter does not duplicate it.

## 10. Relation to Map Reveal

Module 3 (`RevealMap`) calls **exactly the same** internal functions as the public endpoint `RevealAllMap` in `app_world.go`:

| Aspect | `App.RevealAllMap` (`app_world.go:1041`) | `ApplyPvPPreparation` with `RevealMap=true` |
|---|---|---|
| Call `revealBaseMap` | ✅ | ✅ |
| Call `revealDLCMap` | ✅ | ✅ |
| Phase 1 flags / Phase 2 items / Phase 3 BloodStain | ✅ identical | ✅ identical |
| `pushUndo` | ✅ once (in `RevealAllMap`) | ✅ once (in `ApplyPvPPreparation`) — a shared snapshot with the other modules |
| UI risk gate | Tier 1 `map_reveal_full` in `WorldTab` | Tier 0 in `PvPPreparationTab` (grouped with the other PvP modules) |

`needs verification`: the Tier 1 (WorldTab) vs Tier 0 (PvPPreparationTab) divergence — intentional (PvP prep has an aggregate gate), or oversight? There is no documented rationale.

Phase 3 BloodStain (L2 DLC Cover Layer) — see [29-dlc-black-tiles.md](29-dlc-black-tiles.md). PvP prep inherits all the DLC ownership / stale coords / overwritten exploration caveats.

## 11. Relation to Sites of Grace

See §7. In short:

- PvP module 5 = **placeholder**. Selecting `sitesOfGrace=true` in `opts` is a no-op + warning.
- Independent grace endpoints (`GetGraces`, `SetGraceVisited`) work **directly** in `WorldTab`. See [47-site-of-grace-activation.md](47-site-of-grace-activation.md).
- The Gatefront companion flags (`grace_companion_flags.go::GatefrontGraceEventFlagID`) are **not** touched by PvP prep.

## 12. Relation to Item Companion Flags

`ApplyPvPPreparation` **does not use** `data.CompanionEventFlagsForItem` nor the item lifecycle hooks (`AddItemsToCharacter` / `RemoveItemsFromCharacter`).

Indirectly: module 3 (`RevealMap`) adds fragment items via `core.AddItemsToSlot` (low-level), not via `App.AddItemsToCharacter` (app-level). The SET companion flag hook from `app.go:569` **does not fire** for map fragment items. See [50-item-companion-flags.md](50-item-companion-flags.md) §9.

Indirectly in another way: module 2 (`Colosseums`) sets `60100` (as part of `ColosseumGlobalFlags`). The same flag is a companion for the Spectral Steed Whistle ([50](50-item-companion-flags.md) §6.1). Selecting `Colosseums=true` sets `60100` **independently of** owning the Whistle. See §6.4.

## 13. Relation to Game / World State

`ApplyPvPPreparation` **does not modify**:

- `PreEventFlagsScalars` (`LastRestedGrace`, `TotalDeathsCount`, etc. — see [14-game-state.md](14-game-state.md)),
- `WorldGeomMan` / `WorldArea` (see [16-world-state.md](16-world-state.md)),
- `PlayerCoordinates` (see [17-player-coordinates.md](17-player-coordinates.md)),
- `WorldAreaWeather` / `WorldAreaTime` (see [19-weather-time.md](19-weather-time.md)),
- the DLC entry flag `CSDlc[1]` (`DlcSectionOffset` — see [29-dlc-black-tiles.md](29-dlc-black-tiles.md) §13.3).

Note from `app_pvp.go:82`: "Colosseum matchmaking flags set. **Physical gates may still need to be opened once in-game.**" — the `Gate` flags (710xxx) in `ColosseumFlagSet` are a *matchmaking gate marker*, **not** a physical gate. The physical gate is in the `WorldGeomMan` blob and cannot be edited from a save editor.

## 14. Write path and rollback caveats

### 14.1 Single snapshot, no per-module rollback

`pushUndo` once on line 41. All mutations of the 4 active modules share this snapshot. Popping the snapshot via Undo reverts **everything**. There is no per-module Undo (e.g., "undo only Colosseums").

### 14.2 Fail-fast without auto-restore

The first error in module 1–4 causes `return nil, err`. Subsequent modules do not execute, but **the mutations of the earlier modules remain in `slot.Data`**. The snapshot from `pushUndo` is on the Undo stack, but is **not** automatically popped. The user sees the error in the UI (toast) and must press Undo themselves.

`needs verification`: whether there is a user-facing fail-fast test — e.g., module 4 (Summoning Pools) fails and modules 1–3 are executed. The current 10 tests do not cover this scenario.

### 14.3 Slice invalidation between modules

The `flags` slice (`slot.Data[slot.EventFlagsOffset:]`) is **explicitly refreshed** in 2 places:

1. After `MatchmakingRegions` (line 57) — `core.SetUnlockedRegions` reallocates via `RebuildSlot`.
2. After `RevealMap` (line 94) — `AddItemsToSlot` (inside Phase 2) reallocates.

There is no refresh before `Colosseums` (line 61) — `Colosseums` uses `flags` derived on line 43 or refreshed on line 57. OK, because between these points there is no reallocation.

No refresh before `SummoningPools` — `flags` from the previous refresh (`RevealMap` or `MatchmakingRegions`) is current. OK.

### 14.4 Order matters

The order of modules in the code:

1. MatchmakingRegions
2. Colosseums
3. RevealMap
4. SummoningPools
5. SitesOfGrace

This order is **enforced** by the sequential `if opts.X { ... }`. The user cannot change the order via the UI. If the user wants "only RevealMap, then MatchmakingRegions" — it is not possible; either everything in the prescribed order, or manual calls separately.

`needs verification`: whether the order affects correctness. From the analysis: `MatchmakingRegions` reallocates (must be before others that use `flags`); `RevealMap` reallocates (must be before SummoningPools/SitesOfGrace). The coded sequence is correctness-wise OK.

### 14.5 No per-module transaction

`db.SetEventFlag` in modules 2 and 4 — if any flag fails, the error is propagated (`return nil, err`), **the earlier flags from the same module** are already SET. A partial effect is possible.

## 15. UI / frontend status

`frontend/src/components/PvPPreparationTab.tsx`:

| UI element | What it does |
|---|---|
| Profile picker | 3 profiles (`minimal`, `full`, `coop`) + read-only `custom` |
| `MODULES` list of 5 elements | Each with a label, tier, desc; `SitesOfGrace` has `disabled: true` |
| Per-module checkbox `<Chk>` | Toggle `pvpOpts[key]` — Sites of Grace is not clickable |
| Apply button | Calls `ApplyPvPPreparation(charIdx, payload)`, parses warnings, shows a toast |
| `NetworkSpeedPanel` | Embedded — see [44-network-param-tuning.md](44-network-param-tuning.md); a separate endpoint, **not** part of `ApplyPvPPreparation` |
| Notes panel (warnings) | Renders the warnings returned from the backend |

The 3 profiles (from `PROFILE_OPTS`):

```ts
minimal: { matchmakingRegions: true,  colosseums: false, revealMap: false, summoningPools: false, sitesOfGrace: false }
full:    { matchmakingRegions: true,  colosseums: true,  revealMap: true,  summoningPools: false, sitesOfGrace: false }
coop:    { matchmakingRegions: false, colosseums: false, revealMap: true,  summoningPools: true,  sitesOfGrace: false }
```

`sitesOfGrace` in the profiles is always `false` — no profile tries to enable the placeholder module.

`resolveProfile` (line 87) does **NOT** compare `sitesOfGrace` — it compares only the 4 active fields. This means: a profile is recognized correctly even when `sitesOfGrace=true` (but that is a no-op anyway). `needs verification`: whether this is intentional, or a bug.

`anySelected` (line 152) excludes `disabled` modules: `MODULES.some(m => !m.disabled && pvpOpts[m.key])`. The Apply button is disabled if no active module is selected.

## 16. Validation and safety notes

### 16.1 Overclaim "PvP ready"

The name "PvP-ready" suggests that after one click the character is ready for PvP. In reality:

- Matchmaking Regions: ✅ enough for basic eligibility for invasions.
- Colosseums: ✅ matchmaking flags + map markers, but **physical gates** require an in-game open.
- RevealMap: ✅ but it is QoL, not a PvP condition.
- SummoningPools: warning literal — "**Bloody Finger invasion impact is unconfirmed**".
- SitesOfGrace: placeholder.

`needs verification`: whether the current matchmaking balance requires additional conditions (Stats Level range, an item in the inventory, NG+ tier) — `ApplyPvPPreparation` does not modify any of these.

### 16.2 Quest / world progression side effects

Module 2 (Colosseums) sets 15 flags, including `60100` (the Spectral Steed Whistle obtained flag — see §12). Side effects:

- `60100` SET on a save from before the Melina encounter → Torrent may be summoned without the Whistle (a gameplay change).
- Other flags (`Gate`/`NPC`/`MapPOI`) — `needs verification` whether they set anything beyond PvP matchmaking.

Module 4 (SummoningPools) sets ~213 flags. `needs verification` whether any of them collides with other game systems.

### 16.3 Map reveal side effects

See [27-map-reveal.md](27-map-reveal.md) §13 + [29-dlc-black-tiles.md](29-dlc-black-tiles.md) §13. The main risks:

- DLC ownership mismatch (`CSDlc` not checked),
- Phase 3 BloodStain coords may overwrite authentic exploration,
- visual reveal vs gameplay progression (trophies — `needs verification`).

### 16.4 Item companion flags

See §12. In short: `ApplyPvPPreparation` may SET flags from item companion sets (Whistle 60100) **without** the corresponding item in the inventory, if module 2 (Colosseums) is active. This is intentional (Colosseums need 60100 as a global flag), but creates the "flag without an item" edge case described in [50-item-companion-flags.md](50-item-companion-flags.md) §12.4.

### 16.5 Sites of Grace placeholder

A user clicking "Sites of Grace" in the UI will get a warning after Apply — but the checkbox is `disabled`, so in reality it cannot be clicked. **The backend, however, accepts** `opts.SitesOfGrace=true` (e.g., from test code / a direct JS call) and returns the warning. `needs verification`: whether there are external calls that pass `sitesOfGrace=true` with an expectation of mutation.

### 16.6 Rollback / atomicity gaps

See §14.1, §14.2, §14.5. The main ones:

- No per-module Undo.
- No auto-restore after fail-fast.
- No transaction inside a module (partial flag SET on error).

### 16.7 In-game verification gaps

The 10 tests in `pvp_test.go` cover:

- 4 validation paths (no save, invalid slot, empty slot, bad offset),
- 3 warnings (Sites of Grace, Colosseum, Summoning Pools — assert string),
- 3 mutation paths (Colosseums mutate, Summoning Pools mutate, EventFlag roundtrip).

**It does not cover**:

- in-game runtime verification (whether the Bloody Finger actually appears to an invader),
- fail-fast cleanup (partial state in `slot.Data` after an error),
- cross-module ordering edge cases,
- the PvP module RevealMap (comment in `pvp_test.go:14-15`: "revealBaseMap/revealDLCMap will fail on minimal save — those are tested via roundtrip/integration tests").

### 16.8 Platform / version differences

The region, pool, colosseum, grace IDs are snapshots from `regulation.bin`. After a game patch:

- new regions/pools/colosseums may not be in the DB (the same problem as in [11-regions.md](11-regions.md), [47-site-of-grace-activation.md](47-site-of-grace-activation.md)),
- the semantics of the flags may change (rarely, but possible for world state flags).

`needs verification`: there is no automatic detection of "regulation.bin newer than the data snapshot".

## 17. Test coverage

10 tests in `pvp_test.go`:

| Test | Line | What it verifies |
|---|---|---|
| `TestApplyPvPPreparation_NoSave` | 27 | `a.save == nil` → error "no save loaded" |
| `TestApplyPvPPreparation_InvalidSlotIndex` | 35 | `slotIndex < 0 / >= 10` → error |
| `TestApplyPvPPreparation_EmptySlot` | 45 | `slot.Version == 0` → error |
| `TestApplyPvPPreparation_BadFlagsOffset` | 54 | `EventFlagsOffset` out of range → error |
| `TestApplyPvPPreparation_SitesOfGraceWarning` | 66 | `opts.SitesOfGrace=true` → warning literal "planned but not enabled" |
| `TestApplyPvPPreparation_ColosseumWarning` | 77 | `opts.Colosseums=true` → warning about physical gates |
| `TestApplyPvPPreparation_SummoningPoolsWarning` | 94 | `opts.SummoningPools=true` → warning Bloody Finger unconfirmed |
| `TestApplyPvPPreparation_ColosseumsMutate` | 113 | 12+3 colosseum flags SET in the bitfield |
| `TestApplyPvPPreparation_SummoningPoolsMutate` | 137 | All pool IDs SET |
| `TestApplyPvPPreparation_EventFlagRoundtrip` | 166 | SET + readback via `db.GetEventFlag` |

Comment in `pvp_test.go:11-16`: a minimal save with a 20 KB EventFlags region; it does not initialize dynamic offsets — the modules `MatchmakingRegions` (`core.SetUnlockedRegions`) and `RevealMap` (`revealBaseMap/revealDLCMap`) **fail** on a minimal save and are not tested here. They are covered by "roundtrip/integration tests that load real save files".

`needs verification`: the list of "integration tests that load real save files" for PvP prep — whether there is a specific test calling `ApplyPvPPreparation` with `opts.RevealMap=true` on a real slot. Not found in `tests/`.

## 18. Known limits / needs verification

1. **Sites of Grace target semantics** — placeholder, no documented target.
2. **`ColosseumFlagSets` extensibility** — a new DLC colosseum requires manual synchronization of `ColosseumFlagSets` with `GetAllColosseums`.
3. **60100 cross-contamination** — a flag shared by Colosseums ↔ Whistle, intentional? `needs verification`.
4. **Tier mismatch RevealMap** — Tier 1 in `WorldTab`, Tier 0 in `PvPPreparationTab`. Intentional?
5. **Fail-fast auto-restore** — none; the mutations of earlier modules remain.
6. **Per-module Undo** — none; only bulk Undo.
7. **Partial mutation after an error in the loop** — possible in modules 2 and 4.
8. **`resolveProfile` ignores `sitesOfGrace`** — intentional or a bug.
9. **Integration test `RevealMap` in the PvP prep context** — no isolated test.
10. **The number of regions / pools stale after a patch** — no detection.
11. **The backend accepts `sitesOfGrace=true` despite the UI being disabled** — a direct JS call can bypass the UI gating.
12. **Physical colosseum gates in WorldGeomMan** — unreachable via the save editor (`needs verification` whether any blob editing is planned).
13. **In-game verification of PvP matchmaking** — manual, no CI.

## 19. Cross-references

- [11-regions.md](11-regions.md) — Module 1 (Matchmaking Regions) → `core.SetUnlockedRegions`, 104 region IDs.
- [14-game-state.md](14-game-state.md) — `ApplyPvPPreparation` untouched.
- [15-event-flags.md](15-event-flags.md) — the generic helper API used by modules 2 and 4.
- [16-world-state.md](16-world-state.md) — `WorldGeomMan` blob untouched (physical colosseum gates).
- [27-map-reveal.md](27-map-reveal.md) — Module 3 (RevealMap) → `revealBaseMap` + `revealDLCMap`.
- [29-dlc-black-tiles.md](29-dlc-black-tiles.md) — L2 DLC Cover Layer; inherited caveats.
- [47-site-of-grace-activation.md](47-site-of-grace-activation.md) — standalone grace endpoints; PvP module 5 placeholder.
- [50-item-companion-flags.md](50-item-companion-flags.md) — `60100` shared with the Whistle companion set.

## 20. Sources

- `app_pvp.go::ApplyPvPPreparation` (lines 25-113) — single endpoint.
- `app_pvp.go::PvPPreparationOptions` (lines 13-19) — 5 bool fields.
- `backend/db/data/summoning_pools.go::ColosseumFlagSets` / `ColosseumGlobalFlags` / `ColosseumFlagSet.AllFlags()` (lines 335-367).
- `backend/db/db.go::GetAllRegions` / `GetAllColosseums` / `GetAllSummoningPools` (sync.OnceValue cache).
- `frontend/src/components/PvPPreparationTab.tsx` — UI: 5 modules `MODULES`, 3 profiles `PROFILE_OPTS`, `resolveProfile`, `anySelected`, `NetworkSpeedPanel` embed.
- `pvp_test.go` — 10 tests (4 validation + 3 warnings + 3 mutation/roundtrip).
- `app_world.go::revealBaseMap` / `revealDLCMap` — shared with `RevealAllMap`.
- `docs/CHANGELOG.md` — historical ad-hoc runtime PvP verification (Steam Deck, PS4).
