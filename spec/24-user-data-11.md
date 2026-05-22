# 24 — UserData11 (regulation.bin)

> **Scope**: Regulation data — game parameters (params), the last section of the file.

---

## Overview

UserData11 contains `regulation.bin` — the file with game parameters. This is not player data but game-design definitions: weapon stats, effects, NPCs, balance.

Size: ~0x240010 bytes (~2.36 MB).

On PC: preceded by a 16-byte MD5 checksum.

---

## Contents (regulation.bin)

Regulation.bin contains **200+ parameter tables** (params):

| Param Table | Description |
|---|---|
| EquipParamWeapon | Weapon stats |
| EquipParamProtector | Armor stats |
| EquipParamAccessory | Talisman stats |
| EquipParamGoods | Item stats |
| SpEffectParam | Status-effect definitions |
| AtkParam_Pc | Player attack parameters |
| AtkParam_Npc | NPC attack parameters |
| NpcParam | NPC parameters |
| BulletParam | Projectile parameters |
| Magic | Spell parameters |
| ... | (200+ more) |

---

## Internal format

Regulation.bin is a packed BND4 container with many `.param` files. Each `.param` is an array of fixed-format records specific to the given type.

Param parser:
1. Unpack BND4
2. For each .param: read header (row size, row count)
3. Read records sequentially

---

## Editing implications

- **Regulation.bin is identical** for all players running the same game version
- Modification = "modding" game parameters (not save editing)
- In the context of the save editor: **read-only** — copy as-is between platforms
- During platform conversion: regulation.bin should be identical
- Mods (Convergence, Reforged) have a modified regulation.bin

---

## Sources

- er-save-manager: `parser/save.py` lines 231-237 (raw read)
- ER-Save-Editor (Rust): `src/save/common/user_data_11.rs`
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=format:param
- Souls Modding Wiki: https://www.soulsmodding.com/doku.php?id=er-refmat:param:equipparamweapon (param table example)
