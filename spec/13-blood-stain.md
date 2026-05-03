# 13 — Blood Stain

> **Type**: Binary format spec  
> **Scope**: Blood stain data left after player death — location and lost runes.

---

## Overview

Blood Stain stores information about the player's last death: where they died and how many runes were lost. The player can recover runes by picking up the blood stain. Size: 68 bytes (0x44).

---

## Structure (68 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | f32 × 3 | Coordinates (x, y, z) — stain position |
| 0x0C | f32 × 4 | Angle / Quaternion (orientation) |
| 0x1C | u32 | Unknown (unk0x1c) |
| 0x20 | u32 | Unknown (unk0x20) |
| 0x24 | u32 | Unknown (unk0x24) |
| 0x28 | u32 | Unknown (unk0x28) |
| 0x2C | u32 | Unknown (unk0x2c) |
| 0x30 | i32 | Unknown (unk0x30) |
| 0x34 | i32 | Runes (amount of recoverable runes) |
| 0x38 | u8[4] | Map ID (map where the stain is located) |
| 0x3C | u32 | Unknown (unk0x3c) |
| 0x40 | u32 | Unknown (unk0x38) |

---

## Editing implications

- Modifying `Runes` allows changing how many runes the player recovers
- Setting coordinates allows "moving" the stain to an accessible location
- Zeroing the entire structure = no blood stain (player has nothing to recover)
- Useful for corrupted saves where the stain is in an inaccessible location

---

## Sources

- er-save-manager: `parser/world.py` — class `BloodStain` (lines 182-229)
- er-save-manager: `parser/user_data_x.py` line 136: `blood_stain: BloodStain`
