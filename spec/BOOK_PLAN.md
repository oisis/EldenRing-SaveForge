# BOOK_PLAN — `spec/` as the Modder's Handbook

> **Purpose**: roadmap for consolidating `spec/` into a coherent book. The source of truth is the code in `backend/`, `app*.go`, `frontend/src/` plus the current audit `tmp/docs-book-audit.md` (local, gitignored).
>
> **Current phase**: Phase 4 ✅ completed for the main chapters (Map / World / Event Flags / Game State — canonical rewrites 11, 14, 15, 16, 17, 19, 27, 29, 47, 48, 50). Phase 5 (Ban-risk + unsafe edits + validation/safety consolidation) — next.
>
> **Format**: every planned chapter will eventually use the template from [Chapter template](#chapter-template) (at the end of this file).

---

## Target table of contents

After all phases, `spec/` should consist of 30 chapters + 7 appendices, divided into five parts.

### PART I — Orientation

| Ch | Chapter title | Source documents |
|---|---|---|
| Ch.0 | Why this book exists (new) | — |
| Ch.1 | Container & Platforms | 01, 20, 49 |
| Ch.2 | Slot anatomy & sequential parsing | 02, 25 |

### PART II — Binary Format Reference

| Ch | Chapter title | Source documents |
|---|---|---|
| Ch.3 | GaItem Map | 03 (layout), 14 (GaItem Game Data); AoW semantics → Ch.7 |
| Ch.4 | Inventory & Storage model | 07, 10 |
| Ch.5 | Player attributes (PlayerGameData) | 04 + attributes from 26 |
| Ch.6 | Equipment & Active weapon slots | 06, 08 |
| Ch.7 | Ash of War | 54 + AoW pieces from 03/06 |
| Ch.8 | Appearance & Face Data | 09 + 31 |
| Ch.9 | Event Flags | 15 |
| Ch.10 | World, Weather, Coordinates | 16, 17, 19 |
| Ch.11 | Map Reveal — 4 layers | 27 + 11 (regions) + part of 29 |
| Ch.12 | Sites of Grace | 47 + 14 §LastRestedGrace |
| Ch.13 | Game State, NG+, tutorials | 14 |
| Ch.14 | Network Manager (slot-local) | 18 |
| Ch.15 | DLC flags & version | 20, 21 |
| Ch.16 | Player Data Hash & trailing | 22 |
| Ch.17 | UserData10 — account, presets, summaries | 23 |
| Ch.18 | UserData11 — regulation.bin (read-only) | 24 |

### PART III — Editor Internals (how SaveForge does it)

| Ch | Chapter title | Source documents |
|---|---|---|
| Ch.19 | Transactional item adding | 43 + 50 (companion flags) |
| Ch.20 | Acquisition sort (stride-2) | 52 |
| Ch.21 | Inventory ↔ Storage transfer | 53 |
| Ch.22 | Item caps & NG+ scaling | 34 |
| Ch.23 | Inventory categories (game-accurate) | 36 |
| Ch.24 | Slot rebuild & post_unlocked_regions blob | 30 (condensed) + engineering note |
| Ch.25 | Build Templates (JSON export/import) | 55 |

### PART IV — Platform Tuning & PvP

| Ch | Chapter title | Source documents |
|---|---|---|
| Ch.26 | NetworkParam tuning | 44 |
| Ch.27 | PS4 ZSTD raw-block patch | 49 |
| Ch.28 | Modular PvP presets (current scope) | 48 (trimmed — without planned modules) |

### PART V — Runtime Observations

| Ch | Chapter title | Source documents |
|---|---|---|
| Ch.29 | Runtime vs save — using Cheat Engine alongside | 25 |
| Ch.30 | Boss defeat — current 1-flag model | 14 §boss + description of the current `SetBossDefeated` |

### APPENDICES

| App | Title | Source documents |
|---|---|---|
| A | Ban-risk reference | 45 |
| B | Ban-risk UI tier system | 32 |
| C | Verification methodology | 99 |
| D | Parameter softcaps & quick reference | 26 (remainder after extracting attributes to Ch.5) |
| E | Research log — negative results | 30 (full), 42 |
| F | Planned features | 37, 38 |

**Consolidation**: 47 documents → 30 chapters + 6 appendices (1.6×).

---

## Further work phases

### Phase 1 — Directory reorganization + README ✅ COMPLETED

- **Purpose**: separate reference from research-, planned-, and archive-docs without content changes.
- **Scope (historical)**: creation of `research`, `planned`, `archive` subdirectories; `git mv` of 8 documents; rewrite of `README.md` + new `BOOK_PLAN.md`.
- **Acceptance**: the main Polish specification directory contains only documents that are candidates for chapters; no dangling links; vanilla MD5 untouched. ✅
- **Effort**: 2–3 h.
- **Commit**: `1c2fbab docs(lang-pl): reorganize book structure`.
- **Cleanup in Phase 4+** (2026-05): the `research`, `planned`, `archive` subdirectories were removed. Three active documents (38, 30, 42) returned to the root of the Polish documentation tree; the remaining ones (33, 40, 41, 46, 51) were deleted as obsolete.

### Phase 2 — Inventory + Storage + GaItem + Equipment ✅ COMPLETED (main chapters)

- **Purpose**: consolidate 03, 06, 07, 10, 35, 36, 39, 43, 52, 53 into a coherent canonical narrative.
- **Completed**:
  - **Step 1**: `35-gaitem-allocator-invariants.md` — new canonical chapter (handle allocation, capacity invariants). Commit `9f25084 docs(lang-pl): document GaItem allocator invariants`.
  - **Step 2**: `03-gaitem-map.md` — rewrite as canonical (GaItem layout + GaMap, AoW moved to cross-ref to 54). Commit `4f616c8 docs(lang-pl): rewrite GaItem map model`.
  - **Step 3**: `07-inventory.md` + `10-storage.md` — both rewritten as canonical (read-side 12B record). Commit `398d2a4 docs(lang-pl): rewrite inventory and storage models`.
  - **Step 4**: `43-transactional-item-adding.md` — canonical refresh (pre-flight + `SnapshotSlot`/`RestoreSlot` + `ValidatePostMutation`). Commit `7ff576f docs(lang-pl): rewrite transactional item adding`.
  - **Step 5**: `52-acquisition-sort-stride2.md` + `39-inventory-reorder.md` — 52 canonical (stride-2 + 3 write paths); 39 as historical/superseded design note. Commit `91291e7 docs(lang-pl): document acquisition stride sort order`.
  - **Step 6**: `53-inventory-storage-transfer.md` — canonical (two paths: legacy core + workspace; rehandle; equipped guard; SortOrderTab workspace UI). Commit `e3ee3aa docs(lang-pl): rewrite inventory storage transfer`.
  - **Step 7**: `36-inventory-categories-game-order.md` — canonical (18 DB tabs + handle prefix bridge + 76 sub-categories + DLC flag mechanism). Commit `e1ce20d docs(lang-pl): rewrite inventory category mapping`.
  - **Step 8**: `06-equipment.md` — canonical, read-only model (`EquippedItemsItemIds` + `EquippedGreatRune`, no public write API, hypothetical 3 structures from er-save-manager → Historical notes). Commit `84b7daf docs(lang-pl): rewrite equipment reference`.
- **Acceptance**: all 10 chapters use the canonical template, cross-refs between them, source-of-truth in code with line numbers, `needs verification` markers where code does not confirm 100%.
- **Actual effort**: ~10 commits on branch `docs/lang-pl-book-cleanup`.

#### Cross-cutting gaps from Phase 2 (to address in the future)

- Storage Apply in-game / Steam Deck verification (common gap for 52 and 53).
- Workspace path equipped guard (no explicit `IsHandleEquipped` check in `editor.ApplyWorkspaceSave`).
- Workspace post-mutation validation (no `ValidatePostMutation` equivalent to 43).
- UI counters vs allocator capacity end-to-end cross-check.
- DLC sub-mapping completeness (best-effort `melee_subcat.go` curated lookup).
- Equipment write API — **not implemented** (read-only). `EquippedGreatRune` round-trips, but no public setter.
- Hash recompute discipline — `RecalculateSlotHash` called only in tests.
- Game order in-game verification for the current game version (last verification April 2026).

### Phase 3 — Ash of War + Build Template (Ch.7, Ch.25) ✅ COMPLETED (main chapters)

- **Purpose**: close the AoW topic (54 + AoW guard from commit `6881cb9` + invariant from 03/35 + UI from `WeaponEditModal.tsx`) and document Build Template (55).
- **Completed**:
  - **Step 1**: `54-ash-of-war.md` — canonical rewrite (sentinels 0x00/0xFFFFFFFF, strict vs allocate+rebuild write paths, AoW Allocation Safety guard from commit `6881cb9`, shared-handle invariant, ScanAoWAvailability 2-pass, workspace/WeaponEditModal state, fail-closed compat on unknown wepType, DLC wepType 69/94/95 allow-passthrough, frontend `WEP_TYPE_TO_BIT` mirror drift, 8 explicit needs-verification items). Commit `e3a634f docs(lang-pl): rewrite ash of war reference`.
  - **Step 2**: `55-build-template.md` — canonical rewrite (Build Template JSON v1 schema, portable payload rule without save-local handles, AoW relation with fail-closed compat, capacity preflight `CommonItemCount=2688` / `StorageCommonCount=1920`, RAM-only apply with `deepCopySnapshot` rollback, Phase E local library `$UserConfigDir/EldenRing-SaveEditor/templates/` with atomic writes + `_index.json` with `LibraryIndexVersion=1`, 110 Go tests, 12 needs-verification items). Commit `a2e455c docs(lang-pl): rewrite build template reference`.
- **Acceptance**: 54 contains the "what happens when allocateGaItem returns error" table (§15 AoW Allocation Safety) + cross-ref to 55 for Build Template apply; 55 has the full JSON v1 schema + portable payload rule + export/import/apply example + cross-ref to 54 for AoW relation. Both chapters without AoW semantics duplication.
- **Actual effort**: 2 commits on branch `docs/lang-pl-book-cleanup`.

#### Cross-cutting gaps from Phase 3 (to address in the future)

- **AoW affinity gating** — `EquipParamWeapon.defaultWepAttr` / `configurableWepAttr00..23` are not imported into `WeaponGemMounts`. Build Template preview validates compat only by `wepType`, not by infusion variant. (54 §22.L1, 55 §21.L1)
- **DLC wepType gaps (69/94/95)** — backend allow-passthrough; UI fail-closes AoW section visibility; no user-facing information "DLC, compatibility unknown". (54 §22.L2)
- **`gemMountType == 1` semantics** — `CanMountAoW = false` disables the AoW section, but no placeholder/explanation in UI. (54 §22.L3)
- **`AoWCompatMasks` completeness after regulation update** — bitmask generated from `EquipParamGem`; new DLC rows may not be re-imported. (54 §22.L5)
- **Orphan AoW GaItem GC / save bloat** — the allocator does not release handles after AoW reset; save grows linearly with the number of AoW edits. (54 §22.L6)
- **Build Template equipment write API** — ❌ not implemented; apply leaves weapons unequipped. (55 §12, §21.L3)
- **Build Template spell loadout / character stats** — schema v1 does not export attunement slots or PlayerGameData stats. (55 §6)
- **Build Template forward-compat `version=2` tests** — `SchemaVersion=1` is the only accepted version; no tests for unknown-future-fields scenarios. (55 §18, §21.L8)
- **Cross-platform PS4 vs PC portability for Build Template** — schema portable by design, but no E2E PS4↔PC roundtrip test.
- **Frontend/backend `WEP_TYPE_TO_BIT` drift** — single frontend mirror (`WeaponEditModal.tsx`) vs backend, no CI / generator guard. (54 §17, §22.L4)
- **`replace-*` modes not implemented** — `replace-weapons`, `replace-armors`, etc., reserved in the schema; v1 supports only `merge`. (55 §6)

### Phase 4 — Map / World / Event Flags / Game State (Ch.9, Ch.10, Ch.11, Ch.12, Ch.13, Ch.30) ✅ COMPLETED (main chapters)

- **Purpose**: gather everything about map, flags, and game state into one coherent section; resolve conflicts F6 (27/11), F7 (13/29), and F9 (48 overclaim).
- **Completed** (10 separate commits on branch `docs/lang-pl-book-cleanup`):
  - **Step 1**: `15-event-flags.md` — canonical rewrite (3-tier resolver: precomputed → BST → fallback formula, helper API, 4 tests). Commit `316066e docs(lang-pl): rewrite event flags reference`.
  - **Step 2**: `27-map-reveal.md` — canonical master rewrite (4-layer model L0–L3, MapVisible 263 entries, MapSystem 4 entries, MapFragmentItems 24 entries, RevealAllMap/RemoveFogOfWar/ResetMapExploration). Commit `5c962a9 docs(lang-pl): rewrite map reveal reference`.
  - **Step 3**: `11-regions.md` — canonical detail for L0 (104 regions: 35 legacy + 62 overworld + 7 DLC, 11 unique Area values, RebuildSlot relation). Commit `0a3e3d7 docs(lang-pl): rewrite regions reference`.
  - **Step 4**: `29-dlc-black-tiles.md` — canonical detail for L2 (DLCTile* constants from `offset_defs.go:309-322`, synthetic coords 9648/9124 and 3037/1869/7880/7803, Phase 3 in revealDLCMap). Commit `0b59e87 docs(lang-pl): rewrite dlc black tiles reference`.
  - **Step 5**: `47-site-of-grace-activation.md` — canonical rewrite (419 entries, Grace EventFlag 71xxx-76xxx + DoorFlag + companion flags SET-only Gatefront 76111, 6 integration tests). Commit `f96ce6e docs(lang-pl): rewrite site of grace reference`.
  - **Step 6**: `50-item-companion-flags.md` — canonical rewrite (SET+CLEAR symmetric, 6 entries: Whistle + 5 multiplayer items, hook SET in AddItemsToCharacter lines 569-578, hook CLEAR in RemoveItemsFromCharacter lines 706-725, 11 unit + 17 integration tests). Commit `a1b8422 docs(lang-pl): rewrite item companion flags reference`.
  - **Step 7**: `48-pvp-ready-modular-presets.md` — current reference rewrite (5 modules: 4 active + 1 placeholder, single pushUndo, fail-fast without auto-restore, Sites of Grace module E placeholder explicit). Commit `b25fbd2 docs(lang-pl): rewrite pvp modular presets reference`.
  - **Step 8**: `16-world-state.md` — overview/index rewrite (subsystem map 11 rows, read-only verbatim blobs vs write-capable via bitfield, WorldGeomBlock corruption risk). Commit `5a00cdd docs(lang-pl): rewrite world state overview`.
  - **Step 9**: `17-player-coordinates.md` + `19-weather-time.md` — read-only refresh (**17 fix: 57→61 B as `12+4+16+1+12+16`**, no setters; 19 no setters, removed stale corruption heuristics). Commit `d7228a5 docs(lang-pl): refresh coordinates weather and time references`.
  - **Step 10**: `14-game-state.md` — canonical rewrite (PreEventFlagsScalars 29 B with 11 fields, ClearCount write path via SaveCharacter + NG+ event flag sync 50-57, LastRestedGrace read-only, boss multi-flag → 38-boss-multiflag.md). Commit `5c729a7 docs(lang-pl): rewrite game state reference`.
- **Acceptance**: all 10 chapters use the canonical template, cross-refs between them without duplication, source-of-truth in code with line numbers, `needs verification` markers where code does not confirm 100%, no overclaims (the most important fixes: `PlayerCoordinatesSize 57→61 B` in 17, "Phase 1 complete" → "4 active + 1 placeholder" in 48, "SaveCharacter has no pushUndo" → "SaveCharacter has pushUndo" in 14).
- **Actual effort**: 10 commits on branch `docs/lang-pl-book-cleanup`.

#### Cross-cutting gaps from Phase 4 (to address in the future)

- **Stale generated snapshots after game / regulation.bin patch** — `data.Graces/Regions/MapVisible/SummoningPools/ColosseumFlagSets/itemCompanionEventFlags` are static; no auto-detection.
- **PS4 ↔ PC parity tests** — round-trip covered; no per-endpoint platform parity.
- **No cross-subsystem atomic transaction** — orchestrators use a single pushUndo without per-phase rollback.
- **Manual undo / rollback limits** — undo stack depth unknown; bulk operations create N separate snapshots.
- **In-game runtime verification gaps** — no CI in-game loop.
- **Event flag ID correctness after patch** — 3-tier resolver fallback for new flags.
- **Map reveal visual vs progression effects** — `WorldTab.tsx:406` UI note; trophy impact unverified.
- **DLC black tile coordinates stale-after-patch** — empirical values, not game-guaranteed.
- **Sites of Grace SET-only intent + PvP module E placeholder** — SET-only companion flags in grace lifecycle; PvP module returns a warning.
- **Item companion flag IDs stale-after-patch** — 6 hardcoded literals.
- **PvP "ready" scope limited** — physical colosseum gates in WorldGeomMan blob non-editable; Summoning Pools "Bloody Finger impact unconfirmed".
- **Player Coordinates / Weather-Time read-only** — no public setters, any mutation through direct hex edit.
- **Game State: LastRestedGrace read-only** — ClearCount is the only write path via `SaveCharacter` + flag sync 50-57; no progression consistency validator.
- **Boss multi-flag editor remains planned** — single-flag `SetBossDefeated` currently; multi-flag in `38-boss-multiflag.md`.

### Phase 4 Step 11 — index update (current)

- **Purpose**: update README.md + BOOK_PLAN.md after Phase 4.
- **Scope**: README.md (status header, Phase 4 cross-cutting gaps, table-of-contents entries), BOOK_PLAN.md (Phase 4 ✅ COMPLETED with the list of Step 1-10 commit hashes, resolved conflicts F6/F7/F9, merge/rewrite candidates 29/48 marked resolved).
- **Acceptance**: README and BOOK_PLAN reflect the Phase 4 state; no overclaims; links valid.

### Phase 5 — Ban-risk + unsafe edits reference (App. A, App. B) — NEXT

- **Purpose**: stabilize the appendices with the ban-risk reference + consolidate the safety/validation/rollback knowledge scattered across Phase 2-4 chapters.
- **Files**: 45 → App. A (community triggers); 32 → App. B (UI tier system); cross-refs to every chapter with Tier 1/2 edits.
- **Code**: no new — sync with `frontend/src/state/safetyMode.tsx`, `RISK_INFO` in `components/Risk*.tsx`.
- **Acceptance**: every Part III/IV chapter has a "Ban-risk / safety notes" footer with a link to App. A; centralized safety/validation/rollback index with cross-refs to `Phase 2-4` safety notes.
- **Effort**: 3–4 h.

### Phase 6 — Glossary, code-to-doc index, offset index

- **Purpose**: add two new reference files.
- **Files**: `spec/INDEX.md` (hex offsets → chapters); `spec/GLOSSARY.md` (GaItem, AoW, ClearCount, Mirror Favorites, Acquisition Index, ...).
- **Acceptance**: every hex offset ≥ 0x100 in any chapter has an INDEX entry; every term unique to 5+ documents is in GLOSSARY.
- **Effort**: 4–6 h.

**Total effort**: ~35–45 h. Each phase requires a separate branch + separate review.

---

## Documents for later merge / rewrite

A list of documents that remained in the main directory after Phase 1 but need intervention in later phases.

### Merge candidates (to merge with another chapter)

| Doc | Target chapter | Reason | Status |
|---|---|---|---|
| ~~03 §AoW~~ | Ch.7 (from 54) | AoW semantics duplication | ✅ Phase 2 Step 2 + Phase 3 Step 1: cross-ref to 54, no duplication; AoW consolidation closed on the 54 side |
| ~~10~~ | Ch.4 §4.2 (from 07) | identical format as 07, different counters | ✅ Phase 2 Step 3: 10 retained as a separate canonical (both rewritten) |
| ~~54 §AoW write paths / availability / compat~~ | Ch.7 | scattered between 03/06/35/UI components | ✅ Phase 3 Step 1: 54 is the single source of truth for strict vs allocate+rebuild write paths, ScanAoWAvailability, fail-closed compat, AoW guard |
| ~~55 §Build Template portable payload + Phase E library~~ | Ch.25 | scattered in backend/templates + app_templates + frontend templates | ✅ Phase 3 Step 2: 55 covers JSON v1 schema, portable rule, capacity preflight, RAM-only apply + Phase E local library |
| 18 | Ch.14 (with disclaimer) | one short opaque blob, not worth its own chapter | still open |

### Rewrite candidates (to rewrite in canonical template)

| Doc | Reason | Status |
|---|---|---|
| 05-sp-effects | short, "needs verification" in content — either read `structures.go` or mark as a stub | still open |
| ~~07-inventory~~ | solid facts, chaotic order | ✅ Phase 2 Step 3 |
| 09-face-data | most fields "approximate"; `app_appearance.go` knows more | still open |
| 26-parameter-reference | split: attributes → Ch.5, softcaps → App. D | still open |
| ~~29-dlc-black-tiles~~ | split: spec → Ch.11; research log → App. E | ✅ Phase 4 Step 4: canonical detail for L2 DLC Cover Layer; historical binary search test 7-18 moved to `docs/CHANGELOG.md` cross-ref. |
| ~~39-inventory-reorder~~ | status out of date — code exists, stride-2 discovered (see conflict F2) | ✅ Phase 2 Step 5: historical/superseded note |
| ~~48-pvp-ready-modular-presets~~ | split: design+implemented (Ch.28), planned (App. F) | ✅ Phase 4 Step 7: current reference rewrite; 4 active modules + 1 placeholder (Sites of Grace module E) clearly documented without overclaim. |

---

## Doc vs code conflicts (audit summary)

Full list in `tmp/docs-book-audit.md` § F. Summary:

| # | Doc | Claim from the document | Reality from the code |
|---|---|---|---|
| F1 | 38 | `BossData{ EventFlags []uint32 }` + multi-flag boss kill | `backend/db/data/bosses.go:4` — only `Name/Region/Type/Remembrance`; `app_world.go:113 SetBossDefeated` accepts a single `bossID` |
| F2 | 39 | Status: `🔲 Planned — blocked in Phase 0` | ✅ **Resolved in Phase 2 Step 5**: 39 is a historical/superseded design note; canonical mechanics in 52, transfer UX in 53. |
| F3 | 37 | Status: `🔲 Planned` | `backend/vm/preset.go` has `CharacterPreset/VMToPreset/PresetToVM/ValidatePreset` — `needs verification` per Phase |
| F4 | 03/54 | AoW sentinels + handle uniqueness invariant | ✅ **Resolved in Phase 2 Step 2 + Phase 3 Step 1**: 03 has a cross-ref to 54, 54 is the single source of truth for both sentinels (`NoCustomAoWHandle = 0x00000000` canonical, `LegacyNoCustomAoWHandle = 0xFFFFFFFF`) + shared-handle invariant + AoW Allocation Safety guard. |
| F5 | 33/36 | 33 declares Information tab reclassification | ✅ **Resolved in Phase 1 + Phase 2 Step 7**: 33 deleted (post-mortem archived in git history); 36 canonical (handle prefix bridge + sub-categories + DLC flag mechanism). |
| F6 | 27/11 | Both sections cite `core.SetUnlockedRegions` | ✅ **Resolved in Phase 4 Step 2 + Step 3**: 27 is the master for the 4-layer Map Reveal; 11 is the L0 detail and links to 27 for orchestration. No duplication. |
| F7 | 13/29 | 13: fields `unk_0x1c..0x40` as Unknown | ✅ **Partially resolved in Phase 4 Step 4**: 29 is the canonical detail for L2 DLC Cover Layer in `BloodStain` (range `[0x0088..0x0110)`); 13 remains `partial` with a reference (rewrite 13 → Phase 5+). |
| F8 | 14/34 | TODO about ClearCount increment | ✅ **Partially resolved in Phase 4 Step 10**: 14 documents the ClearCount write path via SaveCharacter + NG+ flag sync 50-57; cap @ 7; TODO about auto-increment still open (intentionally). |
| F9 | 48 | Status: `✅ Phase 1 complete` | ✅ **Resolved in Phase 4 Step 7**: 48 status changed to `current reference`; module status table clearly shows 4 active + 1 placeholder; Sites of Grace module E explicit warning. |
| F10 | 51 | Status: `🔲 Planned`, "UI Layout" section | `grep AdvancedSaveEditor` → 0 results; pure design doc |
| F11 | 05 | SpEffect layout 16 B (id/duration/unk1/unk2) + "entry count needs verification" | No `parseSpEffects` in `backend/core/` — the section is an opaque blob in the write path |
| F12 | 09 | Size 303 B, fields 0x20+ "approximate" | `db/data/presets_generated.go` (4 KB+) describes whole presets — internal structure known better in code than in spec |
| F13 | 26 | "Complete reference of all editable parameters" | Covers mostly attributes + softcaps; does not cover the full `RuneArc`, `Crucible Aspect`, `Scadutree Blessing` |

**Owner decision**: each conflict requires a "update doc to match code" or "update code to match doc" decision. The auditor did not make these decisions — they are tasks for Phase 2-5.

---

## Chapter template

Every final book chapter must use the following section order:

````markdown
# Ch.N — Title

> **Part**: I / II / III / IV / V
> **Status**: ✅ Implemented | 🔲 Planned | 🐛 Research | 📚 Reference
> **Code-of-record**: `backend/core/<file>.go`, `app_<feature>.go`
> **Tests-of-record**: `backend/core/<file>_test.go`, `tests/<file>_test.go`
> **Source docs (pre-merge)**: NN-foo.md, NN-bar.md

---

## Purpose
One or two sentences: why this save section / editor function exists. Why the reader is here.

## Status
- What is verified (✅), what is cross-reference only (⚠️), what is uncertain (❓).
- Last verification: <date> on <save path>.

## Source of truth in code
List of functions + files with links to specific lines (e.g., `writer.go:430 allocateGaItem`).
This is the section the reader clicks when they want to see ground truth.

## Binary layout / data model
Hex offset table with types and descriptions. ASCII diagram where it helps. Quote sizes in hex + dec.

## Read path
How the editor reads this section (or how the game interprets it). Point to `reader.go` / `structures.go`.

## Write path
How the editor writes, what invariants it maintains, what counters it updates. Point to `writer.go` / `editor/save.go`.

## Validation / invariants
List of rules that MUST be satisfied (e.g., "NextEquipIndex > every acquisition_index in slot").
With consequences of non-compliance (crash, invisible items, ban risk).

## Tests
List of tests verifying the invariants. Command to run, expected result.

## Known limits
What does NOT currently work, what is TODO, what the editor does not handle. No hiding.

## Ban-risk / safety notes
If editing this section has a risk level — Tier 0/1/2 from spec/45 + spec/32. Link to App. A.

## Historical notes (optional)
If there was research → discovery → implementation, a short how-we-got-here. Otherwise skip.

## See also
Links to other chapters + related research or planned docs in the root of `spec/`.

## Sources
- Reference parsers (er-save-manager, ER-Save-Editor)
- Cheat tables
- Community wiki / Fextralife
- Hex-verified save files (`tmp/save/...`)
````

**Benefits**: every chapter has the same skeleton, the reader knows where to look. "Code-of-record" and "Tests-of-record" are a hard contract — when someone changes the code, it's easy to see which chapter to update.
