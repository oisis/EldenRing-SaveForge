# 20 — Version & Platform Data

> **Type**: Binary format spec  
> **Scope**: Game base version, Steam ID, PS5 Activity.

---

## Overview

Three structures identifying game version and platform.

---

## BaseVersion (16 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u32 | Base Version Copy (version duplicate) |
| 0x04 | u32 | Base Version (game base version) |
| 0x08 | u32 | Is Latest Version (flag) |
| 0x0C | u32 | Unknown (unk0xc) |

---

## Steam ID (8 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u64 | Player's Steam ID |

- PC only — on PS4 this value is 0 or ignored
- When converting PS4→PC: must write a valid Steam ID
- When converting PC→PS4: ignored

---

## PS5 Activity (32 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u8[32] | Opaque data — PS5 activity flags |

Present also in PC saves (zeroed). Internal structure unknown.

---

## Editing implications

- **Steam ID**: critical during platform conversion. Wrong Steam ID = save rejected by Steam
- **Base Version**: changing may trigger save migration or rejection
- **PS5 Activity**: safe to zero out

---

## Sources

- er-save-manager: `parser/world.py` — `BaseVersion` (lines 890-914), `PS5Activity` (922-935)
- er-save-manager: `parser/user_data_x.py` lines 191-193
