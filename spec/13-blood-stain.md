# 13 — Blood Stain

> **Scope**: Data of the blood stain left after the player's death — location and lost runes.

---

## Overview

The Blood Stain stores information about the player's last death: where they died and how many runes they lost. The player can recover the runes by picking up the blood stain. Size: 68 bytes (0x44).

---

## Structure (68 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | f32 × 3 | Coordinates (x, y, z) — position of the stain |
| 0x0C | f32 × 4 | Angle / Quaternion (orientation) |
| 0x1C | u32 | Unknown (unk0x1c) |
| 0x20 | u32 | Unknown (unk0x20) |
| 0x24 | u32 | Unknown (unk0x24) |
| 0x28 | u32 | Unknown (unk0x28) |
| 0x2C | u32 | Unknown (unk0x2c) |
| 0x30 | i32 | Unknown (unk0x30) |
| 0x34 | i32 | Runes (amount of runes to recover) |
| 0x38 | u8[4] | Map ID (map on which the stain is) |
| 0x3C | u32 | Unknown (unk0x3c) |
| 0x40 | u32 | Unknown (unk0x38) |

---

## Editing implications

- Modifying `Runes` lets you change how many runes the player will recover
- Setting the coordinates lets you "move" the stain to an accessible place
- Zeroing the entire structure = no blood stain (the player has nothing to recover)
- Useful for corrupted saves where the stain is in an inaccessible place

---

## Sources

- er-save-manager: `parser/world.py` — the `BloodStain` class (lines 182-229)
- er-save-manager: `parser/user_data_x.py` line 136: `blood_stain: BloodStain`
