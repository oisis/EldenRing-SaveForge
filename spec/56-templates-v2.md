# 56 — Templates v2 (Partially Implemented Extension)

> **Type**: Design doc
> **Status**: 🔄 Partially implemented — Phase 0..6 shipped (additive `version: 2` schema, global Templates library shell, public YAML import/export, create-from-character flow for profile/stats with per-field selection, Save to Library, v2 metadata badge in library and preview, **Phase 5 = library Apply + direct imported-YAML Apply for profile/stats via `ApplyBuildTemplateV2FromLibraryToCharacter` and `ApplyBuildTemplateV2ToCharacterJSON` on the canonical JSON produced by the import preview, Phase 6 = apply-time overrides for the same profile/stats subset on both surfaces via frontend-only canonical-JSON mutation forwarded to the same JSON endpoint**). Apply for v2 sections outside profile/stats remains blocked — Phase 6b+ (weapon level override, equipment/talismans/spells writers, appearance via preset, URL import, multi-character pack) remain design-only. Additive extension of the implemented Build Template subsystem documented in [55-build-template](55-build-template.md).
> **Scope**: Addytywne rozszerzenie istniejącego `saveforge.build-template` JSON v1 do `version: 2` — z publicznym formatem YAML do udostępniania na zewnątrz, nowym sidebar entry point `Templates`, granular selection model, sekcjami całej postaci (profile, stats, equipment, talismans, spells, appearance via preset), single-character first, weapon level override przy apply, plików `.yaml` import/export, importu z URL z pełnymi guardami bezpieczeństwa oraz późniejszą fazą multi-character pack. Document **does not** redefine the v1 baseline — it inherits it from [55-build-template](55-build-template.md).

---

## 1. Title, status and scope

| Aspect | Value |
|---|---|
| Document number | 56 |
| Document type | Design doc — partially implemented extension |
| Status | 🔄 Partially implemented. Phase 0..6 shipped (Phase 5 = library Apply + direct imported-YAML Apply for the profile/stats subset only; Phase 6 = apply-time overrides for the same profile/stats subset, frontend-only mutation of the canonical JSON forwarded to the existing Phase 5 endpoint); Phase 6b+ remain design-only. Each later phase requires a separate user approval per the workflow in `~/.claude/CLAUDE.md`. |
| Baseline reference | [55-build-template](55-build-template.md) — implemented `version: 1`, JSON only, inventory + storage only, local library at `$UserConfigDir/EldenRing-SaveEditor/templates/`. |
| Schema key | Remains `saveforge.build-template` (no rename). Implemented. |
| Schema version | Reader range `1 ≤ version ≤ MaxSchemaVersion (=2)`. v1 builder still emits `SchemaVersion = 1`; the explicit v2 builder (`backend/templates/export_v2.go`) emits `version: 2`. Implemented. |
| External public format | YAML (`.yaml`). JSON remains for the existing local library and for backward-compatible import. Implemented for v1 payloads and for v2 documents produced by the v2 builder. |
| First user-visible entry | Sidebar blue `Templates` button immediately above `Save as...` in `frontend/src/App.tsx` (existing `<aside>` footer block); opens `TemplatesShellModal.tsx`. Implemented. |
| Character scope (first iteration) | Single character. Multi-character pack is deferred to a later phase (§15). |
| URL import | Deferred phase. Backend-only fetch with strict guards (§12). Not implemented. |
| Production code change | Phase 0..6 shipped; later phases remain design-only. Detail in §17 and §17a. |

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
| `equipment` (equipped slots: `weaponRight1/2`, `weaponLeft1/2`, `armorHead/Chest/Arms/Legs`, plus optional `equippedGreatRune`) | later phase (Phase 7a) | slot name → item ID | **No public write API today** for `ChrAsmEquipment` slots 0..9, 12–15, 17–21 — [06-equipment §App-level write API](06-equipment.md) is explicit ("None — equipment is read-only from the UI perspective"). The only existing exception is `EquippedGreatRune` (slot 10), already written by `SyncPlayerToData` at `structures.go:850–852`. **A new controlled writer is required** for the remaining slots (encoded item-ID form, hash 7/8 dependency — see [06-equipment §hash](06-equipment.md)). | requires-new-writer | **no** (except GreatRune) |
| `equippedTalismans` (which talismans occupy `ChrAsmEquipment` slots 17–21) | later phase (Phase 7b) | array of up to 5 talisman item IDs in slot order | **No public write API today** — equipped talismans live in the same `ChrAsmEquipment` block as armor; they are read-only with the rest of equipment. **A new controlled writer is required** (companion to Phase 7a) and must respect the Pouch limit from `profile.talismanSlots`. Distinct from `profile.talismanSlots` (the additional slot count, which already has a writer). | requires-new-writer | no |
| `spells` (equipped sorcery / incantation / gesture loadout — 14 spell slots) | later phase (Phase 7c) | spell / sorcery / incantation / gesture item IDs | **No public write API today.** `EquippedSpells` (14 slots) is currently only referenced by hash-recompute (`backend/core/hash.go:150–195`, `section_hash.go:24`). **A new controlled writer is required.** Distinct from the unlocked-spell inventory entries (which are part of `inventoryWorkspace` and already supported by v1). | requires-new-writer | no |
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

URL import is a **later phase**, separately approved. Nothing about URL import is in scope for the first Templates v2 implementation phase. This section captures the required design constraints so that when the phase is approved, the implementation is bounded by them.

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
- **Body size cap**: hard `io.LimitReader` (planned: 1 MB; final value confirmed at implementation).
- **Timeouts**: request timeout ≤ 10 s; idle timeout ≤ 5 s.
- **TLS**: system root CAs only, **no** `InsecureSkipVerify`, no custom CA injection from URL.
- **User-Agent**: a stable, identifying string set by the backend (final value decided at implementation).
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
- Helper location: ✅ Done — the pure clamp helper now lives in `backend/editor/weapon.go` as the exported `editor.ClampUpgrade` (relocated from the old `app.go::clampUpgrade`, behaviour byte-for-byte unchanged). The planned `TemplateApplyPlan` layer under `backend/templates/` or `backend/editor/` can import it directly without pulling in the `main` package. This was a pure relocation — no policy change, no new clamp rule — and unblocks the Phase 6b weapon-level override and any future inventory-apply pipeline from a backend-importable position. Weapon-level override at apply time is **not** yet implemented — only the import path is unblocked.

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
| Equipment slot write path (`ChrAsmEquipment` slots 0..9, 12–15, 17–21) | requires design decision + new writer | No existing public write API ([06-equipment](06-equipment.md) "App-level write API for equipment slots | ❌ None"). New controlled writer required for Phase 7a; respects hash 7/8 dependency. |
| Equipped talismans write path (`ChrAsmEquipment` slots 17–21) | requires design decision + new writer | Same as equipment; companion to Phase 7a, scheduled as Phase 7b. Must respect `profile.talismanSlots` Pouch limit. |
| Equipped spell loadout write path (`EquippedSpells` 14 slots) | requires design decision + new writer | No existing public write API; only hash recompute references the field today. Phase 7c. |
| Equipment referential integrity (template references item not in target inventory) | requires design decision | Default = warn + skip; opt-in `addMissingEquippedItems` deferred (§13.7). Applies to Phase 7a/7b. |
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

### Phase 6b — weapon level override for the v1 inventory / storage apply

- **Goal**: add `weaponLevelOverride.{standard,somber}` to the apply options and the Apply Preview UI; pre-encode item IDs in the plan layer for weapons coming from the template.
- **Files (planned scope)**: `app_templates.go` (options DTO), apply layer; frontend preview modal. **Prerequisite refactor done**: `clampUpgrade` has been relocated to `backend/editor/weapon.go` as the exported `editor.ClampUpgrade` (byte-for-byte behaviour preserved, see §14.4); the plan layer can import it directly. Weapon-level override at apply time itself is still design-only — the relocation only unblocks the import path.
- **Backend impact**: plan-layer override + helper relocation; existing item-add writer unchanged.
- **Frontend impact**: two new controls in the preview modal, both default `Keep`.
- **Tests**: `Set Standard to +25` against a mixed template → somber weapons clamped to `+10` with `upgrade_clamped_by_override` warning; `Keep` preserves template levels; non-upgradeable weapons remain `+0`; round-trip both platforms.
- **Manual validation**: apply a template with mixed weapons under each control combination.
- **Risks**: low if clamping is per-weapon and reuses the relocated helpers.
- **Out of scope**: changing infusion or AoW; equipped-weapon writers.
- **Requires separate user decision before continuing**: yes.

### Phase 7a — equipment slot writer (new write path)

- **Goal**: implement a new public write path for `ChrAsmEquipment` slots 0..9, 12–15 (weapons + armor), respecting the encoded item-ID form and the hash 7/8 dependency documented in [06-equipment](06-equipment.md). Apply `sections.equipment` from the template through this new writer.
- **Files (planned scope)**: new writer in `backend/core/` (likely `backend/core/equip_write.go`), exposed via `backend/editor/` to the plan; apply layer extension; tests including hex-verified round-trip; frontend preview row for equipment.
- **Backend impact**: new public API for equipment writes. The single existing exception `EquippedGreatRune` (already in `SyncPlayerToData`) is preserved.
- **Tests**: hex-identity round-trip for no-op write; per-slot write; PC + PS4 platform parity; integrity gate interaction; default warn-and-skip when a referenced item is missing from inventory (§13.7).
- **Manual validation**: apply equipment to a fixture character; round-trip both platforms; in-game verification on at least one platform.
- **Risks**: high — first new write path into `ChrAsmEquipment`; touches a section all reference editors treat as read-only; hash 7/8 dependency must be re-validated.
- **Out of scope**: equipped talismans, spell loadout, appearance.
- **Requires separate user decision before continuing**: yes.

### Phase 7b — equipped talismans writer (new write path)

- **Goal**: implement the equipped-talismans writer (`ChrAsmEquipment` slots 17–21). Apply `sections.equippedTalismans` clamped to the target's current `profile.talismanSlots` (refuse if length exceeds `1 + slotCount`, or warn + truncate per §18 #9 decision).
- **Files (planned scope)**: extension of the equipment writer from Phase 7a, or a sibling writer; apply layer extension; tests; frontend preview row.
- **Backend impact**: extends the new public equipment write API to talisman slots.
- **Tests**: respects Pouch limit; rejects overflow per chosen policy; hex round-trip; PC + PS4 parity.
- **Manual validation**: apply equipped talismans; round-trip both platforms.
- **Risks**: medium — relies on Phase 7a infrastructure.
- **Out of scope**: spell loadout; appearance.
- **Requires separate user decision before continuing**: yes.

### Phase 7c — spell loadout writer (new write path)

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

### Phase 9 — URL import (separately approved, full guards)

- **Goal**: implement URL import per §12 with all guards (https-only, IP filter, redirect re-check, body size, timeouts, strict TLS, struct-typed YAML, no includes, no executable types, preview before library, separate confirm before apply).
- **Files (planned scope)**: `backend/templates/url_import.go` (new), `app_templates_url.go` (new), strict tests per guard.
- **Backend impact**: new backend fetch; no other change.
- **Frontend impact**: new dialog for URL paste; clear sourceURL display on preview.
- **Tests**: each guard with an explicit test (`https`-only, loopback rejection, private rejection, redirect re-check, body size, timeout, content-type, parse strictness). SSRF unit tests are mandatory.
- **Manual validation**: a controlled fixture HTTPS endpoint.
- **Risks**: SSRF — gated by §12.3.
- **Out of scope**: authenticated fetches; auto-refresh; multi-character pack.
- **Requires separate user decision before continuing**: yes.

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

### 17a.3. Scope of what is **not** yet validated

- ✅ **Shipped 2026-05-31 (Phase 5D.1)** — Apply of `sections.profile` and `sections.stats` to a real save via `ApplyBuildTemplateV2FromLibraryToCharacter` (library path, `mode: "append"`, `profile.class` intentionally skipped, snapshot + rollback under `slotMu[charIdx]`).
- ✅ **Shipped 2026-05-31 (Phase 5D.2)** — Direct apply of an imported YAML without first saving it to the library, through `ApplyBuildTemplateV2ToCharacterJSON` on the canonical JSON produced by `PreviewBuildTemplateImportYAMLFromFile` (`Import YAML → Preview → Apply to character`, `mode: "append"`, no second file dialog, no TOCTOU re-read). `ApplyBuildTemplateV2FromFileToCharacter` still exists backend/bindings-side but remains intentionally unwired in UI — the JSON path is preferred because it is WYSIWYG with the preview the user just confirmed.
- ✅ **Shipped 2026-05-31 (Phase 6)** — Apply-time overrides for the same profile/stats subset on both surfaces, via frontend-only mutation of the canonical JSON forwarded to `ApplyBuildTemplateV2ToCharacterJSON`. No backend, no bindings, no `App.tsx` change. `profile.class` stays read-only. v1 templates and unsupported v2 sections remain blocked. Weapon level override at apply time stays deferred to Phase 6b.
- Weapon level override at apply time — gated by Phase 6.
- New write paths for `sections.equipment`, `sections.equippedTalismans`, `sections.spells` — gated by Phase 7a / 7b / 7c.
- Appearance preset apply through the template surface — gated by Phase 8 (the underlying `app_appearance.go::ApplyPresetToCharacter` writer already exists, but no apply layer routes the template through it yet).
- URL import — gated by Phase 9 (no `https://` fetch surface exists in the backend for templates today).
- Multi-character pack flow — gated by Phase 10.

Phase 6b+ work remains design-only in this document. Each phase requires a separate user approval before implementation per the workflow in `~/.claude/CLAUDE.md`.

---

## 18. Open decisions intentionally deferred

The following decisions are intentionally not resolved by this document. Each requires a separate, explicit user approval before the corresponding phase begins.

1. **YAML library choice** (likely `gopkg.in/yaml.v3` decoded strictly into typed structs).
2. **Source-of-truth strategy across JSON + YAML** (single Go struct with both `json:` and `yaml:` tags vs separate DTOs).
3. **`sessionID` plumbing for the sidebar surface** (lift to App.tsx, lighter-weight session-less library modal, or context).
4. **Final list of v2 section keys and their canonical names** (e.g. `sections.profile` vs `sections.character.profile`).
5. **Exact body-size cap for URL import** (proposed: 1 MB).
6. **Exact request/idle timeouts for URL import** (proposed: 10 s / 5 s).
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
