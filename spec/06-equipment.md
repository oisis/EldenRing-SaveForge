# 06 — Equipment

> **Type**: Binary format spec  
> **Scope**: Currently equipped items — weapons, armor, talismans, arrows, bolts, quick items, pouch, Great Rune.

---

## Overview

Equipment is described by 4 related structures, each 88 bytes (22 × u32), plus one 28-byte structure for active weapon slots.

Each of the 3 main structures (EquipIndex, ItemIDs, GaitemHandles) has an identical 22-slot layout — but stores different values for the same slots.

---

## 22 equipment slots (88 bytes = 22 × u32)

| # | Offset | Slot | Description |
|---|---|---|---|
| 0 | 0x00 | Left Hand Armament 1 | Left hand weapon — primary |
| 1 | 0x04 | Right Hand Armament 1 | Right hand weapon — primary |
| 2 | 0x08 | Left Hand Armament 2 | Left hand — secondary |
| 3 | 0x0C | Right Hand Armament 2 | Right hand — secondary |
| 4 | 0x10 | Left Hand Armament 3 | Left hand — tertiary |
| 5 | 0x14 | Right Hand Armament 3 | Right hand — tertiary |
| 6 | 0x18 | Arrows 1 | Arrows (primary) |
| 7 | 0x1C | Bolts 1 | Bolts (primary) |
| 8 | 0x20 | Arrows 2 | Arrows (secondary) |
| 9 | 0x24 | Bolts 2 | Bolts (secondary) |
| 10 | 0x28 | Arrows 3 | Arrows (tertiary) — confirmed CT |
| 11 | 0x2C | Bolts 3 | Bolts (tertiary) — confirmed CT |
| 12 | 0x30 | Head | Helm |
| 13 | 0x34 | Chest | Chest armor |
| 14 | 0x38 | Arms | Gauntlets |
| 15 | 0x3C | Legs | Greaves/pants |
| 16 | 0x40 | Hair | Hairstyle (equipment slot — linked to Face Data) |
| 17 | 0x44 | Talisman 1 | Talisman slot 1 |
| 18 | 0x48 | Talisman 2 | Talisman slot 2 |
| 19 | 0x4C | Talisman 3 | Talisman slot 3 (requires quest unlock) |
| 20 | 0x50 | Talisman 4 | Talisman slot 4 (requires quest unlock) |
| 21 | 0x54 | Accessory 5 | Unused in base game (reserved) |

---

## Structure 1: EquippedItemsEquipIndex (88 bytes)

Indices into the inventory array. The value in each slot is the `acquisition_index` from the corresponding Inventory entry.

Value `0xFFFFFFFF` = empty slot (nothing equipped).

---

## Structure 2: ActiveWeaponSlotsAndArmStyle (28 bytes = 7 × u32)

| Offset | Type | Field | Description | Values |
|---|---|---|---|---|
| 0x00 | u32 | ArmStyle | Weapon holding style | 0=EmptyHand, 1=OneHand, 2=LeftBothHand (2H left), 3=RightBothHand (2H right) |
| 0x04 | u32 | LeftWeaponSlot | Active left weapon slot | 0=Primary, 1=Secondary, 2=Tertiary |
| 0x08 | u32 | RightWeaponSlot | Active right weapon slot | 0=Primary, 1=Secondary, 2=Tertiary |
| 0x0C | u32 | LeftArrowSlot | Active arrow slot (left) | 0=Primary, 1=Secondary |
| 0x10 | u32 | RightArrowSlot | Active arrow slot (right) | 0=Primary, 1=Secondary |
| 0x14 | u32 | LeftBoltSlot | Active bolt slot (left) | 0=Primary, 1=Secondary |
| 0x18 | u32 | RightBoltSlot | Active bolt slot (right) | 0=Primary, 1=Secondary |

---

## Structure 3: EquippedItemsItemIds (88 bytes)

Item ID of each equipped item. Corresponds to the ID from the game database.

| Prefix | Type | Example |
|---|---|---|
| 0xxxxxxx – 9xxxxxxx | Weapon | 1000000 = Uchigatana +0 |
| 10xxxxxx – 19xxxxxx | Armor (Protector) | 10100000 = Banished Knight Helm |
| 20xxxxxx – 29xxxxxx | Accessory (Talisman) | 20001000 = Crimson Amber Medallion |
| 40xxxxxx – 49xxxxxx | Goods (Consumable) | 40001001 = Mushroom |
| 50xxxxxx – 59xxxxxx | Gem (Ash of War) | |

Value `0xFFFFFFFF` = empty slot.

---

## Structure 4: EquippedItemsGaitemHandles (88 bytes)

GaItem Handle for each equipped item — points to the specific instance in GaItem Map.

Value `0xFFFFFFFF` = empty slot.

---

## Great Rune (equipped)

Great Rune is a separate field in equipment (not in the 22-slot array):

| Type | Field | Description |
|---|---|---|
| u32 | EquippedGreatRune | ID of equipped Great Rune |

### Great Rune Item IDs:

| Hex ID | Decimal | Great Rune | Effect (active with Rune Arc) |
|---|---|---|---|
| 0x00000000 | 0 | None | — |
| 0xB00000BF | 2952790207 | Godrick's Great Rune | +5 to all attributes |
| 0xB00000C0 | 2952790208 | Radahn's Great Rune | +HP, +FP, +SP (max) |
| 0xB00000C1 | 2952790209 | Morgott's Great Rune | +Max HP (significant) |
| 0xB00000C2 | 2952790210 | Rykard's Great Rune | HP recovery on kill |
| 0xB00000C3 | 2952790211 | Mohg's Great Rune | Phantom bleed effect on summons |
| 0xB00000C4 | 2952790212 | Malenia's Great Rune | HP recovery on attack (after taking damage) |

**Note**: Great Rune requires Rune Arc activation (PlayerGameData offset 0xF7 = GreatRuneActive). Possession of Great Rune is controlled by Event Flags (180–197).

---

## Relationship between structures

```
Slot "Right Hand 1" (index 1):
  EquipIndex[1]     = 42          (42nd element in inventory — acquisition_index)
  ItemIds[1]        = 1000000     (Uchigatana +0)
  GaitemHandles[1]  = 0x80000003  (handle in GaItem Map → weapon entry)
  
ActiveSlots:
  ArmStyle          = 1           (OneHand)
  RightWeaponSlot   = 0           (active primary = slot 1)
```

---

## Quick Items & Pouch (described in spec/08)

Quick Items (10 slots) and Pouch (6 slots) are described in **spec/08-spells-gestures.md** because they sequentially follow Equipped Spells. Brief summary here:

- Quick Slots 1–10: 10 × u32 Item ID (cycling through D-pad)
- Pouch 1–6: 6 × u32 Item ID (hold Y/Triangle + direction)
- Value `0xFFFFFFFF` = empty slot
- Items must exist in inventory — these are references, not copies

---

## Hair Slot (slot #16)

The "Hair" equipment slot is linked to Face Data (Hair_Model_Id). Changing hairstyle in the creator updates both Face Data and the Hair equipment slot. This slot is internal — not visible in the player's equipment UI.

---

## Editing implications

- **Changing weapons** requires updating ALL 3 structures (equip index, item id, handle)
- **Handle** must exist in GaItem Map and have the correct type (0x8... for weapons)
- **Active slot values**: 0, 1, or 2 (corresponding to slots 1/2/3)
- **ArmStyle** affects animations — invalid value may crash
- **Great Rune**: changing also requires the corresponding Event Flag (possession) and GreatRuneActive
- **Talisman 3&4**: require quest unlock (Talisman Pouch items) — slot is physically present but game will lock it
- **Accessory 5**: unused — inserting a value may be unstable
- **Item ID encoding**: Weapon +X = baseID + X (e.g. Uchigatana +5 = 1000005)

---

## Sources

- er-save-manager: `parser/equipment.py` — classes `EquipmentSlots`, `ActiveWeaponSlotsAndArmStyle` (lines 20-163)
- ER-Save-Editor (Rust): `src/save/common/save_slot.rs` — referenced structures
- Cheat Engine: `ER_all-in-one_Hexinton_v3.10` — ChrAsm offsets (ArmStyle, weapon slots, all equipment IDs)
- Cheat Engine: `ER_TGA_v1.9.0` — ChrAsm 2 (equipment item IDs 0x5D8-0x62C), Quick Items, Pouch, Great Rune
