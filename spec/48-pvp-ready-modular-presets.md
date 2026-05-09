# 48 — PvP-Ready Modular Presets

> **Type**: Design doc
> **Status**: 🔲 Planned
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

### `ValidateWorldRegions(ids []uint32) []string`

**Checks:**
- Each ID is present in `data.Regions` map → `warning: unknown region ID`
- Flags IDs ≥ 6 900 000 (DLC) when applied to a non-DLC context → `warning: DLC region`
- Detects duplicate IDs → `warning: duplicate region ID`

**Returns:** warning (not error). Region lock/unlock is reversible.

**Where it lives:** `backend/vm/preset.go` alongside `validateWorldSummoningPools`.

**Tests:**
- Known ID → no warning
- Unknown ID (e.g., `9999999`) → warning
- DLC ID (`6900000`) → warning when DLC flag not set in context
- Duplicate → warning

---

### `ValidateWorldGraces(ids []uint32) []string`

**Checks:**
- Each ID is present in `data.Graces` map → `warning: unknown grace ID`
- Flags IDs outside `71xxx–76xxx` range as suspicious

**Returns:** warning.

**Tests:**
- Known grace (`76100`) → no warning
- Unknown ID (`99999`) → warning

---

### `ValidateWorldMapFlags(ids []uint32) []string`

**Checks:**
- Each ID is in known map flag ranges (`62xxx` or `82xxx`)
- Cross-references against `data.Maps` if available
- Flags IDs outside expected ranges

**Returns:** warning.

---

### `ValidateWorldColosseums(ids []uint32) []string`

**Checks:**
- Each ID is in `data.Colosseums` map
- For each present Activate flag, checks that companion flags (`MapPOI`, `NPC`, `Gate`)
  are also present in the preset's `MapFlags` → `warning: colosseum X missing companion flags`
- Flags presence of `ColosseumGlobalFlags` (`6080, 60100, 69480`)

**Returns:** warning.

**Why:** Colosseum with only the Activate flag set may show in matchmaking but not on the
map, or have the gate visually closed. The full `ColosseumFlagSet` is required.

**Tests:**
- Full `ColosseumFlagSet` for Limgrave → no warning
- Activate-only → warning: "missing MapPOI, NPC, Gate"

---

### `ValidateKnownEventFlags(ids []uint32, context string) []string`

Generic validator for any event flag list:

**Checks:**
- Flags ≥ 1 000 000 000 (outside all known EventFlag blocks) → `warning: ID outside all known ranges`
- Flags matching pre-v1.12 summoning pool pattern (≥ 1 000 000 AND context == "summoningPools") → defer to `validateWorldSummoningPools`
- Flags in unknown block ranges (not in `62xxx`, `71xxx–76xxx`, `670xxx`, `60xxx`, `82xxx`) → `info: unclassified range`

**Returns:** warning/info.

---

### `ValidatePresetModules(preset *CharacterPreset) []string`

Orchestrator. Calls:
1. `validateWorldSummoningPools` (existing)
2. `ValidateWorldRegions`
3. `ValidateWorldGraces`
4. `ValidateWorldMapFlags`
5. `ValidateWorldColosseums`

Called from `ValidatePreset` when `preset.World != nil`.

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

| Phase | Scope | Blocked on |
|---|---|---|
| Phase 1 | Implement remaining validators (`ValidateWorldRegions`, `ValidateWorldColosseums`, `ValidateWorldMapFlags`, `ValidateWorldGraces`) + wire into `ValidatePreset` | Nothing |
| Phase 2 | Add `ValidateKnownEventFlags` generic validator | Phase 1 |
| Phase 3 | UI module checklist in WorldTab / Preset apply dialog | Phase 1 |
| Phase 4 | Module F: BF state classifier read-only display (UD10 + UD0 reader) | spec/46 §7 |
| Phase 5 | Module F: UD11 NetworkParam inspector UI | spec/44 |
| Phase 6 | Varre quest shortcut preset step (Module C) | spec/38 pattern for quest flags |

**Phase 1 is the minimum viable deliverable.** All subsequent phases build on it.

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
