# 52 — Acquisition Sort Stride-2: write model for in-game ordering

> **Type**: Binary write-path spec + design rationale
> **Status**: ✅ canonical, implemented. Algorithm verified empirically (sentinel v1/v2/v3 in-game on Steam Deck) and confirmed in code in `app_inventory_order.go::ReorderInventory/ReorderStorage` and `backend/editor/save.go::writeContainerLayout`. Allocator/capacity/counter invariants are described canonically in [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md); the Inventory/Storage data model in [07-inventory](07-inventory.md), [10-storage](10-storage.md).
> **Scope**: Write-side model for assigning `InventoryItem.Index` (acquisition index) so that the game renders items in the desired order under the "Acquisition Order ↑" sort. Covers: stride-2 base assignment, bucket-collision guard, Inventory + Storage paths, integration with the workspace save layer. The in-game **sort** mechanic (how the game interprets `Index` as the bucket key) and the discovery story are here — this is the canonical home for that mechanism.

---

## Chapter goal

The chapter describes:

- what the acquisition index (`InventoryItem.Index`) is from the perspective of the in-game sort;
- how the game interprets the `Index` value — the bucket key `acqIdx >> 1` (discovered empirically);
- why naive stride-1 does not work and how stride-2 with an even base guarantees success;
- the defensive guard that detects bucket collisions before mutation;
- three places in the code that use stride-2 (Inventory reorder, Storage reorder, workspace save layer);
- which item categories are in the reorder scope and which are out;
- how reorder cooperates with `pushUndo` and why it does not use `SnapshotSlot`/`ValidatePostMutation`.

What the chapter does **NOT** do:

- Does not describe `Index` as a binary field or the inventory record layout — [07-inventory → Binary structures](07-inventory.md#binary--runtime-structures).
- Does not describe the semantics of Inventory ↔ Storage transfer (the fresh `Index` assignment in `MoveItemsBetweenContainers` uses single-stride, not stride-2) — [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- Does not describe `Index` assignment on Add Items — [43-transactional-item-adding](43-transactional-item-adding.md).
- Does not describe allocator/counter invariants or capacity check — [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).
- Does not describe the SortOrder UI flow (drag-and-drop, sort-mode dropdowns) — the full frontend semantics remain in [53-inventory-storage-transfer → Frontend](53-inventory-storage-transfer.md) (with the caveat that the current `SortOrderTab.tsx` does not expose sort dropdowns — see "Direction naming and UI caveats" in this chapter).

---

## Status

- canonical
- implemented (three write-side paths: legacy `ReorderInventory`/`ReorderStorage` + workspace `writeContainerLayout`)
- source-of-truth: backend code + empirical in-game verification (sentinel tests)
- Storage in-game verification: `needs verification` (a fresh in-game / Steam Deck test is not current)
- allocator details: **chapter 35**

---

## Source of truth in code

| Topic | Files / functions | Note |
|---|---|---|
| `InventoryItem.Index` field | `backend/core/structures.go:118-122` | u32 at offset `+8` in the 12 B inventory/storage record |
| Reserved equipment range | `backend/core/offset_defs.go:341` (`InvEquipReservedMax = 432`) | acquisition values ≤ 432 reserved for equipment slots |
| Legacy Inventory reorder | `app_inventory_order.go:255-417` (`ReorderInventory`) | stride-2 + bucket-collision guard, `pushUndo` before mutation |
| Legacy Storage reorder | `app_inventory_order.go:439-...` (`ReorderStorage`) | mirror of Inventory, uses `slot.Storage.NextAcquisitionSortId` |
| Workspace save (newer) | `backend/editor/save.go::writeContainerLayout` (lines 518-..., `baseAcq` at line 566) | stride-2 with `reservedAcq` skip-over for passthrough records (pre-existing acquisition values must not collide with the new base) |
| Read path | `app_inventory_order.go:80-149` (`GetInventoryOrder`), `:160-238` (`GetStorageOrder`) | sorts `Index` ascending, filters by the tab's category |
| Tab → categories map | `app_inventory_order.go:31-39` (`inventoryOrderTabs`) | 6 tabs: `weapons` (= `melee_armaments` + `ranged_and_catalysts` + `shields`), `talismans`, `head`, `chest`, `arms`, `legs` |
| Technical placeholder filter | `app_inventory_order.go:51-58` (`invUnarmedBaseID = 0x0001ADB0`, `isWeaponOrderTechnical`) | 3 "Unarmed" instances in `CommonItems` as an empty-hand placeholder; excluded from Sort Order |
| Item DTO for UI | `app_inventory_order.go:14-29` (`InventoryOrderItem`) | fields: `Handle`, `ItemID`, `Name`, `Category`, `AcquisitionIndex`, `Weight`, `SortId`, `SortGroupId`, `CurrentUpgrade`, `MaxUpgrade`, `InfusionName`, `IconPath`, `IsTechnical` |

---

## Mental model

```
in-game render
"Acquisition Order ↑"
        ▲
        │  sort key = acqIdx >> 1   (game-side, discovered empirically)
        │
        │  bucket(item) = InventoryItem.Index >> 1
        │  → tied items within a bucket are resolved by
        │    another key (probably sortGroupId / handle)
        │
slot.Data
 │
 ├── Inventory.CommonItems[i].Index = u32 ← write-side stride-2 base + pos*2
 └── Storage.CommonItems[i].Index   = u32 ← analogously
```

**Key observation**: the game **does not** compare the full `Index` value (32 bits), only `Index >> 1`. Adjacent pairs `(k, k+1)` with even `k` end up in the same bucket and are swapped by the game's tie-breaker. Stride-2 with an even base guarantees bucket uniqueness: `(base + 2*i) >> 1 = base/2 + i` is strictly increasing.

---

## Acquisition index semantics

`InventoryItem.Index` (offset `+8`, u32) carries a global acquisition counter — the game writes it on every pickup/creation of an inventory entry, monotonically increasing with `NextAcquisitionSortId` (the counter at the end of the section, see [07-inventory → Capacity and counters](07-inventory.md#capacity-and-counters)).

Consequences:

- The field is **per-record** — every inventory/storage slot holds its own value.
- In a sort mode **other than Acquisition Order** (Weight, Type, Attack Power, Alphabetical) the game computes the sort key at runtime from `regulation.bin` (`EquipParamWeapon.sortGroupId/sortId/...` parameters); a change to `Index` in the save is invisible to them.
- Values ≤ `InvEquipReservedMax = 432` are **reserved for equipment-equivalent slots** (`backend/core/offset_defs.go:341`); reorder MUST use `Index > 432`.
- Values `>= 10000` cause a **game crash on load** (sentinel v1 discovery — see below). The algorithm starts at `NextAcquisitionSortId` (typically 500–2000 for a long-running save), so in practice it does not approach that limit.

---

## Stride-2 write model

### Discovery: in-game sentinel tests

Empirical in-game verification on Steam Deck (talismans tab, 65 items):

| Test | Assignment | Result |
|---|---|---|
| Sentinel v1 (hardcoded 10000-10012) | `10000, 10001, ..., 10012` | **Game crashes** on save load |
| Sentinel v2 (safe, stride-1) | `minAcq, minAcq+1, ..., minAcq+N` | Save load OK, **adjacent pairs swapped** |
| Sentinel v3 (stride-2) | `base, base+2, ..., base+N*2` (even `base`) | Save load OK, **order correct** |

Conclusion: the game sorts by `acqIdx >> 1`. Stride-1 with any base `k` produces collisions: `k >> 1 == (k+1) >> 1` when `k` is even.

### Algorithm

```go
// base: starts at NextAcquisitionSortId; ensure even > InvEquipReservedMax (432).
base := slot.Inventory.NextAcquisitionSortId
if base <= uint32(core.InvEquipReservedMax) {
    base = uint32(core.InvEquipReservedMax) + 2  // 434 — minimum safe even value
}
if base%2 != 0 {
    base++
}

// Assignment: item at position pos receives Index = base + pos*2.
for pos, handle := range orderedHandles {
    newIdx := base + uint32(pos)*2
    // write to slot.Data[off+8:] and slot.Inventory.CommonItems[j].Index
}

// Advance NextAcquisitionSortId (monotonic only).
expectedMax := base + uint32(len(orderedHandles)-1)*2
newNextAcq := expectedMax + 1
```

### Proof of bucket uniqueness

For an even `base` and position `i`:

```
bucket(i) = (base + 2*i) >> 1
          = base/2 + i
```

Because `base` is even, `base/2` is an integer. `base/2 + i` is strictly increasing in `i` → no two positions share a bucket.

---

## Bucket collision guard

`ReorderInventory` contains a defensive check before any mutation (`app_inventory_order.go:371-378`):

```go
shiftKeys := make(map[uint32]int, len(orderedHandles))
for pos := range orderedHandles {
    key := (base + uint32(pos)*2) >> 1
    if prevPos, dup := shiftKeys[key]; dup {
        return fmt.Errorf("stride-2 reorder: bucket collision at key=%d positions %d and %d; refusing", key, prevPos, pos)
    }
    shiftKeys[key] = pos
}
```

**Guard status**: unreachable with a correct even base + stride-2 (proof above). Purpose: regression lock-in — if someone later changes the `base/stride` logic, this guard will stop a silent regression (e.g. stride-1 returning to the save).

An analogous pre-mutation guard also exists in `ReorderStorage` (mirror of the same logic for `slot.Storage`).

---

## Inventory reorder path

`(*App).ReorderInventory(charIdx int, tab string, orderedHandles []uint32) error` (`app_inventory_order.go:255`).

Sequence:

1. **Args validation**: tab in `inventoryOrderTabs`, `a.save != nil`, `charIdx ∈ [0, 10)`, `slot.Version > 0`, `len(orderedHandles) > 0`.
2. **Duplicate handle check**: `seen` map; reject error.
3. **Locate handles**: iterate `CommonItems`, match by handle, verify `itemData.Category ∈ tabCategorySet(tab)`, verify `!isWeaponOrderTechnical` (for the weapons tab).
4. **Complete list check**: the count of `orderedHandles` must equal the number of eligible items for the tab in the slot — partial lists rejected.
5. **Compute stride-2 base**: round up `NextAcquisitionSortId` to an even value > `InvEquipReservedMax`.
6. **Bucket collision guard**: pre-mutation check (above).
7. **`pushUndo(charIdx)`** — user-facing undo.
8. **Apply stride-2 indices**: write to `slot.Data[off+8:]` (binary) and `slot.Inventory.CommonItems[j].Index` (runtime); per handle from `orderedHandles`.
9. **Advance counters**: `slot.Inventory.NextAcquisitionSortId = expectedMax + 1`, `slot.Inventory.NextEquipIndex = max(current, newNextAcq)`, plus binary write-back at `NextEquipIndexOff()` (when offset > 0).

Arguments/errors are deterministic — no step leaves the slot in a half-modified state. Mutations (steps 8-9) are executed only after positive validation (steps 1-6).

### Convenience wrappers

- `GetWeaponInventoryOrder(charIdx int)` (`:607`) → delegates to `GetInventoryOrder("weapons")`.
- `ReorderWeaponInventory(charIdx int, orderedHandles []uint32)` (`:613`) → delegates to `ReorderInventory("weapons", ...)`.

---

## Storage reorder path

`(*App).ReorderStorage(charIdx int, tab string, orderedHandles []uint32) error` (`app_inventory_order.go:439`).

Identical semantics to Inventory, with differences:

| Aspect | Inventory | Storage |
|---|---|---|
| Target slice | `slot.Inventory.CommonItems` | `slot.Storage.CommonItems` |
| Binary start offset | `slot.MagicOffset + InvStartFromMagic` | `slot.StorageBoxOffset + StorageHeaderSkip` |
| Counter source | `slot.Inventory.NextAcquisitionSortId` | `slot.Storage.NextAcquisitionSortId` |
| Counter write-back | `slot.Inventory.nextEquipIndexOff` | `slot.Storage.nextEquipIndexOff` |
| Post-mutation reconcile | none (`Inventory.CommonItems` is a physically full array) | none (see `needs verification` in [10-storage](10-storage.md)) |

> ⚠️ **Storage in-game verification — `needs verification`**: backend Storage reorder has full tests (`app_storage_order_test.go`, 9 tests), but the latest in-game sanity check on Steam Deck (that "Acquisition Order ↑" in the box in-game matches the editor's preview) **is not fresh**. See [10-storage → Known limits](10-storage.md#known-limits--needs-verification).

---

## Workspace/editor save integration

The newer write-side path used by `SaveInventoryWorkspaceChanges` (`app_inventory_session.go:192`) → `backend/editor/save.go::ApplyWorkspaceSave` → `writeContainerLayout` (`save.go:518-...`).

This path **rebuilds** the entire `CommonItems` section from the workspace snapshot, instead of performing an in-place `Index` rewrite like legacy reorder. Stride-2 is still used (`save.go:565-...`), but with an additional pre-condition:

- **`occupied`** map: pinned slots for `passthrough` records (pre-existing items the workspace does not modify).
- **`reservedAcq`** map: acquisition values already used by passthrough records.
- **`baseAcq`** start: `equip.NextAcquisitionSortId` rounded up to even, > `InvEquipReservedMax`.
- **Skip-over collision loop** (`save.go:573-...`): if `baseAcq + 2*i` collides with `reservedAcq[*]` for any `i ∈ [0, len(editables))`, the base is incremented by 2 and the loop repeats.

Effect: the workspace path preserves passthrough acquisition values without modifying them, and editable items get a fresh base that does not collide with existing values. The same stride-2 algorithm, a different pre-condition.

> ℹ️ Legacy `ReorderInventory`/`ReorderStorage` assumes that **all** eligible items for the tab are in `orderedHandles` (complete list). The workspace path assumes the opposite — only **editable items** are re-indexed, passthrough remain with the original `Index`.

---

## Category filtering and unknown-category behavior

`tabCategorySet(tab)` (`app_inventory_order.go:62`) returns:

- `map[category]bool` for known tabs (`weapons` → `{melee_armaments, ranged_and_catalysts, shields}`; the others — single-category sets).
- `error` for an unknown tab.

`ReorderInventory`/`ReorderStorage` rejects a handle whose `itemData.Category` is outside the tab set (`app_inventory_order.go:308-310`):

```
handle 0x%08X (category %q) does not belong to sort order tab %q
```

`GetInventoryOrder`/`GetStorageOrder` filters items matching the tabCategorySet — items outside the scope are **invisible** in the Sort Order UI and are not subject to reorder. Their `Index` in the binary remains unchanged.

### Categories outside Sort Order

Currently 6 tabs are supported: `weapons` (weapons + shields + ranged + catalysts), `talismans`, `head`, `chest`, `arms`, `legs`. **Outside the reorder scope**:

- `goods` (consumables, materials, key inventory items) — handle prefix `0xB0`
- `ashes` (spirit ashes)
- `ashes_of_war` (AoW gems)
- `crafting_materials`
- `bolstering_materials`
- `tools`
- `info`
- `sorceries`, `incantations`

These categories have their own `Index` in the binary, but their order in the game is not editable via Save Forge.

> ℹ️ **Unarmed placeholder exclusion**: the game keeps 3 "Unarmed" entries (baseID `0x0001ADB0`) in `Inventory.CommonItems` as technical slots for the empty-hand state. `isWeaponOrderTechnical` (`app_inventory_order.go:56`) excludes them from the weapons tab — `GetInventoryOrder("weapons")` does not return them, `ReorderInventory("weapons", [handle Unarmed])` rejects with the error "is a technical placeholder".

---

## Direction naming and UI caveats

This spec describes the **write-side mechanic** — assigning `Index` values so that the game renders items in the desired order under "Acquisition Order ↑". It does not describe UI dropdowns or the ↑/↓ semantics in the current frontend.

> ⚠️ **The current `SortOrderTab.tsx`** (`frontend/src/components/SortOrderTab.tsx`) **does not expose sort dropdowns** Acquisition ↑/Acquisition ↓/Weight/Type. The component uses the workspace API with a workspace-internal `Position` field; the user re-orders by drag-and-drop in the 5×6 grid, and `SaveInventoryWorkspaceChanges` translates `Position` into a stride-2 acquisition Index in `backend/editor/save.go`.
>
> Historical mentions of Acquisition ↑/↓ dropdowns in other documents (e.g. [53-inventory-storage-transfer](53-inventory-storage-transfer.md)) refer to an earlier UI iteration. **`needs verification`** whether the current SortOrderTab is the final UI version or whether reintroducing sort dropdowns is planned (and whether ↑/↓ naming would match the game).

---

## Validation and rollback relation

`ReorderInventory`/`ReorderStorage` performs:

- **`pushUndo(charIdx)`** before mutation (user-facing undo stack).
- **Pre-mutation validation**: duplicate handle, missing handle, wrong category, technical placeholder, incomplete list, bucket collision guard.

**What is missing** (compared with `AddItemsToCharacter`):

- ❌ **No `SnapshotSlot`/`RestoreSlot`** — reorder does not use an internal safety net.
- ❌ **No `ValidatePostMutation`** after reorder — the `duplicate_index` check in the validator is not enforced for this path.

**`needs verification`**: whether this is a conscious decision (reorder modifies only `Index` — not GaItems, not GaMap, not containers — so less can go wrong), or a gap in transactional safety. Argument for intent: pre-mutation validation (steps 1-6) is restrictive enough to prevent entry-state corruption; once all pre-checks pass, mutations are mechanical and should not fail.

Full transactional safety semantics for `AddItemsToCharacter`/`MoveItemsBetweenContainers` — [35 → Transactional safety](35-gaitem-allocator-invariants.md#transactional-safety-and-rollback), [43-transactional-item-adding](43-transactional-item-adding.md).

---

## Test coverage

### Inventory reorder (`app_inventory_order_test.go`)

| Test | What it verifies |
|---|---|
| `TestGetWeaponInventoryOrder_NoSave` (84) | Reject when `a.save == nil` |
| `TestGetWeaponInventoryOrder_InvalidIdx` (92) | Reject when `charIdx` out of range |
| `TestGetWeaponInventoryOrder_EmptySlot` (102) | Handling a slot with `Version == 0` |
| `TestGetWeaponInventoryOrder_ReturnsWeaponsAscending` (111) | `Index` ascending sort |
| `TestGetWeaponInventoryOrder_HidesUnarmed` (136) | Exclusion of technical Unarmed placeholders |
| `TestGetWeaponInventoryOrder_UpgradeInfusionDecoded` (153) | Decoder of `+N` upgrade and infusion name |
| `TestReorderWeaponInventory_NoSave` (178), `_InvalidCharIdx` (186), `_DuplicateHandle` (196), `_MissingHandle` (205), `_UnarmedHandle` (216), `_IncompleteList` (227) | Pre-mutation validation errors |
| `TestReorderWeaponInventory_HappyPath` (236) | Full happy path: reorder changes `Index` according to `orderedHandles` |
| `TestReorderWeaponInventory_DoesNotTouchGaItems` (291) | Reorder does not modify GaItems or GaMap |
| `TestReorderWeaponInventory_StorageHandle` (334) | A storage handle cannot be used in an Inventory reorder |
| `TestGetInventoryOrder_UnknownTab` (421) | Reject unknown tab |
| `TestGetInventoryOrder_Talismans_Items` (433), `_Head_Items` (458) | Sort Order for the other tabs |
| `TestReorderInventory_Talismans_HappyPath` (482), `_HeadOrChest_HappyPath` (513), `_WrongTabHandle_Blocks` (544) | Reorder for talismans / armor + cross-tab rejection |

### Storage reorder (`app_storage_order_test.go`)

| Test | What it verifies |
|---|---|
| `TestReorderStorage_RejectsMissingHandle` (124), `_RejectsDuplicateHandle` (147), `_RejectsIncompleteList` (169), `_RejectsHandleFromInventory` (342) | Pre-mutation validation |
| `TestReorderStorage_DoesNotTouchInventory` (192), `TestInventoryReorder_DoesNotTouchStorage` (403) | Cross-container isolation |
| `TestReorderStorage_PersistsAcquisitionOrder` (267) | Stride-2 persistently written to the binary |
| `TestReorderStorage_DoesNotTouchHandlesOrQty` (316) | Reorder modifies only `Index`, not handle/qty |
| `TestReorderStorage_RoundTripReread` (357) | Save → reorder → write → reload — order preserved |

### Workspace save (R7-related, partial coverage)

- `backend/editor/save_test.go` (if it exists) and `app_inventory_session_test.go` — cover `writeContainerLayout` stride-2 + reservedAcq skip-over. **`needs verification`**: the full map of workspace-path tests was not counted in this phase.

### In-game empirical (Steam Deck)

- Sentinel v1/v2/v3 (talismans, 65 items) — executed at the time of the stride-2 discovery.
- **`needs verification`**: a fresh Storage reorder test in-game (see [10-storage → Known limits](10-storage.md#known-limits--needs-verification)).

---

## Known limits / needs verification

- **Storage Apply in-game verification** — sanity check in-game (Steam Deck) that "Acquisition Order ↑" in the box matches the editor's preview — `needs verification`.
- **Sort dropdown in `SortOrderTab.tsx`** — historically planned (Acquisition ↑/↓, Weight, Type), but the current component **does not expose** them. Whether they were intentionally removed or whether their reintroduction is planned — `needs verification`.
- **ACQUISITION ↑/↓ direction naming** — earlier bug with reversed semantics relative to the game. Whether it was resolved or disappeared along with the removal of the dropdowns — `needs verification`.
- **Stable tie behavior on multiple reorders** — stride-2 guarantees no collisions within a single write. Multiple reorders with different `base` values may produce items with identical `Index` historically coming from different bases; whether the game keeps a stable tie-breaker — `needs verification`.
- **No `SnapshotSlot`/`RestoreSlot` and `ValidatePostMutation` in the reorder path** — `needs verification`, whether a conscious decision or a transactional-safety gap.
- **Workspace path test coverage** — completeness of tests for `backend/editor/save.go::writeContainerLayout` (passthrough + editable + reservedAcq skip-over) — `needs verification` via a detailed test inventory.
- **`InventoryOrderItem.Weight`, `SortId`, `SortGroupId` population** — fields exist in the DTO (`app_inventory_order.go:21-23`), but whether they are actively populated from `data.ItemData` (Phase 2 from [39-inventory-reorder](39-inventory-reorder.md)) — `needs verification`.

---

## Cross-references

- [03-gaitem-map](03-gaitem-map.md) — GaItem/GaMap binary model; inventory record references GaItem via handle.
- [07-inventory](07-inventory.md) — Inventory data model, `InventoryItem` 12 B layout, `NextAcquisitionSortId`/`NextEquipIndex` counters, visibility gate.
- [10-storage](10-storage.md) — Storage data model, differences vs Inventory, Storage Apply in-game verification status.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator/capacity/counters, transactional safety, snapshot/rollback (canonical reference for the rest of the write-side semantics).
- [39-inventory-reorder](39-inventory-reorder.md) — historical design doc; **the stride-1 algorithm from 39 is incorrect** (see the sentinel v2 discovery).
- [43-transactional-item-adding](43-transactional-item-adding.md) — Add Items pipeline; `Index` assigned single-stride from `NextAcquisitionSortId` (not stride-2).
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — Inventory ↔ Storage transfer; `Index` assignment on transfer (single-stride, monotonic advancement); Sort Order UI flow (with historical mentions of sort dropdowns — see "Direction naming and UI caveats" here).

---

## Sources

- `app_inventory_order.go` — `ReorderInventory`, `ReorderStorage`, `GetInventoryOrder`, `GetStorageOrder`, `GetWeaponInventoryOrder`, `ReorderWeaponInventory`, `inventoryOrderTabs`, `isWeaponOrderTechnical`, `tabCategorySet`, `InventoryOrderItem`
- `backend/editor/save.go` — `writeContainerLayout` (workspace stride-2 with reservedAcq skip-over), `ApplyWorkspaceSave`
- `app_inventory_session.go` — `SaveInventoryWorkspaceChanges` (entry point for the workspace path)
- `backend/core/structures.go` — `InventoryItem`, `EquipInventoryData`, `mapInventory` (load-time NextAcquisitionSortId reconcile)
- `backend/core/offset_defs.go` — `InvEquipReservedMax = 432`, `InvRecordLen = 12`, `CommonItemCount`, `StorageCommonCount`, `StorageHeaderSkip`
- Tests: `app_inventory_order_test.go`, `app_storage_order_test.go`
- Empirical in-game verification (sentinel v1/v2/v3 on Steam Deck, a real PS4 save deployed via SSH)
