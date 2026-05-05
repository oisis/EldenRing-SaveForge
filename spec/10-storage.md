# 10 — Storage Box

> **Type**: Binary format spec  
> **Scope**: Storage box accessible at Site of Grace — item storage outside inventory.

---

## Overview

Storage Box (Inventory Storage) has an identical format to Inventory Held, but with different sizes:
- **Common Items**: 0x780 (1920) slots
- **Key Items**: 0x80 (128) slots

Each slot is 12 bytes (identical format to inventory).

---

## Layout

The section is preceded by a 4-byte `storage_count` header immediately before the first common item
slot (at `StorageBoxOffset`):

```
┌────────────────────────────────────────┐
│ storage_count (u32)                     │  4 bytes  ← at StorageBoxOffset
├────────────────────────────────────────┤
│ Common Items: 1920 × 12 = 23,040 bytes │  (0x5A00)
├────────────────────────────────────────┤
│ Key Items: 128 × 12 = 1,536 bytes      │  (0x600)
├────────────────────────────────────────┤
│ Next Equip Index (u32)                  │  4 bytes
├────────────────────────────────────────┤
│ Next Acquisition Sort ID (u32)          │  4 bytes
└────────────────────────────────────────┘
Total (excl. header): 0x5A00 + 0x600 + 8 = 24,584 bytes (0x6008)
```

---

## Record format (12 bytes) — identical to Inventory

| Offset | Type | Description |
|---|---|---|
| 0x00 | u32 | GaItem Handle |
| 0x04 | u32 | Quantity |
| 0x08 | u32 | Acquisition Index |

---

## Counters

Storage has its own independent Next Equip Index and Next Acquisition Sort ID counters.
The same visibility gate rule applies as for Inventory Held (see spec/07):

**Invariant**: `item.AcquisitionIndex < NextEquipIndex` for every non-empty storage slot.
Items violating this invariant are invisible to the game even though they exist in binary.

External editors may leave `NextEquipIndex < NextAcquisitionSortId` in storage as well. Our editor
reconciles the gap on load in `mapStorage()` using the same binary write-back strategy as inventory.

---

## Editing implications

- Moving an item between Inventory↔Storage: move the record, update both sets of counters
- Storage shares the GaItem Map — the same handle must exist in the map
- Capacity: 1920 common + 128 key = max storage capacity
- Update `storage_count` header when adding or removing items

---

## Sources

- er-save-manager: `parser/user_data_x.py` line 123: `inventory_storage_box: Inventory` (0x780 common, 0x80 key)
- er-save-manager: `parser/equipment.py` — same `Inventory` class with different count parameters
- `backend/core/structures.go`: `StorageBoxOffset`, `mapStorage()`
