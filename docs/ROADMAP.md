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

### Sort Order — dual-grid Inventory + Storage with bidirectional transfer
- **Dual-grid Sort Order tab** — Storage on the left, Inventory on the right; identical 5×6 frames; per-category tabs (weapons, talismans, head, chest, arms, legs) → spec/53
- **Bidirectional drag/drop transfer** — single and multi-item; Ctrl/Cmd toggle + Shift range selection per side; dragging a selected item moves the whole source-side batch; unsaved-preview guards on both sides
- **Duplicate-instance rehandle** — talisman / weapon / armor with the same handle on both sides triggers a rehandle path: new globally-unique handle + new GaItem entry + GaMap mapping → the same item can live in Storage and Inventory simultaneously
- **Inventory Apply Order** — `ReorderInventory` with stride-2 acquisition index (see spec/52); inventory-only mutation, monotonic counter advance
- **Storage Apply Order** — `ReorderStorage` mirrors the inventory path on `slot.Storage` (also stride-2 — same `acqIdx >> 1` bucketing applies in-storage); storage-only mutation, no touch to inventory bytes or counters
- **Storage Acquisition Ordering** — `GetStorageOrder` reads the real per-record `Index`; freshly transferred items receive monotonic indices and appear at the end of Acquisition ↑ on the destination side
- **VM visibility fix** — `mapItems` resolves itemIDs via `GaMap` first (`HandleToItemID` fallback) so rehandled instances are visible in inventory/storage view models immediately after transfer

### Sort Order — per-tile Weapon Edit Modal
- **Weapon edit icon on weapon tiles** — red top-left icon in Sort Order → Weapons grid for Inventory and Storage; opens a per-weapon modal without leaving the tab
- **Upgrade level edit** — new `App.ApplyWeaponUpgradeLevel` endpoint over `core.PatchWeaponItemID`; validates base weapon, infusion offset, and `MaxUpgrade`
- **Infusion edit** — reuses `ApplyWeaponInfusion`; preserves level across affinity changes
- **Ash of War edit** — strict-only assignment via `ApplyWeaponAoWStrict`, search, availability/status badges, Remove AoW, and fail-closed handling for unknown compatibility
- **Sort Order safe refresh** — modal patches metadata by handle, preserving pending preview order and drag/drop state

### Templates v2 — additive sharing format + library shell (foundation)
Phase 0..4 of the design doc `spec/56-templates-v2.md`. v2 apply is enabled for profile/stats from the library and from an imported YAML directly (Phase 5 / 5D.2), with apply-time value overrides for the same scope (Phase 6), v1 weapon level override at apply time on the Sort Order dropdown (Phase 6b), URL import with full SSRF guards (Phase 9), and **v2 `inventory.workspace` apply through the active Inventory Edit Session (Phase 7a)**. Equipment / talismans / spells writers (Phase 7b–7d) and appearance via preset (Phase 8) remain planned.
- **Global Templates shell** — blue `Templates` sidebar entry above `Save as...`, single surface for Library / Create / Import; existing `Export Template ▾` dropdown in Sort Order retained as a shortcut
- **Additive schema v2** — `saveforge.build-template` reader range `1 ≤ v ≤ 2`, new top-level `selection`, new `sections.profile` + `sections.stats` (canonical class selection key: `class`); v1 readers reject v2 cleanly, v2 readers accept v1
- **Public YAML import / export** — strict struct-typed `gopkg.in/yaml.v3` decode; library on disk stays JSON-internal; YAML imports transcode to JSON transparently
- **Create-from-character profile/stats flow** — per-section + per-field selection, live v2 preview, Save to Library
- **v2 metadata in UI** — `v2` badge + selected-sections summary in `TemplateLibraryModal` and `ImportTemplatePreviewModal`
- **Phase 5 — Apply schema v2 profile/stats from library (shipped 2026-05-31)** — `ApplyBuildTemplateV2ToCharacterJSON` / `…FromLibraryToCharacter` / `…FromFileToCharacter` under `slotMu[charIdx]` with `SnapshotSlot` / `RestoreSlot` rollback; `profile.class` intentionally skipped; `ApplyTemplateV2Result.Character` typed as `vm.CharacterViewModel`; library-only UI in `TemplatesShellModal` + `TemplateLibraryModal` with inline confirm, `mode: "append"`, post-apply refresh of `inventoryVersion` / `saveLoadKey` / slots / undo. v1 Apply path unchanged; v2 entries carrying unsupported sections remain disabled.
- **Phase 5D.2 — Direct imported v2 YAML Apply (shipped 2026-05-31)** — `Import YAML → Preview → Apply to character` without the intermediate `Save to Library` step. Reuses the Phase 5D.1 `ApplyBuildTemplateV2ToCharacterJSON` endpoint with the canonical JSON already produced by the import preview, so there is no second file dialog and no TOCTOU re-read between preview and apply. Frontend-only change in `ImportTemplatePreviewModal` + `TemplatesShellModal` (plus their tests); `App.tsx`, the backend, and the bindings are untouched. `ApplyBuildTemplateV2FromFileToCharacter` still exists backend/bindings-side but remains unwired in UI. v1 imported templates never see the v2 Apply button; v2 imports carrying unsupported sections stay disabled; Save to Library remains independent; library Apply path unchanged.
- **Phase 6 — Apply-time overrides for v2 profile/stats (shipped 2026-05-31)** — edit-before-apply for the safe profile + stats subset, available from both the direct YAML import preview and the library list through a second per-entry button `Apply with overrides…`. The fast library Apply path (`ApplyBuildTemplateV2FromLibraryToCharacter`) remains unchanged. Frontend-only: new `ApplyOverridesPanel` + `ApplyOverridesModal` mutate the canonical JSON in memory and post the result through `ApplyBuildTemplateV2ToCharacterJSON(charIdx, mutatedCanonicalJSON, { mode: 'append' })`; backend, bindings, and `App.tsx` are untouched. Editable: `profile.{name,level,runes,soulMemory,clearCount,scadutreeBlessing,shadowRealmBlessing,talismanSlots}` and all eight `stats.*`; per-field ranges mirror the schema validator and the UI pre-checks them, but the backend remains the source of truth. `profile.class` is rendered read-only with a "Skipped on apply (Phase 5)" hint. v1 templates and v2 templates with unsupported sections never expose the overrides button. `Save to Library` keeps persisting the original canonical JSON, not the in-modal edits. Apply-time value editing for inventory / storage / equipment / spells / appearance / sort order / world progress is **not** included; weapon level override at apply time is deferred to Phase 6b / Phase 7.
- **Phase 6b — Weapon level override for v1 inventory.workspace templates (shipped 2026-05-31)** — apply-time runtime option for the existing v1 `inventory.workspace` Apply path in `SortOrderTab.tsx` Templates dropdown. `app_templates.go::ApplyTemplateOptions` gains an additive `WeaponLevelOverride { Enabled bool, StandardLevel *int, SomberLevel *int }` field; default is no-override / `Keep`. `applyTemplateItemsToWorkspace` threads the option through a new `applyWeaponLevelOverride` helper that runs **after** each template-side weapon patch and re-invokes `editor.UpdateWeapon` with `editor.ClampUpgrade(req, MaxUpgrade)` per `editor.EditableItem.MaxUpgrade` — `25` (standard) consumes `StandardLevel`, `10` (somber/special) consumes `SomberLevel`, `0` (unupgradeable) is skipped with a `weapon_unupgradeable` warning, other values are silent skips. Over-cap requests are clamped down with a `weapon_level_clamped` warning. New issue codes `IssueCodeWeaponLevelClamped` + `IssueCodeWeaponUnupgradeable` in `backend/templates/import.go` surface via the standard `ImportPreviewIssue` shape on `report.Warnings` — never `Errors`. The override runs entirely inside the active `InventoryWorkspaceSnapshot` via `editor.UpdateWeapon`; no bytes go to `slot.Data`, `SaveInventoryWorkspaceChanges` remains the only commit point and is never called automatically by the override. UI controls live inside the existing `SortOrderTab.tsx` Templates dropdown (`weapon-override-panel` testid with an `Enabled` master checkbox and two number inputs ranging `0..25` / `0..10`); `TemplatesShellModal`, `ApplyOverridesPanel`, `ImportTemplatePreviewModal`, and `TemplateLibraryModal` are intentionally **not** touched. `frontend/wailsjs/go/models.ts` regenerated for the new shape; `App.tsx` untouched. **No v2 schema change, no new v2 inventory writer, no direct save mutation through `PatchWeaponItemID` from the template apply, no equipment writer.** v2 `inventory.workspace` scope guard in `app_templates_v2_apply.go` preserved unchanged.
- **Phase 7a — v2 `inventory.workspace` apply to active Inventory Edit Session (shipped 2026-05-31)** — first real v2 apply path for `inventory.workspace`. The `app_templates_v2_apply.go` scope guard is lifted for `inventory.workspace` only; the v2 payload is routed through the active `InventoryEditSession`/`InventoryWorkspaceSnapshot` so the writes land in the workspace `SortOrderTab` already operates on — never directly into `slot.Data`. New backend endpoint `App.GetActiveInventoryEditSessionForCharacter(charIdx) → { active, sessionID }` lets the Templates shell look up the active session without changing `App.tsx`. `ApplyTemplateV2Options` gains an additive `SessionID string`. No session for an `inventory.workspace`-bearing template → new `templates.IssueCodeInventorySessionRequired = "inventory_session_required"`; unknown / wrong-character session → new `templates.IssueCodeInventorySessionInvalid = "inventory_session_invalid"`. `ApplyTemplateV2Result` adds `InventoryItemsApplied`, `StorageItemsApplied`, and optional `Workspace *editor.InventoryWorkspaceSnapshot`. **Mixed apply** (profile + stats + inventory.workspace in one v2 template) is atomic — a `rollbackBoth()` closure restores both the slot byte snapshot (`core.RestoreSlot`) and the workspace value snapshot on every error exit. Inventory items go through `applyTemplateItemsToWorkspace(&sess.Workspace, …, editor.ContainerInventory, nil)` with the `nil` override pin documenting that **Phase 6b weapon level override is intentionally not wired into the v2 path in this phase**. UI: `TemplatesShellModal.tsx` looks up the active session for library Apply, direct YAML Apply, URL Apply, and `Apply with overrides…`; missing session surfaces "Open the Sort Order workspace before applying inventory templates." and the binding is **not** called. Profile/stats-only v2 applies still proceed with `sessionID = ''`. `ImportTemplatePreviewModal` and `TemplateLibraryModal` bump their supported-section sets to `['profile', 'stats', 'inventory.workspace']`. **No new v2 schema section**, no equipment writer, no direct save mutation from the v2 inventory path — `SaveInventoryWorkspaceChanges` remains the only commit point. `App.tsx` and `SortOrderTab.tsx` untouched. Phase 7b+ (equipment / talismans / spells writers, appearance via preset, multi-character pack) remain planned below.
- **Phase 9 — URL import with SSRF guards (shipped 2026-05-31)** — `Templates → Import from URL… → Preview → Save to Library / Apply to character / Apply with overrides`. New `PreviewBuildTemplateImportYAMLFromURL(rawURL)` Wails endpoint wraps `backend/templates/url_import.go::FetchYAMLFromURL`, which implements all 13 §12.3 guards (HTTPS-only; pre-connect IP filter on literal and DNS-resolved addresses including loopback / RFC1918 / link-local / ULA / multicast / broadcast / cloud-metadata `169.254.169.254` + `fd00:ec2::254`; redirect re-check on every hop; max 3 redirects; 1 MiB `io.LimitReader` body cap; 10 s total / 5 s idle / TLS / header / dial timeouts; strict TLS with system root CAs and no `InsecureSkipVerify`; identifying User-Agent; Content-Type allowlist; no auth / cookies / custom headers; strict struct-typed YAML decode reused unchanged; no auto-refresh; the fetch alone never mutates anything). The URL preview reuses the existing `ImportTemplatePreviewModal` + `LoadedTemplatePreview { Report, JSON, Path }` shape, so Save to Library, Apply to character (Phase 5D.2), and Apply with overrides (Phase 6) ship without modification on the URL surface. **No library schema change** — `sourceURL` is not persisted in this phase; the library still records what was saved, not where it came from. `App.tsx` untouched; only `frontend/wailsjs/go/main/App.{d.ts,js}` regenerated for the new binding. Out of scope: authenticated downloads, domain allowlist, URL auto-refresh, direct apply without preview.

---

## Planned

### 🟡 Templates v2 — Phase 7b+: equipment / talismans / spells writers + multi-char pack

Continuation of the foundation + Phase 5 profile/stats Apply + Phase 6 apply-time overrides + Phase 6b weapon level override + Phase 9 URL import + Phase 7a v2 inventory.workspace Apply shipped above. v2 apply for `inventory.workspace` shipped through the workspace path in Phase 7a; equipment / talismans / spells / appearance remain blocked by design — Phase 7b+ will lift that one writer at a time.

- **Phase 7b / 7c / 7d** — new public write paths for `ChrAsmEquipment` slots 0..9 / 12–15 (weapons + armor), equipped talisman slots 17–21, and the 14 `EquippedSpells` slots — each with PC + PS4 round-trip parity
- **Phase 7a.2** — wire Phase 6b weapon level override into the v2 `inventory.workspace` Apply path (today the v2 path passes a hard-coded `nil` override; Phase 7a.2 introduces a `WeaponLevelOverride` runtime option on `ApplyTemplateV2Options` mirroring the v1 dropdown semantics)
- **Phase 8** — apply `sections.appearance.preset` through the existing `app_appearance.go::ApplyPresetToCharacter` helper; raw FaceData never written from a template
- **Phase 10** — multi-character pack (`scope: pack`) with explicit source→destination slot mapping and per-slot rollback

**Design:** [spec/56](spec/56-templates-v2.md)

### 🟡 PvP Presets — Advanced → Presets tab (one-click PvP build setup)

One-click curated build templates for invasions, duels, and PvP-ready characters.
Replaces the "Coming Soon" placeholder in **Advanced → Presets** (currently `PvPTab.tsx`).
**Mockup:** `tmp/presets-mockup.png` | **Design:** spec/52 (to be created)

#### UI (based on mockup)

Left panel — preset browser:
- Search field + filter chips: All · Built-in · Custom · Invasions · Duels
- Preset list cards (name, RL badge, category tag, starting class hint)

Right panel — preset details:
- Header: preset name, tags (RL · category · starting class)
- **What will be applied** — per-section checkbox list, togglable before apply
- **Build Summary** — stats preview + weapon/armor/talisman/spell list with upgrade/affinity details
- **Apply Mode** radio at bottom:
  - *Build Only* — stats + inventory + equipment; world-state untouched
  - *Build + PvP Travel* — build + key Sites of Grace + map reveal (travel convenience)
  - *Full PvP Ready* — build + summoning pools + all PvP-relevant world unlocks defined in preset
- Action row: **Apply Preset** · **Save as Custom** · **Export JSON** · **Manage Presets**

#### Built-in presets (starter library, ~5)

| Preset | RL | Category | Use case |
|---|---|---|---|
| RL30 Stormveil Invader | 30 | Invasion | Early-game twink |
| RL60 Liurnia Invader | 60 | Invasion | Mid-game dex |
| RL90 Altus Invader | 90 | Invasion | Altus plateau |
| RL125 Duelist Meta | 125 | Duel | Standard bracket |
| RL150 Endgame Hybrid | 150 | Duel | End-game hybrid |

#### What a preset can contain

- **Character:** level, stats (Vig/Min/End/Str/Dex/Int/Fai/Arc), runes, starting class hint
- **Inventory:** weapons (base ID + upgrade level + affinity offset + socketed AoW), armor, talismans, spells/incantations, consumables, multiplayer items (effigies, cipher rings, fingers), flask setup
- **Equipment slots:** pre-equipped weapon/armor/talismans (applied after inventory add)
- **PvP readiness** (optional — Travel/Full modes only): map reveal, Sites of Grace selection, summoning pools

#### Out of scope

NPC quest progression, boss kills, story endings, narrative flags, "unlock everything" world state.
`Full PvP Ready` applies only the PvP-relevant subset defined in the preset — not a blanket world unlock.

#### Preset JSON schema (`schemaVersion: 1`)

```json
{
  "schemaVersion": 1,
  "type": "preset",
  "id": "rl60_liurnia_invader",
  "name": "RL60 Liurnia Invader",
  "description": "Balanced dex build optimized for Liurnia invasions.",
  "category": "PvP / Invasion",
  "tags": ["pvp", "invasion", "rl60", "dex"],
  "recommendedStartingClass": "Warrior",
  "editable": {
    "stats": true, "weapons": true, "armor": true, "talismans": true,
    "spells": true, "consumables": true, "multiplayerItems": true,
    "maps": true, "sitesOfGrace": true, "summoningPools": true
  },
  "payload": {
    "stats": {},
    "inventory": {},
    "equipment": {},
    "pvpReadiness": { "maps": {}, "sitesOfGrace": {}, "summoningPools": {} }
  }
}
```

Built-in presets ship as embedded JSON files (Go `embed`); each item is resolved by `BaseID`
against existing `backend/db` data — no raw offsets, no hardcoded byte blobs.

#### Safety

- Preview diff shown before apply (per-section summary of what changes)
- Per-section toggles in "What will be applied" — user controls exactly what gets written
- Preset validated before apply: unknown item IDs warned, quantity capped per spec/34, stat floor checked
- Unknown/unrecognised JSON sections are silently ignored or surfaced as warnings — never auto-applied
- World-state / PvP readiness: applied **only** in Travel or Full modes, never in Build Only
- PvP readiness warnings mirror spec/32 risk tier system

#### Implementation phases

1. **Preset model + JSON schema** — Go struct `PvPPreset`, JSON marshal/unmarshal, `schemaVersion` validation
2. **Preset loader + validation** — `LoadBuiltInPresets()`, `LoadPresetFromFile()`, `ValidatePreset()` (IDs, qty caps, stat floor)
3. **UI shell** — `PresetsTab.tsx`: left list + right detail panel, search + filter chips, apply mode radio, action row
4. **Preview / diff engine** — `PreviewPresetApply()` returns per-section change summary before commit
5. **Apply engine** — `ApplyPreset(charIdx, preset, mode, toggles)`: reuses `AddItemsToCharacter`, `ApplyVMToParsedSlot`, `ApplyPvPPreparation`; pushes undo before apply
6. **Built-in preset library** — 5 JSON presets compiled-in via Go `embed`; items resolved by BaseID against `backend/db`
7. **Custom preset import/export** — `SaveAsCustomPreset()` (file dialog), `ImportCustomPreset()` (validate + add to custom list), Manage Presets list
8. **Safety + manual testing** — end-to-end on PS4 save: each mode, each built-in preset, invalid JSON, apply → save → load → verify

#### Acceptance criteria

- Advanced → Presets no longer shows "Coming Soon"
- Built-in preset list visible and filterable (All / Built-in / Custom / Invasions / Duels)
- Selecting a preset populates right panel (Build Summary + What will be applied)
- Per-section toggles adjust what gets written before apply
- Apply Mode controls world-state writes: SoG/map/pools absent in Build Only
- Applied preset → stats + inventory match preset definition after save/reload
- Save as Custom → file dialog → JSON written with correct schema
- Import JSON → validated → appears in Custom filter
- Invalid/unknown JSON sections do not silently corrupt the save

**Effort:** ~30-40h (all 8 phases) | **Depends on:** spec/43 (transactional add), spec/32 (risk tiers), `ApplyPvPPreparation` (existing in `app_pvp.go`)

---

### 🔵 Advanced Save Editor — power-user / research tab
Single tab under **Advanced → Save Editor** for reading and writing known and experimental save values across three technically distinct layers: Event Flags, regulation snapshot params, and raw offsets.

MVP: known EventFlags read/write, manual EventFlag by ID, NetworkParam fields (NETWORK_PARAM_ST), app-known macros, pending changes list, preview diff, atomic Apply, Export patch report.
Not in MVP: raw offset editor, full regulation browser, ShopLineupParam/ItemLotParam write support, patch import/export.
**Design:** [spec/51](spec/51-advanced-save-editor.md) | **Effort:** estimated 20-30h (MVP only)

### 🟡 Character Preset Export/Import — JSON build sharing
Export/import stats + inventory + storage + appearance to portable `.preset.json`.
Replaces disabled `App.ImportCharacter`. Community build sharing.
**Design:** [spec/37](spec/37-character-presets.md) | **Effort:** 10-14h (Phase 1+2 MVP)

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
| Summoning Pools — wrong flag IDs post-patch v1.12 | **fix ready** | [spec/42](spec/42-summoning-pools-bug.md) |
| Colosseum toggle — no visible in-game effect (may need online mode) | unverified | — |
| Boss Kill single-flag — grants runes but boss alive | planned fix | [spec/38](spec/38-boss-multiflag.md) |
| Grace toggle — map visible but not fast-travel activated | known limitation | — |
| Leyndell graces — Royal Capital and Ashen Capital phases mixed into one list | ✅ fixed | — |
| Phantom/duplicate character slot + in-place delete (no shift) | ✅ fixed (PS4-verified) | CHANGELOG |

---

## Final Cleanup (after all features)

- Dead code audit (`ts-prune` + `golangci-lint --enable=unused`)
- Performance profiling: cold start <1s, tab switch <100ms, DB filter no jank
- Frontend: `React.memo`, virtualization, lazy-loading tabs
- Backend: `go test -bench` / `pprof` on hot paths (reader.go, writer.go)
