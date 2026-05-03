# 23 — UserData10 (Account Profile)

> **Type**: Binary format spec  
> **Scope**: Section shared across all slots — Mirror Favorites preset slots, ProfileSummary, SteamID, active slots, CSMenuSystemSaveLoad.

> **Status**: ✅ PC and PS4 offsets verified on live saves (Apr 2026): PC from `tmp/re-character/ER0000-{before,after}.sl2`, PS4 from `tmp/save/oisisk_ps4.txt`. **PC and PS4 have IDENTICAL UserData10 layout** (they differ only in save file headers and presence/absence of checksum — the UserData10.Data itself is the same).

> **Related section**: [31 — Appearance Presets](31-appearance-presets.md) — detailed layout of Mirror Favorites preset slot (0x130 bytes each).

---

## Overview

UserData10 is the section after the 10 character slots. Contains:
- Account information (Steam ID, UI settings)
- 15 appearance preset slots (Mirror Favorites — shared across all characters)
- 10 character summaries (ProfileSummary) — displayed in character select menu
- Active slot flags (10 bytes)
- Additional system menu data

Size: 0x60000 bytes (393,216 bytes) — fixed, regardless of number of active characters.

On PC: preceded by a 16-byte MD5 checksum (like character slots). PS4 has no checksum.

---

## Layout (post-checksum, PC verified)

```
┌─────────────────────────────────────────────────────┐
│ [PC only] MD5 Checksum (16 bytes)                   │ — before UserData10.Data
╞═════════════════════════════════════════════════════╡
│ Steam ID (u64) — 8 bytes                            │ @ 0x00
├─────────────────────────────────────────────────────┤
│ Settings / UI preferences (0x140 = 320 bytes)        │ @ 0x08
├─────────────────────────────────────────────────────┤
│ CSMenuSystemSaveLoad header (8 bytes: unk + length) │ @ 0x148
├─────────────────────────────────────────────────────┤
│ Mirror Favorites preset slots [15]                  │ @ 0x154
│  - Each slot: 0x130 bytes (304)                     │
│  - Total: 15 × 0x130 = 0x11D0 (4560 bytes)          │
│  - Span: 0x154..0x1323                              │
│  - Detailed layout: spec/31-appearance-presets.md   │
├─────────────────────────────────────────────────────┤
│ CSMenuSystemSaveLoad trailer (~0x630 bytes)         │ @ 0x1324
├─────────────────────────────────────────────────────┤
│ Active Slots (10 × u8: 0x01 active, 0x00 empty)     │ @ 0x1954
├─────────────────────────────────────────────────────┤
│ ProfileSummary[10]                                  │ @ 0x195E
│  - Each: 0x24C bytes (588) — name + face snapshot   │
│  - Total: 10 × 0x24C = 0x16F8 (5880 bytes)          │
│  - Span: 0x195E..0x3055                             │
├─────────────────────────────────────────────────────┤
│ ... (more menu data, gestures, regulation ver.)     │ @ 0x3056
│                                                     │
│ Remainder is zeros (padding to 0x60000)             │
└─────────────────────────────────────────────────────┘
```

---

## Offsets (PC and PS4 identical, verified)

| Field | Offset | Notes |
|---|---|---|
| Steam ID | 0x00 (u64) | PC only; on PS4 these 8 bytes have different meaning / zeros |
| Settings | 0x08..0x147 | UI preferences, account |
| CSMenuSystemSaveLoad header | 0x148 | unk + length |
| Mirror Favorites preset[0] | **0x154** | each slot 0x130 bytes, 15 slots |
| Active Slots | **0x1954** | 10 × u8 |
| ProfileSummary[0] | **0x195E** | each 0x24C bytes |
| ProfileSummary stride | **0x24C** | × 10 slots = 0x16F8 bytes |

⚠️ **HISTORICAL BUG (through end of Q2 2026)**: Our `backend/core/save_manager.go` wrote ProfileSummary at `0x31A + i*0x100` (PC) and `0x30A + i*0x100` (PS4). Those offsets lie **inside Mirror Favorites preset slot 1** (slot 1 spans 0x284..0x3B3), so every write corrupted preset slot 1. Hence the existence of `FavSafeSlots = [0, 10..14]` as a workaround. After fixing the offset, `FavSafeSlots` can be removed — all 15 preset slots are usable.

---

## ProfileSummary (0x24C = 588 bytes per slot)

Character summary visible in the character select menu. The game reads ONLY this data when showing the character list.

| Offset (slot-relative) | Type | Description |
|---|---|---|
| 0x000 | 5 × u8 | Marker bytes (observed: `01 01 01 01 01`) |
| 0x005 | 5 × u8 | Padding (zeros) |
| 0x00A | u16[16] | **Character Name** (UTF-16LE, max 16 chars + null) |
| 0x02A | 4 × u8 | Padding |
| 0x02E | u32 | Level |
| 0x032 | ... | (TODO details — observed FACE magic, model IDs, FaceShape, etc.) |
| 0x040 | u8[0x12C] | FaceData snapshot (mirror of slot) — game uses for appearance preview in menu |
| ... | ... | Remaining fields: equipment summary, archetype, starting gift, body_type |

**Important**: Our code currently writes only Name (32 bytes UTF-16) + Level (4 bytes) = 36 bytes. The remaining 552 bytes per slot retain the value last written by the game (previous game save). This is **functionally OK** — the game reads Name and Level (our correct values) plus FaceData snapshot (old but consistent with game data), so the menu shows the correct name and level, but the appearance snapshot may be outdated (cosmetic only).

ProfileSummary MUST be synchronized with data in the character slot — otherwise the menu shows incorrect information.

---

## Active Slots (10 × u8 @ 0x1954)

Array `[10]u8` — indicates which character slots (0-9) are active.
- `0x01` = active (character exists)
- `0x00` = empty

Modification: after adding/removing a character, the corresponding byte must be updated.

---

## CSMenuSystemSaveLoad (0x60000 bytes)

Large block of menu system data — HUD settings, display preferences, quickslot configuration at the account level.

---

## Editing implications

- **Steam ID**: must match the player's Steam ID on PC — otherwise the save won't load
- **ProfileSummary**: after editing name/level in a slot, this MUST also be updated
- **Active Slots**: must be updated after adding/removing a character
- **MD5**: after modifying UserData10 on PC — recalculate checksum
- **Platform conversion**: Active Slots and ProfileSummary offsets are DIFFERENT — wrong offset = corrupted save

---

## Sources

- er-save-manager: `parser/user_data_10.py` — class `UserData10`
- er-save-manager: `parser/save.py` lines 209-228
- Steam Guide: https://steamcommunity.com/sharedfiles/filedetails/?id=2797241037
