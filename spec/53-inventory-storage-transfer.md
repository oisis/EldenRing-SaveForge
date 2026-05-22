# 53 — Transfer Inventory ↔ Storage

> **Type**: Design doc
> **Status**: ✅ Implemented (workspace path) + ✅ Implemented (legacy core path)
> **Scope**: Mechanics of bidirectional record transfer between `slot.Inventory.CommonItems` and `slot.Storage.CommonItems`, the semantics of instance-move vs quantity-merge, rehandle for instance duplicates, equipped guard, and integration with the two-column workspace UI in `SortOrderTab`.

---

## Chapter goal

The chapter describes the **current** mechanics of moving items between character containers (Inventory ⇄ Storage) as implemented by the two coexisting paths in the code:

1. **Legacy core path** — `core.MoveItemsBetweenContainers` invoked via the App-level binding `App.MoveItemsBetweenInventoryAndStorage`. Mutates the binary directly, offers cap-aware merge, rehandle and equipped guard. Remains active as a public binding and a full test surface.
2. **Workspace save path** — `editor.ApplyWorkspaceSave` → `writeContainerLayout`. Wipe-and-replay of entire `CommonItems` regions. Used by the current `SortOrderTab` UI (two-column Storage|Inventory view). **OriginalHandle is preserved** during transfer — the workspace does not invoke rehandle.

The chapter does NOT document sort dropdowns, per-side Apply, or per-side preview guards — none of these exist in the current `SortOrderTab.tsx`. See also [39](39-inventory-reorder.md) as a historical design note.

---

## Status

| Component | Status |
|---|---|
| `core.MoveItemsBetweenContainers` (legacy) | ✅ active, 28 tests in `tests/transfer_test.go` |
| `App.MoveItemsBetweenInventoryAndStorage` (App wrapper) | ✅ active, exposed via Wails bindings |
| `editor.ApplyWorkspaceSave` + `writeContainerLayout` (workspace) | ✅ active, called by every `workspace.save()` from `SortOrderTab` |
| Two-column `SortOrderTab` UI | ✅ currently used |
| Per-side sort dropdowns | ❌ do not exist — the earlier project was not implemented |
| Per-side Apply / Reset Preview / per-side hasChanges guards | ❌ do not exist — the workspace has one global Save/Discard |
| In-game verification (Steam Deck) — Storage Apply / transfer | `needs verification` — no fresh report from a PS4 fixture |

---

## Source of truth in code

| Topic | File / function | Tests |
|---|---|---|
| Core engine | `backend/core/transfer.go:94` — `MoveItemsBetweenContainers(slot, handles, direction, opts)` | `tests/transfer_test.go` |
| App wrapper | `app.go:741` — `MoveItemsBetweenInventoryAndStorage(charIdx, handles, direction string)` | ditto |
| Instance-move | `backend/core/transfer.go:199` — branch for `isInstanceMoveHandle` | `TestMoveNonStackableInvToStorage`, `TestMoveNonStackableStorageToInv` |
| Quantity-merge | `backend/core/transfer.go:252` — cap-aware merge / partial | `TestMoveStackable*`, `TestMoveStackableInvToStorage_MergePartialAtCap` |
| Rehandle | `backend/core/transfer.go:208` — `materializeRehandledInstance` + `rebuildAfterAllocation` | `TestMoveTalismanDestHasSameHandle_RehandlesAndMoves`, `TestMoveTalismanStorageToInventory_DuplicateHandle_RehandlesAndMoves` |
| Equipped guard | `backend/core/transfer.go:194,592` — `IsHandleEquipped` (ChrAsmEquipment scan) | `TestMoveEquippedInvToStorageSkipped` |
| Header reconcile | `backend/core/transfer.go:128-129` — `ReconcileInventoryHeader` / `ReconcileStorageHeader` | `TestMoveHeadersReconciled` |
| Workspace save | `backend/editor/save.go:79` — `ApplyWorkspaceSave` | `backend/editor/save_test.go` (collect/validate AoW, detect removed/transferred, pass-through indices, pickNewHandle), `backend/editor/workspace_test.go` (`TestBuildSnapshot_*`) |
| Workspace save App caller | `app_inventory_session.go:192` — `SaveInventoryWorkspaceChanges(sessionID)` with `core.SnapshotSlot`/`RestoreSlot` + `pushUndo` | `app_inventory_session_save_test.go`, `app_inventory_session_sequential_test.go` |
| Workspace layout writer | `backend/editor/save.go:518` — `writeContainerLayout` (wipe-and-replay, stride-2 acq base from line 566) | ditto |
| Frontend two-column UI | `frontend/src/components/SortOrderTab.tsx` (1303 lines) | — |
| Workspace hook (RAM-only ops) | `frontend/src/hooks/useInventoryWorkspace.ts` — `transferItem`, `moveItem`, `addItem`, `removeItem`, `save`, `discard` | — |

The 12-byte record binary model and `CommonItems` offsets — see [07](07-inventory.md) (Inventory) and [10](10-storage.md) (Storage). GaItem ↔ handle mapping — see [03](03-gaitem-map.md). Allocator invariants — see [35](35-gaitem-allocator-invariants.md). Stride-2 acquisition order — see [52](52-acquisition-sort-stride2.md).

---

## Mental model

```
            ┌─────────────────┐                ┌─────────────────┐
            │  Inventory      │  ◀────────▶    │  Storage        │
            │  CommonItems    │   transfer     │  CommonItems    │
            │  (fixed array,  │                │  (sparse list,  │
            │   CommonItem-   │                │   capacity      │
            │   Count slots)  │                │   StorageCommon-│
            │                 │                │   Count)        │
            └─────────────────┘                └─────────────────┘
                    ▲                                  ▲
                    │ shared                           │
                    └─────────  slot.GaItems  ─────────┘
                                slot.GaMap (handle → itemID)
```

- The handle prefix (top byte) encodes the **type** of the item, never the location. Inventory and Storage share one handle space. The full taxonomy is in [03](03-gaitem-map.md).
- A transfer **never** removes `GaItem` or `GaMap` entries. The same handle still identifies the same instance on the destination side, carrying its upgrade level, infusion and attached AoW.
- For the **transfer** mechanic (not reading) only two prefix categories matter:

| Prefix | Item category | Transfer semantics |
|---|---|---|
| `0x80` Weapon, `0x90` Armor, `0xA0` Accessory/Talisman, `0xC0` AoW | instance-backed | instance-move (the record is transplanted; cap ignored) |
| `0xB0` Goods | stackable | quantity-merge cap-aware (partial move possible) |

---

## Current SortOrderTab workspace model

`SortOrderTab` operates exclusively on the **workspace session** returned by the `useInventoryWorkspace()` hook. All mutations (reorder, transfer, add, remove, weapon edit) happen in RAM on the Go side and do not touch `slot.Data` until `workspace.save()`.

### Operations exposed by the hook

| Operation | Signature | What it does |
|---|---|---|
| `start(charIdx)` | initialises the session for a character | creates a snapshot from the current save |
| `moveItem(uid, container, targetPos)` | reorder within a single container | dispatch to the RAM snapshot; does not call `Reorder*` from `app_inventory_order.go` |
| `transferItem(uid, target)` | cross-grid move | rewrites `Container` in the snapshot from `inventory` to `storage` or vice versa |
| `addItem(spec, target, pos)` | add a new item | dispatch to core `AddItemsToSlotBatch` at save (see [43](43-transactional-item-adding.md)) |
| `removeItem(uid)` | remove an item | the element disappears from the snapshot |
| `updateWeapon(uid, patch)` | upgrade/infusion/AoW patch | pending flags written to `EditableItem` |
| `save()` | commit the whole session to the save file | invokes `editor.ApplyWorkspaceSave(slot, snap, baseline)` |
| `discard()` | drop pending changes | restart the session from the current save |

### What `SortOrderTab` has and does not have

It has (verified in `frontend/src/components/SortOrderTab.tsx`):
- Storage on the left, Inventory on the right (lines 799–911).
- A 5-column × 6-row minimum frame (`GRID_COLS=5`, `GRID_MIN_ROWS=6`), `overflow-y-auto`.
- 6 category tabs: Weapons / Talismans / Head / Chest / Arms / Legs (lines 56–63).
- `UNARMED_BASE_ID = 0x0001ADB0` is excluded from the Weapons tab (frontend `tabFilter` + backend `inventoryOrderTabs`).
- Per-side selection based on **UIDs** (`editor.EditableItem.uid: string`), not on u32 handles. State: `invSelectedUIDs: Set<string>`, `invAnchorUID: string | null`, analogously `sto*`.
- Plain click / Ctrl-Cmd click (toggle) / Shift click (range).
- Drag within the container → `workspace.moveItem(uid, container, targetPos)`.
- Cross-container drag → sequentially `workspace.transferItem(uid, target)` for all UIDs in the batch (if the dragged item is in the selection and the selection > 1, the batch = `Array.from(selection)`).
- Add Item modal → `workspace.addItem(spec, target, -1)`. Cap-aware semantics of adding is the mechanic of [43](43-transactional-item-adding.md).
- Remove (top bar) → `workspace.removeItem(uid)` per selected UID.
- `dirty` flag, `validation` report (errors/warnings), `Save changes` disabled when `errorCount > 0`.

It does not have (confirmed by absence in code):
- An `Acquisition ↑/↓` / `Weight ↑/↓` / `Type ↑/↓` sort dropdown on either side.
- An `Apply Order` / `Reset Preview` button per side.
- Per-side `hasChanges` / `storageHasChanges` flags or three toast guards blocking cross-transfer for "unsaved order".
- A "Dragging N items" banner.

---

## Transfer directions and constraints

```go
const (
    TransferToStorage   TransferDirection = iota // Inventory.CommonItems → Storage.CommonItems
    TransferToInventory                          // Storage.CommonItems → Inventory.CommonItems
)
```

The App wrapper accepts the directive string `"to-storage"` or `"to-inventory"` and converts it to the type (see `app.go:749-757`). Any other string → error.

Constraints checked **in core**:
- `slot.Version == 0` → error (`empty slot`).
- `direction` outside the two known values → error.
- Handle `0` or `0xFFFFFFFF` → skip with `SkipReasonInvalidHandle` (never fatal).

The App wrapper additionally performs a **dry-check** of whether any handle exists in the source binary. If so — `pushUndo(charIdx)` BEFORE actually calling core. If not — undo is not pushed (saving undo stack space).

---

## Drag/drop semantics

### Within-container reorder (`onTileDrop`, `workspace.moveItem`)

The target position is computed from the local tile index in the per-tab view and converted to a global index of the full container list by `computeTargetPosition` (`SortOrderTab.tsx:180-204`). The `MoveItem` backend interprets `targetPosition` in **post-pop slice** — on a downward move the source element is "popped" from the list before insertion, so the frontend subtracts 1 from `prePopTarget` when the target lies past the source.

A drop past the last tile of a tab → insertion just after the last item visible in that tab (`SortOrderTab.tsx:195-201`).

### Cross-grid transfer (`onFrameDrop`, `workspace.transferItem`)

Batch on multi-select: if the dragged UID is in the source selection **and** the selection > 1 → batch = `Array.from(selection)`. Otherwise batch = `[draggedUID]`. NOTE: the batch order is the iteration order of `Set<string>`, not "visible order" — in the current code it is not stably sorted by grid position. (**needs verification** whether this is intentional — from the workspace tests' perspective it changes nothing, because every transfer adds the item at the end of acquisition on the destination side.)

A drop on the frame of the opposite container invokes `await workspace.transferItem(uid, target)` sequentially for all UIDs.

### Occupied vs empty slot — workspace model

The workspace **has no notion of "occupied slot"** in the binary sense. The snapshot is a list of `EditableItem` sorted by position; dropping on position X = insert at position X and shift the rest. No collision check, because the binary is re-emitted by `writeContainerLayout` from scratch.

---

## Inventory → Storage

Legacy core path (`TransferToStorage`):
1. Look up the source record in `Inventory.CommonItems` by handle (`scanRecord`).
2. **Equipped guard**: `IsHandleEquipped(slot, handle)` — if true → skip `SkipReasonEquipped`.
3. Classification: instance-move (prefixes `0x80`/`0x90`/`0xA0`/`0xC0`) vs quantity-merge (`0xB0`).
4. Instance-move:
   - Dest empty slot → `writeRecord(dst)` with the same handle, original `Quantity`, new `Index` (from `assignDestIndex` on the Storage side).
   - Dest **already has the same handle** → **rehandle** path (section "Instance-backed item transfer and rehandle" below).
5. Quantity-merge:
   - Dest exists → `dstExistingQty + transferQty` clamped to `caps[handle]`, partial → `srcQty -= transferQty`.
   - Dest does not exist → new record with `transferQty = min(srcQty, cap)`.
6. After the batch: `ReconcileInventoryHeader`, `ReconcileStorageHeader`, `rescanInventoryList`, `rescanStorageList`.

Workspace path:
- The RAM snapshot rewrites `EditableItem.Container = ContainerStorage`.
- On `workspace.save()` → `ApplyWorkspaceSave` → `writeContainerLayout(slot, snap, ContainerInventory)` (wipe-and-replay without this item) + `writeContainerLayout(slot, snap, ContainerStorage)` (wipe-and-replay with this item at the end of the editable queue).
- `OriginalHandle` is **preserved** — `writeContainerLayout` writes the destination record with `it.OriginalHandle`, so the GaItem still lives under the same handle.
- **No explicit equipped guard** in the workspace pre-flight — in `ApplyWorkspaceSave` only the following are visible: `Validate(snap)`, pending AoW pre-flight, capacity check, pass-through SlotIndex check. `needs verification`: whether `Validate(snap)` blocks the transfer of equipped items, whether an equipped item moved to Storage via workspace save will be rejected.

---

## Storage → Inventory

Legacy core (`TransferToInventory`):
- Mirror of Inventory → Storage **without** the equipped guard (items in Storage are never equipped).
- Instance-move: dest empty slot → the same handle, original qty, a new `Index` from `Inventory.NextAcquisitionSortId` (clamped above `InvEquipReservedMax`).
- Quantity-merge: cap = `MaxInventory` from the DB.

Workspace path:
- Identical mechanics to Inventory → Storage, only `EditableItem.Container = ContainerInventory`.

---

## Inventory reorder and Storage reorder relation

Reorder (changing order within a container) and transfer (changing the container) use **different** paths in the current UI:

| Operation | Legacy binding | Workspace path |
|---|---|---|
| Inventory reorder | `App.ReorderInventory` (`app_inventory_order.go:255`) — stride-2 write, full tests in `app_inventory_order_test.go` | `workspace.moveItem(uid, 'inventory', pos)` in RAM, final write in `writeContainerLayout` with the stride-2 base from line 566 |
| Storage reorder | `App.ReorderStorage` (`app_inventory_order.go:439`) — stride-2 write, tests in `app_storage_order_test.go` | `workspace.moveItem(uid, 'storage', pos)` analogously |
| Inventory ↔ Storage transfer | `App.MoveItemsBetweenInventoryAndStorage` → `core.MoveItemsBetweenContainers` | `workspace.transferItem(uid, target)` |

`SortOrderTab` in the current code **does not call** `ReorderInventory`, nor `ReorderStorage`, nor `MoveItemsBetweenInventoryAndStorage`. All operations go through the workspace. The direct bindings are kept as the public App API and the full test surface.

Stride-2 algorithm and bucket-collision guard — canonically in [52](52-acquisition-sort-stride2.md). `writeContainerLayout` uses the same family of invariants (even `baseAcq` ≥ `InvEquipReservedMax+2`, `acq = base + pos*2`, skip-over of collisions with `reservedAcq` from lines 573-585).

**Storage → Storage** and **Inventory → Inventory** reorder are fully supported by the workspace (drag within the container). There is no code that would block them.

---

## Stackable goods merge and partial moves

This concerns only the legacy core path (`core.MoveItemsBetweenContainers`). The workspace path has no equivalent of cap-aware merge at the save level — the workspace works with pre-batched `EditableItem.Quantity` from the UI, so goods caps are managed at the Add level (see [43](43-transactional-item-adding.md)).

### Cap-aware partial merge — `transferOne` quantity-merge branch

1. `cap = opts.DestCaps[handle]`. Missing entry or `cap == 0` → `SkipReasonMissingCap` (**no** silent unbounded merge — iron rule).
2. Dest has a record with the same handle:
   - `dstExistingQty >= cap` → `SkipReasonDestAtCap`, no move.
   - Otherwise: `transferQty = min(srcQty, cap - dstExistingQty)`. Updates qty of the destination and source.
   - Remaining qty on the source side > 0 → result: `moved=true`, skip = `SkipReasonDestAtCap` with `MovedQty` and `RemainingQty` populated.
3. Dest has no record → creates a new one with `transferQty = min(srcQty, cap)`. The rest as above.

The App wrapper resolves caps from the DB:
```go
itemData, _ := db.GetItemDataFuzzy(itemID)
if dir == core.TransferToStorage { cap = itemData.MaxStorage } else { cap = itemData.MaxInventory }
caps[h] = cap
```

Instance-move handles (`0x80`/`0x90`/`0xA0`/`0xC0`) are **excluded** from `caps` (app.go:813). The wrapper intentionally does not write `MaxInventory=1` for them, so that the contract is unambiguous — instance-move ignores the cap entirely.

---

## Instance-backed item transfer and rehandle

Instance-move for handle prefixes `0x80`/`0x90`/`0xA0`/`0xC0`:

### Empty dest slot — simple path

`writeRecord(dst, handle, srcQty, newIndex)` + `clearRecord(src)`. The handle stays, the GaItem stays, the `Index` on the destination side is **fresh** (from `assignDestIndex` — clamp above `InvEquipReservedMax`).

### Dest already has the same handle — rehandle path (`materializeRehandledInstance`)

For talismans (`0xA0`) the handle = `itemID | prefix`, so two `AddItemsToSlot` entries for the same item in Inventory and Storage produce **two records sharing one handle**. The old semantics rejected the transfer ("dest_duplicate"); the current one **rehandles**:

1. `materializeRehandledInstance(slot, oldHandle)`:
   - Resolve `itemID` from `slot.GaMap[oldHandle]`, fallback for `0xA0`/`0xB0` via bit-swap (`lower | 0x20000000` / `lower | 0x40000000`).
   - `generateUniqueHandle(slot, prefix)` — a new globally unique handle with the same type prefix.
   - `allocateGaItem(slot, newHandle, itemID)` — a new `GaItem` on the destination side.
   - `slot.GaMap[newHandle] = itemID`.
2. `rebuildAfterAllocation(slot)` — `RebuildSlotFull` + `parseFromData`, refresh of dynamic offsets. Snapshots `slot.GaMap` before rebuild and merges stackable handle-encoded entries back in (mirror pattern from [43](43-transactional-item-adding.md)/writer.go).
3. After the rebuild: re-scan of the source record (offsets may have shifted), `writeRecord(dst, newHandle, srcQty, newIndex)`, `clearRecord(src)`.

Skip `SkipReasonHandleAllocFailed` on allocation errors. Allocator invariants — canonically in [35](35-gaitem-allocator-invariants.md).

### Workspace path — no rehandle

`writeContainerLayout` uses `it.OriginalHandle` as the destination handle. If the same item was simultaneously in both containers in the workspace baseline, then each side had its own `EditableItem.OriginalHandle` (from the moment the session was created). There is no "duplicate handle on dest during transfer" scenario, because wipe-and-replay re-emits both sides at the same time. **needs verification**: whether the workspace baseline always distinguishes such pairs (e.g. two talismans of the same item in Inv and Storage) without a UID/handle collision.

---

## Equipped guard

Legacy core path:
- `IsHandleEquipped(slot, handle)` scans `slot.EquipItemsIDOffset .. +ChrAsmEquipmentSize` (`backend/core/transfer.go:592`).
- Match candidates: handle directly, lower 28 bits, `lower | 0x80000000`, `GaMap[handle]`, `GaMap[handle] | 0x80000000`, plus prefix-swaps for talismans (`0xA0 → 0x20`) and goods (`0xB0 → 0x40`).
- Active only for `TransferToStorage`. Storage → Inventory does not check equipped (assumption: items in Storage are not equipped).
- No `EquipItemsIDOffset` → guard returns `false` (does not block).

Workspace path:
- **No** explicit equipped check in `ApplyWorkspaceSave` or `writeContainerLayout`. `needs verification`: whether `Validate(snap)` reports an equipped-item-transferred warning/error. Empirically from the code dump the pre-flight does not show one.
- **needs verification**: behavior after moving an equipped talisman/weapon via workspace UI — whether the game re-registers the equipment slot or returns a rendering error.

---

## Capacity and caps

| Limit | Value | Enforced by |
|---|---|---|
| Inventory `CommonItems` slots | `CommonItemCount` | `containerBinary` (legacy), `writeContainerLayout` pre-flight (workspace) |
| Storage `CommonItems` slots | `StorageCommonCount` | as above |
| Goods per-handle `MaxInventory` | DB | App wrapper at `caps` build |
| Goods per-handle `MaxStorage` | DB | as above |
| Instance-move qty per record | 1 | convention (the record holds `qty=1` for an instance) |

Pre-flight capacity check in the workspace path (`backend/editor/save.go:115-122`):
```go
if invTotal := len(snap.InventoryItems) + len(snap.UnsupportedInventoryRecords); invTotal > core.CommonItemCount { ...reject... }
if stoTotal := len(snap.StorageItems) + len(snap.UnsupportedStorageRecords); stoTotal > core.StorageCommonCount { ...reject... }
```
The workspace **does not check** per-handle goods caps before save — caps are the domain of Add (see [43](43-transactional-item-adding.md)) and the UI limits entered quantities at `addItem`/`updateQuantity`.

---

## Save / discard workflow

```
[user UI action]            [workspace state]                [save file]
──────────────              ──────────────────               ──────────
drag/drop reorder      ──▶  workspace.moveItem    ──▶  RAM (snap.InventoryItems / StorageItems)
drag cross-grid        ──▶  workspace.transferItem ─▶  RAM (it.Container = target)
Add Item modal         ──▶  workspace.addItem     ──▶  RAM (Source=Added, OriginalHandle=0)
Remove                 ──▶  workspace.removeItem  ──▶  RAM (item dropped from snapshot)
WeaponEditModal        ──▶  workspace.updateWeapon ─▶  RAM (PendingUpgrade/Infusion/AoW flags)

[Save changes click]
   confirm modal
   workspace.save()    ──▶  App.SaveInventoryWorkspaceChanges(sessionID)
                              ├─ rollback := core.SnapshotSlot(slot)
                              ├─ a.pushUndo(charIdx)
                              ├─ editor.ApplyWorkspaceSave(slot, snap, baseline)
                              │   ├─ Pre-flight: Validate, capacity, pass-through SlotIndex, pending AoW
                              │   ├─ executeAdds        — core.AddItemsToSlotBatch for Source=Added
                              │   ├─ executeWeaponPatches — core.PatchWeaponItemID for upgrade/infusion
                              │   ├─ executePendingAoWPatches — core.PatchWeaponAoW / PatchWeaponAoWHandle
                              │   ├─ writeContainerLayout(ContainerInventory) — wipe-and-replay
                              │   ├─ writeContainerLayout(ContainerStorage) — wipe-and-replay
                              │   └─ ReconcileInventoryHeader / ReconcileStorageHeader
                              └─ on error: core.RestoreSlot(slot, rollback) (session Dirty=true)
                          ──▶ slot.Data

[Discard changes click]
   confirm modal
   workspace.discard() ──▶  restart the session from the current save (RAM reset)
```

- `Save changes` is **disabled** if `validation.errors.length > 0`.
- The `dirty` flag is global to the session — there is no per-side `dirty`. Every change (reorder in one container, transfer, add, remove) lights one flag.

---

## Validation and rollback caveats

### Legacy core path

- **No snapshot/rollback**. `MoveItemsBetweenContainers` mutates `slot.Data` in a loop. If a middle-of-batch handle exits with `SkipReasonHandleAllocFailed`, earlier successful transfers **stay**. This is by design — `TransferResult` reports per-handle outcome.
- The App wrapper does `pushUndo(charIdx)` BEFORE the core call (when the dry-check finds at least one valid handle), so the whole batch is reversible via Undo — but is not atomically rejected by core.
- Defensive `ReconcileInventoryHeader` / `ReconcileStorageHeader` + `rescanStorageList` / `rescanInventoryList` at the end of the batch — fixup, not rollback.

### Workspace path

- `ApplyWorkspaceSave` **requires external rollback**. The atomicity contract from the docstring (`backend/editor/save.go:70-78`):
  > Callers MUST snapshot slot via `core.SnapshotSlot` BEFORE calling this function and call `core.RestoreSlot` on a non-nil error to roll back partial state. This function does NOT manage its own undo. It only guarantees all rejection checks run BEFORE any mutation; if a check fails, slot.Data is byte-identical to the input. Once writes begin (after the rejection block), an error means slot.Data has been partially mutated. Caller MUST roll back.
- Pre-flight rejection (before any mutation): `Validate(snap)`, missing baseline, pending AoW unknown/incompatible, capacity overflow, pass-through SlotIndex collision/range.
- **The App-level caller enforces the contract**: `App.SaveInventoryWorkspaceChanges` (`app_inventory_session.go:192-228`) takes `rollback := core.SnapshotSlot(slot)` before `pushUndo` + `ApplyWorkspaceSave`, and calls `core.RestoreSlot(slot, rollback)` both when `ApplyWorkspaceSave` returns an error and when the post-save snapshot rebuild fails. The session remains `Dirty=true` after a failed save, which preserves pending edits for a retry.
- Unlike the "Add Items" path (see [43](43-transactional-item-adding.md)), where the whole transaction has its own internal snapshot/restore and `ValidatePostMutation`, the workspace save **has no** post-mutation validation — only pre-flight + external rollback on the App caller side. **needs verification**: whether adding post-mutation validation analogous to 43 is planned.

---

## UI counters and allocator caveats

- `ColumnHeader` (`SortOrderTab.tsx:931`) shows `count` = `view.length` per active tab, NOT the total of the container.
- `selectedCount` is also per-tab — `invSelectedHere = inventoryView.filter(it => invSelectedUIDs.has(it.uid))`. UIDs outside the visible tab remain in the selection but are not counted in the header.
- `useEffect` on tab change (`SortOrderTab.tsx:148-159`) clears UIDs that fell out of `visible` — stale selections do not block batch operations after switching tabs.
- The bottom bar shows `Session ID: {sessionID || '—'}` — the session variable from `workspace.start(charIdx)`.

Allocator-related caveats (summary — details in [35](35-gaitem-allocator-invariants.md)):
- Every rehandle in the legacy core path allocates a new handle + new GaItem. For a batch of N talisman duplicates that is N sequential allocations. **needs verification**: benchmark for a large preset import (e.g. >5 AoW duplicates at once).
- The workspace path **never rehandles** — preserves `OriginalHandle`. No allocator pressure on workspace transfer.

---

## Test coverage

Backend tests covering transfer + ordering:

| Class | Representative tests | Location |
|---|---|---|
| Instance-move both directions | `TestMoveNonStackableInvToStorage`, `TestMoveNonStackableStorageToInv` | `tests/transfer_test.go` |
| Quantity-merge + cap-aware partial | `TestMoveStackableInvToStorage_MergePartialAtCap`, `TestMoveStackableStorageToInv_PartialAtCap`, `TestMoveStackableInvToStorage_NoDest`, `TestMoveStackableStorageToInv_Merge` | `tests/transfer_test.go` |
| Missing cap / dest full | `TestMoveStackableMissingCap`, `TestMoveDestFull` | `tests/transfer_test.go` |
| Equipped guard | `TestMoveEquippedInvToStorageSkipped` | `tests/transfer_test.go` |
| Invalid / mixed batch | `TestMoveInvalidHandle`, `TestMoveMixedValidInvalid` | `tests/transfer_test.go` |
| Header reconcile + no GaItem orphan | `TestMoveHeadersReconciled`, `TestMoveNoOrphanedGaItem` | `tests/transfer_test.go` |
| Round-trip (write + reload) | `TestMoveRoundTripSave` | `tests/transfer_test.go` |
| Rehandle (talisman) | `TestMoveTalismanDestHasSameHandle_RehandlesAndMoves`, `TestMoveTalismanStorageToInventory_DuplicateHandle_RehandlesAndMoves`, `TestMoveTalismanInvToEmptyStorage_PhysicalMove`, `TestMoveTalismanStorageToEmptyInventory_PhysicalMove` | `tests/transfer_test.go` |
| Weapon/armor duplicate allowed | `TestMoveWeaponAllowsDuplicateSameItemIDOnDestination`, `TestMoveArmorAllowsDuplicateSameItemIDOnDestination` | `tests/transfer_test.go` |
| Goods duplicate handle (cap merge) | `TestMoveGoodsDuplicateHandle_StillQuantityMerge` | `tests/transfer_test.go` |
| VM visibility after rehandle | `TestTransferTalismanVisibleInVM_InvToStorage`, `TestTransferTalismanVisibleInVM_StorageToInv` | `tests/transfer_test.go` |
| Storage order uses record Index | `TestStorageOrderUsesRecordIndex` | `tests/transfer_test.go` |
| A fresh transfer appears at the end of acquisition | `TestTransferInvToStorageAppearsAtEndByAcquisition`, `TestTransferStorageToInvAppearsAtEndOfInventory` | `tests/transfer_test.go` |
| Full reorder validation surface (Inventory) | `TestReorderWeaponInventory_*`, `TestReorderInventory_*` | `app_inventory_order_test.go` |
| Storage reorder validation + invariants | `TestReorderStorage_*`, `TestInventoryReorder_DoesNotTouchStorage` | `app_storage_order_test.go` |

Workspace save end-to-end coverage (`editor.ApplyWorkspaceSave` + `writeContainerLayout`) — **needs verification**: the full list of `backend/editor/*_test.go` tests and whether they cover cross-container transfer + workspace save end-to-end with reload scenarios.

---

## Known limits / needs verification

- **Storage Apply / transfer in-game (Steam Deck)**: no fresh report that after `workspace.save()` with a cross-grid transfer the in-game box matches the editor's preview for every Sort Order tab. Cross-ref to [52](52-acquisition-sort-stride2.md) — a common gap for the whole stride-2 pipeline.
- **Equipped guard in workspace path**: no explicit check in `ApplyWorkspaceSave` / `writeContainerLayout`. `needs verification` whether `Validate(snap)` blocks transfer of equipped items and how the game renders the equipment slot after such a transfer.
- **Workspace post-mutation validation**: none — unlike [43](43-transactional-item-adding.md). Rollback is realised in the App wrapper (`SaveInventoryWorkspaceChanges`) via `SnapshotSlot`/`RestoreSlot`, but on the save side itself there is no post-write sanity check. `needs verification` whether it is planned.
- **Batch rehandle performance**: legacy core allocates sequentially. `needs verification` for scenarios with ≥5 talisman/AoW duplicates at once (preset import).
- **Batch order in cross-grid transfer**: the workspace path iterates `Array.from(selection)` — Set iteration order, not visible-grid. Functionally OK (each transfer lands at the end of acquisition on the destination side), but not documented as stable.
- **Equipped detection NG+ / summon / menu transient state**: tested on a single fixture. `needs verification` for broader coverage.
- **Workspace baseline collision for instance-item pairs**: the same talisman on both sides in the baseline. `needs verification` whether the UID separates them reliably and `writeContainerLayout` does not write the same handle twice.

---

## Cross-references

- [03 — GaItem map](03-gaitem-map.md) — handle ↔ itemID, type prefixes.
- [07 — Inventory model](07-inventory.md) — read-side 12B record, `CommonItems` offsets, `CommonItemCount`.
- [10 — Storage model](10-storage.md) — read-side 12B record, `StorageBoxOffset`, `StorageHeaderSkip`, `StorageCommonCount`.
- [35 — GaItem allocator invariants](35-gaitem-allocator-invariants.md) — `generateUniqueHandle`, `allocateGaItem`, `RebuildSlotFull`, `parseFromData` invariants.
- [39 — Inventory reorder (historical)](39-inventory-reorder.md) — historical design note for per-side Apply / sort dropdowns (not implemented in the described form).
- [43 — Transactional item adding](43-transactional-item-adding.md) — cap-aware Add semantics, `SnapshotSlot`/`RestoreSlot`/`ValidatePostMutation` as a contrast with workspace save.
- [52 — Acquisition stride-2 sort order](52-acquisition-sort-stride2.md) — stride-2 algorithm, bucket-collision guard, write path in `ReorderInventory`/`ReorderStorage` and in `writeContainerLayout`.

---

## Sources

- `backend/core/transfer.go` (720 lines) — core engine, instance-move, quantity-merge, rehandle, equipped guard.
- `app.go:741+` — App-level wrapper `MoveItemsBetweenInventoryAndStorage`, cap resolution, push undo.
- `backend/editor/save.go:79+, :518+` — workspace save (`ApplyWorkspaceSave`) and layout writer (`writeContainerLayout`).
- `frontend/src/components/SortOrderTab.tsx` — two-column UI, workspace session as the only mode of operation.
- `frontend/src/hooks/useInventoryWorkspace.ts` — RAM-only operations.
- `tests/transfer_test.go` — 28 tests covering the legacy core path.
- `app_storage_order_test.go`, `app_inventory_order_test.go` — reorder bindings tests.
- `docs/CHANGELOG.md` — entries "feat(sort-order): dual-grid Inventory + Storage with bidirectional transfer".
