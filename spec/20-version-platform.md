# 20 — Version & Platform Data

> **Scope**: Game base version, Steam ID, PS5 Activity.

---

## Overview

Three structures identifying the game version and the platform.

---

## BaseVersion (16 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u32 | Base Version Copy (copy of the version) |
| 0x04 | u32 | Base Version (game base version) |
| 0x08 | u32 | Is Latest Version (flag) |
| 0x0C | u32 | Unknown (unk0xc) |

---

## Steam ID (8 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u64 | Player's Steam ID |

- PC only — on PS4 this value is 0 or ignored
- On PS4→PC conversion: a valid Steam ID must be written
- On PC→PS4 conversion: ignored

---

## PS5 Activity (32 bytes)

| Offset | Type | Description |
|---|---|---|
| 0x00 | u8[32] | Opaque data — PS5 activity flags |

Present in PC saves as well (zeroed). Internal structure unknown.

---

## Editing implications

- **Steam ID**: critical for platform conversion. Wrong Steam ID = save rejected by Steam
- **Base Version**: changing may trigger save migration or rejection
- **PS5 Activity**: safe to zero

---

## Sources

- er-save-manager: `parser/world.py` — `BaseVersion` (lines 890-914), `PS5Activity` (922-935)
- er-save-manager: `parser/user_data_x.py` lines 191-193
