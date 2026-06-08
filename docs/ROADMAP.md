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
Phase 0..4 of the design doc `spec/56-templates-v2.md`. v2 apply is enabled for profile/stats from the library and from an imported YAML directly (Phase 5 / 5D.2), with apply-time value overrides for the same scope (Phase 6), v1 weapon level override at apply time on the Sort Order dropdown (Phase 6b), URL import with full SSRF guards (Phase 9), **v2 `inventory.workspace` apply through the active Inventory Edit Session (Phase 7a)**, **apply-time weapon level override threaded into the v2 inventory.workspace path (Phase 7a.2)**, **backend equipment writer foundation (Phase 7b.0)**, **v2 `sections.equipment` end-to-end (Phase 7b.1) — weapon/ammo/armor slots, no auto-add, equipment + inventory.workspace combo hard rejected**, **v2 talisman apply through `sections.equipment.talisman1..5` (Phase 7c) — extends the existing equipment section instead of introducing a separate `sections.equippedTalismans`, pouch-gated by `1 + profile.talismanSlots`, Talisman5 non-empty always warn+skip**, and **v2 spells import / preview / apply through `sections.spells` (Phase 7d.0 → 7d.4) — 14 named `Spell1..Spell14` pointer slots (`BaseItemID == 0` = explicit clear, nil pointer = leave unchanged, `0x40000000` prefix shared by sorceries AND incantations), new `(s *core.SaveSlot).WriteSpells` batch writer with targeted hash[10] recompute, new `db.ItemIDToMagicParamID(itemID) = itemID & 0x0FFFFFFF` 28-bit-mask helper, frontend gating + preview row + library badge; spells coexist freely with all other sections (no combo restrictions); `WriteSpells` runs **after** `vm.MapViewModelToSlot` and **after** `slot.WriteEquipment` so the VM flush cannot clobber the new region**. Spells create-from-character export (Phase 7d.4b) and appearance via preset (Phase 8) remain planned.
- **Global Templates shell** — blue `Templates` sidebar entry above `Save as...`, single surface for Library / Create / Import; existing `Export Template ▾` dropdown in Sort Order retained as a shortcut
- **Additive schema v2** — `saveforge.build-template` reader range `1 ≤ v ≤ 2`, new top-level `selection`, new `sections.profile` + `sections.stats` (canonical class selection key: `class`); v1 readers reject v2 cleanly, v2 readers accept v1
- **Public YAML import / export** — strict struct-typed `gopkg.in/yaml.v3` decode; library on disk stays JSON-internal; YAML imports transcode to JSON transparently
- **Create-from-character profile/stats flow** — per-section + per-field selection, live v2 preview, Save to Library
- **v2 metadata in UI** — `v2` badge + selected-sections summary in `TemplateLibraryModal` and `ImportTemplatePreviewModal`
- **Phase 5 — Apply schema v2 profile/stats from library (shipped 2026-05-31)** — `ApplyBuildTemplateV2ToCharacterJSON` / `…FromLibraryToCharacter` / `…FromFileToCharacter` under `slotMu[charIdx]` with `SnapshotSlot` / `RestoreSlot` rollback; `profile.class` intentionally skipped; `ApplyTemplateV2Result.Character` typed as `vm.CharacterViewModel`; library-only UI in `TemplatesShellModal` + `TemplateLibraryModal` with inline confirm, `mode: "append"`, post-apply refresh of `inventoryVersion` / `saveLoadKey` / slots / undo. v1 Apply path unchanged; v2 entries carrying unsupported sections remain disabled.
- **Phase 5D.2 — Direct imported v2 YAML Apply (shipped 2026-05-31)** — `Import YAML → Preview → Apply to character` without the intermediate `Save to Library` step. Reuses the Phase 5D.1 `ApplyBuildTemplateV2ToCharacterJSON` endpoint with the canonical JSON already produced by the import preview, so there is no second file dialog and no TOCTOU re-read between preview and apply. Frontend-only change in `ImportTemplatePreviewModal` + `TemplatesShellModal` (plus their tests); `App.tsx`, the backend, and the bindings are untouched. `ApplyBuildTemplateV2FromFileToCharacter` still exists backend/bindings-side but remains unwired in UI. v1 imported templates never see the v2 Apply button; v2 imports carrying unsupported sections stay disabled; Save to Library remains independent; library Apply path unchanged.
- **Phase 6 — Apply-time overrides for v2 profile/stats (shipped 2026-05-31)** — edit-before-apply for the safe profile + stats subset, available from both the direct YAML import preview and the library list through a second per-entry button `Apply with overrides…`. The fast library Apply path (`ApplyBuildTemplateV2FromLibraryToCharacter`) remains unchanged. Frontend-only: new `ApplyOverridesPanel` + `ApplyOverridesModal` mutate the canonical JSON in memory and post the result through `ApplyBuildTemplateV2ToCharacterJSON(charIdx, mutatedCanonicalJSON, { mode: 'append' })`; backend, bindings, and `App.tsx` are untouched. Editable: `profile.{name,level,runes,soulMemory,clearCount,scadutreeBlessing,shadowRealmBlessing,talismanSlots}` and all eight `stats.*`; per-field ranges mirror the schema validator and the UI pre-checks them, but the backend remains the source of truth. `profile.class` is rendered read-only with a "Skipped on apply (Phase 5)" hint. v1 templates and v2 templates with unsupported sections never expose the overrides button. `Save to Library` keeps persisting the original canonical JSON, not the in-modal edits. Apply-time value editing for inventory / storage / equipment / spells / appearance / sort order / world progress is **not** included; weapon level override at apply time is deferred to Phase 6b / Phase 7.
- **Phase 6b — Weapon level override for v1 inventory.workspace templates (shipped 2026-05-31)** — apply-time runtime option for the existing v1 `inventory.workspace` Apply path in `SortOrderTab.tsx` Templates dropdown. `app_templates.go::ApplyTemplateOptions` gains an additive `WeaponLevelOverride { Enabled bool, StandardLevel *int, SomberLevel *int }` field; default is no-override / `Keep`. `applyTemplateItemsToWorkspace` threads the option through a new `applyWeaponLevelOverride` helper that runs **after** each template-side weapon patch and re-invokes `editor.UpdateWeapon` with `editor.ClampUpgrade(req, MaxUpgrade)` per `editor.EditableItem.MaxUpgrade` — `25` (standard) consumes `StandardLevel`, `10` (somber/special) consumes `SomberLevel`, `0` (unupgradeable) is skipped with a `weapon_unupgradeable` warning, other values are silent skips. Over-cap requests are clamped down with a `weapon_level_clamped` warning. New issue codes `IssueCodeWeaponLevelClamped` + `IssueCodeWeaponUnupgradeable` in `backend/templates/import.go` surface via the standard `ImportPreviewIssue` shape on `report.Warnings` — never `Errors`. The override runs entirely inside the active `InventoryWorkspaceSnapshot` via `editor.UpdateWeapon`; no bytes go to `slot.Data`, `SaveInventoryWorkspaceChanges` remains the only commit point and is never called automatically by the override. UI controls live inside the existing `SortOrderTab.tsx` Templates dropdown (`weapon-override-panel` testid with an `Enabled` master checkbox and two number inputs ranging `0..25` / `0..10`); `TemplatesShellModal`, `ApplyOverridesPanel`, `ImportTemplatePreviewModal`, and `TemplateLibraryModal` are intentionally **not** touched. `frontend/wailsjs/go/models.ts` regenerated for the new shape; `App.tsx` untouched. **No v2 schema change, no new v2 inventory writer, no direct save mutation through `PatchWeaponItemID` from the template apply, no equipment writer.** v2 `inventory.workspace` scope guard in `app_templates_v2_apply.go` preserved unchanged.
- **Phase 7a — v2 `inventory.workspace` apply to active Inventory Edit Session (shipped 2026-05-31)** — first real v2 apply path for `inventory.workspace`. The `app_templates_v2_apply.go` scope guard is lifted for `inventory.workspace` only; the v2 payload is routed through the active `InventoryEditSession`/`InventoryWorkspaceSnapshot` so the writes land in the workspace `SortOrderTab` already operates on — never directly into `slot.Data`. New backend endpoint `App.GetActiveInventoryEditSessionForCharacter(charIdx) → { active, sessionID }` lets the Templates shell look up the active session without changing `App.tsx`. `ApplyTemplateV2Options` gains an additive `SessionID string`. No session for an `inventory.workspace`-bearing template → new `templates.IssueCodeInventorySessionRequired = "inventory_session_required"`; unknown / wrong-character session → new `templates.IssueCodeInventorySessionInvalid = "inventory_session_invalid"`. `ApplyTemplateV2Result` adds `InventoryItemsApplied`, `StorageItemsApplied`, and optional `Workspace *editor.InventoryWorkspaceSnapshot`. **Mixed apply** (profile + stats + inventory.workspace in one v2 template) is atomic — a `rollbackBoth()` closure restores both the slot byte snapshot (`core.RestoreSlot`) and the workspace value snapshot on every error exit. Inventory items initially went through `applyTemplateItemsToWorkspace(&sess.Workspace, …, editor.ContainerInventory, nil)` — the `nil` override pin tracked the Phase 7a.2 follow-up that has since shipped (see next bullet). UI: `TemplatesShellModal.tsx` looks up the active session for library Apply, direct YAML Apply, URL Apply, and `Apply with overrides…`; missing session surfaces "Open the Sort Order workspace before applying inventory templates." and the binding is **not** called. Profile/stats-only v2 applies still proceed with `sessionID = ''`. `ImportTemplatePreviewModal` and `TemplateLibraryModal` bump their supported-section sets to `['profile', 'stats', 'inventory.workspace']`. **No new v2 schema section**, no equipment writer, no direct save mutation from the v2 inventory path — `SaveInventoryWorkspaceChanges` remains the only commit point. `App.tsx` and `SortOrderTab.tsx` untouched. Phase 7b+ (equipment / talismans / spells writers, appearance via preset, multi-character pack) remain planned below.
- **Phase 7a.2 — weapon level override on the v2 `inventory.workspace` apply path (shipped 2026-05-31)** — apply-time runtime option that threads the Phase 6b weapon level override into the Phase 7a v2 apply path. `ApplyTemplateV2Options` gains an additive `WeaponLevelOverride *WeaponLevelOverride` field reusing the v1 `WeaponLevelOverride { Enabled bool, StandardLevel *int, SomberLevel *int }` type and the v1 `validateWeaponLevelOverride` validator verbatim (so the bindings expose a single `WeaponLevelOverride` class shared between v1 and v2 surfaces). The two hard-coded `nil` override pins at the v2 inventory + storage `applyTemplateItemsToWorkspace` call sites in `app_templates_v2_apply.go` are replaced with `opts.WeaponLevelOverride`; the helper itself, `applyWeaponLevelOverride`, and the dual snapshot rollback contract from Phase 7a are unchanged. Validation runs **before** `acquireSession` so a structurally broken override bounces with `IssueCodeStructureInvalid` and zero side effects; a structurally valid override on a profile/stats-only template is silently ignored (no items → no-op). `weapon_level_clamped` and `weapon_unupgradeable` warnings flow into `ApplyTemplateV2Result.Preview.Warnings` through the existing `invWarn`/`stoWarn` aggregation. UI: a new `WeaponLevelOverridePanel` (`apply-overrides-weapon-{panel,enabled,standard,somber,error}` testids) is embedded inside the existing `ApplyOverridesModal` and rendered **only** when the canonical JSON's selection nominates `inventory.workspace`; profile/stats-only templates leave the weapon panel unrendered. `ApplyOverridesModal.onConfirm` is extended to a `(mutatedJSON, weaponOverride?) ⇒ …` signature and the runtime option travels exclusively through that argument — never inside the canonical JSON. `TemplatesShellModal.handleConfirmOverrides` forwards `weaponLevelOverride` to `main.ApplyTemplateV2Options.createFrom({ mode, sessionID, weaponLevelOverride })`. The fast library Apply (`ApplyBuildTemplateV2FromLibraryToCharacter`) continues to send no override; the v1 `SortOrderTab.tsx` Phase 6b dropdown is untouched. **No new v2 schema section, no equipment writer, no direct save mutation, no `App.tsx` change.** `SaveInventoryWorkspaceChanges` remains the only commit point. Phase 7b+ (equipment / talismans / spells writers, appearance via preset, multi-character pack) remain planned below.
- **Phase 7b.0 — equipment writer foundation (shipped 2026-05-31)** — backend-only `(s *core.SaveSlot).WriteEquipment([]EquipmentWrite) error` foundation for the future Phase 7b.1 template equipment apply. Covers weapon/ammo slots 0–9 (hash 7) and armor slots 12–15 (hash 8) only; talisman slots 17–21, EquippedGreatRune (slot 10), unknown slots 11/16, spells, and quick items remain out of scope. `EquipmentSlotKind` enum exposes only the 14 supported slots. Each write resolves `handle → itemID` via `slot.GaMap` and writes the encoded ChrAsmEquipment form (`itemID | 0x80000000` for weapons/armor; goods `itemID` directly for ammo, since goods item IDs already carry the `0x40` prefix). `Handle == 0` clears the slot to `0xFFFFFFFF`; `Handle == 0xFFFFFFFF` is rejected as a defensive guard. `0xC0` Ash of War handles are rejected in weapon slots even though the read-side encoding rule would technically accept them (AoW equipping deferred to a later phase). Mismatched prefixes (talisman in weapon slot, weapon in armor slot, etc.), unknown enum values, duplicate slots within a batch, and handles missing from `GaMap` all error out before any byte is touched. Hash recompute is targeted: writes touching slots 0–9 recompute hash entry 7; writes touching slots 12–15 recompute hash entry 8; unrelated hash entries are untouched. Atomicity is enforced by validate-then-mutate ordering — a single invalid write inside a batch aborts the entire batch with zero bytes mutated. **No template schema change, no `app_templates*.go` change, no App method, no Wails bindings change, no frontend change, no template apply integration.** Phase 7b.1 (template apply wiring + strict "item must be in inventory" enforcement) tracked below.
- **Phase 7c — v2 talisman apply through `sections.equipment.talisman1..5` (shipped 2026-06-01)** — extends the Phase 7b.1 `sections.equipment` schema with the five talisman slots (ChrAsmEquipment indices 17–21, hash 8) instead of introducing a separate `sections.equippedTalismans`; the earlier spec/56 design note that reserved a separate section is amended (talismans live in the same `ChrAsmEquipment` struct as weapons/ammo/armor and reuse the Phase 7b.1 resolver, combo guard, preview row, frontend gate, and rollback). `backend/core/equipment_writer.go` gains `EquipSlotTalisman1..5`, a fourth class `slotClassTalisman` that accepts only the `ItemTypeAccessory` (0xA0) handle prefix, and a class-gate reject for 0xA0 in the previously-supported weapon/armor/ammo classes. Encoded slot value is `GaMap[handle]` directly (no `| 0x80000000` mask) because talisman handles map to itemIDs already carrying the `0x20` prefix in GaMap. The hash 8 recompute path widens to also cover indices 17–21; hash 7 stays untouched for talisman-only writes; the validate-then-mutate atomicity contract is preserved unchanged. `backend/templates/schema.go::EquipmentSection` adds `Talisman1..5 *EquipmentItemRef`; `equipmentSelectionFields`, `EquipmentSlotOrder`, `equipmentSlotRef`, and `SetEquipmentSlotRef` extend by the five canonical talisman keys (appended after `armorLegs` to preserve the 14-slot prefix order). Talisman refs reuse `EquipmentItemRef` but the apply layer ignores `Upgrade` / `InfusionName` / `AoWItemID` for talisman slots and producers normally omit those fields. New issue code `IssueCodeTalismanSlotPouchInsufficient` (`talisman_slot_pouch_insufficient`, warning) is emitted by the apply resolver when a non-empty talisman ref targets a slot beyond the **active** talisman pouch capacity = `1 + effective profile.talismanSlots` (clamped to `MaxProfileTalismanSlots = 3`, max 4 active slots). Talisman5 always trips this warning when populated with `baseItemID > 0` because vanilla Elden Ring caps the pouch at 4 active slots; an explicit clear (`baseItemID = 0`) for Talisman5 always bypasses the gate and writes `0xFFFFFFFF`. Mixed templates that also select `profile.talismanSlots` use the template's value (lifted to `1 + value`) so a `profile.talismanSlots = 3 + equipment.talisman4` template applies talisman4 successfully inside a single apply. The strict-existing-only resolver policy is preserved unchanged: missing talisman = `equipment_item_not_in_inventory` warn + skip, ambiguous match = first-wins warning, no auto-add, no storage resolver. The `equipment + inventory.workspace` hard reject (`equipment_inventory_combo_unsupported`) still fires when talismans appear under `sections.equipment` because they are part of the same section. `app_templates_v2_equipment.go` extends `equipmentSlotChrAsmIndex` and `equipmentSlotKindForKey` with the five talisman keys, adds `equipmentSlotIsTalisman` and `talismanSlotOrdinal` helpers, and routes the talisman path through the existing `decodeEquipmentSlotToRef` candidate selection (ammo or talisman → raw value, other → strip `0x80`); `buildEquipmentSection` already iterates `EquipmentSlotOrder`, so talismans appear in exports without a builder change. `itemToEquipmentRef` continues to set `Upgrade` only when `IsWeapon || IsArmor`, so a talisman editable item naturally yields a ref with `BaseItemID + Name` only. `app_templates_v2_apply.go` adds `computeActiveTalismanSlots(slot, tpl)` and threads `activeTalismanSlots` through to `resolveEquipmentWrites`; the resolver in `app_templates_v2_equipment.go` adds `MaxActiveTalismanSlots = 4` and the pouch gate. Frontend has **no source-code change** — `V2_APPLY_SUPPORTED_SECTIONS` already lists `equipment`, the existing `import-preview-equipment-slots` row enumerates whatever slot keys the summary lists (talismans included), and the Library / direct-YAML / URL apply paths reuse the canonical JSON pipeline. **No new section, no separate `sections.equippedTalismans`, no spells, no GreatRune through the template path, no slot 11/16, no quick / pouch items, no auto-add talismans, no storage resolver, no talisman-specific override panel, no `SortOrderTab.tsx` change, no `App.tsx` change, no new App method.** `frontend/wailsjs/go/models.ts` shows no diff (talisman extensions are JSON-payload fields on `EquipmentSection`, exchanged as opaque template JSON rather than a typed model). Manual validation 2026-06-01: equipment export with active talismans + Talisman5 clear, talisman keys in v2 preview row + Apply enabled, equipment-only apply happy path + game-load reload + Save & Quit retained state, pouch gating with `TalismanSlots = 0` (slot 1 OK, slots 2/3/4 warn + skip), mixed `profile.talismanSlots = 3 + equipment.talisman4` lifted cap inside a single apply, `talisman5 baseItemID > 0` warn + skip + `talisman5 baseItemID = 0` clear OK, missing talisman warn + skip with other slots committing, `equipment + inventory.workspace` hard reject unchanged, regression sweep across Phase 5 / 5D.2 / 6 / 6b / 7a / 7a.2 / 7b.1 / 9 / v1 SortOrderTab — user-confirmed `manual OK`. Phase 7d spell writer + apply, optional Phase 7b.2 lifting the equipment + inventory.workspace combo restriction, EquippedGreatRune through the template path, slots 11/16, and quick / pouch items remain future work.
- **Phase 7b.1 — v2 `sections.equipment` end-to-end (shipped 2026-06-01)** — schema + export + preview + apply + frontend surface that wires the Phase 7b.0 `SaveSlot.WriteEquipment` foundation into the Templates v2 apply path. New `EquipmentSection` carries 14 optional slot pointers (weapon LH/RH 1/2/3, `arrows1/2` + `bolts1/2`, `armorHead/Chest/Arms/Legs`); slot 10 EquippedGreatRune, slots 11/16, talismans 17–21, EquippedSpells, and quick / pouch items remain out of scope — matching the Phase 7b.0 writer surface exactly. Each `EquipmentItemRef` carries `BaseItemID` (`0` = explicit clear sentinel), optional informational `Name`, optional `Upgrade` disambiguator (nil = any upgrade), optional `InfusionName`, and optional `AoWItemID` disambiguator. `TemplateSelection.Equipment` joins the existing Selection model with a 14-slot allowlist. The app-layer scanner `buildEquipmentSectionFromSlot(slot, inventoryItems)` reads ChrAsmEquipment at the 14 supported indices, decodes the encoded form (weapon/armor: strip `0x80000000`; ammo: raw goods ItemID), and matches against `editor.EditableItem.ItemID` with a `db.GetItemDataFuzzy` / raw-ID fallback so the export always records what each slot held. The apply path adds `resolveEquipmentWrites(slot, sel, sec)`, which calls `editor.BuildSnapshot` once and delegates to a pure-logic `resolveEquipmentWritesFromItems(items, sel, sec)`: it walks `slot.Inventory.CommonItems` only (storage is intentionally not searched), matches by `BaseItemID` + optional disambiguators, emits `equipment_item_not_in_inventory` warnings + skips on miss, `equipment_item_ambiguous` warnings + first-match-wins on duplicates, and `EquipmentWrite{Handle:0}` for `BaseItemID == 0`. `SaveSlot.WriteEquipment` is invoked after the profile/stats `SyncPlayerToData`, so the Phase 7a `rollbackBoth()` closure naturally covers both the slot snapshot (`core.RestoreSlot`) and the targeted hash 7 / 8 entries the writer recomputes. New issue codes — `IssueCodeEquipmentInventoryComboUnsupported`, `IssueCodeEquipmentItemNotInInventory`, `IssueCodeEquipmentItemAmbiguous`, `IssueCodeEquipmentSlotInvalid` — surface through the standard `ImportPreviewIssue` shape. `ApplyTemplateV2Result` adds `EquipmentSlotsApplied int`. The combo `sections.equipment + sections.inventory.workspace` is **hard-rejected** in both the preview and the apply path (zero side effects): the writer needs `slot.GaMap` to be fresh, and inventory.workspace only commits to the slot when the user clicks Save changes. Equipment-only templates do NOT require an Inventory Edit Session, and the existing `TemplatesShellModal` `needsSession` gate already excludes them automatically — the file is unchanged. Library / direct YAML / file Apply / URL preview Apply / Apply with overrides all funnel through the same `ApplyBuildTemplateV2ToCharacterJSON`, so the new section reaches every entry point. UI: `ImportTemplatePreviewModal.tsx` adds `'equipment'` to `V2_APPLY_SUPPORTED_SECTIONS`, renders an `import-preview-equipment-slots` row from the new `summary.equipmentSlotsPresent`, and tightens the unsupported-section copy to list profile/stats/inventory.workspace/equipment; `TemplateLibraryModal.tsx` enables Apply for equipment-only entries. **No `App.tsx` change, no `SortOrderTab.tsx` change, no `ApplyOverridesPanel.tsx` change, no `WeaponLevelOverridePanel.tsx` change, no equipment override panel, no new App method, no auto-add of missing inventory items, no storage resolver.** `frontend/wailsjs/go/models.ts` regenerated for the two added fields. Manual validation 2026-06-01: equipment export → import → Apply happy path, game-load reload confirmation, missing-item warning, ambiguous match, explicit clear, combo rejection, regression sweep across Phase 5 / 5D.2 / 6 / 6b / 7a / 7a.2 / 9 paths — user-confirmed `manual OK`. Phase 7c talisman writer + apply, Phase 7d spell writer + apply, optional lifting of the equipment + inventory.workspace combo restriction, EquippedGreatRune through the template path, slots 11/16, and quick / pouch items remain future work.
- **Phase 7d.0 → 7d.4 — v2 spells import / preview / apply (shipped 2026-06-02)** — end-to-end spells loadout in Templates v2 through five disciplined sub-phases on `feature/templates-v2-spell-writer-foundation`. **7d.0** (commit `6cb2e60`) ships `backend/core/spell_writer.go::PatchEquippedSpell(slot, slotIndex, spellID) error` — single-slot writer for the 14-slot `EquippedSpells` region with strict pre-validation (nil slot, slot-index range, uninitialised offset, out-of-bounds), idempotent no-op when target bytes match, and no hash side effects (the 7d.3 batch writer owns hash[10]); constants `EquippedSpellSlotCount = 14`, `EquippedSpellSlotSize = 8`, `EquippedSpellEmptySentinel = 0xFFFFFFFF`, `EquippedSpellOccupiedFollower = 0xFFFFFFFF`. **7d.1** (commit `7ac60d0`) ships the schema additions in `backend/templates/schema.go`: `TemplateSections.Spells *SpellsSection`, `TemplateSelection.Spells *SectionSelection`, `SpellsSection` with 14 named pointer fields `Spell1..Spell14`, `SpellSlotRef { BaseItemID uint32; Name string }`, prefix constants (`SpellItemIDPrefix = 0x40000000`, `SpellItemIDPrefixMask = 0xF0000000`), canonical iteration order, allowlist + validators that reject any non-`0x4XXXXXXX` prefix at ingest. Both sorceries AND incantations share the `0x40000000` prefix in the SaveForge DB (`backend/db/data/sorceries.go`, `incantations.go`); the earlier assumption of a separate `0x60XXXXXX` incantation prefix is incorrect and is intentionally **not** documented anywhere. `BaseItemID == 0` is the explicit-clear sentinel; the save-level `0xFFFFFFFF` never appears in the public schema. Nil pointer = leave the live slot unchanged. **7d.2** (commit `fad1315`) ships the export builder + preview/summary: `ExportV2Options.EquippedSpellsRaw []uint32` (strict-reject when length ≠ `SpellSlotCount` AND `selection.spells` is selected), `buildSpellsSection` mapping raw `0xFFFFFFFF` → `&SpellSlotRef{BaseItemID: 0}` and other raw → `&SpellSlotRef{BaseItemID: SpellItemIDPrefix | rawID}`, and `ImportPreviewSummary.SpellSlotsPresent []string` populated via the new helper. Apply remains blocked until 7d.3. **7d.3** (commit `5c3e538`) wires the spells section into the v2 apply path through `(s *SaveSlot) WriteSpells(writes []SpellWrite) error` (pre-validates EVERY write before any byte mutation, rejects duplicate slot indices, dispatches per write through `PatchEquippedSpell`, recomputes **only** hash[10] inline; the global `RecalculateSlotHash` stays unwired in production, `WriteEquipment` continues to own hash[7] / hash[8]). New `db.ItemIDToMagicParamID(itemID uint32) uint32 { return itemID & 0x0FFFFFFF }` — 28-bit mask mirroring `ItemIDToHandlePrefix`; a 16-bit mask would truncate high-payload spell IDs and is regression-guarded. `app_templates_v2_spells.go::resolveSpellWrites` decodes the section into `[]core.SpellWrite` (`BaseItemID == 0` → empty sentinel; `BaseItemID != 0` → defensive prefix re-check + DB membership check via `db.GetItemData(id).Category` must be `"sorceries"` or `"incantations"` + `db.ItemIDToMagicParamID` → raw spell ID; unknown valid-prefix → `IssueCodeUnknownItem` warning + skip). `ApplyTemplateV2Result.SpellSlotsApplied int`. The resolver block sits after the equipment resolver and before the inventory resolver; the `WriteSpells` call sits **after** `vm.MapViewModelToSlot` AND **after** `slot.WriteEquipment` — the post-VM placement is critical because the VM flush rewrites the EquippedSpells region from the cached VM and would silently clobber any earlier spells write. Bindings regen for `spellSlotsApplied` + `spellSlotsPresent`; `frontend/wailsjs/runtime/*` files show mode-bit flip 644 → 755 only with zero content diff. **7d.4** (commit `9e8aabe`) lights up the frontend UI: `V2_APPLY_SUPPORTED_SECTIONS` in `ImportTemplatePreviewModal.tsx` extends with `'spells'`, a new `import-preview-spell-slots` row renders directly under the equipment row, and the unsupported-section tooltip widens accordingly; `TemplateLibraryModal.tsx` ORs `selectedSections.includes('spells')` into `v2HasApplyableSections` so spells-only library entries enable Apply without a session; `TemplatesShellModal.tsx` unchanged (spells do not need a session). `CreateTemplateV2Modal.tsx` **intentionally untouched** — the backend create-from-character path does not yet pass `EquippedSpellsRaw`, so the Spells checkbox is gated by the planned Phase 7d.4b (equipment is in the same deferred state for the same reason). Tests: +5 cases in `ImportTemplatePreviewModal.test.tsx`, +3 cases in `TemplateLibraryModal.test.tsx`; `cd frontend && npx vitest run src/components/templates` → 247 / 247 pass. Manual validation 2026-06-02 user-confirmed `manual OK`: Test A (full 14-slot loadout — 6 occupied + 8 explicit clear, `selection.spells: true`) and Test B (partial leave-unchanged — per-field selection of spell1/spell2/spell3 only, slots 4–14 retain pre-apply state) both passed end-to-end through Import preview → Apply → Save. Verified spell IDs: Catch Flame `0x40001770` (incantation), Glintstone Pebble `0x40000FA0` (sorcery), Rock Sling `0x40001266` (sorcery), Heal `0x40001915` (incantation), Rancorcall `0x40001388` (sorcery); the earlier briefing that named `0x40001388` as Glintstone Pebble was wrong — `0x40001388` is Rancorcall.
- **Phase 8C.1 — v2 items / layout App + UI wiring (shipped 2026-06-08)** — App layer + Create modal export-only wiring for the Phase 8C `sections.items`, `sections.inventoryLayout`, `sections.storageLayout`. `app_templates_v2.go::buildAndValidateTemplateV2FromCharacter` now derives an `ItemsLayoutSource` from the live character snapshot whenever the JSON selection includes `items`, `inventoryLayout`, or `storageLayout`; the four character-source endpoints (`ExportBuildTemplateV2JSONFromCharacter`, `ExportBuildTemplateV2YAMLFromCharacter`, `PreviewBuildTemplateV2FromCharacter`, `SaveBuildTemplateV2FromCharacterToLibrary`) all reach the new path because they share the helper. New private `(*App).buildItemsSourceForCharacter(charIndex)` reuses `editor.BuildSnapshot` under the existing `saveMu.RLock + slotMu[charIndex].Lock` audit pattern (`app_save_audit.go`); no save mutation, no edit session required, no raw GA handles surface to JS. `backend/templates.ImportPreviewSummary` adds three new counts — `itemsEntries`, `inventoryLayoutCount`, `storageLayoutCount` — read directly from `tpl.Sections.{Items,InventoryLayout,StorageLayout}` (zero on v1 documents and v2 documents that did not opt in). `selectedSectionsForTemplate` extends with `items` / `inventoryLayout` / `storageLayout` so `LibraryTemplateEntry.SelectedSections` also carries them, and the modal-level `V2_APPLY_SUPPORTED_SECTIONS` allowlist automatically classifies all three as unsupported — Apply stays disabled with an extended tooltip ("Items / inventoryLayout / storageLayout are export-only"). `CreateTemplateV2Modal.tsx` adds a "Containers" section with three checkboxes (Items, Inventory layout, Storage layout) flagged with an "Export-only" pill; **layout checkboxes are disabled until Items is checked** (decision: explicit gate over auto-checking, mirrors the `BuildV2Template` guard and prevents implicit state changes from surprising the user). Unchecking Items clears both layout selections and re-disables the layout checkboxes. `buildSelectionJSON` gains an optional `containers` argument that emits top-level `items: true` / `inventoryLayout: true` / `storageLayout: true` flags (matches the backend parser). `ImportTemplatePreviewModal.tsx` renders the new counts under the v2 metadata block plus a single export-only note when any of the three are non-zero. `frontend/wailsjs/go/models.ts` regenerated for the three added summary fields only (3-line diff). Apply gating stays intact: items-only selection in a Wails-side roundtrip is rejected by `ApplyBuildTemplateV2ToCharacterJSON` because the apply gate still requires profile / stats / inventory.workspace / equipment / spells. Tests: new `app_templates_v2_items_test.go` (export with items only, export with items + layout, layout-without-items rejection, preview counts in summary, library save preserves items/layout/SelectedSections, items-only apply blocked); +5 cases in `CreateTemplateV2Modal.test.tsx` (containers render, disabled-without-items, selection JSON wire format, Items+InventoryLayout combo); +3 cases in `ImportTemplatePreviewModal.test.tsx` (counts surface, Apply stays disabled for items templates, hidden when zero); existing tooltip-text assertion updated to the extended copy. Validation: `go test ./backend/templates/... -count=1`, `go test . -run 'Test.*Template|...|Test.*Item|Test.*Layout|Test.*Preview' -count=1`, `go vet ./backend/templates/... ./`, `go build ./backend/... ./`, `cd frontend && npx tsc --noEmit`, `cd frontend && npx vitest run src/components/templates src/wails-bindings.contract` — all green (265 / 265 frontend). **Not implemented (Phase 8D+)**: apply path for items / inventoryLayout / storageLayout, inventory / storage writers, manual layout validation, weapon-level override apply for imported items, public JSON exchange (still removed). `frontend/wailsjs/runtime/*` content unchanged; `frontend/package.json.md5` unchanged.
- **Phase 8C — v2 items / layout export-only builder (shipped 2026-06-08)** — backend builder for `sections.items`, `sections.inventoryLayout`, `sections.storageLayout` on top of the Phase 8B schema. Scope is export-only: `templates.BuildV2Template` accepts a new `ItemsSource *ItemsLayoutSource` carrying `editor.EditableItem` slices for inventory + storage; the builder stable-sorts each container by `EditableItem.Position`, generates a deterministic per-template `entryID = "<container>_<4-digit zero-padded post-sort index>"` (`inv_0000`, `sto_0042`), and emits layout entries with compact 0..N-1 positions after per-row skips. Per-row skips with notice (no fatal): `baseItemID==0`, `quantity==0`, `category` outside the Phase 8B allowlist — rationale: v2 items span every category so one stray row should not kill the whole export. Weapon upgrade mapping: `MaxUpgrade=25` → `standard`, `MaxUpgrade=10` → `somber`, `MaxUpgrade=0` → `none`; non-weapon rows emit no upgrade fields. AoW emission mirrors the v1 rule — only `CurrentAoWStatus="custom"` with non-zero `CurrentAoWItemID` reaches the schema. `BuildV2Template` selection guards fail-closed when items/layout selection is set without an `ItemsSource`, or when layout selection is set without items selection (layout refs require items). YAML round-trip and `TemplateLibrary.SaveTemplate`/`LoadTemplate` preserve the new sections; the v1 apply gate (`sections.inventory.workspace`) is NOT touched, so a Phase 8C-emitted template carries the new sections but does not become apply-enabled. 17 new structural tests in `backend/templates/export_v2_items_test.go`. Validation: `go vet ./backend/templates/...`, `go test ./backend/templates/... -count=1`, `go test . -run 'Test.*Template|Test.*Item|Test.*Layout|...' -count=1`, `go build ./backend/... ./` — all green. **Not implemented in this phase**: App layer wiring (`app_templates_v2.go` does not yet route `ItemsSource` from the live workspace session — Phase 8C.1), UI selection checkboxes for items/layout (Phase 8C.1), apply path for items/layout (Phase 8D+), writer mutations of inventory/storage (Phase 8D+), manual validation (deferred to end of items/layout milestone). `frontend/wailsjs/runtime/*` content unchanged.
- **Phase 8B — v2 items / layout schema foundation (shipped 2026-06-08)** — schema-only additions to `backend/templates` that lock the data contract for the upcoming v2 items / inventory layout / storage layout / apply-options surface. **No exporter, no importer, no apply path, no UI, no Wails bindings** — that work lands in Phase 8C+. New top-level `BuildTemplate.ApplyOptions` (sub-DTOs: `items`, `inventoryLayout`, `storageLayout`, `weaponLevelOverride`); new `sections.items.entries[]` flat list of `TemplateItemEntryV2` records keyed by template-local `entryID` (stable string slug, unique within the template); new `sections.inventoryLayout` / `sections.storageLayout` with `{entryRef, position}` rows that reference an existing items entry; new boolean-only `selection.items` / `selection.inventoryLayout` / `selection.storageLayout` keys; `HasAnySelected()` updated. Validation is strict: 18-category allowlist mirroring `backend/db/data` (`melee_armaments`, `ranged_and_catalysts`, `shields`, `ashes_of_war`, `head`, `chest`, `arms`, `legs`, `talismans`, `sorceries`, `incantations`, `tools`, `crafting_materials`, `bolstering_materials`, `arrows_and_bolts`, `key_items`, `gestures`, `dlc`); `quantity == 0` fail-closed (clear/remove semantics belong to apply mode, not entry payload); `location` allowlist `inventory|storage|both`; `upgradeKind` discriminator `standard|somber|none` (empty string ≡ `none`); per-kind upgrade-level ranges (standard 0–25, somber 0–10, none requires no level); `ashOfWarItemID == 0` rejected (omit field to mean any-AoW); duplicate `entryID` rejected; layout `entryRef` must match an existing items entry, duplicate refs and duplicate positions rejected; `applyOptions` mode allowlists (`addMissing|updateExisting|merge|replace` for items, `ignore|append|reorderOnly|replace` for layouts — `replace` is the destructive opt-in for both); `WeaponLevelOverride.useTemplateLevels=true` mutually exclusive with `standardOverride`/`somberOverride`; per-override ranges enforced; bare `WeaponLevelOverride{}` (all nil + `useTemplateLevels=false`) is valid and means "leave live levels untouched." YAML KnownFields(true) keeps refusing unknown top-level keys; `applyOptions` is now a known key. 30 new structural tests in `backend/templates/schema_items_layout_test.go` cover every allowlist branch, the fail-closed branches, the duplicate-itemID-different-entryID case, layout reference validation, and a YAML marshal/parse round-trip. Validation: `go vet ./backend/templates/...`, `go test ./backend/templates/... -count=1` — all green. v1 documents are not affected (Phase 8B sections are additive on v2 only). `frontend/wailsjs/runtime/*` content unchanged. Phase 8C onward will add the exporter / importer / preview / apply / UI on top of this schema foundation.
- **Phase 8A — drop public JSON template exchange (shipped 2026-06-08)** — YAML is now the **only** publicly accessible format for sharing build templates. Removed Wails App methods `ExportBuildTemplateJSON`, `ExportBuildTemplateToFile`, `PreviewBuildTemplateImportJSON`, `PreviewBuildTemplateImportFromFile`, `ApplyBuildTemplateToWorkspaceFromFile`, `ExportLibraryBuildTemplateToFile`; demoted `ApplyBuildTemplateToWorkspaceJSON` to internal `applyBuildTemplateToWorkspaceFromJSON` still consumed by `ApplyBuildTemplateFromLibrary` so already-stored v1 library entries remain applyable. Removed library helper `backend/templates.(*TemplateLibrary).ExportTemplateToFile`. Frontend: removed `ExportTemplateModal.tsx` + tests, the entire `SortOrderTab` "Export Template ▾" dropdown + Template Library entry + weapon-level override panel, and `TemplateLibraryModal`'s per-entry JSON "Export" button + `onExportedToFile` prop. JSON survives strictly as internal storage (library on-disk format) and as canonical hand-off between YAML preview, library save, and v2 apply paths (`SaveImportedBuildTemplateJSONToLibrary`, `ApplyBuildTemplateV2ToCharacterJSON`, `ExportBuildTemplateV2JSONFromCharacter` carry-over). Global Templates shell (sidebar entry) is now the only place users interact with templates; spec/56 §"Phase 8A" + Polish twin document the precise removal list and what stays as internal contract. Validation: `go test ./backend/templates/... -count=1`, `go test . -run 'Test.*Template|Test.*BuildTemplate|Test.*Library|Test.*Import|Test.*Export|Test.*Apply' -count=1`, `go vet ./backend/templates/...`, `cd frontend && npx tsc --noEmit`, `cd frontend && npx vitest run src/components/templates` + SortOrderTab + wails-bindings contract — all green. Manual validation deferred to Phase 8G (end of v2 items/layout milestone). Wails bindings updated manually (`wails` CLI not installed locally); no `frontend/wailsjs/runtime/*` content drift.
- **Phase 9 — URL import with SSRF guards (shipped 2026-05-31)** — `Templates → Import from URL… → Preview → Save to Library / Apply to character / Apply with overrides`. New `PreviewBuildTemplateImportYAMLFromURL(rawURL)` Wails endpoint wraps `backend/templates/url_import.go::FetchYAMLFromURL`, which implements all 13 §12.3 guards (HTTPS-only; pre-connect IP filter on literal and DNS-resolved addresses including loopback / RFC1918 / link-local / ULA / multicast / broadcast / cloud-metadata `169.254.169.254` + `fd00:ec2::254`; redirect re-check on every hop; max 3 redirects; 1 MiB `io.LimitReader` body cap; 10 s total / 5 s idle / TLS / header / dial timeouts; strict TLS with system root CAs and no `InsecureSkipVerify`; identifying User-Agent; Content-Type allowlist; no auth / cookies / custom headers; strict struct-typed YAML decode reused unchanged; no auto-refresh; the fetch alone never mutates anything). The URL preview reuses the existing `ImportTemplatePreviewModal` + `LoadedTemplatePreview { Report, JSON, Path }` shape, so Save to Library, Apply to character (Phase 5D.2), and Apply with overrides (Phase 6) ship without modification on the URL surface. **No library schema change** — `sourceURL` is not persisted in this phase; the library still records what was saved, not where it came from. `App.tsx` untouched; only `frontend/wailsjs/go/main/App.{d.ts,js}` regenerated for the new binding. Out of scope: authenticated downloads, domain allowlist, URL auto-refresh, direct apply without preview.

---

## Planned

### 🟡 Templates v2 — Phase 7d.4b+: spells create-bridge + appearance + multi-char pack

Continuation of the foundation + Phase 5 profile/stats Apply + Phase 6 apply-time overrides + Phase 6b weapon level override + Phase 9 URL import + Phase 7a v2 inventory.workspace Apply + Phase 7a.2 weapon level override on the v2 inventory.workspace path + Phase 7b.0 equipment writer foundation + Phase 7b.1 v2 `sections.equipment` end-to-end + Phase 7c v2 talisman apply through `sections.equipment.talisman1..5` + Phase 7d.0 → 7d.4 v2 spells import / preview / apply through `sections.spells` shipped above. v2 apply now covers profile, stats, inventory.workspace, equipment (weapons + ammo + armor + talismans), and spells; the create-from-character flow still only exports profile/stats (equipment and spells producers hand-author YAML today). EquippedGreatRune through the template path, appearance, and multi-character pack remain future work.

- **Phase 7d.4b** — extend `BuildTemplateV2ExportOptions` with `EquippedSpellsRaw []uint32`, wire `buildAndValidateTemplateV2FromCharacter` to read `slot.Data` at the EquippedSpells region (via `core.readSpellIDs`) and pass the result into `templates.BuildV2Template` whenever `selection.spells.HasAny()`, then add the `Spells` checkbox to `CreateTemplateV2Modal.tsx`. The existing strict-length reject in `buildSpellsSection` (Phase 7d.2) makes the wiring safe by construction. Today producers hand-author the spells YAML; this bridge closes the create-from-character gap. Equipment is in the exact same deferred state for the same reason and will likely land alongside spells in this same bridge phase.
- **Phase 7b.2 (optional)** — lift the `sections.equipment + sections.inventory.workspace` combo restriction once a workspace-backed equipment model or an explicit auto-commit gesture is in place; today the writer needs a fresh `slot.GaMap` and the workspace only commits when the user clicks Save changes
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
