# 07 — Inventory

> **Type**: Binary format spec  
> **Scope**: Items carried by the character. Divided into common items and key items.

---

## Overview

The character inventory (Inventory Held) is a fixed-size slot array, divided into two sections:
- **Common Items**: weapons, armor, talismans, materials, consumables — 0xA80 (2688) slots
- **Key Items**: story-critical items — 0x180 (384) slots

Each slot is 12 bytes. Empty slots have `gaitem_handle == 0` or `gaitem_handle == 0xFFFFFFFF`.

---

## Record format (12 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u32 | GaItem Handle (pointer to GaItem Map) |
| 0x04 | u32 | Quantity |
| 0x08 | u32 | Acquisition Index (order of acquisition, used by equip system) |

---

## Inventory Held section layout

```
┌────────────────────────────────────────┐
│ Common Items: 2688 × 12 = 32,256 bytes │  (0x7E00)
├────────────────────────────────────────┤
│ Key Items: 384 × 12 = 4,608 bytes      │  (0x1200)
├────────────────────────────────────────┤
│ Next Equip Index (u32)                  │  4 bytes
├────────────────────────────────────────┤
│ Next Acquisition Sort ID (u32)          │  4 bytes
└────────────────────────────────────────┘
Total: 0x7E00 + 0x1200 + 8 = 36,872 bytes (0x9008)
```

---

## Counters (trailing)

After the item arrays:

| Field | Type | Description |
|---|---|---|
| Next Equip Index | u32 | Next free equip index (incremented on pickup) |
| Next Acquisition Sort ID | u32 | Next sort ID (display order) |

These counters MUST be updated when adding new items.

---

## Relationship with GaItem Map

```
Inventory Entry:
  gaitem_handle = 0x80000005
  quantity = 1
  acq_index = 15

    ↓ lookup in GaItem Map

GaItem Map Entry [5]:
  handle = 0x80000005
  item_id = 3010000        → Moonveil +0 (weapon)
  unk2, unk3, aow_handle, unk5   (weapon fields)
```

---

## Item types (from item_id)

Item ID has a prefix determining the category:

| ID Prefix | Category |
|---|---|
| 0xxxxxxx - 9xxxxxxx | Weapons (handle 0x8...) |
| 10xxxxxx - 19xxxxxx | Armor (handle 0x9...) |
| 20xxxxxx - 29xxxxxx | Accessories/Talismans (handle 0xA...) |
| 30xxxxxx - 49xxxxxx | Items/Goods (handle 0xB...) |
| (Ash of War) | (handle 0xC...) |

---

## Editing implications

- Adding an item: find a free slot (handle==0), write handle+qty+index, update counter
- New handle must have a corresponding entry in GaItem Map
- `acq_index` must be unique within inventory — use Next Equip Index and increment
- Max quantity depends on item type (weapons=1, materials=999, etc.)
- Key Items are a separate section — don't mix with common

---

## Sources

- er-save-manager: `parser/equipment.py` — class `InventoryItem` (line 191+), `Inventory`
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — inventory structures
- er-save-manager: `parser/user_data_x.py` line 104-105: `inventory_held: Inventory` (0xa80 common, 0x180 key)
