# 56 — Templates v2 (Partially Implemented Extension)

> **Type**: Design doc
> **Status**: 🔄 Partially implemented — Phase 0..6 + Phase 6b + Phase 7a + Phase 7a.2 + Phase 7b.0 + Phase 7b.1 + Phase 7c + Phase 9 shipped (additive `version: 2` schema, global Templates library shell, public YAML import/export, create-from-character flow for profile/stats with per-field selection, Save to Library, v2 metadata badge in library and preview, **Phase 5 = library Apply + direct imported-YAML Apply for profile/stats via `ApplyBuildTemplateV2FromLibraryToCharacter` and `ApplyBuildTemplateV2ToCharacterJSON` on the canonical JSON produced by the import preview, Phase 6 = apply-time overrides for the same profile/stats subset on both surfaces via frontend-only canonical-JSON mutation forwarded to the same JSON endpoint, Phase 6b = apply-time weapon level override for the existing v1 `inventory.workspace` Apply path through an additive `WeaponLevelOverride` field on `app_templates.go::ApplyTemplateOptions`, applied after each template-side weapon patch via `editor.UpdateWeapon` + `editor.ClampUpgrade(req, MaxUpgrade)`; entirely workspace-only, no v2 schema change, no equipment writer; UI controls live inside the existing `SortOrderTab.tsx` Templates dropdown, Phase 7a = first real v2 apply path for `inventory.workspace` routed through the active `InventoryEditSession`/`InventoryWorkspaceSnapshot`, gated by the new `GetActiveInventoryEditSessionForCharacter(charIdx)` lookup and the new additive `ApplyTemplateV2Options.SessionID`; missing session for an inventory-bearing template → `IssueCodeInventorySessionRequired`, unknown/wrong-character session → `IssueCodeInventorySessionInvalid`; mixed profile+stats+inventory.workspace apply is atomic via dual slot+workspace snapshot rollback; Phase 6b weapon level override stayed a v1 dropdown feature in Phase 7a and was wired into the v2 path in Phase 7a.2 below, Phase 7a.2 = apply-time weapon level override threaded into the Phase 7a v2 `inventory.workspace` apply path through an additive `WeaponLevelOverride` field on `ApplyTemplateV2Options` (reusing the v1 type and `validateWeaponLevelOverride` validator verbatim); runtime-only option (never inside the canonical JSON); new `WeaponLevelOverridePanel` embedded inside the existing `ApplyOverridesModal` and rendered only when `selection.inventory.workspace` is present; fast library Apply still sends no override; structurally-valid override + profile/stats-only template silently ignored; `weapon_level_clamped` / `weapon_unupgradeable` warnings flow into `ApplyTemplateV2Result.Preview.Warnings`; v1 `SortOrderTab.tsx` Phase 6b dropdown untouched, Phase 9 = `https://` URL import through `PreviewBuildTemplateImportYAMLFromURL` under the full §12.3 SSRF guard list; the URL preview reuses the same `ImportTemplatePreviewModal`, so Save to Library / Apply to character / Apply with overrides all ship unchanged on the URL surface**). Phase 7b.0 added the backend-only `SaveSlot.WriteEquipment` foundation for weapon / ammo / armor slots. Phase 7b.1 wired that foundation into the v2 apply path through a new `sections.equipment` schema section: equipment templates export, preview, and apply end-to-end without an Inventory Edit Session; the `equipment + inventory.workspace` combo is hard-rejected (GaMap freshness). Phase 7c extends `sections.equipment` with the five talisman slots `talisman1..5` (ChrAsmEquipment indices 17–21, hash 8) — intentionally **not** as a separate `sections.equippedTalismans` section: talismans live in the same `ChrAsmEquipment` struct as weapons/ammo/armor and reuse the Phase 7b.1 resolver, combo guard, preview row, frontend gate, and rollback. Pouch gating: active slots = `1 + profile.talismanSlots`; refs beyond the active count warn + skip with `talisman_slot_pouch_insufficient`; Talisman5 non-empty always warn + skip (vanilla cap = 4 active); `talisman5 baseItemID = 0` clear always allowed; mixed `profile.talismanSlots + equipment.talismanN` uses the template's value before equipment apply runs. Apply for spells / appearance remains blocked — Phase 7d / 8 (spell writer, appearance via preset, multi-character pack) remain design-only. Additive extension of the implemented Build Template subsystem documented in [55-build-template](55-build-template.md).
> **Scope**: Addytywne rozszerzenie istniejącego `saveforge.build-template` JSON v1 do `version: 2` — z publicznym formatem YAML do udostępniania na zewnątrz, nowym sidebar entry point `Templates`, granular selection model, sekcjami całej postaci (profile, stats, equipment, talismans, spells, appearance via preset), single-character first, weapon level override przy apply, plików `.yaml` import/export, importu z URL z pełnymi guardami bezpieczeństwa oraz późniejszą fazą multi-character pack. Document **does not** redefine the v1 baseline — it inherits it from [55-build-template](55-build-template.md).

---

## 1. Title, status and scope

| Aspect | Value |
|---|---|
| Document number | 56 |
| Document type | Design doc — partially implemented extension |
| Status | 🔄 Partially implemented. Phase 0..6 + Phase 6b + Phase 7a + Phase 7a.2 + Phase 7b.0 + Phase 7b.1 + Phase 7c + Phase 9 shipped (Phase 5 = library Apply + direct imported-YAML Apply for the profile/stats subset only; Phase 6 = apply-time overrides for the same profile/stats subset, frontend-only mutation of the canonical JSON forwarded to the existing Phase 5 endpoint; Phase 6b = apply-time weapon level override for the existing v1 `inventory.workspace` Apply path through an additive `WeaponLevelOverride` runtime option on `app_templates.go::ApplyTemplateOptions`, applied after each template-side weapon patch via `editor.UpdateWeapon` + `editor.ClampUpgrade(req, MaxUpgrade)`; workspace-only, no v2 schema change, no equipment writer, UI controls inside the existing `SortOrderTab.tsx` Templates dropdown; Phase 7a = first real v2 apply path for `inventory.workspace` routed through the active `InventoryEditSession`/`InventoryWorkspaceSnapshot` via the new `GetActiveInventoryEditSessionForCharacter` lookup and the new additive `ApplyTemplateV2Options.SessionID`; mixed profile+stats+inventory.workspace apply is atomic via dual slot+workspace snapshot rollback; Phase 7a.2 = apply-time weapon level override threaded into the Phase 7a v2 inventory apply path through an additive `WeaponLevelOverride` field on `ApplyTemplateV2Options` (reusing the v1 type and validator verbatim); runtime-only option carried through `ApplyTemplateV2Options`, never inside the canonical JSON; new `WeaponLevelOverridePanel` embedded inside the existing `ApplyOverridesModal` and rendered only for templates that select `inventory.workspace`; fast library Apply sends no override; warnings flow into `ApplyTemplateV2Result.Preview.Warnings`; Phase 9 = `https://` URL import through `PreviewBuildTemplateImportYAMLFromURL` under the full §12.3 SSRF guard list, reusing the same `ImportTemplatePreviewModal` as the file-import path so all three downstream actions ship unchanged on the URL surface); Phase 7b.0 = backend-only `(s *core.SaveSlot).WriteEquipment([]EquipmentWrite) error` foundation for ChrAsmEquipment slots 0–9 + 12–15 (weapons + ammo + armor); strict GaMap-present-only handle resolution, validate-then-mutate atomicity, targeted hash 7 / 8 recompute, no App method / Wails binding / frontend / schema change; Phase 7b.1 = end-to-end v2 `sections.equipment` apply through the Phase 7b.0 writer — schema, export builder, app-layer scanner, resolver against `slot.Inventory.CommonItems` (storage not searched), `WriteEquipment` dispatch inside the existing rollback scope, four new issue codes (`equipment_inventory_combo_unsupported`, `equipment_item_not_in_inventory`, `equipment_item_ambiguous`, `equipment_slot_invalid`), preview row + frontend gating in `ImportTemplatePreviewModal` + `TemplateLibraryModal`, no equipment override panel; equipment-only templates do NOT require an Inventory Edit Session and the `sections.equipment + sections.inventory.workspace` combo is hard-rejected; Phase 7c = v2 talisman apply by extending the Phase 7b.1 `sections.equipment` schema with `talisman1..5` (ChrAsmEquipment indices 17–21, hash 8) instead of introducing a separate `sections.equippedTalismans`; `backend/core/equipment_writer.go` gains a fourth class `slotClassTalisman` (accepts only the `ItemTypeAccessory` 0xA0 handle prefix), the 5 talisman enum values, and the hash 8 widening to include indices 17–21; encoded slot value = `GaMap[handle]` directly (talisman handles already carry the 0x20 prefix in GaMap, mirroring how ammo encodes goods); the resolver gates non-empty talisman refs against an active-pouch capacity = `1 + effective profile.talismanSlots` (clamped to `MaxProfileTalismanSlots = 3`, hence cap = 4), refs beyond the active count emit a new `talisman_slot_pouch_insufficient` warning + skip; Talisman5 always trips the warning when populated with `baseItemID > 0` (vanilla cap = 4 active slots), `baseItemID = 0` for Talisman5 always clears; mixed templates that also set `profile.talismanSlots` use the template value so a +3 pouch bump in the same template unblocks `talisman4`; resolver still searches only `slot.Inventory.CommonItems` (no storage), missing talisman = `equipment_item_not_in_inventory` warn + skip, ambiguous = first-wins warning, no auto-add; the `equipment + inventory.workspace` hard reject is unchanged; no frontend source-code change (`V2_APPLY_SUPPORTED_SECTIONS` already includes `equipment`, the existing `import-preview-equipment-slots` row enumerates whichever slot keys the summary lists); Phase 7d / 8 / 10 remain design-only. Each later phase requires a separate user approval per the workflow in `~/.claude/CLAUDE.md`. |
| Baseline reference | [55-build-template](55-build-template.md) — implemented `version: 1`, JSON only, inventory + storage only, local library at `$UserConfigDir/EldenRing-SaveEditor/templates/`. |
| Schema key | Remains `saveforge.build-template` (no rename). Implemented. |
| Schema version | Reader range `1 ≤ version ≤ MaxSchemaVersion (=2)`. v1 builder still emits `SchemaVersion = 1`; the explicit v2 builder (`backend/templates/export_v2.go`) emits `version: 2`. Implemented. |
| External public format | YAML (`.yaml`). JSON remains for the existing local library and for backward-compatible import. Implemented for v1 payloads and for v2 documents produced by the v2 builder. |
| First user-visible entry | Sidebar blue `Templates` button immediately above `Save as...` in `frontend/src/App.tsx` (existing `<aside>` footer block); opens `TemplatesShellModal.tsx`. Implemented. |
| Character scope (first iteration) | Single character. Multi-character pack is deferred to a later phase (§15). |
| URL import | **Shipped (Phase 9, 2026-05-31)**. Backend-only fetch through `PreviewBuildTemplateImportYAMLFromURL` → `backend/templates/url_import.go::FetchYAMLFromURL` under the full §12.3 SSRF guard list (HTTPS-only, pre-connect IP filter on literal and DNS-resolved addresses, redirect re-check ×3, 1 MiB body cap, 10 s total / 5 s idle timeouts, strict TLS with system root CAs, identifying User-Agent, Content-Type allowlist, no auth / cookies / custom headers, strict struct-typed YAML decode reused unchanged, no auto-refresh; the fetch alone never mutates anything). The URL preview reuses the existing `ImportTemplatePreviewModal`, so Save to Library / Apply to character / Apply with overrides all ship unchanged on the URL surface. **No library schema change** — `sourceURL` metadata is not added to the library in this phase. |
| Production code change | Phase 0..6 + Phase 6b + Phase 7a + Phase 7a.2 + Phase 7b.0 + Phase 7b.1 + Phase 7c + Phase 9 shipped; later phases remain design-only. Detail in §17 and §17a. |

---

## 2. Implemented baseline inherited from spec/55

The following is the implemented state of Build Template as documented in [55-build-template §2-§20](55-build-template.md). Templates v2 **builds on top of these facts**; it does not contradict or rewrite them.

| Area | Implemented (v1) | Source of truth in code |
|---|---|---|
| Schema key | `saveforge.build-template` | `backend/templates/schema.go::SchemaKey` |
| Schema version | `1` | `backend/templates/schema.go::SchemaVersion` |
| Format | JSON (indented) | `backend/templates/`, `encoding/json` |
| Section coverage | `inventory.workspace.{inventoryItems,storageItems}` only | `backend/templates/schema.go::TemplateSections` |
| Per-item fields | `baseItemID`, `name`, `category`, `quantity`, `upgrade`, `infusionName`, `aowItemID (*uint32, omitempty)`, `container`, `position` | `backend/templates/schema.go::TemplateItem` |
| Export | `BuildTemplateFromSnapshot`, `ExportBuildTemplateJSON/ToFile` | `backend/templates/export.go`, `app_templates.go` |
| Preview | `ParseBuildTemplateJSON`, `PreviewBuildTemplateImport`, `PreviewBuildTemplateImportJSON/FromFile` | `backend/templates/import.go`, `app_templates.go` |
| Apply (RAM-only, append-only) | `ApplyBuildTemplateToWorkspaceJSON/FromFile`, capacity preflight, `deepCopySnapshot` rollback | `app_templates.go` |
| Local library | `$UserConfigDir/EldenRing-SaveEditor/templates/` with `_index.json` (`LibraryIndexVersion=1`), atomic writes, sanitized filenames | `backend/templates/library.go` |
| UI entry today | `Export Template ▾` dropdown in `frontend/src/components/SortOrderTab.tsx` (Inventory → Weapons & Sort Order) + three modals in `frontend/src/components/templates/` | as cited |
| Concurrency model | Per-session lock (`backend/editor/session.go`) + slot lock (`slotMu[i]`) + ascending lock order documented in [55 §10](55-build-template.md) | as cited |
| Integrity gate | Pre-flight in `AddItemsToCharacter` refuses on duplicate acquisition indices; repair via `RepairDuplicateInventoryIndices` | `app_save_integrity.go`, `backend/core/inventory_index_repair.go` |
| AoW handling | Stable `aowItemID` only; never raw handles; fail-closed on unknown compat | [55 §6.4, §9.5, §13](55-build-template.md), [54-ash-of-war](54-ash-of-war.md) |

Templates v2 **must not** alter any of the above for templates that declare `version: 1`. Readers of v1 payloads must continue to behave exactly as today.

---

## 3. Product goals

1. **Single, central UI entry point.** Introduce a sidebar blue `Templates` button immediately above `Save as...`. Surface Library / Create / Import / Apply Preview from one place, decoupled from `Weapons & Sort Order`.
2. **Public, human-readable YAML format for sharing.** Add YAML as the exchange format for export-to-file, import-from-file, and import-from-URL. The format must be hand-editable by advanced users.
3. **Planned coverage of more character data.** Expand from inventory + storage to additional safe-semantic sections of a single character: profile (name/level/runes), stats, equipment selection (slot → item id), talisman item ids and slot count, spell item ids, optional appearance via stable preset name only.
4. **Granular selection.** Per template and per apply, the user picks exactly which sections (and within sections, which sub-groups, e.g. per stat) participate. Selection is encoded in the YAML itself so that a recipient sees only what the sender intended to share.
5. **Local library stays JSON-compatible.** The existing on-disk library and `_index.json` keep working unchanged for v1 payloads. v2 payloads stored locally remain JSON inside the library; YAML is only the external/sharing format.
6. **File and URL import.** Import a `.yaml` template from a local file or directly from an `https://` URL. Both flows preview before apply; URL fetching never modifies the save or the library without an explicit confirm step.
7. **Weapon level override at import time.** When applying a template, the user may override the upgrade level of imported weapons separately for standard (+0..+25) and somber/special (+0..+10) items. Default is `Keep`.
8. **Safety-first apply.** A planned `TemplateApplyPlan` abstraction is responsible for combining v2 sections into a single, atomically-applied or fully-rolled-back operation that respects the existing integrity gate, edit session locking, and post-write validation.
9. **No regression to v1.** Existing local templates, the existing dropdown in `SortOrderTab`, and all existing tests must continue to pass without modification. The dropdown is retained as a shortcut in the first implementation phase; its removal or repositioning is a separate, later decision.
10. **Multi-character packs as a later iteration.** The schema must leave room for `all_characters` packs (§15), but the first iteration ships single-character only.

---

## 4. Non-goals and explicitly excluded unsafe fields

### 4.1. Non-goals for the first Templates v2 iteration

- Importing or exporting **raw event flag IDs** of any kind.
- Editing **progression / unlocks** (graces, bell bearings, cookbooks, bosses, NG+) via raw flag manipulation. Such effects, where allowed at all, must remain mediated by existing named modules (e.g. `app_pvp.go:ApplyPvPPreparation`) and by the implicit POST-FLAGS hooks of `AddItemsToCharacter` documented in [50-item-companion-flags](50-item-companion-flags.md) (companion-flag SET at `app.go:569-578`, pickup-flag SET at `app.go:743+`).
- Exporting or importing **raw FaceData blobs** (`backend/core` FaceData section). Appearance is allowed only by stable preset name from `data.Presets`, and only in a later phase.
- Replacing the implemented v1 subsystem. Templates v2 is purely additive.
- Auto-migrating existing on-disk v1 library entries to v2.
- Removing the existing `Export Template ▾` dropdown in `SortOrderTab` in the first phase.
- Any new HTTP fetch surface beyond the strictly-guarded URL template import (see §12).
- Settings export, app-config sync, or any persistence shared with the templates directory.

### 4.2. Fields that are forbidden in the public YAML and in any planned v2 schema

The portable-payload rule of v1 ([55 §7](55-build-template.md)) is retained and extended. The following must **never** appear as fields in a public YAML template, regardless of section:

- `GaItemHandle` (any prefix: `0x80…` weapon, `0xC0…` AoW, etc.).
- `AoWGaItemHandle` raw value (sentinels `0x00000000`, `0xFFFFFFFF`, or any allocated `0xC0…`).
- Absolute `AcquisitionIndex` values.
- `NextAcquisitionSortId`, `NextEquipIndex`.
- `GaMap` entries.
- Raw event flag IDs.
- Binary offsets within slot data.
- Encryption IV, MD5 / hash bytes, AES key material.
- `SessionID`, `BaseRevision` (SHA256 prefix), `BaselineEditableHandles`.
- Steam ID, PSN identifiers.
- Raw save blobs (FaceData binary, regulation slices, world geometry, etc.).
- Per-item `originalHandle`, `currentAoWHandle`, `uid`, or any `Pending*` workspace-internal fields.

What the public YAML **may** carry (only semantic values):

- Item / weapon / AoW / armor / talisman / spell item IDs (`uint32`).
- Weapon `upgrade` integer + `infusionName` string + `aowItemID` `uint32`.
- Relative `position` integer (array index).
- Profile fields: `name`, `level`, `runes`, `gender`, `voiceType`.
- Stats: `vigor`, `mind`, `endurance`, `strength`, `dexterity`, `intelligence`, `faith`, `arcane`.
- Talisman slot count (a small integer, clamped at apply by the existing Pouch-upgrade machinery).
- Equipment slot assignment (slot name → item ID).
- Spell list (item IDs).
- Appearance preset name (string from `data.Presets`).
- Metadata: `name`, `description`, `author`, `tags`, `createdAt`, `appVersion`, `sourceCharacterName`, `sourceURL`.

---

## 5. Compatibility strategy

### 5.1. Schema key and version

- Schema key remains `saveforge.build-template`. **No rename.**
- A new accepted version `2` is added. The accepted reader range becomes `1 ≤ version ≤ 2`. Writers of expanded templates produce `version: 2`.
- v1 documents continue to parse and apply exactly as today.
- The reader must perform an additive forward-fill when reading v1: missing sections default to "not selected" / "not present" (semantically equivalent to current behavior).
- v2 introduces optional new top-level keys; missing keys mean "not present in this template". A v2 document that contains only `sections.inventory.workspace` is semantically equivalent to a v1 document.

### 5.2. JSON vs YAML

- The existing on-disk **local library remains JSON-internal**. Atomic writes, `_index.json`, sanitized filenames, recovery semantics — all preserved exactly per [55 §19](55-build-template.md).
- A new public **YAML representation** is added for export-to-file, import-from-file, and import-from-URL. The YAML must be a 1:1 mapping of the same Go structs that back the JSON form. Switching format must not lose or transform any field.
- Importing a YAML file is allowed to **save it to the library as JSON** (transparent transcoding) so the library on disk stays homogeneous.
- A v1 JSON file imported into the library is **never** rewritten to v2 on disk. Auto-migration is explicitly out of scope (§4.1).

### 5.3. No destructive rewrites

- Reading a v1 template never overwrites or upgrades the file on disk.
- Writing a v2 template never touches v1 entries in `_index.json` other than by adding new entries.
- `RebuildIndex` continues to skip unparseable / non-validating files (per [55 §19.2](55-build-template.md)).

### 5.4. v1 readers vs v2 documents

- A v1-only reader (e.g. an older app build) encountering a v2 document must reject it via the existing `ValidateBuildTemplate` "unsupported version" path. There is no silent downgrade.
- A v2 reader must always accept v1 documents.

---

## 6. Planned UI

### 6.1. Sidebar entry point

- **Location**: `frontend/src/App.tsx`, inside the existing footer block `<div className="p-4 border-t border-border bg-muted/5 space-y-3">` (current line range ~503–515), inserted **immediately above** the existing `Save As` button.
- **Style**: blue button, matching the existing blue patterns in the app (e.g. `border-blue-500/40 bg-blue-500/10 text-blue-600 hover:bg-blue-500/20` from the header `Review Changes` button, or a darker variant equivalent to `DatabaseTab.tsx`).
- **Visibility**: always visible whenever the app shows the sidebar. Library / Preview / Import / Export remain usable without an active `InventoryEditSession`. Actions that require an active character or workspace (Create from current save, Apply to workspace) are disabled until that context exists.

### 6.2. Templates shell

The button opens a single Templates UI surface (modal or panel — the exact shape is a UI decision for the implementation phase). Conceptually it offers four tabs / sections:

| Section | Requires open character? | Requires open session? |
|---|---|---|
| Library | no | no |
| Create | yes | yes |
| Import (file + URL) | no | only when "Apply directly" is chosen |
| Apply Preview | yes (target character) | yes (target session) |

### 6.3. Retention of the existing dropdown

- The `Export Template ▾` dropdown in `frontend/src/components/SortOrderTab.tsx` is **retained as a shortcut** in the first implementation phase.
- It continues to call the existing Wails bindings exactly as it does today.
- A later, separately approved decision determines whether to remove it, redirect it to the new sidebar surface, or keep it permanently as a power-user shortcut.

### 6.4. State management

- The Templates surface follows the existing React `useState`-per-component pattern (no global store). See [§5 of the audit report context], and existing modals like `cloneModal`, `deleteModal`, `diffModal` in `App.tsx`.
- If the surface needs `sessionID`, the implementation phase decides whether to (a) lift `sessionID` to `App.tsx` state, (b) build a lighter-weight library-only modal independent of session, or (c) keep `sessionID` in `SortOrderTab` and pass it down through props/context. This is an open product decision (§18).

---

## 7. Planned single-character data sections

The following sections may appear in a v2 template, marked individually as supported in the selection mask (§8). All sections are optional; a v2 template containing only the workspace section is valid and is functionally identical to a v1 document.

The `Apply path` column below distinguishes between (a) **existing** writers that Templates v2 may reuse and (b) **new write paths** that must be designed and added before the corresponding section can be applied. The classification is grounded in verified code paths — see §13.6 and the Source-of-truth column below.

| Section key | Phase status | Content | Apply path (planned) | Risk class | Existing writer? |
|---|---|---|---|---|---|
| `inventoryWorkspace` (v1 key `inventory.workspace` retained by the reader) | inherited from v1 | as today | as today (`editor.AddItem` / `editor.UpdateWeapon` → `ApplyWorkspaceSave`) | requires-dependent-writers (v1) | yes (v1) |
| `profile` | planned | `name`, `level`, `runes` (Souls/SoulMemory), `class`, `clearCount` (cap 7), `scadutreeBlessing`, `shadowRealmBlessing` | reuses existing `vm.ApplyVMToParsedSlot` → `slot.SyncPlayerToData` (same path as `app.go::SaveCharacter`, lines 297–345 and 823–860) | safe-semantic | yes |
| `stats` | planned | per-stat scalars: Vigor / Mind / Endurance / Strength / Dexterity / Intelligence / Faith / Arcane | same as `profile` (mapped by `vm.ApplyVMToParsedSlot`, written by `slot.SyncPlayerToData`) | safe-semantic | yes |
| `profile.talismanSlots` (additional Pouch slot count `0..3`) | planned | `uint8`, clamped to `0..3` (game cap; total slots = `1 + value`) | reuses existing `vm.ApplyVMToParsedSlot` → `slot.SyncPlayerToData` (field `Player.TalismanSlots` at `OffTalismanSlots`, written at `structures.go:841`) | safe-semantic | yes |
| `appearance.gender` and `appearance.voiceType` | later phase (Phase 8) | stable preset name (preferred), or explicit `gender` / `voiceType` byte values | **Not** mapped by `vm.ApplyVMToParsedSlot` from VM. Must go through the existing `app_appearance.go::ApplyPresetToCharacter` / `SetCharacterGender` helpers, which take `slotMu[charIdx]`, push undo, and call `SyncPlayerToData` themselves | safe-semantic (preset only) | yes (helpers) |
| `equipment` (equipped slots: `weaponRight1/2`, `weaponLeft1/2`, `armorHead/Chest/Arms/Legs`, plus optional `equippedGreatRune`) | later phase (Phase 7b) | slot name → item ID | **No public write API today** for `ChrAsmEquipment` slots 0..9, 12–15, 17–21 — [06-equipment §App-level write API](06-equipment.md) is explicit ("None — equipment is read-only from the UI perspective"). The only existing exception is `EquippedGreatRune` (slot 10), already written by `SyncPlayerToData` at `structures.go:850–852`. **A new controlled writer is required** for the remaining slots (encoded item-ID form, hash 7/8 dependency — see [06-equipment §hash](06-equipment.md)). | requires-new-writer | **no** (except GreatRune) |
| `equippedTalismans` (which talismans occupy `ChrAsmEquipment` slots 17–21) | later phase (Phase 7c) | array of up to 5 talisman item IDs in slot order | **No public write API today** — equipped talismans live in the same `ChrAsmEquipment` block as armor; they are read-only with the rest of equipment. **A new controlled writer is required** (companion to Phase 7b) and must respect the Pouch limit from `profile.talismanSlots`. Distinct from `profile.talismanSlots` (the additional slot count, which already has a writer). | requires-new-writer | no |
| `spells` (equipped sorcery / incantation / gesture loadout — 14 spell slots) | later phase (Phase 7d) | spell / sorcery / incantation / gesture item IDs | **No public write API today.** `EquippedSpells` (14 slots) is currently only referenced by hash-recompute (`backend/core/hash.go:150–195`, `section_hash.go:24`). **A new controlled writer is required.** Distinct from the unlocked-spell inventory entries (which are part of `inventoryWorkspace` and already supported by v1). | requires-new-writer | no |
| `weapons` (overlay on `inventoryWorkspace`) | planned | optional explicit `upgrade`, `infusionName`, `aowItemID` per inventory / storage weapon already enumerated in the workspace section | reuses existing `editor.UpdateWeapon` workspace mutation; see §14 | safe-semantic (level / infusion), requires-dependent-writers (AoW) | yes (v1) |
| `appearance.preset` | later phase (Phase 8) | stable `preset` name only (must exist in `data.Presets` — `backend/db/data/presets.go::Presets`) | reuses existing `app_appearance.go::ApplyPresetToCharacter` (writes FaceData blob + gender + voiceType via the preset, under `slotMu[charIdx]`). Raw FaceData blob is **never** in the YAML. | safe-semantic (preset only) | yes |

What is intentionally **not** in v2 first iteration:

- No raw FaceData blob field. Appearance, if applied, goes only through the preset path.
- No raw event flag manipulation. Where progression-like effects are needed, they remain mediated by the implicit POST-FLAGS hooks of `AddItemsToCharacter` ([50-item-companion-flags](50-item-companion-flags.md); see `app.go:569-578` for companion-flag SET and `app.go:743+` for pickup-flag SET) and by named modules like `ApplyPvPPreparation`.
- No PvP preparation state inside the template directly. If/when needed, a later phase may add `modules` carrying a list of named module presets (e.g. `pvp.colosseums`) without ever encoding raw flag IDs.
- No raw `Player.Gender` or `Player.VoiceType` bytes outside the appearance preset path. Even though both fields are byte-writeable by `SyncPlayerToData`, `vm.ApplyVMToParsedSlot` does **not** map them from the VM — they are exclusively driven by `app_appearance.go` helpers today, and Templates v2 must keep it that way.
- No raw write of the additional Pouch event flags. The additional Talisman Pouch slot count (`profile.talismanSlots`, `0..3`) is a plain `u8` field in `PlayerGameData` and is written through the existing profile/stats path; it does not require touching any Pouch event flag.

---

## 8. Granular selection model

### 8.1. The `selection` object

A v2 template carries a `selection` object whose shape mirrors `sections`. Each leaf is either a boolean (`true` = include in apply, `false` = ignore even if data is present) or, for per-element groups, a list of explicit keys.

Properties:

- **Authoritative for apply.** The applier acts only on sections (and sub-keys) where `selection` is `true`. Sections present in the YAML but not selected are treated as metadata for review only.
- **Authoritative for export.** When the sender exports a template, only selected fields are written. Unselected fields are omitted (not zeroed).
- **Forward-compatible.** Unknown keys in `selection` are ignored by the reader; missing keys are treated as `false`.
- **Per-stat granularity.** `selection.stats` may be `true` (apply all 8), `false` (apply none), or an object of per-stat booleans (`{ vigor: true, mind: false, ... }`). The same per-element pattern is allowed for talismans (per-item-id), equipment (per-slot), and spells (per-item-id).

### 8.2. UI implication

- Create / Export modal: the user picks which sections and (where applicable) which fields to include. The choices are written into `selection` so the recipient sees the same shape on import.
- Apply Preview modal: the user can further narrow the selection at apply time (e.g. "import everything except Endurance"). The narrowed selection is local — the YAML on disk is not rewritten.
- Defaults: `selection.inventory.workspace = true` when the section is present (to mirror current v1 behavior). All other sections default to `true` when present and `false` when absent.

### 8.3. Validation rules (planned)

- `selection` keys must match a known section / subkey. Unknowns produce a warning, not an error.
- A section selected as `true` but absent from `sections` produces an error (`selection_missing_section`).
- A section present in `sections` but unselected is allowed and silently skipped by the applier.

---

## 9. Public YAML direction (illustrative high-level example only)

The following is a non-normative illustration to anchor the discussion. It is **not** a finalized schema. The implementation phase produces the canonical schema, generated from Go struct tags so that JSON and YAML share a single source of truth.

```yaml
schema: saveforge.build-template
version: 2
appVersion: 1.1.0-alpha
createdAt: 2026-06-01T12:00:00Z

metadata:
  name: RL150 INT Cold-infused
  description: PvP build for invasions
  author: someone
  tags: [pvp, int, cold]
  sourceCharacterName: Tarnished      # informational quote, read-only
  sourceURL: https://example.org/builds/rl150-int.yaml  # only if imported from URL

selection:
  profile: true
  stats:
    vigor: true
    mind: true
    endurance: true
    strength: false
    dexterity: false
    intelligence: true
    faith: false
    arcane: false
  equipment: true
  equippedTalismans: true
  spells: true
  inventoryWorkspace: true

sections:
  profile:
    name: Tarnished
    level: 150
    runes: 0
    talismanSlots: 2   # additional Pouch slot count (0..3); total = 1 + value

  stats:
    vigor: 60
    mind: 25
    endurance: 25
    strength: 12
    dexterity: 18
    intelligence: 80
    faith: 9
    arcane: 7

  # equipment, equippedTalismans, spells, and appearance.preset
  # are LATER PHASES — they require new write paths (see §7 and §13.6).
  # They are shown here for shape, not for the first apply phase.
  equipment:
    weaponRight1: 4030000   # base item ID, no encoded upgrade/infusion
    weaponRight2: null
    weaponLeft1:  2000000
    weaponLeft2:  null
    armorHead:    10000000
    armorChest:   10010000
    armorArms:    10020000
    armorLegs:    10030000

  equippedTalismans:
    items: [80000000, 80010000, 80020000, 80030000]  # up to 5; respects profile.talismanSlots

  spells:
    sorceries: [40000000]
    incantations: []
    gestures: [50000000]

  appearance:
    preset: geralt   # must resolve in data.Presets

  inventoryWorkspace:
    inventoryItems:
      - baseItemID: 4030000
        quantity: 1
        upgrade: 10
        infusionName: Cold
        aowItemID: 2168029136
        container: inventory
        position: 0
    storageItems: []
```

Notes on the example:

- No `GaItemHandle`, no `AoWGaItemHandle`, no `acquisitionIndex`, no offsets — see §4.2.
- The `inventoryWorkspace` key is the preferred v2 spelling. The reader must continue to accept the v1 key `inventory.workspace` for full backward compatibility (a v1 document re-read as v2 stays semantically unchanged). The exact alias policy is open decision §18.
- `equipment` references items by base ID only; the apply path enforces that the item is present (or, by a later opt-in, can be added) in inventory before the slot assignment is committed (see §13.7).
- `equippedTalismans` is the **equipped** loadout (which talismans occupy slots 17–21 of `ChrAsmEquipment`). It is separate from `profile.talismanSlots`, which is the **count** of additional Pouch slots (0..3). The two fields must never be conflated; see §7.
- `selection.stats` shows mixed-mode (object form) granular selection. A boolean shortcut is also legal.
- A canonical, exhaustive schema with field types and constraints is a deliverable of the implementation phase that ships YAML support; it is not produced by this document.

---

## 10. Local library and external export strategy

### 10.1. Local library (existing — unchanged)

- Path: `$UserConfigDir/EldenRing-SaveEditor/templates/`.
- Per-template file: `<sanitized-name>-<id-tail>.json`, mode 0644.
- Index: `_index.json` (`LibraryIndexVersion = 1`).
- Atomic writes (`atomicWriteFile`), recovery semantics, lazy init — unchanged from [55 §19](55-build-template.md).
- A v2 template stored in the library is stored as JSON. The same Go struct backs both forms; the serialiser is the only difference.

### 10.2. Planned additions to library metadata

`LibraryTemplateEntry` may be extended (additive only) with:

- `scope`: `"single"` (default) / `"pack"` (multi-character; reserved for §15).
- `selection` summary: a compact representation of which sections are present in the file (e.g. counters or a small mask), so the library list can show "Profile + Stats + Inventory" without re-parsing the file.
- `sourceURL`: only when the entry originated from a URL import.
- `importedFrom`: free-text origin (e.g. file path, URL host).

All additions are optional. Older library entries without these fields remain valid.

### 10.3. External export

- A new Wails-bound App method (planned name: `ExportLibraryTemplateAsYAMLToFile(id)`) opens a `SaveFileDialog` with `.yaml` as the primary filter and `.json` as a secondary filter. The user picks the desired format.
- The existing `ExportLibraryBuildTemplateToFile(id)` (JSON) remains available for backward compatibility.
- Export from the active workspace gains a parallel `ExportBuildTemplateAsYAMLToFile(sessionID, opts)` method.
- Cancelling the dialog returns the existing sentinel (empty `Path`, no error) — same convention as v1.

### 10.4. Loss-of-data prevention

- Existing operations that touch the library on disk (`SaveTemplate`, `DeleteTemplate`, `RenameTemplate`, `RebuildIndex`) remain atomic.
- A future-only consideration (not in scope for the first phase): periodic snapshot of `_index.json` to an `_index.bak.json` before `RebuildIndex`, to allow user-driven recovery in edge cases.

---

## 11. File import flow

### 11.1. Trigger

- From the sidebar Templates surface, `Import → From file…`.
- Optional: also accessible from the current `SortOrderTab` dropdown (`Import Template Preview…`) in the first phase, for shortcut continuity.

### 11.2. Flow

1. User selects a `.yaml` or `.json` file via `OpenFileDialog`. Cancellation returns the existing sentinel (no error, no toast).
2. The backend reads the file with a hard size cap (planned: 1 MB; subject to confirmation in the implementation phase).
3. Format detection: extension first, content-type heuristic (magic bytes) second.
4. Parse into the same Go structs that back JSON. YAML must be parsed in strict, struct-typed mode (no `interface{}` decode, no `!!include` / aliases / anchors that expand cross-document — see §12.6).
5. `ValidateBuildTemplate` (extended to accept `version: 2` and the new sections).
6. `PreviewBuildTemplateImport` (extended to validate per-section content).
7. Preview report is shown to the user in a non-destructive modal. The modal offers two next steps:
   - **Save to library** — transcodes to JSON and writes through the existing library `SaveTemplate` path. Does not touch the open save or workspace.
   - **Apply to workspace** — requires an active session; otherwise the button is disabled. Goes through the apply architecture in §13.
   - **Cancel** — discards the parsed payload.

### 11.3. Errors and warnings

- Parse failure surfaces as an `ImportPreviewReport` with a single `structure_invalid` error, exactly as today for malformed JSON ([55 §9.2](55-build-template.md)).
- Schema/version mismatch surfaces as `schema_invalid`.
- Per-section validation produces dedicated issue codes (planned, additive). Their exact naming is a deliverable of the implementation phase; they follow the existing `IssueCode*` convention.

### 11.4. Atomicity

- Reading the file never modifies the save.
- Saving to the library uses the existing atomic write path.
- Apply to workspace follows the architecture in §13.

---

## 12. URL import flow and security constraints

URL import is **shipped (Phase 9, 2026-05-31)**, end-to-end manually validated against an `https://` endpoint serving a v2 YAML template. The flow and guards below describe the implemented behaviour; future widening (auth, domain allowlist, auto-refresh, direct apply without preview, optional `sourceURL` library metadata) is **not** part of Phase 9 and requires separate approval before any work begins.

### 12.1. High-level flow

1. User picks `Import → From URL…` from the sidebar Templates surface.
2. User pastes an `https://` URL.
3. Backend Go performs the fetch under strict guards (§12.3).
4. Response body is parsed as YAML (or JSON, by content-type / heuristic).
5. Schema validation runs identically to file import (§11.5).
6. A preview is shown together with the **source URL** clearly displayed.
7. The user may either save to the library or proceed to Apply Preview. Cancelling discards the payload. **The fetch itself never modifies the save or the library.**

### 12.2. Where the fetch lives

- Fetching must be implemented in the **backend (Go)**. The frontend never performs the HTTP request.
- Rationale: backend has full control over TLS, redirect policy, IP filtering, body size, content-type validation. Frontend `fetch` in WKWebView would inherit CSP/CORS surprises and complicate auditability.

### 12.3. Required guards for the first URL-import implementation

- **Scheme**: only `https://`. Reject any other scheme (`http`, `file`, `ftp`, `data`, `javascript`, `about`, `blob`, `chrome`, `chrome-extension`, etc.) at parse time.
- **DNS + IP destination filter (defense in depth)**: resolve the host, reject all of the following ranges before connecting, and re-verify after every redirect:
  - Loopback: `127.0.0.0/8`, `::1`.
  - Link-local: `169.254.0.0/16`, `fe80::/10`.
  - RFC1918 private: `10.0.0.0/8`, `172.16.0.0/12`, `192.168.0.0/16`.
  - ULA: `fc00::/7`.
  - Multicast / broadcast / wildcard / quad-zero.
  - Cloud metadata endpoints (e.g. `169.254.169.254`, `fd00:ec2::254`).
- **Custom redirect policy**: at most 3 redirects; each must pass the IP filter again (TOCTOU defense).
- **Body size cap**: hard `io.LimitReader` — **1 MiB (`1 << 20`)**, decided at Phase 9 implementation and exported as `templates.URLImportMaxBodyBytes`.
- **Timeouts**: total request timeout **10 s** (`URLImportTotalTimeout`); idle / TLS-handshake / response-header / dial timeouts **5 s** each (`URLImportIdleTimeout`). Decided at Phase 9 implementation.
- **TLS**: system root CAs only, `MinVersion: tls.VersionTLS12`, **no** `InsecureSkipVerify`, no custom CA injection from URL.
- **User-Agent**: stable identifying string — `"EldenRing-SaveForge Templates-v2-URL-import"` (`URLImportUserAgent`). Decided at Phase 9 implementation.
- **Content-Type acceptance list**: `application/json`, `application/yaml`, `application/x-yaml`, `text/yaml`, `text/plain`. Reject `text/html`, `application/octet-stream`, etc.
- **No body interpretation as code or commands.** YAML is parsed strictly into typed Go structs.
- **No YAML includes / aliases beyond local anchors that resolve to scalar values.** No cross-document references, no `!!include`, no custom tags, no executable types. The implementation phase picks the YAML library (likely `gopkg.in/yaml.v3` decode into typed structs) with these constraints enforced.
- **No authorization (basic / bearer / cookies) in the first phase.** Authenticated downloads are out of scope.
- **No follow-up auto-refresh from the URL.** A URL is only ever fetched on explicit user action.

### 12.4. State invariants

- **The fetch alone never mutates anything.** No file is written, no library entry is created, no workspace is touched until the user clicks an explicit confirm in Apply Preview or in the library save step.
- All apply-side rules (§13) apply to URL-imported templates without exception.

### 12.5. Errors and warnings

- DNS failure / connection refused / TLS error / timeout / body size exceeded / forbidden IP destination / disallowed scheme / bad content-type / parse failure / schema mismatch — each produces a precise user-visible error tagged by category. None of these conditions writes to the library or to the save.

### 12.6. What URL import is **not**

- Not a templating engine. The downloaded document is data, not instructions.
- Not a substitute for the library. URL import always lands in preview first.
- Not a way to bypass any v1 / v2 validation rule.

---

## 13. Preview / apply architecture

### 13.1. The planned `TemplateApplyPlan`

Implementing v2 sections without a coordinating layer would either duplicate apply logic or risk partial state. A planned `TemplateApplyPlan` abstraction is introduced as the single coordinator:

1. **Plan phase** (no mutation): takes a parsed v2 template + the current open save + apply options. Produces an explicit plan: list of per-section operations, their required preconditions, and a per-section issue/warning list. Calls existing validators (`ValidateBuildTemplate`, `PreviewBuildTemplateImport`) and adds new per-section validators.
2. **Confirm phase** (no mutation): the plan is rendered to the user as the apply preview. The user can re-narrow the selection (§8.2). The plan is regenerated.
3. **Apply phase** (mutation, atomic per slot): the plan is executed under the existing per-slot lock model. Each section uses its existing writer; the plan only orchestrates ordering and rollback.
4. **Post-apply validation**: re-runs the integrity gate scan (`GetSaveInventoryIntegrityReport` / `core.ScanDuplicateInventoryIndices`) and any planned per-section sanity check. A regression triggers rollback.

### 13.2. Rollback / atomicity

- **One snapshot per affected slot** taken before any section mutation. On any error (preview, capacity, mutation, post-apply validation), the slot is restored from snapshot.
- For workspace mutations, the existing `deepCopySnapshot` rollback path is reused; for direct slot edits (profile, stats, equipment, talismans, spells), a per-slot byte-level snapshot is used, modelled on `core.SnapshotSlot` / `core.RestoreSlot` already used by the integrity gate.
- The plan never starts mutation until all per-section validators have passed.
- The plan is allowed to abort mid-way only inside the per-slot critical section; the rollback restores the slot to pre-plan state.

### 13.3. Interaction with the integrity gate

- The pre-flight guard in `AddItemsToCharacter` (refuse on duplicate acquisition indices) remains in force. The plan must not bypass it.
- The plan re-checks integrity before mutation and again after mutation. A post-apply integrity regression is treated as a hard failure and triggers rollback.

### 13.4. Interaction with edit session locking

- The plan acquires the same locks as the underlying writers, in the same ascending order (`saveMu.RLock` → `lifecycleMu[charIdx]` → `editSessionsMu` → `sess.mu` → `slotMu[charIdx]`) — see [55 §10](55-build-template.md) and the existing audit notes.
- The plan does not invent a new lock. It composes existing ones inside one critical section.
- Save / SaveAs is forbidden while the plan is mid-mutation; this is naturally enforced by `slotMu[charIdx]`.

### 13.5. Inventory / workspace section

- Apply of `sections.inventory.workspace` continues to be RAM-only inside the active edit session, exactly as today. The user still must click `Save changes` to persist.
- The plan never calls `SaveInventoryWorkspaceChanges` automatically.

### 13.6. Per-section write paths (verified against code)

The apply layer routes each section to a different writer. The plan must compose these explicitly; there is **no** single per-section `slot.Sync…ToData` family beyond `SyncPlayerToData`. The classification below is grounded in code:

- **`profile` (name / level / runes / class / clearCount / Scadutree / Shadow Realm) and `stats` and `profile.talismanSlots` (additional Pouch slot count 0..3) and `weapons` overlay on `inventoryWorkspace`** → reuse the existing path used by `app.go::SaveCharacter`: `vm.ApplyVMToParsedSlot(&charVM, &slot)` (see `backend/vm/character_vm.go:297-345`) followed by `slot.SyncPlayerToData()` (see `backend/core/structures.go:823-860`). All writes happen under `slotMu[charIdx]`, with a per-slot snapshot taken before the call and rollback on any error.
- **`inventoryWorkspace`** → reuse the existing v1 apply path (RAM-only `editor.AddItem` / `editor.UpdateWeapon`, persisted by the user clicking `Save changes` which calls `ApplyWorkspaceSave`). The plan never calls `SaveInventoryWorkspaceChanges` automatically.
- **`appearance.preset`, `appearance.gender`, `appearance.voiceType`** → reuse the existing `app_appearance.go::ApplyPresetToCharacter` and `SetCharacterGender` helpers. **`vm.ApplyVMToParsedSlot` does not map Gender / VoiceType from the VM**, even though `SyncPlayerToData` writes them — so the apply plan must route these through the appearance helpers, not through the profile/stats path. This means the appearance section depends on a separate writer that already exists and is independently undo-managed.
- **`equipment`, `equippedTalismans`, `spells`** → **no existing public write API.** The editor today is read-only for `ChrAsmEquipment` slots 0..9, 12–15, 17–21 ([06-equipment](06-equipment.md): "App-level write API for equipment slots | ❌ None") and for `EquippedSpells` (14 slots in `backend/core/hash.go:150` and `section_hash.go:24`, referenced only by hash recompute). The single existing exception is `EquippedGreatRune` (slot 10), already written by `SyncPlayerToData` at `structures.go:850-852`. Templates v2 **requires new controlled writers** in `backend/core/` for the remaining equipment slots, for equipped talismans, and for the spell loadout. Each new writer must (a) honour the encoded item-ID form per slot type ([06-equipment §encoded item-ID form](06-equipment.md)), (b) respect the hash 7/8 dependency, (c) take `slotMu[charIdx]`, (d) be covered by per-platform round-trip tests (PC + PS4). These new writers are introduced in the corresponding Phases 7a / 7b / 7c (see §17).
- Per-section snapshot for rollback: the plan takes one snapshot of `slot.Data` per affected slot before any mutation in the slot's critical section, using the `core.SnapshotSlot` / `core.RestoreSlot` pattern already used by the integrity gate.

### 13.7. Equipment slot referential integrity

- If `sections.equipment` references an item that is not present in the target character's inventory, the plan's default behavior is **warning + leave the slot unchanged** (no silent auto-add). An optional opt-in `addMissingEquippedItems: true` may be considered in a later phase, but must not be the default.

### 13.8. Appearance via preset (later phase)

- When introduced, appearance apply goes through `app_appearance.go::ApplyPresetToCharacter` only. Raw FaceData is never written from a template.

### 13.9. Post-apply user step

- For inventory.workspace, the user still clicks `Save changes` to persist to `slot.Data`.
- For direct slot edits (profile, stats, equipment, talismans, spells), the changes are already in `slot.Data` after apply, but only persisted to disk on the next `WriteSave`/`SaveAs` — matching the existing behavior of `SaveCharacter`.

---

## 14. Weapon level override semantics

### 14.1. Goal

Allow the user to, at apply time, override the upgrade level of weapons that **come from the template**, separately for standard and somber/special weapons, without touching:

- Weapons already on the target character that are not in the template.
- The `infusionName` carried by the template's weapons.
- The `aowItemID` carried by the template's weapons.

### 14.2. UI controls (planned)

Three independent controls on the Apply Preview screen, each defaulting to `Keep`:

| Control | Default | Range |
|---|---|---|
| `Standard weapons (+0..+25)` | `Keep` | `Keep` or `Set to +N` with `0 ≤ N ≤ 25` |
| `Somber/special weapons (+0..+10)` | `Keep` | `Keep` or `Set to +N` with `0 ≤ N ≤ 10` |
| Non-upgradeable (MaxUpgrade=0) | locked at +0 | informational only |

### 14.3. Classification source

- Standard vs somber is read from `backend/db/data/types.go::WeaponStatsV1.IsSomber` and `MaxUpgrade`, which are populated from `regulation.bin` (`EquipParamWeapon`).
- Non-upgradeable weapons have `MaxUpgrade == 0` and are never affected by an override.

### 14.4. Apply path

- The override is applied **in the plan layer** before each weapon is handed to `editor.AddItem` / `editor.UpdateWeapon`. The encoded item ID is recomputed via the existing `editor.encodeWeaponItemID(baseID, level, infusionName)`.
- Per-weapon `MaxUpgrade` from DB is the hard clamp. A request to `Set to +N` with `N > MaxUpgrade` results in `N := MaxUpgrade` and a per-item warning in the report (`upgrade_clamped_by_override`, planned code).
- `infusionName` and `aowItemID` from the template are passed through unchanged.
- Override applies to both `inventoryItems` and `storageItems` if both sections are part of the template.
- Helper location: ✅ Done — the pure clamp helper lives in `backend/editor/weapon.go` as the exported `editor.ClampUpgrade` (relocated from the old `app.go::clampUpgrade`, behaviour byte-for-byte unchanged).
- ✅ Shipped 2026-05-31 (Phase 6b, v1 `inventory.workspace` Apply path only) — the v1 apply path in `app_templates.go::applyTemplateItemsToWorkspace` now calls `applyWeaponLevelOverride` **after** each template-side `editor.UpdateWeapon` patch. The override switches on `editor.EditableItem.MaxUpgrade` (populated from `db.GetItemDataFuzzy` by `editor.AddItem`): `25` consumes `StandardLevel`, `10` consumes `SomberLevel`, `0` is skipped with `templates.IssueCodeWeaponUnupgradeable` ("weapon_unupgradeable") on `report.Warnings`, any other value is a silent skip. Over-cap requests are clamped via `editor.ClampUpgrade(req, MaxUpgrade)` and re-applied through `editor.UpdateWeapon{Upgrade: &clamped}` with `templates.IssueCodeWeaponLevelClamped` ("weapon_level_clamped") on `report.Warnings`. The override never adds, removes, or relocates items and never touches `Infusion` or `AoW`. Mutation stays entirely inside the active `InventoryWorkspaceSnapshot`; no bytes are written to `slot.Data` from the override path, and `SaveInventoryWorkspaceChanges` remains the only commit point. The v2 inventory apply remains gated by the `inventory.workspace` scope guard in `app_templates_v2_apply.go` — Phase 6b is a v1 apply-path option, not a new v2 schema section.

### 14.5. Why a single shared level for all weapons is wrong

- Standard upgradeable weapons cap at `+25`; somber/special cap at `+10`; some weapons cap at `+0`.
- A naive single global `+N` with implicit clamp produces silently inconsistent results (e.g. user picks `+25`, expects uniform reinforcement, gets silently mixed levels).
- Splitting the control into Standard / Somber expresses the user intent precisely and aligns with the data model.

### 14.6. Out of scope for this override

- The override does not change infusion or AoW.
- The override does not add or remove weapons.
- The override does not affect weapons on the target character that are not part of the template.

---

## 15. Multi-character pack as a later phase

### 15.1. Position in the roadmap

Multi-character packs (`scope: pack`) are **deferred** to a later phase. The first Templates v2 iteration ships single-character (`scope: single`) only.

### 15.2. Planned shape (sketch only — final schema deferred)

- The YAML carries `scope: pack` and a list of per-character entries, each with its own `sections` and `selection`.
- The applier requires a mapping `sourceCharacter → destinationSlot` chosen by the user. Default mappings (e.g. identity) require an explicit user confirmation step.
- The applier handles slot occupancy: occupied destination slots require an explicit replace confirmation; an empty destination slot is filled if and only if the user opts in.
- The plan executes each character as its own per-slot critical section, with per-slot rollback on failure. A failure in one slot does not silently leave another slot mutated.

### 15.3. Constraints regardless of when this phase ships

- All forbidden-fields rules (§4.2) and apply rules (§13) apply per-slot identically.
- The mapping UI must be explicit. Implicit / silent slot assignment is forbidden.
- The integrity gate runs per-slot, pre- and post-apply.

---

## 16. Risk matrix

Each risk is classified as one of:

- `safe / straightforward` — reuses existing patterns, minimal new design surface.
- `requires design decision` — needs an open product decision before implementation can begin.
- `high-risk / must not implement without separate approval` — needs an explicit user sign-off at the implementation phase, possibly with extra guardrails.

| Risk | Class | Notes |
|---|---|---|
| Sidebar entry point + new modal shell | safe / straightforward | Reuses existing modal patterns; only new UI surface. |
| Additive schema (`version: 2`, new sections, `selection`) | safe / straightforward | Reader range becomes `1 ≤ v ≤ 2`; v1 reader rejects v2 via existing path. |
| Keeping the library JSON-internal | safe / straightforward | No on-disk migration; recovery semantics unchanged. |
| Adding YAML serializer / deserializer | requires design decision | Library choice + struct-tag policy (single source of truth across JSON + YAML). |
| `selection` semantics (boolean vs object per group) | requires design decision | Per-stat vs per-item-id granularity to be finalized. |
| Profile / stats apply path (Level / Class / Souls / SoulMemory / 8 stats / CharacterName / ScadutreeBlessing / ShadowRealmBlessing / ClearCount / additional `profile.talismanSlots` 0..3) | safe / straightforward | Reuses verified existing path `vm.ApplyVMToParsedSlot` → `slot.SyncPlayerToData` (`app.go::SaveCharacter`). |
| Gender / VoiceType apply path | requires design decision | `vm.ApplyVMToParsedSlot` does **not** map these from the VM; must reuse `app_appearance.go::ApplyPresetToCharacter` / `SetCharacterGender`, not the profile/stats path. |
| Equipment slot write path (`ChrAsmEquipment` slots 0..9, 12–15, 17–21) | requires design decision + new writer | No existing public write API ([06-equipment](06-equipment.md) "App-level write API for equipment slots | ❌ None"). New controlled writer required for Phase 7b; respects hash 7/8 dependency. |
| Equipped talismans write path (`ChrAsmEquipment` slots 17–21) | requires design decision + new writer | Same as equipment; companion to Phase 7b, scheduled as Phase 7c. Must respect `profile.talismanSlots` Pouch limit. |
| Equipped spell loadout write path (`EquippedSpells` 14 slots) | requires design decision + new writer | No existing public write API; only hash recompute references the field today. Phase 7d. |
| Equipment referential integrity (template references item not in target inventory) | requires design decision | Default = warn + skip; opt-in `addMissingEquippedItems` deferred (§13.7). Applies to Phase 7b/7c. |
| Additional Talisman Pouch slot count (`profile.talismanSlots`, 0..3) | safe / straightforward | Already written by `SyncPlayerToData` (`structures.go:841`); pure byte field, no raw event-flag write required. Distinct from equipped-talismans writer. |
| Appearance via preset name | requires design decision | Reuses existing `app_appearance.go::ApplyPresetToCharacter`. Limited to entries in `data.Presets`; raw FaceData blob is a high-risk separate decision. |
| Raw FaceData | high-risk / must not implement without separate approval | Out of scope for first v2 iteration. |
| Raw event flag manipulation | high-risk / must not implement without separate approval | Excluded by §4. Any future opt-in must come with named-module mediation. |
| PvP preparation state in templates | requires design decision | Only via named modules (e.g. `pvp.colosseums`), never raw flags. |
| Weapon level override (Standard + Somber, separate) | safe / straightforward | Reuses existing `editor.ClampUpgrade` (✅ relocated to `backend/editor/weapon.go`, see §14.4) + `encodeWeaponItemID` (`backend/editor/weapon.go`). |
| Inventory / storage rebuild semantics for added weapons | inherited from v1 — safe | Same fail-closed rules. |
| Acquisition indices / `NextAcquisitionSortId` interaction | inherited from v1 — safe | Templates never expose these; the integrity gate continues to guard. |
| AoW handles | inherited from v1 — safe (with care) | Only `aowItemID` in YAML; fail-closed compat check unchanged. |
| Equipment dependent on items not in inventory | requires design decision | See §13.7. |
| File import (YAML) | safe / straightforward | Existing file dialog + new parser. |
| URL import — SSRF, redirect TOCTOU, body size, scheme | high-risk / must not implement without separate approval | Strict guards required (§12). |
| URL import — YAML includes / aliases / executable types | high-risk / must not implement without separate approval | Struct-typed decode only. |
| Schema migration for v2 → v3 in the future | requires design decision | Out of scope; document the policy at the time. |
| Migration / coexistence of `Export Template ▾` dropdown and new sidebar entry | requires design decision | Dropdown retained in the first phase; later decision separate. |
| Multi-character pack | requires design decision | Whole pack feature deferred to a later phase (§15). |
| Per-platform parity (PC vs PS4) for template apply | safe — but to be validated per phase | Both round-trip tests must remain green for every feature phase that touches `backend/core/`. |
| Concurrency with `WriteSave`, edit session lifecycle, clone/delete | inherited from v1 — safe | Plan acquires existing locks in the existing ascending order. |
| Backwards compatibility for users sharing v1 files | safe / straightforward | v2 readers always accept v1 documents. |

---

## 17. Recommended phased implementation plan

Each phase is small, independently shippable, and requires a separate user approval before it starts. No phase commits code without going through the standard workflow in `~/.claude/CLAUDE.md` (Plan → OK → Implementation → Tests → Verification → Git).

### Ordering rationale

The first user-visible value is the public sharing format (YAML) for the **already-implemented** v1 inventory/storage scope, behind a stable sidebar entry. Schema expansion for full-character sections comes after that, because:

- YAML is the headline interoperability feature for the user community and can be delivered against the v1 scope without any save-mutation risk.
- The v1 inventory/storage scope is already stable, tested, and bounded — it is the safest surface on which to stabilise the YAML transport layer.
- New full-character sections require new write paths in `backend/core/` (see §7, §13.6) and must not block delivering YAML for the existing scope.
- Each new write path can then be added independently, behind its own per-phase approval.

### Phase 0 — this document and product decisions (current)

- **Status**: ✅ Shipped.
- **Goal**: produce this design document; resolve open decisions in §18.
- **Files**: this spec + the PL mirror; README / BOOK_PLAN registrations.
- **Backend / Frontend impact**: none.
- **Tests**: none.
- **Manual validation**: review.
- **Risks**: none.
- **Out of scope**: any code change.
- **Requires separate user decision before continuing**: completed.

### Phase 1 — sidebar entry + Templates shell wired to existing v1 backend

- **Status**: ✅ Shipped.
- **Goal**: add the blue `Templates` button in `frontend/src/App.tsx`; open a shell that exposes Library / Import-from-file / Export-from-current-session, all bound to the **existing v1 Wails methods**. No schema change, no apply change.
- **Files (planned scope)**: `frontend/src/App.tsx` (sidebar JSX + modal state), new `frontend/src/components/templates/TemplatesShellModal.tsx` (wrapper), tests for the new shell.
- **Backend impact**: none (reuses existing bindings).
- **Frontend impact**: new wrapper, new sidebar button, possible `sessionID` lift (one of the options in §6.4).
- **Tests**: render tests for the shell; visibility tests for the button; no regression in `SortOrderTab` dropdown.
- **Manual validation**: open the app, confirm the button appears, confirm Library / Import / Export still work exactly as v1.
- **Risks**: minor refactor of `sessionID` passing.
- **Out of scope**: any schema change, any YAML, URL import, granular selection, full-character sections.
- **Requires separate user decision before continuing**: completed.

### Phase 2 — public YAML import / export for v1 inventory + storage

- **Status**: ✅ Shipped (split into 2A backend YAML I/O + 2B frontend dialog wiring).
- **Goal**: introduce a YAML representation of the **existing v1 schema** as the public sharing format. The local library remains JSON-internal per §10.1. Import of `.yaml` files transcodes to JSON for library storage. No new schema fields, no full-character sections.
- **Files (planned scope)**: `backend/templates/yaml.go` (new), `go.mod` (new YAML dependency, strict struct-typed decode), `app_templates.go` (new Wails bindings `ExportBuildTemplateAsYAMLToFile`, `ExportLibraryTemplateAsYAMLToFile`, file import accepts `.yaml`), frontend dialog wiring.
- **Backend impact**: new serializer/deserializer; library on disk stays JSON; existing JSON paths unchanged.
- **Frontend impact**: dialog filters include `.yaml`; preview modal accepts YAML payload identically to JSON.
- **Tests**: YAML ↔ JSON round-trip for v1 payloads; reject unsupported YAML tags / anchors expanding cross-document; reject body that does not validate against `ValidateBuildTemplate`.
- **Manual validation**: export v1 template as YAML, hand-edit the file, re-import, confirm preview matches, confirm apply to workspace works exactly as before.
- **Risks**: YAML library choice — must enforce strict, struct-typed decode (open decision §18 #1, resolved by adopting `gopkg.in/yaml.v3` with struct-typed decode).
- **Out of scope**: schema v2 fields, full-character sections, URL import.
- **Requires separate user decision before continuing**: completed.

### Phase 3 — additive schema v2 + `selection` (export-only, no apply)

- **Status**: ✅ Shipped (split into 3A structural schema draft, 3B.0 apply guard, 3B pure builder for profile/stats, 3C metadata for preview/library, 3C.1/3C.2 App-layer JSON + YAML export and Save to Library from `charIndex`, 3D.0 bindings regen, 3D.1 UI v2 metadata badge, 3D.2a/2b CreateTemplateV2Modal wiring).
- **Goal**: extend `backend/templates/schema.go` to declare `version: 2`, the new optional sections (placeholder shape only), and `selection`. Update `ValidateBuildTemplate` to accept the extended shape. Reader range becomes `1 ≤ v ≤ 2`. Writers can emit v2 documents that contain only the v1 workspace section (semantically equivalent to v1).
- **Files (planned scope)**: `backend/templates/schema.go`, `backend/templates/schema_test.go`, `backend/templates/export.go` (builder extended), `backend/templates/import.go` (validator extended), YAML mapping kept aligned (assuming Phase 2 has shipped first).
- **Backend impact**: pure type extension; no apply-side change yet.
- **Frontend impact**: none.
- **Tests**: extensive schema_test scenarios in both directions, including v1 → v2 reader compat and v2-only-with-workspace round-trip; v1 reader (older app build) must reject v2 cleanly via `ValidateBuildTemplate`.
- **Manual validation**: open an existing v1 library entry; confirm it still loads and applies; export it as v2; confirm round-trip.
- **Risks**: silent JSON / YAML field collisions if tag names overlap — guarded by tests.
- **Out of scope**: apply of new sections, weapon override, equipment / talismans / spells writers.
- **Requires separate user decision before continuing**: completed.

### Phase 4 — export + preview of new safe sections (no apply yet)

- **Status**: ✅ Shipped (CreateTemplateV2Modal drives per-section + per-field selection; preview shows v2 metadata + selected sections / fields; apply button absent for v2; v1 workspace apply unchanged).
- **Goal**: implement the `selection` object on the export side (per-section / per-stat checkboxes) and per-section preview validators. The apply button stays hidden for the new sections in this phase; the v1 workspace apply path is unchanged.
- **Files (planned scope)**: `backend/templates/export.go`, `backend/templates/import.go` (additive per-section validators with new issue codes), `frontend/src/components/templates/ExportTemplateModal.tsx`, `frontend/src/components/templates/ImportTemplatePreviewModal.tsx`.
- **Backend impact**: builder respects `selection`; per-section validators added.
- **Frontend impact**: new UI controls on export; preview renders new sections with warnings/errors.
- **Tests**: builder emits only selected sections / sub-fields; round-trip; per-section preview cases.
- **Manual validation**: export a "stats only" v2 template; preview it; confirm structure and that the apply button is not offered for new sections yet.
- **Risks**: low.
- **Out of scope**: applying new sections.
- **Requires separate user decision before continuing**: completed.

### Phase 5 — apply: profile + stats (minimal `TemplateApplyPlan`) — ✅ Shipped 2026-05-31

- **Goal**: implement the safest subset of the planned `TemplateApplyPlan` (§13). Apply only the fields that the existing `vm.ApplyVMToParsedSlot` actually maps from the VM and that `slot.SyncPlayerToData` writes to `slot.Data`:
  - `profile.name`, `profile.level`, `profile.souls`, `profile.soulMemory` (with the existing `runesCostForLevel` clamp), `profile.clearCount` (cap 7), `profile.scadutreeBlessing`, `profile.shadowRealmBlessing`, `profile.talismanSlots` (additional Pouch slot count 0..3, clamped), `stats.*` (all 8).
  - `profile.class` is intentionally **skipped** by the Phase 5 writer and surfaced through `ApplyTemplateV2Result.Skipped`; `className` is **not** an alias of `class`.
  - All of the above goes under `slotMu[charIdx]` with a per-slot `core.SnapshotSlot` taken first and `core.RestoreSlot` on any error. `clearCount` flags and `ProfileSummary` side effects are recomputed on success.
- **Files (shipped scope)**: `app_templates_v2_apply.go` (`ApplyBuildTemplateV2ToCharacterJSON`, `ApplyBuildTemplateV2FromLibraryToCharacter`, `ApplyBuildTemplateV2FromFileToCharacter`, `ApplyTemplateV2Options`, `ApplyTemplateV2Result` with `Character` typed as `vm.CharacterViewModel`), bindings regenerated for the same symbols, UI in `frontend/src/components/templates/TemplatesShellModal.tsx` + `TemplateLibraryModal.tsx` (library Apply, inline confirm, `mode: "append"`) and `frontend/src/components/templates/ImportTemplatePreviewModal.tsx` (Phase 5D.2 — direct imported-YAML Apply button + gating), `frontend/src/App.tsx` (post-apply refresh of `inventoryVersion`, `saveLoadKey`, slots, undo — reused unchanged by Phase 5D.2).
- **Backend impact**: new apply layer; reuses existing writers exactly. Phase 5D.2 added no backend or bindings changes.
- **Frontend impact**: Apply enabled on library entries and on imported-YAML previews whose `selectedSections ⊆ { profile, stats }`; v2 entries carrying any other section remain disabled; v1 imported templates never show the new v2 Apply button. The direct imported-YAML Apply path reuses `ApplyBuildTemplateV2ToCharacterJSON` against the canonical JSON already produced by the import preview — no second file dialog, no TOCTOU re-read between preview and apply. `ApplyBuildTemplateV2FromFileToCharacter` exists backend/bindings-side but is intentionally left unwired in UI; supported flows are now both `Import YAML → Save to Library → Apply from Library` and `Import YAML → Preview → Apply to character`.
- **Tests**: backend apply happy path; rollback on error; `profile.class` reported in `Skipped`; library + file delegation paths covered; frontend (Phase 5D.2) covers v1 imports never offering the new button, all gating failure paths for v2 imports, click forwarding to `ApplyBuildTemplateV2ToCharacterJSON` with `mode: "append"`, `applied=true` success path (close + toasts + `onCharacterTemplateApplied`), `applied=false` and thrown-error paths (error toast + preview stays open), and Save-to-Library independence.
- **Manual validation**: 2026-05-31 — Phase 5D.1: applied a v2 library entry with profile + stats selection to an active character on `feature/templates-v2-apply-profile-stats`; inline confirm fires; Apply succeeds; selected fields change; post-apply refresh reflects the new state; v1 entries remain disabled in the global shell (no `sessionID`); unsupported v2 entries remain disabled. Phase 5D.2: on `feature/templates-v2-direct-yaml-apply`, importing a v2 YAML with `selectedSections ⊆ { profile, stats }` through `Import YAML from File…` and clicking "Apply to character" applied the same fields the library path applies; `profile.class` skip surfaced via info toast when `class` was selected; the preview closed on success and `App.tsx`'s refresh dance updated the visible state; v1 imported YAMLs continued to show only `Save to Library` with no v2 Apply button; v2 imports carrying unsupported sections kept Apply disabled with the supported-scope tooltip; the Phase 5D.1 library Apply path remained unchanged.
- **Risks**: respected — existing locking and integrity gate were preserved. Phase 5D.2 introduced no new lock surface; it reused Phase 5D.1's endpoint as-is.
- **Out of scope**: Gender / VoiceType (Phase 8 via appearance helpers), equipment / equipped talismans / spells / appearance / weapon-level override; apply-time value editing / overrides for the profile/stats subset shipped in Phase 6 below.
- **Requires separate user decision before continuing**: completed.

### Phase 6 — apply-time overrides for profile + stats — ✅ Shipped 2026-05-31

- **Goal**: let the user edit profile + stats values **before** the apply reaches the backend, on the same surfaces Phase 5 already covers (direct YAML import preview + library list). Reuse the Phase 5 backend writer with no new backend code, no new bindings, and no `App.tsx` change.
- **Approach**: frontend-only mutation of the canonical JSON the user already previewed (direct YAML) or stored in the library (library path). The mutated JSON is posted through the existing `ApplyBuildTemplateV2ToCharacterJSON(charIdx, mutatedCanonicalJSON, { mode: "append" })`; the library "Apply with overrides…" path obtains the canonical JSON for an existing entry through the already-shipped `PreviewBuildTemplateFromLibrary` binding instead of adding a new endpoint.
- **Files (shipped scope)**: `frontend/src/components/templates/ApplyOverridesPanel.tsx` (new, exports `ApplyOverridesPanel`, `ApplyOverridesModal`, and the pure `applyOverridesToCanonical` helper), `frontend/src/components/templates/ImportTemplatePreviewModal.tsx` (second v2 button `Apply with overrides…` next to the existing `Apply to character`), `frontend/src/components/templates/TemplateLibraryModal.tsx` (per-entry `Apply with overrides…` button next to the existing Apply), `frontend/src/components/templates/TemplatesShellModal.tsx` (shared `OverridesSource` discriminator + handlers for both surfaces). Frontend tests across all four components (+1 new test file). Backend, bindings, and `App.tsx` are untouched.
- **Editable scope**: identical to the Phase 5 writer — `profile.{name,level,runes,soulMemory,clearCount,scadutreeBlessing,shadowRealmBlessing,talismanSlots}` and all eight `stats.*`. `profile.class` is rendered read-only with a "Skipped on apply (Phase 5)" hint instead of an editable input.
- **UI ranges (mirror schema validator)**: `level [1, 713]`, `clearCount [0, 7]`, `scadutreeBlessing [0, 20]`, `shadowRealmBlessing [0, 10]`, `talismanSlots [0, 3]`, stats `[1, 99]`. `runes` carries a soft warning above `999_000_000` but is not hard-capped. The backend remains the source of truth for all validation; the UI pre-checks ranges to keep the Apply button honest and surfaces inline error copy per field.
- **Selection semantics**: toggling a field off removes both `sections.{profile,stats}[field]` and `selection.{profile,stats}[field]` in the mutated JSON. Toggling a field on adds both. The Phase 5 contract — "applied = selected ∧ present" — is preserved unchanged.
- **Backend impact**: none. `ApplyTemplateV2Options` keeps its single `Mode` field; the JSON endpoint re-validates everything end-to-end.
- **Frontend impact**: two new buttons (one on imported preview, one per v2 library row); one new modal; one new pure helper; existing `Save to Library` button on the import preview remains independent and persists the original canonical JSON, not the in-modal edits. v1 templates never see the new buttons. v2 templates with unsupported sections (equipment / spells / equippedTalismans / appearance / inventory.workspace) keep both v2 buttons disabled with the existing "profile / stats only in this phase" tooltip. The fast library Apply path through `ApplyBuildTemplateV2FromLibraryToCharacter` is unchanged.
- **Tests**: +43 frontend cases — 19 in `ApplyOverridesPanel.test.tsx` (new), +7 in `ImportTemplatePreviewModal.test.tsx`, +5 in `TemplateLibraryModal.test.tsx`, +12 in `TemplatesShellModal.test.tsx`. Covers rendering / mutation / range validation / soft cap / toggle-off removal / `profile.class` read-only / preservation of non-profile/stats sections / invalid-JSON banner / both surfaces forwarding the mutated JSON / success-close / `applied=false` and thrown-error keeping the modal open / cancel discarding edits / invalid value disabling Apply / fast library Apply path untouched / `PreviewBuildTemplateFromLibrary` returning no canonical JSON surfacing an error toast / `profile.class` skip info toast.
- **Manual validation**: 2026-05-31 — on `feature/templates-v2-apply-overrides`, edited profile + stats values through both the direct YAML import path and the library `Apply with overrides…` path; the edits landed on the selected character without touching unrelated fields; the fast library Apply path remained unchanged; v1 imports continued to show only the legacy `Save to Library` action with no overrides button; v2 imports carrying unsupported sections kept the buttons disabled with the supported-scope tooltip; cancelling the overrides modal discarded edits with no save mutation; `Save to Library` continued to persist the original canonical JSON, ignoring the in-modal edits.
- **Risks**: respected — Phase 6 introduces no new lock surface, no new write path, no new endpoint. The backend Phase 5 contract is the only mutation site.
- **Out of scope**: weapon level override at apply time (Phase 6b below), inventory / storage / equipment / spells / appearance / sort order / world progress edits at apply time, item quantities, URL import, multi-character pack, "Save edited copy" of an in-modal edit back to the library.
- **Requires separate user decision before continuing**: completed.

### Phase 6b — weapon level override for the v1 inventory.workspace Apply path — ✅ Shipped 2026-05-31

- **Goal**: let the user, at apply time on the existing v1 `inventory.workspace` Apply path (the `Apply Template ▾` action inside `SortOrderTab.tsx`), force every standard-upgradeable weapon the template adds to `+N` and every somber/special weapon to `+M`, clamped to each weapon's `MaxUpgrade` from the DB. Default = no override (`Enabled = false`), behaviour byte-for-byte identical to the pre-Phase-6b path. No v2 schema change, no v2 inventory writer, no equipment writer.
- **Files (shipped scope)**: `backend/templates/import.go` (two new `IssueCode*` warning codes: `IssueCodeWeaponLevelClamped = "weapon_level_clamped"`, `IssueCodeWeaponUnupgradeable = "weapon_unupgradeable"`); `app_templates.go` (additive `WeaponLevelOverride` struct, additive `WeaponLevelOverride *WeaponLevelOverride` field on the existing `ApplyTemplateOptions`, `validateWeaponLevelOverride` helper, new `applyWeaponLevelOverride` helper, extended `applyTemplateItemsToWorkspace` signature); `app_templates_weapon_override_test.go` (**new**, ~390 lines, 14 test functions / 16 cases with subtests); `frontend/src/components/SortOrderTab.tsx` (new state, parsers, `buildWeaponLevelOverride` payload builder, override panel inside the existing Templates dropdown — `weapon-override-panel` testid with `weapon-override-enabled` master checkbox and two number inputs `weapon-override-standard` range `0..25` / `weapon-override-somber` range `0..10`, inline `weapon-override-error` validation, override-warning toast filter); `frontend/src/components/SortOrderTab.test.tsx` (+233 lines, new `Phase 6b weapon level override` block, +8 cases); `frontend/wailsjs/go/models.ts` regenerated by `make build`'s internal `wails generate module` step (adds the `WeaponLevelOverride` class and the optional `weaponLevelOverride?: WeaponLevelOverride` field on `ApplyTemplateOptions`). `TemplatesShellModal`, `ApplyOverridesPanel`, `ImportTemplatePreviewModal`, `TemplateLibraryModal`, `App.tsx`, and the v2 apply layer (`app_templates_v2_apply.go`) are intentionally untouched.
- **Runtime option shape (final)**:
  ```go
  type WeaponLevelOverride struct {
      Enabled       bool `json:"enabled,omitempty"`
      StandardLevel *int `json:"standardLevel,omitempty"`
      SomberLevel   *int `json:"somberLevel,omitempty"`
  }
  ```
  Both level pointers are independent: the UI can target standard only, somber only, or both at once. `Enabled = false` makes the entire override a no-op regardless of pointer values; `validateWeaponLevelOverride` rejects `Enabled = true` with both pointers nil and rejects negative levels (`StandardLevel < 0` or `SomberLevel < 0`) with the standard `ApplyBuildTemplate: …` error prefix, **before** any workspace mutation runs.
- **Apply layer**: `applyTemplateItemsToWorkspace` now threads the override through and calls `applyWeaponLevelOverride` **after** each template-side `editor.UpdateWeapon` patch (Upgrade / Infusion / AoW from the template). The helper switches on `added.MaxUpgrade` (already populated by `editor.AddItem` via `db.GetItemDataFuzzy` — see §14.3): `25` → standard (uses `StandardLevel` if non-nil); `10` → somber/special (uses `SomberLevel` if non-nil); `0` → unupgradeable; any other value → silent skip. For standard / somber, the helper computes `clamped := editor.ClampUpgrade(req, added.MaxUpgrade)` and calls `editor.UpdateWeapon(snap, added.Handle, container, editor.WeaponPatch{Upgrade: &clamped})`. If `clamped != req`, a `templates.IssueCodeWeaponLevelClamped` warning is appended to `report.Warnings`. For unupgradeable, a `templates.IssueCodeWeaponUnupgradeable` warning is appended and the override is skipped. The override never adds, removes, or relocates items, and never touches `Infusion` or `AoW`. Mutation stays entirely inside the active `InventoryWorkspaceSnapshot`; no bytes go to `slot.Data` from the override path itself; `SaveInventoryWorkspaceChanges` remains the only commit point and is never called automatically by the override.
- **UI placement decision**: controls live inside the existing `SortOrderTab.tsx` Templates dropdown so this phase ships without touching any of the four global Templates modals. The global Templates shell does not expose the override because its v2 Apply paths did not reach the v1 `inventory.workspace` writer when Phase 6b shipped; Phase 7a shipped the v2 `inventory.workspace` apply path and Phase 7a.2 added a sibling `WeaponLevelOverridePanel` inside the existing `ApplyOverridesModal` for the v2 surface — the v1 dropdown stays in place byte-for-byte. Whether to relocate / consolidate the v1 dropdown is now a separate decision (Phase 12). Empty number fields mean "leave that class alone" (the corresponding pointer stays nil in the payload); the inline `weapon-override-error` element blocks Apply when the panel is enabled with both fields empty or either field out of range.
- **Backend impact**: additive option on the existing apply DTO; new pure helpers `validateWeaponLevelOverride` + `applyWeaponLevelOverride`; existing template-side writer unchanged. v2 `inventory.workspace` scope guard preserved.
- **Frontend impact**: one new panel (testid `weapon-override-panel`) inside the Templates dropdown; one new payload-builder helper; one new toast filter for override warnings. No other UI surface changed.
- **Tests**: targeted `go test . -run 'TestApplyTemplate_Override|TestValidateWeaponLevelOverride'` — all PASS (14 functions / 16 cases with subtests). Cases: validator accepts nil + disabled; validator rejects `Enabled = true` with both pointers nil; validator rejects negative `StandardLevel` and `SomberLevel`; override `nil` and `Enabled = false` leave upgrade untouched; `Enabled = true` with both fields nil is rejected before any workspace mutation; `StandardLevel` only touches standard weapons; `SomberLevel` only touches somber weapons; both together touch each class independently; `+26` standard / `+11` somber requests are clamped down with `IssueCodeWeaponLevelClamped` and land at `+25` / `+10`; values exactly at `MaxUpgrade` produce no warning; `MaxUpgrade == 0` (unarmed) emits `IssueCodeWeaponUnupgradeable` and skips; on preview error the workspace stays unchanged; clamp to zero from a zero request emits no warning. Full backend `go test . ./backend/... ./tests/...` 8/8 packages PASS; `go vet` clean; `tsc --noEmit` clean; targeted vitest for `SortOrderTab.test.tsx` 31 PASS (8 new for Phase 6b); full vitest 16 suites / 328 PASS; `make build` PASS.
- **Manual validation**: 2026-05-31 on `feature/templates-v1-weapon-level-override`. Applied a v1 template with mixed standard + somber + unupgradeable weapons under each control combination (override off / standard only / somber only / both / standard above `+25` / somber above `+10`); the per-class behaviour matched the warning lines surfaced by the report; the workspace mutated only through `editor.UpdateWeapon`; the user still committed by `Save changes`; the Phase 5 / 5D.2 / 6 v2 Apply paths and the Phase 9 URL import path were unaffected.
- **Risks**: respected — Phase 6b introduces no new lock surface, no new write path, no new endpoint. Mutation stays inside the existing workspace edit session, gated by the existing `slotMu[charIdx]` of `applyTemplateItemsToWorkspace`'s caller.
- **Out of scope**: equipped-weapon writers (Phase 7b); changing `Infusion` or `AoW` (the template's own values pass through unchanged); per-weapon (rather than per-class) override; direct save mutation through `core.PatchWeaponItemID` from the template apply. (Phase 7a.2 below lifted the override into the v2 `inventory.workspace` apply path; the v1 dropdown semantics remain unchanged.)
- **Requires separate user decision before continuing**: completed.

### Phase 7a — v2 `inventory.workspace` apply to active Inventory Edit Session — ✅ Shipped 2026-05-31

- **Goal**: ship the first real v2 apply path for `inventory.workspace`. Until this phase the v2 apply layer (`app_templates_v2_apply.go`) rejected `inventory.workspace` outright through the scope guard inherited from Phase 5. Phase 7a lifts that guard for `inventory.workspace` only and routes the v2 payload through the **active `InventoryEditSession` / `InventoryWorkspaceSnapshot`** so the writes land in the workspace that `SortOrderTab.tsx` already operates on — never directly into `slot.Data`. The user opens the Sort Order workspace first, applies the v2 template (from the library, the direct YAML import preview, the URL import preview, or `Apply with overrides…`), and commits the change exactly as today through `Save changes` (`SaveInventoryWorkspaceChanges`). No equipment / equipped-talismans / spells / appearance writer — those remain Phase 7b / 7c / 7d / 8. The Phase 6b weapon level override is intentionally **not** wired into the v2 path in this phase — it stays a v1 `SortOrderTab.tsx` Templates dropdown feature; the v2 inventory apply passes a hard-coded `nil` override into `applyTemplateItemsToWorkspace`. Wiring Phase 6b into v2 was tracked as Phase 7a.2 below and has since shipped — the `nil` pin has been replaced by `opts.WeaponLevelOverride`.
- **Approach**: the apply layer classifies the parsed v2 payload into three flags — `hasProfile`, `hasStats`, `hasInventory` — and gates the work on them. For `hasInventory == true` the apply acquires the caller-supplied session (full `lifecycleMu → sess.mu` order preserved), verifies `sess.CharacterIndex == charIdx`, runs capacity preflight on the **existing** workspace before any mutation, takes a `core.SnapshotSlot` of `slot.Data` and a value-type deep copy of `sess.Workspace`, then applies inventory + storage items through `applyTemplateItemsToWorkspace(&sess.Workspace, …, editor.ContainerInventory, nil)` and the storage equivalent. Profile/stats writes run **after** the inventory branch under the same `slotMu[charIdx]` window, reusing the Phase 5 path unchanged. A `rollbackBoth()` closure restores both the slot byte snapshot and the workspace value snapshot on every error exit, so a mixed profile+stats+inventory.workspace apply is atomic. For `hasInventory == false`, the existing Phase 5 edit-session conflict guard is preserved exactly.
- **Files (shipped scope)**: `app_inventory_session.go` (new `ActiveInventoryEditSession` struct + new `App.GetActiveInventoryEditSessionForCharacter(charIdx int)` endpoint that consults `editSessionsMu` + `editSessionByChar` and returns `{ active: bool, sessionID: string }`; never errors); `app_templates_v2_apply.go` (extended `ApplyTemplateV2Options` with `SessionID string json:"sessionID,omitempty"`, extended `ApplyTemplateV2Result` with `InventoryItemsApplied int`, `StorageItemsApplied int`, optional `Workspace *editor.InventoryWorkspaceSnapshot`, new session/scope branching in `ApplyBuildTemplateV2ToCharacterJSON`, dual snapshot rollback, capacity preflight before mutation, sentinel `"inventory.workspace"` appended to `Applied` when items land); `backend/templates/import.go` (new issue codes `IssueCodeInventorySessionRequired = "inventory_session_required"` and `IssueCodeInventorySessionInvalid = "inventory_session_invalid"`); `app_templates_v2_apply_inventory_test.go` (**new**, ~280 lines, 8 tests); `app_templates_v2_apply_test.go` (three existing "without session" tests updated to expect the new issue code); `frontend/src/components/templates/ImportTemplatePreviewModal.tsx` (module-level `V2_APPLY_SUPPORTED_SECTIONS = ['profile', 'stats', 'inventory.workspace']`); `frontend/src/components/templates/TemplateLibraryModal.tsx` (per-row `v2HasApplyableSections` now accepts `inventory.workspace`); `frontend/src/components/templates/TemplatesShellModal.tsx` (imports `GetActiveInventoryEditSessionForCharacter`, adds `INVENTORY_WORKSPACE_SECTION` + `NO_SESSION_MESSAGE` constants, `fetchActiveSessionID(charIndex)` helper, `canonicalJSONNeedsSession(canonical)` helper, and new session checks in `handleApplyV2FromLibrary` / `handleApplyV2FromImportedPreview` / `handleConfirmOverrides`); `frontend/src/components/templates/__tests__/TemplatesShellModal.test.tsx` (new `Phase 7a v2 inventory.workspace apply` block, +8 cases); `frontend/src/components/templates/__tests__/TemplateLibraryModal.test.tsx` (two cases rewritten — v2 `inventory.workspace`-only entries now enable Apply and render the overrides button); `frontend/wailsjs/go/main/App.{d.ts,js}` + `frontend/wailsjs/go/models.ts` regenerated by `make build`'s internal `wails generate module` step (no hand edits — new `ActiveInventoryEditSession` class + `GetActiveInventoryEditSessionForCharacter` method, `sessionID?: string` on `ApplyTemplateV2Options`, `inventoryItemsApplied`, `storageItemsApplied`, optional `workspace?: editor.InventoryWorkspaceSnapshot` on `ApplyTemplateV2Result`).
- **Backend impact**: no new schema section — Phase 7a reuses the existing `inventory.workspace` section from the v1 reader. No new lock surface — session acquire reuses the existing `acquireSession` path. No new writer — `applyTemplateItemsToWorkspace` is the exact same workspace mutation the v1 `inventory.workspace` Apply already uses. The mutation never touches `slot.Data` directly; `SaveInventoryWorkspaceChanges` remains the only commit point.
- **Frontend impact**: profile/stats-only v2 applies still proceed with `sessionID = ''` — the backend ignores it on the non-inventory branch. Library / direct YAML / URL Apply / `Apply with overrides…` all reuse the same canonical JSON path; for templates carrying `inventory.workspace` the shell looks up the active session through `GetActiveInventoryEditSessionForCharacter` and refuses with the "Open the Sort Order workspace before applying inventory templates." toast when none is active. v2 templates carrying unsupported sections outside `{profile, stats, inventory.workspace}` continue to keep both v2 buttons disabled with the supported-scope tooltip.
- **Tests**: targeted Phase 7a backend tests (`TestApplyBuildTemplateV2_Inventory_UnknownSessionRejected`, `..._WrongCharacterSessionRejected`, `..._HappyPath`, `..._EmptyItems_NoOp`, `TestApplyBuildTemplateV2_Mixed_ProfileStatsInventory_HappyPath`, `TestApplyTemplateV2Options_FieldSurface`, `TestApplyBuildTemplateV2_UnknownSectionStillRejected`, plus the three updated `..._InventoryWorkspaceWithoutSessionRejected` tests) all PASS. Phase 6b regression (`TestApplyTemplate_Override*`, `TestValidateWeaponLevelOverride*`) all PASS. Full backend `go test . ./backend/... ./tests/...` 8/8 packages PASS; `go vet` clean; `tsc --noEmit` clean; targeted vitest for `TemplatesShellModal.test.tsx` 66 PASS / `ImportTemplatePreviewModal.test.tsx` 43 PASS / `TemplateLibraryModal.test.tsx` 45 PASS; full vitest 16 suites / **336 PASS** (was 328 before Phase 7a, +8 cases); `make build` PASS.
- **Manual validation**: 2026-05-31 on `feature/templates-v2-inventory-workspace-apply` (HEAD `3e448f0`). Confirmed on a real save: applying a v2 template with `inventory.workspace` selected **without** an open Sort Order workspace surfaced the "Open the Sort Order workspace before applying inventory templates." toast and did **not** call the binding; opening the workspace and reapplying landed the items in the workspace grid; `Save changes` committed them to `slot.Data`; reloading the save showed the items with correct acquisition indices and no integrity warnings. A mixed profile + stats + inventory.workspace v2 template applied atomically — profile/stats fields updated and inventory items landed in a single user action. Both URL import and library apply reused the same canonical JSON path. Phase 5 / 5D.2 / 6 v2 profile/stats-only paths, Phase 6b weapon level override on the v1 SortOrderTab path, and the Phase 9 URL import path were all unaffected.
- **Risks**: respected — Phase 7a introduces no new lock surface beyond the existing session acquire. Mutation stays inside `sess.Workspace`. The dual snapshot rollback closes the window where a partial inventory write could leak after a later profile/stats validation error.
- **Out of scope (still future work)**: Phase 7b — equipment slot writer (see below). Phase 7c — equipped talismans writer. Phase 7d — spell loadout writer. Phase 8 — appearance via preset. Phase 10 — multi-character pack. (Phase 7a.2 — wiring Phase 6b weapon level override into the v2 `inventory.workspace` Apply path — has since shipped, see below.)
- **Requires separate user decision before continuing**: completed.

### Phase 7a.2 — weapon level override on the v2 `inventory.workspace` apply path — ✅ Shipped 2026-05-31

- **Goal**: thread the Phase 6b weapon level override into the Phase 7a v2 `inventory.workspace` apply path so users can override standard / somber weapon upgrade levels at apply time on every v2 surface (library `Apply with overrides…`, direct YAML import preview, URL import preview). Phase 7a deliberately hard-coded a `nil` override at the two `applyTemplateItemsToWorkspace` call sites and tracked the wiring as Phase 7a.2; this phase replaces both pins with `opts.WeaponLevelOverride`. The override remains a **runtime apply option**, never a template schema field — the canonical JSON the user previews never carries it, and the option travels exclusively through the `ApplyTemplateV2Options` struct. The v1 `SortOrderTab.tsx` Phase 6b dropdown is untouched; the v1 apply path (`ApplyBuildTemplateToWorkspaceJSON` with `ApplyTemplateOptions.WeaponLevelOverride`) continues to work byte-for-byte unchanged.
- **Approach**: reuse, do not duplicate. `ApplyTemplateV2Options` gains an additive `WeaponLevelOverride *WeaponLevelOverride json:"weaponLevelOverride,omitempty"`; the **same type** declared in `app_templates.go` for v1 is referenced — the bindings therefore expose a single `WeaponLevelOverride` class shared between v1 and v2 surfaces. `validateWeaponLevelOverride` from the v1 path runs **before** `acquireSession`, so a structurally broken override (`Enabled = true` with both pointers nil, or a negative level) bounces with `templates.IssueCodeStructureInvalid` and zero side effects. On the inventory branch, the two `applyTemplateItemsToWorkspace(&sess.Workspace, …, nil)` calls become `…, opts.WeaponLevelOverride)`; the helper itself, `applyWeaponLevelOverride`, and the Phase 7a dual snapshot rollback are unchanged. Warnings (`weapon_level_clamped`, `weapon_unupgradeable`) flow into `ApplyTemplateV2Result.Preview.Warnings` through the existing `invWarn` / `stoWarn` aggregation; they are warnings, never errors. A structurally valid override on a profile/stats-only template (`selection.inventory.workspace` absent) is silently ignored — the override loop simply has no items to operate on, mirroring how `SessionID` is silently ignored on the non-inventory branch.
- **Files (shipped scope)**: `app_templates_v2_apply.go` (extended `ApplyTemplateV2Options` with `WeaponLevelOverride *WeaponLevelOverride`, validation call before `acquireSession`, two `nil` → `opts.WeaponLevelOverride` substitutions in the inventory + storage `applyTemplateItemsToWorkspace` calls); `app_templates_v2_apply_inventory_test.go` (`TestApplyTemplateV2Options_FieldSurface` compile-time pin updated to the new three-field shape `{Mode, SessionID, WeaponLevelOverride}`); `app_templates_v2_apply_weapon_override_test.go` (**new**, ~340 lines, 14 cases); `frontend/src/components/templates/WeaponLevelOverridePanel.tsx` (**new**, ~145 lines, owns its own state, emits `{ enabled: true, standardLevel?, somberLevel? } | undefined` plus a `hasInvalid` flag, testids `apply-overrides-weapon-{panel,enabled,standard,somber,error}` distinct from the v1 `weapon-override-*` testids on `SortOrderTab.tsx`); `frontend/src/components/templates/ApplyOverridesPanel.tsx` (`ApplyOverridesModal` detects `selection['inventory.workspace']` and conditionally renders `WeaponLevelOverridePanel` under the profile/stats grid; `onConfirm` widened to `(mutatedJSON, weaponOverride?) ⇒ …`; `canApply` blocks while either panel is invalid; status pill switches to "Fix weapon level override to apply." when the weapon panel is the blocker; `ApplyOverridesPanel` itself — the JSON mutator — unchanged); `frontend/src/components/templates/TemplatesShellModal.tsx` (`handleConfirmOverrides` accepts the new second argument and forwards it as `weaponLevelOverride` inside `main.ApplyTemplateV2Options.createFrom({ mode, sessionID, weaponLevelOverride })`); `frontend/wailsjs/go/models.ts` regenerated by `make build`'s internal `wails generate module` step (only delta: `ApplyTemplateV2Options` carries `weaponLevelOverride?: WeaponLevelOverride` plus the matching constructor line; `WeaponLevelOverride` class already existed from the v1 path — no duplicate class generated). New tests: `frontend/src/components/templates/__tests__/WeaponLevelOverridePanel.test.tsx` (**new**, 9 cases) and additions to `__tests__/ApplyOverridesPanel.test.tsx` (+5 Phase 7a.2 cases) and `__tests__/TemplatesShellModal.test.tsx` (+6 Phase 7a.2 cases).
- **Backend impact**: **no new schema section** — Phase 7a.2 is a runtime option only, not a template field. **No new issue codes** — the warnings (`weapon_level_clamped`, `weapon_unupgradeable`) and the rejection code (`structure_invalid`) all predate this phase. **No new lock surface** — validation runs before `acquireSession`, the mutation reuses Phase 7a's session lock and dual snapshot rollback. The override mutation runs entirely inside `sess.Workspace` via `editor.UpdateWeapon`; no bytes are written to `slot.Data` from the override path itself; `SaveInventoryWorkspaceChanges` remains the only commit point.
- **Frontend impact**: `ApplyOverridesPanel` (the JSON mutator) is untouched on profile/stats-only templates — the weapon panel does not render at all in that case. For templates that select `inventory.workspace`, the modal grows a weapon sub-panel below the profile/stats grid; inventory-only templates may use the modal exclusively for weapon level override (the profile/stats grid renders empty headings, no fields). The weapon override **never** travels inside the canonical JSON — it travels exclusively through the new `weaponLevelOverride` field of `ApplyTemplateV2Options`. Phase 7a session gating still wins: an `inventory.workspace`-bearing template without an active Inventory Edit Session is rejected upstream regardless of the override state, and the binding is never called. The fast library Apply (`ApplyBuildTemplateV2FromLibraryToCharacter`) continues to send no override; only `Apply with overrides…` exposes the panel. The v1 `SortOrderTab.tsx` Phase 6b dropdown is untouched.
- **Tests**: targeted Phase 7a.2 backend tests cover nil override no-op; disabled override no-op; standard-only override touches only standard weapons; somber-only override touches only somber weapons; both levels touch each class independently; standard +99 clamps to +25 with `weapon_level_clamped`; somber +99 clamps to +10 with the same code; `MaxUpgrade == 0` (unupgradeable arm) emits `weapon_unupgradeable` and skips; `Enabled = true` with both pointers nil rejected with `IssueCodeStructureInvalid` before any mutation; negative `StandardLevel` rejected; mixed profile+stats+inventory.workspace + override happy path lands all three sections and the override; mixed apply with invalid override shape rolls back all state (no slot bytes mutated, no workspace mutation, `Dirty` flag preserved); profile/stats-only template with a valid override silently ignored. Phase 6b regression (`TestApplyTemplate_Override*`, `TestValidateWeaponLevelOverride*`) all still PASS. Full backend `go test . ./backend/... ./tests/...` PASS; `go vet` clean; `tsc --noEmit` clean; frontend Phase 7a.2 tests (`WeaponLevelOverridePanel.test.tsx` 9 PASS, `ApplyOverridesPanel.test.tsx` +5 PASS, `TemplatesShellModal.test.tsx` +6 PASS); full vitest 17 suites / **357 PASS** (was 336 before Phase 7a.2, +21 cases); `make build` PASS.
- **Manual validation**: 2026-05-31 on `feature/templates-v2-weapon-override` (HEAD `8fccd72`). Confirmed end-to-end on a real save: fast Apply without overrides keeps weapon upgrade levels as declared in the template; `Apply with overrides…` with Standard = 25 sets every standard weapon to +25 (or clamps with `weapon_level_clamped`) and leaves somber weapons at their template-side levels; Somber = 10 mirrors the symmetric case; both levels applied per-class independently; enabling the override with both fields empty disabled Apply; Standard = 26 / Somber = 11 / negative values disabled Apply with the inline error; closing the Sort Order workspace before applying surfaced the Phase 7a no-session toast and the override never reached the backend; a mixed profile + stats + inventory.workspace template with override applied atomically in a single user action; a profile/stats-only template with a valid override applied without rendering the weapon panel and without override warnings; URL import and library `Apply with overrides…` reused the same canonical JSON path and the same override field; the v1 `SortOrderTab.tsx` Phase 6b dropdown continued to work byte-for-byte unchanged.
- **Risks**: respected — reuse of the v1 validator and override helper keeps the contract identical between v1 and v2 surfaces. Validation runs before any mutation, so a broken override cannot leave a partially-applied template. The dual snapshot rollback from Phase 7a continues to cover error exits from `applyTemplateItemsToWorkspace`.
- **Out of scope (still future work)**: equipment writer for `ChrAsmEquipment` slots 0..9 / 12–15 (Phase 7b); equipped-talismans writer (Phase 7c); spell loadout writer (Phase 7d); appearance via preset (Phase 8); multi-character pack (Phase 10); optional future UX consolidation of the v1 `SortOrderTab.tsx` Phase 6b dropdown with the Templates shell now that the v2 path also carries the override (Phase 12, separately approved); per-weapon (rather than per-class) override; changing `Infusion` or `AoW` from the override panel.
- **Requires separate user decision before continuing**: completed.

### Phase 7b.0 — equipment writer foundation — ✅ Shipped 2026-05-31

- **Goal**: ship the **backend-only** writer foundation for `ChrAsmEquipment` slots 0..9 + 12–15 (weapons + ammo + armor) that the future Phase 7b.1 template apply will sit on top of. Phase 7b.0 introduces `(s *core.SaveSlot).WriteEquipment([]EquipmentWrite) error` as a method on `SaveSlot` directly — there is no `App` method, no Wails binding, no UI surface, and no Templates v2 schema change. The writer is callable from any future caller (Phase 7b.1, debug tools, future runtime overrides) without crossing the App boundary.
- **Approach**: the writer is a single Go-level public API in `backend/core/equipment_writer.go`. The `EquipmentSlotKind` enum exposes only the 14 supported slots (weapons LH/RH 1/2/3 → indices 0–5, ammo arrows1/2 + bolts1/2 → indices 6–9, armor head/chest/arms/legs → indices 12–15). Talismans 17–21, EquippedGreatRune (slot 10), unknown slots 11/16, EquippedSpells, quick / pouch items — all intentionally unexposed. Each `EquipmentWrite` carries the `Slot` enum value and a `Handle uint32` resolved from inventory. The writer maintains a strict validate-then-mutate ordering: every write in the batch is validated against the slot's class (weapon / ammo / armor), the handle's type prefix (`0x80` / `0xB0` / `0x90`), the explicit-clear sentinel (`Handle == 0` → write `0xFFFFFFFF`), the invalid-sentinel guard (`Handle == 0xFFFFFFFF` rejected), the duplicate-slot guard, and the GaMap-presence requirement, before any byte of `slot.Data` is touched. Ash of War handles (`0xC0`) are rejected in weapon slots even though the read-side encoding rule (`itemID | 0x80000000`) would technically accept them — equipping AoW as a weapon is deferred. Encoding rules mirror the read-side `IsHandleEquipped` convention: weapon and armor slots store `itemID | 0x80000000`; ammo slots store the goods `itemID` directly because goods item IDs already carry the `0x40` prefix. Hash recompute is targeted to the affected entries only: writes touching slots 0–9 recompute hash 7 (`weaponSlotIndices`), writes touching slots 12–15 recompute hash 8 (the armor subset of `armorSlotIndices`); unrelated hash entries (level, stats, souls, quick items, spells, talisman portion of hash 8) stay byte-identical.
- **Files (shipped scope)**: `backend/core/equipment_writer.go` (**new**, ~245 lines — `EquipmentSlotKind`, `EquipmentWrite`, `WriteEquipment`, `equipmentSlotTable`, internal class-gate); `backend/core/equipment_writer_test.go` (**new**, ~480 lines, 24 cases). No changes to `backend/templates/`, `app_templates*.go`, `frontend/`, `frontend/wailsjs/`, `App.tsx`, or `SortOrderTab.tsx`.
- **Backend impact**: first prod call path through `RecalculateSlotHash`-equivalent helpers in `backend/core/hash.go` (`readEquipSection`, `extractSlots`, `equipmentHash`). The single existing equipment writer — `EquippedGreatRune` via `SyncPlayerToData` at `structures.go:850–852` — is preserved unchanged. Concurrency contract documented in the writer's docstring: callers sharing a `SaveSlot` across goroutines must hold the slot-level lock for the entire `WriteEquipment` call (the writer itself does not lock).
- **Frontend impact**: none.
- **Tests**: 24 unit tests against a synthetic `SaveSlot` (no real save fixture required — `tests/data/{pc,ps4}` are scratch dirs and the real fixtures live in `tmp/save/` which is excluded from the Phase 7b.0 scope per the user's tmp-exclusion rule). Cases cover weapon / armor / ammo encoding correctness, rejection of AoW / talisman / goods handles in the wrong slot, rejection of weapon handles in armor slots, rejection of handles missing from `GaMap`, clear-slot semantics (`Handle == 0`), rejection of the `0xFFFFFFFF` sentinel, rejection of unknown enum values, rejection of duplicate slot keys within a batch, atomicity rollback when the second write of a batch is invalid, hash 7 changes after weapon and ammo writes, hash 8 changes after armor writes, hash 7 stable on armor-only writes, hash 8 stable on weapon-only writes, mixed weapon + armor batches touch both hashes, idempotent writes leave hashes stable, nil-receiver guard, empty batch no-op, unparseable `EquipItemsIDOffset` rejected, and an end-to-end weapon swap + clear round-trip. Full backend `go test . ./backend/... ./tests/...` PASS (9 packages including the new tests); `go vet` clean; `make build` PASS with the Wails bundle rebuilt and `frontend/wailsjs/**` byte-identical.
- **Manual validation**: deliberately deferred to Phase 7b.1 where the template apply path exercises the writer end-to-end on real save fixtures. The synthetic `SaveSlot` test surface exhaustively covers the writer's contract.
- **Risks**: respected — Phase 7b.0 introduces the writer in isolation from the template apply path, the App layer, and the bindings, so any defect can be diagnosed against the writer's unit tests alone. The targeted hash recompute discipline avoids touching unrelated hash entries.
- **Out of scope**: equipped talismans (Phase 7c), spell loadout (Phase 7d), EquippedGreatRune through the template path (slot 10), unknown slots 11/16, quick / pouch items, App method or Wails binding wrappers, template schema or apply integration (all Phase 7b.1).
- **Requires separate user decision before continuing**: completed.

### Phase 7b.1 — v2 `sections.equipment` apply through the Phase 7b.0 writer — ✅ Shipped 2026-06-01

- **Goal**: wire the Phase 7b.0 `SaveSlot.WriteEquipment` foundation into the Templates v2 apply path end-to-end. Ship `sections.equipment` schema, the export builder that captures the currently-equipped 14-slot loadout, the preview row + slot list, the apply pipeline with rollback-protected `WriteEquipment` dispatch, and the frontend surface in the existing Templates shell. Phase 7b.1 covers the exact same 14 slots as the Phase 7b.0 writer (weapons LH/RH 1/2/3, arrows1/2, bolts1/2, armor head/chest/arms/legs); talismans 17–21, EquippedGreatRune (slot 10), unknown slots 11/16, the 14-slot EquippedSpells loadout, and quick / pouch items remain future work. Equipment-only templates apply through the canonical `ApplyBuildTemplateV2ToCharacterJSON` exit shared by library Apply, direct YAML Apply, file Apply, URL preview Apply, and `Apply with overrides…`.
- **Approach**: extend the existing v2 pipeline with a fourth `hasEquipment` flag rather than introduce a parallel apply path. The schema adds `EquipmentSection` (14 optional slot pointers) and `EquipmentItemRef { BaseItemID; Name; Upgrade *int; InfusionName; AoWItemID *uint32 }`; `BaseItemID == 0` is the explicit-clear sentinel; omitted slot pointers mean "no-op"; omitted `Upgrade` means "match any upgrade". The export builder extends `ExportV2Options` with `Equipment *EquipmentSection` and follows the Phase 5 normalisation pattern: boolean shortcut copies the section verbatim, per-field selection drops slot keys with no source value. An app-layer scanner `buildEquipmentSectionFromSlot(slot, inventoryItems)` reads `ChrAsmEquipment` at the 14 supported indices, decodes the encoded form (weapon / armor: strip `0x80000000`; ammo: raw goods itemID), and matches against `editor.EditableItem.ItemID` with a `db.GetItemDataFuzzy` / raw-decoded-ID fallback so the export always records what each slot held. The apply path adds `resolveEquipmentWrites(slot, sel, sec)` which calls `editor.BuildSnapshot` once for a consistent view, then delegates to a pure-logic `resolveEquipmentWritesFromItems(items, sel, sec)`. For each selected slot the resolver matches by `BaseItemID` plus any supplied disambiguators (Upgrade / InfusionName / AoWItemID), searching `InventoryItems` only (storage is intentionally **not** searched — items only in storage are reported as missing per the Phase 7b.1 strict policy). Missing items emit `equipment_item_not_in_inventory` warnings and the slot is skipped (no auto-add); ambiguous matches (>1 after disambiguators) resolve to the first match with an `equipment_item_ambiguous` warning. The resolver builds a `[]core.EquipmentWrite` batch which the apply path dispatches to `SaveSlot.WriteEquipment` AFTER the profile/stats `SyncPlayerToData`, so the Phase 7a `rollbackBoth()` closure (slot bytes + workspace snapshot) naturally covers any partial equipment write. The combo `sections.equipment + sections.inventory.workspace` is **hard-rejected** in both the preview layer and the apply layer (defence in depth) with the dedicated `equipment_inventory_combo_unsupported` code: the writer needs a fresh `slot.GaMap`, and inventory.workspace items only land in the workspace until the user clicks Save changes. Equipment-only templates do NOT require an Inventory Edit Session; the existing `TemplatesShellModal` `needsSession = selectedSections.includes('inventory.workspace')` gate already excludes them, so the shell file is unchanged.
- **Files (shipped scope)**: `backend/templates/schema.go` (new `EquipmentSection`, `EquipmentItemRef`, `TemplateSelection.Equipment`, `TemplateSections.Equipment`, `EquipmentSlotOrder` exported canonical list, `EquipmentSlotRef` / `SetEquipmentSlotRef` helpers, `equipmentSelectionFields` allowlist, `validateEquipmentSection`, `validateEquipmentItemRef`, `validateEquipmentSelection`, `MaxEquipmentItemUpgrade = 25`); `backend/templates/import.go` (new issue codes `IssueCodeEquipmentInventoryComboUnsupported`, `IssueCodeEquipmentItemNotInInventory`, `IssueCodeEquipmentItemAmbiguous`, `IssueCodeEquipmentSlotInvalid`; new `ImportPreviewSummary.EquipmentSlotsPresent`; `selectedSectionsForTemplate` appends `"equipment"`; preview-time combo guard injecting the dedicated code); `backend/templates/schema_equipment_test.go` (**new**, ~270 lines, 17 cases); `backend/templates/export_v2.go` (new `ExportV2Options.Equipment`, `buildEquipmentSection` helper with deep-clone, equipment branch in `BuildV2Template`); `backend/templates/export_v2_equipment_test.go` (**new**, ~165 lines, 7 cases); `app_templates_v2_equipment.go` (**new**, ~260 lines — `equipmentSlotChrAsmIndex`, `equipmentSlotIsAmmo`, `equipmentSlotKindForKey`, `buildEquipmentSectionFromSlot`, `decodeEquipmentSlotToRef`, `itemToEquipmentRef`, `resolveEquipmentWrites`, `resolveEquipmentWritesFromItems`, `lookupEquipmentHandle`); `app_templates_v2_equipment_test.go` (**new**, ~165 lines, 8 cases); `app_templates_v2_apply.go` (extended `ApplyTemplateV2Result.EquipmentSlotsApplied`, new `hasEquipment` branch, combo defence rejection, resolver call before the inventory.workspace branch, `WriteEquipment` dispatch after the profile/stats SyncPlayerToData with inlined `equipment_slot_invalid` error injection on writer failure, `profileOrStatsApplied` skip for the `"equipment"` sentinel); `app_templates_v2_apply_equipment_test.go` (**new**, ~265 lines, 11 cases); `frontend/src/components/templates/ImportTemplatePreviewModal.tsx` (`V2_APPLY_SUPPORTED_SECTIONS` += `'equipment'`, new `equipmentSlotsPresent` from summary, new `import-preview-equipment-slots` testid + render, tightened unsupported-section copy); `frontend/src/components/templates/TemplateLibraryModal.tsx` (`v2HasApplyableSections` accepts `'equipment'`); `frontend/src/components/templates/__tests__/ImportTemplatePreviewModal.test.tsx` (+4 Phase 7b.1 cases; two pre-existing tests using `'equipment'` as the "unsupported" example switched to `'spells'`); `frontend/src/components/templates/__tests__/TemplateLibraryModal.test.tsx` (+2 Phase 7b.1 cases); `frontend/src/components/templates/__tests__/TemplatesShellModal.test.tsx` (+3 Phase 7b.1 cases); `frontend/wailsjs/go/models.ts` regenerated by `make build` (only delta: `ApplyTemplateV2Result.equipmentSlotsApplied: number` and `ImportPreviewSummary.equipmentSlotsPresent?: string[]` plus matching constructor lines).
- **Backend impact**: `ApplyTemplateV2Options` is unchanged — equipment is schema-driven, not a runtime option. `ApplyTemplateV2Result` grows the `EquipmentSlotsApplied int` counter. The dual-snapshot rollback contract from Phase 7a is unchanged and naturally covers `WriteEquipment` failures because the writer mutates `slot.Data` after the snapshot was taken. No new App method; no Wails binding signature change.
- **Frontend impact**: equipment-only templates can be applied through library fast Apply, direct YAML Apply, URL preview Apply, and `Apply with overrides…`. The Phase 6 `ApplyOverridesPanel` and Phase 7a.2 `WeaponLevelOverridePanel` are **not** extended — Phase 7b.1 ships without equipment-specific override controls. The `TemplatesShellModal` source is unchanged: existing session gating naturally excludes equipment-only templates, and the canonical JSON path serves all entry points identically. The Phase 6 / Phase 6b combo with overrides keeps the override editing scope at profile / stats and the embedded weapon level panel — equipment refs themselves are never edited at apply time in this phase.
- **Tests**: targeted Phase 7b.1 backend tests cover schema round-trip (JSON / YAML), per-field allowlist accepts the 14 canonical keys and rejects unknown / `equippedGreatRune`, explicit-clear acceptance, upgrade nil / negative / above-cap, `aowItemID=0` rejection, combo guard in the preview, EquipmentSlot helpers, the export builder's boolean-shortcut + per-field normalisation + deep clone + explicit clear + missing-source + JSON shape paths, the scanner's empty / weapon / armor / ammo encoding match + talisman / GreatRune / unknown slots NOT exported + unreadable offset + unknown-item fallback + multi-slot, and the apply path's match-by-base-id + missing-item warning + ambiguous first-wins + upgrade / infusion disambiguators + explicit clear + per-field selection + full apply combo rejection with zero side effects + sessionID silently ignored on equipment-only + missing-item zero writes + unsupported selection key rejected. Frontend Phase 7b.1 tests cover preview equipment-row render + slot list + combo error display + Apply enablement, library Apply for equipment-only entries (no session required), shell library Apply transparently forwarding the active session ID when present and tolerating the absence, and the backend rejection toast surfacing through the standard shell error path. Full backend `go test . ./backend/... ./tests/...` PASS (9 packages); `go vet` clean; `tsc --noEmit` clean; full vitest 17 suites / **366 PASS** (was 357 before Phase 7b.1, +9 cases on top of the two repurposed tests); `make build` PASS.
- **Manual validation**: 2026-06-01 on `feature/templates-v2-equipment-apply` (HEAD `42ff906`). Confirmed end-to-end on a real save: an equipment-only template exported from a character that has weapons equipped on RH1 / LH1 and armor on all four armor slots round-tripped through the preview modal showing `weaponRightHand1`, `weaponLeftHand1`, `armorHead/Chest/Arms/Legs` in the new equipment-slots row; fast library Apply for the same template re-equipped the listed items without an Inventory Edit Session; saving and reloading the save through both the editor and the in-game flow showed the equipment intact; a template referencing an item missing from inventory (still in the DB, just removed from the character) emitted `equipment_item_not_in_inventory` warnings and skipped the slot without touching other applied sections; a template with `BaseItemID: 0` for `weaponLeftHand1` cleared the slot to empty (`0xFFFFFFFF`) while leaving the other slots untouched; a template carrying both `sections.equipment` and `sections.inventory.workspace` surfaced the `equipment_inventory_combo_unsupported` error at preview time, the Apply button stayed disabled, and the shell did not call the binding; a template carrying two identical weapons in inventory (same baseItemID, same upgrade level) resolved to the first match and emitted the `equipment_item_ambiguous` warning. Regression sweep verified: v2 profile/stats Apply (Phase 5 / 5D.2) unchanged; v2 inventory.workspace Apply still requires an active session and continues to use `Save changes` as the commit point (Phase 7a); v2 weapon level override on the inventory.workspace path still renders inside the Apply with overrides modal and applies correctly (Phase 7a.2); URL import still flows through the same preview / Apply path including for equipment-only templates (Phase 9); v1 `SortOrderTab.tsx` Phase 6b dropdown unchanged byte-for-byte.
- **Risks**: respected — equipment writes happen after profile/stats slot bytes have flushed, but inside the same `slotMu[charIdx]` window and inside the existing snapshot scope; the dual-snapshot rollback covers a `WriteEquipment` failure even though equipment mutates `slot.Data` directly (the snapshot includes hash 7 / 8 bytes the writer recomputes). The combo rejection avoids the GaMap-freshness problem entirely by refusing the only configuration where it could surface. The resolver's strict-existing-only policy avoids any auto-add risk.
- **Out of scope (still future work)**: equipped-talismans writer + apply (Phase 7c); spell loadout writer + apply (Phase 7d); lifting the `equipment + inventory.workspace` combo restriction (would need a workspace-backed equipment model or an explicit auto-commit gesture from the user); appearance via preset (Phase 8); multi-character pack (Phase 10); EquippedGreatRune through the template path (slot 10, still only written by `SyncPlayerToData`); slots 11/16 (unknown semantics); quick / pouch items; equipment-specific override panel in the Apply with overrides modal.
- **Requires separate user decision before continuing**: completed.

### Phase 7c — v2 talisman apply through `sections.equipment.talisman1..5` (shipped 2026-06-01)

- **Status**: ✅ Shipped 2026-06-01 on `feature/templates-v2-talisman-apply`, ff-merged to `main`.
- **Schema decision (amendment to the original Phase 7c design above)**: the original §17 / §13.6 wording reserved a separate `sections.equippedTalismans` schema section because — at the time it was written — `sections.equipment` did not exist. Phase 7b.1 shipped `sections.equipment` first; Phase 7c re-evaluated the cut and intentionally **extends `sections.equipment` with five talisman slots `talisman1..5`** instead of introducing a parallel `sections.equippedTalismans`. Talismans live in the same `ChrAsmEquipment` struct as weapons/ammo/armor (indices 17–21 vs 0–9 / 12–15) and share hash 8 with armor, so the natural cut is "one schema section = one binary section." This deviation from §17's original wording is now load-bearing — references to `sections.equippedTalismans` elsewhere in the spec should be read as historical context, not as the shipped schema.
- **Goal**: ship end-to-end v2 talisman apply through the existing Phase 7b.1 `sections.equipment` pipeline, with explicit clear semantics, strict "must be in inventory" matching, pouch-count gating, and Talisman5 protection (vanilla cap = 4 active slots).
- **Files (shipped scope)**:
  - `backend/core/equipment_writer.go` (+`EquipSlotTalisman1..5` enum values, new `slotClassTalisman` class accepting only the `ItemTypeAccessory` 0xA0 handle prefix, five `equipmentSlotTable` entries mapping to indices 17–21, hash 8 recompute path widened to also cover indices 17–21, encoding path: `GaMap[handle]` directly with no `| 0x80000000` mask, class-gate reject for 0xA0 handles in the previously-supported weapon/armor/ammo classes is preserved unchanged) and `backend/core/equipment_writer_test.go` (+10 cases: talisman encoding, four cross-class rejects, hash 8 recompute, hash 7 stable on talisman-only, idempotent write, mixed armor+talisman batch, atomic rollback).
  - `backend/templates/schema.go` (+`EquipmentSection.Talisman1..5 *EquipmentItemRef`, +five talisman keys in `equipmentSelectionFields` and `EquipmentSlotOrder`, +five case arms in `equipmentSlotRef`/`SetEquipmentSlotRef`, updated `EquipmentSection` doc comment recording the deviation from the original §17 design and the Talisman5 / pouch gating contract) and `backend/templates/import.go` (+`IssueCodeTalismanSlotPouchInsufficient = "talisman_slot_pouch_insufficient"`).
  - `backend/templates/schema_equipment_test.go` (+8 talisman cases: JSON / YAML round-trip, slot-order tail, selection allowlist for 5 keys, explicit clear, slot-ref helpers, preview equipment-slots row listing, `equipment + inventory.workspace` combo guard with a talisman ref, stable issue-code string; one existing test updated from `"talisman1"` (now valid) to `"equippedSpell1"`, `EquipmentSlotOrder` expected length updated from 14 to 19).
  - `backend/templates/export_v2_equipment_test.go` (+3 talisman export cases: verbatim copy of populated slots + explicit clear, per-field selection drops unsupplied slots, JSON shape includes `"talisman1"`). `export_v2.go::buildEquipmentSection` already iterates `EquipmentSlotOrder` and dispatches through `EquipmentSlotRef` / `SetEquipmentSlotRef`, so no builder change is needed.
  - `app_templates_v2_equipment.go` (+five talisman keys in `equipmentSlotChrAsmIndex` mapping to indices 17–21, +five case arms in `equipmentSlotKindForKey` mapping to `core.EquipSlotTalisman1..5`, new helper `equipmentSlotIsTalisman`, `decodeEquipmentSlotToRef` candidate-selection branch: ammo **or** talisman → raw stored value, weapons/armor → strip `0x80000000`; new constant `MaxActiveTalismanSlots = 4`; new helper `talismanSlotOrdinal`; extended `resolveEquipmentWrites` / `resolveEquipmentWritesFromItems` signatures take `activeTalismanSlots uint8` and gate non-empty talisman refs against `MaxActiveTalismanSlots` and `activeTalismanSlots`) and `app_templates_v2_equipment_test.go` (+4 scanner cases: talisman1 match with `IsTalisman` editable item; five talismans populated; unknown talisman emits raw decoded ID; `equipmentSlotIsTalisman` truth table; the negative `TalismansAndGreatRuneNotExported` test renamed to `GreatRuneAndUnknownSlotsNotExported` because talismans now DO export).
  - `app_templates_v2_apply.go` (+`computeActiveTalismanSlots(slot, tpl) uint8` helper returning `1 + effective profile.talismanSlots`, where "effective" is the template's `profile.talismanSlots` when present and selected, else the slot's current persisted `Player.TalismanSlots`, both clamped to `MaxProfileTalismanSlots = 3`; the call site to `resolveEquipmentWrites` now passes the computed active capacity through) and `app_templates_v2_apply_equipment_test.go` (+13 cases: 6 resolver unit tests for talisman matching / pouch / Talisman5 / explicit-clear / clear beyond pouch / missing-item-in-pouch; 4 `computeActiveTalismanSlots` tests; 3 resolver + writer integration tests that exercise the full `resolveEquipmentWritesFromItems` → `WriteEquipment` path without standing up `BuildSnapshot`. The 7 existing resolver call sites in the same file gain the new `activeTalismanSlots = 4` (max, no gating) parameter; one test that used `"talisman1"` as an example of an unknown selection key now uses `"equippedSpell1"`).
  - `frontend/src/components/templates/__tests__/ImportTemplatePreviewModal.test.tsx` (+2 cases: talisman keys render in the existing `import-preview-equipment-slots` row, Apply remains enabled for a talisman-only equipment template).
  - **No source-code change** in `ImportTemplatePreviewModal.tsx`, `TemplateLibraryModal.tsx`, `TemplatesShellModal.tsx`, `ApplyOverridesPanel.tsx`, `WeaponLevelOverridePanel.tsx`, `SortOrderTab.tsx`, or `App.tsx`. `V2_APPLY_SUPPORTED_SECTIONS` already lists `equipment`, the existing `import-preview-equipment-slots` row enumerates whatever slot keys the summary lists (talismans included), and the Library / direct YAML / URL apply paths reuse the canonical JSON pipeline.
  - **Bindings**: `make build` triggered the standard `wails generate module` step; `frontend/wailsjs/go/models.ts` shows no diff because the talisman extensions are JSON-payload fields on `EquipmentSection`, exchanged with the frontend as opaque template JSON rather than a typed Wails model. `App.d.ts` / `App.js` are untouched (no method signature change).
- **Backend impact**: extends the Phase 7b.0 equipment writer with a fourth slot class (`slotClassTalisman`) and the five talisman slot kinds; the writer remains the only public byte-writing entry point for ChrAsmEquipment. Hash 7 is untouched for talisman-only writes; hash 8 recompute already iterates `armorSlotIndices = [12..15, 17..21]` in `backend/core/hash.go`, so the only required change is widening the touched-slot range in `WriteEquipment`. `core.SaveSlot` field surface (data, `EquipItemsIDOffset`, `GaMap`, `Player.TalismanSlots`) is unchanged.
- **Apply contract**:
  - **Selection**: `selection.equipment` allowlist now accepts `talisman1..5` in addition to the 14 weapon/ammo/armor keys; everything else still rejects.
  - **Schema**: each `EquipmentSection.TalismanN` is an optional pointer to the existing `EquipmentItemRef`. Talismans reuse the `EquipmentItemRef` shape, but the apply layer ignores `Upgrade`, `InfusionName`, and `AoWItemID` for talisman slots (talisman items have no upgrade level, no infusion, and no Ash of War); producers normally omit those fields.
  - **Explicit clear**: `baseItemID: 0` writes `0xFFFFFFFF` (empty slot marker) into the slot, regardless of slot index — including Talisman5.
  - **Resolution**: same matching algorithm as Phase 7b.1 — search `slot.Inventory.CommonItems` only (storage NOT searched), match by `BaseItemID`, first-match wins on ambiguity with a warning. Missing-from-inventory talisman emits `equipment_item_not_in_inventory` warning + skip. **No auto-add.**
  - **Pouch gating (new in Phase 7c)**: active talisman slot capacity = `1 + effective profile.talismanSlots`, where "effective" is the template's `profile.talismanSlots` when present and selected (template wins because profile apply runs before equipment apply), otherwise the slot's current persisted `Player.TalismanSlots`; both values are clamped to `MaxProfileTalismanSlots = 3`, hence the resolver caps the active capacity at 4. A non-empty talisman ref targeting an ordinal beyond the active capacity emits `talisman_slot_pouch_insufficient` warning + skip. Explicit-clear refs (`baseItemID = 0`) bypass the gate entirely.
  - **Talisman5 (slot index 21) policy**: always warn + skip when populated with `baseItemID > 0`. Vanilla Elden Ring caps the Pouch at 4 active slots; slot 5 exists in the binary but is unreachable through in-game gameplay. Clearing Talisman5 with `baseItemID = 0` is always allowed and writes `0xFFFFFFFF`.
  - **Combo guard**: `sections.equipment + sections.inventory.workspace` hard reject is unchanged. The same rule applies to talisman-only equipment templates because talismans are inside `sections.equipment`. The reason is `slot.GaMap` freshness — the workspace only commits to the slot when the user clicks Save changes.
  - **Atomicity**: the Phase 7a `rollbackBoth()` closure already covers the slot snapshot taken at the top of the slot lock. A mid-batch validation failure during a mixed armor + talisman write rolls back both slot bytes and hash bytes with zero side effects (covered by the new `TestWriteEquipment_AtomicRollbackOnInvalidTalisman` and `TestWriteEquipment_MixedArmorTalismanBatchHash8`).
  - **Fast-apply path**: equipment-only templates (including talisman-only) continue to skip the Inventory Edit Session requirement; the existing `TemplatesShellModal` `needsSession` gate already excludes them automatically.
- **Frontend impact**: zero source-code change. Talismans appear in the existing equipment preview row when present in the template summary; Library / direct YAML / URL apply paths reuse the canonical JSON pipeline.
- **Tests** (full Phase 7c additions on top of the previously-shipped backend / templates / app / frontend suites):
  - `backend/core/equipment_writer_test.go` +10 talisman cases.
  - `backend/templates/schema_equipment_test.go` +8 talisman cases, 1 existing test rewired (`"talisman1"` → `"equippedSpell1"`), expected `EquipmentSlotOrder` length updated 14 → 19.
  - `backend/templates/export_v2_equipment_test.go` +3 talisman cases.
  - `app_templates_v2_equipment_test.go` +4 cases (one rename from `TalismansAndGreatRuneNotExported`).
  - `app_templates_v2_apply_equipment_test.go` +13 cases (6 resolver, 4 `computeActiveTalismanSlots`, 3 resolver + writer integration), 7 existing resolver call sites extended with the `activeTalismanSlots = 4` parameter, 1 unknown-key test rewired (`"talisman1"` → `"equippedSpell1"`).
  - `frontend/src/components/templates/__tests__/ImportTemplatePreviewModal.test.tsx` +2 cases.
- **Manual validation**: 2026-06-01 — equipment export with active talismans + Talisman5 clear; talisman keys appearing in the v2 preview row + Apply enabled; equipment-only Apply happy path with game-load reload + Save & Quit retaining state; pouch gating with `TalismanSlots = 0` (slot 1 OK, slots 2/3/4 `talisman_slot_pouch_insufficient` warn + skip); mixed `profile.talismanSlots = 3 + equipment.talisman4` lifting the cap inside a single apply; `talisman5 baseItemID > 0` warn + skip; `talisman5 baseItemID = 0` clear OK; missing talisman item warn + skip with other slots committing; `equipment + inventory.workspace` hard reject unchanged; regression sweep across Phase 5 / 5D.2 / 6 / 6b / 7a / 7a.2 / 7b.1 / 9 / v1 SortOrderTab paths — user-confirmed `manual OK`.
- **Risks**: low — the new write path reuses the Phase 7b.0 atomic-batch infrastructure with a single additional class; hash 8's slot list already covered indices 17–21 before Phase 7c. The Talisman5 / pouch policies fail closed (warn + skip), so a misconfigured template can never write an unreachable slot.
- **Out of scope of this phase (still future work)**: spell loadout (Phase 7d); lifting the `equipment + inventory.workspace` combo restriction (optional Phase 7b.2); EquippedGreatRune through the template path; unknown slots 11 / 16; quick items / pouch slots; auto-add of missing talisman items; storage resolution path; talisman-specific override panel; appearance preset (Phase 8); multi-character pack (Phase 10).

### Phase 7d — spell loadout writer (new write path)

- **Goal**: implement a new public write path for the 14 `EquippedSpells` slots. Apply `sections.spells` through it. Today, this field is referenced only by hash recompute (`backend/core/hash.go:150`, `section_hash.go:24`) — no editor write surface.
- **Files (planned scope)**: new writer in `backend/core/` for `EquippedSpells`; apply layer extension; tests; frontend preview row.
- **Backend impact**: new public write API for spell loadout.
- **Tests**: hex round-trip; PC + PS4 parity; preview rejects unknown spell IDs.
- **Manual validation**: apply spells; round-trip both platforms; in-game verification on at least one platform.
- **Risks**: medium — first write into the spell loadout area; per-platform offsets must be re-confirmed.
- **Out of scope**: appearance; URL import; multi-character pack.
- **Requires separate user decision before continuing**: yes.

### Phase 8 — appearance via preset (reuses existing helpers)

- **Goal**: apply `sections.appearance.preset` (and, by extension, gender + voiceType bound to the preset) through the existing `app_appearance.go::ApplyPresetToCharacter` helper. No raw FaceData blob is ever written from a template.
- **Files (planned scope)**: apply layer extension to route the appearance section through `app_appearance.go`; tests.
- **Backend impact**: reuses existing helpers; no new write path.
- **Tests**: apply preset; gender / voice consistency; rollback on failure; preview shows preset name.
- **Manual validation**: apply preset; confirm in-game appearance (Steam Deck verification path).
- **Risks**: appearance is visually disruptive — preview must clearly show the destination preset name and warn the user before apply.
- **Out of scope**: raw FaceData, multi-character pack, URL import.
- **Requires separate user decision before continuing**: yes.

### Phase 9 — URL import (shipped 2026-05-31, full guards)

- **Status**: ✅ Shipped 2026-05-31 on `feature/templates-v2-url-import`, ff-merged to `main`.
- **Goal**: implement URL import per §12 with all guards (https-only, IP filter, redirect re-check, body size, timeouts, strict TLS, struct-typed YAML, no includes, no executable types, preview before library, separate confirm before apply). All guards landed.
- **Approach**: extend the existing file-import preview path with an `https://` source. The shell asks the backend to fetch the URL under §12.3 guards, the backend hands the bytes to the same `previewYAMLPayload` the file path uses, and the resulting `LoadedTemplatePreview { Report, JSON, Path }` flows through the same `ImportTemplatePreviewModal`. All three downstream actions (Save to Library, Apply to character via Phase 5D.2, Apply with overrides via Phase 6) ship unchanged on the URL surface.
- **Files (shipped scope)**: `backend/templates/url_import.go` (**new**, exports `FetchYAMLFromURL`, the `URLImport*` constants, `FetchError`, and all `IssueCodeURL*` codes); `backend/templates/url_import_test.go` (**new**, 28 fetcher cases + 21 `TestIsAllowedAddr` subtests); `app_templates_url.go` (**new**, Wails handler `PreviewBuildTemplateImportYAMLFromURL`); `app_templates_url_test.go` (**new**, 6 integration cases); `frontend/src/components/templates/ImportTemplateFromURLModal.tsx` (**new**); `frontend/src/components/templates/TemplatesShellModal.tsx` (`Import from URL…` button + `onURLImportPreview` callback wiring); `frontend/wailsjs/go/main/App.{d.ts,js}` regenerated by `make build`'s internal `wails generate module` step. `models.ts` unchanged.
- **Backend impact**: new backend fetch through the standard library `net/http` client with custom `Transport.DialContext` (pre-connect IP filter for literal IPs **and** every DNS-resolved address) and custom `Client.CheckRedirect` (re-checks scheme + re-resolves + re-filters on every hop, capped at 3 redirects). Strict `tls.Config { MinVersion: tls.VersionTLS12 }`, system root CAs only, no `InsecureSkipVerify`, no custom CA. Response body capped through `io.LimitReader` at `URLImportMaxBodyBytes = 1 << 20` (1 MiB). Content-Type parsed by `mime.ParseMediaType` and checked against the allowlist. No auth, no cookies, no custom headers. `http.Transport.DisableKeepAlives: true`.
- **Frontend impact**: new small modal with a single `https://` URL input, light client-side validation (regex `^https?://`), Enter-to-submit, in-flight "Fetching…" state, inline error rendering that preserves the input on rejection, and a Cancel that does not call the binding. The shell shows a `Import from URL…` header button next to the existing file-import button (testid `templates-shell-import-url`). The URL preview reuses `ImportTemplatePreviewModal` unchanged — there is no parallel "URL preview" surface.
- **Tests**: each guard is covered by an explicit test. Backend (`backend/templates/url_import_test.go`): `https`-only enforcement, loopback / RFC1918 / link-local / ULA / multicast / broadcast / unspecified rejection, cloud-metadata literal rejection (`169.254.169.254`, `fd00:ec2::254`), DNS-resolved IP filter, redirect re-check on each hop, redirect cap, body size cap, total timeout, TLS error mapping, Content-Type allowlist, bad-status mapping, no auth / no cookies / no custom headers. Handler (`app_templates_url_test.go`): empty URL, whitespace-only URL, `http://` scheme, `data:` scheme, literal loopback, literal cloud-metadata IP. Frontend (`__tests__/ImportTemplateFromURLModal.test.tsx`, 10 cases; `__tests__/TemplatesShellModal.test.tsx` `Phase 9 URL import` block, 9 cases + 1 updated invariant): render, disabled-empty, disabled-non-http(s), trimmed-URL forwarding, in-flight state, inline error preserving input, retry clears error, thrown error surfaced, Cancel does not call the binding, Enter triggers Preview, button visible in shell, successful preview opens `ImportTemplatePreviewModal` with the URL path, Save to Library forwards canonical JSON, Apply to character forwards canonical JSON, Apply with overrides routes through Phase 6, v1 URL imports never see v2 buttons, `report.ok = false` keeps the URL modal open with inline error, thrown binding error keeps the modal open, Cancel closes without calling the binding.
- **Manual validation**: 2026-05-31 — pasting a public `https://` URL serving a v2 YAML opened the preview through the same modal as a file import; Save to Library persisted the URL payload as a library entry whose canonical JSON matches the preview; Apply to character wrote the selected fields through Phase 5D.2's endpoint; Apply with overrides routed the edited canonical JSON through the same endpoint. SSRF guards rejected, in turn, an `http://` URL, a literal `127.0.0.1`, a literal `169.254.169.254`, an empty URL, and a whitespace-only URL — each with the correct `IssueCode*` tag visible inline in the URL modal.
- **Risks**: SSRF — gated by §12.3 and exercised by the fetcher and handler test suites.
- **Out of scope (still future work)**: authenticated downloads (basic / bearer / cookies); URL auto-refresh; domain allowlist; direct apply without preview; optional `sourceURL` metadata persisted in the library schema (deferred until a separate decision approves widening the library shape); multi-character pack.

### Phase 10 — multi-character pack (separately approved)

- **Goal**: see §15. Source→destination mapping UI; per-slot rollback; explicit replace confirmation.
- **Out of scope until separately approved**.

### Phase 11 — named PvP / progression modules in templates (advanced, separately approved)

- **Goal**: optionally add `sections.modules` carrying a list of named module presets (e.g. `pvp.colosseums`) that delegate to existing controlled flows like `ApplyPvPPreparation`. **Never** carries raw flag IDs.
- **Out of scope until separately approved**.

### Phase 12 — optional removal / repositioning of `SortOrderTab` dropdown (separately approved)

- **Goal**: decide whether the existing dropdown becomes a redirect to the sidebar surface, is removed, or remains as a shortcut.
- **Out of scope until separately approved**.

---

## 17a. Validation status

### 17a.1. Manual validation log

| Field | Value |
|---|---|
| Validation date | 2026-05-31 |
| Branch under test | `feature/templating-system` |
| Outcome | ✅ Pass — user confirmed the full create / preview / save / export / re-import flow works end-to-end on a real save. |

| Field | Value |
|---|---|
| Validation date | 2026-05-31 |
| Branch under test | `feature/templates-v2-apply-profile-stats` |
| Outcome | ✅ Pass — Phase 5 v2 Apply for profile/stats from the library manually validated end-to-end (`ApplyBuildTemplateV2FromLibraryToCharacter`, `mode: "append"`). |

| Field | Value |
|---|---|
| Validation date | 2026-05-31 |
| Branch under test | `feature/templates-v2-direct-yaml-apply` |
| Outcome | ✅ Pass — Phase 5D.2 direct imported-YAML Apply for profile/stats manually validated end-to-end (`ApplyBuildTemplateV2ToCharacterJSON` on the canonical JSON produced by `PreviewBuildTemplateImportYAMLFromFile`, `mode: "append"`); no second file dialog, no TOCTOU re-read between preview and apply; v1 imports kept the legacy Save-to-Library-only behaviour; v2 imports with unsupported sections kept Apply disabled. |

| Field | Value |
|---|---|
| Validation date | 2026-05-31 |
| Branch under test | `feature/templates-v2-apply-overrides` |
| Outcome | ✅ Pass — Phase 6 apply-time overrides for profile/stats manually validated end-to-end on both surfaces (direct YAML import preview + library "Apply with overrides…"). Edited values landed on the selected character; unrelated fields untouched; the fast library Apply path (`ApplyBuildTemplateV2FromLibraryToCharacter`) remained unchanged; v1 imports never exposed the overrides button; v2 imports with unsupported sections kept both v2 buttons disabled with the supported-scope tooltip; cancelling the overrides modal discarded edits with no save mutation; `Save to Library` continued to persist the original canonical JSON, ignoring the in-modal edits. |

| Field | Value |
|---|---|
| Validation date | 2026-05-31 |
| Branch under test | `feature/templates-v2-url-import` |
| Outcome | ✅ Pass — Phase 9 URL import manually validated end-to-end against a public `https://` endpoint serving a v2 YAML. The URL preview surfaced in the same `ImportTemplatePreviewModal` as a file import; Save to Library persisted the URL payload as a library entry whose canonical JSON matches what the preview displayed (no `sourceURL` metadata added); Apply to character wrote the selected fields through Phase 5D.2's `ApplyBuildTemplateV2ToCharacterJSON`; Apply with overrides routed the edited canonical JSON through the same endpoint. SSRF guards rejected, in turn, an `http://` URL (`url_disallowed_scheme`), a literal `127.0.0.1` (`url_forbidden_ip`), a literal `169.254.169.254` (`url_forbidden_ip`), an empty URL (`url_empty`), and a whitespace-only URL (`url_empty` after trim) — each with the correct `IssueCode*` tag visible inline in the URL modal without losing the user's input. |

| Field | Value |
|---|---|
| Validation date | 2026-05-31 |
| Branch under test | `feature/templates-v1-weapon-level-override` |
| Outcome | ✅ Pass — Phase 6b weapon level override for the v1 `inventory.workspace` Apply path manually validated end-to-end on a real save. Applied a v1 template with mixed standard + somber + unupgradeable weapons under each control combination (override disabled / `StandardLevel` only / `SomberLevel` only / both / `StandardLevel = 26` clamped to `+25` with `weapon_level_clamped` / `SomberLevel = 11` clamped to `+10` with `weapon_level_clamped` / unupgradeable arm with `weapon_unupgradeable`). The override mutated the workspace only through `editor.UpdateWeapon`; the user still committed by `Save changes`; the Phase 5 / 5D.2 / 6 v2 Apply paths and the Phase 9 URL import path were unaffected; the v2 `inventory.workspace` scope guard in `app_templates_v2_apply.go` continued to refuse v2 inventory.workspace Apply; `TemplatesShellModal`, `ApplyOverridesPanel`, `ImportTemplatePreviewModal`, and `TemplateLibraryModal` were unchanged on every surface. |

| Field | Value |
|---|---|
| Validation date | 2026-05-31 |
| Branch under test | `feature/templates-v2-inventory-workspace-apply` (HEAD `3e448f0`) |
| Outcome | ✅ Pass — Phase 7a v2 `inventory.workspace` apply through the active Inventory Edit Session manually validated end-to-end on a real save. Applying a v2 template with `inventory.workspace` selected without an open Sort Order workspace surfaced the "Open the Sort Order workspace before applying inventory templates." toast and did **not** call the binding; opening the workspace and reapplying landed the items in the workspace grid; `Save changes` committed them to `slot.Data`; reloading the save showed the items with correct acquisition indices and no integrity warnings. A mixed profile + stats + inventory.workspace v2 template applied atomically — profile/stats fields and inventory items landed in a single user action; the dual slot+workspace snapshot rollback was exercised by deliberately introducing a stats validation failure after inventory items had been written — both the slot and the workspace reverted, and no items leaked. Both URL import and library Apply reused the same canonical JSON path. v2 templates carrying unsupported sections outside `{profile, stats, inventory.workspace}` kept both v2 buttons disabled with the supported-scope tooltip. Phase 5 / 5D.2 / 6 v2 profile/stats-only paths, Phase 6b weapon level override on the v1 SortOrderTab path, and the Phase 9 URL import path were all unaffected. `App.tsx`, `SortOrderTab.tsx`, and `ApplyOverridesPanel.tsx` were untouched. |

| Field | Value |
|---|---|
| Validation date | 2026-05-31 |
| Branch under test | `feature/templates-v2-weapon-override` (HEAD `8fccd72`) |
| Outcome | ✅ Pass — Phase 7a.2 apply-time weapon level override on the v2 `inventory.workspace` apply path manually validated end-to-end on a real save. Fast Apply without overrides kept weapon upgrade levels as declared in the template (the v2 path carried `weaponLevelOverride = undefined`). `Apply with overrides…` with `Standard = 25` set every standard weapon to +25 (and clamped over-cap requests with `weapon_level_clamped`) while leaving somber weapons at their template-side levels; `Somber = 10` mirrored the symmetric case for somber/special weapons; both levels set simultaneously applied per-class independently; enabling the panel with both inputs empty disabled Apply with the inline `apply-overrides-weapon-error` element; `Standard = 26` / `Somber = 11` / negative values disabled Apply with the range-specific error message. Closing the Sort Order workspace before applying surfaced the Phase 7a no-session toast ("Open the Sort Order workspace before applying inventory templates.") — the override never reached the backend regardless of the panel state. A mixed profile + stats + inventory.workspace v2 template with override applied atomically — profile/stats fields, inventory items, and the weapon levels all landed in a single user action; deliberately introducing a stats validation failure after inventory items had been written exercised the Phase 7a dual snapshot rollback again, and no override-mutated items leaked. A profile/stats-only template with a structurally valid override applied without rendering the weapon panel and without override warnings (silent ignore). URL import and library `Apply with overrides…` reused the same canonical JSON path and the same `weaponLevelOverride` field of `ApplyTemplateV2Options`. The v1 `SortOrderTab.tsx` Phase 6b dropdown continued to work byte-for-byte unchanged — its `weapon-override-*` testids did not collide with the new `apply-overrides-weapon-*` testids on the v2 surface. `App.tsx` and `SortOrderTab.tsx` were untouched; `WeaponLevelOverridePanel.tsx` is new; `ApplyOverridesPanel.tsx` and `TemplatesShellModal.tsx` carry the conditional-render + extended `onConfirm` plumbing. |

| Field | Value |
|---|---|
| Validation date | 2026-05-31 |
| Branch under test | `feature/templates-v2-equipment-writer-foundation` (HEAD `8fccd72` impl + `aefe848` docs) |
| Outcome | ✅ Pass — Phase 7b.0 equipment writer foundation manually validated through the 24-case `backend/core/equipment_writer_test.go` synthetic-`SaveSlot` suite (PC + PS4 round-trip on real fixtures deliberately deferred to Phase 7b.1 where the apply path exercises the writer end-to-end). All 14 supported slots covered for encoding correctness, class-gate enforcement, GaMap presence requirement, explicit clear, sentinel guard, atomicity, targeted hash 7 / 8 recompute, and an end-to-end weapon swap + clear sequence. No App method, no Wails binding, no frontend — nothing to manually click in the GUI in this phase. Full backend `go test . ./backend/... ./tests/...` PASS; `go vet` clean; `make build` PASS with `frontend/wailsjs/**` byte-identical. |

| Field | Value |
|---|---|
| Validation date | 2026-06-01 |
| Branch under test | `feature/templates-v2-talisman-apply` (HEAD `b9088a7`) |
| Outcome | ✅ Pass — Phase 7c v2 talisman apply through `sections.equipment.talisman1..5` end-to-end manually validated on a real save. Exporting an equipment-only template from a character with active talismans produced YAML containing `sections.equipment.talisman1..N` (depending on equipped count) with `baseItemID` + optional `name` only — no `upgrade` / `infusionName` / `aowItemID` for talisman refs, no separate `sections.equippedTalismans` section. Importing the same template into the preview modal listed the talisman keys in the existing `import-preview-equipment-slots` row; Apply stayed enabled for the equipment-only talisman template without an active Inventory Edit Session. Fast library Apply re-equipped the listed talismans; saving and reloading the save through the editor showed the talismans intact; an in-game load on a real PC save confirmed the character spawned with the correct talisman loadout, talisman effects (stat boosts, damage modifiers) were active, and a Save & Quit + reload round-trip preserved the talismans. Pouch gating with `TalismanSlots = 0` (only 1 active slot) applied to a 4-talisman template equipped slot 1 successfully and emitted `talisman_slot_pouch_insufficient` warnings for slots 2 / 3 / 4 with slots 18 / 19 / 20 left untouched — no hard error, the apply still committed. A mixed template setting `profile.talismanSlots = 3` and `equipment.talisman4` on a character with `TalismanSlots = 0` lifted the pouch to 4 active slots during profile apply and successfully equipped talisman4 in the same apply (covered by `computeActiveTalismanSlots`). A template with `talisman5` and `baseItemID > 0` surfaced `talisman_slot_pouch_insufficient` warn + skip every time, regardless of pouch state; the same template with `talisman5: {baseItemID: 0}` cleared the slot without a warning. A template referencing a talisman missing from inventory emitted `equipment_item_not_in_inventory` warning + skip — no auto-add and no storage resolution. A template carrying both `sections.equipment.talisman1` and `sections.inventory.workspace` surfaced the `equipment_inventory_combo_unsupported` error at preview time — Apply stayed disabled, zero side effects landed on the slot or any open workspace. Regression sweep verified: weapons / ammo / armor apply (Phase 7b.1) unchanged on the same `sections.equipment` surface; v2 profile/stats Apply (Phase 5 / 5D.2) unchanged; v2 inventory.workspace Apply still requires an active session (Phase 7a); v2 weapon level override on the inventory.workspace path still renders `WeaponLevelOverridePanel` inside `ApplyOverridesModal` (Phase 7a.2); URL import still flows through the same preview / Apply path including for templates carrying talismans (Phase 9); v1 `SortOrderTab.tsx` Phase 6b dropdown unchanged byte-for-byte; v2 profile / stats overrides still work (Phase 6). `App.tsx` and `SortOrderTab.tsx` were untouched; `ImportTemplatePreviewModal.tsx`, `TemplateLibraryModal.tsx`, `TemplatesShellModal.tsx`, `ApplyOverridesPanel.tsx`, and `WeaponLevelOverridePanel.tsx` carry no talisman-specific source-code change. |

| Field | Value |
|---|---|
| Validation date | 2026-06-01 |
| Branch under test | `feature/templates-v2-equipment-apply` (HEAD `42ff906`) |
| Outcome | ✅ Pass — Phase 7b.1 v2 `sections.equipment` end-to-end manually validated on a real save. Exported an equipment-only template from a character with weapons equipped on RH1 / LH1 and armor on all four armor slots — the resulting YAML carried `sections.equipment` with the canonical 14-slot field names and no talisman / GreatRune / slot 11 / slot 16 / spells / pouch leak. Importing the same template into the preview modal rendered the new `import-preview-equipment-slots` row listing `weaponRightHand1, weaponLeftHand1, armorHead, armorChest, armorArms, armorLegs`; Apply stayed enabled for the equipment-only template without an active Inventory Edit Session. Fast library Apply re-equipped the listed items; `Save changes` was not needed; saving and reloading the save through the editor showed the equipment intact; an in-game load on a real PC save subsequently confirmed the character spawned with the correct equipment loadout, stats / attack rating / poise matched the equipped items, and a Save & Quit + reload round-trip preserved the equipment. A template referencing an item missing from inventory (still in the DB, just removed from the character) emitted `equipment_item_not_in_inventory` warnings in the preview report and skipped the slot at apply time — no auto-add and no mutation of the affected slot. A template with `BaseItemID: 0` for `weaponLeftHand1` cleared the slot to empty (writing `0xFFFFFFFF` underneath) while leaving the other slots untouched. A template carrying two identical weapons in inventory (same baseItemID, same upgrade level) resolved to the first match and emitted the `equipment_item_ambiguous` warning. A template carrying both `sections.equipment` and `sections.inventory.workspace` surfaced the `equipment_inventory_combo_unsupported` error at preview time — Apply stayed disabled, the shell did not call the binding, and zero side effects landed on the slot or any open workspace. Regression sweep verified: v2 profile/stats Apply (Phase 5 / 5D.2) unchanged; v2 inventory.workspace Apply still requires an active session and continues to use `Save changes` as the only commit point for inventory (Phase 7a); v2 weapon level override on the inventory.workspace path still renders the `WeaponLevelOverridePanel` inside `ApplyOverridesModal` and applies correctly (Phase 7a.2); URL import still flows through the same preview / Apply path including for equipment-only templates (Phase 9); v1 `SortOrderTab.tsx` Phase 6b dropdown unchanged byte-for-byte. `App.tsx` and `SortOrderTab.tsx` were untouched; `ApplyOverridesPanel.tsx` and `WeaponLevelOverridePanel.tsx` carry no equipment-specific controls. |

### 17a.2. Flow exercised

The following user-facing flow was driven manually and confirmed working:

1. Open the global `Templates` sidebar entry → `Create from Character…`.
2. Pick the source character; the modal opens with profile / stats sections collapsed by default.
3. Per-section enable + per-field toggle: enable `profile`, pick a subset of profile fields (e.g. `name`, `level`, `class`); enable `stats`, pick a subset of the 8 stat fields. Canonical selection key for the class field is `class` — `className` is not a valid selection key.
4. `Preview schema v2` renders the v2 metadata (schema key, version, selection summary) and the resolved per-field values from the source character.
5. `Save to Library` writes the v2 document into the local library (JSON on disk per §10.1) with a `v2` badge and a selected-sections summary in the library list.
6. `Export YAML from library` produces a `.yaml` file containing the same v2 payload.
7. `Import` the exported `.yaml` back through the Templates shell; preview shows the same v2 metadata and selected sections.
8. `Apply` for a v2 library entry is enabled **only** when its `selectedSections ⊆ { profile, stats }`. Clicking Apply runs an inline confirm in the library row, then `TemplatesShellModal` calls `ApplyBuildTemplateV2FromLibraryToCharacter(charIdx, libraryEntryID, { mode: "append" })`. On success, `App.tsx` bumps `inventoryVersion` + `saveLoadKey` and triggers `refreshSlots` + `refreshUndoDepth`. v2 entries carrying any other section keep the Apply button disabled; the existing Phase 3B.0 guard in `app_templates.go` still refuses v1 Apply for any v2 document.

### 17a.2a. Phase 5 flow exercised

1. Open the global `Templates` sidebar entry → library.
2. Select a v2 library entry whose `selectedSections ⊆ { profile, stats }`. The Apply button is enabled; v2 entries carrying any other section keep it disabled, and v1 entries still use the unchanged v1 Apply path.
3. Click Apply → inline confirm appears inside the library row.
4. Confirm → `TemplatesShellModal` invokes `ApplyBuildTemplateV2FromLibraryToCharacter` with the active `charIdx` and `mode: "append"`.
5. Backend runs the Phase 5 apply layer under `slotMu[charIdx]` (snapshot + rollback on error), skipping `profile.class` and surfacing it in `ApplyTemplateV2Result.Skipped`.
6. `App.tsx` refreshes `inventoryVersion`, `saveLoadKey`, slots, and undo depth so the visible character / save state updates without a reload.

### 17a.2b. Phase 5D.2 direct imported-YAML flow exercised

1. Open the global `Templates` sidebar entry → `Import YAML from File…`.
2. Pick a v2 `.yaml` template whose `selectedSections ⊆ { profile, stats }`. The shell calls `PreviewBuildTemplateImportYAMLFromFile`, which returns the parsed report **and** a canonical JSON serialisation of the same payload.
3. The `ImportTemplatePreviewModal` opens with the v2 metadata, the report panel, the existing `Save to Library` button, and — only for v2 imports — a new `Apply to character` button (testid `import-preview-apply-v2`).
4. The new button is enabled only when the report is OK, a save is loaded, a character is selected, the selection is non-empty, and every selected section is in the module-level `V2_APPLY_SUPPORTED_SECTIONS = ['profile', 'stats']`. v1 imports omit the `onApplyV2` prop, so the button is not rendered at all.
5. Click Apply → `TemplatesShellModal.handleApplyV2FromImportedPreview` calls `ApplyBuildTemplateV2ToCharacterJSON(charIndex, importedPreview.canonicalJSON, { mode: "append" })`. The bytes that get applied are byte-for-byte the canonical JSON the user previewed — there is no second file dialog and no re-read of the YAML on disk.
6. On `result.applied === true`, the preview closes, a success toast names the YAML path and the slot label, `onCharacterTemplateApplied(charIndex)` fires (so `App.tsx` runs its existing post-Phase-5D.1 refresh dance — `inventoryVersion`, `saveLoadKey`, slots, undo), and an info toast announces the `profile.class` skip if `class` appeared in `result.skippedFields`.
7. On `result.applied === false` or a thrown binding error, an error toast is raised and the preview stays open so the user can retry or close it manually.
8. The existing `Save to Library` action is untouched; clicking it on the same preview saves the imported template into the library as before, and the Phase 5D.1 library Apply path remains the source of truth for entries already stored locally.

### 17a.2c. Phase 6 apply-time overrides flow exercised

1. Open the global `Templates` sidebar entry.
2. **Direct YAML path** — click `Import YAML from File…`, pick a v2 `.yaml` whose `selectedSections ⊆ { profile, stats }`. The preview shell calls `PreviewBuildTemplateImportYAMLFromFile`, which returns the same canonical JSON Phase 5D.2 already consumes. The preview modal renders the v2 metadata, the existing `Save to Library` button, the existing `Apply to character` button (Phase 5D.2), and the new `Apply with overrides…` button (Phase 6, testid `import-preview-apply-v2-overrides`).
3. Click `Apply with overrides…` → `TemplatesShellModal` records an `OverridesSource` of kind `'import'` (carrying the YAML path label) and opens `ApplyOverridesModal` with the preview's canonical JSON.
4. `ApplyOverridesPanel` parses the JSON, renders editable rows for the eight overridable profile fields and the eight stats, renders `profile.class` as read-only with the "Skipped on apply (Phase 5)" hint when present, and ignores any other section. Range-validates every keystroke; emits a mutated canonical JSON whenever the draft changes.
5. Edit values (e.g. raise `profile.level` from 50 to 55, raise `stats.vigor` from 25 to 30, raise `profile.scadutreeBlessing` from 0 to 5). Toggle a previously-unselected field on by clicking its checkbox before typing.
6. Click `Apply to character` in the overrides modal → `TemplatesShellModal.handleConfirmOverrides` posts the mutated JSON through `ApplyBuildTemplateV2ToCharacterJSON(charIdx, mutatedCanonicalJSON, { mode: "append" })`. The library `…FromLibraryToCharacter` endpoint is not called from this surface.
7. On `result.applied === true`, both modals close, a success toast names the source label and the slot, `onCharacterTemplateApplied(charIndex)` fires (so `App.tsx` runs its existing post-Phase-5D.1 refresh dance), and an info toast announces the `profile.class` skip when the template carried `class`.
8. On `result.applied === false` or a thrown binding error, the overrides modal stays open and an error toast is raised so the user can fix the values and retry.
9. **Library path** — click `Apply with overrides…` (testid `library-apply-overrides`) on a v2 library row whose `selectedSections ⊆ { profile, stats }`. `TemplatesShellModal.handleOpenOverridesFromLibrary` calls `PreviewBuildTemplateFromLibrary(entry.id)` to obtain the canonical JSON and the report, then opens the same `ApplyOverridesModal` with an `OverridesSource` of kind `'library'` (carrying the entry label). Steps 4–8 above apply identically.
10. The existing fast library Apply through `ApplyBuildTemplateV2FromLibraryToCharacter` remains the click target of the original `Apply` button on the same row, with its inline confirm row untouched.
11. v1 imports and v1 library entries never render the overrides button. v2 imports / entries carrying any unsupported section keep both v2 buttons disabled with the existing "profile / stats only in this phase" tooltip.

### 17a.2d. Phase 9 URL import flow exercised

1. Open the global `Templates` sidebar entry → `Import from URL…` (testid `templates-shell-import-url`).
2. Paste a public `https://` URL serving a v2 YAML and click `Preview` (or press Enter). `TemplatesShellModal.onURLImportPreview(rawURL)` calls `PreviewBuildTemplateImportYAMLFromURL(rawURL)`, which trims whitespace, delegates to `templates.FetchYAMLFromURL`, and on success hands the bytes to the same `previewYAMLPayload` the file-import path uses.
3. On `report.ok = true` the URL modal closes, the shell sets `importedPreview = { report, canonicalJSON: bundle.json, path: bundle.path ?? rawURL }`, and the existing `ImportTemplatePreviewModal` opens with the URL as the source label. The modal renders identically to a file-imported v2 preview — same v2 metadata, same `Save to Library`, same `Apply to character` (Phase 5D.2), same `Apply with overrides…` (Phase 6).
4. On `report.ok = false` the URL modal stays open with the inline `import-url-error` element rendering the backend's `IssueCode*` message (e.g. `url_forbidden_ip: 127.0.0.1 is not allowed.`), the input value is preserved so the user can edit and retry, and no preview modal is opened.
5. `Save to Library` on a URL-imported preview reuses the existing file-import path: the persisted library entry is byte-for-byte the same canonical JSON the preview displayed; no `sourceURL` metadata is recorded in the library in this phase.
6. `Apply to character` on a URL-imported preview reuses Phase 5D.2's `ApplyBuildTemplateV2ToCharacterJSON(charIdx, canonicalJSON, { mode: "append" })`; no second fetch, no second URL hit, no TOCTOU re-read.
7. `Apply with overrides…` on a URL-imported preview reuses Phase 6's `ApplyOverridesModal` end-to-end; the mutated canonical JSON is posted through the same Phase 5D.2 endpoint.
8. v1 URL imports omit the `onApplyV2` / `onApplyV2WithOverrides` props at the modal level, so neither v2 button is rendered. v2 URL imports carrying any unsupported section keep both v2 buttons disabled with the existing "profile / stats only in this phase" tooltip.
9. SSRF guards run before any preview opens: `http://` schemes, `data:` schemes, literal loopback IPs, literal cloud-metadata IPs, DNS-resolved private IPs, oversized bodies, disallowed Content-Types, timeouts, TLS errors, and redirects to forbidden destinations are all rejected with a precise `IssueCodeURL*` tag and surfaced inline. The fetch alone never modifies the library or the save.

### 17a.2e. Phase 6b weapon level override flow exercised

1. Open the Sort Order tab; load a save and select a slot; open the Templates dropdown (the existing `Apply Template ▾` action — `TemplatesShellModal`, `ApplyOverridesPanel`, `ImportTemplatePreviewModal`, and `TemplateLibraryModal` are untouched on this surface in Phase 6b).
2. The new `weapon-override-panel` (testid) sits inline inside the Templates dropdown. The master `weapon-override-enabled` checkbox is unchecked by default; both `weapon-override-standard` (range `0..25`) and `weapon-override-somber` (range `0..10`) inputs are hidden. With the master off, an Apply behaves byte-for-byte like the pre-Phase-6b path — `WeaponLevelOverride = undefined` reaches the backend.
3. Toggling the master on reveals both number inputs. Both empty + master on surfaces the inline `weapon-override-error` element and disables Apply (matches `validateWeaponLevelOverride` rejecting `Enabled = true` with both pointers nil).
4. Filling only `weapon-override-standard` with `+25` and clicking `Apply` builds `{ enabled: true, standardLevel: 25 }`. The shell calls the v1 inventory.workspace Apply backend, which runs `applyTemplateItemsToWorkspace`; per added weapon, `editor.AddItem` populates `editor.EditableItem.MaxUpgrade` from `db.GetItemDataFuzzy`. After each template-side `editor.UpdateWeapon` patch (Upgrade / Infusion / AoW from the template), `applyWeaponLevelOverride` runs: standard weapons (`MaxUpgrade == 25`) re-receive `editor.UpdateWeapon{Upgrade: &25}`; somber weapons (`MaxUpgrade == 10`) keep their template-side level; unupgradeable entries (`MaxUpgrade == 0`) are silently skipped because `SomberLevel` is nil. No warnings on this case.
5. Filling only `weapon-override-somber` with `+10` and clicking `Apply` builds `{ enabled: true, somberLevel: 10 }`. Somber weapons land at `+10`; standard weapons keep their template-side level; unupgradeable entries are silently skipped.
6. Filling both at the max for their class — `25` standard + `10` somber — applies each class independently in the same apply.
7. Requesting `+26` standard or `+11` somber: the helper computes `clamped := editor.ClampUpgrade(req, MaxUpgrade)` (`+25` and `+10`), re-applies via `editor.UpdateWeapon{Upgrade: &clamped}`, and appends `templates.IssueCodeWeaponLevelClamped` ("weapon_level_clamped") to `report.Warnings`. The frontend toast filter surfaces the override-warning hint without blocking the apply.
8. Adding an unupgradeable weapon (`MaxUpgrade == 0`, e.g. Unarmed) emits `templates.IssueCodeWeaponUnupgradeable` ("weapon_unupgradeable") to `report.Warnings` and skips the override for that entry. The template-side state is preserved.
9. The override path never adds, removes, or relocates items; it never touches `Infusion` or `AoW`; the mutation stays entirely inside the active `InventoryWorkspaceSnapshot`. `SaveInventoryWorkspaceChanges` remains the only commit point. The user still clicks `Save changes` to persist to `slot.Data`.
10. The Phase 5 / 5D.2 / 6 v2 Apply paths and the Phase 9 URL import path do not expose the override panel (their UI surface is `TemplatesShellModal`, not `SortOrderTab.tsx`); their behaviour is byte-for-byte unchanged. The v2 `inventory.workspace` scope guard in `app_templates_v2_apply.go` continues to refuse v2 inventory.workspace Apply.

### 17a.2f. Phase 7a v2 inventory.workspace apply flow exercised

1. Open the Sort Order tab on the target character so an `InventoryEditSession` is acquired and its `sessionID` is registered in `editSessionByChar[charIdx]`. Without this step, every v2 Apply for a template carrying `inventory.workspace` will be refused upstream with the new "Open the Sort Order workspace before applying inventory templates." toast.
2. Open the global `Templates` sidebar entry. Pick one of three surfaces — Library, `Import YAML from File…`, or `Import from URL…`.
3. For any v2 template whose `selectedSections` includes `'inventory.workspace'`, `TemplatesShellModal` calls `GetActiveInventoryEditSessionForCharacter(charIndex)` before the apply binding. The endpoint reads `editSessionsMu` and returns `{ active: true, sessionID }` for the active session, `{ active: false }` for invalid `charIdx` or no active session — it never errors.
4. No active session → red toast surfaces `NO_SESSION_MESSAGE` ("Open the Sort Order workspace before applying inventory templates.") and the binding is **not** called. No backend mutation. The library row / preview button stays clickable so the user can retry after opening the workspace.
5. Active session → the shell forwards `{ mode: "append", sessionID }` to `ApplyBuildTemplateV2ToCharacterJSON(charIdx, canonicalJSON, opts)` for the direct YAML / URL / overrides surfaces, or to `ApplyBuildTemplateV2FromLibraryToCharacter(charIdx, entry.id, opts)` for the library Apply surface (the library path uses the same `ApplyTemplateV2Options` struct).
6. Backend classifies the parsed payload into `hasProfile`, `hasStats`, `hasInventory`. With `hasInventory == true` it calls `acquireSession(opts.SessionID)`; an empty `SessionID` → `IssueCodeInventorySessionRequired`; an unknown session id or a session bound to a different character → `IssueCodeInventorySessionInvalid` (and the session is unlocked on the wrong-character branch before returning). On success, `defer sess.Unlock()` keeps the session held for the duration of the apply.
7. The apply runs capacity preflight on the **existing** `sess.Workspace` before any mutation. On preflight error the apply returns with `Applied = false` and the workspace stays unchanged.
8. Slot + workspace snapshots are taken: `slotBackup := core.SnapshotSlot(slot)` and `workspaceBackup := deepCopySnapshot(sess.Workspace)`. From this point every error exit calls `rollbackBoth()` so a partial mixed apply cannot leak.
9. Inventory items are applied via `applyTemplateItemsToWorkspace(&sess.Workspace, sec.InventoryItems, editor.ContainerInventory, nil)`; storage items via the storage equivalent with the same `nil` override pin. The `nil` is intentional and documents that Phase 6b weapon level override is **not** wired into the v2 path in this phase — Phase 6b stays a v1 `SortOrderTab.tsx` dropdown feature.
10. When inventory items land, `sess.Workspace.Dirty = true` and the snapshot is revalidated. The sentinel `"inventory.workspace"` is appended to `result.Applied` so downstream consumers can detect the section participated in the apply. `result.InventoryItemsApplied` and `result.StorageItemsApplied` carry the per-container counts; `result.Workspace` carries the post-apply snapshot for UI refresh.
11. Profile / stats writes run **after** the inventory branch under the same `slotMu[charIdx]` window, reusing the Phase 5 path (`vm.ApplyVMToParsedSlot` + `slot.SyncPlayerToData`) byte-for-byte. If they fail after inventory items have already landed in `sess.Workspace`, `rollbackBoth()` restores both — atomicity preserved.
12. v2 templates whose `selectedSections` is `{profile, stats}` only continue to apply with `sessionID = ''` exactly as before Phase 7a; `acquireSession` is **not** called on that path, and the existing Phase 5 edit-session conflict guard (refusing apply when a session is held for another open edit) is preserved unchanged.
13. The user still commits the workspace to `slot.Data` through the existing `Save changes` button on `SortOrderTab.tsx` — `SaveInventoryWorkspaceChanges` remains the only commit point. The v2 inventory apply itself never writes to `slot.Data`.

### 17a.2g. Phase 7a.2 v2 weapon level override flow exercised

1. Open the Sort Order tab on the target character so an `InventoryEditSession` is acquired and the Phase 7a session-gating contract is satisfied. Without an active session every v2 Apply for a template carrying `inventory.workspace` is still refused upstream — the Phase 7a.2 override never reaches the backend in that case.
2. Open the global `Templates` sidebar entry. Pick one of three surfaces — Library, `Import YAML from File…`, or `Import from URL…` — and select a v2 template whose `selectedSections` includes `'inventory.workspace'` (with or without `profile` / `stats`).
3. Click `Apply with overrides…` (testid `library-apply-overrides`, `import-preview-apply-v2-overrides`, or the URL-equivalent). `TemplatesShellModal` opens `ApplyOverridesModal` with the preview's canonical JSON exactly as Phase 6 / Phase 7a already did.
4. The modal now parses `selection['inventory.workspace']` from the canonical JSON. When present (boolean `true` or an object with `all: true` / non-empty fields), the modal renders the new `WeaponLevelOverridePanel` (testid `apply-overrides-weapon-panel`) under the existing profile/stats grid. Profile/stats-only templates leave the panel unrendered; inventory-only templates may use the modal exclusively for weapon level override (the profile/stats grid renders empty headings, no fields).
5. The weapon panel is disabled by default with both inputs hidden. Toggling `apply-overrides-weapon-enabled` reveals `apply-overrides-weapon-standard` (range `0..25`) and `apply-overrides-weapon-somber` (range `0..10`) plus the inline `apply-overrides-weapon-error` element (rendered only while the panel is in an invalid state). Both inputs are empty on first reveal — the user must explicitly type at least one level to make the override actionable.
6. The panel emits its state through `onChange(override, hasInvalid)`. `override` is `{ enabled: true, standardLevel?: number, somberLevel?: number } | undefined` — a disabled master toggle, or an enabled master with no inputs typed, emits `undefined`; an enabled master with one or both inputs filled with in-range values emits the structurally valid payload. `hasInvalid` is `true` when the master is enabled and (a) both inputs are empty, (b) `standardLevel` is `< 0` / `> 25`, or (c) `somberLevel` is `< 0` / `> 10`.
7. `ApplyOverridesModal` blocks Apply (`canApply = false`) whenever either the profile/stats panel or the weapon panel reports invalid input. The status pill flips between three messages: `"Ready to apply."`, `"N field(s) need attention."` (profile/stats invalid), and `"Fix weapon level override to apply."` (weapon panel invalid).
8. Clicking Apply calls `onConfirm(mutatedJSON, weaponOverride)` with the panel's current emission. `TemplatesShellModal.handleConfirmOverrides` parses the mutated JSON, runs the Phase 7a session check (`canonicalJSONNeedsSession` + `fetchActiveSessionID`), and posts through `ApplyBuildTemplateV2ToCharacterJSON(charIdx, mutatedJSON, main.ApplyTemplateV2Options.createFrom({ mode: "append", sessionID, weaponLevelOverride }))`. The fast library Apply path (`ApplyBuildTemplateV2FromLibraryToCharacter`) is **not** invoked from this surface and never carries the override.
9. Backend validates the override **before** `acquireSession` via `validateWeaponLevelOverride(opts.WeaponLevelOverride)`. A structurally broken override → `Applied = false`, `Preview.Errors = [{ code: structure_invalid, … }]`, zero side effects. A structurally valid override on a profile/stats-only template (`hasInventory == false`) is silently ignored — the override loop has no items to operate on. For `hasInventory == true`, the two `applyTemplateItemsToWorkspace(&sess.Workspace, …, opts.WeaponLevelOverride)` calls thread the override into the existing helper; `applyWeaponLevelOverride` runs after each template-side weapon patch and emits `weapon_level_clamped` / `weapon_unupgradeable` warnings into `report.Warnings` via the existing `invWarn` / `stoWarn` aggregation. Warnings never roll back.
10. On `Applied = true`, both modals close, a success toast names the source label and the slot, `onCharacterTemplateApplied(charIndex)` fires (so `App.tsx` runs its existing post-Phase-5D.1 refresh dance), and an info toast announces the `profile.class` skip when the template carried `class`. The user still commits the workspace to `slot.Data` through the existing `Save changes` button on `SortOrderTab.tsx` — `SaveInventoryWorkspaceChanges` remains the only commit point. The v2 inventory apply itself never writes to `slot.Data` from the override path.
11. The v1 `SortOrderTab.tsx` Phase 6b dropdown is byte-for-byte unchanged. Its `weapon-override-*` testids do not collide with the v2-surface `apply-overrides-weapon-*` testids on `ApplyOverridesModal`, so v1 dropdown tests and v2 modal tests can coexist in the same Vitest run without interference. The v1 apply path (`ApplyBuildTemplateToWorkspaceJSON` with `ApplyTemplateOptions.WeaponLevelOverride`) is untouched.

### 17a.2h. Phase 7b.1 v2 equipment apply flow exercised

1. Open the global `Templates` sidebar entry → `Create from Character…`. Pick the source character and enable the new `equipment` section (boolean shortcut or per-field for the 14 allowed slot keys: weapons LH/RH 1/2/3, arrows1/2, bolts1/2, armor head/chest/arms/legs). Other sections may be enabled too **except** `inventory.workspace` — selecting both `equipment` and `inventory.workspace` is hard-rejected at preview time with `equipment_inventory_combo_unsupported`.
2. `Preview schema v2` runs `PreviewBuildTemplateImport` which now appends `"equipment"` to `Summary.SelectedSections` and lists the populated 14-slot keys in `Summary.EquipmentSlotsPresent` (canonical order). The new `import-preview-equipment-slots` row renders both lists; `import-preview-selected-sections` shows `equipment` alongside `profile` / `stats` when mixed. v1 templates never expose this row.
3. `Save to Library` persists the canonical v2 JSON. The library list renders the entry with the existing v2 badge and the standard selected-sections summary; equipment-only entries are now Apply-eligible without an Inventory Edit Session.
4. Export the entry as YAML through the existing library `Export as YAML…` action. The resulting file carries `sections.equipment` at the top level with the slot-name-keyed fields (`weaponRightHand1`, `armorHead`, etc.); each slot is either an `EquipmentItemRef` carrying `baseItemID` + optional `name` / `upgrade` / `infusionName` / `aowItemID`, or an explicit-clear sentinel (`{baseItemID: 0}`). Talismans, EquippedGreatRune, slots 11/16, spells, quick / pouch items are absent — the canonical export format documents the Phase 7b.1 scope exactly.
5. From any of three surfaces — Library, `Import YAML from File…`, or `Import from URL…` — pick a v2 equipment-only template. The shell's `needsSession = selectedSections.includes('inventory.workspace')` gate naturally excludes equipment-only templates, so no `GetActiveInventoryEditSessionForCharacter` toast is surfaced. Fast Apply (`library-apply` / `import-preview-apply-v2` / URL-equivalent) calls `ApplyBuildTemplateV2FromLibraryToCharacter` / `ApplyBuildTemplateV2ToCharacterJSON` with `sessionID = ''` for equipment-only templates; an open session ID is forwarded transparently if one happens to be active (backend silently ignores it on the non-inventory branch).
6. Backend classifies the parsed payload into `hasProfile`, `hasStats`, `hasInventory`, `hasEquipment`. `hasEquipment + hasInventory` is hard-rejected before snapshot / session acquire with `equipment_inventory_combo_unsupported` and zero side effects (defence in depth — the same combo is already rejected at preview).
7. The slot snapshot is taken (`core.SnapshotSlot`) inside `slotMu[charIdx]`; the workspace snapshot is taken only when `hasInventory == true` (it is `nil` for equipment-only / equipment+profile / equipment+stats applies). `rollbackBoth()` continues to cover both on every error exit.
8. Equipment resolution runs after profile/stats apply to VM but before any byte mutation. `resolveEquipmentWrites(slot, sel, sec)` calls `editor.BuildSnapshot(slot, "", -1)` once for a consistent view, then delegates to `resolveEquipmentWritesFromItems(items, sel, sec)`: for each selected slot it resolves `baseItemID == 0` to a clear write, walks `InventoryItems` matching `BaseItemID` plus any supplied `Upgrade` / `InfusionName` / `AoWItemID` disambiguators, emits `equipment_item_not_in_inventory` warnings + skips on miss, and `equipment_item_ambiguous` warnings + first-match-wins on duplicates. Storage is intentionally not searched.
9. If the resolver produced any writes, `"equipment"` is appended to `result.Applied`; the `profileOrStatsApplied` check skips `"equipment"` so equipment-only applies do not trigger the unrelated `vm.ApplyVMToParsedSlot` path. Profile/stats writes (when present) run through their existing path with `SyncPlayerToData` / NG+ flag sync / ProfileSummary update.
10. `SaveSlot.WriteEquipment(equipmentWrites)` dispatches the resolved batch. On success, hash 7 / hash 8 are recomputed per Phase 7b.0's targeted-recompute discipline (slots 0–9 → hash 7; slots 12–15 → hash 8) and `result.EquipmentSlotsApplied = len(equipmentWrites)`. On writer failure (handle-prefix mismatch, missing GaMap entry — both unreachable through the resolver but defended), `rollbackBoth()` reverts both `slot.Data` and the workspace, and an `equipment_slot_invalid` error is injected into `report.Errors` with `result.Applied = false`.
11. The user does NOT click `Save changes` for equipment — equipment apply commits directly to `slot.Data` (the writer's hash recompute makes the slot self-consistent), mirroring the `EquippedGreatRune` write path that has always lived inside `SyncPlayerToData`. The `Save changes` button continues to be the only commit point for the `inventory.workspace` section, which is a mutually exclusive selection in this phase.
12. v2 templates whose `selectedSections` contains anything outside `{profile, stats, inventory.workspace, equipment}` keep the v2 Apply buttons disabled with the standard "Unsupported v2 sections — apply is available only for profile, stats, inventory.workspace, and equipment in this phase." tooltip — the supported-section list is the modal-level allowlist in `V2_APPLY_SUPPORTED_SECTIONS`.

### 17a.2i. Phase 7c v2 talisman apply flow exercised

1. Open the global `Templates` sidebar entry → `Create from Character…`. Pick the source character and enable the `equipment` section. The new talisman keys `talisman1..5` join the existing 14-slot allowlist (weapons LH/RH 1/2/3, arrows1/2, bolts1/2, armor head/chest/arms/legs), bringing `equipmentSelectionFields` and `EquipmentSlotOrder` to 19 entries with talismans appended after `armorLegs`.
2. `Preview schema v2` runs `PreviewBuildTemplateImport` which lists every populated slot (weapons/ammo/armor/talismans) in `Summary.EquipmentSlotsPresent` in canonical order. The existing `import-preview-equipment-slots` row renders the list; no new preview row is introduced for talismans — they appear alongside other equipment slots.
3. The canonical YAML produced by `BuildV2Template` carries `sections.equipment.talisman1..N` (where N is the highest populated slot) with `baseItemID` + optional `name`. The exporter intentionally drops `upgrade`, `infusionName`, and `aowItemID` for talismans because `itemToEquipmentRef` sets `Upgrade` only when `IsWeapon || IsArmor` and only emits `AoWItemID` for current weapon AoW state. There is **no separate `sections.equippedTalismans` section** in the canonical YAML.
4. From any of the three surfaces — Library, `Import YAML from File…`, or `Import from URL…` — pick a v2 template carrying talismans. The shell's `needsSession` gate still excludes templates whose `selectedSections` is `{equipment}` only, so equipment-only talisman templates apply without an active Inventory Edit Session.
5. Backend classifies the parsed payload into `hasProfile`, `hasStats`, `hasInventory`, `hasEquipment`. The `hasEquipment + hasInventory` combo remains hard-rejected with `equipment_inventory_combo_unsupported` whenever talismans appear under `sections.equipment` — talismans are not a special case here.
6. Active talisman pouch capacity is computed by `computeActiveTalismanSlots(slot, tpl)` as `1 + effective profile.talismanSlots`, where the effective value is the template's `profile.talismanSlots` when it is present AND `selection.profile.talismanSlots` is true (template wins because profile apply runs before equipment apply), otherwise the slot's current persisted `Player.TalismanSlots`. Both branches clamp to `MaxProfileTalismanSlots = 3`, so the resolver caps the active capacity at 4.
7. `resolveEquipmentWrites(slot, sel, sec, activeTalismanSlots)` walks `EquipmentSlotOrder`. For each populated talisman slot, the new branch determines the slot ordinal (1..5 via `talismanSlotOrdinal`); a non-empty ref where `ord > MaxActiveTalismanSlots || ord > activeTalismanSlots` emits `talisman_slot_pouch_insufficient` warning + skip. Talisman5 always trips the warning when populated with `baseItemID > 0` because `MaxActiveTalismanSlots = 4`. Explicit-clear refs (`baseItemID = 0`) bypass the gate entirely — clearing Talisman5 is always allowed and writes `0xFFFFFFFF`.
8. Otherwise the resolver matches the talisman ref against `slot.Inventory.CommonItems` by `BaseItemID` (storage is intentionally NOT searched, mirroring Phase 7b.1). Missing talisman → `equipment_item_not_in_inventory` warning + skip; ambiguous talisman → first-match-wins + `equipment_item_ambiguous` warning. No auto-add and no storage resolution. `Upgrade` / `InfusionName` / `AoWItemID` are still passed through to `lookupEquipmentHandle` for uniformity but talisman editable items always have `CurrentUpgrade = 0` / `InfusionName = ""` / `!HasCurrentAoW`, so they are no-ops.
9. `SaveSlot.WriteEquipment(equipmentWrites)` dispatches the resolved batch. For talisman slots the encoded slot value is `GaMap[handle]` directly (no `| 0x80000000` mask); the writer's `slotClassTalisman` class accepts only the `ItemTypeAccessory` (0xA0) handle prefix. Hash 8 recompute already iterates `armorSlotIndices = [12..15, 17..21]`, so the touched-slot range simply widens to include 17–21; hash 7 stays untouched for talisman-only writes.
10. `result.EquipmentSlotsApplied` counts talisman writes alongside weapon/ammo/armor writes — there is no separate `talismansApplied` counter. The combined count surfaces in the existing UI summary unchanged.
11. The user does NOT click `Save changes` for talismans — equipment apply commits directly to `slot.Data` (the writer's hash 8 recompute makes the slot self-consistent), mirroring the Phase 7b.1 weapon/ammo/armor write path. `SaveInventoryWorkspaceChanges` continues to be the only commit point for the `inventory.workspace` section, which remains mutually exclusive with `sections.equipment` in this phase.
12. v2 templates whose `selectedSections` contains anything outside `{profile, stats, inventory.workspace, equipment}` keep the v2 Apply buttons disabled with the standard "Unsupported v2 sections — apply is available only for profile, stats, inventory.workspace, and equipment in this phase." tooltip. `sections.equippedTalismans` is **not** a valid section name — a template that ships it would fail strict YAML decode (`KnownFields(true)`) before reaching the apply layer, by design.

### 17a.3. Scope of what is **not** yet validated

- ✅ **Shipped 2026-05-31 (Phase 5D.1)** — Apply of `sections.profile` and `sections.stats` to a real save via `ApplyBuildTemplateV2FromLibraryToCharacter` (library path, `mode: "append"`, `profile.class` intentionally skipped, snapshot + rollback under `slotMu[charIdx]`).
- ✅ **Shipped 2026-05-31 (Phase 5D.2)** — Direct apply of an imported YAML without first saving it to the library, through `ApplyBuildTemplateV2ToCharacterJSON` on the canonical JSON produced by `PreviewBuildTemplateImportYAMLFromFile` (`Import YAML → Preview → Apply to character`, `mode: "append"`, no second file dialog, no TOCTOU re-read). `ApplyBuildTemplateV2FromFileToCharacter` still exists backend/bindings-side but remains intentionally unwired in UI — the JSON path is preferred because it is WYSIWYG with the preview the user just confirmed.
- ✅ **Shipped 2026-05-31 (Phase 6)** — Apply-time overrides for the same profile/stats subset on both surfaces, via frontend-only mutation of the canonical JSON forwarded to `ApplyBuildTemplateV2ToCharacterJSON`. No backend, no bindings, no `App.tsx` change. `profile.class` stays read-only. v1 templates and unsupported v2 sections remain blocked. Weapon level override at apply time stayed deferred to Phase 6b at the time Phase 6 shipped — and is now shipped, see below.
- ✅ **Shipped 2026-05-31 (Phase 6b)** — Apply-time weapon level override for the existing v1 `inventory.workspace` Apply path. Runtime option `WeaponLevelOverride { Enabled bool, StandardLevel *int, SomberLevel *int }` added to `app_templates.go::ApplyTemplateOptions`. Override runs **after** each template-side `editor.UpdateWeapon` patch via a new `applyWeaponLevelOverride` helper that switches on `editor.EditableItem.MaxUpgrade` (25 = standard / 10 = somber / 0 = unupgradeable / silent skip otherwise), clamps over-cap requests via `editor.ClampUpgrade(req, MaxUpgrade)`, and surfaces `templates.IssueCodeWeaponLevelClamped` ("weapon_level_clamped") or `templates.IssueCodeWeaponUnupgradeable` ("weapon_unupgradeable") on `report.Warnings`. Mutation stays inside the active `InventoryWorkspaceSnapshot`; no bytes written to `slot.Data` from the override path itself; `SaveInventoryWorkspaceChanges` remains the only commit point. UI controls live inside the existing `SortOrderTab.tsx` Templates dropdown; `TemplatesShellModal`, `ApplyOverridesPanel`, `ImportTemplatePreviewModal`, `TemplateLibraryModal`, `App.tsx`, and the v2 apply layer are untouched. No v2 schema change, no equipment writer, no direct save mutation through `PatchWeaponItemID` from the template apply.
- ✅ **Shipped 2026-05-31 (Phase 9)** — `https://` URL import through `PreviewBuildTemplateImportYAMLFromURL`, backed by `backend/templates/url_import.go::FetchYAMLFromURL` under the full §12.3 SSRF guard list. The URL preview reuses the existing `ImportTemplatePreviewModal`, so Save to Library / Apply to character / Apply with overrides all ship unchanged on the URL surface. No library schema change (`sourceURL` metadata is not added); no `App.tsx` change. Authenticated downloads, domain allowlist, URL auto-refresh, and direct apply without preview remain out of scope.
- ✅ **Shipped 2026-05-31 (Phase 7a)** — first real v2 apply path for `inventory.workspace` routed through the **active `InventoryEditSession` / `InventoryWorkspaceSnapshot`**. New backend endpoint `App.GetActiveInventoryEditSessionForCharacter(charIdx) → { active, sessionID }`; additive `ApplyTemplateV2Options.SessionID string`; new issue codes `IssueCodeInventorySessionRequired = "inventory_session_required"` and `IssueCodeInventorySessionInvalid = "inventory_session_invalid"`; extended `ApplyTemplateV2Result` with `InventoryItemsApplied`, `StorageItemsApplied`, optional `Workspace *editor.InventoryWorkspaceSnapshot`. Mixed profile+stats+inventory.workspace apply is atomic via a dual snapshot rollback (`core.SnapshotSlot` + value-type workspace deep copy, restored by a single `rollbackBoth()` closure on every error exit). The v2 inventory apply never touches `slot.Data` directly — `SaveInventoryWorkspaceChanges` remains the only commit point. `TemplatesShellModal` looks up the active session for library / direct YAML / URL / overrides surfaces; profile/stats-only v2 applies still proceed with `sessionID = ''`. Phase 6b weapon level override was intentionally **not** wired into the v2 path in Phase 7a — the v2 inventory apply passed a hard-coded `nil` override into `applyTemplateItemsToWorkspace` — and was wired in Phase 7a.2 below.
- ✅ **Shipped 2026-05-31 (Phase 7a.2)** — apply-time weapon level override threaded into the Phase 7a v2 `inventory.workspace` apply path. `ApplyTemplateV2Options` gains an additive `WeaponLevelOverride *WeaponLevelOverride` field that reuses the v1 `WeaponLevelOverride { Enabled bool, StandardLevel *int, SomberLevel *int }` type and the v1 `validateWeaponLevelOverride` validator verbatim (so the bindings expose a single `WeaponLevelOverride` class shared between v1 and v2 surfaces). The two hard-coded `nil` override pins at the v2 inventory + storage `applyTemplateItemsToWorkspace` call sites in `app_templates_v2_apply.go` are replaced with `opts.WeaponLevelOverride`; the helper itself, `applyWeaponLevelOverride`, and the Phase 7a dual snapshot rollback are unchanged. Validation runs **before** `acquireSession` so a structurally broken override bounces with `templates.IssueCodeStructureInvalid` and zero side effects; a structurally valid override on a profile/stats-only template is silently ignored (no items → no-op). `weapon_level_clamped` and `weapon_unupgradeable` warnings flow into `ApplyTemplateV2Result.Preview.Warnings` through the existing `invWarn` / `stoWarn` aggregation. UI: a new `WeaponLevelOverridePanel` (testids `apply-overrides-weapon-{panel,enabled,standard,somber,error}`) is embedded inside the existing `ApplyOverridesModal` and rendered only when `selection.inventory.workspace` is present; profile/stats-only templates leave the weapon panel unrendered; inventory-only templates may use the modal exclusively for weapon level override. `ApplyOverridesModal.onConfirm` is extended to `(mutatedJSON, weaponOverride?) ⇒ …` and the runtime option travels exclusively through that argument — never inside the canonical JSON. `TemplatesShellModal.handleConfirmOverrides` forwards `weaponLevelOverride` to `main.ApplyTemplateV2Options.createFrom({ mode, sessionID, weaponLevelOverride })`. The fast library Apply (`ApplyBuildTemplateV2FromLibraryToCharacter`) continues to send no override; the v1 `SortOrderTab.tsx` Phase 6b dropdown is untouched. No new v2 schema section, no equipment writer, no direct save mutation, no `App.tsx` change.
- ✅ **Shipped 2026-05-31 (Phase 7b.0)** — backend-only `(s *core.SaveSlot).WriteEquipment([]EquipmentWrite) error` foundation for ChrAsmEquipment slots 0–9 + 12–15 (weapons + ammo + armor). Strict GaMap-present-only handle resolution, validate-then-mutate atomicity, targeted hash 7 / 8 recompute, encoded-form output (`itemID | 0x80000000` for weapons / armor; raw goods `itemID` for ammo), explicit-clear via `Handle == 0` → `0xFFFFFFFF`, invalid-sentinel rejection of `Handle == 0xFFFFFFFF`, class-gate rejection of AoW (`0xC0`) handles in weapon slots, talismans / GreatRune (slot 10) / unknown slots 11/16 / spells / quick / pouch items intentionally unexposed by the enum. No App method, no Wails binding, no template schema change, no frontend, no template apply integration — Phase 7b.1 wires the foundation into the template apply path.
- ✅ **Shipped 2026-06-01 (Phase 7c)** — v2 talisman apply by extending the Phase 7b.1 `sections.equipment` schema with `talisman1..5` (ChrAsmEquipment indices 17–21, hash 8). Intentionally **not** a separate `sections.equippedTalismans` section: talismans live in the same `ChrAsmEquipment` struct as weapons/ammo/armor and reuse the Phase 7b.1 resolver, combo guard, preview row, frontend gate, and rollback. `backend/core/equipment_writer.go` extends with `EquipSlotTalisman1..5`, a fourth class `slotClassTalisman` (accepts only the `ItemTypeAccessory` 0xA0 handle prefix and rejects 0x80 / 0x90 / 0xB0 / 0xC0 with class-specific errors), the five slot-table entries, and a hash 8 recompute path widened to cover indices 17–21; the encoded slot value is `GaMap[handle]` directly with no `| 0x80000000` mask (talisman handles carry the 0x20 prefix in GaMap, mirroring how ammo encodes goods). Apply layer adds `MaxActiveTalismanSlots = 4`, `talismanSlotOrdinal(slotKey)`, and `computeActiveTalismanSlots(slot, tpl)` = `1 + effective profile.talismanSlots` (clamped to `MaxProfileTalismanSlots = 3`); the resolver gates non-empty talisman refs against this active capacity and emits the new `templates.IssueCodeTalismanSlotPouchInsufficient = "talisman_slot_pouch_insufficient"` warning + skip when the ordinal exceeds the cap. Talisman5 always trips the warning when populated with `baseItemID > 0` (vanilla cap = 4 active slots); `talisman5: {baseItemID: 0}` clear is always allowed and writes `0xFFFFFFFF`. Mixed templates that also select `profile.talismanSlots` use the template value so a +3 pouch bump unblocks `talisman4` in the same apply. Resolver still searches only `slot.Inventory.CommonItems` (no storage), missing talisman = `equipment_item_not_in_inventory` warn + skip, ambiguous = first-wins warning, no auto-add. The `equipment + inventory.workspace` hard reject is unchanged (talismans are inside `sections.equipment`). Frontend has **no source-code change** — `V2_APPLY_SUPPORTED_SECTIONS` already includes `equipment`, the existing `import-preview-equipment-slots` row enumerates whichever slot keys the summary lists (talismans included), and the Library / direct-YAML / URL apply paths reuse the canonical JSON pipeline. `frontend/wailsjs/go/models.ts` shows no diff (talisman extensions are JSON-payload fields on `EquipmentSection`, exchanged as opaque template JSON rather than a typed Wails model); `App.d.ts` / `App.js` untouched. No `App.tsx` / `SortOrderTab.tsx` / `ApplyOverridesPanel.tsx` / `WeaponLevelOverridePanel.tsx` change.
- ✅ **Shipped 2026-06-01 (Phase 7b.1)** — v2 `sections.equipment` end-to-end through the Phase 7b.0 writer. Schema: 14 optional slot pointers (weapons LH/RH 1/2/3, arrows1/2, bolts1/2, armor head/chest/arms/legs) carrying `EquipmentItemRef { BaseItemID; Name; Upgrade *int; InfusionName; AoWItemID *uint32 }`; `BaseItemID == 0` is the explicit-clear sentinel; omitted slots are no-ops; omitted `Upgrade` matches any level. Export builder + app-layer scanner reads the 14 supported `ChrAsmEquipment` slots, decodes the encoded form, and matches against `editor.EditableItem.ItemID` with DB / raw-decoded-ID fallback. Apply pipeline routes through `ApplyBuildTemplateV2ToCharacterJSON`: scope detection grows a `hasEquipment` flag; the combo `sections.equipment + sections.inventory.workspace` is hard-rejected (defence in depth — preview and apply both) with `equipment_inventory_combo_unsupported`; equipment-only templates do NOT require an Inventory Edit Session. The resolver walks `slot.Inventory.CommonItems` only (storage not searched), matching by `BaseItemID` plus optional `Upgrade` / `InfusionName` / `AoWItemID` disambiguators; missing items emit `equipment_item_not_in_inventory` warnings + skip; ambiguous matches emit `equipment_item_ambiguous` + first-match-wins; `SaveSlot.WriteEquipment` is dispatched after profile/stats `SyncPlayerToData` inside the existing slot snapshot scope, so any writer failure triggers a clean `rollbackBoth()`. New `ApplyTemplateV2Result.EquipmentSlotsApplied int`; new `ImportPreviewSummary.EquipmentSlotsPresent []string`; four new issue codes. UI: `ImportTemplatePreviewModal.tsx` adds `equipment` to `V2_APPLY_SUPPORTED_SECTIONS`, renders an `import-preview-equipment-slots` row, and tightens the unsupported-section copy; `TemplateLibraryModal.tsx` enables Apply for equipment-only entries; `TemplatesShellModal.tsx` is unchanged (existing `needsSession` gate already excludes equipment-only templates); no equipment override panel in `ApplyOverridesPanel.tsx` / `WeaponLevelOverridePanel.tsx`. `App.tsx`, `SortOrderTab.tsx` untouched. `frontend/wailsjs/go/models.ts` regenerated.
- 14-slot EquippedSpells loadout — gated by Phase 7d (new writer).
- Lifting the `equipment + inventory.workspace` combo restriction — optional future Phase 7b.2 (requires either auto-commit of the workspace or a workspace-backed equipment model).
- EquippedGreatRune (slot 10) through the template path — out of scope across the Phase 7 family; the existing `SyncPlayerToData` path remains the only writer.
- Appearance preset apply through the template surface — gated by Phase 8 (the underlying `app_appearance.go::ApplyPresetToCharacter` writer already exists, but no apply layer routes the template through it yet).
- Multi-character pack flow — gated by Phase 10.

Phase 7c+ work remains design-only in this document. Each phase requires a separate user approval before implementation per the workflow in `~/.claude/CLAUDE.md`.

---

## 18. Open decisions intentionally deferred

The following decisions are intentionally not resolved by this document. Each requires a separate, explicit user approval before the corresponding phase begins.

1. **YAML library choice** (likely `gopkg.in/yaml.v3` decoded strictly into typed structs).
2. **Source-of-truth strategy across JSON + YAML** (single Go struct with both `json:` and `yaml:` tags vs separate DTOs).
3. **`sessionID` plumbing for the sidebar surface** (lift to App.tsx, lighter-weight session-less library modal, or context).
4. **Final list of v2 section keys and their canonical names** (e.g. `sections.profile` vs `sections.character.profile`).
5. ~~**Exact body-size cap for URL import**~~ — **decided 2026-05-31 at Phase 9 implementation: 1 MiB (`1 << 20`)**, exported as `templates.URLImportMaxBodyBytes`.
6. ~~**Exact request/idle timeouts for URL import**~~ — **decided 2026-05-31 at Phase 9 implementation: total 10 s, idle / TLS / header / dial 5 s each**, exported as `templates.URLImportTotalTimeout` and `templates.URLImportIdleTimeout`.
7. **`selection` granularity for per-spell / per-talisman lists** (boolean shortcut vs explicit list).
8. **Equipment referential integrity default policy** (warn-and-skip vs opt-in auto-add).
9. **Talisman slot count clamping policy** (refuse if Pouch upgrade insufficient vs clamp + warning).
10. **Disposition of the existing `Export Template ▾` dropdown** after the sidebar surface ships (retain / remove / redirect).
11. **`replace-*` modes for v2** — out of scope for first iteration; whether to ship in a later phase is a separate decision.
12. **Auto-rebuild of `_index.bak.json` snapshot** before `RebuildIndex` — optional later hardening.
13. **PvP / progression named modules in templates** — whether to ever ship and which modules to include.
14. **Multi-character pack mapping UI conventions** — full design deferred to its own phase.
15. **Whether v2-only fields require an `appVersion` minimum gate** (e.g. tag a section with the minimum app version that supports it).
16. **PS4 ↔ PC parity test policy** for new apply phases (proposed: every code-touching phase must keep both round-trip tests green).

---

## 19. Cross-references

- [55-build-template](55-build-template.md) — implemented baseline (v1 schema, exporter, preview, apply, library).
- [54-ash-of-war](54-ash-of-war.md) — AoW sentinels, fail-closed compat, write paths.
- [37-character-presets](37-character-presets.md) — a separate, character-stat-focused mechanism; not the same as Templates.
- [03-gaitem-map](03-gaitem-map.md) — GaItem model; handle semantics excluded from public YAML.
- [06-equipment](06-equipment.md) — equipment slot model; read-only Equipment write API today.
- [07-inventory](07-inventory.md), [10-storage](10-storage.md) — inventory / storage section model.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator invariants relevant to apply.
- [43-transactional-item-adding](43-transactional-item-adding.md) — Add Items pipeline, pre-flight + snapshot + post-mutation validation pattern reused by `TemplateApplyPlan`.
- [50-item-companion-flags](50-item-companion-flags.md) — implicit POST-FLAGS contract preserved by apply.
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — why absolute acquisition indices stay out of templates.
- [48-pvp-ready-modular-presets](48-pvp-ready-modular-presets.md) — named-module pattern for any future progression effects in templates.

---

## 20. Sources

- Existing baseline doc: `spec/55-build-template.md`.
- Existing code (informational, no change in this turn): `backend/templates/{schema,export,import,library}.go`, `app_templates.go`, `frontend/src/components/templates/`, `frontend/src/components/SortOrderTab.tsx`, `frontend/src/App.tsx`.
- Apply-side dependencies (informational): `backend/editor/{session,workspace,add,weapon,save}.go`, `backend/core/{inventory_index_repair,save_manager,writer,backup}.go`, `app_save_integrity.go`, `app.go::SaveCharacter`, `app_appearance.go::ApplyPresetToCharacter`, `app_pvp.go::ApplyPvPPreparation`.
- DB references (informational): `backend/db/db.go::{GetItemDataFuzzy,InfuseTypes,IsAshOfWarCompatibleWithWeapon}`, `backend/db/data/{types,weapon_gem_mount,aow_compat}.go`.
- Workflow constraints: `~/.claude/CLAUDE.md` (global), `.claude/CLAUDE.md` (project).
