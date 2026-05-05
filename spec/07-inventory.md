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

The section is preceded by a 4-byte count header (`common_item_count`) immediately before the first
common item slot (`MagicOffset + InvStartFromMagic - 4`):

```
┌────────────────────────────────────────┐
│ common_item_count (u32)                 │  4 bytes  ← at MagicOffset + 0x1F9
├────────────────────────────────────────┤
│ Common Items: 2688 × 12 = 32,256 bytes │  (0x7E00)
├────────────────────────────────────────┤
│ Key Items: 384 × 12 = 4,608 bytes      │  (0x1200)
├────────────────────────────────────────┤
│ Next Equip Index (u32)                  │  4 bytes
├────────────────────────────────────────┤
│ Next Acquisition Sort ID (u32)          │  4 bytes
└────────────────────────────────────────┘
Total (excl. header): 0x7E00 + 0x1200 + 8 = 36,872 bytes (0x9008)
```

---

## Counters (trailing)

After the item arrays:

| Field | Type | Description |
|---|---|---|
| Next Equip Index | u32 | Visibility gate — items with `Acquisition Index >= NextEquipIndex` are invisible to the game |
| Next Acquisition Sort ID | u32 | Next sort ID assigned to the next added item |

### Visibility gate (critical)

The game treats `item.AcquisitionIndex >= NextEquipIndex` as **invisible** — the item exists in binary
but the equip system ignores it entirely. This is the mechanism behind "item added but not visible
in-game" bugs.

**Invariant**: after any add, `NextEquipIndex` MUST be strictly greater than every item's
`AcquisitionIndex`. The simplest correct update:

```
item.AcquisitionIndex = NextAcquisitionSortId   // assign before increment
NextAcquisitionSortId++
if item.AcquisitionIndex >= NextEquipIndex {
    NextEquipIndex = item.AcquisitionIndex + 1
} else {
    NextEquipIndex++
}
```

External editors (e.g. `er-save-manager`) sometimes increment `NextAcquisitionSortId` without
advancing `NextEquipIndex`, creating a gap. Items added by such editors with high `AcquisitionIndex`
stay permanently invisible until the gap is closed.

**On load**: our editor detects `NextEquipIndex < NextAcquisitionSortId` in `mapInventory()` and
corrects both the in-memory value and the binary bytes at `nextEquipIndexOff` immediately, so
re-saving without further edits is sufficient to fix the visibility gate for the game.

### `common_item_count` header

The 4-byte count at `invStart - 4` is read by the game runtime as the "next free insertion index"
for in-game pickups and the threshold for "inventory full" (`count == 2688`). It must equal the
actual number of non-empty common item slots.

- Stale count (too low after adds by external tools): game inserts at wrong position, potentially
  overwriting valid items.
- Stale count (too high after removes): game reports "inventory full" prematurely.

**On load**: `ReconcileInventoryHeader()` recomputes and writes the correct count to binary.

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

## Orphaned GaItem records

A GaItem record whose handle does not appear in any inventory or storage slot is **orphaned**.
Orphaned records occupy GaItem capacity (5118–5120 entries total) without being player-accessible.
They accumulate when items are removed from inventory/storage without clearing the GaItem binary
entry. Use `RepairOrphanedGaItems(slot)` to scan and clear them.

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

- Adding an item: find a free slot (handle==0), write handle+qty+index, update all three counters
  (`NextEquipIndex`, `NextAcquisitionSortId`, `common_item_count`)
- Removing an item: zero the slot, decrement `common_item_count`, clear the GaItem binary entry
- New handle must have a corresponding entry in GaItem Map
- `AcquisitionIndex` must satisfy `< NextEquipIndex` immediately after the add
- Max quantity depends on item type (weapons=1, materials=999, etc.)
- Key Items are a separate section — don't mix with common

---

## Sources

- er-save-manager: `parser/equipment.py` — class `InventoryItem` (line 191+), `Inventory`
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — inventory structures
- er-save-manager: `parser/user_data_x.py` line 104-105: `inventory_held: Inventory` (0xa80 common, 0x180 key)
- `backend/core/offset_defs.go`: `InvStartFromMagic = 505`
- `backend/core/structures.go`: `mapInventory()`, `ReconcileInventoryHeader()`
- `backend/core/writer.go`: `addToInventory()`, `RepairOrphanedGaItems()`
