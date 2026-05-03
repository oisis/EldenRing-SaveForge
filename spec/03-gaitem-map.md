# 03 — GaItem Map (Item Map)

> **Type**: Binary format spec  
> **Scope**: Table mapping internal "handles" to item IDs. First large section after the slot header.

---

## Description

The GaItem Map is a table with a fixed number of entries (5118 or 5120 depending on slot version), where each entry describes one "slot" for an item in the game. A handle is a unique identifier for an item instance in the save.

The map is critical — inventory, equipment, and storage reference items by handle, not directly by item ID.

---

## Structure

### Entry Count
- `version <= 81`: 5118 entries (0x13FE)
- `version > 81`: 5120 entries (0x1400)

### Handle Types (upper nibble u32)

| Mask (upper 4 bits) | Type | Record Size |
|---|---|---|
| `0x80000000` | Weapon | 21 bytes |
| `0x90000000` | Armor | 16 bytes |
| `0xA0000000` | Accessory (Talisman) | 8 bytes |
| `0xB0000000` | Item (Good) | 8 bytes |
| `0xC0000000` | Ash of War | 8 bytes |
| `0xFFFFFFFF` | Invalid | — |
| `0x00000000` | Empty | — |

### Record Format — Weapon (21 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u32 | GaItem Handle |
| 0x04 | u32 | Item ID |
| 0x08 | u32 | Unknown 2 |
| 0x0C | u32 | Unknown 3 |
| 0x10 | u32 | Ash of War GaItem Handle |
| 0x14 | u8 | Unknown 5 |

### Record Format — Armor (16 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u32 | GaItem Handle |
| 0x04 | u32 | Item ID |
| 0x08 | u32 | Unknown 2 |
| 0x0C | u32 | Unknown 3 |

### Record Format — Item/Accessory/AoW (8 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u32 | GaItem Handle |
| 0x04 | u32 | Item ID |

---

## Relationship with Inventory

```
Inventory Item (12 bytes):
  ├── gaitem_handle → points to an entry in GaItem Map
  ├── quantity      → amount
  └── acq_index     → acquisition order

GaItem Map Entry:
  ├── handle        → same as in inventory
  └── item_id       → actual item ID in the game database
```

---

## Editing Implications

- Adding a new item requires: finding a free slot in GaItem Map + adding an entry in Inventory
- Changing a weapon requires a 21-byte record
- Handle type (upper nibble) MUST match the item type
- Unused slots have handle `0x00000000` or `0xFFFFFFFF`

---

## Sources

- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — GaItem2 struct (lines 156-194)
- er-save-manager: `parser/er_types.py` — Gaitem class
- er-save-manager: `parser/user_data_x.py` — `gaitem_map` field (line 82)
