# 22 — Player Data Hash

> **Scope**: Final hash of the player's data — the last section in the slot.

---

## Overview

PlayerGameDataHash is the last section in the slot. It occupies the remaining bytes until the end of the slot (0x280000). Contains a hash or checksum of the player's data.

---

## Structure

| Offset | Type | Description |
|---|---|---|
| 0x00 | u8[...] | Hash / checksum data (to the end of the slot) |

Exact length: `slot_end - current_position` after parsing all the previous sections.

---

## Known properties

- The game **does NOT validate** this hash on load (confirmed by ER-Save-Editor)
- It is read-only from an editor's perspective — there is no need to recompute it
- The contents are probably FromSoftware's internal tamper-detection hash (but not enforced)

---

## Editing implications

- **Does not require recomputation** — the game ignores it
- Safe to leave unchanged after editing the slot
- When creating a slot from scratch: fill with zeros

---

## Sources

- er-save-manager: `parser/user_data_x.py` line 195: `player_data_hash: PlayerGameDataHash`
- er-save-manager: `parser/world.py` — `PlayerGameDataHash` class
