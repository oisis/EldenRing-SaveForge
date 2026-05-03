# 43 — Transactional Item Adding (Crash Prevention)

> **Type**: Design doc
> **Extracted from**: ROADMAP.md (2026-05-03 cleanup)
> **Status**: ✅ Implemented (v0.7.2)

---

## Problem

`AddItemsToCharacter` modyfikuje slot bez walidacji capacity i bez rollbacku. Partial failure (pełny inventory, pełna tablica GaItems, pełny GaItemData) zostawia slot w niespójnym stanie: orphaned GaItems, handle bez inventory entry, uszkodzony counter. Gra crashuje przy ładowaniu (`EXCEPTION_ACCESS_VIOLATION`).

## Design principle

**ALL-OR-NOTHING** — albo wszystkie żądane itemy zostają dodane, albo żaden. Partial write = corrupted save = niedopuszczalny.

---

## Architecture

```
     ┌─────────────────────────────────────────────────────────┐
     │               AddItemsToCharacter (app.go)              │
     │                                                         │
     │  1. PRE-COMPUTE: finalIDs, quantities, container caps   │
     │  2. PRE-FLIGHT: CheckSlotCapacity() — all fit?          │
     │     └─ NO  → return AddResult{CapHit, 0 added}         │
     │     └─ YES → continue                                   │
     │  3. SNAPSHOT: deep copy slot state                       │
     │  4. MUTATE: AddItemsToSlotBatch() — one rebuild         │
     │  5. POST-FLAGS: event flags, tutorials, containers      │
     │  6. VALIDATE: ValidateSlotIntegrity() — invariants OK?  │
     │     └─ NO  → ROLLBACK to snapshot, return error         │
     │     └─ YES → commit, return AddResult{success}          │
     └─────────────────────────────────────────────────────────┘
```

---

## Implementation steps

### Step 1 — Fix `upsertGaItemData` silent overflow 🔴

When `count >= GaItemDataMaxCount (7000)`, was returning `nil` instead of error. GaItem gets created but never registered → orphaned metadata → game crash.

**Fix:** Return `fmt.Errorf(...)` instead of `nil`.

### Step 2 — Pre-flight capacity check 🔴

`CheckAddCapacity(slot, items []ItemToAdd) (canFitAll bool, details CapacityReport)`

Counts: free inventory CommonItems (2688 - used), storage CommonItems (1920 - used), GaItems (5120 - used), GaItemData (7000 - count).

### Step 3 — `AddResult` return type + all-or-nothing semantics 🔴

```go
type AddResult struct {
    Added     int          `json:"added"`
    Requested int          `json:"requested"`
    Skipped   []SkippedAdd `json:"skipped"`
    CapHit    string       `json:"capHit"`
    FreeInv   int          `json:"freeInv"`
    FreeStore int          `json:"freeStore"`
}
```

### Step 4 — Snapshot + rollback 🔴

```go
snapshot := SnapshotSlot(slot)   // deep copy: Data, GaItems, GaMap, Inventory, Storage, all indices
// ... mutation ...
// on error:
RestoreSlot(slot, snapshot)      // restore all fields
```

Separate from `pushUndo()` — internal safety mechanism, doesn't touch undo stack.

### Step 5 — Batch rebuild (`AddItemsToSlotBatch`) 🟡

```go
type ItemToAdd struct {
    ItemID         uint32
    InvQty         int
    StorageQty     int
    ForceStackable bool
}

func AddItemsToSlotBatch(slot *SaveSlot, items []ItemToAdd) error
```

One `RebuildSlotFull` instead of N. 50 weapons: ~100ms (batch) vs ~2.5-5s (per-item).

### Step 6 — Post-write validation (`ValidatePostMutation`) 🔴

Fast invariant check after every mutation:
1. Every non-empty inventory handle exists in GaMap
2. Every non-stackable GaMap entry has a GaItem record
3. No duplicate Index values
4. NextEquipIndex > max(all indices)
5. GaItemData count matches actual entries
6. Storage count header correct
7. NextAoWIndex <= NextArmamentIndex <= len(GaItems)
8. No handle references itemID=0

Performance target: <10ms.

### Step 7 — Event flag error classification 🟡

Replace `_ = db.SetEventFlag(...)` with logged warnings. Non-critical (AoW duplication, world pickup, tutorial, container) → log only.

### Step 8 — Frontend `AddResult` handling 🟡

- Capacity failure → error toast
- Container trims → console log
- Success → toast with count
- Modal always closes, refresh always fires

### Step 9 — Storage count header reconciliation 🟡

After batch add, reconcile storage count header with actual non-empty count.

### Step 10 — Tests 🔴

16 tests in `tests/capacity_test.go`:
- PreFlight (empty/near-full/full/mixed-stackable)
- AllOrNothing (capacity exceeded / mid-add error → rollback)
- BatchRebuild (single vs multiple / performance <200ms)
- PostValidation (orphan handle / duplicate index / counter mismatch)
- StorageHeaderReconcile
- GaItemDataFull error
- Roundtrip (full inventory / batch 300 items)
- AddResult container cap trim

---

## Implementation order

| Order | Step | Priority | Est. time |
|-------|------|----------|-----------|
| 1 | upsertGaItemData fix | 🔴 | 15 min |
| 2 | Snapshot/rollback | 🔴 | 1-2h |
| 3 | Pre-flight capacity | 🔴 | 2-3h |
| 4 | Post-write validation | 🔴 | 2-3h |
| 5 | AddResult type | 🔴 | 1-2h |
| 6 | Event flag logging | 🟡 | 30 min |
| 7 | Storage header reconcile | 🟡 | 1h |
| 8 | Batch rebuild | 🟡 | 3-4h |
| 9 | Frontend AddResult | 🟡 | 1-2h |
| 10 | Full test suite | 🔴 | 3-4h |

**Total:** 15-22h
