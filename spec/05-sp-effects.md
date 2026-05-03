# 05 — SP Effects (Status Effects)

> **Type**: Binary format spec  
> **Scope**: Active effects on the character — buffs, debuffs, special statuses

---

## Description

The SP Effects (SpEffect) section follows directly after PlayerGameData. It contains a list of active status effects on the character at the moment of saving.

SpEffect in Elden Ring is a universal mechanism — it covers everything from antidotes through Great Rune bonuses to boss attack effects.

---

## Structure

### SPEffect Entry Format

| Offset | Type | Description |
|---|---|---|
| 0x00 | u32 | SpEffect ID (from SpEffectParam table) |
| 0x04 | f32 | Remaining duration (seconds, -1.0 = permanent) |
| 0x08 | u32 | Unknown field 1 |
| 0x0C | u32 | Unknown field 2 |

### Entry Count

The exact number of entries is given as a prefix or constant (requires verification):
- er-save-manager parses SPEffect as a structure with `param_id` + `remaining_time` + unknown fields
- ER-Save-Editor (Rust) does not parse this section in detail

---

## SpEffect ID Examples

SpEffect IDs refer to the `SpEffectParam` table in regulation.bin. Some known categories:

- **Great Runes**: activated great rune effects
- **Buffs**: Wondrous Physick mixes, consumables
- **Debuffs**: poison, rot, frostbite ticking damage
- **Passive**: equipment bonuses (some talismans)

---

## Editing Implications

- Removing all SpEffects is safe — effects will be reapplied on login
- Modifying duration allows permanent buffs (setting -1.0f)
- SpEffect IDs must exist in SpEffectParam — non-existent IDs may crash the game

---

## Sources

- er-save-manager: `parser/character.py` — class `SPEffect`
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=er-refmat:param:speffectparam
- TGA CE Table: https://github.com/The-Grand-Archives/Elden-Ring-CT-TGA — param scripts
