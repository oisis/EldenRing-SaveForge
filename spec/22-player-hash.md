# 22 — Player Data Hash

> **Type**: Binary format spec  
> **Scope**: Final player data hash — last section in the slot.

---

## Overview

PlayerGameDataHash is the last section in the slot. It occupies the remaining bytes until the end of the slot (0x280000). Contains a hash or checksum of player data.

---

## Structure

| Offset | Type | Description |
|---|---|---|
| 0x00 | u8[...] | Hash / checksum data (until end of slot) |

Exact length: `slot_end - current_position` after parsing all preceding sections.

---

## Known properties

- The game **does NOT validate** this hash on load (confirmed by ER-Save-Editor)
- It is read-only from the editor's perspective — no recalculation needed
- Content is likely an internal FromSoftware hash for tampering detection (but not enforced)

---

## Editing implications

- **No recalculation required** — the game ignores it
- Safe to leave unchanged after slot editing
- When creating a slot from scratch: fill with zeros

---

## Sources

- er-save-manager: `parser/user_data_x.py` line 195: `player_data_hash: PlayerGameDataHash`
- er-save-manager: `parser/world.py` — class `PlayerGameDataHash`
