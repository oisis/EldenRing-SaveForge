# ROADMAP

## Legend

| Symbol | Meaning |
|--------|---------|
| 🔴 | Critical / Safety |
| 🟡 | High priority |
| 🟢 | Medium priority |
| 🔵 | Low priority / Exploratory |

---

## Done

### v0.2 — Core
- Save file loading (PC `.sl2` + PS4 `memory.dat`), AES-128 decrypt/encrypt, MD5 checksums
- Character stats editing, inventory management (add/remove items), item database with icons
- SteamID patching, bidirectional PC ↔ PS4 conversion, backup manager

### v0.3–v0.4 — Event Flags & World State
- Event Flags parser (~840 precomputed + BST fallback for 14.7M flags)
- NPC Quest State Editor (36 NPCs, step-by-step progression)
- Boss Kill / Respawn Manager (~120 bosses, single defeat flag)
- Sites of Grace toggle (~460 graces), Unlock All with boss arena filter
- Map Exploration & Fog of War removal (unique feature — no other editor touches FoW)
- Summoning Pools (165), Colosseums (3), Invasion Regions (78 IDs via RebuildSlot)
- Dungeon entrance door auto-unlock (22/25 confirmed via binary diff RE)

### v0.5 — Inventory & Progression
- Cookbook/Recipe checklist, Gesture unlock (57 gestures), Bell Bearing sync (62 entries)
- AoW acquisition flags (116 entries), Great Rune manager, NG+ cycle editor (0-7)
- Talisman Pouch slots, Spirit Ash upgrade level editing
- Item Descriptions & Stats (sourced from ERDB regulation.bin)

### v0.6 — Safety & Categorization
- Ban-risk awareness system (Tier 0/1/2 + Online Safety Mode) → spec/32
- Item Caps enforcement + NG+ scaling + Full Chaos Mode → spec/34
- DB categorization audit (Information tab, 105 items reclassified) → spec/33
- Inventory & Item Database 1:1 game-aligned (18 tabs, ~70 sub-categories) → spec/36
- Container enforcement for Throwing Pots & Aromatics (Cracked/Ritual/Hefty/Perfume caps)

### v0.7 — Safety & Architecture
- RebuildSlotFull — crash-safe slot rebuild replacing FlushGaItems (DLC/Hash preservation)
- Transactional item adding (ALL-OR-NOTHING with pre-flight + snapshot/rollback) → spec/43
- Character Appearance Presets (Mirror Favorites write, 20+ presets, cross-gender) → spec/31
- SSH Deploy + remote game control (Steam Deck workflow) — target management, SFTP, launch/stop
- Undo/Redo (stack depth=5, per-slot deep copy, "Undo" button in sidebar)
- Save file diffing — "Review Changes" dialog before save (inventory, graces, bosses diff)

### v0.8 — Network/PvP
- Faster Invasions — NetworkParam break-in params (interval/timeout/targets) + regulation.bin pipeline (AES→DCX→BND4→PARAM)
- PvP/Networking tab — role-based tuning (Invader/Host/Cooperator/Blue) with presets → spec/44
- Auto-collect world pickups (Golden Seed, Sacred Tear, Scadutree Fragment, Revered Spirit Ash) — 117 pickup flags from ItemLotParam_map, auto-flagged on inventory add + manual Collect All in World Tab

### UI/UX Redesign ("Elden Ring SaveForge")
- Theme system (Dark/Light/Elden Ring), tab consolidation (7→5 tabs)
- Reusable AccordionSection, World Tab sub-tabs (Exploration/Progress/Unlocks)
- QuakeConsole + ToastBar, Inventory/Database split view, rebranding

### v0.8.x — Inventory integrity bugfixes
- **NextEquipIndex visibility gate** — binary write-back on load: saves with `NextEquipIndex < NextAcquisitionSortId` gap (from external editors) are corrected in-binary on open, so items added before are immediately visible in-game without re-adding
- **NextEquipIndex max-guard on add** — replaced `++` with `max(NextEquipIndex, acqIdx) + 1`: gap can never grow during an editor session regardless of pre-existing counter state
- **`common_item_count` header** — reconciled on load and updated on add/remove: fixes "inventory full" false-positive and wrong in-game insertion index after external editor sessions
- **Orphaned GaItem cleanup** — `RemoveItemFromSlot` clears GaItem binary record; `RepairOrphanedGaItems()` purges stale entries from pre-corruption saves
- **Round-trip tests** — `recalculatedRegions()` excludes intentionally-corrected bytes; `TestAcquisitionSortIdIncrementFix` rewrit to assert visibility invariant

---

## Planned

### 🟡 Character Preset Export/Import — JSON build sharing
Export/import stats + inventory + storage + appearance to portable `.preset.json`.
Replaces disabled `App.ImportCharacter`. Community build sharing.
**Design:** [spec/37](spec/37-character-presets.md) | **Effort:** 10-14h (Phase 1+2 MVP)

### 🟡 Inventory Custom Order — drag & drop grid reorder
Manipulate `acquisition_index` to set custom weapon/armor order visible in-game (Acquisition sort mode).
Unique feature — no existing editor offers this.
**Design:** [spec/39](spec/39-inventory-reorder.md) | **Effort:** 12-19h | **Blocker:** Phase 0 in-game verification

### 🟡 Boss Kill — multi-flag rework
Current single-flag approach grants runes but boss remains alive. Needs arena state + quest progression + grace activation flags per boss.
**Design:** [spec/38](spec/38-boss-multiflag.md) | **Reference:** `tmp/repos/er-save-manager/src/er_save_manager/data/boss_data.py` (208 bosses)

### 🟡 DB Cleanup — cut-content, multiplayer dedup, error items
Fix `[ERROR]` items in-game, remove empty flask duplicates, dedup multiplayer active/inactive states, flag uncertain cut-content.
**Design:** [spec/40](spec/40-db-cleanup-plan.md) | **Effort:** 4-6 iterations

### 🟡 Bell Bearing Merchant Kill Flag
Auto-set merchant NPC kill event flag when their Bell Bearing is unlocked. Prevents duplicate drops and broken dialogue.
**Investigation needed:** map merchant BB item ID → NPC kill flag (overlaps with quest_flags_db.py)

### 🟡 Info-tab ground drop — investigation paused
Adding Info items (Notes, About tutorials) causes world copies to drop on the ground. Cosmetic only, no ban risk.
Two approaches tried (world pickup flags + TutorialDataChunk), neither gates the spawn.
**Research:** [spec/41](spec/41-info-tab-ground-drop.md) | **Blocker:** need EMEVD decompilation

### 🟢 AoW Socketing + Infusion Edit — in-editor weapon customization

Edit socketed Ash of War and infusion affinity per weapon directly from Inventory tab.

**UI:** "AoW/Infuse Edit" toggle button (next to Add Favorites in InventoryTab) switches table to edit mode.
In edit mode: only eligible weapons shown (melee_armaments + ranged_armaments that support gem socketing), two dropdown columns — **Infusion** and **Ash of War**.

**AoW rules:**
- Dropdown shows only AoW from player's **inventory** (not storage — must be moved first, matching in-game behavior).
- AoW items display "In Use · WeaponName" badge when already socketed on another weapon.
- Weapons that don't support AoW (Seals, Staves, fixed-skill weapons) are hidden from edit mode view.

**Infusion rules:**
- Infusion is baked into `GaItemFull.ItemID` (`BaseID + infuseOffset + upgrade`); changing affinity = rewriting ItemID.
- Only affinities allowed by the weapon are shown (source: `EquipParamWeapon.disableGemAttr` bitmask from regulation.bin CSV — **needs pre-import into db**).

**Implementation phases:**

1. **Research** — parse `EquipParamWeapon.gemMountType` (column 248) to build a set of weapon BaseIDs that support AoW socketing; parse `disableGemAttr` (column 134) per weapon to know which affinities are valid. Import both as lookup maps into `backend/db/data/`.

2. **Backend: read** — extend `ItemViewModel` with:
   - `AttachedAoWID uint32`, `AttachedAoWName string` (read from `GaItemFull.AoWGaItemHandle`)
   - `InUseByWeapon string` (resolved by scanning all weapon GaItems for matching `AoWGaItemHandle`)
   - `AllowedInfusions []int` (from `disableGemAttr` lookup)

3. **Backend: write** — two new endpoints in `app.go`:
   - `SetWeaponAoW(charIdx, weaponHandle, aowItemID uint32) error` — updates `GaItemFull.AoWGaItemHandle`; aowItemID=0 clears it (`0xFFFFFFFF`)
   - `SetWeaponInfusion(charIdx, weaponHandle, infuseOffset uint32) error` — rewrites `GaItemFull.ItemID` = `BaseID + infuseOffset + currentUpgrade`; validates infuseOffset against `disableGemAttr`

4. **Frontend** — `InventoryTab.tsx`:
   - "AoW/Infuse Edit" button toggles `editMode` state
   - In edit mode: filter to eligible armaments, show two dropdown columns
   - AoW dropdown sources from inventory AoW items (`category === "Ash of War"`, not storage)
   - Infusion dropdown sources from `AllowedInfusions` on the weapon ViewModel

**Mockup (v1, superseded):** `tmp/mockups/aow-socketing-mockup.html`

### 🟢 Lost Ash of War — Key Items support
"Lost Ash of War" (used at Sites of Grace to duplicate AoWs) must be addable from Key Items tab. Verify item is correctly categorized as `key_items` in `backend/db/data/` and not missing or miscategorized. Also check `Lost Spirit Ash` (same mechanic for spirit ashes).

### 🟢 Ban Detector — save file risk scan
Scan a save file for content that is known or suspected to trigger server-side bans. Surfaces findings per-slot in the Tools tab without requiring the file to be loaded for editing.

**Checks (✅ = data available now):**
- ✅ Cut content / `ban_risk` items in inventory + storage (uses existing `db.go` flags)
- ✅ Any attribute > 99
- ✅ Character level > 713
- ✅ Weapon upgrade level above vanilla cap (+25 standard / +10 somber; encoded in item ID)
- ✅ SteamID embedded in save vs. expected account
- ⚠️ Soul Memory mismatch (`current_runes + spent_runes ≠ total_runes_earned`) — needs `total_runes_earned` offset RE
- ⚠️ Boss loot without kill event flags — needs boss_item_id → event_flag mapping

**Output:** per-slot report with risk tier (High / Medium / Info), item list, and plain-language explanation. Optional JSON export.

**Design:** → spec/45 §3 (trigger reference)

### 🟢 Safe Weapon De-leveling — matchmaking bracket control
Scan character's max weapon upgrade level (determines PvP matchmaking bracket), offer safe downgrade.
Useful for PvP builds targeting specific level ranges without starting new character.

### 🟢 Save Corruption Detection UI
Backend `DiagnoseSaveCorruption()` exists. Needs frontend display + auto-repair for recoverable issues.

### 🟢 Dungeon Door Flags — remaining 4 entries
War-Dead Catacombs (requires Radahn defeat), 3 DLC catacombs (m61 tiles, `20{col}{row}{ObjAct}` format).

---

## Backlog

| Priority | Feature |
|----------|---------|
| 🟢 | RebuildSlot + RebuildSlotFull unification (one serialization pipeline) |
| 🔵 | Player Coordinates / Teleportation (CSPlayerCoords 0x3D bytes) |
| 🔵 | Weather & Time of Day editing |
| 🔵 | DLC Progress Manager (Scadutree/Revered Spirit Ash/Miquella's Cross) |
| 🔵 | Save File Merging (inventory/quest between saves — unique feature) |
| 🔵 | Multiplayer Group Passwords (5 × wchar[8] at PGD offset 0x124) |
| 🔵 | Achievement / Trophy Progress Viewer |
| 🔵 | Table virtualization (@tanstack/react-virtual for 1500+ item lists) |
| 🔵 | Standalone preset editor (offline, no save loaded) |
| 🔵 | Regulation Param Browser — searchable reference for all 194 game param tables (7500+ fields) |

---

## Known Bugs

| Bug | Status | Ref |
|-----|--------|-----|
| Summoning Pools — UI works, no in-game effect | paused | [spec/42](spec/42-summoning-pools-bug.md) |
| Colosseum toggle — no visible in-game effect (may need online mode) | unverified | — |
| Boss Kill single-flag — grants runes but boss alive | planned fix | [spec/38](spec/38-boss-multiflag.md) |
| Grace toggle — map visible but not fast-travel activated | known limitation | — |
| Leyndell graces — Royal Capital and Ashen Capital phases mixed into one list | ✅ fixed | — |

---

## Final Cleanup (after all features)

- Dead code audit (`ts-prune` + `golangci-lint --enable=unused`)
- Performance profiling: cold start <1s, tab switch <100ms, DB filter no jank
- Frontend: `React.memo`, virtualization, lazy-loading tabs
- Backend: `go test -bench` / `pprof` on hot paths (reader.go, writer.go)
