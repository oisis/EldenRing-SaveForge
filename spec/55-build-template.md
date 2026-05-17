# 55 — Build Template

> **Type**: Design doc
> **Status**: ✅ Implemented — Phase A + B + C + D + E (local library) shipped (`backend/templates/`, `app_templates.go`, `TemplateLibraryModal`).
> **Scope**: Portable JSON representation of an Inventory Workspace snapshot. Defines the `saveforge.build-template` schema v1, the export contract (`BuildTemplateFromSnapshot`), Ash of War handling rules, and what is deliberately excluded from the payload so a template can be applied to any save without colliding with its handle space.

---

## 1. Purpose

Players want to bootstrap a new character with a known loadout — same weapons, upgrade levels, infusions, Ashes of War, sort order — without re-adding everything by hand. A "build template" is the portable, source-of-truth representation of that loadout.

The template captures **only the game-content identifiers** that survive across saves. It deliberately excludes everything that ties data to a specific save: GaItem handles, session UIDs, acquisition indices, GaItem map flags. This makes templates safe to share between users, between platforms, and between characters within the same save.

This document covers the v1 schema, the exporter contract (Phase A), and the placeholder design for import (Phase D/E). Settings export — UI preferences, theme, deploy targets — is a separate feature with its own schema and is out of scope here.

---

## 2. Schema header

```json
{
  "schema": "saveforge.build-template",
  "version": 1,
  "createdAt": "2026-05-17T12:34:56Z",
  "appVersion": "0.15.0-beta",
  "metadata": { ... },
  "sections": { ... }
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `schema` | string | ✅ | Must equal `"saveforge.build-template"`. Importer rejects any other value. |
| `version` | int | ✅ | Schema version. v1 only in Phase A. Importer accepts `1 ≤ v ≤ SchemaVersion`. |
| `createdAt` | string | ✅ | RFC 3339 UTC timestamp. Informational. |
| `appVersion` | string | optional | SaveForge version that produced the template. Informational. |
| `metadata` | object | optional | User-facing labels (name, description, tags, source character). No load-bearing fields. |
| `sections` | object | ✅ | Section payloads keyed by stable section identifier. |

Forward-compatible behavior: unknown fields under `metadata` are tolerated by Go's `encoding/json` and ignored. Unknown section keys are silently dropped by the v1 parser — a v2 reader will pick them up.

---

## 3. Section: `inventory.workspace`

The Phase A payload. Mirrors the subset of `editor.InventoryWorkspaceSnapshot` that is portable.

```json
"sections": {
  "inventory.workspace": {
    "inventoryItems": [ TemplateItem, ... ],
    "storageItems":   [ TemplateItem, ... ]
  }
}
```

Both arrays must be present (possibly empty), but `ValidateBuildTemplate` rejects the template if both are empty — an empty template has no use.

### 3.1. `TemplateItem`

```json
{
  "baseItemID":   4030000,
  "name":         "Greatsword",
  "category":     "melee_armaments",
  "quantity":     1,
  "upgrade":      25,
  "infusionName": "Heavy",
  "aowItemID":    2168029136,
  "container":    "inventory",
  "position":     0
}
```

| Field | Type | Required | Notes |
|---|---|---|---|
| `baseItemID` | uint32 | ✅ | DB base ID (no upgrade/infusion encoding). Source of truth at import. **Must not be 0.** |
| `name` | string | optional | Debug/display only. Not used at import. |
| `category` | string | optional | Debug/display only. |
| `quantity` | uint32 | ✅ | Stack size. **Must not be 0.** |
| `upgrade` | int | optional | Weapon upgrade level (0–25 / 0–10 depending on weapon). Default 0. |
| `infusionName` | string | optional | One of `db.InfuseTypes` names (`Heavy`, `Keen`, `Quality`, …). Absent or `""` means "Standard". |
| `aowItemID` | uint32 (pointer) | optional | Custom Ash of War gem item ID. Absent if no custom AoW. **Never `0`.** |
| `container` | string | ✅ | `"inventory"` or `"storage"`. Must match the parent array. |
| `position` | int | ✅ | Stable sort position within the container, 0-indexed. |

### 3.2. AoW encoding

`aowItemID` is a pointer + `omitempty` so the JSON contract is:

| Source `CurrentAoWStatus` | JSON output |
|---|---|
| `"custom"` with `CurrentAoWItemID != 0` | `"aowItemID": <id>` |
| `"none"` (no custom AoW, sentinel handle) | field omitted |
| `"missing"` (dangling handle) | field omitted + `aow_missing_skipped` warning |
| `"shared"` (handle referenced by multiple weapons) | field omitted + `aow_shared_skipped` warning |
| empty status (non-weapon) | field omitted |

The exporter **never** writes `aowItemID: 0` and **never** writes the in-save no-custom sentinel handle (`0x00000000` / `0xFFFFFFFF`). Both would leak save-local addressing into the template.

---

## 4. Excluded fields

These fields exist on `editor.EditableItem` but are deliberately **not** emitted by the exporter. They are session- or save-local and would tie the template to one save's handle space:

- `originalHandle` — GaItem handle in the source save.
- `currentAoWHandle` — GaItem handle of the source AoW.
- `uid` — workspace session-scoped UUID.
- `acquisitionIndex` — per-character chronology counter; will be re-issued on import.
- `pendingAoWItemID`, `pendingAoWName`, `pendingAoWClear`, `hasPendingWeaponPatch` — unsaved RAM-only edits; the exporter mirrors *current saved* state, not pending requests.
- `hasGaItem`, `hasCurrentAoW`, `currentAoWShared`, `currentAoWStatus` — derived flags; recomputed at import from the destination save's DB.
- `isWeapon`, `isArmor`, `isTalisman` — derived from DB lookup at import; would only drift.
- `maxUpgrade` — DB constant; never authoritative in a template.
- `iconPath` — cosmetic; localised resource paths.

Pass-through records (`UnsupportedInventoryRecords`, `UnsupportedStorageRecords`) are also excluded from Phase A. They describe items outside the Phase 1 editable allow-list and would not survive an `AddInventoryWorkspaceItem`-based import.

A regression test in `backend/templates/schema_test.go` (`TestSchemaJSON_OmitsForbiddenFields`) marshals a fully-populated template and grep-asserts that none of these field names appear in the JSON.

---

## 5. Exporter contract (Phase A)

```go
func BuildTemplateFromSnapshot(
    snap editor.InventoryWorkspaceSnapshot,
    opts ExportOptions,
) (*BuildTemplate, *ExportReport, error)
```

Behavior:

- `opts.IncludeInventory` and `opts.IncludeStorage` independently gate the two arrays. At least one must be true.
- Items are stable-sorted by `EditableItem.Position`; the resulting array index becomes the template's `position`. Divergence emits one `position_normalized` warning per affected item.
- `BaseItemID == 0` and `Quantity == 0` are exporter errors (return `error`, no template produced).
- `Source: added` and `Source: original` items are exported identically — the template is shape-only.
- `opts.Now` is exposed for tests; production callers leave it zero and the exporter uses `time.Now().UTC()`.

`ExportReport.Warnings[]` is non-empty when AoW state was dropped or positions renormalised. Each entry carries `{code, uid, container, position, message}`. The Phase B UI surfaces these warnings before writing the file.

### 5.1. Warning codes

| Code | Meaning |
|---|---|
| `aow_missing_skipped` | Weapon's source AoW handle did not resolve in `slot.GaMap`. AoW not exported. |
| `aow_shared_skipped` | Weapon's source AoW handle was referenced by ≥2 weapons (save corruption). AoW not exported. |
| `position_normalized` | An item's reported `Position` did not match its final array index after stable sort. |

These strings are stable; importer UIs and tests are expected to assert on them.

---

## 6. Validator contract

```go
func ValidateBuildTemplate(tpl *BuildTemplate) error
```

Phase A scope: structural and invariant checks only. **No DB lookups.** Validates:

- `tpl != nil`
- `tpl.Schema == "saveforge.build-template"`
- `0 < tpl.Version ≤ SchemaVersion`
- `tpl.Sections.InventoryWorkspace != nil`
- At least one of `inventoryItems` / `storageItems` is non-empty
- Per item: `BaseItemID != 0`, `Quantity != 0`, `Container` matches the parent array

DB-level checks (item exists, AoW compatible with weapon type, quantity within `MaxInventory*(NG+1)` cap) belong to the import path (Phase D) where the destination save's context is available.

---

## 7. Phase roadmap

| Phase | Scope | Status |
|---|---|---|
| **A** | Schema + exporter (`backend/templates/`), spec doc, tests | ✅ |
| **B** | Wails bindings `ExportBuildTemplateJSON` / `ExportBuildTemplateToFile`, SortOrderTab dropdown + modal | ✅ |
| **C** | Import preview: `ParseBuildTemplateJSON`, `PreviewBuildTemplateImport`, App endpoints, `ImportTemplatePreviewModal` (read-only) | ✅ |
| **D** | Apply to workspace: `ApplyBuildTemplateToWorkspaceJSON/FromFile`, capacity preflight, RAM-only mutation with rollback, Apply button in preview modal | ✅ |
| **E** | Local template library under `UserConfigDir`/`templates/` | 🔲 pending |
| 2+ | `character.profile` section (level, stats, talisman slots), opt-in via `$enabled` | 🔲 deferred |

### 7.1. Phase B endpoint contract

Two `App`-receiver methods exposed via Wails:

```go
func (a *App) ExportBuildTemplateJSON(sessionID string, opts BuildTemplateExportOptions) (BuildTemplateExportResult, error)
func (a *App) ExportBuildTemplateToFile(sessionID string, opts BuildTemplateExportOptions) (BuildTemplateExportResult, error)
```

Behavior shared by both:

- Resolve the active `InventoryEditSession` by `sessionID`. Unknown session → error `inventory edit session %q not found`.
- Build `templates.ExportOptions` from `opts`, stamp `AppVersion` from the package's `appVersion` variable, populate `Metadata.SourceCharacterIndex` from the session and `Metadata.SourceCharacterName` from the in-save character name (empty if no save is loaded).
- Run `templates.BuildTemplateFromSnapshot` then `ValidateBuildTemplate`. Validation failures surface as wrapped errors.
- **Never** call `SaveInventoryWorkspaceChanges`, **never** mutate slot data, **never** allocate handles. Export is a pure read of the workspace snapshot.
- A workspace with `Dirty=true` is a valid export source — the feature exists precisely so the user can capture a build before pressing Save Changes.

`ExportBuildTemplateJSON` returns the JSON payload as a string (the `JSON` field on `BuildTemplateExportResult`) and skips file I/O entirely. Useful for previews and tests.

`ExportBuildTemplateToFile` shows a native save-file dialog via `runtime.SaveFileDialog`, writes the file with mode 0644, and returns the chosen path. Cancellation (empty path from the dialog) is **not** an error: the result is returned with `Path == ""` and the frontend stays silent. The default filename is derived from `metadata.name`, falling back to `<character>-build.json`, then `saveforge-build-template.json`.

Warnings from `templates.ExportReport` are passed through on `BuildTemplateExportResult.Warnings` so the UI can surface them after a successful write without mutating workspace state.

Settings export (theme, deploy targets, UI preferences) is **not** part of this design. It will be a separate schema `saveforge.settings-export` if/when it becomes a priority.

---

## 7.2. Phase C endpoint contract (Import Preview)

Two `App`-receiver methods exposed via Wails:

```go
func (a *App) PreviewBuildTemplateImportJSON(jsonText string) (templates.ImportPreviewReport, error)
func (a *App) PreviewBuildTemplateImportFromFile() (templates.ImportPreviewReport, error)
```

Behavior shared by both:

- **Dry-run only.** No session is resolved, no workspace mutated, no save written, no handles allocated. `PreviewBuildTemplateImport` is in a package (`backend/templates`) that does not import `backend/editor` or `backend/core` — the absence of those imports is the structural guarantee.
- Parse the JSON via `templates.ParseBuildTemplateJSON`, which wraps `json.Unmarshal` + `ValidateBuildTemplate`. Malformed payloads or schema mismatches produce a NON-OK report with a single `structure_invalid` error, not a Go `error` — this lets the frontend render parse and per-item failures through the same panel.
- Per-item validation runs against the live DB and produces `ImportPreviewIssue` entries with `Code`, `Container`, `Position`, and item IDs so the UI can deep-link.

### Validation rules

| Code | Severity | Trigger |
|---|---|---|
| `structure_invalid` | error | JSON parse / `ValidateBuildTemplate` failed at the schema layer |
| `schema_invalid` | error | `ValidateBuildTemplate` rejected a non-nil template after construction |
| `unknown_item` | error | `db.GetItemDataFuzzy(BaseItemID).Name == ""` OR AoW item missing from DB |
| `quantity_non_positive` | error | `Quantity == 0` (defence in depth alongside the schema validator) |
| `upgrade_out_of_range` | error | `Upgrade < 0` or `Upgrade > db.ItemData.MaxUpgrade` |
| `unknown_infusion` | error | `InfusionName != ""` and not present in `db.InfuseTypes` |
| `aow_not_weapon_target` | error | `aowItemID != nil` but the target item's DB category is not in {melee_armaments, ranged_and_catalysts, shields} |
| `aow_not_ash_category` | error | `aowItemID` resolves but its DB category is not `"ashes_of_war"` |
| `aow_incompatible` | error | `db.IsAshOfWarCompatibleWithWeapon` returned `(false, true)` |
| `aow_compat_unknown` | error | `db.IsAshOfWarCompatibleWithWeapon` returned `(_, false)` — **fail-closed** |
| `name_mismatch_ignored` | warning | template's `Name` differs from DB name; DB is source of truth |
| `unknown_mode` | warning | `ImportPreviewOptions.Mode` is set to a value other than `""` / `"append"` (forward-compat) |

`OK = (len(Errors) == 0)`. Warnings never block.

### Summary counters

`ImportPreviewSummary` buckets items by **resolved DB category** (not the template's debug-only `Category` field):
- `Weapons` ← `{melee_armaments, ranged_and_catalysts, shields}`
- `Armor` ← `{head, chest, arms, legs}`
- `Talismans` ← `{talismans}`
- `Stackables` ← anything else that resolved in DB
- `AoWAssignments` ← count of items whose AoW survived all compat checks

### Fail-closed AoW compatibility

When `db.IsAshOfWarCompatibleWithWeapon` reports `known=false` (missing bitmask data or unrecognised `wepType`), preview emits `aow_compat_unknown` and the report is NOT OK. This is intentional: silently accepting an unknown AoW assignment would let a template apply state that the game cannot represent.

### File dialog cancellation

`PreviewBuildTemplateImportFromFile` returns a sentinel report (`OK=false`, empty errors/warnings, empty summary) when the user dismisses the open-file dialog. The frontend uses `isCancelledPreview` to detect this shape and keeps the preview modal closed — no toast, no error.

---

## 7.3. Phase D endpoint contract (Apply to Workspace)

```go
func (a *App) ApplyBuildTemplateToWorkspaceJSON(sessionID string, jsonText string, opts ApplyTemplateOptions) (ApplyTemplateResult, error)
func (a *App) ApplyBuildTemplateToWorkspaceFromFile(sessionID string, opts ApplyTemplateOptions) (ApplyTemplateResult, error)
```

Result shape:

```go
type ApplyTemplateResult struct {
    Preview   templates.ImportPreviewReport       `json:"preview"`
    Workspace editor.InventoryWorkspaceSnapshot   `json:"workspace"`
    Applied   bool                                `json:"applied"`
}
```

### Invariants

- **Apply is RAM-only.** Apply mutates `sess.Workspace` (sets `Dirty=true`) but **never** calls `SaveInventoryWorkspaceChanges`, **never** writes `slot.Data`, **never** allocates GaItem handles. The save file is untouched until the user clicks Save changes.
- **Use of existing mutation path.** Items are appended to the workspace via `editor.AddItem` (the same call site the existing AddItem modal uses) and weapon-side fields are applied via `editor.UpdateWeapon` (the same call WeaponEditModal uses). No alternative path exists.
- **Mode whitelist.** Phase D ships only `mode="append"` (empty string is normalised to append). Other modes (`"replace-inventory"`, `"replace-all"`) are reserved and reject with an error.

### Validation order

1. Mode whitelist — `""` or `"append"` only. Anything else → Go error, no mutation.
2. Session existence — unknown session → Go error.
3. `ParseBuildTemplateJSON` — schema/structure check. Failures land on `Preview.Errors` as `structure_invalid` (not a Go error), with `Applied=false`.
4. `PreviewBuildTemplateImport` — per-item DB resolution + AoW compat. Failures return `Applied=false` with the preview report unchanged so the UI can render the same panel for parse and per-item issues.
5. Capacity preflight — `len(existing) + len(unsupported) + len(imported)` must fit under `core.CommonItemCount` (2688) for inventory and `core.StorageCommonCount` (1920) for storage. Failures append `capacity_exceeded` to `Preview.Errors` and return `Applied=false`.
6. RAM apply — append each `TemplateItem` via `editor.AddItem(...)` then, for weapons with non-default fields, `editor.UpdateWeapon(...)` with a `WeaponPatch` carrying `SetUpgrade` / `SetInfusionName` / `SetAoWItemID` as needed.

### Apply behavior

- **Append mode ordering** — imported inventory items land after any existing inventory items in template order. Same for storage. Existing items are not reordered.
- **No save-local handles** — every imported item enters with `Source: added`, `OriginalHandle: 0`. The eventual Save step mints fresh handles via the existing handle allocator in `core.AddItemsToSlotBatch`.
- **AoW assignment** — populates `PendingAoWItemID` / `PendingAoWName` + `HasPendingWeaponPatch=true`. Save resolves these into a real GaItem entry per the existing Phase 1.7 flow.
- **Transactional rollback** — before applying any items, the workspace is snapshotted in-memory via `deepCopySnapshot`. If `editor.AddItem` or `editor.UpdateWeapon` returns an error mid-way (unexpected DB drift, unsupported category, etc.), the snapshot is restored and a `Preview.Errors` entry is appended explaining the abort. The session's `Workspace.Dirty` cannot be flipped to true by a partial apply.
- **Cancelled file dialog** — `ApplyBuildTemplateToWorkspaceFromFile` with an empty selected path returns `Applied=false`, empty preview, current workspace. No mutation, no toast, no error.

### Frontend flow

- The preview modal accepts an optional `onApply` callback. When provided AND `report.OK==true`, an "Apply to workspace" button is rendered. Without `onApply` (the Phase C preview-only invocation), the button is hidden.
- `PreviewBuildTemplateImportFromFile` now returns `LoadedTemplatePreview { Report, JSON, Path }` so the Apply flow can re-use the loaded JSON content without re-opening the file dialog.
- On successful apply, the frontend swaps in `result.Workspace` via the `replaceSnapshot` hook method, closes the modal, and toasts: "Template applied to workspace. Click Save changes to persist."
- On `Applied=false` (e.g. capacity overflow surfacing post-preview), the modal stays open and re-renders with the new error report.

---

## 8. Local Build Template Library (Phase E)

Implemented in Phase E. The library is a per-user store of build templates
on the local filesystem so users can save, list, preview, apply, rename,
delete, and export templates without picking a file path every time.

### 8.1. Storage layout

- **Root directory**: `$UserConfigDir/EldenRing-SaveEditor/templates/`
  (macOS: `~/Library/Application Support/EldenRing-SaveEditor/templates/`,
  Linux: `~/.config/EldenRing-SaveEditor/templates/`,
  Windows: `%APPDATA%\EldenRing-SaveEditor\templates\`).
- **Directory mode** 0700 (created on first library use).
- **One template per file**, named `<sanitized-name>-<id-tail>.json`,
  mode 0644. Templates are not secrets; they are designed to be shareable.
- **Index file** `_index.json` (`LibraryIndexVersion=1`) carries metadata
  only (id, name, description, tags, filename, timestamps, item counts).
  Never raw save data. Never GaItem handles.
- **Atomic writes** — every file (templates and the index) is written
  as `.saveforge-tmp-*` next to the destination, fsynced, then renamed.
  A crash mid-write leaves the prior file (if any) intact.
- The library directory must **not** be reused for settings or any
  other non-template data.

### 8.2. Recovery semantics

- **Missing `_index.json`** is not an error; an empty index is returned.
  Users who manually drop JSON files into the directory must invoke
  `RebuildIndex` explicitly.
- **Corrupt `_index.json`** triggers an automatic rebuild from the
  directory contents. Files that fail to parse or validate are skipped
  but remain on disk.
- **Rebuild preserves IDs and CreatedAt** when a file is matched by
  filename to an entry in the prior index, so UIs keying off ID stay
  stable across recovery.

### 8.3. App endpoints

| Method | Purpose |
|---|---|
| `SaveBuildTemplateToLibrary(sessionID, opts)` → `LibraryTemplateEntry` | Build from active workspace + store. Reuses the export codepath; never mutates workspace/save. |
| `ListBuildTemplateLibrary()` → `[]LibraryTemplateEntry` | Index entries sorted newest-first. |
| `PreviewBuildTemplateFromLibrary(id)` → `LoadedTemplatePreview` | Load template + run the same dry-run validator as the file-based preview. Returns JSON for Apply round-trip. |
| `ApplyBuildTemplateFromLibrary(sessionID, id, opts)` → `ApplyTemplateResult` | Delegates to `ApplyBuildTemplateToWorkspaceJSON`. RAM-only. |
| `DeleteBuildTemplateFromLibrary(id)` | Remove file + index entry. |
| `RenameBuildTemplateInLibrary(id, name, description, tags)` → `LibraryTemplateEntry` | Update metadata in the file and the index; bump `updatedAt`. |
| `ExportLibraryBuildTemplateToFile(id)` → `BuildTemplateExportResult` | Save-file dialog + copy chosen template to a user-picked path. Cancellation returns empty Path. |

### 8.4. Invariants

- **Apply from library is RAM-only**, same path as `ApplyBuildTemplateToWorkspaceJSON`.
- **`SaveInventoryWorkspaceChanges` is never invoked from library actions.**
  The user must still click Save changes to persist.
- **Delete and Rename touch the library only** — workspace and save are
  not affected.
- **Library is lazily initialised** via `App.ensureTemplateLibrary` so
  unit tests can inject a temp directory without going through
  `DefaultTemplateLibraryDir`.
- **Settings export is not included** — that is a separate, future
  feature and must not share this directory.

### 8.5. Frontend

- `Export Template ▾` dropdown gains a `Template Library…` item that
  opens `TemplateLibraryModal`.
- `ExportTemplateModal` gains an optional `onSavedToLibrary` callback
  and a second action button **Save to local library**; the existing
  **Export JSON file** button is preserved unchanged.
- Library row actions: Preview, Apply, Export, Rename (inline form),
  Delete (custom React confirm row — no native Wails dialog, so tests
  can drive the flow under jsdom and the UI stays consistent with the
  rest of SortOrderTab).
- Successful Apply from library swaps `result.Workspace` in via
  `useInventoryWorkspace.replaceSnapshot` and toasts:
  *"Template applied to workspace. Click Save changes to persist."*

---

## 9. Sources

- `backend/editor/workspace.go` — `EditableItem`, `InventoryWorkspaceSnapshot`, `AoWStatus*` constants.
- `backend/editor/add.go` — `AddItemSpec`, the natural import-side mirror of `TemplateItem`.
- `backend/editor/save.go` — `ApplyWorkspaceSave`, the eventual import write path.
- `backend/db/db.go` — `GetItemDataFuzzy`, `InfuseTypes`, `IsAoWCompatibleWithWepType`.
- [54-ash-of-war.md](54-ash-of-war.md) — sentinel handle semantics, shared-handle invariant.
- [37-character-presets.md](37-character-presets.md) — prior preset export design; distinct from build templates, but informs versioning conventions.
- [39-inventory-reorder.md](39-inventory-reorder.md), [52-acquisition-sort-stride2.md](52-acquisition-sort-stride2.md), [53-inventory-storage-transfer.md](53-inventory-storage-transfer.md) — workspace UI prior art.
