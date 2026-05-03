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

```
┌────────────────────────────────────────┐
│ Common Items: 1920 × 12 = 23,040 bytes │  (0x5A00)
├────────────────────────────────────────┤
│ Key Items: 128 × 12 = 1,536 bytes      │  (0x600)
├────────────────────────────────────────┤
│ Next Equip Index (u32)                  │  4 bytes
├────────────────────────────────────────┤
│ Next Acquisition Sort ID (u32)          │  4 bytes
└────────────────────────────────────────┘
Total: 0x5A00 + 0x600 + 8 = 24,584 bytes (0x6008)
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

---

## Editing implications

- Moving an item between Inventory↔Storage: move the record, update both sets of counters
- Storage shares the GaItem Map — the same handle must exist in the map
- Capacity: 1920 common + 128 key = max storage capacity

---

## Sources

- er-save-manager: `parser/user_data_x.py` line 123: `inventory_storage_box: Inventory` (0x780 common, 0x80 key)
- er-save-manager: `parser/equipment.py` — same `Inventory` class with different count parameters
