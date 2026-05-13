# 52 — Acquisition Sort: Stride-2 Index Assignment

> **Type**: Design doc
> **Status**: ✅ Implemented
> **Scope**: Explains why ReorderInventory uses stride-2 index assignment (`base + pos*2`) instead of stride-1 (`base + pos`), and documents the in-game sort key discovery.

---

## Background

The game sorts items in "Acquisition Order" view using the `acquisition_index` field at `offset+8` of each 12-byte `InventoryItem` record (`handle u32 | qty u32 | acqIdx u32`). The initial plan (spec/39 Phase 1) assumed stride-1 assignment was sufficient: give position `i` the index `base + i`. This was incorrect.

---

## Discovery: Game Sorts by `acqIdx >> 1`

### Sentinel test methodology

Three sentinel tests were run against a real save deployed to Steam Deck, using the talismans tab (65 items):

| Test | Assignment | Result |
|---|---|---|
| Sentinel v1 (hardcoded 10000–10012) | `10000, 10001, ..., 10012` | **Game crash** at save load |
| Sentinel v2 (safe, stride-1) | `minAcq, minAcq+1, ..., minAcq+N` | Game loaded, **adjacent pairs swapped** |
| Sentinel v3 (stride-2) | `base, base+2, ..., base+N*2` (even base) | Game loaded, **order correct** |

### Root cause

The game buckets items for the "Acquisition Order" sort by `acqIdx >> 1` (right-shift 1 bit), not by the full 32-bit value. Two adjacent stride-1 indices `k` and `k+1` map to the same bucket when `k` is even: `k>>1 == (k+1)>>1`. Within a shared bucket, the game applies a secondary sort key (likely `sortGroupId` or handle), overriding the intended order.

Stride-2 with an even base avoids this entirely: `(base + 2*i) >> 1 = base/2 + i`, which is strictly increasing — unique bucket per item.

---

## Algorithm

```go
// base: start from NextAcquisitionSortId; ensure even and > InvEquipReservedMax (432).
base := slot.Inventory.NextAcquisitionSortId
if base <= uint32(core.InvEquipReservedMax) {
    base = uint32(core.InvEquipReservedMax) + 2  // 434 — minimum safe even value
}
if base%2 != 0 {
    base++
}

// Assign: item at position pos gets index base + pos*2.
for pos, handle := range orderedHandles {
    newIdx := base + uint32(pos)*2
    // write to slot.Data[off+8:] and slot.Inventory.CommonItems[j].Index
}

// Advance NextAcquisitionSortId (advance-only).
expectedMax := base + uint32(len(orderedHandles)-1)*2
newNextAcq := expectedMax + 1
```

### Bucket uniqueness proof

For even `base` and position `i`:

```
bucket(i) = (base + 2*i) >> 1
           = base/2 + i
```

Since `base` is even, `base/2` is an integer. `base/2 + i` is strictly increasing in `i` → no two positions share a bucket.

---

## Safe Value Range

Values ≤ 432 are reserved for equipment slots (`InvEquipReservedMax = 432`). Values `>= 10000` observed to crash the game at load (sentinel v1 finding). Real character indices are typically in the range 500–2000 depending on playtime. The stride-2 algorithm stays in this safe range by using `NextAcquisitionSortId` as the base.

---

## Defensive Guard

`ReorderInventory` includes a compile-time-equivalent collision check before any mutation:

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

This guard is unreachable with a correct even base and stride-2 spacing, but prevents silent regressions if the base/stride logic is ever changed.

---

## Implementation Location

- `app_inventory_order.go` — `ReorderInventory` function
- Applies to all tabs: `weapons`, `talismans`, `head`, `chest`, `arms`, `legs`

---

## Sources

- spec/39: original design doc for Inventory Reorder feature
- Empirical in-game testing on Steam Deck (real PS4 save deployed via SSH)
