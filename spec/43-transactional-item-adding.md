# 43 — Transactional Item Adding: current write path

> **Type**: Design doc + binary write-path spec
> **Status**: ✅ canonical, implemented. Architecture confirmed against the current backend (`app.go::AddItemsToCharacter`, `backend/core/writer.go::AddItemsToSlotBatch`, `backend/core/snapshot.go`, `backend/core/diagnostics.go::ValidatePostMutation`).
> **Scope**: The current item-adding write path for a slot (PRE-FLIGHT → SNAPSHOT → MUTATE → POST-FLAGS → RECONCILE → VALIDATE → ROLLBACK-ON-FAILURE). The chapter shows how the application layer (`app.go`) orchestrates the allocator, container writes, companion flags and the validator into a single transactional operation. Allocator/capacity/counter invariants are documented canonically in [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md); the full list of item companion flags is in [50-item-companion-flags](50-item-companion-flags.md).

---

## Chapter goal

The chapter describes:

- where the Add Items pipeline begins in the application layer and what it orchestrates;
- how pre-flight works (capacity check, duplicate-index scan) — and why it must fail **before** mutation;
- how `AddItemsToSlotBatch` differentiates stackable goods/talismans from instance-backed weapon/armor/AoW;
- where and when the allocator is used (`generateUniqueHandle`, `allocateGaItem`, `upsertGaItemData`);
- how snapshot/rollback (`SnapshotSlot`/`RestoreSlot`) works and how it differs from the user-facing undo stack;
- how the validator (`ValidatePostMutation`) closes the transaction and when it forces a rollback;
- which companion flags are set together with items and how they are transactionally safe (best-effort logging vs hard rollback);
- what is outside the scope of Add Items.

What the chapter does **NOT** do:

- It does not duplicate the binary model of GaItem — [03-gaitem-map](03-gaitem-map.md).
- It does not duplicate allocator/counter/capacity invariants — [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md).
- It does not duplicate the Inventory/Storage read model — [07-inventory](07-inventory.md), [10-storage](10-storage.md).
- It does not describe Inventory ↔ Storage transfer semantics — [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- It does not describe stride-2 acquisition sort — [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md).
- It does not describe the full Ash of War semantics (sentinels, strict apply, availability) — [54-ash-of-war](54-ash-of-war.md).
- It does not describe the full taxonomy of companion flags — [50-item-companion-flags](50-item-companion-flags.md).

---

## Status

- canonical
- implemented (capacity tests, batch rebuild, post-validation, rollback)
- source-of-truth: backend code + `app.go`
- allocator details: **chapter 35**
- companion flags taxonomy: **chapter 50**

---

## Source of truth in code

| Topic | Files / functions | Note |
|---|---|---|
| App-level entry point | `app.go:320-644` (`AddItemsToCharacter`) | orchestrator of the full transaction; args: `charIdx, itemIDs, upgrade25, upgrade10, infuseOffset, upgradeAsh, invQty, storageQty` |
| Capacity report type | `backend/core/capacity.go:71-83` (`CapacityReport`) | fields: `CanFitAll`, `CapHit`, `Free*`, `Needed*` |
| Capacity pre-flight | `backend/core/capacity.go:85-222` (`CheckAddCapacity`, `CountSlotUsage`) | all-or-nothing, used before mutation |
| Duplicate-index pre-flight | `backend/core/diagnostics.go::ScanDuplicateInventoryIndices` + `app.go:344-350` | reject when existing duplicates of `Index` are present in inventory |
| Snapshot/rollback | `backend/core/snapshot.go:28-102` (`SnapshotSlot`, `RestoreSlot`, `SlotSnapshot`) | deep copy: `Data`, `GaItems`, `GaMap`, `Inventory`, `Storage`, all counters + dynamic offsets |
| Batch add | `backend/core/writer.go:273-376` (`AddItemsToSlotBatch`) | 3 phases: GaItems allocation, one `RebuildSlotFull`, write to `Inventory`/`Storage` |
| Allocator | `backend/core/writer.go:402-501` (`generateUniqueHandle`, `allocateGaItem`) | full description: [35](35-gaitem-allocator-invariants.md) |
| GaItemData (weapon/AoW metadata) | `backend/core/writer.go:11-...` (`upsertGaItemData`) | limit `GaItemDataMaxCount = 7000`; rejection on overflow (post-fix) |
| Slot rebuild | `backend/core/slot_rebuild.go:300-460` (`RebuildSlotFull`) | full re-serialization of sections preserving DLC + PlayerGameDataHash regions |
| Post-mutation validator | `backend/core/diagnostics.go:363-468` (`ValidatePostMutation`, `IntegrityError`) | 6 check keys, last line of defense |
| Header reconcile | `backend/core/writer.go:971-1013` (`ReconcileStorageHeader`, `ReconcileInventoryHeader`) | after batch add, before the validator collects checks |
| Containers / cap-aware add | `backend/db/data/container_requirements.go:88-...` (`GetRequiredContainer`, `ApplyContainerCap`) | gated items (Throwing Pots, Aromatics) — best-effort qty trim |
| Companion flags (event flags) | `backend/db/data/ash_of_war_flags.go::AoWItemToFlagID`, `world_pickup_flags.go::WorldPickupFlagID`, `bolstering_pickup_flags.go::BolsteringPickupFlags`, `item_companion_flags.go::CompanionEventFlagsForItem` | best-effort, logged warnings |
| Tutorial IDs | `core.AppendTutorialID` + `data.AboutTutorialID` | tutorial-completion flags for "About *" items |
| Container pickup flags | `data.ContainerPickupFlags`, `data.ContainerVendorPurchaseFlags` | flags activated for the container item (Cracked Pot etc.) |
| Result type | `app.go:295-310` (`SkippedAdd`, `AddResult`) | propagated to JS: `Added`, `Requested`, `Trimmed`, `CapHit`, `Free*`, `Needed*` |
| Undo stack | `app.go::pushUndo` | separate mechanism from snapshot; push before mutation, independent of rollback |

---

## Pipeline overview

```
AddItemsToCharacter(charIdx, itemIDs, upgrades..., invQty, storageQty)
│
├── PRE-FLIGHT (no mutation)
│   ├── 1a. ScanDuplicateInventoryIndices(slot)
│   │   └── len(dups) > 0 → return error, slot untouched
│   ├── 1b. Build existing-qty maps (inv, storage, key items, containers)
│   ├── 1c. FCFS sort: gated items first (ascending ID), the rest stable
│   ├── 1d. Pre-compute prepared items
│   │   ├── apply upgrade25 / upgrade10 / infuseOffset / upgradeAsh
│   │   ├── trim qty for stackable already-at-max
│   │   └── apply container caps (Throwing Pots/Aromatics) → trimmed[]
│   └── 1e. CheckAddCapacity(slot, capacityItems)
│       └── !CanFitAll → return AddResult{CapHit, Free*, Needed*}, slot untouched
│
├── SNAPSHOT
│   ├── pushUndo(charIdx)            ← user-facing undo
│   └── snapshot = SnapshotSlot(slot)  ← internal safety net
│
├── MUTATE
│   ├── AddItemsToSlotBatch(slot, capacityItems)
│   │   ├── Phase 1: GaItems allocation (allocateGaItem, generateUniqueHandle, upsertGaItemData)
│   │   ├── Phase 2: RebuildSlotFull(slot) — ONE rebuild for the whole batch
│   │   └── Phase 3: write to Inventory.CommonItems / Storage.CommonItems
│   └── error → RestoreSlot(slot, snapshot), return wrapped error
│
├── POST-FLAGS (best-effort, log warnings, no rollback)
│   ├── AoW item flag (AoWItemToFlagID)
│   ├── World pickup flag (WorldPickupFlagID)
│   ├── Bolstering pickup flags (BolsteringPickupFlags)
│   ├── Tutorial ID (AboutTutorialID via AppendTutorialID)
│   └── Companion event flags (CompanionEventFlagsForItem)
│
├── CONTAINER KEY ITEMS (transactional, rollback-on-fail)
│   ├── Per used container: AddItemsToSlot(slot, [cID], desired, 0, false)
│   │   └── error → RestoreSlot(slot, snapshot), return wrapped error
│   └── Container pickup flags + Vendor purchase flags (best-effort)
│
├── RECONCILE
│   └── ReconcileStorageHeader(slot)
│
├── POST-VALIDATION
│   └── violations := ValidatePostMutation(slot)
│       └── len(violations) > 0 → RestoreSlot(slot, snapshot), return error with the first violation
│
└── SUCCESS
    └── return AddResult{Added, Trimmed, FreeInv, FreeStore}
```

---

## Entry points and slot selection

Single public entry point: `App.AddItemsToCharacter(charIdx int, itemIDs []uint32, upgrade25, upgrade10, infuseOffset, upgradeAsh, invQty, storageQty int) (AddResult, error)` (`app.go:327`).

- `charIdx` — character slot (`0..9`). Validation: `charIdx < 0 || charIdx >= 10` → error.
- `itemIDs` — list of **base** item IDs (canonical DB values, e.g., `0x000F4240` Uchigatana, `0x40001234` goods).
- `upgrade25`, `upgrade10` — additive shifts for weapons with `MaxUpgrade == 25` / `MaxUpgrade == 10`. The frontend selects the upgrade level globally for the whole batch.
- `infuseOffset` — additive shift for items supporting infusion (`weaponCategorySupportsInfusion`: `melee_armaments` + `shields`). Bows/crossbows/staves/seals do **not** receive an infuse offset, despite `MaxUpgrade == 25` — because the game does not recognize their infused IDs (comment in `app.go:312-318`).
- `upgradeAsh` — shift for `category == "ashes"` (spirit ashes).
- `invQty`, `storageQty` — requested quantity per item: `0` = skip, `-1` = max (from `db.ItemData.Max*`), `> 0` = `min(qty, max)`.

Pre-validation `a.save == nil` → error "no save loaded" before any work.

---

## Batch add model

The whole operation is **batched**: one list of `itemIDs` → one `AddResult`. Performance benefit (`writer.go:271-272`):

> "All GaItem allocations happen in Phase 1, then ONE RebuildSlotFull in Phase 2, then all inventory/storage writes in Phase 3. This is O(1) rebuilds instead of O(N)."

In a traditional "per item" approach every addition would require a slot rebuild and a re-validation of dynamic offsets. Batching reduces this to a single operation.

`AddItemsToSlotBatch` takes a `[]ItemToAdd` (`backend/core/capacity.go:62-69`):

```go
type ItemToAdd struct {
    ItemID         uint32  // already after all shifts (upgrade, infuse, ash)
    InvQty         int     // quantity to add to Inventory.CommonItems
    StorageQty     int     // quantity to add to Storage.CommonItems
    ForceStackable bool    // force stackable treatment (e.g., for arrows)
    IsStackable    bool    // hint from capacity check (informational)
}
```

The frontend never constructs `ItemToAdd` directly — `AddItemsToCharacter` maps `itemIDs` → `capacityItems` after pre-flight.

---

## Stackable goods vs instance-backed items

The allocator differentiates two allocation models by handle prefix (`writer.go:299-349`):

| Type | Handle prefix | Model |
|---|---|---|
| Goods | `0xB0` (`ItemTypeItem`) | **stackable**: one GaItem shared between inv + storage records |
| Accessory / Talisman | `0xA0` (`ItemTypeAccessory`) | **stackable** per item-ID (lookup of existing handle in `slot.GaMap`) |
| Weapon | `0x80` (`ItemTypeWeapon`) | **instance-backed**: a separate GaItem for each record (per destination) |
| Armor | `0x90` (`ItemTypeArmor`) | **instance-backed** |
| AoW gem | `0xC0` (`ItemTypeAow`) | **instance-backed** |
| Arrow / Bolt | `0x80` but `db.IsArrowID(id)` | forced as stackable (`ForceStackable = true`) |

### Stackable path (`writer.go:303-329`)

1. Looks for an existing handle in `slot.GaMap` by matching ItemID.
2. If found: reuses the handle for both `invQty` and `storageQty`.
3. If not found:
   - **goods (`0xB0`) or accessory (`0xA0`)**: constructs a synthetic handle `(itemID & 0x0FFFFFFF) | prefix` and writes `slot.GaMap[handle] = itemID` (no GaItem entry allocation — the handle exists only in the map).
   - **forced stackable (e.g., arrows)**: regular GaItem allocation (`allocNewGaItem`).
4. Appends **one** `pendingInv` with (handle, invQty, storageQty).

### Instance-backed path (`writer.go:331-349`)

1. If `invQty != 0`: allocates a separate GaItem via `allocNewGaItem` (handle + `allocateGaItem` + `upsertGaItemData` if weapon/AoW non-arrow).
2. If `storageQty != 0`: a **second** separate GaItem (separate handle, separate entry).
3. Appends **two** separate `pendingInv` if both containers receive the item.

> ⚠️ Sharing a handle between inv and storage for **non-stackable** items is intentionally forbidden — comment in `writer.go:332-333`: "see AddItemsToSlot for the explanation of why sharing a handle corrupts the save." Full analysis in [03 → GaItems vs containers](03-gaitem-map.md#gaitems-vs-containers) and [53-inventory-storage-transfer](53-inventory-storage-transfer.md) (rehandle path for duplicate-handle transfer).

---

## GaItem allocator integration

`AddItemsToSlotBatch` uses the allocator helper `allocNewGaItem(id, handlePrefix)` (`writer.go:282-297`):

```go
allocNewGaItem := func(id, handlePrefix uint32) (uint32, error) {
    h, err := generateUniqueHandle(slot, handlePrefix)
    if err != nil {
        return 0, err
    }
    if err := allocateGaItem(slot, h, id); err != nil {
        return 0, err
    }
    slot.GaMap[h] = id
    if (handlePrefix == ItemTypeWeapon && !db.IsArrowID(id)) || handlePrefix == ItemTypeAow {
        if err := upsertGaItemData(slot, id); err != nil {
            return 0, err
        }
    }
    return h, nil
}
```

Integration points:

1. **`generateUniqueHandle`** — allocates a new handle from the `slot.NextGaItemHandle` counter, checks uniqueness in `slot.GaMap`, limit 10000 attempts (`MaxHandleAttempts`). Full mechanics: [35 → Handle generation](35-gaitem-allocator-invariants.md#handle-generation).
2. **`allocateGaItem`** — inserts a record into `slot.GaItems[NextAoWIndex]` (for AoW) or `slot.GaItems[NextArmamentIndex]` (for weapon/armor/talisman), updates cursors zone-aware. **The AoW guard after commit `6881cb9`** rejects alloc when `NextArmamentIndex >= len(GaItems)`. Full mechanics: [35 → Allocation zones](35-gaitem-allocator-invariants.md#allocation-zones).
3. **`upsertGaItemData`** — adds an entry to the `GaItemData` section (weapon/AoW metadata; `GaItemDataMaxCount = 7000`). Only for weapon (non-arrow) and AoW. Reject on overflow.

> **Implementer note**: The allocator should fail **pre-mutation** or early in the batch — the error message must be readable (e.g., "armament zone at capacity (NextArmamentIndex 5120 == 5120)"). The post-mutation validator is the last line of defense, **not** the primary error path. A full pathological example (a small number of non-empty entries but armament zone full): [35 → UI counters vs allocator capacity](35-gaitem-allocator-invariants.md#ui-counters-vs-allocator-capacity).

---

## Inventory and Storage mutation boundaries

Phase 3 in `AddItemsToSlotBatch` (after `RebuildSlotFull`) iterates `pending` and writes per-record to the binary:

- **Inventory writes**: `addToInventory(slot, handle, invQty)` (if `invQty > 0`).
- **Storage writes**: `addToStorage(slot, handle, storageQty)` (if `storageQty > 0`).

Those functions overwrite the first empty slot in the appropriate array and set `{handle, quantity, index}`. `index` is assigned from `NextEquipIndex` / `NextAcquisitionSortId` on the container side.

After the pending loop:

- `ReconcileStorageHeader(slot)` (`writer.go:971-991`) — synchronizes `storage_count` header with the non-empty count.
- `ReconcileInventoryHeader(slot)` — analogously for inventory `common_item_count`.

Boundaries:
- `AddItemsToSlotBatch` **does not** modify `Storage.KeyItems` (storage key items are not runtime-exposed; see [10 → Read path](10-storage.md#read-path)).
- `Inventory.KeyItems` is modified exclusively in the higher-layer "Container key items" section (`app.go:582-595`), not in the core batch.

---

## Item companion flags

Some items require setting flags/events together with the addition to inventory. Save Forge orchestrates this in the POST-FLAGS section in `AddItemsToCharacter` (`app.go:526-579`).

### Mappings (best-effort, log warning on error)

| Mapping | File | Situation |
|---|---|---|
| `data.AoWItemToFlagID[itemID] → flagID` | `backend/db/data/ash_of_war_flags.go` | duplication-prevention flag for Lost Ash of War: prevents the game from granting it again |
| `data.WorldPickupFlagID[itemID] → flagID` | `backend/db/data/world_pickup_flags.go` | "picked up in the world" — flag for items with a fixed pickup location |
| `data.BolsteringPickupFlags[itemID] → []flagID` | `backend/db/data/bolstering_pickup_flags.go` | per-instance: for each added Golden Seed / Sacred Tear etc., the next flag from the list is set |
| `data.AboutTutorialID[itemID] → tutorialID` | `backend/db/data/tutorial_ids.go` | "About *" items completed in the tutorial system; set via `core.AppendTutorialID` (`backend/core/tutorial_data.go:78`) |
| `data.CompanionEventFlagsForItem(itemID) → []flagID` | `backend/db/data/item_companion_flags.go` | additional event flags associated with a specific item (e.g., quest steps) |

> ⚠️ **Full list of companion flags and their semantics** — [50-item-companion-flags](50-item-companion-flags.md). Chapter 43 describes only **where in the pipeline** they are set (after batch add, before reconcile) and **the transactional guarantees** (best-effort: log warn on error, no rollback).

### Transactional safety of companion flags

A companion-flag failure **does not** trigger batch rollback. Every `db.SetEventFlag` / `AppendTutorialID` call is wrapped in `if err != nil { runtime.LogWarning... }` (`app.go:530-579`). Rationale:

- Flags are auxiliary for the in-game experience (UI shows the item as "already collected"), not critical for save integrity.
- A hard rollback after a failed `SetEventFlag` would leave the user without items that the allocator and validator have already accepted.
- Failure space: only `slot.EventFlagsOffset <= 0` or a bit out of range — very rare.

### Container key items (rollback-on-fail)

In contrast to companion flags, **container items** (Empty Cracked Pot for Throwing Pots, Empty Cracked Ritual Pot for Aromatics etc.) are added via a separate `AddItemsToSlot` in the "Auto-add containers" section (`app.go:582-595`):

```go
if err := core.AddItemsToSlot(slot, []uint32{cID}, desired, 0, false); err != nil {
    core.RestoreSlot(slot, snapshot)
    return result, fmt.Errorf("rollback after container add: %w", err)
}
```

A container-item failure (e.g., a capacity hit for key items) **triggers rollback** — because without the container, Throwing Pots cannot be used in the game.

---

## Snapshot, rollback and failure semantics

### Snapshot (pre-mutation)

`snapshot := SnapshotSlot(slot)` (`snapshot.go:28-67`) performs a deep copy of every mutable field of `SaveSlot`:

- `slot.Data` (the full byte buffer)
- `slot.Version`, `slot.Player`
- `slot.GaMap`, `slot.GaItems`
- `slot.Inventory.Clone()`, `slot.Storage.Clone()`
- `slot.Warnings`
- All dynamic offsets (`MagicOffset`, `InventoryEnd`, `EventFlagsOffset`, `PlayerDataOffset`, `FaceDataOffset`, `StorageBoxOffset`, `IngameTimerOffset`, `GaItemDataOffset`, `TutorialDataOffset`)
- All counters (`NextAoWIndex`, `NextArmamentIndex`, `NextGaItemHandle`, `PartGaItemHandle`)

### Rollback (`RestoreSlot`)

`RestoreSlot(slot, snap)` (`snapshot.go:69-102`) overwrites all those fields from the snapshot. Invoked in 3 places in `AddItemsToCharacter`:

1. After `AddItemsToSlotBatch` returns an error (`app.go:521-524`).
2. After a failed container key-item add (`app.go:590-593`).
3. After `ValidatePostMutation` returns non-empty violations (`app.go:626-629`).

In each case the slot returns to the state from before `SnapshotSlot`, and the function returns a wrapped error.

### Relationship with the user-facing undo

`pushUndo(charIdx)` (`app.go:517`) is called **before** `SnapshotSlot`. Snapshot and undo are **separate mechanisms**:

- **Undo stack**: the user can undo a successful Add Items operation. The stack contains copies of slot.Data from before the mutation.
- **Snapshot/Restore**: an internal safety net for failed mutation. It does not touch the undo stack.

Consequence: after a failed Add Items, the undo stack contains an unnecessary entry identical to the current state (idempotent undo). Acceptable, because undo statelessly reverts to the state from before pushUndo.

### Failure semantics — categories

| Phase | Failure | Effect |
|---|---|---|
| PRE-FLIGHT 1a | duplicate Index in existing inventory | reject, slot untouched, return descriptive error |
| PRE-FLIGHT 1e | capacity hit | reject, slot untouched, return `AddResult{CapHit, Free*, Needed*}` |
| MUTATE — Phase 1 (allocator) | `armament zone at capacity`, `array full`, handle overflow | RestoreSlot, return wrapped error |
| MUTATE — Phase 2 (RebuildSlotFull) | section overflow, regions issue | RestoreSlot, return wrapped error |
| MUTATE — Phase 3 (container writes) | overflow inv/storage (should be covered by pre-flight) | RestoreSlot, return wrapped error |
| POST-FLAGS | `SetEventFlag` error, `AppendTutorialID` error | log warning, **no rollback** |
| CONTAINER KEY ITEMS | failed `AddItemsToSlot` | RestoreSlot, return wrapped error |
| RECONCILE | no failure path | — |
| POST-VALIDATION | `ValidatePostMutation` returns violations | RestoreSlot, return error with the first violation |

---

## Post-mutation validation

`ValidatePostMutation(slot)` (`diagnostics.go:363-468`) is the **last line of defense**. The allocator and pre-flight should catch 100% of legal user-facing errors; the validator catches bugs (e.g., a counter mismatch after a regression in a new code path).

### Error categories (summary)

The full list of 6 unique check keys + details — [35 → Post-mutation validation](35-gaitem-allocator-invariants.md#post-mutation-validation). From the Add Items perspective the key ones are:

- **`orphan_handle`** — non-empty inventory handle of type `0x80/0x90/0xC0` without a matching entry in `GaMap`. Bug in the allocator path.
- **`duplicate_index`** — two `CommonItems ∪ KeyItems` entries with the same `Index`. Bug in stride-2 / sort logic.
- **`gaitemdata_count`** — `GaItemData count > 7000`. Should be caught by `upsertGaItemData` rejection.
- **`storage_count`** — `storage_count header != non-empty count`. Repaired by `ReconcileStorageHeader`; failure here = bug in the order of operations.
- **`gaitem_indices`** — `NextAoWIndex > NextArmamentIndex` or `NextArmamentIndex > len(GaItems)`. Allocator bug (e.g., bypass of the `6881cb9` AoW guard).
- **`gamap_zero_id`** — `GaMap[h] == 0`. Bug in `upsertGaItemData` / `allocateGaItem`.

### Why the validator is the "last line"

- The validator returns the first violation as a string — it loses context (which item, which position).
- The validator always forces rollback — it does not attempt to repair.
- The validator **does not** distinguish user-facing errors (capacity) from allocator bugs (counter mismatch). For user-facing errors, `AddResult.CapHit` is the proper channel.

Design rule: **the allocator + pre-flight catch 100% of legal errors. The validator catches bugs.**

---

## Capacity and UI counter caveat

Add Items **must not** rely on the UI `ALL ITEMS used/max` / `INVENTORY used/max` / `STORAGE used/max` as the primary capacity check. Those bars (`SlotCapacity` from `app_deploy.go::GetSlotCapacity`) report `non_empty / max` per container — i.e., **container-layer usage**, not allocator-side free space.

### What Add Items uses

`CheckAddCapacity(slot, items)` (`capacity.go:85-222`) counts **all** required resources per item:

- `NeededInv` / `FreeInv` — Inventory.CommonItems slots
- `NeededStorage` / `FreeStorage` — Storage.CommonItems slots
- `NeededGaItems` / `FreeGaItems` — GaItem entries (`len(GaItems)` minus non-empty)
- `NeededGaItemData` / `FreeGaItemData` — `GaItemData` slots (`GaItemDataMaxCount = 7000`)

The first hit sets `CapHit`:

```go
if neededGaItemData > FreeGaItemData      { CapHit = "gaitemdata_full" }
else if neededGaItems > FreeGaItems       { CapHit = "gaitem_full" }
else if neededInvSlots > FreeInv          { CapHit = "inventory_full" }
else if neededStorageSlots > FreeStorage  { CapHit = "storage_full" }
```

### The armament zone trap

Even `CheckAddCapacity` saying "fits" **does not guarantee** allocator success — `FreeGaItems = GaItemsMax - non_empty count` is not the same as **armament zone free** = `len(GaItems) - NextArmamentIndex`. A save with many empty entries between cursors and `NextArmamentIndex == len(GaItems)` will pass pre-flight but fail in `allocateGaItem` with "armament zone at capacity". A full pathological example: [35 → UI counters vs allocator capacity](35-gaitem-allocator-invariants.md#ui-counters-vs-allocator-capacity).

> ⚠️ Zone-aware capacity is not currently exposed to pre-flight or UI — `needs verification` as a planned improvement (see [35 → Known limits](35-gaitem-allocator-invariants.md#known-limits--open-questions)). Currently such a failure is returned as a wrapped error after RestoreSlot, not as `AddResult{CapHit}`.

---

## Error handling and user-facing messages

`AddResult` (`app.go:300-310`) is the only success-path channel that propagates information to the UI:

```go
type AddResult struct {
    Added       int          // number of items actually added
    Requested   int          // len(itemIDs)
    Trimmed     []SkippedAdd // qty trimmed by container caps (best-effort)
    CapHit      string       // "" on success; otherwise "inventory_full" | ...
    FreeInv     int
    FreeStore   int
    NeededInv   int          // only if CapHit != ""
    NeededStore int
}
```

### Error channels

1. **`CapHit != ""`** — pre-flight capacity check failed. UI displays a "Not enough space" toast with `Needed* / Free*`. Slot untouched.
2. **`(AddResult, error)` with `error != nil`** — technical error (allocator failure, rebuild error, validation failure). UI logs the error string. Slot **restored** by RestoreSlot.
3. **`Trimmed != nil` but `CapHit == ""`** — partial success (container cap forced qty trim). UI shows `Added X / Requested Y`, optionally lists trimmed items. Slot modified.
4. **None of the above** — full success. UI refreshes the inventory view.

### Best-effort warnings (not propagated to AddResult)

- Companion flag failures are logged as `runtime.LogWarning` (Wails runtime logger). They do not return to JS as an error.
- Tutorial ID failures analogously.
- Vendor purchase flags analogously.

---

## Test coverage

| Test | What it verifies |
|---|---|
| `tests/capacity_test.go` (12 tests) | Pre-flight in different configurations (`TestPreFlightCapacity_Empty`, `TestPreFlightCapacity_CountsUsage`), batch add (`TestBatchAdd_SingleRebuild`, `TestBatchAdd_MixedStackableNonStackable`), snapshot/rollback (`TestSnapshotRestore_*`), post-validation (`TestPostValidation_*`), header reconcile (`TestStorageHeaderReconcile`), `TestGaItemDataFull_ErrorNotSilent`, `TestRoundtrip_BatchAdd` |
| `tests/bulk_add_test.go` | Stress test of batch add: armament zone capacity, mixed AoW+weapon, multi-category batch |
| `tests/bulk_add_test.go::TestAddItems_RespectsArmamentCapacity` | Allocator AoW guard `6881cb9` (post-fix) |
| `backend/core/gaitem_placement_test.go::TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity` | Allocator-level reject before mutation |
| `app_additems_duplicate_index_test.go` | Pre-flight 1a reject on existing duplicates of Index |
| `app_repair_duplicate_index_test.go` | `RepairDuplicateInventoryIndices` idempotency |
| `tests/item_companion_flags_test.go` | Companion flag setting on Add Items |
| `tests/grace_companion_flags_test.go` | Grace-specific companion flags |
| Round-trip fixtures (`tests/roundtrip_test.go`, `save_modify_test.go`) | Full integration: Add Items → Write → Reload → validate |

---

## Known limits / needs verification

- **Zone-aware capacity in pre-flight** — `CheckAddCapacity` does not compute `armament_zone_free = len(GaItems) - NextArmamentIndex`. A save with `NextArmamentIndex` close to `len(GaItems)` will pass pre-flight but fail in the allocator. Status: `needs verification` as a future task in [35](35-gaitem-allocator-invariants.md#known-limits--open-questions).
- **Best-effort companion flag failures** — there is no channel to propagate them to the UI. The user does not know that "About *" tutorial was not marked as completed. `needs verification` whether this causes observable issues in-game (UI duplicates tutorial after reload, fast-travel discovery does not work, etc.).
- **Vendor purchase flags** (`ContainerVendorPurchaseFlags`) — added for container items, but whether they actually hide the item at a vendor after Add Items — `needs verification`.
- **Full list of handle prefixes handled by the allocator** — `AddItemsToSlotBatch` assumes five types (`0x80/0x90/0xA0/0xB0/0xC0`). A handle with a different prefix is neither expected nor tested. `needs verification` whether `db.ItemIDToHandlePrefix` may return an unsupported value for edge-case ItemIDs.
- **`upsertGaItemData` failure path** — pre-fix (pre-`6881cb9` era) returned `nil` on overflow instead of an error. Currently it returns an error when `count >= GaItemDataMaxCount`. `needs verification` whether all call sites propagate this error correctly (a code review of the full call-tree).

---

## Cross-references

- [03-gaitem-map](03-gaitem-map.md) — binary model of GaItem/GaMap, handle prefixes, 21/16/8 B records.
- [07-inventory](07-inventory.md) — Inventory data model, `InventoryItem` 12 B, `CommonItems`/`KeyItems`, header reconcile.
- [10-storage](10-storage.md) — Storage data model, `ReadStorage`, `ReconcileStorageHeader`.
- [35-gaitem-allocator-invariants](35-gaitem-allocator-invariants.md) — allocator, capacity, counters, validation, snapshot/rollback (the canonical reference for all low-level write-side semantics).
- [50-item-companion-flags](50-item-companion-flags.md) — full taxonomy of companion flags set during Add Items.
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — acquisition `Index` assignment (Add Items uses `NextEquipIndex`/`NextAcquisitionSortId` from the container; the full sort mechanics are in 52).
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — separate operation (after Add Items); rehandle path for duplicate-handle.
- [54-ash-of-war](54-ash-of-war.md) — custom AoW semantics during Add Items (`upsertGaItemData` for AoW gems, `AoWGaItemHandle` sentinels).

---

## Sources

- `app.go` — `AddItemsToCharacter`, `AddResult`, `SkippedAdd`, `weaponCategorySupportsInfusion`, `resolveQty`
- `backend/core/writer.go` — `AddItemsToSlot`, `AddItemsToSlotBatch`, `allocateGaItem`, `generateUniqueHandle`, `upsertGaItemData`, `ReconcileInventoryHeader`, `ReconcileStorageHeader`
- `backend/core/capacity.go` — `CheckAddCapacity`, `CapacityReport`, `CountSlotUsage`, `SlotUsage`, `ItemToAdd`, `handlePrefixForStackable`, `needsGaItemData`
- `backend/core/snapshot.go` — `SnapshotSlot`, `RestoreSlot`, `SlotSnapshot`
- `backend/core/slot_rebuild.go` — `RebuildSlotFull`
- `backend/core/diagnostics.go` — `ValidatePostMutation`, `IntegrityError`, `ScanDuplicateInventoryIndices`
- `backend/db/data/container_requirements.go` — `GetRequiredContainer`, `ApplyContainerCap`
- `backend/db/data/{ash_of_war_flags,world_pickup_flags,bolstering_pickup_flags,container_pickup_flags,item_companion_flags,tutorial_ids}.go` — companion flag mappings
- `backend/core/tutorial_data.go` — `AppendTutorialID` (write path for `AboutTutorialID`)
- Tests: `tests/{capacity,bulk_add,item_companion_flags,grace_companion_flags,roundtrip,save_modify}_test.go`, `backend/core/gaitem_placement_test.go`, `app_additems_duplicate_index_test.go`, `app_repair_duplicate_index_test.go`
- Commit `6881cb9 fix(core): guard AoW allocation at armament capacity` — context for the AoW allocator guard
