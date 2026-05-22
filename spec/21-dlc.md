# 21 — DLC (Downloadable Content)

> **Scope**: DLC flags — pre-order gestures and Shadow of the Erdtree entry.

---

## Overview

The DLC section is 50 bytes (0x32) containing ownership and entry flags for the DLC. Structure: CSDlc — an array of 1-byte bools.

---

## Structure (50 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u8 | Pre-order Gesture "The Ring" (0=no, 1=yes) |
| 0x01 | u8 | Shadow of the Erdtree — entry flag (0=not entered, 1=entered) |
| 0x02 | u8 | Pre-order Gesture "Ring of Miquella" (0=no, 1=yes) |
| 0x03 | u8[47] | Unused (MUST be 0x00) |

---

## Shadow of the Erdtree Entry Flag

- `0`: Character has not entered the DLC
- `1`: Character has entered Shadow of the Erdtree

This flag is one-way — once entered there is no way to undo it in-game. Editing allows a reset.

---

## Validation — unused bytes

**IMPORTANT**: Bytes 3-49 (47 bytes) MUST be zero. Non-zero values in this section **prevent the save from loading** — the game rejects the file.

---

## Editing implications

- **Clear DLC flag**: setting byte[1]=0 lets you "undo" entering the DLC
- **Pre-order gestures**: setting byte[0]=1 or byte[2]=1 unlocks the gestures
- **CRITICAL**: never set non-zero values in bytes 3-49
- Safe to edit — fixed position in the slot (SlotSize - 0xB2 from the end)

---

## Sources

- er-save-manager: `parser/world.py` — `DLC` class (lines 938-987)
- er-save-manager: `parser/user_data_x.py` line 194: `dlc: DLC`
