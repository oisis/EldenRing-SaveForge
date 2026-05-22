# 47 — Site of Grace Activation

> **Type**: Binary format spec + design doc (canonical chapter)
> **Scope**: Site of Grace activation in SaveForge — data model, the SET-only contract for companion flags, the write path, PvP module status, relations to event flags / map / world / game state.

Cross-refs: [11-regions.md](11-regions.md), [14-game-state.md](14-game-state.md), [15-event-flags.md](15-event-flags.md), [16-world-state.md](16-world-state.md), [27-map-reveal.md](27-map-reveal.md), [29-dlc-black-tiles.md](29-dlc-black-tiles.md), [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md), [50-item-companion-flags.md](50-item-companion-flags.md).

---

## 1. Chapter purpose

To define unambiguously what the editor **actually** does on Site of Grace activation:

- which bit is set for the grace itself,
- which door flag for a dungeon (Cat/HG),
- which companion flags are set (SET-only) and why they are not cleared,
- what the editor does **not** touch (`LastRestedGrace`, MapFlags, BonfireId),
- what the status of the "Sites of Grace" module in PvP prep is,
- where the implementation ends and `needs verification` begins.

It does not duplicate the event flag helper API (see [15-event-flags.md](15-event-flags.md)) nor the item companion flags semantics (see [50-item-companion-flags.md](50-item-companion-flags.md)).

## 2. Status

| Aspect | Status |
|---|---|
| Backend endpoint `SetGraceVisited` | ✅ `app_world.go:43` |
| Backend endpoint `GetGraces` | ✅ `app_world.go:14` |
| Static DB | ✅ `backend/db/data/graces.go` — 419 entries (snapshot from `data.Graces`) |
| Companion flags map | ✅ `backend/db/data/grace_companion_flags.go` — 1 entry (Gatefront 76111) |
| `IsKnownGraceID` helper | ✅ `backend/db/db.go:1126` |
| UI `WorldTab` Sites of Grace | ✅ per-grace + per-region + Unlock All / Lock All |
| Tier risk gate `bulk_grace_unlock` | ✅ Tier 1 (`RiskActionButton`) |
| Companion flag policy SET-only | ✅ enforced in `app_world.go:73-81`, covered by a test (`tests/grace_companion_flags_test.go`) |
| PvP preset module `SitesOfGrace` | ❌ placeholder — returns the warning "planned but not enabled in this version" (`app_pvp.go:108-110`) |
| In-world activation sequence (cutscene/animation) | ⚠️ `needs verification` per-category (Church of Elleh overworld — tested, other categories — not) |

## 3. Source of truth in code

| File / symbol | What it contains |
|---|---|
| `app_world.go::GetGraces` | Read visited per-grace via `db.GetEventFlag` |
| `app_world.go::SetGraceVisited` | Write path: pushUndo + grace flag + door flag + companion flags |
| `backend/db/data/graces.go::Graces` | 419 entries `GraceData{Name, DungeonType, DoorFlag, BossArena}` |
| `backend/db/data/grace_companion_flags.go::graceCompanionEventFlags` | Map `graceID → []companionFlag` (SET-only) |
| `backend/db/data/grace_companion_flags.go::CompanionEventFlagsForGrace` | Lookup API |
| `backend/db/data/grace_companion_flags.go::GatefrontGraceEventFlagID` | Constant `0x0001294F` (= 76111) |
| `backend/db/db.go::GraceEntry` | Public struct `{ID, Name, Region, Visited, IsBossArena, DungeonType}` |
| `backend/db/db.go::GetAllGraces` | sync.OnceValue cache; sorting by Region/Name + regex region mapping |
| `backend/db/db.go::IsKnownGraceID` | Predicate (membership in `data.Graces`) |
| `app_pvp.go` (`opts.SitesOfGrace`) | Placeholder; no own logic |
| `frontend/src/components/WorldTab.tsx` | UI: `GetGraces`/`SetGraceVisited`, bulk handlers, Tier 1 risk gate |
| `tests/grace_companion_flags_test.go` | 3 integration tests (SetOnRealSave / NoRTHFlags / SetOnlyNotCleared) |
| `backend/db/data/grace_companion_flags_test.go` | Unit test of the flag constants |

## 4. Mental model

Grace activation in SaveForge is a single bit in the `EventFlags` bitfield, optionally with a small fan-out:

```
SetGraceVisited(slot, graceID, visited)
  ├─ a.pushUndo(slotIndex)                  ← single snapshot rollback
  ├─ SetEventFlag(flags, graceID, visited)  ← grace bit (71xxx–76xxx)
  ├─ if DoorFlag != 0:
  │     SetEventFlag(flags, DoorFlag, visited)   ← dungeon door (Cat/HG)
  └─ if visited == true:
        for each f in CompanionEventFlagsForGrace(graceID):
          SetEventFlag(flags, f, true)       ← SET-only; no clear path
```

The grace bit + the optional `DoorFlag` are a **fully symmetric** SET/CLEAR pair. Companion flags are **asymmetric** — see §8.

## 5. Site of Grace data model

`data.Graces` (`backend/db/data/graces.go`) is a `map[uint32]GraceData` with 419 entries (source snapshot in the repo as of 2026-05-21). Each entry is:

```go
GraceData{
    Name        string  // e.g. "Church of Elleh (Limgrave)"
    DungeonType string  // "catacomb" | "hero_grave" | "" (none)
    DoorFlag    uint32  // 0 if there is no dungeon door
    BossArena   bool    // true if the grace is a boss arena
}
```

`db.GetAllGraces()` (`db.go:858`) converts this to `[]GraceEntry` with an additional `Region` field derived from the string after the parenthesis + regex splitting of the Limgrave (East/West), Liurnia (East/North/West) and Mountaintops (East/West) sub-regions. This is a **runtime derivation**, not a field stored in the save.

| Aspect | Value | Snapshot |
|---|---|---|
| Total count | **419** | verified `awk '/^var Graces/,/^}/' ... \| grep -cE '^\s*0x'` on 2026-05-21 |
| Snapshot | static Go map | regenerated manually from `regulation.bin` on game patches |
| Per-grace fields | `Name`, `DungeonType`, `DoorFlag`, `BossArena` | no region ID — region derived in `GetAllGraces` |

`needs verification`: the count 419 may differ after future DLC patches; there is no automatic regeneration from `regulation.bin` in the build. Snapshot date check: the last edit date of `graces.go`.

## 6. Grace IDs and event flags

### 6.1 ID space

Grace event flags occupy the **71000–76960** bands (with gaps) — see [15-event-flags.md](15-event-flags.md) §BST and the byte/bit table. The character of the bands (most commonly encountered subset):

| Band | Area type |
|---|---|
| 71xxx | Legacy dungeons (Stormveil, Leyndell, base game boss arenas) |
| 72xxx | DLC legacy dungeons (Belurat, Enir-Ilim) |
| 73xxx | Catacombs / Hero's Graves (paired with `DoorFlag`) |
| 74xxx | DLC catacombs / dungeons |
| 76xxx | Overworld (the largest group) |
| 76xxx (DLC subset, up to 76960) | Overworld DLC |

`needs verification`: the band segregation above is descriptive — the code does not use it for classification. `IsKnownGraceID(id)` is simply `_, ok := data.Graces[id]`. A DLC vs base game classifier **does not exist** for graces in the current code (contrast: `IsDLCMapFlag` for map flags — see [27-map-reveal.md](27-map-reveal.md)).

### 6.2 Door flags (dungeon catacombs / hero's graves)

`GraceData.DoorFlag` (if `!= 0`) is set **symmetrically** with the grace bit: both at `visited=true` and `visited=false`. This is the only part of `SetGraceVisited` that actually CLEARs anything on deactivation.

It applies only to entries `DungeonType == "catacomb"` or `"hero_grave"` — see the `Cat()` / `HG()` constructors in `graces.go:19/24`.

`needs verification`: whether the game actually **closes** the dungeon door when the DoorFlag is cleared at `visited=false` **has not been verified in-game**. The mechanic is based on the assumption that `DoorFlag` is a two-way trigger (open if SET, closed if CLEAR) — in the game it may be one-way (open trigger, but CLEAR has no effect).

### 6.3 BonfireId — a separate namespace

The game also uses a **second** ID namespace for graces — `BonfireId`, a 10-digit format (e.g., `1042362950` = Church of Elleh). It is stored as a single scalar `LastRestedGrace` in `PreEventFlagsScalars` (see [14-game-state.md](14-game-state.md)) and managed **only by the game** — the editor does not set it. SaveForge **does not maintain** an `EventFlag ID → BonfireId` mapping.

## 7. Visited / activated semantics

| State | What sets the grace bit | What the editor does on `SetGraceVisited(visited=true)` |
|---|---|---|
| **Grace bit `=1`** | The game on the first rest at the grace; the editor via `SetGraceVisited` | ✅ `SetEventFlag(graceID, true)` |
| **Map marker** | Derived by the UI/EMEVD from the grace bit | ✅ (side effect of SETting the bit) |
| **Fast-travel entry** | same | ✅ (side effect of SETting the bit) |
| **`LastRestedGrace` BonfireId** | The game on a physical touch / rest | ❌ the editor does not touch it — see §13 |
| **In-world activation sequence** (animation, NPC cutscene) | EMEVD on area load (most categories); may require additional area-load flags (e.g., 69300, 78101) | ❌ the editor does not set it |
| **MapFlags (62xxx/82xxx)** | Map reveal — a separate layer | ❌ — see [27-map-reveal.md](27-map-reveal.md) |

`needs verification`: the assumption "EMEVD derives the in-world state from the grace bit on area load" is verified **only for Church of Elleh (76100, overworld)** in a historical save diff (2026-05-09 — see `docs/CHANGELOG.md`). Other categories (legacy dungeons, catacombs, DLC) — were not verified individually.

## 8. Grace companion flags

`graceCompanionEventFlags` in `backend/db/data/grace_companion_flags.go` maps a **grace EventFlag ID → a minimal set of flags** that the game sets together with the grace during the first authentic visit. Currently:

| Grace | EventFlag ID | Companion flags |
|---|---|---|
| Gatefront (Limgrave West) | `0x0001294F` (= 76111) — `GatefrontGraceEventFlagID` | `EventFlagObtainedSpectralSteedWhistle`, `EventFlagMelinaGaveWhistle`, `EventFlagWhistleWorldState`, `EventFlagMelinaAcceptRefusePopup` |

**Only one entry** in the current DB. The companion flag pool for the remaining ~418 graces is empty — `CompanionEventFlagsForGrace(otherGrace)` returns `nil` and the loop `for _, f := range companions` is a no-op.

`needs verification`: whether other graces also require a companion flag fan-out so that the game engine does not re-trigger a dialogue / cutscene — has not been exhaustively investigated. The code construction is forward-compatible (each new pair in the map is automatically active), but it requires manual per-grace research.

The companion flag policy shared with the item path (e.g., Spectral Steed Whistle) — see [50-item-companion-flags.md](50-item-companion-flags.md). The constants `EventFlagObtained...` are shared between `grace_companion_flags.go` and `item_companion_flags.go`.

### 8.1 What does NOT enter the companion set (explicit exclusion)

The test `TestCompanionEventFlagsForGrace_NoForbiddenFlags` (`backend/db/data/grace_companion_flags_test.go:35`) **and** `TestGraceCompanionFlagsNoRTHFlags` (`tests/grace_companion_flags_test.go:49`) enforce a negative list of the same 9 IDs:

| Excluded flag | Reason (from the test comments / `grace_companion_flags.go`) |
|---|---|
| `10009655` | Melina RTH invitation trigger — a separate progress step |
| `11109658` | Gideon welcome (RTH visited marker) |
| `11109659` | Gideon advice |
| `11109786` | RTH transport trigger — transient (cleared by the game engine) |
| `4698` | Melina cutscene trigger — transient |
| `4656` | Level up performed — a separate user action |
| `710770` | Melina leaves Gatefront — research candidate, a runtime test confirmed that it is not required (spec/50 PS4 test 2026-05-11) |
| `69090` | same |
| `69370` | same |

`needs verification`: older iterations of spec/47 additionally listed `4651`–`4653` as "Melina dialog states transient" — these IDs are **not** in either of the two `forbidden` lists in the tests (they are only in `backend/db/data/quests.go:487` as quest popup requirements). The status of their classification as forbidden vs simply unverified — open.

Rule from the comment in `grace_companion_flags.go`: **only flags verified in real saves after an actual in-game occurrence**.

## 9. SET-only asymmetric behavior

**The SET-only contract** for companion flags:

```go
// app_world.go:70-81
// SET-only: companion flags are set on activation but never cleared on deactivation.
// They may also be set by item companion flags or normal game progression — clearing
// them on visited=false would regress saves that obtained the flags through other paths.
if visited {
    if companions := data.CompanionEventFlagsForGrace(graceID); len(companions) > 0 {
        for _, f := range companions {
            if err := db.SetEventFlag(flags, f, true); err != nil {
                fmt.Printf("Warning: companion flag %d for grace %d: %v\n", f, graceID, err)
            }
        }
    }
}
```

Consequences:

| Action | What happens to the grace bit | What happens to companion flags |
|---|---|---|
| `SetGraceVisited(visited=true)`  | `=1` | each flag in the set `=1` |
| `SetGraceVisited(visited=false)` | `=0` | **untouched** (remain in their pre-call state) |
| `SetGraceVisited(visited=true)` after an earlier `=true` | `=1` (idempotent) | `=1` (idempotent) |

The asymmetry is **intentional** — justification in the code comment: companion flags may be set by **another** path (an item companion flag from [50-item-companion-flags.md](50-item-companion-flags.md), normal game progress). CLEAR at `visited=false` would undo progress achieved by another route — a regression risk.

`needs verification`: whether there are situations where the user **wants** to clear companion flags after deactivating a grace (e.g., testing). There is no API nor UI for this — the only path is a manual bitfield mutation.

Contrast: the grace bit + `DoorFlag` are **symmetric** (SET/CLEAR together with `visited`). Only companion flags are SET-only. See §13.4 for rollback implications.

The test enforcing the contract: `TestGraceCompanionFlagsSetOnlyNotCleared` (`tests/grace_companion_flags_test.go:74`).

## 10. Current implemented behavior

### 10.1 Backend endpoints (Wails bindings)

| Endpoint | Signature | What it does |
|---|---|---|
| `GetGraces(slotIndex) ([]db.GraceEntry, error)` | `app_world.go:14` | Returns the full grace list (`db.GetAllGraces`) with `Visited` populated from the bitfield |
| `SetGraceVisited(slotIndex, graceID, visited) error` | `app_world.go:43` | See §4 mental model |

`GetGraces` is **read-only** and tolerates a missing `EventFlagsOffset` (skip and `Visited=false`). It does not error out on warnings (`fmt.Printf` instead of error propagation).

`SetGraceVisited` validates:
- `a.save != nil`,
- `slotIndex ∈ [0, 10)`,
- `slot.EventFlagsOffset` within a sensible range.

There is no validation of `graceID` against `IsKnownGraceID(graceID)` — the endpoint **accepts any ID**. An attempt to set an unknown flag will pass to `db.SetEventFlag` and either update the bit or return an error from the resolver. `needs verification`: whether an unknown `graceID` outside `data.Graces` but in the BST band causes a SET on a non-grace bit.

### 10.2 UI write paths (frontend)

From `frontend/src/components/WorldTab.tsx`:

| Handler | What it calls | Tier risk |
|---|---|---|
| `handleGraceToggle(grace, visited)` | `SetGraceVisited` per-grace | no modal (single toggle) |
| `handleUnlockRegionGraces(rg)` | `SetGraceVisited(true)` per region grace (Promise.all) | no modal (bulk per region) |
| `handleUnlockAllGraces()` | `SetGraceVisited(true)` on all inactive (filter: skipBossArenas + Ashen Capital opt-in) (Promise.all) | **Tier 1** `RiskActionButton riskKey="bulk_grace_unlock"` |
| `handleLockAllGraces()` | `SetGraceVisited(false)` on all active (Promise.all) | no modal |

UI note in `WorldTab.tsx:406`:

> Sites of Grace unlocked here will appear on the map and become available for fast travel. Some graces may still play their in-world activation sequence when visited.

This note deliberately does not promise a full visual activation.

`needs verification`: the bulk handlers call `SetGraceVisited` in `Promise.all` — each call is a **separate** `pushUndo`. A sequence of N calls creates N undo snapshots, not 1 bulk snapshot. Test coverage: no isolated test "Promise.all bulk SetGraceVisited — undo stack size N".

## 11. PvP module status

`app_pvp.go::PrepPvP` has in `PvPPreparationOptions` the field `SitesOfGrace bool`. The code branch (lines 108-110):

```go
if opts.SitesOfGrace {
    warnings = append(warnings, "Sites of Grace module is planned but not enabled in this version.")
}
```

This is a **placeholder** — the module:

- does not read `data.Graces`,
- does not call `SetGraceVisited`,
- does not set any flags,
- does not modify `slot.Data` in any way.

The entire grace activation in the PvP prep path is a **no-op with a warning**. A PvP user who wants to unlock all graces must use `Unlock All` from the `WorldTab` UI separately.

`needs verification`: whether the intent is for the PvP module to do a bulk unlock in the future (analogous to `handleUnlockAllGraces`), or a more selective set (only boss arena graces, only overworld) — not documented in the code. Master status of PvP modules — see [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md).

## 12. Relation to Event Flags

All operations on grace flags + DoorFlag + companion flags go through the **generic helper API** from [15-event-flags.md](15-event-flags.md):

- `db.GetEventFlag(flags, id)` — read a bit,
- `db.SetEventFlag(flags, id, value)` — symmetric SET/CLEAR,
- `resolveEventFlagPosition` — 3-tier resolver (precomputed → BST → fallback).

This chapter **does not duplicate** byte/bit indexing, BST mechanics, snapshot dates. The only local addition is:

- the ID range 71000–76960 as the used namespace,
- the companion flag fan-out (§8) and the SET-only contract (§9) as a grace-specific policy.

`needs verification`: the range 71000–76960 is descriptive (the top-level BST band). The exact `min/max` for the 419 entries in `data.Graces` is not cited — it can be computed with `awk` on the current DB.

## 13. Relation to Map / World State

`SetGraceVisited` **does not touch** any of the Map Reveal layers (see [27-map-reveal.md](27-map-reveal.md)):

- L0 `UnlockedRegions` — untouched,
- L1 MapVisible / MapAcquired event flags 62xxx/63xxx — untouched,
- L2 DLC Cover Layer (BloodStain coords) — untouched (see [29-dlc-black-tiles.md](29-dlc-black-tiles.md)),
- L3 FoW bitfield — untouched.

Conversely: `RevealAllMap` / `revealBaseMap` / `revealDLCMap` (see 27) **do not set** any grace flags nor companion flags. These two systems are **independent** in SaveForge.

`WorldState` / `WorldArea` / `WorldGeomMan` (see [16-world-state.md](16-world-state.md)) — `SetGraceVisited` does not modify any of these sections.

`needs verification`: whether the game displays a grace marker on the map only if the corresponding `MapVisible` flag is also SET (map fragment owned) — not verified. It is possible that the grace bit is enough per se, possible that the region must also be revealed.

## 14. Relation to Game State

`SetGraceVisited` **does not modify** `LastRestedGrace` (`PreEventFlagsScalars`, see [14-game-state.md](14-game-state.md)):

```go
// app_world.go:41
// Does not touch LastRestedGrace — the game updates that automatically on arrival.
```

Reason: `LastRestedGrace` is a BonfireId (a separate namespace — §6.3), managed at runtime by the game on a physical touch / teleport. Setting this field with the editor is not needed; the game will overwrite it on the first arrival.

The second occurrence of BonfireId — in the NetworkManager section (`slot.Data[≈0x1F636A]` in a test slot, ~1300 B past the end of EventFlags) — is also **untouched** by the editor. An empirical observation from the save diff 2026-05-09. `needs verification` as to the exact offset in other slots / versions.

`SetGraceVisited` also does not affect: `TotalDeathsCount`, `InGameCountdownTimer`, the NG+ state ([14-game-state.md](14-game-state.md)).

## 15. Write path and rollback caveats

### 15.1 Atomicity

`SetGraceVisited` has **3 mutations** in a single call (at `visited=true` with DoorFlag + companion flags):

1. the grace bit (`SetEventFlag(graceID, true)`),
2. optionally the door bit (`SetEventFlag(DoorFlag, true)`),
3. optionally N companion bits (`SetEventFlag(companions[i], true)`).

There is no per-flag rollback. If step 1 succeeds and step 2 returns an error, the error is propagated to the caller but **step 1 stays** (the grace bit is already set). Step 3 (companion flags) uses `fmt.Printf` instead of error propagation — errors are **logged, not returned**.

### 15.2 Snapshot undo (save-level)

`a.pushUndo(slotIndex)` at the start of `SetGraceVisited` — the **only** rollback path. A full pre-mutation snapshot of the slot. A per-bit selective undo does not exist.

The bulk handler from the UI (`handleUnlockAllGraces`) does N×`SetGraceVisited` via `Promise.all` — i.e., N×`pushUndo`. The stack grows linearly with the number of graces. `needs verification`: whether `pushUndo` has a depth limit and whether a bulk with ~419 graces does not evict older snapshots.

### 15.3 No idempotency check

`SetGraceVisited(graceID, true)` on an already-SET bit will again pass through `pushUndo` + `SetEventFlag` + companion fan-out. Idempotent in terms of result (the bit stays `=1`), but **not a no-op** in terms of cost (snapshot, mutations).

### 15.4 Companion flags rollback asymmetry

A snapshot undo will revert **all** bitfield changes — including companion flags set in the same call. This is OK from the undo logic perspective, but creates a subtle edge case:

- if a companion flag was already `=1` from another path (item, game progress),
- and `SetGraceVisited(true)` + `Undo` is executed,
- the snapshot will revert to the pre-mutation state, in which the companion flag was `=1`.

In practice: a companion flag may return to `=1` after Undo, even though the SET-only logic would "clear" it. This is correct behavior (Undo is a clean restore), but it is worth being aware of.

## 16. Validation and safety notes

### 16.1 Stale data after a game patch

The 419 entries in `data.Graces` are a **snapshot** from `regulation.bin` — not regenerated automatically. After a DLC patch adding graces, the `data.Graces` band may not cover the new IDs. Consequences:

- new graces will not appear in the UI,
- a bulk Unlock All will not set the new bits,
- `IsKnownGraceID(newID)` will return `false`.

`needs verification`: there is no automatic detection of "regulation.bin newer than the graces.go snapshot".

### 16.2 Wrong EventFlag IDs

`SetGraceVisited` does not validate `graceID` against `data.Graces`. A call from the UI is always correct (the UI iterates over `db.GetAllGraces`), but someone calling from Wails JS / tests may pass any ID. Effect: an unverified flag will be set.

### 16.3 Quest / NPC progression side effects

The Gatefront companion flags (Spectral Steed Whistle, Melina Accord) **are** quest-NPC progression flags. Setting them with the editor on a save from before the first Melina encounter:

- may skip Melina's cutscene,
- may cause the NPC to not appear in the expected places,
- may disrupt the RTH trigger sequence (Melina invitation).

`needs verification`: SaveForge does **not** warn about this before calling `SetGraceVisited(76111, true)` on a fresh save.

### 16.4 SET-only cannot be undone per-flag

See §9 + §15.4. If the user wants to **separate** "grace visited" from "companion flags set", there is no such control — a single endpoint.

### 16.5 No atomic rollback for multi-grace bulk

Bulk Unlock All — N×`SetGraceVisited` via `Promise.all`. If, e.g., 200/419 calls pass and the 201st returns an error, there is no `rollback to pre-bulk state`. The UI cache (`setGraces(prev => ...)`) will update only the successful ones, but the undo stack is 200 separate snapshots.

### 16.6 PvP module placeholder

`opts.SitesOfGrace=true` in `ApplyPvPPreparation` will not set graces. User-facing effect: a warning in the return value, but no fail-loud (no error). It is possible that a PvP user assumes the module does an unlock — see [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md).

### 16.7 In-world activation sequence after a patch

The assumption "EMEVD derives the visual state from the grace bit on area load" was verified on Church of Elleh in 2026-05-09. Newer patches may change EMEVD scripting. `needs verification` after every major game patch.

## 17. Test coverage

| Test | File | What it covers |
|---|---|---|
| `TestGraceCompanionFlagsSetOnRealSave` | `tests/grace_companion_flags_test.go:15` | `CompanionEventFlagsForGrace(76111)` returns a non-empty list; each flag settable + readable on a real slot |
| `TestGraceCompanionFlagsNoRTHFlags` | `tests/grace_companion_flags_test.go:49` | The forbidden list (RTH, transient, level-up) does NOT appear in the Gatefront companion set |
| `TestGraceCompanionFlagsSetOnlyNotCleared` | `tests/grace_companion_flags_test.go:74` | Simulates "flags set by another path"; verifies they remain SET (SET-only contract) |
| `TestCompanionEventFlagsForGrace_Gatefront` | `backend/db/data/grace_companion_flags_test.go:5` | The companion set for Gatefront contains exactly the 4 expected constants (`EventFlagObtainedSpectralSteedWhistle`, `EventFlagMelinaGaveWhistle`, `EventFlagWhistleWorldState`, `EventFlagMelinaAcceptRefusePopup`) |
| `TestCompanionEventFlagsForGrace_Unknown` | `backend/db/data/grace_companion_flags_test.go:28` | `CompanionEventFlagsForGrace(0xDEADBEEF)` returns `nil` (unknown grace ID) |
| `TestCompanionEventFlagsForGrace_NoForbiddenFlags` | `backend/db/data/grace_companion_flags_test.go:35` | Iterates over **all** companion sets in `graceCompanionEventFlags`; checks that none of the 9 forbidden IDs appears |

**No isolated test for**:

- the `SetGraceVisited` endpoint (as a whole — pushUndo + 3 mutations + error propagation),
- `GetGraces` with various `EventFlagsOffset` states,
- bulk Promise.all from the UI,
- DoorFlag SET/CLEAR symmetry per-category (catacomb vs hero_grave).

`needs verification`: adding `TestSetGraceVisitedRoundtrip` (load save → SetGraceVisited(true) → assert the grace bit + DoorFlag + companion flags) would be desirable. Currently the coverage is indirect — through companion tests + a manual in-game test.

## 18. Known limits / needs verification

A condensed list of open questions:

1. **Companion flags for the remaining ~418 graces** — empty pool (`needs verification`: which graces require a fan-out).
2. **The count 419 stale after a patch** — no auto-regeneration from `regulation.bin`.
3. **DLC vs base game classifier** — no `IsDLCGraceID` analog to `IsDLCMapFlag`.
4. **DoorFlag two-way symmetry** — the game may treat CLEAR as a no-op.
5. **In-world activation per-category** — verified only for Church of Elleh overworld.
6. **`SetGraceVisited` validation of `graceID`** — no `IsKnownGraceID` check before mutation.
7. **Bulk Promise.all undo stack** — N snapshots; no per-bulk single snapshot.
8. **PvP module intent** — placeholder, no documented target semantics.
9. **Quest progression side effects** — no pre-mutation warning for NPC-progression-critical flags.
10. **Second occurrence of BonfireId in NetworkManager** — an empirical observation of one slot; not verified cross-slot / cross-platform.

## 19. Cross-references

- [11-regions.md](11-regions.md) — `UnlockedRegions`; untouched by `SetGraceVisited`.
- [14-game-state.md](14-game-state.md) — `LastRestedGrace`, `PreEventFlagsScalars`; untouched by the editor.
- [15-event-flags.md](15-event-flags.md) — master event flag API; chapter 47 is its caller.
- [16-world-state.md](16-world-state.md) — `WorldGeomMan` / `WorldArea`; untouched.
- [27-map-reveal.md](27-map-reveal.md) — 4-layer Map Reveal; independent of grace activation.
- [29-dlc-black-tiles.md](29-dlc-black-tiles.md) — L2 DLC Cover Layer; independent.
- [48-pvp-ready-modular-presets.md](48-pvp-ready-modular-presets.md) — PvP modules; `SitesOfGrace` placeholder.
- [50-item-companion-flags.md](50-item-companion-flags.md) — item companion flags; **shared** constants `EventFlagObtainedSpectralSteedWhistle`, etc.

## 20. Sources

- `app_world.go::GetGraces` / `SetGraceVisited`.
- `backend/db/data/graces.go` — `Graces` map, `Cat()`/`HG()` constructors, `GraceData` struct.
- `backend/db/data/grace_companion_flags.go` — `graceCompanionEventFlags`, `CompanionEventFlagsForGrace`, `GatefrontGraceEventFlagID`.
- `backend/db/db.go` — `GraceEntry`, `GetAllGraces`, `IsKnownGraceID`, region mapping.
- `app_pvp.go:108-110` — placeholder `SitesOfGrace` module.
- `frontend/src/components/WorldTab.tsx` — UI bindings and bulk handlers.
- `tests/grace_companion_flags_test.go` — 3 integration tests.
- `backend/db/data/grace_companion_flags_test.go` — unit tests of the constants.
- `docs/CHANGELOG.md` — historical runtime save diff Church of Elleh (2026-05-09).
