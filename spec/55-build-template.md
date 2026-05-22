# 55 — Build Template

> **Type**: Design doc
> **Status**: ✅ Implemented — Phase A (exporter) + Phase B (file I/O) + Phase C (preview) + Phase D (apply) + Phase E (local library).
> **Scope**: Canonical chapter for the SaveForge Build Template — a portable JSON representation of an Inventory Workspace snapshot. Defines the `saveforge.build-template` v1 schema, the export / preview / apply / library contracts, the portable-payload rule (no save-local handles), and all import safety rules (incl. fail-closed on unknown AoW compat).

---

## 1. Chapter purpose

The Build Template lets you save a workspace inventory + storage (weapons, armor, talismans, stackable items, AoW assignment, infusion, upgrade level) as a single, portable JSON document. The goal:

- Bootstrap a new character with a known setup without manually adding every item.
- Share builds between users / platforms / characters within the same save.
- A per-user local library for quick apply from the UI.

The chapter ties together:

- the v1 schema (binary-stable JSON),
- the export flow (Phase A + B),
- import preview + apply (Phase C + D),
- the local library (Phase E),
- portability / safety rules,
- relations to the canonical inventory / AoW / allocator chapters.

Reference chapters we do **not** repeat:

- [54-ash-of-war](54-ash-of-war.md) — the full AoW semantics (sentinels, write paths, compat matrix).
- [03-gaitem-map](03-gaitem-map.md) — GaItem binary model.
- [06-equipment](06-equipment.md) — equipped gear (outside Phase A scope).
- [07-inventory](07-inventory.md) / [10-storage](10-storage.md) — inventory / storage section model.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator capacity rules.
- [36-inventory-categories-game-order](36-inventory-categories-game-order.md) — item categories.
- [43-transactional-item-adding](43-transactional-item-adding.md) — the Add Items pipeline (used only at save, not at apply).

---

## 2. Status

| Phase | Scope | Status |
|---|---|---|
| **A** | Schema + exporter (`backend/templates/schema.go`, `export.go`), spec, tests | ✅ |
| **B** | Wails bindings `ExportBuildTemplateJSON` / `ExportBuildTemplateToFile`, `ExportTemplateModal`, dropdown in SortOrderTab | ✅ |
| **C** | Import preview: `ParseBuildTemplateJSON`, `PreviewBuildTemplateImport`, endpoints `PreviewBuildTemplateImportJSON/FromFile`, `ImportTemplatePreviewModal` (read-only) | ✅ |
| **D** | Apply to workspace: `ApplyBuildTemplateToWorkspaceJSON/FromFile`, capacity preflight, RAM-only mutation with rollback | ✅ |
| **E** | Local template library under `$UserConfigDir/EldenRing-SaveEditor/templates/`, 9 App endpoints, `TemplateLibraryModal` | ✅ |
| 2+ | `character.profile` section (level, stats, talisman slots), opt-in via `$enabled` | 🔲 deferred |
| 2+ | `replace-inventory` / `replace-all` modes (reserved in `ApplyTemplateOptions.Mode`) | 🔲 deferred |

---

## 3. Source of truth in code

| Area | Files / symbols |
|---|---|
| Schema types | `backend/templates/schema.go`: `BuildTemplate`, `TemplateMetadata`, `TemplateSections`, `InventoryWorkspaceSection`, `TemplateItem`, `ExportWarning`, `ExportReport`. |
| Schema constants | `schema.go`: `SchemaKey = "saveforge.build-template"`, `SchemaVersion = 1`, `WarnCode*`, `ContainerInventory`/`ContainerStorage`. |
| Exporter | `backend/templates/export.go`: `BuildTemplateFromSnapshot`, `ExportOptions`, `convertItems`, `ValidateBuildTemplate`, `validateItems`. |
| Import / preview | `backend/templates/import.go`: `ParseBuildTemplateJSON`, `PreviewBuildTemplateImport`, `ImportPreviewOptions`, `ImportPreviewReport`, `ImportPreviewIssue`, `ImportPreviewSummary`, `IssueCode*`. |
| Library | `backend/templates/library.go`: `TemplateLibrary`, `LibraryTemplateEntry`, `TemplateLibraryIndex`, `LibraryIndexFile = "_index.json"`, `LibraryIndexVersion = 1`, `DefaultTemplateLibraryDir`, `atomicWriteFile`. |
| App bindings | `app_templates.go`: `BuildTemplateExportOptions`, `BuildTemplateExportResult`, `ApplyTemplateOptions`, `ApplyTemplateResult`, `LoadedTemplatePreview` + 15 Wails-exposed `App` methods (Export×2, Preview×2, Apply×2, Library×9) plus 3 internal helpers (`buildAndValidateTemplate`, `sourceCharacterName`, `ensureTemplateLibrary`). |
| Capacity | `backend/core/offset_defs.go`: `CommonItemCount = 0xA80 (2688)`, `StorageCommonCount = 0x780 (1920)`. |
| Apply mutation | `app_templates.go::applyTemplateItemsToWorkspace` → `editor.AddItem` + `editor.UpdateWeapon`. |
| Capacity preflight | `app_templates.go::capacityPreflight`. |
| Rollback | `app_templates.go::deepCopySnapshot`. |
| Frontend modals | `frontend/src/components/templates/`: `ExportTemplateModal.tsx` (calls `ExportBuildTemplateToFile`, `SaveBuildTemplateToLibrary`), `ImportTemplatePreviewModal.tsx` (read-only props, does not call Wails directly), `TemplateLibraryModal.tsx` (calls 8 library Wails methods). |
| Frontend orchestrator | `frontend/src/components/SortOrderTab.tsx` (calls `ExportBuildTemplateToFile`, `PreviewBuildTemplateImportFromFile`, `ApplyBuildTemplateToWorkspaceJSON`; orchestrates the modals + `useInventoryWorkspace.replaceSnapshot` after Apply). |

---

## 4. Mental model

A Build Template = a portable snapshot of the **inventory workspace** contents at a given moment. Three layers of responsibility:

1. **Schema** (`backend/templates/schema.go`) — typed JSON, with no dependency on `editor`, `core`, or `db`.
2. **Pipeline** (`backend/templates/{export,import,library}.go`) — pure functions from snapshot to JSON and from JSON to `ImportPreviewReport`. The library is a per-user store.
3. **App glue** (`app_templates.go`) — ties the schema/pipeline to `editor.InventoryEditSession`, the file dialog, capacity preflight, rollback.

Key invariant: **the template stores only semantic identifiers** (`baseItemID`, `quantity`, `upgrade`, `infusionName`, `aowItemID`). Never save-local addressing (GaItem handle, session UID, acquisition index). Thanks to this, a template created in save A can be applied in save B with no collision in the handle space.

Apply is **RAM-only**: it mutates `sess.Workspace` (sets `Dirty=true`), but never calls `SaveInventoryWorkspaceChanges`, does not write to `slot.Data`, does not allocate GaItem handles. The user must still click "Save changes" — then the save path delegating to `editor.AddItem` + `editor.UpdateWeapon` via `ApplyWorkspaceSave` will allocate the real handles through the allocator from [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).

---

## 5. Build Template vs Character Preset vs Advanced Preset

Three different mechanisms, often confused:

| Mechanism | What it saves | Scope | Status in SaveForge |
|---|---|---|---|
| **Build Template** (this document) | Workspace inventory + storage (weapons with upgrade/infusion/AoW, armor, talismans, stackables) | Cross-save portable | ✅ Phase A–E |
| **Character Preset** ([37-character-presets](37-character-presets.md)) | Starting class (level, stats, starting gear) | Default game presets + custom | a separate mechanism |
| **Advanced Preset** | n/a | (outside SaveForge scope) | does not exist |

Build Template **does not export**:

- character level / stats / attributes,
- spell loadout / gestures / key items,
- equipped gear (the save's `Equipment` section) — see §11,
- map progress / event flags / acquisition history,
- regulation.bin overrides.

`needs verification` for potential future `character.profile` / `equipment` sections — the schema has a slot for them (`$enabled` flag), but in v1 they are out of scope.

---

## 6. JSON v1 schema

### 6.1. Header

```json
{
  "schema": "saveforge.build-template",
  "version": 1,
  "createdAt": "2026-05-17T12:34:56Z",
  "appVersion": "0.15.0-beta",
  "metadata": { … },
  "sections": { … }
}
```

| Field | Type | Required | Source | Portability note |
|---|---|---|---|---|
| `schema` | string | ✅ | `schema.go::SchemaKey` | Must be exactly `"saveforge.build-template"`. The importer rejects any other value. |
| `version` | int | ✅ | `schema.go::SchemaVersion` | Accepted range: `1 ≤ v ≤ SchemaVersion` (currently max = 1). |
| `createdAt` | string (RFC 3339 UTC) | ✅ | `time.Now().UTC().Format(time.RFC3339)` or `opts.Now` (test-only) | Informational. |
| `appVersion` | string | optional | `BuildTemplateExportOptions.AppVersion` | Informational. |
| `metadata` | object | optional | `TemplateMetadata` (name/description/author/tags/sourceCharacterIndex/sourceCharacterName) | **No load-bearing fields** — nothing in the metadata changes the import logic. |
| `sections` | object | ✅ | `TemplateSections` | Key = a stable section identifier. v1 has only `inventory.workspace`. |

### 6.2. The `inventory.workspace` section

```json
"sections": {
  "inventory.workspace": {
    "inventoryItems": [ TemplateItem, … ],
    "storageItems":   [ TemplateItem, … ]
  }
}
```

Both arrays must be present (may be empty), but `ValidateBuildTemplate` rejects the template when both are empty (`"inventory.workspace is empty"`).

### 6.3. `TemplateItem`

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

| Field | Type | Required | Source | Portability note |
|---|---|---|---|---|
| `baseItemID` | uint32 | ✅ | `EditableItem.BaseItemID` (without upgrade/infusion encoding) | DB lookup key at import. **Cannot be 0.** |
| `name` | string | optional | `EditableItem.Name` | Debug/display only; at import the DB name is the source of truth. Drift produces the warning `name_mismatch_ignored`. |
| `category` | string | optional | `EditableItem.Category` | Debug/display only; the resolved DB category is the source of truth. |
| `quantity` | uint32 | ✅ | `EditableItem.Quantity` | Stack size. **Cannot be 0.** |
| `upgrade` | int | optional | `EditableItem.CurrentUpgrade` | Default 0; range `[0, MaxUpgrade]` validated by the DB at import. |
| `infusionName` | string | optional | `EditableItem.InfusionName` | Must be in `db.InfuseTypes` if != `""`. Empty / `"Standard"` → un-infused. |
| `aowItemID` | uint32 (pointer + `omitempty`) | optional | `EditableItem.CurrentAoWItemID` (when `CurrentAoWStatus == AoWStatusCustom`) | See §6.4. **Never `0`, never an in-save handle.** |
| `container` | string | ✅ | `"inventory"` / `"storage"` | Must match the parent array (the validator enforces). |
| `position` | int | ✅ | array index after a stable sort | Stable position within the container; a mismatch vs `EditableItem.Position` → warning `position_normalized`. |

### 6.4. AoW encoding

The `aowItemID` field uses `*uint32 + omitempty`. The decision per the source weapon's AoW state (see [54-ash-of-war §16.4](54-ash-of-war.md)):

| `CurrentAoWStatus` | JSON result |
|---|---|
| `"custom"` with `CurrentAoWItemID != 0` | `"aowItemID": <id>` (the gem's semantic ItemID, prefix `0x8…`) |
| `"none"` | field omitted |
| `"missing"` (dangling handle) | field omitted + warning `aow_missing_skipped` |
| `"shared"` (shared handle) | field omitted + warning `aow_shared_skipped` |
| empty / non-weapon | field omitted |

The exporter **never** writes:

- `"aowItemID": 0` (zero is reserved for "no custom AoW", whose encoding is "field omitted"),
- an in-save sentinel handle (`0x00000000` / `0xFFFFFFFF`) — these are save-local addresses,
- an AoW GaItem handle (`0xC0…`) — also save-local.

The full AoW sentinel semantics and the shared-handle invariant — [54-ash-of-war §6 and §11](54-ash-of-war.md).

### 6.5. Excluded fields

`EditableItem` has more fields than `TemplateItem`. Excluded (test `TestSchemaJSON_OmitsForbiddenFields`):

| `EditableItem` field | Reason for exclusion |
|---|---|
| `originalHandle` | Save-local GaItem handle. |
| `currentAoWHandle` | Save-local AoW GaItem handle. |
| `uid` | Workspace session UUID; rerolled every session. |
| `acquisitionIndex` | Per-character chronology counter ([52-acquisition-sort-stride2](52-acquisition-sort-stride2.md)); renewed at save. |
| `pendingAoWItemID`, `pendingAoWName`, `pendingAoWClear`, `hasPendingWeaponPatch` | RAM-only workspace edits; the exporter mirrors the **saved** state, not the pending request. |
| `hasGaItem`, `hasCurrentAoW`, `currentAoWShared`, `currentAoWStatus` | Derived flags; recompiled at import from the target save's DB. |
| `isWeapon`, `isArmor`, `isTalisman` | Derived from the DB lookup; may drift with the template. |
| `maxUpgrade` | A DB constant; never authoritative in the template. |
| `iconPath` | Cosmetic; localized resource paths. |

Pass-through records (`UnsupportedInventoryRecords`, `UnsupportedStorageRecords`) — items outside the Phase 1 allow-list — are also excluded. They would not survive an import based on `editor.AddItem`.

---

## 7. Portable payload rule

The rule summarizing §6.5: **the template must not carry any value that has meaning only in the handle space of a specific save**.

Specific prohibitions:

| Forbidden | Reason |
|---|---|
| A GaItem handle of any type (`0x80…` weapon, `0xC0…` AoW, etc.) | Save-local; allocated per slot. |
| `AoWGaItemHandle` raw value (sentinel or `0xC0…`) | Save-local; handled by the semantic value `aowItemID`. |
| Session UID, OriginalHandle, acquisition index | Per-session or per-character. |
| Pending* fields | RAM-only in the workspace; do not represent the saved state. |
| Weapon ItemID with encoded upgrade/infusion (`baseID + level + infusionOffset`) | The template stores `baseItemID` + separately `upgrade` + `infusionName`; the full encoded ID is reconstructed at import. |

Regression test: `backend/templates/schema_test.go::TestSchemaJSON_OmitsForbiddenFields` marshals the full payload and asserts via grep that the forbidden field names are absent from the JSON.

`needs verification`: there is no automatic binary scan over the marshalled JSON — the test relies on field names. Hypothetical future fields in `EditableItem` will be automatically excluded only if `omitempty` is set correctly and the name is not in the test whitelist.

---

## 8. Export flow

### 8.1. Core function contract

```go
// backend/templates/export.go
func BuildTemplateFromSnapshot(
    snap editor.InventoryWorkspaceSnapshot,
    opts ExportOptions,
) (*BuildTemplate, *ExportReport, error)
```

Behavior:

1. At least one of `opts.IncludeInventory` / `opts.IncludeStorage` must be true. Otherwise → error before production.
2. For each enabled container: stable-sort `EditableItem.Position`, project onto `TemplateItem`. A `Position` vs final array index mismatch → warning `position_normalized`.
3. `BaseItemID == 0` or `Quantity == 0` → **Go error** (not a warning), no template.
4. AoW handling per §6.4 (status-driven, custom → emit, missing/shared → skip + warning).
5. Stamps `Schema`, `Version`, `CreatedAt` (`time.Now().UTC()` or `opts.Now`), `AppVersion`, `Metadata`.

The function is a **pure read** of `snap` — no snapshot field is mutated (tested by `TestExport_*`).

### 8.2. Validator

```go
func ValidateBuildTemplate(tpl *BuildTemplate) error
```

Scope (structural checks, **no DB lookups**):

- `tpl != nil`
- `tpl.Schema == SchemaKey`
- `0 < tpl.Version ≤ SchemaVersion`
- `tpl.Sections.InventoryWorkspace != nil`
- At least one of `inventoryItems` / `storageItems` non-empty
- Per item: `BaseItemID != 0`, `Quantity != 0`, `Container` consistent with the parent array

DB-level checks (item exists, AoW compat) are in Phase C — see §9.

### 8.3. App endpoints (Phase B)

```go
func (a *App) ExportBuildTemplateJSON(sessionID string, opts BuildTemplateExportOptions) (BuildTemplateExportResult, error)
func (a *App) ExportBuildTemplateToFile(sessionID string, opts BuildTemplateExportOptions) (BuildTemplateExportResult, error)
```

Common behavior:

- Resolve the session by `sessionID` (`a.editSessions[sessionID]`). Unknown → error `inventory edit session %q not found`.
- Stamp `Metadata.SourceCharacterIndex` from the session, `Metadata.SourceCharacterName` from the current save (empty if no save).
- `BuildTemplateFromSnapshot` + `ValidateBuildTemplate` — validation errors are wrapped Go errors.
- **Never** `SaveInventoryWorkspaceChanges`, **never** mutate `slot.Data`, **never** allocate a handle. A `Dirty=true` workspace is a legitimate export source — the feature exists so the user can save a build before clicking Save.

`ExportBuildTemplateJSON` returns the payload as a `string` in `BuildTemplateExportResult.JSON`. `ExportBuildTemplateToFile` shows the native save-file dialog (`runtime.SaveFileDialog`), writes the file mode 0644. Cancelling the dialog (empty path) is **not** an error — the result has `Path == ""` and the UI silently ignores it.

Warnings from `ExportReport` are passed through to `BuildTemplateExportResult.Warnings`.

### 8.4. Warning codes

| Code | Trigger | Stability |
|---|---|---|
| `aow_missing_skipped` | `CurrentAoWStatus == "missing"` (dangling handle) | stable, asserted in tests |
| `aow_shared_skipped` | `CurrentAoWStatus == "shared"` (multi-weapon ref) | stable, asserted in tests |
| `position_normalized` | `EditableItem.Position != array_index_after_sort` | stable, asserted in tests |

---

## 9. Import / preview flow

### 9.1. Core function contract

```go
// backend/templates/import.go
func ParseBuildTemplateJSON(data []byte) (*BuildTemplate, error)
func PreviewBuildTemplateImport(tpl *BuildTemplate, opts ImportPreviewOptions) ImportPreviewReport
```

`ParseBuildTemplateJSON` = `json.Unmarshal` + `ValidateBuildTemplate`. Malformed JSON / wrong schema → Go error.

`PreviewBuildTemplateImport` = **dry-run** validation against the live DB. It does **not** mutate the workspace, does **not** allocate handles, does **not** write the save. The `backend/templates` package does not import `backend/editor` nor `backend/core` — a structural guarantee.

### 9.2. App endpoints (Phase C)

```go
func (a *App) PreviewBuildTemplateImportJSON(jsonText string) (templates.ImportPreviewReport, error)
func (a *App) PreviewBuildTemplateImportFromFile() (LoadedTemplatePreview, error)
```

`LoadedTemplatePreview` bundles `Report`, `JSON`, `Path` — it lets the UI pass the payload to Apply without re-opening the dialog.

A malformed payload does not produce a Go error — it comes back as an `ImportPreviewReport` with `OK=false` and a single `structure_invalid` error. This lets the frontend render parse and per-item failures through the same panel.

Cancelling the file dialog → the sentinel `cancelledPreviewReport` (`OK=false`, empty error/warning/summary). The frontend detects it via `isCancelledPreview` and shows neither a toast nor a modal.

### 9.3. Per-item validation rules

| Code | Severity | Trigger |
|---|---|---|
| `structure_invalid` | error | `ParseBuildTemplateJSON` returned a Go error (unparsable / schema-mismatch) |
| `schema_invalid` | error | `ValidateBuildTemplate` rejected a non-nil template after construction |
| `unknown_item` | error | `db.GetItemDataFuzzy(BaseItemID).Name == ""` OR the AoW item not in the DB |
| `quantity_non_positive` | error | `Quantity == 0` (defense-in-depth at the validator) |
| `upgrade_out_of_range` | error | `Upgrade < 0` or `Upgrade > db.ItemData.MaxUpgrade` |
| `unknown_infusion` | error | `InfusionName != ""` and not in `db.InfuseTypes` |
| `aow_not_weapon_target` | error | `aowItemID != nil` but the target category is not in `{melee_armaments, ranged_and_catalysts, shields}` |
| `aow_not_ash_category` | error | `aowItemID` resolves, but its DB category is not `"ashes_of_war"` |
| `aow_incompatible` | error | `db.IsAshOfWarCompatibleWithWeapon` returned `(false, true)` — known incompatible |
| `aow_compat_unknown` | error | `db.IsAshOfWarCompatibleWithWeapon` returned `(_, false)` — **fail-closed** |
| `unsupported_category` | error | (reserved — used by apply-side checks) |
| `capacity_exceeded` | error | Per-container slot cap (`CommonItemCount` / `StorageCommonCount`) exceeded — added by apply, not preview |
| `name_mismatch_ignored` | warning | The template `Name` differs from the DB; the DB is the source of truth |
| `unknown_mode` | warning | `ImportPreviewOptions.Mode` != `""`/`"append"` (forward-compat) |

`OK = (len(Errors) == 0)`. Warnings never block the import.

### 9.4. Summary

`ImportPreviewSummary` buckets items by the **resolved DB category** (not by the debug-only template `Category`):

- `Weapons` ← `{melee_armaments, ranged_and_catalysts, shields}`
- `Armor` ← `{head, chest, arms, legs}`
- `Talismans` ← `{talismans}`
- `Stackables` ← anything else that resolved in the DB
- `AoWAssignments` ← a counter of items whose AoW passed all compat checks

Items that failed `unknown_item` / `quantity_non_positive` are excluded from the summary (continue in `previewItems`). Items with non-fatal errors (e.g., `upgrade_out_of_range`) still count, so the user sees the intended shape.

### 9.5. Fail-closed AoW compatibility — rationale

The portability vs safety trade-off is explicit and intentional:

- **Portability**: the template does not know the target save's `wepType` nor the `canMountWep_*` bitmask. Storing portable ItemIDs allows applying in saves with a different DLC / patch level.
- **Safety**: silently accepting an unknown AoW assignment would let the template apply a state the game cannot represent ([54-ash-of-war §8.4](54-ash-of-war.md) — the full fail-closed vs passthrough matrix).

Decision: preview/apply **fail-closes** on `known=false`. The user gets the precise message `"AoW compatibility data missing for X on Y — failing closed"` and can either:

- apply the template with a different save (a DB shape matching the AoW),
- manually remove the problematic `aowItemID` from the template before apply,
- wait until `data.AoWCompatMasks` / `data.WepTypeToCanMountBit` are updated for the given wepType (e.g., DLC 69/94/95 — see [54-ash-of-war §17, §22.L2](54-ash-of-war.md)).

`needs verification`: affinity gating (Heavy Longsword vs Standard Longsword) — the preview currently validates compat only by `wepType`, not by the specific infusion variant. A template with an AoW on an infused weapon passes the compat check as long as the baseID + wepType allow it. The full affinity gating matrix is described as `needs verification` in [54-ash-of-war §22.L1](54-ash-of-war.md).

---

## 10. Apply flow

### 10.1. Endpoint contract (Phase D)

```go
func (a *App) ApplyBuildTemplateToWorkspaceJSON(sessionID string, jsonText string, opts ApplyTemplateOptions) (ApplyTemplateResult, error)
func (a *App) ApplyBuildTemplateToWorkspaceFromFile(sessionID string, opts ApplyTemplateOptions) (ApplyTemplateResult, error)
```

```go
type ApplyTemplateResult struct {
    Preview   templates.ImportPreviewReport       `json:"preview"`
    Workspace editor.InventoryWorkspaceSnapshot   `json:"workspace"`
    Applied   bool                                `json:"applied"`
}
```

### 10.2. Apply invariants

- **RAM-only**. Mutates `sess.Workspace` (`Dirty=true` on success). Never calls `SaveInventoryWorkspaceChanges`, does not write `slot.Data`, does not allocate a GaItem handle. The save is untouched until the user clicks "Save changes".
- **Whitelist mode**. Phase D supports only `mode="append"` (an empty string is normalized). `"replace-inventory"` and `"replace-all"` are reserved → Go error.
- **Existing mutation path**. Items are appended via `editor.AddItem` (the same call site as the AddItem modal); weapon fields are applied via `editor.UpdateWeapon` (the same as WeaponEditModal in workspace mode — [54-ash-of-war §16.3](54-ash-of-war.md)). There is no alternative path.

### 10.3. Validation order (early-exit per step)

1. **Mode whitelist** — `""` or `"append"`. Other → Go error, no mutation.
2. **Session existence** — unknown session → Go error.
3. **`ParseBuildTemplateJSON`** — failures come back as an `ImportPreviewReport` with `structure_invalid` (NOT a Go error), `Applied=false`.
4. **`PreviewBuildTemplateImport`** — per-item DB resolution + AoW compat. Failures return the preview report unchanged, `Applied=false`.
5. **Capacity preflight** (`capacityPreflight`):
   - inventory: `len(existing) + len(unsupported) + len(template) ≤ CommonItemCount (2688)`
   - storage: `len(existing) + len(unsupported) + len(template) ≤ StorageCommonCount (1920)`
   - Failures append `capacity_exceeded` to `Preview.Errors`, `Applied=false`.
6. **Snapshot for rollback** — `deepCopySnapshot(sess.Workspace)` before any mutation.
7. **RAM apply** — `applyTemplateItemsToWorkspace` iterates per container:
   - `editor.AddItem(snap, AddItemSpec{BaseItemID, Quantity}, container, len(target_array))` → append mode.
   - If `IsWeapon` and any of `Upgrade>0` / `InfusionName!=""` / `AoWItemID!=nil`: `editor.UpdateWeapon(snap, added.UID, WeaponPatch{...})` with `SetUpgrade` / `SetInfusionName` / `SetAoWItemID` as needed.
   - The first error → restore from the snapshot, `Preview.Errors += apply_error`, `Applied=false`.
8. **Mark dirty** — `sess.Workspace.Dirty = true`, re-validate.

### 10.4. Apply behavior

- **Append ordering**: imported items land **after** the existing ones, in the template order. Existing ones are not reordered.
- **No save-local handles**: every imported item enters as `Source: added`, `OriginalHandle: 0`. The target Save (via `ApplyWorkspaceSave`) will allocate a fresh handle through the allocator from [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) (constraint: `NextArmamentIndex < maxEntries` for AoW — see [54-ash-of-war §15](54-ash-of-war.md)).
- **AoW assignment**: sets `PendingAoWItemID` / `PendingAoWName` + `HasPendingWeaponPatch=true`. The save resolves via `editor.executePendingAoWPatches` → `core.PatchWeaponAoW` (allocate + rebuild — [54-ash-of-war §14](54-ash-of-war.md)).
- **Transactional rollback** applies only to the workspace layer (Phase D). After Save (Phase 1.7+) a failure further down the `ApplyWorkspaceSave` path is a separate problem.
- **Dialog cancellation**: `ApplyBuildTemplateToWorkspaceFromFile` with an empty path → `cancelledApplyResult` (`Applied=false`, empty preview, current workspace). No mutation, no toast, no error.

### 10.5. Frontend Phase D flow

- The preview modal accepts an optional `onApply` callback. When provided AND `report.OK==true`, the "Apply to workspace" button is rendered.
- `PreviewBuildTemplateImportFromFile` returns `LoadedTemplatePreview` with `JSON` so Apply can reuse the payload without a second dialog.
- After a successful apply: `useInventoryWorkspace.replaceSnapshot(result.Workspace)`, modal close, toast: *"Template applied to workspace. Click Save changes to persist."*
- Apply on `Applied=false` (capacity overflow post-preview): the modal stays open and re-renders the new report.

`needs verification`: Phase D rollback tests the happy path + capacity overflow + structure invalid. The edge case "rollback after a partial AddItem N where N+1 fails" is tested indirectly via `deepCopySnapshot`, but there is no dedicated test that `Validation` is fully restored (slices). The test `TestApplyBuildTemplate*` in `app_templates_test.go` should be checked for coverage.

---

## 11. Inventory and item records

Build Template exports only the **editable allow-list** items of the workspace ([07-inventory](07-inventory.md), [10-storage](10-storage.md)):

| Category | Exported? | Note |
|---|---|---|
| `melee_armaments` (weapons + AoW + infusion + upgrade) | ✅ | Full weapon record in the template. |
| `ranged_and_catalysts` (bows, catalysts, sacred seals) | ✅ | With AoW when applicable. |
| `shields` | ✅ | With AoW when applicable. |
| `head`, `chest`, `arms`, `legs` | ✅ | Without modification (upgrade=0 always). |
| `talismans` | ✅ | Stackable=false, always quantity=1. |
| Stackable misc items (e.g., crests, runes, weapons in stackable categories) | ✅ | Exported if `EditableItem.IsWeapon == false` and they are in the workspace allow-list. |
| Spells / sorceries / incantations | ❌ outside Phase A scope | Reserved for `character.profile` v2+. |
| Key items, gestures, cookbooks | ❌ | Not in the workspace allow-list. |
| Equipment slots (equipped gear) | ❌ — see §12 | Save state, not a workspace item. |
| Spirit Ashes (Ash Summons) | ❌ outside Phase A scope | Reserved. |
| Magic / sorceries / prayers loadout | ❌ outside Phase A scope | Reserved. |
| Character level / stats / attributes | ❌ outside Phase A scope | Reserved for `character.profile` v2+. |
| Pass-through `UnsupportedInventoryRecords` / `UnsupportedStorageRecords` | ❌ | Items outside the allow-list — would not survive `editor.AddItem`. |

`needs verification`: the list is complete for v1, but the matrix of "what exactly is the workspace allow-list" should be cross-checked on every change to `editor.SupportedCategories`. The frontend `weaponCategories` / `armorCategories` in `import.go` are copies of the backend ones — no CI guard.

---

## 12. Equipment relation

Build Template **does not export** equipped gear (the save's Equipment section, [06-equipment](06-equipment.md)). Consequences:

- After Apply + Save: the added weapons / armor enter the inventory **unequipped**. The user must equip them manually in-game.
- There is no write path to the equipment section in Phase A–E. Even if the template was exported with an equipped weapon, that fact is not persisted.
- An AoW assigned to an equipped weapon in the source save → after apply it becomes "AoW on an inventory weapon" in the target save. The game's resolver shows the AoW in-game only once the user equips the weapon.

`needs verification`: there is no E2E test "equip → save → reload → equip status preserved" in the full Build Template context. The equipment section itself is covered by tests in `06-equipment`, but the interaction with Build Template apply is not explicitly tested.

---

## 13. Ash of War relation

All AoW semantics are in [54-ash-of-war](54-ash-of-war.md). Build Template does:

1. **Export**: reads `EditableItem.CurrentAoWItemID` (the semantic prefix `0x8…`) when `CurrentAoWStatus == AoWStatusCustom`. Never exports a handle (`0xC0…`) nor the sentinels (`0x00000000` / `0xFFFFFFFF`).
2. **Preview**: calls `db.IsAshOfWarCompatibleWithWeapon(aowID, baseItemID)`. Fail-closed on `known=false`.
3. **Apply**: sets `PendingAoWItemID` in `EditableItem`; the save delegating to `core.PatchWeaponAoW` allocates a fresh AoW GaItem through the existing Phase 1.7 flow.

What Build Template **does not do**:

- It does not check affinity gating (`defaultWepAttr` / `configurableWepAttr00..23`) — that is `needs verification` in [54-ash-of-war §22.L1](54-ash-of-war.md).
- It does not choose between the strict vs legacy AoW path — apply always goes through legacy (allocate + rebuild) at save, because the user has no control over the existing free copies in the target save.
- It does not handle shared-handle conflict resolution — that is [54-ash-of-war §11](54-ash-of-war.md) (enforced by `core.PatchWeaponAoW`, which mints a new handle).

---

## 14. Infusion and weapon metadata

Weapon-side fields extracted from `EditableItem` (per [54-ash-of-war §16.3](54-ash-of-war.md) WeaponPatch semantics):

| Field | Source | Apply path |
|---|---|---|
| `upgrade` | `EditableItem.CurrentUpgrade` (after `decodeWeaponUpgradeInfusion`) | `WeaponPatch.SetUpgrade=true, Upgrade=N` → `editor.UpdateWeapon` |
| `infusionName` | `EditableItem.InfusionName` (string from `db.InfuseTypes`) | `WeaponPatch.SetInfusionName=true, InfusionName="Heavy"` → `editor.UpdateWeapon` |
| `aowItemID` | `EditableItem.CurrentAoWItemID` | `WeaponPatch.SetAoWItemID=true, AoWItemID=…` → `PendingAoWItemID` (save handle) |

`editor.UpdateWeapon` is called **only** if any of the fields is non-default (`Upgrade>0` OR `InfusionName!=""` OR `AoWItemID!=nil`). No weapon-side patch for a weapon with default values → a faster apply path.

The weapon ItemID encoding at save is reconstructed by `encodeWeaponItemID(baseID, level, infusionName)` in [54-ash-of-war §16.3 + editor/weapon.go](54-ash-of-war.md).

---

## 15. Compatibility and validation

### 15.1. Multi-layer validation

| Layer | What it validates | When |
|---|---|---|
| `ValidateBuildTemplate` (Phase A) | Schema, structure, basic invariants. **No DB.** | Export (output) + on every import parse. |
| `PreviewBuildTemplateImport` (Phase C) | Per-item DB resolution, AoW compat, upgrade/infusion range. | Preview + apply step 4. |
| `capacityPreflight` (Phase D) | Per-container slot caps. | Apply step 5 (after preview). |
| `editor.AddItem` / `editor.UpdateWeapon` (Phase D apply step 7) | Workspace allow-list, AoW DB category, AoW infusion conflict. | Apply step 7 — per item. |
| `editor.Validate` (after apply step 8) | Pending* fields conflicts, `CodePendingAoWUnknown`/`Conflict`. | After apply (workspace re-validate). |
| `editor.validatePendingAoWChanges` (at Save, [54-ash-of-war §16.3](54-ash-of-war.md)) | Final AoW compat fail-closed check. | At `ApplyWorkspaceSave`, not at template apply. |

### 15.2. What is NOT validated

- **Affinity gating** — `needs verification` (see §13).
- **Equipment slot occupancy** — Build Template does not touch equipment.
- **DLC presence in the target save** — a template with DLC items applied to a non-DLC save → `unknown_item` (the DB does not ship DLC entries if the DB build is non-DLC). `needs verification` for edge cases.
- **Regulation.bin version drift** — all DB lookups are against the current compiled-in SaveForge DB; there is no version check vs the save's regulation.

### 15.3. Forward-compat

| Field | Behavior |
|---|---|
| Unknown fields in `metadata` | Tolerated by `encoding/json` (no tagged ignore — Go silently forgets). |
| Unknown keys in `sections` | `TemplateSections.InventoryWorkspace` is the only known key; unknown ones are ignored by the parser (tag-based unmarshalling). |
| `ImportPreviewOptions.Mode` != `""`/`"append"` | Warning `unknown_mode` in preview; apply with another mode → Go error. |
| `version > SchemaVersion` | `ValidateBuildTemplate` returns the error `unsupported version`. |

---

## 16. Allocator and capacity relation

Build Template **does not** call `core.allocateGaItem` nor `core.PatchWeapon*` directly. All allocations are deferred to the save:

| Phase | Allocator touch? |
|---|---|
| Export | ❌ pure read |
| Preview | ❌ pure DB read |
| Apply (Phase D) | ❌ only `editor.AddItem` / `editor.UpdateWeapon` (RAM-only) |
| Save (after Apply, `ApplyWorkspaceSave`) | ✅ `editor.executeAdds` → `core.AddItemsToSlotBatch` → `core.allocateGaItem` per item |
| Save AoW (if `PendingAoWItemID`) | ✅ `editor.executePendingAoWPatches` → `core.PatchWeaponAoW` → `core.allocateGaItem` per AoW |

The capacity preflight (§10.3 step 5) checks only the per-container slot count (inventory: 2688, storage: 1920) **before** apply to the workspace. It does not check:

- `NextArmamentIndex < len(GaItems)` (GaItem allocator capacity) — happens at save and may fail if the slot is near full + an AoW add fails per [54-ash-of-war §15](54-ash-of-war.md).
- `NextAoWIndex < maxEntries` — see above.
- `NextGaItemHandle` overflow — a very high budget, but not checked.

`needs verification`: the scenario "preview OK, capacity preflight OK, apply OK, but save fails with `armament zone at capacity`" is theoretically possible. There is no E2E test covering the full template → apply → save path. There is also no user-facing error on a capacity overflow at save (the allocator message is passed through as a Go error by `ApplyWorkspaceSave`).

---

## 17. Error handling and failure semantics

### 17.1. Failure surfaces

| Phase | Failure surface | Format |
|---|---|---|
| Export | Go error (path: backend → app → frontend exception) | `BuildTemplateExportResult{}, error` |
| Preview | `ImportPreviewReport` with `OK=false` + errors | `templates.ImportPreviewReport` |
| Apply structure invalid | `ApplyTemplateResult{Preview.OK=false, Applied=false}` | `Preview.Errors[].Code = "structure_invalid"` |
| Apply preview errors | same as preview phase | Per-item issues in `Preview.Errors` |
| Apply capacity | same, plus `"capacity_exceeded"` | Per-container summary message |
| Apply mutation (AddItem/UpdateWeapon) | rollback + `Preview.Errors` append + `Applied=false` | Wrapped error in the message field |
| Apply mode invalid | Go error | `ApplyTemplateResult{}, error` |
| Apply session not found | Go error | `ApplyTemplateResult{}, error` |
| File dialog cancelled | sentinel non-error | `Applied=false`, empty preview |

### 17.2. What the code does NOT validate (full list of safety gaps)

- ❌ Affinity per AoW infusion variant ([54-ash-of-war §22.L1](54-ash-of-war.md)).
- ❌ DLC presence cross-check at apply (`unknown_item` flags individual items, but there is no global "this template requires DLC X").
- ❌ Cross-platform portability (PS4 vs PC save) — the template is format-agnostic; DLC mapping is the only practical constraint, but there is no explicit cross-check.
- ❌ Equipment slot side effects ([06-equipment](06-equipment.md)).
- ❌ Regulation.bin version drift.
- ❌ Save-side allocator failure prediction (see §16).
- ❌ Spell / gesture / key item loadout.
- ❌ Character stats / level.

Each of these gaps is `needs verification` when extending the schema or on user reports.

---

## 18. Versioning and forward compatibility

| Aspect | Rule |
|---|---|
| `schema` field | Hard `"saveforge.build-template"`. Other values → reject. |
| `version` field | `1 ≤ v ≤ SchemaVersion`. A future v2 must be backward-compatible or a Major bump. |
| Breaking change policy | Bump `SchemaVersion`. Old templates load read-only with an optional migration path. |
| Unknown fields in metadata | `encoding/json` ignores them (tag-based unmarshal). |
| Unknown keys in `sections` | The v1 parser ignores them (the TemplateSections struct has fixed tagged fields). |
| Forward-compat mode | `ImportPreviewOptions.Mode = "<unknown>"` → warning `unknown_mode`; preview proceeds like append. Apply with an unknown mode → Go error. |
| Hash / integrity check | ❌ none. Template files are plain JSON; the user can edit them by hand (and is encouraged to in §19). |
| Schema migration tool | ❌ none. v2+ should provide a migration utility. |

`needs verification`: there are no tests for `version=2` in the current code (`SchemaVersion=1` is the only accepted one), so the forward-compat rules are design intent, not execution-verified.

---

## 19. Local library (Phase E)

A per-user local store of templates on disk.

### 19.1. Disk layout

- **Root**: `$UserConfigDir/EldenRing-SaveEditor/templates/`
  - macOS: `~/Library/Application Support/EldenRing-SaveEditor/templates/`
  - Linux: `~/.config/EldenRing-SaveEditor/templates/`
  - Windows: `%APPDATA%\EldenRing-SaveEditor\templates\`
- **Directory mode**: 0700 (created on first use).
- **One template = one file** `<sanitized-name>-<id-tail>.json`, mode 0644.
- **Index** `_index.json` (`LibraryIndexVersion = 1`) — metadata only (id, name, description, tags, filename, timestamps, item counts). Never raw save data.
- **Atomic writes** — every file written as `.saveforge-tmp-*` + fsync + rename (`atomicWriteFile`). A crash during a write leaves the previous file untouched.
- The directory **must not** be reused for settings or other data.

### 19.2. Recovery semantics

- **Missing `_index.json`** → an empty index (not an error). A user dropping files in manually must call `RebuildIndex` (frontend: the Refresh button).
- **Corrupt `_index.json`** → auto-rebuild from the directory contents (`rebuildIndexLocked`). Unparsable / non-validating files are skipped (left on disk).
- **The rebuild preserves the ID and `CreatedAt`** for files whose filename matches the previous index — the UI keyed by ID is stable across recovery.

### 19.3. App endpoints (Phase E)

| Method | Purpose | Mutates save/workspace? |
|---|---|---|
| `SaveBuildTemplateToLibrary(sessionID, opts)` → `LibraryTemplateEntry` | Build from the active workspace + save. | ❌ |
| `ListBuildTemplateLibrary()` → `[]LibraryTemplateEntry` | Entries from the index sorted newest-first. | ❌ |
| `PreviewBuildTemplateFromLibrary(id)` → `LoadedTemplatePreview` | Load + validator dry-run; returns the JSON for the Apply round-trip. | ❌ |
| `ApplyBuildTemplateFromLibrary(sessionID, id, opts)` → `ApplyTemplateResult` | Delegates to `ApplyBuildTemplateToWorkspaceJSON`. RAM-only workspace mutation. | workspace yes, save no |
| `DeleteBuildTemplateFromLibrary(id)` | Delete the file + index entry. | ❌ |
| `RenameBuildTemplateInLibrary(id, name, description, tags)` → `LibraryTemplateEntry` | Update metadata in the file and index; bump `updatedAt`. | ❌ |
| `ExportLibraryBuildTemplateToFile(id)` → `BuildTemplateExportResult` | Save-file dialog + copy to the chosen path. Cancel → empty Path. | ❌ |
| `RebuildBuildTemplateLibraryIndex()` → `[]LibraryTemplateEntry` | Rescan the directory, rebuild the index. | ❌ |
| `GetBuildTemplateLibraryPath()` → `string` | Absolute path of the directory (the UI uses it in the empty-state and footer). | ❌ |

### 19.4. Invariants

- **Apply from the library = RAM-only** (delegates to `ApplyBuildTemplateToWorkspaceJSON`).
- **`SaveInventoryWorkspaceChanges` is never called from a library action.** The user must click Save changes.
- **Delete and Rename touch only the library** — the workspace and save are not touched.
- **Lazy init** via `App.ensureTemplateLibrary` — unit tests can inject a temporary directory without `DefaultTemplateLibraryDir`.
- **Settings export is NOT in scope** — it is a separate future feature, it must not share this directory.

### 19.5. Manual file management

**The template files on disk are the source of truth**; `_index.json` is a metadata cache. The user can:

- Copy `.json` templates from another computer into the directory.
- Manually edit `metadata.name` / `metadata.tags`.
- Pull templates out to archive them.

After manual changes, the **Refresh** button calls `RebuildBuildTemplateLibraryIndex`:

- Scans `*.json` files.
- Parses + validates every candidate; corrupt / non-validating ones are silently skipped (left on disk).
- Atomically overwrites `_index.json`.
- Preserves the ID and `createdAt` for files matching the previous index.

### 19.6. Frontend (Phase E)

- The `Export Template ▾` dropdown gains an item `Template Library…` opening `TemplateLibraryModal`.
- `ExportTemplateModal` gains an optional `onSavedToLibrary` callback + a **Save to local library** button next to the existing **Export JSON file**.
- `TemplateLibraryModal` per-row actions: Preview, Apply, Export, Rename (inline), Delete (a custom React confirm — no native dialog, so the tests can run under jsdom).
- Header: a **Refresh** button calling `RebuildBuildTemplateLibraryIndex`. The empty-state + footer expose the library path.
- Apply from the library: `useInventoryWorkspace.replaceSnapshot(result.Workspace)` + toast *"Template applied to workspace. Click Save changes to persist."*

---

## 20. Test coverage

| File | Test count | What it locks |
|---|---|---|
| `backend/templates/schema_test.go` | 13 | JSON round-trip, forbidden field exclusion, AoW pointer omitempty, validator reject paths (schema/version/empty/zeroes/container mismatch). |
| `backend/templates/export_test.go` | 15 | Happy path inv/storage/mixed, order preserved, position normalization warning, custom AoW emit, none/missing/shared AoW handling, zero baseID/quantity error, pending fields ignored, empty snapshot, `Now` fallback. |
| `backend/templates/import_test.go` | 22 | Parse round-trip, schema reject paths, preview happy path, name mismatch warning, unknown item/infusion/upgrade range, AoW on non-weapon, AoW not ash category, AoW incompatible (known), AoW compat unknown (fail-closed), mode whitelist. |
| `backend/templates/library_test.go` | 14 | Save/load/delete/rename, index recovery (missing/corrupt), unique filename, reject invalid, export to file, atomic write cleanup, default dir. |
| `app_templates_test.go` | 29 | App-level glue: export JSON/file, preview JSON/file, apply JSON/file, capacity preflight, file dialog cancel, rollback after `AddItem` failure. |
| `app_templates_library_test.go` | 17 | Library-bound App methods (Save/List/Preview/Apply/Delete/Rename/Export from library, `RebuildIndex`, `GetBuildTemplateLibraryPath`). |
| `frontend/src/components/templates/__tests__/ExportTemplateModal.test.tsx` | — | UI behavior of the export modal. |
| `frontend/src/components/templates/__tests__/ImportTemplatePreviewModal.test.tsx` | — | UI behavior of the preview/apply modal. |
| `frontend/src/components/templates/__tests__/TemplateLibraryModal.test.tsx` | — | UI behavior of the library modal. |

Total: **110 Go tests** (counted from `grep -c "^func Test"` in 6 files) + 3 frontend test files. `needs verification`: the number of scenarios in the frontend tests is not enumerated per-test in this document — the full list can be obtained via `grep "test(" frontend/src/components/templates/__tests__/*.test.tsx`.

---

## 21. Known limits / needs verification

| # | Area | Status |
|---|---|---|
| L1 | Affinity gating in preview (Heavy vs Standard infusion variants) | `needs verification` — preview validates compat only by `wepType`, not by infusion variant. See [54-ash-of-war §22.L1](54-ash-of-war.md). |
| L2 | DLC wepType 69/94/95 | `aow_compat_unknown` fail-closes in preview/apply (see §9.5). The UI does not suggest a user-facing solution. |
| L3 | Equipment write API | ❌ none. Apply leaves weapons unequipped. `needs verification` for a future Phase. |
| L4 | Spell / gesture / key item loadout export | ❌ outside v1 scope. Reserved for `character.profile` v2+. |
| L5 | Character stats / level / attributes | ❌ outside v1 scope. Reserved. |
| L6 | DLC item availability cross-check at apply | `needs verification` — `unknown_item` flags individual items, no global "needs DLC X" warning. |
| L7 | Save-side allocator failure prediction | `needs verification` — capacity preflight does not check `NextArmamentIndex` / `NextAoWIndex`. Details in [54-ash-of-war §15](54-ash-of-war.md). |
| L8 | Forward-compat `version=2` tests | `needs verification` — no tests (SchemaVersion=1 is the only accepted one). |
| L9 | Apply rollback matrix | `needs verification` — happy path + capacity + structure invalid are covered, but a full Validation slice restoration test is missing. |
| L10 | `replace-inventory` / `replace-all` modes | Reserved in `ApplyTemplateOptions.Mode`, but **not implemented**. Apply with another mode → Go error. |
| L11 | Cross-platform portability (PS4 vs PC) | `needs verification` — the schema is format-agnostic, but there is no explicit cross-check at apply. |
| L12 | Frontend modal test coverage | Three test files exist (Export/Import/Library), but the scenario matrix is not enumerated in this document. |

---

## 22. Cross-references

- [03-gaitem-map](03-gaitem-map.md) — GaItem binary model and handle prefix semantics.
- [06-equipment](06-equipment.md) — equipped gear (outside Build Template scope).
- [07-inventory](07-inventory.md) / [10-storage](10-storage.md) — inventory / storage section model.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator capacity rules at save.
- [36-inventory-categories-game-order](36-inventory-categories-game-order.md) — item categories (DB mapping).
- [37-character-presets](37-character-presets.md) — a separate mechanism, different scope.
- [43-transactional-item-adding](43-transactional-item-adding.md) — the Add Items pipeline used at save.
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — acquisition sort, why `acquisitionIndex` is excluded from the template.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — prior art for workspace mutations.
- [54-ash-of-war](54-ash-of-war.md) — the canonical chapter for AoW (sentinels, write paths, compat matrix, workspace modal).

---

## 23. Sources

- Schema/pipeline code: `backend/templates/schema.go`, `backend/templates/export.go`, `backend/templates/import.go`, `backend/templates/library.go`.
- App glue: `app_templates.go`.
- Workspace / editor: `backend/editor/workspace.go` (`EditableItem`, `InventoryWorkspaceSnapshot`, constants `AoWStatus*`), `backend/editor/add.go` (`AddItemSpec`), `backend/editor/weapon.go` (`WeaponPatch`, `UpdateWeapon`), `backend/editor/save.go` (`ApplyWorkspaceSave`).
- DB: `backend/db/db.go` (`GetItemDataFuzzy`, `InfuseTypes`, `IsAshOfWarCompatibleWithWeapon`).
- Capacity: `backend/core/offset_defs.go` (`CommonItemCount`, `StorageCommonCount`).
- Frontend: `frontend/src/components/templates/ExportTemplateModal.tsx`, `ImportTemplatePreviewModal.tsx`, `TemplateLibraryModal.tsx`, `frontend/src/components/SortOrderTab.tsx`.
- Tests: `backend/templates/{schema,export,import,library}_test.go`, `app_templates_test.go`, `app_templates_library_test.go`, `frontend/src/components/templates/__tests__/*.test.tsx`.
- History: commits `feat(templates): build template schema + exporter (Phase A)`, `feat(templates): export modal + dropdown (Phase B)`, `feat(templates): import preview (Phase C)`, `feat(templates): apply to workspace (Phase D)`, `feat(templates): local library (Phase E)` — exact SHAs available via `git log --grep="templates"`.
