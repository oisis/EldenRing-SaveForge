# 53 — Inventory ↔ Storage Transfer and Dual-Grid Sort Order

> **Type**: Design doc
> **Status**: ✅ Implemented
> **Scope**: Dual-grid Sort Order tab (Storage + Inventory), bidirectional drag-and-drop transfer (single and multi), independent preview/apply per side, and the duplicate-instance rehandle path that lets the same talisman/weapon/armor live in both containers.

---

## A. Goals

The Sort Order tab renders the active character's stash and inventory side-by-side and lets the player move items between them, sort each side independently, and persist either order back to the save:

- **Layout**: Storage on the left, Inventory on the right; identical 5×6 grid frames, identical tile semantics.
- **Transfer**: drag-and-drop in both directions (Inventory → Storage and Storage → Inventory), single or multi-item batches.
- **Sorting**: each side has its own dropdown (Acquisition ↑/↓, Weight ↑/↓, Type ↑/↓) operating on its own preview.
- **Apply Order**:
    - *Inventory Apply* → rewrites Inventory acquisition order only.
    - *Storage Apply* → rewrites Storage acquisition order only.
    - Neither side's Apply touches the other container.

The feature is exposed under the `Sort Order` tab of the character editor and operates per Sort Order category (`weapons`, `talismans`, `head`, `chest`, `arms`, `legs`).

---

## B. Data Model

### Handle prefixes — type, not location

The high byte of a `GaItem` handle encodes the **item type**, never the container. Both Inventory and Storage share the same handle namespace and GaItem array.

| Prefix mask | Item type | Move semantics |
|---|---|---|
| `0x80` | Weapon | instance-move |
| `0x90` | Armor | instance-move |
| `0xA0` | Accessory / Talisman | instance-move |
| `0xB0` | Goods | quantity-merge (stackable) |
| `0xC0` | Ash of War | instance-move |

### 12-byte record

Inventory and Storage both lay out their item lists as fixed 12-byte records:

```
offset  size  field
0       u32   handle (GaItem handle)
4       u32   quantity (1 for instance items, stack count for goods)
8       u32   Index (per-record acquisition index, used for in-game sort)
```

Inventory starts at `slot.MagicOffset + InvStartFromMagic` (×`CommonItemCount`); Storage starts at `slot.StorageBoxOffset + StorageHeaderSkip` (×`StorageCommonCount`).

### Shared GaItem / GaMap

`slot.GaItems` (array of `GaItemFull`) and `slot.GaMap` (`handle → itemID`) describe item instances at the **save level**, not per container. A transfer must therefore **never** drop GaItem rows or GaMap entries: the same handle continues to identify the same instance on the destination side, carrying its upgrade level, infusion, attached AoW, and any rehandling metadata.

---

## C. Transfer Semantics

The core transfer entry point is:

```go
core.MoveItemsBetweenContainers(slot, handles, direction, opts) (TransferResult, error)
```

where `direction ∈ {TransferToStorage, TransferToInventory}` and `opts.DestCaps` carries per-handle quantity caps (`MaxInventory` / `MaxStorage` from the DB) for cap-aware moves. The App-level wrapper `App.MoveItemsBetweenInventoryAndStorage(charIdx, handles, direction)` resolves caps and invokes core.

### Instance-move (weapon, armor, accessory, AoW)

- The source record is cleared (`handle=0, qty=0, Index=0`).
- An empty slot on the destination side receives the **same handle**, the original `Quantity`, and a freshly assigned `Index` (monotonic via the destination's `NextEquipIndex` / `NextAcquisitionSortId`).
- Caps are ignored (each record holds exactly 1).

### Quantity-merge (goods, stackable)

- If a record with the same handle already exists on the destination side, quantities are merged.
- Cap-aware partial moves are emitted with `SkipReasonDestAtCap` and the response carries `movedQty` / `remainingQty` so the UI can report "moved X, Y stayed".
- Missing caps (`DestCaps[handle] == 0` for a stackable) reject the move with `SkipReasonMissingCap` — no silent unbounded merge.

### Duplicate-instance rehandle path

For instance-move handles, the same handle value already existing on the destination side previously aborted the transfer. The current path **rehandles** the instance:

1. Allocate a new, globally unique handle of the same type prefix.
2. Create a new `GaItem` entry on the destination side carrying the source instance's metadata (upgrade, infusion, attached AoW handle, …).
3. Register the new handle → itemID mapping in `GaMap`.
4. Write the source record clear and the destination record with the new handle.

This is what enables a player to keep the **same talisman / weapon / armor** in both Storage and Inventory after a transfer — the destination instance is a distinct GaItem, not a duplicate of the source's GaItem.

### Equipped-item guard

Inventory → Storage transfers reject any handle currently referenced by `ChrAsmEquipment` with `SkipReasonEquipped`. The player must un-equip before transferring. No equipped check is performed in the opposite direction (Storage items are never equipped).

### Result shape

```go
type TransferResult struct {
    Moved   int            // count of records moved (including partial-stack moves)
    Skipped []TransferSkip // per-handle skip reasons
}

type TransferSkip struct {
    Handle       uint32
    Reason       string // "equipped" | "dest_full" | "dest_at_cap" | "missing_cap" | …
    MovedQty     uint32 // partial-cap moves only
    RemainingQty uint32 // partial-cap moves only
}
```

The function **never** partial-fails a batch: invalid handles are recorded in `Skipped`, valid handles are processed independently. After the batch the binary `common_item_count` headers are reconciled via `ReconcileInventoryHeader` / `ReconcileStorageHeader`.

---

## D. Frontend Behavior

### Independent selections

Inventory and Storage maintain disjoint selection state:

| Side | Selection set | Anchor handle | Block-drag flag |
|---|---|---|---|
| Inventory | `selectedHandles: Set<number>` | `anchorHandle` | `isBlockDragging` |
| Storage | `storageSelectedHandles: Set<number>` | `storageAnchorHandle` | `storageBlockDragging` |

Selection modifiers (same on both sides):

- **Plain click** — select one, set anchor.
- **Ctrl/Cmd click** — toggle membership, set anchor to clicked handle.
- **Shift click** — range from anchor to clicked handle in the current visible order.

### Drag/drop transfer batch resolution

When a drag is initiated, the dragged handle is checked against the source-side selection. The resolved batch for the cross-container drop is:

- If the dragged handle is in the source selection **and** the source selection has more than one entry → batch = all selected source handles, in their visible order.
- Otherwise → batch = `[draggedHandle]`.

Drop on the opposite frame triggers `App.MoveItemsBetweenInventoryAndStorage(charIdx, handles, direction)`.

### Unsaved-preview guards

The UI maintains a base/preview pair per side and derives `hasChanges` / `storageHasChanges` from the comparison. Three guards prevent loss of an unsaved preview:

| Action | Guard | Toast |
|---|---|---|
| Cross-transfer (either direction) | `hasChanges \|\| storageHasChanges` | `"Apply or reset the current order before transferring items."` |
| Inventory Apply | `storageHasChanges` | `"Apply or reset Storage order before applying Inventory order."` |
| Storage Apply | `hasChanges` | `"Apply or reset Inventory order before applying Storage order."` |

After Apply on either side, the affected side reloads from backend (`GetInventoryOrder` / `GetStorageOrder`) and the other side keeps its current base/preview.

### Per-side dropdowns and Apply

Each side has:

- **Sort dropdown** — sorts only that side's preview via `sortByMode`; manual drag flips the side's `sortMode` to `'custom'`.
- **Reset Preview button** — restores that side's preview from its base.
- **Apply Order button** — opens a side-specific confirm modal; on confirm calls `ReorderInventory` / `ReorderStorage`, then reloads that side.

The "Dragging N items" banner appears over whichever frame is currently the source of an active multi-drag.

---

## E. Ordering

### Inventory

- `App.GetInventoryOrder(charIdx, tab)` reads inventory `CommonItems`, filters by Sort Order category, returns items sorted by `Index` ascending.
- `App.ReorderInventory(charIdx, tab, orderedHandles)` rewrites only `slot.Data[off+8:]` (per-record `Index`) using **stride-2** assignment with an even base above `InvEquipReservedMax`. Rationale and proof in [spec/52](52-acquisition-sort-stride2.md): the game buckets the Acquisition Order sort by `acqIdx >> 1`, so stride-1 swaps adjacent pairs.

### Storage

- `App.GetStorageOrder(charIdx, tab)` reads `slot.Data[StorageBoxOffset + StorageHeaderSkip .. ]` and sorts by the in-record `Index` ascending — the real storage acquisition index, not binary position.
- `App.ReorderStorage(charIdx, tab, orderedHandles)` mirrors the Inventory path on `slot.Storage.CommonItems` and `slot.Storage.NextEquipIndex` / `NextAcquisitionSortId`:
    - Same validation: complete list, no duplicates, category match, no technical placeholders (e.g. Unarmed).
    - Same **stride-2** assignment. Stride-2 is correct here too: the game's in-storage browser uses the same `acqIdx >> 1` bucketing as inventory.
    - `pushUndo` before any mutation, monotonic counter advance, defensive `ReconcileStorageHeader` at the end.
    - **No write** to inventory bytes or counters.

### Transfer Index assignment

`MoveItemsBetweenContainers` assigns the destination record's `Index` from the destination side's monotonic counters (`NextEquipIndex` / `NextAcquisitionSortId`), guaranteeing that the freshly transferred item appears at the end of Acquisition ↑ on the destination grid. After the transfer, `GetStorageOrder` / `GetInventoryOrder` immediately reflect this.

---

## F. Test Coverage

Backend tests cover the full transfer + ordering surface. Key classes:

| Class | Representative tests | Where |
|---|---|---|
| Instance-move (both directions) | `TestMoveNonStackableInvToStorage`, `TestMoveNonStackableStorageToInv` | `tests/transfer_test.go` |
| Quantity-merge + cap-aware partial | `TestMoveStackableInvToStorage_MergePartialAtCap`, `TestMoveStackableStorageToInv_PartialAtCap`, `TestMoveStackableMissingCap`, `TestMoveDestFull`, `TestMoveStackableStorageToInv_Merge` | `tests/transfer_test.go` |
| Equipped guard | `TestMoveEquippedInvToStorageSkipped` | `tests/transfer_test.go` |
| Invalid / mixed / duplicate handle batch | `TestMoveInvalidHandle`, `TestMoveMixedValidInvalid` | `tests/transfer_test.go` |
| Header reconcile + GaItem preservation | `TestMoveHeadersReconciled`, `TestMoveNoOrphanedGaItem` | `tests/transfer_test.go` |
| Round-trip (write + reload) | `TestMoveRoundTripSave` | `tests/transfer_test.go` |
| Talisman duplicate rehandle | `TestMoveTalismanDestHasSameHandle_RehandlesAndMoves`, `TestMoveTalismanStorageToInventory_DuplicateHandle_RehandlesAndMoves`, `TestMoveTalismanInvToEmptyStorage_PhysicalMove`, `TestMoveTalismanStorageToEmptyInventory_PhysicalMove` | `tests/transfer_test.go` |
| Weapon / armor duplicate allowed | `TestMoveWeaponAllowsDuplicateSameItemIDOnDestination`, `TestMoveArmorAllowsDuplicateSameItemIDOnDestination` | `tests/transfer_test.go` |
| Goods duplicate handle still merges | `TestMoveGoodsDuplicateHandle_StillQuantityMerge` | `tests/transfer_test.go` |
| VM visibility after rehandle | `TestTransferTalismanVisibleInVM_InvToStorage`, `TestTransferTalismanVisibleInVM_StorageToInv` | `tests/transfer_test.go` |
| Storage order uses record Index | `TestStorageOrderUsesRecordIndex` | `tests/transfer_test.go` |
| New transferred item appears at end of acquisition | `TestTransferInvToStorageAppearsAtEndByAcquisition`, `TestTransferStorageToInvAppearsAtEndOfInventory` | `tests/transfer_test.go` |
| ReorderStorage rejects bad input | `TestReorderStorage_RejectsMissingHandle`, `TestReorderStorage_RejectsDuplicateHandle`, `TestReorderStorage_RejectsIncompleteList`, `TestReorderStorage_RejectsHandleFromInventory` | `app_storage_order_test.go` |
| ReorderStorage persists order | `TestReorderStorage_PersistsAcquisitionOrder`, `TestReorderStorage_DoesNotTouchHandlesOrQty`, `TestReorderStorage_RoundTripReread` | `app_storage_order_test.go` |
| Cross-container isolation | `TestReorderStorage_DoesNotTouchInventory`, `TestInventoryReorder_DoesNotTouchStorage` | `app_storage_order_test.go` |
| Inventory reorder full surface | `TestReorderWeaponInventory_*`, `TestReorderInventory_*` | `app_inventory_order_test.go` |

---

## G. Known Limitations / Future Work

- **Manual in-game verification** is still required for each Sort Order tab (talismans, weapons, armor pieces) on a real save deployed to Steam Deck / console. Automated tests cover binary correctness but not the rendered in-game grid order.
- **Equipped detection** relies on `ChrAsmEquipment` references. Edge cases (e.g. equipment swapped in transient menu states, summons, NG+ inheritance) may need broader validation across multiple save fixtures.
- **Batch rehandle performance**: each duplicate instance currently allocates a new handle + new GaItem sequentially. For a multi-transfer of many duplicated talismans this is O(N) handle allocations; profiling and a batched allocation path may be useful if a large preset import becomes a real workflow.
- **Storage Apply in-game sanity check**: deploy to Steam Deck → run game → verify Storage box "Acquisition Order ↑" matches the editor preview for each Sort Order tab. Same procedure already in use for Inventory (spec/52).
- **Storage drag/drop layout**: the grid currently shows 5×6 = 30 tiles with vertical overflow. For storages with hundreds of items the scroll bar is functional but not ideal; future work could add chunked rendering / virtualization (cross-cutting with the general inventory virtualization Backlog item in `docs/ROADMAP.md`).

---

## Implementation Locations

- `app_inventory_order.go` — `GetInventoryOrder`, `GetStorageOrder`, `ReorderInventory`, `ReorderStorage`
- `app.go` — `MoveItemsBetweenInventoryAndStorage` (App-level wrapper, cap resolution, undo push)
- `backend/core/transfer.go` — `MoveItemsBetweenContainers`, rehandle path, header reconcile
- `frontend/src/components/SortOrderTab.tsx` — dual-grid layout, per-side state, drag/drop, guards, Apply flows

---

## Sources

- spec/39: original Inventory Reorder design
- spec/52: stride-2 discovery and proof
- Empirical in-game testing on Steam Deck (real PS4 save deployed via SSH)
- `tests/transfer_test.go` — extensive duplicate-instance and VM-visibility coverage
