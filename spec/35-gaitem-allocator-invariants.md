# 35 — GaItem Allocator & Invariants

> **Type**: Binary format spec + design doc
> **Status**: ✅ canonical, implemented (based on current backend code; last verified against branch `docs/lang-pl-book-cleanup`)
> **Scope**: Write-side model of `GaItem` entry allocation — capacity, counters, zones, sentinels, post-mutation validation, snapshot/rollback, transactional safety of batch add. This chapter complements [03-gaitem-map](03-gaitem-map.md), which describes only the binary layout. Here the editor semantics *on top of* that layout are described.

---

## Chapter goal

Save Forge must add and move entries in the `slot.GaItems` array in a way that the game will accept on load. This requires honoring a number of invariants that do not follow directly from the binary layout: frame cursors (`NextAoWIndex`, `NextArmamentIndex`), a global handle counter (`NextGaItemHandle`), slot-version-dependent capacity (5118 vs 5120 entries) and the prohibition of handle sharing. The chapter ties these rules together, describes the allocator (`allocateGaItem`), the capacity check (`CheckAddCapacity`), the validator (`ValidatePostMutation`) and the transactional snapshot/rollback (`SnapshotSlot`/`RestoreSlot`).

What the chapter does **NOT** do:
- It does not describe the full Ash of War semantics — that is [54-ash-of-war](54-ash-of-war.md).
- It does not describe Inventory ↔ Storage transfer — that is [53-inventory-storage-transfer](53-inventory-storage-transfer.md).
- It does not design UI capacity bar fixes — it only points out the existing gap (the "UI counters vs allocator capacity" section); the fix is out of scope for Phase 2.

---

## Source of truth in code

| Topic | Files / functions | Note |
|---|---|---|
| `slot.Version` field | `backend/core/structures.go:223`, read in `structures.go:281` | u32 at offset 0x00 of the raw slot data; `Version == 0` = empty slot |
| Capacity threshold | `backend/core/offset_defs.go:346-348` | `GaItemCountOld = 5118 (0x13FE)`, `GaItemCountNew = 5120 (0x1400)`, `GaItemVersionBreak = 81` |
| Capacity selection at load | `backend/core/structures.go:623-628` (`scanGaItems`) | the only place where the code consults `slot.Version` when sizing the array |
| Counter reconstruction | `backend/core/structures.go:681-722` (second pass of `scanGaItems`) | a deterministic function of `GaItems` state; no fallback |
| Allocator | `backend/core/writer.go:422-501` (`allocateGaItem`) | type-segregated placement, shift-right on conflict, AoW guard after commit `6881cb9` |
| Unique handle generator | `backend/core/writer.go:402-419` (`generateUniqueHandle`) | iterates up to `MaxHandleAttempts = 10000` (`offset_defs.go:290`) |
| Capacity pre-flight | `backend/core/capacity.go:7-238` (`SlotUsage`, `CountSlotUsage`, `CheckAddCapacity`, `CapacityReport`) | maps stackable/non-stackable onto needed slots/entries |
| GaItemData (`upsertGaItemData`) | `backend/core/writer.go:11-...` | weapon metadata registration; limit `GaItemDataMaxCount = 7000` |
| Post-mutation validation | `backend/core/diagnostics.go:363-468` (`ValidatePostMutation`, `IntegrityError`) | 6 unique check keys (gaitem_indices produces 2 sub-violation messages); last line of defense |
| Snapshot/rollback | `backend/core/snapshot.go:28-102` (`SlotSnapshot`, `SnapshotSlot`, `RestoreSlot`) | deep copy: `Data`, `GaItems`, `GaMap`, `Inventory`, `Storage`, all counters, all dynamic offsets |
| Batch add | `backend/core/writer.go:109` (`AddItemsToSlot`), `backend/core/writer.go:273` (`AddItemsToSlotBatch`) + `backend/core/slot_rebuild.go:300` (`RebuildSlotFull`) | one rebuild instead of N |
| Repair orphans | `backend/core/writer.go:1015` (`RepairOrphanedGaItems`) | clears entries whose handle does not appear in any inventory/storage; idempotent, returns int |
| Repair duplicate index | `backend/core/inventory_index_repair.go::RepairDuplicateInventoryIndices` | reassigns `Index` for duplicates in `Inventory.CommonItems` + `KeyItems` |
| AoW availability scanner | `backend/core/aow_availability.go::ScanAoWAvailability` | detects a shared handle between two weapons |
| UI capacity binding | `app_deploy.go:31-38, 289-306` (`SlotCapacity`, `GetSlotCapacity`) | exposes `non_empty / max` per container — not `armament zone free` |
| Top-level orchestrator | `app.go:327-644` (`AddItemsToCharacter`) | full chain pre-flight → snapshot → batch → post-flags → reconcile → validate → rollback-on-error |

---

## GaItem capacity by slot version

The `slot.GaItems` array capacity depends on the version of a specific slot, not on the save as a whole:

| `slot.Version` | `len(slot.GaItems)` | Constant in code |
|---|---|---|
| `0` (empty/unused slot) | `GaItemCountNew = 5120` (the `Version > 0 && Version <= 81` condition is not satisfied) | all entries remain empty |
| `1 .. 81` | `GaItemCountOld = 5118` | `GaItemVersionBreak = 81` |
| `> 81` | `GaItemCountNew = 5120` | newer saves after a game patch |

The decision is made **once**, in `scanGaItems` (`structures.go:623-628`):

```go
maxEntries := GaItemCountNew
if s.Version > 0 && s.Version <= GaItemVersionBreak {
    maxEntries = GaItemCountOld
}
s.GaItems = make([]GaItemFull, maxEntries)
```

Entries outside the data range in the file (when the section ends earlier) are filled with the "empty" value (`GaItemFull{Unk2: -1, Unk3: -1, AoWGaItemHandle: 0xFFFFFFFF}`, `structures.go:632`).

> ℹ️ The constants `5118` and `81` also appear in `tests/save_integrity_test.go:132-134`. The test intentionally replicates the production logic for regression — every change to the threshold requires updating the test together with the code.

---

## Runtime capacity rule

After load **no other code path** reads `slot.Version` for operations on `GaItems`. The runtime source-of-truth for capacity is:

```go
len(slot.GaItems)
```

This rule is implemented consistently:

| File:line | What uses it |
|---|---|
| `writer.go:443` | `maxEntries := len(slot.GaItems)` in `allocateGaItem` |
| `writer.go:517` | `maxBuf := len(slot.GaItems) * GaRecordWeapon` in `FlushGaItems` (deprecated) |
| `slot_rebuild.go:318` | `maxBuf := len(slot.GaItems) * GaRecordWeapon` in `RebuildSlotFull` |
| `capacity.go:22` | `GaItemsMax: len(slot.GaItems)` in `CountSlotUsage` |
| `diagnostics.go:145` | log: `len(slot.GaItems)` |
| `diagnostics.go:453-456` | `if slot.NextArmamentIndex > len(slot.GaItems)` in `ValidatePostMutation` |
| `snapshot.go:40` | `gaItemsCopy = make([]GaItemFull, len(slot.GaItems))` |

**Implication for implementers**: never assume `5120` after load. A slot with `Version == 81` has `len = 5118`; any hardcoded `5120` in a new code path will cause out-of-bounds when operating on such a slot.

---

## Counter reconstruction on load

After parsing `GaItems` entries, the parser walks the array a second time (`structures.go:685-722`) and computes counters deterministically:

```go
s.NextAoWIndex = 0
s.NextArmamentIndex = 0
s.NextGaItemHandle = 0
s.PartGaItemHandle = 0x80 // default

maxGlobalCounter := uint32(0)
maxCounterIndex := 0

for i, g := range s.GaItems {
    if g.IsEmpty() { continue }
    h := g.Handle
    typeBits := h & GaHandleTypeMask

    // last AoW position + 1
    if typeBits == ItemTypeAow {
        s.NextAoWIndex = i + 1
    }

    // global handle counter — highest 16-bit counter across all types
    counter := h & 0xFFFF
    if counter >= maxGlobalCounter {
        maxGlobalCounter = counter
        maxCounterIndex = i
    }
}

s.NextArmamentIndex = maxCounterIndex + 1
s.NextGaItemHandle = maxGlobalCounter + 1
```

Properties:
- **No fallback** — the counters follow exclusively from the state of `GaItems`.
- Empty slot (`Version == 0`, all entries empty) → all counters `0`.
- `PartGaItemHandle` (bits 16-23 of the first non-empty handle) has default `0x80`; if no handle has a non-zero `(h >> 16) & 0xFF`, the default stays.
- The logic replicates Rust ER-Save-Editor (`inventory/mod.rs::from_save`).

---

## Counter meanings

| Counter | `SaveSlot` field | Definition | Behavior on allocation |
|---|---|---|---|
| `NextAoWIndex` | `slot.NextAoWIndex int` (`structures.go:256`) | The position where the **next** AoW (handle prefix `0xC0`) will be inserted. Represents the "right edge" of the left AoW zone. | `allocateGaItem` places the new AoW at `slot.GaItems[NextAoWIndex]`, then increments |
| `NextArmamentIndex` | `slot.NextArmamentIndex int` (`structures.go:257`) | The position where the next weapon/armor/talisman (prefix `0x80`, `0x90`, `0xA0`) will be inserted. Represents the "right edge" of the armament zone. AoW alloc also increments it (because it shifts the armament zone right). | `allocateGaItem` places a non-AoW at `slot.GaItems[NextArmamentIndex]`, then increments |
| `NextGaItemHandle` | `slot.NextGaItemHandle uint32` (`structures.go:258`) | Global counter of the lower 16 bits of the handle. `generateUniqueHandle(prefix)` uses it as a base: `prefix \| (counter & 0xFFFF) \| (PartGaItemHandle << 16)`. Incremented after every handle allocation. | grows monotonically up to `MaxHandleAttempts = 10000` tries; wrap-around is rejected as overflow |

> ℹ️ The `PartGaItemHandle` field (1 byte) is an internal "part" tag used in the handle layout. Default `0x80`. It affects `(h >> 16) & 0xFF` — it is not an allocation counter and is not subject to the same invariants as the three main ones.

---

## Required invariants

These invariants must hold after every completed slot mutation. Violating any of them triggers rollback in `app.go::AddItemsToCharacter` (`ValidatePostMutation` returns non-empty `[]IntegrityError`):

| ID | Invariant | Enforcing code |
|---|---|---|
| **I1** | `len(slot.GaItems) ∈ {GaItemCountOld, GaItemCountNew}` — chosen per `slot.Version` | `structures.go::scanGaItems` (load-time) |
| **I2** | `0 ≤ NextAoWIndex ≤ NextArmamentIndex ≤ len(GaItems)` | `diagnostics.go:448-460` (check `gaitem_indices`) |
| **I3** | AoW allocation rejected when `NextArmamentIndex >= len(GaItems)` | `writer.go:461-463` (commit `6881cb9`); see the "AoW allocation edge case" section |
| **I4** | Every non-empty inventory handle of type `0x80/0x90/0xC0` exists in `slot.GaMap` | `diagnostics.go:373-389` (check `orphan_handle`) |
| **I5** | No duplicate `Index` values in the combined set of `Inventory.CommonItems` + `Inventory.KeyItems` | `diagnostics.go:392-419` (check `duplicate_index`); repair: `RepairDuplicateInventoryIndices` |
| **I6** | `GaItemData count ≤ GaItemDataMaxCount (7000)` | `diagnostics.go:421-430` (check `gaitemdata_count`); reject in `upsertGaItemData` |
| **I7** | `slot.Data[StorageBoxOffset:][:4]` (storage header count) == number of non-empty records in `Storage.CommonItems` | `diagnostics.go:434-465` (check `storage_count`); repair: `ReconcileStorageHeader` |
| **I8** | No `slot.GaMap[h] == 0` (handle → `itemID 0` not allowed) | `diagnostics.go:469-477` (check `gamap_zero_id`) |
| **I9** | Allocation **must not** move any counter beyond `len(GaItems)` | `writer.go::allocateGaItem` (gates at lines 447, 461, 482) — returns an error before mutating |
| **I10** | No two weapons (`0x80...`) reference the same non-sentinel `AoWGaItemHandle` (`0xC0xxxxxx`) | `aow_availability.go::ScanAoWAvailability` (field `HasSharedHandleConflict`); full analysis in [54-ash-of-war](54-ash-of-war.md) |

Invariants I1–I9 are **local** to the allocator and adjacent systems (capacity, validation). I10 is cross-cutting with AoW — it is listed here, but the details are in 54.

---

## Allocation zones

The `GaItems` array is logically divided into two zones with no physical separator — the boundary is the counter values:

```
index:  0 ─────────────────────────── len(GaItems)
        │ AoW zone │ armament zone   │
        │  0xC0    │ 0x80, 0x90, 0xA0 │
        ↑          ↑                  ↑
     start    NextAoWIndex    NextArmamentIndex
```

### AoW zone (prefix `0xC0`)

- Grows from index `0` to the right.
- `allocateGaItem` with `handleType == ItemTypeAow` inserts at `NextAoWIndex`, then increments **both** `NextAoWIndex` and `NextArmamentIndex` (because inserting an AoW pushes the armament zone right by 1).
- Conflict (position taken): shift-right to the first empty entry (`writer.go:464-475`).

### Armament zone (prefix `0x80`, `0x90`, `0xA0`)

- Grows from `NextArmamentIndex` to the right (on the right of the AoW zone).
- `allocateGaItem` with a non-AoW handle inserts at `NextArmamentIndex`, increments only `NextArmamentIndex` (`writer.go:481-498`).
- Conflict: shift-right the same way as for AoW.

### Handle generation

`generateUniqueHandle(slot, prefix)` (`writer.go:402-420`):

1. Copies `slot.NextGaItemHandle` to a **local** variable `counter`.
2. Builds the candidate: `h := prefix | (uint32(slot.PartGaItemHandle) << 16) | counter`.
3. Checks whether `h` already exists in `slot.GaMap`. If it does — increments the **local** `counter`, recomputes `h` and tries again.
4. After finding a free handle it writes `slot.NextGaItemHandle = counter + 1` and returns `h`. The `slot.NextGaItemHandle` field is updated **only on success**, never during the loop.
5. Limit: `MaxHandleAttempts = 10000` iterations; after exhaustion it returns the error `"failed to generate unique handle after %d attempts (prefix 0x%X)"`.
6. The lower 16 bits (`counter & 0xFFFF`) give a theoretical maximum population of 65536 handles per `prefix` per `partID`; in practice the limit is enforced by `MaxHandleAttempts` long before that (unless someone supplies a bad partID and narrows the space).

### AoW ↔ armament relationship

- AoW alloc is **two-sided** — modifies both cursors.
- Armament alloc is **one-sided** — modifies only `NextArmamentIndex`.
- Consequence: after a long-running save (lots of in-game pickups, lots of AoW via Lost Ashes), the AoW zone may occupy most of the array. A save with `NextAoWIndex == 500`, `NextArmamentIndex == 5120`, but `non_empty == 60` is **a legal state** — which follows from the fact that empty entries on the right of the cursors also count toward the "zone width".

---

## AoW allocation edge case fixed by 6881cb9

> ⚠️ **Historical bug + current invariant — read carefully when implementing an allocator.**

### Before the fix

An earlier version of `allocateGaItem` in the AoW branch checked only whether `NextAoWIndex < maxEntries` (i.e., whether the left zone had room). After inserting it incremented `NextArmamentIndex` unconditionally. If the save had `NextArmamentIndex == len(GaItems)` (e.g., from a regular play-through PS4 where the armament zone grows to the end of the array), AoW alloc would:

1. Pass the `NextAoWIndex < maxEntries` check.
2. Insert AoW into `slot.GaItems[NextAoWIndex]`.
3. Increment `NextArmamentIndex` from `len(GaItems)` to `len(GaItems) + 1`.
4. `ValidatePostMutation` would detect `NextArmamentIndex > len(GaItems)` (check `gaitem_indices`) → rollback.
5. The user saw "post-mutation validation failed" with a numeric message instead of a readable capacity error.

### After the fix (`6881cb9 fix(core): guard AoW allocation at armament capacity`)

The allocator adds a guard **before mutation** (`writer.go:461-463`):

```go
if slot.NextArmamentIndex >= maxEntries {
    return fmt.Errorf(
        "allocateGaItem: cannot insert AoW — armament zone at capacity (NextArmamentIndex %d == %d)",
        slot.NextArmamentIndex, maxEntries)
}
```

Effects:
- Alloc failure produces a readable, allocator-level capacity message instead of a post-validation numeric error.
- `ValidatePostMutation` remains the last line of defense — if someone added a new write path that bypassed this guard, the validator would still catch the I2 violation.
- Regression lock-in: `backend/core/gaitem_placement_test.go:324-344` (`TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity`).
- Batch coverage: `tests/bulk_add_test.go::TestAddItems_RespectsArmamentCapacity` (line 669+).

### Guideline for implementers

Every new path that constructs an entry in `GaItems` (e.g., a new item type, a custom alloc for a test, diagnostic tools) **must** use `allocateGaItem` or replicate this guard. Do not assume that the post-mutation validator "is enough" — it only rolls back changes, it does not improve the user-facing error message.

---

## UI counters vs allocator capacity

`SlotCapacity` (`app_deploy.go:31`) exposes three `used / max` pairs to JS:

```go
type SlotCapacity struct {
    GaItemsUsed   int  // count(!IsEmpty(g)) for g in slot.GaItems
    GaItemsMax    int  // len(slot.GaItems)
    InventoryUsed int  // non-empty handles in Inventory.CommonItems
    InventoryMax  int  // CommonItemCount = 2688
    StorageUsed   int  // non-empty handles in Storage.CommonItems
    StorageMax    int  // StorageCommonCount = 1920
}
```

The UI bar `All Items used / max` (`frontend/src/App.tsx:529`) shows the **count of non-empty entries against the array size**. This is **not** the number of free allocation slots.

### Pathological example

Suppose a save after a long play-through:

- `len(slot.GaItems) = 5120`
- `gaItemsUsed = 59` (that many non-empty entries in the array)
- `NextArmamentIndex = 5120` (the armament zone has reached the end of the array; empty entries are **between** cursors and on the left, not on the right)
- `NextAoWIndex = 500`

The UI shows `ALL ITEMS 59/5120` — suggesting 5061 free. The user clicks "Add Weapon" → the allocator returns:

```
allocateGaItem: armament/armor array full (index 5120 >= 5120)
```

An attempt to add an AoW (through a path that performs this allocation) hits the `6881cb9` guard:

```
allocateGaItem: cannot insert AoW — armament zone at capacity (NextArmamentIndex 5120 == 5120)
```

### Where the difference comes from

- "Free" in the UI = an `IsEmpty()` entry. Empty entries exist all over the array (the game uses `0x00000000` or `0xFFFFFFFF` as a placeholder); the shift-right in `allocateGaItem` moves contents around, but empty entries between cursors are not available for a new alloc without overwriting.
- "Free" for the allocator = a position the cursor points at **and** that fits in the array. Practical measure for both alloc kinds:

  ```
  allocator_free = len(slot.GaItems) - NextArmamentIndex
  ```

  - **Weapon/Armor/Talisman alloc** requires `NextArmamentIndex < len(GaItems)` (`writer.go:482-483`).
  - **AoW alloc** requires `NextAoWIndex < len(GaItems)` **and** `NextArmamentIndex < len(GaItems)` (`writer.go:447-463`) — because AoW alloc increments **both** cursors.
- In both cases **`NextArmamentIndex` is the hard limit**. The value `NextArmamentIndex - NextAoWIndex` describes the current width of the armament zone (how many weapons/armor/talismans fit between the cursors), not "free room for AoW".

### What the chapter recommends to documentation

In chapters [07-inventory](07-inventory.md) and [10-storage](10-storage.md) add a callout about this gap. The UI fix (exposing `armament_zone_free` as a separate field in `SlotCapacity`) is **out of scope** for Phase 2 — it requires a code change in `app_deploy.go` + `frontend/src/App.tsx`. Entry in future work / `docs/ROADMAP.md`.

---

## Transactional safety and rollback

`app.go::AddItemsToCharacter` (`app.go:327-644`) implements the full all-or-nothing cycle:

```
1. PRE-FLIGHT (no mutation)
   1a. ScanDuplicateInventoryIndices(slot) — reject the batch if the slot already has duplicate Index
   1b. Pre-compute finalIDs (with upgrades, infusions, container caps), trim qty
   1c. CheckAddCapacity(slot, items) — check all limits (inventory, storage, gaItems, gaItemData)
       → CapacityReport.CanFitAll == false → return AddResult{CapHit, ...}, slot untouched

2. SNAPSHOT
   2a. pushUndo(charIdx)
   2b. snapshot := SnapshotSlot(slot)

3. MUTATE
   3a. AddItemsToSlotBatch(slot, capacityItems)
       — allocates GaItems, adds to Inventory/Storage, one RebuildSlotFull
       — error → RestoreSlot(slot, snapshot), return error
   3b. POST-FLAGS (event flags, tutorials, container pickups) — best-effort, log warn

4. RECONCILE
   4a. ReconcileStorageHeader(slot) — reconcile the binary count

5. VALIDATE (last line of defense)
   5a. violations := ValidatePostMutation(slot)
       → len(violations) > 0 → RestoreSlot(slot, snapshot), return error with the first violation

6. COMMIT (implicit)
   6a. Return AddResult{Added, Trimmed, FreeInv, FreeStore}
```

### Snapshot scope

`SnapshotSlot` (`snapshot.go:28-67`) performs a deep copy:

- `slot.Data` (the full byte buffer)
- `slot.Version`
- `slot.Player` (PlayerGameData struct)
- `slot.GaMap` (handle → itemID map)
- `slot.GaItems` (slice of GaItemFull)
- `slot.Inventory.Clone()`, `slot.Storage.Clone()`
- `slot.Warnings`
- All dynamic offsets: `MagicOffset`, `InventoryEnd`, `EventFlagsOffset`, `PlayerDataOffset`, `FaceDataOffset`, `StorageBoxOffset`, `IngameTimerOffset`, `GaItemDataOffset`, `TutorialDataOffset`
- All counters: `NextAoWIndex`, `NextArmamentIndex`, `NextGaItemHandle`, `PartGaItemHandle`

`RestoreSlot` (`snapshot.go:69-102`) overwrites all those fields from the snapshot. **It does not** touch the undo stack — the snapshot is an independent safety mechanism inside a single operation.

### Mutual relationship with undo

`pushUndo` is called *before* `SnapshotSlot`. After a successful batch add, the undo stack contains the state from before the operation (user-facing "Undo"). After a failure, the rollback via `RestoreSlot` reverts runtime changes; the undo stack retains an added, unnecessary entry identical to the current state — acceptable, because undo statelessly rolls back to the snapshot from before pushUndo.

---

## Post-mutation validation

`ValidatePostMutation` (`diagnostics.go:363-468`) is the **last line of defense**. The allocator and other write-side paths should fail earlier (and with a readable message), because the validator only returns a generic description and forces rollback of the entire operation.

### Full list of checks (current code, 6 check keys)

| Check key | What it verifies | Source |
|---|---|---|
| `orphan_handle` | Every non-empty inventory handle of type `0x80/0x90/0xC0` exists in `slot.GaMap` | `diagnostics.go:373-389` |
| `duplicate_index` | No duplicate `Index` in `Inventory.CommonItems` ∪ `KeyItems` | `diagnostics.go:392-419` |
| `gaitemdata_count` | Header count `GaItemData ≤ GaItemDataMaxCount (7000)` | `diagnostics.go:421-430` |
| `storage_count` | Storage header count == number of non-empty records in `Storage.CommonItems` | `diagnostics.go:434-465` |
| `gaitem_indices` | `NextAoWIndex ≤ NextArmamentIndex` **and** `NextArmamentIndex ≤ len(GaItems)` (one check key, two sub-violation messages) | `diagnostics.go:448-460` |
| `gamap_zero_id` | `slot.GaMap[h] != 0` for every h | `diagnostics.go:469-477` |

> ℹ️ The validator may emit more than one `IntegrityError` for the same slot (e.g., multiple orphan handles, multiple Index duplicates). The number of entries in the returned `[]IntegrityError` depends on the slot state; the first entry is what `app.go::AddItemsToCharacter` propagates to the user-facing error string.

### Why the validator should not be the first line of defense

- The validator returns the first violation as a string — it loses context (which item ID, which position).
- The validator does not attempt to repair — it always rolls back the entire operation.
- The validator does not distinguish between a "user-facing error" (e.g., armament zone full) and an "allocator bug" (e.g., counter mismatch).

Design rule: **the allocator and pre-flight should catch 100% of legal user-facing errors. The validator catches bugs.**

---

## Test coverage

### Allocator (`backend/core/gaitem_placement_test.go`)

| Test | What it verifies |
|---|---|
| `TestAllocateGaItem_AoWAtLowIndex` (line 25) | AoW inserted at `NextAoWIndex` |
| `TestAllocateGaItem_WeaponAfterAoW` (line 48) | Weapon inserted past the AoW zone |
| `TestAllocateGaItem_TypeSegregation` (line 80) | AoW and armament do not mix positions |
| `TestAllocateGaItem_ShiftPreservesExisting` (line 132) | Shift-right on conflict preserves neighbouring entries |
| `TestAllocateGaItem_ReturnsFullErrorAtCapacity` (line 253) | Reject when `NextArmamentIndex == maxEntries` for weapon/armor |
| `TestAllocateGaItem_AoWRejectsWhenArmamentZoneAtCapacity` (line 324) | **The `6881cb9` guard** — error "armament zone at capacity" |
| `TestGenerateUniqueHandle_GlobalCounter` (line 158) | Monotonic counter; rejection on duplicates |
| `TestScanGaItems_TrackedIndices` (line 187) | Counter reconstruction at load |

### Capacity + batch (`tests/`)

| Test | What it verifies |
|---|---|
| `tests/capacity_test.go` (16 tests) | Pre-flight, all-or-nothing, container caps, post-validation, header reconcile, storage edge cases |
| `tests/bulk_add_test.go::TestAddItems_RespectsArmamentCapacity` (line 669) | Batch add does not exceed armament zone capacity |
| `tests/bulk_add_test.go::TestAddItems_BulkArmamentZoneOnly` (line 164) | Stress test at the armament zone boundary |
| `tests/bulk_add_test.go::TestAddItems_BulkAoWAndArmamentSplit` (line 336) | Mixed AoW + weapon batch |

### Snapshot/rollback and validator

| Test | What it verifies |
|---|---|
| `tests/capacity_test.go::TestAllOrNothing*` | Snapshot/restore on capacity hit |
| `app_additems_duplicate_index_test.go` | Reject on pre-existing duplicate Index |
| `app_repair_duplicate_index_test.go` | `RepairDuplicateInventoryIndices` idempotency |
| `backend/core/duplicate_index_scan_test.go` | `ScanDuplicateInventoryIndices` correctness |

### AoW guards and availability

| Test | What it verifies |
|---|---|
| `backend/core/aow_strict_test.go::TestAllocateGaItem_NewWeaponUsesZeroSentinel` (line 303) | A new weapon has `AoWGaItemHandle = NoCustomAoWHandle (0x00)` |
| `backend/core/aow_strict_test.go::TestScanAoWAvailability_ZeroSentinelNotCounted` | Sentinel does not count as a used AoW copy |
| `backend/core/aow_strict_test.go::TestScanAoWAvailability_LegacyFFFFFFFFSentinelNotCounted` | Legacy sentinel (`0xFFFFFFFF`) also ignored |

---

## Known limits / open questions

### UI capacity bar (`needs verification` in the UX sense, not code correctness)

- The `All Items used/max` bar in the UI is **misleading** after the `6881cb9` fix for saves with `NextArmamentIndex` close to `len(GaItems)`. The code is not incorrect — the UI simply does not reveal "armament zone free".
- Fixing it requires extending `SlotCapacity` with an `ArmamentZoneFree` field or equivalent and updating the frontend. This is **out of scope** for chapter 35 (and Phase 2 documentation).
- Entry in `docs/ROADMAP.md` as follow-up.

### Rehandle when armament zone is full — potential test gap

- Instance-move transfer (`backend/core/transfer.go::transferOne` rehandle path) calls `materializeRehandledInstance`, which uses `allocateGaItem`. If `NextArmamentIndex == len(GaItems)`, the allocator will return an error → `SkipReasonHandleAllocFailed`.
- I have not found an explicit test for this scenario in `tests/transfer_test.go`. **Status**: `needs verification` by adding a regression test (e.g., `TestMoveTalismanDuplicate_FailsWhenArmamentZoneFull`). The code is deterministic through the shared `allocateGaItem`, so this is not a bug — only missing regression lock-in.

### Storage Apply in-game verification

- Spec [53-inventory-storage-transfer](53-inventory-storage-transfer.md) lists Storage Apply verification on Steam Deck as "future work" / sanity check. The current status in 53 is `needs verification` until confirmed.
- Chapter 35 does not depend on this verification — the allocator and validator are covered by tests independently of the UI Storage Apply behaviour.

### `slot.Version` typical values in the wild

- The code has no explicit list of `Version` values. The most recent value observed from save fixtures: needs verification in [20-version-platform](20-version-platform.md). The `81` threshold was introduced historically by a game patch — the exact patch number is `needs verification`.

---

## Implementation checklist

A list for authors of new write-side paths in Save Forge and other Elden Ring save editors:

- [ ] **Never assume a hardcoded `5120`** in the runtime path. After `LoadSave` always use `len(slot.GaItems)`.
- [ ] **Read `slot.Version` only in the load path** (analogously to `scanGaItems`). After load, `len(slot.GaItems)` is the source of truth.
- [ ] **Do not trust `non_empty count` as allocator free space**. Check `NextArmamentIndex` (for weapon/armor/talisman) and `NextAoWIndex` (for AoW pre-check) before attempting allocation.
- [ ] **AoW alloc requires `NextArmamentIndex < len(GaItems)`**, not just `NextAoWIndex < len(GaItems)`. Inserting an AoW shifts the armament zone right.
- [ ] **Before mutation** call `CheckAddCapacity` (or its equivalent in a new path). All-or-nothing — do not leave a batch in a partially applied state.
- [ ] **Snapshot before mutation** via `SnapshotSlot`. Rollback via `RestoreSlot` on every error (including post-validation errors).
- [ ] **After mutation** call `ReconcileStorageHeader` (and `ReconcileInventoryHeader` if you changed inventory common count) before `ValidatePostMutation`. The validator will otherwise throw a `storage_count` violation.
- [ ] **Check `ValidatePostMutation`** always. Let it be unreachable (as in the current code), but treat it as a contractual integrity guarantee.
- [ ] **Never share `AoWGaItemHandle`** between two weapons. Violating I10 → `EXCEPTION_ACCESS_VIOLATION` when the game loads. Details in [54-ash-of-war](54-ash-of-war.md).
- [ ] **Use `generateUniqueHandle`** instead of manually constructing a handle. Counter wrap-around (>10000 attempts) is a configuration error — propagate it.
- [ ] **Test every new code path** against a save fixture with different `slot.Version` (≤ 81 and > 81) — the code uses `len(GaItems)`, but the fixtures verify that you did not miss a hardcoded constant in the new path.

---

## Cross-references

- [03-gaitem-map](03-gaitem-map.md) — binary layout of `GaItem` (handles, item IDs, record sizes). A Phase 2 rewrite of this chapter is planned after approval of 35.
- [07-inventory](07-inventory.md) — Inventory data model (`Inventory.CommonItems`/`KeyItems`, the visibility gate, `NextEquipIndex`, `NextAcquisitionSortId`). Phase 2 rewrite planned.
- [10-storage](10-storage.md) — Storage data model. Phase 2 rewrite planned.
- [43-transactional-item-adding](43-transactional-item-adding.md) — full add-items architecture (PRE-FLIGHT → SNAPSHOT → MUTATE → RECONCILE → VALIDATE). Phase 2 rewrite planned.
- [52-acquisition-sort-stride2](52-acquisition-sort-stride2.md) — the stride-2 reorder algorithm. Not directly related to the allocator, but shares `slot.Inventory.NextAcquisitionSortId`.
- [53-inventory-storage-transfer](53-inventory-storage-transfer.md) — Inventory ↔ Storage transfer with the rehandle path (uses `allocateGaItem` inside `materializeRehandledInstance`).
- [54-ash-of-war](54-ash-of-war.md) — full AoW semantics, weapon `AoWGaItemHandle` sentinels, handle uniqueness invariant (I10).

---

## Sources

- `backend/core/structures.go` — `SaveSlot`, `scanGaItems`, counter reconstruction
- `backend/core/writer.go` — `allocateGaItem`, `generateUniqueHandle`, `AddItemsToSlot`, `AddItemsToSlotBatch`, `RepairOrphanedGaItems`
- `backend/core/capacity.go` — `SlotUsage`, `CountSlotUsage`, `CheckAddCapacity`
- `backend/core/diagnostics.go` — `ValidatePostMutation`, `IntegrityError`
- `backend/core/snapshot.go` — `SnapshotSlot`, `RestoreSlot`
- `backend/core/slot_rebuild.go` — `RebuildSlotFull`
- `backend/core/aow_availability.go` — `ScanAoWAvailability`, `AoWCopyRaw`
- `backend/core/offset_defs.go` — `GaItemCountOld`, `GaItemCountNew`, `GaItemVersionBreak`, `GaItemDataMaxCount`, `MaxHandleAttempts`, `CommonItemCount`, `StorageCommonCount`
- `app.go::AddItemsToCharacter`, `app_deploy.go::SlotCapacity` / `GetSlotCapacity`
- Tests: `backend/core/gaitem_placement_test.go`, `backend/core/aow_strict_test.go`, `tests/capacity_test.go`, `tests/bulk_add_test.go`
- Commit `6881cb9 fix(core): guard AoW allocation at armament capacity` (root cause + lock-in test)
- `tmp/docs-phase2-gaitem-inventory-plan.md` — Phase 2 plan (consolidation decisions)
- `tmp/docs-phase2-micro-research.md` — H5/H6 research (capacity threshold, transfer caps)
