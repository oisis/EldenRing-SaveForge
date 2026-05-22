# 05 — SP Effects (Status Effects)

> **Scope**: Active effects on the character — buffs, debuffs, special statuses.

---

## Overview

The SP Effects (SpEffect) section follows immediately after PlayerGameData. It contains the list of status effects active on the character at the moment of saving.

In Elden Ring, SpEffect is a universal mechanism — it covers everything from poisons through Great Rune bonuses to boss-attack effects.

---

## Structure

### SPEffect entry format

| Offset | Type | Description |
|---|---|---|
| 0x00 | u32 | SpEffect ID (from the SpEffectParam table) |
| 0x04 | f32 | Remaining duration (seconds, -1.0 = permanent) |
| 0x08 | u32 | Unknown field 1 |
| 0x0C | u32 | Unknown field 2 |

### Entry count

The exact entry count is provided as a prefix or fixed value (needs verification):
- er-save-manager parses SPEffect as a structure with `param_id` + `remaining_time` + unknown fields
- ER-Save-Editor (Rust) does not parse this section in detail

---

## Example SpEffect IDs

SpEffect IDs reference the `SpEffectParam` table in regulation.bin. A few known categories:

- **Great Runes**: activated Great Rune effects
- **Buffs**: Wonderous Physick mixtures, consumables
- **Debuffs**: poison, rot, frostbite ticking damage
- **Passive**: equipment bonuses (some talismans)

---

## Editing implications

- Removing all SpEffects is safe — effects will be reapplied on login
- Modifying duration allows permanent buffs (set to -1.0f)
- SpEffect IDs must exist in SpEffectParam — non-existent IDs can crash the game

---

## Sources

- er-save-manager: `parser/character.py` — class `SPEffect`
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=er-refmat:param:speffectparam
- TGA CE Table: https://github.com/The-Grand-Archives/Elden-Ring-CT-TGA — param scripts
