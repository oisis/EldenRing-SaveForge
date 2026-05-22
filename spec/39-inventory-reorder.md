# 39 — Inventory Reorder — historical design note

> **Type**: Historical project decision log (superseded by current implementation)
> **Status**: 📜 Historical / partial — serves as a log of project decisions, **not** as current technical spec.
> **Extracted from**: `docs/ROADMAP.md` (2026-05-03 cleanup).

---

## Status banner

> ⚠️ **This document is historical.** It was originally a plan for the "Inventory Reorder / Sort Order" feature in 5 phases (0-4). The implementation took **direction** from it, but in several places went a different way:
>
> - **The reorder algorithm** described in this document (Phase 1, stride-1: `base + i`) is **incorrect**. The actual implementation uses **stride-2 with an even base** — the discovery has been documented and is described canonically in [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).
> - **UI ↔ transfer mechanics** (inventory drag&drop, Inventory ↔ Storage transfer, two-column 5×6 layout, sort mode dropdowns, rehandle path) were partially described in older plan iterations — the current canonical description remains in [53-inventory-storage-transfer](53-inventory-storage-transfer.md). The current UI component is `frontend/src/components/SortOrderTab.tsx` (workspace-session model), **not necessarily** exactly identical to the planned `InventoryGrid.tsx` toggle in `InventoryTab.tsx`.
> - **Per-character order persistence** (Phase 4) is **not implemented** — `backend/vm/preset.go::CharacterPreset` has no `InventoryOrder` field (verified 2026-05-19).
>
> **Current source of truth**:
> - Reorder algorithm + acquisition index semantics: [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md)
> - Inventory data model: [07-inventory](07-inventory.md)
> - Storage data model: [10-storage](10-storage.md)
> - Inventory ↔ Storage transfer and Sort Order UI: [53-inventory-storage-transfer](53-inventory-storage-transfer.md)
> - Add Items pipeline (capacity + reorder integration): [43-transactional-item-adding](43-transactional-item-adding.md)
>
> The document is preserved as a **project decision log** for future implementers who want to understand why the feature looks the way it does.

---

## Why this document is historical

The plan from May 2026 described:

1. **The `acquisition_index` editing mechanic** — assuming stride-1 and the hypothesis that Phase 0 in-game verification would confirm the semantics. The Phase 0 verification was performed (sentinel v1/v2/v3 on Steam Deck) and **discovered that stride-1 does not work** — the game sorts by `acqIdx >> 1`, so adjacent pairs are swapped. Stride-2 with an even base was adopted as the final algorithm.
2. **The full game-styled `InventoryGrid.tsx`** as a List / Grid view toggle in `InventoryTab.tsx`, with `@dnd-kit/sortable`, 64×64 tiles, a custom theme and a context menu. Implementation **did not go that way**: instead a separate `SortOrderTab.tsx` was created with a two-column 5×6 layout (Inventory + Storage), a workspace-session model (`useInventoryWorkspace` hook), and no sort dropdowns.
3. **Persistence of `inventoryOrder` in `CharacterPreset`** — optional Phase 4 (2-3h). Not implemented; current presets preserve items + add settings + world + character core, but **not** the per-category acquisition order.

---

## Current source of truth

A reader looking for **current** Inventory Reorder semantics should start with:

| What | Where |
|---|---|
| Stride-2 algorithm (write-side) | [52 → Stride-2 write model](52-acquisition-sort-stride2.md#stride-2-write-model) |
| Bucket-collision guard | [52 → Bucket collision guard](52-acquisition-sort-stride2.md#bucket-collision-guard) |
| Inventory reorder path (legacy `ReorderInventory`) | [52 → Inventory reorder path](52-acquisition-sort-stride2.md#inventory-reorder-path) |
| Storage reorder path (legacy `ReorderStorage`) | [52 → Storage reorder path](52-acquisition-sort-stride2.md#storage-reorder-path) |
| Workspace save integration (`writeContainerLayout`) | [52 → Workspace/editor save integration](52-acquisition-sort-stride2.md#workspaceeditor-save-integration) |
| Inventory data model and `Index` field | [07 → Binary structures](07-inventory.md#binary--runtime-structures) |
| Storage data model and differences vs Inventory | [10-storage](10-storage.md) |
| `InvEquipReservedMax = 432` (reserved equipment range) | [52 → Acquisition index semantics](52-acquisition-sort-stride2.md#acquisition-index-semantics) + [35 → Required invariants](35-gaitem-allocator-invariants.md#required-invariants) |
| Sort Order UI flow (workspace, drag&drop, transfer) | [53-inventory-storage-transfer](53-inventory-storage-transfer.md) |

---

## Phase status table

| Phase in plan | Original plan | Actual implementation status | Current source of truth | Notes |
|---|---|---|---|---|
| **0** — In-game verification (CRITICAL, 1-2h) | Save with 5 weapons, hex-edit `acquisition_index` → 1000..1004, deploy to Steam Deck, verify sorting | ✅ **Done** — sentinel v1/v2/v3 discovery documented | [52 → Stride-2 write model → Discovery](52-acquisition-sort-stride2.md#stride-2-write-model) | Verification result: the stride-1 from Phase 1 was **rejected**; stride-2 with an even base was adopted as final |
| **1** — Backend API + 2 sort modes (3-5h) | `GetInventoryOrder`, `ReorderInventory`, `SortInventory(mode)`. Algorithm: stride-1 `base + i`, `base = max(NextAcquisitionSortId, 1000)`. Categories: weapons, talismans, head/chest/arms/legs. | ✅ **Implemented** — with modifications: the algorithm = **stride-2** (not stride-1), `base = max(NextAcquisitionSortId, InvEquipReservedMax+2)` rounded up to even; `SortInventory(mode)` as a bulk-sort **does not exist** (frontend manages order in workspace, backend only writes final `Index`). Implemented functions: `app_inventory_order.go::ReorderInventory`, `GetInventoryOrder`, `ReorderStorage`, `GetStorageOrder`, `GetWeaponInventoryOrder`, `ReorderWeaponInventory`. | `app_inventory_order.go` + [52](52-acquisition-sort-stride2.md) | The stride-1 in the plan is **incorrect** (sentinel v2 — adjacent pairs swapped by the game). |
| **2** — Erdb import (4 sort modes, 2-3h) | Import `weight`, `attackBasePhysics`, `sortGroupId` into `ItemData` from `tmp/erdb/1.10.0/EquipParam*.csv` via `scripts/import_erdb.go`. Bulk-sort modes: weight, attackPower, sortGroupId, upgradeLevel. | ⚠️ **Partial** — `InventoryOrderItem` DTO (`app_inventory_order.go:14-29`) contains `Weight`, `SortId`, `SortGroupId` fields; **`needs verification`** whether they are actively populated from `data.ItemData` and whether the bulk-sort UI would ever be used (sort dropdowns in the current UI do not exist). | `app_inventory_order.go:14-29` + `data.ItemData` | The `SortOrderTab.tsx` component does not show sort dropdowns — the workspace UI uses the `Position` field, not acquisition. |
| **3** — Game-style grid + drag & drop (8-12h) | `InventoryGrid.tsx` as a List/Grid view toggle in `InventoryTab.tsx`. `@dnd-kit/core` + `@dnd-kit/sortable`. 64×64 tiles, custom theme. Sort dropdown with 6 modes. | ❌ **Not implemented in the described form** — the current UI is `frontend/src/components/SortOrderTab.tsx` (a separate tab, **not** a toggle in InventoryTab), a two-column 5×6 layout (Inventory + Storage), a workspace-session model without sort dropdowns. **`@dnd-kit/sortable` is not installed** in `frontend/package.json` (verified 2026-05-19); the drag&drop in the current `SortOrderTab.tsx` is implemented with native HTML drag events. | `frontend/src/components/SortOrderTab.tsx` + [53-inventory-storage-transfer](53-inventory-storage-transfer.md) | The actual UI is **closer** to the Phase 3 vision than to no feature, but **different** in detail — see spec/53 for the current description. |
| **4** (optional) — Per-character order persistence (2-3h) | Add `inventoryOrder: { weapons: [handle1, ...], armor: [...], talismans: [...] }` to `CharacterPreset`. Apply: after `AddItemsToCharacter` → `ReorderInventory` per category. | ❌ **Not implemented** — `backend/vm/preset.go::CharacterPreset` (verified 2026-05-19) has no `InventoryOrder` or `Order` field or analogue. The preset preserves: `Inventory []PresetItem`, `Storage []PresetItem`, `AddSettings`, `World`, `CharacterPresetCore` — without acquisition order. | `backend/vm/preset.go:15-24` (`CharacterPreset`) | After preset import the items get fresh stride-2 `Index` (from `AddItemsToCharacter` Phase 3 → counters advance); the original order from the source save is **lost**. |

---

## What remains useful

Sections of this plan that have historical/project value **despite** superseded mechanics:

- **"Mechanika" section** (lines 17-23 of the original) — preliminary analysis that `acquisition_index` is a field at offset `+8`, a global counter, controls the "Acquisition Order" sort. All these facts remain correct. Only **the way new values are assigned** (stride-1) turned out to be wrong.
- **"Pułapki" section** (lines 27-32) — four numbered pitfalls regarding `InvEquipReservedMax = 432`, per-category sorting, stackable items, missing `Weight`/`AttackPower`/`SortGroupId` data — all valuable as early-stage risk assessment. Current canonical in [52](52-acquisition-sort-stride2.md) and [07](07-inventory.md).
- **"Otwarte pytania" section** (lines 146-152) — some resolved (Phase 0 verification with sentinel v1/v2/v3 → CANCEL stride-1 → stride-2), others still open (e.g. reset behavior, persistence after WriteSave). See "Historical notes" below.

---

## Superseded mechanics

| Original statement in 39 | Status |
|---|---|
| "stride-1 `base + i`, `base = max(NextAcquisitionSortId, 1000)`" | ❌ **superseded** — the actual algorithm: stride-2 + even base `>= InvEquipReservedMax + 2`. See [52 → Stride-2 write model](52-acquisition-sort-stride2.md#stride-2-write-model). |
| "Phase 0 CRITICAL — if `acquisition_index` does not control sorting, CANCEL the feature" | ✅ Verification done, hypothesis partially true: `acquisition_index` controls the sort, but the game uses `acqIdx >> 1` as the bucket key, not the full value. See [52 → Discovery](52-acquisition-sort-stride2.md#stride-2-write-model). |
| "Phase 1 — `SortInventory(mode)` as a bulk-sort API" | ❌ **not implemented** — the frontend manages order via the workspace `Position`; the backend only writes the final `Index`. |
| "Phase 3 — `InventoryGrid.tsx` toggle in `InventoryTab.tsx`, 64×64 tiles, sort dropdown with 6 modes" | ❌ **not implemented in the described form** — the current `SortOrderTab.tsx` is a separate tab, a 5×6 grid, no sort dropdowns. See [53-inventory-storage-transfer](53-inventory-storage-transfer.md) for the current UI. |
| "Phase 4 — `inventoryOrder` in `CharacterPreset`" | ❌ **not implemented** — the preset format does not contain acquisition order. |

---

## Links to canonical chapters

- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — **canonical** for the reorder algorithm, stride-2, bucket collision, Inventory/Storage paths, workspace integration.
- [07-inventory](07-inventory.md) — **canonical** for the Inventory data model, the `InventoryItem` 12 B layout, `NextAcquisitionSortId`/`NextEquipIndex` counters.
- [10-storage](10-storage.md) — **canonical** for the Storage data model and the differences vs Inventory.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — **canonical** for Inventory ↔ Storage transfer and the Sort Order UI flow (with a note that the current `SortOrderTab.tsx` does not expose sort dropdowns — see "Direction naming and UI caveats" in [52](52-acquisition-sort-stride2.md#direction-naming-and-ui-caveats)).
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator/capacity/counter invariants; reserved equipment range `InvEquipReservedMax = 432`.
- [43-transactional-item-adding](43-transactional-item-adding.md) — Add Items pipeline; `Index` assignment on add (single-stride, not stride-2).

---

## Historical notes

Sections of this plan that we want to preserve **verbatim** as a log of project decisions (they are NOT current spec):

### From the original "Faza 0 — Weryfikacja in-game" section

> Test:
> 1. Save with 5 weapons in slots
> 2. Hex-edit `acquisition_index` of the 5 weapons → 1000, 1001, 1002, 1003, 1004
> 3. Deploy to Steam Deck → load the save
> 4. In-game: switch the sort to Acquisition Order
> 5. Check whether the weapons are in the order [1000..1004] ascending
> 6. Pick up a new item in-game → verify that our 5 weapons keep their order (the new item lands at the end as `next_acquisition_sort_id`)

**Actual execution**: step 2 used 65 talismans instead of 5 weapons; v1/v2/v3 tests discovered the bucket key `acqIdx >> 1`. Result documented in [52](52-acquisition-sort-stride2.md).

### From the original "Otwarte pytania" section

List of questions left in the original:

1. ~~Phase 0 verification: does the user prepare the save with the hex-edit, or does the editor produce the test save?~~ → **resolved**: the user prepared it, verification took place via Steam Deck deploy.
2. ~~Category scope: weapons + armor + talismans only, or also shields / AoW / Ashes?~~ → **resolved**: 6 tabs (`weapons` = melee + ranged + shields, `talismans`, `head`, `chest`, `arms`, `legs`). AoW / ashes / goods **outside** the reorder scope.
3. **Reset button behavior**: return to `sortGroupId` (default in-game), `acquisition` (original pickup order), or disable after dragging? → **there is no "Reset" button** in the current UI; the workspace session has `Position`, not a sort mode.
4. **Persistence after WriteSave**: keep the custom order forever (until Reset), or invalidate after Add Items? → Add Items advances `NextAcquisitionSortId` — new items land **at the end** of Acquisition ↑; existing order preserved.
5. ~~Reserved slots 0-432: does our reorder skip them (recommended) or re-index?~~ → **resolved**: `ReorderInventory`/`ReorderStorage` starts `base = max(NextAcquisitionSortId, InvEquipReservedMax + 2)` — slots ≤ 432 are untouched.

### From the original "Podsumowanie faz" section

The original estimated-effort table:

| Phase | Original estimated effort |
|---|---|
| 0 In-game verification | 1-2h |
| 1 Backend API + 2 sort modes | 3-5h |
| 2 Erdb import + 4 sort modes | 2-3h |
| 3 UI grid + drag & drop | 8-12h |
| 4 (opt.) Preset persistence | 2-3h |
| **Minimum total (0+1+3)** | **12-19h** |
| **Full total (0-3)** | **14-22h** |

**Actual implementation** took separate development sessions, in a shape different from the plan — see `docs/CHANGELOG.md` for `acquisition`/`sort`/`reorder` entries.

---

## Final document status

- **Current role**: historical project decision log.
- **Updates**: the document is not actively maintained. Any write-path / algorithm / UI changes go to the canonical chapter ([52](52-acquisition-sort-stride2.md), [53](53-inventory-storage-transfer.md), [07](07-inventory.md), [10](10-storage.md)).
- **Value**: a log of project decisions for implementers wishing to understand **why** Sort Order looks the way it does and **what** was rejected (stride-1, `InventoryGrid.tsx` toggle, preset persistence).
- **Status in the book**: remains in the main `spec/` directory as a historical design note (superseded by 52/53).
