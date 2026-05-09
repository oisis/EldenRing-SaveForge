# 48 — PvP-Ready Modular Presets

> **Type**: Design doc
> **Status**: ✅ Implemented (Phase 1 complete)
> **Scope**: Decomposition of the monolithic PvP-ready world preset into independent,
> labeled modules with per-module risk tiers, validators, and granular UI controls.

---

## Background

The original `pvp-ready` preset applies a flat `WorldPresetData` blob: graces + regions +
summoning pools + colosseums + map flags. This approach has three problems:

1. **Opaque coupling** — users cannot choose "regions only" without also getting all graces
   and summoning pools.
2. **Unverified claims** — summoning pool activation was presented as an invasion accelerator;
   this is unconfirmed (see spec/46 §11).
3. **Stale data** — old presets contained pre-v1.12 pool IDs (≥ 1 000 000). Validator
   `validateWorldSummoningPools` now detects these, but the root cause is no formal module
   ownership of each flag range.

The fix: decompose into six modules, assign each a confirmed effect and a risk tier,
then expose them individually in the UI.

---

## 1. Existing `WorldPresetData` Structure

```go
type WorldPresetData struct {
    Graces         []uint32  // EventFlags 71xxx–76xxx
    Bosses         []uint32  // boss defeat EventFlags
    SummoningPools []uint32  // EventFlags 670xxx
    Colosseums     []uint32  // EventFlags 60xxx (activate only; multi-flag via ColosseumFlagSets)
    MapFlags       []uint32  // EventFlags 62xxx / 82xxx
    Cookbooks      []uint32  // EventFlags cookbook range
    BellBearings   []uint32  // EventFlags bell bearing range
    Whetblades     []uint32  // EventFlags whetblade range
    Gestures       []uint32  // gesture slot IDs
    Regions        []uint32  // UnlockedRegions — separate binary struct, NOT EventFlags
    WorldPickups   []uint32  // bolstering material pickup EventFlags
}
```

`Regions` is the only field that writes to `slot.UnlockedRegions` via
`core.SetUnlockedRegions`. All other fields write EventFlag bits via `db.SetEventFlag`.

---

## 2. Module Definitions

### Module A — PvP Core (Stats + Items)

| Property | Value |
|---|---|
| Risk tier | Tier 1 (low; standard build editing) |
| Save fields touched | `PlayerGameData`, `Inventory`, `Storage` |
| Backend functions | `CharacterPreset.Character`, `Inventory`, `Storage` |

**Contents:**
- Character level, stats (Vigor/Mind/Endurance/Strength/Dex/Int/Faith/Arcane)
- Weapons, armor, talismans in equipment slots
- PvP consumables in inventory/storage:
  - Bloody Finger (item ID: item lookup from `db`)
  - Festering Bloody Finger
  - Recusant Finger
  - Taunter's Tongue
  - Runes / smithing materials
- Memory Stones (sorcery slot count)

**Confirmed effect:** Direct. All fields are standard character edit operations.

---

### Module B — Matchmaking Regions

| Property | Value |
|---|---|
| Risk tier | Tier 1 (low; unlock is reversible) |
| Save fields touched | `slot.UnlockedRegions` (separate binary struct) |
| Backend functions | `BulkSetUnlockedRegions`, `core.SetUnlockedRegions` |
| Data source | `backend/db/data/regions.go` — 104 entries: 62 overworld base game (6100000–6899999) + 7 DLC (6900000–6999999) + 35 legacy dungeons (1000000–1999999) |

**Contents:**
- All entries from `data.Regions` — overworld + legacy dungeons + DLC land of shadow
- Controls invasion eligibility (Bloody Finger can only reach areas in this list)
- Controls NPC invader appearance (Recusant Henricus etc.)
- Controls "You have entered \<X\>" map label after teleport

**Confirmed effect (spec/11, runtime-observed):** Regions must be present for PvP
matchmaking to consider the slot for invasion. Missing regions = effectively invisible
to invaders in that area.

**Note:** `data.IsDLCRegion(id)` → range `6900000–6999999` = Shadow of the Erdtree.
DLC regions should be marked separately in the UI. Applying them to a non-DLC character
is harmless but logically wrong.

**Validator:** `ValidateWorldRegions` (see §4).

---

### Module C — PvP Access / Progress Flags

| Property | Value |
|---|---|
| Risk tier | Tier 1 (quest flag manipulation; same tier as boss kills) |
| Save fields touched | EventFlags bitfield |
| Backend functions | `db.SetEventFlag`, `SetQuestStep`, `SetColosseumUnlocked` |
| Data source | `quests.go` (White Mask Varre), `summoning_pools.go` (Colosseums) |

**Contents:**

#### Colosseums (confirmed: map + matchmaking unlock)
- Limgrave Colosseum: `ColosseumFlagSets[60360]` → flags `60360, 62730, 69460, 710860`
- Caelid Colosseum: `ColosseumFlagSets[60350]` → flags `60350, 62720, 69450, 710850`
- Royal Colosseum: `ColosseumFlagSets[60370]` → flags `60370, 62740, 69470, 710870`
- Global colosseum flags: `6080, 60100, 69480`

Gate open state (physical door) is stored outside EventFlags (WorldGeom binary blob) —
this cannot be edited from a save editor. Player must open the gate once in-game.

**Preset format limitation:** `ExportWorldState` captures only the activate flags
(60350/60360/60370) into `World.Colosseums`. NPC (69xxx), Gate (710xxx), and global
flags (6080, 60100, 69480) are absent from named data tables and are not exported.
`ApplyWorldState` sets only what is in the preset — it does not auto-expand like
`SetColosseumUnlocked`. For a complete colosseum unlock via preset, the user must
manually add global flags to `World.Colosseums` and ensure MapPOI flags are in
`World.MapFlags`. `ValidateWorldColosseums` warns when these are missing.

#### Varre Quest (Bloody Finger / Mohgwyn Palace access)
Key steps to unlock Bloody Finger via quest path:
- Receiving "Lord of Blood's Favor" — flag `400031`
- Soaking favor with maiden's blood — flag `400033`
- Offering finger / Bloody Finger granted — flags `1035449227, 9432, 9420, 1800`
- Receiving Pureblood Knight's Medal (Mohgwyn shortcut) — flag `400032`

**Caution:** Varre quest flags are a complex multi-step chain (see `quests.go`
"White Mask Varre" — 19 steps). Setting only completion flags without prior
dialogue flags may leave the NPC in an inconsistent state in-game. UI should
offer "skip to Bloody Finger granted" as a preset step, not raw flag toggles.
Implementation should use `SetQuestStep` rather than raw `SetEventFlag`.

**Confirmed effect (quest system):** Setting Bloody Finger flags via quest system
is the same mechanism used for all quest completion. Colosseum flags are hex-verified
(see `tmp/coloseum-debug/`).

**Validator:** `ValidateWorldColosseums` (see §4).

---

### Module D — World QoL (Map + Graces)

| Property | Value |
|---|---|
| Risk tier | Tier 0 (visual/navigation only; no competitive effect) |
| Save fields touched | EventFlags bitfield |
| Backend functions | `RevealAllMap`, `SetGraceVisited` (via `ApplyWorldState`) |
| Data source | `backend/db/data/graces.go`, `maps.go` |

**Contents:**
- Map reveal: `62xxx` (map tile visibility) + `82xxx` (map system flags)
- Sites of Grace: `71xxx–76xxx` — controls map marker + fast-travel list entry
  - DoorFlag companion for 73xxx catacombs/hero graves
  - In-world activation animation: runtime/EMEVD-managed, may still play on first visit (spec/47)

**Confirmed effect:** Map markers appear; fast-travel list populated. No competitive
advantage — does not affect invasion matchmaking, BF wait times, or session state.

**Note:** Map reveal and grace unlock are independent sub-options within QoL. The UI
should allow selecting them separately (map reveal without grace unlock and vice versa).

**Validators:** `ValidateWorldGraces`, `ValidateWorldMapFlags` (see §4).

---

### Module E — Co-op / Summon (Summoning Pools)

| Property | Value |
|---|---|
| Risk tier | Tier 1 (EventFlag set; reversible) |
| Save fields touched | EventFlags bitfield (670xxx block) |
| Backend functions | `SetSummoningPoolActivated` (via `ApplyWorldState`) |
| Data source | `backend/db/data/summoning_pools.go` — 670xxx, 199 entries |
| Existing validator | `validateWorldSummoningPools` in `vm/preset.go` |

**Contents:**
- Martyr Effigy activation flags (`670xxx`) — enables co-op summon signs in dungeons
- DLC summoning pools: `670800–670999` — Shadow of the Erdtree
- Pre-v1.12 IDs (≥ 1 000 000): detected and warned by existing validator

**Confirmed effect:** Enables co-op summon signs near Martyr Effigies.

**Bloody Finger invasion impact: UNCONFIRMED.** Summoning pool activation was
historically assumed to reduce invasion matchmaking wait times. This is not confirmed
by the spec/46 binary analysis. Do not present this module as an invasion speed-up.
Present it only as co-op/summon activation.

**Validator:** `validateWorldSummoningPools` — already implemented.

---

### Module F — Advanced / Research (Diagnostics)

| Property | Value |
|---|---|
| Risk tier | Tier 2 (UD11 write: PS4 file survives, server overwrites on connect) |
| Save fields touched | UD11 NetworkParam (read/write), UD10 BF state (read-only), UD0 CandidateSection (read-only) |
| Backend functions | `core.NetworkParamValues` (existing), new diagnostic readers |
| Data source | spec/44 (NetworkParam), spec/46 (UD10/UD0 state machine) |

**Contents:**

#### UD11 NetworkParam Inspector / Patcher
- Read current values of `breakInRequestIntervalTimeSec`, `breakInRequestTimeOutSec`, etc.
- Optional write of tuned values (see spec/44 for field table)
- **Warning to user**: "PS4/PS5 console overwrites regulation.bin from server on online
  connect. UD11 changes are local-only and unconfirmed to affect actual invasion timing."

#### UD10 BF State Classifier
- Read-only display of `UD10+0x5070`, `UD10+0x194E4`, `UD10+0x5080`
- Map to human-readable state: `PASSIVE / BF-INIT / ACTIVE-BF / SUCCESS / TIMEOUT / PATCHED-IDLE`
- Decision tree from spec/46 §7

#### UD0 Candidate Section Viewer
- Read-only display of `UD0+0x209B00..0x209C43`
- Parse CandidateEntry structs (stride `0x14`, 5 fields)
- Show `SPEC-VALID / DEVIATES / NOT-INITIALIZED` status
- Display V-queue state (IDLE vs ACTIVE, which entry_id is at V0)

**This module is diagnostic only.** Do not expose as a "PvP speed-up" button.
The NetworkParam patcher is an advanced feature for research users who understand
that server-side enforcement renders UD11 changes ineffective online.

---

## 3. Flag Range → Module Assignment

| Data / Flag range | Current meaning in code | Module | Confidence | Notes |
|---|---|---|---|---|
| `1000000–1999999` | `data.Regions` — legacy dungeon interiors (35 entries) | B — Matchmaking Regions | ✅ confirmed | Separate binary struct via `core.SetUnlockedRegions` |
| `6100000–6899999` | `data.Regions` — overworld base game (62 entries) | B — Matchmaking Regions | ✅ confirmed | Separate binary struct via `core.SetUnlockedRegions` |
| `6900000–6999999` | DLC Regions — Land of Shadow (7 entries) | B — Matchmaking Regions (DLC) | ✅ confirmed | `IsDLCRegion()` guard |
| `60350, 60360, 60370` | Colosseum activate | C — PvP Access | ✅ hex-verified | Use `ColosseumFlagSets`, not activate-only |
| `62xxx` | Map tile visibility | D — World QoL | ✅ confirmed | `RevealBaseMap` / `MapFlags` |
| `82xxx` | Map system flags | D — World QoL | ✅ confirmed | `RevealBaseMap` / `MapFlags` |
| `71xxx–76xxx` | Sites of Grace EventFlags | D — World QoL | ✅ confirmed | spec/47 — map marker + fast-travel |
| `670xxx` (base, 100–799) | Summoning Pools — Martyr Effigies | E — Co-op / Summon | ✅ confirmed | BF invasion impact: unconfirmed |
| `670800–670999` | DLC Summoning Pools | E — Co-op / Summon (DLC) | ✅ confirmed | `IsDLCSummoningPool()` |
| `≥ 1 000 000` (old pool IDs) | Pre-v1.12 format — invalid | Legacy / Stale | ✅ confirmed | `validateWorldSummoningPools` warns |
| Varre quest flags | `1035449xxx`, `400031–400037` | C — PvP Access | ⚠️ cross-ref | Multi-step; use `SetQuestStep` |
| Unknown flags `1033438600–1050558540` | Unclassified world object/area state | Unknown | ❓ | Do not assign to a module |
| `6080, 60100, 69480` | Colosseum global flags | C — PvP Access | ✅ hex-verified | `ColosseumGlobalFlags` |

---

## 4. Proposed Validators

All validators live in `backend/vm/validation.go` (existing) or `backend/vm/preset.go`.
None are production-blocking — they emit `warnings []string`, not fatal errors.

### `ValidateWorldRegions(ids []uint32) []string` ✅ Implemented

**Checks:**
- Each ID is present in `data.Regions` map → `warning: world.regions: ID %d not found in region database`
- Detects duplicate IDs → `warning: world.regions: ID %d appears more than once — duplicate will be ignored`
- Legacy dungeon IDs (`1000000–1999999`) are valid — no warning
- DLC region IDs (`6900000–6999999`) are valid — no warning (non-DLC context check reserved for future UI layer)

**Returns:** warning (not error). Region lock/unlock is reversible.

**Where it lives:** `backend/vm/preset.go` as `validateWorldRegions` (private, called from `ValidatePreset`).
DB helper: `db.IsKnownRegionID(id uint32) bool` in `backend/db/db.go`.

**Tests (7/7 passing):**
- Known overworld ID → no warning
- Known legacy dungeon ID → no warning
- Known DLC ID → no warning
- Unknown ID (`9999999`) → warning
- Duplicate known ID → warning
- `nil` World → no warning
- Empty region list → no warning

---

### `ValidateWorldGraces(ids []uint32) []string` ✅ Implemented

**Checks:**
- Each ID is present in `data.Graces` map → `warning: world.graces: ID %d not found in grace database`
- Detects duplicate IDs → `warning: world.graces: ID %d appears more than once — duplicate will be ignored`
- DLC grace IDs (`72xxx`, `74xxx`) are valid — no warning
- `DoorFlag` is NOT required in preset — `SetGraceVisited()` sets it automatically from `data.Graces`
- Does not validate `LastRestedGrace`, `BonfireId`, or EMEVD activation state (see spec/47)

**Returns:** warning (not error).

**Where it lives:** `backend/vm/preset.go` as `validateWorldGraces` (private, called from `ValidatePreset`).
DB helper: `db.IsKnownGraceID(id uint32) bool` in `backend/db/db.go`.

**Tests (7/7 passing):**
- Known base grace (`71000`, `76100`) → no warning
- Known DLC grace (`72000`, `72001`) → no warning
- Catacomb grace with `DoorFlag` (`73000`) → no warning (DoorFlag not required in preset)
- Unknown ID (`99999`) → warning
- Duplicate known ID → warning
- `nil` World → no warning
- Empty grace list → no warning

---

### `ValidateWorldMapFlags(ids []uint32) []string` ✅ Implemented

**Checks:**
- Each ID is present in any of the four map data maps (`data.MapVisible`, `data.MapSystem`, `data.MapAcquired`, `data.MapUnsafe`) → `warning: world.mapFlags: ID %d not found in map flag database`
- Detects duplicate IDs → `warning: world.mapFlags: ID %d appears more than once — duplicate will be ignored`
- Grace IDs (`71xxx–76xxx`) accidentally placed in `MapFlags` → warning (not found in any map data map)
- Summoning pool IDs (`670xxx`) accidentally placed in `MapFlags` → warning (not found in any map data map)

**Returns:** warning (not error).

**Where it lives:** `backend/vm/preset.go` as `validateWorldMapFlags` (private, called from `ValidatePreset`).
DB helper: `db.IsKnownMapFlagID(id uint32) bool` in `backend/db/db.go` — checks all four map data maps.

**Data coverage:**
- `data.MapVisible` — 85 entries (`62010–62221`), tile visibility flags
- `data.MapSystem` — 79 entries (`62000–82002`), includes `62000` (Allow Map Display), `82001` (Show Underground), `82002` (Show Shadow Realm Map)
- `data.MapAcquired` — 24 entries
- `data.MapUnsafe` — 56 entries

**Tests (8/8 passing):**
- Known `MapVisible` IDs (`62000`, `62010`) → no warning
- Known `MapSystem` IDs (`82001`, `82002`) → no warning
- Unknown ID (`99999`) → warning
- Duplicate known ID (`62010` repeated) → warning
- `nil` World → no warning
- Empty map flag list → no warning
- Grace ID (`76100`) placed in `MapFlags` → warning (misplaced)
- Summoning pool ID (`670100`) placed in `MapFlags` → warning (misplaced)

---

### `ValidateWorldColosseums(world *WorldPresetData) []string` ✅ Implemented

**Checks:**
- Each ID in `World.Colosseums` is recognised in `data.ColosseumFlagSets` → `warning: world.colosseums: ID %d not found in colosseum database`
- Detects duplicate IDs → `warning: world.colosseums: ID %d appears more than once — duplicate will be ignored`
- For each activate flag, MapPOI companion flag must be present in `World.MapFlags` → `warning: world.colosseums: %s (ID %d) is missing companion map flag %d — colosseum icon will not appear on map`
- Global colosseum flags (6080, 60100, 69480) must be present in `World.Colosseums` → `warning: world.colosseums: global colosseum flag %d is missing — add to World.Colosseums for full unlock`
- Global flags in `World.Colosseums` are silently accepted (not treated as unknown)

**Returns:** warning (not error). Import is never blocked.

**Where it lives:** `backend/vm/preset.go` as `validateWorldColosseums(world *WorldPresetData, warnings *[]string)`.
DB helpers: `db.IsKnownColosseumID(id uint32) bool`, `db.GetColosseumFlagSet(id uint32) (data.ColosseumFlagSet, bool)` in `backend/db/db.go`.

**Why:** Colosseum with only the Activate flag set may show in matchmaking but lack the
map icon. Preset application (`ApplyWorldState`) does not auto-expand flag sets like
`SetColosseumUnlocked` does — companion flags must be explicit.

**Known limitation:** NPC (69xxx) and Gate (710xxx) companion flags are not captured by
`ExportWorldState` and cannot be verified from preset data alone. Physical gate open
state is outside EventFlags entirely (WorldGeom binary blob) — this validator does not
and cannot check it. Gate state remains future research.

**Tests (8/8 passing):**
- Full set: Limgrave activate (60360) + global flags (6080/60100/69480) + MapPOI (62730) → no warning
- Unknown ID (99999) → warning
- Duplicate activate ID (60360 ×2) → warning
- Activate-only (no MapPOI, no global flags) → MapPOI warning + 3 global flag warnings
- Missing global flags only (MapPOI present) → 3 global flag warnings
- `nil` World → no warning
- Empty `World.Colosseums` → no warning
- All three colosseums (60350/60360/60370 + global + all MapPOIs) → no warning

---

### `validateKnownEventFlags(context string, ids []uint32, warnings *[]string)` ✅ Implemented

Generic safety-net for event flag lists without a dedicated per-database validator.

**Checks:**
- Duplicate IDs → `warning: <context>: ID %d appears more than once — duplicate will be ignored`
- IDs ≥ 1 000 000 000 (outside all known EventFlag address space) → `warning: <context>: ID %d is outside all known EventFlag ranges (>= 1,000,000,000) — likely invalid`

**Where it lives:** `backend/vm/preset.go` as `validateKnownEventFlags` (private). Called from `validatePresetModules` for fields without a dedicated validator.

**Applied to:** `World.Bosses`, `World.Cookbooks`, `World.BellBearings`, `World.Whetblades`, `World.WorldPickups`.

**Intentionally excluded:**
- `World.SummoningPools` — `validateWorldSummoningPools` uses a lower threshold (≥ 1 000 000); applying the generic validator would double-warn IDs like `1035530040`
- `World.Graces`, `World.MapFlags`, `World.Colosseums` — have dedicated DB validators
- `World.Regions` — not EventFlags; writes to `UnlockedRegions` binary struct
- `World.Gestures` — gesture slot IDs, not EventFlag IDs

**Design note:** The generic validator does NOT check "is this ID known in the database" for the fields it covers. That would require a dedicated DB lookup per field and would cause false positives for valid game IDs not yet in the database. Only clearly broken inputs (duplicates, extreme IDs) are flagged.

**Tests (8/8 passing, via `ValidatePreset` integration):**
- Duplicate boss ID → warning
- Boss ID ≥ 1B → warning
- Unknown but non-extreme boss ID → no warning
- Duplicate cookbook ID → warning
- WorldPickup ID ≥ 1B → warning
- SummoningPool pre-v1.12 ID (`1035530040`, ≥ 1B) → only "pre-v1.12" warning, NOT generic ">= 1B" (no double-warning)
- Empty `WorldPresetData` → no warnings
- Multiple invalid fields → all respective warnings aggregated

---

### `validatePresetModules(world *WorldPresetData, warnings *[]string)` ✅ Implemented

Orchestrator for all world preset validators. Replaces the inline validator calls in `ValidatePreset`, which now calls only `validatePresetModules` when `preset.World != nil`.

**Execution order (deterministic):**
1. `validateWorldSummoningPools` — dedicated, threshold ≥ 1 000 000
2. `validateWorldRegions` — dedicated DB lookup
3. `validateWorldGraces` — dedicated DB lookup
4. `validateWorldMapFlags` — dedicated DB lookup (4 map data maps)
5. `validateWorldColosseums` — dedicated DB lookup + MapPOI + global flag check
6. `validateKnownEventFlags("world.bosses", ...)` — generic safety-net
7. `validateKnownEventFlags("world.cookbooks", ...)` — generic safety-net
8. `validateKnownEventFlags("world.bellBearings", ...)` — generic safety-net
9. `validateKnownEventFlags("world.whetblades", ...)` — generic safety-net
10. `validateKnownEventFlags("world.worldPickups", ...)` — generic safety-net

**Excluded from all validation:** `World.Regions` (dedicated), `World.Gestures` (not EventFlags).

---

## 5. UI / UX Proposal

Current UI: single "Apply World Preset" checkbox with no granularity.

**Proposed:** Replace with a module checklist inside the WorldTab or Preset apply dialog.

```
PvP Preparation
──────────────────────────────────────────────────────────────
[✓] Module A — Stats & Equipment            risk: Tier 1
[✓] Module B — Unlock Invasion Regions      risk: Tier 1  ← primary effect
──────────────────────────────────────────────────────────────
[ ] Module C — Varre Quest Shortcut         risk: Tier 1
[ ] Module C — Unlock Colosseums            risk: Tier 1
──────────────────────────────────────────────────────────────
[ ] Module D — Reveal Map                   risk: Tier 0  (visual only)
[ ] Module D — Unlock Sites of Grace        risk: Tier 0  (fast-travel only)
──────────────────────────────────────────────────────────────
[ ] Module E — Activate Summoning Pools     risk: Tier 1
                (co-op; Bloody Finger invasion impact unconfirmed)
──────────────────────────────────────────────────────────────
[ ] Module F — NetworkParam Inspector       risk: Tier 2  (research)
[ ] Module F — BF State Classifier          read-only diagnostic
[ ] Module F — Candidate Section Viewer     read-only diagnostic
```

**Design rules:**
- Modules A + B are default-on when opening the "PvP Preset" flow
- Each module shows its risk tier badge and a one-line description
- Module F items are collapsed behind an "Advanced / Research" accordion
- Summoning Pool module (E) has an inline note: "Activates co-op summon signs.
  Effect on Bloody Finger invasion rate is unconfirmed."
- Colosseum sub-module (C) shows: "Unlocks colosseum matchmaking and map markers.
  Physical gate must be opened once in-game."
- Sites of Grace sub-module (D) shows: "Unlocks map markers and fast-travel.
  Some graces may still play activation animation on first visit." (from spec/47)

**What we do NOT remove:**
- Existing individual flag toggles in WorldTab (Graces accordion, Summoning Pools accordion, etc.)
- Existing `ApplyWorldState` function — the modular UI is built on top of it
- Existing preset export/import — `WorldPresetData` format is unchanged

---

## 6. Product Direction Note for spec/46

> To be appended to `spec/46-faster-invasions-research.md` as a new final section.

**Product direction / SaveForge implications:**

The save-file investigation (spec/46) produced a clear verdict: no writable save-file
field exists that directly reduces invasion wait time. The practical path for PvP
preparation through save editing is:

- **Region unlocks** (Module B) — confirmed to control invasion eligibility and area
  visibility to invaders. This is the primary save-level PvP-relevant feature.
- **UD11 NetworkParam** — values survive in the PS4 file; runtime effect on invasion
  timing is unconfirmed. Expose as research-grade diagnostic (Module F), not as a
  primary PvP feature.
- **UD10 / UD0 session structures** — runtime-only state. Expose as read-only
  diagnostics (Module F). Not a patch target.
- **Summoning Pools** — co-op / summon feature. Bloody Finger invasion impact is
  unconfirmed. Do not present as invasion speed-up (Module E).
- **PvP-ready preset** — must become modular (this spec). A single opaque preset
  conflates confirmed effects (regions) with unconfirmed ones (summoning pools,
  UD11) and visual-only changes (map, graces).

---

## 7. Implementation Phases

| Phase | Scope | Status |
|---|---|---|
| Phase 1 | `ValidateWorldRegions`, `ValidateWorldColosseums`, `ValidateWorldMapFlags`, `ValidateWorldGraces` + `validateKnownEventFlags` (generic) + `validatePresetModules` (orchestrator) + wire all into `ValidatePreset` | ✅ Complete |
| Phase 2 | UI module checklist in WorldTab / Preset apply dialog | 🔲 Planned |
| Phase 3 | Module F: BF state classifier read-only display (UD10 + UD0 reader) | 🔲 Planned (blocked: spec/46 §7) |
| Phase 4 | Module F: UD11 NetworkParam inspector UI | 🔲 Planned (blocked: spec/44) |
| Phase 5 | Varre quest shortcut preset step (Module C) | 🔲 Planned (blocked: spec/38 pattern) |

**Phase 1 is complete.** `validateKnownEventFlags` and `validatePresetModules` were consolidated into Phase 1 (originally planned as Phase 2) since they depend only on Phase 1 validators.

---

## Sources

| File | Relevance |
|---|---|
| `backend/vm/preset.go` | `WorldPresetData`, `ApplyWorldState`, `ValidatePreset` |
| `backend/db/data/regions.go` | 104 region IDs with name + area group (62 overworld + 7 DLC + 35 legacy dungeons) |
| `backend/db/data/summoning_pools.go` | 199 summoning pool IDs + `Colosseums` + `ColosseumFlagSets` |
| `backend/db/data/graces.go` | 419 grace entries with EventFlag IDs + DoorFlags |
| `backend/db/data/quests.go` | `"White Mask Varre"` — 19 quest steps with flags |
| `app_world.go` | All World-tab backend functions |
| `spec/46-faster-invasions-research.md` | Final verdict: UD11 patch has no confirmed runtime effect |
| `spec/47-site-of-grace-activation.md` | Grace unlock confirmed for map/fast-travel; in-world animation open |
| `spec/44-network-param-tuning.md` | NetworkParam field reference |
| `spec/32-ban-risk-system.md` | Tier 0/1/2 definitions |
| `tmp/coloseum-debug/` | Hex-verified colosseum flag sets |
